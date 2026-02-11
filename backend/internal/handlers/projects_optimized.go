// APEX.BUILD Optimized Project Handlers
// Fixes N+1 queries with proper JOINs, selective column loading, and cursor-based pagination
// Implements Redis caching with 30s TTL

package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"apex-build/internal/cache"
	"apex-build/internal/middleware"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// OptimizedHandler extends Handler with caching support
type OptimizedHandler struct {
	*Handler
	projectCache *cache.ProjectCache
	fileCache    *cache.FileCache
	sessionCache *cache.SessionCache
}

// NewOptimizedHandler creates a new optimized handler with caching
func NewOptimizedHandler(h *Handler, redisCache *cache.RedisCache) *OptimizedHandler {
	return &OptimizedHandler{
		Handler:      h,
		projectCache: cache.NewProjectCache(redisCache),
		fileCache:    cache.NewFileCache(redisCache),
		sessionCache: cache.NewSessionCache(redisCache),
	}
}

// ProjectListItem represents a project in list view with minimal data
type ProjectListItem struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Language    string    `json:"language"`
	Framework   string    `json:"framework"`
	IsPublic    bool      `json:"is_public"`
	IsArchived  bool      `json:"is_archived"`
	FileCount   int       `json:"file_count"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// CursorPagination represents cursor-based pagination info
type CursorPagination struct {
	NextCursor string `json:"next_cursor,omitempty"`
	PrevCursor string `json:"prev_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Limit      int    `json:"limit"`
}

// ProjectCursor represents the cursor for pagination
type ProjectCursor struct {
	ID        uint      `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetProjectsOptimized returns paginated projects with N+1 query fix
// Uses selective column loading and proper JOINs
func (oh *OptimizedHandler) GetProjectsOptimized(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	// Parse pagination params
	limit := parseLimit(c, 20, 100)
	cursor := c.Query("cursor")

	ctx := c.Request.Context()

	// Try cache first for non-cursor requests
	if cursor == "" {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}

		cachedList, err := oh.projectCache.GetProjectList(ctx, userID, page, limit)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{
				"projects": cachedList.Projects,
				"pagination": map[string]interface{}{
					"page":        cachedList.Page,
					"limit":       cachedList.Limit,
					"total":       cachedList.Total,
					"total_pages": cachedList.TotalPages,
					"cached":      true,
					"cached_at":   cachedList.CachedAt,
				},
			})
			return
		}
	}

	// Build optimized query
	projects, pagination, err := oh.fetchProjectsOptimized(ctx, userID, limit, cursor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to fetch projects",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"projects":   projects,
		"pagination": pagination,
	})
}

// fetchProjectsOptimized performs the optimized database query
func (oh *OptimizedHandler) fetchProjectsOptimized(ctx context.Context, userID uint, limit int, cursorStr string) ([]ProjectListItem, *CursorPagination, error) {
	// Parse cursor if provided
	var cursor *ProjectCursor
	if cursorStr != "" {
		cursor = decodeCursor(cursorStr)
	}

	// Build query with selective columns - NO N+1!
	// We use a subquery to get file counts instead of Preload
	query := oh.DB.WithContext(ctx).
		Table("projects").
		Select(`
			projects.id,
			projects.name,
			projects.description,
			projects.language,
			projects.framework,
			projects.is_public,
			projects.is_archived,
			projects.updated_at,
			projects.created_at,
			COALESCE(file_counts.count, 0) as file_count
		`).
		Joins(`LEFT JOIN (
			SELECT project_id, COUNT(*) as count
			FROM files
			WHERE deleted_at IS NULL
			GROUP BY project_id
		) file_counts ON file_counts.project_id = projects.id`).
		Where("projects.owner_id = ?", userID).
		Where("projects.deleted_at IS NULL")

	// Apply cursor-based pagination
	if cursor != nil {
		// For cursor pagination, we use (updated_at, id) as the cursor key
		query = query.Where(
			"(projects.updated_at, projects.id) < (?, ?)",
			cursor.UpdatedAt, cursor.ID,
		)
	}

	// Order by updated_at DESC, id DESC for consistent pagination
	query = query.Order("projects.updated_at DESC, projects.id DESC").
		Limit(limit + 1) // Fetch one extra to check if there's more

	// Execute query
	type projectRow struct {
		ID          uint
		Name        string
		Description string
		Language    string
		Framework   string
		IsPublic    bool
		IsArchived  bool
		UpdatedAt   time.Time
		CreatedAt   time.Time
		FileCount   int
	}

	var rows []projectRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, nil, err
	}

	// Check if there's more data
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit] // Remove the extra row
	}

	// Convert to response format
	projects := make([]ProjectListItem, len(rows))
	for i, row := range rows {
		projects[i] = ProjectListItem{
			ID:          row.ID,
			Name:        row.Name,
			Description: row.Description,
			Language:    row.Language,
			Framework:   row.Framework,
			IsPublic:    row.IsPublic,
			IsArchived:  row.IsArchived,
			FileCount:   row.FileCount,
			UpdatedAt:   row.UpdatedAt,
			CreatedAt:   row.CreatedAt,
		}
	}

	// Build pagination info
	pagination := &CursorPagination{
		HasMore: hasMore,
		Limit:   limit,
	}

	if hasMore && len(projects) > 0 {
		lastProject := projects[len(projects)-1]
		pagination.NextCursor = encodeCursor(&ProjectCursor{
			ID:        lastProject.ID,
			UpdatedAt: lastProject.UpdatedAt,
		})
	}

	return projects, pagination, nil
}

// GetProjectOptimized returns a single project with optimized queries
func (oh *OptimizedHandler) GetProjectOptimized(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectIDStr := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	ctx := c.Request.Context()

	// Try cache first
	cachedProject, err := oh.projectCache.GetProject(ctx, uint(projectID))
	if err == nil {
		// Verify access
		if cachedProject.OwnerID != userID && !cachedProject.IsPublic {
			c.JSON(http.StatusForbidden, StandardResponse{
				Success: false,
				Error:   "Access denied",
				Code:    "ACCESS_DENIED",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"project": cachedProject,
			"cached":  true,
		})
		return
	}

	// Fetch with optimized query
	project, err := oh.fetchProjectOptimized(ctx, uint(projectID), userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Project not found",
				Code:    "PROJECT_NOT_FOUND",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to fetch project",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Cache the result
	cachedProj := projectToCache(project)
	oh.projectCache.SetProject(ctx, cachedProj)

	c.JSON(http.StatusOK, gin.H{
		"project": project,
	})
}

// fetchProjectOptimized fetches a single project with optimized query
func (oh *OptimizedHandler) fetchProjectOptimized(ctx context.Context, projectID, userID uint) (*models.Project, error) {
	var project models.Project

	// Single query with JOIN for file count instead of separate Preload
	err := oh.DB.WithContext(ctx).
		Select(`
			projects.*,
			COALESCE(file_counts.count, 0) as file_count
		`).
		Joins(`LEFT JOIN (
			SELECT project_id, COUNT(*) as count
			FROM files
			WHERE deleted_at IS NULL
			GROUP BY project_id
		) file_counts ON file_counts.project_id = projects.id`).
		Where("projects.id = ?", projectID).
		Where("(projects.owner_id = ? OR projects.is_public = ?)", userID, true).
		First(&project).Error

	if err != nil {
		return nil, err
	}

	return &project, nil
}

// GetProjectFilesOptimized returns project files with optimized query
func (oh *OptimizedHandler) GetProjectFilesOptimized(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectIDStr := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	ctx := c.Request.Context()

	// Check project access first with minimal query
	var projectAccess struct {
		OwnerID  uint
		IsPublic bool
	}
	err = oh.DB.WithContext(ctx).
		Table("projects").
		Select("owner_id, is_public").
		Where("id = ? AND deleted_at IS NULL", projectID).
		Scan(&projectAccess).Error

	if err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	if projectAccess.OwnerID != userID && !projectAccess.IsPublic {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	includeContent := c.Query("include_content")
	if includeContent == "true" || includeContent == "1" {
		var files []models.File
		err = oh.DB.WithContext(ctx).
			Where("project_id = ? AND deleted_at IS NULL", projectID).
			Order("path ASC").
			Find(&files).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Success: false,
				Error:   "Failed to fetch files",
				Code:    "DATABASE_ERROR",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"files":           files,
			"total":           len(files),
			"include_content": true,
			"cached":          false,
		})
		return
	}

	// Try cache
	cachedFiles, err := oh.fileCache.GetFileList(ctx, uint(projectID))
	if err == nil {
		c.JSON(http.StatusOK, gin.H{
			"files":  cachedFiles.Files,
			"total":  cachedFiles.Total,
			"cached": true,
		})
		return
	}

	// Fetch files with selective columns (no content for listing!)
	var files []cache.CachedFile
	err = oh.DB.WithContext(ctx).
		Table("files").
		Select("id, project_id, path, name, type, mime_type, size, version, updated_at").
		Where("project_id = ? AND deleted_at IS NULL", projectID).
		Order("path ASC").
		Scan(&files).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to fetch files",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Cache the result
	fileList := &cache.CachedFileList{
		Files: files,
		Total: len(files),
	}
	oh.fileCache.SetFileList(ctx, uint(projectID), fileList)

	c.JSON(http.StatusOK, gin.H{
		"files": files,
		"total": len(files),
	})
}

// InvalidateProjectCache invalidates cache when a project is modified
func (oh *OptimizedHandler) InvalidateProjectCache(ctx context.Context, userID, projectID uint) {
	oh.projectCache.InvalidateUserProjects(ctx, userID)
	oh.projectCache.InvalidateProject(ctx, projectID)
}

// InvalidateFileCache invalidates file cache when files are modified
func (oh *OptimizedHandler) InvalidateFileCache(ctx context.Context, projectID uint) {
	oh.fileCache.InvalidateFileList(ctx, projectID)
}

// Helper functions

func parseLimit(c *gin.Context, defaultLimit, maxLimit int) int {
	limitStr := c.DefaultQuery("limit", fmt.Sprintf("%d", defaultLimit))
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func encodeCursor(cursor *ProjectCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.URLEncoding.EncodeToString(data)
}

func decodeCursor(encoded string) *ProjectCursor {
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}

	var cursor ProjectCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil
	}

	return &cursor
}

func projectToCache(p *models.Project) *cache.CachedProject {
	return &cache.CachedProject{
		ID:           p.ID,
		Name:         p.Name,
		Description:  p.Description,
		Language:     p.Language,
		Framework:    p.Framework,
		OwnerID:      p.OwnerID,
		IsPublic:     p.IsPublic,
		IsArchived:   p.IsArchived,
		FileCount:    len(p.Files),
		Environment:  p.Environment,
		BuildConfig:  p.BuildConfig,
		Dependencies: p.Dependencies,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

// CreateProjectOptimized creates a project with cache invalidation
func (oh *OptimizedHandler) CreateProjectOptimized(c *gin.Context) {
	// Call the original create
	oh.Handler.CreateProject(c)

	// Invalidate cache on success
	if c.Writer.Status() == http.StatusCreated {
		userID, _ := middleware.GetUserID(c)
		oh.projectCache.InvalidateUserProjects(c.Request.Context(), userID)
	}
}

// UpdateProjectOptimized updates a project with cache invalidation
func (oh *OptimizedHandler) UpdateProjectOptimized(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	projectIDStr := c.Param("id")
	projectID, _ := strconv.ParseUint(projectIDStr, 10, 32)

	// Call the original update
	oh.Handler.UpdateProject(c)

	// Invalidate cache on success
	if c.Writer.Status() == http.StatusOK {
		oh.InvalidateProjectCache(c.Request.Context(), userID, uint(projectID))
	}
}

// DeleteProjectOptimized deletes a project with cache invalidation
func (oh *OptimizedHandler) DeleteProjectOptimized(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	projectIDStr := c.Param("id")
	projectID, _ := strconv.ParseUint(projectIDStr, 10, 32)

	// Call the original delete
	oh.Handler.DeleteProject(c)

	// Invalidate cache on success
	if c.Writer.Status() == http.StatusOK {
		oh.InvalidateProjectCache(c.Request.Context(), userID, uint(projectID))
	}
}
