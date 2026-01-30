// APEX.BUILD Custom Domains API Handlers
// REST API endpoints for domain management

package handlers

import (
	"net/http"

	"apex-build/internal/domains"
	"apex-build/internal/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DomainsHandler handles domain-related API requests
type DomainsHandler struct {
	db            *gorm.DB
	domainService *domains.DomainService
}

// NewDomainsHandler creates a new domains handler
func NewDomainsHandler(db *gorm.DB, domainService *domains.DomainService) *DomainsHandler {
	return &DomainsHandler{
		db:            db,
		domainService: domainService,
	}
}

// AddDomainRequest represents the request to add a custom domain
type AddDomainRequest struct {
	Domain    string `json:"domain" binding:"required"`
	ProjectID uint   `json:"project_id" binding:"required"`
}

// AddDomain adds a new custom domain to a project
// POST /api/v1/domains
func (h *DomainsHandler) AddDomain(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req AddDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Verify user owns the project
	var count int64
	h.db.Table("projects").Where("id = ? AND owner_id = ?", req.ProjectID, userID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to add domains to this project"})
		return
	}

	domain, err := h.domainService.AddDomain(c.Request.Context(), userID, req.ProjectID, req.Domain)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get DNS configuration
	dnsRecords := h.domainService.GetDNSConfiguration(domain)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Domain added successfully",
		"domain":  domain,
		"dns_configuration": gin.H{
			"records":      dnsRecords,
			"instructions": "Add the following DNS records to your domain registrar:",
		},
	})
}

// GetDomains returns all domains for a project
// GET /api/v1/projects/:projectId/domains
func (h *DomainsHandler) GetDomains(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	projectID := c.Param("projectId")

	// Verify user owns the project
	var count int64
	h.db.Table("projects").Where("id = ? AND owner_id = ?", projectID, userID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this project"})
		return
	}

	var projectIDUint uint
	if _, err := c.GetQuery("projectId"); err {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Parse project ID from path parameter
	var domains []domains.CustomDomain
	h.db.Where("project_id = ?", projectID).Find(&domains)

	// Get DNS configuration for each domain
	domainsWithDNS := make([]gin.H, len(domains))
	for i, domain := range domains {
		domainsWithDNS[i] = gin.H{
			"domain":            domain,
			"dns_configuration": h.domainService.GetDNSConfiguration(&domain),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"domains":    domains,
		"project_id": projectIDUint,
	})
}

// GetDomain returns a specific domain by ID
// GET /api/v1/domains/:id
func (h *DomainsHandler) GetDomain(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	domainID := c.Param("id")

	domain, err := h.domainService.GetDomain(domainID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Verify ownership
	if domain.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this domain"})
		return
	}

	// Get DNS configuration
	dnsRecords := h.domainService.GetDNSConfiguration(domain)

	c.JSON(http.StatusOK, gin.H{
		"domain":            domain,
		"dns_configuration": dnsRecords,
	})
}

// VerifyDomain triggers domain verification
// POST /api/v1/domains/:id/verify
func (h *DomainsHandler) VerifyDomain(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	domainID := c.Param("id")

	// Get domain and verify ownership
	domain, err := h.domainService.GetDomain(domainID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if domain.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this domain"})
		return
	}

	// Verify domain
	result, err := h.domainService.VerifyDomain(c.Request.Context(), domainID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get updated domain
	domain, _ = h.domainService.GetDomain(domainID)

	c.JSON(http.StatusOK, gin.H{
		"verification": result,
		"domain":       domain,
	})
}

// DeleteDomain removes a custom domain
// DELETE /api/v1/domains/:id
func (h *DomainsHandler) DeleteDomain(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	domainID := c.Param("id")

	if err := h.domainService.DeleteDomain(c.Request.Context(), domainID, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Domain deleted successfully",
	})
}

// SetPrimaryDomain sets a domain as the primary domain
// POST /api/v1/domains/:id/primary
func (h *DomainsHandler) SetPrimaryDomain(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	domainID := c.Param("id")

	// Get domain and verify ownership
	domain, err := h.domainService.GetDomain(domainID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if domain.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this domain"})
		return
	}

	if domain.Status != domains.StatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Domain must be active to set as primary"})
		return
	}

	if err := h.domainService.SetPrimaryDomain(domain.ProjectID, domainID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Primary domain updated",
	})
}

// RegenerateToken regenerates the verification token for a domain
// POST /api/v1/domains/:id/regenerate-token
func (h *DomainsHandler) RegenerateToken(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	domainID := c.Param("id")

	// Get domain and verify ownership
	domain, err := h.domainService.GetDomain(domainID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if domain.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this domain"})
		return
	}

	domain, err = h.domainService.RegenerateVerificationToken(domainID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get updated DNS configuration
	dnsRecords := h.domainService.GetDNSConfiguration(domain)

	c.JSON(http.StatusOK, gin.H{
		"message":           "Verification token regenerated",
		"domain":            domain,
		"dns_configuration": dnsRecords,
	})
}

// GetSSLStatus returns SSL certificate status for a domain
// GET /api/v1/domains/:id/ssl
func (h *DomainsHandler) GetSSLStatus(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	domainID := c.Param("id")

	// Get domain and verify ownership
	domain, err := h.domainService.GetDomain(domainID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if domain.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this domain"})
		return
	}

	if !domain.SSLEnabled {
		c.JSON(http.StatusOK, gin.H{
			"ssl_enabled": false,
			"message":     "SSL is not enabled for this domain",
		})
		return
	}

	// Get certificate details
	var cert domains.SSLCertificate
	if err := h.db.First(&cert, "id = ?", domain.SSLCertID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ssl_enabled": true,
			"message":     "Certificate information unavailable",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ssl_enabled":   true,
		"issuer":        cert.Issuer,
		"issued_at":     cert.IssuedAt,
		"expires_at":    cert.ExpiresAt,
		"auto_renew":    cert.AutoRenew,
		"serial_number": cert.SerialNumber,
		"fingerprint":   cert.Fingerprint,
	})
}

// RegisterRoutes registers all domain routes
func (h *DomainsHandler) RegisterRoutes(rg *gin.RouterGroup) {
	domains := rg.Group("/domains")
	{
		domains.POST("", h.AddDomain)
		domains.GET("/:id", h.GetDomain)
		domains.DELETE("/:id", h.DeleteDomain)
		domains.POST("/:id/verify", h.VerifyDomain)
		domains.POST("/:id/primary", h.SetPrimaryDomain)
		domains.POST("/:id/regenerate-token", h.RegenerateToken)
		domains.GET("/:id/ssl", h.GetSSLStatus)
	}

	// Project-specific domain routes
	rg.GET("/projects/:projectId/domains", h.GetDomains)
}
