// APEX.BUILD Debug Panel
// Complete debug panel with session controls, step toolbar, collapsible
// sections for Variables / Watch / Call Stack / Breakpoints, debug console
// input for expression evaluation, and keyboard shortcuts display

import React, { useState, useCallback, useEffect, useRef } from 'react'
import { cn } from '@/lib/utils'
import debugService, {
  DebugSession,
  DebugSessionStatus,
  Variable,
  StackFrame,
  Breakpoint,
  WatchExpression,
  EvaluateResult,
  DebugEvent,
} from '@/services/debugService'
import { VariablesView } from './VariablesView'
import { CallStackView } from './CallStackView'
import { BreakpointManager } from './BreakpointManager'
import {
  Play,
  Square,
  SkipForward,
  ArrowDownToLine,
  ArrowUpFromLine,
  Pause,
  RotateCcw,
  ChevronDown,
  ChevronRight,
  Bug,
  Terminal,
  Plus,
  X,
  Trash2,
  Keyboard,
  Send,
  Eye,
  AlertCircle,
  Layers,
  CircleDot,
  Variable as VariableIcon,
} from 'lucide-react'
import { Badge } from '@/components/ui/Badge'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface DebugPanelProps {
  className?: string
  projectId?: number
  activeFilePath?: string
  activeFileLanguage?: string
  onNavigateToSource?: (filePath: string, line: number) => void
}

interface ConsoleEntry {
  id: string
  type: 'input' | 'output' | 'error' | 'info'
  content: string
  timestamp: string
}

type Section = 'variables' | 'watch' | 'callstack' | 'breakpoints'

// ---------------------------------------------------------------------------
// Status helpers
// ---------------------------------------------------------------------------

function statusBadge(status: DebugSessionStatus | null) {
  if (!status) {
    return <Badge variant="neutral" size="xs">Idle</Badge>
  }

  const map: Record<DebugSessionStatus, { variant: any; label: string }> = {
    pending: { variant: 'warning', label: 'Starting' },
    running: { variant: 'success', label: 'Running' },
    paused: { variant: 'primary', label: 'Paused' },
    completed: { variant: 'neutral', label: 'Stopped' },
    error: { variant: 'error', label: 'Error' },
  }

  const cfg = map[status]
  return <Badge variant={cfg.variant} size="xs">{cfg.label}</Badge>
}

// ---------------------------------------------------------------------------
// DebugPanel
// ---------------------------------------------------------------------------

const DebugPanel: React.FC<DebugPanelProps> = ({
  className,
  projectId,
  activeFilePath,
  activeFileLanguage,
  onNavigateToSource,
}) => {
  // Session state
  const [session, setSession] = useState<DebugSession | null>(
    debugService.getSession()
  )
  const [status, setStatus] = useState<DebugSessionStatus | null>(
    session?.status ?? null
  )

  // Data state
  const [localVars, setLocalVars] = useState<Variable[]>([])
  const [globalVars, setGlobalVars] = useState<Variable[]>([])
  const [callStack, setCallStack] = useState<StackFrame[]>([])
  const [breakpoints, setBreakpoints] = useState<Breakpoint[]>(
    debugService.getBreakpoints()
  )
  const [watches, setWatches] = useState<WatchExpression[]>(
    debugService.getWatches()
  )
  const [activeFrameId, setActiveFrameId] = useState<string | undefined>()

  // Console state
  const [consoleEntries, setConsoleEntries] = useState<ConsoleEntry[]>([])
  const [consoleInput, setConsoleInput] = useState('')
  const [consoleHistory, setConsoleHistory] = useState<string[]>([])
  const [historyIndex, setHistoryIndex] = useState(-1)
  const consoleEndRef = useRef<HTMLDivElement>(null)
  const consoleInputRef = useRef<HTMLInputElement>(null)

  // Watch input
  const [watchInput, setWatchInput] = useState('')
  const [showAddWatch, setShowAddWatch] = useState(false)
  const watchInputRef = useRef<HTMLInputElement>(null)

  // Section collapse state
  const [collapsedSections, setCollapsedSections] = useState<Set<Section>>(
    new Set()
  )

  // Keyboard shortcuts panel
  const [showShortcuts, setShowShortcuts] = useState(false)

  // -----------------------------------------------------------------------
  // Event subscriptions
  // -----------------------------------------------------------------------

  useEffect(() => {
    const unsubPaused = debugService.on('paused', (event: DebugEvent) => {
      setStatus('paused')
      setSession(debugService.getSession())
      // Fetch fresh data
      refreshPausedState()
    })

    const unsubResumed = debugService.on('resumed', () => {
      setStatus('running')
      setSession(debugService.getSession())
    })

    const unsubStarted = debugService.on('session_started', (event: DebugEvent) => {
      setSession(event.data.session)
      setStatus(event.data.session.status)
      addConsoleEntry('info', 'Debug session started')
    })

    const unsubStopped = debugService.on('session_stopped', () => {
      setSession(null)
      setStatus(null)
      setLocalVars([])
      setGlobalVars([])
      setCallStack([])
      addConsoleEntry('info', 'Debug session stopped')
    })

    const unsubOutput = debugService.on('output', (event: DebugEvent) => {
      addConsoleEntry('output', event.data.text || event.data.message || '')
    })

    const unsubError = debugService.on('error', (event: DebugEvent) => {
      addConsoleEntry('error', event.data.error || event.data.message || 'Unknown error')
    })

    const unsubBpAdded = debugService.on('breakpoint_added', () => {
      setBreakpoints(debugService.getBreakpoints())
    })

    const unsubBpRemoved = debugService.on('breakpoint_removed', () => {
      setBreakpoints(debugService.getBreakpoints())
    })

    const unsubException = debugService.on('exception', (event: DebugEvent) => {
      addConsoleEntry('error', `Exception: ${event.data.description || event.data.text || 'Unknown exception'}`)
    })

    return () => {
      unsubPaused()
      unsubResumed()
      unsubStarted()
      unsubStopped()
      unsubOutput()
      unsubError()
      unsubBpAdded()
      unsubBpRemoved()
      unsubException()
    }
  }, [])

  // -----------------------------------------------------------------------
  // Data fetching when paused
  // -----------------------------------------------------------------------

  const refreshPausedState = useCallback(async () => {
    try {
      const [locals, globals, stack] = await Promise.all([
        debugService.getVariables('local'),
        debugService.getVariables('global'),
        debugService.getCallStack(),
      ])
      setLocalVars(locals)
      setGlobalVars(globals)
      setCallStack(stack)
      if (stack.length > 0) {
        setActiveFrameId(stack[0].id)
      }
      // Refresh watches
      await debugService.evaluateAllWatches()
      setWatches([...debugService.getWatches()])
    } catch {
      // Silently handle -- session may have ended
    }
  }, [])

  // -----------------------------------------------------------------------
  // Console helpers
  // -----------------------------------------------------------------------

  const addConsoleEntry = useCallback(
    (type: ConsoleEntry['type'], content: string) => {
      setConsoleEntries((prev) => [
        ...prev,
        {
          id: `${Date.now()}-${Math.random()}`,
          type,
          content,
          timestamp: new Date().toISOString(),
        },
      ])
    },
    []
  )

  // Auto-scroll console
  useEffect(() => {
    consoleEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [consoleEntries])

  // -----------------------------------------------------------------------
  // Session controls
  // -----------------------------------------------------------------------

  const handleStart = useCallback(async () => {
    if (!projectId || !activeFilePath || !activeFileLanguage) return
    try {
      await debugService.startSession(projectId, activeFilePath, activeFileLanguage)
    } catch (err: any) {
      addConsoleEntry('error', `Failed to start: ${err.message}`)
    }
  }, [projectId, activeFilePath, activeFileLanguage, addConsoleEntry])

  const handleStop = useCallback(async () => {
    try {
      await debugService.stopSession()
    } catch (err: any) {
      addConsoleEntry('error', `Failed to stop: ${err.message}`)
    }
  }, [addConsoleEntry])

  const handleContinue = useCallback(async () => {
    await debugService.continue()
  }, [])

  const handlePause = useCallback(async () => {
    await debugService.pause()
  }, [])

  const handleStepOver = useCallback(async () => {
    await debugService.stepOver()
  }, [])

  const handleStepInto = useCallback(async () => {
    await debugService.stepInto()
  }, [])

  const handleStepOut = useCallback(async () => {
    await debugService.stepOut()
  }, [])

  const handleRestart = useCallback(async () => {
    await debugService.stopSession()
    if (projectId && activeFilePath && activeFileLanguage) {
      await debugService.startSession(projectId, activeFilePath, activeFileLanguage)
    }
  }, [projectId, activeFilePath, activeFileLanguage])

  // -----------------------------------------------------------------------
  // Console expression evaluation
  // -----------------------------------------------------------------------

  const handleEvaluateExpression = useCallback(async () => {
    const expr = consoleInput.trim()
    if (!expr) return

    addConsoleEntry('input', expr)
    setConsoleHistory((prev) => [expr, ...prev].slice(0, 100))
    setConsoleInput('')
    setHistoryIndex(-1)

    const result: EvaluateResult = await debugService.evaluate(expr)
    if (result.error) {
      addConsoleEntry('error', result.error)
    } else {
      addConsoleEntry('output', `${result.value}  // ${result.type}`)
    }
  }, [consoleInput, addConsoleEntry])

  const handleConsoleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') {
        handleEvaluateExpression()
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        if (consoleHistory.length > 0) {
          const newIndex = Math.min(historyIndex + 1, consoleHistory.length - 1)
          setHistoryIndex(newIndex)
          setConsoleInput(consoleHistory[newIndex])
        }
      } else if (e.key === 'ArrowDown') {
        e.preventDefault()
        if (historyIndex > 0) {
          const newIndex = historyIndex - 1
          setHistoryIndex(newIndex)
          setConsoleInput(consoleHistory[newIndex])
        } else {
          setHistoryIndex(-1)
          setConsoleInput('')
        }
      }
    },
    [handleEvaluateExpression, consoleHistory, historyIndex]
  )

  // -----------------------------------------------------------------------
  // Watch expressions
  // -----------------------------------------------------------------------

  const handleAddWatch = useCallback(() => {
    const expr = watchInput.trim()
    if (!expr) return
    debugService.addWatch(expr)
    setWatches([...debugService.getWatches()])
    setWatchInput('')
    setShowAddWatch(false)
  }, [watchInput])

  const handleRemoveWatch = useCallback((id: string) => {
    debugService.removeWatch(id)
    setWatches([...debugService.getWatches()])
  }, [])

  // -----------------------------------------------------------------------
  // Breakpoint management
  // -----------------------------------------------------------------------

  const handleToggleBreakpoint = useCallback(
    async (id: string, enabled: boolean) => {
      await debugService.toggleBreakpoint(id, enabled)
      setBreakpoints([...debugService.getBreakpoints()])
    },
    []
  )

  const handleRemoveBreakpoint = useCallback(async (id: string) => {
    await debugService.removeBreakpoint(id)
    setBreakpoints([...debugService.getBreakpoints()])
  }, [])

  const handleRemoveAllBreakpoints = useCallback(async () => {
    const bps = debugService.getBreakpoints()
    for (const bp of bps) {
      await debugService.removeBreakpoint(bp.id)
    }
    setBreakpoints([])
  }, [])

  // -----------------------------------------------------------------------
  // Frame selection -> navigate
  // -----------------------------------------------------------------------

  const handleFrameSelect = useCallback(
    (frame: StackFrame) => {
      setActiveFrameId(frame.id)
      onNavigateToSource?.(frame.file_path, frame.line)
    },
    [onNavigateToSource]
  )

  const handleBreakpointNavigate = useCallback(
    (filePath: string, line: number) => {
      onNavigateToSource?.(filePath, line)
    },
    [onNavigateToSource]
  )

  // -----------------------------------------------------------------------
  // Section toggle
  // -----------------------------------------------------------------------

  const toggleSection = useCallback((section: Section) => {
    setCollapsedSections((prev) => {
      const next = new Set(prev)
      if (next.has(section)) {
        next.delete(section)
      } else {
        next.add(section)
      }
      return next
    })
  }, [])

  // -----------------------------------------------------------------------
  // Keyboard shortcuts
  // -----------------------------------------------------------------------

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      // Only handle when debug panel is relevant
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return

      if (e.key === 'F5') {
        e.preventDefault()
        if (session && status === 'paused') {
          handleContinue()
        } else if (!session) {
          handleStart()
        }
      } else if (e.key === 'F5' && e.shiftKey) {
        e.preventDefault()
        handleStop()
      } else if (e.key === 'F10') {
        e.preventDefault()
        handleStepOver()
      } else if (e.key === 'F11') {
        e.preventDefault()
        if (e.shiftKey) {
          handleStepOut()
        } else {
          handleStepInto()
        }
      } else if (e.key === 'F6') {
        e.preventDefault()
        handlePause()
      }
    }

    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [session, status, handleContinue, handleStart, handleStop, handleStepOver, handleStepInto, handleStepOut, handlePause])

  // -----------------------------------------------------------------------
  // Derived state
  // -----------------------------------------------------------------------

  const isPaused = status === 'paused'
  const isRunning = status === 'running'
  const isActive = session !== null

  // -----------------------------------------------------------------------
  // Render
  // -----------------------------------------------------------------------

  return (
    <div
      className={cn(
        'flex flex-col h-full bg-gray-900/50 text-sm overflow-hidden',
        className
      )}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-800 bg-gray-900/80">
        <div className="flex items-center gap-2">
          <Bug className="w-4 h-4 text-red-400" />
          <span className="text-xs font-semibold text-gray-200 uppercase tracking-wider">
            Debugger
          </span>
          {statusBadge(status)}
        </div>

        <button
          onClick={() => setShowShortcuts((v) => !v)}
          className="text-gray-500 hover:text-gray-300 transition-colors"
          title="Keyboard shortcuts"
        >
          <Keyboard className="w-3.5 h-3.5" />
        </button>
      </div>

      {/* Keyboard shortcuts panel */}
      {showShortcuts && (
        <div className="px-3 py-2 bg-gray-800/80 border-b border-gray-700 text-[10px] text-gray-400 space-y-1">
          <div className="flex items-center justify-between">
            <span className="font-semibold text-gray-300 uppercase tracking-wider">Shortcuts</span>
            <button onClick={() => setShowShortcuts(false)} className="text-gray-500 hover:text-gray-300">
              <X className="w-3 h-3" />
            </button>
          </div>
          <div className="grid grid-cols-2 gap-x-4 gap-y-0.5">
            <span>Start / Continue</span>
            <kbd className="text-cyan-400 font-mono">F5</kbd>
            <span>Stop</span>
            <kbd className="text-cyan-400 font-mono">Shift+F5</kbd>
            <span>Pause</span>
            <kbd className="text-cyan-400 font-mono">F6</kbd>
            <span>Step Over</span>
            <kbd className="text-cyan-400 font-mono">F10</kbd>
            <span>Step Into</span>
            <kbd className="text-cyan-400 font-mono">F11</kbd>
            <span>Step Out</span>
            <kbd className="text-cyan-400 font-mono">Shift+F11</kbd>
          </div>
        </div>
      )}

      {/* Step controls toolbar */}
      <div className="flex items-center gap-1 px-3 py-1.5 border-b border-gray-800 bg-gray-900/60">
        {/* Start / Continue */}
        {!isActive ? (
          <button
            onClick={handleStart}
            disabled={!projectId || !activeFilePath}
            className="p-1.5 rounded hover:bg-green-500/20 text-green-400 hover:text-green-300 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
            title="Start debugging (F5)"
          >
            <Play className="w-4 h-4" />
          </button>
        ) : isPaused ? (
          <button
            onClick={handleContinue}
            className="p-1.5 rounded hover:bg-green-500/20 text-green-400 hover:text-green-300 transition-colors"
            title="Continue (F5)"
          >
            <Play className="w-4 h-4" />
          </button>
        ) : (
          <button
            onClick={handlePause}
            className="p-1.5 rounded hover:bg-yellow-500/20 text-yellow-400 hover:text-yellow-300 transition-colors"
            title="Pause (F6)"
          >
            <Pause className="w-4 h-4" />
          </button>
        )}

        {/* Step Over */}
        <button
          onClick={handleStepOver}
          disabled={!isPaused}
          className="p-1.5 rounded hover:bg-cyan-500/20 text-cyan-400 hover:text-cyan-300 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          title="Step Over (F10)"
        >
          <SkipForward className="w-4 h-4" />
        </button>

        {/* Step Into */}
        <button
          onClick={handleStepInto}
          disabled={!isPaused}
          className="p-1.5 rounded hover:bg-cyan-500/20 text-cyan-400 hover:text-cyan-300 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          title="Step Into (F11)"
        >
          <ArrowDownToLine className="w-4 h-4" />
        </button>

        {/* Step Out */}
        <button
          onClick={handleStepOut}
          disabled={!isPaused}
          className="p-1.5 rounded hover:bg-cyan-500/20 text-cyan-400 hover:text-cyan-300 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          title="Step Out (Shift+F11)"
        >
          <ArrowUpFromLine className="w-4 h-4" />
        </button>

        <div className="w-px h-5 bg-gray-700 mx-1" />

        {/* Restart */}
        <button
          onClick={handleRestart}
          disabled={!isActive}
          className="p-1.5 rounded hover:bg-orange-500/20 text-orange-400 hover:text-orange-300 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          title="Restart"
        >
          <RotateCcw className="w-4 h-4" />
        </button>

        {/* Stop */}
        <button
          onClick={handleStop}
          disabled={!isActive}
          className="p-1.5 rounded hover:bg-red-500/20 text-red-400 hover:text-red-300 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          title="Stop (Shift+F5)"
        >
          <Square className="w-4 h-4" />
        </button>

        {/* Current location indicator */}
        {isPaused && session?.current_file && (
          <div className="ml-auto flex items-center gap-1 text-[10px] text-gray-500 truncate max-w-[140px]">
            <span className="font-mono truncate">{session.current_file}</span>
            <span className="text-gray-600">:</span>
            <span className="text-cyan-400 font-mono">{session.current_line}</span>
          </div>
        )}
      </div>

      {/* Scrollable sections */}
      <div className="flex-1 overflow-y-auto scrollbar-thin scrollbar-thumb-gray-700 scrollbar-track-transparent">
        {/* Variables section */}
        <div className="border-b border-gray-800">
          <button
            onClick={() => toggleSection('variables')}
            className="flex items-center gap-1 w-full px-3 py-1.5 text-xs font-semibold text-gray-300 hover:bg-gray-800/50 uppercase tracking-wider"
          >
            {collapsedSections.has('variables') ? (
              <ChevronRight className="w-3 h-3" />
            ) : (
              <ChevronDown className="w-3 h-3" />
            )}
            <VariableIcon className="w-3 h-3 text-purple-400" />
            Variables
          </button>
          {!collapsedSections.has('variables') && (
            <VariablesView
              localVariables={localVars}
              globalVariables={globalVars}
              isPaused={isPaused}
            />
          )}
        </div>

        {/* Watch section */}
        <div className="border-b border-gray-800">
          <div className="flex items-center">
            <button
              onClick={() => toggleSection('watch')}
              className="flex items-center gap-1 flex-1 px-3 py-1.5 text-xs font-semibold text-gray-300 hover:bg-gray-800/50 uppercase tracking-wider"
            >
              {collapsedSections.has('watch') ? (
                <ChevronRight className="w-3 h-3" />
              ) : (
                <ChevronDown className="w-3 h-3" />
              )}
              <Eye className="w-3 h-3 text-yellow-400" />
              Watch
              <span className="text-gray-600 normal-case font-normal ml-1">
                {watches.length}
              </span>
            </button>
            <button
              onClick={() => {
                setShowAddWatch(true)
                if (collapsedSections.has('watch')) {
                  toggleSection('watch')
                }
                setTimeout(() => watchInputRef.current?.focus(), 100)
              }}
              className="px-2 text-gray-500 hover:text-cyan-400 transition-colors"
              title="Add watch expression"
            >
              <Plus className="w-3.5 h-3.5" />
            </button>
          </div>

          {!collapsedSections.has('watch') && (
            <div className="pb-1">
              {/* Add watch input */}
              {showAddWatch && (
                <div className="flex items-center gap-1 px-3 py-1">
                  <input
                    ref={watchInputRef}
                    value={watchInput}
                    onChange={(e) => setWatchInput(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleAddWatch()
                      if (e.key === 'Escape') {
                        setShowAddWatch(false)
                        setWatchInput('')
                      }
                    }}
                    placeholder="Expression to watch..."
                    className="flex-1 bg-gray-900 border border-gray-700 focus:border-cyan-500/50 rounded px-2 py-0.5 text-xs text-white font-mono focus:outline-none"
                  />
                  <button
                    onClick={handleAddWatch}
                    className="text-green-400 hover:text-green-300 p-1"
                  >
                    <Plus className="w-3 h-3" />
                  </button>
                  <button
                    onClick={() => {
                      setShowAddWatch(false)
                      setWatchInput('')
                    }}
                    className="text-gray-500 hover:text-gray-300 p-1"
                  >
                    <X className="w-3 h-3" />
                  </button>
                </div>
              )}

              {/* Watch list */}
              {watches.length === 0 && !showAddWatch ? (
                <div className="px-4 py-2 text-xs text-gray-600 italic">
                  No watch expressions
                </div>
              ) : (
                watches.map((w) => (
                  <div
                    key={w.id}
                    className="flex items-center gap-1 px-3 py-0.5 text-xs hover:bg-gray-800/60 group"
                  >
                    <Eye className="w-3 h-3 text-yellow-400/50 flex-shrink-0" />
                    <span className="text-blue-300 font-mono truncate flex-shrink-0 max-w-[100px]">
                      {w.expression}
                    </span>
                    <span className="text-gray-600 flex-shrink-0">=</span>
                    {w.error ? (
                      <span className="text-red-400 truncate flex-1 min-w-0 font-mono">
                        {w.error}
                      </span>
                    ) : (
                      <span className="text-green-400 truncate flex-1 min-w-0 font-mono">
                        {w.value}
                      </span>
                    )}
                    {w.type && !w.error && (
                      <span className="text-[10px] text-gray-600 font-mono flex-shrink-0">
                        {w.type}
                      </span>
                    )}
                    <button
                      onClick={() => handleRemoveWatch(w.id)}
                      className="flex-shrink-0 text-gray-600 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-opacity"
                    >
                      <Trash2 className="w-3 h-3" />
                    </button>
                  </div>
                ))
              )}
            </div>
          )}
        </div>

        {/* Call Stack section */}
        <div className="border-b border-gray-800">
          <button
            onClick={() => toggleSection('callstack')}
            className="flex items-center gap-1 w-full px-3 py-1.5 text-xs font-semibold text-gray-300 hover:bg-gray-800/50 uppercase tracking-wider"
          >
            {collapsedSections.has('callstack') ? (
              <ChevronRight className="w-3 h-3" />
            ) : (
              <ChevronDown className="w-3 h-3" />
            )}
            <Layers className="w-3 h-3 text-cyan-400" />
            Call Stack
            {callStack.length > 0 && (
              <span className="text-gray-600 normal-case font-normal ml-1">
                {callStack.length}
              </span>
            )}
          </button>
          {!collapsedSections.has('callstack') && (
            <CallStackView
              frames={callStack}
              activeFrameId={activeFrameId}
              isPaused={isPaused}
              onFrameSelect={handleFrameSelect}
            />
          )}
        </div>

        {/* Breakpoints section */}
        <div className="border-b border-gray-800">
          <button
            onClick={() => toggleSection('breakpoints')}
            className="flex items-center gap-1 w-full px-3 py-1.5 text-xs font-semibold text-gray-300 hover:bg-gray-800/50 uppercase tracking-wider"
          >
            {collapsedSections.has('breakpoints') ? (
              <ChevronRight className="w-3 h-3" />
            ) : (
              <ChevronDown className="w-3 h-3" />
            )}
            <CircleDot className="w-3 h-3 text-red-400" />
            Breakpoints
            {breakpoints.length > 0 && (
              <span className="text-gray-600 normal-case font-normal ml-1">
                {breakpoints.length}
              </span>
            )}
          </button>
          {!collapsedSections.has('breakpoints') && (
            <BreakpointManager
              breakpoints={breakpoints}
              onToggle={handleToggleBreakpoint}
              onRemove={handleRemoveBreakpoint}
              onRemoveAll={handleRemoveAllBreakpoints}
              onNavigate={handleBreakpointNavigate}
            />
          )}
        </div>
      </div>

      {/* Debug Console */}
      <div className="border-t border-gray-800 flex flex-col min-h-[120px] max-h-[200px]">
        {/* Console header */}
        <div className="flex items-center gap-2 px-3 py-1 bg-gray-900/80 border-b border-gray-800">
          <Terminal className="w-3 h-3 text-gray-400" />
          <span className="text-[10px] text-gray-500 uppercase tracking-wider font-semibold">
            Debug Console
          </span>
          <button
            onClick={() => setConsoleEntries([])}
            className="ml-auto text-gray-600 hover:text-gray-400 transition-colors"
            title="Clear console"
          >
            <Trash2 className="w-3 h-3" />
          </button>
        </div>

        {/* Console output */}
        <div className="flex-1 overflow-y-auto px-3 py-1 font-mono text-xs space-y-0.5 scrollbar-thin scrollbar-thumb-gray-700 scrollbar-track-transparent">
          {consoleEntries.length === 0 && (
            <div className="text-gray-600 italic py-1">
              Evaluate expressions when paused...
            </div>
          )}
          {consoleEntries.map((entry) => (
            <div
              key={entry.id}
              className={cn(
                'flex items-start gap-1',
                entry.type === 'input' && 'text-blue-300',
                entry.type === 'output' && 'text-gray-300',
                entry.type === 'error' && 'text-red-400',
                entry.type === 'info' && 'text-gray-500'
              )}
            >
              {entry.type === 'input' && (
                <span className="text-cyan-500 flex-shrink-0">&gt;</span>
              )}
              {entry.type === 'error' && (
                <AlertCircle className="w-3 h-3 flex-shrink-0 mt-0.5" />
              )}
              <span className="break-all">{entry.content}</span>
            </div>
          ))}
          <div ref={consoleEndRef} />
        </div>

        {/* Console input */}
        <div className="flex items-center gap-2 px-3 py-1.5 border-t border-gray-800 bg-gray-900/60">
          <span className="text-cyan-500 text-xs font-mono">&gt;</span>
          <input
            ref={consoleInputRef}
            value={consoleInput}
            onChange={(e) => setConsoleInput(e.target.value)}
            onKeyDown={handleConsoleKeyDown}
            placeholder={isPaused ? 'Evaluate expression...' : 'Start debug session to evaluate'}
            disabled={!isActive}
            className="flex-1 bg-transparent text-xs text-white font-mono focus:outline-none placeholder-gray-600 disabled:opacity-40"
          />
          <button
            onClick={handleEvaluateExpression}
            disabled={!isActive || !consoleInput.trim()}
            className="text-gray-500 hover:text-cyan-400 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >
            <Send className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>
  )
}

export { DebugPanel }
export default DebugPanel
