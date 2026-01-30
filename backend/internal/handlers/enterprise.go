// APEX.BUILD Enterprise HTTP Handlers
// API endpoints for SSO, SCIM, organizations, audit logs, and RBAC

package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/enterprise"
	"apex-build/internal/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// EnterpriseHandler handles enterprise feature endpoints
type EnterpriseHandler struct {
	db           *gorm.DB
	samlService  *enterprise.SAMLService
	scimService  *enterprise.SCIMService
	auditService *enterprise.AuditService
	rbacService  *enterprise.RBACService
}

// NewEnterpriseHandler creates a new enterprise handler
func NewEnterpriseHandler(
	db *gorm.DB,
	samlService *enterprise.SAMLService,
	scimService *enterprise.SCIMService,
	auditService *enterprise.AuditService,
	rbacService *enterprise.RBACService,
) *EnterpriseHandler {
	return &EnterpriseHandler{
		db:           db,
		samlService:  samlService,
		scimService:  scimService,
		auditService: auditService,
		rbacService:  rbacService,
	}
}

// InitiateSSO starts SAML SSO flow
// GET /api/v1/enterprise/sso/initiate
func (h *EnterpriseHandler) InitiateSSO(c *gin.Context) {
	orgSlug := c.Query("org")
	if orgSlug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization slug required"})
		return
	}

	org, err := h.rbacService.GetOrganizationBySlug(orgSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	if !org.SSOEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SSO not enabled for this organization"})
		return
	}

	relayState := c.Query("relay_state")
	redirectURL, err := h.samlService.GenerateAuthnRequest(org, relayState)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"redirect_url": redirectURL,
	})
}

// HandleSAMLCallback processes SAML response from IdP
// POST /api/v1/enterprise/sso/callback
func (h *EnterpriseHandler) HandleSAMLCallback(c *gin.Context) {
	samlResponse := c.PostForm("SAMLResponse")
	if samlResponse == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing SAML response"})
		return
	}

	relayState := c.PostForm("RelayState")

	// Parse relay state for org info
	orgSlug := c.Query("org")
	if orgSlug == "" && relayState != "" {
		orgSlug = relayState
	}

	if orgSlug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context required"})
		return
	}

	org, err := h.rbacService.GetOrganizationBySlug(orgSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	assertion, err := h.samlService.ProcessSAMLResponse(org, samlResponse)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "SSO authentication failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"assertion": assertion,
		"message":   "SSO authentication successful",
	})
}

// GetSPMetadata returns SAML Service Provider metadata
// GET /api/v1/enterprise/sso/metadata
func (h *EnterpriseHandler) GetSPMetadata(c *gin.Context) {
	metadata, err := h.samlService.GenerateSPMetadata()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/xml", []byte(metadata))
}

// ConfigureSSO configures SSO for an organization
// POST /api/v1/enterprise/organizations/:id/sso
func (h *EnterpriseHandler) ConfigureSSO(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	orgID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	// Check admin permission
	if !h.rbacService.HasPermission(uint(orgID), userID, "organization", "manage") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	org, err := h.rbacService.GetOrganization(uint(orgID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	var config enterprise.SSOConfigRequest
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if err := h.samlService.ConfigureSSO(org, &config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SSO configured successfully",
	})
}

// GetOrganizations returns user's organizations
// GET /api/v1/enterprise/organizations
func (h *EnterpriseHandler) GetOrganizations(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	orgs, err := h.rbacService.GetUserOrganizations(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"organizations": orgs,
	})
}

// GetOrganization returns a specific organization
// GET /api/v1/enterprise/organizations/:id
func (h *EnterpriseHandler) GetOrganization(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	orgID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	if !h.rbacService.HasPermission(uint(orgID), userID, "organization", "read") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	org, err := h.rbacService.GetOrganization(uint(orgID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"organization": org,
	})
}

// GetAuditLogs returns audit logs for an organization
// GET /api/v1/enterprise/organizations/:id/audit-logs
func (h *EnterpriseHandler) GetAuditLogs(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	orgID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	if !h.rbacService.HasPermission(uint(orgID), userID, "audit_logs", "read") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	action := c.Query("action")
	category := c.Query("category")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	logs, total, err := h.auditService.GetAuditLogs(uint(orgID), action, category, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"audit_logs": logs,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
	})
}

// GetRoles returns roles for an organization
// GET /api/v1/enterprise/organizations/:id/roles
func (h *EnterpriseHandler) GetRoles(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	orgID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	if !h.rbacService.HasPermission(uint(orgID), userID, "roles", "read") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	roles, err := h.rbacService.GetRoles(uint(orgID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"roles":   roles,
	})
}

// RegisterEnterpriseRoutes registers all enterprise routes
func (h *EnterpriseHandler) RegisterEnterpriseRoutes(protected *gin.RouterGroup, public *gin.RouterGroup) {
	// Public SSO endpoints (no auth required for SSO flow)
	sso := public.Group("/enterprise/sso")
	{
		sso.GET("/initiate", h.InitiateSSO)
		sso.POST("/callback", h.HandleSAMLCallback)
		sso.GET("/metadata", h.GetSPMetadata)
	}

	// Protected enterprise endpoints
	ent := protected.Group("/enterprise")
	{
		// Organizations
		ent.GET("/organizations", h.GetOrganizations)
		ent.GET("/organizations/:id", h.GetOrganization)
		ent.POST("/organizations/:id/sso", h.ConfigureSSO)
		ent.GET("/organizations/:id/audit-logs", h.GetAuditLogs)
		ent.GET("/organizations/:id/roles", h.GetRoles)
	}

	// SCIM endpoints (authenticated with SCIM token, not JWT)
	if h.scimService != nil {
		h.scimService.RegisterSCIMRoutes(public)
	}
}
