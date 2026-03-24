package handlers

import (
	"testing"

	"apex-build/internal/execution"

	"github.com/stretchr/testify/require"
)

func TestShouldRequireLocalContainerSandbox(t *testing.T) {
	t.Run("requires docker when no e2b fallback exists", func(t *testing.T) {
		cfg := &execution.SandboxFactoryConfig{}
		require.True(t, shouldRequireLocalContainerSandbox(true, cfg))
	})

	t.Run("does not require docker when e2b is configured", func(t *testing.T) {
		cfg := &execution.SandboxFactoryConfig{
			EnableE2B: true,
			E2BApiKey: "e2b_test_key",
		}
		require.False(t, shouldRequireLocalContainerSandbox(true, cfg))
	})

	t.Run("respects disabled force container flag", func(t *testing.T) {
		cfg := &execution.SandboxFactoryConfig{
			EnableE2B: true,
			E2BApiKey: "e2b_test_key",
		}
		require.False(t, shouldRequireLocalContainerSandbox(false, cfg))
	})
}

func TestExecutionHandlerGetSandboxStatusIncludesE2BStats(t *testing.T) {
	factory, err := execution.NewSandboxFactory(&execution.SandboxFactoryConfig{
		EnableE2B:       true,
		PreferE2B:       true,
		E2BApiKey:       "e2b_test_key",
		PreferContainer: false,
		ProcessConfig:   execution.DefaultSandboxConfig(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = factory.Cleanup()
	})

	handler := &ExecutionHandler{
		SandboxFactory: factory,
	}

	status := handler.GetSandboxStatus()
	require.Equal(t, true, status["execution_enabled"])
	require.Equal(t, true, status["container_available"])
	require.Equal(t, true, status["e2b_available"])
	require.Equal(t, true, status["prefer_e2b"])
	require.Equal(t, true, status["container_isolation"])
}
