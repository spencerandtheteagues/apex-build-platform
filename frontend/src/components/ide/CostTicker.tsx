// APEX.BUILD Cost Ticker
// Real-time cost display for IDE status bar with expandable breakdown

import React, { useState, useEffect, useCallback, useRef } from 'react'
import { DollarSign, TrendingUp, ChevronUp, Activity, Zap } from 'lucide-react'
import { cn } from '@/lib/utils'
import apiService from '@/services/api'

interface UsageData {
  totalCost: number
  totalTokens: number
  totalRequests: number
  byProvider: Record<string, {
    provider: string
    cost: number
    tokens: number
    requests: number
    byok_requests: number
  }>
}

interface CostTickerProps {
  className?: string
}

export default function CostTicker({ className }: CostTickerProps) {
  const [isExpanded, setIsExpanded] = useState(false)
  const [usage, setUsage] = useState<UsageData | null>(null)
  const [sessionCost, setSessionCost] = useState(0)
  const [loading, setLoading] = useState(true)
  const containerRef = useRef<HTMLDivElement>(null)
  const sessionStartRef = useRef<number>(Date.now())

  const fetchUsage = useCallback(async () => {
    try {
      const response = await apiService.getBYOKUsage()
      if (response.success) {
        setUsage({
          totalCost: response.data.total_cost,
          totalTokens: response.data.total_tokens,
          totalRequests: response.data.total_requests,
          byProvider: response.data.by_provider,
        })
      }
    } catch {
      // Usage tracking might not be available
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchUsage()
    // Refresh every 30 seconds
    const interval = setInterval(fetchUsage, 30000)
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
              <div className="text-[10px] text-gray-500 uppercase mb-0.5">Requests</div>
              <div className="text-lg font-mono font-bold text-white">
                {usage?.totalRequests ?? 0}
              </div>
            </div>
            <div>
              <div className="text-[10px] text-gray-500 uppercase mb-0.5">Tokens</div>
              <div className="text-lg font-mono font-bold text-white">
                {usage?.totalTokens
                  ? usage.totalTokens > 1000000
                    ? `${(usage.totalTokens / 1000000).toFixed(1)}M`
                    : usage.totalTokens > 1000
                    ? `${(usage.totalTokens / 1000).toFixed(1)}K`
                    : usage.totalTokens.toString()
                  : '0'}
              </div>
            </div>
          </div>

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
            <span>Updates every 30s</span>
            <span className="flex items-center gap-1">
              <div className="w-1.5 h-1.5 rounded-full bg-green-500 animate-pulse" />
              Live
            </span>
          </div>
        </div>
      )}
    </div>
  )
}
