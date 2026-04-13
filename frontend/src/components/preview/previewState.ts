import type { PreviewStatus, ServerDetection, ServerStatus } from './types'

export type PreviewRuntimeState = 'starting' | 'running' | 'degraded' | 'backend_down' | 'failed' | 'stopped'

export interface PreviewRuntimeStateInput {
  loading: boolean
  status: PreviewStatus | null
  connected?: boolean
  error?: string | null
  iframeError?: string | null
  sandboxDegraded?: boolean
  serverDetection?: ServerDetection | null
  serverStatus?: ServerStatus | null
  backendPreviewAvailable?: boolean
}

export const previewRuntimeStateLabels: Record<PreviewRuntimeState, string> = {
  starting: 'Starting',
  running: 'Running',
  degraded: 'Degraded',
  backend_down: 'Backend Down',
  failed: 'Failed',
  stopped: 'Stopped',
}

export function derivePreviewRuntimeState({
  loading,
  status,
  connected,
  error,
  iframeError,
  sandboxDegraded = false,
  serverDetection,
  serverStatus,
  backendPreviewAvailable = true,
}: PreviewRuntimeStateInput): PreviewRuntimeState {
  if (error || iframeError) return 'failed'
  if (loading && !status?.active) return 'starting'
  if (!status?.active) return 'stopped'
  if (serverDetection?.has_backend && backendPreviewAvailable && !serverStatus?.running) return 'backend_down'
  if (connected === false) return 'degraded'
  if (sandboxDegraded) return 'degraded'
  return 'running'
}
