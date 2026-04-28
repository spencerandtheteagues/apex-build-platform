package preview

import (
	"strings"
	"testing"
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
