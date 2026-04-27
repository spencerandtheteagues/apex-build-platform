package execution

import (
	"os/exec"
	"sort"
	"strings"
)

type SandboxToolchainProfile struct {
	AvailableCLIs       []string `json:"available_clis,omitempty"`
	PackageManagers     []string `json:"package_managers,omitempty"`
	DatabaseCLIs        []string `json:"database_clis,omitempty"`
	DeploymentCLIs      []string `json:"deployment_clis,omitempty"`
	BuildTooling        []string `json:"build_tooling,omitempty"`
	Diagnostics         []string `json:"diagnostics,omitempty"`
	NetworkDependentCLI []string `json:"network_dependent_clis,omitempty"`
}

func newSandboxToolchainProfile(
	packageManagers []string,
	databaseCLIs []string,
	deploymentCLIs []string,
	buildTooling []string,
	diagnostics []string,
	networkDependent []string,
) SandboxToolchainProfile {
	profile := SandboxToolchainProfile{
		PackageManagers:     normalizeCLINames(packageManagers),
		DatabaseCLIs:        normalizeCLINames(databaseCLIs),
		DeploymentCLIs:      normalizeCLINames(deploymentCLIs),
		BuildTooling:        normalizeCLINames(buildTooling),
		Diagnostics:         normalizeCLINames(diagnostics),
		NetworkDependentCLI: normalizeCLINames(networkDependent),
	}
	profile.AvailableCLIs = normalizeCLINames(append([]string{},
		append(profile.PackageManagers,
			append(profile.DatabaseCLIs,
				append(profile.DeploymentCLIs,
					append(profile.BuildTooling, profile.Diagnostics...)...)...)...)...,
	))
	return profile
}

func normalizeCLINames(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func mergeToolchainProfiles(profiles ...SandboxToolchainProfile) SandboxToolchainProfile {
	merged := SandboxToolchainProfile{}
	for _, profile := range profiles {
		merged.PackageManagers = append(merged.PackageManagers, profile.PackageManagers...)
		merged.DatabaseCLIs = append(merged.DatabaseCLIs, profile.DatabaseCLIs...)
		merged.DeploymentCLIs = append(merged.DeploymentCLIs, profile.DeploymentCLIs...)
		merged.BuildTooling = append(merged.BuildTooling, profile.BuildTooling...)
		merged.Diagnostics = append(merged.Diagnostics, profile.Diagnostics...)
		merged.NetworkDependentCLI = append(merged.NetworkDependentCLI, profile.NetworkDependentCLI...)
		merged.AvailableCLIs = append(merged.AvailableCLIs, profile.AvailableCLIs...)
	}
	merged.PackageManagers = normalizeCLINames(merged.PackageManagers)
	merged.DatabaseCLIs = normalizeCLINames(merged.DatabaseCLIs)
	merged.DeploymentCLIs = normalizeCLINames(merged.DeploymentCLIs)
	merged.BuildTooling = normalizeCLINames(merged.BuildTooling)
	merged.Diagnostics = normalizeCLINames(merged.Diagnostics)
	merged.NetworkDependentCLI = normalizeCLINames(merged.NetworkDependentCLI)
	merged.AvailableCLIs = normalizeCLINames(merged.AvailableCLIs)
	return merged
}

func fallbackToolchainProfileForLanguage(language string) SandboxToolchainProfile {
	switch language {
	case "python":
		return newSandboxToolchainProfile(
			[]string{"python", "python3", "pip", "pip3"},
			nil,
			nil,
			[]string{"python", "python3"},
			nil,
			nil,
		)
	case "javascript":
		return newSandboxToolchainProfile(
			[]string{"node", "npm", "npx"},
			nil,
			nil,
			[]string{"node", "npm", "npx"},
			nil,
			nil,
		)
	case "go":
		return newSandboxToolchainProfile(
			[]string{"go", "gofmt"},
			nil,
			nil,
			[]string{"go", "gofmt"},
			nil,
			nil,
		)
	case "rust":
		return newSandboxToolchainProfile(
			[]string{"cargo", "rustc"},
			nil,
			nil,
			[]string{"cargo", "rustc"},
			nil,
			nil,
		)
	case "java":
		return newSandboxToolchainProfile(
			[]string{"java", "javac"},
			nil,
			nil,
			[]string{"java", "javac"},
			nil,
			nil,
		)
	case "c":
		return newSandboxToolchainProfile(
			[]string{"gcc", "make"},
			nil,
			nil,
			[]string{"gcc", "make"},
			nil,
			nil,
		)
	case "cpp":
		return newSandboxToolchainProfile(
			[]string{"g++", "make"},
			nil,
			nil,
			[]string{"g++", "make"},
			nil,
			nil,
		)
	default:
		return SandboxToolchainProfile{}
	}
}

func enhancedToolchainProfileForLanguage(language string) SandboxToolchainProfile {
	common := newSandboxToolchainProfile(
		nil,
		[]string{"mysql", "psql", "redis-cli", "sqlite3"},
		nil,
		nil,
		[]string{"curl", "dig", "git", "jq", "nc", "rg", "wget"},
		[]string{"mysql", "psql", "redis-cli"},
	)

	switch language {
	case "python":
		return mergeToolchainProfiles(common, newSandboxToolchainProfile(
			[]string{"pip", "pip3", "pipenv", "poetry", "python", "python3", "uv"},
			nil,
			nil,
			[]string{"python", "python3"},
			nil,
			[]string{"pip", "pip3", "pipenv", "poetry", "uv"},
		))
	case "javascript":
		return mergeToolchainProfiles(common, newSandboxToolchainProfile(
			[]string{"node", "npm", "npx", "pnpm", "yarn"},
			nil,
			[]string{"netlify", "railway", "supabase", "vercel", "wrangler"},
			[]string{"drizzle-kit", "node", "npm", "npx", "prisma", "serve", "tsc", "tsx", "vite"},
			nil,
			[]string{"netlify", "railway", "supabase", "vercel", "wrangler"},
		))
	case "go":
		return mergeToolchainProfiles(common, newSandboxToolchainProfile(
			[]string{"go", "gofmt"},
			nil,
			nil,
			[]string{"go", "gofmt"},
			nil,
			nil,
		))
	case "rust":
		return mergeToolchainProfiles(common, newSandboxToolchainProfile(
			[]string{"cargo", "rustc"},
			nil,
			nil,
			[]string{"cargo", "rustc"},
			nil,
			nil,
		))
	case "java":
		return mergeToolchainProfiles(common, newSandboxToolchainProfile(
			[]string{"gradle", "java", "javac", "mvn"},
			nil,
			nil,
			[]string{"gradle", "java", "javac", "mvn"},
			nil,
			nil,
		))
	case "c":
		return mergeToolchainProfiles(common, newSandboxToolchainProfile(
			[]string{"gcc", "make"},
			nil,
			nil,
			[]string{"gcc", "make"},
			nil,
			nil,
		))
	case "cpp":
		return mergeToolchainProfiles(common, newSandboxToolchainProfile(
			[]string{"g++", "make"},
			nil,
			nil,
			[]string{"g++", "make"},
			nil,
			nil,
		))
	default:
		return common
	}
}

func containerLanguageToolchainProfiles(enhancedLanguages map[string]bool) map[string]SandboxToolchainProfile {
	languages := []string{"python", "javascript", "go", "rust", "java", "c", "cpp"}
	profiles := make(map[string]SandboxToolchainProfile, len(languages))
	for _, language := range languages {
		if enhancedLanguages[language] {
			profiles[language] = enhancedToolchainProfileForLanguage(language)
			continue
		}
		profiles[language] = fallbackToolchainProfileForLanguage(language)
	}
	return profiles
}

func (s *ContainerSandbox) ToolchainProfiles() map[string]SandboxToolchainProfile {
	if s == nil {
		return nil
	}
	s.imageCacheMu.RLock()
	enhanced := make(map[string]bool, len(s.imageCache))
	for language, ready := range s.imageCache {
		if ready {
			enhanced[language] = true
		}
	}
	s.imageCacheMu.RUnlock()
	return containerLanguageToolchainProfiles(enhanced)
}

func (s *ContainerSandbox) ToolchainSummary() SandboxToolchainProfile {
	if s == nil {
		return SandboxToolchainProfile{}
	}
	profiles := s.ToolchainProfiles()
	summary := SandboxToolchainProfile{}
	for _, profile := range profiles {
		summary = mergeToolchainProfiles(summary, profile)
	}
	return summary
}

func e2bToolchainSummary() SandboxToolchainProfile {
	return mergeToolchainProfiles(
		fallbackToolchainProfileForLanguage("javascript"),
		fallbackToolchainProfileForLanguage("python"),
		newSandboxToolchainProfile(
			nil,
			nil,
			nil,
			nil,
			[]string{"curl", "git"},
			[]string{"curl"},
		),
	)
}

func detectHostCLIInventory(catalog []string) []string {
	if len(catalog) == 0 {
		return nil
	}
	available := make([]string, 0, len(catalog))
	for _, cmd := range catalog {
		if _, err := exec.LookPath(cmd); err == nil {
			available = append(available, cmd)
		}
	}
	return normalizeCLINames(available)
}

func sandboxAptPackages(language string) []string {
	packages := []string{
		"bash",
		"build-essential",
		"ca-certificates",
		"curl",
		"default-mysql-client",
		"dnsutils",
		"git",
		"jq",
		"netcat-openbsd",
		"pkg-config",
		"postgresql-client",
		"redis-tools",
		"ripgrep",
		"sqlite3",
		"unzip",
		"wget",
		"xz-utils",
		"zip",
	}

	switch language {
	case "javascript":
		packages = append(packages, "python3", "python3-pip")
	case "java":
		packages = append(packages, "gradle", "maven")
	}

	return normalizeCLINames(packages)
}

func sandboxGlobalInstallCommands(language string) []string {
	switch language {
	case "python":
		return []string{
			"python3 -m pip install --no-cache-dir uv poetry pipenv",
		}
	case "javascript":
		return []string{
			// yarn is pre-installed in node:20-bookworm-slim; installing it again via npm fails with EEXIST
			"npm install -g pnpm typescript tsx vite serve prisma drizzle-kit vercel netlify-cli wrangler @railway/cli supabase",
		}
	default:
		return nil
	}
}

func DefaultAgentCommandCatalog() []string {
	return normalizeCLINames([]string{
		"cargo", "composer", "curl", "drizzle-kit", "g++", "gcc", "git", "go", "gofmt",
		"gradle", "java", "javac", "jq", "make", "mvn", "mysql", "nc", "netlify", "node",
		"npm", "npx", "pip", "pip3", "pipenv", "pnpm", "poetry", "prisma",
		"psql", "python", "python3", "railway", "redis-cli", "rg", "rustc", "serve",
		"sqlite3", "supabase", "tsc", "tsx", "uv", "vercel", "vite", "wget", "wrangler", "yarn",
	})
}
