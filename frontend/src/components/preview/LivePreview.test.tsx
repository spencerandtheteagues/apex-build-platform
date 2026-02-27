/* @vitest-environment jsdom */

import React from 'react'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/services/api', () => ({
  default: {
    client: {
      get: vi.fn(),
      post: vi.fn(),
    },
    startFullStackPreview: vi.fn(),
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
    vi.useRealTimers()
    const mockGet = apiService.client.get as any
    const mockPost = apiService.client.post as any
    const mockStartFullStack = (apiService as any).startFullStackPreview as any
    mockGet.mockReset()
    mockPost.mockReset()
    mockStartFullStack.mockReset()

    mockGet.mockImplementation(async (url: string) => {
      if (url === '/preview/docker/status') return { data: { available: false } }
      if (url === '/preview/bundler/status') return { data: { available: true } }
      if (url.startsWith('/preview/status/')) return { data: { preview: { active: false } } }
      if (url.startsWith('/preview/server/detect/')) return { data: { has_backend: false } }
      if (url.startsWith('/preview/server/status/')) return { data: { server: null } }
      if (url.startsWith('/preview/server/logs/')) return { data: { stdout: '', stderr: '' } }
      throw new Error(`Unexpected GET ${url}`)
    })

    // startFullStackPreview is the primary start path now
    mockStartFullStack.mockImplementation(async (data: any) => {
      return {
        sandbox: false,
        preview: {
          project_id: data.project_id,
          active: true,
          port: 3000,
          url: `http://localhost:3000/project-${data.project_id}`,
          started_at: new Date().toISOString(),
          last_access: new Date().toISOString(),
          connected_clients: 1,
        },
      }
    })

    // Fallback path (only called if fullstack returns 404/405)
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

  afterEach(() => {
    vi.useRealTimers()
  })

  it('auto-starts again after projectId changes', async () => {
    const mockStartFullStack = (apiService as any).startFullStackPreview as any
    const { rerender } = render(<LivePreview projectId={101} autoStart className="h-96" />)

    await waitFor(() => {
      expect(mockStartFullStack).toHaveBeenCalledWith(expect.objectContaining({ project_id: 101 }))
    })

    rerender(<LivePreview projectId={202} autoStart className="h-96" />)

    await waitFor(() => {
      expect(mockStartFullStack).toHaveBeenCalledWith(expect.objectContaining({ project_id: 202 }))
    })
  })

  it('keeps active preview visible when a status poll fails transiently', async () => {
    const mockGet = apiService.client.get as any
    const mockStartFullStack = (apiService as any).startFullStackPreview as any
    const intervalCallbacks: Array<() => void | Promise<void>> = []
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval').mockImplementation((((fn: TimerHandler) => {
      intervalCallbacks.push(fn as () => void)
      return 1 as unknown as ReturnType<typeof setInterval>
    }) as unknown) as typeof setInterval)
    const clearIntervalSpy = vi.spyOn(globalThis, 'clearInterval').mockImplementation(() => {})

    let failStatusPoll = false
    let previewRunning = false
    mockGet.mockImplementation(async (url: string) => {
      if (url === '/preview/docker/status') return { data: { available: false } }
      if (url === '/preview/bundler/status') return { data: { available: true } }
      if (url.startsWith('/preview/status/')) {
        if (failStatusPoll) {
          throw new Error('network hiccup')
        }
        return {
          data: {
            preview: previewRunning
              ? {
                project_id: 303,
                active: true,
                port: 3000,
                url: 'http://localhost:3000/project-303',
                started_at: new Date().toISOString(),
                last_access: new Date().toISOString(),
                connected_clients: 1,
              }
              : { active: false }
          }
        }
      }
      if (url.startsWith('/preview/server/detect/')) return { data: { has_backend: false } }
      if (url.startsWith('/preview/server/status/')) return { data: { server: null } }
      if (url.startsWith('/preview/server/logs/')) return { data: { stdout: '', stderr: '' } }
      throw new Error(`Unexpected GET ${url}`)
    })
    mockStartFullStack.mockImplementation(async (data: any) => {
      previewRunning = true
      return {
        sandbox: false,
        preview: {
          project_id: data.project_id,
          active: true,
          port: 3000,
          url: `http://localhost:3000/project-${data.project_id}`,
          started_at: new Date().toISOString(),
          last_access: new Date().toISOString(),
          connected_clients: 1,
        },
      }
    })

    const view = render(<LivePreview projectId={303} autoStart className="h-96" />)

    await waitFor(() => {
      expect(mockStartFullStack).toHaveBeenCalledWith(expect.objectContaining({ project_id: 303 }))
    })
    await waitFor(() => {
      expect(within(view.container).getAllByTitle('Live Preview').length).toBeGreaterThan(0)
    })

    failStatusPoll = true

    await act(async () => {
      await Promise.all(intervalCallbacks.map(cb => Promise.resolve(cb())))
    })

    // After a transient poll failure, the iframe should still be present
    // and the status should show "Not Running" (since connected becomes false)
    await waitFor(() => {
      expect(within(view.container).getAllByTitle('Live Preview').length).toBeGreaterThan(0)
      expect(within(view.container).getByText('Not Running')).toBeTruthy()
    })

    setIntervalSpy.mockRestore()
    clearIntervalSpy.mockRestore()
  })

  it('polls the active preview sandbox mode even after toggling sandbox setting', async () => {
    const mockGet = apiService.client.get as any
    const mockStartFullStack = (apiService as any).startFullStackPreview as any
    const intervalCallbacks: Array<() => void | Promise<void>> = []
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval').mockImplementation((((fn: TimerHandler) => {
      intervalCallbacks.push(fn as () => void)
      return 1 as unknown as ReturnType<typeof setInterval>
    }) as unknown) as typeof setInterval)
    const clearIntervalSpy = vi.spyOn(globalThis, 'clearInterval').mockImplementation(() => {})

    let previewRunning = false
    mockGet.mockImplementation(async (url: string, config?: any) => {
      if (url === '/preview/docker/status') return { data: { available: true } }
      if (url === '/preview/bundler/status') return { data: { available: true } }
      if (url.startsWith('/preview/status/')) {
        return {
          data: {
            preview: previewRunning
              ? {
                project_id: 404,
                active: true,
                port: 3000,
                url: 'http://localhost:3000/project-404',
                started_at: new Date().toISOString(),
                last_access: new Date().toISOString(),
                connected_clients: 1,
              }
              : { active: false },
            requestSandbox: config?.params?.sandbox
          }
        }
      }
      if (url.startsWith('/preview/server/detect/')) return { data: { has_backend: false } }
      if (url.startsWith('/preview/server/status/')) return { data: { server: null } }
      if (url.startsWith('/preview/server/logs/')) return { data: { stdout: '', stderr: '' } }
      throw new Error(`Unexpected GET ${url}`)
    })
    mockStartFullStack.mockImplementation(async (data: any) => {
      previewRunning = true
      return {
        sandbox: false,
        preview: {
          project_id: data.project_id,
          active: true,
          port: 3000,
          url: `http://localhost:3000/project-${data.project_id}`,
          started_at: new Date().toISOString(),
          last_access: new Date().toISOString(),
          connected_clients: 1,
        },
      }
    })

    const view = render(<LivePreview projectId={404} autoStart className="h-96" />)

    await waitFor(() => {
      expect(mockStartFullStack).toHaveBeenCalledWith(expect.objectContaining({ project_id: 404 }))
    })
    await waitFor(() => {
      expect(within(view.container).getAllByTitle('Live Preview').length).toBeGreaterThan(0)
    })

    fireEvent.click(within(view.container).getByTitle('Settings'))
    fireEvent.click(within(view.container).getByLabelText('Docker Sandbox'))

    await act(async () => {
      await Promise.all(intervalCallbacks.map(cb => Promise.resolve(cb())))
    })

    const statusCalls = mockGet.mock.calls.filter((call: any[]) => String(call[0]).startsWith('/preview/status/'))
    expect(statusCalls.length).toBeGreaterThan(0)
    const latestStatusCall = statusCalls[statusCalls.length - 1]
    expect(latestStatusCall[1]?.params?.sandbox).toBe('0')

    setIntervalSpy.mockRestore()
    clearIntervalSpy.mockRestore()
  })
})
