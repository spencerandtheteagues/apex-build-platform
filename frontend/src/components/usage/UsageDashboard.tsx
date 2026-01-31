// APEX.BUILD Usage Dashboard
// Transparent cost tracking with per-provider breakdowns

import React, { useState, useEffect, useCallback } from 'react'
import {
  DollarSign, TrendingUp, Activity, Zap, BarChart3,
  ChevronLeft, ChevronRight, Loader2, RefreshCw,
} from 'lucide-react'
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

const PROVIDER_COLORS: Record<string, { bg: string; bar: string; text: string; label: string }> = {
  claude: { bg: 'bg-orange-500/10', bar: 'bg-orange-500', text: 'text-orange-400', label: 'Claude' },
  gpt4: { bg: 'bg-green-500/10', bar: 'bg-green-500', text: 'text-green-400', label: 'OpenAI' },
  gemini: { bg: 'bg-blue-500/10', bar: 'bg-blue-500', text: 'text-blue-400', label: 'Gemini' },
  grok: { bg: 'bg-purple-500/10', bar: 'bg-purple-500', text: 'text-purple-400', label: 'Grok' },
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toString()
}

function formatMonth(key: string): string {
  const [year, month] = key.split('-')
  const date = new Date(Number(year), Number(month) - 1)
  return date.toLocaleDateString('en-US', { month: 'long', year: 'numeric' })
}

export default function UsageDashboard() {
  const [usage, setUsage] = useState<UsageData | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [monthKey, setMonthKey] = useState(() => {
    const now = new Date()
    return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
  })

  const fetchUsage = useCallback(async (showRefresh = false) => {
    if (showRefresh) setRefreshing(true)
    else setLoading(true)

    try {
      const response = await apiService.getBYOKUsage(monthKey)
      if (response.success) {
        setUsage({
          totalCost: response.data.total_cost,
          totalTokens: response.data.total_tokens,
          totalRequests: response.data.total_requests,
          byProvider: response.data.by_provider,
        })
      }
    } catch {
      setUsage(null)
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [monthKey])

  useEffect(() => {
    fetchUsage()
  }, [fetchUsage])

  const navigateMonth = (direction: -1 | 1) => {
    const [year, month] = monthKey.split('-').map(Number)
    const date = new Date(year, month - 1 + direction)
    setMonthKey(`${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`)
  }

  const isCurrentMonth = () => {
    const now = new Date()
    return monthKey === `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
  }

  // Cost color
  const getCostColor = (cost: number) => {
    if (cost < 1) return 'text-green-400'
    if (cost < 5) return 'text-yellow-400'
    return 'text-red-400'
  }

  const providers = usage?.byProvider ? Object.entries(usage.byProvider) : []
  const maxProviderCost = providers.length > 0
    ? Math.max(...providers.map(([, d]) => d.cost))
    : 0

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-red-500/10 border border-red-500/30">
            <BarChart3 className="w-5 h-5 text-red-400" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-white">Usage & Costs</h2>
            <p className="text-sm text-gray-400">Track your AI usage and spending transparently</p>
          </div>
        </div>
        <button
          onClick={() => fetchUsage(true)}
          disabled={refreshing}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-gray-400 hover:text-white border border-gray-700 hover:border-gray-600 rounded-lg transition-all"
        >
          <RefreshCw className={cn('w-3.5 h-3.5', refreshing && 'animate-spin')} />
          Refresh
        </button>
      </div>

      {/* Month navigation */}
      <div className="flex items-center justify-center gap-4">
        <button
          onClick={() => navigateMonth(-1)}
          className="p-1.5 rounded-lg border border-gray-700 hover:border-gray-600 text-gray-400 hover:text-white transition-all"
        >
          <ChevronLeft className="w-4 h-4" />
        </button>
        <span className="text-sm font-medium text-white min-w-[160px] text-center">
          {formatMonth(monthKey)}
        </span>
        <button
          onClick={() => navigateMonth(1)}
          disabled={isCurrentMonth()}
          className="p-1.5 rounded-lg border border-gray-700 hover:border-gray-600 text-gray-400 hover:text-white transition-all disabled:opacity-30 disabled:cursor-not-allowed"
        >
          <ChevronRight className="w-4 h-4" />
        </button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-16">
          <Loader2 className="w-6 h-6 text-red-500 animate-spin" />
          <span className="ml-3 text-gray-400">Loading usage data...</span>
        </div>
      ) : (
        <>
          {/* Summary Cards */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {/* Total Cost */}
            <div className="rounded-xl bg-gray-900/70 backdrop-blur-xl border border-gray-700/50 p-5">
              <div className="flex items-center gap-2 mb-3">
                <DollarSign className="w-4 h-4 text-red-400" />
                <span className="text-xs text-gray-400 uppercase tracking-wider">Total Cost</span>
              </div>
              <div className={cn('text-3xl font-mono font-bold', getCostColor(usage?.totalCost ?? 0))}>
                ${(usage?.totalCost ?? 0).toFixed(2)}
              </div>
              <div className="mt-1 text-xs text-gray-500">
                {isCurrentMonth() ? 'This month so far' : formatMonth(monthKey)}
              </div>
            </div>

            {/* Total Requests */}
            <div className="rounded-xl bg-gray-900/70 backdrop-blur-xl border border-gray-700/50 p-5">
              <div className="flex items-center gap-2 mb-3">
                <Zap className="w-4 h-4 text-yellow-400" />
                <span className="text-xs text-gray-400 uppercase tracking-wider">Requests</span>
              </div>
              <div className="text-3xl font-mono font-bold text-white">
                {usage?.totalRequests ?? 0}
              </div>
              <div className="mt-1 text-xs text-gray-500">AI API calls made</div>
            </div>

            {/* Total Tokens */}
            <div className="rounded-xl bg-gray-900/70 backdrop-blur-xl border border-gray-700/50 p-5">
              <div className="flex items-center gap-2 mb-3">
                <Activity className="w-4 h-4 text-blue-400" />
                <span className="text-xs text-gray-400 uppercase tracking-wider">Tokens Used</span>
              </div>
              <div className="text-3xl font-mono font-bold text-white">
                {formatTokens(usage?.totalTokens ?? 0)}
              </div>
              <div className="mt-1 text-xs text-gray-500">Input + output tokens</div>
            </div>
          </div>

          {/* Provider Breakdown */}
          <div className="rounded-xl bg-gray-900/70 backdrop-blur-xl border border-gray-700/50 p-5">
            <div className="flex items-center gap-2 mb-5">
              <TrendingUp className="w-4 h-4 text-red-400" />
              <span className="text-sm font-semibold text-white">Cost by Provider</span>
            </div>

            {providers.length > 0 ? (
              <div className="space-y-4">
                {providers.map(([key, data]) => {
                  const colors = PROVIDER_COLORS[key] || PROVIDER_COLORS.claude
                  const costPercent = maxProviderCost > 0 ? (data.cost / maxProviderCost) * 100 : 0
                  const totalPercent = (usage?.totalCost ?? 0) > 0
                    ? (data.cost / (usage?.totalCost ?? 1)) * 100
                    : 0

                  return (
                    <div key={key} className="space-y-2">
                      {/* Provider label row */}
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <div className={cn('w-3 h-3 rounded-full', colors.bar)} />
                          <span className={cn('text-sm font-medium', colors.text)}>
                            {colors.label}
                          </span>
                          {data.byok_requests > 0 && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-red-500/10 text-red-400 border border-red-500/20">
                              BYOK
                            </span>
                          )}
                        </div>
                        <span className="text-sm font-mono text-white">
                          ${data.cost.toFixed(4)}
                          <span className="text-gray-500 ml-1 text-xs">
                            ({totalPercent.toFixed(0)}%)
                          </span>
                        </span>
                      </div>

                      {/* Cost bar */}
                      <div className="h-2.5 rounded-full bg-gray-800 overflow-hidden">
                        <div
                          className={cn('h-full rounded-full transition-all duration-700 ease-out', colors.bar)}
                          style={{ width: `${Math.max(costPercent, 1)}%` }}
                        />
                      </div>

                      {/* Stats row */}
                      <div className="flex gap-6 text-xs text-gray-500">
                        <span>{data.requests} requests</span>
                        <span>{formatTokens(data.tokens)} tokens</span>
                        {data.byok_requests > 0 && (
                          <span className="text-red-400/70">{data.byok_requests} via your key</span>
                        )}
                      </div>
                    </div>
                  )
                })}
              </div>
            ) : (
              <div className="py-10 text-center text-gray-500 text-sm">
                No usage data for {formatMonth(monthKey)}.
              </div>
            )}
          </div>

          {/* Cost efficiency tip */}
          <div className="rounded-xl bg-red-500/5 border border-red-500/20 p-4 flex items-start gap-3">
            <DollarSign className="w-5 h-5 text-red-400 mt-0.5 shrink-0" />
            <div className="text-sm text-gray-300">
              <strong className="text-red-400">Cost tip:</strong> Using your own API keys (BYOK) means you
              pay the provider directly at their standard rates with zero platform markup. Configure your keys
              in <span className="text-white">Settings &gt; API Keys</span> for unlimited usage.
            </div>
          </div>
        </>
      )}
    </div>
  )
}
