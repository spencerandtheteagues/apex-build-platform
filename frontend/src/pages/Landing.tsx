// APEX.BUILD — Landing Page
// Real-time cost transparency + multi-agent IDE

import React, { useState, useEffect, useRef, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  ArrowRight, Eye, Download, Zap, Shield, Globe, Package,
  Check, Terminal, Bot, DollarSign, TrendingDown, Activity,
  CreditCard, BarChart3, Sparkles, AlertCircle,
} from 'lucide-react'

// ─── Design Tokens ────────────────────────────────────────────────────────────

const C = {
  bg:          '#000000',
  surface:     'rgba(255,255,255,0.03)',
  surfaceHov:  'rgba(255,255,255,0.055)',
  border:      'rgba(255,0,51,0.18)',
  borderDim:   'rgba(255,255,255,0.07)',
  accent:      '#ff0033',
  accentDim:   'rgba(255,0,51,0.11)',
  accentGlow:  'rgba(255,0,51,0.28)',
  green:       '#34d399',
  greenDim:    'rgba(52,211,153,0.10)',
  greenBorder: 'rgba(52,211,153,0.22)',
  text:        '#f0f0f0',
  textSub:     '#9ca3af',
  textMuted:   'rgba(255,255,255,0.32)',
  white:       '#ffffff',
}

const fontHero = '"Orbitron", "Rajdhani", sans-serif'
const fontBody = '"Inter", "Segoe UI", system-ui, sans-serif'
const fontMono = '"Fira Code", "Consolas", monospace'

// ─── Animation helpers ────────────────────────────────────────────────────────

const fadeUp = {
  initial:     { opacity: 0, y: 26 },
  whileInView: { opacity: 1, y: 0 },
  viewport:    { once: true, margin: '-50px' },
  transition:  { duration: 0.52, ease: [0.22, 1, 0.36, 1] },
}

const stagger = (i: number) => ({
  ...fadeUp,
  transition: { ...fadeUp.transition, delay: i * 0.09 },
})

// ─── Sub-components: layout ───────────────────────────────────────────────────

const SectionLabel: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div style={{
    display: 'inline-flex', alignItems: 'center', gap: 7,
    background: C.accentDim, border: `1px solid ${C.border}`,
    color: C.accent, borderRadius: 100,
    padding: '5px 14px', fontSize: '0.7rem',
    fontFamily: fontBody, fontWeight: 700,
    letterSpacing: '0.09em', textTransform: 'uppercase',
    marginBottom: 18,
  }}>
    <Sparkles size={11} /> {children}
  </div>
)

const SectionTitle: React.FC<{ children: React.ReactNode; style?: React.CSSProperties }> = ({ children, style }) => (
  <h2 style={{
    fontFamily: fontHero, fontWeight: 900,
    fontSize: 'clamp(1.9rem, 3.8vw, 2.7rem)',
    color: C.text, lineHeight: 1.14, margin: 0,
    ...style,
  }}>
    {children}
  </h2>
)

const SectionSub: React.FC<{ children: React.ReactNode; center?: boolean }> = ({ children, center }) => (
  <p style={{
    fontFamily: fontBody, fontSize: 'clamp(0.93rem, 1.4vw, 1.08rem)',
    color: C.textSub, lineHeight: 1.72,
    maxWidth: 620, margin: center ? '14px auto 0' : '14px 0 0',
    textAlign: center ? 'center' : 'left',
  }}>
    {children}
  </p>
)

// ─── Agents ───────────────────────────────────────────────────────────────────

const AGENTS = [
  { name: 'Architect',  color: '#a78bfa', bg: 'rgba(167,139,250,0.08)', border: 'rgba(167,139,250,0.22)', icon: '⬡',
    role: 'Plans structure, database schema, and API contracts before a line of code is written.' },
  { name: 'Backend',    color: '#34d399', bg: 'rgba(52,211,153,0.08)',  border: 'rgba(52,211,153,0.22)',  icon: '⚙',
    role: 'Generates REST APIs, authentication, and database layers in Go, Python, or Node.js.' },
  { name: 'Frontend',   color: '#60a5fa', bg: 'rgba(96,165,250,0.08)',  border: 'rgba(96,165,250,0.22)',  icon: '◈',
    role: 'Builds React, Next.js, or Vue UIs with Tailwind — auto-wired to the backend.' },
  { name: 'Reviewer',   color: '#fbbf24', bg: 'rgba(251,191,36,0.08)',  border: 'rgba(251,191,36,0.22)',  icon: '◉',
    role: 'Validates quality, enforces correct ports and CORS config, catches integration bugs.' },
  { name: 'Solver',     color: '#f87171', bg: 'rgba(248,113,113,0.08)', border: 'rgba(248,113,113,0.22)', icon: '✦',
    role: 'Automatically repairs build errors, dependency issues, and type failures.' },
]

// ─── Providers ────────────────────────────────────────────────────────────────

const PROVIDERS = [
  { name: 'Claude',  sub: 'Anthropic',     color: '#D97757', costPer1k: '$0.003' },
  { name: 'GPT-4o',  sub: 'OpenAI',        color: '#10A37F', costPer1k: '$0.005' },
  { name: 'Gemini',  sub: 'Google',        color: '#4285F4', costPer1k: '$0.001' },
  { name: 'Grok',    sub: 'xAI',           color: '#ffffff', costPer1k: '$0.002' },
  { name: 'Ollama',  sub: 'Local / Free',  color: '#7C3AED', costPer1k: '$0.000' },
]

// ─── Tech stacks ──────────────────────────────────────────────────────────────

const STACKS = [
  { label: 'React',      color: '#61DAFB', cat: 'Frontend' },
  { label: 'Next.js',    color: '#e5e5e5', cat: 'Frontend' },
  { label: 'Vue',        color: '#42D392', cat: 'Frontend' },
  { label: 'Tailwind',   color: '#38BDF8', cat: 'Frontend' },
  { label: 'Express',    color: '#e5e5e5', cat: 'Backend'  },
  { label: 'Go',         color: '#00ACD7', cat: 'Backend'  },
  { label: 'FastAPI',    color: '#009688', cat: 'Backend'  },
  { label: 'Django',     color: '#44B78B', cat: 'Backend'  },
  { label: 'PostgreSQL', color: '#336791', cat: 'Database' },
  { label: 'MongoDB',    color: '#47A248', cat: 'Database' },
  { label: 'Redis',      color: '#DC382D', cat: 'Database' },
]

// ─── Live Cost Ticker Demo ────────────────────────────────────────────────────

const AGENT_COSTS = [
  { name: 'Architect', color: '#a78bfa', tokens: 2840, cost: 0.0085 },
  { name: 'Backend',   color: '#34d399', tokens: 5120, cost: 0.0154 },
  { name: 'Frontend',  color: '#60a5fa', tokens: 4890, cost: 0.0147 },
  { name: 'Reviewer',  color: '#fbbf24', tokens: 1960, cost: 0.0059 },
  { name: 'Solver',    color: '#f87171', tokens: 820,  cost: 0.0025 },
]

const CostTickerDemo: React.FC = () => {
  const [running, setRunning] = useState(false)
  const [visibleAgents, setVisibleAgents] = useState(0)
  const [agentProgress, setAgentProgress] = useState<number[]>([0, 0, 0, 0, 0])
  const [totalCost, setTotalCost] = useState(0)
  const [done, setDone] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  const startDemo = useCallback(() => {
    setRunning(true)
    setVisibleAgents(0)
    setAgentProgress([0, 0, 0, 0, 0])
    setTotalCost(0)
    setDone(false)
  }, [])

  useEffect(() => {
    const obs = new IntersectionObserver(([e]) => { if (e.isIntersecting) startDemo() }, { threshold: 0.3 })
    if (ref.current) obs.observe(ref.current)
    return () => obs.disconnect()
  }, [startDemo])

  useEffect(() => {
    if (!running) return
    let agentIdx = 0
    const runAgent = () => {
      if (agentIdx >= AGENT_COSTS.length) { setDone(true); setRunning(false); return }
      const idx = agentIdx
      setVisibleAgents(idx + 1)
      const target = AGENT_COSTS[idx].cost
      let current = 0
      const steps = 40
      const iv = setInterval(() => {
        current += target / steps
        if (current >= target) { current = target; clearInterval(iv); setTimeout(runAgent, 300) }
        setAgentProgress(prev => { const n = [...prev]; n[idx] = current; return n })
        setTotalCost(prev => {
          const sum = AGENT_COSTS.slice(0, idx).reduce((a, b) => a + b.cost, 0)
          return sum + current
        })
      }, 28)
    }
    const t = setTimeout(runAgent, 400)
    return () => clearTimeout(t)
  }, [running])

  useEffect(() => {
    if (!done) return
    const t = setTimeout(() => startDemo(), 4500)
    return () => clearTimeout(t)
  }, [done, startDemo])

  const totalTokens = AGENT_COSTS.slice(0, visibleAgents).reduce((a, b) => a + b.tokens, 0)

  return (
    <div ref={ref} style={{
      background: 'rgba(0,0,0,0.85)',
      border: `1px solid ${C.border}`,
      borderRadius: 14,
      overflow: 'hidden',
      boxShadow: '0 0 60px rgba(255,0,51,0.06)',
    }}>
      {/* Header */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '14px 20px',
        background: 'rgba(255,255,255,0.03)',
        borderBottom: '1px solid rgba(255,255,255,0.06)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Activity size={14} style={{ color: running ? C.accent : C.green }} />
          <span style={{ fontFamily: fontMono, fontSize: '0.72rem', color: C.textMuted }}>
            real-time cost monitor
          </span>
          {running && (
            <span style={{
              width: 6, height: 6, borderRadius: '50%',
              background: C.accent,
              display: 'inline-block',
              animation: 'pulse 1s ease-in-out infinite',
            }} />
          )}
        </div>
        <div style={{
          fontFamily: fontMono, fontSize: '1.1rem', fontWeight: 700,
          color: C.green,
          textShadow: `0 0 16px ${C.green}`,
          minWidth: 80, textAlign: 'right',
        }}>
          ${totalCost.toFixed(4)}
        </div>
      </div>

      {/* Agent rows */}
      <div style={{ padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: 12 }}>
        {AGENT_COSTS.map((agent, i) => {
          const progress = agentProgress[i]
          const pct = (progress / agent.cost) * 100
          const isActive = i === visibleAgents - 1 && running && progress < agent.cost
          const isComplete = progress >= agent.cost - 0.00001
          return (
            <div key={agent.name} style={{
              opacity: i < visibleAgents ? 1 : 0.2,
              transition: 'opacity 0.4s',
            }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 6 }}>
                <span style={{
                  color: agent.color, fontSize: '0.75rem', fontWeight: 700,
                  fontFamily: fontBody, minWidth: 72,
                }}>
                  {agent.name}
                </span>
                <span style={{ color: C.textMuted, fontSize: '0.7rem', fontFamily: fontMono, marginLeft: 'auto' }}>
                  {i < visibleAgents ? `${agent.tokens.toLocaleString()} tokens` : '—'}
                </span>
                <span style={{
                  color: isComplete ? C.green : (isActive ? agent.color : C.textMuted),
                  fontSize: '0.78rem', fontFamily: fontMono,
                  minWidth: 56, textAlign: 'right', fontWeight: 600,
                }}>
                  {i < visibleAgents ? `$${progress.toFixed(4)}` : '—'}
                </span>
              </div>
              <div style={{
                height: 3, background: 'rgba(255,255,255,0.06)',
                borderRadius: 2, overflow: 'hidden',
              }}>
                <div style={{
                  height: '100%', width: `${pct}%`,
                  background: isComplete
                    ? `linear-gradient(90deg, ${agent.color}, ${agent.color}99)`
                    : `linear-gradient(90deg, ${agent.color}99, ${agent.color})`,
                  borderRadius: 2,
                  transition: 'width 0.04s linear',
                  boxShadow: isActive ? `0 0 8px ${agent.color}` : 'none',
                }} />
              </div>
            </div>
          )
        })}
      </div>

      {/* Footer totals */}
      <div style={{
        borderTop: '1px solid rgba(255,255,255,0.06)',
        padding: '14px 20px',
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        flexWrap: 'wrap', gap: 12,
      }}>
        <div style={{ display: 'flex', gap: 24 }}>
          <div>
            <div style={{ fontSize: '0.68rem', color: C.textMuted, fontFamily: fontBody, marginBottom: 2 }}>TOTAL TOKENS</div>
            <div style={{ fontFamily: fontMono, fontSize: '0.9rem', color: C.text }}>{totalTokens.toLocaleString()}</div>
          </div>
          <div>
            <div style={{ fontSize: '0.68rem', color: C.textMuted, fontFamily: fontBody, marginBottom: 2 }}>MODEL</div>
            <div style={{ fontFamily: fontMono, fontSize: '0.9rem', color: C.text }}>claude-sonnet</div>
          </div>
        </div>
        {done && (
          <div style={{
            padding: '6px 14px',
            background: C.greenDim, border: `1px solid ${C.greenBorder}`,
            borderRadius: 6, color: C.green,
            fontFamily: fontBody, fontSize: '0.8rem', fontWeight: 600,
          }}>
            ✓ Total: ${AGENT_COSTS.reduce((a, b) => a + b.cost, 0).toFixed(4)} for full-stack build
          </div>
        )}
      </div>
    </div>
  )
}

// ─── Hero terminal demo ───────────────────────────────────────────────────────

const DEMO_PROMPT = 'Build a task manager with React, Express, and PostgreSQL'

const HeroDemo: React.FC = () => {
  const [phase, setPhase] = useState<'idle' | 'typing' | 'building' | 'done'>('idle')
  const [typed, setTyped] = useState('')
  const [activeAgent, setActiveAgent] = useState<number>(-1)
  const [doneAgents, setDoneAgents] = useState<number[]>([])
  const ref = useRef<HTMLDivElement>(null)

  const restart = useCallback(() => {
    setPhase('idle'); setTyped(''); setActiveAgent(-1); setDoneAgents([])
    setTimeout(() => setPhase('typing'), 200)
  }, [])

  useEffect(() => {
    const obs = new IntersectionObserver(([e]) => { if (e.isIntersecting) setTimeout(() => setPhase('typing'), 700) }, { threshold: 0.4 })
    if (ref.current) obs.observe(ref.current)
    return () => obs.disconnect()
  }, [])

  // Typing
  useEffect(() => {
    if (phase !== 'typing') return
    let i = 0
    const iv = setInterval(() => {
      i++; setTyped(DEMO_PROMPT.slice(0, i))
      if (i >= DEMO_PROMPT.length) { clearInterval(iv); setTimeout(() => setPhase('building'), 500) }
    }, 28)
    return () => clearInterval(iv)
  }, [phase])

  // Agent sequence
  useEffect(() => {
    if (phase !== 'building') return
    let idx = 0
    const next = () => {
      if (idx >= AGENTS.length) { setActiveAgent(-1); setTimeout(() => setPhase('done'), 300); return }
      setActiveAgent(idx)
      setTimeout(() => { setDoneAgents(p => [...p, idx]); idx++; setTimeout(next, 180) }, 700 + Math.random() * 350)
    }
    next()
  }, [phase])

  useEffect(() => {
    if (phase !== 'done') return
    const t = setTimeout(restart, 5000)
    return () => clearTimeout(t)
  }, [phase, restart])

  return (
    <div ref={ref} style={{
      background: 'rgba(0,0,0,0.9)', border: `1px solid ${C.border}`,
      borderRadius: 12, overflow: 'hidden',
      boxShadow: '0 0 50px rgba(255,0,51,0.05)',
    }}>
      {/* Traffic lights */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 6,
        padding: '10px 14px',
        background: 'rgba(255,255,255,0.02)',
        borderBottom: '1px solid rgba(255,255,255,0.05)',
      }}>
        {['#ff5f57','#febc2e','#28c840'].map(c => (
          <div key={c} style={{ width: 9, height: 9, borderRadius: '50%', background: c }} />
        ))}
        <span style={{ marginLeft: 8, color: C.textMuted, fontSize: '0.68rem', fontFamily: fontMono }}>
          apex.build — agent terminal
        </span>
      </div>

      <div style={{ padding: '18px 18px 20px', fontFamily: fontMono, fontSize: '0.78rem', minHeight: 220 }}>
        <div style={{ color: C.textMuted, marginBottom: 8 }}>
          <span style={{ color: C.accent }}>›</span> describe your app
        </div>
        {typed && (
          <div style={{ color: C.text, marginBottom: 18, lineHeight: 1.6 }}>
            "{typed}{phase === 'typing' && <span style={{ animation: 'blink 1s step-start infinite' }}>▊</span>}"
          </div>
        )}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 7 }}>
          {(phase === 'building' || phase === 'done') && AGENTS.map((a, i) => {
            const done = doneAgents.includes(i)
            const active = activeAgent === i
            return (
              <div key={a.name} style={{ display: 'flex', alignItems: 'center', gap: 10, opacity: (done || active) ? 1 : 0.3, transition: 'opacity 0.3s' }}>
                <span style={{ color: a.color }}>{done ? '✓' : active ? '⟳' : '○'}</span>
                <span style={{ color: a.color, minWidth: 76, fontWeight: 600 }}>{a.name}</span>
                <span style={{ color: C.textMuted, fontSize: '0.7rem' }}>{done ? 'done' : active ? 'running...' : 'queued'}</span>
              </div>
            )
          })}
          {phase === 'done' && (
            <div style={{
              marginTop: 10, padding: '9px 12px',
              background: 'rgba(52,211,153,0.08)', border: '1px solid rgba(52,211,153,0.25)',
              borderRadius: 7, color: C.green, display: 'flex', alignItems: 'center',
              gap: 10, animation: 'fadeIn 0.4s ease',
            }}>
              <span>✓</span>
              <span style={{ fontWeight: 600 }}>Build complete</span>
              <span style={{ marginLeft: 'auto', color: 'rgba(52,211,153,0.55)', fontSize: '0.7rem' }}>14 files · $0.047</span>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ─── Navbar ───────────────────────────────────────────────────────────────────

interface LandingProps { onGetStarted: (mode?: 'login' | 'register') => void }

const Navbar: React.FC<LandingProps> = ({ onGetStarted }) => {
  const [scrolled, setScrolled] = useState(false)
  useEffect(() => {
    const h = () => setScrolled(window.scrollY > 24)
    window.addEventListener('scroll', h, { passive: true })
    return () => window.removeEventListener('scroll', h)
  }, [])
  const go = (id: string) => document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' })

  return (
    <nav style={{
      position: 'fixed', top: 0, left: 0, right: 0, zIndex: 100,
      height: 62,
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      padding: '0 clamp(1rem, 5vw, 3rem)',
      background: scrolled ? 'rgba(0,0,0,0.92)' : 'transparent',
      backdropFilter: scrolled ? 'blur(16px)' : 'none',
      borderBottom: scrolled ? `1px solid ${C.border}` : '1px solid transparent',
      transition: 'all 0.3s ease',
    }}>
      <img src="/logo.png" alt="Apex.Build" style={{
        height: 32, objectFit: 'contain',
        filter: 'drop-shadow(0 0 8px rgba(255,0,51,0.45))',
      }} />
      <div style={{ display: 'flex', gap: 28, alignItems: 'center' }}>
        {[['cost','Pricing / Cost'],['agents','AI Team'],['how','How It Works'],['stacks','Stacks']].map(([id,l]) => (
          <button key={id} onClick={() => go(id)} style={{
            background: 'none', border: 'none', color: C.textSub,
            cursor: 'pointer', fontSize: '0.83rem', fontFamily: fontBody,
            letterSpacing: '0.03em', transition: 'color 0.2s',
          }}
          onMouseEnter={e => (e.target as HTMLElement).style.color = C.white}
          onMouseLeave={e => (e.target as HTMLElement).style.color = C.textSub}>
            {l}
          </button>
        ))}
      </div>
      <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
        <button onClick={() => onGetStarted('login')} style={{
          background: 'none', border: `1px solid ${C.borderDim}`,
          color: 'rgba(255,255,255,0.7)', padding: '7px 18px',
          borderRadius: 6, cursor: 'pointer', fontSize: '0.83rem',
          fontFamily: fontBody, transition: 'all 0.2s',
        }}
        onMouseEnter={e => { const el = e.target as HTMLElement; el.style.borderColor = 'rgba(255,255,255,0.25)'; el.style.color = C.white }}
        onMouseLeave={e => { const el = e.target as HTMLElement; el.style.borderColor = C.borderDim; el.style.color = 'rgba(255,255,255,0.7)' }}>
          Sign In
        </button>
        <button onClick={() => onGetStarted('register')} style={{
          background: 'linear-gradient(135deg,#ff0033,#cc0029)',
          border: 'none', color: C.white,
          padding: '7px 20px', borderRadius: 6,
          cursor: 'pointer', fontSize: '0.83rem',
          fontFamily: fontBody, fontWeight: 700,
          boxShadow: '0 0 18px rgba(255,0,51,0.28)',
          transition: 'all 0.2s',
        }}
        onMouseEnter={e => (e.target as HTMLElement).style.boxShadow = '0 0 28px rgba(255,0,51,0.5)'}
        onMouseLeave={e => (e.target as HTMLElement).style.boxShadow = '0 0 18px rgba(255,0,51,0.28)'}>
          Get Started Free
        </button>
      </div>
    </nav>
  )
}

// ─── Hero ─────────────────────────────────────────────────────────────────────

const Hero: React.FC<LandingProps> = ({ onGetStarted }) => (
  <section style={{
    minHeight: '100vh',
    display: 'flex', flexDirection: 'column',
    alignItems: 'center', justifyContent: 'center',
    padding: 'clamp(80px,12vw,120px) clamp(1rem,8vw,5rem) clamp(60px,8vw,80px)',
    textAlign: 'center',
    backgroundImage: [
      'radial-gradient(ellipse 80% 55% at 50% -5%, rgba(255,0,51,0.09) 0%, transparent 65%)',
      'linear-gradient(rgba(255,0,51,0.035) 1px, transparent 1px)',
      'linear-gradient(90deg, rgba(255,0,51,0.035) 1px, transparent 1px)',
    ].join(','),
    backgroundSize: 'auto, 52px 52px, 52px 52px',
  }}>
    <motion.div initial={{ opacity:0, scale:0.88 }} animate={{ opacity:1, scale:1 }}
      transition={{ duration:0.7, ease:[0.22,1,0.36,1] }} style={{ marginBottom: 28 }}>
      <img src="/logo.png" alt="Apex.Build" style={{
        width: 'clamp(150px,20vw,240px)', objectFit: 'contain',
        filter: 'drop-shadow(0 0 40px rgba(255,0,51,0.45)) drop-shadow(0 0 80px rgba(255,0,51,0.18))',
      }} />
    </motion.div>

    <motion.div initial={{ opacity:0,y:14 }} animate={{ opacity:1,y:0 }} transition={{ delay:0.18, duration:0.45 }}>
      <SectionLabel>Multi-Agent Cloud IDE</SectionLabel>
    </motion.div>

    <motion.h1 initial={{ opacity:0,y:22 }} animate={{ opacity:1,y:0 }}
      transition={{ delay:0.28, duration:0.6, ease:[0.22,1,0.36,1] }}
      style={{
        fontFamily: fontHero, fontWeight: 900,
        fontSize: 'clamp(2.4rem,5.8vw,5.2rem)',
        lineHeight: 1.08, margin: '6px 0 0',
        color: C.white, maxWidth: 860,
      }}>
      Not one AI.{' '}
      <span style={{
        background: 'linear-gradient(135deg,#ff0033,#ff6b6b)',
        WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent',
      }}>
        An entire dev team.
      </span>
    </motion.h1>

    <motion.p initial={{ opacity:0,y:18 }} animate={{ opacity:1,y:0 }}
      transition={{ delay:0.42, duration:0.55 }}
      style={{
        fontFamily: fontBody, fontSize: 'clamp(0.98rem,1.9vw,1.2rem)',
        color: C.textSub, lineHeight: 1.72,
        maxWidth: 660, margin: '18px auto 0',
      }}>
      Five specialized AI agents build your complete full-stack application —
      while you watch <strong style={{ color: C.text }}>exactly what every token costs</strong>,
      in real time, down to the cent.
    </motion.p>

    {/* Cost highlight stat */}
    <motion.div initial={{ opacity:0,y:14 }} animate={{ opacity:1,y:0 }}
      transition={{ delay:0.54, duration:0.5 }}
      style={{
        display: 'flex', gap: 28, margin: '30px 0 0',
        flexWrap: 'wrap', justifyContent: 'center',
      }}>
      {[
        { val: '$19',    label: 'per month' },
        { val: '5',      label: 'AI agents per build' },
        { val: '6',      label: 'AI providers supported' },
      ].map(s => (
        <div key={s.label} style={{ textAlign: 'center' }}>
          <div style={{ fontFamily: fontHero, fontSize: '1.6rem', fontWeight: 900, color: C.green }}>
            {s.val}
          </div>
          <div style={{ fontFamily: fontBody, fontSize: '0.72rem', color: C.textMuted,
            letterSpacing: '0.08em', textTransform: 'uppercase' }}>
            {s.label}
          </div>
        </div>
      ))}
    </motion.div>

    <motion.div initial={{ opacity:0,y:14 }} animate={{ opacity:1,y:0 }}
      transition={{ delay:0.62, duration:0.5 }}
      style={{ display:'flex', gap:14, marginTop:32, flexWrap:'wrap', justifyContent:'center' }}>
      <button onClick={() => onGetStarted('register')} style={{
        background: 'linear-gradient(135deg,#ff0033,#cc0029)',
        border: 'none', color: C.white,
        padding: '13px 30px', borderRadius: 8,
        cursor: 'pointer', fontSize: '0.97rem',
        fontFamily: fontBody, fontWeight: 700,
        boxShadow: '0 0 28px rgba(255,0,51,0.35), 0 4px 16px rgba(0,0,0,0.5)',
        display: 'flex', alignItems: 'center', gap: 8,
        transition: 'all 0.2s',
      }}
      onMouseEnter={e => { (e.target as HTMLElement).style.transform='translateY(-1px)'; (e.target as HTMLElement).style.boxShadow='0 0 44px rgba(255,0,51,0.55), 0 6px 20px rgba(0,0,0,0.5)' }}
      onMouseLeave={e => { (e.target as HTMLElement).style.transform='translateY(0)'; (e.target as HTMLElement).style.boxShadow='0 0 28px rgba(255,0,51,0.35), 0 4px 16px rgba(0,0,0,0.5)' }}>
        Start Building Free <ArrowRight size={16} />
      </button>
      <button onClick={() => document.getElementById('cost')?.scrollIntoView({ behavior:'smooth' })} style={{
        background: C.surface, border:`1px solid ${C.borderDim}`,
        color: 'rgba(255,255,255,0.7)',
        padding: '13px 26px', borderRadius: 8,
        cursor: 'pointer', fontSize: '0.97rem',
        fontFamily: fontBody, transition: 'all 0.2s',
      }}
      onMouseEnter={e => { const el = e.target as HTMLElement; el.style.background=C.surfaceHov; el.style.color=C.white }}
      onMouseLeave={e => { const el = e.target as HTMLElement; el.style.background=C.surface; el.style.color='rgba(255,255,255,0.7)' }}>
        See Live Cost Tracker ↓
      </button>
    </motion.div>

    <motion.div initial={{ opacity:0,y:28 }} animate={{ opacity:1,y:0 }}
      transition={{ delay:0.75, duration:0.65, ease:[0.22,1,0.36,1] }}
      style={{ width:'100%', maxWidth:580, marginTop:52 }}>
      <HeroDemo />
    </motion.div>
  </section>
)

// ─── Cost Transparency Section ────────────────────────────────────────────────

const CostSection: React.FC<LandingProps> = ({ onGetStarted }) => (
  <section id="cost" style={{
    padding: 'clamp(60px,8vw,100px) clamp(1rem,8vw,6rem)',
    borderTop: `1px solid ${C.borderDim}`,
    background: 'linear-gradient(180deg, rgba(52,211,153,0.03) 0%, transparent 60%)',
  }}>
    <div style={{
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))',
      gap: 'clamp(40px,6vw,80px)',
      maxWidth: 1100, margin: '0 auto',
      alignItems: 'center',
    }}>
      {/* Left: copy */}
      <div>
        <motion.div {...fadeUp}>
          <SectionLabel>Real-Time Cost Transparency</SectionLabel>
          <SectionTitle style={{ color: C.green }}>
            Know exactly what every task costs you.
          </SectionTitle>
          <SectionSub>
            Watch your spend update live — by agent, by task, by token. Every API call
            is tracked and shown as it happens. No black-box subscriptions hiding what
            you're actually using. Just a transparent cost ticker, always visible.
          </SectionSub>
        </motion.div>

        <motion.div {...stagger(1)} style={{ marginTop: 28, display: 'flex', flexDirection: 'column', gap: 14 }}>
          {[
            { icon: DollarSign,   title: 'Zero Markup',       desc: 'You pay exactly what the AI provider charges. We add $0.00 on top.' },
            { icon: Activity,     title: 'Live Token Counter', desc: 'Watch tokens and dollars tick up per agent as the build runs.' },
            { icon: BarChart3,    title: 'Full Cost Breakdown',desc: 'See cost by agent, task, provider, and model in one dashboard.' },
            { icon: TrendingDown, title: 'Use Cheaper Models', desc: 'Assign budget models to routine tasks, premium models where it matters.' },
          ].map((item, i) => {
            const Icon = item.icon
            return (
              <motion.div key={item.title} {...stagger(i + 2)} style={{
                display: 'flex', gap: 14, alignItems: 'flex-start',
              }}>
                <div style={{
                  flexShrink: 0, width: 36, height: 36,
                  background: C.greenDim, border: `1px solid ${C.greenBorder}`,
                  borderRadius: 8,
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  color: C.green,
                }}>
                  <Icon size={17} />
                </div>
                <div>
                  <div style={{ fontFamily: fontBody, fontWeight: 700, color: C.text, fontSize: '0.9rem', marginBottom: 3 }}>
                    {item.title}
                  </div>
                  <div style={{ fontFamily: fontBody, fontSize: '0.83rem', color: C.textSub, lineHeight: 1.65 }}>
                    {item.desc}
                  </div>
                </div>
              </motion.div>
            )
          })}
        </motion.div>
      </div>

      {/* Right: live demo */}
      <motion.div {...stagger(1)}>
        <CostTickerDemo />

        {/* Comparison callout */}
        <div style={{
          marginTop: 16,
          padding: '14px 18px',
          background: C.accentDim, border: `1px solid ${C.border}`,
          borderRadius: 10,
          display: 'flex', alignItems: 'flex-start', gap: 12,
        }}>
          <AlertCircle size={16} style={{ color: C.accent, flexShrink: 0, marginTop: 2 }} />
          <p style={{ fontFamily: fontBody, fontSize: '0.8rem', color: C.textSub, lineHeight: 1.6, margin: 0 }}>
            <strong style={{ color: C.text }}>Compare:</strong> Replit charges $25/mo and
            hides what each action costs. Apex.Build is{' '}
            <strong style={{ color: C.green }}>$19/mo</strong> — and shows you a live
            cost breakdown for every build, every agent, every token.
          </p>
        </div>
      </motion.div>
    </div>
  </section>
)

// ─── Agent Section ────────────────────────────────────────────────────────────

const AgentSection: React.FC = () => (
  <section id="agents" style={{
    padding: 'clamp(60px,8vw,100px) clamp(1rem,8vw,6rem)',
    borderTop: `1px solid ${C.borderDim}`,
  }}>
    <motion.div {...fadeUp} style={{ textAlign:'center', marginBottom:52 }}>
      <SectionLabel>Your AI Development Team</SectionLabel>
      <SectionTitle style={{ textAlign:'center' }}>Five specialists. One complete app.</SectionTitle>
      <SectionSub center>
        Unlike AI assistants that give you code snippets, Apex.Build deploys a coordinated team.
        Each agent has a dedicated role — and they share architectural context so the whole
        application works together.
      </SectionSub>
    </motion.div>

    <div style={{
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fit, minmax(210px, 1fr))',
      gap: 18, maxWidth: 1200, margin: '0 auto',
    }}>
      {AGENTS.map((agent, i) => (
        <motion.div key={agent.name} {...stagger(i)}
          style={{
            background: agent.bg, border: `1px solid ${agent.border}`,
            borderRadius: 13, padding: '26px 22px',
          }}
          whileHover={{ y: -4, boxShadow: `0 12px 30px ${agent.bg}` }}>
          <div style={{
            width: 42, height: 42,
            background: 'rgba(0,0,0,0.35)', border: `1px solid ${agent.border}`,
            borderRadius: 9, display: 'flex', alignItems: 'center',
            justifyContent: 'center', fontSize: '1.2rem',
            color: agent.color, marginBottom: 14,
          }}>
            {agent.icon}
          </div>
          <h3 style={{
            fontFamily: fontHero, fontWeight: 700,
            color: agent.color, fontSize: '0.88rem',
            letterSpacing: '0.07em', margin: '0 0 9px',
          }}>
            {agent.name.toUpperCase()}
          </h3>
          <p style={{
            fontFamily: fontBody, fontSize: '0.83rem',
            color: C.textSub, lineHeight: 1.65, margin: 0,
          }}>
            {agent.role}
          </p>
        </motion.div>
      ))}
    </div>
  </section>
)

// ─── How It Works ─────────────────────────────────────────────────────────────

const STEPS = [
  { n:'01', icon: Terminal, title:'Describe Your App',
    desc:'Type what you want in plain English. Choose your tech stack, power level, and which AI model handles each agent role.' },
  { n:'02', icon: Bot,      title:'Your Team Builds It',
    desc:'Five agents plan, code, review, and repair in parallel. Watch the cost ticker as tokens flow — live.' },
  { n:'03', icon: Download, title:'Preview and Download',
    desc:'See your app running in-browser. Download complete, runnable code with README, .env.example, and all dependencies installed.' },
]

const HowItWorks: React.FC = () => (
  <section id="how" style={{
    padding: 'clamp(60px,8vw,100px) clamp(1rem,8vw,6rem)',
    borderTop: `1px solid ${C.borderDim}`,
    background: 'linear-gradient(180deg, rgba(255,0,51,0.025) 0%, transparent 100%)',
  }}>
    <motion.div {...fadeUp} style={{ textAlign:'center', marginBottom:52 }}>
      <SectionLabel>How It Works</SectionLabel>
      <SectionTitle style={{ textAlign:'center' }}>From idea to running app in minutes</SectionTitle>
    </motion.div>

    <div style={{
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fit, minmax(270px,1fr))',
      gap: 36, maxWidth: 1100, margin: '0 auto',
    }}>
      {STEPS.map((s, i) => {
        const Icon = s.icon
        return (
          <motion.div key={s.n} {...stagger(i)} style={{ display:'flex', flexDirection:'column', gap:14 }}>
            <div style={{
              fontSize: '3.2rem', fontFamily: fontHero, fontWeight: 900,
              color: C.accentDim, lineHeight: 1,
              WebkitTextStroke: `1px ${C.border}`,
            }}>
              {s.n}
            </div>
            <div style={{
              width: 46, height: 46,
              background: C.accentDim, border: `1px solid ${C.border}`,
              borderRadius: 9, display: 'flex', alignItems: 'center',
              justifyContent: 'center', color: C.accent,
            }}>
              <Icon size={21} />
            </div>
            <h3 style={{
              fontFamily: fontHero, fontWeight: 700,
              color: C.text, fontSize: '1.1rem',
              margin: 0, letterSpacing: '0.04em',
            }}>
              {s.title}
            </h3>
            <p style={{
              fontFamily: fontBody, fontSize: '0.875rem',
              color: C.textSub, lineHeight: 1.7, margin: 0,
            }}>
              {s.desc}
            </p>
          </motion.div>
        )
      })}
    </div>
  </section>
)

// ─── Tech stacks ──────────────────────────────────────────────────────────────

function hexToRgb(hex: string) {
  const r = parseInt(hex.slice(1,3),16)
  const g = parseInt(hex.slice(3,5),16)
  const b = parseInt(hex.slice(5,7),16)
  return `${r},${g},${b}`
}

const TechStacks: React.FC = () => (
  <section id="stacks" style={{
    padding: 'clamp(60px,8vw,100px) clamp(1rem,8vw,6rem)',
    borderTop: `1px solid ${C.borderDim}`,
  }}>
    <motion.div {...fadeUp} style={{ textAlign:'center', marginBottom:48 }}>
      <SectionLabel>Supported Tech Stacks</SectionLabel>
      <SectionTitle style={{ textAlign:'center' }}>Every stack. Every layer.</SectionTitle>
      <SectionSub center>
        Full-stack, API-only, or frontend — any combination. Mix React with Go,
        Next.js with FastAPI, Vue with Django. You pick, the team builds.
      </SectionSub>
    </motion.div>

    <div style={{ display:'flex', flexDirection:'column', gap:22, maxWidth:880, margin:'0 auto' }}>
      {['Frontend','Backend','Database'].map((cat, ci) => (
        <motion.div key={cat} {...stagger(ci)}>
          <div style={{
            fontSize:'0.68rem', fontFamily:fontBody, fontWeight:700,
            letterSpacing:'0.12em', textTransform:'uppercase',
            color: C.textMuted, marginBottom: 10,
          }}>{cat}</div>
          <div style={{ display:'flex', gap:9, flexWrap:'wrap' }}>
            {STACKS.filter(s => s.cat === cat).map(s => (
              <div key={s.label} style={{
                padding: '8px 16px',
                background: `rgba(${hexToRgb(s.color)},0.05)`,
                border: `1px solid rgba(${hexToRgb(s.color)},0.2)`,
                borderRadius: 7, color: s.color,
                fontFamily: fontBody, fontWeight: 600,
                fontSize: '0.85rem', letterSpacing: '0.02em',
              }}>
                {s.label}
              </div>
            ))}
          </div>
        </motion.div>
      ))}
    </div>
  </section>
)

// ─── Features ─────────────────────────────────────────────────────────────────

const FEATURES = [
  { icon: Eye,      title:'Live Browser Preview',       desc:'See your generated app running in-browser. Full-stack previews with backend proxied automatically.' },
  { icon: Zap,      title:'Auto-Wired APIs',             desc:'Frontend and backend auto-configured with matching ports and CORS. Zero manual wiring.' },
  { icon: Package,  title:'Dependencies Installed',      desc:'npm install, pip install, go mod — all done before preview. Your app runs, not just compiles.' },
  { icon: Shield,   title:'Compile Verification',        desc:'Syntax errors and broken imports caught and repaired automatically before you see results.' },
  { icon: Download, title:'Complete Runnable Download',  desc:'README with setup, .env.example, dependency files, and working code all in one zip.' },
  { icon: Globe,    title:'Any Model, Any Role',         desc:'Claude for architecture, Gemini for review, Ollama locally — assign any model to any agent.' },
]

const FeaturesSection: React.FC = () => (
  <section style={{
    padding: 'clamp(60px,8vw,100px) clamp(1rem,8vw,6rem)',
    borderTop: `1px solid ${C.borderDim}`,
    background: 'linear-gradient(180deg, rgba(255,0,51,0.025) 0%, transparent 100%)',
  }}>
    <motion.div {...fadeUp} style={{ textAlign:'center', marginBottom:52 }}>
      <SectionLabel>What Makes It Different</SectionLabel>
      <SectionTitle style={{ textAlign:'center' }}>Built right. Not just built.</SectionTitle>
      <SectionSub center>
        Other tools give you code. Apex.Build gives you a running application — with
        correct configuration, working APIs, installed dependencies, and documentation.
      </SectionSub>
    </motion.div>
    <div style={{
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fit,minmax(290px,1fr))',
      gap: 20, maxWidth: 1200, margin: '0 auto',
    }}>
      {FEATURES.map((f, i) => {
        const Icon = f.icon
        return (
          <motion.div key={f.title} {...stagger(i)}
            style={{ background:C.surface, border:`1px solid ${C.borderDim}`, borderRadius:12, padding:'26px' }}
            whileHover={{ borderColor:C.border, background:C.surfaceHov, y:-2 }}>
            <div style={{
              width:40, height:40,
              background:C.accentDim, border:`1px solid ${C.border}`,
              borderRadius:8, display:'flex', alignItems:'center',
              justifyContent:'center', color:C.accent, marginBottom:16,
            }}>
              <Icon size={19} />
            </div>
            <h3 style={{ fontFamily:fontHero, fontWeight:700, color:C.text, fontSize:'0.95rem', letterSpacing:'0.04em', margin:'0 0 9px' }}>
              {f.title}
            </h3>
            <p style={{ fontFamily:fontBody, fontSize:'0.84rem', color:C.textSub, lineHeight:1.7, margin:0 }}>
              {f.desc}
            </p>
          </motion.div>
        )
      })}
    </div>
  </section>
)

// ─── AI Providers + cost per 1k ───────────────────────────────────────────────

const ProvidersSection: React.FC = () => (
  <section style={{
    padding: 'clamp(60px,8vw,100px) clamp(1rem,8vw,6rem)',
    borderTop: `1px solid ${C.borderDim}`,
  }}>
    <motion.div {...fadeUp} style={{ textAlign:'center', marginBottom:44 }}>
      <SectionLabel>AI Providers</SectionLabel>
      <SectionTitle style={{ textAlign:'center' }}>Use any AI. See exactly what it costs.</SectionTitle>
      <SectionSub center>
        Every provider, every model — with live cost visibility for each one.
        Or bring your own API keys and route requests directly to the provider.
      </SectionSub>
    </motion.div>
    <div style={{
      display:'flex', flexWrap:'wrap', gap:14,
      maxWidth:860, margin:'0 auto', justifyContent:'center',
    }}>
      {PROVIDERS.map((p, i) => (
        <motion.div key={p.name} {...stagger(i)}
          style={{
            padding:'18px 26px', minWidth:152,
            background:C.surface, border:`1px solid ${C.borderDim}`,
            borderRadius:12, textAlign:'center',
          }}
          whileHover={{ borderColor:`${p.color}44`, y:-3 }}>
          <div style={{ fontFamily:fontHero, fontWeight:800, fontSize:'1.05rem', color:p.color, marginBottom:5, letterSpacing:'0.04em' }}>
            {p.name}
          </div>
          <div style={{ fontFamily:fontBody, fontSize:'0.72rem', color:C.textMuted, marginBottom:8 }}>
            {p.sub}
          </div>
          <div style={{
            fontFamily:fontMono, fontSize:'0.8rem',
            color: p.costPer1k === '$0.000' ? C.green : C.textSub,
            fontWeight:600,
          }}>
            {p.costPer1k === '$0.000' ? 'Free' : `${p.costPer1k} / 1k tokens`}
          </div>
        </motion.div>
      ))}
    </div>
    <motion.div {...fadeUp} style={{ textAlign:'center', marginTop:28 }}>
      <span style={{ display:'inline-flex', alignItems:'center', gap:7, fontFamily:fontBody, fontSize:'0.85rem', color:C.textSub }}>
        <Check size={15} style={{ color:C.accent }} />
        Bring your own API keys to route requests directly to the provider
      </span>
    </motion.div>
  </section>
)

// ─── Final CTA ────────────────────────────────────────────────────────────────

const CTASection: React.FC<LandingProps> = ({ onGetStarted }) => (
  <section style={{
    padding: 'clamp(80px,10vw,120px) clamp(1rem,8vw,6rem)',
    borderTop: `1px solid ${C.borderDim}`,
    background: 'radial-gradient(ellipse 70% 70% at 50% 50%, rgba(255,0,51,0.06) 0%, transparent 70%)',
    textAlign: 'center',
  }}>
    <motion.div {...fadeUp} style={{ maxWidth:680, margin:'0 auto', display:'flex', flexDirection:'column', alignItems:'center', gap:22 }}>
      <img src="/logo.png" alt="Apex.Build" style={{
        width:'clamp(90px,13vw,140px)', objectFit:'contain',
        filter:'drop-shadow(0 0 28px rgba(255,0,51,0.42))',
      }} />
      <SectionTitle style={{ textAlign:'center' }}>
        Your first app is{' '}
        <span style={{ background:'linear-gradient(135deg,#ff0033,#ff7777)', WebkitBackgroundClip:'text', WebkitTextFillColor:'transparent' }}>
          minutes away.
        </span>
      </SectionTitle>
      <p style={{ fontFamily:fontBody, fontSize:'1.05rem', color:C.textSub, lineHeight:1.72, margin:0 }}>
        Build complete, working applications with a coordinated AI team — and watch
        every cent of cost tick by in real time, with zero surprises.
      </p>
      <div style={{ display:'flex', flexDirection:'column', alignItems:'center', gap:10 }}>
        <button onClick={() => onGetStarted('register')} style={{
          background:'linear-gradient(135deg,#ff0033,#cc0029)',
          border:'none', color:C.white,
          padding:'15px 44px', borderRadius:9,
          cursor:'pointer', fontSize:'1.05rem',
          fontFamily:fontBody, fontWeight:700,
          boxShadow:'0 0 36px rgba(255,0,51,0.4), 0 4px 22px rgba(0,0,0,0.5)',
          display:'flex', alignItems:'center', gap:10,
          transition:'all 0.2s',
        }}
        onMouseEnter={e => { (e.target as HTMLElement).style.transform='translateY(-2px)'; (e.target as HTMLElement).style.boxShadow='0 0 56px rgba(255,0,51,0.6), 0 8px 30px rgba(0,0,0,0.5)' }}
        onMouseLeave={e => { (e.target as HTMLElement).style.transform='translateY(0)'; (e.target as HTMLElement).style.boxShadow='0 0 36px rgba(255,0,51,0.4), 0 4px 22px rgba(0,0,0,0.5)' }}>
          Create Free Account <ArrowRight size={17} />
        </button>
        <span style={{ fontFamily:fontBody, fontSize:'0.78rem', color:C.textMuted }}>$19/month · cancel any time</span>
      </div>
    </motion.div>
  </section>
)

// ─── Footer ───────────────────────────────────────────────────────────────────

const Footer: React.FC<LandingProps> = ({ onGetStarted }) => (
  <footer style={{
    borderTop:`1px solid ${C.borderDim}`,
    padding:'32px clamp(1rem,8vw,5rem)',
    display:'flex', alignItems:'center',
    justifyContent:'space-between', flexWrap:'wrap', gap:16,
  }}>
    <div style={{ display:'flex', alignItems:'center', gap:10 }}>
      <img src="/logo.png" alt="Apex.Build" style={{ height:26, objectFit:'contain', filter:'drop-shadow(0 0 5px rgba(255,0,51,0.32))' }} />
      <span style={{ fontFamily:fontBody, fontSize:'0.78rem', color:C.textMuted }}>© 2025 Apex.Build</span>
    </div>
    <div style={{ display:'flex', gap:22 }}>
      {[['Sign In','login'],['Get Started','register']].map(([l,m]) => (
        <button key={l} onClick={() => onGetStarted(m as any)} style={{
          background:'none', border:'none', color:C.textMuted,
          cursor:'pointer', fontSize:'0.82rem', fontFamily:fontBody,
          transition:'color 0.2s',
        }}
        onMouseEnter={e => (e.target as HTMLElement).style.color = C.textSub}
        onMouseLeave={e => (e.target as HTMLElement).style.color = C.textMuted}>
          {l}
        </button>
      ))}
    </div>
  </footer>
)

// ─── Keyframes ────────────────────────────────────────────────────────────────

const GlobalStyles: React.FC = () => (
  <style>{`
    @keyframes blink    { 0%,100%{opacity:1} 50%{opacity:0} }
    @keyframes fadeIn   { from{opacity:0} to{opacity:1} }
    @keyframes pulse    { 0%,100%{opacity:1} 50%{opacity:0.3} }
    html { scroll-behavior: smooth }
  `}</style>
)

// ─── Main export ──────────────────────────────────────────────────────────────

export const LandingPage: React.FC<LandingProps> = ({ onGetStarted }) => (
  <div style={{ background:C.bg, color:C.text, fontFamily:fontBody, minHeight:'100vh', overflowX:'hidden' }}>
    <GlobalStyles />
    <Navbar onGetStarted={onGetStarted} />
    <Hero onGetStarted={onGetStarted} />
    <CostSection onGetStarted={onGetStarted} />
    <AgentSection />
    <HowItWorks />
    <TechStacks />
    <FeaturesSection />
    <ProvidersSection />
    <CTASection onGetStarted={onGetStarted} />
    <Footer onGetStarted={onGetStarted} />
  </div>
)

export default LandingPage
