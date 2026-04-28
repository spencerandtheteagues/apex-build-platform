package preview

import (
	"strings"
	"testing"
)

func TestEnvPortUsesLastPORTOverride(t *testing.T) {
	got := envPort([]string{
		"PORT=8080",
		"HOST=0.0.0.0",
		"PORT=9100",
	})
	if got != 9100 {
		t.Fatalf("expected last PORT override to win, got %d", got)
	}
}

func TestFilteredPreviewBackendEnvUsesLastIncludedValue(t *testing.T) {
	t.Setenv("SECRET_KEY", "host-secret")

	got := filteredPreviewBackendEnv([]string{
		"PORT=8080",
		"VITE_API_URL=http://old.example",
		"SECRET_KEY=host-secret",
		"CUSTOM_TOKEN=generated-token",
		"PORT=9100",
		"VITE_API_URL=http://new.example",
	})

	values := make(map[string]string, len(got))
	for _, entry := range got {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			t.Fatalf("invalid env entry %q", entry)
		}
		values[key] = value
	}

	if values["PORT"] != "9100" {
		t.Fatalf("expected last PORT value, got %q from %+v", values["PORT"], got)
	}
	if values["VITE_API_URL"] != "http://new.example" {
		t.Fatalf("expected last VITE_API_URL value, got %q from %+v", values["VITE_API_URL"], got)
	}
	if _, ok := values["SECRET_KEY"]; ok {
		t.Fatalf("expected host secret to be filtered, got %+v", got)
	}
	if values["CUSTOM_TOKEN"] != "generated-token" {
		t.Fatalf("expected generated env to be retained, got %+v", got)
	}
}
