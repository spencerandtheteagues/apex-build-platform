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
  it('logs out via cookie auth and clears legacy token storage', async () => {
    const service = new ApiService('/api/v1')
    localStorage.setItem('apex_access_token', 'access-token-value')
    localStorage.setItem('apex_refresh_token', 'refresh-token-value')
    localStorage.setItem('apex_token_expires', '2026-03-21T21:54:04.333589145Z')
    const post = vi.spyOn(service.client, 'post').mockResolvedValue({} as any)

    await service.logout()

    expect(post).toHaveBeenCalledWith('/auth/logout', {})
    expect(localStorage.getItem('apex_access_token')).toBeNull()
    expect(localStorage.getItem('apex_refresh_token')).toBeNull()
    expect(localStorage.getItem('apex_token_expires')).toBeNull()
  })
})

describe('cookie session compatibility', () => {
  it('clears legacy readable tokens on service initialization', () => {
    localStorage.setItem('apex_access_token', 'access-token-value')
    localStorage.setItem('apex_refresh_token', 'refresh-token-value')
    localStorage.setItem('apex_token_expires', '2099-03-21T21:54:04.333589145Z')

    const service = new ApiService('/api/v1')

    expect(service.isAuthenticated()).toBe(true)
    expect(localStorage.getItem('apex_access_token')).toBeNull()
    expect(localStorage.getItem('apex_refresh_token')).toBeNull()
    expect(localStorage.getItem('apex_token_expires')).toBe('2099-03-21T21:54:04.333589145Z')
  })

  it('stores only a non-secret session expiry marker after login', async () => {
    const service = new ApiService('/api/v1')
    vi.spyOn(service.client, 'post').mockResolvedValue({
      data: {
        message: 'Login successful',
        user: { username: 'verify-user' },
        access_token_expires_at: '2026-03-21T21:54:04.333589145Z',
        refresh_token_expires_at: '2026-03-28T21:39:04.333589145Z',
        token_type: 'Bearer',
        session_strategy: 'cookie',
      },
    } as any)

    await service.login({ username: 'verify-user', password: 'CodexCheck!123' })

    expect(localStorage.getItem('apex_access_token')).toBeNull()
    expect(localStorage.getItem('apex_refresh_token')).toBeNull()
    expect(localStorage.getItem('apex_token_expires')).toBe('2026-03-21T21:54:04.333589145Z')
  })

  it('refreshes via cookie auth and keeps readable tokens out of storage', async () => {
    const service = new ApiService('/api/v1')
    localStorage.setItem('apex_refresh_token', 'refresh-token-value')
    const post = vi.spyOn(service.client, 'post').mockResolvedValue({
      data: {
        message: 'Tokens refreshed successfully',
        access_token_expires_at: '2026-03-21T22:00:00.000Z',
        refresh_token_expires_at: '2026-03-28T22:00:00.000Z',
        token_type: 'Bearer',
        session_strategy: 'cookie',
      },
    } as any)

    const result = await service.refreshToken()

    expect(post).toHaveBeenCalledWith('/auth/refresh', {})
    expect(result.access_token).toBe('')
    expect(result.access_token_expires_at).toBe('2026-03-21T22:00:00.000Z')
    expect(localStorage.getItem('apex_access_token')).toBeNull()
    expect(localStorage.getItem('apex_refresh_token')).toBeNull()
    expect(localStorage.getItem('apex_token_expires')).toBe('2026-03-21T22:00:00.000Z')
  })

  it('does not add readable bearer tokens to outbound request headers', async () => {
    const service = new ApiService('/api/v1')
    localStorage.setItem('apex_access_token', 'access-token-value')

    const interceptor = (service.client.interceptors.request as any).handlers[0].fulfilled
    const config = await interceptor({ headers: {} })

    expect(config.headers.Authorization).toBeUndefined()
  })

  it('does not try to refresh the session when login itself returns 401', async () => {
    const service = new ApiService('/api/v1')
    const refreshSpy = vi.spyOn(service, 'refreshToken')
    const responseRejected = (service.client.interceptors.response as any).handlers[0].rejected
    const loginError = {
      config: { url: '/auth/login' },
      response: {
        status: 401,
        data: { error: 'Invalid credentials' },
      },
    }

    await expect(responseRejected(loginError)).rejects.toBe(loginError)
    expect(refreshSpy).not.toHaveBeenCalled()
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

  it('does not append a readable token to websocket URLs', () => {
    localStorage.setItem('apex_access_token', 'access-token-value')
    const service = new ApiService('/api/v1')

    expect(service.getDeploymentLogsWebSocketUrl('deploy-123')).toBe(`ws://${window.location.host}/ws/deploy/deploy-123`)
    expect(service.getDebugWebSocketUrl('debug-123')).toBe(`ws://${window.location.host}/ws/debug/debug-123`)
    expect(service.getTerminalWebSocketUrl('term-123')).toBe(`ws://${window.location.host}/ws/terminal/term-123`)
  })
})

describe('external deployments', () => {
  it('fetches the provider catalog from the deploy API', async () => {
    const service = new ApiService('/api/v1')
    const get = vi.spyOn(service.client, 'get').mockResolvedValue({
      data: {
        providers: [
          { id: 'railway', name: 'Railway', description: 'Full-stack hosting', features: ['Domains'] },
          { id: 'cloudflare_pages', name: 'Cloudflare Pages', description: 'Edge static hosting', features: ['CDN'] },
        ],
      },
    } as any)

    const providers = await service.getExternalDeploymentProviders()

    expect(get).toHaveBeenCalledWith('/deploy/providers')
    expect(providers.map((provider) => provider.id)).toEqual(['railway', 'cloudflare_pages'])
  })

  it('posts external deployment config including node version, root directory, and Neon database orchestration', async () => {
    const service = new ApiService('/api/v1')
    const post = vi.spyOn(service.client, 'post').mockResolvedValue({
      data: {
        success: true,
        deployment: { id: 'dep-123' },
        message: 'Deployment started',
      },
    } as any)

    await service.startExternalDeployment({
      project_id: 42,
      provider: 'railway',
      framework: 'react',
      node_version: '20',
      root_directory: 'apps/web',
      build_command: 'npm run build',
      start_command: 'npm start',
      database: {
        provider: 'neon',
        project_name: 'apex-db',
        database_name: 'app',
        role_name: 'app_owner',
        pooled: true,
      },
    })

    expect(post).toHaveBeenCalledWith('/deploy', {
      project_id: 42,
      provider: 'railway',
      framework: 'react',
      node_version: '20',
      root_directory: 'apps/web',
      build_command: 'npm run build',
      start_command: 'npm start',
      database: {
        provider: 'neon',
        project_name: 'apex-db',
        database_name: 'app',
        role_name: 'app_owner',
        pooled: true,
      },
    })
  })

  it('reads native deployment logs from the top-level envelope', async () => {
    const service = new ApiService('/api/v1')
    const get = vi.spyOn(service.client, 'get').mockResolvedValue({
      data: {
        success: true,
        logs: [
          { id: 1, deployment_id: 'native-1', timestamp: '2026-01-01T00:00:00Z', level: 'info', source: 'deploy', message: 'ready' },
        ],
      },
    } as any)

    const logs = await service.getDeploymentLogs(22, 'native-1')

    expect(get).toHaveBeenCalledWith('/projects/22/deployments/native-1/logs?limit=100')
    expect(logs).toHaveLength(1)
    expect(logs[0].message).toBe('ready')
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
