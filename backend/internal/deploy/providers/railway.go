package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"apex-build/internal/deploy"
)

type RailwayProvider struct {
	token     string
	workspace string
	run       cliRunner
	lookPath  binaryLookup
}

func NewRailwayProvider(token, workspace string) *RailwayProvider {
	return &RailwayProvider{
		token:     token,
		workspace: strings.TrimSpace(workspace),
		run:       defaultCLIRunner,
		lookPath:  exec.LookPath,
	}
}

func (p *RailwayProvider) Name() deploy.DeploymentProvider {
	return deploy.ProviderRailway
}

func (p *RailwayProvider) Deploy(ctx context.Context, config *deploy.DeploymentConfig, files []deploy.ProjectFile) (*deploy.ProviderDeploymentResult, error) {
	workspaceDir, err := writeProjectWorkspace(files)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workspaceDir)

	projectDir, err := resolveWorkspaceDir(workspaceDir, config.RootDirectory)
	if err != nil {
		return nil, fmt.Errorf("invalid Railway root directory: %w", err)
	}

	env := []string{
		"RAILWAY_TOKEN=" + p.token,
		"CI=true",
	}

	projectName, projectRef, serviceRef, environmentRef := p.resolveRefs(config)
	logs := make([]string, 0, 32)

	if projectRef == "" {
		args := []string{"init", "--name", projectName, "--json"}
		if p.workspace != "" {
			args = append(args, "--workspace", p.workspace)
		}
		out, err := p.run(ctx, projectDir, env, "railway", args...)
		logs = append(logs, splitLogLines(out)...)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Railway project: %w", err)
		}

		statusOut, err := p.run(ctx, projectDir, env, "railway", "status", "--json")
		logs = append(logs, splitLogLines(statusOut)...)
		if err != nil {
			return nil, fmt.Errorf("failed to read Railway project status: %w", err)
		}
		statusData := decodeLooseJSON(statusOut)
		if projectRef == "" {
			projectRef = findStringValue(statusData, "projectId", "projectID", "id")
		}
		if projectName == "" {
			projectName = findStringValue(statusData, "projectName", "name")
		}
	}

	if projectName == "" {
		projectName = sanitizeDeploymentName(fmt.Sprintf("apex-project-%d", config.ProjectID))
	}
	if projectRef == "" {
		projectRef = projectName
	}

	if serviceRef == "" {
		serviceName := "web"
		addOut, err := p.run(ctx, projectDir, env, "railway", "add", "--service", serviceName, "--json")
		logs = append(logs, splitLogLines(addOut)...)
		if err != nil && !strings.Contains(strings.ToLower(addOut), "already exists") {
			return nil, fmt.Errorf("failed to create Railway service: %w", err)
		}
		serviceData := decodeLooseJSON(addOut)
		serviceRef = findStringValue(serviceData, "serviceId", "serviceID", "id", "name")
		if serviceRef == "" {
			serviceRef = serviceName
		}
	}

	if err := p.link(ctx, projectDir, env, projectRef, serviceRef, environmentRef); err != nil {
		return nil, err
	}
	if err := p.ensureRailwayConfig(projectDir, config); err != nil {
		return nil, err
	}

	if len(config.EnvVars) > 0 {
		varOut, err := p.setVariables(ctx, projectDir, env, serviceRef, environmentRef, config.EnvVars)
		logs = append(logs, splitLogLines(varOut)...)
		if err != nil {
			return nil, fmt.Errorf("failed to sync Railway variables: %w", err)
		}
	}

	upArgs := []string{"up", "--ci", "--json", "--service", serviceRef}
	if environmentRef != "" {
		upArgs = append(upArgs, "--environment", environmentRef)
	}
	upOut, err := p.run(ctx, projectDir, env, "railway", upArgs...)
	logs = append(logs, splitLogLines(upOut)...)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy to Railway: %w", err)
	}

	deploymentOut, err := p.deploymentList(ctx, projectDir, env, serviceRef, environmentRef)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect Railway deployment: %w", err)
	}
	deploymentData := latestRailwayDeployment(decodeLooseJSON(deploymentOut))
	deploymentRef := findStringValue(deploymentData, "id", "deploymentId", "deploymentID")

	domainOut, _ := p.domain(ctx, projectDir, env, serviceRef)
	url := extractRailwayURL(domainOut)
	if url == "" {
		url = extractRailwayURL(upOut)
	}
	if url == "" {
		url = extractRailwayURL(deploymentOut)
	}

	result := &deploy.ProviderDeploymentResult{
		ProviderID: marshalProviderRef(map[string]string{
			"railway_project":      projectRef,
			"railway_project_name": projectName,
			"railway_service":      serviceRef,
			"railway_environment":  environmentRef,
			"railway_deployment":   deploymentRef,
		}),
		Status:    railwayStatus(findStringValue(deploymentData, "status", "state")),
		URL:       url,
		BuildLogs: logs,
		Metadata: map[string]any{
			"railway_project":      projectRef,
			"railway_project_name": projectName,
			"railway_service":      serviceRef,
		},
	}
	if environmentRef != "" {
		result.Metadata["railway_environment"] = environmentRef
	}
	if deploymentRef != "" {
		result.Metadata["railway_deployment"] = deploymentRef
	}

	return result, nil
}

func (p *RailwayProvider) GetStatus(ctx context.Context, deploymentID string) (*deploy.ProviderDeploymentResult, error) {
	ref := unmarshalProviderRef(deploymentID)
	projectRef := strings.TrimSpace(ref["railway_project"])
	serviceRef := strings.TrimSpace(ref["railway_service"])
	environmentRef := strings.TrimSpace(ref["railway_environment"])
	if projectRef == "" || serviceRef == "" {
		return nil, fmt.Errorf("invalid Railway deployment reference")
	}

	workspaceDir, err := os.MkdirTemp("", "apex-railway-status-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workspaceDir)

	env := []string{
		"RAILWAY_TOKEN=" + p.token,
		"CI=true",
	}
	if err := p.link(ctx, workspaceDir, env, projectRef, serviceRef, environmentRef); err != nil {
		return nil, err
	}

	statusOut, err := p.deploymentList(ctx, workspaceDir, env, serviceRef, environmentRef)
	if err != nil {
		return nil, err
	}
	deploymentData := latestRailwayDeployment(decodeLooseJSON(statusOut))
	urlOut, _ := p.domain(ctx, workspaceDir, env, serviceRef)

	return &deploy.ProviderDeploymentResult{
		ProviderID:   deploymentID,
		Status:       railwayStatus(findStringValue(deploymentData, "status", "state")),
		URL:          extractRailwayURL(urlOut),
		ErrorMessage: findStringValue(deploymentData, "error", "message"),
	}, nil
}

func (p *RailwayProvider) Cancel(ctx context.Context, deploymentID string) error {
	ref := unmarshalProviderRef(deploymentID)
	projectRef := strings.TrimSpace(ref["railway_project"])
	serviceRef := strings.TrimSpace(ref["railway_service"])
	environmentRef := strings.TrimSpace(ref["railway_environment"])
	if projectRef == "" || serviceRef == "" {
		return fmt.Errorf("invalid Railway deployment reference")
	}

	workspaceDir, err := os.MkdirTemp("", "apex-railway-cancel-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspaceDir)

	env := []string{
		"RAILWAY_TOKEN=" + p.token,
		"CI=true",
	}
	if err := p.link(ctx, workspaceDir, env, projectRef, serviceRef, environmentRef); err != nil {
		return err
	}

	args := []string{"down", "--service", serviceRef, "--yes"}
	if environmentRef != "" {
		args = append(args, "--environment", environmentRef)
	}
	_, err = p.run(ctx, workspaceDir, env, "railway", args...)
	return err
}

func (p *RailwayProvider) GetLogs(ctx context.Context, deploymentID string) ([]string, error) {
	ref := unmarshalProviderRef(deploymentID)
	projectRef := strings.TrimSpace(ref["railway_project"])
	serviceRef := strings.TrimSpace(ref["railway_service"])
	environmentRef := strings.TrimSpace(ref["railway_environment"])
	deploymentRef := strings.TrimSpace(ref["railway_deployment"])
	if projectRef == "" || serviceRef == "" {
		return nil, fmt.Errorf("invalid Railway deployment reference")
	}

	workspaceDir, err := os.MkdirTemp("", "apex-railway-logs-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workspaceDir)

	env := []string{
		"RAILWAY_TOKEN=" + p.token,
		"CI=true",
	}
	if err := p.link(ctx, workspaceDir, env, projectRef, serviceRef, environmentRef); err != nil {
		return nil, err
	}

	args := []string{"service", "logs", "--build", "--deployment", "--lines", "200", "--service", serviceRef}
	if environmentRef != "" {
		args = append(args, "--environment", environmentRef)
	}
	if deploymentRef != "" {
		args = append(args, deploymentRef)
	}
	out, err := p.run(ctx, workspaceDir, env, "railway", args...)
	if err != nil {
		return nil, err
	}
	return splitLogLines(out), nil
}

func (p *RailwayProvider) ValidateConfig(config *deploy.DeploymentConfig) error {
	if config.ProjectID == 0 {
		return fmt.Errorf("project ID is required")
	}
	if strings.TrimSpace(p.token) == "" {
		return fmt.Errorf("RAILWAY_TOKEN is not configured")
	}
	if _, err := p.lookPath("railway"); err != nil {
		return fmt.Errorf("railway CLI is not installed")
	}
	return nil
}

func (p *RailwayProvider) resolveRefs(config *deploy.DeploymentConfig) (projectName, projectRef, serviceRef, environmentRef string) {
	if config == nil {
		return "apex-app", "", "web", ""
	}
	projectName = sanitizeDeploymentName(fmt.Sprintf("apex-project-%d", config.ProjectID))
	serviceRef = "web"
	if config.Custom == nil {
		if strings.EqualFold(strings.TrimSpace(config.Environment), "production") {
			environmentRef = "production"
		}
		return
	}
	if value, ok := config.Custom["railway_project_name"].(string); ok && strings.TrimSpace(value) != "" {
		projectName = sanitizeDeploymentName(value)
	}
	if value, ok := config.Custom["railway_project"].(string); ok {
		projectRef = strings.TrimSpace(value)
	}
	if value, ok := config.Custom["railway_service"].(string); ok && strings.TrimSpace(value) != "" {
		serviceRef = strings.TrimSpace(value)
	}
	if value, ok := config.Custom["railway_environment"].(string); ok {
		environmentRef = strings.TrimSpace(value)
	}
	if environmentRef == "" && strings.EqualFold(strings.TrimSpace(config.Environment), "production") {
		environmentRef = "production"
	}
	return
}

func (p *RailwayProvider) link(ctx context.Context, dir string, env []string, projectRef, serviceRef, environmentRef string) error {
	args := []string{"link", "--project", projectRef, "--json"}
	if serviceRef != "" {
		args = append(args, "--service", serviceRef)
	}
	if environmentRef != "" {
		args = append(args, "--environment", environmentRef)
	}
	if p.workspace != "" {
		args = append(args, "--workspace", p.workspace)
	}
	_, err := p.run(ctx, dir, env, "railway", args...)
	return err
}

func (p *RailwayProvider) setVariables(ctx context.Context, dir string, env []string, serviceRef, environmentRef string, values map[string]string) (string, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	args := []string{"variable", "set"}
	if serviceRef != "" {
		args = append(args, "--service", serviceRef)
	}
	if environmentRef != "" {
		args = append(args, "--environment", environmentRef)
	}
	args = append(args, "--skip-deploys", "--json")
	for _, key := range keys {
		args = append(args, fmt.Sprintf("%s=%s", key, values[key]))
	}

	return p.run(ctx, dir, env, "railway", args...)
}

func (p *RailwayProvider) ensureRailwayConfig(dir string, config *deploy.DeploymentConfig) error {
	if config == nil {
		return nil
	}
	if _, err := os.Stat(filepath.Join(dir, "railway.json")); err == nil {
		return nil
	}
	if _, err := os.Stat(filepath.Join(dir, "railway.toml")); err == nil {
		return nil
	}

	payload := map[string]any{
		"$schema": "https://railway.com/railway.schema.json",
	}
	build := map[string]any{}
	deploySection := map[string]any{}
	if strings.TrimSpace(config.BuildCommand) != "" {
		build["buildCommand"] = config.BuildCommand
	}
	if strings.TrimSpace(config.StartCommand) != "" {
		deploySection["startCommand"] = config.StartCommand
	}
	if len(build) > 0 {
		payload["build"] = build
	}
	if len(deploySection) > 0 {
		payload["deploy"] = deploySection
	}
	if len(payload) == 1 {
		return nil
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "railway.json"), data, 0o644)
}

func (p *RailwayProvider) deploymentList(ctx context.Context, dir string, env []string, serviceRef, environmentRef string) (string, error) {
	args := []string{"deployment", "list", "--json", "--limit", "1"}
	if serviceRef != "" {
		args = append(args, "--service", serviceRef)
	}
	if environmentRef != "" {
		args = append(args, "--environment", environmentRef)
	}
	return p.run(ctx, dir, env, "railway", args...)
}

func (p *RailwayProvider) domain(ctx context.Context, dir string, env []string, serviceRef string) (string, error) {
	args := []string{"domain", "--json"}
	if serviceRef != "" {
		args = append(args, "--service", serviceRef)
	}
	return p.run(ctx, dir, env, "railway", args...)
}

func latestRailwayDeployment(data any) map[string]any {
	switch node := data.(type) {
	case []any:
		if len(node) == 0 {
			return nil
		}
		if deploymentMap, ok := node[0].(map[string]any); ok {
			return deploymentMap
		}
	case map[string]any:
		if deployments, ok := node["deployments"].([]any); ok && len(deployments) > 0 {
			if deploymentMap, ok := deployments[0].(map[string]any); ok {
				return deploymentMap
			}
		}
		return node
	}
	return nil
}

func railwayStatus(status string) deploy.DeploymentStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "succeeded", "live", "deployed":
		return deploy.StatusLive
	case "building", "build", "initializing":
		return deploy.StatusBuilding
	case "deploying", "deploy":
		return deploy.StatusDeploying
	case "failed", "error", "crashed", "removed":
		return deploy.StatusFailed
	case "canceled", "cancelled":
		return deploy.StatusCancelled
	case "queued", "pending":
		return deploy.StatusPending
	default:
		return deploy.StatusPending
	}
}

func extractRailwayURL(output string) string {
	value := strings.TrimSpace(findStringValue(decodeLooseJSON(output), "domain", "serviceDomain", "url"))
	if value == "" {
		value = extractURL(output, "railway")
	}
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "https://" + value
}
