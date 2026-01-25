// APEX.BUILD NPM Registry Integration
// Full NPM package registry support

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
	npmRegistryURL = "https://registry.npmjs.org"
	npmSearchURL   = "https://registry.npmjs.org/-/v1/search"
)

// NPMManager implements PackageManager for NPM
type NPMManager struct {
	DB     *gorm.DB
	client *http.Client
}

// NewNPMManager creates a new NPM package manager
func NewNPMManager(db *gorm.DB) *NPMManager {
	return &NPMManager{
		DB: db,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetType returns the package manager type
func (m *NPMManager) GetType() PackageType {
	return PackageTypeNPM
}

// NPM API Response Types

// NPMSearchResponse represents the response from NPM search API
type NPMSearchResponse struct {
	Objects []NPMSearchObject `json:"objects"`
	Total   int               `json:"total"`
	Time    string            `json:"time"`
}

// NPMSearchObject represents a single search result
type NPMSearchObject struct {
	Package     NPMSearchPackage `json:"package"`
	Score       NPMScore         `json:"score"`
	SearchScore float64          `json:"searchScore"`
}

// NPMSearchPackage represents package info in search results
type NPMSearchPackage struct {
	Name        string            `json:"name"`
	Scope       string            `json:"scope"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Keywords    []string          `json:"keywords"`
	Date        time.Time         `json:"date"`
	Links       NPMLinks          `json:"links"`
	Author      NPMAuthor         `json:"author"`
	Publisher   NPMPublisher      `json:"publisher"`
	Maintainers []NPMMaintainer   `json:"maintainers"`
}

// NPMScore represents the package quality scores
type NPMScore struct {
	Final  float64 `json:"final"`
	Detail struct {
		Quality     float64 `json:"quality"`
		Popularity  float64 `json:"popularity"`
		Maintenance float64 `json:"maintenance"`
	} `json:"detail"`
}

// NPMLinks represents package links
type NPMLinks struct {
	NPM        string `json:"npm"`
	Homepage   string `json:"homepage"`
	Repository string `json:"repository"`
	Bugs       string `json:"bugs"`
}

// NPMAuthor represents a package author
type NPMAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	URL   string `json:"url"`
}

// NPMPublisher represents a package publisher
type NPMPublisher struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

// NPMMaintainer represents a package maintainer
type NPMMaintainer struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

// NPMPackageDetail represents detailed package info
type NPMPackageDetail struct {
	ID              string                       `json:"_id"`
	Rev             string                       `json:"_rev"`
	Name            string                       `json:"name"`
	Description     string                       `json:"description"`
	DistTags        map[string]string            `json:"dist-tags"`
	Versions        map[string]NPMVersionDetail  `json:"versions"`
	Maintainers     []NPMMaintainer              `json:"maintainers"`
	Time            map[string]string            `json:"time"`
	Author          NPMAuthor                    `json:"author"`
	Repository      NPMRepository                `json:"repository"`
	Readme          string                       `json:"readme"`
	ReadmeFilename  string                       `json:"readmeFilename"`
	Homepage        string                       `json:"homepage"`
	Keywords        []string                     `json:"keywords"`
	Bugs            NPMBugs                      `json:"bugs"`
	License         interface{}                  `json:"license"` // Can be string or object
}

// NPMVersionDetail represents version-specific package info
type NPMVersionDetail struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Description     string            `json:"description"`
	Main            string            `json:"main"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	PeerDependencies map[string]string `json:"peerDependencies"`
	Repository      NPMRepository     `json:"repository"`
	Author          NPMAuthor         `json:"author"`
	License         interface{}       `json:"license"`
	Dist            NPMDist           `json:"dist"`
}

// NPMRepository represents repository info
type NPMRepository struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// NPMBugs represents bug tracker info
type NPMBugs struct {
	URL   string `json:"url"`
	Email string `json:"email"`
}

// NPMDist represents distribution info
type NPMDist struct {
	Integrity    string `json:"integrity"`
	Shasum       string `json:"shasum"`
	Tarball      string `json:"tarball"`
	FileCount    int    `json:"fileCount"`
	UnpackedSize int64  `json:"unpackedSize"`
	Signatures   []struct {
		Keyid string `json:"keyid"`
		Sig   string `json:"sig"`
	} `json:"signatures"`
}

// NPMDownloadResponse represents download statistics
type NPMDownloadResponse struct {
	Downloads int    `json:"downloads"`
	Start     string `json:"start"`
	End       string `json:"end"`
	Package   string `json:"package"`
}

// Search searches for NPM packages
func (m *NPMManager) Search(query string, limit int) ([]Package, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Build search URL
	searchURL := fmt.Sprintf("%s?text=%s&size=%d", npmSearchURL, url.QueryEscape(query), limit)

	resp, err := m.client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search npm registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("npm search failed with status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp NPMSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode npm search response: %w", err)
	}

	packages := make([]Package, 0, len(searchResp.Objects))
	for _, obj := range searchResp.Objects {
		pkg := Package{
			Name:        obj.Package.Name,
			Version:     obj.Package.Version,
			Description: obj.Package.Description,
			Homepage:    obj.Package.Links.Homepage,
			Repository:  obj.Package.Links.Repository,
			Author:      obj.Package.Author.Name,
			Keywords:    obj.Package.Keywords,
			PublishedAt: obj.Package.Date,
			PackageType: PackageTypeNPM,
		}

		// Get download count (optional, may slow down search)
		downloads, _ := m.getDownloadCount(obj.Package.Name)
		pkg.Downloads = downloads

		packages = append(packages, pkg)
	}

	return packages, nil
}

// getDownloadCount fetches download statistics for a package
func (m *NPMManager) getDownloadCount(packageName string) (int64, error) {
	// Use npm api for download counts
	url := fmt.Sprintf("https://api.npmjs.org/downloads/point/last-week/%s", url.QueryEscape(packageName))

	resp, err := m.client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, nil
	}

	var dlResp NPMDownloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&dlResp); err != nil {
		return 0, err
	}

	return int64(dlResp.Downloads), nil
}

// GetPackageInfo retrieves detailed information about a package
func (m *NPMManager) GetPackageInfo(name string) (*PackageDetail, error) {
	// Build package info URL
	pkgURL := fmt.Sprintf("%s/%s", npmRegistryURL, url.PathEscape(name))

	resp, err := m.client.Get(pkgURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get npm package info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("package not found: %s", name)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("npm request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var npmPkg NPMPackageDetail
	if err := json.NewDecoder(resp.Body).Decode(&npmPkg); err != nil {
		return nil, fmt.Errorf("failed to decode npm package info: %w", err)
	}

	// Get latest version
	latestVersion := npmPkg.DistTags["latest"]

	// Get version list
	versions := make([]string, 0, len(npmPkg.Versions))
	for v := range npmPkg.Versions {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})

	// Get latest version details
	var deps, devDeps map[string]string
	if latestDetail, ok := npmPkg.Versions[latestVersion]; ok {
		deps = latestDetail.Dependencies
		devDeps = latestDetail.DevDependencies
	}

	// Convert maintainers
	maintainers := make([]Maintainer, len(npmPkg.Maintainers))
	for i, m := range npmPkg.Maintainers {
		maintainers[i] = Maintainer{
			Name:  m.Username,
			Email: m.Email,
		}
	}

	// Get license string
	license := ""
	switch l := npmPkg.License.(type) {
	case string:
		license = l
	case map[string]interface{}:
		if t, ok := l["type"].(string); ok {
			license = t
		}
	}

	// Get repository URL
	repoURL := npmPkg.Repository.URL
	if strings.HasPrefix(repoURL, "git+") {
		repoURL = strings.TrimPrefix(repoURL, "git+")
	}

	// Get download count
	downloads, _ := m.getDownloadCount(name)

	detail := &PackageDetail{
		Package: Package{
			Name:        npmPkg.Name,
			Version:     latestVersion,
			Description: npmPkg.Description,
			Downloads:   downloads,
			Homepage:    npmPkg.Homepage,
			Repository:  repoURL,
			License:     license,
			Author:      npmPkg.Author.Name,
			Keywords:    npmPkg.Keywords,
			PackageType: PackageTypeNPM,
		},
		Versions:        versions,
		LatestVersion:   latestVersion,
		Dependencies:    deps,
		DevDependencies: devDeps,
		Readme:          npmPkg.Readme,
		Maintainers:     maintainers,
		BugTracker:      npmPkg.Bugs.URL,
	}

	return detail, nil
}

// Install installs an NPM package to a project
func (m *NPMManager) Install(projectID uint, packageName, version string, isDev bool) error {
	// Get or create package.json
	file, err := GetDependencyFile(m.DB, projectID, PackageTypeNPM)
	if err != nil {
		return fmt.Errorf("failed to get dependency file: %w", err)
	}

	if file == nil {
		file, err = CreateDependencyFile(m.DB, projectID, PackageTypeNPM)
		if err != nil {
			return fmt.Errorf("failed to create package.json: %w", err)
		}
	}

	// Parse existing package.json
	pkg, err := ParsePackageJSON(file.Content)
	if err != nil {
		return fmt.Errorf("failed to parse package.json: %w", err)
	}

	// If no version specified, get the latest
	if version == "" || version == "latest" {
		info, err := m.GetPackageInfo(packageName)
		if err != nil {
			return fmt.Errorf("failed to get package info: %w", err)
		}
		version = "^" + info.LatestVersion
	} else if !strings.HasPrefix(version, "^") && !strings.HasPrefix(version, "~") &&
		!strings.HasPrefix(version, ">=") && !strings.HasPrefix(version, "<=") {
		version = "^" + version
	}

	// Add to appropriate dependency list
	if isDev {
		// Remove from regular dependencies if present
		delete(pkg.Dependencies, packageName)
		pkg.DevDependencies[packageName] = version
	} else {
		// Remove from dev dependencies if present
		delete(pkg.DevDependencies, packageName)
		pkg.Dependencies[packageName] = version
	}

	// Serialize and update file
	content, err := SerializePackageJSON(pkg)
	if err != nil {
		return fmt.Errorf("failed to serialize package.json: %w", err)
	}

	file.Content = content
	if err := m.DB.Save(file).Error; err != nil {
		return fmt.Errorf("failed to save package.json: %w", err)
	}

	return nil
}

// Uninstall removes an NPM package from a project
func (m *NPMManager) Uninstall(projectID uint, packageName string) error {
	// Get package.json
	file, err := GetDependencyFile(m.DB, projectID, PackageTypeNPM)
	if err != nil {
		return fmt.Errorf("failed to get dependency file: %w", err)
	}

	if file == nil {
		return fmt.Errorf("package.json not found")
	}

	// Parse existing package.json
	pkg, err := ParsePackageJSON(file.Content)
	if err != nil {
		return fmt.Errorf("failed to parse package.json: %w", err)
	}

	// Remove from both dependency lists
	deleted := false
	if _, ok := pkg.Dependencies[packageName]; ok {
		delete(pkg.Dependencies, packageName)
		deleted = true
	}
	if _, ok := pkg.DevDependencies[packageName]; ok {
		delete(pkg.DevDependencies, packageName)
		deleted = true
	}

	if !deleted {
		return fmt.Errorf("package %s is not installed", packageName)
	}

	// Serialize and update file
	content, err := SerializePackageJSON(pkg)
	if err != nil {
		return fmt.Errorf("failed to serialize package.json: %w", err)
	}

	file.Content = content
	if err := m.DB.Save(file).Error; err != nil {
		return fmt.Errorf("failed to save package.json: %w", err)
	}

	return nil
}

// ListInstalled lists all installed NPM packages for a project
func (m *NPMManager) ListInstalled(projectID uint) ([]InstalledPackage, error) {
	// Get package.json
	file, err := GetDependencyFile(m.DB, projectID, PackageTypeNPM)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependency file: %w", err)
	}

	if file == nil {
		return []InstalledPackage{}, nil
	}

	// Parse existing package.json
	pkg, err := ParsePackageJSON(file.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse package.json: %w", err)
	}

	packages := make([]InstalledPackage, 0, len(pkg.Dependencies)+len(pkg.DevDependencies))

	// Process dependencies
	for name, version := range pkg.Dependencies {
		installed := InstalledPackage{
			Name:        name,
			Version:     cleanVersion(version),
			IsDev:       false,
			PackageType: PackageTypeNPM,
		}

		// Check for updates (optional, can be slow)
		if info, err := m.GetPackageInfo(name); err == nil {
			installed.LatestVersion = info.LatestVersion
			installed.UpdateAvailable = compareVersions(info.LatestVersion, cleanVersion(version)) > 0
		}

		packages = append(packages, installed)
	}

	// Process dev dependencies
	for name, version := range pkg.DevDependencies {
		installed := InstalledPackage{
			Name:        name,
			Version:     cleanVersion(version),
			IsDev:       true,
			PackageType: PackageTypeNPM,
		}

		// Check for updates (optional, can be slow)
		if info, err := m.GetPackageInfo(name); err == nil {
			installed.LatestVersion = info.LatestVersion
			installed.UpdateAvailable = compareVersions(info.LatestVersion, cleanVersion(version)) > 0
		}

		packages = append(packages, installed)
	}

	return packages, nil
}

// UpdateDependencyFile updates all packages to their latest versions
func (m *NPMManager) UpdateDependencyFile(projectID uint) error {
	// Get package.json
	file, err := GetDependencyFile(m.DB, projectID, PackageTypeNPM)
	if err != nil {
		return fmt.Errorf("failed to get dependency file: %w", err)
	}

	if file == nil {
		return fmt.Errorf("package.json not found")
	}

	// Parse existing package.json
	pkg, err := ParsePackageJSON(file.Content)
	if err != nil {
		return fmt.Errorf("failed to parse package.json: %w", err)
	}

	// Update dependencies
	for name := range pkg.Dependencies {
		if info, err := m.GetPackageInfo(name); err == nil {
			pkg.Dependencies[name] = "^" + info.LatestVersion
		}
	}

	// Update dev dependencies
	for name := range pkg.DevDependencies {
		if info, err := m.GetPackageInfo(name); err == nil {
			pkg.DevDependencies[name] = "^" + info.LatestVersion
		}
	}

	// Serialize and update file
	content, err := SerializePackageJSON(pkg)
	if err != nil {
		return fmt.Errorf("failed to serialize package.json: %w", err)
	}

	file.Content = content
	if err := m.DB.Save(file).Error; err != nil {
		return fmt.Errorf("failed to save package.json: %w", err)
	}

	return nil
}

// Helper functions

// cleanVersion removes version prefixes like ^, ~, >=, etc.
func cleanVersion(version string) string {
	version = strings.TrimPrefix(version, "^")
	version = strings.TrimPrefix(version, "~")
	version = strings.TrimPrefix(version, ">=")
	version = strings.TrimPrefix(version, "<=")
	version = strings.TrimPrefix(version, ">")
	version = strings.TrimPrefix(version, "<")
	version = strings.TrimPrefix(version, "=")
	return strings.TrimSpace(version)
}

// compareVersions compares two semver versions
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	v1 = cleanVersion(v1)
	v2 = cleanVersion(v2)

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &n2)
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	return 0
}
