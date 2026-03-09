// APEX.BUILD — Landing Page v2
// Above-fold: logo + 2-col feature bullets, no scroll required.
// Scroll: each bullet expands to a full rich detail section.

import React, { useState, useEffect, useRef, useCallback } from 'react'
import { motion, AnimatePresence, useScroll, useTransform } from 'framer-motion'
import {
  Bot, DollarSign, Terminal, GitBranch, Shield, Users,
  Puzzle, Layers, Key, Zap, ArrowRight, Check, ChevronDown,
  Globe, BarChart3, Cpu, Lock, Eye, Code2, Database,
  Package, Activity, TrendingDown, Sparkles, X,
} from 'lucide-react'

// ─── Design tokens ────────────────────────────────────────────────────────────

const C = {
  bg:          '#000000',
  bg2:         '#050505',
  surface:     'rgba(255,255,255,0.025)',
  surfaceHov:  'rgba(255,255,255,0.05)',
  border:      'rgba(255,0,51,0.16)',
  borderDim:   'rgba(255,255,255,0.06)',
  borderBright:'rgba(255,255,255,0.12)',
  accent:      '#ff0033',
  accentSoft:  '#ff3355',
  accentDim:   'rgba(255,0,51,0.09)',
  accentGlow:  'rgba(255,0,51,0.22)',
  green:       '#34d399',
  greenDim:    'rgba(52,211,153,0.10)',
  greenBorder: 'rgba(52,211,153,0.22)',
  text:        '#f0f0f0',
  textSub:     '#9ca3af',
  textMuted:   'rgba(255,255,255,0.28)',
  white:       '#ffffff',
}

const fHero = '"Orbitron","Rajdhani",sans-serif'
const fBody = '"Inter","Segoe UI",system-ui,sans-serif'
const fMono = '"Fira Code","JetBrains Mono","Consolas",monospace'

// ─── Feature data — each item is both a bullet above-fold and a detail section ─

const FEATURES = [
  // ── LEFT COLUMN ──
  {
    id: 'agents',
    icon: Bot,
    color: '#a78bfa',
    bullet: '5 Specialized AI Agents build your app in parallel',
    heading: 'Five AI Specialists. One Complete App.',
    sub: 'APEX.BUILD isn\'t a chatbot that writes code snippets. It\'s a team of five dedicated AI agents that plan, build, review, and fix your entire full-stack application — working simultaneously, each owning their layer.',
    points: [
      { icon: '⬡', label: 'Architect', desc: 'Plans your database schema, API contracts, and folder structure before a single line of code is written. Catches design mistakes before they become tech debt.' },
      { icon: '⚙', label: 'Backend', desc: 'Generates production-grade REST APIs, JWT auth, and database layers in Go, Python, or Node.js. Real error handling, migrations, and middleware included.' },
      { icon: '◈', label: 'Frontend', desc: 'Builds React, Next.js, or Vue UIs with Tailwind — auto-wired to the backend. No manual API integration. No mismatched types.' },
      { icon: '◉', label: 'Reviewer', desc: 'Validates code quality, enforces CORS config, checks integration points, and flags security issues before you see the output.' },
      { icon: '✦', label: 'Solver', desc: 'Automatically detects and repairs build errors, type failures, and dependency conflicts — without being told. Runs in a loop until the build is clean.' },
    ],
    useCase: '"Build me a multi-tenant SaaS with Stripe billing and user auth." All five agents spin up — Architect designs the schema, Backend writes the API, Frontend builds the UI, Reviewer validates it, Solver fixes any issues. Deployable full-stack app in under 5 minutes.',
    visual: 'agents',
  },
  {
    id: 'cost',
    icon: DollarSign,
    color: '#34d399',
    bullet: 'Real-time cost counter — every token, every dollar, live',
    heading: 'No Surprises. Ever.',
    sub: 'Every other AI platform hides the bill until you\'re shocked at month-end. APEX.BUILD shows you a live cost ticker — per agent, per model, per token — as the build runs. You always know what you\'re spending before it\'s spent.',
    points: [
      { icon: '📊', label: 'Per-agent breakdown', desc: 'Cost attributed to each specialist agent individually. See exactly what the Architect spent vs. the Backend agent vs. the Reviewer.' },
      { icon: '💡', label: 'Per-model accuracy', desc: 'Each API call is billed at the actual provider rate — GPT-5 at $2.50/1M, Claude Haiku at $0.25/1M. No blended rates. No averaging.' },
      { icon: '🎯', label: 'Budget controls', desc: 'Set a spend limit per session or per project. Builds stop cleanly at the limit — no surprise overages.' },
      { icon: '📒', label: 'Immutable credit ledger', desc: 'Every credit deduction is logged with timestamp, agent, model, and token count. Full audit trail, always.' },
      { icon: '📈', label: 'Usage analytics', desc: 'See your spending patterns over time — by day, by project, by AI provider. Optimize where your credits go.' },
    ],
    useCase: 'A typical full-stack app build costs $0.04–$0.12 in managed credits. You watch the number tick up in real time, agent by agent, model by model. No monthly surprise invoices. Ever.',
    visual: 'cost',
  },
  {
    id: 'ide',
    icon: Terminal,
    color: '#60a5fa',
    bullet: 'Full cloud IDE — Monaco editor, live preview, sandboxed execution',
    heading: 'A Real IDE in Your Browser.',
    sub: 'Not a code generator. Not a chat window. APEX.BUILD is a full cloud development environment — the same Monaco editor that powers VS Code, a complete file tree, live preview pane, and sandboxed code execution. Zero install. Zero config.',
    points: [
      { icon: '🖥', label: 'Monaco editor', desc: 'Full VS Code-class editor with syntax highlighting, IntelliSense, multi-file tabs, find & replace, and keyboard shortcuts.' },
      { icon: '🌐', label: '20+ languages', desc: 'TypeScript, JavaScript, Python, Go, Rust, Java, C++, Ruby, PHP, and more. Agents write in whatever your stack requires.' },
      { icon: '👁', label: 'Live preview', desc: 'Watch your frontend render as agents write it. Hot-reload, no build step, instant visual feedback during generation.' },
      { icon: '📦', label: 'Sandboxed execution', desc: 'Run code directly in an isolated browser sandbox with configurable memory (512MB) and CPU limits. No leakage, no side effects.' },
      { icon: '🗂', label: 'Asset management', desc: 'Upload images, fonts, and files. Agents reference them automatically in generated code — no manual path wiring.' },
      { icon: '↩️', label: 'Version history', desc: 'Every agent change is versioned. Roll back any file to any previous state with one click.' },
    ],
    useCase: 'Open APEX.BUILD on a Chromebook, an iPad, or any browser. Full professional IDE, all AI providers, instant code execution — zero install, zero config. Your dev environment lives in a tab.',
    visual: 'ide',
  },
  {
    id: 'ai',
    icon: Cpu,
    color: '#D97757',
    bullet: 'Claude · GPT-5 · Gemini · Grok · Ollama — choose your AI',
    heading: 'Every Major AI Model. One Platform.',
    sub: 'Different tasks call for different models. APEX.BUILD routes each agent\'s work to the right provider automatically based on your power mode — or you override manually. Switch mid-build. Mix providers. The platform handles the rest.',
    points: [
      { icon: '🟠', label: 'Claude (Anthropic)', desc: 'Best-in-class for code review, documentation, and complex multi-step reasoning. Haiku (fast), Sonnet (balanced), and Opus (max) available.' },
      { icon: '🟢', label: 'GPT-5 (OpenAI)', desc: 'Strong code generation with fast iteration loops. Full GPT-5 for flagship performance, GPT-4o-mini for cost-efficient tasks.' },
      { icon: '🔵', label: 'Gemini (Google)', desc: 'Best for long-context tasks and multi-modal inputs. Flash-Lite (0.075¢/1M), Flash (0.50¢/1M), and Pro (2¢/1M).' },
      { icon: '⬜', label: 'Grok (xAI)', desc: 'Fast, sharp reasoning. Great for logic-heavy and analysis tasks. Grok-4 (flagship), Grok-4-Fast (speed), Grok-3-Mini (budget).' },
      { icon: '🟣', label: 'Ollama (Local / Free)', desc: 'Run Llama 3, Mistral, or any open-source model locally. Zero API cost. Complete privacy — your data never leaves your machine.' },
    ],
    useCase: 'Power mode: Fast uses Haiku + GPT-4o-mini for boilerplate. Balanced uses Sonnet + GPT-5 for core logic. Max routes everything through Opus + GPT-5 + Grok-4 with full validation loops. Pick the tradeoff that fits your budget.',
    visual: 'ai',
  },
  {
    id: 'git',
    icon: GitBranch,
    color: '#fbbf24',
    bullet: 'Git built-in — branches, commits, pull requests, from the IDE',
    heading: 'Git Without Leaving the Browser.',
    sub: 'APEX.BUILD has full Git integration baked in. Create branches, commit AI-generated code, open pull requests, and push to GitHub — all from the same interface where agents build your app. Your main branch stays clean.',
    points: [
      { icon: '🌿', label: 'Branch management', desc: 'Create, switch, and delete branches. Agents automatically commit their work to the correct feature branch, not main.' },
      { icon: '⬆️', label: 'Push to GitHub', desc: 'One-click push with full ownership verification. Agents can only push to repos you own — no accidental cross-project writes.' },
      { icon: '🔀', label: 'Pull requests', desc: 'Open PRs directly from the IDE. The Reviewer agent can audit any PR on demand and leave structured inline comments.' },
      { icon: '📜', label: 'Commit history', desc: 'Full commit log with diffs and file-level history. See exactly what each agent changed and when.' },
      { icon: '↩️', label: 'Rollback', desc: 'Any agent commit can be reverted. If a build goes sideways, roll back to any clean checkpoint instantly.' },
    ],
    useCase: 'Tell the AI: "Add dark mode to the settings page." It creates a `feature/dark-mode` branch, writes the CSS and toggle logic, commits the changes, opens a PR against main, and adds a description summarizing what changed. Your workflow, automated.',
    visual: 'git',
  },
  {
    id: 'byok',
    icon: Key,
    color: '#f87171',
    bullet: 'Bring Your Own API Keys — full platform at $0.25/1M routing fee',
    heading: 'Your Keys. Your Cost. Our Platform.',
    sub: 'Already have API contracts with Anthropic, OpenAI, Google, or xAI? Use them. Bring your own keys and pay only our flat $0.25/1M token routing fee. You get everything APEX.BUILD offers at raw provider cost — no markup.',
    points: [
      { icon: '💰', label: 'Zero markup on API cost', desc: 'BYOK users pay exactly what the AI provider charges. Our fee is $0.25 per 1M tokens — covering infrastructure and orchestration.' },
      { icon: '🔐', label: 'AES-256 encrypted storage', desc: 'Your keys are encrypted at rest with a unique per-user master key. Plaintext never touches the database. Never transmitted in logs.' },
      { icon: '🎛', label: 'Per-provider flexibility', desc: 'Use your Anthropic key for Claude agents, your OpenAI key for GPT-5 agents — mix and match keys per provider independently.' },
      { icon: '✅', label: 'Instant validation', desc: 'Keys are validated on entry. APEX.BUILD alerts you if a key expires, hits rate limits, or runs out of credits — before a build fails mid-run.' },
      { icon: '📊', label: 'BYOK usage analytics', desc: 'See token usage per provider, per model, per project — even when using your own keys. Full visibility.' },
    ],
    useCase: 'Enterprise teams with existing AI contracts typically save 60–70% vs. managed credits. Paste your key, hit validate, start building. Same full-stack AI IDE, a fraction of the cost.',
    visual: 'byok',
  },

  // ── RIGHT COLUMN ──
  {
    id: 'secrets',
    icon: Shield,
    color: '#818cf8',
    bullet: 'Encrypted secrets vault with audit log & rotation',
    heading: 'Your Secrets Are Actually Secret.',
    sub: 'APEX.BUILD has a built-in secrets vault. Store API keys, database URLs, webhook secrets, and credentials — AES-encrypted, scoped per project, never exposed in generated code. Agents reference secrets by name and never see the value.',
    points: [
      { icon: '🔒', label: 'AES-256 encryption at rest', desc: 'Every secret is encrypted with a unique per-user master key. The plaintext value is never stored in the database or transmitted in logs.' },
      { icon: '🏗', label: 'Project-scoped access', desc: 'Secrets belong to specific projects. Agents building Project A cannot read secrets from Project B, ever.' },
      { icon: '📋', label: 'Immutable audit log', desc: 'Every read, write, update, and rotation is logged with timestamp, actor, and action. Full accountability trail.' },
      { icon: '🔄', label: 'One-click rotation', desc: 'Rotate any secret instantly. The old value is invalidated immediately and logged. New value is re-encrypted and re-deployed.' },
      { icon: '🤖', label: 'AI-safe injection', desc: 'Agents inject secrets as environment variables. The actual value is never written into source code, chat history, or editor output.' },
    ],
    useCase: 'Your Stripe secret key, Supabase URL, and SendGrid API key live in the vault. The Frontend agent writes `process.env.STRIPE_SECRET_KEY` in code. The actual value is injected at runtime. It never appears in your codebase.',
    visual: 'secrets',
  },
  {
    id: 'collab',
    icon: Users,
    color: '#38bdf8',
    bullet: 'Real-time collaboration — multiple devs, one live session',
    heading: 'Build Together, Live.',
    sub: 'APEX.BUILD supports real-time multi-user collaboration. Multiple developers can work in the same project simultaneously — see live cursors, watch AI agents build together, divide work across the team, and review output collectively.',
    points: [
      { icon: '👥', label: 'Live presence', desc: 'See exactly who\'s in the project, where their cursor is, and what they\'re looking at — in real time.' },
      { icon: '🤖', label: 'Shared AI sessions', desc: 'The entire team watches agent builds in real time — no screen sharing required. Everyone sees the same live output.' },
      { icon: '🔑', label: 'Role-based access', desc: 'Owner, Editor, and Viewer roles. Control who can trigger builds, commit code, manage secrets, or invite others.' },
      { icon: '💬', label: 'Inline code comments', desc: 'Comment on specific files or lines of agent-generated code. Discuss, suggest, or approve before merging.' },
      { icon: '📡', label: 'WebSocket-powered', desc: 'Sub-100ms latency collaboration via persistent WebSocket connections with batched message delivery for efficiency.' },
    ],
    useCase: 'Your frontend dev watches the Frontend agent build components. Your backend dev reviews the API agent\'s output and leaves comments. Your architect monitors the schema — all in the same session, all at the same time.',
    visual: 'collab',
  },
  {
    id: 'mcp',
    icon: Puzzle,
    color: '#a3e635',
    bullet: 'MCP support — connect any external tool, API, or data source',
    heading: 'Connect Anything. Build With Everything.',
    sub: 'APEX.BUILD supports the Model Context Protocol — the open standard for connecting AI agents to external tools, databases, and APIs. Your agents aren\'t limited to generating code. They can query your systems, read your data, and act on the real world.',
    points: [
      { icon: '🔌', label: 'Connect any MCP server', desc: 'Add any MCP-compatible server by URL. Agents gain immediate access to its tools and resources — no code changes required.' },
      { icon: '🛠', label: 'Live tool calling', desc: 'Agents call external tools mid-build — query your CRM, pull from your data warehouse, read your Notion docs, hit internal APIs.' },
      { icon: '📂', label: 'Resource access', desc: 'Expose files, database records, or API responses as live context that agents read during code generation.' },
      { icon: '🌍', label: 'Ecosystem compatible', desc: 'MCP is supported by Claude Desktop, Cursor, Cline, and all major AI IDEs — your integrations transfer to every tool.' },
      { icon: '🔒', label: 'Auth-protected connections', desc: 'MCP connections are stored per-user with auth tokens. External servers are only accessible to users who configured them.' },
    ],
    useCase: 'Connect your Jira MCP server. Tell the agent: "Build the feature described in JIRA-442." The agent reads the ticket, understands acceptance criteria, builds accordingly, and writes a PR description that references the ticket. No copy-paste required.',
    visual: 'mcp',
  },
  {
    id: 'templates',
    icon: Layers,
    color: '#fb923c',
    bullet: 'Project templates — skip boilerplate, start from working code',
    heading: 'Skip the Setup. Start With Something Real.',
    sub: 'APEX.BUILD ships with battle-tested project templates for the most common app types. Start from a working foundation and let agents extend it — not build from zero every time. Every template is production-structured, not a tutorial skeleton.',
    points: [
      { icon: '💳', label: 'SaaS starter', desc: 'Auth (JWT), billing (Stripe), user dashboard, subscription management, and REST API — all wired together and deployable.' },
      { icon: '⚡', label: 'REST API', desc: 'Go or Node.js API with JWT auth, PostgreSQL connection, middleware stack, and standard CRUD endpoints. Ready to extend.' },
      { icon: '⚛️', label: 'React app', desc: 'Vite + React + TypeScript + Tailwind + React Router — production-ready frontend scaffold with component patterns in place.' },
      { icon: '🏗', label: 'Full-stack', desc: 'Backend + frontend in a monorepo, wired together with shared TypeScript types and a working local dev environment.' },
      { icon: '📝', label: 'Custom templates', desc: 'Save any project as a reusable template. Share across your team or use your own architectural patterns as a starting point.' },
    ],
    useCase: 'Spin up the SaaS starter template — you\'ve got auth, billing, and a dashboard in 30 seconds. Tell the Architect agent "add a referral system with credit rewards." It knows the existing structure and extends it correctly. Hours compressed to minutes.',
    visual: 'templates',
  },
  {
    id: 'billing',
    icon: BarChart3,
    color: '#34d399',
    bullet: 'Flexible billing — credits, subscriptions, or BYOK. No lock-in.',
    heading: 'Pay How You Build.',
    sub: 'Three subscription tiers for teams that build regularly. Credit packs for one-off projects. BYOK for power users who have their own AI contracts. No annual commitments. Credits never expire. Cancel anytime.',
    points: [
      { icon: '🔨', label: 'Builder — $19/mo', desc: 'Solo developers and indie hackers. Managed credits, up to 5 active projects, all AI providers, full IDE.' },
      { icon: '⚡', label: 'Pro — $49/mo', desc: 'Growing indie teams. More projects, higher credit limits, priority agent queues, advanced analytics.' },
      { icon: '👥', label: 'Team — $99/mo', desc: 'Full development teams. Collaboration features, role management, shared project workspaces, priority support.' },
      { icon: '💰', label: 'Credit top-ups', desc: 'Need more credits on any plan? Buy $10, $25, $50, or $100 packs. Credits never expire. Use them whenever.' },
      { icon: '🔑', label: 'BYOK savings', desc: 'Bring your own API keys on any tier. Pay only the $0.25/1M routing fee. Typical savings: 60–70% vs. managed credits.' },
    ],
    useCase: 'Building one app? Buy a $25 credit pack. No subscription. Building weekly? The Pro plan at $49/mo covers most active teams. Enterprise with existing AI contracts? BYOK on Team is cheaper than a single month of most alternatives.',
    visual: 'billing',
  },
  {
    id: 'export',
    icon: Globe,
    color: '#e879f9',
    bullet: 'Export, deploy, or host — own your code, always',
    heading: 'Own What You Build. Forever.',
    sub: 'Every app built in APEX.BUILD is 100% yours. Export full source code at any time, deploy to your own infrastructure, or use built-in hosting. Standard files, no proprietary formats, no vendor lock-in — ever.',
    points: [
      { icon: '📦', label: 'Full source export', desc: 'Download a complete zip of your project — all source files, configs, dependencies, and assets. Open it anywhere.' },
      { icon: '🚀', label: 'One-click deploy', desc: 'Deploy directly to Render, Railway, or Vercel from the IDE. Agents generate the platform-specific deploy config automatically.' },
      { icon: '🌐', label: 'Custom domains', desc: 'Map any domain to a hosted project. HTTPS provisioned automatically. DNS propagation monitored in real time.' },
      { icon: '🐳', label: 'Docker + CI/CD', desc: 'Request a Dockerfile, GitHub Actions workflow, or Render Blueprint — agents generate complete, working configs.' },
      { icon: '⚙️', label: 'Environment configs', desc: 'Agents generate production `.env` templates, `.env.example` files, and deployment docs as part of every build.' },
    ],
    useCase: 'Finish your app. Click Export. Get a zip. Hand it to your CTO, push it to your own AWS account, or deploy it to any platform. APEX.BUILD never holds your code hostage. It\'s yours from the first commit.',
    visual: 'export',
  },
]

const LEFT_FEATURES  = FEATURES.slice(0, 6)
const RIGHT_FEATURES = FEATURES.slice(6)

interface LandingProps { onGetStarted: (mode?: string, planType?: string) => void }

// ─── Scroll-spy hook ──────────────────────────────────────────────────────────

function useActiveSection() {
  const [active, setActive] = useState<string>('')
  useEffect(() => {
    const obs = new IntersectionObserver(
      entries => {
        const visible = entries.filter(e => e.isIntersecting)
        if (visible.length > 0) setActive(visible[0].target.id)
      },
      { rootMargin: '-30% 0px -60% 0px', threshold: 0 }
    )
    FEATURES.forEach(f => {
      const el = document.getElementById(f.id)
      if (el) obs.observe(el)
    })
    return () => obs.disconnect()
  }, [])
  return active
}

// ─── Sticky side nav (desktop) ────────────────────────────────────────────────

const SideNav: React.FC = () => {
  const active = useActiveSection()
  return (
    <div style={{
      position: 'fixed', left: 20, top: '50%',
      transform: 'translateY(-50%)',
      zIndex: 50, display: 'flex', flexDirection: 'column', gap: 6,
    }}>
      {FEATURES.map(f => (
        <a
          key={f.id}
          href={`#${f.id}`}
          title={f.bullet}
          style={{
            width: 6, height: active === f.id ? 22 : 6,
            borderRadius: 3,
            background: active === f.id ? f.color : C.borderBright,
            transition: 'all 0.3s ease',
            display: 'block',
          }}
        />
      ))}
    </div>
  )
}

// ─── Top nav ──────────────────────────────────────────────────────────────────

const Nav: React.FC<LandingProps> = ({ onGetStarted }) => {
  const [scrolled, setScrolled] = useState(false)
  useEffect(() => {
    const h = () => setScrolled(window.scrollY > 40)
    window.addEventListener('scroll', h)
    return () => window.removeEventListener('scroll', h)
  }, [])

  return (
    <nav style={{
      position: 'fixed', top: 0, left: 0, right: 0, zIndex: 100,
      background: scrolled ? 'rgba(0,0,0,0.95)' : 'transparent',
      backdropFilter: scrolled ? 'blur(16px)' : 'none',
      borderBottom: scrolled ? `1px solid ${C.borderDim}` : '1px solid transparent',
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      padding: '0 36px', height: 56,
      transition: 'all 0.3s ease',
    }}>
      <a href="#" style={{
        display: 'flex', alignItems: 'center', gap: 9, textDecoration: 'none',
      }}>
        <div style={{
          width: 30, height: 30, borderRadius: 7,
          background: `linear-gradient(135deg, ${C.accent} 0%, #ff5500 100%)`,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          boxShadow: `0 0 16px ${C.accentGlow}`,
        }}>
          <Zap size={16} color="#fff" fill="#fff" />
        </div>
        <span style={{
          fontFamily: fHero, fontWeight: 900,
          fontSize: '1.1rem', color: C.white, letterSpacing: '0.07em',
        }}>
          APEX<span style={{ color: C.accent }}>.BUILD</span>
        </span>
      </a>

      <div style={{ display: 'flex', gap: 20, alignItems: 'center' }}>
        {[
          { href: '#agents', label: 'AI Agents' },
          { href: '#cost', label: 'Pricing' },
          { href: '#ide', label: 'IDE' },
          { href: '#byok', label: 'BYOK' },
        ].map(l => (
          <a key={l.href} href={l.href} style={{
            fontFamily: fBody, fontSize: '0.82rem', color: C.textSub,
            textDecoration: 'none', fontWeight: 500, letterSpacing: '0.01em',
            transition: 'color 0.15s',
          }}
            onMouseEnter={e => (e.currentTarget.style.color = C.text)}
            onMouseLeave={e => (e.currentTarget.style.color = C.textSub)}
          >
            {l.label}
          </a>
        ))}
      </div>

      <button onClick={() => onGetStarted()} style={{
        background: `linear-gradient(135deg, ${C.accent} 0%, #cc0029 100%)`,
        color: '#fff', border: 'none', borderRadius: 8,
        padding: '8px 20px', fontFamily: fBody,
        fontWeight: 700, fontSize: '0.84rem', cursor: 'pointer',
        letterSpacing: '0.02em',
        boxShadow: `0 0 20px ${C.accentGlow}`,
        transition: 'box-shadow 0.2s',
      }}>
        Get Started Free
      </button>
    </nav>
  )
}

// ─── Live cost ticker demo ─────────────────────────────────────────────────────

const AGENT_COSTS = [
  { name: 'Architect', color: '#a78bfa', model: 'claude-opus-4-6',       tokens: 2840, cost: 0.0085 },
  { name: 'Backend',   color: '#34d399', model: 'gpt-5',                 tokens: 5120, cost: 0.0128 },
  { name: 'Frontend',  color: '#60a5fa', model: 'claude-sonnet-4-5',     tokens: 4890, cost: 0.0147 },
  { name: 'Reviewer',  color: '#fbbf24', model: 'gemini-3-flash-preview', tokens: 1960, cost: 0.0010 },
  { name: 'Solver',    color: '#f87171', model: 'grok-4-fast',            tokens:  820, cost: 0.0002 },
]

const CostTicker: React.FC = () => {
  const [progress, setProgress] = useState<number[]>([0, 0, 0, 0, 0])
  const [visible, setVisible] = useState(0)
  const [total, setTotal] = useState(0)
  const [done, setDone] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  const run = useCallback(() => {
    setProgress([0, 0, 0, 0, 0])
    setVisible(0)
    setTotal(0)
    setDone(false)
    let idx = 0
    const next = () => {
      if (idx >= AGENT_COSTS.length) { setDone(true); return }
      const i = idx++
      setVisible(i + 1)
      const target = AGENT_COSTS[i].cost
      let cur = 0
      const steps = 45
      const iv = setInterval(() => {
        cur = Math.min(cur + target / steps, target)
        setProgress(p => { const n = [...p]; n[i] = cur; return n })
        setTotal(AGENT_COSTS.slice(0, i).reduce((a, b) => a + b.cost, 0) + cur)
        if (cur >= target) { clearInterval(iv); setTimeout(next, 250) }
      }, 24)
    }
    setTimeout(next, 400)
  }, [])

  useEffect(() => {
    const obs = new IntersectionObserver(([e]) => { if (e.isIntersecting) run() }, { threshold: 0.3 })
    if (ref.current) obs.observe(ref.current)
    return () => obs.disconnect()
  }, [run])

  useEffect(() => {
    if (done) { const t = setTimeout(run, 5000); return () => clearTimeout(t) }
  }, [done, run])

  const totalTokens = AGENT_COSTS.slice(0, visible).reduce((a, b) => a + b.tokens, 0)

  return (
    <div ref={ref} style={{
      background: 'rgba(0,0,0,0.8)', border: `1px solid ${C.border}`,
      borderRadius: 14, overflow: 'hidden',
      boxShadow: `0 0 60px rgba(52,211,153,0.07)`,
      fontFamily: fMono,
    }}>
      {/* Terminal header */}
      <div style={{
        background: 'rgba(255,255,255,0.04)',
        borderBottom: `1px solid ${C.borderDim}`,
        padding: '10px 16px',
        display: 'flex', alignItems: 'center', gap: 7,
      }}>
        {['#ff5f57','#febc2e','#28c840'].map((c, i) => (
          <div key={i} style={{ width: 11, height: 11, borderRadius: '50%', background: c }} />
        ))}
        <span style={{ marginLeft: 8, fontSize: '0.72rem', color: C.textMuted }}>
          apex-build — cost monitor
        </span>
      </div>

      <div style={{ padding: '16px 20px' }}>
        {/* Total */}
        <div style={{
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
          marginBottom: 16,
          paddingBottom: 14, borderBottom: `1px solid ${C.borderDim}`,
        }}>
          <span style={{ fontSize: '0.78rem', color: C.textSub }}>
            <Activity size={12} style={{ display: 'inline', marginRight: 6 }} />
            Live build · {totalTokens.toLocaleString()} tokens
          </span>
          <span style={{
            fontSize: '1.4rem', fontWeight: 700,
            color: done ? C.green : C.green,
            fontVariantNumeric: 'tabular-nums',
            transition: 'color 0.2s',
          }}>
            ${total.toFixed(4)}
          </span>
        </div>

        {/* Agent rows */}
        {AGENT_COSTS.slice(0, visible).map((a, i) => (
          <div key={a.name} style={{ marginBottom: 12 }}>
            <div style={{
              display: 'flex', justifyContent: 'space-between',
              alignItems: 'center', marginBottom: 4,
            }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <div style={{ width: 8, height: 8, borderRadius: '50%', background: a.color }} />
                <span style={{ fontSize: '0.77rem', color: C.text, fontWeight: 600 }}>{a.name}</span>
                <span style={{ fontSize: '0.67rem', color: C.textMuted }}>{a.model}</span>
              </div>
              <span style={{
                fontSize: '0.8rem', color: a.color,
                fontVariantNumeric: 'tabular-nums',
              }}>
                ${progress[i].toFixed(4)}
              </span>
            </div>
            <div style={{ background: C.borderDim, borderRadius: 2, height: 3 }}>
              <div style={{
                height: '100%', borderRadius: 2, background: a.color,
                width: `${a.cost > 0 ? (progress[i] / a.cost) * 100 : 0}%`,
                transition: 'width 0.024s linear',
                boxShadow: `0 0 8px ${a.color}66`,
              }} />
            </div>
          </div>
        ))}

        {done && (
          <div style={{
            display: 'flex', alignItems: 'center', gap: 8, marginTop: 14,
            padding: '10px 14px', borderRadius: 8,
            background: C.greenDim, border: `1px solid ${C.greenBorder}`,
          }}>
            <Check size={13} color={C.green} strokeWidth={2.5} />
            <span style={{ fontSize: '0.78rem', color: C.green, fontWeight: 600 }}>
              Build complete — total cost ${AGENT_COSTS.reduce((a, b) => a + b.cost, 0).toFixed(4)}
            </span>
          </div>
        )}
      </div>
    </div>
  )
}

// ─── Provider badge grid ──────────────────────────────────────────────────────

const PROVIDERS = [
  { name: 'Claude',  sub: 'Anthropic',    color: '#D97757', models: ['Haiku', 'Sonnet', 'Opus'],           costRange: '$0.25–$15/1M' },
  { name: 'GPT-5',   sub: 'OpenAI',       color: '#10A37F', models: ['GPT-5', 'GPT-4o-mini'],              costRange: '$0.15–$2.50/1M' },
  { name: 'Gemini',  sub: 'Google',       color: '#4285F4', models: ['Flash-Lite', 'Flash', 'Pro'],        costRange: '$0.075–$2/1M' },
  { name: 'Grok',    sub: 'xAI',          color: '#e5e5e5', models: ['Grok-4', 'Grok-4-Fast', 'Grok-Mini'], costRange: '$0.20–$2/1M' },
  { name: 'Ollama',  sub: 'Local / Free', color: '#7C3AED', models: ['Llama 3', 'Mistral', 'Any model'],   costRange: 'Free' },
]

// ─── IDE demo mockup ──────────────────────────────────────────────────────────

const IDEDemo: React.FC = () => (
  <div style={{
    background: '#0d0d0d', border: `1px solid ${C.borderDim}`,
    borderRadius: 12, overflow: 'hidden',
    boxShadow: '0 24px 60px rgba(0,0,0,0.6)',
  }}>
    {/* Window chrome */}
    <div style={{
      background: 'rgba(255,255,255,0.04)',
      borderBottom: `1px solid ${C.borderDim}`,
      padding: '9px 14px',
      display: 'flex', alignItems: 'center', gap: 7,
    }}>
      {['#ff5f57','#febc2e','#28c840'].map((c, i) => (
        <div key={i} style={{ width: 10, height: 10, borderRadius: '50%', background: c }} />
      ))}
      <div style={{ marginLeft: 10, display: 'flex', gap: 0 }}>
        {['api/auth.go', 'handlers.go', 'main.go'].map((f, i) => (
          <div key={f} style={{
            padding: '3px 12px', fontSize: '0.7rem', fontFamily: fMono,
            color: i === 0 ? C.text : C.textMuted,
            background: i === 0 ? 'rgba(255,255,255,0.07)' : 'transparent',
            borderRight: `1px solid ${C.borderDim}`,
          }}>{f}</div>
        ))}
      </div>
    </div>
    {/* Code */}
    <div style={{ padding: '16px 20px', fontFamily: fMono, fontSize: '0.72rem', lineHeight: 1.8 }}>
      {[
        { ln: 1,  code: <><span style={{ color: '#a78bfa' }}>func</span> <span style={{ color: '#60a5fa' }}>LoginHandler</span><span style={{ color: C.textSub }}>(c *gin.Context)</span> {'{'}</> },
        { ln: 2,  code: <><span style={{ color: '#9ca3af', paddingLeft: 16 }}>var</span> <span style={{ color: C.text, paddingLeft: 4 }}>req LoginRequest</span></> },
        { ln: 3,  code: <><span style={{ color: C.textMuted, paddingLeft: 16 }}>// Bind JSON, validate, hash-check</span></> },
        { ln: 4,  code: <><span style={{ color: '#34d399', paddingLeft: 16 }}>token</span><span style={{ color: C.textSub }}>, err := </span><span style={{ color: '#60a5fa' }}>generateJWT</span><span style={{ color: C.textSub }}>(user.ID)</span></> },
        { ln: 5,  code: <><span style={{ color: '#a78bfa', paddingLeft: 16 }}>if</span> <span style={{ color: C.text }}> err != nil {'{'}</span></> },
        { ln: 6,  code: <><span style={{ color: '#60a5fa', paddingLeft: 32 }}>c.JSON</span><span style={{ color: C.textSub }}>(500, gin.H{'{'}</span><span style={{ color: '#fbbf24' }}>"error"</span><span style={{ color: C.textSub }}>: err{'}'}</span><span style={{ color: C.textSub }}>)</span></> },
        { ln: 7,  code: <><span style={{ color: C.textSub, paddingLeft: 16 }}>{'}'}</span></> },
        { ln: 8,  code: <><span style={{ color: '#60a5fa', paddingLeft: 16 }}>c.JSON</span><span style={{ color: C.textSub }}>(200, gin.H{'{'}</span><span style={{ color: '#fbbf24' }}>"token"</span><span style={{ color: C.textSub }}>: token{'}'}</span><span style={{ color: C.textSub }}>)</span></> },
        { ln: 9,  code: <><span style={{ color: C.textSub }}>{'}'}</span></> },
      ].map(({ ln, code }) => (
        <div key={ln} style={{ display: 'flex', gap: 20 }}>
          <span style={{ color: C.textMuted, userSelect: 'none', minWidth: 16, textAlign: 'right' }}>{ln}</span>
          <span>{code}</span>
        </div>
      ))}
    </div>
    <div style={{
      background: C.greenDim, borderTop: `1px solid ${C.greenBorder}`,
      padding: '7px 20px',
      display: 'flex', alignItems: 'center', gap: 8,
    }}>
      <div style={{ width: 6, height: 6, borderRadius: '50%', background: C.green }} />
      <span style={{ fontFamily: fMono, fontSize: '0.7rem', color: C.green }}>
        Backend agent — writing auth handler · 3,241 tokens · $0.0097
      </span>
    </div>
  </div>
)

// ─── Above-fold section ───────────────────────────────────────────────────────

const AboveFold: React.FC<LandingProps> = ({ onGetStarted }) => {
  const [mounted, setMounted] = useState(false)
  useEffect(() => { const t = setTimeout(() => setMounted(true), 80); return () => clearTimeout(t) }, [])

  return (
    <section style={{
      minHeight: '100vh', background: C.bg,
      display: 'flex', flexDirection: 'column',
      alignItems: 'center', justifyContent: 'center',
      padding: 'clamp(80px, 10vh, 100px) clamp(20px, 4vw, 48px) 40px',
      position: 'relative', overflow: 'hidden',
    }}>
      {/* Background glow */}
      <div style={{
        position: 'absolute', top: '20%', left: '50%',
        transform: 'translateX(-50%)',
        width: 600, height: 300,
        background: 'radial-gradient(ellipse, rgba(255,0,51,0.07) 0%, transparent 70%)',
        pointerEvents: 'none',
      }} />

      {/* Logo */}
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={mounted ? { opacity: 1, y: 0 } : {}}
        transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
        style={{ textAlign: 'center', marginBottom: 20 }}
      >
        <div style={{
          display: 'inline-flex', alignItems: 'center', gap: 16,
          marginBottom: 14,
        }}>
          <div style={{
            width: 56, height: 56, borderRadius: 14,
            background: `linear-gradient(135deg, ${C.accent} 0%, #ff5500 100%)`,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            boxShadow: `0 0 50px ${C.accentGlow}, 0 0 100px rgba(255,0,51,0.08)`,
          }}>
            <Zap size={30} color="#fff" fill="#fff" />
          </div>
          <h1 style={{
            fontFamily: fHero, fontWeight: 900,
            fontSize: 'clamp(2.4rem, 5.5vw, 3.8rem)',
            color: C.white, letterSpacing: '0.07em',
            margin: 0, lineHeight: 1,
          }}>
            APEX<span style={{ color: C.accent }}>.BUILD</span>
          </h1>
        </div>

        <p style={{
          fontFamily: fBody, fontWeight: 600,
          fontSize: 'clamp(1rem, 2vw, 1.2rem)',
          color: C.textSub, margin: '0 auto 6px',
          letterSpacing: '0.01em',
        }}>
          The AI Cloud IDE that shows you the bill — in real time.
        </p>
        <p style={{
          fontFamily: fBody, fontSize: 'clamp(0.82rem, 1.3vw, 0.92rem)',
          color: C.textMuted, margin: '0 auto',
          maxWidth: 480, lineHeight: 1.55,
        }}>
          5 specialized agents build your full-stack app in parallel. Every token tracked. Every dollar visible. No surprises.
        </p>
      </motion.div>

      {/* 2-column bullets */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={mounted ? { opacity: 1, y: 0 } : {}}
        transition={{ duration: 0.6, delay: 0.15, ease: [0.22, 1, 0.36, 1] }}
        style={{
          display: 'grid', gridTemplateColumns: '1fr 1fr',
          gap: '6px 24px',
          width: '100%', maxWidth: 900,
          margin: '0 auto 24px',
        }}
      >
        {FEATURES.map((f, i) => {
          const Icon = f.icon
          const isLeft = i < 6
          return (
            <a
              key={f.id}
              href={`#${f.id}`}
              style={{
                gridColumn: isLeft ? 1 : 2,
                display: 'flex', alignItems: 'center', gap: 10,
                padding: '9px 13px', borderRadius: 9,
                border: `1px solid ${C.borderDim}`,
                background: C.surface,
                textDecoration: 'none',
                transition: 'all 0.2s ease',
                cursor: 'pointer',
              }}
              onMouseEnter={e => {
                e.currentTarget.style.borderColor = f.color + '50'
                e.currentTarget.style.background = f.color + '0e'
                e.currentTarget.style.transform = 'translateX(2px)'
              }}
              onMouseLeave={e => {
                e.currentTarget.style.borderColor = C.borderDim
                e.currentTarget.style.background = C.surface
                e.currentTarget.style.transform = 'translateX(0)'
              }}
            >
              <div style={{
                width: 26, height: 26, borderRadius: 6, flexShrink: 0,
                background: f.color + '18',
                border: `1px solid ${f.color}28`,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}>
                <Icon size={12} color={f.color} />
              </div>
              <span style={{
                fontFamily: fBody, fontSize: '0.82rem',
                color: C.text, fontWeight: 500, lineHeight: 1.3, flex: 1,
              }}>
                {f.bullet}
              </span>
              <ChevronDown size={12} color={C.textMuted} style={{ flexShrink: 0, rotate: '-90deg' }} />
            </a>
          )
        })}
      </motion.div>

      {/* CTAs */}
      <motion.div
        initial={{ opacity: 0, y: 16 }}
        animate={mounted ? { opacity: 1, y: 0 } : {}}
        transition={{ duration: 0.5, delay: 0.3, ease: [0.22, 1, 0.36, 1] }}
        style={{ display: 'flex', gap: 14, alignItems: 'center', marginBottom: 24 }}
      >
        <button onClick={() => onGetStarted()} style={{
          background: `linear-gradient(135deg, ${C.accent} 0%, #cc0029 100%)`,
          color: '#fff', border: 'none', borderRadius: 10,
          padding: '13px 30px', fontFamily: fBody,
          fontWeight: 700, fontSize: '0.96rem',
          cursor: 'pointer', letterSpacing: '0.02em',
          boxShadow: `0 0 32px ${C.accentGlow}`,
          display: 'flex', alignItems: 'center', gap: 8,
          transition: 'box-shadow 0.2s, transform 0.2s',
        }}
          onMouseEnter={e => {
            e.currentTarget.style.boxShadow = `0 0 50px rgba(255,0,51,0.4)`
            e.currentTarget.style.transform = 'translateY(-1px)'
          }}
          onMouseLeave={e => {
            e.currentTarget.style.boxShadow = `0 0 32px ${C.accentGlow}`
            e.currentTarget.style.transform = 'translateY(0)'
          }}
        >
          Start Building Free <ArrowRight size={16} />
        </button>
        <a href="#agents" style={{
          color: C.textSub, fontFamily: fBody,
          fontSize: '0.88rem', fontWeight: 500,
          textDecoration: 'none',
          display: 'flex', alignItems: 'center', gap: 6,
          transition: 'color 0.15s',
        }}
          onMouseEnter={e => (e.currentTarget.style.color = C.text)}
          onMouseLeave={e => (e.currentTarget.style.color = C.textSub)}
        >
          <Eye size={14} /> See how it works
        </a>
      </motion.div>

      {/* AI provider strip */}
      <motion.div
        initial={{ opacity: 0 }}
        animate={mounted ? { opacity: 1 } : {}}
        transition={{ delay: 0.45, duration: 0.5 }}
        style={{
          display: 'flex', alignItems: 'center', gap: 8,
          flexWrap: 'wrap', justifyContent: 'center',
        }}
      >
        <span style={{ fontFamily: fBody, fontSize: '0.7rem', color: C.textMuted, marginRight: 4 }}>Works with</span>
        {PROVIDERS.map(p => (
          <span key={p.name} style={{
            fontFamily: fBody, fontSize: '0.71rem', fontWeight: 700,
            color: p.color, padding: '3px 11px',
            background: p.color + '12', borderRadius: 100,
            border: `1px solid ${p.color}28`,
          }}>
            {p.name}
          </span>
        ))}
        <span style={{ fontFamily: fBody, fontSize: '0.7rem', color: C.textMuted, marginLeft: 4 }}>
          · No credit card required
        </span>
      </motion.div>

      {/* Scroll hint */}
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: [0, 1, 0] }}
        transition={{ delay: 2, duration: 2.5, repeat: Infinity }}
        style={{
          position: 'absolute', bottom: 28, left: '50%', transform: 'translateX(-50%)',
          display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4,
        }}
      >
        <span style={{ fontFamily: fBody, fontSize: '0.65rem', color: C.textMuted, letterSpacing: '0.12em', textTransform: 'uppercase' }}>
          Scroll to explore
        </span>
        <ChevronDown size={16} color={C.textMuted} />
      </motion.div>
    </section>
  )
}

// ─── Feature detail section ───────────────────────────────────────────────────

const FeatureSection: React.FC<{
  feature: typeof FEATURES[0]
  index: number
  onGetStarted: () => void
}> = ({ feature, index, onGetStarted }) => {
  const Icon = feature.icon
  const isEven = index % 2 === 0

  // Render the right visual for each section
  const renderVisual = () => {
    if (feature.visual === 'cost') return <CostTicker />
    if (feature.visual === 'ide')  return <IDEDemo />
    if (feature.visual === 'ai') {
      return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {PROVIDERS.map(p => (
            <div key={p.name} style={{
              background: C.surface, border: `1px solid ${p.color}28`,
              borderRadius: 10, padding: '14px 18px',
              display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <div style={{
                  width: 36, height: 36, borderRadius: 8,
                  background: p.color + '20', border: `1px solid ${p.color}35`,
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontFamily: fHero, fontWeight: 900, fontSize: '0.7rem', color: p.color,
                }}>{p.name[0]}</div>
                <div>
                  <div style={{ fontFamily: fBody, fontWeight: 700, fontSize: '0.88rem', color: C.text }}>{p.name}</div>
                  <div style={{ fontFamily: fBody, fontSize: '0.72rem', color: C.textMuted }}>{p.sub}</div>
                </div>
              </div>
              <div style={{ textAlign: 'right' }}>
                <div style={{ fontFamily: fMono, fontSize: '0.72rem', color: p.color }}>{p.costRange}</div>
                <div style={{ fontFamily: fBody, fontSize: '0.68rem', color: C.textMuted }}>{p.models.join(' · ')}</div>
              </div>
            </div>
          ))}
        </div>
      )
    }
    if (feature.visual === 'agents') {
      return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {[
            { name: 'Architect',  color: '#a78bfa', status: 'Planning schema…',        pct: 100 },
            { name: 'Backend',    color: '#34d399', status: 'Writing auth endpoints…',  pct:  72 },
            { name: 'Frontend',   color: '#60a5fa', status: 'Building dashboard UI…',   pct:  48 },
            { name: 'Reviewer',   color: '#fbbf24', status: 'Waiting for Backend…',     pct:   0 },
            { name: 'Solver',     color: '#f87171', status: 'On standby',               pct:   0 },
          ].map((a, i) => (
            <motion.div
              key={a.name}
              initial={{ opacity: 0, x: -16 }}
              whileInView={{ opacity: 1, x: 0 }}
              viewport={{ once: true }}
              transition={{ delay: i * 0.1, duration: 0.4 }}
              style={{
                background: C.surface, border: `1px solid ${a.color}22`,
                borderRadius: 9, padding: '12px 16px',
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: a.pct > 0 ? 8 : 0 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <div style={{ width: 8, height: 8, borderRadius: '50%', background: a.pct > 0 ? a.color : C.borderBright }} />
                  <span style={{ fontFamily: fBody, fontWeight: 700, fontSize: '0.84rem', color: C.text }}>{a.name}</span>
                </div>
                <span style={{ fontFamily: fMono, fontSize: '0.7rem', color: a.pct > 0 ? a.color : C.textMuted }}>{a.status}</span>
              </div>
              {a.pct > 0 && (
                <div style={{ background: C.borderDim, borderRadius: 2, height: 3 }}>
                  <motion.div
                    initial={{ width: 0 }}
                    whileInView={{ width: `${a.pct}%` }}
                    viewport={{ once: true }}
                    transition={{ delay: i * 0.1 + 0.3, duration: 0.8, ease: 'easeOut' }}
                    style={{ height: '100%', borderRadius: 2, background: a.color, boxShadow: `0 0 8px ${a.color}55` }}
                  />
                </div>
              )}
            </motion.div>
          ))}
        </div>
      )
    }
    if (feature.visual === 'secrets') {
      return (
        <div style={{
          background: '#0d0d0d', border: `1px solid ${C.borderDim}`,
          borderRadius: 12, overflow: 'hidden', fontFamily: fMono,
        }}>
          <div style={{
            background: 'rgba(255,255,255,0.04)',
            borderBottom: `1px solid ${C.borderDim}`,
            padding: '9px 14px', display: 'flex', alignItems: 'center', gap: 8,
          }}>
            <Lock size={12} color='#818cf8' />
            <span style={{ fontSize: '0.72rem', color: C.textSub }}>Secrets Vault — Project: my-saas-app</span>
          </div>
          {[
            { key: 'STRIPE_SECRET_KEY',   val: 'sk_live_••••••••••••••••••••••••••' },
            { key: 'DATABASE_URL',         val: 'postgresql://••••••••@db.render.com/prod' },
            { key: 'SENDGRID_API_KEY',     val: 'SG.••••••••••••••••••••••••••••••' },
            { key: 'ANTHROPIC_API_KEY',    val: 'sk-ant-api03-••••••••••••••••••••' },
          ].map((s, i) => (
            <div key={s.key} style={{
              padding: '11px 18px',
              borderBottom: `1px solid ${C.borderDim}`,
              display: 'flex', justifyContent: 'space-between', alignItems: 'center',
              background: i % 2 === 0 ? 'transparent' : 'rgba(255,255,255,0.015)',
            }}>
              <div>
                <div style={{ fontSize: '0.74rem', color: '#818cf8', fontWeight: 600, marginBottom: 2 }}>{s.key}</div>
                <div style={{ fontSize: '0.68rem', color: C.textMuted }}>{s.val}</div>
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <div style={{ padding: '3px 8px', borderRadius: 5, background: 'rgba(52,211,153,0.08)', border: '1px solid rgba(52,211,153,0.2)', fontSize: '0.62rem', color: C.green }}>Encrypted</div>
              </div>
            </div>
          ))}
          <div style={{ padding: '9px 18px', fontSize: '0.67rem', color: C.textMuted }}>
            <Eye size={10} style={{ display: 'inline', marginRight: 5 }} />4 secrets · AES-256 · Never exposed in code output
          </div>
        </div>
      )
    }
    // Default: points visual fallback
    return null
  }

  const visual = renderVisual()

  return (
    <section id={feature.id} style={{
      background: isEven ? C.bg : C.bg2,
      borderTop: `1px solid ${C.borderDim}`,
      padding: 'clamp(64px, 8vw, 96px) clamp(24px, 6vw, 80px)',
    }}>
      <div style={{ maxWidth: 1060, margin: '0 auto' }}>
        <div style={{
          display: 'grid',
          gridTemplateColumns: visual ? `1fr 1fr` : '1fr',
          gap: '48px 60px',
          alignItems: 'start',
        }}>
          {/* Left: content */}
          <div>
            <motion.div
              initial={{ opacity: 0, y: 22 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true, margin: '-60px' }}
              transition={{ duration: 0.52, ease: [0.22, 1, 0.36, 1] }}
            >
              {/* Label */}
              <div style={{
                display: 'inline-flex', alignItems: 'center', gap: 8,
                background: feature.color + '12',
                border: `1px solid ${feature.color}30`,
                borderRadius: 100, padding: '4px 13px', marginBottom: 18,
              }}>
                <Icon size={12} color={feature.color} />
                <span style={{
                  fontFamily: fBody, fontSize: '0.7rem', fontWeight: 700,
                  color: feature.color, letterSpacing: '0.09em', textTransform: 'uppercase',
                }}>
                  {feature.bullet}
                </span>
              </div>

              <h2 style={{
                fontFamily: fHero, fontWeight: 900,
                fontSize: 'clamp(1.75rem, 3.2vw, 2.45rem)',
                color: C.text, margin: '0 0 14px', lineHeight: 1.13,
              }}>
                {feature.heading}
              </h2>
              <p style={{
                fontFamily: fBody, fontSize: 'clamp(0.93rem, 1.4vw, 1.05rem)',
                color: C.textSub, lineHeight: 1.74, margin: '0 0 28px',
              }}>
                {feature.sub}
              </p>
            </motion.div>

            {/* Points */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 28 }}>
              {feature.points.map((pt, i) => (
                <motion.div
                  key={pt.label}
                  initial={{ opacity: 0, x: -14 }}
                  whileInView={{ opacity: 1, x: 0 }}
                  viewport={{ once: true, margin: '-40px' }}
                  transition={{ duration: 0.4, delay: i * 0.07, ease: [0.22, 1, 0.36, 1] }}
                  style={{
                    display: 'flex', gap: 14, alignItems: 'flex-start',
                    padding: '12px 16px', borderRadius: 10,
                    background: C.surface, border: `1px solid ${C.borderDim}`,
                  }}
                >
                  <div style={{
                    width: 28, height: 28, borderRadius: 7, flexShrink: 0,
                    background: feature.color + '14',
                    border: `1px solid ${feature.color}28`,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontSize: '0.75rem',
                  }}>
                    {pt.icon}
                  </div>
                  <div>
                    <div style={{
                      fontFamily: fBody, fontWeight: 700,
                      fontSize: '0.88rem', color: feature.color, marginBottom: 3,
                    }}>
                      {pt.label}
                    </div>
                    <div style={{
                      fontFamily: fBody, fontSize: '0.82rem',
                      color: C.textSub, lineHeight: 1.6,
                    }}>
                      {pt.desc}
                    </div>
                  </div>
                </motion.div>
              ))}
            </div>

            {/* Use case */}
            <motion.div
              initial={{ opacity: 0, y: 12 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true, margin: '-30px' }}
              transition={{ duration: 0.45, delay: 0.25 }}
              style={{
                background: feature.color + '09',
                border: `1px solid ${feature.color}24`,
                borderRadius: 11, padding: '16px 20px',
                display: 'flex', gap: 14, alignItems: 'flex-start',
              }}
            >
              <span style={{
                fontFamily: fMono, fontSize: '0.62rem', fontWeight: 700,
                color: feature.color, whiteSpace: 'nowrap',
                textTransform: 'uppercase', letterSpacing: '0.08em', paddingTop: 1,
                background: feature.color + '18', border: `1px solid ${feature.color}30`,
                borderRadius: 5, padding: '3px 8px',
              }}>
                USE CASE
              </span>
              <p style={{
                fontFamily: fBody, fontSize: '0.88rem',
                color: C.text, margin: 0, lineHeight: 1.66,
              }}>
                {feature.useCase}
              </p>
            </motion.div>
          </div>

          {/* Right: visual */}
          {visual && (
            <motion.div
              initial={{ opacity: 0, x: 20 }}
              whileInView={{ opacity: 1, x: 0 }}
              viewport={{ once: true, margin: '-60px' }}
              transition={{ duration: 0.55, delay: 0.12, ease: [0.22, 1, 0.36, 1] }}
            >
              {visual}
            </motion.div>
          )}
        </div>
      </div>
    </section>
  )
}

// ─── Pricing section ──────────────────────────────────────────────────────────

const PricingSection: React.FC<LandingProps> = ({ onGetStarted }) => (
  <section id="pricing" style={{
    background: C.bg,
    borderTop: `1px solid ${C.borderDim}`,
    padding: 'clamp(64px, 8vw, 96px) clamp(24px, 6vw, 80px)',
  }}>
    <div style={{ maxWidth: 1000, margin: '0 auto' }}>
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true, margin: '-60px' }}
        transition={{ duration: 0.5 }}
        style={{ textAlign: 'center', marginBottom: 48 }}
      >
        <div style={{
          display: 'inline-flex', alignItems: 'center', gap: 7,
          background: C.accentDim, border: `1px solid ${C.border}`,
          color: C.accent, borderRadius: 100, padding: '4px 14px',
          fontFamily: fBody, fontSize: '0.7rem', fontWeight: 700,
          letterSpacing: '0.09em', textTransform: 'uppercase', marginBottom: 16,
        }}>
          <Sparkles size={10} /> Pricing
        </div>
        <h2 style={{
          fontFamily: fHero, fontWeight: 900,
          fontSize: 'clamp(1.75rem, 3.5vw, 2.5rem)',
          color: C.white, margin: '0 0 12px',
        }}>
          Start Free. Scale When Ready.
        </h2>
        <p style={{
          fontFamily: fBody, fontSize: 'clamp(0.93rem, 1.4vw, 1.05rem)',
          color: C.textSub, margin: '0 auto',
          maxWidth: 520, lineHeight: 1.7,
        }}>
          No credit card required. Start with included credits.
          Bring your own API keys on any plan and save up to 70%.
        </p>
      </motion.div>

      <div style={{
        display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)',
        gap: 16, marginBottom: 32,
      }}>
        {[
          {
            tier: 'Builder', price: '$19', period: '/mo', highlight: false,
            tag: 'Solo developers',
            features: ['All 5 AI agents', 'Claude · GPT-5 · Gemini · Grok', '5 active projects', 'Managed credits included', 'BYOK support ($0.25/1M)', 'Full cloud IDE'],
          },
          {
            tier: 'Pro', price: '$49', period: '/mo', highlight: true,
            tag: 'Most popular',
            features: ['Everything in Builder', 'Unlimited projects', 'Priority agent queues', 'Higher credit limits', 'Advanced usage analytics', 'Real-time collaboration'],
          },
          {
            tier: 'Team', price: '$99', period: '/mo', highlight: false,
            tag: 'Growing teams',
            features: ['Everything in Pro', 'Role-based access control', 'Shared project workspaces', 'Inline code comments', 'Priority support', 'Team analytics'],
          },
        ].map((plan, i) => (
          <motion.div
            key={plan.tier}
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ delay: i * 0.08, duration: 0.45 }}
            style={{
              background: plan.highlight ? `linear-gradient(160deg, rgba(255,0,51,0.08) 0%, ${C.surface} 100%)` : C.surface,
              border: `1px solid ${plan.highlight ? C.accent : C.borderDim}`,
              borderRadius: 14, padding: '28px 24px',
              boxShadow: plan.highlight ? `0 0 40px ${C.accentGlow}` : 'none',
              position: 'relative',
            }}
          >
            {plan.highlight && (
              <div style={{
                position: 'absolute', top: -11, left: '50%', transform: 'translateX(-50%)',
                background: C.accent, color: '#fff',
                fontFamily: fBody, fontSize: '0.65rem', fontWeight: 700,
                padding: '3px 12px', borderRadius: 100, letterSpacing: '0.06em',
                textTransform: 'uppercase',
              }}>
                {plan.tag}
              </div>
            )}
            <div style={{
              fontFamily: fBody, fontSize: '0.68rem', fontWeight: 700,
              color: plan.highlight ? C.accent : C.textMuted,
              letterSpacing: '0.1em', textTransform: 'uppercase', marginBottom: 8,
            }}>
              {!plan.highlight && plan.tag}
              {plan.highlight && 'For growing builders'}
            </div>
            <div style={{
              fontFamily: fHero, fontWeight: 900,
              fontSize: '1.05rem', color: C.textSub, marginBottom: 4,
            }}>
              {plan.tier}
            </div>
            <div style={{ marginBottom: 20 }}>
              <span style={{ fontFamily: fHero, fontWeight: 900, fontSize: '2.2rem', color: C.white }}>{plan.price}</span>
              <span style={{ fontFamily: fBody, fontSize: '0.82rem', color: C.textSub }}>{plan.period}</span>
            </div>
            {plan.features.map(f => (
              <div key={f} style={{
                display: 'flex', alignItems: 'flex-start', gap: 8, marginBottom: 9,
              }}>
                <Check size={13} color={plan.highlight ? C.accent : C.green} strokeWidth={2.5} style={{ marginTop: 1, flexShrink: 0 }} />
                <span style={{ fontFamily: fBody, fontSize: '0.82rem', color: C.textSub, lineHeight: 1.4 }}>{f}</span>
              </div>
            ))}
            <button onClick={() => onGetStarted()} style={{
              width: '100%', marginTop: 20,
              background: plan.highlight ? C.accent : 'transparent',
              color: plan.highlight ? '#fff' : C.text,
              border: `1px solid ${plan.highlight ? C.accent : C.borderBright}`,
              borderRadius: 8, padding: '10px 0',
              fontFamily: fBody, fontWeight: 700, fontSize: '0.88rem',
              cursor: 'pointer', transition: 'all 0.2s',
            }}>
              Get Started
            </button>
          </motion.div>
        ))}
      </div>

      {/* Credit packs */}
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ delay: 0.3 }}
        style={{
          background: C.surface, border: `1px solid ${C.borderDim}`,
          borderRadius: 12, padding: '20px 28px',
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          flexWrap: 'wrap', gap: 16,
        }}
      >
        <div>
          <div style={{ fontFamily: fBody, fontWeight: 700, fontSize: '0.92rem', color: C.text, marginBottom: 4 }}>
            Just need credits for one project?
          </div>
          <div style={{ fontFamily: fBody, fontSize: '0.82rem', color: C.textSub }}>
            Buy a one-time credit pack — no subscription required. Credits never expire.
          </div>
        </div>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
          {[
            { amount: '$10', credits: '1,000 credits' },
            { amount: '$25', credits: '2,500 credits' },
            { amount: '$50', credits: '5,000 credits' },
            { amount: '$100', credits: '10,000 credits' },
          ].map(p => (
            <button key={p.amount} onClick={() => onGetStarted()} style={{
              background: C.surface, border: `1px solid ${C.borderBright}`,
              borderRadius: 8, padding: '9px 16px',
              fontFamily: fBody, cursor: 'pointer',
              transition: 'all 0.2s',
            }}
              onMouseEnter={e => {
                e.currentTarget.style.borderColor = C.accent
                e.currentTarget.style.background = C.accentDim
              }}
              onMouseLeave={e => {
                e.currentTarget.style.borderColor = C.borderBright
                e.currentTarget.style.background = C.surface
              }}
            >
              <div style={{ fontFamily: fHero, fontWeight: 900, fontSize: '0.95rem', color: C.white }}>{p.amount}</div>
              <div style={{ fontFamily: fBody, fontSize: '0.68rem', color: C.textMuted }}>{p.credits}</div>
            </button>
          ))}
        </div>
      </motion.div>
    </div>
  </section>
)

// ─── Final CTA ─────────────────────────────────────────────────────────────────

const FinalCTA: React.FC<LandingProps> = ({ onGetStarted }) => (
  <section style={{
    background: `radial-gradient(ellipse at 50% 100%, rgba(255,0,51,0.06) 0%, ${C.bg} 70%)`,
    borderTop: `1px solid ${C.borderDim}`,
    padding: 'clamp(64px, 8vw, 96px) clamp(24px, 6vw, 80px)',
    textAlign: 'center',
  }}>
    <motion.div
      initial={{ opacity: 0, y: 24 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: '-60px' }}
      transition={{ duration: 0.55 }}
      style={{ maxWidth: 600, margin: '0 auto' }}
    >
      <div style={{
        width: 56, height: 56, borderRadius: 14, margin: '0 auto 20px',
        background: `linear-gradient(135deg, ${C.accent} 0%, #ff5500 100%)`,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        boxShadow: `0 0 50px ${C.accentGlow}`,
      }}>
        <Zap size={30} color="#fff" fill="#fff" />
      </div>
      <h2 style={{
        fontFamily: fHero, fontWeight: 900,
        fontSize: 'clamp(1.8rem, 3.8vw, 2.6rem)',
        color: C.white, margin: '0 0 14px', lineHeight: 1.12,
      }}>
        Build something real.<br />
        <span style={{ color: C.accent }}>Starting right now.</span>
      </h2>
      <p style={{
        fontFamily: fBody, fontSize: 'clamp(0.93rem, 1.4vw, 1.05rem)',
        color: C.textSub, lineHeight: 1.7, margin: '0 auto 32px',
      }}>
        No credit card. No setup. No waiting.<br />
        Describe your app and watch five AI agents build it.
      </p>
      <button onClick={() => onGetStarted()} style={{
        background: `linear-gradient(135deg, ${C.accent} 0%, #cc0029 100%)`,
        color: '#fff', border: 'none', borderRadius: 11,
        padding: '15px 36px', fontFamily: fBody,
        fontWeight: 700, fontSize: '1.05rem',
        cursor: 'pointer', letterSpacing: '0.02em',
        boxShadow: `0 0 40px ${C.accentGlow}`,
        display: 'inline-flex', alignItems: 'center', gap: 10,
        transition: 'all 0.2s',
      }}
        onMouseEnter={e => {
          e.currentTarget.style.boxShadow = `0 0 60px rgba(255,0,51,0.45)`
          e.currentTarget.style.transform = 'translateY(-2px)'
        }}
        onMouseLeave={e => {
          e.currentTarget.style.boxShadow = `0 0 40px ${C.accentGlow}`
          e.currentTarget.style.transform = 'translateY(0)'
        }}
      >
        Start Building Free <ArrowRight size={18} />
      </button>
    </motion.div>
  </section>
)

// ─── Footer ───────────────────────────────────────────────────────────────────

const Footer: React.FC = () => (
  <footer style={{
    background: '#050505',
    borderTop: `1px solid ${C.borderDim}`,
    padding: '24px clamp(24px, 6vw, 80px)',
    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
    flexWrap: 'wrap', gap: 12,
  }}>
    <div style={{ display: 'flex', alignItems: 'center', gap: 9 }}>
      <div style={{
        width: 24, height: 24, borderRadius: 6,
        background: `linear-gradient(135deg, ${C.accent}, #ff5500)`,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}>
        <Zap size={12} color="#fff" fill="#fff" />
      </div>
      <span style={{ fontFamily: fHero, fontWeight: 900, fontSize: '0.88rem', color: C.white, letterSpacing: '0.07em' }}>
        APEX<span style={{ color: C.accent }}>.BUILD</span>
      </span>
    </div>
    <div style={{ fontFamily: fBody, fontSize: '0.73rem', color: C.textMuted }}>
      © {new Date().getFullYear()} Apex Build · Built with the models it powers
    </div>
    <div style={{ display: 'flex', gap: 20 }}>
      {['Privacy', 'Terms', 'Docs', 'Status'].map(l => (
        <a key={l} href="#" style={{
          fontFamily: fBody, fontSize: '0.73rem', color: C.textMuted,
          textDecoration: 'none', transition: 'color 0.15s',
        }}
          onMouseEnter={e => (e.currentTarget.style.color = C.textSub)}
          onMouseLeave={e => (e.currentTarget.style.color = C.textMuted)}
        >{l}</a>
      ))}
    </div>
  </footer>
)

// ─── Landing page ─────────────────────────────────────────────────────────────

const Landing: React.FC<LandingProps> = ({ onGetStarted }) => (
  <div style={{ background: C.bg, minHeight: '100vh', color: C.text }}>
    <Nav onGetStarted={onGetStarted} />
    <SideNav />
    <AboveFold onGetStarted={onGetStarted} />
    {FEATURES.map((feature, i) => (
      <FeatureSection key={feature.id} feature={feature} index={i} onGetStarted={onGetStarted} />
    ))}
    <PricingSection onGetStarted={onGetStarted} />
    <FinalCTA onGetStarted={onGetStarted} />
    <Footer />
  </div>
)

export { Landing as LandingPage }
export default Landing
