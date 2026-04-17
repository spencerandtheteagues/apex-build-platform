package agents

import (
	"fmt"
	"path/filepath"
	"strings"
)

type PrunedSymbolContextOptions struct {
	ContextLines     int
	MaxImports       int
	MaxSignatures    int
	MaxTargetSymbols int
}

type SymbolContextDeclaration struct {
	Name      string
	Kind      string
	StartLine int
	EndLine   int
	Exported  bool
	Signature string
	Body      string
}

type PrunedSymbolContext struct {
	Path                string
	Language            string
	Parser              string
	ParseSucceeded      bool
	Imports             []string
	TargetSymbols       []SymbolContextDeclaration
	CollapsedSignatures []string
	MatchedTargets      []string
	FocusedWindows      []string
}

func defaultPrunedSymbolContextOptions() PrunedSymbolContextOptions {
	return PrunedSymbolContextOptions{
		ContextLines:     4,
		MaxImports:       20,
		MaxSignatures:    24,
		MaxTargetSymbols: 4,
	}
}

func BuildPrunedSymbolContext(path, content string, targetSymbols []string, focusLines []int, options PrunedSymbolContextOptions) (PrunedSymbolContext, error) {
	opts := options
	def := defaultPrunedSymbolContextOptions()
	if opts.ContextLines <= 0 {
		opts.ContextLines = def.ContextLines
	}
	if opts.MaxImports <= 0 {
		opts.MaxImports = def.MaxImports
	}
	if opts.MaxSignatures <= 0 {
		opts.MaxSignatures = def.MaxSignatures
	}
	if opts.MaxTargetSymbols <= 0 {
		opts.MaxTargetSymbols = def.MaxTargetSymbols
	}

	parsed, err := parseTypeScriptLikeFile(path, content)
	if err != nil {
		return PrunedSymbolContext{}, err
	}

	result := PrunedSymbolContext{
		Path:           filepath.ToSlash(strings.TrimSpace(path)),
		Language:       parsed.Language,
		Parser:         "tree-sitter",
		ParseSucceeded: true,
	}

	if len(parsed.Imports) > opts.MaxImports {
		result.Imports = append([]string(nil), parsed.Imports[:opts.MaxImports]...)
	} else {
		result.Imports = append([]string(nil), parsed.Imports...)
	}

	targetSet := make(map[string]struct{}, len(targetSymbols))
	for _, symbol := range targetSymbols {
		trimmed := strings.ToLower(strings.TrimSpace(symbol))
		if trimmed != "" {
			targetSet[trimmed] = struct{}{}
		}
	}

	targetDecls := make([]SymbolContextDeclaration, 0, opts.MaxTargetSymbols)
	signatures := make([]string, 0, opts.MaxSignatures)
	matchedTargets := make(map[string]struct{}, len(targetSet))

	for _, decl := range parsed.Declarations {
		isTarget := false
		if _, ok := targetSet[strings.ToLower(decl.Name)]; ok {
			isTarget = true
			matchedTargets[decl.Name] = struct{}{}
		}
		if !isTarget && len(focusLines) > 0 && spanContainsLine(decl.StartLine, decl.EndLine, focusLines) {
			isTarget = true
		}

		if isTarget && len(targetDecls) < opts.MaxTargetSymbols {
			targetDecls = append(targetDecls, SymbolContextDeclaration{
				Name:      decl.Name,
				Kind:      decl.Kind,
				StartLine: decl.StartLine,
				EndLine:   decl.EndLine,
				Exported:  decl.Exported,
				Signature: decl.Signature,
				Body:      decl.Body,
			})
			continue
		}
		if strings.TrimSpace(decl.Signature) == "" {
			continue
		}
		if len(signatures) >= opts.MaxSignatures {
			continue
		}
		signatures = append(signatures, decl.Signature)
	}

	result.TargetSymbols = targetDecls
	result.CollapsedSignatures = dedupeNonEmptySortedStrings(signatures)
	for target := range matchedTargets {
		result.MatchedTargets = append(result.MatchedTargets, target)
	}
	result.MatchedTargets = dedupeNonEmptySortedStrings(result.MatchedTargets)
	result.FocusedWindows = extractContextDietWindows(strings.Split(content, "\n"), focusLines, opts.ContextLines)

	return result, nil
}

func spanContainsLine(start, end int, lines []int) bool {
	if start <= 0 || end < start || len(lines) == 0 {
		return false
	}
	for _, line := range lines {
		if line >= start && line <= end {
			return true
		}
	}
	return false
}

func renderPrunedSymbolContext(path string, ctx PrunedSymbolContext) string {
	fence := codeBlockLanguageForAST(ctx.Language)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Pruned file context** (`%s`):\n", filepath.ToSlash(path)))
	if len(ctx.Imports) > 0 {
		sb.WriteString(fmt.Sprintf("Imports:\n```%s\n", fence))
		sb.WriteString(strings.Join(ctx.Imports, "\n"))
		sb.WriteString("\n```\n")
	}
	if len(ctx.TargetSymbols) > 0 {
		sb.WriteString("Target symbol bodies:\n")
		for _, symbol := range ctx.TargetSymbols {
			sb.WriteString(fmt.Sprintf("```%s\n", fence))
			sb.WriteString(symbol.Body)
			sb.WriteString("\n```\n")
		}
	}
	if len(ctx.CollapsedSignatures) > 0 {
		sb.WriteString(fmt.Sprintf("Public signatures:\n```%s\n", fence))
		sb.WriteString(strings.Join(ctx.CollapsedSignatures, "\n"))
		sb.WriteString("\n```\n")
	}
	if len(ctx.FocusedWindows) > 0 {
		sb.WriteString("Focused source windows:\n")
		for _, window := range ctx.FocusedWindows {
			sb.WriteString(fmt.Sprintf("```%s\n", fence))
			sb.WriteString(window)
			sb.WriteString("\n```\n")
		}
	}
	return sb.String()
}

func codeBlockLanguageForAST(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "golang", "go":
		return "go"
	case "tsx":
		return "tsx"
	case "javascript", "jsx":
		return "javascript"
	default:
		return "typescript"
	}
}
