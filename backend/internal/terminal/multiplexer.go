package terminal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
)

// SessionOptions configures a multiplexed terminal session.
type SessionOptions struct {
	ProjectID uint
	UserID    uint
	WorkDir   string
	Shell     string
	Rows      uint16
	Cols      uint16
	Env       map[string]string
	Name      string
}

// SessionSnapshot is a read-only view of a multiplexed session.
type SessionSnapshot struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	ProjectID  uint      `json:"project_id"`
	UserID     uint      `json:"user_id"`
	WorkDir    string    `json:"work_dir"`
	Shell      string    `json:"shell"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
	Rows       uint16    `json:"rows"`
	Cols       uint16    `json:"cols"`
	Clients    int       `json:"clients"`
	Alive      bool      `json:"alive"`
}

// ClientAttachment is a subscriber attached to a multiplexed session.
type ClientAttachment struct {
	ClientID string
	Output   <-chan []byte
	Write    func([]byte) (int, error)
	Resize   func(rows, cols uint16) error
	Close    func() error
}

type clientSubscriber struct {
	id string
	ch chan []byte
}

type session struct {
	id         string
	name       string
	projectID  uint
	userID     uint
	workDir    string
	shell      string
	createdAt  time.Time
	lastActive time.Time
	rows       uint16
	cols       uint16
	cmd        *exec.Cmd
	ptmx       *os.File
	done       chan struct{}

	mu          sync.RWMutex
	subscribers map[string]*clientSubscriber
	history     []byte
	historyMax  int
}

// Multiplexer manages shared PTY sessions with multiple client subscribers.
type Multiplexer struct {
	mu           sync.RWMutex
	sessions     map[string]*session
	maxSessions  int
	sessionTTL   time.Duration
	historyBytes int
}

// NewMultiplexer creates a multiplexer with production-safe defaults.
func NewMultiplexer() *Multiplexer {
	return &Multiplexer{
		sessions:     make(map[string]*session),
		maxSessions:  128,
		sessionTTL:   45 * time.Minute,
		historyBytes: 128 * 1024,
	}
}

// CreateSession creates a shared PTY session suitable for multiple websocket viewers.
func (m *Multiplexer) CreateSession(ctx context.Context, opts SessionOptions) (*SessionSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) >= m.maxSessions {
		return nil, fmt.Errorf("terminal multiplexer capacity reached")
	}

	shellPath := resolveShell(opts.Shell)
	if _, err := os.Stat(shellPath); err != nil {
		return nil, fmt.Errorf("shell not found: %s", shellPath)
	}
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = os.TempDir()
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, fmt.Errorf("create workdir: %w", err)
	}

	rows := opts.Rows
	cols := opts.Cols
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}

	cmd := exec.CommandContext(ctx, shellPath)
	cmd.Dir = workDir
	cmd.Env = append([]string{}, os.Environ()...)
	for k, v := range opts.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: rows, Cols: cols})
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}

	id := uuid.New().String()
	name := opts.Name
	if name == "" {
		name = "mux-terminal"
	}

	s := &session{
		id:          id,
		name:        name,
		projectID:   opts.ProjectID,
		userID:      opts.UserID,
		workDir:     workDir,
		shell:       shellPath,
		createdAt:   time.Now(),
		lastActive:  time.Now(),
		rows:        rows,
		cols:        cols,
		cmd:         cmd,
		ptmx:        ptmx,
		done:        make(chan struct{}),
		subscribers: make(map[string]*clientSubscriber),
		history:     make([]byte, 0, m.historyBytes),
		historyMax:  m.historyBytes,
	}

	m.sessions[id] = s
	go m.readLoop(s)
	go m.waitLoop(s)

	return m.snapshotLocked(s), nil
}

// Attach subscribes a client to a session's output stream.
func (m *Multiplexer) Attach(sessionID string) (*ClientAttachment, error) {
	m.mu.RLock()
	s, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("terminal multiplexer session not found")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	clientID := uuid.New().String()
	sub := &clientSubscriber{
		id: clientID,
		ch: make(chan []byte, 256),
	}
	s.subscribers[clientID] = sub
	s.lastActive = time.Now()

	// Push buffered history first for late-joining clients.
	if len(s.history) > 0 {
		buf := append([]byte(nil), s.history...)
		select {
		case sub.ch <- buf:
		default:
		}
	}

	return &ClientAttachment{
		ClientID: clientID,
		Output:   sub.ch,
		Write: func(p []byte) (int, error) {
			return m.Write(sessionID, p)
		},
		Resize: func(rows, cols uint16) error {
			return m.Resize(sessionID, rows, cols)
		},
		Close: func() error {
			return m.Detach(sessionID, clientID)
		},
	}, nil
}

// Detach removes a client subscriber from a session.
func (m *Multiplexer) Detach(sessionID, clientID string) error {
	m.mu.RLock()
	s, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.subscribers[clientID]
	if !ok {
		return nil
	}
	delete(s.subscribers, clientID)
	close(sub.ch)
	s.lastActive = time.Now()
	return nil
}

// Write sends input into the shared PTY.
func (m *Multiplexer) Write(sessionID string, p []byte) (int, error) {
	m.mu.RLock()
	s, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("terminal multiplexer session not found")
	}
	s.mu.Lock()
	s.lastActive = time.Now()
	ptmx := s.ptmx
	s.mu.Unlock()
	if ptmx == nil {
		return 0, io.ErrClosedPipe
	}
	return ptmx.Write(p)
}

// Resize resizes the PTY.
func (m *Multiplexer) Resize(sessionID string, rows, cols uint16) error {
	m.mu.RLock()
	s, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("terminal multiplexer session not found")
	}
	if rows == 0 || cols == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = rows
	s.cols = cols
	s.lastActive = time.Now()
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: rows, Cols: cols})
}

// Snapshot returns current session metadata.
func (m *Multiplexer) Snapshot(sessionID string) (*SessionSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("terminal multiplexer session not found")
	}
	return m.snapshotLocked(s), nil
}

// SnapshotHistory returns buffered output history for reconnects.
func (m *Multiplexer) SnapshotHistory(sessionID string) ([]byte, error) {
	m.mu.RLock()
	s, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("terminal multiplexer session not found")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]byte(nil), s.history...), nil
}

// ListSessions returns all active sessions.
func (m *Multiplexer) ListSessions() []SessionSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]SessionSnapshot, 0, len(m.sessions))
	for _, s := range m.sessions {
		ss := m.snapshotLocked(s)
		if ss != nil {
			out = append(out, *ss)
		}
	}
	return out
}

// CloseSession terminates a PTY and removes the session.
func (m *Multiplexer) CloseSession(sessionID string) error {
	m.mu.Lock()
	s, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return m.closeSessionResources(s)
}

// StartCleanupRoutine removes idle sessions past TTL.
func (m *Multiplexer) StartCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.cleanupExpired()
			}
		}
	}()
}

func (m *Multiplexer) cleanupExpired() {
	now := time.Now()
	var toClose []string

	m.mu.RLock()
	for id, s := range m.sessions {
		s.mu.RLock()
		expired := now.Sub(s.lastActive) > m.sessionTTL
		s.mu.RUnlock()
		if expired {
			toClose = append(toClose, id)
		}
	}
	m.mu.RUnlock()

	for _, id := range toClose {
		_ = m.CloseSession(id)
	}
}

func (m *Multiplexer) readLoop(s *session) {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			s.mu.Lock()
			s.lastActive = time.Now()
			s.history = append(s.history, chunk...)
			if len(s.history) > s.historyMax {
				s.history = append([]byte(nil), s.history[len(s.history)-s.historyMax:]...)
			}
			subs := make([]*clientSubscriber, 0, len(s.subscribers))
			for _, sub := range s.subscribers {
				subs = append(subs, sub)
			}
			s.mu.Unlock()
			for _, sub := range subs {
				select {
				case sub.ch <- chunk:
				default:
				}
			}
		}
		if err != nil {
			break
		}
	}
}

func (m *Multiplexer) waitLoop(s *session) {
	_ = s.cmd.Wait()
	_ = m.CloseSession(s.id)
}

func (m *Multiplexer) snapshotLocked(s *session) *SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &SessionSnapshot{
		ID:         s.id,
		Name:       s.name,
		ProjectID:  s.projectID,
		UserID:     s.userID,
		WorkDir:    s.workDir,
		Shell:      s.shell,
		CreatedAt:  s.createdAt,
		LastActive: s.lastActive,
		Rows:       s.rows,
		Cols:       s.cols,
		Clients:    len(s.subscribers),
		Alive:      s.ptmx != nil,
	}
}

func (m *Multiplexer) closeSessionResources(s *session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.done:
		return nil
	default:
		close(s.done)
	}

	if s.ptmx != nil {
		_ = s.ptmx.Close()
		s.ptmx = nil
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	for id, sub := range s.subscribers {
		delete(s.subscribers, id)
		close(sub.ch)
	}
	return nil
}

func resolveShell(shell string) string {
	if shell == "" {
		if v := os.Getenv("SHELL"); v != "" {
			shell = v
		} else {
			shell = "/bin/bash"
		}
	}
	switch shell {
	case "bash":
		return "/bin/bash"
	case "zsh":
		return "/bin/zsh"
	case "sh":
		return "/bin/sh"
	default:
		if strings.HasPrefix(shell, "/") {
			return shell
		}
		return "/bin/bash"
	}
}
