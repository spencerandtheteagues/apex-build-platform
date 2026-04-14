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
            who_must_act: 'system',
            partial_progress_allowed: false,
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
        historicalLearning={{
          scope: 'stack:react+go',
          observed_builds: 2,
          repair_strategy_win_rates: ['semantic_diff/import_export_mismatch strategy=targeted_symbol_repair win_rate=1/1'],
          semantic_repair_hints: ['patch=import_export_mismatch files=src/App.tsx'],
        }}
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
    expect(screen.getByText('Category: runtime failure')).toBeTruthy()
    expect(screen.getAllByText('Owner: system').length).toBeGreaterThan(0)
    expect(screen.getByText('Type: verification blocker')).toBeTruthy()
    expect(screen.getByText('Stops forward progress')).toBeTruthy()
    expect(screen.getByText('Checkpoint Continuity')).toBeTruthy()
    expect(screen.getByText('Approval History')).toBeTruthy()
    expect(screen.getByText('User acknowledgement requested')).toBeTruthy()
    expect(screen.getByText('Ack required')).toBeTruthy()
    expect(screen.getByText('Provider Scorecards')).toBeTruthy()
    expect(screen.getByText('Repair Signals')).toBeTruthy()
    expect(screen.getByText('Learning Priors')).toBeTruthy()
    expect(screen.getByText('semantic_diff/import_export_mismatch strategy=targeted_symbol_repair win_rate=1/1')).toBeTruthy()
  })

  it('renders paused orchestration phases truthfully', () => {
    render(
      <OrchestrationOverview
        buildStatus="awaiting_review"
        currentPhase="validation"
        blockers={[
          {
            id: 'plan-gate',
            title: 'Upgrade required for full-stack work',
            type: 'plan_upgrade_required',
            category: 'plan_tier',
            severity: 'blocking',
            summary: 'This request needs backend runtime, which is locked on the free plan.',
            who_must_act: 'user',
            partial_progress_allowed: true,
            plan_tier_related: true,
          },
        ]}
        policyState={{
          plan_type: 'free',
          classification: 'upgrade_required',
          upgrade_required: true,
          upgrade_reason: 'backend runtime',
          required_plan: 'builder',
        }}
        interaction={{
          waiting_for_user: true,
          paused: true,
          pause_reason: 'Waiting for plan acknowledgement before deeper backend work.',
          permission_requests: [],
          permission_rules: [],
          approval_events: [],
        }}
        intentBrief={{
          id: 'intent-2',
          normalized_request: 'Build a SaaS dashboard with auth and billing',
          app_type: 'fullstack',
          required_capabilities: ['auth', 'database', 'billing'],
          created_at: '2026-03-21T21:00:00Z',
        }}
        buildContract={{
          id: 'contract-2',
          build_id: 'build-2',
          app_type: 'fullstack',
          verified: false,
        }}
        verificationReports={[
          {
            id: 'vr-2',
            build_id: 'build-2',
            phase: 'validation',
            surface: 'frontend',
            status: 'blocked',
            generated_at: '2026-03-21T21:10:00Z',
          },
        ]}
      />
    )

    expect(screen.getAllByText('paused').length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Plan gate: upgrade required\./).length).toBeGreaterThan(0)
    expect(screen.getByText('Plan-tier blocker')).toBeTruthy()
    expect(screen.getByText('Partial work can continue')).toBeTruthy()
    expect(screen.getAllByText('Waiting for plan acknowledgement before deeper backend work.').length).toBeGreaterThan(0)
  })

  it('renders skipped repair phases when no repair ladder was needed', () => {
    render(
      <OrchestrationOverview
        buildStatus="completed"
        currentPhase="completed"
        intentBrief={{
          id: 'intent-3',
          normalized_request: 'Build a static marketing page',
          app_type: 'web',
          created_at: '2026-03-21T22:00:00Z',
        }}
        policyState={{
          plan_type: 'free',
          classification: 'static_ready',
          static_frontend_only: true,
        }}
        verificationReports={[
          {
            id: 'vr-3',
            build_id: 'build-3',
            phase: 'validation',
            surface: 'frontend',
            status: 'passed',
            generated_at: '2026-03-21T22:05:00Z',
          },
        ]}
      />
    )

    expect(screen.getByText('skipped')).toBeTruthy()
    expect(screen.getByText('No repair ladder was needed because verification completed without recurring failure fingerprints.')).toBeTruthy()
  })

  it('highlights review-required patch bundles in the build journal', () => {
    render(
      <OrchestrationOverview
        buildStatus="in_progress"
        currentPhase="patch"
        patchBundles={[
          {
            id: 'patch-review-1',
            build_id: 'build-7',
            provider: 'claude',
            justification: 'Compile validator Hydra winner (targeted_node_rewrite)',
            merge_policy: 'review_required',
            review_required: true,
            risk_reasons: ['dependency_changes_require_review'],
            created_at: '2026-04-12T15:00:00Z',
          },
        ]}
      />
    )

    expect(screen.getByText('Patch bundles generated')).toBeTruthy()
    expect(screen.getByText(/require review before merge/i)).toBeTruthy()
    expect(screen.getByText(/Review required before merge/i)).toBeTruthy()
  })
})
