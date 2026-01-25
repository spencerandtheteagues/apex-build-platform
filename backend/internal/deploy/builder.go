// APEX.BUILD Build Service
// Project type detection and build configuration generation

package deploy

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// ProjectType represents the detected type of project
type ProjectType string

const (
	ProjectTypeStaticHTML    ProjectType = "static_html"
	ProjectTypeReact         ProjectType = "react"
	ProjectTypeVue           ProjectType = "vue"
	ProjectTypeNextJS        ProjectType = "nextjs"
	ProjectTypeNuxt          ProjectType = "nuxt"
	ProjectTypeSvelte        ProjectType = "svelte"
	ProjectTypeAngular       ProjectType = "angular"
	ProjectTypeNodeJS        ProjectType = "nodejs"
	ProjectTypeExpress       ProjectType = "express"
	ProjectTypeFastify       ProjectType = "fastify"
	ProjectTypePython        ProjectType = "python"
	ProjectTypeFlask         ProjectType = "flask"
	ProjectTypeDjango        ProjectType = "django"
	ProjectTypeFastAPI       ProjectType = "fastapi"
	ProjectTypeGo            ProjectType = "go"
	ProjectTypeRust          ProjectType = "rust"
	ProjectTypeUnknown       ProjectType = "unknown"
)

// BuildConfig contains build configuration for a project
type BuildConfig struct {
	ProjectType    ProjectType       `json:"project_type"`
	Framework      string            `json:"framework"`
	BuildCommand   string            `json:"build_command"`
	InstallCommand string            `json:"install_command"`
	OutputDir      string            `json:"output_dir"`
	StartCommand   string            `json:"start_command"`
	NodeVersion    string            `json:"node_version,omitempty"`
	PythonVersion  string            `json:"python_version,omitempty"`
	GoVersion      string            `json:"go_version,omitempty"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	Dockerfile     string            `json:"dockerfile,omitempty"`
	Procfile       string            `json:"procfile,omitempty"`
}

// BuildService handles project building and packaging
type BuildService struct{}

// NewBuildService creates a new build service
func NewBuildService() *BuildService {
	return &BuildService{}
}

// DetectProjectType analyzes project files to determine the project type
func (s *BuildService) DetectProjectType(files []ProjectFile) ProjectType {
	fileMap := make(map[string]string)
	for _, f := range files {
		if !f.IsDir {
			fileMap[f.Path] = f.Content
		}
	}

	// Check for package.json (JavaScript/TypeScript projects)
	if content, exists := fileMap["/package.json"]; exists {
		return s.detectJSProjectType(content, fileMap)
	}

	// Check for Python projects
	if _, exists := fileMap["/requirements.txt"]; exists {
		return s.detectPythonProjectType(fileMap)
	}
	if _, exists := fileMap["/Pipfile"]; exists {
		return s.detectPythonProjectType(fileMap)
	}
	if _, exists := fileMap["/pyproject.toml"]; exists {
		return s.detectPythonProjectType(fileMap)
	}

	// Check for Go projects
	if _, exists := fileMap["/go.mod"]; exists {
		return ProjectTypeGo
	}

	// Check for Rust projects
	if _, exists := fileMap["/Cargo.toml"]; exists {
		return ProjectTypeRust
	}

	// Check for static HTML
	if _, exists := fileMap["/index.html"]; exists {
		return ProjectTypeStaticHTML
	}

	return ProjectTypeUnknown
}

// detectJSProjectType determines the specific JavaScript framework
func (s *BuildService) detectJSProjectType(packageJSON string, fileMap map[string]string) ProjectType {
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Scripts         map[string]string `json:"scripts"`
	}

	if err := json.Unmarshal([]byte(packageJSON), &pkg); err != nil {
		return ProjectTypeNodeJS
	}

	allDeps := make(map[string]bool)
	for dep := range pkg.Dependencies {
		allDeps[dep] = true
	}
	for dep := range pkg.DevDependencies {
		allDeps[dep] = true
	}

	// Check for Next.js
	if allDeps["next"] {
		return ProjectTypeNextJS
	}

	// Check for Nuxt
	if allDeps["nuxt"] || allDeps["nuxt3"] {
		return ProjectTypeNuxt
	}

	// Check for Vue
	if allDeps["vue"] {
		return ProjectTypeVue
	}

	// Check for Svelte/SvelteKit
	if allDeps["svelte"] || allDeps["@sveltejs/kit"] {
		return ProjectTypeSvelte
	}

	// Check for Angular
	if allDeps["@angular/core"] {
		return ProjectTypeAngular
	}

	// Check for React
	if allDeps["react"] {
		return ProjectTypeReact
	}

	// Check for Fastify
	if allDeps["fastify"] {
		return ProjectTypeFastify
	}

	// Check for Express
	if allDeps["express"] {
		return ProjectTypeExpress
	}

	// Default to generic Node.js
	return ProjectTypeNodeJS
}

// detectPythonProjectType determines the specific Python framework
func (s *BuildService) detectPythonProjectType(fileMap map[string]string) ProjectType {
	// Check requirements.txt for framework hints
	if reqs, exists := fileMap["/requirements.txt"]; exists {
		reqs = strings.ToLower(reqs)
		if strings.Contains(reqs, "django") {
			return ProjectTypeDjango
		}
		if strings.Contains(reqs, "fastapi") {
			return ProjectTypeFastAPI
		}
		if strings.Contains(reqs, "flask") {
			return ProjectTypeFlask
		}
	}

	// Check for common framework files
	if _, exists := fileMap["/manage.py"]; exists {
		return ProjectTypeDjango
	}
	if _, exists := fileMap["/app.py"]; exists {
		// Could be Flask or FastAPI, check content
		if content, ok := fileMap["/app.py"]; ok {
			if strings.Contains(content, "FastAPI") {
				return ProjectTypeFastAPI
			}
			if strings.Contains(content, "Flask") {
				return ProjectTypeFlask
			}
		}
	}

	return ProjectTypePython
}

// GenerateBuildConfig creates build configuration for a project type
func (s *BuildService) GenerateBuildConfig(projectType ProjectType, files []ProjectFile) *BuildConfig {
	config := &BuildConfig{
		ProjectType: projectType,
		EnvVars:     make(map[string]string),
	}

	switch projectType {
	case ProjectTypeStaticHTML:
		config.Framework = "static"
		config.BuildCommand = ""
		config.OutputDir = "."
		config.InstallCommand = ""
		config.StartCommand = ""

	case ProjectTypeReact:
		config.Framework = "react"
		config.BuildCommand = "npm run build"
		config.OutputDir = "build"
		config.InstallCommand = "npm install"
		config.NodeVersion = "18"
		// Check if it's Vite-based React
		if s.hasFile(files, "/vite.config.js") || s.hasFile(files, "/vite.config.ts") {
			config.OutputDir = "dist"
		}

	case ProjectTypeVue:
		config.Framework = "vue"
		config.BuildCommand = "npm run build"
		config.OutputDir = "dist"
		config.InstallCommand = "npm install"
		config.NodeVersion = "18"

	case ProjectTypeNextJS:
		config.Framework = "nextjs"
		config.BuildCommand = "npm run build"
		config.OutputDir = ".next"
		config.InstallCommand = "npm install"
		config.StartCommand = "npm start"
		config.NodeVersion = "18"

	case ProjectTypeNuxt:
		config.Framework = "nuxt"
		config.BuildCommand = "npm run build"
		config.OutputDir = ".output"
		config.InstallCommand = "npm install"
		config.StartCommand = "npm start"
		config.NodeVersion = "18"

	case ProjectTypeSvelte:
		config.Framework = "svelte"
		config.BuildCommand = "npm run build"
		config.OutputDir = "build"
		config.InstallCommand = "npm install"
		config.NodeVersion = "18"

	case ProjectTypeAngular:
		config.Framework = "angular"
		config.BuildCommand = "npm run build"
		config.OutputDir = "dist"
		config.InstallCommand = "npm install"
		config.NodeVersion = "18"

	case ProjectTypeNodeJS, ProjectTypeExpress, ProjectTypeFastify:
		config.Framework = string(projectType)
		config.BuildCommand = "npm run build || true"
		config.InstallCommand = "npm install"
		config.StartCommand = "npm start"
		config.NodeVersion = "18"
		config.Procfile = "web: npm start"
		config.Dockerfile = s.generateNodeDockerfile()

	case ProjectTypePython:
		config.Framework = "python"
		config.BuildCommand = ""
		config.InstallCommand = "pip install -r requirements.txt"
		config.StartCommand = "python main.py"
		config.PythonVersion = "3.11"
		config.Procfile = "web: python main.py"
		config.Dockerfile = s.generatePythonDockerfile("main.py")

	case ProjectTypeFlask:
		config.Framework = "flask"
		config.BuildCommand = ""
		config.InstallCommand = "pip install -r requirements.txt"
		config.StartCommand = "gunicorn app:app"
		config.PythonVersion = "3.11"
		config.Procfile = "web: gunicorn app:app"
		config.Dockerfile = s.generatePythonDockerfile("app.py")

	case ProjectTypeDjango:
		config.Framework = "django"
		config.BuildCommand = "python manage.py collectstatic --noinput"
		config.InstallCommand = "pip install -r requirements.txt"
		config.StartCommand = "gunicorn config.wsgi:application"
		config.PythonVersion = "3.11"
		config.Procfile = "web: gunicorn config.wsgi:application"
		config.Dockerfile = s.generateDjangoDockerfile()

	case ProjectTypeFastAPI:
		config.Framework = "fastapi"
		config.BuildCommand = ""
		config.InstallCommand = "pip install -r requirements.txt"
		config.StartCommand = "uvicorn main:app --host 0.0.0.0 --port $PORT"
		config.PythonVersion = "3.11"
		config.Procfile = "web: uvicorn main:app --host 0.0.0.0 --port $PORT"
		config.Dockerfile = s.generateFastAPIDockerfile()

	case ProjectTypeGo:
		config.Framework = "go"
		config.BuildCommand = "go build -o app ."
		config.InstallCommand = "go mod download"
		config.StartCommand = "./app"
		config.GoVersion = "1.21"
		config.Procfile = "web: ./app"
		config.Dockerfile = s.generateGoDockerfile()

	case ProjectTypeRust:
		config.Framework = "rust"
		config.BuildCommand = "cargo build --release"
		config.InstallCommand = ""
		config.StartCommand = "./target/release/app"
		config.Dockerfile = s.generateRustDockerfile()

	default:
		config.Framework = "unknown"
		config.BuildCommand = ""
		config.OutputDir = "."
	}

	return config
}

// PackageProject creates a deployable package from project files
func (s *BuildService) PackageProject(files []ProjectFile, config *DeploymentConfig) ([]ProjectFile, error) {
	result := make([]ProjectFile, 0, len(files))

	for _, f := range files {
		// Skip directories
		if f.IsDir {
			continue
		}

		// Skip common files that shouldn't be deployed
		if s.shouldSkipFile(f.Path) {
			continue
		}

		// Add file to package
		result = append(result, ProjectFile{
			Path:     s.normalizePath(f.Path),
			Content:  f.Content,
			Size:     f.Size,
			MimeType: f.MimeType,
		})
	}

	return result, nil
}

// CreateZipArchive creates a ZIP archive of the project files
func (s *BuildService) CreateZipArchive(files []ProjectFile) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	for _, f := range files {
		if f.IsDir {
			continue
		}

		path := strings.TrimPrefix(f.Path, "/")
		writer, err := zipWriter.Create(path)
		if err != nil {
			return nil, fmt.Errorf("failed to create zip entry for %s: %w", f.Path, err)
		}

		if _, err := writer.Write([]byte(f.Content)); err != nil {
			return nil, fmt.Errorf("failed to write content for %s: %w", f.Path, err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip archive: %w", err)
	}

	return buf.Bytes(), nil
}

// CreateBase64Archive creates a base64-encoded ZIP archive
func (s *BuildService) CreateBase64Archive(files []ProjectFile) (string, error) {
	zipData, err := s.CreateZipArchive(files)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(zipData), nil
}

// Helper functions

func (s *BuildService) hasFile(files []ProjectFile, path string) bool {
	for _, f := range files {
		if f.Path == path {
			return true
		}
	}
	return false
}

func (s *BuildService) shouldSkipFile(path string) bool {
	skipPaths := []string{
		"node_modules/",
		".git/",
		".env",
		".env.local",
		".env.development",
		".env.production",
		"__pycache__/",
		".pyc",
		"venv/",
		".venv/",
		"target/",
		".DS_Store",
		"Thumbs.db",
		".idea/",
		".vscode/",
	}

	for _, skip := range skipPaths {
		if strings.Contains(path, skip) {
			return true
		}
	}

	return false
}

func (s *BuildService) normalizePath(path string) string {
	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Clean the path
	return filepath.Clean(path)
}

// Dockerfile generators

func (s *BuildService) generateNodeDockerfile() string {
	return `FROM node:18-alpine

WORKDIR /app

COPY package*.json ./
RUN npm ci --only=production

COPY . .

EXPOSE 3000
CMD ["npm", "start"]`
}

func (s *BuildService) generatePythonDockerfile(entrypoint string) string {
	return fmt.Sprintf(`FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 8000
CMD ["python", "%s"]`, entrypoint)
}

func (s *BuildService) generateDjangoDockerfile() string {
	return `FROM python:3.11-slim

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE 1
ENV PYTHONUNBUFFERED 1

RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq-dev \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt gunicorn

COPY . .

RUN python manage.py collectstatic --noinput

EXPOSE 8000
CMD ["gunicorn", "--bind", "0.0.0.0:8000", "config.wsgi:application"]`
}

func (s *BuildService) generateFastAPIDockerfile() string {
	return `FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 8000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]`
}

func (s *BuildService) generateGoDockerfile() string {
	return `FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/app .

EXPOSE 8080
CMD ["./app"]`
}

func (s *BuildService) generateRustDockerfile() string {
	return `FROM rust:1.73 AS builder

WORKDIR /app

COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo "fn main() {}" > src/main.rs
RUN cargo build --release

COPY . .
RUN touch src/main.rs && cargo build --release

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/target/release/app /usr/local/bin/

EXPOSE 8080
CMD ["app"]`
}

// GenerateNetlifyConfig creates a netlify.toml configuration
func (s *BuildService) GenerateNetlifyConfig(config *BuildConfig) string {
	return fmt.Sprintf(`[build]
  command = "%s"
  publish = "%s"

[build.environment]
  NODE_VERSION = "%s"

[[redirects]]
  from = "/*"
  to = "/index.html"
  status = 200
`, config.BuildCommand, config.OutputDir, config.NodeVersion)
}

// GenerateVercelConfig creates a vercel.json configuration
func (s *BuildService) GenerateVercelConfig(config *BuildConfig) string {
	vercelConfig := map[string]interface{}{
		"version": 2,
	}

	if config.BuildCommand != "" {
		vercelConfig["buildCommand"] = config.BuildCommand
	}
	if config.OutputDir != "" {
		vercelConfig["outputDirectory"] = config.OutputDir
	}
	if config.InstallCommand != "" {
		vercelConfig["installCommand"] = config.InstallCommand
	}
	if config.Framework != "" {
		vercelConfig["framework"] = config.Framework
	}

	data, _ := json.MarshalIndent(vercelConfig, "", "  ")
	return string(data)
}

// GenerateRenderConfig creates a render.yaml configuration
func (s *BuildService) GenerateRenderConfig(config *BuildConfig, name string) string {
	return fmt.Sprintf(`services:
  - type: web
    name: %s
    env: node
    buildCommand: %s
    startCommand: %s
    healthCheckPath: /health
    envVars:
      - key: NODE_ENV
        value: production
`, name, config.BuildCommand, config.StartCommand)
}
