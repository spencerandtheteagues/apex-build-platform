/* @vitest-environment jsdom */

import React from 'react'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/services/api', () => ({
  default: {
    post: vi.fn(),
  },
}))

import apiService from '@/services/api'
import PanicKillButton from './PanicKillButton'

describe('PanicKillButton', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    ;(apiService.post as any).mockReset()
    ;(apiService.post as any).mockResolvedValue({ data: {} })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders with KILL ALL text', () => {
    render(<PanicKillButton />)
    expect(screen.getByText('KILL ALL')).toBeTruthy()
  })

  it('returns null when visible=false', () => {
    const { container } = render(<PanicKillButton visible={false} />)
    expect(container.innerHTML).toBe('')
  })

  it('requires double-click confirmation before killing', async () => {
    vi.useRealTimers()
    render(<PanicKillButton />)

    // First click puts it in confirm state
    fireEvent.click(screen.getByText('KILL ALL'))
    expect(screen.getByText('CONFIRM KILL')).toBeTruthy()
    expect(apiService.post).not.toHaveBeenCalled()

    // Second click triggers the kill
    fireEvent.click(screen.getByText('CONFIRM KILL'))

    await waitFor(() => {
      expect(apiService.post).toHaveBeenCalledWith('/budget/kill-all')
    })
  })

  it('resets confirmation state after 3 seconds', async () => {
    render(<PanicKillButton />)

    fireEvent.click(screen.getByText('KILL ALL'))
    expect(screen.getByText('CONFIRM KILL')).toBeTruthy()

    // Advance timers past the 3s confirmation window
    act(() => {
      vi.advanceTimersByTime(3100)
    })

    expect(screen.getByText('KILL ALL')).toBeTruthy()
  })
})
