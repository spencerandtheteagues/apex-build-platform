package main

import "testing"

func TestPreviewRuntimeVerificationEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		environment string
		setting     string
		chromePath  string
		want        bool
	}{
		{name: "explicit true wins", environment: "development", setting: "true", chromePath: "", want: true},
		{name: "explicit false wins", environment: "production", setting: "false", chromePath: "/usr/bin/chromium-browser", want: false},
		{name: "production defaults on when chrome available", environment: "production", setting: "", chromePath: "/usr/bin/chromium-browser", want: true},
		{name: "production stays off without chrome", environment: "production", setting: "", chromePath: "", want: false},
		{name: "development default stays off", environment: "development", setting: "", chromePath: "/usr/bin/chromium-browser", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := previewRuntimeVerificationEnabled(tc.environment, tc.setting, tc.chromePath); got != tc.want {
				t.Fatalf("previewRuntimeVerificationEnabled(%q, %q, %q) = %v, want %v", tc.environment, tc.setting, tc.chromePath, got, tc.want)
			}
		})
	}
}
