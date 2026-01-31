// Package bundler provides JavaScript/TypeScript bundling capabilities for APEX.BUILD
// Supports React, Vue, and vanilla JavaScript projects using esbuild as the underlying bundler
package bundler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

// Bundler defines the interface for code bundling operations
type Bundler interface {
	// Bundle bundles the project files according to the provided configuration
	Bundle(ctx context.Context, projectID uint, config BundleConfig) (*BundleResult, error)
	// SupportsFramework returns true if the bundler supports the given framework
	SupportsFramework(framework string) bool
	// IsAvailable returns true if the bundler (esbuild) is installed and available
	IsAvailable() bool
	// GetVersion returns the version of the underlying bundler
	GetVersion() string
}

// BundleConfig contains configuration for a bundle operation
type BundleConfig struct {
	// ProjectPath is the root path where project files are located
	ProjectPath string `json:"project_path"`
	// EntryPoint is the main entry file (e.g., "src/index.tsx", "src/main.js")
	EntryPoint string `json:"entry_point"`
	// Format specifies the output format: "esm" (ES modules) or "iife" (immediately invoked function expression)
	Format string `json:"format"`
	// Minify enables minification of the output
	Minify bool `json:"minify"`
	// SourceMap enables source map generation
	SourceMap bool `json:"source_map"`
	// Target specifies the JavaScript target versions (e.g., ["es2020", "chrome90"])
	Target []string `json:"target"`
	// Framework specifies the frontend framework: "react", "vue", "vanilla"
	Framework string `json:"framework"`
	// ExternalDeps lists packages that should not be bundled (loaded from CDN)
	ExternalDeps []string `json:"external_deps"`
	// Define allows defining global constants at build time
	Define map[string]string `json:"define"`
	// Alias allows module path aliasing
	Alias map[string]string `json:"alias"`
	// JSXFactory overrides the JSX factory function (default: React.createElement)
	JSXFactory string `json:"jsx_factory"`
	// JSXFragment overrides the JSX fragment (default: React.Fragment)
	JSXFragment string `json:"jsx_fragment"`
	// JSXImportSource sets the import source for automatic JSX runtime
	JSXImportSource string `json:"jsx_import_source"`
	// Loader specifies file type loaders (e.g., {".png": "dataurl"})
	Loader map[string]string `json:"loader"`
	// PublicPath sets the base path for assets in the output
	PublicPath string `json:"public_path"`
	// TreeShaking enables dead code elimination
	TreeShaking bool `json:"tree_shaking"`
	// Splitting enables code splitting for ESM output
	Splitting bool `json:"splitting"`
}

// BundleResult contains the result of a bundle operation
type BundleResult struct {
	// OutputJS contains the bundled JavaScript code
	OutputJS []byte `json:"output_js"`
	// OutputCSS contains the bundled CSS code (if any)
	OutputCSS []byte `json:"output_css,omitempty"`
	// SourceMap contains the source map (if enabled)
	SourceMap []byte `json:"source_map,omitempty"`
	// Duration is how long the bundling took
	Duration time.Duration `json:"duration"`
	// Warnings contains non-fatal warnings from the bundler
	Warnings []string `json:"warnings,omitempty"`
	// Errors contains fatal errors from the bundler
	Errors []BundleError `json:"errors,omitempty"`
	// Success indicates whether the bundle was successful
	Success bool `json:"success"`
	// Hash is the content hash of the output for cache invalidation
	Hash string `json:"hash"`
	// Metafile contains build metadata (imports, exports, etc.)
	Metafile *BundleMetafile `json:"metafile,omitempty"`
}

// BundleError represents a bundling error with source location
type BundleError struct {
	// Message is the error message
	Message string `json:"message"`
	// File is the file where the error occurred
	File string `json:"file,omitempty"`
	// Line is the line number (1-indexed)
	Line int `json:"line,omitempty"`
	// Column is the column number (0-indexed)
	Column int `json:"column,omitempty"`
	// Text is the source line text
	Text string `json:"text,omitempty"`
	// Suggestion is a suggested fix
	Suggestion string `json:"suggestion,omitempty"`
}

// BundleMetafile contains build metadata
type BundleMetafile struct {
	// Inputs lists all input files processed
	Inputs map[string]MetafileInput `json:"inputs"`
	// Outputs lists all output files produced
	Outputs map[string]MetafileOutput `json:"outputs"`
}

// MetafileInput describes an input file
type MetafileInput struct {
	// Bytes is the size of the input file
	Bytes int64 `json:"bytes"`
	// Imports lists the imports from this file
	Imports []MetafileImport `json:"imports,omitempty"`
}

// MetafileOutput describes an output file
type MetafileOutput struct {
	// Bytes is the size of the output file
	Bytes int64 `json:"bytes"`
	// Inputs lists which input files contributed to this output
	Inputs map[string]MetafileInputInfo `json:"inputs"`
	// Exports lists the exported names
	Exports []string `json:"exports,omitempty"`
}

// MetafileImport describes an import
type MetafileImport struct {
	// Path is the resolved import path
	Path string `json:"path"`
	// Kind is the import kind (import-statement, require-call, etc.)
	Kind string `json:"kind"`
	// External indicates if this import is external
	External bool `json:"external,omitempty"`
}

// MetafileInputInfo describes input contribution to output
type MetafileInputInfo struct {
	// BytesInOutput is the number of bytes this input contributed
	BytesInOutput int64 `json:"bytesInOutput"`
}

// ProjectFiles holds the files to be bundled (from database)
type ProjectFiles struct {
	// ProjectID is the database project ID
	ProjectID uint
	// Files maps file paths to their content
	Files map[string]string
	// PackageJSON holds the parsed package.json if present
	PackageJSON *PackageJSON
}

// PackageJSON represents a parsed package.json file
type PackageJSON struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Main            string            `json:"main"`
	Module          string            `json:"module"`
	Browser         string            `json:"browser"`
	Type            string            `json:"type"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Scripts         map[string]string `json:"scripts"`
}

// DefaultBundleConfig returns a sensible default configuration
func DefaultBundleConfig() BundleConfig {
	return BundleConfig{
		Format:      "esm",
		Minify:      false,
		SourceMap:   true,
		Target:      []string{"es2020"},
		Framework:   "vanilla",
		TreeShaking: true,
		Splitting:   false,
		Loader: map[string]string{
			".png":  "dataurl",
			".jpg":  "dataurl",
			".jpeg": "dataurl",
			".gif":  "dataurl",
			".svg":  "dataurl",
			".woff": "dataurl",
			".woff2": "dataurl",
			".ttf":   "dataurl",
			".eot":   "dataurl",
		},
	}
}

// ReactBundleConfig returns configuration optimized for React projects
func ReactBundleConfig() BundleConfig {
	config := DefaultBundleConfig()
	config.Framework = "react"
	config.JSXImportSource = "react"
	config.Define = map[string]string{
		"process.env.NODE_ENV": `"development"`,
	}
	return config
}

// VueBundleConfig returns configuration optimized for Vue projects
func VueBundleConfig() BundleConfig {
	config := DefaultBundleConfig()
	config.Framework = "vue"
	config.Define = map[string]string{
		"process.env.NODE_ENV": `"development"`,
		"__VUE_OPTIONS_API__":   "true",
		"__VUE_PROD_DEVTOOLS__": "false",
	}
	return config
}

// ComputeFileHash computes a SHA-256 hash of all file contents
func ComputeFileHash(files map[string]string) string {
	hasher := sha256.New()
	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(files))
	for path := range files {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	for _, path := range keys {
		hasher.Write([]byte(path))
		hasher.Write([]byte(files[path]))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// ComputeCacheKey generates a cache key from project ID, config, and file hash
func ComputeCacheKey(projectID uint, config BundleConfig, fileHash string) string {
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%d", projectID)))
	hasher.Write([]byte(config.EntryPoint))
	hasher.Write([]byte(config.Format))
	hasher.Write([]byte(fmt.Sprintf("%v", config.Minify)))
	hasher.Write([]byte(fmt.Sprintf("%v", config.SourceMap)))
	hasher.Write([]byte(config.Framework))
	hasher.Write([]byte(fileHash))
	return hex.EncodeToString(hasher.Sum(nil))[:32]
}
