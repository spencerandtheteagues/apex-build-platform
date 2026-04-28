/* @vitest-environment jsdom */

import { renderHook } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { useThemeLogo } from './useThemeLogo'

describe('useThemeLogo', () => {
  it('uses the metallic platform mark for the fixed production theme', () => {
    document.documentElement.setAttribute('data-ui-theme', 'red')

    const { result } = renderHook(() => useThemeLogo())

    expect(result.current).toBe('/apex-build-mark-metal.png')
  })
})
