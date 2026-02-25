package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/gin-gonic/gin"
)

// generateNonce creates a cryptographically random nonce for CSP
func generateNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// CSPWithNonce returns middleware that generates a per-request nonce
// and sets a Content-Security-Policy header that replaces 'unsafe-inline'
// with the nonce-based directive.
//
// The nonce is stored in the gin context as "csp_nonce" so templates/handlers
// can inject it into <script nonce="..."> and <style nonce="..."> tags.
func CSPWithNonce() gin.HandlerFunc {
	return func(c *gin.Context) {
		nonce := generateNonce()
		if nonce == "" {
			c.Next()
			return
		}

		c.Set("csp_nonce", nonce)

		csp := fmt.Sprintf(
			"default-src 'self'; "+
				"script-src 'self' 'nonce-%s'; "+
				"style-src 'self' 'nonce-%s' https://fonts.googleapis.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data: blob:; "+
				"connect-src 'self' ws: wss:; "+
				"frame-src 'self'",
			nonce, nonce,
		)

		c.Header("Content-Security-Policy", csp)
		c.Next()
	}
}
