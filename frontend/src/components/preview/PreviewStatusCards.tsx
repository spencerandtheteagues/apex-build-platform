import type { PreviewStatus, ServerDetection, ServerStatus } from './types'
import {
  deriveBrowserLocalPreviewRoute,
  previewRuntimeStateLabels,
  type BrowserLocalPreviewCapability,
  type PreviewRuntimeState,
} from './previewState'

interface PreviewStatusCardsProps {
  status: PreviewStatus | null
  runtimeState: PreviewRuntimeState
  activeSandbox: boolean
  sandboxRequired: boolean
  sandboxDegraded: boolean
  serverDetection: ServerDetection | null
  serverStatus: ServerStatus | null
  backendPreviewAvailable: boolean
  backendPreviewReason: string
  bundlerAvailable: boolean
  browserLocalPreviewCapability: BrowserLocalPreviewCapability
  autoRefreshEnabled: boolean
  showDevTools: boolean
  customPath: string
}

export default function PreviewStatusCards({
  status,
  runtimeState,
  activeSandbox,
  sandboxRequired,
  sandboxDegraded,
  serverDetection,
  serverStatus,
  backendPreviewAvailable,
  backendPreviewReason,
  bundlerAvailable,
  browserLocalPreviewCapability,
  autoRefreshEnabled,
  showDevTools,
  customPath,
}: PreviewStatusCardsProps) {
  const runtimeLabel = previewRuntimeStateLabels[runtimeState]
  const runtimeSummary = status?.active
    ? runtimeState === 'running'
      ? `Running on :${status.port}`
      : `${runtimeLabel} · :${status.port}`
    : runtimeLabel
  const browserLocalDetail = browserLocalPreviewCapability.blockers.length > 0
    ? browserLocalPreviewCapability.blockers.slice(0, 2).join(' · ')
    : browserLocalPreviewCapability.reason
  const browserLocalRoute = deriveBrowserLocalPreviewRoute({
    serverDetection,
    bundlerAvailable,
    capability: browserLocalPreviewCapability,
  })

  return (
    <div className="grid grid-cols-1 gap-2 border-b border-gray-800 bg-gray-950/70 px-3 py-3 md:grid-cols-2 xl:grid-cols-4">
      <div className="rounded-lg border border-gray-800 bg-gray-900/70 px-3 py-3">
        <div className="text-[10px] uppercase tracking-[0.18em] text-gray-500">Preview Runtime</div>
        <div className="mt-2 text-sm font-semibold text-white">{runtimeSummary}</div>
        <div className="mt-1 text-xs text-gray-400">
          {activeSandbox ? 'Docker sandbox' : sandboxRequired && sandboxDegraded ? 'Process fallback mode' : 'Process mode'}
        </div>
      </div>
      <div className="rounded-lg border border-gray-800 bg-gray-900/70 px-3 py-3">
        <div className="text-[10px] uppercase tracking-[0.18em] text-gray-500">Backend API</div>
        <div className="mt-2 text-sm font-semibold text-white">
          {!serverDetection?.has_backend
            ? 'Not detected'
            : !backendPreviewAvailable
              ? 'Preview disabled'
              : serverStatus?.running
                ? `Running on :${serverStatus.port}`
                : 'Detected, stopped'}
        </div>
        <div className="mt-1 text-xs text-gray-400">
          {serverDetection?.framework || serverDetection?.server_type || backendPreviewReason || 'No backend runtime'}
        </div>
      </div>
      <div className="rounded-lg border border-gray-800 bg-gray-900/70 px-3 py-3">
        <div className="text-[10px] uppercase tracking-[0.18em] text-gray-500">Environment</div>
        <div className="mt-2 text-sm font-semibold text-white">{bundlerAvailable ? 'Bundler ready' : 'Bundler unavailable'}</div>
        <div className="mt-1 text-xs text-gray-400">
          Auto-refresh {autoRefreshEnabled ? 'on' : 'off'} · DevTools {showDevTools ? 'on' : 'off'}
        </div>
        <div className="mt-1 truncate text-xs text-gray-500" title={browserLocalDetail}>
          Browser-local {browserLocalPreviewCapability.label.toLowerCase()}: {browserLocalDetail}
        </div>
        <div className="mt-1 truncate text-xs text-gray-500" title={browserLocalRoute.reason}>
          Candidate route: {browserLocalRoute.label}
        </div>
      </div>
      <div className="rounded-lg border border-gray-800 bg-gray-900/70 px-3 py-3">
        <div className="text-[10px] uppercase tracking-[0.18em] text-gray-500">Route</div>
        <div className="mt-2 truncate text-sm font-semibold text-white">{customPath.trim() || '/'}</div>
        <div className="mt-1 truncate text-xs text-gray-400">
          {status?.started_at ? `Started ${new Date(status.started_at).toLocaleTimeString()}` : 'Not started yet'}
        </div>
      </div>
    </div>
  )
}
