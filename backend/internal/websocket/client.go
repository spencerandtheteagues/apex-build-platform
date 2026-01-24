// APEX.BUILD WebSocket Client
// Individual client connection handling

package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 8192
)

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, messageData, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Parse the message
		var message Message
		if err := json.Unmarshal(messageData, &message); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			c.sendError("Invalid message format")
			continue
		}

		// Update client's last seen time
		c.mu.Lock()
		c.lastSeen = time.Now()
		c.mu.Unlock()

		// Handle the message based on type
		c.handleMessage(message)
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current WebSocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages from the client
func (c *Client) handleMessage(message Message) {
	// Set message metadata
	message.UserID = c.UserID
	message.Username = c.Username
	message.RoomID = c.RoomID
	message.Timestamp = time.Now()

	switch message.Type {
	case MessageTypeJoinRoom:
		c.handleJoinRoom(message)

	case MessageTypeLeaveRoom:
		c.handleLeaveRoom(message)

	case MessageTypeCursorUpdate:
		c.handleCursorUpdate(message)

	case MessageTypeFileChange:
		c.handleFileChange(message)

	case MessageTypeChat:
		c.handleChat(message)

	case MessageTypeHeartbeat:
		c.handleHeartbeat(message)

	default:
		c.sendError("Unknown message type: " + message.Type)
	}
}

// handleJoinRoom handles room join requests
func (c *Client) handleJoinRoom(message Message) {
	data, ok := message.Data.(map[string]interface{})
	if !ok {
		c.sendError("Invalid join room data")
		return
	}

	newRoomID, ok := data["room_id"].(string)
	if !ok {
		c.sendError("Room ID is required")
		return
	}

	// Leave current room if in one
	if c.RoomID != "" {
		c.handleLeaveRoom(Message{Type: MessageTypeLeaveRoom})
	}

	// Update client's room
	c.mu.Lock()
	c.RoomID = newRoomID
	if projectID, exists := data["project_id"]; exists {
		if pid, ok := projectID.(float64); ok {
			c.ProjectID = uint(pid)
		}
	}
	c.mu.Unlock()

	// Re-register client with new room
	c.hub.register <- c

	log.Printf("Client %d joined room %s", c.UserID, newRoomID)
}

// handleLeaveRoom handles room leave requests
func (c *Client) handleLeaveRoom(message Message) {
	if c.RoomID == "" {
		return
	}

	// Notify other clients in the room
	c.hub.broadcastToRoom(c.RoomID, Message{
		Type:      MessageTypeUserLeft,
		UserID:    c.UserID,
		Username:  c.Username,
		RoomID:    c.RoomID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"user_id":  c.UserID,
			"username": c.Username,
		},
	}, c)

	// Remove from room
	c.hub.mu.Lock()
	if c.hub.rooms[c.RoomID] != nil {
		delete(c.hub.rooms[c.RoomID], c)
		if len(c.hub.rooms[c.RoomID]) == 0 {
			delete(c.hub.rooms, c.RoomID)
		}
	}
	c.hub.mu.Unlock()

	// Clear client's room
	c.mu.Lock()
	c.RoomID = ""
	c.ProjectID = 0
	c.mu.Unlock()

	log.Printf("Client %d left room", c.UserID)
}

// handleCursorUpdate handles cursor position updates
func (c *Client) handleCursorUpdate(message Message) {
	data, ok := message.Data.(map[string]interface{})
	if !ok {
		c.sendError("Invalid cursor update data")
		return
	}

	fileID, ok := data["file_id"].(float64)
	if !ok {
		c.sendError("File ID is required for cursor update")
		return
	}

	line, ok := data["line"].(float64)
	if !ok {
		c.sendError("Line is required for cursor update")
		return
	}

	column, ok := data["column"].(float64)
	if !ok {
		c.sendError("Column is required for cursor update")
		return
	}

	// Update client's cursor position
	c.mu.Lock()
	c.cursorPosition = &CursorPosition{
		FileID:    uint(fileID),
		Line:      int(line),
		Column:    int(column),
		UpdatedAt: time.Now(),
	}
	c.mu.Unlock()

	// Broadcast cursor update to other clients in the room
	c.hub.broadcastToRoom(c.RoomID, message, c)

	log.Printf("Cursor update from client %d: file=%d, line=%d, column=%d",
		c.UserID, uint(fileID), int(line), int(column))
}

// handleFileChange handles file content changes
func (c *Client) handleFileChange(message Message) {
	data, ok := message.Data.(map[string]interface{})
	if !ok {
		c.sendError("Invalid file change data")
		return
	}

	fileID, ok := data["file_id"].(float64)
	if !ok {
		c.sendError("File ID is required for file change")
		return
	}

	changes, ok := data["changes"]
	if !ok {
		c.sendError("Changes are required for file change")
		return
	}

	// Validate change data structure
	if !c.validateFileChanges(changes) {
		c.sendError("Invalid file change format")
		return
	}

	// Broadcast file changes to other clients in the room
	c.hub.broadcastToRoom(c.RoomID, message, c)

	log.Printf("File change from client %d: file=%d", c.UserID, uint(fileID))
}

// handleChat handles chat messages
func (c *Client) handleChat(message Message) {
	data, ok := message.Data.(map[string]interface{})
	if !ok {
		c.sendError("Invalid chat data")
		return
	}

	chatMessage, ok := data["message"].(string)
	if !ok || chatMessage == "" {
		c.sendError("Chat message is required")
		return
	}

	// Validate message length
	if len(chatMessage) > 1000 {
		c.sendError("Chat message too long (max 1000 characters)")
		return
	}

	// Add timestamp and user info to the message
	message.Data = map[string]interface{}{
		"message":   chatMessage,
		"user_id":   c.UserID,
		"username":  c.Username,
		"timestamp": time.Now(),
	}

	// Broadcast chat message to all clients in the room
	c.hub.broadcastToRoom(c.RoomID, message, nil)

	log.Printf("Chat message from client %d in room %s: %s", c.UserID, c.RoomID, chatMessage)
}

// handleHeartbeat handles heartbeat/ping messages
func (c *Client) handleHeartbeat(message Message) {
	// Update last seen time
	c.mu.Lock()
	c.lastSeen = time.Now()
	c.mu.Unlock()

	// Send heartbeat response
	response := Message{
		Type:      MessageTypeHeartbeat,
		UserID:    c.UserID,
		RoomID:    c.RoomID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"pong": true,
		},
	}

	responseData, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling heartbeat response: %v", err)
		return
	}

	select {
	case c.send <- responseData:
	default:
		// Channel full, skip heartbeat response
	}
}

// sendError sends an error message to the client
func (c *Client) sendError(errorMessage string) {
	message := Message{
		Type:      MessageTypeError,
		UserID:    c.UserID,
		RoomID:    c.RoomID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"error": errorMessage,
		},
	}

	messageData, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling error message: %v", err)
		return
	}

	select {
	case c.send <- messageData:
	default:
		// Channel full, skip error message
		log.Printf("Failed to send error message to client %d: %s", c.UserID, errorMessage)
	}
}

// validateFileChanges validates the structure of file change data
func (c *Client) validateFileChanges(changes interface{}) bool {
	changesArray, ok := changes.([]interface{})
	if !ok {
		return false
	}

	for _, change := range changesArray {
		changeMap, ok := change.(map[string]interface{})
		if !ok {
			return false
		}

		// Check required fields for file changes
		requiredFields := []string{"type", "range", "text"}
		for _, field := range requiredFields {
			if _, exists := changeMap[field]; !exists {
				return false
			}
		}

		// Validate range structure
		rangeData, ok := changeMap["range"].(map[string]interface{})
		if !ok {
			return false
		}

		rangeFields := []string{"start", "end"}
		for _, field := range rangeFields {
			position, ok := rangeData[field].(map[string]interface{})
			if !ok {
				return false
			}

			// Check line and character exist
			if _, ok := position["line"].(float64); !ok {
				return false
			}
			if _, ok := position["character"].(float64); !ok {
				return false
			}
		}
	}

	return true
}

// GetUserInfo returns the client's user information
func (c *Client) GetUserInfo() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"user_id":         c.UserID,
		"username":        c.Username,
		"email":          c.Email,
		"room_id":        c.RoomID,
		"project_id":     c.ProjectID,
		"cursor_position": c.cursorPosition,
		"last_seen":      c.lastSeen,
	}
}

// IsInRoom checks if the client is in a specific room
func (c *Client) IsInRoom(roomID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.RoomID == roomID
}