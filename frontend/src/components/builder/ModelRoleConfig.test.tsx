/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

// Mock the UI barrel to avoid transitive useStore/localStorage dependency
vi.mock('@/components/ui', () => ({
  Card: ({ children, ...props }: any) => <div data-testid="card" {...props}>{children}</div>,
  CardContent: ({ children, ...props }: any) => <div data-testid="card-content" {...props}>{children}</div>,
}))

import ModelRoleConfig from './ModelRoleConfig'

const ALL_AVAILABLE: Record<string, string> = {
  claude: 'available',
  gpt4: 'available',
  gemini: 'available',
  grok: 'available',
}

const GROK_OFFLINE: Record<string, string> = {
  claude: 'available',
  gpt4: 'available',
  gemini: 'available',
  grok: 'unavailable',
}

const WITH_OLLAMA: Record<string, string> = {
  claude: 'available',
  gpt4: 'available',
  gemini: 'available',
  grok: 'available',
  ollama: 'available',
}

const OLLAMA_OFFLINE: Record<string, string> = {
  claude: 'available',
  gpt4: 'available',
  gemini: 'available',
  grok: 'available',
  ollama: 'unavailable',
}

describe('ModelRoleConfig', () => {
  it('renders Auto mode by default with default assignments', () => {
    render(
      <ModelRoleConfig
        mode="auto"
        onModeChange={vi.fn()}
        assignments={{}}
        onAssignmentsChange={vi.fn()}
        providerStatuses={ALL_AVAILABLE}
      />
    )

    expect(screen.getByText('Model Configuration')).toBeTruthy()
    expect(screen.getByText('Auto')).toBeTruthy()
    expect(screen.getByText('Manual')).toBeTruthy()

    // Auto mode shows default assignment labels
    expect(screen.getByText('Architect')).toBeTruthy()
    expect(screen.getByText('Coder')).toBeTruthy()
    expect(screen.getByText('Tester')).toBeTruthy()
    expect(screen.getByText('DevOps')).toBeTruthy()

    // Default providers shown as badges
    expect(screen.getByText('Claude')).toBeTruthy()
    expect(screen.getByText('ChatGPT')).toBeTruthy()
    expect(screen.getByText('Gemini')).toBeTruthy()
    expect(screen.getByText('Grok')).toBeTruthy()
  })

  it('switches to manual mode when Manual button clicked', () => {
    const onModeChange = vi.fn()
    render(
      <ModelRoleConfig
        mode="auto"
        onModeChange={onModeChange}
        assignments={{}}
        onAssignmentsChange={vi.fn()}
        providerStatuses={ALL_AVAILABLE}
      />
    )

    fireEvent.click(screen.getByText('Manual'))
    expect(onModeChange).toHaveBeenCalledWith('manual')
  })

  it('shows provider cards with role chips in manual mode', () => {
    render(
      <ModelRoleConfig
        mode="manual"
        onModeChange={vi.fn()}
        assignments={{ architect: 'claude', coder: 'gpt4', tester: 'gemini', devops: 'grok' }}
        onAssignmentsChange={vi.fn()}
        providerStatuses={ALL_AVAILABLE}
      />
    )

    // All provider cards shown with online status
    expect(screen.getAllByText('Online').length).toBe(4)

    // Each provider should have clickable role chips
    // "Architect" appears once per provider card = 4 times, plus we look for assigned state
    expect(screen.getAllByText('Architect').length).toBe(4)
    expect(screen.getAllByText('Coder').length).toBe(4)
    expect(screen.getAllByText('Tester').length).toBe(4)
    expect(screen.getAllByText('DevOps').length).toBe(4)
  })

  it('assigns role to provider when chip is clicked', () => {
    const onAssignmentsChange = vi.fn()
    render(
      <ModelRoleConfig
        mode="manual"
        onModeChange={vi.fn()}
        assignments={{ architect: 'claude', coder: 'gpt4' }}
        onAssignmentsChange={onAssignmentsChange}
        providerStatuses={ALL_AVAILABLE}
      />
    )

    // Click "Tester" chip on the Gemini card (3rd provider)
    const testerChips = screen.getAllByText('Tester')
    // Chips appear in provider order: Claude(0), ChatGPT(1), Gemini(2), Grok(3)
    fireEvent.click(testerChips[2])

    expect(onAssignmentsChange).toHaveBeenCalledWith({
      architect: 'claude',
      coder: 'gpt4',
      tester: 'gemini',
    })
  })

  it('unassigns role when clicking same chip again', () => {
    const onAssignmentsChange = vi.fn()
    render(
      <ModelRoleConfig
        mode="manual"
        onModeChange={vi.fn()}
        assignments={{ architect: 'claude', coder: 'gpt4' }}
        onAssignmentsChange={onAssignmentsChange}
        providerStatuses={ALL_AVAILABLE}
      />
    )

    // Click "Architect" chip on Claude card — should unassign since it's already assigned there
    const architectChips = screen.getAllByText('Architect')
    fireEvent.click(architectChips[0]) // Claude's architect chip

    expect(onAssignmentsChange).toHaveBeenCalledWith({
      coder: 'gpt4',
    })
  })

  it('moves role when clicking chip on different provider', () => {
    const onAssignmentsChange = vi.fn()
    render(
      <ModelRoleConfig
        mode="manual"
        onModeChange={vi.fn()}
        assignments={{ architect: 'claude', coder: 'gpt4' }}
        onAssignmentsChange={onAssignmentsChange}
        providerStatuses={ALL_AVAILABLE}
      />
    )

    // Click "Architect" on ChatGPT card — should move architect from claude to gpt4
    const architectChips = screen.getAllByText('Architect')
    fireEvent.click(architectChips[1]) // ChatGPT's architect chip

    expect(onAssignmentsChange).toHaveBeenCalledWith({
      architect: 'gpt4',
      coder: 'gpt4',
    })
  })

  it('shows validation error when required roles not assigned', () => {
    render(
      <ModelRoleConfig
        mode="manual"
        onModeChange={vi.fn()}
        assignments={{}} // nothing assigned
        onAssignmentsChange={vi.fn()}
        providerStatuses={ALL_AVAILABLE}
      />
    )

    // Role chips exist across provider cards + validation message mentions them
    expect(screen.getAllByText('Architect').length).toBeGreaterThanOrEqual(4)
    expect(screen.getAllByText('Coder').length).toBeGreaterThanOrEqual(4)
    // Validation message should appear
    expect(screen.getByText(/Assign at least/)).toBeTruthy()
  })

  it('hides validation error when required roles are assigned', () => {
    render(
      <ModelRoleConfig
        mode="manual"
        onModeChange={vi.fn()}
        assignments={{ architect: 'claude', coder: 'gpt4' }}
        onAssignmentsChange={vi.fn()}
        providerStatuses={ALL_AVAILABLE}
      />
    )

    expect(screen.queryByText(/Assign at least/)).toBeNull()
  })

  it('shows offline badge for unavailable providers', () => {
    render(
      <ModelRoleConfig
        mode="manual"
        onModeChange={vi.fn()}
        assignments={{ architect: 'claude', coder: 'gpt4', tester: 'gemini' }}
        onAssignmentsChange={vi.fn()}
        providerStatuses={GROK_OFFLINE}
      />
    )

    expect(screen.getAllByText('Online').length).toBe(3)
    expect(screen.getByText('Offline')).toBeTruthy()
  })

  it('disables chips on unavailable providers', () => {
    const onAssignmentsChange = vi.fn()
    render(
      <ModelRoleConfig
        mode="manual"
        onModeChange={vi.fn()}
        assignments={{ architect: 'claude' }}
        onAssignmentsChange={onAssignmentsChange}
        providerStatuses={GROK_OFFLINE}
      />
    )

    // Click a chip on the Grok card (4th provider, index 3)
    const coderChips = screen.getAllByText('Coder')
    fireEvent.click(coderChips[3]) // Grok's coder chip — should be disabled

    // Should NOT have been called since grok is offline
    expect(onAssignmentsChange).not.toHaveBeenCalled()
  })

  it('initializes defaults when switching to manual mode', () => {
    const onAssignmentsChange = vi.fn()
    render(
      <ModelRoleConfig
        mode="auto"
        onModeChange={vi.fn()}
        assignments={{}}
        onAssignmentsChange={onAssignmentsChange}
        providerStatuses={ALL_AVAILABLE}
      />
    )

    fireEvent.click(screen.getByText('Manual'))

    // Should populate with default assignments
    expect(onAssignmentsChange).toHaveBeenCalledWith({
      architect: 'claude',
      coder: 'gpt4',
      tester: 'gemini',
      devops: 'grok',
    })
  })

  it('does not show Ollama when providerStatuses has no ollama key', () => {
    render(
      <ModelRoleConfig
        mode="manual"
        onModeChange={vi.fn()}
        assignments={{ architect: 'claude', coder: 'gpt4' }}
        onAssignmentsChange={vi.fn()}
        providerStatuses={ALL_AVAILABLE}
      />
    )

    // 4 platform providers shown
    expect(screen.getAllByText('Online').length).toBe(4)
    expect(screen.queryByText('Ollama')).toBeNull()
  })

  it('shows Ollama card when providerStatuses includes ollama', () => {
    render(
      <ModelRoleConfig
        mode="manual"
        onModeChange={vi.fn()}
        assignments={{ architect: 'claude', coder: 'gpt4' }}
        onAssignmentsChange={vi.fn()}
        providerStatuses={WITH_OLLAMA}
      />
    )

    // 5 providers shown (4 platform + Ollama)
    expect(screen.getAllByText('Online').length).toBe(5)
    expect(screen.getByText('Ollama')).toBeTruthy()
    expect(screen.getByText('Local')).toBeTruthy()
  })

  it('shows Ollama as offline when ollama status is unavailable', () => {
    render(
      <ModelRoleConfig
        mode="manual"
        onModeChange={vi.fn()}
        assignments={{ architect: 'claude', coder: 'gpt4' }}
        onAssignmentsChange={vi.fn()}
        providerStatuses={OLLAMA_OFFLINE}
      />
    )

    // 4 online + 1 offline
    expect(screen.getAllByText('Online').length).toBe(4)
    expect(screen.getByText('Offline')).toBeTruthy()
    expect(screen.getByText('Ollama')).toBeTruthy()
  })

  it('allows assigning roles to Ollama when available', () => {
    const onAssignmentsChange = vi.fn()
    render(
      <ModelRoleConfig
        mode="manual"
        onModeChange={vi.fn()}
        assignments={{ architect: 'claude', coder: 'gpt4' }}
        onAssignmentsChange={onAssignmentsChange}
        providerStatuses={WITH_OLLAMA}
      />
    )

    // Click "Tester" chip on Ollama card (5th provider, index 4)
    const testerChips = screen.getAllByText('Tester')
    fireEvent.click(testerChips[4])

    expect(onAssignmentsChange).toHaveBeenCalledWith({
      architect: 'claude',
      coder: 'gpt4',
      tester: 'ollama',
    })
  })
})
