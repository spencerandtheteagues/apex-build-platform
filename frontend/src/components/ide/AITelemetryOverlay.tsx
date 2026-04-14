import React, { useMemo } from 'react'
import { Activity, ShieldCheck, Target, Workflow } from 'lucide-react'
import { cn } from '@/lib/utils'
import BuildActivityTimeline, {
  type GlassBoxPatchBundle,
  type GlassBoxThought,
  type GlassBoxVerificationReport,
  type GlassBoxWorkOrder,
} from './BuildActivityTimeline'
import HydraRacePanel, { type HydraProviderPanel } from './HydraRacePanel'
import type { BuildLearningSummaryState } from '@/services/api'

export interface GlassBoxProviderScorecard {
  provider: string
  task_shape: string
  compile_pass_rate?: number
  first_pass_verification_pass_rate?: number
  repair_success_rate?: number
  average_latency_seconds?: number
  average_cost_per_success?: number
}

interface AITelemetryOverlayProps {
  buildStatus: string
  currentPhase?: string
  qualityGateStatus?: string
  qualityGateStage?: string
  aiThoughts: GlassBoxThought[]
  providerPanels: HydraProviderPanel[]
  workOrders?: GlassBoxWorkOrder[]
  verificationReports?: GlassBoxVerificationReport[]
  patchBundles?: GlassBoxPatchBundle[]
  providerScorecards?: GlassBoxProviderScorecard[]
  historicalLearning?: BuildLearningSummaryState
}

const percent = (value?: number): string => {
  if (typeof value !== 'number' || Number.isNaN(value)) return 'n/a'
  if (value <= 1) return `${Math.round(value * 100)}%`
  return `${Math.round(value)}%`
}

const humanize = (value?: string): string =>
  String(value || '')
    .replace(/_/g, ' ')
    .trim()
    .replace(/\b\w/g, (part) => part.toUpperCase())

export default function AITelemetryOverlay({
  buildStatus,
  currentPhase,
  qualityGateStatus,
  qualityGateStage,
  aiThoughts,
  providerPanels,
  workOrders = [],
  verificationReports = [],
  patchBundles = [],
  providerScorecards = [],
  historicalLearning,
}: AITelemetryOverlayProps) {
  const summary = useMemo(() => {
    const activeProviders = providerPanels.filter((panel) => panel.status === 'working' || panel.status === 'thinking').length
    const reviewRequired = patchBundles.filter((bundle) => bundle.review_required || bundle.merge_policy === 'review_required').length
    const failedVerification = verificationReports.filter(
      (report) => report.status === 'failed' || report.status === 'blocked'
    ).length
    const compileRates = providerScorecards
      .map((scorecard) => scorecard.compile_pass_rate)
      .filter((value): value is number => typeof value === 'number')
    const averageCompile = compileRates.length > 0
      ? compileRates.reduce((sum, current) => sum + current, 0) / compileRates.length
      : undefined

    return {
      activeProviders,
      reviewRequired,
      failedVerification,
      averageCompile,
    }
  }, [patchBundles, providerPanels, providerScorecards, verificationReports])

  const learningSignals = [
    ...(historicalLearning?.repair_strategy_win_rates || []),
    ...(historicalLearning?.semantic_repair_hints || []),
    ...(historicalLearning?.recurring_failure_classes || []),
  ].slice(0, 5)

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3">
        <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-gray-500">Build State</div>
          <div className="mt-2 text-sm font-semibold text-white">{humanize(buildStatus)}</div>
          <div className="mt-1 text-xs text-gray-400">
            {currentPhase ? `Phase: ${humanize(currentPhase)}` : 'Phase not reported'}
          </div>
        </div>

        <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-gray-500">Live Providers</div>
          <div className="mt-2 text-sm font-semibold text-white">{summary.activeProviders}</div>
          <div className="mt-1 text-xs text-gray-400">
            {providerPanels.length} total panel{providerPanels.length === 1 ? '' : 's'}
          </div>
        </div>

        <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-gray-500">Quality Gate</div>
          <div className="mt-2 text-sm font-semibold text-white">{humanize(qualityGateStatus || 'pending')}</div>
          <div className="mt-1 text-xs text-gray-400">
            {qualityGateStage ? `Stage: ${humanize(qualityGateStage)}` : 'Stage not reported'}
          </div>
        </div>

        <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-gray-500">Provider Compile Avg</div>
          <div className="mt-2 text-sm font-semibold text-white">{percent(summary.averageCompile)}</div>
          <div className="mt-1 text-xs text-gray-400">
            {providerScorecards.length} scorecard{providerScorecards.length === 1 ? '' : 's'} tracked
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-3">
        <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-3">
          <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.2em] text-gray-500">
            <Workflow className="w-3.5 h-3.5" />
            Work Orders
          </div>
          <div className="mt-2 text-lg font-semibold text-white">{workOrders.length}</div>
        </div>

        <div className={cn(
          'rounded-lg border px-3 py-3',
          summary.failedVerification > 0 ? 'border-amber-500/30 bg-amber-500/10' : 'border-gray-800 bg-gray-950/50'
        )}>
          <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.2em] text-gray-500">
            <ShieldCheck className="w-3.5 h-3.5" />
            Verification Issues
          </div>
          <div className="mt-2 text-lg font-semibold text-white">{summary.failedVerification}</div>
        </div>

        <div className={cn(
          'rounded-lg border px-3 py-3',
          summary.reviewRequired > 0 ? 'border-violet-500/30 bg-violet-500/10' : 'border-gray-800 bg-gray-950/50'
        )}>
          <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.2em] text-gray-500">
            <Target className="w-3.5 h-3.5" />
            Review Required Patches
          </div>
          <div className="mt-2 text-lg font-semibold text-white">{summary.reviewRequired}</div>
        </div>
      </div>

      <HydraRacePanel
        providerPanels={providerPanels}
        patchBundles={patchBundles}
        verificationReports={verificationReports}
      />

      {(historicalLearning && historicalLearning.observed_builds > 0) && (
        <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-3">
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="text-[11px] uppercase tracking-[0.2em] text-gray-500">Learning Priors</div>
              <div className="mt-1 text-xs text-gray-400">
                {historicalLearning.observed_builds} observed build{historicalLearning.observed_builds === 1 ? '' : 's'} in {historicalLearning.scope || 'this scope'}
              </div>
            </div>
            <div className="text-xs font-semibold text-cyan-200">{learningSignals.length} signal{learningSignals.length === 1 ? '' : 's'}</div>
          </div>
          {learningSignals.length > 0 && (
            <div className="mt-3 grid gap-2 md:grid-cols-2">
              {learningSignals.map((signal) => (
                <div key={signal} className="break-words rounded-lg border border-cyan-500/20 bg-cyan-500/10 px-3 py-2 text-xs leading-5 text-cyan-100">
                  {signal}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      <BuildActivityTimeline
        currentPhase={currentPhase}
        qualityGateStatus={qualityGateStatus}
        qualityGateStage={qualityGateStage}
        aiThoughts={aiThoughts}
        workOrders={workOrders}
        verificationReports={verificationReports}
        patchBundles={patchBundles}
      />

      <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-2 text-xs text-gray-500 flex items-center gap-2">
        <Activity className="w-3.5 h-3.5 text-cyan-300" />
        Timeline and race entries are derived from live telemetry, work orders, verification reports, patch bundles, and scorecards.
      </div>
    </div>
  )
}
