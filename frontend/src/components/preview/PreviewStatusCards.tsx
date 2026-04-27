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
  browserLocalPreviewRuntimeEnabled?: boolean
  browserLocalPreviewRuntimeAvailable?: boolean
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
  browserLocalPreviewRuntimeEnabled = false,
  browserLocalPreviewRuntimeAvailable = false,
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
    browserLocalRuntimeEnabled: browserLocalPreviewRuntimeEnabled,
    browserLocalRuntimeAvailable: browserLocalPreviewRuntimeAvailable,
  })

  return (
    <div className="grid grid-cols-1 gap-2 border-b border-[rgba(138,223,255,0.08)] bg-[rgba(3,7,14,0.72)] px-3 py-3 md:grid-cols-2 xl:grid-cols-4">
      <div className="rounded-[20px] border border-[rgba(138,223,255,0.12)] bg-[rgba(7,13,24,0.74)] px-3 py-3">
        <div className="text-[10px] uppercase tracking-[0.18em] text-[#68809b]">Preview Runtime</div>
        <div className="mt-2 text-sm font-semibold text-white">{runtimeSummary}</div>
        <div className="mt-1 text-xs text-[#8fa4bc]">
          {activeSandbox ? 'Docker sandbox' : sandboxRequired && sandboxDegraded ? 'Process fallback mode' : 'Process mode'}
        </div>
      </div>
      <div className="rounded-[20px] border border-[rgba(138,223,255,0.12)] bg-[rgba(7,13,24,0.74)] px-3 py-3">
        <div className="text-[10px] uppercase tracking-[0.18em] text-[#68809b]">Backend API</div>
        <div className="mt-2 text-sm font-semibold text-white">
          {!serverDetection?.has_backend
            ? 'Not detected'
            : !backendPreviewAvailable
              ? 'Preview disabled'
              : serverStatus?.running
                ? `Running on :${serverStatus.port}`
                : 'Detected, stopped'}
        </div>
        <div className="mt-1 text-xs text-[#8fa4bc]">
          {serverDetection?.framework || serverDetection?.server_type || backendPreviewReason || 'No backend runtime'}
        </div>
      </div>
      <div className="rounded-[20px] border border-[rgba(138,223,255,0.12)] bg-[rgba(7,13,24,0.74)] px-3 py-3">
        <div className="text-[10px] uppercase tracking-[0.18em] text-[#68809b]">Environment</div>
        <div className="mt-2 text-sm font-semibold text-white">{bundlerAvailable ? 'Bundler ready' : 'Bundler unavailable'}</div>
        <div className="mt-1 text-xs text-[#8fa4bc]">
          Auto-refresh {autoRefreshEnabled ? 'on' : 'off'} · DevTools {showDevTools ? 'on' : 'off'}
        </div>
        <div className="mt-1 truncate text-xs text-[#68809b]" title={browserLocalDetail}>
          Browser-local {browserLocalPreviewCapability.label.toLowerCase()}: {browserLocalDetail}
        </div>
        <div className="mt-1 truncate text-xs text-[#68809b]" title={browserLocalRoute.reason}>
          Browser-local route: {browserLocalRoute.label}
        </div>
      </div>
      <div className="rounded-[20px] border border-[rgba(138,223,255,0.12)] bg-[rgba(7,13,24,0.74)] px-3 py-3">
        <div className="text-[10px] uppercase tracking-[0.18em] text-[#68809b]">Route</div>
        <div className="mt-2 truncate text-sm font-semibold text-white">{customPath.trim() || '/'}</div>
        <div className="mt-1 truncate text-xs text-[#8fa4bc]">
          {status?.started_at ? `Started ${new Date(status.started_at).toLocaleTimeString()}` : 'Not started yet'}
        </div>
      </div>
    </div>
  )
}
