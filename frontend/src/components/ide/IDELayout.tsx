// APEX.BUILD IDE Layout
// Dark Demon Theme - Fully responsive development environment interface

import React, { useState, useEffect, useCallback, useRef } from 'react'
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
import { File, AICapability } from '@/types'
import {
  Button,
  Badge,
  Avatar,
  Card,
  Loading,
  LoadingOverlay
} from '@/components/ui'
import { MonacoEditor } from '@/components/editor/MonacoEditor'
import { AIAssistant } from '@/components/ai/AIAssistant'
import { FileTree } from '@/components/explorer/FileTree'
import { ProjectDashboard } from '@/components/project/ProjectDashboard'
import { ProjectList } from '@/components/project/ProjectList'
import { MobileNavigation, MobilePanelSwitcher } from '@/components/mobile'
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
  GitBranch,
  Bell,
  User,
  LogOut,
  ChevronLeft,
  ChevronRight,
  PanelLeftClose,
  PanelRightClose,
  PanelBottomClose,
} from 'lucide-react'

export interface IDELayoutProps {
  className?: string
}

type ViewMode = 'projects' | 'dashboard' | 'editor'
type PanelState = 'collapsed' | 'normal' | 'expanded'
type MobilePanel = 'explorer' | 'editor' | 'terminal' | 'ai' | 'settings'

export const IDELayout: React.FC<IDELayoutProps> = ({ className }) => {
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
  const [activeLeftTab, setActiveLeftTab] = useState<'explorer' | 'search' | 'git'>('explorer')
  const [activeRightTab, setActiveRightTab] = useState<'ai' | 'collab' | 'settings'>('ai')
  const [activeBottomTab, setActiveBottomTab] = useState<'terminal' | 'output' | 'problems'>('terminal')

  // Mobile-specific state
  const [mobilePanel, setMobilePanel] = useState<MobilePanel>('editor')
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [mobileOverlayPanel, setMobileOverlayPanel] = useState<'left' | 'right' | null>(null)

  // Editor state
  const [openFiles, setOpenFiles] = useState<File[]>([])
  const [activeFile, setActiveFile] = useState<File | null>(null)
  const [fileContents, setFileContents] = useState<Map<number, string>>(new Map())
  const [unsavedChanges, setUnsavedChanges] = useState<Set<number>>(new Set())

  // Terminal state
  const [terminalOutput, setTerminalOutput] = useState<string[]>([
    'APEX.BUILD Terminal v1.0.0',
    'Welcome to the future of development',
    ''
  ])
  const [terminalInput, setTerminalInput] = useState('')

  // Component refs
  const editorRef = useRef<any>(null)
  const terminalRef = useRef<HTMLDivElement>(null)
  const mainContentRef = useRef<HTMLDivElement>(null)

  const {
    user,
    currentProject,
    files,
    isLoading,
    currentTheme: theme,
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
    if (!openFiles.find(f => f.id === file.id)) {
      setOpenFiles(prev => [...prev, file])
    }
    setActiveFile(file)

    if (viewMode !== 'editor') {
      setViewMode('editor')
    }

    // On mobile, switch to editor panel
    if (isMobile) {
      setMobilePanel('editor')
      setMobileOverlayPanel(null)
    }
  }, [openFiles, viewMode, isMobile])

  // Handle file content change
  const handleFileChange = useCallback((content: string) => {
    if (!activeFile) return

    setFileContents(prev => new Map(prev.set(activeFile.id, content)))
    setUnsavedChanges(prev => new Set(prev.add(activeFile.id)))

    // Send real-time collaboration updates
    if (websocketService.isConnected() && currentProject) {
      const editor = editorRef.current
      if (editor) {
        const position = editor.getPosition()
        websocketService.sendFileChange(
          activeFile.id,
          content,
          position?.lineNumber || 1,
          position?.column || 1
        )
      }
    }
  }, [activeFile, currentProject])

  // Handle file save
  const handleFileSave = useCallback(async (content: string) => {
    if (!activeFile) return

    try {
      await apiService.updateFile(activeFile.id, { content })
      setUnsavedChanges(prev => {
        const newSet = new Set(prev)
        newSet.delete(activeFile.id)
        return newSet
      })
    } catch (error) {
      console.error('Failed to save file:', error)
    }
  }, [activeFile])

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
  const closeFile = useCallback((file: File) => {
    setOpenFiles(prev => prev.filter(f => f.id !== file.id))
    if (activeFile?.id === file.id) {
      const remainingFiles = openFiles.filter(f => f.id !== file.id)
      setActiveFile(remainingFiles[remainingFiles.length - 1] || null)
    }
  }, [openFiles, activeFile])

  // Handle project creation
  const handleProjectCreate = useCallback((project: any) => {
    setViewMode('dashboard')
  }, [])

  // Handle project selection
  const handleProjectSelect = useCallback((project: any) => {
    setViewMode('dashboard')
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

      setOpenFiles(prev => prev.filter(f => f.id !== file.id))
      if (activeFile?.id === file.id) {
        const remainingFiles = openFiles.filter(f => f.id !== file.id)
        setActiveFile(remainingFiles[remainingFiles.length - 1] || null)
      }

      setTerminalOutput(prev => [...prev, `Deleted: ${file.path}`])
    } catch (error) {
      console.error('Failed to delete file:', error)
      setTerminalOutput(prev => [...prev, `Failed to delete: ${error}`])
    }
  }, [activeFile, openFiles])

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

      setOpenFiles(prev => prev.map(f =>
        f.id === file.id ? { ...f, name: newName, path: newPath } : f
      ))

      if (activeFile?.id === file.id) {
        setActiveFile({ ...activeFile, name: newName, path: newPath })
      }

      setTerminalOutput(prev => [...prev, `Renamed: ${file.name} -> ${newName}`])
    } catch (error) {
      console.error('Failed to rename file:', error)
      setTerminalOutput(prev => [...prev, `Failed to rename: ${error}`])
    }
  }, [activeFile])

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
              const editor = editorRef.current
              if (editor) {
                const position = editor.getPosition()
                editor.executeEdits('ai-insert', [{
                  range: {
                    startLineNumber: position.lineNumber,
                    startColumn: position.column,
                    endLineNumber: position.lineNumber,
                    endColumn: position.column
                  },
                  text: code
                }])
              }
            }}
            className="h-full border-0"
          />
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
      case 'settings':
        return (
          <Card variant="cyberpunk" padding="md" className="h-full border-0">
            <h3 className="text-lg font-semibold text-white mb-4">Settings</h3>
            <div className="space-y-4">
              <div>
                <label className="text-sm font-medium text-gray-300">Theme</label>
                <select
                  value={theme.id}
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

import XTerminal from '@/components/terminal/XTerminal'

// ... existing imports

  // Render bottom panel content
  const renderBottomPanel = () => {
    switch (activeBottomTab) {
      case 'terminal':
        return (
          <div className="h-full bg-black">
            <XTerminal
              projectId={currentProject?.id}
              theme={theme.id}
              onTitleChange={(title) => {
                // Optional: update tab title
              }}
            />
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
          />
        )
      case 'editor':
        return (
          <div className="h-full flex flex-col">
            {/* File tabs - scrollable on mobile */}
            {openFiles.length > 0 && (
              <div className="flex items-center bg-gray-900/80 border-b border-gray-700/50 px-1 py-1 overflow-x-auto scrollbar-hide">
                {openFiles.map(file => {
                  const hasUnsavedChanges = unsavedChanges.has(file.id)
                  const isActive = activeFile?.id === file.id

                  return (
                    <div
                      key={file.id}
                      className={cn(
                        'flex items-center gap-2 px-3 py-2 cursor-pointer transition-colors group min-w-0 shrink-0',
                        'touch-target',
                        isActive
                          ? 'bg-gray-800 border-b-2 border-red-500'
                          : 'hover:bg-gray-800/50'
                      )}
                      onClick={() => setActiveFile(file)}
                    >
                      <span className="text-xs">{file.name.split('.').pop()?.toUpperCase()}</span>
                      <span className={cn(
                        'text-sm truncate max-w-[100px] md:max-w-none',
                        isActive ? 'text-white' : 'text-gray-400'
                      )}>
                        {file.name}
                      </span>
                      {hasUnsavedChanges && (
                        <div className="w-1.5 h-1.5 bg-red-500 rounded-full" />
                      )}
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          closeFile(file)
                        }}
                        className="ml-1 opacity-0 group-hover:opacity-100 md:hover:bg-gray-700 rounded transition-all p-1 touch-target"
                      >
                        <X size={12} />
                      </button>
                    </div>
                  )
                })}
              </div>
            )}

            {/* Editor */}
            <div className="flex-1 min-h-0">
              {activeFile ? (
                <MonacoEditor
                  ref={editorRef}
                  file={activeFile}
                  value={fileContents.get(activeFile.id) || activeFile.content}
                  onChange={handleFileChange}
                  onSave={handleFileSave}
                  onAIRequest={handleAIRequest}
                  height="100%"
                />
              ) : (
                <div className="h-full flex items-center justify-center text-center p-4">
                  <div>
                    <Code className="w-12 h-12 md:w-16 md:h-16 text-gray-600 mx-auto mb-4" />
                    <h3 className="text-base md:text-lg font-semibold text-gray-300 mb-2">No file open</h3>
                    <p className="text-sm text-gray-400">Select a file from the explorer to start editing</p>
                  </div>
                </div>
              )}
            </div>
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
        className={cn('h-screen flex flex-col bg-gray-950', className)}
        style={{
          height: `calc(var(--vh, 1vh) * 100)`,
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
    <div className={cn('h-screen flex flex-col bg-gray-950', className)}>
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
                  variant={activeRightTab === 'collab' ? 'primary' : 'ghost'}
                  onClick={() => setActiveRightTab('collab')}
                  icon={<Users size={14} />}
                  className="rounded-none border-0 touch-target"
                >
                  {rightPanelState !== 'collapsed' && 'Collab'}
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
    </div>
  )
}

export default IDELayout
