package auth

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
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
	secure := os.Getenv("ENVIRONMENT") == "production"
	return &CookieConfig{
		Name:     "apex_session",
		Domain:   os.Getenv("COOKIE_DOMAIN"),
		Path:     "/",
		MaxAge:   24 * time.Hour,
		Secure:   secure,
		HTTPOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
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
