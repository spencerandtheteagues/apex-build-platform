package handlers

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"apex-build/internal/auth"

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
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			token = strings.TrimSpace(authHeader[7:])
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

	var allowedOrigins []string
	if envOrigins := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")); envOrigins != "" || strings.TrimSpace(os.Getenv("CORS_ORIGINS")) != "" {
		if envOrigins == "" {
			envOrigins = strings.TrimSpace(os.Getenv("CORS_ORIGINS"))
		}
		for _, allowed := range strings.Split(envOrigins, ",") {
			if trimmed := strings.TrimSpace(allowed); trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	} else {
		allowedOrigins = []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"http://localhost:8080",
			"http://127.0.0.1:3000",
			"https://apex.build",
			"https://www.apex.build",
			"https://apex-frontend-gigq.onrender.com",
		}
	}

	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}
	}

	return false
}

func allowedWebSocketOrigin(r *http.Request) bool {
	return AllowedWebSocketOrigin(r)
}
