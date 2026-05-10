import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/hooks/useStore', () => {
  const storeMocks = {
    handleFSMEvent: vi.fn(),
  }
  return {
    __storeMocks: storeMocks,
    useStore: {
      getState: () => ({
        handleFSMEvent: storeMocks.handleFSMEvent,
      }),
    },
  }
})

import * as mockedStore from '@/hooks/useStore'
import { WebSocketService } from './websocket'

const storeMocks = (mockedStore as any).__storeMocks

describe('WebSocketService', () => {
  beforeEach(() => {
    storeMocks.handleFSMEvent.mockClear()
  })

  it('creates an instance', () => {
    const svc = new WebSocketService()
    expect(svc.isConnected()).toBe(false)
    expect(svc.getCurrentRoom()).toBeNull()
  })

  it('tracks listeners', () => {
    const svc = new WebSocketService()
    const handler = vi.fn()
    const unsub = svc.on('user-joined', handler)
    expect(typeof unsub).toBe('function')
    unsub()
  })

  it('routes FSM bridge messages to the store using the root build id', () => {
    const svc = new WebSocketService()
    ;(svc as any).handleMessage(JSON.stringify({
      type: 'build:fsm:step_complete',
      build_id: 'build-root',
      data: {
        to_state: 'executing',
        progress: 0.4,
      },
    }))

    expect(storeMocks.handleFSMEvent).toHaveBeenCalledWith(
      'build:fsm:step_complete',
      'build-root',
      {
        to_state: 'executing',
        progress: 0.4,
      },
    )
  })
})
