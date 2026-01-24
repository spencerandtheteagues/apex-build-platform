// Package mcp - MCP Client for connecting to external MCP servers
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// MCPClientConnection represents a connection to an external MCP server
type MCPClientConnection struct {
	ID              string
	URL             string
	Name            string
	conn            *websocket.Conn
	serverInfo      *ServerInfo
	capabilities    *ServerCapabilities
	tools           []Tool
	resources       []Resource
	prompts         []Prompt

	requestID       int64
	pending         map[int64]chan *MCPMessage
	mu              sync.RWMutex
	connected       bool
	reconnecting    bool
	lastError       error
	onToolsChanged  func([]Tool)
	onResourcesChanged func([]Resource)
}

// ExternalMCPServer represents a configured external MCP server
type ExternalMCPServer struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	UserID      uint      `json:"user_id" gorm:"index;not null"`
	ProjectID   *uint     `json:"project_id,omitempty" gorm:"index"`
	Name        string    `json:"name" gorm:"not null"`
	Description string    `json:"description,omitempty"`
	URL         string    `json:"url" gorm:"not null"`
	AuthType    string    `json:"auth_type"` // none, bearer, api_key, custom
	AuthHeader  string    `json:"auth_header,omitempty"` // Header name for auth
	// Credentials stored encrypted via secrets manager
	CredentialSecretID *uint     `json:"credential_secret_id,omitempty"`
	Enabled     bool      `json:"enabled" gorm:"default:true"`
	LastStatus  string    `json:"last_status"` // connected, disconnected, error
	LastError   string    `json:"last_error,omitempty"`
	LastConnected *time.Time `json:"last_connected,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MCPConnectionManager manages multiple MCP connections
type MCPConnectionManager struct {
	connections map[string]*MCPClientConnection // URL -> connection
	mu          sync.RWMutex
}

// NewMCPConnectionManager creates a new connection manager
func NewMCPConnectionManager() *MCPConnectionManager {
	return &MCPConnectionManager{
		connections: make(map[string]*MCPClientConnection),
	}
}

// Connect establishes a connection to an external MCP server
func (m *MCPConnectionManager) Connect(ctx context.Context, id, url, name string, headers http.Header) (*MCPClientConnection, error) {
	m.mu.Lock()
	if existing, exists := m.connections[url]; exists && existing.connected {
		m.mu.Unlock()
		return existing, nil
	}
	m.mu.Unlock()

	// Create WebSocket dialer with headers
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	client := &MCPClientConnection{
		ID:        id,
		URL:       url,
		Name:      name,
		conn:      conn,
		pending:   make(map[int64]chan *MCPMessage),
		connected: true,
	}

	m.mu.Lock()
	m.connections[url] = client
	m.mu.Unlock()

	// Start reading messages
	go client.readLoop()

	// Initialize the connection
	if err := client.Initialize(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("MCP initialization failed: %w", err)
	}

	log.Printf("Connected to MCP server: %s (%s)", name, url)
	return client, nil
}

// GetConnection retrieves an existing connection
func (m *MCPConnectionManager) GetConnection(url string) (*MCPClientConnection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conn, exists := m.connections[url]
	return conn, exists && conn.connected
}

// Disconnect closes a connection
func (m *MCPConnectionManager) Disconnect(url string) error {
	m.mu.Lock()
	conn, exists := m.connections[url]
	if exists {
		delete(m.connections, url)
	}
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("connection not found: %s", url)
	}

	return conn.Close()
}

// ListConnections returns all active connections
func (m *MCPConnectionManager) ListConnections() []*MCPClientConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	connections := make([]*MCPClientConnection, 0, len(m.connections))
	for _, conn := range m.connections {
		connections = append(connections, conn)
	}
	return connections
}

// Initialize sends the initialize request
func (c *MCPClientConnection) Initialize(ctx context.Context) error {
	params := map[string]interface{}{
		"protocolVersion": MCPVersion,
		"capabilities": map[string]interface{}{
			"roots": map[string]interface{}{
				"listChanged": true,
			},
		},
		"clientInfo": map[string]string{
			"name":    "APEX.BUILD",
			"version": "1.0.0",
		},
	}

	result, err := c.Request(ctx, MethodInitialize, params)
	if err != nil {
		return err
	}

	var initResult InitializeResult
	if err := json.Unmarshal(result.Result, &initResult); err != nil {
		return fmt.Errorf("failed to parse initialize result: %w", err)
	}

	c.mu.Lock()
	c.serverInfo = &initResult.ServerInfo
	c.capabilities = &initResult.Capabilities
	c.mu.Unlock()

	// Send initialized notification
	c.Notify(MethodInitialized, nil)

	// Fetch available tools, resources, and prompts
	go c.refreshCapabilities(ctx)

	return nil
}

// refreshCapabilities fetches tools, resources, and prompts from the server
func (c *MCPClientConnection) refreshCapabilities(ctx context.Context) {
	// Fetch tools
	if c.capabilities != nil && c.capabilities.Tools != nil {
		if result, err := c.Request(ctx, MethodToolsList, nil); err == nil {
			var toolsResult ToolsListResult
			if json.Unmarshal(result.Result, &toolsResult) == nil {
				c.mu.Lock()
				c.tools = toolsResult.Tools
				c.mu.Unlock()
				if c.onToolsChanged != nil {
					c.onToolsChanged(toolsResult.Tools)
				}
			}
		}
	}

	// Fetch resources
	if c.capabilities != nil && c.capabilities.Resources != nil {
		if result, err := c.Request(ctx, MethodResourcesList, nil); err == nil {
			var resourcesResult ResourcesListResult
			if json.Unmarshal(result.Result, &resourcesResult) == nil {
				c.mu.Lock()
				c.resources = resourcesResult.Resources
				c.mu.Unlock()
				if c.onResourcesChanged != nil {
					c.onResourcesChanged(resourcesResult.Resources)
				}
			}
		}
	}

	// Fetch prompts
	if c.capabilities != nil && c.capabilities.Prompts != nil {
		if result, err := c.Request(ctx, MethodPromptsList, nil); err == nil {
			var promptsResult PromptsListResult
			if json.Unmarshal(result.Result, &promptsResult) == nil {
				c.mu.Lock()
				c.prompts = promptsResult.Prompts
				c.mu.Unlock()
			}
		}
	}
}

// Request sends a request and waits for a response
func (c *MCPClientConnection) Request(ctx context.Context, method Method, params interface{}) (*MCPMessage, error) {
	id := atomic.AddInt64(&c.requestID, 1)

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	msg := &MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsJSON,
	}

	// Create response channel
	respChan := make(chan *MCPMessage, 1)
	c.mu.Lock()
	c.pending[id] = respChan
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	// Send the request
	if err := c.conn.WriteJSON(msg); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timeout")
	}
}

// Notify sends a notification (no response expected)
func (c *MCPClientConnection) Notify(method Method, params interface{}) error {
	var paramsJSON json.RawMessage
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	msg := &MCPMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
	}

	return c.conn.WriteJSON(msg)
}

// CallTool calls a tool on the remote server
func (c *MCPClientConnection) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*ToolCallResult, error) {
	params := ToolCallParams{
		Name:      name,
		Arguments: arguments,
	}

	result, err := c.Request(ctx, MethodToolsCall, params)
	if err != nil {
		return nil, err
	}

	var toolResult ToolCallResult
	if err := json.Unmarshal(result.Result, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	return &toolResult, nil
}

// ReadResource reads a resource from the remote server
func (c *MCPClientConnection) ReadResource(ctx context.Context, uri string) (*ResourceReadResult, error) {
	params := ResourceReadParams{URI: uri}

	result, err := c.Request(ctx, MethodResourcesRead, params)
	if err != nil {
		return nil, err
	}

	var readResult ResourceReadResult
	if err := json.Unmarshal(result.Result, &readResult); err != nil {
		return nil, fmt.Errorf("failed to parse resource result: %w", err)
	}

	return &readResult, nil
}

// GetPrompt gets a rendered prompt from the remote server
func (c *MCPClientConnection) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*PromptGetResult, error) {
	params := PromptGetParams{
		Name:      name,
		Arguments: arguments,
	}

	result, err := c.Request(ctx, MethodPromptsGet, params)
	if err != nil {
		return nil, err
	}

	var promptResult PromptGetResult
	if err := json.Unmarshal(result.Result, &promptResult); err != nil {
		return nil, fmt.Errorf("failed to parse prompt result: %w", err)
	}

	return &promptResult, nil
}

// GetTools returns available tools
func (c *MCPClientConnection) GetTools() []Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tools
}

// GetResources returns available resources
func (c *MCPClientConnection) GetResources() []Resource {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.resources
}

// GetPrompts returns available prompts
func (c *MCPClientConnection) GetPrompts() []Prompt {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.prompts
}

// GetServerInfo returns server information
func (c *MCPClientConnection) GetServerInfo() *ServerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverInfo
}

// IsConnected returns connection status
func (c *MCPClientConnection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// readLoop handles incoming messages
func (c *MCPClientConnection) readLoop() {
	defer func() {
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
	}()

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("MCP client read error: %v", err)
			}
			c.mu.Lock()
			c.lastError = err
			c.mu.Unlock()
			return
		}

		var msg MCPMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("MCP parse error: %v", err)
			continue
		}

		// Handle response
		if msg.ID != nil {
			if id, ok := msg.ID.(float64); ok {
				c.mu.RLock()
				respChan, exists := c.pending[int64(id)]
				c.mu.RUnlock()

				if exists {
					respChan <- &msg
				}
			}
			continue
		}

		// Handle notification
		c.handleNotification(&msg)
	}
}

// handleNotification processes incoming notifications
func (c *MCPClientConnection) handleNotification(msg *MCPMessage) {
	switch msg.Method {
	case NotificationToolUpdate:
		// Refresh tools list
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if result, err := c.Request(ctx, MethodToolsList, nil); err == nil {
				var toolsResult ToolsListResult
				if json.Unmarshal(result.Result, &toolsResult) == nil {
					c.mu.Lock()
					c.tools = toolsResult.Tools
					c.mu.Unlock()
					if c.onToolsChanged != nil {
						c.onToolsChanged(toolsResult.Tools)
					}
				}
			}
		}()

	case NotificationResourceUpdate:
		// Handle resource update
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(msg.Params, &params); err == nil {
			if c.onResourcesChanged != nil {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if result, err := c.Request(ctx, MethodResourcesList, nil); err == nil {
						var resourcesResult ResourcesListResult
						if json.Unmarshal(result.Result, &resourcesResult) == nil {
							c.mu.Lock()
							c.resources = resourcesResult.Resources
							c.mu.Unlock()
							c.onResourcesChanged(resourcesResult.Resources)
						}
					}
				}()
			}
		}

	case NotificationProgress:
		// Log progress for now
		var params struct {
			Token    string  `json:"progressToken"`
			Progress float64 `json:"progress"`
			Total    float64 `json:"total"`
			Message  string  `json:"message"`
		}
		if json.Unmarshal(msg.Params, &params) == nil {
			log.Printf("MCP progress [%s]: %.0f/%.0f - %s", params.Token, params.Progress, params.Total, params.Message)
		}
	}
}

// Close closes the connection
func (c *MCPClientConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false

	// Send shutdown if possible
	if c.conn != nil {
		c.conn.WriteJSON(&MCPMessage{
			JSONRPC: "2.0",
			ID:      atomic.AddInt64(&c.requestID, 1),
			Method:  MethodShutdown,
		})
		return c.conn.Close()
	}
	return nil
}

// OnToolsChanged sets a callback for when tools change
func (c *MCPClientConnection) OnToolsChanged(callback func([]Tool)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onToolsChanged = callback
}

// OnResourcesChanged sets a callback for when resources change
func (c *MCPClientConnection) OnResourcesChanged(callback func([]Resource)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onResourcesChanged = callback
}
