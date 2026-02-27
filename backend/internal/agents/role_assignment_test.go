package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"apex-build/internal/ai"
)

// ---------------------------------------------------------------------------
// 1. ExpandUserCategory
// ---------------------------------------------------------------------------

func TestExpandUserCategory_Architect(t *testing.T) {
	roles := ExpandUserCategory(CategoryArchitect)
	expected := []AgentRole{RolePlanner, RoleArchitect, RoleReviewer}
	if len(roles) != len(expected) {
		t.Fatalf("CategoryArchitect: expected %d roles, got %d", len(expected), len(roles))
	}
	for i, r := range expected {
		if roles[i] != r {
			t.Errorf("CategoryArchitect[%d]: expected %q, got %q", i, r, roles[i])
		}
	}
}

func TestExpandUserCategory_Coder(t *testing.T) {
	roles := ExpandUserCategory(CategoryCoder)
	expected := []AgentRole{RoleFrontend, RoleBackend, RoleDatabase}
	if len(roles) != len(expected) {
		t.Fatalf("CategoryCoder: expected %d roles, got %d", len(expected), len(roles))
	}
	for i, r := range expected {
		if roles[i] != r {
			t.Errorf("CategoryCoder[%d]: expected %q, got %q", i, r, roles[i])
		}
	}
}

func TestExpandUserCategory_Tester(t *testing.T) {
	roles := ExpandUserCategory(CategoryTester)
	expected := []AgentRole{RoleTesting}
	if len(roles) != len(expected) {
		t.Fatalf("CategoryTester: expected %d roles, got %d", len(expected), len(roles))
	}
	if roles[0] != RoleTesting {
		t.Errorf("CategoryTester[0]: expected %q, got %q", RoleTesting, roles[0])
	}
}

func TestExpandUserCategory_DevOps(t *testing.T) {
	roles := ExpandUserCategory(CategoryDevOps)
	expected := []AgentRole{RoleDevOps, RoleSolver}
	if len(roles) != len(expected) {
		t.Fatalf("CategoryDevOps: expected %d roles, got %d", len(expected), len(roles))
	}
	for i, r := range expected {
		if roles[i] != r {
			t.Errorf("CategoryDevOps[%d]: expected %q, got %q", i, r, roles[i])
		}
	}
}

func TestExpandUserCategory_Unknown(t *testing.T) {
	roles := ExpandUserCategory(UserRoleCategory("nonexistent"))
	if roles != nil {
		t.Fatalf("unknown category: expected nil, got %v", roles)
	}
}

func TestExpandUserCategory_EmptyString(t *testing.T) {
	roles := ExpandUserCategory(UserRoleCategory(""))
	if roles != nil {
		t.Fatalf("empty category: expected nil, got %v", roles)
	}
}

func TestExpandUserCategory_CaseSensitive(t *testing.T) {
	// Ensure uppercase variants are not silently accepted.
	for _, cat := range []string{"Architect", "CODER", "Tester", "DEVOPS"} {
		roles := ExpandUserCategory(UserRoleCategory(cat))
		if roles != nil {
			t.Errorf("UserRoleCategory(%q) should return nil (case-sensitive), got %v", cat, roles)
		}
	}
}

func TestExpandUserCategory_NoDuplicateRoles(t *testing.T) {
	categories := []UserRoleCategory{CategoryArchitect, CategoryCoder, CategoryTester, CategoryDevOps}
	for _, cat := range categories {
		roles := ExpandUserCategory(cat)
		seen := make(map[AgentRole]bool)
		for _, r := range roles {
			if seen[r] {
				t.Errorf("category %q has duplicate role %q", cat, r)
			}
			seen[r] = true
		}
	}
}

func TestExpandUserCategory_AllCategoriesCoverExpectedRoles(t *testing.T) {
	// Verify the total number of roles covered across all categories (excluding RoleLead).
	covered := make(map[AgentRole]bool)
	categories := []UserRoleCategory{CategoryArchitect, CategoryCoder, CategoryTester, CategoryDevOps}
	for _, cat := range categories {
		for _, r := range ExpandUserCategory(cat) {
			covered[r] = true
		}
	}
	// planner, architect, reviewer, frontend, backend, database, testing, devops, solver = 9
	// RoleLead is intentionally excluded from user categories.
	expectedCount := 9
	if len(covered) != expectedCount {
		t.Errorf("expected %d distinct roles across all categories, got %d: %v", expectedCount, len(covered), covered)
	}
	if covered[RoleLead] {
		t.Error("RoleLead should not be in any user-facing category")
	}
}

// ---------------------------------------------------------------------------
// 2. BuildRequest.RoleAssignments JSON serialization
// ---------------------------------------------------------------------------

func TestBuildRequest_RoleAssignments_MarshalRoundTrip(t *testing.T) {
	original := BuildRequest{
		Description: "Build a todo app",
		Mode:        ModeFast,
		RoleAssignments: map[string]string{
			"architect": "claude",
			"coder":     "gpt4",
			"tester":    "gemini",
			"devops":    "grok",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded BuildRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(decoded.RoleAssignments) != 4 {
		t.Fatalf("expected 4 role assignments, got %d", len(decoded.RoleAssignments))
	}
	for k, v := range original.RoleAssignments {
		if decoded.RoleAssignments[k] != v {
			t.Errorf("role_assignments[%q]: expected %q, got %q", k, v, decoded.RoleAssignments[k])
		}
	}
}

func TestBuildRequest_RoleAssignments_OmittedWhenNil(t *testing.T) {
	req := BuildRequest{
		Description: "Build a chat app",
		Mode:        ModeFull,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to raw map failed: %v", err)
	}

	if _, exists := raw["role_assignments"]; exists {
		t.Error("role_assignments should be omitted from JSON when nil")
	}
}

func TestBuildRequest_RoleAssignments_EmptyMap(t *testing.T) {
	req := BuildRequest{
		Description:     "Build an app",
		Mode:            ModeFast,
		RoleAssignments: map[string]string{},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded BuildRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// With omitempty, an empty map is omitted from JSON, so after roundtrip
	// the field will be nil. Verify no entries survive regardless.
	if len(decoded.RoleAssignments) != 0 {
		t.Errorf("expected zero role assignments after roundtrip, got %v", decoded.RoleAssignments)
	}
}

func TestBuildRequest_RoleAssignments_PartialAssignment(t *testing.T) {
	// Users can assign only some categories and leave others to the platform.
	req := BuildRequest{
		Description: "Build something cool",
		Mode:        ModeFull,
		RoleAssignments: map[string]string{
			"architect": "claude",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded BuildRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(decoded.RoleAssignments) != 1 {
		t.Fatalf("expected 1 role assignment, got %d", len(decoded.RoleAssignments))
	}
	if decoded.RoleAssignments["architect"] != "claude" {
		t.Errorf("expected architect=claude, got %q", decoded.RoleAssignments["architect"])
	}
}

func TestBuildRequest_JSONFieldName(t *testing.T) {
	// Verify the JSON key is "role_assignments", not "RoleAssignments".
	req := BuildRequest{
		Description: "Test app",
		RoleAssignments: map[string]string{
			"coder": "gemini",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, exists := raw["role_assignments"]; !exists {
		t.Error("expected JSON key 'role_assignments' to be present")
	}
	if _, exists := raw["RoleAssignments"]; exists {
		t.Error("JSON key should be 'role_assignments' (snake_case), not 'RoleAssignments'")
	}
}

// ---------------------------------------------------------------------------
// 3. Handler validation: StartBuild rejects invalid role categories/providers
// ---------------------------------------------------------------------------

func TestStartBuild_RejectsInvalidRoleCategory(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	tests := []struct {
		name     string
		category string
		provider string
	}{
		{"unknown category", "wizard", "claude"},
		{"typo category", "architectt", "claude"},
		{"uppercase category", "Architect", "claude"},
		{"numeric category", "123", "gpt4"},
		{"empty category", "", "claude"},
		{"lead is not a user category", "lead", "claude"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{
				"description":      "Build me a fully functional e-commerce platform with payments",
				"role_assignments": map[string]string{tc.category: tc.provider},
			})
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/build/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			testRouter(am).ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for category %q, got %d: %s", tc.category, w.Code, w.Body.String())
			}
			var respBody map[string]any
			json.Unmarshal(w.Body.Bytes(), &respBody)
			if respBody["error"] != "invalid role category" {
				t.Errorf("expected error='invalid role category', got %q", respBody["error"])
			}
		})
	}
}

func TestStartBuild_RejectsInvalidProvider(t *testing.T) {
	am := &AgentManager{
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	tests := []struct {
		name     string
		provider string
	}{
		{"unknown provider", "deepseek"},
		{"typo provider", "claudee"},
		{"uppercase provider", "Claude"},
		{"empty provider", ""},
		{"numeric provider", "42"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{
				"description":      "Build me a fully functional e-commerce platform with payments",
				"role_assignments": map[string]string{"architect": tc.provider},
			})
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/build/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			testRouter(am).ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for provider %q, got %d: %s", tc.provider, w.Code, w.Body.String())
			}
			var respBody map[string]any
			json.Unmarshal(w.Body.Bytes(), &respBody)
			if respBody["error"] != "invalid provider" {
				t.Errorf("expected error='invalid provider', got %q", respBody["error"])
			}
		})
	}
}

// validationOnlyManager returns an AgentManager wired just enough for
// CreateBuild + the async StartBuild goroutine to not panic on nil maps.
func validationOnlyManager() *AgentManager {
	ctx, cancel := context.WithCancel(context.Background())
	_ = cancel // caller can cancel if needed; GC handles cleanup
	return &AgentManager{
		builds:      make(map[string]*Build),
		agents:      make(map[string]*Agent),
		subscribers: make(map[string][]chan *WSMessage),
		ctx:         ctx,
		cancel:      cancel,
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}
}

func TestStartBuild_AcceptsAllValidProviders(t *testing.T) {
	am := validationOnlyManager()

	validProviders := []string{"claude", "gpt4", "gemini", "grok", "ollama"}
	for _, prov := range validProviders {
		t.Run(prov, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{
				"description":      "Build me a fully functional e-commerce platform with payments",
				"role_assignments": map[string]string{"architect": prov},
			})
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/build/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			testRouter(am).ServeHTTP(w, req)

			// Should NOT get a 400 for role validation.
			// It may fail later (500) because CreateBuild needs a full manager,
			// but that's fine -- we just care it passed validation.
			if w.Code == http.StatusBadRequest {
				var respBody map[string]any
				json.Unmarshal(w.Body.Bytes(), &respBody)
				errMsg, _ := respBody["error"].(string)
				if errMsg == "invalid provider" || errMsg == "invalid role category" {
					t.Fatalf("valid provider %q was rejected: %s", prov, w.Body.String())
				}
			}
		})
	}
}

func TestStartBuild_AcceptsAllValidCategories(t *testing.T) {
	am := validationOnlyManager()

	validCategories := []string{"architect", "coder", "tester", "devops"}
	for _, cat := range validCategories {
		t.Run(cat, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{
				"description":      "Build me a fully functional e-commerce platform with payments",
				"role_assignments": map[string]string{cat: "claude"},
			})
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/build/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			testRouter(am).ServeHTTP(w, req)

			if w.Code == http.StatusBadRequest {
				var respBody map[string]any
				json.Unmarshal(w.Body.Bytes(), &respBody)
				errMsg, _ := respBody["error"].(string)
				if errMsg == "invalid role category" || errMsg == "invalid provider" {
					t.Fatalf("valid category %q was rejected: %s", cat, w.Body.String())
				}
			}
		})
	}
}

func TestStartBuild_AcceptsMultipleRoleAssignments(t *testing.T) {
	am := validationOnlyManager()

	body, _ := json.Marshal(map[string]any{
		"description": "Build me a fully functional e-commerce platform with payments",
		"role_assignments": map[string]string{
			"architect": "claude",
			"coder":     "gpt4",
			"tester":    "gemini",
			"devops":    "grok",
		},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	// Must not be rejected as 400 for role validation.
	if w.Code == http.StatusBadRequest {
		var respBody map[string]any
		json.Unmarshal(w.Body.Bytes(), &respBody)
		errMsg, _ := respBody["error"].(string)
		if errMsg == "invalid role category" || errMsg == "invalid provider" {
			t.Fatalf("all valid assignments were rejected: %s", w.Body.String())
		}
	}
}

func TestStartBuild_NilRoleAssignmentsPassesValidation(t *testing.T) {
	am := validationOnlyManager()

	body, _ := json.Marshal(map[string]any{
		"description": "Build me a fully functional e-commerce platform with payments",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	// Should not fail with role-related 400 errors.
	if w.Code == http.StatusBadRequest {
		var respBody map[string]any
		json.Unmarshal(w.Body.Bytes(), &respBody)
		errMsg, _ := respBody["error"].(string)
		if errMsg == "invalid role category" || errMsg == "invalid provider" {
			t.Fatalf("nil role_assignments should not trigger role validation: %s", w.Body.String())
		}
	}
}

func TestStartBuild_RejectsFirstInvalidInMixedAssignments(t *testing.T) {
	// When one valid and one invalid category are sent together, the handler
	// should still reject with 400.
	am := &AgentManager{
		aiRouter: &stubPreflight{
			configured:    true,
			allProviders:  []ai.AIProvider{ai.ProviderClaude},
			userProviders: []ai.AIProvider{ai.ProviderClaude},
		},
	}

	body, _ := json.Marshal(map[string]any{
		"description": "Build me a fully functional e-commerce platform with payments",
		"role_assignments": map[string]string{
			"architect": "claude",
			"hacker":    "gpt4", // invalid category
		},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/build/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter(am).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for mixed valid/invalid assignments, got %d: %s", w.Code, w.Body.String())
	}
}
