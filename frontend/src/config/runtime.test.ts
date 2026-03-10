import { describe, expect, it } from 'vitest'

import { normalizeConfiguredApiUrl, normalizeConfiguredWsUrl } from './runtime'

describe('normalizeConfiguredApiUrl', () => {
  it('rewrites the broken production API custom domain to the live Render backend', () => {
    expect(normalizeConfiguredApiUrl('https://api.apex.build')).toBe(
      'https://apex-backend-5ypy.onrender.com/api/v1'
    )
    expect(normalizeConfiguredApiUrl('https://api.apex.build/api/v1')).toBe(
      'https://apex-backend-5ypy.onrender.com/api/v1'
    )
  })

  it('appends /api/v1 when a bare origin is provided', () => {
    expect(normalizeConfiguredApiUrl('https://example.com')).toBe('https://example.com/api/v1')
  })
})

describe('normalizeConfiguredWsUrl', () => {
  it('rewrites the broken production websocket custom domain to the live Render backend', () => {
    expect(normalizeConfiguredWsUrl('wss://api.apex.build')).toBe(
      'wss://apex-backend-5ypy.onrender.com/ws'
    )
  })

  it('appends /ws when a bare websocket origin is provided', () => {
    expect(normalizeConfiguredWsUrl('wss://example.com')).toBe('wss://example.com/ws')
  })
})
