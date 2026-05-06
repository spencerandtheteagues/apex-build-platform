package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateMapBuildsRepoDerivedNodes(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "backend/internal/agents/manager.go", "package agents\n")
	mustWrite(t, root, "backend/internal/agents/manager_test.go", "package agents\n")
	mustWrite(t, root, "frontend/src/components/builder/AppBuilder.tsx", "export function AppBuilder() { return null }\n")
	mustWrite(t, root, "backend/migrations/000001_initial_schema.up.sql", "CREATE TABLE completed_builds(id text);\n")

	m, err := GenerateMap(root, nil)
	if err != nil {
		t.Fatalf("GenerateMap returned error: %v", err)
	}
	if m.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %q, want %q", m.SchemaVersion, SchemaVersion)
	}
	if m.Summary.NodeCount == 0 || m.Summary.ContractCount == 0 || m.Summary.EdgeCount == 0 {
		t.Fatalf("expected populated map summary, got %+v", m.Summary)
	}
	agentsNode := findNode(t, m, "ai.orchestration")
	if agentsNode.FileCount != 2 {
		t.Fatalf("agents file count = %d, want 2", agentsNode.FileCount)
	}
	if agentsNode.TestCount != 1 {
		t.Fatalf("agents test count = %d, want 1", agentsNode.TestCount)
	}
}

func TestCollectReferenceEventCountsKnownPockets(t *testing.T) {
	event := CollectReferenceEvent(ReferenceInput{
		BuildID:   "build-1",
		TaskID:    "task-1",
		TaskType:  "fix",
		AgentRole: "solver",
		Provider:  "gpt4",
		Texts: []string{
			"Inspect backend/internal/agents/manager.go, BuildSnapshotState, completed_builds, and contract.preview.runtime.",
		},
	})
	if len(event.Hits) == 0 {
		t.Fatal("expected reference hits")
	}
	telemetry := MergeReferenceTelemetry(nil, event)
	if telemetry.ByNode["ai.orchestration"] == 0 {
		t.Fatalf("expected ai.orchestration reference in %+v", telemetry.ByNode)
	}
	if telemetry.ByStructure["BuildSnapshotState"] == 0 {
		t.Fatalf("expected BuildSnapshotState reference in %+v", telemetry.ByStructure)
	}
	if telemetry.ByDatabase["db.completed_build_snapshots"] == 0 {
		t.Fatalf("expected completed build snapshot db reference in %+v", telemetry.ByDatabase)
	}
	if telemetry.ByContract["contract.preview.runtime"] == 0 {
		t.Fatalf("expected preview contract reference in %+v", telemetry.ByContract)
	}
	if telemetry.ByAgentRole["solver"] != 1 {
		t.Fatalf("expected solver role count, got %+v", telemetry.ByAgentRole)
	}
}

func mustWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func findNode(t *testing.T, m *Map, id string) Node {
	t.Helper()
	for _, node := range m.Nodes {
		if node.ID == id {
			return node
		}
	}
	t.Fatalf("node %s not found", id)
	return Node{}
}
