// APEX.BUILD Cost Ticker
// Real-time cost display for IDE status bar with expandable breakdown

import React, { useState, useEffect, useCallback, useRef } from 'react'
import { DollarSign, TrendingUp, ChevronUp, Activity, Zap, Plus } from 'lucide-react'
import { cn } from '@/lib/utils'
import apiService from '@/services/api'
import { BuyCreditsModal } from '@/components/billing/BuyCreditsModal'

interface UsageData {
  totalCost: number
  totalTokens: number
  totalRequests: number
  byokCost?: number
  platformCost?: number
  byProvider: Record<string, {
    provider: string
    cost: number
    tokens: number
    requests: number
    byok_requests: number
  }>
}

interface BillingData {
  creditBalance: number
  hasUnlimitedCredits: boolean
  bypassBilling: boolean
}

interface CostTickerProps {
  className?: string
}

export default function CostTicker({ className }: CostTickerProps) {
  const [isExpanded, setIsExpanded] = useState(false)
  const [usage, setUsage] = useState<UsageData | null>(null)
  const [billing, setBilling] = useState<BillingData | null>(null)
  const [loading, setLoading] = useState(true)
  const [showBuyCredits, setShowBuyCredits] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const fetchUsage = useCallback(async () => {
    try {
      const response = await apiService.getBYOKUsage()
      if (response.success) {
        setUsage({
          totalCost: response.data.total_cost,
          totalTokens: response.data.total_tokens,
          totalRequests: response.data.total_requests,
          byokCost: response.data.byok_cost,
          platformCost: response.data.platform_cost,
          byProvider: response.data.by_provider,
        })
        if (response.billing) {
          setBilling({
            creditBalance: response.billing.credit_balance,
            hasUnlimitedCredits: response.billing.has_unlimited_credits,
            bypassBilling: response.billing.bypass_billing,
          })
        }
      }
    } catch {
      // Usage tracking might not be available
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchUsage()
    // Refresh every 10 seconds for near real-time billing
    const interval = setInterval(fetchUsage, 10000)
    return () => clearInterval(interval)
  }, [fetchUsage])

  // Close on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsExpanded(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // Cost color coding
  const getCostColor = (cost: number) => {
    if (cost < 1) return 'text-green-400'
    if (cost < 5) return 'text-yellow-400'
    return 'text-red-400'
  }

  const monthCost = usage?.totalCost ?? 0
  const monthColor = getCostColor(monthCost)
  const creditDisplay = billing?.hasUnlimitedCredits || billing?.bypassBilling
    ? 'Unlimited'
    : `$${(billing?.creditBalance ?? 0).toFixed(2)}`

  if (loading) {
    return (
      <div className={cn('flex items-center gap-1.5 px-2 py-1 text-xs text-gray-500', className)}>
        <Activity className="w-3 h-3 animate-pulse" />
        <span>--</span>
      </div>
    )
  }

  return (
    <div ref={containerRef} className={cn('relative', className)}>
      {/* Compact ticker */}
      <button
        type="button"
        onClick={() => setIsExpanded(!isExpanded)}
        className={cn(
          'flex items-center gap-2 px-2.5 py-1 rounded-md text-xs transition-all duration-200',
          'hover:bg-gray-800/70 border border-transparent',
          isExpanded && 'bg-gray-800/70 border-gray-700/50'
        )}
      >
        <DollarSign className={cn('w-3 h-3', monthColor)} />
        <span className="text-gray-400">Credits:</span>
        <span className={cn(
          'font-mono font-medium',
          billing?.hasUnlimitedCredits ? 'text-emerald-400' :
          (billing?.creditBalance ?? 1) <= 0 ? 'text-red-400' :
          (billing?.creditBalance ?? 99) < 2 ? 'text-yellow-400' : 'text-white'
        )}>
          {creditDisplay}
        </span>
        {!billing?.hasUnlimitedCredits && !billing?.bypassBilling && (billing?.creditBalance ?? 1) <= 1 && (
          <button
            onClick={e => { e.stopPropagation(); setShowBuyCredits(true) }}
            className="flex items-center gap-0.5 text-[10px] text-red-400 hover:text-red-300 border border-red-500/30 rounded px-1.5 py-0.5 transition-colors"
            title="Buy more credits"
          >
            <Plus className="w-2.5 h-2.5" /> Add
          </button>
        )}
        <span className="text-gray-600">|</span>
        <span className="text-gray-400">Month:</span>
        <span className={cn('font-mono font-medium', monthColor)}>
          ${monthCost.toFixed(2)}
        </span>
        {usage && usage.totalRequests > 0 && (
          <>
            <span className="text-gray-600">|</span>
            <Zap className="w-3 h-3 text-gray-500" />
            <span className="text-gray-400 font-mono">{usage.totalRequests}</span>
          </>
        )}
        <ChevronUp
          className={cn(
            'w-3 h-3 text-gray-500 transition-transform',
            !isExpanded && 'rotate-180'
          )}
        />
      </button>

      {/* Expanded panel */}
      {isExpanded && (
        <div
          className={cn(
            'absolute bottom-full left-0 mb-2 z-50 w-[320px]',
            'bg-gray-900/95 backdrop-blur-xl border border-gray-700/70 rounded-xl',
            'shadow-2xl shadow-black/50',
            'animate-in fade-in slide-in-from-bottom-2 duration-150'
          )}
        >
          {/* Header */}
          <div className="px-4 py-3 border-b border-gray-800/50">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-white flex items-center gap-2">
                <TrendingUp className="w-4 h-4 text-red-400" />
                Usage Overview
              </h3>
              <span className="text-[10px] text-gray-500 uppercase">
                {new Date().toLocaleDateString('en-US', { month: 'short', year: 'numeric' })}
              </span>
            </div>
          </div>

          {/* Summary */}
          <div className="grid grid-cols-3 gap-3 px-4 py-3 border-b border-gray-800/50">
            <div>
              <div className="text-[10px] text-gray-500 uppercase mb-0.5">Total Cost</div>
              <div className={cn('text-lg font-mono font-bold', monthColor)}>
                ${monthCost.toFixed(2)}
              </div>
            </div>
            <div>
              <div className="text-[10px] text-gray-500 uppercase mb-0.5">Credits</div>
              <div className={cn('text-lg font-mono font-bold', billing?.hasUnlimitedCredits ? 'text-emerald-400' : 'text-white')}>
                {creditDisplay}
              </div>
            </div>
            <div>
              <div className="text-[10px] text-gray-500 uppercase mb-0.5">Requests</div>
              <div className="text-lg font-mono font-bold text-white">
                {usage?.totalRequests ?? 0}
              </div>
            </div>
          </div>

          {(usage?.platformCost !== undefined || usage?.byokCost !== undefined) && (
            <div className="px-4 py-2 text-[10px] text-gray-500 border-b border-gray-800/50">
              Platform: ${(usage?.platformCost ?? 0).toFixed(2)} · BYOK: ${(usage?.byokCost ?? 0).toFixed(2)}
            </div>
          )}

          {/* Per-provider breakdown */}
          {usage?.byProvider && Object.keys(usage.byProvider).length > 0 && (
            <div className="px-4 py-3">
              <div className="text-[10px] text-gray-500 uppercase mb-2">By Provider</div>
              <div className="space-y-2">
                {Object.entries(usage.byProvider).map(([key, data]) => {
                  const percent = monthCost > 0 ? (data.cost / monthCost) * 100 : 0
                  const providerColors: Record<string, string> = {
                    claude: 'bg-orange-500',
                    gpt4: 'bg-green-500',
                    gemini: 'bg-blue-500',
                    grok: 'bg-purple-500',
                  }
                  const barColor = providerColors[key] || 'bg-gray-500'

                  return (
                    <div key={key} className="space-y-1">
                      <div className="flex items-center justify-between text-xs">
                        <span className="text-gray-300 capitalize">{data.provider || key}</span>
                        <div className="flex items-center gap-3 text-gray-400">
                          <span>{data.requests} req</span>
                          <span className="font-mono">${data.cost.toFixed(4)}</span>
                          {data.byok_requests > 0 && (
                            <span className="text-[10px] px-1 py-0.5 rounded bg-red-500/10 text-red-400 border border-red-500/20">
                              BYOK
                            </span>
                          )}
                        </div>
                      </div>
                      {/* Cost bar */}
                      <div className="h-1 rounded-full bg-gray-800 overflow-hidden">
                        <div
                          className={cn('h-full rounded-full transition-all duration-500', barColor)}
                          style={{ width: `${Math.max(percent, 2)}%` }}
                        />
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          )}

          {/* No data state */}
          {(!usage?.byProvider || Object.keys(usage.byProvider).length === 0) && (
            <div className="px-4 py-6 text-center text-xs text-gray-500">
              No usage data yet this month.
            </div>
          )}

          {/* Footer */}
          <div className="px-4 py-2 border-t border-gray-800/50 text-[10px] text-gray-600 flex items-center justify-between">
            <span>Updates every 10s</span>
            <div className="flex items-center gap-3">
              {!billing?.hasUnlimitedCredits && !billing?.bypassBilling && (
                <button
                  onClick={() => { setIsExpanded(false); setShowBuyCredits(true) }}
                  className="flex items-center gap-1 text-[10px] text-red-400 hover:text-red-300 transition-colors"
                >
                  <Plus className="w-2.5 h-2.5" /> Buy Credits
                </button>
              )}
              <span className="flex items-center gap-1">
                <div className="w-1.5 h-1.5 rounded-full bg-green-500 animate-pulse" />
                Live
              </span>
            </div>
          </div>
        </div>
      )}

      {showBuyCredits && (
        <BuyCreditsModal onClose={() => setShowBuyCredits(false)} />
      )}
    </div>
  )
}
