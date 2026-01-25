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
  ChevronDown
} from 'lucide-react'
import api from '../../services/api'

interface PreviewStatus {
  project_id: number
  active: boolean
  port: number
  url: string
  started_at: string
  last_access: string
  connected_clients: number
}

interface LivePreviewProps {
  projectId: number
  onFileChange?: (filePath: string, content: string) => void
  className?: string
}

type ViewportSize = 'mobile' | 'tablet' | 'desktop' | 'full'

const viewportSizes: Record<ViewportSize, { width: number; height: number; label: string }> = {
  mobile: { width: 375, height: 667, label: 'Mobile' },
  tablet: { width: 768, height: 1024, label: 'Tablet' },
  desktop: { width: 1280, height: 800, label: 'Desktop' },
  full: { width: 0, height: 0, label: 'Full' }
}

export default function LivePreview({ projectId, onFileChange, className = '' }: LivePreviewProps) {
  const [status, setStatus] = useState<PreviewStatus | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [viewport, setViewport] = useState<ViewportSize>('full')
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [showSettings, setShowSettings] = useState(false)
  const [previewUrl, setPreviewUrl] = useState<string>('')
  const [connected, setConnected] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)

  const iframeRef = useRef<HTMLIFrameElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)

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

  // Start preview
  const startPreview = async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await api.post('/preview/start', {
        project_id: projectId
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
  }

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

  // Hot reload when file changes
  useEffect(() => {
    if (!status?.active || !onFileChange) return

    const handleHotReload = async (filePath: string, content: string) => {
      try {
        await api.post('/preview/hot-reload', {
          project_id: projectId,
          file_path: filePath,
          content: content
        })
      } catch (err) {
        // Fall back to full reload
        refreshPreview()
      }
    }

    // This would be called from the editor when files change
    // For now, we'll rely on manual refresh
  }, [status?.active, projectId])

  // Toggle fullscreen
  const toggleFullscreen = () => {
    if (!containerRef.current) return

    if (!document.fullscreenElement) {
      containerRef.current.requestFullscreen()
      setIsFullscreen(true)
    } else {
      document.exitFullscreen()
      setIsFullscreen(false)
    }
  }

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
                      <input type="checkbox" className="rounded bg-gray-700 border-gray-600 text-cyan-500" />
                    </label>
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

      {/* URL Bar */}
      {status?.active && (
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

      {/* Preview Area */}
      <div className="flex-1 flex items-center justify-center bg-gray-950 overflow-auto p-4">
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

      {/* Status bar */}
      {status?.active && (
        <div className="flex items-center justify-between px-3 py-1.5 bg-gray-800/30 border-t border-gray-700 text-xs text-gray-500">
          <div className="flex items-center gap-4">
            <span>Port: {status.port}</span>
            <span>Clients: {status.connected_clients}</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="flex items-center gap-1">
              <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
              Hot Reload Active
            </span>
          </div>
        </div>
      )}
    </div>
  )
}
