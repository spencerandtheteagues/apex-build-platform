// APEX.BUILD AI Inline Completions Service
// Ghostwriter-equivalent AI code completion with multi-provider support

package completions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"apex-build/internal/ai"
	"apex-build/internal/pricing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CompletionRequest represents a request for code completion
type CompletionRequest struct {
	ProjectID   uint              `json:"project_id"`
	FileID      uint              `json:"file_id"`
	FilePath    string            `json:"file_path"`
	Language    string            `json:"language"`
	Prefix      string            `json:"prefix"` // Code before cursor
	Suffix      string            `json:"suffix"` // Code after cursor
	Line        int               `json:"line"`
	Column      int               `json:"column"`
	TriggerKind TriggerKind       `json:"trigger_kind"`
	Context     CompletionContext `json:"context,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	StopTokens  []string          `json:"stop_tokens,omitempty"`
}

// TriggerKind indicates what triggered the completion
type TriggerKind string

const (
	TriggerInvoked    TriggerKind = "invoked"      // User explicitly requested
	TriggerCharacter  TriggerKind = "trigger_char" // Triggered by character
	TriggerAutomatic  TriggerKind = "automatic"    // Automatic idle trigger
	TriggerIncomplete TriggerKind = "incomplete"   // Re-trigger for incomplete
)

// CompletionContext provides additional context for completions
type CompletionContext struct {
	FileImports    []string          `json:"file_imports,omitempty"`
	FileSymbols    []string          `json:"file_symbols,omitempty"`
	ProjectSymbols []string          `json:"project_symbols,omitempty"`
	RecentEdits    []RecentEdit      `json:"recent_edits,omitempty"`
	RelatedFiles   []RelatedFile     `json:"related_files,omitempty"`
	Framework      string            `json:"framework,omitempty"`
	Dependencies   map[string]string `json:"dependencies,omitempty"`
}

// RecentEdit represents a recent edit in the file
type RecentEdit struct {
	Line      int    `json:"line"`
	OldText   string `json:"old_text"`
	NewText   string `json:"new_text"`
	Timestamp int64  `json:"timestamp"`
}

// RelatedFile provides context from related files
type RelatedFile struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Snippet  string `json:"snippet"` // Relevant code snippet
}

// CompletionResponse contains the completion result
type CompletionResponse struct {
	ID             string           `json:"id"`
	Completions    []CompletionItem `json:"completions"`
	Provider       string           `json:"provider"`
	Model          string           `json:"model"`
	ProcessingTime int64            `json:"processing_time_ms"`
	CachedHit      bool             `json:"cached_hit"`
	Usage          *CompletionUsage `json:"usage,omitempty"`
}

// CompletionItem represents a single completion suggestion
type CompletionItem struct {
	ID              string           `json:"id"`
	Text            string           `json:"text"`         // The completion text
	DisplayText     string           `json:"display_text"` // Text to display in UI
	InsertText      string           `json:"insert_text"`  // Text to insert (may differ)
	Kind            CompletionKind   `json:"kind"`
	Detail          string           `json:"detail,omitempty"`
	Documentation   string           `json:"documentation,omitempty"`
	SortText        string           `json:"sort_text,omitempty"`
	FilterText      string           `json:"filter_text,omitempty"`
	Confidence      float64          `json:"confidence"` // 0-1 confidence score
	Range           *CompletionRange `json:"range,omitempty"`
	AdditionalEdits []TextEdit       `json:"additional_edits,omitempty"`
}

// CompletionKind categorizes the completion
type CompletionKind string

const (
	KindText          CompletionKind = "text"
	KindMethod        CompletionKind = "method"
	KindFunction      CompletionKind = "function"
	KindConstructor   CompletionKind = "constructor"
	KindField         CompletionKind = "field"
	KindVariable      CompletionKind = "variable"
	KindClass         CompletionKind = "class"
	KindInterface     CompletionKind = "interface"
	KindModule        CompletionKind = "module"
	KindProperty      CompletionKind = "property"
	KindUnit          CompletionKind = "unit"
	KindValue         CompletionKind = "value"
	KindEnum          CompletionKind = "enum"
	KindKeyword       CompletionKind = "keyword"
	KindSnippet       CompletionKind = "snippet"
	KindColor         CompletionKind = "color"
	KindFile          CompletionKind = "file"
	KindReference     CompletionKind = "reference"
	KindCustomColor   CompletionKind = "customcolor"
	KindFolder        CompletionKind = "folder"
	KindTypeParameter CompletionKind = "type_parameter"
)

// CompletionRange specifies where to insert the completion
type CompletionRange struct {
	StartLine   int `json:"start_line"`
	StartColumn int `json:"start_column"`
	EndLine     int `json:"end_line"`
	EndColumn   int `json:"end_column"`
}

// TextEdit represents an additional text edit
type TextEdit struct {
	Range   CompletionRange `json:"range"`
	NewText string          `json:"new_text"`
}

// CompletionUsage tracks token usage
type CompletionUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCost    float64 `json:"estimated_cost"`
}

// CompletionCache stores cached completions
type CompletionCache struct {
	Hash      string `gorm:"primarykey;type:varchar(64)"`
	Response  string `gorm:"type:text"`
	Provider  string `gorm:"type:varchar(50)"`
	HitCount  int    `gorm:"default:0"`
	CreatedAt time.Time
	ExpiresAt time.Time
}

// CompletionService handles AI code completions
type CompletionService struct {
	db           *gorm.DB
	aiRouter     *ai.AIRouter
	byokManager  *ai.BYOKManager
	cache        *sync.Map
	cacheEnabled bool
	cacheTTL     time.Duration

	// Rate limiting
	rateLimiter *CompletionRateLimiter

	// Metrics
	metrics *CompletionMetrics
}

// CompletionRateLimiter manages completion rate limits
type CompletionRateLimiter struct {
	mu       sync.RWMutex
	userReqs map[uint]*userRateLimit
	limit    int // requests per window
	window   time.Duration
}

type userRateLimit struct {
	count     int
	windowEnd time.Time
}

// CompletionMetrics tracks completion performance
type CompletionMetrics struct {
	mu               sync.RWMutex
	totalRequests    int64
	totalLatency     int64
	cacheHits        int64
	cacheMisses      int64
	providerLatency  map[string]int64
	providerRequests map[string]int64
}

// NewCompletionService creates a new completion service
func NewCompletionService(db *gorm.DB, aiRouter *ai.AIRouter, byokManager *ai.BYOKManager) *CompletionService {
	svc := &CompletionService{
		db:           db,
		aiRouter:     aiRouter,
		byokManager:  byokManager,
		cache:        &sync.Map{},
		cacheEnabled: true,
		cacheTTL:     5 * time.Minute,
		rateLimiter: &CompletionRateLimiter{
			userReqs: make(map[uint]*userRateLimit),
			limit:    60, // 60 requests per minute
			window:   time.Minute,
		},
		metrics: &CompletionMetrics{
			providerLatency:  make(map[string]int64),
			providerRequests: make(map[string]int64),
		},
	}

	// Run migrations
	db.AutoMigrate(&CompletionCache{})

	// Start cache cleanup worker
	go svc.cacheCleanupWorker()

	return svc
}

// GetCompletions returns AI-powered code completions
func (s *CompletionService) GetCompletions(ctx context.Context, userID uint, req *CompletionRequest) (*CompletionResponse, error) {
	startTime := time.Now()

	// Check rate limit
	if !s.rateLimiter.Allow(userID) {
		return nil, fmt.Errorf("rate limit exceeded, please try again later")
	}

	// Generate cache key
	cacheKey := s.generateCacheKey(req)

	// Check cache
	if s.cacheEnabled {
		if cached, ok := s.cache.Load(cacheKey); ok {
			response := cached.(*CompletionResponse)
			response.CachedHit = true
			response.ProcessingTime = time.Since(startTime).Milliseconds()
			s.metrics.RecordCacheHit()
			return response, nil
		}
	}
	s.metrics.RecordCacheMiss()

	// Build AI prompt
	prompt := s.buildCompletionPrompt(req)

	// Set default parameters
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 150 // Default for completions
	}
	temperature := req.Temperature
	if temperature == 0 {
		temperature = 0.3 // Lower temperature for more deterministic completions
	}

	// Request completion from AI
	aiReq := &ai.AIRequest{
		Capability:  ai.CapabilityCodeCompletion,
		Prompt:      prompt,
		Code:        req.Prefix,
		Language:    req.Language,
		MaxTokens:   maxTokens,
		Temperature: float32(temperature),
		UserID:      fmt.Sprintf("%d", userID),
	}

	targetRouter := s.aiRouter
	isBYOK := false
	if s.byokManager != nil && userID > 0 {
		if userRouter, hasBYOK, err := s.byokManager.GetRouterForUser(userID); err == nil && userRouter != nil {
			targetRouter = userRouter
			isBYOK = hasBYOK
		}
	}

	// Reserve credits before making the AI call
	var reservation *ai.CreditReservation
	if s.byokManager != nil && userID > 0 {
		powerMode := pricing.ModeFast
		estimateProvider := string(targetRouter.GetDefaultProvider(aiReq.Capability))
		estimatedCost := s.byokManager.EstimateCost(
			estimateProvider,
			aiReq.Model,
			len(prompt)+len(req.Prefix)+len(req.Suffix),
			maxTokens,
			powerMode,
			isBYOK,
		)
		if estimatedCost > 0 {
			res, err := s.byokManager.ReserveCredits(userID, estimatedCost)
			if err != nil {
				if strings.Contains(err.Error(), "INSUFFICIENT_CREDITS") {
					return nil, fmt.Errorf("INSUFFICIENT_CREDITS")
				}
				return nil, fmt.Errorf("failed to reserve credits")
			}
			reservation = res
		}
	}

	aiResp, err := targetRouter.Generate(ctx, aiReq)
	if err != nil {
		if s.byokManager != nil && reservation != nil {
			_ = s.byokManager.FinalizeCredits(reservation, 0)
		}
		return nil, fmt.Errorf("AI completion failed: %w", err)
	}

	// Parse completions from response
	completions := s.parseCompletions(aiResp.Content, req)

	// Extract usage info
	var usage *CompletionUsage
	if aiResp.Usage != nil {
		powerMode := pricing.ModeFast
		modelUsed := ai.GetModelUsed(aiResp, aiReq)
		billedCost := aiResp.Usage.Cost
		if s.byokManager != nil {
			billedCost = s.byokManager.BilledCost(string(aiResp.Provider), modelUsed, aiResp.Usage.PromptTokens, aiResp.Usage.CompletionTokens, powerMode, isBYOK)
		}
		usage = &CompletionUsage{
			PromptTokens:     aiResp.Usage.PromptTokens,
			CompletionTokens: aiResp.Usage.CompletionTokens,
			TotalTokens:      aiResp.Usage.TotalTokens,
			EstimatedCost:    billedCost,
		}
		if reservation != nil {
			_ = s.byokManager.FinalizeCredits(reservation, billedCost)
		}
	} else if reservation != nil && s.byokManager != nil {
		_ = s.byokManager.FinalizeCredits(reservation, 0)
	}

	response := &CompletionResponse{
		ID:             uuid.New().String(),
		Completions:    completions,
		Provider:       string(aiResp.Provider),
		Model:          string(aiResp.Provider),
		ProcessingTime: time.Since(startTime).Milliseconds(),
		CachedHit:      false,
		Usage:          usage,
	}

	// Cache response
	if s.cacheEnabled && len(completions) > 0 {
		s.cache.Store(cacheKey, response)
	}

	// Record metrics
	s.metrics.RecordRequest(string(aiResp.Provider), time.Since(startTime).Milliseconds())

	// Record BYOK usage if applicable
	if s.byokManager != nil && userID > 0 {
		var projectID *uint
		if req.ProjectID != 0 {
			projectID = &req.ProjectID
		}
		inputTokens := 0
		outputTokens := 0
		cost := 0.0
		if aiResp.Usage != nil {
			inputTokens = aiResp.Usage.PromptTokens
			outputTokens = aiResp.Usage.CompletionTokens
			modelUsed := ai.GetModelUsed(aiResp, aiReq)
			cost = s.byokManager.BilledCost(string(aiResp.Provider), modelUsed, inputTokens, outputTokens, pricing.ModeFast, isBYOK)
		}
		modelUsed := ai.GetModelUsed(aiResp, aiReq)
		s.byokManager.RecordUsage(userID, projectID, string(aiResp.Provider), modelUsed, isBYOK,
			inputTokens, outputTokens, cost, string(aiReq.Capability), time.Since(startTime), "success")
	}

	return response, nil
}

// buildCompletionPrompt constructs the AI prompt for completions
func (s *CompletionService) buildCompletionPrompt(req *CompletionRequest) string {
	var sb strings.Builder

	// System context
	sb.WriteString("You are an AI code completion assistant. Complete the code naturally.\n")
	sb.WriteString(fmt.Sprintf("Language: %s\n", req.Language))

	if req.Context.Framework != "" {
		sb.WriteString(fmt.Sprintf("Framework: %s\n", req.Context.Framework))
	}

	// Add imports context
	if len(req.Context.FileImports) > 0 {
		sb.WriteString("\nFile imports:\n")
		for _, imp := range req.Context.FileImports[:min(10, len(req.Context.FileImports))] {
			sb.WriteString(fmt.Sprintf("- %s\n", imp))
		}
	}

	// Add related file context
	if len(req.Context.RelatedFiles) > 0 {
		sb.WriteString("\nRelated code context:\n")
		for _, file := range req.Context.RelatedFiles[:min(3, len(req.Context.RelatedFiles))] {
			sb.WriteString(fmt.Sprintf("// From %s:\n%s\n", file.Path, file.Snippet))
		}
	}

	sb.WriteString("\n---\n")
	sb.WriteString("Complete the following code. Only return the completion, nothing else:\n\n")

	// Add code context
	if len(req.Prefix) > 2000 {
		// Trim prefix to last 2000 chars
		sb.WriteString("...\n")
		sb.WriteString(req.Prefix[len(req.Prefix)-2000:])
	} else {
		sb.WriteString(req.Prefix)
	}

	// Mark cursor position
	sb.WriteString("█") // Cursor marker

	// Add suffix context (limited)
	if req.Suffix != "" {
		suffixPreview := req.Suffix
		if len(suffixPreview) > 500 {
			suffixPreview = suffixPreview[:500] + "\n..."
		}
		sb.WriteString(suffixPreview)
	}

	return sb.String()
}

// getStopTokens returns appropriate stop tokens for the language
func (s *CompletionService) getStopTokens(req *CompletionRequest) []string {
	base := []string{"\n\n", "```", "// ", "/* ", "# "}

	switch req.Language {
	case "python":
		return append(base, "\ndef ", "\nclass ", "\nif __name__")
	case "javascript", "typescript":
		return append(base, "\nfunction ", "\nclass ", "\nconst ", "\nexport ")
	case "go":
		return append(base, "\nfunc ", "\ntype ", "\nvar ", "\nconst ")
	case "rust":
		return append(base, "\nfn ", "\nstruct ", "\nimpl ", "\nmod ")
	case "java":
		return append(base, "\npublic ", "\nprivate ", "\nclass ", "\ninterface ")
	default:
		return base
	}
}

// parseCompletions parses AI response into completion items
func (s *CompletionService) parseCompletions(response string, req *CompletionRequest) []CompletionItem {
	// Clean up response
	completion := strings.TrimSpace(response)
	completion = strings.TrimPrefix(completion, "█") // Remove cursor marker if present

	// Remove any markdown code block markers
	completion = strings.TrimPrefix(completion, "```"+req.Language)
	completion = strings.TrimPrefix(completion, "```")
	completion = strings.TrimSuffix(completion, "```")
	completion = strings.TrimSpace(completion)

	if completion == "" {
		return []CompletionItem{}
	}

	// Split into multiple completion suggestions if possible
	var items []CompletionItem

	// Primary completion
	items = append(items, CompletionItem{
		ID:          uuid.New().String(),
		Text:        completion,
		DisplayText: s.truncateDisplay(completion, 80),
		InsertText:  completion,
		Kind:        s.inferCompletionKind(completion),
		Confidence:  0.9,
		SortText:    "0000",
	})

	// If completion contains multiple lines, offer partial completions
	lines := strings.Split(completion, "\n")
	if len(lines) > 1 {
		// Single line completion
		items = append(items, CompletionItem{
			ID:          uuid.New().String(),
			Text:        lines[0],
			DisplayText: s.truncateDisplay(lines[0], 80),
			InsertText:  lines[0],
			Kind:        s.inferCompletionKind(lines[0]),
			Confidence:  0.85,
			SortText:    "0001",
		})
	}

	return items
}

// inferCompletionKind infers the kind of completion
func (s *CompletionService) inferCompletionKind(text string) CompletionKind {
	text = strings.TrimSpace(text)

	if strings.HasPrefix(text, "function") || strings.HasPrefix(text, "def ") || strings.HasPrefix(text, "func ") {
		return KindFunction
	}
	if strings.HasPrefix(text, "class ") {
		return KindClass
	}
	if strings.HasPrefix(text, "interface ") {
		return KindInterface
	}
	if strings.HasPrefix(text, "const ") || strings.HasPrefix(text, "let ") || strings.HasPrefix(text, "var ") {
		return KindVariable
	}
	if strings.Contains(text, "(") && strings.Contains(text, ")") {
		return KindMethod
	}
	if strings.HasPrefix(text, "import ") || strings.HasPrefix(text, "from ") {
		return KindModule
	}

	return KindText
}

// truncateDisplay truncates text for display
func (s *CompletionService) truncateDisplay(text string, maxLen int) string {
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	if len(text) > maxLen {
		return text[:maxLen-3] + "..."
	}
	return text
}

// generateCacheKey creates a cache key from the request
func (s *CompletionService) generateCacheKey(req *CompletionRequest) string {
	// Include relevant fields in cache key
	data := fmt.Sprintf("%d:%s:%s:%d:%d:%s",
		req.FileID,
		req.Language,
		req.Prefix[max(0, len(req.Prefix)-500):], // Last 500 chars of prefix
		req.Line,
		req.Column,
		req.TriggerKind,
	)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// AcceptCompletion records when a completion is accepted
func (s *CompletionService) AcceptCompletion(userID uint, completionID string, accepted bool) {
	// Track acceptance for improving future suggestions
	// This data can be used to fine-tune or improve ranking
}

// GetInlineCompletion returns a single inline completion (for ghost text)
func (s *CompletionService) GetInlineCompletion(ctx context.Context, userID uint, req *CompletionRequest) (*CompletionItem, error) {
	response, err := s.GetCompletions(ctx, userID, req)
	if err != nil {
		return nil, err
	}

	if len(response.Completions) == 0 {
		return nil, nil
	}

	// Return highest confidence completion
	return &response.Completions[0], nil
}

// Rate limiter methods

func (r *CompletionRateLimiter) Allow(userID uint) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	limit, exists := r.userReqs[userID]

	if !exists || now.After(limit.windowEnd) {
		r.userReqs[userID] = &userRateLimit{
			count:     1,
			windowEnd: now.Add(r.window),
		}
		return true
	}

	if limit.count >= r.limit {
		return false
	}

	limit.count++
	return true
}

// Metrics methods

func (m *CompletionMetrics) RecordRequest(provider string, latencyMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalRequests++
	m.totalLatency += latencyMs
	m.providerRequests[provider]++
	m.providerLatency[provider] += latencyMs
}

func (m *CompletionMetrics) RecordCacheHit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheHits++
}

func (m *CompletionMetrics) RecordCacheMiss() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheMisses++
}

func (m *CompletionMetrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	avgLatency := float64(0)
	if m.totalRequests > 0 {
		avgLatency = float64(m.totalLatency) / float64(m.totalRequests)
	}

	cacheHitRate := float64(0)
	totalCacheReqs := m.cacheHits + m.cacheMisses
	if totalCacheReqs > 0 {
		cacheHitRate = float64(m.cacheHits) / float64(totalCacheReqs) * 100
	}

	return map[string]interface{}{
		"total_requests":    m.totalRequests,
		"avg_latency_ms":    avgLatency,
		"cache_hit_rate":    cacheHitRate,
		"provider_requests": m.providerRequests,
	}
}

// Cache cleanup

func (s *CompletionService) cacheCleanupWorker() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Clear old entries from sync.Map
		cutoff := time.Now().Add(-s.cacheTTL)
		s.cache.Range(func(key, value interface{}) bool {
			// In production, would track creation time
			// For now, just clear periodically
			return true
		})

		// Clear database cache
		s.db.Where("expires_at < ?", cutoff).Delete(&CompletionCache{})
	}
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
