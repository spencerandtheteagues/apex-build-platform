/* eslint-disable react-refresh/only-export-components */
/* @vitest-environment jsdom */

import React from 'react'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { vi } from 'vitest'

type StoreMocks = {
  createProject: ReturnType<typeof vi.fn>
  setCurrentProject: ReturnType<typeof vi.fn>
  addNotification: ReturnType<typeof vi.fn>
}

const getStoreMocks = (): StoreMocks => {
  if (!(globalThis as any).__APEX_APP_BUILDER_STORE_MOCKS__) {
    ;(globalThis as any).__APEX_APP_BUILDER_STORE_MOCKS__ = {
      createProject: vi.fn(),
      setCurrentProject: vi.fn(),
      addNotification: vi.fn(),
    }
  }
  return (globalThis as any).__APEX_APP_BUILDER_STORE_MOCKS__ as StoreMocks
}

vi.mock('@/services/api', () => ({
  default: {
    applyBuildArtifacts: vi.fn(),
    buildPreflight: vi.fn(),
    createFile: vi.fn(),
    exportProject: vi.fn(),
    featureReadiness: vi.fn(),
    getBuildStatus: vi.fn(),
    getBuildDetails: vi.fn(),
    getCompletedBuild: vi.fn(),
    getProject: vi.fn(),
    listBuilds: vi.fn(),
    sendBuildMessage: vi.fn(),
    startBuild: vi.fn(),
    getPlans: vi.fn(),
    createCheckoutSession: vi.fn(),
  },
}))

vi.mock('@/hooks/useStore', () => ({
  useStore: () => {
    const storeMocks = getStoreMocks()
    return {
      user: { id: 7, username: 'tester' },
      currentProject: null,
      createProject: storeMocks.createProject,
      setCurrentProject: storeMocks.setCurrentProject,
      addNotification: storeMocks.addNotification,
    }
  },
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

export const storeMocks = {
  get createProject() {
    return getStoreMocks().createProject
  },
  get setCurrentProject() {
    return getStoreMocks().setCurrentProject
  },
  get addNotification() {
    return getStoreMocks().addNotification
  },
}

export { act, AppBuilder, apiService, cleanup, fireEvent, render, screen, vi, waitFor }

export const MOCK_HISTORY_BUILD_ID = 'history-build-1'

export const buildDetail = (overrides: Record<string, any> = {}) => ({
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
  agents: [],
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

export const installLocalStorageMock = () => {
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

export const installWebSocketMock = () => {
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

export const primeAppBuilderHistoryTestEnv = () => {
  installLocalStorageMock()
  Object.defineProperty(HTMLElement.prototype, 'scrollTo', {
    configurable: true,
    value: vi.fn(),
  })
  ;(apiService.buildPreflight as any).mockReset()
  ;(apiService.applyBuildArtifacts as any).mockReset()
  ;(apiService.createFile as any).mockReset()
  ;(apiService.exportProject as any).mockReset()
  ;(apiService.featureReadiness as any).mockReset()
  ;(apiService.getBuildStatus as any).mockReset()
  ;(apiService.getBuildDetails as any).mockReset()
  ;(apiService.getCompletedBuild as any).mockReset()
  ;(apiService.getProject as any).mockReset()
  ;(apiService.listBuilds as any).mockReset()
  ;(apiService.sendBuildMessage as any).mockReset()
  ;(apiService.startBuild as any).mockReset()
  ;(apiService.getPlans as any).mockReset()
  ;(apiService.createCheckoutSession as any).mockReset()
  storeMocks.createProject.mockReset()
  storeMocks.setCurrentProject.mockReset()
  storeMocks.addNotification.mockReset()

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
  ;(apiService.applyBuildArtifacts as any).mockResolvedValue({
    project_id: 42,
    result: {
      created_project: true,
      total_files: 3,
    },
  })
  ;(apiService.getProject as any).mockResolvedValue({
    id: 42,
    name: 'Preview Canary',
    description: 'Preview-ready project',
    language: 'typescript',
  })
  storeMocks.createProject.mockResolvedValue({
    id: 77,
    name: 'Streamed Preview',
    description: 'Created from streamed files',
    language: 'typescript',
  })
  window.history.replaceState({}, '', '/')
}

export const teardownAppBuilderHistoryTestEnv = async () => {
  cleanup()
  localStorage.clear()
  window.history.replaceState({}, '', '/')
  window.__APEX_CONFIG__ = undefined
  await Promise.resolve()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
}

export const openMockedBuild = async () => {
  fireEvent.click(await screen.findByRole('button', { name: /open mocked build/i }))
  await screen.findByText(/Build Flow/i)
}
