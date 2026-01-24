// APEX.BUILD App Builder - The Heart of the Platform
// Dark Demon Theme - AI-Powered App Generation

import React, { useState, useEffect, useRef, useCallback } from 'react'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import apiService from '@/services/api'
import {
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Badge,
  Avatar,
  LoadingOverlay
} from '@/components/ui'
import {
  Zap,
  Sparkles,
  Rocket,
  Bot,
  Code2,
  FileCode,
  CheckCircle2,
  Circle,
  AlertCircle,
  Clock,
  ChevronRight,
  MessageSquare,
  Send,
  Pause,
  Play,
  RotateCcw,
  Eye,
  Download,
  ExternalLink,
  Cpu,
  Database,
  Layout,
  TestTube,
  Shield,
  Terminal
} from 'lucide-react'

// Types
interface Agent {
  id: string
  role: string
  provider: string
  status: 'idle' | 'working' | 'completed' | 'error'
  progress: number
  currentTask?: {
    type: string
    description: string
  }
}

interface Task {
  id: string
  type: string
  description: string
  status: 'pending' | 'in_progress' | 'completed' | 'failed'
  assignedTo?: string
  output?: {
    files?: Array<{ path: string; language: string }>
    messages?: string[]
  }
}

interface Checkpoint {
  id: string
  number: number
  name: string
  description: string
  progress: number
  createdAt: string
}

interface ChatMessage {
  id: string
  role: 'user' | 'lead' | 'system'
  content: string
  timestamp: Date
}

interface AIThought {
  id: string
  agentId: string
  agentRole: string
  provider: string
  type: 'thinking' | 'action' | 'output' | 'error'
  content: string
  timestamp: Date
}

interface BuildState {
  id: string
  status: 'idle' | 'planning' | 'in_progress' | 'testing' | 'reviewing' | 'completed' | 'failed'
  progress: number
  agents: Agent[]
  tasks: Task[]
  checkpoints: Checkpoint[]
  description: string
}

type BuildMode = 'fast' | 'full'

export const AppBuilder: React.FC = () => {
  // Build state
  const [buildMode, setBuildMode] = useState<BuildMode>('full')
  const [appDescription, setAppDescription] = useState('')
  const [buildState, setBuildState] = useState<BuildState | null>(null)
  const [isBuilding, setIsBuilding] = useState(false)
  const [showChat, setShowChat] = useState(true)
  const [showPreview, setShowPreview] = useState(false)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const [generatedFiles, setGeneratedFiles] = useState<Array<{ path: string; content: string; language: string }>>([])

  // Chat state
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([])
  const [chatInput, setChatInput] = useState('')

  // AI Activity state - real-time thinking and actions
  const [aiThoughts, setAiThoughts] = useState<AIThought[]>([])
  const [showAiActivity, setShowAiActivity] = useState(true)
  const aiActivityRef = useRef<HTMLDivElement>(null)

  // WebSocket
  const wsRef = useRef<WebSocket | null>(null)
  const chatEndRef = useRef<HTMLDivElement>(null)

  const { user } = useStore()

  // Scroll chat to bottom on new messages
  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [chatMessages])

  // Connect to WebSocket when build starts
  const connectWebSocket = useCallback((buildId: string) => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws/build/${buildId}`

    const ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      console.log('WebSocket connected')
      addSystemMessage('Connected to build server')
    }

    ws.onmessage = (event) => {
      const message = JSON.parse(event.data)
      handleWebSocketMessage(message)
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
      addSystemMessage('Connection error - retrying...')
    }

    ws.onclose = () => {
      console.log('WebSocket disconnected')
    }

    wsRef.current = ws
  }, [])

  // Handle incoming WebSocket messages
  const handleWebSocketMessage = (message: any) => {
    const { type, data } = message

    switch (type) {
      case 'build:state':
        setBuildState(prev => ({
          ...prev,
          ...data,
          agents: Object.values(data.agents || {}),
        }))
        break

      case 'build:progress':
        setBuildState(prev => prev ? { ...prev, progress: data.progress } : null)
        break

      case 'agent:spawned':
        addSystemMessage(`${getAgentEmoji(data.role)} ${formatRole(data.role)} agent joined the team`)
        setBuildState(prev => {
          if (!prev) return null
          const newAgent: Agent = {
            id: message.agent_id,
            role: data.role,
            provider: data.provider,
            status: 'idle',
            progress: 0,
          }
          return { ...prev, agents: [...prev.agents, newAgent] }
        })
        break

      case 'agent:working':
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            agents: prev.agents.map(a =>
              a.id === message.agent_id
                ? { ...a, status: 'working', currentTask: { type: data.task_type, description: data.description } }
                : a
            )
          }
        })
        break

      case 'agent:completed':
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            agents: prev.agents.map(a =>
              a.id === message.agent_id ? { ...a, status: 'completed' } : a
            )
          }
        })
        break

      case 'agent:error':
        addSystemMessage(`Agent encountered an error: ${data.error}`)
        break

      case 'file:created':
        addSystemMessage(`Created: ${data.path}`)
        // Track generated files for preview
        setGeneratedFiles(prev => [...prev, {
          path: data.path,
          content: data.content || '',
          language: data.language || 'text'
        }])
        break

      case 'build:checkpoint':
        addSystemMessage(`Checkpoint ${data.number}: ${data.name}`)
        setBuildState(prev => {
          if (!prev) return null
          const checkpoint: Checkpoint = {
            id: data.checkpoint_id,
            number: data.number,
            name: data.name,
            description: data.description,
            progress: prev.progress,
            createdAt: new Date().toISOString(),
          }
          return { ...prev, checkpoints: [...prev.checkpoints, checkpoint] }
        })
        break

      case 'build:completed':
        setIsBuilding(false)
        addSystemMessage('Build completed successfully!')
        setBuildState(prev => prev ? { ...prev, status: 'completed', progress: 100 } : null)
        // Set preview URL if provided by backend
        if (data.preview_url) {
          setPreviewUrl(data.preview_url)
        }
        break

      case 'lead:response':
        setChatMessages(prev => [...prev, {
          id: Date.now().toString(),
          role: 'lead',
          content: data.content,
          timestamp: new Date(),
        }])
        break

      // AI Activity tracking - real-time thinking
      case 'agent:thinking':
        addAiThought(message.agent_id, data.agent_role, data.provider, 'thinking', data.content)
        break

      case 'agent:action':
        addAiThought(message.agent_id, data.agent_role, data.provider, 'action', data.content)
        break

      case 'agent:output':
        addAiThought(message.agent_id, data.agent_role, data.provider, 'output', data.content)
        break
    }
  }

  // Add AI thought to activity panel
  const addAiThought = (agentId: string, agentRole: string, provider: string, type: AIThought['type'], content: string) => {
    const thought: AIThought = {
      id: Date.now().toString() + Math.random(),
      agentId,
      agentRole,
      provider,
      type,
      content,
      timestamp: new Date(),
    }
    setAiThoughts(prev => {
      // Keep last 100 thoughts to prevent memory issues
      const updated = [...prev, thought]
      return updated.slice(-100)
    })
    // Auto-scroll activity panel
    setTimeout(() => {
      aiActivityRef.current?.scrollTo({ top: aiActivityRef.current.scrollHeight, behavior: 'smooth' })
    }, 50)
  }

  // Add system message to chat
  const addSystemMessage = (content: string) => {
    setChatMessages(prev => [...prev, {
      id: Date.now().toString(),
      role: 'system',
      content,
      timestamp: new Date(),
    }])
  }

  // Start the build
  const startBuild = async () => {
    if (!appDescription.trim()) return

    setIsBuilding(true)
    addSystemMessage(`Starting ${buildMode} build for: "${appDescription}"`)

    try {
      // Use the Agent Orchestration System
      const response = await apiService.startBuild({
        description: appDescription,
        mode: buildMode,
      })

      const buildId = response.build_id

      setBuildState({
        id: buildId,
        status: 'planning',
        progress: 5,
        agents: [],
        tasks: [],
        checkpoints: [],
        description: appDescription,
      })

      // Connect to WebSocket for real-time updates
      connectWebSocket(buildId)
      addSystemMessage(`Build started! Connecting to real-time updates...`)

    } catch (error: any) {
      const errorMsg = error.response?.data?.error || error.response?.data?.details || error.message
      addSystemMessage(`Error: ${errorMsg}`)
      setIsBuilding(false)
    }
  }

  // Send chat message
  const sendChatMessage = () => {
    if (!chatInput.trim() || !buildState?.id) return

    setChatMessages(prev => [...prev, {
      id: Date.now().toString(),
      role: 'user',
      content: chatInput,
      timestamp: new Date(),
    }])

    // Send via WebSocket
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        type: 'user:message',
        content: chatInput,
      }))
    }

    setChatInput('')
  }

  // Helper functions
  const getAgentEmoji = (role: string) => {
    const emojis: Record<string, string> = {
      lead: '👨‍💼',
      planner: '📋',
      architect: '🏗️',
      frontend: '🎨',
      backend: '⚙️',
      database: '🗄️',
      testing: '🧪',
      reviewer: '🔍',
    }
    return emojis[role] || '🤖'
  }

  const formatRole = (role: string) => {
    return role.charAt(0).toUpperCase() + role.slice(1)
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'working': return 'text-red-400'
      case 'completed': return 'text-green-400'
      case 'error': return 'text-red-400'
      default: return 'text-gray-400'
    }
  }

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'working': return <Circle className="w-3 h-3 animate-pulse fill-red-400 text-red-400" />
      case 'completed': return <CheckCircle2 className="w-3 h-3 text-green-400" />
      case 'error': return <AlertCircle className="w-3 h-3 text-orange-400" />
      default: return <Circle className="w-3 h-3 text-gray-500" />
    }
  }

  return (
    <div className="h-full overflow-y-auto bg-black text-white">
      {/* Demon theme background */}
      <div className="fixed inset-0 pointer-events-none">
        <div className="absolute inset-0 bg-gradient-to-b from-red-950/10 via-black to-black" />
        {/* Static red gradient accents */}
        <div className="absolute top-0 left-1/4 w-64 h-64 bg-red-900/5 rounded-full" />
        <div className="absolute bottom-0 right-1/4 w-64 h-64 bg-red-800/5 rounded-full" />
      </div>

      <div className="relative z-10 max-w-7xl mx-auto px-4 py-8">
        {/* Header */}
        <div className="text-center mb-12">
          <div className="flex items-center justify-center gap-3 mb-4">
            <div className="relative">
              <div className="w-16 h-16 bg-gradient-to-br from-red-600 to-red-900 rounded-2xl flex items-center justify-center shadow-lg shadow-red-900/50">
                <Rocket className="w-8 h-8 text-white" />
              </div>
              {/* Demon glow effect */}
              <div className="absolute -inset-1 bg-gradient-to-br from-red-600 to-red-900 rounded-2xl opacity-30" style={{ filter: 'blur(8px)' }} />
            </div>
          </div>
          <h1 className="text-4xl font-bold bg-gradient-to-r from-red-400 via-red-500 to-red-600 bg-clip-text text-transparent mb-2">
            Build Your App
          </h1>
          <p className="text-gray-400 text-lg">
            Describe what you want to build, and our AI agents will create it for you
          </p>
        </div>

        {/* Main Content */}
        {!buildState ? (
          // App Description Input
          <div className="max-w-3xl mx-auto">
            <Card variant="cyberpunk" glow="intense" className="border-2 border-red-900/30">
              <CardContent className="p-8">
                {/* Build Mode Toggle */}
                <div className="flex items-center justify-center gap-4 mb-6">
                  <button
                    onClick={() => setBuildMode('fast')}
                    className={cn(
                      'flex items-center gap-2 px-4 py-2 rounded-lg transition-all duration-300',
                      buildMode === 'fast'
                        ? 'bg-red-900/20 border-2 border-red-600 text-red-400 shadow-sm shadow-red-900/30'
                        : 'bg-gray-800/50 border-2 border-gray-700 text-gray-400 hover:border-gray-600'
                    )}
                  >
                    <Zap className="w-4 h-4" />
                    <span className="font-medium">Fast Build</span>
                    <span className="text-xs opacity-70">~3-5 min</span>
                  </button>
                  <button
                    onClick={() => setBuildMode('full')}
                    className={cn(
                      'flex items-center gap-2 px-4 py-2 rounded-lg transition-all duration-300',
                      buildMode === 'full'
                        ? 'bg-red-900/20 border-2 border-red-500 text-red-400 shadow-sm shadow-red-900/30'
                        : 'bg-gray-800/50 border-2 border-gray-700 text-gray-400 hover:border-gray-600'
                    )}
                  >
                    <Sparkles className="w-4 h-4" />
                    <span className="font-medium">Full Build</span>
                    <span className="text-xs opacity-70">10+ min</span>
                  </button>
                </div>

                {/* Description Input */}
                <div className="relative mb-6">
                  <textarea
                    value={appDescription}
                    onChange={(e) => setAppDescription(e.target.value)}
                    placeholder="Describe the app you want to build...

For example:
• Build a todo app with user authentication, categories, and due dates
• Create a dashboard to track cryptocurrency prices with charts
• Make an e-commerce store with product listings and a shopping cart"
                    className={cn(
                      'w-full h-48 bg-gray-900/80 border-2 rounded-xl px-4 py-3',
                      'text-white placeholder-gray-500',
                      'focus:outline-none focus:border-red-600 focus:ring-2 focus:ring-red-900/30',
                      'resize-none transition-colors duration-200',
                      'border-gray-700 hover:border-gray-600'
                    )}
                  />
                  <div className="absolute bottom-3 right-3 text-xs text-gray-500">
                    {appDescription.length} characters
                  </div>
                </div>

                {/* Build Button */}
                <Button
                  onClick={startBuild}
                  disabled={!appDescription.trim() || isBuilding}
                  size="lg"
                  className={cn(
                    'w-full h-14 text-lg font-bold',
                    'bg-gradient-to-r from-red-700 via-red-600 to-red-700',
                    'hover:from-red-600 hover:via-red-500 hover:to-red-600',
                    'shadow-lg shadow-red-900/50 hover:shadow-red-800/60',
                    'disabled:opacity-50 disabled:cursor-not-allowed'
                  )}
                >
                  {isBuilding ? (
                    <>
                      <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin mr-2" />
                      Starting Build...
                    </>
                  ) : (
                    <>
                      <Rocket className="w-5 h-5 mr-2" />
                      Start Building
                    </>
                  )}
                </Button>

                {/* Example Apps */}
                <div className="mt-6 pt-6 border-t border-gray-800">
                  <p className="text-sm text-gray-500 mb-3">Quick examples:</p>
                  <div className="flex flex-wrap gap-2">
                    {[
                      'Todo app with auth',
                      'Blog with comments',
                      'Chat application',
                      'Dashboard with charts',
                      'E-commerce store',
                    ].map((example) => (
                      <button
                        key={example}
                        onClick={() => setAppDescription(example)}
                        className="px-3 py-1.5 text-sm bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg transition-colors"
                      >
                        {example}
                      </button>
                    ))}
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        ) : (
          // Build Progress View
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Left Column - Agents */}
            <div className="lg:col-span-1 space-y-4">
              {/* Build Status */}
              <Card variant="cyberpunk" className="border border-gray-800">
                <CardHeader className="pb-2">
                  <CardTitle className="text-lg flex items-center gap-2">
                    <Bot className="w-5 h-5 text-red-400" />
                    Build Status
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  {/* Progress Bar */}
                  <div className="mb-4">
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-sm text-gray-400">Progress</span>
                      <span className="text-sm font-mono text-red-400">{buildState.progress}%</span>
                    </div>
                    <div className="h-2 bg-gray-800 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-gradient-to-r from-red-600 to-red-900 rounded-full transition-all duration-500"
                        style={{ width: `${buildState.progress}%` }}
                      />
                    </div>
                  </div>

                  {/* Status Badge */}
                  <div className="flex items-center gap-2">
                    <Badge
                      variant={buildState.status === 'completed' ? 'success' : 'primary'}
                      className="capitalize"
                    >
                      {buildState.status.replace('_', ' ')}
                    </Badge>
                    {buildState.status === 'in_progress' && (
                      <span className="text-xs text-gray-400">
                        {buildState.agents.filter(a => a.status === 'working').length} agents working
                      </span>
                    )}
                  </div>
                </CardContent>
              </Card>

              {/* Active Agents */}
              <Card variant="cyberpunk" className="border border-gray-800">
                <CardHeader className="pb-2">
                  <CardTitle className="text-lg flex items-center gap-2">
                    <Cpu className="w-5 h-5 text-orange-500" />
                    AI Agents ({buildState.agents.length})
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-3">
                    {buildState.agents.map((agent) => (
                      <div
                        key={agent.id}
                        className={cn(
                          'p-3 rounded-lg border transition-all duration-300',
                          agent.status === 'working'
                            ? 'bg-red-600/10 border-red-600/30'
                            : 'bg-gray-900/50 border-gray-800'
                        )}
                      >
                        <div className="flex items-center gap-3">
                          <div className="text-2xl">{getAgentEmoji(agent.role)}</div>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2">
                              <span className="font-medium text-white capitalize">{agent.role}</span>
                              {getStatusIcon(agent.status)}
                            </div>
                            {agent.currentTask && (
                              <p className="text-xs text-gray-400 truncate mt-0.5">
                                {agent.currentTask.description}
                              </p>
                            )}
                          </div>
                          <Badge variant="outline" size="xs" className="shrink-0">
                            {agent.provider}
                          </Badge>
                        </div>
                      </div>
                    ))}

                    {buildState.agents.length === 0 && (
                      <div className="text-center text-gray-500 py-4">
                        <Bot className="w-8 h-8 mx-auto mb-2 opacity-50" />
                        <p className="text-sm">Spawning agents...</p>
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>

              {/* Checkpoints */}
              {buildState.checkpoints.length > 0 && (
                <Card variant="cyberpunk" className="border border-gray-800">
                  <CardHeader className="pb-2">
                    <CardTitle className="text-lg flex items-center gap-2">
                      <CheckCircle2 className="w-5 h-5 text-green-400" />
                      Checkpoints
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-2">
                      {buildState.checkpoints.map((cp) => (
                        <div
                          key={cp.id}
                          className="flex items-center gap-3 p-2 rounded-lg bg-gray-900/50"
                        >
                          <div className="w-6 h-6 rounded-full bg-green-500/20 flex items-center justify-center text-xs font-bold text-green-400">
                            {cp.number}
                          </div>
                          <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium text-white">{cp.name}</p>
                            <p className="text-xs text-gray-500">{cp.progress}% complete</p>
                          </div>
                          <Button size="xs" variant="ghost" className="shrink-0">
                            <RotateCcw className="w-3 h-3" />
                          </Button>
                        </div>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              )}

              {/* AI Activity Panel - Real-time Thinking & Actions */}
              <Card variant="cyberpunk" className="border border-purple-900/50">
                <CardHeader className="pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-lg flex items-center gap-2">
                      <Bot className="w-5 h-5 text-purple-400 animate-pulse" />
                      AI Thinking
                    </CardTitle>
                    <Button
                      size="xs"
                      variant="ghost"
                      onClick={() => setShowAiActivity(!showAiActivity)}
                    >
                      <Eye className="w-4 h-4" />
                    </Button>
                  </div>
                </CardHeader>
                {showAiActivity && (
                  <CardContent>
                    <div
                      ref={aiActivityRef}
                      className="space-y-2 max-h-64 overflow-y-auto scrollbar-thin scrollbar-thumb-purple-900 scrollbar-track-gray-900"
                    >
                      {aiThoughts.length === 0 ? (
                        <div className="text-center text-gray-500 py-4">
                          <Bot className="w-8 h-8 mx-auto mb-2 opacity-50" />
                          <p className="text-sm">Waiting for AI activity...</p>
                        </div>
                      ) : (
                        aiThoughts.map((thought) => (
                          <div
                            key={thought.id}
                            className={cn(
                              'p-2 rounded-lg text-sm border-l-2 transition-all',
                              thought.type === 'thinking' && 'bg-purple-900/20 border-purple-500',
                              thought.type === 'action' && 'bg-cyan-900/20 border-cyan-500',
                              thought.type === 'output' && 'bg-green-900/20 border-green-500',
                              thought.type === 'error' && 'bg-red-900/20 border-red-500'
                            )}
                          >
                            <div className="flex items-center gap-2 mb-1">
                              <span className="text-xs font-medium text-gray-400">
                                {getAgentEmoji(thought.agentRole)} {thought.agentRole}
                              </span>
                              <Badge variant="outline" size="xs" className="text-[10px]">
                                {thought.provider}
                              </Badge>
                              <span className="text-[10px] text-gray-600">
                                {thought.timestamp.toLocaleTimeString()}
                              </span>
                            </div>
                            <p className={cn(
                              'text-xs',
                              thought.type === 'thinking' && 'text-purple-300 italic',
                              thought.type === 'action' && 'text-cyan-300',
                              thought.type === 'output' && 'text-green-300 font-mono',
                              thought.type === 'error' && 'text-red-300'
                            )}>
                              {thought.type === 'thinking' && '💭 '}
                              {thought.type === 'action' && '⚡ '}
                              {thought.type === 'output' && '✅ '}
                              {thought.type === 'error' && '❌ '}
                              {thought.content}
                            </p>
                          </div>
                        ))
                      )}
                    </div>
                  </CardContent>
                )}
              </Card>
            </div>

            {/* Middle Column - Activity & Chat */}
            <div className="lg:col-span-2 space-y-4">
              {/* App Description */}
              <Card variant="cyberpunk" className="border border-gray-800">
                <CardContent className="p-4">
                  <div className="flex items-start gap-3">
                    <div className="w-10 h-10 bg-gradient-to-br from-cyan-400 to-blue-500 rounded-lg flex items-center justify-center shrink-0">
                      <Code2 className="w-5 h-5 text-white" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm text-gray-400 mb-1">Building</p>
                      <p className="text-white font-medium">{buildState.description}</p>
                    </div>
                  </div>
                </CardContent>
              </Card>

              {/* Chat Interface */}
              <Card variant="cyberpunk" className="border border-gray-800 flex flex-col h-[500px]">
                <CardHeader className="pb-2 border-b border-gray-800 shrink-0">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-lg flex items-center gap-2">
                      <MessageSquare className="w-5 h-5 text-red-400" />
                      Build Activity
                    </CardTitle>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => setShowChat(!showChat)}
                    >
                      {showChat ? 'Hide Chat' : 'Show Chat'}
                    </Button>
                  </div>
                </CardHeader>
                <CardContent className="flex-1 flex flex-col overflow-hidden p-0">
                  {/* Messages */}
                  <div className="flex-1 overflow-y-auto p-4 space-y-3">
                    {chatMessages.map((msg) => (
                      <div key={msg.id} className={cn(
                        'flex gap-3',
                        msg.role === 'user' && 'justify-end'
                      )}>
                        {msg.role !== 'user' && (
                          <div className={cn(
                            'w-8 h-8 rounded-lg flex items-center justify-center shrink-0',
                            msg.role === 'lead' ? 'bg-red-900/20' : 'bg-gray-800'
                          )}>
                            {msg.role === 'lead' ? (
                              <Bot className="w-4 h-4 text-orange-500" />
                            ) : (
                              <Terminal className="w-4 h-4 text-gray-400" />
                            )}
                          </div>
                        )}
                        <div className={cn(
                          'max-w-[80%] rounded-lg px-4 py-2',
                          msg.role === 'user'
                            ? 'bg-red-600/20 text-cyan-100'
                            : msg.role === 'lead'
                            ? 'bg-red-900/10 text-gray-200'
                            : 'bg-gray-800/50 text-gray-400 text-sm'
                        )}>
                          <p>{msg.content}</p>
                          <p className="text-xs opacity-50 mt-1">
                            {msg.timestamp.toLocaleTimeString()}
                          </p>
                        </div>
                        {msg.role === 'user' && (
                          <Avatar
                            src={user?.avatar_url}
                            fallback={user?.username || 'U'}
                            size="sm"
                          />
                        )}
                      </div>
                    ))}
                    <div ref={chatEndRef} />
                  </div>

                  {/* Chat Input */}
                  {showChat && (
                    <div className="p-4 border-t border-gray-800 shrink-0">
                      <div className="flex gap-2">
                        <input
                          type="text"
                          value={chatInput}
                          onChange={(e) => setChatInput(e.target.value)}
                          onKeyDown={(e) => e.key === 'Enter' && sendChatMessage()}
                          placeholder="Message the lead agent..."
                          className="flex-1 bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white placeholder-gray-500 focus:outline-none focus:border-red-600"
                        />
                        <Button onClick={sendChatMessage} disabled={!chatInput.trim()}>
                          <Send className="w-4 h-4" />
                        </Button>
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>

              {/* Actions and Preview */}
              {buildState.status === 'completed' && (
                <>
                  <Card variant="cyberpunk" className="border border-green-500/30 bg-green-500/5">
                    <CardContent className="p-4">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                          <CheckCircle2 className="w-8 h-8 text-green-400" />
                          <div>
                            <h3 className="font-bold text-white">Build Complete!</h3>
                            <p className="text-sm text-gray-400">
                              {generatedFiles.length} files generated
                            </p>
                          </div>
                        </div>
                        <div className="flex gap-2">
                          <Button
                            variant="outline"
                            className={cn(
                              "border-red-600 text-red-400",
                              showPreview && "bg-red-600/20"
                            )}
                            onClick={() => setShowPreview(!showPreview)}
                          >
                            <Eye className="w-4 h-4 mr-2" />
                            {showPreview ? 'Hide Preview' : 'Show Preview'}
                          </Button>
                          <Button className="bg-green-500 hover:bg-green-400">
                            <ExternalLink className="w-4 h-4 mr-2" />
                            Open in IDE
                          </Button>
                        </div>
                      </div>
                    </CardContent>
                  </Card>

                  {/* Live Preview Panel */}
                  {showPreview && (
                    <Card variant="cyberpunk" className="border border-red-600/30">
                      <CardHeader className="pb-2 border-b border-gray-800">
                        <div className="flex items-center justify-between">
                          <CardTitle className="text-lg flex items-center gap-2">
                            <Eye className="w-5 h-5 text-red-400" />
                            Live Preview
                          </CardTitle>
                          {previewUrl && (
                            <a
                              href={previewUrl}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-xs text-red-400 hover:text-cyan-300 flex items-center gap-1"
                            >
                              <ExternalLink className="w-3 h-3" />
                              Open in new tab
                            </a>
                          )}
                        </div>
                      </CardHeader>
                      <CardContent className="p-0">
                        {previewUrl ? (
                          <iframe
                            src={previewUrl}
                            className="w-full h-[500px] bg-white rounded-b-lg"
                            title="App Preview"
                            sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
                          />
                        ) : (
                          <div className="h-[500px] flex flex-col items-center justify-center bg-gray-900/50 rounded-b-lg">
                            <FileCode className="w-12 h-12 text-gray-600 mb-4" />
                            <p className="text-gray-400 text-center mb-2">
                              Preview not available yet
                            </p>
                            <p className="text-gray-500 text-sm text-center max-w-md">
                              The generated app needs to be deployed to view.
                              Click "Open in IDE" to view the code and run it locally.
                            </p>
                            {generatedFiles.length > 0 && (
                              <div className="mt-4 p-4 bg-gray-800/50 rounded-lg max-w-md w-full">
                                <p className="text-xs text-gray-400 mb-2">Generated files:</p>
                                <div className="space-y-1 max-h-32 overflow-y-auto">
                                  {generatedFiles.map((file, idx) => (
                                    <div key={idx} className="flex items-center gap-2 text-xs">
                                      <FileCode className="w-3 h-3 text-red-400" />
                                      <span className="text-gray-300 truncate">{file.path}</span>
                                    </div>
                                  ))}
                                </div>
                              </div>
                            )}
                          </div>
                        )}
                      </CardContent>
                    </Card>
                  )}
                </>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

export default AppBuilder
