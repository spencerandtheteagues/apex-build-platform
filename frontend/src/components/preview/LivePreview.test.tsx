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
import { formatPreviewStartError } from '@/hooks/usePreviewRuntime'
import LivePreview from './LivePreview'

describe('formatPreviewStartError', () => {
  it('includes sandbox diagnostics instead of hiding the preview startup cause', () => {
    expect(formatPreviewStartError({
      error: 'Failed to start preview',
      diagnostics: {
        sandbox_error: 'container preview build failed: missing package.json',
      },
    })).toBe('Failed to start preview: container preview build failed: missing package.json')
  })
})

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

    // Full-stack preview is reserved for detected backend projects.
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

    // Fallback path (called when fullstack cannot start a frontend-only preview)
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

  it('restarts an active preview runtime from the workspace controls', async () => {
    const mockPost = apiService.client.post as any
    const view = render(<LivePreview projectId={606} autoStart className="h-96" />)

    await waitFor(() => {
      expect(mockPost).toHaveBeenCalledWith('/preview/start', expect.objectContaining({ project_id: 606 }))
    })

    fireEvent.click(within(view.container).getByRole('button', { name: /restart/i }))

    await waitFor(() => {
      expect(mockPost).toHaveBeenCalledWith('/preview/stop', {
        project_id: 606,
        sandbox: false,
      })
      expect(mockPost.mock.calls.filter(([url]: [string]) => url === '/preview/start')).toHaveLength(2)
    })
  })

  it('keeps active preview visible when a status poll fails transiently', async () => {
    const mockGet = apiService.client.get as any
    const mockPost = apiService.client.post as any
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
    mockPost.mockImplementation(async (url: string, data: any) => {
      if (url === '/preview/stop' || url === '/preview/refresh') return { data: {} }
      if (url !== '/preview/start') throw new Error(`Unexpected POST ${url}`)
      previewRunning = true
      return {
        data: {
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
        },
      }
    })

    const view = render(<LivePreview projectId={303} autoStart className="h-96" />)

    await waitFor(() => {
      expect(mockPost).toHaveBeenCalledWith('/preview/start', expect.objectContaining({ project_id: 303 }))
    })
    await waitFor(() => {
      expect(within(view.container).getAllByTitle('Live Preview').length).toBeGreaterThan(0)
    })

    failStatusPoll = true

    await act(async () => {
      await Promise.all(intervalCallbacks.map(cb => Promise.resolve(cb())))
    })

    // After a transient poll failure, the iframe should still be present
    // and the status should show a degraded runtime instead of a hard stop.
    await waitFor(() => {
      expect(within(view.container).getAllByTitle('Live Preview').length).toBeGreaterThan(0)
      expect(within(view.container).getByText('Degraded')).toBeTruthy()
    })

    setIntervalSpy.mockRestore()
    clearIntervalSpy.mockRestore()
  })

  it('polls the active preview sandbox mode even after toggling sandbox setting', async () => {
    const mockGet = apiService.client.get as any
    const mockPost = apiService.client.post as any
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
    mockPost.mockImplementation(async (url: string, data: any) => {
      if (url === '/preview/stop' || url === '/preview/refresh') return { data: {} }
      if (url !== '/preview/start') throw new Error(`Unexpected POST ${url}`)
      previewRunning = true
      return {
        data: {
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
        },
      }
    })

    const view = render(<LivePreview projectId={404} autoStart className="h-96" />)

    await waitFor(() => {
      expect(mockPost).toHaveBeenCalledWith('/preview/start', expect.objectContaining({ project_id: 404 }))
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

  it('locks sandbox mode and disables backend preview controls when secure preview is enforced', async () => {
    const mockGet = apiService.client.get as any

    mockGet.mockImplementation(async (url: string) => {
      if (url === '/preview/docker/status') {
        return {
          data: {
            available: true,
            sandbox_required: true,
            backend_preview_available: false,
            backend_preview_reason: 'Backend runtime preview is disabled while secure sandbox preview is enforced',
          }
        }
      }
      if (url === '/preview/bundler/status') return { data: { available: true } }
      if (url.startsWith('/preview/status/')) return { data: { preview: { active: false } } }
      if (url.startsWith('/preview/server/detect/')) {
        return { data: { has_backend: true, framework: 'express', server_type: 'node' } }
      }
      if (url.startsWith('/preview/server/status/')) {
        return {
          data: {
            available: false,
            reason: 'Backend runtime preview is disabled while secure sandbox preview is enforced',
            server: null,
          }
        }
      }
      if (url.startsWith('/preview/server/logs/')) return { data: { stdout: '', stderr: '' } }
      throw new Error(`Unexpected GET ${url}`)
    })

    const view = render(<LivePreview projectId={505} className="h-96" />)

    await waitFor(() => {
      expect(within(view.container).getByText('API Disabled')).toBeTruthy()
    })

    fireEvent.click(within(view.container).getByTitle('Settings'))

    const sandboxToggle = within(view.container).getByLabelText('Docker Sandbox') as HTMLInputElement
    expect(sandboxToggle.checked).toBe(true)
    expect(sandboxToggle.disabled).toBe(true)

    const startApiButton = within(view.container).getByRole('button', { name: /start api/i }) as HTMLButtonElement
    expect(startApiButton.disabled).toBe(true)
  })

  it('uses process fallback when production sandbox is required but Docker is degraded', async () => {
    const mockGet = apiService.client.get as any
    const mockPost = apiService.client.post as any

    mockGet.mockImplementation(async (url: string) => {
      if (url === '/preview/docker/status') {
        return {
          data: {
            available: false,
            sandbox_required: true,
            sandbox_degraded: true,
            backend_preview_available: true,
            backend_preview_reason: 'Server Docker is unavailable, so preview is using process fallback mode',
          }
        }
      }
      if (url === '/preview/bundler/status') return { data: { available: true } }
      if (url.startsWith('/preview/status/')) return { data: { preview: { active: false }, sandbox_degraded: true } }
      if (url.startsWith('/preview/server/detect/')) return { data: { has_backend: false } }
      if (url.startsWith('/preview/server/status/')) return { data: { server: null } }
      if (url.startsWith('/preview/server/logs/')) return { data: { stdout: '', stderr: '' } }
      throw new Error(`Unexpected GET ${url}`)
    })

    render(<LivePreview projectId={808} autoStart className="h-96" />)

    await waitFor(() => {
      expect(mockPost).toHaveBeenCalledWith('/preview/start', expect.objectContaining({
        project_id: 808,
        sandbox: false,
      }))
    })
  })

  it('uses frontend-only startup before backend detection is available', async () => {
    const mockPost = apiService.client.post as any
    const mockStartFullStack = (apiService as any).startFullStackPreview as any
    const view = render(<LivePreview projectId={818} className="h-96" />)

    fireEvent.click(within(view.container).getAllByRole('button', { name: /start preview/i })[0])

    await waitFor(() => {
      expect(mockPost).toHaveBeenCalledWith('/preview/start', expect.objectContaining({
        project_id: 818,
        sandbox: false,
      }))
    })
    expect(mockStartFullStack).not.toHaveBeenCalled()
  })

  it('falls back to frontend-only preview when fullstack startup fails', async () => {
    const mockPost = apiService.client.post as any
    const mockGet = apiService.client.get as any
    const mockStartFullStack = (apiService as any).startFullStackPreview as any

    mockGet.mockImplementation(async (url: string) => {
      if (url === '/preview/docker/status') return { data: { available: false } }
      if (url === '/preview/bundler/status') return { data: { available: true } }
      if (url.startsWith('/preview/status/')) return { data: { preview: { active: false } } }
      if (url.startsWith('/preview/server/detect/')) {
        return { data: { has_backend: true, framework: 'express', server_type: 'node', command: 'npm', entry_file: 'server.js' } }
      }
      if (url.startsWith('/preview/server/status/')) return { data: { server: { running: false, ready: false } } }
      if (url.startsWith('/preview/server/logs/')) return { data: { stdout: '', stderr: '' } }
      throw new Error(`Unexpected GET ${url}`)
    })

    mockStartFullStack.mockRejectedValueOnce({
      response: {
        status: 500,
        data: { error: 'backend runtime failed before frontend preview was attached' },
      },
    })

    const view = render(<LivePreview projectId={819} autoStart className="h-96" />)

    await waitFor(() => {
      expect(mockPost).toHaveBeenCalledWith('/preview/start', {
        project_id: 819,
        sandbox: false,
      })
    })
    expect(await within(view.container).findByTitle('Live Preview')).toBeTruthy()
    expect(within(view.container).queryByText(/backend runtime failed/i)).toBeNull()
  })

  it('clears a prior failed-start error when a degraded preview starts successfully', async () => {
    const mockPost = apiService.client.post as any

    mockPost
      .mockRejectedValueOnce({
        response: {
          status: 500,
          data: { error: 'Failed to start preview' },
        },
      })
      .mockResolvedValueOnce({
        data: {
          sandbox: false,
          degraded: true,
          diagnostics: {
            backend_started: false,
            backend_error: 'no backend server detected in project',
          },
          preview: {
            project_id: 820,
            active: true,
            port: 3000,
            url: 'http://localhost:3000/project-820',
            started_at: new Date().toISOString(),
            last_access: new Date().toISOString(),
            connected_clients: 1,
          },
        },
      })

    const view = render(<LivePreview projectId={820} className="h-96" />)

    fireEvent.click(within(view.container).getAllByRole('button', { name: /start preview/i })[0])
    expect(await within(view.container).findByText(/Failed to start preview/i)).toBeTruthy()

    fireEvent.click(within(view.container).getAllByRole('button', { name: /start preview/i })[0])

    expect(await within(view.container).findByTitle('Live Preview')).toBeTruthy()
    expect(within(view.container).queryByText(/Failed to start preview/i)).toBeNull()
  })

  it('keeps the iframe visible when optional backend startup degrades', async () => {
    const mockGet = apiService.client.get as any
    const mockStartFullStack = (apiService as any).startFullStackPreview as any

    mockGet.mockImplementation(async (url: string) => {
      if (url === '/preview/docker/status') {
        return {
          data: {
            available: false,
            backend_preview_available: true,
          },
        }
      }
      if (url === '/preview/bundler/status') return { data: { available: true } }
      if (url.startsWith('/preview/status/')) return { data: { preview: { active: false } } }
      if (url.startsWith('/preview/server/detect/')) {
        return { data: { has_backend: true, framework: 'express', server_type: 'node', command: 'npm', entry_file: 'server.js' } }
      }
      if (url.startsWith('/preview/server/status/')) return { data: { server: { running: false, ready: false } } }
      if (url.startsWith('/preview/server/logs/')) return { data: { stdout: '', stderr: '' } }
      throw new Error(`Unexpected GET ${url}`)
    })

    mockStartFullStack.mockImplementation(async (data: any) => ({
      sandbox: false,
      degraded: true,
      diagnostics: {
        backend_started: false,
        backend_error: 'optional backend did not become ready before preview timeout',
      },
      server: { running: false, ready: false },
      preview: {
        project_id: data.project_id,
        active: true,
        port: 3000,
        url: `http://localhost:3000/project-${data.project_id}`,
        started_at: new Date().toISOString(),
        last_access: new Date().toISOString(),
        connected_clients: 1,
      },
    }))

    const view = render(<LivePreview projectId={909} autoStart className="h-96" />)

    await waitFor(() => {
      expect(mockStartFullStack).toHaveBeenCalledWith(expect.objectContaining({
        project_id: 909,
        start_backend: true,
        require_backend: false,
      }))
    })

    expect(await within(view.container).findByTitle('Live Preview')).toBeTruthy()
    expect(within(view.container).queryByText(/optional backend did not become ready/i)).toBeNull()
    expect(within(view.container).getByText('Backend Down')).toBeTruthy()
  })

  it('renders the preview iframe with same-origin sandbox support', async () => {
    const mockPost = apiService.client.post as any
    const mockGet = apiService.client.get as any
    let previewRunning = false

    mockGet.mockImplementation(async (url: string) => {
      if (url === '/preview/docker/status') return { data: { available: false } }
      if (url === '/preview/bundler/status') return { data: { available: true } }
      if (url.startsWith('/preview/status/')) {
        return {
          data: {
            preview: previewRunning
              ? {
                project_id: 707,
                active: true,
                port: 3000,
                url: 'http://localhost:3000/project-707',
                started_at: new Date().toISOString(),
                last_access: new Date().toISOString(),
                connected_clients: 1,
              }
              : { active: false },
          },
        }
      }
      if (url.startsWith('/preview/server/detect/')) return { data: { has_backend: false } }
      if (url.startsWith('/preview/server/status/')) return { data: { server: null } }
      if (url.startsWith('/preview/server/logs/')) return { data: { stdout: '', stderr: '' } }
      throw new Error(`Unexpected GET ${url}`)
    })
    mockPost.mockImplementation(async (url: string, data: any) => {
      if (url === '/preview/stop' || url === '/preview/refresh') return { data: {} }
      if (url !== '/preview/start') throw new Error(`Unexpected POST ${url}`)
      previewRunning = true
      return {
        data: {
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
        },
      }
    })

    const view = render(<LivePreview projectId={707} autoStart className="h-96" />)

    await waitFor(() => {
      expect(mockPost).toHaveBeenCalledWith('/preview/start', expect.objectContaining({ project_id: 707 }))
    })

    const iframe = await within(view.container).findByTitle('Live Preview')
    expect(iframe.getAttribute('sandbox')).toContain('allow-same-origin')
  })
})
