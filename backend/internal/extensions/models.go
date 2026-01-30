// APEX.BUILD Extensions Marketplace Models
// Complete extension system with manifest, permissions, and lifecycle management

package extensions

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ExtensionStatus represents the approval status of an extension
type ExtensionStatus string

const (
	StatusPending  ExtensionStatus = "pending"
	StatusApproved ExtensionStatus = "approved"
	StatusRejected ExtensionStatus = "rejected"
	StatusDisabled ExtensionStatus = "disabled"
)

// ExtensionCategory represents the type of extension
type ExtensionCategory string

const (
	CategoryTheme      ExtensionCategory = "theme"
	CategoryLanguage   ExtensionCategory = "language"
	CategoryFormatter  ExtensionCategory = "formatter"
	CategoryLinter     ExtensionCategory = "linter"
	CategorySnippets   ExtensionCategory = "snippets"
	CategoryKeybinding ExtensionCategory = "keybinding"
	CategoryWidget     ExtensionCategory = "widget"
	CategoryAI         ExtensionCategory = "ai"
	CategoryDebugger   ExtensionCategory = "debugger"
	CategoryOther      ExtensionCategory = "other"
)

// ExtensionPermission represents permissions an extension can request
type ExtensionPermission string

const (
	PermissionFileRead     ExtensionPermission = "file:read"
	PermissionFileWrite    ExtensionPermission = "file:write"
	PermissionTerminal     ExtensionPermission = "terminal"
	PermissionNetwork      ExtensionPermission = "network"
	PermissionStorage      ExtensionPermission = "storage"
	PermissionClipboard    ExtensionPermission = "clipboard"
	PermissionNotification ExtensionPermission = "notification"
	PermissionAI           ExtensionPermission = "ai"
	PermissionSecrets      ExtensionPermission = "secrets"
	PermissionGit          ExtensionPermission = "git"
)

// Extension represents a marketplace extension
type Extension struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Basic information
	Name        string `json:"name" gorm:"uniqueIndex;not null;size:100"`
	DisplayName string `json:"display_name" gorm:"not null;size:200"`
	Author      string `json:"author" gorm:"not null;size:100"`
	AuthorID    uint   `json:"author_id" gorm:"index"`
	Description string `json:"description" gorm:"type:text"`
	Version     string `json:"version" gorm:"not null;size:20"`
	License     string `json:"license" gorm:"size:50"`
	Repository  string `json:"repository" gorm:"size:500"`
	Homepage    string `json:"homepage" gorm:"size:500"`

	// Categorization
	Category ExtensionCategory `json:"category" gorm:"type:varchar(50);index;default:'other'"`
	Tags     string            `json:"tags" gorm:"type:text"` // JSON array of tags

	// Assets
	IconURL       string `json:"icon_url" gorm:"size:500"`
	BannerURL     string `json:"banner_url" gorm:"size:500"`
	Screenshots   string `json:"screenshots" gorm:"type:text"` // JSON array of screenshot URLs
	SourceURL     string `json:"source_url" gorm:"size:500"`   // URL to extension bundle
	ReadmeContent string `json:"readme_content" gorm:"type:text"`
	Changelog     string `json:"changelog" gorm:"type:text"`

	// Extension manifest (permissions, entry points, etc.)
	Manifest string `json:"manifest" gorm:"type:text;not null"` // JSON manifest

	// Statistics
	Downloads       int     `json:"downloads" gorm:"default:0;index"`
	Rating          float64 `json:"rating" gorm:"type:decimal(3,2);default:0.00"`
	RatingCount     int     `json:"rating_count" gorm:"default:0"`
	WeeklyDownloads int     `json:"weekly_downloads" gorm:"default:0"`

	// Status and moderation
	Status       ExtensionStatus `json:"status" gorm:"type:varchar(20);default:'pending';index"`
	ReviewedAt   *time.Time      `json:"reviewed_at"`
	ReviewedBy   *uint           `json:"reviewed_by"`
	ReviewNotes  string          `json:"review_notes" gorm:"type:text"`
	IsFeatured   bool            `json:"is_featured" gorm:"default:false;index"`
	IsVerified   bool            `json:"is_verified" gorm:"default:false"` // Verified publisher
	IsDeprecated bool            `json:"is_deprecated" gorm:"default:false"`

	// Compatibility
	MinPlatformVersion string `json:"min_platform_version" gorm:"size:20"`
	MaxPlatformVersion string `json:"max_platform_version" gorm:"size:20"`

	// Relationships
	Versions     []ExtensionVersion `json:"versions,omitempty" gorm:"foreignKey:ExtensionID"`
	Reviews      []ExtensionReview  `json:"reviews,omitempty" gorm:"foreignKey:ExtensionID"`
	Dependencies []ExtensionDep     `json:"dependencies,omitempty" gorm:"foreignKey:ExtensionID"`
}

// ExtensionManifest represents the parsed manifest.json of an extension
type ExtensionManifest struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
	License     string `json:"license"`
	Repository  string `json:"repository"`
	Icon        string `json:"icon"`

	// Entry points
	Main       string `json:"main"`       // Main JavaScript entry point
	Browser    string `json:"browser"`    // Browser-specific entry point
	Stylesheet string `json:"stylesheet"` // CSS entry point

	// Activation
	ActivationEvents []string `json:"activationEvents"` // Events that activate the extension

	// Extension points (what the extension provides)
	Contributes *ExtensionContributes `json:"contributes"`

	// Required permissions
	Permissions []ExtensionPermission `json:"permissions"`

	// Extension points this extension uses
	ExtensionDependencies []string `json:"extensionDependencies"`

	// Platform requirements
	Engines map[string]string `json:"engines"` // e.g., {"apex": "^1.0.0"}

	// Categories and keywords
	Categories []string `json:"categories"`
	Keywords   []string `json:"keywords"`
}

// ExtensionContributes defines what an extension contributes to the platform
type ExtensionContributes struct {
	// Editor themes
	Themes []ThemeContribution `json:"themes"`

	// Language support
	Languages  []LanguageContribution  `json:"languages"`
	Grammars   []GrammarContribution   `json:"grammars"`
	Snippets   []SnippetContribution   `json:"snippets"`

	// Commands and keybindings
	Commands    []CommandContribution    `json:"commands"`
	Keybindings []KeybindingContribution `json:"keybindings"`
	Menus       map[string][]MenuItem    `json:"menus"`

	// UI contributions
	Views           []ViewContribution      `json:"views"`
	ViewsContainers []ViewContainerContrib  `json:"viewsContainers"`
	Panels          []PanelContribution     `json:"panels"`
	StatusBar       []StatusBarContribution `json:"statusBar"`

	// Configuration
	Configuration []ConfigurationContrib `json:"configuration"`

	// Formatters and linters
	Formatters []FormatterContribution `json:"formatters"`
	Linters    []LinterContribution    `json:"linters"`

	// Debuggers
	Debuggers []DebuggerContribution `json:"debuggers"`
}

// ThemeContribution represents a theme provided by an extension
type ThemeContribution struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	UITheme     string `json:"uiTheme"` // vs, vs-dark, hc-black
	Path        string `json:"path"`    // Path to theme JSON
	Description string `json:"description"`
}

// LanguageContribution represents language support provided by an extension
type LanguageContribution struct {
	ID            string   `json:"id"`
	Aliases       []string `json:"aliases"`
	Extensions    []string `json:"extensions"`
	Filenames     []string `json:"filenames"`
	Configuration string   `json:"configuration"` // Path to language config
	Icon          string   `json:"icon"`
}

// GrammarContribution represents syntax highlighting grammar
type GrammarContribution struct {
	Language   string `json:"language"`
	ScopeName  string `json:"scopeName"`
	Path       string `json:"path"`
	EmbeddedIn []string `json:"embeddedLanguages"`
}

// SnippetContribution represents code snippets
type SnippetContribution struct {
	Language string `json:"language"`
	Path     string `json:"path"`
}

// CommandContribution represents a command provided by an extension
type CommandContribution struct {
	Command     string `json:"command"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

// KeybindingContribution represents a keyboard shortcut
type KeybindingContribution struct {
	Command string `json:"command"`
	Key     string `json:"key"`
	Mac     string `json:"mac"`
	When    string `json:"when"` // Context condition
}

// MenuItem represents a menu item contribution
type MenuItem struct {
	Command string `json:"command"`
	Group   string `json:"group"`
	When    string `json:"when"`
}

// ViewContribution represents a view (sidebar panel)
type ViewContribution struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // tree, webview
	When     string `json:"when"`
	ContextualTitle string `json:"contextualTitle"`
}

// ViewContainerContrib represents a view container (sidebar section)
type ViewContainerContrib struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Icon  string `json:"icon"`
}

// PanelContribution represents a bottom panel contribution
type PanelContribution struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Icon     string `json:"icon"`
	Priority int    `json:"priority"`
}

// StatusBarContribution represents a status bar item
type StatusBarContribution struct {
	ID        string `json:"id"`
	Alignment string `json:"alignment"` // left, right
	Priority  int    `json:"priority"`
	Command   string `json:"command"`
	Text      string `json:"text"`
	Tooltip   string `json:"tooltip"`
}

// ConfigurationContrib represents configuration options
type ConfigurationContrib struct {
	Title      string                       `json:"title"`
	Properties map[string]ConfigurationProp `json:"properties"`
}

// ConfigurationProp represents a single configuration property
type ConfigurationProp struct {
	Type             string        `json:"type"`
	Default          interface{}   `json:"default"`
	Description      string        `json:"description"`
	Enum             []interface{} `json:"enum"`
	EnumDescriptions []string      `json:"enumDescriptions"`
	Minimum          *float64      `json:"minimum"`
	Maximum          *float64      `json:"maximum"`
}

// FormatterContribution represents a code formatter
type FormatterContribution struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	Languages   []string `json:"languages"`
	Command     string   `json:"command"`
}

// LinterContribution represents a code linter
type LinterContribution struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	Languages   []string `json:"languages"`
	Command     string   `json:"command"`
}

// DebuggerContribution represents a debugger
type DebuggerContribution struct {
	Type        string   `json:"type"`
	Label       string   `json:"label"`
	Languages   []string `json:"languages"`
	Program     string   `json:"program"`
	Runtime     string   `json:"runtime"`
}

// ExtensionVersion represents a specific version of an extension
type ExtensionVersion struct {
	ID          uint           `json:"id" gorm:"primarykey"`
	CreatedAt   time.Time      `json:"created_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	ExtensionID  uint   `json:"extension_id" gorm:"index;not null"`
	Version      string `json:"version" gorm:"not null;size:20"`
	Changelog    string `json:"changelog" gorm:"type:text"`
	Manifest     string `json:"manifest" gorm:"type:text;not null"`
	SourceURL    string `json:"source_url" gorm:"size:500"`
	Downloads    int    `json:"downloads" gorm:"default:0"`
	IsPrerelease bool   `json:"is_prerelease" gorm:"default:false"`
	IsYanked     bool   `json:"is_yanked" gorm:"default:false"` // Removed from installation
}

// ExtensionDep represents a dependency between extensions
type ExtensionDep struct {
	ID           uint   `json:"id" gorm:"primarykey"`
	ExtensionID  uint   `json:"extension_id" gorm:"index;not null"`
	DependencyID uint   `json:"dependency_id" gorm:"index;not null"`
	VersionRange string `json:"version_range" gorm:"size:50"` // Semver range
	IsOptional   bool   `json:"is_optional" gorm:"default:false"`
}

// ExtensionReview represents a user review of an extension
type ExtensionReview struct {
	ID          uint           `json:"id" gorm:"primarykey"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	ExtensionID uint   `json:"extension_id" gorm:"index;not null"`
	UserID      uint   `json:"user_id" gorm:"index;not null"`
	Rating      int    `json:"rating" gorm:"not null"` // 1-5
	Title       string `json:"title" gorm:"size:200"`
	Content     string `json:"content" gorm:"type:text"`
	Version     string `json:"version" gorm:"size:20"` // Version reviewed
	IsVerified  bool   `json:"is_verified" gorm:"default:false"` // Verified purchase
	HelpfulCount int   `json:"helpful_count" gorm:"default:0"`
}

// UserExtension represents a user's installed extension
type UserExtension struct {
	ID          uint           `json:"id" gorm:"primarykey"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	UserID      uint `json:"user_id" gorm:"uniqueIndex:idx_user_extension;not null"`
	ExtensionID uint `json:"extension_id" gorm:"uniqueIndex:idx_user_extension;not null"`

	// Installation state
	Enabled       bool   `json:"enabled" gorm:"default:true"`
	Version       string `json:"version" gorm:"size:20;not null"`
	AutoUpdate    bool   `json:"auto_update" gorm:"default:true"`
	InstalledAt   time.Time `json:"installed_at"`
	LastUpdatedAt *time.Time `json:"last_updated_at"`

	// User-specific settings for this extension
	Settings string `json:"settings" gorm:"type:text"` // JSON

	// Granted permissions (user can revoke specific permissions)
	GrantedPermissions string `json:"granted_permissions" gorm:"type:text"` // JSON array

	// Relationships
	Extension *Extension `json:"extension,omitempty" gorm:"foreignKey:ExtensionID"`
}

// ParseManifest parses the manifest JSON string into ExtensionManifest
func (e *Extension) ParseManifest() (*ExtensionManifest, error) {
	var manifest ExtensionManifest
	if err := json.Unmarshal([]byte(e.Manifest), &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// GetTags returns the tags as a slice
func (e *Extension) GetTags() []string {
	if e.Tags == "" {
		return []string{}
	}
	var tags []string
	json.Unmarshal([]byte(e.Tags), &tags)
	return tags
}

// SetTags sets the tags from a slice
func (e *Extension) SetTags(tags []string) {
	data, _ := json.Marshal(tags)
	e.Tags = string(data)
}

// GetScreenshots returns the screenshots as a slice
func (e *Extension) GetScreenshots() []string {
	if e.Screenshots == "" {
		return []string{}
	}
	var screenshots []string
	json.Unmarshal([]byte(e.Screenshots), &screenshots)
	return screenshots
}

// SetScreenshots sets the screenshots from a slice
func (e *Extension) SetScreenshots(screenshots []string) {
	data, _ := json.Marshal(screenshots)
	e.Screenshots = string(data)
}

// ParseSettings parses the user's extension settings
func (ue *UserExtension) ParseSettings() (map[string]interface{}, error) {
	if ue.Settings == "" {
		return make(map[string]interface{}), nil
	}
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(ue.Settings), &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

// SetSettings sets the user's extension settings
func (ue *UserExtension) SetSettings(settings map[string]interface{}) {
	data, _ := json.Marshal(settings)
	ue.Settings = string(data)
}

// GetGrantedPermissions returns the granted permissions as a slice
func (ue *UserExtension) GetGrantedPermissions() []ExtensionPermission {
	if ue.GrantedPermissions == "" {
		return []ExtensionPermission{}
	}
	var perms []ExtensionPermission
	json.Unmarshal([]byte(ue.GrantedPermissions), &perms)
	return perms
}

// SetGrantedPermissions sets the granted permissions from a slice
func (ue *UserExtension) SetGrantedPermissions(perms []ExtensionPermission) {
	data, _ := json.Marshal(perms)
	ue.GrantedPermissions = string(data)
}
