import React, { useMemo } from 'react'
import { Clock3, FileClock, ShieldCheck, Workflow } from 'lucide-react'
import { cn } from '@/lib/utils'
import { patchBundleNeedsReview, patchBundlePendingReview } from './patchBundleReview'

export interface GlassBoxThought {
  id: string
  provider: string
  type: 'thinking' | 'action' | 'output' | 'error'
  content: string
  timestamp: Date
  eventType?: string
  taskType?: string
  files?: string[]
  filesCount?: number
  retryCount?: number
  maxRetries?: number
  isInternal?: boolean
}

export interface GlassBoxWorkOrder {
  id: string
  role?: string
  category?: string
  task_shape?: string
  summary?: string
  preferred_provider?: string
  contract_slice?: {
    surface?: string
    truth_tags?: string[]
  }
}

export interface GlassBoxVerificationReport {
  id: string
  phase?: string
  surface?: string
  status?: string
  warnings?: string[]
  errors?: string[]
  blockers?: string[]
  truth_tags?: string[]
  generated_at?: string
}

export interface GlassBoxPatchBundle {
  id: string
  provider?: string
  merge_policy?: 'auto_merge_safe' | 'review_required'
  review_required?: boolean
  review_status?: 'pending' | 'approved' | 'rejected'
  reviewed_at?: string
  review_message?: string
  review_branch?: string
  suggested_commit_title?: string
  risk_reasons?: string[]
  justification?: string
  created_at?: string
}

interface BuildActivityTimelineProps {
  currentPhase?: string
  qualityGateStatus?: string
  qualityGateStage?: string
  aiThoughts: GlassBoxThought[]
  workOrders?: GlassBoxWorkOrder[]
  verificationReports?: GlassBoxVerificationReport[]
  patchBundles?: GlassBoxPatchBundle[]
}

type ArtifactTone = 'neutral' | 'success' | 'warning' | 'danger'

type ArtifactEvent = {
  id: string
  label: string
  detail: string
  source: string
  tone: ArtifactTone
  timestamp?: string | null
}

const humanize = (value?: string): string =>
  String(value || '')
    .replace(/[_:]/g, ' ')
    .trim()
    .replace(/\b\w/g, (part) => part.toUpperCase())

const formatTimestamp = (value: Date | string | undefined): string | null => {
  if (!value) return null
  const parsed = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(parsed.getTime())) return null
  return parsed.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const thoughtLabel = (thought: GlassBoxThought): string => {
  const mapped: Record<string, string> = {
    'agent:generating': 'Candidate started',
    'agent:generation_failed': 'Candidate failed',
    'agent:retrying': 'Retry attempt',
    'agent:provider_switched': 'Provider switched',
    'agent:completed': 'Task completed',
    'agent:error': 'Task failed',
    'code:generated': 'Candidate output generated',
    'glassbox:war_room_critique_started': 'War Room critique started',
    'glassbox:war_room_critique_resolved': 'War Room critique resolved',
    'glassbox:work_order_compiled': 'Work order compiled',
    'glassbox:provider_route_selected': 'Provider route selected',
    'glassbox:deterministic_gate_passed': 'Deterministic gate passed',
    'glassbox:deterministic_gate_failed': 'Deterministic gate failed',
    'glassbox:hydra_candidate_started': 'Hydra candidate started',
    'glassbox:hydra_candidate_passed': 'Hydra candidate passed',
    'glassbox:hydra_candidate_failed': 'Hydra candidate failed',
    'glassbox:hydra_winner_selected': 'Hydra winner selected',
    'glassbox:patch_review_required': 'Patch review required',
  }
  const key = String(thought.eventType || '').trim().toLowerCase()
  if (key && mapped[key]) return mapped[key]
  if (key) return humanize(key)
  if (thought.type === 'error') return 'Runtime error'
  if (thought.type === 'action') return 'Action update'
  if (thought.type === 'output') return 'Output update'
  return 'Thinking update'
}

const artifactToneClass: Record<ArtifactTone, string> = {
  neutral: 'border-gray-700 bg-gray-950/60 text-gray-300',
  success: 'border-green-500/30 bg-green-500/10 text-green-300',
  warning: 'border-amber-500/30 bg-amber-500/10 text-amber-300',
  danger: 'border-red-500/30 bg-red-500/10 text-red-300',
}

const patchBundleArtifact = (bundle: GlassBoxPatchBundle): Pick<ArtifactEvent, 'label' | 'tone' | 'timestamp'> => {
  if (patchBundlePendingReview(bundle)) {
    return {
      label: 'Patch review required',
      tone: 'warning',
      timestamp: formatTimestamp(bundle.created_at),
    }
  }
  if (patchBundleNeedsReview(bundle) && bundle.review_status === 'approved') {
    return {
      label: 'Patch approved',
      tone: 'success',
      timestamp: formatTimestamp(bundle.reviewed_at || bundle.created_at),
    }
  }
  if (patchBundleNeedsReview(bundle) && bundle.review_status === 'rejected') {
    return {
      label: 'Patch rejected',
      tone: 'danger',
      timestamp: formatTimestamp(bundle.reviewed_at || bundle.created_at),
    }
  }
  return {
    label: 'Hydra winner selected',
    tone: 'success',
    timestamp: formatTimestamp(bundle.created_at),
  }
}

const telemetryToneClass = (thought: GlassBoxThought): string => {
  if (thought.type === 'error') return 'border-red-500/30 bg-red-500/10 text-red-300'
  if (thought.eventType === 'glassbox:patch_review_required') return 'border-amber-500/30 bg-amber-500/10 text-amber-300'
  if (thought.eventType === 'glassbox:hydra_winner_selected') return 'border-green-500/30 bg-green-500/10 text-green-300'
  if (thought.eventType?.startsWith('glassbox:')) return 'border-cyan-500/30 bg-cyan-500/10 text-cyan-300'
  if (thought.eventType === 'agent:retrying') return 'border-amber-500/30 bg-amber-500/10 text-amber-300'
  if (thought.eventType === 'agent:generating') return 'border-cyan-500/30 bg-cyan-500/10 text-cyan-300'
  if (thought.type === 'output') return 'border-green-500/30 bg-green-500/10 text-green-300'
  return 'border-gray-700 bg-gray-950/60 text-gray-300'
}

export default function BuildActivityTimeline({
  currentPhase,
  qualityGateStatus,
  qualityGateStage,
  aiThoughts,
  workOrders = [],
  verificationReports = [],
  patchBundles = [],
}: BuildActivityTimelineProps) {
  const artifactEvents = useMemo<ArtifactEvent[]>(() => {
    const events: ArtifactEvent[] = []

    if (currentPhase) {
      events.push({
        id: `phase-${currentPhase}`,
        label: 'Phase transition',
        detail: `Current phase: ${humanize(currentPhase)}`,
        source: 'build_state.current_phase',
        tone: 'neutral',
      })
    }

    if (qualityGateStatus) {
      const normalized = String(qualityGateStatus).toLowerCase()
      const tone: ArtifactTone =
        normalized === 'passed' ? 'success' :
          normalized === 'failed' ? 'danger' :
            normalized === 'running' ? 'warning' : 'neutral'
      events.push({
        id: `quality-gate-${qualityGateStatus}-${qualityGateStage || 'none'}`,
        label: 'Deterministic gate',
        detail: `${humanize(qualityGateStatus)}${qualityGateStage ? ` · ${humanize(qualityGateStage)}` : ''}`,
        source: 'quality_gate_status',
        tone,
      })
    }

    for (const order of workOrders) {
      const scopeParts = [humanize(order.role), humanize(order.task_shape || order.category)].filter(Boolean)
      events.push({
        id: `wo-${order.id}`,
        label: 'Work order compiled',
        detail: scopeParts.join(' · ') || 'Execution slice created',
        source: 'work_orders',
        tone: 'neutral',
      })
    }

    for (const report of verificationReports) {
      const status = String(report.status || '').toLowerCase()
      const tone: ArtifactTone =
        status === 'passed' ? 'success' :
          status === 'failed' ? 'danger' :
            status === 'blocked' ? 'warning' : 'neutral'
      const issueCount = (report.errors?.length || 0) + (report.blockers?.length || 0)
      events.push({
        id: `vr-${report.id}`,
        label: `Verification ${humanize(report.status) || 'Update'}`,
        detail: `${humanize(report.phase)} · ${humanize(report.surface)}${issueCount > 0 ? ` · ${issueCount} issue${issueCount === 1 ? '' : 's'}` : ''}`,
        source: 'verification_reports',
        tone,
        timestamp: formatTimestamp(report.generated_at),
      })
    }

    for (const bundle of patchBundles) {
      const artifact = patchBundleArtifact(bundle)
      events.push({
        id: `pb-${bundle.id}`,
        label: artifact.label,
        detail: bundle.review_message || bundle.justification || `${bundle.provider || 'provider unknown'} patch bundle`,
        source: 'patch_bundles',
        tone: artifact.tone,
        timestamp: artifact.timestamp,
      })
    }

    return events.slice(0, 60)
  }, [currentPhase, patchBundles, qualityGateStage, qualityGateStatus, verificationReports, workOrders])

  const telemetryEvents = useMemo(() => {
    return aiThoughts
      .filter((thought) => thought.content?.trim())
      .slice(-40)
      .reverse()
  }, [aiThoughts])

  return (
    <div className="rounded-xl border border-gray-800 bg-black/50 overflow-hidden">
      <div className="flex items-center justify-between gap-3 px-4 py-3 border-b border-gray-800">
        <div className="flex items-center gap-2 text-sm font-semibold text-gray-200">
          <Workflow className="w-4 h-4 text-cyan-300" />
          Build Activity Timeline
        </div>
        <div className="text-[10px] uppercase tracking-[0.2em] text-gray-500">
          Real execution artifacts only
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2">
        <section className="border-b xl:border-b-0 xl:border-r border-gray-800 p-4 space-y-2">
          <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.2em] text-gray-500">
            <ShieldCheck className="w-3.5 h-3.5" />
            Orchestration Artifacts
          </div>
          {artifactEvents.length === 0 ? (
            <div className="rounded-lg border border-gray-800 bg-gray-950/40 p-3 text-sm text-gray-500">
              Waiting for work orders, verification reports, and patch bundles.
            </div>
          ) : (
            artifactEvents.map((event) => (
              <div
                key={event.id}
                className={cn('rounded-lg border px-3 py-2.5', artifactToneClass[event.tone])}
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="text-sm font-semibold">{event.label}</div>
                  {event.timestamp && (
                    <span className="text-[10px] font-mono text-gray-500">{event.timestamp}</span>
                  )}
                </div>
                <div className="mt-1 text-xs text-gray-300">{event.detail}</div>
                <div className="mt-1 text-[10px] uppercase tracking-[0.16em] text-gray-500">Source: {event.source}</div>
              </div>
            ))
          )}
        </section>

        <section className="p-4 space-y-2">
          <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.2em] text-gray-500">
            <Clock3 className="w-3.5 h-3.5" />
            Telemetry Stream
          </div>
          {telemetryEvents.length === 0 ? (
            <div className="rounded-lg border border-gray-800 bg-gray-950/40 p-3 text-sm text-gray-500">
              No live telemetry events yet.
            </div>
          ) : (
            telemetryEvents.map((thought) => (
              <div
                key={thought.id}
                className={cn('rounded-lg border px-3 py-2.5', telemetryToneClass(thought))}
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="text-xs font-semibold uppercase tracking-[0.14em]">
                    {thoughtLabel(thought)}
                  </div>
                  <div className="text-[10px] font-mono text-gray-500">{formatTimestamp(thought.timestamp)}</div>
                </div>
                <div className="mt-1 text-sm leading-relaxed">{thought.content}</div>
                <div className="mt-1 flex flex-wrap items-center gap-2 text-[10px] text-gray-500">
                  <span>{humanize(thought.provider)}</span>
                  {thought.taskType && <span>· task: {humanize(thought.taskType)}</span>}
                  {thought.filesCount && thought.filesCount > 0 && <span>· files: {thought.filesCount}</span>}
                  {thought.files && thought.files.length > 0 && (
                    <span className="inline-flex items-center gap-1 text-gray-400">
                      <FileClock className="w-3 h-3" />
                      {thought.files.slice(0, 2).join(', ')}
                      {thought.files.length > 2 ? ` +${thought.files.length - 2}` : ''}
                    </span>
                  )}
                  {typeof thought.retryCount === 'number' && (
                    <span className="inline-flex items-center gap-1">
                      {thought.retryCount}/{thought.maxRetries || '?'} retries
                    </span>
                  )}
                </div>
              </div>
            ))
          )}
        </section>
      </div>
    </div>
  )
}
