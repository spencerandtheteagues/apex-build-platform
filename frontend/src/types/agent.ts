// APEX.BUILD AI Autonomous Agent Types
// Type definitions for the agentic build system

export type AgentStatus =
  | 'idle'
  | 'initializing'
  | 'planning'
  | 'executing'
  | 'paused'
  | 'completed'
  | 'failed'
  | 'cancelled'

export type AgentStepStatus =
  | 'pending'
  | 'in_progress'
  | 'completed'
  | 'failed'
  | 'skipped'

export type AgentStepType =
  | 'analyze'
  | 'plan'
  | 'scaffold'
  | 'implement'
  | 'test'
  | 'review'
  | 'fix'
  | 'deploy'
  | 'checkpoint'

export interface AgentStep {
  id: string
  type: AgentStepType
  title: string
  description: string
  status: AgentStepStatus
  progress: number // 0-100
  startedAt?: string
  completedAt?: string
  duration?: number // milliseconds
  output?: string
  error?: string
  artifacts?: AgentArtifact[]
  subSteps?: AgentSubStep[]
}

export interface AgentSubStep {
  id: string
  title: string
  status: AgentStepStatus
  output?: string
}

export interface AgentArtifact {
  id: string
  type: 'file' | 'log' | 'diff' | 'test_result' | 'checkpoint'
  name: string
  path?: string
  content?: string
  language?: string
  diff?: FileDiff
}

export interface FileDiff {
  filePath: string
  additions: number
  deletions: number
  hunks: DiffHunk[]
}

export interface DiffHunk {
  oldStart: number
  oldLines: number
  newStart: number
  newLines: number
  lines: DiffLine[]
}

export interface DiffLine {
  type: 'add' | 'delete' | 'context'
  content: string
  lineNumber: number
}

export interface AgentTask {
  id: string
  buildId: string
  description: string
  mode: 'fast' | 'full'
  status: AgentStatus
  progress: number // 0-100
  currentStep?: AgentStep
  steps: AgentStep[]
  fileChanges: FileChange[]
  terminalOutput: TerminalEntry[]
  checkpoints: AgentCheckpoint[]
  startedAt: string
  completedAt?: string
  estimatedTimeMs?: number
  elapsedTimeMs: number
  error?: string
  projectId?: number
  projectName?: string
}

export interface FileChange {
  id: string
  path: string
  type: 'create' | 'modify' | 'delete' | 'rename'
  oldPath?: string // for renames
  language?: string
  diff?: FileDiff
  content?: string
  status: 'pending' | 'applied' | 'reverted'
}

export interface TerminalEntry {
  id: string
  timestamp: string
  type: 'command' | 'output' | 'error' | 'info' | 'success' | 'warning'
  content: string
  source?: string // which agent/step produced this
}

export interface AgentCheckpoint {
  id: string
  name: string
  description: string
  createdAt: string
  stepId: string
  canRestore: boolean
}

export interface AgentConfig {
  mode: 'fast' | 'full'
  autoCommit: boolean
  runTests: boolean
  reviewCode: boolean
  maxRetries: number
  timeoutMs: number
}

// WebSocket message types
export type AgentWSMessageType =
  | 'connected'
  | 'status_update'
  | 'step_start'
  | 'step_progress'
  | 'step_complete'
  | 'step_failed'
  | 'file_change'
  | 'terminal_output'
  | 'checkpoint_created'
  | 'agent_message'
  | 'error'
  | 'completed'
  | 'cancelled'

export interface AgentWSMessage {
  type: AgentWSMessageType
  buildId: string
  timestamp: string
  data: any
}

export interface AgentStatusUpdate {
  status: AgentStatus
  progress: number
  currentStepId?: string
  message?: string
}

export interface AgentStepUpdate {
  stepId: string
  status: AgentStepStatus
  progress: number
  output?: string
  error?: string
  artifacts?: AgentArtifact[]
}

export interface AgentFileChangeUpdate {
  fileChange: FileChange
}

export interface AgentTerminalUpdate {
  entry: TerminalEntry
}

export interface AgentCheckpointUpdate {
  checkpoint: AgentCheckpoint
}

export interface AgentMessageUpdate {
  role: 'system' | 'agent' | 'user'
  content: string
  agentName?: string
}

// API request/response types
export interface StartAgentRequest {
  description: string
  mode: 'fast' | 'full'
  config?: Partial<AgentConfig>
}

export interface StartAgentResponse {
  build_id: string
  websocket_url: string
  status: AgentStatus
}

export interface AgentStatusResponse {
  task: AgentTask
}

export interface SendMessageRequest {
  message: string
}

export interface RollbackRequest {
  checkpoint_id: string
}
