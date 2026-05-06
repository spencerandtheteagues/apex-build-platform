package mobile

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type MobileBuildRecord struct {
	ID              string               `json:"id" gorm:"primaryKey;size:64"`
	ProjectID       uint                 `json:"project_id" gorm:"not null;index"`
	UserID          uint                 `json:"user_id" gorm:"not null;index"`
	Platform        string               `json:"platform" gorm:"not null;size:32;index"`
	Profile         string               `json:"profile" gorm:"not null;size:32"`
	ReleaseLevel    string               `json:"release_level" gorm:"not null;size:64"`
	Status          string               `json:"status" gorm:"not null;size:64;index"`
	Provider        string               `json:"provider,omitempty" gorm:"size:64"`
	ProviderBuildID string               `json:"provider_build_id,omitempty" gorm:"size:128;index"`
	ArtifactURL     string               `json:"artifact_url,omitempty" gorm:"size:1024"`
	AppVersion      string               `json:"app_version,omitempty" gorm:"size:64"`
	BuildNumber     string               `json:"build_number,omitempty" gorm:"size:64"`
	VersionCode     int                  `json:"version_code,omitempty"`
	CommitRef       string               `json:"commit_ref,omitempty" gorm:"size:255"`
	FailureType     string               `json:"failure_type,omitempty" gorm:"size:64"`
	FailureMessage  string               `json:"failure_message,omitempty" gorm:"type:text"`
	Logs            []MobileBuildLogLine `json:"logs,omitempty" gorm:"serializer:json"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}

func (MobileBuildRecord) TableName() string {
	return "mobile_build_jobs"
}

type GormMobileBuildStore struct {
	db *gorm.DB
}

func NewGormMobileBuildStore(db *gorm.DB) *GormMobileBuildStore {
	return &GormMobileBuildStore{db: db}
}

func (s *GormMobileBuildStore) Save(ctx context.Context, job MobileBuildJob) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("%w: store is unavailable", ErrMobileBuildInvalidRequest)
	}
	if strings.TrimSpace(job.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrMobileBuildInvalidRequest)
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&MobileBuildRecord{}).Where("id = ?", job.ID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: %s", ErrMobileBuildJobExists, job.ID)
	}
	return s.db.WithContext(ctx).Create(mobileBuildJobToRecord(job)).Error
}

func (s *GormMobileBuildStore) Update(ctx context.Context, job MobileBuildJob) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("%w: store is unavailable", ErrMobileBuildInvalidRequest)
	}
	if strings.TrimSpace(job.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrMobileBuildInvalidRequest)
	}
	var existing MobileBuildRecord
	if err := s.db.WithContext(ctx).First(&existing, "id = ?", job.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %s", ErrMobileBuildJobNotFound, job.ID)
		}
		return err
	}
	record := mobileBuildJobToRecord(job)
	return s.db.WithContext(ctx).Save(record).Error
}

func (s *GormMobileBuildStore) Get(ctx context.Context, id string) (MobileBuildJob, bool, error) {
	if s == nil || s.db == nil {
		return MobileBuildJob{}, false, fmt.Errorf("%w: store is unavailable", ErrMobileBuildInvalidRequest)
	}
	var record MobileBuildRecord
	if err := s.db.WithContext(ctx).First(&record, "id = ?", strings.TrimSpace(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return MobileBuildJob{}, false, nil
		}
		return MobileBuildJob{}, false, err
	}
	return mobileBuildRecordToJob(record), true, nil
}

func (s *GormMobileBuildStore) ListByProject(ctx context.Context, projectID uint) ([]MobileBuildJob, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("%w: store is unavailable", ErrMobileBuildInvalidRequest)
	}
	var records []MobileBuildRecord
	if err := s.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, err
	}
	jobs := make([]MobileBuildJob, 0, len(records))
	for _, record := range records {
		jobs = append(jobs, mobileBuildRecordToJob(record))
	}
	return jobs, nil
}

func (s *GormMobileBuildStore) ListPollable(ctx context.Context, statuses []MobileBuildStatus, updatedBefore time.Time, limit int) ([]MobileBuildJob, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("%w: store is unavailable", ErrMobileBuildInvalidRequest)
	}
	statusValues := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if strings.TrimSpace(string(status)) != "" {
			statusValues = append(statusValues, string(status))
		}
	}
	if len(statusValues) == 0 {
		return nil, nil
	}

	query := s.db.WithContext(ctx).
		Where("status IN ?", statusValues).
		Where("provider_build_id <> ?", "")
	if !updatedBefore.IsZero() {
		query = query.Where("updated_at <= ?", updatedBefore)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	var records []MobileBuildRecord
	if err := query.Order("updated_at ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	jobs := make([]MobileBuildJob, 0, len(records))
	for _, record := range records {
		jobs = append(jobs, mobileBuildRecordToJob(record))
	}
	return jobs, nil
}

func mobileBuildJobToRecord(job MobileBuildJob) *MobileBuildRecord {
	return &MobileBuildRecord{
		ID:              strings.TrimSpace(job.ID),
		ProjectID:       job.ProjectID,
		UserID:          job.UserID,
		Platform:        string(job.Platform),
		Profile:         string(job.Profile),
		ReleaseLevel:    string(job.ReleaseLevel),
		Status:          string(job.Status),
		Provider:        strings.TrimSpace(job.Provider),
		ProviderBuildID: strings.TrimSpace(job.ProviderBuildID),
		ArtifactURL:     strings.TrimSpace(job.ArtifactURL),
		AppVersion:      strings.TrimSpace(job.AppVersion),
		BuildNumber:     strings.TrimSpace(job.BuildNumber),
		VersionCode:     job.VersionCode,
		CommitRef:       strings.TrimSpace(job.CommitRef),
		FailureType:     string(job.FailureType),
		FailureMessage:  RedactMobileBuildSecrets(job.FailureMessage),
		Logs:            redactMobileBuildLogLines(job.Logs, nil),
		CreatedAt:       job.CreatedAt,
		UpdatedAt:       job.UpdatedAt,
	}
}

func mobileBuildRecordToJob(record MobileBuildRecord) MobileBuildJob {
	return MobileBuildJob{
		ID:              record.ID,
		ProjectID:       record.ProjectID,
		UserID:          record.UserID,
		Platform:        MobilePlatform(record.Platform),
		Profile:         MobileBuildProfile(record.Profile),
		ReleaseLevel:    MobileReleaseLevel(record.ReleaseLevel),
		Status:          MobileBuildStatus(record.Status),
		Provider:        record.Provider,
		ProviderBuildID: record.ProviderBuildID,
		ArtifactURL:     record.ArtifactURL,
		AppVersion:      record.AppVersion,
		BuildNumber:     record.BuildNumber,
		VersionCode:     record.VersionCode,
		CommitRef:       record.CommitRef,
		FailureType:     MobileBuildFailureType(record.FailureType),
		FailureMessage:  record.FailureMessage,
		Logs:            redactMobileBuildLogLines(record.Logs, nil),
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
	}
}
