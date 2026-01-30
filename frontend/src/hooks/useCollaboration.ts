// APEX.BUILD Collaboration Hook
// Real-time multiplayer cursor and presence management

import { useEffect, useCallback, useState, useRef } from 'react'
import { collaborationService, UserPresence, CursorInfo, SelectionInfo } from '@/services/collaboration'
import { useStore } from './useStore'

export interface RemoteCursor {
  userId: number
  username: string
  color: string
  fileId?: number
  fileName?: string
  cursor?: CursorInfo
  selection?: SelectionInfo
  isTyping: boolean
  lastActivity: Date
}

export interface UseCollaborationOptions {
  roomId?: string
  projectId?: number
  fileId?: number
  fileName?: string
  onUserJoined?: (user: UserPresence) => void
  onUserLeft?: (userId: number) => void
  onCursorUpdate?: (cursor: RemoteCursor) => void
  onSelectionUpdate?: (cursor: RemoteCursor) => void
  onTypingStart?: (userId: number, username: string) => void
  onTypingStop?: (userId: number, username: string) => void
}

export function useCollaboration(options: UseCollaborationOptions = {}) {
  const {
    roomId,
    projectId,
    fileId,
    fileName,
    onUserJoined,
    onUserLeft,
    onCursorUpdate,
    onSelectionUpdate,
    onTypingStart,
    onTypingStop,
  } = options

  const [isConnected, setIsConnected] = useState(false)
  const [remoteCursors, setRemoteCursors] = useState<Map<number, RemoteCursor>>(new Map())
  const [activeUsers, setActiveUsers] = useState<UserPresence[]>([])
  const cursorThrottleRef = useRef<NodeJS.Timeout | null>(null)
  const selectionThrottleRef = useRef<NodeJS.Timeout | null>(null)
  const lastCursorRef = useRef<{ line: number; column: number } | null>(null)

  const { user } = useStore()

  // Convert UserPresence to RemoteCursor
  const presenceToRemoteCursor = useCallback((presence: UserPresence): RemoteCursor => ({
    userId: presence.userId,
    username: presence.username,
    color: presence.color,
    fileId: presence.fileId,
    fileName: presence.fileName,
    cursor: presence.cursor,
    selection: presence.selection,
    isTyping: presence.isTyping,
    lastActivity: presence.lastActivity,
  }), [])

  // Handle connection events
  useEffect(() => {
    const handleConnected = () => {
      setIsConnected(true)
      console.log('[useCollaboration] Connected to collaboration service')
    }

    const handleDisconnected = () => {
      setIsConnected(false)
      setRemoteCursors(new Map())
      setActiveUsers([])
      console.log('[useCollaboration] Disconnected from collaboration service')
    }

    const handleUserJoined = (presence: UserPresence) => {
      if (presence.userId === user?.id) return

      setActiveUsers(prev => {
        const existing = prev.find(u => u.userId === presence.userId)
        if (existing) {
          return prev.map(u => u.userId === presence.userId ? presence : u)
        }
        return [...prev, presence]
      })

      setRemoteCursors(prev => {
        const newMap = new Map(prev)
        newMap.set(presence.userId, presenceToRemoteCursor(presence))
        return newMap
      })

      onUserJoined?.(presence)
    }

    const handleUserLeft = (userId: number) => {
      setActiveUsers(prev => prev.filter(u => u.userId !== userId))
      setRemoteCursors(prev => {
        const newMap = new Map(prev)
        newMap.delete(userId)
        return newMap
      })
      onUserLeft?.(userId)
    }

    const handleUserList = (users: UserPresence[]) => {
      const filteredUsers = users.filter(u => u.userId !== user?.id)
      setActiveUsers(filteredUsers)

      const newCursors = new Map<number, RemoteCursor>()
      filteredUsers.forEach(u => {
        newCursors.set(u.userId, presenceToRemoteCursor(u))
      })
      setRemoteCursors(newCursors)
    }

    const handleCursorUpdate = (presence: UserPresence) => {
      if (presence.userId === user?.id) return

      const remoteCursor = presenceToRemoteCursor(presence)

      setRemoteCursors(prev => {
        const newMap = new Map(prev)
        newMap.set(presence.userId, remoteCursor)
        return newMap
      })

      onCursorUpdate?.(remoteCursor)
    }

    const handleSelectionUpdate = (presence: UserPresence) => {
      if (presence.userId === user?.id) return

      const remoteCursor = presenceToRemoteCursor(presence)

      setRemoteCursors(prev => {
        const newMap = new Map(prev)
        const existing = prev.get(presence.userId)
        newMap.set(presence.userId, { ...existing, ...remoteCursor })
        return newMap
      })

      onSelectionUpdate?.(remoteCursor)
    }

    const handleTypingStartEvent = (userId: number, username: string) => {
      if (userId === user?.id) return

      setRemoteCursors(prev => {
        const newMap = new Map(prev)
        const existing = prev.get(userId)
        if (existing) {
          newMap.set(userId, { ...existing, isTyping: true })
        }
        return newMap
      })

      onTypingStart?.(userId, username)
    }

    const handleTypingStopEvent = (userId: number, username: string) => {
      if (userId === user?.id) return

      setRemoteCursors(prev => {
        const newMap = new Map(prev)
        const existing = prev.get(userId)
        if (existing) {
          newMap.set(userId, { ...existing, isTyping: false })
        }
        return newMap
      })

      onTypingStop?.(userId, username)
    }

    // Subscribe to events
    collaborationService.on('connected', handleConnected)
    collaborationService.on('disconnected', handleDisconnected)
    collaborationService.on('userJoined', handleUserJoined)
    collaborationService.on('userLeft', handleUserLeft)
    collaborationService.on('userListUpdate', handleUserList)
    collaborationService.on('cursorUpdate', handleCursorUpdate)
    collaborationService.on('selectionUpdate', handleSelectionUpdate)
    collaborationService.on('typingStart', handleTypingStartEvent)
    collaborationService.on('typingStop', handleTypingStopEvent)

    // Check if already connected
    setIsConnected(collaborationService.isConnected())

    return () => {
      collaborationService.off('connected', handleConnected)
      collaborationService.off('disconnected', handleDisconnected)
      collaborationService.off('userJoined', handleUserJoined)
      collaborationService.off('userLeft', handleUserLeft)
      collaborationService.off('userListUpdate', handleUserList)
      collaborationService.off('cursorUpdate', handleCursorUpdate)
      collaborationService.off('selectionUpdate', handleSelectionUpdate)
      collaborationService.off('typingStart', handleTypingStartEvent)
      collaborationService.off('typingStop', handleTypingStopEvent)
    }
  }, [user?.id, onUserJoined, onUserLeft, onCursorUpdate, onSelectionUpdate, onTypingStart, onTypingStop, presenceToRemoteCursor])

  // Join room when roomId/projectId changes
  useEffect(() => {
    if (roomId && isConnected) {
      collaborationService.joinRoom(roomId, projectId)
    }
  }, [roomId, projectId, isConnected])

  // Update cursor position (throttled to 50ms)
  const updateCursor = useCallback((line: number, column: number) => {
    if (!fileId || !fileName) return

    // Skip if position hasn't changed
    if (lastCursorRef.current?.line === line && lastCursorRef.current?.column === column) {
      return
    }
    lastCursorRef.current = { line, column }

    // Throttle cursor updates
    if (cursorThrottleRef.current) {
      clearTimeout(cursorThrottleRef.current)
    }

    cursorThrottleRef.current = setTimeout(() => {
      collaborationService.updateCursor(fileId, fileName, line, column)
      cursorThrottleRef.current = null
    }, 50)
  }, [fileId, fileName])

  // Update selection (throttled to 100ms)
  const updateSelection = useCallback((
    startLine: number,
    startColumn: number,
    endLine: number,
    endColumn: number
  ) => {
    // Throttle selection updates
    if (selectionThrottleRef.current) {
      clearTimeout(selectionThrottleRef.current)
    }

    selectionThrottleRef.current = setTimeout(() => {
      collaborationService.updateSelection(startLine, startColumn, endLine, endColumn)
      selectionThrottleRef.current = null
    }, 100)
  }, [])

  // Clear selection
  const clearSelection = useCallback(() => {
    collaborationService.clearSelection()
  }, [])

  // Start typing indicator
  const startTyping = useCallback(() => {
    collaborationService.startTyping()
  }, [])

  // Stop typing indicator
  const stopTyping = useCallback(() => {
    collaborationService.stopTyping()
  }, [])

  // Follow a user
  const followUser = useCallback((targetUserId: number) => {
    collaborationService.followUser(targetUserId)
  }, [])

  // Stop following
  const stopFollowing = useCallback(() => {
    collaborationService.stopFollowing()
  }, [])

  // Get cursors for a specific file
  const getCursorsForFile = useCallback((targetFileId: number): RemoteCursor[] => {
    return Array.from(remoteCursors.values()).filter(c => c.fileId === targetFileId)
  }, [remoteCursors])

  // Get all remote cursors as array
  const getAllCursors = useCallback((): RemoteCursor[] => {
    return Array.from(remoteCursors.values())
  }, [remoteCursors])

  // Cleanup throttle timers on unmount
  useEffect(() => {
    return () => {
      if (cursorThrottleRef.current) {
        clearTimeout(cursorThrottleRef.current)
      }
      if (selectionThrottleRef.current) {
        clearTimeout(selectionThrottleRef.current)
      }
    }
  }, [])

  return {
    isConnected,
    remoteCursors,
    activeUsers,
    updateCursor,
    updateSelection,
    clearSelection,
    startTyping,
    stopTyping,
    followUser,
    stopFollowing,
    getCursorsForFile,
    getAllCursors,
  }
}

export default useCollaboration
