// APEX.BUILD WebSocket Service
// Real-time collaboration with sub-20ms latency

import { io, Socket } from 'socket.io-client'
import {
  User,
  File,
  CursorPosition,
  ChatMessage,
  WSMessage,
  FileChangeMessage,
  CursorUpdateMessage,
} from '@/types'

export type CollaborationEvent =
  | 'user-joined'
  | 'user-left'
  | 'file-changed'
  | 'cursor-moved'
  | 'chat-message'
  | 'file-locked'
  | 'file-unlocked'
  | 'project-updated'
  | 'ai-request'
  | 'ai-response'

export interface CollaborationEventData {
  'user-joined': { user: User; room_id: string }
  'user-left': { user_id: number; room_id: string }
  'file-changed': FileChangeMessage
  'cursor-moved': CursorUpdateMessage
  'chat-message': ChatMessage
  'file-locked': { file_id: number; user_id: number; user: User }
  'file-unlocked': { file_id: number; user_id: number }
  'project-updated': { project_id: number; changes: Record<string, any> }
  'ai-request': { request_id: string; user_id: number; capability: string }
  'ai-response': { request_id: string; content: string; provider: string }
}

export class WebSocketService {
  private socket: Socket | null = null
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000
  private heartbeatInterval: NodeJS.Timeout | null = null
  private isConnecting = false
  private currentRoom: string | null = null
  private listeners: Map<CollaborationEvent, Set<Function>> = new Map()

  constructor() {
    this.initializeListeners()
  }

  private initializeListeners(): void {
    // Initialize listener sets for each event type
    const events: CollaborationEvent[] = [
      'user-joined',
      'user-left',
      'file-changed',
      'cursor-moved',
      'chat-message',
      'file-locked',
      'file-unlocked',
      'project-updated',
      'ai-request',
      'ai-response',
    ]

    events.forEach((event) => {
      this.listeners.set(event, new Set())
    })
  }

  // Get WebSocket URL for current environment
  private getWsUrl(): string {
    // Check for Vite environment variable
    if (import.meta.env.VITE_WS_URL) {
      return import.meta.env.VITE_WS_URL
    }

    // Production detection - if running on Render or production domain
    const hostname = typeof window !== 'undefined' ? window.location.hostname : ''
    if (hostname.includes('onrender.com') || hostname.includes('apex.build') || hostname === 'apex-frontend-gigq.onrender.com') {
      return 'wss://apex-backend-5ypy.onrender.com'
    }

    // Fallback for local development
    return ''
  }

  // Connect to WebSocket server
  async connect(token: string): Promise<void> {
    if (this.socket?.connected || this.isConnecting) {
      return
    }

    this.isConnecting = true
    const wsUrl = this.getWsUrl()

    try {
      this.socket = io(wsUrl ? `${wsUrl}/ws` : '/ws', {
        auth: {
          token,
        },
        transports: ['websocket', 'polling'],
        timeout: 10000,
        reconnection: true,
        reconnectionAttempts: this.maxReconnectAttempts,
        reconnectionDelay: this.reconnectDelay,
        reconnectionDelayMax: 5000,
      })

      this.setupSocketEvents()

      return new Promise((resolve, reject) => {
        const connectTimeout = setTimeout(() => {
          reject(new Error('Connection timeout'))
        }, 15000)

        this.socket!.on('connect', () => {
          clearTimeout(connectTimeout)
          this.isConnecting = false
          this.reconnectAttempts = 0
          this.startHeartbeat()
          console.log('✅ APEX.BUILD WebSocket connected')
          resolve()
        })

        this.socket!.on('connect_error', (error) => {
          clearTimeout(connectTimeout)
          this.isConnecting = false
          console.error('❌ WebSocket connection error:', error)
          reject(error)
        })
      })
    } catch (error) {
      this.isConnecting = false
      throw error
    }
  }

  private setupSocketEvents(): void {
    if (!this.socket) return

    // Connection events
    this.socket.on('connect', () => {
      console.log('🔌 WebSocket connected')
      this.reconnectAttempts = 0
    })

    this.socket.on('disconnect', (reason) => {
      console.log('🔌 WebSocket disconnected:', reason)
      this.stopHeartbeat()

      if (reason === 'io server disconnect') {
        // Server initiated disconnect, don't auto-reconnect
        return
      }

      // Auto-reconnect for other reasons
      this.scheduleReconnect()
    })

    this.socket.on('reconnect', (attemptNumber) => {
      console.log(`🔄 WebSocket reconnected after ${attemptNumber} attempts`)
      if (this.currentRoom) {
        this.joinRoom(this.currentRoom)
      }
    })

    this.socket.on('reconnect_error', (error) => {
      console.error('🔄 WebSocket reconnect error:', error)
    })

    this.socket.on('reconnect_failed', () => {
      console.error('🔄 WebSocket reconnect failed - maximum attempts reached')
    })

    // Collaboration events
    this.socket.on('user-joined', (data: CollaborationEventData['user-joined']) => {
      this.emit('user-joined', data)
    })

    this.socket.on('user-left', (data: CollaborationEventData['user-left']) => {
      this.emit('user-left', data)
    })

    this.socket.on('file-changed', (data: CollaborationEventData['file-changed']) => {
      this.emit('file-changed', data)
    })

    this.socket.on('cursor-moved', (data: CollaborationEventData['cursor-moved']) => {
      this.emit('cursor-moved', data)
    })

    this.socket.on('chat-message', (data: CollaborationEventData['chat-message']) => {
      this.emit('chat-message', data)
    })

    this.socket.on('file-locked', (data: CollaborationEventData['file-locked']) => {
      this.emit('file-locked', data)
    })

    this.socket.on('file-unlocked', (data: CollaborationEventData['file-unlocked']) => {
      this.emit('file-unlocked', data)
    })

    this.socket.on('project-updated', (data: CollaborationEventData['project-updated']) => {
      this.emit('project-updated', data)
    })

    this.socket.on('ai-request', (data: CollaborationEventData['ai-request']) => {
      this.emit('ai-request', data)
    })

    this.socket.on('ai-response', (data: CollaborationEventData['ai-response']) => {
      this.emit('ai-response', data)
    })

    // Error handling
    this.socket.on('error', (error) => {
      console.error('🚨 WebSocket error:', error)
    })

    // Heartbeat response
    this.socket.on('pong', () => {
      // Heartbeat acknowledged
    })
  }

  private startHeartbeat(): void {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval)
    }

    this.heartbeatInterval = setInterval(() => {
      if (this.socket?.connected) {
        this.socket.emit('ping')
      }
    }, 30000) // Every 30 seconds
  }

  private stopHeartbeat(): void {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval)
      this.heartbeatInterval = null
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error('🔄 Maximum reconnect attempts reached')
      return
    }

    this.reconnectAttempts++
    const delay = Math.min(this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1), 10000)

    setTimeout(() => {
      if (!this.socket?.connected && !this.isConnecting) {
        console.log(`🔄 Attempting to reconnect (${this.reconnectAttempts}/${this.maxReconnectAttempts})`)
        this.socket?.connect()
      }
    }, delay)
  }

  // Event emission and subscription
  private emit<T extends CollaborationEvent>(event: T, data: CollaborationEventData[T]): void {
    const listeners = this.listeners.get(event)
    if (listeners) {
      listeners.forEach((callback) => {
        try {
          callback(data)
        } catch (error) {
          console.error(`Error in ${event} listener:`, error)
        }
      })
    }
  }

  on<T extends CollaborationEvent>(
    event: T,
    callback: (data: CollaborationEventData[T]) => void
  ): () => void {
    const listeners = this.listeners.get(event)
    if (listeners) {
      listeners.add(callback)
      return () => listeners.delete(callback)
    }
    return () => {}
  }

  off<T extends CollaborationEvent>(
    event: T,
    callback?: (data: CollaborationEventData[T]) => void
  ): void {
    const listeners = this.listeners.get(event)
    if (listeners) {
      if (callback) {
        listeners.delete(callback)
      } else {
        listeners.clear()
      }
    }
  }

  // Room management
  async joinRoom(roomId: string): Promise<void> {
    if (!this.socket?.connected) {
      throw new Error('WebSocket not connected')
    }

    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        reject(new Error('Join room timeout'))
      }, 10000)

      this.socket!.emit('join-room', { room_id: roomId }, (response: any) => {
        clearTimeout(timeout)
        if (response.success) {
          this.currentRoom = roomId
          console.log(`🏠 Joined collaboration room: ${roomId}`)
          resolve()
        } else {
          reject(new Error(response.error || 'Failed to join room'))
        }
      })
    })
  }

  async leaveRoom(): Promise<void> {
    if (!this.socket?.connected || !this.currentRoom) {
      return
    }

    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        reject(new Error('Leave room timeout'))
      }, 5000)

      this.socket!.emit('leave-room', { room_id: this.currentRoom }, (response: any) => {
        clearTimeout(timeout)
        if (response.success) {
          console.log(`🚪 Left collaboration room: ${this.currentRoom}`)
          this.currentRoom = null
          resolve()
        } else {
          reject(new Error(response.error || 'Failed to leave room'))
        }
      })
    })
  }

  // File operations
  sendFileChange(fileId: number, content: string, line: number, column: number): void {
    if (!this.socket?.connected || !this.currentRoom) {
      console.warn('Cannot send file change: WebSocket not connected or not in room')
      return
    }

    const message: FileChangeMessage = {
      file_id: fileId,
      content,
      line,
      column,
      change_type: 'replace',
      user_id: 0, // Will be set by server
    }

    this.socket.emit('file-change', {
      room_id: this.currentRoom,
      ...message,
    })
  }

  sendCursorUpdate(
    fileId: number,
    line: number,
    column: number,
    selection?: {
      startLine: number
      startColumn: number
      endLine: number
      endColumn: number
    }
  ): void {
    if (!this.socket?.connected || !this.currentRoom) {
      return
    }

    const message: CursorUpdateMessage = {
      user_id: 0, // Will be set by server
      file_id: fileId,
      line,
      column,
      selection,
    }

    this.socket.emit('cursor-update', {
      room_id: this.currentRoom,
      ...message,
    })
  }

  // Chat operations
  sendChatMessage(message: string, type: 'text' | 'code' | 'file' = 'text'): void {
    if (!this.socket?.connected || !this.currentRoom) {
      console.warn('Cannot send chat message: WebSocket not connected or not in room')
      return
    }

    this.socket.emit('chat-message', {
      room_id: this.currentRoom,
      message,
      type,
    })
  }

  // File locking
  lockFile(fileId: number): void {
    if (!this.socket?.connected || !this.currentRoom) {
      return
    }

    this.socket.emit('lock-file', {
      room_id: this.currentRoom,
      file_id: fileId,
    })
  }

  unlockFile(fileId: number): void {
    if (!this.socket?.connected || !this.currentRoom) {
      return
    }

    this.socket.emit('unlock-file', {
      room_id: this.currentRoom,
      file_id: fileId,
    })
  }

  // AI collaboration
  broadcastAIRequest(requestId: string, capability: string): void {
    if (!this.socket?.connected || !this.currentRoom) {
      return
    }

    this.socket.emit('ai-request', {
      room_id: this.currentRoom,
      request_id: requestId,
      capability,
    })
  }

  shareAIResponse(requestId: string, content: string, provider: string): void {
    if (!this.socket?.connected || !this.currentRoom) {
      return
    }

    this.socket.emit('ai-response', {
      room_id: this.currentRoom,
      request_id: requestId,
      content,
      provider,
    })
  }

  // Connection status
  isConnected(): boolean {
    return this.socket?.connected || false
  }

  getConnectionStatus(): {
    connected: boolean
    room: string | null
    reconnectAttempts: number
  } {
    return {
      connected: this.socket?.connected || false,
      room: this.currentRoom,
      reconnectAttempts: this.reconnectAttempts,
    }
  }

  // Disconnect
  disconnect(): void {
    if (this.socket) {
      this.stopHeartbeat()
      this.socket.disconnect()
      this.socket = null
      this.currentRoom = null
      this.reconnectAttempts = 0
      this.isConnecting = false
      console.log('🔌 WebSocket manually disconnected')
    }
  }

  // Performance monitoring
  measureLatency(): Promise<number> {
    if (!this.socket?.connected) {
      return Promise.reject(new Error('WebSocket not connected'))
    }

    const startTime = performance.now()

    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        reject(new Error('Latency measurement timeout'))
      }, 5000)

      this.socket!.emit('ping-latency', { timestamp: startTime }, (response: any) => {
        clearTimeout(timeout)
        const endTime = performance.now()
        const latency = endTime - startTime
        resolve(latency)
      })
    })
  }
}

// Create singleton instance
export const websocketService = new WebSocketService()

// Export for easy importing
export default websocketService