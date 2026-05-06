package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"apex-build/internal/mobile"
	"apex-build/pkg/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newExportHandlerTestFixture(t *testing.T) (*ExportHandler, uint, *gorm.DB) {
	t.Helper()

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Project{}, &models.File{}, &models.CompletedBuild{}))

	user := models.User{
		Username:     strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_"),
		Email:        strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_") + "@example.com",
		PasswordHash: "hashed-password",
	}
	require.NoError(t, db.Create(&user).Error)

	return NewExportHandler(db, nil, nil), user.ID, db
}

func TestPrepareMobileExpoExportFilesUsesPersistedSpecAndPreservesExistingFiles(t *testing.T) {
	handler, userID, db := newExportHandlerTestFixture(t)

	project := models.Project{
		Name:           "Mobile Export",
		Language:       "typescript",
		OwnerID:        userID,
		TargetPlatform: string(mobile.TargetPlatformMobileExpo),
	}
	require.NoError(t, db.Create(&project).Error)
	require.NoError(t, db.Create(&models.File{
		ProjectID: project.ID,
		Path:      "mobile/package.json",
		Name:      "package.json",
		Type:      "file",
		Content:   `{"name":"keep-existing"}`,
		Size:      int64(len(`{"name":"keep-existing"}`)),
	}).Error)

	spec := mobile.FieldServiceContractorQuoteSpec()
	spec.App.Name = "Persisted Export Spec"
	spec.App.Slug = "persisted-export-spec"
	spec.Identity.DisplayName = "Persisted Export Spec"
	specJSON, err := json.Marshal(spec)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.CompletedBuild{
		BuildID:        "build-mobile-export",
		UserID:         userID,
		ProjectID:      &project.ID,
		Status:         "completed",
		TargetPlatform: string(mobile.TargetPlatformMobileExpo),
		MobileSpecJSON: string(specJSON),
	}).Error)

	require.NoError(t, mobile.PrepareExpoProjectFiles(context.Background(), handler.db, project))

	var packageFile models.File
	require.NoError(t, db.Where("project_id = ? AND path = ?", project.ID, "mobile/package.json").First(&packageFile).Error)
	require.JSONEq(t, `{"name":"keep-existing"}`, packageFile.Content)

	var configFile models.File
	require.NoError(t, db.Where("project_id = ? AND path = ?", project.ID, "mobile/app.config.ts").First(&configFile).Error)
	require.Contains(t, configFile.Content, "Persisted Export Spec")
}

func TestPrepareMobileExpoExportFilesSkipsWebProjectWithoutMobileMetadata(t *testing.T) {
	handler, userID, db := newExportHandlerTestFixture(t)

	project := models.Project{
		Name:           "Web Export",
		Language:       "typescript",
		OwnerID:        userID,
		TargetPlatform: string(mobile.TargetPlatformWeb),
	}
	require.NoError(t, db.Create(&project).Error)

	require.NoError(t, mobile.PrepareExpoProjectFiles(context.Background(), handler.db, project))

	var fileCount int64
	require.NoError(t, db.Model(&models.File{}).Where("project_id = ?", project.ID).Count(&fileCount).Error)
	require.Zero(t, fileCount)
}

func TestPrepareMobileExpoExportFilesUsesMobileCompletedBuildForWebProject(t *testing.T) {
	handler, userID, db := newExportHandlerTestFixture(t)

	project := models.Project{
		Name:           "Build Mobile Export",
		Language:       "typescript",
		OwnerID:        userID,
		TargetPlatform: string(mobile.TargetPlatformWeb),
	}
	require.NoError(t, db.Create(&project).Error)
	require.NoError(t, db.Create(&models.CompletedBuild{
		BuildID:        "build-only-mobile-export",
		UserID:         userID,
		ProjectID:      &project.ID,
		Status:         "completed",
		TargetPlatform: string(mobile.TargetPlatformMobileExpo),
	}).Error)

	require.NoError(t, mobile.PrepareExpoProjectFiles(context.Background(), handler.db, project))

	var fileCount int64
	require.NoError(t, db.Model(&models.File{}).Where("project_id = ? AND path LIKE ?", project.ID, "mobile/%").Count(&fileCount).Error)
	require.Positive(t, fileCount)
}
