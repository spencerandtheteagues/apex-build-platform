// APEX.BUILD Go Modules Integration
// Go package management support via pkg.go.dev and proxy.golang.org

package packages

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	goProxyURL    = "https://proxy.golang.org"
	goSearchURL   = "https://pkg.go.dev/search"
	goModuleURL   = "https://pkg.go.dev"
)

// GoModManager implements PackageManager for Go modules
type GoModManager struct {
	DB     *gorm.DB
	client *http.Client
}

// NewGoModManager creates a new Go modules package manager
func NewGoModManager(db *gorm.DB) *GoModManager {
	return &GoModManager{
		DB: db,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetType returns the package manager type
func (m *GoModManager) GetType() PackageType {
	return PackageTypeGo
}

// GoProxyVersionResponse represents the response from Go proxy version list
type GoProxyVersionResponse struct {
	Version string    `json:"Version"`
	Time    time.Time `json:"Time"`
}

// GoModuleInfo represents module info from Go proxy
type GoModuleInfo struct {
	Version string    `json:"Version"`
	Time    time.Time `json:"Time"`
}

// Search searches for Go packages
// Note: pkg.go.dev doesn't have a public API for search
// We'll use known popular packages and direct module lookup
func (m *GoModManager) Search(query string, limit int) ([]Package, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	packages := make([]Package, 0)
	seen := make(map[string]bool)

	// Try direct module lookup first
	query = strings.TrimSpace(query)
	if isValidGoModule(query) {
		info, err := m.GetPackageInfo(query)
		if err == nil {
			packages = append(packages, info.Package)
			seen[query] = true
		}
	}

	// Get popular packages matching the query
	popularPackages := getPopularGoPackages(query)
	for _, modulePath := range popularPackages {
		if seen[modulePath] {
			continue
		}
		seen[modulePath] = true

		info, err := m.GetPackageInfo(modulePath)
		if err == nil {
			packages = append(packages, info.Package)
			if len(packages) >= limit {
				break
			}
		}
	}

	return packages, nil
}

// getPopularGoPackages returns popular Go packages matching a query
func getPopularGoPackages(query string) []string {
	query = strings.ToLower(query)

	// Map of keywords to popular Go packages
	keywordPackages := map[string][]string{
		"http": {
			"github.com/gin-gonic/gin",
			"github.com/gorilla/mux",
			"github.com/labstack/echo/v4",
			"github.com/gofiber/fiber/v2",
			"github.com/go-chi/chi/v5",
			"net/http",
		},
		"web": {
			"github.com/gin-gonic/gin",
			"github.com/gorilla/mux",
			"github.com/labstack/echo/v4",
			"github.com/gofiber/fiber/v2",
		},
		"api": {
			"github.com/gin-gonic/gin",
			"github.com/go-chi/chi/v5",
			"github.com/grpc/grpc-go",
			"google.golang.org/grpc",
		},
		"database": {
			"gorm.io/gorm",
			"gorm.io/driver/postgres",
			"gorm.io/driver/mysql",
			"gorm.io/driver/sqlite",
			"github.com/jmoiron/sqlx",
			"github.com/lib/pq",
		},
		"sql": {
			"gorm.io/gorm",
			"github.com/jmoiron/sqlx",
			"github.com/lib/pq",
			"github.com/go-sql-driver/mysql",
		},
		"orm": {
			"gorm.io/gorm",
			"github.com/ent/ent",
			"github.com/go-gorp/gorp/v3",
		},
		"json": {
			"encoding/json",
			"github.com/json-iterator/go",
			"github.com/goccy/go-json",
			"github.com/tidwall/gjson",
		},
		"yaml": {
			"gopkg.in/yaml.v3",
			"github.com/go-yaml/yaml",
		},
		"cli": {
			"github.com/spf13/cobra",
			"github.com/urfave/cli/v2",
			"github.com/alecthomas/kong",
		},
		"config": {
			"github.com/spf13/viper",
			"github.com/joho/godotenv",
			"github.com/kelseyhightower/envconfig",
		},
		"log": {
			"github.com/sirupsen/logrus",
			"go.uber.org/zap",
			"github.com/rs/zerolog",
			"log/slog",
		},
		"test": {
			"github.com/stretchr/testify",
			"github.com/onsi/ginkgo/v2",
			"github.com/onsi/gomega",
		},
		"uuid": {
			"github.com/google/uuid",
			"github.com/gofrs/uuid",
		},
		"jwt": {
			"github.com/golang-jwt/jwt/v5",
			"github.com/lestrrat-go/jwx/v2",
		},
		"auth": {
			"github.com/golang-jwt/jwt/v5",
			"golang.org/x/oauth2",
			"github.com/casbin/casbin/v2",
		},
		"websocket": {
			"github.com/gorilla/websocket",
			"nhooyr.io/websocket",
		},
		"redis": {
			"github.com/redis/go-redis/v9",
			"github.com/go-redis/redis/v8",
		},
		"mongo": {
			"go.mongodb.org/mongo-driver",
		},
		"aws": {
			"github.com/aws/aws-sdk-go-v2",
			"github.com/aws/aws-sdk-go",
		},
		"cloud": {
			"cloud.google.com/go",
			"github.com/aws/aws-sdk-go-v2",
			"github.com/Azure/azure-sdk-for-go",
		},
		"crypto": {
			"golang.org/x/crypto",
			"crypto",
		},
		"time": {
			"time",
			"github.com/jinzhu/now",
		},
		"error": {
			"github.com/pkg/errors",
			"errors",
		},
		"gin": {
			"github.com/gin-gonic/gin",
			"github.com/gin-contrib/cors",
			"github.com/gin-contrib/sessions",
		},
		"gorm": {
			"gorm.io/gorm",
			"gorm.io/driver/postgres",
			"gorm.io/driver/mysql",
		},
		"fiber": {
			"github.com/gofiber/fiber/v2",
		},
		"echo": {
			"github.com/labstack/echo/v4",
		},
		"context": {
			"context",
		},
		"sync": {
			"sync",
			"golang.org/x/sync",
		},
		"image": {
			"image",
			"github.com/disintegration/imaging",
		},
		"pdf": {
			"github.com/jung-kurt/gofpdf",
			"github.com/pdfcpu/pdfcpu",
		},
		"excel": {
			"github.com/xuri/excelize/v2",
		},
		"email": {
			"github.com/jordan-wright/email",
			"gopkg.in/gomail.v2",
		},
		"validate": {
			"github.com/go-playground/validator/v10",
		},
	}

	var results []string
	for keyword, packages := range keywordPackages {
		if strings.Contains(query, keyword) || strings.Contains(keyword, query) {
			results = append(results, packages...)
		}
	}

	// Add some always-suggested packages if query is short
	if len(query) <= 3 {
		common := []string{
			"github.com/gin-gonic/gin",
			"gorm.io/gorm",
			"github.com/spf13/cobra",
			"github.com/stretchr/testify",
			"go.uber.org/zap",
			"github.com/google/uuid",
		}
		results = append(results, common...)
	}

	return results
}

// isValidGoModule checks if a string looks like a valid Go module path
func isValidGoModule(path string) bool {
	if path == "" {
		return false
	}
	// Basic check: should contain at least one slash or be a standard library path
	return strings.Contains(path, "/") || strings.Contains(path, ".")
}

// GetPackageInfo retrieves detailed information about a Go module
func (m *GoModManager) GetPackageInfo(modulePath string) (*PackageDetail, error) {
	// Normalize module path
	modulePath = strings.TrimPrefix(modulePath, "https://")
	modulePath = strings.TrimPrefix(modulePath, "http://")
	modulePath = strings.TrimSuffix(modulePath, "/")

	// Get latest version from Go proxy
	latestVersion, versionTime, err := m.getLatestVersion(modulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get module info: %w", err)
	}

	// Get all versions
	versions, err := m.getVersionList(modulePath)
	if err != nil {
		versions = []string{latestVersion}
	}

	// Sort versions (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})

	// Parse module info
	detail := &PackageDetail{
		Package: Package{
			Name:        modulePath,
			Version:     latestVersion,
			Description: getGoModuleDescription(modulePath),
			Homepage:    getGoModuleHomepage(modulePath),
			Repository:  getGoModuleRepository(modulePath),
			License:     "", // Would need to fetch from repo
			Author:      getGoModuleAuthor(modulePath),
			Keywords:    getGoModuleKeywords(modulePath),
			PublishedAt: versionTime,
			PackageType: PackageTypeGo,
		},
		Versions:        versions,
		LatestVersion:   latestVersion,
		Dependencies:    make(map[string]string), // Would need to parse go.mod
		DevDependencies: make(map[string]string),
		Readme:          "",
		Maintainers:     []Maintainer{},
		Documentation:   fmt.Sprintf("https://pkg.go.dev/%s", modulePath),
	}

	return detail, nil
}

// getLatestVersion gets the latest version of a module from Go proxy
func (m *GoModManager) getLatestVersion(modulePath string) (string, time.Time, error) {
	// Try to get @latest version
	infoURL := fmt.Sprintf("%s/%s/@latest", goProxyURL, url.PathEscape(modulePath))

	resp, err := m.client.Get(infoURL)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", time.Time{}, fmt.Errorf("module not found: %s", modulePath)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("go proxy request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var info GoModuleInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", time.Time{}, err
	}

	return info.Version, info.Time, nil
}

// getVersionList gets all versions of a module from Go proxy
func (m *GoModManager) getVersionList(modulePath string) ([]string, error) {
	listURL := fmt.Sprintf("%s/%s/@v/list", goProxyURL, url.PathEscape(modulePath))

	resp, err := m.client.Get(listURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get version list")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	versions := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			versions = append(versions, line)
		}
	}

	return versions, nil
}

// Helper functions to extract info from module path

func getGoModuleDescription(modulePath string) string {
	// Generate a basic description from the module path
	parts := strings.Split(modulePath, "/")
	if len(parts) >= 2 {
		return fmt.Sprintf("Go module: %s", parts[len(parts)-1])
	}
	return fmt.Sprintf("Go module: %s", modulePath)
}

func getGoModuleHomepage(modulePath string) string {
	if strings.HasPrefix(modulePath, "github.com/") {
		return "https://" + modulePath
	}
	if strings.HasPrefix(modulePath, "gitlab.com/") {
		return "https://" + modulePath
	}
	if strings.HasPrefix(modulePath, "bitbucket.org/") {
		return "https://" + modulePath
	}
	return fmt.Sprintf("https://pkg.go.dev/%s", modulePath)
}

func getGoModuleRepository(modulePath string) string {
	if strings.HasPrefix(modulePath, "github.com/") {
		return "https://" + modulePath
	}
	if strings.HasPrefix(modulePath, "gitlab.com/") {
		return "https://" + modulePath
	}
	if strings.HasPrefix(modulePath, "bitbucket.org/") {
		return "https://" + modulePath
	}
	return ""
}

func getGoModuleAuthor(modulePath string) string {
	if strings.HasPrefix(modulePath, "github.com/") {
		parts := strings.Split(modulePath, "/")
		if len(parts) >= 2 {
			return parts[1]
		}
	}
	if strings.HasPrefix(modulePath, "golang.org/") || strings.HasPrefix(modulePath, "google.golang.org/") {
		return "Google"
	}
	return ""
}

func getGoModuleKeywords(modulePath string) []string {
	keywords := []string{"go", "golang", "module"}

	// Add keywords based on module path
	lower := strings.ToLower(modulePath)
	keywordMap := map[string]string{
		"gin":       "web-framework",
		"gorm":      "orm",
		"mux":       "router",
		"echo":      "web-framework",
		"fiber":     "web-framework",
		"cobra":     "cli",
		"viper":     "config",
		"zap":       "logging",
		"logrus":    "logging",
		"testify":   "testing",
		"jwt":       "authentication",
		"websocket": "websocket",
		"redis":     "database",
		"mongo":     "database",
		"postgres":  "database",
		"mysql":     "database",
		"grpc":      "rpc",
		"http":      "http",
		"uuid":      "uuid",
		"validator": "validation",
	}

	for key, keyword := range keywordMap {
		if strings.Contains(lower, key) {
			keywords = append(keywords, keyword)
		}
	}

	return keywords
}

// Install installs a Go module to a project
func (m *GoModManager) Install(projectID uint, packageName, version string, isDev bool) error {
	// Get or create go.mod
	file, err := GetDependencyFile(m.DB, projectID, PackageTypeGo)
	if err != nil {
		return fmt.Errorf("failed to get dependency file: %w", err)
	}

	if file == nil {
		file, err = CreateDependencyFile(m.DB, projectID, PackageTypeGo)
		if err != nil {
			return fmt.Errorf("failed to create go.mod: %w", err)
		}
	}

	// Parse existing go.mod
	mod := ParseGoMod(file.Content)

	// If no version specified, get the latest
	if version == "" || version == "latest" {
		info, err := m.GetPackageInfo(packageName)
		if err != nil {
			return fmt.Errorf("failed to get package info: %w", err)
		}
		version = info.LatestVersion
	}

	// Add to require
	mod.Require[packageName] = version

	// Serialize and update file
	content := SerializeGoMod(mod)
	file.Content = content

	if err := m.DB.Save(file).Error; err != nil {
		return fmt.Errorf("failed to save go.mod: %w", err)
	}

	return nil
}

// Uninstall removes a Go module from a project
func (m *GoModManager) Uninstall(projectID uint, packageName string) error {
	// Get go.mod
	file, err := GetDependencyFile(m.DB, projectID, PackageTypeGo)
	if err != nil {
		return fmt.Errorf("failed to get dependency file: %w", err)
	}

	if file == nil {
		return fmt.Errorf("go.mod not found")
	}

	// Parse existing go.mod
	mod := ParseGoMod(file.Content)

	// Check if module exists
	if _, ok := mod.Require[packageName]; !ok {
		return fmt.Errorf("module %s is not installed", packageName)
	}

	// Remove module
	delete(mod.Require, packageName)

	// Serialize and update file
	content := SerializeGoMod(mod)
	file.Content = content

	if err := m.DB.Save(file).Error; err != nil {
		return fmt.Errorf("failed to save go.mod: %w", err)
	}

	return nil
}

// ListInstalled lists all installed Go modules for a project
func (m *GoModManager) ListInstalled(projectID uint) ([]InstalledPackage, error) {
	// Get go.mod
	file, err := GetDependencyFile(m.DB, projectID, PackageTypeGo)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependency file: %w", err)
	}

	if file == nil {
		return []InstalledPackage{}, nil
	}

	// Parse existing go.mod
	mod := ParseGoMod(file.Content)

	packages := make([]InstalledPackage, 0, len(mod.Require))

	for name, version := range mod.Require {
		installed := InstalledPackage{
			Name:        name,
			Version:     version,
			IsDev:       false, // Go doesn't distinguish dev dependencies in go.mod
			PackageType: PackageTypeGo,
		}

		// Check for updates
		if info, err := m.GetPackageInfo(name); err == nil {
			installed.LatestVersion = info.LatestVersion
			installed.UpdateAvailable = compareVersions(info.LatestVersion, version) > 0
		}

		packages = append(packages, installed)
	}

	return packages, nil
}

// UpdateDependencyFile updates all modules to their latest versions
func (m *GoModManager) UpdateDependencyFile(projectID uint) error {
	// Get go.mod
	file, err := GetDependencyFile(m.DB, projectID, PackageTypeGo)
	if err != nil {
		return fmt.Errorf("failed to get dependency file: %w", err)
	}

	if file == nil {
		return fmt.Errorf("go.mod not found")
	}

	// Parse existing go.mod
	mod := ParseGoMod(file.Content)

	// Update each module to latest version
	for name := range mod.Require {
		if info, err := m.GetPackageInfo(name); err == nil {
			mod.Require[name] = info.LatestVersion
		}
	}

	// Serialize and update file
	content := SerializeGoMod(mod)
	file.Content = content

	if err := m.DB.Save(file).Error; err != nil {
		return fmt.Errorf("failed to save go.mod: %w", err)
	}

	return nil
}
