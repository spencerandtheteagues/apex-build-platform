// APEX.BUILD Version History Handler
// File version management with diff viewing and restore capabilities (Replit parity feature)

package handlers

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"apex-build/internal/middleware"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// VersionHandler handles file version operations
type VersionHandler struct {
	DB *gorm.DB
}

// NewVersionHandler creates a new version handler
func NewVersionHandler(db *gorm.DB) *VersionHandler {
	return &VersionHandler{DB: db}
}

// VersionListResponse represents a list of versions
type VersionListResponse struct {
	Versions   []VersionSummary `json:"versions"`
	Total      int64            `json:"total"`
	HasMore    bool             `json:"has_more"`
	FileID     uint             `json:"file_id"`
	FileName   string           `json:"file_name"`
}

// VersionSummary is a lightweight version representation for lists
type VersionSummary struct {
	ID            uint      `json:"id"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	AuthorName    string    `json:"author_name"`
	AuthorID      uint      `json:"author_id"`
	ChangeType    string    `json:"change_type"`
	ChangeSummary string    `json:"change_summary"`
	LinesAdded    int       `json:"lines_added"`
	LinesRemoved  int       `json:"lines_removed"`
	Size          int64     `json:"size"`
	IsPinned      bool      `json:"is_pinned"`
}

// DiffResponse represents the diff between two versions
type DiffResponse struct {
	OldVersion    int          `json:"old_version"`
	NewVersion    int          `json:"new_version"`
	Hunks         []DiffHunk   `json:"hunks"`
	TotalAdded    int          `json:"total_added"`
	TotalRemoved  int          `json:"total_removed"`
	TotalModified int          `json:"total_modified"`
}

// DiffHunk represents a chunk of differences
type DiffHunk struct {
	OldStart int        `json:"old_start"`
	OldCount int        `json:"old_count"`
	NewStart int        `json:"new_start"`
	NewCount int        `json:"new_count"`
	Lines    []DiffLine `json:"lines"`
}

// DiffLine represents a single line in the diff
type DiffLine struct {
	Type    string `json:"type"` // add, remove, context
	Content string `json:"content"`
	OldLine int    `json:"old_line,omitempty"`
	NewLine int    `json:"new_line,omitempty"`
}

// RegisterVersionRoutes registers version-related routes
func (vh *VersionHandler) RegisterVersionRoutes(rg *gin.RouterGroup) {
	versions := rg.Group("/versions")
	{
		versions.GET("/file/:fileId", vh.GetFileVersions)           // List versions for a file
		versions.GET("/:versionId", vh.GetVersion)                  // Get specific version
		versions.GET("/:versionId/content", vh.GetVersionContent)   // Get version content
		versions.POST("/:versionId/restore", vh.RestoreVersion)     // Restore file to version
		versions.POST("/:versionId/pin", vh.PinVersion)             // Pin/unpin version
		versions.GET("/diff/:oldId/:newId", vh.GetDiff)             // Get diff between versions
		versions.GET("/file/:fileId/diff", vh.GetFileDiff)          // Get diff with current
		versions.DELETE("/:versionId", vh.DeleteVersion)            // Delete unpinned version
	}
}

// GetFileVersions returns all versions for a file
func (vh *VersionHandler) GetFileVersions(c *gin.Context) {
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

	// Get file with project to check access
	var file models.File
	if err := vh.DB.Preload("Project").First(&file, uint(fileID)).Error; err != nil {
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

	// Parse pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	// Get versions
	var versions []models.FileVersion
	var total int64

	vh.DB.Model(&models.FileVersion{}).Where("file_id = ?", uint(fileID)).Count(&total)

	if err := vh.DB.Where("file_id = ?", uint(fileID)).
		Order("version DESC").
		Limit(limit).
		Offset(offset).
		Find(&versions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to fetch versions",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Convert to summaries
	summaries := make([]VersionSummary, len(versions))
	for i, v := range versions {
		summaries[i] = VersionSummary{
			ID:            v.ID,
			Version:       v.Version,
			CreatedAt:     v.CreatedAt,
			AuthorName:    v.AuthorName,
			AuthorID:      v.AuthorID,
			ChangeType:    v.ChangeType,
			ChangeSummary: v.ChangeSummary,
			LinesAdded:    v.LinesAdded,
			LinesRemoved:  v.LinesRemoved,
			Size:          v.Size,
			IsPinned:      v.IsPinned,
		}
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: VersionListResponse{
			Versions: summaries,
			Total:    total,
			HasMore:  int64(offset+limit) < total,
			FileID:   uint(fileID),
			FileName: file.Name,
		},
	})
}

// GetVersion returns a specific version's metadata
func (vh *VersionHandler) GetVersion(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	versionIDStr := c.Param("versionId")
	versionID, err := strconv.ParseUint(versionIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid version ID",
			Code:    "INVALID_VERSION_ID",
		})
		return
	}

	var version models.FileVersion
	if err := vh.DB.Preload("Author").First(&version, uint(versionID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Version not found",
				Code:    "VERSION_NOT_FOUND",
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

	// Check access via project
	var project models.Project
	if err := vh.DB.First(&project, version.ProjectID).Error; err != nil {
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

	// Return version without content (use GetVersionContent for that)
	version.Content = "" // Don't send content in metadata endpoint

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    version,
	})
}

// GetVersionContent returns the full content of a version
func (vh *VersionHandler) GetVersionContent(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	versionIDStr := c.Param("versionId")
	versionID, err := strconv.ParseUint(versionIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid version ID",
			Code:    "INVALID_VERSION_ID",
		})
		return
	}

	var version models.FileVersion
	if err := vh.DB.First(&version, uint(versionID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Version not found",
				Code:    "VERSION_NOT_FOUND",
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
	if err := vh.DB.First(&project, version.ProjectID).Error; err != nil {
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
		Data: map[string]interface{}{
			"version_id": version.ID,
			"version":    version.Version,
			"content":    version.Content,
			"file_name":  version.FileName,
			"file_path":  version.FilePath,
			"size":       version.Size,
			"line_count": version.LineCount,
		},
	})
}

// RestoreVersion restores a file to a specific version
func (vh *VersionHandler) RestoreVersion(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	versionIDStr := c.Param("versionId")
	versionID, err := strconv.ParseUint(versionIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid version ID",
			Code:    "INVALID_VERSION_ID",
		})
		return
	}

	var version models.FileVersion
	if err := vh.DB.First(&version, uint(versionID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Version not found",
				Code:    "VERSION_NOT_FOUND",
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

	// Check ownership
	var project models.Project
	if err := vh.DB.First(&project, version.ProjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied - only project owner can restore versions",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Get current file
	var file models.File
	if err := vh.DB.First(&file, version.FileID).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "File no longer exists",
			Code:    "FILE_NOT_FOUND",
		})
		return
	}

	// Get current user for author name
	var user models.User
	vh.DB.First(&user, userID)

	// Create a new version capturing current state before restore
	currentVersion := CreateFileVersion(vh.DB, &file, userID, user.Username, "pre-restore",
		fmt.Sprintf("State before restore to version %d", version.Version))

	// Update the file with restored content
	err = vh.DB.Transaction(func(tx *gorm.DB) error {
		// Update file content
		updates := map[string]interface{}{
			"content":      version.Content,
			"size":         version.Size,
			"last_edit_by": userID,
			"version":      gorm.Expr("version + 1"),
		}

		if err := tx.Model(&file).Updates(updates).Error; err != nil {
			return err
		}

		// Create restore version record
		restoreVersion := &models.FileVersion{
			FileID:        file.ID,
			ProjectID:     file.ProjectID,
			Version:       file.Version + 1,
			VersionHash:   version.VersionHash,
			Content:       version.Content,
			Size:          version.Size,
			LineCount:     version.LineCount,
			ChangeType:    "restore",
			ChangeSummary: fmt.Sprintf("Restored from version %d", version.Version),
			AuthorID:      userID,
			AuthorName:    user.Username,
			FilePath:      file.Path,
			FileName:      file.Name,
		}

		return tx.Create(restoreVersion).Error
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Failed to restore version",
			Code:    "RESTORE_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: fmt.Sprintf("File restored to version %d", version.Version),
		Data: map[string]interface{}{
			"restored_version":  version.Version,
			"new_version":       file.Version + 1,
			"backup_version_id": currentVersion,
		},
	})
}

// PinVersion pins or unpins a version to prevent auto-deletion
func (vh *VersionHandler) PinVersion(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	versionIDStr := c.Param("versionId")
	versionID, err := strconv.ParseUint(versionIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid version ID",
			Code:    "INVALID_VERSION_ID",
		})
		return
	}

	var req struct {
		Pinned bool `json:"pinned"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to toggle if no body
		req.Pinned = true
	}

	var version models.FileVersion
	if err := vh.DB.First(&version, uint(versionID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Version not found",
				Code:    "VERSION_NOT_FOUND",
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

	// Check ownership
	var project models.Project
	if err := vh.DB.First(&project, version.ProjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Toggle or set pin status
	vh.DB.Model(&version).Update("is_pinned", req.Pinned)

	action := "pinned"
	if !req.Pinned {
		action = "unpinned"
	}

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: fmt.Sprintf("Version %s successfully", action),
		Data: map[string]interface{}{
			"version_id": version.ID,
			"is_pinned":  req.Pinned,
		},
	})
}

// GetDiff returns the diff between two specific versions
func (vh *VersionHandler) GetDiff(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	oldIDStr := c.Param("oldId")
	newIDStr := c.Param("newId")

	oldID, err := strconv.ParseUint(oldIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid old version ID",
			Code:    "INVALID_VERSION_ID",
		})
		return
	}

	newID, err := strconv.ParseUint(newIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid new version ID",
			Code:    "INVALID_VERSION_ID",
		})
		return
	}

	// Get both versions
	var oldVersion, newVersion models.FileVersion
	if err := vh.DB.First(&oldVersion, uint(oldID)).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Old version not found",
			Code:    "VERSION_NOT_FOUND",
		})
		return
	}

	if err := vh.DB.First(&newVersion, uint(newID)).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "New version not found",
			Code:    "VERSION_NOT_FOUND",
		})
		return
	}

	// Verify same file
	if oldVersion.FileID != newVersion.FileID {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Versions must be from the same file",
			Code:    "VERSION_MISMATCH",
		})
		return
	}

	// Check access
	var project models.Project
	if err := vh.DB.First(&project, oldVersion.ProjectID).Error; err != nil {
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

	// Generate diff
	diff := generateDiff(oldVersion.Content, newVersion.Content)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: DiffResponse{
			OldVersion:    oldVersion.Version,
			NewVersion:    newVersion.Version,
			Hunks:         diff.Hunks,
			TotalAdded:    diff.TotalAdded,
			TotalRemoved:  diff.TotalRemoved,
			TotalModified: diff.TotalModified,
		},
	})
}

// GetFileDiff returns diff between a version and current file content
func (vh *VersionHandler) GetFileDiff(c *gin.Context) {
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

	versionStr := c.Query("version")
	version, _ := strconv.Atoi(versionStr)

	// Get file
	var file models.File
	if err := vh.DB.Preload("Project").First(&file, uint(fileID)).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "File not found",
			Code:    "FILE_NOT_FOUND",
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

	// Get the version to compare
	var fileVersion models.FileVersion
	query := vh.DB.Where("file_id = ?", uint(fileID))
	if version > 0 {
		query = query.Where("version = ?", version)
	} else {
		// Get latest version before current
		query = query.Order("version DESC")
	}

	if err := query.First(&fileVersion).Error; err != nil {
		c.JSON(http.StatusNotFound, StandardResponse{
			Success: false,
			Error:   "Version not found",
			Code:    "VERSION_NOT_FOUND",
		})
		return
	}

	// Generate diff
	diff := generateDiff(fileVersion.Content, file.Content)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data: DiffResponse{
			OldVersion:    fileVersion.Version,
			NewVersion:    file.Version,
			Hunks:         diff.Hunks,
			TotalAdded:    diff.TotalAdded,
			TotalRemoved:  diff.TotalRemoved,
			TotalModified: diff.TotalModified,
		},
	})
}

// DeleteVersion deletes a version (only unpinned versions can be deleted)
func (vh *VersionHandler) DeleteVersion(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, StandardResponse{
			Success: false,
			Error:   "User not authenticated",
			Code:    "NOT_AUTHENTICATED",
		})
		return
	}

	versionIDStr := c.Param("versionId")
	versionID, err := strconv.ParseUint(versionIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Invalid version ID",
			Code:    "INVALID_VERSION_ID",
		})
		return
	}

	var version models.FileVersion
	if err := vh.DB.First(&version, uint(versionID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, StandardResponse{
				Success: false,
				Error:   "Version not found",
				Code:    "VERSION_NOT_FOUND",
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

	// Check ownership
	var project models.Project
	if err := vh.DB.First(&project, version.ProjectID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, StandardResponse{
			Success: false,
			Error:   "Database error",
			Code:    "DATABASE_ERROR",
		})
		return
	}

	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, StandardResponse{
			Success: false,
			Error:   "Access denied",
			Code:    "ACCESS_DENIED",
		})
		return
	}

	// Don't allow deleting pinned versions
	if version.IsPinned {
		c.JSON(http.StatusBadRequest, StandardResponse{
			Success: false,
			Error:   "Cannot delete pinned versions. Unpin first.",
			Code:    "VERSION_PINNED",
		})
		return
	}

	// Delete (soft delete via GORM)
	vh.DB.Delete(&version)

	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Message: "Version deleted successfully",
	})
}

// CreateFileVersion creates a new version of a file (called from UpdateFile)
func CreateFileVersion(db *gorm.DB, file *models.File, authorID uint, authorName, changeType, summary string) uint {
	// Calculate hash for deduplication
	hash := sha256.Sum256([]byte(file.Content))
	hashStr := fmt.Sprintf("%x", hash)

	// Check if this exact content already exists as latest version
	var existingVersion models.FileVersion
	if err := db.Where("file_id = ? AND version_hash = ?", file.ID, hashStr).
		Order("version DESC").
		First(&existingVersion).Error; err == nil {
		// Same content already exists, don't create duplicate
		return existingVersion.ID
	}

	// Get current max version
	var maxVersion int
	db.Model(&models.FileVersion{}).
		Where("file_id = ?", file.ID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion)

	// Count lines
	lineCount := strings.Count(file.Content, "\n") + 1
	if file.Content == "" {
		lineCount = 0
	}

	// Calculate lines added/removed from previous version
	var linesAdded, linesRemoved int
	var prevVersion models.FileVersion
	if err := db.Where("file_id = ?", file.ID).
		Order("version DESC").
		First(&prevVersion).Error; err == nil {
		linesAdded, linesRemoved = calculateLineDiff(prevVersion.Content, file.Content)
	} else {
		// First version - all lines are "added"
		linesAdded = lineCount
	}

	version := &models.FileVersion{
		FileID:        file.ID,
		ProjectID:     file.ProjectID,
		Version:       maxVersion + 1,
		VersionHash:   hashStr,
		Content:       file.Content,
		Size:          file.Size,
		LineCount:     lineCount,
		ChangeType:    changeType,
		ChangeSummary: summary,
		LinesAdded:    linesAdded,
		LinesRemoved:  linesRemoved,
		AuthorID:      authorID,
		AuthorName:    authorName,
		FilePath:      file.Path,
		FileName:      file.Name,
	}

	db.Create(version)
	return version.ID
}

// calculateLineDiff calculates lines added and removed between two contents
func calculateLineDiff(oldContent, newContent string) (added, removed int) {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	oldSet := make(map[string]int)
	for _, line := range oldLines {
		oldSet[line]++
	}

	newSet := make(map[string]int)
	for _, line := range newLines {
		newSet[line]++
	}

	// Count additions (lines in new but not in old)
	for line, count := range newSet {
		if oldCount, exists := oldSet[line]; exists {
			if count > oldCount {
				added += count - oldCount
			}
		} else {
			added += count
		}
	}

	// Count removals (lines in old but not in new)
	for line, count := range oldSet {
		if newCount, exists := newSet[line]; exists {
			if count > newCount {
				removed += count - newCount
			}
		} else {
			removed += count
		}
	}

	return added, removed
}

// generateDiff generates a diff between two contents
func generateDiff(oldContent, newContent string) DiffResponse {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Simple Myers diff algorithm implementation
	hunks := computeDiffHunks(oldLines, newLines)

	var totalAdded, totalRemoved int
	for _, hunk := range hunks {
		for _, line := range hunk.Lines {
			switch line.Type {
			case "add":
				totalAdded++
			case "remove":
				totalRemoved++
			}
		}
	}

	return DiffResponse{
		Hunks:         hunks,
		TotalAdded:    totalAdded,
		TotalRemoved:  totalRemoved,
		TotalModified: min(totalAdded, totalRemoved),
	}
}

// computeDiffHunks computes diff hunks using a simplified approach
func computeDiffHunks(oldLines, newLines []string) []DiffHunk {
	// Build LCS (Longest Common Subsequence) matrix
	m, n := len(oldLines), len(newLines)

	// For very large files, use a simpler approach
	if m*n > 10000000 {
		return simpleDiff(oldLines, newLines)
	}

	// LCS DP table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	// Backtrack to find diff
	var lines []DiffLine
	i, j := m, n

	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			lines = append([]DiffLine{{
				Type:    "context",
				Content: oldLines[i-1],
				OldLine: i,
				NewLine: j,
			}}, lines...)
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			lines = append([]DiffLine{{
				Type:    "add",
				Content: newLines[j-1],
				NewLine: j,
			}}, lines...)
			j--
		} else if i > 0 {
			lines = append([]DiffLine{{
				Type:    "remove",
				Content: oldLines[i-1],
				OldLine: i,
			}}, lines...)
			i--
		}
	}

	// Group into hunks (context of 3 lines)
	return groupIntoHunks(lines, 3)
}

// simpleDiff provides a simpler diff for large files
func simpleDiff(oldLines, newLines []string) []DiffHunk {
	var lines []DiffLine

	oldSet := make(map[string]bool)
	for _, line := range oldLines {
		oldSet[line] = true
	}

	newSet := make(map[string]bool)
	for _, line := range newLines {
		newSet[line] = true
	}

	for i, line := range oldLines {
		if !newSet[line] {
			lines = append(lines, DiffLine{
				Type:    "remove",
				Content: line,
				OldLine: i + 1,
			})
		}
	}

	for i, line := range newLines {
		if !oldSet[line] {
			lines = append(lines, DiffLine{
				Type:    "add",
				Content: line,
				NewLine: i + 1,
			})
		}
	}

	if len(lines) == 0 {
		return []DiffHunk{}
	}

	return []DiffHunk{{
		OldStart: 1,
		OldCount: len(oldLines),
		NewStart: 1,
		NewCount: len(newLines),
		Lines:    lines,
	}}
}

// groupIntoHunks groups diff lines into hunks with context
func groupIntoHunks(lines []DiffLine, contextSize int) []DiffHunk {
	if len(lines) == 0 {
		return []DiffHunk{}
	}

	var hunks []DiffHunk
	var currentHunk *DiffHunk
	contextBuffer := make([]DiffLine, 0, contextSize)

	for _, line := range lines {
		if line.Type != "context" {
			// Start new hunk if needed
			if currentHunk == nil {
				currentHunk = &DiffHunk{
					OldStart: max(1, line.OldLine-contextSize),
					NewStart: max(1, line.NewLine-contextSize),
				}
				// Add buffered context
				currentHunk.Lines = append(currentHunk.Lines, contextBuffer...)
				contextBuffer = nil
			}
			currentHunk.Lines = append(currentHunk.Lines, line)
		} else {
			if currentHunk != nil {
				// Add context to current hunk
				currentHunk.Lines = append(currentHunk.Lines, line)

				// Check if we should close the hunk
				contextCount := 0
				for i := len(currentHunk.Lines) - 1; i >= 0 && currentHunk.Lines[i].Type == "context"; i-- {
					contextCount++
				}

				if contextCount >= contextSize*2 {
					// Close hunk and start buffering
					hunks = append(hunks, *currentHunk)
					currentHunk = nil
					contextBuffer = nil
				}
			} else {
				// Buffer context for potential next hunk
				contextBuffer = append(contextBuffer, line)
				if len(contextBuffer) > contextSize {
					contextBuffer = contextBuffer[1:]
				}
			}
		}
	}

	// Add final hunk if exists
	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	// Calculate counts for each hunk
	for i := range hunks {
		var oldCount, newCount int
		for _, line := range hunks[i].Lines {
			switch line.Type {
			case "context":
				oldCount++
				newCount++
			case "remove":
				oldCount++
			case "add":
				newCount++
			}
		}
		hunks[i].OldCount = oldCount
		hunks[i].NewCount = newCount
	}

	return hunks
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
