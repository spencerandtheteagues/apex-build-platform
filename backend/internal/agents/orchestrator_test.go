package agents

import (
	"strings"
	"testing"
	"time"
)

func TestCompleteOrchestrationUsesManagerReadinessGateAndNormalization(t *testing.T) {
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
	if build.Status != BuildCompleted {
		t.Fatalf("expected manager finalization to normalize the backend scaffold and complete the build, got %s", build.Status)
	}
	if build.CompletedAt == nil {
		t.Fatalf("expected completed_at to be set on manager finalization")
	}
	if strings.TrimSpace(build.Error) != "" {
		t.Fatalf("expected normalization to clear readiness errors, got %q", build.Error)
	}
}
