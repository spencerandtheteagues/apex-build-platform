package ai

import "strings"

// normalizeAPIKey strips formatting noise that commonly appears in env-var values.
func normalizeAPIKey(raw string) string {
	key := strings.TrimSpace(raw)
	key = strings.Trim(key, `"'`)
	key = strings.TrimSpace(key)
	key = strings.TrimPrefix(key, "Bearer ")
	key = strings.TrimPrefix(key, "bearer ")
	key = strings.ReplaceAll(key, "\r", "")
	key = strings.ReplaceAll(key, "\n", "")
	return strings.TrimSpace(key)
}
