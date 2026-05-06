package mobile

import (
	"strings"

	"apex-build/pkg/models"
)

// ProjectMetadataFields is the additive mobile metadata accepted by project
// creation/update paths. It intentionally stores workflow status separately
// from binary/store readiness so source export cannot be mistaken for launch.
type ProjectMetadataFields struct {
	TargetPlatform         string   `json:"target_platform,omitempty"`
	MobilePlatforms        []string `json:"mobile_platforms,omitempty"`
	MobileFramework        string   `json:"mobile_framework,omitempty"`
	MobileReleaseLevel     string   `json:"mobile_release_level,omitempty"`
	MobileCapabilities     []string `json:"mobile_capabilities,omitempty"`
	MobileDependencyPolicy string   `json:"mobile_dependency_policy,omitempty"`
}

func ApplyProjectMetadata(project *models.Project, fields ProjectMetadataFields) {
	if project == nil {
		return
	}

	target := normalizeProjectTargetPlatform(fields.TargetPlatform)
	hasMobileMetadata := len(fields.MobilePlatforms) > 0 ||
		strings.TrimSpace(fields.MobileFramework) != "" ||
		strings.TrimSpace(fields.MobileReleaseLevel) != "" ||
		len(fields.MobileCapabilities) > 0 ||
		strings.TrimSpace(fields.MobileDependencyPolicy) != ""

	if target == "" {
		if hasMobileMetadata {
			target = TargetPlatformMobileExpo
		} else {
			target = TargetPlatformWeb
		}
	}

	project.TargetPlatform = string(target)
	project.MobilePreviewStatus = defaultStatus(project.MobilePreviewStatus, "not_requested")
	project.MobileBuildStatus = defaultStatus(project.MobileBuildStatus, "not_requested")
	project.MobileStoreReadinessStatus = defaultStatus(project.MobileStoreReadinessStatus, "not_requested")

	switch target {
	case TargetPlatformMobileExpo:
		project.MobilePlatforms = normalizeProjectMobilePlatforms(fields.MobilePlatforms, []string{string(MobilePlatformAndroid), string(MobilePlatformIOS)})
		project.MobileFramework = defaultString(fields.MobileFramework, string(MobileFrameworkExpoReactNative))
		project.MobileReleaseLevel = defaultString(fields.MobileReleaseLevel, string(ReleaseSourceOnly))
		project.MobileCapabilities = normalizeProjectStringList(fields.MobileCapabilities)
		project.MobileDependencyPolicy = defaultString(fields.MobileDependencyPolicy, "expo-allowlist")
		project.GeneratedMobileClientPath = defaultString(project.GeneratedMobileClientPath, "mobile/")
		project.MobilePreviewStatus = "source_only"
	case TargetPlatformMobileCapacitor:
		project.MobilePlatforms = normalizeProjectMobilePlatforms(fields.MobilePlatforms, []string{string(MobilePlatformAndroid), string(MobilePlatformIOS)})
		project.MobileFramework = defaultString(fields.MobileFramework, string(MobileFrameworkCapacitor))
		project.MobileReleaseLevel = defaultString(fields.MobileReleaseLevel, string(ReleaseSourceOnly))
		project.MobileCapabilities = normalizeProjectStringList(fields.MobileCapabilities)
		project.GeneratedMobileClientPath = defaultString(project.GeneratedMobileClientPath, "mobile/")
		project.MobilePreviewStatus = "source_only"
	default:
		project.MobilePlatforms = nil
		project.MobileFramework = ""
		project.MobileReleaseLevel = ""
		project.MobileCapabilities = nil
		project.MobileDependencyPolicy = ""
		project.GeneratedMobileClientPath = ""
	}
}

func normalizeProjectTargetPlatform(raw string) TargetPlatform {
	switch TargetPlatform(strings.TrimSpace(raw)) {
	case TargetPlatformWeb:
		return TargetPlatformWeb
	case TargetPlatformFullstackWeb:
		return TargetPlatformFullstackWeb
	case TargetPlatformMobileExpo:
		return TargetPlatformMobileExpo
	case TargetPlatformMobileCapacitor:
		return TargetPlatformMobileCapacitor
	case TargetPlatformAPIOnly:
		return TargetPlatformAPIOnly
	default:
		return ""
	}
}

func normalizeProjectMobilePlatforms(platforms []string, fallback []string) []string {
	normalized := make([]string, 0, len(platforms))
	seen := map[string]struct{}{}
	for _, platform := range platforms {
		switch strings.TrimSpace(platform) {
		case string(MobilePlatformAndroid):
			if _, ok := seen[string(MobilePlatformAndroid)]; !ok {
				normalized = append(normalized, string(MobilePlatformAndroid))
				seen[string(MobilePlatformAndroid)] = struct{}{}
			}
		case string(MobilePlatformIOS):
			if _, ok := seen[string(MobilePlatformIOS)]; !ok {
				normalized = append(normalized, string(MobilePlatformIOS))
				seen[string(MobilePlatformIOS)] = struct{}{}
			}
		}
	}
	if len(normalized) == 0 {
		return append([]string(nil), fallback...)
	}
	return normalized
}

func normalizeProjectStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		normalized = append(normalized, value)
		seen[value] = struct{}{}
	}
	return normalized
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func defaultStatus(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
