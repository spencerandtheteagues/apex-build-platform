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
	Keywords: []string{
		"ai", "llm", "openai", "anthropic", "claude", "gpt", "gemini", "grok",
		"ollama", "deepseek", "prompt", "chatbot", "chat bot", "ai tool",
		"ai app", "ai saas", "ai wrapper", "language model", "generative",
		"summarize", "summarizer", "summarization", "resume", "proposal",
		"content generator", "content generation", "copy generator",
		"document analyzer", "document analysis", "pdf analyzer", "pdf analysis",
		"code review", "code reviewer", "ai assistant", "ai agent",
		"token", "byok", "bring your own key", "api key", "model selector",
		"provider", "usage tracking", "cost tracking", "ai credits",
		"image generation", "text generation", "text to", "to text",
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
	Name:     "SaaS Dashboard",
	Category: "SaaS / Admin",
	Priority: 5,
	Keywords: []string{
		"saas", "dashboard", "admin panel", "admin dashboard", "startup app",
		"user management", "analytics dashboard", "multi-tenant", "workspace",
		"organization", "subscription", "stripe billing", "metrics dashboard",
		"kpi", "reporting", "onboarding flow", "settings page",
	},
	ArchitectureContext: `
ACTIVE TEMPLATE: SaaS Dashboard
=================================
This build uses the SaaS Dashboard blueprint: multi-tenant auth, org/workspace
model, dashboard shell, CRUD resources, charts, Stripe billing, and admin role.

Required: auth (signup/login/invite), workspace/org model, dashboard with charts,
at least one CRUD resource, Stripe billing placeholders, role-based access (admin/member),
settings page, responsive layout.
`,
	CustomizationRules: `
Customize domain, color scheme, CRUD entities, chart types, and KPI cards
specifically for the user's described business. Never produce a generic "App" dashboard.
`,
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
