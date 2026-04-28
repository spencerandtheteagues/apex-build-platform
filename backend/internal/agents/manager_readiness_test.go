package agents

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	"apex-build/internal/ai"
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
			{Path: "README.md", Content: "# MoneyFlow\n\n## Setup\n```bash\nnpm install && npm run dev\n```\n"},
			{Path: ".env.example", Content: "VITE_API_URL=http://localhost:3001\n"},
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

	t.Run("static_frontend_without_env_usage_does_not_require_env_example", func(t *testing.T) {
		t.Parallel()

		build := &Build{
			Mode: "balanced",
			TechStack: &TechStack{
				Frontend: "Next.js",
			},
		}
		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "marketing-site",
  "scripts": {
    "dev": "next dev",
    "build": "next build"
  },
  "dependencies": {
    "next": "^14.2.0",
    "react": "^18.3.0",
    "react-dom": "^18.3.0"
  }
}`,
			},
			{Path: "app/page.tsx", Content: "export default function Page(){ return <main>hello</main> }"},
			{Path: "README.md", Content: "# Marketing Site\n\n## Run\nnpm install && npm run dev\n"},
		}

		errs := am.validateFinalBuildReadiness(build, files)
		if containsError(errs, "missing_deliverable: .env.example") {
			t.Fatalf("expected static frontend without env usage to skip .env.example requirement, got %v", errs)
		}
	})

	t.Run("frontend_env_usage_still_requires_env_example", func(t *testing.T) {
		t.Parallel()

		build := &Build{
			Mode: "balanced",
			TechStack: &TechStack{
				Frontend: "React",
			},
		}
		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "dashboard",
  "scripts": {
    "dev": "vite",
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
			{Path: "src/main.tsx", Content: "console.log(import.meta.env.VITE_API_URL)"},
			{Path: "src/App.tsx", Content: "export default function App(){ return <div>ok</div> }"},
			{Path: "README.md", Content: "# Dashboard\n\n## Run\nnpm install && npm run dev\n"},
		}

		errs := am.validateFinalBuildReadiness(build, files)
		if !containsError(errs, "missing_deliverable: .env.example") {
			t.Fatalf("expected env-reading frontend to require .env.example, got %v", errs)
		}
	})

	t.Run("frontend_preview_only_fullstack_skips_backend_runtime_proof", func(t *testing.T) {
		t.Parallel()

		build := &Build{
			Mode:             ModeFull,
			SubscriptionPlan: "builder",
			Plan: &BuildPlan{
				AppType:      "fullstack",
				DeliveryMode: "frontend_preview_only",
			},
			TechStack: &TechStack{
				Frontend: "React",
				Backend:  "Express",
				Database: "PostgreSQL",
			},
			Agents: map[string]*Agent{
				"frontend-1": {ID: "frontend-1", Role: RoleFrontend, Provider: ai.ProviderClaude},
			},
		}
		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "preview-stage",
  "scripts": {
    "build": "vite build",
    "preview": "vite preview"
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
			{Path: "src/main.tsx", Content: "import React from 'react'; import ReactDOM from 'react-dom/client';"},
			{Path: "src/App.tsx", Content: "export default function App(){ return <div>ok</div> }"},
			{Path: "README.md", Content: "# Preview stage\n"},
			{Path: ".env.example", Content: "VITE_API_URL=http://localhost:3001\n"},
		}

		errs := am.validateFinalBuildReadiness(build, files)
		for _, err := range errs {
			lower := strings.ToLower(err)
			if strings.Contains(lower, "backend") || strings.Contains(lower, "integration:") {
				t.Fatalf("expected frontend preview checkpoint to skip backend/integration proof, got %v", errs)
			}
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

	t.Run("rejects_untouched_deterministic_scaffold_placeholder", func(t *testing.T) {
		t.Parallel()

		build := &Build{
			Mode: "fast",
			TechStack: &TechStack{
				Frontend: "React",
			},
		}
		files := []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "pulseboard",
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
			{Path: "src/App.tsx", Content: "export default function App(){ return <main><p>Bootstrapped by APEX.BUILD</p><p>The deterministic scaffold is live. Replace this shell with the real experience.</p></main> }"},
		}

		errs := am.validateFinalBuildReadiness(build, files)
		if !containsError(errs, "deterministic scaffold placeholder content") {
			t.Fatalf("expected scaffold placeholder error, got %v", errs)
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

func TestApplyDeterministicMissingFrontendShellRepairAddsShadcnScaffold(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID: "missing-frontend-shell-shadcn",
		TechStack: &TechStack{
			Frontend: "React",
			Backend:  "Express",
			Styling:  "Tailwind CSS",
		},
		Tasks: []*Task{
			{
				ID:     "backend-only-output",
				Type:   TaskGenerateAPI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "package.json",
							Content: `{
  "name": "backend-only-output",
  "private": true,
  "type": "module",
  "scripts": {
    "start": "node server/index.js"
  },
  "dependencies": {
    "express": "^4.18.2"
  }
}`,
						},
						{
							Path: "server/index.js",
							Content: `import express from "express";
const app = express();
app.get("/api/health", (_req, res) => res.json({ status: "ok" }));
app.listen(process.env.PORT || 3001);`,
						},
					},
				},
			},
		},
	}

	bundle, summary := am.applyDeterministicMissingFrontendShellRepair(build, []string{
		"No recognized frontend entry point found (index.html, src/main.tsx, src/index.tsx, etc.).",
	})
	if bundle == nil {
		t.Fatal("expected deterministic frontend shell repair to produce a patch bundle")
	}
	if !strings.Contains(summary, "generated previewable frontend shell") {
		t.Fatalf("expected summary to describe frontend shell repair, got %q", summary)
	}

	requiredCreates := map[string]bool{
		"components.json":              false,
		"src/lib/utils.ts":             false,
		"src/components/ui/button.tsx": false,
		"src/components/ui/card.tsx":   false,
		"src/components/ui/input.tsx":  false,
		"src/components/ui/badge.tsx":  false,
		"src/components/ui/dialog.tsx": false,
		"src/main.tsx":                 false,
		"src/App.tsx":                  false,
		"src/index.css":                false,
		"tailwind.config.js":           false,
	}

	for _, op := range bundle.Operations {
		if op.Type == PatchCreateFile {
			if _, ok := requiredCreates[op.Path]; ok {
				requiredCreates[op.Path] = true
			}
		}
		if op.Path == "package.json" && strings.Contains(op.Content, `"tailwindcss-animate"`) && strings.Contains(op.Content, `"@radix-ui/react-dialog"`) {
			requiredCreates["tailwind.config.js"] = requiredCreates["tailwind.config.js"]
		}
	}

	for path, found := range requiredCreates {
		if !found {
			t.Fatalf("expected create_file operation for %s, got %+v", path, bundle.Operations)
		}
	}

	var manifestPatched bool
	for _, op := range bundle.Operations {
		if op.Path == "package.json" && strings.Contains(op.Content, `"tailwindcss-animate"`) && strings.Contains(op.Content, `"@radix-ui/react-dialog"`) && strings.Contains(op.Content, `"clsx"`) {
			manifestPatched = true
			break
		}
	}
	if !manifestPatched {
		t.Fatalf("expected package.json patch to include shadcn dependencies, got %+v", bundle.Operations)
	}
}

func TestValidateFinalBuildReadinessEmitsSurfaceVerificationReports(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-surface-reports",
		Mode:      ModeFull,
		TechStack: &TechStack{Frontend: "React"},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				BuildContract: &BuildContract{
					TruthBySurface: map[string][]TruthTag{
						string(SurfaceFrontend):   {TruthScaffolded},
						string(SurfaceDeployment): {TruthScaffolded},
					},
				},
			},
		},
	}

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
		{Path: "src/App.tsx", Content: "export default function App(){ return <div>ok</div> }"},
		{Path: "README.md", Content: "# MoneyFlow\n\n## Setup\n```bash\nnpm install && npm run dev\n```\n"},
		{Path: ".env.example", Content: "VITE_API_URL=http://localhost:3001\n"},
	}

	if errs := am.validateFinalBuildReadiness(build, files); len(errs) != 0 {
		t.Fatalf("expected no readiness errors, got %v", errs)
	}

	state := build.SnapshotState.Orchestration
	if state == nil {
		t.Fatal("expected orchestration state")
	}
	if len(state.VerificationReports) < 3 {
		t.Fatalf("expected verification reports, got %+v", state.VerificationReports)
	}

	var frontendReport *VerificationReport
	for i := range state.VerificationReports {
		report := &state.VerificationReports[i]
		if report.Surface == SurfaceFrontend {
			frontendReport = report
			break
		}
	}
	if frontendReport == nil {
		t.Fatalf("expected frontend verification report, got %+v", state.VerificationReports)
	}
	if frontendReport.Status != VerificationPassed {
		t.Fatalf("frontend verification status = %s, want %s", frontendReport.Status, VerificationPassed)
	}
	if containsTruthTag(state.BuildContract.TruthBySurface[string(SurfaceFrontend)], TruthScaffolded) {
		t.Fatalf("expected scaffolded tag to be cleared after verification, got %+v", state.BuildContract.TruthBySurface[string(SurfaceFrontend)])
	}
	if !containsTruthTag(state.BuildContract.TruthBySurface[string(SurfaceFrontend)], TruthVerified) {
		t.Fatalf("expected verified tag after verification, got %+v", state.BuildContract.TruthBySurface[string(SurfaceFrontend)])
	}
}

func TestFinalValidationRepairHintsIncludesScaffoldReplacementGuidance(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}

	hints := am.finalValidationRepairHints([]string{
		"src/App.tsx still contains deterministic scaffold placeholder content; replace the starter UI with the requested app",
	}, nil, 0)

	if !containsError(hints, "Replace the untouched deterministic scaffold with the requested product UI") {
		t.Fatalf("expected scaffold replacement hint, got %v", hints)
	}
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

	t.Run("missing_local_component_import_fails_preflight", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{
				Path: "package.json",
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
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "@vitejs/plugin-react": "^4.0.0",
    "typescript": "^5.0.0",
    "vite": "^5.0.0"
  }
}`,
			},
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div><script type=\"module\" src=\"/src/main.tsx\"></script></body></html>"},
			{Path: "src/main.tsx", Content: "import React from 'react'; import ReactDOM from 'react-dom/client'; import App from './App'; ReactDOM.createRoot(document.getElementById('root')!).render(<App />);"},
			{Path: "src/App.tsx", Content: "import Projects from './pages/Projects'; export default function App(){ return <Projects /> }"},
			{Path: "src/pages/Projects.tsx", Content: "import KanbanColumn from '../components/projects/KanbanColumn'; export default function Projects(){ return <KanbanColumn /> }"},
			{Path: "vite.config.ts", Content: "import { defineConfig } from 'vite'; import react from '@vitejs/plugin-react'; export default defineConfig({ plugins: [react()] });"},
			{Path: "tsconfig.json", Content: `{"compilerOptions":{"jsx":"react-jsx","module":"ESNext","target":"ES2020","moduleResolution":"Bundler","strict":true,"noEmit":true},"include":["src"]}`},
		}

		errs := am.verifyGeneratedFrontendPreviewReadiness(files, false)
		if !containsError(errs, `source imports local module "../components/projects/KanbanColumn"`) {
			t.Fatalf("expected missing local module preflight error, got %v", errs)
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

	t.Run("selects_monorepo_frontend_package_for_preflight", func(t *testing.T) {
		t.Parallel()

		if _, err := exec.LookPath("npm"); err != nil {
			t.Skip("npm not available")
		}

		files := []GeneratedFile{
			{
				Path: "apps/web/package.json",
				Content: `{
  "name": "web",
  "private": true,
  "scripts": { "build": "node -e \"console.log('ok')\"" },
  "dependencies": { "react": "^18.2.0", "react-dom": "^18.2.0" }
}`,
			},
			{Path: "apps/web/index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
			{Path: "apps/web/src/main.tsx", Content: "import React from 'react'; import ReactDOM from 'react-dom/client'; ReactDOM.createRoot(document.getElementById('root')!).render(<div />);"},
			{Path: "backend/src/server.ts", Content: "import bcrypt from 'bcrypt'; import bodyParser from 'body-parser'; console.log(bcrypt, bodyParser);"},
		}

		errs := am.verifyGeneratedFrontendPreviewReadiness(files, false)
		if containsError(errs, `dependency "bcrypt"`) || containsError(errs, `dependency "body-parser"`) {
			t.Fatalf("expected backend imports to be ignored by frontend preflight, got %v", errs)
		}
	})
}

func TestVerificationNeedsNodeInstall(t *testing.T) {
	t.Parallel()

	t.Run("skips_install_for_node_eval_build", func(t *testing.T) {
		t.Parallel()

		manifest := previewManifest{
			Scripts: map[string]string{
				"build": `node -e "console.log('ok')"`,
			},
			Dependencies: map[string]string{
				"react": "^18.2.0",
			},
		}

		if verificationNeedsNodeInstall(manifest, false, false) {
			t.Fatalf("expected node -e build to skip dependency install")
		}
	})

	t.Run("installs_for_toolchain_builds", func(t *testing.T) {
		t.Parallel()

		manifest := previewManifest{
			Scripts: map[string]string{
				"build": "vite build",
			},
			Dependencies: map[string]string{
				"react": "^18.2.0",
			},
			DevDependencies: map[string]string{
				"vite": "^5.0.0",
			},
		}

		if !verificationNeedsNodeInstall(manifest, false, false) {
			t.Fatalf("expected vite build to require dependency install")
		}
	})

	t.Run("installs_for_preview_probe_commands", func(t *testing.T) {
		t.Parallel()

		manifest := previewManifest{
			Scripts: map[string]string{
				"build":   `node -e "console.log('ok')"`,
				"preview": "vite preview",
			},
			Dependencies: map[string]string{
				"react": "^18.2.0",
			},
			DevDependencies: map[string]string{
				"vite": "^5.0.0",
			},
		}

		if !verificationNeedsNodeInstall(manifest, false, true) {
			t.Fatalf("expected preview probe to require dependency install")
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

		errs := am.verifyGeneratedBackendBuildReadiness(files, false)
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

		errs := am.verifyGeneratedBackendBuildReadiness(files, false)
		if len(errs) != 0 {
			t.Fatalf("expected backend verification success, got %v", errs)
		}
	})

	t.Run("missing_runtime_script_fails_when_runtime_proof_required", func(t *testing.T) {
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

		errs := am.verifyGeneratedBackendBuildReadiness(files, true)
		if !containsError(errs, "missing a runnable start/dev/serve script") {
			t.Fatalf("expected missing runtime script error, got %v", errs)
		}
	})

	t.Run("start_script_runtime_probe_succeeds", func(t *testing.T) {
		t.Parallel()

		if _, err := exec.LookPath("npm"); err != nil {
			t.Skip("npm not available")
		}
		if !canBindLocalhostPort() {
			t.Skip("localhost bind unavailable in this environment")
		}

		files := []GeneratedFile{
			{
				Path: "backend/package.json",
				Content: `{
  "name": "api",
  "private": true,
  "scripts": {
    "build": "node -e \"console.log('ok')\"",
    "start": "node -e \"require('http').createServer((req,res)=>{res.statusCode=req.url==='/health'?200:404;res.end('ok')}).listen(process.env.PORT,'127.0.0.1')\""
  }
}`,
			},
			{Path: "backend/src/server.js", Content: "console.log('ok')"},
		}

		errs := am.verifyGeneratedBackendBuildReadiness(files, false)
		if len(errs) != 0 {
			t.Fatalf("expected backend runtime probe success, got %v", errs)
		}
	})

	t.Run("start_script_runtime_probe_404_fails", func(t *testing.T) {
		t.Parallel()

		if _, err := exec.LookPath("npm"); err != nil {
			t.Skip("npm not available")
		}
		if !canBindLocalhostPort() {
			t.Skip("localhost bind unavailable in this environment")
		}

		files := []GeneratedFile{
			{
				Path: "backend/package.json",
				Content: `{
  "name": "api",
  "private": true,
  "scripts": {
    "build": "node -e \"console.log('ok')\"",
    "start": "node -e \"require('http').createServer((req,res)=>{res.statusCode=404;res.end('missing')}).listen(process.env.PORT,'127.0.0.1')\""
  }
}`,
			},
			{Path: "backend/src/server.js", Content: "console.log('ok')"},
		}

		errs := am.verifyGeneratedBackendBuildReadiness(files, false)
		if !containsError(errs, "Backend runtime probe failed: /health returned HTTP 404") {
			t.Fatalf("expected backend runtime 404 failure, got %v", errs)
		}
	})

	t.Run("dev_script_runtime_probe_succeeds_without_start", func(t *testing.T) {
		t.Parallel()

		if _, err := exec.LookPath("npm"); err != nil {
			t.Skip("npm not available")
		}
		if !canBindLocalhostPort() {
			t.Skip("localhost bind unavailable in this environment")
		}

		files := []GeneratedFile{
			{
				Path: "backend/package.json",
				Content: `{
  "name": "api",
  "private": true,
  "scripts": {
    "build": "node -e \"console.log('ok')\"",
    "dev": "node -e \"require('http').createServer((req,res)=>{res.statusCode=req.url==='/ready'?200:404;res.end('ok')}).listen(process.env.PORT,'127.0.0.1')\""
  }
}`,
			},
			{Path: "backend/src/server.js", Content: "console.log('ok')"},
		}

		errs := am.verifyGeneratedBackendBuildReadiness(files, false)
		if len(errs) != 0 {
			t.Fatalf("expected backend dev runtime probe success, got %v", errs)
		}
	})

	t.Run("python_runtime_probe_succeeds", func(t *testing.T) {
		// No t.Parallel() — spawns a real Python process and binds a port;
		// running concurrently with other probe tests causes port/timing conflicts.
		if _, err := exec.LookPath("python3"); err != nil {
			t.Skip("python3 not available")
		}
		if !canBindLocalhostPort() {
			t.Skip("localhost bind unavailable in this environment")
		}

		files := []GeneratedFile{
			{
				Path: "main.py",
				Content: `import os
from http.server import BaseHTTPRequestHandler, HTTPServer

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/ready":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok")
            return
        self.send_response(404)
        self.end_headers()

    def log_message(self, format, *args):
        return

if __name__ == "__main__":
    port = int(os.environ.get("PORT", "8000"))
    HTTPServer(("127.0.0.1", port), Handler).serve_forever()
`,
			},
		}

		errs := am.verifyGeneratedBackendBuildReadiness(files, false)
		if len(errs) != 0 {
			t.Fatalf("expected python runtime probe success, got %v", errs)
		}
	})

	t.Run("python_runtime_probe_404_fails", func(t *testing.T) {
		// No t.Parallel() — spawns a real Python process; must not run concurrently with other probe tests.
		if _, err := exec.LookPath("python3"); err != nil {
			t.Skip("python3 not available")
		}
		if !canBindLocalhostPort() {
			t.Skip("localhost bind unavailable in this environment")
		}

		files := []GeneratedFile{
			{
				Path: "main.py",
				Content: `import os
from http.server import BaseHTTPRequestHandler, HTTPServer

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(404)
        self.end_headers()

    def log_message(self, format, *args):
        return

if __name__ == "__main__":
    port = int(os.environ.get("PORT", "8000"))
    HTTPServer(("127.0.0.1", port), Handler).serve_forever()
`,
			},
		}

		errs := am.verifyGeneratedBackendBuildReadiness(files, false)
		if !containsError(errs, "Python backend runtime probe failed: /health returned HTTP 404") {
			t.Fatalf("expected python runtime 404 failure, got %v", errs)
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

		errs := am.verifyGeneratedBackendBuildReadiness(files, false)
		if !containsError(errs, "backend/tsconfig.json is missing") {
			t.Fatalf("expected backend tsconfig root mismatch error, got %v", errs)
		}
	})
}

func TestDetectBackendHealthProbePaths(t *testing.T) {
	t.Parallel()

	t.Run("defaults_to_health", func(t *testing.T) {
		t.Parallel()

		got := detectBackendHealthProbePaths([]GeneratedFile{
			{Path: "backend/src/server.ts", Content: `app.get("/ready", (_req, res) => res.send("ok"))`},
		})
		want := []string{"/ready", "/health", "/api/health", "/healthz", "/status", "/api/status", "/api/ready", "/"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("unexpected default probe paths: got %v want %v", got, want)
		}
	})

	t.Run("prefers_api_health_when_present", func(t *testing.T) {
		t.Parallel()

		got := detectBackendHealthProbePaths([]GeneratedFile{
			{Path: "backend/src/server.ts", Content: `app.get("/api/health", (_req, res) => res.send("ok"))`},
		})
		if len(got) == 0 || got[0] != "/api/health" {
			t.Fatalf("expected /api/health to be probed first, got %v", got)
		}
		if got[len(got)-1] != "/" {
			t.Fatalf("expected root fallback last, got %v", got)
		}
	})
}

func TestDetectGoRuntimeTarget(t *testing.T) {
	t.Parallel()

	t.Run("prefers_known_candidate_order", func(t *testing.T) {
		t.Parallel()

		got := detectGoRuntimeTarget([]GeneratedFile{
			{Path: "cmd/api/main.go", Content: "package main\n\nfunc main() {}\n"},
			{Path: "main.go", Content: "package main\n\nfunc main() {}\n"},
			{Path: "helpers.go", Content: "package main\n\nfunc helper() {}\n"},
		})
		if got != "." {
			t.Fatalf("expected top-level package target to win, got %q", got)
		}
	})

	t.Run("falls_back_to_nested_main_package", func(t *testing.T) {
		t.Parallel()

		got := detectGoRuntimeTarget([]GeneratedFile{
			{Path: "internal/app/main.go", Content: "package main\n\nfunc main() {}\n"},
			{Path: "internal/app/server.go", Content: "package main\n\nfunc helper() {}\n"},
		})
		if got != "./internal/app" {
			t.Fatalf("expected fallback package target, got %q", got)
		}
	})
}

func TestDetectBackendRuntimeScript(t *testing.T) {
	t.Parallel()

	t.Run("prefers_start_over_dev", func(t *testing.T) {
		t.Parallel()

		got := detectBackendRuntimeScript(map[string]string{
			"dev":   "tsx watch src/server.ts",
			"start": "node dist/server.js",
		})
		if got != "start" {
			t.Fatalf("expected start to win, got %q", got)
		}
	})

	t.Run("falls_back_to_dev_server", func(t *testing.T) {
		t.Parallel()

		got := detectBackendRuntimeScript(map[string]string{
			"build":      "tsc",
			"dev:server": "tsx watch src/server.ts",
		})
		if got != "dev:server" {
			t.Fatalf("expected dev:server fallback, got %q", got)
		}
	})

	t.Run("ignores_non_runtime_scripts", func(t *testing.T) {
		t.Parallel()

		got := detectBackendRuntimeScript(map[string]string{
			"build": "tsc",
			"test":  "vitest run",
			"lint":  "eslint .",
		})
		if got != "" {
			t.Fatalf("expected no runtime script, got %q", got)
		}
	})
}

func TestDetectPythonRuntimeEntry(t *testing.T) {
	t.Parallel()

	t.Run("prefers_main_py", func(t *testing.T) {
		t.Parallel()

		got := detectPythonRuntimeEntry([]GeneratedFile{
			{Path: "server.py", Content: "if __name__ == \"__main__\":\n    app.run()\n"},
			{Path: "main.py", Content: "if __name__ == \"__main__\":\n    app.run()\n"},
		})
		if got != "main.py" {
			t.Fatalf("expected main.py to win, got %q", got)
		}
	})

	t.Run("falls_back_to_nested_server", func(t *testing.T) {
		t.Parallel()

		got := detectPythonRuntimeEntry([]GeneratedFile{
			{Path: "services/api.py", Content: "if __name__ == \"__main__\":\n    app.run()\n"},
		})
		if got != "services/api.py" {
			t.Fatalf("expected nested api.py fallback, got %q", got)
		}
	})
}

func TestClassifyPythonRuntimeProbeFailure(t *testing.T) {
	t.Parallel()

	t.Run("declared_dependency_missing_on_host_skips", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{Path: "main.py", Content: "import fastapi\n"},
			{Path: "requirements.txt", Content: "fastapi>=0.115.0\nuvicorn[standard]>=0.34.0\n"},
		}
		skip, summary := classifyPythonRuntimeProbeFailure(files, "Traceback...\nModuleNotFoundError: No module named 'fastapi'")
		if !skip {
			t.Fatalf("expected declared dependency to skip verifier")
		}
		if !strings.Contains(summary, "fastapi") {
			t.Fatalf("expected summary to preserve missing module, got %q", summary)
		}
	})

	t.Run("local_module_missing_remains_failure", func(t *testing.T) {
		t.Parallel()

		files := []GeneratedFile{
			{Path: "app/__init__.py", Content: ""},
			{Path: "main.py", Content: "import app.routes\n"},
		}
		skip, _ := classifyPythonRuntimeProbeFailure(files, "Traceback...\nModuleNotFoundError: No module named 'app.routes'")
		if skip {
			t.Fatalf("expected missing local module to remain a real failure")
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

	t.Run("unwraps_file_label_and_markdown_fence_for_config_files", func(t *testing.T) {
		t.Parallel()

		in := "// File: postcss.config.js\n" +
			"```javascript\n" +
			"export default {\n" +
			"  plugins: {\n" +
			"    tailwindcss: {},\n" +
			"    autoprefixer: {},\n" +
			"  },\n" +
			"};\n" +
			"```\n"
		got := normalizeGeneratedFileContent("postcss.config.js", in)
		if strings.Contains(got, "```") || strings.Contains(got, "// File:") {
			t.Fatalf("expected markdown wrapper to be removed, got %q", got)
		}
		if !strings.Contains(got, "export default") || !strings.Contains(got, "tailwindcss") {
			t.Fatalf("expected executable postcss config, got %q", got)
		}
	})

	t.Run("unwraps_leading_file_label_without_fence", func(t *testing.T) {
		t.Parallel()

		in := `// File: src/App.tsx
import React from 'react';

export default function App() {
  return <h1>Preview Smoke Pass</h1>;
}`
		got := normalizeGeneratedFileContent("src/App.tsx", in)
		if strings.Contains(got, "// File:") {
			t.Fatalf("expected file label to be removed, got %q", got)
		}
		if !strings.Contains(got, "export default function App()") {
			t.Fatalf("expected component body to remain intact, got %q", got)
		}
	})

	t.Run("strips_task_protocol_tags_from_generated_source", func(t *testing.T) {
		t.Parallel()

		in := "import React from 'react';\n<task_completion_report>{\"summary\":\"done\"}</task_completion_report>\nexport default function App() {\n  return <h1>Preview Smoke Pass</h1>;\n}\n"
		got := normalizeGeneratedFileContent("src/App.tsx", in)
		if strings.Contains(got, "task_completion_report") {
			t.Fatalf("expected task protocol tags to be removed, got %q", got)
		}
		if !strings.Contains(got, "export default function App()") {
			t.Fatalf("expected component body to remain intact, got %q", got)
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
		if !strings.Contains(got, `"moduleResolution": "Bundler"`) {
			t.Fatalf("expected frontend moduleResolution normalization, got %s", got)
		}
	})

	t.Run("rewrites_frontend_nodenext_module_pair_back_to_bundler", func(t *testing.T) {
		t.Parallel()

		in := `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "jsx": "react-jsx"
  },
  "include": ["src"]
}`
		got := normalizeGeneratedFileContent("tsconfig.json", in)
		if !strings.Contains(got, `"module": "ESNext"`) {
			t.Fatalf("expected frontend module to normalize back to ESNext, got %s", got)
		}
		if !strings.Contains(got, `"moduleResolution": "Bundler"`) {
			t.Fatalf("expected frontend moduleResolution to normalize back to Bundler, got %s", got)
		}
		if strings.Contains(got, `"module": "NodeNext"`) || strings.Contains(got, `"moduleResolution": "NodeNext"`) {
			t.Fatalf("expected NodeNext frontend settings to be removed, got %s", got)
		}
	})

	t.Run("canonicalizes_tsconfig_jsonc_comments_before_verification", func(t *testing.T) {
		t.Parallel()

		in := `{
  // compiler target
  "compilerOptions": {
    "target": "ES2020", // supported target
    "module": "ESNext",
    "jsx": "react-jsx",
  },
  "include": ["src"],
}`
		got := normalizeGeneratedFileContent("tsconfig.json", in)
		if strings.Contains(got, "// compiler target") || strings.Contains(got, "// supported target") {
			t.Fatalf("expected tsconfig comments to be stripped, got %s", got)
		}
		if strings.Contains(got, ",\n  }") || strings.Contains(got, ",\n}") {
			t.Fatalf("expected trailing commas to be removed, got %s", got)
		}
		if !strings.Contains(got, `"moduleResolution": "Bundler"`) {
			t.Fatalf("expected downstream tsconfig normalization to still run, got %s", got)
		}
	})
}

func canBindLocalhostPort() bool {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
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

func TestCanCreateAutomatedFixTask_BlocksConcurrentWriterAcrossActions(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		Mode: ModeFast,
		Tasks: []*Task{
			{
				ID:        "writer-1",
				Type:      TaskFix,
				Status:    TaskInProgress,
				CreatedAt: time.Now(),
				Input: map[string]any{
					"action": "fix_review_issues",
				},
			},
		},
	}

	if am.canCreateAutomatedFixTask(build, "fix_tests") {
		t.Fatalf("expected active fix_review_issues task to block concurrent fix_tests writer")
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

	ok, errs := am.verifyGeneratedCode("build-test", nil, out)
	if ok {
		t.Fatalf("expected verification failure due to parser warning/truncation")
	}
	if !containsError(errs, "unterminated code block") {
		t.Fatalf("expected parser warning surfaced in verification errors, got %v", errs)
	}
}

func TestParseTaskOutputCapturesStructuredPatchBundle(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	resp := "```json\n" +
		"{\"patch_bundle\":{\"justification\":\"repair frontend shell\",\"operations\":[" +
		"{\"type\":\"replace_function\",\"path\":\"src/App.tsx\",\"content\":\"export default function App(){ return <main>ok</main> }\\n\"}," +
		"{\"type\":\"delete_block\",\"path\":\"src/obsolete.ts\"}" +
		"]}}\n" +
		"```"

	out := am.parseTaskOutput(TaskFix, resp)
	if out.StructuredPatchBundle == nil {
		t.Fatal("expected structured patch bundle to be parsed")
	}
	if got := len(out.StructuredPatchBundle.Operations); got != 2 {
		t.Fatalf("expected 2 structured patch operations, got %d", got)
	}
	if len(out.Files) != 0 {
		t.Fatalf("expected structured patch parse to avoid file fallback parsing, got %+v", out.Files)
	}
}

func TestMaterializeStructuredPatchOutputUsesBaselineAndTracksDeletes(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	task := &Task{
		ID:   "task-structured-materialize",
		Type: TaskFix,
		Input: map[string]any{
			"patch_baseline_files": []GeneratedFile{
				{
					Path:     "src/App.tsx",
					Content:  "export default function App(){ return <div>old</div> }\n",
					Language: "typescript",
				},
				{
					Path:     "src/obsolete.ts",
					Content:  "export const obsolete = true\n",
					Language: "typescript",
				},
			},
		},
	}
	output := &TaskOutput{
		StructuredPatchBundle: &PatchBundle{
			Operations: []PatchOperation{
				{
					Type:    PatchReplaceFunction,
					Path:    "src/App.tsx",
					Content: "export default function App(){ return <main>new</main> }\n",
				},
				{
					Type: PatchDeleteBlock,
					Path: "src/obsolete.ts",
				},
			},
		},
	}

	am.materializeStructuredPatchOutput(nil, task, output)

	if len(output.Files) != 1 {
		t.Fatalf("expected one materialized file, got %+v", output.Files)
	}
	if output.Files[0].Path != "src/App.tsx" {
		t.Fatalf("expected materialized update for src/App.tsx, got %+v", output.Files)
	}
	if output.Files[0].IsNew {
		t.Fatalf("expected baseline-backed replacement to stay non-new, got %+v", output.Files[0])
	}
	if len(output.DeletedFiles) != 1 || output.DeletedFiles[0] != "src/obsolete.ts" {
		t.Fatalf("expected deleted file tracking, got %+v", output.DeletedFiles)
	}
	if taskOutputMetricInt(output, "structured_patch_op_count") != 2 {
		t.Fatalf("expected structured patch op metric, got %+v", output.Metrics)
	}
	if taskOutputMetricInt(output, "deleted_file_count") != 1 {
		t.Fatalf("expected deleted file metric, got %+v", output.Metrics)
	}
}

func TestPruneEchoedExistingFilesIgnoresUnchangedContextOutsideOwnership(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID: "build-echo-filter",
		SnapshotFiles: []GeneratedFile{
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>\n", Language: "html"},
			{Path: "src/App.tsx", Content: "export default function App(){ return <div>shell</div> }\n", Language: "typescript"},
			{Path: "src/index.css", Content: "body { background: #000; }\n", Language: "css"},
			{Path: "src/main.tsx", Content: "import React from 'react'\n", Language: "typescript"},
			{Path: "vite.config.ts", Content: "export default {}\n", Language: "typescript"},
		},
	}
	task := &Task{
		ID: "task-database",
		Input: map[string]any{
			"work_order": &BuildWorkOrder{
				Role:           RoleDatabase,
				OwnedFiles:     []string{"migrations/**", "server/db/**"},
				RequiredFiles:  []string{"migrations/001_initial.sql"},
				ForbiddenFiles: []string{"index.html", "src/**", "vite.config.ts"},
			},
		},
	}
	output := &TaskOutput{
		Completion: &TaskCompletionReport{
			Summary:      "database files generated",
			CreatedFiles: []string{"index.html", "src/App.tsx", "migrations/001_initial.sql"},
		},
		Files: []GeneratedFile{
			{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>\n", Language: "html"},
			{Path: "src/App.tsx", Content: "export default function App(){ return <div>shell</div> }\n", Language: "typescript"},
			{Path: "migrations/001_initial.sql", Content: "create table clients(id integer primary key);\n", Language: "sql"},
		},
	}

	am.pruneEchoedExistingFiles(build, output)

	if len(output.Files) != 1 || output.Files[0].Path != "migrations/001_initial.sql" {
		t.Fatalf("expected only owned database file to remain, got %+v", output.Files)
	}
	if output.Completion == nil || len(output.Completion.CreatedFiles) != 1 || output.Completion.CreatedFiles[0] != "migrations/001_initial.sql" {
		t.Fatalf("expected completion report to drop echoed context files, got %+v", output.Completion)
	}
	if taskOutputMetricInt(output, "ignored_unchanged_file_count") != 2 {
		t.Fatalf("expected ignored unchanged file metric, got %+v", output.Metrics)
	}

	errs := am.validateTaskCoordinationOutput(task, output)
	if len(errs) != 0 {
		t.Fatalf("expected unchanged echoed context files to be ignored, got %v", errs)
	}
}

func TestVerifyGeneratedCodeAllowsDeleteOnlyStructuredPatchOutput(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	ok, errs := am.verifyGeneratedCode("build-delete-only", nil, &TaskOutput{
		DeletedFiles: []string{"src/legacy.ts"},
	})
	if !ok {
		t.Fatalf("expected delete-only structured patch output to verify cleanly, got %v", errs)
	}
}

type truncationRouterStub struct {
	response *ai.AIResponse
	err      error
}

func (s *truncationRouterStub) Generate(context.Context, ai.AIProvider, string, GenerateOptions) (*ai.AIResponse, error) {
	return s.response, s.err
}

func (s *truncationRouterStub) GetAvailableProviders() []ai.AIProvider {
	return []ai.AIProvider{ai.ProviderClaude}
}

func (s *truncationRouterStub) GetAvailableProvidersForUser(uint) []ai.AIProvider {
	return s.GetAvailableProviders()
}

func (s *truncationRouterStub) HasConfiguredProviders() bool {
	return true
}

func TestCompleteTruncatedFilesClearsResolvedParserWarning(t *testing.T) {
	t.Parallel()

	am := &AgentManager{
		aiRouter: &truncationRouterStub{
			response: &ai.AIResponse{
				Content: "value = 1\n}\n",
			},
		},
	}

	resp := "// File: packages/backend/src/database/seed.ts\n" +
		"```typescript\n" +
		"export async function seed() {\n" +
		"  const"
	out := am.parseTaskOutput(TaskGenerateSchema, resp)

	am.completeTruncatedFiles(
		context.Background(),
		&Task{ID: "task-1"},
		&Build{UserID: 1, PowerMode: PowerFast},
		&Agent{Provider: ai.ProviderClaude},
		out,
	)

	if len(out.TruncatedFiles) != 0 {
		t.Fatalf("expected no unresolved truncated files, got %v", out.TruncatedFiles)
	}
	joined := strings.Join(out.Messages, " | ")
	if strings.Contains(strings.ToLower(joined), "unterminated code block") {
		t.Fatalf("expected parser warning to be removed after recovery, got %q", joined)
	}

	ok, errs := am.verifyGeneratedCode("build-test", nil, out)
	if !ok {
		t.Fatalf("expected verification success after recovery, got %v", errs)
	}
}

func TestCompleteTruncatedFilesKeepsUnresolvedParserWarning(t *testing.T) {
	t.Parallel()

	am := &AgentManager{
		aiRouter: &truncationRouterStub{
			err: errors.New("continuation unavailable"),
		},
	}

	resp := "// File: packages/backend/src/database/seed.ts\n" +
		"```typescript\n" +
		"export async function seed() {\n" +
		"  const"
	out := am.parseTaskOutput(TaskGenerateSchema, resp)

	am.completeTruncatedFiles(
		context.Background(),
		&Task{ID: "task-2"},
		&Build{UserID: 1, PowerMode: PowerFast},
		&Agent{Provider: ai.ProviderClaude},
		out,
	)

	if len(out.TruncatedFiles) != 1 {
		t.Fatalf("expected unresolved truncated file to remain tracked, got %v", out.TruncatedFiles)
	}
	ok, errs := am.verifyGeneratedCode("build-test", nil, out)
	if ok {
		t.Fatalf("expected verification to fail while truncation remains unresolved")
	}
	if !containsError(errs, "unterminated code block") {
		t.Fatalf("expected parser warning surfaced in verification errors, got %v", errs)
	}
}

func TestCompleteTruncatedFilesKeepsStructurallyIncompleteContinuationTracked(t *testing.T) {
	t.Parallel()

	am := &AgentManager{
		aiRouter: &truncationRouterStub{
			response: &ai.AIResponse{
				Content: "await seedUsers()\n",
			},
		},
	}

	resp := "// File: tests/verify-integration.ts\n" +
		"```typescript\n" +
		"export async function verifyIntegration() {\n" +
		"  await setupApp()\n"
	out := am.parseTaskOutput(TaskTest, resp)

	am.completeTruncatedFiles(
		context.Background(),
		&Task{ID: "task-2b"},
		&Build{UserID: 1, PowerMode: PowerFast},
		&Agent{Provider: ai.ProviderClaude},
		out,
	)

	if len(out.TruncatedFiles) != 1 || out.TruncatedFiles[0] != "tests/verify-integration.ts" {
		t.Fatalf("expected structurally incomplete continuation to remain tracked, got %v", out.TruncatedFiles)
	}
	ok, errs := am.verifyGeneratedCode("build-test", nil, out)
	if ok {
		t.Fatalf("expected verification to fail while JS/TS truncation remains unresolved")
	}
	if !containsError(errs, "Likely truncated source file") {
		t.Fatalf("expected truncation error surfaced after incomplete continuation, got %v", errs)
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

func TestQuickSyntaxCheckDetectsMissingClosingBraceAtEOF(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	errs := am.quickSyntaxCheck(GeneratedFile{
		Path:     "tests/verify-integration.ts",
		Language: "typescript",
		Content:  "export async function verifyIntegration() {\n  await page.goto('/')\n",
	})
	if !containsError(errs, "missing closing '}' before EOF") {
		t.Fatalf("expected missing brace truncation error, got %v", errs)
	}
}

func TestQuickSyntaxCheckAllowsRegexLiteralWithEscapedBrace(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	errs := am.quickSyntaxCheck(GeneratedFile{
		Path:     "src/lib/pattern.ts",
		Language: "typescript",
		Content:  "export const literalBrace = /\\{/;\nexport default literalBrace\n",
	})
	if containsError(errs, "Likely truncated source file") {
		t.Fatalf("expected escaped-brace regex to avoid truncation false positive, got %v", errs)
	}
}

func TestTrackLikelyTruncatedSourceFilesAddsAbruptEOFJSFiles(t *testing.T) {
	t.Parallel()

	out := &TaskOutput{
		Files: []GeneratedFile{
			{
				Path:     "tests/verify-integration.ts",
				Language: "typescript",
				Content:  "export async function verifyIntegration() {\n  await page.goto('/')\n",
			},
		},
	}

	trackLikelyTruncatedSourceFiles(out)

	if len(out.TruncatedFiles) != 1 || out.TruncatedFiles[0] != "tests/verify-integration.ts" {
		t.Fatalf("expected abrupt EOF file to be tracked for continuation, got %v", out.TruncatedFiles)
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

func TestMaxAutomatedRecoveryAttemptsByPowerMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode PowerMode
		want int
	}{
		{name: "fast", mode: PowerFast, want: 3},
		{name: "balanced", mode: PowerBalanced, want: 3},
		{name: "max", mode: PowerMax, want: 4},
		{name: "unknown defaults to fast", mode: PowerMode("unknown"), want: 3},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := maxAutomatedRecoveryAttempts(tt.mode); got != tt.want {
				t.Fatalf("maxAutomatedRecoveryAttempts(%q) = %d, want %d", tt.mode, got, tt.want)
			}
		})
	}
}

func TestRepeatedReadinessErrorClassRequiresThreeAttempts(t *testing.T) {
	t.Parallel()

	if repeatedReadinessErrorClassExhausted(2, "missing_dependency", "missing_dependency") {
		t.Fatalf("expected two attempts to allow another recovery pass")
	}
	if !repeatedReadinessErrorClassExhausted(3, "missing_dependency", "missing_dependency") {
		t.Fatalf("expected third repeated attempt to exhaust the repeated class")
	}
	if repeatedReadinessErrorClassExhausted(3, "", "missing_dependency") {
		t.Fatalf("expected empty prior class to avoid exhaustion")
	}
	if repeatedReadinessErrorClassExhausted(3, "missing_dependency", "syntax") {
		t.Fatalf("expected different classes to avoid exhaustion")
	}
}

func TestCrossAgentFileCoherenceErrorsDetectMissingLocalImport(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID: "cross-coherence-build",
		Tasks: []*Task{
			{
				ID:     "frontend",
				Type:   TaskGenerateUI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{Path: "src/App.tsx", Language: "typescript", Content: `import Panel from "./components/Panel"; export default function App(){ return <Panel /> }`},
					},
				},
			},
		},
	}

	errs := am.crossAgentFileCoherenceErrors(build)
	if len(errs) == 0 || !strings.Contains(errs[0], "generated file") {
		t.Fatalf("expected missing local import coherence error, got %+v", errs)
	}
}

func TestCancelAutomatedRecoveryTasksForLoopCap(t *testing.T) {
	t.Parallel()

	agent := &Agent{
		ID:     "solver-1",
		Role:   RoleSolver,
		Status: StatusWorking,
	}
	recoveryTask := &Task{ID: "t1", Type: TaskFix, Status: TaskInProgress, AssignedTo: agent.ID, Input: map[string]any{"action": "fix_review_issues"}}
	agent.CurrentTask = recoveryTask
	am := &AgentManager{}
	build := &Build{
		ID: "b1",
		Agents: map[string]*Agent{
			agent.ID: agent,
		},
		Tasks: []*Task{
			recoveryTask,
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
	if agent.CurrentTask != nil {
		t.Fatalf("expected cancelled recovery task to be released from agent, got %+v", agent.CurrentTask)
	}
	if agent.Status != StatusIdle {
		t.Fatalf("expected agent to return to idle after loop-cap cancellation, got %s", agent.Status)
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

func TestClassifyNodeInstallFailure(t *testing.T) {
	t.Parallel()

	t.Run("registry_404_remains_artifact_failure", func(t *testing.T) {
		t.Parallel()

		out := `npm ERR! code E404
npm ERR! 404 Not Found - GET https://registry.npmjs.org/@hooked%2fui - Not found`

		skip, summary := classifyNodeInstallFailure(out, errors.New("exit status 1"))
		if skip {
			t.Fatalf("expected npm 404 to remain a real artifact failure")
		}
		if !strings.Contains(summary, "package not found on npm registry") {
			t.Fatalf("expected package-not-found summary, got %q", summary)
		}
	})

	t.Run("network_failure_skips_verifier", func(t *testing.T) {
		t.Parallel()

		out := `npm ERR! code ENOTFOUND
npm ERR! network request to https://registry.npmjs.org/react failed, reason: getaddrinfo ENOTFOUND registry.npmjs.org`

		skip, _ := classifyNodeInstallFailure(out, errors.New("exit status 1"))
		if !skip {
			t.Fatalf("expected network install failure to skip verifier")
		}
	})

	t.Run("native_toolchain_failure_skips_verifier", func(t *testing.T) {
		t.Parallel()

		out := `npm ERR! code 1
npm ERR! command sh -c node-gyp rebuild
npm ERR! gyp ERR! find Python
npm ERR! gyp ERR! stack Error: Could not find any Python installation to use`

		skip, _ := classifyNodeInstallFailure(out, errors.New("exit status 1"))
		if !skip {
			t.Fatalf("expected node-gyp toolchain failure to skip verifier")
		}
	})

	t.Run("peer_dependency_cycle_remains_artifact_failure", func(t *testing.T) {
		t.Parallel()

		out := `npm ERR! code ERESOLVE
npm ERR! ERESOLVE unable to resolve dependency tree
npm ERR! peer react@"^17" from legacy-widget@1.0.0`

		skip, _ := classifyNodeInstallFailure(out, errors.New("exit status 1"))
		if skip {
			t.Fatalf("expected peer dependency conflict to remain a real artifact failure")
		}
	})

	t.Run("malformed_package_json_remains_artifact_failure", func(t *testing.T) {
		t.Parallel()

		out := `npm ERR! code EJSONPARSE
npm ERR! JSON.parse Unexpected end of JSON input while parsing package.json`

		skip, _ := classifyNodeInstallFailure(out, errors.New("exit status 1"))
		if skip {
			t.Fatalf("expected malformed package.json to remain a real artifact failure")
		}
	})
}

func TestClassifyNodeBuildFailure(t *testing.T) {
	t.Parallel()

	t.Run("timeout_without_actionable_errors_skips_verifier", func(t *testing.T) {
		t.Parallel()

		out := `> app@1.0.0 build
> vite build

building for production...`

		skip, _ := classifyNodeBuildFailure(out, errors.New("npm timed out after 90s"))
		if !skip {
			t.Fatalf("expected non-actionable build timeout to skip verifier")
		}
	})

	t.Run("timeout_with_typescript_errors_remains_failure", func(t *testing.T) {
		t.Parallel()

		out := `src/App.tsx(3,27): error TS2307: Cannot find module './missing'`

		skip, summary := classifyNodeBuildFailure(out, errors.New("npm timed out after 90s"))
		if skip {
			t.Fatalf("expected actionable build errors to remain failures")
		}
		if !strings.Contains(summary, "TS2307") {
			t.Fatalf("expected TypeScript failure summary, got %q", summary)
		}
	})
}

func TestPreviewProbeOutputShowsServerReady(t *testing.T) {
	t.Parallel()

	if !previewProbeOutputShowsServerReady(`  Local:   http://127.0.0.1:4173/`) {
		t.Fatalf("expected vite-style local banner to count as ready")
	}
	if previewProbeOutputShowsServerReady(`preview script finished`) {
		t.Fatalf("expected plain output without server banner to be treated as not ready")
	}
}

func TestClassifyPreviewHTTPProbeFailure(t *testing.T) {
	t.Parallel()

	t.Run("reported_ready_but_probe_timed_out_skips", func(t *testing.T) {
		t.Parallel()

		skip, summary := classifyPreviewHTTPProbeFailure(
			`Local:   http://127.0.0.1:4173/`,
			context.DeadlineExceeded,
			true,
		)
		if !skip {
			t.Fatalf("expected ready-but-unreachable preview server to skip verifier")
		}
		if !strings.Contains(summary, "127.0.0.1:4173") {
			t.Fatalf("expected probe summary to preserve preview output, got %q", summary)
		}
	})

	t.Run("bind_failure_skips", func(t *testing.T) {
		t.Parallel()

		skip, _ := classifyPreviewHTTPProbeFailure(
			`Error: listen EADDRNOTAVAIL: address not available 127.0.0.1:4173`,
			errors.New("exit status 1"),
			false,
		)
		if !skip {
			t.Fatalf("expected bind failure to skip verifier")
		}
	})

	t.Run("script_exit_without_server_is_real_failure", func(t *testing.T) {
		t.Parallel()

		skip, summary := classifyPreviewHTTPProbeFailure(
			`> app@1.0.0 preview
> echo nope

nope`,
			errors.New("exit status 1"),
			false,
		)
		if skip {
			t.Fatalf("expected misconfigured preview script to remain a real failure")
		}
		if !strings.Contains(summary, "nope") {
			t.Fatalf("expected preview output in summary, got %q", summary)
		}
	})
}

func TestExtractDependencyRepairHintsFromReadinessErrors(t *testing.T) {
	t.Parallel()

	errs := []string{
		`Preview verification dependency check failed: source imports "react-router-dom" but package.json does not declare dependency "react-router-dom"`,
		`Preview verification dependency check failed: source imports "vitest/config" but package.json does not declare dependency "vitest"`,
		`Preview verification dependency check failed: source imports "@vitejs/plugin-react" but package.json does not declare dependency "@vitejs/plugin-react"`,
	}
	hints := extractDependencyRepairHintsFromReadinessErrors(errs)
	if len(hints) == 0 {
		t.Fatalf("expected dependency repair hints")
	}
	joined := strings.Join(hints, "\n")
	if !strings.Contains(joined, "react-router-dom") || !strings.Contains(joined, "vitest") {
		t.Fatalf("expected missing package names in hints, got %q", joined)
	}
	if !strings.Contains(joined, "Preserve and satisfy imports") {
		t.Fatalf("expected import-preservation hint, got %q", joined)
	}
	if !strings.Contains(joined, "vitest -> devDependencies") {
		t.Fatalf("expected devDependencies placement guidance for vitest, got %q", joined)
	}
	if !strings.Contains(joined, "@vitejs/plugin-react -> devDependencies") {
		t.Fatalf("expected devDependencies placement guidance for vite plugin, got %q", joined)
	}
	if !strings.Contains(joined, "vite.config.ts in ESM syntax") {
		t.Fatalf("expected vite plugin-specific repair hint, got %q", joined)
	}
}

func TestExtractDependencyRepairHintsFromReadinessErrorsIncludesSpecificIntegrationRouteGuidance(t *testing.T) {
	t.Parallel()

	errs := []string{
		`integration: frontend calls /api/auth/login but backend has no matching route`,
		`integration: frontend calls /api/dashboard/kpis but backend has no matching route`,
		`integration: backend does not expose required contract endpoint /api/projects`,
		`integration: backend has no CORS configuration — frontend requests will be blocked by the browser`,
	}

	hints := extractDependencyRepairHintsFromReadinessErrors(errs)
	if len(hints) == 0 {
		t.Fatalf("expected integration repair hints")
	}

	joined := strings.Join(hints, "\n")
	for _, needle := range []string{"/api/auth/login", "/api/dashboard/kpis", "/api/projects", "CORS"} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("expected integration hint to mention %q, got %q", needle, joined)
		}
	}
	if !strings.Contains(joined, "Do not leave placeholder fetches to dead endpoints") {
		t.Fatalf("expected explicit route-repair guidance, got %q", joined)
	}
}

func TestParseMissingDependenciesByVerificationScope(t *testing.T) {
	t.Parallel()

	errs := []string{
		`Preview verification dependency check failed: source imports "zod" but package.json does not declare dependency "zod"`,
		`Preview verification dependency check failed: source imports "@vitejs/plugin-react" but package.json does not declare dependency "@vitejs/plugin-react"`,
		`Backend verification dependency check failed: source imports "bcrypt" but package.json does not declare dependency "bcrypt"`,
	}

	frontend, backend := parseMissingDependenciesByVerificationScope(errs)
	if got := strings.Join(frontend, ","); got != "@vitejs/plugin-react,zod" {
		t.Fatalf("unexpected frontend deps: %q", got)
	}
	if got := strings.Join(backend, ","); got != "bcrypt" {
		t.Fatalf("unexpected backend deps: %q", got)
	}
}

func TestValidatePreviewManifestToolingDependencies(t *testing.T) {
	t.Parallel()

	manifest := previewManifest{
		Scripts: map[string]string{
			"dev":  "concurrently \"vite\" \"tsx watch server/index.ts\"",
			"test": "jest",
		},
		Dependencies: map[string]string{
			"react": "^18.2.0",
		},
		DevDependencies: map[string]string{
			"vite": "^5.0.0",
		},
		Jest: map[string]any{
			"preset":          "ts-jest/presets/default-esm",
			"testEnvironment": "jsdom",
			"moduleNameMapper": map[string]any{
				"\\.(css|less|scss|sass)$": "identity-obj-proxy",
			},
		},
	}

	errs := validatePreviewManifestToolingDependencies(manifest)
	joined := strings.Join(errs, "\n")
	for _, needle := range []string{"concurrently", "tsx", "jest", "ts-jest", "jest-environment-jsdom", "identity-obj-proxy"} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("expected tooling dependency error for %q, got %v", needle, errs)
		}
	}
}

func TestValidatePreviewManifestToolingDependenciesIgnoresExtensions(t *testing.T) {
	t.Parallel()

	manifest := previewManifest{
		Scripts: map[string]string{
			"lint": "eslint . --ext ts,tsx --report-unused-disable-directives --max-warnings 0",
		},
		DevDependencies: map[string]string{
			"eslint": "^8.57.0",
		},
	}

	errs := validatePreviewManifestToolingDependencies(manifest)
	joined := strings.Join(errs, "\n")
	if strings.Contains(joined, `dependency "tsx"`) {
		t.Fatalf("tsx file extension must not require the tsx runner dependency, got %v", errs)
	}

	manifest.Scripts["dev"] = "tsx watch server/index.ts"
	errs = validatePreviewManifestToolingDependencies(manifest)
	if !strings.Contains(strings.Join(errs, "\n"), `dependency "tsx"`) {
		t.Fatalf("actual tsx command should require the tsx dependency, got %v", errs)
	}
}

func TestCheckIntegrationCoherenceCatchesRouteDrift(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		TechStack: &TechStack{Frontend: "React", Backend: "Node"},
		Plan: &BuildPlan{
			APIContract: &BuildAPIContract{
				Endpoints: []APIEndpoint{
					{Method: "GET", Path: "/api/health"},
				},
			},
		},
	}

	files := []GeneratedFile{
		{
			Path: "src/App.tsx",
			Content: "const jobId = \"123\";\n" +
				"fetch(`${API_BASE}/api/health`);\n" +
				"const events = new EventSource(`${API_BASE}/api/transcribe/${jobId}/progress`);\n" +
				"console.log(events);",
		},
		{
			Path: "server/index.ts",
			Content: `import cors from "cors";
app.use(cors());
app.use("/api/health", healthRouter);
app.use("/api", apiRouter);
app.listen(process.env.PORT || 3001);`,
		},
		{
			Path:    "server/src/routes/index.ts",
			Content: `router.get("/health", (_req, res) => res.json({ status: "ok" }));`,
		},
		{
			Path: "server/routes/api.ts",
			Content: `router.post("/transcribe", async (req, res) => {
  const { url } = req.body;
  res.setHeader("Content-Type", "text/event-stream");
  res.write(JSON.stringify({ ok: true, url }));
});`,
		},
	}

	errs := am.checkIntegrationCoherence(build, files)
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, "frontend calls /api/transcribe/:param/progress but backend has no matching route") {
		t.Fatalf("expected missing progress route integration error, got %v", errs)
	}
	if strings.Contains(joined, "backend does not expose required contract endpoint /api/health") {
		t.Fatalf("expected nested mount route resolution to satisfy /api/health, got %v", errs)
	}
}

func TestExtractExpressResolvedRoutesResolvesNestedMountedRouters(t *testing.T) {
	t.Parallel()

	files := []GeneratedFile{
		{
			Path: "server/index.ts",
			Content: `import apiRouter from "./routes/api";
app.use("/api", apiRouter);`,
		},
		{
			Path: "server/routes/api.ts",
			Content: `import authRouter from "./auth";
router.use("/auth", authRouter);`,
		},
		{
			Path: "server/routes/auth.ts",
			Content: `router.post("/login", handler);
router.get("/me", handler);`,
		},
	}

	resolved := extractExpressResolvedRoutes(files)
	for _, route := range []string{"/auth/login", "/auth/me", "/api/auth/login", "/api/auth/me"} {
		if !resolved[route] {
			t.Fatalf("expected resolved routes to include %s, got %+v", route, resolved)
		}
	}
}

func TestCheckIntegrationCoherenceAcceptsNestedMountedExpressRoutes(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		TechStack: &TechStack{Frontend: "React", Backend: "Node"},
	}

	files := []GeneratedFile{
		{
			Path: "src/App.tsx",
			Content: "fetch(`${API_BASE}/api/auth/login`);\n" +
				"fetch(`${API_BASE}/api/auth/me`);",
		},
		{
			Path: "server/index.ts",
			Content: `import cors from "cors";
import apiRouter from "./routes/api";
app.use(cors());
app.use("/api", apiRouter);
app.listen(process.env.PORT || 3001);`,
		},
		{
			Path: "server/routes/api.ts",
			Content: `import authRouter from "./auth";
router.use("/auth", authRouter);`,
		},
		{
			Path: "server/routes/auth.ts",
			Content: `router.post("/login", handler);
router.get("/me", handler);`,
		},
	}

	errs := am.checkIntegrationCoherence(build, files)
	joined := strings.Join(errs, "\n")
	if strings.Contains(joined, "/api/auth/login") || strings.Contains(joined, "/api/auth/me") {
		t.Fatalf("expected nested mounted routes to satisfy integration check, got %v", errs)
	}
}

func TestPatchManifestDependenciesJSON(t *testing.T) {
	t.Parallel()

	updated, added := patchManifestDependenciesJSON(`{
  "name": "app",
  "dependencies": {
    "react": "^18.2.0"
  },
  "devDependencies": {
    "vite": "^5.0.0"
  }
}`, []string{"zod", "@vitejs/plugin-react", "react"})

	if len(added) != 2 {
		t.Fatalf("expected 2 added dependencies, got %v", added)
	}
	if !strings.Contains(strings.Join(added, ","), "zod -> dependencies") {
		t.Fatalf("expected zod in dependencies, got %v", added)
	}
	if !strings.Contains(strings.Join(added, ","), "@vitejs/plugin-react -> devDependencies") {
		t.Fatalf("expected vite plugin in devDependencies, got %v", added)
	}
	if !strings.Contains(updated, `"zod": "^3.23.8"`) {
		t.Fatalf("expected zod version hint, got %s", updated)
	}
	if !strings.Contains(updated, `"@vitejs/plugin-react": "^4.3.4"`) {
		t.Fatalf("expected vite plugin version hint, got %s", updated)
	}
}

func TestCheckIntegrationCoherenceIgnoresFrontendTestOnlyDeadRoutes(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		TechStack: &TechStack{
			Frontend: "React",
			Backend:  "Express",
		},
		Plan: &BuildPlan{
			APIContract: &BuildAPIContract{
				Endpoints: []APIEndpoint{
					{Method: "GET", Path: "/api/health"},
				},
			},
		},
	}

	files := []GeneratedFile{
		{
			Path: "src/App.tsx",
			Content: "fetch(`${API_BASE}/api/health`)\n" +
				"  .then((response) => response.json())\n" +
				"  .then(console.log)\n",
		},
		{
			Path: "src/__tests__/api.test.ts",
			Content: "it('handles 404s', async () => {\n" +
				"  await fetch('/api/nonexistent-route')\n" +
				"})\n",
		},
		{
			Path: "server/index.ts",
			Content: `import cors from "cors";
const app = express();
app.use(cors());
app.get("/api/health", (_req, res) => res.json({ status: "ok" }));
app.listen(process.env.PORT || 3001);`,
		},
	}

	errs := am.checkIntegrationCoherence(build, files)
	joined := strings.Join(errs, "\n")
	if strings.Contains(joined, "/api/nonexistent-route") {
		t.Fatalf("expected test-only dead route to be ignored, got %v", errs)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no integration coherence errors, got %v", errs)
	}
}

func TestApplyDeterministicValidationRepairsCapturesPatchBundle(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-patch-bundle",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		Tasks: []*Task{
			{
				ID:     "task-gen",
				Type:   TaskGenerateUI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path:    "src/App.tsx",
							Content: "import React from 'react';\nexport default function App(){ return <div>ok</div> }\n",
							IsNew:   true,
						},
					},
				},
			},
			{
				ID:     "task-pending",
				Type:   TaskReview,
				Status: TaskPending,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{"missing_manifest: package.json (TypeScript/Node.js project has no package.json)"},
		"missing manifest",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected deterministic repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	hasManifest := false
	for _, file := range files {
		if file.Path == "package.json" {
			hasManifest = true
			break
		}
	}
	if !hasManifest {
		t.Fatalf("expected package.json to be created, got %+v", files)
	}

	state := build.SnapshotState.Orchestration
	if state == nil || len(state.PatchBundles) == 0 {
		t.Fatalf("expected captured patch bundle, got %+v", state)
	}
	bundle := state.PatchBundles[len(state.PatchBundles)-1]
	foundCreate := false
	for _, op := range bundle.Operations {
		if op.Type == PatchCreateFile && op.Path == "package.json" {
			foundCreate = true
			break
		}
	}
	if !foundCreate {
		t.Fatalf("expected create_file operation for package.json, got %+v", bundle.Operations)
	}
}

func TestApplyPatchBundleToBuildUsesSnapshotFileFallback(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID: "build-snapshot-patch",
		SnapshotFiles: []GeneratedFile{
			{
				Path:     "src/App.tsx",
				Content:  "export default function App(){ return <div>old</div> }\n",
				Language: "typescript",
			},
		},
	}

	applied := am.applyPatchBundleToBuild(build, &PatchBundle{
		ID:      "bundle-1",
		BuildID: build.ID,
		Operations: []PatchOperation{
			{Type: PatchReplaceFunction, Path: "src/App.tsx", Content: "export default function App(){ return <div>new</div> }\n"},
			{Type: PatchCreateFile, Path: "package.json", Content: "{\"name\":\"demo\"}\n"},
		},
	})
	if !applied {
		t.Fatal("expected patch bundle to apply")
	}

	files := am.collectGeneratedFiles(build)
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = strings.TrimSpace(file.Content)
	}
	if got := byPath["src/App.tsx"]; !strings.Contains(got, "new") {
		t.Fatalf("expected snapshot-backed file to be patched, got %q", got)
	}
	if got := byPath["package.json"]; !strings.Contains(got, "\"demo\"") {
		t.Fatalf("expected create_file operation to materialize package.json, got %q", got)
	}
}

func TestApplyDeterministicValidationRepairsAppliesBundleToSnapshotFiles(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-snapshot-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path:    "src/App.tsx",
				Content: "import React from 'react';\nexport default function App(){ return <div>ok</div> }\n",
				IsNew:   true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{"missing_manifest: package.json (TypeScript/Node.js project has no package.json)"},
		"missing manifest",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected deterministic repair to apply through patch bundle")
	}

	files := am.collectGeneratedFiles(build)
	hasManifest := false
	for _, file := range files {
		if file.Path == "package.json" {
			hasManifest = true
			break
		}
	}
	if !hasManifest {
		t.Fatalf("expected package.json to exist after snapshot-backed repair, got %+v", files)
	}

	if len(build.SnapshotFiles) == 0 {
		t.Fatalf("expected snapshot files to be updated, got %+v", build.SnapshotFiles)
	}
	state := build.SnapshotState.Orchestration
	if state == nil || len(state.PatchBundles) == 0 {
		t.Fatalf("expected recorded patch bundle, got %+v", state)
	}
}

func TestApplyDeterministicValidationRepairsCreatesMissingLocalModulePlaceholder(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-missing-local-module-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path:    "package.json",
				Content: "{\"name\":\"preview-test\",\"private\":true}\n",
				IsNew:   true,
			},
			{
				Path:    "src/pages/Projects.tsx",
				Content: "import KanbanColumn from '../components/projects/KanbanColumn';\nexport default function Projects(){ return <KanbanColumn /> }\n",
				IsNew:   true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{`Preview verification local import check failed: source imports local module "../components/projects/KanbanColumn" from "src/pages/Projects.tsx" but generated file "src/components/projects/KanbanColumn.tsx" is missing`},
		"missing local module",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected missing local module repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	var placeholder *GeneratedFile
	for i := range files {
		if files[i].Path == "src/components/projects/KanbanColumn.tsx" {
			placeholder = &files[i]
			break
		}
	}
	if placeholder == nil {
		t.Fatalf("expected placeholder file to be created, got %+v", files)
	}
	if !strings.Contains(placeholder.Content, "const KanbanColumn") {
		t.Fatalf("expected placeholder component export, got %q", placeholder.Content)
	}
	if !strings.Contains(placeholder.Content, "export default KanbanColumn") {
		t.Fatalf("expected default export for placeholder component, got %q", placeholder.Content)
	}

	state := build.SnapshotState.Orchestration
	if state == nil || len(state.PatchBundles) == 0 {
		t.Fatalf("expected recorded patch bundle, got %+v", state)
	}
	foundCreate := false
	for _, bundle := range state.PatchBundles {
		for _, op := range bundle.Operations {
			if op.Type == PatchCreateFile && op.Path == "src/components/projects/KanbanColumn.tsx" {
				foundCreate = true
				break
			}
		}
		if foundCreate {
			break
		}
	}
	if !foundCreate {
		t.Fatalf("expected create_file operation for missing local module, got %+v", state.PatchBundles)
	}
}

func TestApplyDeterministicValidationRepairsCreatesAliasModuleAfterRecoveryCap(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:                          "build-missing-alias-module-after-cap",
		Status:                      BuildReviewing,
		Mode:                        ModeFull,
		PowerMode:                   PowerBalanced,
		ReadinessRecoveryAttempts:   maxAutomatedRecoveryAttempts(PowerBalanced),
		RequirePreviewReady:         true,
		PhasedPipelineComplete:      true,
		CompileValidationAttempts:   2,
		CompileValidationPassed:     false,
		PreviewVerificationAttempts: 1,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "fieldops",
  "private": true,
  "scripts": { "build": "vite build", "dev": "vite" },
  "dependencies": { "react": "^18.3.1", "react-dom": "^18.3.1", "react-router-dom": "^6.22.3" },
  "devDependencies": { "@vitejs/plugin-react": "^4.3.1", "typescript": "^5.4.5", "vite": "^5.2.0" }
}`,
				IsNew: true,
			},
			{
				Path: "src/App.tsx",
				Content: `import React from "react";
const Pipeline = React.lazy(() => import("@/components/pages/Pipeline"));
export default function App(){ return <React.Suspense fallback={null}><Pipeline /></React.Suspense>; }
`,
				IsNew: true,
			},
			{
				Path:    "components/pages/Pipeline.cjs",
				Content: "module.exports = {};\n",
				IsNew:   true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{`Preview verification local import check failed: source imports local module "@/components/pages/Pipeline" from "src/App.tsx" but generated file "src/components/pages/Pipeline.tsx" is missing`},
		"missing alias module",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected deterministic alias module repair to apply even after AI recovery cap")
	}

	files := am.collectGeneratedFiles(build)
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}
	if strings.TrimSpace(byPath["src/components/pages/Pipeline.tsx"]) == "" {
		t.Fatalf("expected aliased src module placeholder to be created, got files %+v", files)
	}
	if strings.Contains(byPath["src/App.tsx"], "components/pages/Pipeline.cjs") {
		t.Fatalf("repair must not rewrite alias import to stale root cjs placeholder, got %q", byPath["src/App.tsx"])
	}
}

func TestApplyDeterministicValidationRepairsCreatesFrontendShellForBackendOnlyFullStackBuild(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:                  "build-missing-frontend-shell-repair",
		Status:              BuildInProgress,
		Mode:                ModeFull,
		PowerMode:           PowerBalanced,
		RequirePreviewReady: true,
		Description:         "Build a full-stack app called ClientPulse with a dashboard, auth, projects, and client management.",
		TechStack: &TechStack{
			Frontend: "react",
			Backend:  "express",
		},
		SnapshotFiles: []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "clientpulse",
  "private": true,
  "scripts": {
    "build": "tsc",
    "dev": "tsx watch src/server.ts"
  },
  "dependencies": {
    "express": "^4.18.2",
    "cors": "^2.8.5",
    "pg": "^8.11.3"
  },
  "devDependencies": {
    "tsx": "^4.19.2",
    "typescript": "^5.6.3"
  }
}`,
				IsNew: true,
			},
			{
				Path: "src/server.ts",
				Content: `import express from "express";

const app = express();
app.get("/api/health", (_req, res) => {
  res.json({ ok: true });
});

app.listen(process.env.PORT || 3001, () => {
  console.log("ready");
});
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{"Preview verification failed: No recognized frontend entry point found (index.html, src/main.tsx, src/index.tsx, etc.)."},
		"missing frontend entrypoint",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected missing frontend shell repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}

	for _, required := range []string{"index.html", "src/main.tsx", "src/App.tsx", "vite.config.ts", "package.json"} {
		if strings.TrimSpace(byPath[required]) == "" {
			t.Fatalf("expected %s to exist after repair, got files %+v", required, files)
		}
	}
	if !strings.Contains(byPath["package.json"], `"build": "vite build"`) {
		t.Fatalf("expected repaired manifest to run vite build, got %q", byPath["package.json"])
	}
	if !strings.Contains(byPath["package.json"], `"build:backend": "tsc"`) {
		t.Fatalf("expected original backend build script to be preserved, got %q", byPath["package.json"])
	}
	if !strings.Contains(byPath["package.json"], `"dev": "vite"`) {
		t.Fatalf("expected repaired manifest to expose vite dev, got %q", byPath["package.json"])
	}
	if !strings.Contains(byPath["package.json"], `"dev:backend": "tsx watch src/server.ts"`) {
		t.Fatalf("expected original backend dev script to be preserved, got %q", byPath["package.json"])
	}
	if !strings.Contains(byPath["package.json"], `"react"`) || !strings.Contains(byPath["package.json"], `"@vitejs/plugin-react"`) {
		t.Fatalf("expected repaired manifest to include frontend deps, got %q", byPath["package.json"])
	}
	if !strings.Contains(byPath["src/App.tsx"], "src/server.ts") {
		t.Fatalf("expected recovered App shell to mention backend runtime, got %q", byPath["src/App.tsx"])
	}
	if !strings.Contains(byPath["vite.config.ts"], "http://localhost:3001") {
		t.Fatalf("expected vite proxy target to use backend port, got %q", byPath["vite.config.ts"])
	}

	state := build.SnapshotState.Orchestration
	if state == nil || len(state.PatchBundles) == 0 {
		t.Fatalf("expected captured patch bundle, got %+v", state)
	}
}

func TestApplyDeterministicValidationRepairsDoesNotInventFrontendForBackendOnlyAPIBuild(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-api-only-no-frontend-shell",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		TechStack: &TechStack{
			Backend: "express",
		},
		SnapshotFiles: []GeneratedFile{
			{
				Path: "package.json",
				Content: `{
  "name": "api-only",
  "scripts": {
    "build": "tsc"
  }
}`,
				IsNew: true,
			},
			{
				Path: "src/server.ts",
				Content: `import express from "express";
const app = express();
app.get("/health", (_req, res) => res.send("ok"));
app.listen(3001);
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{"Preview verification failed: No recognized frontend entry point found (index.html, src/main.tsx, src/index.tsx, etc.)."},
		"missing frontend entrypoint",
		time.Now(),
	)
	if repaired {
		t.Fatal("did not expect frontend shell repair for backend-only API build")
	}
}

func TestApplyDeterministicValidationRepairsCreatesDeclarationForMissingCJSModulePlaceholder(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-missing-local-cjs-module-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "server/seed.ts",
				Content: `import { sequelize } from './db/index';
import * as models from './models.cjs';

async function seed() {
  await sequelize.authenticate();
  console.log(models);
}
`,
				IsNew: true,
			},
			{
				Path:    "server/db/index.ts",
				Content: "export const sequelize = {} as any;\n",
				IsNew:   true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{`Preview verification local import check failed: source imports local module "./models.cjs" from "server/seed.ts" but generated file "server/models.cjs" is missing`},
		"missing local CommonJS module",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected missing local CommonJS module repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	byPath := map[string]GeneratedFile{}
	for _, file := range files {
		byPath[file.Path] = file
	}
	cjs, ok := byPath["server/models.cjs"]
	if !ok {
		t.Fatalf("expected CommonJS placeholder file to be created, got %+v", files)
	}
	if !strings.Contains(cjs.Content, "module.exports") {
		t.Fatalf("expected CommonJS placeholder content, got %q", cjs.Content)
	}
	decl, ok := byPath["server/models.cjs.d.ts"]
	if !ok {
		t.Fatalf("expected CommonJS declaration file to be created, got %+v", files)
	}
	if !strings.Contains(decl.Content, "export = defaultExport;") {
		t.Fatalf("expected CommonJS declaration export assignment, got %q", decl.Content)
	}
}

func TestApplyDeterministicMissingLocalModuleRepairSkipsStaleTargets(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-stale-missing-local-module-repair",
		Status:    BuildReviewing,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "src/components/dashboard/DashboardHome.tsx",
				Content: `import KPICard from './KPICard';

export default function DashboardHome() {
  return <KPICard />;
}
`,
				IsNew: true,
			},
			{
				Path: "src/components/dashboard/KPICard.tsx",
				Content: `export default function KPICard() {
  return <div>kpi</div>;
}
`,
				IsNew: true,
			},
		},
	}

	bundle, summary := am.applyDeterministicMissingLocalModuleRepair(
		build,
		[]string{
			`Preview verification build failed: src/components/dashboard/DashboardHome.tsx(7,22): error TS2307: Cannot find module './StatCard' or its corresponding type declarations.`,
			`Preview verification local import check failed: source imports local module "./StatCard.cjs" from "src/components/dashboard/DashboardHome.tsx" but generated file "src/components/dashboard/StatCard.cjs" is missing`,
		},
	)
	if bundle != nil || summary != "" {
		t.Fatalf("expected stale missing-module errors to be ignored, got bundle=%+v summary=%q", bundle, summary)
	}
}

func TestValidateGeneratedLocalModuleImportsResolvesAtAliasIntoSrcTree(t *testing.T) {
	t.Parallel()

	files := []GeneratedFile{
		{
			Path: "src/App.tsx",
			Content: `import MainLayout from "@/components/Layout/MainLayout";

export default function App() {
  return <MainLayout />;
}
`,
		},
		{
			Path: "src/components/Layout/MainLayout.tsx",
			Content: `export default function MainLayout() {
  return <div>ok</div>;
}
`,
		},
	}

	if issues := validateGeneratedLocalModuleImports(files, ""); len(issues) != 0 {
		t.Fatalf("expected @ alias to resolve into src tree, got %+v", issues)
	}
}

func TestValidateGeneratedLocalModuleImportsAcceptsTypeScriptSourceForRuntimeJSImport(t *testing.T) {
	t.Parallel()

	files := []GeneratedFile{
		{
			Path: "server/index.ts",
			Content: `import apiRouter from "./routes/api.js";

export default apiRouter;
`,
		},
		{
			Path: "server/routes/api.ts",
			Content: `export default function apiRouter() {
  return "ok";
}
`,
		},
	}

	if issues := validateGeneratedLocalModuleImports(files, ""); len(issues) != 0 {
		t.Fatalf("expected runtime .js import to resolve to TypeScript source, got %+v", issues)
	}
}

func TestApplyDeterministicPreValidationNormalizationAddsFrontendSrcAliasSupport(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:   "build-prevalidation-src-alias-support",
		Mode: ModeFast,
		Tasks: []*Task{
			{
				ID:     "task-generate-ui",
				Type:   TaskGenerateUI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "package.json",
							Content: `{
  "name": "pulseboard",
  "private": true,
  "scripts": {
    "build": "tsc && vite build",
    "dev": "vite"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "@vitejs/plugin-react": "^4.3.4",
    "typescript": "^5.8.2",
    "vite": "^6.2.1"
  }
}`,
						},
						{
							Path: "src/App.tsx",
							Content: `import MainLayout from "@/components/Layout/MainLayout";

export default function App() {
  return <MainLayout />;
}
`,
						},
						{
							Path: "src/components/Layout/MainLayout.tsx",
							Content: `export default function MainLayout() {
  return <div>ready</div>;
}
`,
						},
						{
							Path: "tsconfig.json",
							Content: `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx"
  },
  "include": ["src"]
}`,
						},
						{
							Path: "vite.config.ts",
							Content: `import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
});
`,
						},
					},
				},
			},
		},
	}

	if !am.applyDeterministicPreValidationNormalization(build) {
		t.Fatalf("expected pre-validation normalization to add frontend path alias support")
	}

	files := am.collectGeneratedFiles(build)
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}

	if !strings.Contains(byPath["vite.config.ts"], `import path from "path";`) {
		t.Fatalf("expected vite config to import path, got %q", byPath["vite.config.ts"])
	}
	if !strings.Contains(byPath["vite.config.ts"], `"@"`) || !strings.Contains(byPath["vite.config.ts"], `path.resolve(__dirname, "./src")`) {
		t.Fatalf("expected vite config to declare @ src alias, got %q", byPath["vite.config.ts"])
	}
	if !strings.Contains(byPath["tsconfig.json"], `"baseUrl": "."`) {
		t.Fatalf("expected tsconfig baseUrl to be set, got %q", byPath["tsconfig.json"])
	}
	if !strings.Contains(byPath["tsconfig.json"], `"@/*"`) || !strings.Contains(byPath["tsconfig.json"], `"src/*"`) {
		t.Fatalf("expected tsconfig paths to declare @/* -> src/*, got %q", byPath["tsconfig.json"])
	}
}

func TestExtractBrokenGeneratedTestPaths(t *testing.T) {
	t.Parallel()

	errs := []string{
		`Preview verification build failed: src/__tests__/AppShell.test.tsx(2,18): error TS2305: Module '"@testing-library/react"' has no exported member 'screen'.`,
		`Preview verification build failed: src/components/AppShell.tsx(11,7): error TS2322: Type '"x"' is not assignable to type '"y"'.`,
	}

	targets := ExtractBrokenTestPaths(strings.Join(errs, "\n"))
	if len(targets) != 1 || targets[0] != "src/__tests__/AppShell.test.tsx" {
		t.Fatalf("unexpected generated test targets: %+v", targets)
	}
}

func TestApplyDeterministicValidationRepairsReplacesBrokenGeneratedTestFile(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-generated-test-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path:    "package.json",
				Content: "{\"name\":\"preview-test\",\"private\":true}\n",
				IsNew:   true,
			},
			{
				Path: "src/__tests__/AppShell.test.tsx",
				Content: `import { render, screen } from "@testing-library/react";
import { AppShell } from "../components/AppShell";
test("renders", () => {
  render(<AppShell />);
  expect(screen.getByText("Dashboard")).toBeInTheDocument();
});`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{`Preview verification build failed: src/__tests__/AppShell.test.tsx(2,18): error TS2305: Module '"@testing-library/react"' has no exported member 'screen'.`},
		"broken generated test",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected broken generated test repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	var repairedFile *GeneratedFile
	for i := range files {
		if files[i].Path == "src/__tests__/AppShell.test.tsx" {
			repairedFile = &files[i]
			break
		}
	}
	if repairedFile == nil {
		t.Fatalf("expected repaired test file to exist, got %+v", files)
	}
	if strings.Contains(repairedFile.Content, `render, screen } from "@testing-library/react"`) {
		t.Fatalf("expected broken RTL screen import to be repaired, got %q", repairedFile.Content)
	}
	if !strings.Contains(repairedFile.Content, "generated verification placeholder") &&
		!strings.Contains(repairedFile.Content, `from '@testing-library/dom'`) {
		t.Fatalf("expected placeholder fallback or patched screen import, got %q", repairedFile.Content)
	}
}

func TestApplyDeterministicValidationRepairsRepairsReactPropMismatch(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-react-prop-mismatch-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "src/components/Button.tsx",
				Content: `export interface ButtonProps {
  label: string
}

export function Button({ label }: ButtonProps) {
  return <button className="rounded-lg bg-indigo-600 px-4 py-2 text-white">{label}</button>
}
`,
				IsNew: true,
			},
			{
				Path: "src/components/ClientCard.tsx",
				Content: `import { Button } from "./Button"

export function ClientCard({ onSelect }: { onSelect?: () => void }) {
  return (
    <div className="rounded-2xl border border-slate-700 p-4">
      <Button className="w-full justify-center" onClick={onSelect}>
        View Details
      </Button>
    </div>
  )
}
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{`Preview verification build failed: src/components/ClientCard.tsx(5,15): error TS2322: Type '{ children: string; className: string; onClick: (() => void) | undefined; }' is not assignable to type 'IntrinsicAttributes & ButtonProps'.`},
		"react prop mismatch",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected react prop mismatch repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	byPath := map[string]GeneratedFile{}
	for _, file := range files {
		byPath[file.Path] = file
	}
	button, ok := byPath["src/components/Button.tsx"]
	if !ok {
		t.Fatalf("expected Button.tsx to remain in generated files, got %+v", files)
	}
	if !strings.Contains(button.Content, "extends React.ButtonHTMLAttributes<HTMLButtonElement>") {
		t.Fatalf("expected ButtonProps to extend button HTML attributes, got %q", button.Content)
	}
	if !strings.Contains(button.Content, "children?: React.ReactNode") {
		t.Fatalf("expected ButtonProps to accept children, got %q", button.Content)
	}
	if !strings.Contains(button.Content, "{ label, children, className, ...buttonProps }") {
		t.Fatalf("expected Button destructure to accept passthrough props, got %q", button.Content)
	}
	if !strings.Contains(button.Content, "<button {...buttonProps}") {
		t.Fatalf("expected Button to spread passthrough props onto root button, got %q", button.Content)
	}
	if !strings.Contains(button.Content, "{children ?? (label)}") {
		t.Fatalf("expected Button to prefer children before label fallback, got %q", button.Content)
	}
}

func TestApplyDeterministicValidationRepairsReplacesBrokenBackendGeneratedTestFileWithPlaceholder(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-generated-backend-test-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path:    "package.json",
				Content: "{\"name\":\"agency-ops\",\"private\":true}\n",
				IsNew:   true,
			},
			{
				Path: "server/__tests__/api.test.ts",
				Content: `import request from "supertest";
import { describe, it, expect } from "@jest/globals";
import { apiRouter } from "../routes/api";

describe("api", () => {
  it("responds", async () => {
    expect(apiRouter).toBeTruthy();
    expect(request).toBeTruthy();
  });
});`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{
			`Preview verification build failed: server/__tests__/api.test.ts(1,21): error TS2307: Cannot find module 'supertest' or its corresponding type declarations.`,
			`server/__tests__/api.test.ts(4,10): error TS2614: Module '"../routes/api"' has no exported member 'apiRouter'. Did you mean to use 'import apiRouter from "../routes/api"' instead?`,
			`server/__tests__/api.test.ts(6,1): error TS2582: Cannot find name 'describe'.`,
		},
		"broken generated backend test",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected broken backend generated test repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	var repairedFile *GeneratedFile
	for i := range files {
		if files[i].Path == "server/__tests__/api.test.ts" {
			repairedFile = &files[i]
			break
		}
	}
	if repairedFile == nil {
		t.Fatalf("expected repaired backend test file to exist, got %+v", files)
	}
	if strings.Contains(repairedFile.Content, "supertest") || strings.Contains(repairedFile.Content, "apiRouter") {
		t.Fatalf("expected backend placeholder repair to strip brittle imports, got %q", repairedFile.Content)
	}
	if !strings.Contains(repairedFile.Content, "generated verification placeholder") {
		t.Fatalf("expected backend placeholder repair content, got %q", repairedFile.Content)
	}
}

func TestApplyDeterministicValidationRepairsReplacesNestedBackendGeneratedTestFileWithPlaceholder(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-generated-nested-backend-test-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path:    "package.json",
				Content: "{\"name\":\"agency-ops\",\"private\":true}\n",
				IsNew:   true,
			},
			{
				Path: "server/__tests__/routes/api.test.ts",
				Content: `import request from "supertest";
import { describe, it, expect } from "@jest/globals";
import api from "../../index";

describe("api", () => {
  it("responds", async () => {
    expect(api).toBeTruthy();
    expect(request).toBeTruthy();
  });
});`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{
			`Preview verification build failed: server/__tests__/routes/api.test.ts(1,21): error TS2307: Cannot find module 'supertest' or its corresponding type declarations.`,
			`server/__tests__/routes/api.test.ts(3,8): error TS1192: Module '"../../index"' has no default export.`,
			`server/__tests__/routes/api.test.ts(5,1): error TS2582: Cannot find name 'describe'.`,
		},
		"broken nested generated backend test",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected nested broken backend generated test repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	var repairedFile *GeneratedFile
	for i := range files {
		if files[i].Path == "server/__tests__/routes/api.test.ts" {
			repairedFile = &files[i]
			break
		}
	}
	if repairedFile == nil {
		t.Fatalf("expected repaired nested backend test file to exist, got %+v", files)
	}
	if strings.Contains(repairedFile.Content, "supertest") || strings.Contains(repairedFile.Content, "../../index") {
		t.Fatalf("expected nested backend placeholder repair to strip brittle imports, got %q", repairedFile.Content)
	}
	if !strings.Contains(repairedFile.Content, "generated verification placeholder") {
		t.Fatalf("expected nested backend placeholder repair content, got %q", repairedFile.Content)
	}
}

func TestApplyDeterministicValidationRepairsStripsSequelizeUniqueKeys(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-sequelize-unique-keys-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "server/db/models.ts",
				Content: `import { Sequelize, DataTypes, Model } from "sequelize";

export class User extends Model {}
User.init({}, {
  sequelize,
  tableName: "user",
  indexes: [
    { fields: ["email"] },
  ],
  uniqueKeys: {
    unique_email_per_tenant: {
      fields: ["tenant_id", "email"],
    },
  },
});
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{
			`Preview verification build failed: server/db/models.ts(9,3): error TS2353: Object literal may only specify known properties, and 'uniqueKeys' does not exist in type 'InitOptions<User>'.`,
		},
		"broken sequelize init options",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected sequelize uniqueKeys repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	var repairedFile *GeneratedFile
	for i := range files {
		if files[i].Path == "server/db/models.ts" {
			repairedFile = &files[i]
			break
		}
	}
	if repairedFile == nil {
		t.Fatalf("expected repaired sequelize models file to exist, got %+v", files)
	}
	if strings.Contains(repairedFile.Content, "uniqueKeys:") {
		t.Fatalf("expected uniqueKeys block to be removed, got %q", repairedFile.Content)
	}
	if !strings.Contains(repairedFile.Content, "indexes:") {
		t.Fatalf("expected surrounding init options to remain intact, got %q", repairedFile.Content)
	}
}

func TestApplyDeterministicValidationRepairsCancelsSupersededRecoveryTasks(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-sequelize-unique-keys-cancel-recovery",
		Status:    BuildReviewing,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		Tasks: []*Task{
			{
				ID:     "task-fix-review",
				Type:   TaskFix,
				Status: TaskInProgress,
				Input: map[string]any{
					"action": "fix_review_issues",
				},
			},
		},
		SnapshotFiles: []GeneratedFile{
			{
				Path: "server/db/models.ts",
				Content: `import { Sequelize, DataTypes, Model } from "sequelize";

export class User extends Model {}
User.init({}, {
  sequelize,
  tableName: "user",
  indexes: [
    { fields: ["email"] },
  ],
  uniqueKeys: {
    unique_email_per_tenant: {
      fields: ["tenant_id", "email"],
    },
  },
});
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{
			`Preview verification build failed: server/db/models.ts(9,3): error TS2353: Object literal may only specify known properties, and 'uniqueKeys' does not exist in type 'InitOptions<User>'.`,
		},
		"broken sequelize init options",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected sequelize uniqueKeys repair to apply")
	}
	if build.Tasks[0].Status != TaskCancelled {
		t.Fatalf("expected superseded recovery task to be cancelled, got %s", build.Tasks[0].Status)
	}
}

func TestApplyDeterministicValidationRepairsClearsStaleSequelizeUniqueKeysError(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-sequelize-unique-keys-stale",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "server/db/models.ts",
				Content: `import { Sequelize, DataTypes, Model } from "sequelize";

export class User extends Model {}
User.init({}, {
  sequelize,
  tableName: "user",
  indexes: [
    { fields: ["email"] },
  ],
});
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{
			`Preview verification build failed: server/db/models.ts(9,3): error TS2353: Object literal may only specify known properties, and 'uniqueKeys' does not exist in type 'InitOptions<User>'.`,
		},
		"stale sequelize uniqueKeys validation",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected stale sequelize uniqueKeys validation to be cleared")
	}

	files := am.collectGeneratedFiles(build)
	var repairedFile *GeneratedFile
	for i := range files {
		if files[i].Path == "server/db/models.ts" {
			repairedFile = &files[i]
			break
		}
	}
	if repairedFile == nil {
		t.Fatalf("expected models file to remain present, got %+v", files)
	}
	if strings.Contains(repairedFile.Content, "uniqueKeys:") {
		t.Fatalf("expected stale validation path to keep clean file content, got %q", repairedFile.Content)
	}
}

func TestApplyDeterministicValidationRepairsStripsSequelizeIndexes(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-sequelize-indexes-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "server/db/models.ts",
				Content: `import { Sequelize, DataTypes, Model } from "sequelize";

export class User extends Model {}
User.init({}, {
  sequelize,
  tableName: "user",
  indexes: [
    { fields: ["email"] },
    { fields: ["tenant_id", "slug"] },
  ],
});
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{
			`Preview verification build failed: server/db/models.ts(7,3): error TS2353: Object literal may only specify known properties, and 'indexes' does not exist in type 'InitOptions<User>'.`,
		},
		"broken sequelize init options",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected sequelize indexes repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	var repairedFile *GeneratedFile
	for i := range files {
		if files[i].Path == "server/db/models.ts" {
			repairedFile = &files[i]
			break
		}
	}
	if repairedFile == nil {
		t.Fatalf("expected repaired sequelize models file to exist, got %+v", files)
	}
	if strings.Contains(repairedFile.Content, "indexes:") {
		t.Fatalf("expected indexes block to be removed, got %q", repairedFile.Content)
	}
	if !strings.Contains(repairedFile.Content, `tableName: "user"`) {
		t.Fatalf("expected surrounding init options to remain intact, got %q", repairedFile.Content)
	}
}

func TestApplyDeterministicValidationRepairsClearsStaleSequelizeIndexesError(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-sequelize-indexes-stale",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "server/db/models.ts",
				Content: `import { Sequelize, DataTypes, Model } from "sequelize";

export class User extends Model {}
User.init({}, {
  sequelize,
  tableName: "user",
});
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{
			`Preview verification build failed: server/db/models.ts(7,3): error TS2353: Object literal may only specify known properties, and 'indexes' does not exist in type 'InitOptions<User>'.`,
		},
		"stale sequelize indexes validation",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected stale sequelize indexes validation to be cleared")
	}
}

func TestApplyDeterministicValidationRepairsClearsStaleImportValidationError(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-stale-import-validation",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "server/seed.ts",
				Content: `import { sequelize } from './db/index';
import * as models from './models.cjs';

async function seed() {
  await sequelize.authenticate();
  console.log(models);
}
`,
				IsNew: true,
			},
			{
				Path: "server/db/index.ts",
				Content: `export const sequelize = {} as any;
`,
				IsNew: true,
			},
			{
				Path: "server/models.cjs",
				Content: `module.exports = {};
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{
			`Preview verification build failed: server/seed.ts(1,10): error TS2305: Module '"./db"' has no exported member 'sequelize'.`,
			`Preview verification build failed: server/seed.ts(2,25): error TS2307: Cannot find module './db/models' or its corresponding type declarations.`,
		},
		"stale seed import validation",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected stale import validation to be cleared")
	}
}

func TestApplyDeterministicValidationRepairsNormalizesSequelizeConstructor(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-sequelize-constructor-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "server/db/index.ts",
				Content: `import { Sequelize } from 'sequelize-typescript';
import path from 'path';

const databaseUrl = process.env.DATABASE_URL;
const urlParts = new URL(databaseUrl!);
const username = urlParts.username;
const password = urlParts.password;
const host = urlParts.hostname;
const port = urlParts.port;
const database = urlParts.pathname.slice(1);

export const sequelize = new Sequelize(database, username, password, {
  host,
  port: Number(port),
  dialect: 'postgres',
  logging: false,
  models: [path.resolve(__dirname, 'models')],
  define: {
    underscored: true,
    freezeTableName: true,
    timestamps: false
  }
});
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{
			`Preview verification build failed: server/db/index.ts(9,30): error TS2769: No overload matches this call.
Overload 1 of 4, '(database: string, username: string, password?: string | undefined, options?: SequelizeOptions | undefined): Sequelize', gave the following error.`,
		},
		"broken sequelize constructor",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected sequelize constructor repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	var repairedFile *GeneratedFile
	for i := range files {
		if files[i].Path == "server/db/index.ts" {
			repairedFile = &files[i]
			break
		}
	}
	if repairedFile == nil {
		t.Fatalf("expected repaired sequelize db file to exist, got %+v", files)
	}
	if strings.Contains(repairedFile.Content, "new Sequelize(database, username, password") {
		t.Fatalf("expected constructor call to be normalized, got %q", repairedFile.Content)
	}
	if !strings.Contains(repairedFile.Content, "new Sequelize(databaseUrl, {") {
		t.Fatalf("expected databaseUrl-form sequelize constructor, got %q", repairedFile.Content)
	}
	if strings.Contains(repairedFile.Content, "database,") || strings.Contains(repairedFile.Content, "username,") || strings.Contains(repairedFile.Content, "password,") {
		t.Fatalf("expected positional credentials to be removed, got %q", repairedFile.Content)
	}
	if !strings.Contains(repairedFile.Content, "models: [") || !strings.Contains(repairedFile.Content, "path.resolve(__dirname, 'models')") {
		t.Fatalf("expected models option to remain intact, got %q", repairedFile.Content)
	}
}

func TestApplyDeterministicValidationRepairsNormalizesSequelizeTypescriptObjectConstructor(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-sequelize-object-constructor-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "server/db/index.ts",
				Content: `import { Sequelize } from 'sequelize-typescript';
import { Tenant } from './models/Tenant';

const databaseUrl = process.env.DATABASE_URL;
if (!databaseUrl) {
  throw new Error('DATABASE_URL environment variable is required');
}

export const sequelize = new Sequelize({
  database: process.env.DB_NAME || 'app',
  username: process.env.DB_USER || 'postgres',
  password: process.env.DB_PASS || 'postgres',
  host: process.env.DB_HOST || 'localhost',
  port: Number(process.env.DB_PORT) || 5432,
  dialect: 'postgres',
  models: [
    Tenant,
  ],
  logging: false,
});
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{
			`Preview verification build failed: server/db/index.ts(16,30): error TS2769: No overload matches this call.
Overload 1 of 4, '(database: string, username: string, password?: string | undefined, options?: SequelizeOptions | undefined): Sequelize', gave the following error.`,
		},
		"broken sequelize constructor",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected sequelize object constructor repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	var repairedFile *GeneratedFile
	for i := range files {
		if files[i].Path == "server/db/index.ts" {
			repairedFile = &files[i]
			break
		}
	}
	if repairedFile == nil {
		t.Fatalf("expected repaired sequelize db file to exist, got %+v", files)
	}
	if !strings.Contains(repairedFile.Content, "new Sequelize(databaseUrl, {") {
		t.Fatalf("expected databaseUrl-form constructor, got %q", repairedFile.Content)
	}
	if strings.Contains(repairedFile.Content, "database: process.env.DB_NAME") ||
		strings.Contains(repairedFile.Content, "username: process.env.DB_USER") ||
		strings.Contains(repairedFile.Content, "password: process.env.DB_PASS") ||
		strings.Contains(repairedFile.Content, "host: process.env.DB_HOST") ||
		strings.Contains(repairedFile.Content, "port: Number(process.env.DB_PORT)") {
		t.Fatalf("expected object-form credential fields to be removed, got %q", repairedFile.Content)
	}
	if !strings.Contains(repairedFile.Content, "models: [") || !strings.Contains(repairedFile.Content, "Tenant") {
		t.Fatalf("expected models option to remain intact, got %q", repairedFile.Content)
	}
}

func TestApplyDeterministicValidationRepairsStripsSequelizeTypescriptTableIndexes(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-sequelize-table-options-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "server/db/models/ActivityLog.ts",
				Content: `import { Table, Column, Model, DataType } from 'sequelize-typescript';

@Table({
  tableName: 'activity_logs',
  indexes: [
    { fields: ['tenant_id'] },
    { fields: ['created_at'] },
  ],
})
export class ActivityLog extends Model {
  @Column(DataType.STRING)
  action!: string;
}
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{
			`Preview verification build failed: server/db/models/ActivityLog.ts(19,3): error TS2769: No overload matches this call.
Overload 1 of 2, '(options: TableOptions<Model<any, any>>): Function', gave the following error.`,
		},
		"broken sequelize table options",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected sequelize table option repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	var repairedFile *GeneratedFile
	for i := range files {
		if files[i].Path == "server/db/models/ActivityLog.ts" {
			repairedFile = &files[i]
			break
		}
	}
	if repairedFile == nil {
		t.Fatalf("expected repaired model file to exist, got %+v", files)
	}
	if strings.Contains(repairedFile.Content, "indexes:") {
		t.Fatalf("expected invalid indexes block to be removed, got %q", repairedFile.Content)
	}
	if !strings.Contains(repairedFile.Content, "tableName: 'activity_logs'") {
		t.Fatalf("expected tableName to remain intact, got %q", repairedFile.Content)
	}
}

func TestApplyDeterministicValidationRepairsRewritesSequelizeTypescriptRuntimeImport(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-sequelize-runtime-import-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path: "server/seed.ts",
				Content: `import { Sequelize } from 'sequelize-typescript';

const databaseUrl = process.env.DATABASE_URL;

const sequelize = new Sequelize(databaseUrl!, {
  logging: false,
  dialect: 'postgres',
});
`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{
			`Preview verification build failed: server/seed.ts(4,19): error TS2769: No overload matches this call.
Overload 1 of 4, '(database: string, username: string, password?: string | undefined, options?: SequelizeOptions | undefined): Sequelize', gave the following error.`,
		},
		"broken sequelize runtime import",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected runtime sequelize import repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	var repairedFile *GeneratedFile
	for i := range files {
		if files[i].Path == "server/seed.ts" {
			repairedFile = &files[i]
			break
		}
	}
	if repairedFile == nil {
		t.Fatalf("expected repaired seed file to exist, got %+v", files)
	}
	if strings.Contains(repairedFile.Content, "sequelize-typescript") {
		t.Fatalf("expected sequelize-typescript import to be rewritten, got %q", repairedFile.Content)
	}
	if !strings.Contains(repairedFile.Content, "import { Sequelize } from 'sequelize';") {
		t.Fatalf("expected sequelize runtime import, got %q", repairedFile.Content)
	}
}

func TestParsePreviewSyntaxErrorTargetFiles(t *testing.T) {
	t.Parallel()

	errs := []string{
		"Preview verification build failed: src/components/LoginForm.tsx(14,49): error TS1002: Unterminated string literal.\n" +
			"src/components/LoginForm.tsx(15,5): error TS1005: ',' expected.\n" +
			"src/App.tsx(2,1): error TS1005: ',' expected.",
	}

	targets := parsePreviewSyntaxErrorTargetFiles(errs)
	got := strings.Join(targets, ",")
	want := "src/App.tsx,src/components/LoginForm.tsx"
	if got != want {
		t.Fatalf("unexpected syntax target files: got %q want %q", got, want)
	}
}

func TestParsePreviewSyntaxErrorTargetFilesHandlesEsbuildErrors(t *testing.T) {
	t.Parallel()

	errs := []string{
		"Preview verification build failed: [vite:esbuild] Transform failed with 2 errors:\n" +
			"/tmp/apex-build-123/src/components/LoginForm.tsx:14:49: ERROR: Unterminated string literal\n" +
			"file: /tmp/apex-build-123/src/components/LoginForm.tsx:14:49\n" +
			"/tmp/apex-build-123/src/App.tsx:2:13: ERROR: Unexpected end of file",
	}

	targets := parsePreviewSyntaxErrorTargetFiles(errs)
	got := strings.Join(targets, ",")
	want := "src/App.tsx,src/components/LoginForm.tsx"
	if got != want {
		t.Fatalf("unexpected esbuild syntax target files: got %q want %q", got, want)
	}
}

func TestParsePreviewSyntaxErrorTargetFilesIncludesAbruptEOFMessages(t *testing.T) {
	t.Parallel()

	errs := []string{
		`provider verification blocked task output: The file tests/verify-integration.ts ends abruptly without completing the function, resulting in a missing closing brace and compilation failure.`,
	}

	targets := parsePreviewSyntaxErrorTargetFiles(errs)
	if len(targets) != 1 || targets[0] != "tests/verify-integration.ts" {
		t.Fatalf("unexpected syntax target files: %+v", targets)
	}
}

func TestParsePreviewJSXInTSRepairTargets(t *testing.T) {
	t.Parallel()

	errs := []string{
		`Final output validation failed: Preview verification build failed: [vite:esbuild] Transform failed with 1 error:
/tmp/apex-preview-verify-1861641592/src/hooks/useAuth.ts:90:6: ERROR: Expected ">" but found "value"`,
	}

	targets := parsePreviewJSXInTSRepairTargets(errs)
	if len(targets) != 1 || targets[0] != "src/hooks/useAuth.ts" {
		t.Fatalf("unexpected JSX-in-TS target files: %+v", targets)
	}
}

func TestApplyDeterministicValidationRepairsConvertsJSXInTSProviderFile(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-jsx-in-ts-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path:    "package.json",
				Content: "{\"name\":\"preview-test\",\"private\":true}\n",
				IsNew:   true,
			},
			{
				Path: "src/hooks/useAuth.ts",
				Content: `import { useState, createContext } from "react";

const AuthContext = createContext<any>(undefined);

export const AuthProvider = ({ children }: { children: React.ReactNode }) => {
  const [user] = useState(null);

  return (
    <AuthContext.Provider
      value={{ user }}
    >
      {children}
    </AuthContext.Provider>
  );
};`,
				IsNew: true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{`Final output validation failed: Preview verification build failed: [vite:esbuild] Transform failed with 1 error:
/tmp/apex-preview-verify-1861641592/src/hooks/useAuth.ts:90:6: ERROR: Expected ">" but found "value"`},
		"jsx in .ts provider",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected JSX-in-TS provider repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	var repairedFile *GeneratedFile
	for i := range files {
		if files[i].Path == "src/hooks/useAuth.ts" {
			repairedFile = &files[i]
			break
		}
	}
	if repairedFile == nil {
		t.Fatalf("expected repaired auth hook file to exist, got %+v", files)
	}
	if !strings.Contains(repairedFile.Content, "React.createElement(AuthContext.Provider") {
		t.Fatalf("expected provider JSX to be normalized with React.createElement, got %q", repairedFile.Content)
	}
	if !strings.Contains(repairedFile.Content, `import React, { useState, createContext } from "react";`) {
		t.Fatalf("expected React default import to be injected, got %q", repairedFile.Content)
	}
}

func TestApplyDeterministicValidationRepairsFixesDefaultExportMismatch(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID: "build-export-mismatch",
		Tasks: []*Task{
			{
				ID:   "frontend-task",
				Type: TaskGenerateUI,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "src/components/Button.tsx",
							Content: `import React from 'react'

type ButtonProps = {
  label: string
}

const Button: React.FC<ButtonProps> = ({ label }) => <button>{label}</button>

export default Button
`,
							IsNew: true,
						},
						{
							Path: "src/components/AppShell.tsx",
							Content: `import React from 'react'
import { Button } from './Button'

export default function AppShell() {
  return <Button label="Book now" />
}
`,
							IsNew: true,
						},
					},
				},
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{`Preview verification build failed: error during build:
RollupError: "Button" is not exported by "src/components/Button.tsx", imported by "src/components/AppShell.tsx".`},
		"preview build failed",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected deterministic export mismatch repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	var repairedImporter string
	for _, file := range files {
		if file.Path == "src/components/AppShell.tsx" {
			repairedImporter = file.Content
			break
		}
	}
	if repairedImporter == "" {
		t.Fatalf("expected repaired importer file, got %+v", files)
	}
	if !strings.Contains(repairedImporter, `import Button from './Button'`) {
		t.Fatalf("expected named import to be rewritten to default import, got %q", repairedImporter)
	}
}

func TestApplyDeterministicProviderBlockedTestRepair(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	output := &TaskOutput{
		Files: []GeneratedFile{
			{
				Path:     "tests/verify-integration.ts",
				Language: "typescript",
				Content:  "export async function verifyIntegration() {\n  const result = await",
			},
		},
		TruncatedFiles: []string{"tests/verify-integration.ts"},
	}

	repaired, summary := am.applyDeterministicProviderBlockedTestRepair(nil, output, []string{
		`The file tests/verify-integration.ts ends abruptly without completing the function, resulting in a missing closing brace and compilation failure.`,
	})
	if !repaired {
		t.Fatal("expected deterministic provider-blocked test repair to apply")
	}
	if !strings.Contains(summary, "tests/verify-integration.ts") {
		t.Fatalf("unexpected repair summary: %q", summary)
	}
	if strings.Contains(output.Files[0].Content, "await") {
		t.Fatalf("expected truncated content to be replaced, got %q", output.Files[0].Content)
	}
	if !strings.Contains(output.Files[0].Content, "generated verification placeholder") {
		t.Fatalf("expected placeholder verification content, got %q", output.Files[0].Content)
	}
	if len(output.TruncatedFiles) != 0 {
		t.Fatalf("expected repaired file to be removed from truncated files, got %+v", output.TruncatedFiles)
	}
}

func TestApplyDeterministicFrontendScaffoldTruncationRepair(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:          "build-frontend-repair",
		Description: "Build a polished agency operations dashboard with project tracking and client management.",
		TechStack: &TechStack{
			Frontend: "React",
			Backend:  "Express",
		},
		Plan: &BuildPlan{
			AppType: "fullstack",
			TechStack: TechStack{
				Frontend: "React",
				Backend:  "Express",
			},
		},
	}
	output := &TaskOutput{
		Files: []GeneratedFile{
			{
				Path:     "src/components/ui/input.tsx",
				Language: "typescript",
				Content:  `import * as React from "react"; const Input = React.forwardRef<HTMLInputElement, React.ComponentProps<"input">>((`,
			},
			{
				Path:     "src/App.tsx",
				Language: "typescript",
				Content:  `export default function App() { return (`,
			},
		},
		TruncatedFiles: []string{"src/components/ui/input.tsx", "src/App.tsx"},
	}

	repaired, summary := am.applyDeterministicFrontendScaffoldTruncationRepair(build, output, []string{
		`src/components/ui/input.tsx: Likely truncated source file (unterminated regular expression literal before EOF)`,
		`src/App.tsx: Likely truncated source file (missing closing '}' before EOF)`,
	})
	if !repaired {
		t.Fatal("expected deterministic frontend scaffold repair to apply")
	}
	if !strings.Contains(summary, "src/components/ui/input.tsx") || !strings.Contains(summary, "src/App.tsx") {
		t.Fatalf("unexpected repair summary: %q", summary)
	}
	if output.Files[0].Content == `import * as React from "react"; const Input = React.forwardRef<HTMLInputElement, React.ComponentProps<"input">>((` {
		t.Fatalf("expected truncated shadcn input to be replaced, got %q", output.Files[0].Content)
	}
	if !strings.Contains(output.Files[0].Content, `export { Input };`) {
		t.Fatalf("expected canonical input scaffold content, got %q", output.Files[0].Content)
	}
	if !strings.Contains(output.Files[1].Content, `APEX recovered preview`) {
		t.Fatalf("expected canonical App shell content, got %q", output.Files[1].Content)
	}
	if len(output.TruncatedFiles) != 0 {
		t.Fatalf("expected repaired scaffold files to be removed from truncated files, got %+v", output.TruncatedFiles)
	}
}

func TestApplyDeterministicFrontendScaffoldTruncationRepairSkipsNonFrontendBuilds(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:          "build-backend-only",
		Description: "Build an internal API service only.",
		TechStack: &TechStack{
			Frontend: "",
			Backend:  "Express",
		},
		Plan: &BuildPlan{
			AppType: "api",
			TechStack: TechStack{
				Backend: "Express",
			},
		},
	}
	output := &TaskOutput{
		Files: []GeneratedFile{
			{Path: "src/components/ui/input.tsx", Language: "typescript", Content: "broken"},
		},
		TruncatedFiles: []string{"src/components/ui/input.tsx"},
	}

	repaired, _ := am.applyDeterministicFrontendScaffoldTruncationRepair(build, output, []string{
		`src/components/ui/input.tsx: Likely truncated source file (unterminated regular expression literal before EOF)`,
	})
	if repaired {
		t.Fatal("expected backend-only build to skip frontend scaffold repair")
	}
}

func TestApplyDeterministicProviderBlockedTestRepairAddsMissingJestDependency(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID: "build-provider-test-tooling",
		Tasks: []*Task{
			{
				ID:     "task-testing",
				Type:   TaskTest,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "package.json",
							Content: `{
  "name": "agency-ops",
  "private": true,
  "scripts": {
    "build": "vite build"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  },
  "devDependencies": {
    "vite": "^5.0.0"
  }
}`,
						},
						{
							Path: "server/__tests__/api.test.ts",
							Content: `import { describe, it, expect } from "@jest/globals";

describe("health", () => {
  it("works", () => {
    expect(true).toBe(true);
  });
});`,
						},
					},
				},
			},
		},
	}
	output := &TaskOutput{
		Files: []GeneratedFile{
			{
				Path:    "server/index.ts",
				Content: `console.log("backend fix");`,
			},
		},
	}

	repaired, summary := am.applyDeterministicProviderBlockedTestRepair(build, output, []string{
		`The 'AFTER' version of package.json does not add Jest to devDependencies, which is required for the test script to run and would cause build failures.`,
	})
	if !repaired {
		t.Fatal("expected deterministic provider-blocked manifest repair to apply")
	}
	if !strings.Contains(summary, "package.json") || !strings.Contains(summary, "jest") {
		t.Fatalf("unexpected repair summary: %q", summary)
	}

	var manifest string
	for _, file := range output.Files {
		if file.Path == "package.json" {
			manifest = file.Content
			break
		}
	}
	if manifest == "" {
		t.Fatal("expected repaired output to include package.json")
	}
	if !strings.Contains(manifest, `"jest"`) {
		t.Fatalf("expected repaired manifest to include jest dependency, got %s", manifest)
	}
}

func TestApplyDeterministicProviderBlockedTestRepairCanonicalizesTSConfigJSON(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	output := &TaskOutput{
		Files: []GeneratedFile{
			{
				Path: "tsconfig.json",
				Content: `{
  // valid for tsconfig, but not strict JSON
  "compilerOptions": {
    "target": "ES2020",
  }
}`,
			},
		},
	}

	repaired, summary := am.applyDeterministicProviderBlockedTestRepair(nil, output, []string{
		`tsconfig.json contains comments, which are not allowed in JSON, causing a compilation error.`,
	})
	if !repaired {
		t.Fatal("expected deterministic tsconfig repair to apply")
	}
	if !strings.Contains(summary, "tsconfig.json") {
		t.Fatalf("unexpected repair summary: %q", summary)
	}
	if strings.Contains(output.Files[0].Content, "// valid for tsconfig") {
		t.Fatalf("expected tsconfig comments to be stripped, got %q", output.Files[0].Content)
	}
	if strings.Contains(output.Files[0].Content, "\"target\": \"ES2020\",\n  }") {
		t.Fatalf("expected trailing comma to be stripped, got %q", output.Files[0].Content)
	}
}

func TestApplyDeterministicProviderBlockedTestRepairAcceptsAlreadyCanonicalTSConfig(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	output := &TaskOutput{
		Files: []GeneratedFile{
			{
				Path: "tsconfig.json",
				Content: `{
  "compilerOptions": {
    "target": "ES2020"
  }
}`,
			},
		},
	}

	repaired, summary := am.applyDeterministicProviderBlockedTestRepair(nil, output, []string{
		`tsconfig.json contains comments, which are not allowed in JSON, causing a compilation error.`,
	})
	if !repaired {
		t.Fatal("expected already-canonical tsconfig to bypass false-positive blocker")
	}
	if !strings.Contains(summary, "tsconfig.json") {
		t.Fatalf("unexpected repair summary: %q", summary)
	}
	if strings.Contains(output.Files[0].Content, "//") {
		t.Fatalf("expected canonical tsconfig to remain clean, got %q", output.Files[0].Content)
	}
}

func TestApplyDeterministicProviderBlockedTestRepairAcceptsCanonicalTSConfigForInvalidJSONSyntaxBlocker(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	output := &TaskOutput{
		Files: []GeneratedFile{
			{
				Path: "tsconfig.json",
				Content: `{
  "compilerOptions": {
    "target": "ES2020"
  }
}`,
			},
		},
	}

	repaired, summary := am.applyDeterministicProviderBlockedTestRepair(nil, output, []string{
		`tsconfig.json contains invalid JSON syntax, as reported in deterministic verification errors, which would cause compilation failures.`,
	})
	if !repaired {
		t.Fatal("expected canonical tsconfig to bypass invalid JSON syntax false-positive blocker")
	}
	if !strings.Contains(summary, "tsconfig.json") {
		t.Fatalf("unexpected repair summary: %q", summary)
	}
	if strings.Contains(output.Files[0].Content, "//") {
		t.Fatalf("expected canonical tsconfig to remain clean, got %q", output.Files[0].Content)
	}
}

func TestApplyDeterministicProviderBlockedTestRepairClearsStaleTruncatedGeneratedTestBlocker(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	output := &TaskOutput{
		Files: []GeneratedFile{
			{
				Path:    "src/App.tsx",
				Content: `export default function App() { return <div>ok</div>; }`,
			},
		},
	}

	repaired, summary := am.applyDeterministicProviderBlockedTestRepair(nil, output, []string{
		`Truncated source in tests/integration/fullstack.test.ts, as it ends abruptly and would cause a compilation error due to incomplete code.`,
	})
	if !repaired {
		t.Fatal("expected stale truncated generated test blocker to be cleared")
	}
	if !strings.Contains(summary, "tests/integration/fullstack.test.ts") {
		t.Fatalf("unexpected repair summary: %q", summary)
	}
	if len(output.Files) != 1 || output.Files[0].Path != "src/App.tsx" {
		t.Fatalf("expected unrelated output files to remain unchanged, got %+v", output.Files)
	}
}

func TestApplyDeterministicPreValidationNormalizationAddsJestDependencyForGeneratedJestTests(t *testing.T) {
	t.Setenv("PATH", "")

	am := &AgentManager{}
	build := &Build{
		ID:   "build-prevalidation-jest-tests",
		Mode: ModeFast,
		Tasks: []*Task{
			{
				ID:     "task-generate-ui",
				Type:   TaskGenerateUI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "package.json",
							Content: `{
  "name": "agency-ops",
  "private": true,
  "scripts": {
    "build": "vite build"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  },
  "devDependencies": {
    "vite": "^5.0.0"
  }
}`,
						},
						{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
						{Path: "src/main.tsx", Content: `import React from "react"; import ReactDOM from "react-dom/client"; ReactDOM.createRoot(document.getElementById("root")!).render(<div />);`},
						{Path: "src/App.tsx", Content: `export default function App(){ return <div>ok</div>; }`},
						{Path: "src/__tests__/app.test.tsx", Content: `import { describe, it, expect } from "@jest/globals"; describe("App", () => { it("renders", () => { expect(true).toBe(true); }); });`},
					},
				},
			},
		},
	}

	if !am.applyDeterministicPreValidationNormalization(build) {
		t.Fatalf("expected pre-validation normalization to trigger for generated jest tests")
	}

	files := am.collectGeneratedFiles(build)
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}

	manifest := byPath["package.json"]
	if !strings.Contains(manifest, `"jest"`) {
		t.Fatalf("expected jest dependency to be added, got %s", manifest)
	}
}

func TestRepairDoubleSingleQuoteCorruption(t *testing.T) {
	t.Parallel()

	input := "import React from ''react'';\nconst x = ''hello'';\n"
	out, changed := repairDoubleSingleQuoteCorruption("src/components/LoginForm.tsx", input)
	if !changed {
		t.Fatalf("expected quote corruption repair to trigger")
	}
	if strings.Contains(out, "''react''") || strings.Contains(out, "''hello''") {
		t.Fatalf("expected doubled single quotes to be repaired, got %q", out)
	}
	if !strings.Contains(out, "'react'") || !strings.Contains(out, "'hello'") {
		t.Fatalf("expected normalized single-quoted strings, got %q", out)
	}
}

func TestApplyDeterministicValidationRepairsAppliesQuoteRepairForEsbuildErrors(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:        "build-esbuild-syntax-repair",
		Status:    BuildInProgress,
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path:     "src/components/LoginForm.tsx",
				Content:  "export default function LoginForm(){ return <button aria-label={''Login''}>Login</button> }\n",
				Language: "typescript",
				IsNew:    true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	repaired := am.applyDeterministicValidationRepairs(
		build,
		[]string{
			"Preview verification build failed: [vite:esbuild] Transform failed with 1 error:\n" +
				"/tmp/apex-build-123/src/components/LoginForm.tsx:14:49: ERROR: Unterminated string literal",
		},
		"preview build failed",
		time.Now(),
	)
	if !repaired {
		t.Fatal("expected deterministic esbuild syntax repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	var repairedContent string
	for _, file := range files {
		if file.Path == "src/components/LoginForm.tsx" {
			repairedContent = file.Content
			break
		}
	}
	if repairedContent == "" {
		t.Fatalf("expected repaired LoginForm file, got %+v", files)
	}
	if strings.Contains(repairedContent, "''Login''") {
		t.Fatalf("expected quote corruption to be repaired, got %q", repairedContent)
	}

	state := build.SnapshotState.Orchestration
	if state == nil || len(state.PatchBundles) == 0 {
		t.Fatalf("expected patch bundle capture for esbuild syntax repair, got %+v", state)
	}
}

func TestParseMissingTypePackagesFromBuildErrors(t *testing.T) {
	t.Parallel()

	errs := []string{
		"Preview verification build failed: src/api/routes/auth.ts(1,24): error TS7016: Could not find a declaration file for module 'express'. '/tmp/x/node_modules/express/index.js' implicitly has an 'any' type.\n" +
			"src/api/server.ts(2,18): error TS7016: Could not find a declaration file for module 'cors'.\n" +
			"src/App.tsx(1,19): error TS7016: Could not find a declaration file for module 'react'.\n" +
			"src/main.tsx(2,22): error TS7016: Could not find a declaration file for module 'react-dom/client'.\n" +
			"server/migrate.ts(1,22): error TS7016: Could not find a declaration file for module 'pg'.\n" +
			"src/lib/x.ts(3,1): error TS7016: Could not find a declaration file for module '@scoped/pkg'.",
	}

	got := strings.Join(parseMissingTypePackagesFromBuildErrors(errs), ",")
	want := "@types/cors,@types/express,@types/pg,@types/react,@types/react-dom"
	if got != want {
		t.Fatalf("unexpected parsed type packages: got %q want %q", got, want)
	}
}

func TestApplyDeterministicTypeDeclarationRepairAddsPgTypes(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID: "build-pg-types-repair",
		Tasks: []*Task{
			{
				ID:     "task-generate-api",
				Type:   TaskGenerateAPI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "package.json",
							Content: `{
  "name": "api-test",
  "private": true,
  "scripts": {
    "build": "tsc -p server/tsconfig.json"
  },
  "dependencies": {
    "pg": "^8.11.3"
  },
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}`,
						},
						{Path: "server/migrate.ts", Content: `import { Pool } from 'pg'; const pool = new Pool(); export default pool;`},
					},
				},
			},
		},
	}

	bundle, summary := am.applyDeterministicTypeDeclarationRepair(build, []string{
		"Preview verification build failed: server/migrate.ts(1,22): error TS7016: Could not find a declaration file for module 'pg'.",
	})
	if bundle == nil {
		t.Fatalf("expected pg type declaration repair to trigger")
	}
	if !am.applyPatchBundleToBuild(build, bundle) {
		t.Fatalf("expected patch bundle to apply")
	}
	if !strings.Contains(summary, "@types/pg") {
		t.Fatalf("expected summary to mention @types/pg, got %q", summary)
	}

	var manifest string
	for _, file := range am.collectGeneratedFiles(build) {
		if file.Path == "package.json" {
			manifest = file.Content
			break
		}
	}
	if manifest == "" {
		t.Fatal("expected patched package.json to remain present")
	}
	if !strings.Contains(manifest, `"@types/pg"`) {
		t.Fatalf("expected package.json to include @types/pg, got %s", manifest)
	}
}

func TestApplyDeterministicTypeDeclarationRepairAnnotatesGeneratedDbErrorHandler(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID: "build-pg-db-error-handler-repair",
		Tasks: []*Task{
			{
				ID:     "task-generate-api",
				Type:   TaskGenerateAPI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "package.json",
							Content: `{
  "name": "api-test",
  "private": true,
  "dependencies": {
    "pg": "^8.11.3"
  },
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}`,
						},
						{
							Path: "server/db/index.ts",
							Content: `import { Pool } from 'pg';

const pool = new Pool();

pool.on('error', (err) => {
  console.error('Unexpected error on idle client', err);
});

export default pool;
`,
						},
					},
				},
			},
		},
	}

	bundle, summary := am.applyDeterministicTypeDeclarationRepair(build, []string{
		"Final output validation failed: Preview verification build failed: server/db/index.ts(1,22): error TS7016: Could not find a declaration file for module 'pg'. '/tmp/apex-preview-verify-1346717894/node_modules/pg/esm/index.mjs' implicitly has an 'any' type.",
		"Final output validation failed: Preview verification build failed: server/db/index.ts(4,19): error TS7006: Parameter 'err' implicitly has an 'any' type.",
	})
	if bundle == nil {
		t.Fatalf("expected pg type declaration repair to trigger")
	}
	if !am.applyPatchBundleToBuild(build, bundle) {
		t.Fatalf("expected patch bundle to apply")
	}
	if !strings.Contains(summary, "@types/pg") {
		t.Fatalf("expected summary to mention @types/pg, got %q", summary)
	}
	if !strings.Contains(summary, "server/db/index.ts") {
		t.Fatalf("expected summary to mention server/db/index.ts, got %q", summary)
	}

	var manifest string
	var dbFile string
	for _, file := range am.collectGeneratedFiles(build) {
		switch file.Path {
		case "package.json":
			manifest = file.Content
		case "server/db/index.ts":
			dbFile = file.Content
		}
	}
	if !strings.Contains(manifest, `"@types/pg"`) {
		t.Fatalf("expected package.json to include @types/pg, got %s", manifest)
	}
	if !strings.Contains(dbFile, "pool.on('error', (err: Error) =>") {
		t.Fatalf("expected db error callback to be typed, got %q", dbFile)
	}
}

func TestApplyDeterministicTypeDeclarationRepairAddsViteEnvDeclaration(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID: "build-vite-env-repair",
		Tasks: []*Task{
			{
				ID:     "task-generate-ui",
				Type:   TaskGenerateUI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "package.json",
							Content: `{
  "name": "preview-test",
  "private": true,
  "scripts": {
    "build": "tsc && vite build"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  },
  "devDependencies": {
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "@vitejs/plugin-react": "^4.0.0",
    "typescript": "^5.0.0",
    "vite": "^5.0.0"
  }
}`,
						},
						{Path: "vite.config.ts", Content: `import { defineConfig } from "vite"; export default defineConfig({});`},
						{Path: "src/main.tsx", Content: `console.log(import.meta.env.VITE_API_URL);`},
					},
				},
			},
		},
	}

	bundle, summary := am.applyDeterministicTypeDeclarationRepair(build, []string{
		"Preview verification build failed: src/main.tsx(1,25): error TS2339: Property 'env' does not exist on type 'ImportMeta'.",
	})
	if bundle == nil {
		t.Fatalf("expected vite env declaration repair to trigger")
	}
	if !am.applyPatchBundleToBuild(build, bundle) {
		t.Fatalf("expected patch bundle to apply")
	}
	if !strings.Contains(summary, "src/vite-env.d.ts") {
		t.Fatalf("expected vite env declaration path in summary, got %q", summary)
	}

	var viteEnv string
	for _, file := range am.collectGeneratedFiles(build) {
		if file.Path == "src/vite-env.d.ts" {
			viteEnv = file.Content
			break
		}
	}
	if viteEnv == "" {
		t.Fatalf("expected src/vite-env.d.ts to be created")
	}
	if !strings.Contains(viteEnv, `/// <reference types="vite/client" />`) {
		t.Fatalf("expected vite client type reference, got %q", viteEnv)
	}
	if !strings.Contains(viteEnv, `readonly env: ImportMetaEnv`) {
		t.Fatalf("expected ImportMeta env declaration, got %q", viteEnv)
	}
}

func TestApplyDeterministicPreValidationNormalizationRepairsStaticReactViteBuild(t *testing.T) {
	t.Setenv("PATH", "")

	am := &AgentManager{}
	build := &Build{
		ID:   "build-prevalidation-react-vite",
		Mode: ModeFast,
		TechStack: &TechStack{
			Frontend: "React",
		},
		Tasks: []*Task{
			{
				ID:     "task-generate-ui",
				Type:   TaskGenerateUI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "package.json",
							Content: `{
  "name": "preview-test",
  "private": true,
  "scripts": {
    "build": "tsc && vite build",
    "test": "vitest run"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  },
  "devDependencies": {
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "@vitejs/plugin-react": "^4.0.0",
    "typescript": "^5.0.0",
    "vite": "^5.0.0"
  }
}`,
						},
						{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
						{Path: "vite.config.ts", Content: `import { defineConfig } from "vite"; import react from "@vitejs/plugin-react"; export default defineConfig({ plugins: [react()] });`},
						{Path: "src/main.tsx", Content: `console.log(import.meta.env.VITE_API_URL);`},
						{Path: "src/App.tsx", Content: `export default function App(){ return <div>ok</div>; }`},
						{Path: "src/App.test.tsx", Content: `import { describe, it, expect } from "vitest"; import { render } from "@testing-library/react"; import "@testing-library/jest-dom"; describe("App", () => { it("renders", () => { render(document.createElement("div")); expect(true).toBe(true); }); });`},
					},
				},
			},
		},
	}

	if !am.applyDeterministicPreValidationNormalization(build) {
		t.Fatalf("expected pre-validation normalization to trigger")
	}

	files := am.collectGeneratedFiles(build)
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}

	manifest := byPath["package.json"]
	if !strings.Contains(manifest, `"preview": "vite preview"`) {
		t.Fatalf("expected preview script to be added, got %s", manifest)
	}
	for _, needle := range []string{`"vitest"`, `"@testing-library/react"`, `"@testing-library/jest-dom"`, `"jsdom"`} {
		if !strings.Contains(manifest, needle) {
			t.Fatalf("expected %s in normalized package.json, got %s", needle, manifest)
		}
	}
	if _, ok := byPath["tsconfig.json"]; !ok {
		t.Fatalf("expected tsconfig.json to be created")
	}
	if _, ok := byPath["src/vite-env.d.ts"]; !ok {
		t.Fatalf("expected src/vite-env.d.ts to be created")
	}

	if errs := am.validateFinalBuildReadiness(build, files); containsError(errs, "dependency check failed") || containsError(errs, "tsconfig.json is missing") {
		t.Fatalf("expected normalized build to avoid manifest/tsconfig readiness errors, got %v", errs)
	}
}

func TestApplyDeterministicPreValidationNormalizationCanonicalizesRadixPackageNames(t *testing.T) {
	t.Setenv("PATH", "")

	am := &AgentManager{}
	build := &Build{
		ID:   "build-prevalidation-radix",
		Mode: ModeFast,
		TechStack: &TechStack{
			Frontend: "React",
		},
		Tasks: []*Task{
			{
				ID:     "task-generate-ui",
				Type:   TaskGenerateUI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "package.json",
							Content: `{
  "name": "radix-test",
  "private": true,
  "scripts": {
    "build": "vite build"
  },
  "dependencies": {
    "radix-ui/react-accordion": "^1.1.2",
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  },
  "devDependencies": {
    "vite": "^5.0.0"
  }
}`,
						},
						{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
						{Path: "src/main.tsx", Content: `import React from "react"; import ReactDOM from "react-dom/client"; import { Accordion } from "radix-ui/react-accordion"; console.log(Accordion); ReactDOM.createRoot(document.getElementById("root")!).render(<div />);`},
						{Path: "src/App.tsx", Content: `export default function App(){ return <div>ok</div>; }`},
					},
				},
			},
		},
	}

	if !am.applyDeterministicPreValidationNormalization(build) {
		t.Fatalf("expected pre-validation normalization to trigger for invalid Radix package name")
	}

	files := am.collectGeneratedFiles(build)
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}

	manifest := byPath["package.json"]
	if strings.Contains(manifest, `"radix-ui/react-accordion"`) {
		t.Fatalf("expected invalid Radix package alias to be removed, got %s", manifest)
	}
	if !strings.Contains(manifest, `"@radix-ui/react-accordion"`) {
		t.Fatalf("expected canonical Radix package name to be present, got %s", manifest)
	}

	if errs := am.validateFinalBuildReadiness(build, files); containsError(errs, `invalid npm package name "radix-ui/react-accordion"`) || containsError(errs, `does not declare dependency "@radix-ui/react-accordion"`) {
		t.Fatalf("expected canonicalized Radix manifest to pass dependency validation, got %v", errs)
	}
}

func TestApplyDeterministicPreValidationNormalizationAddsBackendTSConfigAndTooling(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:   "build-prevalidation-backend-ts",
		Mode: ModeFast,
		Tasks: []*Task{
			{
				ID:     "task-generate-api",
				Type:   TaskGenerateAPI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "server/package.json",
							Content: `{
  "name": "server",
  "scripts": {
    "build": "tsc",
    "dev": "tsx watch src/index.ts"
  },
  "dependencies": {
    "express": "^4.18.2"
  }
}`,
						},
						{Path: "server/src/index.ts", Content: `import express from "express"; const app = express(); app.listen(3001);`},
					},
				},
			},
		},
	}

	if !am.applyDeterministicPreValidationNormalization(build) {
		t.Fatalf("expected backend pre-validation normalization to trigger")
	}

	files := am.collectGeneratedFiles(build)
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}

	manifest := byPath["server/package.json"]
	if !strings.Contains(manifest, `"tsx"`) {
		t.Fatalf("expected tsx dependency to be added, got %s", manifest)
	}
	if !strings.Contains(manifest, `"typescript"`) {
		t.Fatalf("expected typescript dependency to be preserved or added, got %s", manifest)
	}
	if !strings.Contains(manifest, `"dev": "tsx watch src/index.ts"`) && !strings.Contains(manifest, `"dev": "tsx src/index.ts"`) {
		t.Fatalf("expected backend runtime script to remain usable, got %s", manifest)
	}
	if _, ok := byPath["server/tsconfig.json"]; !ok {
		t.Fatalf("expected server/tsconfig.json to be created")
	}
}

func TestApplyDeterministicPreValidationNormalizationAddsMissingBackendRuntimeScripts(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:   "build-prevalidation-backend-runtime",
		Mode: ModeFast,
		Tasks: []*Task{
			{
				ID:     "task-generate-api-runtime",
				Type:   TaskGenerateAPI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "backend/package.json",
							Content: `{
  "name": "api",
  "private": true,
  "scripts": {
    "build": "tsc"
  },
  "dependencies": {
    "express": "^4.18.2"
  }
}`,
						},
						{Path: "backend/src/server.ts", Content: `import express from "express"; const app = express(); app.listen(process.env.PORT || 3001);`},
					},
				},
			},
		},
	}

	if !am.applyDeterministicPreValidationNormalization(build) {
		t.Fatalf("expected backend runtime normalization to trigger")
	}

	files := am.collectGeneratedFiles(build)
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}

	manifest := byPath["backend/package.json"]
	if !strings.Contains(manifest, `"start": "node dist/server.js"`) {
		t.Fatalf("expected backend start script to be added, got %s", manifest)
	}
	if !strings.Contains(manifest, `"dev": "tsx src/server.ts"`) {
		t.Fatalf("expected backend dev runtime script to be added, got %s", manifest)
	}
	if !strings.Contains(manifest, `"tsx"`) {
		t.Fatalf("expected tsx dependency to be added, got %s", manifest)
	}
}

func TestApplyDeterministicPreValidationNormalizationSplitsRootServerTSConfig(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:   "build-prevalidation-root-server-tsconfig",
		Mode: ModeFull,
		Tasks: []*Task{
			{
				ID:     "task-generate-fullstack-runtime",
				Type:   TaskGenerateAPI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "package.json",
							Content: `{
  "name": "agency-ops",
  "private": true,
  "scripts": {
    "build": "npm run build:client && npm run build:server",
    "build:client": "vite build",
    "build:server": "tsc -p tsconfig.json",
    "dev": "concurrently \"npm run dev:client\" \"npm run dev:server\"",
    "dev:client": "vite",
    "dev:server": "tsx watch server/index.ts"
  },
  "dependencies": {
    "pg": "^8.11.3",
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "tsx": "^4.19.3",
    "typescript": "^5.8.2",
    "vite": "^6.2.1"
  }
}`,
						},
						{
							Path: "tsconfig.json",
							Content: `{
  "compilerOptions": {
    "jsx": "react-jsx",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "target": "ES2020",
    "noEmit": false,
    "outDir": "dist"
  },
  "include": ["src", "server"]
}`,
						},
						{Path: "src/main.tsx", Content: `console.log("app");`},
						{Path: "server/index.ts", Content: `import express from "express"; import { Pool } from "pg"; const app = express(); const pool = new Pool(); app.listen(3001); void pool;`},
					},
				},
			},
		},
	}

	if !am.applyDeterministicPreValidationNormalization(build) {
		t.Fatalf("expected mixed root tsconfig normalization to trigger")
	}

	files := am.collectGeneratedFiles(build)
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}

	manifest := byPath["package.json"]
	if !strings.Contains(manifest, `"build:server": "tsc -p server/tsconfig.json"`) {
		t.Fatalf("expected root manifest to compile backend with server/tsconfig.json, got %s", manifest)
	}
	serverTSConfig := byPath["server/tsconfig.json"]
	if serverTSConfig == "" {
		t.Fatal("expected server/tsconfig.json to be created")
	}
	if !strings.Contains(serverTSConfig, `"moduleResolution": "NodeNext"`) {
		t.Fatalf("expected server tsconfig to use NodeNext module resolution, got %s", serverTSConfig)
	}
	if !strings.Contains(serverTSConfig, `"outDir": "../dist/server"`) {
		t.Fatalf("expected server tsconfig to emit into dist/server, got %s", serverTSConfig)
	}
}

func TestApplyDeterministicPreValidationNormalizationAddsNodeNextJSImportExtensions(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID:   "build-prevalidation-nodenext-import-extensions",
		Mode: ModeFull,
		Tasks: []*Task{
			{
				ID:     "task-generate-backend-runtime",
				Type:   TaskGenerateAPI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "package.json",
							Content: `{
  "name": "agency-ops",
  "private": true,
  "type": "module",
  "scripts": {
    "build": "tsc -p server/tsconfig.json"
  },
  "devDependencies": {
    "typescript": "^5.8.2"
  }
}`,
						},
						{
							Path: "server/tsconfig.json",
							Content: `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "outDir": "../dist/server"
  },
  "include": ["./**/*.ts"]
}`,
						},
						{
							Path: "server/index.ts",
							Content: `import apiRouter from "./routes/api";
import { pool } from "./db/models";

void pool;
export { apiRouter };`,
						},
						{
							Path: "server/routes/api.ts",
							Content: `import healthRouter from "./health";

export default healthRouter;`,
						},
						{
							Path:    "server/routes/health.ts",
							Content: `export default function health() { return "ok"; }`,
						},
						{
							Path:    "server/db/models.ts",
							Content: `export const pool = {};`,
						},
					},
				},
			},
		},
	}

	if !am.applyDeterministicPreValidationNormalization(build) {
		t.Fatalf("expected NodeNext import normalization to trigger")
	}

	files := am.collectGeneratedFiles(build)
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}

	indexFile := byPath["server/index.ts"]
	if !strings.Contains(indexFile, `from "./routes/api.js"`) {
		t.Fatalf("expected server/index.ts to use explicit .js runtime extension, got %s", indexFile)
	}
	if !strings.Contains(indexFile, `from "./db/models.js"`) {
		t.Fatalf("expected server/index.ts to patch db import extension, got %s", indexFile)
	}

	routesFile := byPath["server/routes/api.ts"]
	if !strings.Contains(routesFile, `from "./health.js"`) {
		t.Fatalf("expected server/routes/api.ts to use explicit .js runtime extension, got %s", routesFile)
	}
}

func TestNodeVerificationSkipsWhenNPMUnavailable(t *testing.T) {
	t.Setenv("PATH", "")

	am := &AgentManager{}

	frontendFiles := []GeneratedFile{
		{
			Path: "package.json",
			Content: `{
  "name": "preview-test",
  "private": true,
  "scripts": { "build": "vite build" },
  "dependencies": { "react": "^18.2.0", "react-dom": "^18.2.0" },
  "devDependencies": { "vite": "^5.0.0" }
}`,
		},
		{Path: "index.html", Content: "<!doctype html><html><body><div id=\"root\"></div></body></html>"},
		{Path: "src/main.tsx", Content: "console.log('ok')"},
	}
	if errs := am.verifyGeneratedFrontendPreviewReadiness(frontendFiles, false); len(errs) != 0 {
		t.Fatalf("expected frontend verifier to skip cleanly when npm is unavailable, got %v", errs)
	}

	backendFiles := []GeneratedFile{
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
	}
	if errs := am.verifyGeneratedBackendBuildReadiness(backendFiles, false); len(errs) != 0 {
		t.Fatalf("expected backend verifier to skip cleanly when npm is unavailable, got %v", errs)
	}
}

func TestProcessResultSuccessfulCodeTaskCapturesPatchBundleAndVerificationReport(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID:          "task-front-1",
		Type:        TaskGenerateUI,
		Description: "Refresh frontend shell",
		Status:      TaskPending,
		MaxRetries:  2,
		Input:       map[string]any{},
		CreatedAt:   time.Now(),
	}
	build := &Build{
		ID:     "build-front-1",
		Status: BuildInProgress,
		Tasks: []*Task{
			task,
			&Task{ID: "review-pending", Type: TaskReview, Status: TaskPending},
		},
		Agents:    map[string]*Agent{},
		Mode:      ModeFull,
		PowerMode: PowerBalanced,
		SnapshotFiles: []GeneratedFile{
			{
				Path:     "src/App.tsx",
				Content:  "export default function App(){ return <div>old</div> }\n",
				Language: "typescript",
			},
			{
				Path:     "package.json",
				Content:  "{\"name\":\"demo\"}\n",
				Language: "json",
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				BuildContract: &BuildContract{
					ID:      "contract-1",
					BuildID: "build-front-1",
					TruthBySurface: map[string][]TruthTag{
						string(SurfaceFrontend): {TruthScaffolded},
					},
				},
				WorkOrders: []WorkOrder{
					{
						ID:            "wo-front",
						BuildID:       "build-front-1",
						Role:          RoleFrontend,
						Category:      WorkOrderFrontend,
						TaskShape:     TaskShapeFrontendPatch,
						OwnedFiles:    []string{"src/**"},
						RequiredFiles: []string{"package.json"},
						ContractSlice: WorkOrderContractSlice{
							Surface:   SurfaceFrontend,
							TruthTags: []TruthTag{TruthScaffolded},
						},
					},
				},
			},
		},
	}
	agent := &Agent{
		ID:       "agent-front-1",
		Role:     RoleFrontend,
		Provider: ai.ProviderGPT4,
		BuildID:  build.ID,
		Status:   StatusIdle,
	}
	build.Agents[agent.ID] = agent

	am := &AgentManager{
		agents:      map[string]*Agent{agent.ID: agent},
		builds:      map[string]*Build{build.ID: build},
		taskQueue:   make(chan *Task, 2),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	if err := am.AssignTask(agent.ID, task); err != nil {
		t.Fatalf("AssignTask returned error: %v", err)
	}
	select {
	case <-am.taskQueue:
	default:
	}

	am.processResult(&TaskResult{
		TaskID:  task.ID,
		AgentID: agent.ID,
		Success: true,
		Output: &TaskOutput{
			Files: []GeneratedFile{
				{
					Path:     "src/App.tsx",
					Content:  "export default function App(){ return <main>new</main> }\n",
					Language: "typescript",
				},
			},
		},
	})

	state := build.SnapshotState.Orchestration
	if state == nil {
		t.Fatalf("expected orchestration state")
	}
	if len(state.PatchBundles) == 0 {
		t.Fatalf("expected captured patch bundle, got %+v", state)
	}
	bundle := state.PatchBundles[len(state.PatchBundles)-1]
	if bundle.WorkOrderID != "wo-front" {
		t.Fatalf("expected work order id wo-front, got %+v", bundle)
	}
	if bundle.Provider != ai.ProviderGPT4 {
		t.Fatalf("expected provider %s, got %+v", ai.ProviderGPT4, bundle)
	}
	foundPath := false
	for _, op := range bundle.Operations {
		if op.Path == "src/App.tsx" {
			foundPath = true
			break
		}
	}
	if !foundPath {
		t.Fatalf("expected patch operation for src/App.tsx, got %+v", bundle.Operations)
	}

	if len(state.VerificationReports) == 0 {
		t.Fatalf("expected task-local verification report, got %+v", state)
	}
	report := state.VerificationReports[len(state.VerificationReports)-1]
	if report.Phase != "task_local_verification" || report.Status != VerificationPassed {
		t.Fatalf("expected passed task-local verification report, got %+v", report)
	}
	if report.Surface != SurfaceFrontend || report.WorkOrderID != "wo-front" {
		t.Fatalf("expected frontend work-order verification report, got %+v", report)
	}
	if !containsTruthTagLocal(state.BuildContract.TruthBySurface[string(SurfaceFrontend)], TruthVerified) {
		t.Fatalf("expected frontend surface truth to include verified, got %+v", state.BuildContract.TruthBySurface)
	}
	if !hasProviderScorecardSample(state.ProviderScorecards, ai.ProviderGPT4, TaskShapeFrontendPatch) {
		t.Fatalf("expected GPT4 frontend patch scorecard to receive a live sample, got %+v", state.ProviderScorecards)
	}
}

func TestBuildTaskPatchBundleUsesStructuredPatchBundleDirectly(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{ID: "build-structured-bundle"}
	task := &Task{
		ID:          "task-structured-bundle",
		Type:        TaskFix,
		Description: "Repair frontend shell",
		Input: map[string]any{
			"work_order_artifact": WorkOrder{
				ID:       "wo-structured-bundle",
				Role:     RoleSolver,
				Category: WorkOrderRepair,
			},
		},
	}
	agent := &Agent{Provider: ai.ProviderClaude}
	output := &TaskOutput{
		StructuredPatchBundle: &PatchBundle{
			Operations: []PatchOperation{
				{
					Type:    PatchPatchDependency,
					Path:    "package.json",
					Content: "{\n  \"dependencies\": {\n    \"react\": \"^18.0.0\"\n  }\n}\n",
				},
				{
					Type: PatchDeleteBlock,
					Path: "src/obsolete.ts",
				},
			},
		},
	}

	bundle := am.buildTaskPatchBundle(build, agent, task, output)
	if bundle == nil {
		t.Fatal("expected structured patch bundle to be reused")
	}
	if bundle.WorkOrderID != "wo-structured-bundle" {
		t.Fatalf("expected work order id to be preserved, got %+v", bundle)
	}
	if bundle.Provider != ai.ProviderClaude {
		t.Fatalf("expected provider to be attached, got %+v", bundle)
	}
	if bundle.BuildID != build.ID {
		t.Fatalf("expected build id %s, got %+v", build.ID, bundle)
	}
	if len(bundle.Operations) != 2 {
		t.Fatalf("expected structured patch operations to be preserved, got %+v", bundle.Operations)
	}
	if bundle.Operations[0].Type != PatchPatchDependency || bundle.Operations[1].Type != PatchDeleteBlock {
		t.Fatalf("expected structured patch operation types to survive intact, got %+v", bundle.Operations)
	}
}

func TestCollectGeneratedFilesHonorsDeletedFilesFromTaskOutput(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		SnapshotFiles: []GeneratedFile{
			{
				Path:     "src/App.tsx",
				Content:  "export default function App(){ return <div>old</div> }\n",
				Language: "typescript",
			},
			{
				Path:     "src/obsolete.ts",
				Content:  "export const obsolete = true\n",
				Language: "typescript",
			},
		},
		Tasks: []*Task{
			{
				ID:     "task-delete-1",
				Type:   TaskFix,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path:     "src/App.tsx",
							Content:  "export default function App(){ return <main>new</main> }\n",
							Language: "typescript",
						},
					},
					DeletedFiles: []string{"src/obsolete.ts"},
				},
			},
		},
	}

	files := am.collectGeneratedFiles(build)
	if len(files) != 1 {
		t.Fatalf("expected deleted file to be removed from generated set, got %+v", files)
	}
	if files[0].Path != "src/App.tsx" {
		t.Fatalf("expected surviving generated file to be src/App.tsx, got %+v", files)
	}
}

func TestCollectGeneratedFilesPrefersLaterTaskPatchOverLongerEarlierContent(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		SnapshotFiles: []GeneratedFile{
			{
				Path:     "src/App.tsx",
				Content:  "export default function App(){ return <main>Bootstrapped by APEX.BUILD. The deterministic scaffold is live.</main> }\n",
				Language: "typescript",
			},
		},
		Tasks: []*Task{
			{
				ID:     "task-front-1",
				Type:   TaskGenerateUI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path:     "src/App.tsx",
							Content:  "export default function App(){ return <main>Bootstrapped by APEX.BUILD. The deterministic scaffold is live.</main> }\n",
							Language: "typescript",
						},
					},
				},
			},
			{
				ID:     "task-fix-1",
				Type:   TaskFix,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path:     "src/App.tsx",
							Content:  "import AppShell from './components/AppShell'\nexport default function App(){ return <AppShell /> }\n",
							Language: "typescript",
						},
					},
				},
			},
		},
	}

	files := am.collectGeneratedFiles(build)
	if len(files) != 1 {
		t.Fatalf("expected one collected file, got %+v", files)
	}
	if !strings.Contains(files[0].Content, "AppShell") {
		t.Fatalf("expected later repair content to win, got %q", files[0].Content)
	}
	if strings.Contains(files[0].Content, "deterministic scaffold is live") {
		t.Fatalf("expected scaffold placeholder content to be replaced, got %q", files[0].Content)
	}
}

func TestProcessResultVerificationFailureRecordsFailureFingerprintAndScorecard(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID:          "task-front-fail",
		Type:        TaskGenerateUI,
		Description: "Generate broken frontend shell",
		Status:      TaskPending,
		MaxRetries:  2,
		Input:       map[string]any{},
		CreatedAt:   time.Now(),
	}
	build := &Build{
		ID:         "build-front-fail",
		Status:     BuildInProgress,
		Tasks:      []*Task{task},
		Agents:     map[string]*Agent{},
		Mode:       ModeFull,
		PowerMode:  PowerBalanced,
		MaxRetries: 2,
		SnapshotFiles: []GeneratedFile{
			{Path: "src/App.tsx", Content: "export default function App(){ return <div>old</div> }\n", Language: "typescript"},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags:              defaultBuildOrchestrationFlags(),
				ProviderScorecards: defaultProviderScorecards("platform"),
				BuildContract: &BuildContract{
					ID:      "contract-fail",
					BuildID: "build-front-fail",
					TruthBySurface: map[string][]TruthTag{
						string(SurfaceFrontend): {TruthScaffolded},
					},
				},
				WorkOrders: []WorkOrder{
					{
						ID:            "wo-front-fail",
						BuildID:       "build-front-fail",
						Role:          RoleFrontend,
						Category:      WorkOrderFrontend,
						TaskShape:     TaskShapeFrontendPatch,
						OwnedFiles:    []string{"src/**"},
						ContractSlice: WorkOrderContractSlice{Surface: SurfaceFrontend},
					},
				},
			},
		},
	}
	agent := &Agent{
		ID:       "agent-front-fail",
		Role:     RoleFrontend,
		Provider: ai.ProviderGPT4,
		BuildID:  build.ID,
		Status:   StatusIdle,
	}
	build.Agents[agent.ID] = agent

	am := &AgentManager{
		agents:      map[string]*Agent{agent.ID: agent},
		builds:      map[string]*Build{build.ID: build},
		taskQueue:   make(chan *Task, 2),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	if err := am.AssignTask(agent.ID, task); err != nil {
		t.Fatalf("AssignTask returned error: %v", err)
	}
	select {
	case <-am.taskQueue:
	default:
	}

	am.processResult(&TaskResult{
		TaskID:  task.ID,
		AgentID: agent.ID,
		Success: true,
		Output: &TaskOutput{
			Files: []GeneratedFile{
				{
					Path:     "src/App.tsx",
					Content:  "export default function App(){\n  // TODO: finish UI\n  return <main>broken</main>\n}\n",
					Language: "typescript",
				},
			},
		},
	})

	if task.Status != TaskPending || task.RetryCount != 1 {
		t.Fatalf("expected task to be requeued after verification failure, got status=%s retries=%d", task.Status, task.RetryCount)
	}
	if len(am.taskQueue) != 1 {
		t.Fatalf("expected retried task to be requeued, queue len=%d", len(am.taskQueue))
	}

	state := build.SnapshotState.Orchestration
	if state == nil || len(state.FailureFingerprints) == 0 {
		t.Fatalf("expected failure fingerprint to be recorded, got %+v", state)
	}
	fp := state.FailureFingerprints[len(state.FailureFingerprints)-1]
	if fp.FailureClass != "verification_failure" || fp.RepairSucceeded {
		t.Fatalf("expected verification failure fingerprint, got %+v", fp)
	}
	if fp.Provider != ai.ProviderGPT4 || fp.TaskShape != TaskShapeFrontendPatch {
		t.Fatalf("expected GPT4/frontend failure fingerprint, got %+v", fp)
	}
	if len(state.VerificationReports) == 0 || state.VerificationReports[len(state.VerificationReports)-1].Status != VerificationFailed {
		t.Fatalf("expected failed verification report, got %+v", state.VerificationReports)
	}
	if !hasProviderScorecardFailure(state.ProviderScorecards, ai.ProviderGPT4, TaskShapeFrontendPatch) {
		t.Fatalf("expected GPT4 frontend patch scorecard failure sample, got %+v", state.ProviderScorecards)
	}
}

func TestAssignTaskBuildsRepairWorkOrderArtifactForFixTasks(t *testing.T) {
	t.Parallel()

	failedWorkOrder := WorkOrder{
		ID:            "wo-front-failed",
		BuildID:       "build-fix-1",
		Role:          RoleFrontend,
		Category:      WorkOrderFrontend,
		TaskShape:     TaskShapeFrontendPatch,
		Summary:       "Implement the dashboard shell",
		OwnedFiles:    []string{"src/**"},
		RequiredFiles: []string{"src/App.tsx"},
		ReadableFiles: []string{"package.json"},
		ContractSlice: WorkOrderContractSlice{Surface: SurfaceFrontend},
		SurfaceLocalChecks: []string{
			"render dashboard shell",
		},
	}
	failedTask := &Task{
		ID:          "task-failed-ui",
		Type:        TaskGenerateUI,
		Description: "Generate dashboard shell",
		Status:      TaskFailed,
		Input: map[string]any{
			"work_order_artifact": failedWorkOrder,
		},
		Output: &TaskOutput{
			Files: []GeneratedFile{
				{Path: "src/App.tsx", Content: "export default function App(){ return <div>broken</div> }\n", Language: "typescript"},
			},
		},
		CreatedAt: time.Now(),
	}
	fixTask := &Task{
		ID:          "task-fix-ui",
		Type:        TaskFix,
		Description: "Repair frontend integration failure",
		Status:      TaskPending,
		MaxRetries:  2,
		Input: map[string]any{
			"action":          "solve_build_failure",
			"failed_task_id":  failedTask.ID,
			"failure_error":   "integration: frontend calls /api/data but backend has no matching route",
			"previous_errors": []string{"integration: frontend calls /api/data but backend has no matching route"},
		},
		CreatedAt: time.Now(),
	}
	build := &Build{
		ID:         "build-fix-1",
		Status:     BuildInProgress,
		Tasks:      []*Task{failedTask, fixTask},
		Agents:     map[string]*Agent{},
		MaxRetries: 2,
		SnapshotFiles: []GeneratedFile{
			{Path: "src/App.tsx", Content: "export default function App(){ return <div>broken</div> }\n", Language: "typescript"},
			{Path: "package.json", Content: "{\"name\":\"demo\"}\n", Language: "json"},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
				BuildContract: &BuildContract{
					ID:      "contract-fix-1",
					BuildID: "build-fix-1",
					TruthBySurface: map[string][]TruthTag{
						string(SurfaceFrontend): {TruthScaffolded},
					},
				},
			},
		},
	}
	agent := &Agent{
		ID:       "agent-solver-1",
		Role:     RoleSolver,
		Provider: ai.ProviderGPT4,
		BuildID:  build.ID,
		Status:   StatusIdle,
	}
	build.Agents[agent.ID] = agent

	am := &AgentManager{
		agents:      map[string]*Agent{agent.ID: agent},
		builds:      map[string]*Build{build.ID: build},
		taskQueue:   make(chan *Task, 1),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	if err := am.AssignTask(agent.ID, fixTask); err != nil {
		t.Fatalf("AssignTask returned error: %v", err)
	}

	artifact := taskArtifactWorkOrderFromInput(fixTask)
	if artifact == nil {
		t.Fatal("expected repair work order artifact to be attached")
	}
	if artifact.Category != WorkOrderRepair || artifact.TaskShape != TaskShapeRepair {
		t.Fatalf("expected repair work order/task shape, got %+v", artifact)
	}
	if artifact.ContractSlice.Surface != SurfaceFrontend {
		t.Fatalf("expected frontend repair surface from failed task, got %+v", artifact.ContractSlice)
	}
	if artifact.RoutingMode != RoutingModeDiagnosisRepair {
		t.Fatalf("expected diagnosis-repair routing mode, got %+v", artifact)
	}
	if !containsString(artifact.OwnedFiles, "src/**") || !containsString(artifact.OwnedFiles, "src/App.tsx") {
		t.Fatalf("expected repair ownership to include failed scope and concrete file, got %+v", artifact.OwnedFiles)
	}
	hints := repairErrorStringsFromValue(fixTask.Input["repair_hints"])
	if len(hints) == 0 {
		t.Fatalf("expected repair hints to be attached, got %+v", fixTask.Input["repair_hints"])
	}
	joinedHints := strings.Join(hints, "\n")
	if !strings.Contains(joinedHints, "INTEGRATION ROUTE DRIFT") || !strings.Contains(joinedHints, "/api/data") {
		t.Fatalf("expected integration repair hint, got %+v", hints)
	}
}

func TestMarkQueuedTaskExecutionStartedPromotesPendingRetryTask(t *testing.T) {
	t.Parallel()

	build := &Build{ID: "build-retry-start"}
	agent := &Agent{
		ID:       "agent-retry-start",
		Role:     RoleDatabase,
		Provider: ai.ProviderClaude,
		Model:    "claude-sonnet-4-6",
		BuildID:  build.ID,
		Status:   StatusError,
	}
	task := &Task{
		ID:          "task-retry-start",
		Type:        TaskGenerateSchema,
		Description: "Retry schema generation",
		AssignedTo:  agent.ID,
		Status:      TaskPending,
		RetryCount:  1,
	}

	am := &AgentManager{
		subscribers: map[string][]chan *WSMessage{},
	}

	started := am.markQueuedTaskExecutionStarted(agent, task)
	if !started {
		t.Fatalf("expected pending retry task to be promoted to in-progress")
	}
	if task.Status != TaskInProgress {
		t.Fatalf("expected task status in_progress, got %s", task.Status)
	}
	if task.StartedAt == nil {
		t.Fatal("expected retry task start time to be set")
	}
	if task.AssignedTo != agent.ID {
		t.Fatalf("expected task assigned to %s, got %s", agent.ID, task.AssignedTo)
	}
	if agent.Status != StatusWorking {
		t.Fatalf("expected agent status working, got %s", agent.Status)
	}
	if agent.CurrentTask != task {
		t.Fatal("expected agent current task to be updated")
	}
}

func TestRecordTaskExecutionOutcomeAccumulatesTokenCostToRecovery(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID:          "task-recovery-metrics",
		Type:        TaskGenerateUI,
		Description: "Generate frontend shell",
		Status:      TaskPending,
		MaxRetries:  2,
		Input:       map[string]any{},
		CreatedAt:   time.Now(),
	}
	build := &Build{
		ID: "build-recovery-metrics",
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}
	agent := &Agent{
		ID:       "agent-front-metrics",
		Role:     RoleFrontend,
		Provider: ai.ProviderGPT4,
		BuildID:  build.ID,
	}
	am := &AgentManager{}

	am.recordTaskExecutionOutcome(build, agent, task, &TaskOutput{
		Metrics: map[string]any{"total_tokens": 120, "model": "gpt-5"},
	}, false, true, false, "verification_failure")

	task.RetryCount = 1
	am.recordTaskExecutionOutcome(build, agent, task, &TaskOutput{
		Metrics: map[string]any{"total_tokens": 80, "model": "gpt-5"},
	}, true, true, true, "")

	state := build.SnapshotState.Orchestration
	if state == nil || len(state.FailureFingerprints) < 2 {
		t.Fatalf("expected recovery fingerprints to be stored, got %+v", state)
	}
	latest := state.FailureFingerprints[len(state.FailureFingerprints)-1]
	if !latest.RepairSucceeded {
		t.Fatalf("expected latest fingerprint to represent successful recovery, got %+v", latest)
	}
	if latest.TokenCostToRecovery != 200 {
		t.Fatalf("expected cumulative token cost to recovery, got %+v", latest)
	}
}

func containsTruthTagLocal(tags []TruthTag, target TruthTag) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}

func hasProviderScorecardSample(scorecards []ProviderScorecard, provider ai.AIProvider, shape TaskShape) bool {
	for _, scorecard := range scorecards {
		if scorecard.Provider == provider && scorecard.TaskShape == shape && scorecard.SampleCount > 0 && scorecard.SuccessCount > 0 {
			return true
		}
	}
	return false
}

func hasProviderScorecardFailure(scorecards []ProviderScorecard, provider ai.AIProvider, shape TaskShape) bool {
	for _, scorecard := range scorecards {
		if scorecard.Provider == provider && scorecard.TaskShape == shape && scorecard.FailureEventCount > 0 {
			return true
		}
	}
	return false
}

func TestApplyDeterministicIntegrationPreflightRepairsExpressRuntime(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID: "build-express-integration-repair",
		TechStack: &TechStack{
			Frontend: "React",
			Backend:  "Express",
		},
		Plan: &BuildPlan{
			APIContract: &BuildAPIContract{
				CORSOrigins: []string{"http://localhost:5173"},
				Endpoints: []APIEndpoint{
					{Method: "GET", Path: "/api/health"},
				},
			},
		},
		SnapshotFiles: []GeneratedFile{
			{
				Path: "server/index.ts",
				Content: `import express from "express";

const app = express();
app.use(express.json());
app.listen(4000, () => {
  console.log("listening");
});
`,
				Language: "typescript",
				IsNew:    true,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}

	issues := []string{
		`integration: backend does not expose required contract endpoint /api/health`,
		`integration: backend has no CORS configuration — frontend requests will be blocked by the browser`,
		`integration: backend listens on wrong port — must use port 3001 (or $PORT) to match frontend configuration`,
	}

	if !am.applyDeterministicIntegrationPreflightRepairs(build, issues, time.Now()) {
		t.Fatal("expected deterministic integration repair to apply")
	}

	files := am.collectGeneratedFiles(build)
	content := ""
	for _, file := range files {
		if file.Path == "server/index.ts" {
			content = file.Content
			break
		}
	}
	if content == "" {
		t.Fatalf("expected repaired server entry file, got %+v", files)
	}
	for _, needle := range []string{
		`import cors from "cors";`,
		`app.use(cors({ origin: ["http://localhost:5173"], credentials: true }));`,
		`app.get("/api/health", (_req, res) => res.json({ status: "ok" }));`,
		`app.listen(Number(process.env.PORT || 3001), () => {`,
	} {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected repaired server content to contain %q, got:\n%s", needle, content)
		}
	}
}

func TestLaunchIntegrationPreflightRecoveryCreatesScopedFixTask(t *testing.T) {
	t.Parallel()

	build := &Build{
		ID:          "build-integration-recovery",
		Description: "Build a full-stack CRM with auth and dashboards",
		Status:      BuildInProgress,
		MaxRetries:  2,
		TechStack: &TechStack{
			Frontend: "React",
			Backend:  "Express",
		},
		Agents: map[string]*Agent{
			"solver-1": {
				ID:       "solver-1",
				Role:     RoleSolver,
				Provider: ai.ProviderGPT4,
				BuildID:  "build-integration-recovery",
				Status:   StatusIdle,
			},
		},
		SnapshotState: BuildSnapshotState{
			Orchestration: &BuildOrchestrationState{
				Flags: defaultBuildOrchestrationFlags(),
			},
		},
	}
	am := &AgentManager{
		agents:      map[string]*Agent{"solver-1": build.Agents["solver-1"]},
		builds:      map[string]*Build{build.ID: build},
		taskQueue:   make(chan *Task, 1),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
		ctx:         context.Background(),
	}

	issues := []string{
		`integration: frontend calls /api/auth/login but backend has no matching route`,
		`integration: frontend calls /api/dashboard/kpis but backend has no matching route`,
	}

	task, launched := am.launchIntegrationPreflightRecovery(build, issues, time.Now())
	if !launched || task == nil {
		t.Fatal("expected integration preflight recovery task to launch")
	}
	if task.Type != TaskFix {
		t.Fatalf("expected task type %s, got %s", TaskFix, task.Type)
	}
	if action, _ := task.Input["action"].(string); action != "fix_integration_contract" {
		t.Fatalf("expected integration fix action, got %+v", task.Input["action"])
	}
	if skip, _ := task.Input["skip_post_fix_validation"].(bool); !skip {
		t.Fatalf("expected integration fix to skip post-fix validation, got %+v", task.Input["skip_post_fix_validation"])
	}
	if task.AssignedTo != "solver-1" {
		t.Fatalf("expected solver assignment, got %q", task.AssignedTo)
	}
	if task.Status != TaskInProgress {
		t.Fatalf("expected in-progress integration fix task, got %s", task.Status)
	}
	if build.Status != BuildTesting || build.SnapshotState.CurrentPhase != "integration" {
		t.Fatalf("expected build to move into integration/testing state, got status=%s phase=%q", build.Status, build.SnapshotState.CurrentPhase)
	}
	hints := repairErrorStringsFromValue(task.Input["repair_hints"])
	joined := strings.Join(hints, "\n")
	for _, needle := range []string{"/api/auth/login", "/api/dashboard/kpis"} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("expected repair hints to mention %q, got %q", needle, joined)
		}
	}
}

func TestHandleTaskCompletionSkipsPostFixValidationForIntegrationPreflightFix(t *testing.T) {
	t.Parallel()

	build := &Build{
		ID: "build-skip-post-fix-validation",
		Agents: map[string]*Agent{
			"testing-1": {ID: "testing-1", Role: RoleTesting, BuildID: "build-skip-post-fix-validation"},
			"review-1":  {ID: "review-1", Role: RoleReviewer, BuildID: "build-skip-post-fix-validation"},
		},
	}
	am := &AgentManager{
		builds:      map[string]*Build{build.ID: build},
		agents:      map[string]*Agent{"testing-1": build.Agents["testing-1"], "review-1": build.Agents["review-1"]},
		taskQueue:   make(chan *Task, 2),
		resultQueue: make(chan *TaskResult, 1),
		subscribers: map[string][]chan *WSMessage{},
	}

	task := &Task{
		ID:     "fix-integration",
		Type:   TaskFix,
		Status: TaskCompleted,
		Input: map[string]any{
			"action":                   "fix_integration_contract",
			"skip_post_fix_validation": true,
		},
	}

	am.handleTaskCompletion(build.ID, task, &TaskOutput{})

	if len(build.Tasks) != 0 {
		t.Fatalf("expected no follow-up validation tasks for integration preflight fix, got %+v", build.Tasks)
	}
}

func TestApplyDeterministicExpressIntegrationRepairAddsAPIPrefixAlias(t *testing.T) {
	t.Parallel()

	am := &AgentManager{}
	build := &Build{
		ID: "build-express-api-alias-repair",
		TechStack: &TechStack{
			Frontend: "React",
			Backend:  "Express",
		},
		Tasks: []*Task{
			{
				ID:     "task-generate-api",
				Type:   TaskGenerateAPI,
				Status: TaskCompleted,
				Output: &TaskOutput{
					Files: []GeneratedFile{
						{
							Path: "server/index.ts",
							Content: `import express from "express";
import authRouter from "./routes/auth";

const app = express();
app.use(express.json());
app.use("/auth", authRouter);
app.listen(process.env.PORT || 3001);`,
						},
						{
							Path: "server/routes/auth.ts",
							Content: `import { Router } from "express";
const router = Router();
router.post("/login", (_req, res) => res.json({ ok: true }));
router.get("/me", (_req, res) => res.json({ ok: true }));
export default router;`,
						},
					},
				},
			},
		},
	}

	bundle, summary := am.applyDeterministicExpressIntegrationRepair(build, []string{
		`integration: frontend calls /api/auth/login but backend has no matching route`,
		`integration: frontend calls /api/auth/me but backend has no matching route`,
	})
	if bundle == nil {
		t.Fatal("expected express integration repair to produce a patch bundle")
	}
	if !strings.Contains(summary, "api route alias") {
		t.Fatalf("expected summary to mention api route alias, got %q", summary)
	}
	if !am.applyPatchBundleToBuild(build, bundle) {
		t.Fatal("expected patch bundle to apply")
	}

	var entry string
	for _, file := range am.collectGeneratedFiles(build) {
		if file.Path == "server/index.ts" {
			entry = file.Content
			break
		}
	}
	if entry == "" {
		t.Fatal("expected patched server/index.ts to remain present")
	}
	if !strings.Contains(entry, `req.url.startsWith("/api/")`) {
		t.Fatalf("expected Express /api alias middleware to be added, got %s", entry)
	}
}
