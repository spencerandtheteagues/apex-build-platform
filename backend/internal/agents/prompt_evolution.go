package agents

// prompt_evolution.go — Phase 10: live prompt guardrail injection.
//
// When a PromptPackVersion is activated and live_prompt_read_enabled is set,
// the PromptEvolutionStore caches the active guardrails and exposes a context
// string that gets injected alongside the historical build-learning block in
// every agent generation prompt. This closes the feedback loop:
//
//   failure cluster → proposal → approve → benchmark → activate → live injection
//
// The store is append-only in the happy path. Rollbacks create a new version
// record (existing code) and invalidate the cache. No prompt source is ever
// mutated — guardrails are advisory injection text, not source patches.

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"apex-build/pkg/models"

	"gorm.io/gorm"
)

const (
	promptEvolutionCacheTTL      = 2 * time.Minute
	promptEvolutionFeatureFlag   = "APEX_PROMPT_EVOLUTION"
	maxActiveGuardrailsPerTarget = 4
)

func promptEvolutionEnabled() bool {
	switch strings.TrimSpace(strings.ToLower(os.Getenv(promptEvolutionFeatureFlag))) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// activePromptGuardrail is a decoded entry from a live prompt_pack_version row.
type activePromptGuardrail struct {
	VersionID    string
	Scope        string
	TargetPrompt string
	Proposal     string
	FailureClass string
	ActivatedAt  time.Time
}

// PromptEvolutionStore caches active prompt guardrails loaded from the DB.
// It is safe for concurrent use and refreshes automatically on a TTL.
type PromptEvolutionStore struct {
	db      *gorm.DB
	mu      sync.RWMutex
	cache   []activePromptGuardrail
	cacheAt time.Time
	ttl     time.Duration
}

// NewPromptEvolutionStore returns a store backed by db.
func NewPromptEvolutionStore(db *gorm.DB) *PromptEvolutionStore {
	return &PromptEvolutionStore{
		db:  db,
		ttl: promptEvolutionCacheTTL,
	}
}

// Invalidate forces the next call to refresh from DB.
func (s *PromptEvolutionStore) Invalidate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.cacheAt = time.Time{}
	s.mu.Unlock()
}

// GuardrailContext returns an inject-ready prompt block containing all active
// guardrails for the given targetPrompt (or all guardrails if targetPrompt is "").
// Returns "" when prompt evolution is disabled or no guardrails are active.
func (s *PromptEvolutionStore) GuardrailContext(targetPrompt string) string {
	if s == nil || !promptEvolutionEnabled() {
		return ""
	}
	guardrails := s.activeGuardrailsFor(targetPrompt)
	if len(guardrails) == 0 {
		return ""
	}
	return formatGuardrailContext(guardrails, targetPrompt)
}

// EnableLiveRead sets live_prompt_read_enabled = true for the given version ID
// and invalidates the cache. Called after a pack is activated when
// APEX_PROMPT_EVOLUTION is set.
func (s *PromptEvolutionStore) EnableLiveRead(versionID string) error {
	if s == nil || s.db == nil {
		return nil
	}
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil
	}
	err := s.db.Model(&models.PromptPackVersion{}).
		Where("version_id = ?", versionID).
		Update("live_prompt_read_enabled", true).Error
	if err != nil {
		return fmt.Errorf("prompt_evolution: enable live read for %s: %w", versionID, err)
	}
	s.Invalidate()
	return nil
}

// DisableLiveRead sets live_prompt_read_enabled = false (used for rollback/disable).
func (s *PromptEvolutionStore) DisableLiveRead(versionID string) error {
	if s == nil || s.db == nil {
		return nil
	}
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil
	}
	err := s.db.Model(&models.PromptPackVersion{}).
		Where("version_id = ?", versionID).
		Update("live_prompt_read_enabled", false).Error
	if err != nil {
		return fmt.Errorf("prompt_evolution: disable live read for %s: %w", versionID, err)
	}
	s.Invalidate()
	return nil
}

func (s *PromptEvolutionStore) activeGuardrailsFor(targetPrompt string) []activePromptGuardrail {
	s.maybeRefresh()
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []activePromptGuardrail
	for _, g := range s.cache {
		if targetPrompt != "" && strings.TrimSpace(g.TargetPrompt) != strings.TrimSpace(targetPrompt) {
			continue
		}
		out = append(out, g)
		if len(out) >= maxActiveGuardrailsPerTarget {
			break
		}
	}
	return out
}

func (s *PromptEvolutionStore) maybeRefresh() {
	s.mu.RLock()
	fresh := !s.cacheAt.IsZero() && time.Since(s.cacheAt) < s.ttl
	s.mu.RUnlock()
	if fresh {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Re-check under write lock.
	if !s.cacheAt.IsZero() && time.Since(s.cacheAt) < s.ttl {
		return
	}
	loaded, err := s.loadFromDB()
	if err != nil {
		log.Printf("[prompt_evolution] cache refresh error: %v", err)
		return
	}
	s.cache = loaded
	s.cacheAt = time.Now()
}

func (s *PromptEvolutionStore) loadFromDB() ([]activePromptGuardrail, error) {
	if s.db == nil {
		return nil, nil
	}
	var rows []models.PromptPackVersion
	err := s.db.Where("status = ? AND live_prompt_read_enabled = ? AND deleted_at IS NULL",
		string(PromptPackVersionActive), true).
		Order("activated_at ASC").
		Limit(32).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]activePromptGuardrail, 0, len(rows))
	for _, row := range rows {
		var changes []PromptPackDraftChange
		if strings.TrimSpace(row.ChangesJSON) != "" {
			if err2 := json.Unmarshal([]byte(row.ChangesJSON), &changes); err2 != nil {
				continue
			}
		}
		for _, change := range changes {
			proposal := strings.TrimSpace(change.Proposal)
			targetPrompt := strings.TrimSpace(change.TargetPrompt)
			if proposal == "" || targetPrompt == "" {
				continue
			}
			out = append(out, activePromptGuardrail{
				VersionID:    strings.TrimSpace(row.VersionID),
				Scope:        strings.TrimSpace(row.Scope),
				TargetPrompt: targetPrompt,
				Proposal:     proposal,
				FailureClass: strings.TrimSpace(change.FailureCluster),
				ActivatedAt:  derefTime(row.ActivatedAt),
			})
		}
	}
	return out, nil
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// formatGuardrailContext produces an XML-tagged prompt block for agent injection.
func formatGuardrailContext(guardrails []activePromptGuardrail, targetPrompt string) string {
	if len(guardrails) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<active_prompt_guardrails")
	if targetPrompt != "" {
		sb.WriteString(fmt.Sprintf(` target=%q`, targetPrompt))
	}
	sb.WriteString(">\n")
	sb.WriteString("These guardrails were derived from recurring build failures and have been approved and benchmark-validated.\n")
	sb.WriteString("Apply them proactively — they represent patterns that have caused repeated failures on similar builds.\n")
	for _, g := range guardrails {
		sb.WriteString("- ")
		if g.FailureClass != "" {
			sb.WriteString(fmt.Sprintf("[%s] ", g.FailureClass))
		}
		sb.WriteString(g.Proposal)
		sb.WriteString("\n")
	}
	sb.WriteString("</active_prompt_guardrails>\n")
	return sb.String()
}

// ── AgentManager integration ──────────────────────────────────────────────────

// promptEvolutionGuardrailContext returns active guardrails for a given target
// prompt key, safe to call from the prompt assembly hot-path.
func (am *AgentManager) promptEvolutionGuardrailContext(targetPrompt string) string {
	if am == nil {
		return ""
	}
	am.mu.RLock()
	store := am.promptEvolution
	am.mu.RUnlock()
	return store.GuardrailContext(targetPrompt)
}

// promptEvolutionTargetForRole maps an agent role + task type to the
// target_prompt key used in PromptPackDraftChange, so guardrails are only
// injected for the relevant generation phase.
func promptEvolutionTargetForRole(role AgentRole, task *Task) string {
	if task != nil {
		switch task.Type {
		case TaskFix:
			return "compile_repair"
		case TaskPlan:
			return "war_room_contract_planning"
		}
	}
	switch role {
	case RoleFrontend:
		return "interaction_canary"
	case RoleBackend:
		return "auth_contract"
	case RoleDatabase:
		return "data_contract"
	case RoleReviewer:
		return "visual_review"
	default:
		return "" // all guardrails for unrecognized roles
	}
}

// enablePromptPackLiveRead turns on live injection for a version and invalidates
// the cache. Called from activatePromptPackRequest when APEX_PROMPT_EVOLUTION is on.
func (am *AgentManager) enablePromptPackLiveRead(versionID string) {
	if am == nil {
		return
	}
	am.mu.RLock()
	store := am.promptEvolution
	am.mu.RUnlock()
	if store == nil {
		return
	}
	if err := store.EnableLiveRead(versionID); err != nil {
		log.Printf("[prompt_evolution] failed to enable live read for %s: %v", versionID, err)
	}
}
