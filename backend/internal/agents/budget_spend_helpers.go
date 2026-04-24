package agents

// defaultEstimatedRequestCostUSDForPowerMode returns a conservative per-call
// cost estimate for spend tracking when BYOKManager pricing is unavailable.
func defaultEstimatedRequestCostUSDForPowerMode(mode PowerMode) float64 {
	switch mode {
	case PowerMax:
		return 0.15
	case PowerBalanced:
		return 0.06
	case PowerFast:
		return 0.02
	default:
		return 0.02
	}
}
