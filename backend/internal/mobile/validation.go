package mobile

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	androidPackageRE = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){1,}$`)
	iosBundleIDRE    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*(\.[A-Za-z0-9-]+){1,}$`)
	slugRE           = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

func ValidateMobileAppSpec(spec MobileAppSpec) []ValidationError {
	var errs []ValidationError
	add := func(field, message string) {
		errs = append(errs, ValidationError{Field: field, Message: message})
	}

	if strings.TrimSpace(spec.App.Name) == "" {
		add("app.name", "app name is required")
	}
	if strings.TrimSpace(spec.App.Slug) == "" {
		add("app.slug", "app slug is required")
	} else if !slugRE.MatchString(spec.App.Slug) {
		add("app.slug", "must use lowercase letters, numbers, and hyphens")
	}
	if len(spec.App.TargetPlatforms) == 0 {
		add("app.target_platforms", "at least one mobile platform is required")
	}

	platformSet := map[MobilePlatform]struct{}{}
	for _, platform := range spec.App.TargetPlatforms {
		switch platform {
		case MobilePlatformAndroid, MobilePlatformIOS:
			platformSet[platform] = struct{}{}
		default:
			add("app.target_platforms", fmt.Sprintf("unsupported platform %q", platform))
		}
	}
	if _, wantsAndroid := platformSet[MobilePlatformAndroid]; wantsAndroid {
		if strings.TrimSpace(spec.Identity.AndroidPackage) == "" {
			add("identity.android_package", "android package is required when Android is selected")
		} else if !androidPackageRE.MatchString(spec.Identity.AndroidPackage) {
			add("identity.android_package", "invalid Android package name")
		}
	}
	if _, wantsIOS := platformSet[MobilePlatformIOS]; wantsIOS {
		if strings.TrimSpace(spec.Identity.IOSBundleID) == "" {
			add("identity.ios_bundle_identifier", "iOS bundle identifier is required when iOS is selected")
		} else if !iosBundleIDRE.MatchString(spec.Identity.IOSBundleID) {
			add("identity.ios_bundle_identifier", "invalid iOS bundle identifier")
		}
	}
	if strings.TrimSpace(spec.Identity.DisplayName) == "" {
		add("identity.display_name", "display name is required")
	}
	if strings.TrimSpace(spec.Identity.Version) == "" {
		add("identity.version", "app version is required")
	}
	if spec.Architecture.FrontendFramework == "" {
		add("architecture.frontend_framework", "frontend framework is required")
	} else if spec.Architecture.FrontendFramework != MobileFrameworkExpoReactNative && spec.Architecture.FrontendFramework != MobileFrameworkCapacitor {
		add("architecture.frontend_framework", "unsupported mobile frontend framework")
	}
	if spec.Architecture.BackendMode == "" {
		add("architecture.backend_mode", "backend mode is required")
	} else if !isSupportedBackendMode(spec.Architecture.BackendMode) {
		add("architecture.backend_mode", "unsupported backend mode")
	}
	if spec.Architecture.AuthMode == "" {
		add("architecture.auth_mode", "auth mode is required")
	} else if !isSupportedAuthMode(spec.Architecture.AuthMode) {
		add("architecture.auth_mode", "unsupported auth mode")
	}
	if spec.Architecture.DatabaseMode == "" {
		add("architecture.database_mode", "database mode is required")
	} else if !isSupportedDatabaseMode(spec.Architecture.DatabaseMode) {
		add("architecture.database_mode", "unsupported database mode")
	}
	if backendRequired(spec) && len(spec.APIContracts) == 0 {
		add("api_contracts", "backend/API contracts are required for full-stack mobile apps")
	}
	errs = append(errs, validateCapabilities(spec)...)
	errs = append(errs, validateAPIContracts(spec)...)
	errs = append(errs, validatePermissionDescriptions(spec, platformSet)...)
	return errs
}

func isSupportedBackendMode(mode BackendMode) bool {
	switch mode {
	case BackendExistingApexGenerated, BackendNewGenerated, BackendExternalAPIOnly, BackendLocalOnly:
		return true
	default:
		return false
	}
}

func isSupportedAuthMode(mode AuthMode) bool {
	switch mode {
	case AuthNone, AuthEmailPassword, AuthOAuth, AuthMagicLink, AuthEnterpriseSSO:
		return true
	default:
		return false
	}
}

func isSupportedDatabaseMode(mode DatabaseMode) bool {
	switch mode {
	case DatabaseGeneratedBackend, DatabaseLocalSQLite, DatabaseHybridOffline:
		return true
	default:
		return false
	}
}

func backendRequired(spec MobileAppSpec) bool {
	switch spec.Architecture.BackendMode {
	case BackendExistingApexGenerated, BackendNewGenerated, BackendExternalAPIOnly:
		return true
	default:
		return false
	}
}

func validateCapabilities(spec MobileAppSpec) []ValidationError {
	registry := DefaultNativeCapabilityRegistry()
	var errs []ValidationError
	for _, capability := range spec.Capabilities {
		if _, ok := registry.Definition(capability); !ok {
			errs = append(errs, ValidationError{
				Field:   "capabilities",
				Message: fmt.Sprintf("unsupported native capability %q", capability),
			})
		}
	}
	return errs
}

func validateAPIContracts(spec MobileAppSpec) []ValidationError {
	var errs []ValidationError
	for index, contract := range spec.APIContracts {
		prefix := fmt.Sprintf("api_contracts.%d", index)
		if strings.TrimSpace(contract.Name) == "" {
			errs = append(errs, ValidationError{Field: prefix + ".name", Message: "API contract name is required"})
		}
		if strings.TrimSpace(contract.Method) == "" {
			errs = append(errs, ValidationError{Field: prefix + ".method", Message: "API contract method is required"})
		}
		if strings.TrimSpace(contract.Path) == "" {
			errs = append(errs, ValidationError{Field: prefix + ".path", Message: "API contract path is required"})
		}
	}
	return errs
}

func validatePermissionDescriptions(spec MobileAppSpec, platformSet map[MobilePlatform]struct{}) []ValidationError {
	if _, wantsIOS := platformSet[MobilePlatformIOS]; !wantsIOS {
		return nil
	}
	required := map[MobileCapability]string{
		CapabilityCamera:             "NSCameraUsageDescription",
		CapabilityPhotoLibrary:       "NSPhotoLibraryUsageDescription",
		CapabilityLocation:           "NSLocationWhenInUseUsageDescription",
		CapabilityPushNotifications:  "NSUserNotificationsUsageDescription",
		CapabilityLocalNotifications: "NSUserNotificationsUsageDescription",
	}
	var errs []ValidationError
	for _, capability := range spec.Capabilities {
		key, needsDescription := required[capability]
		if !needsDescription {
			continue
		}
		if strings.TrimSpace(spec.Permissions.IOSUsageDescriptions[key]) == "" {
			errs = append(errs, ValidationError{
				Field:   "permissions.ios_usage_descriptions." + key,
				Message: fmt.Sprintf("required when capability %q is enabled", capability),
			})
		}
	}
	return errs
}
