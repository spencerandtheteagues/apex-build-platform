//go:build !cgo

package agents

import "fmt"

func parseTypeScriptLikeWithTreeSitter(language string, source []byte) (*parsedASTFile, error) {
	_ = language
	_ = source
	return nil, fmt.Errorf("tree-sitter AST parsing unavailable without cgo")
}
