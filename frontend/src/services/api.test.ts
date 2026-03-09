/* @vitest-environment jsdom */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiService, isAuthRefreshRequestUrl, reloadExpiredSession } from './api'

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

beforeEach(() => {
  vi.stubGlobal('localStorage', createStorageMock())
})

afterEach(() => {
  localStorage.clear()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('isAuthRefreshRequestUrl', () => {
  it('matches both current and legacy refresh endpoints', () => {
    expect(isAuthRefreshRequestUrl('/auth/refresh')).toBe(true)
    expect(isAuthRefreshRequestUrl('/auth/token/refresh')).toBe(true)
    expect(isAuthRefreshRequestUrl('/api/v1/auth/refresh?x=1')).toBe(true)
  })

  it('does not match non-refresh auth endpoints', () => {
    expect(isAuthRefreshRequestUrl('/auth/login')).toBe(false)
    expect(isAuthRefreshRequestUrl('/auth/logout')).toBe(false)
    expect(isAuthRefreshRequestUrl('/projects')).toBe(false)
    expect(isAuthRefreshRequestUrl()).toBe(false)
  })
})

describe('reloadExpiredSession', () => {
  it('reloads the current app instead of navigating to a missing login route', () => {
    const reload = vi.fn()

    reloadExpiredSession({ reload })

    expect(reload).toHaveBeenCalledTimes(1)
  })
})

describe('logout', () => {
  it('sends the refresh token so the backend can revoke it', async () => {
    const service = new ApiService('/api/v1')
    localStorage.setItem('apex_refresh_token', 'refresh-token-value')
    const post = vi.spyOn(service.client, 'post').mockResolvedValue({} as any)

    await service.logout()

    expect(post).toHaveBeenCalledWith('/auth/logout', { refresh_token: 'refresh-token-value' })
    expect(localStorage.getItem('apex_refresh_token')).toBeNull()
  })
})

describe('getDeploymentLogsWebSocketUrl', () => {
  it('uses the backend deployment websocket route', () => {
    const service = new ApiService('/api/v1')

    expect(service.getDeploymentLogsWebSocketUrl('deploy-123')).toBe(`ws://${window.location.host}/ws/deploy/deploy-123`)
  })
})
