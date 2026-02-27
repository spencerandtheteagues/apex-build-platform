package agents

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// helper: creates a minimal ProposedEdit with optional pre-set fields.
func makeEdit(filePath string) *ProposedEdit {
	return &ProposedEdit{
		FilePath:        filePath,
		AgentID:         "agent-1",
		AgentRole:       "coder",
		TaskID:          "task-1",
		OriginalContent: "old",
		ProposedContent: "new",
		Language:        "go",
	}
}

// ---------------------------------------------------------------------------
// AddProposedEdits
// ---------------------------------------------------------------------------

func TestAddProposedEdits_AssignsUUIDs(t *testing.T) {
	store := NewProposedEditStore()
	edits := []*ProposedEdit{makeEdit("a.go"), makeEdit("b.go")}
	store.AddProposedEdits("build-1", edits)

	for i, e := range edits {
		if e.ID == "" {
			t.Fatalf("edit[%d]: expected UUID to be assigned, got empty string", i)
		}
	}
	// UUIDs must be distinct
	if edits[0].ID == edits[1].ID {
		t.Fatalf("expected distinct UUIDs, both are %s", edits[0].ID)
	}
}

func TestAddProposedEdits_PreservesExistingID(t *testing.T) {
	store := NewProposedEditStore()
	edit := makeEdit("c.go")
	edit.ID = "my-custom-id"
	store.AddProposedEdits("build-1", []*ProposedEdit{edit})

	if edit.ID != "my-custom-id" {
		t.Fatalf("expected ID to stay my-custom-id, got %s", edit.ID)
	}
}

func TestAddProposedEdits_SetsDefaults(t *testing.T) {
	store := NewProposedEditStore()
	edit := makeEdit("d.go")
	store.AddProposedEdits("build-1", []*ProposedEdit{edit})

	if edit.BuildID != "build-1" {
		t.Fatalf("expected BuildID=build-1, got %s", edit.BuildID)
	}
	if edit.Status != EditPending {
		t.Fatalf("expected Status=pending, got %s", edit.Status)
	}
	if edit.CreatedAt.IsZero() {
		t.Fatalf("expected CreatedAt to be set")
	}
}

func TestAddProposedEdits_PreservesExistingBuildID(t *testing.T) {
	store := NewProposedEditStore()
	edit := makeEdit("e.go")
	edit.BuildID = "original-build"
	store.AddProposedEdits("build-1", []*ProposedEdit{edit})

	if edit.BuildID != "original-build" {
		t.Fatalf("expected BuildID to stay original-build, got %s", edit.BuildID)
	}
}

func TestAddProposedEdits_PreservesExistingStatus(t *testing.T) {
	store := NewProposedEditStore()
	edit := makeEdit("f.go")
	edit.Status = EditApproved
	store.AddProposedEdits("build-1", []*ProposedEdit{edit})

	if edit.Status != EditApproved {
		t.Fatalf("expected Status to stay approved, got %s", edit.Status)
	}
}

func TestAddProposedEdits_AppendsAcrossCalls(t *testing.T) {
	store := NewProposedEditStore()
	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go")})
	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("b.go")})

	all := store.GetAllEdits("build-1")
	if len(all) != 2 {
		t.Fatalf("expected 2 edits after two Add calls, got %d", len(all))
	}
}

func TestAddProposedEdits_EmptySliceIsNoOp(t *testing.T) {
	store := NewProposedEditStore()
	store.AddProposedEdits("build-1", []*ProposedEdit{})

	all := store.GetAllEdits("build-1")
	if len(all) != 0 {
		t.Fatalf("expected 0 edits, got %d", len(all))
	}
}

// ---------------------------------------------------------------------------
// GetPendingEdits
// ---------------------------------------------------------------------------

func TestGetPendingEdits_ReturnsPendingOnly(t *testing.T) {
	store := NewProposedEditStore()
	edits := []*ProposedEdit{makeEdit("a.go"), makeEdit("b.go"), makeEdit("c.go")}
	store.AddProposedEdits("build-1", edits)

	// Approve the first one
	_, err := store.ApproveEdits("build-1", []string{edits[0].ID})
	if err != nil {
		t.Fatalf("ApproveEdits: %v", err)
	}

	pending := store.GetPendingEdits("build-1")
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending edits, got %d", len(pending))
	}
	for _, p := range pending {
		if p.Status != EditPending {
			t.Fatalf("expected pending status, got %s for edit %s", p.Status, p.ID)
		}
	}
}

func TestGetPendingEdits_EmptyBuild(t *testing.T) {
	store := NewProposedEditStore()
	pending := store.GetPendingEdits("nonexistent-build")
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending edits for unknown build, got %d", len(pending))
	}
}

func TestGetPendingEdits_NoPendingAfterApproveAll(t *testing.T) {
	store := NewProposedEditStore()
	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go"), makeEdit("b.go")})
	store.ApproveAll("build-1")

	pending := store.GetPendingEdits("build-1")
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending edits after ApproveAll, got %d", len(pending))
	}
}

// ---------------------------------------------------------------------------
// GetAllEdits
// ---------------------------------------------------------------------------

func TestGetAllEdits_ReturnsAllStatuses(t *testing.T) {
	store := NewProposedEditStore()
	edits := []*ProposedEdit{makeEdit("a.go"), makeEdit("b.go"), makeEdit("c.go")}
	store.AddProposedEdits("build-1", edits)

	store.ApproveEdits("build-1", []string{edits[0].ID})
	store.RejectEdits("build-1", []string{edits[1].ID})

	all := store.GetAllEdits("build-1")
	if len(all) != 3 {
		t.Fatalf("expected 3 edits, got %d", len(all))
	}

	statuses := map[ProposedEditStatus]int{}
	for _, e := range all {
		statuses[e.Status]++
	}
	if statuses[EditApproved] != 1 || statuses[EditRejected] != 1 || statuses[EditPending] != 1 {
		t.Fatalf("unexpected status distribution: %+v", statuses)
	}
}

func TestGetAllEdits_ReturnsCopy(t *testing.T) {
	store := NewProposedEditStore()
	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go")})

	all1 := store.GetAllEdits("build-1")
	all2 := store.GetAllEdits("build-1")
	// Modifying the returned slice should not affect subsequent calls
	all1[0] = nil
	if all2[0] == nil {
		t.Fatalf("GetAllEdits should return independent slice copies")
	}
}

func TestGetAllEdits_EmptyBuild(t *testing.T) {
	store := NewProposedEditStore()
	all := store.GetAllEdits("nonexistent")
	if len(all) != 0 {
		t.Fatalf("expected 0 edits for unknown build, got %d", len(all))
	}
}

// ---------------------------------------------------------------------------
// ApproveEdits
// ---------------------------------------------------------------------------

func TestApproveEdits_SetsStatusAndReviewedAt(t *testing.T) {
	store := NewProposedEditStore()
	edits := []*ProposedEdit{makeEdit("a.go"), makeEdit("b.go")}
	store.AddProposedEdits("build-1", edits)

	approved, err := store.ApproveEdits("build-1", []string{edits[0].ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(approved) != 1 {
		t.Fatalf("expected 1 approved edit, got %d", len(approved))
	}
	if approved[0].Status != EditApproved {
		t.Fatalf("expected status approved, got %s", approved[0].Status)
	}
	if approved[0].ReviewedAt == nil {
		t.Fatalf("expected ReviewedAt to be set")
	}
}

func TestApproveEdits_ErrorOnAlreadyApproved(t *testing.T) {
	store := NewProposedEditStore()
	edit := makeEdit("a.go")
	store.AddProposedEdits("build-1", []*ProposedEdit{edit})

	store.ApproveEdits("build-1", []string{edit.ID})
	_, err := store.ApproveEdits("build-1", []string{edit.ID})
	if err == nil {
		t.Fatalf("expected error when approving already-approved edit")
	}
	if !strings.Contains(err.Error(), "not pending") {
		t.Fatalf("expected 'not pending' in error, got: %s", err.Error())
	}
}

func TestApproveEdits_ErrorOnRejected(t *testing.T) {
	store := NewProposedEditStore()
	edit := makeEdit("a.go")
	store.AddProposedEdits("build-1", []*ProposedEdit{edit})

	store.RejectEdits("build-1", []string{edit.ID})
	_, err := store.ApproveEdits("build-1", []string{edit.ID})
	if err == nil {
		t.Fatalf("expected error when approving rejected edit")
	}
	if !strings.Contains(err.Error(), "not pending") {
		t.Fatalf("expected 'not pending' in error, got: %s", err.Error())
	}
}

func TestApproveEdits_ErrorOnMissingID(t *testing.T) {
	store := NewProposedEditStore()
	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go")})

	_, err := store.ApproveEdits("build-1", []string{"nonexistent-id"})
	if err == nil {
		t.Fatalf("expected error for missing edit ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' in error, got: %s", err.Error())
	}
}

func TestApproveEdits_MultipleIDs(t *testing.T) {
	store := NewProposedEditStore()
	edits := []*ProposedEdit{makeEdit("a.go"), makeEdit("b.go"), makeEdit("c.go")}
	store.AddProposedEdits("build-1", edits)

	approved, err := store.ApproveEdits("build-1", []string{edits[0].ID, edits[2].ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(approved) != 2 {
		t.Fatalf("expected 2 approved edits, got %d", len(approved))
	}
}

// ---------------------------------------------------------------------------
// RejectEdits
// ---------------------------------------------------------------------------

func TestRejectEdits_SetsStatusAndReviewedAt(t *testing.T) {
	store := NewProposedEditStore()
	edit := makeEdit("a.go")
	store.AddProposedEdits("build-1", []*ProposedEdit{edit})

	err := store.RejectEdits("build-1", []string{edit.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edit.Status != EditRejected {
		t.Fatalf("expected status rejected, got %s", edit.Status)
	}
	if edit.ReviewedAt == nil {
		t.Fatalf("expected ReviewedAt to be set")
	}
}

func TestRejectEdits_ErrorOnAlreadyRejected(t *testing.T) {
	store := NewProposedEditStore()
	edit := makeEdit("a.go")
	store.AddProposedEdits("build-1", []*ProposedEdit{edit})

	store.RejectEdits("build-1", []string{edit.ID})
	err := store.RejectEdits("build-1", []string{edit.ID})
	if err == nil {
		t.Fatalf("expected error when rejecting already-rejected edit")
	}
	if !strings.Contains(err.Error(), "not pending") {
		t.Fatalf("expected 'not pending' in error, got: %s", err.Error())
	}
}

func TestRejectEdits_ErrorOnMissingID(t *testing.T) {
	store := NewProposedEditStore()
	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go")})

	err := store.RejectEdits("build-1", []string{"ghost-id"})
	if err == nil {
		t.Fatalf("expected error for missing edit ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' in error, got: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// ApproveAll
// ---------------------------------------------------------------------------

func TestApproveAll_ApprovesAllPending(t *testing.T) {
	store := NewProposedEditStore()
	edits := []*ProposedEdit{makeEdit("a.go"), makeEdit("b.go"), makeEdit("c.go")}
	store.AddProposedEdits("build-1", edits)

	// Reject one first so it stays rejected
	store.RejectEdits("build-1", []string{edits[2].ID})

	approved := store.ApproveAll("build-1")
	if len(approved) != 2 {
		t.Fatalf("expected 2 approved (skipping rejected), got %d", len(approved))
	}
	for _, a := range approved {
		if a.Status != EditApproved {
			t.Fatalf("expected approved status, got %s", a.Status)
		}
		if a.ReviewedAt == nil {
			t.Fatalf("expected ReviewedAt to be set on approved edit %s", a.ID)
		}
	}

	// The rejected one should still be rejected
	if edits[2].Status != EditRejected {
		t.Fatalf("expected edit[2] to remain rejected, got %s", edits[2].Status)
	}
}

func TestApproveAll_EmptyBuild(t *testing.T) {
	store := NewProposedEditStore()
	approved := store.ApproveAll("empty-build")
	if len(approved) != 0 {
		t.Fatalf("expected 0 approved for empty build, got %d", len(approved))
	}
}

func TestApproveAll_NoPendingLeft(t *testing.T) {
	store := NewProposedEditStore()
	edits := []*ProposedEdit{makeEdit("a.go")}
	store.AddProposedEdits("build-1", edits)
	store.RejectEdits("build-1", []string{edits[0].ID})

	approved := store.ApproveAll("build-1")
	if len(approved) != 0 {
		t.Fatalf("expected 0 approved when none pending, got %d", len(approved))
	}
}

// ---------------------------------------------------------------------------
// RejectAll
// ---------------------------------------------------------------------------

func TestRejectAll_RejectsAllPending(t *testing.T) {
	store := NewProposedEditStore()
	edits := []*ProposedEdit{makeEdit("a.go"), makeEdit("b.go"), makeEdit("c.go")}
	store.AddProposedEdits("build-1", edits)

	// Approve one first
	store.ApproveEdits("build-1", []string{edits[0].ID})

	err := store.RejectAll("build-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// edits[0] should still be approved
	if edits[0].Status != EditApproved {
		t.Fatalf("expected edit[0] to remain approved, got %s", edits[0].Status)
	}
	// edits[1] and edits[2] should be rejected
	for _, idx := range []int{1, 2} {
		if edits[idx].Status != EditRejected {
			t.Fatalf("expected edit[%d] to be rejected, got %s", idx, edits[idx].Status)
		}
		if edits[idx].ReviewedAt == nil {
			t.Fatalf("expected ReviewedAt set on edit[%d]", idx)
		}
	}
}

func TestRejectAll_EmptyBuild(t *testing.T) {
	store := NewProposedEditStore()
	err := store.RejectAll("empty-build")
	if err != nil {
		t.Fatalf("RejectAll on empty build should not error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Clear
// ---------------------------------------------------------------------------

func TestClear_RemovesAllEditsForBuild(t *testing.T) {
	store := NewProposedEditStore()
	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go"), makeEdit("b.go")})
	store.AddProposedEdits("build-2", []*ProposedEdit{makeEdit("c.go")})

	store.Clear("build-1")

	if len(store.GetAllEdits("build-1")) != 0 {
		t.Fatalf("expected build-1 edits to be cleared")
	}
	if len(store.GetAllEdits("build-2")) != 1 {
		t.Fatalf("expected build-2 edits to be unaffected")
	}
}

func TestClear_NoOpForUnknownBuild(t *testing.T) {
	store := NewProposedEditStore()
	store.Clear("ghost-build") // should not panic
}

// ---------------------------------------------------------------------------
// Build isolation: edits from different builds don't interfere
// ---------------------------------------------------------------------------

func TestBuildIsolation(t *testing.T) {
	store := NewProposedEditStore()
	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go")})
	store.AddProposedEdits("build-2", []*ProposedEdit{makeEdit("b.go")})

	b1 := store.GetAllEdits("build-1")
	b2 := store.GetAllEdits("build-2")

	if len(b1) != 1 || b1[0].FilePath != "a.go" {
		t.Fatalf("build-1 should have 1 edit (a.go), got %+v", b1)
	}
	if len(b2) != 1 || b2[0].FilePath != "b.go" {
		t.Fatalf("build-2 should have 1 edit (b.go), got %+v", b2)
	}
}

// ---------------------------------------------------------------------------
// Concurrency
// ---------------------------------------------------------------------------

func TestConcurrency_AddAndGetPending(t *testing.T) {
	store := NewProposedEditStore()
	const goroutines = 50
	const editsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(goroutines * 2) // half writers, half readers

	// Writers
	for g := 0; g < goroutines; g++ {
		go func(gIdx int) {
			defer wg.Done()
			for i := 0; i < editsPerGoroutine; i++ {
				edit := makeEdit(fmt.Sprintf("file-%d-%d.go", gIdx, i))
				store.AddProposedEdits("build-race", []*ProposedEdit{edit})
			}
		}(g)
	}

	// Readers
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < editsPerGoroutine; i++ {
				_ = store.GetPendingEdits("build-race")
			}
		}()
	}

	wg.Wait()

	total := store.GetAllEdits("build-race")
	expected := goroutines * editsPerGoroutine
	if len(total) != expected {
		t.Fatalf("expected %d edits, got %d", expected, len(total))
	}
}

func TestConcurrency_ApproveAndReject(t *testing.T) {
	store := NewProposedEditStore()
	edits := make([]*ProposedEdit, 100)
	for i := range edits {
		edits[i] = makeEdit(fmt.Sprintf("file-%d.go", i))
	}
	store.AddProposedEdits("build-cr", edits)

	var wg sync.WaitGroup
	// Approve first half, reject second half -- concurrently
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			store.ApproveEdits("build-cr", []string{edits[i].ID})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 50; i < 100; i++ {
			store.RejectEdits("build-cr", []string{edits[i].ID})
		}
	}()
	wg.Wait()

	pending := store.GetPendingEdits("build-cr")
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after concurrent approve/reject, got %d", len(pending))
	}
}
