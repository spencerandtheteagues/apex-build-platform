import { describe, expect, it } from 'vitest'
import {
  extractBuildFailureReason,
  mergeBuildStatusWithTerminalPrecedence,
  normalizeBuildStatus,
  reconcileBuildPayloadWithCompletedDetail,
  resolveBuildCompletedEventStatus,
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
