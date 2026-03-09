/* @vitest-environment jsdom */

import { afterEach, describe, expect, it, vi } from 'vitest'

import { getConfiguredApiUrl, getConfiguredWsUrl } from './runtime'

describe('runtime config', () => {
  afterEach(() => {
    window.__APEX_CONFIG__ = undefined
    vi.unstubAllEnvs()
  })

  it('prefers runtime API and WebSocket values over build-time env vars', () => {
    vi.stubEnv('VITE_API_URL', 'https://build.example/api/v1')
    vi.stubEnv('VITE_WS_URL', 'wss://build.example/ws')
    window.__APEX_CONFIG__ = {
      API_URL: 'https://runtime.example/api/v1',
      WS_URL: 'wss://runtime.example/ws',
    }

    expect(getConfiguredApiUrl()).toBe('https://runtime.example/api/v1')
    expect(getConfiguredWsUrl()).toBe('wss://runtime.example/ws')
  })

  it('falls back to build-time env vars when runtime config is unset', () => {
    vi.stubEnv('VITE_API_URL', 'https://build.example/api/v1')
    vi.stubEnv('VITE_WS_URL', 'wss://build.example/ws')

    expect(getConfiguredApiUrl()).toBe('https://build.example/api/v1')
    expect(getConfiguredWsUrl()).toBe('wss://build.example/ws')
  })
})
