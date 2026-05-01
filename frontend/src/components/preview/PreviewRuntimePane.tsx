import type { CSSProperties, Ref } from 'react'
import { AlertCircle, Loader2, Monitor, Play } from 'lucide-react'
import type { PreviewStatus } from './types'
import { previewRuntimeStateLabels, type PreviewRuntimeState } from './previewState'

interface PreviewRuntimePaneProps {
  status: PreviewStatus | null
  runtimeState: PreviewRuntimeState
  previewSrc: string
  viewportStyle: CSSProperties
  refreshKey: number
  iframeRef: Ref<HTMLIFrameElement>
  loading: boolean
  error: string | null
  iframeLoading: boolean
  iframeError: string | null
  onStartPreview: () => void
  onDismissError: () => void
  onIframeLoad: () => void
  onIframeError: () => void
}

export default function PreviewRuntimePane({
  status,
  runtimeState,
  previewSrc,
  viewportStyle,
  refreshKey,
  iframeRef,
  loading,
  error,
  iframeLoading,
  iframeError,
  onStartPreview,
  onDismissError,
  onIframeLoad,
  onIframeError,
}: PreviewRuntimePaneProps) {
  const runtimeLabel = previewRuntimeStateLabels[runtimeState]

  return (
    <div className="relative h-full flex items-center justify-center bg-gray-950 overflow-auto p-4">
      {error && !status?.active && (
        <div className="flex flex-col items-center justify-center text-red-400">
          <AlertCircle className="w-12 h-12 mb-3" />
          <p className="text-sm">{error}</p>
          <button
            onClick={onDismissError}
            className="mt-3 px-4 py-2 bg-gray-800 hover:bg-gray-700 rounded-md text-sm"
          >
            Dismiss
          </button>
        </div>
      )}

      {!status?.active && !error && (
        <div className="flex flex-col items-center justify-center text-gray-500 gap-4">
          <div className="w-16 h-16 rounded-2xl bg-gray-800/60 border border-gray-700/50 flex items-center justify-center">
            <Monitor className="w-8 h-8 text-gray-600" />
          </div>
          <div className="text-center">
            <p className="text-sm font-medium text-gray-300 mb-1">Preview not running</p>
            <p className="text-xs text-gray-600">Start the preview server to see your app</p>
          </div>
          <button
            onClick={onStartPreview}
            disabled={loading}
            className="flex items-center gap-2 px-5 py-2.5 bg-green-600/20 hover:bg-green-600/30 border border-green-600/30 hover:border-green-500/40 text-green-400 rounded-xl text-sm font-medium transition-all duration-150 disabled:opacity-50"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
            Start Preview
          </button>
        </div>
      )}

      {status?.active && previewSrc && (
        <div
          className="relative bg-white rounded-lg shadow-2xl overflow-hidden transition-all duration-300"
          style={viewportStyle}
        >
          {iframeLoading && (
            <div className="absolute inset-0 z-10 flex items-center justify-center bg-white/90 text-gray-700 pointer-events-none">
              <div className="flex items-center gap-2 text-sm">
                <Loader2 className="w-4 h-4 animate-spin" />
                <span>Loading app preview...</span>
              </div>
            </div>
          )}
          <iframe
            ref={iframeRef}
            key={`${refreshKey}-${previewSrc}`}
            src={previewSrc}
            className="w-full h-full border-0"
            title="Live Preview"
            sandbox="allow-same-origin allow-scripts allow-forms allow-popups allow-modals"
            onLoad={onIframeLoad}
            onError={onIframeError}
          />
        </div>
      )}

      {status?.active && !previewSrc && !error && (
        <div className="flex flex-col items-center justify-center text-gray-400">
          <Loader2 className="w-8 h-8 animate-spin mb-3" />
          <p className="text-sm">{runtimeLabel} preview runtime...</p>
        </div>
      )}

      {status?.active && iframeError && !error && (
        <div className="absolute bottom-6 left-1/2 -translate-x-1/2 z-20 px-4 py-2 rounded-lg border border-yellow-500/30 bg-yellow-500/10 text-yellow-200 text-sm">
          {iframeError}
        </div>
      )}
    </div>
  )
}
