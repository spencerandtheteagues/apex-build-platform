type RuntimeConfig = {
  API_URL?: string
  WS_URL?: string
  VERSION?: string
  ENVIRONMENT?: string
  FEATURES?: Record<string, unknown>
}

const readRuntimeConfig = (): RuntimeConfig => {
  if (typeof window === 'undefined' || !window.__APEX_CONFIG__) {
    return {}
  }
  return window.__APEX_CONFIG__
}

const normalizeConfigValue = (value?: string): string => {
  return typeof value === 'string' ? value.trim() : ''
}

export const getConfiguredApiUrl = (): string => {
  const runtimeValue = normalizeConfigValue(readRuntimeConfig().API_URL)
  if (runtimeValue) {
    return runtimeValue
  }

  return normalizeConfigValue(import.meta.env.VITE_API_URL)
}

export const getConfiguredWsUrl = (): string => {
  const runtimeValue = normalizeConfigValue(readRuntimeConfig().WS_URL)
  if (runtimeValue) {
    return runtimeValue
  }

  return normalizeConfigValue(import.meta.env.VITE_WS_URL)
}
