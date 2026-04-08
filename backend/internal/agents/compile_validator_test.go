package agents

import "testing"

func TestMaxCompileAttemptsByPowerMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode PowerMode
		want int
	}{
		{name: "fast", mode: PowerFast, want: 1},
		{name: "balanced", mode: PowerBalanced, want: 2},
		{name: "max", mode: PowerMax, want: 3},
		{name: "unknown defaults to fast behavior", mode: PowerMode("unknown"), want: 1},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := maxCompileAttempts(tt.mode); got != tt.want {
				t.Fatalf("maxCompileAttempts(%q) = %d, want %d", tt.mode, got, tt.want)
			}
		})
	}
}
