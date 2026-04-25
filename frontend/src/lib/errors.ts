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
    data?: { error?: string; message?: string; details?: string }
  }
  message?: string
}

function isAxiosLike(err: unknown): err is AxiosLikeError {
  return typeof err === 'object' && err !== null && 'response' in err
}

export function getApiErrorMessage(err: unknown, fallback = 'Something went wrong. Please try again.'): string {
  if (isAxiosLike(err)) {
    const status = err.response?.status
    const data = err.response?.data

    // Prefer the server's own error message if it's human-readable
    const serverMsg = data?.error ?? data?.message ?? data?.details
    if (serverMsg && typeof serverMsg === 'string' && serverMsg.length > 0) {
      // Don't surface raw internal messages for 5xx — use friendly fallback
      if (status && status >= 500) {
        return STATUS_MESSAGES[status] ?? fallback
      }
      // Capitalise first letter for consistency
      return serverMsg.charAt(0).toUpperCase() + serverMsg.slice(1)
    }

    if (status && STATUS_MESSAGES[status]) {
      return STATUS_MESSAGES[status]
    }
  }

  if (err instanceof Error && err.message && !err.message.includes('status code')) {
    return err.message
  }

  return fallback
}
