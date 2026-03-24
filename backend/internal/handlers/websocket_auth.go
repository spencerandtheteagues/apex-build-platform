package handlers

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"apex-build/internal/auth"
	"apex-build/internal/origins"

	"github.com/gin-gonic/gin"
)

func WebSocketUserID(c *gin.Context) (uint, error) {
	if userID, exists := c.Get("user_id"); exists {
		if typedUserID, ok := userID.(uint); ok && typedUserID > 0 {
			return typedUserID, nil
		}
	}

	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		var err error
		token, err = auth.WebSocketAccessTokenFromRequest(c)
		if err != nil {
			return 0, errors.New("authentication required")
		}
	}
	if token == "" {
		return 0, errors.New("authentication required")
	}

	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		return 0, errors.New("JWT_SECRET not configured")
	}

	claims, err := auth.NewAuthService(secret).ValidateToken(token)
	if err != nil {
		return 0, errors.New("invalid or expired token")
	}

	return claims.UserID, nil
}

func websocketUserID(c *gin.Context) (uint, error) {
	return WebSocketUserID(c)
}

func AllowedWebSocketOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return os.Getenv("ENVIRONMENT") != "production"
	}
	return origins.IsAllowedOrigin(origin)
}

func allowedWebSocketOrigin(r *http.Request) bool {
	return AllowedWebSocketOrigin(r)
}
