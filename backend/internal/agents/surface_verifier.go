package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type surfaceDeterministicResult struct {
	Ran      bool
	Checks   []string
	Warnings []string
	Errors   []string
}

func (am *AgentManager) runSurfaceDeterministicChecks(build *Build, task *Task, candidate *taskGenerationCandidate) surfaceDeterministicResult {
	result := surfaceDeterministicResult{}
	if am == nil || build == nil || task == nil || candidate == nil || candidate.Output == nil {
		return result
	}

	baseFiles := am.collectGeneratedFiles(build)
	mergedFiles, _ := cvApplyTaskOutputToGeneratedFiles(baseFiles, candidate.Output)
	if len(mergedFiles) == 0 {
		return result
	}

	triage := candidate.Triage
	if triage.TaskShape == "" {
		triage = triageTaskForWaterfall(task)
	}

	switch triage.TaskShape {
	case TaskShapeFrontendPatch:
		return runFrontendDeterministicChecks(mergedFiles)
	case TaskShapeBackendPatch, TaskShapeSchema, TaskShapeIntegration:
		if task.Type == TaskDeploy {
			return runConfigManifestSanityChecks(candidate.Output)
		}
		return runBackendDeterministicChecks(mergedFiles)
	default:
		return result
	}
}

func runFrontendDeterministicChecks(files []GeneratedFile) surfaceDeterministicResult {
	result := surfaceDeterministicResult{}
	pkgPath, pkgScripts := findPackageJSONWithScripts(files, []string{"lint", "typecheck", "build"})
	if pkgPath == "" {
		return result
	}
	if _, err := exec.LookPath("npm"); err != nil {
		result.Warnings = append(result.Warnings, "deterministic check skipped: npm is not installed")
		return result
	}

	workDir, cleanup, err := materializeDeterministicWorkspace(files)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("deterministic check setup failed: %v", err))
		return result
	}
	defer cleanup()

	baseDir := filepath.Dir(filepath.Join(workDir, filepath.FromSlash(pkgPath)))
	if ok, reason := installDeterministicNodeDependencies(baseDir); !ok {
		result.Checks = append(result.Checks, "frontend:dependency_bootstrap")
		result.Warnings = append(result.Warnings, reason)
		return result
	}

	for _, script := range []string{"lint", "typecheck", "build"} {
		if _, ok := pkgScripts[script]; !ok {
			continue
		}
		result.Ran = true
		result.Checks = append(result.Checks, "frontend:"+script)
		output, err := cvRunCommand(context.Background(), baseDir, 45*time.Second, "npm", "run", "--silent", script)
		if err != nil {
			if looksLikeInfraCommandFailure(output, err) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("frontend %s inconclusive: %v", script, err))
				continue
			}
			result.Errors = append(result.Errors, fmt.Sprintf("frontend %s failed: %v", script, err))
		}
	}

	return result
}

func runBackendDeterministicChecks(files []GeneratedFile) surfaceDeterministicResult {
	result := surfaceDeterministicResult{}
	if goModPath := findGeneratedPath(files, "go.mod"); goModPath != "" {
		if _, err := exec.LookPath("go"); err != nil {
			result.Warnings = append(result.Warnings, "deterministic check skipped: go is not installed")
			return result
		}
		workDir, cleanup, err := materializeDeterministicWorkspace(files)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("deterministic backend setup failed: %v", err))
			return result
		}
		defer cleanup()

		baseDir := filepath.Dir(filepath.Join(workDir, filepath.FromSlash(goModPath)))
		result.Ran = true
		result.Checks = append(result.Checks, "backend:go_build")
		output, err := cvRunCommand(context.Background(), baseDir, 60*time.Second, "go", "build", "./...")
		if err != nil {
			if looksLikeInfraCommandFailure(output, err) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("backend build inconclusive: %v", err))
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("backend build failed: %v", err))
			}
		}
		return result
	}

	pkgPath, _ := findPackageJSONWithScripts(files, []string{"build"})
	if pkgPath == "" {
		return result
	}
	if _, err := exec.LookPath("npm"); err != nil {
		result.Warnings = append(result.Warnings, "deterministic check skipped: npm is not installed")
		return result
	}
	workDir, cleanup, err := materializeDeterministicWorkspace(files)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("deterministic backend setup failed: %v", err))
		return result
	}
	defer cleanup()

	baseDir := filepath.Dir(filepath.Join(workDir, filepath.FromSlash(pkgPath)))
	if ok, reason := installDeterministicNodeDependencies(baseDir); !ok {
		result.Checks = append(result.Checks, "backend:dependency_bootstrap")
		result.Warnings = append(result.Warnings, reason)
		return result
	}

	result.Ran = true
	result.Checks = append(result.Checks, "backend:build")
	output, err := cvRunCommand(context.Background(), baseDir, 45*time.Second, "npm", "run", "--silent", "build")
	if err != nil {
		if looksLikeInfraCommandFailure(output, err) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("backend build script inconclusive: %v", err))
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("backend build script failed: %v", err))
		}
	}
	return result
}

func materializeDeterministicWorkspace(files []GeneratedFile) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "apex-deterministic-check-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}
	if err := cvMaterializeFiles(files, tmpDir); err != nil {
		cleanup()
		return "", nil, err
	}
	return tmpDir, cleanup, nil
}

func findPackageJSONWithScripts(files []GeneratedFile, scripts []string) (string, map[string]string) {
	for _, candidate := range []string{"package.json", "frontend/package.json", "backend/package.json"} {
		content := generatedFileContent(files, candidate)
		if strings.TrimSpace(content) == "" {
			continue
		}
		parsed, err := parsePackageScripts(content)
		if err != nil {
			continue
		}
		for _, script := range scripts {
			if _, ok := parsed[script]; ok {
				return candidate, parsed
			}
		}
	}
	for _, file := range files {
		normalized := filepath.ToSlash(strings.TrimSpace(file.Path))
		if !strings.HasSuffix(normalized, "/package.json") {
			continue
		}
		parsed, err := parsePackageScripts(file.Content)
		if err != nil {
			continue
		}
		for _, script := range scripts {
			if _, ok := parsed[script]; ok {
				return normalized, parsed
			}
		}
	}
	return "", nil
}

func parsePackageScripts(content string) (map[string]string, error) {
	if !json.Valid([]byte(content)) {
		return nil, errors.New("package.json is invalid JSON")
	}
	var parsed struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, err
	}
	return parsed.Scripts, nil
}

func findGeneratedPath(files []GeneratedFile, path string) string {
	target := filepath.ToSlash(strings.TrimSpace(path))
	for _, file := range files {
		normalized := filepath.ToSlash(strings.TrimSpace(file.Path))
		if normalized == target || strings.HasSuffix(normalized, "/"+target) {
			return normalized
		}
	}
	return ""
}

func generatedFileContent(files []GeneratedFile, path string) string {
	target := filepath.ToSlash(strings.TrimSpace(path))
	for _, file := range files {
		normalized := filepath.ToSlash(strings.TrimSpace(file.Path))
		if normalized == target || strings.HasSuffix(normalized, "/"+target) {
			return file.Content
		}
	}
	return ""
}

func runConfigManifestSanityChecks(output *TaskOutput) surfaceDeterministicResult {
	result := surfaceDeterministicResult{}
	if output == nil {
		return result
	}
	for _, file := range output.Files {
		path := strings.ToLower(strings.TrimSpace(file.Path))
		content := strings.TrimSpace(file.Content)
		if path == "" || content == "" {
			continue
		}
		switch {
		case strings.HasSuffix(path, ".json"):
			result.Ran = true
			result.Checks = append(result.Checks, "config:json_sanity")
			if !json.Valid([]byte(content)) {
				result.Errors = append(result.Errors, fmt.Sprintf("%s is invalid JSON", file.Path))
			}
		case strings.HasSuffix(path, ".yaml"), strings.HasSuffix(path, ".yml"), strings.HasSuffix(path, "render.yaml"), strings.HasSuffix(path, "docker-compose.yml"), strings.HasSuffix(path, "docker-compose.yaml"):
			result.Ran = true
			result.Checks = append(result.Checks, "config:yaml_sanity")
			var parsed any
			if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s is invalid YAML: %v", file.Path, err))
			}
		}
	}
	return result
}

func installDeterministicNodeDependencies(workDir string) (bool, string) {
	lockPath := filepath.Join(workDir, "package-lock.json")
	yarnLock := filepath.Join(workDir, "yarn.lock")
	pnpmLock := filepath.Join(workDir, "pnpm-lock.yaml")

	args := []string{"install", "--silent", "--no-audit", "--no-fund"}
	if _, err := os.Stat(lockPath); err == nil {
		args = []string{"ci", "--silent", "--no-audit", "--no-fund"}
	}
	if _, err := os.Stat(yarnLock); err == nil {
		return false, "deterministic check skipped: yarn lockfile detected"
	}
	if _, err := os.Stat(pnpmLock); err == nil {
		return false, "deterministic check skipped: pnpm lockfile detected"
	}

	output, err := cvRunCommand(context.Background(), workDir, 90*time.Second, "npm", args...)
	if err != nil {
		return false, fmt.Sprintf("deterministic dependency bootstrap failed (%v)", firstMeaningfulLine(output, err.Error()))
	}
	return true, ""
}

func looksLikeInfraCommandFailure(output string, err error) bool {
	if err == nil {
		return false
	}
	raw := strings.ToLower(strings.TrimSpace(output + "\n" + err.Error()))
	infraHints := []string{
		"eai_again",
		"enotfound",
		"econnreset",
		"socket hang up",
		"network",
		"service unavailable",
		"timed out",
		"context deadline exceeded",
		"temporary failure",
		"permission denied",
		"signal: killed",
	}
	for _, hint := range infraHints {
		if strings.Contains(raw, hint) {
			return true
		}
	}
	return false
}

func firstMeaningfulLine(output string, fallback string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return strings.TrimSpace(fallback)
}
