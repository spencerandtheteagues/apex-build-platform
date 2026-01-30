// APEX.BUILD Presence System
// Real-time cursor, selection, and activity tracking

package collaboration

import (
	"encoding/json"
	"sync"
	"time"
)

// UserPresence represents a user's real-time presence in a collaboration room
type UserPresence struct {
	UserID        uint              `json:"user_id"`
	Username      string            `json:"username"`
	Email         string            `json:"email"`
	AvatarURL     string            `json:"avatar_url,omitempty"`
	Color         string            `json:"color"`
	FileID        uint              `json:"file_id,omitempty"`
	FileName      string            `json:"file_name,omitempty"`
	Cursor        *CursorInfo       `json:"cursor,omitempty"`
	Selection     *SelectionInfo    `json:"selection,omitempty"`
	IsTyping      bool              `json:"is_typing"`
	LastActivity  time.Time         `json:"last_activity"`
	IsFollowing   uint              `json:"is_following,omitempty"` // UserID being followed
	Permission    PermissionLevel   `json:"permission"`
	Status        PresenceStatus    `json:"status"`
}

// CursorInfo represents a user's cursor position
type CursorInfo struct {
	Line      int       `json:"line"`
	Column    int       `json:"column"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SelectionInfo represents a user's text selection
type SelectionInfo struct {
	StartLine   int       `json:"start_line"`
	StartColumn int       `json:"start_column"`
	EndLine     int       `json:"end_line"`
	EndColumn   int       `json:"end_column"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PresenceStatus represents a user's online status
type PresenceStatus string

const (
	StatusOnline  PresenceStatus = "online"
	StatusAway    PresenceStatus = "away"
	StatusBusy    PresenceStatus = "busy"
	StatusOffline PresenceStatus = "offline"
)

// PermissionLevel represents user permissions in a room
type PermissionLevel string

const (
	PermissionViewer PermissionLevel = "viewer"  // Read-only
	PermissionEditor PermissionLevel = "editor"  // Can edit
	PermissionAdmin  PermissionLevel = "admin"   // Full control
	PermissionOwner  PermissionLevel = "owner"   // Project owner
)

// Predefined cursor colors for collaboration
var CursorColors = []string{
	"#FF6B6B", // Red
	"#4ECDC4", // Teal
	"#45B7D1", // Blue
	"#96CEB4", // Green
	"#FFEAA7", // Yellow
	"#DDA0DD", // Plum
	"#98D8C8", // Mint
	"#F7DC6F", // Gold
	"#BB8FCE", // Purple
	"#85C1E9", // Light Blue
	"#F8B500", // Orange
	"#FF69B4", // Hot Pink
}

// PresenceManager manages user presence across rooms
type PresenceManager struct {
	// Map of roomID -> userID -> presence
	rooms      map[string]map[uint]*UserPresence
	colorIndex map[string]int // Track color assignment per room
	mu         sync.RWMutex
}

// NewPresenceManager creates a new presence manager
func NewPresenceManager() *PresenceManager {
	return &PresenceManager{
		rooms:      make(map[string]map[uint]*UserPresence),
		colorIndex: make(map[string]int),
	}
}

// JoinRoom adds a user to a room's presence list
func (pm *PresenceManager) JoinRoom(roomID string, userID uint, username, email, avatarURL string, permission PermissionLevel) *UserPresence {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.rooms[roomID] == nil {
		pm.rooms[roomID] = make(map[uint]*UserPresence)
	}

	// Assign a unique color
	colorIdx := pm.colorIndex[roomID] % len(CursorColors)
	pm.colorIndex[roomID]++

	presence := &UserPresence{
		UserID:       userID,
		Username:     username,
		Email:        email,
		AvatarURL:    avatarURL,
		Color:        CursorColors[colorIdx],
		LastActivity: time.Now(),
		Permission:   permission,
		Status:       StatusOnline,
		IsTyping:     false,
	}

	pm.rooms[roomID][userID] = presence
	return presence
}

// LeaveRoom removes a user from a room's presence list
func (pm *PresenceManager) LeaveRoom(roomID string, userID uint) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.rooms[roomID] != nil {
		delete(pm.rooms[roomID], userID)
		if len(pm.rooms[roomID]) == 0 {
			delete(pm.rooms, roomID)
			delete(pm.colorIndex, roomID)
		}
	}
}

// UpdateCursor updates a user's cursor position
func (pm *PresenceManager) UpdateCursor(roomID string, userID uint, fileID uint, fileName string, line, column int) *UserPresence {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.rooms[roomID] == nil || pm.rooms[roomID][userID] == nil {
		return nil
	}

	presence := pm.rooms[roomID][userID]
	presence.FileID = fileID
	presence.FileName = fileName
	presence.Cursor = &CursorInfo{
		Line:      line,
		Column:    column,
		UpdatedAt: time.Now(),
	}
	presence.LastActivity = time.Now()
	presence.Status = StatusOnline

	return presence
}

// UpdateSelection updates a user's text selection
func (pm *PresenceManager) UpdateSelection(roomID string, userID uint, startLine, startCol, endLine, endCol int) *UserPresence {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.rooms[roomID] == nil || pm.rooms[roomID][userID] == nil {
		return nil
	}

	presence := pm.rooms[roomID][userID]
	presence.Selection = &SelectionInfo{
		StartLine:   startLine,
		StartColumn: startCol,
		EndLine:     endLine,
		EndColumn:   endCol,
		UpdatedAt:   time.Now(),
	}
	presence.LastActivity = time.Now()

	return presence
}

// ClearSelection clears a user's selection
func (pm *PresenceManager) ClearSelection(roomID string, userID uint) *UserPresence {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.rooms[roomID] == nil || pm.rooms[roomID][userID] == nil {
		return nil
	}

	presence := pm.rooms[roomID][userID]
	presence.Selection = nil
	presence.LastActivity = time.Now()

	return presence
}

// SetTyping updates a user's typing status
func (pm *PresenceManager) SetTyping(roomID string, userID uint, isTyping bool) *UserPresence {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.rooms[roomID] == nil || pm.rooms[roomID][userID] == nil {
		return nil
	}

	presence := pm.rooms[roomID][userID]
	presence.IsTyping = isTyping
	presence.LastActivity = time.Now()

	return presence
}

// SetFollowing sets who a user is following
func (pm *PresenceManager) SetFollowing(roomID string, userID, targetUserID uint) *UserPresence {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.rooms[roomID] == nil || pm.rooms[roomID][userID] == nil {
		return nil
	}

	presence := pm.rooms[roomID][userID]
	presence.IsFollowing = targetUserID
	presence.LastActivity = time.Now()

	return presence
}

// StopFollowing stops a user from following another
func (pm *PresenceManager) StopFollowing(roomID string, userID uint) *UserPresence {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.rooms[roomID] == nil || pm.rooms[roomID][userID] == nil {
		return nil
	}

	presence := pm.rooms[roomID][userID]
	presence.IsFollowing = 0
	presence.LastActivity = time.Now()

	return presence
}

// SetStatus updates a user's presence status
func (pm *PresenceManager) SetStatus(roomID string, userID uint, status PresenceStatus) *UserPresence {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.rooms[roomID] == nil || pm.rooms[roomID][userID] == nil {
		return nil
	}

	presence := pm.rooms[roomID][userID]
	presence.Status = status
	presence.LastActivity = time.Now()

	return presence
}

// SetPermission updates a user's permission level
func (pm *PresenceManager) SetPermission(roomID string, userID uint, permission PermissionLevel) *UserPresence {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.rooms[roomID] == nil || pm.rooms[roomID][userID] == nil {
		return nil
	}

	presence := pm.rooms[roomID][userID]
	presence.Permission = permission

	return presence
}

// GetRoomPresence returns all presence information for a room
func (pm *PresenceManager) GetRoomPresence(roomID string) []*UserPresence {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.rooms[roomID] == nil {
		return []*UserPresence{}
	}

	result := make([]*UserPresence, 0, len(pm.rooms[roomID]))
	for _, presence := range pm.rooms[roomID] {
		result = append(result, presence)
	}

	return result
}

// GetUserPresence returns presence information for a specific user
func (pm *PresenceManager) GetUserPresence(roomID string, userID uint) *UserPresence {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.rooms[roomID] == nil {
		return nil
	}

	return pm.rooms[roomID][userID]
}

// GetFollowers returns users following a specific user
func (pm *PresenceManager) GetFollowers(roomID string, userID uint) []*UserPresence {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.rooms[roomID] == nil {
		return []*UserPresence{}
	}

	result := make([]*UserPresence, 0)
	for _, presence := range pm.rooms[roomID] {
		if presence.IsFollowing == userID {
			result = append(result, presence)
		}
	}

	return result
}

// CanEdit checks if a user has edit permissions
func (pm *PresenceManager) CanEdit(roomID string, userID uint) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.rooms[roomID] == nil || pm.rooms[roomID][userID] == nil {
		return false
	}

	permission := pm.rooms[roomID][userID].Permission
	return permission == PermissionEditor || permission == PermissionAdmin || permission == PermissionOwner
}

// IsAdmin checks if a user has admin permissions
func (pm *PresenceManager) IsAdmin(roomID string, userID uint) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.rooms[roomID] == nil || pm.rooms[roomID][userID] == nil {
		return false
	}

	permission := pm.rooms[roomID][userID].Permission
	return permission == PermissionAdmin || permission == PermissionOwner
}

// KickUser removes a user from a room (admin action)
func (pm *PresenceManager) KickUser(roomID string, adminUserID, targetUserID uint) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.rooms[roomID] == nil {
		return false
	}

	// Check admin permission
	adminPresence := pm.rooms[roomID][adminUserID]
	if adminPresence == nil {
		return false
	}

	if adminPresence.Permission != PermissionAdmin && adminPresence.Permission != PermissionOwner {
		return false
	}

	// Cannot kick owner
	targetPresence := pm.rooms[roomID][targetUserID]
	if targetPresence != nil && targetPresence.Permission == PermissionOwner {
		return false
	}

	delete(pm.rooms[roomID], targetUserID)
	return true
}

// CleanupInactive removes users who have been inactive for too long
func (pm *PresenceManager) CleanupInactive(timeout time.Duration) []struct {
	RoomID string
	UserID uint
} {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	removed := make([]struct {
		RoomID string
		UserID uint
	}, 0)

	for roomID, users := range pm.rooms {
		for userID, presence := range users {
			if now.Sub(presence.LastActivity) > timeout {
				delete(users, userID)
				removed = append(removed, struct {
					RoomID string
					UserID uint
				}{roomID, userID})
			}
		}

		// Clean up empty rooms
		if len(users) == 0 {
			delete(pm.rooms, roomID)
			delete(pm.colorIndex, roomID)
		}
	}

	return removed
}

// ToJSON serializes presence to JSON
func (p *UserPresence) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// PresenceUpdate represents a presence change message
type PresenceUpdate struct {
	Type      string        `json:"type"` // cursor_update, selection_update, typing, follow, status
	RoomID    string        `json:"room_id"`
	UserID    uint          `json:"user_id"`
	Presence  *UserPresence `json:"presence"`
	Timestamp time.Time     `json:"timestamp"`
}

// ActivityFeedItem represents an item in the activity feed
type ActivityFeedItem struct {
	ID        string    `json:"id"`
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Action    string    `json:"action"` // joined, left, edited, commented, etc.
	Target    string    `json:"target"` // File name, etc.
	Timestamp time.Time `json:"timestamp"`
}

// ActivityFeed tracks recent activities in a room
type ActivityFeed struct {
	roomID   string
	items    []ActivityFeedItem
	maxItems int
	mu       sync.RWMutex
}

// NewActivityFeed creates a new activity feed
func NewActivityFeed(roomID string, maxItems int) *ActivityFeed {
	return &ActivityFeed{
		roomID:   roomID,
		items:    make([]ActivityFeedItem, 0, maxItems),
		maxItems: maxItems,
	}
}

// AddActivity adds an activity to the feed
func (af *ActivityFeed) AddActivity(item ActivityFeedItem) {
	af.mu.Lock()
	defer af.mu.Unlock()

	af.items = append(af.items, item)
	if len(af.items) > af.maxItems {
		af.items = af.items[len(af.items)-af.maxItems:]
	}
}

// GetRecentActivities returns recent activities
func (af *ActivityFeed) GetRecentActivities(limit int) []ActivityFeedItem {
	af.mu.RLock()
	defer af.mu.RUnlock()

	if limit > len(af.items) {
		limit = len(af.items)
	}

	result := make([]ActivityFeedItem, limit)
	copy(result, af.items[len(af.items)-limit:])

	return result
}
