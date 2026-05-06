package mobile

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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

func sourceFilePaths(files []SourceFile) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	return paths
}
