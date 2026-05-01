// Package bundler - esbuild wrapper implementation
package bundler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
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
			"next":    true,
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
		"./node_modules/.bin/esbuild", // Local node_modules
		"esbuild",                     // In PATH
		"npx --yes esbuild",           // Via npx (may download)
		"/usr/local/bin/esbuild",      // Common global location
	}

	for _, path := range paths {
		probeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		var cmd *exec.Cmd
		if strings.Contains(path, " ") {
			parts := strings.Split(path, " ")
			cmd = exec.CommandContext(probeCtx, parts[0], append(parts[1:], "--version")...)
		} else {
			cmd = exec.CommandContext(probeCtx, path, "--version")
		}

		output, err := cmd.Output()
		cancel()
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
	applyPreviewRuntimeEnvDefines(&config)

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
	case "react", "next":
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
	applyPreviewRuntimeEnvDefines(&config)

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
	if err := b.prepareNextPreviewEntry(bundleDir, files, &config); err != nil {
		return nil, fmt.Errorf("failed to prepare Next.js preview entry: %w", err)
	}
	if err := b.prepareFrameworkShims(bundleDir, &config); err != nil {
		return nil, fmt.Errorf("failed to prepare framework shims: %w", err)
	}

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
		if compiledCSS, ok := b.compileTailwindPreviewCSS(ctx, bundleDir, files, result.OutputCSS); ok {
			result.OutputCSS = compiledCSS
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

func (b *ESBuildBundler) compileTailwindPreviewCSS(ctx context.Context, bundleDir string, files ProjectFiles, bundledCSS []byte) ([]byte, bool) {
	inputCSS := string(bundledCSS)
	if strings.TrimSpace(inputCSS) == "" || !containsTailwindDirective(inputCSS) {
		if css, found := findTailwindSourceCSS(files.Files); found {
			inputCSS = css
		}
	}
	if !containsTailwindDirective(inputCSS) {
		return nil, false
	}

	inputCSS = normalizeTailwindPreviewInput(inputCSS)
	inputPath := filepath.Join(bundleDir, ".apex-tailwind-input.css")
	outputPath := filepath.Join(bundleDir, ".apex-output", "__apex_tailwind.css")
	configPath := filepath.Join(bundleDir, ".apex-tailwind.config.cjs")
	if err := os.WriteFile(inputPath, []byte(inputCSS), 0644); err != nil {
		log.Printf("[bundler] Tailwind preview CSS skipped: failed to write input: %v", err)
		return nil, false
	}
	if err := os.WriteFile(configPath, []byte(previewTailwindConfig()), 0644); err != nil {
		log.Printf("[bundler] Tailwind preview CSS skipped: failed to write config: %v", err)
		return nil, false
	}

	args := []string{"-c", configPath, "-i", inputPath, "-o", outputPath}
	cmdName, cmdArgs, ok := resolveTailwindCommand(args...)
	if !ok {
		log.Printf("[bundler] Tailwind preview CSS skipped: tailwindcss CLI is unavailable")
		return nil, false
	}

	compileCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		compileCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(compileCtx, cmdName, cmdArgs...)
	cmd.Dir = bundleDir
	cmd.Env = append(os.Environ(), "NODE_ENV=development", "FORCE_COLOR=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[bundler] Tailwind preview CSS compile failed: %v: %s", err, strings.TrimSpace(string(output)))
		return nil, false
	}

	compiled, err := os.ReadFile(outputPath)
	if err != nil {
		log.Printf("[bundler] Tailwind preview CSS skipped: failed to read output: %v", err)
		return nil, false
	}
	if len(bytes.TrimSpace(compiled)) == 0 {
		log.Printf("[bundler] Tailwind preview CSS skipped: compiler produced empty CSS")
		return nil, false
	}
	log.Printf("[bundler] Tailwind preview CSS compiled (%d bytes)", len(compiled))
	return compiled, true
}

func resolveTailwindCommand(args ...string) (string, []string, bool) {
	if override := strings.TrimSpace(os.Getenv("APEX_TAILWIND_CLI")); override != "" {
		parts := strings.Fields(override)
		if len(parts) > 0 {
			return parts[0], append(parts[1:], args...), true
		}
	}
	if path, err := exec.LookPath("tailwindcss"); err == nil {
		return path, args, true
	}
	if path, err := exec.LookPath("npx"); err == nil {
		npxArgs := append([]string{"--yes", "tailwindcss@3.4.17"}, args...)
		return path, npxArgs, true
	}
	return "", nil, false
}

func containsTailwindDirective(css string) bool {
	lower := strings.ToLower(css)
	return strings.Contains(lower, "@tailwind ") ||
		strings.Contains(lower, "@import \"tailwindcss\"") ||
		strings.Contains(lower, "@import 'tailwindcss'")
}

func findTailwindSourceCSS(files map[string]string) (string, bool) {
	preferred := []string{
		"src/index.css",
		"src/App.css",
		"src/globals.css",
		"app/globals.css",
		"styles/globals.css",
		"index.css",
	}
	for _, path := range preferred {
		if css, ok := files[path]; ok && containsTailwindDirective(css) {
			return css, true
		}
	}
	for path, css := range files {
		if strings.EqualFold(filepath.Ext(path), ".css") && containsTailwindDirective(css) {
			return css, true
		}
	}
	return "", false
}

func normalizeTailwindPreviewInput(css string) string {
	css = strings.ReplaceAll(css, `@import "tailwindcss";`, "@tailwind base;\n@tailwind components;\n@tailwind utilities;")
	css = strings.ReplaceAll(css, `@import 'tailwindcss';`, "@tailwind base;\n@tailwind components;\n@tailwind utilities;")
	return css
}

func previewTailwindConfig() string {
	return `module.exports = {
  darkMode: ["class"],
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}"
  ],
  theme: {
    extend: {
      colors: {
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))"
        },
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))"
        },
        secondary: {
          DEFAULT: "hsl(var(--secondary))",
          foreground: "hsl(var(--secondary-foreground))"
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))"
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))"
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))"
        },
        popover: {
          DEFAULT: "hsl(var(--popover))",
          foreground: "hsl(var(--popover-foreground))"
        }
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)"
      }
    }
  },
  plugins: []
}
`
}

func (b *ESBuildBundler) prepareFrameworkShims(bundleDir string, config *BundleConfig) error {
	if config == nil {
		return nil
	}

	switch strings.ToLower(config.Framework) {
	case "react", "next":
		return b.prepareReactShims(bundleDir, config)
	default:
		return nil
	}
}

func (b *ESBuildBundler) prepareNextPreviewEntry(bundleDir string, files ProjectFiles, config *BundleConfig) error {
	if config == nil || strings.ToLower(strings.TrimSpace(config.Framework)) != "next" {
		return nil
	}
	entry := strings.TrimSpace(config.EntryPoint)
	if !isNextPreviewPageEntry(entry) {
		return nil
	}

	pageImport := "./" + strings.TrimSuffix(entry, filepath.Ext(entry))
	layoutImport := ""
	for _, candidate := range nextPreviewLayoutCandidates(entry) {
		if _, ok := files.Files[candidate]; ok {
			layoutImport = "./" + strings.TrimSuffix(candidate, filepath.Ext(candidate))
			break
		}
	}

	var source strings.Builder
	source.WriteString("import React from 'react';\n")
	source.WriteString("import { createRoot } from 'react-dom/client';\n")
	source.WriteString("import Page from '")
	source.WriteString(pageImport)
	source.WriteString("';\n")
	if layoutImport != "" {
		source.WriteString("import Layout from '")
		source.WriteString(layoutImport)
		source.WriteString("';\n")
	}
	source.WriteString("const root = document.getElementById('root') || document.body.appendChild(document.createElement('div'));\n")
	if layoutImport != "" {
		source.WriteString("createRoot(root).render(React.createElement(Layout, null, React.createElement(Page)));\n")
	} else {
		source.WriteString("createRoot(root).render(React.createElement(Page));\n")
	}

	entryPath := filepath.Join(bundleDir, ".apex-next-preview-entry.tsx")
	if err := os.WriteFile(entryPath, []byte(source.String()), 0644); err != nil {
		return err
	}
	config.EntryPoint = ".apex-next-preview-entry.tsx"
	config.Framework = "next"
	if config.JSXImportSource == "" {
		config.JSXImportSource = "react"
	}
	return nil
}

func isNextPreviewPageEntry(entry string) bool {
	switch filepath.ToSlash(strings.TrimSpace(entry)) {
	case "app/page.tsx", "app/page.ts", "app/page.jsx", "app/page.js",
		"src/app/page.tsx", "src/app/page.ts", "src/app/page.jsx", "src/app/page.js",
		"pages/index.tsx", "pages/index.ts", "pages/index.jsx", "pages/index.js":
		return true
	default:
		return false
	}
}

func nextPreviewLayoutCandidates(entry string) []string {
	entry = filepath.ToSlash(strings.TrimSpace(entry))
	if strings.HasPrefix(entry, "src/app/") {
		return []string{"src/app/layout.tsx", "src/app/layout.ts", "src/app/layout.jsx", "src/app/layout.js"}
	}
	if strings.HasPrefix(entry, "app/") {
		return []string{"app/layout.tsx", "app/layout.ts", "app/layout.jsx", "app/layout.js"}
	}
	return nil
}

func (b *ESBuildBundler) prepareReactShims(bundleDir string, config *BundleConfig) error {
	if config.Alias == nil {
		config.Alias = make(map[string]string)
	}

	shimDir := filepath.Join(bundleDir, ".apex-shims")
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		return err
	}

	shims := map[string]string{
		"react.js": `const React = globalThis.React;
if (!React) {
  throw new Error("React global not found. The preview CDN dependency did not load.");
}

export default React;
export const Children = React.Children;
export const Component = React.Component;
export const Fragment = React.Fragment;
export const PureComponent = React.PureComponent;
export const StrictMode = React.StrictMode;
export const Suspense = React.Suspense;
export const cloneElement = React.cloneElement;
export const createContext = React.createContext;
export const createElement = React.createElement;
export const createRef = React.createRef;
export const forwardRef = React.forwardRef;
export const isValidElement = React.isValidElement;
export const lazy = React.lazy;
export const memo = React.memo;
export const startTransition = React.startTransition;
export const useCallback = React.useCallback;
export const useContext = React.useContext;
export const useDebugValue = React.useDebugValue;
export const useDeferredValue = React.useDeferredValue;
export const useEffect = React.useEffect;
export const useId = React.useId;
export const useImperativeHandle = React.useImperativeHandle;
export const useInsertionEffect = React.useInsertionEffect;
export const useLayoutEffect = React.useLayoutEffect;
export const useMemo = React.useMemo;
export const useReducer = React.useReducer;
export const useRef = React.useRef;
export const useState = React.useState;
export const useSyncExternalStore = React.useSyncExternalStore;
export const useTransition = React.useTransition;
`,
		"react-dom.js": `const ReactDOM = globalThis.ReactDOM;
if (!ReactDOM) {
  throw new Error("ReactDOM global not found. The preview CDN dependency did not load.");
}

export default ReactDOM;
export const createPortal = ReactDOM.createPortal;
export const findDOMNode = ReactDOM.findDOMNode;
export const flushSync = ReactDOM.flushSync;
export const hydrate = ReactDOM.hydrate;
export const render = ReactDOM.render;
export const unmountComponentAtNode = ReactDOM.unmountComponentAtNode;
`,
		"react-dom-client.js": `import ReactDOM from "./react-dom.js";

export default ReactDOM;
export const createRoot = ReactDOM.createRoot ? ReactDOM.createRoot.bind(ReactDOM) : undefined;
export const hydrateRoot = ReactDOM.hydrateRoot ? ReactDOM.hydrateRoot.bind(ReactDOM) : undefined;
`,
		"react-jsx-runtime.js": `import React from "./react.js";

export const Fragment = React.Fragment;

function createElementWithChildren(type, props, key) {
  const nextProps = props || {};
  const { children, ...rest } = nextProps;
  if (key !== undefined) {
    rest.key = key;
  }
  if (children === undefined) {
    return React.createElement(type, rest);
  }
  if (Array.isArray(children)) {
    return React.createElement(type, rest, ...children);
  }
  return React.createElement(type, rest, children);
}

export function jsx(type, props, key) {
  return createElementWithChildren(type, props, key);
}

export const jsxs = jsx;
export const jsxDEV = jsx;
`,
		"next-link.js": `import React from "./react.js";
export default function Link(props) {
  const { href = "#", children, ...rest } = props || {};
  return React.createElement("a", { href: typeof href === "string" ? href : "#", ...rest }, children);
}
`,
		"next-image.js": `import React from "./react.js";
export default function Image(props) {
  const { src = "", alt = "", width, height, ...rest } = props || {};
  const normalizedSrc = typeof src === "string" ? src : (src && src.src) || "";
  return React.createElement("img", { src: normalizedSrc, alt, width, height, ...rest });
}
`,
		"next-navigation.js": `export function useRouter() {
  return { push(){}, replace(){}, back(){}, forward(){}, refresh(){}, prefetch(){ return Promise.resolve(); } };
}
export function usePathname() { return "/"; }
export function useSearchParams() { return new URLSearchParams(globalThis.location ? globalThis.location.search : ""); }
export function useParams() { return {}; }
export function redirect() {}
export function notFound() {}
`,
		"next-head.js": `import React from "./react.js";
export default function Head(props) {
  return React.createElement(React.Fragment, null, props && props.children);
}
`,
		"next-script.js": `export default function Script() { return null; }
`,
		"next-dynamic.js": `import React from "./react.js";
export default function dynamic(loader) {
  return function DynamicComponent(props) {
    const [Component, setComponent] = React.useState(null);
    React.useEffect(() => {
      Promise.resolve(typeof loader === "function" ? loader() : loader).then((mod) => {
        setComponent(() => mod && (mod.default || mod));
      });
    }, []);
    return Component ? React.createElement(Component, props) : null;
  };
}
`,
		"next-router.js": `export function useRouter() {
  return { pathname: "/", query: {}, asPath: "/", push(){}, replace(){}, back(){}, prefetch(){ return Promise.resolve(); } };
}
export default { useRouter };
`,
		"next-font-google.js": `function fontShim() { return { className: "", style: {}, variable: "" }; }
export const Inter = fontShim;
export const Roboto = fontShim;
export const Poppins = fontShim;
export const Montserrat = fontShim;
export const Manrope = fontShim;
export const Geist = fontShim;
export const Orbitron = fontShim;
export const Space_Grotesk = fontShim;
export const JetBrains_Mono = fontShim;
export default fontShim;
`,
	}

	for name, content := range shims {
		if err := os.WriteFile(filepath.Join(shimDir, name), []byte(content), 0644); err != nil {
			return err
		}
	}

	aliases := map[string]string{
		"react":                 "./.apex-shims/react.js",
		"react-dom":             "./.apex-shims/react-dom.js",
		"react-dom/client":      "./.apex-shims/react-dom-client.js",
		"react/jsx-runtime":     "./.apex-shims/react-jsx-runtime.js",
		"react/jsx-dev-runtime": "./.apex-shims/react-jsx-runtime.js",
		"next/link":             "./.apex-shims/next-link.js",
		"next/image":            "./.apex-shims/next-image.js",
		"next/navigation":       "./.apex-shims/next-navigation.js",
		"next/router":           "./.apex-shims/next-router.js",
		"next/head":             "./.apex-shims/next-head.js",
		"next/script":           "./.apex-shims/next-script.js",
		"next/dynamic":          "./.apex-shims/next-dynamic.js",
		"next/font/google":      "./.apex-shims/next-font-google.js",
	}

	for from, to := range aliases {
		if _, exists := config.Alias[from]; !exists {
			config.Alias[from] = to
		}
	}

	return nil
}

// GenerateHTML generates an HTML file that loads the bundled JavaScript and CSS
func GenerateHTML(result *BundleResult, config BundleConfig) string {
	var sb strings.Builder
	title := template.HTMLEscapeString(config.PreviewTitle())

	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>` + title + `</title>
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

	// Add import map for ESM dependencies so externally-resolved packages
	// (marked via --external) are loaded from CDN at runtime.
	if len(config.ExternalDeps) > 0 {
		// Exclude deps that already have local shims (react ecosystem)
		shimmed := map[string]bool{
			"react": true, "react-dom": true, "react-dom/client": true,
			"react/jsx-runtime": true, "react/jsx-dev-runtime": true,
		}
		var externals []string
		for _, dep := range config.ExternalDeps {
			if !shimmed[dep] {
				externals = append(externals, dep)
			}
		}
		if len(externals) > 0 {
			sb.WriteString(`  <script type="importmap">
  {
    "imports": {
`)
			for i, dep := range externals {
				cdnURL := cdnURLFor(dep)
				comma := ","
				if i == len(externals)-1 {
					comma = ""
				}
				sb.WriteString(fmt.Sprintf("      \"%s\": \"%s\"%s\n", dep, cdnURL, comma))
			}
			sb.WriteString(`    }
  }
  </script>
`)
		}
	}

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

// cdnURLFor maps a package name to a best-effort CDN URL for use in an import map.
// For React ecosystem packages it uses esm.sh so bare imports work at runtime.
func cdnURLFor(pkg string) string {
	switch pkg {
	case "react":
		return "https://esm.sh/react@18"
	case "react-dom":
		return "https://esm.sh/react-dom@18"
	case "react-dom/client":
		return "https://esm.sh/react-dom@18/client"
	case "react/jsx-runtime":
		return "https://esm.sh/react@18/jsx-runtime"
	case "lucide-react":
		return "https://esm.sh/lucide-react"
	case "clsx":
		return "https://esm.sh/clsx"
	case "tailwind-merge":
		return "https://esm.sh/tailwind-merge"
	case "tailwindcss-animate":
		return "https://esm.sh/tailwindcss-animate"
	}
	// Scoped packages and unknown packages fallback to esm.sh
	if strings.HasPrefix(pkg, "@") {
		return fmt.Sprintf("https://esm.sh/%s", pkg)
	}
	return fmt.Sprintf("https://esm.sh/%s", pkg)
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

	validFrameworks := map[string]bool{"react": true, "next": true, "vue": true, "vanilla": true, "preact": true, "solid": true}
	if config.Framework != "" && !validFrameworks[strings.ToLower(config.Framework)] {
		errors = append(errors, fmt.Sprintf("invalid framework '%s': must be one of react, next, vue, vanilla, preact, solid", config.Framework))
	}

	return errors
}
