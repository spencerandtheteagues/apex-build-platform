// APEX-BUILD — Billing Settings
// Unified workspace billing surface with current production UI styling

import React, { useState, useEffect, useCallback } from 'react'
import {
  Zap, CreditCard, ExternalLink, Check, Loader2,
  AlertCircle, RefreshCw, Star, Crown, Users, Rocket, ShieldCheck, X,
} from 'lucide-react'
import apiService from '@/services/api'
import { BuyCreditsModal } from './BuyCreditsModal'

interface Plan {
  type: string
  name: string
  monthly_price_cents: number
  monthly_price_id: string
  monthly_credits_usd: number
  is_popular: boolean
  features: string[]
}

interface Subscription {
  plan_type: string
  plan_name: string
  status: string
  current_period_end: string
  cancel_at_period_end?: boolean
}

interface Invoice {
  id: string
  amount_paid: number
  currency: string
  status: string
  created: number
  hosted_invoice_url: string
  description: string
}

const PLAN_CONFIG: Record<string, {
  color: string
  glow: string
  border: string
  bg: string
  badge: string
  icon: React.ReactNode
  tagline: string
  bestFor: string
  features: string[]
}> = {
  free: {
    color: '#9ca3af',
    glow: 'rgba(156,163,175,0.25)',
    border: 'rgba(156,163,175,0.25)',
    bg: 'rgba(156,163,175,0.04)',
    badge: '',
    icon: <Star size={18} />,
    tagline: 'Static websites & UI mockups',
    bestFor: 'Landing pages, portfolios, and frontend prototypes.',
    features: ['$5 trial credits included', 'Static frontend builds', 'UI mockups & prototypes', 'No credit card required'],
  },
  builder: {
    color: '#3b82f6',
    glow: 'rgba(59,130,246,0.3)',
    border: 'rgba(59,130,246,0.4)',
    bg: 'rgba(59,130,246,0.06)',
    badge: '',
    icon: <Rocket size={18} />,
    tagline: 'Full-stack development unlocked',
    bestFor: 'Early-stage teams shipping their first real app.',
    features: ['Everything in Free', 'Full-stack backend + APIs', 'Database, auth & deployment', 'GitHub import/export', '$12/mo in managed AI credits'],
  },
  pro: {
    color: '#38bdf8',
    glow: 'rgba(56,189,248,0.35)',
    border: 'rgba(56,189,248,0.45)',
    bg: 'rgba(56,189,248,0.05)',
    badge: 'MOST POPULAR',
    icon: <ShieldCheck size={18} />,
    tagline: 'Serious shipping, every week',
    bestFor: 'Founders & operators shipping production app changes weekly.',
    features: ['Everything in Builder', 'Latest Ollama cloud models — high quality, lower cost', 'BYOK + budget caps', 'Max power mode & longer autonomous runs', '$40/mo in managed AI credits'],
  },
  team: {
    color: '#67e8f9',
    glow: 'rgba(103,232,249,0.3)',
    border: 'rgba(103,232,249,0.4)',
    bg: 'rgba(103,232,249,0.05)',
    badge: 'BEST VALUE',
    icon: <Users size={18} />,
    tagline: 'Shared team credit runway',
    bestFor: 'Collaborative product teams with heavier workloads.',
    features: ['Everything in Pro', 'Shared team workspace & billing', 'Admin controls & shared secrets', 'Multi-seat delivery', '$110/mo in managed AI credits'],
  },
  enterprise: {
    color: '#a855f7',
    glow: 'rgba(168,85,247,0.3)',
    border: 'rgba(168,85,247,0.4)',
    bg: 'rgba(168,85,247,0.05)',
    badge: '',
    icon: <Crown size={18} />,
    tagline: 'Custom scale & SLA',
    bestFor: 'Large orgs that need custom limits and dedicated support.',
    features: ['Custom credit limits', 'Dedicated support', 'Custom SLAs', 'SSO + audit logs', 'Volume pricing'],
  },
  owner: {
    color: '#7dd3fc',
    glow: 'rgba(125,211,252,0.3)',
    border: 'rgba(125,211,252,0.4)',
    bg: 'rgba(125,211,252,0.05)',
    badge: 'ADMIN',
    icon: <Crown size={18} />,
    tagline: 'Unlimited platform access',
    bestFor: 'Platform owner account.',
    features: ['Unlimited credits', 'Bypass billing', 'All capabilities unlocked', 'Admin dashboard'],
  },
}

const PACK_CONFIG = [
  { amountUsd: 25, color: '#60a5fa', glow: 'rgba(96,165,250,0.3)', border: 'rgba(96,165,250,0.4)', bg: 'rgba(96,165,250,0.05)', label: 'Builder' },
  { amountUsd: 50, color: '#38bdf8', glow: 'rgba(56,189,248,0.35)', border: 'rgba(56,189,248,0.45)', bg: 'rgba(56,189,248,0.06)', label: 'Pro', popular: true },
  { amountUsd: 100, color: '#67e8f9', glow: 'rgba(103,232,249,0.3)', border: 'rgba(103,232,249,0.4)', bg: 'rgba(103,232,249,0.05)', label: 'Power' },
  { amountUsd: 250, color: '#a855f7', glow: 'rgba(168,85,247,0.3)', border: 'rgba(168,85,247,0.4)', bg: 'rgba(168,85,247,0.05)', label: 'Scale' },
]

const asNumber = (v: unknown): number => typeof v === 'number' && Number.isFinite(v) ? v : 0
const asString = (v: unknown): string => typeof v === 'string' ? v : ''

const normalizePlans = (value: unknown): Plan[] => {
  if (!Array.isArray(value)) return []
  return (value as Record<string, unknown>[])
    .filter(Boolean)
    .map(c => ({
      type: asString(c.type),
      name: asString(c.name) || 'Plan',
      monthly_price_cents: asNumber(c.monthly_price_cents),
      monthly_price_id: asString(c.monthly_price_id),
      monthly_credits_usd: asNumber(c.monthly_credits_usd),
      is_popular: Boolean(c.is_popular),
      features: Array.isArray(c.features) ? c.features.filter((f): f is string => typeof f === 'string') : [],
    }))
    .filter(p => Boolean(p.type))
}

const normalizeSubscription = (value: unknown): Subscription | null => {
  if (!value || typeof value !== 'object') return null
  const c = value as Record<string, unknown>
  const planType = asString(c.plan_type)
  const planName = asString(c.plan_name)
  if (!planType && !planName) return null
  return {
    plan_type: planType || 'free',
    plan_name: planName || 'Free',
    status: asString(c.status),
    current_period_end: asString(c.current_period_end),
    cancel_at_period_end: typeof c.cancel_at_period_end === 'boolean' ? c.cancel_at_period_end : undefined,
  }
}

const normalizeInvoices = (value: unknown): Invoice[] => {
  if (!Array.isArray(value)) return []
  return (value as Record<string, unknown>[])
    .filter(Boolean)
    .map(c => ({
      id: asString(c.id),
      amount_paid: asNumber(c.amount_paid),
      currency: asString(c.currency),
      status: asString(c.status),
      created: asNumber(c.created),
      hosted_invoice_url: asString(c.hosted_invoice_url),
      description: asString(c.description),
    }))
    .filter(i => Boolean(i.id))
}

const isPlaceholderPriceID = (id: string): boolean =>
  !id.trim() || new Set([
    'price_builder_monthly', 'price_builder_annual', 'price_pro_monthly', 'price_pro_annual',
    'price_team_monthly', 'price_team_annual', 'price_enterprise_monthly', 'price_enterprise_annual',
  ]).has(id.trim())

const fallbackPlans: Plan[] = [
  { type: 'free', name: 'Free', monthly_price_cents: 0, monthly_price_id: '', monthly_credits_usd: 0, is_popular: false, features: [] },
  { type: 'builder', name: 'Builder', monthly_price_cents: 2400, monthly_price_id: '', monthly_credits_usd: 12, is_popular: false, features: [] },
  { type: 'pro', name: 'Pro', monthly_price_cents: 5900, monthly_price_id: '', monthly_credits_usd: 40, is_popular: true, features: [] },
  { type: 'team', name: 'Team', monthly_price_cents: 14900, monthly_price_id: '', monthly_credits_usd: 110, is_popular: false, features: [] },
]

export function BillingSettings() {
  const [plans, setPlans] = useState<Plan[]>([])
  const [selfServeReady, setSelfServeReady] = useState(false)
  const [subscription, setSubscription] = useState<Subscription | null>(null)
  const [creditBalance, setCreditBalance] = useState<number | null>(null)
  const [hasUnlimited, setHasUnlimited] = useState(false)
  const [bypassBilling, setBypassBilling] = useState(false)
  const [invoices, setInvoices] = useState<Invoice[]>([])
  const [loading, setLoading] = useState(true)
  const [upgradeLoading, setUpgradeLoading] = useState<string | null>(null)
  const [portalLoading, setPortalLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<{ tone: 'success' | 'info'; message: string } | null>(null)
  const [showBuyCredits, setShowBuyCredits] = useState(false)
  const [buyAmount, setBuyAmount] = useState<number | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [plansRes, subRes, balRes, invRes] = await Promise.allSettled([
        apiService.getPlans(),
        apiService.getSubscription(),
        apiService.getCreditBalance(),
        apiService.getInvoices(),
      ])
      if (plansRes.status === 'fulfilled' && plansRes.value.success && plansRes.value.data) {
        const featured = ['free', 'builder', 'pro', 'team']
        setPlans(normalizePlans(plansRes.value.data.plans).filter(p => featured.includes(p.type)))
        setSelfServeReady(Boolean(plansRes.value.data.self_serve_ready))
      } else {
        setPlans([])
        setSelfServeReady(false)
      }
      if (subRes.status === 'fulfilled' && subRes.value.success && subRes.value.data) {
        setSubscription(normalizeSubscription(subRes.value.data))
      } else {
        setSubscription(null)
      }
      if (balRes.status === 'fulfilled' && balRes.value.success && balRes.value.data) {
        setCreditBalance(asNumber(balRes.value.data.balance))
        setHasUnlimited(Boolean(balRes.value.data.has_unlimited))
        setBypassBilling(Boolean(balRes.value.data.bypass_billing))
      } else {
        setCreditBalance(null)
        setHasUnlimited(false)
        setBypassBilling(false)
      }
      if (invRes.status === 'fulfilled' && invRes.value.success && invRes.value.data) {
        setInvoices(normalizeInvoices(invRes.value.data.invoices))
      } else {
        setInvoices([])
      }
    } catch {
      setError('Failed to load billing information.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    if (typeof window === 'undefined') return
    const url = new URL(window.location.href)
    const params = url.searchParams
    if (params.get('success') === 'true') {
      setNotice({ tone: 'success', message: 'Checkout completed! Your plan will update as soon as Stripe confirms.' })
    } else if (params.get('canceled') === 'true') {
      setNotice({ tone: 'info', message: 'Checkout was canceled. No changes made.' })
    } else if (params.get('credits') === 'success') {
      setNotice({ tone: 'success', message: 'Credits purchased! Your balance will refresh once payment is confirmed.' })
    } else if (params.get('credits') === 'canceled') {
      setNotice({ tone: 'info', message: 'Credit purchase canceled. No charges made.' })
    } else {
      return
    }

    ;['success', 'canceled', 'credits', 'billing'].forEach(k => params.delete(k))
    window.history.replaceState({}, '', `${url.pathname}${params.toString() ? `?${params}` : ''}${url.hash}`)
  }, [])

  const currentPlanType = subscription?.plan_type ?? 'free'

  const handleUpgrade = async (plan: Plan) => {
    if (!selfServeReady || !plan.monthly_price_id || isPlaceholderPriceID(plan.monthly_price_id)) {
      setError('Self-serve billing is not available right now.')
      return
    }
    setUpgradeLoading(plan.type)
    setError(null)
    const hasActiveSub = subscription?.status === 'active' || subscription?.status === 'trialing'
    try {
      if (hasActiveSub && currentPlanType !== 'free') {
        const result = await apiService.changePlan({ plan_type: plan.type, billing_cycle: 'monthly' })
        if (result.success) {
          setSubscription(prev => prev ? { ...prev, plan_type: plan.type, plan_name: plan.name, status: result.data?.status ?? prev.status } : prev)
          setNotice({ tone: 'success', message: `Switched to ${plan.name}. Prorated charge or credit applied immediately.` })
        } else {
          setError(result.error || 'Failed to change plan. Please try again.')
        }
      } else {
        const result = await apiService.createCheckoutSession({ price_id: plan.monthly_price_id })
        if (result.success && result.data?.checkout_url) {
          window.location.href = result.data.checkout_url
        } else {
          setError(result.error || 'Failed to start checkout. Please try again.')
        }
      }
    } catch (err: any) {
      setError(err?.response?.data?.error || err?.message || 'Failed to change plan')
    } finally {
      setUpgradeLoading(null)
    }
  }

  const handleManageSubscription = async () => {
    setPortalLoading(true)
    setError(null)
    try {
      const returnPath = `${window.location.pathname}${window.location.search}${window.location.hash}`
      const result = await apiService.createBillingPortalSession(returnPath)
      if (result.success && result.data?.portal_url) {
        window.location.href = result.data.portal_url
      } else {
        setError(result.error || 'Failed to open billing portal.')
      }
    } catch (err: any) {
      setError(err?.response?.data?.error || err?.message || 'Failed to open billing portal')
    } finally {
      setPortalLoading(false)
    }
  }

  const planCfg = PLAN_CONFIG[currentPlanType] ?? PLAN_CONFIG.free
  const creditDisplay = hasUnlimited || bypassBilling
    ? '∞'
    : creditBalance !== null ? `$${creditBalance.toFixed(2)}` : '--'
  const visiblePlans = plans.length > 0 ? plans : fallbackPlans

  if (loading) {
    return (
      <div className="flex items-center justify-center gap-3 rounded-2xl border border-gray-800 bg-gray-950/50 px-6 py-16 text-sm text-gray-400">
        <Loader2 size={16} className="animate-spin" />
        Loading billing…
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-8">
      {notice && (
        <div className={`flex items-start gap-3 rounded-xl border px-4 py-3 text-sm ${
          notice.tone === 'success'
            ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-300'
            : 'border-blue-500/30 bg-blue-500/10 text-blue-300'
        }`}>
          <AlertCircle size={15} className="mt-0.5 shrink-0" />
          <span className="flex-1">{notice.message}</span>
          <button
            onClick={() => setNotice(null)}
            aria-label="Dismiss"
            className="flex min-h-7 min-w-7 items-center justify-center rounded-md opacity-60 transition hover:opacity-100"
          >
            <X size={14} />
          </button>
        </div>
      )}

      {error && (
        <div className="flex items-start gap-3 rounded-xl border border-sky-500/30 bg-sky-500/10 px-4 py-3 text-sm text-sky-200">
          <AlertCircle size={15} className="mt-0.5 shrink-0" />
          <span className="flex-1">{error}</span>
          <button
            onClick={() => setError(null)}
            aria-label="Dismiss"
            className="flex min-h-7 min-w-7 items-center justify-center rounded-md opacity-60 transition hover:opacity-100"
          >
            <X size={14} />
          </button>
        </div>
      )}

      <section
        className="rounded-3xl border bg-black/70 p-6 shadow-[0_20px_60px_rgba(0,0,0,0.45)] backdrop-blur-xl sm:p-7"
        style={{
          borderColor: planCfg.border,
          backgroundImage: `radial-gradient(circle at top left, ${planCfg.glow}, transparent 32%)`,
          boxShadow: `0 0 48px ${planCfg.glow}, 0 20px 60px rgba(0,0,0,0.45)`,
        }}
      >
        <div className="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-4">
            <div className="flex items-center gap-3">
              <div
                className="flex h-11 w-11 items-center justify-center rounded-2xl border"
                style={{ background: planCfg.glow, borderColor: planCfg.border, color: planCfg.color }}
              >
                {planCfg.icon}
              </div>
              <div>
                <div className="text-[11px] uppercase tracking-[0.24em] text-gray-500">Current Plan</div>
                <div className="mt-1 flex flex-wrap items-center gap-2">
                  <span className="text-xl font-black tracking-[0.04em]" style={{ color: planCfg.color }}>
                    {subscription?.plan_name || 'Free'}
                  </span>
                  {subscription?.status && subscription.status !== 'inactive' && (
                    <span className={`rounded-full px-2.5 py-1 text-[10px] font-bold uppercase tracking-[0.18em] ${
                      subscription.status === 'active'
                        ? 'bg-emerald-500/15 text-emerald-300'
                        : 'bg-sky-500/15 text-sky-300'
                    }`}>
                      {subscription.status}
                    </span>
                  )}
                </div>
              </div>
            </div>

            <p className="max-w-2xl text-sm leading-6 text-gray-300">
              {planCfg.tagline}
              {subscription?.current_period_end && (
                <span className="text-gray-500">
                  {' '}· Renews {new Date(subscription.current_period_end).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
                </span>
              )}
            </p>

            <div className="grid gap-3 sm:grid-cols-3">
              <div className="rounded-2xl border border-gray-800 bg-gray-950/70 p-4">
                <div className="text-[11px] uppercase tracking-[0.18em] text-gray-500">Credit Balance</div>
                <div
                  className="mt-2 font-mono text-3xl font-black leading-none"
                  style={{
                    color: hasUnlimited || bypassBilling ? '#4ade80' : (creditBalance ?? 1) <= 0 ? '#7dd3fc' : '#ffffff',
                    filter: `drop-shadow(0 0 12px ${hasUnlimited || bypassBilling ? 'rgba(74,222,128,0.35)' : planCfg.glow})`,
                  }}
                >
                  {creditDisplay}
                </div>
              </div>
              <div className="rounded-2xl border border-gray-800 bg-gray-950/70 p-4">
                <div className="text-[11px] uppercase tracking-[0.18em] text-gray-500">Best For</div>
                <div className="mt-2 text-sm font-semibold text-gray-200">{planCfg.bestFor}</div>
              </div>
              <div className="rounded-2xl border border-gray-800 bg-gray-950/70 p-4">
                <div className="text-[11px] uppercase tracking-[0.18em] text-gray-500">Billing State</div>
                <div className="mt-2 text-sm font-semibold text-gray-200">
                  {hasUnlimited || bypassBilling ? 'Unlimited / bypassed' : subscription?.status || 'Free tier'}
                </div>
              </div>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-3">
            {!hasUnlimited && !bypassBilling && selfServeReady && (
              <button
                onClick={() => {
                  setBuyAmount(null)
                  setShowBuyCredits(true)
                }}
                className="inline-flex min-h-11 items-center gap-2 rounded-xl border border-sky-400/40 bg-gradient-to-r from-sky-600 to-blue-800 px-4 py-2.5 text-sm font-bold text-white shadow-[0_0_24px_rgba(56,189,248,0.28)] transition hover:from-sky-500 hover:to-blue-700"
              >
                <Zap size={14} />
                Buy Credits
              </button>
            )}
            {subscription?.plan_type && subscription.plan_type !== 'free' && (
              <button
                onClick={handleManageSubscription}
                disabled={portalLoading}
                className="inline-flex min-h-11 items-center gap-2 rounded-xl border border-gray-700 bg-gray-900/80 px-4 py-2.5 text-sm font-semibold text-gray-200 transition hover:border-gray-600 hover:bg-gray-800 disabled:opacity-60"
              >
                {portalLoading ? <Loader2 size={14} className="animate-spin" /> : <ExternalLink size={14} />}
                Manage
              </button>
            )}
            <button
              onClick={() => void load()}
              aria-label="Refresh billing"
              className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-xl border border-gray-700 bg-gray-900/80 text-gray-400 transition hover:border-gray-600 hover:bg-gray-800 hover:text-white"
            >
              <RefreshCw size={14} />
            </button>
          </div>
        </div>
      </section>

      <section className="space-y-4">
        <div>
          <h3 className="text-base font-bold text-white">Subscription Plans</h3>
          <p className="mt-1 text-sm text-gray-400">
            Plan tier unlocks capabilities. Credits cover managed AI usage.
          </p>
        </div>

        <div className="grid grid-cols-1 gap-4 xl:grid-cols-4">
          {visiblePlans.map((plan) => {
            const cfg = PLAN_CONFIG[plan.type] ?? PLAN_CONFIG.free
            const isCurrent = plan.type === currentPlanType
            const priceStr = plan.monthly_price_cents === 0 ? 'Free' : `$${(plan.monthly_price_cents / 100).toFixed(0)}`
            const features = plan.features.length > 0 ? plan.features : cfg.features
            const checkoutAvailable = selfServeReady && !isPlaceholderPriceID(plan.monthly_price_id)

            return (
              <div
                key={plan.type}
                className={`relative flex h-full flex-col rounded-3xl border p-5 backdrop-blur-xl transition ${
                  isCurrent
                    ? 'bg-black/70 shadow-[0_0_28px_rgba(255,255,255,0.06)]'
                    : 'bg-gray-950/60 hover:-translate-y-0.5 hover:bg-gray-950/80'
                }`}
                style={{
                  borderColor: isCurrent ? cfg.border : 'rgba(255,255,255,0.08)',
                  boxShadow: isCurrent ? `0 0 36px ${cfg.glow}, 0 16px 36px rgba(0,0,0,0.38)` : undefined,
                }}
              >
                {(isCurrent || cfg.badge) && (
                  <div
                    className="absolute -top-2.5 left-5 rounded-full px-2.5 py-1 text-[10px] font-black uppercase tracking-[0.18em]"
                    style={{ background: cfg.color, color: '#020617' }}
                  >
                    {isCurrent ? 'Current Plan' : cfg.badge}
                  </div>
                )}

                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="flex items-center gap-2">
                      <span style={{ color: cfg.color }}>{cfg.icon}</span>
                      <span className="text-base font-bold text-white">{plan.name}</span>
                    </div>
                    <div className="mt-4 flex items-end gap-2">
                      <span
                        className="font-mono text-3xl font-black"
                        style={{ color: cfg.color, filter: isCurrent ? `drop-shadow(0 0 10px ${cfg.glow})` : 'none' }}
                      >
                        {priceStr}
                      </span>
                      {plan.monthly_price_cents > 0 && <span className="pb-1 text-xs text-gray-500">/mo</span>}
                    </div>
                    {plan.monthly_credits_usd > 0 && (
                      <div className="mt-2 text-xs font-semibold text-emerald-300">
                        + ${plan.monthly_credits_usd}/mo in credits included
                      </div>
                    )}
                    <p className="mt-3 text-sm leading-6 text-gray-400">{cfg.tagline}</p>
                  </div>
                </div>

                <ul className="mt-5 flex flex-1 flex-col gap-3">
                  {features.slice(0, 5).map(f => (
                    <li key={f} className="flex items-start gap-2 text-sm leading-6 text-gray-300">
                      <Check size={14} className="mt-1 shrink-0" style={{ color: cfg.color }} />
                      <span>{f}</span>
                    </li>
                  ))}
                </ul>

                <div className="mt-6">
                  {isCurrent ? (
                    <div className="rounded-2xl border px-4 py-3 text-center text-sm font-bold" style={{ borderColor: cfg.border, color: cfg.color }}>
                      Active Plan
                    </div>
                  ) : plan.type === 'free' ? (
                    <div className="rounded-2xl border border-gray-800 px-4 py-3 text-center text-sm text-gray-500">
                      Default tier
                    </div>
                  ) : checkoutAvailable ? (
                    <button
                      onClick={() => void handleUpgrade(plan)}
                      disabled={upgradeLoading === plan.type}
                      className="inline-flex min-h-11 w-full items-center justify-center gap-2 rounded-2xl border px-4 py-3 text-sm font-bold transition disabled:opacity-60"
                      style={{
                        borderColor: cfg.border,
                        background: `linear-gradient(135deg, ${cfg.color}22, ${cfg.color}44)`,
                        color: '#fff',
                        boxShadow: `0 0 18px ${cfg.glow}`,
                      }}
                    >
                      {upgradeLoading === plan.type ? (
                        <>
                          <Loader2 size={14} className="animate-spin" />
                          {(subscription?.status === 'active' || subscription?.status === 'trialing') && currentPlanType !== 'free' ? 'Switching…' : 'Redirecting…'}
                        </>
                      ) : (
                        <>
                          <CreditCard size={14} />
                          {(subscription?.status === 'active' || subscription?.status === 'trialing') && currentPlanType !== 'free'
                              ? `Switch to ${plan.name}`
                              : `Upgrade to ${plan.name}`}
                        </>
                      )}
                    </button>
                  ) : (
                    <div className="rounded-2xl border border-gray-800 bg-gray-950/70 px-4 py-3 text-center text-sm font-semibold text-gray-400">
                      Contact support
                    </div>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      </section>

      <section className="space-y-4">
        <div>
          <h3 className="text-base font-bold text-white">Credit Packs</h3>
          <p className="mt-1 text-sm text-gray-400">
            One-time top-ups for extra AI usage runway. Don't unlock plan features on their own.
          </p>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {PACK_CONFIG.map(pack => (
            <button
              key={pack.amountUsd}
              onClick={() => {
                if (!selfServeReady) return
                setBuyAmount(pack.amountUsd)
                setShowBuyCredits(true)
              }}
              disabled={!selfServeReady}
              className="relative flex flex-col gap-4 rounded-3xl border p-5 text-left transition hover:-translate-y-0.5 disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:translate-y-0"
              style={{
                borderColor: pack.border,
                background: `linear-gradient(180deg, ${pack.bg}, rgba(3,7,18,0.88))`,
                boxShadow: `0 0 24px ${pack.glow}, 0 12px 28px rgba(0,0,0,0.35)`,
              }}
            >
              {pack.popular && (
                <div
                  className="absolute -top-2.5 left-1/2 -translate-x-1/2 rounded-full px-2.5 py-1 text-[10px] font-black uppercase tracking-[0.18em]"
                  style={{ background: pack.color, color: '#fff' }}
                >
                  Most Popular
                </div>
              )}

              <div className="flex items-center justify-between">
                <span className="text-[11px] font-black uppercase tracking-[0.22em]" style={{ color: pack.color }}>
                  {pack.label}
                </span>
                <Zap size={14} style={{ color: pack.color }} />
              </div>

              <div>
                <div className="font-mono text-4xl font-black leading-none" style={{ color: pack.color, filter: `drop-shadow(0 0 10px ${pack.glow})` }}>
                  ${pack.amountUsd}
                </div>
                <div className="mt-2 text-sm text-gray-400">${pack.amountUsd}.00 in AI credits</div>
              </div>

              <div className="rounded-2xl border px-3 py-2 text-center text-sm font-semibold" style={{ borderColor: pack.border, background: `${pack.color}18`, color: pack.color }}>
                {selfServeReady ? 'Buy Now' : 'Unavailable'}
              </div>
            </button>
          ))}
        </div>
      </section>

      {invoices.length > 0 && (
        <section className="space-y-4">
          <h3 className="text-base font-bold text-white">Invoice History</h3>
          <div className="overflow-hidden rounded-3xl border border-gray-800 bg-gray-950/60">
            {invoices.map((inv, i) => (
              <div
                key={inv.id}
                className={`flex flex-col gap-4 px-5 py-4 sm:flex-row sm:items-center sm:justify-between ${
                  i > 0 ? 'border-t border-gray-800' : ''
                } ${i % 2 === 0 ? 'bg-white/[0.02]' : ''}`}
              >
                <div>
                  <div className="font-medium text-white">{inv.description || 'Subscription payment'}</div>
                  <div className="mt-1 text-xs text-gray-500">
                    {new Date(inv.created * 1000).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <span className="font-mono font-semibold text-white">${(inv.amount_paid / 100).toFixed(2)}</span>
                  <span className={`rounded-full px-2.5 py-1 text-[10px] font-bold uppercase tracking-[0.18em] ${
                    inv.status === 'paid'
                      ? 'bg-emerald-500/15 text-emerald-300'
                      : 'bg-gray-800 text-gray-400'
                  }`}>
                    {inv.status}
                  </span>
                  {inv.hosted_invoice_url && (
                    <a
                      href={inv.hosted_invoice_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-gray-500 transition hover:text-white"
                    >
                      <ExternalLink size={14} />
                    </a>
                  )}
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {showBuyCredits && (
        <BuyCreditsModal
          defaultAmount={buyAmount ?? undefined}
          onClose={() => {
            setShowBuyCredits(false)
            setBuyAmount(null)
            void load()
          }}
        />
      )}
    </div>
  )
}

export default BillingSettings
