package api

import (
	"testing"
	"time"

	"apex-build/pkg/models"

	"github.com/stretchr/testify/require"
)

func TestShouldIssueVerificationCodeSkipsActiveCode(t *testing.T) {
	expiresAt := time.Now().Add(10 * time.Minute)
	user := &models.User{
		VerificationCode:          "stored-hash",
		VerificationCodeExpiresAt: &expiresAt,
	}

	require.False(t, shouldIssueVerificationCode(user, time.Now()))
}

func TestShouldIssueVerificationCodeIssuesWhenMissingOrExpired(t *testing.T) {
	now := time.Now()
	expiredAt := now.Add(-time.Minute)

	require.True(t, shouldIssueVerificationCode(&models.User{}, now))
	require.True(t, shouldIssueVerificationCode(&models.User{
		VerificationCode:          "stored-hash",
		VerificationCodeExpiresAt: &expiredAt,
	}, now))
}

func TestNormalizeVerificationCodeAcceptsPastedCode(t *testing.T) {
	require.Equal(t, "704978", normalizeVerificationCode("704978"))
	require.Equal(t, "704978", normalizeVerificationCode("704 978"))
	require.Equal(t, "704978", normalizeVerificationCode("704-978"))
	require.Equal(t, "704978", normalizeVerificationCode("\n704 978\t"))
}

func TestVerificationEmailLookupUsesCaseInsensitiveQuery(t *testing.T) {
	clause, email, ok := verificationEmailLookup(" Friend@Test.COM ")

	require.True(t, ok)
	require.Equal(t, "LOWER(email) = LOWER(?)", clause)
	require.Equal(t, "friend@test.com", email)
}
