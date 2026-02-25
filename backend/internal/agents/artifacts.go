package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"apex-build/pkg/models"

	"gorm.io/gorm"
)

// ArtifactFile is the canonical build artifact file representation used for apply/export.
type ArtifactFile struct {
	Path     string `json:"path"`
	Content  string `json:"content,omitempty"`
	Language string `json:"language,omitempty"`
	Size     int64  `json:"size"`
	IsNew    bool   `json:"is_new,omitempty"`
	Deleted  bool   `json:"deleted,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

// BuildArtifactManifest is the canonical artifact contract for build apply/download flows.
type BuildArtifactManifest struct {
	BuildID      string                 `json:"build_id"`
	Revision     string                 `json:"revision"`
	Source       string                 `json:"source"` // live or snapshot
	Description  string                 `json:"description,omitempty"`
	GeneratedAt  time.Time              `json:"generated_at"`
	ProjectID    *uint                  `json:"project_id,omitempty"`
	Files        []ArtifactFile         `json:"files"`
	Entrypoints  map[string]string      `json:"entrypoints,omitempty"`
	RuntimeHints map[string]string      `json:"runtime_hints,omitempty"`
	Verification map[string]interface{} `json:"verification,omitempty"`
	Warnings     []string               `json:"warnings,omitempty"`
	Errors       []string               `json:"errors,omitempty"`
}

// ApplyArtifactsResult summarizes an atomic apply operation.
type ApplyArtifactsResult struct {
	ProjectID      uint     `json:"project_id"`
	CreatedProject bool     `json:"created_project"`
	AppliedFiles   int      `json:"applied_files"`
	DeletedFiles   int      `json:"deleted_files"`
	Manifest       string   `json:"manifest_revision"`
	Project        string   `json:"project_name,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

func buildArtifactManifest(buildID string, source string, description string, projectID *uint, files []GeneratedFile) BuildArtifactManifest {
	artifactFiles := generatedFilesToArtifactFiles(files)
	manifest := BuildArtifactManifest{
		BuildID:      buildID,
		Source:       source,
		Description:  strings.TrimSpace(description),
		GeneratedAt:  time.Now(),
		ProjectID:    projectID,
		Files:        artifactFiles,
		Entrypoints:  detectArtifactEntrypoints(files),
		RuntimeHints: detectArtifactRuntimeHints(files),
		Verification: map[string]interface{}{},
		Warnings:     []string{},
		Errors:       []string{},
	}
	manifest.Revision = computeArtifactRevision(artifactFiles)
	return manifest
}

func generatedFilesToArtifactFiles(files []GeneratedFile) []ArtifactFile {
	out := make([]ArtifactFile, 0, len(files))
	for _, f := range files {
		path := sanitizeArtifactPath(f.Path)
		if path == "" {
			continue
		}
		content := normalizeGeneratedFileContent(path, f.Content)
		sum := sha256.Sum256([]byte(content))
		size := f.Size
		if size == 0 {
			size = int64(len(content))
		}
		out = append(out, ArtifactFile{
			Path:     path,
			Content:  content,
			Language: f.Language,
			Size:     size,
			IsNew:    f.IsNew,
			SHA256:   hex.EncodeToString(sum[:]),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func computeArtifactRevision(files []ArtifactFile) string {
	if len(files) == 0 {
		return "empty"
	}
	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write([]byte(f.SHA256))
		h.Write([]byte{0})
		if f.Deleted {
			h.Write([]byte("deleted"))
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func detectArtifactEntrypoints(files []GeneratedFile) map[string]string {
	paths := map[string]bool{}
	for _, f := range files {
		if p := sanitizeArtifactPath(f.Path); p != "" {
			paths[p] = true
		}
	}
	entrypoints := map[string]string{}
	for _, candidate := range []string{"index.html", "public/index.html", "src/main.tsx", "src/main.ts", "src/index.tsx", "src/index.ts"} {
		if paths[candidate] {
			entrypoints["frontend"] = candidate
			break
		}
	}
	for _, candidate := range []string{"server.js", "src/server.ts", "main.go", "cmd/server/main.go", "app.py"} {
		if paths[candidate] {
			entrypoints["backend"] = candidate
			break
		}
	}
	return entrypoints
}

func detectArtifactRuntimeHints(files []GeneratedFile) map[string]string {
	paths := map[string]bool{}
	for _, f := range files {
		if p := sanitizeArtifactPath(f.Path); p != "" {
			paths[p] = true
		}
	}
	hints := map[string]string{}
	switch {
	case paths["pnpm-lock.yaml"]:
		hints["package_manager"] = "pnpm"
	case paths["yarn.lock"]:
		hints["package_manager"] = "yarn"
	case paths["package-lock.json"]:
		hints["package_manager"] = "npm"
	}
	if paths["package.json"] {
		hints["runtime"] = "node"
	}
	if paths["go.mod"] {
		hints["runtime"] = "go"
	}
	if paths["requirements.txt"] || paths["pyproject.toml"] {
		hints["runtime"] = "python"
	}
	return hints
}

func applyArtifactManifestTx(tx *gorm.DB, project *models.Project, userID uint, manifest BuildArtifactManifest, replaceMissing bool) (ApplyArtifactsResult, error) {
	if tx == nil || project == nil {
		return ApplyArtifactsResult{}, fmt.Errorf("missing transaction or project")
	}

	var existing []models.File
	if err := tx.Where("project_id = ?", project.ID).Find(&existing).Error; err != nil {
		return ApplyArtifactsResult{}, err
	}

	existingByPath := make(map[string]models.File, len(existing))
	for _, f := range existing {
		if p := sanitizeFilePath(f.Path); p != "" {
			existingByPath[p] = f
		}
	}

	seen := make(map[string]bool, len(manifest.Files))
	applied := 0
	deleted := 0

	for _, af := range manifest.Files {
		path := sanitizeArtifactPath(af.Path)
		if path == "" {
			return ApplyArtifactsResult{}, fmt.Errorf("invalid artifact path: %q", af.Path)
		}
		if seen[path] {
			return ApplyArtifactsResult{}, fmt.Errorf("duplicate artifact path: %s", path)
		}
		seen[path] = true

		if af.Deleted {
			if existing, ok := existingByPath[path]; ok {
				if err := tx.Delete(&models.File{}, existing.ID).Error; err != nil {
					return ApplyArtifactsResult{}, err
				}
				deleted++
			}
			continue
		}

		content := af.Content
		fileName := filepath.Base(path)
		mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
		if mimeType == "" {
			mimeType = "text/plain; charset=utf-8"
		}

		if existing, ok := existingByPath[path]; ok {
			updates := map[string]interface{}{
				"name":         fileName,
				"path":         path,
				"type":         "file",
				"mime_type":    mimeType,
				"content":      content,
				"size":         int64(len(content)),
				"last_edit_by": userID,
				"version":      gorm.Expr("version + 1"),
				"updated_at":   time.Now(),
			}
			if err := tx.Model(&models.File{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
				return ApplyArtifactsResult{}, err
			}
			applied++
			continue
		}

		record := models.File{
			ProjectID:  project.ID,
			Name:       fileName,
			Path:       path,
			Type:       "file",
			MimeType:   mimeType,
			Content:    content,
			Size:       int64(len(content)),
			LastEditBy: userID,
			Version:    1,
		}
		if err := tx.Create(&record).Error; err != nil {
			return ApplyArtifactsResult{}, err
		}
		applied++
	}

	if replaceMissing {
		for path, f := range existingByPath {
			if seen[path] {
				continue
			}
			if err := tx.Delete(&models.File{}, f.ID).Error; err != nil {
				return ApplyArtifactsResult{}, err
			}
			deleted++
		}
	}

	return ApplyArtifactsResult{
		ProjectID:    project.ID,
		AppliedFiles: applied,
		DeletedFiles: deleted,
		Manifest:     manifest.Revision,
		Project:      project.Name,
	}, nil
}

func sanitizeArtifactPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	trimmed = strings.TrimPrefix(trimmed, "./")
	trimmed = strings.TrimPrefix(trimmed, "/")
	return sanitizeFilePath(trimmed)
}

func createProjectForArtifactManifestTx(tx *gorm.DB, ownerID uint, buildDescription string, manifest BuildArtifactManifest) (*models.Project, error) {
	genFiles := make([]GeneratedFile, 0, len(manifest.Files))
	for _, af := range manifest.Files {
		if af.Deleted {
			continue
		}
		genFiles = append(genFiles, GeneratedFile{
			Path:     af.Path,
			Content:  af.Content,
			Language: af.Language,
			Size:     af.Size,
			IsNew:    af.IsNew,
		})
	}

	projectName := strings.TrimSpace(buildDescription)
	if projectName == "" {
		projectName = "Generated App"
	}
	if len(projectName) > 100 {
		projectName = strings.TrimSpace(projectName[:100])
	}
	if projectName == "" {
		projectName = "Generated App"
	}

	language := detectGeneratedProjectLanguage(genFiles)
	framework := detectGeneratedProjectFramework(strings.ToLower(strings.TrimSpace(manifest.RuntimeHints["frontend_framework"])))

	project := &models.Project{
		Name:          projectName,
		Description:   buildDescription,
		Language:      language,
		Framework:     framework,
		OwnerID:       ownerID,
		IsPublic:      false,
		RootDirectory: "/",
		Environment:   map[string]interface{}{},
		Dependencies:  map[string]interface{}{},
		BuildConfig: map[string]interface{}{
			"source":            "build_apply",
			"artifact_build_id": manifest.BuildID,
			"artifact_revision": manifest.Revision,
		},
	}

	if err := tx.Create(project).Error; err != nil {
		return nil, err
	}
	return project, nil
}

func manifestJSONCompact(manifest BuildArtifactManifest) string {
	b, err := json.Marshal(manifest)
	if err != nil {
		return ""
	}
	return string(b)
}
