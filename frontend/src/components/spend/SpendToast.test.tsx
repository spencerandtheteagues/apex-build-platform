/* @vitest-environment jsdom */

import React from 'react'
import { act, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import SpendToast from './SpendToast'

describe('SpendToast', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders agent role and formatted cost', () => {
    render(<SpendToast agentRole="architect" cost={0.0135} onDismiss={() => {}} />)

    expect(screen.getByText('architect')).toBeTruthy()
    expect(screen.getByText('$0.0135')).toBeTruthy()
    expect(screen.getByText('Agent')).toBeTruthy()
    expect(screen.getByText('spent')).toBeTruthy()
  })

  it('calls onDismiss after auto-dismiss timeout', () => {
    const onDismiss = vi.fn()
    render(<SpendToast agentRole="coder" cost={0.05} onDismiss={onDismiss} />)

    // Should not dismiss immediately
    expect(onDismiss).not.toHaveBeenCalled()

    // After 3s the visibility timer fires, then 300ms for the animation
    act(() => {
      vi.advanceTimersByTime(3400)
    })

    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('formats zero cost correctly', () => {
    render(<SpendToast agentRole="reviewer" cost={0} onDismiss={() => {}} />)

    expect(screen.getByText('$0.0000')).toBeTruthy()
  })
})
