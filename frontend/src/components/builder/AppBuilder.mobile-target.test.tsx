/* @vitest-environment jsdom */

import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import {
  AppBuilder,
  apiService,
  fireEvent,
  installWebSocketMock,
  primeAppBuilderHistoryTestEnv,
  render,
  screen,
  teardownAppBuilderHistoryTestEnv,
  waitFor,
} from './AppBuilder.history.shared'

describe('AppBuilder mobile target path', () => {
  beforeEach(() => {
    primeAppBuilderHistoryTestEnv()
    installWebSocketMock()
    ;(apiService.startBuild as any).mockResolvedValue({
      build_id: 'mobile-build-1',
      websocket_url: 'ws://runtime.example/ws/build/mobile-build-1',
      status: 'planning',
    })
  })

  afterEach(async () => {
    await teardownAppBuilderHistoryTestEnv()
  })

  it('sends explicit Expo mobile metadata when Mobile App is selected before launch', async () => {
    render(<AppBuilder />)

    fireEvent.change(await screen.findByPlaceholderText(/Describe the app you want to build/i), {
      target: {
        value: 'Build a field inspection mobile app with camera, offline drafts, and job photos.',
      },
    })

    fireEvent.click(screen.getByTestId('target-path-mobile_expo'))
    await screen.findByTestId('mobile-target-panel')
    fireEvent.click(screen.getByTestId('mobile-capability-camera'))

    fireEvent.click(screen.getByTestId('launch-build-button'))

    await waitFor(() => {
      expect(apiService.buildPreflight).toHaveBeenCalledWith(expect.objectContaining({
        target_platform: 'mobile_expo',
        mobile_framework: 'expo-react-native',
        mobile_release_level: 'source_only',
        mobile_dependency_policy: 'expo-allowlist',
      }))
      expect(apiService.startBuild).toHaveBeenCalledWith(expect.objectContaining({
        mode: 'full',
        target_platform: 'mobile_expo',
        mobile_framework: 'expo-react-native',
        mobile_release_level: 'source_only',
        mobile_dependency_policy: 'expo-allowlist',
        mobile_platforms: ['android', 'ios'],
      }))
    })

    const startPayload = (apiService.startBuild as any).mock.calls[0][0]
    expect(startPayload.mobile_capabilities).toEqual(expect.arrayContaining([
      'offlineMode',
      'fileUploads',
      'camera',
    ]))
  })

  it('does not send the visual default target so backend prompt classification can still infer mobile', async () => {
    render(<AppBuilder />)

    fireEvent.change(await screen.findByPlaceholderText(/Describe the app you want to build/i), {
      target: {
        value: 'Build an iOS and Android customer booking app with push reminders.',
      },
    })

    fireEvent.click(screen.getByTestId('launch-build-button'))

    await waitFor(() => {
      expect(apiService.startBuild).toHaveBeenCalled()
    })

    const preflightPayload = (apiService.buildPreflight as any).mock.calls
      .map((call: any[]) => call[0])
      .find((payload: any) => payload?.description === 'Build an iOS and Android customer booking app with push reminders.')
    const startPayload = (apiService.startBuild as any).mock.calls[0][0]
    expect(preflightPayload).toBeTruthy()
    expect(preflightPayload).not.toHaveProperty('target_platform')
    expect(startPayload).not.toHaveProperty('target_platform')
  })
})
