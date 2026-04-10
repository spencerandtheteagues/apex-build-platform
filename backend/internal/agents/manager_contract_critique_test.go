package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	"apex-build/internal/ai"
)

type contractCritiqueRouter struct {
	stubAIRouter
	content string
	err     error
	calls   int
}

func (r *contractCritiqueRouter) Generate(_ context.Context, _ ai.AIProvider, _ string, _ GenerateOptions) (*ai.AIResponse, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	return &ai.AIResponse{Content: r.content}, nil
}

type blockingContractCritiqueRouter struct {
	stubAIRouter
	calls int
}

func (r *blockingContractCritiqueRouter) Generate(ctx context.Context, _ ai.AIProvider, _ string, _ GenerateOptions) (*ai.AIResponse, error) {
	r.calls++
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestProviderAssistedContractCritiqueReturnsVerificationReport(t *testing.T) {
	am := &AgentManager{
		aiRouter: &contractCritiqueRouter{
			stubAIRouter: stubAIRouter{
				providers:             []ai.AIProvider{ai.ProviderClaude},
				hasConfiguredProvider: true,
			},
			content: `{"summary":"contract missing explicit auth strategy","warnings":["auth callback url should be explicit"],"blockers":["auth capability requested without callback/session/token strategy"],"confidence":0.83}`,
		},
		ctx: context.Background(),
	}

	build := &Build{
		ID:           "build-contract-critique",
		UserID:       7,
		ProviderMode: "platform",
	}
	contract := &BuildContract{
		ID:      "contract-critique",
		BuildID: build.ID,
		AppType: "fullstack",
	}

	report := am.providerAssistedContractCritique(build, contract)
	if report == nil {
		t.Fatal("expected provider critique report")
	}
	if report.Phase != "contract_provider_critique" || report.Status != VerificationBlocked {
		t.Fatalf("unexpected critique report: %+v", report)
	}
	if report.Provider != ai.ProviderClaude {
		t.Fatalf("expected critique provider claude, got %+v", report)
	}
	if len(report.Blockers) != 1 || !strings.Contains(report.Blockers[0], "auth capability requested") {
		t.Fatalf("expected parsed blocker, got %+v", report)
	}
}

func TestProviderAssistedContractCritiqueTimesOutAndReturnsNil(t *testing.T) {
	router := &blockingContractCritiqueRouter{
		stubAIRouter: stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderClaude},
			hasConfiguredProvider: true,
		},
	}
	am := &AgentManager{
		aiRouter: router,
		ctx:      context.Background(),
	}

	build := &Build{
		ID:           "build-contract-critique-timeout",
		UserID:       8,
		ProviderMode: "platform",
	}
	contract := &BuildContract{
		ID:      "contract-critique-timeout",
		BuildID: build.ID,
		AppType: "fullstack",
	}

	start := time.Now()
	report := am.providerAssistedContractCritique(build, contract)
	if report != nil {
		t.Fatalf("expected timeout critique to return nil, got %+v", report)
	}
	if router.calls != 1 {
		t.Fatalf("expected one critique call, got %d", router.calls)
	}
	if elapsed := time.Since(start); elapsed > 25*time.Second {
		t.Fatalf("expected critique timeout to stop within 25s, took %s", elapsed)
	}
}

func TestHandlePlanCompletionBlocksOnProviderAssistedContractCritique(t *testing.T) {
	am := &AgentManager{
		aiRouter: &contractCritiqueRouter{
			stubAIRouter: stubAIRouter{
				providers:             []ai.AIProvider{ai.ProviderClaude},
				hasConfiguredProvider: true,
			},
			content: `{"summary":"contract critique found blocker","warnings":[],"blockers":["provider critique: explicit callback/session strategy required for auth"],"confidence":0.8}`,
		},
		agents:      map[string]*Agent{},
		builds:      map[string]*Build{},
		taskQueue:   make(chan *Task, 1),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	build := &Build{
		ID:           "build-critique-block",
		UserID:       99,
		Status:       BuildPlanning,
		Description:  "Build a fullstack app dashboard",
		ProviderMode: "platform",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: func() BuildOrchestrationFlags {
					flags := defaultBuildOrchestrationFlags()
					flags.EnableContractVerification = false
					flags.EnableSelectiveEscalation = true
					return flags
				}(),
				IntentBrief: &IntentBrief{
					ID:                "intent-critique",
					NormalizedRequest: "Build a fullstack app dashboard",
					AppType:           "fullstack",
				},
			},
		},
	}

	plan := createBuildPlanFromPlanningBundle(build.ID, build.Description, &TechStack{
		Frontend: "React",
		Backend:  "Express",
	}, nil)
	if plan == nil {
		t.Fatal("expected build plan")
	}

	am.handlePlanCompletion(build, &TaskOutput{Plan: plan})

	if build.Status != BuildFailed {
		t.Fatalf("expected critique blocker to fail build, got status=%s error=%q", build.Status, build.Error)
	}
	if !strings.Contains(build.Error, "provider critique") {
		t.Fatalf("expected critique blocker in build error, got %q", build.Error)
	}
	state := build.SnapshotState.Orchestration
	if state == nil || len(state.VerificationReports) == 0 {
		t.Fatalf("expected verification reports, got %+v", state)
	}
	foundCritique := false
	for _, report := range state.VerificationReports {
		if report.Phase == "contract_provider_critique" {
			foundCritique = true
			if report.Status != VerificationBlocked {
				t.Fatalf("expected critique report to block, got %+v", report)
			}
		}
	}
	if !foundCritique {
		t.Fatalf("expected provider critique report, got %+v", state.VerificationReports)
	}
}

func TestHandlePlanCompletionSyncsSeededAPIContractBackIntoPlan(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderGPT4},
			hasConfiguredProvider: true,
		},
		agents:      map[string]*Agent{},
		builds:      map[string]*Build{},
		taskQueue:   make(chan *Task, 16),
		resultQueue: make(chan *TaskResult, 16),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	build := &Build{
		ID:           "build-plan-sync",
		UserID:       101,
		Status:       BuildPlanning,
		Mode:         ModeFull,
		Description:  "Build a fullstack CRM where users can create an account, log in, and manage clients from a dashboard.",
		ProviderMode: "platform",
		Agents:       map[string]*Agent{},
		Tasks:        []*Task{},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: func() BuildOrchestrationFlags {
					flags := defaultBuildOrchestrationFlags()
					flags.EnableSelectiveEscalation = false
					return flags
				}(),
				IntentBrief: &IntentBrief{
					ID:                "intent-plan-sync",
					NormalizedRequest: "Build a fullstack CRM where users can create an account, log in, and manage clients from a dashboard.",
					AppType:           "fullstack",
					RequiredCapabilities: []CapabilityRequirement{
						CapabilityAPI,
						CapabilityAuth,
					},
				},
			},
		},
	}
	am.builds[build.ID] = build

	plan := createBuildPlanFromPlanningBundle(build.ID, build.Description, &TechStack{
		Frontend: "React",
		Backend:  "Express",
		Database: "SQLite",
	}, nil)
	if plan == nil {
		t.Fatal("expected build plan")
	}
	plan.APIContract = &BuildAPIContract{
		BackendPort: 3001,
		Endpoints: []APIEndpoint{
			{Method: "GET", Path: "/api/health", Description: "health"},
		},
	}
	plan.APIEndpoints = apiEndpointsFromContract(plan.APIContract)
	plan.EnvVars = append(plan.EnvVars, BuildEnvVar{Name: "JWT_SECRET", Required: true})

	am.handlePlanCompletion(build, &TaskOutput{Plan: plan})

	build.mu.RLock()
	defer build.mu.RUnlock()
	if build.Status == BuildFailed {
		t.Fatalf("expected plan completion to proceed, got failed build error=%q", build.Error)
	}
	if build.Plan == nil || build.Plan.APIContract == nil {
		t.Fatalf("expected seeded API contract on build plan, got %+v", build.Plan)
	}

	endpoints := make(map[string]bool)
	for _, endpoint := range build.Plan.APIContract.Endpoints {
		endpoints[strings.ToUpper(strings.TrimSpace(endpoint.Method))+" "+strings.TrimSpace(endpoint.Path)] = true
	}
	for _, key := range []string{
		"GET /api/health",
		"POST /api/auth/login",
		"GET /api/auth/me",
		"POST /api/auth/register",
	} {
		if !endpoints[key] {
			t.Fatalf("expected seeded API endpoint %q on build plan, got %+v", key, build.Plan.APIContract.Endpoints)
		}
	}

	if len(build.Plan.APIEndpoints) != len(build.Plan.APIContract.Endpoints) {
		t.Fatalf("expected APIEndpoints to sync from APIContract, got endpoints=%d contract=%d", len(build.Plan.APIEndpoints), len(build.Plan.APIContract.Endpoints))
	}
}

func TestShouldRunProviderAssistedContractCritiqueSkipsFastLowRiskContracts(t *testing.T) {
	build := &Build{PowerMode: PowerFast}
	contract := &BuildContract{
		ID:      "contract-fast-low-risk",
		BuildID: "build-fast-low-risk",
		AppType: "frontend",
	}

	if shouldRunProviderAssistedContractCritique(build, contract) {
		t.Fatalf("expected fast low-risk contract critique to be skipped")
	}
}

func TestShouldRunProviderAssistedContractCritiqueKeepsFastAuthContracts(t *testing.T) {
	build := &Build{PowerMode: PowerFast}
	contract := &BuildContract{
		ID:      "contract-fast-auth",
		BuildID: "build-fast-auth",
		AppType: "fullstack",
		AuthContract: &ContractAuthStrategy{
			Required: true,
		},
	}

	if !shouldRunProviderAssistedContractCritique(build, contract) {
		t.Fatalf("expected fast auth contract critique to remain enabled")
	}
}

func TestEffectiveTaskRoutingModeDowngradesFastMediumRiskVerifier(t *testing.T) {
	build := &Build{PowerMode: PowerFast}
	task := &Task{
		ID: "task-fast-medium-verifier",
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:          "wo-fast-medium",
				RoutingMode: RoutingModeSingleWithVerifier,
				RiskLevel:   RiskMedium,
			},
		},
	}

	if got := effectiveTaskRoutingMode(build, task); got != RoutingModeSingleProvider {
		t.Fatalf("expected fast medium-risk verifier route to downgrade, got %s", got)
	}
}

func TestEffectiveTaskRoutingModeKeepsFastHighRiskVerifier(t *testing.T) {
	build := &Build{PowerMode: PowerFast}
	task := &Task{
		ID: "task-fast-high-verifier",
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:          "wo-fast-high",
				RoutingMode: RoutingModeSingleWithVerifier,
				RiskLevel:   RiskHigh,
			},
		},
	}

	if got := effectiveTaskRoutingMode(build, task); got != RoutingModeSingleWithVerifier {
		t.Fatalf("expected fast high-risk verifier route to remain, got %s", got)
	}
}

func TestShouldRunFailureConsensusDefersFirstFastRetryForMediumRisk(t *testing.T) {
	am := &AgentManager{}
	build := &Build{PowerMode: PowerFast}
	task := &Task{RetryCount: 1, Input: map[string]any{"risk_level": string(RiskMedium)}}

	if am.shouldRunFailureConsensus(build, task, "verification failed", "fix_and_retry") {
		t.Fatalf("expected first fast retry consensus to be skipped for medium-risk task")
	}
}

func TestShouldRunFailureConsensusSkipsFirstFastCriticalCodeFailures(t *testing.T) {
	am := &AgentManager{}
	build := &Build{PowerMode: PowerFast}
	task := &Task{
		RetryCount: 1,
		Input: map[string]any{
			"work_order_artifact": WorkOrder{RiskLevel: RiskCritical},
		},
	}

	if am.shouldRunFailureConsensus(build, task, "verification failed", "fix_and_retry") {
		t.Fatalf("expected direct code-fix retries to skip consensus even for critical tasks")
	}
}

func TestShouldRunFailureConsensusKeepsFastCriticalProviderEscalations(t *testing.T) {
	am := &AgentManager{}
	build := &Build{PowerMode: PowerFast}
	task := &Task{
		RetryCount: 1,
		Input: map[string]any{
			"work_order_artifact": WorkOrder{RiskLevel: RiskCritical},
		},
	}

	if !am.shouldRunFailureConsensus(build, task, "verification failed", "switch_provider") {
		t.Fatalf("expected fast critical provider escalation to retain consensus")
	}
}

type taskRoutingRouter struct {
	stubAIRouter
	judgeContent         string
	verifyContent        string
	generationByProvider map[ai.AIProvider]string
	lastProvider         ai.AIProvider
	lastPrompt           string
}

func (r *taskRoutingRouter) Generate(_ context.Context, provider ai.AIProvider, prompt string, _ GenerateOptions) (*ai.AIResponse, error) {
	r.lastProvider = provider
	r.lastPrompt = prompt
	switch {
	case strings.Contains(prompt, "Choose the better build candidate"):
		return &ai.AIResponse{Content: r.judgeContent}, nil
	case strings.Contains(prompt, "Review this AI-generated task result"):
		return &ai.AIResponse{Content: r.verifyContent}, nil
	case r.generationByProvider != nil:
		if content, ok := r.generationByProvider[provider]; ok {
			return &ai.AIResponse{Content: content}, nil
		}
		return &ai.AIResponse{Content: "// File: src/App.tsx\n```typescript\nexport default function App() { return <div>fallback</div>; }\n```", Usage: &ai.Usage{}}, nil
	default:
		return &ai.AIResponse{Content: `{"summary":"ok","warnings":[],"blockers":[],"confidence":0.9}`}, nil
	}
}

func TestJudgeTaskCandidatesReturnsWinnerAndJudgeProvider(t *testing.T) {
	am := &AgentManager{
		aiRouter: &taskRoutingRouter{
			stubAIRouter: stubAIRouter{
				providers:             []ai.AIProvider{ai.ProviderGPT4, ai.ProviderGrok},
				hasConfiguredProvider: true,
			},
			judgeContent: `{"winner_index":1,"rationale":"candidate 1 has fewer verification issues"}`,
		},
		ctx: context.Background(),
	}

	build := &Build{
		ID:           "build-dual-candidate-judge",
		UserID:       21,
		ProviderMode: "platform",
	}
	task := &Task{
		ID:          "task-dual-candidate-judge",
		Type:        TaskGenerateUI,
		Description: "Generate frontend shell",
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:          "wo-dual",
				Role:        RoleFrontend,
				TaskShape:   TaskShapeFrontendPatch,
				RoutingMode: RoutingModeDualCandidate,
			},
		},
	}
	candidates := []*taskGenerationCandidate{
		{
			Provider:           ai.ProviderGPT4,
			Model:              "gpt-a",
			DeterministicScore: 95,
			VerifyPassed:       true,
			Output:             &TaskOutput{Files: []GeneratedFile{{Path: "src/App.tsx", Content: "export default function App(){return <div>a</div>}", Language: "typescript"}}},
		},
		{
			Provider:           ai.ProviderGrok,
			Model:              "grok-b",
			DeterministicScore: 94,
			VerifyPassed:       true,
			Output:             &TaskOutput{Files: []GeneratedFile{{Path: "src/App.tsx", Content: "export default function App(){return <main>b</main>}", Language: "typescript"}}},
		},
	}

	winner, judgeProvider, rationale := am.judgeTaskCandidates(build, task, candidates)
	if winner != 1 {
		t.Fatalf("expected judge winner index 1, got %d", winner)
	}
	if judgeProvider != ai.ProviderGrok {
		t.Fatalf("expected adversarial critique provider grok, got %s", judgeProvider)
	}
	if !strings.Contains(rationale, "fewer verification issues") {
		t.Fatalf("expected rationale to be preserved, got %q", rationale)
	}
}

func TestProviderAssistedTaskVerificationReturnsSurfaceScopedReport(t *testing.T) {
	router := &taskRoutingRouter{
		stubAIRouter: stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderGPT4, ai.ProviderGrok},
			hasConfiguredProvider: true,
		},
		verifyContent: `{"summary":"candidate has one risky integration assumption","warnings":["api base url should remain relative in Next.js"],"blockers":["route handler returns an invalid payload shape"],"confidence":0.78}`,
	}
	am := &AgentManager{
		aiRouter: router,
		ctx:      context.Background(),
	}

	build := &Build{
		ID:           "build-task-verifier",
		UserID:       55,
		ProviderMode: "platform",
	}
	task := &Task{
		ID:          "task-task-verifier",
		Type:        TaskGenerateAPI,
		Description: "Create backend route",
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:                "wo-backend-verifier",
				Role:              RoleBackend,
				TaskShape:         TaskShapeBackendPatch,
				RoutingMode:       RoutingModeSingleWithVerifier,
				PreferredProvider: ai.ProviderGPT4,
				ContractSlice: WorkOrderContractSlice{
					Surface:   SurfaceBackend,
					TruthTags: []TruthTag{TruthScaffolded},
				},
			},
		},
	}
	candidate := &taskGenerationCandidate{
		Provider:     ai.ProviderGPT4,
		Model:        "gpt-5",
		VerifyPassed: true,
		Output: &TaskOutput{
			Files: []GeneratedFile{
				{Path: "app/api/health/route.ts", Content: "export function GET(){ return Response.json({ ok: true }) }", Language: "typescript"},
			},
		},
	}

	report := am.providerAssistedTaskVerification(build, task, candidate)
	if report == nil {
		t.Fatal("expected task verification report")
	}
	if report.Phase != "task_provider_verification" || report.Status != VerificationBlocked {
		t.Fatalf("unexpected task provider verification report: %+v", report)
	}
	if report.Surface != SurfaceBackend || report.WorkOrderID != "wo-backend-verifier" {
		t.Fatalf("expected backend-scoped report, got %+v", report)
	}
	if report.Provider != ai.ProviderGrok {
		t.Fatalf("expected verifier to choose alternate critique provider grok, got %+v", report)
	}
	if len(report.Warnings) != 1 || len(report.Blockers) != 1 {
		t.Fatalf("expected parsed warnings and blockers, got %+v", report)
	}
	if !containsTruthTag(report.TruthTags, TruthScaffolded) {
		t.Fatalf("expected truth tags from work order contract slice, got %+v", report.TruthTags)
	}
}

func TestExecuteTaskDualCandidateUsesJudgeSelectedOutput(t *testing.T) {
	router := &taskRoutingRouter{
		stubAIRouter: stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderGPT4, ai.ProviderClaude, ai.ProviderGrok},
			hasConfiguredProvider: true,
		},
		judgeContent: `{"winner_index":0,"rationale":"candidate 0 is cleaner and more complete"}`,
		generationByProvider: map[ai.AIProvider]string{
			ai.ProviderGPT4:   "// File: src/App.tsx\n```typescript\nexport default function App() { return <div>first candidate</div>; }\n```",
			ai.ProviderClaude: "// File: src/App.tsx\n```typescript\nexport default function App() { return <main>judge selected</main>; }\n```",
		},
	}
	am := &AgentManager{
		aiRouter:    router,
		agents:      map[string]*Agent{},
		builds:      map[string]*Build{},
		taskQueue:   make(chan *Task, 1),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	build := &Build{
		ID:           "build-execute-dual-candidate",
		UserID:       88,
		Status:       BuildInProgress,
		Description:  "Build a frontend shell",
		ProviderMode: "platform",
		Agents:       map[string]*Agent{},
	}
	task := &Task{
		ID:          "task-execute-dual-candidate",
		Type:        TaskGenerateUI,
		Description: "Generate frontend shell",
		AssignedTo:  "agent-execute-dual-candidate",
		MaxRetries:  2,
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:          "wo-execute-dual",
				Role:        RoleFrontend,
				TaskShape:   TaskShapeFrontendPatch,
				RoutingMode: RoutingModeDualCandidate,
			},
		},
	}
	agent := &Agent{
		ID:          task.AssignedTo,
		Role:        RoleFrontend,
		Provider:    ai.ProviderGPT4,
		Model:       selectModelForPowerMode(ai.ProviderGPT4, PowerBalanced),
		BuildID:     build.ID,
		Status:      StatusWorking,
		CurrentTask: task,
	}
	build.Agents[agent.ID] = agent
	am.agents[agent.ID] = agent
	am.builds[build.ID] = build

	am.executeTask(task)

	select {
	case result := <-am.resultQueue:
		if !result.Success {
			t.Fatalf("expected successful dual candidate execution, got %+v", result)
		}
		if result.Output == nil || len(result.Output.Files) != 1 {
			t.Fatalf("expected one selected file output, got %+v", result.Output)
		}
		if !strings.Contains(result.Output.Files[0].Content, "judge selected") {
			t.Fatalf("expected judge-selected candidate content, got %+v", result.Output.Files)
		}
		if got := taskOutputMetricInt(result.Output, "candidate_count"); got != 2 {
			t.Fatalf("expected 2 candidates, got %+v", result.Output.Metrics)
		}
		if got := taskOutputMetricString(result.Output, "selected_provider"); got != string(ai.ProviderClaude) {
			t.Fatalf("expected selected provider claude, got %+v", result.Output.Metrics)
		}
		if got := taskOutputMetricString(result.Output, "candidate_judge_provider"); got != string(ai.ProviderGrok) {
			t.Fatalf("expected judge provider grok, got %+v", result.Output.Metrics)
		}
		if !strings.Contains(taskOutputMetricString(result.Output, "candidate_judge_rationale"), "cleaner and more complete") {
			t.Fatalf("expected judge rationale in metrics, got %+v", result.Output.Metrics)
		}
	default:
		t.Fatal("expected task result from executeTask")
	}

	if agent.Provider != ai.ProviderClaude {
		t.Fatalf("expected executing agent to align to selected provider, got %+v", agent)
	}
}

func TestExecuteTaskSingleWithVerifierBlocksCandidate(t *testing.T) {
	router := &taskRoutingRouter{
		stubAIRouter: stubAIRouter{
			providers:             []ai.AIProvider{ai.ProviderGPT4, ai.ProviderGrok},
			hasConfiguredProvider: true,
		},
		verifyContent: `{"summary":"task verifier found a concrete bug","warnings":[],"blockers":["response payload shape does not match the contract"],"confidence":0.81}`,
		generationByProvider: map[ai.AIProvider]string{
			ai.ProviderGPT4: "// File: app/api/health/route.ts\n```typescript\nexport function GET() { return Response.json({ ok: true }); }\n```",
		},
	}
	am := &AgentManager{
		aiRouter:    router,
		agents:      map[string]*Agent{},
		builds:      map[string]*Build{},
		taskQueue:   make(chan *Task, 1),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	build := &Build{
		ID:           "build-execute-single-verifier",
		UserID:       89,
		Status:       BuildInProgress,
		Description:  "Generate a backend route",
		ProviderMode: "platform",
		Agents:       map[string]*Agent{},
	}
	task := &Task{
		ID:          "task-execute-single-verifier",
		Type:        TaskGenerateAPI,
		Description: "Generate backend route",
		AssignedTo:  "agent-execute-single-verifier",
		MaxRetries:  2,
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:          "wo-execute-verifier",
				Role:        RoleBackend,
				TaskShape:   TaskShapeBackendPatch,
				RoutingMode: RoutingModeSingleWithVerifier,
				ContractSlice: WorkOrderContractSlice{
					Surface: SurfaceBackend,
				},
			},
		},
	}
	agent := &Agent{
		ID:          task.AssignedTo,
		Role:        RoleBackend,
		Provider:    ai.ProviderGPT4,
		Model:       selectModelForPowerMode(ai.ProviderGPT4, PowerBalanced),
		BuildID:     build.ID,
		Status:      StatusWorking,
		CurrentTask: task,
	}
	build.Agents[agent.ID] = agent
	am.agents[agent.ID] = agent
	am.builds[build.ID] = build

	am.executeTask(task)

	select {
	case result := <-am.resultQueue:
		if result.Success {
			t.Fatalf("expected provider verifier to block execution, got %+v", result)
		}
		if result.Error == nil || !strings.Contains(result.Error.Error(), "provider verification blocked task output") {
			t.Fatalf("expected verifier blocker error, got %+v", result)
		}
	default:
		t.Fatal("expected task result from executeTask")
	}
}
