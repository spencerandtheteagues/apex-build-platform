// APEX.BUILD App Builder - The Heart of the Platform
// This is where users describe their app and watch AI agents build it in real-time
// Designed to EXCEED Replit's Agent interface in every way

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

  // Chat state
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([])
  const [chatInput, setChatInput] = useState('')

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
        break

      case 'lead:response':
        setChatMessages(prev => [...prev, {
          id: Date.now().toString(),
          role: 'lead',
          content: data.content,
          timestamp: new Date(),
        }])
        break
    }
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
      const response = await apiService.post('/api/v1/build/start', {
        description: appDescription,
        mode: buildMode,
      })

      setBuildState({
        id: response.build_id,
        status: 'planning',
        progress: 0,
        agents: [],
        tasks: [],
        checkpoints: [],
        description: appDescription,
      })

      connectWebSocket(response.build_id)
    } catch (error: any) {
      addSystemMessage(`Error: ${error.message}`)
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
      case 'working': return 'text-cyan-400'
      case 'completed': return 'text-green-400'
      case 'error': return 'text-red-400'
      default: return 'text-gray-400'
    }
  }

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'working': return <Circle className="w-3 h-3 animate-pulse fill-cyan-400 text-cyan-400" />
      case 'completed': return <CheckCircle2 className="w-3 h-3 text-green-400" />
      case 'error': return <AlertCircle className="w-3 h-3 text-red-400" />
      default: return <Circle className="w-3 h-3 text-gray-500" />
    }
  }

  return (
    <div className="min-h-screen bg-black text-white">
      {/* Animated background */}
      <div className="fixed inset-0 overflow-hidden pointer-events-none">
        <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_center,_var(--tw-gradient-stops))] from-cyan-900/20 via-black to-black" />
        <div className="absolute top-0 left-1/4 w-96 h-96 bg-cyan-500/5 rounded-full blur-3xl animate-pulse" />
        <div className="absolute bottom-0 right-1/4 w-96 h-96 bg-pink-500/5 rounded-full blur-3xl animate-pulse" style={{ animationDelay: '1s' }} />

        {/* Circuit pattern overlay */}
        <div className="absolute inset-0 opacity-5" style={{
          backgroundImage: `url("data:image/svg+xml,%3Csvg width='60' height='60' viewBox='0 0 60 60' xmlns='http://www.w3.org/2000/svg'%3E%3Cg fill='none' fill-rule='evenodd'%3E%3Cg fill='%2300FFFF' fill-opacity='0.4'%3E%3Cpath d='M36 34v-4h-2v4h-4v2h4v4h2v-4h4v-2h-4zm0-30V0h-2v4h-4v2h4v4h2V6h4V4h-4zM6 34v-4H4v4H0v2h4v4h2v-4h4v-2H6zM6 4V0H4v4H0v2h4v4h2V6h4V4H6z'/%3E%3C/g%3E%3C/g%3E%3C/svg%3E")`,
        }} />
      </div>

      <div className="relative z-10 max-w-7xl mx-auto px-4 py-8">
        {/* Header */}
        <div className="text-center mb-12">
          <div className="flex items-center justify-center gap-3 mb-4">
            <div className="relative">
              <div className="w-16 h-16 bg-gradient-to-br from-cyan-400 via-blue-500 to-purple-600 rounded-2xl flex items-center justify-center shadow-lg shadow-cyan-500/30">
                <Rocket className="w-8 h-8 text-white" />
              </div>
              <div className="absolute -inset-1 bg-gradient-to-br from-cyan-400 via-blue-500 to-purple-600 rounded-2xl blur-lg opacity-50 animate-pulse" />
            </div>
          </div>
          <h1 className="text-4xl font-bold bg-gradient-to-r from-cyan-400 via-blue-400 to-purple-400 bg-clip-text text-transparent mb-2">
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
            <Card variant="cyberpunk" glow="intense" className="border-2 border-cyan-500/30">
              <CardContent className="p-8">
                {/* Build Mode Toggle */}
                <div className="flex items-center justify-center gap-4 mb-6">
                  <button
                    onClick={() => setBuildMode('fast')}
                    className={cn(
                      'flex items-center gap-2 px-4 py-2 rounded-lg transition-all duration-300',
                      buildMode === 'fast'
                        ? 'bg-cyan-500/20 border-2 border-cyan-400 text-cyan-400'
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
                        ? 'bg-purple-500/20 border-2 border-purple-400 text-purple-400'
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
                      'focus:outline-none focus:border-cyan-400 focus:ring-2 focus:ring-cyan-400/20',
                      'resize-none transition-all duration-300',
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
                    'bg-gradient-to-r from-cyan-500 via-blue-500 to-purple-500',
                    'hover:from-cyan-400 hover:via-blue-400 hover:to-purple-400',
                    'shadow-lg shadow-cyan-500/30 hover:shadow-cyan-400/50',
                    'transition-all duration-300',
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
                    <Bot className="w-5 h-5 text-cyan-400" />
                    Build Status
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  {/* Progress Bar */}
                  <div className="mb-4">
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-sm text-gray-400">Progress</span>
                      <span className="text-sm font-mono text-cyan-400">{buildState.progress}%</span>
                    </div>
                    <div className="h-2 bg-gray-800 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-gradient-to-r from-cyan-500 to-purple-500 rounded-full transition-all duration-500"
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
                    <Cpu className="w-5 h-5 text-purple-400" />
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
                            ? 'bg-cyan-500/10 border-cyan-500/30'
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
                      <MessageSquare className="w-5 h-5 text-cyan-400" />
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
                            msg.role === 'lead' ? 'bg-purple-500/20' : 'bg-gray-800'
                          )}>
                            {msg.role === 'lead' ? (
                              <Bot className="w-4 h-4 text-purple-400" />
                            ) : (
                              <Terminal className="w-4 h-4 text-gray-400" />
                            )}
                          </div>
                        )}
                        <div className={cn(
                          'max-w-[80%] rounded-lg px-4 py-2',
                          msg.role === 'user'
                            ? 'bg-cyan-500/20 text-cyan-100'
                            : msg.role === 'lead'
                            ? 'bg-purple-500/10 text-gray-200'
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
                          className="flex-1 bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500"
                        />
                        <Button onClick={sendChatMessage} disabled={!chatInput.trim()}>
                          <Send className="w-4 h-4" />
                        </Button>
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>

              {/* Actions */}
              {buildState.status === 'completed' && (
                <Card variant="cyberpunk" className="border border-green-500/30 bg-green-500/5">
                  <CardContent className="p-4">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <CheckCircle2 className="w-8 h-8 text-green-400" />
                        <div>
                          <h3 className="font-bold text-white">Build Complete!</h3>
                          <p className="text-sm text-gray-400">Your app is ready to preview</p>
                        </div>
                      </div>
                      <div className="flex gap-2">
                        <Button variant="outline" className="border-cyan-500 text-cyan-400">
                          <Eye className="w-4 h-4 mr-2" />
                          Preview
                        </Button>
                        <Button className="bg-green-500 hover:bg-green-400">
                          <ExternalLink className="w-4 h-4 mr-2" />
                          Open in IDE
                        </Button>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

export default AppBuilder
