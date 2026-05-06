package mobile

import "strings"

func ClassifyTargetPlatform(prompt string) TargetPlatformClassification {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	if normalized == "" {
		return TargetPlatformClassification{
			TargetPlatform: TargetPlatformWeb,
			Confidence:     0.20,
			Rationale:      "empty prompt defaults to web until the user requests a target",
		}
	}

	if containsAny(normalized, "responsive website", "mobile responsive", "responsive web", "mobile-first website") &&
		!containsAny(normalized, "ios", "android", "apk", "app store", "google play", "testflight", "installed app") {
		return TargetPlatformClassification{
			TargetPlatform: TargetPlatformWeb,
			Confidence:     0.82,
			Rationale:      "prompt asks for responsive web rather than an installed native mobile app",
		}
	}

	capabilities := inferCapabilities(normalized)
	mobileSignals := []string{
		"ios", "android", "mobile app", "native app", "app store", "google play",
		"testflight", "apk", "aab", "ipa", "push notification", "camera", "gps",
		"location", "photo upload", "installed app", "eas build",
	}
	if containsAny(normalized, mobileSignals...) {
		return TargetPlatformClassification{
			TargetPlatform:       TargetPlatformMobileExpo,
			MobilePlatforms:      InferMobilePlatforms(normalized),
			Confidence:           0.91,
			Rationale:            "prompt requests native mobile platforms, store/binary outputs, or native capabilities",
			RequiredCapabilities: capabilities,
			BackendNeeded:        mobileBackendLikely(normalized),
		}
	}

	if containsAny(normalized, "capacitor", "wrap my web app", "convert existing web app to phone app") {
		return TargetPlatformClassification{
			TargetPlatform:  TargetPlatformMobileCapacitor,
			MobilePlatforms: InferMobilePlatforms(normalized),
			Confidence:      0.78,
			Rationale:       "prompt asks to wrap or convert an existing web app",
			BackendNeeded:   mobileBackendLikely(normalized),
		}
	}

	if containsAny(normalized, "api only", "backend api", "rest api", "graphql api") &&
		!containsAny(normalized, "frontend", "dashboard", "mobile", "ios", "android") {
		return TargetPlatformClassification{
			TargetPlatform: TargetPlatformAPIOnly,
			Confidence:     0.80,
			Rationale:      "prompt is API-only with no web or mobile client request",
			BackendNeeded:  true,
		}
	}

	if containsAny(normalized, "backend", "database", "auth", "login", "dashboard", "admin") {
		return TargetPlatformClassification{
			TargetPlatform: TargetPlatformFullstackWeb,
			Confidence:     0.70,
			Rationale:      "prompt asks for backend/auth/data features without native mobile signals",
			BackendNeeded:  true,
		}
	}

	return TargetPlatformClassification{
		TargetPlatform: TargetPlatformWeb,
		Confidence:     0.65,
		Rationale:      "no native mobile or backend-only signal detected",
	}
}

func InferMobilePlatforms(prompt string) []MobilePlatform {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	if normalized == "" {
		return nil
	}
	wantsAndroid := containsAny(normalized, "android", "apk", "aab", "google play", "play store")
	wantsIOS := containsAny(normalized, "ios", "iphone", "ipad", "ipa", "testflight", "app store")
	switch {
	case wantsAndroid && wantsIOS:
		return []MobilePlatform{MobilePlatformAndroid, MobilePlatformIOS}
	case wantsAndroid:
		return []MobilePlatform{MobilePlatformAndroid}
	case wantsIOS:
		return []MobilePlatform{MobilePlatformIOS}
	case containsAny(normalized, "mobile app", "native app", "installed app", "phone app"):
		return []MobilePlatform{MobilePlatformAndroid, MobilePlatformIOS}
	default:
		return nil
	}
}

func inferCapabilities(normalized string) []MobileCapability {
	var out []MobileCapability
	add := func(capability MobileCapability, signals ...string) {
		if containsAny(normalized, signals...) {
			out = append(out, capability)
		}
	}
	add(CapabilityCamera, "camera", "take photo", "scan")
	add(CapabilityPhotoLibrary, "photo library", "image picker", "upload photos", "photo upload")
	add(CapabilityPushNotifications, "push notification", "push reminder", "notifications")
	add(CapabilityLocation, "gps", "location", "nearby")
	add(CapabilityMaps, "map", "maps", "route")
	add(CapabilityFileUploads, "file upload", "upload file", "attachments")
	add(CapabilityOfflineMode, "offline", "local-first", "draft", "sync queue")
	add(CapabilityPayments, "payments", "checkout", "stripe")
	add(CapabilityBiometrics, "biometric", "face id", "touch id")
	add(CapabilityDeepLinks, "deep link", "deeplink")
	return out
}

func mobileBackendLikely(normalized string) bool {
	return containsAny(normalized,
		"login", "auth", "backend", "api", "database", "dashboard", "sync",
		"customers", "jobs", "orders", "payments", "file upload", "push",
	)
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
