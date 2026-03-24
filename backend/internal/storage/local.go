package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalProvider implements storage using local disk
type LocalProvider struct {
	baseDir string
}

// NewLocalProvider creates a new local storage provider
func NewLocalProvider(baseDir string) (*LocalProvider, error) {
	if baseDir == "" {
		baseDir = "./uploads"
	}

	// Ensure base directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
	}

	return &LocalProvider{
		baseDir: baseDir,
	}, nil
}

// Put stores content to local disk
func (l *LocalProvider) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	filePath := filepath.Join(l.baseDir, key)

	// Create directory if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	// Copy content
	_, err = io.Copy(file, reader)
	if err != nil {
		// Clean up on failure
		os.Remove(filePath)
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

// Get reads content from local disk
func (l *LocalProvider) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	filePath := filepath.Join(l.baseDir, key)

	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, fmt.Errorf("file not found: %s", key)
		}
		return nil, 0, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	return file, info.Size(), nil
}

// Delete removes a file from local disk
func (l *LocalProvider) Delete(ctx context.Context, key string) error {
	filePath := filepath.Join(l.baseDir, key)

	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}

	return nil
}

// URL returns a relative URL for the object (served by the API)
func (l *LocalProvider) URL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	// For local storage, return a relative API URL
	// The actual serving is handled by the assets API handler
	return fmt.Sprintf("/api/v1/assets/raw/%s", key), nil
}

// Exists checks if a file exists on local disk
func (l *LocalProvider) Exists(ctx context.Context, key string) (bool, error) {
	filePath := filepath.Join(l.baseDir, key)

	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence %s: %w", filePath, err)
	}

	return true, nil
}