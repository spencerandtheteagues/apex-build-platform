/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/services/api', () => ({
  default: {
    listBuilds: vi.fn(),
    downloadBuildAsZip: vi.fn(),
    cancelBuild: vi.fn(),
    deleteBuild: vi.fn(),
  },
}))

import { BuildHistory } from './BuildHistory'
import apiService from '@/services/api'

const buildSummary = (overrides: Record<string, any> = {}) => ({
  id: 1,
  build_id: 'build-1',
  project_name: '',
  description: 'History build',
  status: 'failed',
  mode: 'full',
  power_mode: 'balanced',
  tech_stack: { frontend: 'React', backend: 'Express', database: 'SQLite' },
  files_count: 8,
  total_cost: 0,
  progress: 92,
  duration_ms: 120000,
  created_at: '2026-03-27T12:00:00Z',
  live: false,
  resumable: false,
  ...overrides,
})

describe('BuildHistory', () => {
  beforeEach(() => {
    ;(apiService.listBuilds as any).mockReset()
    ;(apiService.downloadBuildAsZip as any).mockReset()
    ;(apiService.cancelBuild as any).mockReset()
    ;(apiService.deleteBuild as any).mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('removes a saved terminal build from the recent builds list', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    ;(apiService.listBuilds as any)
      .mockResolvedValueOnce({
        builds: [buildSummary({ build_id: 'failed-build', description: 'Failed history build' })],
        total: 1,
        page: 1,
        limit: 10,
      })
      .mockResolvedValueOnce({
        builds: [],
        total: 0,
        page: 1,
        limit: 10,
      })
    ;(apiService.deleteBuild as any).mockResolvedValue(undefined)

    render(<BuildHistory userId={7} />)

    fireEvent.click(await screen.findByRole('button', { name: /remove build failed history build/i }))

    await waitFor(() => {
      expect(apiService.deleteBuild).toHaveBeenCalledWith('failed-build')
    })
    await waitFor(() => {
      expect(screen.queryByText('Failed history build')).toBeNull()
    })
  })

  it('cancels an active saved build from the recent builds list', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    ;(apiService.listBuilds as any)
      .mockResolvedValueOnce({
        builds: [buildSummary({
          build_id: 'active-build',
          description: 'Active history build',
          status: 'reviewing',
          resumable: true,
          live: true,
        })],
        total: 1,
        page: 1,
        limit: 10,
      })
      .mockResolvedValueOnce({
        builds: [buildSummary({
          build_id: 'active-build',
          description: 'Active history build',
          status: 'cancelled',
          resumable: false,
          live: false,
        })],
        total: 1,
        page: 1,
        limit: 10,
      })
    ;(apiService.cancelBuild as any).mockResolvedValue(undefined)

    render(<BuildHistory userId={7} />)

    fireEvent.click(await screen.findByRole('button', { name: /cancel build active history build/i }))

    await waitFor(() => {
      expect(apiService.cancelBuild).toHaveBeenCalledWith('active-build')
    })
    expect(await screen.findByRole('button', { name: /remove build active history build/i })).toBeTruthy()
  })
})
