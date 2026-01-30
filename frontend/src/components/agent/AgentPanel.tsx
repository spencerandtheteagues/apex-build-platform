// APEX.BUILD Agent Panel Component
// Main UI for the AI Autonomous Agent - task input, progress display, and controls

import React, { useState, useRef, useEffect, useCallback, memo } from 'react'
import { cn } from '@/lib/utils'
import { generateId } from '@/lib/utils'
import {
  AgentTask,
  AgentStatus,
  AgentStep,
  FileChange,
  TerminalEntry,
  AgentCheckpoint,
  AgentStepUpdate,
  AgentStatusUpdate,
  AgentFileChangeUpdate,
  AgentTerminalUpdate,
  AgentCheckpointUpdate,
  AgentMessageUpdate,
} from '@/types/agent'
import { agentApi, createMockAgentTask } from '@/services/agentApi'
import { AgentProgress } from './AgentProgress'
import { GlassPanel } from '@/components/ui/GlassPanel'
import { NeonButton } from '@/components/ui/NeonButton'
import { Progress } from '@/components/ui/progress'
import { Button } from '@/components/ui'
import {
  Zap,
  Play,
  Pause,
  Square,
  RotateCcw,
  Terminal,
  FileCode,
  GitBranch,
  ChevronDown,
  ChevronRight,
  CheckCircle2,
  XCircle,
  AlertCircle,
  Info,
  Clock,
  Send,
  Bot,
  Sparkles,
  Settings,
  History,
  Folder,
  Code,
  Plus,
  Minus,
  RefreshCw,
} from 'lucide-react'

export interface AgentPanelProps {
  className?: string
  onNavigateToIDE?: () => void
  projectId?: number
}

type PanelTab = 'progress' | 'files' | 'terminal' | 'checkpoints'

// Terminal output entry component
const TerminalOutputEntry = memo<{ entry: TerminalEntry }>(({ entry }) => {
  const typeColors: Record<TerminalEntry['type'], string> = {
    command: 'text-cyan-400',
    output: 'text-gray-300',
    error: 'text-red-400',
    info: 'text-blue-400',
    success: 'text-green-400',
    warning: 'text-yellow-400',
  }

  const typeIcons: Record<TerminalEntry['type'], React.ReactNode> = {
    command: <span className="text-green-400">$</span>,
    output: null,
    error: <XCircle className="w-3 h-3" />,
    info: <Info className="w-3 h-3" />,
    success: <CheckCircle2 className="w-3 h-3" />,
    warning: <AlertCircle className="w-3 h-3" />,
  }

  return (
    <div className={cn('flex gap-2 font-mono text-sm', typeColors[entry.type])}>
      {typeIcons[entry.type] && (
        <span className="flex-shrink-0 mt-0.5">{typeIcons[entry.type]}</span>
      )}
      <pre className="whitespace-pre-wrap break-all">{entry.content}</pre>
    </div>
  )
})
TerminalOutputEntry.displayName = 'TerminalOutputEntry'

// File change entry component
const FileChangeEntry = memo<{ change: FileChange }>(({ change }) => {
  const [expanded, setExpanded] = useState(false)

  const typeIcons: Record<FileChange['type'], React.ReactNode> = {
    create: <Plus className="w-4 h-4 text-green-400" />,
    modify: <RefreshCw className="w-4 h-4 text-yellow-400" />,
    delete: <Minus className="w-4 h-4 text-red-400" />,
    rename: <GitBranch className="w-4 h-4 text-blue-400" />,
  }

  const typeLabels: Record<FileChange['type'], string> = {
    create: 'Created',
    modify: 'Modified',
    delete: 'Deleted',
    rename: 'Renamed',
  }

  return (
    <div className="border border-gray-700/50 rounded-lg overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-3 p-3 hover:bg-gray-800/50 transition-colors"
      >
        {expanded ? (
          <ChevronDown className="w-4 h-4 text-gray-400" />
        ) : (
          <ChevronRight className="w-4 h-4 text-gray-400" />
        )}
        {typeIcons[change.type]}
        <span className="flex-1 text-left text-sm text-gray-200 truncate">
          {change.path}
        </span>
        <span className="text-xs text-gray-500">{typeLabels[change.type]}</span>
        {change.diff && (
          <span className="flex items-center gap-1 text-xs">
            <span className="text-green-400">+{change.diff.additions}</span>
            <span className="text-red-400">-{change.diff.deletions}</span>
          </span>
        )}
      </button>

      {expanded && change.diff && (
        <div className="p-3 bg-black/50 border-t border-gray-700/50">
          <pre className="text-xs font-mono overflow-x-auto">
            {change.diff.hunks.map((hunk, i) => (
              <div key={i} className="mb-2">
                <div className="text-gray-500 mb-1">
                  @@ -{hunk.oldStart},{hunk.oldLines} +{hunk.newStart},{hunk.newLines} @@
                </div>
                {hunk.lines.map((line, j) => (
                  <div
                    key={j}
                    className={cn(
                      line.type === 'add' && 'text-green-400 bg-green-900/20',
                      line.type === 'delete' && 'text-red-400 bg-red-900/20',
                      line.type === 'context' && 'text-gray-400'
                    )}
                  >
                    {line.type === 'add' && '+'}
                    {line.type === 'delete' && '-'}
                    {line.type === 'context' && ' '}
                    {line.content}
                  </div>
                ))}
              </div>
            ))}
          </pre>
        </div>
      )}
    </div>
  )
})
FileChangeEntry.displayName = 'FileChangeEntry'

// Checkpoint entry component
const CheckpointEntry = memo<{
  checkpoint: AgentCheckpoint
  onRestore: () => void
}>(({ checkpoint, onRestore }) => {
  return (
    <div className="flex items-center gap-3 p-3 border border-gray-700/50 rounded-lg">
      <History className="w-4 h-4 text-purple-400" />
      <div className="flex-1 min-w-0">
        <p className="text-sm text-gray-200 truncate">{checkpoint.name}</p>
        <p className="text-xs text-gray-500">
          {new Date(checkpoint.createdAt).toLocaleTimeString()}
        </p>
      </div>
      {checkpoint.canRestore && (
        <Button size="xs" variant="ghost" onClick={onRestore}>
          <RotateCcw className="w-3 h-3 mr-1" />
          Restore
        </Button>
      )}
    </div>
  )
})
CheckpointEntry.displayName = 'CheckpointEntry'

// Main Agent Panel
export const AgentPanel: React.FC<AgentPanelProps> = memo(({
  className,
  onNavigateToIDE,
  projectId,
}) => {
  // State
  const [task, setTask] = useState<AgentTask | null>(null)
  const [description, setDescription] = useState('')
  const [buildMode, setBuildMode] = useState<'fast' | 'full'>('fast')
  const [isConnected, setIsConnected] = useState(false)
  const [activeTab, setActiveTab] = useState<PanelTab>('progress')
  const [userMessage, setUserMessage] = useState('')
  const [agentMessages, setAgentMessages] = useState<AgentMessageUpdate[]>([])
  const [isStarting, setIsStarting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Refs
  const terminalRef = useRef<HTMLDivElement>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  // Auto-scroll terminal
  useEffect(() => {
    if (terminalRef.current) {
      terminalRef.current.scrollTop = terminalRef.current.scrollHeight
    }
  }, [task?.terminalOutput])

  // Auto-scroll messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [agentMessages])

  // Update elapsed time
  useEffect(() => {
    if (!task || task.status !== 'executing') return

    const interval = setInterval(() => {
      setTask((prev) => {
        if (!prev || prev.status !== 'executing') return prev
        return {
          ...prev,
          elapsedTimeMs: Date.now() - new Date(prev.startedAt).getTime(),
        }
      })
    }, 100)

    return () => clearInterval(interval)
  }, [task?.status, task?.startedAt])

  // Handle start build
  const handleStartBuild = useCallback(async () => {
    if (!description.trim()) return

    setIsStarting(true)
    setError(null)

    try {
      // Create initial task
      const mockTask = createMockAgentTask(description)
      mockTask.mode = buildMode
      mockTask.status = 'initializing'
      setTask(mockTask)

      // Start the build
      const response = await agentApi.startBuild({
        description,
        mode: buildMode,
      })

      // Connect to WebSocket
      await agentApi.connect(response.build_id, {
        onConnected: () => {
          setIsConnected(true)
          setTask((prev) =>
            prev ? { ...prev, buildId: response.build_id, status: 'planning' } : prev
          )
        },
        onDisconnected: () => setIsConnected(false),
        onStatusUpdate: (data) => {
          setTask((prev) => {
            if (!prev) return prev
            return {
              ...prev,
              status: data.status,
              progress: data.progress,
              currentStep: prev.steps.find((s) => s.id === data.currentStepId),
            }
          })
        },
        onStepStart: (data) => {
          setTask((prev) => {
            if (!prev) return prev
            const steps = prev.steps.map((s) =>
              s.id === data.stepId
                ? { ...s, status: 'in_progress' as const, startedAt: new Date().toISOString() }
                : s
            )
            return { ...prev, steps }
          })
        },
        onStepProgress: (data) => {
          setTask((prev) => {
            if (!prev) return prev
            const steps = prev.steps.map((s) =>
              s.id === data.stepId
                ? { ...s, progress: data.progress, output: data.output || s.output }
                : s
            )
            return { ...prev, steps }
          })
        },
        onStepComplete: (data) => {
          setTask((prev) => {
            if (!prev) return prev
            const steps = prev.steps.map((s) =>
              s.id === data.stepId
                ? {
                    ...s,
                    status: 'completed' as const,
                    progress: 100,
                    completedAt: new Date().toISOString(),
                    duration: s.startedAt
                      ? Date.now() - new Date(s.startedAt).getTime()
                      : undefined,
                    output: data.output || s.output,
                    artifacts: data.artifacts || s.artifacts,
                  }
                : s
            )
            return { ...prev, steps }
          })
        },
        onStepFailed: (data) => {
          setTask((prev) => {
            if (!prev) return prev
            const steps = prev.steps.map((s) =>
              s.id === data.stepId
                ? { ...s, status: 'failed' as const, error: data.error }
                : s
            )
            return { ...prev, steps }
          })
        },
        onFileChange: (data) => {
          setTask((prev) => {
            if (!prev) return prev
            return {
              ...prev,
              fileChanges: [...prev.fileChanges, data.fileChange],
            }
          })
        },
        onTerminalOutput: (data) => {
          setTask((prev) => {
            if (!prev) return prev
            return {
              ...prev,
              terminalOutput: [...prev.terminalOutput, data.entry],
            }
          })
        },
        onCheckpointCreated: (data) => {
          setTask((prev) => {
            if (!prev) return prev
            return {
              ...prev,
              checkpoints: [...prev.checkpoints, data.checkpoint],
            }
          })
        },
        onAgentMessage: (data) => {
          setAgentMessages((prev) => [...prev, data])
        },
        onError: (errorMsg) => {
          setError(errorMsg)
        },
        onCompleted: (completedTask) => {
          setTask((prev) => ({
            ...prev!,
            ...completedTask,
            status: 'completed',
            completedAt: new Date().toISOString(),
          }))
        },
        onCancelled: () => {
          setTask((prev) =>
            prev ? { ...prev, status: 'cancelled' } : prev
          )
        },
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start build')
      setTask(null)
    } finally {
      setIsStarting(false)
    }
  }, [description, buildMode])

  // Handle pause
  const handlePause = useCallback(async () => {
    try {
      await agentApi.pause()
      setTask((prev) => (prev ? { ...prev, status: 'paused' } : prev))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to pause')
    }
  }, [])

  // Handle resume
  const handleResume = useCallback(async () => {
    try {
      await agentApi.resume()
      setTask((prev) => (prev ? { ...prev, status: 'executing' } : prev))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to resume')
    }
  }, [])

  // Handle stop
  const handleStop = useCallback(async () => {
    try {
      await agentApi.cancel()
      setTask((prev) => (prev ? { ...prev, status: 'cancelled' } : prev))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to stop')
    }
  }, [])

  // Handle rollback
  const handleRollback = useCallback(async (checkpointId: string) => {
    try {
      await agentApi.rollback(checkpointId)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to rollback')
    }
  }, [])

  // Handle send message
  const handleSendMessage = useCallback(() => {
    if (!userMessage.trim()) return
    agentApi.sendMessage(userMessage)
    setAgentMessages((prev) => [
      ...prev,
      { role: 'user', content: userMessage },
    ])
    setUserMessage('')
  }, [userMessage])

  // Handle new build
  const handleNewBuild = useCallback(() => {
    agentApi.disconnect()
    setTask(null)
    setDescription('')
    setAgentMessages([])
    setError(null)
    setIsConnected(false)
  }, [])

  // Check if build is active
  const isActive = task && ['initializing', 'planning', 'executing'].includes(task.status)
  const isPaused = task?.status === 'paused'
  const isComplete = task?.status === 'completed'
  const isFailed = task?.status === 'failed'
  const isCancelled = task?.status === 'cancelled'

  return (
    <div className={cn('flex flex-col h-full bg-gray-950', className)}>
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800/50">
        <div className="flex items-center gap-3">
          <div className="relative">
            <div className="w-10 h-10 bg-gradient-to-br from-cyan-500 to-purple-600 rounded-lg flex items-center justify-center shadow-lg shadow-cyan-500/30">
              <Bot className="w-6 h-6 text-white" />
            </div>
            {isActive && (
              <span className="absolute -top-1 -right-1 w-3 h-3 bg-cyan-400 rounded-full animate-pulse" />
            )}
          </div>
          <div>
            <h2 className="text-lg font-bold text-white">AI Agent</h2>
            <p className="text-xs text-gray-400">
              {isConnected ? 'Connected' : 'Autonomous Build System'}
            </p>
          </div>
        </div>

        {task && (
          <div className="flex items-center gap-2">
            {isActive && (
              <>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={isPaused ? handleResume : handlePause}
                >
                  {isPaused ? (
                    <Play className="w-4 h-4 mr-1" />
                  ) : (
                    <Pause className="w-4 h-4 mr-1" />
                  )}
                  {isPaused ? 'Resume' : 'Pause'}
                </Button>
                <Button size="sm" variant="ghost" onClick={handleStop}>
                  <Square className="w-4 h-4 mr-1" />
                  Stop
                </Button>
              </>
            )}
            {(isComplete || isFailed || isCancelled) && (
              <>
                <Button size="sm" variant="ghost" onClick={handleNewBuild}>
                  <Plus className="w-4 h-4 mr-1" />
                  New Build
                </Button>
                {isComplete && onNavigateToIDE && (
                  <NeonButton
                    variant="cyan"
                    size="sm"
                    onClick={onNavigateToIDE}
                    icon={<Code className="w-4 h-4" />}
                  >
                    Open in IDE
                  </NeonButton>
                )}
              </>
            )}
          </div>
        )}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-hidden">
        {!task ? (
          // Input form
          <div className="h-full flex flex-col items-center justify-center p-8">
            <GlassPanel
              variant="dark"
              blur="lg"
              border="glow"
              glow="cyan"
              padding="lg"
              className="w-full max-w-2xl"
            >
              <div className="text-center mb-6">
                <div className="inline-flex items-center justify-center w-16 h-16 bg-gradient-to-br from-cyan-500 to-purple-600 rounded-2xl mb-4 shadow-lg shadow-cyan-500/30">
                  <Sparkles className="w-8 h-8 text-white" />
                </div>
                <h3 className="text-2xl font-bold text-white mb-2">
                  Build with AI
                </h3>
                <p className="text-gray-400">
                  Describe what you want to build and let AI do the heavy lifting
                </p>
              </div>

              {/* Description input */}
              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  What would you like to build?
                </label>
                <textarea
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="E.g., A modern e-commerce dashboard with product management, analytics, and user authentication using React and Tailwind CSS..."
                  className="w-full h-32 bg-gray-900/50 border border-gray-700 rounded-lg px-4 py-3 text-white placeholder:text-gray-500 focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500/50 resize-none"
                />
              </div>

              {/* Build mode selector */}
              <div className="mb-6">
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Build Mode
                </label>
                <div className="grid grid-cols-2 gap-3">
                  <button
                    onClick={() => setBuildMode('fast')}
                    className={cn(
                      'p-4 rounded-lg border transition-all',
                      buildMode === 'fast'
                        ? 'border-cyan-500 bg-cyan-900/20 shadow-[0_0_15px_rgba(0,255,255,0.2)]'
                        : 'border-gray-700 bg-gray-900/50 hover:border-gray-600'
                    )}
                  >
                    <Zap className={cn(
                      'w-5 h-5 mb-2',
                      buildMode === 'fast' ? 'text-cyan-400' : 'text-gray-400'
                    )} />
                    <h4 className={cn(
                      'font-medium',
                      buildMode === 'fast' ? 'text-cyan-400' : 'text-gray-300'
                    )}>
                      Fast Mode
                    </h4>
                    <p className="text-xs text-gray-500 mt-1">
                      Quick implementation with essential features
                    </p>
                  </button>
                  <button
                    onClick={() => setBuildMode('full')}
                    className={cn(
                      'p-4 rounded-lg border transition-all',
                      buildMode === 'full'
                        ? 'border-purple-500 bg-purple-900/20 shadow-[0_0_15px_rgba(168,85,247,0.2)]'
                        : 'border-gray-700 bg-gray-900/50 hover:border-gray-600'
                    )}
                  >
                    <Settings className={cn(
                      'w-5 h-5 mb-2',
                      buildMode === 'full' ? 'text-purple-400' : 'text-gray-400'
                    )} />
                    <h4 className={cn(
                      'font-medium',
                      buildMode === 'full' ? 'text-purple-400' : 'text-gray-300'
                    )}>
                      Full Mode
                    </h4>
                    <p className="text-xs text-gray-500 mt-1">
                      Complete with tests, docs, and optimization
                    </p>
                  </button>
                </div>
              </div>

              {/* Error display */}
              {error && (
                <div className="mb-4 p-3 bg-red-900/20 border border-red-500/30 rounded-lg">
                  <p className="text-sm text-red-400">{error}</p>
                </div>
              )}

              {/* Build button */}
              <NeonButton
                variant="cyan"
                size="lg"
                glow="intense"
                pulse={!description.trim()}
                loading={isStarting}
                disabled={!description.trim() || isStarting}
                onClick={handleStartBuild}
                icon={<Zap className="w-5 h-5" />}
                className="w-full"
              >
                Build with AI
              </NeonButton>

              <p className="text-center text-xs text-gray-500 mt-4">
                AI will analyze requirements, create a plan, and implement your project
              </p>
            </GlassPanel>
          </div>
        ) : (
          // Build progress view
          <div className="h-full flex flex-col">
            {/* Task description */}
            <div className="px-6 py-3 bg-gray-900/50 border-b border-gray-800/50">
              <p className="text-sm text-gray-300 line-clamp-2">{task.description}</p>
            </div>

            {/* Tabs */}
            <div className="flex items-center gap-1 px-6 py-2 bg-gray-900/30 border-b border-gray-800/50">
              {[
                { id: 'progress' as const, label: 'Progress', icon: <Zap className="w-4 h-4" /> },
                { id: 'files' as const, label: 'Files', icon: <FileCode className="w-4 h-4" />, count: task.fileChanges.length },
                { id: 'terminal' as const, label: 'Terminal', icon: <Terminal className="w-4 h-4" /> },
                { id: 'checkpoints' as const, label: 'Checkpoints', icon: <History className="w-4 h-4" />, count: task.checkpoints.length },
              ].map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  className={cn(
                    'flex items-center gap-2 px-3 py-1.5 rounded-md text-sm transition-colors',
                    activeTab === tab.id
                      ? 'bg-cyan-900/30 text-cyan-400 border border-cyan-500/30'
                      : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/50'
                  )}
                >
                  {tab.icon}
                  {tab.label}
                  {tab.count !== undefined && tab.count > 0 && (
                    <span className="px-1.5 py-0.5 text-xs bg-gray-700 rounded">
                      {tab.count}
                    </span>
                  )}
                </button>
              ))}
            </div>

            {/* Tab content */}
            <div className="flex-1 overflow-hidden">
              {activeTab === 'progress' && (
                <div className="h-full overflow-y-auto p-6">
                  <AgentProgress task={task} />

                  {/* Agent messages */}
                  {agentMessages.length > 0 && (
                    <div className="mt-6">
                      <h4 className="text-sm font-medium text-gray-300 mb-3">
                        Agent Activity
                      </h4>
                      <div className="space-y-2 max-h-60 overflow-y-auto">
                        {agentMessages.map((msg, i) => (
                          <div
                            key={i}
                            className={cn(
                              'p-3 rounded-lg text-sm',
                              msg.role === 'user'
                                ? 'bg-cyan-900/20 border border-cyan-500/30 ml-8'
                                : msg.role === 'agent'
                                ? 'bg-purple-900/20 border border-purple-500/30 mr-8'
                                : 'bg-gray-800/50 border border-gray-700/50'
                            )}
                          >
                            {msg.agentName && (
                              <span className="text-xs text-purple-400 mb-1 block">
                                {msg.agentName}
                              </span>
                            )}
                            <p className="text-gray-200">{msg.content}</p>
                          </div>
                        ))}
                        <div ref={messagesEndRef} />
                      </div>
                    </div>
                  )}
                </div>
              )}

              {activeTab === 'files' && (
                <div className="h-full overflow-y-auto p-6">
                  {task.fileChanges.length === 0 ? (
                    <div className="flex flex-col items-center justify-center h-full text-center">
                      <Folder className="w-12 h-12 text-gray-600 mb-4" />
                      <p className="text-gray-400">No file changes yet</p>
                      <p className="text-xs text-gray-500 mt-1">
                        File changes will appear here as the agent works
                      </p>
                    </div>
                  ) : (
                    <div className="space-y-2">
                      {task.fileChanges.map((change) => (
                        <FileChangeEntry key={change.id} change={change} />
                      ))}
                    </div>
                  )}
                </div>
              )}

              {activeTab === 'terminal' && (
                <div className="h-full flex flex-col">
                  <div
                    ref={terminalRef}
                    className="flex-1 overflow-y-auto p-4 bg-black font-mono text-sm"
                  >
                    {task.terminalOutput.length === 0 ? (
                      <div className="flex flex-col items-center justify-center h-full text-center">
                        <Terminal className="w-12 h-12 text-gray-600 mb-4" />
                        <p className="text-gray-400">Terminal output will appear here</p>
                      </div>
                    ) : (
                      <div className="space-y-1">
                        {task.terminalOutput.map((entry) => (
                          <TerminalOutputEntry key={entry.id} entry={entry} />
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              )}

              {activeTab === 'checkpoints' && (
                <div className="h-full overflow-y-auto p-6">
                  {task.checkpoints.length === 0 ? (
                    <div className="flex flex-col items-center justify-center h-full text-center">
                      <History className="w-12 h-12 text-gray-600 mb-4" />
                      <p className="text-gray-400">No checkpoints yet</p>
                      <p className="text-xs text-gray-500 mt-1">
                        Checkpoints allow you to rollback to previous states
                      </p>
                    </div>
                  ) : (
                    <div className="space-y-2">
                      {task.checkpoints.map((checkpoint) => (
                        <CheckpointEntry
                          key={checkpoint.id}
                          checkpoint={checkpoint}
                          onRestore={() => handleRollback(checkpoint.id)}
                        />
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* Message input (when active) */}
            {isActive && (
              <div className="border-t border-gray-800/50 p-4">
                <div className="flex items-center gap-2">
                  <input
                    type="text"
                    value={userMessage}
                    onChange={(e) => setUserMessage(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault()
                        handleSendMessage()
                      }
                    }}
                    placeholder="Send a message to the agent..."
                    className="flex-1 bg-gray-900/50 border border-gray-700 rounded-lg px-4 py-2 text-white placeholder:text-gray-500 focus:border-cyan-500 focus:outline-none"
                  />
                  <Button
                    onClick={handleSendMessage}
                    disabled={!userMessage.trim()}
                  >
                    <Send className="w-4 h-4" />
                  </Button>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
})

AgentPanel.displayName = 'AgentPanel'

export default AgentPanel
