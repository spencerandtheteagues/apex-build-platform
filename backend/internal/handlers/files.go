// APEX.BUILD File Handlers
// File management and operations

package handlers

import (
	"archive/zip"
	"net/http"
	"strconv"

	"apex-build/internal/middleware"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetFiles returns all files in a project
func (h *Handler) GetFiles(c *gin.Context) {
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

	// Verify user has access to the project
	var project models.Project
	result := h.DB.Where("id = ? AND (owner_id = ? OR is_public = ?)", uint(projectID), userID, true).
		First(&project)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Project not found or access denied",
				Code:    "PROJECT_NOT_FOUND",
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

	// Get files
	var files []models.File
	if err := h.DB.Where("project_id = ?", uint(projectID)).
		Order("type DESC, name ASC").
		Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to fetch files",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    files,
	})
}

// CreateFile creates a new file in a project
func (h *Handler) CreateFile(c *gin.Context) {
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

	var req struct {
		Name     string `json:"name" binding:"required,min=1,max=255"`
		Path     string `json:"path" binding:"required,max=1000"`
		Type     string `json:"type" binding:"required,oneof=file directory"`
		Content  string `json:"content"`
		MimeType string `json:"mime_type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Verify user owns the project
	var project models.Project
	if err := h.DB.Where("id = ? AND owner_id = ?", uint(projectID), userID).First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Project not found or access denied",
				Code:    "PROJECT_NOT_FOUND",
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

	// Check if file already exists
	var existingFile models.File
	if err := h.DB.Where("project_id = ? AND path = ?", uint(projectID), req.Path).First(&existingFile).Error; err == nil {
		c.JSON(http.StatusConflict, StandardResponse{
			Success: false,
			Error:   "File already exists at this path",
			Code:    "FILE_EXISTS",
		})
		return
	}

	// Determine MIME type if not provided
	if req.MimeType == "" {
		req.MimeType = h.getMimeType(req.Name)
	}

	// Create file
	file := models.File{
		ProjectID:   uint(projectID),
		Name:        req.Name,
		Path:        req.Path,
		Type:        req.Type,
		MimeType:    req.MimeType,
		Content:     req.Content,
		Size:        int64(len(req.Content)),
		LastEditBy:  userID,
	}

	if err := h.DB.Create(&file).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to create file",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusCreated, StandardResponse{
		Success: true,
		Data:    file,
		Message: "File created successfully",
	})
}

// GetFile returns a specific file
func (h *Handler) GetFile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	fileIDStr := c.Param("id")
	fileID, err := strconv.ParseUint(fileIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid file ID",
			Code:    "INVALID_FILE_ID",
		})
		return
	}

	// Get file with project info
	var file models.File
	result := h.DB.Preload("Project").First(&file, uint(fileID))

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
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

	// Check access permissions
	if file.Project.OwnerID != userID && !file.Project.IsPublic {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    file,
	})
}

// UpdateFile updates a file's content and automatically creates a version history entry
func (h *Handler) UpdateFile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	fileIDStr := c.Param("id")
	fileID, err := strconv.ParseUint(fileIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid file ID",
			Code:    "INVALID_FILE_ID",
		})
		return
	}

	var req struct {
		Content       *string `json:"content"`
		Name          *string `json:"name" binding:"omitempty,min=1,max=255"`
		Path          *string `json:"path" binding:"omitempty,max=1000"`
		CreateVersion *bool   `json:"create_version"` // Explicitly create version (default true for content changes)
		VersionNote   string  `json:"version_note"`   // Optional note for version
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid request format",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Get file with project info
	var file models.File
	result := h.DB.Preload("Project").First(&file, uint(fileID))

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
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

	// Check if user owns the project
	if file.Project.OwnerID != userID {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Check if file is locked by another user
	if file.LockedBy != nil && *file.LockedBy != userID {
		c.JSON(http.StatusConflict, StandardResponse{
			Success: false,
			Error:   "File is locked by another user",
			Code:    "FILE_LOCKED",
		})
		return
	}

	// Get user for version authorship
	var user models.User
	h.DB.First(&user, userID)

	// Determine if we should create a version
	shouldCreateVersion := req.Content != nil // Default: create version on content change
	if req.CreateVersion != nil {
		shouldCreateVersion = *req.CreateVersion
	}

	// Create version before updating (if content is changing)
	var versionID uint
	if shouldCreateVersion && req.Content != nil && *req.Content != file.Content {
		// Determine change type
		changeType := "edit"
		changeSummary := req.VersionNote
		if req.Name != nil && *req.Name != file.Name {
			changeType = "rename"
			if changeSummary == "" {
				changeSummary = "Renamed file"
			}
		}
		if changeSummary == "" {
			changeSummary = "Content updated"
		}

		// Temporarily update file object for versioning
		oldContent := file.Content
		file.Content = *req.Content
		file.Size = int64(len(*req.Content))

		versionID = CreateFileVersion(h.DB, &file, userID, user.Username, changeType, changeSummary)

		// Restore for actual update
		file.Content = oldContent
	}

	// Prepare updates
	updates := make(map[string]interface{})
	updates["last_edit_by"] = userID
	updates["version"] = gorm.Expr("version + 1")

	if req.Content != nil {
		updates["content"] = *req.Content
		updates["size"] = int64(len(*req.Content))
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Path != nil {
		// Check if new path conflicts
		if *req.Path != file.Path {
			var existingFile models.File
			if err := h.DB.Where("project_id = ? AND path = ? AND id != ?", file.ProjectID, *req.Path, file.ID).First(&existingFile).Error; err == nil {
				c.JSON(http.StatusConflict, StandardResponse{
					Success: false,
					Error:   "File already exists at this path",
					Code:    "FILE_EXISTS",
				})
				return
			}
		}
		updates["path"] = *req.Path
	}

	// Update file
	if err := h.DB.Model(&file).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to update file",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	response := map[string]interface{}{
		"message": "File updated successfully",
	}
	if versionID > 0 {
		response["version_id"] = versionID
		response["version_created"] = true
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "File updated successfully",
		Data:    response,
	})
}

// DeleteFile deletes a file
func (h *Handler) DeleteFile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	fileIDStr := c.Param("id")
	fileID, err := strconv.ParseUint(fileIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid file ID",
			Code:    "INVALID_FILE_ID",
		})
		return
	}

	// Get file with project info
	var file models.File
	result := h.DB.Preload("Project").First(&file, uint(fileID))

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
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

	// Check if user owns the project
	if file.Project.OwnerID != userID {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Delete file
	if err := h.DB.Delete(&file).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to delete file",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "File deleted successfully",
	})
}

// DownloadProject exports all project files as a zip archive
func (h *Handler) DownloadProject(c *gin.Context) {
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

	// Verify user has access to the project
	var project models.Project
	result := h.DB.Where("id = ? AND (owner_id = ? OR is_public = ?)", uint(projectID), userID, true).
		First(&project)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Project not found or access denied",
				Code:    "PROJECT_NOT_FOUND",
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

	// Get all files for the project
	var files []models.File
	if err := h.DB.Where("project_id = ?", uint(projectID)).Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to fetch files",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Create zip archive in memory
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", "attachment; filename=\""+project.Name+".zip\"")

	zipWriter := zip.NewWriter(c.Writer)
	defer zipWriter.Close()

	for _, file := range files {
		// Skip directories, only include actual files
		if file.Type == "directory" {
			continue
		}

		// Remove leading slash from path for zip entry
		path := file.Path
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}

		// Create file in zip
		w, err := zipWriter.Create(path)
		if err != nil {
			// Log error but continue with other files
			continue
		}

		// Write file content
		if _, err := w.Write([]byte(file.Content)); err != nil {
			continue
		}
	}
}

// getMimeType determines MIME type based on file extension
func (h *Handler) getMimeType(filename string) string {
	mimeTypes := map[string]string{
		".js":   "text/javascript",
		".jsx":  "text/javascript",
		".ts":   "application/typescript",
		".tsx":  "application/typescript",
		".py":   "text/x-python",
		".go":   "text/x-go",
		".rs":   "text/x-rust",
		".java": "text/x-java",
		".cpp":  "text/x-c++",
		".c":    "text/x-c",
		".html": "text/html",
		".css":  "text/css",
		".json": "application/json",
		".md":   "text/markdown",
		".txt":  "text/plain",
		".yml":  "application/x-yaml",
		".yaml": "application/x-yaml",
		".xml":  "application/xml",
		".svg":  "image/svg+xml",
	}

	// Extract extension
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			ext := filename[i:]
			if mimeType, exists := mimeTypes[ext]; exists {
				return mimeType
			}
			break
		}
	}

	return "text/plain"
}