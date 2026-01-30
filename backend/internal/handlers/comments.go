// APEX.BUILD Code Comments Handler
// Inline code comments and threads for collaboration (Replit parity feature)

package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"apex-build/internal/middleware"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CommentsHandler handles code comment operations
type CommentsHandler struct {
	DB *gorm.DB
}

// NewCommentsHandler creates a new comments handler
func NewCommentsHandler(db *gorm.DB) *CommentsHandler {
	return &CommentsHandler{DB: db}
}

// CreateCommentRequest represents a request to create a comment
type CreateCommentRequest struct {
	FileID      uint   `json:"file_id" binding:"required"`
	ProjectID   uint   `json:"project_id" binding:"required"`
	StartLine   int    `json:"start_line" binding:"required,min=1"`
	EndLine     int    `json:"end_line" binding:"required,min=1"`
	StartColumn int    `json:"start_column"`
	EndColumn   int    `json:"end_column"`
	Content     string `json:"content" binding:"required,min=1"`
	ParentID    *uint  `json:"parent_id"` // For replies
	ThreadID    string `json:"thread_id"` // Existing thread ID for replies
}

// UpdateCommentRequest represents a request to update a comment
type UpdateCommentRequest struct {
	Content string `json:"content" binding:"required,min=1"`
}

// ReactionRequest represents a request to add/remove a reaction
type ReactionRequest struct {
	Emoji string `json:"emoji" binding:"required"`
}

// CommentResponse represents a comment in API responses
type CommentResponse struct {
	ID           uint                  `json:"id"`
	FileID       uint                  `json:"file_id"`
	ProjectID    uint                  `json:"project_id"`
	StartLine    int                   `json:"start_line"`
	EndLine      int                   `json:"end_line"`
	StartColumn  int                   `json:"start_column"`
	EndColumn    int                   `json:"end_column"`
	Content      string                `json:"content"`
	ParentID     *uint                 `json:"parent_id,omitempty"`
	ThreadID     string                `json:"thread_id"`
	AuthorID     uint                  `json:"author_id"`
	AuthorName   string                `json:"author_name"`
	IsResolved   bool                  `json:"is_resolved"`
	ResolvedAt   *time.Time            `json:"resolved_at,omitempty"`
	ResolvedByID *uint                 `json:"resolved_by_id,omitempty"`
	Reactions    map[string][]uint     `json:"reactions,omitempty"`
	Replies      []CommentResponse     `json:"replies,omitempty"`
	ReplyCount   int                   `json:"reply_count"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

// ThreadResponse represents a comment thread
type ThreadResponse struct {
	ThreadID     string            `json:"thread_id"`
	FileID       uint              `json:"file_id"`
	ProjectID    uint              `json:"project_id"`
	StartLine    int               `json:"start_line"`
	EndLine      int               `json:"end_line"`
	IsResolved   bool              `json:"is_resolved"`
	CommentCount int               `json:"comment_count"`
	Comments     []CommentResponse `json:"comments"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// RegisterCommentRoutes registers comment-related routes
func (ch *CommentsHandler) RegisterCommentRoutes(rg *gin.RouterGroup) {
	comments := rg.Group("/comments")
	{
		comments.POST("", ch.CreateComment)
		comments.GET("/file/:fileId", ch.GetFileComments)
		comments.GET("/thread/:threadId", ch.GetThread)
		comments.GET("/:id", ch.GetComment)
		comments.PUT("/:id", ch.UpdateComment)
		comments.DELETE("/:id", ch.DeleteComment)
		comments.POST("/:id/resolve", ch.ResolveThread)
		comments.POST("/:id/unresolve", ch.UnresolveThread)
		comments.POST("/:id/react", ch.AddReaction)
		comments.DELETE("/:id/react", ch.RemoveReaction)
	}
}

// CreateComment creates a new comment or reply
func (ch *CommentsHandler) CreateComment(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format: " + err.Error(),
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Validate line range
	if req.EndLine < req.StartLine {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "End line must be greater than or equal to start line",
			Code:    "INVALID_LINE_RANGE",
		})
		return
	}

	// Verify file exists and user has access
	var file models.File
	if err := ch.DB.Preload("Project").First(&file, req.FileID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "File not found",
				Code:    "FILE_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Check access to the project
	if file.Project.OwnerID != userID && !file.Project.IsPublic {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Get user info for author name
	var user models.User
	if err := ch.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to get user info",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Generate or use existing thread ID
	threadID := req.ThreadID
	if threadID == "" {
		threadID = uuid.New().String()
	}

	// If this is a reply, verify parent comment exists
	if req.ParentID != nil {
		var parentComment models.CodeComment
		if err := ch.DB.First(&parentComment, *req.ParentID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, StandardResponse{
					Success: false,
					Error:   "Parent comment not found",
					Code:    "PARENT_NOT_FOUND",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, StandardResponse{
				Success: false,
				Error:   "Database error",
				Code:    "DATABASE_ERROR",
			})
			return
		}
		// Use the parent's thread ID for replies
		threadID = parentComment.ThreadID
	}

	// Create the comment
	comment := models.CodeComment{
		FileID:      req.FileID,
		ProjectID:   req.ProjectID,
		StartLine:   req.StartLine,
		EndLine:     req.EndLine,
		StartColumn: req.StartColumn,
		EndColumn:   req.EndColumn,
		Content:     req.Content,
		ParentID:    req.ParentID,
		ThreadID:    threadID,
		AuthorID:    userID,
		AuthorName:  user.Username,
		Reactions:   make(map[string][]uint),
	}

	if err := ch.DB.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to create comment",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusCreated, StandardResponse{
		Success: true,
		Message: "Comment created successfully",
		Data:    commentToResponse(&comment, nil),
	})
}

// GetFileComments returns all comments for a file
func (ch *CommentsHandler) GetFileComments(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	fileIDStr := c.Param("fileId")
	fileID, err := strconv.ParseUint(fileIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid file ID",
			Code:    "INVALID_FILE_ID",
		})
		return
	}

	// Verify file exists and user has access
	var file models.File
	if err := ch.DB.Preload("Project").First(&file, uint(fileID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "File not found",
				Code:    "FILE_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Check access
	if file.Project.OwnerID != userID && !file.Project.IsPublic {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Parse query params
	includeResolved := c.DefaultQuery("include_resolved", "true") == "true"
	lineNumber, _ := strconv.Atoi(c.Query("line"))

	// Build query for root comments (no parent)
	query := ch.DB.Where("file_id = ? AND parent_id IS NULL", uint(fileID))

	if !includeResolved {
		query = query.Where("is_resolved = ?", false)
	}

	if lineNumber > 0 {
		// Get comments that span the specified line
		query = query.Where("start_line <= ? AND end_line >= ?", lineNumber, lineNumber)
	}

	var rootComments []models.CodeComment
	if err := query.Order("start_line ASC, created_at ASC").Find(&rootComments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to fetch comments",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Get all replies for these root comments
	threadIDs := make([]string, len(rootComments))
	for i, comment := range rootComments {
		threadIDs[i] = comment.ThreadID
	}

	var allReplies []models.CodeComment
	if len(threadIDs) > 0 {
		ch.DB.Where("thread_id IN ? AND parent_id IS NOT NULL", threadIDs).
			Order("created_at ASC").
			Find(&allReplies)
	}

	// Group replies by thread ID
	repliesByThread := make(map[string][]models.CodeComment)
	for _, reply := range allReplies {
		repliesByThread[reply.ThreadID] = append(repliesByThread[reply.ThreadID], reply)
	}

	// Build response
	responses := make([]CommentResponse, len(rootComments))
	for i, comment := range rootComments {
		replies := repliesByThread[comment.ThreadID]
		responses[i] = *commentToResponse(&comment, replies)
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: map[string]interface{}{
			"file_id":  uint(fileID),
			"comments": responses,
			"total":    len(responses),
		},
	})
}

// GetThread returns all comments in a thread
func (ch *CommentsHandler) GetThread(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	threadID := c.Param("threadId")
	if threadID == "" {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Thread ID is required",
			Code:    "INVALID_THREAD_ID",
		})
		return
	}

	// Get all comments in the thread
	var comments []models.CodeComment
	if err := ch.DB.Where("thread_id = ?", threadID).
		Order("created_at ASC").
		Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	if len(comments) == 0 {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Thread not found",
			Code:    "THREAD_NOT_FOUND",
		})
		return
	}

	// Check access via project
	var project models.Project
	if err := ch.DB.First(&project, comments[0].ProjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	if project.OwnerID != userID && !project.IsPublic {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Find root comment (no parent)
	var rootComment *models.CodeComment
	var replies []models.CodeComment
	for i := range comments {
		if comments[i].ParentID == nil {
			rootComment = &comments[i]
		} else {
			replies = append(replies, comments[i])
		}
	}

	if rootComment == nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Thread has no root comment",
			Code:    "INVALID_THREAD",
		})
		return
	}

	// Build thread response
	threadResponse := ThreadResponse{
		ThreadID:     threadID,
		FileID:       rootComment.FileID,
		ProjectID:    rootComment.ProjectID,
		StartLine:    rootComment.StartLine,
		EndLine:      rootComment.EndLine,
		IsResolved:   rootComment.IsResolved,
		CommentCount: len(comments),
		Comments:     make([]CommentResponse, len(comments)),
		CreatedAt:    rootComment.CreatedAt,
		UpdatedAt:    comments[len(comments)-1].UpdatedAt,
	}

	for i, comment := range comments {
		threadResponse.Comments[i] = *commentToResponse(&comment, nil)
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    threadResponse,
	})
}

// GetComment returns a single comment
func (ch *CommentsHandler) GetComment(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	commentIDStr := c.Param("id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid comment ID",
			Code:    "INVALID_COMMENT_ID",
		})
		return
	}

	var comment models.CodeComment
	if err := ch.DB.First(&comment, uint(commentID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Comment not found",
				Code:    "COMMENT_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Check access
	var project models.Project
	if err := ch.DB.First(&project, comment.ProjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	if project.OwnerID != userID && !project.IsPublic {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    commentToResponse(&comment, nil),
	})
}

// UpdateComment updates a comment's content
func (ch *CommentsHandler) UpdateComment(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	commentIDStr := c.Param("id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid comment ID",
			Code:    "INVALID_COMMENT_ID",
		})
		return
	}

	var req UpdateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	var comment models.CodeComment
	if err := ch.DB.First(&comment, uint(commentID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Comment not found",
				Code:    "COMMENT_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Only the author can edit their comment
	if comment.AuthorID != userID {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Only the comment author can edit it",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Update the comment
	if err := ch.DB.Model(&comment).Update("content", req.Content).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to update comment",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Refresh the comment
	ch.DB.First(&comment, uint(commentID))

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Comment updated successfully",
		Data:    commentToResponse(&comment, nil),
	})
}

// DeleteComment deletes a comment
func (ch *CommentsHandler) DeleteComment(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	commentIDStr := c.Param("id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid comment ID",
			Code:    "INVALID_COMMENT_ID",
		})
		return
	}

	var comment models.CodeComment
	if err := ch.DB.First(&comment, uint(commentID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Comment not found",
				Code:    "COMMENT_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Check if user is author or project owner
	var project models.Project
	if err := ch.DB.First(&project, comment.ProjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	if comment.AuthorID != userID && project.OwnerID != userID {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Only the comment author or project owner can delete it",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// If this is a root comment, delete all replies too
	if comment.ParentID == nil {
		ch.DB.Where("thread_id = ?", comment.ThreadID).Delete(&models.CodeComment{})
	} else {
		// Just delete this comment
		ch.DB.Delete(&comment)
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Comment deleted successfully",
	})
}

// ResolveThread marks a thread as resolved
func (ch *CommentsHandler) ResolveThread(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	commentIDStr := c.Param("id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid comment ID",
			Code:    "INVALID_COMMENT_ID",
		})
		return
	}

	var comment models.CodeComment
	if err := ch.DB.First(&comment, uint(commentID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Comment not found",
				Code:    "COMMENT_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Check access - must be author or project owner
	var project models.Project
	if err := ch.DB.First(&project, comment.ProjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	if comment.AuthorID != userID && project.OwnerID != userID {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Only the comment author or project owner can resolve threads",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Resolve all comments in the thread
	now := time.Now()
	ch.DB.Model(&models.CodeComment{}).
		Where("thread_id = ?", comment.ThreadID).
		Updates(map[string]interface{}{
			"is_resolved":    true,
			"resolved_at":    &now,
			"resolved_by_id": userID,
		})

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Thread resolved successfully",
		Data: map[string]interface{}{
			"thread_id":   comment.ThreadID,
			"resolved_at": now,
			"resolved_by": userID,
		},
	})
}

// UnresolveThread marks a thread as unresolved
func (ch *CommentsHandler) UnresolveThread(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	commentIDStr := c.Param("id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid comment ID",
			Code:    "INVALID_COMMENT_ID",
		})
		return
	}

	var comment models.CodeComment
	if err := ch.DB.First(&comment, uint(commentID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Comment not found",
				Code:    "COMMENT_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Check access
	var project models.Project
	if err := ch.DB.First(&project, comment.ProjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	if comment.AuthorID != userID && project.OwnerID != userID {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Only the comment author or project owner can unresolve threads",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Unresolve all comments in the thread
	ch.DB.Model(&models.CodeComment{}).
		Where("thread_id = ?", comment.ThreadID).
		Updates(map[string]interface{}{
			"is_resolved":    false,
			"resolved_at":    nil,
			"resolved_by_id": nil,
		})

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Thread reopened successfully",
		Data: map[string]interface{}{
			"thread_id": comment.ThreadID,
		},
	})
}

// AddReaction adds an emoji reaction to a comment
func (ch *CommentsHandler) AddReaction(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	commentIDStr := c.Param("id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid comment ID",
			Code:    "INVALID_COMMENT_ID",
		})
		return
	}

	var req ReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Emoji is required",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	var comment models.CodeComment
	if err := ch.DB.First(&comment, uint(commentID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Comment not found",
				Code:    "COMMENT_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Check access
	var project models.Project
	if err := ch.DB.First(&project, comment.ProjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	if project.OwnerID != userID && !project.IsPublic {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Initialize reactions map if nil
	if comment.Reactions == nil {
		comment.Reactions = make(map[string][]uint)
	}

	// Add user to reaction (avoid duplicates)
	users := comment.Reactions[req.Emoji]
	for _, uid := range users {
		if uid == userID {
			c.JSON(http.StatusOK, StandardResponse{
				Success: true,
				Message: "Reaction already exists",
				Data:    comment.Reactions,
			})
			return
		}
	}

	comment.Reactions[req.Emoji] = append(users, userID)

	// Save updated reactions
	if err := ch.DB.Model(&comment).Update("reactions", comment.Reactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to add reaction",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: fmt.Sprintf("Added %s reaction", req.Emoji),
		Data:    comment.Reactions,
	})
}

// RemoveReaction removes an emoji reaction from a comment
func (ch *CommentsHandler) RemoveReaction(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	commentIDStr := c.Param("id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid comment ID",
			Code:    "INVALID_COMMENT_ID",
		})
		return
	}

	var req ReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Emoji is required",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	var comment models.CodeComment
	if err := ch.DB.First(&comment, uint(commentID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Comment not found",
				Code:    "COMMENT_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	if comment.Reactions == nil {
		c.JSON(http.StatusOK, StandardResponse{
			Success: true,
			Message: "No reactions to remove",
			Data:    map[string][]uint{},
		})
		return
	}

	// Remove user from reaction
	users := comment.Reactions[req.Emoji]
	newUsers := make([]uint, 0, len(users))
	for _, uid := range users {
		if uid != userID {
			newUsers = append(newUsers, uid)
		}
	}

	if len(newUsers) == 0 {
		delete(comment.Reactions, req.Emoji)
	} else {
		comment.Reactions[req.Emoji] = newUsers
	}

	// Save updated reactions
	if err := ch.DB.Model(&comment).Update("reactions", comment.Reactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to remove reaction",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: fmt.Sprintf("Removed %s reaction", req.Emoji),
		Data:    comment.Reactions,
	})
}

// commentToResponse converts a CodeComment model to a CommentResponse
func commentToResponse(comment *models.CodeComment, replies []models.CodeComment) *CommentResponse {
	response := &CommentResponse{
		ID:           comment.ID,
		FileID:       comment.FileID,
		ProjectID:    comment.ProjectID,
		StartLine:    comment.StartLine,
		EndLine:      comment.EndLine,
		StartColumn:  comment.StartColumn,
		EndColumn:    comment.EndColumn,
		Content:      comment.Content,
		ParentID:     comment.ParentID,
		ThreadID:     comment.ThreadID,
		AuthorID:     comment.AuthorID,
		AuthorName:   comment.AuthorName,
		IsResolved:   comment.IsResolved,
		ResolvedAt:   comment.ResolvedAt,
		ResolvedByID: comment.ResolvedByID,
		Reactions:    comment.Reactions,
		ReplyCount:   len(replies),
		CreatedAt:    comment.CreatedAt,
		UpdatedAt:    comment.UpdatedAt,
	}

	if len(replies) > 0 {
		response.Replies = make([]CommentResponse, len(replies))
		for i, reply := range replies {
			response.Replies[i] = *commentToResponse(&reply, nil)
		}
	}

	return response
}
