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
  window.__APEX_CONFIG__ = undefined
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
  it('posts the stored refresh token to logout and clears legacy token storage', async () => {
    const service = new ApiService('/api/v1')
    localStorage.setItem('apex_refresh_token', 'refresh-token-value')
    const post = vi.spyOn(service.client, 'post').mockResolvedValue({} as any)

    await service.logout()

    expect(post).toHaveBeenCalledWith('/auth/logout', { refresh_token: 'refresh-token-value' })
    expect(localStorage.getItem('apex_refresh_token')).toBeNull()
  })
})

describe('auth token compatibility', () => {
  it('stores bearer tokens returned by login responses', async () => {
    const service = new ApiService('/api/v1')
    vi.spyOn(service.client, 'post').mockResolvedValue({
      data: {
        message: 'Login successful',
        user: { username: 'verify-user' },
        tokens: {
          access_token: 'access-token-value',
          refresh_token: 'refresh-token-value',
          access_token_expires_at: '2026-03-21T21:54:04.333589145Z',
          refresh_token_expires_at: '2026-03-28T21:39:04.333589145Z',
          token_type: 'Bearer',
        },
      },
    } as any)

    await service.login({ username: 'verify-user', password: 'CodexCheck!123' })

    expect(localStorage.getItem('apex_access_token')).toBe('access-token-value')
    expect(localStorage.getItem('apex_refresh_token')).toBe('refresh-token-value')
  })

  it('refreshes with the stored refresh token and persists rotated tokens', async () => {
    const service = new ApiService('/api/v1')
    localStorage.setItem('apex_refresh_token', 'refresh-token-value')
    const post = vi.spyOn(service.client, 'post').mockResolvedValue({
      data: {
        tokens: {
          access_token: 'next-access-token',
          refresh_token: 'next-refresh-token',
          access_token_expires_at: '2026-03-21T22:00:00.000Z',
          refresh_token_expires_at: '2026-03-28T22:00:00.000Z',
          token_type: 'Bearer',
        },
      },
    } as any)

    const result = await service.refreshToken()

    expect(post).toHaveBeenCalledWith('/auth/refresh', { refresh_token: 'refresh-token-value' })
    expect(result.access_token).toBe('next-access-token')
    expect(localStorage.getItem('apex_access_token')).toBe('next-access-token')
    expect(localStorage.getItem('apex_refresh_token')).toBe('next-refresh-token')
  })

  it('adds the stored access token to outbound request headers', async () => {
    const service = new ApiService('/api/v1')
    localStorage.setItem('apex_access_token', 'access-token-value')

    const interceptor = (service.client.interceptors.request as any).handlers[0].fulfilled
    const config = await interceptor({ headers: {} })

    expect(config.headers.Authorization).toBe('Bearer access-token-value')
  })
})

describe('getDeploymentLogsWebSocketUrl', () => {
  it('uses the backend deployment websocket route', () => {
    const service = new ApiService('/api/v1')

    expect(service.getDeploymentLogsWebSocketUrl('deploy-123')).toBe(`ws://${window.location.host}/ws/deploy/deploy-123`)
  })

  it('prefers runtime websocket config for websocket routes', () => {
    window.__APEX_CONFIG__ = {
      WS_URL: 'wss://runtime.example/ws',
    }

    const service = new ApiService('/api/v1')

    expect(service.getDeploymentLogsWebSocketUrl('deploy-123')).toBe('wss://runtime.example/ws/deploy/deploy-123')
    expect(service.getDebugWebSocketUrl('debug-123')).toBe('wss://runtime.example/ws/debug/debug-123')
    expect(service.getTerminalWebSocketUrl('term-123')).toBe('wss://runtime.example/ws/terminal/term-123')
  })

  it('appends the stored access token to websocket URLs', () => {
    localStorage.setItem('apex_access_token', 'access-token-value')
    const service = new ApiService('/api/v1')

    expect(service.getDeploymentLogsWebSocketUrl('deploy-123')).toBe(
      `ws://${window.location.host}/ws/deploy/deploy-123?token=access-token-value`
    )
    expect(service.getDebugWebSocketUrl('debug-123')).toBe(
      `ws://${window.location.host}/ws/debug/debug-123?token=access-token-value`
    )
    expect(service.getTerminalWebSocketUrl('term-123')).toBe(
      `ws://${window.location.host}/ws/terminal/term-123?token=access-token-value`
    )
  })
})

describe('sendBuildMessage', () => {
  it('posts targeted routing metadata for planner and agent controls', async () => {
    const service = new ApiService('/api/v1')
    const post = vi.spyOn(service.client, 'post').mockResolvedValue({
      data: {
        interaction: {},
        live: false,
      },
    } as any)

    await service.sendBuildMessage('build-123', 'Refine the layout', {
      clientToken: 'token-123',
      targetMode: 'agent',
      targetAgentId: 'frontend-1',
      targetAgentRole: 'frontend',
    })

    expect(post).toHaveBeenCalledWith('/build/build-123/message', {
      content: 'Refine the layout',
      client_token: 'token-123',
      command: undefined,
      target_mode: 'agent',
      target_agent_id: 'frontend-1',
      target_agent_role: 'frontend',
    })
  })

  it('posts restart commands for failed builds', async () => {
    const service = new ApiService('/api/v1')
    const post = vi.spyOn(service.client, 'post').mockResolvedValue({
      data: {
        interaction: {},
        live: false,
      },
    } as any)

    await service.sendBuildMessage('build-123', 'Restart the failed build', {
      command: 'restart_failed',
      targetMode: 'lead',
    })

    expect(post).toHaveBeenCalledWith('/build/build-123/message', {
      content: 'Restart the failed build',
      client_token: undefined,
      command: 'restart_failed',
      target_mode: 'lead',
      target_agent_id: undefined,
      target_agent_role: undefined,
    })
  })
})
