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

interface PreviewToolbarProps {
  loading: boolean
  connected: boolean
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

export default function PreviewToolbar({
  loading,
  connected,
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

  return (
    <div className="flex items-center justify-between px-3 py-2 bg-gray-800/50 border-b border-gray-700">
      <div className="flex items-center gap-2">
        <div
          className={`flex items-center gap-1.5 px-2 py-1 rounded-md text-xs ${
            loading
              ? 'bg-yellow-500/20 text-yellow-400'
              : connected && status?.active && (!serverDetection?.has_backend || serverStatus?.running)
                ? 'bg-green-500/20 text-green-400'
                : connected && status?.active && serverDetection?.has_backend && !serverStatus?.running
                  ? 'bg-orange-500/20 text-orange-400'
                  : error || iframeError
                    ? 'bg-red-500/20 text-red-400'
                    : 'bg-gray-700 text-gray-400'
          }`}
          title={
            loading
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
          {loading ? (
            <>
              <Loader2 className="w-3 h-3 animate-spin" />
              <span>Starting</span>
            </>
          ) : connected && status?.active && (!serverDetection?.has_backend || serverStatus?.running) ? (
            <>
              <Wifi className="w-3 h-3" />
              <span>Running</span>
            </>
          ) : connected && status?.active && serverDetection?.has_backend && !serverStatus?.running ? (
            <>
              <AlertCircle className="w-3 h-3" />
              <span>Backend Down</span>
            </>
          ) : error || iframeError ? (
            <>
              <ShieldOff className="w-3 h-3" />
              <span>Failed</span>
            </>
          ) : (
            <>
              <WifiOff className="w-3 h-3" />
              <span>Not Running</span>
            </>
          )}
        </div>

        {status?.active ? (
          <button
            onClick={onStopPreview}
            disabled={loading}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-red-600/20 hover:bg-red-600/30 text-red-400 rounded-md text-sm transition-colors disabled:opacity-50"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Square className="w-4 h-4" />}
            Stop
          </button>
        ) : (
          <button
            onClick={onStartPreview}
            disabled={loading}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-green-600/20 hover:bg-green-600/30 text-green-400 rounded-md text-sm transition-colors disabled:opacity-50"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
            Start Preview
          </button>
        )}

        {status?.active && (
          <button
            onClick={onRestartPreview}
            disabled={loading}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-cyan-600/20 hover:bg-cyan-600/30 text-cyan-300 rounded-md text-sm transition-colors disabled:opacity-50"
            title="Restart preview runtime"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            Restart
          </button>
        )}

        <button
          onClick={onRefreshPreview}
          disabled={!status?.active || loading}
          className="p-1.5 hover:bg-gray-700 rounded-md text-gray-400 hover:text-white transition-colors disabled:opacity-50"
          title="Reload frame"
        >
          <RefreshCw className="w-4 h-4" />
        </button>

        {serverDetection?.has_backend && (
          <div className="flex items-center gap-1.5 ml-2 pl-2 border-l border-gray-600">
            <div
              className={`flex items-center gap-1.5 px-2 py-1 rounded-md text-xs ${
                !backendPreviewAvailable
                  ? 'bg-gray-700 text-gray-400'
                  : serverStatus?.running
                    ? 'bg-purple-500/20 text-purple-400'
                    : 'bg-gray-700 text-gray-400'
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
                className="flex items-center gap-1 px-2 py-1 bg-red-600/20 hover:bg-red-600/30 text-red-400 rounded-md text-xs transition-colors disabled:opacity-50"
                title="Stop Backend Server"
              >
                {serverLoading ? <Loader2 className="w-3 h-3 animate-spin" /> : <PowerOff className="w-3 h-3" />}
                Stop
              </button>
            ) : (
              <button
                onClick={onStartServer}
                disabled={serverLoading || !backendPreviewAvailable}
                className="flex items-center gap-1 px-2 py-1 bg-purple-600/20 hover:bg-purple-600/30 text-purple-400 rounded-md text-xs transition-colors disabled:opacity-50"
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
                className={`flex items-center gap-1 px-2 py-1 rounded-md text-xs transition-colors ${
                  showServerLogs ? 'bg-purple-600 text-white' : 'hover:bg-gray-700 text-gray-400 hover:text-white'
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
          <div className="ml-2 text-[11px] text-gray-500">{backendPreviewReason}</div>
        )}

        <div className="flex items-center bg-gray-800 rounded-md p-0.5 ml-2">
          <button
            onClick={() => onActiveTabChange('preview')}
            className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
              activeTab === 'preview' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'
            }`}
          >
            Preview
          </button>
          {showDevTools && (
            <>
              <button
                onClick={() => onActiveTabChange('console')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded text-xs font-medium transition-colors ${
                  activeTab === 'console' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'
                }`}
              >
                <Terminal className="w-3 h-3" />
                Console
                {(errorCount > 0 || warnCount > 0) && (
                  <span
                    className={`px-1.5 py-0.5 rounded-full text-[10px] ${
                      errorCount > 0 ? 'bg-red-500 text-white' : 'bg-yellow-500 text-black'
                    }`}
                  >
                    {errorCount > 0 ? errorCount : warnCount}
                  </span>
                )}
              </button>
              <button
                onClick={() => onActiveTabChange('network')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded text-xs font-medium transition-colors ${
                  activeTab === 'network' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'
                }`}
              >
                <Globe className="w-3 h-3" />
                Network
                {networkErrorCount > 0 && (
                  <span className="px-1.5 py-0.5 rounded-full text-[10px] bg-red-500 text-white">{networkErrorCount}</span>
                )}
              </button>
            </>
          )}
        </div>
      </div>

      <div className="flex items-center gap-1">
        <div className="flex items-center bg-gray-800 rounded-md p-0.5 mr-2">
          <button
            onClick={() => onViewportChange('mobile')}
            className={`p-1.5 rounded ${viewport === 'mobile' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'}`}
            title={viewportLabels.mobile}
          >
            <Smartphone className="w-4 h-4" />
          </button>
          <button
            onClick={() => onViewportChange('tablet')}
            className={`p-1.5 rounded ${viewport === 'tablet' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'}`}
            title={viewportLabels.tablet}
          >
            <Tablet className="w-4 h-4" />
          </button>
          <button
            onClick={() => onViewportChange('desktop')}
            className={`p-1.5 rounded ${viewport === 'desktop' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'}`}
            title={viewportLabels.desktop}
          >
            <Monitor className="w-4 h-4" />
          </button>
          <button
            onClick={() => onViewportChange('full')}
            className={`p-1.5 rounded ${viewport === 'full' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'}`}
            title={viewportLabels.full}
          >
            <Maximize2 className="w-4 h-4" />
          </button>
        </div>

        <button
          onClick={onToggleFullscreen}
          className="p-1.5 hover:bg-gray-700 rounded-md text-gray-400 hover:text-white transition-colors"
          title={isFullscreen ? 'Exit Fullscreen' : 'Fullscreen'}
        >
          {isFullscreen ? <Minimize2 className="w-4 h-4" /> : <Maximize2 className="w-4 h-4" />}
        </button>

        <button
          onClick={onOpenInNewTab}
          disabled={!status?.active || !hasPreviewSource}
          className="p-1.5 hover:bg-gray-700 rounded-md text-gray-400 hover:text-white transition-colors disabled:opacity-50"
          title="Open in New Tab"
        >
          <ExternalLink className="w-4 h-4" />
        </button>

        <div className="relative">
          <button
            onClick={() => setShowSettings(prev => !prev)}
            className="p-1.5 hover:bg-gray-700 rounded-md text-gray-400 hover:text-white transition-colors"
            title="Settings"
          >
            <Settings className="w-4 h-4" />
          </button>

          {showSettings && (
            <>
              <div className="fixed inset-0 z-40" onClick={() => setShowSettings(false)} />
              <div className="absolute right-0 top-full mt-1 w-64 bg-gray-800/95 backdrop-blur-md border border-gray-700/80 rounded-xl shadow-2xl shadow-black/60 z-50">
                <div className="p-3">
                  <h4 className="text-sm font-medium text-white mb-3">Preview Settings</h4>
                  <div className="space-y-3">
                    <label className="flex items-center justify-between">
                      <span className="text-sm text-gray-400">Auto-refresh on save</span>
                      <input
                        type="checkbox"
                        checked={autoRefreshEnabled}
                        onChange={event => onAutoRefreshChange(event.target.checked)}
                        className="rounded bg-gray-700 border-gray-600 text-cyan-500"
                      />
                    </label>

                    <label className="flex items-center justify-between">
                      <span className="text-sm text-gray-400">Show dev tools</span>
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
                        <span className="text-sm text-gray-400">Docker Sandbox</span>
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
                      <p className="text-[10px] text-gray-600 -mt-1">Docker not available on server</p>
                    ) : null}
                    {sandboxRequired && !dockerAvailable && (
                      <p className="text-[10px] text-amber-400 -mt-1">
                        Secure preview is required, but Docker is currently unavailable
                      </p>
                    )}

                    <div className="flex items-center justify-between">
                      <span className="text-sm text-gray-400">esbuild Bundler</span>
                      <span
                        className={`text-xs px-1.5 py-0.5 rounded ${
                          bundlerAvailable ? 'bg-green-500/20 text-green-400' : 'bg-gray-700 text-gray-500'
                        }`}
                      >
                        {bundlerAvailable ? 'Available' : 'Not Found'}
                      </span>
                    </div>

                    <div>
                      <span className="text-sm text-gray-400">Custom URL</span>
                      <input
                        type="text"
                        placeholder="/custom-path"
                        value={customPath}
                        onChange={event => onCustomPathChange(event.target.value)}
                        className="w-full mt-1 px-2 py-1 bg-gray-900 border border-gray-700 rounded text-sm text-white"
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
