package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	secretstore "apex-build/internal/secrets"
	"apex-build/pkg/models"

	"gorm.io/gorm"
)

type MobileCredentialType string

const (
	MobileCredentialEASToken             MobileCredentialType = "eas_token"
	MobileCredentialAppleAppStoreConnect MobileCredentialType = "apple_app_store_connect"
	MobileCredentialGooglePlayService    MobileCredentialType = "google_play_service_account"
	MobileCredentialAndroidSigning       MobileCredentialType = "android_signing"
	mobileCredentialSecretNamePrefix                          = "mobile:"
)

var (
	ErrMobileCredentialInvalidType    = errors.New("invalid mobile credential type")
	ErrMobileCredentialInvalidPayload = errors.New("invalid mobile credential payload")
	ErrMobileCredentialNotFound       = errors.New("mobile credential not found")
)

type MobileCredentialSecretPayload struct {
	Type   MobileCredentialType `json:"type"`
	Values map[string]string    `json:"values"`
}

type MobileCredentialInput struct {
	Type   MobileCredentialType `json:"type"`
	Values map[string]string    `json:"values"`
}

type MobileCredentialMetadata struct {
	Type       MobileCredentialType `json:"type"`
	SecretID   uint                 `json:"secret_id"`
	ProjectID  uint                 `json:"project_id"`
	Status     string               `json:"status"`
	Label      string               `json:"label"`
	LastTested *time.Time           `json:"last_tested,omitempty"`
	CreatedAt  time.Time            `json:"created_at"`
	UpdatedAt  time.Time            `json:"updated_at"`
}

type MobileCredentialStatus struct {
	Status   string                     `json:"status"`
	Complete bool                       `json:"complete"`
	Required []MobileCredentialType     `json:"required"`
	Present  []MobileCredentialType     `json:"present"`
	Missing  []MobileCredentialType     `json:"missing"`
	Metadata []MobileCredentialMetadata `json:"metadata"`
	Blockers []string                   `json:"blockers,omitempty"`
}

type MobileCredentialVault struct {
	db      *gorm.DB
	manager *secretstore.SecretsManager
}

func NewMobileCredentialVault(db *gorm.DB, manager *secretstore.SecretsManager) *MobileCredentialVault {
	return &MobileCredentialVault{db: db, manager: manager}
}

func (v *MobileCredentialVault) Store(ctx context.Context, userID uint, project models.Project, input MobileCredentialInput) (MobileCredentialStatus, error) {
	if v == nil || v.db == nil || v.manager == nil {
		return MobileCredentialStatus{}, fmt.Errorf("%w: credential vault unavailable", ErrMobileCredentialInvalidPayload)
	}
	input.Type = normalizeMobileCredentialType(input.Type)
	if !isSupportedMobileCredentialType(input.Type) {
		return MobileCredentialStatus{}, ErrMobileCredentialInvalidType
	}
	values := normalizeCredentialValues(input.Values)
	if err := validateMobileCredentialPayload(input.Type, values); err != nil {
		return MobileCredentialStatus{}, err
	}

	payload := MobileCredentialSecretPayload{Type: input.Type, Values: values}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return MobileCredentialStatus{}, err
	}
	encrypted, salt, fingerprint, err := v.manager.Encrypt(userID, string(encoded))
	if err != nil {
		return MobileCredentialStatus{}, err
	}

	name := mobileCredentialSecretName(input.Type)
	var existing secretstore.Secret
	err = v.db.WithContext(ctx).
		Where("user_id = ? AND project_id = ? AND name = ?", userID, project.ID, name).
		First(&existing).Error
	if err == nil {
		existing.EncryptedValue = encrypted
		existing.Salt = salt
		existing.KeyFingerprint = fingerprint
		existing.Description = mobileCredentialLabel(input.Type)
		existing.Type = secretstore.SecretTypeAPIKey
		existing.UpdatedAt = time.Now()
		if saveErr := v.db.WithContext(ctx).Save(&existing).Error; saveErr != nil {
			return MobileCredentialStatus{}, saveErr
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		projectID := project.ID
		secret := secretstore.Secret{
			UserID:         userID,
			ProjectID:      &projectID,
			Name:           name,
			Description:    mobileCredentialLabel(input.Type),
			Type:           secretstore.SecretTypeAPIKey,
			EncryptedValue: encrypted,
			Salt:           salt,
			KeyFingerprint: fingerprint,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if createErr := v.db.WithContext(ctx).Create(&secret).Error; createErr != nil {
			return MobileCredentialStatus{}, createErr
		}
	} else {
		return MobileCredentialStatus{}, err
	}

	status, err := v.Status(ctx, userID, project)
	if err != nil {
		return MobileCredentialStatus{}, err
	}
	if err := v.UpdateProjectCredentialMetadata(ctx, project, status); err != nil {
		return status, err
	}
	return status, nil
}

func (v *MobileCredentialVault) Status(ctx context.Context, userID uint, project models.Project) (MobileCredentialStatus, error) {
	if v == nil || v.db == nil {
		return MobileCredentialStatus{}, fmt.Errorf("%w: credential vault unavailable", ErrMobileCredentialInvalidPayload)
	}
	var stored []secretstore.Secret
	if err := v.db.WithContext(ctx).
		Where("user_id = ? AND project_id = ? AND name LIKE ?", userID, project.ID, mobileCredentialSecretNamePrefix+"%").
		Order("updated_at DESC").
		Find(&stored).Error; err != nil {
		return MobileCredentialStatus{}, err
	}

	presentSet := map[MobileCredentialType]bool{}
	metadata := make([]MobileCredentialMetadata, 0, len(stored))
	for _, secret := range stored {
		credType, ok := credentialTypeFromSecretName(secret.Name)
		if !ok {
			continue
		}
		presentSet[credType] = true
		metadata = append(metadata, MobileCredentialMetadata{
			Type:      credType,
			SecretID:  secret.ID,
			ProjectID: project.ID,
			Status:    "stored",
			Label:     mobileCredentialLabel(credType),
			CreatedAt: secret.CreatedAt,
			UpdatedAt: secret.UpdatedAt,
		})
	}

	required := RequiredMobileCredentialTypes(project)
	var present []MobileCredentialType
	var missing []MobileCredentialType
	var blockers []string
	for _, credType := range required {
		if presentSet[credType] {
			present = append(present, credType)
			continue
		}
		missing = append(missing, credType)
		blockers = append(blockers, "Add "+mobileCredentialLabel(credType)+".")
	}
	complete := len(required) > 0 && len(missing) == 0
	status := "missing"
	if complete {
		status = "validated"
	} else if len(present) > 0 {
		status = "partial"
	}
	return MobileCredentialStatus{
		Status:   status,
		Complete: complete,
		Required: required,
		Present:  present,
		Missing:  missing,
		Metadata: metadata,
		Blockers: blockers,
	}, nil
}

func (v *MobileCredentialVault) Delete(ctx context.Context, userID uint, project models.Project, credType MobileCredentialType) (MobileCredentialStatus, error) {
	if v == nil || v.db == nil {
		return MobileCredentialStatus{}, fmt.Errorf("%w: credential vault unavailable", ErrMobileCredentialInvalidPayload)
	}
	credType = normalizeMobileCredentialType(credType)
	if !isSupportedMobileCredentialType(credType) {
		return MobileCredentialStatus{}, ErrMobileCredentialInvalidType
	}
	result := v.db.WithContext(ctx).
		Where("user_id = ? AND project_id = ? AND name = ?", userID, project.ID, mobileCredentialSecretName(credType)).
		Delete(&secretstore.Secret{})
	if result.Error != nil {
		return MobileCredentialStatus{}, result.Error
	}
	if result.RowsAffected == 0 {
		return MobileCredentialStatus{}, ErrMobileCredentialNotFound
	}
	status, err := v.Status(ctx, userID, project)
	if err != nil {
		return MobileCredentialStatus{}, err
	}
	if err := v.UpdateProjectCredentialMetadata(ctx, project, status); err != nil {
		return status, err
	}
	return status, nil
}

func (v *MobileCredentialVault) UpdateProjectCredentialMetadata(ctx context.Context, project models.Project, status MobileCredentialStatus) error {
	if v == nil || v.db == nil || project.ID == 0 {
		return nil
	}

	var current models.Project
	if err := v.db.WithContext(ctx).First(&current, project.ID).Error; err != nil {
		return err
	}

	metadata := copyMobileMetadata(current.MobileMetadata)
	metadata["credential_status"] = status.Status
	metadata["credentials_validated"] = status.Complete
	metadata["mobile_credential_required"] = credentialTypeStrings(status.Required)
	metadata["mobile_credential_present"] = credentialTypeStrings(status.Present)
	metadata["mobile_credential_missing"] = credentialTypeStrings(status.Missing)

	current.MobileMetadata = metadata
	return v.db.WithContext(ctx).Select("MobileMetadata").Save(&current).Error
}

func RequiredMobileCredentialTypes(project models.Project) []MobileCredentialType {
	required := []MobileCredentialType{MobileCredentialEASToken}
	platforms := normalizedProjectPlatformSet(project)
	if len(platforms) == 0 {
		platforms[string(MobilePlatformAndroid)] = true
		platforms[string(MobilePlatformIOS)] = true
	}
	if platforms[string(MobilePlatformIOS)] {
		required = append(required, MobileCredentialAppleAppStoreConnect)
	}
	if platforms[string(MobilePlatformAndroid)] {
		required = append(required, MobileCredentialGooglePlayService)
	}
	return required
}

func validateMobileCredentialPayload(credType MobileCredentialType, values map[string]string) error {
	required := map[MobileCredentialType][]string{
		MobileCredentialEASToken:             {"token"},
		MobileCredentialAppleAppStoreConnect: {"key_id", "issuer_id", "private_key", "team_id"},
		MobileCredentialGooglePlayService:    {"service_account_json"},
		MobileCredentialAndroidSigning:       {"keystore_base64", "keystore_password", "key_alias", "key_password"},
	}
	for _, key := range required[credType] {
		if strings.TrimSpace(values[key]) == "" {
			return fmt.Errorf("%w: %s is required for %s", ErrMobileCredentialInvalidPayload, key, credType)
		}
	}
	if credType == MobileCredentialGooglePlayService && !looksLikeGoogleServiceAccount(values["service_account_json"]) {
		return fmt.Errorf("%w: service_account_json must look like a Google service account JSON", ErrMobileCredentialInvalidPayload)
	}
	return nil
}

func looksLikeGoogleServiceAccount(raw string) bool {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return false
	}
	return strings.TrimSpace(asString(parsed["client_email"])) != "" &&
		strings.TrimSpace(asString(parsed["private_key"])) != ""
}

func normalizeCredentialValues(values map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range values {
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}

func normalizeMobileCredentialType(credType MobileCredentialType) MobileCredentialType {
	return MobileCredentialType(strings.TrimSpace(string(credType)))
}

func isSupportedMobileCredentialType(credType MobileCredentialType) bool {
	switch credType {
	case MobileCredentialEASToken, MobileCredentialAppleAppStoreConnect, MobileCredentialGooglePlayService, MobileCredentialAndroidSigning:
		return true
	default:
		return false
	}
}

func mobileCredentialSecretName(credType MobileCredentialType) string {
	return mobileCredentialSecretNamePrefix + string(credType)
}

func credentialTypeFromSecretName(name string) (MobileCredentialType, bool) {
	if !strings.HasPrefix(name, mobileCredentialSecretNamePrefix) {
		return "", false
	}
	credType := MobileCredentialType(strings.TrimPrefix(name, mobileCredentialSecretNamePrefix))
	return credType, isSupportedMobileCredentialType(credType)
}

func mobileCredentialLabel(credType MobileCredentialType) string {
	switch credType {
	case MobileCredentialEASToken:
		return "EAS token"
	case MobileCredentialAppleAppStoreConnect:
		return "Apple App Store Connect API key"
	case MobileCredentialGooglePlayService:
		return "Google Play service account"
	case MobileCredentialAndroidSigning:
		return "Android signing material"
	default:
		return "Mobile credential"
	}
}

func credentialTypeStrings(types []MobileCredentialType) []string {
	out := make([]string, 0, len(types))
	for _, credType := range types {
		out = append(out, string(credType))
	}
	return out
}

func copyMobileMetadata(metadata map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func asString(value interface{}) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}
