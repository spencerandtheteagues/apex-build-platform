package preview

import (
	"strings"
	"testing"
)

func TestNormalizeVisionSeverity(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		issues   []string
		expected string
	}{
		{"explicit critical", "critical", nil, "critical"},
		{"explicit advisory", "advisory", nil, "advisory"},
		{"explicit clean", "clean", nil, "clean"},
		{"case insensitive", "CRITICAL", nil, "critical"},
		{"unknown falls through to infer", "excellent", []string{"blank screen detected"}, "critical"},
		{"empty raw + blank screen issue", "", []string{"The app shows a blank screen"}, "critical"},
		{"empty raw + dark-on-dark", "", []string{"dark-on-dark text contrast failure"}, "critical"},
		{"empty raw + unstyled", "", []string{"No styling applied, browser defaults only"}, "critical"},
		{"empty raw + no css", "", []string{"no css loaded, raw HTML visible"}, "critical"},
		{"empty raw + invisible text", "", []string{"invisible text due to contrast"}, "critical"},
		{"empty raw + advisory issues", "", []string{"spacing could be improved"}, "advisory"},
		{"empty raw + no issues", "", nil, "clean"},
		{"empty raw + empty issues", "", []string{}, "clean"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeVisionSeverity(tt.raw, tt.issues)
			if got != tt.expected {
				t.Errorf("normalizeVisionSeverity(%q, %v) = %q, want %q", tt.raw, tt.issues, got, tt.expected)
			}
		})
	}
}

func TestIsVisionCriticalText(t *testing.T) {
	criticalCases := []string{
		"blank screen", "white screen", "empty screen", "blank page",
		"app shows a completely blank screen",
		"invisible text due to color contrast", "unreadable text",
		"dark-on-dark contrast failure", "light-on-light text",
		"zero contrast between text and background",
		"no styling applied", "no css loaded", "missing css stylesheets",
		"unstyled raw html", "browser defaults only, no tailwind",
		"nothing rendered on the page", "no content visible",
		"completely blank", "completely empty", "completely white",
	}
	for _, text := range criticalCases {
		t.Run(text, func(t *testing.T) {
			if !isVisionCriticalText(strings.ToLower(text)) {
				t.Errorf("expected %q to be critical", text)
			}
		})
	}

	advisoryCases := []string{
		"spacing could be improved", "layout hierarchy could be clearer",
		"button padding is tight", "form controls need more margin",
		"navigation bar could be more prominent", "font size is a bit small",
	}
	for _, text := range advisoryCases {
		t.Run("advisory:"+text, func(t *testing.T) {
			if isVisionCriticalText(strings.ToLower(text)) {
				t.Errorf("expected %q to NOT be critical", text)
			}
		})
	}
}

func TestVisionRepairResultIsCritical(t *testing.T) {
	tests := []struct {
		result   *VisionRepairResult
		expected bool
	}{
		{nil, false},
		{&VisionRepairResult{Severity: "critical"}, true},
		{&VisionRepairResult{Severity: "advisory"}, false},
		{&VisionRepairResult{Severity: "clean"}, false},
		{&VisionRepairResult{}, false},
	}
	for _, tt := range tests {
		got := tt.result.IsCritical()
		if got != tt.expected {
			t.Errorf("IsCritical() on %+v = %v, want %v", tt.result, got, tt.expected)
		}
	}
}

func TestParseVisionRepairResultSeverity(t *testing.T) {
	raw := `{"summary":"blank screen","severity":"critical","issues":["App renders blank"],"repair_hints":["Fix Tailwind"]}`
	result := parseVisionRepairResult(raw)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Severity != "critical" {
		t.Errorf("Severity = %q, want %q", result.Severity, "critical")
	}
}

func TestParseVisionRepairResultInfersSeverity(t *testing.T) {
	raw := `{"summary":"bad","issues":["blank screen detected"],"repair_hints":["Fix CSS"]}`
	result := parseVisionRepairResult(raw)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Severity != "critical" {
		t.Errorf("inferred Severity = %q, want %q", result.Severity, "critical")
	}
}

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
