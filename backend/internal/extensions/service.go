// APEX.BUILD Extensions Marketplace Service
// Business logic for extension management, discovery, and installation

package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Service handles extension marketplace operations
type Service struct {
	db         *gorm.DB
	httpClient *http.Client
}

// escapeLikePattern escapes special characters in LIKE patterns to prevent SQL injection
// via pattern matching. Characters %, _, and \ have special meaning in SQL LIKE clauses.
func escapeLikePattern(input string) string {
	input = strings.ReplaceAll(input, "\\", "\\\\")
	input = strings.ReplaceAll(input, "%", "\\%")
	input = strings.ReplaceAll(input, "_", "\\_")
	return input
}

// NewService creates a new extension service
func NewService(db *gorm.DB) *Service {
	return &Service{
		db: db,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SearchParams defines parameters for searching extensions
type SearchParams struct {
	Query      string            `json:"query"`
	Category   ExtensionCategory `json:"category"`
	Tags       []string          `json:"tags"`
	SortBy     string            `json:"sort_by"` // downloads, rating, recent, name
	SortOrder  string            `json:"sort_order"` // asc, desc
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	Featured   *bool             `json:"featured"`
	MinRating  float64           `json:"min_rating"`
	AuthorID   *uint             `json:"author_id"`
}

// SearchResult represents search results with pagination
type SearchResult struct {
	Extensions  []Extension `json:"extensions"`
	Total       int64       `json:"total"`
	Page        int         `json:"page"`
	PageSize    int         `json:"page_size"`
	TotalPages  int         `json:"total_pages"`
}

// Search searches for extensions based on parameters
func (s *Service) Search(ctx context.Context, params SearchParams) (*SearchResult, error) {
	// Set defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}
	if params.SortBy == "" {
		params.SortBy = "downloads"
	}
	if params.SortOrder == "" {
		params.SortOrder = "desc"
	}

	query := s.db.WithContext(ctx).Model(&Extension{}).
		Where("status = ?", StatusApproved).
		Where("is_deprecated = ?", false)

	// Apply filters
	if params.Query != "" {
		searchQuery := "%" + escapeLikePattern(strings.ToLower(params.Query)) + "%"
		query = query.Where(
			"LOWER(name) LIKE ? ESCAPE '\\' OR LOWER(display_name) LIKE ? ESCAPE '\\' OR LOWER(description) LIKE ? ESCAPE '\\' OR LOWER(tags) LIKE ? ESCAPE '\\'",
			searchQuery, searchQuery, searchQuery, searchQuery,
		)
	}

	if params.Category != "" {
		query = query.Where("category = ?", params.Category)
	}

	if len(params.Tags) > 0 {
		for _, tag := range params.Tags {
			escapedTag := escapeLikePattern(tag)
			query = query.Where("tags LIKE ? ESCAPE '\\'", "%\""+escapedTag+"\"%")
		}
	}

	if params.Featured != nil && *params.Featured {
		query = query.Where("is_featured = ?", true)
	}

	if params.MinRating > 0 {
		query = query.Where("rating >= ?", params.MinRating)
	}

	if params.AuthorID != nil {
		query = query.Where("author_id = ?", *params.AuthorID)
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count extensions: %w", err)
	}

	// Apply sorting
	orderClause := params.SortBy
	if params.SortOrder == "desc" {
		orderClause += " DESC"
	} else {
		orderClause += " ASC"
	}
	query = query.Order(orderClause)

	// Apply pagination
	offset := (params.Page - 1) * params.PageSize
	query = query.Offset(offset).Limit(params.PageSize)

	// Execute query
	var extensions []Extension
	if err := query.Find(&extensions).Error; err != nil {
		return nil, fmt.Errorf("failed to search extensions: %w", err)
	}

	totalPages := int((total + int64(params.PageSize) - 1) / int64(params.PageSize))

	return &SearchResult{
		Extensions: extensions,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetByID retrieves an extension by ID
func (s *Service) GetByID(ctx context.Context, id uint) (*Extension, error) {
	var ext Extension
	if err := s.db.WithContext(ctx).
		Preload("Versions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(10)
		}).
		Preload("Reviews", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(10)
		}).
		First(&ext, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("extension not found")
		}
		return nil, fmt.Errorf("failed to get extension: %w", err)
	}
	return &ext, nil
}

// GetByName retrieves an extension by name
func (s *Service) GetByName(ctx context.Context, name string) (*Extension, error) {
	var ext Extension
	if err := s.db.WithContext(ctx).Where("name = ?", name).First(&ext).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("extension not found")
		}
		return nil, fmt.Errorf("failed to get extension: %w", err)
	}
	return &ext, nil
}

// PublishRequest contains the data needed to publish an extension
type PublishRequest struct {
	Name        string            `json:"name" binding:"required"`
	DisplayName string            `json:"display_name" binding:"required"`
	Description string            `json:"description"`
	Category    ExtensionCategory `json:"category"`
	Tags        []string          `json:"tags"`
	Manifest    *ExtensionManifest `json:"manifest" binding:"required"`
	SourceURL   string            `json:"source_url" binding:"required"`
	IconURL     string            `json:"icon_url"`
	Screenshots []string          `json:"screenshots"`
	Repository  string            `json:"repository"`
	Homepage    string            `json:"homepage"`
	License     string            `json:"license"`
	Readme      string            `json:"readme"`
}

// Publish publishes a new extension or a new version of an existing extension
func (s *Service) Publish(ctx context.Context, authorID uint, authorName string, req *PublishRequest) (*Extension, error) {
	// Validate extension name
	if !isValidExtensionName(req.Name) {
		return nil, fmt.Errorf("invalid extension name: must be lowercase alphanumeric with hyphens")
	}

	// Validate manifest
	if err := validateManifest(req.Manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	// Marshal manifest to JSON
	manifestJSON, err := json.Marshal(req.Manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize manifest: %w", err)
	}

	// Check if extension exists
	var existingExt Extension
	err = s.db.WithContext(ctx).Where("name = ?", req.Name).First(&existingExt).Error

	if err == nil {
		// Extension exists - check ownership
		if existingExt.AuthorID != authorID {
			return nil, fmt.Errorf("you do not own this extension")
		}

		// Create new version
		newVersion := &ExtensionVersion{
			ExtensionID: existingExt.ID,
			Version:     req.Manifest.Version,
			Manifest:    string(manifestJSON),
			SourceURL:   req.SourceURL,
		}

		if err := s.db.WithContext(ctx).Create(newVersion).Error; err != nil {
			return nil, fmt.Errorf("failed to create version: %w", err)
		}

		// Update extension
		updates := map[string]interface{}{
			"version":        req.Manifest.Version,
			"manifest":       string(manifestJSON),
			"source_url":     req.SourceURL,
			"display_name":   req.DisplayName,
			"description":    req.Description,
			"readme_content": req.Readme,
		}

		if req.IconURL != "" {
			updates["icon_url"] = req.IconURL
		}
		if len(req.Screenshots) > 0 {
			screenshots, _ := json.Marshal(req.Screenshots)
			updates["screenshots"] = string(screenshots)
		}
		if len(req.Tags) > 0 {
			tags, _ := json.Marshal(req.Tags)
			updates["tags"] = string(tags)
		}

		if err := s.db.WithContext(ctx).Model(&existingExt).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("failed to update extension: %w", err)
		}

		return &existingExt, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Create new extension
	tags, _ := json.Marshal(req.Tags)
	screenshots, _ := json.Marshal(req.Screenshots)

	ext := &Extension{
		Name:          req.Name,
		DisplayName:   req.DisplayName,
		Author:        authorName,
		AuthorID:      authorID,
		Description:   req.Description,
		Version:       req.Manifest.Version,
		License:       req.License,
		Repository:    req.Repository,
		Homepage:      req.Homepage,
		Category:      req.Category,
		Tags:          string(tags),
		IconURL:       req.IconURL,
		Screenshots:   string(screenshots),
		SourceURL:     req.SourceURL,
		ReadmeContent: req.Readme,
		Manifest:      string(manifestJSON),
		Status:        StatusPending, // Requires review
	}

	if err := s.db.WithContext(ctx).Create(ext).Error; err != nil {
		return nil, fmt.Errorf("failed to create extension: %w", err)
	}

	// Create initial version
	version := &ExtensionVersion{
		ExtensionID: ext.ID,
		Version:     req.Manifest.Version,
		Manifest:    string(manifestJSON),
		SourceURL:   req.SourceURL,
	}

	if err := s.db.WithContext(ctx).Create(version).Error; err != nil {
		// Don't fail, extension is created
		fmt.Printf("Warning: failed to create version record: %v\n", err)
	}

	return ext, nil
}

// Install installs an extension for a user
func (s *Service) Install(ctx context.Context, userID, extensionID uint) (*UserExtension, error) {
	// Get extension
	var ext Extension
	if err := s.db.WithContext(ctx).First(&ext, extensionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("extension not found")
		}
		return nil, fmt.Errorf("failed to get extension: %w", err)
	}

	// Check if approved
	if ext.Status != StatusApproved {
		return nil, fmt.Errorf("extension is not available for installation")
	}

	// Check if already installed
	var existingInstall UserExtension
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND extension_id = ?", userID, extensionID).
		First(&existingInstall).Error

	if err == nil {
		// Already installed, enable it
		if !existingInstall.Enabled {
			existingInstall.Enabled = true
			if err := s.db.WithContext(ctx).Save(&existingInstall).Error; err != nil {
				return nil, fmt.Errorf("failed to enable extension: %w", err)
			}
		}
		return &existingInstall, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Get manifest permissions
	manifest, _ := ext.ParseManifest()
	var grantedPerms []ExtensionPermission
	if manifest != nil {
		grantedPerms = manifest.Permissions
	}
	grantedPermsJSON, _ := json.Marshal(grantedPerms)

	// Create installation
	install := &UserExtension{
		UserID:             userID,
		ExtensionID:        extensionID,
		Enabled:            true,
		Version:            ext.Version,
		AutoUpdate:         true,
		InstalledAt:        time.Now(),
		Settings:           "{}",
		GrantedPermissions: string(grantedPermsJSON),
	}

	if err := s.db.WithContext(ctx).Create(install).Error; err != nil {
		return nil, fmt.Errorf("failed to install extension: %w", err)
	}

	// Increment download count
	s.db.WithContext(ctx).Model(&ext).
		UpdateColumn("downloads", gorm.Expr("downloads + 1")).
		UpdateColumn("weekly_downloads", gorm.Expr("weekly_downloads + 1"))

	install.Extension = &ext
	return install, nil
}

// Uninstall removes an extension for a user
func (s *Service) Uninstall(ctx context.Context, userID, extensionID uint) error {
	result := s.db.WithContext(ctx).
		Where("user_id = ? AND extension_id = ?", userID, extensionID).
		Delete(&UserExtension{})

	if result.Error != nil {
		return fmt.Errorf("failed to uninstall extension: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("extension not installed")
	}

	return nil
}

// Enable enables an installed extension
func (s *Service) Enable(ctx context.Context, userID, extensionID uint) error {
	result := s.db.WithContext(ctx).
		Model(&UserExtension{}).
		Where("user_id = ? AND extension_id = ?", userID, extensionID).
		Update("enabled", true)

	if result.Error != nil {
		return fmt.Errorf("failed to enable extension: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("extension not installed")
	}

	return nil
}

// Disable disables an installed extension
func (s *Service) Disable(ctx context.Context, userID, extensionID uint) error {
	result := s.db.WithContext(ctx).
		Model(&UserExtension{}).
		Where("user_id = ? AND extension_id = ?", userID, extensionID).
		Update("enabled", false)

	if result.Error != nil {
		return fmt.Errorf("failed to disable extension: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("extension not installed")
	}

	return nil
}

// UpdateSettings updates the user's settings for an extension
func (s *Service) UpdateSettings(ctx context.Context, userID, extensionID uint, settings map[string]interface{}) error {
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to serialize settings: %w", err)
	}

	result := s.db.WithContext(ctx).
		Model(&UserExtension{}).
		Where("user_id = ? AND extension_id = ?", userID, extensionID).
		Update("settings", string(settingsJSON))

	if result.Error != nil {
		return fmt.Errorf("failed to update settings: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("extension not installed")
	}

	return nil
}

// GetUserExtensions retrieves all installed extensions for a user
func (s *Service) GetUserExtensions(ctx context.Context, userID uint) ([]UserExtension, error) {
	var installations []UserExtension
	if err := s.db.WithContext(ctx).
		Preload("Extension").
		Where("user_id = ?", userID).
		Find(&installations).Error; err != nil {
		return nil, fmt.Errorf("failed to get user extensions: %w", err)
	}
	return installations, nil
}

// GetUserExtension retrieves a specific user extension
func (s *Service) GetUserExtension(ctx context.Context, userID, extensionID uint) (*UserExtension, error) {
	var installation UserExtension
	if err := s.db.WithContext(ctx).
		Preload("Extension").
		Where("user_id = ? AND extension_id = ?", userID, extensionID).
		First(&installation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user extension: %w", err)
	}
	return &installation, nil
}

// RateExtension adds or updates a rating for an extension
func (s *Service) RateExtension(ctx context.Context, userID, extensionID uint, rating int, title, content string) error {
	if rating < 1 || rating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}

	// Check if user has installed the extension
	var installation UserExtension
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND extension_id = ?", userID, extensionID).
		First(&installation).Error; err != nil {
		return fmt.Errorf("you must install the extension before rating it")
	}

	// Check for existing review
	var existingReview ExtensionReview
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND extension_id = ?", userID, extensionID).
		First(&existingReview).Error

	if err == nil {
		// Update existing review
		existingReview.Rating = rating
		existingReview.Title = title
		existingReview.Content = content
		existingReview.Version = installation.Version

		if err := s.db.WithContext(ctx).Save(&existingReview).Error; err != nil {
			return fmt.Errorf("failed to update review: %w", err)
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new review
		review := &ExtensionReview{
			ExtensionID: extensionID,
			UserID:      userID,
			Rating:      rating,
			Title:       title,
			Content:     content,
			Version:     installation.Version,
			IsVerified:  true,
		}

		if err := s.db.WithContext(ctx).Create(review).Error; err != nil {
			return fmt.Errorf("failed to create review: %w", err)
		}
	} else {
		return fmt.Errorf("database error: %w", err)
	}

	// Update extension rating
	return s.updateExtensionRating(ctx, extensionID)
}

// updateExtensionRating recalculates the average rating for an extension
func (s *Service) updateExtensionRating(ctx context.Context, extensionID uint) error {
	var result struct {
		AvgRating   float64
		RatingCount int
	}

	if err := s.db.WithContext(ctx).
		Model(&ExtensionReview{}).
		Select("AVG(rating) as avg_rating, COUNT(*) as rating_count").
		Where("extension_id = ?", extensionID).
		Scan(&result).Error; err != nil {
		return fmt.Errorf("failed to calculate rating: %w", err)
	}

	return s.db.WithContext(ctx).
		Model(&Extension{}).
		Where("id = ?", extensionID).
		Updates(map[string]interface{}{
			"rating":       result.AvgRating,
			"rating_count": result.RatingCount,
		}).Error
}

// GetCategories returns all available categories with counts
func (s *Service) GetCategories(ctx context.Context) ([]map[string]interface{}, error) {
	type CategoryCount struct {
		Category ExtensionCategory
		Count    int64
	}

	var results []CategoryCount
	if err := s.db.WithContext(ctx).
		Model(&Extension{}).
		Select("category, COUNT(*) as count").
		Where("status = ?", StatusApproved).
		Group("category").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}

	categories := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		categories = append(categories, map[string]interface{}{
			"id":    r.Category,
			"name":  getCategoryName(r.Category),
			"count": r.Count,
		})
	}

	return categories, nil
}

// GetFeatured returns featured extensions
func (s *Service) GetFeatured(ctx context.Context, limit int) ([]Extension, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	var extensions []Extension
	if err := s.db.WithContext(ctx).
		Where("status = ? AND is_featured = ?", StatusApproved, true).
		Order("downloads DESC").
		Limit(limit).
		Find(&extensions).Error; err != nil {
		return nil, fmt.Errorf("failed to get featured extensions: %w", err)
	}

	return extensions, nil
}

// GetTrending returns trending extensions (based on weekly downloads)
func (s *Service) GetTrending(ctx context.Context, limit int) ([]Extension, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	var extensions []Extension
	if err := s.db.WithContext(ctx).
		Where("status = ?", StatusApproved).
		Order("weekly_downloads DESC").
		Limit(limit).
		Find(&extensions).Error; err != nil {
		return nil, fmt.Errorf("failed to get trending extensions: %w", err)
	}

	return extensions, nil
}

// GetRecommended returns personalized recommendations for a user
func (s *Service) GetRecommended(ctx context.Context, userID uint, limit int) ([]Extension, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	// Get user's installed extension categories
	var userCategories []ExtensionCategory
	if err := s.db.WithContext(ctx).
		Model(&UserExtension{}).
		Select("DISTINCT extensions.category").
		Joins("JOIN extensions ON extensions.id = user_extensions.extension_id").
		Where("user_extensions.user_id = ?", userID).
		Pluck("category", &userCategories).Error; err != nil {
		return nil, fmt.Errorf("failed to get user categories: %w", err)
	}

	// Get installed extension IDs
	var installedIDs []uint
	s.db.WithContext(ctx).
		Model(&UserExtension{}).
		Where("user_id = ?", userID).
		Pluck("extension_id", &installedIDs)

	query := s.db.WithContext(ctx).
		Where("status = ?", StatusApproved)

	if len(installedIDs) > 0 {
		query = query.Where("id NOT IN ?", installedIDs)
	}

	// Prefer same categories
	if len(userCategories) > 0 {
		query = query.Where("category IN ?", userCategories)
	}

	var extensions []Extension
	if err := query.
		Order("rating DESC, downloads DESC").
		Limit(limit).
		Find(&extensions).Error; err != nil {
		return nil, fmt.Errorf("failed to get recommended extensions: %w", err)
	}

	// If not enough recommendations, get popular ones
	if len(extensions) < limit {
		remaining := limit - len(extensions)
		existingIDs := make([]uint, len(extensions))
		for i, ext := range extensions {
			existingIDs[i] = ext.ID
		}

		excludeIDs := append(installedIDs, existingIDs...)

		var moreExtensions []Extension
		query := s.db.WithContext(ctx).
			Where("status = ?", StatusApproved)

		if len(excludeIDs) > 0 {
			query = query.Where("id NOT IN ?", excludeIDs)
		}

		if err := query.
			Order("downloads DESC").
			Limit(remaining).
			Find(&moreExtensions).Error; err == nil {
			extensions = append(extensions, moreExtensions...)
		}
	}

	return extensions, nil
}

// ApproveExtension approves an extension (admin only)
func (s *Service) ApproveExtension(ctx context.Context, extensionID, reviewerID uint, notes string) error {
	now := time.Now()
	return s.db.WithContext(ctx).
		Model(&Extension{}).
		Where("id = ?", extensionID).
		Updates(map[string]interface{}{
			"status":       StatusApproved,
			"reviewed_at":  &now,
			"reviewed_by":  reviewerID,
			"review_notes": notes,
		}).Error
}

// RejectExtension rejects an extension (admin only)
func (s *Service) RejectExtension(ctx context.Context, extensionID, reviewerID uint, notes string) error {
	now := time.Now()
	return s.db.WithContext(ctx).
		Model(&Extension{}).
		Where("id = ?", extensionID).
		Updates(map[string]interface{}{
			"status":       StatusRejected,
			"reviewed_at":  &now,
			"reviewed_by":  reviewerID,
			"review_notes": notes,
		}).Error
}

// GetPendingExtensions returns extensions pending review (admin only)
func (s *Service) GetPendingExtensions(ctx context.Context) ([]Extension, error) {
	var extensions []Extension
	if err := s.db.WithContext(ctx).
		Where("status = ?", StatusPending).
		Order("created_at ASC").
		Find(&extensions).Error; err != nil {
		return nil, fmt.Errorf("failed to get pending extensions: %w", err)
	}
	return extensions, nil
}

// DownloadExtensionBundle downloads and returns the extension source
func (s *Service) DownloadExtensionBundle(ctx context.Context, extensionID uint) ([]byte, error) {
	var ext Extension
	if err := s.db.WithContext(ctx).First(&ext, extensionID).Error; err != nil {
		return nil, fmt.Errorf("extension not found: %w", err)
	}

	if ext.SourceURL == "" {
		return nil, fmt.Errorf("extension has no source URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ext.SourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download extension: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download extension: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read extension data: %w", err)
	}

	return data, nil
}

// Helper functions

func isValidExtensionName(name string) bool {
	// Extension names must be lowercase alphanumeric with hyphens
	match, _ := regexp.MatchString(`^[a-z][a-z0-9-]*[a-z0-9]$`, name)
	return match && len(name) >= 3 && len(name) <= 100
}

func validateManifest(manifest *ExtensionManifest) error {
	if manifest == nil {
		return errors.New("manifest is required")
	}

	if manifest.Name == "" {
		return errors.New("name is required")
	}

	if manifest.Version == "" {
		return errors.New("version is required")
	}

	// Validate version format (semver)
	if !isValidSemver(manifest.Version) {
		return errors.New("invalid version format (must be semver)")
	}

	// Validate permissions
	validPerms := map[ExtensionPermission]bool{
		PermissionFileRead:     true,
		PermissionFileWrite:    true,
		PermissionTerminal:     true,
		PermissionNetwork:      true,
		PermissionStorage:      true,
		PermissionClipboard:    true,
		PermissionNotification: true,
		PermissionAI:           true,
		PermissionSecrets:      true,
		PermissionGit:          true,
	}

	for _, perm := range manifest.Permissions {
		if !validPerms[perm] {
			return fmt.Errorf("invalid permission: %s", perm)
		}
	}

	return nil
}

func isValidSemver(version string) bool {
	// Simple semver validation
	match, _ := regexp.MatchString(`^\d+\.\d+\.\d+(-[\w.]+)?(\+[\w.]+)?$`, version)
	return match
}

func getCategoryName(category ExtensionCategory) string {
	names := map[ExtensionCategory]string{
		CategoryTheme:      "Themes",
		CategoryLanguage:   "Language Support",
		CategoryFormatter:  "Formatters",
		CategoryLinter:     "Linters",
		CategorySnippets:   "Snippets",
		CategoryKeybinding: "Keybindings",
		CategoryWidget:     "Widgets",
		CategoryAI:         "AI Tools",
		CategoryDebugger:   "Debuggers",
		CategoryOther:      "Other",
	}

	if name, ok := names[category]; ok {
		return name
	}
	return string(category)
}

// GetTopExtensions returns the top extensions sorted by a specific field
func (s *Service) GetTopExtensions(ctx context.Context, sortBy string, limit int) ([]Extension, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	validSortFields := map[string]bool{
		"downloads":        true,
		"rating":           true,
		"weekly_downloads": true,
		"created_at":       true,
		"updated_at":       true,
	}

	if !validSortFields[sortBy] {
		sortBy = "downloads"
	}

	var extensions []Extension
	if err := s.db.WithContext(ctx).
		Where("status = ?", StatusApproved).
		Order(sortBy + " DESC").
		Limit(limit).
		Find(&extensions).Error; err != nil {
		return nil, fmt.Errorf("failed to get top extensions: %w", err)
	}

	return extensions, nil
}

// GetExtensionsByAuthor returns all extensions by an author
func (s *Service) GetExtensionsByAuthor(ctx context.Context, authorID uint) ([]Extension, error) {
	var extensions []Extension
	if err := s.db.WithContext(ctx).
		Where("author_id = ?", authorID).
		Order("created_at DESC").
		Find(&extensions).Error; err != nil {
		return nil, fmt.Errorf("failed to get author extensions: %w", err)
	}
	return extensions, nil
}

// GetExtensionReviews returns reviews for an extension
func (s *Service) GetExtensionReviews(ctx context.Context, extensionID uint, page, pageSize int) ([]ExtensionReview, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}

	var total int64
	if err := s.db.WithContext(ctx).
		Model(&ExtensionReview{}).
		Where("extension_id = ?", extensionID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count reviews: %w", err)
	}

	offset := (page - 1) * pageSize

	var reviews []ExtensionReview
	if err := s.db.WithContext(ctx).
		Where("extension_id = ?", extensionID).
		Order("helpful_count DESC, created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&reviews).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get reviews: %w", err)
	}

	return reviews, total, nil
}

// GetAvailableVersions returns all versions of an extension
func (s *Service) GetAvailableVersions(ctx context.Context, extensionID uint) ([]ExtensionVersion, error) {
	var versions []ExtensionVersion
	if err := s.db.WithContext(ctx).
		Where("extension_id = ? AND is_yanked = ?", extensionID, false).
		Order("created_at DESC").
		Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("failed to get versions: %w", err)
	}
	return versions, nil
}

// ResetWeeklyDownloads resets weekly download counts (should be called weekly via cron)
func (s *Service) ResetWeeklyDownloads(ctx context.Context) error {
	return s.db.WithContext(ctx).
		Model(&Extension{}).
		Where("1 = 1").
		Update("weekly_downloads", 0).Error
}

// CalculatePopularTags returns the most popular tags
func (s *Service) CalculatePopularTags(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var extensions []Extension
	if err := s.db.WithContext(ctx).
		Where("status = ? AND tags != ''", StatusApproved).
		Select("tags").
		Find(&extensions).Error; err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}

	// Count tag occurrences
	tagCounts := make(map[string]int)
	for _, ext := range extensions {
		tags := ext.GetTags()
		for _, tag := range tags {
			tagCounts[tag]++
		}
	}

	// Sort tags by count
	type tagCount struct {
		tag   string
		count int
	}
	var tagList []tagCount
	for tag, count := range tagCounts {
		tagList = append(tagList, tagCount{tag, count})
	}
	sort.Slice(tagList, func(i, j int) bool {
		return tagList[i].count > tagList[j].count
	})

	// Return top tags
	result := make([]string, 0, limit)
	for i := 0; i < len(tagList) && i < limit; i++ {
		result = append(result, tagList[i].tag)
	}

	return result, nil
}
