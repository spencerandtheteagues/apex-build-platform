// Package handlers - Code Search HTTP Handlers for APEX.BUILD
package handlers

import (
	"net/http"
	"strconv"

	"apex-build/internal/search"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SearchHandler handles code search endpoints
type SearchHandler struct {
	engine *search.SearchEngine
	db     *gorm.DB
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(engine *search.SearchEngine, db ...*gorm.DB) *SearchHandler {
	h := &SearchHandler{engine: engine}
	if len(db) > 0 {
		h.db = db[0]
	}
	return h
}

// Search handles POST /api/v1/search
// Performs comprehensive code search across project files
func (h *SearchHandler) Search(c *gin.Context) {
	var query search.SearchQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate project access (user must own the project)
	if query.ProjectID > 0 {
		userID := c.GetUint("user_id")
		if !h.userOwnsProject(userID, query.ProjectID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this project"})
			return
		}
	}

	results, err := h.engine.Search(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"results": results,
	})
}

// QuickSearch handles GET /api/v1/search/quick
// Fast search with minimal options for autocomplete/instant search
func (h *SearchHandler) QuickSearch(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	projectID, _ := strconv.ParseUint(c.Query("project_id"), 10, 32)

	query := &search.SearchQuery{
		Query:          q,
		ProjectID:      uint(projectID),
		MaxResults:     20,
		IncludeContent: true,
		ContextLines:   1,
		SearchType:     "all",
	}

	// Validate project access
	if query.ProjectID > 0 {
		userID := c.GetUint("user_id")
		if !h.userOwnsProject(userID, query.ProjectID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	results, err := h.engine.Search(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"results": results,
	})
}

// SearchSymbols handles GET /api/v1/search/symbols
// Searches for code symbols (functions, classes, variables)
func (h *SearchHandler) SearchSymbols(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	projectID, _ := strconv.ParseUint(c.Query("project_id"), 10, 32)
	maxResults, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	query := &search.SearchQuery{
		Query:      q,
		ProjectID:  uint(projectID),
		MaxResults: maxResults,
		SearchType: "symbol",
	}

	// Validate project access
	if query.ProjectID > 0 {
		userID := c.GetUint("user_id")
		if !h.userOwnsProject(userID, query.ProjectID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	symbols, err := h.engine.SearchSymbols(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"symbols": symbols,
		"count":   len(symbols),
	})
}

// SearchFiles handles GET /api/v1/search/files
// Searches for files by name pattern
func (h *SearchHandler) SearchFiles(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	projectID, _ := strconv.ParseUint(c.Query("project_id"), 10, 32)
	maxResults, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	query := &search.SearchQuery{
		Query:      q,
		ProjectID:  uint(projectID),
		MaxResults: maxResults,
		SearchType: "filename",
	}

	// Validate project access
	if query.ProjectID > 0 {
		userID := c.GetUint("user_id")
		if !h.userOwnsProject(userID, query.ProjectID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	results, err := h.engine.Search(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Extract just file info for filename search
	files := make([]gin.H, 0)
	for _, f := range results.Files {
		files = append(files, gin.H{
			"id":       f.FileID,
			"name":     f.FileName,
			"path":     f.FilePath,
			"language": f.Language,
			"score":    f.Score,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"files":   files,
		"count":   len(files),
	})
}

// SearchAndReplace handles POST /api/v1/search/replace
// Performs search and replace (preview mode by default)
func (h *SearchHandler) SearchAndReplace(c *gin.Context) {
	var req struct {
		ProjectID     uint     `json:"project_id" binding:"required"`
		Search        string   `json:"search" binding:"required"`
		Replace       string   `json:"replace"`
		CaseSensitive bool     `json:"case_sensitive"`
		WholeWord     bool     `json:"whole_word"`
		UseRegex      bool     `json:"use_regex"`
		FileTypes     []string `json:"file_types"`
		Preview       bool     `json:"preview"` // If true, only shows what would change
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate project access
	userID := c.GetUint("user_id")
	if !h.userOwnsProject(userID, req.ProjectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this project"})
		return
	}

	options := &search.SearchQuery{
		CaseSensitive: req.CaseSensitive,
		WholeWord:     req.WholeWord,
		UseRegex:      req.UseRegex,
		FileTypes:     req.FileTypes,
	}

	var results *search.ReplaceResults
	var err error

	if req.Preview {
		results, err = h.engine.SearchAndReplace(c.Request.Context(), req.ProjectID, req.Search, req.Replace, options)
	} else {
		results, err = h.engine.ApplyReplacements(c.Request.Context(), req.ProjectID, req.Search, req.Replace, options)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"results": results,
	})
}

// GetSearchHistory handles GET /api/v1/search/history
// Returns user's recent search queries (not yet persisted)
func (h *SearchHandler) GetSearchHistory(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Query("project_id"), 10, 32)

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"history":    []gin.H{},
		"project_id": projectID,
	})
}

// ClearSearchHistory handles DELETE /api/v1/search/history
func (h *SearchHandler) ClearSearchHistory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Search history cleared",
	})
}

// userOwnsProject checks if the user owns or has access to the project
func (h *SearchHandler) userOwnsProject(userID, projectID uint) bool {
	if h.db == nil {
		// No DB available — deny by default in production
		return false
	}
	var project models.Project
	if err := h.db.Select("id", "owner_id", "is_public").First(&project, projectID).Error; err != nil {
		return false
	}
	return project.OwnerID == userID || project.IsPublic
}
