/* @vitest-environment jsdom */

import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const connectMock = vi.fn()
const disconnectMock = vi.fn()

vi.mock('@/hooks/useStore', () => ({
  useStore: () => ({
    user: { id: 7, username: 'tester' },
    currentProject: { id: 22, name: 'Preview Canary', language: 'TypeScript' },
    setCurrentProject: vi.fn(),
    files: [],
    isLoading: false,
    currentTheme: { id: 'cyberpunk' },
    setTheme: vi.fn(),
    createFile: vi.fn(),
    deleteFile: vi.fn(),
    fetchFiles: vi.fn(),
    hydrateFile: vi.fn(),
    collaborationUsers: [],
    connect: connectMock,
    disconnect: disconnectMock,
    logout: vi.fn(),
  }),
}))

vi.mock('@/hooks/useMobile', () => ({
  useIsMobile: () => false,
  useIsTablet: () => false,
  useOrientation: () => 'landscape',
  useSwipeGesture: () => undefined,
  useViewportHeight: () => 900,
  useReducedMotion: () => false,
  useLowPowerMode: () => false,
  useSafeAreaInsets: () => ({ top: 0, right: 0, bottom: 0, left: 0 }),
}))

vi.mock('@/hooks/usePaneManager', () => ({
  usePaneManager: () => ({
    layout: { panes: [] },
    activePane: null,
    activePaneId: null,
    openFile: vi.fn(),
    closeFile: vi.fn(),
    setActiveFile: vi.fn(),
    updateFileContent: vi.fn(),
    markFileSaved: vi.fn(),
    splitHorizontal: vi.fn(),
    splitVertical: vi.fn(),
    closePane: vi.fn(),
    focusPane: vi.fn(),
    canSplit: false,
    getPaneActiveFile: vi.fn(),
  }),
}))

vi.mock('@/services/api', () => ({
  default: {
    getFile: vi.fn(),
    updateFile: vi.fn(),
    post: vi.fn(),
    generateAI: vi.fn(),
    exportProject: vi.fn(),
    executeProject: vi.fn(),
    restoreFileVersion: vi.fn(),
  },
}))

vi.mock('@/components/ui', () => {
  const Div = ({ children, ...props }: any) => <div {...props}>{children}</div>
  const Button = ({ children, icon, ...props }: any) => (
    <button {...props}>
      {icon}
      {children}
    </button>
  )
  const Badge = ({ children, ...props }: any) => <div {...props}>{children}</div>
  const Avatar = (props: any) => <div {...props}>Avatar</div>
  return {
    Button,
    Badge,
    Avatar,
    Card: Div,
    Loading: () => <div>Loading</div>,
    LoadingOverlay: () => null,
  }
})

vi.mock('@/components/ai/AIAssistant', () => ({
  AIAssistant: () => <div>AI Assistant</div>,
}))

vi.mock('@/components/explorer/FileTree', () => ({
  FileTree: () => <div>File Tree</div>,
}))

vi.mock('@/components/project/ProjectDashboard', () => ({
  ProjectDashboard: () => <div>Project Dashboard</div>,
}))

vi.mock('@/components/project/ProjectList', () => ({
  ProjectList: () => <div>Project List</div>,
}))

vi.mock('@/components/mobile', () => ({
  MobileNavigation: () => <div>Mobile Navigation</div>,
  MobilePanelSwitcher: () => <div>Mobile Panel Switcher</div>,
}))

vi.mock('@/components/ide/CodeComments', () => ({
  CodeComments: () => <div>Code Comments</div>,
}))

vi.mock('@/components/ide/panels/VersionHistoryPanel', () => ({
  VersionHistoryPanel: () => <div>Version History</div>,
}))

vi.mock('@/components/ide/panels/DatabasePanel', () => ({
  DatabasePanel: () => <div>Database Panel</div>,
}))

vi.mock('@/components/deployment', () => ({
  DeploymentPanel: () => <div>Deployment Panel</div>,
}))

vi.mock('@/components/ide/SearchPanel', () => ({
  SearchPanel: () => <div>Search Panel</div>,
}))

vi.mock('@/components/ide/GitPanel', () => ({
  GitPanel: () => <div>Git Panel</div>,
}))

vi.mock('@/components/ide/SplitPaneEditor', () => ({
  SplitPaneEditor: React.forwardRef((_props: any, _ref) => <div>Split Pane Editor</div>),
}))

vi.mock('@/components/editor/MonacoEditor', () => ({
  MonacoEditor: () => <div>Monaco Editor</div>,
}))

vi.mock('@/components/ide/DiffViewer', () => ({
  DiffViewer: () => <div>Diff Viewer</div>,
}))

vi.mock('@/components/terminal/XTerminal', () => ({
  __esModule: true,
  default: () => <div>Mock Terminal</div>,
}))

vi.mock('@/components/preview/LivePreview', () => ({
  __esModule: true,
  default: ({ projectId }: { projectId: number }) => <div>Live Preview {projectId}</div>,
}))

import { IDELayout } from './IDELayout'

describe('IDELayout preview workspace', () => {
  beforeEach(() => {
    connectMock.mockReset()
    disconnectMock.mockReset()
  })

  it('keeps the preview workspace focused without mounting the terminal panel', async () => {
    render(<IDELayout launchTarget="preview" />)

    expect(await screen.findByText('Preview Workspace')).toBeTruthy()
    expect(await screen.findByText('Live Preview 22')).toBeTruthy()

    await waitFor(() => {
      expect(connectMock).toHaveBeenCalledWith(22)
    })

    expect(screen.queryByText('Mock Terminal')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Terminal' })).toBeNull()
  })

  it('renders the deployment panel from the right sidebar', async () => {
    render(<IDELayout launchTarget="editor" />)

    const deployTab = await screen.findByRole('button', { name: /deploy/i })
    fireEvent.click(deployTab)

    expect(await screen.findByText('Deployment Panel')).toBeTruthy()
  })
})
