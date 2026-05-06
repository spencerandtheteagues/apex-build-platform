package mobile

type CapabilityDefinition struct {
	Capability                 MobileCapability `json:"capability"`
	AllowedPackages            []string         `json:"allowed_packages"`
	AppConfigRequirements      []string         `json:"app_config_requirements,omitempty"`
	IOSPermissionKeys          []string         `json:"ios_permission_keys,omitempty"`
	AndroidPermissions         []string         `json:"android_permissions,omitempty"`
	DevelopmentRebuildRequired bool             `json:"development_rebuild_required"`
	ExpoGoSupported            bool             `json:"expo_go_supported"`
	EASBuildRequired           bool             `json:"eas_build_required"`
	CommonFailureModes         []string         `json:"common_failure_modes,omitempty"`
	RepairInstructions         []string         `json:"repair_instructions,omitempty"`
}

type NativeCapabilityRegistry struct {
	byCapability    map[MobileCapability]CapabilityDefinition
	allowedPackages map[string]struct{}
}

func DefaultNativeCapabilityRegistry() NativeCapabilityRegistry {
	definitions := []CapabilityDefinition{
		{
			Capability:                 CapabilityCamera,
			AllowedPackages:            []string{"expo-camera", "expo-image-picker"},
			IOSPermissionKeys:          []string{"NSCameraUsageDescription"},
			AndroidPermissions:         []string{"android.permission.CAMERA"},
			DevelopmentRebuildRequired: true,
			ExpoGoSupported:            false,
			EASBuildRequired:           true,
			CommonFailureModes:         []string{"missing iOS camera usage description", "missing Android camera permission"},
			RepairInstructions:         []string{"add app config permission text", "keep camera code behind permission checks"},
		},
		{
			Capability:                 CapabilityPhotoLibrary,
			AllowedPackages:            []string{"expo-image-picker", "expo-file-system"},
			IOSPermissionKeys:          []string{"NSPhotoLibraryUsageDescription"},
			DevelopmentRebuildRequired: true,
			ExpoGoSupported:            false,
			EASBuildRequired:           true,
		},
		{
			Capability:                 CapabilityPushNotifications,
			AllowedPackages:            []string{"expo-notifications", "expo-device", "expo-constants"},
			AppConfigRequirements:      []string{"notification icon/color", "project ID"},
			IOSPermissionKeys:          []string{"NSUserNotificationsUsageDescription"},
			AndroidPermissions:         []string{"android.permission.POST_NOTIFICATIONS"},
			DevelopmentRebuildRequired: true,
			ExpoGoSupported:            false,
			EASBuildRequired:           true,
			CommonFailureModes:         []string{"missing EAS project ID", "missing push credentials", "notification permission not requested"},
		},
		{
			Capability:                 CapabilityLocalNotifications,
			AllowedPackages:            []string{"expo-notifications", "expo-device", "expo-constants"},
			AppConfigRequirements:      []string{"notification icon/color"},
			IOSPermissionKeys:          []string{"NSUserNotificationsUsageDescription"},
			AndroidPermissions:         []string{"android.permission.POST_NOTIFICATIONS"},
			DevelopmentRebuildRequired: true,
			ExpoGoSupported:            false,
			EASBuildRequired:           true,
			CommonFailureModes:         []string{"notification permission not requested", "local scheduling not tested in a native build"},
		},
		{
			Capability:                 CapabilityLocation,
			AllowedPackages:            []string{"expo-location"},
			IOSPermissionKeys:          []string{"NSLocationWhenInUseUsageDescription"},
			AndroidPermissions:         []string{"android.permission.ACCESS_FINE_LOCATION", "android.permission.ACCESS_COARSE_LOCATION"},
			DevelopmentRebuildRequired: true,
			ExpoGoSupported:            false,
			EASBuildRequired:           true,
		},
		{
			Capability:                 CapabilityOfflineMode,
			AllowedPackages:            []string{"expo-sqlite", "@react-native-async-storage/async-storage", "zustand", "@tanstack/react-query"},
			DevelopmentRebuildRequired: true,
			ExpoGoSupported:            false,
			EASBuildRequired:           true,
		},
		{
			Capability:                 CapabilityFileUploads,
			AllowedPackages:            []string{"expo-file-system", "expo-document-picker", "expo-sharing"},
			DevelopmentRebuildRequired: true,
			ExpoGoSupported:            false,
			EASBuildRequired:           true,
		},
	}

	registry := NativeCapabilityRegistry{
		byCapability:    map[MobileCapability]CapabilityDefinition{},
		allowedPackages: baseAllowedPackages(),
	}
	for _, definition := range definitions {
		registry.byCapability[definition.Capability] = definition
		for _, pkg := range definition.AllowedPackages {
			registry.allowedPackages[pkg] = struct{}{}
		}
	}
	return registry
}

func baseAllowedPackages() map[string]struct{} {
	packages := []string{
		"expo", "expo-router", "@react-navigation/native", "@react-navigation/native-stack",
		"react", "react-dom", "react-native", "react-native-web", "react-native-safe-area-context", "react-native-screens",
		"expo-secure-store", "expo-sqlite", "expo-file-system", "expo-image-picker",
		"expo-camera", "expo-device", "expo-location", "expo-notifications", "expo-linking",
		"expo-web-browser", "expo-document-picker", "expo-sharing", "expo-print",
		"expo-updates", "expo-constants", "@expo/metro-runtime", "@react-native-async-storage/async-storage",
		"zod", "react-hook-form", "zustand", "@tanstack/react-query",
	}
	out := make(map[string]struct{}, len(packages))
	for _, pkg := range packages {
		out[pkg] = struct{}{}
	}
	return out
}

func (r NativeCapabilityRegistry) Definition(capability MobileCapability) (CapabilityDefinition, bool) {
	definition, ok := r.byCapability[capability]
	return definition, ok
}

func (r NativeCapabilityRegistry) PackageAllowed(pkg string) bool {
	_, ok := r.allowedPackages[pkg]
	return ok
}
