// APEX.BUILD Debug Service
// Frontend service for interacting with the backend debugging API
// Supports JavaScript, TypeScript, Python, and Go debugging

import apiService, {
  DebugSession,
  DebugSessionStatus,
  DebugBreakpoint,
  DebugBreakpointType,
  DebugStackFrame,
  DebugVariable,
  DebugWatchExpression,
  DebugEvaluateResult,
  DebugEvent,
  DebugEventType,
} from './api'

// Re-export types for consumers
export type {
  DebugSession,
  DebugSessionStatus,
  DebugBreakpoint,
  DebugBreakpointType,
  DebugStackFrame,
  DebugVariable,
  DebugWatchExpression,
  DebugEvaluateResult,
  DebugEvent,
  DebugEventType,
}

// Also export local types for backward compatibility
export type { Scope, Variable, StackFrame, Breakpoint, WatchExpression, EvaluateResult, BreakpointType }

// Scope interface
interface Scope {
  type: 'local' | 'closure' | 'global' | 'with' | 'catch' | 'block' | 'script'
  name?: string
  start_line?: number
  end_line?: number
  variables?: Variable[]
}

// Variable interface
interface Variable {
  name: string
  value: string
  type: string
  object_id?: string
  has_children: boolean
  children?: Variable[]
  preview?: string
}

// Stack frame interface (local alias)
interface StackFrame {
  id: string
  index: number
  function_name: string
  file_path: string
  line: number
  column: number
  script_id?: string
  is_async: boolean
  scopes?: Scope[]
  local_vars?: Record<string, string>
}

// Breakpoint interface (local alias)
interface Breakpoint {
  id: string
  session_id: string
  file_id: number
  file_path: string
  line: number
  column: number
  type: BreakpointType
  condition?: string
  log_message?: string
  hit_count: number
  enabled: boolean
  verified: boolean
  breakpoint_id?: string
}

type BreakpointType = DebugBreakpointType

// Watch expression interface (local alias)
interface WatchExpression {
  id: string
  expression: string
  value?: string
  type?: string
  error?: string
}

// Evaluate result (local alias)
interface EvaluateResult {
  value: string
  type: string
  object_id?: string
  has_children: boolean
  preview?: string
  error?: string
}

// Paused event data
export interface PausedEventData {
  reason: 'breakpoint' | 'exception' | 'step' | 'debugger_statement' | 'pause'
  breakpoint?: Breakpoint
  call_stack: StackFrame[]
  exception?: Variable
  hit_breakpoint_ids?: string[]
}

// Debug service event callbacks
export type DebugEventCallback = (event: DebugEvent) => void

class DebugService {
  private eventListeners: Map<DebugEventType, Set<DebugEventCallback>> = new Map()
  private activeSession: DebugSession | null = null
  private websocket: WebSocket | null = null
  private breakpoints: Map<string, Breakpoint> = new Map()
  private watchExpressions: Map<string, WatchExpression> = new Map()
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000

  constructor() {
    // Initialize event listener maps
    const eventTypes: DebugEventType[] = [
      'session_started', 'session_stopped', 'paused', 'resumed',
      'stepping', 'breakpoint_hit', 'breakpoint_verified',
      'breakpoint_added', 'breakpoint_removed', 'exception',
      'output', 'error'
    ]
    eventTypes.forEach(type => {
      this.eventListeners.set(type, new Set())
    })
  }

  // Start a new debug session
  async startSession(
    projectId: number,
    entryFile: string,
    language: string
  ): Promise<DebugSession> {
    try {
      const response = await apiService.startDebugSession({
        project_id: projectId,
        entry_point: entryFile,
        language: language
      })

      this.activeSession = response.session

      // Sync any pending breakpoints
      await this.syncPendingBreakpoints()

      // Connect to debug WebSocket
      await this.connectWebSocket(response.session.id)

      this.emit('session_started', { session: this.activeSession })
      return this.activeSession!
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
      throw error
    }
  }

  // Stop the current debug session
  async stopSession(sessionId?: string): Promise<void> {
    const sid = sessionId || this.activeSession?.id
    if (!sid) return

    try {
      await apiService.stopDebugSession(sid)

      this.disconnectWebSocket()
      this.activeSession = null
      // Only clear server-synced breakpoints, keep temp ones
      this.breakpoints.forEach((bp, id) => {
        if (!id.startsWith('temp-')) {
          this.breakpoints.delete(id)
        }
      })
      this.watchExpressions.clear()

      this.emit('session_stopped', { session_id: sid })
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
      throw error
    }
  }

  // Get current session
  getSession(): DebugSession | null {
    return this.activeSession
  }

  // Sync pending (temp) breakpoints to the server
  private async syncPendingBreakpoints(): Promise<void> {
    if (!this.activeSession) return

    const pendingBps = Array.from(this.breakpoints.values()).filter(bp =>
      bp.id.startsWith('temp-')
    )

    for (const tempBp of pendingBps) {
      try {
        const response = await apiService.setDebugBreakpoint(
          this.activeSession.id,
          {
            file_id: tempBp.file_id,
            file_path: tempBp.file_path,
            line: tempBp.line,
            type: tempBp.type,
            condition: tempBp.condition
          }
        )

        // Replace temp breakpoint with server breakpoint
        this.breakpoints.delete(tempBp.id)
        this.breakpoints.set(response.breakpoint.id, response.breakpoint as Breakpoint)
      } catch (error) {
        console.error('Failed to sync breakpoint:', error)
      }
    }
  }

  // Set a breakpoint
  async setBreakpoint(
    fileId: number,
    filePath: string,
    line: number,
    condition?: string,
    type: BreakpointType = 'line'
  ): Promise<Breakpoint> {
    if (!this.activeSession) {
      // Store breakpoint locally for when session starts
      const tempBp: Breakpoint = {
        id: `temp-${Date.now()}-${line}`,
        session_id: '',
        file_id: fileId,
        file_path: filePath,
        line,
        column: 0,
        type,
        condition,
        hit_count: 0,
        enabled: true,
        verified: false
      }
      this.breakpoints.set(tempBp.id, tempBp)
      this.emit('breakpoint_added', { breakpoint: tempBp })
      return tempBp
    }

    try {
      const response = await apiService.setDebugBreakpoint(
        this.activeSession.id,
        {
          file_id: fileId,
          file_path: filePath,
          line,
          type,
          condition
        }
      )

      const breakpoint = response.breakpoint as Breakpoint
      this.breakpoints.set(breakpoint.id, breakpoint)

      this.emit('breakpoint_added', { breakpoint })
      return breakpoint
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
      throw error
    }
  }

  // Remove a breakpoint
  async removeBreakpoint(breakpointId: string): Promise<void> {
    const breakpoint = this.breakpoints.get(breakpointId)
    if (!breakpoint) return

    if (this.activeSession && !breakpointId.startsWith('temp-')) {
      try {
        await apiService.removeDebugBreakpoint(this.activeSession.id, breakpointId)
      } catch (error) {
        this.emit('error', { error: (error as Error).message })
        throw error
      }
    }

    this.breakpoints.delete(breakpointId)
    this.emit('breakpoint_removed', { breakpoint_id: breakpointId })
  }

  // Toggle breakpoint enabled state
  async toggleBreakpoint(breakpointId: string, enabled: boolean): Promise<void> {
    const breakpoint = this.breakpoints.get(breakpointId)
    if (!breakpoint) return

    breakpoint.enabled = enabled
    this.breakpoints.set(breakpointId, breakpoint)

    if (this.activeSession && !breakpointId.startsWith('temp-')) {
      try {
        await apiService.toggleDebugBreakpoint(
          this.activeSession.id,
          breakpointId,
          enabled
        )
      } catch (error) {
        this.emit('error', { error: (error as Error).message })
      }
    }
  }

  // Get all breakpoints
  getBreakpoints(): Breakpoint[] {
    return Array.from(this.breakpoints.values())
  }

  // Get breakpoints for a specific file
  getBreakpointsForFile(fileId: number): Breakpoint[] {
    return Array.from(this.breakpoints.values()).filter(bp => bp.file_id === fileId)
  }

  // Continue execution
  async continue(): Promise<void> {
    if (!this.activeSession) return

    try {
      await apiService.debugContinue(this.activeSession.id)
      if (this.activeSession) {
        this.activeSession.status = 'running'
      }
      this.emit('resumed', {})
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
    }
  }

  // Step over
  async stepOver(): Promise<void> {
    if (!this.activeSession) return

    try {
      await apiService.debugStepOver(this.activeSession.id)
      this.emit('stepping', { step_type: 'over' })
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
    }
  }

  // Step into
  async stepInto(): Promise<void> {
    if (!this.activeSession) return

    try {
      await apiService.debugStepInto(this.activeSession.id)
      this.emit('stepping', { step_type: 'into' })
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
    }
  }

  // Step out
  async stepOut(): Promise<void> {
    if (!this.activeSession) return

    try {
      await apiService.debugStepOut(this.activeSession.id)
      this.emit('stepping', { step_type: 'out' })
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
    }
  }

  // Pause execution
  async pause(): Promise<void> {
    if (!this.activeSession) return

    try {
      await apiService.debugPause(this.activeSession.id)
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
    }
  }

  // Evaluate an expression
  async evaluate(expression: string, _frameId?: string): Promise<EvaluateResult> {
    if (!this.activeSession) {
      return {
        value: '<no active session>',
        type: 'undefined',
        has_children: false,
        error: 'No active debug session'
      }
    }

    try {
      const response = await apiService.evaluateDebugExpression(
        this.activeSession.id,
        expression
      )
      return response.result as EvaluateResult
    } catch (error) {
      return {
        value: '',
        type: 'error',
        has_children: false,
        error: (error as Error).message
      }
    }
  }

  // Get variables for current scope
  async getVariables(scopeType?: string): Promise<Variable[]> {
    if (!this.activeSession) return []

    try {
      // Use scope type as object ID for root-level variable fetch
      const objectId = scopeType || 'local'
      const response = await apiService.getDebugVariables(
        this.activeSession.id,
        objectId
      )
      return response.variables as Variable[]
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
      return []
    }
  }

  // Expand a variable to get its children
  async expandVariable(objectId: string): Promise<Variable[]> {
    if (!this.activeSession) return []

    try {
      const response = await apiService.getDebugVariables(
        this.activeSession.id,
        objectId
      )
      return response.variables as Variable[]
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
      return []
    }
  }

  // Get call stack
  async getCallStack(): Promise<StackFrame[]> {
    if (!this.activeSession) return []

    try {
      const response = await apiService.getDebugCallStack(this.activeSession.id)
      return response.call_stack as StackFrame[]
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
      return []
    }
  }

  // Add watch expression
  addWatch(expression: string): WatchExpression {
    const watch: WatchExpression = {
      id: `watch-${Date.now()}`,
      expression,
      value: '<not evaluated>',
      type: 'undefined'
    }
    this.watchExpressions.set(watch.id, watch)

    // If we have an active session and it's paused, sync to server
    if (this.activeSession?.status === 'paused') {
      this.syncWatchToServer(watch)
    }

    return watch
  }

  // Sync watch expression to server
  private async syncWatchToServer(watch: WatchExpression): Promise<void> {
    if (!this.activeSession) return

    try {
      const response = await apiService.addDebugWatch(
        this.activeSession.id,
        watch.expression
      )
      // Update the local watch with server data
      const serverWatch = response.watch
      watch.id = serverWatch.id
      watch.value = serverWatch.value
      watch.type = serverWatch.type
      watch.error = serverWatch.error
      this.watchExpressions.set(watch.id, watch)
    } catch (error) {
      // Just evaluate locally
      this.evaluateWatch(watch.id)
    }
  }

  // Remove watch expression
  removeWatch(watchId: string): void {
    const watch = this.watchExpressions.get(watchId)
    if (!watch) return

    // Try to remove from server if we have an active session
    if (this.activeSession && !watchId.startsWith('watch-')) {
      apiService.removeDebugWatch(this.activeSession.id, watchId).catch(() => {
        // Ignore errors for local watches
      })
    }

    this.watchExpressions.delete(watchId)
  }

  // Get all watch expressions
  getWatches(): WatchExpression[] {
    return Array.from(this.watchExpressions.values())
  }

  // Evaluate a watch expression
  async evaluateWatch(watchId: string): Promise<void> {
    const watch = this.watchExpressions.get(watchId)
    if (!watch) return

    const result = await this.evaluate(watch.expression)
    watch.value = result.value
    watch.type = result.type
    watch.error = result.error
    this.watchExpressions.set(watchId, watch)
  }

  // Evaluate all watch expressions
  async evaluateAllWatches(): Promise<void> {
    const promises = Array.from(this.watchExpressions.keys()).map(id =>
      this.evaluateWatch(id)
    )
    await Promise.all(promises)
  }

  // Event listener management
  on(event: DebugEventType, callback: DebugEventCallback): () => void {
    const listeners = this.eventListeners.get(event)
    if (listeners) {
      listeners.add(callback)
    }
    return () => {
      listeners?.delete(callback)
    }
  }

  off(event: DebugEventType, callback?: DebugEventCallback): void {
    const listeners = this.eventListeners.get(event)
    if (listeners) {
      if (callback) {
        listeners.delete(callback)
      } else {
        listeners.clear()
      }
    }
  }

  private emit(type: DebugEventType, data: any): void {
    const event: DebugEvent = {
      type,
      timestamp: new Date().toISOString(),
      data
    }

    const listeners = this.eventListeners.get(type)
    if (listeners) {
      listeners.forEach(callback => {
        try {
          callback(event)
        } catch (error) {
          console.error(`Error in debug event listener for ${type}:`, error)
        }
      })
    }
  }

  // WebSocket connection for real-time debug events
  private async connectWebSocket(sessionId: string): Promise<void> {
    return new Promise((resolve, reject) => {
      const wsUrl = apiService.getDebugWebSocketUrl(sessionId)

      this.websocket = new WebSocket(wsUrl)

      this.websocket.onopen = () => {
        console.log('Debug WebSocket connected')
        this.reconnectAttempts = 0
        resolve()
      }

      this.websocket.onerror = (error) => {
        console.error('Debug WebSocket error:', error)
        reject(error)
      }

      this.websocket.onclose = (event) => {
        console.log('Debug WebSocket closed:', event.code, event.reason)
        this.websocket = null

        // Attempt reconnection if session is still active
        if (this.activeSession && this.reconnectAttempts < this.maxReconnectAttempts) {
          this.reconnectAttempts++
          const delay = this.reconnectDelay * this.reconnectAttempts
          console.log(`Attempting to reconnect in ${delay}ms...`)
          setTimeout(() => {
            if (this.activeSession) {
              this.connectWebSocket(this.activeSession.id).catch(() => {
                // Reconnection failed
              })
            }
          }, delay)
        }
      }

      this.websocket.onmessage = (event) => {
        this.handleWebSocketMessage(event.data)
      }
    })
  }

  private disconnectWebSocket(): void {
    if (this.websocket) {
      this.websocket.close()
      this.websocket = null
    }
    this.reconnectAttempts = 0
  }

  private handleWebSocketMessage(data: string): void {
    try {
      const message = JSON.parse(data)

      switch (message.type) {
        case 'paused':
          if (this.activeSession) {
            this.activeSession.status = 'paused'
            this.activeSession.current_line = message.data.line
            this.activeSession.current_file = message.data.file
          }
          this.evaluateAllWatches()
          this.emit('paused', message.data as PausedEventData)
          break

        case 'resumed':
          if (this.activeSession) {
            this.activeSession.status = 'running'
          }
          this.emit('resumed', {})
          break

        case 'breakpoint_verified':
          const bp = this.breakpoints.get(message.data.id)
          if (bp) {
            bp.verified = true
            bp.breakpoint_id = message.data.breakpoint_id
            this.breakpoints.set(bp.id, bp)
          }
          this.emit('breakpoint_verified', message.data)
          break

        case 'breakpoint_hit':
          this.emit('breakpoint_hit', message.data)
          break

        case 'output':
          this.emit('output', message.data)
          break

        case 'exception':
          this.emit('exception', message.data)
          break

        case 'error':
          this.emit('error', message.data)
          break

        case 'session_stopped':
          this.activeSession = null
          this.disconnectWebSocket()
          this.emit('session_stopped', message.data)
          break

        default:
          console.log('Unknown debug message type:', message.type)
      }
    } catch (error) {
      console.error('Error parsing debug message:', error)
    }
  }

  // Check if debugger is active
  isActive(): boolean {
    return this.activeSession !== null
  }

  // Check if debugger is paused
  isPaused(): boolean {
    return this.activeSession?.status === 'paused'
  }

  // Check if debugger is running
  isRunning(): boolean {
    return this.activeSession?.status === 'running'
  }
}

// Create singleton instance
export const debugService = new DebugService()

// Export for easy importing
export default debugService
