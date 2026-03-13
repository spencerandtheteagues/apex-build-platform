/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/services/api', () => ({
  default: {
    buildPreflight: vi.fn(),
    getBuildStatus: vi.fn(),
    getBuildDetails: vi.fn(),
    getCompletedBuild: vi.fn(),
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
const DEFAULT_RESTART_FAILED_MESSAGE = 'Restart the failed build from the last workable state, keep the valid work, fix the failure, and continue until the app is runnable.'

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

describe('AppBuilder control surface', () => {
  beforeEach(() => {
    localStorage.clear()
    Object.defineProperty(HTMLElement.prototype, 'scrollTo', {
      configurable: true,
      value: vi.fn(),
    })
    ;(apiService.buildPreflight as any).mockReset()
    ;(apiService.getBuildStatus as any).mockReset()
    ;(apiService.getBuildDetails as any).mockReset()
    ;(apiService.getCompletedBuild as any).mockReset()
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
  })

  it('routes planner broadcasts and direct agent messages with the expected targets', async () => {
    localStorage.setItem(ACTIVE_BUILD_STORAGE_KEY, 'build-123')
    ;(apiService.getBuildStatus as any).mockResolvedValue({ status: 'in_progress' })
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail())
    ;(apiService.sendBuildMessage as any).mockResolvedValue({
      interaction: buildDetail().interaction,
      live: false,
    })

    render(<AppBuilder />)

    await screen.findByText('Planner Console')

    fireEvent.click(screen.getByRole('button', { name: 'All Agents' }))

    const plannerInput = await screen.findByPlaceholderText('Broadcast a directive to every active agent...')
    fireEvent.change(plannerInput, { target: { value: 'Keep the user in the loop at each section.' } })
    fireEvent.keyDown(plannerInput, { key: 'Enter', code: 'Enter' })

    await waitFor(() => {
      expect(apiService.sendBuildMessage).toHaveBeenCalledWith(
        'build-123',
        'Keep the user in the loop at each section.',
        expect.objectContaining({
          targetMode: 'all_agents',
        })
      )
    })

    const frontendInput = await screen.findByPlaceholderText('Message Frontend directly...')
    fireEvent.change(frontendInput, { target: { value: 'Expose more build progress in the workspace.' } })
    fireEvent.keyDown(frontendInput, { key: 'Enter', code: 'Enter' })

    await waitFor(() => {
      expect(apiService.sendBuildMessage).toHaveBeenCalledWith(
        'build-123',
        'Expose more build progress in the workspace.',
        expect.objectContaining({
          targetMode: 'agent',
          targetAgentId: 'frontend-1',
          targetAgentRole: 'frontend',
        })
      )
    })
  })

  it('issues a restart command for failed builds', async () => {
    localStorage.setItem(ACTIVE_BUILD_STORAGE_KEY, 'failed-build-123')
    ;(apiService.getBuildStatus as any).mockResolvedValue({ status: 'failed' })
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
})
