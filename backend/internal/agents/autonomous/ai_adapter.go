// Package autonomous - AI Adapter
// Bridges the autonomous agent system with the existing AI infrastructure
package autonomous

import (
	"context"
	"fmt"
	"log"
	"strings"

	"apex-build/internal/ai"
)

// AIAdapter implements the AIProvider interface using the existing AI router
type AIAdapter struct {
	router      *ai.AIRouter
	byokManager *ai.BYOKManager
}

// NewAIAdapter creates a new AI adapter
func NewAIAdapter(router *ai.AIRouter, byokManager *ai.BYOKManager) *AIAdapter {
	return &AIAdapter{
		router:      router,
		byokManager: byokManager,
	}
}

// Generate implements AIProvider.Generate
func (a *AIAdapter) Generate(ctx context.Context, prompt string, opts AIOptions) (string, error) {
	log.Printf("AIAdapter: Generate called with prompt length %d", len(prompt))

	// Map options to AI request
	capability := ai.CapabilityCodeGeneration
	if opts.SystemPrompt != "" {
		// Detect capability from system prompt
		capability = detectCapability(opts.SystemPrompt)
	}

	// Build the full prompt
	fullPrompt := prompt
	if opts.SystemPrompt != "" {
		fullPrompt = fmt.Sprintf(`<system>
%s
</system>

<task>
%s
</task>

<output_format>
For code files, use this exact format:
// File: path/to/filename.ext
`+"```"+`language
[complete code here]
`+"```"+`
</output_format>`, opts.SystemPrompt, prompt)
	}

	// Set defaults
	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4000
	}
	if maxTokens > 8000 {
		maxTokens = 8000
	}

	temperature := opts.Temperature
	if temperature <= 0 {
		temperature = 0.7
	}
	if temperature > 1.5 {
		temperature = 1.5
	}

	// Resolve user-specific router (BYOK-aware)
	targetRouter := a.router
	isBYOK := false
	userID := userIDFromContext(ctx)
	projectID := projectIDFromContext(ctx)
	if userID > 0 && a.byokManager != nil {
		userRouter, hasBYOK, err := a.byokManager.GetRouterForUser(userID)
		if err == nil && userRouter != nil {
			targetRouter = userRouter
			isBYOK = hasBYOK
		}
	}

	// Create AI request
	request := &ai.AIRequest{
		Capability:  capability,
		Prompt:      fullPrompt,
		MaxTokens:   maxTokens,
		Temperature: float32(temperature),
	}
	if userID > 0 {
		request.UserID = fmt.Sprintf("%d", userID)
	}

	log.Printf("AIAdapter: Calling router with capability=%s, max_tokens=%d", capability, maxTokens)

	// Reserve credits before making the AI call
	var reservation *ai.CreditReservation
	if a.byokManager != nil && userID > 0 {
		estimateProvider := string(targetRouter.GetDefaultProvider(request.Capability))
		estimatedCost := a.byokManager.EstimateCost(
			estimateProvider,
			request.Model,
			len(fullPrompt),
			maxTokens,
			"",
			isBYOK,
		)
		if estimatedCost > 0 {
			res, err := a.byokManager.ReserveCredits(userID, estimatedCost)
			if err != nil {
				if strings.Contains(err.Error(), "INSUFFICIENT_CREDITS") {
					return "", fmt.Errorf("INSUFFICIENT_CREDITS")
				}
				return "", fmt.Errorf("failed to reserve credits")
			}
			reservation = res
		}
	}

	// Execute request
	response, err := targetRouter.Generate(ctx, request)
	if err != nil {
		if a.byokManager != nil && reservation != nil {
			_ = a.byokManager.FinalizeCredits(reservation, 0)
		}
		log.Printf("AIAdapter: Generation failed: %v", err)
		return "", fmt.Errorf("AI generation failed: %w", err)
	}

	if response == nil || response.Content == "" {
		return "", fmt.Errorf("AI returned empty response")
	}

	// Record BYOK usage if applicable
	if a.byokManager != nil && userID > 0 {
		inputTokens := 0
		outputTokens := 0
		cost := 0.0
		if response.Usage != nil {
			inputTokens = response.Usage.PromptTokens
			outputTokens = response.Usage.CompletionTokens
		}
		modelUsed := ai.GetModelUsed(response, request)
		cost = a.byokManager.BilledCost(string(response.Provider), modelUsed, inputTokens, outputTokens, "", isBYOK)
		if response.Usage != nil {
			response.Usage.Cost = cost
		}
		a.byokManager.RecordUsage(userID, projectID, string(response.Provider), modelUsed, isBYOK,
			inputTokens, outputTokens, cost, string(request.Capability), response.Duration, "success")
		if reservation != nil {
			_ = a.byokManager.FinalizeCredits(reservation, cost)
		}
	}

	log.Printf("AIAdapter: Generation succeeded, response length %d", len(response.Content))
	return response.Content, nil
}

// Analyze implements AIProvider.Analyze
func (a *AIAdapter) Analyze(ctx context.Context, content string, instruction string, opts AIOptions) (string, error) {
	log.Printf("AIAdapter: Analyze called")

	// Build analysis prompt
	prompt := fmt.Sprintf(`%s

Content to analyze:
%s`, instruction, content)

	// Resolve user-specific router (BYOK-aware)
	targetRouter := a.router
	isBYOK := false
	userID := userIDFromContext(ctx)
	projectID := projectIDFromContext(ctx)
	if userID > 0 && a.byokManager != nil {
		userRouter, hasBYOK, err := a.byokManager.GetRouterForUser(userID)
		if err == nil && userRouter != nil {
			targetRouter = userRouter
			isBYOK = hasBYOK
		}
	}

	// Use code review capability for analysis
	request := &ai.AIRequest{
		Capability:  ai.CapabilityCodeReview,
		Prompt:      prompt,
		MaxTokens:   opts.MaxTokens,
		Temperature: float32(opts.Temperature),
	}
	if userID > 0 {
		request.UserID = fmt.Sprintf("%d", userID)
	}

	if request.MaxTokens <= 0 {
		request.MaxTokens = 2000
	}
	if request.Temperature <= 0 {
		request.Temperature = 0.3
	}

	// Reserve credits before analysis call
	var reservation *ai.CreditReservation
	if a.byokManager != nil && userID > 0 {
		estimateProvider := string(targetRouter.GetDefaultProvider(request.Capability))
		estimatedCost := a.byokManager.EstimateCost(
			estimateProvider,
			request.Model,
			len(prompt),
			request.MaxTokens,
			"",
			isBYOK,
		)
		if estimatedCost > 0 {
			res, err := a.byokManager.ReserveCredits(userID, estimatedCost)
			if err != nil {
				if strings.Contains(err.Error(), "INSUFFICIENT_CREDITS") {
					return "", fmt.Errorf("INSUFFICIENT_CREDITS")
				}
				return "", fmt.Errorf("failed to reserve credits")
			}
			reservation = res
		}
	}

	response, err := targetRouter.Generate(ctx, request)
	if err != nil {
		if a.byokManager != nil && reservation != nil {
			_ = a.byokManager.FinalizeCredits(reservation, 0)
		}
		return "", fmt.Errorf("AI analysis failed: %w", err)
	}

	if response == nil {
		return "", fmt.Errorf("AI returned nil response")
	}

	// Record BYOK usage if applicable
	if a.byokManager != nil && userID > 0 {
		inputTokens := 0
		outputTokens := 0
		cost := 0.0
		if response.Usage != nil {
			inputTokens = response.Usage.PromptTokens
			outputTokens = response.Usage.CompletionTokens
		}
		modelUsed := ai.GetModelUsed(response, request)
		cost = a.byokManager.BilledCost(string(response.Provider), modelUsed, inputTokens, outputTokens, "", isBYOK)
		if response.Usage != nil {
			response.Usage.Cost = cost
		}
		a.byokManager.RecordUsage(userID, projectID, string(response.Provider), modelUsed, isBYOK,
			inputTokens, outputTokens, cost, string(request.Capability), response.Duration, "success")
		if reservation != nil {
			_ = a.byokManager.FinalizeCredits(reservation, cost)
		}
	}

	return response.Content, nil
}

// detectCapability determines the best AI capability based on the prompt
func detectCapability(systemPrompt string) ai.AICapability {
	// Simple keyword matching to detect intent
	keywords := map[ai.AICapability][]string{
		ai.CapabilityCodeGeneration: {"generate", "create", "build", "implement", "write code"},
		ai.CapabilityCodeReview:     {"review", "analyze", "check", "validate", "examine"},
		ai.CapabilityDebugging:      {"debug", "fix", "error", "bug", "problem"},
		ai.CapabilityTesting:        {"test", "spec", "coverage", "assertion"},
		ai.CapabilityRefactoring:    {"refactor", "improve", "optimize", "clean"},
		ai.CapabilityDocumentation:  {"document", "readme", "api doc", "comment"},
		ai.CapabilityArchitecture:   {"architect", "design", "structure", "plan"},
	}

	for capability, words := range keywords {
		for _, word := range words {
			if containsIgnoreCase(systemPrompt, word) {
				return capability
			}
		}
	}

	return ai.CapabilityCodeGeneration
}

// containsIgnoreCase checks if s contains substr (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// MockAIProvider is a mock implementation for testing
type MockAIProvider struct {
	GenerateFunc func(ctx context.Context, prompt string, opts AIOptions) (string, error)
	AnalyzeFunc  func(ctx context.Context, content string, instruction string, opts AIOptions) (string, error)
}

// Generate implements AIProvider.Generate for mock
func (m *MockAIProvider) Generate(ctx context.Context, prompt string, opts AIOptions) (string, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, prompt, opts)
	}
	return "// File: src/App.tsx\n```typescript\nconst App = () => <div>Hello World</div>;\nexport default App;\n```", nil
}

// Analyze implements AIProvider.Analyze for mock
func (m *MockAIProvider) Analyze(ctx context.Context, content string, instruction string, opts AIOptions) (string, error) {
	if m.AnalyzeFunc != nil {
		return m.AnalyzeFunc(ctx, content, instruction, opts)
	}
	return `{"issues": [], "suggestions": ["Code looks good"]}`, nil
}
