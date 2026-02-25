package preview

import "testing"

func TestNormalizePreviewPath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: "   ", want: ""},
		{in: "/index.html", want: "index.html"},
		{in: "./index.html", want: "index.html"},
		{in: "\\src\\App.tsx", want: "src/App.tsx"},
		{in: ".", want: ""},
	}

	for _, tc := range tests {
		if got := normalizePreviewPath(tc.in); got != tc.want {
			t.Fatalf("normalizePreviewPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPreviewLookupVariationsIncludesNormalizedAndHtmlFallbacks(t *testing.T) {
	got := previewLookupVariations("/dashboard")
	want := []string{
		"dashboard",
		"dashboard.html",
		"dashboard/index.html",
		"public/dashboard",
		"src/dashboard",
	}

	if len(got) != len(want) {
		t.Fatalf("previewLookupVariations length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("previewLookupVariations[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestShouldProxyToBackendPathSupportsFullStackPrefixes(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/api/hello", want: true},
		{path: "/graphql", want: true},
		{path: "/trpc/posts.list", want: true},
		{path: "/socket.io/?EIO=4&transport=polling", want: true},
		{path: "/ws", want: true},
		{path: "/dashboard", want: false},
		{path: "/assets/app.js", want: false},
	}

	for _, tc := range tests {
		if got := shouldProxyToBackendPath(tc.path); got != tc.want {
			t.Fatalf("shouldProxyToBackendPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
