// Package preview - Preview Server Factory for APEX.BUILD
// Provides unified interface for both process-based and container-based previews.
package preview

import (
	"context"
	"fmt"
	"sync"

	"gorm.io/gorm"
)

// PreviewServerFactory manages both process and container preview servers
type PreviewServerFactory struct {
	db              *gorm.DB
	processServer   *PreviewServer
	containerServer *ContainerPreviewServer
	dockerAvailable bool
	mu              sync.RWMutex
}

// FactoryConfig holds configuration for the preview factory
type FactoryConfig struct {
	// Enable container-based previews when available
	EnableContainerPreviews bool

	// Container preview configuration
	ContainerConfig *ContainerPreviewConfig

	// Force container mode (fail if Docker not available)
	ForceContainerMode bool
}

// DefaultFactoryConfig returns sensible defaults
func DefaultFactoryConfig() *FactoryConfig {
	return &FactoryConfig{
		EnableContainerPreviews: true,
		ContainerConfig:         DefaultContainerPreviewConfig(),
		ForceContainerMode:      false,
	}
}

// NewPreviewServerFactory creates a new preview server factory
func NewPreviewServerFactory(db *gorm.DB, config *FactoryConfig) (*PreviewServerFactory, error) {
	if config == nil {
		config = DefaultFactoryConfig()
	}

	factory := &PreviewServerFactory{
		db: db,
	}

	// Create process-based preview server (always available)
	factory.processServer = NewPreviewServer(db)

	// Try to create container-based preview server
	if config.EnableContainerPreviews {
		containerServer, err := NewContainerPreviewServer(db, config.ContainerConfig)
		if err != nil {
			if config.ForceContainerMode {
				return nil, fmt.Errorf("container mode required but Docker not available: %w", err)
			}
			// Log warning and continue with process-based only
			fmt.Printf("Warning: Container previews disabled: %v\n", err)
			factory.dockerAvailable = false
		} else {
			factory.containerServer = containerServer
			factory.dockerAvailable = containerServer.IsDockerAvailable()
		}
	}

	return factory, nil
}

// StartPreview starts a preview session using the appropriate backend
func (f *PreviewServerFactory) StartPreview(ctx context.Context, config *PreviewConfig, useSandbox bool) (*PreviewStatus, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Use container preview if requested and available
	if useSandbox && f.dockerAvailable && f.containerServer != nil {
		return f.containerServer.StartContainerPreview(ctx, config)
	}

	// Fall back to process-based preview
	return f.processServer.StartPreview(ctx, config)
}

// StopPreview stops a preview session
func (f *PreviewServerFactory) StopPreview(ctx context.Context, projectID uint, useSandbox bool) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Try to stop both types (one will be no-op)
	if useSandbox && f.containerServer != nil {
		if err := f.containerServer.StopContainerPreview(ctx, projectID); err != nil {
			return err
		}
	}

	return f.processServer.StopPreview(ctx, projectID)
}

// GetPreviewStatus returns the status of a preview session
func (f *PreviewServerFactory) GetPreviewStatus(projectID uint, useSandbox bool) *PreviewStatus {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check container preview first if sandbox mode
	if useSandbox && f.containerServer != nil {
		status := f.containerServer.GetContainerPreviewStatus(projectID)
		if status.Active {
			return status
		}
	}

	// Check process-based preview
	return f.processServer.GetPreviewStatus(projectID)
}

// RefreshPreview triggers a reload of the preview
func (f *PreviewServerFactory) RefreshPreview(projectID uint, changedFiles []string, useSandbox bool) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if useSandbox && f.containerServer != nil {
		// For container previews, we may need to rebuild
		ctx := context.Background()
		return f.containerServer.RefreshContainerPreview(ctx, projectID, changedFiles)
	}

	return f.processServer.RefreshPreview(projectID, changedFiles)
}

// HotReload sends a hot reload update
func (f *PreviewServerFactory) HotReload(projectID uint, filePath, content string) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Hot reload only works with process-based previews
	return f.processServer.HotReload(projectID, filePath, content)
}

// IsDockerAvailable returns whether Docker is available for container previews
func (f *PreviewServerFactory) IsDockerAvailable() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.dockerAvailable
}

// GetAllPreviews returns all active preview sessions
func (f *PreviewServerFactory) GetAllPreviews() []*PreviewStatus {
	f.mu.RLock()
	defer f.mu.RUnlock()

	previews := f.processServer.GetAllPreviews()

	// Add container previews if available
	if f.containerServer != nil {
		f.containerServer.containerMu.RLock()
		for _, session := range f.containerServer.containerSessions {
			previews = append(previews, f.containerServer.getContainerStatus(session))
		}
		f.containerServer.containerMu.RUnlock()
	}

	return previews
}

// CleanupIdleSessions removes idle preview sessions
func (f *PreviewServerFactory) CleanupIdleSessions(maxIdleTime interface{}) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Process-based cleanup
	if duration, ok := maxIdleTime.(interface{ Milliseconds() int64 }); ok {
		_ = duration // Cleanup handled by the preview server
	}
	// Container cleanup is handled by the container server's cleanup loop
}

// GetProcessServer returns the process-based preview server
func (f *PreviewServerFactory) GetProcessServer() *PreviewServer {
	return f.processServer
}

// GetContainerServer returns the container-based preview server (may be nil)
func (f *PreviewServerFactory) GetContainerServer() *ContainerPreviewServer {
	return f.containerServer
}

// GetContainerStats returns container preview statistics
func (f *PreviewServerFactory) GetContainerStats() *ContainerPreviewStats {
	if f.containerServer == nil {
		return nil
	}
	return f.containerServer.GetStats()
}

// Cleanup cleans up all preview resources
func (f *PreviewServerFactory) Cleanup() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Stop all process-based previews
	for _, status := range f.processServer.GetAllPreviews() {
		ctx := context.Background()
		f.processServer.StopPreview(ctx, status.ProjectID)
	}

	// Cleanup container server if available
	if f.containerServer != nil {
		return f.containerServer.Cleanup()
	}

	return nil
}

// DockerStatusInfo provides Docker availability information
type DockerStatusInfo struct {
	Available       bool   `json:"available"`
	ContainerCount  int32  `json:"container_count"`
	MaxContainers   int32  `json:"max_containers"`
	TotalCreated    int64  `json:"total_created"`
	FailedCount     int64  `json:"failed_count"`
	TotalBuildTime  int64  `json:"total_build_time_ms"`
	TotalRuntime    int64  `json:"total_runtime_ms"`
}

// GetDockerStatus returns Docker availability and statistics
func (f *PreviewServerFactory) GetDockerStatus() *DockerStatusInfo {
	f.mu.RLock()
	defer f.mu.RUnlock()

	info := &DockerStatusInfo{
		Available: f.dockerAvailable,
	}

	if f.containerServer != nil && f.dockerAvailable {
		stats := f.containerServer.GetStats()
		info.ContainerCount = stats.ActiveContainers
		info.MaxContainers = f.containerServer.config.MaxContainers
		info.TotalCreated = stats.TotalContainersCreated
		info.FailedCount = stats.FailedContainers
		info.TotalBuildTime = stats.TotalBuildTime
		info.TotalRuntime = stats.TotalRuntime
	}

	return info
}
