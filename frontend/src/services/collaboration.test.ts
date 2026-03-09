/* @vitest-environment jsdom */

import { afterEach, describe, expect, it, vi } from 'vitest'

import { getCollaborationWebSocketUrl } from './collaboration'

describe('getCollaborationWebSocketUrl', () => {
  afterEach(() => {
    window.__APEX_CONFIG__ = undefined
    vi.unstubAllEnvs()
  })

  it('prefers runtime WebSocket config over build-time env', () => {
    vi.stubEnv('VITE_WS_URL', 'wss://build.example/ws')
    window.__APEX_CONFIG__ = {
      WS_URL: 'wss://runtime.example/ws',
    }

    expect(getCollaborationWebSocketUrl('abc123')).toBe('wss://runtime.example/ws/collab?token=abc123')
  })

  it('falls back to the current host when no runtime or build-time config is present', () => {
    expect(getCollaborationWebSocketUrl('abc123')).toBe('ws://localhost:3000/ws/collab?token=abc123')
  })
})
