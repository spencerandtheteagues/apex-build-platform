package startup

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// PrepareDockerSSHMaterial writes optional SSH material used by Docker's
// ssh:// host transport. It is intentionally a no-op unless env-backed
// runner credentials are configured.
func PrepareDockerSSHMaterial() (bool, error) {
	privateKey := normalizeSecretBlock(os.Getenv("DOCKER_SSH_PRIVATE_KEY"))
	knownHosts := normalizeSecretBlock(os.Getenv("DOCKER_SSH_KNOWN_HOSTS"))

	if privateKey == "" && knownHosts == "" {
		return false, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return true, fmt.Errorf("resolve user home for Docker SSH material: %w", err)
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return true, fmt.Errorf("create ssh dir: %w", err)
	}

	if privateKey != "" {
		if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519"), []byte(privateKey), 0o600); err != nil {
			return true, fmt.Errorf("write docker ssh private key: %w", err)
		}
	}

	if knownHosts != "" {
		if err := os.WriteFile(filepath.Join(sshDir, "known_hosts"), []byte(knownHosts), 0o644); err != nil {
			return true, fmt.Errorf("write docker ssh known_hosts: %w", err)
		}
	}

	config := dockerSSHConfig(filepath.Join(sshDir, "id_ed25519"), filepath.Join(sshDir, "known_hosts"))
	if strings.TrimSpace(config) != "" {
		if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(config), 0o600); err != nil {
			return true, fmt.Errorf("write docker ssh config: %w", err)
		}
	}

	return true, nil
}

func normalizeSecretBlock(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	value = strings.ReplaceAll(value, `\n`, "\n")

	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		decodedValue := strings.TrimSpace(string(decoded))
		if strings.Contains(decodedValue, "BEGIN OPENSSH PRIVATE KEY") || strings.Contains(decodedValue, "ssh-ed25519 ") || strings.Contains(decodedValue, "ssh-rsa ") {
			value = decodedValue
		}
	}

	if !strings.HasSuffix(value, "\n") {
		value += "\n"
	}
	return value
}

func dockerSSHConfig(identityFile, knownHostsFile string) string {
	hosts := collectDockerSSHHosts(
		os.Getenv("APEX_EXECUTION_DOCKER_HOST"),
		os.Getenv("APEX_PREVIEW_DOCKER_HOST"),
	)
	if len(hosts) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, host := range hosts {
		builder.WriteString("Host " + host.host + "\n")
		builder.WriteString("  HostName " + host.host + "\n")
		if host.port != "" {
			builder.WriteString("  Port " + host.port + "\n")
		}
		builder.WriteString("  IdentityFile " + identityFile + "\n")
		builder.WriteString("  IdentitiesOnly yes\n")
		builder.WriteString("  StrictHostKeyChecking yes\n")
		builder.WriteString("  UserKnownHostsFile " + knownHostsFile + "\n\n")
	}
	return builder.String()
}

type dockerSSHHost struct {
	host string
	port string
}

func collectDockerSSHHosts(values ...string) []dockerSSHHost {
	seen := map[string]struct{}{}
	hosts := make([]dockerSSHHost, 0, len(values))
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" || !strings.HasPrefix(raw, "ssh://") {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Hostname() == "" {
			continue
		}
		key := parsed.Host
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		hosts = append(hosts, dockerSSHHost{
			host: parsed.Hostname(),
			port: parsed.Port(),
		})
	}
	return hosts
}
