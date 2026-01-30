// APEX.BUILD Collaboration Hub
// Central hub for real-time collaboration management

package collaboration

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Message types for collaboration
const (
	// Connection messages
	MsgJoinRoom       = "join_room"
	MsgLeaveRoom      = "leave_room"
	MsgRoomJoined     = "room_joined"
	MsgRoomLeft       = "room_left"
	MsgUserJoined     = "user_joined"
	MsgUserLeft       = "user_left"
	MsgError          = "error"
	MsgHeartbeat      = "heartbeat"

	// Presence messages
	MsgCursorUpdate     = "cursor_update"
	MsgSelectionUpdate  = "selection_update"
	MsgTypingStart      = "typing_start"
	MsgTypingStop       = "typing_stop"
	MsgFollowUser       = "follow_user"
	MsgStopFollowing    = "stop_following"
	MsgPresenceUpdate   = "presence_update"
	MsgUserList         = "user_list"

	// Document messages
	MsgOperation        = "operation"
	MsgOperationAck     = "operation_ack"
	MsgSyncRequest      = "sync_request"
	MsgSyncResponse     = "sync_response"
	MsgFileChange       = "file_change"

	// Permission messages
	MsgPermissionUpdate = "permission_update"
	MsgKickUser         = "kick_user"
	MsgUserKicked       = "user_kicked"

	// Chat messages
	MsgChat             = "chat"
	MsgChatHistory      = "chat_history"

	// Voice/Video messages
	MsgRTCOffer         = "rtc_offer"
	MsgRTCAnswer        = "rtc_answer"
	MsgRTCCandidate     = "rtc_candidate"
	MsgMediaStateChange = "media_state_change"

	// Activity messages
	MsgActivity         = "activity"
	MsgActivityFeed     = "activity_feed"
)

// CollabMessage represents a collaboration message
type CollabMessage struct {
	Type      string          `json:"type"`
	RoomID    string          `json:"room_id,omitempty"`
	UserID    uint            `json:"user_id,omitempty"`
	Username  string          `json:"username,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// CollabClient represents a connected collaboration client
type CollabClient struct {
	conn          *websocket.Conn
	hub           *CollabHub
	send          chan []byte
	userID        uint
	username      string
	email         string
	avatarURL     string
	roomID        string
	permission    PermissionLevel
	lastSeen      time.Time
	mu            sync.RWMutex
}

// RoomState holds the state for a collaboration room
type RoomState struct {
	RoomID       string
	ProjectID    uint
	Clients      map[uint]*CollabClient
	Documents    map[uint]*Document
	ActivityFeed *ActivityFeed
	ChatHistory  []ChatMessage
	CreatedAt    time.Time
	mu           sync.RWMutex
}

// ChatMessage represents a chat message in a room
type ChatMessage struct {
	ID        string    `json:"id"`
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Message   string    `json:"message"`
	Type      string    `json:"type"` // text, code, system
	Timestamp time.Time `json:"timestamp"`
}

// CollabHub manages all collaboration connections
type CollabHub struct {
	rooms           map[string]*RoomState
	clients         map[uint]*CollabClient
	presenceManager *PresenceManager
	otEngine        *OTEngine
	register        chan *CollabClient
	unregister      chan *CollabClient
	broadcast       chan *broadcastMsg
	mu              sync.RWMutex
}

type broadcastMsg struct {
	roomID  string
	message []byte
	exclude uint // UserID to exclude
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"http://localhost:3001",
			"http://127.0.0.1:3000",
			"https://apex.build",
		}
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return origin == ""
	},
}

// NewCollabHub creates a new collaboration hub
func NewCollabHub() *CollabHub {
	return &CollabHub{
		rooms:           make(map[string]*RoomState),
		clients:         make(map[uint]*CollabClient),
		presenceManager: NewPresenceManager(),
		otEngine:        NewOTEngine(),
		register:        make(chan *CollabClient),
		unregister:      make(chan *CollabClient),
		broadcast:       make(chan *broadcastMsg, 256),
	}
}

// Run starts the hub's main loop
func (h *CollabHub) Run() {
	// Cleanup ticker for inactive users
	cleanupTicker := time.NewTicker(30 * time.Second)
	defer cleanupTicker.Stop()

	for {
		select {
		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case msg := <-h.broadcast:
			h.handleBroadcast(msg)

		case <-cleanupTicker.C:
			h.cleanupInactiveUsers()
		}
	}
}

// handleRegister handles client registration
func (h *CollabHub) handleRegister(client *CollabClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Add to global clients
	h.clients[client.userID] = client

	// If already in a room, add to room
	if client.roomID != "" {
		h.addClientToRoom(client)
	}

	log.Printf("Client registered: UserID=%d, Total=%d", client.userID, len(h.clients))
}

// handleUnregister handles client disconnection
func (h *CollabHub) handleUnregister(client *CollabClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if client.roomID != "" {
		h.removeClientFromRoom(client)
	}

	delete(h.clients, client.userID)
	close(client.send)

	log.Printf("Client unregistered: UserID=%d, Total=%d", client.userID, len(h.clients))
}

// addClientToRoom adds a client to a room
func (h *CollabHub) addClientToRoom(client *CollabClient) {
	room := h.rooms[client.roomID]
	if room == nil {
		room = &RoomState{
			RoomID:       client.roomID,
			Clients:      make(map[uint]*CollabClient),
			Documents:    make(map[uint]*Document),
			ActivityFeed: NewActivityFeed(client.roomID, 100),
			ChatHistory:  make([]ChatMessage, 0),
			CreatedAt:    time.Now(),
		}
		h.rooms[client.roomID] = room
	}

	room.mu.Lock()
	room.Clients[client.userID] = client
	room.mu.Unlock()

	// Add to presence manager
	presence := h.presenceManager.JoinRoom(
		client.roomID,
		client.userID,
		client.username,
		client.email,
		client.avatarURL,
		client.permission,
	)

	// Add activity
	room.ActivityFeed.AddActivity(ActivityFeedItem{
		ID:        generateID(),
		UserID:    client.userID,
		Username:  client.username,
		AvatarURL: client.avatarURL,
		Action:    "joined",
		Timestamp: time.Now(),
	})

	// Notify other clients
	h.broadcastToRoomUnlocked(client.roomID, &CollabMessage{
		Type:      MsgUserJoined,
		RoomID:    client.roomID,
		UserID:    client.userID,
		Username:  client.username,
		Timestamp: time.Now(),
		Data:      mustMarshal(presence),
	}, client.userID)

	// Send user list to new client
	h.sendUserList(client)
}

// removeClientFromRoom removes a client from a room
func (h *CollabHub) removeClientFromRoom(client *CollabClient) {
	room := h.rooms[client.roomID]
	if room == nil {
		return
	}

	room.mu.Lock()
	delete(room.Clients, client.userID)
	isEmpty := len(room.Clients) == 0
	room.mu.Unlock()

	// Remove from presence manager
	h.presenceManager.LeaveRoom(client.roomID, client.userID)

	// Add activity
	room.ActivityFeed.AddActivity(ActivityFeedItem{
		ID:        generateID(),
		UserID:    client.userID,
		Username:  client.username,
		AvatarURL: client.avatarURL,
		Action:    "left",
		Timestamp: time.Now(),
	})

	// Notify other clients
	h.broadcastToRoomUnlocked(client.roomID, &CollabMessage{
		Type:      MsgUserLeft,
		RoomID:    client.roomID,
		UserID:    client.userID,
		Username:  client.username,
		Timestamp: time.Now(),
	}, 0)

	// Clean up empty rooms
	if isEmpty {
		delete(h.rooms, client.roomID)
	}
}

// handleBroadcast broadcasts a message to a room
func (h *CollabHub) handleBroadcast(msg *broadcastMsg) {
	h.mu.RLock()
	room := h.rooms[msg.roomID]
	h.mu.RUnlock()

	if room == nil {
		return
	}

	room.mu.RLock()
	for userID, client := range room.Clients {
		if userID == msg.exclude {
			continue
		}
		select {
		case client.send <- msg.message:
		default:
			// Client buffer full, will be cleaned up
		}
	}
	room.mu.RUnlock()
}

// broadcastToRoomUnlocked broadcasts without acquiring hub lock (caller must hold lock)
func (h *CollabHub) broadcastToRoomUnlocked(roomID string, msg *CollabMessage, excludeUserID uint) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	room := h.rooms[roomID]
	if room == nil {
		return
	}

	room.mu.RLock()
	for userID, client := range room.Clients {
		if userID == excludeUserID {
			continue
		}
		select {
		case client.send <- data:
		default:
		}
	}
	room.mu.RUnlock()
}

// BroadcastToRoom broadcasts a message to all clients in a room
func (h *CollabHub) BroadcastToRoom(roomID string, msg *CollabMessage, excludeUserID uint) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	h.broadcast <- &broadcastMsg{
		roomID:  roomID,
		message: data,
		exclude: excludeUserID,
	}
}

// sendUserList sends the current user list to a client
func (h *CollabHub) sendUserList(client *CollabClient) {
	presences := h.presenceManager.GetRoomPresence(client.roomID)

	msg := &CollabMessage{
		Type:      MsgUserList,
		RoomID:    client.roomID,
		Timestamp: time.Now(),
		Data:      mustMarshal(presences),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	select {
	case client.send <- data:
	default:
	}
}

// cleanupInactiveUsers removes users who have been inactive
func (h *CollabHub) cleanupInactiveUsers() {
	removed := h.presenceManager.CleanupInactive(5 * time.Minute)
	for _, r := range removed {
		h.BroadcastToRoom(r.RoomID, &CollabMessage{
			Type:      MsgUserLeft,
			RoomID:    r.RoomID,
			UserID:    r.UserID,
			Timestamp: time.Now(),
		}, 0)
	}
}

// HandleWebSocket handles WebSocket connection upgrades
func (h *CollabHub) HandleWebSocket(c *gin.Context) {
	// Get user from context
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userID := userIDInterface.(uint)
	username, _ := c.Get("username")
	email, _ := c.Get("email")

	// Get room info from query
	roomID := c.Query("room_id")
	projectIDStr := c.Query("project_id")
	permissionStr := c.DefaultQuery("permission", "editor")

	var projectID uint
	if projectIDStr != "" {
		if pid, err := strconv.ParseUint(projectIDStr, 10, 32); err == nil {
			projectID = uint(pid)
		}
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Parse permission
	permission := PermissionEditor
	switch permissionStr {
	case "viewer":
		permission = PermissionViewer
	case "admin":
		permission = PermissionAdmin
	case "owner":
		permission = PermissionOwner
	}

	// Create client
	client := &CollabClient{
		conn:       conn,
		hub:        h,
		send:       make(chan []byte, 256),
		userID:     userID,
		username:   username.(string),
		email:      email.(string),
		roomID:     roomID,
		permission: permission,
		lastSeen:   time.Now(),
	}

	// Set project ID in room if needed
	if roomID != "" && projectID > 0 {
		h.mu.Lock()
		if room := h.rooms[roomID]; room != nil {
			room.ProjectID = projectID
		}
		h.mu.Unlock()
	}

	// Register client
	h.register <- client

	// Start goroutines
	go client.writePump()
	go client.readPump()
}

// Client methods

// readPump pumps messages from the WebSocket connection
func (c *CollabClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(65536)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.mu.Lock()
		c.lastSeen = time.Now()
		c.mu.Unlock()

		c.handleMessage(data)
	}
}

// writePump pumps messages to the WebSocket connection
func (c *CollabClient) writePump() {
	ticker := time.NewTicker(45 * time.Second)
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

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Batch queued messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
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

// handleMessage processes incoming messages
func (c *CollabClient) handleMessage(data []byte) {
	var msg CollabMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("Invalid message format")
		return
	}

	msg.UserID = c.userID
	msg.Username = c.username
	msg.Timestamp = time.Now()

	switch msg.Type {
	case MsgJoinRoom:
		c.handleJoinRoom(msg)
	case MsgLeaveRoom:
		c.handleLeaveRoom(msg)
	case MsgCursorUpdate:
		c.handleCursorUpdate(msg)
	case MsgSelectionUpdate:
		c.handleSelectionUpdate(msg)
	case MsgTypingStart:
		c.handleTypingStart(msg)
	case MsgTypingStop:
		c.handleTypingStop(msg)
	case MsgFollowUser:
		c.handleFollowUser(msg)
	case MsgStopFollowing:
		c.handleStopFollowing(msg)
	case MsgOperation:
		c.handleOperation(msg)
	case MsgSyncRequest:
		c.handleSyncRequest(msg)
	case MsgChat:
		c.handleChat(msg)
	case MsgPermissionUpdate:
		c.handlePermissionUpdate(msg)
	case MsgKickUser:
		c.handleKickUser(msg)
	case MsgRTCOffer, MsgRTCAnswer, MsgRTCCandidate:
		c.handleRTCSignaling(msg)
	case MsgMediaStateChange:
		c.handleMediaStateChange(msg)
	case MsgHeartbeat:
		c.handleHeartbeat(msg)
	default:
		c.sendError("Unknown message type: " + msg.Type)
	}
}

// Message handlers

func (c *CollabClient) handleJoinRoom(msg CollabMessage) {
	var data struct {
		RoomID    string `json:"room_id"`
		ProjectID uint   `json:"project_id"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		c.sendError("Invalid join room data")
		return
	}

	// Leave current room if in one
	if c.roomID != "" && c.roomID != data.RoomID {
		c.hub.mu.Lock()
		c.hub.removeClientFromRoom(c)
		c.hub.mu.Unlock()
	}

	c.mu.Lock()
	c.roomID = data.RoomID
	c.mu.Unlock()

	c.hub.mu.Lock()
	c.hub.addClientToRoom(c)
	c.hub.mu.Unlock()

	// Send confirmation
	c.send <- mustMarshal(&CollabMessage{
		Type:      MsgRoomJoined,
		RoomID:    data.RoomID,
		UserID:    c.userID,
		Timestamp: time.Now(),
		Data:      mustMarshal(map[string]interface{}{"success": true}),
	})
}

func (c *CollabClient) handleLeaveRoom(msg CollabMessage) {
	if c.roomID == "" {
		return
	}

	c.hub.mu.Lock()
	c.hub.removeClientFromRoom(c)
	c.hub.mu.Unlock()

	c.mu.Lock()
	c.roomID = ""
	c.mu.Unlock()

	c.send <- mustMarshal(&CollabMessage{
		Type:      MsgRoomLeft,
		UserID:    c.userID,
		Timestamp: time.Now(),
	})
}

func (c *CollabClient) handleCursorUpdate(msg CollabMessage) {
	var data struct {
		FileID   uint   `json:"file_id"`
		FileName string `json:"file_name"`
		Line     int    `json:"line"`
		Column   int    `json:"column"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return
	}

	presence := c.hub.presenceManager.UpdateCursor(c.roomID, c.userID, data.FileID, data.FileName, data.Line, data.Column)
	if presence == nil {
		return
	}

	c.hub.BroadcastToRoom(c.roomID, &CollabMessage{
		Type:      MsgCursorUpdate,
		RoomID:    c.roomID,
		UserID:    c.userID,
		Username:  c.username,
		Timestamp: time.Now(),
		Data:      mustMarshal(presence),
	}, c.userID)

	// Notify followers
	followers := c.hub.presenceManager.GetFollowers(c.roomID, c.userID)
	for _, follower := range followers {
		if client := c.hub.clients[follower.UserID]; client != nil {
			client.send <- mustMarshal(&CollabMessage{
				Type:      MsgCursorUpdate,
				RoomID:    c.roomID,
				UserID:    c.userID,
				Username:  c.username,
				Timestamp: time.Now(),
				Data:      mustMarshal(map[string]interface{}{"following": true, "presence": presence}),
			})
		}
	}
}

func (c *CollabClient) handleSelectionUpdate(msg CollabMessage) {
	var data struct {
		StartLine   int  `json:"start_line"`
		StartColumn int  `json:"start_column"`
		EndLine     int  `json:"end_line"`
		EndColumn   int  `json:"end_column"`
		Clear       bool `json:"clear,omitempty"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return
	}

	var presence *UserPresence
	if data.Clear {
		presence = c.hub.presenceManager.ClearSelection(c.roomID, c.userID)
	} else {
		presence = c.hub.presenceManager.UpdateSelection(c.roomID, c.userID, data.StartLine, data.StartColumn, data.EndLine, data.EndColumn)
	}

	if presence == nil {
		return
	}

	c.hub.BroadcastToRoom(c.roomID, &CollabMessage{
		Type:      MsgSelectionUpdate,
		RoomID:    c.roomID,
		UserID:    c.userID,
		Username:  c.username,
		Timestamp: time.Now(),
		Data:      mustMarshal(presence),
	}, c.userID)
}

func (c *CollabClient) handleTypingStart(msg CollabMessage) {
	presence := c.hub.presenceManager.SetTyping(c.roomID, c.userID, true)
	if presence == nil {
		return
	}

	c.hub.BroadcastToRoom(c.roomID, &CollabMessage{
		Type:      MsgTypingStart,
		RoomID:    c.roomID,
		UserID:    c.userID,
		Username:  c.username,
		Timestamp: time.Now(),
	}, c.userID)
}

func (c *CollabClient) handleTypingStop(msg CollabMessage) {
	presence := c.hub.presenceManager.SetTyping(c.roomID, c.userID, false)
	if presence == nil {
		return
	}

	c.hub.BroadcastToRoom(c.roomID, &CollabMessage{
		Type:      MsgTypingStop,
		RoomID:    c.roomID,
		UserID:    c.userID,
		Username:  c.username,
		Timestamp: time.Now(),
	}, c.userID)
}

func (c *CollabClient) handleFollowUser(msg CollabMessage) {
	var data struct {
		TargetUserID uint `json:"target_user_id"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return
	}

	presence := c.hub.presenceManager.SetFollowing(c.roomID, c.userID, data.TargetUserID)
	if presence == nil {
		return
	}

	// Notify target user
	c.hub.BroadcastToRoom(c.roomID, &CollabMessage{
		Type:      MsgFollowUser,
		RoomID:    c.roomID,
		UserID:    c.userID,
		Username:  c.username,
		Timestamp: time.Now(),
		Data:      mustMarshal(map[string]interface{}{"following": data.TargetUserID}),
	}, 0)
}

func (c *CollabClient) handleStopFollowing(msg CollabMessage) {
	presence := c.hub.presenceManager.StopFollowing(c.roomID, c.userID)
	if presence == nil {
		return
	}

	c.hub.BroadcastToRoom(c.roomID, &CollabMessage{
		Type:      MsgStopFollowing,
		RoomID:    c.roomID,
		UserID:    c.userID,
		Username:  c.username,
		Timestamp: time.Now(),
	}, 0)
}

func (c *CollabClient) handleOperation(msg CollabMessage) {
	// Check edit permission
	if !c.hub.presenceManager.CanEdit(c.roomID, c.userID) {
		c.sendError("You don't have edit permission")
		return
	}

	var op TextOperation
	if err := json.Unmarshal(msg.Data, &op); err != nil {
		c.sendError("Invalid operation data")
		return
	}

	op.UserID = c.userID
	op.Timestamp = time.Now()

	// Apply operation with OT
	doc, transformedOps, err := c.hub.otEngine.Apply(op)
	if err != nil {
		c.sendError("Failed to apply operation: " + err.Error())
		return
	}

	// Send ack to sender
	c.send <- mustMarshal(&CollabMessage{
		Type:      MsgOperationAck,
		RoomID:    c.roomID,
		UserID:    c.userID,
		Timestamp: time.Now(),
		Data:      mustMarshal(map[string]interface{}{"version": doc.Version, "success": true}),
	})

	// Broadcast transformed operation to others
	c.hub.BroadcastToRoom(c.roomID, &CollabMessage{
		Type:      MsgOperation,
		RoomID:    c.roomID,
		UserID:    c.userID,
		Username:  c.username,
		Timestamp: time.Now(),
		Data:      mustMarshal(map[string]interface{}{"operations": transformedOps, "version": doc.Version, "file_id": op.FileID}),
	}, c.userID)

	// Add to activity feed
	c.hub.mu.RLock()
	room := c.hub.rooms[c.roomID]
	c.hub.mu.RUnlock()
	if room != nil {
		room.ActivityFeed.AddActivity(ActivityFeedItem{
			ID:        generateID(),
			UserID:    c.userID,
			Username:  c.username,
			AvatarURL: c.avatarURL,
			Action:    "edited",
			Target:    "", // Could add file name here
			Timestamp: time.Now(),
		})
	}
}

func (c *CollabClient) handleSyncRequest(msg CollabMessage) {
	var data struct {
		FileID  uint `json:"file_id"`
		Version int  `json:"version"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		c.sendError("Invalid sync request")
		return
	}

	doc := c.hub.otEngine.GetDocument(data.FileID, "")

	c.send <- mustMarshal(&CollabMessage{
		Type:      MsgSyncResponse,
		RoomID:    c.roomID,
		UserID:    c.userID,
		Timestamp: time.Now(),
		Data:      mustMarshal(doc.State()),
	})
}

func (c *CollabClient) handleChat(msg CollabMessage) {
	var data struct {
		Message string `json:"message"`
		Type    string `json:"type,omitempty"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return
	}

	if data.Message == "" || len(data.Message) > 2000 {
		return
	}

	chatType := "text"
	if data.Type != "" {
		chatType = data.Type
	}

	chatMsg := ChatMessage{
		ID:        generateID(),
		UserID:    c.userID,
		Username:  c.username,
		AvatarURL: c.avatarURL,
		Message:   data.Message,
		Type:      chatType,
		Timestamp: time.Now(),
	}

	// Store in history
	c.hub.mu.Lock()
	if room := c.hub.rooms[c.roomID]; room != nil {
		room.mu.Lock()
		room.ChatHistory = append(room.ChatHistory, chatMsg)
		if len(room.ChatHistory) > 500 {
			room.ChatHistory = room.ChatHistory[len(room.ChatHistory)-500:]
		}
		room.mu.Unlock()
	}
	c.hub.mu.Unlock()

	// Broadcast
	c.hub.BroadcastToRoom(c.roomID, &CollabMessage{
		Type:      MsgChat,
		RoomID:    c.roomID,
		UserID:    c.userID,
		Username:  c.username,
		Timestamp: time.Now(),
		Data:      mustMarshal(chatMsg),
	}, 0)
}

func (c *CollabClient) handlePermissionUpdate(msg CollabMessage) {
	if !c.hub.presenceManager.IsAdmin(c.roomID, c.userID) {
		c.sendError("Admin permission required")
		return
	}

	var data struct {
		TargetUserID uint            `json:"target_user_id"`
		Permission   PermissionLevel `json:"permission"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return
	}

	presence := c.hub.presenceManager.SetPermission(c.roomID, data.TargetUserID, data.Permission)
	if presence == nil {
		return
	}

	// Update client's permission
	c.hub.mu.RLock()
	if room := c.hub.rooms[c.roomID]; room != nil {
		room.mu.Lock()
		if client := room.Clients[data.TargetUserID]; client != nil {
			client.permission = data.Permission
		}
		room.mu.Unlock()
	}
	c.hub.mu.RUnlock()

	c.hub.BroadcastToRoom(c.roomID, &CollabMessage{
		Type:      MsgPermissionUpdate,
		RoomID:    c.roomID,
		UserID:    data.TargetUserID,
		Timestamp: time.Now(),
		Data:      mustMarshal(map[string]interface{}{"user_id": data.TargetUserID, "permission": data.Permission}),
	}, 0)
}

func (c *CollabClient) handleKickUser(msg CollabMessage) {
	var data struct {
		TargetUserID uint `json:"target_user_id"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return
	}

	if !c.hub.presenceManager.KickUser(c.roomID, c.userID, data.TargetUserID) {
		c.sendError("Cannot kick user")
		return
	}

	// Notify kicked user
	c.hub.mu.RLock()
	if room := c.hub.rooms[c.roomID]; room != nil {
		room.mu.RLock()
		if client := room.Clients[data.TargetUserID]; client != nil {
			client.send <- mustMarshal(&CollabMessage{
				Type:      MsgUserKicked,
				RoomID:    c.roomID,
				UserID:    data.TargetUserID,
				Timestamp: time.Now(),
			})
		}
		room.mu.RUnlock()
	}
	c.hub.mu.RUnlock()

	// Broadcast
	c.hub.BroadcastToRoom(c.roomID, &CollabMessage{
		Type:      MsgUserKicked,
		RoomID:    c.roomID,
		UserID:    data.TargetUserID,
		Timestamp: time.Now(),
		Data:      mustMarshal(map[string]interface{}{"kicked_by": c.userID}),
	}, data.TargetUserID)
}

func (c *CollabClient) handleRTCSignaling(msg CollabMessage) {
	var data struct {
		TargetUserID uint            `json:"target_user_id"`
		Payload      json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return
	}

	// Forward to target user
	c.hub.mu.RLock()
	targetClient := c.hub.clients[data.TargetUserID]
	c.hub.mu.RUnlock()

	if targetClient != nil {
		targetClient.send <- mustMarshal(&CollabMessage{
			Type:      msg.Type,
			RoomID:    c.roomID,
			UserID:    c.userID,
			Username:  c.username,
			Timestamp: time.Now(),
			Data:      mustMarshal(map[string]interface{}{"from_user_id": c.userID, "payload": data.Payload}),
		})
	}
}

func (c *CollabClient) handleMediaStateChange(msg CollabMessage) {
	c.hub.BroadcastToRoom(c.roomID, &CollabMessage{
		Type:      MsgMediaStateChange,
		RoomID:    c.roomID,
		UserID:    c.userID,
		Username:  c.username,
		Timestamp: time.Now(),
		Data:      msg.Data,
	}, c.userID)
}

func (c *CollabClient) handleHeartbeat(msg CollabMessage) {
	c.send <- mustMarshal(&CollabMessage{
		Type:      MsgHeartbeat,
		UserID:    c.userID,
		Timestamp: time.Now(),
		Data:      mustMarshal(map[string]interface{}{"pong": true}),
	})
}

func (c *CollabClient) sendError(message string) {
	c.send <- mustMarshal(&CollabMessage{
		Type:      MsgError,
		UserID:    c.userID,
		Timestamp: time.Now(),
		Data:      mustMarshal(map[string]interface{}{"error": message}),
	})
}

// Helper functions

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + strconv.FormatInt(time.Now().UnixNano()%10000, 10)
}

// GetPresenceManager returns the presence manager
func (h *CollabHub) GetPresenceManager() *PresenceManager {
	return h.presenceManager
}

// GetOTEngine returns the OT engine
func (h *CollabHub) GetOTEngine() *OTEngine {
	return h.otEngine
}

// GetRoomState returns the state for a room
func (h *CollabHub) GetRoomState(roomID string) *RoomState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rooms[roomID]
}

// GetChatHistory returns chat history for a room
func (h *CollabHub) GetChatHistory(roomID string) []ChatMessage {
	h.mu.RLock()
	room := h.rooms[roomID]
	h.mu.RUnlock()

	if room == nil {
		return []ChatMessage{}
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	result := make([]ChatMessage, len(room.ChatHistory))
	copy(result, room.ChatHistory)
	return result
}
