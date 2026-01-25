// APEX.BUILD Package Manager
// Core package management interfaces and types

package packages

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"apex-build/pkg/models"

	"gorm.io/gorm"
)

// PackageType represents the type of package manager
type PackageType string

const (
	PackageTypeNPM  PackageType = "npm"
	PackageTypePyPI PackageType = "pip"
	PackageTypeGo   PackageType = "go"
)

// Package represents a package from any registry
type Package struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	Downloads   int64     `json:"downloads"`
	Homepage    string    `json:"homepage"`
	Repository  string    `json:"repository"`
	License     string    `json:"license"`
	Author      string    `json:"author"`
	Keywords    []string  `json:"keywords"`
	PublishedAt time.Time `json:"published_at"`
	PackageType PackageType `json:"package_type"`
}

// PackageDetail contains detailed information about a package
type PackageDetail struct {
	Package
	Versions        []string          `json:"versions"`
	LatestVersion   string            `json:"latest_version"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"dev_dependencies"`
	Readme          string            `json:"readme"`
	Maintainers     []Maintainer      `json:"maintainers"`
	BugTracker      string            `json:"bug_tracker"`
	Documentation   string            `json:"documentation"`
}

// Maintainer represents a package maintainer
type Maintainer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// InstalledPackage represents a package installed in a project
type InstalledPackage struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	IsDev          bool        `json:"is_dev"`
	InstalledAt    time.Time   `json:"installed_at"`
	LatestVersion  string      `json:"latest_version,omitempty"`
	UpdateAvailable bool       `json:"update_available"`
	PackageType    PackageType `json:"package_type"`
}

// PackageManager defines the interface for package management operations
type PackageManager interface {
	// Search searches for packages matching the query
	Search(query string, limit int) ([]Package, error)

	// GetPackageInfo retrieves detailed information about a package
	GetPackageInfo(name string) (*PackageDetail, error)

	// Install installs a package to a project
	Install(projectID uint, packageName, version string, isDev bool) error

	// Uninstall removes a package from a project
	Uninstall(projectID uint, packageName string) error

	// ListInstalled lists all installed packages for a project
	ListInstalled(projectID uint) ([]InstalledPackage, error)

	// UpdateDependencyFile updates the project's dependency file
	UpdateDependencyFile(projectID uint) error

	// GetType returns the package manager type
	GetType() PackageType
}

// PackageManagerService orchestrates package management across different registries
type PackageManagerService struct {
	DB       *gorm.DB
	managers map[PackageType]PackageManager
}

// NewPackageManagerService creates a new package manager service
func NewPackageManagerService(db *gorm.DB) *PackageManagerService {
	svc := &PackageManagerService{
		DB:       db,
		managers: make(map[PackageType]PackageManager),
	}

	// Register package managers
	svc.managers[PackageTypeNPM] = NewNPMManager(db)
	svc.managers[PackageTypePyPI] = NewPyPIManager(db)
	svc.managers[PackageTypeGo] = NewGoModManager(db)

	return svc
}

// GetManager returns the appropriate package manager for the given type
func (s *PackageManagerService) GetManager(pkgType PackageType) (PackageManager, error) {
	manager, ok := s.managers[pkgType]
	if !ok {
		return nil, fmt.Errorf("unsupported package type: %s", pkgType)
	}
	return manager, nil
}

// Search searches for packages across a specific registry
func (s *PackageManagerService) Search(query string, pkgType PackageType, limit int) ([]Package, error) {
	manager, err := s.GetManager(pkgType)
	if err != nil {
		return nil, err
	}
	return manager.Search(query, limit)
}

// GetPackageInfo gets detailed package information
func (s *PackageManagerService) GetPackageInfo(name string, pkgType PackageType) (*PackageDetail, error) {
	manager, err := s.GetManager(pkgType)
	if err != nil {
		return nil, err
	}
	return manager.GetPackageInfo(name)
}

// Install installs a package to a project
func (s *PackageManagerService) Install(projectID uint, packageName, version string, pkgType PackageType, isDev bool) error {
	manager, err := s.GetManager(pkgType)
	if err != nil {
		return err
	}
	return manager.Install(projectID, packageName, version, isDev)
}

// Uninstall removes a package from a project
func (s *PackageManagerService) Uninstall(projectID uint, packageName string, pkgType PackageType) error {
	manager, err := s.GetManager(pkgType)
	if err != nil {
		return err
	}
	return manager.Uninstall(projectID, packageName)
}

// ListInstalled lists all installed packages for a project
func (s *PackageManagerService) ListInstalled(projectID uint, pkgType PackageType) ([]InstalledPackage, error) {
	manager, err := s.GetManager(pkgType)
	if err != nil {
		return nil, err
	}
	return manager.ListInstalled(projectID)
}

// ListAllInstalled lists all installed packages for a project across all package managers
func (s *PackageManagerService) ListAllInstalled(projectID uint) (map[PackageType][]InstalledPackage, error) {
	// Get project to determine language
	var project models.Project
	if err := s.DB.First(&project, projectID).Error; err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	result := make(map[PackageType][]InstalledPackage)

	// Determine which package managers to check based on project language
	var managersToCheck []PackageType

	switch project.Language {
	case "javascript", "typescript":
		managersToCheck = []PackageType{PackageTypeNPM}
	case "python":
		managersToCheck = []PackageType{PackageTypePyPI}
	case "go":
		managersToCheck = []PackageType{PackageTypeGo}
	default:
		// Check all
		managersToCheck = []PackageType{PackageTypeNPM, PackageTypePyPI, PackageTypeGo}
	}

	for _, pkgType := range managersToCheck {
		packages, err := s.ListInstalled(projectID, pkgType)
		if err != nil {
			continue // Skip errors for individual managers
		}
		if len(packages) > 0 {
			result[pkgType] = packages
		}
	}

	return result, nil
}

// UpdateDependencyFile updates the project's dependency file
func (s *PackageManagerService) UpdateDependencyFile(projectID uint, pkgType PackageType) error {
	manager, err := s.GetManager(pkgType)
	if err != nil {
		return err
	}
	return manager.UpdateDependencyFile(projectID)
}

// DetectPackageType detects the package type for a project
func (s *PackageManagerService) DetectPackageType(projectID uint) (PackageType, error) {
	var project models.Project
	if err := s.DB.First(&project, projectID).Error; err != nil {
		return "", fmt.Errorf("project not found: %w", err)
	}

	switch project.Language {
	case "javascript", "typescript":
		return PackageTypeNPM, nil
	case "python":
		return PackageTypePyPI, nil
	case "go":
		return PackageTypeGo, nil
	default:
		return "", fmt.Errorf("unsupported language: %s", project.Language)
	}
}

// Helper functions for working with dependency files

// GetDependencyFile retrieves the appropriate dependency file for a project
func GetDependencyFile(db *gorm.DB, projectID uint, pkgType PackageType) (*models.File, error) {
	var fileName string
	switch pkgType {
	case PackageTypeNPM:
		fileName = "package.json"
	case PackageTypePyPI:
		fileName = "requirements.txt"
	case PackageTypeGo:
		fileName = "go.mod"
	default:
		return nil, fmt.Errorf("unsupported package type: %s", pkgType)
	}

	var file models.File
	err := db.Where("project_id = ? AND name = ?", projectID, fileName).First(&file).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &file, nil
}

// CreateDependencyFile creates a new dependency file for a project
func CreateDependencyFile(db *gorm.DB, projectID uint, pkgType PackageType) (*models.File, error) {
	var fileName, mimeType, content string

	switch pkgType {
	case PackageTypeNPM:
		fileName = "package.json"
		mimeType = "application/json"
		content = `{
  "name": "apex-project",
  "version": "1.0.0",
  "description": "",
  "main": "index.js",
  "scripts": {
    "start": "node index.js",
    "test": "echo \"Error: no test specified\" && exit 1"
  },
  "dependencies": {},
  "devDependencies": {}
}`
	case PackageTypePyPI:
		fileName = "requirements.txt"
		mimeType = "text/plain"
		content = "# Python dependencies\n"
	case PackageTypeGo:
		fileName = "go.mod"
		mimeType = "text/plain"
		content = "module main\n\ngo 1.21\n"
	default:
		return nil, fmt.Errorf("unsupported package type: %s", pkgType)
	}

	file := &models.File{
		ProjectID: projectID,
		Name:      fileName,
		Path:      "/" + fileName,
		Type:      "file",
		MimeType:  mimeType,
		Content:   content,
	}

	if err := db.Create(file).Error; err != nil {
		return nil, err
	}

	return file, nil
}

// ParsePackageJSON parses a package.json file and returns dependencies
type PackageJSON struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Description     string            `json:"description"`
	Main            string            `json:"main"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// ParsePackageJSON parses package.json content
func ParsePackageJSON(content string) (*PackageJSON, error) {
	var pkg PackageJSON
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse package.json: %w", err)
	}
	if pkg.Dependencies == nil {
		pkg.Dependencies = make(map[string]string)
	}
	if pkg.DevDependencies == nil {
		pkg.DevDependencies = make(map[string]string)
	}
	return &pkg, nil
}

// SerializePackageJSON serializes PackageJSON to string
func SerializePackageJSON(pkg *PackageJSON) (string, error) {
	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize package.json: %w", err)
	}
	return string(data), nil
}

// ParseRequirementsTxt parses requirements.txt content
func ParseRequirementsTxt(content string) map[string]string {
	deps := make(map[string]string)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle different version specifiers
		var name, version string
		for _, sep := range []string{"==", ">=", "<=", "~=", "!="} {
			if idx := strings.Index(line, sep); idx != -1 {
				name = strings.TrimSpace(line[:idx])
				version = strings.TrimSpace(line[idx:])
				break
			}
		}
		if name == "" {
			name = line
			version = ""
		}
		deps[name] = version
	}
	return deps
}

// SerializeRequirementsTxt serializes dependencies to requirements.txt format
func SerializeRequirementsTxt(deps map[string]string) string {
	var lines []string
	lines = append(lines, "# Python dependencies")
	for name, version := range deps {
		if version == "" {
			lines = append(lines, name)
		} else if strings.HasPrefix(version, "==") || strings.HasPrefix(version, ">=") ||
			strings.HasPrefix(version, "<=") || strings.HasPrefix(version, "~=") ||
			strings.HasPrefix(version, "!=") {
			lines = append(lines, name+version)
		} else {
			lines = append(lines, name+"=="+version)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// ParseGoMod parses go.mod content
type GoMod struct {
	Module  string
	Go      string
	Require map[string]string
}

// ParseGoMod parses go.mod content
func ParseGoMod(content string) *GoMod {
	mod := &GoMod{
		Require: make(map[string]string),
	}

	lines := strings.Split(content, "\n")
	inRequire := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if strings.HasPrefix(line, "module ") {
			mod.Module = strings.TrimSpace(strings.TrimPrefix(line, "module "))
		} else if strings.HasPrefix(line, "go ") {
			mod.Go = strings.TrimSpace(strings.TrimPrefix(line, "go "))
		} else if strings.HasPrefix(line, "require (") {
			inRequire = true
		} else if line == ")" {
			inRequire = false
		} else if strings.HasPrefix(line, "require ") {
			// Single line require
			parts := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(parts) >= 2 {
				mod.Require[parts[0]] = parts[1]
			}
		} else if inRequire {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				mod.Require[parts[0]] = parts[1]
			}
		}
	}

	return mod
}

// SerializeGoMod serializes GoMod to string
func SerializeGoMod(mod *GoMod) string {
	var sb strings.Builder
	sb.WriteString("module " + mod.Module + "\n\n")
	sb.WriteString("go " + mod.Go + "\n")

	if len(mod.Require) > 0 {
		sb.WriteString("\nrequire (\n")
		for pkg, version := range mod.Require {
			sb.WriteString("\t" + pkg + " " + version + "\n")
		}
		sb.WriteString(")\n")
	}

	return sb.String()
}
