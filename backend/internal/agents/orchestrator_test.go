package agents

import (
	"strings"
	"testing"
	"time"
)

func TestCompleteOrchestrationUsesManagerReadinessGate(t *testing.T) {
	t.Parallel()

	am := &AgentManager{
		subscribers: make(map[string][]chan *WSMessage),
	}
	hub := NewWSHub(am)
	orchestrator := &BuildOrchestrator{
		manager:     am,
		hub:         hub,
		activeBuild: make(map[string]*OrchestrationState),
	}

	build := &Build{
		ID:                        "build-validation-gate",
		Mode:                      ModeFull,
		Status:                    BuildReviewing,
		CreatedAt:                 time.Now().Add(-2 * time.Minute),
		ReadinessRecoveryAttempts: 1,
		Tasks: []*Task{
			{
				ID:     "task-backend",
				Type:   TaskGenerateAPI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "server/package.json",
							Content: `{
  "name": "bad-backend",
  "scripts": {
    "build": "tsc"
  },
  "dependencies": {
    "express": "^4.21.1"
  }
}`,
							Language: "json",
						},
						{
							Path:     "server/src/index.ts",
							Content:  "import express from 'express';\nconst app = express();\napp.listen(3000)\n",
							Language: "typescript",
						},
					},
				},
			},
		},
	}

	state := &OrchestrationState{
		BuildID:   build.ID,
		StartTime: time.Now().Add(-45 * time.Second),
	}

	orchestrator.completeOrchestration(build, state)

	build.mu.RLock()
	defer build.mu.RUnlock()

	if !build.PhasedPipelineComplete {
		t.Fatalf("expected completeOrchestration to mark phased pipeline complete")
	}
	if build.Status == BuildCompleted {
		t.Fatalf("expected orchestrator completion to respect readiness validation and avoid completed status")
	}
	if build.Status != BuildFailed {
		t.Fatalf("expected build to fail validation, got %s", build.Status)
	}
	if build.CompletedAt == nil {
		t.Fatalf("expected completed_at to be set on failed finalization")
	}
	if !strings.Contains(strings.ToLower(build.Error), "validation failed") {
		t.Fatalf("expected validation failure error, got %q", build.Error)
	}
}
