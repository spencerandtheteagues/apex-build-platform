package ai

import "testing"

func TestNormalizeAPIKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "trims quotes and bearer prefix",
			in:   `"Bearer sk-proj-abc123"`,
			want: "sk-proj-abc123",
		},
		{
			name: "strips escaped and real control characters",
			in:   "sk-proj-abc\\n123\r\n\t",
			want: "sk-proj-abc123",
		},
		{
			name: "strips hidden unicode characters",
			in:   "sk-\u200bproj-\ufeffabc123",
			want: "sk-proj-abc123",
		},
		{
			name: "empty input",
			in:   "   ",
			want: "",
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeAPIKey(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeAPIKey(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
