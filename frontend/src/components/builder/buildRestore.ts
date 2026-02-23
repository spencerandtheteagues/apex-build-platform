import type { CompletedBuildDetail } from '@/services/api'

const TERMINAL_BUILD_STATUSES = new Set(['completed', 'failed', 'cancelled'])
const NORMALIZED_BUILD_STATUSES = new Set([
  'idle',
  'pending',
  'planning',
  'in_progress',
  'testing',
  'reviewing',
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
  incomingStatus: unknown
): string | undefined => {
  const normalizedIncoming = normalizeBuildStatus(incomingStatus)
  if (!normalizedIncoming) return undefined
  if (prevStatus && isTerminalBuildStatus(prevStatus) && !isTerminalBuildStatus(normalizedIncoming)) {
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

  if (typeof completed.progress === 'number') {
    next.progress = completedStatus === 'completed'
      ? Math.max(100, completed.progress)
      : completed.progress
  } else if (completedStatus === 'completed') {
    next.progress = 100
  }

  return next
}
