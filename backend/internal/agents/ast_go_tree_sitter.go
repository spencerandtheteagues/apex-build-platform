//go:build cgo

package agents

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	sitter "github.com/smacker/go-tree-sitter"
	_golang "github.com/smacker/go-tree-sitter/golang"
)

func parseGoFileWithTreeSitter(source []byte) (*parsedASTFile, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(_golang.GetLanguage())
	parser.SetOperationLimit(4_000_000)

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()

	tree, err := parser.ParseCtx(ctx, nil, source)
	if err != nil {
		return nil, err
	}
	if tree == nil {
		return nil, fmt.Errorf("tree-sitter Go produced empty syntax tree")
	}

	out := &parsedASTFile{Language: "golang"}
	parseGoTopLevel(tree.RootNode(), source, out)
	return out, nil
}

func parseGoTopLevel(root *sitter.Node, source []byte, out *parsedASTFile) {
	if root == nil {
		return
	}
	for i := 0; i < int(root.ChildCount()); i++ {
		node := root.Child(i)
		if node == nil || !node.IsNamed() {
			continue
		}
		switch node.Type() {
		case "import_declaration":
			extractGoImportDecl(node, source, out)
		case "function_declaration":
			if decl := goFunctionFromNode(node, source); decl.Name != "" {
				out.Declarations = append(out.Declarations, decl)
			}
		case "method_declaration":
			if decl := goMethodFromNode(node, source); decl.Name != "" {
				out.Declarations = append(out.Declarations, decl)
			}
		case "type_declaration":
			for j := 0; j < int(node.ChildCount()); j++ {
				spec := node.Child(j)
				if spec != nil && spec.IsNamed() && spec.Type() == "type_spec" {
					if decl := goTypeSpecDecl(spec, node, source); decl.Name != "" {
						out.Declarations = append(out.Declarations, decl)
					}
				}
			}
		case "const_declaration":
			for j := 0; j < int(node.ChildCount()); j++ {
				spec := node.Child(j)
				if spec != nil && spec.IsNamed() && spec.Type() == "const_spec" {
					if decl := goConstSpecDecl(spec, source); decl.Name != "" {
						out.Declarations = append(out.Declarations, decl)
					}
				}
			}
		case "var_declaration":
			for j := 0; j < int(node.ChildCount()); j++ {
				spec := node.Child(j)
				if spec != nil && spec.IsNamed() && spec.Type() == "var_spec" {
					if decl := goVarSpecDecl(spec, source); decl.Name != "" {
						out.Declarations = append(out.Declarations, decl)
					}
				}
			}
		}
	}
}

func extractGoImportDecl(node *sitter.Node, source []byte, out *parsedASTFile) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil || !child.IsNamed() {
			continue
		}
		switch child.Type() {
		case "import_spec":
			if path := goImportSpecPath(child, source); path != "" {
				out.Imports = append(out.Imports, path)
			}
		case "import_spec_list":
			for j := 0; j < int(child.ChildCount()); j++ {
				spec := child.Child(j)
				if spec != nil && spec.IsNamed() && spec.Type() == "import_spec" {
					if path := goImportSpecPath(spec, source); path != "" {
						out.Imports = append(out.Imports, path)
					}
				}
			}
		}
	}
}

func goImportSpecPath(spec *sitter.Node, source []byte) string {
	pathNode := spec.ChildByFieldName("path")
	if pathNode == nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(pathNode.Content(source)), `"`)
}

func goFunctionFromNode(node *sitter.Node, source []byte) parsedSymbolDeclaration {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return parsedSymbolDeclaration{}
	}
	name := strings.TrimSpace(nameNode.Content(source))
	body := strings.TrimSpace(node.Content(source))
	return parsedSymbolDeclaration{
		Name:      name,
		Kind:      "function_declaration",
		StartLine: int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
		Exported:  goIsExported(name),
		Signature: goSignatureFromBody(body),
		Body:      body,
	}
}

func goMethodFromNode(node *sitter.Node, source []byte) parsedSymbolDeclaration {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return parsedSymbolDeclaration{}
	}
	name := strings.TrimSpace(nameNode.Content(source))
	body := strings.TrimSpace(node.Content(source))
	return parsedSymbolDeclaration{
		Name:      name,
		Kind:      "method_declaration",
		StartLine: int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
		Exported:  goIsExported(name),
		Signature: goSignatureFromBody(body),
		Body:      body,
	}
}

func goTypeSpecDecl(spec *sitter.Node, parent *sitter.Node, source []byte) parsedSymbolDeclaration {
	nameNode := spec.ChildByFieldName("name")
	if nameNode == nil {
		return parsedSymbolDeclaration{}
	}
	name := strings.TrimSpace(nameNode.Content(source))
	body := strings.TrimSpace(parent.Content(source))
	return parsedSymbolDeclaration{
		Name:      name,
		Kind:      "type_declaration",
		StartLine: int(parent.StartPoint().Row) + 1,
		EndLine:   int(parent.EndPoint().Row) + 1,
		Exported:  goIsExported(name),
		Signature: goSignatureFromBody(body),
		Body:      body,
	}
}

func goConstSpecDecl(spec *sitter.Node, source []byte) parsedSymbolDeclaration {
	nameNode := spec.ChildByFieldName("name")
	if nameNode == nil {
		return parsedSymbolDeclaration{}
	}
	name := strings.TrimSpace(nameNode.Content(source))
	specBody := strings.TrimSpace(spec.Content(source))
	sig := "const " + specBody
	if len(sig) > 200 {
		sig = strings.TrimSpace(sig[:200]) + " ..."
	}
	return parsedSymbolDeclaration{
		Name:      name,
		Kind:      "const_declaration",
		StartLine: int(spec.StartPoint().Row) + 1,
		EndLine:   int(spec.EndPoint().Row) + 1,
		Exported:  goIsExported(name),
		Signature: sig,
		Body:      sig,
	}
}

func goVarSpecDecl(spec *sitter.Node, source []byte) parsedSymbolDeclaration {
	nameNode := spec.ChildByFieldName("name")
	if nameNode == nil {
		return parsedSymbolDeclaration{}
	}
	name := strings.TrimSpace(nameNode.Content(source))
	specBody := strings.TrimSpace(spec.Content(source))
	sig := "var " + specBody
	if len(sig) > 200 {
		sig = strings.TrimSpace(sig[:200]) + " ..."
	}
	return parsedSymbolDeclaration{
		Name:      name,
		Kind:      "var_declaration",
		StartLine: int(spec.StartPoint().Row) + 1,
		EndLine:   int(spec.EndPoint().Row) + 1,
		Exported:  goIsExported(name),
		Signature: sig,
		Body:      sig,
	}
}

func goIsExported(name string) bool {
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}

func goSignatureFromBody(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}
	if idx := strings.Index(trimmed, "{"); idx >= 0 {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}
	if len(trimmed) > 200 {
		trimmed = strings.TrimSpace(trimmed[:200]) + " ..."
	}
	return trimmed
}
