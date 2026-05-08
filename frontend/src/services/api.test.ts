/* @vitest-environment jsdom */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiService, PREVIEW_START_TIMEOUT_MS, isAuthRefreshRequestUrl, reloadExpiredSession } from './api'

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

describe('architecture intelligence API', () => {
  it('fetches the admin architecture map from the admin route', async () => {
    const service = new ApiService('/api/v1')
    const get = vi.spyOn(service.client, 'get').mockResolvedValue({
      data: {
        map: {
          schema_version: '1.0.0',
          generated_at: '2026-05-06T00:00:00Z',
          source: 'deterministic_repo_scanner',
          confidence: 0.74,
          summary: {
            node_count: 1,
            edge_count: 0,
            contract_count: 0,
            file_count: 1,
            test_file_count: 0,
            high_risk_nodes: 1,
            critical_nodes: 0,
          },
          nodes: [],
          edges: [],
          contracts: [],
        },
      },
    } as any)

    const map = await service.getAdminArchitectureMap()

    expect(get).toHaveBeenCalledWith('/admin/architecture/map')
    expect(map.source).toBe('deterministic_repo_scanner')
  })

  it('fetches build architecture reference telemetry without prompt text', async () => {
    const service = new ApiService('/api/v1')
    const get = vi.spyOn(service.client, 'get').mockResolvedValue({
      data: {
        references: {
          total_references: 2,
          by_node: { 'ai.orchestration': 1 },
          by_structure: { BuildSnapshotState: 1 },
        },
      },
    } as any)

    const refs = await service.getBuildArchitectureReferences('build-123')

    expect(get).toHaveBeenCalledWith('/build/build-123/architecture-references')
    expect(refs.by_node?.['ai.orchestration']).toBe(1)
    expect(JSON.stringify(refs)).not.toContain('Inspect backend/internal/agents')
  })
})

describe('mobile validation API', () => {
  it('fetches project mobile validation from the project route', async () => {
    const service = new ApiService('/api/v1')
    const get = vi.spyOn(service.client, 'get').mockResolvedValue({
      data: {
        validation: {
          status: 'passed',
          summary: 'Mobile source package passed validation.',
          checks: [],
        },
      },
    } as any)

    const validation = await service.getProjectMobileValidation(42)

    expect(get).toHaveBeenCalledWith('/projects/42/mobile/validation')
    expect(validation.status).toBe('passed')
  })

  it('fetches project mobile readiness scorecard from the project route', async () => {
    const service = new ApiService('/api/v1')
    const get = vi.spyOn(service.client, 'get').mockResolvedValue({
      data: {
        scorecard: {
          overall_score: 61,
          target_score: 95,
          is_ready: false,
          summary: 'Mobile readiness is 61% toward the 95% launch-readiness target.',
          categories: [
            {
              id: 'credentials_signing',
              label: 'Credentials and signing',
              score: 0,
              target: 95,
              status: 'blocked',
              summary: 'No validated credentials are recorded.',
              blockers: ['Add encrypted mobile credentials.'],
            },
          ],
          blockers: ['Add encrypted mobile credentials.'],
        },
      },
    } as any)

    const scorecard = await service.getProjectMobileScorecard(42)

    expect(get).toHaveBeenCalledWith('/projects/42/mobile/scorecard')
    expect(scorecard.overall_score).toBe(61)
    expect(scorecard.categories[0].id).toBe('credentials_signing')
  })

  it('fetches project mobile store-readiness report from the project route', async () => {
    const service = new ApiService('/api/v1')
    const get = vi.spyOn(service.client, 'get').mockResolvedValue({
      data: {
        store_readiness: {
          status: 'draft_ready_needs_manual_store_assets',
          package_path: 'mobile/store/store-readiness.json',
          validation_status: 'passed',
          score: 75,
          target: 95,
          ready_for_submission: false,
          summary: 'Draft store-readiness package is valid.',
        },
      },
    } as any)

    const report = await service.getProjectMobileStoreReadiness(42)

    expect(get).toHaveBeenCalledWith('/projects/42/mobile/store-readiness')
    expect(report.ready_for_submission).toBe(false)
    expect(report.package_path).toBe('mobile/store/store-readiness.json')
  })

  it('checks project mobile build repairs through the project route', async () => {
    const service = new ApiService('/api/v1')
    const post = vi.spyOn(service.client, 'post').mockResolvedValue({
      data: {
        build: {
          id: 'mbld_1',
          project_id: 42,
          user_id: 1,
          platform: 'android',
          profile: 'preview',
          release_level: 'internal_android_apk',
          status: 'repaired_retry_pending',
          created_at: '2026-05-01T00:00:00Z',
          updated_at: '2026-05-01T00:00:00Z',
        },
        repaired: true,
      },
    } as any)

    const result = await service.repairProjectMobileBuild(42, 'mbld_1')

    expect(post).toHaveBeenCalledWith('/projects/42/mobile/builds/mbld_1/repair', {})
    expect(result.repaired).toBe(true)
    expect(result.build.status).toBe('repaired_retry_pending')
  })

  it('submits a project mobile build through the project route', async () => {
    const service = new ApiService('/api/v1')
    const post = vi.spyOn(service.client, 'post').mockResolvedValue({
      data: {
        submission: {
          id: 'msub_1',
          project_id: 42,
          user_id: 1,
          build_id: 'mbld_1',
          platform: 'android',
          status: 'ready_for_google_internal_testing',
          created_at: '2026-05-01T00:00:00Z',
          updated_at: '2026-05-01T00:00:00Z',
        },
      },
    } as any)

    const result = await service.submitProjectMobileBuild(42, 'mbld_1', { track: 'internal' })

    expect(post).toHaveBeenCalledWith('/projects/42/mobile/builds/mbld_1/submit', { track: 'internal' })
    expect(result.submission.status).toBe('ready_for_google_internal_testing')
  })

  it('manages project mobile credentials without exposing raw values in API helpers', async () => {
    const service = new ApiService('/api/v1')
    const get = vi.spyOn(service.client, 'get').mockResolvedValue({
      data: {
        credentials: {
          status: 'partial',
          complete: false,
          required: ['eas_token', 'apple_app_store_connect'],
          present: ['eas_token'],
          missing: ['apple_app_store_connect'],
          metadata: [{ type: 'eas_token', secret_id: 12, project_id: 42, status: 'stored', label: 'EAS token', created_at: '2026-05-06T00:00:00Z', updated_at: '2026-05-06T00:00:00Z' }],
        },
      },
    } as any)
    const post = vi.spyOn(service.client, 'post').mockResolvedValue({
      data: {
        credentials: {
          status: 'partial',
          complete: false,
          required: ['eas_token'],
          present: ['eas_token'],
          missing: [],
          metadata: [],
        },
      },
    } as any)
    const del = vi.spyOn(service.client, 'delete').mockResolvedValue({
      data: {
        credentials: {
          status: 'missing',
          complete: false,
          required: ['eas_token'],
          present: [],
          missing: ['eas_token'],
          metadata: [],
        },
      },
    } as any)

    const listed = await service.getProjectMobileCredentials(42)
    const stored = await service.createProjectMobileCredential(42, { type: 'eas_token', values: { token: 'raw-token' } })
    const deleted = await service.deleteProjectMobileCredential(42, 'eas_token')

    expect(get).toHaveBeenCalledWith('/projects/42/mobile/credentials')
    expect(post).toHaveBeenCalledWith('/projects/42/mobile/credentials', { type: 'eas_token', values: { token: 'raw-token' } })
    expect(del).toHaveBeenCalledWith('/projects/42/mobile/credentials/eas_token')
    expect(JSON.stringify(listed)).not.toContain('raw-token')
    expect(stored.present).toContain('eas_token')
    expect(deleted.missing).toContain('eas_token')
  })

  it('manages project mobile build jobs through project-scoped routes', async () => {
    const service = new ApiService('/api/v1')
    const build = {
      id: 'mbld_123',
      project_id: 42,
      user_id: 7,
      platform: 'android',
      profile: 'preview',
      release_level: 'internal_android_apk',
      status: 'succeeded',
      provider: 'mock-eas',
      artifact_url: 'https://artifacts.example.com/app.apk',
      logs: [{ timestamp: '2026-05-06T00:00:00Z', level: 'info', message: 'queued' }],
      created_at: '2026-05-06T00:00:00Z',
      updated_at: '2026-05-06T00:00:00Z',
    }
    const get = vi.spyOn(service.client, 'get')
      .mockResolvedValueOnce({ data: { builds: [build] } } as any)
      .mockResolvedValueOnce({ data: { build } } as any)
      .mockResolvedValueOnce({ data: { build_id: build.id, logs: build.logs } } as any)
      .mockResolvedValueOnce({ data: { build_id: build.id, artifact_url: build.artifact_url, platform: 'android', profile: 'preview', release_level: 'internal_android_apk' } } as any)
    const post = vi.spyOn(service.client, 'post')
      .mockResolvedValueOnce({ data: { build } } as any)
      .mockResolvedValueOnce({ data: { build } } as any)
      .mockResolvedValueOnce({ data: { build: { ...build, status: 'canceled' } } } as any)
      .mockResolvedValueOnce({ data: { build: { ...build, id: 'mbld_retry', status: 'queued' } } } as any)

    const listed = await service.listProjectMobileBuilds(42)
    const created = await service.createProjectMobileBuild(42, { platform: 'android', profile: 'preview', release_level: 'internal_android_apk' })
    const fetched = await service.getProjectMobileBuild(42, build.id)
    const refreshed = await service.refreshProjectMobileBuild(42, build.id)
    const canceled = await service.cancelProjectMobileBuild(42, build.id)
    const retried = await service.retryProjectMobileBuild(42, build.id)
    const logs = await service.getProjectMobileBuildLogs(42, build.id)
    const artifact = await service.getProjectMobileBuildArtifacts(42, build.id)

    expect(get).toHaveBeenNthCalledWith(1, '/projects/42/mobile/builds')
    expect(post).toHaveBeenNthCalledWith(1, '/projects/42/mobile/builds', { platform: 'android', profile: 'preview', release_level: 'internal_android_apk' })
    expect(post).toHaveBeenNthCalledWith(2, '/projects/42/mobile/builds/mbld_123/refresh', {})
    expect(post).toHaveBeenNthCalledWith(3, '/projects/42/mobile/builds/mbld_123/cancel', {})
    expect(post).toHaveBeenNthCalledWith(4, '/projects/42/mobile/builds/mbld_123/retry', {})
    expect(get).toHaveBeenNthCalledWith(2, '/projects/42/mobile/builds/mbld_123')
    expect(get).toHaveBeenNthCalledWith(3, '/projects/42/mobile/builds/mbld_123/logs')
    expect(get).toHaveBeenNthCalledWith(4, '/projects/42/mobile/builds/mbld_123/artifacts')
    expect(listed[0].id).toBe('mbld_123')
    expect(created.build.status).toBe('succeeded')
    expect(fetched.artifact_url).toContain('app.apk')
    expect(refreshed.artifact_url).toContain('app.apk')
    expect(canceled.status).toBe('canceled')
    expect(retried.build.id).toBe('mbld_retry')
    expect(logs[0].message).toBe('queued')
    expect(artifact.artifact_url).toContain('app.apk')
  })

  it('starts Expo Web mobile previews through the gated preview route', async () => {
    const service = new ApiService('/api/v1')
    const post = vi.spyOn(service.client, 'post').mockResolvedValue({
      data: {
        success: true,
        preview_level: 'expo_web',
        preview_url: 'https://api.example.com/api/v1/preview/backend-proxy/42',
        message: 'Expo Web mobile preview started.',
      },
    } as any)

    const response = await service.startProjectMobileExpoWebPreview(42, { EXPO_PUBLIC_API_BASE_URL: 'https://api.example.com' })

    expect(post).toHaveBeenCalledWith('/preview/mobile/expo-web/start', {
      project_id: 42,
      env_vars: { EXPO_PUBLIC_API_BASE_URL: 'https://api.example.com' },
    }, {
      timeout: PREVIEW_START_TIMEOUT_MS,
    })
    expect(response.preview_level).toBe('expo_web')
    expect(response.preview_url).toContain('/preview/backend-proxy/42')
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
