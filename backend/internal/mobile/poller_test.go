package mobile

import (
	"context"
	"strings"
	"testing"
	"time"

	"apex-build/pkg/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMobileBuildPollerRefreshesJobsAndUpdatesProjectSummary(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Project{}, &MobileBuildRecord{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	project := models.Project{
		Name:              "Mobile Project",
		Language:          "typescript",
		TargetPlatform:    string(TargetPlatformMobileExpo),
		MobilePlatforms:   []string{string(MobilePlatformAndroid)},
		MobileBuildStatus: string(MobileBuildBuilding),
		MobileMetadata:    map[string]interface{}{},
		OwnerID:           9,
	}
	if err := db.Create(&project).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	store := NewGormMobileBuildStore(db)
	if err := store.Save(context.Background(), MobileBuildJob{
		ID:              "mbld_poller",
		ProjectID:       project.ID,
		UserID:          project.OwnerID,
		Platform:        MobilePlatformAndroid,
		Profile:         MobileBuildProfilePreview,
		ReleaseLevel:    ReleaseInternalAndroidAPK,
		Status:          MobileBuildBuilding,
		Provider:        "mock-eas",
		ProviderBuildID: "eas-build-poller",
		CreatedAt:       now.Add(-10 * time.Minute),
		UpdatedAt:       now.Add(-2 * time.Minute),
	}); err != nil {
		t.Fatalf("save build job: %v", err)
	}

	provider := &mockMobileBuildProvider{
		name: "mock-eas",
		refreshResult: MobileBuildProviderResult{
			Status:      MobileBuildSucceeded,
			ArtifactURL: "https://artifacts.example.com/poller.apk",
			Logs: []MobileBuildLogLine{{
				Level:   "info",
				Message: "finished with EAS_TOKEN=should-redact",
			}},
		},
	}
	service := NewMobileBuildService(
		mobileBuildTestFlags(),
		provider,
		store,
		WithMobileBuildClock(func() time.Time { return now }),
	)
	poller := NewMobileBuildPoller(db, service, MobileBuildPollerConfig{
		MinAge:    30 * time.Second,
		BatchSize: 10,
	})

	result, err := poller.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run poller: %v", err)
	}
	if result.Refreshed != 1 || result.Errors != 0 {
		t.Fatalf("unexpected poll result %+v", result)
	}
	if provider.refreshCalls != 1 {
		t.Fatalf("expected one provider refresh call, got %d", provider.refreshCalls)
	}

	var updated models.Project
	if err := db.First(&updated, project.ID).Error; err != nil {
		t.Fatalf("load updated project: %v", err)
	}
	if updated.MobileBuildStatus != string(MobileBuildSucceeded) {
		t.Fatalf("expected project build status succeeded, got %+v", updated)
	}
	if got, _ := updated.MobileMetadata["android_apk_url"].(string); got != "https://artifacts.example.com/poller.apk" {
		t.Fatalf("expected android apk artifact metadata, got %+v", updated.MobileMetadata)
	}
	if message, _ := updated.MobileMetadata["last_mobile_build_failure_message"].(string); strings.Contains(message, "should-redact") {
		t.Fatalf("expected redacted project metadata, got %+v", updated.MobileMetadata)
	}
}
