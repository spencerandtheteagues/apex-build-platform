type RuntimeConfig = {
  API_URL?: string
  WS_URL?: string
  VERSION?: string
  ENVIRONMENT?: string
  FEATURES?: Record<string, unknown>
}

const FALLBACK_RENDER_API_URL = 'https://apex-backend-5ypy.onrender.com/api/v1'
const FALLBACK_RENDER_WS_URL = 'wss://apex-backend-5ypy.onrender.com/ws'
const BROKEN_PRODUCTION_API_HOSTS = new Set(['api.apex.build'])
const BROKEN_PRODUCTION_WS_HOSTS = new Set(['api.apex.build'])

const readRuntimeConfig = (): RuntimeConfig => {
  if (typeof window === 'undefined' || !window.__APEX_CONFIG__) {
    return {}
  }
  return window.__APEX_CONFIG__
}

const normalizeConfigValue = (value?: string): string => {
  return typeof value === 'string' ? value.trim() : ''
}

export const normalizeConfiguredApiUrl = (value?: string): string => {
  const normalized = normalizeConfigValue(value)
  if (!normalized) {
    return ''
  }

  try {
    const parsed = new URL(normalized)
    if (BROKEN_PRODUCTION_API_HOSTS.has(parsed.host)) {
      return FALLBACK_RENDER_API_URL
    }
    if (!parsed.pathname || parsed.pathname === '/') {
      parsed.pathname = '/api/v1'
    }
    return parsed.toString().replace(/\/$/, '')
  } catch {
    return normalized
  }
}

export const normalizeConfiguredWsUrl = (value?: string): string => {
  const normalized = normalizeConfigValue(value)
  if (!normalized) {
    return ''
  }

  try {
    const parsed = new URL(normalized)
    if (BROKEN_PRODUCTION_WS_HOSTS.has(parsed.host)) {
      return FALLBACK_RENDER_WS_URL
    }
    if (!parsed.pathname || parsed.pathname === '/') {
      parsed.pathname = '/ws'
    }
    return parsed.toString().replace(/\/$/, '')
  } catch {
    return normalized
  }
}

const getImportedApiUrl = (): string | undefined => {
  return import.meta.env.VITE_API_URL || import.meta.env.VITE_API_BASE_URL
}

export const getConfiguredApiUrl = (): string => {
  const runtimeValue = normalizeConfiguredApiUrl(readRuntimeConfig().API_URL)
  if (runtimeValue) {
    return runtimeValue
  }

  return normalizeConfiguredApiUrl(getImportedApiUrl())
}

export const getConfiguredWsUrl = (): string => {
  const runtimeValue = normalizeConfiguredWsUrl(readRuntimeConfig().WS_URL)
  if (runtimeValue) {
    return runtimeValue
  }

  return normalizeConfiguredWsUrl(import.meta.env.VITE_WS_URL)
}
