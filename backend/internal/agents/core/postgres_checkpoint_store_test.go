package core

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testCheckpointDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&fsmCheckpointRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func sampleCheckpoint(buildID string) *Checkpoint {
	return &Checkpoint{
		ID:           "cp-" + buildID,
		BuildID:      buildID,
		State:        StateExecuting,
		StepIndex:    3,
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
		Description:  "test checkpoint",
		SnapshotJSON: `{"phase":"test"}`,
		CanRestore:   true,
	}
}

func TestPostgresCheckpointStore_SaveAndGet(t *testing.T) {
	db := testCheckpointDB(t)
	store := NewPostgresCheckpointStore(db)
	ctx := context.Background()

	cp := sampleCheckpoint("build-a")
	if err := store.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}

	got, err := store.GetCheckpoint(ctx, cp.ID)
	if err != nil {
		t.Fatalf("GetCheckpoint: %v", err)
	}
	if got.ID != cp.ID {
		t.Errorf("ID: got %s, want %s", got.ID, cp.ID)
	}
	if got.BuildID != cp.BuildID {
		t.Errorf("BuildID: got %s, want %s", got.BuildID, cp.BuildID)
	}
	if got.State != cp.State {
		t.Errorf("State: got %s, want %s", got.State, cp.State)
	}
	if got.StepIndex != cp.StepIndex {
		t.Errorf("StepIndex: got %d, want %d", got.StepIndex, cp.StepIndex)
	}
	if got.SnapshotJSON != cp.SnapshotJSON {
		t.Errorf("SnapshotJSON: got %q, want %q", got.SnapshotJSON, cp.SnapshotJSON)
	}
	if got.CanRestore != cp.CanRestore {
		t.Errorf("CanRestore: got %v, want %v", got.CanRestore, cp.CanRestore)
	}
}

func TestPostgresCheckpointStore_GetUnknown_ReturnsError(t *testing.T) {
	db := testCheckpointDB(t)
	store := NewPostgresCheckpointStore(db)

	_, err := store.GetCheckpoint(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown checkpoint ID")
	}
}

func TestPostgresCheckpointStore_ListByBuildID(t *testing.T) {
	db := testCheckpointDB(t)
	store := NewPostgresCheckpointStore(db)
	ctx := context.Background()

	cp1 := sampleCheckpoint("build-list")
	cp1.ID = "list-cp-1"
	cp2 := sampleCheckpoint("build-list")
	cp2.ID = "list-cp-2"
	cp3 := sampleCheckpoint("other-build")
	cp3.ID = "other-cp"

	_ = store.SaveCheckpoint(ctx, cp1)
	_ = store.SaveCheckpoint(ctx, cp2)
	_ = store.SaveCheckpoint(ctx, cp3)

	results, err := store.ListCheckpoints(ctx, "build-list")
	if err != nil {
		t.Fatalf("ListCheckpoints: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 checkpoints for build-list, got %d", len(results))
	}
	for _, r := range results {
		if r.BuildID != "build-list" {
			t.Errorf("unexpected BuildID %q in list results", r.BuildID)
		}
	}
}

func TestPostgresCheckpointStore_ListEmpty_ReturnsEmptySlice(t *testing.T) {
	db := testCheckpointDB(t)
	store := NewPostgresCheckpointStore(db)

	results, err := store.ListCheckpoints(context.Background(), "no-such-build")
	if err != nil {
		t.Fatalf("ListCheckpoints: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for unknown build, got %d", len(results))
	}
}

func TestPostgresCheckpointStore_Delete(t *testing.T) {
	db := testCheckpointDB(t)
	store := NewPostgresCheckpointStore(db)
	ctx := context.Background()

	cp := sampleCheckpoint("del-build")
	_ = store.SaveCheckpoint(ctx, cp)

	if err := store.DeleteCheckpoint(ctx, cp.ID); err != nil {
		t.Fatalf("DeleteCheckpoint: %v", err)
	}

	results, _ := store.ListCheckpoints(ctx, cp.BuildID)
	if len(results) != 0 {
		t.Fatalf("expected 0 checkpoints after delete, got %d", len(results))
	}
}

func TestPostgresCheckpointStore_SaveDuplicate_ReturnsError(t *testing.T) {
	db := testCheckpointDB(t)
	store := NewPostgresCheckpointStore(db)
	ctx := context.Background()

	cp := sampleCheckpoint("dup-build")
	_ = store.SaveCheckpoint(ctx, cp)

	// Saving same ID twice should fail (primary key violation)
	err := store.SaveCheckpoint(ctx, cp)
	if err == nil {
		t.Fatal("expected error on duplicate checkpoint ID")
	}
}

// --- FSM with Postgres store integration ---

func TestFSMWithPostgresStore_PersistsCheckpoint(t *testing.T) {
	db := testCheckpointDB(t)
	store := NewPostgresCheckpointStore(db)

	fsm := NewAgentFSM(AgentFSMConfig{
		BuildID:         "pg-build",
		CheckpointStore: store,
	})

	_ = fsm.Transition(EventStart)
	_ = fsm.Transition(EventInitialized)
	_ = fsm.Transition(EventPlanReady)

	cpID, err := fsm.CreateCheckpoint(context.Background(), "pg test", `{"x":1}`)
	if err != nil {
		t.Fatalf("CreateCheckpoint with postgres store: %v", err)
	}

	// Verify it's in the DB
	stored, err := store.GetCheckpoint(context.Background(), cpID)
	if err != nil {
		t.Fatalf("GetCheckpoint from postgres store: %v", err)
	}
	if stored.BuildID != "pg-build" {
		t.Errorf("BuildID in DB = %q, want %q", stored.BuildID, "pg-build")
	}
}

func TestFSMWithPostgresStore_RollbackLooksUpStore(t *testing.T) {
	db := testCheckpointDB(t)
	store := NewPostgresCheckpointStore(db)
	ctx := context.Background()

	// Pre-seed a checkpoint directly in the store (simulates a separate FSM instance)
	cp := &Checkpoint{
		ID:          "ext-cp",
		BuildID:     "ext-build",
		State:       StatePlanning,
		StepIndex:   1,
		CreatedAt:   time.Now().UTC(),
		Description: "external checkpoint",
		CanRestore:  true,
	}
	_ = store.SaveCheckpoint(ctx, cp)

	// New FSM with the same store but no in-memory checkpoints
	fsm := NewAgentFSM(AgentFSMConfig{
		BuildID:         "ext-build",
		CheckpointStore: store,
	})
	_ = fsm.Transition(EventStart)
	_ = fsm.Transition(EventInitialized)
	_ = fsm.Transition(EventPlanReady)
	_ = fsm.Transition(EventStepComplete)

	// Rollback should find the checkpoint via the store
	restored, err := fsm.RollbackTo(ctx, "ext-cp")
	if err != nil {
		t.Fatalf("RollbackTo via postgres store: %v", err)
	}
	if restored.ID != "ext-cp" {
		t.Errorf("restored ID = %q, want ext-cp", restored.ID)
	}
	if fsm.CurrentState() != StatePlanning {
		t.Errorf("state after rollback = %s, want planning", fsm.CurrentState())
	}
}
