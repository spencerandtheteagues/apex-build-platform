package mobile

import (
	"context"
	"strings"
	"testing"

	"apex-build/pkg/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestNormalizeGeneratedMobileProjectExportPathAllowsOnlyGeneratedRoots(t *testing.T) {
	tests := []struct {
		path string
		want string
		ok   bool
	}{
		{path: "mobile/package.json", want: "mobile/package.json", ok: true},
		{path: "/backend/src/server.ts", want: "backend/src/server.ts", ok: true},
		{path: "docs/mobile-backend-routes.md", want: "docs/mobile-backend-routes.md", ok: true},
		{path: "frontend/src/App.tsx", ok: false},
		{path: "../backend/src/server.ts", ok: false},
		{path: "backend/../secrets.env", ok: false},
		{path: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, ok := normalizeGeneratedMobileProjectExportPath(tt.path)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("normalizeGeneratedMobileProjectExportPath(%q) = (%q, %v), want (%q, %v)", tt.path, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPrepareExpoProjectFilesPersistsMobileAndBackendContractFiles(t *testing.T) {
	db := openMobileExportTestDB(t)
	user := models.User{Username: "mobile-owner", Email: "mobile-owner@example.test", PasswordHash: "hash"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	project := models.Project{
		Name:               "Mobile Export",
		Language:           "typescript",
		TargetPlatform:     string(TargetPlatformMobileExpo),
		MobileReleaseLevel: string(ReleaseSourceOnly),
		OwnerID:            user.ID,
	}
	if err := db.Create(&project).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}

	if err := PrepareExpoProjectFiles(context.Background(), db, project); err != nil {
		t.Fatalf("prepare Expo project files: %v", err)
	}

	for _, path := range []string{
		"mobile/docs/api-contract.json",
		"mobile/src/api/endpoints.ts",
		"backend/src/mobileContractRoutes.ts",
		"backend/src/server.ts",
		"docs/mobile-backend-routes.md",
	} {
		var file models.File
		if err := db.Where("project_id = ? AND path = ?", project.ID, path).First(&file).Error; err != nil {
			t.Fatalf("expected generated export file %s: %v", path, err)
		}
		if strings.TrimSpace(file.Content) == "" {
			t.Fatalf("expected generated export file %s to have content", path)
		}
	}
}

func openMobileExportTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Project{}, &models.File{}, &models.CompletedBuild{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}
