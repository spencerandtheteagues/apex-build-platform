// Package metrics provides business metrics collection for APEX.BUILD
package metrics

import (
	"context"
	"log"
	"runtime"
	"time"

	"gorm.io/gorm"
)

// BusinessMetricsCollector collects business metrics from the database
type BusinessMetricsCollector struct {
	db       *gorm.DB
	metrics  *Metrics
	interval time.Duration
	stopCh   chan struct{}
}

// NewBusinessMetricsCollector creates a new business metrics collector
func NewBusinessMetricsCollector(db *gorm.DB, interval time.Duration) *BusinessMetricsCollector {
	return &BusinessMetricsCollector{
		db:       db,
		metrics:  Get(),
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins periodic business metric collection
func (bmc *BusinessMetricsCollector) Start(ctx context.Context) {
	go func() {
		// Initial collection
		bmc.collectAll()

		ticker := time.NewTicker(bmc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				bmc.collectAll()
			case <-bmc.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop stops the business metrics collector
func (bmc *BusinessMetricsCollector) Stop() {
	close(bmc.stopCh)
}

// collectAll collects all business metrics
func (bmc *BusinessMetricsCollector) collectAll() {
	bmc.collectUserMetrics()
	bmc.collectProjectMetrics()
	bmc.collectSubscriptionMetrics()
	bmc.collectSystemMetrics()
	bmc.collectDatabaseMetrics()
}

// collectUserMetrics collects user-related metrics
func (bmc *BusinessMetricsCollector) collectUserMetrics() {
	if bmc.db == nil {
		return
	}

	// Total users
	var totalUsers int64
	if err := bmc.db.Table("users").Count(&totalUsers).Error; err != nil {
		log.Printf("Failed to count total users: %v", err)
	} else {
		bmc.metrics.UpdateTotalUsers(int(totalUsers))
	}

	// Active users (last 5 minutes)
	var activeUsers int64
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	if err := bmc.db.Table("users").Where("updated_at > ?", fiveMinutesAgo).Count(&activeUsers).Error; err != nil {
		log.Printf("Failed to count active users: %v", err)
	} else {
		bmc.metrics.UpdateActiveUsers(int(activeUsers))
	}
}

// collectProjectMetrics collects project-related metrics
func (bmc *BusinessMetricsCollector) collectProjectMetrics() {
	if bmc.db == nil {
		return
	}

	// Total projects
	var totalProjects int64
	if err := bmc.db.Table("projects").Count(&totalProjects).Error; err != nil {
		log.Printf("Failed to count total projects: %v", err)
	} else {
		bmc.metrics.UpdateTotalProjects(int(totalProjects))
	}

	// Active projects (last hour)
	var activeProjects int64
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	if err := bmc.db.Table("projects").Where("updated_at > ?", oneHourAgo).Count(&activeProjects).Error; err != nil {
		log.Printf("Failed to count active projects: %v", err)
	} else {
		bmc.metrics.UpdateActiveProjects(int(activeProjects))
	}
}

// collectSubscriptionMetrics collects subscription-related metrics
func (bmc *BusinessMetricsCollector) collectSubscriptionMetrics() {
	if bmc.db == nil {
		return
	}

	// Count subscriptions by plan type
	type PlanCount struct {
		Plan  string
		Count int64
	}

	var planCounts []PlanCount
	if err := bmc.db.Table("users").
		Select("subscription_type as plan, count(*) as count").
		Group("subscription_type").
		Scan(&planCounts).Error; err != nil {
		log.Printf("Failed to count subscriptions by plan: %v", err)
		return
	}

	for _, pc := range planCounts {
		plan := pc.Plan
		if plan == "" {
			plan = "free"
		}
		bmc.metrics.UpdateSubscriptions(plan, int(pc.Count))
	}
}

// collectSystemMetrics collects system-level metrics
func (bmc *BusinessMetricsCollector) collectSystemMetrics() {
	// Goroutine count
	bmc.metrics.GoroutineNum.Set(float64(runtime.NumGoroutine()))
}

// collectDatabaseMetrics collects database connection metrics
func (bmc *BusinessMetricsCollector) collectDatabaseMetrics() {
	if bmc.db == nil {
		return
	}

	sqlDB, err := bmc.db.DB()
	if err != nil {
		log.Printf("Failed to get database stats: %v", err)
		return
	}

	stats := sqlDB.Stats()
	bmc.metrics.DBConnectionsActive.Set(float64(stats.InUse))
	bmc.metrics.DBConnectionsIdle.Set(float64(stats.Idle))
}

// AIMetricsRecorder provides methods for recording AI-specific metrics
type AIMetricsRecorder struct {
	metrics *Metrics
}

// NewAIMetricsRecorder creates a new AI metrics recorder
func NewAIMetricsRecorder() *AIMetricsRecorder {
	return &AIMetricsRecorder{
		metrics: Get(),
	}
}

// RecordRequest records an AI request with all details
func (r *AIMetricsRecorder) RecordRequest(provider, model, capability string, success bool, duration time.Duration, inputTokens, outputTokens int, cost float64) {
	status := "success"
	if !success {
		status = "error"
	}

	r.metrics.RecordAIRequest(provider, model, capability, status, duration, inputTokens, outputTokens, cost)
}

// RecordFallback records an AI fallback event
func (r *AIMetricsRecorder) RecordFallback(fromProvider, toProvider, reason string) {
	r.metrics.RecordAIFallback(fromProvider, toProvider, reason)
}

// SetProviderHealth sets the health status of an AI provider
func (r *AIMetricsRecorder) SetProviderHealth(provider string, healthy bool) {
	r.metrics.SetAIProviderHealth(provider, healthy)
}

// StartRequest records the start of an AI request (increments in-flight counter)
func (r *AIMetricsRecorder) StartRequest(provider string) {
	r.metrics.AIRequestsInFlight.WithLabelValues(provider).Inc()
}

// EndRequest records the end of an AI request (decrements in-flight counter)
func (r *AIMetricsRecorder) EndRequest(provider string) {
	r.metrics.AIRequestsInFlight.WithLabelValues(provider).Dec()
}

// RecordQuality records the quality score of an AI response
func (r *AIMetricsRecorder) RecordQuality(provider, model string, quality float64) {
	r.metrics.AIResponseQuality.WithLabelValues(provider, model).Observe(quality)
}

// ExecutionMetricsRecorder provides methods for recording code execution metrics
type ExecutionMetricsRecorder struct {
	metrics *Metrics
}

// NewExecutionMetricsRecorder creates a new execution metrics recorder
func NewExecutionMetricsRecorder() *ExecutionMetricsRecorder {
	return &ExecutionMetricsRecorder{
		metrics: Get(),
	}
}

// RecordExecution records a code execution
func (r *ExecutionMetricsRecorder) RecordExecution(language string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}

	r.metrics.RecordCodeExecution(language, status, duration)
}

// StartExecution records the start of a code execution
func (r *ExecutionMetricsRecorder) StartExecution() {
	r.metrics.ExecutionsInFlight.Inc()
}

// EndExecution records the end of a code execution
func (r *ExecutionMetricsRecorder) EndExecution() {
	r.metrics.ExecutionsInFlight.Dec()
}

// SetQueueLength sets the current execution queue length
func (r *ExecutionMetricsRecorder) SetQueueLength(length int) {
	r.metrics.ExecutionQueueLength.Set(float64(length))
}

// RecordContainerUsage records container resource usage
func (r *ExecutionMetricsRecorder) RecordContainerUsage(containerID, language string, cpuPercent float64, memoryBytes int64) {
	r.metrics.ContainerCPUUsage.WithLabelValues(containerID, language).Set(cpuPercent)
	r.metrics.ContainerMemoryUsage.WithLabelValues(containerID, language).Set(float64(memoryBytes))
}

// WebSocketMetricsRecorder provides methods for recording WebSocket metrics
type WebSocketMetricsRecorder struct {
	metrics *Metrics
}

// NewWebSocketMetricsRecorder creates a new WebSocket metrics recorder
func NewWebSocketMetricsRecorder() *WebSocketMetricsRecorder {
	return &WebSocketMetricsRecorder{
		metrics: Get(),
	}
}

// ConnectionOpened records a new WebSocket connection
func (r *WebSocketMetricsRecorder) ConnectionOpened(connType string) {
	r.metrics.RecordWebSocketConnection(connType, 1)
}

// ConnectionClosed records a closed WebSocket connection
func (r *WebSocketMetricsRecorder) ConnectionClosed(connType string) {
	r.metrics.RecordWebSocketConnection(connType, -1)
}

// MessageSent records an outgoing WebSocket message
func (r *WebSocketMetricsRecorder) MessageSent(msgType string, size int) {
	r.metrics.RecordWebSocketMessage(msgType, "out", size)
}

// MessageReceived records an incoming WebSocket message
func (r *WebSocketMetricsRecorder) MessageReceived(msgType string, size int) {
	r.metrics.RecordWebSocketMessage(msgType, "in", size)
}

// RecordLatency records WebSocket message latency
func (r *WebSocketMetricsRecorder) RecordLatency(msgType string, latency time.Duration) {
	r.metrics.WebSocketLatency.WithLabelValues(msgType).Observe(latency.Seconds())
}
