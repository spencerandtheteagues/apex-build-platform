package execution

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestCreateSandboxResponseResolvedAccessToken(t *testing.T) {
	t.Run("prefers current accessToken field", func(t *testing.T) {
		response := createSandboxResponse{
			AccessToken:       "current-token",
			LegacyAccessToken: "legacy-token",
		}
		if got := response.resolvedAccessToken(); got != "current-token" {
			t.Fatalf("resolvedAccessToken() = %q, want %q", got, "current-token")
		}
	})

	t.Run("falls back to legacy access_token field", func(t *testing.T) {
		response := createSandboxResponse{
			LegacyAccessToken: "legacy-token",
		}
		if got := response.resolvedAccessToken(); got != "legacy-token" {
			t.Fatalf("resolvedAccessToken() = %q, want %q", got, "legacy-token")
		}
	})
}

func TestBuildFileWriteCommandEncodesContentSafely(t *testing.T) {
	content := "console.log('hello from e2b')\n"
	command := buildFileWriteCommand("/code/main.js", content)

	if !strings.Contains(command, "mkdir -p '/code'") {
		t.Fatalf("command %q does not create the target directory", command)
	}
	if !strings.Contains(command, "base64 -d > '/code/main.js'") {
		t.Fatalf("command %q does not write the target file", command)
	}
	if !strings.Contains(command, shellQuote(base64.StdEncoding.EncodeToString([]byte(content)))) {
		t.Fatalf("command %q does not include the expected base64 payload", command)
	}
}
