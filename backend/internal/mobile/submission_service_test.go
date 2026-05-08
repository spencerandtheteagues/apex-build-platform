package mobile

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMobileSubmissionServiceGatesFeatureFlagsAndProvider(t *testing.T) {
	service := NewMobileSubmissionService(FeatureFlags{}, &mockMobileSubmissionProvider{}, NewInMemoryMobileSubmissionStore())
	_, err := service.CreateSubmission(context.Background(), validMobileSubmissionRequest())
	if !errors.Is(err, ErrMobileBuildsDisabled) {
		t.Fatalf("expected disabled submission error, got %v", err)
	}

	service = NewMobileSubmissionService(mobileBuildTestFlags(), nil, NewInMemoryMobileSubmissionStore())
	_, err = service.CreateSubmission(context.Background(), validMobileSubmissionRequest())
	if !errors.Is(err, ErrMobileBuildsDisabled) {
		t.Fatalf("expected submit flag disabled error, got %v", err)
	}
}

func TestMobileSubmissionServiceCreatesAndPersistsSubmission(t *testing.T) {
	flags := mobileBuildTestFlags()
	flags.MobileEASSubmitEnabled = true
	provider := &mockMobileSubmissionProvider{
		result: MobileSubmissionProviderResult{
			ProviderSubmissionID: "eas-submit-123",
			Status:               MobileSubmissionReadyForGoogleInternalTesting,
			Logs: []MobileBuildLogLine{{
				Level:   "info",
				Message: "submitted with EAS_TOKEN=should-redact",
			}},
		},
	}
	store := NewInMemoryMobileSubmissionStore()
	service := NewMobileSubmissionService(flags, provider, store, WithMobileSubmissionIDGenerator(func() string { return "msub_test" }))

	job, err := service.CreateSubmission(context.Background(), validMobileSubmissionRequest())
	if err != nil {
		t.Fatalf("create submission: %v", err)
	}
	if job.ID != "msub_test" || job.ProviderSubmissionID != "eas-submit-123" || job.Status != MobileSubmissionReadyForGoogleInternalTesting {
		t.Fatalf("unexpected submission job %+v", job)
	}
	if len(job.Logs) == 0 || strings.Contains(job.Logs[0].Message, "should-redact") {
		t.Fatalf("expected redacted submission logs, got %+v", job.Logs)
	}
	stored, ok, err := service.GetSubmission(context.Background(), "msub_test")
	if err != nil || !ok || stored.ID != "msub_test" {
		t.Fatalf("expected stored submission, got job=%+v ok=%v err=%v", stored, ok, err)
	}
}

func TestMobileSubmissionServiceRecordsProviderFailure(t *testing.T) {
	flags := mobileBuildTestFlags()
	flags.MobileEASSubmitEnabled = true
	service := NewMobileSubmissionService(
		flags,
		&mockMobileSubmissionProvider{err: errors.New("submit failed with EAS_TOKEN=should-redact")},
		NewInMemoryMobileSubmissionStore(),
		WithMobileSubmissionIDGenerator(func() string { return "msub_failed" }),
	)

	job, err := service.CreateSubmission(context.Background(), validMobileSubmissionRequest())
	if !errors.Is(err, ErrMobileBuildProviderFailed) {
		t.Fatalf("expected provider failure, got %v", err)
	}
	if job.Status != MobileSubmissionFailed || job.FailureType != MobileBuildFailureStoreSubmissionFailed {
		t.Fatalf("expected failed submission, got %+v", job)
	}
	if strings.Contains(job.FailureMessage, "should-redact") {
		t.Fatalf("expected redacted failure message, got %q", job.FailureMessage)
	}
}

func validMobileSubmissionRequest() MobileSubmissionRequest {
	return MobileSubmissionRequest{
		ProjectID:       10,
		UserID:          20,
		BuildID:         "mbld_success",
		Platform:        MobilePlatformAndroid,
		ProviderBuildID: "eas-build-123",
	}
}

type mockMobileSubmissionProvider struct {
	result MobileSubmissionProviderResult
	err    error
	calls  int
	last   MobileSubmissionRequest
}

func (p *mockMobileSubmissionProvider) Name() string {
	return "mock-eas-submit"
}

func (p *mockMobileSubmissionProvider) SubmitBuild(_ context.Context, req MobileSubmissionRequest) (MobileSubmissionProviderResult, error) {
	p.calls++
	p.last = req
	if p.err != nil {
		return MobileSubmissionProviderResult{}, p.err
	}
	return p.result, nil
}
