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
  waitFor,
  teardownAppBuilderHistoryTestEnv,
} from './AppBuilder.history.shared'

describe('AppBuilder history open flow', () => {
  beforeEach(() => {
    primeAppBuilderHistoryTestEnv()
  })

  afterEach(async () => {
    await teardownAppBuilderHistoryTestEnv()
  })

  it('keeps the external build_id when reopening a saved build with a numeric database id', async () => {
    const reopened = buildDetail({
      id: 501,
      build_id: MOCK_HISTORY_BUILD_ID,
      status: 'failed',
      progress: 91,
      live: false,
      error: 'Recovered from explicit history open',
    })

    ;(apiService.getCompletedBuild as any).mockResolvedValue(reopened)
    ;(apiService.getBuildDetails as any).mockResolvedValue(reopened)
    ;(apiService.sendBuildMessage as any).mockResolvedValue({
      interaction: { messages: [] },
      live: true,
    })

    render(<AppBuilder />)

    fireEvent.click(await screen.findByRole('button', { name: /open mocked build/i }))
    fireEvent.click(await screen.findByRole('button', { name: /restart failed build/i }))

    await waitFor(() => {
      expect(apiService.sendBuildMessage).toHaveBeenCalledWith(
        MOCK_HISTORY_BUILD_ID,
        expect.any(String),
        expect.objectContaining({ command: 'restart_failed' })
      )
    })
  })

  it('opens a previous build only after the user selects it from history', async () => {
    ;(apiService.getCompletedBuild as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
      status: 'failed',
      progress: 91,
      live: false,
      error: 'Recovered from explicit history open',
    }))
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
      status: 'failed',
      progress: 91,
      live: false,
      error: 'Recovered from explicit history open',
    }))

    render(<AppBuilder />)

    await screen.findByPlaceholderText(/Describe the app you want to build/i)
    expect(apiService.getCompletedBuild).not.toHaveBeenCalled()

    fireEvent.click(await screen.findByRole('button', { name: /open mocked build/i }))

    await screen.findByRole('button', { name: /restart failed build/i })
    expect(apiService.getCompletedBuild).toHaveBeenCalledWith(MOCK_HISTORY_BUILD_ID)
  })
})
