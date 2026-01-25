// APEX.BUILD Vercel Deployment Provider
// Full Vercel API integration for deployments

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
	vercelAPIBase = "https://api.vercel.com"
)

// VercelProvider implements the Provider interface for Vercel
type VercelProvider struct {
	token      string
	httpClient *http.Client
	teamID     string
}

// NewVercelProvider creates a new Vercel provider
func NewVercelProvider(token string) *VercelProvider {
	return &VercelProvider{
		token: token,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// SetTeamID sets the team ID for Vercel deployments
func (p *VercelProvider) SetTeamID(teamID string) {
	p.teamID = teamID
}

// Name returns the provider name
func (p *VercelProvider) Name() deploy.DeploymentProvider {
	return deploy.ProviderVercel
}

// VercelDeploymentRequest represents a Vercel deployment request
type VercelDeploymentRequest struct {
	Name        string                    `json:"name"`
	Files       []VercelFile              `json:"files"`
	ProjectSettings *VercelProjectSettings `json:"projectSettings,omitempty"`
	Target      string                    `json:"target,omitempty"`
	GitSource   *VercelGitSource          `json:"gitSource,omitempty"`
}

// VercelFile represents a file for Vercel deployment
type VercelFile struct {
	File string `json:"file"`
	SHA  string `json:"sha,omitempty"`
	Size int64  `json:"size,omitempty"`
	Data string `json:"data,omitempty"`
}

// VercelProjectSettings contains project settings
type VercelProjectSettings struct {
	Framework          string            `json:"framework,omitempty"`
	BuildCommand       string            `json:"buildCommand,omitempty"`
	OutputDirectory    string            `json:"outputDirectory,omitempty"`
	InstallCommand     string            `json:"installCommand,omitempty"`
	DevCommand         string            `json:"devCommand,omitempty"`
	RootDirectory      string            `json:"rootDirectory,omitempty"`
	NodeVersion        string            `json:"nodeVersion,omitempty"`
	EnvironmentVariables []VercelEnvVar  `json:"environmentVariables,omitempty"`
}

// VercelEnvVar represents an environment variable
type VercelEnvVar struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Target []string `json:"target,omitempty"`
}

// VercelGitSource represents git source information
type VercelGitSource struct {
	Type string `json:"type"`
	Ref  string `json:"ref,omitempty"`
	SHA  string `json:"sha,omitempty"`
}

// VercelDeploymentResponse represents the Vercel API response
type VercelDeploymentResponse struct {
	ID           string   `json:"id"`
	URL          string   `json:"url"`
	Name         string   `json:"name"`
	State        string   `json:"state"` // QUEUED, BUILDING, READY, ERROR, CANCELED
	ReadyState   string   `json:"readyState"`
	CreatedAt    int64    `json:"createdAt"`
	BuildingAt   int64    `json:"buildingAt,omitempty"`
	Ready        int64    `json:"ready,omitempty"`
	Alias        []string `json:"alias,omitempty"`
	InspectorURL string   `json:"inspectorUrl,omitempty"`
	ErrorCode    string   `json:"errorCode,omitempty"`
	ErrorMessage string   `json:"errorMessage,omitempty"`
	Logs         []VercelLog `json:"logs,omitempty"`
}

// VercelLog represents a Vercel build log entry
type VercelLog struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

// VercelError represents a Vercel API error
type VercelError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Deploy deploys files to Vercel
func (p *VercelProvider) Deploy(ctx context.Context, config *deploy.DeploymentConfig, files []deploy.ProjectFile) (*deploy.ProviderDeploymentResult, error) {
	// First, upload all files and get their SHAs
	vercelFiles := make([]VercelFile, 0, len(files))

	for _, f := range files {
		if f.IsDir {
			continue
		}

		// Calculate SHA1 hash
		hash := sha1.Sum([]byte(f.Content))
		sha := hex.EncodeToString(hash[:])

		// Upload file to Vercel
		err := p.uploadFile(ctx, sha, []byte(f.Content))
		if err != nil {
			return nil, fmt.Errorf("failed to upload file %s: %w", f.Path, err)
		}

		// Add to files list with proper path (remove leading /)
		path := strings.TrimPrefix(f.Path, "/")
		vercelFiles = append(vercelFiles, VercelFile{
			File: path,
			SHA:  sha,
			Size: int64(len(f.Content)),
		})
	}

	// Create project settings
	projectSettings := &VercelProjectSettings{}
	if config.Framework != "" {
		projectSettings.Framework = p.mapFramework(config.Framework)
	}
	if config.BuildCommand != "" {
		projectSettings.BuildCommand = config.BuildCommand
	}
	if config.OutputDir != "" {
		projectSettings.OutputDirectory = config.OutputDir
	}
	if config.InstallCmd != "" {
		projectSettings.InstallCommand = config.InstallCmd
	}
	if config.NodeVersion != "" {
		projectSettings.NodeVersion = config.NodeVersion
	}

	// Add environment variables
	if len(config.EnvVars) > 0 {
		for key, value := range config.EnvVars {
			projectSettings.EnvironmentVariables = append(projectSettings.EnvironmentVariables, VercelEnvVar{
				Key:    key,
				Value:  value,
				Target: []string{"production", "preview", "development"},
			})
		}
	}

	// Create deployment request
	deployReq := VercelDeploymentRequest{
		Name:            fmt.Sprintf("apex-project-%d", config.ProjectID),
		Files:           vercelFiles,
		ProjectSettings: projectSettings,
		Target:          p.mapEnvironment(config.Environment),
	}

	// Create deployment
	deployResp, err := p.createDeployment(ctx, &deployReq)
	if err != nil {
		return nil, err
	}

	// Wait for deployment to complete
	finalStatus, err := p.waitForDeployment(ctx, deployResp.ID)
	if err != nil {
		return nil, err
	}

	// Get deployment logs
	logs, _ := p.GetLogs(ctx, deployResp.ID)

	return &deploy.ProviderDeploymentResult{
		ProviderID: deployResp.ID,
		Status:     p.mapStatus(finalStatus.State),
		URL:        fmt.Sprintf("https://%s", finalStatus.URL),
		PreviewURL: finalStatus.InspectorURL,
		BuildLogs:  logs,
	}, nil
}

// uploadFile uploads a file to Vercel
func (p *VercelProvider) uploadFile(ctx context.Context, sha string, content []byte) error {
	url := fmt.Sprintf("%s/v2/files", vercelAPIBase)
	if p.teamID != "" {
		url += "?teamId=" + p.teamID
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(content))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("x-vercel-digest", sha)
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(content)))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 200 OK or 409 Conflict (already exists) are both acceptable
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload file: %s - %s", resp.Status, string(body))
	}

	return nil
}

// createDeployment creates a new deployment
func (p *VercelProvider) createDeployment(ctx context.Context, req *VercelDeploymentRequest) (*VercelDeploymentResponse, error) {
	url := fmt.Sprintf("%s/v13/deployments", vercelAPIBase)
	if p.teamID != "" {
		url += "?teamId=" + p.teamID
	}

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
		var vercelErr VercelError
		json.Unmarshal(respBody, &vercelErr)
		return nil, fmt.Errorf("Vercel API error: %s - %s", vercelErr.Error.Code, vercelErr.Error.Message)
	}

	var deployResp VercelDeploymentResponse
	if err := json.Unmarshal(respBody, &deployResp); err != nil {
		return nil, err
	}

	return &deployResp, nil
}

// waitForDeployment waits for deployment to complete
func (p *VercelProvider) waitForDeployment(ctx context.Context, deploymentID string) (*VercelDeploymentResponse, error) {
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
			status, err := p.getDeploymentStatus(ctx, deploymentID)
			if err != nil {
				return nil, err
			}

			switch status.State {
			case "READY":
				return status, nil
			case "ERROR":
				return nil, fmt.Errorf("deployment failed: %s - %s", status.ErrorCode, status.ErrorMessage)
			case "CANCELED":
				return nil, fmt.Errorf("deployment was cancelled")
			}
			// Continue waiting for QUEUED and BUILDING states
		}
	}
}

// getDeploymentStatus gets the current deployment status
func (p *VercelProvider) getDeploymentStatus(ctx context.Context, deploymentID string) (*VercelDeploymentResponse, error) {
	url := fmt.Sprintf("%s/v13/deployments/%s", vercelAPIBase, deploymentID)
	if p.teamID != "" {
		url += "?teamId=" + p.teamID
	}

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
		var vercelErr VercelError
		json.Unmarshal(body, &vercelErr)
		return nil, fmt.Errorf("Vercel API error: %s - %s", vercelErr.Error.Code, vercelErr.Error.Message)
	}

	var deployResp VercelDeploymentResponse
	if err := json.Unmarshal(body, &deployResp); err != nil {
		return nil, err
	}

	return &deployResp, nil
}

// GetStatus gets the current status of a deployment
func (p *VercelProvider) GetStatus(ctx context.Context, deploymentID string) (*deploy.ProviderDeploymentResult, error) {
	status, err := p.getDeploymentStatus(ctx, deploymentID)
	if err != nil {
		return nil, err
	}

	return &deploy.ProviderDeploymentResult{
		ProviderID:   status.ID,
		Status:       p.mapStatus(status.State),
		URL:          fmt.Sprintf("https://%s", status.URL),
		PreviewURL:   status.InspectorURL,
		ErrorMessage: status.ErrorMessage,
	}, nil
}

// Cancel cancels a deployment
func (p *VercelProvider) Cancel(ctx context.Context, deploymentID string) error {
	url := fmt.Sprintf("%s/v13/deployments/%s/cancel", vercelAPIBase, deploymentID)
	if p.teamID != "" {
		url += "?teamId=" + p.teamID
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, nil)
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
func (p *VercelProvider) GetLogs(ctx context.Context, deploymentID string) ([]string, error) {
	url := fmt.Sprintf("%s/v2/deployments/%s/events", vercelAPIBase, deploymentID)
	if p.teamID != "" {
		url += "?teamId=" + p.teamID
	}

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
		return nil, fmt.Errorf("failed to get logs: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var events []struct {
		Type    string `json:"type"`
		Created int64  `json:"created"`
		Payload struct {
			Text string `json:"text"`
		} `json:"payload"`
	}

	if err := json.Unmarshal(body, &events); err != nil {
		return nil, err
	}

	logs := make([]string, 0, len(events))
	for _, event := range events {
		if event.Payload.Text != "" {
			logs = append(logs, event.Payload.Text)
		}
	}

	return logs, nil
}

// ValidateConfig validates deployment configuration
func (p *VercelProvider) ValidateConfig(config *deploy.DeploymentConfig) error {
	if config.ProjectID == 0 {
		return fmt.Errorf("project ID is required")
	}
	return nil
}

// Helper functions

func (p *VercelProvider) mapStatus(state string) deploy.DeploymentStatus {
	switch state {
	case "QUEUED":
		return deploy.StatusPreparing
	case "BUILDING":
		return deploy.StatusBuilding
	case "READY":
		return deploy.StatusLive
	case "ERROR":
		return deploy.StatusFailed
	case "CANCELED":
		return deploy.StatusCancelled
	default:
		return deploy.StatusPending
	}
}

func (p *VercelProvider) mapFramework(framework string) string {
	frameworkMap := map[string]string{
		"react":   "create-react-app",
		"nextjs":  "nextjs",
		"vue":     "vue",
		"nuxt":    "nuxtjs",
		"svelte":  "svelte",
		"angular": "angular",
		"gatsby":  "gatsby",
		"static":  "",
	}
	if mapped, ok := frameworkMap[framework]; ok && mapped != "" {
		return mapped
	}
	return framework
}

func (p *VercelProvider) mapEnvironment(env string) string {
	switch env {
	case "production":
		return "production"
	case "preview", "staging":
		return "preview"
	default:
		return "production"
	}
}

// ListProjects lists all projects in the Vercel account
func (p *VercelProvider) ListProjects(ctx context.Context) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/v9/projects", vercelAPIBase)
	if p.teamID != "" {
		url += "?teamId=" + p.teamID
	}

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

	var result struct {
		Projects []map[string]interface{} `json:"projects"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.Projects, nil
}

// DeleteDeployment deletes a deployment
func (p *VercelProvider) DeleteDeployment(ctx context.Context, deploymentID string) error {
	url := fmt.Sprintf("%s/v13/deployments/%s", vercelAPIBase, deploymentID)
	if p.teamID != "" {
		url += "?teamId=" + p.teamID
	}

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
		return fmt.Errorf("failed to delete deployment: %s", string(body))
	}

	return nil
}
