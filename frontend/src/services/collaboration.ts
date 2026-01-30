// APEX.BUILD Real-Time Collaboration Service
// Yjs-inspired CRDT with OT fallback for concurrent editing

// Browser-compatible EventEmitter implementation
class EventEmitter {
  private events: Map<string, Set<(...args: any[]) => void>> = new Map()

  on(event: string, listener: (...args: any[]) => void): this {
    if (!this.events.has(event)) {
      this.events.set(event, new Set())
    }
    this.events.get(event)!.add(listener)
    return this
  }

  off(event: string, listener: (...args: any[]) => void): this {
    const listeners = this.events.get(event)
    if (listeners) {
      listeners.delete(listener)
    }
    return this
  }

  emit(event: string, ...args: any[]): boolean {
    const listeners = this.events.get(event)
    if (listeners && listeners.size > 0) {
      listeners.forEach(listener => {
        try {
          listener(...args)
        } catch (e) {
          console.error(`Error in event listener for ${event}:`, e)
        }
      })
      return true
    }
    return false
  }

  removeAllListeners(event?: string): this {
    if (event) {
      this.events.delete(event)
    } else {
      this.events.clear()
    }
    return this
  }

  setMaxListeners(_n: number): this {
    // No-op for browser compatibility
    return this
  }
}

// Types
export interface Operation {
  type: 'insert' | 'delete' | 'retain'
  position: number
  text?: string
  count?: number
}

export interface TextOperation {
  operations: Operation[]
  baseVersion: number
  userId: number
  fileId: number
  timestamp: Date
}

export interface UserPresence {
  userId: number
  username: string
  email: string
  avatarUrl?: string
  color: string
  fileId?: number
  fileName?: string
  cursor?: CursorInfo
  selection?: SelectionInfo
  isTyping: boolean
  lastActivity: Date
  isFollowing?: number
  permission: PermissionLevel
  status: PresenceStatus
}

export interface CursorInfo {
  line: number
  column: number
  updatedAt: Date
}

export interface SelectionInfo {
  startLine: number
  startColumn: number
  endLine: number
  endColumn: number
  updatedAt: Date
}

export type PermissionLevel = 'viewer' | 'editor' | 'admin' | 'owner'
export type PresenceStatus = 'online' | 'away' | 'busy' | 'offline'

export interface ChatMessage {
  id: string
  userId: number
  username: string
  avatarUrl?: string
  message: string
  type: 'text' | 'code' | 'system'
  timestamp: Date
}

export interface ActivityFeedItem {
  id: string
  userId: number
  username: string
  avatarUrl?: string
  action: string
  target?: string
  timestamp: Date
}

export interface RTCPeerState {
  peerId: number
  connected: boolean
  audioEnabled: boolean
  videoEnabled: boolean
  screenSharing: boolean
}

// Message types
export type CollabMessageType =
  | 'join_room'
  | 'leave_room'
  | 'room_joined'
  | 'room_left'
  | 'user_joined'
  | 'user_left'
  | 'error'
  | 'heartbeat'
  | 'cursor_update'
  | 'selection_update'
  | 'typing_start'
  | 'typing_stop'
  | 'follow_user'
  | 'stop_following'
  | 'presence_update'
  | 'user_list'
  | 'operation'
  | 'operation_ack'
  | 'sync_request'
  | 'sync_response'
  | 'file_change'
  | 'permission_update'
  | 'kick_user'
  | 'user_kicked'
  | 'chat'
  | 'chat_history'
  | 'rtc_offer'
  | 'rtc_answer'
  | 'rtc_candidate'
  | 'media_state_change'
  | 'activity'
  | 'activity_feed'

export interface CollabMessage {
  type: CollabMessageType
  roomId?: string
  userId?: number
  username?: string
  data?: any
  timestamp: Date
}

// Collaboration Events
export interface CollabEvents {
  connected: () => void
  disconnected: (reason: string) => void
  error: (error: string) => void
  roomJoined: (roomId: string) => void
  roomLeft: () => void
  userJoined: (presence: UserPresence) => void
  userLeft: (userId: number) => void
  userListUpdate: (users: UserPresence[]) => void
  cursorUpdate: (presence: UserPresence) => void
  selectionUpdate: (presence: UserPresence) => void
  typingStart: (userId: number, username: string) => void
  typingStop: (userId: number, username: string) => void
  followUpdate: (followerId: number, targetId: number | null) => void
  operation: (fileId: number, operations: Operation[], version: number, userId: number) => void
  operationAck: (version: number) => void
  syncResponse: (fileId: number, content: string, version: number) => void
  permissionUpdate: (userId: number, permission: PermissionLevel) => void
  kicked: () => void
  userKicked: (userId: number, kickedBy: number) => void
  chatMessage: (message: ChatMessage) => void
  activityUpdate: (activity: ActivityFeedItem) => void
  rtcOffer: (fromUserId: number, offer: RTCSessionDescriptionInit) => void
  rtcAnswer: (fromUserId: number, answer: RTCSessionDescriptionInit) => void
  rtcCandidate: (fromUserId: number, candidate: RTCIceCandidateInit) => void
  mediaStateChange: (userId: number, state: Partial<RTCPeerState>) => void
}

// Collaboration Service
export class CollaborationService extends EventEmitter {
  private ws: WebSocket | null = null
  private roomId: string | null = null
  private userId: number = 0
  private username: string = ''
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000
  private heartbeatInterval: NodeJS.Timeout | null = null
  private isConnecting = false
  private pendingOperations: Map<number, TextOperation[]> = new Map()
  private documentVersions: Map<number, number> = new Map()
  private users: Map<number, UserPresence> = new Map()
  private typingTimeout: NodeJS.Timeout | null = null

  constructor() {
    super()
    this.setMaxListeners(50)
  }

  // Connection management
  async connect(token: string): Promise<void> {
    if (this.ws?.readyState === WebSocket.OPEN || this.isConnecting) {
      return
    }

    this.isConnecting = true

    return new Promise((resolve, reject) => {
      const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/v1/collab/ws?token=${token}`

      this.ws = new WebSocket(wsUrl)

      const timeout = setTimeout(() => {
        this.isConnecting = false
        reject(new Error('Connection timeout'))
      }, 15000)

      this.ws.onopen = () => {
        clearTimeout(timeout)
        this.isConnecting = false
        this.reconnectAttempts = 0
        this.startHeartbeat()
        this.emit('connected')
        console.log('[Collab] Connected to collaboration server')
        resolve()
      }

      this.ws.onclose = (event) => {
        clearTimeout(timeout)
        this.isConnecting = false
        this.stopHeartbeat()
        this.emit('disconnected', event.reason || 'Connection closed')
        console.log('[Collab] Disconnected:', event.reason)

        if (event.code !== 1000) {
          this.scheduleReconnect()
        }
      }

      this.ws.onerror = (error) => {
        clearTimeout(timeout)
        this.isConnecting = false
        console.error('[Collab] WebSocket error:', error)
        reject(new Error('WebSocket connection failed'))
      }

      this.ws.onmessage = (event) => {
        this.handleMessage(event.data)
      }
    })
  }

  disconnect(): void {
    if (this.ws) {
      this.stopHeartbeat()
      this.ws.close(1000, 'User disconnected')
      this.ws = null
      this.roomId = null
      this.users.clear()
      this.emit('disconnected', 'User disconnected')
    }
  }

  private startHeartbeat(): void {
    this.stopHeartbeat()
    this.heartbeatInterval = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.send({ type: 'heartbeat', timestamp: new Date() })
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
      console.error('[Collab] Max reconnect attempts reached')
      return
    }

    this.reconnectAttempts++
    const delay = Math.min(this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1), 10000)

    setTimeout(() => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        console.log(`[Collab] Reconnecting (${this.reconnectAttempts}/${this.maxReconnectAttempts})`)
        const token = localStorage.getItem('apex_access_token')
        if (token) {
          this.connect(token).catch(console.error)
        }
      }
    }, delay)
  }

  // Message handling
  private send(msg: Partial<CollabMessage>): void {
    if (this.ws?.readyState !== WebSocket.OPEN) {
      console.warn('[Collab] Cannot send: WebSocket not connected')
      return
    }

    this.ws.send(JSON.stringify({
      ...msg,
      timestamp: new Date().toISOString()
    }))
  }

  private handleMessage(data: string): void {
    try {
      const msg: CollabMessage = JSON.parse(data)
      msg.timestamp = new Date(msg.timestamp)

      switch (msg.type) {
        case 'room_joined':
          this.handleRoomJoined(msg)
          break
        case 'room_left':
          this.handleRoomLeft(msg)
          break
        case 'user_joined':
          this.handleUserJoined(msg)
          break
        case 'user_left':
          this.handleUserLeft(msg)
          break
        case 'user_list':
          this.handleUserList(msg)
          break
        case 'cursor_update':
          this.handleCursorUpdate(msg)
          break
        case 'selection_update':
          this.handleSelectionUpdate(msg)
          break
        case 'typing_start':
          this.handleTypingStart(msg)
          break
        case 'typing_stop':
          this.handleTypingStop(msg)
          break
        case 'follow_user':
          this.handleFollowUser(msg)
          break
        case 'stop_following':
          this.handleStopFollowing(msg)
          break
        case 'operation':
          this.handleOperation(msg)
          break
        case 'operation_ack':
          this.handleOperationAck(msg)
          break
        case 'sync_response':
          this.handleSyncResponse(msg)
          break
        case 'permission_update':
          this.handlePermissionUpdate(msg)
          break
        case 'user_kicked':
          this.handleUserKicked(msg)
          break
        case 'chat':
          this.handleChat(msg)
          break
        case 'rtc_offer':
          this.handleRTCOffer(msg)
          break
        case 'rtc_answer':
          this.handleRTCAnswer(msg)
          break
        case 'rtc_candidate':
          this.handleRTCCandidate(msg)
          break
        case 'media_state_change':
          this.handleMediaStateChange(msg)
          break
        case 'activity':
          this.handleActivity(msg)
          break
        case 'error':
          this.handleError(msg)
          break
        case 'heartbeat':
          // Heartbeat response, ignore
          break
        default:
          console.log('[Collab] Unknown message type:', msg.type)
      }
    } catch (error) {
      console.error('[Collab] Error parsing message:', error)
    }
  }

  // Message handlers
  private handleRoomJoined(msg: CollabMessage): void {
    this.roomId = msg.roomId || null
    this.emit('roomJoined', this.roomId)
  }

  private handleRoomLeft(msg: CollabMessage): void {
    this.roomId = null
    this.users.clear()
    this.emit('roomLeft')
  }

  private handleUserJoined(msg: CollabMessage): void {
    const presence = this.parsePresence(msg.data)
    this.users.set(presence.userId, presence)
    this.emit('userJoined', presence)
  }

  private handleUserLeft(msg: CollabMessage): void {
    const userId = msg.userId || msg.data?.user_id
    this.users.delete(userId)
    this.emit('userLeft', userId)
  }

  private handleUserList(msg: CollabMessage): void {
    const users = (msg.data as any[]).map(this.parsePresence)
    this.users.clear()
    users.forEach(u => this.users.set(u.userId, u))
    this.emit('userListUpdate', users)
  }

  private handleCursorUpdate(msg: CollabMessage): void {
    const presence = this.parsePresence(msg.data)
    const existing = this.users.get(presence.userId)
    if (existing) {
      Object.assign(existing, presence)
    } else {
      this.users.set(presence.userId, presence)
    }
    this.emit('cursorUpdate', presence)
  }

  private handleSelectionUpdate(msg: CollabMessage): void {
    const presence = this.parsePresence(msg.data)
    const existing = this.users.get(presence.userId)
    if (existing) {
      Object.assign(existing, presence)
    }
    this.emit('selectionUpdate', presence)
  }

  private handleTypingStart(msg: CollabMessage): void {
    const userId = msg.userId!
    const user = this.users.get(userId)
    if (user) {
      user.isTyping = true
    }
    this.emit('typingStart', userId, msg.username || '')
  }

  private handleTypingStop(msg: CollabMessage): void {
    const userId = msg.userId!
    const user = this.users.get(userId)
    if (user) {
      user.isTyping = false
    }
    this.emit('typingStop', userId, msg.username || '')
  }

  private handleFollowUser(msg: CollabMessage): void {
    const { following } = msg.data
    this.emit('followUpdate', msg.userId!, following)
  }

  private handleStopFollowing(msg: CollabMessage): void {
    this.emit('followUpdate', msg.userId!, null)
  }

  private handleOperation(msg: CollabMessage): void {
    const { operations, version, file_id } = msg.data
    this.documentVersions.set(file_id, version)
    this.emit('operation', file_id, operations, version, msg.userId!)
  }

  private handleOperationAck(msg: CollabMessage): void {
    const { version } = msg.data
    this.emit('operationAck', version)
  }

  private handleSyncResponse(msg: CollabMessage): void {
    const { id, content, version } = msg.data
    this.documentVersions.set(id, version)
    this.emit('syncResponse', id, content, version)
  }

  private handlePermissionUpdate(msg: CollabMessage): void {
    const { user_id, permission } = msg.data
    const user = this.users.get(user_id)
    if (user) {
      user.permission = permission
    }
    this.emit('permissionUpdate', user_id, permission)
  }

  private handleUserKicked(msg: CollabMessage): void {
    const userId = msg.userId!
    if (userId === this.userId) {
      this.roomId = null
      this.users.clear()
      this.emit('kicked')
    } else {
      this.users.delete(userId)
      this.emit('userKicked', userId, msg.data?.kicked_by)
    }
  }

  private handleChat(msg: CollabMessage): void {
    const chatMsg: ChatMessage = {
      id: msg.data.id,
      userId: msg.data.user_id,
      username: msg.data.username,
      avatarUrl: msg.data.avatar_url,
      message: msg.data.message,
      type: msg.data.type || 'text',
      timestamp: new Date(msg.data.timestamp)
    }
    this.emit('chatMessage', chatMsg)
  }

  private handleRTCOffer(msg: CollabMessage): void {
    this.emit('rtcOffer', msg.data.from_user_id, msg.data.payload)
  }

  private handleRTCAnswer(msg: CollabMessage): void {
    this.emit('rtcAnswer', msg.data.from_user_id, msg.data.payload)
  }

  private handleRTCCandidate(msg: CollabMessage): void {
    this.emit('rtcCandidate', msg.data.from_user_id, msg.data.payload)
  }

  private handleMediaStateChange(msg: CollabMessage): void {
    this.emit('mediaStateChange', msg.userId!, msg.data)
  }

  private handleActivity(msg: CollabMessage): void {
    const activity: ActivityFeedItem = {
      id: msg.data.id,
      userId: msg.data.user_id,
      username: msg.data.username,
      avatarUrl: msg.data.avatar_url,
      action: msg.data.action,
      target: msg.data.target,
      timestamp: new Date(msg.data.timestamp)
    }
    this.emit('activityUpdate', activity)
  }

  private handleError(msg: CollabMessage): void {
    console.error('[Collab] Server error:', msg.data?.error)
    this.emit('error', msg.data?.error || 'Unknown error')
  }

  private parsePresence(data: any): UserPresence {
    return {
      userId: data.user_id,
      username: data.username,
      email: data.email,
      avatarUrl: data.avatar_url,
      color: data.color,
      fileId: data.file_id,
      fileName: data.file_name,
      cursor: data.cursor ? {
        line: data.cursor.line,
        column: data.cursor.column,
        updatedAt: new Date(data.cursor.updated_at)
      } : undefined,
      selection: data.selection ? {
        startLine: data.selection.start_line,
        startColumn: data.selection.start_column,
        endLine: data.selection.end_line,
        endColumn: data.selection.end_column,
        updatedAt: new Date(data.selection.updated_at)
      } : undefined,
      isTyping: data.is_typing || false,
      lastActivity: new Date(data.last_activity),
      isFollowing: data.is_following,
      permission: data.permission || 'viewer',
      status: data.status || 'online'
    }
  }

  // Public API
  setUser(userId: number, username: string): void {
    this.userId = userId
    this.username = username
  }

  joinRoom(roomId: string, projectId?: number): void {
    this.send({
      type: 'join_room',
      data: { room_id: roomId, project_id: projectId }
    })
  }

  leaveRoom(): void {
    if (this.roomId) {
      this.send({ type: 'leave_room' })
    }
  }

  updateCursor(fileId: number, fileName: string, line: number, column: number): void {
    this.send({
      type: 'cursor_update',
      data: { file_id: fileId, file_name: fileName, line, column }
    })
  }

  updateSelection(startLine: number, startColumn: number, endLine: number, endColumn: number): void {
    this.send({
      type: 'selection_update',
      data: { start_line: startLine, start_column: startColumn, end_line: endLine, end_column: endColumn }
    })
  }

  clearSelection(): void {
    this.send({
      type: 'selection_update',
      data: { clear: true }
    })
  }

  startTyping(): void {
    this.send({ type: 'typing_start' })

    // Auto-stop typing after 2 seconds
    if (this.typingTimeout) {
      clearTimeout(this.typingTimeout)
    }
    this.typingTimeout = setTimeout(() => {
      this.stopTyping()
    }, 2000)
  }

  stopTyping(): void {
    if (this.typingTimeout) {
      clearTimeout(this.typingTimeout)
      this.typingTimeout = null
    }
    this.send({ type: 'typing_stop' })
  }

  followUser(targetUserId: number): void {
    this.send({
      type: 'follow_user',
      data: { target_user_id: targetUserId }
    })
  }

  stopFollowing(): void {
    this.send({ type: 'stop_following' })
  }

  sendOperation(fileId: number, operations: Operation[]): void {
    const version = this.documentVersions.get(fileId) || 0

    // Store pending operation
    if (!this.pendingOperations.has(fileId)) {
      this.pendingOperations.set(fileId, [])
    }

    const op: TextOperation = {
      operations,
      baseVersion: version,
      userId: this.userId,
      fileId,
      timestamp: new Date()
    }

    this.pendingOperations.get(fileId)!.push(op)

    this.send({
      type: 'operation',
      data: op
    })
  }

  requestSync(fileId: number): void {
    const version = this.documentVersions.get(fileId) || 0
    this.send({
      type: 'sync_request',
      data: { file_id: fileId, version }
    })
  }

  sendChatMessage(message: string, type: 'text' | 'code' = 'text'): void {
    this.send({
      type: 'chat',
      data: { message, type }
    })
  }

  updatePermission(targetUserId: number, permission: PermissionLevel): void {
    this.send({
      type: 'permission_update',
      data: { target_user_id: targetUserId, permission }
    })
  }

  kickUser(targetUserId: number): void {
    this.send({
      type: 'kick_user',
      data: { target_user_id: targetUserId }
    })
  }

  // WebRTC signaling
  sendRTCOffer(targetUserId: number, offer: RTCSessionDescriptionInit): void {
    this.send({
      type: 'rtc_offer',
      data: { target_user_id: targetUserId, payload: offer }
    })
  }

  sendRTCAnswer(targetUserId: number, answer: RTCSessionDescriptionInit): void {
    this.send({
      type: 'rtc_answer',
      data: { target_user_id: targetUserId, payload: answer }
    })
  }

  sendRTCCandidate(targetUserId: number, candidate: RTCIceCandidateInit): void {
    this.send({
      type: 'rtc_candidate',
      data: { target_user_id: targetUserId, payload: candidate }
    })
  }

  sendMediaStateChange(state: Partial<RTCPeerState>): void {
    this.send({
      type: 'media_state_change',
      data: state
    })
  }

  // Getters
  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }

  getCurrentRoomId(): string | null {
    return this.roomId
  }

  getUsers(): UserPresence[] {
    return Array.from(this.users.values())
  }

  getUser(userId: number): UserPresence | undefined {
    return this.users.get(userId)
  }

  getDocumentVersion(fileId: number): number {
    return this.documentVersions.get(fileId) || 0
  }
}

// Create singleton instance
export const collaborationService = new CollaborationService()
export default collaborationService
