import React, { useState, useEffect, useCallback, useRef } from 'react'
import {
  Play,
  Square,
  RefreshCw,
  ExternalLink,
  Smartphone,
  Tablet,
  Monitor,
  Maximize2,
  Minimize2,
  Loader2,
  AlertCircle,
  CheckCircle,
  Wifi,
  WifiOff,
  Settings,
  Terminal,
  Globe,
  Server,
  Power,
  PowerOff,
  FileText,
  Shield,
  ShieldOff
} from 'lucide-react'
import api from '../../services/api'
import ConsolePanel, { ConsoleEntry } from './ConsolePanel'
import NetworkPanel, { NetworkRequest } from './NetworkPanel'

interface PreviewStatus {
  project_id: number
  active: boolean
  port: number
  url: string
  started_at: string
  last_access: string
  connected_clients: number
}

interface ServerStatus {
  running: boolean
  port?: number
  pid?: number
  uptime_seconds?: number
  command?: string
  entry_file?: string
  url?: string
  ready?: boolean
}

interface ServerDetection {
  has_backend: boolean
  server_type?: string
  entry_file?: string
  command?: string
  framework?: string
}

interface LivePreviewProps {
  projectId: number
  onFileChange?: (filePath: string, content: string) => void
  className?: string
  autoStart?: boolean
}

type ViewportSize = 'mobile' | 'tablet' | 'desktop' | 'full'
type ActiveTab = 'preview' | 'console' | 'network'

const viewportSizes: Record<ViewportSize, { width: number; height: number; label: string }> = {
  mobile: { width: 375, height: 667, label: 'Mobile' },
  tablet: { width: 768, height: 1024, label: 'Tablet' },
  desktop: { width: 1280, height: 800, label: 'Desktop' },
  full: { width: 0, height: 0, label: 'Full' }
}

const MAX_CONSOLE_ENTRIES = 1000
const MAX_NETWORK_REQUESTS = 500

export default function LivePreview({ projectId, onFileChange, className = '', autoStart = false }: LivePreviewProps) {
  const [status, setStatus] = useState<PreviewStatus | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [viewport, setViewport] = useState<ViewportSize>('full')
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [showSettings, setShowSettings] = useState(false)
  const [previewUrl, setPreviewUrl] = useState<string>('')
  const [connected, setConnected] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)
  const [activeTab, setActiveTab] = useState<ActiveTab>('preview')

  // Console and Network state
  const [consoleEntries, setConsoleEntries] = useState<ConsoleEntry[]>([])
  const [networkRequests, setNetworkRequests] = useState<NetworkRequest[]>([])

  // Backend server state
  const [serverStatus, setServerStatus] = useState<ServerStatus | null>(null)
  const [serverDetection, setServerDetection] = useState<ServerDetection | null>(null)
  const [serverLoading, setServerLoading] = useState(false)
  const [serverLogs, setServerLogs] = useState<{ stdout: string; stderr: string }>({ stdout: '', stderr: '' })
  const [showServerLogs, setShowServerLogs] = useState(false)

  // Docker sandbox state
  const [useSandbox, setUseSandbox] = useState(false)
  const [dockerAvailable, setDockerAvailable] = useState(false)

  // Bundler state
  const [bundlerAvailable, setBundlerAvailable] = useState(false)

  const iframeRef = useRef<HTMLIFrameElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  // Check Docker and bundler availability on mount
  useEffect(() => {
    const checkCapabilities = async () => {
      try {
        const response = await api.get('/preview/docker/status')
        setDockerAvailable(response.data.available === true)
      } catch {
        setDockerAvailable(false)
      }
      try {
        const response = await api.get('/preview/bundler/status')
        setBundlerAvailable(response.data.available === true)
      } catch {
        setBundlerAvailable(false)
      }
    }
    checkCapabilities()
  }, [])

  // Listen for postMessage from preview iframe
  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      // Security: Validate origin to prevent XSS from malicious iframes
      // Allow same origin, 'null' (sandboxed iframes), and preview server origin
      const allowedOrigins = [
        window.location.origin,
        'null', // Sandboxed iframes report null origin
      ]
      // Also allow any localhost/127.0.0.1 for preview server
      const isLocalhost = event.origin.startsWith('http://localhost:') ||
                          event.origin.startsWith('http://127.0.0.1:')

      if (!allowedOrigins.includes(event.origin) && !isLocalhost) {
        return // Ignore messages from untrusted origins
      }

      // Console messages
      if (event.data?.type === 'apex-console') {
        const entry: ConsoleEntry = {
          id: `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
          level: event.data.level,
          message: event.data.message,
          stack: event.data.stack,
          timestamp: event.data.timestamp
        }
        setConsoleEntries(prev => {
          const newEntries = [...prev, entry]
          // Limit entries to prevent memory issues
          if (newEntries.length > MAX_CONSOLE_ENTRIES) {
            return newEntries.slice(-MAX_CONSOLE_ENTRIES)
          }
          return newEntries
        })
      }

      // Network messages
      if (event.data?.type === 'apex-network') {
        const request: NetworkRequest = {
          id: event.data.id,
          method: event.data.method,
          url: event.data.url,
          status: event.data.status,
          statusText: event.data.statusText,
          duration: event.data.duration,
          error: event.data.error,
          timestamp: event.data.timestamp
        }
        setNetworkRequests(prev => {
          const newRequests = [...prev, request]
          if (newRequests.length > MAX_NETWORK_REQUESTS) {
            return newRequests.slice(-MAX_NETWORK_REQUESTS)
          }
          return newRequests
        })
      }
    }

    window.addEventListener('message', handleMessage)
    return () => window.removeEventListener('message', handleMessage)
  }, [])

  // Clear console/network on preview restart
  const clearDevTools = useCallback(() => {
    setConsoleEntries([])
    setNetworkRequests([])
  }, [])

  // Fetch preview status
  const fetchStatus = useCallback(async () => {
    try {
      const response = await api.get(`/preview/status/${projectId}`)
      setStatus(response.data.preview)
      if (response.data.preview?.active) {
        setPreviewUrl(response.data.preview.url)
        setConnected(true)
      } else {
        setConnected(false)
      }
    } catch (err: any) {
      // Preview not running - that's OK
      setStatus(null)
      setConnected(false)
    }
  }, [projectId])

  useEffect(() => {
    fetchStatus()
    // Poll for status updates
    const interval = setInterval(fetchStatus, 5000)
    return () => clearInterval(interval)
  }, [fetchStatus])

  // Start preview (wrapped in useCallback for dependency safety)
  const startPreview = useCallback(async () => {
    setLoading(true)
    setError(null)
    clearDevTools() // Clear old console/network data
    try {
      const response = await api.post('/preview/start', {
        project_id: projectId,
        sandbox: useSandbox
      })
      setStatus(response.data.preview)
      setPreviewUrl(response.data.preview.url)
      setConnected(true)
      setRefreshKey(prev => prev + 1)
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to start preview')
    } finally {
      setLoading(false)
    }
  }, [projectId, clearDevTools])

  // Auto-start preview when autoStart prop is true
  const autoStartRef = useRef(false)
  useEffect(() => {
    if (autoStart && !autoStartRef.current && !status?.active && !loading) {
      autoStartRef.current = true
      startPreview()
    }
  }, [autoStart, status?.active, loading, startPreview])

  // Stop preview
  const stopPreview = async () => {
    setLoading(true)
    try {
      await api.post('/preview/stop', {
        project_id: projectId
      })
      setStatus(null)
      setPreviewUrl('')
      setConnected(false)
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to stop preview')
    } finally {
      setLoading(false)
    }
  }

  // Refresh preview
  const refreshPreview = async () => {
    try {
      await api.post('/preview/refresh', {
        project_id: projectId
      })
      setRefreshKey(prev => prev + 1)
    } catch (err) {
      // Just reload the iframe
      setRefreshKey(prev => prev + 1)
    }
  }

  // ========== Backend Server Functions ==========

  // Detect backend server on mount
  useEffect(() => {
    const detectBackend = async () => {
      try {
        const response = await api.get(`/preview/server/detect/${projectId}`)
        setServerDetection(response.data)
      } catch (err) {
        // No backend detected or error - that's fine
        setServerDetection({ has_backend: false })
      }
    }
    detectBackend()
  }, [projectId])

  // Fetch server status
  const fetchServerStatus = useCallback(async () => {
    try {
      const response = await api.get(`/preview/server/status/${projectId}`)
      setServerStatus(response.data.server)
    } catch (err) {
      setServerStatus(null)
    }
  }, [projectId])

  // Poll server status
  useEffect(() => {
    if (serverDetection?.has_backend) {
      fetchServerStatus()
      const interval = setInterval(fetchServerStatus, 5000)
      return () => clearInterval(interval)
    }
  }, [serverDetection?.has_backend, fetchServerStatus])

  // Start backend server
  const startServer = async () => {
    setServerLoading(true)
    try {
      const response = await api.post('/preview/server/start', {
        project_id: projectId,
        entry_file: serverDetection?.entry_file,
        command: serverDetection?.command
      })
      setServerStatus({
        running: true,
        port: response.data.port,
        pid: response.data.pid,
        command: response.data.command,
        entry_file: response.data.entry_file,
        url: response.data.url,
        ready: true
      })
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to start backend server')
    } finally {
      setServerLoading(false)
    }
  }

  // Stop backend server
  const stopServer = async () => {
    setServerLoading(true)
    try {
      await api.post('/preview/server/stop', {
        project_id: projectId
      })
      setServerStatus(null)
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to stop backend server')
    } finally {
      setServerLoading(false)
    }
  }

  // Fetch server logs
  const fetchServerLogs = async () => {
    try {
      const response = await api.get(`/preview/server/logs/${projectId}`)
      setServerLogs({
        stdout: response.data.stdout || '',
        stderr: response.data.stderr || ''
      })
    } catch (err) {
      // Ignore errors
    }
  }

  // Fetch logs when showing server logs panel
  useEffect(() => {
    if (showServerLogs && serverStatus?.running) {
      fetchServerLogs()
      const interval = setInterval(fetchServerLogs, 2000)
      return () => clearInterval(interval)
    }
  }, [showServerLogs, serverStatus?.running])

  // Toggle fullscreen
  const toggleFullscreen = () => {
    if (!containerRef.current) return

    if (!document.fullscreenElement) {
      containerRef.current.requestFullscreen()
    } else {
      document.exitFullscreen()
    }
  }

  // Sync fullscreen state with browser (handles Escape key exit)
  useEffect(() => {
    const handleFullscreenChange = () => {
      setIsFullscreen(!!document.fullscreenElement)
    }
    document.addEventListener('fullscreenchange', handleFullscreenChange)
    return () => document.removeEventListener('fullscreenchange', handleFullscreenChange)
  }, [])

  // Open in new tab
  const openInNewTab = () => {
    if (previewUrl) {
      window.open(previewUrl, '_blank')
    }
  }

  // Get viewport style
  const getViewportStyle = () => {
    if (viewport === 'full') {
      return { width: '100%', height: '100%' }
    }
    const size = viewportSizes[viewport]
    return {
      width: `${size.width}px`,
      height: `${size.height}px`,
      maxWidth: '100%',
      maxHeight: '100%'
    }
  }

  // Count unread console errors
  const errorCount = consoleEntries.filter(e => e.level === 'error').length
  const warnCount = consoleEntries.filter(e => e.level === 'warn').length
  const networkErrorCount = networkRequests.filter(r => r.status === 0 || r.status >= 400).length

  return (
    <div
      ref={containerRef}
      className={`flex flex-col bg-gray-900 border border-gray-700 rounded-lg overflow-hidden ${className}`}
    >
      {/* Toolbar */}
      <div className="flex items-center justify-between px-3 py-2 bg-gray-800/50 border-b border-gray-700">
        <div className="flex items-center gap-2">
          {/* Status indicator */}
          <div className={`flex items-center gap-1.5 px-2 py-1 rounded-md text-xs ${
            connected ? 'bg-green-500/20 text-green-400' : 'bg-gray-700 text-gray-400'
          }`}>
            {connected ? (
              <>
                <Wifi className="w-3 h-3" />
                <span>Live</span>
              </>
            ) : (
              <>
                <WifiOff className="w-3 h-3" />
                <span>Offline</span>
              </>
            )}
          </div>

          {/* Play/Stop button */}
          {status?.active ? (
            <button
              onClick={stopPreview}
              disabled={loading}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-red-600/20 hover:bg-red-600/30 text-red-400 rounded-md text-sm transition-colors disabled:opacity-50"
            >
              {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Square className="w-4 h-4" />}
              Stop
            </button>
          ) : (
            <button
              onClick={startPreview}
              disabled={loading}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-green-600/20 hover:bg-green-600/30 text-green-400 rounded-md text-sm transition-colors disabled:opacity-50"
            >
              {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
              Start Preview
            </button>
          )}

          {/* Refresh button */}
          <button
            onClick={refreshPreview}
            disabled={!status?.active || loading}
            className="p-1.5 hover:bg-gray-700 rounded-md text-gray-400 hover:text-white transition-colors disabled:opacity-50"
            title="Refresh"
          >
            <RefreshCw className="w-4 h-4" />
          </button>

          {/* Backend Server Controls - only show if backend detected */}
          {serverDetection?.has_backend && (
            <div className="flex items-center gap-1.5 ml-2 pl-2 border-l border-gray-600">
              {/* Server status indicator */}
              <div className={`flex items-center gap-1.5 px-2 py-1 rounded-md text-xs ${
                serverStatus?.running
                  ? 'bg-purple-500/20 text-purple-400'
                  : 'bg-gray-700 text-gray-400'
              }`}>
                <Server className="w-3 h-3" />
                <span>{serverStatus?.running ? `API :${serverStatus.port}` : 'API Off'}</span>
              </div>

              {/* Server start/stop button */}
              {serverStatus?.running ? (
                <button
                  onClick={stopServer}
                  disabled={serverLoading}
                  className="flex items-center gap-1 px-2 py-1 bg-red-600/20 hover:bg-red-600/30 text-red-400 rounded-md text-xs transition-colors disabled:opacity-50"
                  title="Stop Backend Server"
                >
                  {serverLoading ? <Loader2 className="w-3 h-3 animate-spin" /> : <PowerOff className="w-3 h-3" />}
                  Stop
                </button>
              ) : (
                <button
                  onClick={startServer}
                  disabled={serverLoading}
                  className="flex items-center gap-1 px-2 py-1 bg-purple-600/20 hover:bg-purple-600/30 text-purple-400 rounded-md text-xs transition-colors disabled:opacity-50"
                  title={`Start ${serverDetection.framework || serverDetection.server_type} Server`}
                >
                  {serverLoading ? <Loader2 className="w-3 h-3 animate-spin" /> : <Power className="w-3 h-3" />}
                  Start API
                </button>
              )}

              {/* Server logs button */}
              {serverStatus?.running && (
                <button
                  onClick={() => setShowServerLogs(!showServerLogs)}
                  className={`p-1.5 rounded-md transition-colors ${
                    showServerLogs
                      ? 'bg-purple-600 text-white'
                      : 'hover:bg-gray-700 text-gray-400 hover:text-white'
                  }`}
                  title="Server Logs"
                >
                  <FileText className="w-3.5 h-3.5" />
                </button>
              )}
            </div>
          )}

          {/* Tab buttons */}
          <div className="flex items-center bg-gray-800 rounded-md p-0.5 ml-2">
            <button
              onClick={() => setActiveTab('preview')}
              className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
                activeTab === 'preview' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'
              }`}
            >
              Preview
            </button>
            <button
              onClick={() => setActiveTab('console')}
              className={`flex items-center gap-1.5 px-3 py-1 rounded text-xs font-medium transition-colors ${
                activeTab === 'console' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'
              }`}
            >
              <Terminal className="w-3 h-3" />
              Console
              {(errorCount > 0 || warnCount > 0) && (
                <span className={`px-1.5 py-0.5 rounded-full text-[10px] ${
                  errorCount > 0 ? 'bg-red-500 text-white' : 'bg-yellow-500 text-black'
                }`}>
                  {errorCount > 0 ? errorCount : warnCount}
                </span>
              )}
            </button>
            <button
              onClick={() => setActiveTab('network')}
              className={`flex items-center gap-1.5 px-3 py-1 rounded text-xs font-medium transition-colors ${
                activeTab === 'network' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'
              }`}
            >
              <Globe className="w-3 h-3" />
              Network
              {networkErrorCount > 0 && (
                <span className="px-1.5 py-0.5 rounded-full text-[10px] bg-red-500 text-white">
                  {networkErrorCount}
                </span>
              )}
            </button>
          </div>
        </div>

        <div className="flex items-center gap-1">
          {/* Viewport size buttons */}
          <div className="flex items-center bg-gray-800 rounded-md p-0.5 mr-2">
            <button
              onClick={() => setViewport('mobile')}
              className={`p-1.5 rounded ${viewport === 'mobile' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'}`}
              title="Mobile (375x667)"
            >
              <Smartphone className="w-4 h-4" />
            </button>
            <button
              onClick={() => setViewport('tablet')}
              className={`p-1.5 rounded ${viewport === 'tablet' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'}`}
              title="Tablet (768x1024)"
            >
              <Tablet className="w-4 h-4" />
            </button>
            <button
              onClick={() => setViewport('desktop')}
              className={`p-1.5 rounded ${viewport === 'desktop' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'}`}
              title="Desktop (1280x800)"
            >
              <Monitor className="w-4 h-4" />
            </button>
            <button
              onClick={() => setViewport('full')}
              className={`p-1.5 rounded ${viewport === 'full' ? 'bg-cyan-600 text-white' : 'text-gray-400 hover:text-white'}`}
              title="Full Width"
            >
              <Maximize2 className="w-4 h-4" />
            </button>
          </div>

          {/* Fullscreen toggle */}
          <button
            onClick={toggleFullscreen}
            className="p-1.5 hover:bg-gray-700 rounded-md text-gray-400 hover:text-white transition-colors"
            title={isFullscreen ? 'Exit Fullscreen' : 'Fullscreen'}
          >
            {isFullscreen ? <Minimize2 className="w-4 h-4" /> : <Maximize2 className="w-4 h-4" />}
          </button>

          {/* Open in new tab */}
          <button
            onClick={openInNewTab}
            disabled={!status?.active}
            className="p-1.5 hover:bg-gray-700 rounded-md text-gray-400 hover:text-white transition-colors disabled:opacity-50"
            title="Open in New Tab"
          >
            <ExternalLink className="w-4 h-4" />
          </button>

          {/* Settings */}
          <div className="relative">
            <button
              onClick={() => setShowSettings(!showSettings)}
              className="p-1.5 hover:bg-gray-700 rounded-md text-gray-400 hover:text-white transition-colors"
              title="Settings"
            >
              <Settings className="w-4 h-4" />
            </button>

            {showSettings && (
              <div className="absolute right-0 top-full mt-1 w-64 bg-gray-800 border border-gray-700 rounded-lg shadow-xl z-50">
                <div className="p-3">
                  <h4 className="text-sm font-medium text-white mb-3">Preview Settings</h4>
                  <div className="space-y-3">
                    <label className="flex items-center justify-between">
                      <span className="text-sm text-gray-400">Auto-refresh on save</span>
                      <input type="checkbox" defaultChecked className="rounded bg-gray-700 border-gray-600 text-cyan-500" />
                    </label>
                    <label className="flex items-center justify-between">
                      <span className="text-sm text-gray-400">Show dev tools</span>
                      <input type="checkbox" defaultChecked className="rounded bg-gray-700 border-gray-600 text-cyan-500" />
                    </label>

                    {/* Docker Sandbox Toggle */}
                    <label className="flex items-center justify-between">
                      <div className="flex items-center gap-1.5">
                        {dockerAvailable ? <Shield className="w-3.5 h-3.5 text-green-400" /> : <ShieldOff className="w-3.5 h-3.5 text-gray-500" />}
                        <span className="text-sm text-gray-400">Docker Sandbox</span>
                      </div>
                      <input
                        type="checkbox"
                        checked={useSandbox}
                        onChange={(e) => setUseSandbox(e.target.checked)}
                        disabled={!dockerAvailable}
                        className="rounded bg-gray-700 border-gray-600 text-cyan-500 disabled:opacity-40"
                      />
                    </label>
                    {!dockerAvailable && (
                      <p className="text-[10px] text-gray-600 -mt-1">Docker not available on server</p>
                    )}

                    {/* Bundler Status */}
                    <div className="flex items-center justify-between">
                      <span className="text-sm text-gray-400">esbuild Bundler</span>
                      <span className={`text-xs px-1.5 py-0.5 rounded ${bundlerAvailable ? 'bg-green-500/20 text-green-400' : 'bg-gray-700 text-gray-500'}`}>
                        {bundlerAvailable ? 'Available' : 'Not Found'}
                      </span>
                    </div>

                    <div>
                      <span className="text-sm text-gray-400">Custom URL</span>
                      <input
                        type="text"
                        placeholder="/custom-path"
                        className="w-full mt-1 px-2 py-1 bg-gray-900 border border-gray-700 rounded text-sm text-white"
                      />
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* URL Bar - only show on preview tab */}
      {status?.active && activeTab === 'preview' && (
        <div className="flex items-center gap-2 px-3 py-1.5 bg-gray-800/30 border-b border-gray-700">
          <div className="flex-1 flex items-center bg-gray-900 rounded-md px-3 py-1">
            <span className="text-xs text-gray-500 mr-2">
              {connected ? <CheckCircle className="w-3 h-3 text-green-400" /> : <AlertCircle className="w-3 h-3 text-yellow-400" />}
            </span>
            <input
              type="text"
              value={previewUrl}
              readOnly
              className="flex-1 bg-transparent text-sm text-gray-300 outline-none"
            />
          </div>
        </div>
      )}

      {/* Tab Content */}
      <div className="flex-1 overflow-hidden">
        {/* Preview Tab */}
        {activeTab === 'preview' && (
          <div className="h-full flex items-center justify-center bg-gray-950 overflow-auto p-4">
            {error && (
              <div className="flex flex-col items-center justify-center text-red-400">
                <AlertCircle className="w-12 h-12 mb-3" />
                <p className="text-sm">{error}</p>
                <button
                  onClick={() => setError(null)}
                  className="mt-3 px-4 py-2 bg-gray-800 hover:bg-gray-700 rounded-md text-sm"
                >
                  Dismiss
                </button>
              </div>
            )}

            {!status?.active && !error && (
              <div className="flex flex-col items-center justify-center text-gray-500">
                <Monitor className="w-16 h-16 mb-4 opacity-30" />
                <p className="text-lg mb-2">Preview not running</p>
                <p className="text-sm text-gray-600 mb-4">Click "Start Preview" to see your app</p>
                <button
                  onClick={startPreview}
                  disabled={loading}
                  className="flex items-center gap-2 px-6 py-3 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg transition-colors disabled:opacity-50"
                >
                  {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <Play className="w-5 h-5" />}
                  Start Preview
                </button>
              </div>
            )}

            {status?.active && previewUrl && (
              <div
                className="bg-white rounded-lg shadow-2xl overflow-hidden transition-all duration-300"
                style={getViewportStyle()}
              >
                <iframe
                  ref={iframeRef}
                  key={refreshKey}
                  src={previewUrl}
                  className="w-full h-full border-0"
                  title="Live Preview"
                  sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-modals"
                />
              </div>
            )}
          </div>
        )}

        {/* Console Tab */}
        {activeTab === 'console' && (
          <ConsolePanel
            entries={consoleEntries}
            onClear={() => setConsoleEntries([])}
            className="h-full"
          />
        )}

        {/* Network Tab */}
        {activeTab === 'network' && (
          <NetworkPanel
            requests={networkRequests}
            onClear={() => setNetworkRequests([])}
            className="h-full"
          />
        )}
      </div>

      {/* Server Logs Panel - slides up from bottom */}
      {showServerLogs && serverStatus?.running && (
        <div className="border-t border-gray-700 bg-gray-900 max-h-64 overflow-hidden flex flex-col">
          <div className="flex items-center justify-between px-3 py-2 bg-gray-800/50 border-b border-gray-700">
            <div className="flex items-center gap-2">
              <Server className="w-4 h-4 text-purple-400" />
              <span className="text-sm font-medium text-white">Backend Server Logs</span>
              <span className="text-xs text-gray-500">
                {serverDetection?.framework || serverDetection?.server_type} | Port {serverStatus.port}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={fetchServerLogs}
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

      {/* Status bar */}
      {status?.active && (
        <div className="flex items-center justify-between px-3 py-1.5 bg-gray-800/30 border-t border-gray-700 text-xs text-gray-500">
          <div className="flex items-center gap-4">
            <span>Port: {status.port}</span>
            <span>Clients: {status.connected_clients}</span>
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
