package mobile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

type MobileBuildStatus string

const (
	MobileBuildQueued               MobileBuildStatus = "queued"
	MobileBuildPreparing            MobileBuildStatus = "preparing"
	MobileBuildValidating           MobileBuildStatus = "validating"
	MobileBuildUploading            MobileBuildStatus = "uploading"
	MobileBuildBuilding             MobileBuildStatus = "building"
	MobileBuildSigning              MobileBuildStatus = "signing"
	MobileBuildSucceeded            MobileBuildStatus = "succeeded"
	MobileBuildFailed               MobileBuildStatus = "failed"
	MobileBuildCanceled             MobileBuildStatus = "canceled"
	MobileBuildRepairPending        MobileBuildStatus = "repair_pending"
	MobileBuildRepairedRetryPending MobileBuildStatus = "repaired_retry_pending"
)

type MobileBuildProfile string

const (
	MobileBuildProfileDevelopment MobileBuildProfile = "development"
	MobileBuildProfilePreview     MobileBuildProfile = "preview"
	MobileBuildProfileInternal    MobileBuildProfile = "internal"
	MobileBuildProfileProduction  MobileBuildProfile = "production"
)

type MobileBuildFailureType string

const (
	MobileBuildFailureDependencyInstallFailed MobileBuildFailureType = "dependency_install_failed"
	MobileBuildFailureExpoConfigInvalid       MobileBuildFailureType = "expo_config_invalid"
	MobileBuildFailureUnsupportedNativeModule MobileBuildFailureType = "unsupported_native_module"
	MobileBuildFailureAndroidSigningFailed    MobileBuildFailureType = "android_signing_failed"
	MobileBuildFailureIOSCredentialsFailed    MobileBuildFailureType = "ios_credentials_failed"
	MobileBuildFailureIOSProvisioningFailed   MobileBuildFailureType = "ios_provisioning_failed"
	MobileBuildFailureMetroBundleFailed       MobileBuildFailureType = "metro_bundle_failed"
	MobileBuildFailureTypeScriptFailed        MobileBuildFailureType = "typescript_failed"
	MobileBuildFailureBackendAPIMismatch      MobileBuildFailureType = "backend_api_mismatch"
	MobileBuildFailurePermissionConfigMissing MobileBuildFailureType = "permission_config_missing"
	MobileBuildFailureAppIdentifierInvalid    MobileBuildFailureType = "app_identifier_invalid"
	MobileBuildFailureStoreSubmissionFailed   MobileBuildFailureType = "store_submission_failed"
	MobileBuildFailureUnknown                 MobileBuildFailureType = "unknown"
)

var (
	ErrMobileBuildsDisabled        = errors.New("mobile native builds are disabled")
	ErrMobileBuildPlatformDisabled = errors.New("mobile build platform is disabled")
	ErrMobileBuildInvalidRequest   = errors.New("invalid mobile build request")
	ErrMobileBuildProviderMissing  = errors.New("mobile build provider is not configured")
	ErrMobileBuildProviderFailed   = errors.New("mobile build provider failed")
	ErrMobileBuildJobNotFound      = errors.New("mobile build job not found")
	ErrMobileBuildJobExists        = errors.New("mobile build job already exists")
)

type MobileBuildRequest struct {
	ProjectID    uint               `json:"project_id"`
	UserID       uint               `json:"user_id"`
	Platform     MobilePlatform     `json:"platform"`
	Profile      MobileBuildProfile `json:"profile"`
	ReleaseLevel MobileReleaseLevel `json:"release_level"`
	AppVersion   string             `json:"app_version,omitempty"`
	BuildNumber  string             `json:"build_number,omitempty"`
	VersionCode  int                `json:"version_code,omitempty"`
	CommitRef    string             `json:"commit_ref,omitempty"`
	SourcePath   string             `json:"source_path,omitempty"`
	DryRun       bool               `json:"dry_run,omitempty"`
}

type MobileBuildJob struct {
	ID              string                 `json:"id"`
	ProjectID       uint                   `json:"project_id"`
	UserID          uint                   `json:"user_id"`
	Platform        MobilePlatform         `json:"platform"`
	Profile         MobileBuildProfile     `json:"profile"`
	ReleaseLevel    MobileReleaseLevel     `json:"release_level"`
	Status          MobileBuildStatus      `json:"status"`
	Provider        string                 `json:"provider,omitempty"`
	ProviderBuildID string                 `json:"provider_build_id,omitempty"`
	ArtifactURL     string                 `json:"artifact_url,omitempty"`
	AppVersion      string                 `json:"app_version,omitempty"`
	BuildNumber     string                 `json:"build_number,omitempty"`
	VersionCode     int                    `json:"version_code,omitempty"`
	CommitRef       string                 `json:"commit_ref,omitempty"`
	FailureType     MobileBuildFailureType `json:"failure_type,omitempty"`
	FailureMessage  string                 `json:"failure_message,omitempty"`
	Logs            []MobileBuildLogLine   `json:"logs,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

type MobileBuildLogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level,omitempty"`
	Message   string    `json:"message"`
}

type MobileBuildProviderResult struct {
	ProviderBuildID string                 `json:"provider_build_id,omitempty"`
	Status          MobileBuildStatus      `json:"status,omitempty"`
	ArtifactURL     string                 `json:"artifact_url,omitempty"`
	Logs            []MobileBuildLogLine   `json:"logs,omitempty"`
	FailureType     MobileBuildFailureType `json:"failure_type,omitempty"`
	FailureMessage  string                 `json:"failure_message,omitempty"`
}

type MobileBuildProvider interface {
	Name() string
	CreateBuild(ctx context.Context, req MobileBuildRequest) (MobileBuildProviderResult, error)
}

type MobileBuildProviderRequestValidator interface {
	ValidateBuildRequest(req MobileBuildRequest) error
}

type MobileBuildProviderRefresher interface {
	RefreshBuild(ctx context.Context, job MobileBuildJob) (MobileBuildProviderResult, error)
}

type MobileBuildProviderCanceler interface {
	CancelBuild(ctx context.Context, job MobileBuildJob) (MobileBuildProviderResult, error)
}

type MobileBuildStore interface {
	Save(ctx context.Context, job MobileBuildJob) error
	Update(ctx context.Context, job MobileBuildJob) error
	Get(ctx context.Context, id string) (MobileBuildJob, bool, error)
	ListByProject(ctx context.Context, projectID uint) ([]MobileBuildJob, error)
}

type MobileBuildService struct {
	flags    FeatureFlags
	provider MobileBuildProvider
	store    MobileBuildStore
	now      func() time.Time
	newID    func() string
}

type MobileBuildServiceOption func(*MobileBuildService)

func NewMobileBuildService(flags FeatureFlags, provider MobileBuildProvider, store MobileBuildStore, opts ...MobileBuildServiceOption) *MobileBuildService {
	if store == nil {
		store = NewInMemoryMobileBuildStore()
	}
	service := &MobileBuildService{
		flags:    flags,
		provider: provider,
		store:    store,
		now:      time.Now,
		newID:    newMobileBuildID,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func WithMobileBuildClock(now func() time.Time) MobileBuildServiceOption {
	return func(service *MobileBuildService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithMobileBuildIDGenerator(newID func() string) MobileBuildServiceOption {
	return func(service *MobileBuildService) {
		if newID != nil {
			service.newID = newID
		}
	}
}

func (s *MobileBuildService) CreateBuild(ctx context.Context, req MobileBuildRequest) (MobileBuildJob, error) {
	if s == nil {
		return MobileBuildJob{}, fmt.Errorf("%w: service is nil", ErrMobileBuildInvalidRequest)
	}
	req = NormalizeMobileBuildRequest(req)
	if err := s.ValidateRequest(req); err != nil {
		return MobileBuildJob{}, err
	}
	if s.provider == nil {
		return MobileBuildJob{}, ErrMobileBuildProviderMissing
	}
	if s.store == nil {
		s.store = NewInMemoryMobileBuildStore()
	}

	now := s.now()
	providerName := strings.TrimSpace(s.provider.Name())
	if providerName == "" {
		providerName = "unknown"
	}
	job := MobileBuildJob{
		ID:           s.newID(),
		ProjectID:    req.ProjectID,
		UserID:       req.UserID,
		Platform:     req.Platform,
		Profile:      req.Profile,
		ReleaseLevel: req.ReleaseLevel,
		Status:       MobileBuildQueued,
		Provider:     providerName,
		AppVersion:   strings.TrimSpace(req.AppVersion),
		BuildNumber:  strings.TrimSpace(req.BuildNumber),
		VersionCode:  req.VersionCode,
		CommitRef:    strings.TrimSpace(req.CommitRef),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.store.Save(ctx, job); err != nil {
		return MobileBuildJob{}, err
	}

	result, err := s.provider.CreateBuild(ctx, req)
	if err != nil {
		job.Status = MobileBuildFailed
		job.FailureType = ClassifyMobileBuildFailure(err.Error())
		job.FailureMessage = RedactMobileBuildSecrets(err.Error())
		job.Logs = append(job.Logs, MobileBuildLogLine{
			Timestamp: s.now(),
			Level:     "error",
			Message:   job.FailureMessage,
		})
		job.UpdatedAt = s.now()
		if updateErr := s.store.Update(ctx, job); updateErr != nil {
			return job, updateErr
		}
		return job, fmt.Errorf("%w: %s", ErrMobileBuildProviderFailed, job.FailureMessage)
	}

	job.ProviderBuildID = strings.TrimSpace(result.ProviderBuildID)
	job.ArtifactURL = strings.TrimSpace(result.ArtifactURL)
	job.Status = normalizeMobileBuildStatus(result.Status, MobileBuildQueued)
	job.Logs = redactMobileBuildLogLines(result.Logs, s.now)
	job.FailureType = result.FailureType
	job.FailureMessage = RedactMobileBuildSecrets(result.FailureMessage)
	if job.Status == MobileBuildFailed && job.FailureType == "" {
		job.FailureType = ClassifyMobileBuildFailure(job.FailureMessage)
	}
	job.UpdatedAt = s.now()
	if err := s.store.Update(ctx, job); err != nil {
		return job, err
	}

	return job, nil
}

func (s *MobileBuildService) ValidateRequest(req MobileBuildRequest) error {
	if s == nil {
		return fmt.Errorf("%w: service is nil", ErrMobileBuildInvalidRequest)
	}
	req = NormalizeMobileBuildRequest(req)
	if err := s.ValidatePolicyRequest(req); err != nil {
		return err
	}
	if validator, ok := s.provider.(MobileBuildProviderRequestValidator); ok {
		if err := validator.ValidateBuildRequest(req); err != nil {
			return err
		}
	}
	return nil
}

func (s *MobileBuildService) ValidatePolicyRequest(req MobileBuildRequest) error {
	if s == nil {
		return fmt.Errorf("%w: service is nil", ErrMobileBuildInvalidRequest)
	}
	return ValidateMobileBuildRequest(s.flags, NormalizeMobileBuildRequest(req))
}

func (s *MobileBuildService) GetBuild(ctx context.Context, id string) (MobileBuildJob, bool, error) {
	if s == nil || s.store == nil {
		return MobileBuildJob{}, false, ErrMobileBuildJobNotFound
	}
	return s.store.Get(ctx, id)
}

func (s *MobileBuildService) RefreshBuild(ctx context.Context, id string) (MobileBuildJob, error) {
	if s == nil {
		return MobileBuildJob{}, fmt.Errorf("%w: service is nil", ErrMobileBuildInvalidRequest)
	}
	if s.store == nil {
		return MobileBuildJob{}, ErrMobileBuildJobNotFound
	}
	job, exists, err := s.store.Get(ctx, id)
	if err != nil {
		return MobileBuildJob{}, err
	}
	if !exists {
		return MobileBuildJob{}, ErrMobileBuildJobNotFound
	}
	if err := s.ValidatePolicyRequest(MobileBuildRequest{
		ProjectID:    job.ProjectID,
		UserID:       job.UserID,
		Platform:     job.Platform,
		Profile:      job.Profile,
		ReleaseLevel: job.ReleaseLevel,
	}); err != nil {
		return job, err
	}
	provider, ok := s.provider.(MobileBuildProviderRefresher)
	if s.provider == nil || !ok {
		return job, ErrMobileBuildProviderMissing
	}
	if strings.TrimSpace(job.ProviderBuildID) == "" {
		return job, fmt.Errorf("%w: provider_build_id is required to refresh mobile build", ErrMobileBuildInvalidRequest)
	}

	result, err := provider.RefreshBuild(ctx, job)
	now := s.now()
	if err != nil {
		job.Logs = append(job.Logs, MobileBuildLogLine{
			Timestamp: now,
			Level:     "error",
			Message:   RedactMobileBuildSecrets(err.Error()),
		})
		job.UpdatedAt = now
		if updateErr := s.store.Update(ctx, job); updateErr != nil {
			return job, updateErr
		}
		return job, fmt.Errorf("%w: %s", ErrMobileBuildProviderFailed, RedactMobileBuildSecrets(err.Error()))
	}

	job = applyMobileBuildProviderResult(job, result, now)
	if err := s.store.Update(ctx, job); err != nil {
		return job, err
	}
	return job, nil
}

func (s *MobileBuildService) CancelBuild(ctx context.Context, id string) (MobileBuildJob, error) {
	if s == nil {
		return MobileBuildJob{}, fmt.Errorf("%w: service is nil", ErrMobileBuildInvalidRequest)
	}
	if s.store == nil {
		return MobileBuildJob{}, ErrMobileBuildJobNotFound
	}
	job, exists, err := s.store.Get(ctx, id)
	if err != nil {
		return MobileBuildJob{}, err
	}
	if !exists {
		return MobileBuildJob{}, ErrMobileBuildJobNotFound
	}
	if err := s.ValidatePolicyRequest(MobileBuildRequest{
		ProjectID:    job.ProjectID,
		UserID:       job.UserID,
		Platform:     job.Platform,
		Profile:      job.Profile,
		ReleaseLevel: job.ReleaseLevel,
	}); err != nil {
		return job, err
	}
	if !isCancelableMobileBuildStatus(job.Status) {
		return job, fmt.Errorf("%w: build status %q cannot be canceled", ErrMobileBuildInvalidRequest, job.Status)
	}
	provider, ok := s.provider.(MobileBuildProviderCanceler)
	if s.provider == nil || !ok {
		return job, ErrMobileBuildProviderMissing
	}
	if strings.TrimSpace(job.ProviderBuildID) == "" {
		return job, fmt.Errorf("%w: provider_build_id is required to cancel mobile build", ErrMobileBuildInvalidRequest)
	}

	result, err := provider.CancelBuild(ctx, job)
	now := s.now()
	if err != nil {
		job.Logs = append(job.Logs, MobileBuildLogLine{
			Timestamp: now,
			Level:     "error",
			Message:   RedactMobileBuildSecrets(err.Error()),
		})
		job.UpdatedAt = now
		if updateErr := s.store.Update(ctx, job); updateErr != nil {
			return job, updateErr
		}
		return job, fmt.Errorf("%w: %s", ErrMobileBuildProviderFailed, RedactMobileBuildSecrets(err.Error()))
	}

	if result.Status == "" {
		result.Status = MobileBuildCanceled
	}
	job = applyMobileBuildProviderResult(job, result, now)
	if err := s.store.Update(ctx, job); err != nil {
		return job, err
	}
	return job, nil
}

func (s *MobileBuildService) ListProjectBuilds(ctx context.Context, projectID uint) ([]MobileBuildJob, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	return s.store.ListByProject(ctx, projectID)
}

func isCancelableMobileBuildStatus(status MobileBuildStatus) bool {
	switch status {
	case MobileBuildQueued, MobileBuildPreparing, MobileBuildValidating, MobileBuildUploading, MobileBuildBuilding, MobileBuildSigning:
		return true
	default:
		return false
	}
}

func applyMobileBuildProviderResult(job MobileBuildJob, result MobileBuildProviderResult, now time.Time) MobileBuildJob {
	if providerBuildID := strings.TrimSpace(result.ProviderBuildID); providerBuildID != "" {
		job.ProviderBuildID = providerBuildID
	}
	if artifactURL := strings.TrimSpace(result.ArtifactURL); artifactURL != "" {
		job.ArtifactURL = artifactURL
	}
	if result.Status != "" {
		job.Status = normalizeMobileBuildStatus(result.Status, job.Status)
	}
	if job.Status == "" {
		job.Status = MobileBuildQueued
	}
	job.FailureType = result.FailureType
	job.FailureMessage = RedactMobileBuildSecrets(result.FailureMessage)
	if job.Status == MobileBuildFailed && job.FailureType == "" {
		job.FailureType = ClassifyMobileBuildFailure(job.FailureMessage)
	}
	job.Logs = append(job.Logs, redactMobileBuildLogLines(result.Logs, func() time.Time { return now })...)
	job.UpdatedAt = now
	return job
}

func NormalizeMobileBuildRequest(req MobileBuildRequest) MobileBuildRequest {
	req.Platform = MobilePlatform(strings.ToLower(strings.TrimSpace(string(req.Platform))))
	req.Profile = MobileBuildProfile(strings.ToLower(strings.TrimSpace(string(req.Profile))))
	req.ReleaseLevel = MobileReleaseLevel(strings.ToLower(strings.TrimSpace(string(req.ReleaseLevel))))
	if req.Profile == "" {
		req.Profile = MobileBuildProfilePreview
	}
	if req.ReleaseLevel == "" {
		req.ReleaseLevel = ReleaseDevBuild
	}
	req.AppVersion = strings.TrimSpace(req.AppVersion)
	req.BuildNumber = strings.TrimSpace(req.BuildNumber)
	req.CommitRef = strings.TrimSpace(req.CommitRef)
	req.SourcePath = strings.TrimSpace(req.SourcePath)
	return req
}

func ValidateMobileBuildRequest(flags FeatureFlags, req MobileBuildRequest) error {
	if !flags.MobileBuilderEnabled || !flags.MobileExpoEnabled || !flags.MobileEASBuildEnabled {
		return ErrMobileBuildsDisabled
	}
	if req.ProjectID == 0 {
		return fmt.Errorf("%w: project_id is required", ErrMobileBuildInvalidRequest)
	}
	if req.UserID == 0 {
		return fmt.Errorf("%w: user_id is required", ErrMobileBuildInvalidRequest)
	}
	switch req.Platform {
	case MobilePlatformAndroid:
		if !flags.MobileAndroidBuildsEnabled {
			return fmt.Errorf("%w: android", ErrMobileBuildPlatformDisabled)
		}
	case MobilePlatformIOS:
		if !flags.MobileIOSBuildsEnabled {
			return fmt.Errorf("%w: ios", ErrMobileBuildPlatformDisabled)
		}
	default:
		return fmt.Errorf("%w: unsupported platform %q", ErrMobileBuildInvalidRequest, req.Platform)
	}
	if !isSupportedMobileBuildProfile(req.Profile) {
		return fmt.Errorf("%w: unsupported build profile %q", ErrMobileBuildInvalidRequest, req.Profile)
	}
	if !isBinaryMobileReleaseLevel(req.ReleaseLevel) {
		return fmt.Errorf("%w: release level %q is not a native binary build target", ErrMobileBuildInvalidRequest, req.ReleaseLevel)
	}
	if !mobileReleaseLevelMatchesPlatform(req.ReleaseLevel, req.Platform) {
		return fmt.Errorf("%w: release level %q does not match platform %q", ErrMobileBuildInvalidRequest, req.ReleaseLevel, req.Platform)
	}
	return nil
}

func isSupportedMobileBuildProfile(profile MobileBuildProfile) bool {
	switch profile {
	case MobileBuildProfileDevelopment, MobileBuildProfilePreview, MobileBuildProfileInternal, MobileBuildProfileProduction:
		return true
	default:
		return false
	}
}

func isBinaryMobileReleaseLevel(level MobileReleaseLevel) bool {
	switch level {
	case ReleaseDevBuild, ReleaseInternalAndroidAPK, ReleaseAndroidAAB, ReleaseIOSSimulator, ReleaseIOSInternal, ReleaseTestFlightReady, ReleaseStoreSubmissionReady:
		return true
	default:
		return false
	}
}

func mobileReleaseLevelMatchesPlatform(level MobileReleaseLevel, platform MobilePlatform) bool {
	switch level {
	case ReleaseInternalAndroidAPK, ReleaseAndroidAAB:
		return platform == MobilePlatformAndroid
	case ReleaseIOSSimulator, ReleaseIOSInternal, ReleaseTestFlightReady:
		return platform == MobilePlatformIOS
	default:
		return true
	}
}

func normalizeMobileBuildStatus(status MobileBuildStatus, fallback MobileBuildStatus) MobileBuildStatus {
	switch status {
	case MobileBuildQueued, MobileBuildPreparing, MobileBuildValidating, MobileBuildUploading, MobileBuildBuilding, MobileBuildSigning, MobileBuildSucceeded, MobileBuildFailed, MobileBuildCanceled, MobileBuildRepairPending, MobileBuildRepairedRetryPending:
		return status
	case "":
		return fallback
	default:
		return MobileBuildFailed
	}
}

func ClassifyMobileBuildFailure(message string) MobileBuildFailureType {
	lower := strings.ToLower(message)
	switch {
	case containsBuildFailureSignal(lower, "npm install", "yarn install", "pnpm install", "dependency install", "unable to resolve dependency", "eresolve", "peer dependency"):
		return MobileBuildFailureDependencyInstallFailed
	case containsBuildFailureSignal(lower, "expo config", "app.config", "app.json", "invalid expo"):
		return MobileBuildFailureExpoConfigInvalid
	case containsBuildFailureSignal(lower, "unsupported native module", "native module is not supported", "not compatible with expo", "requires manual xcode", "requires manual gradle"):
		return MobileBuildFailureUnsupportedNativeModule
	case containsBuildFailureSignal(lower, "android signing", "keystore", "upload key", "jarsigner"):
		return MobileBuildFailureAndroidSigningFailed
	case containsBuildFailureSignal(lower, "provisioning profile", "provisioning failed", "no provisioning"):
		return MobileBuildFailureIOSProvisioningFailed
	case containsBuildFailureSignal(lower, "app store connect api key", "apple credential", "apple developer", "ios credential", "asc api key"):
		return MobileBuildFailureIOSCredentialsFailed
	case containsBuildFailureSignal(lower, "metro", "bundle failed", "bundling failed"):
		return MobileBuildFailureMetroBundleFailed
	case containsBuildFailureSignal(lower, "typescript", "tsc", "typecheck"):
		return MobileBuildFailureTypeScriptFailed
	case containsBuildFailureSignal(lower, "backend api mismatch", "api contract mismatch", "endpoint mismatch", "unexpected response shape"):
		return MobileBuildFailureBackendAPIMismatch
	case containsBuildFailureSignal(lower, "missing permission", "permission config", "usage description", "android.permission"):
		return MobileBuildFailurePermissionConfigMissing
	case containsBuildFailureSignal(lower, "bundle identifier", "bundle id", "android package", "application id", "package name is invalid"):
		return MobileBuildFailureAppIdentifierInvalid
	case containsBuildFailureSignal(lower, "store submission", "eas submit", "google play upload", "testflight upload"):
		return MobileBuildFailureStoreSubmissionFailed
	default:
		return MobileBuildFailureUnknown
	}
}

func RedactMobileBuildSecrets(value string) string {
	redacted := value
	for _, redactor := range mobileBuildSecretRedactors {
		redacted = redactor.pattern.ReplaceAllString(redacted, redactor.replacement)
	}
	return redacted
}

var mobileBuildSecretRedactors = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)[^\s,;]+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)(expo[_-]?token\s*[:=]\s*)[^\s,;]+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)(eas[_-]?token\s*[:=]\s*)[^\s,;]+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)("private_key"\s*:\s*")[^"]+(")`), `${1}[REDACTED]${2}`},
	{regexp.MustCompile(`(?i)(apple_[a-z0-9_]*(?:key|secret|token|issuer_id|key_id)\s*[:=]\s*)[^\s,;]+`), `${1}[REDACTED]`},
	{regexp.MustCompile(`(?i)(google_play_[a-z0-9_]*(?:key|secret|token|json)\s*[:=]\s*)[^\s,;]+`), `${1}[REDACTED]`},
}

func redactMobileBuildLogLines(logs []MobileBuildLogLine, now func() time.Time) []MobileBuildLogLine {
	if len(logs) == 0 {
		return nil
	}
	redacted := make([]MobileBuildLogLine, 0, len(logs))
	for _, line := range logs {
		if line.Timestamp.IsZero() && now != nil {
			line.Timestamp = now()
		}
		line.Message = RedactMobileBuildSecrets(line.Message)
		redacted = append(redacted, line)
	}
	return redacted
}

func containsBuildFailureSignal(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

type InMemoryMobileBuildStore struct {
	mu      sync.RWMutex
	jobs    map[string]MobileBuildJob
	project map[uint][]string
}

func NewInMemoryMobileBuildStore() *InMemoryMobileBuildStore {
	return &InMemoryMobileBuildStore{
		jobs:    map[string]MobileBuildJob{},
		project: map[uint][]string{},
	}
}

func (s *InMemoryMobileBuildStore) Save(ctx context.Context, job MobileBuildJob) error {
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

func (s *InMemoryMobileBuildStore) Update(ctx context.Context, job MobileBuildJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(job.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrMobileBuildInvalidRequest)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[job.ID]; !exists {
		return fmt.Errorf("%w: %s", ErrMobileBuildJobNotFound, job.ID)
	}
	s.jobs[job.ID] = job
	return nil
}

func (s *InMemoryMobileBuildStore) Get(ctx context.Context, id string) (MobileBuildJob, bool, error) {
	if err := ctx.Err(); err != nil {
		return MobileBuildJob{}, false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok, nil
}

func (s *InMemoryMobileBuildStore) ListByProject(ctx context.Context, projectID uint) ([]MobileBuildJob, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.project[projectID]
	jobs := make([]MobileBuildJob, 0, len(ids))
	for _, id := range ids {
		if job, ok := s.jobs[id]; ok {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func newMobileBuildID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("mbld_%d", time.Now().UnixNano())
	}
	return "mbld_" + hex.EncodeToString(bytes[:])
}
