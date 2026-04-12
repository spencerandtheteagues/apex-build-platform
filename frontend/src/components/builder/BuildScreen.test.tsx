/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import BuildScreen from './BuildScreen'

const baseProps = () => ({
  buildState: {
    id: 'build-1',
    status: 'in_progress' as const,
    progress: 42,
    description: 'Build a premium project importer that audits uploaded apps, shows a polished React UI first, and keeps the full prompt accessible.',
    agents: [],
    tasks: [],
    checkpoints: [],
    interaction: undefined,
    currentPhase: 'planning',
    blockers: [],
    patchBundles: [] as any[],
    powerMode: 'balanced' as const,
  },
  providerPanels: [],
  aiThoughts: [],
  chatMessages: [],
  generatedFiles: [],
  proposedEdits: [],
  isBuildActive: true,
  buildPaused: false,
  pendingQuestion: null,
  pendingPermissionRequests: [],
  pendingRevisionRequests: [],
  buildActionPending: null,
  hasBYOK: false,
  phaseLabel: 'Planning',
  visibleBlockers: [],
  platformReadinessNotice: null,
  buildFailureAttribution: null,
  showDiffReview: false,
  userId: 7,
  isPreparingPreview: false,
  isCreatingProject: false,
  isStartingOver: false,
  createdProjectId: 22,
  permissionActionId: null,
  rollbackCheckpointId: null,
  chatInput: '',
  setChatInput: vi.fn(),
  plannerSendMode: 'lead' as const,
  setPlannerSendMode: vi.fn(),
  plannerMessagePending: false,
  agentMessageDrafts: {},
  agentMessagePendingId: null,
  onAgentMessageDraftChange: vi.fn(),
  onSendDirectAgentMessage: vi.fn(),
  onSendChatMessage: vi.fn(),
  onPause: vi.fn(),
  onResume: vi.fn(),
  onRestart: vi.fn(),
  onStartOver: vi.fn(),
  onPreviewWorkspace: vi.fn(),
  onOpenInIDE: vi.fn(),
  onDownload: vi.fn(),
  onRollbackCheckpoint: vi.fn(),
  onResolvePermission: vi.fn(),
  onSetShowDiffReview: vi.fn(),
  onLoadProposedEdits: vi.fn(),
  onOpenCompletedBuild: vi.fn(),
})

describe('BuildScreen header prompt actions', () => {
  const writeText = vi.fn()

  beforeEach(() => {
    writeText.mockReset()
    Object.defineProperty(globalThis.navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText,
      },
    })
  })

  it('copies the full build prompt from the header action', async () => {
    const props = baseProps()
    writeText.mockResolvedValue(undefined)

    render(<BuildScreen {...props} />)

    fireEvent.click(screen.getByRole('button', { name: /copy build prompt/i }))

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(props.buildState.description)
    })

    expect(await screen.findByText('Copied')).toBeTruthy()
  })

  it('surfaces review-required patch bundles in the issues overlay', async () => {
    const props = baseProps()
    props.buildState.patchBundles = [
      {
        id: 'patch-1',
        justification: 'Compile validator Hydra winner (strict_ast_syntax_repair)',
        provider: 'gpt4',
        merge_policy: 'review_required',
        review_required: true,
        risk_reasons: ['dependency_changes_require_review'],
      },
    ]

    render(<BuildScreen {...props} />)

    fireEvent.click(screen.getByRole('button', { name: /^Issues(?:\s*\d+)?$/i }))

    expect(await screen.findByText(/Repair Patch Review/i)).toBeTruthy()
    expect(screen.getByText(/Compile validator Hydra winner/i)).toBeTruthy()
    expect(screen.getByText(/merge policy: review required/i)).toBeTruthy()
    expect(screen.getByText(/dependency changes require review/i)).toBeTruthy()
  })

  it('keeps platform and failed-build notices visible after switching overlays', async () => {
    const props: any = baseProps()
    props.buildState.status = 'failed'
    props.platformReadinessNotice = {
      title: 'Redis cache is misconfigured',
      body: 'Redis is using an external allowlisted endpoint.',
      detail: 'Update REDIS_URL to the apex-redis internal connection string and redeploy.',
      isCritical: false,
    }
    props.buildFailureAttribution = {
      title: 'This failure may be platform-related',
      body: 'Primary database connectivity dropped while the build was running.',
      detail: 'Retry after database connectivity returns.',
      capturedError: 'Build session unavailable',
    }

    render(<BuildScreen {...props} />)

    fireEvent.click(screen.getByRole('button', { name: /^Issues(?:\s*\d+)?$/i }))

    expect(await screen.findByText(/Redis cache is misconfigured/i)).toBeTruthy()
    expect(screen.getByText(/This failure may be platform-related/i)).toBeTruthy()
    expect(screen.getByText(/Build session unavailable/i)).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: /^Console$/i }))
    await screen.findByText(/Planner Console/i)

    fireEvent.click(screen.getByRole('button', { name: /^Issues(?:\s*\d+)?$/i }))

    expect(await screen.findByText(/Redis cache is misconfigured/i)).toBeTruthy()
    expect(screen.getByText(/This failure may be platform-related/i)).toBeTruthy()
    expect(screen.getByText(/Build session unavailable/i)).toBeTruthy()
  })
})
