package agents

import (
	"fmt"
	"sort"
	"strings"
)

// AppTemplate is an architecture blueprint the build agents use when a user's prompt
// matches a known app category. It injects stable architecture patterns while
// mandating that the AI customizes domain, branding, input fields, and visual
// design uniquely per prompt — so no two builds from the same template look alike.
type AppTemplate struct {
	ID       string
	Name     string
	Category string

	// Detection — keywords that signal this template applies.
	// Scored: first template with the highest match count wins.
	Keywords []string

	// Priority breaks ties — lower number wins when two templates score equally.
	Priority int

	// ArchitectureContext is injected verbatim into the Planner and Lead system
	// prompts when this template is active.
	ArchitectureContext string

	// CustomizationRules is appended to ArchitectureContext.
	// It tells the AI how to make each build unique despite sharing a skeleton.
	CustomizationRules string

	// AcceptanceChecks are appended to the scaffold acceptance list.
	AcceptanceChecks []string
}

// ----- Template Registry -----

var appTemplateRegistry = []*AppTemplate{
	templateAISaaS,
	templateSaaSDashboard,
	templateCRM,
	templateClientPortal,
	templateMarketplace,
	templateBooking,
	templateInventory,
	templateProjectManagement,
	templateCommunity,
	templateLandingPage,
}

// DetectAppTemplate returns the best-matching template for a build description,
// or nil if no template matches with sufficient confidence.
func DetectAppTemplate(description string) *AppTemplate {
	matches := DetectAppTemplates(description, 1)
	if len(matches) == 0 {
		return nil
	}
	return matches[0]
}

// DetectAppTemplates returns the strongest matching templates for a build
// description. The first result is the primary blueprint; later results are
// optional secondary blueprints used for layered apps such as AI SaaS + Landing Page.
func DetectAppTemplates(description string, maxTemplates int) []*AppTemplate {
	normalized := strings.ToLower(strings.TrimSpace(description))
	if normalized == "" {
		return nil
	}

	type scored struct {
		tmpl  *AppTemplate
		score int
	}

	candidates := make([]scored, 0, len(appTemplateRegistry))
	for _, tmpl := range appTemplateRegistry {
		if !templateEligibleForDescription(normalized, tmpl) {
			continue
		}
		score := 0
		for _, kw := range tmpl.Keywords {
			if strings.Contains(normalized, kw) {
				score++
			}
		}
		if score > 0 {
			candidates = append(candidates, scored{tmpl, score})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	filtered := candidates[:0]
	for _, c := range candidates {
		// Require at least 2 keyword matches OR exactly 1 match on a high-signal keyword
		// to avoid false-positives on vague prompts.
		if c.score >= 2 || isHighSignalMatch(normalized, c.tmpl) {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	landingSupportMode := false
	for _, c := range filtered {
		if c.tmpl.ID != "landing-page" && c.score >= 2 {
			landingSupportMode = true
			break
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		// Landing pages are supporting surfaces when a product/app template also
		// matches. Keep them primary only for standalone marketing-site prompts.
		if landingSupportMode && filtered[i].tmpl.ID != filtered[j].tmpl.ID {
			if filtered[i].tmpl.ID == "landing-page" {
				return false
			}
			if filtered[j].tmpl.ID == "landing-page" {
				return true
			}
		}
		if filtered[i].score != filtered[j].score {
			return filtered[i].score > filtered[j].score
		}
		return filtered[i].tmpl.Priority < filtered[j].tmpl.Priority
	})
	if maxTemplates <= 0 || maxTemplates > len(filtered) {
		maxTemplates = len(filtered)
	}
	out := make([]*AppTemplate, 0, maxTemplates)
	for _, c := range filtered[:maxTemplates] {
		out = append(out, c.tmpl)
	}
	return out
}

func templateEligibleForDescription(normalized string, tmpl *AppTemplate) bool {
	if tmpl == nil {
		return false
	}
	if tmpl.ID != "ai-saas" {
		return true
	}
	return aiSaaSTemplateEligible(normalized)
}

func aiSaaSTemplateEligible(normalized string) bool {
	normalized = strings.ToLower(strings.TrimSpace(normalized))
	if normalized == "" {
		return false
	}

	explicitProductSignals := []string{
		"ai saas", "ai wrapper", "ai tool", "ai app", "ai tool factory",
		"llm app", "openai app", "anthropic app", "chatgpt wrapper", "claude wrapper",
		"chatbot", "chat bot", "prompt optimizer", "prompt tool", "content generator",
		"content generation", "document analyzer", "document analysis", "summarizer",
		"summarization", "image generation", "text generation", "code reviewer",
		"byok", "bring your own key", "token usage", "cost tracking", "ai credits",
	}
	for _, signal := range explicitProductSignals {
		if strings.Contains(normalized, signal) {
			return true
		}
	}

	// Vertical SaaS apps often contain embedded/mock agent panels or model labels
	// without being an AI-wrapper product. Do not let those references inject the
	// AI SaaS Tool Factory's auth/key-vault/metering acceptance checks.
	embeddedAgentSignals := []string{
		"mock ai", "mock adapter", "simulated streaming", "simulated ai",
		"estimate swarm", "risk agent", "proposal agent", "orchestrator",
		"agent panel", "agent panels", "model routing table",
	}
	verticalSignals := []string{
		"contractor", "field-service", "field service", "job pipeline",
		"estimate builder", "crew management", "kanban", "operations platform",
		"operations app", "dashboard", "settings page",
	}
	if containsAnySubstring(normalized, embeddedAgentSignals) && containsAnySubstring(normalized, verticalSignals) {
		return false
	}

	// A bare model/vendor name like "DeepSeek" or "Ollama" is not enough; require
	// the product itself to be an AI generation assistant/tool.
	return strings.Contains(normalized, " ai ") &&
		(strings.Contains(normalized, "assistant") ||
			strings.Contains(normalized, "generator") ||
			strings.Contains(normalized, "analyzer") ||
			strings.Contains(normalized, "copilot"))
}

func containsAnySubstring(s string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func templateRequiresRuntime(tmpl *AppTemplate) bool {
	if tmpl == nil {
		return false
	}
	switch tmpl.ID {
	case "ai-saas", "saas-dashboard", "crm", "client-portal", "marketplace",
		"booking", "inventory", "project-management", "community":
		return true
	default:
		return false
	}
}

// isHighSignalMatch returns true when a single keyword match is still definitive.
func isHighSignalMatch(normalized string, tmpl *AppTemplate) bool {
	highSignal := map[string][]string{
		"ai-saas": {
			"ai saas", "ai wrapper", "llm app", "openai app", "anthropic app",
			"byok", "chatgpt wrapper", "claude wrapper", "ai tool factory",
			// Single-word high-confidence signals for AI generation tools
			"chatbot", "llm", "openai", "anthropic", "ollama", "deepseek",
		},
		"saas-dashboard": {
			"admin panel", "admin dashboard", "management system", "control center",
			"internal tool", "operations dashboard",
		},
		"crm":           {"crm", "sales pipeline", "customer relationship management", "lead tracking", "deal tracking", "sales funnel", "pipeline board"},
		"client-portal": {"client portal", "customer portal", "patient portal", "tenant portal", "vendor portal", "partner portal", "member portal", "client login", "portal access"},
		"marketplace":   {"marketplace", "two-sided", "buyer seller", "directory", "listings", "job board", "classifieds", "vendor directory", "business directory"},
		"booking": {
			"booking app", "scheduling app", "reservation system", "appointment scheduler",
			"appointment booking", "class booking", "resource reservation", "rental reservation",
			"book an appointment", "book a session", "book a class", "consultation booking",
			"salon booking", "clinic scheduling", "availability calendar", "time slot booking",
		},
		"inventory": {
			"inventory management", "inventory app", "inventory system", "inventory tracker",
			"stock management", "warehouse management", "order management system",
			"purchase order system", "fulfillment system", "ecommerce operations",
			"pick and pack", "stock movements", "stock ledger", "reorder points",
		},
		"project-management": {
			"project management", "task management", "task tracker", "kanban board",
			"sprint planning", "agile sprints", "project tracker", "work management",
			"team collaboration", "milestone tracking", "client-visible projects",
		},
		"community": {
			"social network", "community platform", "discussion forum", "creator community",
			"content sharing platform", "private member community", "direct messages",
			"moderation queue", "user profiles", "community feed",
		},
		"landing-page": {
			"landing page", "marketing site", "waitlist", "coming soon", "product launch page",
			"lead capture", "email capture", "newsletter signup", "demo request",
			"sales funnel", "conversion page", "startup waitlist",
		},
	}
	for _, kw := range highSignal[tmpl.ID] {
		if strings.Contains(normalized, kw) {
			return true
		}
	}
	return false
}

// TemplateSystemContext returns the full context string to inject into agent
// system prompts when this template is active.
func TemplateSystemContext(tmpl *AppTemplate, userDescription string) string {
	if tmpl == nil {
		return ""
	}
	return "\n\n" + strings.TrimSpace(tmpl.ArchitectureContext) +
		"\n\n" + strings.TrimSpace(tmpl.CustomizationRules)
}

// TemplateSystemContextForTemplates injects the selected primary blueprint and
// any secondary blueprints into agent prompts. Keep the template list narrow:
// one primary plus one supporting template is enough to handle common layered
// apps without flooding the planner with unrelated blueprint text.
func TemplateSystemContextForTemplates(templates []*AppTemplate, userDescription string) string {
	if len(templates) == 0 {
		return ""
	}
	var b strings.Builder
	for i, tmpl := range templates {
		if tmpl == nil {
			continue
		}
		role := "PRIMARY"
		if i > 0 {
			role = "SECONDARY"
		}
		b.WriteString(fmt.Sprintf("\n\n%s APP BLUEPRINT: %s (%s)\n", role, tmpl.Name, tmpl.ID))
		b.WriteString(strings.TrimSpace(tmpl.ArchitectureContext))
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(tmpl.CustomizationRules))
	}
	return b.String()
}

func templatesForPlan(plan *BuildPlan) []*AppTemplate {
	if plan == nil {
		return nil
	}
	ids := make([]string, 0, 1+len(plan.SecondaryTemplateIDs))
	if id := strings.TrimSpace(plan.TemplateID); id != "" {
		ids = append(ids, id)
	}
	ids = append(ids, plan.SecondaryTemplateIDs...)
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(ids))
	out := make([]*AppTemplate, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		for _, tmpl := range appTemplateRegistry {
			if tmpl.ID == id {
				out = append(out, tmpl)
				break
			}
		}
	}
	return out
}

func templateIDs(templates []*AppTemplate) []string {
	ids := make([]string, 0, len(templates))
	for _, tmpl := range templates {
		if tmpl != nil && strings.TrimSpace(tmpl.ID) != "" {
			ids = append(ids, tmpl.ID)
		}
	}
	return ids
}

// ----- Template #1: AI SaaS Tool Factory -----

var templateAISaaS = &AppTemplate{
	ID:       "ai-saas",
	Name:     "AI SaaS Tool Factory",
	Category: "AI SaaS / AI Wrapper",
	Priority: 1,
	// Keywords are intentionally AI-specific — no generic "saas", "dashboard",
	// "admin", or "dashboard" terms here to avoid collision with template #2.
	// Rule: #1 fires when AI/LLM generation is the core product.
	//       #2 fires when managing business data through a dashboard is the core product.
	Keywords: []string{
		"llm", "openai", "anthropic", "claude", "gpt", "gemini", "grok",
		"ollama", "deepseek", "chatbot", "chat bot", "ai tool",
		"ai app", "ai saas", "ai wrapper", "language model", "generative ai",
		"summarizer", "summarization",
		"content generator", "content generation", "copy generator",
		"document analyzer", "document analysis", "pdf analyzer", "pdf analysis",
		"code reviewer", "ai assistant", "ai agent",
		"byok", "bring your own key", "model selector",
		"token usage", "cost tracking", "ai credits",
		"image generation", "text generation",
		"prompt optimizer", "prompt tool", "prompt engineer",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: AI SaaS Tool Factory
======================================
This build uses the AI SaaS Tool Factory blueprint. The following stable
architecture MUST be implemented regardless of the specific tool being built.

REQUIRED SUBSYSTEMS — all must be present and functional:

1. AUTHENTICATION
   - Sign up / log in with JWT or session-based auth
   - Protected routes — unauthenticated users redirect to login
   - User profile stored in database

2. WORKSPACE / USER SETTINGS
   - Each user has a workspace (can be implicit / single-workspace)
   - Settings page: profile, default provider, default model, theme

3. ENCRYPTED API KEY VAULT
   - Users can save their own AI provider keys (BYOK mode)
   - Keys are ONLY stored server-side, encrypted at rest
   - Raw key NEVER returned to frontend, NEVER logged, NEVER in browser devtools
   - Key status (valid/missing/error) is safe to expose, key value is not
   - "Test connection" endpoint validates the key without exposing it
   - Platform managed-credit fallback when no BYOK key is configured

4. AI PROVIDER ROUTER
   - Support at minimum: the primary provider for this tool's purpose
   - Normalize all provider calls behind one internal interface
   - Provider selector UI so users can switch (OpenAI / Anthropic / Ollama Cloud / xAI)
   - Model selector showing available models for the chosen provider
   - Graceful error handling: provider failures, rate limits, invalid keys

5. TOOL EXECUTION ENGINE
   - Main tool page: input form + output panel (split layout or chat layout)
   - System prompt + user input compiled server-side before calling provider
   - Output rendered with markdown support where appropriate
   - Copy-to-clipboard on output
   - Save / favorite output action

6. USAGE METERING (mandatory — apps without metering will lose money)
   - Record per-generation: workspace_id, user_id, provider, model,
     input_tokens, output_tokens, estimated_cost_cents, status, created_at
   - Monthly usage rollup per workspace
   - Usage visible on dashboard: generations this month, estimated spend
   - Plan-based generation limits enforced server-side

7. GENERATION HISTORY
   - History page / tab with filterable list of past generations
   - Columns: date, provider, model, prompt preview, cost, favorite, open, delete
   - Filters: provider, model, favorites, date range, text search

8. BILLING PLACEHOLDERS
   - Billing page with plan tiers (Free / Pro / Team)
   - Stripe integration wired if STRIPE_SECRET_KEY is provided, else placeholder UI
   - Free tier enforces generation limits; paid tiers unlock higher limits and history

9. DASHBOARD
   - Authenticated home screen showing:
     usage stats (generations this month, estimated AI spend),
     current plan, API key status, recent generations list

10. ADMIN PANEL (if time allows, otherwise defer with clear placeholder)
    - Workspace usage overview across all users
    - Total generations, total cost, top users

TECH STACK FOR AI SAAS APPS:
- Frontend: React + TypeScript + Tailwind CSS
- Backend: Express + TypeScript (or the user's requested stack)
- Database: PostgreSQL — tables: users, provider_keys, generations, usage_rollups, saved_outputs
- Auth: JWT (jsonwebtoken + bcryptjs) stored in httpOnly cookie or Authorization header
- API key encryption: AES-256-GCM server-side; never decrypt on frontend

ACCEPTANCE: this build is NOT complete until:
- User can sign up, log in, and access their dashboard
- User can save an API key and get back a status (not the key value)
- User can run the main tool and receive output
- Every generation is saved to the generations table
- Usage stats are visible on dashboard
- History page shows past runs
- No raw API key appears in browser network tab or console
`,
	CustomizationRules: `
CUSTOMIZATION MANDATE — NEVER PRODUCE THE SAME APP TWICE:
==========================================================
The AI SaaS Tool Factory blueprint is a skeleton. Everything visible to the
end user MUST be customized to the specific tool the user described.

Read the user's prompt carefully and extract:

A. TOOL IDENTITY
   - What is the tool called? (derive from prompt if not stated)
   - What does it do in one sentence?
   - Who is the target user? (developer / marketer / recruiter / founder / etc.)

B. MAIN INPUT FIELDS (derive from the specific tool purpose)
   Examples by tool type:
   - Prompt optimizer: rough prompt, goal, audience, tone, must-include, must-avoid, context
   - Resume optimizer: resume text, job description, target role, tone
   - Proposal writer: client need, service offered, price range, timeline, tone
   - Code reviewer: code snippet, language, review focus (security/performance/style)
   - Blog writer: topic, target audience, tone, word count, SEO keywords
   - Email writer: purpose, recipient context, desired tone, key points
   - Document summarizer: document text, summary depth, audience, format
   NEVER use generic "input" and "output" — always use domain-specific field names and labels.

C. SYSTEM PROMPT / PROMPT RECIPE
   Write a server-side prompt compiler that turns the user's input fields into
   a strong model instruction. The system prompt must be tailored to the
   specific tool domain, not generic.

D. OUTPUT FORMAT
   Choose the output rendering format that matches the tool:
   - Structured text → markdown renderer
   - Code → syntax-highlighted code block
   - Prose → clean text with copy button
   - JSON / data → formatted JSON viewer
   - Email / document → styled document preview

E. VISUAL IDENTITY
   Design the UI specifically for this tool's domain and target user.
   - Color palette: choose colors that match the tool's industry and mood
     (e.g., legal tool → deep navy/gold; marketing tool → vibrant/playful;
     developer tool → dark terminal aesthetic; HR tool → professional slate)
   - Layout: choose the layout pattern that best fits the UX
     (split console / chat workspace / wizard flow / dashboard-first)
   - Name the app and create a logo placeholder or wordmark in the header
   NEVER use a generic "blue SaaS" default. Every build must look purpose-built.

F. DEFAULT PROVIDER / MODEL
   Select the provider and model that best fits the tool's needs:
   - Fast + cheap tools (prompt optimization, short summaries): gpt-4o-mini or claude-haiku
   - Quality-critical tools (resume, proposal, legal): claude-sonnet or gpt-4o
   - Code-focused tools: gpt-4o or deepseek-coder
   - Creative tools: claude-sonnet
   Pre-configure this as the default but allow user override.

G. MONETIZATION DEFAULTS
   Set generation limits appropriate to the tool:
   - Free: 10–25 generations/month (enough to evaluate, not enough to abuse)
   - Pro: 200–500 generations/month
   - Team: 1000+ or unlimited with fair-use
`,
	AcceptanceChecks: []string{
		"ai-saas-auth: signup, login, and protected routes all function",
		"ai-saas-key-vault: provider key can be saved; raw key never returned to frontend",
		"ai-saas-tool-execution: main tool runs and output is saved to database",
		"ai-saas-usage-metering: generations table records provider/model/tokens/cost for every run",
		"ai-saas-history: history page shows at least the most recent generation",
		"ai-saas-key-security: no API key visible in browser network tab or devtools console",
	},
}

// ----- Templates #2–#10: production blueprints with routing constraints -----

var templateSaaSDashboard = &AppTemplate{
	ID:       "saas-dashboard",
	Name:     "SaaS Dashboard / Admin Panel",
	Category: "Dashboard / Admin",
	Priority: 5,
	// No "saas" standalone — that would collide with template #1.
	// This template fires on dashboard/admin/CRUD/workspace signals.
	// Template #1 fires when AI generation is the core product.
	// Template #2 fires when managing business data through a dashboard is the core product.
	Keywords: []string{
		"dashboard", "admin panel", "admin dashboard", "management system",
		"control center", "internal tool", "operations app", "operations dashboard",
		"customer management", "project management", "task management",
		"business portal", "team dashboard", "client dashboard",
		"analytics dashboard", "reporting", "reports",
		"crud", "records", "roles", "permissions", "rbac",
		"workspace", "organization", "company account",
		"audit log", "audit trail", "activity feed",
		"team management", "member management", "invite",
		"data table", "searchable", "filterable", "paginated",
		"billing", "settings", "onboarding",
		"clients", "accounts", "leads", "cases", "students", "patients",
		"jobs", "work orders", "deliverables", "invoices",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: SaaS Dashboard / Admin Panel Blueprint
========================================================
This build uses the SaaS Dashboard / Admin Panel blueprint. The following
stable architecture MUST be implemented regardless of the specific business domain.

NOTE: This template is for apps where managing business data through dashboards,
tables, forms, reports, teams, and permissions is the core product. If the core
product is AI generation, use the AI SaaS Tool Factory blueprint instead.

REQUIRED SUBSYSTEMS — all must be present and functional:

1. AUTHENTICATION
   - Sign up / log in / forgot password
   - Protected routes — unauthenticated users redirect to login
   - Onboarding flow (workspace creation on first login)

2. WORKSPACE / ORGANIZATION MODEL
   - Every user belongs to one or more workspaces
   - Every data query MUST filter by workspace_id — no cross-workspace data leakage
   - Workspace switcher in the app shell if multi-workspace
   - Workspace settings: name, slug, timezone, currency, date format

3. ROLE-BASED ACCESS CONTROL (RBAC) — 5 default roles:
   - owner: full control including billing and workspace deletion
   - admin: manage team, settings, and all records
   - manager: create and update operational records, view reports
   - member: create and update assigned work
   - viewer: read-only
   CRITICAL: permission checks MUST be server-side on every mutating API route.
   Never rely solely on UI hiding to enforce permissions.

4. DASHBOARD
   - Stat cards for key business metrics (count-based and value-based)
   - At least one chart (line, bar, or donut) using Recharts or similar
   - Recent activity feed
   - All metrics scoped to current workspace
   - Loading skeletons while data fetches

5. DATA TABLES — every list view must have ALL of these:
   - Server-side search (text filter)
   - Column sorting
   - Status/category filters
   - Pagination (page size + total count)
   - Row actions (view, edit, archive/delete)
   - Loading skeleton state
   - Empty state with CTA
   - Error state with retry
   - MOBILE: desktop table becomes stacked record cards on small screens
     NEVER a horizontally scrolling crushed table on mobile

6. CRUD MODULES with RECORD DETAIL PAGES
   - List page → detail page → edit form for every major entity
   - Soft delete (archive) by default — set archived_at, exclude from default queries
   - Related records shown on detail page

7. AUDIT LOGS — mandatory on every mutation
   - Every create, update, and archive/delete action records:
     workspace_id, actor_id, action, entity_type, entity_id,
     before_json, after_json, ip_address, created_at
   - Audit log page with search, filter by entity type, pagination
   - Before/after detail drawer on each log entry

8. ACTIVITY FEED
   - Human-readable activity messages on the dashboard and record detail pages
   - actor performed action on entity at time

9. TEAM MANAGEMENT
   - List members with name/email/role/status
   - Invite by email with role selection
   - Change member role (owner/admin only)
   - Remove member (owner/admin only)
   - Pending invite list with cancel option

10. NOTIFICATIONS
    - Notification bell in topbar with unread count
    - Dropdown list of recent notifications
    - Mark as read / mark all as read
    - Types: task_assigned, task_due_soon, project_due_soon,
      member_invited, plan_limit_warning, billing_issue

11. BILLING PAGE (when the generated app has SaaS monetization)
    - Current plan with usage meters
    - Available plan cards
    - Plan limits enforced server-side (not just hidden in UI)
    - Invoice history

12. SETTINGS PAGE — separate save action per section
    - Workspace profile (name, slug, logo, timezone, currency)
    - Team defaults
    - Notification preferences
    - Security (allowed domains, session timeout, 2FA placeholder)
    - Danger zone (transfer ownership, archive/delete workspace)

DATABASE TABLES REQUIRED:
  users, workspaces, workspace_members, workspace_invites,
  [domain entities — customized per prompt],
  activity_events, audit_logs, notifications,
  billing_customers, usage_rollups, workspace_settings

TECH STACK:
- Frontend: React + TypeScript + Tailwind CSS + Recharts (charts)
- Backend: Express + TypeScript (or user's requested stack)
- Database: PostgreSQL with real typed tables (NOT generic JSONB resource blobs)
- Auth: JWT (bcryptjs + jsonwebtoken)

NON-NEGOTIABLES:
- Every protected page must require authentication
- Every workspace-scoped query must filter by workspace_id
- Every mutating API route must check workspace membership AND role permissions
- Every create/update/archive must produce an audit log entry
- Mobile tables must render as stacked cards, not crushed desktop tables
- Soft delete (archived_at) not hard delete for business records
- Plan limits enforced server-side when billing is included
- Build must compile without TypeScript errors
`,
	CustomizationRules: `
CUSTOMIZATION MANDATE — EVERY BUILD MUST BE DOMAIN-SPECIFIC:
=============================================================
The SaaS Dashboard blueprint is a skeleton. Everything visible must be
customized for the business the user described. Generic "Customers / Projects /
Tasks" is only used when the user gives no domain signals.

A. DOMAIN LANGUAGE — rename modules and fields to match the business
   Examples:
   | User's domain        | Customers →  | Projects →       | Tasks →       |
   |----------------------|--------------|------------------|---------------|
   | Contractor           | Clients      | Jobs             | Work Orders   |
   | Real estate          | Leads        | Properties       | Showings      |
   | Agency               | Clients      | Client Projects  | Deliverables  |
   | Fitness coaching     | Members      | Programs         | Sessions      |
   | Medical office       | Patients     | Appointments     | Follow-ups    |
   | Legal / law firm     | Clients      | Cases            | Case Tasks    |
   | School admin         | Students     | Classes          | Assignments   |
   | SaaS product admin   | Accounts     | Subscriptions    | Support Tickets|
   | Construction         | Clients      | Jobs             | Work Orders   |
   Use domain language EVERYWHERE: nav, page titles, forms, tables, empty
   states, activity feed messages, audit log labels.

B. DATA MODEL — generate real typed tables for the domain
   Start from the default schema, then rename/extend the business entities.
   NEVER force every app to use generic customers/projects/tasks names.
   Generate the correct tables with the correct field names and types.

C. DASHBOARD WIDGETS — pick metrics that make sense for the business
   Contractor: active jobs, pending estimates, unpaid invoices, revenue this month
   SaaS: active accounts, MRR, trial users, churn risk, open tickets
   Agency: active clients, projects in progress, deliverables due, pending approvals
   School: total students, active classes, assignments due, attendance rate

D. NAVIGATION — match the app's workflow
   Include only the modules the app actually uses.
   Order navigation by frequency of use, not alphabetically.

E. ROLES — adapt role names and permissions to the domain
   Contractor: Owner / Office Admin / Project Manager / Field Worker / Viewer
   Agency: Owner / Admin / Account Manager / Designer / Client Viewer
   School: Administrator / Teacher / Staff / Student Viewer / Parent Viewer
   SaaS: Owner / Admin / Support Agent / Analyst / Viewer

F. VISUAL IDENTITY — derive theme from the prompt's industry and tone
   Enterprise / B2B → clean light theme, blue primary
   Technical / ops tool → dark operator theme
   Premium / executive → executive graphite
   Friendly / startup / education → startup fresh greens/blues
   NEVER produce a generic blue-on-white default for every build.

G. WORKFLOW ACTIONS — add domain-specific action buttons
   Don't just generate static CRUD. Add obvious workflow actions:
   "Convert estimate to job", "Mark invoice paid", "Complete task",
   "Assign teammate", "Approve deliverable", "Close ticket",
   "Archive client" — whatever the domain naturally needs.

H. BILLING — only include if the prompt signals SaaS monetization
   Include billing for: subscription / plans / paid users / Stripe / upgrade
   Skip or hide billing for: internal tools / private dashboards / school apps

I. COMPLEXITY SCALING
   Simple (1-2 modules, no billing, no complex permissions) → deliver core fast
   Medium (3-5 modules, teams, reports, audit logs) → full template
   High (billing, RBAC, multiple workflows, integrations) → full template + extras

J. INFER — never stop with questions when defaults are obvious
   "Build me a dashboard for my business" → clean B2B theme, classic sidebar,
   Customers / Projects / Tasks / Reports / Team / Settings, Owner/Admin/Member/Viewer
`,
	AcceptanceChecks: []string{
		"dashboard-auth: login, signup, and protected route redirect all function",
		"dashboard-workspace: every data query is scoped to workspace_id",
		"dashboard-rbac: mutating API routes check membership and role server-side",
		"dashboard-audit: every create/update/archive produces an audit_logs row",
		"dashboard-tables: list pages have search, sort, filter, pagination, empty state, loading state",
		"dashboard-mobile: tables render as stacked cards at mobile viewport, not overflow tables",
		"dashboard-soft-delete: archive/delete sets archived_at rather than hard-deleting",
	},
}

var templateCRM = &AppTemplate{
	ID:       "crm",
	Name:     "CRM / Sales Pipeline",
	Category: "CRM / Sales Pipeline",
	Priority: 4,
	// Keywords: sales-process signals. Does NOT include "dashboard" or "admin" alone
	// since those route to #2. Does NOT include "client portal" which routes to #3.
	Keywords: []string{
		"crm", "sales pipeline", "sales crm", "lead tracking", "deal tracking",
		"lead management", "deal management", "pipeline stages", "pipeline board",
		"leads", "prospects", "opportunities", "win rate", "forecast", "forecasting",
		"customer relationship", "customer relationship management",
		"contacts", "companies", "accounts", "deals",
		"sales rep", "sales reps", "sales team", "sales dashboard",
		"sales funnel", "lead source", "sales activity", "follow up", "follow-up",
		"outreach", "cold outreach", "sales tracker",
		"quotes", "proposals", "sales proposal",
		"lost reason", "deal won", "deal lost", "mark won", "mark lost",
		"pipeline value", "weighted forecast",
		"real estate crm", "contractor crm", "recruiting crm", "agency crm",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: CRM / Sales Pipeline
======================================
This build uses the CRM / Sales Pipeline blueprint. The following architecture
MUST be implemented — a generic CRUD dashboard is NOT sufficient.

CRITICAL ARCHITECTURE RULE:
  Use real typed CRM entities with proper relationships.
  Do NOT reduce CRM to generic records/record_types/record_fields tables.
  Generic CRUD kills type safety, reporting, pipeline behavior, and forecasting.

REQUIRED SUBSYSTEMS — all must be present and functional:

1. AUTHENTICATION + WORKSPACE
   - Sign up / log in with JWT or session auth
   - Workspace / organization: one workspace per team
   - workspace_members: workspace_id, user_id, role, status
   - Protected routes redirect unauthenticated users
   - Onboarding flow: create workspace + first pipeline on first login

2. ROLE-BASED ACCESS CONTROL
   - Roles: owner / admin / sales_manager / sales_rep / viewer
   - Permission map: owner > admin > sales_manager > sales_rep > viewer
   - Viewer: read-only on all CRM entities, no mutations
   - Sales rep: create/update own leads/contacts/deals/activities/tasks/quotes
   - Sales manager: convert leads, mark won/lost, read reports + forecast
   - Admin/owner: archive, settings, team management, audit log
   - All mutating API routes check role permissions server-side

3. LEADS
   - leads table: workspace_id, full_name, email, phone, company_name, title,
     source, status, score, estimated_value_cents, qualification_notes,
     converted_at, converted_contact_id, converted_company_id, converted_deal_id,
     owner_id, archived_at
   - Status: new / contacted / qualified / unqualified / converted
   - Lead conversion: creates or links contact + company + optionally a deal
   - Every conversion sets converted_at and linked IDs, creates audit log
   - Lead scoring: numeric score field, displayed on list and detail

4. CONTACTS
   - contacts table: workspace_id, company_id, full_name, email, phone, mobile,
     title, linkedin_url, lifecycle_stage, owner_id, archived_at
   - Lifecycle stages: lead / prospect / customer / former_customer / partner
   - Contact detail: related deals, activities, notes, tasks, quotes

5. COMPANIES / ACCOUNTS
   - companies table: workspace_id, name, website, industry, size,
     annual_revenue_cents, phone, address fields, owner_id, status, archived_at
   - Company detail: contacts, deals, activities, notes, tasks, quotes, revenue summary

6. PIPELINES + PIPELINE STAGES
   - pipelines table: workspace_id, name, is_default
   - pipeline_stages table: pipeline_id, name, stage_key, position, probability,
     is_won, is_lost, color
   - Default pipeline created on workspace setup
   - Default stages: New → Qualified → Discovery → Proposal → Negotiation → Won → Lost
   - Pipeline stages configurable in settings (name, order, probability, color)

7. DEALS / OPPORTUNITIES
   - deals table: workspace_id, pipeline_id, stage_id, company_id, primary_contact_id,
     lead_id, name, amount_cents, currency, probability, status, expected_close_date,
     closed_at, won_at, lost_at, lost_reason, lost_notes, next_step, owner_id, archived_at
   - Status: open / won / lost
   - Stage movement: validated server-side (stage must belong to deal's pipeline)
   - Stage movement auto-updates probability from stage default
   - Moving to won stage: sets status=won, won_at, closed_at
   - Moving to lost stage: requires lost_reason, sets status=lost, lost_at, closed_at
   - Every stage change creates activity event + audit log (before + after JSON)

8. SALES ACTIVITIES
   - sales_activities table: workspace_id, lead_id, contact_id, company_id, deal_id,
     owner_id, type, subject, description, outcome, activity_at, duration_minutes
   - Activity types: call / email / meeting / demo / proposal_sent / quote_sent /
     follow_up / note / sms / linkedin_message / site_visit / contract_sent / other
   - Activity outcomes: connected / left_voicemail / no_answer / interested /
     not_interested / meeting_booked / proposal_requested / closed_won / other
   - Activities can attach to any combination of lead/contact/company/deal
   - Activity create can optionally complete a task + create follow-up task

9. NOTES, TASKS, AND REMINDERS
   - notes table: workspace_id, [lead/contact/company/deal]_id, author_id, body
   - crm_tasks table: workspace_id, [lead/contact/company/deal]_id, assigned_to,
     title, type, status, priority, due_at, reminder_at, completed_at, archived_at
   - Task types: follow_up / call / email / meeting / demo / proposal / quote /
     contract / renewal / check_in / other
   - Task status: todo / in_progress / done / cancelled
   - Completing a task creates activity event + audit log
   - Overdue tasks (due_at < now, status != done) shown on dashboard

10. QUOTES / PROPOSALS
    - quotes table: workspace_id, deal_id, company_id, contact_id, quote_number,
      title, status, amount_cents, currency, valid_until, sent_at, accepted_at,
      declined_at, content_json, archived_at
    - quote_line_items table: quote_id, name, description, quantity, unit_price_cents,
      total_cents, position
    - Status: draft / sent / viewed / accepted / declined / expired / archived
    - Sending sets sent_at, creates activity event + audit log
    - Accepting sets accepted_at, can optionally advance deal stage
    - Quote amount = sum of line items unless manually overridden

11. SALES DASHBOARD METRICS
    - Open pipeline value: sum of amount_cents for status=open
    - Weighted forecast: sum of (amount_cents × probability/100) for status=open
    - New leads: count of status=new leads
    - Won this month: sum of amount_cents for won_at >= start of current month
    - Overdue follow-ups: count of tasks where due_at < now AND status != done
    - Pipeline by stage: bar chart of deal value per stage
    - Lead source breakdown: donut chart
    - Recent sales activity feed

12. REPORTS
    - Pipeline value by stage
    - Weighted forecast over time
    - Deals won by month (bar chart)
    - Win rate (won / (won + lost))
    - Average deal size
    - Lead source performance
    - Lead conversion rate
    - Activities by rep
    - Lost reason breakdown
    - Revenue by owner
    - All reports: filter by date range, owner, pipeline, source

13. NOTIFICATIONS + AUDIT LOG
    - notifications: workspace_id, user_id, type, title, body, read_at
    - Notification types: lead_assigned, deal_assigned, task_assigned, task_due_soon,
      task_overdue, deal_stage_changed, deal_won, deal_lost, quote_sent, quote_accepted
    - Topbar bell with unread count + dropdown + mark-as-read
    - audit_logs: workspace_id, actor_id, action, entity_type, entity_id,
      before_json, after_json, ip_address
    - Audit: create / update / archive / stage_change / lead_convert /
      deal_won / deal_lost / quote_send / quote_accept

DATABASE TABLES REQUIRED:
  users, workspaces, workspace_members, workspace_invites,
  companies, contacts, leads,
  pipelines, pipeline_stages, deals,
  sales_activities, notes, crm_tasks,
  quotes, quote_line_items,
  notifications, activity_events, audit_logs,
  usage_rollups, workspace_settings

TECH STACK:
- Frontend: React + TypeScript + Tailwind CSS + Recharts (charts)
- Backend: Express + TypeScript (or user's stack)
- Database: PostgreSQL — real typed tables, NOT generic record blobs
- Auth: JWT (bcryptjs + jsonwebtoken)

PIPELINE BOARD REQUIREMENTS:
- Desktop: Kanban columns by stage, drag-and-drop stage movement
- Mobile: stage tabs or stacked sections + explicit "Move Stage" action per card
- NEVER build a wide horizontal Trello-board-only UI that breaks on mobile
- Each deal card shows: name, company/contact, amount, probability, owner,
  expected close date, overdue task indicator

NON-NEGOTIABLES:
- Every protected route requires authentication
- Every workspace-scoped query filters by workspace_id
- Every mutating API route checks workspace membership AND role server-side
- Pipeline stage movement validated server-side (stage must belong to deal's pipeline)
- Lost stage movement requires lost_reason
- Lead conversion creates real linked contact/company/deal records
- Every create/update/archive/stage-change/convert/won/lost/quote action → audit log
- Mobile tables render as stacked cards — not overflow desktop tables
- Soft delete (archived_at) not hard delete
- Build must compile without TypeScript errors
`,
	CustomizationRules: `
CUSTOMIZATION MANDATE — EVERY CRM BUILD MUST MATCH THE SALES DOMAIN:
======================================================================
The CRM blueprint is a skeleton. Everything visible must be customized for the
sales process and industry the user described. Generic "Leads/Contacts/Deals"
is the fallback only when the prompt gives no domain signals.

A. DOMAIN LANGUAGE — rename modules to match the sales domain
   | Domain               | Companies →  | Deals →         | Extra rename     |
   |----------------------|--------------|-----------------|------------------|
   | Contractor / trades  | Clients      | Jobs/Estimates  | site_visits      |
   | Real estate          | Brokerages   | Transactions    | properties/offers|
   | Agency               | Companies    | Proposals       | discovery_calls  |
   | SaaS B2B             | Accounts     | Opportunities   | subscriptions    |
   | Recruiting           | Companies    | Placements      | candidates/jobs  |
   | Fundraising          | Orgs/Donors  | Donations/Pledges | campaigns      |
   | Insurance            | Agencies     | Policies        | renewals         |
   | Legal                | Firms/Clients| Matters         | case_tasks       |
   | Medical sales        | Practices    | Deals           | reps/territories |
   | Local service        | Clients      | Jobs            | estimates/routes |
   Use domain language EVERYWHERE: nav, page titles, forms, tables, empty states,
   pipeline stage names, activity types, reports, audit log labels.

B. PIPELINE STAGES — generate stages that match the actual sales process
   Default B2B: New → Qualified → Discovery → Proposal → Negotiation → Won → Lost
   Contractor: Lead → Estimate Sent → Estimate Approved → In Progress → Won → Lost
   Real estate: Inquiry → Showing → Offer → Under Contract → Closed → Lost
   Agency: Lead → Discovery Call → Proposal Sent → Contract → Active → Won → Lost
   Recruiting: Candidate → Screening → Interview → Offer → Placed → Lost
   SaaS: Lead → Trial → Demo → Negotiation → Closed Won → Closed Lost
   NEVER use the same generic stages when the domain has a clear sales workflow.

C. LEAD SOURCES — customize to the domain
   Local service: website, referral, google_maps, yelp, direct, repeat_client
   SaaS: website, inbound, outbound, referral, conference, partner, paid_ads
   Real estate: zillow, referral, open_house, social, direct, agent_referral
   Recruiting: linkedin, referral, job_board, direct, website, agency_referral

D. LOST REASONS — customize to the domain
   Contractor: price, chose_other_contractor, timing, no_budget, project_cancelled
   SaaS: price, chose_competitor, no_budget, bad_fit, timing, unresponsive
   Real estate: bought_elsewhere, sold_themselves, timing, financing_fell_through

E. SALES ACTIVITIES — add domain-specific types
   Real estate: showing / open_house / offer_submitted / inspection / closing
   Recruiting: screening_call / interview / reference_check / offer_extended
   Contractor: site_visit / estimate_review / permit_pulled / site_meeting

F. DASHBOARD WIDGETS — pick metrics relevant to the domain
   Contractor: active jobs, estimates pending, won revenue this month, overdue tasks
   SaaS: open pipeline, weighted forecast, trials starting, churned ARR risk
   Real estate: active listings, showings this week, offers under contract, closed this month
   Agency: active proposals, conversion rate, retainer revenue, deals closing soon

G. VISUAL IDENTITY — match the industry and user type
   Enterprise / B2B SaaS → clean light theme, blue/slate
   Technical / ops → dark operator theme, purple/blue
   Local service / trades → warm professional, orange or amber accents
   Real estate → premium dark theme, gold accents
   NEVER produce a generic blue-on-white default for every build.

H. ROLES — rename for the domain when natural
   Real estate: Broker / Agent / Admin / Viewer
   Recruiting: Manager / Recruiter / Coordinator / Viewer
   Local service: Owner / Estimator / Field Tech / Viewer
   (Keep generic Owner/Admin/Manager/Rep/Viewer when domain doesn't suggest alternatives)

I. INFER — never stop with questions when defaults are obvious
   "Build me a CRM" → classic B2B layout, Leads/Contacts/Companies/Deals/Pipeline,
   clean light theme, default stages, 5 roles
   "Build a contractor CRM" → Leads/Clients/Jobs/Estimates, warm theme, contractor stages
`,
	AcceptanceChecks: []string{
		"crm-auth: login, signup, onboarding, and protected route redirect all function",
		"crm-pipeline-created: default pipeline with stages is created on workspace setup",
		"crm-lead-convert: lead conversion creates or links real contact/company and optionally deal",
		"crm-stage-move: pipeline stage movement validates stage belongs to deal's pipeline server-side",
		"crm-lost-reason: moving deal to lost stage requires lost_reason",
		"crm-won-lost: mark-won sets won_at, mark-lost sets lost_at + lost_reason",
		"crm-rbac: mutating API routes check workspace membership AND role server-side; viewer cannot mutate",
		"crm-audit: create/update/archive/stage-change/convert/won/lost/quote actions all produce audit_log rows",
		"crm-workspace-scope: every data query filters by workspace_id",
		"crm-pipeline-board: pipeline board groups deals by stage; mobile uses tabs or move-stage action (no desktop-only drag required)",
		"crm-tables: list pages have search, sort, filter, pagination, empty state, loading state",
		"crm-mobile: tables render as stacked cards at mobile viewport",
		"crm-reports: reports page renders pipeline value, win rate, lead source, and lost reason charts",
		"crm-soft-delete: archive sets archived_at, no hard deletes on CRM records",
	},
}

var templateClientPortal = &AppTemplate{
	ID:       "client-portal",
	Name:     "Client / Customer Portal",
	Category: "Portal",
	Priority: 4,
	// Keywords: external-access signals only. Pure-internal dashboards route to #2.
	// This template fires when external clients/customers need their own login.
	Keywords: []string{
		"client portal", "customer portal", "client dashboard", "customer dashboard",
		"client login", "customer login", "client area", "client access",
		"member portal", "patient portal", "student portal", "parent portal",
		"tenant portal", "vendor portal", "partner portal",
		"agency portal", "contractor portal", "consultant portal",
		"legal portal", "accounting portal", "medical portal", "school portal",
		"approval portal", "invoice portal", "request portal",
		"secure file sharing", "signed download", "client documents",
		"project status portal", "client-facing dashboard",
		"messages with clients", "client invite", "client contact",
		"external users", "outside users", "portal access",
		"deliverables portal", "client deliverables",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Client / Customer Portal
==========================================
This build uses the Client / Customer Portal blueprint. The following
architecture MUST be implemented regardless of the specific business domain.

CRITICAL SECURITY RULE — TWO SEPARATE USER CLASSES:
  Internal users  → workspace_members table  (owner/admin/manager/staff)
  External users  → client_contacts table    (client_admin/client_member/client_viewer)
  NEVER mix them. NEVER filter client access only on the frontend.
  Every portal API route must validate client_id server-side.

REQUIRED SUBSYSTEMS — all must be present and functional:

1. AUTHENTICATION — two login paths
   - Internal login → workspace dashboard (/dashboard)
   - Client login → portal dashboard (/portal)
   - Accept-invite flow for client contacts (/auth/accept-invite)
   - Signup → onboarding → workspace creation for internal users
   - Protected routes redirect unauthenticated users

2. INTERNAL WORKSPACE MEMBERSHIP
   - workspace_members: workspace_id, user_id, role, invited_by, status, joined_at
   - Roles: owner / admin / manager / staff
   - Permission map: owner > admin > manager > staff
   - Invite by email with role selection
   - Pending invite list with cancel option

3. CLIENT ACCOUNTS
   - clients table: workspace_id, name, primary_email, company, status, notes,
     internal_notes, slug, created_by_user_id, archived_at, created_at, updated_at
   - Status: lead / active / paused / completed / archived
   - Soft delete via archived_at (never hard-delete client records)
   - Every client query must filter by workspace_id

4. CLIENT CONTACTS (portal users)
   - client_contacts table: workspace_id, client_id, user_id (nullable until accepted),
     name, email, role, portal_access (bool), status, invite_token, invite_sent_at,
     invite_accepted_at, last_active_at, archived_at
   - Roles: client_admin / client_member / client_viewer
   - portal: permission prefix for all client-side permissions
   - Client users can only access records for their own client_id

5. PORTAL PROJECTS / JOBS / CASES (domain-renamed per prompt)
   - portal_projects: workspace_id, client_id, name, status, priority, visibility,
     due_at, archived_at
   - Visibility: internal_only | client_visible
   - Client portal queries MUST filter visibility = client_visible
   - Status: planning / active / waiting_on_client / blocked / completed

6. SECURE FILE SHARING
   - portal_files: workspace_id, client_id, project_id, uploader_user_id,
     uploader_contact_id, file_name, storage_key, mime_type, size_bytes,
     category, visibility, access_scope, archived_at
   - Visibility: internal_only | client_visible | specific_contacts | project_team_only
   - storage_key stored — NOT a public URL
   - Downloads hit a backend route that validates auth + workspace + client + visibility
   - Backend generates short-lived signed URL — never permanent public URL
   - Every download creates an audit log entry
   - File categories: general/contract/invoice/proposal/deliverable/report/
     image/document/legal/medical/school/other

7. MESSAGING (two-sided threads)
   - message_threads: workspace_id, client_id, project_id, subject, status, visibility,
     last_message_at, created_by_user_id, created_by_contact_id, archived_at
   - messages: workspace_id, thread_id, sender_user_id, sender_contact_id,
     body, visibility, created_at
   - Client users only see threads for their own client_id
   - Internal-only messages never appear in portal API responses
   - Every message create updates thread last_message_at
   - Every message create notifies recipients

8. APPROVAL WORKFLOWS
   - approval_requests: workspace_id, client_id, project_id, title, description,
     status, due_at, decided_at, decided_by_contact_id, decision_note, created_by_user_id
   - Status: pending / approved / rejected / cancelled / expired
   - Client approves/rejects with optional decision note
   - Decision records decided_at, decided_by_contact_id
   - Every decision creates audit log + activity event

9. INVOICES
   - portal_invoices: workspace_id, client_id, invoice_number, amount_cents, currency,
     status, due_at, paid_at, stripe_payment_intent_id, notes, archived_at
   - Status: draft / sent / viewed / unpaid / paid / overdue / void / refunded
   - Client users only see invoices for their own client_id
   - Pay action: Stripe Checkout session or placeholder — webhook updates status
   - PDF downloads use the same signed-URL file access flow

10. REQUEST FORMS (client intake)
    - request_forms: workspace_id, title, description, schema (JSONB), status, visibility
    - form_submissions: workspace_id, client_id, contact_id, form_id, data (JSONB),
      status, reviewed_by_user_id, reviewed_at, notes, archived_at
    - Clients submit structured requests; internal team reviews and responds

11. DUAL DASHBOARDS
    - /dashboard (internal): active clients count, active projects, pending approvals,
      unpaid invoices, messages this week, recent activity feed, project status donut
    - /portal (client): active projects card, pending approvals, new files (14d),
      unpaid invoices, recent messages preview, project timeline

12. NOTIFICATIONS + AUDIT LOG
    - notifications: workspace_id, user_id OR contact_id, type, title, body,
      read_at, created_at
    - Notification types: client_invited, approval_requested, approval_decided,
      file_uploaded, invoice_sent, new_message, request_submitted, project_status_changed
    - Topbar bell with unread count, dropdown, mark-as-read
    - audit_logs: workspace_id, actor_user_id, actor_contact_id, action, entity_type,
      entity_id, diff (JSONB), ip_address, created_at
    - Audit every: create / update / archive / upload / download / message / approval / payment / request

13. PORTAL BRANDING + SETTINGS
    - Workspace profile: name, slug, logo, timezone, currency, date format
    - Portal branding: portal name, logo, primary color, accent color,
      welcome message, support email, custom footer
    - Client access defaults: default role, require email verification,
      allow client uploads, allow approvals, allow invoice payments
    - Security: allowed email domains, session timeout, file download expiry, max upload size
    - Danger zone: disable portal, archive workspace

DATABASE TABLES REQUIRED:
  users, workspaces, workspace_members, workspace_invites,
  clients, client_contacts,
  portal_projects, portal_files, message_threads, messages,
  approval_requests, portal_invoices, request_forms, form_submissions,
  activity_events, audit_logs, notifications, workspace_settings

TECH STACK:
- Frontend: React + TypeScript + Tailwind CSS
- Backend: Express + TypeScript (or user's stack)
- Database: PostgreSQL — typed tables NOT generic JSONB blobs
- Auth: JWT (bcryptjs + jsonwebtoken)
- File storage: S3-compatible (R2/S3/Minio) — never public URLs

PORTAL LAYOUT RULES:
- Internal admin: dense sidebar layout, full data tables, admin-grade controls
- Client portal: simple card-first layout, plain-language labels, mobile-first
- NEVER make client portal screens look like dense internal admin tables

NON-NEGOTIABLES:
- Internal users use workspace_members; external users use client_contacts — never mixed
- Every portal API route validates client_id server-side, not only in the frontend
- Every protected route requires authentication
- Every workspace-scoped query filters by workspace_id
- Client portal queries ALWAYS filter visibility = client_visible AND client_id = current
- File downloads require backend auth check + signed URL — no permanent public URLs
- Internal-only fields stripped from all portal API responses
- Soft delete (archived_at) not hard delete for all business records
- Every mutation produces an audit_log row
- Mobile tables render as stacked cards — never overflow desktop tables on mobile
`,
	CustomizationRules: `
CUSTOMIZATION MANDATE — EVERY PORTAL BUILD MUST BE DOMAIN-SPECIFIC:
====================================================================
The Client Portal blueprint is a skeleton. Everything visible to users MUST be
customized for the specific business and user types described in the prompt.

A. BUSINESS DOMAIN + ENTITY RENAMING
   Identify the domain and rename generic modules accordingly:
   | Domain              | "Clients" →   | "Projects" →       | External user → |
   |---------------------|---------------|--------------------|-----------------|
   | Agency              | Clients       | Client Projects    | Client          |
   | Legal / law firm    | Clients       | Cases              | Client          |
   | Construction        | Clients       | Jobs               | Client          |
   | Accounting          | Clients       | Engagements        | Client          |
   | Medical office      | Patients      | Appointments       | Patient         |
   | School              | Students      | Classes/Programs   | Parent/Student  |
   | Property mgmt       | Tenants       | Properties/Units   | Tenant          |
   | Vendor mgmt         | Vendors       | Purchase Orders    | Vendor          |
   | Software impl.      | Customers     | Implementations    | Customer        |
   | Consulting          | Clients       | Engagements        | Client          |
   Use domain language EVERYWHERE: nav, titles, forms, tables, empty states,
   activity feed, audit log labels, notifications.

B. DATA MODEL — rename and extend the base tables for the domain
   Rename portal_projects to portal_cases / portal_jobs / portal_engagements etc.
   Add domain-specific fields. Generate correct typed tables — not generic blobs.

C. CLIENT-VISIBLE MODULE SELECTION — include only what the domain needs
   Agency portal: projects, files (deliverables), messages, approvals, invoices
   Legal portal: cases, documents (contracts/legal), messages, approvals, invoices
   Medical portal: appointments, forms (intake), documents (medical), messages
   School portal: classes, assignments, documents (reports), messages
   Tenant portal: maintenance requests, documents (leases), invoices/rent, messages
   Do not bolt on modules the domain doesn't need.

D. APPROVAL WORKFLOW NAMES — match the domain
   Agency: "Approve Deliverable", "Approve Proposal", "Approve Design Proof"
   Legal: "Approve Settlement", "Approve Document", "Sign Off on Strategy"
   Construction: "Approve Change Order", "Approve Milestone", "Approve Estimate"
   Accounting: "Approve Tax Return", "Approve Financial Statement"
   Medical: "Consent Form Submission", "Approve Treatment Plan"

E. FILE CATEGORIES — narrow to what the domain uses
   Agency: deliverable / design / contract / invoice / other
   Legal: contract / brief / evidence / correspondence / court_document / other
   Medical: form / medical_record / lab_result / consent / billing / other
   School: report / assignment / permission_slip / transcript / other

F. PORTAL DASHBOARD WIDGETS — show what the client actually cares about
   Agency client: active projects, pending approvals, recent deliverables, unpaid invoices
   Legal client: active cases, documents awaiting review, invoices due
   Medical patient: upcoming appointments, pending forms, recent documents
   School parent: active classes, assignments due, recent documents, messages

G. VISUAL IDENTITY — derive from the domain's tone and audience
   Agency/creative → vibrant, modern, card-first with brand accent color
   Legal/accounting → dark navy or deep charcoal, gold or amber accents, formal
   Medical → clean white, calming blue or teal, clinical precision
   School → friendly primary colors, approachable, large type
   Property mgmt → earthy, neutral grays, professional
   Construction → industrial dark theme, orange or amber accents
   NEVER produce a generic blue-on-white for every build.

H. ROLES — rename internal and client roles to match the domain
   Construction: Owner / Project Manager / Site Supervisor / Client / Client Viewer
   Legal: Partner / Associate / Paralegal / Client / Client Viewer
   Medical: Admin / Practitioner / Staff / Patient / Authorized Representative
   School: Administrator / Teacher / Staff / Parent / Student

I. INFER — never ask unnecessary questions when defaults are obvious
   "Build a client portal for my agency" → agency portal, Clients/Projects/Deliverables/
   Messages/Approvals/Invoices, professional blue-on-dark theme, desktop + mobile
   "Build a patient portal" → Patients/Appointments/Forms/Documents/Messages,
   medical white-teal theme, mobile-first layout
`,
	AcceptanceChecks: []string{
		"portal-auth: login, signup, accept-invite, and protected route redirect all function",
		"portal-two-classes: internal users use workspace_members, client users use client_contacts — never mixed",
		"portal-client-isolation: client user cannot access another client's portal records",
		"portal-visibility: client portal queries filter visibility=client_visible server-side",
		"portal-file-security: file download validates auth+workspace+client_id+visibility before returning signed URL",
		"portal-signed-urls: no permanent public file URLs — downloads return short-lived signed URLs only",
		"portal-audit: create/update/archive/upload/download/message/approval/payment produce audit_log rows",
		"portal-internal-only: internal_only fields and records are stripped from all portal API responses",
		"portal-workspace-scope: every data query filters by workspace_id",
		"portal-mobile: client portal screens use card-first mobile layout, not overflow desktop tables",
		"portal-approvals: client can approve/reject, decision records decided_at and decided_by_contact_id",
		"portal-notifications: topbar bell with unread count, mark-as-read, client users only see their own notifications",
	},
}

var templateMarketplace = &AppTemplate{
	ID:       "marketplace",
	Name:     "Marketplace / Directory / Listings",
	Category: "Marketplace / Directory / Listings",
	Priority: 3,
	// Keywords: public-browse signals. Does NOT include "client portal" (#3) or
	// "sales pipeline" (#4). Fires when users search/browse/list/buy from public listings.
	Keywords: []string{
		"marketplace", "directory", "listings", "listing app",
		"two-sided", "multi-sided",
		"buyer", "seller", "vendor", "supplier", "provider",
		"search and filter", "browse listings", "searchable listings",
		"public profiles", "provider profiles", "vendor profiles",
		"business directory", "resource directory", "talent directory",
		"expert directory", "mentor directory", "creator directory",
		"job board", "job listings", "property listings", "real estate listings",
		"classified ads", "classifieds", "local listings",
		"service providers", "find a provider", "find a contractor",
		"reviews", "ratings", "favorites",
		"inquiry", "inquiries", "request a quote",
		"seller dashboard", "vendor dashboard", "provider dashboard",
		"moderation queue", "listing approval",
		"hire", "rent", "book a provider",
		"airbnb for", "upwork for", "fiverr for", "etsy for",
		"local marketplace", "online marketplace",
		"tutor marketplace", "contractor marketplace",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Marketplace / Directory / Listings
====================================================
This build uses the Marketplace / Directory / Listings blueprint.

FIRST — CLASSIFY MARKETPLACE MODE before designing the schema:

  DIRECTORY: public searchable profiles/listings, inquiry/contact CTA, no platform payment
  LEAD GEN MARKETPLACE: searchable providers + inquiry forms + request quotes, no required checkout
  TRANSACTIONAL MARKETPLACE: buyers + sellers + listings + checkout + orders + commission
  BOOKING MARKETPLACE: providers + listing detail + booking requests + accept/decline flow
  PROFILE DIRECTORY: people profiles + filters + favorites + contact/apply action
  CLASSIFIEDS: user-posted listings + categories + location + contact seller + expiration

  Rule: Many "marketplace" prompts are actually DIRECTORY or LEAD GEN mode.
  Default to the simplest mode that satisfies the prompt. Add orders/payments only when explicitly requested.

REQUIRED SUBSYSTEMS — all must be present and functional:

1. AUTHENTICATION + USER TYPES
   - Login / signup for buyers, sellers, and admin separately or combined
   - workspace_members: internal team (owner/admin/moderator)
   - sellers table: provider/vendor/seller profiles, onboarding_status, verification_status
   - buyers table: customer/buyer profiles (optional for directory mode)
   - user_profiles: public-facing profile record linked to user
   - Seller onboarding flow: profile → listing → submit for review → approval

2. PUBLIC LISTING BROWSE (core public surface)
   - Public homepage: hero search, category grid, featured listings
   - Browse page: listing grid with search + filters + sort + pagination
   - Listing detail page: gallery, description, seller summary, price, CTA, reviews
   - Seller/provider profile page: bio, listings, reviews, contact action
   - Category browse pages
   - ALL of these are unauthenticated public pages

3. LISTINGS — EXPLICIT STATUS + MODERATION GATES
   - listings table: workspace_id, seller_id, category_id, title, slug, description,
     listing_type, status, moderation_status, visibility, price_cents, pricing_type,
     is_remote, is_featured, average_rating, review_count, view_count, favorite_count,
     published_at, expires_at, archived_at, search_text (tsvector)
   - Status: draft / published / paused / archived / expired
   - Moderation status: pending / approved / rejected / flagged / needs_changes
   - Public browse MUST filter: status=published AND moderation_status=approved AND visibility=public AND archived_at IS NULL
   - New listings default to draft or pending — never auto-visible without approval
   - Every status/moderation change creates audit log

4. SEARCH + FILTERING
   - Keyword search (PostgreSQL full-text via search_text tsvector)
   - Category filter
   - Location filter
   - Price range filter
   - Rating filter
   - Remote/local toggle
   - Seller verified filter
   - Sort: relevance / newest / rating / price_low / price_high / featured
   - Pagination or infinite scroll
   - Mobile filter drawer (not sidebar-only)
   - All search/filter logic is server-side — never trust frontend filtering for visibility

5. SELLERS / PROVIDERS
   - Seller profile creation and onboarding wizard
   - Seller dashboard: published listings, pending listings, inquiries, messages, rating
   - Seller can create/edit/pause/archive own listings only
   - Seller routes validate listing ownership server-side (seller cannot edit another's listings)
   - verification_status: unverified / pending / verified / rejected / suspended
   - Suspended seller → all their listings hidden from public browse

6. FAVORITES
   - favorites table: workspace_id, user_id, listing_id (unique)
   - Authenticated buyers can favorite/unfavorite listings
   - Favorite action updates listing.favorite_count
   - Saved favorites page in buyer profile

7. INQUIRIES + MESSAGING
   - inquiries table: workspace_id, listing_id, seller_id, buyer_id, name, email, message, status
   - Inquiry is the default contact flow for directory and lead-gen modes
   - Inquiry statuses: new / read / responded / qualified / closed / spam / archived
   - Inquiry creation: validates listing is published + approved; notifies seller; creates audit log
   - message_threads + messages tables if full messaging is requested
   - Buyers and sellers can only access threads they are part of

8. REVIEWS + RATINGS
   - reviews table: workspace_id, listing_id, seller_id, buyer_id, rating, title, body,
     status, moderation_status, response_body, responded_at
   - Review modes: open_reviews / verified_only_reviews / admin_approved_reviews / reviews_disabled
   - Default: verified_only for transactional, admin_approved for directory
   - Review eligibility validated server-side (if verified-only: must have completed order/booking)
   - Rating 1–5, validates on create
   - Approved review triggers recalculation of listing.average_rating and seller.average_rating
   - Seller can respond to review once
   - Every review action creates audit log

9. MODERATION (mandatory — user-created public content requires moderation)
   - Moderation queue: pending listings + flagged listings + flagged reviews + reported content
   - Only admin/moderator roles can moderate
   - Moderation actions: approve / reject / request_changes / flag / archive / suspend_seller
   - Every moderation action creates moderation_events row + audit_log row
   - Rejected content stores rejection reason
   - content_reports table: reporter_user_id, entity_type, entity_id, reason, status

10. ORDERS (transactional marketplace mode only)
    - orders table: workspace_id, listing_id, seller_id, buyer_id, order_number, status,
      payment_status, subtotal_cents, platform_fee_cents, total_cents, stripe session/intent IDs
    - Order/payment statuses as defined above
    - Checkout: ALL pricing calculated server-side — never trust frontend price
    - Platform commission: calculated server-side via (subtotal × rate_bps / 10000) + fixed_fee
    - Stripe Checkout session created server-side; webhook handler must be idempotent
    - Seller payout: placeholder (Stripe Connect optional)

11. BOOKING REQUESTS (booking marketplace mode only)
    - booking_requests table: workspace_id, listing_id, seller_id, buyer_id,
      requested_start_at, requested_end_at, status, message, quoted_price_cents
    - Booking statuses: requested / accepted / declined / cancelled / completed / expired
    - Seller accepts or declines; optional payment after acceptance
    - Full calendar availability → route to Booking/Scheduling Blueprint instead

12. ADMIN DASHBOARD
    - Published listings count, pending moderation count, active sellers, new inquiries
    - Optional order volume (transactional mode)
    - Listing growth chart, category breakdown donut
    - Recent activity feed
    - Admin tables: all listings, all sellers, all buyers, all categories, moderation queue,
      reports, orders (if enabled), audit log

13. NOTIFICATIONS + AUDIT LOG
    - notifications: user_id, type, title, body, read_at
    - Notification types: listing_approved/rejected, new_inquiry, new_message, new_review,
      order_created/paid, booking_requested/accepted/declined, content_reported
    - Topbar bell with unread count + dropdown + mark-as-read
    - audit_logs: actor_id, action, entity_type, entity_id, before_json, after_json
    - Audit: create/update/archive/publish/moderate/review/order/booking/payment actions

DATABASE TABLES REQUIRED:
  users, workspaces, workspace_members,
  user_profiles, sellers, buyers, categories,
  listings, listing_media, listing_attributes,
  favorites, inquiries, message_threads, messages,
  reviews, orders, booking_requests,
  content_reports, moderation_events,
  notifications, activity_events, audit_logs, workspace_settings

TECH STACK:
- Frontend: React + TypeScript + Tailwind CSS
- Backend: Express + TypeScript (or user's stack)
- Database: PostgreSQL — real typed tables with tsvector search index on listings
- Auth: JWT (bcryptjs + jsonwebtoken)
- File storage: S3-compatible for listing media (optional for simple directory)

LAYOUT RULES:
- Public pages: clean marketing-grade layout with hero search, fast card grid, mobile drawer filters
- Seller pages: focused operational dashboard
- Admin pages: dense data tables with bulk moderation actions
- NEVER build only a desktop listing table with no mobile card layout

NON-NEGOTIABLES:
- Public browse ONLY returns status=published AND moderation_status=approved AND visibility=public
- Draft/pending/rejected/archived listings never appear in public search
- Seller routes validate listing ownership server-side
- Buyer routes validate buyer ownership server-side
- Review eligibility checked server-side
- All checkout pricing/fees calculated server-side
- Stripe webhook handler must be idempotent
- Every moderation action creates moderation_event + audit_log
- Suspended seller → all their listings removed from public browse
- Build must compile without TypeScript errors
`,
	CustomizationRules: `
CUSTOMIZATION MANDATE — EVERY MARKETPLACE/DIRECTORY BUILD MUST BE DOMAIN-SPECIFIC:
====================================================================================
The Marketplace blueprint is a skeleton. Everything visible must be customized
for the domain, listing type, and buyer/seller terminology the user described.

A. CLASSIFY MODE FIRST
   Read the prompt and pick the right mode before designing:
   - "directory" / "find a ___" / "searchable profiles" → DIRECTORY
   - "request quotes" / "get matched" / "lead generation" → LEAD GEN
   - "buy" / "checkout" / "sell digital" / "commission" → TRANSACTIONAL
   - "book an appointment" / "schedule a session" → BOOKING
   - "post an ad" / "classified" / "buy and sell" → CLASSIFIEDS
   - "talent profiles" / "expert profiles" → PROFILE DIRECTORY

B. DOMAIN LANGUAGE — rename modules to match the listing domain
   | Domain              | Sellers →    | Listings →     | Buyers →     |
   |---------------------|--------------|----------------|--------------|
   | Contractor mktpl    | Contractors  | Services       | Customers    |
   | Real estate         | Agents       | Properties     | Buyers       |
   | Job board           | Companies    | Jobs           | Job Seekers  |
   | Talent directory    | Talent       | Portfolios     | Clients      |
   | Tutor marketplace   | Tutors       | Subjects       | Students     |
   | Consultant dir.     | Consultants  | Services       | Clients      |
   | Restaurant dir.     | Restaurants  | Menu / About   | Diners       |
   | Vendor directory    | Vendors      | Products/Svc   | Buyers       |
   | Classifieds         | Sellers      | Classified Ads | Buyers       |
   Use domain language EVERYWHERE: nav, search placeholder, card labels,
   detail page headings, inquiry form, seller dashboard, admin tables, notifications.

C. CATEGORIES — generate starter categories that make sense for the domain
   Contractor: General Contractors / Roofing / Plumbing / Electrical / HVAC / ...
   Consultants: Strategy / Marketing / Finance / Operations / Software / AI / ...
   Real estate: Residential / Commercial / Land / Rental / Luxury / ...
   Job board: Engineering / Design / Marketing / Sales / Operations / ...
   Tutors: Math / Science / English / Test Prep / Coding / Languages / ...

D. LISTING FIELDS — add domain-specific fields beyond title/description/price
   Properties: bedrooms, bathrooms, sqft, lot_size, year_built, property_type
   Jobs: salary_range, employment_type, remote, experience_level, skills
   Services: service_area, duration, availability, certifications, license
   Products: condition, quantity, shipping_options, specs

E. SEARCH FILTERS — narrow to what the domain actually needs
   Properties: location, price range, bedrooms, bathrooms, property_type, for_sale/rent
   Jobs: role category, location, remote/on-site, salary range, experience level
   Services: category, location, verified, price range, remote consultation
   Products: category, price range, condition, location

F. PRICING TYPE — pick the model that matches
   Services: starting_at / hourly / quote_required / contact_for_price
   Products: fixed price
   Rentals: per_night / per_week / per_month
   Jobs: salary range (text field, not currency widget)
   Directory: free / contact_for_price

G. REVIEWS — default to the right mode for the domain
   Transactional: verified_only_reviews (must have placed order)
   Service booking: verified_only_reviews (must have completed booking)
   Directory: admin_approved_reviews (anyone can submit, admin approves)
   MVP / simple: reviews_disabled if user did not ask for reviews

H. VISUAL IDENTITY — match the domain's audience and tone
   Local services / trades → warm, trustworthy, green or amber
   Real estate → premium dark theme, gold accents
   Job board → clean professional blue/white
   Marketplace for creators / talent → vibrant, expressive
   B2B vendor directory → enterprise blue, clean and minimal
   NEVER produce the same generic theme for every build.

I. PAYMENT COMPLEXITY — match to prompt
   No checkout signal → directory mode, no orders table, no Stripe
   "buy" / "checkout" / "payments" signal → transactional mode with orders + Stripe
   Note: two-sided Stripe payouts (Stripe Connect) are complex — use placeholder payout
   system unless explicitly requested; document it clearly in the UI

J. INFER — never ask unnecessary questions
   "Build me a marketplace for contractors" → contractor directory mode, local services theme,
   Contractors/Services/Customers, inquiry-based contact, reviews, admin moderation
   "Build a job board" → job board mode, Companies/Jobs/Job Seekers, apply CTA, no checkout
`,
	AcceptanceChecks: []string{
		"marketplace-mode: template classified into correct mode (directory/lead-gen/transactional/booking/classifieds) before build",
		"marketplace-visibility: public browse only returns status=published AND moderation_status=approved AND visibility=public",
		"marketplace-draft-hidden: draft/pending/rejected/archived listings never appear in public search",
		"marketplace-seller-ownership: seller API routes validate listing ownership server-side",
		"marketplace-moderation: moderation queue exists; approve/reject/flag actions create moderation_event + audit_log",
		"marketplace-suspended-seller: suspending a seller removes all their listings from public browse",
		"marketplace-reviews: review eligibility checked server-side; approved reviews trigger rating recalculation",
		"marketplace-payments: checkout pricing calculated server-side; Stripe webhook is idempotent (transactional mode only)",
		"marketplace-search: keyword search, category filter, price/rating filters, sort, and pagination all work",
		"marketplace-mobile: listing cards and filter drawer are mobile-friendly; no desktop-only layout",
		"marketplace-audit: create/update/publish/moderate/review/order/booking actions produce audit_log rows",
		"marketplace-notifications: sellers receive inquiry/review/booking notifications; admins receive moderation alerts",
	},
}

var templateBooking = &AppTemplate{
	ID:       "booking",
	Name:     "Booking / Scheduling / Reservations",
	Category: "Booking",
	Priority: 3,
	Keywords: []string{
		"booking", "scheduling", "reservation", "appointment", "calendar",
		"availability", "time slot", "time slots", "schedule", "book a", "book an",
		"book appointment", "appointment booking", "appointment scheduler",
		"salon", "spa", "clinic", "medical scheduling", "doctor appointment",
		"dentist", "physical therapy", "therapist appointment",
		"consultant", "consultation booking", "coaching session",
		"tutor", "tutoring session", "class booking", "class scheduling",
		"fitness class", "yoga class", "gym booking", "workshop registration",
		"conference room", "room reservation", "resource reservation",
		"equipment rental", "rental booking", "venue booking", "venue reservation",
		"court booking", "coworking desk",
		"home service", "contractor scheduling", "service call",
		"cancellation policy", "rescheduling", "reminder", "booking confirmation",
		"provider schedule", "staff schedule", "intake form", "deposit booking",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Booking / Scheduling / Reservations
=====================================================

STEP 0 — CLASSIFY BOOKING MODE BEFORE ANY SCHEMA DESIGN
Examine the user's prompt and pick exactly one mode:
  appointment_booking   — 1:1 appointments (salon, clinic, coach, consultant)
  class_scheduling      — group sessions with capacity (fitness, workshops, training)
  resource_reservation  — book a room/court/desk/equipment by time block
  rental_reservation    — multi-day date-range rentals (cabin, vehicle, venue)
  consultation_booking  — expert calls/discovery sessions (lawyers, advisors, tutors)
  service_booking       — on-site/home services dispatched to customer location
  event_registration    — fixed-seat events (meetups, webinars, conferences)
Mode drives: capacity model, resource vs. provider focus, booking flow steps, UI layout.

SUBSYSTEM 1 — WORKSPACE + AUTH
workspaces(id, name, slug, owner_id, timezone NOT NULL, default_currency, booking_mode, plan, status)
workspace_members(workspace_id, user_id, role: owner|admin|manager|provider|staff|viewer)
workspace_invites(workspace_id, email, role, token UNIQUE, expires_at)
workspace_settings(workspace_id, key, value jsonb) — UNIQUE(workspace_id, key)
Every workspace-scoped query MUST filter by workspace_id. JWT auth required for all admin routes.

SUBSYSTEM 2 — CUSTOMERS
customers(workspace_id, user_id nullable, name, email, phone, timezone, notes, status, metadata jsonb, archived_at)
Customers may or may not have user accounts. Create customer record on booking if none exists.
Index: (workspace_id, email), (workspace_id, status), (workspace_id, archived_at).

SUBSYSTEM 3 — PROVIDERS
providers(workspace_id, user_id nullable, name, email, title, bio, timezone NOT NULL, status, accepts_bookings bool, color, archived_at)
provider_services(provider_id, service_id, workspace_id, custom_price_cents nullable, custom_duration_minutes nullable)
Providers have their own timezone. Slots are computed in provider timezone then returned in requested timezone.

SUBSYSTEM 4 — SERVICES
services(workspace_id, name, slug UNIQUE per workspace, duration_minutes, buffer_before_minutes, buffer_after_minutes, price_cents, deposit_cents, currency, payment_required, deposit_required, capacity default 1, location_type, status, requires_approval, requires_intake_form, cancellation_policy, reschedule_policy, archived_at)
service_resources(service_id, resource_id) — junction: which resources a service requires

SUBSYSTEM 5 — RESOURCES (for resource_reservation / rental modes)
resources(workspace_id, name, type, description, capacity, status, metadata jsonb, archived_at)
Required when booking mode involves physical rooms, courts, equipment, or assets.
Resource conflict check is independent of provider conflict check.

SUBSYSTEM 6 — AVAILABILITY ENGINE (SERVER-SIDE ONLY — NEVER TRUST FRONTEND SLOT)
availability_rules(workspace_id, provider_id nullable, resource_id nullable, service_id nullable, day_of_week 0-6, start_time, end_time, timezone, effective_start_date nullable, effective_end_date nullable, is_active bool)
schedule_exceptions(workspace_id, provider_id nullable, resource_id nullable, date, exception_type: unavailable|available_override|holiday|vacation|sick_day|special_hours|maintenance|blocked, start_time nullable, end_time nullable, notes)

Slot generation order:
1. Load service (duration, buffers, capacity).
2. Load eligible providers matching service assignment.
3. Load required resource if applicable.
4. Load availability_rules for requested date range.
5. Load schedule_exceptions for date range.
6. Load existing active bookings (status IN [pending, confirmed]) for overlap window.
7. Generate candidate slots at service duration intervals.
8. Apply buffer_before and buffer_after to each candidate (total slot span = buffer_before + duration + buffer_after).
9. Remove slots blocked by schedule_exceptions (unavailable/holiday/vacation/maintenance/blocked).
10. Apply available_override exceptions (open additional windows).
11. Remove slots that overlap existing active bookings.
12. Return available slots in the requested timezone.

SUBSYSTEM 7 — CONFLICT DETECTION (MANDATORY — RUNS SERVER-SIDE ON EVERY CREATE AND RESCHEDULE)
Provider conflict: SELECT bookings WHERE provider_id=? AND status IN ['pending','confirmed'] AND archived_at IS NULL AND start_at < newEnd AND end_at > newStart (exclude self on reschedule).
Resource conflict: same query on resource_id field.
Run BOTH checks in a single server-side transaction before creating or confirming a booking.
NEVER create a confirmed booking without passing both conflict checks in that same request.

SUBSYSTEM 8 — BOOKING LIFECYCLE
bookings(workspace_id, service_id, provider_id nullable, resource_id nullable, customer_id, start_at TIMESTAMPTZ, end_at TIMESTAMPTZ, timezone NOT NULL, status, payment_status, notes, internal_notes, intake_submission_json, rescheduled_from_booking_id nullable, cancelled_at, cancelled_by, cancellation_reason, confirmed_at, completed_at, no_show_at, archived_at)

Booking status flow:
  draft → pending → confirmed → completed
                 ↓           → no_show
  pending/confirmed → cancelled
  pending/confirmed/rescheduled → rescheduled (creates new booking, marks old as rescheduled)
  pending/confirmed → expired (system job for unpaid/unconfirmed TTL)

Payment status: not_required | unpaid | pending | paid | refunded | partially_refunded | failed

SUBSYSTEM 9 — PUBLIC BOOKING CREATION ROUTE (POST /api/public/bookings)
Exact server-side sequence — no skipping:
1. Validate workspace active.
2. Validate service active and not archived.
3. Validate provider active + service assignment if provider_id provided.
4. Validate resource active if resource_id provided.
5. Validate customer input (name required, email format if provided).
6. Validate intake form submission if service.requires_intake_form=true.
7. Recompute selected time slot availability server-side (re-run slot generation for that slot's window).
8. Check provider conflict (transaction start).
9. Check resource conflict (same transaction).
10. Calculate price and deposit server-side from service record — NEVER from request body.
11. Create or find customer record.
12. Create booking (status=pending if requires_approval or payment required; else confirmed).
13. If payment/deposit required: create Stripe Checkout session server-side, return checkout_url.
14. Create booking_reminders (24h before, 1h before, filtered to future times).
15. Create notifications for provider and admin.
16. Create activity_event and audit_log entry.
17. Return booking confirmation or checkout URL.

SUBSYSTEM 10 — CONFIRM / CANCEL / RESCHEDULE / COMPLETE / NO-SHOW ROUTES
POST /api/bookings/:id/confirm — requires bookings:confirm; re-checks conflicts before confirming; sets confirmed_at; creates reminders if missing; audit log.
POST /api/bookings/:id/cancel — validate cancellation_policy; set status=cancelled + cancelled_at + reason; cancel pending reminders; placeholder refund if paid; notify; audit log.
POST /api/bookings/:id/reschedule — validate reschedule_policy; recompute new slot server-side; check conflicts; update start_at/end_at/timezone; regenerate reminders; audit log.
POST /api/bookings/:id/complete — requires bookings:complete or provider_bookings:complete; sets completed_at; audit log.
POST /api/bookings/:id/no-show — requires bookings:no_show or provider_bookings:no_show; sets no_show_at; audit log.

Cancellation policies: anytime | minimum_notice_required | admin_only | provider_or_admin_only | non_cancelable
Reschedule policies: anytime | minimum_notice_required | admin_only | non_reschedulable

SUBSYSTEM 11 — REMINDER SYSTEM
booking_reminders(workspace_id, booking_id, reminder_type: email|sms_placeholder|in_app, scheduled_for, sent_at, status: scheduled|sent|cancelled|failed|skipped)
On booking create: create reminders for 24h-before and 1h-before (skip if scheduled_for is in the past).
On booking cancel: UPDATE booking_reminders SET status='cancelled' WHERE booking_id=? AND status='scheduled'.
On booking reschedule: cancel all existing scheduled reminders, create new ones for new start_at.
Never send reminders for cancelled bookings. Never send after start_at has passed.

SUBSYSTEM 12 — PAYMENTS + DEPOSITS
Enable only when prompt includes: payment, deposit, paid booking, Stripe, checkout, reservation fee, appointment fee, class fee, rental fee, paid consultation.
Price and deposit are ALWAYS calculated server-side from service record. Never trust price from frontend.
Stripe Checkout: create session server-side; webhook handler must be idempotent (check booking_payments for existing provider_payment_id).
booking_payments(workspace_id, booking_id, amount_cents, payment_type: full|deposit, provider_payment_id, status: pending|succeeded|failed|refunded)
Webhook: checkout.session.completed → update payment_status + booking_payments + audit_log.

SUBSYSTEM 13 — INTAKE FORMS
intake_forms(workspace_id, service_id, schema_json, is_active bool)
schema_json: { fields: [{ name, label, type: text|textarea|select|boolean|number, options[], required }] }
When service.requires_intake_form=true: render intake step in booking flow; validate required fields server-side; save to booking.intake_submission_json.
Never expose intake_submission_json in public listing APIs. Staff/admin only.

SUBSYSTEM 14 — CALENDAR VIEW + ADMIN DASHBOARD
Calendar must support: day / week / month / list views. Provider filter, service filter, resource filter, status filter. Click-to-open booking detail. Internal users can create bookings from calendar slot.
Mobile: list/day fallback.
Dashboard widgets: bookings_today, upcoming_bookings, pending_requests, cancelled_this_week, revenue_today (if payments enabled), no_show_rate, provider_utilization, recent_activity_feed.
Reports: bookings by period, bookings by provider, bookings by service, cancellation rate, no-show rate, revenue if payments enabled.

SUBSYSTEM 15 — AUDIT LOG + ACTIVITY FEED
audit_logs(workspace_id, actor_id, customer_id, action, entity_type, entity_id, before_json, after_json, ip_address, created_at)
activity_events(workspace_id, actor_id, customer_id, event_type, metadata jsonb, created_at)
Audit log actions: booking.created, booking.confirmed, booking.cancelled, booking.rescheduled, booking.completed, booking.no_show, booking.payment_received, booking.payment_refunded, reminder.sent, service.created, service.updated, provider.created, availability.updated.
Every mutating API must write an audit_log entry. No exceptions.

SUBSYSTEM 16 — PUBLIC BOOKING PAGES + INTERNAL ADMIN PAGES
Public (unauthenticated): /booking (home), /booking/services, /booking/services/:id, /booking/providers/:id (if mode supports), /booking/confirm, /booking/success, /booking/cancel, /booking/reschedule.
Internal (auth required): /dashboard, /calendar, /bookings, /bookings/:id, /customers, /customers/:id, /services, /providers, /providers/:id, /resources (if mode uses resources), /availability, /payments (if enabled), /reports, /team, /settings, /audit-log.
Provider role sees own bookings and own schedule only. Customer role (if enabled) sees own booking history only. Never mix internal staff data into public booking APIs.

ROLES: owner > admin > manager > provider > staff > customer > viewer.
Every API route validates: (1) authentication, (2) workspace_id match, (3) role permission. Providers manage only their own bookings unless role >= manager.
`,
	CustomizationRules: `
BOOKING DOMAIN CUSTOMIZATION — apply these at every layer (nav, page titles, labels, empty states, sample data):

Mode-driven entity renaming:
  appointment_booking:   Providers → use domain term (stylists/clinicians/coaches/advisors/tutors), Customers → Clients or Patients in medical domains
  class_scheduling:      Services → Classes, capacity field drives enrollment limit, show seats-remaining badge
  resource_reservation:  Resources are primary; Providers optional or hidden, Services = resource types
  rental_reservation:    Resources → Equipment/Rooms/Vehicles/Venues, date picker replaces time slot picker for multi-day spans
  consultation_booking:  Providers → Consultants/Advisors/Experts, Services → Consultation Types
  service_booking:       Providers → Technicians/Crew, add location/address field to booking form
  event_registration:    Services → Events with fixed start/end; no per-slot generation; seat count = capacity

Business domain examples:
  Salon / spa          → Stylists, Services, Clients; colors = warm/premium
  Medical / clinic     → Clinicians or Providers, Appointments, Patients; add privacy/intake emphasis
  Fitness studio       → Instructors, Classes, Members; capacity badge required on class cards
  Tutor platform       → Tutors, Sessions, Students
  Contractor           → Technicians, Service Calls, Customers; add address + job notes fields
  Conference rooms     → Rooms, Reservations, Employees; no public booking page needed
  Equipment rental     → Equipment, Rentals, Customers; date range picker
  Restaurant           → Tables (resources), Reservations, Guests; party size = capacity field
  Consulting firm      → Consultants, Consultations, Clients; intake form for project details

Availability defaults by domain:
  Salon / clinic       → Mon–Sat, 9am–6pm, 15–30min buffer after
  Fitness classes      → Daily, fixed class times, capacity 10–20
  Consultant           → Weekdays, 9am–5pm, 15min buffer before + after
  Home service         → Weekdays + Sat, 8am–6pm, 60min buffer (travel)
  Conference room      → Weekdays, 7am–8pm, 15min buffer between

Payment defaults by domain:
  Salon / spa         → Full payment at booking (default payment_required=true)
  Medical             → Payment NOT required by default (bill separately)
  Fitness classes     → Deposit or full payment depending on policy
  Consultant          → Deposit required for paid consultations
  Equipment rental    → Full payment + deposit model typical

Calendar layout defaults:
  1:1 appointments    → Week view with provider column (admin_calendar_first or appointment)
  Classes             → class_schedule layout (class_calendar + capacity_indicator)
  Rooms / resources   → resource_reservation layout (resource_grid)
  Operations-heavy    → admin_calendar_first (calendar + booking_drawer + provider/status filter)

Always use domain vocabulary in:
  - Navigation labels
  - Page headings and subheadings
  - Booking flow step labels (e.g. "Choose Stylist" not "Select Provider")
  - Calendar event titles
  - Empty state messages
  - Email/notification copy placeholders
  - Dashboard widget labels
`,
	AcceptanceChecks: []string{
		"booking-compile: app builds and passes TypeScript checks with zero errors",
		"booking-public-page: public booking home renders services without authentication",
		"booking-slot-api: GET /api/public/availability recomputes slots server-side including buffer_before and buffer_after",
		"booking-slot-api: available slots exclude confirmed and pending bookings that overlap the window",
		"booking-slot-api: available slots respect schedule_exceptions (unavailable types block slots; available_override opens extra windows)",
		"booking-create-server-side: POST /api/public/bookings recomputes selected slot server-side before creating booking — never trusts frontend slot selection",
		"booking-conflict-provider: booking creation checks provider conflict inside server-side transaction and returns 409 on double-book attempt",
		"booking-conflict-resource: booking creation checks resource conflict server-side if resource_id is present",
		"booking-price-server-side: booking price and deposit are calculated from service record on server — never from request body values",
		"booking-intake: intake form renders and required fields are validated server-side when service.requires_intake_form=true",
		"booking-lifecycle: confirm/cancel/reschedule/complete/no-show routes each enforce role permission and write audit_log entry",
		"booking-reschedule-conflict: reschedule route recomputes new slot server-side and checks conflicts before updating booking",
		"booking-reminders-create: booking creation inserts booking_reminders rows for 24h-before and 1h-before (only for future times)",
		"booking-reminders-cancel: booking cancellation sets all pending booking_reminders to status=cancelled",
		"booking-reminders-reschedule: booking reschedule cancels existing scheduled reminders and creates new ones for updated start_at",
		"booking-payment-webhook: Stripe checkout.session.completed webhook updates payment_status and booking_payments and is idempotent (no duplicate payment records on replay)",
		"booking-audit-log: every create/confirm/cancel/reschedule/complete/no-show/payment action produces audit_log row with before_json and after_json",
		"booking-workspace-scope: every API query filters by workspace_id; no cross-workspace data leakage",
		"booking-auth-gate: all /dashboard, /calendar, /bookings, /customers, /providers, /services, /availability routes redirect unauthenticated users",
		"booking-role-permissions: provider role can only read/act on own assigned bookings; customer role can only view own booking history",
		"booking-calendar: calendar view renders day/week/month/list modes; provider and service filters work; clicking a booking opens detail",
		"booking-dashboard: dashboard widgets (bookings_today, upcoming_bookings, pending_requests, cancelled_this_week) load with real data",
		"booking-public-isolation: public booking APIs do not expose internal_notes, intake_submission_json, or provider contact details to unauthenticated callers",
	},
}

var templateInventory = &AppTemplate{
	ID:       "inventory",
	Name:     "Inventory / Orders / E-commerce Operations",
	Category: "Operations",
	Priority: 3,
	Keywords: []string{
		"inventory", "stock", "warehouse", "order management", "orders", "sales orders",
		"sku", "skus", "variants", "barcode", "product catalog", "product management",
		"vendor", "supplier", "purchase order", "purchase orders", "procurement",
		"reorder", "reorder point", "low stock", "stock alert",
		"asset tracking", "asset management", "equipment inventory", "tool inventory",
		"fulfillment", "pick and pack", "pick list", "packing", "shipping",
		"stock adjustment", "stock transfer", "stock count", "cycle count",
		"receiving", "goods receipt", "bin", "warehouse location",
		"returns", "rma", "return merchandise",
		"ecommerce operations", "order fulfillment", "retail backoffice",
		"parts inventory", "supply tracker", "inventory tracker",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Inventory / Orders / E-commerce Operations
============================================================

STEP 0 — CLASSIFY INVENTORY MODE BEFORE ANY SCHEMA DESIGN
Examine the user's prompt and pick exactly one mode:
  basic_inventory          — products/SKUs + stock levels + adjustments + low-stock alerts (simplest)
  warehouse_inventory      — multi-location + receiving + picking + cycle counts + transfers
  retail_pos_backoffice    — stores + customers + sales orders + returns + daily summaries
  ecommerce_order_ops      — online orders + fulfillment + shipments + returns + payment placeholders
  vendor_purchase_mgmt     — vendors + purchase orders + receiving + reorder points + cost tracking
  rental_inventory         — assets + check-out/check-in + availability + damage tracking + deposits
  manufacturing_light      — raw materials + finished goods + bill-of-materials placeholder + production runs
Mode drives: entity naming, which modules to build, fulfillment flow complexity, mobile layout.

SUBSYSTEM 1 — WORKSPACE + AUTH
workspaces(id, name, slug, owner_id, default_currency, inventory_mode, plan, status)
workspace_members(workspace_id, user_id, role: owner|admin|operations_manager|warehouse_staff|purchasing|sales_staff|viewer)
workspace_settings(workspace_id, key, value jsonb) — UNIQUE(workspace_id, key)
Every workspace-scoped query MUST filter by workspace_id. JWT auth required for all routes.

SUBSYSTEM 2 — PRODUCT CATALOG
product_categories(workspace_id, parent_id nullable, name, slug UNIQUE per workspace, position, is_active)
products(workspace_id, category_id, name, slug UNIQUE per workspace, description, brand, product_type: physical|digital|service, status, image_url, archived_at)
skus(workspace_id, product_id, sku_code UNIQUE per workspace, barcode UNIQUE when provided, name, variant_options jsonb, unit_of_measure, cost_cents, price_cents, currency, weight_grams, status, track_inventory bool, allow_backorder bool, archived_at)
Product → SKU is 1:many. SKU is the trackable/sellable unit. Products can be archived without deleting stock history. SKU code uniqueness enforced per workspace.

SUBSYSTEM 3 — INVENTORY LEDGER (THE CRITICAL RULE — NON-NEGOTIABLE)
stock_levels(workspace_id, sku_id, location_id, quantity_on_hand, quantity_reserved, quantity_available, quantity_incoming, reorder_point, reorder_quantity) — UNIQUE(workspace_id, sku_id, location_id)
stock_movements(workspace_id, sku_id, location_id, movement_type, quantity_delta, quantity_before, quantity_after, reference_type, reference_id, reason, notes, created_by)

INVENTORY IS A LEDGER, NOT AN EDITABLE NUMBER. Rules:
  - NEVER allow direct edits to stock_levels.quantity_on_hand from a form.
  - Every stock quantity change MUST create a stock_movements row AND update stock_levels in the same DB transaction.
  - quantity_available = quantity_on_hand - quantity_reserved.
  - Positive quantity_delta = stock in. Negative = stock out.

Movement types: initial_stock | manual_adjustment | purchase_receipt | sales_reservation | sales_unreservation | sales_fulfillment | return_restock | return_damaged | transfer_out | transfer_in | cycle_count_adjustment | inventory_write_off | inventory_found | production_consumption | production_output | rental_checkout | rental_checkin

createStockMovement() must run inside db.$transaction: upsert stock_level → compute quantityBefore/quantityAfter → reject if quantityAfter < 0 unless allow_negative enabled → insert stock_movement → update stock_level → return movement.

SUBSYSTEM 4 — INVENTORY LOCATIONS
inventory_locations(workspace_id, name, code UNIQUE per workspace, location_type: warehouse|store|virtual|truck, address fields, status, is_default, archived_at)
Locations are where stock physically lives. Multi-location requires stock_levels per sku+location pair.

SUBSYSTEM 5 — STOCK TRANSFERS
stock_transfers(workspace_id, transfer_number, from_location_id, to_location_id, status: draft|submitted|in_transit|received|cancelled, notes, submitted_at, received_at)
stock_transfer_items(transfer_id, sku_id, quantity_requested, quantity_received)
Transfer OUT creates negative transfer_out stock_movement at source location. Transfer IN creates positive transfer_in stock_movement at destination. For MVP: submit+receive in one step. For warehouse mode: support in_transit state.

SUBSYSTEM 6 — VENDORS + PURCHASE ORDERS
vendors(workspace_id, name, contact_name, email, phone, website, address fields, status, notes, archived_at)
vendor_skus(vendor_id, sku_id, workspace_id, vendor_sku_code, vendor_cost_cents, lead_time_days, minimum_order_quantity, is_preferred) — UNIQUE(vendor_id, sku_id)
purchase_orders(workspace_id, vendor_id, location_id, po_number UNIQUE per workspace, status: draft|submitted|approved|partially_received|received|cancelled|closed, subtotal_cents, tax_cents, shipping_cents, total_cents, currency, expected_at, submitted_at, approved_at, received_at, cancelled_at, created_by, approved_by, archived_at)
purchase_order_items(purchase_order_id, sku_id, quantity_ordered, quantity_received, unit_cost_cents, line_total_cents)

PO lifecycle: draft → submitted → approved (if required) → partially_received → received → closed.
PO totals ALWAYS calculated server-side. Receiving: validate quantities; prevent over-receiving unless enabled; create purchase_receipt stock_movements per item; update quantity_received on items; update PO status; audit_log.
Partial receiving is supported — PO stays partially_received until all items received.

SUBSYSTEM 7 — CUSTOMERS + SALES ORDERS
customers(workspace_id, name, email, phone, company, address fields, status, notes, archived_at)
sales_orders(workspace_id, customer_id, location_id, order_number UNIQUE per workspace, status: draft|confirmed|cancelled|closed, payment_status: unpaid|pending|paid|partially_refunded|refunded|failed|not_required, fulfillment_status: unfulfilled|partially_fulfilled|fulfilled|cancelled|returned, subtotal_cents, discount_cents, tax_cents, shipping_cents, total_cents, currency, shipping_address_json, billing_address_json, notes, internal_notes, confirmed_at, paid_at, cancelled_at, fulfilled_at, archived_at)
sales_order_items(sales_order_id, sku_id, quantity_ordered, quantity_reserved, quantity_fulfilled, unit_price_cents, line_total_cents)

Order totals ALWAYS calculated server-side (calculateOrderTotals from line items, never from request body).
Order confirmation: check available stock unless backorders allowed → reserve stock (update quantity_reserved + quantity_available in stock_levels) → set status=confirmed → create sales_reservation stock_movements → audit_log.
Order cancellation: release reservations (sales_unreservation movements) → update stock_levels → audit_log.

SUBSYSTEM 8 — FULFILLMENT (PICK → PACK → SHIP)
fulfillments(workspace_id, sales_order_id, location_id, fulfillment_number UNIQUE per workspace, status: pending|picking|picked|packing|packed|shipped|cancelled, picked_at, packed_at, shipped_at, cancelled_at)
fulfillment_items(fulfillment_id, sales_order_item_id, sku_id, quantity)
shipments(workspace_id, fulfillment_id, sales_order_id, shipment_number, carrier, service_level, tracking_number, status: label_pending|shipped|in_transit|delivered|failed, shipped_at, delivered_at)

Fulfillment CANNOT ship more quantity than ordered/reserved.
Ship action (POST /api/fulfillment/:id/ship):
  1. Validate fulfillment:ship permission + workspace.
  2. Verify fulfillment is packed or ready.
  3. For each item: create negative sales_fulfillment stock_movement; reduce quantity_reserved; increment quantity_fulfilled on sales_order_item.
  4. Create shipment record if carrier/tracking provided.
  5. Update fulfillment status=shipped; update order fulfillment_status.
  6. Audit_log.

SUBSYSTEM 9 — RETURNS
returns(workspace_id, sales_order_id, customer_id, return_number UNIQUE per workspace, status: requested|approved|rejected|received|completed|cancelled, reason, resolution, refund_status: not_required|pending|approved|issued|rejected, approved_at, received_at, completed_at, cancelled_at, archived_at)
return_items(return_id, sales_order_item_id nullable, sku_id, quantity, condition: new|opened|used|damaged|defective|unknown, restock_action: restock|quarantine|discard|repair|inspect)

Return receiving MUST decide restock_action per item:
  - restock: create positive return_restock stock_movement → adds to sellable stock.
  - quarantine/discard/damage: do NOT add to quantity_on_hand unless explicitly configured.
  - repair/inspect: hold in non-sellable state (no stock_movement until resolved).
Return completion can trigger refund placeholder. All return actions create audit_log.

SUBSYSTEM 10 — LOW STOCK + REORDER
Low-stock condition: stock_levels.quantity_available <= stock_levels.reorder_point (when reorder_point is set).
Reorder suggestion includes: SKU, product, location, available qty, reorder_point, suggested reorder qty, preferred vendor (from vendor_skus.is_preferred=true), vendor cost, lead_time_days.
Low-stock page loads active SKUs with active locations where available <= reorder_point, with quick "Create PO" action.

SUBSYSTEM 11 — CYCLE COUNTS
Cycle count behavior: select location → count SKU quantities → compare to quantity_on_hand → differences create cycle_count_adjustment stock_movements with reason + counted_by + counted_at → audit_log.
NEVER overwrite inventory without going through stock_movements.

SUBSYSTEM 12 — AUDIT LOG + ACTIVITY FEED
audit_logs(workspace_id, actor_id, action, entity_type, entity_id, before_json, after_json, ip_address, created_at)
activity_events(workspace_id, actor_id, entity_type, entity_id, action, message, metadata jsonb, created_at)
Every stock-changing action, every order lifecycle event, every PO action, every fulfillment action, every return action MUST write an audit_log row. No exceptions.
Audit actions: product.created/updated/archived, sku.created/updated, stock.adjusted, stock.transferred, po.created/submitted/approved/received/cancelled, order.created/confirmed/cancelled/paid/fulfilled, fulfillment.picked/packed/shipped, return.requested/approved/received/completed.

SUBSYSTEM 13 — DASHBOARD + REPORTS
Dashboard widgets: total_skus, inventory_value (sum of quantity_on_hand * cost_cents per SKU), low_stock_items count, open_orders count, pending_fulfillments count, open_purchase_orders count, sales_by_day chart, recent_activity.
Reports: inventory valuation, low stock + reorder suggestions, stock movement history, inventory by location, sales by SKU, sales by product, orders by status, fulfillment performance, POs by vendor, receiving history, returns by reason, dead stock / slow movers, top selling products, gross revenue placeholder.
All reports filter by workspace_id + support date range + location + product/category filters.

SUBSYSTEM 14 — PAGES
Internal (auth required, workspace required): /dashboard, /products, /products/:id, /skus, /skus/:id, /inventory, /inventory/adjustments, /inventory/movements, /inventory/counts, /inventory/low-stock, /locations, /locations/:id, /vendors, /vendors/:id, /purchase-orders, /purchase-orders/:id, /receiving, /customers, /customers/:id, /orders, /orders/:id, /fulfillment, /fulfillment/pick-list, /fulfillment/packing, /fulfillment/shipments, /returns, /returns/:id, /reports, /team, /settings, /audit-log.

ROLES: owner > admin > operations_manager > warehouse_staff > purchasing > sales_staff > viewer.
warehouse_staff: receives stock, picks/packs/ships, cycle counts, adjustments.
purchasing: creates/manages vendors and POs only.
sales_staff: creates orders and customers; no inventory write access except returns.
viewer: read-only across all modules.
Every mutating API: (1) validate auth, (2) validate workspace_id, (3) validate role permission. Viewer role can never mutate data.
`,
	CustomizationRules: `
INVENTORY DOMAIN CUSTOMIZATION — rename modules, entities, and labels to match the user's business language:

Mode-driven entity renaming:
  basic_inventory:       Products/SKUs → use domain term; Locations = single default warehouse (hide multi-location UI)
  warehouse_inventory:   Locations → Warehouses/Bins; add zone/bin fields; show transfer + cycle count prominently
  retail_pos_backoffice: Locations → Stores; Orders → Sales / Transactions; show daily-sales summary widget
  ecommerce_order_ops:   Orders → Customer Orders; Fulfillment is primary flow; Shipments with tracking prominent
  vendor_purchase_mgmt:  Vendors/POs/Receiving are primary modules; simplify or hide fulfillment
  rental_inventory:      Products → Equipment/Assets; Orders → Rentals/Reservations; status = checked-out/checked-in; show damage reports
  manufacturing_light:   Products → Raw Materials + Finished Goods; add BOM placeholder; production_consumption/output movement types

Business domain examples:
  Restaurant supplies    → Ingredients, Suppliers, Stock Counts, Waste Adjustments; no customer/order module
  Parts warehouse        → Parts, Part Numbers, Bins placeholder, Vendors, Receiving, Customer Orders
  Retail store           → Products, Stores, Customers, Sales, Returns, Daily Summary
  E-commerce ops         → Products, Variants, Warehouses, Orders, Fulfillment, Shipments, Returns
  Equipment rental       → Equipment, Check-out, Check-in, Availability, Damage Reports
  Medical/lab supplies   → Supplies, Expiry tracking placeholder, Storage Locations, Purchase Orders
  Tool crib              → Tools, Tool Numbers, Check-out, Check-in, Maintenance placeholder
  Auto parts             → Parts, SKUs, Bins, Vendors, Customer Orders, Fulfillment

Mobile warehouse layout:
  Use scan/search-first layout — big input at top, results below.
  Show: SKU code, product name, location, on-hand quantity, primary action button.
  Actions: Receive, Adjust, Pick, Pack, Ship, Count — each must be one large tap-friendly button.
  Avoid dense desktop tables on warehouse mobile views.
  warehouse_inventory and ecommerce_order_ops modes require a dedicated mobile-optimized warehouse section.

Inventory rules defaults by domain:
  Restaurant supplies    → allow_negative=false, require_adjustment_reason=true, no backorders
  Parts warehouse        → allow_backorder=false for critical parts; reorder_point per SKU
  Retail store           → allow_backorder optional; reserve_on_confirmation=true
  E-commerce ops         → allow_backorder configurable; auto-create fulfillment on confirmation
  Equipment rental       → no backorders; check-out reduces available; check-in returns stock

Always use domain vocabulary in:
  - Navigation labels (e.g. "Check-out" not "Fulfill", "Ingredients" not "Products")
  - Page titles and headings
  - Table column headers
  - Form labels
  - Empty state messages (e.g. "No parts in stock" not "No inventory")
  - Dashboard widget labels
  - Activity feed messages
  - Audit log event names
`,
	AcceptanceChecks: []string{
		"inventory-compile: app builds and passes TypeScript checks with zero errors",
		"inventory-ledger: stock adjustment route creates stock_movement row AND updates stock_levels in same DB transaction — no direct edits to quantity_on_hand",
		"inventory-ledger-negative: stock movement that would produce quantity_after < 0 is rejected unless allow_negative is enabled",
		"inventory-movement-types: stock movements use correct typed movement_type (purchase_receipt, sales_fulfillment, return_restock, etc.) not a generic 'adjustment'",
		"inventory-po-receiving: POST /api/purchase-orders/:id/receive creates purchase_receipt stock_movements per line item and updates stock_levels",
		"inventory-po-totals: purchase order totals are calculated server-side from line items — never trusted from request body",
		"inventory-po-over-receive: receiving route rejects quantities exceeding quantity_ordered unless over-receiving is enabled",
		"inventory-order-totals: sales order totals are calculated server-side from line items",
		"inventory-order-confirm: order confirmation checks available stock (unless backorders allowed), reserves stock via sales_reservation movements, and sets fulfillment_status=unfulfilled",
		"inventory-order-cancel: order cancellation releases reservations via sales_unreservation movements and updates stock_levels.quantity_available",
		"inventory-fulfillment-ship: ship action creates negative sales_fulfillment stock_movements, releases reserved quantities, and updates sales_order_items.quantity_fulfilled",
		"inventory-fulfillment-limit: fulfillment cannot ship more quantity than ordered/reserved — validated server-side",
		"inventory-return-restock: return receiving with restock action creates positive return_restock stock_movement adding to sellable stock",
		"inventory-return-damage: return receiving with quarantine/discard/damage action does NOT add to quantity_on_hand",
		"inventory-cycle-count: cycle count adjustment creates cycle_count_adjustment stock_movement with before/after quantities",
		"inventory-low-stock: low-stock page loads SKUs where quantity_available <= reorder_point and shows preferred vendor + reorder quantity",
		"inventory-audit-log: every stock change, order lifecycle event, PO action, fulfillment action, and return action produces audit_log row",
		"inventory-workspace-scope: every API query filters by workspace_id; no cross-workspace data leakage possible",
		"inventory-auth-gate: all operations routes redirect unauthenticated users",
		"inventory-role-permissions: warehouse_staff cannot create orders; sales_staff cannot adjust inventory; viewer cannot mutate any data",
		"inventory-dashboard: dashboard widgets (total_skus, inventory_value, low_stock_items, open_orders, pending_fulfillments, open_purchase_orders) load with real data",
		"inventory-mobile: mobile warehouse views use card layout with large action buttons (Receive, Adjust, Pick, Pack, Ship, Count) — no dense desktop tables",
	},
}

var templateProjectManagement = &AppTemplate{
	ID:       "project-management",
	Name:     "Project Management / Task Management / Collaboration",
	Category: "Project Management / Task Management / Collaboration",
	Priority: 4,
	Keywords: []string{
		"project management", "task management", "task tracker", "kanban",
		"kanban board", "task board", "project tracker", "work management",
		"team collaboration", "team tasks", "collaboration", "trello", "asana",
		"jira", "sprint", "sprints", "agile", "backlog", "milestone",
		"milestones", "deliverables", "assign", "assignee", "due date",
		"priority", "subtasks", "checklists", "dependencies", "time tracking",
		"client-visible projects", "construction project tracking", "content calendar",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Project Management / Task Management / Collaboration Blueprint
==============================================================================

STEP 0 — CLASSIFY PROJECT MANAGEMENT MODE BEFORE SCHEMA DESIGN
Pick exactly one mode:
  simple_task_manager          — tasks, statuses, assignees, due dates, simple dashboard
  team_project_management      — workspaces, projects, project members, tasks, comments
  agency_project_delivery      — clients, deliverables, approvals, milestones, time tracking
  software_kanban              — epics, stories/tasks, backlog, board, releases
  agile_sprints                — backlog, sprints, velocity, story points, retrospectives placeholder
  operations_workflow          — recurring operational work, checklists, SLAs, handoffs
  client_visible_projects      — internal vs client-visible tasks, comments, files
  construction_project_tracking — jobs, phases, crews, punch lists, site notes
  content_calendar             — campaigns, drafts, approvals, publishing dates

Mode drives entity naming, workflow states, dashboard metrics, board layout,
mobile behavior, and whether sprints/time tracking/client visibility are primary.

SUBSYSTEM 1 — AUTH + WORKSPACE
workspaces(id, name, slug, owner_id, default_timezone, default_currency, project_mode, status)
workspace_members(workspace_id, user_id, role: owner|admin|project_manager|member|client_viewer|viewer, status)
workspace_settings(workspace_id, key, value jsonb)
Every protected route requires authentication. Every workspace-scoped query filters by workspace_id.

SUBSYSTEM 2 — PROJECTS
projects(workspace_id, client_id nullable, name, slug, description, status, health, start_date, due_date, completed_at, owner_id, visibility, archived_at)
Project status: planning | active | on_hold | completed | archived
Project health: on_track | at_risk | blocked | complete
Project detail pages show overview, board, list, calendar, timeline placeholder, files, settings, activity, and members.

SUBSYSTEM 3 — PROJECT MEMBERS + ROLES
project_members(project_id, user_id, role: project_owner|manager|contributor|client_viewer|viewer)
Project-scoped access must validate project membership or workspace-level permission.
Client-visible projects must never expose internal-only tasks, comments, attachments, or fields.

SUBSYSTEM 4 — TASKS
tasks(workspace_id, project_id, parent_task_id nullable, title, description, status_id, priority, assignee_id, reporter_id, start_date, due_date, completed_at, position, estimate_minutes, actual_minutes, client_visible bool, archived_at)
Task priority: low | medium | high | urgent
Tasks support subtasks, checklists, labels, assignees, due dates, attachments, comments, mentions, dependencies, and activity history.
Task lifecycle and status moves are validated server-side.

SUBSYSTEM 5 — STATUSES + BOARDS
task_statuses(workspace_id, project_id nullable, name, status_key, category: todo|in_progress|review|done|blocked, position, color, is_done)
Kanban board columns are backed by persisted statuses. Moving a card updates status_id and position server-side.
Server route for board moves must validate status belongs to same project/workspace and persist ordering.
Mobile board uses status tabs or stacked status sections, not a crushed horizontal desktop board.

SUBSYSTEM 6 — SUBTASKS + CHECKLISTS
subtasks(workspace_id, task_id, title, status, assignee_id, due_date, position, completed_at)
task_checklist_items(task_id, title, checked, position, completed_by, completed_at)
Completing all required checklist items can optionally enable task completion.

SUBSYSTEM 7 — DEPENDENCIES
task_dependencies(workspace_id, predecessor_task_id, successor_task_id, dependency_type: blocks|relates_to)
Dependency creation must reject self-dependencies, cross-project dependencies unless explicitly allowed, and circular dependency chains.
Blocked tasks show dependency warnings on board/list/detail.

SUBSYSTEM 8 — LABELS + MILESTONES
task_labels(workspace_id, name, color)
task_label_assignments(task_id, label_id)
milestones(workspace_id, project_id, name, due_date, status, description)
Milestone progress = done tasks / total linked tasks. Overdue milestone indicators appear on dashboard.

SUBSYSTEM 9 — SPRINTS + BACKLOG (ONLY WHEN MODE NEEDS IT)
sprints(workspace_id, project_id, name, goal, start_date, end_date, status: planned|active|completed|cancelled)
backlog is a task list filtered to unscheduled or unassigned sprint tasks.
Sprint completion locks sprint metrics and creates activity/audit records.

SUBSYSTEM 10 — COMMENTS, MENTIONS, ATTACHMENTS
comments(workspace_id, task_id, author_id, body, visibility: internal|client, archived_at)
mentions are parsed server-side and create notifications for mentioned members.
attachments(workspace_id, task_id, comment_id nullable, uploaded_by, file_name, mime_type, size_bytes, storage_key, visibility)
Attachment downloads validate workspace/project/task access before returning a URL.

SUBSYSTEM 11 — TIME TRACKING (WHEN REQUESTED OR MODE BENEFITS)
time_entries(workspace_id, task_id, user_id, minutes, description, started_at, ended_at, billable bool)
Reports show total time by project, member, task status, and date range.

SUBSYSTEM 12 — DASHBOARD + REPORTS
Dashboard widgets should include active projects, tasks due soon, overdue tasks, blocked tasks, completed this week, workload by member, project health, upcoming milestones, recent activity.
Reports include task throughput, cycle time, work by status, workload by assignee, overdue trend, milestone progress, sprint velocity when enabled, time by project when enabled.

SUBSYSTEM 13 — NOTIFICATIONS + ACTIVITY + AUDIT
notifications(workspace_id, user_id, type, title, body, entity_type, entity_id, read_at)
activity_events(workspace_id, actor_id, entity_type, entity_id, action, message, metadata jsonb)
audit_logs(workspace_id, actor_id, action, entity_type, entity_id, before_json, after_json, ip_address)
Every create, update, archive, assignment, status move, reorder, dependency change, comment, attachment, milestone, sprint, and time-entry action writes audit_log.

SUBSYSTEM 14 — PAGES
Auth pages: login, signup, forgot-password, accept-invite, onboarding.
Core pages: dashboard, projects, project detail, project board, project list, project calendar, project timeline placeholder, backlog/sprints when enabled, tasks, task detail, my work, reports, team, notifications, settings, audit log.

NON-NEGOTIABLES:
- Every protected route requires auth.
- Every workspace-scoped query filters by workspace_id.
- Every project-scoped query validates project membership or workspace-level permission.
- Every mutating API route checks role permissions server-side.
- Task moves between statuses and order changes are validated and persisted server-side.
- Dependency creation prevents circular dependencies.
- Client-visible data never leaks internal-only tasks, comments, attachments, or fields.
- Attachments require server-side access checks before download.
- Every major list has loading, empty, and error states.
- Mobile task views are card-first and action-focused.
- Build must compile without TypeScript errors.
`,
	CustomizationRules: `
PROJECT MANAGEMENT DOMAIN CUSTOMIZATION — rename entities and workflows to match the user's domain.

Mode-driven vocabulary:
  simple_task_manager          → Tasks, Lists, My Work, Today
  team_project_management      → Projects, Tasks, Milestones, Team
  agency_project_delivery      → Clients, Projects, Deliverables, Approvals, Time
  software_kanban              → Epics, Stories, Bugs, Backlog, Releases
  agile_sprints                → Backlog, Sprints, Velocity, Story Points
  operations_workflow          → Workflows, Requests, SLAs, Handoffs
  client_visible_projects      → Client Projects, Client Notes, Approval Queue
  construction_project_tracking → Jobs, Phases, Crews, Punch Lists, Site Notes
  content_calendar             → Campaigns, Content Items, Reviews, Publish Calendar

Workflow defaults:
  Software: Backlog → Ready → In Progress → Code Review → QA → Done
  Agency: Brief → In Progress → Internal Review → Client Review → Approved → Delivered
  Construction: Planned → Scheduled → In Progress → Inspection → Punch List → Complete
  Content: Idea → Drafting → Editing → Scheduled → Published
  Simple: Todo → Doing → Done

Dashboard metrics must match the domain:
  Software: sprint progress, open bugs, blocked work, velocity, releases at risk
  Agency: active deliverables, approvals waiting, overdue tasks, billable hours
  Construction: jobs active, phases delayed, crews assigned, punch items open
  Content: items scheduled, drafts due, approvals pending, publishing cadence

Visual identity:
  Technical teams → dense dark or clean graphite with sharp status color coding
  Agency/client work → polished light or dark executive UI with client-safe language
  Construction/field ops → rugged professional cards, high-contrast mobile actions
  Content/calendar → editorial calendar feel with color-coded statuses
Never ship a generic CRUD admin UI when the domain has clear workflow language.
`,
	AcceptanceChecks: []string{
		"project-compile: app builds and passes TypeScript checks with zero errors",
		"project-auth-gate: all dashboard/project/task/team/settings routes require authentication",
		"project-workspace-scope: every API query filters by workspace_id; no cross-workspace data leakage",
		"project-membership-scope: project detail, task, comment, and attachment APIs validate project membership or workspace-level permission",
		"project-role-permissions: client_viewer and viewer cannot mutate tasks, comments, files, settings, or statuses",
		"project-task-create: task create validates title, project_id, status_id, priority, assignee, and due date server-side",
		"project-task-move: board move validates destination status belongs to the project/workspace and persists both status_id and position",
		"project-task-order: reordering cards persists deterministic positions and survives reload",
		"project-dependencies: dependency creation rejects self-dependencies and circular dependency chains",
		"project-comments: task comments render in chronological order and mentions create notifications",
		"project-attachments: attachment download route validates task/project access before returning file URL",
		"project-client-visibility: client-visible mode hides internal-only tasks, comments, attachments, and fields",
		"project-dashboard: active projects, due soon, overdue, blocked, completed, workload, and milestone metrics load from real data",
		"project-mobile: board/list/task detail render as usable card-first mobile views without crushed tables",
		"project-audit-log: create/update/archive/assign/status-move/reorder/dependency/comment/attachment/milestone/sprint/time-entry actions write audit_log rows",
	},
}

var templateCommunity = &AppTemplate{
	ID:       "community",
	Name:     "Social / Community / Content / Messaging Platform",
	Category: "Social / Community / Content / Messaging",
	Priority: 2,
	Keywords: []string{
		"community", "community platform", "social network", "forum", "discussion",
		"discussion forum", "posts", "post feed", "feed", "followers", "following",
		"likes", "comments", "reactions", "bookmarks", "members", "groups",
		"spaces", "creator", "creator community", "niche network", "content sharing",
		"direct messages", "messaging", "moderation", "moderation queue",
		"content reports", "user profiles", "public profiles", "private profiles",
		"hashtags", "mentions", "internal social", "company social", "school community",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Social / Community / Content / Messaging Platform Blueprint
============================================================================

STEP 0 — CLASSIFY SOCIAL/COMMUNITY MODE BEFORE SCHEMA DESIGN
Pick exactly one mode:
  social_network              — profiles, follows, feed, reactions, comments
  community_platform          — communities/spaces, memberships, posts, roles
  discussion_forum            — categories, threads, replies, moderation
  creator_community           — creator profile, posts, members, announcements
  content_sharing_platform    — media/content posts, bookmarks, tags, discovery
  private_member_community    — gated signup, private feeds, member roles
  team_social_hub             — internal company/school updates and discussions
  messaging_first_community   — conversations and DMs are primary
  local_community_app         — neighborhoods/events/groups/local posts

Mode drives feed type, profile visibility, group taxonomy, moderation depth,
privacy controls, messaging complexity, notification volume, and mobile layout.

SUBSYSTEM 1 — AUTH + PROFILES
users(id, email, password_hash/session identity)
profiles(user_id, username unique, display_name, bio, avatar_url, location, website, visibility: public|private|members_only, status)
Onboarding collects username, display name, avatar placeholder, interests/topics, and notification preferences.
Username uniqueness and reserved usernames are validated server-side.

SUBSYSTEM 2 — COMMUNITIES / GROUPS / SPACES
communities(workspace_id nullable, slug unique, name, description, visibility: public|private|invite_only, rules, owner_id, status)
community_members(community_id, user_id, role: owner|admin|moderator|member|limited_member|viewer, status)
Community roles control post/comment/moderation/settings permissions server-side.

SUBSYSTEM 3 — POSTS
posts(author_id, community_id nullable, title nullable, body, content_type, visibility: public|followers|members|private|community, status: draft|published|hidden|archived, pinned_at, archived_at)
Posts support rich text rendering, tags/hashtags, mentions, optional media attachments, bookmarks, reactions, and reports.
Post visibility is enforced server-side on feed, detail, search, profile, and community routes.

SUBSYSTEM 4 — FEEDS
Feed builders:
  home feed: eligible public/member posts ranked by recency and engagement
  following feed: posts by followed users and joined communities
  community feed: posts scoped to community membership/visibility
  profile feed: posts visible to current viewer after privacy/block checks
Feeds use cursor pagination and never leak hidden/private/member-only content.

SUBSYSTEM 5 — COMMENTS + REPLIES
comments(post_id, parent_comment_id nullable, author_id, body, status: published|hidden|archived, visibility inherited)
Nested replies optional; simple apps can use one-level replies.
Comment create validates user can view/post on the parent content and creates notifications for author/mentions.

SUBSYSTEM 6 — REACTIONS, BOOKMARKS, FOLLOWS
post_reactions(post_id, user_id, reaction_type)
comment_reactions(comment_id, user_id, reaction_type)
bookmarks(user_id, post_id)
follows(follower_id, followed_user_id)
Reaction/bookmark/follow toggles are idempotent and update counts consistently.
Blocking/muting rules override feed visibility and notifications.

SUBSYSTEM 7 — MESSAGING (OPTIONAL UNLESS REQUESTED)
conversations(id, type: direct|group, created_by)
conversation_members(conversation_id, user_id, role, muted_at, last_read_at)
messages(conversation_id, sender_id, body, status, created_at)
Message access checks require membership in the conversation. Use polling by default; WebSocket/realtime only if explicitly requested.

SUBSYSTEM 8 — MODERATION + REPORTING
content_reports(reporter_id, entity_type: post|comment|user|community, entity_id, reason, details, status: open|reviewing|resolved|dismissed)
moderation_actions(moderator_id, action, entity_type, entity_id, reason, expires_at nullable)
Moderation queue shows reported content, reporter, reason, status, and action history.
Actions include hide post/comment, dismiss report, warn user, suspend user, ban from community, restore content.

SUBSYSTEM 9 — PRIVACY + SAFETY
blocks(blocker_id, blocked_user_id)
mutes(user_id, muted_user_id)
privacy_settings(user_id, key, value)
Blocked users cannot message, follow, or see restricted profile/feed content. Muted users are removed from feed/notifications.
Public/private/member-only profile visibility is enforced server-side.

SUBSYSTEM 10 — NOTIFICATIONS + AUDIT
notifications(user_id, type, title, body, entity_type, entity_id, read_at)
Notification types: mention, comment, reply, reaction, follow, message, community_invite, report_update, moderation_action.
audit_logs(actor_id, action, entity_type, entity_id, before_json, after_json, ip_address)
Every post/comment/community/settings/moderation/privacy/messaging mutation writes audit_log.

SUBSYSTEM 11 — ADMIN / COMMUNITY MANAGER DASHBOARDS
Admin dashboard: total users, active users, posts, comments, reports open, reports resolved, top communities, recent moderation actions.
Community manager dashboard: members, pending requests, reported content, pinned posts, activity trend, member growth.

SUBSYSTEM 12 — PAGES
Public/auth: login, signup, onboarding/profile/interests.
Social: feed, following, trending placeholder, explore, people, communities, tags, profile, user profile, community pages, post detail, bookmarks, messages if enabled, notifications, moderation, settings.
Admin: users, communities, posts, reports, moderation actions, settings, audit log.

NON-NEGOTIABLES:
- Content visibility is enforced server-side, not just hidden in the UI.
- Feed queries must filter private/member-only/hidden content for the current viewer.
- Blocking and muting change both visibility and notification behavior.
- Moderation reports and actions are auditable.
- Public profile routes never expose private settings, email, auth data, or hidden posts.
- Messaging routes require conversation membership.
- Every list/feed has loading, empty, error, and pagination/loader states.
- Mobile feed and post detail are first-class; no dense desktop-only table layouts.
- Build must compile without TypeScript errors.
`,
	CustomizationRules: `
SOCIAL/COMMUNITY DOMAIN CUSTOMIZATION — match language, content types, and moderation to the community.

Mode-driven vocabulary:
  social_network            → Profiles, Feed, Following, Explore, Posts
  community_platform        → Spaces, Members, Discussions, Announcements
  discussion_forum          → Categories, Threads, Replies, Solved/Locked
  creator_community         → Creator, Members, Updates, Perks, Announcements
  content_sharing_platform  → Posts, Collections, Tags, Bookmarks
  private_member_community  → Members, Member Feed, Invite Requests, Resources
  team_social_hub           → Teams, Updates, Channels, Recognition
  messaging_first_community → Inbox, Conversations, Threads, Members
  local_community_app       → Neighborhoods, Events, Recommendations, Groups

Reaction labels:
  Professional → Like / Insightful / Helpful
  Creator/fan → Like / Love / Fire / Saved
  Forum/support → Helpful / Same issue / Solved
  Local community → Recommend / Going / Interested

Moderation defaults:
  Public social apps need reports, block/mute, hidden content, admin queue.
  Private/member apps need invite approval, member roles, and community rules.
  Messaging-first apps need conversation membership checks and block enforcement.

Visual identity:
  Professional network → clean slate/blue with trust-focused layout
  Creator community → bold creator-led brand with rich cards
  Forum/support → readable dense discussion UI with status badges
  Local community → warm civic/community visual language
  Team social hub → calm internal-product UI with clear notifications
Never generate a generic CRUD dashboard for social/community prompts.
`,
	AcceptanceChecks: []string{
		"community-compile: app builds and passes TypeScript checks with zero errors",
		"community-auth-profile: signup/login and onboarding create a profile with unique username validation",
		"community-profile-privacy: public/private/member-only profile visibility is enforced server-side",
		"community-feed-visibility: feed APIs filter hidden, private, member-only, blocked, and muted content correctly",
		"community-post-create: post creation validates author permissions, community membership, visibility, body/media fields, and status",
		"community-comments: comments/replies validate viewer access to parent post and create notifications for author/mentions",
		"community-reactions: post/comment reaction toggles are idempotent and keep counts consistent",
		"community-bookmarks: bookmark toggle persists per-user saves and bookmark list only shows visible posts",
		"community-follows: follow/unfollow is idempotent and blocked users cannot follow or message each other",
		"community-groups: community join/leave/member-role routes enforce community visibility and permissions",
		"community-messaging: message send/read routes require conversation membership when messaging is enabled",
		"community-moderation: reports enter moderation queue and moderation actions can hide/restore content with audit log rows",
		"community-admin: admin/moderator dashboards show users, posts, reports, and moderation activity from real data",
		"community-mobile: mobile feed, post detail, composer, notifications, and messages are usable without desktop tables",
		"community-audit-log: post/comment/community/settings/moderation/privacy/messaging mutations write audit_log rows",
	},
}

var templateLandingPage = &AppTemplate{
	ID:       "landing-page",
	Name:     "Landing Page / Marketing Site / Funnel / Waitlist",
	Category: "Landing Page / Marketing / Funnel",
	Priority: 2,
	Keywords: []string{
		"landing page", "marketing site", "marketing website", "waitlist",
		"coming soon", "product page", "product launch page", "launch page",
		"startup page", "website", "sales funnel", "lead capture",
		"email capture", "newsletter", "newsletter signup", "sign up page",
		"hero section", "pricing page", "testimonials", "demo request",
		"contact form", "waitlist form", "lead magnet", "conversion page",
		"agency website", "local business site", "creator page", "event page",
		"course funnel", "pre-launch", "beta signup", "early access",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Landing Page / Marketing Site / Funnel / Waitlist Blueprint
============================================================================

STEP 0 — CLASSIFY LANDING PAGE MODE BEFORE SECTION DESIGN
Pick exactly one mode:
  saas_landing_page          — explains/sells software, pricing and demo CTA likely
  startup_waitlist           — early access signup and product preview
  product_launch_page        — release announcement, product visuals, availability
  agency_website             — services, case studies, demo/contact CTA
  local_business_site        — local trust, services, contact/location CTA
  creator_page               — personal/creator offer, newsletter/community CTA
  course_or_coaching_funnel  — offer, outcomes, curriculum, testimonials, application CTA
  newsletter_signup          — editorial promise, sample issues, signup CTA
  event_registration_page    — event details, schedule, speakers, registration CTA
  lead_magnet_funnel         — downloadable asset, qualifying fields, thank-you flow
  investor_demo_page         — concise product thesis, proof, CTA, claim safeguards

Mode drives section order, form fields, CTA routing, proof requirements, SEO copy,
analytics events, and whether backend lead storage/admin is required.

SUBSYSTEM 1 — PUBLIC MARKETING SHELL
PublicShell includes marketing header, mobile menu, footer, primary CTA, secondary CTA, legal links, and responsive layout.
Header nav uses sections/pages that actually exist. CTA buttons route to working anchors/pages/forms.
Sticky mobile CTA appears after hero on small screens.

SUBSYSTEM 2 — LANDING SECTIONS
Required section set unless the prompt explicitly narrows scope:
hero, problem, solution, features, benefits, how it works, use cases, proof/testimonials, pricing when relevant, FAQ, final CTA.
Every section must use prompt-specific copy. Avoid generic filler like "transform your workflow" unless grounded in the prompt.

SUBSYSTEM 3 — FORMS + LEAD CAPTURE
Lead capture routes: waitlist, newsletter, contact, demo request, lead magnet, event registration as needed.
Form submissions validate server-side: email format, required fields, max lengths, spam honeypot, optional rate-limit placeholder.
Leads store: name, email, company, role, phone optional, source form, UTM params, message/notes, consent, created_at.
Thank-you pages confirm submission and provide next action.

SUBSYSTEM 4 — UTM + CONVERSION EVENTS
Capture utm_source, utm_medium, utm_campaign, utm_content, utm_term when present.
Track placeholder conversion events: hero_cta_clicked, form_started, form_submitted, pricing_cta_clicked, demo_requested, waitlist_joined.
Analytics provider is a placeholder unless the user provides a provider/key.

SUBSYSTEM 5 — SEO + SOCIAL PREVIEW
SEO metadata: title, description, canonical, OpenGraph title/description/image placeholder, Twitter card, JSON-LD organization/product/service/event schema as appropriate.
Sitemap and robots placeholders should be included for app/router stacks that support them.
Each public page has useful metadata derived from the prompt.

SUBSYSTEM 6 — CLAIM VERIFICATION SAFEGUARDS
Claims must be categorized as:
  verified — supported by prompt-provided facts or generated app behavior
  user_provided — explicitly provided by the user but not independently verified
  placeholder — illustrative proof/case study/stat that must not be presented as fact
Do not invent competitor claims, revenue numbers, customer logos, compliance badges, security certifications, or performance guarantees.
If proof is missing, use honest placeholders like "Add customer proof here" or seed/demo testimonials clearly framed as examples.

SUBSYSTEM 7 — PRICING / PLANS (WHEN RELEVANT)
Pricing cards are optional and only included when prompt implies paid product/service.
Pricing CTA routes to demo/contact/waitlist unless real checkout is requested.
Do not include fake Stripe checkout unless user requests payments and backend is in scope.

SUBSYSTEM 8 — ADMIN / CONTENT / LEAD VIEW (OPTIONAL)
For full-stack landing apps, include protected admin lead table, content section editor placeholder, claim review panel, analytics summary, settings, audit log.
For frontend-only/static landing pages, keep forms as local/mock submissions with clear success states and no fake external integrations.

SUBSYSTEM 9 — LEGAL / TRUST
Include privacy, terms, and cookies placeholder pages when app has forms/tracking.
Trust/security sections must be accurate and not overstate compliance.

SUBSYSTEM 10 — PAGES
Public pages: homepage, about optional, pricing optional, features/use-cases optional, blog/case-studies optional, contact, demo/waitlist/newsletter/download thank-you pages, legal pages.
Admin pages only when backend/admin scope is present: leads, content, settings, audit log.

NON-NEGOTIABLES:
- One clear primary conversion goal.
- Hero explains who it is for, what it does, and why it matters.
- CTA buttons are obvious and route to working destinations.
- Forms validate server-side or clearly use a local mock when runtime-free/static.
- UTM parameters are captured when present.
- SEO title, description, OpenGraph, and canonical metadata are set.
- Claims do not overstate capability beyond prompt-supported facts.
- Any performance, pricing, security, compliance, or competitor comparison claim is verified, user-provided, or placeholder.
- Mobile hero, CTA, forms, pricing, and FAQ are first-class.
- Build must compile without TypeScript errors.
`,
	CustomizationRules: `
LANDING PAGE CUSTOMIZATION — optimize message, sections, and CTA for the prompt's offer.

Mode-driven sections:
  SaaS landing page: Hero, Logo Cloud, Problem, Solution, Features, How It Works, Use Cases, Proof, Pricing, FAQ, Final CTA
  Startup waitlist: Hero, Waitlist Form, Problem, Promise, Feature Preview, Audience, Placeholder Proof, FAQ, Final CTA
  Product launch: Hero, Launch Announcement, Screenshots, Benefits, Features, Availability/Pricing, FAQ, CTA
  Agency website: Hero, Services, Process, Case Studies/Proof Placeholders, Testimonials, Pricing/Packages optional, Contact
  Local business: Hero, Services, Service Area, Trust Badges, Testimonials, Contact/Map Placeholder, FAQ
  Creator/newsletter: Hero, Sample Content, Audience Promise, Signup, Archive Preview, About Creator, FAQ
  Event: Hero, Schedule, Speakers, Venue/Online Details, Registration, FAQ
  Lead magnet: Hero, Asset Preview, Why Download, Form, Thank You Flow, Follow-up CTA

Copy rules:
  Headline must be specific to the audience and offer.
  Subhead must explain outcome and mechanism.
  Feature cards must include concrete nouns from the prompt.
  Testimonials/case studies/logos are placeholders unless user supplied real proof.
  Avoid unverifiable superlatives like "best", "#1", "guaranteed", or competitor superiority unless user provided evidence.

CTA routing:
  Waitlist/building soon → primary CTA goes to waitlist form.
  SaaS/demo → primary CTA goes to demo request or trial placeholder.
  Agency/local → primary CTA goes to contact form.
  Newsletter → primary CTA goes to signup form.
  Event → primary CTA goes to registration form.

Visual identity:
  Developer/SaaS → crisp technical UI, product screenshots, code/product motifs
  Premium B2B → executive dark/graphite or clean slate with strong typography
  Local service → warm trust-building UI with location/contact prominence
  Creator/course → personality-forward, editorial, human tone
  Event → high-energy timeline/schedule visual system
Never produce the same generic blue SaaS page for every marketing prompt.
`,
	AcceptanceChecks: []string{
		"landing-compile: app builds and passes TypeScript checks with zero errors",
		"landing-conversion-goal: homepage has one clear primary CTA and secondary CTA does not compete visually",
		"landing-hero-specificity: hero states target audience, product/service, and outcome using prompt-specific language",
		"landing-cta-routing: all header, hero, pricing, final, and sticky mobile CTA buttons route to existing anchors/pages/forms",
		"landing-form-validation: lead/waitlist/contact/demo/newsletter forms validate required fields, email format, and max lengths",
		"landing-form-success: successful form submission shows a clear thank-you state or routes to a thank-you page",
		"landing-utm-capture: forms capture UTM parameters when present",
		"landing-seo: title, description, canonical, OpenGraph, and JSON-LD metadata are present and prompt-specific",
		"landing-claim-safety: pricing/security/performance/compliance/competitor claims are verified, user-provided, or marked placeholder",
		"landing-proof-honesty: customer logos, testimonials, metrics, and case studies are not presented as real unless user supplied them",
		"landing-mobile: hero, CTA, forms, pricing, FAQ, and final CTA are usable and visually polished on mobile",
		"landing-performance: no heavy unused dashboard/app shell is generated for a marketing-only prompt",
		"landing-legal: privacy/terms/cookies placeholders exist when forms or tracking placeholders are included",
		"landing-analytics: conversion event placeholders exist for CTA clicks and form submissions without requiring real external keys",
	},
}
