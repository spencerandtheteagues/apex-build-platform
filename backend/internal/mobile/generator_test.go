package mobile

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

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
		!hasSourceFile(files, "mobile/docs/api-contract.json") ||
		!hasSourceFile(files, "mobile/docs/api-contract.md") ||
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

	for _, expected := range []string{
		"json?: unknown",
		"auth?: boolean",
		"buildPath",
		"SecureStore.getItemAsync('auth_token')",
		"public code?: string",
		"export function isApiError",
		"extractAPIErrorCode(details)",
		"new ApiError(message, response.status, details, code)",
	} {
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

func TestGenerateExpoProjectCreatesAPIContractManifest(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}

	manifestFile := findSourceFile(t, files, "mobile/docs/api-contract.json")
	markdownFile := findSourceFile(t, files, "mobile/docs/api-contract.md")
	var manifest map[string]any
	if err := json.Unmarshal([]byte(manifestFile.Content), &manifest); err != nil {
		t.Fatalf("api contract manifest must be valid JSON: %v\n%s", err, manifestFile.Content)
	}
	if manifest["openapi"] != "3.1.0" {
		t.Fatalf("expected OpenAPI 3.1 manifest, got %+v", manifest["openapi"])
	}
	paths, ok := manifest["paths"].(map[string]any)
	if !ok {
		t.Fatalf("expected paths object, got %+v", manifest["paths"])
	}
	if _, ok := paths["/api/jobs/{id}/photos"]; !ok {
		t.Fatalf("expected OpenAPI path conversion for upload endpoint, got %+v", paths)
	}
	for _, expected := range []string{
		`"operationId": "uploadJobPhoto"`,
		`"x-mobile-path": "/api/jobs/:id/photos"`,
		`"token_storage": "expo-secure-store:auth_token"`,
	} {
		if !strings.Contains(manifestFile.Content, expected) {
			t.Fatalf("manifest missing %q\n%s", expected, manifestFile.Content)
		}
	}
	if !strings.Contains(markdownFile.Content, "| `uploadJobPhoto` | `POST` | `/api/jobs/:id/photos` | yes | `FormData` | `PhotoAsset` |") {
		t.Fatalf("markdown contract missing upload row\n%s", markdownFile.Content)
	}
}

func TestGenerateExpoProjectCreatesContractDrivenBackendRoutes(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}

	for _, path := range []string{
		"backend/package.json",
		"backend/tsconfig.json",
		"backend/src/server.ts",
		"backend/src/authAdapter.ts",
		"backend/src/persistenceAdapter.ts",
		"backend/src/uploadAdapter.ts",
		"backend/src/mobileContractRoutes.ts",
		"docs/mobile-backend-routes.md",
	} {
		if !hasSourceFile(files, path) {
			t.Fatalf("expected generated backend contract file %s; got %+v", path, sourceFilePaths(files))
		}
	}

	routes := findSourceFile(t, files, "backend/src/mobileContractRoutes.ts")
	for _, expected := range []string{
		`"helper": "uploadJobPhoto"`,
		`"path": "/api/jobs/:id/photos"`,
		`"requestType": "EstimateDraft"`,
		`"bodyMode": "multipart"`,
		"requestType: string;",
		"issueDemoSession(loginEmailFromBody(body))",
		"shouldPersistContractMutation(definition)",
		"persistContractMutation(definition, ctx)",
		"mobileStore.upsert(typeName, record)",
		"deleteContractRecord(definition, ctx)",
		"routeHTTPResponse(404",
		"recordUpload(ctx.params, ctx.rawBody, ctx.headers)",
		"createPhotoAsset",
		"responseForType(definition.responseType, ctx)",
	} {
		if !strings.Contains(routes.Content, expected) {
			t.Fatalf("expected generated backend routes to contain %q\n%s", expected, routes.Content)
		}
	}

	server := findSourceFile(t, files, "backend/src/server.ts")
	for _, expected := range []string{"authenticateMobileRequest", "mobileContractRoutes", "Missing bearer token", "MOBILE_API_ALLOWED_ORIGIN", "'/healthz'", "INVALID_JSON_BODY", "writeRouteResult"} {
		if !strings.Contains(server.Content, expected) {
			t.Fatalf("expected generated backend server to contain %q\n%s", expected, server.Content)
		}
	}

	auth := findSourceFile(t, files, "backend/src/authAdapter.ts")
	for _, expected := range []string{"issueDemoSession", "authenticateMobileRequest", "APEX_MOBILE_DEMO_TOKEN"} {
		if !strings.Contains(auth.Content, expected) {
			t.Fatalf("expected generated auth adapter to contain %q\n%s", expected, auth.Content)
		}
	}

	persistence := findSourceFile(t, files, "backend/src/persistenceAdapter.ts")
	for _, expected := range []string{"InMemoryMobileStore", "upsert", "get(typeName: string, id: string)", "remove(typeName: string, id: string)", "const seedData"} {
		if !strings.Contains(persistence.Content, expected) {
			t.Fatalf("expected generated persistence adapter to contain %q\n%s", expected, persistence.Content)
		}
	}

	uploads := findSourceFile(t, files, "backend/src/uploadAdapter.ts")
	for _, expected := range []string{"recordUpload", "mobileStore.append('PhotoAsset'", "byteLength"} {
		if !strings.Contains(uploads.Content, expected) {
			t.Fatalf("expected generated upload adapter to contain %q\n%s", expected, uploads.Content)
		}
	}

	docs := findSourceFile(t, files, "docs/mobile-backend-routes.md")
	if !strings.Contains(docs.Content, "| `uploadJobPhoto` | `POST` | `/api/jobs/:id/photos` | yes | multipart | `PhotoAsset` |") {
		t.Fatalf("expected generated backend docs to include upload route\n%s", docs.Content)
	}
}

func TestGenerateExpoProjectSkipsBackendRoutesForLocalOnlyMobileSpec(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	spec.Architecture.BackendMode = BackendLocalOnly
	spec.Architecture.DatabaseMode = DatabaseLocalSQLite
	spec.Architecture.AuthMode = AuthNone
	spec.APIContracts = nil

	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated local-only files, got errors %+v", errs)
	}
	if hasSourceFile(files, "backend/src/mobileContractRoutes.ts") {
		t.Fatalf("expected local-only mobile app to omit generated backend source")
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
	if !hasMobileValidationCheck(report, "api_contract_manifest", MobileValidationPassed) {
		t.Fatalf("expected API contract manifest check to pass, got %+v", report.Checks)
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

func TestValidateProjectSourcePackageRequiresAPIContractManifest(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}
	files = removeSourceFile(files, "mobile/docs/api-contract.json")

	report := ValidateProjectSourcePackage(modelsProjectForSpec(spec, string(ReleaseSourceOnly), "not_requested"), modelFilesFromSource(files))
	if report.Status != MobileValidationFailed {
		t.Fatalf("expected validation failed, got %+v", report)
	}
	if !stringSliceContains(report.MissingFiles, "mobile/docs/api-contract.json") {
		t.Fatalf("expected missing api-contract.json, got %+v", report.MissingFiles)
	}
}

func TestValidateProjectSourcePackageRequiresGeneratedBackendRoutesForFullStackSpec(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}
	files = removeSourceFile(files, "backend/src/mobileContractRoutes.ts")

	report := ValidateProjectSourcePackage(modelsProjectForSpec(spec, string(ReleaseSourceOnly), "not_requested"), modelFilesFromSource(files))
	if report.Status != MobileValidationFailed {
		t.Fatalf("expected validation failed, got %+v", report)
	}
	if !hasMobileValidationCheck(report, "generated_backend_routes", MobileValidationFailed) {
		t.Fatalf("expected generated backend routes failure, got %+v", report.Checks)
	}
}

func TestValidateProjectSourcePackageRejectsBackendRouteManifestDrift(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}
	routeFile := findSourceFile(t, files, "backend/src/mobileContractRoutes.ts")
	files = replaceSourceFileContent(files, "backend/src/mobileContractRoutes.ts", strings.ReplaceAll(routeFile.Content, `"path": "/api/jobs/:id/photos"`, `"path": "/api/jobs/:id/photo-upload"`))

	report := ValidateProjectSourcePackage(modelsProjectForSpec(spec, string(ReleaseSourceOnly), "not_requested"), modelFilesFromSource(files))
	if report.Status != MobileValidationFailed {
		t.Fatalf("expected validation failed, got %+v", report)
	}
	if !hasMobileValidationCheck(report, "generated_backend_routes", MobileValidationFailed) {
		t.Fatalf("expected generated backend routes drift failure, got %+v", report.Checks)
	}
}

func TestValidateProjectSourcePackageDoesNotRequireGeneratedBackendRoutesForLocalOnlySpec(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	spec.Architecture.BackendMode = BackendLocalOnly
	spec.Architecture.DatabaseMode = DatabaseLocalSQLite
	spec.Architecture.AuthMode = AuthNone
	spec.APIContracts = nil

	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}

	report := ValidateProjectSourcePackage(modelsProjectForSpec(spec, string(ReleaseSourceOnly), "not_requested"), modelFilesFromSource(files))
	if report.Status != MobileValidationPassed {
		t.Fatalf("expected local-only validation passed without backend routes, got %+v", report)
	}
}

func TestValidateProjectSourcePackageRejectsInvalidAPIContractManifest(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}
	files = replaceSourceFileContent(files, "mobile/docs/api-contract.json", `{"openapi":"3.0.0","paths":{}}`)

	report := ValidateProjectSourcePackage(modelsProjectForSpec(spec, string(ReleaseSourceOnly), "not_requested"), modelFilesFromSource(files))
	if report.Status != MobileValidationFailed {
		t.Fatalf("expected validation failed, got %+v", report)
	}
	if !hasMobileValidationCheck(report, "api_contract_manifest", MobileValidationFailed) {
		t.Fatalf("expected API contract manifest failure, got %+v", report.Checks)
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

func TestValidateGeneratedExpoFilesIgnoresGeneratedBackendProcessEnv(t *testing.T) {
	files := []SourceFile{
		{Path: "backend/src/server.ts", Content: "const port = process.env.PORT ?? '8080';", Language: "typescript", IsNew: true},
	}
	errs := ValidateGeneratedExpoFiles(files, map[string]string{"expo": "~55.0.0"}, DefaultNativeCapabilityRegistry())
	if hasValidationError(errs, "backend/src/server.ts") {
		t.Fatalf("expected generated backend process.env to be outside mobile browser-only policy, got %+v", errs)
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

func TestGeneratedMobileBackendTypecheckSmoke(t *testing.T) {
	if os.Getenv("MOBILE_BACKEND_TYPECHECK_SMOKE") != "1" {
		t.Skip("set MOBILE_BACKEND_TYPECHECK_SMOKE=1 to run generated backend TypeScript smoke")
	}

	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}

	root := t.TempDir()
	backendRoot := filepath.Join(root, "backend")
	for _, file := range files {
		if !strings.HasPrefix(file.Path, "backend/") {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("create generated backend directory: %v", err)
		}
		if err := os.WriteFile(target, []byte(file.Content), 0o644); err != nil {
			t.Fatalf("write generated backend file %s: %v", file.Path, err)
		}
	}

	runGeneratedCommand(t, backendRoot, "npm", "install", "--package-lock=false", "--ignore-scripts")
	runGeneratedCommand(t, backendRoot, "npm", "run", "typecheck")
}

func TestGeneratedMobileBackendRuntimeSmoke(t *testing.T) {
	if os.Getenv("MOBILE_BACKEND_RUNTIME_SMOKE") != "1" {
		t.Skip("set MOBILE_BACKEND_RUNTIME_SMOKE=1 to run generated backend runtime smoke")
	}

	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}

	root := t.TempDir()
	backendRoot := filepath.Join(root, "backend")
	for _, file := range files {
		if !strings.HasPrefix(file.Path, "backend/") {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("create generated backend directory: %v", err)
		}
		if err := os.WriteFile(target, []byte(file.Content), 0o644); err != nil {
			t.Fatalf("write generated backend file %s: %v", file.Path, err)
		}
	}

	runGeneratedCommand(t, backendRoot, "npm", "install", "--package-lock=false", "--ignore-scripts")
	script := `
import { mobileContractRoutes } from './src/mobileContractRoutes.ts';
async function main() {
  const login = mobileContractRoutes.find((route) => route.path === '/api/auth/login');
  const listJobs = mobileContractRoutes.find((route) => route.path === '/api/jobs');
  const syncEstimate = mobileContractRoutes.find((route) => route.path === '/api/estimates/sync');
  const upload = mobileContractRoutes.find((route) => route.path === '/api/jobs/:id/photos');
  if (!login || !listJobs || !syncEstimate || !upload) throw new Error('missing generated contract routes');
  const session = await login.handle({ params: {}, body: { email: 'smoke@example.test', password: 'pw' }, rawBody: Buffer.alloc(0), headers: {}, auth: null });
  if (!session?.token || session.user.email !== 'smoke@example.test') throw new Error('login route did not issue session');
  const jobs = await listJobs.handle({ params: {}, body: undefined, rawBody: Buffer.alloc(0), headers: {}, auth: session });
  if (!Array.isArray(jobs) || jobs.length === 0) throw new Error('jobs route did not return seeded records');
  const estimate = await syncEstimate.handle({ params: {}, body: { id: 'estimate-smoke', laborHours: 2 }, rawBody: Buffer.alloc(0), headers: {}, auth: session });
  if (estimate?.id !== 'estimate-smoke' || estimate.laborHours !== 2 || estimate.finalPrice !== 1170) {
    throw new Error('estimate mutation did not persist and enrich contract record');
  }
  const params = upload.match('/api/jobs/job-101/photos');
  if (!params || params.id !== 'job-101') throw new Error('upload route path matcher failed');
  const photo = await upload.handle({ params, body: undefined, rawBody: Buffer.from('image'), headers: { 'content-type': 'image/jpeg' }, auth: session });
  if (!photo?.url || photo.jobId !== 'job-101' || photo.byteLength !== 5) throw new Error('upload route did not record metadata');
}
main().catch((error) => {
  console.error(error);
  process.exit(1);
});
`
	runGeneratedCommand(t, backendRoot, "npx", "tsx", "--eval", script)
}

func TestGeneratedMobileBackendHTTPSmoke(t *testing.T) {
	if os.Getenv("MOBILE_BACKEND_HTTP_SMOKE") != "1" {
		t.Skip("set MOBILE_BACKEND_HTTP_SMOKE=1 to run generated backend HTTP smoke")
	}

	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}

	root := t.TempDir()
	backendRoot := filepath.Join(root, "backend")
	for _, file := range files {
		if !strings.HasPrefix(file.Path, "backend/") {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("create generated backend directory: %v", err)
		}
		if err := os.WriteFile(target, []byte(file.Content), 0o644); err != nil {
			t.Fatalf("write generated backend file %s: %v", file.Path, err)
		}
	}

	runGeneratedCommand(t, backendRoot, "npm", "install", "--package-lock=false", "--ignore-scripts")

	port := freeTCPPort(t)
	baseURL := "http://127.0.0.1:" + port
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "npx", "tsx", "src/server.ts")
	cmd.Dir = backendRoot
	cmd.Env = append(os.Environ(),
		"PORT="+port,
		"APEX_MOBILE_DEMO_TOKEN=smoke-token",
		"MOBILE_API_ALLOWED_ORIGIN=https://mobile.example.test",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatalf("start generated backend server: %v", err)
	}
	defer terminateGeneratedBackendProcess(t, cancel, cmd)

	client := &http.Client{Timeout: 2 * time.Second}
	waitForGeneratedBackendHTTP(t, client, baseURL, &output)

	status, headers, body := generatedHTTPRequest(t, client, http.MethodPost, baseURL+"/api/auth/login", "", "application/json", `{"email":"http-smoke@example.test","password":"pw"}`)
	if status != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", status, body)
	}
	if headers.Get("Access-Control-Allow-Origin") != "https://mobile.example.test" {
		t.Fatalf("expected configured CORS origin, got %q", headers.Get("Access-Control-Allow-Origin"))
	}
	var session struct {
		Token string `json:"token"`
		User  struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &session); err != nil {
		t.Fatalf("decode login response: %v\n%s", err, body)
	}
	if session.Token != "smoke-token" || session.User.Email != "http-smoke@example.test" {
		t.Fatalf("unexpected login session: %+v", session)
	}

	status, _, body = generatedHTTPRequest(t, client, http.MethodGet, baseURL+"/api/jobs", "", "", "")
	if status != http.StatusUnauthorized {
		t.Fatalf("expected auth-gated jobs 401 without token, got %d: %s", status, body)
	}

	status, _, body = generatedHTTPRequest(t, client, http.MethodGet, baseURL+"/api/jobs", session.Token, "", "")
	if status != http.StatusOK {
		t.Fatalf("expected jobs 200, got %d: %s", status, body)
	}
	var jobs []map[string]any
	if err := json.Unmarshal(body, &jobs); err != nil {
		t.Fatalf("decode jobs response: %v\n%s", err, body)
	}
	if len(jobs) == 0 {
		t.Fatalf("expected seeded jobs, got %s", body)
	}

	status, _, body = generatedHTTPRequest(t, client, http.MethodPost, baseURL+"/api/estimates/sync", session.Token, "application/json", `{"id":"estimate-http","laborHours":2}`)
	if status != http.StatusOK {
		t.Fatalf("expected estimate sync 200, got %d: %s", status, body)
	}
	var estimate map[string]any
	if err := json.Unmarshal(body, &estimate); err != nil {
		t.Fatalf("decode estimate response: %v\n%s", err, body)
	}
	if estimate["id"] != "estimate-http" || estimate["laborHours"] != float64(2) || estimate["finalPrice"] != float64(1170) {
		t.Fatalf("expected persisted/enriched estimate, got %+v", estimate)
	}

	status, _, body = generatedHTTPRequest(t, client, http.MethodPost, baseURL+"/api/estimates/sync", session.Token, "application/json", `{"id":`)
	if status != http.StatusBadRequest || !strings.Contains(string(body), "INVALID_JSON_BODY") {
		t.Fatalf("expected invalid JSON 400, got %d: %s", status, body)
	}

	status, _, body = generatedHTTPRequest(t, client, http.MethodGet, baseURL+"/healthz", "", "", "")
	if status != http.StatusOK || !strings.Contains(string(body), "apex-generated-mobile-backend") {
		t.Fatalf("expected healthz 200, got %d: %s", status, body)
	}

	status, _, body = generatedHTTPRequest(t, client, http.MethodPost, baseURL+"/api/jobs/job-http/photos", session.Token, "image/jpeg", "image")
	if status != http.StatusOK {
		t.Fatalf("expected upload 200, got %d: %s", status, body)
	}
	var photo map[string]any
	if err := json.Unmarshal(body, &photo); err != nil {
		t.Fatalf("decode photo response: %v\n%s", err, body)
	}
	if photo["jobId"] != "job-http" || photo["byteLength"] != float64(5) || photo["contentType"] != "image/jpeg" {
		t.Fatalf("expected recorded upload metadata, got %+v", photo)
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

func terminateGeneratedBackendProcess(t *testing.T, cancel context.CancelFunc, cmd *exec.Cmd) {
	t.Helper()
	cancel()
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-done:
		return
	case <-time.After(2 * time.Second):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
	}
}

func freeTCPPort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve generated backend smoke port: %v", err)
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP listener address, got %T", listener.Addr())
	}
	return strconv.Itoa(addr.Port)
}

func waitForGeneratedBackendHTTP(t *testing.T, client *http.Client, baseURL string, output *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/__apex_probe")
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("generated backend server did not become reachable\n%s", output.String())
}

func generatedHTTPRequest(t *testing.T, client *http.Client, method string, url string, token string, contentType string, payload string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("create generated backend request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send generated backend request %s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read generated backend response: %v", err)
	}
	return resp.StatusCode, resp.Header.Clone(), body
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

func replaceSourceFileContent(files []SourceFile, path string, content string) []SourceFile {
	updated := make([]SourceFile, 0, len(files))
	for _, file := range files {
		if file.Path == path {
			file.Content = content
			file.Size = int64(len(content))
		}
		updated = append(updated, file)
	}
	return updated
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
