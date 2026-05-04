package agents

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

const buildPollTokenHeader = "X-Apex-Build-Poll-Token"

func newBuildPollToken() (string, string) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", ""
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	return token, hashBuildPollToken(token)
}

func hashBuildPollToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:])
}

func validBuildPollToken(token, expectedHash string) bool {
	actualHash := hashBuildPollToken(token)
	expectedHash = strings.TrimSpace(expectedHash)
	if actualHash == "" || expectedHash == "" || len(actualHash) != len(expectedHash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
}
