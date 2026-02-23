/* @vitest-environment jsdom */

import React from 'react'
import { render, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'

vi.mock('@/services/api', () => ({
  default: {
    client: {
      get: vi.fn(),
      post: vi.fn(),
    },
  },
}))

vi.mock('./ConsolePanel', () => ({
  default: () => null,
}))

vi.mock('./NetworkPanel', () => ({
  default: () => null,
}))

import apiService from '@/services/api'
import LivePreview from './LivePreview'

describe('LivePreview', () => {
  beforeEach(() => {
    const mockGet = apiService.client.get as any
    const mockPost = apiService.client.post as any
    mockGet.mockReset()
    mockPost.mockReset()

    mockGet.mockImplementation(async (url: string) => {
      if (url === '/preview/docker/status') return { data: { available: false } }
      if (url === '/preview/bundler/status') return { data: { available: true } }
      if (url.startsWith('/preview/status/')) return { data: { preview: { active: false } } }
      if (url.startsWith('/preview/server/detect/')) return { data: { has_backend: false } }
      if (url.startsWith('/preview/server/status/')) return { data: { server: null } }
      if (url.startsWith('/preview/server/logs/')) return { data: { stdout: '', stderr: '' } }
      throw new Error(`Unexpected GET ${url}`)
    })

    mockPost.mockImplementation(async (url: string, body?: any) => {
      if (url === '/preview/start') {
        return {
          data: {
            sandbox: false,
            preview: {
              project_id: body.project_id,
              active: true,
              port: 3000,
              url: `http://localhost:3000/project-${body.project_id}`,
              started_at: new Date().toISOString(),
              last_access: new Date().toISOString(),
              connected_clients: 1,
            },
          },
        }
      }
      if (url === '/preview/stop' || url === '/preview/refresh') return { data: {} }
      throw new Error(`Unexpected POST ${url}`)
    })
  })

  it('auto-starts again after projectId changes', async () => {
    const mockPost = apiService.client.post as any
    const { rerender } = render(<LivePreview projectId={101} autoStart className="h-96" />)

    await waitFor(() => {
      expect(mockPost).toHaveBeenCalledWith('/preview/start', expect.objectContaining({ project_id: 101 }))
    })

    rerender(<LivePreview projectId={202} autoStart className="h-96" />)

    await waitFor(() => {
      expect(mockPost).toHaveBeenCalledWith('/preview/start', expect.objectContaining({ project_id: 202 }))
    })
  })
})
