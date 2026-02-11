// APEX.BUILD IDE Layout
// Dark Demon Theme - Fully responsive development environment interface
// Optimized with React.lazy for Monaco Editor and XTerminal

import React, { useState, useEffect, useCallback, useRef, lazy, Suspense } from 'react'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import {
  useIsMobile,
  useIsTablet,
  useOrientation,
  useSwipeGesture,
  useViewportHeight,
  useReducedMotion,
  useLowPowerMode,
  useSafeAreaInsets,
} from '@/hooks/useMobile'
import apiService from '@/services/api'
import websocketService from '@/services/websocket'
import { File, AICapability, FileVersion } from '@/types'
import {
  Button,
  Badge,
  Avatar,
  Card,
  Loading,
  LoadingOverlay
} from '@/components/ui'
import { AIAssistant } from '@/components/ai/AIAssistant'
import { FileTree } from '@/components/explorer/FileTree'
import { ProjectDashboard } from '@/components/project/ProjectDashboard'
import { ProjectList } from '@/components/project/ProjectList'
import { MobileNavigation, MobilePanelSwitcher } from '@/components/mobile'
import { CodeComments } from '@/components/ide/CodeComments'
import { VersionHistoryPanel } from '@/components/ide/panels/VersionHistoryPanel'
import { DatabasePanel } from '@/components/ide/panels/DatabasePanel'
import { SplitPaneEditor, SplitPaneEditorRef } from '@/components/ide/SplitPaneEditor'
import { usePaneManager } from '@/hooks/usePaneManager'

// Lazy load heavy components for better initial load performance
// Monaco Editor is ~800KB-1.2MB, XTerminal is ~200KB
const MonacoEditor = lazy(() => import('@/components/editor/MonacoEditor').then(m => ({ default: m.MonacoEditor })))
const DiffViewer = lazy(() => import('@/components/ide/DiffViewer').then(m => ({ default: m.DiffViewer })))
const XTerminal = lazy(() => import('@/components/terminal/XTerminal').then(m => ({ default: m.default })))
const LivePreview = lazy(() => import('@/components/preview/LivePreview').then(m => ({ default: m.default })))
import {
  Menu,
  X,
  Folder,
  FileText,
  Terminal,
  Brain,
  Settings,
  Users,
  Play,
  Save,
  Download,
  Share2,
  Maximize2,
  Minimize2,
  RotateCcw,
  Zap,
  Code,
  Search,
  Monitor,
  GitBranch,
  Bell,
  User,
  LogOut,
  ChevronLeft,
  ChevronRight,
  PanelLeftClose,
  PanelRightClose,
  PanelBottomClose,
  Database,
  Bot,
  MessageSquare,
} from 'lucide-react'

// Loading fallback for lazy-loaded components
const EditorLoadingFallback = () => (
  <div className="flex items-center justify-center h-full bg-gray-900/50">
    <div className="text-center">
      <Loading size="lg" variant="spinner" />
      <p className="mt-3 text-sm text-gray-400">Loading editor...</p>
    </div>
  </div>
)

const TerminalLoadingFallback = () => (
  <div className="flex items-center justify-center h-full bg-black">
    <div className="text-center">
      <Loading size="md" variant="spinner" />
      <p className="mt-2 text-xs text-gray-500">Loading terminal...</p>
    </div>
  </div>
)

export interface IDELayoutProps {
  className?: string
  onNavigateToAgent?: () => void
}

type ViewMode = 'projects' | 'dashboard' | 'editor'
type PanelState = 'collapsed' | 'normal' | 'expanded'
type MobilePanel = 'explorer' | 'editor' | 'terminal' | 'ai' | 'settings'

export const IDELayout: React.FC<IDELayoutProps> = ({ className, onNavigateToAgent }) => {
  // Responsive hooks
  const isMobile = useIsMobile()
  const isTablet = useIsTablet()
  const orientation = useOrientation()
  const viewportHeight = useViewportHeight()
  const reducedMotion = useReducedMotion()
  const lowPower = useLowPowerMode()
  const safeArea = useSafeAreaInsets()

  // Layout state
  const [viewMode, setViewMode] = useState<ViewMode>('projects')
  const [leftPanelState, setLeftPanelState] = useState<PanelState>(isMobile ? 'collapsed' : 'normal')
  const [rightPanelState, setRightPanelState] = useState<PanelState>(isMobile ? 'collapsed' : 'normal')
  const [bottomPanelState, setBottomPanelState] = useState<PanelState>('collapsed')
  const [activeLeftTab, setActiveLeftTab] = useState<'explorer' | 'search' | 'git' | 'history'>('explorer')
  const [activeRightTab, setActiveRightTab] = useState<'ai' | 'comments' | 'collab' | 'database' | 'settings'>('ai')
  const [activeBottomTab, setActiveBottomTab] = useState<'terminal' | 'output' | 'problems'>('terminal')
  const [showPreview, setShowPreview] = useState(false)

  // Mobile-specific state
  const [mobilePanel, setMobilePanel] = useState<MobilePanel>('editor')
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [mobileOverlayPanel, setMobileOverlayPanel] = useState<'left' | 'right' | null>(null)

  // Editor state with pane management
  const {
    layout,
    activePane,
    activePaneId,
    openFile: paneOpenFile,
    closeFile: paneCloseFile,
    setActiveFile: paneSetActiveFile,
    updateFileContent: paneUpdateFileContent,
    markFileSaved: paneMarkFileSaved,
    splitHorizontal,
    splitVertical,
    closePane,
    focusPane,
    canSplit,
    getPaneActiveFile
  } = usePaneManager()

  // Derived state from active pane for compatibility with existing components
  const activeFile = activePane ? activePane.files.find(f => f.file.id === activePane.activeFileId)?.file || null : null
  const openFiles = activePane ? activePane.files.map(f => f.file) : []

  // Diff state
  const [showDiff, setShowDiff] = useState(false)
  const [diffVersion, setDiffVersion] = useState<FileVersion | null>(null)

  // Terminal state
  const [terminalOutput, setTerminalOutput] = useState<string[]>([
    'APEX.BUILD Terminal v1.0.0',
    'Welcome to the future of development',
    ''
  ])
  const [terminalInput, setTerminalInput] = useState('')

  // Component refs
  const splitPaneRef = useRef<SplitPaneEditorRef>(null)
  const terminalRef = useRef<HTMLDivElement>(null)
  const mainContentRef = useRef<HTMLDivElement>(null)

  const {
    user,
    currentProject,
    files,
    isLoading,
    currentTheme: theme,
    setTheme,
    collaborationUsers,
    connect,
    disconnect,
    logout
  } = useStore()

  // Update panel states when responsive breakpoints change
  useEffect(() => {
    if (isMobile) {
      setLeftPanelState('collapsed')
      setRightPanelState('collapsed')
    } else if (isTablet) {
      setLeftPanelState('normal')
      setRightPanelState('collapsed')
    } else {
      setLeftPanelState('normal')
      setRightPanelState('normal')
    }
  }, [isMobile, isTablet])

  // Initialize WebSocket connection when project is selected
  useEffect(() => {
    if (currentProject && user) {
      connect(currentProject.id)
      return () => {
        disconnect()
      }
    }
  }, [currentProject, user, connect, disconnect])

  // Swipe gesture for mobile panel switching
  useSwipeGesture(mainContentRef, {
    onSwipeLeft: () => {
      if (isMobile && mobileOverlayPanel === 'left') {
        setMobileOverlayPanel(null)
      } else if (isMobile && mobilePanel === 'explorer') {
        setMobilePanel('editor')
      } else if (isMobile && mobilePanel === 'editor') {
        setMobilePanel('ai')
      }
    },
    onSwipeRight: () => {
      if (isMobile && mobilePanel === 'ai') {
        setMobilePanel('editor')
      } else if (isMobile && mobilePanel === 'editor') {
        setMobilePanel('explorer')
      }
    },
  })

  // Handle file selection
  const handleFileSelect = useCallback((file: File) => {
    paneOpenFile(file)

    if (viewMode !== 'editor') {
      setViewMode('editor')
    }

    // On mobile, switch to editor panel
    if (isMobile) {
      setMobilePanel('editor')
      setMobileOverlayPanel(null)
    }
  }, [paneOpenFile, viewMode, isMobile])

  // Handle file content change
  const handleFileChange = useCallback((fileId: number, content: string, paneId: string) => {
    paneUpdateFileContent(fileId, content, paneId)

    // Send real-time collaboration updates
    if (websocketService.isConnected() && currentProject) {
      websocketService.sendFileChange(
        fileId,
        content,
        1, // Default position
        1
      )
    }
  }, [paneUpdateFileContent, currentProject])

  // Handle file save
  const handleFileSave = useCallback(async (fileId: number, content: string, paneId: string) => {
    try {
      await apiService.updateFile(fileId, { content })
      paneMarkFileSaved(fileId, paneId)

      // Trigger preview hot reload when preview is active
      if (showPreview && currentProject) {
        const file = files.find(f => f.id === fileId)
        if (file) {
          try {
            await apiService.post('/preview/hot-reload', {
              project_id: currentProject.id,
              file_path: file.path,
              content: content
            })
          } catch {
            // Preview might not be running - ignore silently
          }
        }
      }
    } catch (error) {
      console.error('Failed to save file:', error)
    }
  }, [paneMarkFileSaved, showPreview, currentProject, files])

  // Handle AI request
  const handleAIRequest = useCallback(async (capability: AICapability, prompt: string, code: string) => {
    try {
      const response = await apiService.generateAI({
        capability,
        prompt,
        code,
        language: activeFile?.name.split('.').pop() || 'javascript',
        context: {
          project_id: currentProject?.id,
          file_id: activeFile?.id,
        }
      })
      return response
    } catch (error) {
      console.error('AI request failed:', error)
      throw error
    }
  }, [activeFile, currentProject])

  // Close file tab
  const closeFile = useCallback((fileId: number, paneId?: string) => {
    paneCloseFile(fileId, paneId)
  }, [paneCloseFile])

  // Handle project creation
  const handleProjectCreate = useCallback((project: any) => {
    setViewMode('dashboard')
  }, [])

  // Handle project selection
  const handleProjectSelect = useCallback((project: any) => {
    setViewMode('dashboard')
  }, [])

  const handleDashboardShare = useCallback(() => {
    if (!currentProject) return
    const url = `${window.location.origin}/project/${currentProject.id}`
    navigator.clipboard.writeText(url).then(() => {
      setTerminalOutput(prev => [...prev, `Project URL copied to clipboard: ${url}`])
    }).catch(() => {
      setTerminalOutput(prev => [...prev, `Project URL: ${url}`])
    })
  }, [currentProject])

  const handleDashboardDownload = useCallback(async () => {
    if (!currentProject) return
    try {
      await apiService.exportProject(currentProject.id, currentProject.name)
      setTerminalOutput(prev => [...prev, `Download started: ${currentProject.name}.zip`])
    } catch (error) {
      console.error('Export failed:', error)
      setTerminalOutput(prev => [...prev, 'Download failed. Please try again.'])
    }
  }, [currentProject])

  const handleDashboardRun = useCallback(() => {
    if (!currentProject) return
    setViewMode('editor')
    setShowPreview(true)
  }, [currentProject])

  const handleDashboardSettings = useCallback(() => {
    setRightPanelState('normal')
    setActiveRightTab('settings')
  }, [])

  // Handle file creation
  const handleFileCreate = useCallback(async (parentPath: string, name: string, type: 'file' | 'directory') => {
    if (!currentProject) return

    try {
      const fullPath = parentPath === '/' ? `/${name}` : `${parentPath}/${name}`
      const newFile = await apiService.createFile(currentProject.id, {
        name: name,
        path: fullPath,
        type: type,
        content: type === 'file' ? '' : undefined
      })

      setTerminalOutput(prev => [...prev, `Created ${type}: ${fullPath}`])

      if (type === 'file') {
        handleFileSelect(newFile)
      }
    } catch (error) {
      console.error('Failed to create file:', error)
      setTerminalOutput(prev => [...prev, `Failed to create ${type}: ${error}`])
    }
  }, [currentProject, handleFileSelect])

  // Handle file deletion
  const handleFileDelete = useCallback(async (file: File) => {
    try {
      await apiService.deleteFile(file.id)

      paneCloseFile(file.id)

      setTerminalOutput(prev => [...prev, `Deleted: ${file.path}`])
    } catch (error) {
      console.error('Failed to delete file:', error)
      setTerminalOutput(prev => [...prev, `Failed to delete: ${error}`])
    }
  }, [paneCloseFile])

  // Handle file rename
  const handleFileRename = useCallback(async (file: File, newName: string) => {
    try {
      const pathParts = file.path.split('/')
      pathParts[pathParts.length - 1] = newName
      const newPath = pathParts.join('/')

      await apiService.updateFile(file.id, {
        name: newName,
        path: newPath
      })

      // Update in all panes
      layout.panes.forEach(pane => {
        if (pane.files.find(f => f.file.id === file.id)) {
          // This is a bit complex as paneManager doesn't have rename, 
          // but we can just reopen the file with new data
          paneOpenFile({ ...file, name: newName, path: newPath }, pane.id)
        }
      })

      setTerminalOutput(prev => [...prev, `Renamed: ${file.name} -> ${newName}`])
    } catch (error) {
      console.error('Failed to rename file:', error)
      setTerminalOutput(prev => [...prev, `Failed to rename: ${error}`])
    }
  }, [layout.panes, paneOpenFile])

  // Handle version preview (diff)
  const handlePreviewVersion = useCallback((version: FileVersion) => {
    setDiffVersion(version)
    setShowDiff(true)
  }, [])

  // Handle version restore
  const handleRestoreVersion = useCallback(async (version: FileVersion) => {
    if (!activeFile) return
    
    try {
      const restoredFile = await apiService.restoreFileVersion(version.id)
      
      // Update file content in all panes that have this file open
      layout.panes.forEach(pane => {
        if (pane.files.find(f => f.file.id === restoredFile.id)) {
          paneUpdateFileContent(restoredFile.id, restoredFile.content, pane.id)
          paneMarkFileSaved(restoredFile.id, pane.id)
        }
      })
      
      setTerminalOutput(prev => [...prev, `Restored version ${version.version}`])
      setShowDiff(false)
      setDiffVersion(null)
    } catch (error) {
      console.error('Failed to restore version:', error)
      setTerminalOutput(prev => [...prev, `Failed to restore version: ${error}`])
    }
  }, [activeFile, layout.panes, paneUpdateFileContent, paneMarkFileSaved])

  // Terminal command execution
  const executeTerminalCommand = useCallback(async (command: string) => {
    setTerminalOutput(prev => [...prev, `$ ${command}`])
    setTerminalInput('')

    try {
      if (command.startsWith('run') || command === 'start') {
        if (currentProject) {
          setTerminalOutput(prev => [...prev, 'Executing project...'])
          const execution = await apiService.executeCode({
            project_id: currentProject.id,
            command: 'npm start',
            language: currentProject.language,
          })
          setTerminalOutput(prev => [...prev, execution.output])
        }
      } else if (command === 'clear') {
        setTerminalOutput(['APEX.BUILD Terminal v1.0.0', 'Welcome to the future of development', ''])
      } else {
        setTerminalOutput(prev => [...prev, `Command not found: ${command}`])
      }
    } catch (error) {
      setTerminalOutput(prev => [...prev, `Error: ${error}`])
    }
  }, [currentProject])

  // Handle mobile tab change
  const handleMobileTabChange = useCallback((tab: MobilePanel) => {
    if (tab === 'settings') {
      setMobileMenuOpen(true)
    } else {
      setMobilePanel(tab)
    }
  }, [])

  // Render left panel content
  const renderLeftPanel = () => {
    switch (activeLeftTab) {
      case 'explorer':
        return (
          <FileTree
            projectId={currentProject?.id}
            onFileSelect={handleFileSelect}
            onFileCreate={handleFileCreate}
            onFileDelete={handleFileDelete}
            onFileRename={handleFileRename}
            className="h-full border-0"
          />
        )
      case 'search':
        return (
          <Card variant="cyberpunk" padding="md" className="h-full border-0">
            <div className="text-center text-gray-400">
              <Search className="w-8 h-8 mx-auto mb-2" />
              <p className="text-sm">Global search coming soon</p>
            </div>
          </Card>
        )
      case 'git':
        return (
          <Card variant="cyberpunk" padding="md" className="h-full border-0">
            <div className="text-center text-gray-400">
              <GitBranch className="w-8 h-8 mx-auto mb-2" />
              <p className="text-sm">Git integration coming soon</p>
            </div>
          </Card>
        )
      case 'history':
        return currentProject ? (
          <VersionHistoryPanel
            file={activeFile}
            projectId={currentProject.id}
            onPreviewVersion={handlePreviewVersion}
            onRestoreVersion={handleRestoreVersion}
            className="h-full border-0"
          />
        ) : null
      default:
        return null
    }
  }

  // Render right panel content
  const renderRightPanel = () => {
    switch (activeRightTab) {
      case 'ai':
        return (
          <AIAssistant
            projectId={currentProject?.id}
            fileId={activeFile?.id}
            onCodeInsert={(code) => {
              splitPaneRef.current?.insertCode(code)
            }}
            className="h-full border-0"
          />
        )
      case 'comments':
        return activeFile && currentProject && user ? (
          <CodeComments
            file={activeFile}
            projectId={currentProject.id}
            currentUserId={user.id}
            currentUsername={user.username}
            onCommentClick={(line) => {
              splitPaneRef.current?.revealLine(line)
            }}
            className="h-full border-0"
          />
        ) : (
          <Card variant="cyberpunk" padding="md" className="h-full border-0">
            <div className="text-center text-gray-400">
              <MessageSquare className="w-8 h-8 mx-auto mb-2" />
              <p className="text-sm">Select a file to view comments</p>
            </div>
          </Card>
        )
      case 'collab':
        return (
          <Card variant="synthwave" padding="md" className="h-full border-0">
            <h3 className="text-lg font-semibold text-white mb-4">Collaboration</h3>
            {collaborationUsers.length > 0 ? (
              <div className="space-y-3">
                {collaborationUsers.map(user => (
                  <div key={user.id} className="flex items-center gap-3">
                    <Avatar
                      src={user.avatar_url}
                      fallback={user.username}
                      status="online"
                      showStatus
                      size="sm"
                    />
                    <div>
                      <div className="text-sm font-medium text-white">{user.username}</div>
                      <div className="text-xs text-gray-400">Active now</div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center text-gray-400">
                <Users className="w-8 h-8 mx-auto mb-2" />
                <p className="text-sm">No collaborators online</p>
              </div>
            )}
          </Card>
        )
      case 'database':
        return currentProject ? (
          <DatabasePanel
            projectId={currentProject.id}
            className="h-full border-0"
          />
        ) : null
      case 'settings':
        return (
          <Card variant="cyberpunk" padding="md" className="h-full border-0">
            <h3 className="text-lg font-semibold text-white mb-4">Settings</h3>
            <div className="space-y-4">
              <div>
                <label className="text-sm font-medium text-gray-300">Theme</label>
                <select
                  value={theme.id}
                  onChange={(e) => setTheme(e.target.value)}
                  className="w-full mt-1 bg-gray-800 border border-gray-600 rounded px-3 py-2 text-white focus:border-red-500 focus:outline-none touch-target"
                >
                  <option value="cyberpunk">Cyberpunk</option>
                  <option value="matrix">Matrix</option>
                  <option value="synthwave">Synthwave</option>
                  <option value="neonCity">Neon City</option>
                </select>
              </div>
            </div>
          </Card>
        )
      default:
        return null
    }
  }

  // Render bottom panel content
  const renderBottomPanel = () => {
    switch (activeBottomTab) {
      case 'terminal':
        return (
          <div className="h-full bg-black">
            <Suspense fallback={<TerminalLoadingFallback />}>
              <XTerminal
                projectId={currentProject?.id}
                theme={theme.id}
                onTitleChange={(title) => {
                  // Optional: update tab title
                }}
              />
            </Suspense>
          </div>
        )
      case 'output':
        return (
          <Card variant="cyberpunk" padding="md" className="h-full border-0">
            <div className="text-center text-gray-400">
              <FileText className="w-8 h-8 mx-auto mb-2" />
              <p className="text-sm">Output panel</p>
            </div>
          </Card>
        )
      case 'problems':
        return (
          <Card variant="cyberpunk" padding="md" className="h-full border-0">
            <div className="text-center text-gray-400">
              <Zap className="w-8 h-8 mx-auto mb-2" />
              <p className="text-sm">Problems panel</p>
            </div>
          </Card>
        )
      default:
        return null
    }
  }

  // Main content based on view mode
  const renderMainContent = () => {
    switch (viewMode) {
      case 'projects':
        return (
          <ProjectList
            onProjectSelect={handleProjectSelect}
            onProjectCreate={handleProjectCreate}
            className="p-4 md:p-6"
          />
        )
      case 'dashboard':
        return (
          <ProjectDashboard
            projectId={currentProject?.id}
            className="h-full"
            onShare={handleDashboardShare}
            onSettings={handleDashboardSettings}
            onRunProject={handleDashboardRun}
            onDownload={handleDashboardDownload}
          />
        )
      case 'editor':
        return (
          <div className="h-full flex">
            {/* Editor Area */}
            <div className={cn(
              'flex-1 min-h-0 min-w-0',
              showPreview && currentProject && 'w-1/2'
            )}>
              {showDiff && diffVersion && activeFile ? (
                <Suspense fallback={<EditorLoadingFallback />}>
                  <DiffViewer
                    originalContent={diffVersion.content}
                    modifiedContent={activePane?.files.find(f => f.file.id === activeFile.id)?.content || activeFile.content}
                    originalLabel={`v${diffVersion.version}`}
                    modifiedLabel="Working Copy"
                    language={activeFile.name.split('.').pop() || 'plaintext'}
                    onClose={() => {
                      setShowDiff(false)
                      setDiffVersion(null)
                    }}
                  />
                </Suspense>
              ) : (
                <SplitPaneEditor
                  ref={splitPaneRef}
                  layout={layout}
                  activePaneId={activePaneId}
                  canSplit={canSplit}
                  onFocusPane={focusPane}
                  onClosePane={closePane}
                  onFileSelect={paneSetActiveFile}
                  onFileClose={closeFile}
                  onFileChange={handleFileChange}
                  onFileSave={handleFileSave}
                  onAIRequest={handleAIRequest}
                  onSplitHorizontal={splitHorizontal}
                  onSplitVertical={splitVertical}
                />
              )}
            </div>

            {/* Live Preview Pane */}
            {showPreview && currentProject && (
              <div className="w-1/2 border-l border-gray-800 min-w-0">
                <Suspense fallback={<EditorLoadingFallback />}>
                  <LivePreview
                    projectId={currentProject.id}
                    autoStart={true}
                    className="h-full"
                  />
                </Suspense>
              </div>
            )}
          </div>
        )
      default:
        return null
    }
  }

  // Mobile layout
  if (isMobile) {
    return (
      <div
        className={cn('h-full flex flex-col bg-gray-950 min-h-0', className)}
        style={{
          height: '100%',
          paddingTop: safeArea.top,
        }}
      >
        {/* Loading overlay */}
        <LoadingOverlay isVisible={isLoading} text="Loading APEX.BUILD..." />

        {/* Mobile Navigation */}
        <MobileNavigation
          activeTab={mobilePanel}
          onTabChange={handleMobileTabChange}
          onMenuToggle={setMobileMenuOpen}
          user={user || undefined}
          onLogout={logout}
        />

        {/* Main content area with safe area padding */}
        <div
          ref={mainContentRef}
          className="flex-1 overflow-hidden"
          style={{
            paddingTop: '56px', // Header height
            paddingBottom: '64px', // Tab bar height + safe area
          }}
        >
          {/* Mobile Panel Content */}
          {mobilePanel === 'explorer' && currentProject && (
            <div className="h-full overflow-auto">
              {renderLeftPanel()}
            </div>
          )}

          {mobilePanel === 'editor' && (
            <div className="h-full">
              {viewMode === 'projects' ? (
                renderMainContent()
              ) : viewMode === 'dashboard' ? (
                renderMainContent()
              ) : (
                renderMainContent()
              )}
            </div>
          )}

          {mobilePanel === 'terminal' && (
            <div className="h-full p-2">
              {renderBottomPanel()}
            </div>
          )}

          {mobilePanel === 'ai' && currentProject && (
            <div className="h-full overflow-auto">
              {renderRightPanel()}
            </div>
          )}
        </div>
      </div>
    )
  }

  // Desktop/Tablet layout
  return (
    <div className={cn('h-full flex flex-col bg-gray-950 min-h-0', className)}>
      {/* Loading overlay */}
      <LoadingOverlay isVisible={isLoading} text="Loading APEX.BUILD..." />

      {/* Top bar */}
      <div className="h-12 bg-gray-900/95 backdrop-blur-md border-b border-gray-800 flex items-center justify-between px-4">
        {/* Left side */}
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 bg-gradient-to-br from-red-500 to-red-900 rounded" />
            <span className="text-lg font-bold text-white hidden sm:inline">APEX.BUILD</span>
          </div>

          {/* Navigation */}
          <div className="flex items-center gap-1">
            <Button
              size="sm"
              variant={viewMode === 'projects' ? 'primary' : 'ghost'}
              onClick={() => setViewMode('projects')}
              icon={<Folder size={14} />}
              className="touch-target"
            >
              <span className="hidden sm:inline">Projects</span>
            </Button>
            {currentProject && (
              <>
                <Button
                  size="sm"
                  variant={viewMode === 'dashboard' ? 'primary' : 'ghost'}
                  onClick={() => setViewMode('dashboard')}
                  icon={<FileText size={14} />}
                  className="touch-target"
                >
                  <span className="hidden sm:inline">Dashboard</span>
                </Button>
                <Button
                  size="sm"
                  variant={viewMode === 'editor' ? 'primary' : 'ghost'}
                  onClick={() => setViewMode('editor')}
                  icon={<Code size={14} />}
                  className="touch-target"
                >
                  <span className="hidden sm:inline">Editor</span>
                </Button>
                {viewMode === 'editor' && (
                  <Button
                    size="sm"
                    variant={showPreview ? 'primary' : 'ghost'}
                    onClick={() => setShowPreview(!showPreview)}
                    icon={<Monitor size={14} />}
                    className="touch-target"
                  >
                    <span className="hidden sm:inline">Preview</span>
                  </Button>
                )}
              </>
            )}
          </div>
        </div>

        {/* Right side */}
        <div className="flex items-center gap-2">
          {/* Project info */}
          {currentProject && (
            <div className="hidden md:flex items-center gap-2 px-3 py-1 bg-gray-800 rounded">
              <span className="text-sm text-gray-300 truncate max-w-[150px]">{currentProject.name}</span>
              <Badge variant="outline" size="xs">
                {currentProject.language}
              </Badge>
            </div>
          )}

          {/* Project action buttons */}
          {currentProject && (
            <div className="flex items-center gap-1">
              <Button
                size="sm"
                variant="ghost"
                icon={<Download size={14} />}
                className="touch-target"
                title="Download ZIP"
                onClick={async () => {
                  try {
                    await apiService.exportProject(currentProject.id, currentProject.name)
                  } catch (err) {
                    console.error('Export failed:', err)
                  }
                }}
              >
                <span className="hidden lg:inline">ZIP</span>
              </Button>
              <Button
                size="sm"
                variant={showPreview ? 'primary' : 'ghost'}
                icon={<Play size={14} />}
                className="touch-target"
                title={showPreview ? 'Stop Preview' : 'Run Project'}
                onClick={() => {
                  if (!showPreview) {
                    setViewMode('editor')
                  }
                  setShowPreview(!showPreview)
                }}
              >
                <span className="hidden lg:inline">{showPreview ? 'Stop' : 'Run'}</span>
              </Button>
              <Button
                size="sm"
                variant="ghost"
                icon={<Share2 size={14} />}
                className="touch-target"
                title="Copy project URL"
                onClick={() => {
                  const url = `${window.location.origin}/project/${currentProject.id}`
                  navigator.clipboard.writeText(url).then(() => {
                    setTerminalOutput(prev => [...prev, `Project URL copied to clipboard: ${url}`])
                  }).catch(() => {
                    setTerminalOutput(prev => [...prev, `Project URL: ${url}`])
                  })
                }}
              >
                <span className="hidden lg:inline">Share</span>
              </Button>
            </div>
          )}

          {/* User menu */}
          {user && (
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                variant="ghost"
                icon={<Bell size={14} />}
                className="touch-target"
              />
              <Avatar
                src={user.avatar_url}
                fallback={user.username}
                size="sm"
                status="online"
                showStatus
              />
            </div>
          )}
        </div>
      </div>

      {/* Main content area */}
      <div ref={mainContentRef} className="flex-1 flex overflow-hidden">
        {/* Left sidebar */}
        {(viewMode === 'dashboard' || viewMode === 'editor') && currentProject && (
          <div className={cn(
            'bg-gray-900/80 border-r border-gray-800 flex flex-col',
            reducedMotion ? '' : 'transition-all duration-300',
            leftPanelState === 'collapsed' && 'w-12',
            leftPanelState === 'normal' && 'w-64 lg:w-80',
            leftPanelState === 'expanded' && 'w-80 lg:w-96'
          )}>
            {/* Sidebar tabs */}
            <div className="h-10 bg-gray-800/50 border-b border-gray-700/50 flex items-center justify-between">
              <div className="flex">
                <Button
                  size="xs"
                  variant={activeLeftTab === 'explorer' ? 'primary' : 'ghost'}
                  onClick={() => setActiveLeftTab('explorer')}
                  icon={<Folder size={14} />}
                  className="rounded-none border-0 touch-target"
                >
                  {leftPanelState !== 'collapsed' && 'Explorer'}
                </Button>
                <Button
                  size="xs"
                  variant={activeLeftTab === 'search' ? 'primary' : 'ghost'}
                  onClick={() => setActiveLeftTab('search')}
                  icon={<Search size={14} />}
                  className="rounded-none border-0 touch-target"
                >
                  {leftPanelState !== 'collapsed' && 'Search'}
                </Button>
                <Button
                  size="xs"
                  variant={activeLeftTab === 'git' ? 'primary' : 'ghost'}
                  onClick={() => setActiveLeftTab('git')}
                  icon={<GitBranch size={14} />}
                  className="rounded-none border-0 touch-target"
                >
                  {leftPanelState !== 'collapsed' && 'Git'}
                </Button>
                <Button
                  size="xs"
                  variant={activeLeftTab === 'history' ? 'primary' : 'ghost'}
                  onClick={() => setActiveLeftTab('history')}
                  icon={<RotateCcw size={14} />}
                  className="rounded-none border-0 touch-target"
                >
                  {leftPanelState !== 'collapsed' && 'History'}
                </Button>
              </div>
              <Button
                size="xs"
                variant="ghost"
                onClick={() => setLeftPanelState(leftPanelState === 'collapsed' ? 'normal' : 'collapsed')}
                icon={<PanelLeftClose size={14} />}
                className="mr-1 touch-target"
              />
            </div>

            {/* Sidebar content */}
            <div className="flex-1 overflow-hidden">
              {leftPanelState !== 'collapsed' && renderLeftPanel()}
            </div>
          </div>
        )}

        {/* Main editor area */}
        <div className="flex-1 flex flex-col overflow-hidden min-w-0">
          <div className="flex-1 overflow-hidden">
            {renderMainContent()}
          </div>

          {/* Bottom panel */}
          {bottomPanelState !== 'collapsed' && (
            <div className={cn(
              'bg-gray-900/80 border-t border-gray-800',
              reducedMotion ? '' : 'transition-all duration-300',
              bottomPanelState === 'normal' && 'h-48 md:h-60',
              bottomPanelState === 'expanded' && 'h-72 md:h-96'
            )}>
              {/* Bottom tabs */}
              <div className="h-10 bg-gray-800/50 border-b border-gray-700/50 flex items-center justify-between">
                <div className="flex">
                  <Button
                    size="xs"
                    variant={activeBottomTab === 'terminal' ? 'primary' : 'ghost'}
                    onClick={() => setActiveBottomTab('terminal')}
                    icon={<Terminal size={14} />}
                    className="rounded-none border-0 touch-target"
                  >
                    Terminal
                  </Button>
                  <Button
                    size="xs"
                    variant={activeBottomTab === 'output' ? 'primary' : 'ghost'}
                    onClick={() => setActiveBottomTab('output')}
                    icon={<FileText size={14} />}
                    className="rounded-none border-0 touch-target"
                  >
                    Output
                  </Button>
                  <Button
                    size="xs"
                    variant={activeBottomTab === 'problems' ? 'primary' : 'ghost'}
                    onClick={() => setActiveBottomTab('problems')}
                    icon={<Zap size={14} />}
                    className="rounded-none border-0 touch-target"
                  >
                    Problems
                  </Button>
                </div>

                <Button
                  size="xs"
                  variant="ghost"
                  onClick={() => setBottomPanelState('collapsed')}
                  icon={<X size={14} />}
                  className="touch-target"
                />
              </div>

              {/* Bottom content */}
              <div className="flex-1 overflow-hidden">
                {renderBottomPanel()}
              </div>
            </div>
          )}
        </div>

        {/* Right sidebar */}
        {(viewMode === 'dashboard' || viewMode === 'editor') && currentProject && (
          <div className={cn(
            'bg-gray-900/80 border-l border-gray-800 flex flex-col',
            reducedMotion ? '' : 'transition-all duration-300',
            rightPanelState === 'collapsed' && 'w-12',
            rightPanelState === 'normal' && 'w-64 lg:w-80',
            rightPanelState === 'expanded' && 'w-80 lg:w-96'
          )}>
            {/* Sidebar tabs */}
            <div className="h-10 bg-gray-800/50 border-b border-gray-700/50 flex items-center justify-between">
              <Button
                size="xs"
                variant="ghost"
                onClick={() => setRightPanelState(rightPanelState === 'collapsed' ? 'normal' : 'collapsed')}
                icon={<PanelRightClose size={14} />}
                className="ml-1 touch-target"
              />
              <div className="flex">
                <Button
                  size="xs"
                  variant={activeRightTab === 'ai' ? 'primary' : 'ghost'}
                  onClick={() => setActiveRightTab('ai')}
                  icon={<Brain size={14} />}
                  className="rounded-none border-0 touch-target"
                >
                  {rightPanelState !== 'collapsed' && 'AI'}
                </Button>
                <Button
                  size="xs"
                  variant={activeRightTab === 'comments' ? 'primary' : 'ghost'}
                  onClick={() => setActiveRightTab('comments')}
                  icon={<MessageSquare size={14} />}
                  className="rounded-none border-0 touch-target"
                >
                  {rightPanelState !== 'collapsed' && 'Comments'}
                </Button>
                <Button
                  size="xs"
                  variant={activeRightTab === 'collab' ? 'primary' : 'ghost'}
                  onClick={() => setActiveRightTab('collab')}
                  icon={<Users size={14} />}
                  className="rounded-none border-0 touch-target"
                >
                  {rightPanelState !== 'collapsed' && 'Collab'}
                </Button>
                <Button
                  size="xs"
                  variant={activeRightTab === 'database' ? 'primary' : 'ghost'}
                  onClick={() => setActiveRightTab('database')}
                  icon={<Database size={14} />}
                  className="rounded-none border-0 touch-target"
                >
                  {rightPanelState !== 'collapsed' && 'Database'}
                </Button>
                <Button
                  size="xs"
                  variant={activeRightTab === 'settings' ? 'primary' : 'ghost'}
                  onClick={() => setActiveRightTab('settings')}
                  icon={<Settings size={14} />}
                  className="rounded-none border-0 touch-target"
                >
                  {rightPanelState !== 'collapsed' && 'Settings'}
                </Button>
              </div>
            </div>

            {/* Sidebar content */}
            <div className="flex-1 overflow-hidden">
              {rightPanelState !== 'collapsed' && renderRightPanel()}
            </div>
          </div>
        )}
      </div>

      {/* Footer for panels toggle */}
      <div className="h-6 bg-gray-900/95 border-t border-gray-800 flex items-center justify-center">
        <div className="flex items-center gap-1">
          <Button
            size="xs"
            variant="ghost"
            onClick={() => setBottomPanelState(bottomPanelState === 'collapsed' ? 'normal' : 'collapsed')}
            icon={<Terminal size={12} />}
            title="Toggle Terminal"
            className="touch-target"
          />
        </div>
      </div>

      {/* Floating AI Agent Button */}
      {onNavigateToAgent && (
        <button
          onClick={onNavigateToAgent}
          className="fixed bottom-20 right-6 w-14 h-14 bg-gradient-to-br from-cyan-500 to-purple-600 rounded-full shadow-lg shadow-cyan-500/30 hover:shadow-cyan-500/50 flex items-center justify-center transition-all hover:scale-110 active:scale-95 z-50 group"
          title="Open AI Agent"
        >
          <Bot className="w-6 h-6 text-white" />
          <span className="absolute -top-1 -right-1 w-4 h-4 bg-green-400 rounded-full border-2 border-gray-950 animate-pulse" />
          <span className="absolute inset-0 rounded-full bg-cyan-400/20 animate-ping" />
        </button>
      )}
    </div>
  )
}

export default IDELayout
