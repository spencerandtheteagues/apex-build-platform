// Package agents - WebSocket Handler
// Real-time communication between agents and the frontend
package agents

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

// WebSocketClaims represents JWT claims for WebSocket authentication
type WebSocketClaims struct {
	UserID  uint `json:"user_id"`
	IsAdmin bool `json:"is_admin"`
	jwt.RegisteredClaims
}

// validateWebSocketToken validates a JWT token for WebSocket connections
func validateWebSocketToken(tokenString string) (*WebSocketClaims, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, errors.New("JWT_SECRET not configured")
	}

	token, err := jwt.ParseWithClaims(tokenString, &WebSocketClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*WebSocketClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token claims")
}

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
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		// Allow env-configured origins first
		if envOrigins := os.Getenv("CORS_ALLOWED_ORIGINS"); envOrigins != "" {
			for _, allowed := range strings.Split(envOrigins, ",") {
				if strings.TrimSpace(allowed) == origin {
					return true
				}
			}
			return false
		}

		// Non-production: allow all for local development
		if os.Getenv("ENVIRONMENT") != "production" {
			return true
		}

		// Production: strict origin check
		allowedOrigins := []string{
			"https://apex.build",
			"https://www.apex.build",
			"https://apex-frontend-gigq.onrender.com",
		}
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return false
	},
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
	hub     *WSHub
	conn    *websocket.Conn
	buildID string
	userID  uint
	send    chan []byte
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

	log.Printf("WebSocket connection request for build %s", buildID)

	// Verify build exists
	build, err := h.manager.GetBuild(buildID)
	if err != nil {
		log.Printf("WebSocket connection rejected: build %s not found", buildID)
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	// Get user ID from context (set by auth middleware) or from token query param
	var uid uint
	if userID, exists := c.Get("user_id"); exists {
		uid, _ = userID.(uint)
	} else {
		// Check for token in query parameter for WebSocket connections
		token := c.Query("token")
		if token == "" {
			log.Printf("WebSocket connection rejected: no authentication for build %s", buildID)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		// Validate the token and extract user ID
		// This should use the same JWT validation as the auth middleware
		claims, err := validateWebSocketToken(token)
		if err != nil {
			log.Printf("WebSocket connection rejected: invalid token for build %s: %v", buildID, err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		uid = claims.UserID
	}

	// Verify user has access to this build
	if uid != build.UserID && !isUserAdminDB(h.manager, uid) {
		log.Printf("WebSocket connection rejected: user %d not authorized for build %s (owner: %d)", uid, buildID, build.UserID)
		c.JSON(http.StatusForbidden, gin.H{"error": "not authorized for this build"})
		return
	}

	// Upgrade to WebSocket (after auth check)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed for build %s: %v", buildID, err)
		return
	}

	log.Printf("WebSocket connection established for build %s", buildID)

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
	h.manager.Subscribe(buildID, updateChan)

	// Forward agent updates to WebSocket
	go func() {
		for msg := range updateChan {
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
	go wsConn.readPump(updateChan)
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
func (c *WSConnection) readPump(updateChan chan *WSMessage) {
	defer func() {
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
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.handleMessage(message)
	}
}

// handleMessage processes incoming WebSocket messages
func (c *WSConnection) handleMessage(message []byte) {
	var msg struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Failed to parse WebSocket message: %v", err)
		return
	}

	switch msg.Type {
	case "user:message":
		// User is sending a message to the lead agent
		if err := c.hub.manager.SendMessage(c.buildID, msg.Content); err != nil {
			log.Printf("Failed to send message to lead agent: %v", err)
		}

	case "build:start":
		// User wants to start the build
		if err := c.hub.manager.StartBuild(c.buildID); err != nil {
			log.Printf("Failed to start build: %v", err)
		}

	case "build:cancel":
		if err := c.hub.manager.CancelBuild(c.buildID); err != nil {
			log.Printf("Failed to cancel build %s: %v", c.buildID, err)
		}

	case "build:rollback":
		// User wants to rollback to a checkpoint
		var rollbackMsg struct {
			CheckpointID string `json:"checkpoint_id"`
		}
		if err := json.Unmarshal(message, &rollbackMsg); err == nil {
			if err := c.hub.manager.RollbackToCheckpoint(c.buildID, rollbackMsg.CheckpointID); err != nil {
				log.Printf("Failed to rollback: %v", err)
			}
		}

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
