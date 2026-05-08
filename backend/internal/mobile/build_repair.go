package mobile

import "strings"

type MobileBuildRepairPlan struct {
	FailureType              MobileBuildFailureType    `json:"failure_type"`
	Title                    string                    `json:"title"`
	Summary                  string                    `json:"summary"`
	RetryRecommended         bool                      `json:"retry_recommended"`
	RequiresCredentialAction bool                      `json:"requires_credential_action,omitempty"`
	RequiresSourceChange     bool                      `json:"requires_source_change,omitempty"`
	Actions                  []MobileBuildRepairAction `json:"actions"`
}

type MobileBuildRepairAction struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	Blocking    bool   `json:"blocking,omitempty"`
}

func AttachMobileBuildRepairPlan(job MobileBuildJob) MobileBuildJob {
	job.RepairPlan = BuildMobileBuildRepairPlan(job)
	return job
}

func BuildMobileBuildRepairPlan(job MobileBuildJob) *MobileBuildRepairPlan {
	if !mobileBuildStatusNeedsRepairPlan(job.Status) {
		return nil
	}
	failureType := job.FailureType
	if failureType == "" {
		failureType = ClassifyMobileBuildFailure(job.FailureMessage)
	}
	if failureType == "" {
		failureType = MobileBuildFailureUnknown
	}

	plan := mobileBuildRepairPlanForFailureType(failureType)
	plan.FailureType = failureType
	plan.RetryRecommended = plan.RetryRecommended && IsRetryableMobileBuildStatus(job.Status)
	if strings.TrimSpace(plan.Title) == "" {
		plan.Title = "Review native build failure"
	}
	if strings.TrimSpace(plan.Summary) == "" {
		plan.Summary = "Apex classified this native build failure but does not have enough detail to safely auto-repair it."
	}
	return &plan
}

func mobileBuildStatusNeedsRepairPlan(status MobileBuildStatus) bool {
	switch status {
	case MobileBuildFailed, MobileBuildRepairPending, MobileBuildRepairedRetryPending:
		return true
	default:
		return false
	}
}

func MobileBuildFailureRequiresSourcePreRetryValidation(failureType MobileBuildFailureType) bool {
	switch failureType {
	case MobileBuildFailureDependencyInstallFailed,
		MobileBuildFailureExpoConfigInvalid,
		MobileBuildFailureUnsupportedNativeModule,
		MobileBuildFailureMetroBundleFailed,
		MobileBuildFailureTypeScriptFailed,
		MobileBuildFailureBackendAPIMismatch,
		MobileBuildFailurePermissionConfigMissing,
		MobileBuildFailureAppIdentifierInvalid:
		return true
	default:
		return false
	}
}

func ValidateMobileBuildPreRetrySource(previous MobileBuildJob, report MobileValidationReport) error {
	failureType := previous.FailureType
	if failureType == "" {
		failureType = ClassifyMobileBuildFailure(previous.FailureMessage)
	}
	if !MobileBuildFailureRequiresSourcePreRetryValidation(failureType) {
		return nil
	}
	if report.Status != MobileValidationPassed {
		return MobileBuildPreRetryValidationError{FailureType: failureType, Report: report}
	}
	return nil
}

type MobileBuildPreRetryValidationError struct {
	FailureType MobileBuildFailureType
	Report      MobileValidationReport
}

func (e MobileBuildPreRetryValidationError) Error() string {
	summary := strings.TrimSpace(e.Report.Summary)
	if summary == "" {
		summary = "mobile source validation did not pass"
	}
	return ErrMobileBuildPreRetryFailed.Error() + ": " + summary
}

func (e MobileBuildPreRetryValidationError) Unwrap() error {
	return ErrMobileBuildPreRetryFailed
}

func mobileBuildRepairPlanForFailureType(failureType MobileBuildFailureType) MobileBuildRepairPlan {
	switch failureType {
	case MobileBuildFailureDependencyInstallFailed:
		return MobileBuildRepairPlan{
			Title:                "Repair mobile dependency install",
			Summary:              "The native build failed while installing JavaScript dependencies. Keep Expo dependencies on the allowlist and rerun source validation before another EAS attempt.",
			RetryRecommended:     true,
			RequiresSourceChange: true,
			Actions: []MobileBuildRepairAction{
				{ID: "inspect-package-json", Label: "Inspect package.json", Description: "Remove unsupported native modules and align Expo package versions with the generated SDK.", Owner: "apex", Blocking: true},
				{ID: "rerun-source-validation", Label: "Rerun mobile source validation", Description: "Confirm dependency allowlist, TypeScript, app config, and browser-only API checks before retry.", Owner: "apex", Blocking: true},
			},
		}
	case MobileBuildFailureExpoConfigInvalid:
		return MobileBuildRepairPlan{
			Title:                "Fix Expo app config",
			Summary:              "Expo rejected the generated app configuration. Check identifiers, version fields, assets, plugins, and capability-derived permissions.",
			RetryRecommended:     true,
			RequiresSourceChange: true,
			Actions: []MobileBuildRepairAction{
				{ID: "validate-app-config", Label: "Validate app config", Description: "Verify app.config.ts/app.json, icon/splash assets, bundle identifiers, package names, and EAS profiles.", Owner: "apex", Blocking: true},
			},
		}
	case MobileBuildFailureUnsupportedNativeModule:
		return MobileBuildRepairPlan{
			Title:                "Remove unsupported native module",
			Summary:              "The generated app referenced a native dependency that is not supported by the current Expo allowlist or requires manual native project edits.",
			RetryRecommended:     true,
			RequiresSourceChange: true,
			Actions: []MobileBuildRepairAction{
				{ID: "replace-native-module", Label: "Replace native dependency", Description: "Swap unsupported packages for Expo-supported modules or document the manual config-plugin requirement.", Owner: "apex", Blocking: true},
			},
		}
	case MobileBuildFailureAndroidSigningFailed:
		return MobileBuildRepairPlan{
			Title:                    "Fix Android signing credentials",
			Summary:                  "Android signing failed. The app source may be valid, but the keystore/upload-key path needs credential repair before retry.",
			RetryRecommended:         true,
			RequiresCredentialAction: true,
			Actions: []MobileBuildRepairAction{
				{ID: "check-android-signing", Label: "Check Android signing", Description: "Reconnect or regenerate Android signing credentials and verify package name consistency.", Owner: "user", Blocking: true},
			},
		}
	case MobileBuildFailureIOSCredentialsFailed, MobileBuildFailureIOSProvisioningFailed:
		return MobileBuildRepairPlan{
			Title:                    "Fix iOS credentials/provisioning",
			Summary:                  "iOS build signing or provisioning failed. Apple team, App Store Connect, bundle identifier, and provisioning setup need verification before retry.",
			RetryRecommended:         true,
			RequiresCredentialAction: true,
			Actions: []MobileBuildRepairAction{
				{ID: "check-ios-credentials", Label: "Check Apple credentials", Description: "Verify App Store Connect API key, team ID, bundle identifier, certificates, and provisioning profile availability.", Owner: "user", Blocking: true},
			},
		}
	case MobileBuildFailureMetroBundleFailed:
		return MobileBuildRepairPlan{
			Title:                "Repair Metro bundling",
			Summary:              "Metro failed to bundle the React Native app. This usually points to import, routing, asset, or React Native compatibility issues.",
			RetryRecommended:     true,
			RequiresSourceChange: true,
			Actions: []MobileBuildRepairAction{
				{ID: "run-metro-checks", Label: "Run bundling checks", Description: "Inspect Metro output, missing imports, asset paths, Expo Router files, and React Native unsupported APIs.", Owner: "apex", Blocking: true},
			},
		}
	case MobileBuildFailureTypeScriptFailed:
		return MobileBuildRepairPlan{
			Title:                "Repair TypeScript errors",
			Summary:              "The generated mobile source failed TypeScript validation. Fix type mismatches before retrying native binaries.",
			RetryRecommended:     true,
			RequiresSourceChange: true,
			Actions: []MobileBuildRepairAction{
				{ID: "run-typecheck", Label: "Run TypeScript check", Description: "Run the generated mobile typecheck and patch contract/type mismatches with minimal edits.", Owner: "apex", Blocking: true},
			},
		}
	case MobileBuildFailureBackendAPIMismatch:
		return MobileBuildRepairPlan{
			Title:                "Repair mobile/backend API mismatch",
			Summary:              "The mobile client and backend contract disagree. Regenerate or patch API contract types, routes, and endpoint helpers from one source of truth.",
			RetryRecommended:     true,
			RequiresSourceChange: true,
			Actions: []MobileBuildRepairAction{
				{ID: "sync-api-contract", Label: "Sync API contract", Description: "Compare mobile/docs/api-contract.json, mobile/src/api/endpoints.ts, and generated backend routes.", Owner: "apex", Blocking: true},
			},
		}
	case MobileBuildFailurePermissionConfigMissing:
		return MobileBuildRepairPlan{
			Title:                "Add missing native permissions",
			Summary:              "A native capability needs explicit iOS usage descriptions or Android permissions before build.",
			RetryRecommended:     true,
			RequiresSourceChange: true,
			Actions: []MobileBuildRepairAction{
				{ID: "update-permission-manifest", Label: "Update permission manifest", Description: "Add capability-derived app config permissions and human-readable iOS usage descriptions.", Owner: "apex", Blocking: true},
			},
		}
	case MobileBuildFailureAppIdentifierInvalid:
		return MobileBuildRepairPlan{
			Title:                "Fix app identifiers",
			Summary:              "The Android package name or iOS bundle identifier is invalid or mismatched with credentials.",
			RetryRecommended:     true,
			RequiresSourceChange: true,
			Actions: []MobileBuildRepairAction{
				{ID: "fix-app-identifiers", Label: "Fix identifiers", Description: "Validate Android package and iOS bundle identifier syntax and keep them aligned with signing credentials.", Owner: "apex", Blocking: true},
			},
		}
	case MobileBuildFailureStoreSubmissionFailed:
		return MobileBuildRepairPlan{
			Title:                    "Repair store submission setup",
			Summary:                  "Store upload failed after binary generation. Build artifacts and store metadata should be checked separately from source validity.",
			RetryRecommended:         false,
			RequiresCredentialAction: true,
			Actions: []MobileBuildRepairAction{
				{ID: "check-store-setup", Label: "Check store setup", Description: "Verify Google Play/App Store Connect app records, tracks, agreements, metadata, and upload permissions.", Owner: "user", Blocking: true},
			},
		}
	default:
		return MobileBuildRepairPlan{
			Title:            "Review native build failure",
			Summary:          "Apex could not confidently classify this failure. Inspect redacted logs before deciding whether source, credentials, or provider state caused it.",
			RetryRecommended: false,
			Actions: []MobileBuildRepairAction{
				{ID: "inspect-redacted-logs", Label: "Inspect redacted logs", Description: "Review build logs for dependency, config, credential, bundling, or provider outage signals.", Owner: "apex", Blocking: true},
			},
		}
	}
}
