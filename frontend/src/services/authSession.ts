import { TokenResponse } from '@/types'

const ACCESS_TOKEN_STORAGE_KEY = 'apex_access_token'
const REFRESH_TOKEN_STORAGE_KEY = 'apex_refresh_token'
const TOKEN_EXPIRY_STORAGE_KEY = 'apex_token_expires'

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

const isTokenResponse = (value: unknown): value is TokenResponse => {
  if (!value || typeof value !== 'object') {
    return false
  }

  const tokenRecord = value as Record<string, unknown>
  return typeof tokenRecord.access_token === 'string'
    && typeof tokenRecord.refresh_token === 'string'
    && typeof tokenRecord.access_token_expires_at === 'string'
    && typeof tokenRecord.token_type === 'string'
}

const clearStoredValue = (key: string): void => {
  const storage = getStorage()
  if (!storage) {
    return
  }
  storage.removeItem(key)
}

export const clearStoredAuthTokens = (): void => {
  clearStoredValue(ACCESS_TOKEN_STORAGE_KEY)
  clearStoredValue(REFRESH_TOKEN_STORAGE_KEY)
  clearStoredValue(TOKEN_EXPIRY_STORAGE_KEY)
}

export const getStoredAccessToken = (): string | null => {
  return readStoredValue(ACCESS_TOKEN_STORAGE_KEY)
}

export const getStoredRefreshToken = (): string | null => {
  return readStoredValue(REFRESH_TOKEN_STORAGE_KEY)
}

export const storeAuthTokens = (tokens: TokenResponse): void => {
  const storage = getStorage()
  if (!storage) {
    return
  }

  storage.setItem(ACCESS_TOKEN_STORAGE_KEY, tokens.access_token)
  storage.setItem(REFRESH_TOKEN_STORAGE_KEY, tokens.refresh_token)
  storage.setItem(TOKEN_EXPIRY_STORAGE_KEY, tokens.access_token_expires_at)
}

export const markCookieSessionRefreshed = (
  expiresAt: string = new Date(Date.now() + 15 * 60 * 1000).toISOString()
): void => {
  clearStoredAuthTokens()

  const storage = getStorage()
  if (!storage) {
    return
  }

  storage.setItem(TOKEN_EXPIRY_STORAGE_KEY, expiresAt)
}

export const extractTokenResponse = (value: unknown): TokenResponse | null => {
  if (isTokenResponse(value)) {
    return value
  }

  if (!value || typeof value !== 'object') {
    return null
  }

  const tokenRecord = value as Record<string, unknown>
  if (isTokenResponse(tokenRecord.tokens)) {
    return tokenRecord.tokens
  }
  if (isTokenResponse(tokenRecord.data)) {
    return tokenRecord.data
  }

  return null
}

export const appendStoredAccessTokenToWebSocketUrl = (rawUrl: string): string => {
  const accessToken = getStoredAccessToken()
  if (!accessToken || !rawUrl) {
    return rawUrl
  }

  try {
    const url = new URL(rawUrl)
    if (url.searchParams.has('token')) {
      return url.toString()
    }
    url.searchParams.set('token', accessToken)
    return url.toString()
  } catch {
    if (/([?&])token=/.test(rawUrl)) {
      return rawUrl
    }
    const separator = rawUrl.includes('?') ? '&' : '?'
    return `${rawUrl}${separator}token=${encodeURIComponent(accessToken)}`
  }
}
