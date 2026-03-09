package agents

import (
	"strings"
	"testing"
)

func TestChunkedEditorReassemblesEditedChunk(t *testing.T) {
	editor := NewChunkedEditor()

	lines := make([]string, 0, 520)
	for i := 0; i < 520; i++ {
		lines = append(lines, "line "+strings.Repeat("x", 8))
	}
	original := strings.Join(lines, "\n")
	chunks := editor.SplitIntoChunks(original, 200, 20)
	if len(chunks) < 2 {
		t.Fatalf("expected chunked output for large file, got %d chunk(s)", len(chunks))
	}

	edited := strings.Replace(chunks[1].Content, "line xxxxxxxx", "edited line", 1)
	result := editor.ReassembleFromResults(original, chunks, []ChunkEditResult{
		{
			ChunkIndex:  chunks[1].Index,
			EditedText:  edited,
			ChangesMade: true,
		},
	})

	if !strings.Contains(result, "edited line") {
		t.Fatalf("expected reassembled content to contain edited chunk output")
	}
}

func TestContextSelectorKeepsErroredFileInBudget(t *testing.T) {
	selector := NewContextSelectorWithLimits(150, 2)
	allFiles := map[string]string{
		"src/App.tsx":       "export const App = () => <div>app</div>",
		"src/api/client.ts": "import axios from 'axios'\nexport const client = axios.create()",
		"package.json":      `{"name":"demo"}`,
	}

	selected := selector.Select(
		allFiles,
		[]string{"src/api/client.ts:12: Cannot find module 'axios'"},
		[]string{"src/App.tsx", "src/api/client.ts"},
	)

	if _, ok := selected["src/api/client.ts"]; !ok {
		t.Fatalf("expected errored file to be selected, got %v", mapsKeys(selected))
	}
	if len(selected) == 0 || len(selected) > 2 {
		t.Fatalf("expected bounded selection size, got %d", len(selected))
	}
}

func TestErrorAnalyzerParsesFencedJSONResponse(t *testing.T) {
	analyzer := NewErrorAnalyzer(&stubPreflight{}, "")

	plan, err := analyzer.parseResponse("```json\n{\"summary\":\"missing dep\",\"repairs\":[{\"file_path\":\"package.json\",\"problem\":\"missing axios\",\"instruction\":\"add axios\",\"code_fix\":\"\\\"axios\\\": \\\"1.7.0\\\"\"}]}\n```", nil)
	if err != nil {
		t.Fatalf("parseResponse returned error: %v", err)
	}
	if plan.Summary != "missing dep" {
		t.Fatalf("unexpected summary: %q", plan.Summary)
	}
	if len(plan.Repairs) != 1 || plan.Repairs[0].FilePath != "package.json" {
		t.Fatalf("unexpected repairs: %+v", plan.Repairs)
	}
}

func mapsKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
