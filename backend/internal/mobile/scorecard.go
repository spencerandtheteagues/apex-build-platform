package mobile

import (
	"fmt"
	"math"
	"strings"

	"apex-build/pkg/models"
)

type MobileReadinessCategoryStatus string

const (
	MobileReadinessComplete      MobileReadinessCategoryStatus = "complete"
	MobileReadinessPartial       MobileReadinessCategoryStatus = "partial"
	MobileReadinessBlocked       MobileReadinessCategoryStatus = "blocked"
	MobileReadinessNotApplicable MobileReadinessCategoryStatus = "not_applicable"
)

type MobileReadinessScorecard struct {
	OverallScore int                       `json:"overall_score"`
	TargetScore  int                       `json:"target_score"`
	IsReady      bool                      `json:"is_ready"`
	Summary      string                    `json:"summary"`
	Categories   []MobileReadinessCategory `json:"categories"`
	Blockers     []string                  `json:"blockers,omitempty"`
	NextActions  []string                  `json:"next_actions,omitempty"`
}

type MobileReadinessCategory struct {
	ID       string                        `json:"id"`
	Label    string                        `json:"label"`
	Score    int                           `json:"score"`
	Target   int                           `json:"target"`
	Status   MobileReadinessCategoryStatus `json:"status"`
	Summary  string                        `json:"summary"`
	Evidence []string                      `json:"evidence,omitempty"`
	Blockers []string                      `json:"blockers,omitempty"`
}

func BuildMobileReadinessScorecard(project models.Project, files []models.File, validation MobileValidationReport) MobileReadinessScorecard {
	const targetScore = 95
	fileMap := mobileFileMap(files)
	categories := []MobileReadinessCategory{
		scoreTargetMetadata(project, fileMap, targetScore),
		scoreSourceGeneration(fileMap, validation, targetScore),
		scoreSourceExportPackage(fileMap, validation, targetScore),
		scoreSourceValidation(validation, targetScore),
		scoreBuildOrchestration(project, targetScore),
		scoreNativeArtifacts(project, targetScore),
		scoreCredentialsAndSigning(project, targetScore),
		scoreStoreReadiness(project, validation, targetScore),
	}

	total := 0
	ready := true
	var blockers []string
	var nextActions []string
	for _, category := range categories {
		total += category.Score
		if category.Score < targetScore && category.Status != MobileReadinessNotApplicable {
			ready = false
		}
		for _, blocker := range category.Blockers {
			if blocker == "" {
				continue
			}
			blockers = appendUniqueString(blockers, blocker)
			nextActions = appendUniqueString(nextActions, blocker)
		}
	}

	overall := 0
	if len(categories) > 0 {
		overall = int(math.Round(float64(total) / float64(len(categories))))
	}

	summary := fmt.Sprintf("Mobile readiness is %d%% toward the 95%% launch-readiness target.", overall)
	if ready {
		summary = "Mobile readiness meets the 95% launch-readiness target for source, validation, native artifacts, credentials, and store package evidence."
	}

	return MobileReadinessScorecard{
		OverallScore: clampScore(overall),
		TargetScore:  targetScore,
		IsReady:      ready,
		Summary:      summary,
		Categories:   categories,
		Blockers:     blockers,
		NextActions:  nextActions,
	}
}

func scoreTargetMetadata(project models.Project, fileMap map[string]models.File, target int) MobileReadinessCategory {
	score := 0
	var evidence []string
	var blockers []string

	if strings.TrimSpace(project.TargetPlatform) == string(TargetPlatformMobileExpo) {
		score += 25
		evidence = append(evidence, "Project target is mobile_expo.")
	} else if HasExplicitMobileExportMetadata(project) || len(fileMap) > 0 {
		score += 15
		evidence = append(evidence, "Mobile metadata or source files are present.")
	} else {
		return readinessCategory("target_metadata", "Target metadata", 0, target, "No first-class mobile target metadata found.", nil, []string{"Select Mobile App / Expo target before claiming Android or iOS readiness."})
	}

	if strings.TrimSpace(project.MobileFramework) == string(MobileFrameworkExpoReactNative) || strings.Contains(fileMap["mobile/package.json"].Content, "\"expo\"") {
		score += 20
		evidence = append(evidence, "Expo/React Native framework evidence found.")
	} else {
		blockers = append(blockers, "Set mobile framework to Expo/React Native.")
	}

	platforms := normalizedProjectPlatformSet(project)
	if platforms[string(MobilePlatformAndroid)] || strings.Contains(fileMap["mobile/app.config.ts"].Content, "package:") {
		score += 15
		evidence = append(evidence, "Android target evidence found.")
	} else {
		blockers = append(blockers, "Add Android platform metadata and package identifier.")
	}
	if platforms[string(MobilePlatformIOS)] || strings.Contains(fileMap["mobile/app.config.ts"].Content, "bundleIdentifier:") {
		score += 15
		evidence = append(evidence, "iOS target evidence found.")
	} else {
		blockers = append(blockers, "Add iOS platform metadata and bundle identifier.")
	}

	if strings.TrimSpace(project.MobileReleaseLevel) != "" {
		score += 10
		evidence = append(evidence, "Mobile release level is tracked separately.")
	} else {
		blockers = append(blockers, "Track a separate mobile release level.")
	}
	if strings.TrimSpace(project.GeneratedMobileClientPath) != "" || len(fileMap) > 0 {
		score += 15
		evidence = append(evidence, "Generated mobile client path/source exists.")
	} else {
		blockers = append(blockers, "Generate the Expo mobile client under mobile/.")
	}

	return readinessCategory("target_metadata", "Target metadata", score, target, "Mobile target metadata and platform identity evidence.", evidence, blockers)
}

func scoreSourceGeneration(fileMap map[string]models.File, validation MobileValidationReport, target int) MobileReadinessCategory {
	required := requiredMobileSourceFiles()
	present := 0
	for _, path := range required {
		if _, ok := fileMap[path]; ok {
			present++
		}
	}
	score := 0
	if len(required) > 0 {
		score = int(math.Round(float64(present) / float64(len(required)) * 100))
	}
	var blockers []string
	if present < len(required) {
		blockers = append(blockers, "Generate all required Expo source, config, asset, and store-readiness files.")
	}
	evidence := []string{fmt.Sprintf("%d of %d required mobile source files are present.", present, len(required))}
	if validation.Status == MobileValidationPassed {
		evidence = append(evidence, "Source validation passed.")
	}
	return readinessCategory("source_generation", "Expo source generation", score, target, "Generated Expo project file coverage.", evidence, blockers)
}

func scoreSourceExportPackage(fileMap map[string]models.File, validation MobileValidationReport, target int) MobileReadinessCategory {
	exportPaths := []string{"mobile/package.json", "mobile/app.config.ts", "mobile/eas.json", "mobile/README.md", "mobile/BUILD.md", "mobile/STORE_RELEASE.md"}
	present := 0
	for _, path := range exportPaths {
		if _, ok := fileMap[path]; ok {
			present++
		}
	}
	score := 0
	if len(exportPaths) > 0 {
		score = int(math.Round(float64(present) / float64(len(exportPaths)) * 100))
	}
	if validation.Status == MobileValidationPassed && score >= target {
		score = 100
	}
	var blockers []string
	if score < target {
		blockers = append(blockers, "Make the mobile/ source package export-complete for ZIP and GitHub export.")
	}
	return readinessCategory("source_export", "Source export package", score, target, "ZIP/GitHub export package completeness.", []string{fmt.Sprintf("%d of %d export-critical files are present.", present, len(exportPaths))}, blockers)
}

func scoreSourceValidation(validation MobileValidationReport, target int) MobileReadinessCategory {
	score := 0
	var blockers []string
	switch validation.Status {
	case MobileValidationPassed:
		score = 100
	case MobileValidationWarning:
		score = 75
		blockers = append(blockers, "Clear mobile validation warnings.")
	case MobileValidationFailed:
		score = 25
		blockers = append(blockers, "Fix mobile source validation failures before native builds.")
	default:
		blockers = append(blockers, "Run mobile source validation.")
	}
	return readinessCategory("source_validation", "Source validation", score, target, validation.Summary, validationCheckEvidence(validation), blockers)
}

func scoreBuildOrchestration(project models.Project, target int) MobileReadinessCategory {
	status := strings.TrimSpace(project.MobileBuildStatus)
	score := 35
	var blockers []string
	var evidence []string
	switch status {
	case string(MobileBuildSucceeded):
		score = 100
		evidence = append(evidence, "Project has a succeeded mobile build status.")
	case string(MobileBuildQueued), string(MobileBuildPreparing), string(MobileBuildValidating), string(MobileBuildUploading), string(MobileBuildBuilding), string(MobileBuildSigning):
		score = 65
		evidence = append(evidence, "A native build job is in progress.")
		blockers = append(blockers, "Wait for mobile build job to reach succeeded.")
	case string(MobileBuildFailed):
		score = 40
		evidence = append(evidence, "Last native build failed.")
		blockers = append(blockers, "Repair and rerun the failed mobile build job.")
	default:
		evidence = append(evidence, "Internal build-service foundation exists, but no persisted native build job has succeeded for this project.")
		blockers = append(blockers, "Add persistent EAS build jobs and run Android/iOS native builds.")
	}
	return readinessCategory("build_orchestration", "Build orchestration", score, target, "Native build workflow status.", evidence, blockers)
}

func scoreNativeArtifacts(project models.Project, target int) MobileReadinessCategory {
	status := strings.TrimSpace(project.MobileBuildStatus)
	artifacts := mobileMetadataStringList(project.MobileMetadata, "artifact_urls")
	hasAndroidArtifact := mobileMetadataHasAny(project.MobileMetadata, "android_artifact_url", "android_aab_url", "android_apk_url")
	hasIOSArtifact := mobileMetadataHasAny(project.MobileMetadata, "ios_artifact_url", "ios_simulator_artifact_url")
	if len(artifacts) > 0 {
		hasAndroidArtifact = hasAndroidArtifact || strings.Contains(strings.ToLower(strings.Join(artifacts, " ")), "android")
		hasIOSArtifact = hasIOSArtifact || strings.Contains(strings.ToLower(strings.Join(artifacts, " ")), "ios")
	}

	score := 0
	var evidence []string
	var blockers []string
	platforms := normalizedProjectPlatformSet(project)
	if !platforms[string(MobilePlatformAndroid)] && !platforms[string(MobilePlatformIOS)] {
		platforms[string(MobilePlatformAndroid)] = true
		platforms[string(MobilePlatformIOS)] = true
	}
	if platforms[string(MobilePlatformAndroid)] {
		if hasAndroidArtifact && status == string(MobileBuildSucceeded) {
			score += 50
			evidence = append(evidence, "Android artifact evidence found.")
		} else {
			blockers = append(blockers, "Produce a signed Android APK/AAB artifact through EAS Build.")
		}
	}
	if platforms[string(MobilePlatformIOS)] {
		if hasIOSArtifact && status == string(MobileBuildSucceeded) {
			score += 50
			evidence = append(evidence, "iOS artifact evidence found.")
		} else {
			blockers = append(blockers, "Produce an iOS simulator/internal artifact through EAS Build.")
		}
	}
	if status == string(MobileBuildSucceeded) && len(blockers) == 0 {
		score = 100
	}
	return readinessCategory("native_artifacts", "Native artifacts", score, target, "Signed Android/iOS artifact proof.", evidence, blockers)
}

func scoreCredentialsAndSigning(project models.Project, target int) MobileReadinessCategory {
	if mobileMetadataBool(project.MobileMetadata, "credentials_validated") ||
		strings.EqualFold(mobileMetadataString(project.MobileMetadata, "credential_status"), "validated") {
		return readinessCategory("credentials_signing", "Credentials and signing", 100, target, "User-provided build credentials are validated.", []string{"Credential metadata reports validated state."}, nil)
	}
	if strings.EqualFold(mobileMetadataString(project.MobileMetadata, "credential_status"), "partial") {
		missing := mobileMetadataStringList(project.MobileMetadata, "mobile_credential_missing")
		blockers := []string{"Add the missing mobile credentials before native EAS builds can reach the 95% target."}
		if len(missing) > 0 {
			blockers = []string{"Add missing mobile credentials: " + strings.Join(missing, ", ") + "."}
		}
		return readinessCategory("credentials_signing", "Credentials and signing", 50, target, "Some required mobile credentials are stored, but the platform credential set is incomplete.", []string{"Credential metadata reports partial state."}, blockers)
	}
	return readinessCategory("credentials_signing", "Credentials and signing", 0, target, "No validated EAS/Apple/Google signing credentials are recorded.", nil, []string{"Add encrypted user-provided mobile credential vault and validate EAS/Apple/Google signing prerequisites."})
}

func scoreStoreReadiness(project models.Project, validation MobileValidationReport, target int) MobileReadinessCategory {
	if strings.EqualFold(strings.TrimSpace(project.MobileStoreReadinessStatus), "succeeded") {
		return readinessCategory("store_readiness", "Store-readiness package", 100, target, "Store-readiness package is marked succeeded.", []string{"Project store-readiness status is succeeded."}, nil)
	}
	hasStoreCheck := false
	storePassed := false
	for _, check := range validation.Checks {
		if check.ID == "store_readiness" {
			hasStoreCheck = true
			storePassed = check.Status == MobileValidationPassed
			break
		}
	}
	if storePassed {
		return readinessCategory("store_readiness", "Store-readiness package", 75, target, "Draft store metadata package is valid but still needs signed artifact references and owner-supplied store assets.", []string{"Store-readiness JSON validates as a draft package."}, []string{"Attach native build artifact references and owner-supplied store URLs/screenshots to mark store package ready."})
	}
	if hasStoreCheck {
		return readinessCategory("store_readiness", "Store-readiness package", 35, target, "Store-readiness package exists but failed validation.", nil, []string{"Fix store-readiness metadata, privacy/data-safety, release notes, and screenshot checklist."})
	}
	return readinessCategory("store_readiness", "Store-readiness package", 0, target, "No store-readiness package evidence found.", nil, []string{"Generate the store-readiness package."})
}

func readinessCategory(id, label string, score int, target int, summary string, evidence []string, blockers []string) MobileReadinessCategory {
	score = clampScore(score)
	status := MobileReadinessPartial
	if score >= target {
		status = MobileReadinessComplete
	} else if score < 50 && len(blockers) > 0 {
		status = MobileReadinessBlocked
	}
	return MobileReadinessCategory{
		ID:       id,
		Label:    label,
		Score:    score,
		Target:   target,
		Status:   status,
		Summary:  summary,
		Evidence: evidence,
		Blockers: blockers,
	}
}

func requiredMobileSourceFiles() []string {
	return []string{
		"mobile/package.json",
		"mobile/app.config.ts",
		"mobile/eas.json",
		"mobile/README.md",
		"mobile/BUILD.md",
		"mobile/STORE_RELEASE.md",
		"mobile/src/permissions/nativeCapabilities.ts",
		"mobile/store/store-readiness.json",
		"mobile/store/privacy-data-safety.md",
		"mobile/store/screenshot-checklist.md",
		"mobile/store/release-notes.md",
	}
}

func validationCheckEvidence(validation MobileValidationReport) []string {
	evidence := make([]string, 0, len(validation.Checks))
	for _, check := range validation.Checks {
		evidence = append(evidence, check.Label+": "+string(check.Status))
	}
	return evidence
}

func normalizedProjectPlatformSet(project models.Project) map[string]bool {
	out := map[string]bool{}
	for _, platform := range project.MobilePlatforms {
		platform = strings.ToLower(strings.TrimSpace(platform))
		if platform == "" {
			continue
		}
		out[platform] = true
	}
	return out
}

func mobileMetadataBool(metadata map[string]interface{}, key string) bool {
	value, ok := metadata[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true") || strings.EqualFold(typed, "validated") || strings.EqualFold(typed, "succeeded")
	default:
		return false
	}
}

func mobileMetadataString(metadata map[string]interface{}, key string) string {
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	if typed, ok := value.(string); ok {
		return strings.TrimSpace(typed)
	}
	return ""
}

func mobileMetadataStringList(metadata map[string]interface{}, key string) []string {
	value, ok := metadata[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if itemString, ok := item.(string); ok && strings.TrimSpace(itemString) != "" {
				out = append(out, strings.TrimSpace(itemString))
			}
		}
		return out
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{strings.TrimSpace(typed)}
	default:
		return nil
	}
}

func mobileMetadataHasAny(metadata map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if strings.TrimSpace(mobileMetadataString(metadata, key)) != "" {
			return true
		}
	}
	return false
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
