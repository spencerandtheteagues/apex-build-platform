// APEX.BUILD Multiplayer Cursors Component
// Real-time cursor visualization for Monaco editor collaboration

import React, { useEffect, useRef, useState, useMemo, useCallback } from 'react'
import * as monaco from 'monaco-editor'
import { RemoteCursor } from '@/hooks/useCollaboration'
import { cn } from '@/lib/utils'

export interface MultiplayerCursorsProps {
  editor: monaco.editor.IStandaloneCodeEditor | null
  cursors: RemoteCursor[]
  fileId?: number
  className?: string
}

// Cursor decoration IDs by user
interface CursorDecorations {
  cursorId: string[]
  selectionId: string[]
  labelElement?: HTMLDivElement
}

// Create CSS class name for cursor color
const getCursorClassName = (color: string): string => {
  // Convert hex color to valid CSS class name
  return `collab-cursor-${color.replace('#', '')}`
}

// Create CSS class name for selection color
const getSelectionClassName = (color: string): string => {
  return `collab-selection-${color.replace('#', '')}`
}

// Inject cursor styles dynamically
const injectCursorStyles = (color: string) => {
  const cursorClass = getCursorClassName(color)
  const selectionClass = getSelectionClassName(color)

  // Check if style already exists
  if (document.getElementById(`cursor-style-${cursorClass}`)) {
    return
  }

  const style = document.createElement('style')
  style.id = `cursor-style-${cursorClass}`
  style.textContent = `
    .${cursorClass} {
      background-color: ${color} !important;
      width: 2px !important;
      animation: cursor-blink 1s ease-in-out infinite;
    }
    .${selectionClass} {
      background-color: ${color}30 !important;
      border-left: 2px solid ${color};
    }
    @keyframes cursor-blink {
      0%, 50% { opacity: 1; }
      51%, 100% { opacity: 0.4; }
    }
  `
  document.head.appendChild(style)
}

// Cursor label component
interface CursorLabelProps {
  username: string
  color: string
  isTyping: boolean
  top: number
  left: number
}

const CursorLabel: React.FC<CursorLabelProps> = ({ username, color, isTyping, top, left }) => {
  return (
    <div
      className="absolute z-50 pointer-events-none transition-all duration-100 ease-out"
      style={{
        top: `${top}px`,
        left: `${left}px`,
        transform: 'translateY(-100%)',
      }}
    >
      <div
        className="flex items-center gap-1 px-2 py-0.5 rounded-t-md text-xs font-medium whitespace-nowrap shadow-lg"
        style={{
          backgroundColor: color,
          color: getContrastColor(color),
        }}
      >
        <span>{username}</span>
        {isTyping && (
          <span className="flex gap-0.5">
            <span className="w-1 h-1 bg-current rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
            <span className="w-1 h-1 bg-current rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
            <span className="w-1 h-1 bg-current rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
          </span>
        )}
      </div>
      <div
        className="w-0.5 h-3"
        style={{ backgroundColor: color }}
      />
    </div>
  )
}

// Get contrasting text color for background
function getContrastColor(hexColor: string): string {
  const hex = hexColor.replace('#', '')
  const r = parseInt(hex.substr(0, 2), 16)
  const g = parseInt(hex.substr(2, 2), 16)
  const b = parseInt(hex.substr(4, 2), 16)
  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  return luminance > 0.5 ? '#000000' : '#ffffff'
}

export const MultiplayerCursors: React.FC<MultiplayerCursorsProps> = ({
  editor,
  cursors,
  fileId,
  className,
}) => {
  const [cursorLabels, setCursorLabels] = useState<Map<number, { top: number; left: number }>>(new Map())
  const decorationsRef = useRef<Map<number, CursorDecorations>>(new Map())
  const editorContainerRef = useRef<HTMLDivElement | null>(null)

  // Filter cursors for current file
  const fileCursors = useMemo(() => {
    if (!fileId) return cursors
    return cursors.filter(c => c.fileId === fileId)
  }, [cursors, fileId])

  // Update cursor label positions
  const updateLabelPositions = useCallback(() => {
    if (!editor) return

    const newLabels = new Map<number, { top: number; left: number }>()

    fileCursors.forEach(cursor => {
      if (!cursor.cursor) return

      try {
        // Get the position in the editor coordinate system
        const position = new monaco.Position(cursor.cursor.line, cursor.cursor.column)
        const coordinates = editor.getScrolledVisiblePosition(position)

        if (coordinates) {
          // Get editor DOM node for offset calculation
          const editorDom = editor.getDomNode()
          if (editorDom) {
            const editorRect = editorDom.getBoundingClientRect()
            newLabels.set(cursor.userId, {
              top: coordinates.top + editorRect.top,
              left: coordinates.left + editorRect.left,
            })
          }
        }
      } catch (e) {
        // Position might be invalid, skip
      }
    })

    setCursorLabels(newLabels)
  }, [editor, fileCursors])

  // Update decorations when cursors change
  useEffect(() => {
    if (!editor) return

    const model = editor.getModel()
    if (!model) return

    // Collect all decorations to add
    const newDecorations: monaco.editor.IModelDeltaDecoration[] = []
    const oldDecorationIds: string[] = []

    // Collect old decoration IDs
    decorationsRef.current.forEach(deco => {
      oldDecorationIds.push(...deco.cursorId, ...deco.selectionId)
    })

    // Create new decorations for each cursor
    fileCursors.forEach(cursor => {
      if (!cursor.cursor) return

      // Inject styles for this cursor's color
      injectCursorStyles(cursor.color)

      const cursorClass = getCursorClassName(cursor.color)
      const selectionClass = getSelectionClassName(cursor.color)

      // Validate cursor position
      const lineCount = model.getLineCount()
      const line = Math.min(Math.max(1, cursor.cursor.line), lineCount)
      const maxColumn = model.getLineMaxColumn(line)
      const column = Math.min(Math.max(1, cursor.cursor.column), maxColumn)

      // Add cursor decoration (thin line)
      newDecorations.push({
        range: new monaco.Range(line, column, line, column),
        options: {
          className: cursorClass,
          stickiness: monaco.editor.TrackedRangeStickiness.NeverGrowsWhenTypingAtEdges,
          zIndex: 100,
        },
      })

      // Add selection decoration if there's a selection
      if (cursor.selection) {
        const startLine = Math.min(Math.max(1, cursor.selection.startLine), lineCount)
        const endLine = Math.min(Math.max(1, cursor.selection.endLine), lineCount)
        const startCol = Math.min(Math.max(1, cursor.selection.startColumn), model.getLineMaxColumn(startLine))
        const endCol = Math.min(Math.max(1, cursor.selection.endColumn), model.getLineMaxColumn(endLine))

        newDecorations.push({
          range: new monaco.Range(startLine, startCol, endLine, endCol),
          options: {
            className: selectionClass,
            stickiness: monaco.editor.TrackedRangeStickiness.NeverGrowsWhenTypingAtEdges,
          },
        })
      }
    })

    // Apply decorations
    const newIds = editor.deltaDecorations(oldDecorationIds, newDecorations)

    // Store new decoration IDs per user
    const newDecoMap = new Map<number, CursorDecorations>()
    let idIndex = 0

    fileCursors.forEach(cursor => {
      if (!cursor.cursor) return

      const deco: CursorDecorations = {
        cursorId: [newIds[idIndex++] || ''],
        selectionId: cursor.selection ? [newIds[idIndex++] || ''] : [],
      }
      newDecoMap.set(cursor.userId, deco)
    })

    decorationsRef.current = newDecoMap

    // Update label positions
    updateLabelPositions()
  }, [editor, fileCursors, updateLabelPositions])

  // Listen for scroll and resize events to update label positions
  useEffect(() => {
    if (!editor) return

    const scrollDisposable = editor.onDidScrollChange(() => {
      updateLabelPositions()
    })

    const layoutDisposable = editor.onDidLayoutChange(() => {
      updateLabelPositions()
    })

    // Also update on window resize
    const handleResize = () => {
      updateLabelPositions()
    }
    window.addEventListener('resize', handleResize)

    return () => {
      scrollDisposable.dispose()
      layoutDisposable.dispose()
      window.removeEventListener('resize', handleResize)
    }
  }, [editor, updateLabelPositions])

  // Cleanup decorations on unmount
  useEffect(() => {
    return () => {
      if (editor) {
        const allIds: string[] = []
        decorationsRef.current.forEach(deco => {
          allIds.push(...deco.cursorId, ...deco.selectionId)
        })
        if (allIds.length > 0) {
          editor.deltaDecorations(allIds, [])
        }
      }
    }
  }, [editor])

  // Get editor container for portal
  useEffect(() => {
    if (editor) {
      editorContainerRef.current = editor.getDomNode() as HTMLDivElement
    }
  }, [editor])

  return (
    <>
      {/* Render cursor labels */}
      {fileCursors.map(cursor => {
        const labelPos = cursorLabels.get(cursor.userId)
        if (!labelPos || !cursor.cursor) return null

        return (
          <CursorLabel
            key={cursor.userId}
            username={cursor.username}
            color={cursor.color}
            isTyping={cursor.isTyping}
            top={labelPos.top}
            left={labelPos.left}
          />
        )
      })}
    </>
  )
}

// User presence indicator for sidebar/header
export interface UserPresenceIndicatorProps {
  users: RemoteCursor[]
  maxVisible?: number
  size?: 'sm' | 'md' | 'lg'
  className?: string
  onUserClick?: (userId: number) => void
}

export const UserPresenceIndicator: React.FC<UserPresenceIndicatorProps> = ({
  users,
  maxVisible = 4,
  size = 'md',
  className,
  onUserClick,
}) => {
  const visibleUsers = users.slice(0, maxVisible)
  const remainingCount = Math.max(0, users.length - maxVisible)

  const sizeClasses = {
    sm: 'w-6 h-6 text-xs',
    md: 'w-8 h-8 text-sm',
    lg: 'w-10 h-10 text-base',
  }

  const sizeClass = sizeClasses[size]

  return (
    <div className={cn('flex items-center -space-x-2', className)}>
      {visibleUsers.map((user, index) => (
        <button
          key={user.userId}
          onClick={() => onUserClick?.(user.userId)}
          className={cn(
            'relative rounded-full border-2 border-gray-900 flex items-center justify-center font-medium transition-transform hover:scale-110 hover:z-10',
            sizeClass
          )}
          style={{
            backgroundColor: user.color,
            color: getContrastColor(user.color),
            zIndex: visibleUsers.length - index,
          }}
          title={`${user.username}${user.isTyping ? ' (typing...)' : ''}`}
        >
          {user.username.charAt(0).toUpperCase()}
          {user.isTyping && (
            <span className="absolute -bottom-1 -right-1 w-3 h-3 bg-green-500 rounded-full border border-gray-900 flex items-center justify-center">
              <span className="w-1.5 h-1.5 bg-white rounded-full animate-pulse" />
            </span>
          )}
        </button>
      ))}

      {remainingCount > 0 && (
        <div
          className={cn(
            'relative rounded-full bg-gray-700 border-2 border-gray-900 flex items-center justify-center text-gray-300 font-medium',
            sizeClass
          )}
          style={{ zIndex: 0 }}
          title={`${remainingCount} more users`}
        >
          +{remainingCount}
        </div>
      )}
    </div>
  )
}

// Active users list component
export interface ActiveUsersListProps {
  users: RemoteCursor[]
  currentUserId?: number
  onFollowUser?: (userId: number) => void
  onStopFollowing?: () => void
  followingUserId?: number
  className?: string
}

export const ActiveUsersList: React.FC<ActiveUsersListProps> = ({
  users,
  currentUserId,
  onFollowUser,
  onStopFollowing,
  followingUserId,
  className,
}) => {
  return (
    <div className={cn('flex flex-col gap-2', className)}>
      <div className="text-xs font-medium text-gray-400 uppercase tracking-wider px-2">
        Active Users ({users.length})
      </div>
      <div className="flex flex-col gap-1">
        {users.map(user => (
          <div
            key={user.userId}
            className={cn(
              'flex items-center gap-3 px-3 py-2 rounded-lg transition-colors',
              followingUserId === user.userId
                ? 'bg-cyan-500/20 border border-cyan-500/50'
                : 'hover:bg-gray-800/50'
            )}
          >
            <div
              className="w-8 h-8 rounded-full flex items-center justify-center font-medium text-sm"
              style={{
                backgroundColor: user.color,
                color: getContrastColor(user.color),
              }}
            >
              {user.username.charAt(0).toUpperCase()}
            </div>

            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium text-white truncate">
                {user.username}
                {user.userId === currentUserId && (
                  <span className="text-gray-500 ml-1">(you)</span>
                )}
              </div>
              <div className="text-xs text-gray-400 truncate">
                {user.fileName ? (
                  <>
                    {user.fileName}
                    {user.cursor && (
                      <span className="text-gray-500">
                        {' '}Ln {user.cursor.line}, Col {user.cursor.column}
                      </span>
                    )}
                  </>
                ) : (
                  <span className="text-gray-500">No file open</span>
                )}
              </div>
            </div>

            <div className="flex items-center gap-2">
              {user.isTyping && (
                <div className="flex gap-0.5 items-center">
                  <span className="w-1.5 h-1.5 bg-green-400 rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
                  <span className="w-1.5 h-1.5 bg-green-400 rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
                  <span className="w-1.5 h-1.5 bg-green-400 rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
                </div>
              )}

              {user.userId !== currentUserId && (
                followingUserId === user.userId ? (
                  <button
                    onClick={onStopFollowing}
                    className="text-xs px-2 py-1 bg-cyan-500/20 text-cyan-400 rounded hover:bg-cyan-500/30 transition-colors"
                  >
                    Following
                  </button>
                ) : (
                  <button
                    onClick={() => onFollowUser?.(user.userId)}
                    className="text-xs px-2 py-1 text-gray-400 hover:text-white hover:bg-gray-700 rounded transition-colors"
                  >
                    Follow
                  </button>
                )
              )}
            </div>
          </div>
        ))}

        {users.length === 0 && (
          <div className="text-sm text-gray-500 text-center py-4">
            No other users in this session
          </div>
        )}
      </div>
    </div>
  )
}

export default MultiplayerCursors
