import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { AlertTriangle, Box, ExternalLink, PackageCheck, RefreshCw, ShieldCheck, Smartphone, XCircle } from 'lucide-react'

import { Button, Loading } from '@/components/ui'
import { getApiErrorMessage } from '@/lib/errors'
import { cn, formatRelativeTime } from '@/lib/utils'
import type { Notification } from '@/types'
import type {
  ApiService,
  MobileBuildJob,
  MobileBuildProfile,
  MobileBuildRequest,
  MobileBuildStatus,
  MobileCredentialStatus,
  MobilePlatform,
  MobileReleaseLevel,
} from '@/services/api'
import MobileCredentialPanel from './MobileCredentialPanel'

type Notify = (notification: Omit<Notification, 'id' | 'timestamp'>) => void

interface MobileBuildOperationsPanelProps {
  projectId: number
  mobilePlatforms: readonly string[]
  mobileBuildStatus?: string
  apiService: Pick<
    ApiService,
    | 'listProjectMobileBuilds'
    | 'createProjectMobileBuild'
    | 'refreshProjectMobileBuild'
    | 'cancelProjectMobileBuild'
    | 'retryProjectMobileBuild'
    | 'getProjectMobileCredentials'
    | 'createProjectMobileCredential'
    | 'deleteProjectMobileCredential'
    | 'startProjectMobileExpoWebPreview'
  >
  addNotification: Notify
}

interface MobileBuildTarget {
  id: string
  label: string
  platform: MobilePlatform
  profile: MobileBuildProfile
  release_level: MobileReleaseLevel
  description: string
}

const buildStatusLabels: Record<MobileBuildStatus, string> = {
  queued: 'Queued',
  preparing: 'Preparing',
  validating: 'Validating',
  uploading: 'Uploading',
  building: 'Building',
  signing: 'Signing',
  succeeded: 'Succeeded',
  failed: 'Failed',
  canceled: 'Canceled',
  repair_pending: 'Repair pending',
  repaired_retry_pending: 'Retry pending',
}

const buildStatusClasses: Record<MobileBuildStatus, string> = {
  queued: 'border-cyan-300/20 bg-cyan-300/10 text-cyan-100',
  preparing: 'border-cyan-300/20 bg-cyan-300/10 text-cyan-100',
  validating: 'border-cyan-300/20 bg-cyan-300/10 text-cyan-100',
  uploading: 'border-blue-300/20 bg-blue-300/10 text-blue-100',
  building: 'border-blue-300/20 bg-blue-300/10 text-blue-100',
  signing: 'border-amber-300/20 bg-amber-300/10 text-amber-100',
  succeeded: 'border-emerald-300/20 bg-emerald-300/10 text-emerald-100',
  failed: 'border-red-300/20 bg-red-300/10 text-red-100',
  canceled: 'border-white/10 bg-white/[0.03] text-gray-300',
  repair_pending: 'border-amber-300/20 bg-amber-300/10 text-amber-100',
  repaired_retry_pending: 'border-amber-300/20 bg-amber-300/10 text-amber-100',
}

const releaseLevelLabels: Record<MobileReleaseLevel, string> = {
  source_only: 'Source only',
  web_preview: 'Browser preview',
  dev_build: 'Development build',
  internal_android_apk: 'Android APK',
  android_aab: 'Android AAB',
  ios_simulator: 'iOS simulator',
  ios_internal: 'iOS internal',
  testflight_ready: 'TestFlight-ready',
  store_submission_ready: 'Store submission-ready',
}

const terminalStatuses = new Set<MobileBuildStatus>(['succeeded', 'failed', 'canceled'])

const normalizeMobilePlatforms = (platforms: readonly string[]): MobilePlatform[] => {
  const normalized = new Set<MobilePlatform>()
  for (const platform of platforms) {
    if (platform === 'android' || platform === 'ios') {
      normalized.add(platform)
    }
  }
  if (normalized.size === 0) {
    normalized.add('android')
    normalized.add('ios')
  }
  return Array.from(normalized)
}

const buildTargetsForPlatforms = (platforms: readonly string[]): MobileBuildTarget[] => {
  const normalized = normalizeMobilePlatforms(platforms)
  const targets: MobileBuildTarget[] = []
  if (normalized.includes('android')) {
    targets.push({
      id: 'android-apk',
      label: 'Start Android APK',
      platform: 'android',
      profile: 'preview',
      release_level: 'internal_android_apk',
      description: 'Internal installable APK via EAS Build when server flags and EAS credentials are configured.',
    })
    targets.push({
      id: 'android-aab',
      label: 'Start Android AAB',
      platform: 'android',
      profile: 'production',
      release_level: 'android_aab',
      description: 'Production Android App Bundle artifact. Store upload remains a separate gated workflow.',
    })
  }
  if (normalized.includes('ios')) {
    targets.push({
      id: 'ios-internal',
      label: 'Start iOS Internal',
      platform: 'ios',
      profile: 'internal',
      release_level: 'ios_internal',
      description: 'Internal iOS build through EAS when Apple credentials and iOS build flags are configured.',
    })
  }
  return targets
}

const upsertBuild = (builds: MobileBuildJob[], next: MobileBuildJob) => {
  const merged = [next, ...builds.filter((build) => build.id !== next.id)]
  return merged.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
}

const extractCredentialsFromError = (error: unknown): MobileCredentialStatus | null => {
  const response = typeof error === 'object' && error !== null && 'response' in error
    ? (error as { response?: { data?: { credentials?: MobileCredentialStatus } } }).response
    : undefined
  return response?.data?.credentials ?? null
}

const formatPlatform = (platform: MobilePlatform) => platform === 'ios' ? 'iOS' : 'Android'

const formatBuildTitle = (build: MobileBuildJob) =>
  `${formatPlatform(build.platform)} ${releaseLevelLabels[build.release_level] || build.release_level}`

const latestLogMessage = (build: MobileBuildJob) => {
  const logs = build.logs || []
  return logs.length > 0 ? logs[logs.length - 1]?.message : null
}

export const MobileBuildOperationsPanel: React.FC<MobileBuildOperationsPanelProps> = ({
  projectId,
  mobilePlatforms,
  mobileBuildStatus,
  apiService,
  addNotification,
}) => {
  const [builds, setBuilds] = useState<MobileBuildJob[]>([])
  const [credentials, setCredentials] = useState<MobileCredentialStatus | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [actionId, setActionId] = useState<string | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [webPreviewUrl, setWebPreviewUrl] = useState<string | null>(null)

  const buildTargets = useMemo(() => buildTargetsForPlatforms(mobilePlatforms), [mobilePlatforms])
  const missingEAS = Boolean(credentials?.missing?.includes('eas_token'))
  const hasEAS = Boolean(credentials?.present?.includes('eas_token'))

  const loadBuildState = useCallback(async () => {
    setIsLoading(true)
    setLoadError(null)
    const [buildResult, credentialResult] = await Promise.allSettled([
      apiService.listProjectMobileBuilds(projectId),
      apiService.getProjectMobileCredentials(projectId),
    ])
    if (buildResult.status === 'fulfilled') {
      setBuilds(buildResult.value)
    } else {
      setBuilds([])
      setLoadError(getApiErrorMessage(buildResult.reason, 'Unable to load native build jobs.'))
    }
    if (credentialResult.status === 'fulfilled') {
      setCredentials(credentialResult.value)
    }
    setIsLoading(false)
  }, [apiService, projectId])

  useEffect(() => {
    void loadBuildState()
  }, [loadBuildState])

  const handleCreateBuild = async (target: MobileBuildTarget) => {
    const request: MobileBuildRequest = {
      platform: target.platform,
      profile: target.profile,
      release_level: target.release_level,
    }
    setActionId(target.id)
    try {
      const response = await apiService.createProjectMobileBuild(projectId, request)
      setBuilds((current) => upsertBuild(current, response.build))
      if (response.credentials) {
        setCredentials(response.credentials)
      }
      addNotification({
        type: 'success',
        title: 'Native build queued',
        message: `${formatPlatform(target.platform)} build request was accepted by Apex.`,
      })
    } catch (error) {
      const credentialStatus = extractCredentialsFromError(error)
      if (credentialStatus) {
        setCredentials(credentialStatus)
      }
      addNotification({
        type: 'error',
        title: 'Native build blocked',
        message: getApiErrorMessage(error, 'Apex could not start this native build.'),
      })
    } finally {
      setActionId(null)
    }
  }

  const handleRefreshBuild = async (build: MobileBuildJob) => {
    setActionId(`refresh-${build.id}`)
    try {
      const refreshed = await apiService.refreshProjectMobileBuild(projectId, build.id)
      setBuilds((current) => upsertBuild(current, refreshed))
      addNotification({
        type: 'success',
        title: 'Build refreshed',
        message: `${formatBuildTitle(refreshed)} is now ${buildStatusLabels[refreshed.status] || refreshed.status}.`,
      })
    } catch (error) {
      addNotification({
        type: 'error',
        title: 'Refresh failed',
        message: getApiErrorMessage(error, 'Unable to refresh this mobile build.'),
      })
    } finally {
      setActionId(null)
    }
  }

  const handleCancelBuild = async (build: MobileBuildJob) => {
    setActionId(`cancel-${build.id}`)
    try {
      const canceled = await apiService.cancelProjectMobileBuild(projectId, build.id)
      setBuilds((current) => upsertBuild(current, canceled))
      addNotification({
        type: 'success',
        title: 'Build cancel requested',
        message: `${formatBuildTitle(canceled)} was marked ${buildStatusLabels[canceled.status] || canceled.status}.`,
      })
    } catch (error) {
      addNotification({
        type: 'error',
        title: 'Cancel failed',
        message: getApiErrorMessage(error, 'Unable to cancel this mobile build.'),
      })
    } finally {
      setActionId(null)
    }
  }

  const handleRetryBuild = async (build: MobileBuildJob) => {
    setActionId(`retry-${build.id}`)
    try {
      const response = await apiService.retryProjectMobileBuild(projectId, build.id)
      setBuilds((current) => upsertBuild(current, response.build))
      if (response.credentials) {
        setCredentials(response.credentials)
      }
      addNotification({
        type: 'success',
        title: 'Build retry queued',
        message: `${formatBuildTitle(response.build)} retry was accepted by Apex.`,
      })
    } catch (error) {
      const credentialStatus = extractCredentialsFromError(error)
      if (credentialStatus) {
        setCredentials(credentialStatus)
      }
      addNotification({
        type: 'error',
        title: 'Retry blocked',
        message: getApiErrorMessage(error, 'Unable to retry this mobile build.'),
      })
    } finally {
      setActionId(null)
    }
  }

  const handleStartWebPreview = async () => {
    setActionId('expo-web-preview')
    try {
      const response = await apiService.startProjectMobileExpoWebPreview(projectId)
      const previewUrl = response.preview_url || response.url || ''
      setWebPreviewUrl(previewUrl)
      addNotification({
        type: 'success',
        title: 'Expo Web preview started',
        message: response.message || 'Browser-rendered mobile preview is available. This is not a native Android/iOS build.',
      })
    } catch (error) {
      addNotification({
        type: 'error',
        title: 'Expo Web preview failed',
        message: getApiErrorMessage(error, 'Unable to start the browser-rendered mobile preview.'),
      })
    } finally {
      setActionId(null)
    }
  }

  const handleOpenArtifact = (build: MobileBuildJob) => {
    if (!build.artifact_url) return
    window.open(build.artifact_url, '_blank', 'noopener,noreferrer')
  }

  return (
    <section
      className="rounded-[28px] border border-blue-300/16 bg-[linear-gradient(180deg,rgba(12,18,30,0.94)_0%,rgba(5,8,14,0.98)_100%)] shadow-[0_24px_80px_rgba(0,0,0,0.28)]"
      data-testid="mobile-build-operations"
    >
      <div className="grid gap-0 lg:grid-cols-[minmax(0,1.08fr)_minmax(340px,0.92fr)]">
        <div className="relative overflow-hidden px-6 py-6 md:px-8">
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(96,165,250,0.16),transparent_34%),radial-gradient(circle_at_bottom_right,rgba(16,185,129,0.10),transparent_30%)]" />
          <div className="relative">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-blue-300/20 bg-blue-300/10 text-blue-100">
                  <PackageCheck className="h-6 w-6" />
                </div>
                <div>
                  <div className="text-[11px] font-semibold uppercase tracking-[0.28em] text-blue-200/80">
                    Native Build Pipeline
                  </div>
                  <h2 className="mt-1 text-2xl font-semibold tracking-[-0.03em] text-white">
                    Android/iOS build controls
                  </h2>
                </div>
              </div>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                onClick={() => void loadBuildState()}
                disabled={isLoading || Boolean(actionId)}
                className="rounded-2xl border border-white/10 bg-white/[0.03] text-gray-100 hover:bg-white/[0.06]"
              >
                <RefreshCw className={cn('mr-2 h-4 w-4', isLoading && 'animate-spin')} />
                Reload
              </Button>
            </div>

            <p className="mt-5 max-w-3xl text-sm leading-7 text-gray-300">
              These controls call the server-side mobile build API. Apex still separates source export, preview, native binaries,
              store metadata, upload, and store approval into separate statuses.
            </p>

            <div className="mt-5 grid gap-3 md:grid-cols-3">
              <div className="rounded-2xl border border-white/8 bg-black/24 px-4 py-4">
                <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.22em] text-gray-500">
                  <ShieldCheck className="h-3.5 w-3.5" />
                  EAS token
                </div>
                <div className={cn('mt-2 text-sm font-semibold', hasEAS ? 'text-emerald-100' : 'text-amber-100')}>
                  {hasEAS ? 'Stored for builds' : 'Required before queueing'}
                </div>
              </div>
              <div className="rounded-2xl border border-white/8 bg-black/24 px-4 py-4">
                <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.22em] text-gray-500">
                  <Smartphone className="h-3.5 w-3.5" />
                  Platforms
                </div>
                <div className="mt-2 text-sm font-semibold text-white">
                  {normalizeMobilePlatforms(mobilePlatforms).map(formatPlatform).join(' + ')}
                </div>
              </div>
              <div className="rounded-2xl border border-white/8 bg-black/24 px-4 py-4">
                <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.22em] text-gray-500">
                  <Box className="h-3.5 w-3.5" />
                  Project status
                </div>
                <div className="mt-2 text-sm font-semibold text-white">
                  {mobileBuildStatus ? buildStatusLabels[mobileBuildStatus as MobileBuildStatus] || mobileBuildStatus : 'No native build yet'}
                </div>
              </div>
            </div>

            {credentials?.blockers?.length || missingEAS ? (
              <div className="mt-4 rounded-2xl border border-amber-300/14 bg-amber-300/8 px-4 py-3">
                <div className="flex items-start gap-3">
                  <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-200" />
                  <div className="text-xs leading-5 text-amber-50/90">
                    {missingEAS
                      ? 'Add an EAS token in mobile credentials before starting native builds.'
                      : credentials?.blockers?.[0]}
                  </div>
                </div>
              </div>
            ) : null}

            <MobileCredentialPanel
              projectId={projectId}
              credentials={credentials}
              apiService={apiService}
              addNotification={addNotification}
              onCredentialsChange={setCredentials}
            />

            <div className="mt-5 rounded-2xl border border-cyan-300/14 bg-cyan-300/8 px-4 py-4" data-testid="mobile-expo-web-preview-card">
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div className="max-w-2xl">
                  <div className="text-sm font-semibold text-white">Expo Web preview</div>
                  <p className="mt-2 text-xs leading-5 text-gray-300">
                    Starts `npm run web` from the generated `mobile/` Expo source and proxies it in Apex.
                    This is browser-rendered and does not prove a native APK/AAB, TestFlight, or store build.
                  </p>
                  {webPreviewUrl ? (
                    <div className="mt-2 truncate text-xs text-cyan-100">{webPreviewUrl}</div>
                  ) : null}
                </div>
                <div className="flex shrink-0 flex-wrap gap-2">
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    onClick={() => void handleStartWebPreview()}
                    disabled={Boolean(actionId)}
                    className="rounded-xl border border-cyan-300/14 bg-cyan-300/10 px-3 text-xs text-cyan-50"
                  >
                    <Smartphone className="mr-2 h-3.5 w-3.5" />
                    {actionId === 'expo-web-preview' ? 'Starting...' : 'Start Expo Web Preview'}
                  </Button>
                  {webPreviewUrl ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      onClick={() => window.open(webPreviewUrl, '_blank', 'noopener,noreferrer')}
                      className="rounded-xl border border-white/10 bg-white/[0.03] px-3 text-xs text-gray-100"
                    >
                      <ExternalLink className="mr-2 h-3.5 w-3.5" />
                      Open
                    </Button>
                  ) : null}
                </div>
              </div>
            </div>

            <div className="mt-5 grid gap-3 md:grid-cols-3">
              {buildTargets.map((target) => (
                <button
                  key={target.id}
                  type="button"
                  onClick={() => void handleCreateBuild(target)}
                  disabled={isLoading || Boolean(actionId) || missingEAS}
                  className={cn(
                    'rounded-2xl border border-blue-300/16 bg-blue-300/8 px-4 py-4 text-left transition hover:border-blue-200/30 hover:bg-blue-300/12',
                    (isLoading || Boolean(actionId) || missingEAS) && 'cursor-not-allowed opacity-55 hover:border-blue-300/16 hover:bg-blue-300/8',
                  )}
                >
                  <div className="text-sm font-semibold text-white">{target.label}</div>
                  <div className="mt-2 text-xs leading-5 text-gray-300">{target.description}</div>
                  {actionId === target.id ? (
                    <div className="mt-3 text-xs font-medium text-cyan-100">Submitting...</div>
                  ) : null}
                </button>
              ))}
            </div>
          </div>
        </div>

        <div className="border-t border-white/6 bg-black/22 p-6 lg:border-l lg:border-t-0">
          <div className="mb-4 flex items-center justify-between gap-3">
            <div>
              <div className="text-[11px] font-semibold uppercase tracking-[0.26em] text-blue-200/80">Build Jobs</div>
              <div className="mt-1 text-sm text-gray-400">{builds.length} recorded</div>
            </div>
          </div>

          {isLoading ? (
            <div className="rounded-2xl border border-white/8 bg-black/24 px-4 py-8">
              <Loading variant="dots" color="cyberpunk" text="Loading native build jobs..." />
            </div>
          ) : loadError ? (
            <div className="rounded-2xl border border-red-300/14 bg-red-300/8 px-4 py-4 text-sm leading-6 text-red-50">
              {loadError}
            </div>
          ) : builds.length > 0 ? (
            <div className="space-y-3">
              {builds.slice(0, 5).map((build) => {
                const canCancel = !terminalStatuses.has(build.status) && Boolean(build.provider_build_id)
                const canRetry = build.status === 'failed' || build.status === 'canceled' || build.status === 'repair_pending' || build.status === 'repaired_retry_pending'
                const logMessage = latestLogMessage(build)
                return (
                  <div key={build.id} className="rounded-2xl border border-white/8 bg-black/24 px-4 py-4" data-testid={`mobile-build-job-${build.id}`}>
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <div className="text-sm font-semibold text-white">{formatBuildTitle(build)}</div>
                        <div className="mt-1 text-xs text-gray-500">
                          {build.provider_build_id ? `Provider build ${build.provider_build_id}` : 'Waiting for provider build ID'}
                        </div>
                      </div>
                      <span className={cn('shrink-0 rounded-full border px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.16em]', buildStatusClasses[build.status] || buildStatusClasses.queued)}>
                        {buildStatusLabels[build.status] || build.status}
                      </span>
                    </div>

                    <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-gray-400">
                      <span>{build.profile}</span>
                      <span>{formatRelativeTime(build.updated_at)}</span>
                      {build.artifact_url ? <span className="text-emerald-200">Artifact ready</span> : null}
                    </div>

                    {build.failure_message ? (
                      <div className="mt-3 rounded-xl border border-red-300/12 bg-red-300/8 px-3 py-2 text-xs leading-5 text-red-50/90">
                        {build.failure_message}
                      </div>
                    ) : logMessage ? (
                      <div className="mt-3 rounded-xl border border-white/8 bg-black/22 px-3 py-2 text-xs leading-5 text-gray-300">
                        {logMessage}
                      </div>
                    ) : null}

                    <div className="mt-4 flex flex-wrap gap-2">
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        onClick={() => void handleRefreshBuild(build)}
                        disabled={Boolean(actionId)}
                        className="rounded-xl border border-white/10 bg-white/[0.03] px-3 text-xs text-gray-100"
                      >
                        <RefreshCw className={cn('mr-2 h-3.5 w-3.5', actionId === `refresh-${build.id}` && 'animate-spin')} />
                        Refresh
                      </Button>
                      {canCancel ? (
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          onClick={() => void handleCancelBuild(build)}
                          disabled={Boolean(actionId)}
                          className="rounded-xl border border-red-300/14 bg-red-300/8 px-3 text-xs text-red-50"
                        >
                          <XCircle className="mr-2 h-3.5 w-3.5" />
                          Cancel
                        </Button>
                      ) : null}
                      {canRetry ? (
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          onClick={() => void handleRetryBuild(build)}
                          disabled={Boolean(actionId) || missingEAS}
                          className="rounded-xl border border-cyan-300/14 bg-cyan-300/8 px-3 text-xs text-cyan-50"
                        >
                          <RefreshCw className={cn('mr-2 h-3.5 w-3.5', actionId === `retry-${build.id}` && 'animate-spin')} />
                          Retry
                        </Button>
                      ) : null}
                      {build.artifact_url ? (
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          onClick={() => handleOpenArtifact(build)}
                          className="rounded-xl border border-emerald-300/14 bg-emerald-300/8 px-3 text-xs text-emerald-50"
                        >
                          <ExternalLink className="mr-2 h-3.5 w-3.5" />
                          Artifact
                        </Button>
                      ) : null}
                    </div>
                  </div>
                )
              })}
            </div>
          ) : (
            <div className="rounded-2xl border border-dashed border-white/10 bg-black/18 px-5 py-8 text-center">
              <PackageCheck className="mx-auto h-9 w-9 text-white/20" />
              <p className="mt-3 text-sm font-medium text-white">No native builds yet</p>
              <p className="mt-2 text-xs leading-5 text-gray-400">
                Start with an internal Android APK or iOS internal build after the EAS token is connected.
              </p>
            </div>
          )}
        </div>
      </div>
    </section>
  )
}

export default MobileBuildOperationsPanel
