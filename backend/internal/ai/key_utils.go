package ai

import (
	"fmt"
	"regexp"
	"strings"
)

// maxProviderErrorBody bounds how much of a provider response body we keep when
// surfacing it in an error/health message, to avoid log spam and oversized health payloads.
const maxProviderErrorBody = 300

var (
	// standaloneSecretPatterns match API-key-shaped tokens that must never appear in
	// logs, error messages, or the public /health diagnostic output.
	standaloneSecretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`AIza[0-9A-Za-z_\-]{20,}`), // Google / Gemini API keys
		regexp.MustCompile(`xai-[0-9A-Za-z._\-]+`),    // xAI / Grok API keys
		regexp.MustCompile(`sk-[0-9A-Za-z._\-]{8,}`),  // OpenAI / Anthropic API keys
		regexp.MustCompile(`gsk_[0-9A-Za-z._\-]{8,}`), // misc provider keys
	}
	// keyQueryPattern matches the Gemini-style `?key=SECRET` query parameter so a leaked
	// URL (e.g. from a transport-level *url.Error) does not expose the key.
	keyQueryPattern = regexp.MustCompile(`(?i)([?&](?:key|api_key|access_token)=)[^&\s"'\\]+`)
	// bearerPattern matches `Authorization: Bearer SECRET` style tokens.
	bearerPattern = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._\-]+`)
)

// redactSecrets strips API key material from a message so it is safe to log or surface.
// It removes the literal apiKey first (when known), then any key-shaped tokens and the
// Gemini `?key=` query parameter and bearer tokens. Pass an empty apiKey when unknown.
func redactSecrets(msg, apiKey string) string {
	if msg == "" {
		return ""
	}
	if apiKey != "" {
		msg = strings.ReplaceAll(msg, apiKey, "[redacted]")
	}
	for _, re := range standaloneSecretPatterns {
		msg = re.ReplaceAllString(msg, "[redacted]")
	}
	msg = keyQueryPattern.ReplaceAllString(msg, "${1}[redacted]")
	msg = bearerPattern.ReplaceAllString(msg, "${1}[redacted]")
	return msg
}

// truncateForLog trims and shortens a string to at most n bytes, appending an ellipsis marker.
func truncateForLog(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}

// detailSuffix formats a provider HTTP status and an already-sanitized body snippet for
// appending to an error message. The status always appears so diagnostics are actionable
// even when the body is empty; the snippet is included only when present.
func detailSuffix(status int, sanitizedDetail string) string {
	if sanitizedDetail == "" {
		return fmt.Sprintf(" (status=%d)", status)
	}
	return fmt.Sprintf(" (status=%d: %s)", status, sanitizedDetail)
}

// sanitizeProviderBody returns a redacted, truncated, single-line view of a provider response
// body suitable for embedding in an error message or health diagnostic.
func sanitizeProviderBody(body []byte, apiKey string) string {
	if len(body) == 0 {
		return ""
	}
	cleaned := strings.Join(strings.Fields(string(body)), " ")
	return truncateForLog(redactSecrets(cleaned, apiKey), maxProviderErrorBody)
}

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
