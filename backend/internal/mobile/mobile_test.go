package mobile

import (
	"os"
	"testing"
)

func TestValidateMobileAppSpecAcceptsFullStackExpoSpec(t *testing.T) {
	spec := validContractorQuoteSpec()

	if errs := ValidateMobileAppSpec(spec); len(errs) != 0 {
		t.Fatalf("expected valid spec, got %+v", errs)
	}
}

func TestValidateMobileAppSpecRejectsInvalidIdentifiers(t *testing.T) {
	spec := validContractorQuoteSpec()
	spec.Identity.AndroidPackage = "Apex.FieldOps"
	spec.Identity.IOSBundleID = "not a bundle"

	errs := ValidateMobileAppSpec(spec)
	if !hasValidationError(errs, "identity.android_package") {
		t.Fatalf("expected android package validation error, got %+v", errs)
	}
	if !hasValidationError(errs, "identity.ios_bundle_identifier") {
		t.Fatalf("expected iOS bundle validation error, got %+v", errs)
	}
}

func TestValidateMobileAppSpecRequiresIOSPermissionDescriptions(t *testing.T) {
	spec := validContractorQuoteSpec()
	delete(spec.Permissions.IOSUsageDescriptions, "NSCameraUsageDescription")

	errs := ValidateMobileAppSpec(spec)
	if !hasValidationError(errs, "permissions.ios_usage_descriptions.NSCameraUsageDescription") {
		t.Fatalf("expected missing camera permission description, got %+v", errs)
	}
}

func TestValidateMobileAppSpecRejectsUnsupportedModesCapabilitiesAndIncompleteAPIContracts(t *testing.T) {
	spec := validContractorQuoteSpec()
	spec.Architecture.BackendMode = BackendMode("ftp_backend")
	spec.Architecture.AuthMode = AuthMode("shared_password")
	spec.Architecture.DatabaseMode = DatabaseMode("browser_indexeddb")
	spec.Capabilities = append(spec.Capabilities, MobileCapability("arbitraryNativeModule"))
	spec.APIContracts = []MobileAPIContractSpec{{Method: "POST", Path: "/api/estimates"}}

	errs := ValidateMobileAppSpec(spec)
	if !hasValidationError(errs, "architecture.backend_mode") {
		t.Fatalf("expected backend mode validation error, got %+v", errs)
	}
	if !hasValidationError(errs, "architecture.auth_mode") {
		t.Fatalf("expected auth mode validation error, got %+v", errs)
	}
	if !hasValidationError(errs, "architecture.database_mode") {
		t.Fatalf("expected database mode validation error, got %+v", errs)
	}
	if !hasValidationError(errs, "capabilities") {
		t.Fatalf("expected unsupported capability validation error, got %+v", errs)
	}
	if !hasValidationError(errs, "api_contracts.0.name") {
		t.Fatalf("expected API contract name validation error, got %+v", errs)
	}
}

func TestClassifyTargetPlatformDistinguishesNativeMobileFromResponsiveWeb(t *testing.T) {
	native := ClassifyTargetPlatform("Build a contractor quote builder mobile app for iOS and Android with camera photo uploads, offline drafts, login, and push reminders.")
	if native.TargetPlatform != TargetPlatformMobileExpo {
		t.Fatalf("expected mobile_expo, got %+v", native)
	}
	if len(native.MobilePlatforms) != 2 {
		t.Fatalf("expected Android and iOS platform inference, got %+v", native.MobilePlatforms)
	}
	if !native.BackendNeeded {
		t.Fatalf("expected backend needed for full-stack mobile prompt")
	}
	if len(native.RequiredCapabilities) < 3 {
		t.Fatalf("expected inferred native capabilities, got %+v", native.RequiredCapabilities)
	}

	web := ClassifyTargetPlatform("Build a responsive website with a mobile-first layout for phones.")
	if web.TargetPlatform != TargetPlatformWeb {
		t.Fatalf("expected web for responsive website, got %+v", web)
	}
}

func TestNativeCapabilityRegistryAllowsExpoPackagesAndRejectsUnsupportedNativeModules(t *testing.T) {
	registry := DefaultNativeCapabilityRegistry()
	if !registry.PackageAllowed("expo-camera") {
		t.Fatal("expected expo-camera to be allowed")
	}
	if registry.PackageAllowed("react-native-vision-camera") {
		t.Fatal("expected arbitrary native module to be disallowed by default")
	}
	camera, ok := registry.Definition(CapabilityCamera)
	if !ok {
		t.Fatal("expected camera capability definition")
	}
	if !camera.EASBuildRequired || !camera.DevelopmentRebuildRequired {
		t.Fatalf("expected camera to require EAS/dev rebuild, got %+v", camera)
	}
}

func TestLoadFeatureFlagsFromEnvDefaultsSafeAndGatesEAS(t *testing.T) {
	t.Setenv("MOBILE_BUILDER_ENABLED", "")
	t.Setenv("MOBILE_EAS_BUILD_ENABLED", "")
	flags := LoadFeatureFlagsFromEnv()
	if flags.MobileBuilderEnabled || flags.MobileEASBuildEnabled || flags.MobileEASSubmitEnabled {
		t.Fatalf("expected mobile flags to default off, got %+v", flags)
	}

	t.Setenv("MOBILE_BUILDER_ENABLED", "true")
	t.Setenv("MOBILE_EXPO_ENABLED", "yes")
	t.Setenv("MOBILE_EAS_BUILD_ENABLED", "false")
	flags = LoadFeatureFlagsFromEnv()
	if !flags.MobileBuilderEnabled || !flags.MobileExpoEnabled {
		t.Fatalf("expected source-generation flags enabled, got %+v", flags)
	}
	if flags.MobileEASBuildEnabled {
		t.Fatalf("expected EAS build to remain disabled, got %+v", flags)
	}

	_ = os.Unsetenv("MOBILE_BUILDER_ENABLED")
}

func validContractorQuoteSpec() MobileAppSpec {
	return MobileAppSpec{
		App: MobileAppIdentity{
			Name:            "Apex FieldOps Mobile",
			Slug:            "apex-fieldops-mobile",
			TargetPlatforms: []MobilePlatform{MobilePlatformAndroid, MobilePlatformIOS},
			PrimaryUseCase:  "contractor quote builder",
			AppCategory:     "business",
		},
		Identity: MobileBinaryIdentity{
			AndroidPackage: "com.apexbuild.fieldops",
			IOSBundleID:    "com.apexbuild.fieldops",
			DisplayName:    "Apex FieldOps",
			Version:        "1.0.0",
			VersionCode:    1,
			BuildNumber:    "1",
		},
		Architecture: MobileArchitecture{
			FrontendFramework: MobileFrameworkExpoReactNative,
			BackendMode:       BackendNewGenerated,
			AuthMode:          AuthEmailPassword,
			DatabaseMode:      DatabaseHybridOffline,
		},
		Capabilities: []MobileCapability{
			CapabilityCamera,
			CapabilityPhotoLibrary,
			CapabilityFileUploads,
			CapabilityOfflineMode,
			CapabilityPushNotifications,
		},
		APIContracts: []MobileAPIContractSpec{
			{Name: "create estimate", Method: "POST", Path: "/api/estimates", Request: "EstimateInput", Response: "Estimate"},
		},
		Permissions: MobilePermissionSpec{
			IOSUsageDescriptions: map[string]string{
				"NSCameraUsageDescription":            "Attach job-site photos to estimates.",
				"NSPhotoLibraryUsageDescription":      "Select existing job-site photos for estimates.",
				"NSUserNotificationsUsageDescription": "Send quote follow-up reminders.",
			},
			AndroidPermissions: []string{"android.permission.CAMERA", "android.permission.POST_NOTIFICATIONS"},
		},
	}
}

func hasValidationError(errs []ValidationError, field string) bool {
	for _, err := range errs {
		if err.Field == field {
			return true
		}
	}
	return false
}
