// APEX.BUILD One-Click Deployment Service
// Core deployment orchestration and management

package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeploymentStatus represents the current state of a deployment
type DeploymentStatus string

const (
	StatusPending   DeploymentStatus = "pending"
	StatusPreparing DeploymentStatus = "preparing"
	StatusBuilding  DeploymentStatus = "building"
	StatusDeploying DeploymentStatus = "deploying"
	StatusLive      DeploymentStatus = "live"
	StatusFailed    DeploymentStatus = "failed"
	StatusCancelled DeploymentStatus = "cancelled"
)

// DeploymentProvider represents supported deployment providers
type DeploymentProvider string

const (
	ProviderVercel          DeploymentProvider = "vercel"
	ProviderNetlify         DeploymentProvider = "netlify"
	ProviderRender          DeploymentProvider = "render"
	ProviderRailway         DeploymentProvider = "railway"
	ProviderCloudflarePages DeploymentProvider = "cloudflare_pages"
)

// DatabaseProvider represents supported managed database orchestration providers.
type DatabaseProvider string

const (
	DatabaseProviderNeon DatabaseProvider = "neon"
)

// Deployment represents a single deployment instance
type Deployment struct {
	ID           string                 `json:"id" gorm:"primarykey;type:varchar(36)"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	DeletedAt    gorm.DeletedAt         `json:"-" gorm:"index"`
	ProjectID    uint                   `json:"project_id" gorm:"not null;index"`
	UserID       uint                   `json:"user_id" gorm:"not null;index"`
	Provider     DeploymentProvider     `json:"provider" gorm:"not null;type:varchar(50)"`
	Status       DeploymentStatus       `json:"status" gorm:"not null;type:varchar(50);default:'pending'"`
	URL          string                 `json:"url,omitempty"`
	PreviewURL   string                 `json:"preview_url,omitempty"`
	Environment  string                 `json:"environment" gorm:"default:'production'"` // production, preview, development
	Branch       string                 `json:"branch" gorm:"default:'main'"`
	CommitSHA    string                 `json:"commit_sha,omitempty"`
	CommitMsg    string                 `json:"commit_msg,omitempty"`
	BuildTime    int64                  `json:"build_time,omitempty"`  // milliseconds
	DeployTime   int64                  `json:"deploy_time,omitempty"` // milliseconds
	TotalTime    int64                  `json:"total_time,omitempty"`  // milliseconds
	ErrorMessage string                 `json:"error_message,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty" gorm:"serializer:json"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" gorm:"serializer:json"`
	StartedAt    *time.Time             `json:"started_at,omitempty"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
}

// DeploymentLog represents a log entry for a deployment
type DeploymentLog struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	DeploymentID string    `json:"deployment_id" gorm:"not null;index;type:varchar(36)"`
	Timestamp    time.Time `json:"timestamp" gorm:"not null"`
	Level        string    `json:"level" gorm:"not null;type:varchar(20)"` // info, warn, error, debug
	Message      string    `json:"message" gorm:"type:text"`
	Phase        string    `json:"phase,omitempty" gorm:"type:varchar(50)"` // prepare, build, deploy
	Metadata     string    `json:"metadata,omitempty" gorm:"type:text"`
}

// DeploymentConfig contains configuration for a deployment
type DeploymentConfig struct {
	ProjectID     uint                   `json:"project_id"`
	Provider      DeploymentProvider     `json:"provider"`
	Environment   string                 `json:"environment"`
	Branch        string                 `json:"branch"`
	EnvVars       map[string]string      `json:"env_vars,omitempty"`
	BuildCommand  string                 `json:"build_command,omitempty"`
	OutputDir     string                 `json:"output_dir,omitempty"`
	InstallCmd    string                 `json:"install_cmd,omitempty"`
	StartCommand  string                 `json:"start_command,omitempty"`
	Framework     string                 `json:"framework,omitempty"`
	NodeVersion   string                 `json:"node_version,omitempty"`
	RootDirectory string                 `json:"root_directory,omitempty"`
	Database      *DatabaseConfig        `json:"database,omitempty"`
	Custom        map[string]interface{} `json:"custom,omitempty"`
}

// DatabaseConfig contains configuration for an optional managed database
// provisioned alongside a deployment.
type DatabaseConfig struct {
	Provider     DatabaseProvider `json:"provider"`
	ProjectName  string           `json:"project_name,omitempty"`
	BranchName   string           `json:"branch_name,omitempty"`
	DatabaseName string           `json:"database_name,omitempty"`
	RoleName     string           `json:"role_name,omitempty"`
	RegionID     string           `json:"region_id,omitempty"`
	OrgID        string           `json:"org_id,omitempty"`
	PGVersion    int              `json:"pg_version,omitempty"`
	Pooled       bool             `json:"pooled,omitempty"`
}

// DeploymentResult contains the result of a deployment operation
type DeploymentResult struct {
	DeploymentID string           `json:"deployment_id"`
	Status       DeploymentStatus `json:"status"`
	URL          string           `json:"url,omitempty"`
	PreviewURL   string           `json:"preview_url,omitempty"`
	Logs         []DeploymentLog  `json:"logs,omitempty"`
	Error        string           `json:"error,omitempty"`
}

// Provider interface for deployment providers
type Provider interface {
	Name() DeploymentProvider
	Deploy(ctx context.Context, config *DeploymentConfig, files []ProjectFile) (*ProviderDeploymentResult, error)
	GetStatus(ctx context.Context, deploymentID string) (*ProviderDeploymentResult, error)
	Cancel(ctx context.Context, deploymentID string) error
	GetLogs(ctx context.Context, deploymentID string) ([]string, error)
	ValidateConfig(config *DeploymentConfig) error
}

// ProviderDeploymentResult is the result from a provider
type ProviderDeploymentResult struct {
	ProviderID   string           `json:"provider_id"`
	Status       DeploymentStatus `json:"status"`
	URL          string           `json:"url,omitempty"`
	PreviewURL   string           `json:"preview_url,omitempty"`
	BuildLogs    []string         `json:"build_logs,omitempty"`
	ErrorMessage string           `json:"error_message,omitempty"`
	Metadata     map[string]any   `json:"metadata,omitempty"`
}

// DatabaseProvisioner provisions or reuses managed database infrastructure for
// a deployment and returns runtime environment variables plus provider metadata.
type DatabaseProvisioner interface {
	Name() DatabaseProvider
	ValidateConfig(config *DatabaseConfig) error
	EnsureDatabase(ctx context.Context, config *DeploymentConfig) (*ProvisionedDatabaseResult, error)
}

// ProvisionedDatabaseResult contains the deployment-facing output of a managed
// database orchestration step. Sensitive credentials live only in EnvVars.
type ProvisionedDatabaseResult struct {
	EnvVars  map[string]string `json:"env_vars,omitempty"`
	Logs     []string          `json:"logs,omitempty"`
	Metadata map[string]any    `json:"metadata,omitempty"`
}

// ProjectFile represents a file to be deployed
type ProjectFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
	IsDir    bool   `json:"is_dir"`
}

// DeploymentService handles all deployment operations
type DeploymentService struct {
	db        *gorm.DB
	builder   *BuildService
	providers map[DeploymentProvider]Provider
	databases map[DatabaseProvider]DatabaseProvisioner
	tokens    map[DeploymentProvider]string // API tokens for providers
	mu        sync.RWMutex
	active    map[string]context.CancelFunc // active deployment cancellation functions

	monitorPollInterval time.Duration
}

// NewDeploymentService creates a new deployment service
func NewDeploymentService(db *gorm.DB, vercelToken, netlifyToken, renderToken, railwayToken, cloudflarePagesToken string) *DeploymentService {
	svc := &DeploymentService{
		db:                  db,
		builder:             NewBuildService(),
		providers:           make(map[DeploymentProvider]Provider),
		databases:           make(map[DatabaseProvider]DatabaseProvisioner),
		active:              make(map[string]context.CancelFunc),
		monitorPollInterval: 8 * time.Second,
	}

	// Store tokens for lazy provider initialization
	svc.tokens = map[DeploymentProvider]string{
		ProviderVercel:          vercelToken,
		ProviderNetlify:         netlifyToken,
		ProviderRender:          renderToken,
		ProviderRailway:         railwayToken,
		ProviderCloudflarePages: cloudflarePagesToken,
	}

	return svc
}

// RegisterProvider registers a deployment provider
func (s *DeploymentService) RegisterProvider(providerType DeploymentProvider, provider Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[providerType] = provider
}

// RegisterDatabaseProvisioner registers a managed database provisioner.
func (s *DeploymentService) RegisterDatabaseProvisioner(providerType DatabaseProvider, provisioner DatabaseProvisioner) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.databases[providerType] = provisioner
}

// StartDeployment initiates a new deployment
func (s *DeploymentService) StartDeployment(ctx context.Context, userID uint, config *DeploymentConfig) (*Deployment, error) {
	// Validate provider
	provider, ok := s.providers[config.Provider]
	if !ok {
		return nil, fmt.Errorf("provider %s is not configured", config.Provider)
	}

	// Validate configuration
	if err := provider.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid deployment config: %w", err)
	}
	if err := s.validateDatabaseConfig(config); err != nil {
		return nil, err
	}

	// Create deployment record
	deployment := &Deployment{
		ID:          uuid.New().String(),
		ProjectID:   config.ProjectID,
		UserID:      userID,
		Provider:    config.Provider,
		Status:      StatusPending,
		Environment: config.Environment,
		Branch:      config.Branch,
		Config: map[string]interface{}{
			"build_command":  config.BuildCommand,
			"output_dir":     config.OutputDir,
			"install_cmd":    config.InstallCmd,
			"start_command":  config.StartCommand,
			"framework":      config.Framework,
			"node_version":   config.NodeVersion,
			"root_directory": config.RootDirectory,
			"database":       config.Database,
			"custom":         config.Custom,
		},
		Metadata: make(map[string]interface{}),
	}

	if err := s.db.Create(deployment).Error; err != nil {
		return nil, fmt.Errorf("failed to create deployment record: %w", err)
	}

	// Start deployment in background
	go s.executeDeployment(deployment, config, provider)

	return deployment, nil
}

// executeDeployment runs the deployment process
func (s *DeploymentService) executeDeployment(deployment *Deployment, config *DeploymentConfig, provider Provider) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Register cancellation function
	s.mu.Lock()
	s.active[deployment.ID] = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.active, deployment.ID)
		s.mu.Unlock()
	}()

	startTime := time.Now()
	deployment.StartedAt = &startTime
	s.updateStatus(deployment, StatusPreparing, "")
	s.addLog(deployment.ID, "info", "Preparing deployment...", "prepare")

	// Get project files from database
	var files []struct {
		Path     string
		Content  string
		Size     int64
		MimeType string
		Type     string
	}
	if err := s.db.Table("files").
		Select("path, content, size, mime_type, type").
		Where("project_id = ?", config.ProjectID).
		Find(&files).Error; err != nil {
		s.failDeployment(deployment, fmt.Sprintf("Failed to fetch project files: %v", err))
		return
	}

	// Convert to ProjectFile format
	projectFiles := make([]ProjectFile, len(files))
	for i, f := range files {
		projectFiles[i] = ProjectFile{
			Path:     f.Path,
			Content:  f.Content,
			Size:     f.Size,
			MimeType: f.MimeType,
			IsDir:    f.Type == "directory",
		}
	}

	// Detect project type and prepare build
	projectType := s.builder.DetectProjectType(projectFiles)
	s.addLog(deployment.ID, "info", fmt.Sprintf("Detected project type: %s", projectType), "prepare")

	// Generate deployment configuration defaults when runtime/build fields are missing.
	if config.BuildCommand == "" || config.OutputDir == "" || config.InstallCmd == "" ||
		config.StartCommand == "" || config.Framework == "" || config.NodeVersion == "" {
		applyGeneratedDeploymentDefaults(config, s.builder.GenerateBuildConfig(projectType, projectFiles))
	}

	if config.Database != nil {
		provisioner, ok := s.databases[config.Database.Provider]
		if !ok {
			s.failDeployment(deployment, fmt.Sprintf("Managed database provider %s is not configured", config.Database.Provider))
			return
		}
		s.addLog(deployment.ID, "info", fmt.Sprintf("Provisioning %s database resources...", config.Database.Provider), "prepare")
		dbResult, err := provisioner.EnsureDatabase(ctx, config)
		if err != nil {
			s.failDeployment(deployment, fmt.Sprintf("Failed to provision managed database: %v", err))
			return
		}
		for key, value := range dbResult.EnvVars {
			if config.EnvVars == nil {
				config.EnvVars = make(map[string]string)
			}
			config.EnvVars[key] = value
		}
		s.mergeDeploymentMetadata(deployment, dbResult.Metadata)
		for _, line := range dbResult.Logs {
			s.addLog(deployment.ID, "info", line, "prepare")
		}
		s.addLog(deployment.ID, "info", fmt.Sprintf("Managed %s database ready; runtime env updated", config.Database.Provider), "prepare")
	}

	s.addLog(deployment.ID, "info", fmt.Sprintf("Build command: %s", config.BuildCommand), "prepare")
	s.addLog(deployment.ID, "info", fmt.Sprintf("Output directory: %s", config.OutputDir), "prepare")

	// Update status to building
	s.updateStatus(deployment, StatusBuilding, "")
	buildStartTime := time.Now()
	s.addLog(deployment.ID, "info", "Starting build process...", "build")

	// Create deployable package
	packageData, err := s.builder.PackageProject(projectFiles, config)
	if err != nil {
		s.failDeployment(deployment, fmt.Sprintf("Failed to package project: %v", err))
		return
	}

	buildDuration := time.Since(buildStartTime).Milliseconds()
	deployment.BuildTime = buildDuration
	s.addLog(deployment.ID, "info", fmt.Sprintf("Build completed in %dms", buildDuration), "build")

	// Update status to deploying
	s.updateStatus(deployment, StatusDeploying, "")
	deployStartTime := time.Now()
	s.addLog(deployment.ID, "info", fmt.Sprintf("Deploying to %s...", config.Provider), "deploy")

	// Deploy to provider
	result, err := provider.Deploy(ctx, config, packageData)
	if err != nil {
		s.failDeployment(deployment, fmt.Sprintf("Deployment failed: %v", err))
		return
	}

	// Add provider logs
	for _, log := range result.BuildLogs {
		s.addLog(deployment.ID, "info", log, "deploy")
	}

	s.applyProviderResult(deployment, result)
	switch result.Status {
	case StatusFailed:
		s.failDeployment(deployment, firstNonEmpty(result.ErrorMessage, "Deployment failed"))
		return
	case StatusCancelled:
		completedAt := time.Now()
		deployment.CompletedAt = &completedAt
		s.updateStatus(deployment, StatusCancelled, firstNonEmpty(result.ErrorMessage, "Deployment cancelled"))
		s.addLog(deployment.ID, "warn", "Deployment cancelled by provider", "deploy")
		return
	case StatusLive:
		s.completeSuccessfulDeployment(deployment, deployStartTime, startTime)
		return
	default:
		status := result.Status
		if status == "" {
			status = StatusDeploying
		}
		s.updateStatus(deployment, status, "")
		s.addLog(deployment.ID, "info", fmt.Sprintf("Provider reported %s; monitoring until terminal state", status), "deploy")
		if err := s.monitorProviderDeployment(ctx, deployment, provider, result.ProviderID, deployStartTime, startTime, result.BuildLogs); err != nil {
			if errors.Is(err, context.Canceled) && s.isDeploymentCancelled(deployment.ID) {
				return
			}
			s.failDeployment(deployment, fmt.Sprintf("Deployment monitoring failed: %v", err))
		}
	}
}

// GetDeploymentStatus returns the current status of a deployment
func (s *DeploymentService) GetDeploymentStatus(deploymentID string) (*Deployment, error) {
	var deployment Deployment
	if err := s.db.First(&deployment, "id = ?", deploymentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("deployment not found")
		}
		return nil, err
	}
	return &deployment, nil
}

// CancelDeployment cancels an in-progress deployment
func (s *DeploymentService) CancelDeployment(deploymentID string) error {
	s.mu.RLock()
	cancel, exists := s.active[deploymentID]
	s.mu.RUnlock()

	if exists {
		cancel()
	}

	var deployment Deployment
	if err := s.db.First(&deployment, "id = ?", deploymentID).Error; err != nil {
		return err
	}

	if deployment.Status == StatusLive || deployment.Status == StatusFailed || deployment.Status == StatusCancelled {
		return fmt.Errorf("cannot cancel deployment in %s state", deployment.Status)
	}

	deployment.Status = StatusCancelled
	completedAt := time.Now()
	deployment.CompletedAt = &completedAt

	if err := s.db.Save(&deployment).Error; err != nil {
		return err
	}

	s.addLog(deploymentID, "warn", "Deployment cancelled by user", "")
	return nil
}

// GetDeploymentLogs returns logs for a deployment
func (s *DeploymentService) GetDeploymentLogs(deploymentID string, limit int) ([]DeploymentLog, error) {
	var logs []DeploymentLog
	query := s.db.Where("deployment_id = ?", deploymentID).Order("timestamp ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// GetDeploymentHistory returns deployment history for a project
func (s *DeploymentService) GetDeploymentHistory(projectID uint, page, limit int) ([]Deployment, int64, error) {
	var deployments []Deployment
	var total int64

	s.db.Model(&Deployment{}).Where("project_id = ?", projectID).Count(&total)

	offset := (page - 1) * limit
	if err := s.db.Where("project_id = ?", projectID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&deployments).Error; err != nil {
		return nil, 0, err
	}

	return deployments, total, nil
}

// GetAvailableProviders returns configured providers
func (s *DeploymentService) GetAvailableProviders() []map[string]interface{} {
	providers := make([]map[string]interface{}, 0)
	for name := range s.providers {
		providers = append(providers, map[string]interface{}{
			"id":          string(name),
			"name":        getProviderDisplayName(name),
			"description": getProviderDescription(name),
			"features":    getProviderFeatures(name),
		})
	}
	sort.Slice(providers, func(i, j int) bool {
		left, _ := providers[i]["id"].(string)
		right, _ := providers[j]["id"].(string)
		return left < right
	})
	return providers
}

// Redeploy triggers a redeployment using the same configuration
func (s *DeploymentService) Redeploy(ctx context.Context, deploymentID string, userID uint) (*Deployment, error) {
	var original Deployment
	if err := s.db.First(&original, "id = ?", deploymentID).Error; err != nil {
		return nil, err
	}

	// Verify user owns the deployment
	if original.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}

	return s.StartDeployment(ctx, userID, restoreDeploymentConfig(&original))
}

// Helper functions

func (s *DeploymentService) updateStatus(deployment *Deployment, status DeploymentStatus, errorMsg string) {
	deployment.Status = status
	if errorMsg != "" {
		deployment.ErrorMessage = errorMsg
	}
	s.db.Save(deployment)
}

func (s *DeploymentService) failDeployment(deployment *Deployment, errorMsg string) {
	completedAt := time.Now()
	deployment.CompletedAt = &completedAt
	s.updateStatus(deployment, StatusFailed, errorMsg)
	s.addLog(deployment.ID, "error", errorMsg, "")
}

func (s *DeploymentService) addLog(deploymentID, level, message, phase string) {
	log := DeploymentLog{
		DeploymentID: deploymentID,
		Timestamp:    time.Now(),
		Level:        level,
		Message:      message,
		Phase:        phase,
	}
	s.db.Create(&log)
}

func (s *DeploymentService) validateDatabaseConfig(config *DeploymentConfig) error {
	if config == nil || config.Database == nil {
		return nil
	}
	provisioner, ok := s.databases[config.Database.Provider]
	if !ok {
		return fmt.Errorf("database provider %s is not configured", config.Database.Provider)
	}
	if err := provisioner.ValidateConfig(config.Database); err != nil {
		return fmt.Errorf("invalid database config: %w", err)
	}
	for _, key := range []string{"DATABASE_URL", "POSTGRES_URL", "PGHOST", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD"} {
		if _, exists := config.EnvVars[key]; exists {
			return fmt.Errorf("environment variable %s cannot be set when managed database orchestration is enabled", key)
		}
	}
	return nil
}

func (s *DeploymentService) mergeDeploymentMetadata(deployment *Deployment, metadata map[string]any) {
	if deployment == nil || len(metadata) == 0 {
		return
	}
	if deployment.Metadata == nil {
		deployment.Metadata = make(map[string]interface{})
	}
	for key, value := range metadata {
		deployment.Metadata[key] = value
	}
	custom := map[string]any{}
	if deployment.Config == nil {
		deployment.Config = make(map[string]interface{})
	}
	if existing, ok := deployment.Config["custom"].(map[string]any); ok {
		for key, value := range existing {
			custom[key] = value
		}
	} else if existing, ok := deployment.Config["custom"].(map[string]interface{}); ok {
		for key, value := range existing {
			custom[key] = value
		}
	}
	for key, value := range metadata {
		custom[key] = value
	}
	deployment.Config["custom"] = custom
}

func (s *DeploymentService) applyProviderResult(deployment *Deployment, result *ProviderDeploymentResult) {
	if deployment == nil || result == nil {
		return
	}
	deployment.URL = result.URL
	deployment.PreviewURL = result.PreviewURL
	s.mergeDeploymentMetadata(deployment, result.Metadata)
	if deployment.Metadata == nil {
		deployment.Metadata = make(map[string]interface{})
	}
	deployment.Metadata["provider_id"] = result.ProviderID
	s.db.Save(deployment)
}

func (s *DeploymentService) completeSuccessfulDeployment(deployment *Deployment, deployStartTime, startTime time.Time) {
	if deployment == nil {
		return
	}
	deployDuration := time.Since(deployStartTime).Milliseconds()
	deployment.DeployTime = deployDuration
	totalDuration := time.Since(startTime).Milliseconds()
	deployment.TotalTime = totalDuration
	completedAt := time.Now()
	deployment.CompletedAt = &completedAt
	s.updateStatus(deployment, StatusLive, "")
	if deployment.URL != "" {
		s.addLog(deployment.ID, "info", fmt.Sprintf("Deployment live at: %s", deployment.URL), "deploy")
	}
	s.addLog(deployment.ID, "info", fmt.Sprintf("Total deployment time: %dms", totalDuration), "deploy")
}

func (s *DeploymentService) monitorProviderDeployment(ctx context.Context, deployment *Deployment, provider Provider, providerID string, deployStartTime, startTime time.Time, initialLogs []string) error {
	if providerID == "" {
		return fmt.Errorf("provider deployment reference is missing")
	}
	seenLogs := make(map[string]struct{}, len(initialLogs))
	for _, line := range initialLogs {
		if line == "" {
			continue
		}
		seenLogs[line] = struct{}{}
	}
	lastStatus := deployment.Status
	pollInterval := s.monitorPollInterval
	if pollInterval <= 0 {
		pollInterval = 8 * time.Second
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if logs, err := provider.GetLogs(ctx, providerID); err == nil {
				for _, line := range logs {
					if _, exists := seenLogs[line]; exists || line == "" {
						continue
					}
					seenLogs[line] = struct{}{}
					s.addLog(deployment.ID, "info", line, "deploy")
				}
			}

			statusResult, err := provider.GetStatus(ctx, providerID)
			if err != nil {
				continue
			}
			s.applyProviderResult(deployment, statusResult)

			if statusResult.Status != "" && statusResult.Status != lastStatus {
				lastStatus = statusResult.Status
				s.updateStatus(deployment, statusResult.Status, "")
				s.addLog(deployment.ID, "info", fmt.Sprintf("Provider status changed to %s", statusResult.Status), "deploy")
			}

			switch statusResult.Status {
			case StatusLive:
				s.completeSuccessfulDeployment(deployment, deployStartTime, startTime)
				return nil
			case StatusFailed:
				return fmt.Errorf("%s", firstNonEmpty(statusResult.ErrorMessage, "provider reported failed status"))
			case StatusCancelled:
				completedAt := time.Now()
				deployment.CompletedAt = &completedAt
				s.updateStatus(deployment, StatusCancelled, firstNonEmpty(statusResult.ErrorMessage, "Deployment cancelled"))
				s.addLog(deployment.ID, "warn", "Deployment cancelled by provider", "deploy")
				return nil
			}
		}
	}
}

func (s *DeploymentService) isDeploymentCancelled(deploymentID string) bool {
	var deployment Deployment
	if err := s.db.Select("status").First(&deployment, "id = ?", deploymentID).Error; err != nil {
		return false
	}
	return deployment.Status == StatusCancelled
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func getProviderDisplayName(provider DeploymentProvider) string {
	switch provider {
	case ProviderVercel:
		return "Vercel"
	case ProviderNetlify:
		return "Netlify"
	case ProviderRender:
		return "Render"
	case ProviderRailway:
		return "Railway"
	case ProviderCloudflarePages:
		return "Cloudflare Pages"
	default:
		return string(provider)
	}
}

func getProviderDescription(provider DeploymentProvider) string {
	switch provider {
	case ProviderVercel:
		return "Deploy frontend applications with automatic HTTPS and global CDN"
	case ProviderNetlify:
		return "Deploy static sites and serverless functions with instant rollbacks"
	case ProviderRender:
		return "Deploy full-stack applications with managed databases and services"
	case ProviderRailway:
		return "Deploy full-stack applications with Railpack builds and managed service networking"
	case ProviderCloudflarePages:
		return "Deploy static frontends to Cloudflare's global edge with Pages projects"
	default:
		return ""
	}
}

func getProviderFeatures(provider DeploymentProvider) []string {
	switch provider {
	case ProviderVercel:
		return []string{"Edge Functions", "Automatic HTTPS", "Global CDN", "Preview Deployments", "Analytics"}
	case ProviderNetlify:
		return []string{"Serverless Functions", "Forms", "Identity", "Split Testing", "Large Media"}
	case ProviderRender:
		return []string{"Background Workers", "Cron Jobs", "Managed Databases", "Private Networking", "Auto-scaling"}
	case ProviderRailway:
		return []string{"Railpack Builds", "Generated Domains", "Service Variables", "Private Networking", "Managed Databases"}
	case ProviderCloudflarePages:
		return []string{"Global Edge CDN", "Preview Branches", "Static Assets", "Pages Functions", "Wrangler Deploys"}
	default:
		return []string{}
	}
}

func applyGeneratedDeploymentDefaults(config *DeploymentConfig, buildConfig *BuildConfig) {
	if config == nil || buildConfig == nil {
		return
	}
	if config.BuildCommand == "" {
		config.BuildCommand = buildConfig.BuildCommand
	}
	if config.OutputDir == "" {
		config.OutputDir = buildConfig.OutputDir
	}
	if config.InstallCmd == "" {
		config.InstallCmd = buildConfig.InstallCommand
	}
	if config.StartCommand == "" {
		config.StartCommand = buildConfig.StartCommand
	}
	if config.Framework == "" {
		config.Framework = buildConfig.Framework
	}
	if config.NodeVersion == "" {
		config.NodeVersion = buildConfig.NodeVersion
	}
}

func restoreDeploymentConfig(original *Deployment) *DeploymentConfig {
	if original == nil {
		return &DeploymentConfig{}
	}
	config := &DeploymentConfig{
		ProjectID:   original.ProjectID,
		Provider:    original.Provider,
		Environment: original.Environment,
		Branch:      original.Branch,
	}

	if original.Config == nil {
		return config
	}
	if bc, ok := original.Config["build_command"].(string); ok {
		config.BuildCommand = bc
	}
	if od, ok := original.Config["output_dir"].(string); ok {
		config.OutputDir = od
	}
	if ic, ok := original.Config["install_cmd"].(string); ok {
		config.InstallCmd = ic
	}
	if sc, ok := original.Config["start_command"].(string); ok {
		config.StartCommand = sc
	}
	if fw, ok := original.Config["framework"].(string); ok {
		config.Framework = fw
	}
	if nv, ok := original.Config["node_version"].(string); ok {
		config.NodeVersion = nv
	}
	if rd, ok := original.Config["root_directory"].(string); ok {
		config.RootDirectory = rd
	}
	if database, ok := original.Config["database"]; ok && database != nil {
		data, err := json.Marshal(database)
		if err == nil {
			var databaseConfig DatabaseConfig
			if err := json.Unmarshal(data, &databaseConfig); err == nil && databaseConfig.Provider != "" {
				config.Database = &databaseConfig
			}
		}
	}
	if custom, ok := original.Config["custom"].(map[string]any); ok {
		config.Custom = custom
	} else if custom, ok := original.Config["custom"].(map[string]interface{}); ok {
		config.Custom = custom
	}
	return config
}

// MarshalJSON custom marshaler for DeploymentConfig
func (c DeploymentConfig) MarshalJSON() ([]byte, error) {
	type Alias DeploymentConfig
	return json.Marshal(&struct {
		Alias
	}{
		Alias: (Alias)(c),
	})
}
