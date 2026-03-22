/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/services/api', () => ({
  default: {
    getAvailableModels: vi.fn(),
  },
}))

import apiService from '@/services/api'
import ModelSelector from './ModelSelector'

describe('ModelSelector', () => {
  it('hides Ollama for users without paid BYOK access', async () => {
    vi.mocked(apiService.getAvailableModels).mockResolvedValueOnce({
      success: true,
      data: {
        claude: [
          { id: 'claude-sonnet-4-6', name: 'Claude Sonnet 4.6', speed: 'medium', cost_tier: 'medium', description: 'Balanced quality and speed' },
        ],
        ollama: [
          { id: 'deepseek-r1:8b', name: 'DeepSeek-R1 (8b)', speed: 'variable', cost_tier: 'free', description: 'Reasoning model (local)' },
        ],
      },
    })

    render(<ModelSelector value="auto" canUseBYOK={false} />)

    fireEvent.click(screen.getByRole('button', { name: /auto/i }))

    await waitFor(() => {
      expect(screen.getByText('Claude')).toBeTruthy()
    })
    expect(screen.queryByText('Ollama (Local)')).toBeNull()
  })

  it('shows Ollama when paid BYOK access is enabled', async () => {
    vi.mocked(apiService.getAvailableModels).mockResolvedValueOnce({
      success: true,
      data: {
        claude: [
          { id: 'claude-sonnet-4-6', name: 'Claude Sonnet 4.6', speed: 'medium', cost_tier: 'medium', description: 'Balanced quality and speed' },
        ],
        ollama: [
          { id: 'deepseek-r1:8b', name: 'DeepSeek-R1 (8b)', speed: 'variable', cost_tier: 'free', description: 'Reasoning model (local)' },
        ],
      },
    })

    render(<ModelSelector value="auto" canUseBYOK />)

    fireEvent.click(screen.getByRole('button', { name: /auto/i }))

    await waitFor(() => {
      expect(screen.getByText('Ollama (Local)')).toBeTruthy()
    })
  })
})
