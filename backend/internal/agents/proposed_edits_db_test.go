package agents

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testDBProposedEdits(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&proposedEditRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// ---------------------------------------------------------------------------
// CRUD via DB-backed store
// ---------------------------------------------------------------------------

func TestProposedEditStoreDB_AddAndGetAll(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	edits := []*ProposedEdit{
		makeEdit("a.go"),
		makeEdit("b.go"),
	}
	store.AddProposedEdits("build-1", edits)

	all := store.GetAllEdits("build-1")
	if len(all) != 2 {
		t.Fatalf("expected 2 edits, got %d", len(all))
	}

	// Verify DB rows exist
	var count int64
	db.Model(&proposedEditRow{}).Where("build_id = ?", "build-1").Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 DB rows, got %d", count)
	}
}

func TestProposedEditStoreDB_GetPending(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	edits := []*ProposedEdit{makeEdit("a.go"), makeEdit("b.go")}
	store.AddProposedEdits("build-1", edits)

	pending := store.GetPendingEdits("build-1")
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending edits, got %d", len(pending))
	}
}

func TestProposedEditStoreDB_StatusTransition_Approve(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	edit := makeEdit("a.go")
	store.AddProposedEdits("build-1", []*ProposedEdit{edit})

	approved, err := store.ApproveEdits("build-1", []string{edit.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(approved) != 1 {
		t.Fatalf("expected 1 approved, got %d", len(approved))
	}
	if approved[0].Status != EditApproved {
		t.Fatalf("expected status approved, got %s", approved[0].Status)
	}

	// DB should reflect the change
	var row proposedEditRow
	db.Where("id = ?", edit.ID).First(&row)
	if row.Status != "approved" {
		t.Fatalf("expected DB status=approved, got %s", row.Status)
	}
	if row.ReviewedAt == nil {
		t.Fatalf("expected ReviewedAt set in DB")
	}
}

func TestProposedEditStoreDB_StatusTransition_Reject(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	edit := makeEdit("a.go")
	store.AddProposedEdits("build-1", []*ProposedEdit{edit})

	err := store.RejectEdits("build-1", []string{edit.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var row proposedEditRow
	db.Where("id = ?", edit.ID).First(&row)
	if row.Status != "rejected" {
		t.Fatalf("expected DB status=rejected, got %s", row.Status)
	}
}

func TestProposedEditStoreDB_ApproveAll(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go"), makeEdit("b.go")})

	approved := store.ApproveAll("build-1")
	if len(approved) != 2 {
		t.Fatalf("expected 2 approved, got %d", len(approved))
	}

	var pendingCount int64
	db.Model(&proposedEditRow{}).Where("build_id = ? AND status = 'pending'", "build-1").Count(&pendingCount)
	if pendingCount != 0 {
		t.Fatalf("expected 0 pending in DB after ApproveAll, got %d", pendingCount)
	}
}

func TestProposedEditStoreDB_RejectAll(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go"), makeEdit("b.go")})

	err := store.RejectAll("build-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rejectedCount int64
	db.Model(&proposedEditRow{}).Where("build_id = ? AND status = 'rejected'", "build-1").Count(&rejectedCount)
	if rejectedCount != 2 {
		t.Fatalf("expected 2 rejected in DB after RejectAll, got %d", rejectedCount)
	}
}

// ---------------------------------------------------------------------------
// Filtering by build
// ---------------------------------------------------------------------------

func TestProposedEditStoreDB_BuildIsolation(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go")})
	store.AddProposedEdits("build-2", []*ProposedEdit{makeEdit("b.go")})

	b1 := store.GetAllEdits("build-1")
	b2 := store.GetAllEdits("build-2")

	if len(b1) != 1 || b1[0].FilePath != "a.go" {
		t.Fatalf("expected build-1 to have 1 edit (a.go), got %+v", b1)
	}
	if len(b2) != 1 || b2[0].FilePath != "b.go" {
		t.Fatalf("expected build-2 to have 1 edit (b.go), got %+v", b2)
	}

	// DB verification
	var count1, count2 int64
	db.Model(&proposedEditRow{}).Where("build_id = ?", "build-1").Count(&count1)
	db.Model(&proposedEditRow{}).Where("build_id = ?", "build-2").Count(&count2)
	if count1 != 1 || count2 != 1 {
		t.Fatalf("expected 1 row per build in DB, got %d and %d", count1, count2)
	}
}

func TestProposedEditStoreDB_Clear(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go"), makeEdit("b.go")})
	store.AddProposedEdits("build-2", []*ProposedEdit{makeEdit("c.go")})

	store.Clear("build-1")

	if len(store.GetAllEdits("build-1")) != 0 {
		t.Fatalf("expected build-1 to be empty after Clear")
	}
	if len(store.GetAllEdits("build-2")) != 1 {
		t.Fatalf("expected build-2 to still have 1 edit")
	}

	var dbCount int64
	db.Model(&proposedEditRow{}).Where("build_id = ?", "build-1").Count(&dbCount)
	if dbCount != 0 {
		t.Fatalf("expected 0 DB rows for build-1 after Clear, got %d", dbCount)
	}
}

// ---------------------------------------------------------------------------
// Status transitions edge cases
// ---------------------------------------------------------------------------

func TestProposedEditStoreDB_ApproveAlreadyApproved_NoError(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	edit := makeEdit("a.go")
	store.AddProposedEdits("build-1", []*ProposedEdit{edit})

	_, err := store.ApproveEdits("build-1", []string{edit.ID})
	if err != nil {
		t.Fatalf("first approve: %v", err)
	}

	// Second approve should not error because DB is authoritative
	_, err = store.ApproveEdits("build-1", []string{edit.ID})
	if err != nil {
		t.Fatalf("second approve on DB-backed store should not error, got: %v", err)
	}
}

func TestProposedEditStoreDB_RejectAlreadyRejected_NoError(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	edit := makeEdit("a.go")
	store.AddProposedEdits("build-1", []*ProposedEdit{edit})

	err := store.RejectEdits("build-1", []string{edit.ID})
	if err != nil {
		t.Fatalf("first reject: %v", err)
	}

	err = store.RejectEdits("build-1", []string{edit.ID})
	if err != nil {
		t.Fatalf("second reject on DB-backed store should not error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Pending conflict detection
// ---------------------------------------------------------------------------

func TestProposedEditStoreDB_HasPendingConflict(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go")})

	if !store.HasPendingConflict("build-1", "a.go") {
		t.Fatalf("expected HasPendingConflict=true")
	}
	if store.HasPendingConflict("build-1", "b.go") {
		t.Fatalf("expected HasPendingConflict=false for unknown file")
	}
}

func TestProposedEditStoreDB_PendingConflicts(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	store.AddProposedEdits("build-1", []*ProposedEdit{
		makeEdit("a.go"),
		makeEdit("a.go"),
		makeEdit("b.go"),
	})

	conflicts := store.PendingConflicts("build-1")
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0] != "a.go" {
		t.Fatalf("expected conflict on a.go, got %s", conflicts[0])
	}
}

// ---------------------------------------------------------------------------
// DB error count
// ---------------------------------------------------------------------------

func TestProposedEditStoreDB_DBErrorCount_ZeroInitially(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	if store.DBErrorCount() != 0 {
		t.Fatalf("expected DBErrorCount=0 initially, got %d", store.DBErrorCount())
	}
}

func TestProposedEditStoreDB_GetPendingEdits_DBFailureFallsBack(t *testing.T) {
	// Use a closed DB to simulate failure
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go")})

	// Close the underlying DB to force errors
	sqlDB, _ := db.DB()
	sqlDB.Close()

	// Should fall back to in-memory cache without panic
	pending := store.GetPendingEdits("build-1")
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending edit from in-memory fallback, got %d", len(pending))
	}
	// Note: DBErrorCount only tracks write failures, not read query failures.
	// The fallback itself is the success signal.
}

func TestProposedEditStoreDB_GetAllEdits_DBFailureFallsBack(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go")})

	sqlDB, _ := db.DB()
	sqlDB.Close()

	all := store.GetAllEdits("build-1")
	if len(all) != 1 {
		t.Fatalf("expected 1 edit from in-memory fallback, got %d", len(all))
	}
}

func TestProposedEditStoreDB_AddProposedEdits_DBFailureFallsBack(t *testing.T) {
	db := testDBProposedEdits(t)
	store := NewProposedEditStoreWithDB(db)

	// Add one edit successfully first to populate in-memory cache
	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("a.go")})

	// Close DB and add another
	sqlDB, _ := db.DB()
	sqlDB.Close()

	store.AddProposedEdits("build-1", []*ProposedEdit{makeEdit("b.go")})

	// Both should be in memory despite DB failure
	all := store.GetAllEdits("build-1")
	if len(all) != 2 {
		t.Fatalf("expected 2 edits in memory after DB failure fallback, got %d", len(all))
	}
	if store.DBErrorCount() == 0 {
		t.Fatalf("expected DBErrorCount > 0 after DB write failure")
	}
}
