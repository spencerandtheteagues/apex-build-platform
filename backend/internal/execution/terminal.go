// APEX.BUILD Interactive Terminal
// PTY-based terminal sessions with WebSocket support

package execution

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	terminalmux "apex-build/internal/terminal"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// TerminalSession represents an interactive terminal session
type TerminalSession struct {
	ID         string    `json:"id"`
	ProjectID  uint      `json:"project_id"`
	UserID     uint      `json:"user_id"`
	WorkDir    string    `json:"work_dir"`
	Shell      string    `json:"shell"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
	IsActive   bool      `json:"is_active"`
	Rows       uint16    `json:"rows"`
	Cols       uint16    `json:"cols"`
	Name       string    `json:"name"`

	// Internal fields
	cmd        *exec.Cmd
	pty        *os.File
	ws         *websocket.Conn
	wsMu       sync.Mutex
	done       chan struct{}
	history    []string
	historyMu  sync.RWMutex
	historyMax int
	customEnv  map[string]string
}

// TerminalManager manages terminal sessions
type TerminalManager struct {
	sessions    map[string]*TerminalSession
	sessionsMu  sync.RWMutex
	upgrader    websocket.Upgrader
	maxSessions int
	sessionTTL  time.Duration
	Multiplexer *terminalmux.Multiplexer
}

// TerminalMessage represents a message between client and terminal
type TerminalMessage struct {
	Type   string `json:"type"`
	Data   string `json:"data,omitempty"`
	Rows   uint16 `json:"rows,omitempty"`
	Cols   uint16 `json:"cols,omitempty"`
	Signal string `json:"signal,omitempty"`
}

// Terminal message types
const (
	TerminalMessageTypeInput  = "input"
	TerminalMessageTypeOutput = "output"
	TerminalMessageTypeResize = "resize"
	TerminalMessageTypeSignal = "signal"
	TerminalMessageTypePing   = "ping"
	TerminalMessageTypePong   = "pong"
	TerminalMessageTypeError  = "error"
	TerminalMessageTypeExit   = "exit"
)

// NewTerminalManager creates a new terminal manager
func NewTerminalManager() *TerminalManager {
	return &TerminalManager{
		sessions:    make(map[string]*TerminalSession),
		maxSessions: 100,
		sessionTTL:  30 * time.Minute,
		Multiplexer: terminalmux.NewMultiplexer(),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				allowedOrigins := []string{
					"http://localhost:3000",
					"http://localhost:5173",
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
		},
	}
}

// TerminalCreateOptions contains options for creating a terminal session
type TerminalCreateOptions struct {
	ProjectID   uint
	UserID      uint
	WorkDir     string
	Shell       string // bash, zsh, sh, or custom path
	Rows        uint16
	Cols        uint16
	Environment map[string]string
	Name        string
}

// GetAvailableShells returns a list of available shells on the system
func GetAvailableShells() []map[string]string {
	shells := []map[string]string{}

	shellPaths := []struct {
		name string
		path string
	}{
		{"bash", "/bin/bash"},
		{"zsh", "/bin/zsh"},
		{"sh", "/bin/sh"},
		{"fish", "/usr/bin/fish"},
		{"dash", "/bin/dash"},
	}

	for _, s := range shellPaths {
		if _, err := os.Stat(s.path); err == nil {
			shells = append(shells, map[string]string{
				"name": s.name,
				"path": s.path,
			})
		}
	}

	return shells
}

// ResolveShell resolves a shell name to its path
func ResolveShell(shellName string) string {
	switch shellName {
	case "bash":
		return "/bin/bash"
	case "zsh":
		return "/bin/zsh"
	case "sh":
		return "/bin/sh"
	case "fish":
		return "/usr/bin/fish"
	case "dash":
		return "/bin/dash"
	default:
		// If it's a path, use it directly
		if shellName != "" && shellName[0] == '/' {
			return shellName
		}
		// Default to bash
		return "/bin/bash"
	}
}

// CreateSession creates a new terminal session
func (tm *TerminalManager) CreateSession(projectID, userID uint, workDir string) (*TerminalSession, error) {
	return tm.CreateSessionWithOptions(TerminalCreateOptions{
		ProjectID: projectID,
		UserID:    userID,
		WorkDir:   workDir,
	})
}

// CreateSessionWithOptions creates a new terminal session with custom options
func (tm *TerminalManager) CreateSessionWithOptions(opts TerminalCreateOptions) (*TerminalSession, error) {
	tm.sessionsMu.Lock()
	defer tm.sessionsMu.Unlock()

	// Check session limit
	if len(tm.sessions) >= tm.maxSessions {
		return nil, fmt.Errorf("maximum number of terminal sessions reached")
	}

	// Determine shell
	shell := opts.Shell
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		shell = "/bin/bash"
	}
	shell = ResolveShell(shell)

	// Verify shell exists
	if _, err := os.Stat(shell); os.IsNotExist(err) {
		shell = "/bin/sh"
		if _, err := os.Stat(shell); os.IsNotExist(err) {
			return nil, fmt.Errorf("no shell available: tried %s and /bin/sh", opts.Shell)
		}
	}

	// Validate work directory
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = os.TempDir()
	}
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		if err := os.MkdirAll(workDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create work directory: %w", err)
		}
	}

	// Set default dimensions
	rows := opts.Rows
	cols := opts.Cols
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}

	// Generate session name
	name := opts.Name
	if name == "" {
		name = fmt.Sprintf("Terminal %d", len(tm.sessions)+1)
	}

	session := &TerminalSession{
		ID:         uuid.New().String(),
		ProjectID:  opts.ProjectID,
		UserID:     opts.UserID,
		WorkDir:    workDir,
		Shell:      shell,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		IsActive:   true,
		Rows:       rows,
		Cols:       cols,
		done:       make(chan struct{}),
		history:    make([]string, 0, 1000),
		historyMax: 1000,
	}

	// Store custom environment
	if opts.Environment != nil {
		session.customEnv = opts.Environment
	}

	tm.sessions[session.ID] = session
	return session, nil
}

// GetSession retrieves a terminal session by ID
func (tm *TerminalManager) GetSession(sessionID string) (*TerminalSession, bool) {
	tm.sessionsMu.RLock()
	defer tm.sessionsMu.RUnlock()
	session, exists := tm.sessions[sessionID]
	return session, exists
}

// GetUserSessions retrieves all sessions for a user
func (tm *TerminalManager) GetUserSessions(userID uint) []*TerminalSession {
	tm.sessionsMu.RLock()
	defer tm.sessionsMu.RUnlock()

	var sessions []*TerminalSession
	for _, session := range tm.sessions {
		if session.UserID == userID {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// DestroySession terminates and removes a terminal session
func (tm *TerminalManager) DestroySession(sessionID string) error {
	tm.sessionsMu.Lock()
	session, exists := tm.sessions[sessionID]
	if !exists {
		tm.sessionsMu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}
	delete(tm.sessions, sessionID)
	tm.sessionsMu.Unlock()

	// Stop the session
	session.Stop()
	return nil
}

// CleanupInactiveSessions removes inactive sessions
func (tm *TerminalManager) CleanupInactiveSessions() {
	tm.sessionsMu.Lock()
	defer tm.sessionsMu.Unlock()

	now := time.Now()
	for id, session := range tm.sessions {
		if now.Sub(session.LastActive) > tm.sessionTTL {
			session.Stop()
			delete(tm.sessions, id)
			log.Printf("Cleaned up inactive terminal session: %s", id)
		}
	}
}

// Start starts the terminal session with a PTY
func (ts *TerminalSession) Start() error {
	// Create command
	ts.cmd = exec.Command(ts.Shell)
	ts.cmd.Dir = ts.WorkDir

	// Build environment
	env := os.Environ()
	env = append(env,
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		fmt.Sprintf("APEX_PROJECT_ID=%d", ts.ProjectID),
		fmt.Sprintf("APEX_SESSION_ID=%s", ts.ID),
		fmt.Sprintf("APEX_TERMINAL_NAME=%s", ts.Name),
	)

	// Add custom environment variables
	if ts.customEnv != nil {
		for k, v := range ts.customEnv {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	ts.cmd.Env = env

	// Start with PTY
	ptmx, err := pty.StartWithSize(ts.cmd, &pty.Winsize{
		Rows: ts.Rows,
		Cols: ts.Cols,
	})
	if err != nil {
		return fmt.Errorf("failed to start PTY: %w", err)
	}

	ts.pty = ptmx
	ts.IsActive = true
	return nil
}

// Stop terminates the terminal session
func (ts *TerminalSession) Stop() {
	ts.IsActive = false

	// Close the done channel to signal readers
	select {
	case <-ts.done:
		// Already closed
	default:
		close(ts.done)
	}

	// Close PTY
	if ts.pty != nil {
		ts.pty.Close()
	}

	// Kill process
	if ts.cmd != nil && ts.cmd.Process != nil {
		ts.cmd.Process.Kill()
		ts.cmd.Wait()
	}

	// Close WebSocket
	ts.wsMu.Lock()
	if ts.ws != nil {
		ts.ws.Close()
	}
	ts.wsMu.Unlock()
}

// Resize resizes the terminal
func (ts *TerminalSession) Resize(rows, cols uint16) error {
	if ts.pty == nil {
		return fmt.Errorf("terminal not started")
	}

	ts.Rows = rows
	ts.Cols = cols

	return pty.Setsize(ts.pty, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

// Write writes data to the terminal
func (ts *TerminalSession) Write(data []byte) (int, error) {
	if ts.pty == nil {
		return 0, fmt.Errorf("terminal not started")
	}

	ts.LastActive = time.Now()
	return ts.pty.Write(data)
}

// Read reads data from the terminal
func (ts *TerminalSession) Read(buf []byte) (int, error) {
	if ts.pty == nil {
		return 0, fmt.Errorf("terminal not started")
	}

	return ts.pty.Read(buf)
}

// SendSignal sends a signal to the terminal process
func (ts *TerminalSession) SendSignal(signal os.Signal) error {
	if ts.cmd == nil || ts.cmd.Process == nil {
		return fmt.Errorf("no process running")
	}

	return ts.cmd.Process.Signal(signal)
}

// AddToHistory adds a command to the history
func (ts *TerminalSession) AddToHistory(cmd string) {
	ts.historyMu.Lock()
	defer ts.historyMu.Unlock()

	if len(ts.history) >= ts.historyMax {
		ts.history = ts.history[1:]
	}
	ts.history = append(ts.history, cmd)
}

// GetHistory returns the command history
func (ts *TerminalSession) GetHistory() []string {
	ts.historyMu.RLock()
	defer ts.historyMu.RUnlock()

	result := make([]string, len(ts.history))
	copy(result, ts.history)
	return result
}

// HandleWebSocket handles WebSocket connection for a terminal session
func (tm *TerminalManager) HandleWebSocket(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session ID required"})
		return
	}

	// Get session
	session, exists := tm.GetSession(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	// Upgrade to WebSocket
	ws, err := tm.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	session.wsMu.Lock()
	session.ws = ws
	session.wsMu.Unlock()

	defer func() {
		ws.Close()
		session.wsMu.Lock()
		session.ws = nil
		session.wsMu.Unlock()
	}()

	// Start terminal if not already started
	if session.pty == nil {
		if err := session.Start(); err != nil {
			sendError(ws, fmt.Sprintf("Failed to start terminal: %v", err))
			return
		}
	}

	// Start reading from PTY
	go tm.readFromPTY(session, ws)

	// Handle incoming WebSocket messages
	tm.handleWebSocketMessages(session, ws)
}

// readFromPTY reads output from PTY and sends to WebSocket
func (tm *TerminalManager) readFromPTY(session *TerminalSession, ws *websocket.Conn) {
	buf := make([]byte, 4096)

	for {
		select {
		case <-session.done:
			return
		default:
			n, err := session.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("PTY read error: %v", err)
				}
				sendMessage(ws, TerminalMessage{
					Type: TerminalMessageTypeExit,
					Data: "Terminal session ended",
				})
				return
			}

			if n > 0 {
				sendMessage(ws, TerminalMessage{
					Type: TerminalMessageTypeOutput,
					Data: string(buf[:n]),
				})
			}
		}
	}
}

// handleWebSocketMessages handles incoming WebSocket messages
func (tm *TerminalManager) handleWebSocketMessages(session *TerminalSession, ws *websocket.Conn) {
	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			return
		}

		var msg TerminalMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("Invalid message format: %v", err)
			continue
		}

		switch msg.Type {
		case TerminalMessageTypeInput:
			session.Write([]byte(msg.Data))

		case TerminalMessageTypeResize:
			if msg.Rows > 0 && msg.Cols > 0 {
				if err := session.Resize(msg.Rows, msg.Cols); err != nil {
					sendError(ws, fmt.Sprintf("Resize failed: %v", err))
				}
			}

		case TerminalMessageTypeSignal:
			switch msg.Signal {
			case "SIGINT":
				session.SendSignal(os.Interrupt)
			case "SIGTERM":
				session.SendSignal(os.Kill)
			case "SIGTSTP":
				// Ctrl+Z - handled differently
			}

		case TerminalMessageTypePing:
			sendMessage(ws, TerminalMessage{Type: TerminalMessageTypePong})
			session.LastActive = time.Now()

		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

// sendMessage sends a message via WebSocket
func sendMessage(ws *websocket.Conn, msg TerminalMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return ws.WriteMessage(websocket.TextMessage, data)
}

// sendError sends an error message via WebSocket
func sendError(ws *websocket.Conn, errMsg string) error {
	return sendMessage(ws, TerminalMessage{
		Type: TerminalMessageTypeError,
		Data: errMsg,
	})
}

// TerminalHandler provides HTTP handlers for terminal management
type TerminalHandler struct {
	manager *TerminalManager
}

// NewTerminalHandler creates a new terminal handler
func NewTerminalHandler(manager *TerminalManager) *TerminalHandler {
	return &TerminalHandler{manager: manager}
}

// CreateSessionHandler handles POST /api/v1/terminal/sessions
func (h *TerminalHandler) CreateSessionHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	var req struct {
		ProjectID   uint              `json:"project_id"`
		WorkDir     string            `json:"work_dir"`
		Shell       string            `json:"shell"`
		Name        string            `json:"name"`
		Rows        uint16            `json:"rows"`
		Cols        uint16            `json:"cols"`
		Environment map[string]string `json:"environment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	session, err := h.manager.CreateSessionWithOptions(TerminalCreateOptions{
		ProjectID:   req.ProjectID,
		UserID:      userID.(uint),
		WorkDir:     req.WorkDir,
		Shell:       req.Shell,
		Name:        req.Name,
		Rows:        req.Rows,
		Cols:        req.Cols,
		Environment: req.Environment,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"session_id":  session.ID,
			"project_id":  session.ProjectID,
			"work_dir":    session.WorkDir,
			"shell":       session.Shell,
			"name":        session.Name,
			"rows":        session.Rows,
			"cols":        session.Cols,
			"created_at":  session.CreatedAt,
			"ws_endpoint": fmt.Sprintf("/ws/terminal/%s", session.ID),
		},
	})
}

// GetAvailableShellsHandler handles GET /api/v1/terminal/shells
func (h *TerminalHandler) GetAvailableShellsHandler(c *gin.Context) {
	shells := GetAvailableShells()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"shells": shells,
		},
	})
}

// GetSessionHandler handles GET /api/v1/terminal/sessions/:id
func (h *TerminalHandler) GetSessionHandler(c *gin.Context) {
	sessionID := c.Param("id")

	session, exists := h.manager.GetSession(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	// Get shell name from path
	shellName := session.Shell
	if idx := len(session.Shell) - 1; idx >= 0 {
		for i := idx; i >= 0; i-- {
			if session.Shell[i] == '/' {
				shellName = session.Shell[i+1:]
				break
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"id":          session.ID,
			"project_id":  session.ProjectID,
			"user_id":     session.UserID,
			"work_dir":    session.WorkDir,
			"shell":       session.Shell,
			"shell_name":  shellName,
			"name":        session.Name,
			"created_at":  session.CreatedAt,
			"last_active": session.LastActive,
			"is_active":   session.IsActive,
			"rows":        session.Rows,
			"cols":        session.Cols,
			"ws_endpoint": fmt.Sprintf("/ws/terminal/%s", session.ID),
		},
	})
}

// ListSessionsHandler handles GET /api/v1/terminal/sessions
func (h *TerminalHandler) ListSessionsHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	sessions := h.manager.GetUserSessions(userID.(uint))

	sessionData := make([]map[string]interface{}, len(sessions))
	for i, session := range sessions {
		sessionData[i] = map[string]interface{}{
			"id":          session.ID,
			"project_id":  session.ProjectID,
			"work_dir":    session.WorkDir,
			"created_at":  session.CreatedAt,
			"last_active": session.LastActive,
			"is_active":   session.IsActive,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    sessionData,
	})
}

// DeleteSessionHandler handles DELETE /api/v1/terminal/sessions/:id
func (h *TerminalHandler) DeleteSessionHandler(c *gin.Context) {
	sessionID := c.Param("id")

	if err := h.manager.DestroySession(sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Terminal session terminated",
	})
}

// ResizeSessionHandler handles POST /api/v1/terminal/sessions/:id/resize
func (h *TerminalHandler) ResizeSessionHandler(c *gin.Context) {
	sessionID := c.Param("id")

	var req struct {
		Rows uint16 `json:"rows" binding:"required"`
		Cols uint16 `json:"cols" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	session, exists := h.manager.GetSession(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	if err := session.Resize(req.Rows, req.Cols); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Terminal resized",
	})
}

// GetHistoryHandler handles GET /api/v1/terminal/sessions/:id/history
func (h *TerminalHandler) GetHistoryHandler(c *gin.Context) {
	sessionID := c.Param("id")

	session, exists := h.manager.GetSession(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"history": session.GetHistory(),
		},
	})
}

// WebSocketHandler handles WebSocket connection for terminal
func (h *TerminalHandler) WebSocketHandler(c *gin.Context) {
	h.manager.HandleWebSocket(c)
}

// StartCleanupRoutine starts the background cleanup routine
func (tm *TerminalManager) StartCleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			tm.CleanupInactiveSessions()
		}
	}()
}

// GetActiveSessionCount returns the number of active sessions
func (tm *TerminalManager) GetActiveSessionCount() int {
	tm.sessionsMu.RLock()
	defer tm.sessionsMu.RUnlock()
	return len(tm.sessions)
}

// GetStats returns terminal manager statistics
func (tm *TerminalManager) GetStats() map[string]interface{} {
	tm.sessionsMu.RLock()
	defer tm.sessionsMu.RUnlock()

	activeSessions := 0
	for _, session := range tm.sessions {
		if session.IsActive {
			activeSessions++
		}
	}

	return map[string]interface{}{
		"total_sessions":   len(tm.sessions),
		"active_sessions":  activeSessions,
		"max_sessions":     tm.maxSessions,
		"session_ttl_mins": tm.sessionTTL.Minutes(),
	}
}
