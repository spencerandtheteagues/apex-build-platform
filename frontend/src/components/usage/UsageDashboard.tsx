// APEX.BUILD Usage Dashboard
// Comprehensive usage tracking with plan quotas, warnings, and upgrade prompts

import React, { useState, useEffect, useCallback } from 'react'
import {
  DollarSign, TrendingUp, Activity, Zap, BarChart3,
  ChevronLeft, ChevronRight, Loader2, RefreshCw,
  AlertTriangle, AlertCircle, CheckCircle, ArrowUpRight,
  FolderOpen, HardDrive, Cpu, Sparkles, Crown, Infinity,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import apiService, {
  CurrentUsageData,
  UsageLimitsData,
  UsageWarning,
  PlanType
} from '@/services/api'

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

const PLAN_COLORS: Record<PlanType, { bg: string; border: string; text: string; badge: string }> = {
  free: { bg: 'bg-gray-500/10', border: 'border-gray-500/30', text: 'text-gray-400', badge: 'bg-gray-500/20' },
  pro: { bg: 'bg-blue-500/10', border: 'border-blue-500/30', text: 'text-blue-400', badge: 'bg-blue-500/20' },
  team: { bg: 'bg-purple-500/10', border: 'border-purple-500/30', text: 'text-purple-400', badge: 'bg-purple-500/20' },
  enterprise: { bg: 'bg-amber-500/10', border: 'border-amber-500/30', text: 'text-amber-400', badge: 'bg-amber-500/20' },
  owner: { bg: 'bg-red-500/10', border: 'border-red-500/30', text: 'text-red-400', badge: 'bg-red-500/20' },
}

const PLAN_NAMES: Record<PlanType, string> = {
  free: 'Free',
  pro: 'Pro',
  team: 'Team',
  enterprise: 'Enterprise',
  owner: 'Owner',
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

function formatNumber(n: number): string {
  if (n === -1) return 'Unlimited'
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

function formatBytes(bytes: number): string {
  if (bytes === -1) return 'Unlimited'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (bytes >= 1024 && i < units.length - 1) {
    bytes /= 1024
    i++
  }
  return `${bytes.toFixed(1)} ${units[i]}`
}

function formatMinutes(minutes: number): string {
  if (minutes === -1) return 'Unlimited'
  if (minutes >= 60) {
    const hours = Math.floor(minutes / 60)
    const mins = minutes % 60
    return mins > 0 ? `${hours}h ${mins}m` : `${hours} hours`
  }
  return `${minutes} minutes`
}

// Progress bar with gradient based on usage percentage
function UsageProgressBar({
  percentage,
  unlimited = false,
  size = 'normal'
}: {
  percentage: number
  unlimited?: boolean
  size?: 'normal' | 'small'
}) {
  const getBarColor = () => {
    if (unlimited) return 'bg-gradient-to-r from-emerald-500 to-emerald-400'
    if (percentage >= 100) return 'bg-gradient-to-r from-red-500 to-red-400'
    if (percentage >= 90) return 'bg-gradient-to-r from-orange-500 to-orange-400'
    if (percentage >= 80) return 'bg-gradient-to-r from-yellow-500 to-yellow-400'
    return 'bg-gradient-to-r from-blue-500 to-blue-400'
  }

  const height = size === 'small' ? 'h-1.5' : 'h-2.5'
  const displayPercentage = unlimited ? 100 : Math.min(percentage, 100)

  return (
    <div className={cn('w-full rounded-full bg-gray-800 overflow-hidden', height)}>
      <div
        className={cn('h-full rounded-full transition-all duration-700 ease-out', getBarColor())}
        style={{ width: `${Math.max(displayPercentage, 2)}%` }}
      />
    </div>
  )
}

// Individual quota card
function QuotaCard({
  icon: Icon,
  title,
  current,
  limit,
  percentage,
  unlimited,
  formatValue,
  period,
  iconColor,
}: {
  icon: React.ElementType
  title: string
  current: number
  limit: number
  percentage: number
  unlimited: boolean
  formatValue: (n: number) => string
  period?: string
  iconColor: string
}) {
  const getStatusIcon = () => {
    if (unlimited) return <Infinity className="w-4 h-4 text-emerald-400" />
    if (percentage >= 100) return <AlertCircle className="w-4 h-4 text-red-400" />
    if (percentage >= 90) return <AlertTriangle className="w-4 h-4 text-orange-400" />
    if (percentage >= 80) return <AlertTriangle className="w-4 h-4 text-yellow-400" />
    return <CheckCircle className="w-4 h-4 text-emerald-400" />
  }

  return (
    <div className="rounded-xl bg-gray-900/70 backdrop-blur-xl border border-gray-700/50 p-5">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Icon className={cn('w-4 h-4', iconColor)} />
          <span className="text-sm font-medium text-white">{title}</span>
        </div>
        {getStatusIcon()}
      </div>

      <div className="mb-3">
        <div className="flex items-baseline gap-1">
          <span className="text-2xl font-mono font-bold text-white">
            {formatValue(current)}
          </span>
          {!unlimited && (
            <>
              <span className="text-gray-500">/</span>
              <span className="text-sm text-gray-400">{formatValue(limit)}</span>
            </>
          )}
        </div>
        {period && (
          <div className="text-xs text-gray-500 mt-0.5">per {period}</div>
        )}
      </div>

      <UsageProgressBar percentage={percentage} unlimited={unlimited} />

      <div className="mt-2 flex items-center justify-between text-xs">
        <span className={cn(
          unlimited ? 'text-emerald-400' :
          percentage >= 100 ? 'text-red-400' :
          percentage >= 80 ? 'text-yellow-400' :
          'text-gray-500'
        )}>
          {unlimited ? 'Unlimited' : `${percentage.toFixed(0)}% used`}
        </span>
        {!unlimited && percentage < 100 && (
          <span className="text-gray-500">
            {formatValue(limit - current)} remaining
          </span>
        )}
      </div>
    </div>
  )
}

// Warning banner
function WarningBanner({ warnings }: { warnings: UsageWarning[] }) {
  if (warnings.length === 0) return null

  const criticalWarnings = warnings.filter(w => w.severity === 'critical')
  const highWarnings = warnings.filter(w => w.severity === 'high')
  const regularWarnings = warnings.filter(w => w.severity === 'warning')

  const getBannerStyle = () => {
    if (criticalWarnings.length > 0) return {
      bg: 'bg-red-500/10',
      border: 'border-red-500/30',
      icon: AlertCircle,
      iconColor: 'text-red-400',
    }
    if (highWarnings.length > 0) return {
      bg: 'bg-orange-500/10',
      border: 'border-orange-500/30',
      icon: AlertTriangle,
      iconColor: 'text-orange-400',
    }
    return {
      bg: 'bg-yellow-500/10',
      border: 'border-yellow-500/30',
      icon: AlertTriangle,
      iconColor: 'text-yellow-400',
    }
  }

  const style = getBannerStyle()
  const Icon = style.icon

  return (
    <div className={cn('rounded-xl border p-4', style.bg, style.border)}>
      <div className="flex items-start gap-3">
        <Icon className={cn('w-5 h-5 mt-0.5 shrink-0', style.iconColor)} />
        <div className="flex-1">
          <div className="font-medium text-white mb-2">
            {criticalWarnings.length > 0
              ? 'Usage Limit Reached'
              : 'Approaching Usage Limits'}
          </div>
          <ul className="space-y-1.5">
            {warnings.map((warning, i) => (
              <li key={i} className="text-sm text-gray-300 flex items-start gap-2">
                <span className={cn(
                  'shrink-0 w-1.5 h-1.5 rounded-full mt-1.5',
                  warning.severity === 'critical' ? 'bg-red-400' :
                  warning.severity === 'high' ? 'bg-orange-400' :
                  'bg-yellow-400'
                )} />
                {warning.message}
              </li>
            ))}
          </ul>
          <a
            href="/settings/billing"
            className="inline-flex items-center gap-1.5 mt-3 px-3 py-1.5 text-sm font-medium text-white bg-red-500 hover:bg-red-600 rounded-lg transition-colors"
          >
            Upgrade Plan
            <ArrowUpRight className="w-3.5 h-3.5" />
          </a>
        </div>
      </div>
    </div>
  )
}

// Plan comparison for upgrade prompt
function PlanComparison({
  currentPlan,
  limits
}: {
  currentPlan: PlanType
  limits: UsageLimitsData | null
}) {
  if (!limits || currentPlan === 'enterprise' || currentPlan === 'owner') return null

  const plans: PlanType[] = ['free', 'pro', 'team', 'enterprise']
  const currentIndex = plans.indexOf(currentPlan)
  const nextPlan = plans[currentIndex + 1] as PlanType

  if (!nextPlan || !limits.all_plans[nextPlan]) return null

  const nextLimits = limits.all_plans[nextPlan]
  const nextPricing = limits.pricing[nextPlan]

  return (
    <div className="rounded-xl bg-gradient-to-br from-red-500/10 via-purple-500/10 to-blue-500/10 border border-red-500/20 p-5">
      <div className="flex items-center gap-2 mb-4">
        <Crown className="w-5 h-5 text-amber-400" />
        <span className="font-semibold text-white">Upgrade to {PLAN_NAMES[nextPlan]}</span>
        <span className={cn(
          'px-2 py-0.5 text-xs font-medium rounded-full',
          PLAN_COLORS[nextPlan].badge,
          PLAN_COLORS[nextPlan].text
        )}>
          ${(nextPricing.price_monthly / 100).toFixed(0)}/mo
        </span>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
        <div>
          <div className="text-xs text-gray-500 mb-1">Projects</div>
          <div className="text-sm font-mono text-white">
            {nextLimits.projects === -1 ? 'Unlimited' : nextLimits.projects}
          </div>
        </div>
        <div>
          <div className="text-xs text-gray-500 mb-1">Storage</div>
          <div className="text-sm font-mono text-white">
            {formatBytes(nextLimits.storage_bytes)}
          </div>
        </div>
        <div>
          <div className="text-xs text-gray-500 mb-1">AI Requests/mo</div>
          <div className="text-sm font-mono text-white">
            {formatNumber(nextLimits.ai_requests)}
          </div>
        </div>
        <div>
          <div className="text-xs text-gray-500 mb-1">Exec Time/day</div>
          <div className="text-sm font-mono text-white">
            {formatMinutes(nextLimits.execution_minutes)}
          </div>
        </div>
      </div>

      <a
        href="/settings/billing"
        className="inline-flex items-center justify-center gap-2 w-full px-4 py-2.5 text-sm font-medium text-white bg-gradient-to-r from-red-500 to-purple-500 hover:from-red-600 hover:to-purple-600 rounded-lg transition-all"
      >
        Upgrade Now
        <ArrowUpRight className="w-4 h-4" />
      </a>
    </div>
  )
}

export default function UsageDashboard() {
  const [usage, setUsage] = useState<UsageData | null>(null)
  const [planUsage, setPlanUsage] = useState<CurrentUsageData | null>(null)
  const [limits, setLimits] = useState<UsageLimitsData | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [activeTab, setActiveTab] = useState<'quotas' | 'costs'>('quotas')
  const [monthKey, setMonthKey] = useState(() => {
    const now = new Date()
    return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
  })

  const fetchUsage = useCallback(async (showRefresh = false) => {
    if (showRefresh) setRefreshing(true)
    else setLoading(true)

    try {
      // Fetch all usage data in parallel
      const [byokResponse, currentUsageResponse, limitsResponse] = await Promise.allSettled([
        apiService.getBYOKUsage(monthKey),
        apiService.getCurrentUsage(),
        apiService.getUsageLimits(),
      ])

      // Handle BYOK usage (AI costs)
      if (byokResponse.status === 'fulfilled' && byokResponse.value.success) {
        setUsage({
          totalCost: byokResponse.value.data.total_cost,
          totalTokens: byokResponse.value.data.total_tokens,
          totalRequests: byokResponse.value.data.total_requests,
          byProvider: byokResponse.value.data.by_provider,
        })
      }

      // Handle plan usage quotas
      if (currentUsageResponse.status === 'fulfilled') {
        setPlanUsage(currentUsageResponse.value)
      }

      // Handle plan limits
      if (limitsResponse.status === 'fulfilled') {
        setLimits(limitsResponse.value)
      }
    } catch {
      // Silently handle errors - individual responses may still have succeeded
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

  const currentPlan = planUsage?.plan || 'free'
  const planStyle = PLAN_COLORS[currentPlan]

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-red-500/10 border border-red-500/30">
            <BarChart3 className="w-5 h-5 text-red-400" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h2 className="text-lg font-semibold text-white">Usage & Limits</h2>
              {planUsage && (
                <span className={cn(
                  'px-2 py-0.5 text-xs font-medium rounded-full',
                  planStyle.badge,
                  planStyle.text
                )}>
                  {PLAN_NAMES[currentPlan]} Plan
                </span>
              )}
            </div>
            <p className="text-sm text-gray-400">Track your plan quotas and AI spending</p>
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

      {/* Tab selector */}
      <div className="flex items-center gap-1 p-1 bg-gray-900/50 rounded-lg border border-gray-700/50 w-fit">
        <button
          onClick={() => setActiveTab('quotas')}
          className={cn(
            'px-4 py-1.5 text-sm font-medium rounded-md transition-all',
            activeTab === 'quotas'
              ? 'bg-red-500 text-white'
              : 'text-gray-400 hover:text-white'
          )}
        >
          Plan Quotas
        </button>
        <button
          onClick={() => setActiveTab('costs')}
          className={cn(
            'px-4 py-1.5 text-sm font-medium rounded-md transition-all',
            activeTab === 'costs'
              ? 'bg-red-500 text-white'
              : 'text-gray-400 hover:text-white'
          )}
        >
          AI Costs
        </button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-16">
          <Loader2 className="w-6 h-6 text-red-500 animate-spin" />
          <span className="ml-3 text-gray-400">Loading usage data...</span>
        </div>
      ) : activeTab === 'quotas' ? (
        <>
          {/* Warning banners */}
          {planUsage?.warnings && planUsage.warnings.length > 0 && (
            <WarningBanner warnings={planUsage.warnings} />
          )}

          {/* Quota cards */}
          {planUsage && (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
              <QuotaCard
                icon={FolderOpen}
                title="Projects"
                current={planUsage.usage.projects.current}
                limit={planUsage.usage.projects.limit}
                percentage={planUsage.usage.projects.percentage}
                unlimited={planUsage.usage.projects.unlimited}
                formatValue={formatNumber}
                iconColor="text-blue-400"
              />
              <QuotaCard
                icon={HardDrive}
                title="Storage"
                current={planUsage.usage.storage.current}
                limit={planUsage.usage.storage.limit}
                percentage={planUsage.usage.storage.percentage}
                unlimited={planUsage.usage.storage.unlimited}
                formatValue={formatBytes}
                iconColor="text-purple-400"
              />
              <QuotaCard
                icon={Sparkles}
                title="AI Requests"
                current={planUsage.usage.ai_requests.current}
                limit={planUsage.usage.ai_requests.limit}
                percentage={planUsage.usage.ai_requests.percentage}
                unlimited={planUsage.usage.ai_requests.unlimited}
                formatValue={formatNumber}
                period="month"
                iconColor="text-amber-400"
              />
              <QuotaCard
                icon={Cpu}
                title="Execution Time"
                current={planUsage.usage.execution_minutes.current}
                limit={planUsage.usage.execution_minutes.limit}
                percentage={planUsage.usage.execution_minutes.percentage}
                unlimited={planUsage.usage.execution_minutes.unlimited}
                formatValue={formatMinutes}
                period="day"
                iconColor="text-emerald-400"
              />
            </div>
          )}

          {/* Upgrade prompt (only for non-enterprise/owner plans) */}
          {planUsage && <PlanComparison currentPlan={currentPlan} limits={limits} />}

          {/* Plan comparison table for free users */}
          {currentPlan === 'free' && limits && (
            <div className="rounded-xl bg-gray-900/70 backdrop-blur-xl border border-gray-700/50 p-5">
              <h3 className="text-sm font-semibold text-white mb-4">Compare Plans</h3>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-gray-700">
                      <th className="text-left py-2 px-3 text-gray-400 font-medium">Feature</th>
                      <th className="text-center py-2 px-3 text-gray-400 font-medium">Free</th>
                      <th className="text-center py-2 px-3 text-blue-400 font-medium">Pro $12/mo</th>
                      <th className="text-center py-2 px-3 text-purple-400 font-medium">Team $29/mo</th>
                      <th className="text-center py-2 px-3 text-amber-400 font-medium">Enterprise $79/mo</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-800">
                    <tr>
                      <td className="py-2 px-3 text-gray-300">Projects</td>
                      <td className="py-2 px-3 text-center text-gray-400">{limits.all_plans.free?.projects || 3}</td>
                      <td className="py-2 px-3 text-center text-white">{limits.all_plans.pro?.projects || 25}</td>
                      <td className="py-2 px-3 text-center text-white">{limits.all_plans.team?.projects || 100}</td>
                      <td className="py-2 px-3 text-center text-emerald-400">Unlimited</td>
                    </tr>
                    <tr>
                      <td className="py-2 px-3 text-gray-300">Storage</td>
                      <td className="py-2 px-3 text-center text-gray-400">100 MB</td>
                      <td className="py-2 px-3 text-center text-white">5 GB</td>
                      <td className="py-2 px-3 text-center text-white">25 GB</td>
                      <td className="py-2 px-3 text-center text-emerald-400">Unlimited</td>
                    </tr>
                    <tr>
                      <td className="py-2 px-3 text-gray-300">AI Requests/month</td>
                      <td className="py-2 px-3 text-center text-gray-400">1,000</td>
                      <td className="py-2 px-3 text-center text-white">10,000</td>
                      <td className="py-2 px-3 text-center text-white">50,000</td>
                      <td className="py-2 px-3 text-center text-emerald-400">Unlimited</td>
                    </tr>
                    <tr>
                      <td className="py-2 px-3 text-gray-300">Execution Time/day</td>
                      <td className="py-2 px-3 text-center text-gray-400">10 min</td>
                      <td className="py-2 px-3 text-center text-white">2 hours</td>
                      <td className="py-2 px-3 text-center text-white">8 hours</td>
                      <td className="py-2 px-3 text-center text-emerald-400">Unlimited</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      ) : (
        <>
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
