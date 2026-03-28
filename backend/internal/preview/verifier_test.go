package preview

import (
	"context"
	"testing"
)

func TestVerifier_PassesValidReactApp(t *testing.T) {
	files := []VerifiableFile{
		{
			Path: "index.html",
			Content: `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>App</title></head>
<body>
  <div id="root"></div>
  <script type="module" src="/src/main.tsx"></script>
</body>
</html>`,
		},
		{
			Path: "src/main.tsx",
			Content: `import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'

ReactDOM.createRoot(document.getElementById('root')!).render(<App />)
`,
		},
		{
			Path: "src/App.tsx",
			Content: `export default function App() {
  return <div>Hello World</div>
}
`,
		},
		{
			Path: "package.json",
			Content: `{
  "name": "my-app",
  "scripts": { "dev": "vite", "build": "vite build" },
  "dependencies": { "react": "^18.0.0" }
}`,
		},
		{
			Path:    "vite.config.ts",
			Content: `import { defineConfig } from 'vite'\nexport default defineConfig({ root: '.', })\n`,
		},
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, false)

	if !result.Passed {
		t.Errorf("expected pass, got failure: kind=%s details=%s", result.FailureKind, result.Details)
	}
}

func TestVerifier_FailsMissingEntrypoint(t *testing.T) {
	files := []VerifiableFile{
		{Path: "package.json", Content: `{"name":"x","scripts":{"dev":"vite"},"dependencies":{"react":"^18"}}`},
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, false)

	if result.Passed {
		t.Error("expected failure for missing entrypoint")
	}
	if result.FailureKind != "missing_entrypoint" {
		t.Errorf("expected failure_kind=missing_entrypoint, got %q", result.FailureKind)
	}
	if len(result.RepairHints) == 0 {
		t.Error("expected at least one repair hint")
	}
}

func TestVerifier_FailsBlankEntrypoint(t *testing.T) {
	files := []VerifiableFile{
		{Path: "index.html", Content: "   "},
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, false)

	if result.Passed {
		t.Error("expected failure for blank entrypoint")
	}
	if result.FailureKind != "blank_screen" {
		t.Errorf("expected failure_kind=blank_screen, got %q", result.FailureKind)
	}
}

func TestVerifier_FailsMarkdownFencesInHTML(t *testing.T) {
	files := []VerifiableFile{
		{
			Path:    "index.html",
			Content: "```html\n<!DOCTYPE html>\n<html><body><div id='root'></div></body></html>\n```",
		},
		{
			Path:    "src/main.tsx",
			Content: "import ReactDOM from 'react-dom/client'\nReactDOM.createRoot(document.getElementById('root')!).render(<div/>)\n",
		},
		{
			Path:    "src/App.tsx",
			Content: "export default function App() { return <div>hi</div> }\n",
		},
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, false)

	if result.Passed {
		t.Error("expected failure for markdown fences in HTML")
	}
	if result.FailureKind != "corrupt_content" {
		t.Errorf("expected failure_kind=corrupt_content, got %q", result.FailureKind)
	}
}

func TestVerifier_FailsEmptyHTMLBody(t *testing.T) {
	files := []VerifiableFile{
		{
			Path:    "index.html",
			Content: "<!DOCTYPE html><html><head></head><body></body></html>",
		},
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, false)

	if result.Passed {
		t.Error("expected failure for empty body")
	}
	if result.FailureKind != "blank_screen" {
		t.Errorf("expected blank_screen, got %q", result.FailureKind)
	}
}

func TestVerifier_FailsMissingRenderCall(t *testing.T) {
	files := []VerifiableFile{
		{
			Path:    "src/main.tsx",
			Content: "import React from 'react'\nimport App from './App'\n// forgot to call render\n",
		},
		{
			Path:    "src/App.tsx",
			Content: "export default function App() { return <div>hi</div> }",
		},
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, false)

	if result.Passed {
		t.Error("expected failure for missing render call")
	}
	if result.FailureKind != "missing_entrypoint" {
		t.Errorf("expected missing_entrypoint, got %q", result.FailureKind)
	}
}

func TestVerifier_FailsInvalidPackageJSON(t *testing.T) {
	files := []VerifiableFile{
		{
			Path:    "index.html",
			Content: "<!DOCTYPE html><html><body><div id='root'>content here</div></body></html>",
		},
		{
			Path:    "package.json",
			Content: `{"name": "x"}`, // missing scripts
		},
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, false)

	if result.Passed {
		t.Error("expected failure for missing scripts in package.json")
	}
	if result.FailureKind != "invalid_package_json" {
		t.Errorf("expected invalid_package_json, got %q", result.FailureKind)
	}
}

func TestVerifier_FullStack_FailsMissingBackend(t *testing.T) {
	files := []VerifiableFile{
		{
			Path:    "index.html",
			Content: "<!DOCTYPE html><html><body><div id='root'>content</div></body></html>",
		},
		// No backend entry file
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, true /* isFullStack */)

	if result.Passed {
		t.Error("expected failure for missing backend entry")
	}
	if result.FailureKind != "backend_missing" {
		t.Errorf("expected backend_missing, got %q", result.FailureKind)
	}
}

func TestVerifier_FullStack_FailsNoListenCall(t *testing.T) {
	files := []VerifiableFile{
		{
			Path:    "index.html",
			Content: "<!DOCTYPE html><html><body><div id='root'>content</div></body></html>",
		},
		{
			Path:    "server.js",
			Content: "const express = require('express')\nconst app = express()\napp.get('/', (req, res) => res.send('ok'))\n// missing app.listen",
		},
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, true)

	if result.Passed {
		t.Error("expected failure for missing listen call")
	}
	if result.FailureKind != "backend_no_listen" {
		t.Errorf("expected backend_no_listen, got %q", result.FailureKind)
	}
}

func TestVerifier_FullStack_AcceptsServerIndexTSBackendEntry(t *testing.T) {
	files := []VerifiableFile{
		{
			Path:    "index.html",
			Content: "<!DOCTYPE html><html><body><div id='root'>content</div></body></html>",
		},
		{
			Path: "server/index.ts",
			Content: `import express from 'express'
const app = express()
app.get('/api/health', (_req, res) => res.json({ ok: true }))
app.listen(3001)
`,
		},
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, true)

	if !result.Passed {
		t.Fatalf("expected server/index.ts backend entry to pass static detection, got kind=%s details=%s", result.FailureKind, result.Details)
	}
}

func TestVerifier_FullStack_AcceptsMountedRouteModuleInServerTree(t *testing.T) {
	files := []VerifiableFile{
		{
			Path:    "index.html",
			Content: "<!DOCTYPE html><html><body><div id='root'>content</div></body></html>",
		},
		{
			Path: "server/index.ts",
			Content: `import cors from "cors"
import express from "express"
import apiRouter from "./routes/api"

const app = express()
app.use(cors())
app.use(express.json())
app.use("/api", apiRouter)
app.listen(3001)
`,
		},
		{
			Path: "server/routes/api.ts",
			Content: `import { Router } from "express"

const router = Router()
router.get("/health", (_req, res) => res.json({ ok: true }))
router.post("/auth/login", (_req, res) => res.json({ token: "x" }))

export default router
`,
		},
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, true)

	if !result.Passed {
		t.Fatalf("expected mounted route module to satisfy backend route detection, got kind=%s details=%s", result.FailureKind, result.Details)
	}
}

func TestVerifier_FullStack_AcceptsAppImportWithRoutesInSiblingFile(t *testing.T) {
	files := []VerifiableFile{
		{
			Path:    "index.html",
			Content: "<!DOCTYPE html><html><body><div id='root'>content</div></body></html>",
		},
		{
			Path: "server/index.ts",
			Content: `import app from "./app"

const port = Number(process.env.PORT || 3001)
app.listen(port)
`,
		},
		{
			Path: "server/app.ts",
			Content: `import express from "express"

const app = express()
app.get("/api/health", (_req, res) => res.json({ ok: true }))

export default app
`,
		},
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, true)

	if !result.Passed {
		t.Fatalf("expected sibling app module routes to satisfy backend route detection, got kind=%s details=%s", result.FailureKind, result.Details)
	}
}

func TestVerifier_FullStack_PassesValidExpressApp(t *testing.T) {
	files := []VerifiableFile{
		{
			Path:    "index.html",
			Content: "<!DOCTYPE html><html><body><div id='root'>content here</div></body></html>",
		},
		{
			Path: "server.js",
			Content: `const express = require('express')
const app = express()
app.get('/api/health', (req, res) => res.json({ ok: true }))
app.get('/api/todos', (req, res) => res.json([]))
app.listen(3000, () => console.log('Server running on port 3000'))
`,
		},
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, true)

	if !result.Passed {
		t.Errorf("expected pass, got failure: kind=%s details=%s", result.FailureKind, result.Details)
	}
}

func TestVerifier_HTTPBootCheck_VanillaHTML(t *testing.T) {
	// Only run if context allows (fast)
	files := []VerifiableFile{
		{
			Path: "index.html",
			Content: `<!DOCTYPE html>
<html>
<head><title>Vanilla App</title></head>
<body>
  <h1>Hello, World!</h1>
  <p>This is a vanilla HTML app with no bundler.</p>
</body>
</html>`,
		},
	}

	v := NewVerifier(nil)
	result := v.VerifyFiles(context.Background(), files, false)

	if !result.Passed {
		t.Errorf("vanilla HTML should pass HTTP boot check: kind=%s details=%s", result.FailureKind, result.Details)
	}

	// Verify the HTTP boot check actually ran
	bootCheckRan := false
	for _, c := range result.Checks {
		if c.Name == "http_boot" {
			bootCheckRan = true
			if !c.Passed {
				t.Errorf("http_boot check failed: %s", c.Detail)
			}
		}
	}
	if !bootCheckRan {
		t.Log("http_boot check did not run (vanilla detection may have been skipped)")
	}
}
