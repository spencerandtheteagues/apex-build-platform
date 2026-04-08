import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  getConfiguredApiUrl,
  getConfiguredWsUrl,
  normalizeConfiguredApiUrl,
  normalizeConfiguredWsUrl,
} from './runtime'

afterEach(() => {
  vi.unstubAllEnvs()
  window.__APEX_CONFIG__ = undefined
})

describe('normalizeConfiguredApiUrl', () => {
  it('rewrites the broken production API custom domain to the active custom domain', () => {
    expect(normalizeConfiguredApiUrl('https://api.apex.build')).toBe(
      'https://api.apex-build.dev/api/v1'
    )
    expect(normalizeConfiguredApiUrl('https://api.apex.build/api/v1')).toBe(
      'https://api.apex-build.dev/api/v1'
    )
  })

  it('appends /api/v1 when a bare origin is provided', () => {
    expect(normalizeConfiguredApiUrl('https://example.com')).toBe('https://example.com/api/v1')
  })
})

describe('normalizeConfiguredWsUrl', () => {
  it('rewrites the broken production websocket custom domain to the active custom domain', () => {
    expect(normalizeConfiguredWsUrl('wss://api.apex.build')).toBe(
      'wss://api.apex-build.dev/ws'
    )
  })

  it('appends /ws when a bare websocket origin is provided', () => {
    expect(normalizeConfiguredWsUrl('wss://example.com')).toBe('wss://example.com/ws')
  })
})

describe('getConfiguredApiUrl', () => {
  it('falls back to VITE_API_BASE_URL when VITE_API_URL is unset', () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://legacy.example.com')

    expect(getConfiguredApiUrl()).toBe('https://legacy.example.com/api/v1')
  })

  it('prefers VITE_API_URL over the legacy alias when both are set', () => {
    vi.stubEnv('VITE_API_URL', 'https://primary.example.com')
    vi.stubEnv('VITE_API_BASE_URL', 'https://legacy.example.com')

    expect(getConfiguredApiUrl()).toBe('https://primary.example.com/api/v1')
  })

  it('prefers the imported local dev API target over runtime production config on localhost', () => {
    vi.stubEnv('VITE_API_URL', 'http://127.0.0.1:8080/api/v1')
    window.__APEX_CONFIG__ = {
      API_URL: 'https://api.apex-build.dev/api/v1',
    }

    expect(getConfiguredApiUrl()).toBe('http://127.0.0.1:8080/api/v1')
  })
})

describe('getConfiguredWsUrl', () => {
  it('prefers the imported local dev websocket target over runtime production config on localhost', () => {
    vi.stubEnv('VITE_WS_URL', 'ws://127.0.0.1:8080/ws')
    window.__APEX_CONFIG__ = {
      WS_URL: 'wss://api.apex-build.dev/ws',
    }

    expect(getConfiguredWsUrl()).toBe('ws://127.0.0.1:8080/ws')
  })
})
