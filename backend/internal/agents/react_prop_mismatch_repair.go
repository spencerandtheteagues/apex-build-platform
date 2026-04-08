package agents

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type reactPropMismatchRepairTarget struct {
	SourcePath string
	Line       int
}

type jsxComponentUsage struct {
	ComponentName string
	HasClassName  bool
	HasOnClick    bool
	HasChildren   bool
}

var reactPropMismatchErrorRe = regexp.MustCompile(`([A-Za-z0-9_./-]+\.(?:tsx?|jsx?))\((\d+),(\d+)\): error TS2322:`)

func parseReactPropMismatchRepairTargets(errors []string) []reactPropMismatchRepairTarget {
	if len(errors) == 0 {
		return nil
	}

	seen := map[string]bool{}
	targets := make([]reactPropMismatchRepairTarget, 0)
	for _, msg := range errors {
		if !strings.Contains(msg, "TS2322") || !strings.Contains(msg, "is not assignable to type") {
			continue
		}
		for _, match := range reactPropMismatchErrorRe.FindAllStringSubmatch(msg, -1) {
			if len(match) != 4 {
				continue
			}
			sourcePath := sanitizeFilePath(strings.TrimSpace(match[1]))
			line, _ := strconv.Atoi(match[2])
			if sourcePath == "" || line <= 0 {
				continue
			}
			key := strings.ToLower(fmt.Sprintf("%s:%d", sourcePath, line))
			if seen[key] {
				continue
			}
			seen[key] = true
			targets = append(targets, reactPropMismatchRepairTarget{
				SourcePath: sourcePath,
				Line:       line,
			})
		}
	}

	sort.Slice(targets, func(i, j int) bool {
		if targets[i].SourcePath == targets[j].SourcePath {
			return targets[i].Line < targets[j].Line
		}
		return targets[i].SourcePath < targets[j].SourcePath
	})
	return targets
}

func findLikelyJSXComponentUsage(content string, line int) jsxComponentUsage {
	if strings.TrimSpace(content) == "" || line <= 0 {
		return jsxComponentUsage{}
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return jsxComponentUsage{}
	}
	lineIdx := line - 1
	if lineIdx < 0 {
		lineIdx = 0
	}
	if lineIdx >= len(lines) {
		lineIdx = len(lines) - 1
	}

	start := lineIdx - 3
	if start < 0 {
		start = 0
	}
	end := lineIdx + 4
	if end > len(lines) {
		end = len(lines)
	}
	segment := strings.Join(lines[start:end], "\n")
	tagRe := regexp.MustCompile(`(?s)<([A-Z][A-Za-z0-9_]*)\b([^>]*)>`)
	matches := tagRe.FindAllStringSubmatch(segment, -1)
	if len(matches) == 0 {
		return jsxComponentUsage{}
	}

	match := matches[len(matches)-1]
	if len(match) != 3 {
		return jsxComponentUsage{}
	}

	componentName := sanitizeGeneratedIdentifier(match[1])
	if componentName == "" {
		return jsxComponentUsage{}
	}
	openingTag := strings.TrimSpace(match[0])
	attrs := match[2]
	hasChildren := !strings.HasSuffix(openingTag, "/>") && strings.Contains(segment, "</"+componentName+">")
	return jsxComponentUsage{
		ComponentName: componentName,
		HasClassName:  strings.Contains(attrs, "className="),
		HasOnClick:    strings.Contains(attrs, "onClick="),
		HasChildren:   hasChildren,
	}
}

func importClauseReferencesBinding(clause string, bindingName string) bool {
	bindingName = sanitizeGeneratedIdentifier(bindingName)
	clause = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(clause), "type "))
	if clause == "" || bindingName == "" {
		return false
	}

	defaultImport := ""
	namedClause := ""
	switch {
	case strings.Contains(clause, ","):
		parts := strings.SplitN(clause, ",", 2)
		defaultImport = sanitizeGeneratedIdentifier(strings.TrimSpace(parts[0]))
		namedClause = strings.TrimSpace(parts[1])
	case !strings.HasPrefix(clause, "{") && !strings.HasPrefix(clause, "*"):
		defaultImport = sanitizeGeneratedIdentifier(clause)
	default:
		namedClause = clause
	}

	if defaultImport == bindingName {
		return true
	}
	if strings.HasPrefix(namedClause, "{") && strings.Contains(namedClause, "}") {
		inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(namedClause, "{"), "}"))
		for _, part := range strings.Split(inner, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			localName := part
			if strings.Contains(part, " as ") {
				pieces := strings.SplitN(part, " as ", 2)
				localName = strings.TrimSpace(pieces[1])
			}
			if sanitizeGeneratedIdentifier(localName) == bindingName {
				return true
			}
		}
	}
	return false
}

func resolveImportedLocalComponentPath(importerPath, sourceContent, componentName string, existing map[string]bool) string {
	componentName = sanitizeGeneratedIdentifier(componentName)
	if importerPath == "" || strings.TrimSpace(sourceContent) == "" || componentName == "" {
		return ""
	}

	importRe := regexp.MustCompile(`(?m)^\s*import\s+(.+?)\s+from\s+['"]([^'"]+)['"]`)
	for _, match := range importRe.FindAllStringSubmatch(sourceContent, -1) {
		if len(match) != 3 {
			continue
		}
		clause := strings.TrimSpace(match[1])
		specifier := strings.TrimSpace(match[2])
		if !strings.HasPrefix(specifier, ".") && !strings.HasPrefix(specifier, "@/") && !strings.HasPrefix(specifier, "~/") {
			continue
		}
		if !importClauseReferencesBinding(clause, componentName) {
			continue
		}
		for _, candidate := range localImportResolutionCandidates(importerPath, specifier) {
			sanitized := sanitizeFilePath(candidate)
			if sanitized != "" && existing[strings.ToLower(sanitized)] {
				return sanitized
			}
		}
	}

	return ""
}

func detectComponentPropTypeName(content, componentName string) string {
	componentName = sanitizeGeneratedIdentifier(componentName)
	if strings.TrimSpace(content) == "" || componentName == "" {
		return ""
	}

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)(?:export\s+)?const\s+` + regexp.QuoteMeta(componentName) + `\s*:\s*React\.(?:FC|FunctionComponent)\s*<([A-Za-z_][A-Za-z0-9_]*)>`),
		regexp.MustCompile(`(?s)(?:export\s+)?const\s+` + regexp.QuoteMeta(componentName) + `\s*=\s*\([^)]*:\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)\s*=>`),
		regexp.MustCompile(`(?s)(?:export\s+)?function\s+` + regexp.QuoteMeta(componentName) + `\s*\([^)]*:\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)`),
	}
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(content)
		if len(matches) == 2 {
			return sanitizeGeneratedIdentifier(matches[1])
		}
	}
	return ""
}

func detectPrimaryIntrinsicElementName(content string) string {
	match := regexp.MustCompile(`(?s)<(button|a|input|textarea|select|form|div)\b`).FindStringSubmatch(content)
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func intrinsicAttributeTypeForElement(elementName string) string {
	switch strings.TrimSpace(elementName) {
	case "button":
		return "React.ButtonHTMLAttributes<HTMLButtonElement>"
	case "a":
		return "React.AnchorHTMLAttributes<HTMLAnchorElement>"
	case "input":
		return "React.InputHTMLAttributes<HTMLInputElement>"
	case "textarea":
		return "React.TextareaHTMLAttributes<HTMLTextAreaElement>"
	case "select":
		return "React.SelectHTMLAttributes<HTMLSelectElement>"
	case "form":
		return "React.FormHTMLAttributes<HTMLFormElement>"
	default:
		return "React.HTMLAttributes<HTMLDivElement>"
	}
}

func extendReactPropsTypeDefinition(content, typeName, attrType string) (string, bool) {
	typeName = sanitizeGeneratedIdentifier(typeName)
	attrType = strings.TrimSpace(attrType)
	if strings.TrimSpace(content) == "" || typeName == "" || attrType == "" {
		return content, false
	}

	if strings.Contains(content, attrType) || strings.Contains(content, "PropsWithChildren<"+attrType) {
		return content, false
	}

	typeAliasRe := regexp.MustCompile(`(?s)type\s+` + regexp.QuoteMeta(typeName) + `\s*=\s*\{(.*?)\}`)
	if matches := typeAliasRe.FindStringSubmatch(content); len(matches) == 2 {
		body := strings.TrimSpace(matches[1])
		replacement := fmt.Sprintf("type %s = React.PropsWithChildren<%s & {\n%s\n}>", typeName, attrType, indentMultiline(body, "  "))
		return typeAliasRe.ReplaceAllString(content, replacement), true
	}

	interfaceRe := regexp.MustCompile(`(?s)interface\s+` + regexp.QuoteMeta(typeName) + `(?:\s+extends\s+([^{]+))?\s*\{(.*?)\}`)
	if matches := interfaceRe.FindStringSubmatch(content); len(matches) == 3 {
		existingExt := strings.TrimSpace(matches[1])
		body := strings.TrimSpace(matches[2])
		extendsParts := make([]string, 0, 2)
		if existingExt != "" {
			for _, part := range strings.Split(existingExt, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					extendsParts = append(extendsParts, part)
				}
			}
		}
		alreadyExtends := false
		for _, part := range extendsParts {
			if part == attrType {
				alreadyExtends = true
				break
			}
		}
		if !alreadyExtends {
			extendsParts = append(extendsParts, attrType)
		}
		if !strings.Contains(body, "children?:") {
			if body == "" {
				body = "children?: React.ReactNode"
			} else {
				body = "children?: React.ReactNode\n" + body
			}
		}
		replacement := fmt.Sprintf("interface %s extends %s {\n%s\n}", typeName, strings.Join(extendsParts, ", "), indentMultiline(body, "  "))
		return interfaceRe.ReplaceAllString(content, replacement), true
	}

	return content, false
}

func mergedDestructuredFields(fields string, extras []string) (string, string) {
	parts := strings.Split(fields, ",")
	ordered := make([]string, 0, len(parts)+len(extras))
	seen := map[string]bool{}
	spreadIdent := ""

	keyForField := func(field string) string {
		field = strings.TrimSpace(field)
		if strings.HasPrefix(field, "...") {
			return strings.TrimSpace(strings.TrimPrefix(field, "..."))
		}
		field = strings.SplitN(field, ":", 2)[0]
		field = strings.SplitN(field, "=", 2)[0]
		return strings.TrimSpace(field)
	}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key := keyForField(part)
		if strings.HasPrefix(part, "...") && spreadIdent == "" {
			spreadIdent = key
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		ordered = append(ordered, part)
	}

	for _, extra := range extras {
		extra = strings.TrimSpace(extra)
		if extra == "" {
			continue
		}
		key := keyForField(extra)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if strings.HasPrefix(extra, "...") && spreadIdent == "" {
			spreadIdent = key
		}
		ordered = append(ordered, extra)
	}

	return strings.Join(ordered, ", "), spreadIdent
}

func addComponentPassthroughDestructure(content, componentName, fallbackSpreadIdent string) (string, string, bool) {
	componentName = sanitizeGeneratedIdentifier(componentName)
	fallbackSpreadIdent = sanitizeGeneratedIdentifier(fallbackSpreadIdent)
	if strings.TrimSpace(content) == "" || componentName == "" || fallbackSpreadIdent == "" {
		return content, "", false
	}

	extras := []string{"children", "className", "..." + fallbackSpreadIdent}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?s)((?:export\s+)?const\s+` + regexp.QuoteMeta(componentName) + `\s*:\s*React\.(?:FC|FunctionComponent)\s*<[^>]+>\s*=\s*)\(\{\s*([^}]*)\}\s*\)`),
		regexp.MustCompile(`(?s)((?:export\s+)?const\s+` + regexp.QuoteMeta(componentName) + `\s*=\s*)\(\{\s*([^}]*)\}\s*:\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)`),
		regexp.MustCompile(`(?s)((?:export\s+)?function\s+` + regexp.QuoteMeta(componentName) + `\s*)\(\{\s*([^}]*)\}\s*:\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)`),
	}

	for idx, pattern := range patterns {
		matches := pattern.FindStringSubmatch(content)
		if len(matches) == 0 {
			continue
		}
		mergedFields, spreadIdent := mergedDestructuredFields(matches[2], extras)
		if spreadIdent == "" {
			spreadIdent = fallbackSpreadIdent
		}
		replacement := ""
		switch idx {
		case 0:
			replacement = matches[1] + "({ " + mergedFields + " })"
		case 1, 2:
			replacement = matches[1] + "({ " + mergedFields + " }: " + matches[3] + ")"
		}
		repaired := pattern.ReplaceAllString(content, replacement)
		return repaired, spreadIdent, repaired != content
	}

	return content, fallbackSpreadIdent, false
}

func jsxAttrValueExpression(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return `""`
	}
	if strings.HasPrefix(raw, "{") && strings.HasSuffix(raw, "}") {
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "{"), "}"))
	}
	return raw
}

func addPassthroughSpreadToIntrinsicRoot(content, elementName, spreadIdent string) (string, bool) {
	elementName = strings.TrimSpace(elementName)
	spreadIdent = sanitizeGeneratedIdentifier(spreadIdent)
	if strings.TrimSpace(content) == "" || elementName == "" || spreadIdent == "" {
		return content, false
	}

	openTagRe := regexp.MustCompile(`(?s)<` + regexp.QuoteMeta(elementName) + `\b[^>]*>`)
	openTag := openTagRe.FindString(content)
	if openTag == "" || strings.Contains(openTag, "{..."+spreadIdent+"}") {
		return content, false
	}

	repairedTag := strings.Replace(openTag, "<"+elementName, "<"+elementName+" {..."+spreadIdent+"}", 1)
	classAttrRe := regexp.MustCompile(`\sclassName\s*=\s*(".*?"|'.*?'|\{[^}]*\})`)
	if classMatch := classAttrRe.FindStringSubmatch(repairedTag); len(classMatch) == 2 {
		mergedClass := ` className={[` + jsxAttrValueExpression(classMatch[1]) + `, className].filter(Boolean).join(" ")}`
		repairedTag = classAttrRe.ReplaceAllString(repairedTag, mergedClass)
	}

	if repairedTag == openTag {
		return content, false
	}
	return strings.Replace(content, openTag, repairedTag, 1), true
}

func addChildrenFallbackToIntrinsicRoot(content, elementName string) (string, bool) {
	elementName = strings.TrimSpace(elementName)
	if strings.TrimSpace(content) == "" || elementName == "" {
		return content, false
	}

	rootRe := regexp.MustCompile(`(?s)(<` + regexp.QuoteMeta(elementName) + `\b[^>]*>)(.*?)(</` + regexp.QuoteMeta(elementName) + `>)`)
	matches := rootRe.FindStringSubmatch(content)
	if len(matches) != 4 {
		return content, false
	}

	inner := strings.TrimSpace(matches[2])
	if inner == "" || strings.Contains(inner, "children") {
		return content, false
	}

	var fallbackExpr string
	switch {
	case strings.HasPrefix(inner, "{") && strings.HasSuffix(inner, "}"):
		fallbackExpr = "(" + strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(inner, "{"), "}")) + ")"
	case !strings.Contains(inner, "<"):
		fallbackExpr = strconv.Quote(inner)
	default:
		return content, false
	}

	replacement := matches[1] + "{children ?? " + fallbackExpr + "}" + matches[3]
	repaired := rootRe.ReplaceAllString(content, replacement)
	return repaired, repaired != content
}

func repairReactComponentPassthroughProps(targetPath, content, componentName string) (string, bool) {
	componentName = sanitizeGeneratedIdentifier(componentName)
	if strings.TrimSpace(targetPath) == "" || strings.TrimSpace(content) == "" || componentName == "" {
		return content, false
	}

	propTypeName := detectComponentPropTypeName(content, componentName)
	elementName := detectPrimaryIntrinsicElementName(content)
	if propTypeName == "" || elementName == "" {
		return content, false
	}

	repaired := content
	changed := false

	attrType := intrinsicAttributeTypeForElement(elementName)
	if updated, ok := extendReactPropsTypeDefinition(repaired, propTypeName, attrType); ok {
		repaired = updated
		changed = true
	}
	repaired = ensureReactDefaultImport(repaired)

	if updated, spreadIdent, ok := addComponentPassthroughDestructure(repaired, componentName, elementName+"Props"); ok {
		repaired = updated
		changed = true
		if spreadIdent != "" {
			if rootUpdated, rootChanged := addPassthroughSpreadToIntrinsicRoot(repaired, elementName, spreadIdent); rootChanged {
				repaired = rootUpdated
				changed = true
			}
		}
	}

	if elementName == "button" || elementName == "a" {
		if updated, ok := addChildrenFallbackToIntrinsicRoot(repaired, elementName); ok {
			repaired = updated
			changed = true
		}
	}

	if !changed {
		return content, false
	}
	repaired = normalizeGeneratedFileContent(filepath.ToSlash(targetPath), repaired)
	return repaired, strings.TrimSpace(repaired) != strings.TrimSpace(content)
}

func (am *AgentManager) applyDeterministicReactPropMismatchRepair(build *Build, readinessErrors []string) (*PatchBundle, string) {
	if build == nil || len(readinessErrors) == 0 {
		return nil, ""
	}

	targets := parseReactPropMismatchRepairTargets(readinessErrors)
	if len(targets) == 0 {
		return nil, ""
	}

	files, plan := am.buildGeneratedFilePatchPlan(build)
	if len(files) == 0 {
		return nil, ""
	}

	existing := make(map[string]bool, len(files))
	for _, file := range files {
		path := sanitizeFilePath(file.Path)
		if path != "" {
			existing[strings.ToLower(path)] = true
		}
	}

	applied := make([]string, 0, len(targets))
	seenComponents := map[string]bool{}
	for _, target := range targets {
		sourceContent := plan.content(target.SourcePath)
		if strings.TrimSpace(sourceContent) == "" {
			continue
		}

		usage := findLikelyJSXComponentUsage(sourceContent, target.Line)
		if usage.ComponentName == "" || (!usage.HasClassName && !usage.HasOnClick && !usage.HasChildren) {
			continue
		}

		componentPath := resolveImportedLocalComponentPath(target.SourcePath, sourceContent, usage.ComponentName, existing)
		if componentPath == "" {
			continue
		}
		componentKey := strings.ToLower(componentPath)
		if seenComponents[componentKey] {
			continue
		}

		componentContent := plan.content(componentPath)
		if strings.TrimSpace(componentContent) == "" {
			continue
		}

		repaired, changed := repairReactComponentPassthroughProps(componentPath, componentContent, usage.ComponentName)
		if !changed {
			continue
		}
		if !plan.patchFile(componentPath, repaired, am.detectLanguage(componentPath)) {
			continue
		}
		seenComponents[componentKey] = true
		applied = append(applied, fmt.Sprintf("%s -> %s", target.SourcePath, componentPath))
	}

	if len(applied) == 0 {
		return nil, ""
	}

	summary := "react component prop passthrough repair on " + strings.Join(applied, ", ")
	return am.bundleFromPatchPlan(build.ID, files, plan, "react_prop_mismatch_repair: "+summary), summary
}
