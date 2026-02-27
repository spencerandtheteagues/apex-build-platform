/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/services/api', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
  },
}))

import apiService from '@/services/api'
import BudgetSettings from './BudgetSettings'

describe('BudgetSettings', () => {
  beforeEach(() => {
    ;(apiService.get as any).mockReset()
    ;(apiService.post as any).mockReset()
    ;(apiService.delete as any).mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders existing caps on mount', async () => {
    ;(apiService.get as any).mockResolvedValue({
      data: {
        caps: [
          { id: 1, cap_type: 'daily', limit_usd: 5.0, action: 'stop', is_active: true },
          { id: 2, cap_type: 'monthly', limit_usd: 100.0, action: 'warn', is_active: true },
        ],
      },
    })

    render(<BudgetSettings />)

    await waitFor(() => {
      expect(screen.getByText('Daily Limit')).toBeTruthy()
    })
    expect(screen.getByText('$5.00')).toBeTruthy()
    // "Hard Stop" appears in both the cap badge and the action select dropdown
    expect(screen.getAllByText('Hard Stop').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Monthly Limit')).toBeTruthy()
    expect(screen.getByText('$100.00')).toBeTruthy()
    // "Warn Only" appears in both the cap badge and the action select dropdown
    expect(screen.getAllByText('Warn Only').length).toBeGreaterThanOrEqual(1)
  })

  it('shows empty state when no caps', async () => {
    ;(apiService.get as any).mockResolvedValue({ data: { caps: [] } })

    render(<BudgetSettings />)

    await waitFor(() => {
      expect(screen.getByText('Add')).toBeTruthy()
    })
    expect(screen.queryByText('Daily Limit')).toBeNull()
    expect(screen.queryByText('Monthly Limit')).toBeNull()
  })

  it('creates a new cap when form is submitted', async () => {
    ;(apiService.get as any).mockResolvedValue({ data: { caps: [] } })
    ;(apiService.post as any).mockResolvedValue({ data: {} })

    render(<BudgetSettings />)

    await waitFor(() => {
      expect(screen.getByText('Add')).toBeTruthy()
    })

    const limitInput = screen.getByPlaceholderText('10.00')
    fireEvent.change(limitInput, { target: { value: '25.00' } })

    fireEvent.click(screen.getByText('Add'))

    await waitFor(() => {
      expect(apiService.post).toHaveBeenCalledWith('/budget/caps', {
        cap_type: 'daily',
        limit_usd: 25.0,
        action: 'stop',
      })
    })
  })

  it('deletes a cap when trash button is clicked', async () => {
    ;(apiService.get as any).mockResolvedValue({
      data: {
        caps: [{ id: 42, cap_type: 'daily', limit_usd: 10.0, action: 'stop', is_active: true }],
      },
    })
    ;(apiService.delete as any).mockResolvedValue({ data: {} })

    render(<BudgetSettings />)

    await waitFor(() => {
      expect(screen.getByText('Daily Limit')).toBeTruthy()
    })

    // The last button in each cap row is the delete button
    const deleteButtons = screen.getAllByRole('button')
    // Find the button that isn't "Add" — the first button should be the trash
    const trashButton = deleteButtons.find(btn =>
      !btn.textContent?.includes('Add')
    )
    if (trashButton) {
      fireEvent.click(trashButton)
    }

    await waitFor(() => {
      expect(apiService.delete).toHaveBeenCalledWith('/budget/caps/42')
    })
  })
})
