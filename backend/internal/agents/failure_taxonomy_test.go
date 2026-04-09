package agents

import "testing"

func TestAppendVerificationReportRecordsFailureTaxonomy(t *testing.T) {
	build := &Build{ID: "build-taxonomy-preview"}

	appendVerificationReport(build, VerificationReport{
		ID:        "report-preview-fail",
		BuildID:   build.ID,
		Phase:     "preview_verification",
		Surface:   SurfaceFrontend,
		Status:    VerificationFailed,
		ChecksRun: []string{"preview_entrypoint", "failure_class:preview_boot"},
		Errors:    []string{"Vite preview server failed to boot"},
	})

	taxonomy := build.SnapshotState.FailureTaxonomy
	if taxonomy == nil {
		t.Fatal("expected failure taxonomy to be recorded")
	}
	if taxonomy.CurrentCategory != FailureCategoryPreviewBoot {
		t.Fatalf("expected current category %q, got %q", FailureCategoryPreviewBoot, taxonomy.CurrentCategory)
	}
	if taxonomy.CurrentClass != "preview_boot" {
		t.Fatalf("expected current class preview_boot, got %q", taxonomy.CurrentClass)
	}
	if taxonomy.CurrentPhase != "preview_verification" {
		t.Fatalf("expected current phase preview_verification, got %q", taxonomy.CurrentPhase)
	}
	if taxonomy.CurrentSurface != SurfaceFrontend {
		t.Fatalf("expected current surface frontend, got %q", taxonomy.CurrentSurface)
	}
	if taxonomy.Counts[string(FailureCategoryPreviewBoot)] != 1 {
		t.Fatalf("expected preview boot count 1, got %+v", taxonomy.Counts)
	}
	if len(taxonomy.Recent) != 1 {
		t.Fatalf("expected 1 recent failure record, got %d", len(taxonomy.Recent))
	}
}

func TestAppendVerificationReportClearsCurrentFailureTaxonomyOnPass(t *testing.T) {
	build := &Build{
		ID: "build-taxonomy-pass",
		SnapshotState: BuildSnapshotState{
			FailureTaxonomy: &BuildFailureTaxonomy{
				CurrentCategory: FailureCategoryPreviewBoot,
				CurrentClass:    "preview_boot",
				CurrentPhase:    "preview_verification",
				CurrentSurface:  SurfaceFrontend,
				LastCategory:    FailureCategoryPreviewBoot,
				LastClass:       "preview_boot",
				LastPhase:       "preview_verification",
				LastSurface:     SurfaceFrontend,
				Counts:          map[string]int{string(FailureCategoryPreviewBoot): 1},
			},
		},
	}

	appendVerificationReport(build, VerificationReport{
		ID:      "report-preview-pass",
		BuildID: build.ID,
		Phase:   "preview_verification",
		Surface: SurfaceFrontend,
		Status:  VerificationPassed,
	})

	taxonomy := build.SnapshotState.FailureTaxonomy
	if taxonomy == nil {
		t.Fatal("expected failure taxonomy to remain present")
	}
	if taxonomy.CurrentCategory != "" || taxonomy.CurrentClass != "" || taxonomy.CurrentPhase != "" {
		t.Fatalf("expected current failure state to clear on passing report, got %+v", taxonomy)
	}
	if taxonomy.LastCategory != FailureCategoryPreviewBoot || taxonomy.LastClass != "preview_boot" {
		t.Fatalf("expected last failure to be preserved, got %+v", taxonomy)
	}
}

func TestUpdateBuildSnapshotStateLockedRecordsPlanningFailureAndDedupesFollowup(t *testing.T) {
	build := &Build{
		ID: "build-taxonomy-planning",
		SnapshotState: BuildSnapshotState{
			CurrentPhase: "planning",
		},
	}

	updated := updateBuildSnapshotStateLocked(build, &WSMessage{
		Type: WSBuildError,
		Data: map[string]any{
			"phase": "planning",
			"error": "planning step hit a timeout while freezing architecture",
		},
	})
	if !updated {
		t.Fatal("expected snapshot state to update")
	}

	taxonomy := build.SnapshotState.FailureTaxonomy
	if taxonomy == nil {
		t.Fatal("expected planning failure taxonomy to be captured")
	}
	if taxonomy.CurrentCategory != FailureCategoryPlanning {
		t.Fatalf("expected planning category, got %q", taxonomy.CurrentCategory)
	}
	if taxonomy.CurrentClass != "timeout" {
		t.Fatalf("expected timeout class, got %q", taxonomy.CurrentClass)
	}
	if taxonomy.Counts[string(FailureCategoryPlanning)] != 1 {
		t.Fatalf("expected one planning failure, got %+v", taxonomy.Counts)
	}

	appendVerificationReport(build, VerificationReport{
		ID:        "report-preview-fail-dedupe",
		BuildID:   build.ID,
		Phase:     "preview_verification",
		Surface:   SurfaceFrontend,
		Status:    VerificationFailed,
		ChecksRun: []string{"failure_class:preview_boot"},
		Errors:    []string{"preview server failed to boot"},
	})
	if build.SnapshotState.FailureTaxonomy.Counts[string(FailureCategoryPreviewBoot)] != 1 {
		t.Fatalf("expected preview failure count 1 after verification report, got %+v", build.SnapshotState.FailureTaxonomy.Counts)
	}

	updateBuildSnapshotStateLocked(build, &WSMessage{
		Type: WSBuildError,
		Data: map[string]any{
			"phase":         "preview_verification",
			"failure_class": "preview_boot",
			"surface":       "frontend",
			"error":         "preview server failed to boot",
		},
	})

	if got := build.SnapshotState.FailureTaxonomy.Counts[string(FailureCategoryPreviewBoot)]; got != 1 {
		t.Fatalf("expected duplicate preview failure to be deduped, got count %d", got)
	}
}
