/* @vitest-environment jsdom */

import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import {
  AppBuilder,
  apiService,
  buildDetail,
  fireEvent,
  MOCK_HISTORY_BUILD_ID,
  primeAppBuilderHistoryTestEnv,
  render,
  screen,
  teardownAppBuilderHistoryTestEnv,
  waitFor,
} from './AppBuilder.history.shared'

describe('AppBuilder terminal self-heal flow', () => {
  beforeEach(() => {
    primeAppBuilderHistoryTestEnv()
  })

  afterEach(async () => {
    await teardownAppBuilderHistoryTestEnv()
  })

  it('self-heals an active build snapshot when the server already reports a terminal completion', async () => {
    ;(apiService.getCompletedBuild as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
      status: 'in_progress',
      progress: 82,
      live: false,
    }))
    ;(apiService.getBuildDetails as any)
      .mockResolvedValueOnce(buildDetail({
        id: MOCK_HISTORY_BUILD_ID,
        build_id: MOCK_HISTORY_BUILD_ID,
        status: 'in_progress',
        progress: 82,
        live: false,
      }))
      .mockResolvedValueOnce(buildDetail({
        id: MOCK_HISTORY_BUILD_ID,
        build_id: MOCK_HISTORY_BUILD_ID,
        status: 'completed',
        progress: 100,
        live: false,
      }))

    render(<AppBuilder />)

    fireEvent.click(await screen.findByRole('button', { name: /open mocked build/i }))

    await waitFor(() => {
      expect((apiService.getBuildDetails as any).mock.calls.length).toBeGreaterThanOrEqual(2)
      expect(screen.getAllByRole('button', { name: /^Preview$/i }).length).toBeGreaterThan(1)
    })
  })
})
