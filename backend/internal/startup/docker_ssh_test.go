package startup

import (
	"strings"
	"testing"
)

func TestNormalizeSecretBlockExpandsLiteralNewlines(t *testing.T) {
	got := normalizeSecretBlock("line-one\\nline-two")
	if got != "line-one\nline-two\n" {
		t.Fatalf("normalizeSecretBlock returned %q", got)
	}
}

func TestCollectDockerSSHHostsParsesSSHHosts(t *testing.T) {
	hosts := collectDockerSSHHosts(
		"ssh://apexrunner@177.7.36.223",
		"ssh://apexrunner@177.7.36.223:2222",
		"tcp://runner.example.com:2376",
	)
	if len(hosts) != 2 {
		t.Fatalf("expected 2 unique ssh hosts, got %d", len(hosts))
	}
	if hosts[0].host != "177.7.36.223" {
		t.Fatalf("unexpected first host: %+v", hosts[0])
	}
}

func TestDockerSSHConfigIncludesIdentitySettings(t *testing.T) {
	t.Setenv("APEX_EXECUTION_DOCKER_HOST", "ssh://apexrunner@177.7.36.223")
	cfg := dockerSSHConfig("/tmp/id_ed25519", "/tmp/known_hosts")
	if !strings.Contains(cfg, "Host 177.7.36.223") {
		t.Fatalf("expected host entry, got %q", cfg)
	}
	if !strings.Contains(cfg, "IdentityFile /tmp/id_ed25519") {
		t.Fatalf("expected identity file entry, got %q", cfg)
	}
	if !strings.Contains(cfg, "StrictHostKeyChecking yes") {
		t.Fatalf("expected strict host key checking, got %q", cfg)
	}
}
