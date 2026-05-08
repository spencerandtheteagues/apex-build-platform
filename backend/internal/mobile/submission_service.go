package mobile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type MobileSubmissionStatus string

const (
	MobileSubmissionQueued                         MobileSubmissionStatus = "queued"
	MobileSubmissionValidatingCredentials          MobileSubmissionStatus = "validating_credentials"
	MobileSubmissionUploading                      MobileSubmissionStatus = "uploading"
	MobileSubmissionSubmittedToStorePipeline       MobileSubmissionStatus = "submitted_to_store_pipeline"
	MobileSubmissionProcessing                     MobileSubmissionStatus = "processing"
	MobileSubmissionFailed                         MobileSubmissionStatus = "failed"
	MobileSubmissionCompletedUpload                MobileSubmissionStatus = "completed_upload"
	MobileSubmissionRequiresManualReviewSubmission MobileSubmissionStatus = "requires_manual_review_submission"
	MobileSubmissionReadyForTestFlight             MobileSubmissionStatus = "ready_for_testflight"
	MobileSubmissionReadyForGoogleInternalTesting  MobileSubmissionStatus = "ready_for_google_internal_testing"
)

type MobileSubmissionRequest struct {
	ProjectID       uint           `json:"project_id"`
	UserID          uint           `json:"user_id"`
	BuildID         string         `json:"build_id"`
	Platform        MobilePlatform `json:"platform"`
	ArtifactURL     string         `json:"artifact_url,omitempty"`
	ProviderBuildID string         `json:"provider_build_id,omitempty"`
	Track           string         `json:"track,omitempty"`
	DryRun          bool           `json:"dry_run,omitempty"`
}

type MobileSubmissionJob struct {
	ID                   string                 `json:"id"`
	ProjectID            uint                   `json:"project_id"`
	UserID               uint                   `json:"user_id"`
	BuildID              string                 `json:"build_id"`
	Platform             MobilePlatform         `json:"platform"`
	Status               MobileSubmissionStatus `json:"status"`
	Provider             string                 `json:"provider,omitempty"`
	ProviderSubmissionID string                 `json:"provider_submission_id,omitempty"`
	Track                string                 `json:"track,omitempty"`
	ArtifactURL          string                 `json:"artifact_url,omitempty"`
	FailureType          MobileBuildFailureType `json:"failure_type,omitempty"`
	FailureMessage       string                 `json:"failure_message,omitempty"`
	Logs                 []MobileBuildLogLine   `json:"logs,omitempty"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
}

type MobileSubmissionProviderResult struct {
	ProviderSubmissionID string                 `json:"provider_submission_id,omitempty"`
	Status               MobileSubmissionStatus `json:"status,omitempty"`
	Logs                 []MobileBuildLogLine   `json:"logs,omitempty"`
	FailureType          MobileBuildFailureType `json:"failure_type,omitempty"`
	FailureMessage       string                 `json:"failure_message,omitempty"`
}

type MobileSubmissionProvider interface {
	Name() string
	SubmitBuild(ctx context.Context, req MobileSubmissionRequest) (MobileSubmissionProviderResult, error)
}

type MobileSubmissionStore interface {
	Save(ctx context.Context, job MobileSubmissionJob) error
	Update(ctx context.Context, job MobileSubmissionJob) error
	Get(ctx context.Context, id string) (MobileSubmissionJob, bool, error)
	ListByProject(ctx context.Context, projectID uint) ([]MobileSubmissionJob, error)
}

type MobileSubmissionService struct {
	flags    FeatureFlags
	provider MobileSubmissionProvider
	store    MobileSubmissionStore
	now      func() time.Time
	newID    func() string
}

type MobileSubmissionServiceOption func(*MobileSubmissionService)

func NewMobileSubmissionService(flags FeatureFlags, provider MobileSubmissionProvider, store MobileSubmissionStore, opts ...MobileSubmissionServiceOption) *MobileSubmissionService {
	if store == nil {
		store = NewInMemoryMobileSubmissionStore()
	}
	service := &MobileSubmissionService{
		flags:    flags,
		provider: provider,
		store:    store,
		now:      time.Now,
		newID:    newMobileSubmissionID,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func WithMobileSubmissionIDGenerator(newID func() string) MobileSubmissionServiceOption {
	return func(service *MobileSubmissionService) {
		if newID != nil {
			service.newID = newID
		}
	}
}

func (s *MobileSubmissionService) CreateSubmission(ctx context.Context, req MobileSubmissionRequest) (MobileSubmissionJob, error) {
	if s == nil {
		return MobileSubmissionJob{}, fmt.Errorf("%w: service is nil", ErrMobileBuildInvalidRequest)
	}
	req = NormalizeMobileSubmissionRequest(req)
	if err := ValidateMobileSubmissionRequest(s.flags, req); err != nil {
		return MobileSubmissionJob{}, err
	}
	if s.provider == nil {
		return MobileSubmissionJob{}, ErrMobileBuildProviderMissing
	}
	if s.store == nil {
		s.store = NewInMemoryMobileSubmissionStore()
	}

	now := s.now()
	providerName := strings.TrimSpace(s.provider.Name())
	if providerName == "" {
		providerName = "unknown"
	}
	job := MobileSubmissionJob{
		ID:          s.newID(),
		ProjectID:   req.ProjectID,
		UserID:      req.UserID,
		BuildID:     req.BuildID,
		Platform:    req.Platform,
		Status:      MobileSubmissionQueued,
		Provider:    providerName,
		Track:       req.Track,
		ArtifactURL: req.ArtifactURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.store.Save(ctx, job); err != nil {
		return MobileSubmissionJob{}, err
	}

	result, err := s.provider.SubmitBuild(ctx, req)
	if err != nil {
		job.Status = MobileSubmissionFailed
		job.FailureType = MobileBuildFailureStoreSubmissionFailed
		job.FailureMessage = RedactMobileBuildSecrets(err.Error())
		job.Logs = append(job.Logs, MobileBuildLogLine{Timestamp: s.now(), Level: "error", Message: job.FailureMessage})
		job.UpdatedAt = s.now()
		if updateErr := s.store.Update(ctx, job); updateErr != nil {
			return job, updateErr
		}
		return job, fmt.Errorf("%w: %s", ErrMobileBuildProviderFailed, job.FailureMessage)
	}

	job.ProviderSubmissionID = strings.TrimSpace(result.ProviderSubmissionID)
	job.Status = normalizeMobileSubmissionStatus(result.Status, MobileSubmissionSubmittedToStorePipeline)
	job.Logs = redactMobileBuildLogLines(result.Logs, s.now)
	job.FailureType = result.FailureType
	job.FailureMessage = RedactMobileBuildSecrets(result.FailureMessage)
	if job.Status == MobileSubmissionFailed && job.FailureType == "" {
		job.FailureType = MobileBuildFailureStoreSubmissionFailed
	}
	job.UpdatedAt = s.now()
	if err := s.store.Update(ctx, job); err != nil {
		return job, err
	}
	return job, nil
}

func (s *MobileSubmissionService) GetSubmission(ctx context.Context, id string) (MobileSubmissionJob, bool, error) {
	if s == nil || s.store == nil {
		return MobileSubmissionJob{}, false, ErrMobileBuildJobNotFound
	}
	return s.store.Get(ctx, id)
}

func (s *MobileSubmissionService) ListProjectSubmissions(ctx context.Context, projectID uint) ([]MobileSubmissionJob, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	return s.store.ListByProject(ctx, projectID)
}

func NormalizeMobileSubmissionRequest(req MobileSubmissionRequest) MobileSubmissionRequest {
	req.BuildID = strings.TrimSpace(req.BuildID)
	req.Platform = MobilePlatform(strings.ToLower(strings.TrimSpace(string(req.Platform))))
	req.ArtifactURL = strings.TrimSpace(req.ArtifactURL)
	req.ProviderBuildID = strings.TrimSpace(req.ProviderBuildID)
	req.Track = strings.ToLower(strings.TrimSpace(req.Track))
	if req.Track == "" {
		if req.Platform == MobilePlatformAndroid {
			req.Track = "internal"
		} else {
			req.Track = "testflight"
		}
	}
	return req
}

func ValidateMobileSubmissionRequest(flags FeatureFlags, req MobileSubmissionRequest) error {
	if !flags.MobileBuilderEnabled || !flags.MobileExpoEnabled || !flags.MobileEASSubmitEnabled {
		return ErrMobileBuildsDisabled
	}
	if req.ProjectID == 0 || req.UserID == 0 {
		return fmt.Errorf("%w: project_id and user_id are required", ErrMobileBuildInvalidRequest)
	}
	if strings.TrimSpace(req.BuildID) == "" {
		return fmt.Errorf("%w: build_id is required", ErrMobileBuildInvalidRequest)
	}
	switch req.Platform {
	case MobilePlatformAndroid:
		if !flags.MobileAndroidBuildsEnabled {
			return ErrMobileBuildPlatformDisabled
		}
	case MobilePlatformIOS:
		if !flags.MobileIOSBuildsEnabled {
			return ErrMobileBuildPlatformDisabled
		}
	default:
		return fmt.Errorf("%w: platform must be android or ios", ErrMobileBuildInvalidRequest)
	}
	if strings.TrimSpace(req.ProviderBuildID) == "" && strings.TrimSpace(req.ArtifactURL) == "" {
		return fmt.Errorf("%w: provider_build_id or artifact_url is required for store submission", ErrMobileBuildInvalidRequest)
	}
	return nil
}

func normalizeMobileSubmissionStatus(status MobileSubmissionStatus, fallback MobileSubmissionStatus) MobileSubmissionStatus {
	switch status {
	case MobileSubmissionQueued,
		MobileSubmissionValidatingCredentials,
		MobileSubmissionUploading,
		MobileSubmissionSubmittedToStorePipeline,
		MobileSubmissionProcessing,
		MobileSubmissionFailed,
		MobileSubmissionCompletedUpload,
		MobileSubmissionRequiresManualReviewSubmission,
		MobileSubmissionReadyForTestFlight,
		MobileSubmissionReadyForGoogleInternalTesting:
		return status
	default:
		if fallback != "" {
			return fallback
		}
		return MobileSubmissionSubmittedToStorePipeline
	}
}

func newMobileSubmissionID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("msub_%d", time.Now().UnixNano())
	}
	return "msub_" + hex.EncodeToString(buf[:])
}

type InMemoryMobileSubmissionStore struct {
	mu      sync.RWMutex
	jobs    map[string]MobileSubmissionJob
	project map[uint][]string
}

func NewInMemoryMobileSubmissionStore() *InMemoryMobileSubmissionStore {
	return &InMemoryMobileSubmissionStore{jobs: map[string]MobileSubmissionJob{}, project: map[uint][]string{}}
}

func (s *InMemoryMobileSubmissionStore) Save(ctx context.Context, job MobileSubmissionJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(job.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrMobileBuildInvalidRequest)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[job.ID]; exists {
		return fmt.Errorf("%w: %s", ErrMobileBuildJobExists, job.ID)
	}
	s.jobs[job.ID] = job
	s.project[job.ProjectID] = append(s.project[job.ProjectID], job.ID)
	return nil
}

func (s *InMemoryMobileSubmissionStore) Update(ctx context.Context, job MobileSubmissionJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[job.ID]; !exists {
		return fmt.Errorf("%w: %s", ErrMobileBuildJobNotFound, job.ID)
	}
	s.jobs[job.ID] = job
	return nil
}

func (s *InMemoryMobileSubmissionStore) Get(ctx context.Context, id string) (MobileSubmissionJob, bool, error) {
	if err := ctx.Err(); err != nil {
		return MobileSubmissionJob{}, false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[strings.TrimSpace(id)]
	return job, ok, nil
}

func (s *InMemoryMobileSubmissionStore) ListByProject(ctx context.Context, projectID uint) ([]MobileSubmissionJob, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.project[projectID]
	jobs := make([]MobileSubmissionJob, 0, len(ids))
	for _, id := range ids {
		if job, ok := s.jobs[id]; ok {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

type MobileSubmissionRecord struct {
	ID                   string               `json:"id" gorm:"primaryKey;size:64"`
	ProjectID            uint                 `json:"project_id" gorm:"not null;index"`
	UserID               uint                 `json:"user_id" gorm:"not null;index"`
	BuildID              string               `json:"build_id" gorm:"not null;size:64;index"`
	Platform             string               `json:"platform" gorm:"not null;size:32;index"`
	Status               string               `json:"status" gorm:"not null;size:64;index"`
	Provider             string               `json:"provider,omitempty" gorm:"size:64"`
	ProviderSubmissionID string               `json:"provider_submission_id,omitempty" gorm:"size:128;index"`
	Track                string               `json:"track,omitempty" gorm:"size:64"`
	ArtifactURL          string               `json:"artifact_url,omitempty" gorm:"size:1024"`
	FailureType          string               `json:"failure_type,omitempty" gorm:"size:64"`
	FailureMessage       string               `json:"failure_message,omitempty" gorm:"type:text"`
	Logs                 []MobileBuildLogLine `json:"logs,omitempty" gorm:"serializer:json"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
}

func (MobileSubmissionRecord) TableName() string {
	return "mobile_submission_jobs"
}

type GormMobileSubmissionStore struct {
	db *gorm.DB
}

func NewGormMobileSubmissionStore(db *gorm.DB) *GormMobileSubmissionStore {
	return &GormMobileSubmissionStore{db: db}
}

func (s *GormMobileSubmissionStore) Save(ctx context.Context, job MobileSubmissionJob) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("%w: store is unavailable", ErrMobileBuildInvalidRequest)
	}
	if strings.TrimSpace(job.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrMobileBuildInvalidRequest)
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&MobileSubmissionRecord{}).Where("id = ?", job.ID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: %s", ErrMobileBuildJobExists, job.ID)
	}
	return s.db.WithContext(ctx).Create(mobileSubmissionJobToRecord(job)).Error
}

func (s *GormMobileSubmissionStore) Update(ctx context.Context, job MobileSubmissionJob) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("%w: store is unavailable", ErrMobileBuildInvalidRequest)
	}
	var existing MobileSubmissionRecord
	if err := s.db.WithContext(ctx).First(&existing, "id = ?", job.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %s", ErrMobileBuildJobNotFound, job.ID)
		}
		return err
	}
	return s.db.WithContext(ctx).Save(mobileSubmissionJobToRecord(job)).Error
}

func (s *GormMobileSubmissionStore) Get(ctx context.Context, id string) (MobileSubmissionJob, bool, error) {
	if s == nil || s.db == nil {
		return MobileSubmissionJob{}, false, fmt.Errorf("%w: store is unavailable", ErrMobileBuildInvalidRequest)
	}
	var record MobileSubmissionRecord
	if err := s.db.WithContext(ctx).First(&record, "id = ?", strings.TrimSpace(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return MobileSubmissionJob{}, false, nil
		}
		return MobileSubmissionJob{}, false, err
	}
	return mobileSubmissionRecordToJob(record), true, nil
}

func (s *GormMobileSubmissionStore) ListByProject(ctx context.Context, projectID uint) ([]MobileSubmissionJob, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("%w: store is unavailable", ErrMobileBuildInvalidRequest)
	}
	var records []MobileSubmissionRecord
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	jobs := make([]MobileSubmissionJob, 0, len(records))
	for _, record := range records {
		jobs = append(jobs, mobileSubmissionRecordToJob(record))
	}
	return jobs, nil
}

func mobileSubmissionJobToRecord(job MobileSubmissionJob) *MobileSubmissionRecord {
	return &MobileSubmissionRecord{
		ID:                   strings.TrimSpace(job.ID),
		ProjectID:            job.ProjectID,
		UserID:               job.UserID,
		BuildID:              strings.TrimSpace(job.BuildID),
		Platform:             string(job.Platform),
		Status:               string(job.Status),
		Provider:             strings.TrimSpace(job.Provider),
		ProviderSubmissionID: strings.TrimSpace(job.ProviderSubmissionID),
		Track:                strings.TrimSpace(job.Track),
		ArtifactURL:          strings.TrimSpace(job.ArtifactURL),
		FailureType:          string(job.FailureType),
		FailureMessage:       RedactMobileBuildSecrets(job.FailureMessage),
		Logs:                 redactMobileBuildLogLines(job.Logs, nil),
		CreatedAt:            job.CreatedAt,
		UpdatedAt:            job.UpdatedAt,
	}
}

func mobileSubmissionRecordToJob(record MobileSubmissionRecord) MobileSubmissionJob {
	return MobileSubmissionJob{
		ID:                   record.ID,
		ProjectID:            record.ProjectID,
		UserID:               record.UserID,
		BuildID:              record.BuildID,
		Platform:             MobilePlatform(record.Platform),
		Status:               MobileSubmissionStatus(record.Status),
		Provider:             record.Provider,
		ProviderSubmissionID: record.ProviderSubmissionID,
		Track:                record.Track,
		ArtifactURL:          record.ArtifactURL,
		FailureType:          MobileBuildFailureType(record.FailureType),
		FailureMessage:       record.FailureMessage,
		Logs:                 redactMobileBuildLogLines(record.Logs, nil),
		CreatedAt:            record.CreatedAt,
		UpdatedAt:            record.UpdatedAt,
	}
}
