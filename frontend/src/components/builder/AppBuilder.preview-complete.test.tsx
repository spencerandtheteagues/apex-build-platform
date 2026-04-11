/* @vitest-environment jsdom */

import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import {
  act,
  AppBuilder,
  apiService,
  buildDetail,
  installWebSocketMock,
  MOCK_HISTORY_BUILD_ID,
  openMockedBuild,
  primeAppBuilderHistoryTestEnv,
  render,
  teardownAppBuilderHistoryTestEnv,
  vi,
  waitFor,
} from './AppBuilder.history.shared'

describe('AppBuilder preview completion flow', () => {
  beforeEach(() => {
    primeAppBuilderHistoryTestEnv()
  })

  afterEach(async () => {
    await teardownAppBuilderHistoryTestEnv()
  })

  it('auto-opens the preview workspace when a live build completes successfully', async () => {
    const connections = installWebSocketMock()
    const onNavigateToIDE = vi.fn()

    ;(apiService.getCompletedBuild as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
      status: 'in_progress',
      live: true,
      websocket_url: 'wss://runtime.example/ws/build/history-build-1',
    }))
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
      status: 'in_progress',
      live: true,
      websocket_url: 'wss://runtime.example/ws/build/history-build-1',
    }))

    render(<AppBuilder onNavigateToIDE={onNavigateToIDE} />)

    await openMockedBuild()

    await waitFor(() => {
      expect(connections).toHaveLength(1)
    })

    await act(async () => {
      connections[0]?.onopen?.()
      connections[0]?.onmessage?.({
        data: JSON.stringify({
          type: 'build:completed',
          build_id: MOCK_HISTORY_BUILD_ID,
          data: {
            status: 'completed',
            files_count: 3,
            files: [
              {
                path: 'src/App.tsx',
                content: 'export default function App(){return null}',
                language: 'typescript',
              },
            ],
          },
        }),
      } as MessageEvent)
    })

    await waitFor(() => {
      expect(apiService.applyBuildArtifacts).toHaveBeenCalledWith(MOCK_HISTORY_BUILD_ID)
      expect(onNavigateToIDE).toHaveBeenCalledWith({ target: 'preview', projectId: 42 })
    })
  })
})
