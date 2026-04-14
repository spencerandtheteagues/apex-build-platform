import { describe, expect, it } from 'vitest'
import {
  deriveBrowserLocalPreviewCapability,
  deriveBrowserLocalPreviewRoute,
  derivePreviewRuntimeState,
} from './previewState'

const activePreview = {
  project_id: 1,
  active: true,
  port: 5173,
  url: 'http://localhost:5173',
  started_at: '2026-04-13T00:00:00Z',
  last_access: '2026-04-13T00:00:00Z',
  connected_clients: 1,
}

describe('derivePreviewRuntimeState', () => {
  it('classifies starting before an active preview exists', () => {
    expect(derivePreviewRuntimeState({ loading: true, status: null })).toBe('starting')
  })

  it('keeps active frontend-only previews running', () => {
    expect(derivePreviewRuntimeState({ loading: false, status: activePreview })).toBe('running')
  })

  it('classifies backend-down separately from a hard preview failure', () => {
    expect(
      derivePreviewRuntimeState({
        loading: false,
        status: activePreview,
        serverDetection: { has_backend: true },
        serverStatus: null,
        backendPreviewAvailable: true,
      }),
    ).toBe('backend_down')
  })

  it('classifies sandbox fallback as degraded when the preview is otherwise running', () => {
    expect(
      derivePreviewRuntimeState({
        loading: false,
        status: activePreview,
        sandboxDegraded: true,
      }),
    ).toBe('degraded')
  })

  it('classifies active previews with a transient status disconnect as degraded', () => {
    expect(
      derivePreviewRuntimeState({
        loading: false,
        status: activePreview,
        connected: false,
      }),
    ).toBe('degraded')
  })

  it('classifies visible runtime errors as failed', () => {
    expect(
      derivePreviewRuntimeState({
        loading: false,
        status: activePreview,
        error: 'Preview failed',
      }),
    ).toBe('failed')
  })
})

describe('deriveBrowserLocalPreviewCapability', () => {
  it('marks browser-local preview ready when isolation primitives are present', () => {
    expect(
      deriveBrowserLocalPreviewCapability({
        secureContext: true,
        crossOriginIsolated: true,
        sharedArrayBufferAvailable: true,
        webAssemblyAvailable: true,
      }).state,
    ).toBe('ready')
  })

  it('keeps missing isolation separate from unsupported browsers', () => {
    const capability = deriveBrowserLocalPreviewCapability({
      secureContext: true,
      crossOriginIsolated: false,
      sharedArrayBufferAvailable: false,
      webAssemblyAvailable: true,
    })

    expect(capability.state).toBe('needs_isolation')
    expect(capability.blockers).toContain('COOP/COEP isolation missing')
    expect(capability.blockers).toContain('SharedArrayBuffer unavailable')
  })

  it('marks insecure contexts unsupported for browser-local preview', () => {
    const capability = deriveBrowserLocalPreviewCapability({
      secureContext: false,
      crossOriginIsolated: true,
      sharedArrayBufferAvailable: true,
      webAssemblyAvailable: true,
    })

    expect(capability.state).toBe('unsupported')
    expect(capability.blockers).toContain('Secure context unavailable')
  })
})

describe('deriveBrowserLocalPreviewRoute', () => {
  const readyCapability = deriveBrowserLocalPreviewCapability({
    secureContext: true,
    crossOriginIsolated: true,
    sharedArrayBufferAvailable: true,
    webAssemblyAvailable: true,
  })
  const blockedCapability = deriveBrowserLocalPreviewCapability({
    secureContext: true,
    crossOriginIsolated: false,
    sharedArrayBufferAvailable: false,
    webAssemblyAvailable: true,
  })

  it('keeps backend projects on the platform runtime', () => {
    expect(
      deriveBrowserLocalPreviewRoute({
        serverDetection: { has_backend: true, framework: 'express' },
        bundlerAvailable: true,
        capability: readyCapability,
      }).state,
    ).toBe('platform_runtime')
  })

  it('marks frontend-only projects as browser-local eligible when prerequisites are ready but runtime is disabled', () => {
    expect(
      deriveBrowserLocalPreviewRoute({
        serverDetection: { has_backend: false },
        bundlerAvailable: true,
        capability: readyCapability,
      }).state,
    ).toBe('browser_local_eligible')
  })

  it('marks frontend-only projects as browser-local active only when the runtime is enabled', () => {
    const route = deriveBrowserLocalPreviewRoute({
      serverDetection: { has_backend: false },
      bundlerAvailable: true,
      capability: readyCapability,
      browserLocalRuntimeEnabled: true,
    })

    expect(route.state).toBe('browser_local_active')
    expect(route.reason).toContain('enabled browser-local runtime')
  })

  it('blocks frontend-only browser-local routing when isolation is missing', () => {
    const route = deriveBrowserLocalPreviewRoute({
      serverDetection: { has_backend: false },
      bundlerAvailable: true,
      capability: blockedCapability,
    })

    expect(route.state).toBe('browser_local_blocked')
    expect(route.reason).toContain('cross-origin isolation')
  })
})
