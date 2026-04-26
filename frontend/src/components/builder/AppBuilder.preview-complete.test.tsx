/* @vitest-environment jsdom */

import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import {
  act,
  AppBuilder,
  apiService,
  buildDetail,
  fireEvent,
  installWebSocketMock,
  MOCK_HISTORY_BUILD_ID,
  openMockedBuild,
  primeAppBuilderHistoryTestEnv,
  render,
  screen,
  storeMocks,
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

  it('opens a near-complete frontend preview from streamed files when canonical artifacts are not ready', async () => {
    const onNavigateToIDE = vi.fn()

    ;(apiService.getCompletedBuild as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
      status: 'reviewing',
      progress: 96,
      live: false,
      files: [
        {
          path: 'src/App.tsx',
          content: 'export default function App(){return <main>Preview ready</main>}',
          language: 'typescript',
        },
      ],
    }))
    ;(apiService.getBuildDetails as any).mockResolvedValue(buildDetail({
      id: MOCK_HISTORY_BUILD_ID,
      build_id: MOCK_HISTORY_BUILD_ID,
      status: 'reviewing',
      progress: 96,
      live: false,
      files: [
        {
          path: 'src/App.tsx',
          content: 'export default function App(){return <main>Preview ready</main>}',
          language: 'typescript',
        },
      ],
    }))
    ;(apiService.applyBuildArtifacts as any).mockRejectedValueOnce(new Error('Artifacts are still syncing'))
    ;(apiService.createFile as any).mockResolvedValue({})

    render(<AppBuilder onNavigateToIDE={onNavigateToIDE} />)

    await openMockedBuild()

    const previewButtons = await screen.findAllByRole('button', { name: /open frontend preview/i })
    fireEvent.click(previewButtons[0])

    await waitFor(() => {
      expect(apiService.applyBuildArtifacts).toHaveBeenCalledWith(MOCK_HISTORY_BUILD_ID, expect.any(Object))
      expect(storeMocks.createProject).toHaveBeenCalled()
      expect(apiService.createFile).toHaveBeenCalledWith(77, expect.objectContaining({
        path: 'src/App.tsx',
        content: expect.stringContaining('Preview ready'),
      }))
      expect(onNavigateToIDE).toHaveBeenCalledWith({ target: 'preview', projectId: 77 })
    })
  })
})
