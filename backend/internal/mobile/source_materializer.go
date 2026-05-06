package mobile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"apex-build/pkg/models"

	"gorm.io/gorm"
)

type MaterializedMobileSource struct {
	RootDir   string
	MobileDir string
	Cleanup   func() error
}

func MaterializeProjectMobileSource(ctx context.Context, db *gorm.DB, project models.Project) (MaterializedMobileSource, error) {
	if db == nil {
		return MaterializedMobileSource{}, fmt.Errorf("%w: database is required to materialize source", ErrMobileBuildInvalidRequest)
	}
	if err := PrepareExpoProjectFiles(ctx, db, project); err != nil {
		return MaterializedMobileSource{}, err
	}

	var files []models.File
	if err := db.WithContext(ctx).
		Where("project_id = ? AND type <> ? AND (path LIKE ? OR path LIKE ?)", project.ID, "directory", "mobile/%", "/mobile/%").
		Find(&files).Error; err != nil {
		return MaterializedMobileSource{}, err
	}
	if len(files) == 0 {
		return MaterializedMobileSource{}, fmt.Errorf("%w: no mobile source files found", ErrMobileBuildInvalidRequest)
	}

	root, err := os.MkdirTemp("", fmt.Sprintf("apex-mobile-build-%d-*", project.ID))
	if err != nil {
		return MaterializedMobileSource{}, err
	}
	root = filepath.Clean(root)
	cleanup := func() error { return os.RemoveAll(root) }

	for _, file := range files {
		path, ok := normalizeGeneratedMobilePath(file.Path)
		if !ok {
			_ = cleanup()
			return MaterializedMobileSource{}, fmt.Errorf("%w: unsafe mobile source path %q", ErrMobileBuildInvalidRequest, file.Path)
		}
		target := filepath.Clean(filepath.Join(root, filepath.FromSlash(path)))
		if !strings.HasPrefix(target, root+string(os.PathSeparator)) {
			_ = cleanup()
			return MaterializedMobileSource{}, fmt.Errorf("%w: unsafe mobile source path %q", ErrMobileBuildInvalidRequest, file.Path)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			_ = cleanup()
			return MaterializedMobileSource{}, err
		}
		if err := os.WriteFile(target, []byte(file.Content), 0o644); err != nil {
			_ = cleanup()
			return MaterializedMobileSource{}, err
		}
	}

	packagePath := filepath.Join(root, "mobile", "package.json")
	if _, err := os.Stat(packagePath); err != nil {
		_ = cleanup()
		return MaterializedMobileSource{}, fmt.Errorf("%w: materialized Expo source is missing package.json", ErrMobileBuildInvalidRequest)
	}

	return MaterializedMobileSource{
		RootDir:   root,
		MobileDir: filepath.Join(root, "mobile"),
		Cleanup:   cleanup,
	}, nil
}

func ProjectMobileSourceFingerprint(files []models.File) string {
	hash := sha256.New()
	for _, file := range files {
		path, ok := normalizeGeneratedMobilePath(file.Path)
		if !ok {
			continue
		}
		hash.Write([]byte(path))
		hash.Write([]byte{0})
		hash.Write([]byte(file.Content))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}
