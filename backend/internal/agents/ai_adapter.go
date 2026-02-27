// Package agents - AI Router Adapter
// Connects the agent system to the existing AI service layer
package agents

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"apex-build/internal/ai"
)

// modelsByPowerMode maps each provider + power mode to the correct model ID
// Updated February 2026 with latest verified models
var modelsByPowerMode = map[ai.AIProvider]map[PowerMode]string{
	ai.ProviderClaude: {
		PowerMax:      "claude-opus-4-6",
		PowerBalanced: "claude-sonnet-4-5-20250929",
		PowerFast:     "claude-haiku-4-5-20251001",
	},
	ai.ProviderGPT4: {
		PowerMax:      "gpt-5.2-codex",
		PowerBalanced: "gpt-5",
		PowerFast:     "gpt-4o-mini",
	},
	ai.ProviderGemini: {
		PowerMax:      "gemini-3-pro-preview",
		PowerBalanced: "gemini-3-flash-preview",
		PowerFast:     "gemini-2.5-flash-lite",
	},
	ai.ProviderOllama: {
		PowerMax:      "deepseek-r1:14b",
		PowerBalanced: "deepseek-r1:7b",
		PowerFast:     "deepseek-r1:7b",
	},
}

// selectModelForPowerMode returns the best model ID for a given provider and power mode
func selectModelForPowerMode(provider ai.AIProvider, mode PowerMode) string {
	if mode == "" {
		mode = PowerFast // Default to cheapest
	}
	if provider == ai.ProviderOllama {
		if model := selectOllamaModelOverride(mode); model != "" {
			return model
		}
	}
	if providerModels, ok := modelsByPowerMode[provider]; ok {
		if model, ok := providerModels[mode]; ok {
			return model
		}
	}
	return "" // Empty string = let the AI client pick its own default
}

func selectOllamaModelOverride(mode PowerMode) string {
	switch mode {
	case PowerMax:
		if v := strings.TrimSpace(os.Getenv("OLLAMA_MODEL_MAX")); v != "" {
			return v
		}
	case PowerBalanced:
		if v := strings.TrimSpace(os.Getenv("OLLAMA_MODEL_BALANCED")); v != "" {
			return v
		}
	default:
		if v := strings.TrimSpace(os.Getenv("OLLAMA_MODEL_FAST")); v != "" {
			return v
		}
	}
	if v := strings.TrimSpace(os.Getenv("OLLAMA_MODEL_DEFAULT")); v != "" {
		return v
	}
	return ""
}

// AIRouterAdapter adapts the existing AI router to the agent system interface
type AIRouterAdapter struct {
	router      *ai.AIRouter
	byokManager *ai.BYOKManager
	startupTime time.Time // Track when adapter was created for grace period
}

// NewAIRouterAdapter creates a new adapter wrapping the existing AI router
func NewAIRouterAdapter(router *ai.AIRouter, byokManager *ai.BYOKManager) *AIRouterAdapter {
	return &AIRouterAdapter{
		router:      router,
		byokManager: byokManager,
		startupTime: time.Now(), // Track startup for grace period
	}
}

// Generate executes an AI generation request using the specified provider
func (a *AIRouterAdapter) Generate(ctx context.Context, provider ai.AIProvider, prompt string, opts GenerateOptions) (*ai.AIResponse, error) {
	log.Printf("AIRouterAdapter.Generate called with provider: %s", provider)

	// Determine which router to use
	targetRouter := a.router
	isBYOK := false
	if opts.UsePlatformKeys {
		log.Printf("Using platform router (forced platform mode) for user %d", opts.UserID)
	} else if opts.UserID > 0 && a.byokManager != nil {
		userRouter, hasBYOK, err := a.byokManager.GetRouterForUser(opts.UserID)
		if err == nil && userRouter != nil {
			targetRouter = userRouter
			isBYOK = hasBYOK
			log.Printf("Using user-specific router for user %d", opts.UserID)
		} else {
			log.Printf("Failed to get user router, falling back to platform router: %v", err)
		}
	} else {
		log.Printf("Using platform router (UserID=%d, BYOKManager=%v)", opts.UserID, a.byokManager != nil)
	}

	// Map agent provider to AI router capability
	capability := a.mapProviderToCapability(provider, opts)
	log.Printf("Mapped to capability: %s", capability)

	// Build the full prompt with system prompt - format optimized for code generation
	var fullPrompt string
	if opts.SystemPrompt != "" {
		// Use a clear structure that AI models understand well
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
	} else {
		fullPrompt = prompt
	}

	// Add context messages if provided
	if len(opts.Context) > 0 {
		contextStr := "\n<previous_context>\n"
		for _, msg := range opts.Context {
			contextStr += fmt.Sprintf("<%s>\n%s\n</%s>\n", msg.Role, msg.Content, msg.Role)
		}
		contextStr += "</previous_context>\n\n"
		fullPrompt = contextStr + fullPrompt
	}

	// Map agent provider to AI package provider
	var aiProvider ai.AIProvider
	switch provider {
	case ai.ProviderClaude:
		aiProvider = ai.ProviderClaude
	case ai.ProviderGPT4:
		aiProvider = ai.ProviderGPT4
	case ai.ProviderGemini:
		aiProvider = ai.ProviderGemini
	case ai.ProviderGrok:
		aiProvider = ai.ProviderGrok
	case ai.ProviderOllama:
		aiProvider = ai.ProviderOllama
	default:
		aiProvider = ai.ProviderOllama // Safer default: prefer local model over broken cloud fallback
	}
	log.Printf("Mapped agent provider %s to AI provider %s", provider, aiProvider)

	// Ensure reasonable token limits
	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4000
	}
	if maxTokens > 24000 {
		maxTokens = 24000
	}
	// Local Ollama inference slows dramatically with large token budgets in the
	// multi-agent pipeline. Cap aggressively so local/dev builds remain usable.
	if aiProvider == ai.ProviderOllama {
		switch opts.PowerMode {
		case PowerMax:
			if maxTokens > 3072 {
				maxTokens = 3072
			}
		case PowerBalanced:
			if maxTokens > 2048 {
				maxTokens = 2048
			}
		default: // PowerFast / unset
			if maxTokens > 1536 {
				maxTokens = 1536
			}
		}
	}

	temperature := opts.Temperature
	if temperature <= 0 {
		temperature = 0.7
	}
	if temperature > 1.5 {
		temperature = 1.5
	}

	// Select model based on power mode
	model := selectModelForPowerMode(aiProvider, opts.PowerMode)

	// Create AI request
	request := &ai.AIRequest{
		Model:       model,
		Capability:  capability,
		Prompt:      fullPrompt,
		MaxTokens:   maxTokens,
		Temperature: float32(temperature),
		Provider:    aiProvider,
	}
	if opts.UserID > 0 {
		request.UserID = fmt.Sprintf("%d", opts.UserID)
	}

	log.Printf("Calling AI router.Generate with capability=%s, provider=%s, model=%s, prompt_length=%d, max_tokens=%d",
		capability, aiProvider, model, len(fullPrompt), maxTokens)

	// Reserve credits before making the AI call
	var reservation *ai.CreditReservation
	if a.byokManager != nil && opts.UserID > 0 {
		estimatedCost := a.byokManager.EstimateCost(string(aiProvider), model, len(fullPrompt), maxTokens, string(opts.PowerMode), isBYOK)
		if estimatedCost > 0 {
			res, err := a.byokManager.ReserveCredits(opts.UserID, estimatedCost)
			if err != nil {
				if strings.Contains(err.Error(), "INSUFFICIENT_CREDITS") {
					return nil, fmt.Errorf("INSUFFICIENT_CREDITS: %s", insufficientCreditsBuildMessage)
				}
				return nil, fmt.Errorf("failed to reserve credits")
			}
			reservation = res
		}
	}

	// Execute the request using Generate method on the selected router.
	// Add a timeout when the caller did not provide one to prevent hung providers
	// from freezing the build pipeline indefinitely.
	genCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var genCancel context.CancelFunc
		genCtx, genCancel = context.WithTimeout(ctx, 90*time.Second)
		defer genCancel()
	}

	response, err := targetRouter.Generate(genCtx, request)
	if err != nil {
		if a.byokManager != nil && reservation != nil {
			_ = a.byokManager.FinalizeCredits(reservation, 0)
		}
		if isInsufficientCreditsErrorMessage(err.Error()) {
			return nil, fmt.Errorf("INSUFFICIENT_CREDITS: %s", insufficientCreditsBuildMessage)
		}
		log.Printf("AI generation failed: %v", err)
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	if response == nil || response.Content == "" {
		log.Printf("AI generation returned empty response")
		return nil, fmt.Errorf("AI generation returned empty response")
	}

	// Record BYOK usage if applicable
	if a.byokManager != nil && opts.UserID > 0 && response != nil {
		inputTokens := 0
		outputTokens := 0
		cost := 0.0
		if response.Usage != nil {
			inputTokens = response.Usage.PromptTokens
			outputTokens = response.Usage.CompletionTokens
		}
		modelUsed := ai.GetModelUsed(response, request)
		cost = a.byokManager.BilledCost(string(response.Provider), modelUsed, inputTokens, outputTokens, string(opts.PowerMode), isBYOK)
		if response.Usage != nil {
			response.Usage.Cost = cost
		}
		a.byokManager.RecordUsage(opts.UserID, nil, string(response.Provider), modelUsed, isBYOK,
			inputTokens, outputTokens, cost, string(request.Capability), response.Duration, "success")
		if reservation != nil {
			_ = a.byokManager.FinalizeCredits(reservation, cost)
		}
	}

	log.Printf("AI generation succeeded, response length: %d", len(response.Content))
	return response, nil
}

// mapProviderToCapability determines the best capability based on provider and context
func (a *AIRouterAdapter) mapProviderToCapability(provider ai.AIProvider, opts GenerateOptions) ai.AICapability {
	// Check system prompt for hints about the task type
	sysPrompt := strings.ToLower(opts.SystemPrompt)

	if strings.Contains(sysPrompt, "plan") || strings.Contains(sysPrompt, "architect") {
		return ai.CapabilityCodeReview // Claude excels at analysis
	}

	if strings.Contains(sysPrompt, "test") {
		return ai.CapabilityTesting
	}

	if strings.Contains(sysPrompt, "debug") || strings.Contains(sysPrompt, "fix") {
		return ai.CapabilityDebugging
	}

	if strings.Contains(sysPrompt, "frontend") || strings.Contains(sysPrompt, "backend") ||
		strings.Contains(sysPrompt, "generate") || strings.Contains(sysPrompt, "code") {
		return ai.CapabilityCodeGeneration
	}

	if strings.Contains(sysPrompt, "review") {
		return ai.CapabilityCodeReview
	}

	if strings.Contains(sysPrompt, "complete") || strings.Contains(sysPrompt, "assist") {
		return ai.CapabilityCodeCompletion
	}

	// Default to code generation
	return ai.CapabilityCodeGeneration
}

// GenerateWithStreaming generates content and streams it back via callback
func (a *AIRouterAdapter) GenerateWithStreaming(
	ctx context.Context,
	provider ai.AIProvider,
	prompt string,
	opts GenerateOptions,
	callback func(chunk string) error,
) error {
	// For now, generate full response then simulate streaming
	// A full implementation would use actual streaming APIs
	response, err := a.Generate(ctx, provider, prompt, opts)
	if err != nil {
		return err
	}
	if response == nil || response.Content == "" {
		return fmt.Errorf("empty response")
	}

	// Simulate streaming by sending chunks
	words := strings.Fields(response.Content)
	chunk := ""
	for i, word := range words {
		chunk += word + " "
		if i > 0 && i%5 == 0 {
			if err := callback(chunk); err != nil {
				return err
			}
			chunk = ""
		}
	}
	if chunk != "" {
		if err := callback(chunk); err != nil {
			return err
		}
	}

	return nil
}

// GenerateCode specifically generates code with proper formatting
func (a *AIRouterAdapter) GenerateCode(
	ctx context.Context,
	provider ai.AIProvider,
	description string,
	language string,
	opts GenerateOptions,
) (string, error) {
	prompt := fmt.Sprintf(`Generate complete, production-ready %s code for the following:

%s

MANDATORY REQUIREMENTS:
1. Complete, fully functional implementation - NO demos, mocks, placeholders, or TODOs
2. Include ALL necessary imports, dependencies, and type definitions
3. Implement proper error handling for all edge cases
4. Follow industry best practices and security standards
5. Add comments only for complex business logic
6. If this requires external API keys or credentials:
   - Use environment variables (process.env.API_KEY pattern)
   - Add clear comments indicating where users must provide their own keys
   - Build ALL other functionality completely

FORBIDDEN:
- Demo data or mock responses
- Placeholder functions that return fake data
- TODO comments
- Incomplete implementations
- "Example" or "sample" code that doesn't work

Output only the code, no explanations.`, language, description)

	if opts.SystemPrompt == "" {
		opts.SystemPrompt = fmt.Sprintf(`You are a senior %s developer building production applications for APEX.BUILD.

ABSOLUTE RULES:
1. NEVER output demo code, mock data, or placeholders under any circumstances
2. Every function you write must be complete and production-ready
3. If you need API keys or credentials from the user, use environment variable patterns and clearly mark them
4. If you can build functionality without external dependencies, build it completely
5. Real implementations only - no stubs, no examples, no "this would be" code

When external resources are needed (API keys, database credentials, third-party services):
- Build as much as possible without them
- Use environment variable patterns for credentials
- Add ONE clear comment indicating what the user needs to provide
- Continue building all other functionality completely`, language)
	}

	response, err := a.Generate(ctx, provider, prompt, opts)
	if err != nil {
		return "", err
	}
	if response == nil || response.Content == "" {
		return "", fmt.Errorf("empty response")
	}
	return response.Content, nil
}

// GenerateFullStackApp generates a complete full-stack application
func (a *AIRouterAdapter) GenerateFullStackApp(
	ctx context.Context,
	description string,
	techStack TechStack,
) (*AppGeneration, error) {
	// Step 1: Generate architecture plan
	archPrompt := fmt.Sprintf(`Create a detailed architecture plan for:
%s

Tech Stack:
- Frontend: %s
- Backend: %s
- Database: %s
- Styling: %s

Output a JSON structure with:
{
  "data_models": [...],
  "api_endpoints": [...],
  "components": [...],
  "files": [...]
}`,
		description,
		techStack.Frontend,
		techStack.Backend,
		techStack.Database,
		techStack.Styling,
	)

	archResponse, err := a.Generate(ctx, ai.ProviderClaude, archPrompt, GenerateOptions{
		MaxTokens: 4000,
		SystemPrompt: `You are a senior software architect. Create detailed, comprehensive architecture plans.
Output valid JSON that can be parsed programmatically.`,
		UsePlatformKeys: true,
	})
	if err != nil {
		return nil, fmt.Errorf("architecture generation failed: %w", err)
	}
	if archResponse == nil || archResponse.Content == "" {
		return nil, fmt.Errorf("architecture generation failed: empty response")
	}

	// Step 2: Generate each file based on the plan
	// This is a simplified version - full implementation would parse the JSON
	// and generate each file individually

	result := &AppGeneration{
		Plan:      archResponse.Content,
		Files:     make([]GeneratedFile, 0),
		TechStack: techStack,
	}

	// Generate main files based on tech stack
	files := a.getFilesToGenerate(techStack)
	for _, fileSpec := range files {
		content, err := a.GenerateCode(ctx, ai.ProviderGPT4, fileSpec.Description, fileSpec.Language, GenerateOptions{
			MaxTokens: 4000,
		})
		if err != nil {
			continue // Log but don't fail entire generation
		}

		result.Files = append(result.Files, GeneratedFile{
			Path:     fileSpec.Path,
			Content:  content,
			Language: fileSpec.Language,
			Size:     int64(len(content)),
			IsNew:    true,
		})
	}

	return result, nil
}

// AppGeneration holds the results of full app generation
type AppGeneration struct {
	Plan      string          `json:"plan"`
	Files     []GeneratedFile `json:"files"`
	TechStack TechStack       `json:"tech_stack"`
}

// FileSpec describes a file to generate
type FileSpec struct {
	Path        string
	Description string
	Language    string
}

// getFilesToGenerate returns the files to generate for a tech stack
func (a *AIRouterAdapter) getFilesToGenerate(stack TechStack) []FileSpec {
	files := make([]FileSpec, 0)

	// Frontend files based on framework
	switch stack.Frontend {
	case "react", "React":
		files = append(files,
			FileSpec{"src/App.tsx", "Main React application component with routing", "typescript"},
			FileSpec{"src/index.tsx", "React application entry point", "typescript"},
			FileSpec{"src/components/Layout.tsx", "Main layout component with navigation", "typescript"},
			FileSpec{"src/hooks/useApi.ts", "Custom hook for API calls", "typescript"},
			FileSpec{"src/types/index.ts", "TypeScript type definitions", "typescript"},
		)
	case "vue", "Vue":
		files = append(files,
			FileSpec{"src/App.vue", "Main Vue application component", "vue"},
			FileSpec{"src/main.ts", "Vue application entry point", "typescript"},
			FileSpec{"src/components/Layout.vue", "Main layout component", "vue"},
		)
	case "next", "Next.js":
		files = append(files,
			FileSpec{"app/page.tsx", "Home page component", "typescript"},
			FileSpec{"app/layout.tsx", "Root layout component", "typescript"},
			FileSpec{"components/Header.tsx", "Header component", "typescript"},
		)
	}

	// Backend files based on framework
	switch stack.Backend {
	case "node", "Node.js", "express", "Express":
		files = append(files,
			FileSpec{"server/index.ts", "Express server entry point with middleware", "typescript"},
			FileSpec{"server/routes/api.ts", "API route definitions", "typescript"},
			FileSpec{"server/middleware/auth.ts", "Authentication middleware", "typescript"},
			FileSpec{"server/controllers/main.ts", "Main controller with CRUD operations", "typescript"},
		)
	case "go", "Go":
		files = append(files,
			FileSpec{"main.go", "Go server entry point", "go"},
			FileSpec{"handlers/handlers.go", "HTTP handlers", "go"},
			FileSpec{"middleware/auth.go", "Authentication middleware", "go"},
		)
	case "python", "Python", "fastapi", "FastAPI":
		files = append(files,
			FileSpec{"main.py", "FastAPI application entry point", "python"},
			FileSpec{"routers/api.py", "API route definitions", "python"},
			FileSpec{"models/schemas.py", "Pydantic models", "python"},
		)
	}

	// Database files
	switch stack.Database {
	case "postgresql", "PostgreSQL", "postgres":
		files = append(files,
			FileSpec{"database/schema.sql", "Database schema with tables", "sql"},
			FileSpec{"database/migrations/001_initial.sql", "Initial migration", "sql"},
		)
	case "mongodb", "MongoDB":
		files = append(files,
			FileSpec{"database/models.ts", "Mongoose models", "typescript"},
		)
	}

	// Config files
	files = append(files,
		FileSpec{"package.json", "Project dependencies and scripts", "json"},
		FileSpec{".env.example", "Environment variables template", "env"},
		FileSpec{"README.md", "Project documentation", "markdown"},
	)

	return files
}

// ValidateCode checks if generated code is syntactically correct
func (a *AIRouterAdapter) ValidateCode(ctx context.Context, code string, language string) (*ValidationResult, error) {
	prompt := fmt.Sprintf(`Analyze this %s code for issues:

%s

Check for:
1. Syntax errors
2. Missing imports
3. Undefined variables
4. Type errors (if applicable)
5. Incomplete implementations (TODOs, placeholders)

Output JSON:
{
  "valid": true/false,
  "issues": [{"line": N, "severity": "error/warning", "message": "..."}],
  "suggestions": ["..."]
}`, language, code)

	response, err := a.Generate(ctx, ai.ProviderClaude, prompt, GenerateOptions{
		MaxTokens:       2000,
		SystemPrompt:    "You are a code analyzer. Output valid JSON only.",
		UsePlatformKeys: true,
	})
	if err != nil {
		return nil, err
	}
	if response == nil || response.Content == "" {
		return nil, fmt.Errorf("empty response")
	}

	// Parse the response (simplified)
	result := &ValidationResult{
		Valid:       !strings.Contains(strings.ToLower(response.Content), "\"valid\": false"),
		RawResponse: response.Content,
	}

	return result, nil
}

// ValidationResult holds code validation results
type ValidationResult struct {
	Valid       bool     `json:"valid"`
	Issues      []Issue  `json:"issues"`
	Suggestions []string `json:"suggestions"`
	RawResponse string   `json:"-"`
}

// Issue represents a code issue
type Issue struct {
	Line     int    `json:"line"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// GetAvailableProvidersForUser returns available providers for a specific user (BYOK aware)
func (a *AIRouterAdapter) GetAvailableProvidersForUser(userID uint) []ai.AIProvider {
	if a.byokManager == nil {
		return a.GetAvailableProviders()
	}

	userRouter, hasBYOK, err := a.byokManager.GetRouterForUser(userID)
	if err != nil || userRouter == nil {
		log.Printf("Failed to get router for user %d: %v", userID, err)
		return a.GetAvailableProviders()
	}

	// Platform-key users should not be filtered aggressively by BYOK health gating.
	// Return platform provider availability (healthy first, degraded fallback included).
	if !hasBYOK {
		return a.GetAvailableProviders()
	}

	allowedBYOKProviders := map[ai.AIProvider]bool{}
	if keys, keyErr := a.byokManager.GetKeys(userID); keyErr != nil {
		log.Printf("Failed to load BYOK keys for user %d: %v", userID, keyErr)
	} else {
		for _, key := range keys {
			if !key.IsActive {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(key.Provider)) {
			case "anthropic", "claude":
				allowedBYOKProviders[ai.ProviderClaude] = true
			case "openai", "gpt4", "gpt-4":
				allowedBYOKProviders[ai.ProviderGPT4] = true
			case "gemini", "google":
				allowedBYOKProviders[ai.ProviderGemini] = true
			case "ollama":
				allowedBYOKProviders[ai.ProviderOllama] = true
			case "grok", "xai", "x.ai":
				allowedBYOKProviders[ai.ProviderGrok] = true
			}
		}
	}
	if len(allowedBYOKProviders) == 0 {
		log.Printf("No active BYOK providers found in key metadata for user %d", userID)
		return nil
	}

	healthStatus := userRouter.GetHealthStatus()
	available := make([]ai.AIProvider, 0)

	// Map AI router providers to agent providers and check health
	providerMappings := map[ai.AIProvider]ai.AIProvider{
		ai.ProviderClaude: ai.ProviderClaude,
		ai.ProviderGPT4:   ai.ProviderGPT4,
		ai.ProviderGemini: ai.ProviderGemini,
		ai.ProviderGrok:   ai.ProviderGrok,
		ai.ProviderOllama: ai.ProviderOllama,
	}

	// Startup grace period: 30 seconds after adapter creation
	const startupGracePeriod = 30 * time.Second
	isInGracePeriod := time.Since(a.startupTime) < startupGracePeriod

	for aiProvider, agentProvider := range providerMappings {
		if !allowedBYOKProviders[aiProvider] {
			continue
		}
		if healthy, exists := healthStatus[aiProvider]; exists {
			// Health status is known
			if healthy {
				available = append(available, agentProvider)
			} else if isInGracePeriod {
				// During grace period, give unhealthy providers a chance (might be startup delay)
				log.Printf("Provider %s unhealthy but in grace period, including anyway", aiProvider)
				available = append(available, agentProvider)
			} else {
				log.Printf("Provider %s marked as unhealthy, skipping", aiProvider)
			}
		} else {
			// Health status unknown - provider may be configured but health check hasn't run
			log.Printf("Provider %s health status unknown", aiProvider)
			if isInGracePeriod {
				// During grace period, assume configured providers are available
				log.Printf("Provider %s health unknown but in grace period, assuming healthy", aiProvider)
				available = append(available, agentProvider)
			}
		}
	}

	log.Printf("Available providers for user %d: %v", userID, available)
	return available
}

// GetAvailableProviders returns a list of healthy, available AI providers (Platform default)
func (a *AIRouterAdapter) GetAvailableProviders() []ai.AIProvider {
	healthStatus := a.router.GetHealthStatus()
	healthyAvailable := make([]ai.AIProvider, 0)
	degradedAvailable := make([]ai.AIProvider, 0)

	// Map AI router providers to agent providers and check health
	providerMappings := map[ai.AIProvider]ai.AIProvider{
		ai.ProviderClaude: ai.ProviderClaude,
		ai.ProviderGPT4:   ai.ProviderGPT4,
		ai.ProviderGemini: ai.ProviderGemini,
		ai.ProviderGrok:   ai.ProviderGrok,
		ai.ProviderOllama: ai.ProviderOllama,
	}

	for aiProvider, agentProvider := range providerMappings {
		if healthy, exists := healthStatus[aiProvider]; exists {
			if healthy {
				healthyAvailable = append(healthyAvailable, agentProvider)
				log.Printf("Provider %s is available and healthy", agentProvider)
			} else {
				// Keep configured-but-unhealthy providers as degraded fallbacks.
				// This prevents one flaky health check from completely removing a provider.
				degradedAvailable = append(degradedAvailable, agentProvider)
				log.Printf("Provider %s is configured but unhealthy (kept as degraded fallback)", agentProvider)
			}
		}
	}

	available := make([]ai.AIProvider, 0, len(healthyAvailable)+len(degradedAvailable))
	available = append(available, healthyAvailable...)
	available = append(available, degradedAvailable...)

	if len(healthyAvailable) == 0 && len(degradedAvailable) > 0 {
		log.Printf("No healthy providers reported; using %d degraded configured provider(s)", len(degradedAvailable))
	}

	log.Printf("Available providers: %v", available)
	return available
}

// HasConfiguredProviders reports whether any platform/BYOK provider client is configured.
func (a *AIRouterAdapter) HasConfiguredProviders() bool {
	if a == nil || a.router == nil {
		return false
	}

	healthStatus := a.router.GetHealthStatus()
	return len(healthStatus) > 0
}
