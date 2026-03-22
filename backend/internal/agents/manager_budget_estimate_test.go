package agents

import "testing"

func TestEstimatedRequestCostUSDForBuildScalesByPowerMode(t *testing.T) {
	fast := estimatedRequestCostUSDForBuild(&Build{PowerMode: PowerFast})
	balanced := estimatedRequestCostUSDForBuild(&Build{PowerMode: PowerBalanced})
	max := estimatedRequestCostUSDForBuild(&Build{PowerMode: PowerMax})

	if !(fast < balanced && balanced < max) {
		t.Fatalf("expected fast < balanced < max, got fast=%0.4f balanced=%0.4f max=%0.4f", fast, balanced, max)
	}
}

func TestEstimatedRequestCostUSDForBuildUsesReasonableFallbacks(t *testing.T) {
	if got := estimatedRequestCostUSDForBuild(nil); got != defaultEstimatedRequestCostUSD {
		t.Fatalf("expected nil build fallback %0.4f, got %0.4f", defaultEstimatedRequestCostUSD, got)
	}

	if got := estimatedRequestCostUSDForBuild(&Build{Mode: ModeFull}); got != 0.06 {
		t.Fatalf("expected full-build fallback cost 0.06, got %0.4f", got)
	}
}
