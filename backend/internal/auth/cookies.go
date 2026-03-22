package auth

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	AccessTokenCookieName  = "apex_access_token"
	RefreshTokenCookieName = "apex_refresh_token"
)

// CookieConfig holds httpOnly cookie settings
type CookieConfig struct {
	Name     string
	Domain   string
	Path     string
	MaxAge   time.Duration
	Secure   bool
	HTTPOnly bool
	SameSite http.SameSite
}

// DefaultCookieConfig returns production-safe cookie defaults
func DefaultCookieConfig() *CookieConfig {
	return defaultCookieConfig("apex_session", 24*time.Hour)
}

func AccessTokenCookieConfig() *CookieConfig {
	return defaultCookieConfig(AccessTokenCookieName, 15*time.Minute)
}

func RefreshTokenCookieConfig() *CookieConfig {
	return defaultCookieConfig(RefreshTokenCookieName, 7*24*time.Hour)
}

func defaultCookieConfig(name string, maxAge time.Duration) *CookieConfig {
	secure := cookieSecureDefault()
	return &CookieConfig{
		Name:     name,
		Domain:   os.Getenv("COOKIE_DOMAIN"),
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   secure,
		HTTPOnly: true,
		SameSite: cookieSameSiteDefault(secure),
	}
}

func cookieSecureDefault() bool {
	if raw, ok := os.LookupEnv("COOKIE_SECURE"); ok {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return os.Getenv("ENVIRONMENT") == "production"
}

func cookieSameSiteDefault(secure bool) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("COOKIE_SAME_SITE"))) {
	case "strict":
		return http.SameSiteStrictMode
	case "lax":
		return http.SameSiteLaxMode
	case "none":
		if secure {
			return http.SameSiteNoneMode
		}
	}

	if os.Getenv("ENVIRONMENT") == "production" && secure {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

// SetTokenCookie writes the JWT token as an httpOnly cookie
func SetTokenCookie(c *gin.Context, token string, cfg *CookieConfig) {
	if cfg == nil {
		cfg = DefaultCookieConfig()
	}
	c.SetSameSite(cfg.SameSite)
	c.SetCookie(
		cfg.Name,
		token,
		int(cfg.MaxAge.Seconds()),
		cfg.Path,
		cfg.Domain,
		cfg.Secure,
		cfg.HTTPOnly,
	)
}

// ClearTokenCookie removes the session cookie
func ClearTokenCookie(c *gin.Context, cfg *CookieConfig) {
	if cfg == nil {
		cfg = DefaultCookieConfig()
	}
	c.SetSameSite(cfg.SameSite)
	c.SetCookie(cfg.Name, "", -1, cfg.Path, cfg.Domain, cfg.Secure, cfg.HTTPOnly)
}

// GetTokenFromCookie reads the JWT from the httpOnly cookie
func GetTokenFromCookie(c *gin.Context) (string, error) {
	cfg := DefaultCookieConfig()
	return c.Cookie(cfg.Name)
}

func SetAccessTokenCookie(c *gin.Context, token string) {
	SetTokenCookie(c, token, AccessTokenCookieConfig())
}

func SetRefreshTokenCookie(c *gin.Context, token string) {
	SetTokenCookie(c, token, RefreshTokenCookieConfig())
}

func ClearAccessTokenCookie(c *gin.Context) {
	ClearTokenCookie(c, AccessTokenCookieConfig())
}

func ClearRefreshTokenCookie(c *gin.Context) {
	ClearTokenCookie(c, RefreshTokenCookieConfig())
}

func ClearAuthCookies(c *gin.Context) {
	ClearAccessTokenCookie(c)
	ClearRefreshTokenCookie(c)
}

func GetAccessTokenFromCookie(c *gin.Context) (string, error) {
	return c.Cookie(AccessTokenCookieName)
}

func GetRefreshTokenFromCookie(c *gin.Context) (string, error) {
	return c.Cookie(RefreshTokenCookieName)
}

func AccessTokenFromRequest(c *gin.Context) (string, error) {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		token := strings.TrimSpace(authHeader[7:])
		if token != "" {
			return token, nil
		}
	}

	token, err := GetAccessTokenFromCookie(c)
	if err == nil && strings.TrimSpace(token) != "" {
		return strings.TrimSpace(token), nil
	}
	return "", errors.New("authentication required")
}

func WebSocketAccessTokenFromRequest(c *gin.Context) (string, error) {
	if token := strings.TrimSpace(c.Query("token")); token != "" {
		return token, nil
	}
	return AccessTokenFromRequest(c)
}
