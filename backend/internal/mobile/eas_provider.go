package mobile

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type MobileCredentialValueResolver interface {
	ResolveCredentialValues(ctx context.Context, userID, projectID uint, credType MobileCredentialType) (map[string]string, error)
}

type EASCommand struct {
	CLIPath string
	WorkDir string
	Args    []string
	Env     []string
}

type EASCommandRunner interface {
	Run(ctx context.Context, command EASCommand) (string, error)
}

type LocalEASCommandRunner struct{}

func (LocalEASCommandRunner) Run(ctx context.Context, command EASCommand) (string, error) {
	cliPath := strings.TrimSpace(command.CLIPath)
	if cliPath == "" {
		cliPath = "eas"
	}
	cmd := exec.CommandContext(ctx, cliPath, command.Args...)
	cmd.Dir = command.WorkDir
	cmd.Env = append(os.Environ(), command.Env...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

type EASBuildProviderConfig struct {
	CLIPath     string
	Timeout     time.Duration
	Credentials MobileCredentialValueResolver
	Runner      EASCommandRunner
}

type EASBuildProvider struct {
	cliPath     string
	timeout     time.Duration
	credentials MobileCredentialValueResolver
	runner      EASCommandRunner
}

func NewEASBuildProvider(config EASBuildProviderConfig) *EASBuildProvider {
	cliPath := strings.TrimSpace(config.CLIPath)
	if cliPath == "" {
		cliPath = "eas"
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	runner := config.Runner
	if runner == nil {
		runner = LocalEASCommandRunner{}
	}
	return &EASBuildProvider{
		cliPath:     cliPath,
		timeout:     timeout,
		credentials: config.Credentials,
		runner:      runner,
	}
}

func (p *EASBuildProvider) Name() string {
	return "eas"
}

func (p *EASBuildProvider) ValidateBuildRequest(req MobileBuildRequest) error {
	if p == nil {
		return ErrMobileBuildProviderMissing
	}
	sourcePath := strings.TrimSpace(req.SourcePath)
	if sourcePath == "" {
		return fmt.Errorf("%w: source_path is required for EAS builds", ErrMobileBuildInvalidRequest)
	}
	if !filepath.IsAbs(sourcePath) {
		return fmt.Errorf("%w: source_path must be absolute for EAS builds", ErrMobileBuildInvalidRequest)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("%w: source_path is not readable", ErrMobileBuildInvalidRequest)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: source_path must be a directory", ErrMobileBuildInvalidRequest)
	}
	if _, err := os.Stat(filepath.Join(sourcePath, "package.json")); err != nil {
		return fmt.Errorf("%w: source_path must contain package.json", ErrMobileBuildInvalidRequest)
	}
	if _, err := os.Stat(filepath.Join(sourcePath, "eas.json")); err != nil {
		return fmt.Errorf("%w: source_path must contain eas.json", ErrMobileBuildInvalidRequest)
	}
	if !sourceContainsExpoAppConfig(sourcePath) {
		return fmt.Errorf("%w: source_path must contain app.config.ts, app.config.js, app.json, or app.json5", ErrMobileBuildInvalidRequest)
	}
	return nil
}

func (p *EASBuildProvider) CreateBuild(ctx context.Context, req MobileBuildRequest) (MobileBuildProviderResult, error) {
	if p == nil || p.runner == nil {
		return MobileBuildProviderResult{}, ErrMobileBuildProviderMissing
	}
	if err := p.ValidateBuildRequest(req); err != nil {
		return MobileBuildProviderResult{}, err
	}
	easToken, err := p.resolveEASToken(ctx, req)
	if err != nil {
		return MobileBuildProviderResult{}, err
	}
	if req.DryRun {
		return MobileBuildProviderResult{
			Status: MobileBuildQueued,
			Logs: []MobileBuildLogLine{{
				Level:   "info",
				Message: "EAS build dry run validated source path, credentials, and command construction without invoking the provider.",
			}},
		}, nil
	}

	runCtx := ctx
	cancel := func() {}
	if p.timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, p.timeout)
	}
	defer cancel()

	args := []string{
		"build",
		"--platform", string(req.Platform),
		"--profile", string(req.Profile),
		"--non-interactive",
		"--no-wait",
		"--json",
	}
	output, err := p.runner.Run(runCtx, EASCommand{
		CLIPath: p.cliPath,
		WorkDir: req.SourcePath,
		Args:    args,
		Env: []string{
			"EXPO_TOKEN=" + easToken,
			"EAS_TOKEN=" + easToken,
			"EAS_NO_VCS=1",
			"CI=1",
		},
	})
	if err != nil {
		return MobileBuildProviderResult{}, fmt.Errorf("%w: %s", ErrMobileBuildProviderFailed, RedactMobileBuildSecrets(output+" "+err.Error()))
	}
	result := parseEASBuildOutput(output)
	result.Logs = append([]MobileBuildLogLine{{
		Level:   "info",
		Message: "EAS build queued with --no-wait. Use provider build ID for follow-up polling.",
	}}, result.Logs...)
	if result.Status == "" {
		result.Status = MobileBuildBuilding
	}
	return result, nil
}

func (p *EASBuildProvider) RefreshBuild(ctx context.Context, job MobileBuildJob) (MobileBuildProviderResult, error) {
	if p == nil || p.runner == nil {
		return MobileBuildProviderResult{}, ErrMobileBuildProviderMissing
	}
	providerBuildID := strings.TrimSpace(job.ProviderBuildID)
	if providerBuildID == "" {
		return MobileBuildProviderResult{}, fmt.Errorf("%w: provider_build_id is required for EAS build refresh", ErrMobileBuildInvalidRequest)
	}
	easToken, err := p.resolveEASToken(ctx, MobileBuildRequest{ProjectID: job.ProjectID, UserID: job.UserID})
	if err != nil {
		return MobileBuildProviderResult{}, err
	}

	runCtx := ctx
	cancel := func() {}
	if p.timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, p.timeout)
	}
	defer cancel()

	output, err := p.runner.Run(runCtx, EASCommand{
		CLIPath: p.cliPath,
		Args: []string{
			"build:view",
			providerBuildID,
			"--json",
		},
		Env: []string{
			"EXPO_TOKEN=" + easToken,
			"EAS_TOKEN=" + easToken,
			"CI=1",
		},
	})
	if err != nil {
		return MobileBuildProviderResult{}, fmt.Errorf("%w: %s", ErrMobileBuildProviderFailed, RedactMobileBuildSecrets(output+" "+err.Error()))
	}
	result := parseEASBuildOutput(output)
	if result.ProviderBuildID == "" {
		result.ProviderBuildID = providerBuildID
	}
	result.Logs = append([]MobileBuildLogLine{{
		Level:   "info",
		Message: "EAS build status refreshed from provider.",
	}}, result.Logs...)
	return result, nil
}

func (p *EASBuildProvider) resolveEASToken(ctx context.Context, req MobileBuildRequest) (string, error) {
	if p.credentials == nil {
		return "", fmt.Errorf("%w: EAS credential resolver is not configured", ErrMobileCredentialInvalidPayload)
	}
	values, err := p.credentials.ResolveCredentialValues(ctx, req.UserID, req.ProjectID, MobileCredentialEASToken)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(values["token"])
	if token == "" {
		return "", fmt.Errorf("%w: EAS token is empty", ErrMobileCredentialInvalidPayload)
	}
	return token, nil
}

func sourceContainsExpoAppConfig(sourcePath string) bool {
	for _, name := range []string{"app.config.ts", "app.config.js", "app.json", "app.json5"} {
		if _, err := os.Stat(filepath.Join(sourcePath, name)); err == nil {
			return true
		}
	}
	return false
}

func parseEASBuildOutput(output string) MobileBuildProviderResult {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return MobileBuildProviderResult{}
	}
	decoded, ok := decodeFirstJSONValue(trimmed)
	if !ok {
		return MobileBuildProviderResult{
			Logs: []MobileBuildLogLine{{Level: "info", Message: RedactMobileBuildSecrets(trimmed)}},
		}
	}
	if items, ok := decoded.([]any); ok && len(items) > 0 {
		decoded = items[0]
	}
	object, ok := decoded.(map[string]any)
	if !ok {
		return MobileBuildProviderResult{}
	}
	return MobileBuildProviderResult{
		ProviderBuildID: firstString(object, "id", "buildId", "build_id"),
		Status:          mapEASStatus(firstString(object, "status", "state")),
		ArtifactURL:     easArtifactURL(object),
		Logs:            []MobileBuildLogLine{{Level: "info", Message: RedactMobileBuildSecrets(trimmed)}},
	}
}

func decodeFirstJSONValue(output string) (any, bool) {
	startObject := strings.Index(output, "{")
	startArray := strings.Index(output, "[")
	start := -1
	switch {
	case startObject >= 0 && startArray >= 0:
		if startObject < startArray {
			start = startObject
		} else {
			start = startArray
		}
	case startObject >= 0:
		start = startObject
	case startArray >= 0:
		start = startArray
	}
	if start < 0 {
		return nil, false
	}
	var decoded any
	decoder := json.NewDecoder(strings.NewReader(output[start:]))
	if err := decoder.Decode(&decoded); err != nil {
		return nil, false
	}
	return decoded, true
}

func firstString(object map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := object[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func easArtifactURL(object map[string]any) string {
	if value := firstString(object, "artifactUrl", "artifact_url", "artifactsUrl", "artifacts_url"); value != "" {
		return value
	}
	artifacts, ok := object["artifacts"].(map[string]any)
	if !ok {
		return ""
	}
	return firstString(artifacts, "buildUrl", "build_url", "applicationArchiveUrl", "application_archive_url")
}

func mapEASStatus(status string) MobileBuildStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "finished", "succeeded", "success", "done":
		return MobileBuildSucceeded
	case "errored", "failed", "error":
		return MobileBuildFailed
	case "canceled", "cancelled":
		return MobileBuildCanceled
	case "queued", "new", "in_queue", "in-queue":
		return MobileBuildQueued
	case "in_progress", "in-progress", "building", "pending":
		return MobileBuildBuilding
	default:
		return ""
	}
}
