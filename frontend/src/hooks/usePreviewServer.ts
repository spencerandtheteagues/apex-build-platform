import { useCallback, useEffect, useState } from 'react'
import apiService from '@/services/api'
import type { ServerDetection, ServerStatus } from '@/components/preview/types'

interface UsePreviewServerOptions {
  projectId: number
  setError: (value: string | null) => void
}

export function usePreviewServer({ projectId, setError }: UsePreviewServerOptions) {
  const [serverStatus, setServerStatus] = useState<ServerStatus | null>(null)
  const [serverDetection, setServerDetection] = useState<ServerDetection | null>(null)
  const [serverLoading, setServerLoading] = useState(false)
  const [serverLogs, setServerLogs] = useState<{ stdout: string; stderr: string }>({ stdout: '', stderr: '' })
  const [showServerLogs, setShowServerLogs] = useState(false)

  useEffect(() => {
    setServerStatus(null)
    setServerDetection(null)
    setServerLoading(false)
    setServerLogs({ stdout: '', stderr: '' })
    setShowServerLogs(false)
  }, [projectId])

  useEffect(() => {
    const detectBackend = async () => {
      try {
        const response = await apiService.client.get(`/preview/server/detect/${projectId}`)
        setServerDetection(response.data)
      } catch {
        setServerDetection({ has_backend: false })
      }
    }

    void detectBackend()
  }, [projectId])

  const fetchServerStatus = useCallback(async () => {
    try {
      const response = await apiService.client.get(`/preview/server/status/${projectId}`)
      setServerStatus(response.data.server)
    } catch {
      setServerStatus(null)
    }
  }, [projectId])

  useEffect(() => {
    if (!serverDetection?.has_backend) {
      return
    }

    void fetchServerStatus()
    const interval = setInterval(fetchServerStatus, 5000)
    return () => clearInterval(interval)
  }, [fetchServerStatus, serverDetection?.has_backend])

  const startServer = useCallback(async () => {
    setServerLoading(true)
    try {
      const response = await apiService.client.post('/preview/server/start', {
        project_id: projectId,
        entry_file: serverDetection?.entry_file,
        command: serverDetection?.command,
      })

      setServerStatus({
        running: true,
        port: response.data.port,
        pid: response.data.pid,
        command: response.data.command,
        entry_file: response.data.entry_file,
        url: response.data.url,
        ready: true,
      })
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to start backend server')
    } finally {
      setServerLoading(false)
    }
  }, [projectId, serverDetection?.command, serverDetection?.entry_file, setError])

  const stopServer = useCallback(async () => {
    setServerLoading(true)
    try {
      await apiService.client.post('/preview/server/stop', {
        project_id: projectId,
      })
      setServerStatus(null)
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to stop backend server')
    } finally {
      setServerLoading(false)
    }
  }, [projectId, setError])

  const fetchServerLogs = useCallback(async () => {
    try {
      const response = await apiService.client.get(`/preview/server/logs/${projectId}`)
      setServerLogs({
        stdout: response.data.stdout || '',
        stderr: response.data.stderr || '',
      })
    } catch {
      // Keep existing log state on transient failures.
    }
  }, [projectId])

  const toggleServerLogs = useCallback(async () => {
    const next = !showServerLogs
    setShowServerLogs(next)
    if (next) {
      await fetchServerLogs()
    }
  }, [fetchServerLogs, showServerLogs])

  useEffect(() => {
    if (!showServerLogs || !serverDetection?.has_backend) {
      return
    }

    void fetchServerLogs()
    if (!serverStatus?.running) {
      return
    }

    const interval = setInterval(fetchServerLogs, 2000)
    return () => clearInterval(interval)
  }, [fetchServerLogs, serverDetection?.has_backend, serverStatus?.running, showServerLogs])

  return {
    serverStatus,
    setServerStatus,
    serverDetection,
    serverLoading,
    serverLogs,
    showServerLogs,
    setShowServerLogs,
    fetchServerStatus,
    startServer,
    stopServer,
    fetchServerLogs,
    toggleServerLogs,
  }
}

export default usePreviewServer
