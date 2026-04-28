/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/components/billing/BuyCreditsModal', () => ({
  BuyCreditsModal: () => null,
}))

vi.mock('@/hooks/useStore', () => ({
  useUser: () => ({
    subscription_type: 'builder',
    credit_balance: 50,
    has_unlimited_credits: false,
    bypass_billing: false,
  }),
}))

vi.mock('@/services/api', () => ({
  default: {
    getBYOKUsage: vi.fn(),
  },
}))

import apiService from '@/services/api'
import CostTicker from './CostTicker'

describe('CostTicker', () => {
  beforeEach(() => {
    vi.mocked(apiService.getBYOKUsage).mockResolvedValue({
      success: true,
      data: {
        total_cost: 8.8585,
        total_tokens: 1200,
        total_requests: 131,
        by_provider: {},
      },
      billing: {
        credit_balance: 50,
        has_unlimited_credits: false,
        bypass_billing: false,
      },
    } as any)
  })

  it('opens the monthly usage dropdown downward so it stays readable below the header', async () => {
    const { container } = render(<CostTicker />)

    await waitFor(() => {
      expect(screen.getByText('$8.86')).toBeTruthy()
    })

    fireEvent.click(screen.getByRole('button'))

    expect(await screen.findByText('Usage Overview')).toBeTruthy()
    expect(container.querySelector('.top-full')).toBeTruthy()
    expect(container.querySelector('.bottom-full')).toBeNull()
  })
})
