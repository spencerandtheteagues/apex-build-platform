// APEX.BUILD Extension Sandbox
// Secure sandboxed execution environment for extensions

package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// SandboxConfig configures the sandbox environment
type SandboxConfig struct {
	// Resource limits
	MaxMemoryMB    int           `json:"max_memory_mb"`
	MaxCPUPercent  int           `json:"max_cpu_percent"`
	ExecuteTimeout time.Duration `json:"execute_timeout"`

	// Permissions
	AllowedPermissions []ExtensionPermission `json:"allowed_permissions"`

	// Network restrictions
	AllowedDomains []string `json:"allowed_domains"`
	BlockedDomains []string `json:"blocked_domains"`
}

// DefaultSandboxConfig returns the default sandbox configuration
func DefaultSandboxConfig() *SandboxConfig {
	return &SandboxConfig{
		MaxMemoryMB:    256,
		MaxCPUPercent:  50,
		ExecuteTimeout: 30 * time.Second,
		AllowedPermissions: []ExtensionPermission{
			PermissionStorage,
			PermissionNotification,
		},
		AllowedDomains: []string{},
		BlockedDomains: []string{},
	}
}

// Sandbox provides a secure execution environment for extensions
type Sandbox struct {
	mu       sync.RWMutex
	config   *SandboxConfig
	runtimes map[uint]*ExtensionRuntime
}

// NewSandbox creates a new sandbox environment
func NewSandbox(config *SandboxConfig) *Sandbox {
	if config == nil {
		config = DefaultSandboxConfig()
	}

	return &Sandbox{
		config:   config,
		runtimes: make(map[uint]*ExtensionRuntime),
	}
}

// ExtensionRuntime represents a running extension instance
type ExtensionRuntime struct {
	ExtensionID    uint
	Extension      *Extension
	UserID         uint
	UserExtension  *UserExtension
	StartedAt      time.Time
	LastActivity   time.Time

	// Granted permissions for this session
	Permissions    []ExtensionPermission

	// State
	State          map[string]interface{}

	// Communication channels
	MessageChan    chan *ExtensionMessage
	ResponseChan   chan *ExtensionResponse
	StopChan       chan struct{}

	// Metrics
	MemoryUsage    int64
	CPUTime        int64
	APICallCount   int
}

// ExtensionMessage represents a message from the extension
type ExtensionMessage struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// ExtensionResponse represents a response to the extension
type ExtensionResponse struct {
	Success bool                   `json:"success"`
	Data    interface{}            `json:"data"`
	Error   string                 `json:"error,omitempty"`
}

// LoadExtension loads an extension into the sandbox
func (s *Sandbox) LoadExtension(ctx context.Context, ext *Extension, userExt *UserExtension) (*ExtensionRuntime, error) {
	if ext == nil {
		return nil, errors.New("extension is required")
	}

	if ext.Status != StatusApproved {
		return nil, errors.New("extension is not approved")
	}

	// Parse manifest
	manifest, err := ext.ParseManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Check permissions
	grantedPerms := userExt.GetGrantedPermissions()
	for _, reqPerm := range manifest.Permissions {
		hasPermission := false
		for _, grantedPerm := range grantedPerms {
			if grantedPerm == reqPerm {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			// Check if permission is allowed by sandbox
			isAllowed := false
			for _, allowedPerm := range s.config.AllowedPermissions {
				if allowedPerm == reqPerm {
					isAllowed = true
					break
				}
			}
			if !isAllowed {
				return nil, fmt.Errorf("extension requires permission: %s", reqPerm)
			}
		}
	}

	runtime := &ExtensionRuntime{
		ExtensionID:   ext.ID,
		Extension:     ext,
		UserID:        userExt.UserID,
		UserExtension: userExt,
		StartedAt:     time.Now(),
		LastActivity:  time.Now(),
		Permissions:   grantedPerms,
		State:         make(map[string]interface{}),
		MessageChan:   make(chan *ExtensionMessage, 100),
		ResponseChan:  make(chan *ExtensionResponse, 100),
		StopChan:      make(chan struct{}),
	}

	s.mu.Lock()
	s.runtimes[ext.ID] = runtime
	s.mu.Unlock()

	// Start the extension runtime
	go s.runExtension(ctx, runtime, manifest)

	return runtime, nil
}

// runExtension runs the extension in a sandboxed environment
func (s *Sandbox) runExtension(ctx context.Context, runtime *ExtensionRuntime, manifest *ExtensionManifest) {
	defer func() {
		s.mu.Lock()
		delete(s.runtimes, runtime.ExtensionID)
		s.mu.Unlock()

		close(runtime.MessageChan)
		close(runtime.ResponseChan)
	}()

	// Set up timeout
	timeout := s.config.ExecuteTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Main extension loop
	for {
		select {
		case <-timeoutCtx.Done():
			return
		case <-runtime.StopChan:
			return
		case msg, ok := <-runtime.MessageChan:
			if !ok {
				return
			}

			// Process message
			response := s.handleExtensionMessage(runtime, msg)
			runtime.ResponseChan <- response
			runtime.LastActivity = time.Now()
			runtime.APICallCount++
		}
	}
}

// handleExtensionMessage handles messages from extensions
func (s *Sandbox) handleExtensionMessage(runtime *ExtensionRuntime, msg *ExtensionMessage) *ExtensionResponse {
	switch msg.Type {
	case "storage.get":
		return s.handleStorageGet(runtime, msg)
	case "storage.set":
		return s.handleStorageSet(runtime, msg)
	case "notification.show":
		return s.handleNotificationShow(runtime, msg)
	case "file.read":
		return s.handleFileRead(runtime, msg)
	case "file.write":
		return s.handleFileWrite(runtime, msg)
	case "command.execute":
		return s.handleCommandExecute(runtime, msg)
	default:
		return &ExtensionResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown message type: %s", msg.Type),
		}
	}
}

// handleStorageGet handles storage read requests
func (s *Sandbox) handleStorageGet(runtime *ExtensionRuntime, msg *ExtensionMessage) *ExtensionResponse {
	if !s.hasPermission(runtime, PermissionStorage) {
		return &ExtensionResponse{Success: false, Error: "storage permission not granted"}
	}

	key, _ := msg.Payload["key"].(string)
	if key == "" {
		return &ExtensionResponse{Success: false, Error: "key is required"}
	}

	value, ok := runtime.State[key]
	if !ok {
		// Try to load from user extension settings
		settings, _ := runtime.UserExtension.ParseSettings()
		value = settings[key]
	}

	return &ExtensionResponse{Success: true, Data: value}
}

// handleStorageSet handles storage write requests
func (s *Sandbox) handleStorageSet(runtime *ExtensionRuntime, msg *ExtensionMessage) *ExtensionResponse {
	if !s.hasPermission(runtime, PermissionStorage) {
		return &ExtensionResponse{Success: false, Error: "storage permission not granted"}
	}

	key, _ := msg.Payload["key"].(string)
	if key == "" {
		return &ExtensionResponse{Success: false, Error: "key is required"}
	}

	value := msg.Payload["value"]
	runtime.State[key] = value

	return &ExtensionResponse{Success: true}
}

// handleNotificationShow handles notification display requests
func (s *Sandbox) handleNotificationShow(runtime *ExtensionRuntime, msg *ExtensionMessage) *ExtensionResponse {
	if !s.hasPermission(runtime, PermissionNotification) {
		return &ExtensionResponse{Success: false, Error: "notification permission not granted"}
	}

	// In a real implementation, this would send a notification to the user
	title, _ := msg.Payload["title"].(string)
	message, _ := msg.Payload["message"].(string)

	fmt.Printf("[Extension %d] Notification: %s - %s\n", runtime.ExtensionID, title, message)

	return &ExtensionResponse{Success: true}
}

// handleFileRead handles file read requests
func (s *Sandbox) handleFileRead(runtime *ExtensionRuntime, msg *ExtensionMessage) *ExtensionResponse {
	if !s.hasPermission(runtime, PermissionFileRead) {
		return &ExtensionResponse{Success: false, Error: "file:read permission not granted"}
	}

	// In a real implementation, this would read the file with proper access control
	path, _ := msg.Payload["path"].(string)
	if path == "" {
		return &ExtensionResponse{Success: false, Error: "path is required"}
	}

	// Placeholder - would need actual file system integration
	return &ExtensionResponse{
		Success: true,
		Data:    map[string]interface{}{"content": "", "exists": false},
	}
}

// handleFileWrite handles file write requests
func (s *Sandbox) handleFileWrite(runtime *ExtensionRuntime, msg *ExtensionMessage) *ExtensionResponse {
	if !s.hasPermission(runtime, PermissionFileWrite) {
		return &ExtensionResponse{Success: false, Error: "file:write permission not granted"}
	}

	// In a real implementation, this would write the file with proper access control
	path, _ := msg.Payload["path"].(string)
	if path == "" {
		return &ExtensionResponse{Success: false, Error: "path is required"}
	}

	// Placeholder - would need actual file system integration
	return &ExtensionResponse{Success: true}
}

// handleCommandExecute handles command execution requests
func (s *Sandbox) handleCommandExecute(runtime *ExtensionRuntime, msg *ExtensionMessage) *ExtensionResponse {
	if !s.hasPermission(runtime, PermissionTerminal) {
		return &ExtensionResponse{Success: false, Error: "terminal permission not granted"}
	}

	// In a real implementation, this would execute commands in a sandboxed terminal
	command, _ := msg.Payload["command"].(string)
	if command == "" {
		return &ExtensionResponse{Success: false, Error: "command is required"}
	}

	// Placeholder - would need actual terminal integration with security controls
	return &ExtensionResponse{
		Success: true,
		Data:    map[string]interface{}{"output": "", "exitCode": 0},
	}
}

// hasPermission checks if the runtime has a specific permission
func (s *Sandbox) hasPermission(runtime *ExtensionRuntime, perm ExtensionPermission) bool {
	for _, p := range runtime.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// UnloadExtension unloads an extension from the sandbox
func (s *Sandbox) UnloadExtension(extensionID uint) error {
	s.mu.Lock()
	runtime, exists := s.runtimes[extensionID]
	s.mu.Unlock()

	if !exists {
		return errors.New("extension not loaded")
	}

	// Signal stop
	close(runtime.StopChan)

	return nil
}

// GetRuntime gets the runtime for an extension
func (s *Sandbox) GetRuntime(extensionID uint) (*ExtensionRuntime, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	runtime, exists := s.runtimes[extensionID]
	return runtime, exists
}

// GetActiveRuntimes returns all active runtimes
func (s *Sandbox) GetActiveRuntimes() []*ExtensionRuntime {
	s.mu.RLock()
	defer s.mu.RUnlock()

	runtimes := make([]*ExtensionRuntime, 0, len(s.runtimes))
	for _, r := range s.runtimes {
		runtimes = append(runtimes, r)
	}
	return runtimes
}

// SendMessage sends a message to an extension
func (s *Sandbox) SendMessage(extensionID uint, msg *ExtensionMessage) (*ExtensionResponse, error) {
	s.mu.RLock()
	runtime, exists := s.runtimes[extensionID]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.New("extension not loaded")
	}

	// Send message
	select {
	case runtime.MessageChan <- msg:
	default:
		return nil, errors.New("message queue full")
	}

	// Wait for response with timeout
	select {
	case response := <-runtime.ResponseChan:
		return response, nil
	case <-time.After(5 * time.Second):
		return nil, errors.New("response timeout")
	}
}

// ExtensionAPI provides the API that extensions can call
type ExtensionAPI struct {
	sandbox *Sandbox
	runtime *ExtensionRuntime
}

// NewExtensionAPI creates a new extension API instance
func NewExtensionAPI(sandbox *Sandbox, runtime *ExtensionRuntime) *ExtensionAPI {
	return &ExtensionAPI{
		sandbox: sandbox,
		runtime: runtime,
	}
}

// Storage provides storage API methods
func (api *ExtensionAPI) Storage() *StorageAPI {
	return &StorageAPI{api: api}
}

// StorageAPI provides storage methods
type StorageAPI struct {
	api *ExtensionAPI
}

// Get retrieves a value from storage
func (s *StorageAPI) Get(key string) (interface{}, error) {
	msg := &ExtensionMessage{
		Type:    "storage.get",
		Payload: map[string]interface{}{"key": key},
	}

	resp, err := s.api.sandbox.SendMessage(s.api.runtime.ExtensionID, msg)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, errors.New(resp.Error)
	}

	return resp.Data, nil
}

// Set stores a value in storage
func (s *StorageAPI) Set(key string, value interface{}) error {
	msg := &ExtensionMessage{
		Type: "storage.set",
		Payload: map[string]interface{}{
			"key":   key,
			"value": value,
		},
	}

	resp, err := s.api.sandbox.SendMessage(s.api.runtime.ExtensionID, msg)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Error)
	}

	return nil
}

// ExtensionBundle represents a packaged extension
type ExtensionBundle struct {
	Manifest *ExtensionManifest `json:"manifest"`
	Files    map[string]string  `json:"files"` // path -> base64 content
	Assets   map[string]string  `json:"assets"` // asset name -> base64 content
}

// PackExtension packages an extension for distribution
func PackExtension(manifest *ExtensionManifest, files map[string]string, assets map[string]string) ([]byte, error) {
	bundle := &ExtensionBundle{
		Manifest: manifest,
		Files:    files,
		Assets:   assets,
	}

	return json.Marshal(bundle)
}

// UnpackExtension unpacks an extension bundle
func UnpackExtension(data []byte) (*ExtensionBundle, error) {
	var bundle ExtensionBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("failed to unpack extension: %w", err)
	}

	if bundle.Manifest == nil {
		return nil, errors.New("bundle missing manifest")
	}

	return &bundle, nil
}
