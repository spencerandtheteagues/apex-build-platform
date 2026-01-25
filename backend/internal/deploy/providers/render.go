// APEX.BUILD Render Deployment Provider
// Full Render API integration for deployments

package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"apex-build/internal/deploy"
)

const (
	renderAPIBase = "https://api.render.com/v1"
)

// RenderProvider implements the Provider interface for Render
type RenderProvider struct {
	token      string
	httpClient *http.Client
	ownerID    string
}

// NewRenderProvider creates a new Render provider
func NewRenderProvider(token string) *RenderProvider {
	return &RenderProvider{
		token: token,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// SetOwnerID sets the owner ID for Render services
func (p *RenderProvider) SetOwnerID(ownerID string) {
	p.ownerID = ownerID
}

// Name returns the provider name
func (p *RenderProvider) Name() deploy.DeploymentProvider {
	return deploy.ProviderRender
}

// RenderServiceType defines the type of service
type RenderServiceType string

const (
	ServiceTypeWebService    RenderServiceType = "web_service"
	ServiceTypeStaticSite    RenderServiceType = "static_site"
	ServiceTypePrivateService RenderServiceType = "private_service"
	ServiceTypeBackgroundWorker RenderServiceType = "background_worker"
	ServiceTypeCronJob       RenderServiceType = "cron_job"
)

// RenderService represents a Render service
type RenderService struct {
	ID           string            `json:"id"`
	Type         RenderServiceType `json:"type"`
	Name         string            `json:"name"`
	Slug         string            `json:"slug"`
	Suspended    string            `json:"suspended"` // not_suspended, suspended
	Suspenders   []string          `json:"suspenders,omitempty"`
	AutoDeploy   string            `json:"autoDeploy"` // yes, no
	Branch       string            `json:"branch,omitempty"`
	BuildFilter  *RenderBuildFilter `json:"buildFilter,omitempty"`
	CreatedAt    string            `json:"createdAt"`
	UpdatedAt    string            `json:"updatedAt"`
	NotifyOnFail string            `json:"notifyOnFail"`
	OwnerID      string            `json:"ownerId"`
	Repo         string            `json:"repo,omitempty"`
	ServiceDetails *RenderServiceDetails `json:"serviceDetails,omitempty"`
}

// RenderServiceDetails contains service-specific details
type RenderServiceDetails struct {
	Env            string                 `json:"env,omitempty"` // docker, elixir, go, node, python, ruby, rust, static
	EnvSpecificDetails map[string]interface{} `json:"envSpecificDetails,omitempty"`
	BuildCommand   string                 `json:"buildCommand,omitempty"`
	StartCommand   string                 `json:"startCommand,omitempty"`
	PreDeployCommand string               `json:"preDeployCommand,omitempty"`
	Plan           string                 `json:"plan,omitempty"` // free, starter, standard, pro, etc.
	Region         string                 `json:"region,omitempty"` // oregon, frankfurt, ohio, singapore
	NumInstances   int                    `json:"numInstances,omitempty"`
	HealthCheckPath string                `json:"healthCheckPath,omitempty"`
	PublishPath    string                 `json:"publishPath,omitempty"` // For static sites
	URL            string                 `json:"url,omitempty"`
}

// RenderBuildFilter for selective builds
type RenderBuildFilter struct {
	Paths         []string `json:"paths,omitempty"`
	IgnoredPaths  []string `json:"ignoredPaths,omitempty"`
}

// RenderCreateServiceRequest for creating a new service
type RenderCreateServiceRequest struct {
	Type           RenderServiceType `json:"type"`
	Name           string            `json:"name"`
	OwnerID        string            `json:"ownerId"`
	Repo           string            `json:"repo,omitempty"`
	Branch         string            `json:"branch,omitempty"`
	AutoDeploy     string            `json:"autoDeploy,omitempty"`
	EnvVars        []RenderEnvVar    `json:"envVars,omitempty"`
	SecretFiles    []RenderSecretFile `json:"secretFiles,omitempty"`
	ServiceDetails *RenderServiceDetails `json:"serviceDetails,omitempty"`
}

// RenderEnvVar represents an environment variable
type RenderEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
	GenerateValue bool `json:"generateValue,omitempty"`
}

// RenderSecretFile represents a secret file
type RenderSecretFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// RenderDeploy represents a deployment
type RenderDeploy struct {
	ID           string        `json:"id"`
	Commit       *RenderCommit `json:"commit,omitempty"`
	Status       string        `json:"status"` // created, build_in_progress, update_in_progress, live, deactivated, build_failed, update_failed, canceled, pre_deploy_in_progress, pre_deploy_failed
	Trigger      string        `json:"trigger,omitempty"` // api, deploy_hook, git_push, manual, sync
	FinishedAt   string        `json:"finishedAt,omitempty"`
	CreatedAt    string        `json:"createdAt"`
	UpdatedAt    string        `json:"updatedAt"`
}

// RenderCommit represents commit information
type RenderCommit struct {
	ID        string `json:"id"`
	Message   string `json:"message,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// RenderDeployResponse wraps the deploy response
type RenderDeployResponse struct {
	Deploy *RenderDeploy `json:"deploy"`
}

// Deploy deploys to Render
func (p *RenderProvider) Deploy(ctx context.Context, config *deploy.DeploymentConfig, files []deploy.ProjectFile) (*deploy.ProviderDeploymentResult, error) {
	// Get or create the owner ID
	if p.ownerID == "" {
		ownerID, err := p.getOwnerID(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get owner ID: %w", err)
		}
		p.ownerID = ownerID
	}

	// Determine service type based on project
	serviceType := p.detectServiceType(config)

	// Try to find existing service
	serviceName := fmt.Sprintf("apex-project-%d", config.ProjectID)
	service, err := p.findService(ctx, serviceName)

	if err != nil || service == nil {
		// Create new service
		service, err = p.createService(ctx, serviceName, serviceType, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create service: %w", err)
		}
	}

	// Trigger a new deploy
	deployResp, err := p.triggerDeploy(ctx, service.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to trigger deploy: %w", err)
	}

	// Wait for deployment to complete
	finalDeploy, err := p.waitForDeployment(ctx, service.ID, deployResp.Deploy.ID)
	if err != nil {
		return nil, err
	}

	// Get logs
	logs, _ := p.GetLogs(ctx, service.ID+"/"+finalDeploy.ID)

	// Get the service URL
	serviceURL := ""
	if service.ServiceDetails != nil {
		serviceURL = service.ServiceDetails.URL
	}
	if serviceURL == "" {
		serviceURL = fmt.Sprintf("https://%s.onrender.com", service.Slug)
	}

	return &deploy.ProviderDeploymentResult{
		ProviderID: service.ID + "/" + finalDeploy.ID,
		Status:     p.mapStatus(finalDeploy.Status),
		URL:        serviceURL,
		BuildLogs:  logs,
	}, nil
}

// getOwnerID gets the owner ID for the API token
func (p *RenderProvider) getOwnerID(ctx context.Context) (string, error) {
	url := renderAPIBase + "/owners"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("failed to get owners: %s", string(body))
	}

	var owners []struct {
		Owner struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"owner"`
	}

	if err := json.Unmarshal(body, &owners); err != nil {
		return "", err
	}

	if len(owners) == 0 {
		return "", fmt.Errorf("no owners found for this API key")
	}

	return owners[0].Owner.ID, nil
}

// findService finds an existing service by name
func (p *RenderProvider) findService(ctx context.Context, name string) (*RenderService, error) {
	url := fmt.Sprintf("%s/services?name=%s", renderAPIBase, name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to list services: %s", string(body))
	}

	var services []struct {
		Service RenderService `json:"service"`
	}

	if err := json.Unmarshal(body, &services); err != nil {
		return nil, err
	}

	for _, s := range services {
		if s.Service.Name == name {
			return &s.Service, nil
		}
	}

	return nil, nil
}

// createService creates a new service
func (p *RenderProvider) createService(ctx context.Context, name string, serviceType RenderServiceType, config *deploy.DeploymentConfig) (*RenderService, error) {
	serviceDetails := &RenderServiceDetails{
		Plan:   "free",
		Region: "oregon",
	}

	// Set environment based on framework
	switch config.Framework {
	case "nodejs", "express", "fastify", "nextjs":
		serviceDetails.Env = "node"
	case "python", "flask", "django", "fastapi":
		serviceDetails.Env = "python"
	case "go":
		serviceDetails.Env = "go"
	case "rust":
		serviceDetails.Env = "rust"
	default:
		if serviceType == ServiceTypeStaticSite {
			serviceDetails.Env = "static"
		} else {
			serviceDetails.Env = "node"
		}
	}

	if config.BuildCommand != "" {
		serviceDetails.BuildCommand = config.BuildCommand
	}
	if config.OutputDir != "" && serviceType == ServiceTypeStaticSite {
		serviceDetails.PublishPath = config.OutputDir
	}

	// Convert environment variables
	envVars := make([]RenderEnvVar, 0)
	for key, value := range config.EnvVars {
		envVars = append(envVars, RenderEnvVar{
			Key:   key,
			Value: value,
		})
	}

	// Add NODE_ENV for Node projects
	if serviceDetails.Env == "node" {
		envVars = append(envVars, RenderEnvVar{
			Key:   "NODE_ENV",
			Value: "production",
		})
	}

	createReq := RenderCreateServiceRequest{
		Type:           serviceType,
		Name:           name,
		OwnerID:        p.ownerID,
		AutoDeploy:     "yes",
		EnvVars:        envVars,
		ServiceDetails: serviceDetails,
	}

	body, err := json.Marshal(createReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", renderAPIBase+"/services", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to create service: %s", string(respBody))
	}

	var result struct {
		Service RenderService `json:"service"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return &result.Service, nil
}

// triggerDeploy triggers a new deployment
func (p *RenderProvider) triggerDeploy(ctx context.Context, serviceID string) (*RenderDeployResponse, error) {
	url := fmt.Sprintf("%s/services/%s/deploys", renderAPIBase, serviceID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to trigger deploy: %s", string(body))
	}

	var deployResp RenderDeployResponse
	if err := json.Unmarshal(body, &deployResp); err != nil {
		return nil, err
	}

	return &deployResp, nil
}

// waitForDeployment waits for deployment to complete
func (p *RenderProvider) waitForDeployment(ctx context.Context, serviceID, deployID string) (*RenderDeploy, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(15 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("deployment timed out")
		case <-ticker.C:
			deploy, err := p.getDeployStatus(ctx, serviceID, deployID)
			if err != nil {
				return nil, err
			}

			switch deploy.Status {
			case "live":
				return deploy, nil
			case "build_failed", "update_failed", "pre_deploy_failed":
				return nil, fmt.Errorf("deployment failed: %s", deploy.Status)
			case "canceled":
				return nil, fmt.Errorf("deployment was canceled")
			case "deactivated":
				return nil, fmt.Errorf("deployment was deactivated")
			}
			// Continue waiting for created, build_in_progress, update_in_progress, pre_deploy_in_progress
		}
	}
}

// getDeployStatus gets the current deployment status
func (p *RenderProvider) getDeployStatus(ctx context.Context, serviceID, deployID string) (*RenderDeploy, error) {
	url := fmt.Sprintf("%s/services/%s/deploys/%s", renderAPIBase, serviceID, deployID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to get deploy status: %s", string(body))
	}

	var result RenderDeploy
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetStatus gets the current status of a deployment
func (p *RenderProvider) GetStatus(ctx context.Context, deploymentID string) (*deploy.ProviderDeploymentResult, error) {
	parts := strings.Split(deploymentID, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid deployment ID format")
	}

	serviceID, deployID := parts[0], parts[1]

	deployStatus, err := p.getDeployStatus(ctx, serviceID, deployID)
	if err != nil {
		return nil, err
	}

	// Get service for URL
	service, err := p.getService(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	serviceURL := ""
	if service.ServiceDetails != nil {
		serviceURL = service.ServiceDetails.URL
	}
	if serviceURL == "" {
		serviceURL = fmt.Sprintf("https://%s.onrender.com", service.Slug)
	}

	return &deploy.ProviderDeploymentResult{
		ProviderID: deploymentID,
		Status:     p.mapStatus(deployStatus.Status),
		URL:        serviceURL,
	}, nil
}

// getService gets a service by ID
func (p *RenderProvider) getService(ctx context.Context, serviceID string) (*RenderService, error) {
	url := fmt.Sprintf("%s/services/%s", renderAPIBase, serviceID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to get service: %s", string(body))
	}

	var service RenderService
	if err := json.Unmarshal(body, &service); err != nil {
		return nil, err
	}

	return &service, nil
}

// Cancel cancels a deployment
func (p *RenderProvider) Cancel(ctx context.Context, deploymentID string) error {
	parts := strings.Split(deploymentID, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid deployment ID format")
	}

	serviceID, deployID := parts[0], parts[1]
	url := fmt.Sprintf("%s/services/%s/deploys/%s/cancel", renderAPIBase, serviceID, deployID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to cancel deployment: %s", string(body))
	}

	return nil
}

// GetLogs gets deployment logs
func (p *RenderProvider) GetLogs(ctx context.Context, deploymentID string) ([]string, error) {
	parts := strings.Split(deploymentID, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid deployment ID format")
	}

	serviceID := parts[0]
	url := fmt.Sprintf("%s/services/%s/logs", renderAPIBase, serviceID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return []string{"Logs not available"}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var logEntries []struct {
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
		Level     string `json:"level"`
	}

	if err := json.Unmarshal(body, &logEntries); err != nil {
		// Try to parse as plain text
		return strings.Split(string(body), "\n"), nil
	}

	logs := make([]string, 0, len(logEntries))
	for _, entry := range logEntries {
		logs = append(logs, fmt.Sprintf("[%s] %s: %s", entry.Timestamp, entry.Level, entry.Message))
	}

	return logs, nil
}

// ValidateConfig validates deployment configuration
func (p *RenderProvider) ValidateConfig(config *deploy.DeploymentConfig) error {
	if config.ProjectID == 0 {
		return fmt.Errorf("project ID is required")
	}
	return nil
}

// Helper functions

func (p *RenderProvider) mapStatus(status string) deploy.DeploymentStatus {
	switch status {
	case "created":
		return deploy.StatusPreparing
	case "build_in_progress", "pre_deploy_in_progress":
		return deploy.StatusBuilding
	case "update_in_progress":
		return deploy.StatusDeploying
	case "live":
		return deploy.StatusLive
	case "build_failed", "update_failed", "pre_deploy_failed":
		return deploy.StatusFailed
	case "canceled":
		return deploy.StatusCancelled
	default:
		return deploy.StatusPending
	}
}

func (p *RenderProvider) detectServiceType(config *deploy.DeploymentConfig) RenderServiceType {
	// Check if it's a static site
	staticFrameworks := []string{"react", "vue", "svelte", "angular", "static"}
	for _, fw := range staticFrameworks {
		if config.Framework == fw {
			return ServiceTypeStaticSite
		}
	}

	// Default to web service for everything else
	return ServiceTypeWebService
}

// DeleteService deletes a service
func (p *RenderProvider) DeleteService(ctx context.Context, serviceID string) error {
	url := fmt.Sprintf("%s/services/%s", renderAPIBase, serviceID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete service: %s", string(body))
	}

	return nil
}

// SuspendService suspends a service
func (p *RenderProvider) SuspendService(ctx context.Context, serviceID string) error {
	url := fmt.Sprintf("%s/services/%s/suspend", renderAPIBase, serviceID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to suspend service: %s", string(body))
	}

	return nil
}

// ResumeService resumes a suspended service
func (p *RenderProvider) ResumeService(ctx context.Context, serviceID string) error {
	url := fmt.Sprintf("%s/services/%s/resume", renderAPIBase, serviceID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to resume service: %s", string(body))
	}

	return nil
}

// UpdateEnvVars updates environment variables for a service
func (p *RenderProvider) UpdateEnvVars(ctx context.Context, serviceID string, envVars map[string]string) error {
	url := fmt.Sprintf("%s/services/%s/env-vars", renderAPIBase, serviceID)

	vars := make([]RenderEnvVar, 0, len(envVars))
	for key, value := range envVars {
		vars = append(vars, RenderEnvVar{Key: key, Value: value})
	}

	body, err := json.Marshal(vars)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update env vars: %s", string(respBody))
	}

	return nil
}

// ScaleService scales the number of instances
func (p *RenderProvider) ScaleService(ctx context.Context, serviceID string, instances int) error {
	url := fmt.Sprintf("%s/services/%s/scale", renderAPIBase, serviceID)

	body, err := json.Marshal(map[string]int{"numInstances": instances})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to scale service: %s", string(respBody))
	}

	return nil
}
