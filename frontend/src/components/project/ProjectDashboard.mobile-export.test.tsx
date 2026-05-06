/* @vitest-environment jsdom */

import React from 'react'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { Project } from '@/types'
import ProjectDashboard from './ProjectDashboard'

// Vitest in this repo predates vi.hoisted; var keeps the mock factory safely hoistable.
var mockCurrentProject: Project | null = null

vi.mock('@/hooks/useStore', () => ({
  useStore: () => ({
    currentProject: mockCurrentProject,
    files: [],
    collaborationUsers: [],
    apiService: {
      getProject: vi.fn(),
      getFiles: vi.fn(),
      getExecutionHistory: vi.fn(),
      getAIUsage: vi.fn(),
      exportProject: vi.fn(),
      executeProject: vi.fn(),
    },
    setCurrentProject: vi.fn(),
    addNotification: vi.fn(),
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
  })

  it('shows truthful Expo source export messaging for mobile projects', () => {
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
    expect(screen.getByText('Android + iOS')).toBeTruthy()
    expect(screen.getByText('Source only')).toBeTruthy()
    expect(screen.getByText(/Native APK\/AAB, iOS builds, TestFlight, and store submission are separate gated workflows/)).toBeTruthy()
  })

  it('does not show mobile export messaging for legacy web projects', () => {
    mockCurrentProject = {
      ...baseProject,
      name: 'Apex Web App',
      target_platform: 'fullstack_web',
    }

    render(<ProjectDashboard />)

    expect(screen.queryByTestId('mobile-export-readiness')).toBeNull()
    expect(screen.queryByText('Expo/React Native export is source-ready')).toBeNull()
  })
})
