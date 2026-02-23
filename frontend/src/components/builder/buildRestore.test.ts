import { describe, expect, it } from 'vitest'
import { reconcileBuildPayloadWithCompletedDetail } from './buildRestore'

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
})

