// APEX.BUILD App Builder - Command Center Interface
// Dark Demon Theme - AI-Powered App Generation with Futuristic UI

import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react'
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
  LoadingOverlay,
  AnimatedBackground
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
  Terminal,
  Server,
  Globe,
  Layers,
  Github
} from 'lucide-react'
import { GitHubImportWizard } from '@/components/import/GitHubImportWizard'

// ============================================================================
// TYPES
// ============================================================================

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
  availableProviders?: string[]
}

type BuildMode = 'fast' | 'full'

interface TechStack {
  id: string
  name: string
  icon: React.ReactNode
  category: 'frontend' | 'backend' | 'database' | 'deploy'
  description: string
}

interface AppBuilderProps {
  onNavigateToIDE?: () => void
}

// ============================================================================
// ANIMATED BACKGROUND COMPONENTS
// ============================================================================

const HexGrid: React.FC = () => {
  return (
    <div className="absolute inset-0 overflow-hidden opacity-30 pointer-events-none">
      <svg className="absolute inset-0 w-full h-full" xmlns="http://www.w3.org/2000/svg">
        <defs>
          <pattern id="hexagons" width="50" height="43.4" patternUnits="userSpaceOnUse" patternTransform="scale(2)">
            <polygon
              fill="none"
              stroke="rgba(204, 0, 0, 0.3)"
              strokeWidth="0.5"
              points="24.8,22 37.3,29.2 37.3,43.7 24.8,50.9 12.3,43.7 12.3,29.2"
              transform="translate(0, -21.7)"
            />
            <polygon
              fill="none"
              stroke="rgba(204, 0, 0, 0.3)"
              strokeWidth="0.5"
              points="24.8,22 37.3,29.2 37.3,43.7 24.8,50.9 12.3,43.7 12.3,29.2"
              transform="translate(25, 0)"
            />
          </pattern>
        </defs>
        <rect width="100%" height="100%" fill="url(#hexagons)" />
      </svg>
    </div>
  )
}

const FloatingParticles: React.FC = () => {
  const particles = useMemo(() =>
    Array.from({ length: 25 }, (_, i) => ({
      id: i,
      size: Math.random() * 3 + 1,
      x: Math.random() * 100,
      y: Math.random() * 100,
      duration: Math.random() * 20 + 10,
      delay: Math.random() * 5,
    })), []
  )

  return (
    <div className="absolute inset-0 overflow-hidden pointer-events-none">
      {particles.map((particle) => (
        <div
          key={particle.id}
          className="absolute rounded-full bg-red-500/40"
          style={{
            width: particle.size,
            height: particle.size,
            left: `${particle.x}%`,
            top: `${particle.y}%`,
            animation: `float ${particle.duration}s ease-in-out infinite`,
            animationDelay: `${particle.delay}s`,
            boxShadow: '0 0 8px rgba(204, 0, 0, 0.6)',
          }}
        />
      ))}
    </div>
  )
}

// ============================================================================
// ANIMATED LOGO COMPONENT
// ============================================================================

const AnimatedLogo: React.FC = () => {
  return (
    <div className="relative group">
      {/* Subtle glow behind logo */}
      <div className="absolute inset-0 bg-red-600/20 rounded-full blur-3xl scale-150 animate-pulse" style={{ animationDuration: '3s' }} />

      {/* Main logo - large and clean, no background box */}
      <div className="relative w-56 h-56 flex items-center justify-center group-hover:scale-105 transition-transform duration-500">
        <img
          src="/logo.png"
          alt="APEX"
          className="w-full h-full object-contain relative z-10 drop-shadow-[0_0_30px_rgba(220,38,38,0.5)]"
        />
      </div>

      {/* Corner accents - HUD style brackets */}
      <div className="absolute -top-4 -left-4 w-6 h-6 border-t-2 border-l-2 border-red-500/70 rounded-tl" />
      <div className="absolute -top-4 -right-4 w-6 h-6 border-t-2 border-r-2 border-red-500/70 rounded-tr" />
      <div className="absolute -bottom-4 -left-4 w-6 h-6 border-b-2 border-l-2 border-red-500/70 rounded-bl" />
      <div className="absolute -bottom-4 -right-4 w-6 h-6 border-b-2 border-r-2 border-red-500/70 rounded-br" />
    </div>
  )
}

// ============================================================================
// ANIMATED TITLE COMPONENT
// ============================================================================

const AnimatedTitle: React.FC = () => {
  return (
    <h1 className="text-5xl md:text-6xl font-black relative tracking-tight">
      <span
        className="bg-gradient-to-r from-red-400 via-orange-400 to-red-500 bg-clip-text text-transparent"
        style={{
          backgroundSize: '200% auto',
          animation: 'gradient-shift 3s linear infinite',
        }}
      >
        Build Your App
      </span>
      {/* Glow layer */}
      <span
        className="absolute inset-0 bg-gradient-to-r from-red-400 via-orange-400 to-red-500 bg-clip-text text-transparent blur-lg opacity-50 pointer-events-none"
        style={{
          backgroundSize: '200% auto',
          animation: 'gradient-shift 3s linear infinite',
        }}
        aria-hidden="true"
      >
        Build Your App
      </span>
    </h1>
  )
}

// ============================================================================
// TYPEWRITER SUBTITLE COMPONENT
// ============================================================================

const TypewriterSubtitle: React.FC<{ text: string }> = ({ text }) => {
  const [displayedText, setDisplayedText] = useState('')
  const [showCursor, setShowCursor] = useState(true)

  useEffect(() => {
    let index = 0
    const timer = setInterval(() => {
      if (index < text.length) {
        setDisplayedText(text.slice(0, index + 1))
        index++
      } else {
        clearInterval(timer)
      }
    }, 35)

    return () => clearInterval(timer)
  }, [text])

  useEffect(() => {
    const cursorTimer = setInterval(() => {
      setShowCursor(prev => !prev)
    }, 530)
    return () => clearInterval(cursorTimer)
  }, [])

  return (
    <p className="text-gray-400 text-lg md:text-xl font-light tracking-wide">
      {displayedText}
      <span className={cn("inline-block w-0.5 h-5 bg-red-500 ml-1 align-middle transition-opacity duration-100", showCursor ? "opacity-100" : "opacity-0")} />
    </p>
  )
}

// ============================================================================
// PREMIUM TEXTAREA COMPONENT
// ============================================================================

interface PremiumTextareaProps {
  value: string
  onChange: (value: string) => void
  maxLength?: number
}

const PremiumTextarea: React.FC<PremiumTextareaProps> = ({ value, onChange, maxLength = 2000 }) => {
  const [isFocused, setIsFocused] = useState(false)
  const isEmpty = value.length === 0
  const progressPercent = (value.length / maxLength) * 100

  return (
    <div className="relative group">
      {/* Animated border container */}
      <div className={cn(
        "absolute -inset-[2px] rounded-2xl transition-all duration-500",
        isEmpty && !isFocused && "animate-pulse",
        isFocused
          ? "bg-gradient-to-r from-red-500 via-orange-500 to-red-500 shadow-lg shadow-red-900/50"
          : "bg-gradient-to-r from-red-900/50 to-red-800/50"
      )} style={isFocused ? { backgroundSize: '200% auto', animation: 'gradient-shift 2s linear infinite' } : {}} />

      {/* Glass effect background */}
      <div className="absolute inset-0 rounded-xl bg-black/90 backdrop-blur-xl" />

      {/* Inner glow on focus */}
      {isFocused && (
        <div className="absolute inset-0 rounded-xl bg-gradient-to-b from-red-900/20 via-transparent to-red-900/10 pointer-events-none" />
      )}

      {/* Textarea */}
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onFocus={() => setIsFocused(true)}
        onBlur={() => setIsFocused(false)}
        maxLength={maxLength}
        placeholder="Describe the app you want to build...

For example:
- Build a todo app with user authentication, categories, and due dates
- Create a dashboard to track cryptocurrency prices with charts
- Make an e-commerce store with product listings and a shopping cart"
        className={cn(
          "relative w-full h-56 bg-transparent rounded-xl px-5 py-4",
          "text-white placeholder-gray-500 text-base leading-relaxed",
          "focus:outline-none resize-none",
          "transition-all duration-300",
          "z-10"
        )}
      />

      {/* Character count and progress bar */}
      <div className="absolute bottom-4 right-4 flex items-center gap-3 z-20">
        <div className="w-28 h-2 bg-gray-800 rounded-full overflow-hidden border border-gray-700">
          <div
            className={cn(
              "h-full rounded-full transition-all duration-500 relative overflow-hidden",
              progressPercent > 80 ? "bg-orange-500" : progressPercent > 50 ? "bg-yellow-500" : "bg-red-500"
            )}
            style={{ width: `${progressPercent}%` }}
          >
            <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/30 to-transparent" style={{ animation: 'shimmer 1.5s infinite' }} />
          </div>
        </div>
        <span className="text-xs text-gray-500 font-mono tabular-nums">
          {value.length.toLocaleString()}/{maxLength.toLocaleString()}
        </span>
      </div>

      {/* Corner decorations */}
      <div className="absolute top-2 left-2 w-5 h-5 border-t-2 border-l-2 border-red-600/50 rounded-tl pointer-events-none" />
      <div className="absolute top-2 right-2 w-5 h-5 border-t-2 border-r-2 border-red-600/50 rounded-tr pointer-events-none" />
      <div className="absolute bottom-12 left-2 w-5 h-5 border-b-2 border-l-2 border-red-600/50 rounded-bl pointer-events-none" />
      <div className="absolute bottom-12 right-2 w-5 h-5 border-b-2 border-r-2 border-red-600/50 rounded-br pointer-events-none" />
    </div>
  )
}

// ============================================================================
// TECH STACK CARD COMPONENT
// ============================================================================

interface TechStackCardProps {
  stack: TechStack
  isSelected: boolean
  onClick: () => void
}

const TechStackCard: React.FC<TechStackCardProps> = ({ stack, isSelected, onClick }) => {
  return (
    <button
      onClick={onClick}
      className={cn(
        "relative group p-4 rounded-xl transition-all duration-300 text-left overflow-hidden",
        "border-2 backdrop-blur-sm",
        isSelected
          ? "border-red-500 bg-red-950/40 shadow-lg shadow-red-900/40 scale-[1.02]"
          : "border-gray-800 bg-gray-900/50 hover:border-gray-600 hover:bg-gray-900/70 hover:scale-[1.01]"
      )}
    >
      {/* Holographic scan effect */}
      <div className={cn(
        "absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none",
        "bg-gradient-to-r from-transparent via-white/5 to-transparent"
      )} style={{ animation: 'scan-horizontal 2s linear infinite' }} />

      {/* Selected glow */}
      {isSelected && (
        <>
          <div className="absolute inset-0 bg-gradient-to-br from-red-600/20 via-transparent to-orange-600/20 pointer-events-none" />
          <div className="absolute -inset-[1px] bg-gradient-to-r from-red-500 via-orange-500 to-red-500 rounded-xl opacity-40 blur-sm -z-10" />
        </>
      )}

      {/* Content */}
      <div className="relative z-10 flex items-center gap-3">
        <div className={cn(
          "w-10 h-10 rounded-lg flex items-center justify-center transition-all duration-300 flex-shrink-0",
          isSelected
            ? "bg-red-600 text-white shadow-lg shadow-red-900/50"
            : "bg-gray-800 text-gray-400 group-hover:text-white group-hover:bg-gray-700"
        )}>
          {stack.icon}
        </div>
        <div className="flex-1 min-w-0">
          <h4 className={cn(
            "font-bold text-sm transition-colors",
            isSelected ? "text-white" : "text-gray-200"
          )}>
            {stack.name}
          </h4>
          <p className={cn(
            "text-xs transition-colors",
            isSelected ? "text-red-300" : "text-gray-500"
          )}>{stack.description}</p>
        </div>
      </div>
    </button>
  )
}

// ============================================================================
// EPIC BUILD BUTTON COMPONENT
// ============================================================================

interface EpicBuildButtonProps {
  onClick: () => void
  disabled: boolean
  isLoading: boolean
}

const EpicBuildButton: React.FC<EpicBuildButtonProps> = ({ onClick, disabled, isLoading }) => {
  const [ripples, setRipples] = useState<{ id: number; x: number; y: number }[]>([])
  const buttonRef = useRef<HTMLButtonElement>(null)

  const handleClick = (e: React.MouseEvent<HTMLButtonElement>) => {
    if (disabled) return

    // Create ripple effect
    const rect = buttonRef.current?.getBoundingClientRect()
    if (rect) {
      const x = e.clientX - rect.left
      const y = e.clientY - rect.top
      const newRipple = { id: Date.now(), x, y }
      setRipples(prev => [...prev, newRipple])
      setTimeout(() => {
        setRipples(prev => prev.filter(r => r.id !== newRipple.id))
      }, 800)
    }

    onClick()
  }

  return (
    <button
      ref={buttonRef}
      onClick={handleClick}
      disabled={disabled}
      className={cn(
        "relative w-full h-18 py-5 rounded-2xl font-black text-xl overflow-hidden",
        "transition-all duration-300 transform",
        disabled
          ? "opacity-50 cursor-not-allowed"
          : "hover:scale-[1.02] hover:shadow-2xl hover:shadow-red-900/60 active:scale-[0.98]"
      )}
    >
      {/* Animated gradient background */}
      <div className={cn(
        "absolute inset-0 bg-gradient-to-r from-red-700 via-orange-600 to-red-700",
        !disabled && !isLoading && "animate-gradient-shift"
      )} style={{ backgroundSize: '200% auto' }} />

      {/* Pulsing glow effect */}
      {!disabled && !isLoading && (
        <div className="absolute -inset-1 bg-gradient-to-r from-red-500 via-orange-500 to-red-500 rounded-2xl opacity-60 blur-lg animate-pulse" />
      )}

      {/* Inner shine */}
      <div className="absolute inset-0 bg-gradient-to-b from-white/25 via-transparent to-black/30" />

      {/* Animated border for loading */}
      {isLoading && (
        <div className="absolute inset-0 rounded-2xl overflow-hidden">
          <div
            className="absolute inset-[-100%] bg-gradient-conic from-red-500 via-transparent to-red-500"
            style={{ animation: 'spin 1.5s linear infinite' }}
          />
          <div className="absolute inset-[2px] bg-gradient-to-r from-red-700 via-orange-600 to-red-700 rounded-2xl" />
        </div>
      )}

      {/* Ripple effects */}
      {ripples.map(ripple => (
        <span
          key={ripple.id}
          className="absolute bg-white/40 rounded-full pointer-events-none"
          style={{
            left: ripple.x - 20,
            top: ripple.y - 20,
            width: 40,
            height: 40,
            animation: 'ripple-expand 0.8s ease-out forwards',
          }}
        />
      ))}

      {/* Scan line effect when not loading */}
      {!isLoading && !disabled && (
        <div className="absolute inset-0 overflow-hidden rounded-2xl pointer-events-none">
          <div
            className="absolute inset-0 bg-gradient-to-r from-transparent via-white/20 to-transparent w-1/3"
            style={{ animation: 'scan-horizontal 2s linear infinite' }}
          />
        </div>
      )}

      {/* Content */}
      <span className="relative z-10 flex items-center justify-center gap-4 text-white drop-shadow-lg">
        {isLoading ? (
          <>
            <div className="w-7 h-7 border-3 border-white/30 border-t-white rounded-full animate-spin" />
            <span className="tracking-wide">INITIALIZING SYSTEMS...</span>
          </>
        ) : (
          <>
            <Rocket className="w-7 h-7" style={{ animation: 'float 2s ease-in-out infinite' }} />
            <span className="tracking-wider">LAUNCH BUILD</span>
            <Sparkles className="w-6 h-6 animate-pulse" />
          </>
        )}
      </span>
    </button>
  )
}

// ============================================================================
// AGENT CARD COMPONENT (Animated)
// ============================================================================

interface AgentCardProps {
  agent: Agent
  index: number
  getAgentEmoji: (role: string) => string
  getStatusIcon: (status: string) => React.ReactNode
}

const AgentCard: React.FC<AgentCardProps> = ({ agent, index, getAgentEmoji, getStatusIcon }) => {
  return (
    <div
      className={cn(
        "p-4 rounded-xl border-2 transition-all duration-500",
        agent.status === 'working'
          ? "bg-gradient-to-br from-red-950/60 to-orange-950/40 border-red-600/60 shadow-lg shadow-red-900/40"
          : agent.status === 'completed'
          ? "bg-gradient-to-br from-green-950/40 to-emerald-950/30 border-green-600/40"
          : agent.status === 'error'
          ? "bg-gradient-to-br from-orange-950/40 to-red-950/30 border-orange-600/40"
          : "bg-gray-900/60 border-gray-800"
      )}
      style={{
        animation: 'fade-in-up 0.5s ease-out forwards',
        animationDelay: `${index * 100}ms`,
        opacity: 0,
      }}
    >
      <div className="flex items-center gap-4">
        {/* Agent Avatar */}
        <div className={cn(
          "relative w-14 h-14 rounded-xl flex items-center justify-center text-2xl transition-all duration-300",
          agent.status === 'working'
            ? "bg-red-900/50"
            : agent.status === 'completed'
            ? "bg-green-900/40"
            : "bg-gray-800"
        )}>
          {agent.status === 'working' && (
            <>
              <div className="absolute inset-0 rounded-xl border-2 border-red-500/50 animate-ping" style={{ animationDuration: '1.5s' }} />
              <div className="absolute inset-0 rounded-xl bg-red-500/20 animate-pulse" />
            </>
          )}
          <span className="relative z-10">{getAgentEmoji(agent.role)}</span>
        </div>

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <span className="font-bold text-white capitalize text-lg">{agent.role}</span>
            {getStatusIcon(agent.status)}
          </div>
          {agent.currentTask && (
            <p className="text-sm text-gray-400 truncate">
              {agent.currentTask.description}
            </p>
          )}

          {/* Progress bar for working agents */}
          {agent.status === 'working' && (
            <div className="mt-2 h-2 bg-gray-800 rounded-full overflow-hidden border border-gray-700">
              <div
                className="h-full bg-gradient-to-r from-red-500 via-orange-500 to-red-500 rounded-full transition-all duration-500 relative overflow-hidden"
                style={{ width: `${agent.progress || 50}%`, backgroundSize: '200% auto', animation: 'gradient-shift 2s linear infinite' }}
              >
                <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/30 to-transparent" style={{ animation: 'shimmer 1s linear infinite' }} />
              </div>
            </div>
          )}
        </div>

        <Badge
          variant="outline"
          className={cn(
            "shrink-0 uppercase text-xs font-bold tracking-wider px-3 py-1",
            agent.provider === 'claude' && "border-orange-500/60 text-orange-400 bg-orange-500/10",
            agent.provider === 'gpt' && "border-green-500/60 text-green-400 bg-green-500/10",
            agent.provider === 'gemini' && "border-blue-500/60 text-blue-400 bg-blue-500/10"
          )}
        >
          {agent.provider}
        </Badge>
      </div>
    </div>
  )
}

// ============================================================================
// BUILD COMPLETE CELEBRATION
// ============================================================================

const BuildCompleteCard: React.FC<{ filesCount: number; onPreview: () => void; onOpenIDE: () => void; isCreating: boolean; showPreview: boolean }> = ({
  filesCount,
  onPreview,
  onOpenIDE,
  isCreating,
  showPreview
}) => {
  return (
    <Card variant="cyberpunk" className="relative overflow-hidden border-2 border-green-500/60 bg-gradient-to-br from-green-950/40 via-black to-emerald-950/30">
      {/* Success particles */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        {Array.from({ length: 15 }).map((_, i) => (
          <div
            key={i}
            className="absolute w-1.5 h-1.5 bg-green-400 rounded-full"
            style={{
              left: `${Math.random() * 100}%`,
              top: `${Math.random() * 100}%`,
              animation: `sparkle ${1 + Math.random()}s ease-out infinite`,
              animationDelay: `${Math.random() * 2}s`,
            }}
          />
        ))}
      </div>

      <CardContent className="p-6 relative z-10">
        <div className="flex items-center justify-between flex-wrap gap-4">
          <div className="flex items-center gap-4">
            <div className="relative">
              <CheckCircle2 className="w-14 h-14 text-green-400" style={{ animation: 'bounce-slow 2s ease-in-out infinite' }} />
              <div className="absolute inset-0 bg-green-400/40 rounded-full blur-xl animate-pulse" />
            </div>
            <div>
              <h3 className="font-black text-2xl text-white mb-1">Build Complete!</h3>
              <p className="text-green-400 font-mono text-lg">
                {filesCount} files generated successfully
              </p>
            </div>
          </div>
          <div className="flex gap-3 flex-wrap">
            <Button
              variant="outline"
              size="lg"
              className={cn(
                "border-2 border-red-600 text-red-400 hover:bg-red-950/50 transition-all font-semibold",
                showPreview && "bg-red-950/50 shadow-lg shadow-red-900/30"
              )}
              onClick={onPreview}
            >
              <Eye className="w-5 h-5 mr-2" />
              {showPreview ? 'Hide Preview' : 'Preview'}
            </Button>
            <Button
              size="lg"
              className="bg-gradient-to-r from-green-600 to-emerald-600 hover:from-green-500 hover:to-emerald-500 font-bold shadow-lg shadow-green-900/50 text-white"
              onClick={onOpenIDE}
              disabled={isCreating}
            >
              {isCreating ? (
                <>
                  <Clock className="w-5 h-5 mr-2 animate-spin" />
                  Creating...
                </>
              ) : (
                <>
                  <ExternalLink className="w-5 h-5 mr-2" />
                  Open in IDE
                </>
              )}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

// ============================================================================
// TERMINAL OUTPUT COMPONENT
// ============================================================================

const TerminalOutput: React.FC<{ messages: ChatMessage[]; isBuilding: boolean }> = ({ messages, isBuilding }) => {
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    containerRef.current?.scrollTo({ top: containerRef.current.scrollHeight, behavior: 'smooth' })
  }, [messages.length])

  return (
    <div
      ref={containerRef}
      className="bg-black/90 rounded-xl p-4 font-mono text-sm h-72 overflow-y-auto border border-gray-800"
    >
      {/* Terminal header */}
      <div className="flex items-center gap-2 mb-4 pb-3 border-b border-gray-800">
        <div className="w-3 h-3 rounded-full bg-red-500 shadow-lg shadow-red-500/50" />
        <div className="w-3 h-3 rounded-full bg-yellow-500 shadow-lg shadow-yellow-500/50" />
        <div className="w-3 h-3 rounded-full bg-green-500 shadow-lg shadow-green-500/50" />
        <span className="ml-3 text-gray-500 text-xs tracking-wider uppercase">APEX Build Terminal</span>
      </div>

      {messages.map((msg, index) => (
        <div
          key={msg.id}
          className={cn(
            "flex items-start gap-2 mb-2",
            msg.role === 'system' && "text-gray-400",
            msg.role === 'lead' && "text-orange-400",
            msg.role === 'user' && "text-cyan-400"
          )}
          style={{ animation: 'fade-in 0.2s ease-out', animationDelay: `${index * 30}ms` }}
        >
          <span className="text-red-500 select-none font-bold">{'>'}</span>
          <span className="flex-1 break-words">{msg.content}</span>
          <span className="text-gray-600 text-xs shrink-0">{msg.timestamp.toLocaleTimeString()}</span>
        </div>
      ))}

      {/* Blinking cursor */}
      {isBuilding && (
        <div className="flex items-center gap-2 text-red-500">
          <span className="font-bold">{'>'}</span>
          <span className="w-2.5 h-5 bg-red-500 animate-pulse" />
        </div>
      )}
    </div>
  )
}

// ============================================================================
// MAIN APP BUILDER COMPONENT
// ============================================================================

export const AppBuilder: React.FC<AppBuilderProps> = ({ onNavigateToIDE }) => {
  // Build state
  const [buildMode, setBuildMode] = useState<BuildMode>('full')
  const [appDescription, setAppDescription] = useState('')
  const [buildState, setBuildState] = useState<BuildState | null>(null)
  const [isBuilding, setIsBuilding] = useState(false)
  const [showChat, setShowChat] = useState(true)
  const [showPreview, setShowPreview] = useState(false)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const [generatedFiles, setGeneratedFiles] = useState<Array<{ path: string; content: string; language: string }>>([])
  const [createdProjectId, setCreatedProjectId] = useState<number | null>(null)
  const [isCreatingProject, setIsCreatingProject] = useState(false)
  const [selectedStack, setSelectedStack] = useState<Set<string>>(new Set(['nextjs', 'postgresql', 'vercel']))

  // Chat state
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([])
  const [chatInput, setChatInput] = useState('')

  // AI Activity state
  const [aiThoughts, setAiThoughts] = useState<AIThought[]>([])
  const [showAiActivity, setShowAiActivity] = useState(true)
  const aiActivityRef = useRef<HTMLDivElement>(null)

  // WebSocket
  const wsRef = useRef<WebSocket | null>(null)
  const chatEndRef = useRef<HTMLDivElement>(null)
  const wsReconnectAttempts = useRef(0)
  const maxWsReconnectAttempts = 5

  // Ref to track current isBuilding state (prevents stale closure in WebSocket onclose)
  const isBuildingRef = useRef(isBuilding)
  useEffect(() => {
    isBuildingRef.current = isBuilding
  }, [isBuilding])

  const { user, createProject, setCurrentProject } = useStore()

  // Tech stack options
  const techStacks: TechStack[] = [
    { id: 'nextjs', name: 'Next.js', icon: <Globe className="w-5 h-5" />, category: 'frontend', description: 'React Framework' },
    { id: 'react', name: 'React', icon: <Layout className="w-5 h-5" />, category: 'frontend', description: 'UI Library' },
    { id: 'vue', name: 'Vue.js', icon: <Layers className="w-5 h-5" />, category: 'frontend', description: 'Progressive Framework' },
    { id: 'node', name: 'Node.js', icon: <Server className="w-5 h-5" />, category: 'backend', description: 'JavaScript Runtime' },
    { id: 'python', name: 'Python', icon: <Code2 className="w-5 h-5" />, category: 'backend', description: 'FastAPI/Django' },
    { id: 'go', name: 'Go', icon: <Zap className="w-5 h-5" />, category: 'backend', description: 'High Performance' },
    { id: 'postgresql', name: 'PostgreSQL', icon: <Database className="w-5 h-5" />, category: 'database', description: 'Relational DB' },
    { id: 'mongodb', name: 'MongoDB', icon: <Database className="w-5 h-5" />, category: 'database', description: 'Document DB' },
    { id: 'vercel', name: 'Vercel', icon: <Rocket className="w-5 h-5" />, category: 'deploy', description: 'Edge Deployment' },
    { id: 'docker', name: 'Docker', icon: <Server className="w-5 h-5" />, category: 'deploy', description: 'Containerized' },
  ]

  const toggleStack = (id: string) => {
    setSelectedStack(prev => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  // Scroll chat to bottom
  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [chatMessages])

  // Cleanup WebSocket
  useEffect(() => {
    return () => {
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [])

  // WebSocket URL builder
  const buildWebSocketUrl = useCallback((buildId: string): string => {
    const token = localStorage.getItem('apex_access_token')
    const appendToken = (url: string) => {
      if (!token) return url
      const separator = url.includes('?') ? '&' : '?'
      return `${url}${separator}token=${encodeURIComponent(token)}`
    }

    if (import.meta.env.VITE_WS_URL) {
      const baseWsUrl = import.meta.env.VITE_WS_URL.replace(/\/ws\/?$/, '').replace(/\/$/, '')
      return appendToken(`${baseWsUrl}/ws/build/${buildId}`)
    } else if (import.meta.env.VITE_API_URL) {
      const apiUrl = import.meta.env.VITE_API_URL.replace('/api/v1', '').replace(/\/$/, '')
      const wsProtocol = apiUrl.startsWith('https') ? 'wss' : 'ws'
      const wsHost = apiUrl.replace(/^https?:\/\//, '')
      return appendToken(`${wsProtocol}://${wsHost}/ws/build/${buildId}`)
    } else {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      return appendToken(`${protocol}//${window.location.host}/ws/build/${buildId}`)
    }
  }, [])

  // WebSocket connection
  const connectWebSocket = useCallback((buildId: string) => {
    const wsUrl = buildWebSocketUrl(buildId)
    console.log('Connecting to WebSocket:', wsUrl)

    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.close()
    }

    const ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      console.log('WebSocket connected')
      wsReconnectAttempts.current = 0
      addSystemMessage('Connected to build server')
    }

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data)
        handleWebSocketMessage(message)
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e)
      }
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }

    ws.onclose = (event) => {
      console.log('WebSocket disconnected, code:', event.code)

      // Use ref to access current isBuilding state (prevents stale closure)
      if (isBuildingRef.current && wsReconnectAttempts.current < maxWsReconnectAttempts) {
        wsReconnectAttempts.current++
        const delay = Math.min(1000 * Math.pow(2, wsReconnectAttempts.current - 1), 10000)
        addSystemMessage(`Connection lost. Reconnecting in ${delay / 1000}s...`)

        setTimeout(() => {
          if (isBuildingRef.current) {
            connectWebSocket(buildId)
          }
        }, delay)
      } else if (wsReconnectAttempts.current >= maxWsReconnectAttempts) {
        addSystemMessage('Connection failed after multiple attempts. Please refresh to retry.')
      }
    }

    wsRef.current = ws
  }, [buildWebSocketUrl])

  // Handle WebSocket messages
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
        setBuildState(prev => {
          if (!prev) return null
          const updates: Partial<BuildState> = { progress: data.progress }

          if (data.phase === 'provider_check' && data.available_providers) {
            updates.availableProviders = data.available_providers
            addSystemMessage(`AI Providers available: ${data.available_providers.join(', ')} (${data.provider_count} total)`)
          }

          if (data.inactivity_warning) {
            addSystemMessage(`${data.message}`)
          }

          return { ...prev, ...updates }
        })
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

      case 'agent:thinking':
        addAiThought(message.agent_id, data.agent_role, data.provider, 'thinking', data.content)
        break

      case 'agent:action':
        addAiThought(message.agent_id, data.agent_role, data.provider, 'action', data.content)
        break

      case 'agent:output':
        addAiThought(message.agent_id, data.agent_role, data.provider, 'output', data.content)
        break

      case 'build:error':
        addSystemMessage(`Build Error: ${data.error || 'Unknown error'}${data.details ? ` - ${data.details}` : ''}`)
        if (data.recoverable) {
          break
        }
        setIsBuilding(false)
        setBuildState(prev => prev ? { ...prev, status: 'failed' } : null)
        break

      case 'build:started':
        addSystemMessage('Build initialized, spawning agents...')
        setBuildState(prev => prev ? { ...prev, status: data.status || 'planning' } : null)
        break

      case 'agent:generation_failed':
        addSystemMessage(`AI generation failed for ${data.agent_role || 'agent'} (${data.provider || 'unknown'}): ${data.error || 'Unknown error'}`)
        if (data.retry_count !== undefined && data.max_retries !== undefined) {
          if (data.retry_count < data.max_retries) {
            addSystemMessage(`Retrying... (attempt ${data.retry_count + 1}/${data.max_retries})`)
          } else {
            addSystemMessage(`Max retries reached. The AI provider may be unavailable.`)
          }
        }
        break

      case 'agent:generating':
        addAiThought(message.agent_id, data.agent_role, data.provider, 'action', data.content || `Generating code with ${data.provider}...`)
        break

      case 'agent:retrying':
        addSystemMessage(`${data.agent_role || 'Agent'} retrying task (attempt ${data.retry_count}/${data.max_retries})...`)
        break

      case 'code:generated':
        addSystemMessage(`${data.agent_role || 'Agent'} generated ${data.files_count || 0} file(s)`)
        if (data.files && Array.isArray(data.files)) {
          data.files.forEach((file: any) => {
            if (file.path) {
              setGeneratedFiles(prev => [...prev, {
                path: file.path,
                content: file.content || '',
                language: file.language || 'text'
              }])
            }
          })
        }
        break

      case 'agent:progress':
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            agents: prev.agents.map(a =>
              a.id === message.agent_id ? { ...a, progress: data.progress || 0 } : a
            )
          }
        })
        break

      case 'preview:ready':
        if (data.url) {
          setPreviewUrl(data.url)
          addSystemMessage(`Preview ready: ${data.url}`)
        }
        break
    }
  }

  // Add AI thought
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
      const updated = [...prev, thought]
      return updated.slice(-100)
    })
    setTimeout(() => {
      aiActivityRef.current?.scrollTo({ top: aiActivityRef.current.scrollHeight, behavior: 'smooth' })
    }, 50)
  }

  // Add system message
  const addSystemMessage = (content: string) => {
    setChatMessages(prev => [...prev, {
      id: Date.now().toString(),
      role: 'system',
      content,
      timestamp: new Date(),
    }])
  }

  // Start build
  const startBuild = async () => {
    if (!appDescription.trim()) return

    setIsBuilding(true)
    setGeneratedFiles([])
    setAiThoughts([])
    setChatMessages([])
    wsReconnectAttempts.current = 0

    addSystemMessage(`Starting ${buildMode} build for: "${appDescription}"`)

    try {
      const response = await apiService.startBuild({
        description: appDescription,
        mode: buildMode,
      })

      if (!response || !response.build_id) {
        throw new Error('Invalid response from build API - no build_id returned')
      }

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

      connectWebSocket(buildId)
      addSystemMessage(`Build started! Build ID: ${buildId}`)

    } catch (error: unknown) {
      console.error('Build start failed:', error)

      let errorMsg = 'Unknown error occurred'

      // Type-safe error handling with proper narrowing
      const isAxiosError = (err: unknown): err is { response?: { data?: { error?: string; details?: string; message?: string }; status?: number }; message?: string } => {
        return typeof err === 'object' && err !== null
      }

      if (isAxiosError(error)) {
        if (error.response?.data?.error) {
          errorMsg = error.response.data.error
        } else if (error.response?.data?.details) {
          errorMsg = error.response.data.details
        } else if (error.response?.data?.message) {
          errorMsg = error.response.data.message
        } else if (error.message) {
          errorMsg = error.message
        }

        if (error.response?.status === 401) {
          errorMsg = 'Authentication required. Please log in to start a build.'
        } else if (error.response?.status === 403) {
          errorMsg = 'You do not have permission to start builds.'
        } else if (error.response?.status === 429) {
          errorMsg = 'Too many requests. Please wait a moment before trying again.'
        } else if (error.response?.status && error.response.status >= 500) {
          errorMsg = 'Server error. Please try again later.'
        } else if (!error.response && error.message?.includes('Network')) {
          errorMsg = 'Network error. Please check your connection and try again.'
        }
      } else if (error instanceof Error) {
        errorMsg = error.message
      }

      addSystemMessage(`Error: ${errorMsg}`)
      setIsBuilding(false)
      setBuildState(null)
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

    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        type: 'user:message',
        content: chatInput,
      }))
    }

    setChatInput('')
  }

  // Create project and open in IDE
  const openInIDE = async () => {
    if (generatedFiles.length === 0) {
      addSystemMessage('No files to create project from')
      return
    }

    setIsCreatingProject(true)
    try {
      const projectName = appDescription.slice(0, 50).replace(/[^a-zA-Z0-9\s-]/g, '').trim() || 'Generated App'

      const extensions = generatedFiles.map(f => f.path.split('.').pop()?.toLowerCase() || '')
      let language = 'javascript'
      if (extensions.some(e => ['tsx', 'ts'].includes(e))) language = 'typescript'
      else if (extensions.some(e => ['py'].includes(e))) language = 'python'
      else if (extensions.some(e => ['go'].includes(e))) language = 'go'
      else if (extensions.some(e => ['rs'].includes(e))) language = 'rust'

      const project = await createProject({
        name: projectName,
        description: appDescription,
        language,
        is_public: false,
      })

      if (project) {
        for (const file of generatedFiles) {
          try {
            await apiService.createFile(project.id, {
              path: file.path,
              name: file.path.split('/').pop() || file.path,
              type: 'file',
              content: file.content,
            })
          } catch (err) {
            console.error(`Failed to save file ${file.path}:`, err)
          }
        }

        setCreatedProjectId(project.id)
        setCurrentProject(project)
        addSystemMessage(`Project "${projectName}" created with ${generatedFiles.length} files!`)

        if (onNavigateToIDE) {
          onNavigateToIDE()
        }
      }
    } catch (error: unknown) {
      console.error('Failed to create project:', error)
      const message = error instanceof Error ? error.message : 'Unknown error'
      addSystemMessage(`Failed to create project: ${message}`)
    } finally {
      setIsCreatingProject(false)
    }
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

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'working': return <Circle className="w-4 h-4 fill-red-400 text-red-400 animate-pulse" />
      case 'completed': return <CheckCircle2 className="w-4 h-4 text-green-400" />
      case 'error': return <AlertCircle className="w-4 h-4 text-orange-400" />
      default: return <Circle className="w-4 h-4 text-gray-500" />
    }
  }

  const [showImportModal, setShowImportModal] = useState(false)
  const [showGitHubImport, setShowGitHubImport] = useState(false)
  const [replitUrl, setReplitUrl] = useState('')
  const [isImporting, setIsImporting] = useState(false)

  const handleReplitImport = async () => {
    if (!replitUrl.trim()) return
    setIsImporting(true)
    try {
      alert('Replit import initialized. Our agents are analyzing the project...')
      setShowImportModal(false)
      setAppDescription(`Imported from Replit: ${replitUrl}`)
      startBuild()
    } catch (error) {
      console.error('Import failed:', error)
    } finally {
      setIsImporting(false)
    }
  }

  // ============================================================================
  // RENDER
  // ============================================================================

  return (
    <div className="min-h-screen overflow-y-auto bg-black text-white relative">
      {/* CSS Keyframe Animations */}
      <style>{`
        @keyframes gradient-shift {
          0%, 100% { background-position: 0% 50%; }
          50% { background-position: 100% 50%; }
        }
        @keyframes scan-line {
          0% { transform: translateY(-100%); }
          100% { transform: translateY(400%); }
        }
        @keyframes scan-horizontal {
          0% { transform: translateX(-100%); }
          100% { transform: translateX(400%); }
        }
        @keyframes shimmer {
          0% { transform: translateX(-100%); }
          100% { transform: translateX(100%); }
        }
        @keyframes float {
          0%, 100% { transform: translateY(0); }
          50% { transform: translateY(-10px); }
        }
        @keyframes fade-in {
          from { opacity: 0; }
          to { opacity: 1; }
        }
        @keyframes fade-in-up {
          from { opacity: 0; transform: translateY(20px); }
          to { opacity: 1; transform: translateY(0); }
        }
        @keyframes ripple-expand {
          0% { transform: scale(0); opacity: 1; }
          100% { transform: scale(10); opacity: 0; }
        }
        @keyframes sparkle {
          0%, 100% { opacity: 0; transform: scale(0); }
          50% { opacity: 1; transform: scale(1); }
        }
        @keyframes bounce-slow {
          0%, 100% { transform: translateY(0); }
          50% { transform: translateY(-8px); }
        }
        .animate-gradient-shift {
          animation: gradient-shift 3s linear infinite;
        }
      `}</style>

      {/* Animated background layers */}
      <div className="fixed inset-0 pointer-events-none">
        {/* Base gradient */}
        <div className="absolute inset-0 bg-gradient-to-b from-red-950/30 via-black to-black" />

        {/* Animated background component */}
        <AnimatedBackground variant="full" intensity="low" interactive={false} className="opacity-40" />

        {/* Hex grid pattern */}
        <HexGrid />

        {/* Floating particles */}
        <FloatingParticles />

        {/* Radial gradient accents */}
        <div className="absolute top-0 left-1/4 w-[500px] h-[500px] bg-red-900/15 rounded-full blur-3xl" />
        <div className="absolute bottom-0 right-1/4 w-[500px] h-[500px] bg-orange-900/10 rounded-full blur-3xl" />
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[900px] h-[900px] bg-red-950/10 rounded-full blur-3xl" />
      </div>

      {/* Main content */}
      <div className="relative z-10 p-6 md:p-8 lg:p-12">
        {/* Replit Import Modal */}
        {showImportModal && (
          <div className="fixed inset-0 bg-black/95 flex items-center justify-center z-[100] p-4 backdrop-blur-md">
            <Card variant="cyberpunk" glow="intense" className="w-full max-w-lg border-2 border-red-600/60" style={{ animation: 'fade-in-up 0.3s ease-out' }}>
              <CardHeader>
                <CardTitle className="text-2xl flex items-center gap-3">
                  <Download className="w-7 h-7 text-red-500" />
                  Import from Replit
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-6">
                <p className="text-gray-400 leading-relaxed">
                  Enter the URL of the Replit project you want to migrate to APEX.BUILD.
                  Our agents will analyze the source and reconstruct it with optimized performance.
                </p>
                <div className="space-y-2">
                  <label className="text-sm font-semibold text-gray-300">Replit URL</label>
                  <input
                    type="text"
                    value={replitUrl}
                    onChange={(e) => setReplitUrl(e.target.value)}
                    placeholder="https://replit.com/@username/project-name"
                    className="w-full bg-black border-2 border-gray-700 rounded-xl px-4 py-3 text-white focus:border-red-600 focus:ring-2 focus:ring-red-900/30 outline-none transition-all"
                  />
                </div>
                <div className="flex gap-4">
                  <Button
                    onClick={handleReplitImport}
                    disabled={isImporting || !replitUrl.includes('replit.com')}
                    className="flex-1 bg-red-600 hover:bg-red-500 font-semibold"
                  >
                    {isImporting ? 'Analyzing...' : 'Start Migration'}
                  </Button>
                  <Button
                    variant="ghost"
                    onClick={() => setShowImportModal(false)}
                    className="border-2 border-gray-700 hover:bg-gray-900"
                  >
                    Cancel
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>
        )}

        {/* Header Section */}
        <div className="text-center mb-16 pt-8">
          <div className="flex items-center justify-center gap-6 mb-10">
            <AnimatedLogo />
          </div>
          <AnimatedTitle />
          <div className="mt-6">
            <TypewriterSubtitle text="Describe what you want to build, and our AI agents will create it for you" />
          </div>
        </div>

        {/* Main Content */}
        {!buildState ? (
          // App Description Input
          <div className="max-w-4xl mx-auto">
            <Card variant="cyberpunk" glow="intense" className="border-2 border-red-900/40 bg-black/60 backdrop-blur-xl">
              <CardContent className="p-8 md:p-10">
                {/* Build Mode Toggle */}
                <div className="flex items-center justify-center gap-6 mb-10">
                  <button
                    onClick={() => setBuildMode('fast')}
                    className={cn(
                      'relative flex items-center gap-4 px-8 py-4 rounded-xl transition-all duration-300 overflow-hidden group',
                      buildMode === 'fast'
                        ? 'bg-gradient-to-r from-red-900/50 to-orange-900/40 border-2 border-red-500 text-red-400 shadow-xl shadow-red-900/40'
                        : 'bg-gray-900/60 border-2 border-gray-700 text-gray-400 hover:border-gray-600 hover:text-gray-300'
                    )}
                  >
                    {buildMode === 'fast' && (
                      <div className="absolute inset-0 bg-gradient-to-r from-red-600/10 via-orange-600/10 to-red-600/10 animate-pulse" />
                    )}
                    <Zap className={cn("w-6 h-6 relative z-10", buildMode === 'fast' && "animate-pulse")} />
                    <div className="relative z-10 text-left">
                      <span className="font-bold block text-lg">Fast Build</span>
                      <span className="text-sm opacity-70">~3-5 minutes</span>
                    </div>
                  </button>
                  <button
                    onClick={() => setBuildMode('full')}
                    className={cn(
                      'relative flex items-center gap-4 px-8 py-4 rounded-xl transition-all duration-300 overflow-hidden group',
                      buildMode === 'full'
                        ? 'bg-gradient-to-r from-red-900/50 to-orange-900/40 border-2 border-red-500 text-red-400 shadow-xl shadow-red-900/40'
                        : 'bg-gray-900/60 border-2 border-gray-700 text-gray-400 hover:border-gray-600 hover:text-gray-300'
                    )}
                  >
                    {buildMode === 'full' && (
                      <div className="absolute inset-0 bg-gradient-to-r from-red-600/10 via-orange-600/10 to-red-600/10 animate-pulse" />
                    )}
                    <Sparkles className={cn("w-6 h-6 relative z-10", buildMode === 'full' && "animate-pulse")} />
                    <div className="relative z-10 text-left">
                      <span className="font-bold block text-lg">Full Build</span>
                      <span className="text-sm opacity-70">10+ minutes</span>
                    </div>
                  </button>
                </div>

                {/* Premium Textarea */}
                <div className="mb-10">
                  <PremiumTextarea
                    value={appDescription}
                    onChange={setAppDescription}
                    maxLength={2000}
                  />
                </div>

                {/* Tech Stack Selection */}
                <div className="mb-10">
                  <h3 className="text-xl font-bold text-gray-200 mb-5 flex items-center gap-3">
                    <Cpu className="w-6 h-6 text-red-400" />
                    Technology Stack
                  </h3>
                  <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-3">
                    {techStacks.map((stack) => (
                      <TechStackCard
                        key={stack.id}
                        stack={stack}
                        isSelected={selectedStack.has(stack.id)}
                        onClick={() => toggleStack(stack.id)}
                      />
                    ))}
                  </div>
                </div>

                {/* Epic Build Button */}
                <div className="space-y-4">
                  <EpicBuildButton
                    onClick={startBuild}
                    disabled={!appDescription.trim()}
                    isLoading={isBuilding}
                  />

                  <Button
                    onClick={() => setShowImportModal(true)}
                    variant="outline"
                    size="lg"
                    className="w-full h-14 border-2 border-red-900/60 text-red-400 hover:bg-red-950/40 hover:border-red-700 transition-all duration-300 font-semibold"
                  >
                    <Download className="w-5 h-5 mr-3" />
                    Migrate from Replit
                  </Button>

                  <Button
                    onClick={() => setShowGitHubImport(true)}
                    variant="outline"
                    size="lg"
                    className="w-full h-14 border-2 border-gray-700 text-gray-300 hover:bg-gray-800 hover:border-gray-600 transition-all duration-300 font-semibold"
                  >
                    <Github className="w-5 h-5 mr-3" />
                    Import from GitHub
                  </Button>
                </div>

                {/* Example Apps */}
                <div className="mt-10 pt-8 border-t border-gray-800">
                  <p className="text-sm text-gray-500 mb-4 font-medium">Quick examples:</p>
                  <div className="flex flex-wrap gap-3">
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
                        className="px-5 py-2.5 text-sm bg-gray-900/80 hover:bg-gray-800 text-gray-300 rounded-lg transition-all duration-200 border border-gray-800 hover:border-red-900/60 hover:text-white hover:shadow-lg hover:shadow-red-900/20"
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
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 max-w-7xl mx-auto">
            {/* Left Column - Agents & Status */}
            <div className="lg:col-span-1 space-y-6">
              {/* Build Status */}
              <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm overflow-hidden">
                <CardHeader className="pb-4 border-b border-gray-800">
                  <CardTitle className="text-xl flex items-center gap-3">
                    <div className="relative">
                      <Bot className="w-7 h-7 text-red-400" />
                      {buildState.status === 'in_progress' && (
                        <div className="absolute inset-0">
                          <Bot className="w-7 h-7 text-red-400 animate-ping opacity-50" />
                        </div>
                      )}
                    </div>
                    Build Status
                  </CardTitle>
                </CardHeader>
                <CardContent className="pt-5">
                  {/* Progress Bar */}
                  <div className="mb-6">
                    <div className="flex items-center justify-between mb-3">
                      <span className="text-sm text-gray-400 font-medium">Progress</span>
                      <span className="text-xl font-mono font-black text-red-400">{buildState.progress}%</span>
                    </div>
                    <div className="h-4 bg-gray-900 rounded-full overflow-hidden border border-gray-800">
                      <div
                        className="h-full rounded-full transition-all duration-500 relative overflow-hidden"
                        style={{
                          width: `${buildState.progress}%`,
                          background: 'linear-gradient(90deg, #dc2626, #ea580c, #dc2626)',
                          backgroundSize: '200% auto',
                          animation: 'gradient-shift 2s linear infinite',
                        }}
                      >
                        <div
                          className="absolute inset-0 bg-gradient-to-r from-transparent via-white/30 to-transparent"
                          style={{ animation: 'shimmer 1s linear infinite' }}
                        />
                      </div>
                    </div>
                  </div>

                  {/* Status Badge */}
                  <div className="flex items-center gap-3">
                    <Badge
                      variant={buildState.status === 'completed' ? 'success' : buildState.status === 'failed' ? 'error' : 'primary'}
                      className="capitalize text-sm px-4 py-1.5 font-semibold"
                    >
                      {buildState.status === 'in_progress' && <Circle className="w-2 h-2 mr-2 fill-current animate-pulse" />}
                      {buildState.status.replace('_', ' ')}
                    </Badge>
                    {buildState.status === 'in_progress' && (
                      <span className="text-xs text-gray-400">
                        {buildState.agents.filter(a => a.status === 'working').length} agents working
                      </span>
                    )}
                  </div>

                  {/* Available AI Providers */}
                  {buildState.availableProviders && buildState.availableProviders.length > 0 && (
                    <div className="mt-5 pt-5 border-t border-gray-800">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-xs text-gray-500 font-medium">AI Providers:</span>
                        {buildState.availableProviders.map((provider) => (
                          <Badge
                            key={provider}
                            variant="outline"
                            className={cn(
                              'text-xs',
                              provider === 'claude' && 'border-orange-500/60 text-orange-400 bg-orange-500/10',
                              provider === 'gpt' && 'border-green-500/60 text-green-400 bg-green-500/10',
                              provider === 'gemini' && 'border-blue-500/60 text-blue-400 bg-blue-500/10'
                            )}
                          >
                            {provider === 'claude' ? 'Claude' : provider === 'gpt' ? 'GPT-4' : provider === 'gemini' ? 'Gemini' : provider}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>

              {/* Active Agents */}
              <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm">
                <CardHeader className="pb-4 border-b border-gray-800">
                  <CardTitle className="text-xl flex items-center gap-3">
                    <Cpu className="w-7 h-7 text-orange-500" />
                    AI Agents ({buildState.agents.length})
                  </CardTitle>
                </CardHeader>
                <CardContent className="pt-5">
                  <div className="space-y-3">
                    {buildState.agents.map((agent, index) => (
                      <AgentCard
                        key={agent.id}
                        agent={agent}
                        index={index}
                        getAgentEmoji={getAgentEmoji}
                        getStatusIcon={getStatusIcon}
                      />
                    ))}

                    {buildState.agents.length === 0 && (
                      <div className="text-center text-gray-500 py-10">
                        <Bot className="w-14 h-14 mx-auto mb-4 opacity-50 animate-pulse" />
                        <p className="font-medium">Spawning agents...</p>
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>

              {/* Checkpoints */}
              {buildState.checkpoints.length > 0 && (
                <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm">
                  <CardHeader className="pb-4 border-b border-gray-800">
                    <CardTitle className="text-xl flex items-center gap-3">
                      <CheckCircle2 className="w-7 h-7 text-green-400" />
                      Checkpoints
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="pt-5">
                    <div className="space-y-3">
                      {buildState.checkpoints.map((cp, index) => (
                        <div
                          key={cp.id}
                          className="flex items-center gap-4 p-3 rounded-xl bg-gray-900/60 border border-gray-800"
                          style={{ animation: 'fade-in-up 0.3s ease-out', animationDelay: `${index * 100}ms` }}
                        >
                          <div className="w-9 h-9 rounded-full bg-green-500/20 flex items-center justify-center text-sm font-bold text-green-400 border border-green-500/40">
                            {cp.number}
                          </div>
                          <div className="flex-1 min-w-0">
                            <p className="text-sm font-semibold text-white">{cp.name}</p>
                            <p className="text-xs text-gray-500">{cp.progress}% complete</p>
                          </div>
                          <Button size="xs" variant="ghost" className="shrink-0 hover:bg-gray-800">
                            <RotateCcw className="w-4 h-4" />
                          </Button>
                        </div>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              )}

              {/* AI Activity Panel */}
              <Card variant="cyberpunk" className="border-2 border-purple-900/60 bg-black/60 backdrop-blur-sm">
                <CardHeader className="pb-4 border-b border-purple-900/40">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xl flex items-center gap-3">
                      <Bot className="w-7 h-7 text-purple-400 animate-pulse" />
                      AI Thinking
                    </CardTitle>
                    <Button
                      size="xs"
                      variant="ghost"
                      onClick={() => setShowAiActivity(!showAiActivity)}
                      className="hover:bg-purple-900/30"
                    >
                      <Eye className="w-5 h-5" />
                    </Button>
                  </div>
                </CardHeader>
                {showAiActivity && (
                  <CardContent className="pt-5">
                    <div
                      ref={aiActivityRef}
                      className="space-y-2 max-h-72 overflow-y-auto scrollbar-thin scrollbar-thumb-purple-900 scrollbar-track-gray-900"
                    >
                      {aiThoughts.length === 0 ? (
                        <div className="text-center text-gray-500 py-10">
                          <Bot className="w-14 h-14 mx-auto mb-4 opacity-50 animate-pulse" />
                          <p className="font-medium">Waiting for AI activity...</p>
                        </div>
                      ) : (
                        aiThoughts.map((thought) => (
                          <div
                            key={thought.id}
                            className={cn(
                              'p-3 rounded-lg text-sm border-l-4 transition-all',
                              thought.type === 'thinking' && 'bg-purple-900/30 border-purple-500',
                              thought.type === 'action' && 'bg-cyan-900/30 border-cyan-500',
                              thought.type === 'output' && 'bg-green-900/30 border-green-500',
                              thought.type === 'error' && 'bg-red-900/30 border-red-500'
                            )}
                            style={{ animation: 'fade-in 0.2s ease-out' }}
                          >
                            <div className="flex items-center gap-2 mb-1.5">
                              <span className="text-xs font-semibold text-gray-400">
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
                              'text-xs leading-relaxed',
                              thought.type === 'thinking' && 'text-purple-300 italic',
                              thought.type === 'action' && 'text-cyan-300',
                              thought.type === 'output' && 'text-green-300 font-mono',
                              thought.type === 'error' && 'text-red-300'
                            )}>
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

            {/* Middle/Right Column - Activity & Chat */}
            <div className="lg:col-span-2 space-y-6">
              {/* App Description */}
              <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm">
                <CardContent className="p-5">
                  <div className="flex items-start gap-4">
                    <div className="w-14 h-14 bg-gradient-to-br from-red-500 to-orange-600 rounded-xl flex items-center justify-center shrink-0 shadow-lg shadow-red-900/40">
                      <Code2 className="w-7 h-7 text-white" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm text-gray-400 mb-1 font-medium">Building</p>
                      <p className="text-white font-bold text-xl">{buildState.description}</p>
                    </div>
                  </div>
                </CardContent>
              </Card>

              {/* Terminal Output / Chat Interface */}
              <Card variant="cyberpunk" className="border-2 border-gray-800 bg-black/60 backdrop-blur-sm flex flex-col">
                <CardHeader className="pb-4 border-b border-gray-800 shrink-0">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xl flex items-center gap-3">
                      <Terminal className="w-7 h-7 text-red-400" />
                      Build Activity
                    </CardTitle>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => setShowChat(!showChat)}
                      className="hover:bg-gray-800"
                    >
                      {showChat ? 'Hide Chat' : 'Show Chat'}
                    </Button>
                  </div>
                </CardHeader>
                <CardContent className="flex-1 flex flex-col overflow-hidden p-5">
                  {/* Terminal Output */}
                  <TerminalOutput messages={chatMessages} isBuilding={isBuilding} />

                  {/* Chat Input */}
                  {showChat && (
                    <div className="pt-4 mt-4 border-t border-gray-800 shrink-0">
                      <div className="flex gap-3">
                        <input
                          type="text"
                          value={chatInput}
                          onChange={(e) => setChatInput(e.target.value)}
                          onKeyDown={(e) => e.key === 'Enter' && sendChatMessage()}
                          placeholder="Message the lead agent..."
                          className="flex-1 bg-black border-2 border-gray-700 rounded-xl px-5 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-red-600 focus:ring-2 focus:ring-red-900/30 transition-all"
                        />
                        <Button
                          onClick={sendChatMessage}
                          disabled={!chatInput.trim()}
                          className="px-6 bg-red-600 hover:bg-red-500 font-semibold"
                        >
                          <Send className="w-5 h-5" />
                        </Button>
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>

              {/* Actions and Preview */}
              {buildState.status === 'completed' && (
                <>
                  <BuildCompleteCard
                    filesCount={generatedFiles.length}
                    onPreview={() => setShowPreview(!showPreview)}
                    onOpenIDE={openInIDE}
                    isCreating={isCreatingProject}
                    showPreview={showPreview}
                  />

                  {/* Live Preview Panel */}
                  {showPreview && (
                    <Card variant="cyberpunk" className="border-2 border-red-600/40 bg-black/60 backdrop-blur-sm overflow-hidden">
                      <CardHeader className="pb-4 border-b border-gray-800">
                        <div className="flex items-center justify-between">
                          <CardTitle className="text-xl flex items-center gap-3">
                            <Eye className="w-7 h-7 text-red-400" />
                            Live Preview
                          </CardTitle>
                          {previewUrl && (
                            <a
                              href={previewUrl}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-sm text-red-400 hover:text-orange-400 flex items-center gap-2 transition-colors font-medium"
                            >
                              <ExternalLink className="w-4 h-4" />
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
                          <div className="h-[500px] flex flex-col items-center justify-center bg-gray-900/60 rounded-b-lg">
                            <FileCode className="w-20 h-20 text-gray-600 mb-6 animate-pulse" />
                            <p className="text-gray-400 text-center mb-3 text-xl font-semibold">
                              Preview not available yet
                            </p>
                            <p className="text-gray-500 text-sm text-center max-w-md leading-relaxed">
                              The generated app needs to be deployed to view.
                              Click "Open in IDE" to view the code and run it locally.
                            </p>
                            {generatedFiles.length > 0 && (
                              <div className="mt-6 p-5 bg-gray-800/60 rounded-xl max-w-md w-full border border-gray-700">
                                <p className="text-xs text-gray-400 mb-3 font-semibold">Generated files:</p>
                                <div className="space-y-2 max-h-36 overflow-y-auto">
                                  {generatedFiles.map((file, idx) => (
                                    <div
                                      key={idx}
                                      className="flex items-center gap-3 text-xs"
                                      style={{ animation: 'fade-in 0.2s ease-out', animationDelay: `${idx * 50}ms` }}
                                    >
                                      <FileCode className="w-4 h-4 text-red-400 shrink-0" />
                                      <span className="text-gray-300 truncate font-mono">{file.path}</span>
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

      {/* GitHub Import Modal */}
      {showGitHubImport && (
        <div className="fixed inset-0 bg-black/90 backdrop-blur-sm z-[100] flex items-center justify-center p-4">
          <GitHubImportWizard onClose={() => setShowGitHubImport(false)} />
        </div>
      )}
    </div>
  )
}

export default AppBuilder
