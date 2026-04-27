import { useState } from 'react'
import {
  AlertCircle,
  ExternalLink,
  FileText,
  Globe,
  Loader2,
  Maximize2,
  Minimize2,
  Monitor,
  Play,
  Power,
  PowerOff,
  RefreshCw,
  Server,
  Settings,
  Shield,
  ShieldOff,
  Smartphone,
  Square,
  Tablet,
  Terminal,
  Wifi,
  WifiOff,
} from 'lucide-react'
import type { ActiveTab, PreviewStatus, ServerDetection, ServerStatus, ViewportSize } from './types'
import { previewRuntimeStateLabels, type PreviewRuntimeState } from './previewState'

interface PreviewToolbarProps {
  loading: boolean
  runtimeState: PreviewRuntimeState
  status: PreviewStatus | null
  error: string | null
  iframeError: string | null
  onStartPreview: () => void
  onStopPreview: () => void
  onRestartPreview: () => void
  onRefreshPreview: () => void
  serverDetection: ServerDetection | null
  serverStatus: ServerStatus | null
  backendPreviewAvailable: boolean
  backendPreviewReason: string
  serverLoading: boolean
  showServerLogs: boolean
  onStartServer: () => void
  onStopServer: () => void
  onToggleServerLogs: () => void
  activeTab: ActiveTab
  onActiveTabChange: (value: ActiveTab) => void
  showDevTools: boolean
  errorCount: number
  warnCount: number
  networkErrorCount: number
  viewport: ViewportSize
  onViewportChange: (value: ViewportSize) => void
  isFullscreen: boolean
  onToggleFullscreen: () => void
  onOpenInNewTab: () => void
  hasPreviewSource: boolean
  autoRefreshEnabled: boolean
  onAutoRefreshChange: (value: boolean) => void
  dockerAvailable: boolean
  useSandbox: boolean
  onUseSandboxChange: (value: boolean) => void
  sandboxRequired: boolean
  sandboxDegraded: boolean
  bundlerAvailable: boolean
  customPath: string
  onCustomPathChange: (value: string) => void
  onShowDevToolsChange: (value: boolean) => void
}

const viewportLabels: Record<ViewportSize, string> = {
  mobile: 'Mobile (375x667)',
  tablet: 'Tablet (768x1024)',
  desktop: 'Desktop (1280x800)',
  full: 'Full Width',
}

const runtimeStateClasses: Record<PreviewRuntimeState, string> = {
  starting: 'border border-[rgba(255,214,102,0.25)] bg-[rgba(255,214,102,0.14)] text-[#ffd76b]',
  running: 'border border-[rgba(112,248,184,0.25)] bg-[rgba(112,248,184,0.14)] text-[#70f8b8]',
  degraded: 'border border-[rgba(255,184,107,0.24)] bg-[rgba(255,184,107,0.12)] text-[#ffcf91]',
  backend_down: 'border border-[rgba(255,184,107,0.24)] bg-[rgba(255,184,107,0.12)] text-[#ffcf91]',
  failed: 'border border-[rgba(255,118,118,0.24)] bg-[rgba(255,118,118,0.12)] text-[#ff9494]',
  stopped: 'border border-[#17314d] bg-[rgba(7,13,24,0.82)] text-[#7f95ad]',
}

export default function PreviewToolbar({
  loading,
  runtimeState,
  status,
  error,
  iframeError,
  onStartPreview,
  onStopPreview,
  onRestartPreview,
  onRefreshPreview,
  serverDetection,
  serverStatus,
  backendPreviewAvailable,
  backendPreviewReason,
  serverLoading,
  showServerLogs,
  onStartServer,
  onStopServer,
  onToggleServerLogs,
  activeTab,
  onActiveTabChange,
  showDevTools,
  errorCount,
  warnCount,
  networkErrorCount,
  viewport,
  onViewportChange,
  isFullscreen,
  onToggleFullscreen,
  onOpenInNewTab,
  hasPreviewSource,
  autoRefreshEnabled,
  onAutoRefreshChange,
  dockerAvailable,
  useSandbox,
  onUseSandboxChange,
  sandboxRequired,
  sandboxDegraded,
  bundlerAvailable,
  customPath,
  onCustomPathChange,
  onShowDevToolsChange,
}: PreviewToolbarProps) {
  const [showSettings, setShowSettings] = useState(false)
  const runtimeLabel = previewRuntimeStateLabels[runtimeState]

  return (
    <div className="flex items-center justify-between border-b border-[rgba(138,223,255,0.08)] bg-[rgba(6,12,22,0.9)] px-3 py-2.5">
      <div className="flex items-center gap-2">
        <div
          className={`flex items-center gap-1.5 rounded-xl px-2.5 py-1.5 text-xs ${runtimeStateClasses[runtimeState]}`}
          title={
            runtimeState === 'starting'
              ? 'Starting preview...'
              : error
                ? `Error: ${error}`
                : serverStatus?.last_error
                  ? `Backend error: ${serverStatus.last_error}`
                  : serverStatus?.exit_code !== undefined && serverStatus.exit_code !== 0
                    ? `Backend exited with code ${serverStatus.exit_code}`
                    : undefined
          }
        >
          {runtimeState === 'starting' ? (
            <>
              <Loader2 className="w-3 h-3 animate-spin" />
              <span>{runtimeLabel}</span>
            </>
          ) : runtimeState === 'running' ? (
            <>
              <Wifi className="w-3 h-3" />
              <span>{runtimeLabel}</span>
            </>
          ) : runtimeState === 'degraded' || runtimeState === 'backend_down' ? (
            <>
              <AlertCircle className="w-3 h-3" />
              <span>{runtimeLabel}</span>
            </>
          ) : runtimeState === 'failed' ? (
            <>
              <ShieldOff className="w-3 h-3" />
              <span>{runtimeLabel}</span>
            </>
          ) : (
            <>
              <WifiOff className="w-3 h-3" />
              <span>{runtimeLabel}</span>
            </>
          )}
        </div>

        {status?.active ? (
          <button
            onClick={onStopPreview}
            disabled={loading}
            className="flex items-center gap-1.5 rounded-xl border border-[rgba(255,118,118,0.24)] bg-[rgba(255,118,118,0.12)] px-3 py-1.5 text-sm text-[#ff9494] transition-colors hover:bg-[rgba(255,118,118,0.18)] disabled:opacity-50"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Square className="w-4 h-4" />}
            Stop
          </button>
        ) : (
          <button
            onClick={onStartPreview}
            disabled={loading}
            className="flex items-center gap-1.5 rounded-xl border border-[rgba(112,248,184,0.25)] bg-[rgba(112,248,184,0.14)] px-3 py-1.5 text-sm text-[#70f8b8] transition-colors hover:bg-[rgba(112,248,184,0.2)] disabled:opacity-50"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
            Start Preview
          </button>
        )}

        {status?.active && (
          <button
            onClick={onRestartPreview}
            disabled={loading}
            className="flex items-center gap-1.5 rounded-xl border border-[rgba(138,223,255,0.26)] bg-[rgba(47,168,255,0.14)] px-3 py-1.5 text-sm text-[#bdeeff] transition-colors hover:bg-[rgba(47,168,255,0.2)] disabled:opacity-50"
            title="Restart preview runtime"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            Restart
          </button>
        )}

        <button
          onClick={onRefreshPreview}
          disabled={!status?.active || loading}
          className="rounded-lg p-1.5 text-[#8499af] transition-colors hover:bg-[rgba(11,20,35,0.92)] hover:text-white disabled:opacity-50"
          title="Reload frame"
        >
          <RefreshCw className="w-4 h-4" />
        </button>

        {serverDetection?.has_backend && (
          <div className="ml-2 flex items-center gap-1.5 border-l border-[rgba(138,223,255,0.08)] pl-2">
            <div
              className={`flex items-center gap-1.5 rounded-xl px-2.5 py-1.5 text-xs ${
                !backendPreviewAvailable
                  ? 'border border-[#17314d] bg-[rgba(7,13,24,0.82)] text-[#7f95ad]'
                  : serverStatus?.running
                    ? 'border border-[rgba(138,223,255,0.24)] bg-[rgba(47,168,255,0.14)] text-[#bdeeff]'
                    : 'border border-[#17314d] bg-[rgba(7,13,24,0.82)] text-[#7f95ad]'
              }`}
              title={!backendPreviewAvailable && backendPreviewReason ? backendPreviewReason : undefined}
            >
              <Server className="w-3 h-3" />
              <span>
                {!backendPreviewAvailable ? 'API Disabled' : serverStatus?.running ? `API :${serverStatus.port}` : 'API Off'}
              </span>
            </div>

            {serverStatus?.running ? (
              <button
                onClick={onStopServer}
                disabled={serverLoading}
                className="flex items-center gap-1 rounded-xl border border-[rgba(255,118,118,0.24)] bg-[rgba(255,118,118,0.12)] px-2 py-1 text-xs text-[#ff9494] transition-colors hover:bg-[rgba(255,118,118,0.18)] disabled:opacity-50"
                title="Stop Backend Server"
              >
                {serverLoading ? <Loader2 className="w-3 h-3 animate-spin" /> : <PowerOff className="w-3 h-3" />}
                Stop
              </button>
            ) : (
              <button
                onClick={onStartServer}
                disabled={serverLoading || !backendPreviewAvailable}
                className="flex items-center gap-1 rounded-xl border border-[rgba(138,223,255,0.24)] bg-[rgba(47,168,255,0.14)] px-2 py-1 text-xs text-[#bdeeff] transition-colors hover:bg-[rgba(47,168,255,0.2)] disabled:opacity-50"
                title={
                  backendPreviewAvailable
                    ? `Start ${serverDetection.framework || serverDetection.server_type} Server`
                    : backendPreviewReason || 'Backend preview is unavailable'
                }
              >
                {serverLoading ? <Loader2 className="w-3 h-3 animate-spin" /> : <Power className="w-3 h-3" />}
                Start API
              </button>
            )}

            {(serverStatus?.running ||
              serverDetection?.entry_file ||
              serverStatus?.last_error ||
              serverStatus?.exit_code !== undefined) && (
              <button
                onClick={onToggleServerLogs}
                className={`flex items-center gap-1 rounded-xl px-2 py-1 text-xs transition-colors ${
                  showServerLogs ? 'bg-[rgba(47,168,255,0.18)] text-white' : 'text-[#7f95ad] hover:bg-[rgba(11,20,35,0.92)] hover:text-white'
                }`}
                title="Server Logs"
              >
                <FileText className="w-3.5 h-3.5" />
                <span>{showServerLogs ? 'Hide Logs' : 'Logs'}</span>
              </button>
            )}
          </div>
        )}

        {serverDetection?.has_backend && !backendPreviewAvailable && backendPreviewReason && (
          <div className="ml-2 text-[11px] text-[#6a8096]">{backendPreviewReason}</div>
        )}

        <div className="ml-2 flex items-center rounded-xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] p-0.5">
          <button
            onClick={() => onActiveTabChange('preview')}
            className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
              activeTab === 'preview' ? 'bg-[linear-gradient(135deg,rgba(138,223,255,0.18),rgba(47,168,255,0.24))] text-white' : 'text-[#7f95ad] hover:text-white'
            }`}
          >
            Preview
          </button>
          {showDevTools && (
            <>
              <button
                onClick={() => onActiveTabChange('console')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded text-xs font-medium transition-colors ${
                  activeTab === 'console' ? 'bg-[linear-gradient(135deg,rgba(138,223,255,0.18),rgba(47,168,255,0.24))] text-white' : 'text-[#7f95ad] hover:text-white'
                }`}
              >
                <Terminal className="w-3 h-3" />
                Console
                {(errorCount > 0 || warnCount > 0) && (
                  <span
                    className={`px-1.5 py-0.5 rounded-full text-[10px] ${
                      errorCount > 0 ? 'bg-[rgba(255,118,118,0.92)] text-white' : 'bg-[rgba(255,214,102,0.92)] text-black'
                    }`}
                  >
                    {errorCount > 0 ? errorCount : warnCount}
                  </span>
                )}
              </button>
              <button
                onClick={() => onActiveTabChange('network')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded text-xs font-medium transition-colors ${
                  activeTab === 'network' ? 'bg-[linear-gradient(135deg,rgba(138,223,255,0.18),rgba(47,168,255,0.24))] text-white' : 'text-[#7f95ad] hover:text-white'
                }`}
              >
                <Globe className="w-3 h-3" />
                Network
                {networkErrorCount > 0 && (
                  <span className="rounded-full bg-[rgba(255,118,118,0.92)] px-1.5 py-0.5 text-[10px] text-white">{networkErrorCount}</span>
                )}
              </button>
            </>
          )}
        </div>
      </div>

      <div className="flex items-center gap-1">
        <div className="mr-2 flex items-center rounded-xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] p-0.5">
          <button
            onClick={() => onViewportChange('mobile')}
            className={`rounded-lg p-1.5 ${viewport === 'mobile' ? 'bg-[linear-gradient(135deg,rgba(138,223,255,0.18),rgba(47,168,255,0.24))] text-white' : 'text-[#7f95ad] hover:text-white'}`}
            title={viewportLabels.mobile}
          >
            <Smartphone className="w-4 h-4" />
          </button>
          <button
            onClick={() => onViewportChange('tablet')}
            className={`rounded-lg p-1.5 ${viewport === 'tablet' ? 'bg-[linear-gradient(135deg,rgba(138,223,255,0.18),rgba(47,168,255,0.24))] text-white' : 'text-[#7f95ad] hover:text-white'}`}
            title={viewportLabels.tablet}
          >
            <Tablet className="w-4 h-4" />
          </button>
          <button
            onClick={() => onViewportChange('desktop')}
            className={`rounded-lg p-1.5 ${viewport === 'desktop' ? 'bg-[linear-gradient(135deg,rgba(138,223,255,0.18),rgba(47,168,255,0.24))] text-white' : 'text-[#7f95ad] hover:text-white'}`}
            title={viewportLabels.desktop}
          >
            <Monitor className="w-4 h-4" />
          </button>
          <button
            onClick={() => onViewportChange('full')}
            className={`rounded-lg p-1.5 ${viewport === 'full' ? 'bg-[linear-gradient(135deg,rgba(138,223,255,0.18),rgba(47,168,255,0.24))] text-white' : 'text-[#7f95ad] hover:text-white'}`}
            title={viewportLabels.full}
          >
            <Maximize2 className="w-4 h-4" />
          </button>
        </div>

        <button
          onClick={onToggleFullscreen}
          className="rounded-lg p-1.5 text-[#8499af] transition-colors hover:bg-[rgba(11,20,35,0.92)] hover:text-white"
          title={isFullscreen ? 'Exit Fullscreen' : 'Fullscreen'}
        >
          {isFullscreen ? <Minimize2 className="w-4 h-4" /> : <Maximize2 className="w-4 h-4" />}
        </button>

        <button
          onClick={onOpenInNewTab}
          disabled={!status?.active || !hasPreviewSource}
          className="rounded-lg p-1.5 text-[#8499af] transition-colors hover:bg-[rgba(11,20,35,0.92)] hover:text-white disabled:opacity-50"
          title="Open in New Tab"
        >
          <ExternalLink className="w-4 h-4" />
        </button>

        <div className="relative">
          <button
            onClick={() => setShowSettings(prev => !prev)}
            className="rounded-lg p-1.5 text-[#8499af] transition-colors hover:bg-[rgba(11,20,35,0.92)] hover:text-white"
            title="Settings"
          >
            <Settings className="w-4 h-4" />
          </button>

          {showSettings && (
            <>
              <div className="fixed inset-0 z-40" onClick={() => setShowSettings(false)} />
              <div className="absolute right-0 top-full z-50 mt-1 w-64 rounded-2xl border border-[#17314d] bg-[rgba(4,9,18,0.96)] shadow-2xl shadow-black/60 backdrop-blur-xl">
                <div className="p-3">
                  <h4 className="text-sm font-medium text-white mb-3">Preview Settings</h4>
                  <div className="space-y-3">
                    <label className="flex items-center justify-between">
                      <span className="text-sm text-[#8fa4bc]">Auto-refresh on save</span>
                      <input
                        type="checkbox"
                        checked={autoRefreshEnabled}
                        onChange={event => onAutoRefreshChange(event.target.checked)}
                        className="rounded bg-gray-700 border-gray-600 text-cyan-500"
                      />
                    </label>

                    <label className="flex items-center justify-between">
                      <span className="text-sm text-[#8fa4bc]">Show dev tools</span>
                      <input
                        type="checkbox"
                        checked={showDevTools}
                        onChange={event => onShowDevToolsChange(event.target.checked)}
                        className="rounded bg-gray-700 border-gray-600 text-cyan-500"
                      />
                    </label>

                    <label className="flex items-center justify-between">
                      <div className="flex items-center gap-1.5">
                        {dockerAvailable ? (
                          <Shield className="w-3.5 h-3.5 text-green-400" />
                        ) : (
                          <ShieldOff className="w-3.5 h-3.5 text-gray-500" />
                        )}
                        <span className="text-sm text-[#8fa4bc]">Docker Sandbox</span>
                      </div>
                      <input
                        type="checkbox"
                        checked={useSandbox}
                        onChange={event => onUseSandboxChange(event.target.checked)}
                        disabled={!dockerAvailable || sandboxRequired}
                        className="rounded bg-gray-700 border-gray-600 text-cyan-500 disabled:opacity-40"
                      />
                    </label>
                    {sandboxRequired && sandboxDegraded ? (
                      <p className="text-[10px] text-amber-300 -mt-1">
                        Secure sandbox is unavailable, so preview will fall back to process mode
                      </p>
                    ) : sandboxRequired ? (
                      <p className="text-[10px] text-cyan-500 -mt-1">Secure preview is enforced by the server</p>
                    ) : !dockerAvailable ? (
                      <p className="text-[10px] text-[#6a8096] -mt-1">Docker not available on server</p>
                    ) : null}
                    {sandboxRequired && !dockerAvailable && (
                      <p className="text-[10px] text-amber-400 -mt-1">
                        Secure preview is required, but Docker is currently unavailable
                      </p>
                    )}

                    <div className="flex items-center justify-between">
                      <span className="text-sm text-[#8fa4bc]">esbuild Bundler</span>
                      <span
                        className={`text-xs px-1.5 py-0.5 rounded ${
                          bundlerAvailable ? 'bg-[rgba(112,248,184,0.14)] text-[#70f8b8]' : 'bg-[rgba(7,13,24,0.82)] text-[#6a8096]'
                        }`}
                      >
                        {bundlerAvailable ? 'Available' : 'Not Found'}
                      </span>
                    </div>

                    <div>
                      <span className="text-sm text-[#8fa4bc]">Custom URL</span>
                      <input
                        type="text"
                        placeholder="/custom-path"
                        value={customPath}
                        onChange={event => onCustomPathChange(event.target.value)}
                        className="mt-1 w-full rounded-xl border border-[#17314d] bg-[rgba(7,13,24,0.9)] px-2.5 py-2 text-sm text-white"
                      />
                    </div>
                  </div>
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
