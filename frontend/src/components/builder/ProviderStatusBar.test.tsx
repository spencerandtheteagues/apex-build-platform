/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import ProviderStatusBar from './ProviderStatusBar'

describe('ProviderStatusBar', () => {
  it('renders provider model selectors and forwards selection changes', () => {
    const onModelSelect = vi.fn()

    render(
      <ProviderStatusBar
        providerPanels={[
          { provider: 'gpt4', status: 'working', statusLabel: 'WORKING', liveModelName: 'ChatGPT 5.4 Pro', available: true },
          { provider: 'gemini', status: 'idle', statusLabel: 'IDLE', liveModelName: 'Gemini 3.1 Pro', available: true },
          { provider: 'grok', status: 'idle', statusLabel: 'IDLE', liveModelName: 'Grok 4.20', available: true },
          { provider: 'claude', status: 'idle', statusLabel: 'IDLE', liveModelName: 'Claude Opus 4.6', available: true },
          { provider: 'ollama', status: 'idle', statusLabel: 'IDLE', liveModelName: 'Kimi K2.6', available: true },
        ]}
        hasBYOK={false}
        isBuildActive
        selectedModels={{ gpt4: 'auto', gemini: 'auto', grok: 'auto', claude: 'auto', ollama: 'auto' }}
        modelOptions={{
          gpt4: [{ id: 'gpt-5.4-pro', name: 'ChatGPT 5.4 Pro' }],
          gemini: [{ id: 'gemini-3.1-pro', name: 'Gemini 3.1 Pro' }],
          grok: [{ id: 'grok-4.20-0309-reasoning', name: 'Grok 4.20' }],
          claude: [{ id: 'claude-opus-4-6', name: 'Claude Opus 4.6' }],
          ollama: [{ id: 'kimi-k2.6', name: 'Kimi K2.6' }],
        }}
        onModelSelect={onModelSelect}
      />
    )

    fireEvent.change(screen.getByLabelText('ChatGPT model'), {
      target: { value: 'gpt-5.4-pro' },
    })

    expect(onModelSelect).toHaveBeenCalledWith('gpt4', 'gpt-5.4-pro')
    expect((screen.getByLabelText('Kimi / Local model') as HTMLSelectElement).disabled).toBe(true)
    expect(screen.getByText('BYOK ONLY')).toBeTruthy()
  })
})
