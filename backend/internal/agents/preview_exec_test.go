package agents

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunPreviewCheckCommandTimesOutProcessTree(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("process-group timeout semantics differ on windows")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh is not available")
	}

	start := time.Now()
	_, err := runPreviewCheckCommand(t.TempDir(), 1500*time.Millisecond, "sh", "-c", "sleep 30 & wait")
	if err == nil || !strings.Contains(err.Error(), "timed out after") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 6*time.Second {
		t.Fatalf("expected timeout cleanup to return promptly, took %s", elapsed)
	}
}
