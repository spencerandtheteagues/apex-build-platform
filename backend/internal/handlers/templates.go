// Package handlers - Project Templates HTTP Handlers
package handlers

import (
	"net/http"
	"time"

	"apex-build/internal/templates"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TemplatesHandler handles template-related endpoints
type TemplatesHandler struct {
	db *gorm.DB
}

// NewTemplatesHandler creates a new templates handler
func NewTemplatesHandler(db *gorm.DB) *TemplatesHandler {
	return &TemplatesHandler{db: db}
}

// ListTemplates returns all available project templates
func (h *TemplatesHandler) ListTemplates(c *gin.Context) {
	category := c.Query("category")
	popular := c.Query("popular")

	var templateList []templates.Template

	if popular == "true" {
		templateList = templates.GetPopularTemplates()
	} else if category != "" {
		templateList = templates.GetTemplatesByCategory(templates.TemplateCategory(category))
	} else {
		templateList = templates.GetAllTemplates()
	}

	// Group by category for frontend
	categories := make(map[string][]templates.Template)
	for _, t := range templateList {
		cat := string(t.Category)
		categories[cat] = append(categories[cat], t)
	}

	c.JSON(http.StatusOK, gin.H{
		"templates":  templateList,
		"categories": categories,
		"count":      len(templateList),
	})
}

// GetTemplate returns a specific template by ID
func (h *TemplatesHandler) GetTemplate(c *gin.Context) {
	templateID := c.Param("id")

	template, err := templates.GetTemplateByID(templateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"template": template})
}

// CreateProjectFromTemplate creates a new project from a template
func (h *TemplatesHandler) CreateProjectFromTemplate(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		TemplateID  string `json:"template_id" binding:"required"`
		ProjectName string `json:"project_name" binding:"required"`
		Description string `json:"description,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the template
	template, err := templates.GetTemplateByID(req.TemplateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	// Create the project
	project := &models.Project{
		OwnerID:     userID,
		Name:        req.ProjectName,
		Description: req.Description,
		Language:    template.Language,
		Framework:   template.Framework,
		IsPublic:    false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.db.Create(project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		return
	}

	// Create files from template
	var files []models.File
	for _, tf := range template.Files {
		file := models.File{
			ProjectID: project.ID,
			Name:      getFileName(tf.Path),
			Path:      tf.Path,
			Content:   tf.Content,
			MimeType:  getMimeType(tf.Path),
			Size:      int64(len(tf.Content)),
			Version:   1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		files = append(files, file)
	}

	// Add package.json if dependencies exist
	if len(template.Dependencies) > 0 || len(template.DevDependencies) > 0 {
		packageJSON := generatePackageJSON(req.ProjectName, template)
		files = append(files, models.File{
			ProjectID: project.ID,
			Name:      "package.json",
			Path:      "package.json",
			Content:   packageJSON,
			MimeType:  "application/json",
			Size:      int64(len(packageJSON)),
			Version:   1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}

	// Add .env.example if env vars exist
	if len(template.EnvVars) > 0 {
		envContent := generateEnvExample(template.EnvVars)
		files = append(files, models.File{
			ProjectID: project.ID,
			Name:      ".env.example",
			Path:      ".env.example",
			Content:   envContent,
			MimeType:  "text/plain",
			Size:      int64(len(envContent)),
			Version:   1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}

	// Batch create files
	if len(files) > 0 {
		if err := h.db.Create(&files).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create files"})
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Project created from template",
		"project":     project,
		"files_count": len(files),
		"template":    template.Name,
	})
}

// GetCategories returns all template categories
func (h *TemplatesHandler) GetCategories(c *gin.Context) {
	categories := []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	}{
		{"frontend", "Frontend", "Client-side web applications", "🎨"},
		{"backend", "Backend", "Server-side applications and services", "⚙️"},
		{"fullstack", "Full Stack", "Complete web applications", "🏗️"},
		{"api", "API", "REST and GraphQL APIs", "🔌"},
		{"mobile", "Mobile", "Mobile applications", "📱"},
		{"game", "Game", "Game development", "🎮"},
		{"data", "Data Science", "Data analysis and ML", "📊"},
		{"automation", "Automation", "Bots and scripts", "🤖"},
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// Helper functions

func getFileName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

func getMimeType(path string) string {
	ext := ""
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			ext = path[i:]
			break
		}
	}

	mimeTypes := map[string]string{
		".js":    "text/javascript",
		".ts":    "text/typescript",
		".tsx":   "text/typescript",
		".jsx":   "text/javascript",
		".json":  "application/json",
		".html":  "text/html",
		".css":   "text/css",
		".md":    "text/markdown",
		".py":    "text/x-python",
		".go":    "text/x-go",
		".vue":   "text/x-vue",
		".yaml":  "text/yaml",
		".yml":   "text/yaml",
		".toml":  "text/toml",
		".sql":   "text/x-sql",
		".sh":    "text/x-sh",
		".txt":   "text/plain",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "text/plain"
}

func generatePackageJSON(name string, template *templates.Template) string {
	deps := "{\n"
	first := true
	for pkg, version := range template.Dependencies {
		if !first {
			deps += ",\n"
		}
		deps += "    \"" + pkg + "\": \"" + version + "\""
		first = false
	}
	deps += "\n  }"

	devDeps := "{\n"
	first = true
	for pkg, version := range template.DevDependencies {
		if !first {
			devDeps += ",\n"
		}
		devDeps += "    \"" + pkg + "\": \"" + version + "\""
		first = false
	}
	devDeps += "\n  }"

	scripts := "{\n"
	first = true
	for cmd, script := range template.Scripts {
		if !first {
			scripts += ",\n"
		}
		scripts += "    \"" + cmd + "\": \"" + script + "\""
		first = false
	}
	scripts += "\n  }"

	return `{
  "name": "` + name + `",
  "version": "1.0.0",
  "description": "Created with APEX.BUILD",
  "scripts": ` + scripts + `,
  "dependencies": ` + deps + `,
  "devDependencies": ` + devDeps + `
}
`
}

func generateEnvExample(envVars []templates.EnvVar) string {
	content := "# Environment Variables\n# Copy this file to .env and fill in your values\n\n"
	for _, ev := range envVars {
		content += "# " + ev.Description + "\n"
		if ev.Required {
			content += "# (Required)\n"
		}
		if ev.Default != "" {
			content += ev.Name + "=" + ev.Default + "\n\n"
		} else {
			content += ev.Name + "=\n\n"
		}
	}
	return content
}
