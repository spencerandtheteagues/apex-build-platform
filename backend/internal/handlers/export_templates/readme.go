package export_templates

import "fmt"

// ReadmeForProject returns a README.md with setup instructions
func ReadmeForProject(name, description, stack string) string {
	setupCmd := "npm install && npm run dev"
	switch stack {
	case "go", "golang":
		setupCmd = "go mod download && go run ./cmd/..."
	case "python", "django", "flask", "fastapi":
		setupCmd = "pip install -r requirements.txt && python main.py"
	case "rust":
		setupCmd = "cargo build && cargo run"
	}

	return fmt.Sprintf(`# %s

%s

## Quick Start

`+"```bash"+`
# Clone the repository
git clone <repo-url>
cd %s

# Install dependencies and start
%s
`+"```"+`

## Tech Stack

- **Framework:** %s
- **Built with:** [APEX.BUILD](https://apex.build)

## Project Structure

See the source files for the full project layout.

## License

MIT
`, name, description, name, setupCmd, stack)
}
