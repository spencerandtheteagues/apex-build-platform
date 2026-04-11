/* @vitest-environment jsdom */

import React from 'react'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

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
  BuildHistory: () => null,
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

describe('AppBuilder resume state', () => {
  beforeEach(() => {
    installLocalStorageMock()
    ;(apiService.buildPreflight as any).mockReset()
    ;(apiService.featureReadiness as any).mockReset()
    ;(apiService.getBuildStatus as any).mockReset()
    ;(apiService.getBuildDetails as any).mockReset()
    ;(apiService.getCompletedBuild as any).mockReset()
    ;(apiService.listBuilds as any).mockReset()

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
    window.history.replaceState({}, '', '/')
  })

  afterEach(async () => {
    cleanup()
    localStorage.clear()
    window.history.replaceState({}, '', '/')
    window.__APEX_CONFIG__ = undefined
    await Promise.resolve()
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
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
})
