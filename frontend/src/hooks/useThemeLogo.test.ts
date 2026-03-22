/* @vitest-environment jsdom */

import { renderHook } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { useThemeLogo } from './useThemeLogo'

describe('useThemeLogo', () => {
  it('uses the transparent platform logo for the red theme', () => {
    document.documentElement.setAttribute('data-ui-theme', 'red')

    const { result } = renderHook(() => useThemeLogo())

    expect(result.current).toBe('/apex-build-logo-transparent.png')
  })

  it('uses the blue logo variant for the blue theme', () => {
    document.documentElement.setAttribute('data-ui-theme', 'blue')

    const { result } = renderHook(() => useThemeLogo())

    expect(result.current).toBe('/logo-blue.png')
  })
})
