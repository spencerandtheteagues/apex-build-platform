import { describe, expect, it } from 'vitest'
import {
  extractBuildFailureReason,
  mergeBuildStatusWithTerminalPrecedence,
  normalizeBuildStatus,
  parseBuildTelemetryThoughts,
  readBuildTelemetrySnapshot,
  reconcileBuildPayloadWithCompletedDetail,
  resolveBuildCompletedEventStatus,
  upsertBuildTelemetrySnapshot,
} from './buildRestore'

describe('reconcileBuildPayloadWithCompletedDetail', () => {
  it('overrides stale active status/progress with terminal completed build detail', () => {
    const payload = {
      build_id: 'build-123',
      status: 'reviewing',
      progress: 87,
      live: true,
      agents: [{ id: 'a1' }],
      files: [],
    }

    const completed = {
      id: 1,
      build_id: 'build-123',
      project_id: 42,
      project_name: 'Demo',
      description: 'Demo app',
      status: 'completed',
      mode: 'full',
      power_mode: 'balanced',
      tech_stack: null,
      files_count: 1,
      total_cost: 0,
      progress: 100,
      duration_ms: 5000,
      created_at: new Date().toISOString(),
      files: [{ path: '/src/main.tsx', content: 'export {}', language: 'typescript', size: 10, is_new: true }],
    }

    const result = reconcileBuildPayloadWithCompletedDetail(payload, completed as any)

    expect(result.status).toBe('completed')
    expect(result.progress).toBe(100)
    expect(result.live).toBe(false)
    expect(result.project_id).toBe(42)
    expect(result.files).toHaveLength(1)
    expect(result.agents).toEqual([{ id: 'a1' }])
  })

  it('does not override with non-terminal completed detail status', () => {
    const payload = { status: 'in_progress', progress: 25, live: true }
    const detail = {
      status: 'in_progress',
      progress: 30,
      files: [],
    }

    const result = reconcileBuildPayloadWithCompletedDetail(payload, detail as any)

    expect(result).toEqual(payload)
  })

  it('preserves completed detail error text when hydrating terminal builds', () => {
    const payload = { status: 'failed', progress: 90, live: false }
    const detail = {
      status: 'failed',
      progress: 90,
      error: 'Validation failed: tests did not pass',
      files: [],
    }

    const result = reconcileBuildPayloadWithCompletedDetail(payload, detail as any)

    expect(result.error).toBe('Validation failed: tests did not pass')
  })

  it('hydrates server activity timeline when the payload does not already include one', () => {
    const payload = { status: 'completed', progress: 100, live: false, agents: [], tasks: [], checkpoints: [], activity_timeline: [] }
    const detail = {
      status: 'completed',
      progress: 100,
      files: [],
      agents: [
        {
          id: 'agent-1',
          role: 'architect',
          provider: 'claude',
          status: 'completed',
          progress: 100,
        },
      ],
      tasks: [
        {
          id: 'task-1',
          type: 'plan',
          description: 'Define the execution plan',
          status: 'completed',
        },
      ],
      checkpoints: [
        {
          id: 'checkpoint-1',
          number: 1,
          name: 'Plan Ready',
          description: 'Initial plan locked',
          progress: 35,
          restorable: false,
          created_at: '2026-03-12T12:08:00.000Z',
        },
      ],
      activity_timeline: [
        {
          id: 'thought-1',
          agent_id: 'agent-1',
          agent_role: 'architect',
          provider: 'claude',
          type: 'thinking',
          content: 'Reviewing the final preview handoff',
          timestamp: '2026-03-12T12:10:00.000Z',
        },
      ],
      current_phase: 'validation',
      quality_gate_required: true,
      quality_gate_status: 'running',
      quality_gate_stage: 'preview',
      quality_gate_active: true,
      available_providers: ['claude', 'gpt4'],
      capability_state: {
        required_capabilities: ['auth', 'database'],
        requires_backend_runtime: true,
        requires_database: true,
      },
      policy_state: {
        plan_type: 'builder',
        classification: 'full_stack_candidate',
        full_stack_eligible: true,
      },
      blockers: [
        {
          id: 'blocker-1',
          title: 'Verification blocker',
          type: 'verification_blocker',
          category: 'runtime_failure',
          severity: 'blocking',
        },
      ],
      approvals: [
        {
          id: 'database',
          kind: 'database',
          title: 'Database access',
          status: 'satisfied',
          required: true,
          requested_at: '2026-03-12T12:00:00.000Z',
        },
      ],
      truth_by_surface: {
        frontend: ['prototype_ui_only'],
      },
    }

    const result = reconcileBuildPayloadWithCompletedDetail(payload, detail as any)

    expect(Array.isArray(result.activity_timeline)).toBe(true)
    expect(result.activity_timeline).toHaveLength(1)
    expect(result.agents).toHaveLength(1)
    expect(result.tasks).toHaveLength(1)
    expect(result.checkpoints).toHaveLength(1)
    expect(result.current_phase).toBe('validation')
    expect(result.quality_gate_required).toBe(true)
    expect(result.quality_gate_status).toBe('running')
    expect(result.quality_gate_stage).toBe('preview')
    expect(result.quality_gate_active).toBe(true)
    expect(result.available_providers).toEqual(['claude', 'gpt4'])
    expect(result.policy_state?.classification).toBe('full_stack_candidate')
    expect(result.capability_state?.requires_database).toBe(true)
    expect(result.blockers).toHaveLength(1)
    expect(result.approvals).toHaveLength(1)
    expect(result.truth_by_surface?.frontend).toEqual(['prototype_ui_only'])
  })
})

describe('build status normalization helpers', () => {
  it('normalizes common backend status aliases', () => {
    expect(normalizeBuildStatus('running')).toBe('in_progress')
    expect(normalizeBuildStatus('building')).toBe('in_progress')
    expect(normalizeBuildStatus('SUCCESS')).toBe('completed')
    expect(normalizeBuildStatus('canceled')).toBe('cancelled')
  })

  it('preserves terminal status when a stale non-terminal update arrives later', () => {
    expect(mergeBuildStatusWithTerminalPrecedence('completed', 'in_progress')).toBe('completed')
    expect(mergeBuildStatusWithTerminalPrecedence('failed', 'reviewing')).toBe('failed')
  })

  it('treats build:completed without explicit status as successful', () => {
    expect(resolveBuildCompletedEventStatus(undefined)).toBe('completed')
    expect(resolveBuildCompletedEventStatus('completed')).toBe('completed')
    expect(resolveBuildCompletedEventStatus('failed')).toBe('failed')
  })

  it('extracts the most specific failure reason from build payloads', () => {
    expect(extractBuildFailureReason({ error: 'Build failed', details: 'Unit tests failed' })).toBe('Unit tests failed')
    expect(extractBuildFailureReason({ error_detail: 'Provider timeout' })).toBe('Provider timeout')
  })
})

describe('build telemetry snapshot helpers', () => {
  it('stores and restores persisted AI telemetry for a build', () => {
    const raw = upsertBuildTelemetrySnapshot(null, {
      buildId: 'build-telemetry-1',
      updatedAt: '2026-03-12T12:00:00.000Z',
      thoughts: [
        {
          id: 'thought-1',
          agentId: 'agent-1',
          agentRole: 'architect',
          provider: 'claude',
          type: 'thinking',
          content: 'Planning runtime handoff',
          timestamp: '2026-03-12T11:59:30.000Z',
        },
      ],
    })

    const snapshot = readBuildTelemetrySnapshot(raw, 'build-telemetry-1')
    expect(snapshot?.buildId).toBe('build-telemetry-1')
    expect(snapshot?.thoughts).toHaveLength(1)
    expect(snapshot?.thoughts[0].provider).toBe('claude')
    expect(snapshot?.thoughts[0].content).toContain('runtime handoff')
  })

  it('drops malformed telemetry entries and keeps the most recent builds only', () => {
    let raw = JSON.stringify({
      broken: {
        buildId: 'broken',
        updatedAt: '2026-03-12T10:00:00.000Z',
        thoughts: [{ nope: true }],
      },
    })

    for (let index = 0; index < 22; index += 1) {
      raw = upsertBuildTelemetrySnapshot(raw, {
        buildId: `build-${index}`,
        updatedAt: `2026-03-12T12:${String(index).padStart(2, '0')}:00.000Z`,
        thoughts: [
          {
            id: `thought-${index}`,
            agentId: `agent-${index}`,
            agentRole: 'coder',
            provider: 'gpt4',
            type: 'output',
            content: `Update ${index}`,
            timestamp: `2026-03-12T12:${String(index).padStart(2, '0')}:30.000Z`,
          },
        ],
      })
    }

    expect(readBuildTelemetrySnapshot(raw, 'broken')).toBeNull()
    expect(readBuildTelemetrySnapshot(raw, 'build-0')).toBeNull()
    expect(readBuildTelemetrySnapshot(raw, 'build-1')).toBeNull()
    expect(readBuildTelemetrySnapshot(raw, 'build-21')?.thoughts[0].content).toBe('Update 21')
  })

  it('parses snake_case API telemetry entries into persisted thought shape', () => {
    const thoughts = parseBuildTelemetryThoughts([
      {
        id: 'thought-server-1',
        agent_id: 'agent-server-1',
        agent_role: 'verifier',
        provider: 'gemini',
        model: 'gemini-2.5-pro',
        type: 'error',
        event_type: 'agent:generation_failed',
        task_id: 'task-1',
        task_type: 'verify',
        retry_count: 1,
        max_retries: 3,
        is_internal: true,
        content: 'Validation failed on vite env typing',
        timestamp: '2026-03-12T12:30:00.000Z',
      },
    ])

    expect(thoughts).toHaveLength(1)
    expect(thoughts[0].agentId).toBe('agent-server-1')
    expect(thoughts[0].agentRole).toBe('verifier')
    expect(thoughts[0].eventType).toBe('agent:generation_failed')
    expect(thoughts[0].retryCount).toBe(1)
    expect(thoughts[0].isInternal).toBe(true)
  })
})
