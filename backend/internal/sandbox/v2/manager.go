package v2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// IsolationMode controls the runtime isolation backend.
type IsolationMode string

const (
	IsolationDocker      IsolationMode = "docker"
	IsolationGVisor      IsolationMode = "gvisor"
	IsolationFirecracker IsolationMode = "firecracker"
)

// ResourceQuota defines execution limits for a language/runtime.
type ResourceQuota struct {
	MemoryBytes    int64
	CPUCores       float64
	PidsLimit      int64
	Timeout        time.Duration
	MaxOutputBytes int64
}

// CacheMountSpec describes a package-cache mount and env var wiring.
type CacheMountSpec struct {
	Name          string
	ContainerPath string
	Env           map[string]string
}

// LanguageTemplate defines runtime defaults for a supported language.
type LanguageTemplate struct {
	Language        string
	FileName        string
	Image           string
	WorkDir         string
	CommandTemplate []string
	Env             map[string]string
	CacheMounts     []CacheMountSpec
}

// ManagerConfig configures Sandbox v2.
type ManagerConfig struct {
	DockerHost            string
	DefaultIsolation      IsolationMode
	GVisorRuntime         string
	FirecrackerProxyCmd   string
	WorkspaceRoot         string
	PackageCacheRoot      string
	NetworkEnabled        bool
	ReadOnlyRootFS        bool
	NoNewPrivileges       bool
	PullImages            bool
	DefaultQuota          ResourceQuota
	LanguageQuotas        map[string]ResourceQuota
	Templates             map[string]LanguageTemplate
	DefaultTmpfsSize      string
	DefaultSharedMemSize  int64
	AllowedDockerRuntimes []string
	EnablePackageCache    bool
}

// Manager stores templates, quotas, and runtime configuration for sandbox v2.
type Manager struct {
	cfg       ManagerConfig
	templates map[string]LanguageTemplate
	mu        sync.RWMutex
}

// DefaultConfig returns a production-biased Sandbox v2 configuration.
func DefaultConfig() ManagerConfig {
	workspaceRoot := os.Getenv("SANDBOX_V2_WORKSPACE_ROOT")
	if workspaceRoot == "" {
		workspaceRoot = filepath.Join(os.TempDir(), "apex-sandbox-v2")
	}

	cacheRoot := os.Getenv("SANDBOX_V2_PACKAGE_CACHE_ROOT")
	if cacheRoot == "" {
		cacheRoot = filepath.Join(os.TempDir(), "apex-sandbox-v2-cache")
	}

	return ManagerConfig{
		DockerHost:           envOr("DOCKER_HOST", "unix:///var/run/docker.sock"),
		DefaultIsolation:     defaultIsolationFromEnv(),
		GVisorRuntime:        envOr("SANDBOX_V2_GVISOR_RUNTIME", "runsc"),
		FirecrackerProxyCmd:  os.Getenv("SANDBOX_V2_FIRECRACKER_PROXY_CMD"),
		WorkspaceRoot:        workspaceRoot,
		PackageCacheRoot:     cacheRoot,
		NetworkEnabled:       false,
		ReadOnlyRootFS:       true,
		NoNewPrivileges:      true,
		PullImages:           false,
		DefaultTmpfsSize:     "64m",
		DefaultSharedMemSize: 64 * 1024 * 1024,
		AllowedDockerRuntimes: []string{
			"",
			"runc",
			"runsc",
		},
		EnablePackageCache: true,
		DefaultQuota: ResourceQuota{
			MemoryBytes:    512 * 1024 * 1024,
			CPUCores:       1.0,
			PidsLimit:      128,
			Timeout:        45 * time.Second,
			MaxOutputBytes: 1 << 20,
		},
		LanguageQuotas: defaultLanguageQuotas(),
		Templates:      DefaultLanguageTemplates(),
	}
}

// NewManager constructs a sandbox v2 manager.
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	config := DefaultConfig()
	if cfg != nil {
		config = mergeConfig(config, *cfg)
	}

	if config.WorkspaceRoot == "" {
		return nil, fmt.Errorf("workspace root is required")
	}
	if err := os.MkdirAll(config.WorkspaceRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace root: %w", err)
	}
	if config.EnablePackageCache {
		if err := os.MkdirAll(config.PackageCacheRoot, 0o755); err != nil {
			return nil, fmt.Errorf("create package cache root: %w", err)
		}
	}

	m := &Manager{
		cfg:       config,
		templates: make(map[string]LanguageTemplate, len(config.Templates)),
	}

	for k, v := range config.Templates {
		m.templates[normalizeLanguage(k)] = normalizeTemplate(v)
	}

	return m, nil
}

// Config returns a copy of the manager config.
func (m *Manager) Config() ManagerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// GetTemplate returns a language template.
func (m *Manager) GetTemplate(language string) (LanguageTemplate, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.templates[normalizeLanguage(language)]
	return t, ok
}

// RegisterTemplate adds or replaces a language template.
func (m *Manager) RegisterTemplate(template LanguageTemplate) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.templates[normalizeLanguage(template.Language)] = normalizeTemplate(template)
}

// EffectiveQuota resolves a language-specific quota override.
func (m *Manager) EffectiveQuota(language string) ResourceQuota {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quota := m.cfg.DefaultQuota
	if q, ok := m.cfg.LanguageQuotas[normalizeLanguage(language)]; ok {
		if q.MemoryBytes > 0 {
			quota.MemoryBytes = q.MemoryBytes
		}
		if q.CPUCores > 0 {
			quota.CPUCores = q.CPUCores
		}
		if q.PidsLimit > 0 {
			quota.PidsLimit = q.PidsLimit
		}
		if q.Timeout > 0 {
			quota.Timeout = q.Timeout
		}
		if q.MaxOutputBytes > 0 {
			quota.MaxOutputBytes = q.MaxOutputBytes
		}
	}
	return quota
}

// WorkspaceRootForProject returns a project-scoped workspace root path.
func (m *Manager) WorkspaceRootForProject(projectID string) string {
	projectID = sanitizeID(projectID)
	if projectID == "" {
		projectID = "anonymous"
	}
	return filepath.Join(m.cfg.WorkspaceRoot, projectID)
}

// PackageCachePath returns a host cache directory for a cache mount key.
func (m *Manager) PackageCachePath(projectID, cacheName string) (string, error) {
	if !m.cfg.EnablePackageCache {
		return "", nil
	}
	projectID = sanitizeID(projectID)
	if projectID == "" {
		projectID = "shared"
	}
	cacheName = sanitizeID(cacheName)
	if cacheName == "" {
		return "", fmt.Errorf("invalid cache name")
	}
	p := filepath.Join(m.cfg.PackageCacheRoot, projectID, cacheName)
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	return p, nil
}

func defaultIsolationFromEnv() IsolationMode {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SANDBOX_V2_ISOLATION"))) {
	case string(IsolationGVisor):
		return IsolationGVisor
	case string(IsolationFirecracker):
		return IsolationFirecracker
	default:
		return IsolationDocker
	}
}

func defaultLanguageQuotas() map[string]ResourceQuota {
	return map[string]ResourceQuota{
		"python": {
			MemoryBytes:    256 * 1024 * 1024,
			CPUCores:       0.5,
			PidsLimit:      64,
			Timeout:        30 * time.Second,
			MaxOutputBytes: 1 << 20,
		},
		"javascript": {
			MemoryBytes:    256 * 1024 * 1024,
			CPUCores:       0.75,
			PidsLimit:      96,
			Timeout:        30 * time.Second,
			MaxOutputBytes: 1 << 20,
		},
		"typescript": {
			MemoryBytes:    512 * 1024 * 1024,
			CPUCores:       1.0,
			PidsLimit:      128,
			Timeout:        45 * time.Second,
			MaxOutputBytes: 1 << 20,
		},
		"go": {
			MemoryBytes:    768 * 1024 * 1024,
			CPUCores:       1.5,
			PidsLimit:      192,
			Timeout:        60 * time.Second,
			MaxOutputBytes: 1 << 20,
		},
		"rust": {
			MemoryBytes:    1024 * 1024 * 1024,
			CPUCores:       2.0,
			PidsLimit:      256,
			Timeout:        90 * time.Second,
			MaxOutputBytes: 1 << 20,
		},
		"java": {
			MemoryBytes:    1024 * 1024 * 1024,
			CPUCores:       1.5,
			PidsLimit:      256,
			Timeout:        90 * time.Second,
			MaxOutputBytes: 1 << 20,
		},
		"c": {
			MemoryBytes:    384 * 1024 * 1024,
			CPUCores:       1.0,
			PidsLimit:      128,
			Timeout:        45 * time.Second,
			MaxOutputBytes: 1 << 20,
		},
		"cpp": {
			MemoryBytes:    512 * 1024 * 1024,
			CPUCores:       1.25,
			PidsLimit:      160,
			Timeout:        60 * time.Second,
			MaxOutputBytes: 1 << 20,
		},
	}
}

// DefaultLanguageTemplates defines sandbox-v2 multi-language execution templates.
func DefaultLanguageTemplates() map[string]LanguageTemplate {
	return map[string]LanguageTemplate{
		"python": {
			Language: "python",
			FileName: "main.py",
			Image:    "python:3.12-slim-bookworm",
			WorkDir:  "/workspace",
			CommandTemplate: []string{
				"python3", "-u", "{{file}}",
			},
			Env: map[string]string{
				"PYTHONDONTWRITEBYTECODE":       "1",
				"PYTHONUNBUFFERED":              "1",
				"PIP_DISABLE_PIP_VERSION_CHECK": "1",
			},
			CacheMounts: []CacheMountSpec{
				{
					Name:          "pip",
					ContainerPath: "/cache/pip",
					Env:           map[string]string{"PIP_CACHE_DIR": "/cache/pip"},
				},
			},
		},
		"javascript": {
			Language: "javascript",
			FileName: "main.js",
			Image:    "node:20-slim",
			WorkDir:  "/workspace",
			CommandTemplate: []string{
				"node", "{{file}}",
			},
			Env: map[string]string{
				"NODE_ENV": "production",
			},
			CacheMounts: []CacheMountSpec{
				{
					Name:          "npm",
					ContainerPath: "/cache/npm",
					Env:           map[string]string{"NPM_CONFIG_CACHE": "/cache/npm"},
				},
			},
		},
		"typescript": {
			Language: "typescript",
			FileName: "main.ts",
			Image:    "node:20-slim",
			WorkDir:  "/workspace",
			CommandTemplate: []string{
				"sh", "-lc", "npm --yes --cache /cache/npm exec tsx {{file}}",
			},
			Env: map[string]string{
				"NODE_ENV": "production",
			},
			CacheMounts: []CacheMountSpec{
				{
					Name:          "npm",
					ContainerPath: "/cache/npm",
					Env:           map[string]string{"NPM_CONFIG_CACHE": "/cache/npm"},
				},
			},
		},
		"go": {
			Language: "go",
			FileName: "main.go",
			Image:    "golang:1.22-bookworm",
			WorkDir:  "/workspace",
			CommandTemplate: []string{
				"sh", "-lc", "go run {{file}}",
			},
			Env: map[string]string{
				"CGO_ENABLED": "0",
			},
			CacheMounts: []CacheMountSpec{
				{
					Name:          "go-build",
					ContainerPath: "/cache/go-build",
					Env:           map[string]string{"GOCACHE": "/cache/go-build"},
				},
				{
					Name:          "go-mod",
					ContainerPath: "/cache/go-mod",
					Env:           map[string]string{"GOMODCACHE": "/cache/go-mod"},
				},
			},
		},
		"rust": {
			Language: "rust",
			FileName: "main.rs",
			Image:    "rust:1.75-slim-bookworm",
			WorkDir:  "/workspace",
			CommandTemplate: []string{
				"sh", "-lc", "rustc {{file}} -O -o /tmp/main && /tmp/main",
			},
			CacheMounts: []CacheMountSpec{
				{
					Name:          "cargo-home",
					ContainerPath: "/cache/cargo-home",
					Env:           map[string]string{"CARGO_HOME": "/cache/cargo-home"},
				},
				{
					Name:          "cargo-target",
					ContainerPath: "/cache/cargo-target",
					Env:           map[string]string{"CARGO_TARGET_DIR": "/cache/cargo-target"},
				},
			},
		},
		"java": {
			Language: "java",
			FileName: "Main.java",
			Image:    "eclipse-temurin:21-jdk-jammy",
			WorkDir:  "/workspace",
			CommandTemplate: []string{
				"sh", "-lc", "javac {{file}} && java ${APEX_JAVA_CLASS:-Main}",
			},
			CacheMounts: []CacheMountSpec{
				{
					Name:          "m2",
					ContainerPath: "/cache/m2",
					Env:           map[string]string{"MAVEN_CONFIG": "/cache/m2"},
				},
			},
		},
		"c": {
			Language: "c",
			FileName: "main.c",
			Image:    "gcc:13-bookworm",
			WorkDir:  "/workspace",
			CommandTemplate: []string{
				"sh", "-lc", "gcc -O2 {{file}} -o /tmp/main -lm && /tmp/main",
			},
		},
		"cpp": {
			Language: "cpp",
			FileName: "main.cpp",
			Image:    "gcc:13-bookworm",
			WorkDir:  "/workspace",
			CommandTemplate: []string{
				"sh", "-lc", "g++ -O2 -std=c++17 {{file}} -o /tmp/main && /tmp/main",
			},
		},
	}
}

func mergeConfig(base, override ManagerConfig) ManagerConfig {
	if override.DockerHost != "" {
		base.DockerHost = override.DockerHost
	}
	if override.DefaultIsolation != "" {
		base.DefaultIsolation = override.DefaultIsolation
	}
	if override.GVisorRuntime != "" {
		base.GVisorRuntime = override.GVisorRuntime
	}
	if override.FirecrackerProxyCmd != "" {
		base.FirecrackerProxyCmd = override.FirecrackerProxyCmd
	}
	if override.WorkspaceRoot != "" {
		base.WorkspaceRoot = override.WorkspaceRoot
	}
	if override.PackageCacheRoot != "" {
		base.PackageCacheRoot = override.PackageCacheRoot
	}
	if override.NetworkEnabled {
		base.NetworkEnabled = true
	}
	if override.ReadOnlyRootFS {
		base.ReadOnlyRootFS = true
	}
	if override.NoNewPrivileges {
		base.NoNewPrivileges = true
	}
	if override.PullImages {
		base.PullImages = true
	}
	if override.DefaultTmpfsSize != "" {
		base.DefaultTmpfsSize = override.DefaultTmpfsSize
	}
	if override.DefaultSharedMemSize > 0 {
		base.DefaultSharedMemSize = override.DefaultSharedMemSize
	}
	if override.AllowedDockerRuntimes != nil {
		base.AllowedDockerRuntimes = append([]string(nil), override.AllowedDockerRuntimes...)
	}
	if override.DefaultQuota.MemoryBytes > 0 {
		base.DefaultQuota = override.DefaultQuota
	}
	if override.LanguageQuotas != nil {
		base.LanguageQuotas = override.LanguageQuotas
	}
	if override.Templates != nil {
		base.Templates = override.Templates
	}
	base.EnablePackageCache = override.EnablePackageCache || base.EnablePackageCache
	return base
}

func normalizeTemplate(t LanguageTemplate) LanguageTemplate {
	t.Language = normalizeLanguage(t.Language)
	if t.WorkDir == "" {
		t.WorkDir = "/workspace"
	}
	if t.Env == nil {
		t.Env = map[string]string{}
	}
	if t.CacheMounts == nil {
		t.CacheMounts = []CacheMountSpec{}
	}
	return t
}

func normalizeLanguage(language string) string {
	lang := strings.ToLower(strings.TrimSpace(language))
	switch lang {
	case "js", "node", "nodejs":
		return "javascript"
	case "ts":
		return "typescript"
	case "py", "python3":
		return "python"
	case "golang":
		return "go"
	case "c++":
		return "cpp"
	default:
		return lang
	}
}

func sanitizeID(in string) string {
	in = strings.ToLower(strings.TrimSpace(in))
	if in == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(in))
	for _, r := range in {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
