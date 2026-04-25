// APEX-BUILD Collaboration WebSocket Service
// Raw WebSocket implementation matching backend gorilla/websocket hub

import { getConfiguredWsUrl } from '@/config/runtime'
import {
  User,
  CursorPosition,
  ChatMessage,
  WSMessage,
  FileChangeMessage,
  CursorUpdateMessage,
  FSMWSMessageType,
} from '@/types'
import { useStore } from '@/hooks/useStore'

const DEFAULT_PRODUCTION_WS_URL = 'wss://apex-backend-5ypy.onrender.com/ws'

// Collaboration event types (kebab-case, matching frontend consumers)
export type CollaborationEvent =
  | 'user-joined'
  | 'user-left'
  | 'file-changed'
  | 'cursor-moved'
  | 'chat-message'
  | 'file-locked'
  | 'file-unlocked'
  | 'project-updated'

export interface CollaborationEventData {
  'user-joined': { user: User; room_id: string }
  'user-left': { user_id: number; room_id: string }
  'file-changed': FileChangeMessage
  'cursor-moved': CursorUpdateMessage
  'chat-message': ChatMessage
  'file-locked': { file_id: number; user_id: number; user: User }
  'file-unlocked': { file_id: number; user_id: number }
  'project-updated': { project_id: number; changes: Record<string, any> }
}

// Backend message type mapping
const BACKEND_TO_FRONTEND_EVENT: Record<string, CollaborationEvent> = {
  'user_joined': 'user-joined',
  'user_left': 'user-left',
  'file_change': 'file-changed',
  'cursor_update': 'cursor-moved',
  'chat': 'chat-message',
  'file_locked': 'file-locked',
  'file_unlocked': 'file-unlocked',
  'project_updated': 'project-updated',
}

const FRONTEND_TO_BACKEND_TYPE: Record<string, string> = {
  'join-room': 'join_room',
  'leave-room': 'leave_room',
  'file-change': 'file_change',
  'cursor-update': 'cursor_update',
  'chat-message': 'chat',
}

const DEBUG = import.meta.env.DEV

export class WebSocketService {
  private ws: WebSocket | null = null
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000
  private heartbeatInterval: ReturnType<typeof setInterval> | null = null
  private isConnecting = false
  private currentRoom: string | null = null
  private listeners: Map<CollaborationEvent, Set<Function>> = new Map()
  private pendingJoinRoom: { resolve: () => void; reject: (err: Error) => void } | null = null

  private log(...args: any[]): void {
    if (DEBUG) console.log('[WS-Collab]', ...args)
  }

  constructor() {
    this.initializeListeners()
  }

  private initializeListeners(): void {
    const events: CollaborationEvent[] = [
      'user-joined',
      'user-left',
      'file-changed',
      'cursor-moved',
      'chat-message',
      'file-locked',
      'file-unlocked',
      'project-updated',
    ]
    events.forEach((event) => {
      this.listeners.set(event, new Set())
    })
  }

  private getWsUrl(): string {
    const configuredWsUrl = getConfiguredWsUrl()
    if (configuredWsUrl) {
      return configuredWsUrl.replace(/\/ws(\/collab)?$/, '') + '/ws/collab'
    }

    const hostname = typeof window !== 'undefined' ? window.location.hostname : ''
    const productionHosts = ['onrender.com', 'apex-build.dev', 'apex.build', 'web.app', 'firebaseapp.com']
    const isProduction = productionHosts.some((h) => hostname.includes(h))
      || hostname === 'apex-frontend-gigq.onrender.com'

    if (isProduction) {
      return DEFAULT_PRODUCTION_WS_URL + '/collab'
    }

    // Local development — derive from current host
    const protocol = typeof window !== 'undefined' && window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}/ws/collab`
  }

  async connect(): Promise<void> {
    if (this.isConnecting) return
    if (this.ws?.readyState === WebSocket.OPEN) return

    this.closeExisting()
    this.isConnecting = true

    const url = this.getWsUrl()
    this.log('Connecting to', url)

    return new Promise((resolve, reject) => {
      try {
        this.ws = new WebSocket(url)
      } catch (err) {
        this.isConnecting = false
        reject(new Error('Failed to create WebSocket: ' + (err as Error).message))
        return
      }

      const timeout = setTimeout(() => {
        this.isConnecting = false
        this.ws?.close()
        reject(new Error('Connection timeout'))
      }, 15000)

      this.ws.onopen = () => {
        clearTimeout(timeout)
        this.isConnecting = false
        this.reconnectAttempts = 0
        this.startHeartbeat()
        this.log('✅ Connected to collaboration hub')
        resolve()
      }

      this.ws.onmessage = (event) => {
        this.handleMessage(event.data)
      }

      this.ws.onerror = (error) => {
        clearTimeout(timeout)
        this.isConnecting = false
        console.error('❌ WebSocket error:', error)
        reject(new Error('WebSocket connection failed'))
      }

      this.ws.onclose = (event) => {
        clearTimeout(timeout)
        this.isConnecting = false
        this.stopHeartbeat()
        this.log('🔌 Connection closed', event.code, event.reason)

        if (this.pendingJoinRoom) {
          this.pendingJoinRoom.reject(new Error('Connection closed before join completed'))
          this.pendingJoinRoom = null
        }

        if (!event.wasClean && this.reconnectAttempts < this.maxReconnectAttempts) {
          this.scheduleReconnect()
        }
      }
    })
  }

  private handleMessage(rawData: string | ArrayBuffer | Blob): void {
    let message: any
    try {
      if (typeof rawData === 'string') {
        message = JSON.parse(rawData)
      } else {
        this.log('Received non-text message, ignoring')
        return
      }
    } catch {
      this.log('Failed to parse message:', rawData)
      return
    }

    const msgType = message.type as string
    const data = message.data || message

    this.log('←', msgType, data)

    // Map backend event names to frontend event names
    const frontendEvent = BACKEND_TO_FRONTEND_EVENT[msgType]
    if (frontendEvent) {
      this.emit(frontendEvent, data)
      return
    }

    // FSM build events — pass through to zustand store
    if (msgType?.startsWith('build:fsm:')) {
      const buildID = data?.build_id || data?.buildId || ''
      useStore.getState().handleFSMEvent(msgType as FSMWSMessageType, buildID, data)
      return
    }

    // Handle join-room response (backend sends 'room_joined', not 'join_room')
    if (msgType === 'room_joined' && this.pendingJoinRoom) {
      if (data?.success) {
        this.pendingJoinRoom.resolve()
      } else {
        this.pendingJoinRoom.reject(new Error(data?.error || 'Failed to join room'))
      }
      this.pendingJoinRoom = null
      return
    }

    // Other backend events we don't map
    this.log('Unhandled message type:', msgType)
  }

  private send(type: string, data?: Record<string, any>): void {
    if (this.ws?.readyState !== WebSocket.OPEN) {
      console.warn('Cannot send — WebSocket not open')
      return
    }
    const payload = data ? { type, ...data } : { type }
    this.ws.send(JSON.stringify(payload))
  }

  private closeExisting(): void {
    if (this.ws) {
      this.ws.onclose = null
      this.ws.onmessage = null
      this.ws.onerror = null
      this.ws.close()
      this.ws = null
    }
    this.stopHeartbeat()
  }

  private startHeartbeat(): void {
    this.stopHeartbeat()
    this.heartbeatInterval = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.send('heartbeat')
      }
    }, 30000)
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
    this.log(`🔄 Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`)

    setTimeout(() => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        this.connect().catch(() => {
          // Reconnect handled by onclose
        })
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
    if (this.ws?.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket not connected')
    }

    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.pendingJoinRoom = null
        reject(new Error('Join room timeout'))
      }, 10000)

      this.pendingJoinRoom = {
        resolve: () => {
          clearTimeout(timeout)
          this.currentRoom = roomId
          this.log(`🏠 Joined room: ${roomId}`)
          resolve()
        },
        reject: (err: Error) => {
          clearTimeout(timeout)
          reject(err)
        },
      }

      this.send('join_room', { room_id: roomId })
    })
  }

  leaveRoom(): void {
    if (!this.currentRoom || this.ws?.readyState !== WebSocket.OPEN) {
      this.currentRoom = null
      return
    }
    this.send('leave_room', { room_id: this.currentRoom })
    this.currentRoom = null
    this.log('🚪 Left room')
  }

  // File operations
  sendFileChange(fileId: number, content: string, line: number, column: number): void {
    if (this.ws?.readyState !== WebSocket.OPEN || !this.currentRoom) {
      console.warn('Cannot send file change: WebSocket not connected or not in room')
      return
    }

    this.send('file_change', {
      room_id: this.currentRoom,
      file_id: fileId,
      content,
      line,
      column,
      change_type: 'replace',
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
    if (this.ws?.readyState !== WebSocket.OPEN || !this.currentRoom) {
      console.warn('Cannot send cursor update: WebSocket not connected or not in room')
      return
    }

    this.send('cursor_update', {
      room_id: this.currentRoom,
      file_id: fileId,
      line,
      column,
      selection,
    })
  }

  sendChatMessage(message: string): void {
    if (this.ws?.readyState !== WebSocket.OPEN || !this.currentRoom) {
      console.warn('Cannot send chat: WebSocket not connected or not in room')
      return
    }

    this.send('chat', {
      room_id: this.currentRoom,
      content: message,
    })
  }

  disconnect(): void {
    this.closeExisting()
    this.currentRoom = null
    this.reconnectAttempts = 0
    this.log('🔌 Disconnected')
  }

  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }

  getCurrentRoom(): string | null {
    return this.currentRoom
  }
}

// Singleton instance
let wsInstance: WebSocketService | null = null

export function getWebSocketService(): WebSocketService {
  if (!wsInstance) {
    wsInstance = new WebSocketService()
  }
  return wsInstance
}

export function resetWebSocketService(): void {
  if (wsInstance) {
    wsInstance.disconnect()
    wsInstance = null
  }
}
