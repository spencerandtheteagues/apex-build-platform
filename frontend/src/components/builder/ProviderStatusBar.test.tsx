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
          { provider: 'gpt4', status: 'working', statusLabel: 'WORKING', liveModelName: 'ChatGPT 5.4 Codex', available: true },
          { provider: 'gemini', status: 'idle', statusLabel: 'IDLE', liveModelName: 'Gemini 3.1 Pro', available: true },
          { provider: 'grok', status: 'idle', statusLabel: 'IDLE', liveModelName: 'Grok 4.20', available: true },
          { provider: 'claude', status: 'idle', statusLabel: 'IDLE', liveModelName: 'Claude Opus 4.7', available: true },
          { provider: 'ollama', status: 'idle', statusLabel: 'IDLE', liveModelName: 'Kimi K2.6', available: true },
        ]}
        hasBYOK={false}
        isBuildActive
        selectedModels={{ gpt4: 'auto', gemini: 'auto', grok: 'auto', claude: 'auto', ollama: 'auto' }}
        modelOptions={{
          gpt4: [{ id: 'gpt-5.4-codex', name: 'ChatGPT 5.4 Codex' }],
          gemini: [{ id: 'gemini-3.1-pro', name: 'Gemini 3.1 Pro' }],
          grok: [{ id: 'grok-4.20-0309-reasoning', name: 'Grok 4.20' }],
          claude: [{ id: 'claude-opus-4-7', name: 'Claude Opus 4.7' }],
          ollama: [{ id: 'kimi-k2.6', name: 'Kimi K2.6' }],
        }}
        onModelSelect={onModelSelect}
      />
    )

    fireEvent.change(screen.getByLabelText('ChatGPT model'), {
      target: { value: 'gpt-5.4-codex' },
    })

    expect(onModelSelect).toHaveBeenCalledWith('gpt4', 'gpt-5.4-codex')
    expect((screen.getByLabelText('Ollama model') as HTMLSelectElement).disabled).toBe(false)
    expect(screen.getByText('Hosted Ready')).toBeTruthy()
    expect(screen.getByText('Hosted Ollama Cloud route is available.')).toBeTruthy()
    expect(screen.getAllByText('Cloud').length).toBeGreaterThan(0)
  })

  it('treats ollama as a first-class configurable provider when BYOK is enabled', () => {
    const onModelSelect = vi.fn()

    render(
      <ProviderStatusBar
        providerPanels={[
          { provider: 'gpt4', status: 'completed', statusLabel: 'DONE', liveModelName: 'ChatGPT 5.4 Codex', available: true },
          { provider: 'gemini', status: 'completed', statusLabel: 'DONE', liveModelName: 'Gemini 3.1 Pro', available: true },
          { provider: 'grok', status: 'completed', statusLabel: 'DONE', liveModelName: 'Grok 4.20', available: true },
          { provider: 'claude', status: 'completed', statusLabel: 'DONE', liveModelName: 'Claude Opus 4.7', available: true },
          { provider: 'ollama', status: 'working', statusLabel: 'WORKING', liveModelName: 'Kimi K2.6', available: true },
        ]}
        hasBYOK
        isBuildActive
        selectedModels={{ gpt4: 'auto', gemini: 'auto', grok: 'auto', claude: 'auto', ollama: 'auto' }}
        modelOptions={{
          gpt4: [{ id: 'gpt-5.4-codex', name: 'ChatGPT 5.4 Codex' }],
          gemini: [{ id: 'gemini-3.1-pro', name: 'Gemini 3.1 Pro' }],
          grok: [{ id: 'grok-4.20-0309-reasoning', name: 'Grok 4.20' }],
          claude: [{ id: 'claude-opus-4-7', name: 'Claude Opus 4.7' }],
          ollama: [{ id: 'kimi-k2.6', name: 'Kimi K2.6' }],
        }}
        onModelSelect={onModelSelect}
      />
    )

    fireEvent.change(screen.getByLabelText('Ollama model'), {
      target: { value: 'kimi-k2.6' },
    })

    expect(onModelSelect).toHaveBeenCalledWith('ollama', 'kimi-k2.6')
    expect((screen.getByLabelText('Ollama model') as HTMLSelectElement).disabled).toBe(false)
    expect(screen.getByText('BYOK Ready')).toBeTruthy()
    expect(screen.getByText('BYOK routing is enabled for this build.')).toBeTruthy()
    expect(screen.getAllByText('Kimi K2.6').length).toBeGreaterThan(0)
  })
})
