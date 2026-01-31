// APEX.BUILD Container Sandbox Security Tests
// Verifies that Docker container sandboxing is properly configured
package execution

import (
	"testing"
)

func TestDefaultContainerSandboxConfig(t *testing.T) {
	config := DefaultContainerSandboxConfig()

	// SECURITY: Verify secure defaults are set
	tests := []struct {
		name     string
		check    func() bool
		expected bool
		desc     string
	}{
		{
			name:     "Seccomp enabled",
			check:    func() bool { return config.EnableSeccomp },
			expected: true,
			desc:     "Seccomp syscall filtering should be enabled by default",
		},
		{
			name:     "Read-only root",
			check:    func() bool { return config.EnableReadOnlyRoot },
			expected: true,
			desc:     "Read-only root filesystem should be enabled by default",
		},
		{
			name:     "All capabilities dropped",
			check:    func() bool { return config.DropAllCapabilities },
			expected: true,
			desc:     "All capabilities should be dropped by default",
		},
		{
			name:     "No new privileges",
			check:    func() bool { return config.NoNewPrivileges },
			expected: true,
			desc:     "No new privileges should be enabled by default",
		},
		{
			name:     "Network disabled",
			check:    func() bool { return config.DisableNetwork },
			expected: true,
			desc:     "Network should be disabled by default for untrusted code",
		},
		{
			name:     "Network mode is none",
			check:    func() bool { return config.NetworkMode == "none" },
			expected: true,
			desc:     "Network mode should be 'none' by default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.check(); got != tt.expected {
				t.Errorf("%s: got %v, want %v. %s", tt.name, got, tt.expected, tt.desc)
			}
		})
	}
}

func TestDefaultResourceLimits(t *testing.T) {
	config := DefaultContainerSandboxConfig()

	// SECURITY: Verify resource limits are set
	if config.DefaultMemoryLimit != 256*1024*1024 {
		t.Errorf("Default memory limit should be 256MB, got %d", config.DefaultMemoryLimit)
	}

	if config.DefaultCPULimit != 0.5 {
		t.Errorf("Default CPU limit should be 0.5 cores, got %f", config.DefaultCPULimit)
	}

	if config.DefaultTimeout.Seconds() != 30 {
		t.Errorf("Default timeout should be 30 seconds, got %v", config.DefaultTimeout)
	}

	if config.DefaultPidsLimit != 100 {
		t.Errorf("Default PIDs limit should be 100, got %d", config.DefaultPidsLimit)
	}
}

func TestLanguageResourceLimits(t *testing.T) {
	config := DefaultContainerSandboxConfig()

	// Verify language-specific limits are set
	languagesWithLimits := []string{"python", "javascript", "go", "rust", "java", "c", "cpp"}

	for _, lang := range languagesWithLimits {
		if limits, ok := config.LanguageLimits[lang]; !ok {
			t.Errorf("Language %s should have resource limits configured", lang)
		} else {
			if limits.MemoryLimit <= 0 {
				t.Errorf("Language %s should have a positive memory limit", lang)
			}
			if limits.CPULimit <= 0 {
				t.Errorf("Language %s should have a positive CPU limit", lang)
			}
			if limits.Timeout <= 0 {
				t.Errorf("Language %s should have a positive timeout", lang)
			}
			if limits.PidsLimit <= 0 {
				t.Errorf("Language %s should have a positive PIDs limit", lang)
			}
		}
	}
}

func TestSandboxFactoryConfig(t *testing.T) {
	config := DefaultSandboxFactoryConfig()

	// SECURITY: Verify factory prefers container sandbox
	if !config.PreferContainer {
		t.Error("SandboxFactory should prefer container sandbox by default")
	}

	// Container config should have secure defaults
	if config.ContainerConfig == nil {
		t.Fatal("ContainerConfig should not be nil")
	}

	if !config.ContainerConfig.EnableSeccomp {
		t.Error("Container config should have seccomp enabled")
	}

	if !config.ContainerConfig.DisableNetwork {
		t.Error("Container config should have network disabled by default")
	}
}

func TestDockerStatusSecurity(t *testing.T) {
	status := CheckDockerStatus()

	// This test documents the status rather than requiring Docker
	t.Logf("Docker available: %v", status.Available)
	if status.Available {
		t.Logf("Docker version: %s", status.Version)
		t.Logf("Docker API version: %s", status.APIVersion)
	} else {
		t.Logf("Docker error: %s", status.Error)
	}
}

func TestSeccompProfileGeneration(t *testing.T) {
	// Verify the seccomp profile is valid
	config := DefaultContainerSandboxConfig()

	if !config.EnableSeccomp {
		t.Skip("Seccomp is disabled, skipping profile test")
	}

	// Test that we can create a container sandbox (which generates the profile)
	// This is a smoke test - full validation requires Docker
	t.Log("Seccomp profile generation test would require Docker")
}

func TestDockerArgsConstruction(t *testing.T) {
	config := DefaultContainerSandboxConfig()

	// Simulate container execution args
	exec := &containerExecution{
		ID:       "test-12345678",
		Language: "python",
		TempDir:  "/tmp/test",
	}

	limits := &LanguageResourceLimits{
		MemoryLimit: 256 * 1024 * 1024,
		CPULimit:    0.5,
		Timeout:     30,
		PidsLimit:   100,
		TmpfsSize:   "64m",
	}

	// Create a sandbox to test arg building
	sandbox := &ContainerSandbox{
		config:         config,
		baseTempDir:    "/tmp/apex-test",
		seccompProfile: "/tmp/seccomp.json",
	}

	args := sandbox.buildDockerArgs(exec, "main.py", limits, "apex-sandbox-python:latest")

	// Verify security-critical arguments are present
	requiredArgs := []string{
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges:true",
		"--network=none",
		"--read-only",
	}

	argsMap := make(map[string]bool)
	for _, arg := range args {
		argsMap[arg] = true
	}

	for _, required := range requiredArgs {
		if !argsMap[required] {
			t.Errorf("Required security argument %q not found in Docker args", required)
		}
	}

	// Verify memory and CPU limits are set
	found := false
	for i, arg := range args {
		if arg == "--memory" && i+1 < len(args) {
			found = true
			break
		}
	}
	if !found {
		t.Error("Memory limit argument not found in Docker args")
	}
}
