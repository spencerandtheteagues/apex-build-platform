/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import LiveActivityFeed from './LiveActivityFeed'

const makeThought = (id: string, content: string, provider = 'grok') => ({
  id,
  provider,
  type: 'action' as const,
  content,
  timestamp: new Date('2026-04-08T11:15:00Z'),
})

const defaultProps = () => ({
  aiThoughts: [],
  chatMessages: [],
  buildStatus: 'in_progress',
  interaction: undefined,
  isBuildActive: true,
  pendingQuestion: null,
  pendingPermissionRequests: [],
  buildPaused: false,
  onFocusChatInput: vi.fn(),
  onOpenIssues: vi.fn(),
  onResume: vi.fn(),
  buildCompleted: false,
  onOpenPreview: vi.fn(),
  isPreparingPreview: false,
})

describe('LiveActivityFeed', () => {
  const scrollIntoView = vi.fn()

  beforeEach(() => {
    scrollIntoView.mockReset()
    Object.defineProperty(window, 'requestAnimationFrame', {
      configurable: true,
      value: (callback: FrameRequestCallback) => {
        callback(0)
        return 1
      },
    })
    Object.defineProperty(window, 'cancelAnimationFrame', {
      configurable: true,
      value: vi.fn(),
    })
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('auto-scrolls to the latest activity when new agent updates arrive', () => {
    const props = defaultProps()
    const { rerender } = render(
      <LiveActivityFeed
        {...props}
        aiThoughts={[makeThought('thought-1', 'Claude started planning the scaffold.')]}
      />
    )

    scrollIntoView.mockClear()

    rerender(
      <LiveActivityFeed
        {...props}
        aiThoughts={[
          makeThought('thought-1', 'Claude started planning the scaffold.'),
          makeThought('thought-2', 'Grok is generating the upload flow.'),
        ]}
      />
    )

    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'end', behavior: 'auto' })
  })

  it('stops auto-scrolling after the user scrolls up and resumes on demand', () => {
    const thoughts = [
      makeThought('thought-1', 'Claude started planning the scaffold.'),
      makeThought('thought-2', 'ChatGPT is building the frontend shell.'),
    ]
    const props = defaultProps()
    const { rerender } = render(
      <LiveActivityFeed
        {...props}
        aiThoughts={thoughts}
      />
    )

    const feed = screen.getByLabelText('Live activity feed')
    let scrollTopValue = 120
    Object.defineProperty(feed, 'scrollHeight', {
      configurable: true,
      get: () => 1000,
    })
    Object.defineProperty(feed, 'clientHeight', {
      configurable: true,
      get: () => 300,
    })
    Object.defineProperty(feed, 'scrollTop', {
      configurable: true,
      get: () => scrollTopValue,
      set: (value: number) => {
        scrollTopValue = value
      },
    })

    fireEvent.scroll(feed)
    scrollIntoView.mockClear()

    rerender(
      <LiveActivityFeed
        {...props}
        aiThoughts={[
          ...thoughts,
          makeThought('thought-3', 'Grok is refining the file import validation.'),
        ]}
      />
    )

    expect(scrollIntoView).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: /latest/i }))

    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'end', behavior: 'smooth' })
  })
})
