// Package metrics provides Prometheus metrics for APEX.BUILD monitoring
// Exports HTTP, AI, code execution, WebSocket, and business metrics
package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	once     sync.Once
	instance *Metrics
)

// Metrics holds all Prometheus metric collectors for APEX.BUILD
type Metrics struct {
	// HTTP Metrics
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.Gauge
	HTTPResponseSize     *prometheus.HistogramVec

	// User/Business Metrics
	ActiveUsersGauge    prometheus.Gauge
	ActiveProjectsGauge prometheus.Gauge
	TotalUsersGauge     prometheus.Gauge
	TotalProjectsGauge  prometheus.Gauge

	// Code Execution Metrics
	CodeExecutionsTotal    *prometheus.CounterVec
	CodeExecutionDuration  *prometheus.HistogramVec
	ExecutionsInFlight     prometheus.Gauge
	ExecutionQueueLength   prometheus.Gauge
	ContainerCPUUsage      *prometheus.GaugeVec
	ContainerMemoryUsage   *prometheus.GaugeVec

	// AI Metrics
	AIRequestsTotal       *prometheus.CounterVec
	AIRequestDuration     *prometheus.HistogramVec
	AITokensUsed          *prometheus.CounterVec
	AICostTotal           *prometheus.CounterVec
	AIRequestsInFlight    *prometheus.GaugeVec
	AIProviderHealth      *prometheus.GaugeVec
	AIResponseQuality     *prometheus.HistogramVec
	AIFallbacksTotal      *prometheus.CounterVec

	// WebSocket Metrics
	WebSocketConnectionsGauge *prometheus.GaugeVec
	WebSocketMessagesTotal    *prometheus.CounterVec
	WebSocketMessageSize      *prometheus.HistogramVec
	WebSocketLatency          *prometheus.HistogramVec

	// Database Metrics
	DBConnectionsActive prometheus.Gauge
	DBConnectionsIdle   prometheus.Gauge
	DBQueryDuration     *prometheus.HistogramVec
	DBErrorsTotal       *prometheus.CounterVec

	// Cache Metrics
	CacheHitsTotal   *prometheus.CounterVec
	CacheMissesTotal *prometheus.CounterVec
	CacheSize        *prometheus.GaugeVec

	// Billing/Subscription Metrics
	SubscriptionsTotal *prometheus.GaugeVec
	RevenueTotal       *prometheus.CounterVec
	SignupsTotal       prometheus.Counter
	ChurnTotal         prometheus.Counter

	// System Metrics
	BuildInfo    *prometheus.GaugeVec
	StartupTime  prometheus.Gauge
	GoroutineNum prometheus.Gauge
}

// Get returns the singleton Metrics instance
func Get() *Metrics {
	once.Do(func() {
		instance = newMetrics()
	})
	return instance
}

// newMetrics creates and registers all Prometheus metrics
func newMetrics() *Metrics {
	m := &Metrics{}

	// HTTP Metrics
	m.HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests by endpoint, method, and status code",
		},
		[]string{"endpoint", "method", "status"},
	)

	m.HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "apex",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"endpoint", "method"},
	)

	m.HTTPRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "Current number of HTTP requests being processed",
		},
	)

	m.HTTPResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "apex",
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"endpoint"},
	)

	// User/Business Metrics
	m.ActiveUsersGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "business",
			Name:      "active_users",
			Help:      "Number of currently active users (last 5 minutes)",
		},
	)

	m.ActiveProjectsGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "business",
			Name:      "active_projects",
			Help:      "Number of projects with activity in last hour",
		},
	)

	m.TotalUsersGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "business",
			Name:      "total_users",
			Help:      "Total number of registered users",
		},
	)

	m.TotalProjectsGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "business",
			Name:      "total_projects",
			Help:      "Total number of projects",
		},
	)

	// Code Execution Metrics
	m.CodeExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "execution",
			Name:      "total",
			Help:      "Total number of code executions by language and status",
		},
		[]string{"language", "status"},
	)

	m.CodeExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "apex",
			Subsystem: "execution",
			Name:      "duration_seconds",
			Help:      "Code execution duration in seconds",
			Buckets:   []float64{.1, .25, .5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"language"},
	)

	m.ExecutionsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "execution",
			Name:      "in_flight",
			Help:      "Number of code executions currently running",
		},
	)

	m.ExecutionQueueLength = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "execution",
			Name:      "queue_length",
			Help:      "Number of code executions waiting in queue",
		},
	)

	m.ContainerCPUUsage = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "container",
			Name:      "cpu_usage_percent",
			Help:      "Container CPU usage percentage",
		},
		[]string{"container_id", "language"},
	)

	m.ContainerMemoryUsage = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "container",
			Name:      "memory_usage_bytes",
			Help:      "Container memory usage in bytes",
		},
		[]string{"container_id", "language"},
	)

	// AI Metrics
	m.AIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "ai",
			Name:      "requests_total",
			Help:      "Total number of AI requests by provider, model, and status",
		},
		[]string{"provider", "model", "capability", "status"},
	)

	m.AIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "apex",
			Subsystem: "ai",
			Name:      "request_duration_seconds",
			Help:      "AI request duration in seconds",
			Buckets:   []float64{.5, 1, 2, 3, 5, 10, 20, 30, 60},
		},
		[]string{"provider", "model"},
	)

	m.AITokensUsed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "ai",
			Name:      "tokens_total",
			Help:      "Total number of AI tokens used by provider and type",
		},
		[]string{"provider", "model", "token_type"},
	)

	m.AICostTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "ai",
			Name:      "cost_dollars",
			Help:      "Total AI cost in dollars by provider",
		},
		[]string{"provider", "model"},
	)

	m.AIRequestsInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "ai",
			Name:      "requests_in_flight",
			Help:      "Current number of AI requests being processed by provider",
		},
		[]string{"provider"},
	)

	m.AIProviderHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "ai",
			Name:      "provider_health",
			Help:      "AI provider health status (1=healthy, 0=unhealthy)",
		},
		[]string{"provider"},
	)

	m.AIResponseQuality = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "apex",
			Subsystem: "ai",
			Name:      "response_quality",
			Help:      "AI response quality score (0-1)",
			Buckets:   []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		},
		[]string{"provider", "model"},
	)

	m.AIFallbacksTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "ai",
			Name:      "fallbacks_total",
			Help:      "Total number of AI provider fallbacks",
		},
		[]string{"from_provider", "to_provider", "reason"},
	)

	// WebSocket Metrics
	m.WebSocketConnectionsGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "websocket",
			Name:      "connections",
			Help:      "Current number of WebSocket connections by type",
		},
		[]string{"type"},
	)

	m.WebSocketMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "websocket",
			Name:      "messages_total",
			Help:      "Total number of WebSocket messages by type and direction",
		},
		[]string{"type", "direction"},
	)

	m.WebSocketMessageSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "apex",
			Subsystem: "websocket",
			Name:      "message_size_bytes",
			Help:      "WebSocket message size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 2, 10),
		},
		[]string{"type"},
	)

	m.WebSocketLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "apex",
			Subsystem: "websocket",
			Name:      "latency_seconds",
			Help:      "WebSocket message latency in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"type"},
	)

	// Database Metrics
	m.DBConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "database",
			Name:      "connections_active",
			Help:      "Number of active database connections",
		},
	)

	m.DBConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "database",
			Name:      "connections_idle",
			Help:      "Number of idle database connections",
		},
	)

	m.DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "apex",
			Subsystem: "database",
			Name:      "query_duration_seconds",
			Help:      "Database query duration in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		},
		[]string{"operation", "table"},
	)

	m.DBErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "database",
			Name:      "errors_total",
			Help:      "Total number of database errors",
		},
		[]string{"operation", "error_type"},
	)

	// Cache Metrics
	m.CacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "cache",
			Name:      "hits_total",
			Help:      "Total number of cache hits",
		},
		[]string{"cache_name"},
	)

	m.CacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "cache",
			Name:      "misses_total",
			Help:      "Total number of cache misses",
		},
		[]string{"cache_name"},
	)

	m.CacheSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "cache",
			Name:      "size_bytes",
			Help:      "Current cache size in bytes",
		},
		[]string{"cache_name"},
	)

	// Billing/Subscription Metrics
	m.SubscriptionsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "billing",
			Name:      "subscriptions_total",
			Help:      "Total number of subscriptions by plan type",
		},
		[]string{"plan"},
	)

	m.RevenueTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "billing",
			Name:      "revenue_dollars",
			Help:      "Total revenue in dollars by plan type",
		},
		[]string{"plan", "type"},
	)

	m.SignupsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "billing",
			Name:      "signups_total",
			Help:      "Total number of new user signups",
		},
	)

	m.ChurnTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "apex",
			Subsystem: "billing",
			Name:      "churn_total",
			Help:      "Total number of subscription cancellations",
		},
	)

	// System Metrics
	m.BuildInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "build",
			Name:      "info",
			Help:      "Build information",
		},
		[]string{"version", "commit", "build_date"},
	)

	m.StartupTime = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "server",
			Name:      "startup_timestamp",
			Help:      "Server startup timestamp",
		},
	)

	m.GoroutineNum = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "apex",
			Subsystem: "server",
			Name:      "goroutines",
			Help:      "Current number of goroutines",
		},
	)

	// Set startup time
	m.StartupTime.Set(float64(time.Now().Unix()))

	return m
}

// RecordHTTPRequest records an HTTP request metric
func (m *Metrics) RecordHTTPRequest(endpoint, method string, statusCode int, duration time.Duration, responseSize int) {
	status := statusCodeToLabel(statusCode)
	m.HTTPRequestsTotal.WithLabelValues(endpoint, method, status).Inc()
	m.HTTPRequestDuration.WithLabelValues(endpoint, method).Observe(duration.Seconds())
	m.HTTPResponseSize.WithLabelValues(endpoint).Observe(float64(responseSize))
}

// RecordCodeExecution records a code execution metric
func (m *Metrics) RecordCodeExecution(language, status string, duration time.Duration) {
	m.CodeExecutionsTotal.WithLabelValues(language, status).Inc()
	m.CodeExecutionDuration.WithLabelValues(language).Observe(duration.Seconds())
}

// RecordAIRequest records an AI request metric
func (m *Metrics) RecordAIRequest(provider, model, capability, status string, duration time.Duration, inputTokens, outputTokens int, cost float64) {
	m.AIRequestsTotal.WithLabelValues(provider, model, capability, status).Inc()
	m.AIRequestDuration.WithLabelValues(provider, model).Observe(duration.Seconds())
	m.AITokensUsed.WithLabelValues(provider, model, "input").Add(float64(inputTokens))
	m.AITokensUsed.WithLabelValues(provider, model, "output").Add(float64(outputTokens))
	m.AICostTotal.WithLabelValues(provider, model).Add(cost)
}

// RecordWebSocketConnection records a WebSocket connection change
func (m *Metrics) RecordWebSocketConnection(connType string, delta int) {
	m.WebSocketConnectionsGauge.WithLabelValues(connType).Add(float64(delta))
}

// RecordWebSocketMessage records a WebSocket message
func (m *Metrics) RecordWebSocketMessage(msgType, direction string, size int) {
	m.WebSocketMessagesTotal.WithLabelValues(msgType, direction).Inc()
	m.WebSocketMessageSize.WithLabelValues(msgType).Observe(float64(size))
}

// RecordCacheOperation records a cache hit or miss
func (m *Metrics) RecordCacheOperation(cacheName string, hit bool) {
	if hit {
		m.CacheHitsTotal.WithLabelValues(cacheName).Inc()
	} else {
		m.CacheMissesTotal.WithLabelValues(cacheName).Inc()
	}
}

// RecordDBQuery records a database query
func (m *Metrics) RecordDBQuery(operation, table string, duration time.Duration, err error) {
	m.DBQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
	if err != nil {
		m.DBErrorsTotal.WithLabelValues(operation, "query_error").Inc()
	}
}

// SetBuildInfo sets build information
func (m *Metrics) SetBuildInfo(version, commit, buildDate string) {
	m.BuildInfo.WithLabelValues(version, commit, buildDate).Set(1)
}

// UpdateActiveUsers updates the active users gauge
func (m *Metrics) UpdateActiveUsers(count int) {
	m.ActiveUsersGauge.Set(float64(count))
}

// UpdateActiveProjects updates the active projects gauge
func (m *Metrics) UpdateActiveProjects(count int) {
	m.ActiveProjectsGauge.Set(float64(count))
}

// UpdateTotalUsers updates the total users gauge
func (m *Metrics) UpdateTotalUsers(count int) {
	m.TotalUsersGauge.Set(float64(count))
}

// UpdateTotalProjects updates the total projects gauge
func (m *Metrics) UpdateTotalProjects(count int) {
	m.TotalProjectsGauge.Set(float64(count))
}

// UpdateSubscriptions updates subscription counts by plan
func (m *Metrics) UpdateSubscriptions(plan string, count int) {
	m.SubscriptionsTotal.WithLabelValues(plan).Set(float64(count))
}

// RecordSignup records a new user signup
func (m *Metrics) RecordSignup() {
	m.SignupsTotal.Inc()
}

// RecordChurn records a subscription cancellation
func (m *Metrics) RecordChurn() {
	m.ChurnTotal.Inc()
}

// RecordRevenue records revenue
func (m *Metrics) RecordRevenue(plan, revenueType string, amount float64) {
	m.RevenueTotal.WithLabelValues(plan, revenueType).Add(amount)
}

// SetAIProviderHealth sets the health status of an AI provider
func (m *Metrics) SetAIProviderHealth(provider string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	m.AIProviderHealth.WithLabelValues(provider).Set(value)
}

// RecordAIFallback records an AI provider fallback
func (m *Metrics) RecordAIFallback(fromProvider, toProvider, reason string) {
	m.AIFallbacksTotal.WithLabelValues(fromProvider, toProvider, reason).Inc()
}

// Helper function to convert status code to label
func statusCodeToLabel(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}
