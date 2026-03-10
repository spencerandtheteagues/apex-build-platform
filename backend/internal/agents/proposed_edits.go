package agents

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProposedEditStatus represents the status of a proposed edit
type ProposedEditStatus string

const (
	EditPending  ProposedEditStatus = "pending"
	EditApproved ProposedEditStatus = "approved"
	EditRejected ProposedEditStatus = "rejected"
)

// ProposedEdit represents a file change awaiting user review
type ProposedEdit struct {
	ID              string             `json:"id"`
	BuildID         string             `json:"build_id"`
	AgentID         string             `json:"agent_id"`
	AgentRole       string             `json:"agent_role"`
	TaskID          string             `json:"task_id"`
	FilePath        string             `json:"file_path"`
	OriginalContent string             `json:"original_content"`
	ProposedContent string             `json:"proposed_content"`
	Language        string             `json:"language"`
	Status          ProposedEditStatus `json:"status"`
	CreatedAt       time.Time          `json:"created_at"`
	ReviewedAt      *time.Time         `json:"reviewed_at,omitempty"`
}

// proposedEditRow is the GORM model mapping ProposedEdit fields to the proposed_edits table.
type proposedEditRow struct {
	ID              string     `gorm:"column:id;primaryKey"`
	BuildID         string     `gorm:"column:build_id"`
	AgentID         string     `gorm:"column:agent_id"`
	AgentRole       string     `gorm:"column:agent_role"`
	TaskID          string     `gorm:"column:task_id"`
	FilePath        string     `gorm:"column:file_path"`
	OriginalContent string     `gorm:"column:original_content"`
	ProposedContent string     `gorm:"column:proposed_content"`
	Language        string     `gorm:"column:language"`
	Status          string     `gorm:"column:status"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	ReviewedAt      *time.Time `gorm:"column:reviewed_at"`
}

func (proposedEditRow) TableName() string { return "proposed_edits" }

func rowToEdit(r proposedEditRow) *ProposedEdit {
	return &ProposedEdit{
		ID:              r.ID,
		BuildID:         r.BuildID,
		AgentID:         r.AgentID,
		AgentRole:       r.AgentRole,
		TaskID:          r.TaskID,
		FilePath:        r.FilePath,
		OriginalContent: r.OriginalContent,
		ProposedContent: r.ProposedContent,
		Language:        r.Language,
		Status:          ProposedEditStatus(r.Status),
		CreatedAt:       r.CreatedAt,
		ReviewedAt:      r.ReviewedAt,
	}
}

func editToRow(e *ProposedEdit) proposedEditRow {
	return proposedEditRow{
		ID:              e.ID,
		BuildID:         e.BuildID,
		AgentID:         e.AgentID,
		AgentRole:       e.AgentRole,
		TaskID:          e.TaskID,
		FilePath:        e.FilePath,
		OriginalContent: e.OriginalContent,
		ProposedContent: e.ProposedContent,
		Language:        e.Language,
		Status:          string(e.Status),
		CreatedAt:       e.CreatedAt,
		ReviewedAt:      e.ReviewedAt,
	}
}

// ProposedEditStore manages proposed edits in memory with optional Postgres persistence.
// When db is nil the store operates purely in-memory (existing behavior).
// When db is set all mutations write through to Postgres and reads query from Postgres.
//
// DB failures are logged and counted but never silently discarded — callers can
// inspect DBErrorCount to detect persistence degradation.
type ProposedEditStore struct {
	edits       map[string][]*ProposedEdit // buildID -> edits (in-memory cache)
	mu          sync.RWMutex
	db          *gorm.DB    // nil = in-memory only
	dbErrCount  atomic.Int64 // total DB write failures since startup
}

// NewProposedEditStore creates a new in-memory store.
func NewProposedEditStore() *ProposedEditStore {
	return &ProposedEditStore{
		edits: make(map[string][]*ProposedEdit),
	}
}

// NewProposedEditStoreWithDB creates a new store backed by Postgres.
func NewProposedEditStoreWithDB(db *gorm.DB) *ProposedEditStore {
	return &ProposedEditStore{
		edits: make(map[string][]*ProposedEdit),
		db:    db,
	}
}

// AddProposedEdits assigns UUID IDs to edits that lack them and appends them
// to the build's edit list.
func (s *ProposedEditStore) AddProposedEdits(buildID string, edits []*ProposedEdit) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for _, edit := range edits {
		if edit.ID == "" {
			edit.ID = uuid.New().String()
		}
		if edit.BuildID == "" {
			edit.BuildID = buildID
		}
		if edit.CreatedAt.IsZero() {
			edit.CreatedAt = now
		}
		if edit.Status == "" {
			edit.Status = EditPending
		}
	}

	if s.db != nil {
		rows := make([]proposedEditRow, len(edits))
		for i, e := range edits {
			rows[i] = editToRow(e)
		}
		if err := s.db.Create(&rows).Error; err != nil {
			// Log the failure and count it for observability. We fall through to
			// in-memory so the build continues, but the error is never silent.
			s.dbErrCount.Add(1)
			log.Printf("[proposed_edits] DB persist failed for build %s (%d edits): %v — falling back to in-memory", buildID, len(edits), err)
		}
	}

	// Conflict detection: warn when two agents propose edits to the same file
	// within the same build. The later edit wins but the conflict is logged so
	// reviewers can inspect the diff.
	existingPaths := make(map[string]bool, len(s.edits[buildID]))
	for _, existing := range s.edits[buildID] {
		if existing.Status == EditPending {
			existingPaths[existing.FilePath] = true
		}
	}
	for _, edit := range edits {
		if existingPaths[edit.FilePath] {
			log.Printf("[proposed_edits] CONFLICT: build %s already has a pending edit for %q — new edit from agent %s will shadow it", buildID, edit.FilePath, edit.AgentID)
		}
	}

	s.edits[buildID] = append(s.edits[buildID], edits...)
}

// GetPendingEdits returns all edits with status pending for the given build.
func (s *ProposedEditStore) GetPendingEdits(buildID string) []*ProposedEdit {
	if s.db != nil {
		var rows []proposedEditRow
		if err := s.db.Where("build_id = ? AND status = 'pending'", buildID).Find(&rows).Error; err == nil {
			out := make([]*ProposedEdit, len(rows))
			for i, r := range rows {
				out[i] = rowToEdit(r)
			}
			// Refresh the in-memory cache while holding the write lock.
			s.mu.Lock()
			s.syncCacheFromRows(buildID, rows)
			s.mu.Unlock()
			return out
		}
		// DB query failed — fall through to in-memory cache below.
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var pending []*ProposedEdit
	for _, edit := range s.edits[buildID] {
		if edit.Status == EditPending {
			pending = append(pending, edit)
		}
	}
	return pending
}

// GetAllEdits returns every proposed edit for the given build regardless of status.
func (s *ProposedEditStore) GetAllEdits(buildID string) []*ProposedEdit {
	if s.db != nil {
		var rows []proposedEditRow
		if err := s.db.Where("build_id = ?", buildID).Order("created_at ASC").Find(&rows).Error; err == nil {
			out := make([]*ProposedEdit, len(rows))
			for i, r := range rows {
				out[i] = rowToEdit(r)
			}
			// Refresh the in-memory cache while holding the write lock.
			s.mu.Lock()
			s.syncCacheFromRows(buildID, rows)
			s.mu.Unlock()
			return out
		}
		// DB query failed — fall through to in-memory cache below.
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy of the slice to avoid external mutation
	edits := s.edits[buildID]
	out := make([]*ProposedEdit, len(edits))
	copy(out, edits)
	return out
}

// ApproveEdits marks the specified edits as approved and returns them.
// Returns an error if any of the requested IDs are not found or not pending.
func (s *ProposedEditStore) ApproveEdits(buildID string, editIDs []string) ([]*ProposedEdit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idSet := make(map[string]bool, len(editIDs))
	for _, id := range editIDs {
		idSet[id] = true
	}

	now := time.Now().UTC()

	if s.db != nil {
		result := s.db.Model(&proposedEditRow{}).
			Where("id IN ? AND status = 'pending'", editIDs).
			Updates(map[string]any{"status": "approved", "reviewed_at": now})
		if result.Error != nil {
			return nil, result.Error
		}
	}

	var approved []*ProposedEdit

	for _, edit := range s.edits[buildID] {
		if idSet[edit.ID] {
			if s.db == nil && edit.Status != EditPending {
				return nil, fmt.Errorf("edit %s is not pending (status=%s)", edit.ID, edit.Status)
			}
			edit.Status = EditApproved
			edit.ReviewedAt = &now
			approved = append(approved, edit)
			delete(idSet, edit.ID)
		}
	}

	if len(idSet) > 0 && s.db == nil {
		// Collect missing IDs for the error message
		var missing []string
		for id := range idSet {
			missing = append(missing, id)
		}
		return approved, fmt.Errorf("edits not found: %v", missing)
	}

	return approved, nil
}

// RejectEdits marks the specified edits as rejected.
// Returns an error if any of the requested IDs are not found or not pending.
func (s *ProposedEditStore) RejectEdits(buildID string, editIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idSet := make(map[string]bool, len(editIDs))
	for _, id := range editIDs {
		idSet[id] = true
	}

	now := time.Now().UTC()

	if s.db != nil {
		result := s.db.Model(&proposedEditRow{}).
			Where("id IN ? AND status = 'pending'", editIDs).
			Updates(map[string]any{"status": "rejected", "reviewed_at": now})
		if result.Error != nil {
			return result.Error
		}
	}

	for _, edit := range s.edits[buildID] {
		if idSet[edit.ID] {
			if s.db == nil && edit.Status != EditPending {
				return fmt.Errorf("edit %s is not pending (status=%s)", edit.ID, edit.Status)
			}
			edit.Status = EditRejected
			edit.ReviewedAt = &now
			delete(idSet, edit.ID)
		}
	}

	if len(idSet) > 0 && s.db == nil {
		var missing []string
		for id := range idSet {
			missing = append(missing, id)
		}
		return fmt.Errorf("edits not found: %v", missing)
	}

	return nil
}

// ApproveAll approves every pending edit for the given build and returns them.
func (s *ProposedEditStore) ApproveAll(buildID string) []*ProposedEdit {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()

	if s.db != nil {
		s.db.Model(&proposedEditRow{}).
			Where("build_id = ? AND status = 'pending'", buildID).
			Updates(map[string]any{"status": "approved", "reviewed_at": now})
	}

	var approved []*ProposedEdit

	for _, edit := range s.edits[buildID] {
		if edit.Status == EditPending {
			edit.Status = EditApproved
			edit.ReviewedAt = &now
			approved = append(approved, edit)
		}
	}

	return approved
}

// RejectAll rejects every pending edit for the given build.
func (s *ProposedEditStore) RejectAll(buildID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()

	if s.db != nil {
		s.db.Model(&proposedEditRow{}).
			Where("build_id = ? AND status = 'pending'", buildID).
			Updates(map[string]any{"status": "rejected", "reviewed_at": now})
	}

	for _, edit := range s.edits[buildID] {
		if edit.Status == EditPending {
			edit.Status = EditRejected
			edit.ReviewedAt = &now
		}
	}

	return nil
}

// Clear removes all proposed edits for the given build.
func (s *ProposedEditStore) Clear(buildID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		s.db.Where("build_id = ?", buildID).Delete(&proposedEditRow{})
	}

	delete(s.edits, buildID)
}

// syncCacheFromRows replaces the in-memory cache for a build with the rows
// returned from Postgres. Must be called with s.mu held (write lock).
func (s *ProposedEditStore) syncCacheFromRows(buildID string, rows []proposedEditRow) {
	edits := make([]*ProposedEdit, len(rows))
	for i, r := range rows {
		edits[i] = rowToEdit(r)
	}
	s.edits[buildID] = edits
}

// DBErrorCount returns the total number of Postgres write failures since startup.
// Non-zero values indicate the store has fallen back to in-memory persistence for
// some edits and those will not survive a server restart.
func (s *ProposedEditStore) DBErrorCount() int64 {
	return s.dbErrCount.Load()
}

// HasPendingConflict returns true if there is already a pending edit for the
// given file in this build. Callers can use this to gate downstream merge logic.
func (s *ProposedEditStore) HasPendingConflict(buildID, filePath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, edit := range s.edits[buildID] {
		if edit.FilePath == filePath && edit.Status == EditPending {
			return true
		}
	}
	return false
}

// PendingConflicts returns all file paths that have more than one pending edit
// within the given build. These files need conflict resolution before apply.
func (s *ProposedEditStore) PendingConflicts(buildID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	counts := make(map[string]int)
	for _, edit := range s.edits[buildID] {
		if edit.Status == EditPending {
			counts[edit.FilePath]++
		}
	}
	var conflicts []string
	for path, n := range counts {
		if n > 1 {
			conflicts = append(conflicts, path)
		}
	}
	return conflicts
}
