// Package autonomous - Task Planner
// Decomposes natural language requirements into structured execution plans
package autonomous

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Planner handles task decomposition and planning
type Planner struct {
	ai AIProvider
}

// NewPlanner creates a new planner
func NewPlanner(ai AIProvider) *Planner {
	return &Planner{ai: ai}
}

// CreatePlan generates an execution plan from a natural language description
func (p *Planner) CreatePlan(ctx context.Context, description string) (*ExecutionPlan, error) {
	log.Printf("Planner: Creating plan for: %s", truncate(description, 100))

	// First, analyze the requirements
	analysis, err := p.analyzeRequirements(ctx, description)
	if err != nil {
		return nil, fmt.Errorf("requirement analysis failed: %w", err)
	}

	// Perform risk assessment
	analysis = p.assessRisks(analysis)
	if analysis.RiskScore > 80 {
		log.Printf("Planner: WARNING - High risk score (%d) detected for build", analysis.RiskScore)
	}

	// Generate the structured plan with build verification gates
	plan, err := p.generatePlan(ctx, description, analysis)
	if err != nil {
		return nil, fmt.Errorf("plan generation failed: %w", err)
	}

	// Add mandatory build verification steps
	plan = p.addBuildVerificationGates(plan, analysis)

	// Optimize step ordering based on dependencies
	plan = p.optimizeStepOrder(plan)

	// Validate and optimize the plan
	if err := p.validatePlan(plan); err != nil {
		return nil, fmt.Errorf("plan validation failed: %w", err)
	}

	log.Printf("Planner: Created plan with %d steps (risk score: %d)", len(plan.Steps), analysis.RiskScore)
	return plan, nil
}

// assessRisks evaluates potential risks in the build
func (p *Planner) assessRisks(analysis *RequirementAnalysis) *RequirementAnalysis {
	risks := make([]RiskAssessment, 0)
	score := 0

	// Complexity risk
	switch analysis.Complexity {
	case "complex":
		score += 25
		risks = append(risks, RiskAssessment{
			Type:        "complexity",
			Severity:    "high",
			Description: "High complexity project may require more iterations",
			Mitigation:  "Break down into smaller, verifiable components",
		})
	case "medium":
		score += 10
	}

	// Feature count risk
	if len(analysis.Features) > 5 {
		score += 15
		risks = append(risks, RiskAssessment{
			Type:        "scope",
			Severity:    "medium",
			Description: fmt.Sprintf("Large feature count (%d) increases failure risk", len(analysis.Features)),
			Mitigation:  "Prioritize core features and implement incrementally",
		})
	}

	// External dependency risk
	if analysis.TechStack != nil {
		if len(analysis.TechStack.Extras) > 5 {
			score += 10
			risks = append(risks, RiskAssessment{
				Type:        "dependency",
				Severity:    "medium",
				Description: "Many external dependencies increase integration complexity",
				Mitigation:  "Verify dependency compatibility before code generation",
			})
		}
	}

	// Security risk for certain features
	for _, feature := range analysis.Features {
		featureLower := strings.ToLower(feature.Name + " " + feature.Description)
		if strings.Contains(featureLower, "auth") || strings.Contains(featureLower, "payment") ||
			strings.Contains(featureLower, "user data") || strings.Contains(featureLower, "credential") {
			score += 15
			risks = append(risks, RiskAssessment{
				Type:        "security",
				Severity:    "high",
				Description: fmt.Sprintf("Feature '%s' requires careful security implementation", feature.Name),
				Mitigation:  "Include security review step and follow OWASP guidelines",
			})
			break
		}
	}

	// Data model risk
	if len(analysis.DataModels) > 10 {
		score += 10
		risks = append(risks, RiskAssessment{
			Type:        "data",
			Severity:    "medium",
			Description: "Complex data model may have relationship issues",
			Mitigation:  "Validate schema before proceeding to code generation",
		})
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	analysis.RiskScore = score
	analysis.Risks = risks
	return analysis
}

// addBuildVerificationGates inserts mandatory verification steps
func (p *Planner) addBuildVerificationGates(plan *ExecutionPlan, analysis *RequirementAnalysis) *ExecutionPlan {
	newSteps := make([]*PlanStep, 0)
	verifyStepCount := 0

	for _, step := range plan.Steps {
		newSteps = append(newSteps, step)

		// After code generation steps, add verification
		if step.ActionType == ActionAIGenerate {
			verifyStepCount++
			verifyStep := &PlanStep{
				ID:          uuid.New().String(),
				Order:       step.Order + 1,
				Name:        fmt.Sprintf("Verify: %s", step.Name),
				Description: "Run build verification to catch compilation errors",
				ActionType:  ActionVerifyBuild,
				Input: map[string]interface{}{
					"generated_by": step.ID,
					"tech_stack":   analysis.TechStack,
				},
				Dependencies: []string{step.ID},
				Status:       StepPending,
			}
			newSteps = append(newSteps, verifyStep)
		}
	}

	// Always add final comprehensive verification
	finalVerify := &PlanStep{
		ID:          uuid.New().String(),
		Order:       len(newSteps),
		Name:        "Final Build Verification",
		Description: "Comprehensive build, lint, type-check, and test verification",
		ActionType:  ActionVerifyBuild,
		Input: map[string]interface{}{
			"comprehensive": true,
			"tech_stack":    analysis.TechStack,
			"run_tests":     true,
			"run_lint":      true,
		},
		Status: StepPending,
	}
	if len(newSteps) > 0 {
		finalVerify.Dependencies = []string{newSteps[len(newSteps)-1].ID}
	}
	newSteps = append(newSteps, finalVerify)

	plan.Steps = newSteps
	return plan
}

// optimizeStepOrder reorders steps based on dependency graph for parallel execution
func (p *Planner) optimizeStepOrder(plan *ExecutionPlan) *ExecutionPlan {
	// Build dependency map
	dependencyCount := make(map[string]int)
	dependents := make(map[string][]string)

	for _, step := range plan.Steps {
		dependencyCount[step.ID] = len(step.Dependencies)
		for _, depID := range step.Dependencies {
			dependents[depID] = append(dependents[depID], step.ID)
		}
	}

	// Topological sort with level assignment
	ordered := make([]*PlanStep, 0)
	ready := make([]*PlanStep, 0)
	stepMap := make(map[string]*PlanStep)

	for _, step := range plan.Steps {
		stepMap[step.ID] = step
		if dependencyCount[step.ID] == 0 {
			ready = append(ready, step)
		}
	}

	level := 0
	for len(ready) > 0 {
		nextReady := make([]*PlanStep, 0)
		for _, step := range ready {
			step.Order = level
			ordered = append(ordered, step)

			for _, depID := range dependents[step.ID] {
				dependencyCount[depID]--
				if dependencyCount[depID] == 0 {
					nextReady = append(nextReady, stepMap[depID])
				}
			}
		}
		ready = nextReady
		level++
	}

	plan.Steps = ordered
	return plan
}

// RequirementAnalysis holds the parsed requirements
type RequirementAnalysis struct {
	AppType       string           `json:"app_type"`        // web, api, cli, fullstack
	Features      []Feature        `json:"features"`        // Key features to implement
	DataModels    []DataModel      `json:"data_models"`     // Data structures needed
	TechStack     *TechStack       `json:"tech_stack"`      // Recommended technologies
	Complexity    string           `json:"complexity"`      // simple, medium, complex
	EstimatedTime string           `json:"estimated_time"`  // Rough time estimate
	RiskScore     int              `json:"risk_score"`      // 0-100 risk assessment
	Risks         []RiskAssessment `json:"risks"`           // Identified risks
	Dependencies  []DependencyInfo `json:"dependencies"`    // External dependencies
	PreflightChecks []PreflightCheck `json:"preflight_checks"` // Pre-build requirements
}

// RiskAssessment identifies potential risks in the build
type RiskAssessment struct {
	Type        string `json:"type"`        // security, complexity, integration, dependency
	Severity    string `json:"severity"`    // low, medium, high, critical
	Description string `json:"description"`
	Mitigation  string `json:"mitigation"`
}

// DependencyInfo tracks external dependencies
type DependencyInfo struct {
	Name         string   `json:"name"`
	Version      string   `json:"version,omitempty"`
	Type         string   `json:"type"` // npm, pip, go, cargo
	Required     bool     `json:"required"`
	Alternatives []string `json:"alternatives,omitempty"`
}

// PreflightCheck defines pre-build requirements
type PreflightCheck struct {
	Name        string `json:"name"`
	Command     string `json:"command,omitempty"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// Feature represents a feature to implement
type Feature struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"` // high, medium, low
	Dependencies []string `json:"dependencies,omitempty"`
}

// DataModel represents a data structure
type DataModel struct {
	Name   string            `json:"name"`
	Fields map[string]string `json:"fields"` // field_name: type
}

// analyzeRequirements uses AI to understand what needs to be built
func (p *Planner) analyzeRequirements(ctx context.Context, description string) (*RequirementAnalysis, error) {
	prompt := fmt.Sprintf(`Analyze the following application requirements and output a structured JSON analysis.

REQUIREMENTS:
%s

Output ONLY valid JSON in this exact format (no markdown, no explanation):
{
  "app_type": "web|api|cli|fullstack",
  "features": [
    {"name": "feature name", "description": "what it does", "priority": "high|medium|low", "dependencies": []}
  ],
  "data_models": [
    {"name": "ModelName", "fields": {"field_name": "type"}}
  ],
  "tech_stack": {
    "frontend": "React|Vue|Next.js|none",
    "backend": "Node|Go|Python|none",
    "database": "PostgreSQL|MongoDB|SQLite|none",
    "styling": "Tailwind|CSS Modules|styled-components|none",
    "extras": ["additional libraries"]
  },
  "complexity": "simple|medium|complex",
  "estimated_time": "30min|1hour|2hours|4hours|8hours"
}`, description)

	response, err := p.ai.Generate(ctx, prompt, AIOptions{
		MaxTokens:    2000,
		Temperature:  0.3,
		SystemPrompt: "You are a senior software architect. Analyze requirements precisely and output valid JSON only.",
	})
	if err != nil {
		return nil, err
	}

	// Parse the JSON response
	analysis := &RequirementAnalysis{}
	if err := p.parseJSONResponse(response, analysis); err != nil {
		// Fallback to default analysis if parsing fails
		log.Printf("Planner: JSON parsing failed, using default analysis: %v", err)
		analysis = p.createDefaultAnalysis(description)
	}

	return analysis, nil
}

// generatePlan creates the detailed execution plan
func (p *Planner) generatePlan(ctx context.Context, description string, analysis *RequirementAnalysis) (*ExecutionPlan, error) {
	plan := &ExecutionPlan{
		ID:           uuid.New().String(),
		Steps:        make([]*PlanStep, 0),
		TechStack:    analysis.TechStack,
		CreatedAt:    time.Now(),
		Dependencies: make([]string, 0),
	}

	// Add steps based on the analysis
	stepOrder := 0

	// Step 1: Project setup
	plan.Steps = append(plan.Steps, &PlanStep{
		ID:          uuid.New().String(),
		Order:       stepOrder,
		Name:        "Initialize Project Structure",
		Description: "Create project directories and configuration files",
		ActionType:  ActionCreateFile,
		Input: map[string]interface{}{
			"type":       "project_init",
			"tech_stack": analysis.TechStack,
		},
		Status: StepPending,
	})
	stepOrder++

	// Step 2: Install dependencies
	deps := p.getDependencies(analysis)
	if len(deps) > 0 {
		plan.Steps = append(plan.Steps, &PlanStep{
			ID:          uuid.New().String(),
			Order:       stepOrder,
			Name:        "Install Dependencies",
			Description: fmt.Sprintf("Install required packages: %s", strings.Join(deps, ", ")),
			ActionType:  ActionInstallDeps,
			Input: map[string]interface{}{
				"dependencies": deps,
				"tech_stack":   analysis.TechStack,
			},
			Dependencies: []string{plan.Steps[0].ID},
			Status:       StepPending,
		})
		stepOrder++
	}

	// Step 3: Generate data models/schemas
	if len(analysis.DataModels) > 0 {
		plan.Steps = append(plan.Steps, &PlanStep{
			ID:          uuid.New().String(),
			Order:       stepOrder,
			Name:        "Create Data Models",
			Description: "Generate database schemas and TypeScript types",
			ActionType:  ActionAIGenerate,
			Input: map[string]interface{}{
				"type":        "data_models",
				"models":      analysis.DataModels,
				"description": description,
			},
			Dependencies: []string{plan.Steps[0].ID},
			Status:       StepPending,
		})
		stepOrder++
	}

	// Step 4: Generate backend code
	if analysis.TechStack != nil && analysis.TechStack.Backend != "" && analysis.TechStack.Backend != "none" {
		backendStep := &PlanStep{
			ID:          uuid.New().String(),
			Order:       stepOrder,
			Name:        "Generate Backend Code",
			Description: fmt.Sprintf("Create %s backend with API endpoints", analysis.TechStack.Backend),
			ActionType:  ActionAIGenerate,
			Input: map[string]interface{}{
				"type":        "backend",
				"framework":   analysis.TechStack.Backend,
				"features":    analysis.Features,
				"models":      analysis.DataModels,
				"description": description,
			},
			Status: StepPending,
		}
		if len(plan.Steps) > 0 {
			backendStep.Dependencies = []string{plan.Steps[len(plan.Steps)-1].ID}
		}
		plan.Steps = append(plan.Steps, backendStep)
		stepOrder++
	}

	// Step 5: Generate frontend code
	if analysis.TechStack != nil && analysis.TechStack.Frontend != "" && analysis.TechStack.Frontend != "none" {
		frontendStep := &PlanStep{
			ID:          uuid.New().String(),
			Order:       stepOrder,
			Name:        "Generate Frontend Code",
			Description: fmt.Sprintf("Create %s frontend with components", analysis.TechStack.Frontend),
			ActionType:  ActionAIGenerate,
			Input: map[string]interface{}{
				"type":        "frontend",
				"framework":   analysis.TechStack.Frontend,
				"styling":     analysis.TechStack.Styling,
				"features":    analysis.Features,
				"description": description,
			},
			Status: StepPending,
		}
		if len(plan.Steps) > 0 {
			frontendStep.Dependencies = []string{plan.Steps[len(plan.Steps)-1].ID}
		}
		plan.Steps = append(plan.Steps, frontendStep)
		stepOrder++
	}

	// Step 6: Generate tests
	plan.Steps = append(plan.Steps, &PlanStep{
		ID:          uuid.New().String(),
		Order:       stepOrder,
		Name:        "Generate Tests",
		Description: "Create unit and integration tests",
		ActionType:  ActionAIGenerate,
		Input: map[string]interface{}{
			"type":        "tests",
			"features":    analysis.Features,
			"tech_stack":  analysis.TechStack,
			"description": description,
		},
		Dependencies: []string{plan.Steps[len(plan.Steps)-1].ID},
		Status:       StepPending,
	})
	stepOrder++

	// Step 7: Run tests
	plan.Steps = append(plan.Steps, &PlanStep{
		ID:          uuid.New().String(),
		Order:       stepOrder,
		Name:        "Run Tests",
		Description: "Execute test suite to verify implementation",
		ActionType:  ActionRunTests,
		Input: map[string]interface{}{
			"tech_stack": analysis.TechStack,
		},
		Dependencies: []string{plan.Steps[len(plan.Steps)-1].ID},
		Status:       StepPending,
	})
	stepOrder++

	// Step 8: Generate documentation
	plan.Steps = append(plan.Steps, &PlanStep{
		ID:          uuid.New().String(),
		Order:       stepOrder,
		Name:        "Generate Documentation",
		Description: "Create README and API documentation",
		ActionType:  ActionAIGenerate,
		Input: map[string]interface{}{
			"type":        "documentation",
			"features":    analysis.Features,
			"tech_stack":  analysis.TechStack,
			"description": description,
		},
		Dependencies: []string{plan.Steps[len(plan.Steps)-1].ID},
		Status:       StepPending,
	})

	// Calculate estimated time
	plan.EstimatedTime = p.calculateEstimatedTime(analysis)

	return plan, nil
}

// validatePlan ensures the plan is valid and complete
func (p *Planner) validatePlan(plan *ExecutionPlan) error {
	if len(plan.Steps) == 0 {
		return fmt.Errorf("plan has no steps")
	}

	stepIDs := make(map[string]bool)
	for _, step := range plan.Steps {
		if step.ID == "" {
			return fmt.Errorf("step has no ID")
		}
		if step.Name == "" {
			return fmt.Errorf("step %s has no name", step.ID)
		}
		stepIDs[step.ID] = true
	}

	// Validate dependencies
	for _, step := range plan.Steps {
		for _, depID := range step.Dependencies {
			if !stepIDs[depID] {
				return fmt.Errorf("step %s has invalid dependency %s", step.ID, depID)
			}
		}
	}

	return nil
}

// Helper methods

func (p *Planner) parseJSONResponse(response string, target interface{}) error {
	// Try to find JSON in the response
	response = strings.TrimSpace(response)

	// Remove markdown code blocks if present
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	// Find the first { and last }
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start != -1 && end != -1 && end > start {
		response = response[start : end+1]
	}

	return json.Unmarshal([]byte(response), target)
}

func (p *Planner) createDefaultAnalysis(description string) *RequirementAnalysis {
	// Detect app type from description
	descLower := strings.ToLower(description)

	appType := "web"
	if strings.Contains(descLower, "api") || strings.Contains(descLower, "backend only") {
		appType = "api"
	} else if strings.Contains(descLower, "fullstack") || strings.Contains(descLower, "full stack") {
		appType = "fullstack"
	} else if strings.Contains(descLower, "cli") || strings.Contains(descLower, "command line") {
		appType = "cli"
	}

	// Default tech stack
	stack := &TechStack{
		Frontend: "React",
		Backend:  "Node",
		Database: "PostgreSQL",
		Styling:  "Tailwind",
		Extras:   []string{"TypeScript", "Vite"},
	}

	if appType == "api" {
		stack.Frontend = ""
	}

	return &RequirementAnalysis{
		AppType: appType,
		Features: []Feature{
			{
				Name:        "Core Functionality",
				Description: description,
				Priority:    "high",
			},
		},
		DataModels: []DataModel{},
		TechStack:  stack,
		Complexity: "medium",
		EstimatedTime: "2hours",
	}
}

func (p *Planner) getDependencies(analysis *RequirementAnalysis) []string {
	deps := make([]string, 0)

	if analysis.TechStack == nil {
		return deps
	}

	// Add dependencies based on tech stack
	switch analysis.TechStack.Frontend {
	case "React":
		deps = append(deps, "react", "react-dom", "@types/react", "vite")
	case "Vue":
		deps = append(deps, "vue", "@vue/compiler-sfc", "vite")
	case "Next.js":
		deps = append(deps, "next", "react", "react-dom")
	}

	switch analysis.TechStack.Styling {
	case "Tailwind":
		deps = append(deps, "tailwindcss", "postcss", "autoprefixer")
	case "styled-components":
		deps = append(deps, "styled-components")
	}

	switch analysis.TechStack.Backend {
	case "Node":
		deps = append(deps, "express", "@types/express", "cors")
	case "Go":
		deps = append(deps, "github.com/gin-gonic/gin")
	case "Python":
		deps = append(deps, "fastapi", "uvicorn", "pydantic")
	}

	switch analysis.TechStack.Database {
	case "PostgreSQL":
		deps = append(deps, "pg", "@types/pg")
	case "MongoDB":
		deps = append(deps, "mongoose")
	case "SQLite":
		deps = append(deps, "better-sqlite3")
	}

	deps = append(deps, analysis.TechStack.Extras...)

	return deps
}

func (p *Planner) calculateEstimatedTime(analysis *RequirementAnalysis) time.Duration {
	// Base time based on complexity
	baseTime := 30 * time.Minute

	switch analysis.Complexity {
	case "simple":
		baseTime = 15 * time.Minute
	case "medium":
		baseTime = 45 * time.Minute
	case "complex":
		baseTime = 90 * time.Minute
	}

	// Add time for features
	baseTime += time.Duration(len(analysis.Features)*10) * time.Minute

	// Add time for data models
	baseTime += time.Duration(len(analysis.DataModels)*5) * time.Minute

	return baseTime
}

// RefineStep refines a plan step with more detail
func (p *Planner) RefineStep(ctx context.Context, step *PlanStep, context string) (*PlanStep, error) {
	prompt := fmt.Sprintf(`Refine this execution step with more specific details.

STEP:
Name: %s
Description: %s
Action Type: %s
Current Input: %v

CONTEXT:
%s

Output a more detailed step as JSON:
{
  "name": "refined name",
  "description": "more specific description",
  "action_type": "%s",
  "input": {
    "detailed": "input parameters"
  }
}`, step.Name, step.Description, step.ActionType, step.Input, context, step.ActionType)

	response, err := p.ai.Generate(ctx, prompt, AIOptions{
		MaxTokens:    1000,
		Temperature:  0.3,
		SystemPrompt: "You are a software architect. Provide detailed implementation steps.",
	})
	if err != nil {
		return nil, err
	}

	var refined struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		ActionType  string                 `json:"action_type"`
		Input       map[string]interface{} `json:"input"`
	}

	if err := p.parseJSONResponse(response, &refined); err != nil {
		return step, nil // Return original if parsing fails
	}

	// Update the step
	step.Name = refined.Name
	step.Description = refined.Description
	if refined.Input != nil {
		step.Input = refined.Input
	}

	return step, nil
}

// AdaptPlan modifies the plan based on feedback
func (p *Planner) AdaptPlan(ctx context.Context, plan *ExecutionPlan, feedback string) (*ExecutionPlan, error) {
	prompt := fmt.Sprintf(`Adapt this execution plan based on the feedback.

CURRENT PLAN:
%d steps total

FEEDBACK:
%s

Suggest modifications as JSON:
{
  "add_steps": [
    {"name": "...", "description": "...", "action_type": "...", "after_step": "step_id or 'start'"}
  ],
  "remove_steps": ["step_id"],
  "modify_steps": [
    {"step_id": "...", "new_name": "...", "new_description": "..."}
  ]
}`, len(plan.Steps), feedback)

	response, err := p.ai.Generate(ctx, prompt, AIOptions{
		MaxTokens:    1500,
		Temperature:  0.4,
		SystemPrompt: "You are a software architect. Adapt plans based on feedback.",
	})
	if err != nil {
		return nil, err
	}

	// Parse and apply modifications
	var mods struct {
		AddSteps    []map[string]interface{} `json:"add_steps"`
		RemoveSteps []string                 `json:"remove_steps"`
		ModifySteps []map[string]interface{} `json:"modify_steps"`
	}

	if err := p.parseJSONResponse(response, &mods); err != nil {
		log.Printf("Planner: Could not parse adaptation response: %v", err)
		return plan, nil
	}

	// Apply removals
	for _, stepID := range mods.RemoveSteps {
		for i, step := range plan.Steps {
			if step.ID == stepID {
				plan.Steps = append(plan.Steps[:i], plan.Steps[i+1:]...)
				break
			}
		}
	}

	// Apply modifications
	for _, mod := range mods.ModifySteps {
		stepID, ok := mod["step_id"].(string)
		if !ok {
			continue
		}
		for _, step := range plan.Steps {
			if step.ID == stepID {
				if newName, ok := mod["new_name"].(string); ok {
					step.Name = newName
				}
				if newDesc, ok := mod["new_description"].(string); ok {
					step.Description = newDesc
				}
			}
		}
	}

	// Reorder steps
	for i, step := range plan.Steps {
		step.Order = i
	}

	return plan, nil
}
