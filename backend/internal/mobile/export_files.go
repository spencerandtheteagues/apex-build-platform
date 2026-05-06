package mobile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"apex-build/pkg/models"

	"gorm.io/gorm"
)

func PrepareExpoProjectFiles(ctx context.Context, db *gorm.DB, project models.Project) error {
	if db == nil {
		return nil
	}

	spec, shouldGenerate, err := ExpoSpecForProjectExport(ctx, db, project)
	if err != nil || !shouldGenerate {
		return err
	}

	files, validationErrors := GenerateExpoProject(spec, ExpoGeneratorOptions{})
	if len(validationErrors) > 0 {
		return fmt.Errorf("generated Expo project failed validation: %s", FormatValidationErrors(validationErrors))
	}

	for _, file := range files {
		path, ok := normalizeGeneratedMobilePath(file.Path)
		if !ok {
			return fmt.Errorf("generated Expo project produced unsafe path %q", file.Path)
		}

		var existing models.File
		if err := db.WithContext(ctx).
			Where("project_id = ? AND path = ?", project.ID, path).
			First(&existing).Error; err == nil {
			continue
		} else if err != gorm.ErrRecordNotFound {
			return err
		}

		contentHash := sha256.Sum256([]byte(file.Content))
		projectFile := models.File{
			ProjectID:  project.ID,
			Path:       path,
			Name:       filepath.Base(path),
			Type:       "file",
			MimeType:   MimeTypeForSourceFile(file),
			Content:    file.Content,
			Size:       int64(len(file.Content)),
			Hash:       hex.EncodeToString(contentHash[:]),
			Version:    1,
			LastEditBy: project.OwnerID,
		}
		if err := db.WithContext(ctx).Create(&projectFile).Error; err != nil {
			return err
		}
	}

	return nil
}

func ExpoSpecForProjectExport(ctx context.Context, db *gorm.DB, project models.Project) (MobileAppSpec, bool, error) {
	var latestSpecBuild models.CompletedBuild
	err := db.WithContext(ctx).
		Where("project_id = ? AND user_id = ? AND target_platform = ? AND mobile_spec_json <> ?", project.ID, project.OwnerID, string(TargetPlatformMobileExpo), "").
		Order("created_at DESC").
		First(&latestSpecBuild).Error
	if err == nil {
		return SpecFromCompletedBuild(latestSpecBuild)
	}
	if err != gorm.ErrRecordNotFound {
		return MobileAppSpec{}, false, err
	}

	if strings.TrimSpace(project.TargetPlatform) != string(TargetPlatformMobileExpo) {
		var latestMobileBuild models.CompletedBuild
		err := db.WithContext(ctx).
			Where("project_id = ? AND user_id = ? AND target_platform = ?", project.ID, project.OwnerID, string(TargetPlatformMobileExpo)).
			Order("created_at DESC").
			First(&latestMobileBuild).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return MobileAppSpec{}, false, nil
			}
			return MobileAppSpec{}, false, err
		}
		return SpecFromCompletedBuild(latestMobileBuild)
	}

	if !HasExplicitMobileExportMetadata(project) {
		return MobileAppSpec{}, false, nil
	}
	return FieldServiceContractorQuoteSpec(), true, nil
}

func SpecFromCompletedBuild(build models.CompletedBuild) (MobileAppSpec, bool, error) {
	if strings.TrimSpace(build.MobileSpecJSON) == "" {
		return FieldServiceContractorQuoteSpec(), true, nil
	}

	var spec MobileAppSpec
	if err := json.Unmarshal([]byte(build.MobileSpecJSON), &spec); err != nil {
		return MobileAppSpec{}, false, fmt.Errorf("invalid persisted mobile_spec_json for build %q: %w", build.BuildID, err)
	}
	return spec, true, nil
}

func HasExplicitMobileExportMetadata(project models.Project) bool {
	if strings.TrimSpace(project.TargetPlatform) == string(TargetPlatformMobileExpo) {
		return true
	}
	return len(project.MobilePlatforms) > 0 ||
		strings.TrimSpace(project.MobileFramework) != "" ||
		strings.TrimSpace(project.MobileReleaseLevel) != "" ||
		len(project.MobileCapabilities) > 0 ||
		strings.TrimSpace(project.GeneratedMobileClientPath) != "" ||
		strings.TrimSpace(project.AndroidPackage) != "" ||
		strings.TrimSpace(project.IOSBundleIdentifier) != "" ||
		strings.TrimSpace(project.AppDisplayName) != "" ||
		len(project.MobileMetadata) > 0
}

func FormatValidationErrors(errs []ValidationError) string {
	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		if err.Field == "" {
			messages = append(messages, err.Message)
			continue
		}
		messages = append(messages, err.Field+": "+err.Message)
	}
	return strings.Join(messages, "; ")
}

func MimeTypeForSourceFile(file SourceFile) string {
	switch file.Language {
	case "json":
		return "application/json"
	case "markdown":
		return "text/markdown"
	case "dotenv":
		return "text/plain"
	case "typescript":
		return "application/typescript"
	case "image/png":
		return "image/png"
	default:
		return "text/plain"
	}
}

func normalizeGeneratedMobilePath(path string) (string, bool) {
	cleaned := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(strings.TrimSpace(path), "/")))
	if cleaned == "." || cleaned == "" {
		return "", false
	}
	if strings.HasPrefix(cleaned, "../") || cleaned == ".." || strings.Contains(cleaned, "/../") {
		return "", false
	}
	if !strings.HasPrefix(cleaned, "mobile/") {
		return "", false
	}
	return cleaned, true
}
