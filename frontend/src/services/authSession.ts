const LEGACY_ACCESS_TOKEN_STORAGE_KEY = 'apex_access_token'
const LEGACY_REFRESH_TOKEN_STORAGE_KEY = 'apex_refresh_token'
const TOKEN_EXPIRY_STORAGE_KEY = 'apex_token_expires'

type SessionMetadata = {
  access_token_expires_at: string
  refresh_token_expires_at?: string
  token_type?: string
  session_strategy?: string
}

const getStorage = (): Storage | null => {
  if (typeof localStorage === 'undefined') {
    return null
  }

  return typeof localStorage.getItem === 'function'
    && typeof localStorage.setItem === 'function'
    && typeof localStorage.removeItem === 'function'
    ? localStorage
    : null
}

const readStoredValue = (key: string): string | null => {
  const storage = getStorage()
  if (!storage) {
    return null
  }

  const value = storage.getItem(key)
  const trimmed = typeof value === 'string' ? value.trim() : ''
  return trimmed || null
}

const clearStoredValue = (key: string): void => {
  const storage = getStorage()
  if (!storage) {
    return
  }
  storage.removeItem(key)
}

const asRecord = (value: unknown): Record<string, unknown> | null => {
  if (!value || typeof value !== 'object') {
    return null
  }
  return value as Record<string, unknown>
}

const isSessionMetadata = (value: unknown): value is SessionMetadata => {
  const record = asRecord(value)
  return !!record && typeof record.access_token_expires_at === 'string'
}

const stripTokenQueryParam = (rawUrl: string): string => {
  if (!rawUrl) {
    return rawUrl
  }

  try {
    const baseUrl = typeof window !== 'undefined' ? window.location.origin : 'http://localhost'
    const url = new URL(rawUrl, baseUrl)
    url.searchParams.delete('token')

    if (/^wss?:\/\//i.test(rawUrl)) {
      return url.toString()
    }

    return `${url.pathname}${url.search}${url.hash}`
  } catch {
    return rawUrl
      .replace(/([?&])token=[^&#]*(&)?/g, (_match, prefix: string, trailingAmp: string | undefined) => {
        return trailingAmp ? prefix : ''
      })
      .replace(/\?&/, '?')
      .replace(/[?&]$/, '')
  }
}

export const clearStoredAuthTokens = (): void => {
  clearStoredValue(LEGACY_ACCESS_TOKEN_STORAGE_KEY)
  clearStoredValue(LEGACY_REFRESH_TOKEN_STORAGE_KEY)
  clearStoredValue(TOKEN_EXPIRY_STORAGE_KEY)
}

export const clearLegacyReadableAuthTokens = (): void => {
  clearStoredValue(LEGACY_ACCESS_TOKEN_STORAGE_KEY)
  clearStoredValue(LEGACY_REFRESH_TOKEN_STORAGE_KEY)
}

export const getStoredSessionExpiry = (): string | null => {
  return readStoredValue(TOKEN_EXPIRY_STORAGE_KEY)
}

export const markCookieSessionRefreshed = (
  expiresAt: string = new Date(Date.now() + 15 * 60 * 1000).toISOString()
): void => {
  clearLegacyReadableAuthTokens()

  const storage = getStorage()
  if (!storage) {
    return
  }

  storage.setItem(TOKEN_EXPIRY_STORAGE_KEY, expiresAt)
}

export const extractSessionMetadata = (value: unknown): SessionMetadata | null => {
  if (isSessionMetadata(value)) {
    return value
  }

  const record = asRecord(value)
  if (!record) {
    return null
  }

  if (isSessionMetadata(record.tokens)) {
    return record.tokens
  }
  if (isSessionMetadata(record.data)) {
    return record.data
  }

  return null
}

export const buildAuthenticatedWebSocketUrl = (rawUrl: string): string => {
  return stripTokenQueryParam(rawUrl)
}
