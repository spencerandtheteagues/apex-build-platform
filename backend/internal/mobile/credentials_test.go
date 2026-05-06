package mobile

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	secretstore "apex-build/internal/secrets"
	"apex-build/pkg/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMobileCredentialVaultStoresEncryptedCredentialAndReturnsMetadataOnly(t *testing.T) {
	gormDB, manager := newMobileCredentialTestDB(t)
	project := createMobileCredentialTestProject(t, gormDB, []string{string(MobilePlatformAndroid), string(MobilePlatformIOS)})
	vault := NewMobileCredentialVault(gormDB, manager)

	status, err := vault.Store(context.Background(), project.OwnerID, project, MobileCredentialInput{
		Type: MobileCredentialEASToken,
		Values: map[string]string{
			"token": "eas-secret-token",
		},
	})
	if err != nil {
		t.Fatalf("store EAS credential: %v", err)
	}
	if status.Status != "partial" || status.Complete {
		t.Fatalf("expected partial status after only EAS credential, got %+v", status)
	}
	if len(status.Metadata) != 1 || status.Metadata[0].Type != MobileCredentialEASToken {
		t.Fatalf("expected EAS metadata only, got %+v", status.Metadata)
	}
	encoded, _ := json.Marshal(status)
	if strings.Contains(string(encoded), "eas-secret-token") {
		t.Fatalf("metadata response leaked secret: %s", encoded)
	}

	var stored secretstore.Secret
	if err := gormDB.Where("user_id = ? AND project_id = ? AND name = ?", project.OwnerID, project.ID, mobileCredentialSecretName(MobileCredentialEASToken)).First(&stored).Error; err != nil {
		t.Fatalf("fetch stored secret: %v", err)
	}
	if strings.Contains(stored.EncryptedValue, "eas-secret-token") {
		t.Fatalf("encrypted storage contains raw secret: %+v", stored)
	}
	var payload MobileCredentialSecretPayload
	if err := manager.DecryptJSON(project.OwnerID, stored.EncryptedValue, stored.Salt, &payload); err != nil {
		t.Fatalf("decrypt stored payload: %v", err)
	}
	if payload.Values["token"] != "eas-secret-token" {
		t.Fatalf("unexpected decrypted payload %+v", payload)
	}
}

func TestMobileCredentialVaultMarksValidatedWhenRequiredPlatformCredentialsExist(t *testing.T) {
	gormDB, manager := newMobileCredentialTestDB(t)
	project := createMobileCredentialTestProject(t, gormDB, []string{string(MobilePlatformAndroid), string(MobilePlatformIOS)})
	vault := NewMobileCredentialVault(gormDB, manager)

	for _, input := range []MobileCredentialInput{
		{Type: MobileCredentialEASToken, Values: map[string]string{"token": "eas-secret-token"}},
		{Type: MobileCredentialAppleAppStoreConnect, Values: map[string]string{
			"key_id":      "APPLEKEY",
			"issuer_id":   "issuer",
			"private_key": "-----BEGIN PRIVATE KEY-----\napple\n-----END PRIVATE KEY-----",
			"team_id":     "TEAM123",
		}},
		{Type: MobileCredentialGooglePlayService, Values: map[string]string{"service_account_json": `{"client_email":"play@example.iam.gserviceaccount.com","private_key":"-----BEGIN PRIVATE KEY-----\nplay\n-----END PRIVATE KEY-----"}`}},
	} {
		if _, err := vault.Store(context.Background(), project.OwnerID, project, input); err != nil {
			t.Fatalf("store %s credential: %v", input.Type, err)
		}
	}

	status, err := vault.Status(context.Background(), project.OwnerID, project)
	if err != nil {
		t.Fatalf("credential status: %v", err)
	}
	if !status.Complete || status.Status != "validated" {
		t.Fatalf("expected validated status, got %+v", status)
	}
	if len(status.Missing) != 0 {
		t.Fatalf("expected no missing credentials, got %+v", status.Missing)
	}

	var updated models.Project
	if err := gormDB.First(&updated, project.ID).Error; err != nil {
		t.Fatalf("fetch updated project: %v", err)
	}
	if !mobileMetadataBool(updated.MobileMetadata, "credentials_validated") {
		t.Fatalf("expected project metadata to mark credentials validated, got %+v", updated.MobileMetadata)
	}
}

func TestMobileCredentialVaultBuildStatusRequiresEASTokenOnly(t *testing.T) {
	gormDB, manager := newMobileCredentialTestDB(t)
	project := createMobileCredentialTestProject(t, gormDB, []string{string(MobilePlatformAndroid)})
	vault := NewMobileCredentialVault(gormDB, manager)

	status, err := vault.BuildStatus(context.Background(), project.OwnerID, project, MobilePlatformAndroid, ReleaseInternalAndroidAPK)
	if err != nil {
		t.Fatalf("build credential status: %v", err)
	}
	if status.Complete || len(status.Missing) != 1 || status.Missing[0] != MobileCredentialEASToken {
		t.Fatalf("expected only EAS token missing for build credentials, got %+v", status)
	}

	if _, err := vault.Store(context.Background(), project.OwnerID, project, MobileCredentialInput{
		Type:   MobileCredentialEASToken,
		Values: map[string]string{"token": "eas-secret-token"},
	}); err != nil {
		t.Fatalf("store EAS credential: %v", err)
	}
	status, err = vault.BuildStatus(context.Background(), project.OwnerID, project, MobilePlatformAndroid, ReleaseInternalAndroidAPK)
	if err != nil {
		t.Fatalf("build credential status after storing EAS: %v", err)
	}
	if !status.Complete || len(status.Missing) != 0 {
		t.Fatalf("expected EAS token to complete build credential status, got %+v", status)
	}
	readinessStatus, err := vault.Status(context.Background(), project.OwnerID, project)
	if err != nil {
		t.Fatalf("store readiness credential status: %v", err)
	}
	if readinessStatus.Complete || len(readinessStatus.Missing) != 1 || readinessStatus.Missing[0] != MobileCredentialGooglePlayService {
		t.Fatalf("expected store readiness to still require Google Play credential, got %+v", readinessStatus)
	}
}

func TestMobileCredentialVaultResolvesEncryptedCredentialValues(t *testing.T) {
	gormDB, manager := newMobileCredentialTestDB(t)
	project := createMobileCredentialTestProject(t, gormDB, []string{string(MobilePlatformAndroid)})
	vault := NewMobileCredentialVault(gormDB, manager)

	if _, err := vault.Store(context.Background(), project.OwnerID, project, MobileCredentialInput{
		Type:   MobileCredentialEASToken,
		Values: map[string]string{"token": "eas-secret-token"},
	}); err != nil {
		t.Fatalf("store EAS credential: %v", err)
	}

	values, err := vault.ResolveCredentialValues(context.Background(), project.OwnerID, project.ID, MobileCredentialEASToken)
	if err != nil {
		t.Fatalf("resolve EAS credential values: %v", err)
	}
	if values["token"] != "eas-secret-token" {
		t.Fatalf("unexpected resolved token values %+v", values)
	}

	var stored secretstore.Secret
	if err := gormDB.Where("user_id = ? AND project_id = ? AND name = ?", project.OwnerID, project.ID, mobileCredentialSecretName(MobileCredentialEASToken)).First(&stored).Error; err != nil {
		t.Fatalf("fetch stored EAS secret: %v", err)
	}
	stored.EncryptedValue = strings.ReplaceAll(stored.EncryptedValue, stored.EncryptedValue[len(stored.EncryptedValue)-4:], "xxxx")
	if err := gormDB.Save(&stored).Error; err != nil {
		t.Fatalf("corrupt stored EAS secret: %v", err)
	}
	if _, err := vault.ResolveCredentialValues(context.Background(), project.OwnerID, project.ID, MobileCredentialEASToken); err == nil {
		t.Fatalf("expected corrupted encrypted credential to fail resolution")
	}
}

func TestMobileCredentialVaultRejectsInvalidPayloadShape(t *testing.T) {
	gormDB, manager := newMobileCredentialTestDB(t)
	project := createMobileCredentialTestProject(t, gormDB, []string{string(MobilePlatformAndroid)})
	vault := NewMobileCredentialVault(gormDB, manager)

	_, err := vault.Store(context.Background(), project.OwnerID, project, MobileCredentialInput{
		Type:   MobileCredentialGooglePlayService,
		Values: map[string]string{"service_account_json": `{"client_email":"missing-private-key@example.com"}`},
	})
	if err == nil || !strings.Contains(err.Error(), "service_account_json") {
		t.Fatalf("expected invalid google service account payload error, got %v", err)
	}
}

func TestMobileCredentialVaultDeleteUpdatesProjectCredentialStatus(t *testing.T) {
	gormDB, manager := newMobileCredentialTestDB(t)
	project := createMobileCredentialTestProject(t, gormDB, []string{string(MobilePlatformAndroid)})
	vault := NewMobileCredentialVault(gormDB, manager)

	for _, input := range []MobileCredentialInput{
		{Type: MobileCredentialEASToken, Values: map[string]string{"token": "eas-secret-token"}},
		{Type: MobileCredentialGooglePlayService, Values: map[string]string{"service_account_json": `{"client_email":"play@example.iam.gserviceaccount.com","private_key":"secret"}`}},
	} {
		if _, err := vault.Store(context.Background(), project.OwnerID, project, input); err != nil {
			t.Fatalf("store %s credential: %v", input.Type, err)
		}
	}

	status, err := vault.Delete(context.Background(), project.OwnerID, project, MobileCredentialGooglePlayService)
	if err != nil {
		t.Fatalf("delete google credential: %v", err)
	}
	if status.Complete || status.Status != "partial" {
		t.Fatalf("expected partial status after delete, got %+v", status)
	}
	if len(status.Missing) != 1 || status.Missing[0] != MobileCredentialGooglePlayService {
		t.Fatalf("expected Google credential missing, got %+v", status.Missing)
	}
}

func newMobileCredentialTestDB(t *testing.T) (*gorm.DB, *secretstore.SecretsManager) {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := gormDB.AutoMigrate(&models.User{}, &models.Project{}, &secretstore.Secret{}, &secretstore.SecretAuditLog{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	manager, err := secretstore.NewSecretsManager("mobile-credential-test-master-key")
	if err != nil {
		t.Fatalf("new secrets manager: %v", err)
	}
	return gormDB, manager
}

func createMobileCredentialTestProject(t *testing.T, gormDB *gorm.DB, platforms []string) models.Project {
	t.Helper()
	user := models.User{
		Username:     strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_"),
		Email:        strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_") + "@example.com",
		PasswordHash: "hashed",
	}
	if err := gormDB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	project := models.Project{
		Name:            "Mobile Credentials",
		Language:        "typescript",
		OwnerID:         user.ID,
		TargetPlatform:  string(TargetPlatformMobileExpo),
		MobilePlatforms: platforms,
		MobileFramework: string(MobileFrameworkExpoReactNative),
		MobileMetadata:  map[string]interface{}{},
	}
	if err := gormDB.Create(&project).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	return project
}
