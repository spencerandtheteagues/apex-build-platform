package providers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"apex-build/internal/deploy"
)

type recordedCommand struct {
	name string
	args []string
	dir  string
	env  []string
}

func TestRailwayProviderDeployCreatesConfigAndPersistsRefs(t *testing.T) {
	t.Parallel()

	var commands []recordedCommand
	var capturedRailwayConfig string

	provider := &RailwayProvider{
		token: "railway-token",
		run: func(_ context.Context, dir string, env []string, name string, args ...string) (string, error) {
			commands = append(commands, recordedCommand{name: name, args: append([]string(nil), args...), dir: dir, env: append([]string(nil), env...)})
			switch {
			case name == "railway" && len(args) >= 1 && args[0] == "init":
				return `{"id":"proj_123","name":"apex-project-42"}`, nil
			case name == "railway" && len(args) >= 1 && args[0] == "status":
				return `{"projectId":"proj_123","projectName":"apex-project-42"}`, nil
			case name == "railway" && len(args) >= 1 && args[0] == "add":
				return `{"name":"web"}`, nil
			case name == "railway" && len(args) >= 1 && args[0] == "link":
				return `{"linked":true}`, nil
			case name == "railway" && len(args) >= 2 && args[0] == "variable" && args[1] == "set":
				return `{"updated":true}`, nil
			case name == "railway" && len(args) >= 1 && args[0] == "up":
				data, err := os.ReadFile(filepath.Join(dir, "railway.json"))
				if err != nil {
					t.Fatalf("expected generated railway.json: %v", err)
				}
				capturedRailwayConfig = string(data)
				return `{"message":"deploy started"}`, nil
			case name == "railway" && len(args) >= 2 && args[0] == "deployment" && args[1] == "list":
				return `[{"id":"dep_456","status":"SUCCESS"}]`, nil
			case name == "railway" && len(args) >= 1 && args[0] == "domain":
				return `{"domain":"web.up.railway.app"}`, nil
			default:
				return "", fmt.Errorf("unexpected command: %s %s", name, strings.Join(args, " "))
			}
		},
		lookPath: func(file string) (string, error) {
			if file != "railway" {
				t.Fatalf("unexpected binary lookup: %s", file)
			}
			return "/usr/bin/railway", nil
		},
	}

	result, err := provider.Deploy(context.Background(), &deploy.DeploymentConfig{
		ProjectID:    42,
		Provider:     deploy.ProviderRailway,
		Environment:  "production",
		BuildCommand: "npm run build",
		StartCommand: "npm start",
		EnvVars: map[string]string{
			"API_URL":  "https://api.example.com",
			"NODE_ENV": "production",
		},
	}, []deploy.ProjectFile{
		{Path: "/package.json", Content: `{"name":"demo"}`},
		{Path: "/src/main.tsx", Content: `console.log("hello")`},
	})
	if err != nil {
		t.Fatalf("deploy returned error: %v", err)
	}

	if result.URL != "https://web.up.railway.app" {
		t.Fatalf("expected Railway URL, got %q", result.URL)
	}
	if result.Status != deploy.StatusLive {
		t.Fatalf("expected live status, got %s", result.Status)
	}
	if result.Metadata["railway_project"] != "proj_123" {
		t.Fatalf("expected project metadata, got %+v", result.Metadata)
	}
	if result.Metadata["railway_service"] != "web" {
		t.Fatalf("expected service metadata, got %+v", result.Metadata)
	}
	if !strings.Contains(capturedRailwayConfig, `"buildCommand": "npm run build"`) {
		t.Fatalf("expected build command in generated railway.json, got %s", capturedRailwayConfig)
	}
	if !strings.Contains(capturedRailwayConfig, `"startCommand": "npm start"`) {
		t.Fatalf("expected start command in generated railway.json, got %s", capturedRailwayConfig)
	}

	var variableCall *recordedCommand
	for i := range commands {
		cmd := &commands[i]
		if cmd.name == "railway" && len(cmd.args) >= 2 && cmd.args[0] == "variable" && cmd.args[1] == "set" {
			variableCall = cmd
			break
		}
	}
	if variableCall == nil {
		t.Fatalf("expected railway variable set call, got %+v", commands)
	}
	gotArgs := strings.Join(variableCall.args, " ")
	if !strings.Contains(gotArgs, "API_URL=https://api.example.com") || !strings.Contains(gotArgs, "NODE_ENV=production") {
		t.Fatalf("expected variable sync args, got %s", gotArgs)
	}
}

func TestCloudflarePagesProviderDeployBuildsAndDeploysOutputDir(t *testing.T) {
	t.Parallel()

	var commands []recordedCommand

	provider := &CloudflarePagesProvider{
		apiToken:  "cf-token",
		accountID: "acct_123",
		run: func(_ context.Context, dir string, env []string, name string, args ...string) (string, error) {
			commands = append(commands, recordedCommand{name: name, args: append([]string(nil), args...), dir: dir, env: append([]string(nil), env...)})
			switch {
			case name == "sh" && len(args) == 2 && args[0] == "-lc" && args[1] == "npm ci":
				return "installed", nil
			case name == "sh" && len(args) == 2 && args[0] == "-lc" && args[1] == "npm run build":
				if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o755); err != nil {
					t.Fatalf("failed to create dist dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dir, "dist", "index.html"), []byte("<html></html>"), 0o644); err != nil {
					t.Fatalf("failed to create dist artifact: %v", err)
				}
				return "built", nil
			case name == "wrangler" && len(args) >= 4 && args[0] == "pages" && args[1] == "project" && args[2] == "list":
				return `[]`, nil
			case name == "wrangler" && len(args) >= 4 && args[0] == "pages" && args[1] == "project" && args[2] == "create":
				return `created`, nil
			case name == "wrangler" && len(args) >= 3 && args[0] == "pages" && args[1] == "deploy":
				if !strings.HasSuffix(args[2], "/dist") {
					t.Fatalf("expected deploy dir to end with /dist, got %q", args[2])
				}
				return `Deployment complete! https://apex-project-7.pages.dev`, nil
			case name == "wrangler" && len(args) >= 4 && args[0] == "pages" && args[1] == "deployment" && args[2] == "list":
				return `[{"id":"cf_dep_1","status":"success","url":"https://apex-project-7.pages.dev"}]`, nil
			default:
				return "", fmt.Errorf("unexpected command: %s %s", name, strings.Join(args, " "))
			}
		},
		lookPath: func(file string) (string, error) {
			if file != "wrangler" {
				t.Fatalf("unexpected binary lookup: %s", file)
			}
			return "/usr/bin/wrangler", nil
		},
	}

	result, err := provider.Deploy(context.Background(), &deploy.DeploymentConfig{
		ProjectID:    7,
		Provider:     deploy.ProviderCloudflarePages,
		Branch:       "main",
		Framework:    "react",
		InstallCmd:   "npm ci",
		BuildCommand: "npm run build",
		OutputDir:    "dist",
		EnvVars: map[string]string{
			"VITE_API_URL": "https://api.example.com",
		},
	}, []deploy.ProjectFile{
		{Path: "/package.json", Content: `{"name":"demo"}`},
		{Path: "/src/main.tsx", Content: `console.log("hello")`},
	})
	if err != nil {
		t.Fatalf("deploy returned error: %v", err)
	}

	if result.URL != "https://apex-project-7.pages.dev" {
		t.Fatalf("expected Pages URL, got %q", result.URL)
	}
	if result.Status != deploy.StatusLive {
		t.Fatalf("expected live status, got %s", result.Status)
	}
	if result.Metadata["cloudflare_pages_project"] != "apex-project-7" {
		t.Fatalf("expected project metadata, got %+v", result.Metadata)
	}

	sawDeploy := false
	for _, cmd := range commands {
		if cmd.name == "wrangler" && len(cmd.args) >= 3 && cmd.args[0] == "pages" && cmd.args[1] == "deploy" {
			sawDeploy = true
		}
	}
	if !sawDeploy {
		t.Fatalf("expected wrangler pages deploy call, got %+v", commands)
	}
}

func TestCloudflarePagesValidateConfigRejectsRuntimeApps(t *testing.T) {
	t.Parallel()

	provider := &CloudflarePagesProvider{
		apiToken:  "cf-token",
		accountID: "acct_123",
		lookPath: func(file string) (string, error) {
			return "/usr/bin/wrangler", nil
		},
	}

	err := provider.ValidateConfig(&deploy.DeploymentConfig{
		ProjectID:    1,
		Framework:    "nextjs",
		StartCommand: "npm start",
	})
	if err == nil {
		t.Fatal("expected validation error for runtime app")
	}
}
