package mobile

import (
	"testing"

	"apex-build/pkg/models"
)

func TestBuildMobileReadinessScorecardBlocksNativeReadinessWithoutCredentialsAndArtifacts(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}
	project := modelsProjectForSpec(spec, string(ReleaseSourceOnly), "not_requested")
	project.MobilePlatforms = []string{string(MobilePlatformAndroid), string(MobilePlatformIOS)}
	project.MobileFramework = string(MobileFrameworkExpoReactNative)
	project.GeneratedMobileClientPath = "mobile/"

	validation := ValidateProjectSourcePackage(project, modelFilesFromSource(files))
	scorecard := BuildMobileReadinessScorecard(project, modelFilesFromSource(files), validation)

	if scorecard.IsReady {
		t.Fatalf("expected scorecard to block launch readiness without artifacts/credentials, got %+v", scorecard)
	}
	if scorecard.OverallScore >= scorecard.TargetScore {
		t.Fatalf("expected overall score below target without artifacts/credentials, got %+v", scorecard)
	}
	if categoryScore(scorecard, "source_generation") < 95 {
		t.Fatalf("expected source generation to be >=95, got %+v", scorecard.Categories)
	}
	if categoryScore(scorecard, "source_validation") < 95 {
		t.Fatalf("expected source validation to be >=95, got %+v", scorecard.Categories)
	}
	if categoryScore(scorecard, "native_artifacts") != 0 {
		t.Fatalf("expected native artifacts to be 0 without build proof, got %+v", scorecard.Categories)
	}
	if categoryScore(scorecard, "credentials_signing") != 0 {
		t.Fatalf("expected credentials/signing to be 0 without validated credentials, got %+v", scorecard.Categories)
	}
	if !scorecardHasBlocker(scorecard, "Produce a signed Android APK/AAB artifact through EAS Build.") {
		t.Fatalf("expected Android artifact blocker, got %+v", scorecard.Blockers)
	}
	if !scorecardHasBlocker(scorecard, "Add encrypted user-provided mobile credential vault and validate EAS/Apple/Google signing prerequisites.") {
		t.Fatalf("expected credential blocker, got %+v", scorecard.Blockers)
	}
}

func TestBuildMobileReadinessScorecardReachesReadyWithEvidenceForAllCategories(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	files, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(errs) > 0 {
		t.Fatalf("expected generated files, got errors %+v", errs)
	}
	project := modelsProjectForSpec(spec, string(ReleaseStoreSubmissionReady), string(MobileBuildSucceeded))
	project.MobilePlatforms = []string{string(MobilePlatformAndroid), string(MobilePlatformIOS)}
	project.MobileFramework = string(MobileFrameworkExpoReactNative)
	project.GeneratedMobileClientPath = "mobile/"
	project.MobileStoreReadinessStatus = "succeeded"
	project.MobileMetadata = map[string]interface{}{
		"credentials_validated": true,
		"android_artifact_url":  "https://artifacts.example.com/app.aab",
		"ios_artifact_url":      "https://artifacts.example.com/app.tar.gz",
	}

	validation := ValidateProjectSourcePackage(project, modelFilesFromSource(files))
	scorecard := BuildMobileReadinessScorecard(project, modelFilesFromSource(files), validation)

	if !scorecard.IsReady {
		t.Fatalf("expected ready scorecard, got %+v", scorecard)
	}
	if scorecard.OverallScore < 95 {
		t.Fatalf("expected overall score >=95, got %+v", scorecard)
	}
	for _, category := range scorecard.Categories {
		if category.Score < 95 {
			t.Fatalf("expected category %s to be >=95, got %+v", category.ID, category)
		}
	}
	if len(scorecard.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %+v", scorecard.Blockers)
	}
}

func TestBuildMobileReadinessScorecardMarksWebProjectNotReady(t *testing.T) {
	project := models.Project{Name: "Web App", TargetPlatform: string(TargetPlatformFullstackWeb)}
	validation := ValidateProjectSourcePackage(project, nil)
	scorecard := BuildMobileReadinessScorecard(project, nil, validation)

	if scorecard.IsReady {
		t.Fatalf("expected web project to be not ready for mobile, got %+v", scorecard)
	}
	if categoryScore(scorecard, "target_metadata") != 0 {
		t.Fatalf("expected target metadata score 0, got %+v", scorecard.Categories)
	}
}

func categoryScore(scorecard MobileReadinessScorecard, id string) int {
	for _, category := range scorecard.Categories {
		if category.ID == id {
			return category.Score
		}
	}
	return -1
}

func scorecardHasBlocker(scorecard MobileReadinessScorecard, blocker string) bool {
	for _, existing := range scorecard.Blockers {
		if existing == blocker {
			return true
		}
	}
	return false
}
