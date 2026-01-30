package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a user in the APEX.BUILD platform
type User struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Basic user information
	Username     string `json:"username" gorm:"uniqueIndex;not null"`
	Email        string `json:"email" gorm:"uniqueIndex;not null"`
	PasswordHash string `json:"-" gorm:"not null"`
	FullName     string `json:"full_name"`
	AvatarURL    string `json:"avatar_url"`

	// Account status
	IsActive   bool `json:"is_active" gorm:"default:true"`
	IsVerified bool `json:"is_verified" gorm:"default:false"`

	// Admin and special privileges
	IsAdmin           bool `json:"is_admin" gorm:"default:false"`
	IsSuperAdmin      bool `json:"is_super_admin" gorm:"default:false"`
	HasUnlimitedCredits bool `json:"has_unlimited_credits" gorm:"default:false"`
	BypassBilling     bool `json:"bypass_billing" gorm:"default:false"`
	BypassRateLimits  bool `json:"bypass_rate_limits" gorm:"default:false"`

	// Subscription and billing (Stripe integration)
	StripeCustomerID   string    `json:"stripe_customer_id" gorm:"index"`
	SubscriptionID     string    `json:"subscription_id" gorm:"index"`
	SubscriptionStatus string    `json:"subscription_status" gorm:"default:'inactive'"` // active, canceled, past_due, trialing, inactive
	SubscriptionType   string    `json:"subscription_type" gorm:"default:'free'"`       // free, pro, team, enterprise
	SubscriptionEnd    time.Time `json:"subscription_end"`
	BillingCycleStart  time.Time `json:"billing_cycle_start"`

	// Usage tracking for AI services
	MonthlyAIRequests int     `json:"monthly_ai_requests" gorm:"default:0"`
	MonthlyAICost     float64 `json:"monthly_ai_cost" gorm:"default:0.0"`
	CreditBalance     float64 `json:"credit_balance" gorm:"default:0.0"`

	// Preferences
	PreferredTheme string `json:"preferred_theme" gorm:"default:'cyberpunk'"` // cyberpunk, matrix, synthwave, neonCity
	PreferredAI    string `json:"preferred_ai" gorm:"default:'auto'"`         // auto, claude, gpt4, gemini

	// Relationships
	Projects    []Project    `json:"projects" gorm:"foreignKey:OwnerID"`
	Sessions    []Session    `json:"sessions" gorm:"foreignKey:UserID"`
	AIRequests  []AIRequest  `json:"ai_requests" gorm:"foreignKey:UserID"`
	CollabRooms []CollabRoom `json:"collab_rooms" gorm:"many2many:user_collab_rooms;"`
}

// Project represents a coding project
type Project struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Basic project information
	Name        string `json:"name" gorm:"not null"`
	Description string `json:"description"`
	Language    string `json:"language" gorm:"not null"` // javascript, python, go, rust, etc.
	Framework   string `json:"framework"`                // react, next, django, gin, etc.

	// Project ownership and access
	OwnerID   uint `json:"owner_id" gorm:"not null"`
	Owner     User `json:"owner" gorm:"foreignKey:OwnerID"`
	IsPublic  bool `json:"is_public" gorm:"default:false"`
	IsArchived bool `json:"is_archived" gorm:"default:false"`

	// Project structure
	RootDirectory string `json:"root_directory" gorm:"default:'/'"` // File system root path
	EntryPoint    string `json:"entry_point"`                       // Main file (main.go, index.js, etc.)

	// Runtime configuration
	Environment map[string]interface{} `json:"environment" gorm:"serializer:json"` // Environment variables
	Dependencies map[string]interface{} `json:"dependencies" gorm:"serializer:json"` // Package.json, go.mod equivalent
	BuildConfig map[string]interface{} `json:"build_config" gorm:"serializer:json"` // Build and run configuration

	// Collaboration
	CollabRoomID *uint `json:"collab_room_id"`

	// Relationships
	Files       []File       `json:"files" gorm:"foreignKey:ProjectID"`
	Executions  []Execution  `json:"executions" gorm:"foreignKey:ProjectID"`
	AIRequests  []AIRequest  `json:"ai_requests" gorm:"foreignKey:ProjectID"`
}

// File represents a file within a project
type File struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// File identification
	ProjectID uint    `json:"project_id" gorm:"not null"`
	Project   Project `json:"project" gorm:"foreignKey:ProjectID"`
	Path      string  `json:"path" gorm:"not null"`      // Relative path from project root
	Name      string  `json:"name" gorm:"not null"`      // File name with extension
	Type      string  `json:"type" gorm:"not null"`      // file, directory
	MimeType  string  `json:"mime_type"`                 // application/javascript, text/plain, etc.

	// File content
	Content string `json:"content" gorm:"type:text"`  // File contents
	Size    int64  `json:"size" gorm:"default:0"`     // File size in bytes
	Hash    string `json:"hash"`                      // SHA-256 hash for change detection

	// Versioning
	Version   int       `json:"version" gorm:"default:1"`
	LastEditBy uint     `json:"last_edit_by"`
	LastEditor User     `json:"last_editor" gorm:"foreignKey:LastEditBy"`

	// File status
	IsLocked bool `json:"is_locked" gorm:"default:false"`
	LockedBy *uint `json:"locked_by"`
	LockedAt *time.Time `json:"locked_at"`
}

// Session represents a user's active session
type Session struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Session identification
	UserID    uint   `json:"user_id" gorm:"not null"`
	User      User   `json:"user" gorm:"foreignKey:UserID"`
	SessionID string `json:"session_id" gorm:"uniqueIndex;not null"` // JWT token ID or session identifier
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`

	// Session state
	IsActive  bool       `json:"is_active" gorm:"default:true"`
	ExpiresAt time.Time  `json:"expires_at"`
	LastSeen  time.Time  `json:"last_seen"`

	// Current context
	CurrentProjectID *uint    `json:"current_project_id"`
	CurrentProject   *Project `json:"current_project" gorm:"foreignKey:CurrentProjectID"`
}

// AIRequest represents a request to an AI service
type AIRequest struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Request identification
	RequestID string `json:"request_id" gorm:"uniqueIndex;not null"` // UUID for tracking
	UserID    uint   `json:"user_id" gorm:"not null"`
	User      User   `json:"user" gorm:"foreignKey:UserID"`
	ProjectID *uint  `json:"project_id"`
	Project   *Project `json:"project" gorm:"foreignKey:ProjectID"`

	// AI request details
	Provider   string `json:"provider" gorm:"not null"`    // claude, gpt4, gemini
	Capability string `json:"capability" gorm:"not null"`  // code_generation, code_review, etc.
	Prompt     string `json:"prompt" gorm:"type:text"`     // User's prompt
	Code       string `json:"code" gorm:"type:text"`       // Code context if provided
	Language   string `json:"language"`                    // Programming language
	Context    map[string]interface{} `json:"context" gorm:"serializer:json"` // Additional context

	// AI response
	Response    string  `json:"response" gorm:"type:text"`    // AI's response
	TokensUsed  int     `json:"tokens_used" gorm:"default:0"` // Total tokens consumed
	Cost        float64 `json:"cost" gorm:"default:0.0"`      // Cost in USD
	Duration    int64   `json:"duration" gorm:"default:0"`    // Response time in milliseconds

	// Request status
	Status    string `json:"status" gorm:"default:'pending'"` // pending, completed, failed
	ErrorMsg  string `json:"error_msg"`                       // Error message if failed

	// Quality feedback
	UserRating   *int    `json:"user_rating"`   // 1-5 rating from user
	UserFeedback *string `json:"user_feedback"` // Text feedback from user
}

// Execution represents a code execution instance
type Execution struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Execution identification
	ExecutionID string  `json:"execution_id" gorm:"uniqueIndex;not null"` // UUID
	ProjectID   uint    `json:"project_id" gorm:"not null"`
	Project     Project `json:"project" gorm:"foreignKey:ProjectID"`
	UserID      uint    `json:"user_id" gorm:"not null"`
	User        User    `json:"user" gorm:"foreignKey:UserID"`

	// Execution context
	Command     string `json:"command" gorm:"not null"`      // Command executed
	Language    string `json:"language" gorm:"not null"`     // Programming language
	Environment map[string]interface{} `json:"environment" gorm:"serializer:json"` // Environment variables
	Input       string `json:"input" gorm:"type:text"`       // Input provided to execution

	// Execution results
	Output    string `json:"output" gorm:"type:text"`       // Standard output
	ErrorOut  string `json:"error_out" gorm:"type:text"`    // Standard error
	ExitCode  int    `json:"exit_code" gorm:"default:0"`    // Exit code
	Duration  int64  `json:"duration" gorm:"default:0"`     // Execution time in milliseconds

	// Execution state
	Status      string     `json:"status" gorm:"default:'running'"` // running, completed, failed, timeout
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`

	// Resource usage
	MemoryUsed int64 `json:"memory_used" gorm:"default:0"` // Memory used in bytes
	CPUTime    int64 `json:"cpu_time" gorm:"default:0"`    // CPU time in milliseconds
}

// CollabRoom represents a real-time collaboration session
type CollabRoom struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Room identification
	RoomID    string   `json:"room_id" gorm:"uniqueIndex;not null"` // UUID for WebSocket connections
	ProjectID uint     `json:"project_id" gorm:"not null"`
	Project   *Project `json:"project" gorm:"foreignKey:ProjectID"`

	// Room state
	IsActive      bool `json:"is_active" gorm:"default:true"`
	MaxUsers      int  `json:"max_users" gorm:"default:10"`
	CurrentUsers  int  `json:"current_users" gorm:"default:0"`

	// Collaboration settings
	AllowAnonymous bool   `json:"allow_anonymous" gorm:"default:false"`
	RequireAuth    bool   `json:"require_auth" gorm:"default:true"`
	Password       string `json:"password"` // Optional room password

	// Relationships
	Users       []User           `json:"users" gorm:"many2many:user_collab_rooms;"`
	Cursors     []CursorPosition `json:"cursors" gorm:"foreignKey:RoomID"`
	ChatMessages []ChatMessage   `json:"chat_messages" gorm:"foreignKey:RoomID"`
}

// CursorPosition tracks user cursors in real-time collaboration
type CursorPosition struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Position tracking
	RoomID    uint `json:"room_id" gorm:"not null"`
	UserID    uint `json:"user_id" gorm:"not null"`
	User      User `json:"user" gorm:"foreignKey:UserID"`
	FileID    uint `json:"file_id" gorm:"not null"`
	File      File `json:"file" gorm:"foreignKey:FileID"`

	// Cursor coordinates
	Line   int `json:"line" gorm:"not null"`
	Column int `json:"column" gorm:"not null"`

	// Selection range (if any)
	SelectionStartLine   *int `json:"selection_start_line"`
	SelectionStartColumn *int `json:"selection_start_column"`
	SelectionEndLine     *int `json:"selection_end_line"`
	SelectionEndColumn   *int `json:"selection_end_column"`

	// Status
	IsActive   bool      `json:"is_active" gorm:"default:true"`
	LastActive time.Time `json:"last_active"`
}

// ChatMessage represents chat messages in collaboration rooms
type ChatMessage struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Message details
	RoomID    uint `json:"room_id" gorm:"not null"`
	UserID    uint `json:"user_id" gorm:"not null"`
	User      User `json:"user" gorm:"foreignKey:UserID"`
	Message   string     `json:"message" gorm:"not null"`
	Type      string     `json:"type" gorm:"default:'text'"` // text, system, code, file

	// Message status
	IsEdited bool       `json:"is_edited" gorm:"default:false"`
	EditedAt *time.Time `json:"edited_at"`
}

// FileVersion represents a historical version of a file (Replit parity feature)
// Enables version history with diff viewing and restore capabilities
type FileVersion struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// File relationship
	FileID    uint   `json:"file_id" gorm:"not null;index"`
	File      *File  `json:"file,omitempty" gorm:"foreignKey:FileID"`
	ProjectID uint   `json:"project_id" gorm:"not null;index"`

	// Version identification
	Version     int    `json:"version" gorm:"not null"`          // Sequential version number
	VersionHash string `json:"version_hash" gorm:"index"`        // SHA-256 hash for deduplication

	// Content snapshot
	Content   string `json:"content" gorm:"type:text"`           // Full file content at this version
	Size      int64  `json:"size" gorm:"default:0"`              // Content size in bytes
	LineCount int    `json:"line_count" gorm:"default:0"`        // Number of lines

	// Change metadata
	ChangeType    string `json:"change_type" gorm:"default:'edit'"` // create, edit, rename, restore
	ChangeSummary string `json:"change_summary"`                    // Brief description of changes
	LinesAdded    int    `json:"lines_added" gorm:"default:0"`      // Lines added from previous version
	LinesRemoved  int    `json:"lines_removed" gorm:"default:0"`    // Lines removed from previous version

	// Author information
	AuthorID   uint   `json:"author_id" gorm:"not null"`
	Author     *User  `json:"author,omitempty" gorm:"foreignKey:AuthorID"`
	AuthorName string `json:"author_name"`                        // Cached for display

	// File path at this version (captures renames)
	FilePath string `json:"file_path" gorm:"not null"`
	FileName string `json:"file_name" gorm:"not null"`

	// Retention flags
	IsPinned    bool `json:"is_pinned" gorm:"default:false"`     // Pinned versions are never auto-deleted
	IsAutoSave  bool `json:"is_auto_save" gorm:"default:false"`  // Auto-save vs manual save
}

// CodeComment represents an inline code comment for collaboration (Replit parity feature)
type CodeComment struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// File relationship
	FileID    uint    `json:"file_id" gorm:"not null;index"`
	File      *File   `json:"file,omitempty" gorm:"foreignKey:FileID"`
	ProjectID uint    `json:"project_id" gorm:"not null;index"`

	// Position in file
	StartLine   int `json:"start_line" gorm:"not null"`
	EndLine     int `json:"end_line" gorm:"not null"`
	StartColumn int `json:"start_column" gorm:"default:0"`
	EndColumn   int `json:"end_column" gorm:"default:0"`

	// Comment content
	Content string `json:"content" gorm:"type:text;not null"`

	// Thread management
	ParentID  *uint          `json:"parent_id" gorm:"index"`          // For replies
	Parent    *CodeComment   `json:"parent,omitempty" gorm:"foreignKey:ParentID"`
	Replies   []CodeComment  `json:"replies,omitempty" gorm:"foreignKey:ParentID"`
	ThreadID  string         `json:"thread_id" gorm:"index"`          // Groups comments in same thread

	// Author
	AuthorID   uint   `json:"author_id" gorm:"not null"`
	Author     *User  `json:"author,omitempty" gorm:"foreignKey:AuthorID"`
	AuthorName string `json:"author_name"`

	// Status
	IsResolved   bool       `json:"is_resolved" gorm:"default:false"`
	ResolvedAt   *time.Time `json:"resolved_at"`
	ResolvedByID *uint      `json:"resolved_by_id"`
	ResolvedBy   *User      `json:"resolved_by,omitempty" gorm:"foreignKey:ResolvedByID"`

	// Reactions (emoji reactions to comments)
	Reactions map[string][]uint `json:"reactions" gorm:"serializer:json"` // emoji -> user IDs
}

// UserCollabRoom represents the many-to-many relationship between users and collaboration rooms
type UserCollabRoom struct {
	UserID      uint      `json:"user_id" gorm:"primarykey"`
	CollabRoomID uint     `json:"collab_room_id" gorm:"primarykey"`
	JoinedAt    time.Time `json:"joined_at"`
	Role        string    `json:"role" gorm:"default:'member'"` // owner, admin, member, viewer
	IsActive    bool      `json:"is_active" gorm:"default:true"`
}