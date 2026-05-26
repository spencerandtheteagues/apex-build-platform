/* @vitest-environment jsdom */

import React from 'react'
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import EditorLoadingFallback from './EditorLoadingFallback'

describe('EditorLoadingFallback', () => {
  it('exposes an accessible, busy status region', () => {
    render(<EditorLoadingFallback />)
    const status = screen.getByRole('status')
    expect(status).toBeTruthy()
    expect(status.getAttribute('aria-busy')).toBe('true')
    // role="status" already implies aria-live="polite"; an explicit one would
    // cause double-announce in some screen readers, so it must be absent.
    expect(status.getAttribute('aria-live')).toBeNull()
    // No aria-label: the visible text is the accessible name.
    expect(status.getAttribute('aria-label')).toBeNull()
  })

  it('preserves the original visible "Loading editor..." text', () => {
    render(<EditorLoadingFallback />)
    expect(screen.getByText('Loading editor...')).toBeTruthy()
  })

  it('uses a custom label as both visible text and accessible status', () => {
    render(<EditorLoadingFallback label="Loading code editor" />)
    const status = screen.getByRole('status')
    expect(status.textContent).toContain('Loading code editor')
  })

  it('hides the decorative skeleton from assistive technology', () => {
    const { container } = render(<EditorLoadingFallback />)
    expect(container.querySelector('[aria-hidden="true"]')).toBeTruthy()
  })

  it('disables the shimmer under prefers-reduced-motion', () => {
    const { container } = render(<EditorLoadingFallback />)
    // motion-reduce:animate-none gates the pulse on prefers-reduced-motion: reduce.
    expect(container.querySelector('.motion-reduce\\:animate-none')).toBeTruthy()
  })
})
