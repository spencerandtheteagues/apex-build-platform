//go:build !cgo

package agents

import "fmt"

func parseGoFileWithTreeSitter(source []byte) (*parsedASTFile, error) {
	_ = source
	return nil, fmt.Errorf("tree-sitter Go parsing unavailable without cgo")
}
