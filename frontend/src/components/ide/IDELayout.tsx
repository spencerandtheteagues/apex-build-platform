// APEX-BUILD IDE Layout
// Dark Demon Theme - Fully responsive development environment interface
// Optimized with React.lazy for Monaco Editor and XTerminal

import React, { useState, useEffect, useCallback, useRef, useMemo, lazy, Suspense } from 'react'
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
import { DeploymentPanel } from '@/components/deployment'
import { SearchPanel } from '@/components/ide/SearchPanel'
import { GitPanel } from '@/components/ide/GitPanel'
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
  Rocket,
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
  AlertCircle,
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
  launchTarget?: 'dashboard' | 'editor' | 'preview'
  launchRequestId?: number
}

type ViewMode = 'projects' | 'dashboard' | 'editor' | 'preview'
type PanelState = 'collapsed' | 'normal' | 'expanded'
type MobilePanel = 'explorer' | 'editor' | 'terminal' | 'ai' | 'settings'

const ideChromeButtonClass = 'touch-target !h-9 rounded-xl !border !border-gray-700/80 !bg-gray-900/75 !px-3 !text-gray-200 !shadow-none hover:!border-gray-600 hover:!bg-gray-800 hover:!text-white'
const ideChromeButtonActiveClass = 'touch-target !h-9 rounded-xl !border !border-red-500/50 !bg-red-500/12 !px-3 !text-white !shadow-none hover:!bg-red-500/18'
const ideIconButtonClass = 'touch-target !h-8 !w-8 rounded-lg !border !border-gray-700/70 !bg-gray-900/75 !p-0 !text-gray-300 !shadow-none hover:!border-gray-600 hover:!bg-gray-800 hover:!text-white'

const idePanelTabClass = (active: boolean) => cn(
  'touch-target !h-10 rounded-none !border-0 !px-3 !text-[13px] !font-medium !shadow-none',
  active
    ? '!bg-red-500/15 !text-white'
    : '!bg-transparent !text-gray-400 hover:!bg-gray-800/85 hover:!text-white'
)

export const IDELayout: React.FC<IDELayoutProps> = ({
  className,
  onNavigateToAgent,
  launchTarget = 'dashboard',
  launchRequestId = 0,
}) => {
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
  const [activeRightTab, setActiveRightTab] = useState<'ai' | 'comments' | 'collab' | 'database' | 'deploy' | 'settings'>('ai')
  const [activeBottomTab, setActiveBottomTab] = useState<'terminal' | 'output' | 'problems'>('terminal')
  const [showPreview, setShowPreview] = useState(false)
  const [previewAutoRefresh, setPreviewAutoRefresh] = useState(true)
  const [showNotifications, setShowNotifications] = useState(false)

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
    'APEX-BUILD Terminal v1.0.0',
    'Welcome to the future of development',
    ''
  ])
  const [terminalInput, setTerminalInput] = useState('')
  const problemLines = useMemo(
    () => terminalOutput.filter(line => /error|failed|exception|warn/i.test(line)),
    [terminalOutput]
  )

  // Component refs
  const splitPaneRef = useRef<SplitPaneEditorRef>(null)
  const terminalRef = useRef<HTMLDivElement>(null)
  const mainContentRef = useRef<HTMLDivElement>(null)

  const {
    user,
    currentProject,
    setCurrentProject,
    files,
    isLoading,
    currentTheme: theme,
    setTheme,
    createFile: createFileAction,
    deleteFile: deleteFileAction,
    fetchFiles,
    hydrateFile,
    collaborationUsers,
    connect,
    disconnect,
    logout
  } = useStore()
  const initializedProjectViewRef = useRef(false)
  const lastLaunchTargetRef = useRef<'dashboard' | 'editor' | 'preview' | null>(null)
  const lastLaunchRequestIdRef = useRef<number | null>(null)

  const openProjectsView = useCallback(() => {
    setShowPreview(false)
    setViewMode('projects')
  }, [])

  const openDashboardView = useCallback(() => {
    if (!currentProject) {
      openProjectsView()
      return
    }
    setShowPreview(false)
    setViewMode('dashboard')
  }, [currentProject, openProjectsView])

  const openEditorWorkspace = useCallback((options?: { keepPreview?: boolean }) => {
    setViewMode('editor')
    setShowPreview(options?.keepPreview ?? false)
    if (isMobile) {
      setMobilePanel('editor')
      setMobileOverlayPanel(null)
    }
  }, [isMobile])

  const openPreviewWorkspace = useCallback(() => {
    if (!currentProject) return
    setShowPreview(true)
    setViewMode('preview')
    if (isMobile) {
      setMobilePanel('editor')
      setMobileOverlayPanel(null)
    }
  }, [currentProject, isMobile])

  useEffect(() => {
    if (!currentProject) {
      initializedProjectViewRef.current = false
      lastLaunchTargetRef.current = null
      lastLaunchRequestIdRef.current = null
      openProjectsView()
      return
    }

    const nextLaunchTarget = launchTarget || 'dashboard'
    const launchTargetChanged = lastLaunchTargetRef.current !== nextLaunchTarget
    const launchRequestChanged = lastLaunchRequestIdRef.current !== launchRequestId

    if (!initializedProjectViewRef.current || launchTargetChanged || launchRequestChanged) {
      if (nextLaunchTarget === 'preview') {
        openPreviewWorkspace()
      } else if (nextLaunchTarget === 'editor') {
        openEditorWorkspace()
      } else {
        openDashboardView()
      }
      initializedProjectViewRef.current = true
      lastLaunchTargetRef.current = nextLaunchTarget
      lastLaunchRequestIdRef.current = launchRequestId
    }
  }, [currentProject, launchRequestId, launchTarget, openDashboardView, openEditorWorkspace, openPreviewWorkspace, openProjectsView])

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
  const handleFileSelect = useCallback(async (file: File) => {
    let selectedFile = file
    const hasContent = typeof (file as any).content === 'string'
    if (!hasContent) {
      try {
        const fullFile = await apiService.getFile(file.id)
        hydrateFile(fullFile)
        selectedFile = fullFile
      } catch (error) {
        console.error('Failed to load file content:', error)
      }
    }

    paneOpenFile(selectedFile)

    if (viewMode !== 'editor') {
      openEditorWorkspace({ keepPreview: false })
    }

    // On mobile, switch to editor panel
    if (isMobile) {
      setMobilePanel('editor')
      setMobileOverlayPanel(null)
    }
  }, [paneOpenFile, viewMode, isMobile, hydrateFile, openEditorWorkspace])

  // Handle file content change
  const handleFileChange = useCallback((fileId: number, content: string, paneId: string) => {
    paneUpdateFileContent(fileId, content, paneId)
  }, [paneUpdateFileContent])

  // Handle file save
  const handleFileSave = useCallback(async (fileId: number, content: string, paneId: string) => {
    try {
      await apiService.updateFile(fileId, { content })
      paneMarkFileSaved(fileId, paneId)

      // Trigger preview hot reload when preview is active
      if (previewAutoRefresh && showPreview && currentProject) {
        const file = files.find(f => f.id === fileId)
        if (file) {
          try {
            await apiService.post('/preview/hot-reload', {
              project_id: currentProject.id,
              file_path: file.path,
              content: content
            })
          } catch {
            try {
              await apiService.post('/preview/refresh', {
                project_id: currentProject.id,
                changed_files: [file.path],
              })
            } catch {
              // Preview might not be running - ignore silently
            }
          }
        }
      }
    } catch (error) {
      console.error('Failed to save file:', error)
    }
  }, [paneMarkFileSaved, previewAutoRefresh, showPreview, currentProject, files])

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
    setCurrentProject(project)
    setShowPreview(false)
    setViewMode('dashboard')
    if (isMobile) {
      setMobilePanel('editor')
      setMobileOverlayPanel(null)
    }
  }, [isMobile, setCurrentProject])

  // Handle project selection
  const handleProjectSelect = useCallback((project: any) => {
    setCurrentProject(project)
    setShowPreview(false)
    setViewMode('dashboard')
    if (isMobile) {
      setMobilePanel('editor')
      setMobileOverlayPanel(null)
    }
  }, [isMobile, setCurrentProject])

  const handleProjectRun = useCallback((project: any) => {
    setCurrentProject(project)
    setShowPreview(true)
    setViewMode('preview')
    if (bottomPanelState === 'collapsed') {
      setBottomPanelState('normal')
    }
    if (isMobile) {
      setMobilePanel('editor')
      setMobileOverlayPanel(null)
    }
  }, [bottomPanelState, isMobile, setCurrentProject])

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
    openPreviewWorkspace()
  }, [currentProject, openPreviewWorkspace])

  const handleDashboardSettings = useCallback(() => {
    setRightPanelState('normal')
    setActiveRightTab('settings')
  }, [])

  // Handle file creation
  const handleFileCreate = useCallback(async (parentPath: string, name: string, type: 'file' | 'directory') => {
    if (!currentProject) return

    try {
      const fullPath = parentPath === '/' ? `/${name}` : `${parentPath}/${name}`
      const newFile = await createFileAction(currentProject.id, {
        name: name,
        path: fullPath,
        type: type,
        content: type === 'file' ? '' : undefined
      })

      setTerminalOutput(prev => [...prev, `Created ${type}: ${fullPath}`])

      if (type === 'file') {
        await handleFileSelect(newFile)
      }
    } catch (error) {
      console.error('Failed to create file:', error)
      setTerminalOutput(prev => [...prev, `Failed to create ${type}: ${error}`])
    }
  }, [currentProject, handleFileSelect, createFileAction])

  // Handle file deletion
  const handleFileDelete = useCallback(async (file: File) => {
    try {
      await deleteFileAction(file.id)

      paneCloseFile(file.id)

      setTerminalOutput(prev => [...prev, `Deleted: ${file.path}`])
    } catch (error) {
      console.error('Failed to delete file:', error)
      setTerminalOutput(prev => [...prev, `Failed to delete: ${error}`])
    }
  }, [paneCloseFile, deleteFileAction])

  // Handle file rename
  const handleFileRename = useCallback(async (file: File, newName: string) => {
    try {
      const pathParts = file.path.split('/')
      pathParts[pathParts.length - 1] = newName
      const newPath = pathParts.join('/')

      const updated = await apiService.updateFile(file.id, {
        name: newName,
        path: newPath
      })

      hydrateFile(updated)

      // Update in all panes
      layout.panes.forEach(pane => {
        if (pane.files.find(f => f.file.id === file.id)) {
          // This is a bit complex as paneManager doesn't have rename, 
          // but we can just reopen the file with new data
          paneOpenFile({ ...updated }, pane.id)
        }
      })

      if (currentProject) {
        await fetchFiles(currentProject.id)
      }

      setTerminalOutput(prev => [...prev, `Renamed: ${file.name} -> ${newName}`])
    } catch (error) {
      console.error('Failed to rename file:', error)
      setTerminalOutput(prev => [...prev, `Failed to rename: ${error}`])
    }
  }, [layout.panes, paneOpenFile, currentProject, fetchFiles, hydrateFile])

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
          const language = (currentProject.language || '').toLowerCase()
          if (language === 'javascript' || language === 'typescript') {
            openPreviewWorkspace()
            setTerminalOutput(prev => [...prev, 'Preview workspace opened.'])
            return
          }
          setTerminalOutput(prev => [...prev, 'Executing project...'])
          const execution = await apiService.executeProject({
            project_id: currentProject.id,
          })
          if (execution.output) {
            setTerminalOutput(prev => [...prev, execution.output])
          } else {
            setTerminalOutput(prev => [...prev, `Execution finished with status: ${execution.status}`])
          }
        }
      } else if (command === 'clear') {
        setTerminalOutput(['APEX-BUILD Terminal v1.0.0', 'Welcome to the future of development', ''])
      } else {
        setTerminalOutput(prev => [...prev, `Command not found: ${command}`])
      }
    } catch (error) {
      setTerminalOutput(prev => [...prev, `Error: ${error}`])
    }
  }, [currentProject, openPreviewWorkspace])

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
          <SearchPanel
            projectId={currentProject?.id}
            onFileOpen={(file, line) => {
              handleFileSelect(file)
              if (line) {
                splitPaneRef.current?.revealLine(line)
              }
            }}
            className="h-full border-0"
          />
        )
      case 'git':
        return (
          <GitPanel
            projectId={currentProject?.id}
            className="h-full border-0"
          />
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
      case 'deploy':
        return currentProject ? (
          <div className="h-full overflow-auto">
            <DeploymentPanel
              projectId={currentProject.id}
              projectName={currentProject.name}
              className="min-h-full"
            />
          </div>
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
          <Card variant="cyberpunk" padding="md" className="h-full border-0 flex flex-col">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-2">
                <FileText className="w-4 h-4 text-cyan-400" />
                <h3 className="text-sm font-semibold text-white">Output Log</h3>
              </div>
              <Button
                size="xs"
                variant="ghost"
                onClick={() => setTerminalOutput([])}
                className="text-gray-400 hover:text-white"
              >
                Clear
              </Button>
            </div>
            <div className="flex-1 overflow-auto rounded bg-black/40 border border-gray-800 p-2 font-mono text-xs text-gray-300 space-y-1">
              {terminalOutput.length > 0 ? (
                terminalOutput.map((line, idx) => (
                  <div key={`${idx}-${line}`} className="whitespace-pre-wrap">
                    {line}
                  </div>
                ))
              ) : (
                <div className="text-gray-500 text-center py-4">No output yet</div>
              )}
            </div>
          </Card>
        )
      case 'problems':
        return (
          <Card variant="cyberpunk" padding="md" className="h-full border-0 flex flex-col">
            <div className="flex items-center gap-2 mb-2">
              <Zap className="w-4 h-4 text-yellow-400" />
              <h3 className="text-sm font-semibold text-white">Problems</h3>
            </div>
            <div className="flex-1 overflow-auto rounded bg-black/40 border border-gray-800 p-2 text-xs">
              {problemLines.length === 0 ? (
                <div className="text-gray-500 text-center py-4">No problems detected</div>
              ) : (
                <div className="space-y-2">
                  {problemLines.map((line, idx) => (
                    <div key={`${idx}-${line}`} className="flex items-start gap-2 text-red-300">
                      <AlertCircle className="w-3.5 h-3.5 mt-0.5 text-red-400" />
                      <span className="whitespace-pre-wrap">{line}</span>
                    </div>
                  ))}
                </div>
              )}
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
          <div className="h-full overflow-auto">
            <ProjectList
              onProjectSelect={handleProjectSelect}
              onProjectCreate={handleProjectCreate}
              onProjectRun={handleProjectRun}
              className="p-4 md:p-6"
            />
          </div>
        )
      case 'dashboard':
        return (
          <div className="h-full overflow-auto">
            <ProjectDashboard
              projectId={currentProject?.id}
              className="min-h-full"
              onShare={handleDashboardShare}
              onSettings={handleDashboardSettings}
              onRunProject={handleDashboardRun}
              onDownload={handleDashboardDownload}
            />
          </div>
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
              <div className="w-1/2 border-l border-gray-800 min-w-0 min-h-0 flex">
                <Suspense fallback={<EditorLoadingFallback />}>
                  <LivePreview
                    projectId={currentProject.id}
                    autoStart={true}
                    autoRefreshOnSave={previewAutoRefresh}
                    onAutoRefreshChange={setPreviewAutoRefresh}
                    className="h-full"
                  />
                </Suspense>
              </div>
            )}
          </div>
        )
      case 'preview':
        return currentProject ? (
          <div className="h-full flex flex-col bg-[radial-gradient(circle_at_top,#141b27_0%,#0b0f16_45%,#05070b_100%)]">
            <div className="border-b border-gray-800/80 px-4 py-4 md:px-6">
              <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
                <div className="min-w-0">
                  <div className="text-[11px] uppercase tracking-[0.24em] text-red-300/80">Preview Workspace</div>
                  <div className="mt-1 flex flex-wrap items-center gap-3">
                    <h2 className="text-xl font-semibold text-white">{currentProject.name}</h2>
                    <Badge variant="outline" className="border-red-500/40 bg-red-500/10 text-red-200">
                      Live Runtime
                    </Badge>
                    <Badge variant="outline" className="border-gray-700 bg-gray-900/70 text-gray-300">
                      Auto-refresh {previewAutoRefresh ? 'on' : 'off'}
                    </Badge>
                  </div>
                  <p className="mt-2 max-w-2xl text-sm text-gray-400">
                    Run the generated app in a dedicated workspace, then jump back into the editor only when you need to patch code.
                  </p>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setPreviewAutoRefresh((prev) => !prev)}
                    className="border-gray-700 bg-gray-900/70 text-gray-200 hover:bg-gray-800"
                  >
                    {previewAutoRefresh ? 'Pause Auto-refresh' : 'Enable Auto-refresh'}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => openEditorWorkspace({ keepPreview: true })}
                    className="border-cyan-500/40 bg-cyan-500/10 text-cyan-200 hover:bg-cyan-500/20"
                  >
                    Split With Editor
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => openEditorWorkspace()}
                    className="border-gray-700 bg-gray-900/70 text-gray-200 hover:bg-gray-800"
                  >
                    Open Editor Only
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={openDashboardView}
                    className={ideChromeButtonClass}
                  >
                    Back to Dashboard
                  </Button>
                </div>
              </div>
            </div>
            <div className="flex-1 min-h-0 p-4 md:p-6">
              <div className="h-full overflow-hidden rounded-2xl border border-red-500/20 bg-black/60 shadow-[0_24px_80px_rgba(0,0,0,0.35)]">
                <Suspense fallback={<EditorLoadingFallback />}>
                  <LivePreview
                    projectId={currentProject.id}
                    autoStart={true}
                    autoRefreshOnSave={previewAutoRefresh}
                    onAutoRefreshChange={setPreviewAutoRefresh}
                    className="h-full"
                  />
                </Suspense>
              </div>
            </div>
          </div>
        ) : null
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
        <LoadingOverlay isVisible={isLoading} text="Loading APEX-BUILD..." />

        {/* Mobile Navigation */}
        <MobileNavigation
          activeTab={mobilePanel}
          onTabChange={handleMobileTabChange}
          onMenuToggle={setMobileMenuOpen}
          user={user || undefined}
          onLogout={logout}
          onSearch={() => setActiveLeftTab('search')}
          onGitOpen={() => setActiveLeftTab('git')}
          onProjectsOpen={openProjectsView}
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
    <div className={cn('h-full flex flex-col bg-[radial-gradient(circle_at_top,#141b27_0%,#0b0f16_38%,#05070b_100%)] min-h-0', className)}>
      {/* Loading overlay */}
      <LoadingOverlay isVisible={isLoading} text="Loading APEX-BUILD..." />

      {/* Top bar */}
      <div className="h-14 bg-[#0b0f16]/92 backdrop-blur-md border-b border-white/5 shadow-[0_1px_0_rgba(255,255,255,0.03)] flex items-center justify-between px-4">
        {/* Left side */}
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded overflow-hidden flex items-center justify-center">
              <img
                src="/apex-build-logo-transparent.png"
                alt="APEX"
                className="w-6 h-6 object-contain drop-shadow-[0_0_6px_rgba(239,68,68,0.6)]"
                onError={(e) => {
                  const img = e.currentTarget
                  img.style.display = 'none'
                  const fallback = document.createElement('div')
                  fallback.className = 'w-6 h-6 bg-gradient-to-br from-red-500 to-red-900 rounded'
                  img.parentElement?.appendChild(fallback)
                }}
              />
            </div>
            <span className="text-lg font-bold text-white hidden sm:inline tracking-tight">APEX-BUILD</span>
          </div>

          {/* Navigation */}
          <div className="flex items-center gap-1">
              <Button
                size="sm"
                variant="ghost"
                onClick={openProjectsView}
                icon={<Folder size={14} />}
                className={viewMode === 'projects' ? ideChromeButtonActiveClass : ideChromeButtonClass}
              >
              <span className="hidden sm:inline">Projects</span>
            </Button>
            {currentProject && (
              <>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={openDashboardView}
                  icon={<FileText size={14} />}
                  className={viewMode === 'dashboard' ? ideChromeButtonActiveClass : ideChromeButtonClass}
                >
                  <span className="hidden sm:inline">Dashboard</span>
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => openEditorWorkspace({ keepPreview: false })}
                  icon={<Code size={14} />}
                  className={viewMode === 'editor' ? ideChromeButtonActiveClass : ideChromeButtonClass}
                >
                  <span className="hidden sm:inline">Editor</span>
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={openPreviewWorkspace}
                  icon={<Monitor size={14} />}
                  className={viewMode === 'preview' ? ideChromeButtonActiveClass : ideChromeButtonClass}
                >
                  <span className="hidden sm:inline">Preview</span>
                </Button>
                {viewMode === 'editor' && (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setShowPreview(!showPreview)}
                    icon={<Monitor size={14} />}
                    className={showPreview ? ideChromeButtonActiveClass : ideChromeButtonClass}
                  >
                    <span className="hidden sm:inline">Split Preview</span>
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
            <div className="hidden md:flex items-center gap-2 px-3 py-1.5 bg-gray-900/75 border border-gray-700/80 rounded-xl">
              <span className="text-sm font-medium text-gray-100 truncate max-w-[150px]">{currentProject.name}</span>
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
                className={ideChromeButtonClass}
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
                variant="ghost"
                icon={<Play size={14} />}
                className={viewMode === 'preview' ? ideChromeButtonActiveClass : ideChromeButtonClass}
                title={viewMode === 'preview' ? 'Close Preview Workspace' : 'Open Preview Workspace'}
                onClick={() => {
                  if (viewMode === 'preview') {
                    openEditorWorkspace()
                    return
                  }
                  openPreviewWorkspace()
                }}
              >
                <span className="hidden lg:inline">{viewMode === 'preview' ? 'Close Preview' : 'Preview'}</span>
              </Button>
              <Button
                size="sm"
                variant="ghost"
                icon={<Share2 size={14} />}
                className={ideChromeButtonClass}
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
              <div className="relative">
                <Button
                  size="sm"
                  variant="ghost"
                  icon={<Bell size={14} />}
                  className={cn(ideIconButtonClass, showNotifications && '!border-gray-600 !bg-gray-800 !text-white')}
                  onClick={() => setShowNotifications(!showNotifications)}
                  aria-label="Notifications"
                  title="Notifications"
                />
                {showNotifications && (
                  <>
                    {/* Backdrop to close on outside click */}
                    <div
                      className="fixed inset-0 z-40"
                      onClick={() => setShowNotifications(false)}
                    />
                    <div className="absolute right-0 mt-2 w-64 bg-gray-900/95 backdrop-blur-md border border-gray-700/80 rounded-xl shadow-2xl shadow-black/60 z-50">
                      <div className="px-4 py-3 border-b border-gray-800 text-sm font-semibold text-white">
                        Notifications
                      </div>
                      <div className="px-4 py-6 text-center">
                        <Bell className="w-8 h-8 text-gray-700 mx-auto mb-2" />
                        <p className="text-xs text-gray-500">No notifications yet</p>
                      </div>
                    </div>
                  </>
                )}
              </div>
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
      <div ref={mainContentRef} className="flex-1 flex overflow-hidden min-h-0 min-w-0">
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
            <div className="h-10 bg-gray-800/50 border-b border-gray-700/50 flex items-center justify-between shrink-0">
              {leftPanelState === 'collapsed' ? (
                /* Collapsed: show only expand button, centered */
                <div className="flex-1 flex items-center justify-center">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setLeftPanelState('normal')}
                    icon={<ChevronRight size={14} />}
                    className={ideIconButtonClass}
                    aria-label="Expand sidebar"
                    title="Expand sidebar"
                  />
                </div>
              ) : (
                /* Expanded: show tabs and collapse button */
                <>
                  <div className="flex overflow-hidden">
                    <Button
                      size="sm"
                      variant={activeLeftTab === 'explorer' ? 'primary' : 'ghost'}
                      onClick={() => setActiveLeftTab('explorer')}
                      icon={<Folder size={14} />}
                      className={idePanelTabClass(activeLeftTab === 'explorer')}
                      title="Explorer"
                    >
                      Explorer
                    </Button>
                    <Button
                      size="sm"
                      variant={activeLeftTab === 'search' ? 'primary' : 'ghost'}
                      onClick={() => setActiveLeftTab('search')}
                      icon={<Search size={14} />}
                      className={idePanelTabClass(activeLeftTab === 'search')}
                      title="Search"
                    >
                      Search
                    </Button>
                    <Button
                      size="sm"
                      variant={activeLeftTab === 'git' ? 'primary' : 'ghost'}
                      onClick={() => setActiveLeftTab('git')}
                      icon={<GitBranch size={14} />}
                      className={idePanelTabClass(activeLeftTab === 'git')}
                      title="Git"
                    >
                      Git
                    </Button>
                    <Button
                      size="sm"
                      variant={activeLeftTab === 'history' ? 'primary' : 'ghost'}
                      onClick={() => setActiveLeftTab('history')}
                      icon={<RotateCcw size={14} />}
                      className={idePanelTabClass(activeLeftTab === 'history')}
                      title="Version History"
                    >
                      History
                    </Button>
                  </div>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setLeftPanelState('collapsed')}
                    icon={<PanelLeftClose size={14} />}
                    className={cn(ideIconButtonClass, 'mr-1 shrink-0')}
                    aria-label="Collapse sidebar"
                    title="Collapse sidebar"
                  />
                </>
              )}
            </div>

            {/* Sidebar content */}
            <div className="flex-1 min-h-0 overflow-hidden">
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
          {(viewMode === 'dashboard' || viewMode === 'editor') && bottomPanelState !== 'collapsed' && (
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
                    size="sm"
                    variant={activeBottomTab === 'terminal' ? 'primary' : 'ghost'}
                    onClick={() => setActiveBottomTab('terminal')}
                    icon={<Terminal size={14} />}
                    className={idePanelTabClass(activeBottomTab === 'terminal')}
                  >
                    Terminal
                  </Button>
                  <Button
                    size="sm"
                    variant={activeBottomTab === 'output' ? 'primary' : 'ghost'}
                    onClick={() => setActiveBottomTab('output')}
                    icon={<FileText size={14} />}
                    className={idePanelTabClass(activeBottomTab === 'output')}
                  >
                    Output
                  </Button>
                  <Button
                    size="sm"
                    variant={activeBottomTab === 'problems' ? 'primary' : 'ghost'}
                    onClick={() => setActiveBottomTab('problems')}
                    icon={<Zap size={14} />}
                    className={idePanelTabClass(activeBottomTab === 'problems')}
                  >
                    Problems
                  </Button>
                </div>

                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => setBottomPanelState('collapsed')}
                  icon={<X size={14} />}
                  className={ideIconButtonClass}
                />
              </div>

              {/* Bottom content */}
              <div className={cn(
                'overflow-hidden',
                bottomPanelState === 'normal' ? 'h-[calc(100%-40px)]' : 'h-[calc(100%-40px)]'
              )}>
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
            <div className="h-10 bg-gray-800/50 border-b border-gray-700/50 flex items-center justify-between shrink-0">
              {rightPanelState === 'collapsed' ? (
                /* Collapsed: show only expand button, centered */
                <div className="flex-1 flex items-center justify-center">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setRightPanelState('normal')}
                    icon={<ChevronLeft size={14} />}
                    className={ideIconButtonClass}
                    aria-label="Expand right panel"
                    title="Expand right panel"
                  />
                </div>
              ) : (
                /* Expanded: show collapse button and tabs */
                <>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setRightPanelState('collapsed')}
                    icon={<PanelRightClose size={14} />}
                    className={cn(ideIconButtonClass, 'ml-1 shrink-0')}
                    aria-label="Collapse right panel"
                    title="Collapse right panel"
                  />
                  <div className="flex overflow-hidden">
                    <Button
                      size="sm"
                      variant={activeRightTab === 'ai' ? 'primary' : 'ghost'}
                      onClick={() => setActiveRightTab('ai')}
                      icon={<Brain size={14} />}
                      className={idePanelTabClass(activeRightTab === 'ai')}
                      title="AI Assistant"
                    >
                      AI
                    </Button>
                    <Button
                      size="sm"
                      variant={activeRightTab === 'comments' ? 'primary' : 'ghost'}
                      onClick={() => setActiveRightTab('comments')}
                      icon={<MessageSquare size={14} />}
                      className={idePanelTabClass(activeRightTab === 'comments')}
                      title="Code Comments"
                    >
                      Comments
                    </Button>
                    <Button
                      size="sm"
                      variant={activeRightTab === 'collab' ? 'primary' : 'ghost'}
                      onClick={() => setActiveRightTab('collab')}
                      icon={<Users size={14} />}
                      className={idePanelTabClass(activeRightTab === 'collab')}
                      title="Collaboration"
                    >
                      Collab
                    </Button>
                    <Button
                      size="sm"
                      variant={activeRightTab === 'database' ? 'primary' : 'ghost'}
                      onClick={() => setActiveRightTab('database')}
                      icon={<Database size={14} />}
                      className={idePanelTabClass(activeRightTab === 'database')}
                      title="Database"
                    >
                      Database
                    </Button>
                    <Button
                      size="sm"
                      variant={activeRightTab === 'deploy' ? 'primary' : 'ghost'}
                      onClick={() => setActiveRightTab('deploy')}
                      icon={<Rocket size={14} />}
                      className={idePanelTabClass(activeRightTab === 'deploy')}
                      title="Deploy"
                    >
                      Deploy
                    </Button>
                    <Button
                      size="sm"
                      variant={activeRightTab === 'settings' ? 'primary' : 'ghost'}
                      onClick={() => setActiveRightTab('settings')}
                      icon={<Settings size={14} />}
                      className={idePanelTabClass(activeRightTab === 'settings')}
                      title="Settings"
                    >
                      Settings
                    </Button>
                  </div>
                </>
              )}
            </div>

            {/* Sidebar content */}
            <div className="flex-1 min-h-0 overflow-hidden">
              {rightPanelState !== 'collapsed' && renderRightPanel()}
            </div>
          </div>
        )}
      </div>

      {/* Footer for panels toggle */}
      <div className="h-7 bg-gray-900/95 border-t border-gray-800/60 flex items-center justify-between px-3 shrink-0">
        <div className="flex items-center gap-1">
          <button
            onClick={() => setBottomPanelState(bottomPanelState === 'collapsed' ? 'normal' : 'collapsed')}
            title={bottomPanelState === 'collapsed' ? 'Open Terminal' : 'Close Terminal'}
            aria-label={bottomPanelState === 'collapsed' ? 'Open Terminal' : 'Close Terminal'}
            className={cn(
              'flex items-center gap-1.5 px-2.5 py-1 rounded text-[11px] font-medium transition-colors duration-150',
              bottomPanelState !== 'collapsed'
                ? 'text-white bg-red-500/15 hover:bg-red-500/20'
                : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/70'
            )}
          >
            <Terminal size={11} />
            <span>Terminal</span>
          </button>
        </div>
        <div className="text-[10px] text-gray-600 font-mono select-none">APEX-BUILD</div>
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
