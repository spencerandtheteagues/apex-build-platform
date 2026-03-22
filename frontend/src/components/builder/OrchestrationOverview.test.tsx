/* @vitest-environment jsdom */

import React from 'react'
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/components/ui', () => ({
  Badge: ({ children, ...props }: any) => <span {...props}>{children}</span>,
  Card: ({ children, ...props }: any) => <div {...props}>{children}</div>,
  CardContent: ({ children, ...props }: any) => <div {...props}>{children}</div>,
  CardHeader: ({ children, ...props }: any) => <div {...props}>{children}</div>,
  CardTitle: ({ children, ...props }: any) => <div {...props}>{children}</div>,
}))

import OrchestrationOverview from './OrchestrationOverview'

describe('OrchestrationOverview', () => {
  it('renders the build journal and mock-to-real diff from orchestration state', () => {
    render(
      <OrchestrationOverview
        buildStatus="in_progress"
        currentPhase="validation"
        qualityGateStatus="running"
        capabilityState={{
          required_capabilities: ['auth', 'database', 'billing'],
          requires_backend_runtime: true,
          requires_database: true,
          requires_auth: true,
          requires_billing: true,
          requires_publish: true,
        }}
        policyState={{
          plan_type: 'builder',
          classification: 'full_stack_candidate',
          full_stack_eligible: true,
          publish_enabled: true,
        }}
        blockers={[
          {
            id: 'blocker-1',
            title: 'Verification blocker',
            type: 'verification_blocker',
            category: 'runtime_failure',
            severity: 'blocking',
            summary: 'Response payload shape does not match the contract.',
          },
        ]}
        approvals={[
          {
            id: 'database',
            kind: 'database',
            title: 'Database access',
            status: 'satisfied',
            required: true,
            source_type: 'policy',
            actor: 'system',
            requested_at: '2026-03-21T20:00:00Z',
          },
          {
            id: 'permission_request_perm-1',
            kind: 'permission_program',
            title: 'Program access for docker',
            status: 'pending',
            required: true,
            summary: 'Docker is needed to run the preview image.',
            source_type: 'permission_request',
            actor: 'user',
            acknowledgement_required: true,
            requested_at: '2026-03-21T20:03:00Z',
          },
        ]}
        checkpoints={[
          {
            id: 'checkpoint-1',
            number: 1,
            name: 'Planning locked',
            description: 'Saved before implementation',
            progress: 42,
            restorable: true,
            created_at: '2026-03-21T20:06:00Z',
          },
        ]}
        interaction={{
          waiting_for_user: true,
          pause_reason: 'Waiting for permission acknowledgement.',
          approval_events: [
            {
              id: 'event-1',
              kind: 'user_acknowledgement',
              title: 'User acknowledgement requested',
              status: 'pending',
              summary: 'Confirm the billing provider before continuing.',
              source_type: 'pending_question',
              actor: 'lead',
              timestamp: '2026-03-21T20:02:00Z',
            },
            {
              id: 'event-2',
              kind: 'permission_program',
              title: 'Permission request for docker',
              status: 'satisfied',
              summary: 'Approved for this build.',
              source_type: 'permission_request',
              actor: 'user',
              timestamp: '2026-03-21T20:04:00Z',
            },
          ],
          permission_requests: [],
          permission_rules: [],
        }}
        intentBrief={{
          id: 'intent-1',
          normalized_request: 'Build a billing dashboard',
          app_type: 'fullstack',
          complexity_class: 'complex',
          cost_sensitivity: 'medium',
          deployment_target: 'render',
          required_capabilities: ['auth', 'database', 'billing'],
          created_at: '2026-03-21T20:00:00Z',
        }}
        buildContract={{
          id: 'contract-1',
          build_id: 'build-1',
          app_type: 'fullstack',
          verified: true,
          auth_contract: {
            required: true,
            provider: 'session',
            session_strategy: 'cookie',
          },
          backend_resource_map: [
            { name: 'api', kind: 'service' },
          ],
          db_schema_contract: [
            { name: 'users' },
          ],
          env_var_contract: [
            { name: 'STRIPE_SECRET_KEY', required: true },
          ],
          truth_by_surface: {
            frontend: ['partially_wired'],
            backend: ['verified'],
            integration: ['needs_external_api'],
          },
        }}
        workOrders={[
          {
            id: 'wo-1',
            build_id: 'build-1',
            role: 'frontend',
            category: 'implementation',
            task_shape: 'patch_generation',
            summary: 'Build dashboard shell',
          },
        ]}
        patchBundles={[
          {
            id: 'patch-1',
            build_id: 'build-1',
            created_at: '2026-03-21T20:05:00Z',
          },
        ]}
        verificationReports={[
          {
            id: 'vr-1',
            build_id: 'build-1',
            phase: 'validation',
            surface: 'backend',
            status: 'failed',
            blockers: ['Response payload shape does not match the contract.'],
            generated_at: '2026-03-21T20:10:00Z',
          },
        ]}
        promotionDecision={{
          id: 'promo-1',
          build_id: 'build-1',
          readiness_state: 'integration_ready',
          generated_at: '2026-03-21T20:12:00Z',
          truth_by_surface: {
            frontend: ['partially_wired'],
            backend: ['verified'],
            deployment: ['scaffolded'],
          },
        }}
        providerScorecards={[
          {
            provider: 'claude',
            task_shape: 'contract_compile',
            first_pass_verification_pass_rate: 88,
            repair_success_rate: 91,
            average_latency_seconds: 4.2,
            average_cost_per_success: 1.8,
            hosted_eligible: true,
          },
        ]}
        failureFingerprints={[
          {
            id: 'ff-1',
            build_id: 'build-1',
            task_shape: 'patch_generation',
            provider: 'gpt',
            failure_class: 'contract_mismatch',
            repair_path_chosen: ['targeted_patch', 'reverify'],
            repair_success: true,
            created_at: '2026-03-21T20:11:00Z',
          },
        ]}
        truthBySurface={{
          frontend: ['partially_wired'],
          backend: ['verified'],
          deployment: ['scaffolded'],
        }}
      />
    )

    expect(screen.getByText('Build Journal')).toBeTruthy()
    expect(screen.getByText('Request parsed')).toBeTruthy()
    expect(screen.getByText('Build classification selected')).toBeTruthy()
    expect(screen.getByText('Classification: full stack candidate.')).toBeTruthy()
    expect(screen.getByText('Build dashboard shell')).toBeTruthy()
    expect(screen.getByText('backend failed during validation.')).toBeTruthy()
    expect(screen.getByText('Mock-To-Real Diff')).toBeTruthy()
    expect(screen.getAllByText('Frontend').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Deployment').length).toBeGreaterThan(0)
    expect(screen.getByText('Checkpoint Continuity')).toBeTruthy()
    expect(screen.getByText('Approval History')).toBeTruthy()
    expect(screen.getByText('User acknowledgement requested')).toBeTruthy()
    expect(screen.getByText('Ack required')).toBeTruthy()
    expect(screen.getByText('Provider Scorecards')).toBeTruthy()
    expect(screen.getByText('Repair Signals')).toBeTruthy()
  })
})
