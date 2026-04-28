import React from 'react'
import { motion } from 'framer-motion'
import {
  ArrowRight,
  Brain,
  CheckCircle2,
  ChevronRight,
  Code2,
  Cpu,
  Database,
  DollarSign,
  FileCode2,
  Github,
  KeyRound,
  Layers3,
  LockKeyhole,
  PlugZap,
  Rocket,
  ShieldCheck,
  TerminalSquare,
  UploadCloud,
  Workflow,
} from 'lucide-react'

interface LandingProps {
  onGetStarted: (mode?: string, planType?: string) => void
}

type IconType = React.ComponentType<React.SVGProps<SVGSVGElement>>

const fadeUp = {
  initial: { opacity: 0, y: 22 },
  whileInView: { opacity: 1, y: 0 },
  viewport: { once: true, margin: '-80px' },
  transition: { duration: 0.55, ease: [0.22, 1, 0.36, 1] },
} as const

const FEATURE_PILLARS: Array<{
  icon: IconType
  label: string
  title: string
  body: string
}> = [
  {
    icon: DollarSign,
    label: 'Credit burn',
    title: 'Reduce waste before tokens leave your wallet.',
    body: 'Apex routes work to the cheapest capable model, caps budgets, shows every agent cost live, and lets BYOK or local Ollama models collapse expensive Power Mode runs to a routing fee.',
  },
  {
    icon: ShieldCheck,
    label: 'Enterprise builds',
    title: 'Designed for complex production apps.',
    body: 'Auth, billing, APIs, Postgres, audit logs, tests, reviews, deploy targets, secrets, and handoff docs are first-class build surfaces instead of afterthoughts.',
  },
  {
    icon: Workflow,
    label: '9-agent system',
    title: 'Parallel specialists, not a single chat loop.',
    body: 'Architect, planner, frontend, backend, database, testing, docs, debug, and reviewer agents work from a contract with visible tasks, checkpoints, and verification evidence.',
  },
]

const CONTROL_SURFACES: Array<{ icon: IconType; title: string; body: string; status: string }> = [
  {
    icon: Github,
    title: 'GitHub handoff',
    body: 'Import repos, branch generated work, export ZIPs, and push finished projects back to GitHub with ownership intact.',
    status: 'repo control',
  },
  {
    icon: UploadCloud,
    title: 'Project intake',
    body: 'Bring Replit apps, ZIPs, files, screenshots, images, wireframes, and product context into the build contract.',
    status: 'context control',
  },
  {
    icon: KeyRound,
    title: 'BYOK routing',
    body: 'Assign OpenAI, Anthropic, Gemini, xAI, Ollama Cloud (Pro+), or local models by role while spend stays visible per provider.',
    status: 'model control',
  },
  {
    icon: PlugZap,
    title: 'MCP connectors',
    body: 'Connect tool servers, inspect resources, wire external systems, and let agents call approved tools from project context.',
    status: 'tool control',
  },
  {
    icon: LockKeyhole,
    title: 'Secrets vault',
    body: 'Manage environment variables, API keys, OAuth credentials, SSH keys, rotation, and encrypted handoff rules.',
    status: 'security control',
  },
  {
    icon: Rocket,
    title: 'Deploy gates',
    body: 'Preview, verify, publish, rollback, and hand off deploy targets with logs, approvals, and launch readiness visible.',
    status: 'release control',
  },
  {
    icon: TerminalSquare,
    title: 'Cloud IDE',
    body: 'Monaco editor, file explorer, search, Git, terminal, output, problems, preview, comments, and collaboration panels.',
    status: 'editor control',
  },
  {
    icon: Database,
    title: 'Backend readiness',
    body: 'Track schema, migrations, API contracts, generated services, tests, review gates, and production risk before deploy.',
    status: 'system control',
  },
]

const AGENTS: Array<{
  number: string
  name: string
  model: string
  body: string
  icon: IconType
}> = [
  { number: '01', name: 'Kimi K2.6 Conductor', model: 'Default: Kimi K2.6', body: 'Routes work, tracks dependencies, protects context, and keeps spend inside the build contract.', icon: Workflow },
  { number: '02', name: 'Lead Architect', model: 'Default: Claude Opus 4.6', body: 'Owns architecture, data models, API contracts, security boundaries, and system shape.', icon: Layers3 },
  { number: '03', name: 'Planner', model: 'Default: Gemini 3.1 Pro', body: 'Turns the prompt into work orders, checkpoints, acceptance criteria, and cross-agent memory.', icon: Brain },
  { number: '04', name: 'Frontend', model: 'Default: ChatGPT 5.4 Pro', body: 'Builds React surfaces, layouts, state, accessibility, visual polish, and product flows.', icon: Code2 },
  { number: '05', name: 'Backend', model: 'Default: ChatGPT 5.4 Pro', body: 'Builds APIs, middleware, auth, billing, integrations, background jobs, and services.', icon: TerminalSquare },
  { number: '06', name: 'Database', model: 'Default: Gemini 3.1 Pro', body: 'Designs schema, migrations, seed data, query boundaries, and persistence contracts.', icon: Database },
  { number: '07', name: 'Logic and Debug', model: 'Default: Grok 4.20', body: 'Handles business rules, failure repair, integration conflicts, and complex edge cases.', icon: Cpu },
  { number: '08', name: 'Testing', model: 'Default: Claude Opus 4.6', body: 'Runs unit, integration, E2E, accessibility, smoke, and deploy-readiness checks.', icon: ShieldCheck },
  { number: '09', name: 'Reviewer and Docs', model: 'Default: Claude Opus 4.6 + Gemini 3.1 Pro', body: 'Audits security, code quality, handoff docs, READMEs, runbooks, and launch evidence.', icon: FileCode2 },
]

const MODEL_ASSIGNMENTS = [
  ['Conductor', 'Kimi K2.6'],
  ['Architecture + review', 'Claude Opus 4.6'],
  ['Frontend + backend', 'ChatGPT 5.4 Pro'],
  ['Planning + database', 'Gemini 3.1 Pro'],
  ['Logic + debug', 'Grok 4.20'],
]

const COMPARISON = [
  ['Architecture', '9 specialized agents across routed models', 'Single primary agent loop', 'Frontend-heavy prompt loop', 'Single orchestration path'],
  ['Credit burn', 'Live spend, caps, BYOK, local model swaps', 'Opaque credits', 'Opaque credits', 'Known burn before stability'],
  ['Enterprise app depth', 'Backend, DB, auth, billing, deploy, tests', 'Useful IDE plus agent', 'Strong UI generation', 'Fast prototypes'],
  ['Code ownership', 'GitHub export, ZIP, deploy anywhere', 'Platform-centered', 'Limited handoff', 'Export friction varies'],
  ['Control', 'Model roles, MCP, secrets, settings, review gates', 'Lower-level workspace controls', 'Simple prompt controls', 'Prompt-first controls'],
]

const PRICING_NOTES = [
  'Fast mode uses low-cost models for prototypes and quick iterations.',
  'Balanced mode is the default path for production-quality full-stack work.',
  'Power mode uses flagship routing and heavier validation for critical apps.',
  'BYOK routes your own provider or local model keys with a platform routing fee.',
  'Credit cards are required for trials. Paid-plan trials run 3 days.',
]

const launchCss = `
  .landing-v3 {
    min-height: 100vh;
    color: #f7fbff;
    background:
      linear-gradient(rgba(255,255,255,.025) 1px, transparent 1px),
      linear-gradient(90deg, rgba(255,255,255,.025) 1px, transparent 1px),
      radial-gradient(circle at 20% 16%, rgba(0, 171, 255, .15), transparent 28rem),
      radial-gradient(circle at 88% 20%, rgba(132, 255, 205, .08), transparent 26rem),
      #020408;
    background-size: 96px 96px, 96px 96px, auto, auto, auto;
    font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    overflow-x: hidden;
  }

  .landing-v3 * {
    box-sizing: border-box;
  }

  .launch-shell {
    width: min(1510px, calc(100% - 48px));
    margin: 0 auto;
  }

  .launch-nav {
    position: sticky;
    top: 0;
    z-index: 40;
    border-bottom: 1px solid rgba(255,255,255,.08);
    background: rgba(2, 4, 8, .82);
    backdrop-filter: blur(22px);
  }

  .launch-nav-inner {
    height: 76px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 24px;
  }

  .launch-brand {
    display: inline-flex;
    align-items: center;
    gap: 12px;
    color: #fff;
    text-decoration: none;
    font-weight: 950;
    font-size: 1.02rem;
    text-transform: uppercase;
  }

  .launch-brand img {
    width: 48px;
    height: 48px;
    object-fit: contain;
    filter:
      drop-shadow(0 0 14px rgba(126, 231, 255, .58))
      drop-shadow(0 0 34px rgba(15, 145, 255, .25));
  }

  .launch-brand span {
    font-family: "Arial Black", Impact, Inter, system-ui, sans-serif;
  }

  .launch-brand strong {
    color: #fff;
  }

  .launch-brand em {
    color: #9da7b5;
    font-style: normal;
  }

  .launch-nav-links {
    display: flex;
    align-items: center;
    gap: 28px;
  }

  .launch-nav-links a,
  .launch-link-button {
    color: #aeb8c6;
    font-weight: 720;
    font-size: .94rem;
    text-decoration: none;
    background: transparent;
    border: 0;
    cursor: pointer;
  }

  .launch-nav-links a:hover,
  .launch-link-button:hover {
    color: #fff;
  }

  .launch-nav-actions {
    display: flex;
    align-items: center;
    gap: 14px;
  }

  .launch-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    min-height: 44px;
    border-radius: 12px;
    border: 1px solid rgba(255,255,255,.12);
    padding: 0 18px;
    color: #f8fbff;
    background: rgba(255,255,255,.05);
    font-weight: 850;
    text-decoration: none;
    cursor: pointer;
    transition: transform .18s ease, border-color .18s ease, background .18s ease;
  }

  .launch-btn:hover {
    transform: translateY(-1px);
    border-color: rgba(149, 226, 255, .38);
    background: rgba(255,255,255,.08);
  }

  .launch-btn-primary {
    color: #00111d;
    background: linear-gradient(135deg, #f8fdff, #8adfff 48%, #2fa8ff);
    border-color: rgba(188, 239, 255, .85);
    box-shadow: 0 0 30px rgba(47, 168, 255, .32);
  }

  .launch-btn-primary:hover {
    background: linear-gradient(135deg, #fff, #a7ebff 48%, #37adff);
  }

  .launch-hero {
    position: relative;
    min-height: auto;
    padding: clamp(42px, 6vh, 72px) 0 86px;
    display: grid;
    align-items: start;
  }

  .launch-hero-grid {
    position: relative;
    display: grid;
    grid-template-columns: minmax(760px, 1.25fr) minmax(390px, .75fr);
    gap: clamp(34px, 3.4vw, 56px);
    align-items: start;
  }

  .launch-hero-copy {
    position: relative;
    z-index: 2;
    isolation: isolate;
    display: grid;
    grid-template-columns: clamp(132px, 10vw, 185px) minmax(0, 1fr);
    gap: clamp(22px, 2.3vw, 40px);
    align-items: start;
    padding-left: 0;
  }

  .launch-brain-core {
    position: relative;
    z-index: 0;
    width: 100%;
    max-width: 185px;
    justify-self: end;
    margin-top: clamp(138px, 17vh, 196px);
    pointer-events: none;
    opacity: .86;
    filter:
      drop-shadow(0 0 18px rgba(89, 223, 255, .82))
      drop-shadow(0 0 58px rgba(11, 160, 255, .36));
  }

  .launch-brain-core img {
    display: block;
    width: 100%;
    height: auto;
  }

  .launch-hero-content {
    min-width: 0;
  }

  .launch-eyebrow {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    border: 1px solid rgba(64, 196, 255, .35);
    color: #54cfff;
    background: rgba(0, 137, 255, .12);
    padding: 8px 14px;
    border-radius: 999px;
    font-size: .82rem;
    font-weight: 850;
    margin-bottom: 28px;
  }

  .launch-title {
    margin: 0;
    max-width: 740px;
    color: #f8fbff;
    font-family: "Arial Black", Impact, Inter, system-ui, sans-serif;
    font-size: clamp(3.35rem, 3.75vw, 5.25rem);
    line-height: .94;
    letter-spacing: 0;
    text-transform: uppercase;
    text-wrap: balance;
    text-shadow:
      0 1px 0 #fff,
      0 0 18px rgba(210, 237, 255, .72),
      0 0 52px rgba(76, 201, 255, .45),
      0 14px 48px rgba(0, 0, 0, .95);
  }

  .launch-title .launch-title-line {
    display: block;
    white-space: nowrap;
  }

  .launch-lede {
    max-width: 700px;
    margin: 24px 0 0;
    color: #bec8d6;
    font-size: clamp(1.02rem, 1.12vw, 1.2rem);
    line-height: 1.66;
  }

  .launch-proof-list {
    display: grid;
    gap: 12px;
    margin: 24px 0 0;
    padding: 0;
    list-style: none;
  }

  .launch-proof-list li {
    display: flex;
    gap: 11px;
    color: #dce6f3;
    font-size: 1rem;
    line-height: 1.45;
  }

  .launch-proof-list svg {
    flex: 0 0 auto;
    color: #22d3ee;
    margin-top: 2px;
  }

  .launch-hero-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 14px;
    margin-top: 34px;
  }

  .launch-workspace {
    position: relative;
    z-index: 3;
    margin-top: 28px;
    width: 100%;
    max-width: 680px;
    justify-self: end;
    border: 1px solid rgba(185, 226, 255, .2);
    border-radius: 18px;
    background: linear-gradient(180deg, rgba(12, 17, 26, .94), rgba(3, 7, 14, .98));
    box-shadow:
      0 26px 90px rgba(0, 0, 0, .66),
      0 0 74px rgba(18, 150, 255, .16),
      inset 0 1px 0 rgba(255,255,255,.08);
    overflow: hidden;
  }

  .workspace-top {
    height: 60px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-bottom: 1px solid rgba(255,255,255,.08);
    padding: 0 18px;
    font-family: "JetBrains Mono", Consolas, monospace;
    color: #7d8999;
  }

  .workspace-dots {
    display: flex;
    gap: 8px;
  }

  .workspace-dots span {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    display: block;
  }

  .workspace-dots span:nth-child(1) { background: #ff4d5d; }
  .workspace-dots span:nth-child(2) { background: #f7b84b; }
  .workspace-dots span:nth-child(3) { background: #22d3a6; }

  .workspace-body {
    display: grid;
    grid-template-columns: 206px minmax(0, 1fr);
    min-height: 430px;
  }

  .workspace-agents {
    border-right: 1px solid rgba(255,255,255,.08);
    padding: 22px 18px;
  }

  .workspace-agents h3,
  .workspace-main h3 {
    margin: 0 0 16px;
    color: #7d8999;
    font-size: .75rem;
    text-transform: uppercase;
    letter-spacing: 0;
    font-weight: 850;
  }

  .workspace-agent {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 40px;
    border-radius: 10px;
    color: #b7c2d1;
    padding: 0 10px;
    font-weight: 720;
    font-size: .9rem;
  }

  .workspace-agent.active {
    background: rgba(35, 158, 255, .15);
    color: #eff8ff;
  }

  .workspace-agent i {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #17d3a4;
    box-shadow: 0 0 16px rgba(23, 211, 164, .55);
  }

  .workspace-agent.active i {
    background: #2aa7ff;
    box-shadow: 0 0 18px rgba(42, 167, 255, .72);
  }

  .workspace-main {
    padding: 22px;
    min-width: 0;
  }

  .prompt-box {
    border: 1px solid rgba(255,255,255,.1);
    border-radius: 16px;
    background: rgba(3, 7, 14, .88);
    padding: 20px;
    min-height: 150px;
  }

  .prompt-box p {
    margin: 0;
    color: #f8fbff;
    font-size: 1rem;
    line-height: 1.6;
  }

  .prompt-tools {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    margin-top: 22px;
    padding-top: 16px;
    border-top: 1px solid rgba(255,255,255,.08);
  }

  .prompt-tools span {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: #aeb8c6;
    border: 1px solid rgba(255,255,255,.11);
    background: rgba(255,255,255,.04);
    border-radius: 10px;
    padding: 8px 10px;
    font-size: .82rem;
    font-weight: 760;
  }

  .workspace-metrics {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 14px;
    margin-top: 18px;
  }

  .workspace-metric {
    border: 1px solid rgba(255,255,255,.09);
    border-radius: 14px;
    background: rgba(255,255,255,.035);
    padding: 14px;
  }

  .workspace-metric span {
    display: block;
    color: #7d8999;
    font-size: .72rem;
    text-transform: uppercase;
    font-weight: 850;
  }

  .workspace-metric strong {
    display: block;
    color: #fff;
    font-size: clamp(1rem, 1.02vw, 1.18rem);
    margin-top: 8px;
    line-height: 1.18;
  }

  .terminal-strip {
    margin-top: 18px;
    border: 1px solid rgba(35, 211, 238, .2);
    background: #020408;
    border-radius: 14px;
    padding: 14px;
    color: #67e8f9;
    font-family: "JetBrains Mono", Consolas, monospace;
    font-size: .8rem;
    line-height: 1.8;
  }

  .landing-section {
    padding: 94px 0;
    border-top: 1px solid rgba(255,255,255,.06);
  }

  .section-kicker {
    color: #38bdf8;
    font-size: .82rem;
    font-weight: 900;
    text-transform: uppercase;
    margin-bottom: 14px;
  }

  .section-title {
    max-width: 970px;
    margin: 0;
    color: #fff;
    font-family: "Arial Black", Impact, Inter, system-ui, sans-serif;
    font-size: clamp(2.4rem, 4.2vw, 5.4rem);
    line-height: .98;
    letter-spacing: 0;
    text-transform: uppercase;
    text-shadow: 0 0 30px rgba(148, 225, 255, .18);
  }

  .section-copy {
    max-width: 860px;
    margin: 20px 0 0;
    color: #aeb8c6;
    font-size: 1.08rem;
    line-height: 1.75;
  }

  .pillar-grid,
  .surface-grid,
  .pricing-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 18px;
    margin-top: 36px;
  }

  .launch-card {
    border: 1px solid rgba(255,255,255,.1);
    border-radius: 16px;
    background: linear-gradient(180deg, rgba(255,255,255,.055), rgba(255,255,255,.025));
    box-shadow: inset 0 1px 0 rgba(255,255,255,.07);
    padding: 24px;
  }

  .launch-card:hover {
    border-color: rgba(121, 214, 255, .26);
    background: linear-gradient(180deg, rgba(255,255,255,.07), rgba(255,255,255,.03));
  }

  .card-icon {
    width: 46px;
    height: 46px;
    border-radius: 14px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: #7dd3fc;
    border: 1px solid rgba(125, 211, 252, .2);
    background: rgba(14, 165, 233, .11);
    margin-bottom: 18px;
  }

  .launch-card .label {
    display: block;
    color: #6ee7b7;
    font-size: .74rem;
    font-weight: 900;
    text-transform: uppercase;
    margin-bottom: 12px;
  }

  .launch-card h3 {
    margin: 0;
    color: #fff;
    font-size: 1.22rem;
    line-height: 1.25;
  }

  .launch-card p {
    margin: 12px 0 0;
    color: #aeb8c6;
    line-height: 1.62;
  }

  .surface-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 16px;
  }

  .surface-card {
    position: relative;
    min-height: 252px;
    overflow: hidden;
    background:
      linear-gradient(145deg, rgba(17, 27, 39, .78), rgba(4, 8, 15, .96)),
      radial-gradient(circle at 82% 0%, rgba(56, 189, 248, .18), transparent 16rem);
  }

  .surface-card::before {
    content: "";
    position: absolute;
    inset: 0;
    pointer-events: none;
    background:
      linear-gradient(90deg, transparent, rgba(125, 231, 255, .08), transparent);
    opacity: 0;
    transform: translateX(-80%);
    transition: opacity .2s ease, transform .5s ease;
  }

  .surface-card:hover::before {
    opacity: 1;
    transform: translateX(80%);
  }

  .surface-index {
    position: absolute;
    top: 20px;
    right: 22px;
    color: rgba(174, 184, 198, .22);
    font-family: "Arial Black", Impact, Inter, system-ui, sans-serif;
    font-size: 2.1rem;
    line-height: 1;
  }

  .surface-card h3,
  .surface-card p,
  .surface-card .card-icon,
  .surface-card .surface-status {
    position: relative;
    z-index: 1;
  }

  .surface-command {
    margin-top: 30px;
    border: 1px solid rgba(125, 231, 255, .14);
    border-radius: 18px;
    background: rgba(4, 10, 18, .7);
    box-shadow: inset 0 1px 0 rgba(255,255,255,.06), 0 18px 70px rgba(0, 0, 0, .24);
    padding: 14px;
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }

  .surface-command span {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    border: 1px solid rgba(255,255,255,.1);
    border-radius: 999px;
    background: rgba(255,255,255,.035);
    color: #d8e6f7;
    padding: 9px 12px;
    font-size: .82rem;
    font-weight: 850;
  }

  .surface-command span::before {
    content: "";
    width: 7px;
    height: 7px;
    border-radius: 999px;
    background: #24d3ee;
    box-shadow: 0 0 14px rgba(36, 211, 238, .7);
  }

  .surface-status {
    display: inline-flex;
    margin-top: 20px;
    color: #9ff4d0;
    border: 1px solid rgba(110, 231, 183, .2);
    background: rgba(16, 185, 129, .08);
    border-radius: 999px;
    padding: 6px 9px;
    font-size: .72rem;
    font-weight: 850;
  }

  .agents-section {
    position: relative;
    overflow: hidden;
  }

  .agents-section::before {
    content: "";
    position: absolute;
    inset: 8% -10% auto auto;
    width: 48vw;
    height: 48vw;
    max-width: 760px;
    max-height: 760px;
    pointer-events: none;
    background:
      radial-gradient(circle, rgba(35, 211, 238, .18), transparent 60%),
      conic-gradient(from 110deg, rgba(56, 189, 248, .22), transparent 28%, rgba(110, 231, 183, .12), transparent 58%);
    filter: blur(42px);
    opacity: .42;
  }

  .agents-shell {
    position: relative;
    z-index: 1;
  }

  .agents-layout {
    display: grid;
    grid-template-columns: minmax(0, .95fr) minmax(420px, .72fr);
    gap: clamp(24px, 4vw, 62px);
    align-items: end;
  }

  .agents-title {
    max-width: 880px;
  }

  .agent-proof-rail {
    margin-top: 26px;
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }

  .agent-proof-rail span {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    border: 1px solid rgba(125, 231, 255, .16);
    border-radius: 999px;
    background: rgba(7, 15, 25, .74);
    color: #dbeafe;
    padding: 10px 13px;
    font-size: .84rem;
    font-weight: 850;
  }

  .agent-proof-rail svg {
    color: #38d7ff;
  }

  .agent-command-panel {
    position: relative;
    isolation: isolate;
    border: 1px solid rgba(125, 231, 255, .16);
    border-radius: 22px;
    background:
      linear-gradient(145deg, rgba(10, 16, 26, .98), rgba(2, 6, 12, .99)),
      radial-gradient(circle at 80% 0%, rgba(56, 189, 248, .2), transparent 18rem);
    box-shadow:
      inset 0 1px 0 rgba(255,255,255,.08),
      0 24px 90px rgba(0, 0, 0, .35),
      0 0 52px rgba(56, 189, 248, .08);
    padding: 22px;
    overflow: hidden;
  }

  .agent-command-panel::before {
    content: "";
    position: absolute;
    inset: 0;
    pointer-events: none;
    background:
      linear-gradient(rgba(255,255,255,.04) 1px, transparent 1px),
      linear-gradient(90deg, rgba(255,255,255,.04) 1px, transparent 1px);
    background-size: 38px 38px;
    mask-image: linear-gradient(to bottom, black, transparent 82%);
  }

  .agent-panel-top,
  .agent-pipeline,
  .agent-budget-line {
    position: relative;
    z-index: 1;
  }

  .agent-panel-top {
    display: flex;
    justify-content: space-between;
    gap: 18px;
    align-items: start;
    border-bottom: 1px solid rgba(255,255,255,.09);
    padding-bottom: 18px;
  }

  .agent-panel-top span,
  .agent-budget-line span,
  .agent-step em {
    color: #8da0b8;
    font-size: .72rem;
    text-transform: uppercase;
    letter-spacing: .12em;
    font-style: normal;
  }

  .agent-panel-top strong {
    color: #fff;
    font-size: 1.18rem;
  }

  .agent-panel-meter {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: #8ff9cf;
    font-family: "JetBrains Mono", Consolas, monospace;
    font-size: .82rem;
    font-weight: 900;
  }

  .agent-panel-meter::before {
    content: "";
    width: 8px;
    height: 8px;
    border-radius: 999px;
    background: #24d3ee;
    box-shadow: 0 0 18px rgba(36, 211, 238, .75);
  }

  .agent-pipeline {
    display: grid;
    gap: 10px;
    margin-top: 18px;
  }

  .agent-step {
    display: grid;
    grid-template-columns: 36px minmax(0, 1fr) auto;
    gap: 12px;
    align-items: center;
    border: 1px solid rgba(255,255,255,.08);
    border-radius: 14px;
    background: rgba(255,255,255,.035);
    padding: 12px;
  }

  .agent-step span {
    color: #38d7ff;
    font-family: "JetBrains Mono", Consolas, monospace;
    font-weight: 950;
  }

  .agent-step strong {
    color: #f8fbff;
  }

  .agent-budget-line {
    margin-top: 16px;
    border: 1px solid rgba(110, 231, 183, .16);
    border-radius: 16px;
    background: rgba(6, 78, 59, .12);
    display: flex;
    justify-content: space-between;
    gap: 18px;
    padding: 14px;
  }

  .agent-budget-line strong {
    color: #9ff4d0;
    font-family: "JetBrains Mono", Consolas, monospace;
  }

  .agent-model-map {
    position: relative;
    z-index: 1;
    margin-top: 16px;
    border: 1px solid rgba(125, 231, 255, .12);
    border-radius: 16px;
    background: rgba(255,255,255,.035);
    padding: 14px;
  }

  .agent-model-map > span,
  .agent-custom-note span {
    display: block;
    color: #8da0b8;
    font-size: .72rem;
    text-transform: uppercase;
    letter-spacing: .12em;
    margin-bottom: 10px;
  }

  .agent-model-map ul {
    list-style: none;
    padding: 0;
    margin: 0;
    display: grid;
    gap: 8px;
  }

  .agent-model-map li {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 16px;
    color: #aeb8c6;
    font-size: .88rem;
  }

  .agent-model-map strong {
    color: #f8fbff;
    font-family: "JetBrains Mono", Consolas, monospace;
    font-size: .82rem;
    text-align: right;
  }

  .agent-custom-note {
    position: relative;
    z-index: 1;
    margin-top: 16px;
    border: 1px solid rgba(110, 231, 183, .18);
    border-radius: 16px;
    background: linear-gradient(135deg, rgba(6, 78, 59, .18), rgba(14, 165, 233, .08));
    padding: 14px;
  }

  .agent-custom-note p {
    margin: 0;
    color: #d8e6f7;
    line-height: 1.5;
    font-size: .92rem;
  }

  .agent-board {
    margin-top: 28px;
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 14px;
  }

  .agent-row {
    position: relative;
    overflow: hidden;
    display: grid;
    grid-template-columns: 46px minmax(0, 1fr);
    gap: 15px;
    min-height: 184px;
    border: 1px solid rgba(255,255,255,.09);
    border-radius: 18px;
    background:
      linear-gradient(145deg, rgba(16, 24, 36, .66), rgba(3, 7, 14, .9)),
      radial-gradient(circle at 100% 0%, rgba(56, 189, 248, .12), transparent 13rem);
    box-shadow: inset 0 1px 0 rgba(255,255,255,.055);
    padding: 18px;
  }

  .agent-row::before {
    content: "";
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 2px;
    background: linear-gradient(90deg, #24d3ee, rgba(110, 231, 183, .2), transparent);
    opacity: .62;
  }

  .agent-row:hover {
    border-color: rgba(125, 231, 255, .24);
    transform: translateY(-2px);
  }

  .agent-icon {
    position: relative;
    z-index: 1;
    width: 42px;
    height: 42px;
    border-radius: 14px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: #7dd3fc;
    border: 1px solid rgba(125, 211, 252, .2);
    background: rgba(14, 165, 233, .11);
  }

  .agent-row-content {
    position: relative;
    z-index: 1;
    min-width: 0;
  }

  .agent-card-top {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    align-items: start;
  }

  .agent-number {
    color: #38d7ff;
    font-family: "JetBrains Mono", Consolas, monospace;
    font-weight: 950;
    font-size: .86rem;
  }

  .agent-model {
    color: #b7eaff;
    font-size: .72rem;
    text-transform: uppercase;
    letter-spacing: .08em;
    text-align: right;
    max-width: 210px;
  }

  .agent-row strong {
    display: block;
    color: #fff;
    margin-top: 12px;
    margin-bottom: 6px;
    font-size: 1.08rem;
  }

  .agent-row p {
    margin: 0;
    color: #aeb8c6;
    font-size: .94rem;
    line-height: 1.55;
  }

  .comparison-wrap {
    margin-top: 36px;
    overflow-x: auto;
    border: 1px solid rgba(255,255,255,.1);
    border-radius: 18px;
    background: rgba(255,255,255,.035);
  }

  .comparison-table {
    width: 100%;
    min-width: 900px;
    border-collapse: collapse;
  }

  .comparison-table th,
  .comparison-table td {
    padding: 18px;
    border-bottom: 1px solid rgba(255,255,255,.075);
    color: #aeb8c6;
    text-align: left;
    vertical-align: top;
    line-height: 1.45;
  }

  .comparison-table th {
    color: #fff;
    font-size: .82rem;
    text-transform: uppercase;
  }

  .comparison-table td:first-child,
  .comparison-table td:nth-child(2) {
    color: #f7fbff;
    font-weight: 780;
  }

  .pricing-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .price-card {
    min-height: 320px;
  }

  .price-card.featured {
    border-color: rgba(56, 189, 248, .35);
    box-shadow: 0 0 54px rgba(56, 189, 248, .12), inset 0 1px 0 rgba(255,255,255,.08);
  }

  .price-card .price {
    display: block;
    color: #fff;
    font-size: 2.3rem;
    font-weight: 950;
    margin: 18px 0 6px;
  }

  .price-card ul,
  .truth-list {
    list-style: none;
    padding: 0;
    margin: 18px 0 0;
    display: grid;
    gap: 10px;
  }

  .price-card li,
  .truth-list li {
    display: flex;
    gap: 10px;
    color: #b8c4d4;
    line-height: 1.45;
  }

  .price-card li svg,
  .truth-list li svg {
    flex: 0 0 auto;
    color: #6ee7b7;
    margin-top: 2px;
  }

  .cta-band {
    padding: 76px 0;
  }

  .cta-inner {
    border: 1px solid rgba(255,255,255,.12);
    border-radius: 24px;
    background:
      linear-gradient(135deg, rgba(56, 189, 248, .13), rgba(110, 231, 183, .06)),
      rgba(255,255,255,.04);
    padding: clamp(28px, 5vw, 58px);
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 30px;
    align-items: center;
  }

  .footer {
    border-top: 1px solid rgba(255,255,255,.08);
    padding: 34px 0;
    color: #758195;
  }

  .footer-inner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 24px;
  }

  .footer a {
    color: #aeb8c6;
    text-decoration: none;
    margin-left: 18px;
  }

  @media (max-width: 1200px) {
    .launch-hero-grid {
      grid-template-columns: 1fr;
    }

    .launch-workspace {
      max-width: 920px;
      justify-self: center;
      width: 100%;
    }

    .surface-grid,
    .pricing-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .agents-layout {
      grid-template-columns: 1fr;
      align-items: start;
    }

    .agent-command-panel {
      max-width: 760px;
    }

    .agent-board {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 820px) {
    .launch-shell {
      width: min(100% - 28px, 1510px);
    }

    .launch-nav-links,
    .launch-nav-actions .launch-link-button {
      display: none;
    }

    .launch-hero {
      padding: 42px 0 60px;
    }

    .launch-hero-copy {
      grid-template-columns: minmax(86px, 118px) minmax(0, 1fr);
      gap: 18px;
      align-items: start;
      padding-left: 0;
      padding-top: 0;
    }

    .launch-brain-core {
      width: 100%;
      max-width: 118px;
      justify-self: start;
      margin-top: 58px;
      opacity: .48;
      filter:
        drop-shadow(0 0 14px rgba(89, 223, 255, .62))
        drop-shadow(0 0 34px rgba(11, 160, 255, .25));
    }

    .launch-title {
      font-size: clamp(3.1rem, 14vw, 5.2rem);
    }

    .launch-title .launch-title-line {
      white-space: normal;
    }

    .workspace-body {
      grid-template-columns: 1fr;
    }

    .workspace-agents {
      display: none;
    }

    .workspace-metrics,
    .pillar-grid,
    .surface-grid,
    .agent-board,
    .pricing-grid {
      grid-template-columns: 1fr;
    }

    .agents-title {
      font-size: clamp(2.5rem, 11vw, 4.8rem);
    }

    .agent-command-panel {
      padding: 16px;
    }

    .agent-step,
    .agent-budget-line {
      grid-template-columns: 1fr;
    }

    .agent-model-map li,
    .agent-budget-line {
      align-items: flex-start;
      flex-direction: column;
      gap: 6px;
    }

    .agent-model {
      text-align: left;
      max-width: none;
    }

    .cta-inner,
    .footer-inner {
      grid-template-columns: 1fr;
      display: grid;
    }

    .footer a {
      margin-left: 0;
      margin-right: 16px;
    }
  }

  @media (max-width: 560px) {
    .landing-v3,
    .launch-nav,
    .launch-hero {
      width: 100vw;
      max-width: 100vw;
      overflow-x: hidden;
    }

    .launch-shell {
      width: min(1510px, calc(100% - 24px));
    }

    .launch-nav-inner {
      position: relative;
      height: 68px;
      gap: 10px;
    }

    .launch-brand {
      gap: 8px;
      font-size: .88rem;
    }

    .launch-brand img {
      width: 38px;
      height: 38px;
    }

    .launch-nav-actions {
      display: none;
    }

    .launch-nav-actions .launch-btn-primary {
      width: 44px;
      min-width: 44px;
      min-height: 40px;
      padding: 0;
      border-radius: 11px;
      font-size: 0;
      gap: 0;
    }

    .launch-nav-actions .launch-btn-primary svg {
      display: block;
      width: 18px;
      height: 18px;
    }

    .launch-hero-copy {
      display: block;
      padding-top: 0;
    }

    .launch-brain-core {
      display: none;
    }

    .launch-eyebrow {
      max-width: 100%;
      border-radius: 14px;
      padding: 8px 11px;
      font-size: .73rem;
    }

    .launch-title {
      max-width: 100%;
      font-size: clamp(2.28rem, 9.1vw, 2.85rem);
      line-height: .96;
      overflow-wrap: break-word;
    }

    .launch-lede {
      font-size: .96rem;
      line-height: 1.58;
    }

    .launch-lede,
    .launch-proof-list li {
      overflow-wrap: anywhere;
    }

    .launch-title .launch-title-line {
      white-space: normal;
    }

    .launch-hero-actions {
      display: grid;
      grid-template-columns: 1fr;
    }

    .launch-hero-actions .launch-btn {
      width: 100%;
    }
  }
`

const Brand = () => (
  <a className="launch-brand" href="/" aria-label="APEX-BUILD home">
    <img src="/apex-build-mark-metal.png" alt="" aria-hidden="true" />
    <span>
      <strong>APEX</strong>
      <em>-BUILD</em>
    </span>
  </a>
)

const Nav: React.FC<LandingProps> = ({ onGetStarted }) => (
  <header className="launch-nav">
    <div className="launch-shell launch-nav-inner">
      <Brand />
      <nav className="launch-nav-links" aria-label="Primary navigation">
        <a href="#features">Features</a>
        <a href="#agents">Agents</a>
        <a href="#pricing">Pricing</a>
        <a href="#docs">Docs</a>
      </nav>
      <div className="launch-nav-actions">
        <button className="launch-link-button" onClick={() => onGetStarted('login')}>Sign in</button>
        <button className="launch-btn launch-btn-primary" onClick={() => onGetStarted('register')}>
          Start building <ArrowRight width={18} height={18} />
        </button>
      </div>
    </div>
  </header>
)

const HeroWorkspace = () => (
  <motion.aside
    className="launch-workspace"
    initial={false}
    animate={{ opacity: 1, y: 0, scale: 1 }}
    transition={{ duration: .7, delay: .15, ease: [0.22, 1, 0.36, 1] }}
    aria-label="APEX-BUILD workspace preview"
  >
    <div className="workspace-top">
      <div className="workspace-dots" aria-hidden="true"><span /><span /><span /></div>
      <span>apex-build.dev/workspace</span>
      <strong style={{ color: '#29a8ff' }}>$0.144 / $0.500</strong>
    </div>
    <div className="workspace-body">
      <div className="workspace-agents">
        <h3>Kimi K2.6 conductor</h3>
        {['Architect', 'Reviewer', 'Frontend', 'Backend', 'Database', 'Testing', 'Docs', 'Debug'].map((agent, index) => (
          <div key={agent} className={`workspace-agent ${index >= 2 && index <= 5 ? 'active' : ''}`}>
            <i />
            {agent}
          </div>
        ))}
      </div>
      <div className="workspace-main">
        <h3>Production build contract</h3>
        <div className="prompt-box">
          <p>
            Build a multi-tenant customer portal with auth, Stripe billing, Postgres,
            audit logs, admin roles, tests, GitHub export, and a deploy-ready Vercel frontend.
          </p>
          <div className="prompt-tools">
            <span><Github width={14} height={14} /> GitHub</span>
            <span><UploadCloud width={14} height={14} /> Replit</span>
            <span><FileCode2 width={14} height={14} /> ZIP</span>
            <span><Brain width={14} height={14} /> Image</span>
            <span><PlugZap width={14} height={14} /> MCP</span>
          </div>
        </div>
        <div className="workspace-metrics">
          <div className="workspace-metric"><span>Credit burn</span><strong>80%+ lower</strong></div>
          <div className="workspace-metric"><span>Build class</span><strong>enterprise</strong></div>
          <div className="workspace-metric"><span>Verification</span><strong>pre-ship</strong></div>
        </div>
        <div className="terminal-strip">
          <div>ok repository imported - schema mapped - budget cap active</div>
          <div>ok auth, billing, api routes, tests assigned to agents</div>
          <div>ok preview, GitHub export, deploy checks queued</div>
        </div>
      </div>
    </div>
  </motion.aside>
)

const Hero: React.FC<LandingProps> = ({ onGetStarted }) => (
  <section className="launch-hero">
    <div className="launch-shell launch-hero-grid">
      <div className="launch-hero-copy">
        <div className="launch-brain-core" aria-hidden="true">
          <img src="/apex-neural-core-cutout.png" alt="" />
        </div>
        <motion.div
          className="launch-hero-content"
          initial={false}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: .5 }}
        >
          <div className="launch-eyebrow">
            <Cpu width={15} height={15} />
            The next generation of AI software creation
          </div>
          <h1 className="launch-title">
            <span className="launch-title-line">The most</span>
            <span className="launch-title-line">advanced AI</span>
            <span className="launch-title-line">app builder</span>
            <span className="launch-title-line">ever created.</span>
          </h1>
          <p className="launch-lede">
            APEX-BUILD turns one prompt into production-grade software using 9 specialized agents,
            flagship and open-weight model routing, Kimi K2.6 orchestration, live cost controls,
            BYOK, GitHub export, and a full enterprise cloud IDE.
          </p>
          <ul className="launch-proof-list">
            <li><CheckCircle2 width={18} height={18} /> Cuts token waste by routing each task to the model built for that job.</li>
            <li><CheckCircle2 width={18} height={18} /> Built for complex enterprise apps: auth, billing, data, APIs, tests, review, deploy.</li>
            <li><CheckCircle2 width={18} height={18} /> Gives engineers control surfaces for every serious workflow, not just a prompt box.</li>
          </ul>
          <div className="launch-hero-actions">
            <button className="launch-btn launch-btn-primary" onClick={() => onGetStarted('register', 'pro')}>
              Build with Apex <ArrowRight width={18} height={18} />
            </button>
            <a className="launch-btn" href="#features">
              See the control surface <ChevronRight width={18} height={18} />
            </a>
          </div>
        </motion.div>
      </div>
      <HeroWorkspace />
    </div>
  </section>
)

const Pillars = () => (
  <section id="features" className="landing-section">
    <div className="launch-shell">
      <motion.div {...fadeUp}>
        <div className="section-kicker">Why Apex wins</div>
        <h2 className="section-title">Lower credit burn. Higher software ceiling.</h2>
        <p className="section-copy">
          Apex is not a prettier prompt box. It is a multi-agent production software factory
          with economics, architecture, and verification designed into the workflow.
        </p>
      </motion.div>
      <div className="pillar-grid">
        {FEATURE_PILLARS.map((item) => (
          <motion.article key={item.title} className="launch-card" {...fadeUp}>
            <div className="card-icon"><item.icon width={23} height={23} /></div>
            <span className="label">{item.label}</span>
            <h3>{item.title}</h3>
            <p>{item.body}</p>
          </motion.article>
        ))}
      </div>
    </div>
  </section>
)

const Surfaces = () => (
  <section className="landing-section">
    <div className="launch-shell">
      <motion.div {...fadeUp}>
        <div className="section-kicker">Platform cockpit</div>
        <h2 className="section-title">Control the build from import to deploy.</h2>
        <p className="section-copy">
          Apex gives serious builders the levers they expect: context intake, model routing,
          MCP tools, secrets, GitHub handoff, database readiness, review gates, deploy controls,
          and live spend visibility in one connected workflow.
        </p>
      </motion.div>
      <div className="surface-command" aria-label="Platform control rail">
        <span>Import context</span>
        <span>Route models</span>
        <span>Attach tools</span>
        <span>Secure secrets</span>
        <span>Review evidence</span>
        <span>Ship anywhere</span>
      </div>
      <div className="surface-grid">
        {CONTROL_SURFACES.map((item, index) => (
          <motion.article key={item.title} className="launch-card surface-card" {...fadeUp}>
            <span className="surface-index">{String(index + 1).padStart(2, '0')}</span>
            <div className="card-icon"><item.icon width={22} height={22} /></div>
            <h3>{item.title}</h3>
            <p>{item.body}</p>
            <span className="surface-status">{item.status}</span>
          </motion.article>
        ))}
      </div>
    </div>
  </section>
)

const Agents = () => (
  <section id="agents" className="landing-section agents-section">
    <div className="launch-shell agents-shell">
      <div className="agents-layout">
        <motion.div className="agents-copy" {...fadeUp}>
          <div className="section-kicker">Agent architecture</div>
          <h2 className="section-title agents-title">Nine specialists. One governed build.</h2>
          <p className="section-copy">
            Apex breaks complex software requests into owned work orders, routes them to the right
            model lane, then keeps every agent moving in parallel with checkpoints, budget caps,
            review evidence, and deploy gates.
          </p>
          <div className="agent-proof-rail" aria-label="Agent system controls">
            <span><Workflow width={15} height={15} /> Parallel work orders</span>
            <span><DollarSign width={15} height={15} /> Spend guardrails</span>
            <span><ShieldCheck width={15} height={15} /> Review gates</span>
            <span><Github width={15} height={15} /> GitHub handoff</span>
          </div>
        </motion.div>
        <motion.aside className="agent-command-panel" aria-label="Live orchestration preview" {...fadeUp}>
          <div className="agent-panel-top">
            <div>
              <span>orchestration live</span>
              <strong>Build contract executing</strong>
            </div>
            <div className="agent-panel-meter">09 active</div>
          </div>
          <div className="agent-pipeline">
            <div className="agent-step"><span>01</span><strong>Plan</strong><em>locked</em></div>
            <div className="agent-step"><span>02</span><strong>Build</strong><em>parallel</em></div>
            <div className="agent-step"><span>03</span><strong>Review</strong><em>running</em></div>
            <div className="agent-step"><span>04</span><strong>Deploy</strong><em>gated</em></div>
          </div>
          <div className="agent-model-map" aria-label="Default model assignments">
            <span>default model map</span>
            <ul>
              {MODEL_ASSIGNMENTS.map(([task, model]) => (
                <li key={task}>
                  {task}
                  <strong>{model}</strong>
                </li>
              ))}
            </ul>
          </div>
          <div className="agent-budget-line">
            <span>current spend</span>
            <strong>$0.144 / $0.500</strong>
          </div>
          <div className="agent-custom-note">
            <span>customizable routing</span>
            <p>
              These are default assignments. Swap any role to your own OpenAI, Anthropic,
              Gemini, xAI, Kimi, Ollama cloud, or local model setup whenever you want.
            </p>
          </div>
        </motion.aside>
      </div>
      <div className="agent-board">
        {AGENTS.map((agent) => (
          <motion.div key={agent.name} className="agent-row" {...fadeUp}>
            <div className="agent-icon"><agent.icon width={21} height={21} /></div>
            <div className="agent-row-content">
              <div className="agent-card-top">
                <span className="agent-number">{agent.number}</span>
                <span className="agent-model">{agent.model}</span>
              </div>
              <strong>{agent.name}</strong>
              <p>{agent.body}</p>
            </div>
          </motion.div>
        ))}
      </div>
    </div>
  </section>
)

const ModelRouting = () => (
  <section className="landing-section">
    <div className="launch-shell">
      <motion.div {...fadeUp}>
        <div className="section-kicker">BYOK and open-weight routing</div>
        <h2 className="section-title">Bring the expensive models. Replace them when you want.</h2>
        <p className="section-copy">
          Use hosted flagship models when quality demands it, or swap roles to Ollama cloud and local
          open-weight models when cost matters. Apex keeps the same build contract and orchestration layer.
        </p>
      </motion.div>
      <div className="pillar-grid">
        {[
          {
            title: 'Flagship role routing',
            body: 'Claude, OpenAI, Gemini, xAI, and Kimi roles can be assigned by task shape and quality target.',
            Icon: Brain,
          },
          {
            title: 'Ollama Cloud (Pro+)',
            body: 'Route any agent role through kimi-k2.6, GLM-5.1, DeepSeek V4, Qwen 3.5, Gemma 4, Devstral 2, or more — 7 top open-weight models, one flat-rate subscription.',
            Icon: Cpu,
          },
          {
            title: 'Live cost ledger',
            body: 'Every model call is reflected in the budget ticker, provider scorecard, and credit ledger as the build runs.',
            Icon: DollarSign,
          },
        ].map(({ title, body, Icon }) => {
          const ModelIcon = Icon as IconType
          return (
            <motion.article key={title} className="launch-card" {...fadeUp}>
              <div className="card-icon"><ModelIcon width={22} height={22} /></div>
              <h3>{title}</h3>
              <p>{body}</p>
            </motion.article>
          )
        })}
      </div>
    </div>
  </section>
)

const Comparison = () => (
  <section className="landing-section">
    <div className="launch-shell">
      <motion.div {...fadeUp}>
        <div className="section-kicker">Head to head</div>
        <h2 className="section-title">Built to beat single-agent app builders.</h2>
        <p className="section-copy">
          Replit, Lovable, and Bolt are useful tools. Apex is positioned as the next step:
          a controlled multi-agent build system for people shipping real software.
        </p>
      </motion.div>
      <div className="comparison-wrap">
        <table className="comparison-table">
          <thead>
            <tr>
              <th>Category</th>
              <th>APEX-BUILD</th>
              <th>Replit</th>
              <th>Lovable</th>
              <th>Bolt</th>
            </tr>
          </thead>
          <tbody>
            {COMPARISON.map((row) => (
              <tr key={row[0]}>
                {row.map((cell) => <td key={cell}>{cell}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  </section>
)

const Pricing = ({ onGetStarted }: LandingProps) => (
  <section id="pricing" className="landing-section">
    <div className="launch-shell">
      <motion.div {...fadeUp}>
        <div className="section-kicker">Pricing truth</div>
        <h2 className="section-title">Simple trials. Transparent usage. No fake free lunch.</h2>
        <p className="section-copy">
          Apex should feel honest from the first click: credit cards are required for trials,
          paid-plan trials are 3 days, and model spend stays visible while agents work.
        </p>
      </motion.div>
      <div className="pricing-grid">
        {[
          ['Free', '$0', ['Static frontend experiments', 'Prompt-to-UI exploration', 'Upgrade required for backend/auth/billing']],
          ['Builder', '$24', ['Full-stack builds', 'GitHub import/export', 'BYOK — any provider API key', 'Live preview and IDE']],
          ['Pro', '$59', ['Ollama Cloud builds — 7 top open-weight models', 'BYOK controls', 'Budget caps', 'Priority queue'], true],
          ['Team', '$149', ['Everything in Pro', 'Collaboration', 'Shared secrets', 'Admin controls']],
        ].map(([name, price, features, featured]) => (
          <motion.article key={name as string} className={`launch-card price-card ${featured ? 'featured' : ''}`} {...fadeUp}>
            <h3>{name}</h3>
            <span className="price">{price}</span>
            <button className="launch-btn launch-btn-primary" onClick={() => onGetStarted('register', String(name).toLowerCase())}>
              Select {name}
            </button>
            <ul>
              {(features as string[]).map((feature) => (
                <li key={feature}><CheckCircle2 width={16} height={16} /> {feature}</li>
              ))}
            </ul>
          </motion.article>
        ))}
      </div>
      <ul className="truth-list" style={{ marginTop: 28 }}>
        {PRICING_NOTES.map((note) => (
          <li key={note}><CheckCircle2 width={17} height={17} /> {note}</li>
        ))}
      </ul>
    </div>
  </section>
)

const Docs = () => (
  <section id="docs" className="landing-section">
    <div className="launch-shell">
      <motion.div {...fadeUp}>
        <div className="section-kicker">Operator documentation</div>
        <h2 className="section-title">Readable for founders. Precise enough for engineers.</h2>
        <p className="section-copy">
          Apex output includes runbooks, architecture notes, API documentation, deployment notes,
          environment variables, billing assumptions, and a verification summary so teams can own the code.
        </p>
      </motion.div>
      <div className="pillar-grid">
        {[
          {
            title: 'Build contract',
            body: 'Intent, scope, stack, risk, acceptance criteria, and work orders stay visible before the run begins.',
            Icon: Layers3,
          },
          {
            title: 'Review evidence',
            body: 'Code review, test status, security checks, and unresolved risks are exposed before deploy handoff.',
            Icon: ShieldCheck,
          },
          {
            title: 'Handoff docs',
            body: 'README, local run instructions, env vars, APIs, deploy notes, and ownership map ship with the repo.',
            Icon: Code2,
          },
        ].map(({ title, body, Icon }) => {
          const DocsIcon = Icon as IconType
          return (
            <motion.article key={title} className="launch-card" {...fadeUp}>
              <div className="card-icon"><DocsIcon width={22} height={22} /></div>
              <h3>{title}</h3>
              <p>{body}</p>
            </motion.article>
          )
        })}
      </div>
    </div>
  </section>
)

const CTA = ({ onGetStarted }: LandingProps) => (
  <section className="cta-band">
    <div className="launch-shell">
      <div className="cta-inner">
        <div>
          <div className="section-kicker">Launch-ready positioning</div>
          <h2 className="section-title">Stop paying prompt boxes to guess. Put a build system to work.</h2>
          <p className="section-copy">
            Import an existing project or start a brand new one from scratch. Then attach context,
            set a budget, pick your models, and watch the agents build with controls you can actually inspect.
          </p>
        </div>
        <button className="launch-btn launch-btn-primary" onClick={() => onGetStarted('register', 'pro')}>
          Start the build <ArrowRight width={18} height={18} />
        </button>
      </div>
    </div>
  </section>
)

const Footer = () => (
  <footer className="footer">
    <div className="launch-shell footer-inner">
      <Brand />
      <div>
        <a href="/?legal=terms">Terms</a>
        <a href="/?legal=privacy">Privacy</a>
        <a href="/?legal=billing">Billing</a>
        <a href="/?help=1">Help</a>
      </div>
    </div>
  </footer>
)

const Landing: React.FC<LandingProps> = ({ onGetStarted }) => (
  <div className="landing-v3">
    <style>{launchCss}</style>
    <Nav onGetStarted={onGetStarted} />
    <Hero onGetStarted={onGetStarted} />
    <Pillars />
    <Surfaces />
    <Agents />
    <ModelRouting />
    <Comparison />
    <Pricing onGetStarted={onGetStarted} />
    <Docs />
    <CTA onGetStarted={onGetStarted} />
    <Footer />
  </div>
)

export { Landing as LandingPage }
export default Landing
