// APEX.BUILD Agent Progress Component
// Visual progress indicator with step highlighting and time tracking

import React, { memo, useMemo } from 'react'
import { cn } from '@/lib/utils'
import { useThemeLogo } from '@/hooks/useThemeLogo'
import {
  AgentTask,
  AgentStep,
  AgentStatus,
  AgentStepStatus,
  AgentStepType,
} from '@/types/agent'
import { Progress } from '@/components/ui/progress'
import {
  Brain,
  FileCode,
  FolderTree,
  Code,
  TestTube,
  Shield,
  Wrench,
  Rocket,
  History,
  CheckCircle2,
  XCircle,
  Circle,
  Loader2,
  Clock,
  Zap,
  Pause,
} from 'lucide-react'

export interface AgentProgressProps {
  task: AgentTask
  className?: string
  compact?: boolean
}

const DeployIcon = () => {
  const logoSrc = useThemeLogo()
  return <img src={logoSrc} alt="Deploy" className="w-4 h-4 object-contain" />
}

// Step type icons
const stepIcons: Record<AgentStepType, React.ReactNode> = {
  analyze: <Brain className="w-4 h-4" />,
  plan: <FileCode className="w-4 h-4" />,
  scaffold: <FolderTree className="w-4 h-4" />,
  implement: <Code className="w-4 h-4" />,
  test: <TestTube className="w-4 h-4" />,
  review: <Shield className="w-4 h-4" />,
  fix: <Wrench className="w-4 h-4" />,
  deploy: <DeployIcon />,
  checkpoint: <History className="w-4 h-4" />,
}

// Status icons
const statusIcons: Record<AgentStepStatus, React.ReactNode> = {
  pending: <Circle className="w-4 h-4 text-gray-500" />,
  in_progress: <Loader2 className="w-4 h-4 text-cyan-400 animate-spin" />,
  completed: <CheckCircle2 className="w-4 h-4 text-green-400" />,
  failed: <XCircle className="w-4 h-4 text-red-400" />,
  skipped: <Circle className="w-4 h-4 text-gray-600" />,
}

// Status colors
const statusColors: Record<AgentStepStatus, string> = {
  pending: 'border-gray-600 bg-gray-900/50',
  in_progress: 'border-cyan-500/50 bg-cyan-900/20 shadow-[0_0_15px_rgba(0,255,255,0.2)]',
  completed: 'border-green-500/50 bg-green-900/20',
  failed: 'border-red-500/50 bg-red-900/20',
  skipped: 'border-gray-700 bg-gray-900/30 opacity-50',
}

// Format time
function formatTime(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const mins = Math.floor(ms / 60000)
  const secs = Math.floor((ms % 60000) / 1000)
  return `${mins}m ${secs}s`
}

// Format estimated time
function formatEstimatedTime(ms: number | undefined): string {
  if (!ms) return '--'
  const mins = Math.floor(ms / 60000)
  const secs = Math.floor((ms % 60000) / 1000)
  if (mins === 0) return `~${secs}s`
  return `~${mins}m ${secs}s`
}

// Calculate remaining time
function calculateRemainingTime(task: AgentTask): string {
  if (!task.estimatedTimeMs || task.status !== 'executing') return '--'
  const remaining = task.estimatedTimeMs - task.elapsedTimeMs
  if (remaining <= 0) return 'Almost done...'
  return formatEstimatedTime(remaining)
}

// Get overall status color
function getStatusColor(status: AgentStatus): string {
  switch (status) {
    case 'idle':
      return 'text-gray-400'
    case 'initializing':
    case 'planning':
    case 'executing':
      return 'text-cyan-400'
    case 'paused':
      return 'text-yellow-400'
    case 'completed':
      return 'text-green-400'
    case 'failed':
    case 'cancelled':
      return 'text-red-400'
    default:
      return 'text-gray-400'
  }
}

// Get status label
function getStatusLabel(status: AgentStatus): string {
  switch (status) {
    case 'idle':
      return 'Ready'
    case 'initializing':
      return 'Initializing...'
    case 'planning':
      return 'Planning...'
    case 'executing':
      return 'Building...'
    case 'paused':
      return 'Paused'
    case 'completed':
      return 'Completed'
    case 'failed':
      return 'Failed'
    case 'cancelled':
      return 'Cancelled'
    default:
      return status
  }
}

// Step item component
const StepItem = memo<{
  step: AgentStep
  isActive: boolean
  index: number
  compact?: boolean
}>(({ step, isActive, index, compact }) => {
  return (
    <div
      className={cn(
        'relative flex items-start gap-3 p-3 rounded-lg border transition-all duration-300',
        statusColors[step.status],
        isActive && 'ring-1 ring-cyan-400/50'
      )}
    >
      {/* Step number and icon */}
      <div className="flex items-center gap-2 flex-shrink-0">
        <div
          className={cn(
            'w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold',
            step.status === 'completed' && 'bg-green-500/20 text-green-400',
            step.status === 'in_progress' && 'bg-cyan-500/20 text-cyan-400',
            step.status === 'failed' && 'bg-red-500/20 text-red-400',
            step.status === 'pending' && 'bg-gray-700/50 text-gray-400',
            step.status === 'skipped' && 'bg-gray-800/50 text-gray-500'
          )}
        >
          {step.status === 'completed' ? (
            <CheckCircle2 className="w-4 h-4" />
          ) : step.status === 'in_progress' ? (
            <Loader2 className="w-4 h-4 animate-spin" />
          ) : step.status === 'failed' ? (
            <XCircle className="w-4 h-4" />
          ) : (
            index + 1
          )}
        </div>
      </div>

      {/* Step content */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-gray-400">{stepIcons[step.type]}</span>
          <h4
            className={cn(
              'font-medium truncate',
              step.status === 'in_progress' && 'text-cyan-400',
              step.status === 'completed' && 'text-green-400',
              step.status === 'failed' && 'text-red-400',
              (step.status === 'pending' || step.status === 'skipped') && 'text-gray-300'
            )}
          >
            {step.title}
          </h4>
        </div>

        {!compact && (
          <p className="text-sm text-gray-500 mt-0.5 truncate">{step.description}</p>
        )}

        {/* Progress bar for active step */}
        {step.status === 'in_progress' && (
          <div className="mt-2">
            <Progress
              value={step.progress}
              variant="cyberpunk"
              size="sm"
              className="h-1.5"
            />
          </div>
        )}

        {/* Duration */}
        {step.duration && step.status === 'completed' && (
          <div className="flex items-center gap-1 mt-1 text-xs text-gray-500">
            <Clock className="w-3 h-3" />
            <span>{formatTime(step.duration)}</span>
          </div>
        )}

        {/* Error message */}
        {step.error && step.status === 'failed' && (
          <p className="text-xs text-red-400 mt-1 truncate">{step.error}</p>
        )}

        {/* Output preview */}
        {step.output && step.status === 'in_progress' && !compact && (
          <div className="mt-2 p-2 bg-black/30 rounded text-xs text-gray-400 font-mono truncate">
            {step.output.split('\n').slice(-1)[0]}
          </div>
        )}
      </div>
    </div>
  )
})
StepItem.displayName = 'StepItem'

// Main progress component
export const AgentProgress: React.FC<AgentProgressProps> = memo(({
  task,
  className,
  compact = false,
}) => {
  // Calculate completed steps
  const completedSteps = useMemo(
    () => task.steps.filter((s) => s.status === 'completed').length,
    [task.steps]
  )

  // Get current step
  const currentStep = useMemo(
    () => task.steps.find((s) => s.status === 'in_progress'),
    [task.steps]
  )

  return (
    <div className={cn('space-y-4', className)}>
      {/* Header with overall progress */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          {/* Status indicator */}
          <div
            className={cn(
              'flex items-center gap-2 px-3 py-1.5 rounded-full border',
              task.status === 'executing' && 'border-cyan-500/50 bg-cyan-900/20',
              task.status === 'paused' && 'border-yellow-500/50 bg-yellow-900/20',
              task.status === 'completed' && 'border-green-500/50 bg-green-900/20',
              task.status === 'failed' && 'border-red-500/50 bg-red-900/20',
              (task.status === 'idle' || task.status === 'initializing' || task.status === 'planning') &&
                'border-gray-600 bg-gray-900/50'
            )}
          >
            {task.status === 'executing' && (
              <Loader2 className="w-4 h-4 text-cyan-400 animate-spin" />
            )}
            {task.status === 'paused' && <Pause className="w-4 h-4 text-yellow-400" />}
            {task.status === 'completed' && (
              <CheckCircle2 className="w-4 h-4 text-green-400" />
            )}
            {task.status === 'failed' && <XCircle className="w-4 h-4 text-red-400" />}
            {(task.status === 'idle' || task.status === 'initializing' || task.status === 'planning') && (
              <Zap className="w-4 h-4 text-gray-400" />
            )}
            <span className={cn('text-sm font-medium', getStatusColor(task.status))}>
              {getStatusLabel(task.status)}
            </span>
          </div>

          {/* Step counter */}
          <span className="text-sm text-gray-400">
            {completedSteps}/{task.steps.length} steps
          </span>
        </div>

        {/* Time info */}
        <div className="flex items-center gap-4 text-sm">
          <div className="flex items-center gap-1 text-gray-400">
            <Clock className="w-4 h-4" />
            <span>{formatTime(task.elapsedTimeMs)}</span>
          </div>
          {task.status === 'executing' && (
            <div className="flex items-center gap-1 text-cyan-400">
              <span>ETA:</span>
              <span>{calculateRemainingTime(task)}</span>
            </div>
          )}
        </div>
      </div>

      {/* Overall progress bar */}
      <div className="space-y-1">
        <Progress
          value={task.progress}
          variant={
            task.status === 'failed'
              ? 'synthwave'
              : task.status === 'completed'
              ? 'matrix'
              : 'cyberpunk'
          }
          size="md"
          showValue
          animated={task.status === 'executing'}
        />
      </div>

      {/* Current step highlight */}
      {currentStep && (
        <div className="p-3 bg-cyan-900/10 border border-cyan-500/30 rounded-lg">
          <div className="flex items-center gap-2 text-cyan-400">
            <Loader2 className="w-4 h-4 animate-spin" />
            <span className="font-medium">{currentStep.title}</span>
            {currentStep.progress > 0 && (
              <span className="text-sm text-cyan-300/70">({currentStep.progress}%)</span>
            )}
          </div>
          {currentStep.output && (
            <p className="mt-1 text-sm text-gray-400 font-mono truncate">
              {currentStep.output.split('\n').slice(-1)[0]}
            </p>
          )}
        </div>
      )}

      {/* Step list */}
      {!compact && (
        <div className="space-y-2 max-h-80 overflow-y-auto pr-2 custom-scrollbar">
          {task.steps.map((step, index) => (
            <StepItem
              key={step.id}
              step={step}
              isActive={step.id === currentStep?.id}
              index={index}
              compact={compact}
            />
          ))}
        </div>
      )}

      {/* Compact step indicators */}
      {compact && (
        <div className="flex items-center gap-1">
          {task.steps.map((step, index) => (
            <div
              key={step.id}
              className={cn(
                'flex-1 h-1.5 rounded-full transition-all duration-300',
                step.status === 'completed' && 'bg-green-500',
                step.status === 'in_progress' && 'bg-cyan-500 animate-pulse',
                step.status === 'failed' && 'bg-red-500',
                step.status === 'pending' && 'bg-gray-700',
                step.status === 'skipped' && 'bg-gray-800'
              )}
              title={`${step.title}: ${step.status}`}
            />
          ))}
        </div>
      )}
    </div>
  )
})

AgentProgress.displayName = 'AgentProgress'

export default AgentProgress
