import { useCallback, useEffect, useMemo, useState } from 'react'
import type { ConsoleEntry } from '@/components/preview/ConsolePanel'
import type { NetworkRequest } from '@/components/preview/NetworkPanel'

const MAX_CONSOLE_ENTRIES = 1000
const MAX_NETWORK_REQUESTS = 500

const isTrustedPreviewOrigin = (origin: string): boolean => {
  const isLocalhost = origin.startsWith('http://localhost:') || origin.startsWith('http://127.0.0.1:')
  const isSameOrigin = origin === window.location.origin
  return isLocalhost || isSameOrigin
}

export function usePreviewDevtools() {
  const [consoleEntries, setConsoleEntries] = useState<ConsoleEntry[]>([])
  const [networkRequests, setNetworkRequests] = useState<NetworkRequest[]>([])

  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      if (!isTrustedPreviewOrigin(event.origin)) {
        return
      }
      if (!event.data || typeof event.data !== 'object' || typeof event.data.type !== 'string') {
        return
      }

      if (event.data.type === 'apex-console') {
        const entry: ConsoleEntry = {
          id: `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
          level: event.data.level,
          message: event.data.message,
          stack: event.data.stack,
          timestamp: event.data.timestamp,
        }

        setConsoleEntries(prev => {
          const next = [...prev, entry]
          if (next.length > MAX_CONSOLE_ENTRIES) {
            return next.slice(-MAX_CONSOLE_ENTRIES)
          }
          return next
        })
        return
      }

      if (event.data.type === 'apex-network') {
        const request: NetworkRequest = {
          id: event.data.id,
          method: event.data.method,
          url: event.data.url,
          status: event.data.status,
          statusText: event.data.statusText,
          duration: event.data.duration,
          error: event.data.error,
          timestamp: event.data.timestamp,
        }

        setNetworkRequests(prev => {
          const next = [...prev, request]
          if (next.length > MAX_NETWORK_REQUESTS) {
            return next.slice(-MAX_NETWORK_REQUESTS)
          }
          return next
        })
      }
    }

    window.addEventListener('message', handleMessage)
    return () => window.removeEventListener('message', handleMessage)
  }, [])

  const clearConsole = useCallback(() => {
    setConsoleEntries([])
  }, [])

  const clearNetwork = useCallback(() => {
    setNetworkRequests([])
  }, [])

  const clearDevTools = useCallback(() => {
    setConsoleEntries([])
    setNetworkRequests([])
  }, [])

  const errorCount = useMemo(() => consoleEntries.filter(entry => entry.level === 'error').length, [consoleEntries])
  const warnCount = useMemo(() => consoleEntries.filter(entry => entry.level === 'warn').length, [consoleEntries])
  const networkErrorCount = useMemo(
    () => networkRequests.filter(request => request.status === 0 || request.status >= 400).length,
    [networkRequests]
  )

  return {
    consoleEntries,
    setConsoleEntries,
    networkRequests,
    setNetworkRequests,
    clearConsole,
    clearNetwork,
    clearDevTools,
    errorCount,
    warnCount,
    networkErrorCount,
  }
}

export default usePreviewDevtools
