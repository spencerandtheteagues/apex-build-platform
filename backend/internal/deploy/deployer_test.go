package deploy

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubDeploymentProvider struct {
	name DeploymentProvider
}

func (s stubDeploymentProvider) Name() DeploymentProvider { return s.name }
func (stubDeploymentProvider) Deploy(_ context.Context, _ *DeploymentConfig, _ []ProjectFile) (*ProviderDeploymentResult, error) {
	return nil, nil
}
func (stubDeploymentProvider) GetStatus(_ context.Context, _ string) (*ProviderDeploymentResult, error) {
	return nil, nil
}
func (stubDeploymentProvider) Cancel(_ context.Context, _ string) error { return nil }
func (stubDeploymentProvider) GetLogs(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (stubDeploymentProvider) ValidateConfig(_ *DeploymentConfig) error { return nil }

type monitoringProviderStub struct {
	statuses []*ProviderDeploymentResult
	logs     [][]string
	index    int
}

func (m *monitoringProviderStub) Name() DeploymentProvider { return ProviderRailway }
func (m *monitoringProviderStub) Deploy(_ context.Context, _ *DeploymentConfig, _ []ProjectFile) (*ProviderDeploymentResult, error) {
	return nil, nil
}
func (m *monitoringProviderStub) GetStatus(_ context.Context, _ string) (*ProviderDeploymentResult, error) {
	if m.index >= len(m.statuses) {
		return m.statuses[len(m.statuses)-1], nil
	}
	status := m.statuses[m.index]
	m.index++
	return status, nil
}
func (m *monitoringProviderStub) Cancel(_ context.Context, _ string) error { return nil }
func (m *monitoringProviderStub) GetLogs(_ context.Context, _ string) ([]string, error) {
	if len(m.logs) == 0 {
		return nil, nil
	}
	if m.index-1 >= 0 && m.index-1 < len(m.logs) {
		return m.logs[m.index-1], nil
	}
	return m.logs[len(m.logs)-1], nil
}
func (m *monitoringProviderStub) ValidateConfig(_ *DeploymentConfig) error { return nil }

type stubDatabaseProvisioner struct {
	name DatabaseProvider
}

func (s stubDatabaseProvisioner) Name() DatabaseProvider               { return s.name }
func (stubDatabaseProvisioner) ValidateConfig(_ *DatabaseConfig) error { return nil }
func (stubDatabaseProvisioner) EnsureDatabase(_ context.Context, _ *DeploymentConfig) (*ProvisionedDatabaseResult, error) {
	return nil, nil
}

func TestApplyGeneratedDeploymentDefaultsBackfillsRuntimeFields(t *testing.T) {
	config := &DeploymentConfig{
		ProjectID: 1,
		Provider:  ProviderRender,
	}
	buildConfig := &BuildConfig{
		Framework:      "fastapi",
		BuildCommand:   "pip install -r requirements.txt",
		InstallCommand: "pip install -r requirements.txt",
		StartCommand:   "uvicorn main:app --host 0.0.0.0 --port $PORT",
		NodeVersion:    "20",
		OutputDir:      ".",
	}

	applyGeneratedDeploymentDefaults(config, buildConfig)

	if config.Framework != "fastapi" {
		t.Fatalf("expected framework backfilled, got %q", config.Framework)
	}
	if config.BuildCommand != buildConfig.BuildCommand {
		t.Fatalf("expected build command %q, got %q", buildConfig.BuildCommand, config.BuildCommand)
	}
	if config.InstallCmd != buildConfig.InstallCommand {
		t.Fatalf("expected install command %q, got %q", buildConfig.InstallCommand, config.InstallCmd)
	}
	if config.StartCommand != buildConfig.StartCommand {
		t.Fatalf("expected start command %q, got %q", buildConfig.StartCommand, config.StartCommand)
	}
	if config.NodeVersion != buildConfig.NodeVersion {
		t.Fatalf("expected node version %q, got %q", buildConfig.NodeVersion, config.NodeVersion)
	}
}

func TestApplyGeneratedDeploymentDefaultsPreservesExplicitValues(t *testing.T) {
	config := &DeploymentConfig{
		BuildCommand: "custom build",
		InstallCmd:   "custom install",
		StartCommand: "custom start",
		Framework:    "custom",
		NodeVersion:  "22",
		OutputDir:    "custom-dist",
	}
	buildConfig := &BuildConfig{
		Framework:      "react",
		BuildCommand:   "npm run build",
		InstallCommand: "npm install",
		StartCommand:   "npm start",
		NodeVersion:    "18",
		OutputDir:      "dist",
	}

	applyGeneratedDeploymentDefaults(config, buildConfig)

	if config.BuildCommand != "custom build" || config.InstallCmd != "custom install" || config.StartCommand != "custom start" {
		t.Fatalf("expected explicit commands to be preserved, got %+v", config)
	}
	if config.Framework != "custom" || config.NodeVersion != "22" || config.OutputDir != "custom-dist" {
		t.Fatalf("expected explicit runtime fields to be preserved, got %+v", config)
	}
}

func TestRestoreDeploymentConfigRestoresStartCommandAndNodeVersion(t *testing.T) {
	config := restoreDeploymentConfig(&Deployment{
		ProjectID:   7,
		Provider:    ProviderRender,
		Environment: "production",
		Branch:      "main",
		Config: map[string]interface{}{
			"build_command": "npm run build",
			"output_dir":    "dist",
			"install_cmd":   "npm ci",
			"start_command": "npm run serve",
			"framework":     "nextjs",
			"node_version":  "20",
		},
	})

	if config.ProjectID != 7 || config.Provider != ProviderRender || config.Environment != "production" || config.Branch != "main" {
		t.Fatalf("unexpected restored identity fields: %+v", config)
	}
	if config.StartCommand != "npm run serve" {
		t.Fatalf("expected restored start command, got %q", config.StartCommand)
	}
	if config.NodeVersion != "20" {
		t.Fatalf("expected restored node version, got %q", config.NodeVersion)
	}
}

func TestRestoreDeploymentConfigRestoresCustomAndRootDirectory(t *testing.T) {
	config := restoreDeploymentConfig(&Deployment{
		ProjectID:   8,
		Provider:    ProviderRailway,
		Environment: "production",
		Branch:      "main",
		Config: map[string]interface{}{
			"root_directory": "apps/web",
			"custom": map[string]interface{}{
				"railway_project": "proj_123",
				"railway_service": "web",
			},
		},
	})

	if config.RootDirectory != "apps/web" {
		t.Fatalf("expected restored root directory, got %q", config.RootDirectory)
	}
	if config.Custom["railway_project"] != "proj_123" || config.Custom["railway_service"] != "web" {
		t.Fatalf("expected restored custom provider state, got %+v", config.Custom)
	}
}

func TestRestoreDeploymentConfigRestoresDatabaseConfig(t *testing.T) {
	config := restoreDeploymentConfig(&Deployment{
		ProjectID:   9,
		Provider:    ProviderRender,
		Environment: "production",
		Branch:      "main",
		Config: map[string]interface{}{
			"database": map[string]interface{}{
				"provider":      "neon",
				"project_name":  "apex-db",
				"database_name": "app",
				"role_name":     "app_owner",
				"pg_version":    float64(16),
				"pooled":        true,
			},
		},
	})

	if config.Database == nil {
		t.Fatal("expected database config to be restored")
	}
	if config.Database.Provider != DatabaseProviderNeon || config.Database.ProjectName != "apex-db" {
		t.Fatalf("unexpected restored database config: %+v", config.Database)
	}
	if !config.Database.Pooled || config.Database.PGVersion != 16 {
		t.Fatalf("expected pooled Neon config with PG 16, got %+v", config.Database)
	}
}

func TestGetAvailableProvidersSortedWithNewProviderMetadata(t *testing.T) {
	service := &DeploymentService{
		providers: map[DeploymentProvider]Provider{
			ProviderCloudflarePages: stubDeploymentProvider{name: ProviderCloudflarePages},
			ProviderRailway:         stubDeploymentProvider{name: ProviderRailway},
			ProviderRender:          stubDeploymentProvider{name: ProviderRender},
		},
	}

	providers := service.GetAvailableProviders()
	if len(providers) != 3 {
		t.Fatalf("expected 3 providers, got %+v", providers)
	}

	ids := []string{
		providers[0]["id"].(string),
		providers[1]["id"].(string),
		providers[2]["id"].(string),
	}
	want := []string{"cloudflare_pages", "railway", "render"}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("provider order = %v, want %v", ids, want)
		}
	}

	if providers[0]["name"] != "Cloudflare Pages" {
		t.Fatalf("expected Cloudflare Pages display name, got %+v", providers[0])
	}
	if providers[1]["description"] == "" {
		t.Fatalf("expected Railway description, got %+v", providers[1])
	}
}

func TestValidateDatabaseConfigRequiresRegisteredProvisioner(t *testing.T) {
	service := &DeploymentService{
		databases: map[DatabaseProvider]DatabaseProvisioner{},
	}

	err := service.validateDatabaseConfig(&DeploymentConfig{
		Database: &DatabaseConfig{Provider: DatabaseProviderNeon},
	})
	if err == nil {
		t.Fatal("expected missing provisioner validation error")
	}
}

func TestValidateDatabaseConfigRejectsReservedEnvCollisions(t *testing.T) {
	service := &DeploymentService{
		databases: map[DatabaseProvider]DatabaseProvisioner{
			DatabaseProviderNeon: stubDatabaseProvisioner{name: DatabaseProviderNeon},
		},
	}

	err := service.validateDatabaseConfig(&DeploymentConfig{
		EnvVars:  map[string]string{"DATABASE_URL": "postgres://override"},
		Database: &DatabaseConfig{Provider: DatabaseProviderNeon},
	})
	if err == nil {
		t.Fatal("expected env collision validation error")
	}
}

func TestMonitorProviderDeploymentCompletesLiveDeployment(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Deployment{}, &DeploymentLog{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	service := &DeploymentService{
		db:                  db,
		monitorPollInterval: 5 * time.Millisecond,
	}
	deployment := &Deployment{
		ID:        "dep-1",
		ProjectID: 1,
		UserID:    1,
		Provider:  ProviderRailway,
		Status:    StatusDeploying,
		Config:    map[string]interface{}{},
		Metadata:  map[string]interface{}{},
	}
	if err := db.Create(deployment).Error; err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	provider := &monitoringProviderStub{
		statuses: []*ProviderDeploymentResult{
			{ProviderID: "provider-1", Status: StatusDeploying, URL: "https://preview.example.com"},
			{
				ProviderID: "provider-1",
				Status:     StatusLive,
				URL:        "https://app.example.com",
				Metadata:   map[string]any{"railway_project": "proj_123"},
			},
		},
		logs: [][]string{
			{"build started"},
			{"build started", "deployment ready"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := service.monitorProviderDeployment(ctx, deployment, provider, "provider-1", time.Now().Add(-25*time.Millisecond), time.Now().Add(-40*time.Millisecond), nil); err != nil {
		t.Fatalf("monitor provider deployment: %v", err)
	}

	var stored Deployment
	if err := db.First(&stored, "id = ?", deployment.ID).Error; err != nil {
		t.Fatalf("reload deployment: %v", err)
	}
	if stored.Status != StatusLive {
		t.Fatalf("expected live status, got %s", stored.Status)
	}
	if stored.URL != "https://app.example.com" {
		t.Fatalf("expected final URL saved, got %q", stored.URL)
	}
	if stored.CompletedAt == nil || stored.DeployTime == 0 || stored.TotalTime == 0 {
		t.Fatalf("expected completed deployment timings, got %+v", stored)
	}

	var logs []DeploymentLog
	if err := db.Where("deployment_id = ?", deployment.ID).Order("timestamp asc").Find(&logs).Error; err != nil {
		t.Fatalf("load logs: %v", err)
	}
	if len(logs) < 2 {
		t.Fatalf("expected provider monitoring logs to be persisted, got %+v", logs)
	}
}
