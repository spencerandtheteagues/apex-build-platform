package alwayson

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// DeploymentAPI is the subset of hosting behavior required by the always-on controller.
type DeploymentAPI interface {
	SetAlwaysOn(deploymentID string, enabled bool, keepAliveInterval int) error
	GetAlwaysOnStatus(deploymentID string) (map[string]interface{}, error)
}

// InventoryProvider returns deployment IDs that should be reconciled by the controller.
type InventoryProvider func(ctx context.Context) ([]string, error)

// Config controls controller behavior.
type Config struct {
	ReconcileInterval   time.Duration
	DefaultKeepAliveSec int
	MaxConcurrent       int
	LogPrefix           string
}

// DefaultConfig returns production-safe defaults.
func DefaultConfig() Config {
	return Config{
		ReconcileInterval:   45 * time.Second,
		DefaultKeepAliveSec: 60,
		MaxConcurrent:       8,
		LogPrefix:           "always-on-controller",
	}
}

// Service is a lightweight reconciliation loop for always-on deployments.
type Service struct {
	api       DeploymentAPI
	cfg       Config
	inventory InventoryProvider

	totalReconciles int64
	totalEnsures    int64
	totalErrors     int64
	lastRunUnix     int64
	lastSuccessUnix int64
	activeWorkers   int64

	mu sync.RWMutex
}

// NewService creates an always-on controller service.
func NewService(api DeploymentAPI, cfg *Config) *Service {
	config := DefaultConfig()
	if cfg != nil {
		if cfg.ReconcileInterval > 0 {
			config.ReconcileInterval = cfg.ReconcileInterval
		}
		if cfg.DefaultKeepAliveSec > 0 {
			config.DefaultKeepAliveSec = cfg.DefaultKeepAliveSec
		}
		if cfg.MaxConcurrent > 0 {
			config.MaxConcurrent = cfg.MaxConcurrent
		}
		if cfg.LogPrefix != "" {
			config.LogPrefix = cfg.LogPrefix
		}
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 1
	}
	return &Service{
		api: api,
		cfg: config,
	}
}

// SetInventoryProvider configures dynamic deployment discovery for periodic reconcile.
func (s *Service) SetInventoryProvider(provider InventoryProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inventory = provider
}

// Ensure enables or disables always-on state for a deployment.
func (s *Service) Ensure(ctx context.Context, deploymentID string, enabled bool, keepAliveSec int) error {
	if s == nil || s.api == nil {
		return nil
	}
	if keepAliveSec <= 0 {
		keepAliveSec = s.cfg.DefaultKeepAliveSec
	}

	atomic.AddInt64(&s.totalEnsures, 1)
	if err := s.api.SetAlwaysOn(deploymentID, enabled, keepAliveSec); err != nil {
		atomic.AddInt64(&s.totalErrors, 1)
		return err
	}
	if _, err := s.api.GetAlwaysOnStatus(deploymentID); err != nil {
		atomic.AddInt64(&s.totalErrors, 1)
		return err
	}
	_ = ctx
	return nil
}

// Reconcile ensures all provided deployments remain always-on.
func (s *Service) Reconcile(ctx context.Context, deploymentIDs []string) error {
	if s == nil || s.api == nil {
		return nil
	}
	atomic.StoreInt64(&s.lastRunUnix, time.Now().Unix())
	atomic.AddInt64(&s.totalReconciles, 1)

	sem := make(chan struct{}, s.cfg.MaxConcurrent)
	errCh := make(chan error, len(deploymentIDs))
	var wg sync.WaitGroup

	for _, id := range deploymentIDs {
		deploymentID := id
		if deploymentID == "" {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			atomic.AddInt64(&s.activeWorkers, 1)
			defer func() {
				atomic.AddInt64(&s.activeWorkers, -1)
				<-sem
			}()

			if err := s.Ensure(ctx, deploymentID, true, 0); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	var firstErr error
	for err := range errCh {
		if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr == nil {
		atomic.StoreInt64(&s.lastSuccessUnix, time.Now().Unix())
	}
	return firstErr
}

// Start launches the periodic reconcile loop.
func (s *Service) Start(ctx context.Context) {
	if s == nil || s.api == nil {
		return
	}
	ticker := time.NewTicker(s.cfg.ReconcileInterval)
	defer ticker.Stop()

	log.Printf("%s: started (interval=%s, workers=%d)", s.cfg.LogPrefix, s.cfg.ReconcileInterval, s.cfg.MaxConcurrent)
	for {
		select {
		case <-ctx.Done():
			log.Printf("%s: stopped", s.cfg.LogPrefix)
			return
		case <-ticker.C:
			s.runInventoryReconcile(ctx)
		}
	}
}

func (s *Service) runInventoryReconcile(ctx context.Context) {
	s.mu.RLock()
	inventory := s.inventory
	s.mu.RUnlock()
	if inventory == nil {
		return
	}
	ids, err := inventory(ctx)
	if err != nil {
		atomic.AddInt64(&s.totalErrors, 1)
		log.Printf("%s: inventory error: %v", s.cfg.LogPrefix, err)
		return
	}
	if len(ids) == 0 {
		return
	}
	if err := s.Reconcile(ctx, ids); err != nil {
		log.Printf("%s: reconcile error: %v", s.cfg.LogPrefix, err)
	}
}

// Stats returns controller metrics for admin/ops endpoints.
func (s *Service) Stats() map[string]interface{} {
	if s == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"reconcile_interval": s.cfg.ReconcileInterval.String(),
		"default_keepalive":  s.cfg.DefaultKeepAliveSec,
		"max_concurrent":     s.cfg.MaxConcurrent,
		"total_reconciles":   atomic.LoadInt64(&s.totalReconciles),
		"total_ensures":      atomic.LoadInt64(&s.totalEnsures),
		"total_errors":       atomic.LoadInt64(&s.totalErrors),
		"active_workers":     atomic.LoadInt64(&s.activeWorkers),
		"last_run_unix":      atomic.LoadInt64(&s.lastRunUnix),
		"last_success_unix":  atomic.LoadInt64(&s.lastSuccessUnix),
	}
}
