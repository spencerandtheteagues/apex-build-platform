package agents

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
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

// ProposedEditStore manages proposed edits in memory
type ProposedEditStore struct {
	edits map[string][]*ProposedEdit // buildID -> edits
	mu    sync.RWMutex
}

// NewProposedEditStore creates a new store
func NewProposedEditStore() *ProposedEditStore {
	return &ProposedEditStore{
		edits: make(map[string][]*ProposedEdit),
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

	s.edits[buildID] = append(s.edits[buildID], edits...)
}

// GetPendingEdits returns all edits with status pending for the given build.
func (s *ProposedEditStore) GetPendingEdits(buildID string) []*ProposedEdit {
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
	var approved []*ProposedEdit

	for _, edit := range s.edits[buildID] {
		if idSet[edit.ID] {
			if edit.Status != EditPending {
				return nil, fmt.Errorf("edit %s is not pending (status=%s)", edit.ID, edit.Status)
			}
			edit.Status = EditApproved
			edit.ReviewedAt = &now
			approved = append(approved, edit)
			delete(idSet, edit.ID)
		}
	}

	if len(idSet) > 0 {
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

	for _, edit := range s.edits[buildID] {
		if idSet[edit.ID] {
			if edit.Status != EditPending {
				return fmt.Errorf("edit %s is not pending (status=%s)", edit.ID, edit.Status)
			}
			edit.Status = EditRejected
			edit.ReviewedAt = &now
			delete(idSet, edit.ID)
		}
	}

	if len(idSet) > 0 {
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

	delete(s.edits, buildID)
}
