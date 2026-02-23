import type { CompletedBuildDetail } from '@/services/api'

const TERMINAL_BUILD_STATUSES = new Set(['completed', 'failed', 'cancelled'])

export const isTerminalBuildStatus = (status: string): boolean => {
  return TERMINAL_BUILD_STATUSES.has(status)
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

