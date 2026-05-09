package agents

import "testing"

func TestCostThresholdErrorIsProviderLevelFailure(t *testing.T) {
	err := "failed to select provider: estimated cost 0.117905 exceeds threshold 0.100000 for provider claude"
	if !isProviderLevelFailureMessage(err) {
		t.Fatalf("expected cost-threshold provider selection error to be provider-level")
	}
}
