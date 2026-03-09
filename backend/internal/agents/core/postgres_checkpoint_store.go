package core

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// fsmCheckpointRow is the GORM model for the fsm_checkpoints table.
type fsmCheckpointRow struct {
	ID           string    `gorm:"primaryKey"`
	BuildID      string    `gorm:"not null;index:idx_fsm_checkpoints_build_id"`
	State        string    `gorm:"not null"`
	StepIndex    int       `gorm:"not null;default:0"`
	Description  string    `gorm:"not null;default:''"`
	SnapshotJSON string    `gorm:"not null;default:''"`
	CanRestore   bool      `gorm:"not null;default:true"`
	CreatedAt    time.Time `gorm:"not null;autoCreateTime"`
}

func (fsmCheckpointRow) TableName() string { return "fsm_checkpoints" }

// PostgresCheckpointStore implements CheckpointStore backed by Postgres via GORM.
type PostgresCheckpointStore struct {
	db *gorm.DB
}

// NewPostgresCheckpointStore creates a new PostgresCheckpointStore.
// The caller is responsible for running migrations before using this store.
func NewPostgresCheckpointStore(db *gorm.DB) *PostgresCheckpointStore {
	return &PostgresCheckpointStore{db: db}
}

func (s *PostgresCheckpointStore) SaveCheckpoint(ctx context.Context, cp *Checkpoint) error {
	row := fsmCheckpointRow{
		ID:           cp.ID,
		BuildID:      cp.BuildID,
		State:        string(cp.State),
		StepIndex:    cp.StepIndex,
		Description:  cp.Description,
		SnapshotJSON: cp.SnapshotJSON,
		CanRestore:   cp.CanRestore,
		CreatedAt:    cp.CreatedAt,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("postgres_checkpoint_store: save: %w", err)
	}
	return nil
}

func (s *PostgresCheckpointStore) GetCheckpoint(ctx context.Context, id string) (*Checkpoint, error) {
	var row fsmCheckpointRow
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("postgres_checkpoint_store: get %s: %w", id, err)
	}
	return rowToCheckpoint(&row), nil
}

func (s *PostgresCheckpointStore) ListCheckpoints(ctx context.Context, buildID string) ([]*Checkpoint, error) {
	var rows []fsmCheckpointRow
	if err := s.db.WithContext(ctx).
		Where("build_id = ?", buildID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("postgres_checkpoint_store: list %s: %w", buildID, err)
	}
	result := make([]*Checkpoint, len(rows))
	for i := range rows {
		result[i] = rowToCheckpoint(&rows[i])
	}
	return result, nil
}

func (s *PostgresCheckpointStore) DeleteCheckpoint(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).Delete(&fsmCheckpointRow{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("postgres_checkpoint_store: delete %s: %w", id, err)
	}
	return nil
}

func rowToCheckpoint(row *fsmCheckpointRow) *Checkpoint {
	return &Checkpoint{
		ID:           row.ID,
		BuildID:      row.BuildID,
		State:        AgentState(row.State),
		StepIndex:    row.StepIndex,
		Description:  row.Description,
		SnapshotJSON: row.SnapshotJSON,
		CanRestore:   row.CanRestore,
		CreatedAt:    row.CreatedAt,
	}
}
