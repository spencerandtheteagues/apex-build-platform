package agents

import (
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
	templateLandingPage,
	templateCommunity,
}

// DetectAppTemplate returns the best-matching template for a build description,
// or nil if no template matches with sufficient confidence.
func DetectAppTemplate(description string) *AppTemplate {
	normalized := strings.ToLower(strings.TrimSpace(description))
	if normalized == "" {
		return nil
	}

	type scored struct {
		tmpl  *AppTemplate
		score int
	}

	var candidates []scored
	for _, tmpl := range appTemplateRegistry {
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

	// Pick highest score; break ties with Priority (lower wins).
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.score > best.score || (c.score == best.score && c.tmpl.Priority < best.tmpl.Priority) {
			best = c
		}
	}

	// Require at least 2 keyword matches OR exactly 1 match on a high-signal keyword
	// to avoid false-positives on vague prompts.
	if best.score < 2 && !isHighSignalMatch(normalized, best.tmpl) {
		return nil
	}

	return best.tmpl
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
		"crm": {"crm", "sales pipeline", "customer relationship"},
		"marketplace": {"marketplace", "two-sided", "buyer seller"},
		"booking": {"booking app", "scheduling app", "reservation system"},
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

// ----- Templates #2–#10 (detection-ready; full architecture content to be added) -----

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
	Category: "CRM",
	Priority: 4,
	Keywords: []string{
		"crm", "sales pipeline", "customer relationship", "contacts", "leads",
		"deals", "pipeline", "sales funnel", "lead management", "sales tracker",
		"prospect", "opportunity", "account management", "follow up",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: CRM / Sales Pipeline
======================================
Required: contacts table, companies table, deals/opportunities with pipeline stages,
notes/activity log, reminders, lead capture form, import/export (CSV), email action
placeholders, role-based access.
`,
	CustomizationRules: `
Customize pipeline stage names, entity fields, color scheme, and industry focus
(real estate / recruiting / agency / SaaS / etc.) based on the user's prompt.
`,
}

var templateClientPortal = &AppTemplate{
	ID:       "client-portal",
	Name:     "Client Portal",
	Category: "Portal",
	Priority: 4,
	Keywords: []string{
		"client portal", "customer portal", "client dashboard", "client access",
		"deliverables", "file sharing", "project status", "invoice portal",
		"agency portal", "contractor portal", "consultant portal",
		"approval portal", "client login", "client area",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Client Portal
================================
Required: two roles (admin/team + client), client-specific views, file
uploads/downloads, project status visibility, invoice list, messaging or
comments, approval flows, role-based route guards.
`,
	CustomizationRules: `
Customize entity names, file types, status labels, and visual identity for the
specific service business (agency / law firm / accountant / contractor / etc.).
`,
}

var templateMarketplace = &AppTemplate{
	ID:       "marketplace",
	Name:     "Marketplace / Directory",
	Category: "Marketplace",
	Priority: 3,
	Keywords: []string{
		"marketplace", "directory", "listings", "listing app", "two-sided",
		"buyer", "seller", "vendor", "supplier", "search and filter",
		"reviews", "ratings", "booking flow", "hire", "rent",
		"airbnb for", "upwork for", "fiverr for", "etsy for",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Marketplace / Directory
=========================================
Required: listings (search/filter/sort), seller/provider profiles, buyer flow,
reviews/ratings, favorites, contact or booking CTA, admin moderation queue.
Complexity warning: two-sided payments need Stripe Connect — if not requested,
use placeholder or contact-based flow instead.
`,
	CustomizationRules: `
Classify complexity from the prompt: simple directory vs full marketplace with payments.
Customize listing schema, search filters, and visual identity for the specific vertical.
`,
}

var templateBooking = &AppTemplate{
	ID:       "booking",
	Name:     "Booking / Scheduling",
	Category: "Booking",
	Priority: 3,
	Keywords: []string{
		"booking", "scheduling", "reservation", "appointment", "calendar",
		"availability", "time slot", "schedule", "book a", "book an",
		"salon", "spa", "consultant", "tutor", "venue", "class",
		"cancellation", "reminder", "confirmation",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Booking / Scheduling
======================================
Required: service catalog, availability/calendar rules, booking form,
confirmation flow, cancellation flow, email confirmation placeholders,
admin view of all bookings, reminder system placeholder.
`,
	CustomizationRules: `
Customize service types, duration increments, provider names, and visual
identity for the specific booking context (salon / clinic / tutor / venue / etc.).
`,
}

var templateInventory = &AppTemplate{
	ID:       "inventory",
	Name:     "Inventory / Order Management",
	Category: "Operations",
	Priority: 3,
	Keywords: []string{
		"inventory", "stock", "warehouse", "order management", "orders",
		"sku", "product catalog", "vendor", "supplier", "purchase order",
		"reorder", "asset tracking", "asset management", "equipment",
		"work order", "fulfillment", "shipping",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Inventory / Order Management
==============================================
Required: items/SKUs with stock levels, vendor/supplier table, order workflow
(pending/processing/fulfilled), reorder alerts, barcode/QR placeholder,
reports (low stock, order history), role-based access (admin/warehouse/viewer).
`,
	CustomizationRules: `
Customize entity names, status labels, and workflow steps for the specific
operations context (retail / manufacturing / rental / asset tracking).
`,
}

var templateProjectManagement = &AppTemplate{
	ID:       "project-management",
	Name:     "Project Management",
	Category: "Productivity",
	Priority: 4,
	Keywords: []string{
		"project management", "task management", "task tracker", "kanban",
		"trello", "asana", "jira", "sprint", "backlog", "milestone",
		"project tracker", "todo list", "task board", "work management",
		"team tasks", "assign", "assignee", "due date", "priority",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Project Management
=====================================
Required: projects, tasks with status/assignee/due date, at least one view
(Kanban board or list), comments on tasks, notifications placeholder, member
management, role-based permissions.
`,
	CustomizationRules: `
Customize status names, project types, and visual identity for the specific
context (software dev / construction / agency / personal / school).
`,
}

var templateLandingPage = &AppTemplate{
	ID:       "landing-page",
	Name:     "Landing Page / Waitlist",
	Category: "Marketing",
	Priority: 2,
	Keywords: []string{
		"landing page", "marketing site", "waitlist", "coming soon",
		"product page", "launch page", "startup page", "website",
		"lead capture", "email capture", "newsletter", "sign up page",
		"hero section", "pricing page", "testimonials",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Landing Page / Waitlist
=========================================
Required: hero, problem/solution section, feature highlights, social proof
placeholder, pricing section, FAQ, waitlist/contact form with email capture,
analytics placeholder (gtag), mobile-responsive, SEO meta tags.
`,
	CustomizationRules: `
Derive headline, subhead, feature copy, and visual identity entirely from
the user's product description. Every landing page must feel product-specific.
`,
}

var templateCommunity = &AppTemplate{
	ID:       "community",
	Name:     "Community / Social",
	Category: "Social",
	Priority: 2,
	Keywords: []string{
		"community", "social network", "forum", "discussion", "posts",
		"feed", "followers", "following", "likes", "comments",
		"niche network", "creator", "members", "groups",
		"internal social", "company social", "school community",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: Community / Social
=====================================
Required: user profiles, post creation/feed, comments, likes/reactions,
follow system, notifications placeholder, groups or categories,
moderation tools (report + admin ban), mobile-responsive feed.
Complexity warning: real-time features need WebSockets — use polling as
a simpler default unless real-time is explicitly requested.
`,
	CustomizationRules: `
Customize feed content type, reaction labels, group taxonomy, and visual
identity for the specific community context (niche interest / professional / school / internal).
`,
}
