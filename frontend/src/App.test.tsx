/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const selectProjectMock = vi.fn()

vi.mock('./services/api', () => ({
  default: {
    refreshToken: vi.fn(),
    clearStoredAuth: vi.fn(),
  },
}))

vi.mock('./hooks/useStore', () => ({
  useStore: () => ({
    user: { id: 7, username: 'tester', subscription_type: 'free' },
    isAuthenticated: true,
    isLoading: false,
    currentProject: null,
    login: vi.fn(),
    register: vi.fn(),
    refreshUser: vi.fn(),
    selectProject: selectProjectMock,
    setCurrentProject: vi.fn(),
    updateProfile: vi.fn(),
    logout: vi.fn(),
  }),
}))

vi.mock('./components/ui/ErrorBoundary', () => ({
  ErrorBoundary: ({ children }: any) => <>{children}</>,
}))

vi.mock('./components/ui', () => {
  const Div = ({ children, ...props }: any) => <div {...props}>{children}</div>
  const Button = ({ children, ...props }: any) => <button {...props}>{children}</button>
  const Input = (props: any) => <input {...props} />
  return {
    LoadingOverlay: () => null,
    Card: Div,
    CardContent: Div,
    CardHeader: Div,
    CardTitle: Div,
    Button,
    Input,
    AnimatedBackground: () => null,
  }
})

vi.mock('./components/builder/AppBuilder', () => ({
  AppBuilder: ({ onNavigateToIDE }: any) => (
    <button
      type="button"
      onClick={() => onNavigateToIDE?.({ target: 'preview', projectId: 16 })}
    >
      Open preview workspace
    </button>
  ),
}))

vi.mock('./components/ide/IDELayout', () => ({
  IDELayout: ({ launchTarget }: any) => (
    <div>Mock IDE {launchTarget}</div>
  ),
}))

vi.mock('./components/admin/AdminDashboard', () => ({
  AdminDashboard: () => <div>Mock Admin</div>,
}))

vi.mock('./pages/Explore', () => ({
  ExplorePage: () => <div>Mock Explore</div>,
}))

vi.mock('./components/import/GitHubImportWizard', () => ({
  GitHubImportWizard: () => null,
}))

vi.mock('./components/settings/APIKeySettings', () => ({
  default: () => <div>API Keys</div>,
}))

vi.mock('./components/ai/ModelSelector', () => ({
  default: () => <div>Model Selector</div>,
}))

vi.mock('./components/spend/SpendDashboard', () => ({
  default: () => <div>Spend Dashboard</div>,
}))

vi.mock('./components/budget/BudgetSettings', () => ({
  default: () => <div>Budget Settings</div>,
}))

vi.mock('./components/billing/BillingSettings', () => ({
  default: () => <div>Billing Settings</div>,
}))

vi.mock('./pages/Landing', () => ({
  LandingPage: () => <div>Mock Landing</div>,
}))

vi.mock('./components/help/HelpCenter', () => ({
  HelpButton: () => null,
  HelpCenter: () => null,
}))

vi.mock('./components/ide/CostTicker', () => ({
  default: () => <div>Cost Ticker</div>,
}))

vi.mock('./components/settings/LegalDocuments', () => ({
  __esModule: true,
  default: () => <div>Legal Documents</div>,
  LegalDocumentLinks: () => null,
}))

import App from './App'

const installLocalStorageMock = () => {
  const store = new Map<string, string>()
  const storage = {
    getItem: vi.fn((key: string) => store.get(String(key)) ?? null),
    setItem: vi.fn((key: string, value: string) => {
      store.set(String(key), String(value))
    }),
    removeItem: vi.fn((key: string) => {
      store.delete(String(key))
    }),
    clear: vi.fn(() => {
      store.clear()
    }),
    key: vi.fn((index: number) => Array.from(store.keys())[index] ?? null),
    get length() {
      return store.size
    },
  }

  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: storage,
  })
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    value: storage,
  })

  return storage
}

describe('App IDE navigation', () => {
  beforeEach(() => {
    selectProjectMock.mockReset()
    installLocalStorageMock()
    window.history.replaceState({}, '', '/')
  })

  it('selects the target project when builder navigation opens the IDE preview workspace', async () => {
    render(<App />)

    await screen.findByRole('button', { name: /open preview workspace/i })
    fireEvent.click(screen.getByRole('button', { name: /open preview workspace/i }))

    await waitFor(() => {
      expect(selectProjectMock).toHaveBeenCalledWith(16)
    })
    await waitFor(() => {
      expect(window.location.pathname).toBe('/project/16')
    })
    expect(await screen.findByText('Mock IDE preview')).toBeTruthy()
  })
})
