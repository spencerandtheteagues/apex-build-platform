// Package hosting - Hosting Proxy/Router for APEX.BUILD
// Routes *.apex.app requests to the correct project deployment
package hosting

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// HostingProxy routes incoming requests from *.apex.app to the correct deployment
type HostingProxy struct {
	db            *gorm.DB
	hostingDomain string // "apex.app" or custom domain
	proxyCache    sync.Map // subdomain -> *httputil.ReverseProxy
	routeCache    sync.Map // subdomain -> *NativeDeployment
	cacheTTL      time.Duration
}

// ProxyConfig holds proxy configuration
type ProxyConfig struct {
	HostingDomain string        // e.g., "apex.app"
	CacheTTL      time.Duration // Cache TTL for route lookups
}

// NewHostingProxy creates a new hosting proxy
func NewHostingProxy(db *gorm.DB, config *ProxyConfig) *HostingProxy {
	if config == nil {
		config = &ProxyConfig{
			HostingDomain: "apex.app",
			CacheTTL:      30 * time.Second,
		}
	}

	proxy := &HostingProxy{
		db:            db,
		hostingDomain: config.HostingDomain,
		cacheTTL:      config.CacheTTL,
	}

	// Start cache cleanup goroutine
	go proxy.cleanupCache()

	return proxy
}

// ServeHTTP handles incoming requests and routes them to the correct deployment
func (p *HostingProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract subdomain from host
	subdomain := p.extractSubdomain(r.Host)
	if subdomain == "" {
		p.serveNotFound(w, r, "Invalid hostname")
		return
	}

	// Look up deployment
	deployment, err := p.getDeployment(subdomain)
	if err != nil {
		p.serveNotFound(w, r, "Project not found")
		return
	}

	// Check if deployment is running
	if deployment.Status != StatusRunning {
		p.serveMaintenancePage(w, r, deployment)
		return
	}

	// Proxy the request
	p.proxyRequest(w, r, deployment)
}

// extractSubdomain extracts the subdomain from the host header
func (p *HostingProxy) extractSubdomain(host string) string {
	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Check if it's a *.apex.app domain
	suffix := "." + p.hostingDomain
	if strings.HasSuffix(host, suffix) {
		subdomain := strings.TrimSuffix(host, suffix)
		// Handle preview subdomains (preview-xxx.apex.app)
		subdomain = strings.TrimPrefix(subdomain, "preview-")
		return subdomain
	}

	// Check for custom domains (looked up in database)
	return p.lookupCustomDomain(host)
}

// lookupCustomDomain checks if a custom domain is registered
func (p *HostingProxy) lookupCustomDomain(domain string) string {
	// Check cache first
	if cached, ok := p.routeCache.Load("custom:" + domain); ok {
		entry := cached.(*cacheEntry)
		if time.Now().Before(entry.expiry) {
			return entry.subdomain
		}
	}

	// Look up in database
	var deployment NativeDeployment
	if err := p.db.Where("custom_domain = ? AND status = ?", domain, StatusRunning).First(&deployment).Error; err != nil {
		return ""
	}

	// Cache the result
	p.routeCache.Store("custom:"+domain, &cacheEntry{
		subdomain: deployment.Subdomain,
		expiry:    time.Now().Add(p.cacheTTL),
	})

	return deployment.Subdomain
}

type cacheEntry struct {
	subdomain  string
	deployment *NativeDeployment
	expiry     time.Time
}

// getDeployment retrieves deployment info, using cache when possible
func (p *HostingProxy) getDeployment(subdomain string) (*NativeDeployment, error) {
	// Check cache
	if cached, ok := p.routeCache.Load(subdomain); ok {
		entry := cached.(*cacheEntry)
		if time.Now().Before(entry.expiry) && entry.deployment != nil {
			return entry.deployment, nil
		}
	}

	// Look up in database
	var deployment NativeDeployment
	if err := p.db.Where("subdomain = ? AND deleted_at IS NULL", subdomain).First(&deployment).Error; err != nil {
		return nil, fmt.Errorf("deployment not found: %w", err)
	}

	// Cache the result
	p.routeCache.Store(subdomain, &cacheEntry{
		deployment: &deployment,
		expiry:     time.Now().Add(p.cacheTTL),
	})

	return &deployment, nil
}

// proxyRequest forwards the request to the deployment's container
func (p *HostingProxy) proxyRequest(w http.ResponseWriter, r *http.Request, deployment *NativeDeployment) {
	// Get or create reverse proxy for this deployment
	proxy := p.getOrCreateProxy(deployment)
	if proxy == nil {
		p.serveError(w, r, "Failed to create proxy")
		return
	}

	// Update request metrics (async to not block the request)
	go p.updateRequestMetrics(deployment.ID)

	// Update last request timestamp
	go p.updateLastRequest(deployment.ID)

	// Proxy the request
	proxy.ServeHTTP(w, r)
}

// getOrCreateProxy returns a reverse proxy for the deployment
func (p *HostingProxy) getOrCreateProxy(deployment *NativeDeployment) *httputil.ReverseProxy {
	// Check cache
	if cached, ok := p.proxyCache.Load(deployment.ID); ok {
		return cached.(*httputil.ReverseProxy)
	}

	// Create target URL
	// In production, this would be the container's internal address
	targetURL := p.getTargetURL(deployment)
	if targetURL == nil {
		return nil
	}

	// Create reverse proxy with custom settings
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.Host = targetURL.Host

			// Add forwarding headers
			if clientIP, _, err := strings.Cut(req.RemoteAddr, ":"); err {
				req.Header.Set("X-Forwarded-For", clientIP)
			}
			req.Header.Set("X-Forwarded-Host", req.Host)
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Real-IP", strings.Split(req.RemoteAddr, ":")[0])

			// Add deployment identifier
			req.Header.Set("X-Apex-Deployment-ID", deployment.ID)
			req.Header.Set("X-Apex-Subdomain", deployment.Subdomain)
		},
		ModifyResponse: func(resp *http.Response) error {
			// Add security headers
			resp.Header.Set("X-Content-Type-Options", "nosniff")
			resp.Header.Set("X-Frame-Options", "SAMEORIGIN")
			resp.Header.Set("X-XSS-Protection", "1; mode=block")

			// Add cache headers for static assets
			if p.isStaticAsset(resp.Request.URL.Path) {
				resp.Header.Set("Cache-Control", "public, max-age=31536000, immutable")
			}

			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Proxy error for %s: %v", deployment.Subdomain, err)
			p.serveError(w, r, "Deployment temporarily unavailable")
		},
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	// Cache the proxy
	p.proxyCache.Store(deployment.ID, proxy)

	return proxy
}

// getTargetURL returns the internal URL for the deployment's container
func (p *HostingProxy) getTargetURL(deployment *NativeDeployment) *url.URL {
	// In production, this would be the container's internal network address
	// For Docker, it might be: http://apex-{deploymentID}:3000
	// For Kubernetes, it might be: http://{serviceName}.{namespace}.svc.cluster.local:3000

	var host string
	if deployment.ContainerID != "" {
		// Docker networking
		host = deployment.ContainerID
	} else {
		// Fallback to localhost for development
		host = "localhost"
	}

	port := deployment.ContainerPort
	if port == 0 {
		port = 3000
	}

	targetURL, err := url.Parse(fmt.Sprintf("http://%s:%d", host, port))
	if err != nil {
		log.Printf("Failed to parse target URL for deployment %s: %v", deployment.ID, err)
		return nil
	}

	return targetURL
}

// isStaticAsset checks if the path is a static asset
func (p *HostingProxy) isStaticAsset(path string) bool {
	staticExtensions := []string{".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".woff", ".woff2", ".ttf", ".ico"}
	for _, ext := range staticExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// updateRequestMetrics increments request count for the deployment
func (p *HostingProxy) updateRequestMetrics(deploymentID string) {
	p.db.Model(&NativeDeployment{}).
		Where("id = ?", deploymentID).
		UpdateColumn("total_requests", gorm.Expr("total_requests + ?", 1))
}

// updateLastRequest updates the last request timestamp
func (p *HostingProxy) updateLastRequest(deploymentID string) {
	now := time.Now()
	p.db.Model(&NativeDeployment{}).
		Where("id = ?", deploymentID).
		Update("last_request_at", &now)
}

// serveNotFound renders a 404 page
func (p *HostingProxy) serveNotFound(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	io.WriteString(w, p.getNotFoundPage(message))
}

// serveError renders a 500 page
func (p *HostingProxy) serveError(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	io.WriteString(w, p.getErrorPage(message))
}

// serveMaintenancePage renders a maintenance page for non-running deployments
func (p *HostingProxy) serveMaintenancePage(w http.ResponseWriter, r *http.Request, deployment *NativeDeployment) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var statusCode int
	var statusMessage string

	switch deployment.Status {
	case StatusPending, StatusProvisioning:
		statusCode = http.StatusServiceUnavailable
		statusMessage = "This deployment is being provisioned. Please check back in a few moments."
	case StatusBuilding:
		statusCode = http.StatusServiceUnavailable
		statusMessage = "This deployment is currently building. Please check back in a few moments."
	case StatusDeploying:
		statusCode = http.StatusServiceUnavailable
		statusMessage = "This deployment is starting up. Please check back in a few moments."
	case StatusStopped:
		statusCode = http.StatusServiceUnavailable
		statusMessage = "This deployment has been stopped by its owner."
	case StatusFailed:
		statusCode = http.StatusServiceUnavailable
		statusMessage = "This deployment failed to start. The project owner has been notified."
	default:
		statusCode = http.StatusServiceUnavailable
		statusMessage = "This deployment is temporarily unavailable."
	}

	w.WriteHeader(statusCode)
	io.WriteString(w, p.getMaintenancePage(deployment.Subdomain, statusMessage, string(deployment.Status)))
}

// getNotFoundPage returns the 404 HTML page
func (p *HostingProxy) getNotFoundPage(message string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>404 - Not Found | APEX.BUILD</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #0f0f1a 0%%, #1a1a2e 100%%);
            color: #fff;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container {
            text-align: center;
            padding: 40px;
        }
        h1 {
            font-size: 120px;
            color: #00d4ff;
            text-shadow: 0 0 30px rgba(0, 212, 255, 0.5);
            margin-bottom: 20px;
        }
        h2 { font-size: 24px; margin-bottom: 16px; color: #ccc; }
        p { color: #888; margin-bottom: 30px; }
        a {
            display: inline-block;
            padding: 12px 30px;
            background: linear-gradient(90deg, #00d4ff, #00ff88);
            color: #0f0f1a;
            text-decoration: none;
            border-radius: 6px;
            font-weight: 600;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        a:hover {
            transform: translateY(-2px);
            box-shadow: 0 5px 20px rgba(0, 212, 255, 0.4);
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>404</h1>
        <h2>%s</h2>
        <p>The project you're looking for doesn't exist or hasn't been deployed yet.</p>
        <a href="https://apex.build">Go to APEX.BUILD</a>
    </div>
</body>
</html>`, message)
}

// getErrorPage returns the error HTML page
func (p *HostingProxy) getErrorPage(message string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Error | APEX.BUILD</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #0f0f1a 0%%, #1a1a2e 100%%);
            color: #fff;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container { text-align: center; padding: 40px; }
        .icon {
            font-size: 80px;
            margin-bottom: 20px;
        }
        h2 { font-size: 24px; margin-bottom: 16px; color: #ff6b6b; }
        p { color: #888; margin-bottom: 30px; }
        button {
            padding: 12px 30px;
            background: linear-gradient(90deg, #00d4ff, #00ff88);
            color: #0f0f1a;
            border: none;
            border-radius: 6px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        button:hover {
            transform: translateY(-2px);
            box-shadow: 0 5px 20px rgba(0, 212, 255, 0.4);
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">&#x26A0;</div>
        <h2>%s</h2>
        <p>We're having trouble connecting to this deployment. Please try again in a moment.</p>
        <button onclick="location.reload()">Retry</button>
    </div>
</body>
</html>`, message)
}

// getMaintenancePage returns the maintenance HTML page
func (p *HostingProxy) getMaintenancePage(subdomain, message, status string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="refresh" content="10">
    <title>%s.apex.app - Starting | APEX.BUILD</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #0f0f1a 0%%, #1a1a2e 100%%);
            color: #fff;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container { text-align: center; padding: 40px; max-width: 500px; }
        .loader {
            width: 60px;
            height: 60px;
            border: 3px solid #333;
            border-top-color: #00d4ff;
            border-radius: 50%%;
            animation: spin 1s linear infinite;
            margin: 0 auto 30px;
        }
        @keyframes spin { to { transform: rotate(360deg); } }
        h2 { font-size: 24px; margin-bottom: 16px; color: #00d4ff; }
        p { color: #888; margin-bottom: 20px; line-height: 1.6; }
        .status {
            display: inline-block;
            padding: 6px 16px;
            background: rgba(0, 212, 255, 0.1);
            border: 1px solid rgba(0, 212, 255, 0.3);
            border-radius: 20px;
            color: #00d4ff;
            font-size: 14px;
            text-transform: capitalize;
        }
        .refresh-text { font-size: 12px; color: #555; margin-top: 30px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="loader"></div>
        <h2>%s.apex.app</h2>
        <p>%s</p>
        <span class="status">%s</span>
        <p class="refresh-text">This page will automatically refresh...</p>
    </div>
</body>
</html>`, subdomain, subdomain, message, status)
}

// cleanupCache periodically cleans up expired cache entries
func (p *HostingProxy) cleanupCache() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		// Clean route cache
		p.routeCache.Range(func(key, value interface{}) bool {
			if entry, ok := value.(*cacheEntry); ok {
				if now.After(entry.expiry) {
					p.routeCache.Delete(key)
				}
			}
			return true
		})
	}
}

// InvalidateCache removes a deployment from the cache
func (p *HostingProxy) InvalidateCache(subdomain string) {
	p.routeCache.Delete(subdomain)
	// Also invalidate the proxy
	if cached, ok := p.routeCache.Load(subdomain); ok {
		if entry, ok := cached.(*cacheEntry); ok && entry.deployment != nil {
			p.proxyCache.Delete(entry.deployment.ID)
		}
	}
}

// InvalidateDeploymentCache removes a deployment from the cache by ID
func (p *HostingProxy) InvalidateDeploymentCache(deploymentID string) {
	p.proxyCache.Delete(deploymentID)
}

// HealthCheckHandler returns a handler for the hosting proxy health check
func (p *HostingProxy) HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"status":"ok","service":"apex-hosting-proxy"}`)
	}
}

// StartProxyServer starts the hosting proxy HTTP server
func StartProxyServer(ctx context.Context, db *gorm.DB, addr string) error {
	proxy := NewHostingProxy(db, nil)

	mux := http.NewServeMux()
	mux.Handle("/", proxy)
	mux.HandleFunc("/health", proxy.HealthCheckHandler())

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	log.Printf("Hosting proxy starting on %s", addr)
	return server.ListenAndServe()
}
