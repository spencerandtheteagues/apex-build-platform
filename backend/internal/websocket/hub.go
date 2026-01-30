// APEX.BUILD WebSocket Hub
// Real-time collaboration and communication system

package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Hub maintains active client connections and manages message broadcasting
type Hub struct {
	// Registered clients by room ID
	rooms map[string]map[*Client]bool

	// Active clients by user ID
	clients map[uint]*Client

	// Inbound messages from clients
	broadcast chan []byte

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Shutdown channel for graceful termination
	shutdown chan struct{}

	// Mutex for thread safety
	mu sync.RWMutex
}

// Client represents a WebSocket client connection
type Client struct {
	// The WebSocket connection
	conn *websocket.Conn

	// User information
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`

	// Current room/project
	RoomID    string `json:"room_id"`
	ProjectID uint   `json:"project_id"`

	// Buffered channel of outbound messages
	send chan []byte

	// Hub reference
	hub *Hub

	// Last activity time
	lastSeen time.Time

	// Cursor position
	cursorPosition *CursorPosition

	// Mutex for thread safety
	mu sync.RWMutex
}

// CursorPosition represents a user's cursor position in a file
type CursorPosition struct {
	FileID   uint `json:"file_id"`
	Line     int  `json:"line"`
	Column   int  `json:"column"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message types for WebSocket communication
const (
	MessageTypeJoinRoom      = "join_room"
	MessageTypeLeaveRoom     = "leave_room"
	MessageTypeCursorUpdate  = "cursor_update"
	MessageTypeFileChange    = "file_change"
	MessageTypeChat          = "chat"
	MessageTypeUserJoined    = "user_joined"
	MessageTypeUserLeft      = "user_left"
	MessageTypeUserList      = "user_list"
	MessageTypeError         = "error"
	MessageTypeHeartbeat     = "heartbeat"
)

// Message represents a WebSocket message
type Message struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data,omitempty"`
	UserID    uint        `json:"user_id,omitempty"`
	Username  string      `json:"username,omitempty"`
	RoomID    string      `json:"room_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// WebSocket upgrader configuration
// SECURITY: Strict origin checking - no empty origins in production
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		// Get allowed origins from environment or use defaults
		allowedOriginsEnv := os.Getenv("CORS_ALLOWED_ORIGINS")
		var allowedOrigins []string
		if allowedOriginsEnv != "" {
			allowedOrigins = strings.Split(allowedOriginsEnv, ",")
		} else {
			allowedOrigins = []string{
				"http://localhost:3000",
				"http://localhost:5173",
				"http://127.0.0.1:3000",
				"http://127.0.0.1:5173",
				"https://apex.build",
				"https://www.apex.build",
				"https://apex-frontend-gigq.onrender.com",
			}
		}

		for _, allowed := range allowedOrigins {
			if strings.TrimSpace(allowed) == origin {
				return true
			}
		}

		// SECURITY: Only allow empty origin in non-production for testing tools
		env := os.Getenv("ENVIRONMENT")
		if origin == "" && env != "production" {
			return true
		}

		return false
	},
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		clients:    make(map[uint]*Client),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		shutdown:   make(chan struct{}),
	}
}

// Run starts the hub's main loop
// FIXED: Added shutdown channel support to prevent goroutine leaks
func (h *Hub) Run() {
	for {
		select {
		case <-h.shutdown:
			// Graceful shutdown - close all client connections
			h.mu.Lock()
			for _, client := range h.clients {
				close(client.send)
			}
			h.clients = make(map[uint]*Client)
			h.rooms = make(map[string]map[*Client]bool)
			h.mu.Unlock()
			log.Println("WebSocket Hub shutdown complete")
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// Shutdown gracefully stops the hub
func (h *Hub) Shutdown() {
	close(h.shutdown)
}

// registerClient handles client registration
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Add client to global clients map
	h.clients[client.UserID] = client

	// Add client to room
	if client.RoomID != "" {
		if h.rooms[client.RoomID] == nil {
			h.rooms[client.RoomID] = make(map[*Client]bool)
		}
		h.rooms[client.RoomID][client] = true

		// Notify other clients in the room
		h.broadcastToRoom(client.RoomID, Message{
			Type:      MessageTypeUserJoined,
			UserID:    client.UserID,
			Username:  client.Username,
			RoomID:    client.RoomID,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"user_id":  client.UserID,
				"username": client.Username,
				"email":    client.Email,
			},
		}, client)

		// Send current user list to the new client
		h.sendUserList(client)
	}

	log.Printf("Client registered: UserID=%d, RoomID=%s, Total clients=%d",
		client.UserID, client.RoomID, len(h.clients))
}

// unregisterClient handles client disconnection
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove from global clients map
	delete(h.clients, client.UserID)

	// Remove from room
	if client.RoomID != "" && h.rooms[client.RoomID] != nil {
		delete(h.rooms[client.RoomID], client)

		// Clean up empty rooms
		if len(h.rooms[client.RoomID]) == 0 {
			delete(h.rooms, client.RoomID)
		} else {
			// Notify other clients in the room
			h.broadcastToRoom(client.RoomID, Message{
				Type:      MessageTypeUserLeft,
				UserID:    client.UserID,
				Username:  client.Username,
				RoomID:    client.RoomID,
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"user_id":  client.UserID,
					"username": client.Username,
				},
			}, nil)
		}
	}

	// Close the send channel
	close(client.send)

	log.Printf("Client unregistered: UserID=%d, RoomID=%s, Total clients=%d",
		client.UserID, client.RoomID, len(h.clients))
}

// broadcastMessage sends a message to all clients in the appropriate room
func (h *Hub) broadcastMessage(message []byte) {
	var msg Message
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Error unmarshaling message: %v", err)
		return
	}

	if msg.RoomID != "" {
		h.broadcastToRoom(msg.RoomID, msg, nil)
	}
}

// broadcastToRoom sends a message to all clients in a specific room
func (h *Hub) broadcastToRoom(roomID string, message Message, excludeClient *Client) {
	h.mu.RLock()
	roomClients := h.rooms[roomID]
	h.mu.RUnlock()

	if roomClients == nil {
		return
	}

	messageData, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	for client := range roomClients {
		if client == excludeClient {
			continue
		}

		select {
		case client.send <- messageData:
		default:
			// Client's send channel is full, remove the client
			close(client.send)
			delete(roomClients, client)
		}
	}
}

// sendUserList sends the current user list to a client
func (h *Hub) sendUserList(client *Client) {
	h.mu.RLock()
	roomClients := h.rooms[client.RoomID]
	h.mu.RUnlock()

	if roomClients == nil {
		return
	}

	users := make([]map[string]interface{}, 0, len(roomClients))
	for roomClient := range roomClients {
		users = append(users, map[string]interface{}{
			"user_id":  roomClient.UserID,
			"username": roomClient.Username,
			"email":    roomClient.Email,
			"cursor_position": roomClient.cursorPosition,
			"last_seen": roomClient.lastSeen,
		})
	}

	message := Message{
		Type:      MessageTypeUserList,
		RoomID:    client.RoomID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"users": users,
		},
	}

	messageData, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling user list: %v", err)
		return
	}

	select {
	case client.send <- messageData:
	default:
		// Client's send channel is full
		log.Printf("Failed to send user list to client %d", client.UserID)
	}
}

// GetRoomUsers returns a list of users in a specific room
func (h *Hub) GetRoomUsers(roomID string) []map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	roomClients := h.rooms[roomID]
	if roomClients == nil {
		return []map[string]interface{}{}
	}

	users := make([]map[string]interface{}, 0, len(roomClients))
	for client := range roomClients {
		users = append(users, map[string]interface{}{
			"user_id":  client.UserID,
			"username": client.Username,
			"email":    client.Email,
			"cursor_position": client.cursorPosition,
			"last_seen": client.lastSeen,
		})
	}

	return users
}

// GetActiveRooms returns a list of active rooms
func (h *Hub) GetActiveRooms() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	rooms := make([]string, 0, len(h.rooms))
	for roomID := range h.rooms {
		rooms = append(rooms, roomID)
	}

	return rooms
}

// GetClientCount returns the total number of connected clients
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients)
}

// SendToUser sends a message to a specific user
func (h *Hub) SendToUser(userID uint, message Message) error {
	h.mu.RLock()
	client, exists := h.clients[userID]
	h.mu.RUnlock()

	if !exists {
		return nil // User not connected
	}

	messageData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	select {
	case client.send <- messageData:
		return nil
	default:
		return nil // Client's send channel is full
	}
}

// BroadcastToRoom broadcasts a message to all users in a room
func (h *Hub) BroadcastToRoom(roomID string, message Message) {
	h.broadcastToRoom(roomID, message, nil)
}

// HandleWebSocket handles WebSocket connection upgrades
func (h *Hub) HandleWebSocket(c *gin.Context) {
	// Extract user information from context (set by auth middleware)
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userID := userIDInterface.(uint)
	username, _ := c.Get("username")
	email, _ := c.Get("email")

	// Get room/project ID from query parameters
	roomID := c.Query("room_id")
	projectIDStr := c.Query("project_id")

	var projectID uint
	if projectIDStr != "" {
		if pid, err := strconv.ParseUint(projectIDStr, 10, 32); err == nil {
			projectID = uint(pid)
		}
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Create new client
	client := &Client{
		conn:      conn,
		UserID:    userID,
		Username:  username.(string),
		Email:     email.(string),
		RoomID:    roomID,
		ProjectID: projectID,
		send:      make(chan []byte, 256),
		hub:       h,
		lastSeen:  time.Now(),
	}

	// Register client
	h.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}