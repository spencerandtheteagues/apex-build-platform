// APEX-BUILD — Billing Settings
// Shows current plan, credit balance, plan upgrade options, and invoice history

import React, { useState, useEffect, useCallback } from 'react'
import { Zap, CreditCard, ExternalLink, Check, ChevronRight, Loader2, AlertCircle, RefreshCw } from 'lucide-react'
import { cn } from '@/lib/utils'
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

const planNarrative = (planType: string) => {
  switch (planType) {
    case 'free':
      return 'Static frontend websites, mockups, and honest prototype work with a one-time $5 managed trial.'
    case 'builder':
      return 'Unlocks backend, auth, database, deploy, publish, and BYOK for serious app builds.'
    case 'pro':
      return 'Best default for weekly shipping, longer autonomous runs, and heavier managed usage.'
    case 'team':
      return 'Shared team workflow with the highest included credit runway.'
    default:
      return 'Plan details are still loading.'
  }
}

const planCallouts = (planType: string) => {
  switch (planType) {
    case 'free':
      return ['One-time $5 trial', 'Static/frontend-only', 'Credits do not unlock paid capabilities']
    case 'builder':
      return ['Full-stack unlocked', 'Publish unlocked', 'BYOK unlocked']
    case 'pro':
      return ['Longer runs', 'Higher included credits', 'Priority for heavier app workflows']
    case 'team':
      return ['Shared workspace', 'Largest included credits', 'Best for multi-seat delivery']
    default:
      return []
  }
}

const asNumber = (value: unknown): number => {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

const asString = (value: unknown): string => {
  return typeof value === 'string' ? value : ''
}

const normalizePlans = (value: unknown): Plan[] => {
  if (!Array.isArray(value)) {
    return []
  }

  return value
    .filter((candidate): candidate is Record<string, unknown> => Boolean(candidate) && typeof candidate === 'object')
    .map((candidate) => ({
      type: asString(candidate.type),
      name: asString(candidate.name) || 'Plan',
      monthly_price_cents: asNumber(candidate.monthly_price_cents),
      monthly_price_id: asString(candidate.monthly_price_id),
      monthly_credits_usd: asNumber(candidate.monthly_credits_usd),
      is_popular: Boolean(candidate.is_popular),
      features: Array.isArray(candidate.features) ? candidate.features.filter((feature): feature is string => typeof feature === 'string') : [],
    }))
    .filter((plan) => Boolean(plan.type))
}

const normalizeSubscription = (value: unknown): Subscription | null => {
  if (!value || typeof value !== 'object') {
    return null
  }

  const candidate = value as Record<string, unknown>
  const planType = asString(candidate.plan_type)
  const planName = asString(candidate.plan_name)
  if (!planType && !planName) {
    return null
  }

  return {
    plan_type: planType || 'free',
    plan_name: planName || 'Free',
    status: asString(candidate.status),
    current_period_end: asString(candidate.current_period_end),
    cancel_at_period_end: typeof candidate.cancel_at_period_end === 'boolean' ? candidate.cancel_at_period_end : undefined,
  }
}

const normalizeInvoices = (value: unknown): Invoice[] => {
  if (!Array.isArray(value)) {
    return []
  }

  return value
    .filter((candidate): candidate is Record<string, unknown> => Boolean(candidate) && typeof candidate === 'object')
    .map((candidate) => ({
      id: asString(candidate.id),
      amount_paid: asNumber(candidate.amount_paid),
      currency: asString(candidate.currency),
      status: asString(candidate.status),
      created: asNumber(candidate.created),
      hosted_invoice_url: asString(candidate.hosted_invoice_url),
      description: asString(candidate.description),
    }))
    .filter((invoice) => Boolean(invoice.id))
}

const isPlaceholderPriceID = (priceID: string): boolean => {
  const normalized = priceID.trim()
  if (!normalized) {
    return true
  }

  return new Set([
    'price_builder_monthly',
    'price_builder_annual',
    'price_pro_monthly',
    'price_pro_annual',
    'price_team_monthly',
    'price_team_annual',
    'price_enterprise_monthly',
    'price_enterprise_annual',
  ]).has(normalized)
}

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
        setPlans(normalizePlans(plansRes.value.data.plans).filter((plan) => featured.includes(plan.type)))
      } else {
        setPlans([])
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
      setPlans([])
      setSubscription(null)
      setCreditBalance(null)
      setHasUnlimited(false)
      setBypassBilling(false)
      setInvoices([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  useEffect(() => {
    if (typeof window === 'undefined') return

    const url = new URL(window.location.href)
    const params = url.searchParams

    if (params.get('success') === 'true') {
      setNotice({ tone: 'success', message: 'Checkout completed. Billing status will update as soon as Stripe confirms the payment.' })
    } else if (params.get('canceled') === 'true') {
      setNotice({ tone: 'info', message: 'Checkout was canceled. No changes were made to your subscription.' })
    } else if (params.get('credits') === 'success') {
      setNotice({ tone: 'success', message: 'Credit purchase completed. Your balance will refresh as soon as the payment is confirmed.' })
    } else if (params.get('credits') === 'canceled') {
      setNotice({ tone: 'info', message: 'Credit purchase was canceled. No charges were made.' })
    } else {
      return
    }

    params.delete('success')
    params.delete('canceled')
    params.delete('credits')
    params.delete('billing')

    const nextSearch = params.toString()
    const nextUrl = `${url.pathname}${nextSearch ? `?${nextSearch}` : ''}${url.hash}`
    window.history.replaceState({}, '', nextUrl)
  }, [])

  const handleUpgrade = async (plan: Plan) => {
    if (!plan.monthly_price_id || isPlaceholderPriceID(plan.monthly_price_id)) {
      setError('Stripe is not configured in this environment. Set STRIPE_PRICE_* environment variables.')
      return
    }
    setUpgradeLoading(plan.type)
    setError(null)
    try {
      const result = await apiService.createCheckoutSession({
        price_id: plan.monthly_price_id,
      })
      if (result.success && result.data?.checkout_url) {
        window.location.href = result.data.checkout_url
      } else {
        setError(result.error || 'Failed to start checkout. Please try again.')
      }
    } catch (err: any) {
      setError(err?.response?.data?.error || err?.message || 'Failed to start checkout')
    } finally {
      setUpgradeLoading(null)
    }
  }

  const handleManageSubscription = async () => {
    setPortalLoading(true)
    setError(null)
    try {
      const result = await apiService.createBillingPortalSession(window.location.href)
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

  const currentPlanType = subscription?.plan_type ?? 'free'

  const creditDisplay = hasUnlimited || bypassBilling
    ? 'Unlimited'
    : creditBalance !== null
      ? `$${creditBalance.toFixed(2)}`
      : '--'

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12 text-gray-500 gap-2 text-sm">
        <Loader2 className="w-4 h-4 animate-spin" />
        Loading billing…
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {notice && (
        <div className={cn(
          'flex items-start gap-3 p-3 rounded-lg border text-sm',
          notice.tone === 'success'
            ? 'bg-emerald-500/10 border-emerald-500/25 text-emerald-300'
            : 'bg-blue-500/10 border-blue-500/25 text-blue-300'
        )}>
          <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>{notice.message}</span>
          <button
            onClick={() => setNotice(null)}
            className="ml-auto opacity-70 hover:opacity-100"
            aria-label="Dismiss billing notice"
          >
            ✕
          </button>
        </div>
      )}

      {error && (
        <div className="flex items-start gap-3 p-3 bg-red-500/10 border border-red-500/25 rounded-lg text-sm text-red-400">
          <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>{error}</span>
          <button onClick={() => setError(null)} className="ml-auto text-red-400/60 hover:text-red-400">✕</button>
        </div>
      )}

      {/* Credit Balance Card */}
      <div className="overflow-hidden rounded-2xl border border-gray-800 bg-black/70 shadow-[0_24px_70px_rgba(0,0,0,0.38)]">
        <div className="bg-[radial-gradient(circle_at_top_left,rgba(255,0,51,0.18),transparent_38%),radial-gradient(circle_at_top_right,rgba(34,211,238,0.14),transparent_34%),linear-gradient(180deg,rgba(255,255,255,0.03),transparent_60%)] p-5">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
            <div className="space-y-3">
              <div className="text-xs text-gray-500 uppercase flex items-center gap-1.5 tracking-[0.18em]">
                <Zap className="w-3 h-3" /> Billing control plane
              </div>
              <div className={cn(
                'text-4xl font-mono font-black',
                hasUnlimited || bypassBilling ? 'text-emerald-400' :
                (creditBalance ?? 1) <= 0 ? 'text-red-400' :
                (creditBalance ?? 99) < 2 ? 'text-yellow-400' : 'text-white'
              )}>
                {creditDisplay}
              </div>
              <div className="max-w-2xl text-sm leading-6 text-gray-300">
                {planNarrative(currentPlanType)} Credits pay for managed AI usage. Subscription tier unlocks capability boundaries like backend, publish, and BYOK.
              </div>
              <div className="flex flex-wrap gap-2">
                {planCallouts(currentPlanType).map((callout) => (
                  <div key={callout} className="rounded-full border border-gray-700 bg-gray-950/70 px-3 py-1.5 text-[11px] uppercase tracking-[0.14em] text-gray-300">
                    {callout}
                  </div>
                ))}
              </div>
            </div>

            <div className="grid min-w-[280px] gap-3">
              <div className="rounded-xl border border-gray-800 bg-gray-950/70 p-4">
                <div className="text-[11px] uppercase tracking-[0.18em] text-gray-500">Current plan</div>
                <div className="mt-2 text-lg font-semibold text-white">
                  {subscription?.plan_name || 'Free'}
                </div>
                {subscription?.status && subscription.status !== 'inactive' && (
                  <div className="mt-2">
                    <span className={cn(
                      'inline-flex rounded-full px-2 py-1 text-[10px] font-bold uppercase',
                      subscription.status === 'active' ? 'bg-emerald-500/20 text-emerald-400' :
                      subscription.status === 'past_due' ? 'bg-yellow-500/20 text-yellow-400' :
                      'bg-gray-700 text-gray-400'
                    )}>
                      {subscription.status}
                    </span>
                  </div>
                )}
                {subscription?.current_period_end && (
                  <div className="mt-2 text-xs text-gray-400">
                    Renews {new Date(subscription.current_period_end).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
                  </div>
                )}
              </div>

              <div className="flex flex-wrap gap-2">
                {!hasUnlimited && !bypassBilling && (
                  <button
                    onClick={() => setShowBuyCredits(true)}
                    className="flex flex-1 items-center justify-center gap-2 rounded-xl bg-red-600 px-4 py-3 text-sm font-semibold text-white transition hover:bg-red-500"
                  >
                    <CreditCard className="w-4 h-4" />
                    Buy credits
                  </button>
                )}
                {subscription?.plan_type !== 'free' && (
                  <button
                    onClick={handleManageSubscription}
                    disabled={portalLoading}
                    className="flex flex-1 items-center justify-center gap-2 rounded-xl border border-gray-700 bg-gray-900/70 px-4 py-3 text-sm font-medium text-gray-200 transition hover:border-gray-600 hover:text-white"
                  >
                    {portalLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <ExternalLink className="w-4 h-4" />}
                    Manage subscription
                  </button>
                )}
              </div>
            </div>
          </div>
        </div>

        <div className="border-t border-gray-800 bg-gray-950/50 px-5 py-4">
          {currentPlanType === 'free' ? (
            <div className="rounded-xl border border-amber-500/20 bg-amber-500/8 px-4 py-3 text-sm leading-6 text-amber-100/85">
              Free accounts can build static frontend websites and UI mockups. Upgrade to Builder or higher to unlock backend, database, auth, billing, realtime, deployment, publish, and BYOK. Credit packs extend managed usage but do not unlock those paid capabilities on their own.
            </div>
          ) : (
            <div className="rounded-xl border border-emerald-500/20 bg-emerald-500/8 px-4 py-3 text-sm leading-6 text-emerald-100/85">
              Your subscription unlocks app capability boundaries. Credits cover managed model usage inside that entitlement instead of acting as the entitlement itself.
            </div>
          )}
        </div>
      </div>

      {/* Plan Upgrade Grid */}
      <div>
        <div className="mb-3 flex items-center justify-between">
          <div>
            <h3 className="text-sm font-semibold text-white">Plans</h3>
            <div className="mt-1 text-xs text-gray-500">Free is for websites. Monthly plans unlock real apps. Credits handle usage above the included monthly runway.</div>
          </div>
          <button onClick={load} className="text-xs text-gray-500 hover:text-gray-300 flex items-center gap-1 transition-colors">
            <RefreshCw className="w-3 h-3" /> Refresh
          </button>
        </div>
        <div className="grid grid-cols-1 gap-3 xl:grid-cols-4">
          {plans.map(plan => {
            const isCurrent = plan.type === currentPlanType
            const isUpgrade = !isCurrent
            const priceStr = plan.monthly_price_cents === 0
              ? 'Free'
              : `$${(plan.monthly_price_cents / 100).toFixed(0)}/mo`

            return (
              <div
                key={plan.type}
                className={cn(
                  'relative flex flex-col gap-4 rounded-2xl border p-5 transition-all shadow-[0_24px_60px_rgba(0,0,0,0.18)]',
                  isCurrent
                    ? 'border-emerald-500/40 bg-emerald-500/7'
                    : plan.is_popular
                      ? 'border-green-500/30 bg-green-500/7'
                      : 'border-gray-800 bg-gray-900/50'
                )}
              >
                {plan.is_popular && !isCurrent && (
                  <div className="absolute -top-2.5 left-4 text-[10px] bg-green-500 text-black font-black px-2 py-0.5 rounded-full uppercase">
                    Popular
                  </div>
                )}
                {isCurrent && (
                  <div className="absolute -top-2.5 left-4 text-[10px] bg-emerald-500 text-black font-black px-2 py-0.5 rounded-full uppercase">
                    Current
                  </div>
                )}

                <div className="flex items-start justify-between">
                  <div>
                    <div className="font-bold text-white text-sm">{plan.name}</div>
                    <div className="text-lg font-black font-mono text-white mt-0.5">{priceStr}</div>
                    {plan.monthly_credits_usd > 0 && (
                      <div className="text-[11px] text-emerald-400 mt-0.5">
                        + ${plan.monthly_credits_usd.toFixed(0)} credits / mo
                      </div>
                    )}
                    <div className="mt-2 max-w-xs text-xs leading-5 text-gray-400">
                      {planNarrative(plan.type)}
                    </div>
                  </div>

                  {isUpgrade && plan.type !== 'free' && (
                    <button
                      onClick={() => handleUpgrade(plan)}
                      disabled={upgradeLoading === plan.type}
                      className={cn(
                        'flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all',
                        plan.is_popular
                          ? 'bg-green-600 hover:bg-green-500 text-white'
                          : 'bg-gray-800 hover:bg-gray-700 text-gray-200 border border-gray-700'
                      )}
                    >
                      {upgradeLoading === plan.type
                        ? <Loader2 className="w-3 h-3 animate-spin" />
                        : <ChevronRight className="w-3 h-3" />
                      }
                      Upgrade
                    </button>
                  )}
                </div>

                <ul className="space-y-1.5">
                  {plan.features.slice(0, 5).map(f => (
                    <li key={f} className="flex items-start gap-1.5 text-[11px] leading-5 text-gray-300">
                      <Check className="w-3 h-3 text-emerald-400 flex-shrink-0 mt-0.5" />
                      {f}
                    </li>
                  ))}
                </ul>

                <div className="mt-auto rounded-xl border border-gray-800 bg-black/25 px-3 py-3">
                  <div className="text-[10px] uppercase tracking-[0.16em] text-gray-500">Best for</div>
                  <div className="mt-2 text-xs leading-5 text-gray-300">
                    {plan.type === 'free' && 'Landing pages, UI mockups, and static marketing surfaces.'}
                    {plan.type === 'builder' && 'Early-stage app teams that need backend, auth, and deployment without enterprise overhead.'}
                    {plan.type === 'pro' && 'Founders and operators shipping production app changes every week.'}
                    {plan.type === 'team' && 'Collaborative product teams that want shared credit runway and heavier workflows.'}
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {/* Invoice History */}
      {invoices.length > 0 && (
        <div>
          <h3 className="text-sm font-semibold text-white mb-3">Invoice History</h3>
          <div className="rounded-xl border border-gray-800 overflow-hidden">
            {invoices.map((inv, i) => (
              <div
                key={inv.id}
                className={cn(
                  'flex items-center justify-between px-4 py-3 text-sm',
                  i > 0 && 'border-t border-gray-800'
                )}
              >
                <div>
                  <div className="text-white text-xs font-medium">
                    {inv.description || 'Subscription payment'}
                  </div>
                  <div className="text-gray-500 text-[11px] mt-0.5">
                    {new Date(inv.created * 1000).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <span className="font-mono text-white text-xs">
                    ${(inv.amount_paid / 100).toFixed(2)}
                  </span>
                  <span className={cn(
                    'text-[10px] px-1.5 py-0.5 rounded uppercase font-bold',
                    inv.status === 'paid' ? 'bg-emerald-500/15 text-emerald-400' : 'bg-gray-800 text-gray-400'
                  )}>
                    {inv.status}
                  </span>
                  {inv.hosted_invoice_url && (
                    <a
                      href={inv.hosted_invoice_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-gray-500 hover:text-gray-300 transition-colors"
                    >
                      <ExternalLink className="w-3 h-3" />
                    </a>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {showBuyCredits && (
        <BuyCreditsModal onClose={() => { setShowBuyCredits(false); load() }} />
      )}
    </div>
  )
}

export default BillingSettings
