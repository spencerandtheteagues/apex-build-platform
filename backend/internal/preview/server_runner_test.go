package preview

import (
	"context"
	"fmt"
	"testing"

	"apex-build/pkg/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakePortCheckingRuntime struct {
	unavailable map[int]bool
}

func (f *fakePortCheckingRuntime) StartProcess(*ProcessStartConfig) (*ProcessHandle, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakePortCheckingRuntime) Name() string {
	return "fake-port-checker"
}

func (f *fakePortCheckingRuntime) IsPortAvailable(port int) bool {
	return !f.unavailable[port]
}

func TestServerRunnerAllocatePortUsesRuntimeAvailability(t *testing.T) {
	t.Parallel()

	runner := NewServerRunnerWithRuntime(nil, &fakePortCheckingRuntime{
		unavailable: map[int]bool{9100: true, 9101: true},
	})

	port := runner.allocatePort(42)

	if port != 9102 {
		t.Fatalf("port = %d, want 9102", port)
	}
}

func TestDetectNodeServerCommandPrefersDevServer(t *testing.T) {
	t.Parallel()

	command, ok := detectNodeServerCommand(`{
		"scripts": {
			"start": "node dist/server/index.js",
			"dev:server": "tsx watch server/index.ts"
		}
	}`)
	if !ok {
		t.Fatal("expected node server command")
	}
	if command != "npm run dev:server" {
		t.Fatalf("command = %q, want npm run dev:server", command)
	}
}

func TestDetectNodeServerCommandFallsBackToServe(t *testing.T) {
	t.Parallel()

	command, ok := detectNodeServerCommand(`{"scripts":{"serve":"node server.js"}}`)
	if !ok {
		t.Fatal("expected node server command")
	}
	if command != "npm run serve" {
		t.Fatalf("command = %q, want npm run serve", command)
	}
}

func TestDetectNodeServerCommandPrefersNextDevRuntime(t *testing.T) {
	t.Parallel()

	command, ok := detectNodeServerCommand(`{
		"scripts": {
			"dev": "next dev",
			"start": "next start"
		},
		"dependencies": {
			"next": "^15.3.2",
			"react": "^18.3.0",
			"react-dom": "^18.3.0"
		}
	}`)
	if !ok {
		t.Fatal("expected next server command")
	}
	if command != "npm run dev" {
		t.Fatalf("command = %q, want npm run dev", command)
	}
}

func TestDetectNodeServerCommandRunsNextEvenWhenScriptsAreMissing(t *testing.T) {
	t.Parallel()

	command, ok := detectNodeServerCommand(`{
		"dependencies": {
			"next": "^15.3.2",
			"react": "^18.3.0",
			"react-dom": "^18.3.0"
		}
	}`)
	if !ok {
		t.Fatal("expected next server command")
	}
	if command != "npm exec next dev" {
		t.Fatalf("command = %q, want npm exec next dev", command)
	}
}

func TestDetectNodeServerCommandIgnoresNextStartForPreview(t *testing.T) {
	t.Parallel()

	command, ok := detectNodeServerCommand(`{
		"scripts": {
			"start": "next start"
		},
		"dependencies": {
			"next": "^15.3.2",
			"react": "^18.3.0",
			"react-dom": "^18.3.0"
		}
	}`)
	if !ok {
		t.Fatal("expected next server command")
	}
	if command != "npm exec next dev" {
		t.Fatalf("command = %q, want npm exec next dev", command)
	}
}

func TestBuildServerCommandPinsNextDevHostAndPort(t *testing.T) {
	t.Parallel()

	cmd, args, err := buildServerCommand("npm run dev", "app/page.tsx", "next", 9123)
	if err != nil {
		t.Fatalf("build command: %v", err)
	}
	if cmd != "npm" {
		t.Fatalf("cmd = %q, want npm", cmd)
	}
	want := []string{"run", "dev", "--", "--hostname", "0.0.0.0", "--port", "9123"}
	if fmt.Sprint(args) != fmt.Sprint(want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}

	cmd, args, err = buildServerCommand("npm exec next dev", "app/page.tsx", "next", 9124)
	if err != nil {
		t.Fatalf("build command: %v", err)
	}
	want = []string{"exec", "--", "next", "dev", "--hostname", "0.0.0.0", "--port", "9124"}
	if cmd != "npm" || fmt.Sprint(args) != fmt.Sprint(want) {
		t.Fatalf("cmd/args = %q %#v, want npm %#v", cmd, args, want)
	}
}

func TestDetectServerPrefersDevServerForTypescriptBackend(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.File{}); err != nil {
		t.Fatalf("migrate files: %v", err)
	}
	files := []models.File{
		{
			ProjectID: 7,
			Path:      "package.json",
			Name:      "package.json",
			Type:      "file",
			Content: `{
				"dependencies": {"express": "^4.21.2"},
				"devDependencies": {"tsx": "^4.19.3"},
				"scripts": {
					"start": "node dist/server/index.js",
					"dev:server": "tsx watch server/index.ts"
				}
			}`,
		},
		{
			ProjectID: 7,
			Path:      "server/index.ts",
			Name:      "index.ts",
			Type:      "file",
			Content:   `import express from "express";`,
		},
	}
	for _, file := range files {
		if err := db.Create(&file).Error; err != nil {
			t.Fatalf("create file %s: %v", file.Path, err)
		}
	}

	runner := NewServerRunner(db)
	detection, err := runner.DetectServer(context.Background(), 7)
	if err != nil {
		t.Fatalf("detect server: %v", err)
	}
	if !detection.HasBackend {
		t.Fatal("expected backend detection")
	}
	if detection.Command != "npm run dev:server" {
		t.Fatalf("command = %q, want npm run dev:server", detection.Command)
	}
	if detection.EntryFile != "server/index.ts" {
		t.Fatalf("entry file = %q, want server/index.ts", detection.EntryFile)
	}
}

func TestDetectServerTreatsNextAppRouterAsRuntimePreview(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file:next_app_router_runtime?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.File{}); err != nil {
		t.Fatalf("migrate files: %v", err)
	}
	files := []models.File{
		{
			ProjectID: 11,
			Path:      "package.json",
			Name:      "package.json",
			Type:      "file",
			Content: `{
				"scripts": {
					"dev": "next dev",
					"build": "next build",
					"start": "next start"
				},
				"dependencies": {
					"next": "^15.3.2",
					"react": "^18.3.0",
					"react-dom": "^18.3.0"
				}
			}`,
		},
		{
			ProjectID: 11,
			Path:      "app/page.tsx",
			Name:      "page.tsx",
			Type:      "file",
			Content:   `export default function Page() { return <main>Hello</main> }`,
		},
	}
	for _, file := range files {
		if err := db.Create(&file).Error; err != nil {
			t.Fatalf("create file %s: %v", file.Path, err)
		}
	}

	runner := NewServerRunner(db)
	detection, err := runner.DetectServer(context.Background(), 11)
	if err != nil {
		t.Fatalf("detect server: %v", err)
	}
	if !detection.HasBackend {
		t.Fatal("expected next runtime detection")
	}
	if detection.ServerType != "node" {
		t.Fatalf("server type = %q, want node", detection.ServerType)
	}
	if detection.Framework != "next" {
		t.Fatalf("framework = %q, want next", detection.Framework)
	}
	if detection.Command != "npm run dev" {
		t.Fatalf("command = %q, want npm run dev", detection.Command)
	}
	if detection.EntryFile != "app/page.tsx" {
		t.Fatalf("entry file = %q, want app/page.tsx", detection.EntryFile)
	}
}
