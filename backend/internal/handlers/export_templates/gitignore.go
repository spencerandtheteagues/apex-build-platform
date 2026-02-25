package export_templates

// GitignoreForStack returns a .gitignore appropriate for the detected tech stack
func GitignoreForStack(stack string) string {
	base := `# Dependencies
node_modules/
vendor/
.venv/
__pycache__/

# Build output
dist/
build/
*.o
*.exe

# Environment
.env
.env.local
.env.*.local

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Logs
*.log
npm-debug.log*
`
	switch stack {
	case "node", "react", "vue", "next", "svelte":
		return base + `# Node specific
.next/
.nuxt/
.output/
.cache/
coverage/
`
	case "go", "golang":
		return base + `# Go specific
/bin/
*.test
go.sum
`
	case "python", "django", "flask", "fastapi":
		return base + `# Python specific
*.pyc
*.pyo
*.egg-info/
.eggs/
htmlcov/
.pytest_cache/
.mypy_cache/
`
	case "rust":
		return base + `# Rust specific
/target/
Cargo.lock
`
	default:
		return base
	}
}
