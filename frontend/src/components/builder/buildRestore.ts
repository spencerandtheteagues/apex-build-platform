import type { CompletedBuildDetail } from '@/services/api'

const TERMINAL_BUILD_STATUSES = new Set(['completed', 'failed', 'cancelled'])
const AI_THOUGHT_TYPES = new Set(['thinking', 'action', 'output', 'error'])
const MAX_TELEMETRY_BUILD_SNAPSHOTS = 20
const MAX_TELEMETRY_THOUGHTS = 160
const NORMALIZED_BUILD_STATUSES = new Set([
  'idle',
  'pending',
  'planning',
  'in_progress',
  'testing',
  'reviewing',
  'awaiting_review',
  'completed',
  'failed',
  'cancelled',
])

const BUILD_STATUS_ALIASES: Record<string, string> = {
  building: 'in_progress',
  running: 'in_progress',
  inprogress: 'in_progress',
  success: 'completed',
  succeeded: 'completed',
  done: 'completed',
  error: 'failed',
  canceled: 'cancelled',
}

export interface PersistedAIThought {
  id: string
  agentId: string
  agentRole: string
  provider: string
  model?: string
  type: 'thinking' | 'action' | 'output' | 'error'
  eventType?: string
  taskId?: string
  taskType?: string
  files?: string[]
  filesCount?: number
  retryCount?: number
  maxRetries?: number
  isInternal?: boolean
  content: string
  timestamp: string
}

export interface BuildTelemetrySnapshot {
  buildId: string
  updatedAt: string
  thoughts: PersistedAIThought[]
}

const isFiniteOptionalNumber = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value)

const readStringCandidate = (candidate: Record<string, unknown>, ...keys: string[]): string | undefined => {
  for (const key of keys) {
    const value = candidate[key]
    if (typeof value === 'string' && value.trim()) {
      return value
    }
  }
  return undefined
}

const readNumberCandidate = (candidate: Record<string, unknown>, ...keys: string[]): number | undefined => {
  for (const key of keys) {
    if (isFiniteOptionalNumber(candidate[key])) {
      return candidate[key] as number
    }
  }
  return undefined
}

const readBooleanCandidate = (candidate: Record<string, unknown>, ...keys: string[]): boolean | undefined => {
  for (const key of keys) {
    if (typeof candidate[key] === 'boolean') {
      return candidate[key] as boolean
    }
  }
  return undefined
}

const normalizePersistedAIThought = (value: unknown): PersistedAIThought | null => {
  if (!value || typeof value !== 'object') return null
  const candidate = value as Record<string, unknown>
  const id = readStringCandidate(candidate, 'id')
  const agentId = readStringCandidate(candidate, 'agentId', 'agent_id')
  const agentRole = readStringCandidate(candidate, 'agentRole', 'agent_role')
  const provider = readStringCandidate(candidate, 'provider')
  const content = readStringCandidate(candidate, 'content')
  const timestamp = readStringCandidate(candidate, 'timestamp')
  if (
    !id ||
    !agentId ||
    !agentRole ||
    !provider ||
    !content ||
    !timestamp
  ) {
    return null
  }

  const rawType = readStringCandidate(candidate, 'type')
  const type = rawType && AI_THOUGHT_TYPES.has(rawType)
    ? rawType as PersistedAIThought['type']
    : null
  if (!type) return null

  const files = Array.isArray(candidate.files)
    ? candidate.files.filter((entry): entry is string => typeof entry === 'string')
    : undefined

  return {
    id,
    agentId,
    agentRole,
    provider,
    model: readStringCandidate(candidate, 'model'),
    type,
    eventType: readStringCandidate(candidate, 'eventType', 'event_type'),
    taskId: readStringCandidate(candidate, 'taskId', 'task_id'),
    taskType: readStringCandidate(candidate, 'taskType', 'task_type'),
    files: files && files.length > 0 ? files : undefined,
    filesCount: readNumberCandidate(candidate, 'filesCount', 'files_count'),
    retryCount: readNumberCandidate(candidate, 'retryCount', 'retry_count'),
    maxRetries: readNumberCandidate(candidate, 'maxRetries', 'max_retries'),
    isInternal: readBooleanCandidate(candidate, 'isInternal', 'is_internal'),
    content,
    timestamp,
  }
}

const parseTelemetryCache = (raw: string | null | undefined): Record<string, BuildTelemetrySnapshot> => {
  if (!raw) return {}

  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return {}

    const snapshots: Record<string, BuildTelemetrySnapshot> = {}
    for (const [buildId, value] of Object.entries(parsed as Record<string, unknown>)) {
      if (!value || typeof value !== 'object') continue
      const candidate = value as Record<string, unknown>
      const thoughts = Array.isArray(candidate.thoughts)
        ? candidate.thoughts
          .map((thought) => normalizePersistedAIThought(thought))
          .filter((thought): thought is PersistedAIThought => thought !== null)
          .slice(-MAX_TELEMETRY_THOUGHTS)
        : []
      const normalizedBuildId = typeof candidate.buildId === 'string' ? candidate.buildId : buildId
      if (!normalizedBuildId || thoughts.length === 0) continue
      snapshots[normalizedBuildId] = {
        buildId: normalizedBuildId,
        updatedAt: typeof candidate.updatedAt === 'string' && candidate.updatedAt.trim()
          ? candidate.updatedAt
          : thoughts[thoughts.length - 1].timestamp,
        thoughts,
      }
    }
    return snapshots
  } catch {
    return {}
  }
}

export const readBuildTelemetrySnapshot = (
  raw: string | null | undefined,
  buildId: string
): BuildTelemetrySnapshot | null => {
  if (!buildId) return null
  const cache = parseTelemetryCache(raw)
  return cache[buildId] || null
}

export const upsertBuildTelemetrySnapshot = (
  raw: string | null | undefined,
  snapshot: BuildTelemetrySnapshot
): string => {
  const cache = parseTelemetryCache(raw)
  const thoughts = snapshot.thoughts
    .map((thought) => normalizePersistedAIThought(thought))
    .filter((thought): thought is PersistedAIThought => thought !== null)
    .slice(-MAX_TELEMETRY_THOUGHTS)

  if (snapshot.buildId && thoughts.length > 0) {
    cache[snapshot.buildId] = {
      buildId: snapshot.buildId,
      updatedAt: snapshot.updatedAt || thoughts[thoughts.length - 1].timestamp,
      thoughts,
    }
  }

  const nextEntries = Object.values(cache)
    .sort((left, right) => {
      const leftTime = Date.parse(left.updatedAt)
      const rightTime = Date.parse(right.updatedAt)
      return (Number.isFinite(rightTime) ? rightTime : 0) - (Number.isFinite(leftTime) ? leftTime : 0)
    })
    .slice(0, MAX_TELEMETRY_BUILD_SNAPSHOTS)

  return JSON.stringify(
    nextEntries.reduce<Record<string, BuildTelemetrySnapshot>>((acc, entry) => {
      acc[entry.buildId] = entry
      return acc
    }, {})
  )
}

export const parseBuildTelemetryThoughts = (value: unknown): PersistedAIThought[] => {
  if (!Array.isArray(value)) return []
  return value
    .map((thought) => normalizePersistedAIThought(thought))
    .filter((thought): thought is PersistedAIThought => thought !== null)
    .slice(-MAX_TELEMETRY_THOUGHTS)
}

export const isTerminalBuildStatus = (status: string): boolean => {
  return TERMINAL_BUILD_STATUSES.has(status)
}

export const normalizeBuildStatus = (status: unknown): string | null => {
  if (typeof status !== 'string') return null
  const normalized = status.trim().toLowerCase()
  if (!normalized) return null
  const aliased = BUILD_STATUS_ALIASES[normalized] || normalized
  return NORMALIZED_BUILD_STATUSES.has(aliased) ? aliased : null
}

export const mergeBuildStatusWithTerminalPrecedence = (
  prevStatus: string | undefined,
  incomingStatus: unknown,
  options?: { allowTerminalRevival?: boolean }
): string | undefined => {
  const normalizedIncoming = normalizeBuildStatus(incomingStatus)
  if (!normalizedIncoming) return undefined
  if (
    prevStatus &&
    isTerminalBuildStatus(prevStatus) &&
    !isTerminalBuildStatus(normalizedIncoming) &&
    !options?.allowTerminalRevival
  ) {
    return prevStatus
  }
  return normalizedIncoming
}

export const resolveBuildCompletedEventStatus = (status: unknown): 'completed' | 'failed' => {
  const normalized = normalizeBuildStatus(status)
  if (!normalized) return 'completed'
  return normalized === 'failed' || normalized === 'cancelled' ? 'failed' : 'completed'
}

export const extractBuildFailureReason = (payload: Record<string, any> | null | undefined): string | undefined => {
  if (!payload || typeof payload !== 'object') return undefined

  const candidates = [payload.details, payload.error_detail, payload.error, payload.message]
  for (const value of candidates) {
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  return undefined
}

export const reconcileBuildPayloadWithCompletedDetail = (
  payload: Record<string, any>,
  completed?: CompletedBuildDetail
): Record<string, any> => {
  if (!completed) return payload

  const completedStatus = String(completed.status || '').trim()
  if (!isTerminalBuildStatus(completedStatus)) {
    return payload
  }

  const next = { ...payload }
  const completedFiles = Array.isArray(completed.files) ? completed.files : []
  const payloadHasFiles = Array.isArray(next.files) && next.files.length > 0

  next.id = next.id || completed.build_id
  next.build_id = next.build_id || completed.build_id
  next.project_id = next.project_id ?? completed.project_id ?? null
  next.description = next.description || completed.description || ''
  next.mode = next.mode || completed.mode
  next.power_mode = next.power_mode || completed.power_mode
  next.status = completedStatus
  next.live = false
  next.resumable = false
  if (!next.error && completed.error) {
    next.error = completed.error
  }

  if (!payloadHasFiles && completedFiles.length > 0) {
    next.files = completedFiles
  }
  if ((!Array.isArray(next.agents) || next.agents.length === 0) && Array.isArray(completed.agents) && completed.agents.length > 0) {
    next.agents = completed.agents
  }
  if ((!Array.isArray(next.tasks) || next.tasks.length === 0) && Array.isArray(completed.tasks) && completed.tasks.length > 0) {
    next.tasks = completed.tasks
  }
  if ((!Array.isArray(next.checkpoints) || next.checkpoints.length === 0) && Array.isArray(completed.checkpoints) && completed.checkpoints.length > 0) {
    next.checkpoints = completed.checkpoints
  }
  if ((!Array.isArray(next.activity_timeline) || next.activity_timeline.length === 0) && Array.isArray(completed.activity_timeline) && completed.activity_timeline.length > 0) {
    next.activity_timeline = completed.activity_timeline
  }
  if (!readStringCandidate(next, 'phase', 'current_phase', 'currentPhase') && typeof completed.current_phase === 'string' && completed.current_phase.trim()) {
    next.current_phase = completed.current_phase
  }
  if (typeof next.quality_gate_required !== 'boolean' && typeof completed.quality_gate_required === 'boolean') {
    next.quality_gate_required = completed.quality_gate_required
  }
  if (!readStringCandidate(next, 'quality_gate_status') && typeof completed.quality_gate_status === 'string') {
    next.quality_gate_status = completed.quality_gate_status
  }
  if (!readStringCandidate(next, 'quality_gate_stage') && typeof completed.quality_gate_stage === 'string' && completed.quality_gate_stage.trim()) {
    next.quality_gate_stage = completed.quality_gate_stage
  }
  if (typeof next.quality_gate_active !== 'boolean' && typeof completed.quality_gate_active === 'boolean') {
    next.quality_gate_active = completed.quality_gate_active
  }
  if (typeof next.quality_gate_passed !== 'boolean' && typeof completed.quality_gate_passed === 'boolean') {
    next.quality_gate_passed = completed.quality_gate_passed
  }
  if ((!Array.isArray(next.available_providers) || next.available_providers.length === 0) && Array.isArray(completed.available_providers) && completed.available_providers.length > 0) {
    next.available_providers = completed.available_providers
  }
  if (!next.capability_state && completed.capability_state) {
    next.capability_state = completed.capability_state
  }
  if (!next.policy_state && completed.policy_state) {
    next.policy_state = completed.policy_state
  }
  if ((!Array.isArray(next.blockers) || next.blockers.length === 0) && Array.isArray(completed.blockers) && completed.blockers.length > 0) {
    next.blockers = completed.blockers
  }
  if ((!Array.isArray(next.approvals) || next.approvals.length === 0) && Array.isArray(completed.approvals) && completed.approvals.length > 0) {
    next.approvals = completed.approvals
  }
  if (!next.intent_brief && completed.intent_brief) {
    next.intent_brief = completed.intent_brief
  }
  if (!next.build_contract && completed.build_contract) {
    next.build_contract = completed.build_contract
  }
  if ((!Array.isArray(next.work_orders) || next.work_orders.length === 0) && Array.isArray(completed.work_orders) && completed.work_orders.length > 0) {
    next.work_orders = completed.work_orders
  }
  if ((!Array.isArray(next.patch_bundles) || next.patch_bundles.length === 0) && Array.isArray(completed.patch_bundles) && completed.patch_bundles.length > 0) {
    next.patch_bundles = completed.patch_bundles
  }
  if ((!Array.isArray(next.verification_reports) || next.verification_reports.length === 0) && Array.isArray(completed.verification_reports) && completed.verification_reports.length > 0) {
    next.verification_reports = completed.verification_reports
  }
  if (!next.promotion_decision && completed.promotion_decision) {
    next.promotion_decision = completed.promotion_decision
  }
  if ((!Array.isArray(next.provider_scorecards) || next.provider_scorecards.length === 0) && Array.isArray(completed.provider_scorecards) && completed.provider_scorecards.length > 0) {
    next.provider_scorecards = completed.provider_scorecards
  }
  if ((!Array.isArray(next.failure_fingerprints) || next.failure_fingerprints.length === 0) && Array.isArray(completed.failure_fingerprints) && completed.failure_fingerprints.length > 0) {
    next.failure_fingerprints = completed.failure_fingerprints
  }
  if (!next.historical_learning && completed.historical_learning) {
    next.historical_learning = completed.historical_learning
  }
  if (!next.truth_by_surface && completed.truth_by_surface) {
    next.truth_by_surface = completed.truth_by_surface
  }

  if (typeof completed.progress === 'number') {
    next.progress = completedStatus === 'completed'
      ? Math.max(100, completed.progress)
      : completed.progress
  } else if (completedStatus === 'completed') {
    next.progress = 100
  }

  return next
}
