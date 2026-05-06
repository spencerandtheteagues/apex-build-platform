package agents

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"apex-build/internal/ai"
	"apex-build/internal/architecture"
	"apex-build/pkg/models"

	"github.com/gin-gonic/gin"
)

func TestRecordArchitectureReferencesStoresMetadataCountsOnly(t *testing.T) {
	build := &Build{
		ID:            "build-refs",
		UserID:        7,
		SnapshotState: BuildSnapshotState{},
	}
	agent := &Agent{ID: "agent-1", Role: RoleSolver, Provider: ai.ProviderGPT4, BuildID: build.ID}
	task := &Task{ID: "task-1", Type: TaskFix}
	manager := &AgentManager{}

	manager.recordArchitectureReferences(
		build,
		agent,
		task,
		ai.ProviderGPT4,
		"gpt-test",
		"Inspect backend/internal/agents/manager.go and BuildSnapshotState.",
		"Check completed_builds and contract.build.lifecycle before patching.",
	)

	refs := build.SnapshotState.ArchitectureReferences
	if refs == nil {
		t.Fatal("expected architecture reference telemetry")
	}
	if refs.ByNode["ai.orchestration"] == 0 {
		t.Fatalf("expected orchestration node count, got %+v", refs.ByNode)
	}
	if refs.ByStructure["BuildSnapshotState"] == 0 {
		t.Fatalf("expected structure count, got %+v", refs.ByStructure)
	}
	if refs.ByDatabase["db.completed_build_snapshots"] == 0 {
		t.Fatalf("expected database surface count, got %+v", refs.ByDatabase)
	}
	if refs.ByContract["contract.build.lifecycle"] == 0 {
		t.Fatalf("expected contract count, got %+v", refs.ByContract)
	}
	if len(refs.RecentEvents) != 1 {
		t.Fatalf("expected one recent event, got %d", len(refs.RecentEvents))
	}
	for _, event := range refs.RecentEvents {
		if event.AgentRole != string(RoleSolver) || event.Provider != string(ai.ProviderGPT4) {
			t.Fatalf("unexpected event metadata: %+v", event)
		}
	}
}

func TestGetAdminArchitectureMapMergesLiveReferenceTelemetry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	refs := architecture.MergeReferenceTelemetry(nil, architecture.ReferenceEvent{
		BuildID:   "build-refs",
		TaskID:    "task-1",
		TaskType:  string(TaskFix),
		AgentRole: string(RoleSolver),
		Provider:  string(ai.ProviderGPT4),
		Model:     "gpt-test",
		Timestamp: time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC),
		Hits: []architecture.ReferenceHit{
			{NodeID: "ai.orchestration", Directory: "backend/internal/agents", Count: 3},
			{Contract: "contract.build.lifecycle", Count: 2},
			{Database: "db.completed_build_snapshots", Count: 1},
			{Structure: "BuildSnapshotState", Count: 1},
		},
	})
	manager := &AgentManager{builds: map[string]*Build{
		"build-refs": {
			ID:     "build-refs",
			UserID: 9,
			SnapshotState: BuildSnapshotState{
				ArchitectureReferences: refs,
			},
		},
	}}
	handler := &BuildHandler{manager: manager}
	router := gin.New()
	router.GET("/admin/architecture/map", handler.GetAdminArchitectureMap)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/architecture/map", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Map architecture.Map `json:"map"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Map.ReferenceTelemetry == nil {
		t.Fatal("expected reference telemetry in architecture map")
	}
	if response.Map.ReferenceTelemetry.TotalReferences != 7 {
		t.Fatalf("expected 7 total references, got %d", response.Map.ReferenceTelemetry.TotalReferences)
	}
	if response.Map.ReferenceTelemetry.ByNode["ai.orchestration"] != 3 {
		t.Fatalf("expected orchestration references in telemetry, got %+v", response.Map.ReferenceTelemetry.ByNode)
	}
	if response.Map.ReferenceTelemetry.ByContract["contract.build.lifecycle"] != 2 {
		t.Fatalf("expected lifecycle contract references, got %+v", response.Map.ReferenceTelemetry.ByContract)
	}
	if response.Map.ReferenceTelemetry.ByDatabase["db.completed_build_snapshots"] != 1 {
		t.Fatalf("expected database references, got %+v", response.Map.ReferenceTelemetry.ByDatabase)
	}
	if response.Map.ReferenceTelemetry.ByStructure["BuildSnapshotState"] != 1 {
		t.Fatalf("expected structure references, got %+v", response.Map.ReferenceTelemetry.ByStructure)
	}
	if response.Map.Summary.ReferenceCount != 7 {
		t.Fatalf("expected summary reference count to match telemetry, got %d", response.Map.Summary.ReferenceCount)
	}

	foundNodeReference := false
	for _, node := range response.Map.Nodes {
		if node.ID == "ai.orchestration" && node.References == 3 {
			foundNodeReference = true
			break
		}
	}
	if !foundNodeReference {
		t.Fatalf("expected ai.orchestration node to include reference count, got %+v", response.Map.Nodes)
	}
}

func TestGetAdminArchitectureMapMergesCompletedSnapshotReferenceTelemetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openBuildTestDB(t)

	terminalRefs := architecture.MergeReferenceTelemetry(nil, architecture.ReferenceEvent{
		BuildID:   "completed-refs",
		TaskID:    "task-2",
		TaskType:  string(TaskReview),
		AgentRole: string(RoleReviewer),
		Provider:  string(ai.ProviderGrok),
		Model:     "grok-test",
		Timestamp: time.Date(2026, 5, 5, 13, 0, 0, 0, time.UTC),
		Hits: []architecture.ReferenceHit{
			{NodeID: "runtime.preview", Directory: "backend/internal/preview", Count: 4},
			{Contract: "contract.preview.runtime", Count: 1},
		},
	})
	terminalState, err := json.Marshal(BuildSnapshotState{ArchitectureReferences: terminalRefs})
	if err != nil {
		t.Fatalf("marshal terminal state: %v", err)
	}
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "completed-refs",
		UserID:      1,
		ProjectName: "Completed telemetry",
		Description: "completed architecture telemetry",
		Status:      string(BuildCompleted),
		StateJSON:   string(terminalState),
	}).Error; err != nil {
		t.Fatalf("create completed snapshot: %v", err)
	}

	inProgressRefs := architecture.MergeReferenceTelemetry(nil, architecture.ReferenceEvent{
		BuildID:   "active-refs",
		Timestamp: time.Date(2026, 5, 5, 14, 0, 0, 0, time.UTC),
		Hits: []architecture.ReferenceHit{
			{NodeID: "billing.spend", Directory: "backend/internal/billing", Count: 99},
		},
	})
	inProgressState, err := json.Marshal(BuildSnapshotState{ArchitectureReferences: inProgressRefs})
	if err != nil {
		t.Fatalf("marshal in-progress state: %v", err)
	}
	if err := db.Create(&models.CompletedBuild{
		BuildID:     "active-refs",
		UserID:      1,
		ProjectName: "Active telemetry",
		Description: "active architecture telemetry",
		Status:      string(BuildInProgress),
		StateJSON:   string(inProgressState),
	}).Error; err != nil {
		t.Fatalf("create in-progress snapshot: %v", err)
	}

	handler := &BuildHandler{db: db}
	router := gin.New()
	router.GET("/admin/architecture/map", handler.GetAdminArchitectureMap)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/architecture/map", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Map architecture.Map `json:"map"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	refs := response.Map.ReferenceTelemetry
	if refs == nil {
		t.Fatal("expected reference telemetry from completed snapshot")
	}
	if refs.TotalReferences != 5 {
		t.Fatalf("expected terminal snapshot references only, got %d", refs.TotalReferences)
	}
	if refs.ByNode["runtime.preview"] != 4 {
		t.Fatalf("expected completed preview node references, got %+v", refs.ByNode)
	}
	if refs.ByContract["contract.preview.runtime"] != 1 {
		t.Fatalf("expected completed preview contract references, got %+v", refs.ByContract)
	}
	if refs.ByNode["billing.spend"] != 0 {
		t.Fatalf("did not expect in-progress snapshot references in admin aggregate, got %+v", refs.ByNode)
	}
	if response.Map.Summary.ReferenceCount != 5 {
		t.Fatalf("expected summary reference count from completed snapshots, got %d", response.Map.Summary.ReferenceCount)
	}
}
