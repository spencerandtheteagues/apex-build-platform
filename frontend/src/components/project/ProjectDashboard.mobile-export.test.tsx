/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { File, Project } from '@/types'
import ProjectDashboard from './ProjectDashboard'

// Vitest in this repo predates vi.hoisted; var keeps the mock factory safely hoistable.
var mockCurrentProject: Project | null = null
var mockFiles: File[] = []
var mockApiService: any
var mockAddNotification: any

vi.mock('@/hooks/useStore', () => ({
  useStore: () => ({
    currentProject: mockCurrentProject,
    files: mockFiles,
    collaborationUsers: [],
    apiService: mockApiService,
    setCurrentProject: vi.fn(),
    addNotification: mockAddNotification,
  }),
}))

const baseProject: Project = {
  id: 42,
  name: 'Apex FieldOps Mobile',
  description: 'Mobile field operations workspace',
  language: 'TypeScript',
  framework: 'React',
  owner_id: 1,
  owner: {
    id: 1,
    username: 'admin',
    email: 'admin@example.com',
    is_active: true,
    is_verified: true,
    subscription_type: 'pro',
    monthly_ai_requests: 0,
    monthly_ai_cost: 0,
    preferred_theme: 'cyberpunk',
    preferred_ai: 'auto',
    created_at: '2026-05-01T00:00:00.000Z',
    updated_at: '2026-05-01T00:00:00.000Z',
  },
  is_public: false,
  is_archived: false,
  root_directory: '/',
  created_at: '2026-05-01T00:00:00.000Z',
  updated_at: '2026-05-01T00:00:00.000Z',
}

describe('ProjectDashboard mobile export visibility', () => {
  beforeEach(() => {
    mockCurrentProject = null
    mockFiles = []
    mockAddNotification = vi.fn()
    mockApiService = {
      getProject: vi.fn(),
      getFiles: vi.fn(),
      getExecutionHistory: vi.fn(),
      getAIUsage: vi.fn(),
      getProjectMobileValidation: vi.fn().mockResolvedValue({
        status: 'passed',
        summary: 'Mobile source package passed validation.',
        store_readiness_state: 'draft_ready_needs_manual_store_assets',
        checks: [
          {
            id: 'required_files',
            label: 'Required mobile files',
            status: 'passed',
            detail: 'All required files are present.',
            required: true,
          },
        ],
      }),
      getProjectMobileScorecard: vi.fn().mockResolvedValue({
        overall_score: 61,
        target_score: 95,
        is_ready: false,
        summary: 'Mobile readiness is 61% toward the 95% launch-readiness target.',
        categories: [
          { id: 'source_generation', label: 'Expo source generation', score: 100, target: 95, status: 'complete', summary: 'Generated Expo project file coverage.' },
          { id: 'native_artifacts', label: 'Native artifacts', score: 0, target: 95, status: 'blocked', summary: 'Signed Android/iOS artifact proof.' },
        ],
        blockers: ['Produce a signed Android APK/AAB artifact through EAS Build.'],
      }),
      getProjectMobileStoreReadiness: vi.fn().mockResolvedValue({
        status: 'draft_ready_needs_manual_store_assets',
        package_path: 'mobile/store/store-readiness.json',
        validation_status: 'passed',
        store_readiness_state: 'draft_ready_needs_manual_store_assets',
        score: 75,
        target: 95,
        ready_for_submission: false,
        summary: 'Draft store-readiness package is valid, but native artifacts remain separate gates.',
        missing_items: ['Signed Android/iOS build artifacts produced by a successful native build job.'],
        truthful_status_notes: ['This package is a draft for launch preparation, not proof of App Store or Google Play approval.'],
      }),
      getProjectMobileCredentials: vi.fn().mockResolvedValue({
        status: 'validated',
        complete: true,
        required: ['eas_token'],
        present: ['eas_token'],
        missing: [],
        metadata: [
          {
            type: 'eas_token',
            secret_id: 91,
            project_id: 42,
            status: 'stored',
            label: 'EAS token',
            created_at: '2026-05-01T00:00:00.000Z',
            updated_at: '2026-05-01T00:00:00.000Z',
          },
        ],
      }),
      createProjectMobileCredential: vi.fn().mockResolvedValue({
        status: 'validated',
        complete: true,
        required: ['eas_token'],
        present: ['eas_token'],
        missing: [],
        metadata: [
          {
            type: 'eas_token',
            secret_id: 92,
            project_id: 42,
            status: 'stored',
            label: 'EAS token',
            created_at: '2026-05-01T00:00:00.000Z',
            updated_at: '2026-05-01T00:20:00.000Z',
          },
        ],
      }),
      deleteProjectMobileCredential: vi.fn().mockResolvedValue({
        status: 'missing',
        complete: false,
        required: ['eas_token'],
        present: [],
        missing: ['eas_token'],
        metadata: [],
        blockers: ['Add EAS token.'],
      }),
      listProjectMobileBuilds: vi.fn().mockResolvedValue([
        {
          id: 'mbld_existing',
          project_id: 42,
          user_id: 1,
          platform: 'android',
          profile: 'preview',
          release_level: 'internal_android_apk',
          status: 'building',
          provider: 'eas',
          provider_build_id: 'eas-build-1',
          created_at: '2026-05-01T00:00:00.000Z',
          updated_at: '2026-05-01T00:05:00.000Z',
          logs: [{ timestamp: '2026-05-01T00:05:00.000Z', level: 'info', message: 'EAS build is running.' }],
        },
      ]),
      listProjectMobileSubmissions: vi.fn().mockResolvedValue([]),
      createProjectMobileBuild: vi.fn().mockResolvedValue({
        build: {
          id: 'mbld_created',
          project_id: 42,
          user_id: 1,
          platform: 'android',
          profile: 'preview',
          release_level: 'internal_android_apk',
          status: 'queued',
          provider: 'eas',
          provider_build_id: 'eas-build-created',
          created_at: '2026-05-01T00:10:00.000Z',
          updated_at: '2026-05-01T00:10:00.000Z',
        },
      }),
      refreshProjectMobileBuild: vi.fn().mockResolvedValue({
        id: 'mbld_existing',
        project_id: 42,
        user_id: 1,
        platform: 'android',
        profile: 'preview',
        release_level: 'internal_android_apk',
        status: 'succeeded',
        provider: 'eas',
        provider_build_id: 'eas-build-1',
        artifact_url: 'https://artifacts.example.com/app.apk',
        created_at: '2026-05-01T00:00:00.000Z',
        updated_at: '2026-05-01T00:15:00.000Z',
      }),
      cancelProjectMobileBuild: vi.fn(),
      repairProjectMobileBuild: vi.fn().mockResolvedValue({
        build: {
          id: 'mbld_failed',
          project_id: 42,
          user_id: 1,
          platform: 'android',
          profile: 'preview',
          release_level: 'internal_android_apk',
          status: 'repaired_retry_pending',
          provider: 'eas',
          provider_build_id: 'eas-build-failed',
          failure_message: 'Metro bundle failed.',
          repair_plan: {
            failure_type: 'metro_bundle_failed',
            title: 'Repair Metro bundling',
            summary: 'Source validation passed; retry can be queued.',
            retry_recommended: true,
            requires_source_change: true,
            actions: [],
          },
          created_at: '2026-05-01T00:00:00.000Z',
          updated_at: '2026-05-01T00:12:00.000Z',
        },
        repaired: true,
      }),
      retryProjectMobileBuild: vi.fn().mockResolvedValue({
        build: {
          id: 'mbld_retry',
          project_id: 42,
          user_id: 1,
          platform: 'android',
          profile: 'preview',
          release_level: 'internal_android_apk',
          status: 'queued',
          provider: 'eas',
          provider_build_id: 'eas-build-retry',
          created_at: '2026-05-01T00:20:00.000Z',
          updated_at: '2026-05-01T00:20:00.000Z',
        },
      }),
      submitProjectMobileBuild: vi.fn().mockResolvedValue({
        submission: {
          id: 'msub_created',
          project_id: 42,
          user_id: 1,
          build_id: 'mbld_success',
          platform: 'android',
          status: 'ready_for_google_internal_testing',
          provider: 'eas',
          provider_submission_id: 'eas-submit-created',
          created_at: '2026-05-01T00:30:00.000Z',
          updated_at: '2026-05-01T00:30:00.000Z',
        },
      }),
      startProjectMobileExpoWebPreview: vi.fn().mockResolvedValue({
        success: true,
        preview_level: 'expo_web',
        preview_url: 'https://api.example.com/api/v1/preview/backend-proxy/42',
        message: 'Expo Web mobile preview started. This is browser-rendered and is not a native Android/iOS build.',
      }),
      exportProject: vi.fn(),
      executeProject: vi.fn(),
    }
  })

  it('shows truthful Expo source export and validation messaging for mobile projects', async () => {
    mockCurrentProject = {
      ...baseProject,
      target_platform: 'mobile_expo',
      mobile_framework: 'expo-react-native',
      mobile_platforms: ['android', 'ios'],
      mobile_release_level: 'source_only',
      mobile_capabilities: ['offlineMode', 'fileUploads', 'camera'],
    }

    render(<ProjectDashboard />)

    expect(screen.getByTestId('mobile-export-readiness')).toBeTruthy()
    expect(screen.getByText('Expo/React Native export is source-ready')).toBeTruthy()
    expect(screen.getByText(/ZIP and GitHub export will include a/)).toBeTruthy()
    expect(screen.getAllByText('Android + iOS').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Source only')).toBeTruthy()
    expect(screen.getByText(/Native APK\/AAB, iOS builds, TestFlight, and store submission are separate gated workflows/)).toBeTruthy()
    expect(await screen.findByText('Validation passed')).toBeTruthy()
    expect(screen.getByText('Mobile source package passed validation.')).toBeTruthy()
    expect(await screen.findByText('61% / 95%')).toBeTruthy()
    expect(screen.getByText('95% Readiness Target')).toBeTruthy()
    expect(screen.getByText(/Next blocker: Produce a signed Android APK\/AAB artifact through EAS Build/)).toBeTruthy()
    expect(await screen.findByTestId('mobile-build-operations')).toBeTruthy()
    expect(await screen.findByTestId('mobile-store-readiness-panel')).toBeTruthy()
    expect(screen.getByText('Native Build Pipeline')).toBeTruthy()
    expect(screen.getByText('Store Readiness')).toBeTruthy()
    expect(screen.getByText('Stored for builds')).toBeTruthy()
    expect(screen.getByTestId('mobile-build-job-mbld_existing')).toBeTruthy()
    expect(screen.getByText('EAS build is running.')).toBeTruthy()
    expect(mockApiService.getProjectMobileValidation).toHaveBeenCalledWith(42)
    expect(mockApiService.getProjectMobileScorecard).toHaveBeenCalledWith(42)
    expect(mockApiService.listProjectMobileBuilds).toHaveBeenCalledWith(42)
    expect(mockApiService.getProjectMobileCredentials).toHaveBeenCalledWith(42)
  })

  it('renders store readiness metadata from the generated store package', async () => {
    mockFiles = [
      {
        id: 501,
        project_id: 42,
        path: 'mobile/store/store-readiness.json',
        name: 'store-readiness.json',
        type: 'file',
        content: JSON.stringify({
          status: 'draft_ready_needs_manual_store_assets',
          app_name: 'Apex FieldOps Mobile',
          short_description: 'Run field jobs from a native mobile app.',
          category: 'Business',
          release_notes: 'Initial internal release.',
          android_package: 'dev.apexbuild.fieldops',
          ios_bundle_identifier: 'dev.apexbuild.fieldops',
          version: '1.0.0',
          version_code: 1,
          build_number: '1',
          data_safety_draft: {
            data_collected: ['Account credentials or session token', 'Job and estimate records'],
            data_linked_to_user: ['Account credentials or session token'],
            data_used_for_tracking: ['None declared by generated scaffold'],
            privacy_notes: ['Review all generated defaults against the production backend before submission.'],
          },
          screenshot_checklist: [
            { platform: 'Android', device: 'Phone', purpose: 'Home/jobs list with representative data' },
            { platform: 'iOS', device: '6.7-inch iPhone', purpose: 'Primary create/edit workflow' },
          ],
          manual_prerequisites: ['Privacy policy URL and support URL supplied by the app owner.'],
          truthful_status_notes: ['This package is a draft for launch preparation, not proof of App Store or Google Play approval.'],
          missing_items: ['Signed Android/iOS build artifacts produced by a successful native build job.'],
        }),
        size: 1200,
        version: 1,
        is_locked: false,
        created_at: '2026-05-01T00:00:00.000Z',
        updated_at: '2026-05-01T00:00:00.000Z',
      },
    ]
    mockCurrentProject = {
      ...baseProject,
      target_platform: 'mobile_expo',
      mobile_framework: 'expo-react-native',
      mobile_platforms: ['android', 'ios'],
      mobile_release_level: 'source_only',
      mobile_capabilities: ['offlineMode', 'fileUploads'],
      android_package: 'dev.apexbuild.fieldops',
      ios_bundle_identifier: 'dev.apexbuild.fieldops',
    }

    render(<ProjectDashboard />)

    const panel = await screen.findByTestId('mobile-store-readiness-panel')
    expect(within(panel).getByText('Apex FieldOps Mobile')).toBeTruthy()
    expect(within(panel).getByText('Draft Ready Needs Manual Store Assets')).toBeTruthy()
    expect(within(panel).getAllByText('dev.apexbuild.fieldops', { exact: false }).length).toBeGreaterThanOrEqual(1)
    expect(within(panel).getByText('Signed Android/iOS build artifacts produced by a successful native build job.')).toBeTruthy()
    expect(within(panel).getByText(/not proof of App Store or Google Play approval/)).toBeTruthy()
    expect(within(panel).getByRole('button', { name: /Download ZIP with store package/i })).toBeTruthy()
  })

  it('queues an Android APK build from the mobile build operations panel', async () => {
    mockApiService.listProjectMobileBuilds.mockResolvedValueOnce([])
    mockCurrentProject = {
      ...baseProject,
      target_platform: 'mobile_expo',
      mobile_framework: 'expo-react-native',
      mobile_platforms: ['android'],
      mobile_release_level: 'source_only',
      mobile_capabilities: ['offlineMode'],
    }

    render(<ProjectDashboard />)

    const startButton = await screen.findByRole('button', { name: /Start Android APK/i })
    fireEvent.click(startButton)

    await waitFor(() => {
      expect(mockApiService.createProjectMobileBuild).toHaveBeenCalledWith(42, {
        platform: 'android',
        profile: 'preview',
        release_level: 'internal_android_apk',
      })
    })
    expect(await screen.findByTestId('mobile-build-job-mbld_created')).toBeTruthy()
    expect(mockAddNotification).toHaveBeenCalledWith(expect.objectContaining({
      type: 'success',
      title: 'Native build queued',
    }))
  })

  it('explains native build plan blocks before provider work', async () => {
    mockApiService.listProjectMobileBuilds.mockResolvedValueOnce([])
    mockApiService.createProjectMobileBuild.mockRejectedValueOnce({
      response: {
        status: 402,
        data: {
          code: 'MOBILE_BUILD_PLAN_REQUIRED',
          current_plan: 'free',
          required_plan: 'builder',
          suggestion: 'Free accounts can generate and export mobile source. Builder unlocks Android builds; Pro or higher unlocks iOS builds.',
        },
      },
    })
    mockCurrentProject = {
      ...baseProject,
      target_platform: 'mobile_expo',
      mobile_framework: 'expo-react-native',
      mobile_platforms: ['android'],
      mobile_release_level: 'source_only',
      mobile_capabilities: ['offlineMode'],
    }

    render(<ProjectDashboard />)

    fireEvent.click(await screen.findByRole('button', { name: /Start Android APK/i }))

    await waitFor(() => {
      expect(mockApiService.createProjectMobileBuild).toHaveBeenCalledWith(42, {
        platform: 'android',
        profile: 'preview',
        release_level: 'internal_android_apk',
      })
    })
    expect(mockAddNotification).toHaveBeenCalledWith(expect.objectContaining({
      type: 'error',
      title: 'Native build blocked',
      message: expect.stringContaining('Free accounts can generate and export mobile source'),
    }))
  })

  it('starts an Expo Web preview with honest browser-rendered labeling', async () => {
    mockApiService.listProjectMobileBuilds.mockResolvedValueOnce([])
    mockCurrentProject = {
      ...baseProject,
      target_platform: 'mobile_expo',
      mobile_framework: 'expo-react-native',
      mobile_platforms: ['android', 'ios'],
      mobile_release_level: 'source_only',
      mobile_capabilities: ['offlineMode'],
    }

    render(<ProjectDashboard />)

    const previewCard = await screen.findByTestId('mobile-expo-web-preview-card')
    expect(within(previewCard).getByText(/browser-rendered and does not prove a native APK\/AAB/)).toBeTruthy()
    fireEvent.click(within(previewCard).getByRole('button', { name: /Start Expo Web Preview/i }))

    await waitFor(() => {
      expect(mockApiService.startProjectMobileExpoWebPreview).toHaveBeenCalledWith(42)
    })
    expect(await within(previewCard).findByText('https://api.example.com/api/v1/preview/backend-proxy/42')).toBeTruthy()
    expect(mockAddNotification).toHaveBeenCalledWith(expect.objectContaining({
      type: 'success',
      title: 'Expo Web preview started',
      message: expect.stringContaining('not a native Android/iOS build'),
    }))
  })

  it('stores an EAS token from the mobile credential panel without echoing the secret', async () => {
    mockApiService.getProjectMobileCredentials.mockResolvedValueOnce({
      status: 'missing',
      complete: false,
      required: ['eas_token'],
      present: [],
      missing: ['eas_token'],
      metadata: [],
      blockers: ['Add EAS token.'],
    })
    mockApiService.listProjectMobileBuilds.mockResolvedValueOnce([])
    mockCurrentProject = {
      ...baseProject,
      target_platform: 'mobile_expo',
      mobile_framework: 'expo-react-native',
      mobile_platforms: ['android'],
      mobile_release_level: 'source_only',
      mobile_capabilities: ['offlineMode'],
    }

    render(<ProjectDashboard />)

    const easCard = await screen.findByTestId('mobile-credential-eas_token')
    fireEvent.click(within(easCard).getByRole('button', { name: /Add/i }))
    fireEvent.change(within(easCard).getByLabelText('EAS access token'), {
      target: { value: 'eas-secret-token' },
    })
    fireEvent.click(within(easCard).getByRole('button', { name: /Save encrypted credential/i }))

    await waitFor(() => {
      expect(mockApiService.createProjectMobileCredential).toHaveBeenCalledWith(42, {
        type: 'eas_token',
        values: { token: 'eas-secret-token' },
      })
    })
    expect(screen.queryByDisplayValue('eas-secret-token')).toBeNull()
    expect(mockAddNotification).toHaveBeenCalledWith(expect.objectContaining({
      type: 'success',
      title: 'Mobile credential stored',
      message: expect.not.stringContaining('eas-secret-token'),
    }))
  })

  it('deletes a stored mobile credential from the credential panel', async () => {
    mockApiService.listProjectMobileBuilds.mockResolvedValueOnce([])
    mockCurrentProject = {
      ...baseProject,
      target_platform: 'mobile_expo',
      mobile_framework: 'expo-react-native',
      mobile_platforms: ['android'],
      mobile_release_level: 'source_only',
      mobile_capabilities: ['offlineMode'],
    }

    render(<ProjectDashboard />)

    const easCard = await screen.findByTestId('mobile-credential-eas_token')
    fireEvent.click(within(easCard).getByRole('button', { name: /Delete/i }))

    await waitFor(() => {
      expect(mockApiService.deleteProjectMobileCredential).toHaveBeenCalledWith(42, 'eas_token')
    })
    expect(mockAddNotification).toHaveBeenCalledWith(expect.objectContaining({
      type: 'success',
      title: 'Mobile credential deleted',
    }))
  })

  it('retries a failed mobile build from the operations panel', async () => {
    mockApiService.listProjectMobileBuilds.mockResolvedValueOnce([
      {
        id: 'mbld_failed',
        project_id: 42,
        user_id: 1,
        platform: 'android',
        profile: 'preview',
        release_level: 'internal_android_apk',
        status: 'failed',
        provider: 'eas',
        provider_build_id: 'eas-build-failed',
        failure_message: 'Metro bundle failed.',
        repair_plan: {
          failure_type: 'metro_bundle_failed',
          title: 'Repair Metro bundling',
          summary: 'Metro failed to bundle the React Native app.',
          retry_recommended: true,
          requires_source_change: true,
          actions: [
            {
              id: 'run-metro-checks',
              label: 'Run bundling checks',
              description: 'Inspect Metro output, missing imports, asset paths, Expo Router files, and React Native unsupported APIs.',
              owner: 'apex',
              blocking: true,
            },
          ],
        },
        created_at: '2026-05-01T00:00:00.000Z',
        updated_at: '2026-05-01T00:05:00.000Z',
      },
    ])
    mockCurrentProject = {
      ...baseProject,
      target_platform: 'mobile_expo',
      mobile_framework: 'expo-react-native',
      mobile_platforms: ['android'],
      mobile_release_level: 'source_only',
      mobile_capabilities: ['offlineMode'],
    }

    render(<ProjectDashboard />)

    expect(await screen.findByTestId('mobile-build-job-mbld_failed')).toBeTruthy()
    expect((await screen.findByTestId('mobile-build-repair-plan-mbld_failed')).textContent).toContain('Repair Metro bundling')
    expect(screen.getByText(/Apex: Run bundling checks/i)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: /Retry/i }))

    await waitFor(() => {
      expect(mockApiService.retryProjectMobileBuild).toHaveBeenCalledWith(42, 'mbld_failed')
    })
    expect(await screen.findByTestId('mobile-build-job-mbld_retry')).toBeTruthy()
    expect(mockAddNotification).toHaveBeenCalledWith(expect.objectContaining({
      type: 'success',
      title: 'Build retry queued',
    }))
  })

  it('checks a failed mobile build repair before retry', async () => {
    mockApiService.listProjectMobileBuilds.mockResolvedValueOnce([
      {
        id: 'mbld_failed',
        project_id: 42,
        user_id: 1,
        platform: 'android',
        profile: 'preview',
        release_level: 'internal_android_apk',
        status: 'failed',
        provider: 'eas',
        provider_build_id: 'eas-build-failed',
        failure_message: 'Metro bundle failed.',
        repair_plan: {
          failure_type: 'metro_bundle_failed',
          title: 'Repair Metro bundling',
          summary: 'Metro failed to bundle the React Native app.',
          retry_recommended: true,
          requires_source_change: true,
          actions: [
            {
              id: 'run-metro-checks',
              label: 'Run bundling checks',
              description: 'Inspect Metro output, missing imports, asset paths, Expo Router files, and React Native unsupported APIs.',
              owner: 'apex',
              blocking: true,
            },
          ],
        },
        created_at: '2026-05-01T00:00:00.000Z',
        updated_at: '2026-05-01T00:05:00.000Z',
      },
    ])
    mockCurrentProject = {
      ...baseProject,
      target_platform: 'mobile_expo',
      mobile_framework: 'expo-react-native',
      mobile_platforms: ['android'],
      mobile_release_level: 'source_only',
      mobile_capabilities: ['offlineMode'],
    }

    render(<ProjectDashboard />)

    const job = await screen.findByTestId('mobile-build-job-mbld_failed')
    fireEvent.click(within(job).getByRole('button', { name: /Check repair/i }))

    await waitFor(() => {
      expect(mockApiService.repairProjectMobileBuild).toHaveBeenCalledWith(42, 'mbld_failed')
    })
    expect(await screen.findByText('Retry pending')).toBeTruthy()
    expect(mockAddNotification).toHaveBeenCalledWith(expect.objectContaining({
      type: 'success',
      title: 'Repair checks passed',
    }))
  })

  it('submits a succeeded mobile build to the gated store upload workflow', async () => {
    mockApiService.listProjectMobileBuilds.mockResolvedValueOnce([
      {
        id: 'mbld_success',
        project_id: 42,
        user_id: 1,
        platform: 'android',
        profile: 'production',
        release_level: 'android_aab',
        status: 'succeeded',
        provider: 'eas',
        provider_build_id: 'eas-build-success',
        artifact_url: 'https://artifacts.example.com/app.aab',
        created_at: '2026-05-01T00:00:00.000Z',
        updated_at: '2026-05-01T00:05:00.000Z',
      },
    ])
    mockCurrentProject = {
      ...baseProject,
      target_platform: 'mobile_expo',
      mobile_framework: 'expo-react-native',
      mobile_platforms: ['android'],
      mobile_release_level: 'source_only',
      mobile_capabilities: ['offlineMode'],
    }

    render(<ProjectDashboard />)

    const job = await screen.findByTestId('mobile-build-job-mbld_success')
    fireEvent.click(within(job).getByRole('button', { name: /Submit upload/i }))

    await waitFor(() => {
      expect(mockApiService.submitProjectMobileBuild).toHaveBeenCalledWith(42, 'mbld_success', {})
    })
    expect(mockAddNotification).toHaveBeenCalledWith(expect.objectContaining({
      type: 'success',
      title: 'Store upload queued',
    }))
  })

  it('explains store upload quota blocks before provider work', async () => {
    mockApiService.listProjectMobileBuilds.mockResolvedValueOnce([
      {
        id: 'mbld_success',
        project_id: 42,
        user_id: 1,
        platform: 'android',
        profile: 'production',
        release_level: 'android_aab',
        status: 'succeeded',
        provider: 'eas',
        provider_build_id: 'eas-build-success',
        artifact_url: 'https://artifacts.example.com/app.aab',
        created_at: '2026-05-01T00:00:00.000Z',
        updated_at: '2026-05-01T00:05:00.000Z',
      },
    ])
    mockApiService.submitProjectMobileBuild.mockRejectedValueOnce({
      response: {
        status: 429,
        data: {
          code: 'MOBILE_SUBMISSION_QUOTA_EXCEEDED',
          current_plan: 'pro',
          current_usage: 5,
          monthly_limit: 5,
          suggestion: 'Wait for the next billing cycle or upgrade before uploading more mobile builds to store pipelines.',
        },
      },
    })
    mockCurrentProject = {
      ...baseProject,
      target_platform: 'mobile_expo',
      mobile_framework: 'expo-react-native',
      mobile_platforms: ['android'],
      mobile_release_level: 'source_only',
      mobile_capabilities: ['offlineMode'],
    }

    render(<ProjectDashboard />)

    const job = await screen.findByTestId('mobile-build-job-mbld_success')
    fireEvent.click(within(job).getByRole('button', { name: /Submit upload/i }))

    await waitFor(() => {
      expect(mockApiService.submitProjectMobileBuild).toHaveBeenCalledWith(42, 'mbld_success', {})
    })
    expect(mockAddNotification).toHaveBeenCalledWith(expect.objectContaining({
      type: 'error',
      title: 'Store upload blocked',
      message: expect.stringContaining('Usage: 5/5 this month'),
    }))
  })

  it('does not submit the same native build when an upload job already exists', async () => {
    mockApiService.listProjectMobileBuilds.mockResolvedValueOnce([
      {
        id: 'mbld_success',
        project_id: 42,
        user_id: 1,
        platform: 'android',
        profile: 'production',
        release_level: 'android_aab',
        status: 'succeeded',
        provider: 'eas',
        provider_build_id: 'eas-build-success',
        artifact_url: 'https://artifacts.example.com/app.aab',
        created_at: '2026-05-01T00:00:00.000Z',
        updated_at: '2026-05-01T00:05:00.000Z',
      },
    ])
    mockApiService.listProjectMobileSubmissions.mockResolvedValueOnce([
      {
        id: 'msub_existing',
        project_id: 42,
        user_id: 1,
        build_id: 'mbld_success',
        platform: 'android',
        status: 'ready_for_google_internal_testing',
        provider: 'eas',
        provider_submission_id: 'eas-submit-existing',
        track: 'internal',
        created_at: '2026-05-01T00:30:00.000Z',
        updated_at: '2026-05-01T00:30:00.000Z',
      },
    ])
    mockCurrentProject = {
      ...baseProject,
      target_platform: 'mobile_expo',
      mobile_framework: 'expo-react-native',
      mobile_platforms: ['android'],
      mobile_release_level: 'source_only',
      mobile_capabilities: ['offlineMode'],
    }

    render(<ProjectDashboard />)

    const job = await screen.findByTestId('mobile-build-job-mbld_success')
    expect(await screen.findByTestId('mobile-submission-job-msub_existing')).toBeTruthy()
    expect(within(job).getByText(/Ready for Google internal testing/i)).toBeTruthy()
    expect((within(job).getByRole('button', { name: /Upload requested/i }) as HTMLButtonElement).disabled).toBe(true)
    expect(within(job).queryByRole('button', { name: /Submit upload/i })).toBeNull()
    expect(mockApiService.submitProjectMobileBuild).not.toHaveBeenCalled()
  })

  it('does not show mobile export messaging for legacy web projects', async () => {
    mockCurrentProject = {
      ...baseProject,
      name: 'Apex Web App',
      target_platform: 'fullstack_web',
    }

    render(<ProjectDashboard />)

    expect(screen.queryByTestId('mobile-export-readiness')).toBeNull()
    expect(screen.queryByTestId('mobile-build-operations')).toBeNull()
    expect(screen.queryByText('Expo/React Native export is source-ready')).toBeNull()
    await waitFor(() => {
      expect(mockApiService.getProjectMobileValidation).not.toHaveBeenCalled()
      expect(mockApiService.getProjectMobileScorecard).not.toHaveBeenCalled()
      expect(mockApiService.getProjectMobileStoreReadiness).not.toHaveBeenCalled()
      expect(mockApiService.listProjectMobileSubmissions).not.toHaveBeenCalled()
    })
  })
})
