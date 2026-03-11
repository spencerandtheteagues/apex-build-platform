// Package agents - WebSocket Handler
// Real-time communication between agents and the frontend
package agents

import (
	apihandlers "apex-build/internal/handlers"
	"apex-build/internal/applog"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// isUserAdminDB checks if a user has admin privileges by querying the database.
// Returns false if DB is unavailable (safe default).
func isUserAdminDB(am *AgentManager, userID uint) bool {
	if am == nil || am.db == nil {
		return false
	}
	var isAdmin bool
	err := am.db.Raw("SELECT is_admin FROM users WHERE id = ? AND deleted_at IS NULL", userID).Scan(&isAdmin).Error
	if err != nil {
		return false
	}
	return isAdmin
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     apihandlers.AllowedWebSocketOrigin,
}

// WSHub manages WebSocket connections for builds
type WSHub struct {
	connections map[string]map[*WSConnection]bool
	broadcast   chan *broadcastMessage
	register    chan *registerRequest
	unregister  chan *WSConnection
	manager     *AgentManager
	mu          sync.RWMutex
}

// WSConnection represents a single WebSocket connection
type WSConnection struct {
	hub       *WSHub
	conn      *websocket.Conn
	buildID   string
	userID    uint
	send      chan []byte
	closeOnce sync.Once
}

type broadcastMessage struct {
	buildID string
	message []byte
}

type registerRequest struct {
	conn    *WSConnection
	buildID string
}

// NewWSHub creates a new WebSocket hub
func NewWSHub(manager *AgentManager) *WSHub {
	hub := &WSHub{
		connections: make(map[string]map[*WSConnection]bool),
		broadcast:   make(chan *broadcastMessage, 256),
		register:    make(chan *registerRequest),
		unregister:  make(chan *WSConnection),
		manager:     manager,
	}
	go hub.run()
	return hub
}

// run handles WebSocket events
func (h *WSHub) run() {
	for {
		select {
		case req := <-h.register:
			h.mu.Lock()
			if h.connections[req.buildID] == nil {
				h.connections[req.buildID] = make(map[*WSConnection]bool)
			}
			// Limit concurrent WebSocket connections per build to prevent DoS.
			const maxConnsPerBuild = 20
			if len(h.connections[req.buildID]) >= maxConnsPerBuild {
				h.mu.Unlock()
				log.Printf("WebSocket connection rejected for build %s: too many connections (%d)", req.buildID, maxConnsPerBuild)
				req.conn.conn.WriteMessage(websocket.CloseMessage, []byte("too many connections"))
				req.conn.conn.Close()
				break
			}
			h.connections[req.buildID][req.conn] = true
			h.mu.Unlock()

			log.Printf("WebSocket client connected to build %s", req.buildID)

		case conn := <-h.unregister:
			h.mu.Lock()
			if conns, ok := h.connections[conn.buildID]; ok {
				if _, ok := conns[conn]; ok {
					delete(conns, conn)
					conn.closeSend()
				}
			}
			h.mu.Unlock()

			log.Printf("WebSocket client disconnected from build %s", conn.buildID)

		case msg := <-h.broadcast:
			h.mu.RLock()
			conns := h.connections[msg.buildID]
			h.mu.RUnlock()

			for conn := range conns {
				select {
				case conn.send <- msg.message:
				default:
					h.mu.Lock()
					conn.closeSend()
					delete(h.connections[msg.buildID], conn)
					h.mu.Unlock()
				}
			}
		}
	}
}

func (c *WSConnection) closeSend() {
	c.closeOnce.Do(func() {
		close(c.send)
	})
}

// Broadcast sends a message to all connections for a build
func (h *WSHub) Broadcast(buildID string, msg *WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal WebSocket message: %v", err)
		return
	}

	h.broadcast <- &broadcastMessage{
		buildID: buildID,
		message: data,
	}
}

// HandleWebSocket handles new WebSocket connections
func (h *WSHub) HandleWebSocket(c *gin.Context) {
	buildID := c.Param("buildId")
	if buildID == "" {
		log.Printf("WebSocket connection rejected: missing buildId")
		c.JSON(http.StatusBadRequest, gin.H{"error": "build_id is required"})
		return
	}

	applog.Info("ws_request", "event", "ws_request", "build_id", buildID, "client_ip", c.ClientIP())

	// Verify build exists
	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		applog.WSRejected(buildID, "build_not_found")
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	uid, err := apihandlers.WebSocketUserID(c)
	if err != nil {
		applog.WSRejected(buildID, "auth_failed: "+err.Error())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	// Verify user has access to this build
	if uid != build.UserID && !isUserAdminDB(h.manager, uid) {
		applog.WSRejected(buildID, "forbidden")
		c.JSON(http.StatusForbidden, gin.H{"error": "not authorized for this build"})
		return
	}

	// Upgrade to WebSocket (after auth check)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		applog.WSRejected(buildID, "upgrade_failed: "+err.Error())
		return
	}

	applog.WSConnected(buildID, uid)

	wsConn := &WSConnection{
		hub:     h,
		conn:    conn,
		buildID: buildID,
		userID:  uid,
		send:    make(chan []byte, 256),
	}

	// Register connection
	h.register <- &registerRequest{conn: wsConn, buildID: buildID}

	// Subscribe to agent manager updates
	updateChan := make(chan *WSMessage, 100)
	forwardDone := make(chan struct{})
	h.manager.Subscribe(buildID, updateChan)

	// Forward agent updates to WebSocket
	go func() {
		for {
			select {
			case <-forwardDone:
				return
			case msg := <-updateChan:
				if msg == nil {
					continue
				}
				data, err := json.Marshal(msg)
				if err != nil {
					log.Printf("Failed to marshal WebSocket message for build %s: %v", buildID, err)
					continue
				}
				select {
				case wsConn.send <- data:
				default:
					log.Printf("WebSocket send buffer full for build %s, dropping message", buildID)
				}
			}
		}
	}()

	// Send initial build state first
	wsConn.sendBuildState()

	// Send connection confirmation
	confirmMsg := &WSMessage{
		Type:      "connection:established",
		BuildID:   buildID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"message":  "Connected to build stream",
			"build_id": buildID,
		},
	}
	if data, err := json.Marshal(confirmMsg); err == nil {
		select {
		case wsConn.send <- data:
		default:
		}
	}

	// Start connection handlers
	go wsConn.writePump()
	go wsConn.readPump(updateChan, forwardDone)
}

// writePump sends messages to the WebSocket connection
func (c *WSConnection) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump reads messages from the WebSocket connection
func (c *WSConnection) readPump(updateChan chan *WSMessage, forwardDone chan struct{}) {
	defer func() {
		close(forwardDone)
		c.hub.manager.Unsubscribe(c.buildID, updateChan)
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB max message size
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				applog.WSDisconnected(c.buildID, c.userID, err.Error())
			} else {
				applog.WSDisconnected(c.buildID, c.userID, "clean_close")
			}
			break
		}

		c.handleMessage(message)
	}
}

// handleMessage processes incoming WebSocket messages
func (c *WSConnection) handleMessage(message []byte) {
	var msg struct {
		Type         string         `json:"type"`
		Content      string         `json:"content"`
		Message      string         `json:"message"`
		ClientToken  string         `json:"client_token"`
		CheckpointID string         `json:"checkpoint_id"`
		Data         map[string]any `json:"data"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Failed to parse WebSocket message: %v", err)
		return
	}

	msgType := strings.ToLower(strings.TrimSpace(msg.Type))
	command := strings.ToLower(strings.TrimSpace(stringValue(msg.Data["command"])))
	content := firstNonEmpty(msg.Content, msg.Message, stringValue(msg.Data["content"]), stringValue(msg.Data["message"]))
	clientToken := firstNonEmpty(msg.ClientToken, stringValue(msg.Data["client_token"]))

	switch msgType {
	case "user:message", "user_message":
		if err := c.hub.manager.SendMessageWithClientToken(c.buildID, content, clientToken); err != nil {
			log.Printf("Failed to send message to lead agent: %v", err)
		}

	case "build:cancel":
		if err := c.hub.manager.CancelBuild(c.buildID); err != nil {
			log.Printf("Failed to cancel build %s: %v", c.buildID, err)
		}

	case "build:rollback":
		checkpointID := firstNonEmpty(msg.CheckpointID, stringValue(msg.Data["checkpoint_id"]))
		if checkpointID != "" {
			if err := c.hub.manager.RollbackToCheckpoint(c.buildID, checkpointID); err != nil {
				log.Printf("Failed to rollback: %v", err)
			}
		}

	case "pause", "build:pause":
		if _, err := c.hub.manager.PauseBuild(c.buildID, "Paused from WebSocket command"); err != nil {
			log.Printf("Failed to pause build %s: %v", c.buildID, err)
		}

	case "resume", "build:resume":
		if _, err := c.hub.manager.ResumeBuild(c.buildID, "Resumed from WebSocket command"); err != nil {
			log.Printf("Failed to resume build %s: %v", c.buildID, err)
		}

	case "command":
		switch command {
		case "pause":
			if _, err := c.hub.manager.PauseBuild(c.buildID, "Paused from WebSocket command"); err != nil {
				log.Printf("Failed to pause build %s: %v", c.buildID, err)
			}
		case "resume":
			if _, err := c.hub.manager.ResumeBuild(c.buildID, "Resumed from WebSocket command"); err != nil {
				log.Printf("Failed to resume build %s: %v", c.buildID, err)
			}
		case "cancel":
			if err := c.hub.manager.CancelBuild(c.buildID); err != nil {
				log.Printf("Failed to cancel build %s: %v", c.buildID, err)
			}
		case "rollback":
			checkpointID := firstNonEmpty(msg.CheckpointID, stringValue(msg.Data["checkpoint_id"]))
			if checkpointID != "" {
				if err := c.hub.manager.RollbackToCheckpoint(c.buildID, checkpointID); err != nil {
					log.Printf("Failed to rollback build %s: %v", c.buildID, err)
				}
			}
		case "start":
			log.Printf("Ignoring deprecated build:start websocket command for build %s", c.buildID)
		default:
			log.Printf("Unknown WebSocket command type: %s", command)
		}

	case "build:start":
		log.Printf("Ignoring deprecated build:start websocket event for build %s", c.buildID)

	default:
		log.Printf("Unknown WebSocket message type: %s", msg.Type)
	}
}

// sendBuildState sends the current build state to a new connection
func (c *WSConnection) sendBuildState() {
	build, err := c.hub.manager.GetBuild(c.buildID)
	if err != nil {
		log.Printf("Failed to get build %s for state sync: %v", c.buildID, err)
		return
	}

	build.mu.RLock()
	defer build.mu.RUnlock()
	interaction := copyBuildInteractionStateLocked(build)

	// Convert agents map to a serializable format
	agentsList := make([]map[string]any, 0, len(build.Agents))
	for _, agent := range build.Agents {
		agent.mu.RLock()
		agentData := map[string]any{
			"id":         agent.ID,
			"role":       string(agent.Role),
			"provider":   string(agent.Provider),
			"model":      agent.Model,
			"status":     string(agent.Status),
			"progress":   agent.Progress,
			"created_at": agent.CreatedAt,
			"updated_at": agent.UpdatedAt,
			"error":      agent.Error,
		}
		if agent.CurrentTask != nil {
			agentData["current_task"] = map[string]any{
				"id":          agent.CurrentTask.ID,
				"type":        string(agent.CurrentTask.Type),
				"description": agent.CurrentTask.Description,
				"status":      string(agent.CurrentTask.Status),
			}
		}
		agent.mu.RUnlock()
		agentsList = append(agentsList, agentData)
	}

	// Convert tasks to a serializable format
	tasksList := make([]map[string]any, 0, len(build.Tasks))
	for _, task := range build.Tasks {
		taskData := map[string]any{
			"id":          task.ID,
			"type":        string(task.Type),
			"description": task.Description,
			"status":      string(task.Status),
			"priority":    task.Priority,
			"assigned_to": task.AssignedTo,
			"created_at":  task.CreatedAt,
			"error":       task.Error,
		}
		if task.Output != nil {
			taskData["files_count"] = len(task.Output.Files)
		}
		tasksList = append(tasksList, taskData)
	}

	// Collect all generated files
	allFiles := make([]map[string]any, 0)
	for _, task := range build.Tasks {
		if task.Output != nil {
			for _, file := range task.Output.Files {
				allFiles = append(allFiles, map[string]any{
					"path":     file.Path,
					"language": file.Language,
					"size":     file.Size,
					"is_new":   file.IsNew,
				})
			}
		}
	}

	// Send build info
	state := &WSMessage{
		Type:      "build:state",
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"status":       string(build.Status),
			"mode":         string(build.Mode),
			"power_mode":   string(build.PowerMode),
			"description":  build.Description,
			"progress":     build.Progress,
			"agents":       agentsList,
			"agents_count": len(agentsList),
			"tasks":        tasksList,
			"tasks_count":  len(tasksList),
			"files":        allFiles,
			"files_count":  len(allFiles),
			"checkpoints":  build.Checkpoints,
			"created_at":   build.CreatedAt,
			"updated_at":   build.UpdatedAt,
			"completed_at": build.CompletedAt,
			"error":        build.Error,
			"messages":     interaction.Messages,
			"interaction":  interaction,
		},
	}

	data, err := json.Marshal(state)
	if err != nil {
		log.Printf("Failed to marshal build state for %s: %v", c.buildID, err)
		return
	}

	select {
	case c.send <- data:
		log.Printf("Sent build state for %s (%d agents, %d tasks, %d files)", c.buildID, len(agentsList), len(tasksList), len(allFiles))
	default:
		log.Printf("Failed to send build state for %s: channel full", c.buildID)
	}
}

// GetConnectionCount returns the number of active connections for a build
func (h *WSHub) GetConnectionCount(buildID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections[buildID])
}

// CloseAllConnections closes all connections for a build
func (h *WSHub) CloseAllConnections(buildID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for conn := range h.connections[buildID] {
		conn.closeSend()
	}
	delete(h.connections, buildID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
