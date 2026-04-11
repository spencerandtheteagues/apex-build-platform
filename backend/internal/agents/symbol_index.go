package agents

import "strings"

type SymbolLocation struct {
	Path      string
	Name      string
	Kind      string
	StartLine int
	EndLine   int
	Exported  bool
}

type SymbolIndex struct {
	byFile   map[string][]SymbolLocation
	bySymbol map[string][]SymbolLocation
}

func BuildSymbolIndex(files map[string]string) *SymbolIndex {
	index := &SymbolIndex{
		byFile:   make(map[string][]SymbolLocation, len(files)),
		bySymbol: make(map[string][]SymbolLocation, len(files)*2),
	}
	for path, content := range files {
		parsed, err := parseTypeScriptLikeFile(path, content)
		if err != nil || parsed == nil {
			continue
		}
		locations := make([]SymbolLocation, 0, len(parsed.Declarations))
		for _, decl := range parsed.Declarations {
			if strings.TrimSpace(decl.Name) == "" {
				continue
			}
			loc := SymbolLocation{
				Path:      path,
				Name:      decl.Name,
				Kind:      decl.Kind,
				StartLine: decl.StartLine,
				EndLine:   decl.EndLine,
				Exported:  decl.Exported,
			}
			locations = append(locations, loc)
			key := strings.ToLower(strings.TrimSpace(decl.Name))
			index.bySymbol[key] = append(index.bySymbol[key], loc)
		}
		if len(locations) > 0 {
			index.byFile[path] = locations
		}
	}
	return index
}

func (s *SymbolIndex) SymbolsForFile(path string) []SymbolLocation {
	if s == nil || strings.TrimSpace(path) == "" {
		return nil
	}
	locations := s.byFile[path]
	return append([]SymbolLocation(nil), locations...)
}

func (s *SymbolIndex) Lookup(name string) []SymbolLocation {
	if s == nil {
		return nil
	}
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return nil
	}
	locations := s.bySymbol[key]
	return append([]SymbolLocation(nil), locations...)
}
