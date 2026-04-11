type RuntimeConfig = {
  API_URL?: string
  WS_URL?: string
  VERSION?: string
  ENVIRONMENT?: string
  FEATURES?: Record<string, unknown>
}

const PRIMARY_PRODUCTION_API_URL = 'https://api.apex-build.dev/api/v1'
const PRIMARY_PRODUCTION_WS_URL = 'wss://api.apex-build.dev/ws'
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
      return PRIMARY_PRODUCTION_API_URL
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
      return PRIMARY_PRODUCTION_WS_URL
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

const isLocalDevelopmentHost = (): boolean => {
  if (typeof window === 'undefined') {
    return false
  }

  const host = window.location.hostname.trim().toLowerCase()
  if (!host) {
    return false
  }

  if (host === 'localhost' || host === '::1') {
    return true
  }

  if (host.startsWith('127.') || host.startsWith('10.') || host.startsWith('192.168.')) {
    return true
  }

  const private172Match = /^172\.(1[6-9]|2\d|3[0-1])\./.test(host)
  return private172Match
}

export const getConfiguredApiUrl = (): string => {
  const importedValue = normalizeConfiguredApiUrl(getImportedApiUrl())
  if (isLocalDevelopmentHost() && importedValue) {
    return importedValue
  }

  const runtimeValue = normalizeConfiguredApiUrl(readRuntimeConfig().API_URL)
  if (runtimeValue) {
    return runtimeValue
  }

  return importedValue
}

export const getConfiguredWsUrl = (): string => {
  const importedValue = normalizeConfiguredWsUrl(import.meta.env.VITE_WS_URL)
  if (isLocalDevelopmentHost() && importedValue) {
    return importedValue
  }

  const runtimeValue = normalizeConfiguredWsUrl(readRuntimeConfig().WS_URL)
  if (runtimeValue) {
    return runtimeValue
  }

  return importedValue
}

export const getRuntimeConfiguredWsUrl = (): string => {
  return normalizeConfiguredWsUrl(readRuntimeConfig().WS_URL)
}
