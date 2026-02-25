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
		// Split pattern on **
		parts := strings.SplitN(pattern, "**", 2)
		prefix := strings.TrimRight(parts[0], "/")
		suffix := strings.TrimLeft(parts[1], "/")

		// Check if path starts with prefix
		if prefix != "" && !strings.HasPrefix(path, prefix) {
			return false
		}

		// Check if path ends with suffix (if any)
		if suffix != "" {
			matched, _ := filepath.Match(suffix, filepath.Base(path))
			return matched
		}

		// ** with just prefix means everything under that dir
		return true
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
