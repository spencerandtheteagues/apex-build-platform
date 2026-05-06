package mobile

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGormMobileBuildStorePersistsJobsAndRedactsLogs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&MobileBuildRecord{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	store := NewGormMobileBuildStore(db)
	job := MobileBuildJob{
		ID:           "mbld_gorm_roundtrip",
		ProjectID:    44,
		UserID:       7,
		Platform:     MobilePlatformAndroid,
		Profile:      MobileBuildProfilePreview,
		ReleaseLevel: ReleaseInternalAndroidAPK,
		Status:       MobileBuildQueued,
		Logs: []MobileBuildLogLine{{
			Timestamp: time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
			Level:     "info",
			Message:   "queued with EXPO_TOKEN=secret-token",
		}},
		CreatedAt: time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
	}
	if err := store.Save(context.Background(), job); err != nil {
		t.Fatalf("save job: %v", err)
	}
	job.Status = MobileBuildSucceeded
	job.ArtifactURL = "https://artifacts.example.com/app.apk"
	if err := store.Update(context.Background(), job); err != nil {
		t.Fatalf("update job: %v", err)
	}

	stored, ok, err := store.Get(context.Background(), job.ID)
	if err != nil || !ok {
		t.Fatalf("expected stored job, ok=%v err=%v", ok, err)
	}
	if stored.Status != MobileBuildSucceeded || stored.ArtifactURL == "" {
		t.Fatalf("unexpected stored job %+v", stored)
	}
	if len(stored.Logs) != 1 || strings.Contains(stored.Logs[0].Message, "secret-token") {
		t.Fatalf("expected redacted stored logs, got %+v", stored.Logs)
	}
	list, err := store.ListByProject(context.Background(), 44)
	if err != nil {
		t.Fatalf("list project jobs: %v", err)
	}
	if len(list) != 1 || list[0].ID != job.ID {
		t.Fatalf("unexpected project job list %+v", list)
	}
}

func TestMobileBuildServiceRejectsWhenEASBuildDisabled(t *testing.T) {
	provider := &mockMobileBuildProvider{}
	service := NewMobileBuildService(FeatureFlags{}, provider, NewInMemoryMobileBuildStore())

	_, err := service.CreateBuild(context.Background(), validMobileBuildRequest())
	if !errors.Is(err, ErrMobileBuildsDisabled) {
		t.Fatalf("expected builds disabled error, got %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider should not be called when builds are disabled, got %d calls", provider.calls)
	}
}

func TestMobileBuildServiceRejectsDisabledPlatform(t *testing.T) {
	flags := mobileBuildTestFlags()
	flags.MobileAndroidBuildsEnabled = false
	provider := &mockMobileBuildProvider{}
	service := NewMobileBuildService(flags, provider, NewInMemoryMobileBuildStore())

	_, err := service.CreateBuild(context.Background(), validMobileBuildRequest())
	if !errors.Is(err, ErrMobileBuildPlatformDisabled) {
		t.Fatalf("expected platform disabled error, got %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider should not be called when platform is disabled, got %d calls", provider.calls)
	}
}

func TestMobileBuildServiceRejectsSourceOnlyReleaseAsBinaryBuild(t *testing.T) {
	flags := mobileBuildTestFlags()
	provider := &mockMobileBuildProvider{}
	service := NewMobileBuildService(flags, provider, NewInMemoryMobileBuildStore())
	req := validMobileBuildRequest()
	req.ReleaseLevel = ReleaseSourceOnly

	_, err := service.CreateBuild(context.Background(), req)
	if !errors.Is(err, ErrMobileBuildInvalidRequest) {
		t.Fatalf("expected invalid request error, got %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider should not be called for source-only release level, got %d calls", provider.calls)
	}
}

func TestMobileBuildServiceRejectsMismatchedPlatformReleaseLevel(t *testing.T) {
	flags := mobileBuildTestFlags()
	provider := &mockMobileBuildProvider{}
	service := NewMobileBuildService(flags, provider, NewInMemoryMobileBuildStore())
	req := validMobileBuildRequest()
	req.Platform = MobilePlatformIOS
	req.ReleaseLevel = ReleaseAndroidAAB

	_, err := service.CreateBuild(context.Background(), req)
	if !errors.Is(err, ErrMobileBuildInvalidRequest) {
		t.Fatalf("expected invalid request error, got %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider should not be called for mismatched release level, got %d calls", provider.calls)
	}
}

func TestMobileBuildServiceCreatesAndroidBuildJobWithProviderResult(t *testing.T) {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	store := NewInMemoryMobileBuildStore()
	provider := &mockMobileBuildProvider{
		name: "mock-eas",
		result: MobileBuildProviderResult{
			ProviderBuildID: "eas-build-123",
			Status:          MobileBuildBuilding,
			ArtifactURL:     "https://artifacts.example.com/app.apk",
			Logs: []MobileBuildLogLine{{
				Level:   "info",
				Message: "queued with Authorization: Bearer secret-token and EXPO_TOKEN=abc123",
			}},
		},
	}
	service := NewMobileBuildService(
		mobileBuildTestFlags(),
		provider,
		store,
		WithMobileBuildClock(func() time.Time { return now }),
		WithMobileBuildIDGenerator(func() string { return "mbld_test_success" }),
	)

	job, err := service.CreateBuild(context.Background(), validMobileBuildRequest())
	if err != nil {
		t.Fatalf("expected build job, got error %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("expected one provider call, got %d", provider.calls)
	}
	if provider.lastReq.Platform != MobilePlatformAndroid || provider.lastReq.Profile != MobileBuildProfilePreview {
		t.Fatalf("unexpected provider request %+v", provider.lastReq)
	}
	if job.ID != "mbld_test_success" || job.Provider != "mock-eas" || job.ProviderBuildID != "eas-build-123" {
		t.Fatalf("unexpected job identity/provider fields %+v", job)
	}
	if job.Status != MobileBuildBuilding {
		t.Fatalf("expected building status, got %q", job.Status)
	}
	if job.ArtifactURL == "" {
		t.Fatalf("expected artifact URL, got %+v", job)
	}
	if len(job.Logs) != 1 || strings.Contains(job.Logs[0].Message, "secret-token") || strings.Contains(job.Logs[0].Message, "abc123") {
		t.Fatalf("expected redacted logs, got %+v", job.Logs)
	}

	stored, ok, err := store.Get(context.Background(), job.ID)
	if err != nil || !ok {
		t.Fatalf("expected stored job, ok=%v err=%v", ok, err)
	}
	if stored.Status != MobileBuildBuilding || stored.ProviderBuildID != "eas-build-123" {
		t.Fatalf("unexpected stored job %+v", stored)
	}
}

func TestMobileBuildServiceNormalizesRequestEnumCasing(t *testing.T) {
	provider := &mockMobileBuildProvider{
		name: "mock-eas",
		result: MobileBuildProviderResult{
			Status: MobileBuildQueued,
		},
	}
	service := NewMobileBuildService(mobileBuildTestFlags(), provider, NewInMemoryMobileBuildStore())
	req := validMobileBuildRequest()
	req.Platform = MobilePlatform(" Android ")
	req.Profile = MobileBuildProfile(" Preview ")
	req.ReleaseLevel = MobileReleaseLevel(" Internal_Android_APK ")

	if _, err := service.CreateBuild(context.Background(), req); err != nil {
		t.Fatalf("expected normalized request to pass, got %v", err)
	}
	if provider.lastReq.Platform != MobilePlatformAndroid || provider.lastReq.Profile != MobileBuildProfilePreview || provider.lastReq.ReleaseLevel != ReleaseInternalAndroidAPK {
		t.Fatalf("expected normalized provider request, got %+v", provider.lastReq)
	}
}

func TestMobileBuildServiceRecordsProviderFailure(t *testing.T) {
	store := NewInMemoryMobileBuildStore()
	provider := &mockMobileBuildProvider{
		name: "mock-eas",
		err:  errors.New("metro bundle failed with EXPO_TOKEN=should-not-leak"),
	}
	service := NewMobileBuildService(
		mobileBuildTestFlags(),
		provider,
		store,
		WithMobileBuildIDGenerator(func() string { return "mbld_test_failure" }),
	)

	job, err := service.CreateBuild(context.Background(), validMobileBuildRequest())
	if !errors.Is(err, ErrMobileBuildProviderFailed) {
		t.Fatalf("expected provider failed error, got %v", err)
	}
	if strings.Contains(err.Error(), "should-not-leak") {
		t.Fatalf("expected returned error to redact secrets, got %v", err)
	}
	if job.Status != MobileBuildFailed {
		t.Fatalf("expected failed job, got %+v", job)
	}
	if job.FailureType != MobileBuildFailureMetroBundleFailed {
		t.Fatalf("expected metro failure classification, got %+v", job)
	}
	if strings.Contains(job.FailureMessage, "should-not-leak") {
		t.Fatalf("expected redacted failure message, got %q", job.FailureMessage)
	}
	if len(job.Logs) == 0 || strings.Contains(job.Logs[0].Message, "should-not-leak") {
		t.Fatalf("expected redacted failure log, got %+v", job.Logs)
	}

	stored, ok, err := store.Get(context.Background(), job.ID)
	if err != nil || !ok {
		t.Fatalf("expected stored failed job, ok=%v err=%v", ok, err)
	}
	if stored.Status != MobileBuildFailed || stored.FailureType != MobileBuildFailureMetroBundleFailed {
		t.Fatalf("unexpected stored failed job %+v", stored)
	}
}

func TestInMemoryMobileBuildStoreRoundTrip(t *testing.T) {
	store := NewInMemoryMobileBuildStore()
	job := MobileBuildJob{
		ID:        "mbld_roundtrip",
		ProjectID: 44,
		UserID:    7,
		Platform:  MobilePlatformIOS,
		Profile:   MobileBuildProfileInternal,
		Status:    MobileBuildQueued,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := store.Save(context.Background(), job); err != nil {
		t.Fatalf("save job: %v", err)
	}
	job.Status = MobileBuildSucceeded
	if err := store.Update(context.Background(), job); err != nil {
		t.Fatalf("update job: %v", err)
	}
	stored, ok, err := store.Get(context.Background(), job.ID)
	if err != nil || !ok {
		t.Fatalf("expected stored job, ok=%v err=%v", ok, err)
	}
	if stored.Status != MobileBuildSucceeded {
		t.Fatalf("expected succeeded job, got %+v", stored)
	}
	list, err := store.ListByProject(context.Background(), 44)
	if err != nil {
		t.Fatalf("list by project: %v", err)
	}
	if len(list) != 1 || list[0].ID != job.ID {
		t.Fatalf("unexpected project jobs %+v", list)
	}
}

func TestClassifyMobileBuildFailure(t *testing.T) {
	cases := []struct {
		message string
		want    MobileBuildFailureType
	}{
		{"npm install failed with ERESOLVE peer dependency", MobileBuildFailureDependencyInstallFailed},
		{"invalid Expo config in app.config.ts", MobileBuildFailureExpoConfigInvalid},
		{"unsupported native module requires manual Xcode edits", MobileBuildFailureUnsupportedNativeModule},
		{"keystore upload key rejected", MobileBuildFailureAndroidSigningFailed},
		{"missing provisioning profile", MobileBuildFailureIOSProvisioningFailed},
		{"App Store Connect API key invalid", MobileBuildFailureIOSCredentialsFailed},
		{"Metro bundle failed", MobileBuildFailureMetroBundleFailed},
		{"TypeScript typecheck failed", MobileBuildFailureTypeScriptFailed},
		{"backend API mismatch on endpoint response shape", MobileBuildFailureBackendAPIMismatch},
		{"missing permission usage description", MobileBuildFailurePermissionConfigMissing},
		{"bundle identifier is invalid", MobileBuildFailureAppIdentifierInvalid},
		{"EAS submit TestFlight upload failed", MobileBuildFailureStoreSubmissionFailed},
		{"unexpected provider outage", MobileBuildFailureUnknown},
	}
	for _, tc := range cases {
		if got := ClassifyMobileBuildFailure(tc.message); got != tc.want {
			t.Fatalf("ClassifyMobileBuildFailure(%q) = %q, want %q", tc.message, got, tc.want)
		}
	}
}

func TestRedactMobileBuildSecrets(t *testing.T) {
	raw := `Authorization: Bearer token123 EXPO_TOKEN=expo-secret EAS_TOKEN=eas-secret "private_key":"-----BEGIN PRIVATE KEY-----abc" APPLE_API_KEY=apple-secret GOOGLE_PLAY_JSON=google-secret`
	redacted := RedactMobileBuildSecrets(raw)
	for _, secret := range []string{"token123", "expo-secret", "eas-secret", "abc", "apple-secret", "google-secret"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("expected %q to be redacted from %q", secret, redacted)
		}
	}
	if strings.Count(redacted, "[REDACTED]") < 6 {
		t.Fatalf("expected redaction markers, got %q", redacted)
	}
}

type mockMobileBuildProvider struct {
	name    string
	result  MobileBuildProviderResult
	err     error
	calls   int
	lastReq MobileBuildRequest
}

func (p *mockMobileBuildProvider) Name() string {
	return p.name
}

func (p *mockMobileBuildProvider) CreateBuild(_ context.Context, req MobileBuildRequest) (MobileBuildProviderResult, error) {
	p.calls++
	p.lastReq = req
	if p.err != nil {
		return MobileBuildProviderResult{}, p.err
	}
	return p.result, nil
}

func mobileBuildTestFlags() FeatureFlags {
	return FeatureFlags{
		MobileBuilderEnabled:       true,
		MobileExpoEnabled:          true,
		MobileEASBuildEnabled:      true,
		MobileAndroidBuildsEnabled: true,
		MobileIOSBuildsEnabled:     true,
	}
}

func validMobileBuildRequest() MobileBuildRequest {
	return MobileBuildRequest{
		ProjectID:    71,
		UserID:       9,
		Platform:     MobilePlatformAndroid,
		Profile:      MobileBuildProfilePreview,
		ReleaseLevel: ReleaseInternalAndroidAPK,
		AppVersion:   "1.0.0",
		BuildNumber:  "1",
		VersionCode:  1,
		CommitRef:    "main",
		SourcePath:   "mobile/",
	}
}
