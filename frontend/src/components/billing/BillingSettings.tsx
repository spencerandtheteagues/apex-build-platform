// APEX-BUILD — Billing Settings
// Full in-app billing page with color-coded plan + credit pack cards

import React, { useState, useEffect, useCallback } from 'react'
import {
  Zap, CreditCard, ExternalLink, Check, Loader2,
  AlertCircle, RefreshCw, Star, Crown, Users, Rocket, ShieldCheck,
} from 'lucide-react'
import apiService from '@/services/api'
import { BuyCreditsModal } from './BuyCreditsModal'

// ─── Types ────────────────────────────────────────────────────────────────────

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

// ─── Plan config ──────────────────────────────────────────────────────────────

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
    features: ['Static frontend builds', 'One-time $5 managed trial', 'UI mockups & prototypes', 'Honest free tier — no surprises'],
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
    features: ['Backend + API generation', 'Database-backed apps', 'Auth + deployment', 'Publish to production', 'BYOK (bring your own keys)'],
  },
  pro: {
    color: '#00f5ff',
    glow: 'rgba(0,245,255,0.35)',
    border: 'rgba(0,245,255,0.45)',
    bg: 'rgba(0,245,255,0.05)',
    badge: 'MOST POPULAR',
    icon: <ShieldCheck size={18} />,
    tagline: 'Serious shipping, every week',
    bestFor: 'Founders & operators shipping production app changes weekly.',
    features: ['Everything in Builder', 'Longer autonomous runs', 'Higher included credits', 'Max power mode access', 'Priority for heavy workflows'],
  },
  team: {
    color: '#f59e0b',
    glow: 'rgba(245,158,11,0.3)',
    border: 'rgba(245,158,11,0.4)',
    bg: 'rgba(245,158,11,0.05)',
    badge: 'BEST VALUE',
    icon: <Users size={18} />,
    tagline: 'Shared team credit runway',
    bestFor: 'Collaborative product teams with heavier workloads.',
    features: ['Everything in Pro', 'Shared team workspace', 'Largest included credit runway', 'Multi-seat delivery', 'Team billing management'],
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
    color: '#ff0033',
    glow: 'rgba(255,0,51,0.3)',
    border: 'rgba(255,0,51,0.4)',
    bg: 'rgba(255,0,51,0.05)',
    badge: 'ADMIN',
    icon: <Crown size={18} />,
    tagline: 'Unlimited platform access',
    bestFor: 'Platform owner account.',
    features: ['Unlimited credits', 'Bypass billing', 'All capabilities unlocked', 'Admin dashboard'],
  },
}

// Credit pack color tiers
const PACK_CONFIG = [
  { amountUsd: 25,  color: '#22c55e', glow: 'rgba(34,197,94,0.3)',   border: 'rgba(34,197,94,0.35)',   bg: 'rgba(34,197,94,0.05)',   label: 'Starter' },
  { amountUsd: 50,  color: '#00f5ff', glow: 'rgba(0,245,255,0.35)',  border: 'rgba(0,245,255,0.45)',   bg: 'rgba(0,245,255,0.06)',   label: 'Builder', popular: true },
  { amountUsd: 100, color: '#f97316', glow: 'rgba(249,115,22,0.3)',  border: 'rgba(249,115,22,0.4)',   bg: 'rgba(249,115,22,0.05)',  label: 'Pro' },
  { amountUsd: 250, color: '#a855f7', glow: 'rgba(168,85,247,0.3)',  border: 'rgba(168,85,247,0.4)',   bg: 'rgba(168,85,247,0.05)',  label: 'Power' },
]

// ─── Helpers ──────────────────────────────────────────────────────────────────

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
    'price_builder_monthly','price_builder_annual','price_pro_monthly','price_pro_annual',
    'price_team_monthly','price_team_annual','price_enterprise_monthly','price_enterprise_annual',
  ]).has(id.trim())

// ─── Component ────────────────────────────────────────────────────────────────

export function BillingSettings() {
  const [plans, setPlans] = useState<Plan[]>([])
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
      } else setPlans([])
      if (subRes.status === 'fulfilled' && subRes.value.success && subRes.value.data)
        setSubscription(normalizeSubscription(subRes.value.data))
      else setSubscription(null)
      if (balRes.status === 'fulfilled' && balRes.value.success && balRes.value.data) {
        setCreditBalance(asNumber(balRes.value.data.balance))
        setHasUnlimited(Boolean(balRes.value.data.has_unlimited))
        setBypassBilling(Boolean(balRes.value.data.bypass_billing))
      } else { setCreditBalance(null); setHasUnlimited(false); setBypassBilling(false) }
      if (invRes.status === 'fulfilled' && invRes.value.success && invRes.value.data)
        setInvoices(normalizeInvoices(invRes.value.data.invoices))
      else setInvoices([])
    } catch {
      setError('Failed to load billing information.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  useEffect(() => {
    if (typeof window === 'undefined') return
    const url = new URL(window.location.href)
    const params = url.searchParams
    if (params.get('success') === 'true')
      setNotice({ tone: 'success', message: 'Checkout completed! Your plan will update as soon as Stripe confirms.' })
    else if (params.get('canceled') === 'true')
      setNotice({ tone: 'info', message: 'Checkout was canceled. No changes made.' })
    else if (params.get('credits') === 'success')
      setNotice({ tone: 'success', message: 'Credits purchased! Your balance will refresh once payment is confirmed.' })
    else if (params.get('credits') === 'canceled')
      setNotice({ tone: 'info', message: 'Credit purchase canceled. No charges made.' })
    else return
    ;['success','canceled','credits','billing'].forEach(k => params.delete(k))
    window.history.replaceState({}, '', `${url.pathname}${params.toString() ? `?${params}` : ''}${url.hash}`)
  }, [])

  const handleUpgrade = async (plan: Plan) => {
    if (!plan.monthly_price_id || isPlaceholderPriceID(plan.monthly_price_id)) {
      setError('Stripe is not configured in this environment.')
      return
    }
    setUpgradeLoading(plan.type)
    setError(null)
    try {
      const result = await apiService.createCheckoutSession({ price_id: plan.monthly_price_id })
      if (result.success && result.data?.checkout_url) window.location.href = result.data.checkout_url
      else setError(result.error || 'Failed to start checkout. Please try again.')
    } catch (err: any) {
      setError(err?.response?.data?.error || err?.message || 'Failed to start checkout')
    } finally { setUpgradeLoading(null) }
  }

  const handleManageSubscription = async () => {
    setPortalLoading(true)
    setError(null)
    try {
      const result = await apiService.createBillingPortalSession(window.location.href)
      if (result.success && result.data?.portal_url) window.location.href = result.data.portal_url
      else setError(result.error || 'Failed to open billing portal.')
    } catch (err: any) {
      setError(err?.response?.data?.error || err?.message || 'Failed to open billing portal')
    } finally { setPortalLoading(false) }
  }

  const currentPlanType = subscription?.plan_type ?? 'free'
  const planCfg = PLAN_CONFIG[currentPlanType] ?? PLAN_CONFIG.free
  const creditDisplay = hasUnlimited || bypassBilling
    ? '∞'
    : creditBalance !== null ? `$${creditBalance.toFixed(2)}` : '--'

  if (loading) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '4rem 0', gap: 10, color: 'rgba(255,255,255,0.4)', fontSize: '0.875rem' }}>
        <Loader2 size={16} style={{ animation: 'spin 1s linear infinite' }} />
        Loading billing…
        <style>{`@keyframes spin{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}`}</style>
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 32 }}>

      {/* ── Notices ────────────────────────────────────────────────────── */}
      {notice && (
        <div style={{
          display: 'flex', alignItems: 'flex-start', gap: 12, padding: '12px 16px',
          borderRadius: 10, fontSize: '0.875rem',
          background: notice.tone === 'success' ? 'rgba(34,197,94,0.08)' : 'rgba(59,130,246,0.08)',
          border: `1px solid ${notice.tone === 'success' ? 'rgba(34,197,94,0.25)' : 'rgba(59,130,246,0.25)'}`,
          color: notice.tone === 'success' ? '#4ade80' : '#60a5fa',
        }}>
          <AlertCircle size={15} style={{ flexShrink: 0, marginTop: 1 }} />
          <span style={{ flex: 1 }}>{notice.message}</span>
          <button onClick={() => setNotice(null)} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'inherit', opacity: 0.6 }}>✕</button>
        </div>
      )}
      {error && (
        <div style={{
          display: 'flex', alignItems: 'flex-start', gap: 12, padding: '12px 16px',
          borderRadius: 10, fontSize: '0.875rem',
          background: 'rgba(255,0,51,0.08)', border: '1px solid rgba(255,0,51,0.25)', color: '#f87171',
        }}>
          <AlertCircle size={15} style={{ flexShrink: 0, marginTop: 1 }} />
          <span style={{ flex: 1 }}>{error}</span>
          <button onClick={() => setError(null)} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'inherit', opacity: 0.6 }}>✕</button>
        </div>
      )}

      {/* ── Account Status Hero ────────────────────────────────────────── */}
      <div style={{
        borderRadius: 16,
        border: `1px solid ${planCfg.border}`,
        background: `radial-gradient(circle at top left, ${planCfg.glow}, transparent 50%), #0a0a0a`,
        boxShadow: `0 0 60px ${planCfg.glow}, 0 16px 40px rgba(0,0,0,0.5)`,
        padding: '24px 28px',
        display: 'grid',
        gridTemplateColumns: '1fr auto',
        gap: 24,
        alignItems: 'center',
      }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div style={{
              width: 36, height: 36,
              background: `${planCfg.glow}`,
              border: `1px solid ${planCfg.border}`,
              borderRadius: 8,
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              color: planCfg.color,
            }}>
              {planCfg.icon}
            </div>
            <div>
              <div style={{ fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.18em', color: 'rgba(255,255,255,0.4)', marginBottom: 2 }}>
                Current Plan
              </div>
              <div style={{ fontWeight: 800, fontSize: '1.1rem', color: planCfg.color, letterSpacing: '0.04em' }}>
                {subscription?.plan_name || 'Free'}
                {subscription?.status && subscription.status !== 'inactive' && (
                  <span style={{
                    marginLeft: 8, fontSize: '0.65rem', fontWeight: 700,
                    padding: '2px 7px', borderRadius: 100,
                    background: subscription.status === 'active' ? 'rgba(34,197,94,0.15)' : 'rgba(245,158,11,0.15)',
                    color: subscription.status === 'active' ? '#4ade80' : '#fbbf24',
                    textTransform: 'uppercase', letterSpacing: '0.1em',
                    verticalAlign: 'middle',
                  }}>
                    {subscription.status}
                  </span>
                )}
              </div>
            </div>
          </div>

          <div style={{ fontSize: '0.8rem', color: 'rgba(255,255,255,0.5)', lineHeight: 1.55 }}>
            {planCfg.tagline}
            {subscription?.current_period_end && (
              <span style={{ marginLeft: 8, color: 'rgba(255,255,255,0.3)' }}>
                · Renews {new Date(subscription.current_period_end).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
              </span>
            )}
          </div>
        </div>

        <div style={{ textAlign: 'right', display: 'flex', flexDirection: 'column', gap: 8, alignItems: 'flex-end' }}>
          <div>
            <div style={{ fontSize: '0.65rem', textTransform: 'uppercase', letterSpacing: '0.18em', color: 'rgba(255,255,255,0.35)', marginBottom: 4 }}>
              Credit Balance
            </div>
            <div style={{
              fontFamily: 'ui-monospace, monospace', fontWeight: 900, fontSize: '2.2rem',
              color: hasUnlimited || bypassBilling ? '#4ade80' : (creditBalance ?? 1) <= 0 ? '#f87171' : '#ffffff',
              filter: `drop-shadow(0 0 12px ${hasUnlimited || bypassBilling ? 'rgba(74,222,128,0.6)' : planCfg.glow})`,
              lineHeight: 1,
            }}>
              {creditDisplay}
            </div>
          </div>

          <div style={{ display: 'flex', gap: 8 }}>
            {!hasUnlimited && !bypassBilling && (
              <button
                onClick={() => { setBuyAmount(null); setShowBuyCredits(true) }}
                style={{
                  background: 'linear-gradient(135deg,#ff0033,#cc0029)',
                  border: 'none', borderRadius: 8, padding: '8px 14px',
                  color: '#fff', fontSize: '0.8rem', fontWeight: 700, cursor: 'pointer',
                  display: 'flex', alignItems: 'center', gap: 6,
                  boxShadow: '0 0 20px rgba(255,0,51,0.3)',
                }}
              >
                <Zap size={13} />
                Buy Credits
              </button>
            )}
            {subscription?.plan_type && subscription.plan_type !== 'free' && (
              <button
                onClick={handleManageSubscription}
                disabled={portalLoading}
                style={{
                  background: 'rgba(255,255,255,0.05)', border: '1px solid rgba(255,255,255,0.12)',
                  borderRadius: 8, padding: '8px 14px',
                  color: 'rgba(255,255,255,0.7)', fontSize: '0.8rem', fontWeight: 600, cursor: 'pointer',
                  display: 'flex', alignItems: 'center', gap: 6,
                }}
              >
                {portalLoading ? <Loader2 size={13} style={{ animation: 'spin 1s linear infinite' }} /> : <ExternalLink size={13} />}
                Manage
              </button>
            )}
            <button onClick={load} style={{ background: 'none', border: '1px solid rgba(255,255,255,0.08)', borderRadius: 8, padding: '8px 10px', color: 'rgba(255,255,255,0.4)', cursor: 'pointer' }}>
              <RefreshCw size={13} />
            </button>
          </div>
        </div>
      </div>

      {/* ── Subscription Plan Cards ────────────────────────────────────── */}
      <div>
        <div style={{ marginBottom: 14 }}>
          <h3 style={{ margin: 0, color: '#fff', fontSize: '0.95rem', fontWeight: 700 }}>Subscription Plans</h3>
          <p style={{ margin: '4px 0 0', color: 'rgba(255,255,255,0.4)', fontSize: '0.78rem' }}>
            Plan tier unlocks capabilities. Credits cover managed AI usage.
          </p>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 14 }}>
          {(plans.length > 0 ? plans : [
            { type: 'free', name: 'Free', monthly_price_cents: 0, monthly_price_id: '', monthly_credits_usd: 0, is_popular: false, features: [] },
            { type: 'builder', name: 'Builder', monthly_price_cents: 2400, monthly_price_id: '', monthly_credits_usd: 12, is_popular: false, features: [] },
            { type: 'pro', name: 'Pro', monthly_price_cents: 5900, monthly_price_id: '', monthly_credits_usd: 40, is_popular: true, features: [] },
            { type: 'team', name: 'Team', monthly_price_cents: 14900, monthly_price_id: '', monthly_credits_usd: 110, is_popular: false, features: [] },
          ]).map(plan => {
            const cfg = PLAN_CONFIG[plan.type] ?? PLAN_CONFIG.free
            const isCurrent = plan.type === currentPlanType
            const priceStr = plan.monthly_price_cents === 0 ? 'Free' : `$${(plan.monthly_price_cents / 100).toFixed(0)}`
            const features = plan.features.length > 0 ? plan.features : cfg.features

            return (
              <div
                key={plan.type}
                style={{
                  position: 'relative',
                  borderRadius: 14,
                  border: `1px solid ${isCurrent ? cfg.border : 'rgba(255,255,255,0.08)'}`,
                  background: isCurrent ? cfg.bg : 'rgba(255,255,255,0.025)',
                  boxShadow: isCurrent ? `0 0 40px ${cfg.glow}, 0 8px 32px rgba(0,0,0,0.4)` : '0 4px 20px rgba(0,0,0,0.3)',
                  padding: '20px 18px 18px',
                  display: 'flex', flexDirection: 'column', gap: 14,
                  transition: 'all 0.2s',
                }}
                onMouseEnter={e => {
                  if (!isCurrent) {
                    ;(e.currentTarget as HTMLDivElement).style.border = `1px solid ${cfg.border}`
                    ;(e.currentTarget as HTMLDivElement).style.background = cfg.bg
                    ;(e.currentTarget as HTMLDivElement).style.boxShadow = `0 0 30px ${cfg.glow}, 0 8px 32px rgba(0,0,0,0.4)`
                  }
                }}
                onMouseLeave={e => {
                  if (!isCurrent) {
                    ;(e.currentTarget as HTMLDivElement).style.border = '1px solid rgba(255,255,255,0.08)'
                    ;(e.currentTarget as HTMLDivElement).style.background = 'rgba(255,255,255,0.025)'
                    ;(e.currentTarget as HTMLDivElement).style.boxShadow = '0 4px 20px rgba(0,0,0,0.3)'
                  }
                }}
              >
                {/* Badge */}
                {(isCurrent || cfg.badge) && (
                  <div style={{
                    position: 'absolute', top: -11, left: 16,
                    fontSize: '0.6rem', fontWeight: 900, letterSpacing: '0.1em',
                    padding: '3px 9px', borderRadius: 100, textTransform: 'uppercase',
                    background: isCurrent ? cfg.color : cfg.color,
                    color: ['#00f5ff','#f59e0b'].includes(cfg.color) ? '#000' : '#fff',
                  }}>
                    {isCurrent ? 'CURRENT PLAN' : cfg.badge}
                  </div>
                )}

                {/* Header */}
                <div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
                    <div style={{ color: cfg.color }}>{cfg.icon}</div>
                    <span style={{ fontWeight: 700, fontSize: '0.95rem', color: '#fff' }}>{plan.name}</span>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 4 }}>
                    <span style={{
                      fontFamily: 'ui-monospace, monospace', fontWeight: 900, fontSize: '1.8rem',
                      color: cfg.color,
                      filter: isCurrent ? `drop-shadow(0 0 10px ${cfg.glow})` : 'none',
                    }}>
                      {priceStr}
                    </span>
                    {plan.monthly_price_cents > 0 && (
                      <span style={{ fontSize: '0.75rem', color: 'rgba(255,255,255,0.35)' }}>/mo</span>
                    )}
                  </div>
                  {plan.monthly_credits_usd > 0 && (
                    <div style={{ fontSize: '0.72rem', color: '#4ade80', marginTop: 2 }}>
                      + ${plan.monthly_credits_usd}/mo in credits included
                    </div>
                  )}
                  <div style={{ fontSize: '0.75rem', color: 'rgba(255,255,255,0.45)', marginTop: 5, lineHeight: 1.5 }}>
                    {cfg.tagline}
                  </div>
                </div>

                {/* Features */}
                <ul style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 6, flex: 1 }}>
                  {features.slice(0, 5).map(f => (
                    <li key={f} style={{ display: 'flex', alignItems: 'flex-start', gap: 7, fontSize: '0.75rem', color: 'rgba(255,255,255,0.7)', lineHeight: 1.4 }}>
                      <Check size={12} style={{ color: cfg.color, flexShrink: 0, marginTop: 1 }} />
                      {f}
                    </li>
                  ))}
                </ul>

                {/* CTA */}
                {isCurrent ? (
                  <div style={{
                    textAlign: 'center', padding: '8px 0', fontSize: '0.75rem', fontWeight: 700,
                    color: cfg.color, borderTop: `1px solid ${cfg.border}`,
                  }}>
                    ✓ Active Plan
                  </div>
                ) : plan.type === 'free' ? (
                  <div style={{
                    textAlign: 'center', padding: '8px 0', fontSize: '0.72rem',
                    color: 'rgba(255,255,255,0.3)', borderTop: '1px solid rgba(255,255,255,0.06)',
                  }}>
                    Default tier
                  </div>
                ) : (
                  <button
                    onClick={() => handleUpgrade(plan)}
                    disabled={upgradeLoading === plan.type}
                    style={{
                      width: '100%', padding: '10px 0',
                      background: `linear-gradient(135deg, ${cfg.color}22, ${cfg.color}44)`,
                      border: `1px solid ${cfg.border}`,
                      borderRadius: 8, cursor: 'pointer',
                      color: cfg.color, fontWeight: 700, fontSize: '0.82rem',
                      display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 7,
                      transition: 'all 0.2s',
                      boxShadow: `0 0 16px ${cfg.glow}`,
                    }}
                    onMouseEnter={e => { (e.currentTarget as HTMLButtonElement).style.background = `${cfg.color}33` }}
                    onMouseLeave={e => { (e.currentTarget as HTMLButtonElement).style.background = `linear-gradient(135deg, ${cfg.color}22, ${cfg.color}44)` }}
                  >
                    {upgradeLoading === plan.type
                      ? <><Loader2 size={13} style={{ animation: 'spin 1s linear infinite' }} /> Redirecting…</>
                      : <><CreditCard size={13} /> Upgrade to {plan.name}</>
                    }
                  </button>
                )}
              </div>
            )
          })}
        </div>
      </div>

      {/* ── Credit Pack Cards ──────────────────────────────────────────── */}
      <div>
        <div style={{ marginBottom: 14 }}>
          <h3 style={{ margin: 0, color: '#fff', fontSize: '0.95rem', fontWeight: 700 }}>Credit Packs</h3>
          <p style={{ margin: '4px 0 0', color: 'rgba(255,255,255,0.4)', fontSize: '0.78rem' }}>
            One-time top-ups for extra AI usage runway. Don't unlock plan features on their own.
          </p>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))', gap: 12 }}>
          {PACK_CONFIG.map(pack => (
            <button
              key={pack.amountUsd}
              onClick={() => { setBuyAmount(pack.amountUsd); setShowBuyCredits(true) }}
              style={{
                position: 'relative',
                borderRadius: 12,
                border: `1px solid ${pack.border}`,
                background: pack.bg,
                boxShadow: `0 0 24px ${pack.glow}, 0 4px 16px rgba(0,0,0,0.4)`,
                padding: '18px 16px 16px',
                cursor: 'pointer', textAlign: 'left',
                transition: 'all 0.18s',
                display: 'flex', flexDirection: 'column', gap: 8,
              }}
              onMouseEnter={e => {
                ;(e.currentTarget as HTMLButtonElement).style.boxShadow = `0 0 40px ${pack.glow}, 0 8px 28px rgba(0,0,0,0.5)`
                ;(e.currentTarget as HTMLButtonElement).style.transform = 'translateY(-2px)'
              }}
              onMouseLeave={e => {
                ;(e.currentTarget as HTMLButtonElement).style.boxShadow = `0 0 24px ${pack.glow}, 0 4px 16px rgba(0,0,0,0.4)`
                ;(e.currentTarget as HTMLButtonElement).style.transform = 'translateY(0)'
              }}
            >
              {pack.popular && (
                <div style={{
                  position: 'absolute', top: -10, left: '50%', transform: 'translateX(-50%)',
                  fontSize: '0.58rem', fontWeight: 900, letterSpacing: '0.1em',
                  padding: '2px 10px', borderRadius: 100, textTransform: 'uppercase',
                  background: pack.color,
                  color: pack.color === '#00f5ff' ? '#000' : '#fff',
                  whiteSpace: 'nowrap',
                }}>
                  Most Popular
                </div>
              )}

              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <span style={{ fontSize: '0.65rem', fontWeight: 700, letterSpacing: '0.14em', textTransform: 'uppercase', color: pack.color }}>
                  {pack.label}
                </span>
                <Zap size={14} style={{ color: pack.color }} />
              </div>

              <div>
                <div style={{
                  fontFamily: 'ui-monospace, monospace', fontWeight: 900, fontSize: '2rem',
                  color: pack.color,
                  filter: `drop-shadow(0 0 10px ${pack.glow})`,
                  lineHeight: 1,
                }}>
                  ${pack.amountUsd}
                </div>
                <div style={{ fontSize: '0.72rem', color: 'rgba(255,255,255,0.4)', marginTop: 4 }}>
                  ${pack.amountUsd}.00 in AI credits
                </div>
              </div>

              <div style={{
                padding: '6px 10px', borderRadius: 7,
                background: `${pack.color}18`, border: `1px solid ${pack.border}`,
                fontSize: '0.7rem', color: pack.color, fontWeight: 600, textAlign: 'center',
              }}>
                Buy Now →
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* ── Invoice History ────────────────────────────────────────────── */}
      {invoices.length > 0 && (
        <div>
          <h3 style={{ margin: '0 0 12px', color: '#fff', fontSize: '0.95rem', fontWeight: 700 }}>Invoice History</h3>
          <div style={{ borderRadius: 12, border: '1px solid rgba(255,255,255,0.08)', overflow: 'hidden' }}>
            {invoices.map((inv, i) => (
              <div
                key={inv.id}
                style={{
                  display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                  padding: '12px 16px', fontSize: '0.82rem',
                  borderTop: i > 0 ? '1px solid rgba(255,255,255,0.06)' : 'none',
                  background: i % 2 === 0 ? 'rgba(255,255,255,0.015)' : 'transparent',
                }}
              >
                <div>
                  <div style={{ color: '#fff', fontWeight: 500 }}>{inv.description || 'Subscription payment'}</div>
                  <div style={{ color: 'rgba(255,255,255,0.35)', fontSize: '0.72rem', marginTop: 2 }}>
                    {new Date(inv.created * 1000).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
                  </div>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                  <span style={{ fontFamily: 'ui-monospace, monospace', color: '#fff', fontWeight: 600 }}>
                    ${(inv.amount_paid / 100).toFixed(2)}
                  </span>
                  <span style={{
                    fontSize: '0.65rem', padding: '2px 7px', borderRadius: 100, fontWeight: 700, textTransform: 'uppercase',
                    background: inv.status === 'paid' ? 'rgba(34,197,94,0.15)' : 'rgba(255,255,255,0.06)',
                    color: inv.status === 'paid' ? '#4ade80' : 'rgba(255,255,255,0.4)',
                  }}>
                    {inv.status}
                  </span>
                  {inv.hosted_invoice_url && (
                    <a href={inv.hosted_invoice_url} target="_blank" rel="noopener noreferrer"
                      style={{ color: 'rgba(255,255,255,0.3)', display: 'flex' }}
                      onMouseEnter={e => ((e.currentTarget as HTMLAnchorElement).style.color = 'rgba(255,255,255,0.7)')}
                      onMouseLeave={e => ((e.currentTarget as HTMLAnchorElement).style.color = 'rgba(255,255,255,0.3)')}
                    >
                      <ExternalLink size={13} />
                    </a>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {showBuyCredits && (
        <BuyCreditsModal
          defaultAmount={buyAmount ?? undefined}
          onClose={() => { setShowBuyCredits(false); setBuyAmount(null); load() }}
        />
      )}

      <style>{`@keyframes spin{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}`}</style>
    </div>
  )
}

export default BillingSettings
