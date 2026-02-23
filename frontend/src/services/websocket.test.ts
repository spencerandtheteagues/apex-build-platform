import { describe, expect, it } from 'vitest'

import { isSocketIoEndpointUnavailableError } from './websocket'

describe('isSocketIoEndpointUnavailableError', () => {
  it('detects common socket.io 404 handshake failures', () => {
    expect(isSocketIoEndpointUnavailableError(new Error('websocket error 404'))).toBe(true)
    expect(isSocketIoEndpointUnavailableError({ message: 'xhr poll error' })).toBe(true)
    expect(isSocketIoEndpointUnavailableError('Not Found')).toBe(true)
  })

  it('does not classify unrelated connection errors as unsupported endpoint', () => {
    expect(isSocketIoEndpointUnavailableError(new Error('timeout exceeded'))).toBe(false)
    expect(isSocketIoEndpointUnavailableError({ message: 'ECONNRESET' })).toBe(false)
  })
})
