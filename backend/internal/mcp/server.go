// Package mcp - Model Context Protocol Server Implementation
// Enables APEX.BUILD to act as an MCP server and connect to external MCP tools
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// MCPVersion is the supported MCP protocol version
const MCPVersion = "2024-11-05"

// MessageType defines MCP message types
type MessageType string

const (
	MessageTypeRequest      MessageType = "request"
	MessageTypeResponse     MessageType = "response"
	MessageTypeNotification MessageType = "notification"
)

// Method defines MCP methods
type Method string

const (
	// Lifecycle methods
	MethodInitialize  Method = "initialize"
	MethodInitialized Method = "initialized"
	MethodShutdown    Method = "shutdown"

	// Tool methods
	MethodToolsList Method = "tools/list"
	MethodToolsCall Method = "tools/call"

	// Resource methods
	MethodResourcesList     Method = "resources/list"
	MethodResourcesRead     Method = "resources/read"
	MethodResourcesSubscribe Method = "resources/subscribe"

	// Prompt methods
	MethodPromptsList Method = "prompts/list"
	MethodPromptsGet  Method = "prompts/get"

	// Logging
	MethodLoggingSetLevel Method = "logging/setLevel"

	// Notifications
	NotificationProgress       Method = "notifications/progress"
	NotificationResourceUpdate Method = "notifications/resources/updated"
	NotificationToolUpdate     Method = "notifications/tools/updated"
)

// MCPMessage is the base message structure
type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  Method          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents an error response
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Common error codes
const (
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
)

// ServerCapabilities defines what the server supports
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type LoggingCapability struct{}

// ServerInfo describes the MCP server
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is returned from initialize
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// Tool represents an available tool
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolsListResult contains available tools
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ToolCallParams for calling a tool
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolCallResult from a tool execution
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents content in a result
type ContentBlock struct {
	Type     string `json:"type"` // text, image, resource
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // base64 for binary
	URI      string `json:"uri,omitempty"`
}

// Resource represents a readable resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourcesListResult contains available resources
type ResourcesListResult struct {
	Resources []Resource `json:"resources"`
}

// ResourceReadParams for reading a resource
type ResourceReadParams struct {
	URI string `json:"uri"`
}

// ResourceReadResult contains resource content
type ResourceReadResult struct {
	Contents []ResourceContent `json:"contents"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // base64
}

// Prompt represents a prompt template
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptsListResult contains available prompts
type PromptsListResult struct {
	Prompts []Prompt `json:"prompts"`
}

// PromptGetParams for getting a prompt
type PromptGetParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// PromptGetResult contains rendered prompt
type PromptGetResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

type PromptMessage struct {
	Role    string         `json:"role"` // user, assistant
	Content ContentBlock   `json:"content"`
}

// ToolHandler is a function that handles tool calls
type ToolHandler func(ctx context.Context, arguments map[string]interface{}) (*ToolCallResult, error)

// ResourceHandler provides resource content
type ResourceHandler func(ctx context.Context, uri string) (*ResourceReadResult, error)

// PromptHandler provides prompt content
type PromptHandler func(ctx context.Context, name string, arguments map[string]string) (*PromptGetResult, error)

// MCPServer implements the Model Context Protocol
type MCPServer struct {
	name    string
	version string

	tools         map[string]Tool
	toolHandlers  map[string]ToolHandler
	resources     map[string]Resource
	resourceHandler ResourceHandler
	prompts       map[string]Prompt
	promptHandler PromptHandler

	clients    map[string]*MCPClient
	mu         sync.RWMutex
	upgrader   websocket.Upgrader
	logLevel   string
}

// MCPClient represents a connected client
type MCPClient struct {
	ID            string
	conn          *websocket.Conn
	server        *MCPServer
	initialized   bool
	clientInfo    *ClientInfo
	subscriptions map[string]bool // resource URIs
	mu            sync.Mutex
}

// ClientInfo from initialize request
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// NewMCPServer creates a new MCP server
func NewMCPServer(name, version string) *MCPServer {
	return &MCPServer{
		name:         name,
		version:      version,
		tools:        make(map[string]Tool),
		toolHandlers: make(map[string]ToolHandler),
		resources:    make(map[string]Resource),
		prompts:      make(map[string]Prompt),
		clients:      make(map[string]*MCPClient),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Configure for production
			},
		},
		logLevel: "info",
	}
}

// RegisterTool adds a tool to the server
func (s *MCPServer) RegisterTool(tool Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = tool
	s.toolHandlers[tool.Name] = handler
}

// RegisterResource adds a resource to the server
func (s *MCPServer) RegisterResource(resource Resource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources[resource.URI] = resource
}

// SetResourceHandler sets the handler for resource reads
func (s *MCPServer) SetResourceHandler(handler ResourceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resourceHandler = handler
}

// RegisterPrompt adds a prompt template
func (s *MCPServer) RegisterPrompt(prompt Prompt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts[prompt.Name] = prompt
}

// SetPromptHandler sets the handler for prompt requests
func (s *MCPServer) SetPromptHandler(handler PromptHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.promptHandler = handler
}

// HandleWebSocket handles incoming WebSocket connections
func (s *MCPServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("MCP WebSocket upgrade failed: %v", err)
		return
	}

	client := &MCPClient{
		ID:            uuid.New().String(),
		conn:          conn,
		server:        s,
		subscriptions: make(map[string]bool),
	}

	s.mu.Lock()
	s.clients[client.ID] = client
	s.mu.Unlock()

	log.Printf("MCP client connected: %s", client.ID)

	go client.readLoop()
}

// readLoop handles incoming messages from a client
func (c *MCPClient) readLoop() {
	defer func() {
		c.server.mu.Lock()
		delete(c.server.clients, c.ID)
		c.server.mu.Unlock()
		c.conn.Close()
		log.Printf("MCP client disconnected: %s", c.ID)
	}()

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("MCP read error: %v", err)
			}
			return
		}

		var msg MCPMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.sendError(nil, ErrCodeParse, "Parse error", nil)
			continue
		}

		c.handleMessage(&msg)
	}
}

// handleMessage processes an incoming MCP message
func (c *MCPClient) handleMessage(msg *MCPMessage) {
	ctx := context.Background()

	switch msg.Method {
	case MethodInitialize:
		c.handleInitialize(msg)
	case MethodInitialized:
		// Notification, no response needed
		c.initialized = true
	case MethodShutdown:
		c.sendResult(msg.ID, nil)
	case MethodToolsList:
		c.handleToolsList(msg)
	case MethodToolsCall:
		c.handleToolsCall(ctx, msg)
	case MethodResourcesList:
		c.handleResourcesList(msg)
	case MethodResourcesRead:
		c.handleResourcesRead(ctx, msg)
	case MethodResourcesSubscribe:
		c.handleResourcesSubscribe(msg)
	case MethodPromptsList:
		c.handlePromptsList(msg)
	case MethodPromptsGet:
		c.handlePromptsGet(ctx, msg)
	case MethodLoggingSetLevel:
		c.handleLoggingSetLevel(msg)
	default:
		c.sendError(msg.ID, ErrCodeMethodNotFound, fmt.Sprintf("Method not found: %s", msg.Method), nil)
	}
}

func (c *MCPClient) handleInitialize(msg *MCPMessage) {
	var params struct {
		ProtocolVersion string      `json:"protocolVersion"`
		Capabilities    interface{} `json:"capabilities"`
		ClientInfo      ClientInfo  `json:"clientInfo"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		c.sendError(msg.ID, ErrCodeInvalidParams, "Invalid params", nil)
		return
	}

	c.clientInfo = &params.ClientInfo

	result := InitializeResult{
		ProtocolVersion: MCPVersion,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: true,
			},
			Resources: &ResourcesCapability{
				Subscribe:   true,
				ListChanged: true,
			},
			Prompts: &PromptsCapability{
				ListChanged: true,
			},
			Logging: &LoggingCapability{},
		},
		ServerInfo: ServerInfo{
			Name:    c.server.name,
			Version: c.server.version,
		},
	}

	c.sendResult(msg.ID, result)
}

func (c *MCPClient) handleToolsList(msg *MCPMessage) {
	c.server.mu.RLock()
	tools := make([]Tool, 0, len(c.server.tools))
	for _, tool := range c.server.tools {
		tools = append(tools, tool)
	}
	c.server.mu.RUnlock()

	c.sendResult(msg.ID, ToolsListResult{Tools: tools})
}

func (c *MCPClient) handleToolsCall(ctx context.Context, msg *MCPMessage) {
	var params ToolCallParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		c.sendError(msg.ID, ErrCodeInvalidParams, "Invalid params", nil)
		return
	}

	c.server.mu.RLock()
	handler, exists := c.server.toolHandlers[params.Name]
	c.server.mu.RUnlock()

	if !exists {
		c.sendError(msg.ID, ErrCodeInvalidParams, fmt.Sprintf("Unknown tool: %s", params.Name), nil)
		return
	}

	result, err := handler(ctx, params.Arguments)
	if err != nil {
		c.sendResult(msg.ID, ToolCallResult{
			Content: []ContentBlock{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
		return
	}

	c.sendResult(msg.ID, result)
}

func (c *MCPClient) handleResourcesList(msg *MCPMessage) {
	c.server.mu.RLock()
	resources := make([]Resource, 0, len(c.server.resources))
	for _, res := range c.server.resources {
		resources = append(resources, res)
	}
	c.server.mu.RUnlock()

	c.sendResult(msg.ID, ResourcesListResult{Resources: resources})
}

func (c *MCPClient) handleResourcesRead(ctx context.Context, msg *MCPMessage) {
	var params ResourceReadParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		c.sendError(msg.ID, ErrCodeInvalidParams, "Invalid params", nil)
		return
	}

	c.server.mu.RLock()
	handler := c.server.resourceHandler
	c.server.mu.RUnlock()

	if handler == nil {
		c.sendError(msg.ID, ErrCodeInternal, "No resource handler configured", nil)
		return
	}

	result, err := handler(ctx, params.URI)
	if err != nil {
		c.sendError(msg.ID, ErrCodeInternal, err.Error(), nil)
		return
	}

	c.sendResult(msg.ID, result)
}

func (c *MCPClient) handleResourcesSubscribe(msg *MCPMessage) {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		c.sendError(msg.ID, ErrCodeInvalidParams, "Invalid params", nil)
		return
	}

	c.mu.Lock()
	c.subscriptions[params.URI] = true
	c.mu.Unlock()

	c.sendResult(msg.ID, nil)
}

func (c *MCPClient) handlePromptsList(msg *MCPMessage) {
	c.server.mu.RLock()
	prompts := make([]Prompt, 0, len(c.server.prompts))
	for _, p := range c.server.prompts {
		prompts = append(prompts, p)
	}
	c.server.mu.RUnlock()

	c.sendResult(msg.ID, PromptsListResult{Prompts: prompts})
}

func (c *MCPClient) handlePromptsGet(ctx context.Context, msg *MCPMessage) {
	var params PromptGetParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		c.sendError(msg.ID, ErrCodeInvalidParams, "Invalid params", nil)
		return
	}

	c.server.mu.RLock()
	handler := c.server.promptHandler
	c.server.mu.RUnlock()

	if handler == nil {
		c.sendError(msg.ID, ErrCodeInternal, "No prompt handler configured", nil)
		return
	}

	result, err := handler(ctx, params.Name, params.Arguments)
	if err != nil {
		c.sendError(msg.ID, ErrCodeInternal, err.Error(), nil)
		return
	}

	c.sendResult(msg.ID, result)
}

func (c *MCPClient) handleLoggingSetLevel(msg *MCPMessage) {
	var params struct {
		Level string `json:"level"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		c.sendError(msg.ID, ErrCodeInvalidParams, "Invalid params", nil)
		return
	}

	c.server.mu.Lock()
	c.server.logLevel = params.Level
	c.server.mu.Unlock()

	c.sendResult(msg.ID, nil)
}

// sendResult sends a successful response
func (c *MCPClient) sendResult(id interface{}, result interface{}) {
	resultJSON, _ := json.Marshal(result)
	c.send(&MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultJSON,
	})
}

// sendError sends an error response
func (c *MCPClient) sendError(id interface{}, code int, message string, data interface{}) {
	c.send(&MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	})
}

// send sends a message to the client
func (c *MCPClient) send(msg *MCPMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.conn.WriteJSON(msg); err != nil {
		log.Printf("MCP send error: %v", err)
	}
}

// NotifyResourceUpdate notifies clients of a resource change
func (s *MCPServer) NotifyResourceUpdate(uri string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, client := range s.clients {
		client.mu.Lock()
		subscribed := client.subscriptions[uri]
		client.mu.Unlock()

		if subscribed {
			client.send(&MCPMessage{
				JSONRPC: "2.0",
				Method:  NotificationResourceUpdate,
				Params:  json.RawMessage(fmt.Sprintf(`{"uri":"%s"}`, uri)),
			})
		}
	}
}

// NotifyProgress sends a progress notification
func (c *MCPClient) NotifyProgress(token string, progress, total float64, message string) {
	params := map[string]interface{}{
		"progressToken": token,
		"progress":      progress,
		"total":         total,
	}
	if message != "" {
		params["message"] = message
	}

	paramsJSON, _ := json.Marshal(params)
	c.send(&MCPMessage{
		JSONRPC: "2.0",
		Method:  NotificationProgress,
		Params:  paramsJSON,
	})
}
