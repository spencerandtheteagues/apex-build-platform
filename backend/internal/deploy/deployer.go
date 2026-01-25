// APEX.BUILD One-Click Deployment Service
// Core deployment orchestration and management

package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeploymentStatus represents the current state of a deployment
type DeploymentStatus string

const (
	StatusPending    DeploymentStatus = "pending"
	StatusPreparing  DeploymentStatus = "preparing"
	StatusBuilding   DeploymentStatus = "building"
	StatusDeploying  DeploymentStatus = "deploying"
	StatusLive       DeploymentStatus = "live"
	StatusFailed     DeploymentStatus = "failed"
	StatusCancelled  DeploymentStatus = "cancelled"
)

// DeploymentProvider represents supported deployment providers
type DeploymentProvider string

const (
	ProviderVercel  DeploymentProvider = "vercel"
	ProviderNetlify DeploymentProvider = "netlify"
	ProviderRender  DeploymentProvider = "render"
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
	BuildTime    int64                  `json:"build_time,omitempty"`     // milliseconds
	DeployTime   int64                  `json:"deploy_time,omitempty"`    // milliseconds
	TotalTime    int64                  `json:"total_time,omitempty"`     // milliseconds
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
	Framework     string                 `json:"framework,omitempty"`
	NodeVersion   string                 `json:"node_version,omitempty"`
	RootDirectory string                 `json:"root_directory,omitempty"`
	Custom        map[string]interface{} `json:"custom,omitempty"`
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
	tokens    map[DeploymentProvider]string  // API tokens for providers
	mu        sync.RWMutex
	active    map[string]context.CancelFunc // active deployment cancellation functions
}

// NewDeploymentService creates a new deployment service
func NewDeploymentService(db *gorm.DB, vercelToken, netlifyToken, renderToken string) *DeploymentService {
	svc := &DeploymentService{
		db:        db,
		builder:   NewBuildService(),
		providers: make(map[DeploymentProvider]Provider),
		active:    make(map[string]context.CancelFunc),
	}

	// Store tokens for lazy provider initialization
	svc.tokens = map[DeploymentProvider]string{
		ProviderVercel:  vercelToken,
		ProviderNetlify: netlifyToken,
		ProviderRender:  renderToken,
	}

	return svc
}

// RegisterProvider registers a deployment provider
func (s *DeploymentService) RegisterProvider(providerType DeploymentProvider, provider Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[providerType] = provider
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
			"build_command": config.BuildCommand,
			"output_dir":    config.OutputDir,
			"install_cmd":   config.InstallCmd,
			"framework":     config.Framework,
			"node_version":  config.NodeVersion,
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

	// Generate deployment configuration if not provided
	if config.BuildCommand == "" || config.OutputDir == "" {
		buildConfig := s.builder.GenerateBuildConfig(projectType, projectFiles)
		if config.BuildCommand == "" {
			config.BuildCommand = buildConfig.BuildCommand
		}
		if config.OutputDir == "" {
			config.OutputDir = buildConfig.OutputDir
		}
		if config.InstallCmd == "" {
			config.InstallCmd = buildConfig.InstallCommand
		}
		if config.Framework == "" {
			config.Framework = buildConfig.Framework
		}
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

	deployDuration := time.Since(deployStartTime).Milliseconds()
	deployment.DeployTime = deployDuration
	totalDuration := time.Since(startTime).Milliseconds()
	deployment.TotalTime = totalDuration

	// Update deployment with results
	completedAt := time.Now()
	deployment.CompletedAt = &completedAt
	deployment.URL = result.URL
	deployment.PreviewURL = result.PreviewURL
	deployment.Metadata["provider_id"] = result.ProviderID

	s.updateStatus(deployment, StatusLive, "")
	s.addLog(deployment.ID, "info", fmt.Sprintf("Deployment live at: %s", result.URL), "deploy")
	s.addLog(deployment.ID, "info", fmt.Sprintf("Total deployment time: %dms", totalDuration), "deploy")
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

	// Create new deployment config from original
	config := &DeploymentConfig{
		ProjectID:   original.ProjectID,
		Provider:    original.Provider,
		Environment: original.Environment,
		Branch:      original.Branch,
	}

	if original.Config != nil {
		if bc, ok := original.Config["build_command"].(string); ok {
			config.BuildCommand = bc
		}
		if od, ok := original.Config["output_dir"].(string); ok {
			config.OutputDir = od
		}
		if ic, ok := original.Config["install_cmd"].(string); ok {
			config.InstallCmd = ic
		}
		if fw, ok := original.Config["framework"].(string); ok {
			config.Framework = fw
		}
	}

	return s.StartDeployment(ctx, userID, config)
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

func getProviderDisplayName(provider DeploymentProvider) string {
	switch provider {
	case ProviderVercel:
		return "Vercel"
	case ProviderNetlify:
		return "Netlify"
	case ProviderRender:
		return "Render"
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
	default:
		return []string{}
	}
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
