package mobile

import (
	"os"
	"strings"
)

type FeatureFlags struct {
	MobileBuilderEnabled       bool `json:"mobile_builder_enabled"`
	MobileExpoEnabled          bool `json:"mobile_expo_enabled"`
	MobileCapacitorEnabled     bool `json:"mobile_capacitor_enabled"`
	MobileEASBuildEnabled      bool `json:"mobile_eas_build_enabled"`
	MobileEASSubmitEnabled     bool `json:"mobile_eas_submit_enabled"`
	MobileStoreMetadataEnabled bool `json:"mobile_store_metadata_enabled"`
	MobileIOSBuildsEnabled     bool `json:"mobile_ios_builds_enabled"`
	MobileAndroidBuildsEnabled bool `json:"mobile_android_builds_enabled"`
}

func LoadFeatureFlagsFromEnv() FeatureFlags {
	return FeatureFlags{
		MobileBuilderEnabled:       envBool("MOBILE_BUILDER_ENABLED", false),
		MobileExpoEnabled:          envBool("MOBILE_EXPO_ENABLED", false),
		MobileCapacitorEnabled:     envBool("MOBILE_CAPACITOR_ENABLED", false),
		MobileEASBuildEnabled:      envBool("MOBILE_EAS_BUILD_ENABLED", false),
		MobileEASSubmitEnabled:     envBool("MOBILE_EAS_SUBMIT_ENABLED", false),
		MobileStoreMetadataEnabled: envBool("MOBILE_STORE_METADATA_ENABLED", false),
		MobileIOSBuildsEnabled:     envBool("MOBILE_IOS_BUILDS_ENABLED", false),
		MobileAndroidBuildsEnabled: envBool("MOBILE_ANDROID_BUILDS_ENABLED", false),
	}
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "y", "on", "enabled":
		return true
	case "0", "false", "no", "n", "off", "disabled":
		return false
	default:
		return fallback
	}
}
