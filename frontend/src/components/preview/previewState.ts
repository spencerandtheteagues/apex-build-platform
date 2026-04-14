import type { PreviewStatus, ServerDetection, ServerStatus } from './types'

export type PreviewRuntimeState = 'starting' | 'running' | 'degraded' | 'backend_down' | 'failed' | 'stopped'
export type BrowserLocalPreviewCapabilityState = 'ready' | 'needs_isolation' | 'unsupported'
export type BrowserLocalPreviewRouteState =
  | 'platform_runtime'
  | 'browser_local_eligible'
  | 'browser_local_active'
  | 'browser_local_blocked'

export interface PreviewRuntimeStateInput {
  loading: boolean
  status: PreviewStatus | null
  connected?: boolean
  error?: string | null
  iframeError?: string | null
  sandboxDegraded?: boolean
  serverDetection?: ServerDetection | null
  serverStatus?: ServerStatus | null
  backendPreviewAvailable?: boolean
}

export interface BrowserLocalPreviewCapabilityInput {
  secureContext?: boolean
  crossOriginIsolated?: boolean
  sharedArrayBufferAvailable?: boolean
  webAssemblyAvailable?: boolean
}

export interface BrowserLocalPreviewCapability {
  state: BrowserLocalPreviewCapabilityState
  label: string
  reason: string
  blockers: string[]
}

export interface BrowserLocalPreviewRouteInput {
  serverDetection: ServerDetection | null
  bundlerAvailable: boolean
  capability: BrowserLocalPreviewCapability
  browserLocalRuntimeEnabled?: boolean
  browserLocalRuntimeAvailable?: boolean
}

export interface BrowserLocalPreviewRoute {
  state: BrowserLocalPreviewRouteState
  label: string
  reason: string
}

export const previewRuntimeStateLabels: Record<PreviewRuntimeState, string> = {
  starting: 'Starting',
  running: 'Running',
  degraded: 'Degraded',
  backend_down: 'Backend Down',
  failed: 'Failed',
  stopped: 'Stopped',
}

export const browserLocalPreviewCapabilityLabels: Record<BrowserLocalPreviewCapabilityState, string> = {
  ready: 'Ready',
  needs_isolation: 'Needs Isolation',
  unsupported: 'Unsupported',
}

export const browserLocalPreviewRouteLabels: Record<BrowserLocalPreviewRouteState, string> = {
  platform_runtime: 'Platform Runtime',
  browser_local_eligible: 'Browser-Local Eligible',
  browser_local_active: 'Browser-Local Active',
  browser_local_blocked: 'Browser-Local Blocked',
}

export function derivePreviewRuntimeState({
  loading,
  status,
  connected,
  error,
  iframeError,
  sandboxDegraded = false,
  serverDetection,
  serverStatus,
  backendPreviewAvailable = true,
}: PreviewRuntimeStateInput): PreviewRuntimeState {
  if (error || iframeError) return 'failed'
  if (loading && !status?.active) return 'starting'
  if (!status?.active) return 'stopped'
  if (serverDetection?.has_backend && backendPreviewAvailable && !serverStatus?.running) return 'backend_down'
  if (connected === false) return 'degraded'
  if (sandboxDegraded) return 'degraded'
  return 'running'
}

export function deriveBrowserLocalPreviewCapability({
  secureContext = false,
  crossOriginIsolated = false,
  sharedArrayBufferAvailable = false,
  webAssemblyAvailable = true,
}: BrowserLocalPreviewCapabilityInput): BrowserLocalPreviewCapability {
  const blockers: string[] = []
  if (!secureContext) blockers.push('Secure context unavailable')
  if (!crossOriginIsolated) blockers.push('COOP/COEP isolation missing')
  if (!sharedArrayBufferAvailable) blockers.push('SharedArrayBuffer unavailable')
  if (!webAssemblyAvailable) blockers.push('WebAssembly unavailable')

  const isolationOnly = blockers.every((blocker) =>
    blocker === 'COOP/COEP isolation missing' || blocker === 'SharedArrayBuffer unavailable',
  )
  const state: BrowserLocalPreviewCapabilityState = blockers.length === 0
    ? 'ready'
    : isolationOnly
      ? 'needs_isolation'
      : 'unsupported'

  return {
    state,
    label: browserLocalPreviewCapabilityLabels[state],
    reason: state === 'ready'
      ? 'Browser-local preview prerequisites are available.'
      : state === 'needs_isolation'
        ? 'Enable cross-origin isolation before browser-local preview evaluation.'
        : 'Browser-local preview prerequisites are not available in this browser context.',
    blockers,
  }
}

export function detectBrowserLocalPreviewCapability(): BrowserLocalPreviewCapability {
  const runtime = globalThis as typeof globalThis & {
    crossOriginIsolated?: boolean
    SharedArrayBuffer?: typeof SharedArrayBuffer
  }

  return deriveBrowserLocalPreviewCapability({
    secureContext: Boolean(runtime.isSecureContext),
    crossOriginIsolated: Boolean(runtime.crossOriginIsolated),
    sharedArrayBufferAvailable: typeof runtime.SharedArrayBuffer === 'function',
    webAssemblyAvailable: typeof runtime.WebAssembly === 'object',
  })
}

export function deriveBrowserLocalPreviewRoute({
  serverDetection,
  bundlerAvailable,
  capability,
  browserLocalRuntimeEnabled = false,
  browserLocalRuntimeAvailable = false,
}: BrowserLocalPreviewRouteInput): BrowserLocalPreviewRoute {
  if (serverDetection == null) {
    return {
      state: 'platform_runtime',
      label: browserLocalPreviewRouteLabels.platform_runtime,
      reason: 'Waiting for project runtime detection.',
    }
  }
  if (serverDetection.has_backend) {
    return {
      state: 'platform_runtime',
      label: browserLocalPreviewRouteLabels.platform_runtime,
      reason: 'Backend runtime detected; keep API and database execution on the platform preview path.',
    }
  }
  if (!bundlerAvailable) {
    return {
      state: 'platform_runtime',
      label: browserLocalPreviewRouteLabels.platform_runtime,
      reason: 'Project bundler support is unavailable, so keep using platform preview startup.',
    }
  }
  if (capability.state === 'ready') {
    if (!browserLocalRuntimeEnabled || !browserLocalRuntimeAvailable) {
      return {
        state: 'browser_local_eligible',
        label: browserLocalPreviewRouteLabels.browser_local_eligible,
        reason: !browserLocalRuntimeEnabled
          ? 'Frontend-only project is eligible for browser-local preview evaluation, but the runtime adapter is not enabled.'
          : 'Frontend-only project is eligible for browser-local preview evaluation, but no browser-local runtime adapter is bundled.',
      }
    }
    return {
      state: 'browser_local_active',
      label: browserLocalPreviewRouteLabels.browser_local_active,
      reason: 'Frontend-only project can use the enabled browser-local runtime path.',
    }
  }
  return {
    state: 'browser_local_blocked',
    label: browserLocalPreviewRouteLabels.browser_local_blocked,
    reason: capability.reason,
  }
}
