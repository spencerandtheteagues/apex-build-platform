// APEX.BUILD Debug Service
// Frontend service for interacting with the backend debugging API
// Supports JavaScript, TypeScript, Python, and Go debugging

import apiService from './api'

// Debug session status matching backend
export type DebugSessionStatus = 'pending' | 'running' | 'paused' | 'completed' | 'error'

// Breakpoint types matching backend
export type BreakpointType = 'line' | 'conditional' | 'logpoint' | 'exception' | 'function'

// Debug session interface
export interface DebugSession {
  id: string
  project_id: number
  user_id: number
  file_id: number
  status: DebugSessionStatus
  language: string
  entry_point: string
  working_directory: string
  debug_port: number
  devtools_url?: string
  process_id?: number
  error_message?: string
  started_at?: string
  ended_at?: string
  current_line?: number
  current_file?: string
}

// Breakpoint interface
export interface Breakpoint {
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
  breakpoint_id?: string // CDP breakpoint ID
}

// Stack frame interface
export interface StackFrame {
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

// Scope interface
export interface Scope {
  type: 'local' | 'closure' | 'global' | 'with' | 'catch' | 'block' | 'script'
  name?: string
  start_line?: number
  end_line?: number
  variables?: Variable[]
}

// Variable interface
export interface Variable {
  name: string
  value: string
  type: string
  object_id?: string
  has_children: boolean
  children?: Variable[]
  preview?: string
}

// Watch expression interface
export interface WatchExpression {
  id: string
  expression: string
  value?: string
  type?: string
  error?: string
}

// Debug event interface
export interface DebugEvent {
  type: DebugEventType
  timestamp: string
  data: any
}

export type DebugEventType =
  | 'session_started'
  | 'session_stopped'
  | 'paused'
  | 'resumed'
  | 'stepping'
  | 'breakpoint_hit'
  | 'breakpoint_verified'
  | 'breakpoint_added'
  | 'breakpoint_removed'
  | 'exception'
  | 'output'
  | 'error'

// Paused event data
export interface PausedEventData {
  reason: 'breakpoint' | 'exception' | 'step' | 'debugger_statement' | 'pause'
  breakpoint?: Breakpoint
  call_stack: StackFrame[]
  exception?: Variable
  hit_breakpoint_ids?: string[]
}

// Evaluate result
export interface EvaluateResult {
  value: string
  type: string
  object_id?: string
  has_children: boolean
  preview?: string
  error?: string
}

// Debug service event callbacks
export type DebugEventCallback = (event: DebugEvent) => void

class DebugService {
  private eventListeners: Map<DebugEventType, Set<DebugEventCallback>> = new Map()
  private activeSession: DebugSession | null = null
  private websocket: WebSocket | null = null
  private breakpoints: Map<string, Breakpoint> = new Map()
  private watchExpressions: Map<string, WatchExpression> = new Map()
  private pendingCallbacks: Map<number, (data: any) => void> = new Map()
  private messageId = 0

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
      const response = await fetch(`/api/v1/debug/sessions`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('apex_access_token')}`
        },
        body: JSON.stringify({
          project_id: projectId,
          entry_point: entryFile,
          language: language
        })
      })

      if (!response.ok) {
        throw new Error(`Failed to start debug session: ${response.statusText}`)
      }

      const data = await response.json()
      this.activeSession = data.session

      // Connect to debug WebSocket
      await this.connectWebSocket(data.session.id)

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
      await fetch(`/api/v1/debug/sessions/${sid}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('apex_access_token')}`
        }
      })

      this.disconnectWebSocket()
      this.activeSession = null
      this.breakpoints.clear()
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
      const response = await fetch(`/api/v1/debug/sessions/${this.activeSession.id}/breakpoints`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('apex_access_token')}`
        },
        body: JSON.stringify({
          file_id: fileId,
          file_path: filePath,
          line,
          type,
          condition
        })
      })

      if (!response.ok) {
        throw new Error(`Failed to set breakpoint: ${response.statusText}`)
      }

      const data = await response.json()
      const breakpoint = data.breakpoint as Breakpoint
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
        await fetch(`/api/v1/debug/sessions/${this.activeSession.id}/breakpoints/${breakpointId}`, {
          method: 'DELETE',
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('apex_access_token')}`
          }
        })
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
        await fetch(`/api/v1/debug/sessions/${this.activeSession.id}/breakpoints/${breakpointId}/toggle`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('apex_access_token')}`
          },
          body: JSON.stringify({ enabled })
        })
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
      await this.sendDebugCommand('continue')
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
      await this.sendDebugCommand('stepOver')
      this.emit('stepping', { step_type: 'over' })
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
    }
  }

  // Step into
  async stepInto(): Promise<void> {
    if (!this.activeSession) return

    try {
      await this.sendDebugCommand('stepInto')
      this.emit('stepping', { step_type: 'into' })
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
    }
  }

  // Step out
  async stepOut(): Promise<void> {
    if (!this.activeSession) return

    try {
      await this.sendDebugCommand('stepOut')
      this.emit('stepping', { step_type: 'out' })
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
    }
  }

  // Pause execution
  async pause(): Promise<void> {
    if (!this.activeSession) return

    try {
      await this.sendDebugCommand('pause')
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
    }
  }

  // Evaluate an expression
  async evaluate(expression: string, frameId?: string): Promise<EvaluateResult> {
    if (!this.activeSession) {
      return {
        value: '<no active session>',
        type: 'undefined',
        has_children: false,
        error: 'No active debug session'
      }
    }

    try {
      const response = await fetch(`/api/v1/debug/sessions/${this.activeSession.id}/evaluate`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('apex_access_token')}`
        },
        body: JSON.stringify({
          expression,
          frame_id: frameId
        })
      })

      if (!response.ok) {
        throw new Error(`Failed to evaluate: ${response.statusText}`)
      }

      const data = await response.json()
      return data.result as EvaluateResult
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
      const url = scopeType
        ? `/api/v1/debug/sessions/${this.activeSession.id}/variables?scope=${scopeType}`
        : `/api/v1/debug/sessions/${this.activeSession.id}/variables`

      const response = await fetch(url, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('apex_access_token')}`
        }
      })

      if (!response.ok) {
        throw new Error(`Failed to get variables: ${response.statusText}`)
      }

      const data = await response.json()
      return data.variables as Variable[]
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
      return []
    }
  }

  // Expand a variable to get its children
  async expandVariable(objectId: string): Promise<Variable[]> {
    if (!this.activeSession) return []

    try {
      const response = await fetch(
        `/api/v1/debug/sessions/${this.activeSession.id}/variables/${objectId}`,
        {
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('apex_access_token')}`
          }
        }
      )

      if (!response.ok) {
        throw new Error(`Failed to expand variable: ${response.statusText}`)
      }

      const data = await response.json()
      return data.children as Variable[]
    } catch (error) {
      this.emit('error', { error: (error as Error).message })
      return []
    }
  }

  // Get call stack
  async getCallStack(): Promise<StackFrame[]> {
    if (!this.activeSession) return []

    try {
      const response = await fetch(`/api/v1/debug/sessions/${this.activeSession.id}/callstack`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('apex_access_token')}`
        }
      })

      if (!response.ok) {
        throw new Error(`Failed to get call stack: ${response.statusText}`)
      }

      const data = await response.json()
      return data.frames as StackFrame[]
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

    // Evaluate immediately if session is paused
    if (this.activeSession?.status === 'paused') {
      this.evaluateWatch(watch.id)
    }

    return watch
  }

  // Remove watch expression
  removeWatch(watchId: string): void {
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
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const wsUrl = `${protocol}//${window.location.host}/api/v1/debug/sessions/${sessionId}/ws`

      this.websocket = new WebSocket(wsUrl)

      this.websocket.onopen = () => {
        console.log('Debug WebSocket connected')
        resolve()
      }

      this.websocket.onerror = (error) => {
        console.error('Debug WebSocket error:', error)
        reject(error)
      }

      this.websocket.onclose = () => {
        console.log('Debug WebSocket closed')
        this.websocket = null
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

        case 'output':
          this.emit('output', message.data)
          break

        case 'exception':
          this.emit('exception', message.data)
          break

        case 'error':
          this.emit('error', message.data)
          break

        default:
          console.log('Unknown debug message type:', message.type)
      }
    } catch (error) {
      console.error('Error parsing debug message:', error)
    }
  }

  private async sendDebugCommand(command: string, params?: any): Promise<any> {
    if (!this.activeSession) {
      throw new Error('No active debug session')
    }

    const response = await fetch(`/api/v1/debug/sessions/${this.activeSession.id}/command`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('apex_access_token')}`
      },
      body: JSON.stringify({
        command,
        params
      })
    })

    if (!response.ok) {
      throw new Error(`Debug command failed: ${response.statusText}`)
    }

    return response.json()
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
