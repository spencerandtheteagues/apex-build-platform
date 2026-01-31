// Package bundler - esbuild wrapper implementation
package bundler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ESBuildBundler implements the Bundler interface using esbuild
type ESBuildBundler struct {
	// esbuildPath is the path to the esbuild executable
	esbuildPath string
	// version is the cached esbuild version
	version string
	// available indicates if esbuild is available
	available bool
	// cache is the bundle cache
	cache *BundleCache
	// mu protects the bundler state
	mu sync.RWMutex
	// tempDir is the base temp directory for bundle operations
	tempDir string
	// supportedFrameworks lists frameworks this bundler supports
	supportedFrameworks map[string]bool
}

// NewESBuildBundler creates a new esbuild-based bundler
func NewESBuildBundler(cache *BundleCache) *ESBuildBundler {
	bundler := &ESBuildBundler{
		cache:   cache,
		tempDir: os.TempDir(),
		supportedFrameworks: map[string]bool{
			"react":   true,
			"vue":     true,
			"vanilla": true,
			"preact":  true,
			"svelte":  false, // Needs separate plugin
			"solid":   true,
		},
	}
	bundler.detectEsbuild()
	return bundler
}

// detectEsbuild finds and validates the esbuild installation
func (b *ESBuildBundler) detectEsbuild() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Try multiple ways to find esbuild
	paths := []string{
		"esbuild",                                    // In PATH
		"npx esbuild",                               // Via npx
		"./node_modules/.bin/esbuild",              // Local node_modules
		"/usr/local/bin/esbuild",                   // Common global location
	}

	for _, path := range paths {
		var cmd *exec.Cmd
		if strings.Contains(path, " ") {
			parts := strings.Split(path, " ")
			cmd = exec.Command(parts[0], append(parts[1:], "--version")...)
		} else {
			cmd = exec.Command(path, "--version")
		}

		output, err := cmd.Output()
		if err == nil {
			version := strings.TrimSpace(string(output))
			b.esbuildPath = path
			b.version = version
			b.available = true
			log.Printf("[bundler] Found esbuild at %s (version %s)", path, version)
			return
		}
	}

	log.Printf("[bundler] Warning: esbuild not found. Bundling will be unavailable.")
	b.available = false
}

// IsAvailable returns true if esbuild is installed
func (b *ESBuildBundler) IsAvailable() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.available
}

// GetVersion returns the esbuild version
func (b *ESBuildBundler) GetVersion() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.version
}

// SupportsFramework returns true if the framework is supported
func (b *ESBuildBundler) SupportsFramework(framework string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	supported, exists := b.supportedFrameworks[strings.ToLower(framework)]
	return exists && supported
}

// Bundle performs the bundling operation
func (b *ESBuildBundler) Bundle(ctx context.Context, projectID uint, config BundleConfig) (*BundleResult, error) {
	if !b.IsAvailable() {
		return &BundleResult{
			Success: false,
			Errors: []BundleError{{
				Message: "esbuild is not available. Please install esbuild: npm install -g esbuild",
			}},
		}, nil
	}

	start := time.Now()

	// Check cache first if available
	if b.cache != nil {
		fileHash := config.ProjectPath // In real usage, compute from files
		cacheKey := ComputeCacheKey(projectID, config, fileHash)
		if cached := b.cache.Get(cacheKey); cached != nil {
			log.Printf("[bundler] Cache hit for project %d", projectID)
			return cached, nil
		}
	}

	// Create temp directory for this bundle operation
	bundleDir, err := os.MkdirTemp(b.tempDir, fmt.Sprintf("apex-bundle-%d-", projectID))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(bundleDir)

	// Build esbuild arguments
	args, err := b.buildArgs(config, bundleDir)
	if err != nil {
		return nil, fmt.Errorf("failed to build esbuild arguments: %w", err)
	}

	// Run esbuild
	result, err := b.runEsbuild(ctx, config.ProjectPath, args)
	if err != nil {
		return nil, fmt.Errorf("esbuild execution failed: %w", err)
	}

	result.Duration = time.Since(start)

	// Read output files
	if result.Success {
		if err := b.readOutputFiles(bundleDir, result); err != nil {
			log.Printf("[bundler] Warning: failed to read some output files: %v", err)
		}

		// Compute output hash
		hasher := make([]byte, 0)
		hasher = append(hasher, result.OutputJS...)
		hasher = append(hasher, result.OutputCSS...)
		result.Hash = ComputeFileHash(map[string]string{"output": string(hasher)})

		// Cache the result
		if b.cache != nil {
			fileHash := config.ProjectPath
			cacheKey := ComputeCacheKey(projectID, config, fileHash)
			b.cache.Set(cacheKey, result)
		}
	}

	log.Printf("[bundler] Bundle completed for project %d in %v (success: %v, warnings: %d, errors: %d)",
		projectID, result.Duration, result.Success, len(result.Warnings), len(result.Errors))

	return result, nil
}

// buildArgs constructs esbuild command line arguments
func (b *ESBuildBundler) buildArgs(config BundleConfig, outputDir string) ([]string, error) {
	args := []string{
		config.EntryPoint,
		"--bundle",
		fmt.Sprintf("--outdir=%s", outputDir),
	}

	// Output format
	switch strings.ToLower(config.Format) {
	case "esm", "es6":
		args = append(args, "--format=esm")
	case "iife":
		args = append(args, "--format=iife")
	case "cjs", "commonjs":
		args = append(args, "--format=cjs")
	default:
		args = append(args, "--format=esm")
	}

	// Minification
	if config.Minify {
		args = append(args, "--minify")
	}

	// Source maps
	if config.SourceMap {
		args = append(args, "--sourcemap=inline")
	}

	// Target
	if len(config.Target) > 0 {
		args = append(args, fmt.Sprintf("--target=%s", strings.Join(config.Target, ",")))
	}

	// Framework-specific settings
	switch strings.ToLower(config.Framework) {
	case "react":
		args = append(args, "--jsx=automatic")
		if config.JSXImportSource != "" {
			args = append(args, fmt.Sprintf("--jsx-import-source=%s", config.JSXImportSource))
		} else {
			args = append(args, "--jsx-import-source=react")
		}
	case "preact":
		args = append(args, "--jsx=automatic", "--jsx-import-source=preact")
	case "solid":
		args = append(args, "--jsx=preserve") // Solid uses its own JSX transform
	case "vue":
		// Vue SFCs need plugin support; for now we handle .js/.ts files
		args = append(args, "--jsx=automatic")
	}

	// Custom JSX settings (override framework defaults)
	if config.JSXFactory != "" {
		args = append(args, fmt.Sprintf("--jsx-factory=%s", config.JSXFactory))
	}
	if config.JSXFragment != "" {
		args = append(args, fmt.Sprintf("--jsx-fragment=%s", config.JSXFragment))
	}

	// Define constants
	for key, value := range config.Define {
		args = append(args, fmt.Sprintf("--define:%s=%s", key, value))
	}

	// Aliases
	for from, to := range config.Alias {
		args = append(args, fmt.Sprintf("--alias:%s=%s", from, to))
	}

	// External dependencies
	for _, dep := range config.ExternalDeps {
		args = append(args, fmt.Sprintf("--external:%s", dep))
	}

	// Loaders
	for ext, loader := range config.Loader {
		args = append(args, fmt.Sprintf("--loader:%s=%s", ext, loader))
	}

	// Tree shaking
	if config.TreeShaking {
		args = append(args, "--tree-shaking=true")
	}

	// Code splitting
	if config.Splitting && (strings.ToLower(config.Format) == "esm" || strings.ToLower(config.Format) == "es6") {
		args = append(args, "--splitting")
	}

	// Public path
	if config.PublicPath != "" {
		args = append(args, fmt.Sprintf("--public-path=%s", config.PublicPath))
	}

	// Always output metafile for analysis
	args = append(args, fmt.Sprintf("--metafile=%s/meta.json", outputDir))

	// Log level for capturing warnings
	args = append(args, "--log-level=warning")

	return args, nil
}

// runEsbuild executes the esbuild command
func (b *ESBuildBundler) runEsbuild(ctx context.Context, workDir string, args []string) (*BundleResult, error) {
	result := &BundleResult{
		Warnings: make([]string, 0),
		Errors:   make([]BundleError, 0),
	}

	var cmd *exec.Cmd
	if strings.Contains(b.esbuildPath, " ") {
		parts := strings.Split(b.esbuildPath, " ")
		cmd = exec.CommandContext(ctx, parts[0], append(parts[1:], args...)...)
	} else {
		cmd = exec.CommandContext(ctx, b.esbuildPath, args...)
	}

	cmd.Dir = workDir

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set environment
	cmd.Env = append(os.Environ(),
		"NODE_ENV=development",
		"FORCE_COLOR=0", // Disable color output for easier parsing
	)

	err := cmd.Run()

	// Parse stderr for warnings and errors
	b.parseOutput(stderr.String(), result)
	b.parseOutput(stdout.String(), result)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Errors = append(result.Errors, BundleError{
				Message: "Bundle operation timed out",
			})
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			// Non-zero exit code means build failed
			if len(result.Errors) == 0 {
				result.Errors = append(result.Errors, BundleError{
					Message: fmt.Sprintf("esbuild exited with code %d: %s", exitErr.ExitCode(), stderr.String()),
				})
			}
		} else {
			result.Errors = append(result.Errors, BundleError{
				Message: fmt.Sprintf("Failed to run esbuild: %v", err),
			})
		}
		result.Success = false
	} else {
		result.Success = true
	}

	return result, nil
}

// parseOutput parses esbuild output for warnings and errors
func (b *ESBuildBundler) parseOutput(output string, result *BundleResult) {
	if output == "" {
		return
	}

	// esbuild error/warning format:
	// X [ERROR] Message
	//     file.ts:line:col:
	//       code line
	//       ^
	//
	// or warning:
	// ! [WARNING] Message

	scanner := bufio.NewScanner(strings.NewReader(output))
	errorPattern := regexp.MustCompile(`^[X>]\s*\[ERROR\]\s*(.+)$`)
	warningPattern := regexp.MustCompile(`^[!>]\s*\[WARNING\]\s*(.+)$`)
	locationPattern := regexp.MustCompile(`^\s+(.+?):(\d+):(\d+):?\s*$`)

	var currentError *BundleError

	for scanner.Scan() {
		line := scanner.Text()

		if matches := errorPattern.FindStringSubmatch(line); matches != nil {
			if currentError != nil {
				result.Errors = append(result.Errors, *currentError)
			}
			currentError = &BundleError{
				Message: strings.TrimSpace(matches[1]),
			}
		} else if matches := warningPattern.FindStringSubmatch(line); matches != nil {
			result.Warnings = append(result.Warnings, strings.TrimSpace(matches[1]))
		} else if currentError != nil {
			if matches := locationPattern.FindStringSubmatch(line); matches != nil {
				currentError.File = matches[1]
				fmt.Sscanf(matches[2], "%d", &currentError.Line)
				fmt.Sscanf(matches[3], "%d", &currentError.Column)
			} else if strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "^") {
				// Capture the source line
				if currentError.Text == "" {
					currentError.Text = strings.TrimSpace(line)
				}
			}
		}
	}

	if currentError != nil {
		result.Errors = append(result.Errors, *currentError)
	}
}

// readOutputFiles reads the generated output files
func (b *ESBuildBundler) readOutputFiles(outputDir string, result *BundleResult) error {
	// Read JavaScript output
	jsFiles, err := filepath.Glob(filepath.Join(outputDir, "*.js"))
	if err == nil && len(jsFiles) > 0 {
		// Concatenate all JS files (usually just one)
		var jsContent bytes.Buffer
		for _, f := range jsFiles {
			content, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			jsContent.Write(content)
			jsContent.WriteString("\n")
		}
		result.OutputJS = jsContent.Bytes()
	}

	// Read CSS output
	cssFiles, err := filepath.Glob(filepath.Join(outputDir, "*.css"))
	if err == nil && len(cssFiles) > 0 {
		var cssContent bytes.Buffer
		for _, f := range cssFiles {
			content, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			cssContent.Write(content)
			cssContent.WriteString("\n")
		}
		result.OutputCSS = cssContent.Bytes()
	}

	// Read metafile
	metaPath := filepath.Join(outputDir, "meta.json")
	if metaContent, err := os.ReadFile(metaPath); err == nil {
		var metafile BundleMetafile
		if json.Unmarshal(metaContent, &metafile) == nil {
			result.Metafile = &metafile
		}
	}

	return nil
}

// BundleFromFiles bundles files directly provided in memory
func (b *ESBuildBundler) BundleFromFiles(ctx context.Context, projectID uint, files ProjectFiles, config BundleConfig) (*BundleResult, error) {
	if !b.IsAvailable() {
		return &BundleResult{
			Success: false,
			Errors: []BundleError{{
				Message: "esbuild is not available. Please install esbuild: npm install -g esbuild",
			}},
		}, nil
	}

	start := time.Now()

	// Compute file hash for caching
	fileHash := ComputeFileHash(files.Files)

	// Check cache
	if b.cache != nil {
		cacheKey := ComputeCacheKey(projectID, config, fileHash)
		if cached := b.cache.Get(cacheKey); cached != nil {
			log.Printf("[bundler] Cache hit for project %d (hash: %s)", projectID, fileHash[:8])
			return cached, nil
		}
	}

	// Create temp directory and write files
	bundleDir, err := os.MkdirTemp(b.tempDir, fmt.Sprintf("apex-bundle-%d-", projectID))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(bundleDir)

	// Write all files to temp directory
	for path, content := range files.Files {
		fullPath := filepath.Join(bundleDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", path, err)
		}
	}

	// Create output directory
	outputDir := filepath.Join(bundleDir, ".apex-output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Update config to use the temp directory
	config.ProjectPath = bundleDir

	// Build args with correct entry point
	args, err := b.buildArgs(config, outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to build esbuild arguments: %w", err)
	}

	// Run esbuild
	result, err := b.runEsbuild(ctx, bundleDir, args)
	if err != nil {
		return nil, fmt.Errorf("esbuild execution failed: %w", err)
	}

	result.Duration = time.Since(start)

	// Read output files
	if result.Success {
		if err := b.readOutputFiles(outputDir, result); err != nil {
			log.Printf("[bundler] Warning: failed to read some output files: %v", err)
		}

		// Compute output hash
		hasher := make([]byte, 0)
		hasher = append(hasher, result.OutputJS...)
		hasher = append(hasher, result.OutputCSS...)
		result.Hash = ComputeFileHash(map[string]string{"output": string(hasher)})

		// Cache the result
		if b.cache != nil {
			cacheKey := ComputeCacheKey(projectID, config, fileHash)
			b.cache.Set(cacheKey, result)
		}
	}

	log.Printf("[bundler] Bundle from files completed for project %d in %v (success: %v, js: %d bytes, css: %d bytes)",
		projectID, result.Duration, result.Success, len(result.OutputJS), len(result.OutputCSS))

	return result, nil
}

// GenerateHTML generates an HTML file that loads the bundled JavaScript and CSS
func GenerateHTML(result *BundleResult, config BundleConfig) string {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>APEX Preview</title>
`)

	// Add CSS inline if present
	if len(result.OutputCSS) > 0 {
		sb.WriteString("  <style>\n")
		sb.Write(result.OutputCSS)
		sb.WriteString("\n  </style>\n")
	}

	// Framework-specific dependencies from CDN
	switch strings.ToLower(config.Framework) {
	case "react":
		sb.WriteString(`  <script crossorigin src="https://unpkg.com/react@18/umd/react.development.js"></script>
  <script crossorigin src="https://unpkg.com/react-dom@18/umd/react-dom.development.js"></script>
`)
	case "vue":
		sb.WriteString(`  <script src="https://unpkg.com/vue@3/dist/vue.global.js"></script>
`)
	case "preact":
		sb.WriteString(`  <script src="https://unpkg.com/preact@10/dist/preact.umd.js"></script>
`)
	}

	sb.WriteString(`</head>
<body>
  <div id="root"></div>
  <div id="app"></div>
`)

	// Add bundled JavaScript
	if len(result.OutputJS) > 0 {
		if config.Format == "esm" {
			sb.WriteString(`  <script type="module">
`)
		} else {
			sb.WriteString(`  <script>
`)
		}
		sb.Write(result.OutputJS)
		sb.WriteString(`
  </script>
`)
	}

	sb.WriteString(`</body>
</html>`)

	return sb.String()
}

// StreamBundle streams the bundled output to a writer (useful for large bundles)
func (b *ESBuildBundler) StreamBundle(ctx context.Context, projectID uint, config BundleConfig, w io.Writer) error {
	result, err := b.Bundle(ctx, projectID, config)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("bundle failed: %v", result.Errors)
	}

	_, err = w.Write(result.OutputJS)
	return err
}

// ValidateConfig validates a bundle configuration
func ValidateConfig(config BundleConfig) []string {
	var errors []string

	if config.EntryPoint == "" {
		errors = append(errors, "entry_point is required")
	}

	validFormats := map[string]bool{"esm": true, "es6": true, "iife": true, "cjs": true, "commonjs": true}
	if config.Format != "" && !validFormats[strings.ToLower(config.Format)] {
		errors = append(errors, fmt.Sprintf("invalid format '%s': must be one of esm, iife, cjs", config.Format))
	}

	validFrameworks := map[string]bool{"react": true, "vue": true, "vanilla": true, "preact": true, "solid": true}
	if config.Framework != "" && !validFrameworks[strings.ToLower(config.Framework)] {
		errors = append(errors, fmt.Sprintf("invalid framework '%s': must be one of react, vue, vanilla, preact, solid", config.Framework))
	}

	return errors
}
