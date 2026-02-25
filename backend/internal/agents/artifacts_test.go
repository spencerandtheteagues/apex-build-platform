package agents

import "testing"

func TestGeneratedFilesToArtifactFilesSanitizesAndHashes(t *testing.T) {
	files := []GeneratedFile{
		{Path: "/src/App.tsx", Content: "export const App = () => null;\n", Language: "typescript"},
		{Path: "  ", Content: "ignored"},
	}

	out := generatedFilesToArtifactFiles(files)
	if len(out) != 1 {
		t.Fatalf("expected 1 artifact file, got %d", len(out))
	}
	if out[0].Path != "src/App.tsx" {
		t.Fatalf("path = %q, want %q", out[0].Path, "src/App.tsx")
	}
	if out[0].SHA256 == "" {
		t.Fatalf("expected sha256 to be set")
	}
	if out[0].Size == 0 {
		t.Fatalf("expected non-zero size")
	}
}

func TestBuildArtifactManifestRevisionDeterministicAcrossOrder(t *testing.T) {
	filesA := []GeneratedFile{
		{Path: "b.txt", Content: "b"},
		{Path: "a.txt", Content: "a"},
	}
	filesB := []GeneratedFile{
		{Path: "a.txt", Content: "a"},
		{Path: "b.txt", Content: "b"},
	}

	manifestA := buildArtifactManifest("build-1", "live", "desc", nil, filesA)
	manifestB := buildArtifactManifest("build-1", "live", "desc", nil, filesB)

	if manifestA.Revision == "" || manifestB.Revision == "" {
		t.Fatalf("expected revisions to be populated")
	}
	if manifestA.Revision != manifestB.Revision {
		t.Fatalf("revisions differ for same artifact set: %s vs %s", manifestA.Revision, manifestB.Revision)
	}
	if len(manifestA.Files) != 2 || manifestA.Files[0].Path != "a.txt" || manifestA.Files[1].Path != "b.txt" {
		t.Fatalf("expected canonical sorted file order, got %+v", manifestA.Files)
	}
}
