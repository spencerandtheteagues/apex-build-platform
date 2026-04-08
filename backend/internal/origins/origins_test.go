package origins

import "testing"

func TestIsAllowedOriginIncludesLocalAPIProxyOrigin(t *testing.T) {
	if !IsAllowedOrigin("http://127.0.0.1:8080") {
		t.Fatal("expected local API proxy origin to be allowed")
	}
}

func TestIsAllowedOriginAllowsLoopbackDevelopmentOrigins(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")

	tests := []string{
		"http://127.0.0.1:9000",
		"http://localhost:4321",
		"http://[::1]:8080",
	}

	for _, origin := range tests {
		if !IsAllowedOrigin(origin) {
			t.Fatalf("expected loopback origin %q to be allowed in development", origin)
		}
	}
}

func TestIsAllowedOriginNormalizesFormatting(t *testing.T) {
	if !IsAllowedOrigin(" https://apex-build.dev/ ") {
		t.Fatal("expected normalized origin with whitespace and trailing slash to be allowed")
	}
}

func TestIsConfiguredOriginBlocksArbitraryDevelopmentLoopback(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")

	if IsConfiguredOrigin("http://localhost:4000") {
		t.Fatal("expected arbitrary loopback origin to require explicit configuration")
	}
	if !IsConfiguredOrigin("http://localhost:3000") {
		t.Fatal("expected explicitly configured localhost origin to remain allowed")
	}
}
