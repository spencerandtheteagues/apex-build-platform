package agents

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestValidateFinalBuildReadiness(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}

	t.Run("valid_react_output", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "moneyflow",
  "scripts": {
    "dev": "vite",
    "build": "vite build"
  },
  "dependencies": {
    "react": "^18.3.0",
    "react-dom": "^18.3.0"
  }
}`,
			},
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "src/main.tsx", Content: "import React from 'react';"},
			{Path: "src/App.tsx", Content: "export const App = () => <div>ok</div>;"},
		}

		errs := am.validateFinalBuildReadiness(nil, files)
		if len(errs) != 0 {
			t.Fatalf("expected no readiness errors, got %v", errs)
		}
	})

	t.Run("accepts_frontend_start_plus_build_without_dev", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "moneyflow",
  "scripts": {
    "start": "vite",
    "build": "vite build"
  },
  "dependencies": {
    "react": "^18.3.0",
    "react-dom": "^18.3.0"
  },
  "devDependencies": {
    "vite": "^5.0.0"
  }
}`,
			},
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "src/main.tsx", Content: "import React from 'react';"},
			{Path: "src/App.tsx", Content: "export default function App(){ return <div>ok</div> }"},
		}

		errs := am.validateFinalBuildReadiness(nil, files)
		if containsError(errs, "missing runnable scripts") {
			t.Fatalf("expected start+build to satisfy frontend script rule, got %v", errs)
		}
	})

	t.Run("incomplete_frontend_output", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "moneyflow",
  "dependencies": {
    "uuid": "^9.0.1"
  }
}`,
			},
			{Path: "src/tests/MoneyFlowApp.test.tsx", Content: "describe('x', () => {})"},
			{Path: "src/utils/validation.ts", Content: "export const x = 1"},
		}

		errs := am.validateFinalBuildReadiness(nil, files)
		if len(errs) == 0 {
			t.Fatalf("expected readiness errors for incomplete frontend output")
		}
		if !containsError(errs, "HTML entry point") {
			t.Fatalf("expected missing HTML entry point error, got %v", errs)
		}
		if !containsError(errs, "missing an entry source file") {
			t.Fatalf("expected missing frontend entry source error, got %v", errs)
		}
	})

	t.Run("no_files", func(t *testing.T) {
		t.Parallel()

		errs := am.validateFinalBuildReadiness(nil, nil)
		if len(errs) == 0 {
			t.Fatalf("expected readiness error for empty output")
		}
		if !containsError(errs, "No files were generated") {
			t.Fatalf("unexpected readiness errors: %v", errs)
		}
	})

	t.Run("rejects_unresolved_patch_markers", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "moneyflow",
  "scripts": {
    "dev": "vite",
    "build": "vite build"
  },
  "dependencies": {
    "react": "^18.3.0",
    "react-dom": "^18.3.0"
  }
}`,
			},
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "src/main.tsx", Content: "import React from 'react';"},
			{Path: "src/App.tsx", Content: "const x = 1;\n<<<<<<< SEARCH\nconst a = 1;\n=======\nconst a = 2;\n>>>>>>> REPLACE\nexport default function App(){ return <div>{x}</div> }"},
		}

		errs := am.validateFinalBuildReadiness(nil, files)
		if len(errs) == 0 {
			t.Fatalf("expected readiness errors for unresolved patch markers")
		}
		if !containsError(errs, "unresolved patch/merge markers") {
			t.Fatalf("expected unresolved marker error, got %v", errs)
		}
	})

	t.Run("rejects_frontend_backend_hybrid_source", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "todo-app",
  "scripts": {
    "dev": "vite",
    "build": "vite build"
  },
  "dependencies": {
    "react": "^18.3.0",
    "react-dom": "^18.3.0",
    "express": "^4.18.2"
  }
}`,
			},
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "src/main.tsx", Content: "import React from 'react';"},
			{Path: "src/App.tsx", Content: "import React from 'react';\nimport express from 'express';\nexport default function App(){ return <div/> }"},
		}

		errs := am.validateFinalBuildReadiness(nil, files)
		if !containsError(errs, "mixes frontend React and backend Express code") {
			t.Fatalf("expected hybrid source error, got %v", errs)
		}
	})

	t.Run("rejects_explanatory_prose_appended_to_source", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "todo-app",
  "scripts": {
    "dev": "vite",
    "build": "vite build"
  },
  "dependencies": {
    "react": "^18.3.0",
    "react-dom": "^18.3.0"
  }
}`,
			},
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "src/main.tsx", Content: "import React from 'react';"},
			{Path: "src/components/App.tsx", Content: "export default function App(){ return <div/> }\n\nThis implementation includes:\n1. Input validation\n2. Authentication"},
		}

		errs := am.validateFinalBuildReadiness(nil, files)
		if !containsError(errs, "contains explanatory prose appended to source code") {
			t.Fatalf("expected prose-appended source error, got %v", errs)
		}
	})

	t.Run("accepts_monorepo_apps_web_frontend_entry", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "agency-ops",
  "workspaces": ["apps/web"],
  "scripts": { "build": "npm run build --workspaces" },
  "dependencies": { "react": "^18.3.0", "react-dom": "^18.3.0" }
}`,
			},
			{Path: "apps/web/index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "apps/web/src/main.tsx", Content: "import React from 'react'; import ReactDOM from 'react-dom/client';"},
			{Path: "apps/web/src/App.tsx", Content: "export default function App(){ return <div/> }"},
		}

		errs := am.validateFinalBuildReadiness(nil, files)
		if containsError(errs, "missing an entry source file") {
			t.Fatalf("unexpected missing frontend entry for apps/web monorepo output: %v", errs)
		}
		if containsError(errs, "missing an HTML entry point") {
			t.Fatalf("unexpected missing HTML entry for apps/web monorepo output: %v", errs)
		}
	})

	t.Run("accepts_monorepo_packages_frontend_entry", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "agency-ops",
  "workspaces": ["packages/frontend", "packages/backend"],
  "scripts": { "build": "npm run build --workspaces" },
  "dependencies": { "react": "^18.3.0", "react-dom": "^18.3.0" }
}`,
			},
			{Path: "packages/frontend/index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "packages/frontend/src/main.tsx", Content: "import React from 'react'; import ReactDOM from 'react-dom/client';"},
			{Path: "packages/frontend/src/App.tsx", Content: "export default function App(){ return <div/> }"},
		}

		errs := am.validateFinalBuildReadiness(nil, files)
		if containsError(errs, "missing an entry source file") {
			t.Fatalf("unexpected missing frontend entry for packages/frontend monorepo output: %v", errs)
		}
		if containsError(errs, "missing an HTML entry point") {
			t.Fatalf("unexpected missing HTML entry for packages/frontend monorepo output: %v", errs)
		}
	})

	t.Run("rejects_mixed_backend_persistence_stacks", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "mixed-backend",
  "scripts": { "build": "tsc" },
  "dependencies": { "react": "^18.3.0", "react-dom": "^18.3.0" }
}`,
			},
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "src/main.tsx", Content: "import React from 'react';"},
			{Path: "apps/api/src/db.ts", Content: "import { PrismaClient } from '@prisma/client'; const prisma = new PrismaClient();"},
			{Path: "apps/api/src/models/task.ts", Content: "import { Schema, model } from 'mongoose'; const taskSchema = new Schema({}); export default model('Task', taskSchema);"},
		}

		errs := am.validateFinalBuildReadiness(nil, files)
		if !containsError(errs, "mixes multiple persistence stacks") {
			t.Fatalf("expected mixed persistence stack error, got %v", errs)
		}
	})
}

func containsError(errors []string, want string) bool {
	for _, err := range errors {
		if strings.Contains(err, want) {
			return true
		}
	}
	return false
}

func TestVerifyGeneratedFrontendPreviewReadiness(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}

	t.Run("missing_build_script", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "preview-test",
  "scripts": {
    "dev": "vite"
  }
}`,
			},
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "src/main.tsx", Content: "console.log('x')"},
		}

		errs := am.verifyGeneratedFrontendPreviewReadiness(files, false)
		if !containsError(errs, "missing a build script") {
			t.Fatalf("expected missing build script error, got %v", errs)
		}
	})

	t.Run("no_deps_build_script_succeeds", func(t *testing.T) {
		t.Parallel()

		if _, err := exec.LookPath("npm"); err != nil {
			t.Skip("npm not available")
		}

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "preview-test",
  "private": true,
  "scripts": {
    "build": "node -e \"console.log('ok')\""
  }
}`,
			},
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "src/main.js", Content: "console.log('ok')"},
		}

		errs := am.verifyGeneratedFrontendPreviewReadiness(files, false)
		if len(errs) != 0 {
			t.Fatalf("expected preview verification success, got %v", errs)
		}
	})

	t.Run("invalid_dependency_subpath_fails_preflight", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "preview-test",
  "private": true,
  "scripts": {
    "build": "node -e \"console.log('ok')\""
  },
  "dependencies": {
    "@shadcn/ui/navigation": "^1.0.0"
  }
}`,
			},
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "src/main.js", Content: "console.log('ok')"},
		}

		errs := am.verifyGeneratedFrontendPreviewReadiness(files, false)
		if !containsError(errs, "dependency check failed") {
			t.Fatalf("expected dependency preflight failure, got %v", errs)
		}
		if !containsError(errs, "subpath import") {
			t.Fatalf("expected subpath dependency error, got %v", errs)
		}
	})

	t.Run("missing_imported_dependency_fails_preflight", func(t *testing.T) {
		t.Parallel()

		if _, err := exec.LookPath("npm"); err != nil {
			t.Skip("npm not available")
		}

		files := []GeneratedFile{
			{
				Path: "frontend/package.json",
				Content: `{
  "name": "preview-test",
  "private": true,
  "scripts": {
    "build": "vite build"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  },
  "devDependencies": {
    "vite": "^5.0.0",
    "@vitejs/plugin-react": "^4.0.0",
    "typescript": "^5.0.0"
  }
}`,
			},
			{Path: "frontend/index.html", Content: "<!doctype html><html><body><div id=\"root\"></div><script type=\"module\" src=\"/src/main.tsx\"></script></body></html>"},
			{Path: "frontend/src/main.tsx", Content: "import React from 'react'; import ReactDOM from 'react-dom/client'; import axios from 'axios'; console.log(axios); ReactDOM.createRoot(document.getElementById('root')!).render(<div />);"},
		}

		errs := am.verifyGeneratedFrontendPreviewReadiness(files, false)
		if !containsError(errs, `does not declare dependency "axios"`) {
			t.Fatalf("expected missing imported dependency error, got %v", errs)
		}
	})

	t.Run("ignores_test_only_imports_for_preflight", func(t *testing.T) {
		t.Parallel()

		if _, err := exec.LookPath("npm"); err != nil {
			t.Skip("npm not available")
		}

		files := []GeneratedFile{
			{
				Path: "frontend/package.json",
				Content: `{
  "name": "preview-test",
  "private": true,
  "scripts": { "build": "node -e \"console.log('ok')\"" },
  "dependencies": { "react": "^18.2.0", "react-dom": "^18.2.0" }
}`,
			},
			{Path: "frontend/index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "frontend/src/main.tsx", Content: "import React from 'react'; console.log(React);"},
			{Path: "frontend/src/__tests__/setupTests.ts", Content: "import { jest } from '@jest/globals'; console.log(jest);"},
		}

		errs := am.verifyGeneratedFrontendPreviewReadiness(files, false)
		if containsError(errs, "@jest/globals") {
			t.Fatalf("expected test-only import to be ignored, got %v", errs)
		}
	})

	t.Run("ignores_vitest_config_imports_for_preflight", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{Path: "frontend/src/main.tsx", Content: "import React from 'react'; console.log(React);"},
			{Path: "frontend/vitest.config.ts", Content: "import { defineConfig } from 'vitest/config'; export default defineConfig({});"},
		}
		manifest := previewManifest{
			Dependencies: map[string]string{"react": "^18.2.0"},
		}

		errs := validateGeneratedImportDependencies(files, "frontend/", manifest)
		if containsError(errs, "vitest/config") || containsError(errs, `dependency "vitest"`) {
			t.Fatalf("expected vitest config imports to be ignored, got %v", errs)
		}
	})

	t.Run("frontend_tsc_without_tsconfig_fails_preflight", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "preview-test",
  "private": true,
  "scripts": { "build": "tsc && vite build", "dev": "vite" },
  "dependencies": { "react": "^18.2.0", "react-dom": "^18.2.0" },
  "devDependencies": { "typescript": "^5.0.0", "vite": "^5.0.0" }
}`,
			},
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "src/main.tsx", Content: "import React from 'react'; console.log(React);"},
		}

		errs := am.verifyGeneratedFrontendPreviewReadiness(files, false)
		if !containsError(errs, "build script runs tsc but tsconfig.json is missing") {
			t.Fatalf("expected missing frontend tsconfig preflight error, got %v", errs)
		}
	})
}

func TestVerifyGeneratedBackendBuildReadiness(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}

	t.Run("missing_build_script", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "backend/package.json",
				Content: `{
  "name": "api",
  "scripts": { "dev": "tsx src/server.ts" }
}`,
			},
			{Path: "backend/src/server.ts", Content: "console.log('x')"},
		}

		errs := am.verifyGeneratedBackendBuildReadiness(files)
		if !containsError(errs, "missing a build script") {
			t.Fatalf("expected missing build script error, got %v", errs)
		}
	})

	t.Run("no_deps_build_script_succeeds", func(t *testing.T) {
		t.Parallel()

		if _, err := exec.LookPath("npm"); err != nil {
			t.Skip("npm not available")
		}

		files := []GeneratedFile{
			{
				Path: "backend/package.json",
				Content: `{
  "name": "api",
  "private": true,
  "scripts": { "build": "node -e \"console.log('ok')\"" }
}`,
			},
			{Path: "backend/src/server.js", Content: "console.log('ok')"},
		}

		errs := am.verifyGeneratedBackendBuildReadiness(files)
		if len(errs) != 0 {
			t.Fatalf("expected backend verification success, got %v", errs)
		}
	})

	t.Run("tsc_root_mismatch_fails_preflight", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "backend/package.json",
				Content: `{
  "name": "api",
  "private": true,
  "scripts": { "build": "tsc" },
  "devDependencies": { "typescript": "^5.0.0" }
}`,
			},
			{Path: "backend/src/server.ts", Content: "console.log('ok')"},
			{Path: "tsconfig.json", Content: `{"compilerOptions":{"target":"ES2022"}}`},
		}

		errs := am.verifyGeneratedBackendBuildReadiness(files)
		if !containsError(errs, "backend/tsconfig.json is missing") {
			t.Fatalf("expected backend tsconfig root mismatch error, got %v", errs)
		}
	})
}

func TestNormalizeGeneratedFileContent(t *testing.T) {
	t.Parallel()

	t.Run("repairs_doubled_single_quotes_in_code", func(t *testing.T) {
		t.Parallel()

		in := "import { defineConfig } from ''vite'';\nconst p = ''./src'';\n"
		got := normalizeGeneratedFileContent("vite.config.ts", in)
		if strings.Contains(got, "''vite''") || strings.Contains(got, "''./src''") {
			t.Fatalf("expected doubled single quotes to be repaired, got %q", got)
		}
		if !strings.Contains(got, "from 'vite'") {
			t.Fatalf("expected repaired vite import, got %q", got)
		}
	})

	t.Run("does_not_touch_non_code_without_strong_indicators", func(t *testing.T) {
		t.Parallel()

		in := "author: O''Reilly"
		got := normalizeGeneratedFileContent("notes.txt", in)
		if got != in {
			t.Fatalf("unexpected normalization for non-code content: %q", got)
		}
	})

	t.Run("repairs_pg_query_resultrow_generic_mismatch_pattern", func(t *testing.T) {
		t.Parallel()

		in := `import pg, { QueryResultRow } from 'pg';
export async function query<T extends QueryResultRow>(text: string, params?: any[]): Promise<pg.QueryResult<T>> {
  const result = await pool.query<T>(text, params);
  return result;
}`
		got := normalizeGeneratedFileContent("packages/backend/src/config/database.ts", in)
		if strings.Contains(got, "import pg, { QueryResultRow } from 'pg';") {
			t.Fatalf("expected pg import normalization, got %q", got)
		}
		if !strings.Contains(got, "T extends pg.QueryResultRow = pg.QueryResultRow") {
			t.Fatalf("expected pg generic normalization, got %q", got)
		}
	})

	t.Run("adds_backend_tsconfig_test_excludes_for_build", func(t *testing.T) {
		t.Parallel()

		in := `{
  "compilerOptions": {
    "target": "ES2022"
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}`
		got := normalizeGeneratedFileContent("packages/backend/tsconfig.json", in)
		if !strings.Contains(got, `"src/**/__tests__/**"`) {
			t.Fatalf("expected __tests__ exclude, got %s", got)
		}
		if !strings.Contains(got, `"src/**/*.test.ts"`) {
			t.Fatalf("expected test exclude, got %s", got)
		}
		if !strings.Contains(got, `"node_modules"`) || !strings.Contains(got, `"dist"`) {
			t.Fatalf("expected original excludes to remain, got %s", got)
		}
	})

	t.Run("adds_frontend_module_resolution_for_esnext_jsx", func(t *testing.T) {
		t.Parallel()

		in := `{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "jsx": "react-jsx"
  },
  "include": ["src"]
}`
		got := normalizeGeneratedFileContent("frontend/tsconfig.json", in)
		if !strings.Contains(got, `"moduleResolution": "Node"`) {
			t.Fatalf("expected frontend moduleResolution normalization, got %s", got)
		}
	})
}

func TestCanCreateAutomatedFixTask_DedupesActiveAndRecent(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		Mode:  ModeFast,
		Tasks: []*Task{},
	}

	if !am.canCreateAutomatedFixTask(build, "fix_review_issues") {
		t.Fatalf("expected fix task creation allowed for empty build")
	}

	build.Tasks = append(build.Tasks, &Task{
		ID:        "t1",
		Type:      TaskFix,
		Status:    TaskPending,
		CreatedAt: time.Now(),
		Input: map[string]any{
			"action": "fix_review_issues",
		},
	})
	if am.canCreateAutomatedFixTask(build, "fix_review_issues") {
		t.Fatalf("expected active pending fix task to block duplicate creation")
	}

	build.Tasks[0].Status = TaskCompleted
	build.Tasks[0].CreatedAt = time.Now()
	if am.canCreateAutomatedFixTask(build, "fix_review_issues") {
		t.Fatalf("expected recent completed fix task to block duplicate creation")
	}

	build.Tasks[0].CreatedAt = time.Now().Add(-30 * time.Second)
	if !am.canCreateAutomatedFixTask(build, "fix_review_issues") {
		t.Fatalf("expected old completed fix task to allow new creation")
	}
}

func TestParseTaskOutputFlagsUnterminatedCodeBlock(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	resp := "// File: packages/backend/src/database/seed.ts\n" +
		"```typescript\n" +
		"export async function seed() {\n" +
		"  const"

	out := am.parseTaskOutput(TaskGenerateSchema, resp)
	if len(out.Files) != 1 {
		t.Fatalf("expected one parsed file, got %d", len(out.Files))
	}
	joined := strings.Join(out.Messages, " | ")
	if !strings.Contains(strings.ToLower(joined), "unterminated code block") {
		t.Fatalf("expected parser warning, got %q", joined)
	}

	ok, errs := am.verifyGeneratedCode("build-test", out)
	if ok {
		t.Fatalf("expected verification failure due to parser warning/truncation")
	}
	if !containsError(errs, "unterminated code block") {
		t.Fatalf("expected parser warning surfaced in verification errors, got %v", errs)
	}
}

func TestQuickSyntaxCheckDetectsLikelyTruncatedTypeScriptEOF(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	errs := am.quickSyntaxCheck(GeneratedFile{
		Path:     "packages/backend/src/database/seed.ts",
		Language: "typescript",
		Content:  "export async function seed() {\n  const",
	})
	if !containsError(errs, "Likely truncated source file") {
		t.Fatalf("expected truncation error, got %v", errs)
	}
}

func TestTaskHasRecentTruncationError(t *testing.T) {
	t.Parallel()

	if taskHasRecentTruncationError(nil) {
		t.Fatalf("expected false for nil task")
	}
	task := &Task{
		ErrorHistory: []ErrorAttempt{
			{Error: "some other failure"},
			{Error: "AI response parsing warning: unterminated code block; final file output may be truncated"},
		},
	}
	if !taskHasRecentTruncationError(task) {
		t.Fatalf("expected truncation error detection")
	}
}

func TestCancelAutomatedRecoveryTasksForLoopCap(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID: "b1",
		Tasks: []*Task{
			{ID: "t1", Type: TaskFix, Status: TaskInProgress, Input: map[string]any{"action": "fix_review_issues"}},
			{ID: "t2", Type: TaskTest, Status: TaskPending, Input: map[string]any{"action": "regression_test"}},
			{ID: "t3", Type: TaskReview, Status: TaskPending, Input: map[string]any{"action": "post_fix_review"}},
			{ID: "t4", Type: TaskReview, Status: TaskCompleted, Input: map[string]any{"action": "post_fix_review"}},
			{ID: "t5", Type: TaskGenerateUI, Status: TaskInProgress, Input: map[string]any{"action": "generate_ui"}},
		},
	}

	am.cancelAutomatedRecoveryTasksForLoopCap(build)

	if build.Tasks[0].Status != TaskCancelled {
		t.Fatalf("expected fix task to be cancelled, got %s", build.Tasks[0].Status)
	}
	if build.Tasks[1].Status != TaskCancelled {
		t.Fatalf("expected regression test task to be cancelled, got %s", build.Tasks[1].Status)
	}
	if build.Tasks[2].Status != TaskCancelled {
		t.Fatalf("expected post-fix review task to be cancelled, got %s", build.Tasks[2].Status)
	}
	if build.Tasks[3].Status != TaskCompleted {
		t.Fatalf("expected completed task unchanged, got %s", build.Tasks[3].Status)
	}
	if build.Tasks[4].Status != TaskInProgress {
		t.Fatalf("expected non-recovery task unchanged, got %s", build.Tasks[4].Status)
	}
}

func TestSummarizePreviewInstallFailure(t *testing.T) {
	t.Parallel()

	out := `npm ERR! code E404
npm ERR! 404 Not Found - GET https://registry.npmjs.org/@hooked%2fui - Not found
npm ERR! 404
`
	got := summarizePreviewInstallFailure(out)
	if !strings.Contains(got, "package not found on npm registry") {
		t.Fatalf("expected npm 404 summary, got %q", got)
	}
	if !strings.Contains(got, "@hooked%2fui") && !strings.Contains(got, "@hooked/ui") {
		t.Fatalf("expected package identifier in summary, got %q", got)
	}

	t.Run("prefers_npm_err_lines_over_warning_spam", func(t *testing.T) {
		t.Parallel()

		out := `npm WARN deprecated foo@1.0.0: old
npm WARN deprecated bar@1.0.0: old
npm ERR! code 1
npm ERR! path /tmp/app/node_modules/bcrypt
npm ERR! command failed
npm ERR! command sh -c node-pre-gyp install --fallback-to-build`
		got := summarizePreviewInstallFailure(out)
		if strings.Contains(got, "npm WARN deprecated") {
			t.Fatalf("expected warning spam to be omitted, got %q", got)
		}
		if !strings.Contains(got, "npm ERR! code 1") || !strings.Contains(got, "node-pre-gyp") {
			t.Fatalf("expected npm error lines, got %q", got)
		}
	})
}

func TestSummarizePreviewBuildFailure(t *testing.T) {
	t.Parallel()

	out := `> agency-frontend@1.0.0 build
> vite build

The CJS build of Vite's Node API is deprecated.
src/App.tsx(3,27): error TS2307: Cannot find module './components/dashboard/Dashboard'
error during build:
RollupError: Could not resolve "./components/dashboard/Dashboard"`

	got := summarizePreviewBuildFailure(out)
	if strings.Contains(strings.ToLower(got), "deprecated") {
		t.Fatalf("expected deprecation warning to be filtered, got %q", got)
	}
	if !strings.Contains(got, "TS2307") && !strings.Contains(got, "RollupError") {
		t.Fatalf("expected actionable build error summary, got %q", got)
	}
}
