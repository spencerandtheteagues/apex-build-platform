package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateAndValidatePreviewToken(t *testing.T) {
	service := NewAuthService("preview-test-secret")

	token, err := service.GeneratePreviewToken(42, 99, 5*time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := service.ValidatePreviewToken(token, 99)
	require.NoError(t, err)
	require.Equal(t, uint(42), claims.UserID)
	require.Equal(t, uint(99), claims.ProjectID)
}

func TestValidatePreviewTokenRejectsWrongProject(t *testing.T) {
	service := NewAuthService("preview-test-secret")

	token, err := service.GeneratePreviewToken(42, 99, 5*time.Minute)
	require.NoError(t, err)

	_, err = service.ValidatePreviewToken(token, 100)
	require.ErrorIs(t, err, ErrInvalidToken)
}
