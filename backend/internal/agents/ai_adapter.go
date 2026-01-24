// Package agents - AI Router Adapter
// Connects the agent system to the existing AI service layer
package agents

import (
	"context"
	"fmt"
	"log"
	"strings"

	"apex-build/internal/ai"
)

// AIRouterAdapter adapts the existing AI router to the agent system interface
type AIRouterAdapter struct {
	router *ai.AIRouter
}

// NewAIRouterAdapter creates a new adapter wrapping the existing AI router
func NewAIRouterAdapter(router *ai.AIRouter) *AIRouterAdapter {
	return &AIRouterAdapter{router: router}
}

// Generate executes an AI generation request using the specified provider
func (a *AIRouterAdapter) Generate(ctx context.Context, provider AIProvider, prompt string, opts GenerateOptions) (string, error) {
	log.Printf("AIRouterAdapter.Generate called with provider: %s", provider)

	// Map agent provider to AI router capability
	capability := a.mapProviderToCapability(provider, opts)
	log.Printf("Mapped to capability: %s", capability)

	// Build the full prompt with system prompt
	fullPrompt := prompt
	if opts.SystemPrompt != "" {
		fullPrompt = fmt.Sprintf("[System Instructions]\n%s\n\n[Task]\n%s", opts.SystemPrompt, prompt)
	}

	// Add context messages if provided
	if len(opts.Context) > 0 {
		contextStr := "\n[Previous Context]\n"
		for _, msg := range opts.Context {
			contextStr += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
		}
		fullPrompt = contextStr + "\n" + fullPrompt
	}

	// Map agent provider to AI package provider
	var aiProvider ai.AIProvider
	switch provider {
	case ProviderClaude:
		aiProvider = ai.ProviderClaude
	case ProviderGPT:
		aiProvider = ai.ProviderGPT4
	case ProviderGemini:
		aiProvider = ai.ProviderGemini
	default:
		aiProvider = ai.ProviderClaude
	}
	log.Printf("Mapped agent provider %s to AI provider %s", provider, aiProvider)

	// Create AI request
	request := &ai.AIRequest{
		Capability:  capability,
		Prompt:      fullPrompt,
		MaxTokens:   opts.MaxTokens,
		Temperature: float32(opts.Temperature),
		Provider:    aiProvider,
	}

	log.Printf("Calling AI router.Generate with capability=%s, provider=%s, prompt_length=%d",
		capability, aiProvider, len(fullPrompt))

	// Execute the request using Generate method
	response, err := a.router.Generate(ctx, request)
	if err != nil {
		log.Printf("AI generation failed: %v", err)
		return "", fmt.Errorf("AI generation failed: %w", err)
	}

	log.Printf("AI generation succeeded, response length: %d", len(response.Content))
	return response.Content, nil
}

// mapProviderToCapability determines the best capability based on provider and context
func (a *AIRouterAdapter) mapProviderToCapability(provider AIProvider, opts GenerateOptions) ai.AICapability {
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
	provider AIProvider,
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

	// Simulate streaming by sending chunks
	words := strings.Fields(response)
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
	provider AIProvider,
	description string,
	language string,
	opts GenerateOptions,
) (string, error) {
	prompt := fmt.Sprintf(`Generate complete, production-ready %s code for the following:

%s

Requirements:
- Complete, working implementation with NO placeholders or TODOs
- Include all necessary imports
- Follow best practices and conventions
- Add meaningful error handling
- Include comments for complex logic

Output only the code, no explanations.`, language, description)

	if opts.SystemPrompt == "" {
		opts.SystemPrompt = fmt.Sprintf(`You are an expert %s developer. Generate clean, production-ready code.
Never use placeholders, TODOs, or incomplete implementations.
Always provide fully working code that can be used immediately.`, language)
	}

	return a.Generate(ctx, provider, prompt, opts)
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

	archResponse, err := a.Generate(ctx, ProviderClaude, archPrompt, GenerateOptions{
		MaxTokens: 4000,
		SystemPrompt: `You are a senior software architect. Create detailed, comprehensive architecture plans.
Output valid JSON that can be parsed programmatically.`,
	})
	if err != nil {
		return nil, fmt.Errorf("architecture generation failed: %w", err)
	}

	// Step 2: Generate each file based on the plan
	// This is a simplified version - full implementation would parse the JSON
	// and generate each file individually

	result := &AppGeneration{
		Plan:     archResponse,
		Files:    make([]GeneratedFile, 0),
		TechStack: techStack,
	}

	// Generate main files based on tech stack
	files := a.getFilesToGenerate(techStack)
	for _, fileSpec := range files {
		content, err := a.GenerateCode(ctx, ProviderGPT, fileSpec.Description, fileSpec.Language, GenerateOptions{
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
	Plan      string           `json:"plan"`
	Files     []GeneratedFile  `json:"files"`
	TechStack TechStack        `json:"tech_stack"`
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

	response, err := a.Generate(ctx, ProviderClaude, prompt, GenerateOptions{
		MaxTokens: 2000,
		SystemPrompt: "You are a code analyzer. Output valid JSON only.",
	})
	if err != nil {
		return nil, err
	}

	// Parse the response (simplified)
	result := &ValidationResult{
		Valid:       !strings.Contains(strings.ToLower(response), "\"valid\": false"),
		RawResponse: response,
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
