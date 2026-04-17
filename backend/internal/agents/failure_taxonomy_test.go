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

func TestPreviewAdvisoryPassFingerprintsRecorded(t *testing.T) {
	build := &Build{ID: "build-advisory-fp"}

	warnings := []string{
		"visual:layout overflow on sidebar",
		"interaction:click target too small",
	}
	files := []GeneratedFile{
		{Path: "src/main.tsx", Content: "// entry"},
		{Path: "src/App.tsx", Content: "// app"},
	}
	frontendFiles := frontendFilePathsFromFiles(files)
	recordPreviewAdvisoryPassFingerprints(build, warnings, frontendFiles)

	build.mu.RLock()
	fps := build.SnapshotState.Orchestration.FailureFingerprints
	build.mu.RUnlock()

	if len(fps) < 2 {
		t.Fatalf("expected at least 2 fingerprints, got %d", len(fps))
	}
	classes := map[string]bool{}
	for _, fp := range fps {
		classes[fp.FailureClass] = true
	}
	if !classes["visual_layout"] {
		t.Errorf("expected visual_layout fingerprint, got %v", classes)
	}
	if !classes["interaction_canary"] {
		t.Errorf("expected interaction_canary fingerprint, got %v", classes)
	}
	for _, fp := range fps {
		if !fp.RepairSucceeded {
			t.Errorf("expected RepairSucceeded=true for advisory pass fingerprint, got false for %q", fp.FailureClass)
		}
	}
}

func TestPreviewRepairLaunchAndOutcomeFingerprints(t *testing.T) {
	build := &Build{ID: "build-repair-fp"}

	// Seed a failure taxonomy so recordPreviewRepairOutcome can read LastClass
	recordBuildFailureTaxonomy(&build.SnapshotState, BuildFailureRecord{
		Category: FailureCategoryVisual,
		Class:    "visual_layout",
		Phase:    "preview_verification",
	})

	files := []string{"src/main.tsx"}

	// Record the launch (failure)
	recordPreviewRepairLaunch(build, "visual_layout", files)

	// Record the repair succeeding
	recordPreviewRepairOutcome(build, true, files)

	build.mu.RLock()
	fps := build.SnapshotState.Orchestration.FailureFingerprints
	build.mu.RUnlock()

	if len(fps) < 2 {
		t.Fatalf("expected at least 2 fingerprints, got %d", len(fps))
	}
	var hasFailure, hasSuccess bool
	for _, fp := range fps {
		if fp.FailureClass == "visual_layout" {
			if fp.RepairSucceeded {
				hasSuccess = true
			} else {
				hasFailure = true
			}
		}
	}
	if !hasFailure {
		t.Error("expected a RepairSucceeded=false fingerprint from launch")
	}
	if !hasSuccess {
		t.Error("expected a RepairSucceeded=true fingerprint from outcome")
	}
}

func TestExtractAdvisoryFailureClasses(t *testing.T) {
	cases := []struct {
		warnings []string
		expected []string
	}{
		{[]string{"visual:contrast issue"}, []string{"visual_layout"}},
		{[]string{"interaction:click failed"}, []string{"interaction_canary"}},
		{[]string{"vision:layout overflow"}, []string{"visual_layout"}},
		{
			[]string{"visual:overflow", "interaction:blank after click", "visual:another"},
			[]string{"visual_layout", "interaction_canary"},
		},
		{[]string{"no prefix here"}, nil},
	}
	for _, tc := range cases {
		got := extractAdvisoryFailureClasses(tc.warnings)
		if len(got) != len(tc.expected) {
			t.Errorf("input %v: expected %v, got %v", tc.warnings, tc.expected, got)
			continue
		}
		for i, v := range tc.expected {
			if got[i] != v {
				t.Errorf("input %v: expected[%d]=%q, got %q", tc.warnings, i, v, got[i])
			}
		}
	}
}
