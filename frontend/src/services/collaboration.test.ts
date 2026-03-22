/* @vitest-environment jsdom */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { getCollaborationWebSocketUrl } from './collaboration'

const createStorageMock = (): Storage => {
  const store = new Map<string, string>()

  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => {
      store.set(key, String(value))
    },
    removeItem: (key: string) => {
      store.delete(key)
    },
    clear: () => {
      store.clear()
    },
    key: (index: number) => Array.from(store.keys())[index] ?? null,
    get length() {
      return store.size
    },
  } as Storage
}

describe('getCollaborationWebSocketUrl', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', createStorageMock())
  })

  afterEach(() => {
    localStorage.clear()
    window.__APEX_CONFIG__ = undefined
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
  })

  it('prefers runtime WebSocket config over build-time env', () => {
    vi.stubEnv('VITE_WS_URL', 'wss://build.example/ws')
    window.__APEX_CONFIG__ = {
      WS_URL: 'wss://runtime.example/ws',
    }

    expect(getCollaborationWebSocketUrl()).toBe('wss://runtime.example/ws/collab')
  })

  it('falls back to the current host when no runtime or build-time config is present', () => {
    expect(getCollaborationWebSocketUrl()).toBe('ws://localhost:3000/ws/collab')
  })

  it('appends the stored access token for websocket auth', () => {
    localStorage.setItem('apex_access_token', 'access-token-value')

    expect(getCollaborationWebSocketUrl()).toBe('ws://localhost:3000/ws/collab?token=access-token-value')
  })
})
