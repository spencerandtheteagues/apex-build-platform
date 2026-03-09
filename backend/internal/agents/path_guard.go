package agents

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ErrProtectedPath is returned when an agent tries to modify a protected file
type ErrProtectedPath struct {
	Path    string
	Pattern string
}

func (e *ErrProtectedPath) Error() string {
	return fmt.Sprintf("path %q is protected by pattern %q", e.Path, e.Pattern)
}

// PathGuard checks file paths against protected patterns
type PathGuard struct{}

// NewPathGuard creates a new PathGuard
func NewPathGuard() *PathGuard {
	return &PathGuard{}
}

// CheckPath checks if a path matches any protected pattern.
// Returns *ErrProtectedPath if the path is protected, nil otherwise.
// Patterns use filepath.Match syntax (*, ?, [...]) plus ** for recursive matching.
func (pg *PathGuard) CheckPath(path string, patterns []string) *ErrProtectedPath {
	normalized := filepath.Clean(path)
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if pg.matches(normalized, pattern) {
			return &ErrProtectedPath{Path: path, Pattern: pattern}
		}
	}
	return nil
}

// CheckPaths checks multiple paths against the protected patterns.
// Returns the first ErrProtectedPath encountered, or nil if all paths are allowed.
func (pg *PathGuard) CheckPaths(paths []string, patterns []string) *ErrProtectedPath {
	for _, p := range paths {
		if err := pg.CheckPath(p, patterns); err != nil {
			return err
		}
	}
	return nil
}

func (pg *PathGuard) matches(path, pattern string) bool {
	// Handle ** recursive glob
	if strings.Contains(pattern, "**") {
		// Split pattern on ** (only first occurrence)
		parts := strings.SplitN(pattern, "**", 2)
		prefix := strings.TrimRight(parts[0], "/")
		suffix := strings.TrimLeft(parts[1], "/")

		// ** alone (no prefix, no suffix) matches everything
		if prefix == "" && suffix == "" {
			return true
		}

		// Determine the remaining path after stripping the prefix
		remaining := path
		if prefix != "" {
			// Path must start with "prefix/" or equal prefix exactly
			if !strings.HasPrefix(path, prefix+"/") && path != prefix {
				return false
			}
			if path == prefix {
				// Path is exactly the prefix directory; only matches if suffix is also empty
				return suffix == ""
			}
			// Strip "prefix/" from the front
			remaining = path[len(prefix)+1:]
		}

		// No suffix: ** matches everything under the prefix dir
		if suffix == "" {
			return true
		}

		// Try matching suffix against the full remaining path and against just its basename.
		// This makes src/**/*.js match both src/Button.js and src/components/ui/Button.js.
		if matched, _ := filepath.Match(suffix, remaining); matched {
			return true
		}
		matched, _ := filepath.Match(suffix, filepath.Base(remaining))
		return matched
	}

	// Try direct match
	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Try matching just the filename
	matched, _ = filepath.Match(pattern, filepath.Base(path))
	return matched
}
