package agents

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"apex-build/internal/ai"

	"github.com/google/uuid"
)

type BuildComplexityClass string

const (
	ComplexitySimple  BuildComplexityClass = "simple"
	ComplexityMedium  BuildComplexityClass = "medium"
	ComplexityComplex BuildComplexityClass = "complex"
)

type CapabilityRequirement string

const (
	CapabilityAuth        CapabilityRequirement = "auth"
	CapabilityDatabase    CapabilityRequirement = "database"
	CapabilityStorage     CapabilityRequirement = "storage"
	CapabilityAPI         CapabilityRequirement = "api"
	CapabilityJobs        CapabilityRequirement = "jobs"
	CapabilityBilling     CapabilityRequirement = "billing"
	CapabilitySearch      CapabilityRequirement = "search"
	CapabilityRealtime    CapabilityRequirement = "realtime"
	CapabilityFileUpload  CapabilityRequirement = "file_upload"
	CapabilityExternalAPI CapabilityRequirement = "external_api"
)

type CostSensitivity string

const (
	CostSensitivityHigh   CostSensitivity = "high"
	CostSensitivityMedium CostSensitivity = "medium"
	CostSensitivityLow    CostSensitivity = "low"
)

type WorkOrderCategory string

const (
	WorkOrderFrontend     WorkOrderCategory = "frontend_feature"
	WorkOrderBackend      WorkOrderCategory = "backend_resource"
	WorkOrderData         WorkOrderCategory = "data_schema"
	WorkOrderIntegration  WorkOrderCategory = "integration_glue"
	WorkOrderAuth         WorkOrderCategory = "auth_flow"
	WorkOrderConfig       WorkOrderCategory = "config_deploy"
	WorkOrderVerification WorkOrderCategory = "verification"
	WorkOrderRepair       WorkOrderCategory = "repair"
)

type TaskRiskLevel string

const (
	RiskLow      TaskRiskLevel = "low"
	RiskMedium   TaskRiskLevel = "medium"
	RiskHigh     TaskRiskLevel = "high"
	RiskCritical TaskRiskLevel = "critical"
)

type ProviderRoutingMode string

const (
	RoutingModeSingleProvider     ProviderRoutingMode = "mode_a_single_provider"
	RoutingModeSingleWithVerifier ProviderRoutingMode = "mode_b_single_with_verifier"
	RoutingModeDualCandidate      ProviderRoutingMode = "mode_c_dual_candidate"
	RoutingModeDiagnosisRepair    ProviderRoutingMode = "mode_d_diagnosis_repair"
)

type TaskShape string

const (
	TaskShapeIntentNormalization TaskShape = "intent_normalization"
	TaskShapeContract            TaskShape = "contract"
	TaskShapeFrontendPatch       TaskShape = "frontend_patch"
	TaskShapeBackendPatch        TaskShape = "backend_patch"
	TaskShapeSchema              TaskShape = "schema"
	TaskShapeIntegration         TaskShape = "integration"
	TaskShapeVerification        TaskShape = "verification"
	TaskShapeRepair              TaskShape = "repair"
	TaskShapeDiagnosis           TaskShape = "diagnosis"
	TaskShapeAdversarialCritique TaskShape = "adversarial_critique"
	TaskShapePromotion           TaskShape = "promotion"
)

type VerificationStatus string

const (
	VerificationPassed  VerificationStatus = "passed"
	VerificationFailed  VerificationStatus = "failed"
	VerificationBlocked VerificationStatus = "blocked"
)

type ReadinessState string

const (
	ReadinessPrototypeReady   ReadinessState = "prototype_ready"
	ReadinessPreviewReady     ReadinessState = "preview_ready"
	ReadinessIntegrationReady ReadinessState = "integration_ready"
	ReadinessTestReady        ReadinessState = "test_ready"
	ReadinessProductionCand   ReadinessState = "production_candidate"
	ReadinessBlocked          ReadinessState = "blocked"
)

type TruthTag string

const (
	TruthMocked              TruthTag = "mocked"
	TruthPrototypeUIOnly     TruthTag = "prototype_ui_only"
	TruthScaffolded          TruthTag = "scaffolded"
	TruthPartiallyWired      TruthTag = "partially_wired"
	TruthLiveLogicConnected  TruthTag = "live_logic_connected"
	TruthVerified            TruthTag = "verified"
	TruthTested              TruthTag = "tested"
	TruthBlocked             TruthTag = "blocked"
	TruthNeedsSecrets        TruthTag = "needs_secrets"
	TruthNeedsApproval       TruthTag = "needs_approval"
	TruthNeedsExternalAPI    TruthTag = "needs_external_api"
	TruthNeedsBackendRuntime TruthTag = "needs_backend_runtime"
	TruthExperimental        TruthTag = "experimental"
	TruthUpgradeRequired     TruthTag = "upgrade_required"
	TruthProductionCandidate TruthTag = "production_candidate"
	TruthProductionReady     TruthTag = "production_ready"
)

type ContractSurface string

const (
	SurfaceFrontend    ContractSurface = "frontend"
	SurfaceBackend     ContractSurface = "backend"
	SurfaceData        ContractSurface = "data"
	SurfaceIntegration ContractSurface = "integration"
	SurfaceDeployment  ContractSurface = "deployment"
	SurfaceGlobal      ContractSurface = "global"
)

type BuildOrchestrationFlags struct {
	EnableIntentBrief              bool `json:"enable_intent_brief"`
	EnableBuildContract            bool `json:"enable_build_contract"`
	EnableContractVerification     bool `json:"enable_contract_verification"`
	EnablePatchBundles             bool `json:"enable_patch_bundles"`
	EnableSurfaceLocalVerification bool `json:"enable_surface_local_verification"`
	EnableSelectiveEscalation      bool `json:"enable_selective_escalation"`
	EnableRepairLadder             bool `json:"enable_repair_ladder"`
	EnablePromotionDecision        bool `json:"enable_promotion_decision"`
	EnableFailureFingerprinting    bool `json:"enable_failure_fingerprinting"`
	EnableProviderScorecards       bool `json:"enable_provider_scorecards"`
	HostedProvidersOnly            bool `json:"hosted_providers_only"`
}

type IntentBrief struct {
	ID                    string                  `json:"id"`
	NormalizedRequest     string                  `json:"normalized_request"`
	AppType               string                  `json:"app_type"`
	RequiredFeatures      []string                `json:"required_features,omitempty"`
	NonGoals              []string                `json:"non_goals,omitempty"`
	ComplexityClass       BuildComplexityClass    `json:"complexity_class"`
	RequiredCapabilities  []CapabilityRequirement `json:"required_capabilities,omitempty"`
	DeploymentTarget      string                  `json:"deployment_target,omitempty"`
	RiskFlags             []string                `json:"risk_flags,omitempty"`
	CostSensitivity       CostSensitivity         `json:"cost_sensitivity"`
	AcceptanceSummarySeed []string                `json:"acceptance_summary_seed,omitempty"`
	CreatedAt             time.Time               `json:"created_at"`
}

type ContractRoute struct {
	Path    string          `json:"path"`
	File    string          `json:"file,omitempty"`
	Surface ContractSurface `json:"surface"`
}

type ContractBackendResource struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	DependsOn []string `json:"depends_on,omitempty"`
}

type ContractAuthStrategy struct {
	Required         bool   `json:"required"`
	Provider         string `json:"provider,omitempty"`
	SessionStrategy  string `json:"session_strategy,omitempty"`
	TokenStrategy    string `json:"token_strategy,omitempty"`
	CallbackStrategy string `json:"callback_strategy,omitempty"`
}

type ContractDependency struct {
	Name    string `json:"name"`
	Surface string `json:"surface"`
	Reason  string `json:"reason,omitempty"`
}

type RuntimeCommandContract struct {
	FrontendInstall string `json:"frontend_install,omitempty"`
	FrontendBuild   string `json:"frontend_build,omitempty"`
	FrontendPreview string `json:"frontend_preview,omitempty"`
	BackendInstall  string `json:"backend_install,omitempty"`
	BackendBuild    string `json:"backend_build,omitempty"`
	BackendStart    string `json:"backend_start,omitempty"`
	TestCommand     string `json:"test_command,omitempty"`
}

type SurfaceAcceptanceContract struct {
	Surface  ContractSurface `json:"surface"`
	Checks   []string        `json:"checks,omitempty"`
	Required bool            `json:"required"`
}

type SurfaceVerificationGate struct {
	Surface ContractSurface `json:"surface"`
	Gates   []string        `json:"gates,omitempty"`
}

type BuildContract struct {
	ID                   string                      `json:"id"`
	BuildID              string                      `json:"build_id"`
	SpecHash             string                      `json:"spec_hash,omitempty"`
	AppType              string                      `json:"app_type"`
	DeliveryMode         string                      `json:"delivery_mode,omitempty"`
	RoutePageMap         []ContractRoute             `json:"route_page_map,omitempty"`
	BackendResourceMap   []ContractBackendResource   `json:"backend_resource_map,omitempty"`
	APIContract          *BuildAPIContract           `json:"api_contract,omitempty"`
	DBSchemaContract     []DataModel                 `json:"db_schema_contract,omitempty"`
	AuthContract         *ContractAuthStrategy       `json:"auth_contract,omitempty"`
	EnvVarContract       []BuildEnvVar               `json:"env_var_contract,omitempty"`
	DependencySkeleton   []ContractDependency        `json:"dependency_skeleton,omitempty"`
	FileOwnershipPlan    []BuildOwnership            `json:"file_ownership_plan,omitempty"`
	RuntimeContract      RuntimeCommandContract      `json:"runtime_contract"`
	AcceptanceBySurface  []SurfaceAcceptanceContract `json:"acceptance_by_surface,omitempty"`
	VerificationGates    []SurfaceVerificationGate   `json:"required_verification_gates,omitempty"`
	TruthBySurface       map[string][]TruthTag       `json:"truth_by_surface,omitempty"`
	VerificationWarnings []string                    `json:"verification_warnings,omitempty"`
	VerificationBlockers []string                    `json:"verification_blockers,omitempty"`
	Verified             bool                        `json:"verified"`
	VerifiedAt           *time.Time                  `json:"verified_at,omitempty"`
}

type WorkOrderContractSlice struct {
	Surface         ContractSurface `json:"surface"`
	OwnedChecks     []string        `json:"owned_checks,omitempty"`
	RelevantRoutes  []string        `json:"relevant_routes,omitempty"`
	RelevantEnvVars []string        `json:"relevant_env_vars,omitempty"`
	RelevantModels  []string        `json:"relevant_models,omitempty"`
	TruthTags       []TruthTag      `json:"truth_tags,omitempty"`
}

type WorkOrder struct {
	ID                 string                 `json:"id"`
	BuildID            string                 `json:"build_id"`
	Role               AgentRole              `json:"role"`
	Category           WorkOrderCategory      `json:"category"`
	TaskShape          TaskShape              `json:"task_shape"`
	Summary            string                 `json:"summary,omitempty"`
	OwnedFiles         []string               `json:"owned_files,omitempty"`
	RequiredFiles      []string               `json:"required_files,omitempty"`
	ReadableFiles      []string               `json:"readable_files,omitempty"`
	ForbiddenFiles     []string               `json:"forbidden_files,omitempty"`
	ContractSlice      WorkOrderContractSlice `json:"contract_slice"`
	RequiredOutputs    []string               `json:"required_outputs,omitempty"`
	RequiredSymbols    []string               `json:"required_exports,omitempty"`
	SurfaceLocalChecks []string               `json:"surface_local_acceptance_checks,omitempty"`
	MaxContextBudget   int                    `json:"max_context_budget"`
	RiskLevel          TaskRiskLevel          `json:"risk_level"`
	RoutingMode        ProviderRoutingMode    `json:"routing_mode"`
	PreferredProvider  ai.AIProvider          `json:"preferred_provider,omitempty"`
}

type PatchOperationType string

const (
	PatchCreateFile             PatchOperationType = "create_file"
	PatchReplaceSymbol          PatchOperationType = "replace_symbol"
	PatchReplaceFunction        PatchOperationType = "replace_function"
	PatchInsertAfterSymbol      PatchOperationType = "insert_after_symbol"
	PatchPatchJSONKey           PatchOperationType = "patch_json_key"
	PatchPatchEnvVar            PatchOperationType = "patch_env_var"
	PatchPatchRouteRegistration PatchOperationType = "patch_route_registration"
	PatchPatchDependency        PatchOperationType = "patch_dependency"
	PatchPatchSchemaEntity      PatchOperationType = "patch_schema_entity"
	PatchDeleteBlock            PatchOperationType = "delete_block"
	PatchRenameSymbol           PatchOperationType = "rename_symbol"
)

type PatchOperation struct {
	Type           PatchOperationType `json:"type"`
	Path           string             `json:"path,omitempty"`
	Symbol         string             `json:"symbol,omitempty"`
	Anchor         string             `json:"anchor,omitempty"`
	Content        string             `json:"content,omitempty"`
	Key            string             `json:"key,omitempty"`
	Value          string             `json:"value,omitempty"`
	OldName        string             `json:"old_name,omitempty"`
	NewName        string             `json:"new_name,omitempty"`
	ConflictPolicy string             `json:"conflict_policy,omitempty"`
}

type PatchBundle struct {
	ID               string           `json:"id"`
	BuildID          string           `json:"build_id"`
	WorkOrderID      string           `json:"work_order_id,omitempty"`
	Provider         ai.AIProvider    `json:"provider,omitempty"`
	WholeFileRewrite bool             `json:"whole_file_rewrite,omitempty"`
	Justification    string           `json:"justification,omitempty"`
	Operations       []PatchOperation `json:"operations,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
}

type VerificationReport struct {
	ID              string             `json:"id"`
	BuildID         string             `json:"build_id"`
	WorkOrderID     string             `json:"work_order_id,omitempty"`
	Phase           string             `json:"phase"`
	Surface         ContractSurface    `json:"surface"`
	Status          VerificationStatus `json:"status"`
	Deterministic   bool               `json:"deterministic"`
	Provider        ai.AIProvider      `json:"provider,omitempty"`
	ChecksRun       []string           `json:"checks_run,omitempty"`
	Warnings        []string           `json:"warnings,omitempty"`
	Errors          []string           `json:"errors,omitempty"`
	Blockers        []string           `json:"blockers,omitempty"`
	TruthTags       []TruthTag         `json:"truth_tags,omitempty"`
	ConfidenceScore float64            `json:"confidence_score,omitempty"`
	GeneratedAt     time.Time          `json:"generated_at"`
}

type PromotionDecision struct {
	ID                  string                `json:"id"`
	BuildID             string                `json:"build_id"`
	ReadinessState      ReadinessState        `json:"readiness_state"`
	UnresolvedBlockers  []string              `json:"unresolved_blockers,omitempty"`
	ConfidenceScore     float64               `json:"confidence_score"`
	TruthBySurface      map[string][]TruthTag `json:"truth_by_surface,omitempty"`
	FullBuildResults    []string              `json:"full_build_results,omitempty"`
	PreviewReady        bool                  `json:"preview_ready"`
	IntegrationReady    bool                  `json:"integration_ready"`
	ProductionCandidate bool                  `json:"production_candidate"`
	GeneratedAt         time.Time             `json:"generated_at"`
}

type FailureFingerprint struct {
	ID                  string        `json:"id"`
	BuildID             string        `json:"build_id"`
	StackCombination    string        `json:"stack_combination,omitempty"`
	TaskShape           TaskShape     `json:"task_shape"`
	Provider            ai.AIProvider `json:"provider,omitempty"`
	Model               string        `json:"model,omitempty"`
	FailureClass        string        `json:"failure_class"`
	FilesInvolved       []string      `json:"files_involved,omitempty"`
	RepairPathChosen    []string      `json:"repair_path_chosen,omitempty"`
	RepairSucceeded     bool          `json:"repair_success"`
	TokenCostToRecovery int           `json:"token_cost_to_recovery,omitempty"`
	CreatedAt           time.Time     `json:"created_at"`
}

type ProviderScorecard struct {
	Provider                  ai.AIProvider `json:"provider"`
	TaskShape                 TaskShape     `json:"task_shape"`
	CompilePassRate           float64       `json:"compile_pass_rate"`
	FirstPassVerificationRate float64       `json:"first_pass_verification_pass_rate"`
	RepairSuccessRate         float64       `json:"repair_success_rate"`
	TruncationRate            float64       `json:"truncation_rate"`
	AverageAcceptedTokens     float64       `json:"average_accepted_tokens_per_success"`
	AverageCostPerSuccess     float64       `json:"average_cost_per_success"`
	AverageLatencySeconds     float64       `json:"average_latency_seconds"`
	FailureClassRecurrence    float64       `json:"failure_class_recurrence"`
	PromotionRate             float64       `json:"promotion_rate"`
	HostedEligible            bool          `json:"hosted_eligible"`
	SampleCount               int           `json:"sample_count,omitempty"`
	SuccessCount              int           `json:"success_count,omitempty"`
	FirstPassSampleCount      int           `json:"first_pass_sample_count,omitempty"`
	FirstPassSuccessCount     int           `json:"first_pass_success_count,omitempty"`
	RepairAttemptCount        int           `json:"repair_attempt_count,omitempty"`
	RepairSuccessCount        int           `json:"repair_success_count,omitempty"`
	TruncationEventCount      int           `json:"truncation_event_count,omitempty"`
	FailureEventCount         int           `json:"failure_event_count,omitempty"`
	PromotionAttemptCount     int           `json:"promotion_attempt_count,omitempty"`
	PromotionSuccessCount     int           `json:"promotion_success_count,omitempty"`
	TokenSampleCount          int           `json:"token_sample_count,omitempty"`
	CostSampleCount           int           `json:"cost_sample_count,omitempty"`
	LatencySampleCount        int           `json:"latency_sample_count,omitempty"`
}

type BuildOrchestrationState struct {
	Flags               BuildOrchestrationFlags `json:"flags"`
	IntentBrief         *IntentBrief            `json:"intent_brief,omitempty"`
	BuildContract       *BuildContract          `json:"build_contract,omitempty"`
	WorkOrders          []WorkOrder             `json:"work_orders,omitempty"`
	PatchBundles        []PatchBundle           `json:"patch_bundles,omitempty"`
	VerificationReports []VerificationReport    `json:"verification_reports,omitempty"`
	PromotionDecision   *PromotionDecision      `json:"promotion_decision,omitempty"`
	FailureFingerprints []FailureFingerprint    `json:"failure_fingerprints,omitempty"`
	ProviderScorecards  []ProviderScorecard     `json:"provider_scorecards,omitempty"`
}

func defaultBuildOrchestrationFlags() BuildOrchestrationFlags {
	return BuildOrchestrationFlags{
		EnableIntentBrief:              envBool("APEX_ENABLE_INTENT_BRIEF", true),
		EnableBuildContract:            envBool("APEX_ENABLE_BUILD_CONTRACT", true),
		EnableContractVerification:     envBool("APEX_ENABLE_CONTRACT_VERIFICATION", true),
		EnablePatchBundles:             envBool("APEX_ENABLE_PATCH_BUNDLES", true),
		EnableSurfaceLocalVerification: envBool("APEX_ENABLE_SURFACE_LOCAL_VERIFICATION", true),
		EnableSelectiveEscalation:      envBool("APEX_ENABLE_SELECTIVE_ESCALATION", true),
		EnableRepairLadder:             envBool("APEX_ENABLE_REPAIR_LADDER", true),
		EnablePromotionDecision:        envBool("APEX_ENABLE_PROMOTION_DECISION", true),
		EnableFailureFingerprinting:    envBool("APEX_ENABLE_FAILURE_FINGERPRINTING", true),
		EnableProviderScorecards:       envBool("APEX_ENABLE_PROVIDER_SCORECARDS", true),
		HostedProvidersOnly:            envBool("APEX_HOSTED_PROVIDERS_ONLY", true),
	}
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func ensureBuildOrchestrationStateLocked(build *Build) *BuildOrchestrationState {
	if build == nil {
		return nil
	}
	if build.SnapshotState.Orchestration == nil {
		build.SnapshotState.Orchestration = &BuildOrchestrationState{
			Flags:              defaultBuildOrchestrationFlags(),
			ProviderScorecards: defaultProviderScorecards(build.ProviderMode),
		}
	}
	if build.SnapshotState.Orchestration.ProviderScorecards == nil {
		build.SnapshotState.Orchestration.ProviderScorecards = defaultProviderScorecards(build.ProviderMode)
	}
	return build.SnapshotState.Orchestration
}

func compileIntentBriefFromRequest(req *BuildRequest, providerMode string) *IntentBrief {
	if req == nil {
		return nil
	}
	normalized := normalizeCompactText(firstNonEmpty(req.Prompt, req.Description))
	if normalized == "" {
		return nil
	}
	appType := inferIntentAppType(normalized, req.TechStack)
	capabilities := detectRequiredCapabilities(normalized, req.TechStack)
	riskFlags := deriveIntentRiskFlags(capabilities, normalized)
	acceptance := deriveAcceptanceSeed(appType, capabilities, req.RequirePreviewReady)
	return &IntentBrief{
		ID:                    uuid.New().String(),
		NormalizedRequest:     normalized,
		AppType:               appType,
		RequiredFeatures:      capabilityStrings(capabilities),
		NonGoals:              deriveIntentNonGoals(normalized),
		ComplexityClass:       classifyIntentComplexity(capabilities, appType),
		RequiredCapabilities:  capabilities,
		DeploymentTarget:      deriveDeploymentTarget(normalized),
		RiskFlags:             riskFlags,
		CostSensitivity:       deriveCostSensitivity(normalized, req.PowerMode, providerMode),
		AcceptanceSummarySeed: acceptance,
		CreatedAt:             time.Now().UTC(),
	}
}

func compileBuildContractFromPlan(buildID string, intent *IntentBrief, plan *BuildPlan) *BuildContract {
	if plan == nil {
		return nil
	}
	apiContract := seedIntentAPIContract(cloneAPIContract(plan.APIContract), intent)
	normalizedPlan := *plan
	normalizedPlan.APIContract = apiContract
	normalizedPlan.APIEndpoints = apiEndpointsFromContract(apiContract)
	contract := &BuildContract{
		ID:                  uuid.New().String(),
		BuildID:             buildID,
		SpecHash:            plan.SpecHash,
		AppType:             firstNonEmpty(plan.AppType, intentAppType(intent)),
		DeliveryMode:        strings.TrimSpace(plan.DeliveryMode),
		RoutePageMap:        deriveContractRoutes(plan),
		BackendResourceMap:  deriveBackendResources(&normalizedPlan),
		APIContract:         apiContract,
		DBSchemaContract:    normalizeDataModels(plan.DataModels),
		AuthContract:        deriveAuthContract(&normalizedPlan, intent),
		EnvVarContract:      append([]BuildEnvVar(nil), plan.EnvVars...),
		DependencySkeleton:  deriveDependencySkeleton(plan),
		FileOwnershipPlan:   append([]BuildOwnership(nil), plan.Ownership...),
		RuntimeContract:     deriveRuntimeContract(plan),
		AcceptanceBySurface: deriveAcceptanceBySurface(plan),
		VerificationGates:   deriveVerificationGates(plan),
		TruthBySurface:      deriveInitialTruthBySurface(plan, intent),
	}
	return contract
}

func seedIntentAPIContract(contract *BuildAPIContract, intent *IntentBrief) *BuildAPIContract {
	needsRuntimeContract := contract != nil || capabilityRequired(intent, CapabilityAPI) || capabilityRequired(intent, CapabilityAuth)
	if !needsRuntimeContract {
		return nil
	}
	if contract == nil {
		contract = &BuildAPIContract{}
	}
	if contract.BackendPort == 0 && capabilityRequired(intent, CapabilityAPI) {
		contract.BackendPort = 3001
	}
	if strings.TrimSpace(contract.APIBaseURL) == "" && (capabilityRequired(intent, CapabilityAPI) || capabilityRequired(intent, CapabilityAuth)) {
		contract.APIBaseURL = "/api"
	}

	addEndpoint := func(method, path, description string, auth bool, output string) {
		for _, existing := range contract.Endpoints {
			if strings.EqualFold(strings.TrimSpace(existing.Method), method) && strings.EqualFold(strings.TrimSpace(existing.Path), path) {
				return
			}
		}
		contract.Endpoints = append(contract.Endpoints, APIEndpoint{
			Method:      method,
			Path:        path,
			Description: description,
			Auth:        auth,
			Output:      output,
		})
	}

	addEndpoint("GET", "/api/health", "Health check", false, `{ status: "ok" }`)

	if capabilityRequired(intent, CapabilityAuth) {
		addEndpoint("POST", "/api/auth/login", "Authenticate a user and start a session", false, `{ token: string, user: object }`)
		addEndpoint("GET", "/api/auth/me", "Return the current authenticated user", true, `{ user: object }`)
		if intentSuggestsAccountRegistration(intent) {
			addEndpoint("POST", "/api/auth/register", "Create a new account", false, `{ user: object }`)
		}
	}

	return contract
}

func intentSuggestsAccountRegistration(intent *IntentBrief) bool {
	if intent == nil {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(intent.NormalizedRequest))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "sign up") ||
		strings.Contains(normalized, "signup") ||
		strings.Contains(normalized, "register") ||
		strings.Contains(normalized, "create an account") ||
		strings.Contains(normalized, "create account")
}

func verifyAndNormalizeBuildContract(intent *IntentBrief, contract *BuildContract) (*BuildContract, VerificationReport) {
	report := VerificationReport{
		ID:            uuid.New().String(),
		BuildID:       buildContractBuildID(contract),
		Phase:         "contract_verification",
		Surface:       SurfaceGlobal,
		Status:        VerificationPassed,
		Deterministic: true,
		ChecksRun: []string{
			"required_capability_coverage",
			"surface_acceptance_completeness",
			"runtime_command_consistency",
			"frontend_backend_contract_coherence",
			"auth_contract_strategy",
			"schema_contract_coverage",
		},
		GeneratedAt: time.Now().UTC(),
	}
	if contract == nil {
		report.Status = VerificationBlocked
		report.Blockers = []string{"build contract is missing"}
		report.ConfidenceScore = 0
		return nil, report
	}

	corrected := *contract
	now := time.Now().UTC()

	if corrected.APIContract != nil && corrected.APIContract.APIBaseURL == "" &&
		hasSurface(corrected.AcceptanceBySurface, SurfaceIntegration) {
		corrected.APIContract.APIBaseURL = "/api"
		report.Warnings = append(report.Warnings, "filled missing API base URL from full-stack default")
	}
	if isEmptyRuntimeContract(corrected.RuntimeContract) {
		corrected.RuntimeContract = deriveRuntimeContractFromAppType(corrected.AppType)
		report.Warnings = append(report.Warnings, "filled missing runtime/build commands from stack defaults")
	}
	if len(corrected.AcceptanceBySurface) == 0 {
		corrected.AcceptanceBySurface = deriveAcceptanceBySurfaceFromAppType(corrected.AppType)
		report.Warnings = append(report.Warnings, "generated fallback acceptance criteria by surface")
	}
	if len(corrected.VerificationGates) == 0 {
		corrected.VerificationGates = deriveVerificationGatesFromAppType(corrected.AppType)
		report.Warnings = append(report.Warnings, "generated fallback verification gates by surface")
	}

	requiredCaps := map[CapabilityRequirement]bool{}
	if intent != nil {
		for _, cap := range intent.RequiredCapabilities {
			requiredCaps[cap] = true
		}
	}
	frontendPreviewOnly := strings.EqualFold(strings.TrimSpace(corrected.DeliveryMode), "frontend_preview_only")

	// Warn but don't block when API endpoints weren't pre-planned — they'll be generated from the spec.
	if requiredCaps[CapabilityAPI] && (corrected.APIContract == nil || len(corrected.APIContract.Endpoints) == 0) {
		if strings.TrimSpace(corrected.AppType) == "" {
			report.Blockers = append(report.Blockers, "API capability requested but API contract has no endpoints")
		} else {
			report.Warnings = append(report.Warnings, "API capability detected but no endpoints were pre-planned; endpoints will be derived during code generation")
		}
	}
	// Only hard-block if the app type is completely undefined AND no schema was extracted.
	// Missing schema entities alone is not a blocker — the LLM will generate schema from
	// the description during code generation. File storage (CapabilityStorage) never
	// requires pre-planned schema entities.
	if requiredCaps[CapabilityDatabase] &&
		strings.TrimSpace(corrected.AppType) == "" &&
		len(corrected.DBSchemaContract) == 0 {
		report.Blockers = append(report.Blockers, "database capability requested without app type or schema entities")
	}
	if requiredCaps[CapabilityDatabase] && len(corrected.DBSchemaContract) == 0 {
		if frontendPreviewOnly {
			report.Warnings = append(report.Warnings, "database-backed runtime is deferred to a later paid/full-stack pass; frontend preview mode stays truthful")
		}
		report.Warnings = append(report.Warnings, "database capability detected but no schema entities were pre-planned; schema will be derived during code generation")
	}
	if requiredCaps[CapabilityAuth] {
		if frontendPreviewOnly {
			report.Warnings = append(report.Warnings, "auth/runtime capability is deferred in frontend-preview-only mode")
		} else if corrected.AuthContract == nil || (!corrected.AuthContract.Required) {
			report.Blockers = append(report.Blockers, "auth capability requested without an auth contract")
		} else if corrected.AuthContract.CallbackStrategy == "" &&
			corrected.AuthContract.SessionStrategy == "" &&
			corrected.AuthContract.TokenStrategy == "" {
			report.Blockers = append(report.Blockers, "auth contract is missing callback/session/token strategy")
		}
	}
	if requiredCaps[CapabilityBilling] && frontendPreviewOnly {
		report.Warnings = append(report.Warnings, "billing/runtime capability is deferred in frontend-preview-only mode")
	} else if requiredCaps[CapabilityBilling] && !contractHasEnvVar(corrected.EnvVarContract, "STRIPE", "BILLING") {
		report.Blockers = append(report.Blockers, "billing capability requested without billing/env contract")
	}
	if !frontendPreviewOnly && requiresIntegrationSurface(corrected) && (corrected.APIContract == nil || corrected.APIContract.BackendPort == 0) {
		report.Blockers = append(report.Blockers, "frontend/backend integration requested without backend runtime contract")
	}
	if missingAcceptanceSurfaces(corrected.AcceptanceBySurface, corrected) {
		report.Blockers = append(report.Blockers, "acceptance criteria are incomplete for one or more required surfaces")
	}
	if runtimeCommandMismatch(corrected.RuntimeContract, corrected) {
		report.Blockers = append(report.Blockers, "runtime/build command contract is incomplete or inconsistent with declared surfaces")
	}

	if len(report.Blockers) > 0 {
		report.Status = VerificationBlocked
		report.Errors = append([]string(nil), report.Blockers...)
		report.ConfidenceScore = 0.2
		corrected.VerificationBlockers = append([]string(nil), report.Blockers...)
		corrected.VerificationWarnings = append([]string(nil), report.Warnings...)
		return &corrected, report
	}

	corrected.Verified = true
	corrected.VerifiedAt = &now
	corrected.VerificationWarnings = append([]string(nil), report.Warnings...)
	corrected.VerificationBlockers = nil
	report.ConfidenceScore = 0.88
	return &corrected, report
}

func compileWorkOrdersFromPlan(buildID string, contract *BuildContract, plan *BuildPlan, scorecards []ProviderScorecard) []WorkOrder {
	return compileWorkOrdersFromPlanWithCost(buildID, contract, plan, scorecards, CostSensitivityMedium)
}

// compileWorkOrdersFromPlanWithCost compiles work orders, routing each to the provider
// that best satisfies both task shape and the build's cost sensitivity constraint.
func compileWorkOrdersFromPlanWithCost(buildID string, contract *BuildContract, plan *BuildPlan, scorecards []ProviderScorecard, sensitivity CostSensitivity) []WorkOrder {
	if plan == nil {
		return nil
	}
	orders := make([]WorkOrder, 0, len(plan.WorkOrders))
	for _, order := range plan.WorkOrders {
		surface := contractSurfaceForRole(order.Role)
		taskShape := taskShapeForRole(order.Role)
		orders = append(orders, WorkOrder{
			ID:                 uuid.New().String(),
			BuildID:            buildID,
			Role:               order.Role,
			Category:           workOrderCategoryForRole(order.Role),
			TaskShape:          taskShape,
			Summary:            strings.TrimSpace(order.Summary),
			OwnedFiles:         append([]string(nil), order.OwnedFiles...),
			RequiredFiles:      append([]string(nil), order.RequiredFiles...),
			ReadableFiles:      readableFilesForWorkOrder(order),
			ForbiddenFiles:     append([]string(nil), order.ForbiddenFiles...),
			ContractSlice:      buildWorkOrderContractSlice(surface, order, contract, plan),
			RequiredOutputs:    append([]string(nil), order.RequiredOutputs...),
			RequiredSymbols:    deriveRequiredSymbols(order, contract),
			SurfaceLocalChecks: append([]string(nil), order.AcceptanceChecks...),
			MaxContextBudget:   contextBudgetForRole(order.Role),
			RiskLevel:          riskLevelForWorkOrder(order.Role, contract),
			RoutingMode:        routingModeForRole(order.Role, contract),
			PreferredProvider:  preferredProviderForTaskShapeWithCost(taskShape, scorecards, sensitivity),
		})
	}
	return orders
}

func cloneWorkOrderArtifact(workOrder *WorkOrder) *WorkOrder {
	if workOrder == nil {
		return nil
	}
	clone := *workOrder
	clone.OwnedFiles = append([]string(nil), workOrder.OwnedFiles...)
	clone.RequiredFiles = append([]string(nil), workOrder.RequiredFiles...)
	clone.ReadableFiles = append([]string(nil), workOrder.ReadableFiles...)
	clone.ForbiddenFiles = append([]string(nil), workOrder.ForbiddenFiles...)
	clone.RequiredOutputs = append([]string(nil), workOrder.RequiredOutputs...)
	clone.RequiredSymbols = append([]string(nil), workOrder.RequiredSymbols...)
	clone.SurfaceLocalChecks = append([]string(nil), workOrder.SurfaceLocalChecks...)
	clone.ContractSlice = WorkOrderContractSlice{
		Surface:         workOrder.ContractSlice.Surface,
		OwnedChecks:     append([]string(nil), workOrder.ContractSlice.OwnedChecks...),
		RelevantRoutes:  append([]string(nil), workOrder.ContractSlice.RelevantRoutes...),
		RelevantEnvVars: append([]string(nil), workOrder.ContractSlice.RelevantEnvVars...),
		RelevantModels:  append([]string(nil), workOrder.ContractSlice.RelevantModels...),
		TruthTags:       append([]TruthTag(nil), workOrder.ContractSlice.TruthTags...),
	}
	return &clone
}

func legacyBuildWorkOrderFromArtifact(workOrder *WorkOrder) *BuildWorkOrder {
	if workOrder == nil {
		return nil
	}
	return &BuildWorkOrder{
		Role:             workOrder.Role,
		Summary:          strings.TrimSpace(workOrder.Summary),
		OwnedFiles:       append([]string(nil), workOrder.OwnedFiles...),
		RequiredFiles:    append([]string(nil), workOrder.RequiredFiles...),
		ForbiddenFiles:   append([]string(nil), workOrder.ForbiddenFiles...),
		AcceptanceChecks: append([]string(nil), workOrder.SurfaceLocalChecks...),
		RequiredOutputs:  append([]string(nil), workOrder.RequiredOutputs...),
	}
}

func findOrchestrationWorkOrder(build *Build, role AgentRole) *WorkOrder {
	if build == nil {
		return nil
	}
	build.mu.RLock()
	defer build.mu.RUnlock()
	return findOrchestrationWorkOrderInState(build.SnapshotState.Orchestration, role)
}

func findOrchestrationWorkOrderInState(state *BuildOrchestrationState, role AgentRole) *WorkOrder {
	if state == nil {
		return nil
	}
	for i := range state.WorkOrders {
		if state.WorkOrders[i].Role == role {
			return cloneWorkOrderArtifact(&state.WorkOrders[i])
		}
	}
	return nil
}

func compilePromotionDecision(build *Build, readinessErrors []string) *PromotionDecision {
	if build == nil {
		return nil
	}
	build.mu.RLock()
	orch := cloneBuildOrchestrationState(build.SnapshotState.Orchestration)
	status := build.Status
	requirePreviewReady := build.RequirePreviewReady
	plan := build.Plan
	buildErr := strings.TrimSpace(build.Error)
	build.mu.RUnlock()
	truthBySurface := map[string][]TruthTag{}
	if orch != nil && orch.BuildContract != nil {
		for surface, tags := range orch.BuildContract.TruthBySurface {
			truthBySurface[surface] = append([]TruthTag(nil), tags...)
		}
	}

	readiness := ReadinessBlocked
	confidence := 0.2
	results := []string{}
	switch status {
	case BuildCompleted:
		hasBackend := plan != nil && strings.TrimSpace(plan.TechStack.Backend) != ""
		hasFrontend := plan != nil && strings.TrimSpace(plan.TechStack.Frontend) != ""
		if requirePreviewReady || hasFrontend {
			readiness = ReadinessPreviewReady
		} else {
			readiness = ReadinessPrototypeReady
		}
		if hasFrontend && hasBackend {
			readiness = ReadinessIntegrationReady
		}
		confidence = 0.86
		results = append(results, "final readiness validation passed")
	case BuildCancelled:
		results = append(results, "build cancelled before promotion")
	default:
		if buildErr != "" {
			results = append(results, buildErr)
		}
	}
	if len(readinessErrors) > 0 {
		results = append(results, readinessErrors...)
	}

	for surface, tags := range truthBySurface {
		next := append([]TruthTag(nil), tags...)
		if status == BuildCompleted && len(readinessErrors) == 0 {
			next = appendTruthTag(next, TruthVerified)
			if requirePreviewReady && surface == string(SurfaceDeployment) {
				next = appendTruthTag(next, TruthVerified)
			}
		} else {
			next = appendTruthTag(next, TruthBlocked)
		}
		truthBySurface[surface] = normalizeTruthTags(next)
	}

	productionCandidate := readiness == ReadinessIntegrationReady && noOutstandingTruthConstraints(truthBySurface)
	if productionCandidate {
		readiness = ReadinessProductionCand
		confidence = 0.93
	}

	return &PromotionDecision{
		ID:                  uuid.New().String(),
		BuildID:             build.ID,
		ReadinessState:      readiness,
		UnresolvedBlockers:  append([]string(nil), readinessErrors...),
		ConfidenceScore:     confidence,
		TruthBySurface:      truthBySurface,
		FullBuildResults:    results,
		PreviewReady:        readiness == ReadinessPreviewReady || readiness == ReadinessIntegrationReady || readiness == ReadinessProductionCand,
		IntegrationReady:    readiness == ReadinessIntegrationReady || readiness == ReadinessProductionCand,
		ProductionCandidate: productionCandidate,
		GeneratedAt:         time.Now().UTC(),
	}
}

func defaultProviderScorecards(providerMode string) []ProviderScorecard {
	hostedEligible := hostedProviderMode(providerMode)
	scorecards := []ProviderScorecard{
		{Provider: ai.ProviderClaude, TaskShape: TaskShapeContract, CompilePassRate: 0.92, FirstPassVerificationRate: 0.89, RepairSuccessRate: 0.84, TruncationRate: 0.04, AverageAcceptedTokens: 9500, AverageCostPerSuccess: 0.11, AverageLatencySeconds: 8.0, FailureClassRecurrence: 0.15, PromotionRate: 0.83, HostedEligible: true},
		{Provider: ai.ProviderClaude, TaskShape: TaskShapeDiagnosis, CompilePassRate: 0.90, FirstPassVerificationRate: 0.91, RepairSuccessRate: 0.88, TruncationRate: 0.03, AverageAcceptedTokens: 8200, AverageCostPerSuccess: 0.10, AverageLatencySeconds: 7.8, FailureClassRecurrence: 0.13, PromotionRate: 0.85, HostedEligible: true},
		{Provider: ai.ProviderGPT4, TaskShape: TaskShapeFrontendPatch, CompilePassRate: 0.91, FirstPassVerificationRate: 0.87, RepairSuccessRate: 0.86, TruncationRate: 0.05, AverageAcceptedTokens: 7600, AverageCostPerSuccess: 0.12, AverageLatencySeconds: 7.2, FailureClassRecurrence: 0.16, PromotionRate: 0.82, HostedEligible: true},
		{Provider: ai.ProviderGPT4, TaskShape: TaskShapeBackendPatch, CompilePassRate: 0.92, FirstPassVerificationRate: 0.88, RepairSuccessRate: 0.87, TruncationRate: 0.04, AverageAcceptedTokens: 7300, AverageCostPerSuccess: 0.12, AverageLatencySeconds: 7.0, FailureClassRecurrence: 0.15, PromotionRate: 0.84, HostedEligible: true},
		{Provider: ai.ProviderGPT4, TaskShape: TaskShapeRepair, CompilePassRate: 0.89, FirstPassVerificationRate: 0.83, RepairSuccessRate: 0.88, TruncationRate: 0.06, AverageAcceptedTokens: 6100, AverageCostPerSuccess: 0.09, AverageLatencySeconds: 6.9, FailureClassRecurrence: 0.17, PromotionRate: 0.80, HostedEligible: true},
		{Provider: ai.ProviderGemini, TaskShape: TaskShapeVerification, CompilePassRate: 0.94, FirstPassVerificationRate: 0.93, RepairSuccessRate: 0.79, TruncationRate: 0.02, AverageAcceptedTokens: 5400, AverageCostPerSuccess: 0.06, AverageLatencySeconds: 5.8, FailureClassRecurrence: 0.11, PromotionRate: 0.88, HostedEligible: true},
		{Provider: ai.ProviderGemini, TaskShape: TaskShapePromotion, CompilePassRate: 0.93, FirstPassVerificationRate: 0.92, RepairSuccessRate: 0.76, TruncationRate: 0.02, AverageAcceptedTokens: 5600, AverageCostPerSuccess: 0.06, AverageLatencySeconds: 5.9, FailureClassRecurrence: 0.12, PromotionRate: 0.87, HostedEligible: true},
		// Grok-4.20-reasoning: strong coder and repair model — real build contributor
		{Provider: ai.ProviderGrok, TaskShape: TaskShapeBackendPatch, CompilePassRate: 0.90, FirstPassVerificationRate: 0.86, RepairSuccessRate: 0.88, TruncationRate: 0.04, AverageAcceptedTokens: 7800, AverageCostPerSuccess: 0.10, AverageLatencySeconds: 7.5, FailureClassRecurrence: 0.15, PromotionRate: 0.83, HostedEligible: true},
		{Provider: ai.ProviderGrok, TaskShape: TaskShapeRepair, CompilePassRate: 0.91, FirstPassVerificationRate: 0.87, RepairSuccessRate: 0.91, TruncationRate: 0.04, AverageAcceptedTokens: 6800, AverageCostPerSuccess: 0.09, AverageLatencySeconds: 7.2, FailureClassRecurrence: 0.13, PromotionRate: 0.85, HostedEligible: true},
		{Provider: ai.ProviderGrok, TaskShape: TaskShapeDiagnosis, CompilePassRate: 0.88, FirstPassVerificationRate: 0.89, RepairSuccessRate: 0.87, TruncationRate: 0.04, AverageAcceptedTokens: 7200, AverageLatencySeconds: 7.0, FailureClassRecurrence: 0.14, PromotionRate: 0.84, HostedEligible: true},
		{Provider: ai.ProviderGrok, TaskShape: TaskShapeAdversarialCritique, CompilePassRate: 0.81, FirstPassVerificationRate: 0.84, RepairSuccessRate: 0.68, TruncationRate: 0.05, AverageAcceptedTokens: 4300, AverageCostPerSuccess: 0.04, AverageLatencySeconds: 4.8, FailureClassRecurrence: 0.21, PromotionRate: 0.74, HostedEligible: true},
		{Provider: ai.ProviderOllama, TaskShape: TaskShapeRepair, CompilePassRate: 0.74, FirstPassVerificationRate: 0.70, RepairSuccessRate: 0.72, TruncationRate: 0.09, AverageAcceptedTokens: 6800, AverageCostPerSuccess: 0, AverageLatencySeconds: 14.0, FailureClassRecurrence: 0.27, PromotionRate: 0.68, HostedEligible: !hostedEligible},
	}
	if hostedEligible {
		filtered := make([]ProviderScorecard, 0, len(scorecards))
		for _, scorecard := range scorecards {
			if scorecard.Provider == ai.ProviderOllama {
				continue
			}
			filtered = append(filtered, scorecard)
		}
		return filtered
	}
	return scorecards
}

func preferredProviderForTaskShape(shape TaskShape, scorecards []ProviderScorecard) ai.AIProvider {
	ranked := rankedProvidersForTaskShape(shape, scorecards)
	if len(ranked) == 0 {
		return ""
	}
	return ranked[0]
}

func rankedProvidersForTaskShape(shape TaskShape, scorecards []ProviderScorecard) []ai.AIProvider {
	return rankedProvidersForTaskShapeWithCost(shape, scorecards, CostSensitivityMedium)
}

// rankedProvidersForTaskShapeWithCost ranks providers for a given task shape, using the
// build's CostSensitivity as a hard constraint on the cost-penalty weighting.
func rankedProvidersForTaskShapeWithCost(shape TaskShape, scorecards []ProviderScorecard, sensitivity CostSensitivity) []ai.AIProvider {
	type providerScore struct {
		provider ai.AIProvider
		score    float64
	}
	ranked := make([]providerScore, 0, len(scorecards))
	for _, scorecard := range scorecards {
		if scorecard.TaskShape != shape {
			continue
		}
		ranked = append(ranked, providerScore{provider: scorecard.Provider, score: scoreProviderScorecardWithCost(scorecard, sensitivity)})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].provider < ranked[j].provider
		}
		return ranked[i].score > ranked[j].score
	})
	out := make([]ai.AIProvider, 0, len(ranked))
	seen := map[ai.AIProvider]bool{}
	for _, item := range ranked {
		if item.provider == "" || seen[item.provider] {
			continue
		}
		seen[item.provider] = true
		out = append(out, item.provider)
	}
	return out
}

func preferredProviderForTaskShapeWithCost(shape TaskShape, scorecards []ProviderScorecard, sensitivity CostSensitivity) ai.AIProvider {
	ranked := rankedProvidersForTaskShapeWithCost(shape, scorecards, sensitivity)
	if len(ranked) == 0 {
		return ""
	}
	return ranked[0]
}

func scoreProviderScorecard(scorecard ProviderScorecard) float64 {
	return scoreProviderScorecardWithCost(scorecard, CostSensitivityMedium)
}

// scoreProviderScorecardWithCost scores a provider scorecard factoring in cost sensitivity.
// CostSensitivityHigh amplifies the cost penalty (budget mode — prefer cheap providers).
// CostSensitivityLow reduces the penalty (PowerMax mode — prefer quality regardless of cost).
func scoreProviderScorecardWithCost(scorecard ProviderScorecard, sensitivity CostSensitivity) float64 {
	costPenaltyMultiplier := 1.0
	switch sensitivity {
	case CostSensitivityHigh:
		costPenaltyMultiplier = 3.0 // strongly penalise expensive providers on fast/budget mode
	case CostSensitivityLow:
		costPenaltyMultiplier = 0.1 // nearly ignore cost when quality is paramount (PowerMax)
	}
	score := scorecard.CompilePassRate + scorecard.FirstPassVerificationRate + scorecard.RepairSuccessRate + scorecard.PromotionRate
	score -= scorecard.TruncationRate + scorecard.FailureClassRecurrence
	score -= scorecard.AverageCostPerSuccess * 0.5 * costPenaltyMultiplier
	return score
}

func hostedProviderMode(providerMode string) bool {
	return strings.TrimSpace(strings.ToLower(providerMode)) != "byok"
}

func providerModeHintForProviders(providers []ai.AIProvider) string {
	if len(providers) == 0 {
		return "platform"
	}
	if len(hostedPlatformProviders(providers)) == len(providers) {
		return "platform"
	}
	return "byok"
}

func hostedPlatformProviders(providers []ai.AIProvider) []ai.AIProvider {
	if len(providers) == 0 {
		return nil
	}
	filtered := make([]ai.AIProvider, 0, len(providers))
	for _, provider := range providers {
		switch provider {
		case ai.ProviderClaude, ai.ProviderGPT4, ai.ProviderGemini, ai.ProviderGrok:
			filtered = append(filtered, provider)
		}
	}
	return filtered
}

func normalizeCompactText(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	return strings.Join(strings.Fields(trimmed), " ")
}

func normalizeDetectionText(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	mapped := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, input)
	return " " + strings.Join(strings.Fields(mapped), " ") + " "
}

func containsAffirmedTerm(normalized string, term string) bool {
	descriptionTokens := strings.Fields(normalized)
	termTokens := strings.Fields(term)
	if len(descriptionTokens) == 0 || len(termTokens) == 0 {
		return false
	}

	for start := 0; start <= len(descriptionTokens)-len(termTokens); start++ {
		if !matchesTokenSequence(descriptionTokens, termTokens, start) {
			continue
		}
		if termSequenceNegated(descriptionTokens, start) {
			continue
		}
		return true
	}
	return false
}

func containsAnyAffirmedTerm(normalized string, terms []string) bool {
	for _, term := range terms {
		if containsAffirmedTerm(normalized, term) {
			return true
		}
	}
	return false
}

func matchesTokenSequence(descriptionTokens []string, termTokens []string, start int) bool {
	for offset, token := range termTokens {
		if descriptionTokens[start+offset] != token {
			return false
		}
	}
	return true
}

func termSequenceNegated(descriptionTokens []string, start int) bool {
	lookback := 3
	if start < lookback {
		lookback = start
	}
	for idx := start - 1; idx >= start-lookback; idx-- {
		switch descriptionTokens[idx] {
		case "no", "without", "avoid", "skip", "omit":
			return true
		case "not":
			return true
		}
	}

	if start >= 2 && descriptionTokens[start-2] == "do" && descriptionTokens[start-1] == "not" {
		return true
	}
	if spansNegatedRequirementClause(descriptionTokens, start) {
		return true
	}
	return false
}

func spansNegatedRequirementClause(descriptionTokens []string, start int) bool {
	if start <= 0 {
		return false
	}

	negationTokens := map[string]bool{
		"no":      true,
		"without": true,
		"avoid":   true,
		"skip":    true,
		"omit":    true,
	}
	requirementVerbs := map[string]bool{
		"require":    true,
		"requires":   true,
		"required":   true,
		"requiring":  true,
		"need":       true,
		"needs":      true,
		"needed":     true,
		"needing":    true,
		"handle":     true,
		"handles":    true,
		"handled":    true,
		"handling":   true,
		"support":    true,
		"supports":   true,
		"supported":  true,
		"supporting": true,
	}

	windowStart := start - 6
	if windowStart < 0 {
		windowStart = 0
	}
	for negIdx := start - 1; negIdx >= windowStart; negIdx-- {
		if !negationTokens[descriptionTokens[negIdx]] {
			continue
		}
		for idx := negIdx + 1; idx < start; idx++ {
			if requirementVerbs[descriptionTokens[idx]] {
				return true
			}
		}
	}
	return false
}

func inferIntentAppType(description string, stack *TechStack) string {
	if stack != nil {
		if strings.TrimSpace(stack.Frontend) != "" && strings.TrimSpace(stack.Backend) != "" {
			return "fullstack"
		}
		if strings.TrimSpace(stack.Backend) != "" {
			return "api"
		}
		if strings.TrimSpace(stack.Frontend) != "" {
			return "web"
		}
	}
	normalized := normalizeDetectionText(description)
	dashboardWithRuntimeSignals := containsAffirmedTerm(normalized, normalizeDetectionText("dashboard")) &&
		containsAnyAffirmedTerm(normalized, []string{
			normalizeDetectionText("auth"),
			normalizeDetectionText("login"),
			normalizeDetectionText("signup"),
			normalizeDetectionText("database"),
			normalizeDetectionText("postgres"),
			normalizeDetectionText("mysql"),
			normalizeDetectionText("sqlite"),
			normalizeDetectionText("stripe"),
			normalizeDetectionText("billing"),
			normalizeDetectionText("payment"),
			normalizeDetectionText("subscription"),
			normalizeDetectionText("api"),
			normalizeDetectionText("backend"),
			normalizeDetectionText("server"),
			normalizeDetectionText("endpoint"),
		})
	switch {
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("full stack"),
		normalizeDetectionText("fullstack"),
		normalizeDetectionText("app with api"),
	}) || dashboardWithRuntimeSignals:
		return "fullstack"
	case containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("api"),
		normalizeDetectionText("backend"),
		normalizeDetectionText("service"),
		normalizeDetectionText("server"),
	}):
		return "api"
	default:
		return "web"
	}
}

func detectRequiredCapabilities(description string, stack *TechStack) []CapabilityRequirement {
	normalized := normalizeDetectionText(description)
	caps := []CapabilityRequirement{}
	if stack != nil && strings.TrimSpace(stack.Backend) != "" {
		caps = append(caps, CapabilityAPI)
	} else if containsAnyAffirmedTerm(normalized, []string{
		normalizeDetectionText("api"),
		normalizeDetectionText("backend"),
		normalizeDetectionText("server"),
		normalizeDetectionText("webhook"),
		normalizeDetectionText("endpoint"),
	}) {
		caps = append(caps, CapabilityAPI)
	}
	detect := []struct {
		cap      CapabilityRequirement
		keywords []string
	}{
		{CapabilityAuth, []string{"auth", "login", "signup", "session", "oauth"}},
		{CapabilityDatabase, []string{"database", "postgres", "mysql", "sqlite", "record", "persist"}},
		{CapabilityStorage, []string{"upload", "uploads", "storage", "file storage", "s3", "blob", "bucket", "object store"}},
		{CapabilityJobs, []string{"queue", "job", "worker", "cron", "background"}},
		{CapabilityBilling, []string{"stripe", "billing", "payment", "subscription"}},
		{CapabilitySearch, []string{"search", "filter", "query"}},
		{CapabilityRealtime, []string{"realtime", "real-time", "stream", "websocket", "socket"}},
		{CapabilityFileUpload, []string{"upload", "uploads", "file upload", "transcribe", "attachment", "attachments"}},
		{CapabilityExternalAPI, []string{"api integration", "third-party", "openai", "anthropic", "external api"}},
	}
	for _, item := range detect {
		for _, keyword := range item.keywords {
			term := normalizeDetectionText(keyword)
			if containsAffirmedTerm(normalized, term) {
				caps = append(caps, item.cap)
				break
			}
		}
	}
	if stack != nil && strings.TrimSpace(stack.Database) != "" {
		caps = append(caps, CapabilityDatabase)
	}
	return dedupeCapabilities(caps)
}

func deriveIntentRiskFlags(capabilities []CapabilityRequirement, description string) []string {
	flags := []string{}
	for _, cap := range capabilities {
		switch cap {
		case CapabilityAuth:
			flags = append(flags, "auth")
		case CapabilityBilling:
			flags = append(flags, "billing")
		case CapabilityDatabase:
			flags = append(flags, "data_integrity")
		case CapabilityExternalAPI:
			flags = append(flags, "external_api")
		case CapabilityJobs:
			flags = append(flags, "background_jobs")
		}
	}
	if strings.Contains(strings.ToLower(description), "production") {
		flags = append(flags, "production_intent")
	}
	return dedupeStrings(flags)
}

func deriveIntentNonGoals(description string) []string {
	lower := strings.ToLower(description)
	nonGoals := []string{}
	for _, marker := range []string{"without", "do not", "don't", "no "} {
		if idx := strings.Index(lower, marker); idx >= 0 {
			fragment := strings.TrimSpace(description[idx:])
			if fragment != "" {
				nonGoals = append(nonGoals, fragment)
			}
		}
	}
	return dedupeStrings(nonGoals)
}

func classifyIntentComplexity(capabilities []CapabilityRequirement, appType string) BuildComplexityClass {
	score := len(capabilities)
	if appType == "fullstack" {
		score++
	}
	switch {
	case score >= 5:
		return ComplexityComplex
	case score >= 3:
		return ComplexityMedium
	default:
		return ComplexitySimple
	}
}

func deriveDeploymentTarget(description string) string {
	lower := strings.ToLower(description)
	for _, target := range []string{"render", "vercel", "fly", "docker", "kubernetes"} {
		if strings.Contains(lower, target) {
			return target
		}
	}
	return "preview"
}

func deriveCostSensitivity(description string, mode PowerMode, providerMode string) CostSensitivity {
	lower := strings.ToLower(description)
	if strings.Contains(lower, "cheap") || strings.Contains(lower, "budget") || mode == PowerFast {
		return CostSensitivityHigh
	}
	if mode == PowerMax && hostedProviderMode(providerMode) {
		return CostSensitivityLow
	}
	return CostSensitivityMedium
}

func deriveAcceptanceSeed(appType string, capabilities []CapabilityRequirement, requirePreview bool) []string {
	seed := []string{}
	switch appType {
	case "fullstack":
		seed = append(seed, "frontend routes render", "backend API responds", "frontend/backend contract aligns", "interactive preview serves the app")
	case "api":
		seed = append(seed, "API contract endpoints respond")
	default:
		seed = append(seed, "frontend renders and navigation works", "interactive preview serves the app")
	}
	if requirePreview {
		seed = append(seed, "preview build installs, compiles, and serves")
	}
	for _, cap := range capabilities {
		switch cap {
		case CapabilityAuth:
			seed = append(seed, "auth flow has session/token strategy")
		case CapabilityDatabase:
			seed = append(seed, "schema entities cover required data")
		case CapabilityBilling:
			seed = append(seed, "billing secrets and callback paths are declared")
		}
	}
	return dedupeStrings(seed)
}

func capabilityStrings(capabilities []CapabilityRequirement) []string {
	out := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		out = append(out, string(capability))
	}
	return out
}

func intentAppType(intent *IntentBrief) string {
	if intent == nil {
		return ""
	}
	return intent.AppType
}

func deriveContractRoutes(plan *BuildPlan) []ContractRoute {
	routes := []ContractRoute{}
	seen := map[string]bool{}
	add := func(path string, file string, surface ContractSurface) {
		path = strings.TrimSpace(path)
		file = strings.TrimSpace(file)
		key := surfaceKey(surface, path, file)
		if path == "" || seen[key] {
			return
		}
		seen[key] = true
		routes = append(routes, ContractRoute{Path: path, File: file, Surface: surface})
	}
	for _, file := range plan.Files {
		if path := routePathFromFile(file.Path); path != "" {
			add(path, file.Path, SurfaceFrontend)
		}
	}
	if len(routes) == 0 && strings.TrimSpace(plan.TechStack.Frontend) != "" {
		add("/", "src/App.tsx", SurfaceFrontend)
	}
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Surface != routes[j].Surface {
			return routes[i].Surface < routes[j].Surface
		}
		return routes[i].Path < routes[j].Path
	})
	return routes
}

func routePathFromFile(path string) string {
	clean := strings.TrimSpace(strings.ToLower(path))
	switch {
	case strings.Contains(clean, "/pages/index"), strings.HasSuffix(clean, "/app/page.tsx"), strings.HasSuffix(clean, "/app/page.jsx"), clean == "src/app.tsx":
		return "/"
	case strings.Contains(clean, "/pages/"):
		name := strings.TrimSuffix(filepathBase(clean), filepathExt(clean))
		if name == "index" {
			return "/"
		}
		return "/" + strings.TrimPrefix(name, "_")
	case strings.Contains(clean, "/app/") && strings.Contains(clean, "/page."):
		segment := strings.TrimPrefix(clean, "app/")
		segment = strings.TrimSuffix(segment, "/page.tsx")
		segment = strings.TrimSuffix(segment, "/page.jsx")
		segment = strings.TrimSuffix(segment, "/page.ts")
		segment = strings.TrimSuffix(segment, "/page.js")
		if segment == "" {
			return "/"
		}
		return "/" + strings.Trim(segment, "/")
	default:
		return ""
	}
}

func deriveBackendResources(plan *BuildPlan) []ContractBackendResource {
	resources := []ContractBackendResource{}
	seen := map[string]bool{}
	for _, endpoint := range plan.APIEndpoints {
		name := backendResourceName(endpoint.Path)
		key := endpoint.Method + ":" + endpoint.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		resources = append(resources, ContractBackendResource{
			Name:      name,
			Kind:      "api_endpoint",
			DependsOn: []string{endpoint.Method + " " + endpoint.Path},
		})
	}
	for _, model := range plan.DataModels {
		key := "model:" + model.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		resources = append(resources, ContractBackendResource{
			Name: model.Name,
			Kind: "data_model",
		})
	}
	sort.SliceStable(resources, func(i, j int) bool {
		if resources[i].Kind != resources[j].Kind {
			return resources[i].Kind < resources[j].Kind
		}
		return resources[i].Name < resources[j].Name
	})
	return resources
}

func backendResourceName(path string) string {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	if trimmed == "" {
		return "root"
	}
	segments := strings.Split(trimmed, "/")
	for _, segment := range segments {
		if segment == "" || strings.HasPrefix(segment, ":") {
			continue
		}
		return segment
	}
	return segments[0]
}

func deriveAuthContract(plan *BuildPlan, intent *IntentBrief) *ContractAuthStrategy {
	required := capabilityRequired(intent, CapabilityAuth)
	provider := ""
	sessionStrategy := ""
	tokenStrategy := ""
	callbackStrategy := ""
	for _, env := range plan.EnvVars {
		name := strings.ToUpper(env.Name)
		switch {
		case strings.Contains(name, "CLERK"):
			provider = "clerk"
		case strings.Contains(name, "AUTH0"):
			provider = "auth0"
		case strings.Contains(name, "SUPABASE"):
			provider = "supabase"
		case strings.Contains(name, "JWT"), strings.Contains(name, "TOKEN"):
			tokenStrategy = "token"
		case strings.Contains(name, "SESSION"):
			sessionStrategy = "session"
		case strings.Contains(name, "CALLBACK"), strings.Contains(name, "REDIRECT"):
			callbackStrategy = "callback_url"
		}
	}
	for _, endpoint := range plan.APIEndpoints {
		lower := strings.ToLower(endpoint.Path)
		if strings.Contains(lower, "auth") || strings.Contains(lower, "login") || strings.Contains(lower, "signup") {
			required = true
		}
		if strings.Contains(lower, "callback") {
			callbackStrategy = "callback_url"
		}
		if strings.Contains(lower, "session") {
			sessionStrategy = "session"
		}
		if strings.Contains(lower, "token") {
			tokenStrategy = "token"
		}
	}
	if !required && provider == "" && sessionStrategy == "" && tokenStrategy == "" && callbackStrategy == "" {
		return nil
	}
	return &ContractAuthStrategy{
		Required:         required,
		Provider:         provider,
		SessionStrategy:  sessionStrategy,
		TokenStrategy:    tokenStrategy,
		CallbackStrategy: callbackStrategy,
	}
}

func capabilityRequired(intent *IntentBrief, capability CapabilityRequirement) bool {
	if intent == nil {
		return false
	}
	for _, item := range intent.RequiredCapabilities {
		if item == capability {
			return true
		}
	}
	return false
}

func deriveDependencySkeleton(plan *BuildPlan) []ContractDependency {
	deps := []ContractDependency{}
	add := func(name, surface, reason string) {
		if strings.TrimSpace(name) == "" {
			return
		}
		deps = append(deps, ContractDependency{Name: name, Surface: surface, Reason: reason})
	}
	if strings.TrimSpace(plan.TechStack.Frontend) != "" {
		add(plan.TechStack.Frontend, string(SurfaceFrontend), "declared frontend stack")
	}
	if strings.TrimSpace(plan.TechStack.Backend) != "" {
		add(plan.TechStack.Backend, string(SurfaceBackend), "declared backend stack")
	}
	if strings.TrimSpace(plan.TechStack.Database) != "" {
		add(plan.TechStack.Database, string(SurfaceData), "declared database stack")
	}
	if strings.TrimSpace(plan.TechStack.Styling) != "" {
		add(plan.TechStack.Styling, string(SurfaceFrontend), "declared styling stack")
	}
	for _, extra := range plan.TechStack.Extras {
		add(extra, string(SurfaceIntegration), "declared extra dependency")
	}
	sort.SliceStable(deps, func(i, j int) bool {
		if deps[i].Surface != deps[j].Surface {
			return deps[i].Surface < deps[j].Surface
		}
		return deps[i].Name < deps[j].Name
	})
	return dedupeDependencies(deps)
}

func deriveRuntimeContract(plan *BuildPlan) RuntimeCommandContract {
	contract := RuntimeCommandContract{}
	if strings.TrimSpace(plan.TechStack.Frontend) != "" {
		contract.FrontendInstall = "npm install"
		contract.FrontendBuild = "npm run build"
		contract.FrontendPreview = "npm run preview -- --host 0.0.0.0"
	}
	switch strings.ToLower(strings.TrimSpace(plan.TechStack.Backend)) {
	case "go", "golang":
		contract.BackendBuild = "go build ./..."
		contract.BackendStart = "go run ."
	case "python", "fastapi", "flask":
		contract.BackendInstall = "pip install -r requirements.txt"
		contract.BackendBuild = "python -m py_compile $(find . -name '*.py')"
		contract.BackendStart = "python app.py"
	case "", "none":
	default:
		contract.BackendInstall = "npm install"
		contract.BackendBuild = "npm run build"
		contract.BackendStart = "npm run start"
	}
	if strings.TrimSpace(plan.TechStack.Frontend) != "" || strings.TrimSpace(plan.TechStack.Backend) != "" {
		contract.TestCommand = "npm test"
	}
	return contract
}

func deriveRuntimeContractFromAppType(appType string) RuntimeCommandContract {
	switch appType {
	case "api":
		return RuntimeCommandContract{
			BackendInstall: "npm install",
			BackendBuild:   "npm run build",
			BackendStart:   "npm run start",
			TestCommand:    "npm test",
		}
	case "fullstack":
		return RuntimeCommandContract{
			FrontendInstall: "npm install",
			FrontendBuild:   "npm run build",
			FrontendPreview: "npm run preview -- --host 0.0.0.0",
			BackendInstall:  "npm install",
			BackendBuild:    "npm run build",
			BackendStart:    "npm run start",
			TestCommand:     "npm test",
		}
	default:
		return RuntimeCommandContract{
			FrontendInstall: "npm install",
			FrontendBuild:   "npm run build",
			FrontendPreview: "npm run preview -- --host 0.0.0.0",
			TestCommand:     "npm test",
		}
	}
}

func deriveAcceptanceBySurface(plan *BuildPlan) []SurfaceAcceptanceContract {
	if plan == nil {
		return nil
	}
	grouped := map[ContractSurface][]string{}
	for _, check := range plan.Acceptance {
		surface := contractSurfaceForRole(check.Owner)
		grouped[surface] = append(grouped[surface], check.Description)
	}
	out := make([]SurfaceAcceptanceContract, 0, len(grouped)+4)
	indexBySurface := map[ContractSurface]int{}
	for surface, checks := range grouped {
		indexBySurface[surface] = len(out)
		out = append(out, SurfaceAcceptanceContract{
			Surface:  surface,
			Checks:   dedupeStrings(checks),
			Required: true,
		})
	}
	for _, fallback := range deriveAcceptanceBySurfaceFromAppType(plan.AppType) {
		if idx, exists := indexBySurface[fallback.Surface]; exists {
			out[idx].Checks = dedupeStrings(append(out[idx].Checks, fallback.Checks...))
			out[idx].Required = out[idx].Required || fallback.Required
			continue
		}
		indexBySurface[fallback.Surface] = len(out)
		out = append(out, fallback)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Surface < out[j].Surface })
	return out
}

func deriveAcceptanceBySurfaceFromAppType(appType string) []SurfaceAcceptanceContract {
	out := []SurfaceAcceptanceContract{}
	switch appType {
	case "fullstack":
		out = append(out,
			SurfaceAcceptanceContract{Surface: SurfaceFrontend, Checks: []string{"frontend renders and route shells load"}, Required: true},
			SurfaceAcceptanceContract{Surface: SurfaceBackend, Checks: []string{"backend endpoints compile and mount"}, Required: true},
			SurfaceAcceptanceContract{Surface: SurfaceIntegration, Checks: []string{"frontend/backend API contract aligns"}, Required: true},
		)
	case "api":
		out = append(out,
			SurfaceAcceptanceContract{Surface: SurfaceBackend, Checks: []string{"backend endpoints compile and mount"}, Required: true},
			SurfaceAcceptanceContract{Surface: SurfaceIntegration, Checks: []string{"runtime contract is coherent"}, Required: true},
		)
	default:
		out = append(out,
			SurfaceAcceptanceContract{Surface: SurfaceFrontend, Checks: []string{"frontend renders and route shells load"}, Required: true},
		)
	}
	out = append(out, SurfaceAcceptanceContract{Surface: SurfaceDeployment, Checks: []string{"preview/build command contract is declared"}, Required: true})
	return out
}

func deriveVerificationGates(plan *BuildPlan) []SurfaceVerificationGate {
	return deriveVerificationGatesFromAppType(plan.AppType)
}

func deriveVerificationGatesFromAppType(appType string) []SurfaceVerificationGate {
	gates := []SurfaceVerificationGate{
		{Surface: SurfaceDeployment, Gates: []string{"build_command_contract", "preview_runtime_contract"}},
	}
	switch appType {
	case "fullstack":
		gates = append(gates,
			SurfaceVerificationGate{Surface: SurfaceFrontend, Gates: []string{"syntax_or_type_pass", "export_presence", "route_reference_integrity", "api_client_symbol_integrity"}},
			SurfaceVerificationGate{Surface: SurfaceBackend, Gates: []string{"compile_pass", "route_registration_presence", "handler_symbol_integrity", "middleware_presence"}},
			SurfaceVerificationGate{Surface: SurfaceIntegration, Gates: []string{"frontend_api_path_matches_backend_route", "auth_flow_coherence", "base_url_alignment"}},
		)
	case "api":
		gates = append(gates,
			SurfaceVerificationGate{Surface: SurfaceBackend, Gates: []string{"compile_pass", "route_registration_presence", "handler_symbol_integrity"}},
			SurfaceVerificationGate{Surface: SurfaceIntegration, Gates: []string{"runtime_contract_alignment"}},
		)
	default:
		gates = append(gates,
			SurfaceVerificationGate{Surface: SurfaceFrontend, Gates: []string{"syntax_or_type_pass", "export_presence", "route_reference_integrity"}},
		)
	}
	return gates
}

func deriveInitialTruthBySurface(plan *BuildPlan, intent *IntentBrief) map[string][]TruthTag {
	truth := map[string][]TruthTag{}
	add := func(surface ContractSurface, tags ...TruthTag) {
		key := string(surface)
		truth[key] = normalizeTruthTags(append(truth[key], tags...))
	}
	if strings.TrimSpace(plan.TechStack.Frontend) != "" {
		add(SurfaceFrontend, TruthScaffolded)
	}
	if strings.TrimSpace(plan.TechStack.Backend) != "" {
		add(SurfaceBackend, TruthScaffolded, TruthNeedsBackendRuntime)
	}
	if strings.TrimSpace(plan.TechStack.Database) != "" || capabilityRequired(intent, CapabilityDatabase) {
		add(SurfaceData, TruthScaffolded)
	}
	if strings.TrimSpace(plan.TechStack.Frontend) != "" && strings.TrimSpace(plan.TechStack.Backend) != "" {
		add(SurfaceIntegration, TruthPartiallyWired)
	}
	add(SurfaceDeployment, TruthScaffolded)
	for _, env := range plan.EnvVars {
		if env.Required {
			add(SurfaceDeployment, TruthNeedsSecrets)
			if strings.Contains(strings.ToUpper(env.Name), "OPENAI") || strings.Contains(strings.ToUpper(env.Name), "API_KEY") {
				add(SurfaceIntegration, TruthNeedsExternalAPI)
			}
		}
	}
	return truth
}

func contractSurfaceForRole(role AgentRole) ContractSurface {
	switch role {
	case RoleFrontend:
		return SurfaceFrontend
	case RoleBackend, RoleArchitect, RoleReviewer, RoleSolver:
		return SurfaceBackend
	case RoleDatabase:
		return SurfaceData
	case RoleTesting:
		return SurfaceIntegration
	case RoleDevOps:
		return SurfaceDeployment
	default:
		return SurfaceGlobal
	}
}

func taskShapeForRole(role AgentRole) TaskShape {
	switch role {
	case RolePlanner, RoleArchitect:
		return TaskShapeContract
	case RoleFrontend:
		return TaskShapeFrontendPatch
	case RoleBackend:
		return TaskShapeBackendPatch
	case RoleDatabase:
		return TaskShapeSchema
	case RoleTesting, RoleReviewer:
		return TaskShapeVerification
	case RoleSolver:
		return TaskShapeRepair
	case RoleDevOps:
		return TaskShapeIntegration
	default:
		return TaskShapeContract
	}
}

func workOrderCategoryForRole(role AgentRole) WorkOrderCategory {
	switch role {
	case RoleFrontend:
		return WorkOrderFrontend
	case RoleBackend:
		return WorkOrderBackend
	case RoleDatabase:
		return WorkOrderData
	case RoleTesting, RoleReviewer:
		return WorkOrderVerification
	case RoleSolver:
		return WorkOrderRepair
	case RoleDevOps:
		return WorkOrderConfig
	default:
		return WorkOrderIntegration
	}
}

func readableFilesForWorkOrder(order BuildWorkOrder) []string {
	base := append([]string(nil), order.RequiredFiles...)
	base = append(base, "package.json", "tsconfig.json", ".env.example")
	return dedupeStrings(base)
}

func buildWorkOrderContractSlice(surface ContractSurface, order BuildWorkOrder, contract *BuildContract, plan *BuildPlan) WorkOrderContractSlice {
	slice := WorkOrderContractSlice{
		Surface:     surface,
		OwnedChecks: append([]string(nil), order.AcceptanceChecks...),
	}
	if contract != nil {
		if tags, ok := contract.TruthBySurface[string(surface)]; ok {
			slice.TruthTags = append([]TruthTag(nil), tags...)
		}
		if contract.APIContract != nil {
			for _, endpoint := range contract.APIContract.Endpoints {
				slice.RelevantRoutes = append(slice.RelevantRoutes, fmt.Sprintf("%s %s", endpoint.Method, endpoint.Path))
			}
		}
		for _, env := range contract.EnvVarContract {
			slice.RelevantEnvVars = append(slice.RelevantEnvVars, env.Name)
		}
	}
	if plan != nil {
		for _, model := range plan.DataModels {
			slice.RelevantModels = append(slice.RelevantModels, model.Name)
		}
	}
	slice.RelevantRoutes = dedupeStrings(slice.RelevantRoutes)
	slice.RelevantEnvVars = dedupeStrings(slice.RelevantEnvVars)
	slice.RelevantModels = dedupeStrings(slice.RelevantModels)
	return slice
}

func deriveRequiredSymbols(order BuildWorkOrder, contract *BuildContract) []string {
	symbols := append([]string(nil), order.RequiredOutputs...)
	if contract != nil && contract.APIContract != nil {
		for _, endpoint := range contract.APIContract.Endpoints {
			if endpoint.Output != "" {
				symbols = append(symbols, endpoint.Output)
			}
		}
	}
	return dedupeStrings(symbols)
}

func contextBudgetForRole(role AgentRole) int {
	switch role {
	case RolePlanner, RoleArchitect, RoleReviewer:
		return 12000
	case RoleTesting:
		return 10000
	case RoleSolver:
		return 9000
	default:
		return 8000
	}
}

func riskLevelForWorkOrder(role AgentRole, contract *BuildContract) TaskRiskLevel {
	switch role {
	case RoleArchitect, RolePlanner, RoleDevOps:
		return RiskHigh
	case RoleTesting, RoleReviewer:
		return RiskMedium
	}
	if contract != nil && contract.AuthContract != nil && contract.AuthContract.Required {
		return RiskHigh
	}
	return RiskMedium
}

func routingModeForRole(role AgentRole, contract *BuildContract) ProviderRoutingMode {
	switch role {
	case RoleTesting, RoleReviewer:
		return RoutingModeSingleWithVerifier
	case RoleSolver:
		return RoutingModeDiagnosisRepair
	}
	if contract != nil && contract.AuthContract != nil && contract.AuthContract.Required {
		return RoutingModeSingleWithVerifier
	}
	return RoutingModeSingleProvider
}

func appendVerificationReport(build *Build, report VerificationReport) {
	if build == nil {
		return
	}
	build.mu.Lock()
	defer build.mu.Unlock()
	state := ensureBuildOrchestrationStateLocked(build)
	if state == nil {
		return
	}
	applyVerificationReportTruth(state, report)
	state.VerificationReports = append(state.VerificationReports, report)
	if len(state.VerificationReports) > 32 {
		state.VerificationReports = append([]VerificationReport(nil), state.VerificationReports[len(state.VerificationReports)-32:]...)
	}
}

func appendPatchBundle(build *Build, bundle PatchBundle) {
	if build == nil {
		return
	}
	build.mu.Lock()
	defer build.mu.Unlock()
	state := ensureBuildOrchestrationStateLocked(build)
	if state == nil {
		return
	}
	applyPatchBundleTruth(state, bundle)
	state.PatchBundles = append(state.PatchBundles, bundle)
	if len(state.PatchBundles) > 32 {
		state.PatchBundles = append([]PatchBundle(nil), state.PatchBundles[len(state.PatchBundles)-32:]...)
	}
}

func setPromotionDecision(build *Build, decision *PromotionDecision) {
	if build == nil || decision == nil {
		return
	}
	build.mu.Lock()
	defer build.mu.Unlock()
	state := ensureBuildOrchestrationStateLocked(build)
	if state == nil {
		return
	}
	state.PromotionDecision = decision
}

func appendFailureFingerprint(build *Build, fingerprint FailureFingerprint) {
	if build == nil {
		return
	}
	build.mu.Lock()
	defer build.mu.Unlock()
	state := ensureBuildOrchestrationStateLocked(build)
	if state == nil {
		return
	}
	state.FailureFingerprints = append(state.FailureFingerprints, fingerprint)
	if len(state.FailureFingerprints) > 32 {
		state.FailureFingerprints = append([]FailureFingerprint(nil), state.FailureFingerprints[len(state.FailureFingerprints)-32:]...)
	}
}

type providerTaskOutcome struct {
	Provider             ai.AIProvider
	TaskShape            TaskShape
	Success              bool
	FirstPass            bool
	VerificationObserved bool
	VerificationPassed   bool
	RepairAttempted      bool
	PromotionObserved    bool
	PromotionSucceeded   bool
	Truncated            bool
	TotalTokens          int
	Cost                 float64
	LatencySeconds       float64
	FailureClass         string
}

func recordProviderTaskOutcome(build *Build, outcome providerTaskOutcome) {
	if build == nil || outcome.Provider == "" || outcome.TaskShape == "" {
		return
	}

	build.mu.Lock()
	defer build.mu.Unlock()

	state := ensureBuildOrchestrationStateLocked(build)
	if state == nil || !state.Flags.EnableProviderScorecards {
		return
	}

	scorecard := ensureProviderScorecardLocked(state, build.ProviderMode, outcome.Provider, outcome.TaskShape)
	seedProviderScorecardCounts(scorecard)

	scorecard.SampleCount++
	if outcome.Success {
		scorecard.SuccessCount++
	}
	if outcome.FirstPass && outcome.VerificationObserved {
		scorecard.FirstPassSampleCount++
		if outcome.VerificationPassed {
			scorecard.FirstPassSuccessCount++
		}
	}
	if outcome.RepairAttempted {
		scorecard.RepairAttemptCount++
		if outcome.Success {
			scorecard.RepairSuccessCount++
		}
	}
	if outcome.Truncated {
		scorecard.TruncationEventCount++
	}
	if strings.TrimSpace(outcome.FailureClass) != "" {
		scorecard.FailureEventCount++
	}
	if outcome.PromotionObserved {
		scorecard.PromotionAttemptCount++
		if outcome.PromotionSucceeded {
			scorecard.PromotionSuccessCount++
		}
	}
	if outcome.Success && outcome.TotalTokens > 0 {
		scorecard.AverageAcceptedTokens = nextAveragedMetric(scorecard.AverageAcceptedTokens, scorecard.TokenSampleCount, float64(outcome.TotalTokens))
		scorecard.TokenSampleCount++
	}
	if outcome.Success && outcome.Cost > 0 {
		scorecard.AverageCostPerSuccess = nextAveragedMetric(scorecard.AverageCostPerSuccess, scorecard.CostSampleCount, outcome.Cost)
		scorecard.CostSampleCount++
	}
	if outcome.Success && outcome.LatencySeconds > 0 {
		scorecard.AverageLatencySeconds = nextAveragedMetric(scorecard.AverageLatencySeconds, scorecard.LatencySampleCount, outcome.LatencySeconds)
		scorecard.LatencySampleCount++
	}

	scorecard.CompilePassRate = safeRatio(scorecard.SuccessCount, scorecard.SampleCount, scorecard.CompilePassRate)
	scorecard.FirstPassVerificationRate = safeRatio(scorecard.FirstPassSuccessCount, scorecard.FirstPassSampleCount, scorecard.FirstPassVerificationRate)
	scorecard.RepairSuccessRate = safeRatio(scorecard.RepairSuccessCount, scorecard.RepairAttemptCount, scorecard.RepairSuccessRate)
	scorecard.TruncationRate = safeRatio(scorecard.TruncationEventCount, scorecard.SampleCount, scorecard.TruncationRate)
	scorecard.FailureClassRecurrence = safeRatio(scorecard.FailureEventCount, scorecard.SampleCount, scorecard.FailureClassRecurrence)
	scorecard.PromotionRate = safeRatio(scorecard.PromotionSuccessCount, scorecard.PromotionAttemptCount, scorecard.PromotionRate)
}

func ensureProviderScorecardLocked(state *BuildOrchestrationState, providerMode string, provider ai.AIProvider, shape TaskShape) *ProviderScorecard {
	for i := range state.ProviderScorecards {
		if state.ProviderScorecards[i].Provider == provider && state.ProviderScorecards[i].TaskShape == shape {
			return &state.ProviderScorecards[i]
		}
	}

	scorecard := ProviderScorecard{
		Provider:       provider,
		TaskShape:      shape,
		HostedEligible: !hostedProviderMode(providerMode) || provider != ai.ProviderOllama,
	}
	state.ProviderScorecards = append(state.ProviderScorecards, scorecard)
	return &state.ProviderScorecards[len(state.ProviderScorecards)-1]
}

func seedProviderScorecardCounts(scorecard *ProviderScorecard) {
	if scorecard == nil || scorecard.SampleCount > 0 || scorecard.SuccessCount > 0 || scorecard.FirstPassSampleCount > 0 ||
		scorecard.RepairAttemptCount > 0 || scorecard.PromotionAttemptCount > 0 || scorecard.TokenSampleCount > 0 ||
		scorecard.CostSampleCount > 0 || scorecard.LatencySampleCount > 0 {
		return
	}

	if scorecard.CompilePassRate <= 0 && scorecard.FirstPassVerificationRate <= 0 && scorecard.RepairSuccessRate <= 0 &&
		scorecard.TruncationRate <= 0 && scorecard.PromotionRate <= 0 && scorecard.FailureClassRecurrence <= 0 {
		return
	}

	scorecard.SampleCount = 12
	scorecard.SuccessCount = weightedCount(scorecard.CompilePassRate, scorecard.SampleCount)
	scorecard.FirstPassSampleCount = 10
	scorecard.FirstPassSuccessCount = weightedCount(scorecard.FirstPassVerificationRate, scorecard.FirstPassSampleCount)
	scorecard.RepairAttemptCount = 8
	scorecard.RepairSuccessCount = weightedCount(scorecard.RepairSuccessRate, scorecard.RepairAttemptCount)
	scorecard.TruncationEventCount = weightedCount(scorecard.TruncationRate, scorecard.SampleCount)
	scorecard.FailureEventCount = weightedCount(scorecard.FailureClassRecurrence, scorecard.SampleCount)
	scorecard.PromotionAttemptCount = 10
	scorecard.PromotionSuccessCount = weightedCount(scorecard.PromotionRate, scorecard.PromotionAttemptCount)
	if scorecard.AverageAcceptedTokens > 0 {
		scorecard.TokenSampleCount = 6
	}
	if scorecard.AverageCostPerSuccess > 0 {
		scorecard.CostSampleCount = 6
	}
	if scorecard.AverageLatencySeconds > 0 {
		scorecard.LatencySampleCount = 6
	}
}

func weightedCount(rate float64, total int) int {
	if total <= 0 || rate <= 0 {
		return 0
	}
	if rate >= 1 {
		return total
	}
	count := int(math.Round(rate * float64(total)))
	if count < 0 {
		return 0
	}
	if count > total {
		return total
	}
	return count
}

func nextAveragedMetric(current float64, sampleCount int, sample float64) float64 {
	if sampleCount <= 0 {
		return sample
	}
	return ((current * float64(sampleCount)) + sample) / float64(sampleCount+1)
}

func safeRatio(successes int, total int, fallback float64) float64 {
	if total <= 0 {
		return fallback
	}
	return float64(successes) / float64(total)
}

func buildPatchBundleFromFileDiff(buildID string, justification string, before, after []GeneratedFile) *PatchBundle {
	beforeByPath := generatedFileMap(before)
	afterByPath := generatedFileMap(after)
	paths := make([]string, 0, len(beforeByPath)+len(afterByPath))
	seen := map[string]bool{}
	for path := range beforeByPath {
		paths = append(paths, path)
		seen[path] = true
	}
	for path := range afterByPath {
		if !seen[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)

	ops := make([]PatchOperation, 0, len(paths))
	wholeFileRewrite := false
	for _, path := range paths {
		beforeFile, hadBefore := beforeByPath[path]
		afterFile, hasAfter := afterByPath[path]
		switch {
		case !hadBefore && hasAfter:
			ops = append(ops, PatchOperation{
				Type:    PatchCreateFile,
				Path:    path,
				Content: afterFile.Content,
			})
		case hadBefore && !hasAfter:
			ops = append(ops, PatchOperation{
				Type:   PatchDeleteBlock,
				Path:   path,
				Anchor: path,
			})
		case hadBefore && hasAfter && strings.TrimSpace(beforeFile.Content) != strings.TrimSpace(afterFile.Content):
			op := inferPatchOperation(path, beforeFile.Content, afterFile.Content)
			ops = append(ops, op)
			if op.Type == PatchReplaceFunction || op.Type == PatchReplaceSymbol {
				wholeFileRewrite = true
			}
		}
	}

	if len(ops) == 0 {
		return nil
	}
	return &PatchBundle{
		ID:               uuid.New().String(),
		BuildID:          buildID,
		WholeFileRewrite: wholeFileRewrite,
		Justification:    strings.TrimSpace(justification),
		Operations:       ops,
		CreatedAt:        time.Now().UTC(),
	}
}

func generatedFileMap(files []GeneratedFile) map[string]GeneratedFile {
	out := make(map[string]GeneratedFile, len(files))
	for _, file := range files {
		path := sanitizeFilePath(file.Path)
		if path == "" {
			continue
		}
		file.Path = path
		out[path] = file
	}
	return out
}

func inferPatchOperation(path, before, after string) PatchOperation {
	op := PatchOperation{
		Type:    PatchReplaceFunction,
		Path:    path,
		Content: after,
	}
	lower := strings.ToLower(path)
	switch {
	case filepathBase(lower) == "package.json":
		op.Type = inferManifestPatchOperation(before, after)
	case strings.HasSuffix(lower, ".env") || strings.HasSuffix(lower, ".env.example"):
		op.Type = PatchPatchEnvVar
	case strings.Contains(lower, "route") || strings.Contains(lower, "router"):
		op.Type = PatchPatchRouteRegistration
	case strings.Contains(lower, "schema") || strings.Contains(lower, "prisma") || strings.Contains(lower, "migration"):
		op.Type = PatchPatchSchemaEntity
	case strings.HasSuffix(lower, ".md"):
		op.Type = PatchReplaceSymbol
	case contentShrank(before, after):
		op.Type = PatchDeleteBlock
	}
	return op
}

func inferManifestPatchOperation(before, after string) PatchOperationType {
	beforeLower := strings.ToLower(before)
	afterLower := strings.ToLower(after)
	switch {
	case strings.Contains(afterLower, `"dependencies"`) || strings.Contains(afterLower, `"devdependencies"`):
		if beforeLower != afterLower {
			return PatchPatchDependency
		}
	default:
	}
	return PatchPatchJSONKey
}

func contentShrank(before, after string) bool {
	return len(strings.TrimSpace(after)) < len(strings.TrimSpace(before))
}

func buildContractBuildID(contract *BuildContract) string {
	if contract == nil {
		return ""
	}
	return contract.BuildID
}

func hasSurface(acceptance []SurfaceAcceptanceContract, surface ContractSurface) bool {
	for _, item := range acceptance {
		if item.Surface == surface {
			return true
		}
	}
	return false
}

func contractHasEnvVar(envVars []BuildEnvVar, parts ...string) bool {
	for _, env := range envVars {
		upper := strings.ToUpper(env.Name)
		for _, part := range parts {
			if strings.Contains(upper, part) {
				return true
			}
		}
	}
	return false
}

func requiresIntegrationSurface(contract BuildContract) bool {
	return hasSurface(contract.AcceptanceBySurface, SurfaceIntegration) ||
		(len(contract.RoutePageMap) > 0 && contract.APIContract != nil && len(contract.APIContract.Endpoints) > 0)
}

func missingAcceptanceSurfaces(acceptance []SurfaceAcceptanceContract, contract BuildContract) bool {
	if len(acceptance) == 0 {
		return true
	}
	frontendPreviewOnly := strings.EqualFold(strings.TrimSpace(contract.DeliveryMode), "frontend_preview_only")
	required := []ContractSurface{SurfaceDeployment}
	if len(contract.RoutePageMap) > 0 {
		required = append(required, SurfaceFrontend)
	}
	if !frontendPreviewOnly && contract.APIContract != nil && len(contract.APIContract.Endpoints) > 0 {
		required = append(required, SurfaceBackend)
	}
	if !frontendPreviewOnly && len(contract.DBSchemaContract) > 0 {
		required = append(required, SurfaceData)
	}
	if !frontendPreviewOnly && requiresIntegrationSurface(contract) {
		required = append(required, SurfaceIntegration)
	}
	for _, surface := range required {
		if !hasSurface(acceptance, surface) {
			return true
		}
	}
	return false
}

func runtimeCommandMismatch(runtime RuntimeCommandContract, contract BuildContract) bool {
	frontendPreviewOnly := strings.EqualFold(strings.TrimSpace(contract.DeliveryMode), "frontend_preview_only")
	if len(contract.RoutePageMap) > 0 && (runtime.FrontendBuild == "" || runtime.FrontendPreview == "") {
		return true
	}
	if !frontendPreviewOnly && contract.APIContract != nil && len(contract.APIContract.Endpoints) > 0 && runtime.BackendStart == "" {
		return true
	}
	return false
}

func isEmptyRuntimeContract(runtime RuntimeCommandContract) bool {
	return runtime.FrontendInstall == "" &&
		runtime.FrontendBuild == "" &&
		runtime.FrontendPreview == "" &&
		runtime.BackendInstall == "" &&
		runtime.BackendBuild == "" &&
		runtime.BackendStart == "" &&
		runtime.TestCommand == ""
}

func noOutstandingTruthConstraints(truth map[string][]TruthTag) bool {
	for _, tags := range truth {
		for _, tag := range tags {
			switch tag {
			case TruthBlocked, TruthScaffolded, TruthMocked, TruthNeedsSecrets, TruthNeedsExternalAPI, TruthNeedsBackendRuntime:
				return false
			}
		}
	}
	return true
}

func normalizeTruthTags(tags []TruthTag) []TruthTag {
	if len(tags) == 0 {
		return nil
	}
	seen := map[TruthTag]bool{}
	out := make([]TruthTag, 0, len(tags))
	for _, tag := range tags {
		if seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func appendTruthTag(tags []TruthTag, tag TruthTag) []TruthTag {
	return normalizeTruthTags(append(tags, tag))
}

func removeTruthTags(tags []TruthTag, remove ...TruthTag) []TruthTag {
	if len(tags) == 0 {
		return nil
	}
	removeSet := make(map[TruthTag]bool, len(remove))
	for _, tag := range remove {
		removeSet[tag] = true
	}
	out := make([]TruthTag, 0, len(tags))
	for _, tag := range tags {
		if removeSet[tag] {
			continue
		}
		out = append(out, tag)
	}
	return normalizeTruthTags(out)
}

func applyVerificationReportTruth(state *BuildOrchestrationState, report VerificationReport) {
	if state == nil || state.BuildContract == nil {
		return
	}
	key := string(report.Surface)
	if key == "" {
		return
	}
	tags := append([]TruthTag(nil), state.BuildContract.TruthBySurface[key]...)
	switch report.Status {
	case VerificationPassed:
		tags = removeTruthTags(tags, TruthScaffolded, TruthPartiallyWired, TruthBlocked)
		if report.Surface == SurfaceIntegration {
			tags = append(tags, TruthLiveLogicConnected)
		}
		tags = append(tags, TruthVerified)
	case VerificationFailed, VerificationBlocked:
		tags = append(tags, TruthBlocked)
	}
	state.BuildContract.TruthBySurface[key] = normalizeTruthTags(tags)
}

func inferContractSurfaceFromPath(path string) ContractSurface {
	path = strings.ToLower(sanitizeFilePath(path))
	if path == "" {
		return SurfaceGlobal
	}

	switch {
	case strings.HasPrefix(path, ".env"), path == "render.yaml", path == "docker-compose.yml",
		path == "dockerfile", strings.HasPrefix(path, "dockerfile"), strings.HasSuffix(path, ".dockerfile"),
		path == "vercel.json", path == "netlify.toml":
		return SurfaceDeployment
	case strings.HasPrefix(path, "migrations/"), strings.HasPrefix(path, "prisma/"),
		strings.HasPrefix(path, "alembic/"), path == "schema.sql",
		strings.Contains(path, "/schema."), strings.Contains(path, "migration"):
		return SurfaceData
	case strings.HasPrefix(path, "app/api/"), strings.HasPrefix(path, "server/"),
		strings.HasPrefix(path, "backend/"), strings.HasPrefix(path, "routers/"),
		strings.HasPrefix(path, "handlers/"), strings.HasPrefix(path, "middleware/"),
		strings.HasPrefix(path, "cmd/"), strings.HasPrefix(path, "internal/"),
		strings.HasPrefix(path, "pkg/"), path == "main.go", path == "main.py",
		strings.HasSuffix(path, ".go"), strings.HasSuffix(path, ".py"):
		return SurfaceBackend
	case strings.HasPrefix(path, "src/"), strings.HasPrefix(path, "public/"),
		strings.HasPrefix(path, "components/"), strings.HasPrefix(path, "styles/"),
		strings.HasPrefix(path, "app/"), path == "index.html",
		strings.HasPrefix(path, "vite.config"), strings.HasPrefix(path, "tailwind.config"),
		strings.HasPrefix(path, "postcss.config"), strings.HasPrefix(path, "next.config"),
		strings.HasSuffix(path, ".tsx"), strings.HasSuffix(path, ".jsx"),
		strings.HasSuffix(path, ".css"), strings.HasSuffix(path, ".scss"):
		return SurfaceFrontend
	case path == "package.json", path == "tsconfig.json", path == "go.mod",
		path == "requirements.txt", path == "pyproject.toml", path == "readme.md":
		return SurfaceIntegration
	default:
		return SurfaceGlobal
	}
}

func applyScaffoldBootstrapTruth(state *BuildOrchestrationState, files []GeneratedFile) {
	if state == nil || state.BuildContract == nil || len(files) == 0 {
		return
	}

	surfaces := map[ContractSurface]bool{}
	for _, file := range files {
		surface := inferContractSurfaceFromPath(file.Path)
		if surface == SurfaceGlobal {
			continue
		}
		surfaces[surface] = true
	}
	if surfaces[SurfaceFrontend] && surfaces[SurfaceBackend] {
		surfaces[SurfaceIntegration] = true
	}
	surfaces[SurfaceDeployment] = true

	for surface := range surfaces {
		key := string(surface)
		tags := append([]TruthTag(nil), state.BuildContract.TruthBySurface[key]...)
		tags = removeTruthTags(tags, TruthBlocked)
		if surface == SurfaceIntegration {
			tags = append(tags, TruthPartiallyWired)
		} else {
			tags = append(tags, TruthScaffolded)
		}
		state.BuildContract.TruthBySurface[key] = normalizeTruthTags(tags)
	}
}

func applyPatchBundleTruth(state *BuildOrchestrationState, bundle PatchBundle) {
	if state == nil || state.BuildContract == nil {
		return
	}

	surfaces := map[ContractSurface]bool{}
	if bundle.WorkOrderID != "" {
		for _, workOrder := range state.WorkOrders {
			if workOrder.ID == bundle.WorkOrderID && workOrder.ContractSlice.Surface != "" {
				surfaces[workOrder.ContractSlice.Surface] = true
				break
			}
		}
	}
	for _, op := range bundle.Operations {
		surface := inferContractSurfaceFromPath(op.Path)
		if surface == SurfaceGlobal {
			continue
		}
		surfaces[surface] = true
	}
	if surfaces[SurfaceFrontend] && surfaces[SurfaceBackend] {
		surfaces[SurfaceIntegration] = true
	}

	for surface := range surfaces {
		key := string(surface)
		tags := append([]TruthTag(nil), state.BuildContract.TruthBySurface[key]...)
		tags = removeTruthTags(tags, TruthBlocked, TruthScaffolded)
		switch surface {
		case SurfaceFrontend, SurfaceBackend, SurfaceData, SurfaceIntegration:
			tags = append(tags, TruthPartiallyWired)
		case SurfaceDeployment:
			tags = append(tags, TruthScaffolded)
		}
		state.BuildContract.TruthBySurface[key] = normalizeTruthTags(tags)
	}
}

func dedupeCapabilities(values []CapabilityRequirement) []CapabilityRequirement {
	if len(values) == 0 {
		return nil
	}
	seen := map[CapabilityRequirement]bool{}
	out := make([]CapabilityRequirement, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func dedupeDependencies(values []ContractDependency) []ContractDependency {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]ContractDependency, 0, len(values))
	for _, value := range values {
		key := value.Surface + ":" + value.Name
		if value.Name == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func surfaceKey(surface ContractSurface, path string, file string) string {
	return string(surface) + ":" + path + ":" + file
}

func stackCombinationFromBuild(build *Build) string {
	if build == nil || build.Plan == nil {
		return ""
	}
	parts := []string{
		strings.TrimSpace(build.Plan.TechStack.Frontend),
		strings.TrimSpace(build.Plan.TechStack.Backend),
		strings.TrimSpace(build.Plan.TechStack.Database),
	}
	return strings.Join(parts, "|")
}

func normalizeFailureClass(message string) string {
	lower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(lower, "contract"):
		return "contract_violation"
	case strings.Contains(lower, "coordination"):
		return "coordination_violation"
	case strings.Contains(lower, "verification"):
		return "verification_failure"
	case strings.Contains(lower, "truncat") || strings.Contains(lower, "unterminated code block") || strings.Contains(lower, "abrupt eof"):
		return "truncation"
	case strings.Contains(lower, "validation"):
		return "final_validation_failure"
	case strings.Contains(lower, "timeout"), strings.Contains(lower, "deadline exceeded"), strings.Contains(lower, "context canceled"):
		return "timeout"
	case strings.Contains(lower, "credit"):
		return "budget"
	case strings.Contains(lower, "preview"):
		return "preview_verification"
	default:
		return "build_failure"
	}
}

func fingerprintFiles(files []GeneratedFile) []string {
	if len(files) == 0 {
		return nil
	}
	out := make([]string, 0, len(files))
	for i := 0; i < len(files) && i < 8; i++ {
		path := strings.TrimSpace(files[i].Path)
		if path != "" {
			out = append(out, path)
		}
	}
	return out
}

func repairPathForBuild(build *Build) []string {
	if build == nil {
		return nil
	}
	build.mu.RLock()
	defer build.mu.RUnlock()
	path := []string{"final_readiness"}
	if build.ReadinessRecoveryAttempts > 0 {
		path = append(path, "deterministic_repair", "solver_recovery")
	}
	return path
}

func filepathBase(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func filepathExt(path string) string {
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		return path[idx:]
	}
	return ""
}
