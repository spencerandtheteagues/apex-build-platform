import React, { startTransition, useDeferredValue, useMemo, useState } from 'react'
import { AlertCircle, CheckCircle2, Clock3, FileDiff, Filter, Layers3, Search, ShieldCheck, Sparkles, Workflow } from 'lucide-react'

import type {
  BuildApproval,
  BuildBlocker,
  BuildCapabilityState,
  BuildContractSummary,
  BuildFailureFingerprintState,
  BuildIntentBrief,
  BuildInteractionState,
  BuildPatchBundleState,
  BuildPolicyState,
  BuildPromotionDecisionState,
  BuildProviderScorecardState,
  BuildVerificationReportState,
  BuildWorkOrderState,
} from '@/services/api'
import { Badge, Card, CardContent, CardHeader, CardTitle } from '@/components/ui'
import { cn } from '@/lib/utils'

type OrchestrationOverviewProps = {
  buildStatus: string
  currentPhase?: string
  qualityGateStatus?: string
  capabilityState?: BuildCapabilityState
  policyState?: BuildPolicyState
  blockers?: BuildBlocker[]
  approvals?: BuildApproval[]
  checkpoints?: CheckpointLike[]
  interaction?: BuildInteractionState
  intentBrief?: BuildIntentBrief
  buildContract?: BuildContractSummary
  workOrders?: BuildWorkOrderState[]
  patchBundles?: BuildPatchBundleState[]
  verificationReports?: BuildVerificationReportState[]
  promotionDecision?: BuildPromotionDecisionState
  providerScorecards?: BuildProviderScorecardState[]
  failureFingerprints?: BuildFailureFingerprintState[]
  truthBySurface?: Record<string, string[]>
}

type CheckpointLike = {
  id: string
  number: number
  name: string
  description: string
  progress: number
  restorable?: boolean
  created_at?: string
  createdAt?: string
}

type TimelineStatus = 'pending' | 'in_progress' | 'paused' | 'blocked' | 'failed' | 'completed' | 'skipped'
type MockRealStatus = 'real' | 'partial' | 'blocked' | 'missing'

const EMPTY_TAGS: string[] = []
const EMPTY_BLOCKERS: BuildBlocker[] = []
const EMPTY_APPROVALS: BuildApproval[] = []
const EMPTY_CAPABILITIES: string[] = []
const EMPTY_CHECKPOINTS: CheckpointLike[] = []

const PHASE_DEFS = [
  { id: 'intent', label: 'Request Intake', description: 'Normalize the request and classify static-ready vs upgrade-required vs full-stack.' },
  { id: 'contract', label: 'Contract Compilation', description: 'Compile the build contract and its initial truth constraints.' },
  { id: 'work', label: 'Work Orders', description: 'Slice the contract into owned execution units.' },
  { id: 'patch', label: 'Patch Generation', description: 'Apply scaffold, implementation, and repair patches.' },
  { id: 'verify', label: 'Surface Verification', description: 'Run verification and record blockers instead of faking readiness.' },
  { id: 'repair', label: 'Repair Ladder', description: 'Select recovery paths only when verification or fingerprints justify them.' },
  { id: 'promote', label: 'Readiness Promotion', description: 'Promote only when verification truth supports the target readiness state.' },
] as const

const timelineStatusTone: Record<TimelineStatus, string> = {
  pending: 'border-gray-700 bg-gray-950/60 text-gray-300',
  in_progress: 'border-cyan-500/40 bg-cyan-500/10 text-cyan-300',
  paused: 'border-violet-500/40 bg-violet-500/10 text-violet-300',
  blocked: 'border-red-500/40 bg-red-500/10 text-red-300',
  failed: 'border-rose-500/40 bg-rose-500/10 text-rose-300',
  completed: 'border-green-500/40 bg-green-500/10 text-green-300',
  skipped: 'border-gray-700 bg-black/40 text-gray-400',
}

const classificationTone = (classification?: string) => {
  switch (classification) {
    case 'static_ready':
      return 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300'
    case 'upgrade_required':
      return 'border-amber-500/40 bg-amber-500/10 text-amber-300'
    case 'full_stack_candidate':
      return 'border-cyan-500/40 bg-cyan-500/10 text-cyan-300'
    default:
      return 'border-gray-700 bg-gray-950/60 text-gray-300'
  }
}

const approvalTone = (status: BuildApproval['status']) => {
  switch (status) {
    case 'satisfied':
      return 'border-green-500/40 bg-green-500/10 text-green-300'
    case 'pending':
      return 'border-amber-500/40 bg-amber-500/10 text-amber-300'
    case 'denied':
      return 'border-red-500/40 bg-red-500/10 text-red-300'
    default:
      return 'border-gray-700 bg-gray-950/60 text-gray-300'
  }
}

const blockerTone = (severity?: string) => {
  switch (severity) {
    case 'blocking':
      return 'border-red-500/40 bg-red-500/10 text-red-300'
    case 'warning':
      return 'border-amber-500/40 bg-amber-500/10 text-amber-300'
    default:
      return 'border-gray-700 bg-gray-950/60 text-gray-300'
  }
}

const approvalEventTone = (status?: string) => {
  switch (status) {
    case 'satisfied':
      return 'border-green-500/40 bg-green-500/10 text-green-300'
    case 'denied':
      return 'border-red-500/40 bg-red-500/10 text-red-300'
    default:
      return 'border-amber-500/40 bg-amber-500/10 text-amber-300'
  }
}

const includesPhase = (currentPhase: string | undefined, ...values: string[]) => {
  const normalized = String(currentPhase || '').trim().toLowerCase()
  return values.some((value) => normalized.includes(value))
}

const phaseStatus = (
  phaseId: string,
  props: OrchestrationOverviewProps
): TimelineStatus => {
  const { buildStatus, currentPhase, blockers, interaction, intentBrief, buildContract, workOrders, patchBundles, verificationReports, promotionDecision, failureFingerprints } = props
  const blocked = (blockers || []).some((item) => item.severity === 'blocking')
  const paused = Boolean(interaction?.paused || interaction?.waiting_for_user || buildStatus === 'awaiting_review')
  const failed = buildStatus === 'failed'

  switch (phaseId) {
    case 'intent':
      if (intentBrief) return 'completed'
      if (paused && includesPhase(currentPhase, 'planning', 'intent')) return 'paused'
      return includesPhase(currentPhase, 'planning', 'intent') ? 'in_progress' : 'pending'
    case 'contract':
      if (buildContract?.verified) return 'completed'
      if (failed && (includesPhase(currentPhase, 'contract') || Boolean(buildContract))) return 'failed'
      if (paused && (includesPhase(currentPhase, 'contract', 'planning') || Boolean(buildContract))) return 'paused'
      if (buildContract) return 'in_progress'
      return includesPhase(currentPhase, 'contract', 'planning') ? 'in_progress' : 'pending'
    case 'work':
      if ((workOrders || []).length > 0) return 'completed'
      if (failed && includesPhase(currentPhase, 'work', 'provider_check', 'in_progress')) return 'failed'
      if (paused && includesPhase(currentPhase, 'work', 'provider_check', 'in_progress')) return 'paused'
      return includesPhase(currentPhase, 'work', 'provider_check', 'in_progress') ? 'in_progress' : 'pending'
    case 'patch':
      if ((patchBundles || []).length > 0) return 'completed'
      if (failed && includesPhase(currentPhase, 'patch', 'implement', 'in_progress')) return 'failed'
      if (paused && includesPhase(currentPhase, 'patch', 'implement', 'in_progress')) return 'paused'
      return includesPhase(currentPhase, 'patch', 'implement', 'in_progress') ? 'in_progress' : 'pending'
    case 'verify':
      if ((verificationReports || []).length > 0 && buildStatus === 'completed') return 'completed'
      if (failed && ((verificationReports || []).length > 0 || includesPhase(currentPhase, 'validation', 'testing', 'review'))) return 'failed'
      if (paused && includesPhase(currentPhase, 'validation', 'testing', 'review')) return 'paused'
      if (blocked && ((verificationReports || []).length > 0 || includesPhase(currentPhase, 'validation', 'testing', 'review'))) return 'blocked'
      return includesPhase(currentPhase, 'validation', 'testing', 'review') ? 'in_progress' : 'pending'
    case 'repair':
      if ((failureFingerprints || []).length === 0 && buildStatus === 'completed' && (verificationReports || []).length > 0) return 'skipped'
      if ((failureFingerprints || []).some((item) => item.repair_success)) return 'completed'
      if (failed && (((failureFingerprints || []).length > 0) || blocked)) return 'failed'
      if (paused && ((failureFingerprints || []).length > 0 || blocked)) return 'paused'
      if (blocked || (failureFingerprints || []).length > 0) return 'in_progress'
      return 'pending'
    case 'promote':
      if (promotionDecision) return buildStatus === 'failed' ? 'blocked' : 'completed'
      if (failed && includesPhase(currentPhase, 'promotion', 'completed')) return 'failed'
      if (paused && includesPhase(currentPhase, 'promotion', 'completed')) return 'paused'
      if (blocked && includesPhase(currentPhase, 'promotion', 'completed')) return 'blocked'
      return includesPhase(currentPhase, 'promotion', 'completed') ? 'in_progress' : 'pending'
    default:
      return 'pending'
  }
}

const humanize = (value: string | undefined) =>
  String(value || '')
    .replace(/_/g, ' ')
    .trim()

const patchBundleNeedsReview = (bundle: BuildPatchBundleState): boolean => {
  return bundle.review_required === true || bundle.merge_policy === 'review_required'
}

const formatTimestamp = (value?: string) => {
  if (!value) return null
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return null
  return parsed.toLocaleString([], {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

const truthHasAny = (tags: string[] | undefined, candidates: string[]) => {
  const set = new Set((tags || []).map((tag) => String(tag).trim().toLowerCase()))
  return candidates.some((candidate) => set.has(candidate))
}

const statusTone = (status: MockRealStatus) => {
  switch (status) {
    case 'real':
      return 'border-green-500/40 bg-green-500/10 text-green-300'
    case 'partial':
      return 'border-cyan-500/40 bg-cyan-500/10 text-cyan-300'
    case 'blocked':
      return 'border-amber-500/40 bg-amber-500/10 text-amber-300'
    default:
      return 'border-gray-700 bg-gray-950/60 text-gray-300'
  }
}

type JournalEntry = {
  id: string
  title: string
  detail: string
  timestamp?: string | null
  tone: 'info' | 'success' | 'warning'
}

type DiffRow = {
  id: string
  label: string
  ui: MockRealStatus
  backend: MockRealStatus
  data: MockRealStatus
  integrations: MockRealStatus
  verification: MockRealStatus
  readiness: MockRealStatus
  gating: string
}

type TimelineItem = {
  id: string
  label: string
  description: string
  status: TimelineStatus
  timestamp?: string | null
  substeps: string[]
}

const journalToneBadge: Record<JournalEntry['tone'], string> = {
  success: 'border-green-500/40 bg-green-500/10 text-green-300',
  warning: 'border-amber-500/40 bg-amber-500/10 text-amber-300',
  info: 'border-cyan-500/40 bg-cyan-500/10 text-cyan-300',
}

const truthNarrative = (surface: string, tags: string[]) => {
  const normalized = tags.map((tag) => String(tag).trim().toLowerCase())

  if (normalized.includes('production_ready')) {
    return `${humanize(surface)} is fully wired, verified, and safe to treat as production-ready.`
  }
  if (normalized.includes('production_candidate')) {
    return `${humanize(surface)} is close to promotion, but a final deployment or validation step still stands between preview and production.`
  }
  if (normalized.includes('verified')) {
    return `${humanize(surface)} has verification evidence behind it, even if some adjacent systems still need promotion or deployment checks.`
  }
  if (normalized.includes('upgrade_required')) {
    return `${humanize(surface)} is blocked by plan policy. Honest prototype or static work can continue, but paid capability gates are stopping the real path.`
  }
  if (normalized.includes('needs_secrets') || normalized.includes('needs_external_api')) {
    return `${humanize(surface)} is structurally mapped, but it still depends on secrets or external systems before it can be considered real.`
  }
  if (normalized.includes('partially_wired') || normalized.includes('scaffolded')) {
    return `${humanize(surface)} has meaningful implementation work in place, but it is still somewhere between scaffold and live wiring.`
  }
  if (normalized.includes('prototype_ui_only') || normalized.includes('mocked')) {
    return `${humanize(surface)} currently represents prototype output, not a live integrated capability.`
  }
  return `${humanize(surface)} is tracked by orchestration truth, but more verification and wiring detail still needs to accumulate.`
}

export function OrchestrationOverview(props: OrchestrationOverviewProps) {
  const [journalQuery, setJournalQuery] = useState('')
  const [journalToneFilter, setJournalToneFilter] = useState<'all' | JournalEntry['tone']>('all')
  const deferredJournalQuery = useDeferredValue(journalQuery.trim().toLowerCase())
  const truthBySurface = props.truthBySurface || props.promotionDecision?.truth_by_surface || props.buildContract?.truth_by_surface || {}
  const requiredCapabilities = props.capabilityState?.required_capabilities || EMPTY_CAPABILITIES
  const contract = props.buildContract
  const intent = props.intentBrief
  const checkpoints = props.checkpoints || EMPTY_CHECKPOINTS
  const blockers = props.blockers || EMPTY_BLOCKERS

  const timeline = useMemo<TimelineItem[]>(() => {
    return PHASE_DEFS.map((phase) => {
      const status = phaseStatus(phase.id, props)
      let timestamp: string | null = null
      let substeps: string[] = []
      const waitingOnUser = Boolean(props.interaction?.waiting_for_user || props.interaction?.paused)
      const classification = props.policyState?.classification

      switch (phase.id) {
        case 'intent':
          timestamp = formatTimestamp(intent?.created_at)
          if (intent?.normalized_request) {
            substeps.push('Request normalized from the original prompt.')
          }
          if (props.policyState?.classification) {
            substeps.push(`Classification: ${humanize(props.policyState.classification)}.`)
          }
          if (requiredCapabilities.length > 0) {
            substeps.push(`Capabilities inferred: ${requiredCapabilities.slice(0, 3).map(humanize).join(', ')}.`)
          }
          break
        case 'contract':
          if (contract && !contract.verified) {
            substeps.push('Contract exists but final contract verification is still in progress.')
          }
          if (contract?.verified) {
            substeps.push('Contract verification completed.')
          }
          if (contract?.auth_contract?.required) {
            substeps.push(`Auth strategy: ${[contract.auth_contract.provider, contract.auth_contract.session_strategy || contract.auth_contract.token_strategy].filter(Boolean).join(' · ')}.`)
          }
          if ((contract?.env_var_contract || []).length > 0) {
            substeps.push(`${contract?.env_var_contract?.length || 0} secret or env requirement${(contract?.env_var_contract?.length || 0) === 1 ? '' : 's'} identified.`)
          }
          break
        case 'work':
          substeps = (props.workOrders || []).slice(0, 3).map((order) => order.summary || `${humanize(order.role)} ${humanize(order.category)}`)
          if (substeps.length === 0 && status !== 'pending') {
            substeps.push('Work order slicing is in progress.')
          }
          break
        case 'patch':
          timestamp = formatTimestamp((props.patchBundles || []).slice().sort((left, right) => String(right.created_at || '').localeCompare(String(left.created_at || '')))[0]?.created_at)
          substeps = (props.patchBundles || []).slice(0, 3).map((bundle) => {
            const summary = bundle.justification || `Patch bundle recorded${bundle.provider ? ` via ${bundle.provider}` : ''}.`
            return patchBundleNeedsReview(bundle) ? `${summary} Review required before merge.` : summary
          })
          if (substeps.length === 0 && status !== 'pending') {
            substeps.push('Patch generation or repair is active.')
          }
          break
        case 'verify':
          timestamp = formatTimestamp((props.verificationReports || []).slice().sort((left, right) => String(right.generated_at || '').localeCompare(String(left.generated_at || '')))[0]?.generated_at)
          substeps = (props.verificationReports || []).slice(0, 3).map((report) => `${humanize(report.surface)} ${humanize(report.status)} during ${humanize(report.phase)}.`)
          if (substeps.length === 0 && blockers.length > 0) {
            substeps.push(blockers[0].summary || blockers[0].title)
          }
          break
        case 'repair':
          timestamp = formatTimestamp((props.failureFingerprints || []).slice().sort((left, right) => String(right.created_at || '').localeCompare(String(left.created_at || '')))[0]?.created_at)
          substeps = (props.failureFingerprints || []).slice(0, 3).map((fingerprint) => {
            const repairPath = Array.isArray(fingerprint.repair_path_chosen) && fingerprint.repair_path_chosen.length > 0
              ? fingerprint.repair_path_chosen.map(humanize).join(', ')
              : 'repair path not recorded yet'
            return `${humanize(fingerprint.failure_class || 'unknown failure')} via ${fingerprint.provider || 'unknown provider'}; repair path: ${repairPath}.`
          })
          if (substeps.length === 0 && status === 'skipped') {
            substeps.push('No repair ladder was needed because verification completed without recurring failure fingerprints.')
          } else if (substeps.length === 0 && (status === 'in_progress' || status === 'failed' || status === 'blocked')) {
            substeps.push('Repair selection is active because verification truth still has unresolved failures.')
          }
          break
        case 'promote':
          timestamp = formatTimestamp(props.promotionDecision?.generated_at)
          if (props.promotionDecision?.readiness_state) {
            substeps.push(`Readiness state: ${humanize(props.promotionDecision.readiness_state)}.`)
          }
          if (props.promotionDecision?.production_candidate) {
            substeps.push('Promotion marked this build as a production candidate.')
          } else if (props.promotionDecision) {
            substeps.push('Promotion remains constrained by current verification truth.')
          }
          if (checkpoints.length > 0) {
            substeps.push(`${checkpoints.length} checkpoint${checkpoints.length === 1 ? '' : 's'} available for recovery.`)
          }
          break
        default:
          break
      }

      if (phase.id === 'work' || phase.id === 'patch' || phase.id === 'verify' || phase.id === 'repair' || phase.id === 'promote') {
        if (classification === 'upgrade_required') {
          substeps.push(`Plan gate: ${humanize(classification)}. Honest static-only work may continue, but paid-only steps will stop until Builder or higher is approved.`)
        } else if (classification) {
          substeps.push(`Plan gate: ${humanize(classification)} on the current ${props.policyState?.plan_type || 'active'} plan.`)
        }
      }

      if (waitingOnUser && (status === 'paused' || status === 'blocked' || status === 'in_progress')) {
        substeps.push(props.interaction?.pause_reason || props.interaction?.pending_question || 'Waiting on a user acknowledgement, reply, or permission decision.')
      }

      if (substeps.length === 0) {
        substeps = ['Waiting for orchestration state to populate this phase.']
      }

      return {
        ...phase,
        status,
        timestamp,
        substeps: substeps.slice(0, 3),
      }
    })
  }, [blockers, checkpoints, contract, intent, props, requiredCapabilities])

  const surfaces = Object.entries(truthBySurface)
    .filter(([, tags]) => Array.isArray(tags) && tags.length > 0)
    .sort(([left], [right]) => left.localeCompare(right))

  const requiredApprovals = (props.approvals || EMPTY_APPROVALS).filter((approval) => approval.required)
  const frontendTags = truthBySurface.frontend || EMPTY_TAGS
  const backendTags = truthBySurface.backend || EMPTY_TAGS
  const dataTags = truthBySurface.data || EMPTY_TAGS
  const integrationTags = truthBySurface.integration || EMPTY_TAGS
  const deploymentTags = truthBySurface.deployment || EMPTY_TAGS
  const deliveryModeLabel = contract?.delivery_mode
    ? humanize(contract.delivery_mode)
    : props.policyState?.static_frontend_only
      ? 'Frontend preview only'
      : props.policyState?.classification === 'full_stack_candidate'
        ? 'Full stack preview'
        : 'Preview pending'
  const architectureItems = [
    {
      label: 'Delivery',
      value: deliveryModeLabel,
    },
    {
      label: 'Frontend',
      value: contract?.app_type === 'api'
        ? 'API-first build'
        : props.policyState?.classification === 'static_ready'
          ? 'Static frontend'
          : 'Frontend with app shell',
    },
    {
      label: 'Backend',
      value: contract?.backend_resource_map && contract.backend_resource_map.length > 0
        ? contract.backend_resource_map.map((resource) => `${resource.kind}:${resource.name}`).slice(0, 2).join(', ')
        : props.capabilityState?.requires_backend_runtime
          ? 'Runtime required'
          : 'Not required',
    },
    {
      label: 'Data',
      value: contract?.db_schema_contract && contract.db_schema_contract.length > 0
        ? contract.db_schema_contract.map((model) => model.name).slice(0, 3).join(', ')
        : props.capabilityState?.requires_database
          ? 'Persistent storage needed'
          : 'None',
    },
    {
      label: 'Auth',
      value: contract?.auth_contract?.required
        ? [contract.auth_contract.provider, contract.auth_contract.session_strategy || contract.auth_contract.token_strategy].filter(Boolean).join(' · ')
        : props.capabilityState?.requires_auth
          ? 'Required'
          : 'Not required',
    },
    {
      label: 'Deploy',
      value: intent?.deployment_target || (props.capabilityState?.requires_publish ? 'Publish flow requested' : 'Preview only'),
    },
    {
      label: 'Cost',
      value: [intent?.complexity_class, intent?.cost_sensitivity].filter(Boolean).map(humanize).join(' · ') || 'Not inferred yet',
    },
  ]
  const journal = useMemo<JournalEntry[]>(() => {
    const items: JournalEntry[] = []

    if (intent) {
      items.push({
        id: 'intent',
        title: 'Request parsed',
        detail: `${humanize(intent.app_type)} build request normalized with ${requiredCapabilities.length || 0} inferred capability checks.`,
        timestamp: formatTimestamp(intent.created_at),
        tone: 'success',
      })
    }
    if (props.policyState?.classification) {
      items.push({
        id: 'classification',
        title: 'Build classification selected',
        detail: props.policyState.classification === 'upgrade_required'
          ? `Upgrade required because this request needs ${humanize(props.policyState.upgrade_reason || 'paid capabilities')}.`
          : props.policyState.classification === 'static_ready'
            ? 'Static/frontend-only mode is valid on the current plan.'
            : 'Full-stack candidate path is allowed on the current plan.',
        tone: props.policyState.classification === 'upgrade_required' ? 'warning' : 'success',
      })
    }
    if (contract) {
      items.push({
        id: 'contract',
        title: 'Architecture selected',
        detail: `${humanize(contract.app_type || intent?.app_type)} contract compiled${contract.verified ? ' and verified' : ''}.`,
        tone: contract.verified ? 'success' : 'info',
      })
    }
    if ((props.workOrders || []).length > 0) {
      items.push({
        id: 'work-orders',
        title: 'Work orders prepared',
        detail: `${props.workOrders?.length || 0} owned work order${(props.workOrders?.length || 0) === 1 ? '' : 's'} generated for execution.`,
        tone: 'info',
      })
    }
    if ((props.patchBundles || []).length > 0) {
      const latestPatchTime = props.patchBundles?.map((bundle) => formatTimestamp(bundle.created_at || '')).find(Boolean) || null
      const reviewRequiredCount = (props.patchBundles || []).filter(patchBundleNeedsReview).length
      items.push({
        id: 'patches',
        title: 'Patch bundles generated',
        detail: reviewRequiredCount > 0
          ? `${props.patchBundles?.length || 0} patch bundle${(props.patchBundles?.length || 0) === 1 ? '' : 's'} recorded; ${reviewRequiredCount} require review before merge.`
          : `${props.patchBundles?.length || 0} patch bundle${(props.patchBundles?.length || 0) === 1 ? '' : 's'} recorded for implementation or repair.`,
        timestamp: latestPatchTime,
        tone: reviewRequiredCount > 0 ? 'warning' : 'info',
      })
    }
    if (checkpoints.length > 0) {
      const latestCheckpoint = checkpoints[checkpoints.length - 1]
      items.push({
        id: 'checkpoint',
        title: 'Checkpoint persisted',
        detail: `${checkpoints.length} recovery checkpoint${checkpoints.length === 1 ? '' : 's'} saved. Latest: ${latestCheckpoint.name}.`,
        timestamp: formatTimestamp(latestCheckpoint.created_at || latestCheckpoint.createdAt),
        tone: 'info',
      })
    }
    if ((props.verificationReports || []).length > 0) {
      const failed = (props.verificationReports || []).filter((report) => report.status !== 'passed')
      const lastVerification = (props.verificationReports || []).slice().sort((left, right) => String(right.generated_at || '').localeCompare(String(left.generated_at || '')))[0]
      items.push({
        id: 'verification',
        title: failed.length > 0 ? 'Verification found unresolved issues' : 'Verification reports recorded',
        detail: failed.length > 0
          ? `${failed.length} verification report${failed.length === 1 ? '' : 's'} reported blockers or failures.`
          : `${props.verificationReports?.length || 0} verification report${(props.verificationReports?.length || 0) === 1 ? '' : 's'} support the current readiness state.`,
        timestamp: formatTimestamp(lastVerification?.generated_at),
        tone: failed.length > 0 ? 'warning' : 'success',
      })
    }
    if (blockers.length > 0) {
      const primaryBlocker = blockers[0]
      items.push({
        id: 'blocker',
        title: 'Build is currently blocked',
        detail: primaryBlocker.summary || primaryBlocker.title,
        tone: 'warning',
      })
    }
    if (props.interaction?.waiting_for_user || props.interaction?.paused) {
      items.push({
        id: 'interaction',
        title: props.interaction.paused ? 'Build paused for durable review' : 'Awaiting user acknowledgement',
        detail: props.interaction.pause_reason || props.interaction.pending_question || 'The build is waiting on a persisted approval or user decision.',
        tone: 'warning',
      })
    }
    if (props.promotionDecision) {
      items.push({
        id: 'promotion',
        title: 'Promotion decision updated',
        detail: `Current readiness: ${humanize(props.promotionDecision.readiness_state || 'pending')}.`,
        timestamp: formatTimestamp(props.promotionDecision.generated_at),
        tone: props.promotionDecision.production_candidate ? 'success' : 'info',
      })
    }

    return items
  }, [blockers, checkpoints, contract, intent, props.interaction, props.patchBundles, props.policyState, props.promotionDecision, props.verificationReports, props.workOrders, requiredCapabilities.length])

  const filteredJournal = useMemo(() => {
    return journal.filter((entry) => {
      if (journalToneFilter !== 'all' && entry.tone !== journalToneFilter) {
        return false
      }
      if (!deferredJournalQuery) {
        return true
      }
      const haystack = `${entry.title} ${entry.detail}`.toLowerCase()
      return haystack.includes(deferredJournalQuery)
    })
  }, [deferredJournalQuery, journal, journalToneFilter])

  const diffRows = useMemo<DiffRow[]>(() => {
    const verificationPassed = (surface: string) => (props.verificationReports || []).some((report) => report.surface === surface && report.status === 'passed')
    const planGate = props.policyState?.classification === 'upgrade_required'
      ? `Upgrade to ${props.policyState.required_plan || 'builder'}`
      : 'Available on current plan'

    return [
      {
        id: 'frontend',
        label: 'Frontend',
        ui: truthHasAny(frontendTags, ['verified', 'live_logic_connected']) ? 'real' : truthHasAny(frontendTags, ['prototype_ui_only', 'scaffolded', 'partially_wired']) ? 'partial' : 'missing',
        backend: props.capabilityState?.requires_backend_runtime ? 'partial' : 'missing',
        data: props.capabilityState?.requires_database ? 'partial' : 'missing',
        integrations: props.capabilityState?.requires_external_api ? 'partial' : 'missing',
        verification: verificationPassed('frontend') ? 'real' : truthHasAny(frontendTags, ['verified']) ? 'partial' : 'missing',
        readiness: props.promotionDecision?.preview_ready ? 'real' : 'partial',
        gating: props.policyState?.classification === 'static_ready' ? 'Current plan supports this path' : planGate,
      },
      {
        id: 'backend',
        label: 'Backend',
        ui: 'missing',
        backend: truthHasAny(backendTags, ['live_logic_connected', 'verified']) ? 'real' : truthHasAny(backendTags, ['partially_wired', 'scaffolded', 'needs_backend_runtime']) ? 'partial' : props.capabilityState?.requires_backend_runtime ? 'blocked' : 'missing',
        data: truthHasAny(dataTags, ['verified']) ? 'real' : truthHasAny(dataTags, ['partially_wired', 'scaffolded']) ? 'partial' : 'missing',
        integrations: truthHasAny(integrationTags, ['live_logic_connected', 'verified']) ? 'real' : truthHasAny(integrationTags, ['partially_wired', 'needs_external_api']) ? 'partial' : 'missing',
        verification: verificationPassed('backend') ? 'real' : truthHasAny(backendTags, ['verified']) ? 'partial' : props.capabilityState?.requires_backend_runtime ? 'blocked' : 'missing',
        readiness: props.promotionDecision?.integration_ready ? 'real' : props.capabilityState?.requires_backend_runtime ? 'partial' : 'missing',
        gating: props.capabilityState?.requires_backend_runtime ? planGate : 'Not required',
      },
      {
        id: 'deployment',
        label: 'Deployment',
        ui: 'missing',
        backend: props.capabilityState?.requires_publish ? 'partial' : 'missing',
        data: 'missing',
        integrations: truthHasAny(deploymentTags, ['verified', 'production_candidate']) ? 'real' : truthHasAny(deploymentTags, ['scaffolded', 'partially_wired']) ? 'partial' : props.capabilityState?.requires_publish ? 'blocked' : 'missing',
        verification: verificationPassed('deployment') ? 'real' : truthHasAny(deploymentTags, ['verified']) ? 'partial' : 'missing',
        readiness: props.promotionDecision?.production_candidate ? 'real' : props.capabilityState?.requires_publish ? 'partial' : 'missing',
        gating: props.policyState?.publish_enabled ? 'Publish enabled' : 'Paid publish required',
      },
    ]
  }, [backendTags, dataTags, deploymentTags, frontendTags, integrationTags, props.capabilityState, props.policyState, props.promotionDecision, props.verificationReports])

  const latestCheckpoint = checkpoints.length > 0 ? checkpoints[checkpoints.length - 1] : undefined
  const restorableCheckpointCount = checkpoints.filter((checkpoint) => checkpoint.restorable !== false).length
  const pendingApprovals = requiredApprovals.filter((approval) => approval.status === 'pending').length
  const deniedApprovals = requiredApprovals.filter((approval) => approval.status === 'denied').length
  const mismatchedApprovals = requiredApprovals.filter((approval) => approval.mismatch_detected)
  const approvalHistory = [...(props.interaction?.approval_events || [])]
    .sort((left, right) => String(right.timestamp || '').localeCompare(String(left.timestamp || '')))
    .slice(0, 8)
  const topProviderScorecards = (props.providerScorecards || []).slice(0, 4)
  const recentFingerprints = (props.failureFingerprints || []).slice(0, 4)
  const verifiedSurfaceCount = Object.values(truthBySurface).filter((tags) => truthHasAny(tags, ['verified', 'production_candidate', 'production_ready'])).length
  const activeBlockerCount = blockers.length
  const unresolvedVerificationCount = (props.verificationReports || []).filter((report) => report.status !== 'passed').length
  const recoveredFailureCount = recentFingerprints.filter((fingerprint) => fingerprint.repair_success).length
  const readinessLabel = humanize(props.promotionDecision?.readiness_state || props.qualityGateStatus || 'pending')
  const classificationLabel = humanize(props.policyState?.classification || 'pending')
  const planLabel = humanize(props.policyState?.plan_type || 'unknown')
  const planSummary = props.policyState?.classification === 'upgrade_required'
    ? `This request crossed into paid capability territory. The system should still ship a truthful frontend preview now, while deferring backend/runtime scope until Builder or higher is active.`
    : props.policyState?.classification === 'static_ready'
      ? 'The current request fits the free static path. Backend, publish, and BYOK claims should stay off the table unless the user explicitly upgrades.'
      : 'The current plan allows the full-stack path, so orchestration can keep verifying runtime, integration, and promotion work instead of falling back to a mock.'
  const nextAction = blockers[0]?.unblocks_with
    || blockers[0]?.summary
    || props.interaction?.pending_question
    || props.interaction?.pause_reason
    || (props.promotionDecision?.production_candidate
      ? 'Deployment, secrets, and final release checks are the remaining high-value actions.'
      : 'Keep working through verification and repair evidence until promotion truth improves.')
  const architectureRisks = [
    props.capabilityState?.requires_external_api ? 'External APIs are part of the contract, so live readiness is sensitive to credentials, quotas, and third-party uptime.' : null,
    props.capabilityState?.requires_database ? 'Persistent data is in scope, so schema drift and environment configuration matter more than a pure static build.' : null,
    props.capabilityState?.requires_publish && !props.policyState?.publish_enabled ? 'Publish is requested but still gated by plan policy, so the build should stop short of implying a deployable release.' : null,
    contract?.verification_warnings?.[0] || null,
  ].filter(Boolean) as string[]
  const architectureAlternatives = [
    props.policyState?.classification === 'upgrade_required' ? 'Cheaper path: continue in static-only mode and defer backend/runtime scope until the app surface is validated.' : null,
    props.capabilityState?.requires_database ? 'Faster path: keep the first iteration file-backed or mocked until schema and auth requirements settle.' : null,
    props.capabilityState?.requires_backend_runtime ? 'Scalable path: preserve the current frontend contract while moving backend responsibilities behind a typed API boundary.' : null,
  ].filter(Boolean) as string[]

  return (
    <div className="space-y-6">
      <Card variant="cyberpunk" className="overflow-hidden border-2 border-gray-800 bg-black/70 backdrop-blur-md">
        <CardContent className="relative p-0">
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(34,211,238,0.18),transparent_35%),radial-gradient(circle_at_top_right,rgba(168,85,247,0.14),transparent_32%),linear-gradient(180deg,rgba(255,255,255,0.03),transparent_55%)]" />
          <div className="relative grid gap-6 px-6 py-6 xl:grid-cols-[1.4fr,0.9fr]">
            <div className="space-y-5">
              <div className="space-y-3">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="outline" className={cn('text-xs', classificationTone(props.policyState?.classification))}>
                    {classificationLabel}
                  </Badge>
                  <Badge variant="outline" className="text-xs border-violet-500/30 bg-violet-500/10 text-violet-200">
                    Readiness: {readinessLabel}
                  </Badge>
                  <Badge variant="outline" className="text-xs border-gray-700 bg-gray-950/70 text-gray-300">
                    Plan: {planLabel}
                  </Badge>
                </div>
                <div>
                  <div className="text-2xl font-semibold tracking-tight text-white">Orchestration summary</div>
                  <div className="mt-2 max-w-3xl text-sm leading-6 text-gray-300">
                    {planSummary}
                  </div>
                </div>
              </div>

              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                {[
                  { label: 'Verified surfaces', value: verifiedSurfaceCount, tone: 'text-emerald-300 border-emerald-500/25 bg-emerald-500/8' },
                  { label: 'Active blockers', value: activeBlockerCount, tone: activeBlockerCount > 0 ? 'text-amber-200 border-amber-500/25 bg-amber-500/8' : 'text-gray-200 border-gray-800 bg-gray-950/70' },
                  { label: 'Verification gaps', value: unresolvedVerificationCount, tone: unresolvedVerificationCount > 0 ? 'text-rose-200 border-rose-500/25 bg-rose-500/8' : 'text-gray-200 border-gray-800 bg-gray-950/70' },
                  { label: 'Recovered failures', value: recoveredFailureCount, tone: 'text-cyan-200 border-cyan-500/25 bg-cyan-500/8' },
                ].map((item) => (
                  <div key={item.label} className={cn('rounded-2xl border px-4 py-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]', item.tone)}>
                    <div className="text-[11px] uppercase tracking-[0.18em] text-gray-500">{item.label}</div>
                    <div className="mt-2 text-3xl font-semibold text-white">{item.value}</div>
                  </div>
                ))}
              </div>
            </div>

            <div className="grid gap-3">
              <div className="rounded-2xl border border-gray-800 bg-gray-950/70 p-4 shadow-[0_24px_60px_rgba(0,0,0,0.28)]">
                <div className="text-[11px] uppercase tracking-[0.18em] text-gray-500">Next action</div>
                <div className="mt-3 text-sm font-medium leading-6 text-white">{nextAction}</div>
              </div>
              <div className="rounded-2xl border border-gray-800 bg-gray-950/70 p-4 shadow-[0_24px_60px_rgba(0,0,0,0.28)]">
                <div className="text-[11px] uppercase tracking-[0.18em] text-gray-500">Capability pressure</div>
                <div className="mt-3 flex flex-wrap gap-2">
                  {requiredCapabilities.length === 0 ? (
                    <div className="text-sm text-gray-400">No capability detector output yet.</div>
                  ) : (
                    requiredCapabilities.map((capability) => (
                      <Badge key={capability} variant="outline" className="text-[11px] border-gray-700 bg-black/40 text-gray-300">
                        {humanize(capability)}
                      </Badge>
                    ))
                  )}
                </div>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm">
        <CardHeader className="pb-4 border-b border-gray-800">
          <CardTitle className="text-xl flex items-center gap-3">
            <Workflow className="w-7 h-7 text-cyan-400" />
            Build Timeline
          </CardTitle>
        </CardHeader>
        <CardContent className="pt-5 space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline" className={cn('text-xs', classificationTone(props.policyState?.classification))}>
              {humanize(props.policyState?.classification || 'pending')}
            </Badge>
            {props.policyState?.plan_type && (
              <Badge variant="outline" className="text-xs border-gray-700 bg-gray-950/60 text-gray-300">
                Plan: {props.policyState.plan_type}
              </Badge>
            )}
            {props.currentPhase && (
              <Badge variant="outline" className="text-xs border-gray-700 bg-gray-950/60 text-gray-300">
                Phase: {humanize(props.currentPhase)}
              </Badge>
            )}
          </div>

          <div className="grid gap-3 md:grid-cols-2">
            {timeline.map((phase) => (
              <div key={phase.id} className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="text-sm font-semibold text-white">{phase.label}</div>
                    <div className="mt-1 text-xs text-gray-400">{phase.description}</div>
                    <div className="mt-3 space-y-1">
                      {phase.substeps.map((substep, index) => (
                        <div key={`${phase.id}-${index}`} className="text-xs text-gray-300">
                          {substep}
                        </div>
                      ))}
                    </div>
                  </div>
                  <div className="flex flex-col items-end gap-2">
                    <Badge variant="outline" className={cn('text-[11px]', timelineStatusTone[phase.status])}>
                      {humanize(phase.status)}
                    </Badge>
                    <div className="text-[11px] text-gray-500">{phase.timestamp || 'Awaiting timestamp'}</div>
                  </div>
                </div>
              </div>
            ))}
          </div>

          {requiredCapabilities.length > 0 && (
            <div className="pt-2">
              <div className="text-xs uppercase tracking-wide text-gray-500">Capability Detector</div>
              <div className="mt-2 flex flex-wrap gap-2">
                {requiredCapabilities.map((capability) => (
                  <Badge key={capability} variant="outline" className="text-xs border-gray-700 bg-gray-950/60 text-gray-300">
                    {humanize(capability)}
                  </Badge>
                ))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <div className="grid gap-6 xl:grid-cols-2">
        <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm">
          <CardHeader className="pb-4 border-b border-gray-800">
            <CardTitle className="text-xl flex items-center gap-3">
              <Sparkles className="w-7 h-7 text-emerald-400" />
              Truth Tags
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-5 space-y-4">
            {surfaces.length === 0 ? (
              <div className="text-sm text-gray-500">Truth tags will appear as the build contract and promotion state mature.</div>
            ) : (
              surfaces.map(([surface, tags]) => (
                <div key={surface} className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
                  <div className="text-xs uppercase tracking-wide text-gray-500">{surface}</div>
                  <div className="mt-2 text-sm leading-6 text-gray-300">{truthNarrative(surface, tags)}</div>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {tags.map((tag) => (
                      <Badge key={`${surface}-${tag}`} variant="outline" className="text-xs border-cyan-500/30 bg-cyan-500/10 text-cyan-200">
                        {humanize(tag)}
                      </Badge>
                    ))}
                  </div>
                </div>
              ))
            )}
          </CardContent>
        </Card>

        <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm">
          <CardHeader className="pb-4 border-b border-gray-800">
            <CardTitle className="text-xl flex items-center gap-3">
              <AlertCircle className="w-7 h-7 text-amber-400" />
              Blockers
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-5 space-y-3">
            {blockers.length === 0 ? (
              <div className="rounded-xl border border-green-500/30 bg-green-500/10 p-4 text-sm text-green-200">
                No active blockers. The current plan and build state are clear.
              </div>
            ) : (
              blockers.map((blocker) => (
                <div key={blocker.id} className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="text-sm font-semibold text-white">{blocker.title}</div>
                      {blocker.summary && <div className="mt-1 text-sm text-gray-300">{blocker.summary}</div>}
                      <div className="mt-2 flex flex-wrap gap-2">
                        <Badge variant="outline" className="text-[11px] border-gray-700 bg-black/40 text-gray-300">
                          Category: {humanize(blocker.category)}
                        </Badge>
                        {blocker.who_must_act && (
                          <Badge variant="outline" className="text-[11px] border-gray-700 bg-black/40 text-gray-300">
                            Owner: {humanize(blocker.who_must_act)}
                          </Badge>
                        )}
                        <Badge variant="outline" className="text-[11px] border-gray-700 bg-black/40 text-gray-300">
                          Type: {humanize(blocker.type)}
                        </Badge>
                        <Badge
                          variant="outline"
                          className={cn(
                            'text-[11px]',
                            blocker.partial_progress_allowed
                              ? 'border-cyan-500/40 bg-cyan-500/10 text-cyan-300'
                              : 'border-gray-700 bg-black/40 text-gray-300',
                          )}
                        >
                          {blocker.partial_progress_allowed ? 'Partial work can continue' : 'Stops forward progress'}
                        </Badge>
                        {blocker.plan_tier_related && (
                          <Badge variant="outline" className="text-[11px] border-amber-500/40 bg-amber-500/10 text-amber-300">
                            Plan-tier blocker
                          </Badge>
                        )}
                      </div>
                      {blocker.unblocks_with && <div className="mt-2 text-xs text-gray-500">Unblock: {blocker.unblocks_with}</div>}
                    </div>
                    <Badge variant="outline" className={cn('text-[11px]', blockerTone(blocker.severity))}>
                      {humanize(blocker.severity)}
                    </Badge>
                  </div>
                </div>
              ))
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-6 xl:grid-cols-2">
        <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm">
          <CardHeader className="pb-4 border-b border-gray-800">
            <CardTitle className="text-xl flex items-center gap-3">
              <Clock3 className="w-7 h-7 text-cyan-400" />
              Build Journal
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-5 space-y-3">
            <div className="rounded-2xl border border-gray-800 bg-gray-950/70 p-3">
              <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                <label className="relative block lg:max-w-sm lg:flex-1">
                  <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-500" />
                  <input
                    value={journalQuery}
                    onChange={(event) => {
                      const value = event.target.value
                      startTransition(() => setJournalQuery(value))
                    }}
                    placeholder="Search request parsing, blockers, repairs, checkpoints..."
                    className="h-11 w-full rounded-xl border border-gray-800 bg-black/40 pl-10 pr-4 text-sm text-white outline-none transition focus:border-cyan-500/40 focus:bg-black/60"
                  />
                </label>
                <div className="flex flex-wrap items-center gap-2">
                  <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-gray-500">
                    <Filter className="h-3.5 w-3.5" />
                    Filter
                  </div>
                  {(['all', 'success', 'warning', 'info'] as const).map((tone) => (
                    <button
                      key={tone}
                      type="button"
                      onClick={() => setJournalToneFilter(tone)}
                      className={cn(
                        'rounded-full border px-3 py-1.5 text-[11px] font-medium uppercase tracking-[0.14em] transition',
                        journalToneFilter === tone
                          ? tone === 'all'
                            ? 'border-white/20 bg-white/10 text-white'
                            : journalToneBadge[tone]
                          : 'border-gray-800 bg-black/30 text-gray-400 hover:border-gray-700 hover:text-gray-200',
                      )}
                    >
                      {tone}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            {journal.length === 0 ? (
              <div className="text-sm text-gray-500">Journal entries will appear as orchestration state becomes available.</div>
            ) : filteredJournal.length === 0 ? (
              <div className="rounded-xl border border-gray-800 bg-gray-950/60 p-4 text-sm text-gray-400">
                No journal entries match the current search or tone filter.
              </div>
            ) : (
              filteredJournal.map((entry) => (
                <div key={entry.id} className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="text-sm font-semibold text-white">{entry.title}</div>
                      <div className="mt-1 text-sm text-gray-300">{entry.detail}</div>
                    </div>
                    <Badge
                      variant="outline"
                      className={cn(
                        'text-[11px]',
                        journalToneBadge[entry.tone],
                      )}
                    >
                      {entry.timestamp || 'Current'}
                    </Badge>
                  </div>
                </div>
              ))
            )}
          </CardContent>
        </Card>

        <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm">
          <CardHeader className="pb-4 border-b border-gray-800">
            <CardTitle className="text-xl flex items-center gap-3">
              <FileDiff className="w-7 h-7 text-violet-400" />
              Mock-To-Real Diff
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-5 space-y-4">
            <div className="space-y-3">
              {diffRows.map((row) => (
                <div key={row.id} className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
                  <div className="flex items-center justify-between gap-3">
                    <div className="text-sm font-semibold text-white">{row.label}</div>
                    <div className="text-xs text-gray-500">{row.gating}</div>
                  </div>
                  <div className="mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
                    {[
                      { label: 'UI', value: row.ui },
                      { label: 'Backend', value: row.backend },
                      { label: 'Data path', value: row.data },
                      { label: 'Integrations', value: row.integrations },
                      { label: 'Verification', value: row.verification },
                      { label: 'Readiness', value: row.readiness },
                    ].map((cell) => (
                      <div key={`${row.id}-${cell.label}`} className="rounded-lg border border-gray-800 bg-black/30 px-3 py-2">
                        <div className="text-[11px] uppercase tracking-wide text-gray-500">{cell.label}</div>
                        <Badge variant="outline" className={cn('mt-2 text-[11px]', statusTone(cell.value))}>
                          {humanize(cell.value)}
                        </Badge>
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>

      <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm">
        <CardHeader className="pb-4 border-b border-gray-800">
          <CardTitle className="text-xl flex items-center gap-3">
            <ShieldCheck className="w-7 h-7 text-violet-400" />
            Approvals And Readiness
          </CardTitle>
        </CardHeader>
        <CardContent className="pt-5">
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {requiredApprovals.length === 0 ? (
              <div className="rounded-xl border border-gray-800 bg-gray-950/60 p-4 text-sm text-gray-500">
                No durable approval summaries are available for this build yet.
              </div>
            ) : (
              requiredApprovals.map((approval) => (
                <div key={approval.id} className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="text-sm font-semibold text-white">{approval.title}</div>
                      <div className="mt-1 text-xs text-gray-400">{approval.summary || approval.reason}</div>
                      <div className="mt-2 flex flex-wrap gap-2">
                        {approval.source_type && (
                          <Badge variant="outline" className="text-[11px] border-gray-700 bg-black/40 text-gray-300">
                            Source: {humanize(approval.source_type)}
                          </Badge>
                        )}
                        {approval.actor && (
                          <Badge variant="outline" className="text-[11px] border-gray-700 bg-black/40 text-gray-300">
                            Owner: {humanize(approval.actor)}
                          </Badge>
                        )}
                        {approval.acknowledgement_required && (
                          <Badge variant="outline" className="text-[11px] border-amber-500/40 bg-amber-500/10 text-amber-300">
                            Ack required
                          </Badge>
                        )}
                        {approval.mismatch_detected && (
                          <Badge variant="outline" className="text-[11px] border-red-500/40 bg-red-500/10 text-red-300">
                            Mismatch detected
                          </Badge>
                        )}
                      </div>
                      {approval.mismatch_reason && (
                        <div className="mt-2 text-xs text-red-300">{approval.mismatch_reason}</div>
                      )}
                    </div>
                    <Badge variant="outline" className={cn('text-[11px]', approvalTone(approval.status))}>
                      {humanize(approval.status)}
                    </Badge>
                  </div>
                </div>
              ))
            )}
          </div>

          <div className="mt-5 grid gap-3 md:grid-cols-3">
            <div className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-white">
                <Layers3 className="w-4 h-4 text-cyan-400" />
                Work Orders
              </div>
              <div className="mt-2 text-2xl font-semibold text-white">{props.workOrders?.length || 0}</div>
            </div>
            <div className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-white">
                <Clock3 className="w-4 h-4 text-amber-400" />
                Verification Reports
              </div>
              <div className="mt-2 text-2xl font-semibold text-white">{props.verificationReports?.length || 0}</div>
            </div>
            <div className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-white">
                <CheckCircle2 className="w-4 h-4 text-green-400" />
                Readiness
              </div>
              <div className="mt-2 text-lg font-semibold text-white">
                {humanize(props.promotionDecision?.readiness_state || props.qualityGateStatus || 'pending')}
              </div>
            </div>
          </div>

          <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            <div className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
              <div className="text-xs uppercase tracking-wide text-gray-500">Approval Audit</div>
              <div className="mt-3 flex flex-wrap gap-2">
                <Badge variant="outline" className="text-[11px] border-amber-500/40 bg-amber-500/10 text-amber-300">
                  Pending: {pendingApprovals}
                </Badge>
                <Badge variant="outline" className="text-[11px] border-red-500/40 bg-red-500/10 text-red-300">
                  Denied: {deniedApprovals}
                </Badge>
                <Badge variant="outline" className="text-[11px] border-violet-500/40 bg-violet-500/10 text-violet-300">
                  Mismatch: {mismatchedApprovals.length}
                </Badge>
              </div>
              {(props.interaction?.waiting_for_user || props.interaction?.paused) && (
                <div className="mt-3 text-sm text-amber-200">
                  {props.interaction.paused ? 'Paused until review completes.' : 'Waiting on user acknowledgement or permission resolution.'}
                </div>
              )}
            </div>

            <div className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
              <div className="text-xs uppercase tracking-wide text-gray-500">Checkpoint Continuity</div>
              <div className="mt-3 text-sm text-white">
                {checkpoints.length === 0 ? 'No persisted checkpoints yet.' : `${checkpoints.length} checkpoint${checkpoints.length === 1 ? '' : 's'} captured.`}
              </div>
              {latestCheckpoint && (
                <div className="mt-2 text-xs text-gray-400">
                  Latest: {latestCheckpoint.name} · {formatTimestamp(latestCheckpoint.created_at || latestCheckpoint.createdAt) || 'Timestamp unavailable'}
                </div>
              )}
              <div className="mt-3 flex flex-wrap gap-2">
                <Badge variant="outline" className="text-[11px] border-gray-700 bg-black/40 text-gray-300">
                  Restorable: {restorableCheckpointCount}
                </Badge>
                <Badge variant="outline" className="text-[11px] border-gray-700 bg-black/40 text-gray-300">
                  Waiting: {props.interaction?.waiting_for_user ? 'yes' : 'no'}
                </Badge>
              </div>
            </div>

            <div className="rounded-xl border border-gray-800 bg-gray-950/60 p-4 md:col-span-2 xl:col-span-1">
              <div className="text-xs uppercase tracking-wide text-gray-500">Approval History</div>
              <div className="mt-3 space-y-3">
                {approvalHistory.length === 0 ? (
                  <div className="text-sm text-gray-500">No durable approval events recorded yet.</div>
                ) : (
                  approvalHistory.map((event) => (
                    <div key={event.id} className="rounded-lg border border-gray-800 bg-black/30 p-3">
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div className="text-sm font-semibold text-white">{event.title}</div>
                          <div className="mt-1 text-xs text-gray-400">{event.summary || humanize(event.kind)}</div>
                          <div className="mt-2 flex flex-wrap gap-2">
                            {event.source_type && (
                              <Badge variant="outline" className="text-[11px] border-gray-700 bg-black/40 text-gray-300">
                                Source: {humanize(event.source_type)}
                              </Badge>
                            )}
                            {event.actor && (
                              <Badge variant="outline" className="text-[11px] border-gray-700 bg-black/40 text-gray-300">
                                Actor: {humanize(event.actor)}
                              </Badge>
                            )}
                          </div>
                        </div>
                        <div className="flex flex-col items-end gap-2">
                          <Badge variant="outline" className={cn('text-[11px]', approvalEventTone(event.status))}>
                            {humanize(event.status)}
                          </Badge>
                          <div className="text-[11px] text-gray-500">{formatTimestamp(event.timestamp) || 'Current'}</div>
                        </div>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm">
        <CardHeader className="pb-4 border-b border-gray-800">
          <CardTitle className="text-xl flex items-center gap-3">
            <Layers3 className="w-7 h-7 text-orange-400" />
            Architecture Explainer
          </CardTitle>
        </CardHeader>
          <CardContent className="pt-5 space-y-4">
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {architectureItems.map((item) => (
              <div key={item.label} className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
                <div className="text-xs uppercase tracking-wide text-gray-500">{item.label}</div>
                <div className="mt-2 text-sm font-semibold text-white">{item.value || 'Not specified'}</div>
              </div>
            ))}
          </div>

          <div className="grid gap-4 xl:grid-cols-2">
            <div className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
              <div className="text-xs uppercase tracking-wide text-gray-500">Primary risks</div>
              <div className="mt-3 space-y-2">
                {architectureRisks.length === 0 ? (
                  <div className="text-sm text-gray-400">No major architecture risks have been inferred yet.</div>
                ) : (
                  architectureRisks.slice(0, 4).map((risk) => (
                    <div key={risk} className="rounded-lg border border-amber-500/20 bg-amber-500/6 px-3 py-2 text-sm leading-6 text-amber-100">
                      {risk}
                    </div>
                  ))
                )}
              </div>
            </div>

            <div className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
              <div className="text-xs uppercase tracking-wide text-gray-500">Cheaper, faster, and scalable alternatives</div>
              <div className="mt-3 space-y-2">
                {architectureAlternatives.length === 0 ? (
                  <div className="text-sm text-gray-400">No alternative path suggestions have been inferred yet.</div>
                ) : (
                  architectureAlternatives.slice(0, 4).map((option) => (
                    <div key={option} className="rounded-lg border border-cyan-500/20 bg-cyan-500/6 px-3 py-2 text-sm leading-6 text-cyan-100">
                      {option}
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>

          {(contract?.verification_warnings && contract.verification_warnings.length > 0) && (
            <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4">
              <div className="text-xs uppercase tracking-wide text-amber-300">Tradeoffs And Warnings</div>
              <div className="mt-2 space-y-1 text-sm text-amber-100">
                {contract.verification_warnings.slice(0, 4).map((warning) => (
                  <div key={warning}>{warning}</div>
                ))}
              </div>
            </div>
          )}

          {(contract?.env_var_contract && contract.env_var_contract.length > 0) && (
            <div className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
              <div className="text-xs uppercase tracking-wide text-gray-500">Secrets And External Dependencies</div>
              <div className="mt-2 flex flex-wrap gap-2">
                {contract.env_var_contract.slice(0, 6).map((envVar) => (
                  <Badge key={envVar.name} variant="outline" className="text-xs border-gray-700 bg-gray-950/60 text-gray-300">
                    {envVar.name}
                  </Badge>
                ))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <div className="grid gap-6 xl:grid-cols-2">
        <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm">
          <CardHeader className="pb-4 border-b border-gray-800">
            <CardTitle className="text-xl flex items-center gap-3">
              <Sparkles className="w-7 h-7 text-cyan-400" />
              Provider Scorecards
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-5 space-y-3">
            {topProviderScorecards.length === 0 ? (
              <div className="text-sm text-gray-500">Provider performance data will appear after orchestration captures retries and verification outcomes.</div>
            ) : (
              topProviderScorecards.map((scorecard) => (
                <div key={`${scorecard.provider}-${scorecard.task_shape}`} className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <div className="text-sm font-semibold text-white">{scorecard.provider}</div>
                      <div className="mt-1 text-xs text-gray-400">{humanize(scorecard.task_shape)}</div>
                    </div>
                    <Badge variant="outline" className={cn('text-[11px]', scorecard.hosted_eligible ? 'border-green-500/40 bg-green-500/10 text-green-300' : 'border-gray-700 bg-black/40 text-gray-300')}>
                      {scorecard.hosted_eligible ? 'Hosted eligible' : 'BYOK only'}
                    </Badge>
                  </div>
                  <div className="mt-3 grid gap-2 sm:grid-cols-2">
                    <div className="rounded-lg border border-gray-800 bg-black/30 px-3 py-2">
                      <div className="text-[11px] uppercase tracking-wide text-gray-500">First-pass verify</div>
                      <div className="mt-1 text-sm text-white">{scorecard.first_pass_verification_pass_rate ?? 0}%</div>
                    </div>
                    <div className="rounded-lg border border-gray-800 bg-black/30 px-3 py-2">
                      <div className="text-[11px] uppercase tracking-wide text-gray-500">Repair success</div>
                      <div className="mt-1 text-sm text-white">{scorecard.repair_success_rate ?? 0}%</div>
                    </div>
                    <div className="rounded-lg border border-gray-800 bg-black/30 px-3 py-2">
                      <div className="text-[11px] uppercase tracking-wide text-gray-500">Avg latency</div>
                      <div className="mt-1 text-sm text-white">{scorecard.average_latency_seconds ?? 0}s</div>
                    </div>
                    <div className="rounded-lg border border-gray-800 bg-black/30 px-3 py-2">
                      <div className="text-[11px] uppercase tracking-wide text-gray-500">Avg cost</div>
                      <div className="mt-1 text-sm text-white">${scorecard.average_cost_per_success ?? 0}</div>
                    </div>
                  </div>
                </div>
              ))
            )}
          </CardContent>
        </Card>

        <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm">
          <CardHeader className="pb-4 border-b border-gray-800">
            <CardTitle className="text-xl flex items-center gap-3">
              <AlertCircle className="w-7 h-7 text-rose-400" />
              Repair Signals
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-5 space-y-3">
            {recentFingerprints.length === 0 ? (
              <div className="text-sm text-gray-500">Failure fingerprints will appear when the repair ladder records repeated failure classes.</div>
            ) : (
              recentFingerprints.map((fingerprint) => (
                <div key={fingerprint.id} className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="text-sm font-semibold text-white">{humanize(fingerprint.failure_class || 'unknown failure')}</div>
                      <div className="mt-1 text-xs text-gray-400">
                        {fingerprint.provider || 'Unknown provider'} · {humanize(fingerprint.task_shape || 'unspecified task')}
                      </div>
                    </div>
                    <Badge variant="outline" className={cn('text-[11px]', fingerprint.repair_success ? 'border-green-500/40 bg-green-500/10 text-green-300' : 'border-amber-500/40 bg-amber-500/10 text-amber-300')}>
                      {fingerprint.repair_success ? 'Recovered' : 'Needs repair'}
                    </Badge>
                  </div>
                  {Array.isArray(fingerprint.repair_path_chosen) && fingerprint.repair_path_chosen.length > 0 && (
                    <div className="mt-3 flex flex-wrap gap-2">
                      {fingerprint.repair_path_chosen.map((path) => (
                        <Badge key={`${fingerprint.id}-${path}`} variant="outline" className="text-[11px] border-gray-700 bg-black/40 text-gray-300">
                          {humanize(path)}
                        </Badge>
                      ))}
                    </div>
                  )}
                  {(fingerprint.repair_strategy || fingerprint.patch_class) && (
                    <div className="mt-3 text-xs text-gray-400">
                      {[
                        fingerprint.repair_strategy && `Strategy: ${humanize(fingerprint.repair_strategy)}`,
                        fingerprint.patch_class && `Patch: ${humanize(fingerprint.patch_class)}`,
                      ].filter(Boolean).join(' | ')}
                    </div>
                  )}
                </div>
              ))
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

export default OrchestrationOverview
