// APEX.BUILD PyPI Registry Integration
// Full Python Package Index support

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
	pypiBaseURL   = "https://pypi.org"
	pypiSearchURL = "https://pypi.org/search"
)

// PyPIManager implements PackageManager for PyPI
type PyPIManager struct {
	DB     *gorm.DB
	client *http.Client
}

// NewPyPIManager creates a new PyPI package manager
func NewPyPIManager(db *gorm.DB) *PyPIManager {
	return &PyPIManager{
		DB: db,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetType returns the package manager type
func (m *PyPIManager) GetType() PackageType {
	return PackageTypePyPI
}

// PyPI API Response Types

// PyPIPackageResponse represents the response from PyPI package API
type PyPIPackageResponse struct {
	Info     PyPIPackageInfo       `json:"info"`
	Releases map[string][]PyPIFile `json:"releases"`
	URLs     []PyPIFile            `json:"urls"`
}

// PyPIPackageInfo represents package metadata
type PyPIPackageInfo struct {
	Author                 string   `json:"author"`
	AuthorEmail            string   `json:"author_email"`
	BugTrackerURL          string   `json:"bugtrack_url"`
	Classifiers            []string `json:"classifiers"`
	Description            string   `json:"description"`
	DescriptionContentType string   `json:"description_content_type"`
	DocsURL                string   `json:"docs_url"`
	DownloadURL            string   `json:"download_url"`
	Downloads              struct {
		LastDay   int `json:"last_day"`
		LastMonth int `json:"last_month"`
		LastWeek  int `json:"last_week"`
	} `json:"downloads"`
	Homepage           string            `json:"home_page"`
	Keywords           string            `json:"keywords"`
	License            string            `json:"license"`
	Maintainer         string            `json:"maintainer"`
	MaintainerEmail    string            `json:"maintainer_email"`
	Name               string            `json:"name"`
	PackageURL         string            `json:"package_url"`
	Platform           string            `json:"platform"`
	ProjectURL         string            `json:"project_url"`
	ProjectURLs        map[string]string `json:"project_urls"`
	ReleaseURL         string            `json:"release_url"`
	RequiresDist       []string          `json:"requires_dist"`
	RequiresPython     string            `json:"requires_python"`
	Summary            string            `json:"summary"`
	Version            string            `json:"version"`
	Yanked             bool              `json:"yanked"`
	YankedReason       string            `json:"yanked_reason"`
}

// PyPIFile represents a release file
type PyPIFile struct {
	CommentText    string `json:"comment_text"`
	Digests        struct {
		Blake2b256 string `json:"blake2b_256"`
		MD5        string `json:"md5"`
		SHA256     string `json:"sha256"`
	} `json:"digests"`
	Downloads         int     `json:"downloads"`
	Filename          string  `json:"filename"`
	HasSig            bool    `json:"has_sig"`
	MD5Digest         string  `json:"md5_digest"`
	PackageType       string  `json:"packagetype"`
	PythonVersion     string  `json:"python_version"`
	RequiresPython    string  `json:"requires_python"`
	Size              int64   `json:"size"`
	UploadTime        string  `json:"upload_time"`
	UploadTimeISO8601 string  `json:"upload_time_iso_8601"`
	URL               string  `json:"url"`
	Yanked            bool    `json:"yanked"`
	YankedReason      string  `json:"yanked_reason"`
}

// PyPISearchResult represents a package in search results
type PyPISearchResult struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Summary     string `json:"summary"`
}

// Search searches for PyPI packages
// Note: PyPI doesn't have a public search API, so we use the JSON API for known packages
// or search via XML-RPC (limited) or web scraping (not implemented here)
// For a real implementation, you might use the PyPI Simple API or a third-party search service
func (m *PyPIManager) Search(query string, limit int) ([]Package, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// PyPI doesn't have a simple search API
	// We'll try to get package info directly if the query looks like a package name
	// For a production app, you'd want to use a search service or implement XML-RPC

	// Try direct package lookup first
	packages := make([]Package, 0)

	// Check if query looks like a package name (no spaces, alphanumeric with dashes/underscores)
	if isValidPackageName(query) {
		info, err := m.GetPackageInfo(query)
		if err == nil {
			packages = append(packages, info.Package)
			return packages, nil
		}
	}

	// For partial matches, we'll try common variations
	// This is a workaround since PyPI doesn't have a public search API
	variations := []string{
		query,
		strings.ReplaceAll(query, " ", "-"),
		strings.ReplaceAll(query, " ", "_"),
		strings.ToLower(strings.ReplaceAll(query, " ", "-")),
	}

	seen := make(map[string]bool)
	for _, name := range variations {
		if seen[name] {
			continue
		}
		seen[name] = true

		info, err := m.GetPackageInfo(name)
		if err == nil {
			packages = append(packages, info.Package)
			if len(packages) >= limit {
				break
			}
		}
	}

	// Try common popular packages that match the query
	popularPackages := getPopularPyPIPackages(query)
	for _, name := range popularPackages {
		if seen[name] {
			continue
		}
		seen[name] = true

		info, err := m.GetPackageInfo(name)
		if err == nil {
			packages = append(packages, info.Package)
			if len(packages) >= limit {
				break
			}
		}
	}

	return packages, nil
}

// getPopularPyPIPackages returns popular packages matching a query
func getPopularPyPIPackages(query string) []string {
	query = strings.ToLower(query)

	// Map of keywords to popular packages
	keywordPackages := map[string][]string{
		"http":     {"requests", "httpx", "aiohttp", "urllib3", "httplib2"},
		"web":      {"django", "flask", "fastapi", "tornado", "bottle", "pyramid"},
		"api":      {"fastapi", "flask-restful", "django-rest-framework", "connexion"},
		"database": {"sqlalchemy", "psycopg2", "pymongo", "redis", "pymysql"},
		"sql":      {"sqlalchemy", "sqlite3", "psycopg2", "pymysql", "sqlmodel"},
		"data":     {"pandas", "numpy", "scipy", "polars", "dask"},
		"ml":       {"scikit-learn", "tensorflow", "pytorch", "keras", "xgboost"},
		"ai":       {"openai", "langchain", "transformers", "anthropic"},
		"test":     {"pytest", "unittest", "nose", "coverage", "tox"},
		"async":    {"asyncio", "aiohttp", "httpx", "anyio", "trio"},
		"json":     {"json", "orjson", "ujson", "simplejson"},
		"yaml":     {"pyyaml", "ruamel.yaml"},
		"cli":      {"click", "typer", "argparse", "fire", "rich"},
		"log":      {"logging", "loguru", "structlog"},
		"image":    {"pillow", "opencv-python", "imageio", "scikit-image"},
		"parse":    {"beautifulsoup4", "lxml", "html5lib", "parsel"},
		"scrape":   {"beautifulsoup4", "scrapy", "selenium", "playwright"},
		"crypto":   {"cryptography", "pycryptodome", "hashlib"},
		"aws":      {"boto3", "botocore", "aiobotocore"},
		"cloud":    {"boto3", "google-cloud", "azure"},
		"env":      {"python-dotenv", "environs", "pydantic-settings"},
		"config":   {"python-dotenv", "configparser", "toml", "pydantic"},
		"type":     {"pydantic", "mypy", "typing-extensions", "typeguard"},
		"format":   {"black", "autopep8", "yapf", "isort"},
		"lint":     {"pylint", "flake8", "ruff", "bandit"},
	}

	var results []string
	for keyword, packages := range keywordPackages {
		if strings.Contains(query, keyword) || strings.Contains(keyword, query) {
			results = append(results, packages...)
		}
	}

	return results
}

// isValidPackageName checks if a string looks like a valid package name
func isValidPackageName(name string) bool {
	if name == "" || strings.Contains(name, " ") {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

// GetPackageInfo retrieves detailed information about a PyPI package
func (m *PyPIManager) GetPackageInfo(name string) (*PackageDetail, error) {
	// Build package info URL
	pkgURL := fmt.Sprintf("%s/pypi/%s/json", pypiBaseURL, url.PathEscape(name))

	resp, err := m.client.Get(pkgURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get pypi package info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("package not found: %s", name)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pypi request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var pypiPkg PyPIPackageResponse
	if err := json.NewDecoder(resp.Body).Decode(&pypiPkg); err != nil {
		return nil, fmt.Errorf("failed to decode pypi package info: %w", err)
	}

	// Get version list
	versions := make([]string, 0, len(pypiPkg.Releases))
	for v, files := range pypiPkg.Releases {
		// Skip yanked versions
		allYanked := true
		for _, f := range files {
			if !f.Yanked {
				allYanked = false
				break
			}
		}
		if !allYanked && len(files) > 0 {
			versions = append(versions, v)
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})

	// Parse dependencies from requires_dist
	deps := make(map[string]string)
	devDeps := make(map[string]string)
	for _, req := range pypiPkg.Info.RequiresDist {
		name, version, isDev := parseRequirement(req)
		if name != "" {
			if isDev {
				devDeps[name] = version
			} else {
				deps[name] = version
			}
		}
	}

	// Parse keywords
	keywords := strings.Split(pypiPkg.Info.Keywords, ",")
	for i := range keywords {
		keywords[i] = strings.TrimSpace(keywords[i])
	}

	// Get maintainers
	maintainers := []Maintainer{}
	if pypiPkg.Info.Author != "" {
		maintainers = append(maintainers, Maintainer{
			Name:  pypiPkg.Info.Author,
			Email: pypiPkg.Info.AuthorEmail,
		})
	}
	if pypiPkg.Info.Maintainer != "" && pypiPkg.Info.Maintainer != pypiPkg.Info.Author {
		maintainers = append(maintainers, Maintainer{
			Name:  pypiPkg.Info.Maintainer,
			Email: pypiPkg.Info.MaintainerEmail,
		})
	}

	// Get repository URL from project URLs
	repository := ""
	if urls := pypiPkg.Info.ProjectURLs; urls != nil {
		for key, url := range urls {
			key = strings.ToLower(key)
			if strings.Contains(key, "source") || strings.Contains(key, "repository") ||
				strings.Contains(key, "github") || strings.Contains(key, "code") {
				repository = url
				break
			}
		}
	}

	// Get documentation URL
	docsURL := pypiPkg.Info.DocsURL
	if docsURL == "" && pypiPkg.Info.ProjectURLs != nil {
		for key, url := range pypiPkg.Info.ProjectURLs {
			key = strings.ToLower(key)
			if strings.Contains(key, "doc") {
				docsURL = url
				break
			}
		}
	}

	detail := &PackageDetail{
		Package: Package{
			Name:        pypiPkg.Info.Name,
			Version:     pypiPkg.Info.Version,
			Description: pypiPkg.Info.Summary,
			Downloads:   int64(pypiPkg.Info.Downloads.LastMonth),
			Homepage:    pypiPkg.Info.Homepage,
			Repository:  repository,
			License:     pypiPkg.Info.License,
			Author:      pypiPkg.Info.Author,
			Keywords:    keywords,
			PackageType: PackageTypePyPI,
		},
		Versions:        versions,
		LatestVersion:   pypiPkg.Info.Version,
		Dependencies:    deps,
		DevDependencies: devDeps,
		Readme:          pypiPkg.Info.Description,
		Maintainers:     maintainers,
		BugTracker:      pypiPkg.Info.BugTrackerURL,
		Documentation:   docsURL,
	}

	return detail, nil
}

// parseRequirement parses a PEP 508 requirement string
func parseRequirement(req string) (name, version string, isDev bool) {
	// Remove any markers (conditions after ;)
	if idx := strings.Index(req, ";"); idx != -1 {
		marker := strings.ToLower(req[idx:])
		req = req[:idx]
		// Check if it's a dev/test dependency based on marker
		isDev = strings.Contains(marker, "extra") &&
			(strings.Contains(marker, "dev") || strings.Contains(marker, "test"))
	}

	req = strings.TrimSpace(req)

	// Parse version specifier
	for _, op := range []string{">=", "<=", "==", "!=", "~=", ">", "<"} {
		if idx := strings.Index(req, op); idx != -1 {
			name = strings.TrimSpace(req[:idx])
			version = strings.TrimSpace(req[idx:])
			return
		}
	}

	// No version specifier
	if idx := strings.Index(req, "["); idx != -1 {
		name = strings.TrimSpace(req[:idx])
	} else {
		name = strings.TrimSpace(req)
	}
	return
}

// Install installs a PyPI package to a project
func (m *PyPIManager) Install(projectID uint, packageName, version string, isDev bool) error {
	// Get or create requirements.txt
	file, err := GetDependencyFile(m.DB, projectID, PackageTypePyPI)
	if err != nil {
		return fmt.Errorf("failed to get dependency file: %w", err)
	}

	if file == nil {
		file, err = CreateDependencyFile(m.DB, projectID, PackageTypePyPI)
		if err != nil {
			return fmt.Errorf("failed to create requirements.txt: %w", err)
		}
	}

	// Parse existing requirements.txt
	deps := ParseRequirementsTxt(file.Content)

	// If no version specified, get the latest
	if version == "" || version == "latest" {
		info, err := m.GetPackageInfo(packageName)
		if err != nil {
			return fmt.Errorf("failed to get package info: %w", err)
		}
		version = "==" + info.LatestVersion
	} else if !strings.HasPrefix(version, "==") && !strings.HasPrefix(version, ">=") &&
		!strings.HasPrefix(version, "<=") && !strings.HasPrefix(version, "~=") {
		version = "==" + version
	}

	// Add package
	deps[packageName] = version

	// Serialize and update file
	content := SerializeRequirementsTxt(deps)
	file.Content = content

	if err := m.DB.Save(file).Error; err != nil {
		return fmt.Errorf("failed to save requirements.txt: %w", err)
	}

	return nil
}

// Uninstall removes a PyPI package from a project
func (m *PyPIManager) Uninstall(projectID uint, packageName string) error {
	// Get requirements.txt
	file, err := GetDependencyFile(m.DB, projectID, PackageTypePyPI)
	if err != nil {
		return fmt.Errorf("failed to get dependency file: %w", err)
	}

	if file == nil {
		return fmt.Errorf("requirements.txt not found")
	}

	// Parse existing requirements.txt
	deps := ParseRequirementsTxt(file.Content)

	// Find and remove package (case-insensitive)
	found := false
	for name := range deps {
		if strings.EqualFold(name, packageName) {
			delete(deps, name)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("package %s is not installed", packageName)
	}

	// Serialize and update file
	content := SerializeRequirementsTxt(deps)
	file.Content = content

	if err := m.DB.Save(file).Error; err != nil {
		return fmt.Errorf("failed to save requirements.txt: %w", err)
	}

	return nil
}

// ListInstalled lists all installed PyPI packages for a project
func (m *PyPIManager) ListInstalled(projectID uint) ([]InstalledPackage, error) {
	// Get requirements.txt
	file, err := GetDependencyFile(m.DB, projectID, PackageTypePyPI)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependency file: %w", err)
	}

	if file == nil {
		return []InstalledPackage{}, nil
	}

	// Parse existing requirements.txt
	deps := ParseRequirementsTxt(file.Content)

	packages := make([]InstalledPackage, 0, len(deps))

	for name, version := range deps {
		installed := InstalledPackage{
			Name:        name,
			Version:     cleanPyPIVersion(version),
			IsDev:       false, // requirements.txt doesn't distinguish
			PackageType: PackageTypePyPI,
		}

		// Check for updates
		if info, err := m.GetPackageInfo(name); err == nil {
			installed.LatestVersion = info.LatestVersion
			installed.UpdateAvailable = compareVersions(info.LatestVersion, cleanPyPIVersion(version)) > 0
		}

		packages = append(packages, installed)
	}

	return packages, nil
}

// UpdateDependencyFile updates all packages to their latest versions
func (m *PyPIManager) UpdateDependencyFile(projectID uint) error {
	// Get requirements.txt
	file, err := GetDependencyFile(m.DB, projectID, PackageTypePyPI)
	if err != nil {
		return fmt.Errorf("failed to get dependency file: %w", err)
	}

	if file == nil {
		return fmt.Errorf("requirements.txt not found")
	}

	// Parse existing requirements.txt
	deps := ParseRequirementsTxt(file.Content)

	// Update each package to latest version
	for name := range deps {
		if info, err := m.GetPackageInfo(name); err == nil {
			deps[name] = "==" + info.LatestVersion
		}
	}

	// Serialize and update file
	content := SerializeRequirementsTxt(deps)
	file.Content = content

	if err := m.DB.Save(file).Error; err != nil {
		return fmt.Errorf("failed to save requirements.txt: %w", err)
	}

	return nil
}

// cleanPyPIVersion removes version specifiers
func cleanPyPIVersion(version string) string {
	version = strings.TrimPrefix(version, "==")
	version = strings.TrimPrefix(version, ">=")
	version = strings.TrimPrefix(version, "<=")
	version = strings.TrimPrefix(version, "~=")
	version = strings.TrimPrefix(version, "!=")
	version = strings.TrimPrefix(version, ">")
	version = strings.TrimPrefix(version, "<")
	return strings.TrimSpace(version)
}
