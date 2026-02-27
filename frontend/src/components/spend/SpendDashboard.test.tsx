/* @vitest-environment jsdom */

import React from 'react'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/services/api', () => ({
  default: {
    get: vi.fn(),
  },
}))

import apiService from '@/services/api'
import SpendDashboard from './SpendDashboard'

describe('SpendDashboard', () => {
  beforeEach(() => {
    ;(apiService.get as any).mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  const mockApiResponses = (overrides: { summary?: any; breakdown?: any; history?: any } = {}) => {
    ;(apiService.get as any).mockImplementation(async (url: string) => {
      if (url === '/spend/summary') {
        return {
          data: overrides.summary ?? {
            daily_spend: 1.2345,
            monthly_spend: 42.5678,
            daily_count: 15,
            monthly_count: 350,
          },
        }
      }
      if (url.startsWith('/spend/breakdown')) {
        return {
          data: overrides.breakdown ?? {
            items: [
              { key: 'claude', billed_cost: 30.0, raw_cost: 18.0, input_tokens: 100000, output_tokens: 50000, count: 200 },
              { key: 'gpt4', billed_cost: 12.0, raw_cost: 8.0, input_tokens: 60000, output_tokens: 30000, count: 100 },
            ],
          },
        }
      }
      if (url.startsWith('/spend/history')) {
        return {
          data: overrides.history ?? {
            events: [
              {
                id: 1,
                created_at: '2026-02-26T10:00:00Z',
                provider: 'claude',
                model: 'claude-sonnet-4-5',
                agent_role: 'architect',
                build_id: 'b-123',
                input_tokens: 1000,
                output_tokens: 500,
                billed_cost: 0.0135,
                is_byok: false,
                target_file: 'main.go',
              },
            ],
          },
        }
      }
      if (url === '/spend/export/csv') {
        return { data: new Blob(['id,cost\n1,0.01'], { type: 'text/csv' }) }
      }
      throw new Error(`Unexpected GET ${url}`)
    })
  }

  it('shows loading state initially', () => {
    // Never resolve the API calls
    ;(apiService.get as any).mockReturnValue(new Promise(() => {}))

    render(<SpendDashboard />)
    expect(screen.getByText('Loading spend data...')).toBeTruthy()
  })

  it('renders summary cards with formatted data', async () => {
    mockApiResponses()

    render(<SpendDashboard />)

    await waitFor(() => {
      expect(screen.getByText('Spend Dashboard')).toBeTruthy()
      expect(screen.getByText("Today's Spend")).toBeTruthy()
      expect(screen.getByText('$1.2345')).toBeTruthy()
      expect(screen.getByText('$42.5678')).toBeTruthy()
    })
  })

  it('renders breakdown items', async () => {
    mockApiResponses()

    render(<SpendDashboard />)

    await waitFor(() => {
      expect(screen.getByText('Cost Breakdown')).toBeTruthy()
      // "claude" appears in both breakdown and history, so use getAllByText
      expect(screen.getAllByText('claude').length).toBeGreaterThanOrEqual(1)
      expect(screen.getByText('gpt4')).toBeTruthy()
      expect(screen.getByText('200 calls')).toBeTruthy()
    })
  })

  it('renders history table', async () => {
    mockApiResponses()

    render(<SpendDashboard />)

    await waitFor(() => {
      expect(screen.getByText('Recent Activity')).toBeTruthy()
      expect(screen.getByText('architect')).toBeTruthy()
      // Cost text includes $ prefix; use a regex to handle whitespace between $ and number
      expect(screen.getByText(/\$\s*0\.0135/)).toBeTruthy()
    })
  })

  it('shows empty breakdown message when no data', async () => {
    mockApiResponses({ breakdown: { items: [] } })

    render(<SpendDashboard />)

    await waitFor(() => {
      expect(screen.getByText('No spend data yet')).toBeTruthy()
    })
  })

  it('has an Export CSV button', async () => {
    mockApiResponses()

    render(<SpendDashboard />)

    await waitFor(() => {
      expect(screen.getByText('Export CSV')).toBeTruthy()
    })
  })
})
