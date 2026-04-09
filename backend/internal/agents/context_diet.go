package agents

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	contextDietImportRe    = regexp.MustCompile(`^\s*import\s+.+$`)
	contextDietSignatureRe = regexp.MustCompile(`^\s*(export\s+)?(async\s+)?(function|class|interface|type)\s+[A-Za-z0-9_]+|\s*(export\s+)?(const|let|var)\s+[A-Za-z0-9_]+\s*=\s*(async\s*)?(\(|<)|^\s*export\s*\{.+\}`)
)

func buildContextDietSection(path, content string, focusLines []int, contextLines int) string {
	lines := strings.Split(content, "\n")
	imports := extractContextDietImports(lines)
	signatures := extractContextDietSignatures(lines)
	windows := extractContextDietWindows(lines, focusLines, contextLines)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Pruned file context** (`%s`):\n", filepath.ToSlash(path)))
	if len(imports) > 0 {
		sb.WriteString("Imports:\n```typescript\n")
		sb.WriteString(strings.Join(imports, "\n"))
		sb.WriteString("\n```\n")
	}
	if len(signatures) > 0 {
		sb.WriteString("Public signatures:\n```typescript\n")
		sb.WriteString(strings.Join(signatures, "\n"))
		sb.WriteString("\n```\n")
	}
	if len(windows) > 0 {
		sb.WriteString("Focused source windows:\n")
		for _, window := range windows {
			sb.WriteString("```typescript\n")
			sb.WriteString(window)
			sb.WriteString("\n```\n")
		}
	}
	return sb.String()
}

func extractContextDietImports(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, 0, 12)
	for _, line := range lines {
		if contextDietImportRe.MatchString(line) {
			out = append(out, line)
		}
		if len(out) >= 20 {
			break
		}
	}
	return out
}

func extractContextDietSignatures(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, 0, 20)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if contextDietSignatureRe.MatchString(line) {
			out = append(out, trimmed)
		}
		if len(out) >= 24 {
			break
		}
	}
	return out
}

func extractContextDietWindows(lines []string, focusLines []int, contextLines int) []string {
	if len(lines) == 0 || len(focusLines) == 0 {
		return nil
	}
	sorted := append([]int(nil), focusLines...)
	sort.Ints(sorted)

	type window struct {
		start int
		end   int
	}
	windows := make([]window, 0, len(sorted))
	for _, focus := range sorted {
		if focus <= 0 {
			continue
		}
		start := focus - 1 - contextLines
		if start < 0 {
			start = 0
		}
		end := focus - 1 + contextLines
		if end >= len(lines) {
			end = len(lines) - 1
		}
		if len(windows) == 0 {
			windows = append(windows, window{start: start, end: end})
			continue
		}
		last := &windows[len(windows)-1]
		if start <= last.end+2 {
			if end > last.end {
				last.end = end
			}
			continue
		}
		windows = append(windows, window{start: start, end: end})
	}

	rendered := make([]string, 0, len(windows))
	for _, win := range windows {
		var sb strings.Builder
		for idx := win.start; idx <= win.end; idx++ {
			prefix := "  "
			lineNo := idx + 1
			for _, focus := range focusLines {
				if focus == lineNo {
					prefix = "→ "
					break
				}
			}
			sb.WriteString(fmt.Sprintf("%s%4d | %s\n", prefix, lineNo, lines[idx]))
		}
		rendered = append(rendered, strings.TrimRight(sb.String(), "\n"))
	}
	return rendered
}
