import { describe, expect, it, vi } from 'vitest'
import { WebSocketService } from './websocket'

describe('WebSocketService', () => {
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
})
