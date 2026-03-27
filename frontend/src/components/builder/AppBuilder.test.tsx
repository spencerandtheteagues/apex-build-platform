/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/services/api', () => ({
  default: {
    buildPreflight: vi.fn(),
    featureReadiness: vi.fn(),
    getBuildStatus: vi.fn(),
    getBuildDetails: vi.fn(),
    getCompletedBuild: vi.fn(),
    listBuilds: vi.fn(),
    sendBuildMessage: vi.fn(),
  },
}))

vi.mock('@/hooks/useStore', () => ({
  useStore: () => ({
    user: { id: 7, username: 'tester' },
    currentProject: null,
    createProject: vi.fn(),
    setCurrentProject: vi.fn(),
    addNotification: vi.fn(),
  }),
}))

vi.mock('@/hooks/useThemeLogo', () => ({
  useThemeLogo: () => '',
}))

vi.mock('@/components/ui', () => {
  const Div = ({ children, ...props }: any) => {
    const { variant, glow, size, loading, icon, iconPosition, iconAnimation, ...rest } = props
    return <div {...rest}>{children}</div>
  }
  const Button = ({ children, ...props }: any) => {
    const { variant, glow, size, loading, icon, iconPosition, iconAnimation, ...rest } = props
    return <button {...rest}>{children}</button>
  }
  const Badge = ({ children, ...props }: any) => {
    const { variant, ...rest } = props
    return <span {...rest}>{children}</span>
  }
  return {
    Button,
    Card: Div,
    CardContent: Div,
    CardHeader: Div,
    CardTitle: Div,
    Badge,
    Avatar: Div,
    LoadingOverlay: () => null,
    AnimatedBackground: () => null,
  }
})

vi.mock('./ModelRoleConfig', () => ({
  default: () => null,
}))

vi.mock('./OnboardingTour', () => ({
  OnboardingTour: () => null,
}))

vi.mock('./BuildHistory', () => ({
  BuildHistory: ({ onOpenBuild }: any) => (
    <button
      type="button"
      onClick={() => onOpenBuild?.('history-build-1', 'resume')}
    >
      Open mocked build
    </button>
  ),
}))

vi.mock('@/components/project/AssetUploader', () => ({
  AssetUploader: () => null,
}))

vi.mock('@/components/import/GitHubImportWizard', () => ({
  GitHubImportWizard: () => null,
}))

vi.mock('@/components/billing/BuyCreditsModal', () => ({
  BuyCreditsModal: () => null,
}))

vi.mock('@/components/diff/DiffReviewPanel', () => ({
  default: () => null,
}))

import { AppBuilder } from './AppBuilder'
import apiService from '@/services/api'

const ACTIVE_BUILD_STORAGE_KEY = 'apex_active_build_id:7'
const DEFAULT_RESTART_FAILED_MESSAGE = 'Restart the failed build from the last workable state, keep the valid work, fix the failure, and continue until the app is runnable.'
const MOCK_HISTORY_BUILD_ID = 'history-build-1'

const buildDetail = (overrides: Record<string, any> = {}) => ({
  id: 'build-123',
  build_id: 'build-123',
  status: 'in_progress',
  progress: 48,
  description: 'Build a collaborative app builder console',
  files: [
    {
      path: 'src/App.tsx',
      content: 'export default function App(){return null}',
      language: 'typescript',
    },
  ],
  agents: [
    {
      id: 'lead-1',
      role: 'lead',
      provider: 'claude',
      model: 'claude-sonnet-4-6',
      status: 'working',
      progress: 55,
      current_task: {
        type: 'plan',
        description: 'Coordinating the build plan',
      },
    },
    {
      id: 'frontend-1',
      role: 'frontend',
      provider: 'gpt4',
      model: 'gpt-4.1',
      status: 'working',
      progress: 40,
      current_task: {
        type: 'generate_ui',
        description: 'Refining the workspace layout',
      },
    },
  ],
  tasks: [],
  checkpoints: [],
  interaction: {
    messages: [
      {
        id: 'lead-msg-1',
        role: 'lead',
        content: 'Planner online.',
        timestamp: '2026-03-12T12:00:00Z',
      },
    ],
    permission_rules: [],
    permission_requests: [],
    pending_revisions: [],
  },
  available_providers: ['claude', 'gpt4'],
  live: false,
  ...overrides,
})

const installLocalStorageMock = () => {
  const store = new Map<string, string>()
  const storage = {
    getItem: vi.fn((key: string) => store.get(String(key)) ?? null),
    setItem: vi.fn((key: string, value: string) => {
      store.set(String(key), String(value))
    }),
    removeItem: vi.fn((key: string) => {
      store.delete(String(key))
    }),
    clear: vi.fn(() => {
      store.clear()
    }),
    key: vi.fn((index: number) => Array.from(store.keys())[index] ?? null),
    get length() {
      return store.size
    },
  }

  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: storage,
  })
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    value: storage,
  })

  return storage
}

const installWebSocketMock = () => {
  const connections: Array<{
    url: string
    readyState: number
    onopen: ((event?: Event) => void) | null
    onmessage: ((event: MessageEvent) => void) | null
    onerror: ((event?: Event) => void) | null
    onclose: ((event: CloseEvent) => void) | null
    close: () => void
  }> = []

  class MockWebSocket {
    static CONNECTING = 0
    static OPEN = 1
    static CLOSING = 2
    static CLOSED = 3

    readyState = MockWebSocket.CONNECTING
    onopen: ((event?: Event) => void) | null = null
    onmessage: ((event: MessageEvent) => void) | null = null
    onerror: ((event?: Event) => void) | null = null
    onclose: ((event: CloseEvent) => void) | null = null

    constructor(public url: string) {
      connections.push(this)
    }

    close = () => {
      this.readyState = MockWebSocket.CLOSED
    }
  }

  Object.defineProperty(globalThis, 'WebSocket', {
    configurable: true,
    value: MockWebSocket,
  })
  Object.defineProperty(window, 'WebSocket', {
    configurable: true,
    value: MockWebSocket,
  })

  return connections
}

const openMockedBuild = async () => {
  fireEvent.click(await screen.findByRole('button', { name: /open mocked build/i }))
  await screen.findByText(/Build Flow/i)
}

describe('AppBuilder control surface', () => {
  beforeEach(() => {
    installLocalStorageMock()
    Object.defineProperty(HTMLElement.prototype, 'scrollTo', {
      configurable: true,
      value: vi.fn(),
    })
    ;(apiService.buildPreflight as any).mockReset()
    ;(apiService.featureReadiness as any).mockReset()
    ;(apiService.getBuildStatus as any).mockReset()
    ;(apiService.getBuildDetails as any).mockReset()
    ;(apiService.getCompletedBuild as any).mockReset()
    ;(apiService.listBuilds as any).mockReset()
    ;(apiService.sendBuildMessage as any).mockReset()

    ;(apiService.buildPreflight as any).mockResolvedValue({
      provider_statuses: {
        claude: 'available',
        gpt4: 'available',
        gemini: 'available',
        grok: 'available',
      },
      has_byok: false,
      ready: true,
    })
    ;(apiService.listBuilds as any).mockResolvedValue({
      builds: [],
      total: 0,
      page: 1,
      limit: 10,
    })
    ;(apiService.featureReadiness as any).mockResolvedValue({
      phase: 'ready',
      status: 'healthy',
      ready: true,
      degraded_features: [],
      services: [],
    })
  })

  it('routes planner broadcasts and direct agent messages with the expected targets', async () => {
    ;(apiService.getCompletedBuild as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
    }))
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
    }))
    ;(apiService.sendBuildMessage as any).mockResolvedValue({
      interaction: buildDetail().interaction,
      live: false,
    })

    render(<AppBuilder />)

    await openMockedBuild()

    fireEvent.click(screen.getByRole('button', { name: /^Steer Build$/i }))

    await screen.findByText(/Planner Console/i)

    fireEvent.click(screen.getByRole('button', { name: 'All Agents' }))

    const plannerInput = await screen.findByPlaceholderText('Broadcast a directive to every active agent...')
    fireEvent.change(plannerInput, { target: { value: 'Keep the user in the loop at each section.' } })
    fireEvent.keyDown(plannerInput, { key: 'Enter', code: 'Enter' })

    await waitFor(() => {
      expect(apiService.sendBuildMessage).toHaveBeenCalledWith(
        MOCK_HISTORY_BUILD_ID,
        'Keep the user in the loop at each section.',
        expect.objectContaining({
          targetMode: 'all_agents',
        })
      )
    })

    fireEvent.click(screen.getByRole('button', { name: /activity/i }))

    await screen.findByText(/AI Agents Working/i)

    const frontendInput = await screen.findByPlaceholderText('Message Frontend directly...')
    fireEvent.change(frontendInput, { target: { value: 'Expose more build progress in the workspace.' } })
    fireEvent.keyDown(frontendInput, { key: 'Enter', code: 'Enter' })

    await waitFor(() => {
      expect(apiService.sendBuildMessage).toHaveBeenCalledWith(
        MOCK_HISTORY_BUILD_ID,
        'Expose more build progress in the workspace.',
        expect.objectContaining({
          targetMode: 'agent',
          targetAgentId: 'frontend-1',
          targetAgentRole: 'frontend',
        })
      )
    })
  })

  it('shows only live agent and task boxes while a build is active', async () => {
    ;(apiService.getCompletedBuild as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
    }))
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
      agents: [
        {
          id: 'frontend-1',
          role: 'frontend',
          provider: 'gpt4',
          model: 'gpt-4.1',
          status: 'working',
          progress: 62,
          current_task: {
            type: 'generate_ui',
            description: 'Refining the live workspace shell',
          },
        },
        {
          id: 'backend-1',
          role: 'backend',
          provider: 'claude',
          model: 'claude-sonnet-4-6',
          status: 'completed',
          progress: 100,
          current_task: {
            type: 'generate_api',
            description: 'Completed API contract wiring',
          },
        },
        {
          id: 'reviewer-1',
          role: 'reviewer',
          provider: 'claude',
          model: 'claude-sonnet-4-6',
          status: 'error',
          progress: 100,
          current_task: {
            type: 'test',
            description: 'Verifier false positive on mocks',
          },
        },
      ],
      tasks: [
        {
          id: 'task-live',
          type: 'fix',
          description: 'Finishing the live recovery pass',
          status: 'in_progress',
        },
        {
          id: 'task-done',
          type: 'generate_api',
          description: 'Completed API contract wiring',
          status: 'completed',
        },
        {
          id: 'task-failed',
          type: 'test',
          description: 'Verifier false positive on mocks',
          status: 'failed',
        },
      ],
    }))

    render(<AppBuilder />)

    await openMockedBuild()

    fireEvent.click(await screen.findByRole('button', { name: /activity/i }))

    await screen.findByText(/AI Agents Working/i)

    expect(screen.getByPlaceholderText('Message Frontend directly...')).toBeTruthy()
    expect(screen.queryByPlaceholderText('Message Backend directly...')).toBeNull()
    expect(screen.queryByPlaceholderText('Message Reviewer directly...')).toBeNull()

    expect(screen.getByText('Live Tasks')).toBeTruthy()
    expect(screen.getByText('Finishing the live recovery pass')).toBeTruthy()
    expect(screen.queryByText('Completed API contract wiring')).toBeNull()
    expect(screen.queryByText('Verifier false positive on mocks')).toBeNull()
  })

  it('issues a restart command for failed builds', async () => {
    ;(apiService.getCompletedBuild as any).mockResolvedValue(buildDetail({
      id: 'failed-build-123',
      build_id: 'failed-build-123',
      status: 'failed',
      progress: 92,
      live: false,
      error: 'Preview validation failed',
    }))
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail({
      id: 'failed-build-123',
      build_id: 'failed-build-123',
      status: 'failed',
      progress: 92,
      live: false,
      error: 'Preview validation failed',
    }))
    ;(apiService.sendBuildMessage as any).mockResolvedValue({
      interaction: buildDetail().interaction,
      live: false,
    })

    render(<AppBuilder />)

    fireEvent.click(await screen.findByRole('button', { name: /open mocked build/i }))

    const restartButton = await screen.findByRole('button', { name: /restart failed build/i })
    fireEvent.click(restartButton)

    await waitFor(() => {
      expect(apiService.sendBuildMessage).toHaveBeenCalledWith(
        'failed-build-123',
        DEFAULT_RESTART_FAILED_MESSAGE,
        expect.objectContaining({
          command: 'restart_failed',
          targetMode: 'lead',
        })
      )
    })
  })

  it('defaults to the compact overview and only opens deep panels when selected', async () => {
    ;(apiService.getCompletedBuild as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
    }))
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
      blockers: [
        {
          id: 'blocker-1',
          title: 'Missing API key',
          type: 'runtime',
          category: 'secrets',
          severity: 'blocking',
          summary: 'The backend needs an API key before live transcription can run.',
          unblocks_with: 'Provide the required secret and rerun verification.',
        },
      ],
    }))

    render(<AppBuilder />)

    await openMockedBuild()

    expect(screen.queryByText(/Planner Console/i)).toBeNull()
    expect(screen.queryByText(/Build Timeline/i)).toBeNull()
    expect(screen.queryByText(/AI Agents Working/i)).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: /files/i }))
    await screen.findByText('src/App.tsx')

    fireEvent.click(screen.getByRole('button', { name: /timeline/i }))
    await screen.findByText(/Planner And System Feed/i)

    fireEvent.click(screen.getByRole('button', { name: /issues/i }))
    await screen.findByText(/Missing API key/i)

    fireEvent.click(screen.getByRole('button', { name: /diagnostics/i }))
    await screen.findByText(/Build Timeline/i)
  })

  it('surfaces the Redis allowlist fix when platform readiness exposes the misconfiguration', async () => {
    ;(apiService.getCompletedBuild as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
    }))
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
    }))
    ;(apiService.featureReadiness as any).mockResolvedValue({
      phase: 'ready',
      status: 'degraded',
      ready: true,
      degraded_features: ['redis_cache'],
      services: [
        {
          name: 'primary_database',
          tier: 'critical',
          state: 'ready',
          summary: 'Primary database connected',
        },
        {
          name: 'redis_cache',
          tier: 'optional',
          state: 'degraded',
          summary: 'Using in-memory cache fallback',
          details: {
            fallback_reason: 'redis ping failed: AUTH failed: Client IP address is not in the allowlist.',
            recommended_fix: 'On Render, point REDIS_URL at the apex-redis internal connection string instead of an external allowlisted Redis URL.',
          },
        },
      ],
    })

    render(<AppBuilder />)

    await openMockedBuild()

    await screen.findByText(/Redis cache is misconfigured/i)
    expect(screen.getByText(/Redis is using an external allowlisted endpoint/i)).toBeTruthy()
    expect(screen.getByText(/internal connection string/i)).toBeTruthy()
    expect(screen.getByRole('button', { name: /open issues/i })).toBeTruthy()
  })

  it('frames failed builds as platform-related when critical runtime services are degraded', async () => {
    ;(apiService.getCompletedBuild as any).mockResolvedValue(buildDetail({
      id: 'failed-build-123',
      build_id: 'failed-build-123',
      status: 'failed',
      progress: 88,
      live: false,
      error: 'Build session unavailable',
    }))
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail({
      id: 'failed-build-123',
      build_id: 'failed-build-123',
      status: 'failed',
      progress: 88,
      live: false,
      error: 'Build session unavailable',
    }))
    ;(apiService.featureReadiness as any).mockResolvedValue({
      phase: 'failed',
      status: 'failed',
      ready: false,
      degraded_features: [],
      services: [
        {
          name: 'primary_database',
          tier: 'critical',
          state: 'failed',
          summary: 'Primary database unavailable',
        },
      ],
    })

    render(<AppBuilder />)

    fireEvent.click(await screen.findByRole('button', { name: /open mocked build/i }))

    await screen.findByText(/This failure may be platform-related/i)
    expect(screen.getAllByText(/Primary database connectivity dropped while the build was running/i).length).toBeGreaterThan(0)
    expect(screen.getByText(/Captured build error: Build session unavailable/i)).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: /console/i }))

    await screen.findByText(/Planner Console/i)
    expect(screen.getAllByText(/This failure may be platform-related/i).length).toBeGreaterThan(0)
  })

  it('hides live agent and task panels for failed builds even if stale worker state is present', async () => {
    ;(apiService.getCompletedBuild as any).mockResolvedValue(buildDetail({
      id: 'failed-build-123',
      build_id: 'failed-build-123',
      status: 'failed',
      progress: 92,
      live: true,
      error: 'Preview validation failed',
      tasks: [
        {
          id: 'task-live',
          type: 'fix',
          description: 'Finishing the live recovery pass',
          status: 'in_progress',
        },
      ],
    }))
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail({
      id: 'failed-build-123',
      build_id: 'failed-build-123',
      status: 'failed',
      progress: 92,
      live: true,
      error: 'Preview validation failed',
      tasks: [
        {
          id: 'task-live',
          type: 'fix',
          description: 'Finishing the live recovery pass',
          status: 'in_progress',
        },
      ],
    }))

    render(<AppBuilder />)

    fireEvent.click(await screen.findByRole('button', { name: /open mocked build/i }))

    await screen.findByRole('button', { name: /restart failed build/i })

    expect(screen.queryByText(/AI Agents Working/i)).toBeNull()
    expect(screen.queryByText('Live Tasks')).toBeNull()
    expect(screen.queryByText('Finishing the live recovery pass')).toBeNull()
  })

  it('reconnects restart recovery even when a failed build detail incorrectly reports a live session', async () => {
    const connections = installWebSocketMock()

    ;(apiService.getCompletedBuild as any).mockResolvedValue(buildDetail({
      id: 'failed-build-123',
      build_id: 'failed-build-123',
      status: 'failed',
      progress: 92,
      live: true,
      websocket_url: 'wss://runtime.example/ws/build/failed-build-123',
      error: 'Preview validation failed',
    }))
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail({
      id: 'failed-build-123',
      build_id: 'failed-build-123',
      status: 'failed',
      progress: 92,
      live: true,
      websocket_url: 'wss://runtime.example/ws/build/failed-build-123',
      error: 'Preview validation failed',
    }))
    ;(apiService.sendBuildMessage as any).mockResolvedValue({
      interaction: buildDetail().interaction,
      live: true,
    })

    render(<AppBuilder />)

    fireEvent.click(await screen.findByRole('button', { name: /open mocked build/i }))

    const restartButton = await screen.findByRole('button', { name: /restart failed build/i })
    fireEvent.click(restartButton)

    await waitFor(() => {
      expect(connections).toHaveLength(1)
      expect(connections[0]?.url).toBe('wss://runtime.example/ws/build/failed-build-123')
    })
  })

  it('keeps the builder on a fresh prompt after login until the user opens a previous build', async () => {
    localStorage.setItem(ACTIVE_BUILD_STORAGE_KEY, 'legacy-failed-build')
    localStorage.setItem('apex_last_workflow_build_id:7', 'legacy-failed-build')

    render(<AppBuilder />)

    await screen.findByPlaceholderText(/Describe the app you want to build/i)

    expect(apiService.getBuildStatus).not.toHaveBeenCalled()
    expect(apiService.getBuildDetails).not.toHaveBeenCalled()
    expect(apiService.getCompletedBuild).not.toHaveBeenCalled()
  })

  it('opens a previous build only after the user selects it from history', async () => {
    ;(apiService.getCompletedBuild as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
      status: 'failed',
      progress: 91,
      live: false,
      error: 'Recovered from explicit history open',
    }))
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
      status: 'failed',
      progress: 91,
      live: false,
      error: 'Recovered from explicit history open',
    }))

    render(<AppBuilder />)

    await screen.findByPlaceholderText(/Describe the app you want to build/i)
    expect(apiService.getCompletedBuild).not.toHaveBeenCalled()

    fireEvent.click(await screen.findByRole('button', { name: /open mocked build/i }))

    await screen.findByRole('button', { name: /restart failed build/i })
    expect(apiService.getCompletedBuild).toHaveBeenCalledWith(MOCK_HISTORY_BUILD_ID)
  })
})
