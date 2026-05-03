import { useCallback, useEffect, useRef, useState } from 'react'
import apiService, { PREVIEW_START_TIMEOUT_MS } from '@/services/api'
import type { PreviewStatus, ServerDetection, ServerStatus } from '@/components/preview/types'

export function formatPreviewStartError(responseData: any): string {
  const diagnostics = responseData?.diagnostics || {}
  const details = [
    responseData?.details,
    diagnostics.sandbox_error,
    diagnostics.runtime_error,
    diagnostics.preview_error,
    diagnostics.backend_error,
    responseData?.message,
  ]
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .filter((value, index, values) => values.indexOf(value) === index)
    .join(' | ')

  const message = responseData?.error || responseData?.message || 'Failed to start preview'
  return details ? `${message}: ${details}` : message
}

export function stablePreviewEmbedUrl(rawUrl: string): string {
  if (!rawUrl) return ''

  try {
    const url = new URL(rawUrl, window.location.origin)
    if (url.pathname.includes('/api/v1/preview/proxy/')) {
      // Preview status can rotate scoped auth tokens on every poll. The API
      // also sets the HttpOnly preview cookie, so keeping the query token in
      // iframe src only creates a false URL change and reloads the app.
      url.searchParams.delete('preview_token')
      url.searchParams.delete('token')
      return url.toString()
    }
  } catch {
    // Fall through to the conservative string cleanup below.
  }

  return rawUrl
    .replace(/([?&])preview_token=[^&]*/g, '$1')
    .replace(/([?&])token=[^&]*/g, '$1')
    .replace(/[?&]$/, '')
    .replace(/\?&/, '?')
}

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
  const activeSandboxRef = useRef(activeSandbox)
  const useSandboxRef = useRef(useSandbox)
  const statusActiveRef = useRef(status?.active)

  useEffect(() => {
    activeProjectIdRef.current = projectId
  }, [projectId])

  useEffect(() => {
    activeSandboxRef.current = activeSandbox
  }, [activeSandbox])

  useEffect(() => {
    useSandboxRef.current = useSandbox
  }, [useSandbox])

  useEffect(() => {
    statusActiveRef.current = status?.active
  }, [status?.active])

  const setPreferredUseSandbox = useCallback((value: boolean) => {
    useSandboxRef.current = value
    setUseSandbox(value)
  }, [])

  const loadPreviewCapabilities = useCallback(async () => {
    const capabilities = {
      dockerAvailable: false,
      sandboxRequired: false,
      sandboxCapabilityDegraded: false,
      bundlerAvailable: false,
    }

    try {
      const response = await apiService.client.get('/preview/docker/status')
      capabilities.dockerAvailable = response.data.available === true
      capabilities.sandboxRequired = response.data.sandbox_required === true
      capabilities.sandboxCapabilityDegraded = response.data.sandbox_degraded === true

      setDockerAvailable(capabilities.dockerAvailable)
      setSandboxRequired(capabilities.sandboxRequired)
      if (!statusActiveRef.current) {
        setSandboxDegraded(capabilities.sandboxCapabilityDegraded)
      }
      setBackendPreviewAvailable(response.data.backend_preview_available !== false)
      setBackendPreviewReason(response.data.backend_preview_reason || '')
      if (capabilities.sandboxRequired) {
        setPreferredUseSandbox(capabilities.dockerAvailable && !capabilities.sandboxCapabilityDegraded)
      }
    } catch {
      setDockerAvailable(false)
      if (!statusActiveRef.current) {
        setSandboxDegraded(true)
      }
    }

    try {
      const response = await apiService.client.get('/preview/bundler/status')
      capabilities.bundlerAvailable = response.data.available === true
      setBundlerAvailable(capabilities.bundlerAvailable)
    } catch {
      setBundlerAvailable(false)
    }

    return capabilities
  }, [setPreferredUseSandbox])

  useEffect(() => {
    void loadPreviewCapabilities()
  }, [loadPreviewCapabilities])

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
    statusActiveRef.current = false
    lastAutoStartedProjectRef.current = null
  }, [clearDevTools, onServerStatusHint, projectId, setError])

  const fetchStatus = useCallback(async () => {
    const requestProjectId = projectId
    const statusRequestSandbox = statusActiveRef.current ? activeSandboxRef.current : useSandboxRef.current

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
        setPreviewUrl(stablePreviewEmbedUrl(response.data.preview.url))
        setIframeError(null)
        setError(null)
        setConnected(true)
        statusActiveRef.current = true
        return true
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
      statusActiveRef.current = false
      return false
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
      return false
    }
  }, [onServerStatusHint, projectId, setError])

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
        const capabilities = await loadPreviewCapabilities()
        const preferredSandbox = capabilities.sandboxRequired
          ? capabilities.dockerAvailable && !capabilities.sandboxCapabilityDegraded
          : useSandboxRef.current
        const requestedSandbox = preferredSandbox && capabilities.dockerAvailable && !capabilities.sandboxCapabilityDegraded
        let data: any
        if (serverDetection?.has_backend === true) {
          try {
            const fullStackRequest: {
              project_id: number
              sandbox: boolean
              require_backend: boolean
              start_backend: boolean
              backend_entry_file?: string
              backend_command?: string
            } = {
              project_id: projectId,
              sandbox: requestedSandbox,
              require_backend: false,
              start_backend: true,
              backend_entry_file: serverDetection.entry_file,
              backend_command: serverDetection.command,
            }
            data = await apiService.startFullStackPreview(fullStackRequest)
          } catch (fullStackErr: any) {
            const statusCode = fullStackErr?.response?.status
            const terminalStartFailure = statusCode === 401 || statusCode === 403 || statusCode === 429
            if (terminalStartFailure) {
              throw fullStackErr
            }
            try {
              const response = await apiService.client.post('/preview/start', {
                project_id: projectId,
                sandbox: requestedSandbox,
              }, {
                timeout: PREVIEW_START_TIMEOUT_MS,
              })
              data = response.data
            } catch {
              throw fullStackErr
            }
          }
        } else {
          const response = await apiService.client.post('/preview/start', {
            project_id: projectId,
            sandbox: requestedSandbox,
          }, {
            timeout: PREVIEW_START_TIMEOUT_MS,
          })
          data = response.data
        }

        if (activeProjectIdRef.current !== requestProjectId) return

        const actualSandbox = typeof data.sandbox === 'boolean' ? data.sandbox : requestedSandbox
        setStatus(data.preview)
        setPreviewUrl(stablePreviewEmbedUrl(data.proxy_url || data.preview?.url || data.url || ''))
        setIframeLoading(true)
        setIframeError(null)
        setConnected(true)
        statusActiveRef.current = true
        setActiveSandbox(actualSandbox)
        setSandboxDegraded(data.sandbox_degraded === true)
        if (data.server !== undefined) {
          onServerStatusHint(data.server)
        } else {
          onServerStatusHint(null)
        }

        // A degraded optional backend is not a failed preview. If the frontend
        // preview is active, keep the iframe visible and let the status cards
        // explain backend/runtime degradation instead of pinning an old error.
        setError(null)
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

        try {
          const recovered = await fetchStatus()
          if (recovered) {
            setError(null)
            setIframeError(null)
            break
          }
        } catch {
          // Keep the original startup failure visible when status recovery is unavailable.
        }

        setError(formatPreviewStartError(err.response?.data))
        break
      }
    }

    if (activeProjectIdRef.current === requestProjectId) {
      setLoading(false)
    }
  }, [clearDevTools, fetchStatus, loadPreviewCapabilities, onServerStatusHint, projectId, serverDetection, setError])

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
      statusActiveRef.current = false
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
    setUseSandbox: setPreferredUseSandbox,
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
