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
})
