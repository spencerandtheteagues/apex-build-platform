package mobile

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const (
	DefaultExpoSDKVersion        = "~55.0.0"
	DefaultReactVersion          = "^19.2.5"
	DefaultReactNativeVersion    = "^0.85.3"
	DefaultReactNativeWebVersion = "^0.21.2"
)

type SourceFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language"`
	Size     int64  `json:"size"`
	IsNew    bool   `json:"is_new"`
}

type ExpoGeneratorOptions struct {
	SDKVersion         string `json:"sdk_version,omitempty"`
	ReactVersion       string `json:"react_version,omitempty"`
	ReactNativeVersion string `json:"react_native_version,omitempty"`
}

func GenerateExpoProject(spec MobileAppSpec, options ExpoGeneratorOptions) ([]SourceFile, []ValidationError) {
	if errs := ValidateMobileAppSpec(spec); len(errs) > 0 {
		return nil, errs
	}
	if spec.Architecture.FrontendFramework != MobileFrameworkExpoReactNative {
		return nil, []ValidationError{{Field: "architecture.frontend_framework", Message: "Expo generator requires expo-react-native"}}
	}
	options = normalizeExpoGeneratorOptions(options)
	dependencies := ExpoDependenciesForSpec(spec, options)
	if errs := ValidateExpoDependencyPolicy(dependencies, DefaultNativeCapabilityRegistry()); len(errs) > 0 {
		return nil, errs
	}

	files := []SourceFile{
		sourceFile("mobile/package.json", packageJSON(spec, dependencies), "json"),
		sourceFile("mobile/app.config.ts", appConfigTS(spec), "typescript"),
		sourceFile("mobile/eas.json", easJSON(), "json"),
		sourceFile("mobile/tsconfig.json", tsconfigJSON(), "json"),
		sourceFile("mobile/.env.example", envExample(spec), "dotenv"),
		sourceFile("mobile/README.md", mobileReadme(spec), "markdown"),
		sourceFile("mobile/BUILD.md", buildMd(spec), "markdown"),
		sourceFile("mobile/STORE_RELEASE.md", storeReleaseMd(spec), "markdown"),
		sourceFile("mobile/docs/api-contract.json", APIContractManifestJSON(spec), "json"),
		sourceFile("mobile/docs/api-contract.md", APIContractMarkdown(spec), "markdown"),
		sourceFile("mobile/store/store-readiness.json", StoreReadinessJSON(spec), "json"),
		sourceFile("mobile/store/privacy-data-safety.md", StorePrivacyDataSafetyMarkdown(spec), "markdown"),
		sourceFile("mobile/store/screenshot-checklist.md", StoreScreenshotChecklistMarkdown(spec), "markdown"),
		sourceFile("mobile/store/release-notes.md", StoreReleaseNotesMarkdown(spec), "markdown"),
		pngSourceFile("mobile/assets/icon.png"),
		pngSourceFile("mobile/assets/splash.png"),
		pngSourceFile("mobile/assets/adaptive-icon.png"),
		sourceFile("mobile/app/_layout.tsx", rootLayoutTSX(), "typescript"),
		sourceFile("mobile/app/index.tsx", indexTSX(), "typescript"),
		sourceFile("mobile/app/(auth)/login.tsx", loginTSX(), "typescript"),
		sourceFile("mobile/app/(tabs)/_layout.tsx", tabsLayoutTSX(), "typescript"),
		sourceFile("mobile/app/(tabs)/jobs.tsx", jobsScreenTSX(), "typescript"),
		sourceFile("mobile/app/(tabs)/customers.tsx", customersScreenTSX(), "typescript"),
		sourceFile("mobile/app/(tabs)/estimates.tsx", estimatesScreenTSX(), "typescript"),
		sourceFile("mobile/app/modals/estimate-detail.tsx", estimateDetailTSX(), "typescript"),
		sourceFile("mobile/src/api/client.ts", apiClientTS(), "typescript"),
		sourceFile("mobile/src/api/endpoints.ts", apiEndpointsTS(spec), "typescript"),
		sourceFile("mobile/src/api/types.ts", apiTypesTS(spec), "typescript"),
		sourceFile("mobile/src/auth/AuthProvider.tsx", authProviderTSX(), "typescript"),
		sourceFile("mobile/src/components/feedback/EmptyState.tsx", emptyStateTSX(), "typescript"),
		sourceFile("mobile/src/components/ui/Screen.tsx", screenTSX(), "typescript"),
		sourceFile("mobile/src/data/localStore.ts", localStoreTS(), "typescript"),
		sourceFile("mobile/src/data/syncQueue.ts", syncQueueTS(), "typescript"),
		sourceFile("mobile/src/features/fieldService/sampleData.ts", sampleDataTS(), "typescript"),
		sourceFile("mobile/src/permissions/nativeCapabilities.ts", nativeCapabilitiesTS(spec), "typescript"),
		sourceFile("mobile/src/theme/theme.ts", themeTS(), "typescript"),
	}
	files = append(files, backendContractSourceFiles(spec)...)
	if errs := ValidateGeneratedExpoFiles(files, dependencies, DefaultNativeCapabilityRegistry()); len(errs) > 0 {
		return nil, errs
	}
	return files, nil
}

func normalizeExpoGeneratorOptions(options ExpoGeneratorOptions) ExpoGeneratorOptions {
	if strings.TrimSpace(options.SDKVersion) == "" {
		options.SDKVersion = DefaultExpoSDKVersion
	}
	if strings.TrimSpace(options.ReactVersion) == "" {
		options.ReactVersion = DefaultReactVersion
	}
	if strings.TrimSpace(options.ReactNativeVersion) == "" {
		options.ReactNativeVersion = DefaultReactNativeVersion
	}
	return options
}

func ExpoDependenciesForSpec(spec MobileAppSpec, options ExpoGeneratorOptions) map[string]string {
	options = normalizeExpoGeneratorOptions(options)
	dependencies := map[string]string{
		"expo":                           options.SDKVersion,
		"expo-constants":                 "^55.0.13",
		"expo-linking":                   "^55.0.15",
		"expo-router":                    "^55.0.14",
		"expo-secure-store":              "^55.0.13",
		"react":                          options.ReactVersion,
		"react-dom":                      options.ReactVersion,
		"react-native":                   options.ReactNativeVersion,
		"react-native-safe-area-context": "^5.7.0",
		"react-native-screens":           "^4.24.0",
		"react-native-web":               DefaultReactNativeWebVersion,
		"@expo/metro-runtime":            "^55.0.11",
		"@react-native-async-storage/async-storage": "^3.0.2",
		"zod":                   "^4.4.3",
		"zustand":               "^5.0.13",
		"@tanstack/react-query": "^5.100.9",
	}
	defaultVersions := defaultExpoDependencyVersions(options)
	for _, capability := range spec.Capabilities {
		definition, ok := DefaultNativeCapabilityRegistry().Definition(capability)
		if !ok {
			continue
		}
		for _, pkg := range definition.AllowedPackages {
			if _, exists := dependencies[pkg]; !exists {
				dependencies[pkg] = defaultVersions[pkg]
			}
		}
	}
	return dependencies
}

func defaultExpoDependencyVersions(options ExpoGeneratorOptions) map[string]string {
	options = normalizeExpoGeneratorOptions(options)
	return map[string]string{
		"@expo/metro-runtime":                       "^55.0.11",
		"@react-native-async-storage/async-storage": "^3.0.2",
		"@tanstack/react-query":                     "^5.100.9",
		"expo":                                      options.SDKVersion,
		"expo-camera":                               "^55.0.18",
		"expo-constants":                            "^55.0.13",
		"expo-device":                               "^55.0.16",
		"expo-document-picker":                      "^55.0.13",
		"expo-file-system":                          "^55.0.19",
		"expo-image-picker":                         "^55.0.20",
		"expo-linking":                              "^55.0.15",
		"expo-location":                             "^55.0.14",
		"expo-notifications":                        "^55.0.22",
		"expo-print":                                "^55.0.21",
		"expo-router":                               "^55.0.14",
		"expo-secure-store":                         "^55.0.13",
		"expo-sharing":                              "^55.0.18",
		"expo-sqlite":                               "^55.0.15",
		"expo-updates":                              "^55.1.9",
		"expo-web-browser":                          "^55.0.14",
		"react":                                     options.ReactVersion,
		"react-dom":                                 options.ReactVersion,
		"react-hook-form":                           "^7.75.0",
		"react-native":                              options.ReactNativeVersion,
		"react-native-safe-area-context":            "^5.7.0",
		"react-native-screens":                      "^4.24.0",
		"react-native-web":                          DefaultReactNativeWebVersion,
		"zod":                                       "^4.4.3",
		"zustand":                                   "^5.0.13",
	}
}

func ValidateExpoDependencyPolicy(dependencies map[string]string, registry NativeCapabilityRegistry) []ValidationError {
	var errs []ValidationError
	for pkg := range dependencies {
		if !registry.PackageAllowed(pkg) {
			errs = append(errs, ValidationError{Field: "dependencies." + pkg, Message: "package is not allowed for generated Expo mobile apps"})
		}
		if strings.TrimSpace(dependencies[pkg]) == "" || dependencies[pkg] == "*" || dependencies[pkg] == "latest" {
			errs = append(errs, ValidationError{Field: "dependencies." + pkg, Message: "package version must be pinned to a deterministic supported range"})
		}
	}
	return errs
}

func ValidateGeneratedExpoFiles(files []SourceFile, dependencies map[string]string, registry NativeCapabilityRegistry) []ValidationError {
	errs := ValidateExpoDependencyPolicy(dependencies, registry)
	blocked := []string{"window.", "document.", "localStorage", "sessionStorage", "process.env."}
	for _, file := range files {
		if !strings.HasPrefix(filepath.ToSlash(file.Path), "mobile/") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file.Path))
		if ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".jsx" {
			continue
		}
		for _, token := range blocked {
			if token == "process.env." && file.Path == "mobile/app.config.ts" {
				continue
			}
			if strings.Contains(file.Content, token) {
				errs = append(errs, ValidationError{Field: file.Path, Message: "generated mobile code contains browser-only API: " + token})
			}
		}
	}
	return errs
}

func sourceFile(path, content, language string) SourceFile {
	return SourceFile{Path: path, Content: content, Language: language, Size: int64(len(content)), IsNew: true}
}

func pngSourceFile(path string) SourceFile {
	content := string([]byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0x60, 0x18, 0x05, 0x00,
		0x01, 0x0c, 0x00, 0x01, 0x5f, 0x91, 0x8d, 0xb8,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44,
		0xae, 0x42, 0x60, 0x82,
	})
	return SourceFile{Path: path, Content: content, Language: "image/png", Size: int64(len(content)), IsNew: true}
}

func packageJSON(spec MobileAppSpec, dependencies map[string]string) string {
	dependencyKeys := make([]string, 0, len(dependencies))
	for key := range dependencies {
		dependencyKeys = append(dependencyKeys, key)
	}
	sort.Strings(dependencyKeys)
	sortedDependencies := map[string]string{}
	for _, key := range dependencyKeys {
		sortedDependencies[key] = dependencies[key]
	}
	payload := map[string]any{
		"name":    spec.App.Slug,
		"version": spec.Identity.Version,
		"private": true,
		"main":    "expo-router/entry",
		"scripts": map[string]string{
			"start":     "expo start",
			"android":   "expo run:android",
			"ios":       "expo run:ios",
			"web":       "expo start --web",
			"typecheck": "tsc --noEmit",
			"doctor":    "npx expo-doctor",
		},
		"dependencies": sortedDependencies,
		"devDependencies": map[string]string{
			"@types/react": "~19.2.0",
			"typescript":   "~5.9.0",
		},
	}
	encoded, _ := json.MarshalIndent(payload, "", "  ")
	return string(encoded) + "\n"
}

func appConfigTS(spec MobileAppSpec) string {
	androidPermissions, _ := json.Marshal(spec.Permissions.AndroidPermissions)
	iosInfoPlist, _ := json.Marshal(spec.Permissions.IOSUsageDescriptions)
	return fmt.Sprintf(`import type { ExpoConfig } from 'expo/config';

const config: ExpoConfig = {
  name: %q,
  slug: %q,
  version: %q,
  scheme: %q,
  orientation: 'portrait',
  userInterfaceStyle: 'automatic',
  newArchEnabled: true,
  icon: './assets/icon.png',
  splash: {
    image: './assets/splash.png',
    resizeMode: 'contain',
    backgroundColor: '#0F172A'
  },
  assetBundlePatterns: ['**/*'],
  ios: {
    supportsTablet: true,
    bundleIdentifier: %q,
    buildNumber: %q,
    infoPlist: %s
  },
  android: {
    package: %q,
    versionCode: %d,
    adaptiveIcon: {
      foregroundImage: './assets/adaptive-icon.png',
      backgroundColor: '#0F172A'
    },
    permissions: %s
  },
  plugins: ['expo-router', 'expo-secure-store'],
  extra: {
    apiBaseUrl: process.env.EXPO_PUBLIC_API_BASE_URL,
    eas: {
      projectId: process.env.EXPO_PUBLIC_EAS_PROJECT_ID
    }
  }
};

export default config;
`, spec.Identity.DisplayName, spec.App.Slug, spec.Identity.Version, spec.App.Slug, spec.Identity.IOSBundleID, firstNonEmptyString(spec.Identity.BuildNumber, "1"), string(iosInfoPlist), spec.Identity.AndroidPackage, maxInt(spec.Identity.VersionCode, 1), string(androidPermissions))
}

func easJSON() string {
	return `{
  "cli": {
    "version": ">= 16.0.0",
    "appVersionSource": "remote"
  },
  "build": {
    "development": {
      "developmentClient": true,
      "distribution": "internal"
    },
    "preview": {
      "distribution": "internal",
      "android": {
        "buildType": "apk"
      }
    },
    "internal": {
      "distribution": "internal"
    },
    "production": {
      "autoIncrement": true,
      "android": {
        "buildType": "app-bundle"
      }
    }
  },
  "submit": {
    "production": {}
  }
}
`
}

func tsconfigJSON() string {
	return `{
  "extends": "expo/tsconfig.base",
  "compilerOptions": {
    "strict": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"]
    }
  },
  "include": ["app", "src", "app.config.ts"]
}
`
}

func envExample(spec MobileAppSpec) string {
	return fmt.Sprintf("EXPO_PUBLIC_API_BASE_URL=https://api.example.com\nEXPO_PUBLIC_EAS_PROJECT_ID=\nAPP_VARIANT=development\nAPP_SLUG=%s\n", spec.App.Slug)
}

func mobileReadme(spec MobileAppSpec) string {
	return fmt.Sprintf(`# %s

Generated Expo/React Native mobile client.

This source package is not proof of an Android/iOS binary or store approval. Build, signing, and submission are separate steps.

## Start

1. Install dependencies with npm install.
2. Copy .env.example to .env.
3. Run npm run start.
4. Use npm run web for a browser-rendered mobile preview only.

## Target

- Framework: Expo React Native
- Android package: %s
- iOS bundle identifier: %s
- Release level: source only
`, spec.Identity.DisplayName, spec.Identity.AndroidPackage, spec.Identity.IOSBundleID)
}

func buildMd(spec MobileAppSpec) string {
	return fmt.Sprintf(`# Build Guide

## Source Validation

- npm run typecheck
- npm run doctor
- npm run web

## Native Builds

Native builds require EAS credentials and signing setup. Apex must not claim an APK, AAB, iOS internal build, TestFlight-ready build, or store-submission-ready package until those jobs finish successfully.

## Identifiers

- Android package: %s
- iOS bundle identifier: %s
`, spec.Identity.AndroidPackage, spec.Identity.IOSBundleID)
}

func storeReleaseMd(spec MobileAppSpec) string {
	return fmt.Sprintf(`# Store Release Checklist

## Metadata Draft

- App name: %s
- Short description: %s
- Category: %s

## Required Before Submission

- Production icon and splash assets
- Privacy policy URL
- Support URL
- Screenshots for required device sizes
- Android data safety answers
- Apple privacy nutrition label answers
- Signed Android/iOS build artifacts
- Store-console processing complete

## Generated Store Package

- mobile/store/store-readiness.json
- mobile/store/privacy-data-safety.md
- mobile/store/screenshot-checklist.md
- mobile/store/release-notes.md

These files are launch-preparation drafts. They do not mean the app has been uploaded, reviewed, approved, or released by Apple or Google.
`, spec.Identity.DisplayName, spec.Store.ShortDescription, spec.Store.Category)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func maxInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
