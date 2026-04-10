package execution

import (
	"strings"
	"testing"
)

func TestEnhancedToolchainProfileForJavaScriptIncludesDeploymentAndDatabaseCLIs(t *testing.T) {
	t.Parallel()

	profile := enhancedToolchainProfileForLanguage("javascript")

	for _, command := range []string{"pnpm", "vercel", "wrangler", "railway", "supabase", "psql", "mysql", "redis-cli", "prisma"} {
		if !containsCLI(profile.AvailableCLIs, command) {
			t.Fatalf("expected javascript toolchain to include %q, got %+v", command, profile.AvailableCLIs)
		}
	}
	if !containsCLI(profile.NetworkDependentCLI, "vercel") {
		t.Fatalf("expected vercel to be marked network-dependent, got %+v", profile.NetworkDependentCLI)
	}
}

func TestGenerateDockerfileIncludesExpandedCLIInstalls(t *testing.T) {
	t.Parallel()

	sandbox := &ContainerSandbox{config: DefaultContainerSandboxConfig()}

	jsDockerfile := sandbox.generateDockerfile("javascript")
	for _, snippet := range []string{"postgresql-client", "default-mysql-client", "redis-tools", "netlify-cli", "@railway/cli", "wrangler"} {
		if !strings.Contains(jsDockerfile, snippet) {
			t.Fatalf("expected javascript dockerfile to contain %q, got %q", snippet, jsDockerfile)
		}
	}

	pythonDockerfile := sandbox.generateDockerfile("python")
	if !strings.Contains(pythonDockerfile, "uv poetry pipenv") {
		t.Fatalf("expected python dockerfile to install uv/poetry/pipenv, got %q", pythonDockerfile)
	}
}

func TestDefaultAgentCommandCatalogIncludesExpandedCLIs(t *testing.T) {
	t.Parallel()

	catalog := DefaultAgentCommandCatalog()
	for _, command := range []string{"pnpm", "vercel", "railway", "psql", "mysql", "redis-cli", "wrangler", "supabase"} {
		if !containsCLI(catalog, command) {
			t.Fatalf("expected command catalog to include %q, got %+v", command, catalog)
		}
	}
}

func containsCLI(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
