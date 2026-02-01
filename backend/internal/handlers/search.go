// Package handlers - Code Search HTTP Handlers for APEX.BUILD
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"apex-build/internal/search"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

// SearchHandler handles code search endpoints
type SearchHandler struct {
	engine *search.SearchEngine
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(engine *search.SearchEngine) *SearchHandler {
	return &SearchHandler{engine: engine}
}

// Search handles POST /api/v1/search
// Performs comprehensive code search across project files
func (h *SearchHandler) Search(c *gin.Context) {
	var query search.SearchQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetUint("user_id")

	// Validate project access (user must own the project)
	if query.ProjectID > 0 {
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

	// Save search to history (non-blocking, don't fail on error)
	go h.saveSearchHistory(userID, query.ProjectID, query.Query, query.SearchType, results.TotalMatches)

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

	userID := c.GetUint("user_id")
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

	// Save search to history (non-blocking, don't fail on error)
	go h.saveSearchHistory(userID, query.ProjectID, q, "all", results.TotalMatches)

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

	userID := c.GetUint("user_id")
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

	// Save search to history (non-blocking, don't fail on error)
	go h.saveSearchHistory(userID, query.ProjectID, q, "symbol", len(symbols))

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

	userID := c.GetUint("user_id")
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

	// Save search to history (non-blocking, don't fail on error)
	go h.saveSearchHistory(userID, query.ProjectID, q, "filename", len(files))

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
// Returns user's recent search queries (limited to 50 entries)
func (h *SearchHandler) GetSearchHistory(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectID, _ := strconv.ParseUint(c.Query("project_id"), 10, 32)

	db := h.engine.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
		return
	}

	var history []models.SearchHistory

	// Build query - filter by user and optionally by project
	query := db.Where("user_id = ?", userID)
	if projectID > 0 {
		query = query.Where("project_id = ?", uint(projectID))
	}

	// Get last 50 entries, ordered by timestamp descending
	if err := query.Order("timestamp DESC").Limit(50).Find(&history).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch search history"})
		return
	}

	// Convert to response format
	historyResponse := make([]gin.H, 0, len(history))
	for _, h := range history {
		entry := gin.H{
			"id":           h.ID,
			"query":        h.Query,
			"search_type":  h.SearchType,
			"timestamp":    h.Timestamp,
			"result_count": h.ResultCount,
		}
		if h.ProjectID != nil {
			entry["project_id"] = *h.ProjectID
		}
		historyResponse = append(historyResponse, entry)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"history": historyResponse,
		"count":   len(historyResponse),
	})
}

// ClearSearchHistory handles DELETE /api/v1/search/history
// Clears user's search history (optionally filtered by project)
func (h *SearchHandler) ClearSearchHistory(c *gin.Context) {
	userID := c.GetUint("user_id")
	projectID, _ := strconv.ParseUint(c.Query("project_id"), 10, 32)

	db := h.engine.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
		return
	}

	// Build delete query - filter by user and optionally by project
	query := db.Where("user_id = ?", userID)
	if projectID > 0 {
		query = query.Where("project_id = ?", uint(projectID))
	}

	result := query.Delete(&models.SearchHistory{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear search history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Search history cleared",
		"deleted_count": result.RowsAffected,
	})
}

// saveSearchHistory saves a search query to the user's history
// This is called asynchronously to avoid blocking search responses
func (h *SearchHandler) saveSearchHistory(userID uint, projectID uint, query string, searchType string, resultCount int) {
	// Skip empty queries
	if query == "" {
		return
	}

	db := h.engine.GetDB()
	if db == nil {
		return
	}

	// Create the history entry
	var projectIDPtr *uint
	if projectID > 0 {
		projectIDPtr = &projectID
	}

	// Set default search type if not provided
	if searchType == "" {
		searchType = "all"
	}

	history := models.SearchHistory{
		UserID:      userID,
		ProjectID:   projectIDPtr,
		Query:       query,
		SearchType:  searchType,
		Timestamp:   time.Now(),
		ResultCount: resultCount,
	}

	// Save to database
	if err := db.Create(&history).Error; err != nil {
		// Log error but don't fail - history is not critical
		return
	}

	// Enforce limit of 50 entries per user - delete oldest entries if over limit
	h.pruneSearchHistory(userID)
}

// pruneSearchHistory removes old search history entries to maintain the 50 entry limit
func (h *SearchHandler) pruneSearchHistory(userID uint) {
	db := h.engine.GetDB()
	if db == nil {
		return
	}

	const maxHistoryEntries = 50

	// Count entries for user
	var count int64
	db.Model(&models.SearchHistory{}).Where("user_id = ?", userID).Count(&count)

	if count <= maxHistoryEntries {
		return
	}

	// Find the ID threshold - get the 50th newest entry's ID
	var threshold models.SearchHistory
	if err := db.Where("user_id = ?", userID).
		Order("timestamp DESC").
		Offset(maxHistoryEntries - 1).
		Limit(1).
		First(&threshold).Error; err != nil {
		return
	}

	// Delete entries older than the threshold
	db.Where("user_id = ? AND timestamp < ?", userID, threshold.Timestamp).
		Delete(&models.SearchHistory{})
}

// userOwnsProject checks if a user owns a project or has access to it
// Returns true if:
// - User is the owner of the project
// - Project is public
// - User is an admin (checked via is_admin or is_super_admin flags)
func (h *SearchHandler) userOwnsProject(userID, projectID uint) bool {
	db := h.engine.GetDB()
	if db == nil {
		// If no database, deny access for security
		return false
	}

	// First check the project
	var project models.Project
	result := db.Select("owner_id", "is_public").First(&project, projectID)
	if result.Error != nil {
		// Project not found or error - deny access
		return false
	}

	// Allow access if project is public
	if project.IsPublic {
		return true
	}

	// Allow access if user is the owner
	if project.OwnerID == userID {
		return true
	}

	// Check if user is an admin
	var user models.User
	result = db.Select("is_admin", "is_super_admin").First(&user, userID)
	if result.Error != nil {
		return false
	}

	// Allow access if user is admin or super admin
	return user.IsAdmin || user.IsSuperAdmin
}
