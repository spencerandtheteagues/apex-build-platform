package origins

import (
	"net"
	"net/url"
	"os"
	"strings"
)

var defaultAllowedOrigins = []string{
	"http://localhost:3000",
	"http://localhost:3001",
	"http://localhost:5173",
	"http://localhost:5180",
	"http://localhost:8080",
	"http://127.0.0.1:3000",
	"http://127.0.0.1:5173",
	"http://127.0.0.1:5180",
	"http://127.0.0.1:8080",
	"https://apex-build.dev",
	"https://www.apex-build.dev",
	"https://apex.build",
	"https://www.apex.build",
	"https://apex-frontend-gigq.onrender.com",
}

func AllowedOrigins() []string {
	if raw := envOrigins(); raw != "" {
		parsed := splitAndTrim(raw)
		if len(parsed) > 0 {
			return parsed
		}
	}

	return append([]string(nil), defaultAllowedOrigins...)
}

func IsAllowedOrigin(origin string) bool {
	origin = normalizeOrigin(origin)
	if origin == "" {
		return false
	}

	if IsConfiguredOrigin(origin) {
		return true
	}

	if !IsProductionEnvironment() && isLoopbackOrigin(origin) {
		return true
	}

	return false
}

func IsConfiguredOrigin(origin string) bool {
	origin = normalizeOrigin(origin)
	if origin == "" {
		return false
	}

	for _, allowed := range AllowedOrigins() {
		if origin == normalizeOrigin(allowed) {
			return true
		}
	}

	return false
}

func PreviewFrameAncestors() string {
	ancestors := []string{
		"'self'",
		"https://apex-build.dev",
		"https://www.apex-build.dev",
		"https://apex.build",
		"https://www.apex.build",
	}

	if extra := strings.TrimSpace(os.Getenv("FRAME_ANCESTORS_EXTRA")); extra != "" {
		ancestors = append(ancestors, splitAndTrim(extra)...)
	} else if !IsProductionEnvironment() {
		ancestors = append(
			ancestors,
			"https://apex-frontend-gigq.onrender.com",
			"http://localhost:*",
			"http://127.0.0.1:*",
		)
	}

	seen := make(map[string]struct{}, len(ancestors))
	unique := make([]string, 0, len(ancestors))
	for _, ancestor := range ancestors {
		normalized := strings.TrimSpace(ancestor)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		unique = append(unique, normalized)
	}

	return strings.Join(unique, " ")
}

func IsProductionEnvironment() bool {
	for _, key := range []string{"GO_ENV", "APEX_ENV", "ENVIRONMENT", "ENV"} {
		value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		if value == "production" || value == "prod" {
			return true
		}
	}

	return false
}

func envOrigins() string {
	if value := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")); value != "" {
		return value
	}

	return strings.TrimSpace(os.Getenv("CORS_ORIGINS"))
}

func splitAndTrim(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func normalizeOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return ""
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed.String()
}

func isLoopbackOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
