package preview

import "testing"

func TestParseVisionRepairResultExtractsEmbeddedJSONObject(t *testing.T) {
	raw := "Here is the analysis:\n{\"summary\":\"Layout is mostly correct\",\"issues\":[\"Hero copy is low contrast\"],\"repair_hints\":[\"Increase contrast on the hero headline\"]}\nThanks."

	result := parseVisionRepairResult(raw)
	if result == nil {
		t.Fatal("expected parsed vision result")
	}
	if result.Summary != "Layout is mostly correct" {
		t.Fatalf("unexpected summary: %#v", result)
	}
	if len(result.Issues) != 1 || result.Issues[0] != "Hero copy is low contrast" {
		t.Fatalf("unexpected issues: %#v", result.Issues)
	}
	if len(result.RepairHints) != 1 || result.RepairHints[0] != "Increase contrast on the hero headline" {
		t.Fatalf("unexpected repair hints: %#v", result.RepairHints)
	}
}

func TestParseVisionRepairResultFallsBackToRawSummary(t *testing.T) {
	raw := "The page is visibly blank and likely missing critical styling."

	result := parseVisionRepairResult(raw)
	if result == nil {
		t.Fatal("expected fallback result")
	}
	if result.Summary != raw {
		t.Fatalf("expected raw summary fallback, got %#v", result)
	}
}
