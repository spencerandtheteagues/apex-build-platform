import { useCallback, useEffect, useRef, useState } from 'react'
import apiService from '@/services/api'
import type { PreviewStatus, ServerDetection, ServerStatus } from '@/components/preview/types'

interface UsePreviewRuntimeOptions {
  projectId: number
  autoStart: boolean
  clearDevTools: () => void
  setError: (value: string | null) => void
  serverDetection: ServerDetection | null
  onServerStatusHint: (value: ServerStatus | null) => void
}

export function usePreviewRuntime({
  projectId,
  autoStart,
  clearDevTools,
  setError,
  serverDetection,
  onServerStatusHint,
}: UsePreviewRuntimeOptions) {
  const [status, setStatus] = useState<PreviewStatus | null>(null)
  const [loading, setLoading] = useState(false)
  const [previewUrl, setPreviewUrl] = useState('')
  const [iframeLoading, setIframeLoading] = useState(false)
  const [iframeError, setIframeError] = useState<string | null>(null)
  const [connected, setConnected] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)
  const [useSandbox, setUseSandbox] = useState(false)
  const [activeSandbox, setActiveSandbox] = useState(false)
  const [dockerAvailable, setDockerAvailable] = useState(false)
  const [sandboxRequired, setSandboxRequired] = useState(false)
  const [sandboxDegraded, setSandboxDegraded] = useState(false)
  const [backendPreviewAvailable, setBackendPreviewAvailable] = useState(true)
  const [backendPreviewReason, setBackendPreviewReason] = useState('')
  const [bundlerAvailable, setBundlerAvailable] = useState(false)

  const activeProjectIdRef = useRef(projectId)
  const lastAutoStartedProjectRef = useRef<number | null>(null)

  useEffect(() => {
    activeProjectIdRef.current = projectId
  }, [projectId])

  useEffect(() => {
    const checkCapabilities = async () => {
      try {
        const response = await apiService.client.get('/preview/docker/status')
        const previewDockerAvailable = response.data.available === true
        const previewSandboxRequired = response.data.sandbox_required === true
        const previewSandboxDegraded = response.data.sandbox_degraded === true
        setDockerAvailable(previewDockerAvailable)
        setSandboxRequired(previewSandboxRequired)
        setSandboxDegraded(previewSandboxDegraded)
        setBackendPreviewAvailable(response.data.backend_preview_available !== false)
        setBackendPreviewReason(response.data.backend_preview_reason || '')
        if (previewSandboxRequired) {
          setUseSandbox(true)
        }
      } catch {
        setDockerAvailable(false)
      }

      try {
        const response = await apiService.client.get('/preview/bundler/status')
        setBundlerAvailable(response.data.available === true)
      } catch {
        setBundlerAvailable(false)
      }
    }

    void checkCapabilities()
  }, [])

  useEffect(() => {
    setStatus(null)
    setError(null)
    setLoading(false)
    setPreviewUrl('')
    setIframeLoading(false)
    setIframeError(null)
    setConnected(false)
    setActiveSandbox(false)
    onServerStatusHint(null)
    clearDevTools()
    lastAutoStartedProjectRef.current = null
  }, [clearDevTools, onServerStatusHint, projectId, setError])

  useEffect(() => {
    if (status?.active && previewUrl) {
      setIframeLoading(true)
      setIframeError(null)
      return
    }
    if (!status?.active) {
      setIframeLoading(false)
      setIframeError(null)
    }
  }, [previewUrl, status?.active])

  const fetchStatus = useCallback(async () => {
    const requestProjectId = projectId
    const statusRequestSandbox = status?.active ? activeSandbox : useSandbox

    try {
      const response = await apiService.client.get(`/preview/status/${projectId}`, {
        params: { sandbox: statusRequestSandbox ? '1' : '0' },
      })

      if (activeProjectIdRef.current !== requestProjectId) return

      setSandboxDegraded(response.data.sandbox_degraded === true)
      setStatus(response.data.preview)

      if (response.data.preview?.active) {
        if (typeof response.data.sandbox === 'boolean') {
          setActiveSandbox(response.data.sandbox)
        }
        if (response.data.server !== undefined) {
          onServerStatusHint(response.data.server)
        }
        setPreviewUrl(response.data.preview.url)
        setConnected(true)
        return
      }

      if (response.data.server !== undefined) {
        onServerStatusHint(response.data.server)
      } else {
        onServerStatusHint(null)
      }

      setPreviewUrl('')
      setIframeLoading(false)
      setIframeError(null)
      setConnected(false)
    } catch (err: any) {
      if (activeProjectIdRef.current !== requestProjectId) return

      const statusCode = err?.response?.status
      const previewMissing = statusCode === 404 || statusCode === 410
      if (previewMissing) {
        setStatus(null)
        setPreviewUrl('')
        setIframeLoading(false)
        setIframeError(null)
      }
      setConnected(false)
    }
  }, [activeSandbox, onServerStatusHint, projectId, status?.active, useSandbox])

  useEffect(() => {
    void fetchStatus()
    const interval = setInterval(fetchStatus, 5000)
    return () => clearInterval(interval)
  }, [fetchStatus])

  const startPreview = useCallback(async () => {
    const requestProjectId = projectId
    setLoading(true)
    setError(null)
    clearDevTools()

    const maxRetries = 3
    for (let attempt = 0; attempt <= maxRetries; attempt += 1) {
      try {
        let data: any
        try {
          data = await apiService.startFullStackPreview({
            project_id: projectId,
            sandbox: useSandbox,
            start_backend: Boolean(serverDetection?.has_backend),
            require_backend: false,
            backend_entry_file: serverDetection?.entry_file,
            backend_command: serverDetection?.command,
          })
        } catch (fullStackErr: any) {
          const statusCode = fullStackErr?.response?.status
          if (statusCode !== 404 && statusCode !== 405) {
            throw fullStackErr
          }
          const response = await apiService.client.post('/preview/start', {
            project_id: projectId,
            sandbox: useSandbox,
          })
          data = response.data
        }

        if (activeProjectIdRef.current !== requestProjectId) return

        const actualSandbox = typeof data.sandbox === 'boolean' ? data.sandbox : useSandbox
        setStatus(data.preview)
        setPreviewUrl(data.proxy_url || data.preview?.url || data.url || '')
        setIframeLoading(true)
        setIframeError(null)
        setConnected(true)
        setActiveSandbox(actualSandbox)
        setUseSandbox(actualSandbox)
        setSandboxDegraded(data.sandbox_degraded === true)
        if (data.server !== undefined) {
          onServerStatusHint(data.server)
        } else {
          onServerStatusHint(null)
        }

        if (data.degraded && data.diagnostics?.backend_error) {
          setError(`Preview degraded: ${data.diagnostics.backend_error}`)
        }
        setRefreshKey(prev => prev + 1)
        setLoading(false)
        return
      } catch (err: any) {
        if (activeProjectIdRef.current !== requestProjectId) return

        const statusCode = err.response?.status
        if (statusCode === 429 && attempt < maxRetries) {
          await new Promise(resolve => setTimeout(resolve, (attempt + 1) * 2000))
          continue
        }

        setError(err.response?.data?.error || 'Failed to start preview')
        break
      }
    }

    if (activeProjectIdRef.current === requestProjectId) {
      setLoading(false)
    }
  }, [clearDevTools, onServerStatusHint, projectId, serverDetection, setError, useSandbox])

  useEffect(() => {
    const activeForCurrentProject = status?.active && status.project_id === projectId
    if (!autoStart || !projectId || activeForCurrentProject || loading) return
    if (lastAutoStartedProjectRef.current === projectId) return

    lastAutoStartedProjectRef.current = projectId
    void startPreview()
  }, [autoStart, loading, projectId, startPreview, status?.active, status?.project_id])

  useEffect(() => {
    if (!autoStart && lastAutoStartedProjectRef.current === projectId) {
      lastAutoStartedProjectRef.current = null
    }
  }, [autoStart, projectId])

  const stopPreview = useCallback(async (options?: { silent?: boolean }) => {
    setLoading(true)
    try {
      await apiService.client.post('/preview/stop', {
        project_id: projectId,
        sandbox: activeSandbox,
      })
      setStatus(null)
      setPreviewUrl('')
      setIframeLoading(false)
      setIframeError(null)
      setConnected(false)
      setActiveSandbox(false)
      onServerStatusHint(null)
      return true
    } catch (err: any) {
      if (!options?.silent) {
        setError(err.response?.data?.error || 'Failed to stop preview')
      }
      return false
    } finally {
      setLoading(false)
    }
  }, [activeSandbox, onServerStatusHint, projectId, setError])

  const restartPreview = useCallback(async () => {
    if (loading) return
    setError(null)
    setIframeError(null)
    if (status?.active) {
      await stopPreview({ silent: true })
    }
    await startPreview()
  }, [loading, setError, startPreview, status?.active, stopPreview])

  const refreshPreview = useCallback(async () => {
    try {
      setIframeLoading(true)
      setIframeError(null)
      await apiService.client.post('/preview/refresh', {
        project_id: projectId,
        sandbox: activeSandbox,
      })
      setRefreshKey(prev => prev + 1)
    } catch {
      setRefreshKey(prev => prev + 1)
    }
  }, [activeSandbox, projectId])

  return {
    status,
    loading,
    previewUrl,
    iframeLoading,
    setIframeLoading,
    iframeError,
    setIframeError,
    connected,
    refreshKey,
    useSandbox,
    setUseSandbox,
    activeSandbox,
    dockerAvailable,
    sandboxRequired,
    sandboxDegraded,
    backendPreviewAvailable,
    backendPreviewReason,
    bundlerAvailable,
    fetchStatus,
    startPreview,
    stopPreview,
    restartPreview,
    refreshPreview,
  }
}

export default usePreviewRuntime
