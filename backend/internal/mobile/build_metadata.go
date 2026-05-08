package mobile

import (
	"strings"

	"apex-build/pkg/models"
)

func ApplyMobileBuildJobToProject(project *models.Project, job MobileBuildJob) {
	if project == nil || strings.TrimSpace(job.ID) == "" {
		return
	}

	project.MobileBuildStatus = string(job.Status)
	metadata := copyMobileMetadata(project.MobileMetadata)
	metadata["last_mobile_build_id"] = job.ID
	metadata["last_mobile_build_platform"] = string(job.Platform)
	metadata["last_mobile_build_profile"] = string(job.Profile)
	metadata["last_mobile_build_release_level"] = string(job.ReleaseLevel)
	metadata["last_mobile_build_status"] = string(job.Status)
	if job.ProviderBuildID != "" {
		metadata["last_mobile_provider_build_id"] = job.ProviderBuildID
	}
	if job.FailureType != "" {
		metadata["last_mobile_build_failure_type"] = string(job.FailureType)
	}
	if job.FailureMessage != "" {
		metadata["last_mobile_build_failure_message"] = RedactMobileBuildSecrets(job.FailureMessage)
	}
	if plan := BuildMobileBuildRepairPlan(job); plan != nil {
		metadata["last_mobile_build_repair_required"] = true
		metadata["last_mobile_build_repair_title"] = plan.Title
		metadata["last_mobile_build_repair_requires_credentials"] = plan.RequiresCredentialAction
		metadata["last_mobile_build_repair_requires_source_change"] = plan.RequiresSourceChange
	} else {
		metadata["last_mobile_build_repair_required"] = false
		delete(metadata, "last_mobile_build_repair_title")
		delete(metadata, "last_mobile_build_repair_requires_credentials")
		delete(metadata, "last_mobile_build_repair_requires_source_change")
	}
	if job.ArtifactURL != "" {
		metadata["last_mobile_build_artifact_url"] = job.ArtifactURL
		metadata["artifact_urls"] = appendUniqueMobileBuildMetadataString(mobileMetadataStringList(metadata, "artifact_urls"), job.ArtifactURL)
		switch job.Platform {
		case MobilePlatformAndroid:
			metadata["android_artifact_url"] = job.ArtifactURL
			if job.ReleaseLevel == ReleaseInternalAndroidAPK {
				metadata["android_apk_url"] = job.ArtifactURL
			}
			if job.ReleaseLevel == ReleaseAndroidAAB {
				metadata["android_aab_url"] = job.ArtifactURL
			}
		case MobilePlatformIOS:
			metadata["ios_artifact_url"] = job.ArtifactURL
			if job.ReleaseLevel == ReleaseIOSSimulator {
				metadata["ios_simulator_artifact_url"] = job.ArtifactURL
			}
		}
	}
	project.MobileMetadata = metadata
}

func appendUniqueMobileBuildMetadataString(values []string, next string) []string {
	next = strings.TrimSpace(next)
	if next == "" {
		return values
	}
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}
