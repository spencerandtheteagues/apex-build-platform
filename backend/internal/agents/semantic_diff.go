package agents

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// semanticDiffRoutingEnabled returns true unless APEX_SEMANTIC_DIFF_ROUTING is
// explicitly set to a falsy value. Default is enabled.
func semanticDiffRoutingEnabled() bool {
	val := strings.TrimSpace(strings.ToLower(os.Getenv("APEX_SEMANTIC_DIFF_ROUTING")))
	if val == "" {
		return true
	}
	return val == "1" || val == "true" || val == "yes" || val == "on"
}

// FileImportGraph maps each file path to the set of file paths it imports.
// Paths are normalized to the same form as GeneratedFile.Path.
type FileImportGraph map[string][]string

// reImport matches ES module static imports and re-exports.
// Handles: import ... from "path", export ... from "path", import("path").
var reImport = regexp.MustCompile(`(?:import|export)\s+(?:[^'"]*?\s+from\s+)?['"]([^'"]+)['"]`)

// buildFileImportGraph constructs a file-level import graph from generated
// files. Only intra-project imports (relative paths or @/ aliases) are
// followed; node_modules imports are ignored.
func buildFileImportGraph(files []GeneratedFile) FileImportGraph {
	// Index all files by path for quick lookup.
	pathIndex := make(map[string]struct{}, len(files))
	for _, f := range files {
		pathIndex[f.Path] = struct{}{}
	}

	graph := make(FileImportGraph, len(files))

	for _, f := range files {
		lower := strings.ToLower(f.Path)
		// Only parse files that can contain ES imports.
		if !isSupportedImportFileType(lower) {
			continue
		}

		matches := reImport.FindAllStringSubmatch(f.Content, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			spec := m[1]
			resolved := resolveImportSpecifier(f.Path, spec, pathIndex)
			if resolved == "" {
				continue
			}
			graph[f.Path] = append(graph[f.Path], resolved)
		}
	}

	return graph
}

// isSupportedImportFileType returns true for file types that may contain ES
// import statements.
func isSupportedImportFileType(lower string) bool {
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// resolveImportSpecifier converts an import specifier to a normalized file
// path that matches one of the keys in pathIndex, or returns "" when the
// import cannot be resolved to a project file.
func resolveImportSpecifier(fromPath, spec string, pathIndex map[string]struct{}) string {
	// Ignore node_modules and absolute non-alias imports.
	if !strings.HasPrefix(spec, ".") && !strings.HasPrefix(spec, "@/") {
		return ""
	}

	var candidate string
	if strings.HasPrefix(spec, "@/") {
		// Vite/tsconfig @/ alias → src/
		candidate = "src/" + spec[2:]
	} else {
		// Relative import: resolve against the directory of the importing file.
		dir := filepath.Dir(fromPath)
		candidate = filepath.Join(dir, spec)
		// Normalize to forward slashes (generated file paths use /).
		candidate = filepath.ToSlash(candidate)
	}

	// Exact match.
	if _, ok := pathIndex[candidate]; ok {
		return candidate
	}

	// Try common extensions.
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx"} {
		with := candidate + ext
		if _, ok := pathIndex[with]; ok {
			return with
		}
	}

	// Try index file inside the directory.
	for _, idx := range []string{"/index.ts", "/index.tsx", "/index.js"} {
		with := candidate + idx
		if _, ok := pathIndex[with]; ok {
			return with
		}
	}

	return ""
}

// reverseFileImportGraph builds the inverse graph: file → set of files that
// import it. This is the propagation direction for impact analysis.
func reverseFileImportGraph(graph FileImportGraph) FileImportGraph {
	rev := make(FileImportGraph, len(graph))
	for importer, deps := range graph {
		for _, dep := range deps {
			rev[dep] = append(rev[dep], importer)
		}
	}
	return rev
}

// resolveAffectedFiles performs a BFS from changedPaths over the reverse
// import graph to find every file that transitively imports a changed file.
// Returns the union of changedPaths and all transitive importers, sorted.
//
// If the graph is empty or changedPaths is empty, returns nil (caller should
// fall back to all files).
func resolveAffectedFiles(changedPaths []string, revGraph FileImportGraph) []string {
	if len(changedPaths) == 0 || len(revGraph) == 0 {
		return nil
	}

	visited := make(map[string]struct{}, len(changedPaths)*4)
	queue := make([]string, 0, len(changedPaths)*4)

	for _, p := range changedPaths {
		if _, seen := visited[p]; !seen {
			visited[p] = struct{}{}
			queue = append(queue, p)
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, importer := range revGraph[current] {
			if _, seen := visited[importer]; !seen {
				visited[importer] = struct{}{}
				queue = append(queue, importer)
			}
		}
	}

	result := make([]string, 0, len(visited))
	for p := range visited {
		result = append(result, p)
	}
	sort.Strings(result)
	return result
}

// semanticDiffHint holds the affected-file scope computed for a revision task.
type semanticDiffHint struct {
	// AffectedFiles is the computed transitive impact set. nil means unknown
	// (fall back to full regeneration).
	AffectedFiles []string
	// Uncertainty is true when the graph was too thin to trust the result.
	Uncertainty bool
}

// minFilesForGraphTrust is the minimum number of files a build must have
// before we trust the import graph for scoping. Builds with fewer files
// tend to be small enough that full regeneration is cheap.
const minFilesForGraphTrust = 6

// computeSemanticDiffHint returns a hint describing which files need to be
// touched for a follow-up revision that mentions the given keywords.
//
// When the flag is off, the build is tiny, or the graph cannot be built,
// returns a hint with Uncertainty=true so callers fall back to full
// regeneration.
func computeSemanticDiffHint(build *Build, allFiles []GeneratedFile, userRequest string) semanticDiffHint {
	if !semanticDiffRoutingEnabled() {
		return semanticDiffHint{Uncertainty: true}
	}

	if len(allFiles) < minFilesForGraphTrust {
		return semanticDiffHint{Uncertainty: true}
	}

	graph := buildFileImportGraph(allFiles)
	if len(graph) == 0 {
		return semanticDiffHint{Uncertainty: true}
	}

	rev := reverseFileImportGraph(graph)

	// Identify directly-mentioned files by scanning the user request for path
	// substrings that match known file paths.
	mentionedPaths := extractMentionedPaths(userRequest, allFiles)

	// If the user didn't mention specific files, we can't scope the change —
	// fall back to full regeneration to be safe.
	if len(mentionedPaths) == 0 {
		return semanticDiffHint{Uncertainty: true}
	}

	affected := resolveAffectedFiles(mentionedPaths, rev)
	if len(affected) == 0 {
		// Mentioned paths exist but nothing imports them — only those files change.
		affected = mentionedPaths
	}

	log.Printf("[semantic_diff] {\"build_id\":%q,\"changed_files\":%d,\"affected_files\":%d,\"total_files\":%d}",
		buildIDOrEmpty(build), len(mentionedPaths), len(affected), len(allFiles))

	return semanticDiffHint{AffectedFiles: affected}
}

// buildIDOrEmpty safely returns the build ID for logging.
func buildIDOrEmpty(build *Build) string {
	if build == nil {
		return ""
	}
	return build.ID
}

// extractMentionedPaths returns the subset of known file paths that appear
// as substrings in the user's request (case-insensitive, filename only).
func extractMentionedPaths(userRequest string, allFiles []GeneratedFile) []string {
	lower := strings.ToLower(userRequest)
	var out []string
	for _, f := range allFiles {
		base := strings.ToLower(filepath.Base(f.Path))
		// Require the base name to appear in the request so we don't over-match.
		if base != "" && strings.Contains(lower, base) {
			out = append(out, f.Path)
		}
	}
	// Deduplicate while preserving stable order.
	seen := make(map[string]struct{}, len(out))
	deduped := out[:0]
	for _, p := range out {
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			deduped = append(deduped, p)
		}
	}
	sort.Strings(deduped)
	return deduped
}
