package agents

import (
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	previewVerificationInstallTimeout = 6 * time.Second
	previewVerificationBuildTimeout = 6 * time.Second
	previewVerificationNodeTestTimeout = 6 * time.Second

	os.Exit(m.Run())
}
