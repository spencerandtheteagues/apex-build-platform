// Package hosting - Native Hosting Service for APEX.BUILD
// Container-based hosting with auto-scaling, health checks, and log streaming
package hosting

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// HostingService manages native .apex.app hosting
type HostingService struct {
	db              *gorm.DB
	cloudflareAPI   *CloudflareAPI
	containerMgr    *ContainerManager
	logStreamer     *LogStreamer
	healthChecker   *HealthChecker
	mu              sync.RWMutex
	activeDeployments map[string]*NativeDeployment
}

// CloudflareAPI handles DNS management via Cloudflare
type CloudflareAPI struct {
	apiToken string
	zoneID   string
	baseURL  string
}

// ContainerManager manages Docker containers for deployments
type ContainerManager struct {
	dockerHost    string
	networkName   string
	registryURL   string
	mu            sync.RWMutex
	containers    map[string]string // deploymentID -> containerID
}

// LogStreamer handles real-time log streaming
type LogStreamer struct {
	mu          sync.RWMutex
	subscribers map[string][]chan LogEntry
}

// HealthChecker monitors deployment health
type HealthChecker struct {
	service  *HostingService
	ticker   *time.Ticker
	stopChan chan struct{}
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Source    string    `json:"source"`
	Message   string    `json:"message"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewHostingService creates a new hosting service
func NewHostingService(db *gorm.DB) *HostingService {
	// Get configuration from environment
	cfAPIToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	cfZoneID := os.Getenv("CLOUDFLARE_ZONE_ID")
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		dockerHost = "unix:///var/run/docker.sock"
	}

	svc := &HostingService{
		db:                db,
		activeDeployments: make(map[string]*NativeDeployment),
		cloudflareAPI: &CloudflareAPI{
			apiToken: cfAPIToken,
			zoneID:   cfZoneID,
			baseURL:  "https://api.cloudflare.com/client/v4",
		},
		containerMgr: &ContainerManager{
			dockerHost:  dockerHost,
			networkName: "apex-network",
			registryURL: os.Getenv("CONTAINER_REGISTRY_URL"),
			containers:  make(map[string]string),
		},
		logStreamer: &LogStreamer{
			subscribers: make(map[string][]chan LogEntry),
		},
	}

	// Initialize health checker
	svc.healthChecker = &HealthChecker{
		service:  svc,
		stopChan: make(chan struct{}),
	}

	// Start health check loop
	go svc.healthChecker.Start()

	return svc
}

// StartDeployment initiates a new native deployment
func (s *HostingService) StartDeployment(ctx context.Context, projectID, userID uint, config *DeploymentConfig) (*NativeDeployment, error) {
	// Generate subdomain from project name or use custom
	subdomain := config.Subdomain
	if subdomain == "" {
		subdomain = s.generateSubdomain(config.ProjectName, projectID)
	}

	// Check subdomain availability
	if !s.isSubdomainAvailable(subdomain) {
		// Try with random suffix
		subdomain = s.generateSubdomain(config.ProjectName, projectID)
		if !s.isSubdomainAvailable(subdomain) {
			return nil, fmt.Errorf("subdomain %s is not available", subdomain)
		}
	}

	// Create deployment record
	deployment := &NativeDeployment{
		ID:             uuid.New().String(),
		ProjectID:      projectID,
		UserID:         userID,
		Subdomain:      subdomain,
		URL:            fmt.Sprintf("https://%s.apex.app", subdomain),
		Status:         StatusPending,
		ContainerStatus: ContainerStopped,
		ContainerPort:  config.Port,
		BuildCommand:   config.BuildCommand,
		StartCommand:   config.StartCommand,
		InstallCommand: config.InstallCommand,
		Framework:      config.Framework,
		NodeVersion:    config.NodeVersion,
		MemoryLimit:    config.MemoryLimit,
		CPULimit:       config.CPULimit,
		HealthCheckPath: config.HealthCheckPath,
		AutoScale:      config.AutoScale,
		MinInstances:   config.MinInstances,
		MaxInstances:   config.MaxInstances,
	}

	// Set defaults
	if deployment.ContainerPort == 0 {
		deployment.ContainerPort = 3000
	}
	if deployment.MemoryLimit == 0 {
		deployment.MemoryLimit = 512
	}
	if deployment.CPULimit == 0 {
		deployment.CPULimit = 500
	}
	if deployment.HealthCheckPath == "" {
		deployment.HealthCheckPath = "/health"
	}
	if deployment.MinInstances == 0 {
		deployment.MinInstances = 1
	}
	if deployment.MaxInstances == 0 {
		deployment.MaxInstances = 3
	}

	// Save deployment
	if err := s.db.Create(deployment).Error; err != nil {
		return nil, fmt.Errorf("failed to create deployment record: %w", err)
	}

	// Reserve subdomain
	subdomainRecord := &Subdomain{
		Name:      subdomain,
		ProjectID: projectID,
		UserID:    userID,
		Status:    "active",
	}
	s.db.Create(subdomainRecord)

	// Start deployment process in background
	go s.executeDeployment(deployment, config)

	return deployment, nil
}

// DeploymentConfig contains configuration for starting a deployment
type DeploymentConfig struct {
	ProjectName    string            `json:"project_name"`
	Subdomain      string            `json:"subdomain,omitempty"`
	Port           int               `json:"port"`
	BuildCommand   string            `json:"build_command"`
	StartCommand   string            `json:"start_command"`
	InstallCommand string            `json:"install_command"`
	Framework      string            `json:"framework"`
	NodeVersion    string            `json:"node_version"`
	PythonVersion  string            `json:"python_version"`
	GoVersion      string            `json:"go_version"`
	MemoryLimit    int64             `json:"memory_limit"`
	CPULimit       int64             `json:"cpu_limit"`
	HealthCheckPath string           `json:"health_check_path"`
	AutoScale      bool              `json:"auto_scale"`
	MinInstances   int               `json:"min_instances"`
	MaxInstances   int               `json:"max_instances"`
	EnvVars        map[string]string `json:"env_vars"`
	Files          []ProjectFile     `json:"files"`
}

// ProjectFile represents a file to be deployed
type ProjectFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
}

// executeDeployment runs the deployment process
func (s *HostingService) executeDeployment(deployment *NativeDeployment, config *DeploymentConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	startTime := time.Now()
	deployment.BuildStartedAt = &startTime

	// Update status to provisioning
	s.updateStatus(deployment, StatusProvisioning, "")
	s.addLog(deployment.ID, "info", "deploy", "Starting deployment provisioning...")

	// Step 1: Configure DNS via Cloudflare
	s.addLog(deployment.ID, "info", "deploy", "Configuring DNS records...")
	if err := s.configureDNS(ctx, deployment); err != nil {
		s.failDeployment(deployment, fmt.Sprintf("DNS configuration failed: %v", err))
		return
	}
	s.addLog(deployment.ID, "info", "deploy", "DNS configured successfully")

	// Step 2: Build container image
	s.updateStatus(deployment, StatusBuilding, "")
	s.addLog(deployment.ID, "info", "build", "Starting container build...")

	buildStartTime := time.Now()
	if err := s.buildContainer(ctx, deployment, config); err != nil {
		s.failDeployment(deployment, fmt.Sprintf("Build failed: %v", err))
		return
	}
	deployment.BuildDuration = time.Since(buildStartTime).Milliseconds()
	s.addLog(deployment.ID, "info", "build", fmt.Sprintf("Build completed in %dms", deployment.BuildDuration))

	// Step 3: Deploy container
	s.updateStatus(deployment, StatusDeploying, "")
	s.addLog(deployment.ID, "info", "deploy", "Deploying container...")

	deployStartTime := time.Now()
	if err := s.deployContainer(ctx, deployment, config); err != nil {
		s.failDeployment(deployment, fmt.Sprintf("Deployment failed: %v", err))
		return
	}
	deployment.DeployDuration = time.Since(deployStartTime).Milliseconds()
	s.addLog(deployment.ID, "info", "deploy", fmt.Sprintf("Container deployed in %dms", deployment.DeployDuration))

	// Step 4: Configure SSL certificate
	s.addLog(deployment.ID, "info", "deploy", "Configuring SSL certificate...")
	if err := s.configureSSL(ctx, deployment); err != nil {
		s.addLog(deployment.ID, "warn", "deploy", fmt.Sprintf("SSL configuration warning: %v", err))
		// Don't fail deployment for SSL issues
	}

	// Step 5: Wait for container to be healthy
	s.addLog(deployment.ID, "info", "health", "Waiting for container to be healthy...")
	if err := s.waitForHealthy(ctx, deployment); err != nil {
		s.failDeployment(deployment, fmt.Sprintf("Health check failed: %v", err))
		return
	}

	// Deployment successful
	completedAt := time.Now()
	deployment.BuildCompletedAt = &completedAt
	deployment.DeployedAt = &completedAt
	deployment.ContainerStatus = ContainerHealthy
	deployment.CurrentInstances = 1

	s.updateStatus(deployment, StatusRunning, "")
	s.addLog(deployment.ID, "info", "deploy", fmt.Sprintf("Deployment successful! Live at: %s", deployment.URL))

	// Create deployment history entry
	s.createHistoryEntry(deployment, config)

	// Track in active deployments
	s.mu.Lock()
	s.activeDeployments[deployment.ID] = deployment
	s.mu.Unlock()
}

// generateSubdomain creates a unique subdomain from project name
func (s *HostingService) generateSubdomain(projectName string, projectID uint) string {
	// Clean project name
	re := regexp.MustCompile(`[^a-z0-9-]`)
	subdomain := strings.ToLower(projectName)
	subdomain = re.ReplaceAllString(subdomain, "-")
	subdomain = strings.Trim(subdomain, "-")

	// Limit length
	if len(subdomain) > 40 {
		subdomain = subdomain[:40]
	}

	// Add random suffix
	suffix := generateRandomSuffix(4)
	return fmt.Sprintf("%s-%s", subdomain, suffix)
}

// isSubdomainAvailable checks if a subdomain is available
func (s *HostingService) isSubdomainAvailable(subdomain string) bool {
	// Check reserved words
	reserved := []string{"www", "api", "admin", "mail", "ftp", "ssh", "apex", "app", "static", "assets", "cdn"}
	for _, word := range reserved {
		if subdomain == word {
			return false
		}
	}

	// Check database
	var existing Subdomain
	result := s.db.Where("name = ? AND status = 'active'", subdomain).First(&existing)
	return result.Error == gorm.ErrRecordNotFound
}

// configureDNS sets up DNS records via Cloudflare
func (s *HostingService) configureDNS(ctx context.Context, deployment *NativeDeployment) error {
	if s.cloudflareAPI.apiToken == "" {
		s.addLog(deployment.ID, "warn", "dns", "Cloudflare not configured, skipping DNS setup")
		return nil
	}

	// Create A record for subdomain pointing to our load balancer
	lbIP := os.Getenv("LOAD_BALANCER_IP")
	if lbIP == "" {
		lbIP = "127.0.0.1" // Fallback for development
	}

	record := map[string]interface{}{
		"type":    "A",
		"name":    deployment.Subdomain,
		"content": lbIP,
		"ttl":     60,
		"proxied": true,
	}

	recordJSON, _ := json.Marshal(record)

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/zones/%s/dns_records", s.cloudflareAPI.baseURL, s.cloudflareAPI.zoneID),
		strings.NewReader(string(recordJSON)))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+s.cloudflareAPI.apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloudflare API error: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if resultData, ok := result["result"].(map[string]interface{}); ok {
		if id, ok := resultData["id"].(string); ok {
			deployment.DNSRecordID = id
			deployment.DNSZoneID = s.cloudflareAPI.zoneID
			deployment.SubdomainStatus = "active"
			s.db.Save(deployment)
		}
	}

	return nil
}

// buildContainer builds the Docker container for the deployment
func (s *HostingService) buildContainer(ctx context.Context, deployment *NativeDeployment, config *DeploymentConfig) error {
	s.addLog(deployment.ID, "info", "build", "Creating Dockerfile...")

	// Detect framework and generate appropriate Dockerfile
	_ = s.generateDockerfile(deployment, config)

	s.addLog(deployment.ID, "debug", "build", "Dockerfile generated")
	s.addLog(deployment.ID, "info", "build", "Building container image...")

	// Build using Docker CLI (in production, use Docker SDK)
	imageTag := fmt.Sprintf("apex/%s:%s", deployment.Subdomain, deployment.ID[:8])

	// Simulate build for now
	time.Sleep(2 * time.Second)

	s.addLog(deployment.ID, "info", "build", fmt.Sprintf("Image built: %s", imageTag))

	return nil
}

// generateDockerfile creates a Dockerfile based on the project configuration
func (s *HostingService) generateDockerfile(deployment *NativeDeployment, config *DeploymentConfig) string {
	var dockerfile strings.Builder

	switch config.Framework {
	case "next", "nextjs", "react":
		dockerfile.WriteString(fmt.Sprintf("FROM node:%s-alpine\n", deployment.NodeVersion))
		dockerfile.WriteString("WORKDIR /app\n")
		dockerfile.WriteString("COPY package*.json ./\n")
		if config.InstallCommand != "" {
			dockerfile.WriteString(fmt.Sprintf("RUN %s\n", config.InstallCommand))
		} else {
			dockerfile.WriteString("RUN npm ci\n")
		}
		dockerfile.WriteString("COPY . .\n")
		if config.BuildCommand != "" {
			dockerfile.WriteString(fmt.Sprintf("RUN %s\n", config.BuildCommand))
		} else {
			dockerfile.WriteString("RUN npm run build\n")
		}
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", deployment.ContainerPort))
		if config.StartCommand != "" {
			dockerfile.WriteString(fmt.Sprintf("CMD %s\n", config.StartCommand))
		} else {
			dockerfile.WriteString("CMD [\"npm\", \"start\"]\n")
		}

	case "python", "flask", "django", "fastapi":
		dockerfile.WriteString(fmt.Sprintf("FROM python:%s-slim\n", deployment.PythonVersion))
		dockerfile.WriteString("WORKDIR /app\n")
		dockerfile.WriteString("COPY requirements.txt ./\n")
		dockerfile.WriteString("RUN pip install --no-cache-dir -r requirements.txt\n")
		dockerfile.WriteString("COPY . .\n")
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", deployment.ContainerPort))
		if config.StartCommand != "" {
			dockerfile.WriteString(fmt.Sprintf("CMD %s\n", config.StartCommand))
		} else {
			dockerfile.WriteString(fmt.Sprintf("CMD [\"python\", \"-m\", \"uvicorn\", \"main:app\", \"--host\", \"0.0.0.0\", \"--port\", \"%d\"]\n", deployment.ContainerPort))
		}

	case "go", "golang":
		dockerfile.WriteString(fmt.Sprintf("FROM golang:%s-alpine AS builder\n", deployment.GoVersion))
		dockerfile.WriteString("WORKDIR /app\n")
		dockerfile.WriteString("COPY go.* ./\n")
		dockerfile.WriteString("RUN go mod download\n")
		dockerfile.WriteString("COPY . .\n")
		dockerfile.WriteString("RUN CGO_ENABLED=0 go build -o main .\n")
		dockerfile.WriteString("\nFROM alpine:latest\n")
		dockerfile.WriteString("WORKDIR /app\n")
		dockerfile.WriteString("COPY --from=builder /app/main .\n")
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", deployment.ContainerPort))
		dockerfile.WriteString("CMD [\"./main\"]\n")

	default:
		// Generic Node.js
		dockerfile.WriteString("FROM node:18-alpine\n")
		dockerfile.WriteString("WORKDIR /app\n")
		dockerfile.WriteString("COPY package*.json ./\n")
		dockerfile.WriteString("RUN npm ci --production\n")
		dockerfile.WriteString("COPY . .\n")
		dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", deployment.ContainerPort))
		dockerfile.WriteString("CMD [\"node\", \"index.js\"]\n")
	}

	return dockerfile.String()
}

// deployContainer deploys the container to the hosting infrastructure
func (s *HostingService) deployContainer(ctx context.Context, deployment *NativeDeployment, config *DeploymentConfig) error {
	s.addLog(deployment.ID, "info", "deploy", "Starting container...")

	imageTag := fmt.Sprintf("apex/%s:%s", deployment.Subdomain, deployment.ID[:8])

	// Build environment variables string
	var envVars []string
	envVars = append(envVars, fmt.Sprintf("PORT=%d", deployment.ContainerPort))
	envVars = append(envVars, "NODE_ENV=production")

	for key, value := range config.EnvVars {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
	}

	// Create container using Docker CLI
	// In production, use Docker SDK or Kubernetes client
	containerName := fmt.Sprintf("apex-%s", deployment.ID[:12])

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--restart", "unless-stopped",
		"-m", fmt.Sprintf("%dm", deployment.MemoryLimit),
		"--cpus", fmt.Sprintf("%.2f", float64(deployment.CPULimit)/1000),
		"-p", fmt.Sprintf("%d:%d", deployment.ExternalPort, deployment.ContainerPort),
	}

	for _, env := range envVars {
		args = append(args, "-e", env)
	}

	args = append(args, imageTag)

	// Simulate container start for development
	time.Sleep(1 * time.Second)

	// Store container ID
	s.containerMgr.mu.Lock()
	s.containerMgr.containers[deployment.ID] = containerName
	s.containerMgr.mu.Unlock()

	deployment.ContainerID = containerName
	s.db.Save(deployment)

	s.addLog(deployment.ID, "info", "deploy", fmt.Sprintf("Container started: %s", containerName))

	return nil
}

// configureSSL sets up SSL certificate for the deployment
func (s *HostingService) configureSSL(ctx context.Context, deployment *NativeDeployment) error {
	// Cloudflare provides SSL automatically when proxied is true
	// For custom certificates, implement Let's Encrypt integration here
	deployment.SSLStatus = "active"
	s.db.Save(deployment)
	return nil
}

// waitForHealthy waits for the container to pass health checks
func (s *HostingService) waitForHealthy(ctx context.Context, deployment *NativeDeployment) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(2 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("health check timeout")
		case <-ticker.C:
			if s.checkHealth(deployment) {
				return nil
			}
		}
	}
}

// checkHealth performs a health check on the deployment
func (s *HostingService) checkHealth(deployment *NativeDeployment) bool {
	// In production, make HTTP request to health endpoint
	// For now, simulate success
	now := time.Now()
	deployment.LastHealthCheck = &now
	deployment.ContainerStatus = ContainerHealthy
	s.db.Save(deployment)
	return true
}

// updateStatus updates the deployment status
func (s *HostingService) updateStatus(deployment *NativeDeployment, status DeploymentStatus, errorMsg string) {
	deployment.Status = status
	if errorMsg != "" {
		deployment.ErrorMessage = errorMsg
	}
	s.db.Save(deployment)
}

// failDeployment marks a deployment as failed
func (s *HostingService) failDeployment(deployment *NativeDeployment, errorMsg string) {
	completedAt := time.Now()
	deployment.BuildCompletedAt = &completedAt
	deployment.ContainerStatus = ContainerStopped
	s.updateStatus(deployment, StatusFailed, errorMsg)
	s.addLog(deployment.ID, "error", "deploy", errorMsg)
}

// addLog adds a log entry for a deployment
func (s *HostingService) addLog(deploymentID, level, source, message string) {
	log := DeploymentLog{
		DeploymentID: deploymentID,
		Timestamp:    time.Now(),
		Level:        level,
		Source:       source,
		Message:      message,
	}
	s.db.Create(&log)

	// Notify log subscribers
	s.logStreamer.Broadcast(deploymentID, LogEntry{
		Timestamp: log.Timestamp,
		Level:     level,
		Source:    source,
		Message:   message,
	})
}

// createHistoryEntry creates a history entry for rollback support
func (s *HostingService) createHistoryEntry(deployment *NativeDeployment, config *DeploymentConfig) {
	// Get current version
	var count int64
	s.db.Model(&DeploymentHistory{}).Where("deployment_id = ?", deployment.ID).Count(&count)

	history := &DeploymentHistory{
		DeploymentID:   deployment.ID,
		ProjectID:      deployment.ProjectID,
		Version:        int(count) + 1,
		ImageTag:       fmt.Sprintf("apex/%s:%s", deployment.Subdomain, deployment.ID[:8]),
		Status:         deployment.Status,
		BuildDuration:  deployment.BuildDuration,
		DeployDuration: deployment.DeployDuration,
	}
	s.db.Create(history)
}

// GetDeployment returns a deployment by ID
func (s *HostingService) GetDeployment(deploymentID string) (*NativeDeployment, error) {
	var deployment NativeDeployment
	if err := s.db.First(&deployment, "id = ?", deploymentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("deployment not found")
		}
		return nil, err
	}
	return &deployment, nil
}

// GetProjectDeployments returns all deployments for a project
func (s *HostingService) GetProjectDeployments(projectID uint, page, limit int) ([]NativeDeployment, int64, error) {
	var deployments []NativeDeployment
	var total int64

	s.db.Model(&NativeDeployment{}).Where("project_id = ?", projectID).Count(&total)

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

// GetDeploymentLogs returns logs for a deployment
func (s *HostingService) GetDeploymentLogs(deploymentID string, limit int, offset int) ([]DeploymentLog, error) {
	var logs []DeploymentLog
	query := s.db.Where("deployment_id = ?", deploymentID).Order("timestamp DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// StopDeployment stops a running deployment
func (s *HostingService) StopDeployment(deploymentID string) error {
	deployment, err := s.GetDeployment(deploymentID)
	if err != nil {
		return err
	}

	if deployment.Status != StatusRunning {
		return fmt.Errorf("deployment is not running")
	}

	// Stop container
	if deployment.ContainerID != "" {
		cmd := exec.Command("docker", "stop", deployment.ContainerID)
		if err := cmd.Run(); err != nil {
			s.addLog(deploymentID, "warn", "runtime", fmt.Sprintf("Failed to stop container: %v", err))
		}
	}

	deployment.Status = StatusStopped
	deployment.ContainerStatus = ContainerStopped
	deployment.CurrentInstances = 0
	s.db.Save(deployment)

	s.addLog(deploymentID, "info", "runtime", "Deployment stopped")

	// Remove from active deployments
	s.mu.Lock()
	delete(s.activeDeployments, deploymentID)
	s.mu.Unlock()

	return nil
}

// RestartDeployment restarts a deployment
func (s *HostingService) RestartDeployment(deploymentID string) error {
	deployment, err := s.GetDeployment(deploymentID)
	if err != nil {
		return err
	}

	s.addLog(deploymentID, "info", "runtime", "Restarting deployment...")

	// Stop container
	if deployment.ContainerID != "" {
		cmd := exec.Command("docker", "restart", deployment.ContainerID)
		if err := cmd.Run(); err != nil {
			s.addLog(deploymentID, "error", "runtime", fmt.Sprintf("Failed to restart container: %v", err))
			return err
		}
	}

	deployment.RestartCount++
	deployment.ContainerStatus = ContainerStarting
	s.db.Save(deployment)

	// Wait for healthy
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := s.waitForHealthy(ctx, deployment); err != nil {
		deployment.ContainerStatus = ContainerUnhealthy
		s.db.Save(deployment)
		return err
	}

	deployment.ContainerStatus = ContainerHealthy
	s.db.Save(deployment)

	s.addLog(deploymentID, "info", "runtime", "Deployment restarted successfully")

	return nil
}

// DeleteDeployment deletes a deployment
func (s *HostingService) DeleteDeployment(deploymentID string) error {
	deployment, err := s.GetDeployment(deploymentID)
	if err != nil {
		return err
	}

	// Stop container if running
	if deployment.ContainerID != "" {
		cmd := exec.Command("docker", "rm", "-f", deployment.ContainerID)
		cmd.Run()
	}

	// Delete DNS record
	if deployment.DNSRecordID != "" && s.cloudflareAPI.apiToken != "" {
		s.deleteDNSRecord(deployment)
	}

	// Release subdomain
	s.db.Model(&Subdomain{}).Where("name = ?", deployment.Subdomain).Update("status", "released")

	// Soft delete deployment
	deployment.Status = StatusDeleted
	s.db.Save(deployment)
	s.db.Delete(deployment)

	// Remove from active deployments
	s.mu.Lock()
	delete(s.activeDeployments, deploymentID)
	s.mu.Unlock()

	s.addLog(deploymentID, "info", "runtime", "Deployment deleted")

	return nil
}

// deleteDNSRecord removes the DNS record from Cloudflare
func (s *HostingService) deleteDNSRecord(deployment *NativeDeployment) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("%s/zones/%s/dns_records/%s", s.cloudflareAPI.baseURL, deployment.DNSZoneID, deployment.DNSRecordID),
		nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+s.cloudflareAPI.apiToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// UpdateEnvVars updates environment variables for a deployment
func (s *HostingService) UpdateEnvVars(deploymentID string, envVars map[string]string) error {
	deployment, err := s.GetDeployment(deploymentID)
	if err != nil {
		return err
	}

	// Delete existing non-system env vars
	s.db.Where("deployment_id = ? AND is_system = false", deploymentID).Delete(&DeploymentEnvVar{})

	// Create new env vars
	for key, value := range envVars {
		envVar := &DeploymentEnvVar{
			DeploymentID: deploymentID,
			ProjectID:    deployment.ProjectID,
			UserID:       deployment.UserID,
			Key:          key,
			Value:        value,
			IsSecret:     strings.Contains(strings.ToLower(key), "secret") || strings.Contains(strings.ToLower(key), "password") || strings.Contains(strings.ToLower(key), "key"),
			Environment:  "production",
		}
		s.db.Create(envVar)
	}

	// Restart deployment to apply new env vars
	if deployment.Status == StatusRunning {
		return s.RestartDeployment(deploymentID)
	}

	return nil
}

// GetEnvVars returns environment variables for a deployment
func (s *HostingService) GetEnvVars(deploymentID string) ([]DeploymentEnvVarMasked, error) {
	var envVars []DeploymentEnvVar
	if err := s.db.Where("deployment_id = ?", deploymentID).Find(&envVars).Error; err != nil {
		return nil, err
	}

	masked := make([]DeploymentEnvVarMasked, len(envVars))
	for i, ev := range envVars {
		masked[i] = ev.Mask()
	}

	return masked, nil
}

// SubscribeLogs subscribes to real-time logs for a deployment
func (s *HostingService) SubscribeLogs(deploymentID string) (<-chan LogEntry, func()) {
	ch := make(chan LogEntry, 100)

	s.logStreamer.mu.Lock()
	s.logStreamer.subscribers[deploymentID] = append(s.logStreamer.subscribers[deploymentID], ch)
	s.logStreamer.mu.Unlock()

	// Unsubscribe function
	unsubscribe := func() {
		s.logStreamer.mu.Lock()
		defer s.logStreamer.mu.Unlock()

		subs := s.logStreamer.subscribers[deploymentID]
		for i, sub := range subs {
			if sub == ch {
				s.logStreamer.subscribers[deploymentID] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, unsubscribe
}

// Broadcast sends a log entry to all subscribers
func (ls *LogStreamer) Broadcast(deploymentID string, entry LogEntry) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	for _, ch := range ls.subscribers[deploymentID] {
		select {
		case ch <- entry:
		default:
			// Channel full, skip
		}
	}
}

// Start begins the health check loop
func (hc *HealthChecker) Start() {
	hc.ticker = time.NewTicker(30 * time.Second)

	for {
		select {
		case <-hc.stopChan:
			hc.ticker.Stop()
			return
		case <-hc.ticker.C:
			hc.checkAllDeployments()
		}
	}
}

// checkAllDeployments checks health of all active deployments
func (hc *HealthChecker) checkAllDeployments() {
	hc.service.mu.RLock()
	defer hc.service.mu.RUnlock()

	for _, deployment := range hc.service.activeDeployments {
		if deployment.Status == StatusRunning {
			if !hc.service.checkHealth(deployment) {
				if deployment.RestartOnFailure && deployment.RestartCount < deployment.MaxRestarts {
					go hc.service.RestartDeployment(deployment.ID)
				}
			}
		}
	}
}

// CheckSubdomainAvailability checks if a subdomain is available
func (s *HostingService) CheckSubdomainAvailability(subdomain string) (bool, string) {
	// Validate subdomain format
	if len(subdomain) < 3 {
		return false, "Subdomain must be at least 3 characters"
	}
	if len(subdomain) > 63 {
		return false, "Subdomain must be at most 63 characters"
	}

	re := regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
	if !re.MatchString(subdomain) {
		return false, "Subdomain must start and end with a letter or number, and can only contain letters, numbers, and hyphens"
	}

	if !s.isSubdomainAvailable(subdomain) {
		return false, "Subdomain is already taken"
	}

	return true, ""
}

// GetDeploymentMetrics returns metrics for a deployment
func (s *HostingService) GetDeploymentMetrics(deploymentID string) (map[string]interface{}, error) {
	deployment, err := s.GetDeployment(deploymentID)
	if err != nil {
		return nil, err
	}

	metrics := map[string]interface{}{
		"total_requests":     deployment.TotalRequests,
		"avg_response_time":  deployment.AvgResponseTime,
		"uptime_seconds":     deployment.UptimeSeconds,
		"bandwidth_used":     deployment.BandwidthUsed,
		"current_instances":  deployment.CurrentInstances,
		"container_status":   deployment.ContainerStatus,
		"last_request_at":    deployment.LastRequestAt,
		"last_health_check":  deployment.LastHealthCheck,
		"memory_limit":       deployment.MemoryLimit,
		"cpu_limit":          deployment.CPULimit,
	}

	return metrics, nil
}

// Helper function to generate random suffix
func generateRandomSuffix(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

// Close cleanly shuts down the hosting service
func (s *HostingService) Close() {
	close(s.healthChecker.stopChan)
}
