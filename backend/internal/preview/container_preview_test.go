package preview

import (
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDerivePreviewConnectHostFromSSHDockerHost(t *testing.T) {
	t.Parallel()

	got := derivePreviewConnectHost("ssh://apexrunner@177.7.36.223")
	if got != "177.7.36.223" {
		t.Fatalf("connect host = %q, want 177.7.36.223", got)
	}
}

func TestContainerPreviewURLUsesRemoteConnectHost(t *testing.T) {
	t.Parallel()

	server := &ContainerPreviewServer{
		config: &ContainerPreviewConfig{ConnectHost: "177.7.36.223"},
	}

	got := server.previewURL(10000)
	if got != "http://177.7.36.223:10000" {
		t.Fatalf("preview URL = %q, want remote host URL", got)
	}

	if got := server.previewDialAddress(10000); got != "177.7.36.223:10000" {
		t.Fatalf("preview dial address = %q, want remote host address", got)
	}
}

func TestContainerPreviewURLFallsBackToLocalhost(t *testing.T) {
	t.Parallel()

	server := &ContainerPreviewServer{
		config: &ContainerPreviewConfig{},
	}

	got := server.previewURL(10000)
	if got != "http://localhost:10000" {
		t.Fatalf("preview URL = %q, want localhost URL", got)
	}

	if got := server.previewDialAddress(10000); got != "localhost:10000" {
		t.Fatalf("preview dial address = %q, want localhost address", got)
	}
}

func TestNodeDockerfileInstallsDevDependenciesForPreviewBuilds(t *testing.T) {
	t.Parallel()

	server := &ContainerPreviewServer{}
	dockerfile := server.nodeDockerfile()

	for _, forbidden := range []string{
		"npm install --production",
		"npm ci --production",
		"npm run build 2>/dev/null || true",
		"npm run build || true",
	} {
		if strings.Contains(dockerfile, forbidden) {
			t.Fatalf("node preview Dockerfile must not contain %q:\n%s", forbidden, dockerfile)
		}
	}
	for _, required := range []string{
		"npm ci --include=dev",
		"npm install --include=dev",
		"npm run build;",
	} {
		if !strings.Contains(dockerfile, required) {
			t.Fatalf("node preview Dockerfile missing %q:\n%s", required, dockerfile)
		}
	}
}

func TestContainerPreviewBuildUsesDockerLayerCache(t *testing.T) {
	t.Parallel()

	server := &ContainerPreviewServer{}
	args := server.dockerBuildArgs("/tmp/apex-preview", "apex-preview-1:latest")
	joined := strings.Join(args, " ")

	if strings.Contains(joined, "--no-cache") {
		t.Fatalf("preview container builds should use Docker layer cache, got args: %v", args)
	}
}

func TestAssignContainerPortSkipsLocallyOccupiedBasePort(t *testing.T) {
	occupied, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("reserve occupied port: %v", err)
	}
	defer occupied.Close()
	basePort := occupied.Addr().(*net.TCPAddr).Port

	projectID := uint(601)
	server := &ContainerPreviewServer{
		PreviewServer: &PreviewServer{
			portMap: map[uint]int{},
		},
		config: &ContainerPreviewConfig{BasePort: basePort},
	}

	port, err := server.assignContainerPort(projectID)
	if err != nil {
		t.Fatalf("assign container port: %v", err)
	}
	if port == basePort {
		t.Fatalf("container preview reused occupied port %d", basePort)
	}
	if got := server.portMap[projectID]; got != port {
		t.Fatalf("portMap[%d] = %d, want %d", projectID, got, port)
	}
}

func TestAssignContainerPortAvoidsBlockedRetryPort(t *testing.T) {
	t.Parallel()

	projectID := uint(602)
	server := &ContainerPreviewServer{
		PreviewServer: &PreviewServer{
			portMap: map[uint]int{projectID: 10000},
		},
		config: &ContainerPreviewConfig{BasePort: 10000, ConnectHost: "177.7.36.223"},
	}

	port, err := server.assignContainerPortAvoiding(projectID, map[int]bool{10000: true})
	if err != nil {
		t.Fatalf("assign retry container port: %v", err)
	}
	if port != 10001 {
		t.Fatalf("retry port = %d, want 10001", port)
	}
	if got := server.portMap[projectID]; got != 10001 {
		t.Fatalf("portMap[%d] = %d, want 10001", projectID, got)
	}
}

func TestDockerPortAllocationOutputClassifiesRetryableBindFailures(t *testing.T) {
	t.Parallel()

	retryable := []string{
		"docker run failed: exit status 125\nOutput: Bind for 0.0.0.0:10000 failed: port is already allocated",
		"Error response from daemon: Ports are not available: exposing port TCP 0.0.0.0:10000: listen tcp 0.0.0.0:10000: bind: address already in use",
	}
	for _, message := range retryable {
		if !dockerPortAllocationOutput(errors.New(message)) {
			t.Fatalf("expected retryable port allocation error: %q", message)
		}
	}
	if dockerPortAllocationOutput(errors.New("docker run failed: permission denied while trying to connect to the Docker daemon socket")) {
		t.Fatal("permission errors must not be treated as retryable port allocation")
	}
}

func TestPreviewContainerNamesAreDeterministic(t *testing.T) {
	t.Parallel()

	server := &ContainerPreviewServer{
		config: &ContainerPreviewConfig{ImagePrefix: "custom-preview"},
	}

	if got := server.previewContainerName(53); got != "apex-preview-53" {
		t.Fatalf("container name = %q, want apex-preview-53", got)
	}
	if got := server.previewImageName(53); got != "custom-preview-53:latest" {
		t.Fatalf("image name = %q, want custom-preview-53:latest", got)
	}
}

func TestDockerMissingResourceOutputClassifiesIdempotentCleanup(t *testing.T) {
	t.Parallel()

	missingMessages := []string{
		"Error response from daemon: No such container: apex-preview-53",
		"Error response from daemon: No such image: apex-preview-53:latest",
		"Error: No such object: apex-preview-53",
	}
	for _, message := range missingMessages {
		if !dockerMissingResourceOutput([]byte(message)) {
			t.Fatalf("expected missing resource output to be ignored: %q", message)
		}
	}

	if dockerMissingResourceOutput([]byte("permission denied while trying to connect to the Docker daemon socket")) {
		t.Fatal("permission errors must not be treated as idempotent cleanup")
	}
}

func TestContainerPreviewStatusDropsDeadSession(t *testing.T) {
	t.Parallel()

	projectID := uint(53)
	server := &ContainerPreviewServer{
		PreviewServer: &PreviewServer{
			portMap: map[uint]int{projectID: 10000},
		},
		containerSessions: map[uint]*ContainerSession{
			projectID: {
				ProjectID:     projectID,
				ContainerID:   "missing-container",
				ContainerName: "apex-preview-53",
				Port:          10000,
				StartedAt:     time.Now(),
				LastAccess:    time.Now(),
			},
		},
		config: &ContainerPreviewConfig{ConnectHost: "177.7.36.223"},
		stats:  &ContainerPreviewStats{},
		containerRunningCheck: func(containerID string) bool {
			return false
		},
	}
	atomic.StoreInt32(&server.stats.ActiveContainers, 1)

	status := server.GetContainerPreviewStatus(projectID)
	if status.Active {
		t.Fatalf("dead container session reported active: %+v", status)
	}
	if _, exists := server.containerSessions[projectID]; exists {
		t.Fatal("dead container session was not removed from memory")
	}
	if _, exists := server.portMap[projectID]; exists {
		t.Fatal("dead container session port was not released")
	}
	if got := atomic.LoadInt32(&server.stats.ActiveContainers); got != 0 {
		t.Fatalf("active container count = %d, want 0", got)
	}
}

func TestContainerPreviewStatusRecoversMissingLiveSession(t *testing.T) {
	t.Parallel()

	projectID := uint(58)
	server := &ContainerPreviewServer{
		PreviewServer: &PreviewServer{
			portMap: map[uint]int{},
		},
		containerSessions: map[uint]*ContainerSession{},
		config:            &ContainerPreviewConfig{ConnectHost: "177.7.36.223"},
		stats:             &ContainerPreviewStats{},
		dockerAvailable:   true,
		sessionRecoverer: func(projectID uint) *ContainerSession {
			return &ContainerSession{
				ProjectID:     projectID,
				ContainerID:   "apex-preview-58",
				ContainerName: "apex-preview-58",
				Port:          10000,
				InternalPort:  3000,
				StartedAt:     time.Now(),
				LastAccess:    time.Now(),
				stopChan:      make(chan struct{}),
			}
		},
	}

	status := server.GetContainerPreviewStatus(projectID)
	if !status.Active {
		t.Fatalf("expected recovered container session to be active: %+v", status)
	}
	if status.Port != 10000 {
		t.Fatalf("port = %d, want 10000", status.Port)
	}
	if status.URL != "http://177.7.36.223:10000" {
		t.Fatalf("url = %q, want remote preview URL", status.URL)
	}
}
