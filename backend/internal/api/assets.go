// APEX.BUILD Asset Handlers
// User-uploaded files that AI agents use automatically when building apps.
// Users upload a logo, CSV, PDF, etc. — agents see it in their context and use it.

package api

import (
	"encoding/csv"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"apex-build/internal/middleware"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	maxAssetSize      = 50 * 1024 * 1024 // 50 MB per file
	uploadsBaseDir    = "./uploads/projects"
	contentPreviewMax = 800 // chars of text preview for AI context
)

// allowedMimeTypes maps mime prefix → friendly file type label
var allowedMimeTypes = map[string]string{
	"image/":      "image",
	"video/":      "video",
	"text/csv":    "csv",
	"text/plain":  "text",
	"application/pdf":  "pdf",
	"application/json": "text",
	"application/vnd.ms-excel":                                           "csv",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": "csv",
}

func classifyMime(mimeType string) (string, bool) {
	if ft, ok := allowedMimeTypes[mimeType]; ok {
		return ft, true
	}
	for prefix, ft := range allowedMimeTypes {
		if strings.HasSuffix(prefix, "/") && strings.HasPrefix(mimeType, prefix) {
			return ft, true
		}
	}
	return "other", false
}

func detectMimeType(f multipart.File) (string, error) {
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return http.DetectContentType(buf[:n]), nil
}

func buildContentPreview(f multipart.File, fileType, originalName string) string {
	f.Seek(0, io.SeekStart) //nolint:errcheck

	switch fileType {
	case "image":
		cfg, _, err := image.DecodeConfig(f)
		if err != nil {
			return fmt.Sprintf("Image file: %s", originalName)
		}
		return fmt.Sprintf("Image: %s — %d×%d pixels", originalName, cfg.Width, cfg.Height)

	case "csv":
		r := csv.NewReader(f)
		r.FieldsPerRecord = -1
		r.LazyQuotes = true
		var lines []string
		for i := 0; i < 4; i++ {
			row, err := r.Read()
			if err != nil {
				break
			}
			lines = append(lines, strings.Join(row, " | "))
		}
		if len(lines) == 0 {
			return fmt.Sprintf("CSV file: %s (empty or unreadable)", originalName)
		}
		result := fmt.Sprintf("CSV file: %s\nColumns: %s", originalName, lines[0])
		if len(lines) > 1 {
			result += "\nSample rows:\n" + strings.Join(lines[1:], "\n")
		}
		return result

	case "text":
		buf := make([]byte, contentPreviewMax)
		n, _ := f.Read(buf)
		if n == 0 {
			return fmt.Sprintf("Text file: %s (empty)", originalName)
		}
		preview := strings.TrimSpace(string(buf[:n]))
		if n == contentPreviewMax {
			preview += "..."
		}
		return fmt.Sprintf("Text file: %s\n%s", originalName, preview)

	case "pdf":
		return fmt.Sprintf("PDF document: %s (agent can reference this file by name)", originalName)

	case "video":
		return fmt.Sprintf("Video file: %s (agent can reference this file by name)", originalName)

	default:
		return fmt.Sprintf("File: %s", originalName)
	}
}

// UploadAsset handles POST /api/v1/projects/:id/assets
func (s *Server) UploadAsset(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Not authenticated"})
		return
	}

	projectIDStr := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid project ID"})
		return
	}

	db := s.db.GetDB()

	// Verify ownership
	var project models.Project
	if err := db.Where("id = ? AND owner_id = ?", uint(projectID), userID).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Project not found or access denied"})
		return
	}

	// Parse multipart form (50 MB max)
	if err := c.Request.ParseMultipartForm(maxAssetSize); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "File too large or invalid form (max 50 MB)"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "No file provided — use field name 'file'"})
		return
	}
	defer file.Close()

	if header.Size > maxAssetSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("File too large: %.1f MB (max 50 MB)", float64(header.Size)/1024/1024),
		})
		return
	}

	// Detect real mime type from magic bytes
	detectedMime, err := detectMimeType(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Could not read file"})
		return
	}

	// Correct common misdetections
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == ".csv" && detectedMime == "text/plain" {
		detectedMime = "text/csv"
	}
	if ext == ".json" && detectedMime == "text/plain" {
		detectedMime = "application/json"
	}

	mediaType, _, _ := mime.ParseMediaType(detectedMime)
	if mediaType == "" {
		mediaType = detectedMime
	}

	fileType, allowed := classifyMime(mediaType)
	if !allowed {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("File type not allowed: %s. Allowed: images, videos, CSV, PDF, text files.", mediaType),
		})
		return
	}

	// Extract content preview for AI context
	preview := buildContentPreview(file, fileType, header.Filename)

	// Prepare storage directory
	projectUploadDir := filepath.Join(uploadsBaseDir, fmt.Sprintf("%d", projectID))
	if err := os.MkdirAll(projectUploadDir, 0755); err != nil {
		log.Printf("ERROR: Failed to create upload dir %s: %v", projectUploadDir, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Storage error"})
		return
	}

	// UUID-based filename prevents collisions and path traversal
	storedName := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	storagePath := filepath.Join(projectUploadDir, storedName)

	dst, err := os.Create(storagePath)
	if err != nil {
		log.Printf("ERROR: Failed to create file %s: %v", storagePath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Could not save file"})
		return
	}
	defer dst.Close()

	file.Seek(0, io.SeekStart) //nolint:errcheck
	written, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(storagePath)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save file"})
		return
	}

	asset := models.ProjectAsset{
		ProjectID:      uint(projectID),
		UserID:         userID,
		OriginalName:   header.Filename,
		StoredName:     storedName,
		MimeType:       mediaType,
		FileSize:       written,
		FileType:       fileType,
		ContentPreview: preview,
		StoragePath:    storagePath,
	}
	if err := db.Create(&asset).Error; err != nil {
		os.Remove(storagePath)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
		return
	}

	log.Printf("Asset uploaded: project=%d user=%d file=%s type=%s size=%d bytes",
		projectID, userID, header.Filename, fileType, written)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"asset":   asset,
		"message": fmt.Sprintf("'%s' uploaded. AI agents will automatically use it when building your app.", header.Filename),
	})
}

// ListAssets handles GET /api/v1/projects/:id/assets
func (s *Server) ListAssets(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Not authenticated"})
		return
	}

	projectIDStr := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid project ID"})
		return
	}

	db := s.db.GetDB()

	var project models.Project
	if err := db.Where("id = ? AND (owner_id = ? OR is_public = ?)", uint(projectID), userID, true).
		First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Project not found or access denied"})
		return
	}

	var assets []models.ProjectAsset
	if err := db.Where("project_id = ?", uint(projectID)).
		Order("created_at DESC").
		Find(&assets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"assets":  assets,
		"count":   len(assets),
	})
}

// DeleteAsset handles DELETE /api/v1/projects/:id/assets/:assetId
func (s *Server) DeleteAsset(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Not authenticated"})
		return
	}

	projectIDStr := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid project ID"})
		return
	}

	assetIDStr := c.Param("assetId")
	assetID, err := strconv.ParseUint(assetIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid asset ID"})
		return
	}

	db := s.db.GetDB()

	var asset models.ProjectAsset
	if err := db.Where("id = ? AND project_id = ? AND user_id = ?", uint(assetID), uint(projectID), userID).
		First(&asset).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Asset not found or access denied"})
		return
	}

	if asset.StoragePath != "" {
		if err := os.Remove(asset.StoragePath); err != nil && !os.IsNotExist(err) {
			log.Printf("WARNING: Could not delete asset file %s: %v", asset.StoragePath, err)
		}
	}

	if err := db.Delete(&asset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database error"})
		return
	}

	log.Printf("Asset deleted: id=%d project=%d user=%d file=%s", asset.ID, projectID, userID, asset.OriginalName)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("'%s' deleted.", asset.OriginalName),
	})
}
