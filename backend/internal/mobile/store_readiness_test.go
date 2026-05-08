package mobile

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildMobileStoreReadinessReportSeparatesDraftFromSubmissionReady(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	sourceFiles, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	require.Empty(t, errs)
	files := modelFilesFromSource(sourceFiles)
	project := modelsProjectForSpec(spec, string(ReleaseSourceOnly), string(MobileBuildSucceeded))

	validation := ValidateProjectSourcePackage(project, files)
	scorecard := BuildMobileReadinessScorecard(project, files, validation)
	report := BuildMobileStoreReadinessReport(project, files, validation, scorecard)

	require.False(t, report.ReadyForSubmission)
	require.Equal(t, "draft_ready_needs_manual_store_assets", report.Status)
	require.NotNil(t, report.Package)
	require.NotEmpty(t, report.ManualPrerequisites)
	require.Contains(t, report.Summary, "Draft store-readiness package is valid")
}

func TestBuildMobileStoreReadinessReportMarksReadyOnlyWithSucceededProjectStatus(t *testing.T) {
	spec := FieldServiceContractorQuoteSpec()
	sourceFiles, errs := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	require.Empty(t, errs)
	files := modelFilesFromSource(sourceFiles)
	project := modelsProjectForSpec(spec, string(ReleaseStoreSubmissionReady), string(MobileBuildSucceeded))
	project.MobileStoreReadinessStatus = "succeeded"

	validation := ValidateProjectSourcePackage(project, files)
	scorecard := BuildMobileReadinessScorecard(project, files, validation)
	report := BuildMobileStoreReadinessReport(project, files, validation, scorecard)

	require.True(t, report.ReadyForSubmission)
	require.Equal(t, "ready_for_submission_workflow", report.Status)
	require.Contains(t, report.Summary, "not store approval")
}
