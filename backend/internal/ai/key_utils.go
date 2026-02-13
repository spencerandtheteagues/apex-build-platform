package ai

import "strings"

// normalizeAPIKey strips formatting noise that commonly appears in env-var values.
func normalizeAPIKey(raw string) string {
	key := strings.TrimSpace(raw)
	if key == "" {
		return ""
	}

	key = strings.Trim(key, `"'`)
	key = strings.TrimSpace(key)
	if len(key) >= len("bearer ") && strings.EqualFold(key[:len("bearer ")], "bearer ") {
		key = strings.TrimSpace(key[len("bearer "):])
	}

	// Strip both literal escapes and actual control characters.
	key = strings.ReplaceAll(key, `\r`, "")
	key = strings.ReplaceAll(key, `\n`, "")
	key = strings.ReplaceAll(key, "\r", "")
	key = strings.ReplaceAll(key, "\n", "")
	key = strings.ReplaceAll(key, "\t", "")

	// Keep only visible ASCII bytes to avoid malformed Authorization headers.
	filtered := make([]byte, 0, len(key))
	for i := 0; i < len(key); i++ {
		b := key[i]
		if b >= 33 && b <= 126 {
			filtered = append(filtered, b)
		}
	}

	return strings.TrimSpace(string(filtered))
}
