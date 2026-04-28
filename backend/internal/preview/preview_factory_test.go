package preview

import (
	"testing"
	"time"
)

func TestPreviewFactoryDoesNotFallbackToProcessWhenSandboxRequested(t *testing.T) {
	t.Parallel()

	projectID := uint(53)
	now := time.Now()
	factory := &PreviewServerFactory{
		processServer: &PreviewServer{
			sessions: map[uint]*PreviewSession{
				projectID: {
					ProjectID:  projectID,
					Port:       9000,
					Clients:    map[*SafeClient]bool{},
					StartedAt:  now,
					LastAccess: now,
					stopChan:   make(chan struct{}),
				},
			},
			portMap: map[uint]int{projectID: 9000},
		},
		containerServer: &ContainerPreviewServer{
			PreviewServer: &PreviewServer{
				portMap: map[uint]int{},
			},
			containerSessions: map[uint]*ContainerSession{},
			config:            &ContainerPreviewConfig{ConnectHost: "177.7.36.223"},
			stats:             &ContainerPreviewStats{},
		},
		dockerAvailable: true,
	}

	sandboxStatus := factory.GetPreviewStatus(projectID, true)
	if sandboxStatus.Active {
		t.Fatalf("sandbox status fell back to process preview: %+v", sandboxStatus)
	}

	processStatus := factory.GetPreviewStatus(projectID, false)
	if !processStatus.Active {
		t.Fatalf("process preview status should remain active: %+v", processStatus)
	}
}
