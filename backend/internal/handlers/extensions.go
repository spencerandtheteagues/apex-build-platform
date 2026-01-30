// APEX.BUILD Extensions Marketplace HTTP Handlers
// API endpoints for extension discovery, installation, and management

package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/extensions"
	"apex-build/internal/middleware"

	"github.com/gin-gonic/gin"
)

// ExtensionsHandler handles extension marketplace requests
type ExtensionsHandler struct {
	service *extensions.Service
}

// NewExtensionsHandler creates a new extensions handler
func NewExtensionsHandler(service *extensions.Service) *ExtensionsHandler {
	return &ExtensionsHandler{service: service}
}

// SearchExtensions searches the marketplace
// GET /api/v1/extensions
func (h *ExtensionsHandler) SearchExtensions(c *gin.Context) {
	params := extensions.SearchParams{
		Query:    c.Query("q"),
		SortBy:   c.DefaultQuery("sort", "downloads"),
		SortOrder: c.DefaultQuery("order", "desc"),
	}

	if category := c.Query("category"); category != "" {
		params.Category = extensions.ExtensionCategory(category)
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	params.Page = page
	params.PageSize = pageSize

	if minRating := c.Query("min_rating"); minRating != "" {
		if r, err := strconv.ParseFloat(minRating, 64); err == nil {
			params.MinRating = r
		}
	}

	result, err := h.service.Search(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetExtension returns a single extension
// GET /api/v1/extensions/:id
func (h *ExtensionsHandler) GetExtension(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid extension ID"})
		return
	}

	ext, err := h.service.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"extension": ext,
	})
}

// GetFeatured returns featured extensions
// GET /api/v1/extensions/featured
func (h *ExtensionsHandler) GetFeatured(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	exts, err := h.service.GetFeatured(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"extensions": exts,
	})
}

// GetTrending returns trending extensions
// GET /api/v1/extensions/trending
func (h *ExtensionsHandler) GetTrending(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	exts, err := h.service.GetTrending(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"extensions": exts,
	})
}

// GetCategories returns extension categories
// GET /api/v1/extensions/categories
func (h *ExtensionsHandler) GetCategories(c *gin.Context) {
	categories, err := h.service.GetCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"categories": categories,
	})
}

// GetRecommended returns personalized recommendations
// GET /api/v1/extensions/recommended
func (h *ExtensionsHandler) GetRecommended(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	exts, err := h.service.GetRecommended(c.Request.Context(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"extensions": exts,
	})
}

// InstallExtension installs an extension for the user
// POST /api/v1/extensions/:id/install
func (h *ExtensionsHandler) InstallExtension(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid extension ID"})
		return
	}

	install, err := h.service.Install(c.Request.Context(), userID, uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"installation": install,
		"message":      "Extension installed",
	})
}

// UninstallExtension uninstalls an extension
// DELETE /api/v1/extensions/:id/install
func (h *ExtensionsHandler) UninstallExtension(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid extension ID"})
		return
	}

	if err := h.service.Uninstall(c.Request.Context(), userID, uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Extension uninstalled",
	})
}

// GetMyExtensions returns user's installed extensions
// GET /api/v1/extensions/mine
func (h *ExtensionsHandler) GetMyExtensions(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	installations, err := h.service.GetUserExtensions(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"extensions":   installations,
	})
}

// PublishExtension publishes a new extension
// POST /api/v1/extensions/publish
func (h *ExtensionsHandler) PublishExtension(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	username, _ := c.Get("username")
	authorName := ""
	if username != nil {
		authorName = username.(string)
	}

	var req extensions.PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	ext, err := h.service.Publish(c.Request.Context(), userID, authorName, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":   true,
		"extension": ext,
		"message":   "Extension published (pending review)",
	})
}

// RateExtension adds a review/rating
// POST /api/v1/extensions/:id/rate
func (h *ExtensionsHandler) RateExtension(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid extension ID"})
		return
	}

	var req struct {
		Rating  int    `json:"rating" binding:"required,min=1,max=5"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if err := h.service.RateExtension(c.Request.Context(), userID, uint(id), req.Rating, req.Title, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Review submitted",
	})
}

// GetExtensionReviews returns reviews for an extension
// GET /api/v1/extensions/:id/reviews
func (h *ExtensionsHandler) GetExtensionReviews(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid extension ID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	reviews, total, err := h.service.GetExtensionReviews(c.Request.Context(), uint(id), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"reviews": reviews,
		"total":   total,
	})
}

// RegisterExtensionRoutes registers all extension marketplace routes
func (h *ExtensionsHandler) RegisterExtensionRoutes(rg *gin.RouterGroup) {
	ext := rg.Group("/extensions")
	{
		// Public browsing (still needs auth for user context)
		ext.GET("", h.SearchExtensions)
		ext.GET("/featured", h.GetFeatured)
		ext.GET("/trending", h.GetTrending)
		ext.GET("/categories", h.GetCategories)
		ext.GET("/recommended", h.GetRecommended)
		ext.GET("/:id", h.GetExtension)
		ext.GET("/:id/reviews", h.GetExtensionReviews)

		// User actions
		ext.GET("/mine", h.GetMyExtensions)
		ext.POST("/:id/install", h.InstallExtension)
		ext.DELETE("/:id/install", h.UninstallExtension)
		ext.POST("/:id/rate", h.RateExtension)

		// Publishing
		ext.POST("/publish", h.PublishExtension)
	}
}
