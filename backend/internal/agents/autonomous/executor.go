// Package autonomous - Action Executor
// Executes plan steps including file operations, terminal commands, and AI calls
package autonomous

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Executor handles action execution
type Executor struct {
	ai      AIProvider
	workDir string
}

// NewExecutor creates a new executor
func NewExecutor(ai AIProvider, workDir string) *Executor {
	return &Executor{
		ai:      ai,
		workDir: workDir,
	}
}

// ExecuteStep executes a single plan step
func (e *Executor) ExecuteStep(ctx context.Context, step *PlanStep, task *AutonomousTask) (map[string]interface{}, error) {
	log.Printf("Executor: Executing step %s (%s)", step.Name, step.ActionType)

	switch step.ActionType {
	case ActionCreateFile:
		return e.executeCreateFile(ctx, step, task)
	case ActionModifyFile:
		return e.executeModifyFile(ctx, step, task)
	case ActionDeleteFile:
		return e.executeDeleteFile(ctx, step, task)
	case ActionRunCommand:
		return e.executeRunCommand(ctx, step, task)
	case ActionRunTests:
		return e.executeRunTests(ctx, step, task)
	case ActionInstallDeps:
		return e.executeInstallDeps(ctx, step, task)
	case ActionAIGenerate:
		return e.executeAIGenerate(ctx, step, task)
	case ActionAIAnalyze:
		return e.executeAIAnalyze(ctx, step, task)
	case ActionValidate:
		return e.executeValidate(ctx, step, task)
	case ActionDeploy:
		return e.executeDeploy(ctx, step, task)
	case ActionRollback:
		return e.executeRollback(ctx, step, task)
	default:
		return nil, fmt.Errorf("unknown action type: %s", step.ActionType)
	}
}

// executeCreateFile creates a new file
func (e *Executor) executeCreateFile(ctx context.Context, step *PlanStep, task *AutonomousTask) (map[string]interface{}, error) {
	input := step.Input

	// Handle project initialization
	if inputType, ok := input["type"].(string); ok && inputType == "project_init" {
		return e.initializeProject(ctx, input, task)
	}

	// Get file path and content
	filePath, ok := input["path"].(string)
	if !ok {
		return nil, fmt.Errorf("file path not specified")
	}

	content, ok := input["content"].(string)
	if !ok {
		return nil, fmt.Errorf("file content not specified")
	}

	// Resolve full path
	fullPath := filepath.Join(e.workDir, filePath)

	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file %s: %w", fullPath, err)
	}

	log.Printf("Executor: Created file %s (%d bytes)", filePath, len(content))

	return map[string]interface{}{
		"path":    filePath,
		"size":    len(content),
		"created": true,
	}, nil
}

// initializeProject creates the project structure
func (e *Executor) initializeProject(ctx context.Context, input map[string]interface{}, task *AutonomousTask) (map[string]interface{}, error) {
	techStack, _ := input["tech_stack"].(*TechStack)
	if techStack == nil {
		// Try to extract from map
		if ts, ok := input["tech_stack"].(map[string]interface{}); ok {
			techStack = &TechStack{
				Frontend: getStringFromMap(ts, "frontend"),
				Backend:  getStringFromMap(ts, "backend"),
				Database: getStringFromMap(ts, "database"),
				Styling:  getStringFromMap(ts, "styling"),
			}
		} else {
			techStack = &TechStack{
				Frontend: "React",
				Backend:  "Node",
			}
		}
	}

	filesCreated := []string{}

	// Create base directories
	dirs := []string{
		"src",
		"src/components",
		"src/hooks",
		"src/utils",
		"src/types",
		"public",
		"tests",
	}

	if techStack.Backend != "" && techStack.Backend != "none" {
		dirs = append(dirs, "server", "server/routes", "server/middleware", "server/models")
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(e.workDir, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			log.Printf("Executor: Warning - failed to create directory %s: %v", dir, err)
		}
	}

	// Create package.json
	packageJSON := e.generatePackageJSON(task.Description, techStack)
	if err := os.WriteFile(filepath.Join(e.workDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		log.Printf("Executor: Warning - failed to create package.json: %v", err)
	} else {
		filesCreated = append(filesCreated, "package.json")
	}

	// Create tsconfig.json
	tsconfigJSON := e.generateTsConfig()
	if err := os.WriteFile(filepath.Join(e.workDir, "tsconfig.json"), []byte(tsconfigJSON), 0644); err != nil {
		log.Printf("Executor: Warning - failed to create tsconfig.json: %v", err)
	} else {
		filesCreated = append(filesCreated, "tsconfig.json")
	}

	// Create .env.example
	envExample := e.generateEnvExample(techStack)
	if err := os.WriteFile(filepath.Join(e.workDir, ".env.example"), []byte(envExample), 0644); err != nil {
		log.Printf("Executor: Warning - failed to create .env.example: %v", err)
	} else {
		filesCreated = append(filesCreated, ".env.example")
	}

	// Create .gitignore
	gitignore := e.generateGitignore()
	if err := os.WriteFile(filepath.Join(e.workDir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		log.Printf("Executor: Warning - failed to create .gitignore: %v", err)
	} else {
		filesCreated = append(filesCreated, ".gitignore")
	}

	// Create Tailwind config if using Tailwind
	if techStack.Styling == "Tailwind" {
		tailwindConfig := e.generateTailwindConfig()
		if err := os.WriteFile(filepath.Join(e.workDir, "tailwind.config.js"), []byte(tailwindConfig), 0644); err != nil {
			log.Printf("Executor: Warning - failed to create tailwind.config.js: %v", err)
		} else {
			filesCreated = append(filesCreated, "tailwind.config.js")
		}
	}

	log.Printf("Executor: Initialized project with %d files", len(filesCreated))

	return map[string]interface{}{
		"files_created": filesCreated,
		"directories":   dirs,
	}, nil
}

// executeModifyFile modifies an existing file
func (e *Executor) executeModifyFile(ctx context.Context, step *PlanStep, task *AutonomousTask) (map[string]interface{}, error) {
	input := step.Input

	filePath, ok := input["path"].(string)
	if !ok {
		return nil, fmt.Errorf("file path not specified")
	}

	fullPath := filepath.Join(e.workDir, filePath)

	// Read existing content
	existingContent, err := os.ReadFile(fullPath)
	if err != nil {
		// If file doesn't exist, treat as create
		if os.IsNotExist(err) {
			return e.executeCreateFile(ctx, step, task)
		}
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Apply modifications based on type
	modificationType, _ := input["modification_type"].(string)
	var newContent string

	switch modificationType {
	case "replace":
		newContent, _ = input["content"].(string)
	case "append":
		appendContent, _ := input["content"].(string)
		newContent = string(existingContent) + "\n" + appendContent
	case "insert":
		insertContent, _ := input["content"].(string)
		position, _ := input["position"].(int)
		existingLines := strings.Split(string(existingContent), "\n")
		if position < 0 || position > len(existingLines) {
			position = len(existingLines)
		}
		newLines := append(existingLines[:position], append([]string{insertContent}, existingLines[position:]...)...)
		newContent = strings.Join(newLines, "\n")
	case "search_replace":
		search, _ := input["search"].(string)
		replace, _ := input["replace"].(string)
		newContent = strings.ReplaceAll(string(existingContent), search, replace)
	default:
		newContent, _ = input["content"].(string)
	}

	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file %s: %w", fullPath, err)
	}

	log.Printf("Executor: Modified file %s", filePath)

	return map[string]interface{}{
		"path":     filePath,
		"modified": true,
		"old_size": len(existingContent),
		"new_size": len(newContent),
	}, nil
}

// executeDeleteFile deletes a file
func (e *Executor) executeDeleteFile(ctx context.Context, step *PlanStep, task *AutonomousTask) (map[string]interface{}, error) {
	filePath, ok := step.Input["path"].(string)
	if !ok {
		return nil, fmt.Errorf("file path not specified")
	}

	fullPath := filepath.Join(e.workDir, filePath)

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{
				"path":    filePath,
				"deleted": false,
				"reason":  "file not found",
			}, nil
		}
		return nil, fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}

	log.Printf("Executor: Deleted file %s", filePath)

	return map[string]interface{}{
		"path":    filePath,
		"deleted": true,
	}, nil
}

// executeRunCommand runs a terminal command
func (e *Executor) executeRunCommand(ctx context.Context, step *PlanStep, task *AutonomousTask) (map[string]interface{}, error) {
	command, ok := step.Input["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command not specified")
	}

	// Parse command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = e.workDir

	// Capture output
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set environment
	cmd.Env = os.Environ()

	log.Printf("Executor: Running command: %s", command)

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	result := map[string]interface{}{
		"command":  command,
		"stdout":   stdout.String(),
		"stderr":   stderr.String(),
		"duration": duration.String(),
	}

	if err != nil {
		result["exit_code"] = -1
		if exitError, ok := err.(*exec.ExitError); ok {
			result["exit_code"] = exitError.ExitCode()
		}
		return result, fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
	}

	result["exit_code"] = 0
	log.Printf("Executor: Command completed in %v", duration)

	return result, nil
}

// executeRunTests runs the test suite
func (e *Executor) executeRunTests(ctx context.Context, step *PlanStep, task *AutonomousTask) (map[string]interface{}, error) {
	techStack, _ := step.Input["tech_stack"].(*TechStack)

	// Determine test command based on tech stack
	var testCmd string
	switch {
	case techStack != nil && techStack.Backend == "Go":
		testCmd = "go test ./..."
	case techStack != nil && techStack.Backend == "Python":
		testCmd = "pytest"
	default:
		testCmd = "npm test"
	}

	// Check if tests exist
	testDir := filepath.Join(e.workDir, "tests")
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		return map[string]interface{}{
			"skipped": true,
			"reason":  "no tests directory found",
		}, nil
	}

	step.Input["command"] = testCmd
	return e.executeRunCommand(ctx, step, task)
}

// executeInstallDeps installs dependencies
func (e *Executor) executeInstallDeps(ctx context.Context, step *PlanStep, task *AutonomousTask) (map[string]interface{}, error) {
	deps, _ := step.Input["dependencies"].([]string)
	if len(deps) == 0 {
		return map[string]interface{}{
			"skipped": true,
			"reason":  "no dependencies specified",
		}, nil
	}

	techStack, _ := step.Input["tech_stack"].(*TechStack)

	// Determine package manager and install command
	var installCmd string
	switch {
	case techStack != nil && techStack.Backend == "Go":
		installCmd = "go mod tidy"
	case techStack != nil && techStack.Backend == "Python":
		installCmd = "pip install " + strings.Join(deps, " ")
	default:
		// Check if package.json exists
		if _, err := os.Stat(filepath.Join(e.workDir, "package.json")); err == nil {
			installCmd = "npm install"
		} else {
			installCmd = "npm init -y && npm install " + strings.Join(deps, " ")
		}
	}

	step.Input["command"] = installCmd
	return e.executeRunCommand(ctx, step, task)
}

// executeAIGenerate uses AI to generate code
func (e *Executor) executeAIGenerate(ctx context.Context, step *PlanStep, task *AutonomousTask) (map[string]interface{}, error) {
	input := step.Input
	genType, _ := input["type"].(string)
	description, _ := input["description"].(string)

	var prompt string
	var systemPrompt string

	switch genType {
	case "backend":
		framework, _ := input["framework"].(string)
		prompt = e.generateBackendPrompt(description, framework, input)
		systemPrompt = "You are a senior backend developer. Generate complete, production-ready code with no placeholders."

	case "frontend":
		framework, _ := input["framework"].(string)
		styling, _ := input["styling"].(string)
		prompt = e.generateFrontendPrompt(description, framework, styling, input)
		systemPrompt = "You are a senior frontend developer. Generate complete, production-ready React components with no placeholders."

	case "data_models":
		models, _ := input["models"].([]DataModel)
		prompt = e.generateDataModelsPrompt(description, models)
		systemPrompt = "You are a database architect. Generate complete schema definitions and TypeScript types."

	case "tests":
		prompt = e.generateTestsPrompt(description, input)
		systemPrompt = "You are a QA engineer. Generate comprehensive tests with good coverage."

	case "documentation":
		prompt = e.generateDocumentationPrompt(description, input)
		systemPrompt = "You are a technical writer. Generate clear, helpful documentation."

	default:
		prompt = fmt.Sprintf("Generate code for: %s", description)
		systemPrompt = "You are a senior software engineer. Generate complete, production-ready code."
	}

	// Add validation feedback if present
	if feedback, ok := input["validation_feedback"]; ok {
		prompt += fmt.Sprintf("\n\nPrevious attempt had issues:\n%v\n\nFix these issues in your response.", feedback)
	}

	// Add error analysis if present
	if analysis, ok := input["previous_error_analysis"].(string); ok {
		prompt += fmt.Sprintf("\n\nPrevious error analysis:\n%s\n\nAvoid these issues.", analysis)
	}

	response, err := e.ai.Generate(ctx, prompt, AIOptions{
		MaxTokens:    8000,
		Temperature:  0.3,
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	// Parse the response and create files
	files := e.parseAIResponse(response)

	filesCreated := []string{}
	for _, file := range files {
		fullPath := filepath.Join(e.workDir, file.Path)

		// Create directory if needed
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Executor: Warning - failed to create directory %s: %v", dir, err)
			continue
		}

		if err := os.WriteFile(fullPath, []byte(file.Content), 0644); err != nil {
			log.Printf("Executor: Warning - failed to write file %s: %v", file.Path, err)
			continue
		}

		filesCreated = append(filesCreated, file.Path)
		log.Printf("Executor: Created %s (%d bytes)", file.Path, len(file.Content))
	}

	return map[string]interface{}{
		"files_created": filesCreated,
		"response_size": len(response),
	}, nil
}

// executeAIAnalyze uses AI to analyze content
func (e *Executor) executeAIAnalyze(ctx context.Context, step *PlanStep, task *AutonomousTask) (map[string]interface{}, error) {
	content, _ := step.Input["content"].(string)
	instruction, _ := step.Input["instruction"].(string)

	response, err := e.ai.Analyze(ctx, content, instruction, AIOptions{
		MaxTokens:   2000,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	return map[string]interface{}{
		"analysis": response,
	}, nil
}

// executeValidate validates generated output
func (e *Executor) executeValidate(ctx context.Context, step *PlanStep, task *AutonomousTask) (map[string]interface{}, error) {
	// This is typically handled by the Validator component
	return map[string]interface{}{
		"validated": true,
	}, nil
}

// executeDeploy handles deployment
func (e *Executor) executeDeploy(ctx context.Context, step *PlanStep, task *AutonomousTask) (map[string]interface{}, error) {
	// Placeholder for deployment logic
	return map[string]interface{}{
		"deployed": false,
		"reason":   "deployment not yet implemented",
	}, nil
}

// executeRollback rolls back changes
func (e *Executor) executeRollback(ctx context.Context, step *PlanStep, task *AutonomousTask) (map[string]interface{}, error) {
	// Placeholder for rollback logic
	return map[string]interface{}{
		"rolledback": false,
		"reason":     "rollback not yet implemented",
	}, nil
}

// GeneratedFile represents a parsed file from AI response
type GeneratedFile struct {
	Path    string
	Content string
}

// parseAIResponse extracts files from AI-generated response
func (e *Executor) parseAIResponse(response string) []GeneratedFile {
	files := make([]GeneratedFile, 0)

	// Look for file markers: // File: path/to/file.ext
	lines := strings.Split(response, "\n")
	var currentFile *GeneratedFile
	var codeBuffer strings.Builder
	inCodeBlock := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check for file path markers
		if strings.HasPrefix(trimmedLine, "// File:") || strings.HasPrefix(trimmedLine, "# File:") ||
			strings.HasPrefix(trimmedLine, "/* File:") {

			// Save previous file if any
			if currentFile != nil && codeBuffer.Len() > 0 {
				currentFile.Content = strings.TrimSpace(codeBuffer.String())
				files = append(files, *currentFile)
			}

			// Extract file path
			filePath := ""
			if strings.HasPrefix(trimmedLine, "// File:") {
				filePath = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "// File:"))
			} else if strings.HasPrefix(trimmedLine, "# File:") {
				filePath = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "# File:"))
			} else if strings.HasPrefix(trimmedLine, "/* File:") {
				filePath = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "/* File:"))
				filePath = strings.TrimSuffix(filePath, "*/")
			}

			if filePath != "" {
				currentFile = &GeneratedFile{Path: strings.TrimSpace(filePath)}
				codeBuffer.Reset()
			}
			continue
		}

		// Check for code block markers
		if strings.HasPrefix(trimmedLine, "```") {
			if !inCodeBlock {
				inCodeBlock = true
				continue
			} else {
				inCodeBlock = false
				if currentFile != nil && codeBuffer.Len() > 0 {
					currentFile.Content = strings.TrimSpace(codeBuffer.String())
					files = append(files, *currentFile)
					currentFile = nil
					codeBuffer.Reset()
				}
				continue
			}
		}

		// Add line to buffer if in a file context
		if currentFile != nil {
			if codeBuffer.Len() > 0 {
				codeBuffer.WriteString("\n")
			}
			codeBuffer.WriteString(line)
		}
	}

	// Handle last file
	if currentFile != nil && codeBuffer.Len() > 0 {
		currentFile.Content = strings.TrimSpace(codeBuffer.String())
		files = append(files, *currentFile)
	}

	return files
}

// Prompt generators

func (e *Executor) generateBackendPrompt(description string, framework string, input map[string]interface{}) string {
	features, _ := input["features"].([]Feature)
	models, _ := input["models"].([]DataModel)

	featureList := ""
	for _, f := range features {
		featureList += fmt.Sprintf("- %s: %s\n", f.Name, f.Description)
	}

	modelList := ""
	for _, m := range models {
		modelList += fmt.Sprintf("- %s: %v\n", m.Name, m.Fields)
	}

	return fmt.Sprintf(`Generate a complete %s backend for:
%s

Features to implement:
%s

Data models:
%s

Requirements:
1. Create complete API endpoints with full CRUD operations
2. Include proper error handling and validation
3. Add authentication middleware
4. Use TypeScript with proper types
5. Follow RESTful best practices

For EVERY file, use this format:
// File: path/to/filename.ext
`+"```"+`language
[complete file content]
`+"```"+`

Generate these files:
- server/index.ts (main server with middleware)
- server/routes/api.ts (API routes)
- server/middleware/auth.ts (authentication)
- server/models/types.ts (TypeScript types)
- server/controllers/main.ts (business logic)`, framework, description, featureList, modelList)
}

func (e *Executor) generateFrontendPrompt(description string, framework string, styling string, input map[string]interface{}) string {
	features, _ := input["features"].([]Feature)

	featureList := ""
	for _, f := range features {
		featureList += fmt.Sprintf("- %s: %s\n", f.Name, f.Description)
	}

	return fmt.Sprintf(`Generate a complete %s frontend with %s for:
%s

Features to implement:
%s

Requirements:
1. Create reusable, well-structured components
2. Include proper state management
3. Add responsive design
4. Use TypeScript with proper types
5. Include loading and error states

For EVERY file, use this format:
// File: path/to/filename.ext
`+"```"+`language
[complete file content]
`+"```"+`

Generate these files:
- src/App.tsx (main app component with routing)
- src/components/Layout.tsx (main layout)
- src/components/Header.tsx (navigation header)
- src/hooks/useApi.ts (API hook)
- src/types/index.ts (TypeScript types)
- src/pages/Home.tsx (home page)`, framework, styling, description, featureList)
}

func (e *Executor) generateDataModelsPrompt(description string, models []DataModel) string {
	modelList := ""
	for _, m := range models {
		modelList += fmt.Sprintf("\n%s:\n", m.Name)
		for field, typ := range m.Fields {
			modelList += fmt.Sprintf("  - %s: %s\n", field, typ)
		}
	}

	return fmt.Sprintf(`Generate data models and database schemas for:
%s

Models to create:
%s

Generate these files:
- src/types/models.ts (TypeScript interfaces)
- database/schema.sql (PostgreSQL schema)
- server/models/index.ts (ORM models)

For EVERY file, use this format:
// File: path/to/filename.ext
`+"```"+`language
[complete file content]
`+"```", description, modelList)
}

func (e *Executor) generateTestsPrompt(description string, input map[string]interface{}) string {
	features, _ := input["features"].([]Feature)

	featureList := ""
	for _, f := range features {
		featureList += fmt.Sprintf("- %s\n", f.Name)
	}

	return fmt.Sprintf(`Generate comprehensive tests for:
%s

Features to test:
%s

Generate these files:
- tests/unit/components.test.ts (unit tests)
- tests/integration/api.test.ts (API tests)
- tests/e2e/app.test.ts (end-to-end tests)

For EVERY file, use this format:
// File: path/to/filename.ext
`+"```"+`language
[complete file content]
`+"```", description, featureList)
}

func (e *Executor) generateDocumentationPrompt(description string, input map[string]interface{}) string {
	features, _ := input["features"].([]Feature)
	techStack, _ := input["tech_stack"].(*TechStack)

	featureList := ""
	for _, f := range features {
		featureList += fmt.Sprintf("- %s: %s\n", f.Name, f.Description)
	}

	stackInfo := "Not specified"
	if techStack != nil {
		stackInfo = fmt.Sprintf("Frontend: %s, Backend: %s, Database: %s", techStack.Frontend, techStack.Backend, techStack.Database)
	}

	return fmt.Sprintf(`Generate documentation for:
%s

Features:
%s

Tech Stack: %s

Generate these files:
- README.md (project documentation with setup instructions)
- docs/API.md (API documentation)

For EVERY file, use this format:
// File: path/to/filename.ext
`+"```"+`markdown
[complete file content]
`+"```", description, featureList, stackInfo)
}

// Config file generators

func (e *Executor) generatePackageJSON(description string, stack *TechStack) string {
	deps := []string{`"express": "^4.18.2"`, `"cors": "^2.8.5"`}
	devDeps := []string{`"typescript": "^5.0.0"`, `"@types/node": "^20.0.0"`}

	if stack.Frontend == "React" {
		deps = append(deps, `"react": "^18.2.0"`, `"react-dom": "^18.2.0"`)
		devDeps = append(devDeps, `"@types/react": "^18.2.0"`, `"vite": "^5.0.0"`)
	}

	if stack.Styling == "Tailwind" {
		devDeps = append(devDeps, `"tailwindcss": "^3.3.0"`, `"postcss": "^8.4.0"`, `"autoprefixer": "^10.4.0"`)
	}

	return fmt.Sprintf(`{
  "name": "apex-generated-app",
  "version": "1.0.0",
  "description": "%s",
  "main": "server/index.ts",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "server": "ts-node server/index.ts",
    "test": "vitest",
    "lint": "eslint . --ext .ts,.tsx"
  },
  "dependencies": {
    %s
  },
  "devDependencies": {
    %s
  }
}`, strings.ReplaceAll(description, `"`, `\"`), strings.Join(deps, ",\n    "), strings.Join(devDeps, ",\n    "))
}

func (e *Executor) generateTsConfig() string {
	return `{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"]
    }
  },
  "include": ["src", "server"],
  "references": [{ "path": "./tsconfig.node.json" }]
}`
}

func (e *Executor) generateEnvExample(stack *TechStack) string {
	env := `# Server Configuration
PORT=3000
NODE_ENV=development

# Database
DATABASE_URL=postgresql://user:password@localhost:5432/myapp

# Authentication
JWT_SECRET=your-secret-key-here

# API Keys (if needed)
# OPENAI_API_KEY=sk-...
`

	if stack.Database == "MongoDB" {
		env = strings.Replace(env, "DATABASE_URL=postgresql://", "MONGODB_URI=mongodb://", 1)
	}

	return env
}

func (e *Executor) generateGitignore() string {
	return `# Dependencies
node_modules/
.pnp
.pnp.js

# Build
dist/
build/
.next/
out/

# Environment
.env
.env.local
.env.*.local

# IDE
.vscode/
.idea/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Logs
*.log
npm-debug.log*

# Testing
coverage/

# Misc
*.pem
.vercel
`
}

func (e *Executor) generateTailwindConfig() string {
	return `/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {},
  },
  plugins: [],
}`
}

// Helper functions

func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// StreamCommandOutput streams command output line by line
func (e *Executor) StreamCommandOutput(ctx context.Context, command string, callback func(line string)) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = e.workDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Stream stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			callback(scanner.Text())
		}
	}()

	// Stream stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			callback("[stderr] " + scanner.Text())
		}
	}()

	return cmd.Wait()
}
