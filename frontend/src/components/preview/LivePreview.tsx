import React, { useEffect, useMemo, useRef, useState } from 'react'
import { AlertCircle, CheckCircle, RefreshCw, Server, Square } from 'lucide-react'
import ConsolePanel from './ConsolePanel'
import NetworkPanel from './NetworkPanel'
import PreviewRuntimePane from './PreviewRuntimePane'
import PreviewStatusCards from './PreviewStatusCards'
import PreviewToolbar from './PreviewToolbar'
import { derivePreviewRuntimeState, detectBrowserLocalPreviewCapability } from './previewState'
import type { ActiveTab, ViewportSize } from './types'
import { usePreviewDevtools } from '@/hooks/usePreviewDevtools'
import { usePreviewRuntime } from '@/hooks/usePreviewRuntime'
import { usePreviewServer } from '@/hooks/usePreviewServer'

interface LivePreviewProps {
  projectId: number
  onFileChange?: (filePath: string, content: string) => void
  className?: string
  autoStart?: boolean
  autoRefreshOnSave?: boolean
  onAutoRefreshChange?: (enabled: boolean) => void
}

const viewportSizes: Record<ViewportSize, { width: number; height: number }> = {
  mobile: { width: 375, height: 667 },
  tablet: { width: 768, height: 1024 },
  desktop: { width: 1280, height: 800 },
  full: { width: 0, height: 0 },
}

export default function LivePreview({
  projectId,
  onFileChange: _onFileChange,
  className = '',
  autoStart = false,
  autoRefreshOnSave,
  onAutoRefreshChange,
}: LivePreviewProps) {
  const [viewport, setViewport] = useState<ViewportSize>('full')
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [activeTab, setActiveTab] = useState<ActiveTab>('preview')
  const [showDevTools, setShowDevTools] = useState(true)
  const [customPath, setCustomPath] = useState('')
  const [internalAutoRefresh, setInternalAutoRefresh] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const iframeRef = useRef<HTMLIFrameElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  const autoRefreshEnabled = typeof autoRefreshOnSave === 'boolean' ? autoRefreshOnSave : internalAutoRefresh
  const setAutoRefreshEnabled = (value: boolean) => {
    if (typeof autoRefreshOnSave === 'boolean') {
      onAutoRefreshChange?.(value)
      return
    }
    setInternalAutoRefresh(value)
    onAutoRefreshChange?.(value)
  }

  const {
    consoleEntries,
    setConsoleEntries,
    networkRequests,
    setNetworkRequests,
    clearDevTools,
    errorCount,
    warnCount,
    networkErrorCount,
  } = usePreviewDevtools()

  const {
    serverStatus,
    setServerStatus,
    serverDetection,
    serverLoading,
    serverLogs,
    showServerLogs,
    setShowServerLogs,
    startServer,
    stopServer,
    fetchServerLogs,
    toggleServerLogs,
  } = usePreviewServer({ projectId, setError })

  const runtime = usePreviewRuntime({
    projectId,
    autoStart,
    clearDevTools,
    setError,
    serverDetection,
    onServerStatusHint: setServerStatus,
  })
  const runtimeStatusActive = runtime.status?.active
  const setRuntimeIframeLoading = runtime.setIframeLoading
  const setRuntimeIframeError = runtime.setIframeError

  useEffect(() => {
    if (!showDevTools && activeTab !== 'preview') {
      setActiveTab('preview')
    }
  }, [activeTab, showDevTools])

  const previewSrc = useMemo(() => {
    if (!runtime.previewUrl) return ''

    const trimmed = customPath.trim()
    if (!trimmed) return runtime.previewUrl
    if (/^https?:\/\//i.test(trimmed)) return trimmed

    const normalized = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
    try {
      const url = new URL(runtime.previewUrl)
      url.pathname = url.pathname.replace(/\/$/, '') + normalized
      return url.toString()
    } catch {
      return runtime.previewUrl
    }
  }, [customPath, runtime.previewUrl])

  const displayUrl = useMemo(() => {
    if (!previewSrc) return ''
    try {
      const url = new URL(previewSrc)
      url.searchParams.delete('token')
      url.searchParams.delete('preview_token')
      return url.toString()
    } catch {
      return previewSrc.replace(/([?&]token=)[^&]+/, '$1•••').replace(/([?&]preview_token=)[^&]+/, '$1•••')
    }
  }, [previewSrc])

  const runtimeState = useMemo(
    () =>
      derivePreviewRuntimeState({
        loading: runtime.loading,
        status: runtime.status,
        connected: runtime.connected,
        error,
        iframeError: runtime.iframeError,
        sandboxDegraded: runtime.sandboxDegraded,
        serverDetection,
        serverStatus,
        backendPreviewAvailable: runtime.backendPreviewAvailable,
      }),
    [
      error,
      runtime.backendPreviewAvailable,
      runtime.connected,
      runtime.iframeError,
      runtime.loading,
      runtime.sandboxDegraded,
      runtime.status,
      serverDetection,
      serverStatus,
    ],
  )
  const browserLocalPreviewCapability = useMemo(() => detectBrowserLocalPreviewCapability(), [])
  const browserLocalPreviewRuntimeEnabled = import.meta.env.VITE_ENABLE_BROWSER_LOCAL_PREVIEW === 'true'
    || import.meta.env.VITE_ENABLE_WEBCONTAINER_PREVIEW === 'true'

  useEffect(() => {
    if (runtimeStatusActive && previewSrc) {
      setRuntimeIframeLoading(true)
      setRuntimeIframeError(null)
      return
    }
    if (!runtimeStatusActive) {
      setRuntimeIframeLoading(false)
      setRuntimeIframeError(null)
    }
  }, [previewSrc, runtimeStatusActive, setRuntimeIframeError, setRuntimeIframeLoading])

  const viewportStyle = useMemo(() => {
    if (viewport === 'full') {
      return { width: '100%', height: '100%' }
    }
    const size = viewportSizes[viewport]
    return {
      width: `${size.width}px`,
      height: `${size.height}px`,
      maxWidth: '100%',
      maxHeight: '100%',
    }
  }, [viewport])

  const toggleFullscreen = () => {
    if (!containerRef.current) return
    if (!document.fullscreenElement) {
      void containerRef.current.requestFullscreen()
    } else {
      void document.exitFullscreen()
    }
  }

  useEffect(() => {
    const handleFullscreenChange = () => {
      setIsFullscreen(Boolean(document.fullscreenElement))
    }
    document.addEventListener('fullscreenchange', handleFullscreenChange)
    return () => document.removeEventListener('fullscreenchange', handleFullscreenChange)
  }, [])

  const openInNewTab = () => {
    if (!previewSrc) return
    window.open(previewSrc, '_blank')
  }

  return (
    <div
      ref={containerRef}
      className={`min-h-0 flex flex-col bg-gray-900 border border-gray-700 rounded-lg overflow-hidden ${className}`}
    >
      {runtime.sandboxDegraded && (
        <div className="flex items-center gap-2 px-3 py-2 bg-amber-500/10 border-b border-amber-500/20 text-amber-200 text-xs">
          <AlertCircle className="w-3.5 h-3.5 text-amber-300" />
          <span>Platform Docker is unavailable. Preview is using process fallback mode.</span>
        </div>
      )}

      <PreviewToolbar
        loading={runtime.loading}
        runtimeState={runtimeState}
        status={runtime.status}
        error={error}
        iframeError={runtime.iframeError}
        onStartPreview={() => {
          void runtime.startPreview()
        }}
        onStopPreview={() => {
          void runtime.stopPreview()
        }}
        onRestartPreview={() => {
          void runtime.restartPreview()
        }}
        onRefreshPreview={() => {
          void runtime.refreshPreview()
        }}
        serverDetection={serverDetection}
        serverStatus={serverStatus}
        backendPreviewAvailable={runtime.backendPreviewAvailable}
        backendPreviewReason={runtime.backendPreviewReason}
        serverLoading={serverLoading}
        showServerLogs={showServerLogs}
        onStartServer={() => {
          void startServer()
        }}
        onStopServer={() => {
          void stopServer()
        }}
        onToggleServerLogs={() => {
          void toggleServerLogs()
        }}
        activeTab={activeTab}
        onActiveTabChange={setActiveTab}
        showDevTools={showDevTools}
        errorCount={errorCount}
        warnCount={warnCount}
        networkErrorCount={networkErrorCount}
        viewport={viewport}
        onViewportChange={setViewport}
        isFullscreen={isFullscreen}
        onToggleFullscreen={toggleFullscreen}
        onOpenInNewTab={openInNewTab}
        hasPreviewSource={Boolean(previewSrc)}
        autoRefreshEnabled={autoRefreshEnabled}
        onAutoRefreshChange={setAutoRefreshEnabled}
        dockerAvailable={runtime.dockerAvailable}
        useSandbox={runtime.useSandbox}
        onUseSandboxChange={runtime.setUseSandbox}
        sandboxRequired={runtime.sandboxRequired}
        sandboxDegraded={runtime.sandboxDegraded}
        bundlerAvailable={runtime.bundlerAvailable}
        customPath={customPath}
        onCustomPathChange={setCustomPath}
        onShowDevToolsChange={setShowDevTools}
      />

      {runtime.status?.active && activeTab === 'preview' && (
        <div className="flex items-center gap-2 px-3 py-1.5 bg-gray-900/50 border-b border-gray-700/60">
          <div className="flex-1 flex items-center gap-2 bg-gray-800/60 rounded-lg px-3 py-1.5 border border-gray-700/40">
            {runtime.connected ? (
              <CheckCircle className="w-3 h-3 text-green-400 shrink-0" />
            ) : (
              <AlertCircle className="w-3 h-3 text-yellow-400 shrink-0" />
            )}
            <input
              type="text"
              value={displayUrl}
              readOnly
              aria-label="Preview URL"
              className="flex-1 bg-transparent text-xs text-gray-400 outline-none font-mono truncate cursor-default select-all"
            />
          </div>
        </div>
      )}

      {activeTab === 'preview' && (
        <PreviewStatusCards
          status={runtime.status}
          runtimeState={runtimeState}
          activeSandbox={runtime.activeSandbox}
          sandboxRequired={runtime.sandboxRequired}
          sandboxDegraded={runtime.sandboxDegraded}
          serverDetection={serverDetection}
          serverStatus={serverStatus}
          backendPreviewAvailable={runtime.backendPreviewAvailable}
          backendPreviewReason={runtime.backendPreviewReason}
          bundlerAvailable={runtime.bundlerAvailable}
          browserLocalPreviewCapability={browserLocalPreviewCapability}
          browserLocalPreviewRuntimeEnabled={browserLocalPreviewRuntimeEnabled}
          autoRefreshEnabled={autoRefreshEnabled}
          showDevTools={showDevTools}
          customPath={customPath}
        />
      )}

      <div className="flex-1 overflow-hidden">
        {activeTab === 'preview' && (
          <PreviewRuntimePane
            status={runtime.status}
            runtimeState={runtimeState}
            previewSrc={previewSrc}
            viewportStyle={viewportStyle}
            refreshKey={runtime.refreshKey}
            iframeRef={iframeRef}
            loading={runtime.loading}
            error={error}
            iframeLoading={runtime.iframeLoading}
            iframeError={runtime.iframeError}
            onStartPreview={() => {
              void runtime.startPreview()
            }}
            onDismissError={() => setError(null)}
            onIframeLoad={() => {
              runtime.setIframeLoading(false)
              runtime.setIframeError(null)
            }}
            onIframeError={() => {
              runtime.setIframeLoading(false)
              runtime.setIframeError('Preview frame failed to load. Try Refresh or restart preview.')
            }}
          />
        )}

        {showDevTools && activeTab === 'console' && (
          <ConsolePanel
            entries={consoleEntries}
            onClear={() => setConsoleEntries([])}
            className="h-full"
          />
        )}

        {showDevTools && activeTab === 'network' && (
          <NetworkPanel
            requests={networkRequests}
            onClear={() => setNetworkRequests([])}
            className="h-full"
          />
        )}
      </div>

      {showServerLogs && serverDetection?.has_backend && (
        <div className="border-t border-gray-700 bg-gray-900 max-h-64 overflow-hidden flex flex-col">
          <div className="flex items-center justify-between px-3 py-2 bg-gray-800/50 border-b border-gray-700">
            <div className="flex items-center gap-2">
              <Server className="w-4 h-4 text-purple-400" />
              <span className="text-sm font-medium text-white">Backend Server Logs</span>
              <span className="text-xs text-gray-500">
                {serverDetection.framework || serverDetection.server_type || 'runtime'} | Port {serverStatus?.port ?? 'n/a'}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => {
                  void fetchServerLogs()
                }}
                className="p-1 hover:bg-gray-700 rounded text-gray-400 hover:text-white"
                title="Refresh Logs"
              >
                <RefreshCw className="w-3.5 h-3.5" />
              </button>
              <button
                onClick={() => setShowServerLogs(false)}
                className="p-1 hover:bg-gray-700 rounded text-gray-400 hover:text-white"
                title="Close"
              >
                <Square className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
          <div className="flex-1 overflow-auto p-2 font-mono text-xs">
            {serverLogs.stderr && (
              <div className="mb-2">
                <div className="text-red-400 font-semibold mb-1">stderr:</div>
                <pre className="text-red-300 whitespace-pre-wrap">{serverLogs.stderr}</pre>
              </div>
            )}
            {serverLogs.stdout && (
              <div>
                <div className="text-green-400 font-semibold mb-1">stdout:</div>
                <pre className="text-gray-300 whitespace-pre-wrap">{serverLogs.stdout}</pre>
              </div>
            )}
            {!serverLogs.stdout && !serverLogs.stderr && (
              <div className="text-gray-500 text-center py-4">No logs yet...</div>
            )}
          </div>
        </div>
      )}

      {runtime.status?.active && (
        <div className="flex items-center justify-between px-3 py-1.5 bg-gray-800/30 border-t border-gray-700 text-xs text-gray-500">
          <div className="flex items-center gap-4">
            <span>Port: {runtime.status.port}</span>
            <span>Clients: {runtime.status.connected_clients}</span>
            <span className="text-gray-600">|</span>
            <span>Console: {consoleEntries.length}</span>
            <span>Network: {networkRequests.length}</span>
            {serverStatus?.running && (
              <>
                <span className="text-gray-600">|</span>
                <span className="flex items-center gap-1 text-purple-400">
                  <Server className="w-3 h-3" />
                  API: {serverStatus.port}
                  {serverStatus.uptime_seconds && serverStatus.uptime_seconds > 0 && (
                    <span className="text-gray-500">
                      ({Math.floor(serverStatus.uptime_seconds / 60)}m {serverStatus.uptime_seconds % 60}s)
                    </span>
                  )}
                </span>
              </>
            )}
          </div>
          <div className="flex items-center gap-2">
            <span className="flex items-center gap-1">
              <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
              DevTools Active
            </span>
            {serverStatus?.running && (
              <span className="flex items-center gap-1">
                <div className="w-2 h-2 rounded-full bg-purple-500 animate-pulse" />
                API Running
              </span>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
