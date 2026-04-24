package agents

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ---------------------------------------------------------------------------
// Allowed path patterns
// ---------------------------------------------------------------------------

func TestPathGuard_AllowedPatterns(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		patterns []string
		wantErr  bool
	}{
		{
			name:     "simple file allowed",
			path:     "main.go",
			patterns: []string{"*.ts", "*.js"},
			wantErr:  false,
		},
		{
			name:     "directory prefix allowed when no match",
			path:     "src/components/Button.tsx",
			patterns: []string{"*.env", "*.secret"},
			wantErr:  false,
		},
		{
			name:     "nested path with multiple patterns none match",
			path:     "packages/core/utils.ts",
			patterns: []string{"*.lock", "Dockerfile"},
			wantErr:  false,
		},
		{
			name:     "path with dots in directory name",
			path:     "some.dir/file.go",
			patterns: []string{"*.env"},
			wantErr:  false,
		},
	}

	pg := NewPathGuard()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pg.CheckPath(tt.path, tt.patterns)
			if tt.wantErr && err == nil {
				t.Fatalf("expected ErrProtectedPath for path=%q", tt.path)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Blocked path patterns
// ---------------------------------------------------------------------------

func TestPathGuard_BlockedPatterns(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		patterns []string
		wantPat  string
	}{
		{
			name:     "exact block",
			path:     "package-lock.json",
			patterns: []string{"package-lock.json"},
			wantPat:  "package-lock.json",
		},
		{
			name:     "wildcard block in any directory",
			path:     "node_modules/lodash/index.js",
			patterns: []string{"node_modules/**"},
			wantPat:  "node_modules/**",
		},
		{
			name:     "suffix block matches nested",
			path:     "config/secrets.env",
			patterns: []string{"*.env"},
			wantPat:  "*.env",
		},
		{
			name:     "multiple patterns second match",
			path:     "deploy.yaml",
			patterns: []string{"*.env", "*.yaml"},
			wantPat:  "*.yaml",
		},
		{
			name:     "question mark wildcard",
			path:     "data.1",
			patterns: []string{"data.?"},
			wantPat:  "data.?",
		},
		{
			name:     "character class",
			path:     "backup.tar.gz",
			patterns: []string{"*.[tg]z"},
			wantPat:  "*.[tg]z",
		},
	}

	pg := NewPathGuard()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pg.CheckPath(tt.path, tt.patterns)
			if err == nil {
				t.Fatalf("expected ErrProtectedPath for path=%q", tt.path)
			}
			if err.Pattern != tt.wantPat {
				t.Fatalf("expected pattern %q, got %q", tt.wantPat, err.Pattern)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Path traversal prevention
// ---------------------------------------------------------------------------

func TestPathGuard_PathTraversalPrevention(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		patterns []string
		wantErr  bool
	}{
		{
			name:     "traversal up then into protected dir",
			path:     "src/../../etc/passwd",
			patterns: []string{"etc/**"},
			// filepath.Clean resolves to "../etc/passwd" which does NOT start with "etc/"
			wantErr:  false,
		},
		{
			name:     "current dir prefix cleaned away",
			path:     "./src/main.go",
			patterns: []string{"src/**"},
			wantErr:  true, // cleaned to "src/main.go"
		},
		{
			name:     "double dots in middle",
			path:     "src/../.env",
			patterns: []string{"*.env"},
			wantErr:  true, // cleaned to ".env"
		},
		{
			name:     "traversal that escapes root",
			path:     "../../../secret.key",
			patterns: []string{"*.key"},
			wantErr:  true, // filepath.Match("*.key", "secret.key") matches by basename
		},
		{
			name:     "safe path with dots in name",
			path:     "src/components/ui.button.tsx",
			patterns: []string{"*.env"},
			wantErr:  false,
		},
		{
			name:     "traversal stays outside protected",
			path:     "src/../lib/helper.go",
			patterns: []string{"*.env"},
			wantErr:  false, // cleaned to "lib/helper.go"
		},
	}

	pg := NewPathGuard()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pg.CheckPath(tt.path, tt.patterns)
			if tt.wantErr && err == nil {
				t.Fatalf("expected ErrProtectedPath for path=%q", tt.path)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil for path=%q, got %v", tt.path, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestPathGuard_EmptyPath(t *testing.T) {
	pg := NewPathGuard()

	t.Run("empty path with no patterns", func(t *testing.T) {
		err := pg.CheckPath("", nil)
		if err != nil {
			t.Fatalf("expected nil for empty path, got %v", err)
		}
	})

	t.Run("empty path with patterns", func(t *testing.T) {
		// filepath.Clean("") returns "." and patterns like "*.env" won't match
		err := pg.CheckPath("", []string{"*.env"})
		if err != nil {
			t.Fatalf("expected nil for empty path, got %v", err)
		}
	})

	t.Run("dot path with wildcard", func(t *testing.T) {
		// filepath.Clean(".") returns "." and filepath.Match("*", ".") returns true
		// because * matches any sequence of non-separator characters.
		err := pg.CheckPath(".", []string{"*"})
		if err == nil {
			t.Fatalf("expected ErrProtectedPath for '.', got nil")
		}
	})
}

func TestPathGuard_RelativePaths(t *testing.T) {
	pg := NewPathGuard()

	tests := []struct {
		name     string
		path     string
		patterns []string
		wantErr  bool
	}{
		{
			name:     "single dot prefix cleaned away",
			path:     "./README.md",
			patterns: []string{"README.md"},
			wantErr:  true,
		},
		{
			name:     "double dot in path",
			path:     "docs/../README.md",
			patterns: []string{"README.md"},
			wantErr:  true, // cleaned to "README.md"
		},
		{
			name:     "relative stays safe",
			path:     "./src/main.go",
			patterns: []string{"*.env"},
			wantErr:  false,
		},
		{
			name:     "multiple dots in filename",
			path:     "src/main.test.go",
			patterns: []string{"*.env"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pg.CheckPath(tt.path, tt.patterns)
			if tt.wantErr && err == nil {
				t.Fatalf("expected ErrProtectedPath for path=%q", tt.path)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil for path=%q, got %v", tt.path, err)
			}
		})
	}
}

func TestPathGuard_Symlinks(t *testing.T) {
	// Only run on Unix-like systems where symlinks are reliable in tests
	if runtime.GOOS == "windows" {
		t.Skip("skipping symlink tests on Windows")
	}

	// Create a temp directory with a symlink
	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "real.go")
	if err := os.WriteFile(realFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("create real file: %v", err)
	}

	linkFile := filepath.Join(tmpDir, "link.go")
	if err := os.Symlink(realFile, linkFile); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	pg := NewPathGuard()

	t.Run("symlink path checked as-is", func(t *testing.T) {
		// filepath.Clean does NOT resolve symlinks, so linkFile is checked literally.
		// The basename "link.go" matches "*.go".
		err := pg.CheckPath(linkFile, []string{"*.go"})
		if err == nil {
			t.Fatalf("expected symlink path %q to match *.go by basename", linkFile)
		}
	})

	t.Run("symlink does not resolve to real path", func(t *testing.T) {
		// filepath.Clean does NOT resolve symlinks, so the path stays as linkFile
		// and matches by basename.
		err := pg.CheckPath(linkFile, []string{"link.go"})
		if err == nil {
			t.Fatalf("expected link.go to match by basename")
		}
	})

	t.Run("symlink in protected directory", func(t *testing.T) {
		protectedDir := filepath.Join(tmpDir, "protected")
		if err := os.MkdirAll(protectedDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		linkInProtected := filepath.Join(protectedDir, "link.go")
		if err := os.Symlink(realFile, linkInProtected); err != nil {
			t.Fatalf("create symlink: %v", err)
		}

		// Use ** alone which matches everything regardless of absolute/relative path.
		err := pg.CheckPath(linkInProtected, []string{"**"})
		if err == nil {
			t.Fatalf("expected symlink in protected dir to match **")
		}
	})
}

func TestPathGuard_UnicodeAndSpecialCharacters(t *testing.T) {
	pg := NewPathGuard()

	tests := []struct {
		name     string
		path     string
		patterns []string
		wantErr  bool
	}{
		{
			name:     "unicode filename allowed",
			path:     "src/组件.tsx",
			patterns: []string{"*.env"},
			wantErr:  false,
		},
		{
			name:     "unicode filename blocked",
			path:     "配置.env",
			patterns: []string{"*.env"},
			wantErr:  true,
		},
		{
			name:     "space in filename allowed",
			path:     "my file.go",
			patterns: []string{"*.env"},
			wantErr:  false,
		},
		{
			name:     "space in filename blocked",
			path:     "my secret.env",
			patterns: []string{"*.env"},
			wantErr:  true,
		},
		{
			name:     "percent encoded not decoded",
			path:     "file%20name.go",
			patterns: []string{"*.env"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pg.CheckPath(tt.path, tt.patterns)
			if tt.wantErr && err == nil {
				t.Fatalf("expected ErrProtectedPath for path=%q", tt.path)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil for path=%q, got %v", tt.path, err)
			}
		})
	}
}

func TestPathGuard_VeryLongPath(t *testing.T) {
	pg := NewPathGuard()

	// Build a very deep path
	var path string
	for i := 0; i < 50; i++ {
		path = filepath.Join(path, "deep")
	}
	path = filepath.Join(path, "file.env")

	t.Run("deep path matches pattern", func(t *testing.T) {
		err := pg.CheckPath(path, []string{"*.env"})
		if err == nil {
			t.Fatalf("expected deep path to match *.env by basename")
		}
	})

	t.Run("deep path prefix match", func(t *testing.T) {
		err := pg.CheckPath(path, []string{"deep/**"})
		if err == nil {
			t.Fatalf("expected deep path to match deep/** prefix")
		}
	})
}

func TestPathGuard_CheckPaths_AllBlocked(t *testing.T) {
	pg := NewPathGuard()

	paths := []string{"main.go", "README.md", ".env", "config.yaml"}
	patterns := []string{"*.env", "*.secret"}

	err := pg.CheckPaths(paths, patterns)
	if err == nil {
		t.Fatal("expected error for .env in paths")
	}
	if err.Path != ".env" {
		t.Fatalf("expected first blocked path to be .env, got %s", err.Path)
	}
}

func TestPathGuard_CheckPaths_AllAllowed(t *testing.T) {
	pg := NewPathGuard()

	paths := []string{"main.go", "README.md", "config.yaml"}
	patterns := []string{"*.env", "*.secret"}

	err := pg.CheckPaths(paths, patterns)
	if err != nil {
		t.Fatalf("expected nil when no paths match, got %v", err)
	}
}

func TestPathGuard_DoubleStarEdgeCases(t *testing.T) {
	pg := NewPathGuard()

	tests := []struct {
		name     string
		path     string
		patterns []string
		wantErr  bool
	}{
		{
			name:     "**/prefix — NOT SUPPORTED by current impl (mid-string **)",
			path:     "a/b/c/src/main.ts",
			patterns: []string{"**/src/**"},
			wantErr:  false,
		},
		{
			name:     "**/suffix.ts matches any depth",
			path:     "very/deep/path/file.ts",
			patterns: []string{"**/*.ts"},
			wantErr:  true,
		},
		{
			name:     "prefix/**/suffix — NOT SUPPORTED by current impl",
			path:     "src/components/ui/Button.tsx",
			patterns: []string{"src/**/ui/*.tsx"},
			wantErr:  false,
		},
		{
			name:     "double star at root matches file directly",
			path:     "file.ts",
			patterns: []string{"**/*.ts"},
			wantErr:  true,
		},
		{
			name:     "double star alone matches everything including dot",
			path:     "",
			patterns: []string{"**"},
			wantErr:  true, // filepath.Clean("") = ".", ** alone matches everything per impl
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pg.CheckPath(tt.path, tt.patterns)
			if tt.wantErr && err == nil {
				t.Fatalf("expected ErrProtectedPath for path=%q", tt.path)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil for path=%q, got %v", tt.path, err)
			}
		})
	}
}
