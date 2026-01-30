// APEX.BUILD Community Handlers
// API handlers for project sharing and discovery

package community

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"apex-build/internal/middleware"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CommunityHandler handles all community-related API endpoints
type CommunityHandler struct {
	DB *gorm.DB
}

// NewCommunityHandler creates a new community handler
func NewCommunityHandler(db *gorm.DB) *CommunityHandler {
	return &CommunityHandler{DB: db}
}

// Response types
type StandardResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Code    string      `json:"code,omitempty"`
	Message string      `json:"message,omitempty"`
}

// ========== EXPLORE ENDPOINTS ==========

// GetExplore returns trending and featured projects
func (h *CommunityHandler) GetExplore(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	// Get featured projects
	var featured []FeaturedProject
	h.DB.Preload("Project").Preload("Project.Owner").
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Order("featured_at DESC").
		Limit(6).
		Find(&featured)

	// Get trending projects (by trend score)
	var trendingStats []ProjectStats
	h.DB.Where("trend_score > 0").
		Order("trend_score DESC").
		Limit(12).
		Find(&trendingStats)

	trendingIDs := make([]uint, len(trendingStats))
	for i, s := range trendingStats {
		trendingIDs[i] = s.ProjectID
	}

	var trending []models.Project
	if len(trendingIDs) > 0 {
		h.DB.Preload("Owner").
			Where("id IN ? AND is_public = ?", trendingIDs, true).
			Find(&trending)
	}

	// Get recent public projects
	var recent []models.Project
	h.DB.Preload("Owner").
		Where("is_public = ?", true).
		Order("created_at DESC").
		Limit(12).
		Find(&recent)

	// Get categories
	var categories []ProjectCategory
	h.DB.Order("sort_order ASC").Find(&categories)

	// Enrich projects with stats and user's star status
	trendingWithStats := h.enrichProjects(trending, userID)
	recentWithStats := h.enrichProjects(recent, userID)
	featuredProjects := make([]ProjectWithStats, len(featured))
	for i, f := range featured {
		p := h.enrichProject(f.Project, userID)
		featuredProjects[i] = p
	}

	c.JSON(http.StatusOK, gin.H{
		"featured":   featuredProjects,
		"trending":   trendingWithStats,
		"recent":     recentWithStats,
		"categories": categories,
	})
}

// SearchProjects searches public projects
func (h *CommunityHandler) SearchProjects(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	query := c.Query("q")
	category := c.Query("category")
	language := c.Query("language")
	sortBy := c.DefaultQuery("sort", "trending") // trending, recent, stars, forks
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	// Build query
	dbQuery := h.DB.Model(&models.Project{}).
		Preload("Owner").
		Where("is_public = ?", true)

	// Text search
	if query != "" {
		searchTerm := "%" + strings.ToLower(query) + "%"
		dbQuery = dbQuery.Where(
			"LOWER(name) LIKE ? OR LOWER(description) LIKE ?",
			searchTerm, searchTerm,
		)
	}

	// Category filter
	if category != "" {
		var cat ProjectCategory
		if err := h.DB.Where("slug = ?", category).First(&cat).Error; err == nil {
			subQuery := h.DB.Model(&ProjectCategoryAssignment{}).
				Select("project_id").
				Where("category_id = ?", cat.ID)
			dbQuery = dbQuery.Where("id IN (?)", subQuery)
		}
	}

	// Language filter
	if language != "" {
		dbQuery = dbQuery.Where("language = ?", language)
	}

	// Get total count
	var total int64
	dbQuery.Count(&total)

	// Apply sorting
	switch sortBy {
	case "recent":
		dbQuery = dbQuery.Order("created_at DESC")
	case "stars":
		dbQuery = dbQuery.Joins("LEFT JOIN project_stats ON project_stats.project_id = projects.id").
			Order("COALESCE(project_stats.star_count, 0) DESC")
	case "forks":
		dbQuery = dbQuery.Joins("LEFT JOIN project_stats ON project_stats.project_id = projects.id").
			Order("COALESCE(project_stats.fork_count, 0) DESC")
	default: // trending
		dbQuery = dbQuery.Joins("LEFT JOIN project_stats ON project_stats.project_id = projects.id").
			Order("COALESCE(project_stats.trend_score, 0) DESC")
	}

	// Fetch projects
	var projects []models.Project
	dbQuery.Offset(offset).Limit(limit).Find(&projects)

	// Enrich with stats
	projectsWithStats := h.enrichProjects(projects, userID)

	c.JSON(http.StatusOK, gin.H{
		"projects": projectsWithStats,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetProjectsByCategory returns projects in a category
func (h *CommunityHandler) GetProjectsByCategory(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	slug := c.Param("slug")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	// Find category
	var category ProjectCategory
	if err := h.DB.Where("slug = ?", slug).First(&category).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Category not found",
			Code:    "CATEGORY_NOT_FOUND",
		})
		return
	}

	// Get project IDs in category
	subQuery := h.DB.Model(&ProjectCategoryAssignment{}).
		Select("project_id").
		Where("category_id = ?", category.ID)

	var total int64
	h.DB.Model(&models.Project{}).
		Where("id IN (?) AND is_public = ?", subQuery, true).
		Count(&total)

	var projects []models.Project
	h.DB.Preload("Owner").
		Where("id IN (?) AND is_public = ?", subQuery, true).
		Joins("LEFT JOIN project_stats ON project_stats.project_id = projects.id").
		Order("COALESCE(project_stats.trend_score, 0) DESC").
		Offset(offset).Limit(limit).
		Find(&projects)

	projectsWithStats := h.enrichProjects(projects, userID)

	c.JSON(http.StatusOK, gin.H{
		"category": category,
		"projects": projectsWithStats,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// ========== PROJECT PAGE ENDPOINTS ==========

// GetPublicProject returns a public project page
func (h *CommunityHandler) GetPublicProject(c *gin.Context) {
	username := c.Param("username")
	projectName := c.Param("project")
	userID, authenticated := middleware.GetUserID(c)

	// Find project owner
	var owner models.User
	if err := h.DB.Where("username = ?", username).First(&owner).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "User not found",
			Code:    "USER_NOT_FOUND",
		})
		return
	}

	// Find project
	var project models.Project
	query := h.DB.Preload("Owner").Preload("Files").
		Where("owner_id = ? AND name = ?", owner.ID, projectName)

	// Only allow viewing public projects unless owner
	if !authenticated || userID != owner.ID {
		query = query.Where("is_public = ?", true)
	}

	if err := query.First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	// Record view
	h.recordView(project.ID, userID, c.ClientIP())

	// Get stats
	var stats ProjectStats
	h.DB.FirstOrCreate(&stats, ProjectStats{ProjectID: project.ID})

	// Check if user starred this project
	isStarred := false
	if authenticated {
		var star ProjectStar
		if err := h.DB.Where("user_id = ? AND project_id = ?", userID, project.ID).First(&star).Error; err == nil {
			isStarred = true
		}
	}

	// Check if this is a fork
	var fork ProjectFork
	isFork := false
	var originalID *uint
	if err := h.DB.Where("forked_id = ?", project.ID).First(&fork).Error; err == nil {
		isFork = true
		originalID = &fork.OriginalID
	}

	// Get categories
	var assignments []ProjectCategoryAssignment
	h.DB.Preload("Category").Where("project_id = ?", project.ID).Find(&assignments)
	categories := make([]string, len(assignments))
	for i, a := range assignments {
		categories[i] = a.Category.Slug
	}

	// Get README content if exists
	readmeContent := ""
	for _, file := range project.Files {
		if strings.ToLower(file.Name) == "readme.md" {
			readmeContent = file.Content
			break
		}
	}

	// Get recent comments
	var comments []ProjectComment
	h.DB.Preload("User").Preload("Replies").Preload("Replies.User").
		Where("project_id = ? AND parent_id IS NULL", project.ID).
		Order("created_at DESC").
		Limit(20).
		Find(&comments)

	c.JSON(http.StatusOK, gin.H{
		"project":    project,
		"stats":      stats,
		"is_starred": isStarred,
		"is_fork":    isFork,
		"original_id": originalID,
		"categories": categories,
		"readme":     readmeContent,
		"comments":   comments,
	})
}

// ========== STAR ENDPOINTS ==========

// StarProject adds a star to a project
func (h *CommunityHandler) StarProject(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "Authentication required",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	// Verify project exists and is public
	var project models.Project
	if err := h.DB.Where("id = ? AND is_public = ?", uint(projectID), true).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	// Check if already starred
	var existing ProjectStar
	if err := h.DB.Where("user_id = ? AND project_id = ?", userID, projectID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, StandardResponse{
			Success: false,
			Error:   "Project already starred",
			Code:    "ALREADY_STARRED",
		})
		return
	}

	// Create star
	star := ProjectStar{
		UserID:    userID,
		ProjectID: uint(projectID),
	}
	if err := h.DB.Create(&star).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to star project",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Update stats
	h.updateProjectStats(uint(projectID))
	h.updateUserStats(project.OwnerID)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Project starred successfully",
	})
}

// UnstarProject removes a star from a project
func (h *CommunityHandler) UnstarProject(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "Authentication required",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	// Delete star
	result := h.DB.Where("user_id = ? AND project_id = ?", userID, projectID).Delete(&ProjectStar{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Star not found",
			Code:    "NOT_STARRED",
		})
		return
	}

	// Get project owner to update their stats
	var project models.Project
	if err := h.DB.First(&project, projectID).Error; err == nil {
		h.updateProjectStats(uint(projectID))
		h.updateUserStats(project.OwnerID)
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Star removed successfully",
	})
}

// ========== FORK ENDPOINTS ==========

// ForkProject creates a fork of a project
func (h *CommunityHandler) ForkProject(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "Authentication required",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	// Get original project with files
	var original models.Project
	if err := h.DB.Preload("Files").
		Where("id = ? AND is_public = ?", uint(projectID), true).
		First(&original).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found or not public",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	// Can't fork your own project
	if original.OwnerID == userID {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Cannot fork your own project",
			Code:    "CANNOT_FORK_OWN",
		})
		return
	}

	// Check if user already forked this project
	var existingFork ProjectFork
	if err := h.DB.Where("original_id = ? AND user_id = ?", projectID, userID).First(&existingFork).Error; err == nil {
		c.JSON(http.StatusConflict, StandardResponse{
			Success: false,
			Error:   "You have already forked this project",
			Code:    "ALREADY_FORKED",
			Data:    gin.H{"forked_project_id": existingFork.ForkedID},
		})
		return
	}

	// Create forked project
	forked := models.Project{
		Name:          original.Name,
		Description:   "Forked from " + original.Name,
		Language:      original.Language,
		Framework:     original.Framework,
		OwnerID:       userID,
		IsPublic:      false, // Start as private
		RootDirectory: original.RootDirectory,
		EntryPoint:    original.EntryPoint,
		Environment:   original.Environment,
		Dependencies:  original.Dependencies,
		BuildConfig:   original.BuildConfig,
	}

	if err := h.DB.Create(&forked).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to create fork",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Copy all files
	for _, file := range original.Files {
		newFile := models.File{
			ProjectID: forked.ID,
			Path:      file.Path,
			Name:      file.Name,
			Type:      file.Type,
			MimeType:  file.MimeType,
			Content:   file.Content,
			Size:      file.Size,
		}
		h.DB.Create(&newFile)
	}

	// Create fork record
	fork := ProjectFork{
		OriginalID: uint(projectID),
		ForkedID:   forked.ID,
		UserID:     userID,
	}
	h.DB.Create(&fork)

	// Update stats
	h.updateProjectStats(uint(projectID))
	h.updateUserStats(original.OwnerID)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Project forked successfully",
		"project": forked,
	})
}

// ========== COMMENT ENDPOINTS ==========

// GetComments returns comments for a project
func (h *CommunityHandler) GetComments(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	// Verify project is public
	var project models.Project
	if err := h.DB.Where("id = ? AND is_public = ?", projectID, true).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	var total int64
	h.DB.Model(&ProjectComment{}).
		Where("project_id = ? AND parent_id IS NULL", projectID).
		Count(&total)

	var comments []ProjectComment
	h.DB.Preload("User").Preload("Replies").Preload("Replies.User").
		Where("project_id = ? AND parent_id IS NULL", projectID).
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&comments)

	c.JSON(http.StatusOK, gin.H{
		"comments": comments,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// CreateComment adds a comment to a project
func (h *CommunityHandler) CreateComment(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "Authentication required",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	var req struct {
		Content  string `json:"content" binding:"required,min=1,max=5000"`
		ParentID *uint  `json:"parent_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Verify project is public
	var project models.Project
	if err := h.DB.Where("id = ? AND is_public = ?", projectID, true).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	// If replying, verify parent exists
	if req.ParentID != nil {
		var parent ProjectComment
		if err := h.DB.Where("id = ? AND project_id = ?", *req.ParentID, projectID).First(&parent).Error; err != nil {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Parent comment not found",
				Code:    "PARENT_NOT_FOUND",
			})
			return
		}
	}

	comment := ProjectComment{
		ProjectID: uint(projectID),
		UserID:    userID,
		ParentID:  req.ParentID,
		Content:   req.Content,
	}

	if err := h.DB.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to create comment",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Load user
	h.DB.Preload("User").First(&comment, comment.ID)

	// Update stats
	h.updateProjectStats(uint(projectID))

	c.JSON(http.StatusCreated, gin.H{
		"message": "Comment added",
		"comment": comment,
	})
}

// DeleteComment removes a comment
func (h *CommunityHandler) DeleteComment(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "Authentication required",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	commentID, err := strconv.ParseUint(c.Param("commentId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid comment ID",
			Code:    "INVALID_COMMENT_ID",
		})
		return
	}

	// Find comment
	var comment ProjectComment
	if err := h.DB.First(&comment, commentID).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Comment not found",
			Code:    "COMMENT_NOT_FOUND",
		})
		return
	}

	// Only owner can delete
	if comment.UserID != userID {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Not authorized to delete this comment",
			Code:    "NOT_AUTHORIZED",
		})
		return
	}

	projectID := comment.ProjectID
	h.DB.Delete(&comment)

	// Update stats
	h.updateProjectStats(projectID)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Comment deleted",
	})
}

// ========== USER PROFILE ENDPOINTS ==========

// GetUserProfile returns a public user profile
func (h *CommunityHandler) GetUserProfile(c *gin.Context) {
	username := c.Param("username")
	currentUserID, authenticated := middleware.GetUserID(c)

	var user models.User
	if err := h.DB.Where("username = ?", username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "User not found",
			Code:    "USER_NOT_FOUND",
		})
		return
	}

	// Get or create user stats
	var stats UserStats
	h.DB.FirstOrCreate(&stats, UserStats{UserID: user.ID})

	// Check if current user follows this user
	isFollowing := false
	if authenticated && currentUserID != user.ID {
		var follow UserFollow
		if err := h.DB.Where("follower_id = ? AND following_id = ?", currentUserID, user.ID).First(&follow).Error; err == nil {
			isFollowing = true
		}
	}

	profile := UserPublicProfile{
		ID:          user.ID,
		Username:    user.Username,
		FullName:    user.FullName,
		AvatarURL:   user.AvatarURL,
		JoinedAt:    user.CreatedAt.Format(time.RFC3339),
		UserStats:   &stats,
		IsFollowing: isFollowing,
	}

	c.JSON(http.StatusOK, gin.H{
		"profile": profile,
	})
}

// GetUserProjects returns public projects for a user
func (h *CommunityHandler) GetUserProjects(c *gin.Context) {
	username := c.Param("username")
	currentUserID, authenticated := middleware.GetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	var user models.User
	if err := h.DB.Where("username = ?", username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "User not found",
			Code:    "USER_NOT_FOUND",
		})
		return
	}

	// Build query - show all if viewing own profile, else only public
	query := h.DB.Model(&models.Project{}).Where("owner_id = ?", user.ID)
	if !authenticated || currentUserID != user.ID {
		query = query.Where("is_public = ?", true)
	}

	var total int64
	query.Count(&total)

	var projects []models.Project
	query.Preload("Owner").
		Order("updated_at DESC").
		Offset(offset).Limit(limit).
		Find(&projects)

	projectsWithStats := h.enrichProjects(projects, currentUserID)

	c.JSON(http.StatusOK, gin.H{
		"projects": projectsWithStats,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetUserStarredProjects returns projects a user has starred
func (h *CommunityHandler) GetUserStarredProjects(c *gin.Context) {
	username := c.Param("username")
	currentUserID, _ := middleware.GetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	var user models.User
	if err := h.DB.Where("username = ?", username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "User not found",
			Code:    "USER_NOT_FOUND",
		})
		return
	}

	// Get starred project IDs
	subQuery := h.DB.Model(&ProjectStar{}).Select("project_id").Where("user_id = ?", user.ID)

	var total int64
	h.DB.Model(&models.Project{}).
		Where("id IN (?) AND is_public = ?", subQuery, true).
		Count(&total)

	var projects []models.Project
	h.DB.Preload("Owner").
		Where("id IN (?) AND is_public = ?", subQuery, true).
		Order("updated_at DESC").
		Offset(offset).Limit(limit).
		Find(&projects)

	projectsWithStats := h.enrichProjects(projects, currentUserID)

	c.JSON(http.StatusOK, gin.H{
		"projects": projectsWithStats,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// ========== FOLLOW ENDPOINTS ==========

// FollowUser follows a user
func (h *CommunityHandler) FollowUser(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "Authentication required",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	targetUsername := c.Param("username")

	var target models.User
	if err := h.DB.Where("username = ?", targetUsername).First(&target).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "User not found",
			Code:    "USER_NOT_FOUND",
		})
		return
	}

	// Can't follow yourself
	if target.ID == userID {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Cannot follow yourself",
			Code:    "CANNOT_FOLLOW_SELF",
		})
		return
	}

	// Check if already following
	var existing UserFollow
	if err := h.DB.Where("follower_id = ? AND following_id = ?", userID, target.ID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, StandardResponse{
			Success: false,
			Error:   "Already following this user",
			Code:    "ALREADY_FOLLOWING",
		})
		return
	}

	follow := UserFollow{
		FollowerID:  userID,
		FollowingID: target.ID,
	}

	if err := h.DB.Create(&follow).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to follow user",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Update stats for both users
	h.updateUserStats(userID)
	h.updateUserStats(target.ID)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "User followed successfully",
	})
}

// UnfollowUser unfollows a user
func (h *CommunityHandler) UnfollowUser(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "Authentication required",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	targetUsername := c.Param("username")

	var target models.User
	if err := h.DB.Where("username = ?", targetUsername).First(&target).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "User not found",
			Code:    "USER_NOT_FOUND",
		})
		return
	}

	result := h.DB.Where("follower_id = ? AND following_id = ?", userID, target.ID).Delete(&UserFollow{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Not following this user",
			Code:    "NOT_FOLLOWING",
		})
		return
	}

	// Update stats
	h.updateUserStats(userID)
	h.updateUserStats(target.ID)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "User unfollowed successfully",
	})
}

// GetFollowers returns users following a user
func (h *CommunityHandler) GetFollowers(c *gin.Context) {
	username := c.Param("username")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	var user models.User
	if err := h.DB.Where("username = ?", username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "User not found",
			Code:    "USER_NOT_FOUND",
		})
		return
	}

	var total int64
	h.DB.Model(&UserFollow{}).Where("following_id = ?", user.ID).Count(&total)

	var follows []UserFollow
	h.DB.Preload("Follower").
		Where("following_id = ?", user.ID).
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&follows)

	followers := make([]gin.H, len(follows))
	for i, f := range follows {
		followers[i] = gin.H{
			"id":         f.Follower.ID,
			"username":   f.Follower.Username,
			"full_name":  f.Follower.FullName,
			"avatar_url": f.Follower.AvatarURL,
			"followed_at": f.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"followers": followers,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetFollowing returns users a user is following
func (h *CommunityHandler) GetFollowing(c *gin.Context) {
	username := c.Param("username")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	var user models.User
	if err := h.DB.Where("username = ?", username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "User not found",
			Code:    "USER_NOT_FOUND",
		})
		return
	}

	var total int64
	h.DB.Model(&UserFollow{}).Where("follower_id = ?", user.ID).Count(&total)

	var follows []UserFollow
	h.DB.Preload("Following").
		Where("follower_id = ?", user.ID).
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&follows)

	following := make([]gin.H, len(follows))
	for i, f := range follows {
		following[i] = gin.H{
			"id":         f.Following.ID,
			"username":   f.Following.Username,
			"full_name":  f.Following.FullName,
			"avatar_url": f.Following.AvatarURL,
			"followed_at": f.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"following": following,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// ========== CATEGORY ENDPOINTS ==========

// GetCategories returns all categories
func (h *CommunityHandler) GetCategories(c *gin.Context) {
	var categories []ProjectCategory
	h.DB.Order("sort_order ASC").Find(&categories)

	// Get project counts for each category
	for i := range categories {
		var count int64
		h.DB.Model(&ProjectCategoryAssignment{}).
			Where("category_id = ?", categories[i].ID).
			Count(&count)
		// We'd need to add a field or return differently
	}

	c.JSON(http.StatusOK, gin.H{
		"categories": categories,
	})
}

// SetProjectCategories sets categories for a project (owner only)
func (h *CommunityHandler) SetProjectCategories(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "Authentication required",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	projectID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid project ID",
			Code:    "INVALID_PROJECT_ID",
		})
		return
	}

	var req struct {
		Categories []string `json:"categories" binding:"required"` // Category slugs
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Verify ownership
	var project models.Project
	if err := h.DB.Where("id = ? AND owner_id = ?", projectID, userID).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Project not found or access denied",
			Code:    "PROJECT_NOT_FOUND",
		})
		return
	}

	// Remove existing assignments
	h.DB.Where("project_id = ?", projectID).Delete(&ProjectCategoryAssignment{})

	// Add new assignments (max 3 categories)
	maxCategories := 3
	if len(req.Categories) > maxCategories {
		req.Categories = req.Categories[:maxCategories]
	}

	for _, slug := range req.Categories {
		var cat ProjectCategory
		if err := h.DB.Where("slug = ?", slug).First(&cat).Error; err == nil {
			assignment := ProjectCategoryAssignment{
				ProjectID:  uint(projectID),
				CategoryID: cat.ID,
			}
			h.DB.Create(&assignment)
		}
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Categories updated",
	})
}

// ========== HELPER FUNCTIONS ==========

func (h *CommunityHandler) enrichProjects(projects []models.Project, userID uint) []ProjectWithStats {
	result := make([]ProjectWithStats, len(projects))

	for i, p := range projects {
		result[i] = h.enrichProject(p, userID)
	}

	return result
}

func (h *CommunityHandler) enrichProject(p models.Project, userID uint) ProjectWithStats {
	pws := ProjectWithStats{
		Project: p,
	}

	// Get stats
	var stats ProjectStats
	h.DB.FirstOrCreate(&stats, ProjectStats{ProjectID: p.ID})
	pws.Stats = &stats

	// Check if user starred
	if userID > 0 {
		var star ProjectStar
		if err := h.DB.Where("user_id = ? AND project_id = ?", userID, p.ID).First(&star).Error; err == nil {
			pws.IsStarred = true
		}
	}

	// Check if fork
	var fork ProjectFork
	if err := h.DB.Where("forked_id = ?", p.ID).First(&fork).Error; err == nil {
		pws.IsFork = true
		pws.OriginalID = &fork.OriginalID
	}

	// Get categories
	var assignments []ProjectCategoryAssignment
	h.DB.Preload("Category").Where("project_id = ?", p.ID).Find(&assignments)
	categories := make([]string, len(assignments))
	for i, a := range assignments {
		categories[i] = a.Category.Slug
	}
	pws.Categories = categories

	return pws
}

func (h *CommunityHandler) recordView(projectID uint, userID uint, ip string) {
	// Hash IP for privacy
	hash := sha256.Sum256([]byte(ip))
	ipHash := hex.EncodeToString(hash[:8])

	// Check if viewed recently (within 1 hour) by same user/IP
	oneHourAgo := time.Now().Add(-time.Hour)
	var existing ProjectView
	query := h.DB.Where("project_id = ? AND viewed_at > ?", projectID, oneHourAgo)

	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	} else {
		query = query.Where("ip_hash = ?", ipHash)
	}

	if query.First(&existing).Error == nil {
		return // Already viewed recently
	}

	// Record view
	view := ProjectView{
		ProjectID: projectID,
		IPHash:    ipHash,
		ViewedAt:  time.Now(),
	}
	if userID > 0 {
		view.UserID = &userID
	}
	h.DB.Create(&view)

	// Update view count
	h.DB.Model(&ProjectStats{}).
		Where("project_id = ?", projectID).
		UpdateColumn("view_count", gorm.Expr("view_count + 1"))
}

func (h *CommunityHandler) updateProjectStats(projectID uint) {
	var stats ProjectStats
	h.DB.FirstOrCreate(&stats, ProjectStats{ProjectID: projectID})

	// Count stars
	var starCount int64
	h.DB.Model(&ProjectStar{}).Where("project_id = ?", projectID).Count(&starCount)

	// Count forks
	var forkCount int64
	h.DB.Model(&ProjectFork{}).Where("original_id = ?", projectID).Count(&forkCount)

	// Count comments
	var commentCount int64
	h.DB.Model(&ProjectComment{}).Where("project_id = ?", projectID).Count(&commentCount)

	// Calculate trend score (simplified algorithm)
	// Higher score for recent activity
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)

	var recentStars int64
	h.DB.Model(&ProjectStar{}).
		Where("project_id = ? AND created_at > ?", projectID, weekAgo).
		Count(&recentStars)

	var recentForks int64
	h.DB.Model(&ProjectFork{}).
		Where("original_id = ? AND created_at > ?", projectID, weekAgo).
		Count(&recentForks)

	var recentViews int64
	h.DB.Model(&ProjectView{}).
		Where("project_id = ? AND viewed_at > ?", projectID, weekAgo).
		Count(&recentViews)

	// Trend score formula: stars*3 + forks*5 + views*0.1 + recent_bonus
	trendScore := float64(starCount)*3 + float64(forkCount)*5 + float64(stats.ViewCount)*0.1 +
		float64(recentStars)*10 + float64(recentForks)*15 + float64(recentViews)*0.5

	h.DB.Model(&stats).Updates(ProjectStats{
		StarCount:    int(starCount),
		ForkCount:    int(forkCount),
		CommentCount: int(commentCount),
		TrendScore:   trendScore,
		UpdatedAt:    now,
	})
}

func (h *CommunityHandler) updateUserStats(userID uint) {
	var stats UserStats
	h.DB.FirstOrCreate(&stats, UserStats{UserID: userID})

	// Count followers
	var followerCount int64
	h.DB.Model(&UserFollow{}).Where("following_id = ?", userID).Count(&followerCount)

	// Count following
	var followingCount int64
	h.DB.Model(&UserFollow{}).Where("follower_id = ?", userID).Count(&followingCount)

	// Count public projects
	var projectCount int64
	h.DB.Model(&models.Project{}).Where("owner_id = ? AND is_public = ?", userID, true).Count(&projectCount)

	// Count total stars received
	var totalStars int64
	h.DB.Model(&ProjectStar{}).
		Joins("JOIN projects ON projects.id = project_stars.project_id").
		Where("projects.owner_id = ?", userID).
		Count(&totalStars)

	// Count total forks
	var totalForks int64
	h.DB.Model(&ProjectFork{}).
		Joins("JOIN projects ON projects.id = project_forks.original_id").
		Where("projects.owner_id = ?", userID).
		Count(&totalForks)

	h.DB.Model(&stats).Updates(UserStats{
		FollowerCount:  int(followerCount),
		FollowingCount: int(followingCount),
		ProjectCount:   int(projectCount),
		TotalStars:     int(totalStars),
		TotalForks:     int(totalForks),
		UpdatedAt:      time.Now(),
	})
}

// RegisterRoutes registers all community routes
func (h *CommunityHandler) RegisterRoutes(router *gin.RouterGroup) {
	// Public routes (no auth required for viewing)
	router.GET("/explore", h.GetExplore)
	router.GET("/explore/search", h.SearchProjects)
	router.GET("/explore/categories", h.GetCategories)
	router.GET("/explore/category/:slug", h.GetProjectsByCategory)

	// User profile routes (mostly public)
	router.GET("/users/:username", h.GetUserProfile)
	router.GET("/users/:username/projects", h.GetUserProjects)
	router.GET("/users/:username/starred", h.GetUserStarredProjects)
	router.GET("/users/:username/followers", h.GetFollowers)
	router.GET("/users/:username/following", h.GetFollowing)

	// Project page route (public)
	router.GET("/project/:username/:project", h.GetPublicProject)
	router.GET("/projects/:id/comments", h.GetComments)
}

// RegisterProtectedRoutes registers routes that require authentication
func (h *CommunityHandler) RegisterProtectedRoutes(router *gin.RouterGroup) {
	// Star actions
	router.POST("/projects/:id/star", h.StarProject)
	router.DELETE("/projects/:id/star", h.UnstarProject)

	// Fork action
	router.POST("/projects/:id/fork", h.ForkProject)

	// Comment actions
	router.POST("/projects/:id/comments", h.CreateComment)
	router.DELETE("/projects/:id/comments/:commentId", h.DeleteComment)

	// Follow actions
	router.POST("/users/:username/follow", h.FollowUser)
	router.DELETE("/users/:username/follow", h.UnfollowUser)

	// Project categories (owner only)
	router.PUT("/projects/:id/categories", h.SetProjectCategories)
}
