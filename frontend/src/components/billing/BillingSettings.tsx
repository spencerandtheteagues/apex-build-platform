// APEX.BUILD — Billing Settings
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
        // Filter to just the paid plans shown on the landing page
        const featured = ['free', 'builder', 'pro', 'team']
        setPlans(plansRes.value.data.plans.filter(p => featured.includes(p.type)))
      }

      if (subRes.status === 'fulfilled' && subRes.value.success && subRes.value.data) {
        setSubscription(subRes.value.data)
      }

      if (balRes.status === 'fulfilled' && balRes.value.success && balRes.value.data) {
        setCreditBalance(balRes.value.data.balance)
        setHasUnlimited(balRes.value.data.has_unlimited)
        setBypassBilling(balRes.value.data.bypass_billing)
      }

      if (invRes.status === 'fulfilled' && invRes.value.success && invRes.value.data) {
        setInvoices(invRes.value.data.invoices)
      }
    } catch {
      setError('Failed to load billing information.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleUpgrade = async (plan: Plan) => {
    if (!plan.monthly_price_id || plan.monthly_price_id.startsWith('price_') && !plan.monthly_price_id.includes('_test_')) {
      // price IDs not configured — open portal or show info
      setError('Stripe is not configured in this environment. Set STRIPE_PRICE_* environment variables.')
      return
    }
    setUpgradeLoading(plan.type)
    setError(null)
    try {
      const result = await apiService.createCheckoutSession({
        price_id: plan.monthly_price_id,
        success_url: window.location.href + '?billing=success',
        cancel_url: window.location.href + '?billing=cancel',
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

      {error && (
        <div className="flex items-start gap-3 p-3 bg-red-500/10 border border-red-500/25 rounded-lg text-sm text-red-400">
          <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>{error}</span>
          <button onClick={() => setError(null)} className="ml-auto text-red-400/60 hover:text-red-400">✕</button>
        </div>
      )}

      {/* Credit Balance Card */}
      <div className="bg-gray-900/60 border border-gray-800 rounded-xl p-5">
        <div className="flex items-center justify-between">
          <div>
            <div className="text-xs text-gray-500 uppercase mb-1 flex items-center gap-1.5">
              <Zap className="w-3 h-3" /> AI Credit Balance
            </div>
            <div className={cn(
              'text-3xl font-mono font-black',
              hasUnlimited || bypassBilling ? 'text-emerald-400' :
              (creditBalance ?? 1) <= 0 ? 'text-red-400' :
              (creditBalance ?? 99) < 2 ? 'text-yellow-400' : 'text-white'
            )}>
              {creditDisplay}
            </div>
            {!hasUnlimited && !bypassBilling && (
              <div className="text-xs text-gray-500 mt-1">
                Used to pay for AI model calls during builds
              </div>
            )}
          </div>
          {!hasUnlimited && !bypassBilling && (
            <button
              onClick={() => setShowBuyCredits(true)}
              className="flex items-center gap-2 px-4 py-2 bg-red-600 hover:bg-red-500 text-white text-sm font-semibold rounded-lg transition-colors"
            >
              <CreditCard className="w-4 h-4" />
              Buy Credits
            </button>
          )}
        </div>

        {subscription && subscription.plan_name && (
          <div className="mt-4 pt-4 border-t border-gray-800 flex items-center justify-between">
            <div>
              <span className="text-xs text-gray-500 uppercase">Current Plan</span>
              <div className="text-sm font-semibold text-white mt-0.5">
                {subscription.plan_name}
                {subscription.status && subscription.status !== 'inactive' && (
                  <span className={cn(
                    'ml-2 text-[10px] px-1.5 py-0.5 rounded-full uppercase font-bold',
                    subscription.status === 'active' ? 'bg-emerald-500/20 text-emerald-400' :
                    subscription.status === 'past_due' ? 'bg-yellow-500/20 text-yellow-400' :
                    'bg-gray-700 text-gray-400'
                  )}>
                    {subscription.status}
                  </span>
                )}
              </div>
            </div>
            {subscription.plan_type !== 'free' && (
              <button
                onClick={handleManageSubscription}
                disabled={portalLoading}
                className="flex items-center gap-1.5 text-xs text-gray-400 hover:text-white transition-colors"
              >
                {portalLoading ? <Loader2 className="w-3 h-3 animate-spin" /> : <ExternalLink className="w-3 h-3" />}
                Manage subscription
              </button>
            )}
          </div>
        )}
      </div>

      {/* Plan Upgrade Grid */}
      <div>
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-semibold text-white">Plans</h3>
          <button onClick={load} className="text-xs text-gray-500 hover:text-gray-300 flex items-center gap-1 transition-colors">
            <RefreshCw className="w-3 h-3" /> Refresh
          </button>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
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
                  'relative rounded-xl border p-4 flex flex-col gap-3 transition-all',
                  isCurrent
                    ? 'border-emerald-500/40 bg-emerald-500/5'
                    : plan.is_popular
                      ? 'border-green-500/30 bg-green-500/5'
                      : 'border-gray-800 bg-gray-900/40'
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

                <ul className="space-y-1">
                  {plan.features.slice(0, 4).map(f => (
                    <li key={f} className="flex items-start gap-1.5 text-[11px] text-gray-400">
                      <Check className="w-3 h-3 text-emerald-400 flex-shrink-0 mt-0.5" />
                      {f}
                    </li>
                  ))}
                </ul>
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
