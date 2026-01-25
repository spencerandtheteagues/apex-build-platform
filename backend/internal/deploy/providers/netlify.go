// APEX.BUILD Netlify Deployment Provider
// Full Netlify API integration for deployments

package providers

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"apex-build/internal/deploy"
)

const (
	netlifyAPIBase = "https://api.netlify.com/api/v1"
)

// NetlifyProvider implements the Provider interface for Netlify
type NetlifyProvider struct {
	token      string
	httpClient *http.Client
}

// NewNetlifyProvider creates a new Netlify provider
func NewNetlifyProvider(token string) *NetlifyProvider {
	return &NetlifyProvider{
		token: token,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Name returns the provider name
func (p *NetlifyProvider) Name() deploy.DeploymentProvider {
	return deploy.ProviderNetlify
}

// NetlifySite represents a Netlify site
type NetlifySite struct {
	ID                string            `json:"id"`
	SiteID            string            `json:"site_id"`
	Name              string            `json:"name"`
	URL               string            `json:"url"`
	SslURL            string            `json:"ssl_url"`
	AdminURL          string            `json:"admin_url"`
	CustomDomain      string            `json:"custom_domain,omitempty"`
	DomainAliases     []string          `json:"domain_aliases,omitempty"`
	State             string            `json:"state"`
	CreatedAt         string            `json:"created_at"`
	UpdatedAt         string            `json:"updated_at"`
	DeployURL         string            `json:"deploy_url,omitempty"`
	DeployID          string            `json:"deploy_id,omitempty"`
	BuildSettings     *NetlifyBuildSettings `json:"build_settings,omitempty"`
}

// NetlifyBuildSettings contains build configuration
type NetlifyBuildSettings struct {
	Cmd              string            `json:"cmd,omitempty"`
	Dir              string            `json:"dir,omitempty"`
	EnvVars          map[string]string `json:"env,omitempty"`
	InstallCmd       string            `json:"install_cmd,omitempty"`
	FunctionsDir     string            `json:"functions_dir,omitempty"`
	BaseRelDir       bool              `json:"base_rel_dir,omitempty"`
}

// NetlifyDeployRequest represents a deployment request
type NetlifyDeployRequest struct {
	Files       map[string]string `json:"files"`
	Async       bool              `json:"async,omitempty"`
	Draft       bool              `json:"draft,omitempty"`
	Title       string            `json:"title,omitempty"`
	Branch      string            `json:"branch,omitempty"`
	FunctionSchedules []NetlifyFunctionSchedule `json:"function_schedules,omitempty"`
}

// NetlifyFunctionSchedule defines a scheduled function
type NetlifyFunctionSchedule struct {
	Name string `json:"name"`
	Cron string `json:"cron"`
}

// NetlifyDeployResponse represents the deployment response
type NetlifyDeployResponse struct {
	ID            string   `json:"id"`
	SiteID        string   `json:"site_id"`
	BuildID       string   `json:"build_id,omitempty"`
	State         string   `json:"state"` // new, pending_review, accepted, rejected, enqueued, building, ready, error
	Name          string   `json:"name"`
	URL           string   `json:"url"`
	SslURL        string   `json:"ssl_url"`
	AdminURL      string   `json:"admin_url"`
	DeployURL     string   `json:"deploy_url"`
	DeploySslURL  string   `json:"deploy_ssl_url"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	PublishedAt   string   `json:"published_at,omitempty"`
	Title         string   `json:"title,omitempty"`
	Branch        string   `json:"branch,omitempty"`
	CommitRef     string   `json:"commit_ref,omitempty"`
	ReviewURL     string   `json:"review_url,omitempty"`
	Framework     string   `json:"framework,omitempty"`
	ErrorMessage  string   `json:"error_message,omitempty"`
	Required      []string `json:"required,omitempty"` // Files that need to be uploaded
	RequiredFunctions []string `json:"required_functions,omitempty"`
}

// NetlifyUploadResponse represents file upload response
type NetlifyUploadResponse struct {
	ID   string `json:"id"`
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
}

// Deploy deploys files to Netlify
func (p *NetlifyProvider) Deploy(ctx context.Context, config *deploy.DeploymentConfig, files []deploy.ProjectFile) (*deploy.ProviderDeploymentResult, error) {
	// First, create or get the site
	siteName := fmt.Sprintf("apex-project-%d", config.ProjectID)
	site, err := p.getOrCreateSite(ctx, siteName, config)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create site: %w", err)
	}

	// Calculate file digests
	fileDigests := make(map[string]string)
	fileContents := make(map[string][]byte)

	for _, f := range files {
		if f.IsDir {
			continue
		}

		// Clean path - remove leading /
		path := strings.TrimPrefix(f.Path, "/")

		// Calculate SHA1 hash
		hash := sha1.Sum([]byte(f.Content))
		sha := hex.EncodeToString(hash[:])

		fileDigests["/"+path] = sha
		fileContents["/"+path] = []byte(f.Content)
	}

	// Create deployment with file digests
	deployReq := NetlifyDeployRequest{
		Files: fileDigests,
		Async: true,
		Draft: config.Environment != "production",
		Title: fmt.Sprintf("APEX.BUILD deployment - %s", time.Now().Format(time.RFC3339)),
		Branch: config.Branch,
	}

	deployResp, err := p.createDeployment(ctx, site.ID, &deployReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	// Upload required files
	if len(deployResp.Required) > 0 {
		for _, requiredPath := range deployResp.Required {
			content, exists := fileContents[requiredPath]
			if !exists {
				// Try without leading slash
				content, exists = fileContents[strings.TrimPrefix(requiredPath, "/")]
			}
			if !exists {
				continue
			}

			err := p.uploadFile(ctx, deployResp.ID, requiredPath, content)
			if err != nil {
				return nil, fmt.Errorf("failed to upload file %s: %w", requiredPath, err)
			}
		}
	}

	// Wait for deployment to complete
	finalStatus, err := p.waitForDeployment(ctx, deployResp.ID)
	if err != nil {
		return nil, err
	}

	// Get build logs
	logs, _ := p.GetLogs(ctx, deployResp.ID)

	return &deploy.ProviderDeploymentResult{
		ProviderID:   deployResp.ID,
		Status:       p.mapStatus(finalStatus.State),
		URL:          finalStatus.SslURL,
		PreviewURL:   finalStatus.DeploySslURL,
		BuildLogs:    logs,
		ErrorMessage: finalStatus.ErrorMessage,
	}, nil
}

// getOrCreateSite gets an existing site or creates a new one
func (p *NetlifyProvider) getOrCreateSite(ctx context.Context, name string, config *deploy.DeploymentConfig) (*NetlifySite, error) {
	// Try to find existing site
	sites, err := p.listSites(ctx)
	if err == nil {
		for _, site := range sites {
			if site.Name == name {
				return &site, nil
			}
		}
	}

	// Create new site
	return p.createSite(ctx, name, config)
}

// createSite creates a new Netlify site
func (p *NetlifyProvider) createSite(ctx context.Context, name string, config *deploy.DeploymentConfig) (*NetlifySite, error) {
	siteReq := map[string]interface{}{
		"name": name,
		"build_settings": map[string]interface{}{
			"cmd":         config.BuildCommand,
			"dir":         config.OutputDir,
			"install_cmd": config.InstallCmd,
		},
	}

	body, err := json.Marshal(siteReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", netlifyAPIBase+"/sites", bytes.NewReader(body))
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
		return nil, fmt.Errorf("failed to create site: %s - %s", resp.Status, string(respBody))
	}

	var site NetlifySite
	if err := json.Unmarshal(respBody, &site); err != nil {
		return nil, err
	}

	return &site, nil
}

// listSites lists all sites
func (p *NetlifyProvider) listSites(ctx context.Context) ([]NetlifySite, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", netlifyAPIBase+"/sites", nil)
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

	var sites []NetlifySite
	if err := json.Unmarshal(body, &sites); err != nil {
		return nil, err
	}

	return sites, nil
}

// createDeployment creates a new deployment
func (p *NetlifyProvider) createDeployment(ctx context.Context, siteID string, req *NetlifyDeployRequest) (*NetlifyDeployResponse, error) {
	url := fmt.Sprintf("%s/sites/%s/deploys", netlifyAPIBase, siteID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to create deployment: %s - %s", resp.Status, string(respBody))
	}

	var deployResp NetlifyDeployResponse
	if err := json.Unmarshal(respBody, &deployResp); err != nil {
		return nil, err
	}

	return &deployResp, nil
}

// uploadFile uploads a file to a deployment
func (p *NetlifyProvider) uploadFile(ctx context.Context, deployID, path string, content []byte) error {
	url := fmt.Sprintf("%s/deploys/%s/files%s", netlifyAPIBase, deployID, path)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(content))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload file: %s - %s", resp.Status, string(body))
	}

	return nil
}

// waitForDeployment waits for deployment to complete
func (p *NetlifyProvider) waitForDeployment(ctx context.Context, deployID string) (*NetlifyDeployResponse, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(10 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("deployment timed out")
		case <-ticker.C:
			status, err := p.getDeploymentStatus(ctx, deployID)
			if err != nil {
				return nil, err
			}

			switch status.State {
			case "ready":
				return status, nil
			case "error":
				return nil, fmt.Errorf("deployment failed: %s", status.ErrorMessage)
			}
			// Continue waiting for new, pending_review, accepted, enqueued, building states
		}
	}
}

// getDeploymentStatus gets the current deployment status
func (p *NetlifyProvider) getDeploymentStatus(ctx context.Context, deployID string) (*NetlifyDeployResponse, error) {
	url := fmt.Sprintf("%s/deploys/%s", netlifyAPIBase, deployID)

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
		return nil, fmt.Errorf("failed to get deployment status: %s", string(body))
	}

	var deployResp NetlifyDeployResponse
	if err := json.Unmarshal(body, &deployResp); err != nil {
		return nil, err
	}

	return &deployResp, nil
}

// GetStatus gets the current status of a deployment
func (p *NetlifyProvider) GetStatus(ctx context.Context, deploymentID string) (*deploy.ProviderDeploymentResult, error) {
	status, err := p.getDeploymentStatus(ctx, deploymentID)
	if err != nil {
		return nil, err
	}

	return &deploy.ProviderDeploymentResult{
		ProviderID:   status.ID,
		Status:       p.mapStatus(status.State),
		URL:          status.SslURL,
		PreviewURL:   status.DeploySslURL,
		ErrorMessage: status.ErrorMessage,
	}, nil
}

// Cancel cancels a deployment
func (p *NetlifyProvider) Cancel(ctx context.Context, deploymentID string) error {
	url := fmt.Sprintf("%s/deploys/%s/cancel", netlifyAPIBase, deploymentID)

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
func (p *NetlifyProvider) GetLogs(ctx context.Context, deploymentID string) ([]string, error) {
	// Get the deploy details first to get build_id
	deploy, err := p.getDeploymentStatus(ctx, deploymentID)
	if err != nil {
		return nil, err
	}

	if deploy.BuildID == "" {
		return []string{"Build logs not available"}, nil
	}

	// Get build logs
	url := fmt.Sprintf("%s/builds/%s/log", netlifyAPIBase, deploy.BuildID)

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

	// Parse the log lines
	var logEntries []struct {
		Message string `json:"message"`
		Section string `json:"section"`
		Time    int64  `json:"ts"`
	}

	if err := json.Unmarshal(body, &logEntries); err != nil {
		// If it's not JSON, treat it as plain text
		return strings.Split(string(body), "\n"), nil
	}

	logs := make([]string, 0, len(logEntries))
	for _, entry := range logEntries {
		if entry.Message != "" {
			logs = append(logs, entry.Message)
		}
	}

	return logs, nil
}

// ValidateConfig validates deployment configuration
func (p *NetlifyProvider) ValidateConfig(config *deploy.DeploymentConfig) error {
	if config.ProjectID == 0 {
		return fmt.Errorf("project ID is required")
	}
	return nil
}

// Helper functions

func (p *NetlifyProvider) mapStatus(state string) deploy.DeploymentStatus {
	switch state {
	case "new", "pending_review", "accepted":
		return deploy.StatusPreparing
	case "enqueued", "building":
		return deploy.StatusBuilding
	case "ready":
		return deploy.StatusLive
	case "error":
		return deploy.StatusFailed
	default:
		return deploy.StatusPending
	}
}

// PublishDeploy publishes a deploy to production
func (p *NetlifyProvider) PublishDeploy(ctx context.Context, siteID, deployID string) error {
	url := fmt.Sprintf("%s/sites/%s/deploys/%s/restore", netlifyAPIBase, siteID, deployID)

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
		return fmt.Errorf("failed to publish deploy: %s", string(body))
	}

	return nil
}

// LockDeploy locks a deploy (prevents auto-publishing)
func (p *NetlifyProvider) LockDeploy(ctx context.Context, deployID string) error {
	url := fmt.Sprintf("%s/deploys/%s/lock", netlifyAPIBase, deployID)

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
		return fmt.Errorf("failed to lock deploy: %s", string(body))
	}

	return nil
}

// UnlockDeploy unlocks a deploy
func (p *NetlifyProvider) UnlockDeploy(ctx context.Context, deployID string) error {
	url := fmt.Sprintf("%s/deploys/%s/unlock", netlifyAPIBase, deployID)

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
		return fmt.Errorf("failed to unlock deploy: %s", string(body))
	}

	return nil
}

// DeleteSite deletes a site
func (p *NetlifyProvider) DeleteSite(ctx context.Context, siteID string) error {
	url := fmt.Sprintf("%s/sites/%s", netlifyAPIBase, siteID)

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
		return fmt.Errorf("failed to delete site: %s", string(body))
	}

	return nil
}

// UpdateSiteSettings updates site settings
func (p *NetlifyProvider) UpdateSiteSettings(ctx context.Context, siteID string, settings map[string]interface{}) error {
	url := fmt.Sprintf("%s/sites/%s", netlifyAPIBase, siteID)

	body, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(body))
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
		return fmt.Errorf("failed to update site settings: %s", string(respBody))
	}

	return nil
}

// SetEnvironmentVariables sets environment variables for a site
func (p *NetlifyProvider) SetEnvironmentVariables(ctx context.Context, siteID string, envVars map[string]string) error {
	return p.UpdateSiteSettings(ctx, siteID, map[string]interface{}{
		"build_settings": map[string]interface{}{
			"env": envVars,
		},
	})
}
