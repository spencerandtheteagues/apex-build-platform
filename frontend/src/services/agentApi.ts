// APEX.BUILD Agent API Service
// WebSocket connection and API calls for the AI Autonomous Agent

import {
  AgentTask,
  AgentStatus,
  AgentStep,
  AgentWSMessage,
  AgentWSMessageType,
  AgentStatusUpdate,
  AgentStepUpdate,
  AgentFileChangeUpdate,
  AgentTerminalUpdate,
  AgentCheckpointUpdate,
  AgentMessageUpdate,
  StartAgentRequest,
  StartAgentResponse,
  AgentCheckpoint,
  FileChange,
  TerminalEntry,
} from '@/types/agent'
import { apiService } from './api'
import { generateId } from '@/lib/utils'

// Event handler types
export type AgentEventHandler<T = any> = (data: T) => void

export interface AgentEventHandlers {
  onConnected?: AgentEventHandler<void>
  onDisconnected?: AgentEventHandler<void>
  onStatusUpdate?: AgentEventHandler<AgentStatusUpdate>
  onStepStart?: AgentEventHandler<AgentStepUpdate>
  onStepProgress?: AgentEventHandler<AgentStepUpdate>
  onStepComplete?: AgentEventHandler<AgentStepUpdate>
  onStepFailed?: AgentEventHandler<AgentStepUpdate>
  onFileChange?: AgentEventHandler<AgentFileChangeUpdate>
  onTerminalOutput?: AgentEventHandler<AgentTerminalUpdate>
  onCheckpointCreated?: AgentEventHandler<AgentCheckpointUpdate>
  onAgentMessage?: AgentEventHandler<AgentMessageUpdate>
  onError?: AgentEventHandler<string>
  onCompleted?: AgentEventHandler<AgentTask>
  onCancelled?: AgentEventHandler<void>
}

export class AgentApiService {
  private ws: WebSocket | null = null
  private buildId: string | null = null
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000
  private handlers: AgentEventHandlers = {}
  private isConnecting = false
  private heartbeatInterval: NodeJS.Timeout | null = null
  private messageQueue: string[] = []

  // Start a new build task
  async startBuild(request: StartAgentRequest): Promise<StartAgentResponse> {
    const response = await apiService.startBuild({
      description: request.description,
      mode: request.mode,
    })

    this.buildId = response.build_id
    return {
      ...response,
      status: response.status as AgentStatus
    }
  }

  // Connect to WebSocket for real-time updates
  async connect(buildId: string, handlers: AgentEventHandlers): Promise<void> {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.disconnect()
    }

    this.buildId = buildId
    this.handlers = handlers
    this.isConnecting = true

    return new Promise((resolve, reject) => {
      const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const token = localStorage.getItem('apex_access_token')
      const baseUrl = `${wsProtocol}//${window.location.host}/ws/build/${buildId}`
      const wsUrl = token ? `${baseUrl}?token=${encodeURIComponent(token)}` : baseUrl

      try {
        this.ws = new WebSocket(wsUrl)
      } catch (error) {
        this.isConnecting = false
        reject(error)
        return
      }

      const timeout = setTimeout(() => {
        this.isConnecting = false
        reject(new Error('WebSocket connection timeout'))
      }, 10000)

      this.ws.onopen = () => {
        clearTimeout(timeout)
        this.isConnecting = false
        this.reconnectAttempts = 0
        this.startHeartbeat()
        this.handlers.onConnected?.()
        this.flushMessageQueue()
        resolve()
      }

      this.ws.onmessage = (event) => {
        try {
          const message: AgentWSMessage = JSON.parse(event.data)
          this.handleMessage(message)
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error)
        }
      }

      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error)
        this.handlers.onError?.('WebSocket connection error')
      }

      this.ws.onclose = (event) => {
        clearTimeout(timeout)
        this.stopHeartbeat()
        this.isConnecting = false

        if (event.code !== 1000) {
          // Abnormal closure - attempt reconnect
          this.scheduleReconnect()
        }

        this.handlers.onDisconnected?.()
      }
    })
  }

  // Handle incoming WebSocket messages
  private handleMessage(message: AgentWSMessage): void {
    switch (message.type) {
      case 'connected':
        // Already handled in onopen
        break

      case 'status_update':
        this.handlers.onStatusUpdate?.(message.data as AgentStatusUpdate)
        break

      case 'step_start':
        this.handlers.onStepStart?.(message.data as AgentStepUpdate)
        break

      case 'step_progress':
        this.handlers.onStepProgress?.(message.data as AgentStepUpdate)
        break

      case 'step_complete':
        this.handlers.onStepComplete?.(message.data as AgentStepUpdate)
        break

      case 'step_failed':
        this.handlers.onStepFailed?.(message.data as AgentStepUpdate)
        break

      case 'file_change':
        this.handlers.onFileChange?.(message.data as AgentFileChangeUpdate)
        break

      case 'terminal_output':
        this.handlers.onTerminalOutput?.(message.data as AgentTerminalUpdate)
        break

      case 'checkpoint_created':
        this.handlers.onCheckpointCreated?.(message.data as AgentCheckpointUpdate)
        break

      case 'agent_message':
        this.handlers.onAgentMessage?.(message.data as AgentMessageUpdate)
        break

      case 'error':
        this.handlers.onError?.(message.data.error || 'Unknown error')
        break

      case 'completed':
        this.handlers.onCompleted?.(message.data as AgentTask)
        break

      case 'cancelled':
        this.handlers.onCancelled?.()
        break

      default:
        console.warn('Unknown WebSocket message type:', message.type)
    }
  }

  // Send a message to the agent
  sendMessage(message: string): void {
    const payload = JSON.stringify({
      type: 'user_message',
      data: { message },
    })

    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(payload)
    } else {
      this.messageQueue.push(payload)
    }
  }

  // Pause the build
  async pause(): Promise<void> {
    if (this.buildId) {
      this.sendCommand('pause')
    }
  }

  // Resume the build
  async resume(): Promise<void> {
    if (this.buildId) {
      this.sendCommand('resume')
    }
  }

  // Cancel the build
  async cancel(): Promise<void> {
    if (this.buildId) {
      await apiService.cancelBuild(this.buildId)
      this.sendCommand('cancel')
    }
  }

  // Rollback to a checkpoint
  async rollback(checkpointId: string): Promise<void> {
    if (this.buildId) {
      await apiService.rollbackBuild(this.buildId, checkpointId)
    }
  }

  // Send a command to the agent
  private sendCommand(command: string): void {
    const payload = JSON.stringify({
      type: 'command',
      data: { command },
    })

    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(payload)
    }
  }

  // Get current build status
  async getStatus(): Promise<AgentTask | null> {
    if (!this.buildId) return null
    const response = await apiService.getBuildStatus(this.buildId)
    return response as AgentTask
  }

  // Get build details
  async getDetails(): Promise<any> {
    if (!this.buildId) return null
    return await apiService.getBuildDetails(this.buildId)
  }

  // Get checkpoints
  async getCheckpoints(): Promise<AgentCheckpoint[]> {
    if (!this.buildId) return []
    return await apiService.getBuildCheckpoints(this.buildId)
  }

  // Get file changes
  async getFileChanges(): Promise<FileChange[]> {
    if (!this.buildId) return []
    const files = await apiService.getBuildFiles(this.buildId)
    return files as FileChange[]
  }

  // Get agents involved in the build
  async getAgents(): Promise<any[]> {
    if (!this.buildId) return []
    return await apiService.getBuildAgents(this.buildId)
  }

  // Get tasks
  async getTasks(): Promise<any[]> {
    if (!this.buildId) return []
    return await apiService.getBuildTasks(this.buildId)
  }

  // Disconnect WebSocket
  disconnect(): void {
    this.stopHeartbeat()
    if (this.ws) {
      this.ws.close(1000, 'Client disconnect')
      this.ws = null
    }
    this.buildId = null
    this.handlers = {}
    this.messageQueue = []
  }

  // Check if connected
  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }

  // Get current build ID
  getBuildId(): string | null {
    return this.buildId
  }

  // Private methods

  private startHeartbeat(): void {
    this.stopHeartbeat()
    this.heartbeatInterval = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: 'ping' }))
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
      console.error('Max reconnect attempts reached')
      this.handlers.onError?.('Connection lost. Please refresh the page.')
      return
    }

    this.reconnectAttempts++
    const delay = Math.min(
      this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1),
      10000
    )

    setTimeout(() => {
      if (this.buildId && !this.isConnecting && !this.isConnected()) {
        console.log(`Attempting reconnect (${this.reconnectAttempts}/${this.maxReconnectAttempts})`)
        this.connect(this.buildId, this.handlers).catch((err) => {
          console.error('Reconnect failed:', err)
        })
      }
    }, delay)
  }

  private flushMessageQueue(): void {
    while (this.messageQueue.length > 0 && this.ws?.readyState === WebSocket.OPEN) {
      const message = this.messageQueue.shift()
      if (message) {
        this.ws.send(message)
      }
    }
  }
}

// Create singleton instance
export const agentApi = new AgentApiService()

// Export for easy importing
export default agentApi

// Helper function to create a mock task for demo/development
export function createMockAgentTask(description: string): AgentTask {
  const buildId = generateId()
  return {
    id: generateId(),
    buildId,
    description,
    mode: 'fast',
    status: 'idle',
    progress: 0,
    steps: [
      {
        id: generateId(),
        type: 'analyze',
        title: 'Analyzing Requirements',
        description: 'Understanding the project requirements and constraints',
        status: 'pending',
        progress: 0,
      },
      {
        id: generateId(),
        type: 'plan',
        title: 'Creating Build Plan',
        description: 'Generating a detailed implementation plan',
        status: 'pending',
        progress: 0,
      },
      {
        id: generateId(),
        type: 'scaffold',
        title: 'Scaffolding Project',
        description: 'Setting up project structure and dependencies',
        status: 'pending',
        progress: 0,
      },
      {
        id: generateId(),
        type: 'implement',
        title: 'Implementing Features',
        description: 'Writing code for all features',
        status: 'pending',
        progress: 0,
      },
      {
        id: generateId(),
        type: 'test',
        title: 'Running Tests',
        description: 'Executing test suite and validating functionality',
        status: 'pending',
        progress: 0,
      },
      {
        id: generateId(),
        type: 'review',
        title: 'Code Review',
        description: 'AI-powered code review and optimization',
        status: 'pending',
        progress: 0,
      },
    ],
    fileChanges: [],
    terminalOutput: [],
    checkpoints: [],
    startedAt: new Date().toISOString(),
    elapsedTimeMs: 0,
  }
}
