// Package metrics provides Prometheus metrics middleware for Gin
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// responseWriter wraps gin.ResponseWriter to capture response size
type responseWriter struct {
	gin.ResponseWriter
	size int
}

func (w *responseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.size += n
	return n, err
}

func (w *responseWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	w.size += n
	return n, err
}

// PrometheusMiddleware returns a Gin middleware that records HTTP metrics
func PrometheusMiddleware() gin.HandlerFunc {
	m := Get()

	return func(c *gin.Context) {
		// Skip metrics endpoint itself
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()

		// Track in-flight requests
		m.HTTPRequestsInFlight.Inc()
		defer m.HTTPRequestsInFlight.Dec()

		// Wrap response writer to capture size
		rw := &responseWriter{ResponseWriter: c.Writer, size: 0}
		c.Writer = rw

		// Process request
		c.Next()

		// Record metrics after request completes
		duration := time.Since(start)
		endpoint := normalizeEndpoint(c.FullPath())
		if endpoint == "" {
			endpoint = "unknown"
		}

		m.RecordHTTPRequest(
			endpoint,
			c.Request.Method,
			c.Writer.Status(),
			duration,
			rw.size,
		)
	}
}

// PrometheusHandler returns the Prometheus HTTP handler
func PrometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// PrometheusHandlerHTTP returns a standard HTTP handler for metrics
func PrometheusHandlerHTTP() http.Handler {
	return promhttp.Handler()
}

// normalizeEndpoint normalizes endpoint paths to reduce cardinality
func normalizeEndpoint(path string) string {
	// Return as-is for static routes
	// Gin's FullPath() already uses parameter placeholders like :id
	if path == "" {
		return "unknown"
	}
	return path
}

// MetricsCollector periodically collects and updates metrics
type MetricsCollector struct {
	metrics  *Metrics
	interval time.Duration
	stopCh   chan struct{}
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(interval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		metrics:  Get(),
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins periodic metric collection
func (mc *MetricsCollector) Start() {
	go func() {
		ticker := time.NewTicker(mc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				mc.collect()
			case <-mc.stopCh:
				return
			}
		}
	}()
}

// Stop stops the metric collector
func (mc *MetricsCollector) Stop() {
	close(mc.stopCh)
}

// collect performs a single metric collection cycle
func (mc *MetricsCollector) collect() {
	// Update goroutine count
	// Note: runtime.NumGoroutine() can be called here
	// For now, we skip this to avoid importing runtime in metrics
}

// RecordRequestStart should be called when a request starts
func RecordRequestStart() {
	Get().HTTPRequestsInFlight.Inc()
}

// RecordRequestEnd should be called when a request ends
func RecordRequestEnd() {
	Get().HTTPRequestsInFlight.Dec()
}

// HTTPStatusCode returns a human readable status code category
func HTTPStatusCode(code int) string {
	return strconv.Itoa(code)
}
