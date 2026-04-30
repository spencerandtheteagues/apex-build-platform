package agents

import (
	"strings"
	"testing"
)

func TestDetectAppTemplateDoesNotTreatEmbeddedMockAgentsAsAISaaS(t *testing.T) {
	t.Parallel()

	description := `Build Apex FieldOps AI, a contractor field operations platform with dashboard metrics,
job pipeline, estimate builder, crew management, settings, and an Estimate Swarm modal with simulated
Kimi K2.6, GLM-5.1, and DeepSeek V4 agent panels. All data must be in memory. No database, no external
APIs, and no real API keys are needed.`

	tmpl := DetectAppTemplate(description)
	if tmpl != nil && tmpl.ID == "ai-saas" {
		t.Fatalf("embedded/mock agent feature incorrectly selected AI SaaS template: %+v", tmpl)
	}
}

func TestDetectAppTemplateKeepsExplicitAISaaSIntent(t *testing.T) {
	t.Parallel()

	description := `Build an AI SaaS app for document analysis with BYOK, provider routing,
token usage tracking, generation history, and a model selector.`

	tmpl := DetectAppTemplate(description)
	if tmpl == nil || tmpl.ID != "ai-saas" {
		t.Fatalf("expected explicit AI SaaS prompt to select ai-saas template, got %+v", tmpl)
	}
}

func TestDetectAppTemplateSelectsAllProductionBlueprints(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		description string
		wantID      string
	}{
		{
			name:        "ai saas",
			description: "Build an AI SaaS app for document analysis with BYOK, token usage tracking, generation history, and a model selector.",
			wantID:      "ai-saas",
		},
		{
			name:        "saas dashboard",
			description: "Build an operations dashboard and admin panel management system with RBAC, audit log, reporting, data tables, team management, and workspace settings.",
			wantID:      "saas-dashboard",
		},
		{
			name:        "crm",
			description: "Build a sales CRM with lead tracking, deal tracking, pipeline stages, sales activities, weighted forecast, and a pipeline board.",
			wantID:      "crm",
		},
		{
			name:        "client portal",
			description: "Build a client portal with client login, portal access, customer documents, account dashboard, support tickets, and invoice history.",
			wantID:      "client-portal",
		},
		{
			name:        "marketplace",
			description: "Build a two-sided marketplace with vendor directory listings, buyer seller workflows, provider profiles, search filters, and booking requests.",
			wantID:      "marketplace",
		},
		{
			name:        "booking",
			description: "Build an appointment booking and scheduling app with availability calendar, time slot booking, reservation system, and consultation booking.",
			wantID:      "booking",
		},
		{
			name:        "inventory",
			description: "Build an inventory management system with warehouse management, stock movements, stock ledger, reorder points, purchase orders, and fulfillment.",
			wantID:      "inventory",
		},
		{
			name:        "project management",
			description: "Build a project management and task collaboration platform with a kanban board, sprint planning, task assignments, milestone tracking, comments, and team collaboration dashboards.",
			wantID:      "project-management",
		},
		{
			name:        "community",
			description: "Build a social community platform with user profiles, a community feed, discussion forum threads, direct messages, reactions, bookmarks, and a moderation queue.",
			wantID:      "community",
		},
		{
			name:        "landing page",
			description: "Build a premium landing page and marketing site for a startup waitlist with lead capture, email capture, demo request CTA, pricing section, testimonials, and a sales funnel.",
			wantID:      "landing-page",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpl := DetectAppTemplate(tc.description)
			if tmpl == nil || tmpl.ID != tc.wantID {
				t.Fatalf("expected %s template, got %+v", tc.wantID, tmpl)
			}
		})
	}
}

func TestAllAppTemplatesAreWiredBlueprints(t *testing.T) {
	t.Parallel()

	if len(appTemplateRegistry) != 10 {
		t.Fatalf("expected 10 app templates, got %d", len(appTemplateRegistry))
	}

	acceptancePrefixes := map[string]string{
		"ai-saas":            "ai-saas-",
		"saas-dashboard":     "dashboard-",
		"crm":                "crm-",
		"client-portal":      "portal-",
		"marketplace":        "marketplace-",
		"booking":            "booking-",
		"inventory":          "inventory-",
		"project-management": "project-",
		"community":          "community-",
		"landing-page":       "landing-",
	}

	seen := make(map[string]bool, len(appTemplateRegistry))
	for _, tmpl := range appTemplateRegistry {
		if tmpl == nil {
			t.Fatal("template registry contains nil template")
		}
		if seen[tmpl.ID] {
			t.Fatalf("duplicate template id %q", tmpl.ID)
		}
		seen[tmpl.ID] = true
		if tmpl.ID == "" || tmpl.Name == "" || tmpl.Category == "" {
			t.Fatalf("template has incomplete identity: %+v", tmpl)
		}
		if len(tmpl.Keywords) < 5 {
			t.Fatalf("template %s has too few detection keywords: %d", tmpl.ID, len(tmpl.Keywords))
		}
		if !strings.Contains(tmpl.ArchitectureContext, "ACTIVE TEMPLATE:") {
			t.Fatalf("template %s does not expose active architecture context", tmpl.ID)
		}
		if !strings.Contains(tmpl.ArchitectureContext, "SUBSYSTEM") &&
			!strings.Contains(tmpl.ArchitectureContext, "REQUIRED SUBSYSTEMS") {
			t.Fatalf("template %s does not define subsystem-level generation constraints", tmpl.ID)
		}
		if strings.TrimSpace(tmpl.CustomizationRules) == "" {
			t.Fatalf("template %s has no customization rules", tmpl.ID)
		}
		if len(tmpl.AcceptanceChecks) < 10 {
			t.Fatalf("template %s has too few acceptance checks: %d", tmpl.ID, len(tmpl.AcceptanceChecks))
		}
		prefix, ok := acceptancePrefixes[tmpl.ID]
		if !ok {
			t.Fatalf("template %s is missing an acceptance-check prefix invariant", tmpl.ID)
		}
		for _, check := range tmpl.AcceptanceChecks {
			if !strings.HasPrefix(check, prefix) {
				t.Fatalf("template %s acceptance check %q does not use prefix %q", tmpl.ID, check, prefix)
			}
		}
	}
}

func TestDetectAppTemplatesSupportsSeveralLayeredBlueprintPairs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		description string
		wantIDs     []string
	}{
		{
			name:        "marketplace with booking",
			description: "Build a two-sided contractor marketplace with searchable provider listings, provider profiles, inquiry forms, appointment booking, scheduling, availability calendar, and booking requests.",
			wantIDs:     []string{"marketplace", "booking"},
		},
		{
			name:        "client portal with project management",
			description: "Build a client portal for an agency with client login, client-facing dashboard, client-visible projects, project management, tasks, milestones, approvals, files, and messages.",
			wantIDs:     []string{"client-portal", "project-management"},
		},
		{
			name:        "community with landing page",
			description: "Build a private member community platform with user profiles, community feed, direct messages, moderation queue, plus a public marketing site landing page with waitlist and lead capture.",
			wantIDs:     []string{"community", "landing-page"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			templates := DetectAppTemplates(tc.description, 2)
			got := templateIDs(templates)
			if strings.Join(got, ",") != strings.Join(tc.wantIDs, ",") {
				t.Fatalf("expected templates %+v, got %+v", tc.wantIDs, got)
			}
		})
	}
}

func TestDetectAppTemplatesSupportsPrimaryAndSecondaryBlueprints(t *testing.T) {
	t.Parallel()

	description := `Build an AI SaaS prompt optimizer with BYOK, provider routing, token usage
tracking, and generation history. Also include a public landing page with waitlist signup,
lead capture, newsletter signup, testimonials, pricing, and a demo request CTA.`

	templates := DetectAppTemplates(description, 2)
	if len(templates) != 2 {
		t.Fatalf("expected primary + secondary templates, got %+v", templateIDs(templates))
	}
	if templates[0].ID != "ai-saas" || templates[1].ID != "landing-page" {
		t.Fatalf("expected ai-saas primary and landing-page secondary, got %+v", templateIDs(templates))
	}
}

func TestTemplateSystemContextForTemplatesIncludesSecondaryBlueprint(t *testing.T) {
	t.Parallel()

	ctx := TemplateSystemContextForTemplates([]*AppTemplate{templateMarketplace, templateBooking}, "")
	if !strings.Contains(ctx, "PRIMARY APP BLUEPRINT: Marketplace") {
		t.Fatalf("expected primary marketplace context, got %q", ctx)
	}
	if !strings.Contains(ctx, "SECONDARY APP BLUEPRINT: Booking") {
		t.Fatalf("expected secondary booking context, got %q", ctx)
	}
}

func TestDetectAppTemplateSelectsProjectManagementBlueprint(t *testing.T) {
	t.Parallel()

	description := `Build a project management and task collaboration platform with a kanban board,
sprint planning, task assignments, milestone tracking, comments, and team collaboration dashboards.`

	tmpl := DetectAppTemplate(description)
	if tmpl == nil || tmpl.ID != "project-management" {
		t.Fatalf("expected project-management template, got %+v", tmpl)
	}
	if len(tmpl.AcceptanceChecks) < 10 {
		t.Fatalf("expected project-management template to include detailed acceptance checks, got %d", len(tmpl.AcceptanceChecks))
	}
}

func TestDetectAppTemplateSelectsCommunityBlueprint(t *testing.T) {
	t.Parallel()

	description := `Build a social community platform with user profiles, a community feed,
discussion forum threads, direct messages, reactions, bookmarks, and a moderation queue.`

	tmpl := DetectAppTemplate(description)
	if tmpl == nil || tmpl.ID != "community" {
		t.Fatalf("expected community template, got %+v", tmpl)
	}
	if len(tmpl.AcceptanceChecks) < 10 {
		t.Fatalf("expected community template to include detailed acceptance checks, got %d", len(tmpl.AcceptanceChecks))
	}
}

func TestDetectAppTemplateSelectsLandingPageBlueprint(t *testing.T) {
	t.Parallel()

	description := `Build a premium landing page and marketing site for a startup waitlist with
lead capture, email capture, demo request CTA, pricing section, testimonials, and a sales funnel.`

	tmpl := DetectAppTemplate(description)
	if tmpl == nil || tmpl.ID != "landing-page" {
		t.Fatalf("expected landing-page template, got %+v", tmpl)
	}
	if len(tmpl.AcceptanceChecks) < 10 {
		t.Fatalf("expected landing-page template to include detailed acceptance checks, got %d", len(tmpl.AcceptanceChecks))
	}
}
