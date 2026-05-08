package main

import (
	"os"
	"strings"
)

func executionLaunchReadinessDetails(status map[string]interface{}, forceContainer bool) map[string]interface{} {
	if status == nil {
		status = map[string]interface{}{}
	}
	e2bAvailable := boolMapValue(status, "e2b_available")
	containerAvailable := boolMapValue(status, "container_available")
	containerRequired := forceContainer || boolMapValue(status, "container_required")
	launchReady := !containerRequired || e2bAvailable || containerAvailable

	runtimeConfig := map[string]any{
		"e2b_configured":            envConfigured("E2B_API_KEY"),
		"docker_host_configured":    envAnyConfigured("APEX_EXECUTION_DOCKER_HOST", "DOCKER_HOST", "APEX_PREVIEW_DOCKER_HOST"),
		"docker_socket_configured":  envAnyConfigured("APEX_EXECUTION_DOCKER_SOCKET", "EXECUTION_DOCKER_SOCKET"),
		"docker_context_configured": envAnyConfigured("APEX_EXECUTION_DOCKER_CONTEXT", "DOCKER_CONTEXT", "APEX_PREVIEW_DOCKER_CONTEXT"),
	}

	issues := make([]string, 0, 2)
	missingEnv := make([]string, 0, 2)
	if !launchReady {
		issues = append(issues, "required_execution_sandbox_unavailable")
		if !envConfigured("E2B_API_KEY") {
			missingEnv = append(missingEnv, "E2B_API_KEY")
		}
		if !executionDockerConfigured() {
			missingEnv = append(missingEnv, "APEX_EXECUTION_DOCKER_HOST")
		}
		status["recommended_fix"] = "Set E2B_API_KEY or configure a reachable remote Docker runtime for code execution, then redeploy and confirm code_execution is ready."
	}

	status["force_container"] = forceContainer
	status["launch_ready"] = launchReady
	status["runtime_config"] = runtimeConfig
	if len(issues) > 0 {
		status["issues"] = issues
	}
	if len(missingEnv) > 0 {
		status["missing_env"] = missingEnv
	}

	return status
}

func previewLaunchReadinessDetails(status map[string]interface{}, forceContainer bool) map[string]interface{} {
	if status == nil {
		status = map[string]interface{}{}
	}

	sandboxRequired := boolMapValue(status, "sandbox_required") || forceContainer
	sandboxReady := boolMapValue(status, "sandbox_ready")
	sandboxDegraded := boolMapValue(status, "sandbox_degraded")
	serverRunner := nestedMapValue(status, "server_runner")
	backendRuntime := stringMapValue(serverRunner, "runtime")
	backendAvailable := boolMapValue(serverRunner, "available")
	backendRuntimeSetting := strings.ToLower(strings.TrimSpace(os.Getenv("APEX_PREVIEW_BACKEND_RUNTIME")))

	runtimeConfig := map[string]any{
		"backend_runtime_setting":           firstNonEmpty(backendRuntimeSetting, "auto"),
		"e2b_configured":                    envConfigured("E2B_API_KEY"),
		"preview_docker_host_configured":    envAnyConfigured("APEX_PREVIEW_DOCKER_HOST", "APEX_PREVIEW_DOCKER_SOCKET", "DOCKER_HOST"),
		"preview_docker_context_configured": envAnyConfigured("APEX_PREVIEW_DOCKER_CONTEXT", "DOCKER_CONTEXT"),
		"preview_connect_host_configured":   envConfigured("APEX_PREVIEW_CONNECT_HOST"),
	}

	issues := make([]string, 0, 3)
	missingEnv := make([]string, 0, 2)
	if sandboxRequired && !sandboxReady {
		issues = append(issues, "preview_sandbox_unavailable")
		if !previewDockerConfigured() {
			missingEnv = appendMissingEnv(missingEnv, "APEX_PREVIEW_DOCKER_HOST")
		}
	}
	if sandboxRequired && sandboxDegraded {
		issues = append(issues, "preview_sandbox_fallback")
		if !previewDockerConfigured() {
			missingEnv = appendMissingEnv(missingEnv, "APEX_PREVIEW_DOCKER_HOST")
		}
	}
	if !backendAvailable {
		issues = append(issues, "preview_backend_runtime_unavailable")
	}
	switch backendRuntimeSetting {
	case "container", "docker":
		if backendRuntime != "container" {
			issues = append(issues, "preview_backend_container_runtime_unavailable")
			if !previewDockerConfigured() {
				missingEnv = appendMissingEnv(missingEnv, "APEX_PREVIEW_DOCKER_HOST")
			}
		}
	case "e2b":
		if backendRuntime != "e2b" {
			issues = append(issues, "preview_backend_e2b_runtime_unavailable")
			if !envConfigured("E2B_API_KEY") {
				missingEnv = appendMissingEnv(missingEnv, "E2B_API_KEY")
			}
		}
	}

	launchReady := len(issues) == 0
	status["force_container"] = forceContainer
	status["launch_ready"] = launchReady
	status["runtime_config"] = runtimeConfig
	if len(issues) > 0 {
		status["issues"] = issues
		status["recommended_fix"] = "Configure a reachable preview Docker runtime or E2B preview runtime, then redeploy and confirm preview_service is ready."
	}
	if len(missingEnv) > 0 {
		status["missing_env"] = missingEnv
	}
	if sandboxDegraded && stringMapValue(serverRunner, "reason") == "" {
		serverRunner["reason"] = "Server Docker is unavailable, so preview is using process fallback mode"
		status["server_runner"] = serverRunner
	}

	return status
}

func boolMapValue(values map[string]interface{}, key string) bool {
	if values == nil {
		return false
	}
	value, _ := values[key].(bool)
	return value
}

func stringMapValue(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func nestedMapValue(values map[string]interface{}, key string) map[string]interface{} {
	if values == nil {
		return map[string]interface{}{}
	}
	if nested, ok := values[key].(map[string]interface{}); ok {
		return nested
	}
	return map[string]interface{}{}
}

func envConfigured(key string) bool {
	return strings.TrimSpace(os.Getenv(key)) != ""
}

func envAnyConfigured(keys ...string) bool {
	for _, key := range keys {
		if envConfigured(key) {
			return true
		}
	}
	return false
}

func executionDockerConfigured() bool {
	return envAnyConfigured(
		"APEX_EXECUTION_DOCKER_HOST",
		"APEX_EXECUTION_DOCKER_SOCKET",
		"EXECUTION_DOCKER_SOCKET",
		"APEX_EXECUTION_DOCKER_CONTEXT",
		"APEX_PREVIEW_DOCKER_HOST",
		"APEX_PREVIEW_DOCKER_CONTEXT",
		"DOCKER_HOST",
		"DOCKER_CONTEXT",
	)
}

func previewDockerConfigured() bool {
	return envAnyConfigured(
		"APEX_PREVIEW_DOCKER_HOST",
		"APEX_PREVIEW_DOCKER_SOCKET",
		"APEX_PREVIEW_DOCKER_CONTEXT",
		"DOCKER_HOST",
		"DOCKER_CONTEXT",
	)
}

func appendMissingEnv(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
