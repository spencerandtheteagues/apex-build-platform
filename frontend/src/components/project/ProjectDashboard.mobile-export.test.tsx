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
    })
  })
})
