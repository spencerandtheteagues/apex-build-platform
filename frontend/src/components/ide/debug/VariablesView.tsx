// APEX.BUILD Debug Variables View
// Tree view of local and global variables with expandable objects,
// type indicators, and inline primitive editing

import React, { useState, useCallback, useRef, useEffect } from 'react'
import { cn } from '@/lib/utils'
import debugService, {
  Variable,
} from '@/services/debugService'
import {
  ChevronRight,
  ChevronDown,
  Hash,
  Type,
  ToggleLeft,
  Braces,
  List,
  Circle,
  Pencil,
  Check,
  X,
} from 'lucide-react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface VariablesViewProps {
  className?: string
  localVariables: Variable[]
  globalVariables: Variable[]
  isPaused: boolean
  onVariableExpand?: (objectId: string) => Promise<Variable[]>
}

interface VariableRowProps {
  variable: Variable
  depth: number
  isPaused: boolean
  onExpand?: (objectId: string) => Promise<Variable[]>
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const TYPE_CONFIG: Record<string, { icon: React.ReactNode; color: string }> = {
  string: {
    icon: <Type className="w-3 h-3" />,
    color: 'text-green-400',
  },
  number: {
    icon: <Hash className="w-3 h-3" />,
    color: 'text-cyan-400',
  },
  boolean: {
    icon: <ToggleLeft className="w-3 h-3" />,
    color: 'text-yellow-400',
  },
  object: {
    icon: <Braces className="w-3 h-3" />,
    color: 'text-purple-400',
  },
  array: {
    icon: <List className="w-3 h-3" />,
    color: 'text-pink-400',
  },
  undefined: {
    icon: <Circle className="w-3 h-3" />,
    color: 'text-gray-500',
  },
  null: {
    icon: <Circle className="w-3 h-3" />,
    color: 'text-gray-500',
  },
  function: {
    icon: <Braces className="w-3 h-3" />,
    color: 'text-orange-400',
  },
}

function getTypeConfig(type: string) {
  return TYPE_CONFIG[type.toLowerCase()] ?? TYPE_CONFIG['undefined']
}

function formatValue(variable: Variable): string {
  if (variable.preview) return variable.preview
  if (variable.type === 'string') return `"${variable.value}"`
  return variable.value
}

function isPrimitive(type: string): boolean {
  const t = type.toLowerCase()
  return t === 'string' || t === 'number' || t === 'boolean'
}

// ---------------------------------------------------------------------------
// VariableRow -- single row with expand / collapse and inline editing
// ---------------------------------------------------------------------------

const VariableRow: React.FC<VariableRowProps> = ({
  variable,
  depth,
  isPaused,
  onExpand,
}) => {
  const [expanded, setExpanded] = useState(false)
  const [children, setChildren] = useState<Variable[]>(variable.children ?? [])
  const [loading, setLoading] = useState(false)
  const [editing, setEditing] = useState(false)
  const [editValue, setEditValue] = useState(variable.value)
  const inputRef = useRef<HTMLInputElement>(null)

  const typeConfig = getTypeConfig(variable.type)

  const handleToggle = useCallback(async () => {
    if (!variable.has_children) return

    if (!expanded && children.length === 0 && variable.object_id && onExpand) {
      setLoading(true)
      try {
        const loaded = await onExpand(variable.object_id)
        setChildren(loaded)
      } catch {
        // silently ignore -- will show empty
      } finally {
        setLoading(false)
      }
    }
    setExpanded((prev) => !prev)
  }, [expanded, children.length, variable, onExpand])

  const handleStartEdit = useCallback(() => {
    if (!isPaused || !isPrimitive(variable.type)) return
    setEditValue(variable.value)
    setEditing(true)
  }, [isPaused, variable])

  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus()
      inputRef.current.select()
    }
  }, [editing])

  const handleConfirmEdit = useCallback(async () => {
    setEditing(false)
    // Evaluate the assignment expression to set the value
    const expression = isPrimitive(variable.type) && variable.type === 'string'
      ? `${variable.name} = "${editValue}"`
      : `${variable.name} = ${editValue}`
    await debugService.evaluate(expression)
  }, [editValue, variable])

  const handleCancelEdit = useCallback(() => {
    setEditing(false)
    setEditValue(variable.value)
  }, [variable.value])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') handleConfirmEdit()
      if (e.key === 'Escape') handleCancelEdit()
    },
    [handleConfirmEdit, handleCancelEdit]
  )

  return (
    <>
      <div
        className={cn(
          'flex items-center gap-1 py-0.5 px-2 hover:bg-gray-800/60 rounded cursor-default group text-xs',
          editing && 'bg-gray-800/80'
        )}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
      >
        {/* Expand / collapse toggle */}
        <button
          onClick={handleToggle}
          className={cn(
            'flex-shrink-0 w-4 h-4 flex items-center justify-center',
            variable.has_children
              ? 'text-gray-400 hover:text-white'
              : 'invisible'
          )}
        >
          {loading ? (
            <div className="w-3 h-3 border border-gray-400 border-t-transparent rounded-full animate-spin" />
          ) : expanded ? (
            <ChevronDown className="w-3 h-3" />
          ) : (
            <ChevronRight className="w-3 h-3" />
          )}
        </button>

        {/* Type icon */}
        <span className={cn('flex-shrink-0', typeConfig.color)}>
          {typeConfig.icon}
        </span>

        {/* Variable name */}
        <span className="text-blue-300 font-medium truncate flex-shrink-0 max-w-[120px]">
          {variable.name}
        </span>

        <span className="text-gray-600 flex-shrink-0">:</span>

        {/* Value -- either editable input or display */}
        {editing ? (
          <div className="flex items-center gap-1 flex-1 min-w-0">
            <input
              ref={inputRef}
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              onKeyDown={handleKeyDown}
              className="flex-1 min-w-0 bg-gray-900 border border-cyan-500/50 rounded px-1 py-0 text-xs text-white focus:outline-none focus:border-cyan-400"
            />
            <button
              onClick={handleConfirmEdit}
              className="text-green-400 hover:text-green-300"
            >
              <Check className="w-3 h-3" />
            </button>
            <button
              onClick={handleCancelEdit}
              className="text-red-400 hover:text-red-300"
            >
              <X className="w-3 h-3" />
            </button>
          </div>
        ) : (
          <span
            className={cn(
              'truncate flex-1 min-w-0',
              typeConfig.color,
              isPrimitive(variable.type) && isPaused && 'cursor-pointer'
            )}
            onDoubleClick={handleStartEdit}
            title={formatValue(variable)}
          >
            {formatValue(variable)}
          </span>
        )}

        {/* Edit button on hover for primitives */}
        {!editing && isPrimitive(variable.type) && isPaused && (
          <button
            onClick={handleStartEdit}
            className="flex-shrink-0 opacity-0 group-hover:opacity-100 text-gray-500 hover:text-white transition-opacity"
            title="Edit value"
          >
            <Pencil className="w-3 h-3" />
          </button>
        )}

        {/* Type badge */}
        <span className="flex-shrink-0 text-[10px] text-gray-600 ml-1 font-mono">
          {variable.type}
        </span>
      </div>

      {/* Children (expanded) */}
      {expanded &&
        children.map((child, idx) => (
          <VariableRow
            key={`${child.name}-${idx}`}
            variable={child}
            depth={depth + 1}
            isPaused={isPaused}
            onExpand={onExpand}
          />
        ))}
    </>
  )
}

// ---------------------------------------------------------------------------
// VariablesView -- main exported component
// ---------------------------------------------------------------------------

const VariablesView: React.FC<VariablesViewProps> = ({
  className,
  localVariables,
  globalVariables,
  isPaused,
  onVariableExpand,
}) => {
  const [showLocals, setShowLocals] = useState(true)
  const [showGlobals, setShowGlobals] = useState(false)

  const handleExpand = useCallback(
    async (objectId: string): Promise<Variable[]> => {
      if (onVariableExpand) return onVariableExpand(objectId)
      return debugService.expandVariable(objectId)
    },
    [onVariableExpand]
  )

  return (
    <div className={cn('flex flex-col text-sm', className)}>
      {/* Local variables section */}
      <button
        onClick={() => setShowLocals((v) => !v)}
        className="flex items-center gap-1 px-2 py-1.5 text-xs font-semibold text-gray-300 hover:bg-gray-800/50 uppercase tracking-wider"
      >
        {showLocals ? (
          <ChevronDown className="w-3 h-3" />
        ) : (
          <ChevronRight className="w-3 h-3" />
        )}
        Local
        <span className="ml-auto text-gray-600 normal-case font-normal">
          {localVariables.length}
        </span>
      </button>

      {showLocals && (
        <div className="pb-1">
          {localVariables.length === 0 ? (
            <div className="px-4 py-2 text-xs text-gray-600 italic">
              {isPaused ? 'No local variables' : 'Not paused'}
            </div>
          ) : (
            localVariables.map((v, idx) => (
              <VariableRow
                key={`local-${v.name}-${idx}`}
                variable={v}
                depth={0}
                isPaused={isPaused}
                onExpand={handleExpand}
              />
            ))
          )}
        </div>
      )}

      {/* Global variables section */}
      <button
        onClick={() => setShowGlobals((v) => !v)}
        className="flex items-center gap-1 px-2 py-1.5 text-xs font-semibold text-gray-300 hover:bg-gray-800/50 uppercase tracking-wider border-t border-gray-800"
      >
        {showGlobals ? (
          <ChevronDown className="w-3 h-3" />
        ) : (
          <ChevronRight className="w-3 h-3" />
        )}
        Global
        <span className="ml-auto text-gray-600 normal-case font-normal">
          {globalVariables.length}
        </span>
      </button>

      {showGlobals && (
        <div className="pb-1">
          {globalVariables.length === 0 ? (
            <div className="px-4 py-2 text-xs text-gray-600 italic">
              {isPaused ? 'No global variables' : 'Not paused'}
            </div>
          ) : (
            globalVariables.map((v, idx) => (
              <VariableRow
                key={`global-${v.name}-${idx}`}
                variable={v}
                depth={0}
                isPaused={isPaused}
                onExpand={handleExpand}
              />
            ))
          )}
        </div>
      )}
    </div>
  )
}

export { VariablesView }
export default VariablesView
