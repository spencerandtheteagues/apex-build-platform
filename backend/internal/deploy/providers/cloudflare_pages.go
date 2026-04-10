package providers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"apex-build/internal/deploy"
)

type CloudflarePagesProvider struct {
	apiToken  string
	accountID string
	run       cliRunner
	lookPath  binaryLookup
}

func NewCloudflarePagesProvider(apiToken, accountID string) *CloudflarePagesProvider {
	return &CloudflarePagesProvider{
		apiToken:  strings.TrimSpace(apiToken),
		accountID: strings.TrimSpace(accountID),
		run:       defaultCLIRunner,
		lookPath:  exec.LookPath,
	}
}

func (p *CloudflarePagesProvider) Name() deploy.DeploymentProvider {
	return deploy.ProviderCloudflarePages
}

func (p *CloudflarePagesProvider) Deploy(ctx context.Context, config *deploy.DeploymentConfig, files []deploy.ProjectFile) (*deploy.ProviderDeploymentResult, error) {
	workspaceDir, err := writeProjectWorkspace(files)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workspaceDir)

	projectDir, err := resolveWorkspaceDir(workspaceDir, config.RootDirectory)
	if err != nil {
		return nil, fmt.Errorf("invalid Cloudflare root directory: %w", err)
	}

	env := []string{
		"CLOUDFLARE_API_TOKEN=" + p.apiToken,
		"CLOUDFLARE_ACCOUNT_ID=" + p.accountID,
		"CI=true",
	}

	projectName := p.resolveProjectName(config)
	logs := make([]string, 0, 32)

	buildLogs, err := p.buildStaticSite(ctx, projectDir, env, config)
	logs = append(logs, buildLogs...)
	if err != nil {
		return nil, err
	}

	if err := p.ensureProject(ctx, projectDir, env, projectName, config.Branch); err != nil {
		return nil, err
	}

	deployDir, err := p.resolveDeployDirectory(projectDir, config.OutputDir)
	if err != nil {
		return nil, err
	}

	args := []string{"pages", "deploy", deployDir, "--project-name", projectName}
	if strings.TrimSpace(config.Branch) != "" {
		args = append(args, "--branch", strings.TrimSpace(config.Branch))
	}
	deployOut, err := p.run(ctx, projectDir, env, "wrangler", args...)
	logs = append(logs, splitLogLines(deployOut)...)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy to Cloudflare Pages: %w", err)
	}

	deploymentOut, _ := p.deploymentList(ctx, projectDir, env, projectName)
	deploymentData := latestCloudflarePagesDeployment(decodeLooseJSON(deploymentOut))
	url := extractCloudflarePagesURL(deploymentOut)
	if url == "" {
		url = extractCloudflarePagesURL(deployOut)
	}
	deploymentID := findStringValue(deploymentData, "id", "deployment_id", "deploymentId", "short_id")

	result := &deploy.ProviderDeploymentResult{
		ProviderID: marshalProviderRef(map[string]string{
			"cloudflare_pages_project":    projectName,
			"cloudflare_pages_deployment": deploymentID,
			"cloudflare_pages_branch":     strings.TrimSpace(config.Branch),
		}),
		Status:    cloudflarePagesStatus(findStringValue(deploymentData, "status", "latest_stage", "stage")),
		URL:       url,
		BuildLogs: logs,
		Metadata: map[string]any{
			"cloudflare_pages_project": projectName,
		},
	}
	if deploymentID != "" {
		result.Metadata["cloudflare_pages_deployment"] = deploymentID
	}
	if strings.TrimSpace(config.Branch) != "" {
		result.Metadata["cloudflare_pages_branch"] = strings.TrimSpace(config.Branch)
	}

	return result, nil
}

func (p *CloudflarePagesProvider) GetStatus(ctx context.Context, deploymentID string) (*deploy.ProviderDeploymentResult, error) {
	ref := unmarshalProviderRef(deploymentID)
	projectName := strings.TrimSpace(ref["cloudflare_pages_project"])
	if projectName == "" {
		return nil, fmt.Errorf("invalid Cloudflare Pages deployment reference")
	}

	workspaceDir, err := os.MkdirTemp("", "apex-cf-pages-status-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workspaceDir)

	env := []string{
		"CLOUDFLARE_API_TOKEN=" + p.apiToken,
		"CLOUDFLARE_ACCOUNT_ID=" + p.accountID,
		"CI=true",
	}

	out, err := p.deploymentList(ctx, workspaceDir, env, projectName)
	if err != nil {
		return nil, err
	}
	deploymentData := latestCloudflarePagesDeployment(decodeLooseJSON(out))

	return &deploy.ProviderDeploymentResult{
		ProviderID: deploymentID,
		Status:     cloudflarePagesStatus(findStringValue(deploymentData, "status", "latest_stage", "stage")),
		URL:        extractCloudflarePagesURL(out),
	}, nil
}

func (p *CloudflarePagesProvider) Cancel(context.Context, string) error {
	return fmt.Errorf("Cloudflare Pages cancellation is not supported by this provider")
}

func (p *CloudflarePagesProvider) GetLogs(context.Context, string) ([]string, error) {
	return []string{"Cloudflare Pages historical logs are not exposed through this integration."}, nil
}

func (p *CloudflarePagesProvider) ValidateConfig(config *deploy.DeploymentConfig) error {
	if config.ProjectID == 0 {
		return fmt.Errorf("project ID is required")
	}
	if strings.TrimSpace(p.apiToken) == "" {
		return fmt.Errorf("CLOUDFLARE_API_TOKEN is not configured")
	}
	if strings.TrimSpace(p.accountID) == "" {
		return fmt.Errorf("CLOUDFLARE_ACCOUNT_ID is not configured")
	}
	if _, err := p.lookPath("wrangler"); err != nil {
		return fmt.Errorf("wrangler CLI is not installed")
	}
	if !supportsCloudflarePages(config.Framework, config.StartCommand) {
		return fmt.Errorf("Cloudflare Pages only supports static frontend deployments")
	}
	return nil
}

func (p *CloudflarePagesProvider) resolveProjectName(config *deploy.DeploymentConfig) string {
	if config != nil && config.Custom != nil {
		if value, ok := config.Custom["cloudflare_pages_project"].(string); ok && strings.TrimSpace(value) != "" {
			return sanitizeDeploymentName(value)
		}
	}
	return sanitizeDeploymentName(fmt.Sprintf("apex-project-%d", config.ProjectID))
}

func (p *CloudflarePagesProvider) buildStaticSite(ctx context.Context, projectDir string, env []string, config *deploy.DeploymentConfig) ([]string, error) {
	logs := make([]string, 0, 8)
	if strings.TrimSpace(config.InstallCmd) != "" {
		out, err := runShell(ctx, p.run, projectDir, append(env, flattenEnvVars(config.EnvVars)...), config.InstallCmd)
		logs = append(logs, splitLogLines(out)...)
		if err != nil {
			return logs, fmt.Errorf("failed to install dependencies for Cloudflare Pages: %w", err)
		}
	}
	if strings.TrimSpace(config.BuildCommand) != "" {
		out, err := runShell(ctx, p.run, projectDir, append(env, flattenEnvVars(config.EnvVars)...), config.BuildCommand)
		logs = append(logs, splitLogLines(out)...)
		if err != nil {
			return logs, fmt.Errorf("failed to build static site for Cloudflare Pages: %w", err)
		}
	}
	return logs, nil
}

func (p *CloudflarePagesProvider) ensureProject(ctx context.Context, dir string, env []string, projectName, branch string) error {
	listOut, err := p.run(ctx, dir, env, "wrangler", "pages", "project", "list", "--json")
	if err == nil && cloudflareProjectExists(decodeLooseJSON(listOut), projectName) {
		return nil
	}

	args := []string{"pages", "project", "create", projectName}
	if strings.TrimSpace(branch) != "" {
		args = append(args, "--production-branch", strings.TrimSpace(branch))
	}
	out, err := p.run(ctx, dir, env, "wrangler", args...)
	if err != nil && !strings.Contains(strings.ToLower(out), "already exists") {
		return fmt.Errorf("failed to create Cloudflare Pages project: %w", err)
	}
	return nil
}

func (p *CloudflarePagesProvider) resolveDeployDirectory(projectDir, outputDir string) (string, error) {
	if strings.TrimSpace(outputDir) == "" || strings.TrimSpace(outputDir) == "." {
		return projectDir, nil
	}
	relPath, err := safeRelativePath(outputDir)
	if err != nil {
		return "", err
	}
	target := filepath.Join(projectDir, relPath)
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", outputDir)
	}
	return target, nil
}

func (p *CloudflarePagesProvider) deploymentList(ctx context.Context, dir string, env []string, projectName string) (string, error) {
	return p.run(ctx, dir, env, "wrangler", "pages", "deployment", "list", "--project-name", projectName, "--json")
}

func supportsCloudflarePages(framework, startCommand string) bool {
	if strings.TrimSpace(startCommand) != "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(framework)) {
	case "", "static", "react", "vue", "svelte", "angular", "static_html":
		return true
	default:
		return false
	}
}

func flattenEnvVars(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return env
}

func cloudflareProjectExists(data any, projectName string) bool {
	switch node := data.(type) {
	case []any:
		for _, item := range node {
			if findStringValue(item, "name", "project_name") == projectName {
				return true
			}
		}
	case map[string]any:
		if findStringValue(node, "name", "project_name") == projectName {
			return true
		}
	}
	return false
}

func latestCloudflarePagesDeployment(data any) map[string]any {
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

func cloudflarePagesStatus(status string) deploy.DeploymentStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "active", "live", "ready":
		return deploy.StatusLive
	case "failure", "failed", "error":
		return deploy.StatusFailed
	case "building", "queued", "pending":
		return deploy.StatusBuilding
	case "canceled", "cancelled":
		return deploy.StatusCancelled
	default:
		return deploy.StatusPending
	}
}

func extractCloudflarePagesURL(output string) string {
	value := strings.TrimSpace(findStringValue(decodeLooseJSON(output), "url", "aliases"))
	if value == "" {
		value = extractURL(output, ".pages.dev")
	}
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "https://" + value
}
