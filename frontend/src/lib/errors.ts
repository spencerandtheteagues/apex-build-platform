// Human-readable messages for API error responses.
// Checks the server's response body first, then falls back to status-code messages.

const STATUS_MESSAGES: Record<number, string> = {
  400: 'Something looks off with your request. Check your details and try again.',
  401: 'Incorrect email or password.',
  403: 'You don\'t have permission to do that.',
  404: 'Not found.',
  409: 'An account with that email or username already exists. Try logging in instead.',
  410: 'This link has expired.',
  422: 'Some of the information you provided isn\'t valid.',
  429: 'Too many attempts. Please wait a moment and try again.',
  500: 'Something went wrong on our end. Please try again in a moment.',
  502: 'The server is temporarily unavailable. Please try again shortly.',
  503: 'We\'re under maintenance. Please check back soon.',
}

interface AxiosLikeError {
  response?: {
    status?: number
    data?: { error?: string; message?: string; details?: string } | string
  }
  request?: unknown
  code?: string
  isAxiosError?: boolean
  config?: {
    baseURL?: string
    url?: string
  }
  message?: string
}

function isAxiosLike(err: unknown): err is AxiosLikeError {
  if (typeof err !== 'object' || err === null) return false

  return (
    'response' in err ||
    'request' in err ||
    'isAxiosError' in err ||
    'code' in err ||
    'config' in err
  )
}

function joinApiTarget(baseURL?: string, url?: string): string {
  if (!baseURL && !url) return 'the Apex API'
  if (!baseURL) return url ?? 'the Apex API'
  if (!url) return baseURL

  const base = baseURL.endsWith('/') ? baseURL.slice(0, -1) : baseURL
  const path = url.startsWith('/') ? url : `/${url}`
  return `${base}${path}`
}

function isLocalProxyTarget(baseURL?: string): boolean {
  return !baseURL || baseURL.startsWith('/')
}

function getTransportErrorMessage(err: AxiosLikeError): string | null {
  if (err.response) return null

  const hasTransportFailure =
    err.isAxiosError === true ||
    err.code === 'ERR_NETWORK' ||
    err.code === 'ECONNABORTED' ||
    err.message === 'Network Error' ||
    'request' in err

  if (!hasTransportFailure) return null

  const target = joinApiTarget(err.config?.baseURL, err.config?.url)

  if (err.code === 'ECONNABORTED') {
    return `The request to ${target} timed out. Check the backend health and try again.`
  }

  if (isLocalProxyTarget(err.config?.baseURL)) {
    return `Cannot reach the Apex API at ${target}. In local dev, start the backend on http://localhost:8080 and make sure the Vite proxy is running.`
  }

  return `Cannot reach the Apex API at ${target}. Check the configured API URL or backend health.`
}

function getLocalProxyStatusMessage(status: number | undefined, err: AxiosLikeError): string | null {
  if (!status || status < 500 || !isLocalProxyTarget(err.config?.baseURL)) {
    return null
  }

  const target = joinApiTarget(err.config?.baseURL, err.config?.url)
  return `The local Apex API returned HTTP ${status} for ${target}. Start or check the backend on http://localhost:8080, then retry.`
}

export function getApiErrorMessage(err: unknown, fallback = 'Something went wrong. Please try again.'): string {
  if (isAxiosLike(err)) {
    const transportMessage = getTransportErrorMessage(err)
    if (transportMessage) return transportMessage

    const status = err.response?.status
    const data = err.response?.data

    // Prefer the server's own error message if it's human-readable
    const serverMsg = typeof data === 'object' && data !== null
      ? data.error ?? data.message ?? data.details
      : undefined
    if (serverMsg && typeof serverMsg === 'string' && serverMsg.length > 0) {
      // Don't surface raw internal messages for 5xx — use friendly fallback
      if (status && status >= 500) {
        return STATUS_MESSAGES[status] ?? fallback
      }
      // Capitalise first letter for consistency
      return serverMsg.charAt(0).toUpperCase() + serverMsg.slice(1)
    }

    const localProxyStatusMessage = getLocalProxyStatusMessage(status, err)
    if (localProxyStatusMessage) return localProxyStatusMessage

    if (status && STATUS_MESSAGES[status]) {
      return STATUS_MESSAGES[status]
    }
  }

  if (err instanceof Error && err.message && !err.message.includes('status code')) {
    return err.message
  }

  return fallback
}
