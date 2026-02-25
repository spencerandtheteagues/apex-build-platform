package execution

import (
	"os"
	"path/filepath"
	"strings"
)

// PackageCacheMount describes a host<->container cache bind mount and env wiring.
type PackageCacheMount struct {
	HostPath       string
	ContainerPath  string
	ReadOnly       bool
	EnvironmentMap map[string]string
}

// PackageCacheManager manages shared Nix-style package cache directories for container executions.
// "Nix-style" here means deterministic, content-addressable cache roots mounted into sandboxes
// per toolchain/language, while keeping the execution workspace ephemeral.
type PackageCacheManager struct {
	enabled bool
	baseDir string
}

func NewPackageCacheManager(baseDir string, enabled bool) *PackageCacheManager {
	if baseDir == "" {
		baseDir = filepath.Join(os.TempDir(), "apex-sandbox-pkg-cache")
	}
	m := &PackageCacheManager{
		enabled: enabled,
		baseDir: baseDir,
	}
	if m.enabled {
		_ = os.MkdirAll(m.baseDir, 0o755)
	}
	return m
}

func (m *PackageCacheManager) Enabled() bool {
	return m != nil && m.enabled
}

// MountsForLanguage returns cache mounts for a language.
func (m *PackageCacheManager) MountsForLanguage(language string) []PackageCacheMount {
	if !m.Enabled() {
		return nil
	}

	lang := strings.ToLower(strings.TrimSpace(language))
	switch lang {
	case "javascript", "typescript", "js", "ts":
		return []PackageCacheMount{
			m.mount("npm", "/cache/npm", map[string]string{"NPM_CONFIG_CACHE": "/cache/npm"}),
		}
	case "python", "py":
		return []PackageCacheMount{
			m.mount("pip", "/cache/pip", map[string]string{"PIP_CACHE_DIR": "/cache/pip"}),
		}
	case "go", "golang":
		return []PackageCacheMount{
			m.mount("go-build", "/cache/go-build", map[string]string{"GOCACHE": "/cache/go-build"}),
			m.mount("go-mod", "/cache/go-mod", map[string]string{"GOMODCACHE": "/cache/go-mod"}),
		}
	case "rust", "rs":
		return []PackageCacheMount{
			m.mount("cargo-home", "/cache/cargo-home", map[string]string{"CARGO_HOME": "/cache/cargo-home"}),
			m.mount("cargo-target", "/cache/cargo-target", map[string]string{"CARGO_TARGET_DIR": "/cache/cargo-target"}),
		}
	case "java":
		return []PackageCacheMount{
			m.mount("m2", "/cache/m2", map[string]string{"MAVEN_CONFIG": "/cache/m2"}),
		}
	default:
		return nil
	}
}

func (m *PackageCacheManager) mount(name, containerPath string, env map[string]string) PackageCacheMount {
	hostPath := filepath.Join(m.baseDir, sanitizeCacheName(name))
	_ = os.MkdirAll(hostPath, 0o755)
	return PackageCacheMount{
		HostPath:       hostPath,
		ContainerPath:  containerPath,
		EnvironmentMap: env,
	}
}

func sanitizeCacheName(in string) string {
	in = strings.ToLower(strings.TrimSpace(in))
	if in == "" {
		return "default"
	}
	var b strings.Builder
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
