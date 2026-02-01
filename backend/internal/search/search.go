// Package search - Advanced Code Search Engine for APEX.BUILD
// Provides full-text search, regex matching, semantic search, and intelligent ranking
package search

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"apex-build/pkg/models"

	"gorm.io/gorm"
)

// escapeLikePattern escapes special characters in LIKE patterns to prevent SQL injection
// via pattern matching. Characters %, _, and \ have special meaning in SQL LIKE clauses.
func escapeLikePattern(input string) string {
	input = strings.ReplaceAll(input, "\\", "\\\\")
	input = strings.ReplaceAll(input, "%", "\\%")
	input = strings.ReplaceAll(input, "_", "\\_")
	return input
}

// SearchEngine handles all code search operations
type SearchEngine struct {
	db    *gorm.DB
	cache *SearchCache
	mu    sync.RWMutex
}

// SearchCache provides fast caching for search results
type SearchCache struct {
	results map[string]*CachedResult
	mu      sync.RWMutex
	ttl     time.Duration
}

// CachedResult stores cached search results with expiry
type CachedResult struct {
	Results   *SearchResults
	CreatedAt time.Time
}

// SearchQuery represents a search request
type SearchQuery struct {
	Query          string   `json:"query" binding:"required"`
	ProjectID      uint     `json:"project_id"`
	FileTypes      []string `json:"file_types,omitempty"`      // e.g., [".go", ".ts", ".js"]
	Paths          []string `json:"paths,omitempty"`           // Filter by paths
	ExcludePaths   []string `json:"exclude_paths,omitempty"`   // Paths to exclude
	CaseSensitive  bool     `json:"case_sensitive,omitempty"`
	WholeWord      bool     `json:"whole_word,omitempty"`
	UseRegex       bool     `json:"use_regex,omitempty"`
	IncludeContent bool     `json:"include_content,omitempty"` // Include matching line content
	ContextLines   int      `json:"context_lines,omitempty"`   // Lines of context around match
	MaxResults     int      `json:"max_results,omitempty"`     // Limit results (default: 100)
	Offset         int      `json:"offset,omitempty"`          // Pagination offset
	SearchType     string   `json:"search_type,omitempty"`     // "content", "filename", "symbol", "all"
}

// SearchResults contains aggregated search results
type SearchResults struct {
	Query        string         `json:"query"`
	TotalMatches int            `json:"total_matches"`
	FileMatches  int            `json:"file_matches"`
	Files        []*FileResult  `json:"files"`
	Suggestions  []string       `json:"suggestions,omitempty"`
	Duration     time.Duration  `json:"duration"`
	Truncated    bool           `json:"truncated"`
	Stats        *SearchStats   `json:"stats"`
}

// FileResult represents matches within a single file
type FileResult struct {
	FileID    uint          `json:"file_id"`
	FileName  string        `json:"file_name"`
	FilePath  string        `json:"file_path"`
	Language  string        `json:"language"`
	Matches   []*LineMatch  `json:"matches"`
	Score     float64       `json:"score"` // Relevance score
}

// LineMatch represents a single match within a file
type LineMatch struct {
	LineNumber    int      `json:"line_number"`
	ColumnStart   int      `json:"column_start"`
	ColumnEnd     int      `json:"column_end"`
	Content       string   `json:"content"`
	ContextBefore []string `json:"context_before,omitempty"`
	ContextAfter  []string `json:"context_after,omitempty"`
	MatchText     string   `json:"match_text"`
}

// SearchStats provides statistics about the search
type SearchStats struct {
	FilesSearched   int                   `json:"files_searched"`
	LinesSearched   int                   `json:"lines_searched"`
	BytesSearched   int64                 `json:"bytes_searched"`
	MatchesByType   map[string]int        `json:"matches_by_type"`
	TopFiles        []string              `json:"top_files"`
}

// SymbolResult represents a code symbol (function, class, variable)
type SymbolResult struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"` // function, class, variable, interface, etc.
	FilePath   string `json:"file_path"`
	LineNumber int    `json:"line_number"`
	Signature  string `json:"signature,omitempty"`
	Container  string `json:"container,omitempty"` // Parent class/module
}

// NewSearchEngine creates a new search engine instance
func NewSearchEngine(db *gorm.DB) *SearchEngine {
	return &SearchEngine{
		db: db,
		cache: &SearchCache{
			results: make(map[string]*CachedResult),
			ttl:     5 * time.Minute,
		},
	}
}

// GetDB returns the underlying GORM database instance for search history operations
func (e *SearchEngine) GetDB() *gorm.DB {
	return e.db
}

// Search performs a comprehensive code search
func (e *SearchEngine) Search(ctx context.Context, query *SearchQuery) (*SearchResults, error) {
	start := time.Now()

	// Set defaults
	if query.MaxResults <= 0 || query.MaxResults > 500 {
		query.MaxResults = 100
	}
	if query.ContextLines <= 0 {
		query.ContextLines = 2
	}
	if query.SearchType == "" {
		query.SearchType = "all"
	}

	// Check cache
	cacheKey := e.generateCacheKey(query)
	if cached := e.cache.Get(cacheKey); cached != nil {
		cached.Duration = time.Since(start)
		return cached, nil
	}

	// Fetch files to search
	files, err := e.getFilesToSearch(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch files: %w", err)
	}

	results := &SearchResults{
		Query:   query.Query,
		Files:   make([]*FileResult, 0),
		Stats: &SearchStats{
			MatchesByType: make(map[string]int),
		},
	}

	// Build search pattern
	pattern, err := e.buildSearchPattern(query)
	if err != nil {
		return nil, fmt.Errorf("invalid search pattern: %w", err)
	}

	// Search each file concurrently
	var wg sync.WaitGroup
	resultsChan := make(chan *FileResult, len(files))
	semaphore := make(chan struct{}, 10) // Limit concurrent goroutines

	for _, file := range files {
		wg.Add(1)
		go func(f models.File) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if fileResult := e.searchFile(ctx, query, &f, pattern); fileResult != nil {
				resultsChan <- fileResult
			}

			// Update stats
			e.mu.Lock()
			results.Stats.FilesSearched++
			results.Stats.LinesSearched += strings.Count(f.Content, "\n") + 1
			results.Stats.BytesSearched += f.Size
			e.mu.Unlock()
		}(file)
	}

	// Wait for all searches to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for fileResult := range resultsChan {
		results.Files = append(results.Files, fileResult)
		results.TotalMatches += len(fileResult.Matches)
		results.FileMatches++

		// Track matches by file type
		ext := e.getFileExtension(fileResult.FilePath)
		results.Stats.MatchesByType[ext]++
	}

	// Sort by relevance score
	sort.Slice(results.Files, func(i, j int) bool {
		return results.Files[i].Score > results.Files[j].Score
	})

	// Apply pagination
	if query.Offset > 0 && query.Offset < len(results.Files) {
		results.Files = results.Files[query.Offset:]
	}
	if len(results.Files) > query.MaxResults {
		results.Files = results.Files[:query.MaxResults]
		results.Truncated = true
	}

	// Generate suggestions for empty results
	if results.TotalMatches == 0 {
		results.Suggestions = e.generateSuggestions(query)
	}

	// Extract top files for stats
	for i, f := range results.Files {
		if i >= 5 {
			break
		}
		results.Stats.TopFiles = append(results.Stats.TopFiles, f.FilePath)
	}

	results.Duration = time.Since(start)

	// Cache results
	e.cache.Set(cacheKey, results)

	return results, nil
}

// SearchSymbols searches for code symbols (functions, classes, etc.)
func (e *SearchEngine) SearchSymbols(ctx context.Context, query *SearchQuery) ([]*SymbolResult, error) {
	files, err := e.getFilesToSearch(ctx, query)
	if err != nil {
		return nil, err
	}

	var symbols []*SymbolResult
	var mu sync.Mutex

	// Symbol patterns for different languages
	patterns := map[string][]*regexp.Regexp{
		".go": {
			regexp.MustCompile(`(?m)^func\s+(\w+)\s*\(`),                    // Go functions
			regexp.MustCompile(`(?m)^func\s+\([^)]+\)\s+(\w+)\s*\(`),        // Go methods
			regexp.MustCompile(`(?m)^type\s+(\w+)\s+struct\s*\{`),           // Go structs
			regexp.MustCompile(`(?m)^type\s+(\w+)\s+interface\s*\{`),        // Go interfaces
			regexp.MustCompile(`(?m)^var\s+(\w+)\s+`),                        // Go variables
			regexp.MustCompile(`(?m)^const\s+(\w+)\s+`),                      // Go constants
		},
		".ts": {
			regexp.MustCompile(`(?m)^(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*[(<]`), // TS functions
			regexp.MustCompile(`(?m)^(?:export\s+)?class\s+(\w+)`),                         // TS classes
			regexp.MustCompile(`(?m)^(?:export\s+)?interface\s+(\w+)`),                     // TS interfaces
			regexp.MustCompile(`(?m)^(?:export\s+)?type\s+(\w+)\s*=`),                      // TS types
			regexp.MustCompile(`(?m)^(?:export\s+)?const\s+(\w+)\s*[=:]`),                  // TS constants
		},
		".js": {
			regexp.MustCompile(`(?m)^(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(`), // JS functions
			regexp.MustCompile(`(?m)^(?:export\s+)?class\s+(\w+)`),                       // JS classes
			regexp.MustCompile(`(?m)^(?:export\s+)?const\s+(\w+)\s*=`),                   // JS constants
			regexp.MustCompile(`(?m)(\w+)\s*:\s*(?:async\s+)?function\s*\(`),             // Object methods
		},
		".py": {
			regexp.MustCompile(`(?m)^def\s+(\w+)\s*\(`),                  // Python functions
			regexp.MustCompile(`(?m)^class\s+(\w+)\s*[:(]`),              // Python classes
			regexp.MustCompile(`(?m)^(\w+)\s*=\s*(?:lambda|def)`),        // Python lambdas/funcs
		},
		".tsx": {
			regexp.MustCompile(`(?m)^(?:export\s+)?(?:async\s+)?function\s+(\w+)`),       // TSX functions
			regexp.MustCompile(`(?m)^(?:export\s+)?const\s+(\w+)\s*[=:]\s*(?:\([^)]*\)|[^=])*=>`), // Arrow functions
			regexp.MustCompile(`(?m)^(?:export\s+)?class\s+(\w+)`),                        // TSX classes
		},
	}

	queryLower := strings.ToLower(query.Query)

	for _, file := range files {
		ext := e.getFileExtension(file.Path)
		filePatterns, ok := patterns[ext]
		if !ok {
			continue
		}

		lines := strings.Split(file.Content, "\n")
		for _, pattern := range filePatterns {
			matches := pattern.FindAllStringSubmatchIndex(file.Content, -1)
			for _, match := range matches {
				if len(match) >= 4 {
					name := file.Content[match[2]:match[3]]
					if strings.Contains(strings.ToLower(name), queryLower) {
						// Find line number
						lineNum := 1
						for i, line := range lines {
							if strings.Contains(line, name) {
								lineNum = i + 1
								break
							}
						}

						mu.Lock()
						symbols = append(symbols, &SymbolResult{
							Name:       name,
							Kind:       e.determineSymbolKind(pattern.String()),
							FilePath:   file.Path,
							LineNumber: lineNum,
						})
						mu.Unlock()
					}
				}
			}
		}
	}

	// Sort by name relevance
	sort.Slice(symbols, func(i, j int) bool {
		iScore := e.symbolRelevance(symbols[i].Name, query.Query)
		jScore := e.symbolRelevance(symbols[j].Name, query.Query)
		return iScore > jScore
	})

	if len(symbols) > query.MaxResults {
		symbols = symbols[:query.MaxResults]
	}

	return symbols, nil
}

// SearchAndReplace performs search and replace across files
func (e *SearchEngine) SearchAndReplace(ctx context.Context, projectID uint, search, replace string, options *SearchQuery) (*ReplaceResults, error) {
	options.Query = search
	options.ProjectID = projectID

	searchResults, err := e.Search(ctx, options)
	if err != nil {
		return nil, err
	}

	results := &ReplaceResults{
		SearchQuery:    search,
		ReplaceWith:    replace,
		FilesModified:  0,
		TotalReplaces:  0,
		ModifiedFiles:  make([]*ModifiedFile, 0),
		Preview:        true, // Default to preview mode
	}

	for _, fileResult := range searchResults.Files {
		// Get full file content
		var file models.File
		if err := e.db.First(&file, fileResult.FileID).Error; err != nil {
			continue
		}

		// Perform replacement
		var newContent string
		var replacements int

		if options.UseRegex {
			re, err := regexp.Compile(search)
			if err != nil {
				continue
			}
			newContent = re.ReplaceAllString(file.Content, replace)
			replacements = len(re.FindAllString(file.Content, -1))
		} else if options.CaseSensitive {
			newContent = strings.ReplaceAll(file.Content, search, replace)
			replacements = strings.Count(file.Content, search)
		} else {
			newContent = e.replaceIgnoreCase(file.Content, search, replace)
			replacements = strings.Count(strings.ToLower(file.Content), strings.ToLower(search))
		}

		if replacements > 0 {
			results.ModifiedFiles = append(results.ModifiedFiles, &ModifiedFile{
				FileID:        file.ID,
				FilePath:      file.Path,
				OriginalSize:  len(file.Content),
				ModifiedSize:  len(newContent),
				Replacements:  replacements,
				PreviewDiff:   e.generateDiff(file.Content, newContent),
			})
			results.TotalReplaces += replacements
			results.FilesModified++
		}
	}

	return results, nil
}

// ApplyReplacements applies the search and replace changes to the database
func (e *SearchEngine) ApplyReplacements(ctx context.Context, projectID uint, search, replace string, options *SearchQuery) (*ReplaceResults, error) {
	results, err := e.SearchAndReplace(ctx, projectID, search, replace, options)
	if err != nil {
		return nil, err
	}

	// Apply changes in a transaction
	tx := e.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, modFile := range results.ModifiedFiles {
		var file models.File
		if err := tx.First(&file, modFile.FileID).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		// Apply replacement
		var newContent string
		if options.UseRegex {
			re, _ := regexp.Compile(search)
			newContent = re.ReplaceAllString(file.Content, replace)
		} else if options.CaseSensitive {
			newContent = strings.ReplaceAll(file.Content, search, replace)
		} else {
			newContent = e.replaceIgnoreCase(file.Content, search, replace)
		}

		// Update file
		file.Content = newContent
		file.Size = int64(len(newContent))
		file.Version++
		file.UpdatedAt = time.Now()

		if err := tx.Save(&file).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	results.Preview = false
	return results, nil
}

// ReplaceResults contains results of search and replace operation
type ReplaceResults struct {
	SearchQuery    string          `json:"search_query"`
	ReplaceWith    string          `json:"replace_with"`
	FilesModified  int             `json:"files_modified"`
	TotalReplaces  int             `json:"total_replaces"`
	ModifiedFiles  []*ModifiedFile `json:"modified_files"`
	Preview        bool            `json:"preview"`
}

// ModifiedFile represents a file that was modified by search/replace
type ModifiedFile struct {
	FileID       uint   `json:"file_id"`
	FilePath     string `json:"file_path"`
	OriginalSize int    `json:"original_size"`
	ModifiedSize int    `json:"modified_size"`
	Replacements int    `json:"replacements"`
	PreviewDiff  string `json:"preview_diff"`
}

// Helper methods

func (e *SearchEngine) getFilesToSearch(ctx context.Context, query *SearchQuery) ([]models.File, error) {
	var files []models.File

	db := e.db.WithContext(ctx)

	if query.ProjectID > 0 {
		db = db.Where("project_id = ?", query.ProjectID)
	}

	// Filter by file types
	if len(query.FileTypes) > 0 {
		var conditions []string
		var args []interface{}
		for _, ext := range query.FileTypes {
			conditions = append(conditions, "path LIKE ? ESCAPE '\\'")
			args = append(args, "%"+escapeLikePattern(ext))
		}
		db = db.Where(strings.Join(conditions, " OR "), args...)
	}

	// Filter by paths
	if len(query.Paths) > 0 {
		var conditions []string
		var args []interface{}
		for _, path := range query.Paths {
			conditions = append(conditions, "path LIKE ? ESCAPE '\\'")
			args = append(args, escapeLikePattern(path)+"%")
		}
		db = db.Where(strings.Join(conditions, " OR "), args...)
	}

	// Exclude paths
	if len(query.ExcludePaths) > 0 {
		for _, path := range query.ExcludePaths {
			db = db.Where("path NOT LIKE ? ESCAPE '\\'", escapeLikePattern(path)+"%")
		}
	}

	// Exclude binary files and large files
	db = db.Where("mime_type LIKE ?", "text/%")
	db = db.Where("size < ?", 1024*1024) // Max 1MB files

	if err := db.Find(&files).Error; err != nil {
		return nil, err
	}

	return files, nil
}

func (e *SearchEngine) buildSearchPattern(query *SearchQuery) (*regexp.Regexp, error) {
	pattern := query.Query

	if !query.UseRegex {
		// Escape special regex characters
		pattern = regexp.QuoteMeta(pattern)
	}

	if query.WholeWord {
		pattern = `\b` + pattern + `\b`
	}

	flags := "(?m)" // Multiline mode
	if !query.CaseSensitive {
		flags += "(?i)" // Case insensitive
	}

	return regexp.Compile(flags + pattern)
}

func (e *SearchEngine) searchFile(ctx context.Context, query *SearchQuery, file *models.File, pattern *regexp.Regexp) *FileResult {
	matches := pattern.FindAllStringIndex(file.Content, -1)
	if len(matches) == 0 {
		// Also check filename if searching all
		if query.SearchType == "all" || query.SearchType == "filename" {
			if !pattern.MatchString(file.Name) && !pattern.MatchString(file.Path) {
				return nil
			}
		} else {
			return nil
		}
	}

	result := &FileResult{
		FileID:   file.ID,
		FileName: file.Name,
		FilePath: file.Path,
		Language: e.detectLanguage(file.Path),
		Matches:  make([]*LineMatch, 0),
	}

	lines := strings.Split(file.Content, "\n")

	for _, match := range matches {
		// Find line number
		lineNum := 1
		charCount := 0
		for i, line := range lines {
			if charCount+len(line)+1 > match[0] {
				lineNum = i + 1
				break
			}
			charCount += len(line) + 1
		}

		// Calculate column
		lineStart := charCount
		if lineNum > 1 {
			lineStart = 0
			for i := 0; i < lineNum-1; i++ {
				lineStart += len(lines[i]) + 1
			}
		}
		columnStart := match[0] - lineStart + 1
		columnEnd := match[1] - lineStart + 1

		lineMatch := &LineMatch{
			LineNumber:  lineNum,
			ColumnStart: columnStart,
			ColumnEnd:   columnEnd,
			MatchText:   file.Content[match[0]:match[1]],
		}

		// Get line content
		if lineNum > 0 && lineNum <= len(lines) {
			lineMatch.Content = lines[lineNum-1]
		}

		// Add context lines
		if query.ContextLines > 0 && query.IncludeContent {
			for i := lineNum - query.ContextLines - 1; i < lineNum-1 && i >= 0; i++ {
				lineMatch.ContextBefore = append(lineMatch.ContextBefore, lines[i])
			}
			for i := lineNum; i < lineNum+query.ContextLines && i < len(lines); i++ {
				lineMatch.ContextAfter = append(lineMatch.ContextAfter, lines[i])
			}
		}

		result.Matches = append(result.Matches, lineMatch)
	}

	// Calculate relevance score
	result.Score = e.calculateRelevance(query, result, file)

	return result
}

func (e *SearchEngine) calculateRelevance(query *SearchQuery, result *FileResult, file *models.File) float64 {
	score := float64(len(result.Matches)) * 10.0

	// Boost for exact filename match
	if strings.Contains(strings.ToLower(file.Name), strings.ToLower(query.Query)) {
		score += 50.0
	}

	// Boost for common/important files
	importantFiles := []string{"main", "index", "app", "config", "routes", "api"}
	for _, imp := range importantFiles {
		if strings.Contains(strings.ToLower(file.Name), imp) {
			score += 20.0
			break
		}
	}

	// Penalty for very long files (matches in small files are more significant)
	lineCount := strings.Count(file.Content, "\n") + 1
	if lineCount > 1000 {
		score *= 0.8
	}

	// Boost for matches early in file
	if len(result.Matches) > 0 && result.Matches[0].LineNumber < 50 {
		score += 15.0
	}

	return score
}

func (e *SearchEngine) getFileExtension(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
	}
	return ""
}

func (e *SearchEngine) detectLanguage(path string) string {
	ext := e.getFileExtension(path)
	languages := map[string]string{
		".go":    "go",
		".ts":    "typescript",
		".tsx":   "typescript",
		".js":    "javascript",
		".jsx":   "javascript",
		".py":    "python",
		".rs":    "rust",
		".java":  "java",
		".c":     "c",
		".cpp":   "cpp",
		".h":     "c",
		".css":   "css",
		".html":  "html",
		".json":  "json",
		".yaml":  "yaml",
		".yml":   "yaml",
		".md":    "markdown",
		".sql":   "sql",
		".sh":    "bash",
		".vue":   "vue",
		".svelte": "svelte",
	}
	if lang, ok := languages[ext]; ok {
		return lang
	}
	return "plaintext"
}

func (e *SearchEngine) determineSymbolKind(pattern string) string {
	patternLower := strings.ToLower(pattern)
	switch {
	case strings.Contains(patternLower, "func"):
		return "function"
	case strings.Contains(patternLower, "class"):
		return "class"
	case strings.Contains(patternLower, "interface"):
		return "interface"
	case strings.Contains(patternLower, "type"):
		return "type"
	case strings.Contains(patternLower, "struct"):
		return "struct"
	case strings.Contains(patternLower, "const"):
		return "constant"
	case strings.Contains(patternLower, "var"):
		return "variable"
	default:
		return "symbol"
	}
}

func (e *SearchEngine) symbolRelevance(name, query string) float64 {
	nameLower := strings.ToLower(name)
	queryLower := strings.ToLower(query)

	if name == query {
		return 100.0
	}
	if nameLower == queryLower {
		return 90.0
	}
	if strings.HasPrefix(nameLower, queryLower) {
		return 80.0
	}
	if strings.Contains(nameLower, queryLower) {
		return 60.0
	}
	return 30.0
}

func (e *SearchEngine) generateSuggestions(query *SearchQuery) []string {
	suggestions := make([]string, 0)

	// Suggest common typo corrections
	if len(query.Query) > 3 {
		suggestions = append(suggestions, "Check for typos in your search query")
	}

	// Suggest broadening search
	if query.CaseSensitive {
		suggestions = append(suggestions, "Try disabling case-sensitive search")
	}

	if len(query.FileTypes) > 0 {
		suggestions = append(suggestions, "Try removing file type filters")
	}

	if query.WholeWord {
		suggestions = append(suggestions, "Try disabling whole-word matching")
	}

	return suggestions
}

func (e *SearchEngine) generateCacheKey(query *SearchQuery) string {
	return fmt.Sprintf("%d:%s:%v:%v:%v:%s",
		query.ProjectID, query.Query, query.CaseSensitive,
		query.WholeWord, query.UseRegex, query.SearchType)
}

func (e *SearchEngine) replaceIgnoreCase(content, search, replace string) string {
	re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(search))
	return re.ReplaceAllString(content, replace)
}

func (e *SearchEngine) generateDiff(original, modified string) string {
	// Simple diff - show first few changed lines
	origLines := strings.Split(original, "\n")
	modLines := strings.Split(modified, "\n")

	var diff strings.Builder
	changes := 0
	maxChanges := 5

	for i := 0; i < len(origLines) && i < len(modLines) && changes < maxChanges; i++ {
		if origLines[i] != modLines[i] {
			diff.WriteString(fmt.Sprintf("Line %d:\n", i+1))
			diff.WriteString(fmt.Sprintf("- %s\n", truncateString(origLines[i], 80)))
			diff.WriteString(fmt.Sprintf("+ %s\n", truncateString(modLines[i], 80)))
			changes++
		}
	}

	return diff.String()
}

func truncateString(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen-3]) + "..."
}

// Cache methods

func (c *SearchCache) Get(key string) *SearchResults {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if cached, ok := c.results[key]; ok {
		if time.Since(cached.CreatedAt) < c.ttl {
			return cached.Results
		}
		// Expired - delete
		delete(c.results, key)
	}
	return nil
}

func (c *SearchCache) Set(key string, results *SearchResults) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.results[key] = &CachedResult{
		Results:   results,
		CreatedAt: time.Now(),
	}

	// Clean old entries periodically
	if len(c.results) > 100 {
		c.cleanup()
	}
}

func (c *SearchCache) cleanup() {
	now := time.Now()
	for key, cached := range c.results {
		if now.Sub(cached.CreatedAt) > c.ttl {
			delete(c.results, key)
		}
	}
}
