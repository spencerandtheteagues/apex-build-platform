// Package bundler - Tests for the bundler package
package bundler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultBundleConfig(t *testing.T) {
	config := DefaultBundleConfig()

	if config.Format != "esm" {
		t.Errorf("Expected format 'esm', got '%s'", config.Format)
	}
	if config.Minify != false {
		t.Error("Expected Minify to be false by default")
	}
	if config.SourceMap != true {
		t.Error("Expected SourceMap to be true by default")
	}
	if len(config.Target) != 1 || config.Target[0] != "es2020" {
		t.Error("Expected Target to be ['es2020']")
	}
	if config.Framework != "vanilla" {
		t.Errorf("Expected framework 'vanilla', got '%s'", config.Framework)
	}
}

func TestReactBundleConfig(t *testing.T) {
	config := ReactBundleConfig()

	if config.Framework != "react" {
		t.Errorf("Expected framework 'react', got '%s'", config.Framework)
	}
	if config.JSXImportSource != "react" {
		t.Errorf("Expected JSXImportSource 'react', got '%s'", config.JSXImportSource)
	}
	if config.Define["process.env.NODE_ENV"] != `"development"` {
		t.Error("Expected process.env.NODE_ENV to be defined")
	}
}

func TestComputeFileHash(t *testing.T) {
	files := map[string]string{
		"index.js": "console.log('hello');",
		"style.css": "body { color: red; }",
	}

	hash1 := ComputeFileHash(files)
	if hash1 == "" {
		t.Error("Expected non-empty hash")
	}

	// Same files should produce same hash
	hash2 := ComputeFileHash(files)
	if hash1 != hash2 {
		t.Error("Expected same hash for same content")
	}

	// Different files should produce different hash
	files["index.js"] = "console.log('world');"
	hash3 := ComputeFileHash(files)
	if hash1 == hash3 {
		t.Error("Expected different hash for different content")
	}
}

func TestComputeCacheKey(t *testing.T) {
	config := BundleConfig{
		EntryPoint: "src/index.tsx",
		Format:     "esm",
		Minify:     false,
		SourceMap:  true,
		Framework:  "react",
	}

	key1 := ComputeCacheKey(1, config, "hash123")
	if key1 == "" {
		t.Error("Expected non-empty cache key")
	}
	if len(key1) != 32 {
		t.Errorf("Expected cache key length 32, got %d", len(key1))
	}

	// Different project ID should produce different key
	key2 := ComputeCacheKey(2, config, "hash123")
	if key1 == key2 {
		t.Error("Expected different key for different project ID")
	}

	// Different file hash should produce different key
	key3 := ComputeCacheKey(1, config, "hash456")
	if key1 == key3 {
		t.Error("Expected different key for different file hash")
	}
}

func TestValidateConfig(t *testing.T) {
	// Valid config
	config := BundleConfig{
		EntryPoint: "src/index.tsx",
		Format:     "esm",
		Framework:  "react",
	}
	errors := ValidateConfig(config)
	if len(errors) != 0 {
		t.Errorf("Expected no errors, got %v", errors)
	}

	// Missing entry point
	config.EntryPoint = ""
	errors = ValidateConfig(config)
	if len(errors) != 1 {
		t.Error("Expected error for missing entry point")
	}

	// Invalid format
	config.EntryPoint = "index.js"
	config.Format = "invalid"
	errors = ValidateConfig(config)
	if len(errors) != 1 {
		t.Error("Expected error for invalid format")
	}

	// Invalid framework
	config.Format = "esm"
	config.Framework = "invalid"
	errors = ValidateConfig(config)
	if len(errors) != 1 {
		t.Error("Expected error for invalid framework")
	}
}

func TestBundleCacheBasic(t *testing.T) {
	config := CacheConfig{
		MaxSize:         10,
		TTL:             5 * time.Second,
		CleanupInterval: 1 * time.Second,
	}
	cache := NewBundleCache(config)
	defer cache.Close()

	// Test set and get
	result := &BundleResult{
		OutputJS: []byte("console.log('test');"),
		Success:  true,
	}
	cache.Set("key1", result)

	got := cache.Get("key1")
	if got == nil {
		t.Fatal("Expected to get cached result")
	}
	if string(got.OutputJS) != "console.log('test');" {
		t.Error("Cached content doesn't match")
	}

	// Test miss
	got = cache.Get("nonexistent")
	if got != nil {
		t.Error("Expected nil for non-existent key")
	}

	// Test stats
	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
	if stats.CurrentSize != 1 {
		t.Errorf("Expected size 1, got %d", stats.CurrentSize)
	}
}

func TestBundleCacheInvalidate(t *testing.T) {
	cache := NewBundleCache(DefaultCacheConfig())
	defer cache.Close()

	result := &BundleResult{Success: true}
	cache.Set("key1", result)
	cache.Set("key2", result)

	// Invalidate one key
	invalidated := cache.Invalidate("key1")
	if !invalidated {
		t.Error("Expected key to be invalidated")
	}

	if cache.Get("key1") != nil {
		t.Error("Expected key1 to be removed")
	}
	if cache.Get("key2") == nil {
		t.Error("Expected key2 to still exist")
	}
}

func TestBundleCacheLRU(t *testing.T) {
	config := CacheConfig{
		MaxSize:         3,
		TTL:             10 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
	cache := NewBundleCache(config)
	defer cache.Close()

	result := &BundleResult{Success: true}

	// Fill cache
	cache.Set("key1", result)
	cache.Set("key2", result)
	cache.Set("key3", result)

	// Access key1 to make it recently used
	cache.Get("key1")

	// Add key4, should evict key2 (least recently used)
	cache.Set("key4", result)

	if cache.Get("key1") == nil {
		t.Error("Expected key1 to still exist (recently accessed)")
	}
	if cache.Get("key2") != nil {
		t.Error("Expected key2 to be evicted (LRU)")
	}
	if cache.Get("key3") == nil {
		t.Error("Expected key3 to still exist")
	}
	if cache.Get("key4") == nil {
		t.Error("Expected key4 to exist")
	}
}

func TestESBuildBundlerAvailability(t *testing.T) {
	cache := NewBundleCache(DefaultCacheConfig())
	defer cache.Close()

	bundler := NewESBuildBundler(cache)

	// Just test that the bundler initializes without crashing
	// Actual availability depends on system having esbuild installed
	version := bundler.GetVersion()
	available := bundler.IsAvailable()

	t.Logf("esbuild available: %v, version: %s", available, version)

	// Test framework support
	if !bundler.SupportsFramework("react") {
		t.Error("Expected react to be supported")
	}
	if !bundler.SupportsFramework("vue") {
		t.Error("Expected vue to be supported")
	}
	if !bundler.SupportsFramework("vanilla") {
		t.Error("Expected vanilla to be supported")
	}
	if bundler.SupportsFramework("svelte") {
		t.Error("Expected svelte to not be supported (needs plugin)")
	}
}

func TestESBuildBundlerIntegration(t *testing.T) {
	cache := NewBundleCache(DefaultCacheConfig())
	defer cache.Close()

	bundler := NewESBuildBundler(cache)
	if !bundler.IsAvailable() {
		t.Skip("esbuild not available, skipping integration test")
	}

	// Create a temp directory with test files
	tmpDir, err := os.MkdirTemp("", "bundler-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write a simple React component
	indexContent := `
import React from 'react';
import { createRoot } from 'react-dom/client';

function App() {
    return <h1>Hello World</h1>;
}

const root = createRoot(document.getElementById('root'));
root.render(<App />);
`
	indexPath := filepath.Join(tmpDir, "src", "index.tsx")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}
	if err := os.WriteFile(indexPath, []byte(indexContent), 0644); err != nil {
		t.Fatalf("Failed to write index.tsx: %v", err)
	}

	// Create a simple package.json
	packageJSON := `{
  "name": "test-app",
  "version": "1.0.0",
  "dependencies": {
    "react": "^18.0.0",
    "react-dom": "^18.0.0"
  }
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	// Create project files
	files := ProjectFiles{
		ProjectID: 1,
		Files: map[string]string{
			"src/index.tsx": indexContent,
			"package.json":  packageJSON,
		},
	}

	config := BundleConfig{
		EntryPoint:   "src/index.tsx",
		Format:       "iife",
		Minify:       false,
		SourceMap:    false,
		Target:       []string{"es2020"},
		Framework:    "react",
		ExternalDeps: []string{"react", "react-dom"}, // Mark as external since not installed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := bundler.BundleFromFiles(ctx, 1, files, config)
	if err != nil {
		t.Fatalf("Bundle failed: %v", err)
	}

	if !result.Success {
		for _, e := range result.Errors {
			t.Logf("Bundle error: %s (file: %s, line: %d)", e.Message, e.File, e.Line)
		}
		t.Fatal("Expected bundle to succeed")
	}

	if len(result.OutputJS) == 0 {
		t.Error("Expected non-empty JS output")
	}

	t.Logf("Bundle successful: %d bytes JS, %d bytes CSS, duration: %v",
		len(result.OutputJS), len(result.OutputCSS), result.Duration)
}

func TestGenerateHTML(t *testing.T) {
	result := &BundleResult{
		OutputJS:  []byte("console.log('test');"),
		OutputCSS: []byte("body { color: red; }"),
		Success:   true,
	}

	config := BundleConfig{
		Framework: "react",
		Format:    "esm",
	}

	html := GenerateHTML(result, config)

	// Check for essential elements
	if !containsStr(html, "<!DOCTYPE html>") {
		t.Error("Expected HTML doctype")
	}
	if !containsStr(html, "console.log('test');") {
		t.Error("Expected JS content in HTML")
	}
	if !containsStr(html, "body { color: red; }") {
		t.Error("Expected CSS content in HTML")
	}
	if !containsStr(html, `<div id="root">`) {
		t.Error("Expected root div for React")
	}
	if !containsStr(html, `type="module"`) {
		t.Error("Expected module script type for ESM")
	}
}

// Helper function
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
