// Package agents — smart context selection for large projects.
//
// When an AI agent needs to edit or analyse code it typically has access to
// every file the executor wrote. For large projects this can be hundreds of
// files that far exceed the LLM's context window. ContextSelector trims the
// candidate set down to the most relevant files using:
//
//  1. Error-path heuristic: files mentioned in build errors are always included.
//  2. Recency heuristic: recently modified files are preferred over stale ones.
//  3. Dependency graph heuristic: files that import an errored file are included.
//  4. Token budget enforcement: selection stops once the budget is consumed.
//
// The result is a map[filePath]content suitable for passing directly to
// ErrorAnalyzer.Analyze or an LLM prompt builder.
//
// Design principles:
//   - Zero I/O: callers pass the file map; the selector is a pure transformer.
//   - Deterministic: given the same inputs and budget it always returns the
//     same selection (ties are broken by path for stability).
//   - Safe: if the project is small enough to fit in budget, all files are
//     returned unchanged.

package agents

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	// DefaultTokenBudget is the maximum estimated token count for the selected
	// context. 1 token ≈ 4 characters is a conservative heuristic.
	DefaultTokenBudget = 80_000

	// MaxFilesInContext is an upper bound on how many files can be included
	// regardless of the token budget, to avoid overwhelming the model.
	MaxFilesInContext = 12

	// charsPerToken is the character-to-token ratio used for budget estimation.
	charsPerToken = 4
)

// FileEntry holds a file's content and scoring metadata for context selection.
type FileEntry struct {
	Path        string
	Content     string
	Score       int  // higher = more relevant; selection is highest-score-first
	MentionedInError bool
}

// ContextSelector selects the most relevant subset of files to include in an
// LLM prompt, staying within a token budget.
type ContextSelector struct {
	tokenBudget int
	maxFiles    int
}

// NewContextSelector creates a selector with default limits.
func NewContextSelector() *ContextSelector {
	return &ContextSelector{
		tokenBudget: DefaultTokenBudget,
		maxFiles:    MaxFilesInContext,
	}
}

// NewContextSelectorWithLimits creates a selector with custom limits.
// tokenBudget ≤ 0 uses DefaultTokenBudget; maxFiles ≤ 0 uses MaxFilesInContext.
func NewContextSelectorWithLimits(tokenBudget, maxFiles int) *ContextSelector {
	if tokenBudget <= 0 {
		tokenBudget = DefaultTokenBudget
	}
	if maxFiles <= 0 {
		maxFiles = MaxFilesInContext
	}
	return &ContextSelector{tokenBudget: tokenBudget, maxFiles: maxFiles}
}

// Select returns a map of filePath → content, restricted to the most relevant
// files that fit within the token budget.
//
// allFiles is the full project file map (path → content).
// errors is the list of raw build error strings used to identify hot files.
// recentlyModified is an ordered slice of paths (most recently modified first)
// used as a tiebreaker when relevance scores are equal.
func (cs *ContextSelector) Select(
	allFiles map[string]string,
	errors []string,
	recentlyModified []string,
) map[string]string {
	if len(allFiles) == 0 {
		return nil
	}

	// Fast path: if the total size already fits, return everything.
	if cs.fitsInBudget(allFiles) && len(allFiles) <= cs.maxFiles {
		out := make(map[string]string, len(allFiles))
		for k, v := range allFiles {
			out[k] = v
		}
		return out
	}

	entries := cs.scoreFiles(allFiles, errors, recentlyModified)

	// Sort descending by score, then ascending by path for stability.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score > entries[j].Score
		}
		return entries[i].Path < entries[j].Path
	})

	selected := make(map[string]string)
	tokensUsed := 0

	for _, e := range entries {
		if len(selected) >= cs.maxFiles {
			break
		}
		cost := estimateTokens(e.Content)
		if tokensUsed+cost > cs.tokenBudget {
			// Try to include a truncated version for high-value error files.
			if e.MentionedInError {
				truncated := truncateToTokenBudget(e.Content, cs.tokenBudget-tokensUsed)
				if truncated != "" {
					selected[e.Path] = truncated
					tokensUsed += estimateTokens(truncated)
				}
			}
			continue
		}
		selected[e.Path] = e.Content
		tokensUsed += cost
	}

	return selected
}

// scoreFiles assigns a relevance score to each file based on error mentions,
// recency, and import relationships.
func (cs *ContextSelector) scoreFiles(
	allFiles map[string]string,
	errors []string,
	recentlyModified []string,
) []FileEntry {

	// Build a set of files mentioned in errors (by path fragment).
	errorPaths := extractPathsFromErrors(errors)

	// Build recency rank map (lower index = more recent = higher bonus).
	recencyRank := make(map[string]int, len(recentlyModified))
	for i, p := range recentlyModified {
		recencyRank[p] = i
	}

	// Build a simple import graph: for each file, which other files does it
	// reference? Used to boost files that import error-mentioned files.
	importedBy := buildImportGraph(allFiles)

	entries := make([]FileEntry, 0, len(allFiles))
	for path, content := range allFiles {
		score := 0
		mentionedInError := false

		// +100 if path is directly mentioned in an error message.
		for ep := range errorPaths {
			if strings.Contains(path, ep) || strings.Contains(ep, path) {
				score += 100
				mentionedInError = true
				break
			}
		}

		// +50 if this file imports a file mentioned in an error.
		if !mentionedInError {
			for imported := range importedBy[path] {
				if errorPaths[imported] {
					score += 50
					break
				}
			}
		}

		// +30 for package.json / tsconfig / vite.config — always useful.
		base := baseName(path)
		if base == "package.json" || base == "tsconfig.json" || strings.HasPrefix(base, "vite.config") {
			score += 30
		}

		// Recency bonus: up to +40 for the most recently modified file,
		// tapering linearly down to 0 for items beyond position 10.
		if rank, ok := recencyRank[path]; ok && rank < 10 {
			score += 40 - rank*4
		}

		// Slight penalty for very large files that burn the budget.
		if estimateTokens(content) > 4000 {
			score -= 10
		}

		entries = append(entries, FileEntry{
			Path:             path,
			Content:          content,
			Score:            score,
			MentionedInError: mentionedInError,
		})
	}

	return entries
}

// fitsInBudget reports whether the total content of allFiles fits in budget.
func (cs *ContextSelector) fitsInBudget(allFiles map[string]string) bool {
	total := 0
	for _, v := range allFiles {
		total += estimateTokens(v)
		if total > cs.tokenBudget {
			return false
		}
	}
	return true
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// estimateTokens approximates the number of tokens in s using the 4-chars/token
// heuristic. It counts runes (not bytes) for better accuracy with UTF-8 content.
func estimateTokens(s string) int {
	return (utf8.RuneCountInString(s) + charsPerToken - 1) / charsPerToken
}

// truncateToTokenBudget trims s to approximately tokenBudget tokens and appends
// a truncation notice. Returns "" if the budget is too small to be useful.
func truncateToTokenBudget(s string, tokenBudget int) string {
	if tokenBudget < 50 {
		return ""
	}
	maxChars := tokenBudget * charsPerToken
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	suffix := "\n// ... (truncated for context budget)"
	keep := maxChars - len([]rune(suffix))
	if keep <= 0 {
		return ""
	}
	return string(runes[:keep]) + suffix
}

// pathInErrorRe matches file paths that appear in common error message formats.
var pathInErrorRe = regexp.MustCompile(`(?:^|[\s('"\\])([./\w-]+\.[a-zA-Z]{1,6})(?::\d+)?`)

// extractPathsFromErrors extracts file-path fragments from error strings.
// Returns a set of base-name fragments for fast containment checks.
func extractPathsFromErrors(errors []string) map[string]bool {
	paths := make(map[string]bool)
	for _, e := range errors {
		for _, m := range pathInErrorRe.FindAllStringSubmatch(e, -1) {
			if len(m) >= 2 && m[1] != "" {
				p := strings.TrimLeft(m[1], "./")
				if p != "" {
					paths[p] = true
					// Also add the base name so a short match like "App.tsx" hits "src/App.tsx".
					paths[baseName(p)] = true
				}
			}
		}
	}
	return paths
}

// importRe matches ES/TS import statements and require() calls to extract
// the module specifier (relative paths only — not node_modules).
var importRe = regexp.MustCompile(`(?:import\s+.*?from\s+|require\s*\(\s*)['"](\./[^'"]+|\.\.\/[^'"]+)['"]`)

// buildImportGraph builds a map: filePath → set of files it imports.
// Only relative imports are considered (node_modules are skipped).
func buildImportGraph(allFiles map[string]string) map[string]map[string]bool {
	graph := make(map[string]map[string]bool, len(allFiles))
	for path, content := range allFiles {
		for _, m := range importRe.FindAllStringSubmatch(content, -1) {
			if len(m) < 2 {
				continue
			}
			spec := m[1]
			if graph[path] == nil {
				graph[path] = make(map[string]bool)
			}
			graph[path][spec] = true
		}
	}
	return graph
}

// baseName returns the file name component of a path (everything after the
// last "/" or "\").
func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
