package agents

import "testing"

func TestPathGuard_DirectMatch(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		patterns []string
		wantErr  bool
		wantPat  string
	}{
		{
			name:     "*.env matches .env",
			path:     ".env",
			patterns: []string{"*.env"},
			wantErr:  true,
			wantPat:  "*.env",
		},
		{
			name:     "*.env matches production.env",
			path:     "production.env",
			patterns: []string{"*.env"},
			wantErr:  true,
			wantPat:  "*.env",
		},
		{
			name:     "exact filename match",
			path:     "Dockerfile",
			patterns: []string{"Dockerfile"},
			wantErr:  true,
			wantPat:  "Dockerfile",
		},
		{
			name:     "no match returns nil",
			path:     "main.go",
			patterns: []string{"*.env", "*.secret"},
			wantErr:  false,
		},
	}

	pg := NewPathGuard()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pg.CheckPath(tt.path, tt.patterns)
			if tt.wantErr && err == nil {
				t.Fatalf("expected ErrProtectedPath for path=%q patterns=%v", tt.path, tt.patterns)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
			if tt.wantErr && err.Pattern != tt.wantPat {
				t.Fatalf("expected pattern %q, got %q", tt.wantPat, err.Pattern)
			}
		})
	}
}

func TestPathGuard_RecursiveGlob(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		patterns []string
		wantErr  bool
	}{
		{
			name:     "src/** matches src/foo/bar.ts",
			path:     "src/foo/bar.ts",
			patterns: []string{"src/**"},
			wantErr:  true,
		},
		{
			name:     "src/** matches src/index.ts",
			path:     "src/index.ts",
			patterns: []string{"src/**"},
			wantErr:  true,
		},
		{
			name:     "src/** does not match lib/foo.ts",
			path:     "lib/foo.ts",
			patterns: []string{"src/**"},
			wantErr:  false,
		},
		{
			name:     "** alone matches anything",
			path:     "deeply/nested/file.txt",
			patterns: []string{"**"},
			wantErr:  true,
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

func TestPathGuard_PrefixWithSuffix(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		patterns []string
		wantErr  bool
	}{
		{
			name:     "config/**/*.json matches config/prod/db.json",
			path:     "config/prod/db.json",
			patterns: []string{"config/**/*.json"},
			wantErr:  true,
		},
		{
			name:     "config/**/*.json matches config/db.json",
			path:     "config/db.json",
			patterns: []string{"config/**/*.json"},
			wantErr:  true,
		},
		{
			name:     "config/**/*.json does not match config/db.yaml",
			path:     "config/db.yaml",
			patterns: []string{"config/**/*.json"},
			wantErr:  false,
		},
		{
			name:     "config/**/*.json does not match other/db.json",
			path:     "other/db.json",
			patterns: []string{"config/**/*.json"},
			wantErr:  false,
		},
		{
			name:     "deploy/**/*.yml matches deploy/k8s/service.yml",
			path:     "deploy/k8s/service.yml",
			patterns: []string{"deploy/**/*.yml"},
			wantErr:  true,
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

func TestPathGuard_FilenameOnlyMatch(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		patterns []string
		wantErr  bool
	}{
		{
			name:     "*.env matches nested path by basename",
			path:     "config/.env",
			patterns: []string{"*.env"},
			wantErr:  true,
		},
		{
			name:     "Makefile matches nested Makefile",
			path:     "services/api/Makefile",
			patterns: []string{"Makefile"},
			wantErr:  true,
		},
		{
			name:     "*.lock matches yarn.lock in subdirectory",
			path:     "frontend/yarn.lock",
			patterns: []string{"*.lock"},
			wantErr:  true,
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

func TestPathGuard_EmptyPatterns(t *testing.T) {
	pg := NewPathGuard()

	t.Run("nil patterns", func(t *testing.T) {
		err := pg.CheckPath("anything.go", nil)
		if err != nil {
			t.Fatalf("expected nil with nil patterns, got %v", err)
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		err := pg.CheckPath("anything.go", []string{})
		if err != nil {
			t.Fatalf("expected nil with empty patterns, got %v", err)
		}
	})

	t.Run("whitespace-only patterns are skipped", func(t *testing.T) {
		err := pg.CheckPath("anything.go", []string{"", "  ", "\t"})
		if err != nil {
			t.Fatalf("expected nil with whitespace-only patterns, got %v", err)
		}
	})
}

func TestPathGuard_CheckPaths(t *testing.T) {
	pg := NewPathGuard()

	t.Run("returns first protected path", func(t *testing.T) {
		paths := []string{"main.go", ".env", "secret.key"}
		patterns := []string{"*.env", "*.key"}

		err := pg.CheckPaths(paths, patterns)
		if err == nil {
			t.Fatalf("expected ErrProtectedPath, got nil")
		}
		if err.Path != ".env" {
			t.Fatalf("expected first match to be .env, got %s", err.Path)
		}
	})

	t.Run("returns nil when no path matches", func(t *testing.T) {
		paths := []string{"main.go", "handler.go", "types.go"}
		patterns := []string{"*.env", "*.secret"}

		err := pg.CheckPaths(paths, patterns)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("empty paths returns nil", func(t *testing.T) {
		err := pg.CheckPaths([]string{}, []string{"*.env"})
		if err != nil {
			t.Fatalf("expected nil for empty paths, got %v", err)
		}
	})

	t.Run("single path multiple patterns", func(t *testing.T) {
		err := pg.CheckPaths([]string{"deploy/k8s/secret.yaml"}, []string{"deploy/**/*.yaml"})
		if err == nil {
			t.Fatalf("expected ErrProtectedPath for deploy path")
		}
	})
}

func TestPathGuard_ErrorMessage(t *testing.T) {
	err := &ErrProtectedPath{Path: ".env", Pattern: "*.env"}
	msg := err.Error()
	if msg != `path ".env" is protected by pattern "*.env"` {
		t.Fatalf("unexpected error message: %s", msg)
	}
}

func TestPathGuard_PathNormalization(t *testing.T) {
	pg := NewPathGuard()

	t.Run("cleans trailing slash", func(t *testing.T) {
		// filepath.Clean("src/./foo.go") -> "src/foo.go"
		err := pg.CheckPath("src/./foo.go", []string{"src/**"})
		if err == nil {
			t.Fatalf("expected match after path cleaning")
		}
	})

	t.Run("cleans double slash", func(t *testing.T) {
		err := pg.CheckPath("src//foo.go", []string{"src/**"})
		if err == nil {
			t.Fatalf("expected match after path cleaning")
		}
	})
}

func TestPathGuard_FirstPatternWins(t *testing.T) {
	pg := NewPathGuard()
	err := pg.CheckPath(".env", []string{"*.env", ".*"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Pattern != "*.env" {
		t.Fatalf("expected first matching pattern *.env, got %s", err.Pattern)
	}
}
