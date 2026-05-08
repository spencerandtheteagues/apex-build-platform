package mobile

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"

	"apex-build/pkg/models"
)

type MobileValidationStatus string

const (
	MobileValidationPassed    MobileValidationStatus = "passed"
	MobileValidationWarning   MobileValidationStatus = "warning"
	MobileValidationFailed    MobileValidationStatus = "failed"
	MobileValidationNotMobile MobileValidationStatus = "not_mobile"
)

type MobileValidationReport struct {
	Status              MobileValidationStatus  `json:"status"`
	TargetPlatform      string                  `json:"target_platform,omitempty"`
	ReleaseLevel        string                  `json:"release_level,omitempty"`
	MobileBuildStatus   string                  `json:"mobile_build_status,omitempty"`
	StoreReadinessState string                  `json:"store_readiness_state,omitempty"`
	Summary             string                  `json:"summary"`
	Checks              []MobileValidationCheck `json:"checks"`
	MissingFiles        []string                `json:"missing_files,omitempty"`
	Warnings            []string                `json:"warnings,omitempty"`
	Errors              []string                `json:"errors,omitempty"`
}

type MobileValidationCheck struct {
	ID       string                 `json:"id"`
	Label    string                 `json:"label"`
	Status   MobileValidationStatus `json:"status"`
	Detail   string                 `json:"detail"`
	Required bool                   `json:"required"`
}

type packageJSONValidationShape struct {
	Scripts      map[string]string `json:"scripts"`
	Dependencies map[string]string `json:"dependencies"`
}

type apiContractManifestValidationShape struct {
	OpenAPI    string                             `json:"openapi"`
	Paths      map[string]map[string]apiOperation `json:"paths"`
	ApexMobile apiContractManifestMetadata        `json:"x-apex-mobile"`
}

type apiContractManifestMetadata struct {
	BackendMode BackendMode `json:"backend_mode"`
	Operations  []apiContractOperation
}

type apiOperation struct {
	OperationID  string `json:"operationId"`
	MobilePath   string `json:"x-mobile-path"`
	ResponseType string `json:"x-response-type"`
	MobileHelper string `json:"x-mobile-helper"`
	RequestType  string `json:"x-request-type"`
}

type apiContractOperation struct {
	Helper       string
	Method       string
	Path         string
	ResponseType string
}

func ValidateProjectSourcePackage(project models.Project, files []models.File) MobileValidationReport {
	report := MobileValidationReport{
		Status:            MobileValidationPassed,
		TargetPlatform:    strings.TrimSpace(project.TargetPlatform),
		ReleaseLevel:      firstNonEmptyString(project.MobileReleaseLevel, string(ReleaseSourceOnly)),
		MobileBuildStatus: firstNonEmptyString(project.MobileBuildStatus, "not_requested"),
		Summary:           "Expo mobile source package passed validation.",
		Checks:            []MobileValidationCheck{},
	}

	fileMap := mobileProjectFileMap(files)
	if !HasExplicitMobileExportMetadata(project) && len(fileMap) == 0 {
		report.Status = MobileValidationNotMobile
		report.Summary = "This project has no mobile metadata or mobile source package."
		report.Checks = append(report.Checks, MobileValidationCheck{
			ID:       "mobile_target",
			Label:    "Mobile target",
			Status:   MobileValidationNotMobile,
			Detail:   "No mobile target metadata was found.",
			Required: false,
		})
		return report
	}

	requiredFiles := []string{
		"mobile/package.json",
		"mobile/app.config.ts",
		"mobile/eas.json",
		"mobile/README.md",
		"mobile/BUILD.md",
		"mobile/STORE_RELEASE.md",
		"mobile/docs/api-contract.json",
		"mobile/docs/api-contract.md",
		"mobile/src/api/client.ts",
		"mobile/src/api/endpoints.ts",
		"mobile/src/api/types.ts",
		"mobile/src/permissions/nativeCapabilities.ts",
		"mobile/store/store-readiness.json",
		"mobile/store/privacy-data-safety.md",
		"mobile/store/screenshot-checklist.md",
		"mobile/store/release-notes.md",
	}
	missingFiles := missingRequiredFiles(fileMap, requiredFiles)
	report.MissingFiles = missingFiles
	if len(missingFiles) > 0 {
		addMobileCheck(&report, "required_files", "Required mobile files", MobileValidationFailed, "Missing "+strings.Join(missingFiles, ", "), true)
	} else {
		addMobileCheck(&report, "required_files", "Required mobile files", MobileValidationPassed, "All required Expo and store-readiness files are present.", true)
	}

	dependencies := validatePackageJSON(fileMap["mobile/package.json"], &report)
	validateAppConfig(project, fileMap["mobile/app.config.ts"], &report)
	validateSourcePolicy(files, dependencies, &report)
	manifestMeta := validateAPIContractManifest(fileMap["mobile/docs/api-contract.json"], &report)
	validateGeneratedBackendRoutes(fileMap, manifestMeta, &report)
	validateStoreReadiness(fileMap["mobile/store/store-readiness.json"], &report)
	validateReleaseTruth(project, &report)

	finalizeMobileValidationReport(&report)
	return report
}

func mobileProjectFileMap(files []models.File) map[string]models.File {
	fileMap := map[string]models.File{}
	for _, file := range files {
		if file.Type == "directory" {
			continue
		}
		path := normalizeProjectFilePath(file.Path)
		if strings.HasPrefix(path, "mobile/") ||
			strings.HasPrefix(path, "backend/") ||
			path == "docs/mobile-backend-routes.md" {
			fileMap[path] = file
		}
	}
	return fileMap
}

func missingRequiredFiles(fileMap map[string]models.File, requiredFiles []string) []string {
	var missing []string
	for _, path := range requiredFiles {
		if _, ok := fileMap[path]; !ok {
			missing = append(missing, path)
		}
	}
	return missing
}

func validatePackageJSON(file models.File, report *MobileValidationReport) map[string]string {
	if strings.TrimSpace(file.Path) == "" {
		return nil
	}

	var parsed packageJSONValidationShape
	if err := json.Unmarshal([]byte(file.Content), &parsed); err != nil {
		addMobileCheck(report, "package_json", "Expo package manifest", MobileValidationFailed, "package.json is not valid JSON.", true)
		return nil
	}

	var problems []string
	for _, script := range []string{"start", "web", "typecheck", "doctor"} {
		if strings.TrimSpace(parsed.Scripts[script]) == "" {
			problems = append(problems, "missing script "+script)
		}
	}
	for _, err := range ValidateExpoDependencyPolicy(parsed.Dependencies, DefaultNativeCapabilityRegistry()) {
		problems = append(problems, err.Field+": "+err.Message)
	}
	if len(problems) > 0 {
		addMobileCheck(report, "package_json", "Expo package manifest", MobileValidationFailed, strings.Join(problems, "; "), true)
		return parsed.Dependencies
	}

	addMobileCheck(report, "package_json", "Expo package manifest", MobileValidationPassed, "Dependencies are allowlisted and validation scripts are present.", true)
	return parsed.Dependencies
}

func validateAppConfig(project models.Project, file models.File, report *MobileValidationReport) {
	if strings.TrimSpace(file.Path) == "" {
		return
	}
	content := file.Content
	var problems []string
	for _, token := range []string{"newArchEnabled: true", "bundleIdentifier:", "package:", "permissions:", "plugins:"} {
		if !strings.Contains(content, token) {
			problems = append(problems, "missing "+token)
		}
	}
	if project.AndroidPackage != "" && !strings.Contains(content, project.AndroidPackage) {
		problems = append(problems, "Android package does not match project metadata")
	}
	if project.IOSBundleIdentifier != "" && !strings.Contains(content, project.IOSBundleIdentifier) {
		problems = append(problems, "iOS bundle identifier does not match project metadata")
	}
	if len(problems) > 0 {
		addMobileCheck(report, "app_config", "Expo app config", MobileValidationFailed, strings.Join(problems, "; "), true)
		return
	}
	addMobileCheck(report, "app_config", "Expo app config", MobileValidationPassed, "App identifiers, permissions, plugins, and New Architecture flag are present.", true)
}

func validateSourcePolicy(files []models.File, dependencies map[string]string, report *MobileValidationReport) {
	sourceFiles := make([]SourceFile, 0, len(files))
	for _, file := range files {
		path := normalizeProjectFilePath(file.Path)
		if !strings.HasPrefix(path, "mobile/") || file.Type == "directory" {
			continue
		}
		sourceFiles = append(sourceFiles, SourceFile{
			Path:     path,
			Content:  file.Content,
			Language: sourceLanguageForPath(path),
			Size:     file.Size,
			IsNew:    false,
		})
	}
	errs := ValidateGeneratedExpoFiles(sourceFiles, dependencies, DefaultNativeCapabilityRegistry())
	if len(errs) > 0 {
		messages := make([]string, 0, len(errs))
		for _, err := range errs {
			messages = append(messages, err.Field+": "+err.Message)
		}
		addMobileCheck(report, "source_policy", "Mobile source policy", MobileValidationFailed, strings.Join(messages, "; "), true)
		return
	}
	addMobileCheck(report, "source_policy", "Mobile source policy", MobileValidationPassed, "No browser-only APIs or unsupported dependencies were found in mobile source.", true)
}

func validateAPIContractManifest(file models.File, report *MobileValidationReport) apiContractManifestMetadata {
	if strings.TrimSpace(file.Path) == "" {
		return apiContractManifestMetadata{}
	}
	var manifest apiContractManifestValidationShape
	if err := json.Unmarshal([]byte(file.Content), &manifest); err != nil {
		addMobileCheck(report, "api_contract_manifest", "API contract manifest", MobileValidationFailed, "api-contract.json is not valid JSON.", true)
		return apiContractManifestMetadata{}
	}
	if manifest.OpenAPI != "3.1.0" {
		addMobileCheck(report, "api_contract_manifest", "API contract manifest", MobileValidationFailed, "api-contract.json must declare OpenAPI 3.1.0.", true)
		return manifest.ApexMobile
	}
	if len(manifest.Paths) == 0 {
		addMobileCheck(report, "api_contract_manifest", "API contract manifest", MobileValidationFailed, "api-contract.json must include at least one API path.", true)
		return manifest.ApexMobile
	}
	if strings.TrimSpace(string(manifest.ApexMobile.BackendMode)) == "" {
		addMobileCheck(report, "api_contract_manifest", "API contract manifest", MobileValidationFailed, "api-contract.json must include x-apex-mobile metadata.", true)
		return manifest.ApexMobile
	}
	manifest.ApexMobile.Operations = apiOperationsFromManifest(manifest.Paths)
	addMobileCheck(report, "api_contract_manifest", "API contract manifest", MobileValidationPassed, "OpenAPI-style mobile API contract manifest is present and parseable.", true)
	return manifest.ApexMobile
}

func apiOperationsFromManifest(paths map[string]map[string]apiOperation) []apiContractOperation {
	operations := make([]apiContractOperation, 0)
	pathKeys := make([]string, 0, len(paths))
	for path := range paths {
		pathKeys = append(pathKeys, path)
	}
	sort.Strings(pathKeys)
	for _, openAPIPath := range pathKeys {
		methods := paths[openAPIPath]
		methodKeys := make([]string, 0, len(methods))
		for method := range methods {
			methodKeys = append(methodKeys, method)
		}
		sort.Strings(methodKeys)
		for _, method := range methodKeys {
			operation := methods[method]
			helper := firstNonEmptyString(operation.MobileHelper, operation.OperationID)
			path := firstNonEmptyString(operation.MobilePath, openAPIPath)
			if helper == "" || path == "" {
				continue
			}
			operations = append(operations, apiContractOperation{
				Helper:       helper,
				Method:       strings.ToUpper(method),
				Path:         path,
				ResponseType: operation.ResponseType,
			})
		}
	}
	return operations
}

func validateGeneratedBackendRoutes(fileMap map[string]models.File, manifestMeta apiContractManifestMetadata, report *MobileValidationReport) {
	switch manifestMeta.BackendMode {
	case BackendNewGenerated, BackendExistingApexGenerated:
	default:
		addMobileCheck(report, "generated_backend_routes", "Generated backend route source", MobileValidationPassed, "No Apex-generated backend route source is required for this mobile backend mode.", false)
		return
	}

	requiredFiles := []string{
		"backend/package.json",
		"backend/tsconfig.json",
		"backend/src/server.ts",
		"backend/src/authAdapter.ts",
		"backend/src/persistenceAdapter.ts",
		"backend/src/uploadAdapter.ts",
		"backend/src/mobileContractRoutes.ts",
		"docs/mobile-backend-routes.md",
	}
	missing := missingRequiredFiles(fileMap, requiredFiles)
	if len(missing) > 0 {
		addMobileCheck(report, "generated_backend_routes", "Generated backend route source", MobileValidationFailed, "Missing "+strings.Join(missing, ", "), true)
		return
	}
	routes := fileMap["backend/src/mobileContractRoutes.ts"].Content
	if !strings.Contains(routes, "mobileContractRoutes") || !strings.Contains(routes, "handleContractRoute") {
		addMobileCheck(report, "generated_backend_routes", "Generated backend route source", MobileValidationFailed, "backend/src/mobileContractRoutes.ts is missing generated route registry code.", true)
		return
	}
	var drift []string
	for _, operation := range manifestMeta.Operations {
		for _, token := range []string{
			`"helper": "` + operation.Helper + `"`,
			`"method": "` + operation.Method + `"`,
			`"path": "` + operation.Path + `"`,
		} {
			if !strings.Contains(routes, token) {
				drift = append(drift, operation.Helper+" missing "+token)
			}
		}
	}
	if len(drift) > 0 {
		addMobileCheck(report, "generated_backend_routes", "Generated backend route source", MobileValidationFailed, "Backend route source does not match API contract manifest: "+strings.Join(drift, "; "), true)
		return
	}
	addMobileCheck(report, "generated_backend_routes", "Generated backend route source", MobileValidationPassed, "Generated backend route source is present and aligned to the API contract manifest.", true)
}

func validateStoreReadiness(file models.File, report *MobileValidationReport) {
	if strings.TrimSpace(file.Path) == "" {
		return
	}
	var pkg StoreReadinessPackage
	if err := json.Unmarshal([]byte(file.Content), &pkg); err != nil {
		addMobileCheck(report, "store_readiness", "Store-readiness package", MobileValidationFailed, "store-readiness.json is not valid JSON.", true)
		return
	}
	report.StoreReadinessState = pkg.Status
	if errs := ValidateStoreReadinessPackage(pkg); len(errs) > 0 {
		addMobileCheck(report, "store_readiness", "Store-readiness package", MobileValidationFailed, FormatValidationErrors(errs), true)
		return
	}
	if !truthNotesDisclaimApproval(pkg.TruthfulStatusNotes) {
		addMobileCheck(report, "store_readiness", "Store-readiness package", MobileValidationFailed, "Truthful status notes must state that store approval is not proven.", true)
		return
	}
	addMobileCheck(report, "store_readiness", "Store-readiness package", MobileValidationPassed, "Metadata, privacy draft, screenshot checklist, release notes, and manual prerequisites are present.", true)
}

func validateReleaseTruth(project models.Project, report *MobileValidationReport) {
	releaseLevel := MobileReleaseLevel(firstNonEmptyString(project.MobileReleaseLevel, string(ReleaseSourceOnly)))
	buildStatus := firstNonEmptyString(project.MobileBuildStatus, "not_requested")

	if releaseLevelRequiresNativeBuild(releaseLevel) && buildStatus != "succeeded" {
		addMobileCheck(report, "release_truth", "Release status truthfulness", MobileValidationFailed, "Native release level "+string(releaseLevel)+" requires a succeeded mobile build job before it can be claimed.", true)
		return
	}
	if releaseLevel == ReleaseStoreSubmissionReady && strings.TrimSpace(project.MobileStoreReadinessStatus) != "succeeded" {
		addMobileCheck(report, "release_truth", "Release status truthfulness", MobileValidationFailed, "Store-submission-ready requires a succeeded store readiness/submission workflow before it can be claimed.", true)
		return
	}
	addMobileCheck(report, "release_truth", "Release status truthfulness", MobileValidationPassed, "Current status is source/export readiness only; native builds and store submission remain separate workflows.", true)
}

func finalizeMobileValidationReport(report *MobileValidationReport) {
	hasWarning := false
	for _, check := range report.Checks {
		switch check.Status {
		case MobileValidationFailed:
			report.Status = MobileValidationFailed
			report.Errors = append(report.Errors, check.Detail)
		case MobileValidationWarning:
			hasWarning = true
			report.Warnings = append(report.Warnings, check.Detail)
		}
	}
	if report.Status == MobileValidationFailed {
		report.Summary = "Mobile source package failed validation."
		return
	}
	if hasWarning {
		report.Status = MobileValidationWarning
		report.Summary = "Mobile source package passed with warnings."
		return
	}
	report.Status = MobileValidationPassed
	report.Summary = "Mobile source package passed validation."
}

func addMobileCheck(report *MobileValidationReport, id, label string, status MobileValidationStatus, detail string, required bool) {
	report.Checks = append(report.Checks, MobileValidationCheck{
		ID:       id,
		Label:    label,
		Status:   status,
		Detail:   detail,
		Required: required,
	})
}

func releaseLevelRequiresNativeBuild(level MobileReleaseLevel) bool {
	switch level {
	case ReleaseDevBuild, ReleaseInternalAndroidAPK, ReleaseAndroidAAB, ReleaseIOSSimulator, ReleaseIOSInternal, ReleaseTestFlightReady, ReleaseStoreSubmissionReady:
		return true
	default:
		return false
	}
}

func truthNotesDisclaimApproval(notes []string) bool {
	joined := strings.ToLower(strings.Join(notes, " "))
	return strings.Contains(joined, "not proof") || strings.Contains(joined, "not approved")
}

func normalizeProjectFilePath(path string) string {
	return filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(path), "/"))
}

func sourceLanguageForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	default:
		return "text"
	}
}
