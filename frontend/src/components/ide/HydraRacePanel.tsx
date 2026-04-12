import React, { useMemo } from 'react'
import { CheckCircle2, Clock3, GitBranch, Layers3, Trophy } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { GlassBoxPatchBundle, GlassBoxThought, GlassBoxVerificationReport } from './BuildActivityTimeline'

export interface HydraProviderPanel {
  provider: string
  liveModelName: string
  available: boolean
  status: 'idle' | 'working' | 'thinking' | 'completed' | 'error' | 'unavailable'
  statusLabel: string
  thoughts: GlassBoxThought[]
  currentTaskLabel?: string
}

interface HydraRacePanelProps {
  providerPanels: HydraProviderPanel[]
  patchBundles?: GlassBoxPatchBundle[]
  verificationReports?: GlassBoxVerificationReport[]
}

const providerTone: Record<string, string> = {
  claude: 'border-orange-500/30 bg-orange-500/10 text-orange-200',
  gpt4: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-200',
  gemini: 'border-sky-500/30 bg-sky-500/10 text-sky-200',
  grok: 'border-fuchsia-500/30 bg-fuchsia-500/10 text-fuchsia-200',
  ollama: 'border-gray-500/30 bg-gray-500/10 text-gray-200',
}

const statusTone: Record<HydraProviderPanel['status'], string> = {
  working: 'border-cyan-500/40 bg-cyan-500/10 text-cyan-200',
  thinking: 'border-blue-500/40 bg-blue-500/10 text-blue-200',
  completed: 'border-green-500/40 bg-green-500/10 text-green-200',
  error: 'border-red-500/40 bg-red-500/10 text-red-200',
  unavailable: 'border-gray-600 bg-gray-900/40 text-gray-500',
  idle: 'border-gray-700 bg-gray-950/60 text-gray-300',
}

const humanize = (value?: string): string =>
  String(value || '')
    .replace(/_/g, ' ')
    .trim()
    .replace(/\b\w/g, (part) => part.toUpperCase())

const formatTime = (value?: Date | string): string | null => {
  if (!value) return null
  const parsed = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(parsed.getTime())) return null
  return parsed.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

export default function HydraRacePanel({
  providerPanels,
  patchBundles = [],
  verificationReports = [],
}: HydraRacePanelProps) {
  const reviewRequiredCount = useMemo(
    () => patchBundles.filter((bundle) => bundle.review_required || bundle.merge_policy === 'review_required').length,
    [patchBundles]
  )
  const failedVerifications = useMemo(
    () => verificationReports.filter((report) => report.status === 'failed' || report.status === 'blocked').length,
    [verificationReports]
  )

  return (
    <div className="rounded-xl border border-gray-800 bg-black/50 overflow-hidden">
      <div className="flex items-center justify-between gap-3 px-4 py-3 border-b border-gray-800">
        <div className="flex items-center gap-2 text-sm font-semibold text-gray-200">
          <Layers3 className="w-4 h-4 text-cyan-300" />
          Hydra Candidate Race
        </div>
        <div className="text-[11px] text-gray-500">
          {patchBundles.length} patch bundle{patchBundles.length === 1 ? '' : 's'} · {reviewRequiredCount} review required · {failedVerifications} verification issues
        </div>
      </div>

      <div className="p-4 space-y-4">
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
          {providerPanels.map((panel) => {
            const latestThought = panel.thoughts[panel.thoughts.length - 1]
            const providerClass = providerTone[String(panel.provider || '').toLowerCase()] || 'border-gray-700 bg-gray-950/40 text-gray-200'
            const statusClass = statusTone[panel.status] || statusTone.idle
            return (
              <div key={panel.provider} className={cn('rounded-lg border px-3 py-3', providerClass)}>
                <div className="flex items-center justify-between gap-3">
                  <div className="text-sm font-semibold">{humanize(panel.provider)}</div>
                  <span className={cn('text-[10px] uppercase tracking-[0.14em] border rounded px-2 py-0.5', statusClass)}>
                    {panel.statusLabel}
                  </span>
                </div>
                <div className="mt-1 text-xs text-gray-400 font-mono">{panel.liveModelName || 'Model unavailable'}</div>
                {panel.currentTaskLabel && (
                  <div className="mt-2 text-sm text-gray-200">{panel.currentTaskLabel}</div>
                )}
                {latestThought && (
                  <div className="mt-2 text-[11px] text-gray-400">
                    <span className="text-gray-500">Latest:</span> {latestThought.content}
                    <span className="ml-2 text-gray-500">{formatTime(latestThought.timestamp)}</span>
                  </div>
                )}
              </div>
            )
          })}
        </div>

        <div className="space-y-2">
          <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.2em] text-gray-500">
            <Trophy className="w-3.5 h-3.5" />
            Patch Outcomes
          </div>

          {patchBundles.length === 0 ? (
            <div className="rounded-lg border border-gray-800 bg-gray-950/40 p-3 text-sm text-gray-500">
              No patch bundles recorded yet.
            </div>
          ) : (
            patchBundles.slice(0, 10).map((bundle) => {
              const reviewRequired = bundle.review_required || bundle.merge_policy === 'review_required'
              return (
                <div
                  key={bundle.id}
                  className={cn(
                    'rounded-lg border px-3 py-2.5',
                    reviewRequired ? 'border-amber-500/30 bg-amber-500/10' : 'border-green-500/30 bg-green-500/10'
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="text-sm font-semibold text-gray-100">
                      {bundle.justification || 'Hydra winner patch bundle'}
                    </div>
                    <span className={cn(
                      'text-[10px] uppercase tracking-[0.14em] px-2 py-0.5 rounded border',
                      reviewRequired
                        ? 'border-amber-500/40 text-amber-200 bg-amber-500/15'
                        : 'border-green-500/40 text-green-200 bg-green-500/15'
                    )}>
                      {reviewRequired ? 'Review required' : 'Auto merge safe'}
                    </span>
                  </div>
                  <div className="mt-1 flex flex-wrap items-center gap-2 text-[11px] text-gray-400">
                    {bundle.provider && <span>{humanize(bundle.provider)}</span>}
                    {bundle.review_branch && (
                      <span className="inline-flex items-center gap-1">
                        <GitBranch className="w-3 h-3" />
                        {bundle.review_branch}
                      </span>
                    )}
                    {bundle.created_at && (
                      <span className="inline-flex items-center gap-1">
                        <Clock3 className="w-3 h-3" />
                        {formatTime(bundle.created_at)}
                      </span>
                    )}
                  </div>
                  {bundle.suggested_commit_title && (
                    <div className="mt-1 text-[11px] text-gray-400">
                      Suggested commit: {bundle.suggested_commit_title}
                    </div>
                  )}
                  {Array.isArray(bundle.risk_reasons) && bundle.risk_reasons.length > 0 && (
                    <div className="mt-2 flex flex-wrap gap-1.5">
                      {bundle.risk_reasons.slice(0, 4).map((reason) => (
                        <span key={`${bundle.id}-${reason}`} className="text-[10px] border border-gray-700 rounded px-1.5 py-0.5 text-gray-300 bg-black/35">
                          {humanize(reason)}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              )
            })
          )}
        </div>

        {failedVerifications > 0 && (
          <div className="rounded-lg border border-red-500/25 bg-red-950/15 px-3 py-2 text-xs text-red-300 inline-flex items-center gap-2">
            <CheckCircle2 className="w-3.5 h-3.5" />
            {failedVerifications} verification report{failedVerifications === 1 ? '' : 's'} still failed or blocked.
          </div>
        )}
      </div>
    </div>
  )
}
