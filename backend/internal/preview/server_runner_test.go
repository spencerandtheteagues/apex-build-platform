package preview

import (
	"context"
	"testing"

	"apex-build/pkg/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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
