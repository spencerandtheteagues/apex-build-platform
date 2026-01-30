// Package handlers - MCP Server HTTP Handlers
package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"apex-build/internal/mcp"
	"apex-build/internal/secrets"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// MCPHandler handles MCP server management endpoints
type MCPHandler struct {
	db              *gorm.DB
	server          *mcp.MCPServer
	connManager     *mcp.MCPConnectionManager
	secretsManager  *secrets.SecretsManager
}

// NewMCPHandler creates a new MCP handler
func NewMCPHandler(db *gorm.DB, server *mcp.MCPServer, connManager *mcp.MCPConnectionManager, secretsManager *secrets.SecretsManager) *MCPHandler {
	return &MCPHandler{
		db:             db,
		server:         server,
		connManager:    connManager,
		secretsManager: secretsManager,
	}
}

// AddExternalServerRequest is the request body for adding an external MCP server
type AddExternalServerRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description,omitempty"`
	URL         string  `json:"url" binding:"required"`
	ProjectID   *uint   `json:"project_id,omitempty"`
	AuthType    string  `json:"auth_type"` // none, bearer, api_key, custom
	AuthHeader  string  `json:"auth_header,omitempty"`
	Credential  string  `json:"credential,omitempty"` // Will be encrypted
}

// ListExternalServers returns all configured MCP servers for the user
func (h *MCPHandler) ListExternalServers(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectIDStr := c.Query("project_id")

	var servers []mcp.ExternalMCPServer
	query := h.db.Where("user_id = ?", userID)

	if projectIDStr != "" {
		projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project_id"})
			return
		}
		query = query.Where("project_id = ? OR project_id IS NULL", uint(projectID))
	}

	if err := query.Order("created_at DESC").Find(&servers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch MCP servers"})
		return
	}

	// Enrich with connection status
	type ServerResponse struct {
		mcp.ExternalMCPServer
		Connected  bool          `json:"connected"`
		Tools      []mcp.Tool    `json:"tools,omitempty"`
		Resources  []mcp.Resource `json:"resources,omitempty"`
	}

	response := make([]ServerResponse, len(servers))
	for i, server := range servers {
		response[i] = ServerResponse{
			ExternalMCPServer: server,
		}

		if conn, exists := h.connManager.GetConnection(server.URL); exists {
			response[i].Connected = conn.IsConnected()
			response[i].Tools = conn.GetTools()
			response[i].Resources = conn.GetResources()
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"servers": response,
		"count":   len(response),
	})
}

// AddExternalServer adds a new external MCP server configuration
func (h *MCPHandler) AddExternalServer(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req AddExternalServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check for duplicate
	var existing mcp.ExternalMCPServer
	if h.db.Where("user_id = ? AND url = ?", userID, req.URL).First(&existing).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "MCP server with this URL already exists"})
		return
	}

	server := &mcp.ExternalMCPServer{
		UserID:      userID,
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		Description: req.Description,
		URL:         req.URL,
		AuthType:    req.AuthType,
		AuthHeader:  req.AuthHeader,
		Enabled:     true,
		LastStatus:  "configured",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// If credentials provided, encrypt and store as a secret
	if req.Credential != "" {
		encryptedValue, salt, fingerprint, err := h.secretsManager.Encrypt(userID, req.Credential)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt credentials"})
			return
		}

		credSecret := &secrets.Secret{
			UserID:         userID,
			ProjectID:      req.ProjectID,
			Name:           "mcp_" + req.Name + "_credential",
			Description:    "MCP server credential for " + req.Name,
			Type:           secrets.SecretTypeAPIKey,
			EncryptedValue: encryptedValue,
			Salt:           salt,
			KeyFingerprint: fingerprint,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		if err := h.db.Create(credSecret).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save credentials"})
			return
		}

		server.CredentialSecretID = &credSecret.ID
	}

	if err := h.db.Create(server).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save MCP server"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "MCP server added successfully",
		"server":  server,
	})
}

// ConnectToServer connects to an external MCP server
func (h *MCPHandler) ConnectToServer(c *gin.Context) {
	userID := c.GetUint("user_id")
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	var server mcp.ExternalMCPServer
	if err := h.db.Where("id = ? AND user_id = ?", serverID, userID).First(&server).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}

	// Build headers for authentication
	headers := http.Header{}
	if server.CredentialSecretID != nil {
		var credSecret secrets.Secret
		if err := h.db.First(&credSecret, *server.CredentialSecretID).Error; err == nil {
			credential, err := h.secretsManager.Decrypt(userID, credSecret.EncryptedValue, credSecret.Salt)
			if err == nil {
				headerName := "Authorization"
				if server.AuthHeader != "" {
					headerName = server.AuthHeader
				}

				switch server.AuthType {
				case "bearer":
					headers.Set(headerName, "Bearer "+credential)
				case "api_key":
					headers.Set(headerName, credential)
				default:
					headers.Set(headerName, credential)
				}
			}
		}
	}

	// Connect with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	conn, err := h.connManager.Connect(ctx, strconv.FormatUint(serverID, 10), server.URL, server.Name, headers)
	if err != nil {
		server.LastStatus = "error"
		server.LastError = err.Error()
		h.db.Save(&server)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to connect to MCP server",
			"details": err.Error(),
		})
		return
	}

	// Update server status
	now := time.Now()
	server.LastStatus = "connected"
	server.LastError = ""
	server.LastConnected = &now
	h.db.Save(&server)

	// Return connection info
	c.JSON(http.StatusOK, gin.H{
		"message":     "Connected to MCP server",
		"server_info": conn.GetServerInfo(),
		"tools":       conn.GetTools(),
		"resources":   conn.GetResources(),
		"prompts":     conn.GetPrompts(),
	})
}

// DisconnectFromServer disconnects from an external MCP server
func (h *MCPHandler) DisconnectFromServer(c *gin.Context) {
	userID := c.GetUint("user_id")
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	var server mcp.ExternalMCPServer
	if err := h.db.Where("id = ? AND user_id = ?", serverID, userID).First(&server).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}

	if err := h.connManager.Disconnect(server.URL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disconnect"})
		return
	}

	server.LastStatus = "disconnected"
	h.db.Save(&server)

	c.JSON(http.StatusOK, gin.H{"message": "Disconnected from MCP server"})
}

// CallTool calls a tool on a connected MCP server
func (h *MCPHandler) CallTool(c *gin.Context) {
	userID := c.GetUint("user_id")
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	var req struct {
		ToolName  string                 `json:"tool_name" binding:"required"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var server mcp.ExternalMCPServer
	if err := h.db.Where("id = ? AND user_id = ?", serverID, userID).First(&server).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}

	conn, exists := h.connManager.GetConnection(server.URL)
	if !exists || !conn.IsConnected() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Not connected to MCP server"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	result, err := conn.CallTool(ctx, req.ToolName, req.Arguments)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":   result,
		"is_error": result.IsError,
	})
}

// ReadResource reads a resource from a connected MCP server
func (h *MCPHandler) ReadResource(c *gin.Context) {
	userID := c.GetUint("user_id")
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	uri := c.Query("uri")
	if uri == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URI required"})
		return
	}

	var server mcp.ExternalMCPServer
	if err := h.db.Where("id = ? AND user_id = ?", serverID, userID).First(&server).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}

	conn, exists := h.connManager.GetConnection(server.URL)
	if !exists || !conn.IsConnected() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Not connected to MCP server"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := conn.ReadResource(ctx, uri)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"contents": result.Contents})
}

// DeleteExternalServer removes an MCP server configuration
func (h *MCPHandler) DeleteExternalServer(c *gin.Context) {
	userID := c.GetUint("user_id")
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	var server mcp.ExternalMCPServer
	if err := h.db.Where("id = ? AND user_id = ?", serverID, userID).First(&server).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}

	// Disconnect if connected
	h.connManager.Disconnect(server.URL)

	// Delete associated credential secret
	if server.CredentialSecretID != nil {
		h.db.Delete(&secrets.Secret{}, *server.CredentialSecretID)
	}

	// Delete server
	if err := h.db.Delete(&server).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete server"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "MCP server deleted"})
}

// HandleMCPWebSocket handles incoming MCP WebSocket connections (for APEX.BUILD acting as MCP server)
func (h *MCPHandler) HandleMCPWebSocket(c *gin.Context) {
	h.server.HandleWebSocket(c.Writer, c.Request)
}

// GetAvailableTools returns all tools available across all connected MCP servers
func (h *MCPHandler) GetAvailableTools(c *gin.Context) {
	userID := c.GetUint("user_id")

	var servers []mcp.ExternalMCPServer
	if err := h.db.Where("user_id = ? AND enabled = ?", userID, true).Find(&servers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch servers"})
		return
	}

	type ToolWithServer struct {
		mcp.Tool
		ServerID   uint   `json:"server_id"`
		ServerName string `json:"server_name"`
	}

	var allTools []ToolWithServer
	for _, server := range servers {
		if conn, exists := h.connManager.GetConnection(server.URL); exists && conn.IsConnected() {
			for _, tool := range conn.GetTools() {
				allTools = append(allTools, ToolWithServer{
					Tool:       tool,
					ServerID:   server.ID,
					ServerName: server.Name,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"tools": allTools,
		"count": len(allTools),
	})
}
