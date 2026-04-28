package preview

import "testing"

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
