package agents

import (
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	previewVerificationInstallTimeout = 12 * time.Second
	previewVerificationBuildTimeout = 12 * time.Second
	previewVerificationNodeTestTimeout = 12 * time.Second

	os.Exit(m.Run())
}
