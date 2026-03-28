//go:build integration

package preview

// Integration test (requires npm + vite on PATH).
// Run with: go test ./internal/preview/... -run TestRuntimeVerifier_Integration -tags integration

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestRuntimeVerifier_Integration_ValidReactApp(t *testing.T) {
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not available")
	}

	files := []VerifiableFile{
		{
			Path: "index.html",
			Content: `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Test</title></head>
<body>
  <div id="root"></div>
  <script type="module" src="/src/main.jsx"></script>
</body>
</html>`,
		},
		{
			Path:    "src/main.jsx",
			Content: `import React from 'react'; import { createRoot } from 'react-dom/client'; createRoot(document.getElementById('root')).render(React.createElement('div', null, 'Hello'))`,
		},
		{
			Path:    "src/App.jsx",
			Content: `export default function App() { return React.createElement('div', null, 'App') }`,
		},
		{
			Path: "package.json",
			Content: `{
  "name": "test-app",
  "version": "1.0.0",
  "scripts": { "dev": "vite", "build": "vite build" },
  "dependencies": { "react": "^18.2.0", "react-dom": "^18.2.0" },
  "devDependencies": { "vite": "^5.0.0", "@vitejs/plugin-react": "^4.0.0" }
}`,
		},
		{
			Path:    "vite.config.js",
			Content: `import { defineConfig } from 'vite'; import react from '@vitejs/plugin-react'; export default defineConfig({ plugins: [react()] })`,
		},
	}

	rv := NewRuntimeVerifier()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result := rv.VerifyViteApp(ctx, files)
	if result.Skipped {
		t.Skip("runtime verification skipped (npm/vite not available)")
	}
	if !result.Passed {
		t.Errorf("expected valid React app to pass runtime verification: kind=%s details=%s\nlogs=%s",
			result.FailureKind, result.Details, result.ServerLogs)
	}
}
