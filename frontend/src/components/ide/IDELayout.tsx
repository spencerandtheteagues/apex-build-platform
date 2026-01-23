// APEX.BUILD IDE Layout
// Complete cyberpunk development environment interface

import React, { useState, useEffect, useCallback, useRef } from 'react'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
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
import {
  Sidebar,
  SidebarLeft,
  SidebarRight,
  Panel,
  PanelGroup,
  PanelResizeHandle,
} from 'react-resizable-panels'
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
  LogOut
} from 'lucide-react'

export interface IDELayoutProps {
  className?: string
}

type ViewMode = 'projects' | 'dashboard' | 'editor'
type PanelState = 'collapsed' | 'normal' | 'expanded'

export const IDELayout: React.FC<IDELayoutProps> = ({ className }) => {
  // Layout state
  const [viewMode, setViewMode] = useState<ViewMode>('projects')
  const [leftPanelState, setLeftPanelState] = useState<PanelState>('normal')
  const [rightPanelState, setRightPanelState] = useState<PanelState>('normal')
  const [bottomPanelState, setBottomPanelState] = useState<PanelState>('collapsed')
  const [activeLeftTab, setActiveLeftTab] = useState<'explorer' | 'search' | 'git'>('explorer')
  const [activeRightTab, setActiveRightTab] = useState<'ai' | 'collab' | 'settings'>('ai')
  const [activeBottomTab, setActiveBottomTab] = useState<'terminal' | 'output' | 'problems'>('terminal')

  // Editor state
  const [openFiles, setOpenFiles] = useState<File[]>([])
  const [activeFile, setActiveFile] = useState<File | null>(null)
  const [fileContents, setFileContents] = useState<Map<number, string>>(new Map())
  const [unsavedChanges, setUnsavedChanges] = useState<Set<number>>(new Set())

  // Terminal state
  const [terminalOutput, setTerminalOutput] = useState<string[]>([
    '🚀 APEX.BUILD Terminal v1.0.0',
    '⚡ Welcome to the future of development',
    ''
  ])
  const [terminalInput, setTerminalInput] = useState('')

  // Component refs
  const editorRef = useRef<any>(null)
  const terminalRef = useRef<HTMLDivElement>(null)

  const {
    user,
    currentProject,
    files,
    isLoading,
    apiService,
    websocketService,
    theme,
    collaborationUsers,
    connect,
    disconnect
  } = useStore()

  // Initialize WebSocket connection when project is selected
  useEffect(() => {
    if (currentProject && user) {
      connect(currentProject.id)
      return () => disconnect()
    }
  }, [currentProject, user, connect, disconnect])

  // Handle file selection
  const handleFileSelect = useCallback((file: File) => {
    if (!openFiles.find(f => f.id === file.id)) {
      setOpenFiles(prev => [...prev, file])
    }
    setActiveFile(file)

    if (viewMode !== 'editor') {
      setViewMode('editor')
    }
  }, [openFiles, viewMode])

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
  }, [activeFile, websocketService, currentProject])

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

      // Update file in the files array
      // This would typically be handled by the store
    } catch (error) {
      console.error('Failed to save file:', error)
    }
  }, [activeFile, apiService])

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

      // The AI assistant component will handle displaying the response
      return response
    } catch (error) {
      console.error('AI request failed:', error)
      throw error
    }
  }, [activeFile, currentProject, apiService])

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

  // Terminal command execution
  const executeTerminalCommand = useCallback(async (command: string) => {
    setTerminalOutput(prev => [...prev, `$ ${command}`])
    setTerminalInput('')

    try {
      if (command.startsWith('run') || command === 'start') {
        // Execute project
        if (currentProject) {
          setTerminalOutput(prev => [...prev, '⚡ Executing project...'])
          const execution = await apiService.executeCode({
            project_id: currentProject.id,
            command: 'npm start', // This would be determined by project type
            language: currentProject.language,
          })
          setTerminalOutput(prev => [...prev, execution.output])
        }
      } else if (command === 'clear') {
        setTerminalOutput(['🚀 APEX.BUILD Terminal v1.0.0', '⚡ Welcome to the future of development', ''])
      } else {
        // Echo other commands for now
        setTerminalOutput(prev => [...prev, `Command not found: ${command}`])
      }
    } catch (error) {
      setTerminalOutput(prev => [...prev, `Error: ${error}`])
    }
  }, [currentProject, apiService])

  // Render left panel content
  const renderLeftPanel = () => {
    switch (activeLeftTab) {
      case 'explorer':
        return (
          <FileTree
            projectId={currentProject?.id}
            onFileSelect={handleFileSelect}
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
              // Insert code into editor
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
                  className="w-full mt-1 bg-gray-800 border border-gray-600 rounded px-3 py-2 text-white focus:border-cyan-400 focus:outline-none"
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
          <div className="h-full flex flex-col bg-black/90 border border-gray-700/50 rounded-lg">
            <div className="flex items-center justify-between px-4 py-2 bg-gray-900/80 border-b border-gray-700/50 rounded-t-lg">
              <div className="flex items-center gap-2">
                <Terminal className="w-4 h-4 text-cyan-400" />
                <span className="text-sm font-medium text-white">Terminal</span>
              </div>
              <Button
                size="xs"
                variant="ghost"
                onClick={() => setTerminalOutput(['🚀 APEX.BUILD Terminal v1.0.0', '⚡ Welcome to the future of development', ''])}
                icon={<RotateCcw size={12} />}
              >
                Clear
              </Button>
            </div>

            <div className="flex-1 p-4 font-mono text-sm overflow-auto" ref={terminalRef}>
              {terminalOutput.map((line, index) => (
                <div key={index} className="text-green-400 whitespace-pre-wrap">
                  {line}
                </div>
              ))}
            </div>

            <div className="flex items-center p-4 border-t border-gray-700/50">
              <span className="text-green-400 font-mono text-sm mr-2">$</span>
              <input
                type="text"
                value={terminalInput}
                onChange={(e) => setTerminalInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && terminalInput.trim()) {
                    executeTerminalCommand(terminalInput.trim())
                  }
                }}
                className="flex-1 bg-transparent text-green-400 font-mono text-sm focus:outline-none"
                placeholder="Enter command..."
              />
            </div>
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
            className="p-6"
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
            {/* File tabs */}
            {openFiles.length > 0 && (
              <div className="flex items-center bg-gray-900/80 border-b border-gray-700/50 px-1 py-1">
                {openFiles.map(file => {
                  const hasUnsavedChanges = unsavedChanges.has(file.id)
                  const isActive = activeFile?.id === file.id

                  return (
                    <div
                      key={file.id}
                      className={cn(
                        'flex items-center gap-2 px-3 py-2 cursor-pointer transition-colors group min-w-0',
                        isActive
                          ? 'bg-gray-800 border-b-2 border-cyan-400'
                          : 'hover:bg-gray-800/50'
                      )}
                      onClick={() => setActiveFile(file)}
                    >
                      <span className="text-xs">{file.name.split('.').pop()?.toUpperCase()}</span>
                      <span className={cn(
                        'text-sm truncate',
                        isActive ? 'text-white' : 'text-gray-400'
                      )}>
                        {file.name}
                      </span>
                      {hasUnsavedChanges && (
                        <div className="w-1.5 h-1.5 bg-cyan-400 rounded-full" />
                      )}
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          closeFile(file)
                        }}
                        className="ml-1 opacity-0 group-hover:opacity-100 hover:bg-gray-700 rounded transition-all p-1"
                      >
                        <X size={12} />
                      </button>
                    </div>
                  )
                })}
              </div>
            )}

            {/* Editor */}
            <div className="flex-1">
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
                <div className="h-full flex items-center justify-center text-center">
                  <div>
                    <Code className="w-16 h-16 text-gray-600 mx-auto mb-4" />
                    <h3 className="text-lg font-semibold text-gray-300 mb-2">No file open</h3>
                    <p className="text-gray-400">Select a file from the explorer to start editing</p>
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

  return (
    <div className={cn('h-screen flex flex-col bg-gray-950', className)}>
      {/* Loading overlay */}
      <LoadingOverlay isVisible={isLoading} text="Loading APEX.BUILD..." />

      {/* Top bar */}
      <div className="h-12 bg-gray-900/95 backdrop-blur-md border-b border-gray-800 flex items-center justify-between px-4">
        {/* Left side */}
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 bg-gradient-to-br from-cyan-400 to-blue-600 rounded" />
            <span className="text-lg font-bold text-white">APEX.BUILD</span>
          </div>

          {/* Navigation */}
          <div className="flex items-center gap-1">
            <Button
              size="sm"
              variant={viewMode === 'projects' ? 'primary' : 'ghost'}
              onClick={() => setViewMode('projects')}
              icon={<Folder size={14} />}
            >
              Projects
            </Button>
            {currentProject && (
              <>
                <Button
                  size="sm"
                  variant={viewMode === 'dashboard' ? 'primary' : 'ghost'}
                  onClick={() => setViewMode('dashboard')}
                  icon={<FileText size={14} />}
                >
                  Dashboard
                </Button>
                <Button
                  size="sm"
                  variant={viewMode === 'editor' ? 'primary' : 'ghost'}
                  onClick={() => setViewMode('editor')}
                  icon={<Code size={14} />}
                >
                  Editor
                </Button>
              </>
            )}
          </div>
        </div>

        {/* Right side */}
        <div className="flex items-center gap-2">
          {/* Project info */}
          {currentProject && (
            <div className="flex items-center gap-2 px-3 py-1 bg-gray-800 rounded">
              <span className="text-sm text-gray-300">{currentProject.name}</span>
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
      <div className="flex-1 flex overflow-hidden">
        {/* Left sidebar */}
        {(viewMode === 'dashboard' || viewMode === 'editor') && currentProject && (
          <div className={cn(
            'bg-gray-900/80 border-r border-gray-800 flex flex-col transition-all duration-300',
            leftPanelState === 'collapsed' && 'w-12',
            leftPanelState === 'normal' && 'w-80',
            leftPanelState === 'expanded' && 'w-96'
          )}>
            {/* Sidebar tabs */}
            <div className="h-10 bg-gray-800/50 border-b border-gray-700/50 flex items-center">
              <Button
                size="xs"
                variant={activeLeftTab === 'explorer' ? 'primary' : 'ghost'}
                onClick={() => setActiveLeftTab('explorer')}
                icon={<Folder size={14} />}
                className="rounded-none border-0"
              >
                {leftPanelState !== 'collapsed' && 'Explorer'}
              </Button>
              <Button
                size="xs"
                variant={activeLeftTab === 'search' ? 'primary' : 'ghost'}
                onClick={() => setActiveLeftTab('search')}
                icon={<Search size={14} />}
                className="rounded-none border-0"
              >
                {leftPanelState !== 'collapsed' && 'Search'}
              </Button>
              <Button
                size="xs"
                variant={activeLeftTab === 'git' ? 'primary' : 'ghost'}
                onClick={() => setActiveLeftTab('git')}
                icon={<GitBranch size={14} />}
                className="rounded-none border-0"
              >
                {leftPanelState !== 'collapsed' && 'Git'}
              </Button>
            </div>

            {/* Sidebar content */}
            <div className="flex-1 overflow-hidden">
              {leftPanelState !== 'collapsed' && renderLeftPanel()}
            </div>
          </div>
        )}

        {/* Main editor area */}
        <div className="flex-1 flex flex-col overflow-hidden">
          <div className="flex-1 overflow-hidden">
            {renderMainContent()}
          </div>

          {/* Bottom panel */}
          {bottomPanelState !== 'collapsed' && (
            <div className={cn(
              'bg-gray-900/80 border-t border-gray-800 transition-all duration-300',
              bottomPanelState === 'normal' && 'h-60',
              bottomPanelState === 'expanded' && 'h-96'
            )}>
              {/* Bottom tabs */}
              <div className="h-10 bg-gray-800/50 border-b border-gray-700/50 flex items-center justify-between">
                <div className="flex">
                  <Button
                    size="xs"
                    variant={activeBottomTab === 'terminal' ? 'primary' : 'ghost'}
                    onClick={() => setActiveBottomTab('terminal')}
                    icon={<Terminal size={14} />}
                    className="rounded-none border-0"
                  >
                    Terminal
                  </Button>
                  <Button
                    size="xs"
                    variant={activeBottomTab === 'output' ? 'primary' : 'ghost'}
                    onClick={() => setActiveBottomTab('output')}
                    icon={<FileText size={14} />}
                    className="rounded-none border-0"
                  >
                    Output
                  </Button>
                  <Button
                    size="xs"
                    variant={activeBottomTab === 'problems' ? 'primary' : 'ghost'}
                    onClick={() => setActiveBottomTab('problems')}
                    icon={<Zap size={14} />}
                    className="rounded-none border-0"
                  >
                    Problems
                  </Button>
                </div>

                <Button
                  size="xs"
                  variant="ghost"
                  onClick={() => setBottomPanelState('collapsed')}
                  icon={<X size={14} />}
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
            'bg-gray-900/80 border-l border-gray-800 flex flex-col transition-all duration-300',
            rightPanelState === 'collapsed' && 'w-12',
            rightPanelState === 'normal' && 'w-80',
            rightPanelState === 'expanded' && 'w-96'
          )}>
            {/* Sidebar tabs */}
            <div className="h-10 bg-gray-800/50 border-b border-gray-700/50 flex items-center">
              <Button
                size="xs"
                variant={activeRightTab === 'ai' ? 'primary' : 'ghost'}
                onClick={() => setActiveRightTab('ai')}
                icon={<Brain size={14} />}
                className="rounded-none border-0"
              >
                {rightPanelState !== 'collapsed' && 'AI'}
              </Button>
              <Button
                size="xs"
                variant={activeRightTab === 'collab' ? 'primary' : 'ghost'}
                onClick={() => setActiveRightTab('collab')}
                icon={<Users size={14} />}
                className="rounded-none border-0"
              >
                {rightPanelState !== 'collapsed' && 'Collab'}
              </Button>
              <Button
                size="xs"
                variant={activeRightTab === 'settings' ? 'primary' : 'ghost'}
                onClick={() => setActiveRightTab('settings')}
                icon={<Settings size={14} />}
                className="rounded-none border-0"
              >
                {rightPanelState !== 'collapsed' && 'Settings'}
              </Button>
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
          />
        </div>
      </div>
    </div>
  )
}

export default IDELayout