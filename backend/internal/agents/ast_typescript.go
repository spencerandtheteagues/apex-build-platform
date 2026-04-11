package agents

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type parsedASTFile struct {
	Language     string
	Imports      []string
	Declarations []parsedSymbolDeclaration
}

type parsedSymbolDeclaration struct {
	Name      string
	Kind      string
	StartLine int
	EndLine   int
	Exported  bool
	Signature string
	Body      string
}

func parseTypeScriptLikeFile(path, content string) (*parsedASTFile, error) {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(path)))
	language := ""
	switch ext {
	case ".ts":
		language = "typescript"
	case ".tsx":
		language = "tsx"
	case ".js":
		language = "javascript"
	case ".jsx":
		language = "jsx"
	default:
		return nil, fmt.Errorf("unsupported AST context file type: %s", ext)
	}

	parsed, err := parseTypeScriptLikeWithTreeSitter(language, []byte(content))
	if err != nil {
		return nil, err
	}
	if parsed == nil {
		return nil, fmt.Errorf("tree-sitter parser returned nil context")
	}
	parsed.Language = language
	parsed.Imports = dedupeNonEmptySortedStrings(parsed.Imports)
	sort.SliceStable(parsed.Declarations, func(i, j int) bool {
		if parsed.Declarations[i].StartLine == parsed.Declarations[j].StartLine {
			return parsed.Declarations[i].Name < parsed.Declarations[j].Name
		}
		return parsed.Declarations[i].StartLine < parsed.Declarations[j].StartLine
	})
	return parsed, nil
}

func dedupeNonEmptySortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := set[trimmed]; exists {
			continue
		}
		set[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}
