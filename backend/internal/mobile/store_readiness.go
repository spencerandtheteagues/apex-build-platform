package mobile

import (
	"encoding/json"
	"fmt"
	"strings"
)

type StoreReadinessPackage struct {
	Status              string                   `json:"status"`
	AppName             string                   `json:"app_name"`
	ShortDescription    string                   `json:"short_description"`
	FullDescription     string                   `json:"full_description"`
	Keywords            []string                 `json:"keywords"`
	Category            string                   `json:"category"`
	ReleaseNotes        string                   `json:"release_notes"`
	AndroidPackage      string                   `json:"android_package"`
	IOSBundleIdentifier string                   `json:"ios_bundle_identifier"`
	Version             string                   `json:"version"`
	VersionCode         int                      `json:"version_code"`
	BuildNumber         string                   `json:"build_number"`
	Permissions         MobilePermissionSpec     `json:"permissions"`
	DataSafetyDraft     StoreDataSafetyDraft     `json:"data_safety_draft"`
	ScreenshotChecklist []StoreScreenshotTarget  `json:"screenshot_checklist"`
	ManualPrerequisites []string                 `json:"manual_prerequisites"`
	TruthfulStatusNotes []string                 `json:"truthful_status_notes"`
	MissingItems        []string                 `json:"missing_items"`
	CapabilitySummary   []StoreCapabilitySummary `json:"capability_summary"`
}

type StoreDataSafetyDraft struct {
	DataCollected       []string `json:"data_collected"`
	DataLinkedToUser    []string `json:"data_linked_to_user"`
	DataUsedForTracking []string `json:"data_used_for_tracking"`
	PrivacyNotes        []string `json:"privacy_notes"`
}

type StoreScreenshotTarget struct {
	Platform string `json:"platform"`
	Device   string `json:"device"`
	Purpose  string `json:"purpose"`
}

type StoreCapabilitySummary struct {
	Capability string `json:"capability"`
	StoreRisk  string `json:"store_risk"`
	UserReason string `json:"user_reason"`
}

func GenerateStoreReadinessPackage(spec MobileAppSpec) StoreReadinessPackage {
	pkg := StoreReadinessPackage{
		Status:              "draft_ready_needs_manual_store_assets",
		AppName:             firstNonEmptyString(spec.Identity.DisplayName, spec.App.Name),
		ShortDescription:    strings.TrimSpace(spec.Store.ShortDescription),
		FullDescription:     strings.TrimSpace(spec.Store.FullDescription),
		Keywords:            append([]string(nil), spec.Store.Keywords...),
		Category:            strings.TrimSpace(spec.Store.Category),
		ReleaseNotes:        strings.TrimSpace(spec.Store.ReleaseNotes),
		AndroidPackage:      strings.TrimSpace(spec.Identity.AndroidPackage),
		IOSBundleIdentifier: strings.TrimSpace(spec.Identity.IOSBundleID),
		Version:             strings.TrimSpace(spec.Identity.Version),
		VersionCode:         spec.Identity.VersionCode,
		BuildNumber:         strings.TrimSpace(spec.Identity.BuildNumber),
		Permissions: MobilePermissionSpec{
			IOSUsageDescriptions: copyStringMap(spec.Permissions.IOSUsageDescriptions),
			AndroidPermissions:   append([]string(nil), spec.Permissions.AndroidPermissions...),
		},
		DataSafetyDraft:     dataSafetyDraftForSpec(spec),
		ScreenshotChecklist: screenshotTargetsForSpec(spec),
		ManualPrerequisites: []string{
			"Production icon and splash assets reviewed at required store sizes.",
			"Privacy policy URL and support URL supplied by the app owner.",
			"Signed Android/iOS build artifacts produced by a successful native build job.",
			"Store-console tax, compliance, encryption, and account agreements completed by the app owner.",
		},
		TruthfulStatusNotes: []string{
			"This package is a draft for launch preparation, not proof of App Store or Google Play approval.",
			"EAS Submit can upload binaries when enabled, but store listing metadata, screenshots, and review answers still require separate validation.",
		},
		CapabilitySummary: capabilitySummariesForSpec(spec),
	}
	pkg.MissingItems = missingStoreReadinessItems(pkg)
	return pkg
}

func StoreReadinessJSON(spec MobileAppSpec) string {
	pkg := GenerateStoreReadinessPackage(spec)
	encoded, _ := json.MarshalIndent(pkg, "", "  ")
	return string(encoded) + "\n"
}

func StorePrivacyDataSafetyMarkdown(spec MobileAppSpec) string {
	pkg := GenerateStoreReadinessPackage(spec)
	lines := []string{
		"# Privacy and Data Safety Draft",
		"",
		"This is a draft to help prepare App Store privacy labels and Google Play Data safety answers. It is not legal advice and must be reviewed before submission.",
		"",
		"## Data Collected",
	}
	lines = appendMarkdownList(lines, pkg.DataSafetyDraft.DataCollected)
	lines = append(lines, "", "## Data Linked to User")
	lines = appendMarkdownList(lines, pkg.DataSafetyDraft.DataLinkedToUser)
	lines = append(lines, "", "## Data Used for Tracking")
	lines = appendMarkdownList(lines, pkg.DataSafetyDraft.DataUsedForTracking)
	lines = append(lines, "", "## Permission Explanations")
	for key, value := range pkg.Permissions.IOSUsageDescriptions {
		lines = append(lines, fmt.Sprintf("- iOS `%s`: %s", key, value))
	}
	for _, permission := range pkg.Permissions.AndroidPermissions {
		lines = append(lines, fmt.Sprintf("- Android `%s`", permission))
	}
	lines = append(lines, "", "## Review Notes")
	lines = appendMarkdownList(lines, pkg.DataSafetyDraft.PrivacyNotes)
	return strings.Join(lines, "\n") + "\n"
}

func StoreScreenshotChecklistMarkdown(spec MobileAppSpec) string {
	pkg := GenerateStoreReadinessPackage(spec)
	lines := []string{
		"# Screenshot Checklist",
		"",
		"Capture real-device or simulator screenshots after the native build path is validated. Browser-rendered Expo Web screenshots are not proof of native behavior.",
		"",
	}
	for _, target := range pkg.ScreenshotChecklist {
		lines = append(lines, fmt.Sprintf("- [%s] %s: %s", target.Platform, target.Device, target.Purpose))
	}
	return strings.Join(lines, "\n") + "\n"
}

func StoreReleaseNotesMarkdown(spec MobileAppSpec) string {
	pkg := GenerateStoreReadinessPackage(spec)
	return fmt.Sprintf(`# Release Notes Draft

## Version

- App version: %s
- Android version code: %d
- iOS build number: %s

## Notes

%s
`, pkg.Version, maxInt(pkg.VersionCode, 1), firstNonEmptyString(pkg.BuildNumber, "1"), firstNonEmptyString(pkg.ReleaseNotes, "Initial internal build."))
}

func ValidateStoreReadinessPackage(pkg StoreReadinessPackage) []ValidationError {
	var errs []ValidationError
	if strings.TrimSpace(pkg.AppName) == "" {
		errs = append(errs, ValidationError{Field: "store.app_name", Message: "app name is required"})
	}
	if strings.TrimSpace(pkg.ShortDescription) == "" {
		errs = append(errs, ValidationError{Field: "store.short_description", Message: "short description is required"})
	}
	if strings.TrimSpace(pkg.Category) == "" {
		errs = append(errs, ValidationError{Field: "store.category", Message: "category is required"})
	}
	if strings.TrimSpace(pkg.AndroidPackage) == "" {
		errs = append(errs, ValidationError{Field: "identity.android_package", Message: "Android package is required"})
	}
	if strings.TrimSpace(pkg.IOSBundleIdentifier) == "" {
		errs = append(errs, ValidationError{Field: "identity.ios_bundle_identifier", Message: "iOS bundle identifier is required"})
	}
	return errs
}

func dataSafetyDraftForSpec(spec MobileAppSpec) StoreDataSafetyDraft {
	collected := []string{"Account credentials or session token", "Customer contact details", "Job and estimate records"}
	linked := []string{"Account credentials or session token", "Customer contact details", "Job and estimate records"}
	notes := []string{"Do not embed server API keys in the mobile app.", "Review all generated defaults against the actual production backend before submission."}

	if hasCapability(spec, CapabilityCamera) || hasCapability(spec, CapabilityPhotoLibrary) || hasCapability(spec, CapabilityFileUploads) {
		collected = append(collected, "Photos or documents selected by the user")
		linked = append(linked, "Photos or documents selected by the user")
		notes = append(notes, "Photo/file collection depends on user-selected upload workflows.")
	}
	if hasCapability(spec, CapabilityLocation) || hasCapability(spec, CapabilityMaps) {
		collected = append(collected, "Approximate or precise location when enabled by the user")
		linked = append(linked, "Location data if stored with jobs or customer records")
		notes = append(notes, "Location usage must match the final product behavior and permission copy.")
	}
	if hasCapability(spec, CapabilityPushNotifications) || hasCapability(spec, CapabilityLocalNotifications) {
		collected = append(collected, "Device push token")
		linked = append(linked, "Device push token")
	}

	return StoreDataSafetyDraft{
		DataCollected:       normalizeProjectStringList(collected),
		DataLinkedToUser:    normalizeProjectStringList(linked),
		DataUsedForTracking: []string{"None declared by generated scaffold"},
		PrivacyNotes:        notes,
	}
}

func screenshotTargetsForSpec(spec MobileAppSpec) []StoreScreenshotTarget {
	targets := []StoreScreenshotTarget{}
	if platformEnabled(spec, MobilePlatformAndroid) {
		targets = append(targets,
			StoreScreenshotTarget{Platform: "Android", Device: "Phone", Purpose: "Home/jobs list with representative data"},
			StoreScreenshotTarget{Platform: "Android", Device: "Phone", Purpose: "Primary create/edit workflow"},
		)
	}
	if platformEnabled(spec, MobilePlatformIOS) {
		targets = append(targets,
			StoreScreenshotTarget{Platform: "iOS", Device: "6.7-inch iPhone", Purpose: "Home/jobs list with representative data"},
			StoreScreenshotTarget{Platform: "iOS", Device: "6.7-inch iPhone", Purpose: "Primary create/edit workflow"},
			StoreScreenshotTarget{Platform: "iOS", Device: "iPad", Purpose: "Tablet layout if iPad support remains enabled"},
		)
	}
	return targets
}

func capabilitySummariesForSpec(spec MobileAppSpec) []StoreCapabilitySummary {
	summaries := make([]StoreCapabilitySummary, 0, len(spec.Capabilities))
	for _, capability := range spec.Capabilities {
		summaries = append(summaries, StoreCapabilitySummary{
			Capability: string(capability),
			StoreRisk:  storeRiskForCapability(capability),
			UserReason: userReasonForCapability(capability),
		})
	}
	return summaries
}

func storeRiskForCapability(capability MobileCapability) string {
	switch capability {
	case CapabilityCamera, CapabilityPhotoLibrary, CapabilityFileUploads:
		return "Requires accurate permission copy and privacy/data safety disclosure."
	case CapabilityLocation, CapabilityMaps:
		return "Requires precise location-use disclosure if enabled in production."
	case CapabilityPushNotifications, CapabilityLocalNotifications:
		return "Requires notification consent flow and accurate messaging disclosure."
	case CapabilityPayments, CapabilityInAppPurchases:
		return "Requires store payment policy review before release."
	default:
		return "Review final implementation before store submission."
	}
}

func userReasonForCapability(capability MobileCapability) string {
	switch capability {
	case CapabilityCamera:
		return "Capture job-site photos."
	case CapabilityPhotoLibrary:
		return "Select existing project photos."
	case CapabilityFileUploads:
		return "Attach files to jobs, estimates, or customer records."
	case CapabilityOfflineMode:
		return "Save field drafts without reliable connectivity."
	case CapabilityPushNotifications, CapabilityLocalNotifications:
		return "Send reminders and job updates."
	case CapabilityLocation, CapabilityMaps:
		return "Support field routing or job-site context."
	default:
		return "Capability requested by the app specification."
	}
}

func missingStoreReadinessItems(pkg StoreReadinessPackage) []string {
	missing := append([]string(nil), pkg.ManualPrerequisites...)
	if strings.TrimSpace(pkg.ReleaseNotes) == "" {
		missing = append(missing, "Release notes reviewed and finalized.")
	}
	return missing
}

func hasCapability(spec MobileAppSpec, capability MobileCapability) bool {
	for _, item := range spec.Capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

func platformEnabled(spec MobileAppSpec, platform MobilePlatform) bool {
	for _, item := range spec.App.TargetPlatforms {
		if item == platform {
			return true
		}
	}
	return false
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func appendMarkdownList(lines []string, values []string) []string {
	if len(values) == 0 {
		return append(lines, "- Not declared in generated draft.")
	}
	for _, value := range values {
		lines = append(lines, "- "+value)
	}
	return lines
}
