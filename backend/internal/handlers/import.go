// Package handlers - GitHub Repository Import Handler for APEX.BUILD
// Enables one-click import of GitHub repositories similar to Replit's replit.new/URL feature
package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"apex-build/internal/git"
	"apex-build/internal/secrets"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ImportHandler handles GitHub repository imports
type ImportHandler struct {
	db             *gorm.DB
	gitService     *git.GitService
	secretsManager *secrets.SecretsManager
}

// NewImportHandler creates a new import handler
func NewImportHandler(db *gorm.DB, gitService *git.GitService, secretsManager *secrets.SecretsManager) *ImportHandler {
	return &ImportHandler{
		db:             db,
		gitService:     gitService,
		secretsManager: secretsManager,
	}
}

// GitHubImportRequest represents a request to import a GitHub repository
type GitHubImportRequest struct {
	URL         string `json:"url" binding:"required"`
	ProjectName string `json:"project_name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	Token       string `json:"token"` // Optional GitHub personal access token for private repos
}

// GitHubImportResponse represents the response from a GitHub import
type GitHubImportResponse struct {
	ProjectID       uint                   `json:"project_id"`
	ProjectName     string                 `json:"project_name"`
	Language        string                 `json:"language"`
	Framework       string                 `json:"framework"`
	DetectedStack   map[string]interface{} `json:"detected_stack"`
	FileCount       int                    `json:"file_count"`
	Status          string                 `json:"status"`
	Message         string                 `json:"message"`
	ImportDuration  int64                  `json:"import_duration_ms"`
	RepositoryURL   string                 `json:"repository_url"`
	DefaultBranch   string                 `json:"default_branch"`
}

// ImportStatus tracks the status of an import operation
type ImportStatus struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index"`
	ProjectID uint      `json:"project_id"`
	URL       string    `json:"url"`
	Status    string    `json:"status"` // pending, cloning, analyzing, importing, completed, failed
	Progress  int       `json:"progress"` // 0-100
	Message   string    `json:"message"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LanguageDetection contains detected language/framework information
type LanguageDetection struct {
	PrimaryLanguage string            `json:"primary_language"`
	Framework       string            `json:"framework"`
	PackageManager  string            `json:"package_manager"`
	EntryPoint      string            `json:"entry_point"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"dev_dependencies,omitempty"`
	Scripts         map[string]string `json:"scripts,omitempty"`
}

// GitHubRepoInfo contains GitHub repository metadata
type GitHubRepoInfo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	DefaultBranch string `json:"default_branch"`
	Language      string `json:"language"`
	Size          int    `json:"size"`
	Private       bool   `json:"private"`
	Fork          bool   `json:"fork"`
	StarCount     int    `json:"stargazers_count"`
	ForkCount     int    `json:"forks_count"`
}

// ImportGitHub handles importing a GitHub repository
// POST /api/v1/projects/import/github
func (h *ImportHandler) ImportGitHub(c *gin.Context) {
	startTime := time.Now()
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var req GitHubImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse and validate GitHub URL
	owner, repo, err := parseGitHubURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Step 1: Fetch repository info from GitHub API
	repoInfo, err := h.getGitHubRepoInfo(ctx, owner, repo, req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Failed to access repository: %v", err),
			"hint":  "Make sure the repository exists and you have access to it. For private repos, provide a personal access token.",
		})
		return
	}

	// Step 2: Determine project name
	projectName := req.ProjectName
	if projectName == "" {
		projectName = repoInfo.Name
	}

	// Check if project name already exists for user
	var existingProject models.Project
	if err := h.db.Where("owner_id = ? AND name = ?", userID, projectName).First(&existingProject).Error; err == nil {
		// Add timestamp suffix to make unique
		projectName = fmt.Sprintf("%s-%d", projectName, time.Now().Unix())
	}

	// Step 3: Get repository tree (file listing)
	files, err := h.getGitHubRepoTree(ctx, owner, repo, repoInfo.DefaultBranch, req.Token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to fetch repository files: %v", err),
		})
		return
	}

	// Step 4: Detect language and framework
	detection := h.detectLanguageAndFramework(files)

	// Step 5: Create project
	description := req.Description
	if description == "" {
		description = repoInfo.Description
	}

	project := &models.Project{
		Name:        projectName,
		Description: description,
		Language:    detection.PrimaryLanguage,
		Framework:   detection.Framework,
		OwnerID:     userID.(uint),
		IsPublic:    req.IsPublic,
		EntryPoint:  detection.EntryPoint,
		Environment: map[string]interface{}{
			"github_url":      req.URL,
			"github_owner":    owner,
			"github_repo":     repo,
			"default_branch":  repoInfo.DefaultBranch,
			"package_manager": detection.PackageManager,
		},
		Dependencies: map[string]interface{}{
			"dependencies":    detection.Dependencies,
			"devDependencies": detection.DevDependencies,
		},
		BuildConfig: map[string]interface{}{
			"scripts": detection.Scripts,
		},
	}

	if err := h.db.Create(project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		return
	}

	// Step 6: Download and store files
	fileCount := 0
	for _, file := range files {
		if file.Type == "blob" && !shouldSkipFile(file.Path) {
			content, err := h.getGitHubFileContent(ctx, owner, repo, file.Path, repoInfo.DefaultBranch, req.Token)
			if err != nil {
				continue // Skip files that can't be downloaded
			}

			// Get file name from path
			pathParts := strings.Split(file.Path, "/")
			fileName := pathParts[len(pathParts)-1]

			// Determine file type
			fileType := "file"
			mimeType := getFileMimeType(fileName)

			dbFile := &models.File{
				ProjectID: project.ID,
				Path:      file.Path,
				Name:      fileName,
				Type:      fileType,
				Content:   content,
				Size:      int64(len(content)),
				MimeType:  mimeType,
				Version:   1,
			}

			if err := h.db.Create(dbFile).Error; err == nil {
				fileCount++
			}
		}
	}

	// Step 7: Connect to GitHub repository for future sync
	if req.Token != "" {
		_, _ = h.gitService.ConnectRepository(ctx, project.ID, req.URL, req.Token)
	}

	duration := time.Since(startTime).Milliseconds()

	c.JSON(http.StatusCreated, GitHubImportResponse{
		ProjectID:      project.ID,
		ProjectName:    project.Name,
		Language:       detection.PrimaryLanguage,
		Framework:      detection.Framework,
		DetectedStack: map[string]interface{}{
			"language":        detection.PrimaryLanguage,
			"framework":       detection.Framework,
			"package_manager": detection.PackageManager,
			"entry_point":     detection.EntryPoint,
		},
		FileCount:      fileCount,
		Status:         "completed",
		Message:        fmt.Sprintf("Successfully imported %d files from %s", fileCount, req.URL),
		ImportDuration: duration,
		RepositoryURL:  req.URL,
		DefaultBranch:  repoInfo.DefaultBranch,
	})
}

// ValidateGitHubURL validates a GitHub URL and returns repository info
// POST /api/v1/projects/import/github/validate
func (h *ImportHandler) ValidateGitHubURL(c *gin.Context) {
	var req struct {
		URL   string `json:"url" binding:"required"`
		Token string `json:"token"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	owner, repo, err := parseGitHubURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	ctx := c.Request.Context()

	// Fetch repository info
	repoInfo, err := h.getGitHubRepoInfo(ctx, owner, repo, req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid":   false,
			"error":   fmt.Sprintf("Cannot access repository: %v", err),
			"hint":    "For private repositories, provide a personal access token.",
			"private": strings.Contains(err.Error(), "404"),
		})
		return
	}

	// Get file tree for language detection
	files, _ := h.getGitHubRepoTree(ctx, owner, repo, repoInfo.DefaultBranch, req.Token)
	detection := h.detectLanguageAndFramework(files)

	c.JSON(http.StatusOK, gin.H{
		"valid":          true,
		"owner":          owner,
		"repo":           repo,
		"name":           repoInfo.Name,
		"description":    repoInfo.Description,
		"default_branch": repoInfo.DefaultBranch,
		"language":       repoInfo.Language,
		"size":           repoInfo.Size,
		"private":        repoInfo.Private,
		"fork":           repoInfo.Fork,
		"stars":          repoInfo.StarCount,
		"forks":          repoInfo.ForkCount,
		"detected_stack": map[string]interface{}{
			"language":        detection.PrimaryLanguage,
			"framework":       detection.Framework,
			"package_manager": detection.PackageManager,
			"entry_point":     detection.EntryPoint,
		},
	})
}

// RegisterImportRoutes registers import-related routes
func (h *ImportHandler) RegisterImportRoutes(rg *gin.RouterGroup) {
	imports := rg.Group("/projects/import")
	{
		imports.POST("/github", h.ImportGitHub)
		imports.POST("/github/validate", h.ValidateGitHubURL)
	}
}

// Helper functions

// parseGitHubURL extracts owner and repo from various GitHub URL formats
func parseGitHubURL(url string) (owner, repo string, err error) {
	// Clean up the URL
	url = strings.TrimSpace(url)
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")

	// Handle different GitHub URL formats:
	// - https://github.com/owner/repo
	// - http://github.com/owner/repo
	// - github.com/owner/repo
	// - git@github.com:owner/repo

	patterns := []struct {
		regex   string
		ownerIdx int
		repoIdx  int
	}{
		{`^https?://github\.com/([^/]+)/([^/]+)/?$`, 1, 2},
		{`^github\.com/([^/]+)/([^/]+)/?$`, 1, 2},
		{`^git@github\.com:([^/]+)/([^/]+)$`, 1, 2},
		{`^([^/]+)/([^/]+)$`, 1, 2}, // Short format: owner/repo
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p.regex)
		matches := re.FindStringSubmatch(url)
		if matches != nil {
			owner = matches[p.ownerIdx]
			repo = matches[p.repoIdx]

			// Validate owner and repo names
			if !isValidGitHubName(owner) || !isValidGitHubName(repo) {
				continue
			}

			return owner, repo, nil
		}
	}

	return "", "", fmt.Errorf("invalid GitHub URL format. Expected: https://github.com/owner/repo or owner/repo")
}

// isValidGitHubName checks if a string is a valid GitHub username/repo name
func isValidGitHubName(name string) bool {
	if len(name) == 0 || len(name) > 100 {
		return false
	}
	// GitHub names can contain alphanumeric characters and hyphens
	re := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)
	return re.MatchString(name) || len(name) == 1
}

// getGitHubRepoInfo fetches repository metadata from GitHub API
func (h *ImportHandler) getGitHubRepoInfo(ctx context.Context, owner, repo, token string) (*GitHubRepoInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "APEX.BUILD")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 404 {
			return nil, fmt.Errorf("repository not found or is private")
		}
		if resp.StatusCode == 403 {
			return nil, fmt.Errorf("rate limit exceeded or access denied")
		}
		return nil, fmt.Errorf("GitHub API error: %d", resp.StatusCode)
	}

	var repoInfo GitHubRepoInfo
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return nil, err
	}

	return &repoInfo, nil
}

// GitHubTreeEntry represents a file/folder in the repo tree
type GitHubTreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int    `json:"size"`
}

// getGitHubRepoTree fetches the complete file tree for a repository
func (h *ImportHandler) getGitHubRepoTree(ctx context.Context, owner, repo, branch, token string) ([]GitHubTreeEntry, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", owner, repo, branch)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "APEX.BUILD")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch repository tree: %d", resp.StatusCode)
	}

	var result struct {
		Tree      []GitHubTreeEntry `json:"tree"`
		Truncated bool              `json:"truncated"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Tree, nil
}

// getGitHubFileContent downloads a file's content from GitHub
func (h *ImportHandler) getGitHubFileContent(ctx context.Context, owner, repo, path, branch, token string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, branch)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "APEX.BUILD")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch file content: %d", resp.StatusCode)
	}

	var content struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return "", err
	}

	if content.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content.Content, "\n", ""))
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}

	return content.Content, nil
}

// detectLanguageAndFramework analyzes files to detect the primary language and framework
func (h *ImportHandler) detectLanguageAndFramework(files []GitHubTreeEntry) LanguageDetection {
	detection := LanguageDetection{
		PrimaryLanguage: "javascript", // Default
		Dependencies:    make(map[string]string),
		DevDependencies: make(map[string]string),
		Scripts:         make(map[string]string),
	}

	// Count file extensions
	extCount := make(map[string]int)
	hasFile := make(map[string]bool)

	for _, file := range files {
		if file.Type != "blob" {
			continue
		}

		// Track specific config files
		baseName := strings.ToLower(file.Path)
		if strings.Contains(baseName, "/") {
			parts := strings.Split(baseName, "/")
			baseName = parts[len(parts)-1]
		}
		hasFile[baseName] = true

		// Count extensions
		parts := strings.Split(file.Path, ".")
		if len(parts) > 1 {
			ext := "." + parts[len(parts)-1]
			extCount[ext]++
		}
	}

	// Detect language based on files
	switch {
	case hasFile["package.json"]:
		detection.PrimaryLanguage = "javascript"
		detection.PackageManager = "npm"

		// Detect framework
		if hasFile["next.config.js"] || hasFile["next.config.mjs"] || hasFile["next.config.ts"] {
			detection.Framework = "nextjs"
			detection.EntryPoint = "pages/index.js"
		} else if hasFile["nuxt.config.js"] || hasFile["nuxt.config.ts"] {
			detection.Framework = "nuxt"
			detection.EntryPoint = "pages/index.vue"
		} else if hasFile["vite.config.js"] || hasFile["vite.config.ts"] {
			detection.Framework = "vite"
			detection.EntryPoint = "src/main.tsx"
		} else if hasFile["angular.json"] {
			detection.Framework = "angular"
			detection.EntryPoint = "src/main.ts"
		} else if hasFile["svelte.config.js"] {
			detection.Framework = "svelte"
			detection.EntryPoint = "src/main.js"
		} else if extCount[".tsx"] > 0 || extCount[".jsx"] > 0 {
			detection.Framework = "react"
			if extCount[".tsx"] > extCount[".jsx"] {
				detection.PrimaryLanguage = "typescript"
				detection.EntryPoint = "src/index.tsx"
			} else {
				detection.EntryPoint = "src/index.jsx"
			}
		} else if hasFile["express"] || hasFile["server.js"] || hasFile["app.js"] {
			detection.Framework = "express"
			detection.EntryPoint = "server.js"
		}

		// Check for TypeScript
		if hasFile["tsconfig.json"] {
			detection.PrimaryLanguage = "typescript"
		}

		// Check for yarn
		if hasFile["yarn.lock"] {
			detection.PackageManager = "yarn"
		} else if hasFile["pnpm-lock.yaml"] {
			detection.PackageManager = "pnpm"
		} else if hasFile["bun.lockb"] {
			detection.PackageManager = "bun"
		}

	case hasFile["go.mod"]:
		detection.PrimaryLanguage = "go"
		detection.PackageManager = "go modules"

		if hasFile["main.go"] {
			detection.EntryPoint = "main.go"
		}

		// Detect Go framework
		if hasFile["gin"] || extCount[".go"] > 0 {
			detection.Framework = "gin" // Default for Go APIs
		}

	case hasFile["requirements.txt"] || hasFile["pyproject.toml"] || hasFile["setup.py"]:
		detection.PrimaryLanguage = "python"

		if hasFile["pyproject.toml"] {
			detection.PackageManager = "poetry"
		} else if hasFile["pipfile"] {
			detection.PackageManager = "pipenv"
		} else {
			detection.PackageManager = "pip"
		}

		// Detect Python framework
		if hasFile["manage.py"] || hasFile["django"] {
			detection.Framework = "django"
			detection.EntryPoint = "manage.py"
		} else if hasFile["app.py"] || hasFile["main.py"] {
			detection.Framework = "flask"
			if hasFile["app.py"] {
				detection.EntryPoint = "app.py"
			} else {
				detection.EntryPoint = "main.py"
			}
		} else if hasFile["fastapi"] {
			detection.Framework = "fastapi"
			detection.EntryPoint = "main.py"
		}

	case hasFile["cargo.toml"]:
		detection.PrimaryLanguage = "rust"
		detection.PackageManager = "cargo"
		detection.EntryPoint = "src/main.rs"

		if hasFile["actix"] || hasFile["rocket"] {
			detection.Framework = "actix"
		}

	case hasFile["pubspec.yaml"]:
		detection.PrimaryLanguage = "dart"
		detection.PackageManager = "pub"
		detection.Framework = "flutter"
		detection.EntryPoint = "lib/main.dart"

	case hasFile["gemfile"]:
		detection.PrimaryLanguage = "ruby"
		detection.PackageManager = "bundler"

		if hasFile["config.ru"] || hasFile["rails"] {
			detection.Framework = "rails"
			detection.EntryPoint = "config.ru"
		}

	case hasFile["composer.json"]:
		detection.PrimaryLanguage = "php"
		detection.PackageManager = "composer"

		if hasFile["artisan"] {
			detection.Framework = "laravel"
			detection.EntryPoint = "public/index.php"
		}

	case hasFile["pom.xml"] || hasFile["build.gradle"]:
		detection.PrimaryLanguage = "java"
		if hasFile["build.gradle"] {
			detection.PackageManager = "gradle"
		} else {
			detection.PackageManager = "maven"
		}

		if hasFile["spring"] {
			detection.Framework = "spring"
		}

	default:
		// Determine by file extension count
		maxCount := 0
		for ext, count := range extCount {
			if count > maxCount {
				maxCount = count
				switch ext {
				case ".ts", ".tsx":
					detection.PrimaryLanguage = "typescript"
				case ".js", ".jsx":
					detection.PrimaryLanguage = "javascript"
				case ".py":
					detection.PrimaryLanguage = "python"
				case ".go":
					detection.PrimaryLanguage = "go"
				case ".rs":
					detection.PrimaryLanguage = "rust"
				case ".rb":
					detection.PrimaryLanguage = "ruby"
				case ".java":
					detection.PrimaryLanguage = "java"
				case ".php":
					detection.PrimaryLanguage = "php"
				case ".c", ".cpp", ".cc":
					detection.PrimaryLanguage = "cpp"
				case ".cs":
					detection.PrimaryLanguage = "csharp"
				case ".swift":
					detection.PrimaryLanguage = "swift"
				case ".kt":
					detection.PrimaryLanguage = "kotlin"
				}
			}
		}
	}

	return detection
}

// shouldSkipFile determines if a file should be skipped during import
func shouldSkipFile(path string) bool {
	skipPatterns := []string{
		"node_modules/",
		".git/",
		".github/",
		".vscode/",
		".idea/",
		"__pycache__/",
		".pytest_cache/",
		"dist/",
		"build/",
		".next/",
		".nuxt/",
		"vendor/",
		"target/",
		".DS_Store",
		"Thumbs.db",
		".env",
		".env.local",
		".env.production",
		"*.lock",
		"package-lock.json",
		"yarn.lock",
		"pnpm-lock.yaml",
	}

	pathLower := strings.ToLower(path)
	for _, pattern := range skipPatterns {
		if strings.HasPrefix(pattern, "*") {
			// Suffix match
			suffix := pattern[1:]
			if strings.HasSuffix(pathLower, suffix) {
				return true
			}
		} else if strings.HasSuffix(pattern, "/") {
			// Directory prefix match
			if strings.HasPrefix(pathLower, pattern) || strings.Contains(pathLower, "/"+pattern) {
				return true
			}
		} else {
			// Exact match or contains
			if pathLower == pattern || strings.HasSuffix(pathLower, "/"+pattern) {
				return true
			}
		}
	}

	return false
}

// getFileMimeType returns the MIME type for a file based on extension
func getFileMimeType(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) < 2 {
		return "text/plain"
	}
	ext := "." + parts[len(parts)-1]

	mimeTypes := map[string]string{
		".js":     "text/javascript",
		".ts":     "text/typescript",
		".tsx":    "text/typescript",
		".jsx":    "text/javascript",
		".json":   "application/json",
		".html":   "text/html",
		".htm":    "text/html",
		".css":    "text/css",
		".scss":   "text/scss",
		".sass":   "text/sass",
		".less":   "text/less",
		".md":     "text/markdown",
		".markdown": "text/markdown",
		".py":     "text/x-python",
		".go":     "text/x-go",
		".rs":     "text/x-rust",
		".rb":     "text/x-ruby",
		".php":    "text/x-php",
		".java":   "text/x-java",
		".c":      "text/x-c",
		".cpp":    "text/x-c++",
		".cc":     "text/x-c++",
		".h":      "text/x-c",
		".hpp":    "text/x-c++",
		".cs":     "text/x-csharp",
		".swift":  "text/x-swift",
		".kt":     "text/x-kotlin",
		".yaml":   "text/yaml",
		".yml":    "text/yaml",
		".xml":    "application/xml",
		".svg":    "image/svg+xml",
		".png":    "image/png",
		".jpg":    "image/jpeg",
		".jpeg":   "image/jpeg",
		".gif":    "image/gif",
		".ico":    "image/x-icon",
		".woff":   "font/woff",
		".woff2":  "font/woff2",
		".ttf":    "font/ttf",
		".eot":    "application/vnd.ms-fontobject",
		".sh":     "text/x-shellscript",
		".bash":   "text/x-shellscript",
		".zsh":    "text/x-shellscript",
		".sql":    "text/x-sql",
		".graphql": "text/x-graphql",
		".gql":    "text/x-graphql",
		".vue":    "text/x-vue",
		".svelte": "text/x-svelte",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "text/plain"
}
