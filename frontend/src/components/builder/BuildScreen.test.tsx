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
    historicalLearning: undefined as any,
    promptPackActivationRequests: undefined as any,
    promptPackVersions: undefined as any,
    promptPackActivationEvents: undefined as any,
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
  patchBundleActionId: null,
  chatInput: '',
  setChatInput: vi.fn(),
  plannerSendMode: 'lead' as const,
  setPlannerSendMode: vi.fn(),
  plannerMessagePending: false,
  providerModelOverrides: {
    claude: 'auto',
    gpt4: 'auto',
    gemini: 'auto',
    grok: 'auto',
  },
  providerModelOptions: {
    claude: [{ id: 'claude-opus-4-6', name: 'Claude Opus 4.6' }],
    gpt4: [{ id: 'gpt-5.4', name: 'ChatGPT 5.4' }],
    gemini: [{ id: 'gemini-3.1-pro', name: 'Gemini 3.1 Pro' }],
    grok: [{ id: 'grok-4.20-0309-reasoning', name: 'Grok 4.20' }],
  },
  providerModelPendingProvider: null,
  agentMessageDrafts: {},
  agentMessagePendingId: null,
  onAgentMessageDraftChange: vi.fn(),
  onSendDirectAgentMessage: vi.fn(),
  onSendChatMessage: vi.fn(),
  onSelectProviderModel: vi.fn(),
  onPause: vi.fn(),
  onResume: vi.fn(),
  onRestart: vi.fn(),
  onStartOver: vi.fn(),
  onPreviewWorkspace: vi.fn(),
  onOpenInIDE: vi.fn(),
  onDownload: vi.fn(),
  onRollbackCheckpoint: vi.fn(),
  onResolvePermission: vi.fn(),
  onApprovePatchBundle: vi.fn(),
  onRejectPatchBundle: vi.fn(),
  onReviewPromptProposal: vi.fn(),
  onBenchmarkPromptProposal: vi.fn(),
  onCreatePromptPackDraft: vi.fn(),
  onRequestPromptPackActivation: vi.fn(),
  onActivatePromptPackRequest: vi.fn(),
  onRollbackPromptPackVersion: vi.fn(),
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
        review_branch: 'ai-repair/20260412-compile-validator-bundle-1',
        suggested_commit_title: 'AI repair: Compile validator Hydra winner (strict_ast_syntax_repair)',
        risk_reasons: ['dependency_changes_require_review'],
      },
    ]

    render(<BuildScreen {...props} />)

    fireEvent.click(screen.getByRole('button', { name: /^Issues(?:\s*\d+)?$/i }))

    expect(await screen.findByText(/Repair Patch Review/i)).toBeTruthy()
    expect(screen.getAllByText(/Compile validator Hydra winner/i).length).toBeGreaterThan(0)
    expect(screen.getByText(/merge policy: review required/i)).toBeTruthy()
    expect(screen.getByText(/review branch: ai-repair\/20260412-compile-validator-bundle-1/i)).toBeTruthy()
    expect(screen.getByText(/suggested commit: AI repair:/i)).toBeTruthy()
    expect(screen.getByText(/no proposed-edit diff is attached yet/i)).toBeTruthy()
    expect(screen.getByText(/dependency changes require review/i)).toBeTruthy()
    expect(screen.getByRole('button', { name: /approve/i })).toBeTruthy()
    expect(screen.getByRole('button', { name: /reject/i })).toBeTruthy()
  })

  it('approves and rejects review-required patch bundles from the issues overlay', async () => {
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
    fireEvent.click(await screen.findByRole('button', { name: /approve/i }))
    expect(props.onApprovePatchBundle).toHaveBeenCalledWith('patch-1')

    fireEvent.click(screen.getByRole('button', { name: /reject/i }))
    expect(props.onRejectPatchBundle).toHaveBeenCalledWith('patch-1')
  })

  it('does not show approved patch bundles as pending review work', async () => {
    const props = baseProps()
    props.buildState.patchBundles = [
      {
        id: 'patch-1',
        justification: 'Already approved repair',
        merge_policy: 'review_required',
        review_required: true,
        review_status: 'approved',
      },
    ]

    render(<BuildScreen {...props} />)

    fireEvent.click(screen.getByRole('button', { name: /^Issues(?:\s*\d+)?$/i }))

    expect(screen.queryByText(/Repair Patch Review/i)).toBeNull()
    expect(screen.queryByText(/Already approved repair/i)).toBeNull()
  })

  it('reviews benchmark-gated prompt proposals from the AI detail overlay', async () => {
    const props = baseProps()
    props.buildState.historicalLearning = {
      scope: 'stack:react+go',
      observed_builds: 2,
      prompt_improvement_proposals: [
        {
          id: 'prompt-preview',
          scope: 'stack:react+go',
          target_prompt: 'preview_repair',
          failure_cluster: 'preview_verification',
          proposal: 'Emphasize deterministic preview checks before visual polish.',
          evidence: ['failure_class=preview_verification count=2'],
          benchmark_gate: 'Run generated preview smoke benchmarks.',
          requires_approval: true,
          review_state: 'proposed',
          benchmark_status: 'not_started',
        },
      ],
    }

    render(<BuildScreen {...props} />)

    fireEvent.click(screen.getByRole('button', { name: /AI Detail/i }))

    expect(await screen.findByText('Prompt Proposals')).toBeTruthy()
    expect(screen.getByText(/Run generated preview smoke benchmarks/i)).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: /^Approve$/i }))
    expect(props.onReviewPromptProposal).toHaveBeenCalledWith('prompt-preview', 'approve')

    fireEvent.click(screen.getByRole('button', { name: /^Reject$/i }))
    expect(props.onReviewPromptProposal).toHaveBeenCalledWith('prompt-preview', 'reject')
  })

  it('runs benchmark gates for approved prompt proposals from the AI detail overlay', async () => {
    const props = baseProps()
    props.buildState.historicalLearning = {
      scope: 'stack:react+go',
      observed_builds: 2,
      prompt_improvement_proposals: [
        {
          id: 'prompt-approved',
          scope: 'stack:react+go',
          target_prompt: 'preview_repair',
          failure_cluster: 'preview_verification',
          proposal: 'Emphasize deterministic preview checks before visual polish.',
          evidence: ['failure_class=preview_verification count=2'],
          benchmark_gate: 'Run generated preview smoke benchmarks.',
          requires_approval: true,
          review_state: 'approved',
          benchmark_status: 'not_started',
        },
      ],
      prompt_adoption_candidates: [
        {
          id: 'adoption-prompt-approved',
          proposal_id: 'prompt-approved',
          scope: 'stack:react+go',
          target_prompt: 'preview_repair',
          failure_cluster: 'preview_verification',
          proposal: 'Emphasize deterministic preview checks before visual polish.',
          benchmark_gate: 'Run generated preview smoke benchmarks.',
          benchmark_status: 'passed',
          status: 'ready_for_adoption',
          prompt_mutated: false,
        },
      ],
      prompt_pack_drafts: [
        {
          id: 'prompt-pack-draft-1',
          version: 'draft-001',
          scope: 'stack:react+go',
          source_candidate_ids: ['adoption-prompt-approved'],
          changes: [
            {
              candidate_id: 'adoption-prompt-approved',
              proposal_id: 'prompt-approved',
              target_prompt: 'preview_repair',
              failure_cluster: 'preview_verification',
              proposal: 'Emphasize deterministic preview checks before visual polish.',
              benchmark_gate: 'Run generated preview smoke benchmarks.',
            },
          ],
          status: 'inactive_draft',
          prompt_mutated: false,
          activation_ready: false,
        },
      ],
    }

    render(<BuildScreen {...props} />)

    fireEvent.click(screen.getByRole('button', { name: /AI Detail/i }))

    expect(await screen.findByText(/Benchmark: Not Started/i)).toBeTruthy()
    expect(screen.getByText('Adoption Registry')).toBeTruthy()
    expect(screen.getByText(/live prompt generation has not changed/i)).toBeTruthy()
    expect(screen.getByText('Prompt-Pack Drafts')).toBeTruthy()
    expect(screen.getByText('draft-001')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: /run benchmark/i }))
    expect(props.onBenchmarkPromptProposal).toHaveBeenCalledWith('prompt-approved')
    fireEvent.click(screen.getByRole('button', { name: /create prompt-pack draft/i }))
    expect(props.onCreatePromptPackDraft).toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: /request admin activation/i }))
    expect(props.onRequestPromptPackActivation).toHaveBeenCalledWith('prompt-pack-draft-1')
  })

  it('activates pending prompt-pack requests into the registry from the AI detail overlay', async () => {
    const props = baseProps()
    props.buildState.historicalLearning = {
      scope: 'stack:react+go',
      observed_builds: 2,
      prompt_pack_drafts: [
        {
          id: 'prompt-pack-draft-1',
          version: 'draft-001',
          scope: 'stack:react+go',
          source_candidate_ids: ['adoption-prompt-approved'],
          changes: [
            {
              candidate_id: 'adoption-prompt-approved',
              proposal_id: 'prompt-approved',
              target_prompt: 'preview_repair',
              failure_cluster: 'preview_verification',
              proposal: 'Emphasize deterministic preview checks before visual polish.',
              benchmark_gate: 'Run generated preview smoke benchmarks.',
            },
          ],
          status: 'inactive_draft',
          prompt_mutated: false,
          activation_ready: false,
        },
      ],
    }
    props.buildState.promptPackActivationRequests = [
      {
        id: 'prompt-pack-activation-request-1',
        build_id: 'build-1',
        draft_id: 'prompt-pack-draft-1',
        draft_version: 'draft-001',
        scope: 'stack:react+go',
        status: 'pending_admin_activation',
        requested_by_id: 7,
        feature_flag: 'APEX_PROMPT_PACK_ACTIVATION_REQUESTS',
        prompt_mutated: false,
      },
    ]

    render(<BuildScreen {...props} />)

    fireEvent.click(screen.getByRole('button', { name: /AI Detail/i }))

    expect(await screen.findByText(/Pending Admin Activation/i)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: /activate registry version/i }))
    expect(props.onActivatePromptPackRequest).toHaveBeenCalledWith('prompt-pack-activation-request-1')
  })

  it('rolls back an active registry version from the AI detail overlay', async () => {
    const props = baseProps()
    props.buildState.historicalLearning = {
      scope: 'stack:react+go',
      observed_builds: 2,
      prompt_pack_drafts: [
        {
          id: 'prompt-pack-draft-1',
          version: 'draft-001',
          scope: 'stack:react+go',
          source_candidate_ids: ['adoption-prompt-approved'],
          changes: [],
          status: 'inactive_draft',
          prompt_mutated: false,
          activation_ready: false,
        },
      ],
    }
    props.buildState.promptPackActivationRequests = [
      {
        id: 'prompt-pack-activation-request-1',
        build_id: 'build-1',
        draft_id: 'prompt-pack-draft-1',
        draft_version: 'draft-001',
        scope: 'stack:react+go',
        status: 'activated_in_registry',
        requested_by_id: 7,
        feature_flag: 'APEX_PROMPT_PACK_ACTIVATION_REQUESTS',
        prompt_mutated: false,
      },
    ]
    props.buildState.promptPackVersions = [
      {
        id: 'prompt-pack-version-active-1',
        scope: 'stack:react+go',
        version: 'draft-001',
        status: 'active_registry_version',
        source_build_id: 'build-1',
        source_draft_id: 'prompt-pack-draft-1',
        source_request_id: 'prompt-pack-activation-request-1',
        activated_by_id: 7,
        prompt_mutated: false,
        live_prompt_read_enabled: false,
      },
    ]

    render(<BuildScreen {...props} />)

    fireEvent.click(screen.getByRole('button', { name: /AI Detail/i }))

    expect(await screen.findByText(/Active Registry Version/i)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: /rollback registry version/i }))
    expect(props.onRollbackPromptPackVersion).toHaveBeenCalledWith('prompt-pack-version-active-1')
  })

  it('opens proposed edit review from review-required repair bundles', async () => {
    const baseline = baseProps()
    const props = {
      ...baseline,
      buildState: {
        ...baseline.buildState,
        status: 'awaiting_review' as const,
        patchBundles: [
          {
            id: 'patch-1',
            justification: 'Compile validator Hydra winner (strict_ast_syntax_repair)',
            provider: 'gpt4',
            merge_policy: 'review_required' as const,
            review_required: true,
            review_branch: 'ai-repair/20260412-compile-validator-bundle-1',
            suggested_commit_title: 'AI repair: Compile validator Hydra winner (strict_ast_syntax_repair)',
            risk_reasons: ['dependency_changes_require_review'],
          },
        ],
      },
      proposedEdits: [
        {
          id: 'edit-1',
          build_id: 'build-1',
          agent_id: 'agent-1',
          agent_role: 'frontend',
          file_path: 'src/App.tsx',
          original_content: 'export default function App() { return null }',
          proposed_content: 'export default function App() { return <main /> }',
          language: 'tsx',
          status: 'pending' as const,
        },
      ],
    }

    render(<BuildScreen {...props} />)

    fireEvent.click(screen.getByRole('button', { name: /^Issues(?:\s*\d+)?$/i }))
    fireEvent.click(await screen.findByRole('button', { name: /open diff review/i }))

    expect(props.onSetShowDiffReview).toHaveBeenCalledWith(true)
  })

  it('keeps platform and failed-build notices visible after switching overlays', async () => {
    const baseline = baseProps()
    const props = {
      ...baseline,
      buildState: {
        ...baseline.buildState,
        status: 'failed' as const,
      },
      platformReadinessNotice: {
        title: 'Redis cache is misconfigured',
        body: 'Redis is using an external allowlisted endpoint.',
        detail: 'Update REDIS_URL to the apex-redis internal connection string and redeploy.',
        isCritical: false,
      },
      buildFailureAttribution: {
        title: 'This failure may be platform-related',
        body: 'Primary database connectivity dropped while the build was running.',
        detail: 'Retry after database connectivity returns.',
        capturedError: 'Build session unavailable',
      },
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
