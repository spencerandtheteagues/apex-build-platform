// Package agents - WebSocket Handler
// Real-time communication between agents and the frontend
package agents

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
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
					close(conn.send)
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
					close(conn.send)
					delete(h.connections[msg.buildID], conn)
					h.mu.Unlock()
				}
			}
		}
	}
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "build_id is required"})
		return
	}

	// Verify build exists
	_, err := h.manager.GetBuild(buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "build not found"})
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

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
				continue
			}
			select {
			case wsConn.send <- data:
			default:
			}
		}
	}()

	// Start connection handlers
	go wsConn.writePump()
	go wsConn.readPump(updateChan)

	// Send initial build state
	wsConn.sendBuildState()
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
		// User wants to cancel the build
		// TODO: Implement build cancellation

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
		return
	}

	build.mu.RLock()
	defer build.mu.RUnlock()

	// Send build info
	state := &WSMessage{
		Type:      "build:state",
		BuildID:   build.ID,
		Timestamp: time.Now(),
		Data: map[string]any{
			"status":       string(build.Status),
			"mode":         string(build.Mode),
			"description":  build.Description,
			"progress":     build.Progress,
			"agents":       build.Agents,
			"tasks":        build.Tasks,
			"checkpoints":  build.Checkpoints,
			"created_at":   build.CreatedAt,
			"updated_at":   build.UpdatedAt,
			"completed_at": build.CompletedAt,
		},
	}

	data, err := json.Marshal(state)
	if err != nil {
		return
	}

	select {
	case c.send <- data:
	default:
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
		close(conn.send)
	}
	delete(h.connections, buildID)
}
