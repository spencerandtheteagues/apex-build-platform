package agents

import (
	"time"

	"apex-build/internal/ai"
)

// defaultGenerateTimeout provides a single provider-aware timeout policy for
// ad-hoc generation flows that do not already carry an explicit deadline.
func defaultGenerateTimeout(provider ai.AIProvider, mode PowerMode) time.Duration {
	if mode == "" {
		mode = PowerFast
	}

	if provider == ai.ProviderOllama {
		switch mode {
		case PowerMax:
			return 6 * time.Minute
		case PowerBalanced:
			return 5 * time.Minute
		default:
			return 4 * time.Minute
		}
	}

	switch mode {
	case PowerMax:
		return 5 * time.Minute
	case PowerBalanced:
		return 3 * time.Minute
	default:
		return 2 * time.Minute
	}
}
