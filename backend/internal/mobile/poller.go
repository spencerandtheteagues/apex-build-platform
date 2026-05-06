package mobile

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"apex-build/pkg/models"

	"gorm.io/gorm"
)

const (
	defaultMobileBuildPollInterval  = time.Minute
	defaultMobileBuildPollMinAge    = 30 * time.Second
	defaultMobileBuildPollBatchSize = 10
)

type MobileBuildPollerConfig struct {
	Interval  time.Duration
	MinAge    time.Duration
	BatchSize int
	LogPrefix string
}

type MobileBuildPollResult struct {
	Refreshed int
	Errors    int
}

type MobileBuildPoller struct {
	db      *gorm.DB
	service *MobileBuildService
	config  MobileBuildPollerConfig
}

func NewMobileBuildPoller(db *gorm.DB, service *MobileBuildService, config MobileBuildPollerConfig) *MobileBuildPoller {
	return &MobileBuildPoller{
		db:      db,
		service: service,
		config:  normalizeMobileBuildPollerConfig(config),
	}
}

func (p *MobileBuildPoller) Start(ctx context.Context) {
	if p == nil || p.service == nil {
		return
	}
	config := normalizeMobileBuildPollerConfig(p.config)
	go func() {
		log.Printf("%s started interval=%s min_age=%s batch_size=%d", config.LogPrefix, config.Interval, config.MinAge, config.BatchSize)
		ticker := time.NewTicker(config.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Printf("%s stopped", config.LogPrefix)
				return
			default:
			}

			result, err := p.RunOnce(ctx)
			if err != nil {
				log.Printf("%s refreshed=%d errors=%d err=%s", config.LogPrefix, result.Refreshed, result.Errors, RedactMobileBuildSecrets(err.Error()))
			}

			select {
			case <-ctx.Done():
				log.Printf("%s stopped", config.LogPrefix)
				return
			case <-ticker.C:
			}
		}
	}()
}

func (p *MobileBuildPoller) RunOnce(ctx context.Context) (MobileBuildPollResult, error) {
	if p == nil || p.service == nil {
		return MobileBuildPollResult{}, fmt.Errorf("%w: poller service is unavailable", ErrMobileBuildInvalidRequest)
	}
	if p.db == nil {
		return MobileBuildPollResult{}, fmt.Errorf("%w: poller database is unavailable", ErrMobileBuildInvalidRequest)
	}
	config := normalizeMobileBuildPollerConfig(p.config)
	refreshed, refreshErrs := p.service.RefreshPollableBuilds(ctx, config.BatchSize, config.MinAge)

	errs := make([]error, 0, len(refreshErrs)+len(refreshed))
	for _, err := range refreshErrs {
		if err != nil {
			errs = append(errs, err)
		}
	}

	for _, job := range refreshed {
		if strings.TrimSpace(job.ID) == "" || job.ProjectID == 0 {
			continue
		}
		var project models.Project
		if err := p.db.WithContext(ctx).First(&project, job.ProjectID).Error; err != nil {
			errs = append(errs, err)
			continue
		}
		ApplyMobileBuildJobToProject(&project, job)
		if err := p.db.WithContext(ctx).
			Select("MobileBuildStatus", "MobileMetadata").
			Save(&project).
			Error; err != nil {
			errs = append(errs, err)
		}
	}

	result := MobileBuildPollResult{
		Refreshed: len(refreshed),
		Errors:    len(errs),
	}
	return result, errors.Join(errs...)
}

func normalizeMobileBuildPollerConfig(config MobileBuildPollerConfig) MobileBuildPollerConfig {
	if config.Interval <= 0 {
		config.Interval = defaultMobileBuildPollInterval
	}
	if config.MinAge < 0 {
		config.MinAge = 0
	}
	if config.MinAge == 0 {
		config.MinAge = defaultMobileBuildPollMinAge
	}
	if config.BatchSize <= 0 {
		config.BatchSize = defaultMobileBuildPollBatchSize
	}
	if strings.TrimSpace(config.LogPrefix) == "" {
		config.LogPrefix = "mobile_eas_build_poller"
	}
	return config
}
