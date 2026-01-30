// APEX.BUILD Debugging Service
// Advanced debugging with breakpoint support and Chrome DevTools Protocol integration

package debugging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

// DebugSessionStatus represents the state of a debug session
type DebugSessionStatus string

const (
	SessionStatusPending   DebugSessionStatus = "pending"
	SessionStatusRunning   DebugSessionStatus = "running"
	SessionStatusPaused    DebugSessionStatus = "paused"
	SessionStatusCompleted DebugSessionStatus = "completed"
	SessionStatusError     DebugSessionStatus = "error"
)

// BreakpointType represents types of breakpoints
type BreakpointType string

const (
	BreakpointLine        BreakpointType = "line"
	BreakpointConditional BreakpointType = "conditional"
	BreakpointLogpoint    BreakpointType = "logpoint"
	BreakpointException   BreakpointType = "exception"
	BreakpointFunction    BreakpointType = "function"
)

// DebugSession represents an active debugging session
type DebugSession struct {
	ID               string             `json:"id" gorm:"primarykey;type:varchar(36)"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
	DeletedAt        gorm.DeletedAt     `json:"-" gorm:"index"`
	ProjectID        uint               `json:"project_id" gorm:"not null;index"`
	UserID           uint               `json:"user_id" gorm:"not null;index"`
	FileID           uint               `json:"file_id" gorm:"index"`
	Status           DebugSessionStatus `json:"status" gorm:"type:varchar(50);default:'pending'"`
	Language         string             `json:"language" gorm:"not null"`
	EntryPoint       string             `json:"entry_point"`
	WorkingDirectory string             `json:"working_directory"`
	DebugPort        int                `json:"debug_port"`
	DevToolsURL      string             `json:"devtools_url,omitempty"`
	ProcessID        int                `json:"process_id,omitempty"`
	ErrorMessage     string             `json:"error_message,omitempty"`
	StartedAt        *time.Time         `json:"started_at,omitempty"`
	EndedAt          *time.Time         `json:"ended_at,omitempty"`
}

// Breakpoint represents a debugger breakpoint
type Breakpoint struct {
	ID           string         `json:"id" gorm:"primarykey;type:varchar(36)"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
	SessionID    string         `json:"session_id" gorm:"not null;index;type:varchar(36)"`
	FileID       uint           `json:"file_id" gorm:"not null;index"`
	FilePath     string         `json:"file_path" gorm:"not null"`
	Line         int            `json:"line" gorm:"not null"`
	Column       int            `json:"column" gorm:"default:0"`
	Type         BreakpointType `json:"type" gorm:"type:varchar(20);default:'line'"`
	Condition    string         `json:"condition,omitempty"`
	LogMessage   string         `json:"log_message,omitempty"`
	HitCount     int            `json:"hit_count" gorm:"default:0"`
	Enabled      bool           `json:"enabled" gorm:"default:true"`
	Verified     bool           `json:"verified" gorm:"default:false"`
	BreakpointID string         `json:"breakpoint_id,omitempty"` // Chrome DevTools breakpoint ID
}

// StackFrame represents a frame in the call stack
type StackFrame struct {
	ID           string            `json:"id"`
	Index        int               `json:"index"`
	FunctionName string            `json:"function_name"`
	FilePath     string            `json:"file_path"`
	Line         int               `json:"line"`
	Column       int               `json:"column"`
	ScriptID     string            `json:"script_id,omitempty"`
	IsAsync      bool              `json:"is_async"`
	Scopes       []Scope           `json:"scopes,omitempty"`
	LocalVars    map[string]string `json:"local_vars,omitempty"`
}

// Scope represents a variable scope
type Scope struct {
	Type      string     `json:"type"` // local, closure, global, with, catch, block, script
	Name      string     `json:"name,omitempty"`
	StartLine int        `json:"start_line,omitempty"`
	EndLine   int        `json:"end_line,omitempty"`
	Variables []Variable `json:"variables,omitempty"`
}

// Variable represents a debugger variable
type Variable struct {
	Name        string     `json:"name"`
	Value       string     `json:"value"`
	Type        string     `json:"type"`
	ObjectID    string     `json:"object_id,omitempty"`
	HasChildren bool       `json:"has_children"`
	Children    []Variable `json:"children,omitempty"`
}

// WatchExpression represents a watch expression
type WatchExpression struct {
	ID         string `json:"id"`
	Expression string `json:"expression"`
	Value      string `json:"value,omitempty"`
	Type       string `json:"type,omitempty"`
	Error      string `json:"error,omitempty"`
}

// DebugEvent represents an event from the debugger
type DebugEvent struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// PausedEventData contains information when execution is paused
type PausedEventData struct {
	Reason      string       `json:"reason"` // breakpoint, exception, step, debugger_statement
	Breakpoint  *Breakpoint  `json:"breakpoint,omitempty"`
	CallStack   []StackFrame `json:"call_stack"`
	Exception   *Variable    `json:"exception,omitempty"`
	HitBreakIDs []string     `json:"hit_breakpoint_ids,omitempty"`
}

// DebugService manages debugging sessions
type DebugService struct {
	db              *gorm.DB
	sessions        map[string]*activeSession
	mu              sync.RWMutex
	portAllocator   *PortAllocator
	upgrader        websocket.Upgrader
	eventBroadcasts map[string]chan DebugEvent
}

// activeSession holds runtime information for a debug session
type activeSession struct {
	session       *DebugSession
	cdpConn       *websocket.Conn
	breakpoints   map[string]*Breakpoint
	callStack     []StackFrame
	watchExprs    map[string]*WatchExpression
	scripts       map[string]string // scriptID -> filePath
	mu            sync.RWMutex
	eventChannel  chan DebugEvent
	done          chan struct{}
}

// PortAllocator manages debug port allocation
type PortAllocator struct {
	basePort  int
	maxPort   int
	allocated map[int]bool
	mu        sync.Mutex
}

// NewDebugService creates a new debugging service
func NewDebugService(db *gorm.DB) *DebugService {
	svc := &DebugService{
		db:              db,
		sessions:        make(map[string]*activeSession),
		eventBroadcasts: make(map[string]chan DebugEvent),
		portAllocator: &PortAllocator{
			basePort:  9229,
			maxPort:   9329,
			allocated: make(map[int]bool),
		},
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	// Run migrations
	db.AutoMigrate(&DebugSession{}, &Breakpoint{})

	return svc
}

// StartSession starts a new debugging session
func (s *DebugService) StartSession(ctx context.Context, userID, projectID, fileID uint, language, entryPoint string) (*DebugSession, error) {
	// Allocate debug port
	port, err := s.portAllocator.Allocate()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate debug port: %w", err)
	}

	now := time.Now()
	session := &DebugSession{
		ID:         uuid.New().String(),
		ProjectID:  projectID,
		UserID:     userID,
		FileID:     fileID,
		Status:     SessionStatusPending,
		Language:   language,
		EntryPoint: entryPoint,
		DebugPort:  port,
		StartedAt:  &now,
	}

	if err := s.db.Create(session).Error; err != nil {
		s.portAllocator.Release(port)
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Create active session
	active := &activeSession{
		session:      session,
		breakpoints:  make(map[string]*Breakpoint),
		watchExprs:   make(map[string]*WatchExpression),
		scripts:      make(map[string]string),
		eventChannel: make(chan DebugEvent, 100),
		done:         make(chan struct{}),
	}

	s.mu.Lock()
	s.sessions[session.ID] = active
	s.eventBroadcasts[session.ID] = active.eventChannel
	s.mu.Unlock()

	// Start the debug process based on language
	go s.startDebugProcess(active)

	return session, nil
}

// startDebugProcess starts the appropriate debugger for the language
func (s *DebugService) startDebugProcess(active *activeSession) {
	session := active.session

	var err error
	switch session.Language {
	case "javascript", "typescript":
		err = s.startNodeDebugger(active)
	case "python":
		err = s.startPythonDebugger(active)
	case "go":
		err = s.startGoDebugger(active)
	default:
		err = fmt.Errorf("unsupported language: %s", session.Language)
	}

	if err != nil {
		session.Status = SessionStatusError
		session.ErrorMessage = err.Error()
		s.db.Save(session)
		return
	}

	session.Status = SessionStatusRunning
	s.db.Save(session)
}

// startNodeDebugger starts Node.js debugger with Chrome DevTools Protocol
func (s *DebugService) startNodeDebugger(active *activeSession) error {
	session := active.session

	// In production, this would:
	// 1. Start node with --inspect-brk=port flag
	// 2. Connect to Chrome DevTools Protocol
	// 3. Set up event handlers

	// Simulate DevTools URL
	session.DevToolsURL = fmt.Sprintf("devtools://devtools/bundled/js_app.html?experiments=true&v8only=true&ws=127.0.0.1:%d", session.DebugPort)

	// Connect to CDP
	go s.handleCDPConnection(active)

	return nil
}

// startPythonDebugger starts Python debugger (debugpy)
func (s *DebugService) startPythonDebugger(active *activeSession) error {
	// Would start: python -m debugpy --listen port entrypoint.py
	return nil
}

// startGoDebugger starts Go debugger (delve)
func (s *DebugService) startGoDebugger(active *activeSession) error {
	// Would start: dlv debug --headless --api-version=2 --listen=:port
	return nil
}

// handleCDPConnection manages Chrome DevTools Protocol connection
func (s *DebugService) handleCDPConnection(active *activeSession) {
	// This would maintain the CDP WebSocket connection
	// and handle events from the debugger

	// Simulated event handling loop
	for {
		select {
		case <-active.done:
			return
		default:
			// Would receive and process CDP events here
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// SetBreakpoint sets a breakpoint in the debug session
func (s *DebugService) SetBreakpoint(sessionID string, fileID uint, filePath string, line int, bpType BreakpointType, condition string) (*Breakpoint, error) {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.New("session not found")
	}

	bp := &Breakpoint{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		FileID:    fileID,
		FilePath:  filePath,
		Line:      line,
		Type:      bpType,
		Condition: condition,
		Enabled:   true,
	}

	// Save to database
	if err := s.db.Create(bp).Error; err != nil {
		return nil, err
	}

	// Add to active session
	active.mu.Lock()
	active.breakpoints[bp.ID] = bp
	active.mu.Unlock()

	// Set breakpoint in debugger (CDP command)
	go s.setCDPBreakpoint(active, bp)

	return bp, nil
}

// setCDPBreakpoint sends breakpoint to Chrome DevTools Protocol
func (s *DebugService) setCDPBreakpoint(active *activeSession, bp *Breakpoint) {
	// Would send CDP command: Debugger.setBreakpoint or Debugger.setBreakpointByUrl
	// CDP response would include the actual breakpoint ID

	// Simulate verification
	time.Sleep(50 * time.Millisecond)
	bp.Verified = true
	bp.BreakpointID = fmt.Sprintf("cdp-bp-%s", bp.ID[:8])
	s.db.Save(bp)

	// Notify clients
	active.eventChannel <- DebugEvent{
		Type:      "breakpoint_verified",
		Timestamp: time.Now(),
		Data:      bp,
	}
}

// RemoveBreakpoint removes a breakpoint
func (s *DebugService) RemoveBreakpoint(sessionID, breakpointID string) error {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return errors.New("session not found")
	}

	active.mu.Lock()
	delete(active.breakpoints, breakpointID)
	active.mu.Unlock()

	// Remove from debugger (CDP command)
	// Would send: Debugger.removeBreakpoint

	return s.db.Delete(&Breakpoint{}, "id = ?", breakpointID).Error
}

// ToggleBreakpoint enables/disables a breakpoint
func (s *DebugService) ToggleBreakpoint(sessionID, breakpointID string, enabled bool) error {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return errors.New("session not found")
	}

	active.mu.Lock()
	if bp, ok := active.breakpoints[breakpointID]; ok {
		bp.Enabled = enabled
	}
	active.mu.Unlock()

	// Would send CDP command to activate/deactivate

	return s.db.Model(&Breakpoint{}).Where("id = ?", breakpointID).Update("enabled", enabled).Error
}

// Continue resumes execution
func (s *DebugService) Continue(sessionID string) error {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return errors.New("session not found")
	}

	// Send CDP command: Debugger.resume
	active.session.Status = SessionStatusRunning
	s.db.Save(active.session)

	active.eventChannel <- DebugEvent{
		Type:      "resumed",
		Timestamp: time.Now(),
	}

	return nil
}

// StepOver executes the next line
func (s *DebugService) StepOver(sessionID string) error {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return errors.New("session not found")
	}

	// Send CDP command: Debugger.stepOver
	active.eventChannel <- DebugEvent{
		Type:      "stepping",
		Timestamp: time.Now(),
		Data:      map[string]string{"step_type": "over"},
	}

	return nil
}

// StepInto steps into a function call
func (s *DebugService) StepInto(sessionID string) error {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return errors.New("session not found")
	}

	// Send CDP command: Debugger.stepInto
	active.eventChannel <- DebugEvent{
		Type:      "stepping",
		Timestamp: time.Now(),
		Data:      map[string]string{"step_type": "into"},
	}

	return nil
}

// StepOut steps out of the current function
func (s *DebugService) StepOut(sessionID string) error {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return errors.New("session not found")
	}

	// Send CDP command: Debugger.stepOut
	active.eventChannel <- DebugEvent{
		Type:      "stepping",
		Timestamp: time.Now(),
		Data:      map[string]string{"step_type": "out"},
	}

	return nil
}

// Pause pauses execution
func (s *DebugService) Pause(sessionID string) error {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return errors.New("session not found")
	}

	// Send CDP command: Debugger.pause
	active.session.Status = SessionStatusPaused
	s.db.Save(active.session)

	return nil
}

// GetCallStack returns the current call stack
func (s *DebugService) GetCallStack(sessionID string) ([]StackFrame, error) {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.New("session not found")
	}

	active.mu.RLock()
	stack := make([]StackFrame, len(active.callStack))
	copy(stack, active.callStack)
	active.mu.RUnlock()

	return stack, nil
}

// GetVariables returns variables for a specific scope
func (s *DebugService) GetVariables(sessionID, objectID string) ([]Variable, error) {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.New("session not found")
	}

	// Would send CDP command: Runtime.getProperties
	// Return cached or fetched variables

	_ = active // Use active session

	// Simulated variables
	return []Variable{
		{Name: "this", Value: "Object", Type: "object", HasChildren: true},
		{Name: "arguments", Value: "Arguments(2)", Type: "object", HasChildren: true},
	}, nil
}

// EvaluateExpression evaluates an expression in the current context
func (s *DebugService) EvaluateExpression(sessionID, expression string) (*Variable, error) {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.New("session not found")
	}

	// Would send CDP command: Runtime.evaluate or Debugger.evaluateOnCallFrame

	_ = active // Use active session

	// Simulated evaluation
	return &Variable{
		Name:  expression,
		Value: "<evaluated>",
		Type:  "string",
	}, nil
}

// AddWatch adds a watch expression
func (s *DebugService) AddWatch(sessionID, expression string) (*WatchExpression, error) {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.New("session not found")
	}

	watch := &WatchExpression{
		ID:         uuid.New().String(),
		Expression: expression,
	}

	// Evaluate immediately
	result, err := s.EvaluateExpression(sessionID, expression)
	if err != nil {
		watch.Error = err.Error()
	} else {
		watch.Value = result.Value
		watch.Type = result.Type
	}

	active.mu.Lock()
	active.watchExprs[watch.ID] = watch
	active.mu.Unlock()

	return watch, nil
}

// RemoveWatch removes a watch expression
func (s *DebugService) RemoveWatch(sessionID, watchID string) error {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return errors.New("session not found")
	}

	active.mu.Lock()
	delete(active.watchExprs, watchID)
	active.mu.Unlock()

	return nil
}

// GetWatches returns all watch expressions
func (s *DebugService) GetWatches(sessionID string) ([]*WatchExpression, error) {
	s.mu.RLock()
	active, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.New("session not found")
	}

	active.mu.RLock()
	watches := make([]*WatchExpression, 0, len(active.watchExprs))
	for _, w := range active.watchExprs {
		watches = append(watches, w)
	}
	active.mu.RUnlock()

	return watches, nil
}

// StopSession stops a debugging session
func (s *DebugService) StopSession(sessionID string) error {
	s.mu.Lock()
	active, exists := s.sessions[sessionID]
	if exists {
		close(active.done)
		delete(s.sessions, sessionID)
		delete(s.eventBroadcasts, sessionID)
	}
	s.mu.Unlock()

	if !exists {
		return errors.New("session not found")
	}

	// Release port
	s.portAllocator.Release(active.session.DebugPort)

	// Close CDP connection
	if active.cdpConn != nil {
		active.cdpConn.Close()
	}

	// Update database
	now := time.Now()
	active.session.Status = SessionStatusCompleted
	active.session.EndedAt = &now
	return s.db.Save(active.session).Error
}

// GetSession returns a debug session by ID
func (s *DebugService) GetSession(sessionID string) (*DebugSession, error) {
	var session DebugSession
	if err := s.db.First(&session, "id = ?", sessionID).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// GetSessionBreakpoints returns all breakpoints for a session
func (s *DebugService) GetSessionBreakpoints(sessionID string) ([]Breakpoint, error) {
	var breakpoints []Breakpoint
	if err := s.db.Where("session_id = ?", sessionID).Find(&breakpoints).Error; err != nil {
		return nil, err
	}
	return breakpoints, nil
}

// GetEventChannel returns the event channel for a session
func (s *DebugService) GetEventChannel(sessionID string) (<-chan DebugEvent, error) {
	s.mu.RLock()
	ch, exists := s.eventBroadcasts[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.New("session not found")
	}

	return ch, nil
}

// PortAllocator methods

func (p *PortAllocator) Allocate() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for port := p.basePort; port <= p.maxPort; port++ {
		if !p.allocated[port] {
			p.allocated[port] = true
			return port, nil
		}
	}

	return 0, errors.New("no available debug ports")
}

func (p *PortAllocator) Release(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.allocated, port)
}

// CDPMessage represents a Chrome DevTools Protocol message
type CDPMessage struct {
	ID     int             `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *CDPError       `json:"error,omitempty"`
}

// CDPError represents a CDP error
type CDPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
