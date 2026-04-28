import { describe, expect, it } from 'vitest'

import { getApiErrorMessage } from './errors'

describe('getApiErrorMessage', () => {
  it('uses the server auth response for invalid credentials', () => {
    expect(
      getApiErrorMessage({
        response: {
          status: 401,
          data: { error: 'incorrect email or password' },
        },
      }),
    ).toBe('Incorrect email or password')
  })

  it('falls back to the auth status message when the response has no body', () => {
    expect(
      getApiErrorMessage({
        response: {
          status: 401,
          data: {},
        },
      }),
    ).toBe('Incorrect email or password.')
  })

  it('turns Axios network failures into an actionable local backend message', () => {
    expect(
      getApiErrorMessage({
        isAxiosError: true,
        code: 'ERR_NETWORK',
        message: 'Network Error',
        config: {
          baseURL: '/api/v1',
          url: '/auth/login',
        },
      }),
    ).toBe(
      'Cannot reach the Apex API at /api/v1/auth/login. In local dev, start the backend on http://localhost:8080 and make sure the Vite proxy is running.',
    )
  })

  it('turns Axios timeouts into an actionable health message', () => {
    expect(
      getApiErrorMessage({
        isAxiosError: true,
        code: 'ECONNABORTED',
        message: 'timeout of 30000ms exceeded',
        config: {
          baseURL: 'https://api.apex-build.dev/api/v1',
          url: '/auth/login',
        },
      }),
    ).toBe(
      'The request to https://api.apex-build.dev/api/v1/auth/login timed out. Check the backend health and try again.',
    )
  })

  it('turns empty local proxy server errors into backend startup guidance', () => {
    expect(
      getApiErrorMessage({
        isAxiosError: true,
        response: {
          status: 500,
          data: '',
        },
        config: {
          baseURL: '/api/v1',
          url: '/auth/login',
        },
      }),
    ).toBe(
      'The local Apex API returned HTTP 500 for /api/v1/auth/login. Start or check the backend on http://localhost:8080, then retry.',
    )
  })
})
