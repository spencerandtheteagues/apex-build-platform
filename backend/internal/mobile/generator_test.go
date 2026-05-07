package mobile

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"apex-build/pkg/models"
)

func TestGenerateExpoProjectCreatesFieldServiceSource(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}
	if !hasSourceFile(files, "mobile/package.json") ||
		!hasSourceFile(files, "mobile/app.config.ts") ||
		!hasSourceFile(files, "mobile/app/(tabs)/jobs.tsx") ||
		!hasSourceFile(files, "mobile/src/api/client.ts") ||
		!hasSourceFile(files, "mobile/src/api/endpoints.ts") ||
		!hasSourceFile(files, "mobile/src/api/types.ts") ||
		!hasSourceFile(files, "mobile/store/store-readiness.json") ||
		!hasSourceFile(files, "mobile/store/privacy-data-safety.md") ||
		!hasSourceFile(files, "mobile/store/screenshot-checklist.md") ||
		!hasSourceFile(files, "mobile/store/release-notes.md") ||
		!hasSourceFile(files, "mobile/assets/icon.png") ||
		!hasSourceFile(files, "mobile/assets/splash.png") ||
		!hasSourceFile(files, "mobile/assets/adaptive-icon.png") {
		t.Fatalf("expected core Expo files, got %+v", sourceFilePaths(files))
	}
	if errs := ValidateGeneratedExpoFiles(files, ExpoDependenciesForSpec(spec, ExpoGeneratorOptions{}), DefaultNativeCapabilityRegistry()); len(errs) > 0 {
		t.Fatalf("expected generated files to pass policy validation, got %+v", errs)
	}
}

func TestGenerateExpoProjectCreatesContractDrivenAPIClient(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}

	client := findSourceFile(t, files, "mobile/src/api/client.ts")
	endpoints := findSourceFile(t, files, "mobile/src/api/endpoints.ts")
	types := findSourceFile(t, files, "mobile/src/api/types.ts")

	for _, expected := range []string{"json?: unknown", "auth?: boolean", "buildPath", "SecureStore.getItemAsync('auth_token')"} {
		if !strings.Contains(client.Content, expected) {
			t.Fatalf("expected API client to contain %q\n%s", expected, client.Content)
		}
	}
	for _, expected := range []string{
		"export async function login(payload: LoginRequest): Promise<AuthSession>",
		"auth: false",
		"export async function listJobs(): Promise<Job[]>",
		"export async function syncEstimate(payload: EstimateDraft): Promise<Estimate>",
		"export async function uploadJobPhoto(pathParams: { id: string | number }, formData: FormData): Promise<PhotoAsset>",
		"buildPath('/api/jobs/:id/photos', pathParams)",
	} {
		if !strings.Contains(endpoints.Content, expected) {
			t.Fatalf("expected endpoints to contain %q\n%s", expected, endpoints.Content)
		}
	}
	for _, expected := range []string{"export type LoginRequest", "export type Customer", "export type Estimate", "export type PhotoAsset"} {
		if !strings.Contains(types.Content, expected) {
			t.Fatalf("expected API types to contain %q\n%s", expected, types.Content)
		}
	}
}

func TestGenerateExpoProjectWiresScreensToAPIEndpointsWithOfflineFallback(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}

	auth := findSourceFile(t, files, "mobile/src/auth/AuthProvider.tsx")
	jobs := findSourceFile(t, files, "mobile/app/(tabs)/jobs.tsx")
	estimates := findSourceFile(t, files, "mobile/app/(tabs)/estimates.tsx")

	for _, expected := range []string{"import { login } from '@/api/endpoints'", "await login({ email, password })", "demo-token-"} {
		if !strings.Contains(auth.Content, expected) {
			t.Fatalf("expected auth provider to contain %q\n%s", expected, auth.Content)
		}
	}
	for _, expected := range []string{"import { listJobs } from '@/api/endpoints'", "queryFn: listJobs", "offlineJobs", "Backend unavailable. Showing offline sample data."} {
		if !strings.Contains(jobs.Content, expected) {
			t.Fatalf("expected jobs screen to contain %q\n%s", expected, jobs.Content)
		}
	}
	for _, expected := range []string{"import { syncEstimate } from '@/api/endpoints'", "await syncEstimate(draft)", "await queueDraftForSync(draft)"} {
		if !strings.Contains(estimates.Content, expected) {
			t.Fatalf("expected estimates screen to contain %q\n%s", expected, estimates.Content)
		}
	}
}

func TestGenerateStoreReadinessPackageLabelsDraftsAndManualPrerequisites(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	pkg := GenerateStoreReadinessPackage(spec)

	if pkg.Status != "draft_ready_needs_manual_store_assets" {
		t.Fatalf("unexpected store readiness status %q", pkg.Status)
	}
	if len(pkg.ManualPrerequisites) == 0 || len(pkg.MissingItems) == 0 {
		t.Fatalf("expected manual prerequisites and missing items, got %+v", pkg)
	}
	if !stringSliceContains(pkg.DataSafetyDraft.DataCollected, "Photos or documents selected by the user") {
		t.Fatalf("expected photo/file data safety draft, got %+v", pkg.DataSafetyDraft.DataCollected)
	}
	if !hasScreenshotTarget(pkg.ScreenshotChecklist, "Android") || !hasScreenshotTarget(pkg.ScreenshotChecklist, "iOS") {
		t.Fatalf("expected Android and iOS screenshot targets, got %+v", pkg.ScreenshotChecklist)
	}
	if errs := ValidateStoreReadinessPackage(pkg); len(errs) > 0 {
		t.Fatalf("expected store readiness package to validate, got %+v", errs)
	}
}

func TestValidateProjectSourcePackagePassesGeneratedExpoSource(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}

	report := ValidateProjectSourcePackage(modelsProjectForSpec(spec, string(ReleaseSourceOnly), "not_requested"), modelFilesFromSource(files))
	if report.Status != MobileValidationPassed {
		t.Fatalf("expected validation passed, got %+v", report)
	}
	if !hasMobileValidationCheck(report, "store_readiness", MobileValidationPassed) {
		t.Fatalf("expected store readiness check to pass, got %+v", report.Checks)
	}
}

func TestValidateProjectSourcePackageFailsMissingRequiredFiles(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}
	files = removeSourceFile(files, "mobile/eas.json")

	report := ValidateProjectSourcePackage(modelsProjectForSpec(spec, string(ReleaseSourceOnly), "not_requested"), modelFilesFromSource(files))
	if report.Status != MobileValidationFailed {
		t.Fatalf("expected validation failed, got %+v", report)
	}
	if !stringSliceContains(report.MissingFiles, "mobile/eas.json") {
		t.Fatalf("expected missing eas.json, got %+v", report.MissingFiles)
	}
}

func TestValidateProjectSourcePackageRequiresGeneratedAPIClientFiles(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}
	files = removeSourceFile(files, "mobile/src/api/endpoints.ts")

	report := ValidateProjectSourcePackage(modelsProjectForSpec(spec, string(ReleaseSourceOnly), "not_requested"), modelFilesFromSource(files))
	if report.Status != MobileValidationFailed {
		t.Fatalf("expected validation failed, got %+v", report)
	}
	if !stringSliceContains(report.MissingFiles, "mobile/src/api/endpoints.ts") {
		t.Fatalf("expected missing endpoints.ts, got %+v", report.MissingFiles)
	}
}

func TestValidateProjectSourcePackageRejectsNativeClaimWithoutBuildSuccess(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}

	report := ValidateProjectSourcePackage(modelsProjectForSpec(spec, string(ReleaseAndroidAAB), "not_requested"), modelFilesFromSource(files))
	if report.Status != MobileValidationFailed {
		t.Fatalf("expected native release claim to fail without build success, got %+v", report)
	}
	if !hasMobileValidationCheck(report, "release_truth", MobileValidationFailed) {
		t.Fatalf("expected release truth check failure, got %+v", report.Checks)
	}
}

func TestGenerateExpoProjectRejectsInvalidSpec(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	spec.Identity.AndroidPackage = "Invalid.Package"
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) == 0 {
		t.Fatalf("expected invalid spec error, got files %+v", files)
	}
	if !hasValidationError(errs, "identity.android_package") {
		t.Fatalf("expected android package error, got %+v", errs)
	}
}

func TestValidateExpoDependencyPolicyRejectsUnsupportedPackage(t *testing.T) {
	errs := ValidateExpoDependencyPolicy(map[string]string{
		"expo":                       "~55.0.0",
		"react-native-vision-camera": "^4.0.0",
	}, DefaultNativeCapabilityRegistry())
	if !hasValidationError(errs, "dependencies.react-native-vision-camera") {
		t.Fatalf("expected unsupported dependency error, got %+v", errs)
	}
}

func TestValidateExpoDependencyPolicyRequiresDeterministicVersions(t *testing.T) {
	errs := ValidateExpoDependencyPolicy(map[string]string{
		"expo":        "latest",
		"expo-camera": "*",
	}, DefaultNativeCapabilityRegistry())
	if !hasValidationError(errs, "dependencies.expo") {
		t.Fatalf("expected latest version to be rejected, got %+v", errs)
	}
	if !hasValidationError(errs, "dependencies.expo-camera") {
		t.Fatalf("expected wildcard version to be rejected, got %+v", errs)
	}
}

func TestValidateGeneratedExpoFilesRejectsBrowserOnlyAPIs(t *testing.T) {
	files := []SourceFile{{Path: "mobile/src/bad.ts", Content: "export const bad = window.location.href;", Language: "typescript", IsNew: true}}
	errs := ValidateGeneratedExpoFiles(files, map[string]string{"expo": "~55.0.0"}, DefaultNativeCapabilityRegistry())
	if !hasValidationError(errs, "mobile/src/bad.ts") {
		t.Fatalf("expected browser API validation error, got %+v", errs)
	}
}

func TestValidateGeneratedExpoFilesAllowsProcessEnvOnlyInAppConfig(t *testing.T) {
	files := []SourceFile{
		{Path: "mobile/app.config.ts", Content: "export default { extra: { apiBaseUrl: process.env.EXPO_PUBLIC_API_BASE_URL } };", Language: "typescript", IsNew: true},
		{Path: "mobile/src/api/client.ts", Content: "export const bad = process.env.EXPO_PUBLIC_API_BASE_URL;", Language: "typescript", IsNew: true},
	}
	errs := ValidateGeneratedExpoFiles(files, map[string]string{"expo": "~55.0.0"}, DefaultNativeCapabilityRegistry())
	if !hasValidationError(errs, "mobile/src/api/client.ts") {
		t.Fatalf("expected runtime process.env validation error, got %+v", errs)
	}
	if hasValidationError(errs, "mobile/app.config.ts") {
		t.Fatalf("expected app config process.env to be allowed, got %+v", errs)
	}
}

func TestGeneratedExpoProjectDependencyResolutionSmoke(t *testing.T) {
	if os.Getenv("MOBILE_GENERATOR_INSTALL_SMOKE") != "1" {
		t.Skip("set MOBILE_GENERATOR_INSTALL_SMOKE=1 to run generated Expo dependency resolution smoke")
	}

	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}

	root := t.TempDir()
	mobileRoot := filepath.Join(root, "mobile")
	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, file.Path)), 0o755); err != nil {
			t.Fatalf("create generated directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, file.Path), []byte(file.Content), 0o644); err != nil {
			t.Fatalf("write generated file %s: %v", file.Path, err)
		}
	}

	if os.Getenv("MOBILE_GENERATOR_FULL_INSTALL_SMOKE") != "1" {
		runGeneratedCommand(t, mobileRoot, "npm", "install", "--package-lock-only", "--ignore-scripts")
		return
	}

	runGeneratedCommand(t, mobileRoot, "npm", "install", "--package-lock=false", "--ignore-scripts")
	if os.Getenv("MOBILE_GENERATOR_FULL_INSTALL_SMOKE") == "1" {
		runGeneratedCommand(t, mobileRoot, "npm", "run", "typecheck")
	}
}

func runGeneratedCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(output))
	}
}

func hasSourceFile(files []SourceFile, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func findSourceFile(t *testing.T, files []SourceFile, path string) SourceFile {
	t.Helper()
	for _, file := range files {
		if file.Path == path {
			return file
		}
	}
	t.Fatalf("missing generated file %s; got %+v", path, sourceFilePaths(files))
	return SourceFile{}
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func hasScreenshotTarget(targets []StoreScreenshotTarget, platform string) bool {
	for _, target := range targets {
		if target.Platform == platform {
			return true
		}
	}
	return false
}

func hasMobileValidationCheck(report MobileValidationReport, id string, status MobileValidationStatus) bool {
	for _, check := range report.Checks {
		if check.ID == id && check.Status == status {
			return true
		}
	}
	return false
}

func removeSourceFile(files []SourceFile, path string) []SourceFile {
	filtered := make([]SourceFile, 0, len(files))
	for _, file := range files {
		if file.Path != path {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func modelFilesFromSource(files []SourceFile) []models.File {
	modelFiles := make([]models.File, 0, len(files))
	for _, file := range files {
		modelFiles = append(modelFiles, models.File{
			Path:    file.Path,
			Name:    filepath.Base(file.Path),
			Type:    "file",
			Content: file.Content,
			Size:    file.Size,
		})
	}
	return modelFiles
}

func modelsProjectForSpec(spec MobileAppSpec, releaseLevel string, buildStatus string) models.Project {
	return models.Project{
		Name:                spec.App.Name,
		Language:            "typescript",
		TargetPlatform:      string(TargetPlatformMobileExpo),
		MobileReleaseLevel:  releaseLevel,
		MobileBuildStatus:   buildStatus,
		AndroidPackage:      spec.Identity.AndroidPackage,
		IOSBundleIdentifier: spec.Identity.IOSBundleID,
	}
}

func sourceFilePaths(files []SourceFile) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	return paths
}
