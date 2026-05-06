package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func GenerateMap(root string, telemetry *ReferenceTelemetry) (*Map, error) {
	if strings.TrimSpace(root) == "" {
		root = ResolveRepoRoot(".")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	nodes := make([]Node, 0, len(defaultPocketRules()))
	totalFiles := 0
	totalTests := 0
	highRiskNodes := 0
	criticalNodes := 0

	for _, rule := range defaultPocketRules() {
		fileCount, testCount := countFiles(absRoot, rule.Directory)
		totalFiles += fileCount
		totalTests += testCount
		if rule.RiskLevel == "high" {
			highRiskNodes++
		}
		if rule.RiskLevel == "critical" {
			criticalNodes++
		}
		node := Node{
			ID:            rule.ID,
			Name:          rule.Name,
			Layer:         rule.Layer,
			Type:          rule.Type,
			Description:   rule.Description,
			RiskLevel:     rule.RiskLevel,
			RiskScore:     rule.RiskScore,
			CodeLocations: []CodeLocation{{Path: rule.Directory}},
			FileCount:     fileCount,
			TestCount:     testCount,
			Tags:          append([]string(nil), rule.Tags...),
		}
		if telemetry != nil && telemetry.ByNode != nil {
			node.References = telemetry.ByNode[rule.ID]
		}
		nodes = append(nodes, node)
	}

	contracts := make([]Contract, 0, len(defaultContractRules()))
	for _, rule := range defaultContractRules() {
		contract := Contract{
			ID:             rule.ID,
			ContractType:   rule.ContractType,
			Producer:       rule.Producer,
			Consumers:      append([]string(nil), rule.Consumers...),
			SchemaLocation: rule.SchemaLocation,
			TestLocations:  append([]string(nil), rule.TestLocations...),
		}
		if telemetry != nil && telemetry.ByContract != nil {
			contract.References = telemetry.ByContract[rule.ID]
		}
		contracts = append(contracts, contract)
	}

	edges := defaultEdges()
	summary := Summary{
		NodeCount:     len(nodes),
		EdgeCount:     len(edges),
		ContractCount: len(contracts),
		FileCount:     totalFiles,
		TestFileCount: totalTests,
		HighRiskNodes: highRiskNodes,
		CriticalNodes: criticalNodes,
	}
	if telemetry != nil {
		summary.ReferenceCount = telemetry.TotalReferences
	}

	return &Map{
		SchemaVersion:       SchemaVersion,
		GeneratedAt:         time.Now().UTC(),
		RepoRoot:            absRoot,
		Source:              "deterministic_repo_scanner",
		Confidence:          0.74,
		Summary:             summary,
		Nodes:               nodes,
		Edges:               edges,
		Contracts:           contracts,
		DiagnosticPlaybooks: defaultDiagnosticPlaybooks(),
		QualityGates:        defaultQualityGates(),
		ReferenceTelemetry:  CloneReferenceTelemetry(telemetry),
	}, nil
}

func ResolveRepoRoot(start string) string {
	if strings.TrimSpace(start) == "" {
		start = "."
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	original := abs
	info, err := os.Stat(abs)
	if err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
		original = abs
	}
	for {
		if looksLikeRepoRoot(abs) {
			return abs
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}
	return original
}

func looksLikeRepoRoot(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		return false
	}
	if info, err := os.Stat(filepath.Join(dir, "backend")); err != nil || !info.IsDir() {
		return false
	}
	if info, err := os.Stat(filepath.Join(dir, "frontend")); err != nil || !info.IsDir() {
		return false
	}
	return true
}

func countFiles(root, rel string) (int, int) {
	dir := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return 0, 0
	}
	files := 0
	tests := 0
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			switch name {
			case ".git", "node_modules", "dist", "build", "coverage", ".next", ".vite":
				return filepath.SkipDir
			}
			return nil
		}
		files++
		lower := strings.ToLower(filepath.ToSlash(path))
		if strings.Contains(lower, "_test.go") ||
			strings.Contains(lower, ".test.") ||
			strings.Contains(lower, ".spec.") ||
			strings.Contains(lower, "/tests/") {
			tests++
		}
		return nil
	})
	return files, tests
}

func defaultEdges() []Edge {
	return []Edge{
		{ID: "edge.web_builder.api_client", FromNode: "web.builder", ToNode: "web.api_client", EdgeType: "runtime_call", Criticality: "high", Description: "Build UI calls typed frontend API client."},
		{ID: "edge.web_preview.preview_runtime", FromNode: "web.preview", ToNode: "runtime.preview", EdgeType: "runtime_call", Criticality: "critical", Description: "Preview UI depends on backend preview lifecycle APIs."},
		{ID: "edge.api_core.handlers", FromNode: "api.core", ToNode: "api.handlers", EdgeType: "route_dispatch", Criticality: "high", Description: "Core router dispatches feature endpoints."},
		{ID: "edge.api_handlers.agents", FromNode: "api.handlers", ToNode: "ai.orchestration", EdgeType: "runtime_call", Criticality: "critical", Description: "Build handlers invoke orchestration manager."},
		{ID: "edge.agents.providers", FromNode: "ai.orchestration", ToNode: "ai.providers", EdgeType: "provider_call", Criticality: "critical", Description: "Agents route model calls through provider adapter/router."},
		{ID: "edge.agents.persistence", FromNode: "ai.orchestration", ToNode: "data.persistence", EdgeType: "data_write", Criticality: "critical", Description: "Build lifecycle persists snapshots and generated artifacts."},
		{ID: "edge.preview.persistence", FromNode: "runtime.preview", ToNode: "data.persistence", EdgeType: "data_read", Criticality: "high", Description: "Preview starts from stored project/generated files."},
		{ID: "edge.billing.providers", FromNode: "billing.spend", ToNode: "ai.providers", EdgeType: "cost_flow", Criticality: "critical", Description: "Provider usage and routing feed spend attribution."},
		{ID: "edge.auth.api", FromNode: "security.auth", ToNode: "api.core", EdgeType: "auth_check", Criticality: "critical", Description: "Protected routes rely on auth/session middleware."},
		{ID: "edge.deploy.runtime", FromNode: "deployment.runtime", ToNode: "data.persistence", EdgeType: "artifact_read", Criticality: "high", Description: "Deployments read project/build artifacts and logs."},
	}
}

func defaultDiagnosticPlaybooks() []DiagnosticPlaybook {
	return []DiagnosticPlaybook{
		{
			ID: "playbook.preview_unstable", Symptom: "Preview fails, flashes white, or restarts",
			EntryNodes:     []string{"web.preview", "runtime.preview", "ai.orchestration"},
			Checks:         []string{"Check preview status contract", "Inspect console/page errors", "Verify generated app entrypoint", "Check router basename/proxy path", "Inspect preview repair attempts"},
			SafeRepairPlan: []string{"Reproduce with one build ID", "Patch smallest preview contract/runtime seam", "Add focused preview regression", "Rerun live canary only after local proof"},
		},
		{
			ID: "playbook.build_stalls", Symptom: "Build stalls before terminal completion",
			EntryNodes:     []string{"ai.orchestration", "ai.providers", "data.persistence"},
			Checks:         []string{"Inspect phase/status/progress invariant", "Check provider timeout and fallback path", "Check snapshot write/read consistency", "Check repair loop attempt bounds"},
			SafeRepairPlan: []string{"Build transition trace", "Patch terminal condition or retry boundary", "Add orchestration regression", "Validate no extra provider loop"},
		},
		{
			ID: "playbook.cost_spike", Symptom: "Unexpected model or repair spend",
			EntryNodes:     []string{"ai.providers", "billing.spend", "ai.orchestration"},
			Checks:         []string{"Compare requested/actual provider", "Inspect retry counts", "Check context size/reference hotspots", "Check BYOK/platform attribution"},
			SafeRepairPlan: []string{"Fix routing or idempotency first", "Add spend attribution regression", "Keep expensive live tests manual"},
		},
	}
}

func defaultQualityGates() []QualityGate {
	return []QualityGate{
		{ID: "gate.contract_first", Description: "High-risk cross-surface changes identify producers, consumers, and tests before editing.", Required: true, Checks: []string{"contract map", "targeted producer test", "targeted consumer test or typecheck"}},
		{ID: "gate.preview_reliability", Description: "Preview-related changes prove runtime stability before live canary spend.", Required: true, Checks: []string{"preview unit/integration test", "no console/page errors", "stable preview sample"}},
		{ID: "gate.reference_privacy", Description: "Reference telemetry stores metadata counts only, not prompt text or generated content.", Required: true, Checks: []string{"no prompt text persisted", "counts aggregated by known node/contract/database"}},
	}
}
