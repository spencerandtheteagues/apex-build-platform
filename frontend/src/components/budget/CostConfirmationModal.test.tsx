/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import CostConfirmationModal from './CostConfirmationModal'

describe('CostConfirmationModal', () => {
  const defaultProps = {
    estimatedCostMin: 0.5,
    estimatedCostMax: 2.0,
    dailyRemaining: 10.0,
    monthlyRemaining: 50.0,
    onConfirm: vi.fn(),
    onCancel: vi.fn(),
  }

  it('renders cost estimate range', () => {
    render(<CostConfirmationModal {...defaultProps} />)

    expect(screen.getByText('Cost Estimate')).toBeTruthy()
    expect(screen.getByText('$0.50 - $2.00')).toBeTruthy()
    expect(screen.getByText('$10.00')).toBeTruthy()
    expect(screen.getByText('$50.00')).toBeTruthy()
  })

  it('shows budget warning when cost exceeds remaining', () => {
    render(
      <CostConfirmationModal
        {...defaultProps}
        estimatedCostMax={15.0}
        dailyRemaining={10.0}
      />
    )

    expect(
      screen.getByText(/This build may exceed your budget cap/)
    ).toBeTruthy()
  })

  it('does not show budget warning when within limits', () => {
    render(<CostConfirmationModal {...defaultProps} />)

    expect(
      screen.queryByText(/This build may exceed your budget cap/)
    ).toBeNull()
  })

  it('calls onConfirm when Start Build is clicked', () => {
    const onConfirm = vi.fn()
    render(<CostConfirmationModal {...defaultProps} onConfirm={onConfirm} />)

    fireEvent.click(screen.getByText('Start Build'))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('calls onCancel when Cancel is clicked', () => {
    const onCancel = vi.fn()
    render(<CostConfirmationModal {...defaultProps} onCancel={onCancel} />)

    fireEvent.click(screen.getByText('Cancel'))
    expect(onCancel).toHaveBeenCalledTimes(1)
  })
})
