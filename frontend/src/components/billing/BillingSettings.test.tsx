/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'

vi.mock('@/services/api', () => ({
  default: {
    getPlans: vi.fn(),
    getSubscription: vi.fn(),
    getCreditBalance: vi.fn(),
    getInvoices: vi.fn(),
    changePlan: vi.fn(),
    createCheckoutSession: vi.fn(),
  },
}))

vi.mock('./BuyCreditsModal', () => ({
  BuyCreditsModal: () => <div>Buy Credits Modal</div>,
}))

import apiService from '@/services/api'
import { BillingSettings } from './BillingSettings'

const apiMocks = apiService as unknown as {
  getPlans: ReturnType<typeof vi.fn>
  getSubscription: ReturnType<typeof vi.fn>
  getCreditBalance: ReturnType<typeof vi.fn>
  getInvoices: ReturnType<typeof vi.fn>
  changePlan: ReturnType<typeof vi.fn>
  createCheckoutSession: ReturnType<typeof vi.fn>
}

describe('BillingSettings', () => {
  beforeEach(() => {
    apiMocks.getPlans.mockReset()
    apiMocks.getSubscription.mockReset()
    apiMocks.getCreditBalance.mockReset()
    apiMocks.getInvoices.mockReset()

    apiMocks.getPlans.mockResolvedValue({
      success: true,
      data: {
        self_serve_ready: true,
        plans: [
          {
            type: 'free',
            name: 'Free',
            monthly_price_cents: 0,
            monthly_price_id: '',
            monthly_credits_usd: 0,
            is_popular: false,
            features: ['Static frontend websites'],
          },
          {
            type: 'builder',
            name: 'Builder',
            monthly_price_cents: 2400,
            monthly_price_id: 'price_builder_monthly_live',
            monthly_credits_usd: 12,
            is_popular: true,
            features: ['Backend generation', 'Publish', 'BYOK'],
          },
        ],
      },
    })
    apiMocks.getInvoices.mockResolvedValue({ success: true, data: { invoices: [] } })
  })

  it('renders free-tier gating truthfully', async () => {
    apiMocks.getSubscription.mockResolvedValue({
      success: true,
      data: {
        plan_type: 'free',
        plan_name: 'Free',
        status: 'inactive',
      },
    })
    apiMocks.getCreditBalance.mockResolvedValue({
      success: true,
      data: {
        balance: 3,
        has_unlimited: false,
        bypass_billing: false,
      },
    })

    render(<BillingSettings />)

    expect(await screen.findByText('Subscription Plans')).toBeTruthy()
    expect(screen.getByText(/Plan tier unlocks capabilities\. Credits cover managed AI usage\./)).toBeTruthy()
    expect(screen.getAllByText('Static websites & UI mockups').length).toBeGreaterThan(0)
    expect(screen.getByText('Static frontend websites')).toBeTruthy()
    expect(screen.getByText(/One-time top-ups for extra AI usage runway\. Don't unlock plan features on their own\./)).toBeTruthy()
    expect(screen.getByText('Credit Packs')).toBeTruthy()
  })

  it('shows paid-plan narrative and renewal details', async () => {
    apiMocks.getSubscription.mockResolvedValue({
      success: true,
      data: {
        plan_type: 'builder',
        plan_name: 'Builder',
        status: 'active',
        current_period_end: '2026-04-21T00:00:00Z',
      },
    })
    apiMocks.getCreditBalance.mockResolvedValue({
      success: true,
      data: {
        balance: 14,
        has_unlimited: false,
        bypass_billing: false,
      },
    })

    render(<BillingSettings />)

    expect(await screen.findByText('Subscription Plans')).toBeTruthy()
    await waitFor(() => {
      expect(screen.getAllByText('Full-stack development unlocked').length).toBeGreaterThan(0)
    })
    expect(screen.getByText('Manage')).toBeTruthy()
    expect(screen.getByText('Backend generation')).toBeTruthy()
    expect(screen.getByText('Publish')).toBeTruthy()
    expect(screen.getByText('BYOK')).toBeTruthy()
    expect(screen.getByText(/Renews/)).toBeTruthy()
  })

  it('does not render paid checkout buttons when billing is not self-serve ready', async () => {
    apiMocks.getPlans.mockResolvedValueOnce({
      success: true,
      data: {
        self_serve_ready: false,
        plans: [
          {
            type: 'free',
            name: 'Free',
            monthly_price_cents: 0,
            monthly_price_id: '',
            monthly_credits_usd: 0,
            is_popular: false,
            features: ['Static frontend websites'],
          },
          {
            type: 'builder',
            name: 'Builder',
            monthly_price_cents: 2400,
            monthly_price_id: 'price_builder_monthly',
            monthly_credits_usd: 12,
            is_popular: true,
            features: ['Backend generation'],
          },
        ],
      },
    })
    apiMocks.getSubscription.mockResolvedValue({
      success: true,
      data: {
        plan_type: 'free',
        plan_name: 'Free',
        status: 'inactive',
      },
    })
    apiMocks.getCreditBalance.mockResolvedValue({
      success: true,
      data: {
        balance: 3,
        has_unlimited: false,
        bypass_billing: false,
      },
    })

    render(<BillingSettings />)

    expect(await screen.findByText('Subscription Plans')).toBeTruthy()
    expect(screen.queryByText('Upgrade to Builder')).toBeNull()
    expect(screen.getByText('Contact support')).toBeTruthy()
  })

  it('shows upgrade button for paid user looking at higher tier', async () => {
    apiMocks.getPlans.mockResolvedValue({
      success: true,
      data: {
        self_serve_ready: true,
        plans: [
          {
            type: 'free',
            name: 'Free',
            monthly_price_cents: 0,
            monthly_price_id: '',
            monthly_credits_usd: 0,
            is_popular: false,
            features: [],
          },
          {
            type: 'builder',
            name: 'Builder',
            monthly_price_cents: 2400,
            monthly_price_id: 'price_builder_monthly_live',
            monthly_credits_usd: 12,
            is_popular: true,
            features: ['Backend generation', 'Publish', 'BYOK'],
          },
          {
            type: 'pro',
            name: 'Pro',
            monthly_price_cents: 5900,
            monthly_price_id: 'price_pro_monthly_live',
            monthly_credits_usd: 40,
            is_popular: false,
            features: ['Ollama cloud models', 'BYOK + caps', 'Long runs'],
          },
        ],
      },
    })
    apiMocks.getSubscription.mockResolvedValue({
      success: true,
      data: {
        plan_type: 'builder',
        plan_name: 'Builder',
        status: 'active',
        current_period_end: '2026-04-21T00:00:00Z',
      },
    })
    apiMocks.getCreditBalance.mockResolvedValue({
      success: true,
      data: {
        balance: 14,
        has_unlimited: false,
        bypass_billing: false,
      },
    })

    render(<BillingSettings />)

    expect(await screen.findByText('Subscription Plans')).toBeTruthy()
    expect(screen.getByText('Upgrade to Pro')).toBeTruthy()
    expect(screen.getByText('Downgrade to Free')).toBeTruthy()
  })

  it('downgrade button communicates period-end effect', async () => {
    apiMocks.changePlan.mockResolvedValue({
      success: true,
      data: {
        plan_type: 'builder',
        billing_cycle: 'monthly',
        status: 'active',
        current_period_end: '2026-04-21T00:00:00Z',
      },
    })

    apiMocks.getPlans.mockResolvedValue({
      success: true,
      data: {
        self_serve_ready: true,
        plans: [
          {
            type: 'free',
            name: 'Free',
            monthly_price_cents: 0,
            monthly_price_id: '',
            monthly_credits_usd: 0,
            is_popular: false,
            features: [],
          },
          {
            type: 'builder',
            name: 'Builder',
            monthly_price_cents: 2400,
            monthly_price_id: 'price_builder_monthly_live',
            monthly_credits_usd: 12,
            is_popular: true,
            features: ['Backend generation'],
          },
          {
            type: 'pro',
            name: 'Pro',
            monthly_price_cents: 5900,
            monthly_price_id: 'price_pro_monthly_live',
            monthly_credits_usd: 40,
            is_popular: false,
            features: ['Ollama cloud models'],
          },
        ],
      },
    })
    apiMocks.getSubscription.mockResolvedValue({
      success: true,
      data: {
        plan_type: 'pro',
        plan_name: 'Pro',
        status: 'active',
        current_period_end: '2026-05-21T00:00:00Z',
      },
    })
    apiMocks.getCreditBalance.mockResolvedValue({
      success: true,
      data: {
        balance: 40,
        has_unlimited: false,
        bypass_billing: false,
      },
    })

    const { container } = render(<BillingSettings />)

    expect(await screen.findByText('Subscription Plans')).toBeTruthy()
    const downgradeBtn = screen.getByText('Downgrade to Builder')
    expect(downgradeBtn).toBeTruthy()

    fireEvent.click(downgradeBtn)

    await waitFor(() => {
      expect(container.textContent).toContain('Downgrade scheduled')
      expect(container.textContent).toContain('until the end of the current period')
    })
  })

  it('upgrade button communicates immediate prorated charge', async () => {
    apiMocks.changePlan.mockResolvedValue({
      success: true,
      data: {
        plan_type: 'pro',
        billing_cycle: 'monthly',
        status: 'active',
        current_period_end: '2026-05-21T00:00:00Z',
      },
    })

    apiMocks.getPlans.mockResolvedValue({
      success: true,
      data: {
        self_serve_ready: true,
        plans: [
          {
            type: 'free',
            name: 'Free',
            monthly_price_cents: 0,
            monthly_price_id: '',
            monthly_credits_usd: 0,
            is_popular: false,
            features: [],
          },
          {
            type: 'builder',
            name: 'Builder',
            monthly_price_cents: 2400,
            monthly_price_id: 'price_builder_monthly_live',
            monthly_credits_usd: 12,
            is_popular: true,
            features: ['Backend generation'],
          },
          {
            type: 'pro',
            name: 'Pro',
            monthly_price_cents: 5900,
            monthly_price_id: 'price_pro_monthly_live',
            monthly_credits_usd: 40,
            is_popular: false,
            features: ['Ollama cloud models'],
          },
        ],
      },
    })
    apiMocks.getSubscription.mockResolvedValue({
      success: true,
      data: {
        plan_type: 'builder',
        plan_name: 'Builder',
        status: 'active',
        current_period_end: '2026-04-21T00:00:00Z',
      },
    })
    apiMocks.getCreditBalance.mockResolvedValue({
      success: true,
      data: {
        balance: 12,
        has_unlimited: false,
        bypass_billing: false,
      },
    })

    const { container } = render(<BillingSettings />)

    expect(await screen.findByText('Subscription Plans')).toBeTruthy()
    const upgradeBtn = screen.getByText('Upgrade to Pro')
    expect(upgradeBtn).toBeTruthy()

    fireEvent.click(upgradeBtn)

    await waitFor(() => {
      expect(container.textContent).toContain('Upgraded to Pro')
      expect(container.textContent).toContain('prorated difference immediately')
    })
  })
})
