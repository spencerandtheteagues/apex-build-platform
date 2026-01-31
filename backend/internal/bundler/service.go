// Package bundler - High-level bundler service for APEX.BUILD
package bundler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"

	"gorm.io/gorm"

	"apex-build/pkg/models"
)

// Service provides high-level bundling operations integrated with the database
type Service struct {
	db      *gorm.DB
	bundler *ESBuildBundler
	cache   *BundleCache
	mu      sync.RWMutex
}

// NewService creates a new bundler service
func NewService(db *gorm.DB) *Service {
	cache := NewBundleCache(DefaultCacheConfig())
	bundler := NewESBuildBundler(cache)

	service := &Service{
		db:      db,
		bundler: bundler,
		cache:   cache,
	}

	return service
}

// IsAvailable returns true if the bundler is available
func (s *Service) IsAvailable() bool {
	return s.bundler.IsAvailable()
}

// GetVersion returns the bundler version
func (s *Service) GetVersion() string {
	return s.bundler.GetVersion()
}

// BundleProject bundles a project from the database
func (s *Service) BundleProject(ctx context.Context, projectID uint, config BundleConfig) (*BundleResult, error) {
	// Load project files from database
	files, err := s.loadProjectFiles(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load project files: %w", err)
	}

	// Detect entry point if not specified
	if config.EntryPoint == "" {
		config.EntryPoint = s.detectEntryPoint(files)
	}

	// Detect framework if not specified
	if config.Framework == "" {
		config.Framework = s.detectFramework(files)
	}

	// Set sensible defaults based on framework
	config = s.applyFrameworkDefaults(config)

	log.Printf("[bundler-service] Bundling project %d with entry point %s (framework: %s)",
		projectID, config.EntryPoint, config.Framework)

	// Bundle the files
	return s.bundler.BundleFromFiles(ctx, projectID, *files, config)
}

// NeedsBundling determines if a project needs bundling
func (s *Service) NeedsBundling(ctx context.Context, projectID uint) (bool, string) {
	files, err := s.loadProjectFiles(ctx, projectID)
	if err != nil {
		return false, ""
	}

	// Check for TypeScript files
	for path := range files.Files {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".ts" || ext == ".tsx" {
			return true, "typescript"
		}
	}

	// Check for JSX files
	for path := range files.Files {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".jsx" {
			return true, "jsx"
		}
	}

	// Check for React/Vue in package.json
	if files.PackageJSON != nil {
		deps := make(map[string]bool)
		for dep := range files.PackageJSON.Dependencies {
			deps[dep] = true
		}
		for dep := range files.PackageJSON.DevDependencies {
			deps[dep] = true
		}

		if deps["react"] || deps["react-dom"] {
			return true, "react"
		}
		if deps["vue"] {
			return true, "vue"
		}
		if deps["preact"] {
			return true, "preact"
		}
		if deps["svelte"] {
			return true, "svelte"
		}
		if deps["solid-js"] {
			return true, "solid"
		}
	}

	// Check for ES module imports/exports in JS files
	for path, content := range files.Files {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".js" || ext == ".mjs" {
			if strings.Contains(content, "import ") || strings.Contains(content, "export ") {
				return true, "esm"
			}
		}
	}

	return false, ""
}

// InvalidateCache invalidates the cache for a project
func (s *Service) InvalidateCache(projectID uint) {
	s.cache.InvalidateByProjectID(projectID)
	log.Printf("[bundler-service] Cache invalidated for project %d", projectID)
}

// GetCacheStats returns cache statistics
func (s *Service) GetCacheStats() CacheStats {
	return s.cache.Stats()
}

// loadProjectFiles loads all files for a project from the database
func (s *Service) loadProjectFiles(ctx context.Context, projectID uint) (*ProjectFiles, error) {
	var files []models.File
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&files).Error; err != nil {
		return nil, err
	}

	result := &ProjectFiles{
		ProjectID: projectID,
		Files:     make(map[string]string, len(files)),
	}

	for _, file := range files {
		if file.Type == "file" {
			result.Files[file.Path] = file.Content

			// Parse package.json if found
			if file.Path == "package.json" || file.Name == "package.json" {
				var pkg PackageJSON
				if err := json.Unmarshal([]byte(file.Content), &pkg); err == nil {
					result.PackageJSON = &pkg
				}
			}
		}
	}

	return result, nil
}

// detectEntryPoint finds the most likely entry point
func (s *Service) detectEntryPoint(files *ProjectFiles) string {
	// Priority order for entry points
	entryPoints := []string{
		"src/index.tsx",
		"src/index.ts",
		"src/index.jsx",
		"src/index.js",
		"src/main.tsx",
		"src/main.ts",
		"src/main.jsx",
		"src/main.js",
		"src/App.tsx",
		"src/App.jsx",
		"index.tsx",
		"index.ts",
		"index.jsx",
		"index.js",
		"main.tsx",
		"main.ts",
		"main.jsx",
		"main.js",
	}

	for _, ep := range entryPoints {
		if _, exists := files.Files[ep]; exists {
			return ep
		}
	}

	// Check package.json main/module fields
	if files.PackageJSON != nil {
		if files.PackageJSON.Module != "" {
			if _, exists := files.Files[files.PackageJSON.Module]; exists {
				return files.PackageJSON.Module
			}
		}
		if files.PackageJSON.Main != "" {
			if _, exists := files.Files[files.PackageJSON.Main]; exists {
				return files.PackageJSON.Main
			}
		}
	}

	// Fall back to first JS/TS file found
	for path := range files.Files {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".tsx" || ext == ".ts" || ext == ".jsx" || ext == ".js" {
			return path
		}
	}

	return "src/index.js" // Default fallback
}

// detectFramework detects the framework from package.json
func (s *Service) detectFramework(files *ProjectFiles) string {
	if files.PackageJSON == nil {
		return "vanilla"
	}

	deps := make(map[string]bool)
	for dep := range files.PackageJSON.Dependencies {
		deps[dep] = true
	}
	for dep := range files.PackageJSON.DevDependencies {
		deps[dep] = true
	}

	// Check in priority order
	if deps["react"] || deps["react-dom"] {
		return "react"
	}
	if deps["vue"] {
		return "vue"
	}
	if deps["preact"] {
		return "preact"
	}
	if deps["solid-js"] {
		return "solid"
	}
	if deps["svelte"] {
		return "svelte"
	}

	return "vanilla"
}

// applyFrameworkDefaults applies sensible defaults based on framework
func (s *Service) applyFrameworkDefaults(config BundleConfig) BundleConfig {
	// Set defaults if not specified
	if config.Format == "" {
		config.Format = "esm"
	}
	if len(config.Target) == 0 {
		config.Target = []string{"es2020"}
	}
	if config.Loader == nil {
		config.Loader = map[string]string{
			".png":   "dataurl",
			".jpg":   "dataurl",
			".jpeg":  "dataurl",
			".gif":   "dataurl",
			".svg":   "dataurl",
			".woff":  "dataurl",
			".woff2": "dataurl",
			".ttf":   "dataurl",
			".eot":   "dataurl",
		}
	}

	// Framework-specific defaults
	switch strings.ToLower(config.Framework) {
	case "react":
		if config.Define == nil {
			config.Define = make(map[string]string)
		}
		config.Define["process.env.NODE_ENV"] = `"development"`
		if config.JSXImportSource == "" {
			config.JSXImportSource = "react"
		}

	case "vue":
		if config.Define == nil {
			config.Define = make(map[string]string)
		}
		config.Define["process.env.NODE_ENV"] = `"development"`
		config.Define["__VUE_OPTIONS_API__"] = "true"
		config.Define["__VUE_PROD_DEVTOOLS__"] = "false"

	case "preact":
		if config.Define == nil {
			config.Define = make(map[string]string)
		}
		config.Define["process.env.NODE_ENV"] = `"development"`
		if config.JSXImportSource == "" {
			config.JSXImportSource = "preact"
		}

	case "solid":
		if config.Define == nil {
			config.Define = make(map[string]string)
		}
		config.Define["process.env.NODE_ENV"] = `"development"`
	}

	return config
}

// GeneratePreviewHTML generates an HTML file for preview with the bundled code
func (s *Service) GeneratePreviewHTML(result *BundleResult, config BundleConfig, originalHTML string) string {
	if !result.Success {
		return s.generateErrorHTML(result.Errors)
	}

	// If there's original HTML, inject the bundle into it
	if originalHTML != "" {
		return s.injectBundleIntoHTML(originalHTML, result, config)
	}

	// Generate new HTML
	return GenerateHTML(result, config)
}

// injectBundleIntoHTML injects the bundled JS/CSS into existing HTML
func (s *Service) injectBundleIntoHTML(html string, result *BundleResult, config BundleConfig) string {
	// Add CSS before </head>
	if len(result.OutputCSS) > 0 {
		cssTag := fmt.Sprintf("<style>\n%s\n</style>", string(result.OutputCSS))
		if strings.Contains(html, "</head>") {
			html = strings.Replace(html, "</head>", cssTag+"\n</head>", 1)
		} else {
			html = cssTag + html
		}
	}

	// Add JS before </body>
	if len(result.OutputJS) > 0 {
		var scriptTag string
		if config.Format == "esm" {
			scriptTag = fmt.Sprintf(`<script type="module">
%s
</script>`, string(result.OutputJS))
		} else {
			scriptTag = fmt.Sprintf(`<script>
%s
</script>`, string(result.OutputJS))
		}

		if strings.Contains(html, "</body>") {
			html = strings.Replace(html, "</body>", scriptTag+"\n</body>", 1)
		} else {
			html = html + scriptTag
		}
	}

	return html
}

// generateErrorHTML generates an error page for bundle failures
func (s *Service) generateErrorHTML(errors []BundleError) string {
	var errorList strings.Builder
	for _, err := range errors {
		errorList.WriteString("<div class='error'>")
		if err.File != "" {
			errorList.WriteString(fmt.Sprintf("<span class='file'>%s", err.File))
			if err.Line > 0 {
				errorList.WriteString(fmt.Sprintf(":%d:%d", err.Line, err.Column))
			}
			errorList.WriteString("</span><br>")
		}
		errorList.WriteString(fmt.Sprintf("<span class='message'>%s</span>", err.Message))
		if err.Text != "" {
			errorList.WriteString(fmt.Sprintf("<pre class='code'>%s</pre>", err.Text))
		}
		errorList.WriteString("</div>")
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <title>Build Error - APEX Preview</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, monospace;
      background: #1a1a2e;
      color: #eee;
      padding: 40px;
      margin: 0;
    }
    h1 {
      color: #ff6b6b;
      margin-bottom: 20px;
    }
    .error {
      background: #2d2d44;
      border-left: 4px solid #ff6b6b;
      padding: 16px;
      margin: 16px 0;
      border-radius: 4px;
    }
    .file {
      color: #4ecdc4;
      font-weight: bold;
    }
    .message {
      color: #ff6b6b;
    }
    .code {
      background: #1a1a2e;
      padding: 12px;
      border-radius: 4px;
      overflow-x: auto;
      color: #94a3b8;
    }
    .hint {
      margin-top: 30px;
      padding: 20px;
      background: #2d2d44;
      border-radius: 8px;
    }
    .hint h3 { color: #4ecdc4; margin-top: 0; }
  </style>
</head>
<body>
  <h1>Build Failed</h1>
  %s
  <div class="hint">
    <h3>Tips:</h3>
    <ul>
      <li>Check for syntax errors in your code</li>
      <li>Make sure all imports point to valid files</li>
      <li>Verify package.json has correct dependencies</li>
    </ul>
  </div>
</body>
</html>`, errorList.String())
}

// BuildRequest represents a build request from the API
type BuildRequest struct {
	ProjectID  uint   `json:"project_id" binding:"required"`
	EntryPoint string `json:"entry_point"`
	Format     string `json:"format"`
	Minify     bool   `json:"minify"`
	SourceMap  bool   `json:"source_map"`
	Framework  string `json:"framework"`
}

// BuildResponse represents a build response
type BuildResponse struct {
	Success    bool          `json:"success"`
	DurationMs int64         `json:"duration_ms"`
	Warnings   []string      `json:"warnings,omitempty"`
	Errors     []BundleError `json:"errors,omitempty"`
	Hash       string        `json:"hash,omitempty"`
	JSSize     int           `json:"js_size,omitempty"`
	CSSSize    int           `json:"css_size,omitempty"`
}

// HandleBuildRequest processes a build request
func (s *Service) HandleBuildRequest(ctx context.Context, req BuildRequest) (*BuildResponse, error) {
	config := BundleConfig{
		EntryPoint: req.EntryPoint,
		Format:     req.Format,
		Minify:     req.Minify,
		SourceMap:  req.SourceMap,
		Framework:  req.Framework,
	}

	result, err := s.BundleProject(ctx, req.ProjectID, config)
	if err != nil {
		return nil, err
	}

	return &BuildResponse{
		Success:    result.Success,
		DurationMs: result.Duration.Milliseconds(),
		Warnings:   result.Warnings,
		Errors:     result.Errors,
		Hash:       result.Hash,
		JSSize:     len(result.OutputJS),
		CSSSize:    len(result.OutputCSS),
	}, nil
}

// WatchForChanges sets up a file watcher for automatic rebuilds
// This would be called by the preview server
func (s *Service) WatchForChanges(ctx context.Context, projectID uint, onChange func(*BundleResult)) {
	// In the APEX.BUILD architecture, file changes come through WebSocket
	// So this is a no-op - the preview server handles this
	log.Printf("[bundler-service] Watch mode not needed - changes come through WebSocket")
}

// Shutdown cleans up the bundler service
func (s *Service) Shutdown() {
	s.cache.Close()
	log.Printf("[bundler-service] Shutdown complete")
}

// Status returns the current status of the bundler service
type ServiceStatus struct {
	Available   bool       `json:"available"`
	Version     string     `json:"version"`
	CacheStats  CacheStats `json:"cache_stats"`
	LastError   string     `json:"last_error,omitempty"`
}

func (s *Service) Status() ServiceStatus {
	return ServiceStatus{
		Available:  s.IsAvailable(),
		Version:    s.GetVersion(),
		CacheStats: s.GetCacheStats(),
	}
}
