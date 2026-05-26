/* @vitest-environment jsdom */

import React from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { OnboardingTour } from './OnboardingTour'
import { onboardingStarters } from './onboardingStarters'

const ONBOARDING_KEY = 'apex_onboarding_completed'

const advanceToStarterStep = () => {
  for (let i = 0; i < 5; i += 1) {
    fireEvent.click(screen.getByText('Next'))
  }
}

describe('OnboardingTour', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.runOnlyPendingTimers()
    vi.useRealTimers()
  })

  it('shows first-run onboarding after the opening delay when not completed', () => {
    render(<OnboardingTour />)

    expect(screen.queryByText('Welcome to APEX-BUILD')).toBeNull()

    act(() => {
      vi.advanceTimersByTime(800)
    })

    expect(screen.getByText('Welcome to APEX-BUILD')).toBeTruthy()
  })

  it('does not show onboarding when completion is already stored', () => {
    localStorage.setItem(ONBOARDING_KEY, 'true')

    render(<OnboardingTour />)

    act(() => {
      vi.advanceTimersByTime(1000)
    })

    expect(screen.queryByText('Welcome to APEX-BUILD')).toBeNull()
  })

  it('forceShow overrides stored completion for support and demos', () => {
    localStorage.setItem(ONBOARDING_KEY, 'true')

    render(<OnboardingTour forceShow />)

    expect(screen.getByText('Welcome to APEX-BUILD')).toBeTruthy()
  })

  it('renders as an accessible modal dialog', () => {
    render(<OnboardingTour forceShow />)

    const dialog = screen.getByRole('dialog', { name: 'Welcome to APEX-BUILD' })
    expect(dialog).toBeTruthy()
    expect(dialog.getAttribute('aria-modal')).toBe('true')
  })

  it('selects a starter prompt and marks onboarding complete', () => {
    const onStarterSelect = vi.fn()
    const onComplete = vi.fn()

    render(<OnboardingTour forceShow onStarterSelect={onStarterSelect} onComplete={onComplete} />)

    advanceToStarterStep()
    fireEvent.click(screen.getByRole('button', { name: /prefill prompt: portfolio site/i }))

    expect(onStarterSelect).toHaveBeenCalledWith(expect.objectContaining({
      id: 'portfolio-site',
      mode: 'fast',
    }))
    expect(onStarterSelect.mock.calls[0][0].prompt).toContain('portfolio website')

    act(() => {
      vi.advanceTimersByTime(300)
    })

    expect(localStorage.getItem(ONBOARDING_KEY)).toBe('true')
    expect(onComplete).toHaveBeenCalledTimes(1)
  })

  it('opens a blank workspace from the starter step', () => {
    const onOpenBlankWorkspace = vi.fn()
    const onComplete = vi.fn()

    render(<OnboardingTour forceShow onOpenBlankWorkspace={onOpenBlankWorkspace} onComplete={onComplete} />)

    advanceToStarterStep()
    fireEvent.click(screen.getByRole('button', { name: /open blank workspace/i }))

    expect(onOpenBlankWorkspace).toHaveBeenCalledTimes(1)

    act(() => {
      vi.advanceTimersByTime(300)
    })

    expect(localStorage.getItem(ONBOARDING_KEY)).toBe('true')
    expect(onComplete).toHaveBeenCalledTimes(1)
  })

  it('marks onboarding complete when continuing to the builder from the last step', () => {
    const onComplete = vi.fn()

    render(<OnboardingTour forceShow onComplete={onComplete} />)

    advanceToStarterStep()
    fireEvent.click(screen.getByText('Continue to builder'))

    act(() => {
      vi.advanceTimersByTime(300)
    })

    expect(localStorage.getItem(ONBOARDING_KEY)).toBe('true')
    expect(onComplete).toHaveBeenCalledTimes(1)
  })

  it('keeps fast starter prompts scoped to frontend-only builds', () => {
    for (const starter of onboardingStarters.filter((item) => item.mode === 'fast')) {
      expect(starter.prompt).toMatch(/no backend, no auth, no database, and no server runtime claims/i)
      expect(starter.prompt).not.toMatch(/lorem ipsum|placeholder content|coming soon/i)
    }
  })
})
