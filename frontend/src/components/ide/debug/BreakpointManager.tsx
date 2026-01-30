// APEX.BUILD Breakpoint Manager
// Lists all breakpoints across files with enable/disable toggle,
// conditional expression editor, remove actions, and "Remove All"

import React, { useState, useCallback, useRef, useEffect } from 'react'
import { cn } from '@/lib/utils'
import debugService, { Breakpoint, BreakpointType } from '@/services/debugService'
import {
  Circle,
  CircleDot,
  Trash2,
  FileCode,
  ChevronDown,
  ChevronRight,
  Edit3,
  Check,
  X,
  XCircle,
  AlertTriangle,
  MessageSquare,
  Zap,
} from 'lucide-react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface BreakpointManagerProps {
  className?: string
  breakpoints: Breakpoint[]
  onToggle?: (id: string, enabled: boolean) => void
  onRemove?: (id: string) => void
  onRemoveAll?: () => void
  onNavigate?: (filePath: string, line: number) => void
  onConditionChange?: (id: string, condition: string) => void
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function breakpointTypeIcon(type: BreakpointType) {
  switch (type) {
    case 'conditional':
      return <AlertTriangle className="w-3 h-3 text-yellow-400" />
    case 'logpoint':
      return <MessageSquare className="w-3 h-3 text-blue-400" />
    case 'exception':
      return <Zap className="w-3 h-3 text-red-400" />
    case 'function':
      return <Zap className="w-3 h-3 text-purple-400" />
    default:
      return null
  }
}

function fileName(path: string): string {
  const parts = path.split('/')
  return parts[parts.length - 1] || path
}

// ---------------------------------------------------------------------------
// BreakpointRow
// ---------------------------------------------------------------------------

interface BreakpointRowProps {
  breakpoint: Breakpoint
  onToggle?: (id: string, enabled: boolean) => void
  onRemove?: (id: string) => void
  onNavigate?: (filePath: string, line: number) => void
  onConditionChange?: (id: string, condition: string) => void
}

const BreakpointRow: React.FC<BreakpointRowProps> = ({
  breakpoint,
  onToggle,
  onRemove,
  onNavigate,
  onConditionChange,
}) => {
  const [editingCondition, setEditingCondition] = useState(false)
  const [conditionValue, setConditionValue] = useState(breakpoint.condition || '')
  const [showDetails, setShowDetails] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (editingCondition && inputRef.current) {
      inputRef.current.focus()
    }
  }, [editingCondition])

  const handleToggle = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation()
      onToggle?.(breakpoint.id, !breakpoint.enabled)
    },
    [breakpoint, onToggle]
  )

  const handleRemove = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation()
      onRemove?.(breakpoint.id)
    },
    [breakpoint.id, onRemove]
  )

  const handleNavigate = useCallback(() => {
    onNavigate?.(breakpoint.file_path, breakpoint.line)
  }, [breakpoint, onNavigate])

  const handleSaveCondition = useCallback(() => {
    setEditingCondition(false)
    onConditionChange?.(breakpoint.id, conditionValue)
  }, [breakpoint.id, conditionValue, onConditionChange])

  const handleCancelCondition = useCallback(() => {
    setEditingCondition(false)
    setConditionValue(breakpoint.condition || '')
  }, [breakpoint.condition])

  const handleConditionKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') handleSaveCondition()
      if (e.key === 'Escape') handleCancelCondition()
    },
    [handleSaveCondition, handleCancelCondition]
  )

  return (
    <div className="border-b border-gray-800/50 last:border-b-0">
      <div
        className={cn(
          'flex items-center gap-2 px-3 py-1.5 text-xs hover:bg-gray-800/60 transition-colors cursor-pointer group',
          !breakpoint.enabled && 'opacity-50'
        )}
        onClick={handleNavigate}
      >
        {/* Enable / disable toggle */}
        <button
          onClick={handleToggle}
          className={cn(
            'flex-shrink-0 transition-colors',
            breakpoint.enabled
              ? 'text-red-400 hover:text-red-300'
              : 'text-gray-600 hover:text-gray-400'
          )}
          title={breakpoint.enabled ? 'Disable breakpoint' : 'Enable breakpoint'}
        >
          {breakpoint.enabled ? (
            <CircleDot className="w-3.5 h-3.5" />
          ) : (
            <Circle className="w-3.5 h-3.5" />
          )}
        </button>

        {/* Type icon */}
        {breakpointTypeIcon(breakpoint.type)}

        {/* File name and line */}
        <div className="flex items-center gap-1 flex-1 min-w-0">
          <FileCode className="w-3 h-3 text-gray-500 flex-shrink-0" />
          <span className="text-gray-300 truncate font-mono">
            {fileName(breakpoint.file_path)}
          </span>
          <span className="text-gray-600 flex-shrink-0">:</span>
          <span className="text-cyan-400 font-mono flex-shrink-0">
            {breakpoint.line}
          </span>
        </div>

        {/* Hit count */}
        {breakpoint.hit_count > 0 && (
          <span className="flex-shrink-0 text-[10px] text-gray-500 font-mono bg-gray-800 px-1 rounded">
            {breakpoint.hit_count}x
          </span>
        )}

        {/* Verified indicator */}
        {!breakpoint.verified && (
          <span className="flex-shrink-0 w-1.5 h-1.5 rounded-full bg-yellow-500" title="Unverified" />
        )}

        {/* Detail toggle */}
        <button
          onClick={(e) => {
            e.stopPropagation()
            setShowDetails((v) => !v)
          }}
          className="flex-shrink-0 text-gray-600 hover:text-gray-400 opacity-0 group-hover:opacity-100 transition-opacity"
          title="Edit condition"
        >
          <Edit3 className="w-3 h-3" />
        </button>

        {/* Remove button */}
        <button
          onClick={handleRemove}
          className="flex-shrink-0 text-gray-600 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-opacity"
          title="Remove breakpoint"
        >
          <Trash2 className="w-3 h-3" />
        </button>
      </div>

      {/* Condition editor (expanded) */}
      {showDetails && (
        <div className="px-3 pb-2 pt-0.5">
          <div className="flex items-center gap-1 text-[10px] text-gray-500 mb-1 uppercase tracking-wider">
            Condition expression
          </div>
          {editingCondition ? (
            <div className="flex items-center gap-1">
              <input
                ref={inputRef}
                value={conditionValue}
                onChange={(e) => setConditionValue(e.target.value)}
                onKeyDown={handleConditionKeyDown}
                placeholder="e.g. i > 5 && x !== null"
                className="flex-1 bg-gray-900 border border-cyan-500/50 rounded px-2 py-1 text-xs text-white font-mono focus:outline-none focus:border-cyan-400"
              />
              <button
                onClick={handleSaveCondition}
                className="text-green-400 hover:text-green-300 p-1"
              >
                <Check className="w-3 h-3" />
              </button>
              <button
                onClick={handleCancelCondition}
                className="text-red-400 hover:text-red-300 p-1"
              >
                <X className="w-3 h-3" />
              </button>
            </div>
          ) : (
            <button
              onClick={() => setEditingCondition(true)}
              className={cn(
                'w-full text-left px-2 py-1 rounded text-xs font-mono border transition-colors',
                breakpoint.condition
                  ? 'text-yellow-300 bg-yellow-500/10 border-yellow-500/30 hover:border-yellow-400/50'
                  : 'text-gray-600 bg-gray-900/50 border-gray-800 hover:border-gray-700'
              )}
            >
              {breakpoint.condition || 'Click to add condition...'}
            </button>
          )}

          {/* Log message for logpoints */}
          {breakpoint.type === 'logpoint' && breakpoint.log_message && (
            <div className="mt-1.5 px-2 py-1 bg-blue-500/10 border border-blue-500/20 rounded text-xs text-blue-300 font-mono">
              {breakpoint.log_message}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// BreakpointManager
// ---------------------------------------------------------------------------

const BreakpointManager: React.FC<BreakpointManagerProps> = ({
  className,
  breakpoints,
  onToggle,
  onRemove,
  onRemoveAll,
  onNavigate,
  onConditionChange,
}) => {
  // Group breakpoints by file
  const grouped = breakpoints.reduce<Record<string, Breakpoint[]>>(
    (acc, bp) => {
      const key = bp.file_path
      if (!acc[key]) acc[key] = []
      acc[key].push(bp)
      return acc
    },
    {}
  )

  const [collapsedFiles, setCollapsedFiles] = useState<Set<string>>(new Set())

  const toggleFile = useCallback((file: string) => {
    setCollapsedFiles((prev) => {
      const next = new Set(prev)
      if (next.has(file)) {
        next.delete(file)
      } else {
        next.add(file)
      }
      return next
    })
  }, [])

  if (breakpoints.length === 0) {
    return (
      <div className={cn('flex flex-col', className)}>
        <div className="flex items-center justify-center gap-2 px-3 py-6 text-xs text-gray-600">
          <Circle className="w-4 h-4" />
          <span>No breakpoints set</span>
        </div>
      </div>
    )
  }

  return (
    <div className={cn('flex flex-col', className)}>
      {/* Header with Remove All */}
      <div className="flex items-center justify-between px-3 py-1.5 border-b border-gray-800">
        <span className="text-[10px] text-gray-500 uppercase tracking-wider font-semibold">
          Breakpoints ({breakpoints.length})
        </span>
        <button
          onClick={onRemoveAll}
          className="flex items-center gap-1 text-[10px] text-gray-500 hover:text-red-400 transition-colors"
          title="Remove all breakpoints"
        >
          <XCircle className="w-3 h-3" />
          Remove All
        </button>
      </div>

      {/* Grouped by file */}
      {Object.entries(grouped).map(([filePath, fileBps]) => {
        const collapsed = collapsedFiles.has(filePath)
        return (
          <div key={filePath}>
            {/* File header */}
            <button
              onClick={() => toggleFile(filePath)}
              className="flex items-center gap-1 w-full px-3 py-1 text-[10px] text-gray-400 hover:bg-gray-800/40 transition-colors"
            >
              {collapsed ? (
                <ChevronRight className="w-3 h-3" />
              ) : (
                <ChevronDown className="w-3 h-3" />
              )}
              <FileCode className="w-3 h-3" />
              <span className="truncate font-mono">{fileName(filePath)}</span>
              <span className="ml-auto text-gray-600">{fileBps.length}</span>
            </button>

            {/* Breakpoints in file */}
            {!collapsed &&
              fileBps
                .sort((a, b) => a.line - b.line)
                .map((bp) => (
                  <BreakpointRow
                    key={bp.id}
                    breakpoint={bp}
                    onToggle={onToggle}
                    onRemove={onRemove}
                    onNavigate={onNavigate}
                    onConditionChange={onConditionChange}
                  />
                ))}
          </div>
        )
      })}
    </div>
  )
}

export { BreakpointManager }
export default BreakpointManager
