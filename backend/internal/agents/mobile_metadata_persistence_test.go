package agents

import (
	"testing"
	"time"

	"apex-build/internal/mobile"
	"apex-build/pkg/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPersistBuildSnapshotStoresMobileMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:mobile-metadata?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.CompletedBuild{}); err != nil {
		t.Fatalf("migrate completed builds: %v", err)
	}

	now := time.Now().UTC()
	am := &AgentManager{db: db}
	build := &Build{
		ID:                 "build-mobile-metadata",
		UserID:             42,
		Status:             BuildCompleted,
		Mode:               ModeFull,
		PowerMode:          PowerBalanced,
		Description:        "Build an Android and iOS field service mobile app",
		TargetPlatform:     mobile.TargetPlatformMobileExpo,
		MobilePlatforms:    []mobile.MobilePlatform{mobile.MobilePlatformAndroid, mobile.MobilePlatformIOS},
		MobileFramework:    mobile.MobileFrameworkExpoReactNative,
		MobileReleaseLevel: mobile.ReleaseSourceOnly,
		MobileCapabilities: []mobile.MobileCapability{mobile.CapabilityCamera, mobile.CapabilityOfflineMode},
		MobileAppSpec: &mobile.MobileAppSpec{
			App: mobile.MobileAppIdentity{
				Name:            "FieldOps Mobile",
				Slug:            "fieldops-mobile",
				TargetPlatforms: []mobile.MobilePlatform{mobile.MobilePlatformAndroid, mobile.MobilePlatformIOS},
			},
			Identity: mobile.MobileBinaryIdentity{
				AndroidPackage: "com.apexbuild.fieldops",
				IOSBundleID:    "com.apexbuild.fieldops",
				DisplayName:    "FieldOps Mobile",
				Version:        "1.0.0",
				VersionCode:    7,
				BuildNumber:    "7",
			},
			Architecture: mobile.MobileArchitecture{
				FrontendFramework: mobile.MobileFrameworkExpoReactNative,
				BackendMode:       mobile.BackendNewGenerated,
				AuthMode:          mobile.AuthEmailPassword,
				DatabaseMode:      mobile.DatabaseHybridOffline,
			},
		},
		Plan: &BuildPlan{
			AppType:      "fullstack",
			DeliveryMode: "mobile_source_only",
			TechStack:    TechStack{Frontend: "Expo React Native", Backend: "Go"},
		},
		Agents:      map[string]*Agent{},
		Tasks:       []*Task{},
		Checkpoints: []*Checkpoint{},
		Progress:    100,
		CreatedAt:   now.Add(-2 * time.Minute),
		UpdatedAt:   now,
		CompletedAt: &now,
	}
	files := []GeneratedFile{{Path: "mobile/package.json", Content: "{}", Language: "json", IsNew: true}}

	if err := am.persistBuildSnapshotCritical(build, files); err != nil {
		t.Fatalf("persist first snapshot: %v", err)
	}
	build.MobileCapabilities = append(build.MobileCapabilities, mobile.CapabilityPushNotifications)
	if err := am.persistBuildSnapshotCritical(build, files); err != nil {
		t.Fatalf("persist updated snapshot: %v", err)
	}

	var snapshot models.CompletedBuild
	if err := db.Where("build_id = ?", build.ID).First(&snapshot).Error; err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if snapshot.TargetPlatform != string(mobile.TargetPlatformMobileExpo) {
		t.Fatalf("target platform = %q", snapshot.TargetPlatform)
	}
	if len(snapshot.MobilePlatforms) != 2 {
		t.Fatalf("mobile platforms = %+v", snapshot.MobilePlatforms)
	}
	if snapshot.MobileFramework != string(mobile.MobileFrameworkExpoReactNative) {
		t.Fatalf("mobile framework = %q", snapshot.MobileFramework)
	}
	if !stringSliceContains(snapshot.MobileCapabilities, string(mobile.CapabilityPushNotifications)) {
		t.Fatalf("expected updated capability persisted through upsert, got %+v", snapshot.MobileCapabilities)
	}
	if snapshot.AndroidPackage != "com.apexbuild.fieldops" || snapshot.IOSBundleIdentifier != "com.apexbuild.fieldops" {
		t.Fatalf("expected binary identifiers persisted, got android=%q ios=%q", snapshot.AndroidPackage, snapshot.IOSBundleIdentifier)
	}
	if snapshot.MobileSpecJSON == "" {
		t.Fatal("expected mobile spec json to be stored")
	}
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
