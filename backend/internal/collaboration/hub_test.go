package collaboration

import (
	"errors"
	"testing"

	"apex-build/internal/auth"
	"apex-build/pkg/models"

	"github.com/stretchr/testify/require"
)

func TestValidateCollaborationWebSocketToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-collab-secret-with-sufficient-length")

	authService := auth.NewAuthService("test-collab-secret-with-sufficient-length")
	tokens, err := authService.GenerateTokens(&models.User{
		ID:               42,
		Username:         "alice",
		Email:            "alice@example.com",
		SubscriptionType: "free",
	})
	require.NoError(t, err)

	claims, err := validateCollaborationWebSocketToken(tokens.AccessToken)
	require.NoError(t, err)
	require.Equal(t, uint(42), claims.UserID)
	require.Equal(t, "alice", claims.Username)
	require.Equal(t, "alice@example.com", claims.Email)
}

func TestValidateCollaborationWebSocketTokenRejectsBlacklistedTokens(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-collab-secret-with-sufficient-length")

	authService := auth.NewAuthService("test-collab-secret-with-sufficient-length")
	tokens, err := authService.GenerateTokens(&models.User{
		ID:               42,
		Username:         "alice",
		Email:            "alice@example.com",
		SubscriptionType: "free",
	})
	require.NoError(t, err)
	require.NoError(t, authService.BlacklistToken(tokens.AccessToken))

	_, err = validateCollaborationWebSocketToken(tokens.AccessToken)
	require.True(t, errors.Is(err, auth.ErrTokenBlacklisted))
}
