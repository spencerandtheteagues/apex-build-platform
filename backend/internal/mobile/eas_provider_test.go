package mobile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestEASBuildProviderQueuesNoWaitBuildWithVaultToken(t *testing.T) {
	sourcePath := newEASProviderSourceDir(t)
	resolver := &fakeMobileCredentialResolver{
		values: map[string]string{"token": "secret-eas-token"},
	}
	runner := &fakeEASCommandRunner{
		output: `{"id":"eas-build-123","status":"in_progress","artifacts":{"buildUrl":"https://artifacts.example.com/app.apk"}}`,
	}
	provider := NewEASBuildProvider(EASBuildProviderConfig{
		CLIPath:     "/usr/local/bin/eas",
		Credentials: resolver,
		Runner:      runner,
	})

	result, err := provider.CreateBuild(context.Background(), MobileBuildRequest{
		ProjectID:    71,
		UserID:       9,
		Platform:     MobilePlatformAndroid,
		Profile:      MobileBuildProfilePreview,
		ReleaseLevel: ReleaseInternalAndroidAPK,
		SourcePath:   sourcePath,
	})
	if err != nil {
		t.Fatalf("expected EAS provider result, got error %v", err)
	}
	if resolver.calls != 1 || resolver.lastType != MobileCredentialEASToken {
		t.Fatalf("expected EAS token resolution, resolver=%+v", resolver)
	}
	if runner.calls != 1 {
		t.Fatalf("expected one EAS command call, got %d", runner.calls)
	}
	if runner.lastCommand.CLIPath != "/usr/local/bin/eas" || runner.lastCommand.WorkDir != sourcePath {
		t.Fatalf("unexpected command path fields %+v", runner.lastCommand)
	}
	for _, arg := range []string{"build", "--platform", "android", "--profile", "preview", "--non-interactive", "--no-wait", "--json"} {
		if !slices.Contains(runner.lastCommand.Args, arg) {
			t.Fatalf("expected command args to include %q, got %+v", arg, runner.lastCommand.Args)
		}
	}
	if !slices.Contains(runner.lastCommand.Env, "EXPO_TOKEN=secret-eas-token") || !slices.Contains(runner.lastCommand.Env, "EAS_TOKEN=secret-eas-token") {
		t.Fatalf("expected EAS token env vars, got %+v", runner.lastCommand.Env)
	}
	if !slices.Contains(runner.lastCommand.Env, "EAS_NO_VCS=1") || !slices.Contains(runner.lastCommand.Env, "CI=1") {
		t.Fatalf("expected non-interactive EAS env vars, got %+v", runner.lastCommand.Env)
	}
	if result.ProviderBuildID != "eas-build-123" || result.Status != MobileBuildBuilding || result.ArtifactURL == "" {
		t.Fatalf("unexpected provider result %+v", result)
	}
	for _, line := range result.Logs {
		if strings.Contains(line.Message, "secret-eas-token") {
			t.Fatalf("provider result leaked token in logs: %+v", result.Logs)
		}
	}
}

func TestEASBuildProviderDryRunValidatesWithoutInvokingRunner(t *testing.T) {
	sourcePath := newEASProviderSourceDir(t)
	resolver := &fakeMobileCredentialResolver{values: map[string]string{"token": "secret-eas-token"}}
	runner := &fakeEASCommandRunner{}
	provider := NewEASBuildProvider(EASBuildProviderConfig{
		Credentials: resolver,
		Runner:      runner,
	})

	result, err := provider.CreateBuild(context.Background(), MobileBuildRequest{
		ProjectID:    71,
		UserID:       9,
		Platform:     MobilePlatformAndroid,
		Profile:      MobileBuildProfilePreview,
		ReleaseLevel: ReleaseInternalAndroidAPK,
		SourcePath:   sourcePath,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("expected dry-run success, got %v", err)
	}
	if runner.calls != 0 {
		t.Fatalf("dry run should not invoke EAS runner, got %d calls", runner.calls)
	}
	if resolver.calls != 1 {
		t.Fatalf("dry run should still validate credentials, got %d calls", resolver.calls)
	}
	if result.Status != MobileBuildQueued || len(result.Logs) == 0 {
		t.Fatalf("unexpected dry-run result %+v", result)
	}
}

func TestEASBuildProviderRejectsUnsafeSourcePath(t *testing.T) {
	provider := NewEASBuildProvider(EASBuildProviderConfig{
		Credentials: &fakeMobileCredentialResolver{values: map[string]string{"token": "secret-eas-token"}},
		Runner:      &fakeEASCommandRunner{},
	})

	err := provider.ValidateBuildRequest(MobileBuildRequest{
		ProjectID:    71,
		UserID:       9,
		Platform:     MobilePlatformAndroid,
		Profile:      MobileBuildProfilePreview,
		ReleaseLevel: ReleaseInternalAndroidAPK,
		SourcePath:   "relative/mobile",
	})
	if !errors.Is(err, ErrMobileBuildInvalidRequest) {
		t.Fatalf("expected invalid request for relative source path, got %v", err)
	}

	err = provider.ValidateBuildRequest(MobileBuildRequest{
		ProjectID:    71,
		UserID:       9,
		Platform:     MobilePlatformAndroid,
		Profile:      MobileBuildProfilePreview,
		ReleaseLevel: ReleaseInternalAndroidAPK,
		SourcePath:   t.TempDir(),
	})
	if !errors.Is(err, ErrMobileBuildInvalidRequest) {
		t.Fatalf("expected invalid request for source without package.json, got %v", err)
	}
}

func TestEASBuildProviderRedactsRunnerFailures(t *testing.T) {
	sourcePath := newEASProviderSourceDir(t)
	resolver := &fakeMobileCredentialResolver{values: map[string]string{"token": "secret-eas-token"}}
	runner := &fakeEASCommandRunner{
		output: "upload failed with EXPO_TOKEN=secret-eas-token",
		err:    errors.New("provider rejected EAS_TOKEN=secret-eas-token"),
	}
	provider := NewEASBuildProvider(EASBuildProviderConfig{
		Credentials: resolver,
		Runner:      runner,
	})

	_, err := provider.CreateBuild(context.Background(), MobileBuildRequest{
		ProjectID:    71,
		UserID:       9,
		Platform:     MobilePlatformAndroid,
		Profile:      MobileBuildProfilePreview,
		ReleaseLevel: ReleaseInternalAndroidAPK,
		SourcePath:   sourcePath,
	})
	if !errors.Is(err, ErrMobileBuildProviderFailed) {
		t.Fatalf("expected provider failure, got %v", err)
	}
	if strings.Contains(err.Error(), "secret-eas-token") {
		t.Fatalf("expected redacted provider error, got %v", err)
	}
}

func TestEASBuildProviderRefreshesBuildStatusWithArtifact(t *testing.T) {
	resolver := &fakeMobileCredentialResolver{values: map[string]string{"token": "secret-eas-token"}}
	runner := &fakeEASCommandRunner{
		output: `warning before json {"id":"eas-build-123","status":"finished","artifacts":{"applicationArchiveUrl":"https://artifacts.example.com/app.aab"}} trailing text`,
	}
	provider := NewEASBuildProvider(EASBuildProviderConfig{
		CLIPath:     "/usr/local/bin/eas",
		Credentials: resolver,
		Runner:      runner,
	})

	result, err := provider.RefreshBuild(context.Background(), MobileBuildJob{
		ID:              "mbld_refresh",
		ProjectID:       71,
		UserID:          9,
		Platform:        MobilePlatformAndroid,
		Profile:         MobileBuildProfilePreview,
		ReleaseLevel:    ReleaseInternalAndroidAPK,
		ProviderBuildID: "eas-build-123",
	})
	if err != nil {
		t.Fatalf("expected refresh result, got %v", err)
	}
	if resolver.calls != 1 || resolver.lastType != MobileCredentialEASToken {
		t.Fatalf("expected EAS token resolution, resolver=%+v", resolver)
	}
	if runner.calls != 1 {
		t.Fatalf("expected one EAS refresh call, got %d", runner.calls)
	}
	for _, arg := range []string{"build:view", "eas-build-123", "--json"} {
		if !slices.Contains(runner.lastCommand.Args, arg) {
			t.Fatalf("expected refresh args to include %q, got %+v", arg, runner.lastCommand.Args)
		}
	}
	if result.ProviderBuildID != "eas-build-123" || result.Status != MobileBuildSucceeded || result.ArtifactURL == "" {
		t.Fatalf("unexpected refresh result %+v", result)
	}
	for _, line := range result.Logs {
		if strings.Contains(line.Message, "secret-eas-token") {
			t.Fatalf("refresh result leaked token in logs: %+v", result.Logs)
		}
	}
}

func TestEASBuildProviderRefreshRedactsRunnerFailures(t *testing.T) {
	resolver := &fakeMobileCredentialResolver{values: map[string]string{"token": "secret-eas-token"}}
	runner := &fakeEASCommandRunner{
		output: "inspect failed with EXPO_TOKEN=secret-eas-token",
		err:    errors.New("provider rejected EAS_TOKEN=secret-eas-token"),
	}
	provider := NewEASBuildProvider(EASBuildProviderConfig{
		Credentials: resolver,
		Runner:      runner,
	})

	_, err := provider.RefreshBuild(context.Background(), MobileBuildJob{
		ID:              "mbld_refresh",
		ProjectID:       71,
		UserID:          9,
		ProviderBuildID: "eas-build-123",
	})
	if !errors.Is(err, ErrMobileBuildProviderFailed) {
		t.Fatalf("expected provider failure, got %v", err)
	}
	if strings.Contains(err.Error(), "secret-eas-token") {
		t.Fatalf("expected redacted refresh error, got %v", err)
	}
}

func TestEASBuildProviderCancelsBuild(t *testing.T) {
	resolver := &fakeMobileCredentialResolver{values: map[string]string{"token": "secret-eas-token"}}
	runner := &fakeEASCommandRunner{
		output: `{"id":"eas-build-123","status":"canceled"}`,
	}
	provider := NewEASBuildProvider(EASBuildProviderConfig{
		CLIPath:     "/usr/local/bin/eas",
		Credentials: resolver,
		Runner:      runner,
	})

	result, err := provider.CancelBuild(context.Background(), MobileBuildJob{
		ID:              "mbld_cancel",
		ProjectID:       71,
		UserID:          9,
		ProviderBuildID: "eas-build-123",
	})
	if err != nil {
		t.Fatalf("expected cancel result, got %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("expected one EAS cancel call, got %d", runner.calls)
	}
	for _, arg := range []string{"build:cancel", "eas-build-123", "--non-interactive", "--json"} {
		if !slices.Contains(runner.lastCommand.Args, arg) {
			t.Fatalf("expected cancel args to include %q, got %+v", arg, runner.lastCommand.Args)
		}
	}
	if result.ProviderBuildID != "eas-build-123" || result.Status != MobileBuildCanceled {
		t.Fatalf("unexpected cancel result %+v", result)
	}
}

func TestEASBuildProviderCancelRedactsRunnerFailures(t *testing.T) {
	resolver := &fakeMobileCredentialResolver{values: map[string]string{"token": "secret-eas-token"}}
	runner := &fakeEASCommandRunner{
		output: "cancel failed with EXPO_TOKEN=secret-eas-token",
		err:    errors.New("provider rejected EAS_TOKEN=secret-eas-token"),
	}
	provider := NewEASBuildProvider(EASBuildProviderConfig{
		Credentials: resolver,
		Runner:      runner,
	})

	_, err := provider.CancelBuild(context.Background(), MobileBuildJob{
		ID:              "mbld_cancel",
		ProjectID:       71,
		UserID:          9,
		ProviderBuildID: "eas-build-123",
	})
	if !errors.Is(err, ErrMobileBuildProviderFailed) {
		t.Fatalf("expected provider failure, got %v", err)
	}
	if strings.Contains(err.Error(), "secret-eas-token") {
		t.Fatalf("expected redacted cancel error, got %v", err)
	}
}

func newEASProviderSourceDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := writeMobileTestFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"build":"expo export"}}`)); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := writeMobileTestFile(filepath.Join(dir, "eas.json"), []byte(`{"build":{"preview":{"distribution":"internal"}}}`)); err != nil {
		t.Fatalf("write eas.json: %v", err)
	}
	if err := writeMobileTestFile(filepath.Join(dir, "app.config.ts"), []byte(`export default { expo: { name: 'App', slug: 'app' } }`)); err != nil {
		t.Fatalf("write app.config.ts: %v", err)
	}
	return dir
}

func writeMobileTestFile(path string, content []byte) error {
	return os.WriteFile(path, content, 0o644)
}

type fakeMobileCredentialResolver struct {
	values      map[string]string
	err         error
	calls       int
	lastUserID  uint
	lastProject uint
	lastType    MobileCredentialType
}

func (r *fakeMobileCredentialResolver) ResolveCredentialValues(_ context.Context, userID, projectID uint, credType MobileCredentialType) (map[string]string, error) {
	r.calls++
	r.lastUserID = userID
	r.lastProject = projectID
	r.lastType = credType
	if r.err != nil {
		return nil, r.err
	}
	return r.values, nil
}

type fakeEASCommandRunner struct {
	output      string
	err         error
	calls       int
	lastCommand EASCommand
}

func (r *fakeEASCommandRunner) Run(_ context.Context, command EASCommand) (string, error) {
	r.calls++
	r.lastCommand = command
	return r.output, r.err
}
