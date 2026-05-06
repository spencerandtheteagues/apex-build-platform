package mobile

func FieldServiceContractorQuoteSpec() MobileAppSpec {
	return MobileAppSpec{
		App: MobileAppIdentity{
			Name:            "FieldOps Quote Builder",
			Slug:            "fieldops-quote-builder",
			Description:     "Contractor field-service quote builder with customers, jobs, estimates, photos, offline drafts, and push reminders.",
			TargetPlatforms: []MobilePlatform{MobilePlatformAndroid, MobilePlatformIOS},
			PrimaryUseCase:  "contractor quote builder",
			AppCategory:     "business",
			Audience:        "small contractors and field-service crews",
		},
		Identity: MobileBinaryIdentity{
			AndroidPackage:   "com.apexbuild.fieldops",
			IOSBundleID:      "com.apexbuild.fieldops",
			DisplayName:      "FieldOps Quotes",
			Version:          "1.0.0",
			VersionCode:      1,
			BuildNumber:      "1",
			IconAssetPath:    "./assets/icon.png",
			SplashAssetPath:  "./assets/splash.png",
			AdaptiveIconPath: "./assets/adaptive-icon.png",
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
			CapabilityLocalNotifications,
		},
		Screens: []MobileScreenSpec{
			{Route: "/(tabs)/jobs", Purpose: "List active jobs and sync state", Data: []string{"Job"}, Actions: []string{"open job", "create estimate", "attach photo"}, States: []string{"loading", "empty", "error", "offline"}, AuthRequired: true},
			{Route: "/(tabs)/customers", Purpose: "Manage customers", Data: []string{"Customer"}, Actions: []string{"call customer", "view jobs"}, States: []string{"loading", "empty", "error"}, AuthRequired: true},
			{Route: "/(tabs)/estimates", Purpose: "Build itemized quote drafts", Data: []string{"Estimate"}, Actions: []string{"save draft", "sync", "export PDF"}, States: []string{"draft", "pending sync", "synced"}, AuthRequired: true},
		},
		Navigation: MobileNavigationSpec{
			Tabs:      []string{"jobs", "customers", "estimates"},
			Stacks:    []string{"auth", "tabs"},
			Modals:    []string{"estimate-detail"},
			AuthGates: []string{"email_password_session"},
		},
		DataModels: []MobileDataModelSpec{
			{Name: "Customer", Fields: map[string]string{"id": "string", "name": "string", "phone": "string", "email": "string", "address": "string"}, Validation: []string{"name required"}},
			{Name: "Job", Fields: map[string]string{"id": "string", "customerId": "string", "title": "string", "status": "string", "urgency": "string"}, Relationships: map[string]string{"customer": "Customer"}},
			{Name: "Estimate", Fields: map[string]string{"id": "string", "jobId": "string", "laborHours": "number", "laborRate": "number", "materialsCost": "number", "markupPercent": "number", "finalPrice": "number"}, Relationships: map[string]string{"job": "Job"}},
		},
		APIContracts: []MobileAPIContractSpec{
			{Name: "login", Method: "POST", Path: "/api/auth/login", Request: "LoginRequest", Response: "AuthSession"},
			{Name: "list jobs", Method: "GET", Path: "/api/jobs", Response: "Job[]"},
			{Name: "sync estimate", Method: "POST", Path: "/api/estimates/sync", Request: "EstimateDraft", Response: "Estimate"},
			{Name: "upload job photo", Method: "POST", Path: "/api/jobs/:id/photos", Request: "multipart/form-data", Response: "PhotoAsset"},
		},
		Permissions: MobilePermissionSpec{
			IOSUsageDescriptions: map[string]string{
				"NSCameraUsageDescription":            "Attach job-site photos to customer jobs and estimates.",
				"NSPhotoLibraryUsageDescription":      "Select existing project photos for customer estimates.",
				"NSUserNotificationsUsageDescription": "Send quote follow-up reminders and schedule updates.",
			},
			AndroidPermissions: []string{"android.permission.CAMERA", "android.permission.POST_NOTIFICATIONS"},
		},
		Store: MobileStoreSpec{
			ShortDescription: "Create contractor quotes, track jobs, and sync field drafts.",
			FullDescription:  "FieldOps Quote Builder helps small contractors capture customer details, build estimates, attach job-site photos, and keep offline drafts ready to sync.",
			Keywords:         []string{"contractor", "field service", "estimates", "quotes"},
			Category:         "Business",
			ReleaseNotes:     "Initial internal build for field-service quote workflows.",
		},
		Quality: MobileQualitySpec{
			TestPlan:                  []string{"typecheck", "expo doctor", "auth smoke", "offline draft smoke", "photo upload smoke"},
			AccessibilityRequirements: []string{"touch targets at least 44px", "labels on primary actions"},
			PerformanceRequirements:   []string{"FlatList for job/customer lists", "avoid large unoptimized assets"},
			OfflineRequirements:       []string{"draft estimates survive restart", "pending sync state is visible"},
		},
	}
}
