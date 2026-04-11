//go:build cgo

package agents

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	_jsii "github.com/smacker/go-tree-sitter/javascript"
	_tstsx "github.com/smacker/go-tree-sitter/typescript/tsx"
	_tsts "github.com/smacker/go-tree-sitter/typescript/typescript"
)

var astIdentifierRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

func parseTypeScriptLikeWithTreeSitter(language string, source []byte) (*parsedASTFile, error) {
	lang, err := treeSitterLanguageForCode(language)
	if err != nil {
		return nil, err
	}

	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	parser.SetOperationLimit(4_000_000)

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()

	tree, err := parser.ParseCtx(ctx, nil, source)
	if err != nil {
		return nil, err
	}
	if tree == nil || tree.RootNode() == nil {
		return nil, fmt.Errorf("tree-sitter produced empty syntax tree")
	}

	out := &parsedASTFile{Language: language}
	walkAST(tree.RootNode(), func(node *sitter.Node) {
		if node == nil {
			return
		}
		nodeType := strings.TrimSpace(node.Type())
		if nodeType == "" {
			return
		}
		switch nodeType {
		case "import_statement":
			value := strings.TrimSpace(node.Content(source))
			if value != "" {
				out.Imports = append(out.Imports, value)
			}
		case "function_declaration",
			"class_declaration",
			"interface_declaration",
			"type_alias_declaration",
			"enum_declaration",
			"lexical_declaration",
			"variable_declaration":
			decl := declarationFromNode(node, source)
			if decl.Name != "" && decl.Body != "" {
				out.Declarations = append(out.Declarations, decl)
			}
		}
	})

	return out, nil
}

func treeSitterLanguageForCode(language string) (*sitter.Language, error) {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "typescript":
		return _tsts.GetLanguage(), nil
	case "tsx":
		return _tstsx.GetLanguage(), nil
	case "javascript", "jsx":
		return _jsii.GetLanguage(), nil
	default:
		return nil, fmt.Errorf("unsupported tree-sitter language: %s", language)
	}
}

func walkAST(node *sitter.Node, visit func(*sitter.Node)) {
	if node == nil {
		return
	}
	visit(node)
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		walkAST(child, visit)
	}
}

func declarationFromNode(node *sitter.Node, source []byte) parsedSymbolDeclaration {
	kind := strings.TrimSpace(node.Type())
	body := strings.TrimSpace(node.Content(source))
	name := declarationName(node, source)
	if name == "" {
		name = "anonymous"
	}

	start := int(node.StartPoint().Row) + 1
	end := int(node.EndPoint().Row) + 1
	signature := declarationSignature(kind, body)

	exported := declarationIsExported(node)
	if !exported && strings.HasPrefix(strings.TrimSpace(body), "export ") {
		exported = true
	}

	return parsedSymbolDeclaration{
		Name:      name,
		Kind:      kind,
		StartLine: start,
		EndLine:   end,
		Exported:  exported,
		Signature: signature,
		Body:      body,
	}
}

func declarationName(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	if byField := node.ChildByFieldName("name"); byField != nil {
		name := strings.TrimSpace(byField.Content(source))
		if name != "" {
			return cleanDeclarationName(name)
		}
	}

	nodeType := node.Type()
	if nodeType == "lexical_declaration" || nodeType == "variable_declaration" {
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil || child.Type() != "variable_declarator" {
				continue
			}
			if nameNode := child.ChildByFieldName("name"); nameNode != nil {
				name := strings.TrimSpace(nameNode.Content(source))
				if name != "" {
					return cleanDeclarationName(name)
				}
			}
		}
	}

	content := strings.TrimSpace(node.Content(source))
	return cleanDeclarationName(astIdentifierRe.FindString(content))
}

func declarationSignature(kind, body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	lines := strings.Split(body, "\n")
	first := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		first = trimmed
		break
	}
	if first == "" {
		return ""
	}
	if idx := strings.Index(first, "{"); idx >= 0 {
		first = strings.TrimSpace(first[:idx])
	}
	if idx := strings.Index(first, "=>"); idx >= 0 && !strings.HasSuffix(first, "=>") {
		first = strings.TrimSpace(first[:idx+2])
	}
	if !strings.HasSuffix(first, ";") && (kind == "lexical_declaration" || kind == "variable_declaration" || kind == "type_alias_declaration") {
		first += ";"
	}
	if len(first) > 200 {
		first = strings.TrimSpace(first[:200]) + " ..."
	}
	return first
}

func declarationIsExported(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	parent := node.Parent()
	for parent != nil {
		typ := parent.Type()
		if typ == "export_statement" {
			return true
		}
		if typ == "program" {
			break
		}
		parent = parent.Parent()
	}
	return false
}

func cleanDeclarationName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if trimmed == "default" {
		return ""
	}
	return strings.Trim(trimmed, "'\"`")
}
