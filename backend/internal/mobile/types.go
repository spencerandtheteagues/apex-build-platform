package mobile

type TargetPlatform string

const (
	TargetPlatformWeb             TargetPlatform = "web"
	TargetPlatformFullstackWeb    TargetPlatform = "fullstack_web"
	TargetPlatformMobileExpo      TargetPlatform = "mobile_expo"
	TargetPlatformMobileCapacitor TargetPlatform = "mobile_capacitor"
	TargetPlatformAPIOnly         TargetPlatform = "api_only"
)

type MobilePlatform string

const (
	MobilePlatformAndroid MobilePlatform = "android"
	MobilePlatformIOS     MobilePlatform = "ios"
)

type MobileFramework string

const (
	MobileFrameworkExpoReactNative MobileFramework = "expo-react-native"
	MobileFrameworkCapacitor       MobileFramework = "capacitor"
)

type MobileReleaseLevel string

const (
	ReleaseSourceOnly           MobileReleaseLevel = "source_only"
	ReleaseWebPreview           MobileReleaseLevel = "web_preview"
	ReleaseDevBuild             MobileReleaseLevel = "dev_build"
	ReleaseInternalAndroidAPK   MobileReleaseLevel = "internal_android_apk"
	ReleaseAndroidAAB           MobileReleaseLevel = "android_aab"
	ReleaseIOSSimulator         MobileReleaseLevel = "ios_simulator"
	ReleaseIOSInternal          MobileReleaseLevel = "ios_internal"
	ReleaseTestFlightReady      MobileReleaseLevel = "testflight_ready"
	ReleaseStoreSubmissionReady MobileReleaseLevel = "store_submission_ready"
)

type BackendMode string

const (
	BackendExistingApexGenerated BackendMode = "existing_apex_generated_backend"
	BackendNewGenerated          BackendMode = "new_generated_backend"
	BackendExternalAPIOnly       BackendMode = "external_api_only"
	BackendLocalOnly             BackendMode = "local_only"
)

type AuthMode string

const (
	AuthNone          AuthMode = "none"
	AuthEmailPassword AuthMode = "email_password"
	AuthOAuth         AuthMode = "oauth"
	AuthMagicLink     AuthMode = "magic_link"
	AuthEnterpriseSSO AuthMode = "enterprise_sso_future"
)

type DatabaseMode string

const (
	DatabaseGeneratedBackend DatabaseMode = "generated_backend_db"
	DatabaseLocalSQLite      DatabaseMode = "local_sqlite"
	DatabaseHybridOffline    DatabaseMode = "hybrid_offline_sync"
)

type MobileCapability string

const (
	CapabilityCamera             MobileCapability = "camera"
	CapabilityPhotoLibrary       MobileCapability = "photoLibrary"
	CapabilityPushNotifications  MobileCapability = "pushNotifications"
	CapabilityLocation           MobileCapability = "location"
	CapabilityMaps               MobileCapability = "maps"
	CapabilityFileUploads        MobileCapability = "fileUploads"
	CapabilityOfflineMode        MobileCapability = "offlineMode"
	CapabilityLocalNotifications MobileCapability = "localNotifications"
	CapabilityBackgroundTasks    MobileCapability = "backgroundTasks"
	CapabilityPayments           MobileCapability = "payments"
	CapabilityInAppPurchases     MobileCapability = "inAppPurchases"
	CapabilityBiometrics         MobileCapability = "biometrics"
	CapabilityDeepLinks          MobileCapability = "deepLinks"
	CapabilityUniversalLinks     MobileCapability = "universalLinks"
)

type MobileAppSpec struct {
	App          MobileAppIdentity       `json:"app"`
	Identity     MobileBinaryIdentity    `json:"identity"`
	Architecture MobileArchitecture      `json:"architecture"`
	Capabilities []MobileCapability      `json:"capabilities,omitempty"`
	Screens      []MobileScreenSpec      `json:"screens,omitempty"`
	Navigation   MobileNavigationSpec    `json:"navigation,omitempty"`
	DataModels   []MobileDataModelSpec   `json:"data_models,omitempty"`
	APIContracts []MobileAPIContractSpec `json:"api_contracts,omitempty"`
	Permissions  MobilePermissionSpec    `json:"permissions,omitempty"`
	Store        MobileStoreSpec         `json:"store,omitempty"`
	Quality      MobileQualitySpec       `json:"quality,omitempty"`
}

type MobileAppIdentity struct {
	Name            string           `json:"name"`
	Slug            string           `json:"slug"`
	Description     string           `json:"description,omitempty"`
	TargetPlatforms []MobilePlatform `json:"target_platforms"`
	PrimaryUseCase  string           `json:"primary_use_case,omitempty"`
	AppCategory     string           `json:"app_category,omitempty"`
	Audience        string           `json:"audience,omitempty"`
}

type MobileBinaryIdentity struct {
	AndroidPackage   string `json:"android_package,omitempty"`
	IOSBundleID      string `json:"ios_bundle_identifier,omitempty"`
	DisplayName      string `json:"display_name"`
	Version          string `json:"version"`
	VersionCode      int    `json:"version_code,omitempty"`
	BuildNumber      string `json:"build_number,omitempty"`
	IconAssetPath    string `json:"icon_asset_path,omitempty"`
	SplashAssetPath  string `json:"splash_asset_path,omitempty"`
	AdaptiveIconPath string `json:"adaptive_icon_path,omitempty"`
}

type MobileArchitecture struct {
	FrontendFramework MobileFramework `json:"frontend_framework"`
	BackendMode       BackendMode     `json:"backend_mode"`
	AuthMode          AuthMode        `json:"auth_mode"`
	DatabaseMode      DatabaseMode    `json:"database_mode"`
}

type MobileScreenSpec struct {
	Route        string   `json:"route"`
	Purpose      string   `json:"purpose"`
	Data         []string `json:"data,omitempty"`
	Actions      []string `json:"actions,omitempty"`
	States       []string `json:"states,omitempty"`
	AuthRequired bool     `json:"auth_required,omitempty"`
}

type MobileNavigationSpec struct {
	Tabs      []string `json:"tabs,omitempty"`
	Stacks    []string `json:"stacks,omitempty"`
	Modals    []string `json:"modals,omitempty"`
	AuthGates []string `json:"auth_gates,omitempty"`
}

type MobileDataModelSpec struct {
	Name          string            `json:"name"`
	Fields        map[string]string `json:"fields,omitempty"`
	Relationships map[string]string `json:"relationships,omitempty"`
	Validation    []string          `json:"validation,omitempty"`
}

type MobileAPIContractSpec struct {
	Name     string `json:"name"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Request  string `json:"request,omitempty"`
	Response string `json:"response,omitempty"`
}

type MobilePermissionSpec struct {
	IOSUsageDescriptions map[string]string `json:"ios_usage_descriptions,omitempty"`
	AndroidPermissions   []string          `json:"android_permissions,omitempty"`
}

type MobileStoreSpec struct {
	ShortDescription string   `json:"short_description,omitempty"`
	FullDescription  string   `json:"full_description,omitempty"`
	Keywords         []string `json:"keywords,omitempty"`
	Category         string   `json:"category,omitempty"`
	AgeRatingInputs  []string `json:"age_rating_inputs,omitempty"`
	PrivacyInputs    []string `json:"privacy_inputs,omitempty"`
	ReleaseNotes     string   `json:"release_notes,omitempty"`
}

type MobileQualitySpec struct {
	TestPlan                  []string `json:"test_plan,omitempty"`
	AccessibilityRequirements []string `json:"accessibility_requirements,omitempty"`
	PerformanceRequirements   []string `json:"performance_requirements,omitempty"`
	OfflineRequirements       []string `json:"offline_requirements,omitempty"`
}

type TargetPlatformClassification struct {
	TargetPlatform         TargetPlatform     `json:"target_platform"`
	MobilePlatforms        []MobilePlatform   `json:"mobile_platforms,omitempty"`
	Confidence             float64            `json:"confidence"`
	Rationale              string             `json:"rationale"`
	RequiredCapabilities   []MobileCapability `json:"required_capabilities,omitempty"`
	UnsupportedOrAmbiguous []string           `json:"unsupported_or_ambiguous,omitempty"`
	BackendNeeded          bool               `json:"backend_needed"`
}
