package autonomous

import (
	"strings"
	"testing"
)

func TestGeneratePackageJSONIncludesShadcnDepsForReactTailwind(t *testing.T) {
	t.Parallel()

	executor := &Executor{}
	stack := &TechStack{
		Frontend: "React",
		Styling:  "Tailwind",
	}

	manifest := executor.generatePackageJSON("Recovered preview shell", stack)

	for _, snippet := range []string{
		`"server": "tsx server/index.ts"`,
		`"test": "vitest run"`,
		`"test:coverage": "vitest run --coverage"`,
		`"test:e2e": "playwright test"`,
		`"react": "18.2.0"`,
		`"react-dom": "18.2.0"`,
		`"clsx": "2.1.1"`,
		`"class-variance-authority": "0.7.0"`,
		`"tailwind-merge": "2.5.2"`,
		`"@radix-ui/react-slot": "1.1.0"`,
		`"@radix-ui/react-dialog": "1.1.1"`,
		`"tailwindcss": "3.4.3"`,
		`"postcss": "8.4.38"`,
		`"autoprefixer": "10.4.19"`,
		`"tailwindcss-animate": "1.0.7"`,
		`"@testing-library/react": "16.0.0"`,
		`"@testing-library/user-event": "14.5.2"`,
		`"@testing-library/jest-dom": "6.5.0"`,
		`"@playwright/test": "1.47.0"`,
		`"@vitest/coverage-v8": "1.5.0"`,
		`"jsdom": "24.1.1"`,
		`"tsx": "4.19.2"`,
	} {
		if !strings.Contains(manifest, snippet) {
			t.Fatalf("expected manifest to contain %q, got %q", snippet, manifest)
		}
	}
}

func TestGenerateTailwindConfigIncludesAnimatePluginAndSemanticTokens(t *testing.T) {
	t.Parallel()

	executor := &Executor{}
	config := executor.generateTailwindConfig()

	for _, snippet := range []string{
		`import animate from "tailwindcss-animate"`,
		`darkMode: ["class"]`,
		`border: "hsl(var(--border))"`,
		`background: "hsl(var(--background))"`,
		`foreground: "hsl(var(--foreground))"`,
		`plugins: [animate]`,
	} {
		if !strings.Contains(config, snippet) {
			t.Fatalf("expected tailwind config to contain %q, got %q", snippet, config)
		}
	}
}

func TestAllowedCommandsIncludesExpandedToolingCatalog(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"pnpm", "vercel", "railway", "psql", "mysql", "redis-cli", "wrangler", "supabase"} {
		if !allowedCommands[command] {
			t.Fatalf("expected allowed commands to include %q", command)
		}
	}
}
