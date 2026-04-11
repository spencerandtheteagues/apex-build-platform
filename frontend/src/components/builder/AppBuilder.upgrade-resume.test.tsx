/* @vitest-environment jsdom */

import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import {
  AppBuilder,
  apiService,
  buildDetail,
  primeAppBuilderHistoryTestEnv,
  render,
  teardownAppBuilderHistoryTestEnv,
  waitFor,
} from './AppBuilder.history.shared'

describe('AppBuilder upgrade resume flow', () => {
  beforeEach(() => {
    primeAppBuilderHistoryTestEnv()
  })

  afterEach(async () => {
    await teardownAppBuilderHistoryTestEnv()
  })

  it('restores the same build after returning from upgrade checkout', async () => {
    window.history.replaceState({}, '', '/?upgrade=success&resume_build=history-build-1')
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail({
      id: 'history-build-1',
      build_id: 'history-build-1',
      status: 'in_progress',
      live: false,
    }))

    render(<AppBuilder />)

    await waitFor(() => {
      expect(apiService.getBuildDetails).toHaveBeenCalledWith('history-build-1')
    })
  })
})
