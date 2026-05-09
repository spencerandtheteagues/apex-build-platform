package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMobileFileMigrationCoversProductionMetadataColumns(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "000014_mobile_project_snapshot_metadata.up.sql")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mobile migration: %v", err)
	}
	sql := strings.ToLower(string(raw))

	projectColumns := []string{
		"add column if not exists target_platform",
		"add column if not exists mobile_platforms",
		"add column if not exists mobile_framework",
		"add column if not exists mobile_release_level",
		"add column if not exists mobile_capabilities",
		"add column if not exists mobile_dependency_policy",
		"add column if not exists mobile_preview_status",
		"add column if not exists mobile_build_status",
		"add column if not exists mobile_store_readiness_status",
		"add column if not exists generated_backend_url",
		"add column if not exists generated_mobile_client_path",
		"add column if not exists eas_project_id",
		"add column if not exists android_package",
		"add column if not exists ios_bundle_identifier",
		"add column if not exists app_display_name",
		"add column if not exists app_version",
		"add column if not exists build_number",
		"add column if not exists version_code",
		"add column if not exists icon_asset_ref",
		"add column if not exists splash_asset_ref",
		"add column if not exists permission_manifest",
		"add column if not exists store_metadata_draft_ref",
		"add column if not exists mobile_metadata",
	}
	assertMigrationSectionContains(t, path, sql, "alter table if exists projects", projectColumns)

	completedBuildColumns := []string{
		"add column if not exists target_platform",
		"add column if not exists mobile_platforms",
		"add column if not exists mobile_framework",
		"add column if not exists mobile_release_level",
		"add column if not exists mobile_capabilities",
		"add column if not exists android_package",
		"add column if not exists ios_bundle_identifier",
		"add column if not exists app_display_name",
		"add column if not exists app_version",
		"add column if not exists build_number",
		"add column if not exists version_code",
		"add column if not exists mobile_spec_json",
		"add column if not exists mobile_metadata",
	}
	assertMigrationSectionContains(t, path, sql, "alter table if exists completed_builds", completedBuildColumns)

	requiredFragments := []string{
		"create table if not exists mobile_submission_jobs",
		"provider_submission_id",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration %s missing %q", path, fragment)
		}
	}
}

func assertMigrationSectionContains(t *testing.T, path string, sql string, sectionStart string, fragments []string) {
	t.Helper()
	start := strings.Index(sql, sectionStart)
	if start < 0 {
		t.Fatalf("migration %s missing %q", path, sectionStart)
	}
	section := sql[start:]
	if end := strings.Index(section, ";"); end >= 0 {
		section = section[:end]
	}
	for _, fragment := range fragments {
		if !strings.Contains(section, fragment) {
			t.Fatalf("migration %s section %q missing %q", path, sectionStart, fragment)
		}
	}
}
