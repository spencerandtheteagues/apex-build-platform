/* @vitest-environment jsdom */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'

vi.mock('@/services/api', () => ({
  default: {
    getCurrentUser: vi.fn(() => null),
    isAuthenticated: vi.fn(() => false),
  },
}))

import { useStore } from '@/hooks/useStore'
import type { FSMWSMessageType } from '@/types'

const getState = () => useStore.getState()

describe('fsmSlice', () => {
  beforeEach(() => {
    getState().clearAllFSMStates()
  })

  afterEach(() => {
    getState().clearAllFSMStates()
    vi.useRealTimers()
  })

  it('starts with empty FSM state', () => {
    expect(getState().fsmStates.size).toBe(0)
    expect(getState().fsmActiveBuilds).toEqual([])
  })

  it('handleFSMStarted creates an entry and tracks the build as active', () => {
    getState().handleFSMStarted('build-1', {
      to_state: 'planning',
      event: 'start',
      progress: 0.0,
      step_id: 'init',
      timestamp: '2026-05-09T12:00:00Z',
    })

    const fsm = getState().getFSMState('build-1')
    expect(fsm).toBeDefined()
    expect(fsm?.currentState).toBe('planning')
    expect(fsm?.currentEvent).toBe('start')
    expect(fsm?.stepID).toBe('init')
    expect(getState().fsmActiveBuilds).toContain('build-1')
  })

  it('subsequent events update currentState and preserve previousState', () => {
    getState().handleFSMStarted('build-2', { to_state: 'planning' })
    getState().handleFSMPlanReady('build-2', {
      from_state: 'planning',
      to_state: 'executing',
      event: 'plan_ready',
      progress: 0.1,
    })

    const fsm = getState().getFSMState('build-2')
    expect(fsm?.previousState).toBe('planning')
    expect(fsm?.currentState).toBe('executing')
    expect(fsm?.currentEvent).toBe('plan_ready')
    expect(fsm?.progress).toBe(0.1)
  })

  it('preserves prior fields when an event omits them', () => {
    getState().handleFSMStarted('build-3', {
      to_state: 'planning',
      step_id: 'step-1',
      retry_count: 2,
    })
    getState().handleFSMStepComplete('build-3', { progress: 0.5 })

    const fsm = getState().getFSMState('build-3')
    expect(fsm?.stepID).toBe('step-1')
    expect(fsm?.retryCount).toBe(2)
    expect(fsm?.progress).toBe(0.5)
  })

  it('does not duplicate the build in fsmActiveBuilds across events', () => {
    getState().handleFSMStarted('build-4', { to_state: 'planning' })
    getState().handleFSMPlanReady('build-4', { to_state: 'executing' })
    getState().handleFSMStepComplete('build-4', { progress: 0.5 })

    const active = getState().fsmActiveBuilds.filter((id) => id === 'build-4')
    expect(active.length).toBe(1)
  })

  it('clearFSMState removes a single build', () => {
    getState().handleFSMStarted('build-5', { to_state: 'planning' })
    getState().handleFSMStarted('build-6', { to_state: 'planning' })
    getState().clearFSMState('build-5')

    expect(getState().getFSMState('build-5')).toBeUndefined()
    expect(getState().getFSMState('build-6')).toBeDefined()
    expect(getState().fsmActiveBuilds).not.toContain('build-5')
    expect(getState().fsmActiveBuilds).toContain('build-6')
  })

  it('clearAllFSMStates resets the slice', () => {
    getState().handleFSMStarted('build-7', { to_state: 'planning' })
    getState().handleFSMStarted('build-8', { to_state: 'planning' })
    getState().clearAllFSMStates()

    expect(getState().fsmStates.size).toBe(0)
    expect(getState().fsmActiveBuilds).toEqual([])
  })

  it('handleFSMEvent generic dispatcher accepts any FSM message type', () => {
    const msgType: FSMWSMessageType = 'build:fsm:checkpoint_created'
    getState().handleFSMEvent(msgType, 'build-9', {
      checkpoint_id: 'ckpt-1',
      to_state: 'executing',
    })

    const fsm = getState().getFSMState('build-9')
    expect(fsm?.checkpointID).toBe('ckpt-1')
    expect(fsm?.currentState).toBe('executing')
  })

  it('ignores FSM events when no build id is available', () => {
    getState().handleFSMEvent('build:fsm:started', '', { to_state: 'planning' })
    getState().handleFSMEvent('build:fsm:started', '   ', { to_state: 'planning' })

    expect(getState().fsmStates.size).toBe(0)
    expect(getState().fsmActiveBuilds).toEqual([])
  })

  it('falls back to a build id from the event payload', () => {
    getState().handleFSMEvent('build:fsm:started', '   ', {
      build_id: ' build-from-payload ',
      to_state: 'planning',
    })

    expect(getState().getFSMState('build-from-payload')?.currentState).toBe('planning')
    expect(getState().fsmActiveBuilds).toContain('build-from-payload')
  })

  it('rollback events surface checkpoint and error details', () => {
    getState().handleFSMValidationFail('build-10', {
      to_state: 'rolling_back',
      error: 'validator: smoke test failed',
      retry_count: 1,
    })
    getState().handleFSMRollbackComplete('build-10', {
      to_state: 'executing',
      checkpoint_id: 'ckpt-42',
    })

    const fsm = getState().getFSMState('build-10')
    expect(fsm?.errorMessage).toBe('validator: smoke test failed')
    expect(fsm?.checkpointID).toBe('ckpt-42')
    expect(fsm?.currentState).toBe('executing')
  })

  it('uses provided timestamp or falls back to now', () => {
    getState().handleFSMStarted('build-11', {
      to_state: 'planning',
      timestamp: '2026-05-09T01:23:45Z',
    })
    expect(getState().getFSMState('build-11')?.timestamp).toBe('2026-05-09T01:23:45Z')

    getState().handleFSMStarted('build-12', { to_state: 'planning' })
    const ts = getState().getFSMState('build-12')?.timestamp
    expect(ts).toBeDefined()
    expect(() => new Date(ts!).toISOString()).not.toThrow()
  })

  it('auto-clears terminal FSM states after the retention window', () => {
    vi.useFakeTimers()

    getState().handleFSMEvent('build:fsm:validation_pass', 'build-13', {
      to_state: 'completed',
    })

    expect(getState().getFSMState('build-13')).toBeDefined()
    vi.advanceTimersByTime(299999)
    expect(getState().getFSMState('build-13')).toBeDefined()
    vi.advanceTimersByTime(1)

    expect(getState().getFSMState('build-13')).toBeUndefined()
    expect(getState().fsmActiveBuilds).not.toContain('build-13')
  })
})
