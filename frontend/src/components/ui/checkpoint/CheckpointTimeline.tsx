// APEX.BUILD Checkpoint Timeline UI
// Visual timeline of build checkpoints with rollback capability.
// Shows state transitions, validation results, and retry attempts.

import React, { useState } from 'react'
import { cn } from '@/lib/utils'
import type { AgentCheckpoint } from '@/types/agent'

export interface CheckpointTimelineProps {
  checkpoints: TimelineCheckpoint[]
  currentStepIndex: number
  onRollback?: (checkpointId: string) => void
  className?: string
}

export interface TimelineCheckpoint {
  id: string
  name: string
  description: string
  state: string
  stepIndex: number
  createdAt: string
  canRestore: boolean
  validationScore?: number
  verdict?: 'pass' | 'soft_fail' | 'hard_fail'
  attempt?: number
}

const stateColors: Record<string, { dot: string; line: string; bg: string }> = {
  idle:         { dot: 'bg-gray-500',    line: 'bg-gray-700',    bg: 'bg-gray-500/10' },
  initializing: { dot: 'bg-blue-400',    line: 'bg-blue-700',    bg: 'bg-blue-500/10' },
  planning:     { dot: 'bg-purple-400',  line: 'bg-purple-700',  bg: 'bg-purple-500/10' },
  executing:    { dot: 'bg-amber-400',   line: 'bg-amber-700',   bg: 'bg-amber-500/10' },
  validating:   { dot: 'bg-cyan-400',    line: 'bg-cyan-700',    bg: 'bg-cyan-500/10' },
  retrying:     { dot: 'bg-orange-400',  line: 'bg-orange-700',  bg: 'bg-orange-500/10' },
  rolling_back: { dot: 'bg-red-400',     line: 'bg-red-700',     bg: 'bg-red-500/10' },
  completed:    { dot: 'bg-emerald-400', line: 'bg-emerald-700', bg: 'bg-emerald-500/10' },
  failed:       { dot: 'bg-red-500',     line: 'bg-red-800',     bg: 'bg-red-500/10' },
  cancelled:    { dot: 'bg-gray-400',    line: 'bg-gray-600',    bg: 'bg-gray-500/10' },
}

const verdictBadge: Record<string, { text: string; color: string }> = {
  pass:      { text: 'PASS',      color: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/30' },
  soft_fail: { text: 'SOFT FAIL', color: 'text-orange-400 bg-orange-500/10 border-orange-500/30' },
  hard_fail: { text: 'HARD FAIL', color: 'text-red-400 bg-red-500/10 border-red-500/30' },
}

export const CheckpointTimeline: React.FC<CheckpointTimelineProps> = ({
  checkpoints,
  currentStepIndex,
  onRollback,
  className,
}) => {
  const [expandedId, setExpandedId] = useState<string | null>(null)

  if (checkpoints.length === 0) {
    return (
      <div className={cn('flex items-center justify-center py-8 text-gray-500 text-sm font-mono', className)}>
        No checkpoints yet
      </div>
    )
  }

  return (
    <div className={cn('relative', className)}>
      {/* Section header */}
      <div className="flex items-center gap-2 mb-4">
        <div className="w-2 h-2 rounded-full bg-amber-500 animate-pulse" />
        <h3 className="text-sm font-mono font-bold text-amber-300/90 tracking-wider uppercase">
          Build Checkpoints
        </h3>
        <span className="text-xs text-gray-500 font-mono ml-auto">
          {checkpoints.length} checkpoint{checkpoints.length !== 1 ? 's' : ''}
        </span>
      </div>

      {/* Timeline */}
      <div className="relative ml-3">
        {/* Vertical line */}
        <div className="absolute left-[7px] top-0 bottom-0 w-px bg-gradient-to-b from-amber-600/40 via-gray-700/30 to-transparent" />

        {checkpoints.map((cp, index) => {
          const colors = stateColors[cp.state] || stateColors.idle
          const isActive = cp.stepIndex === currentStepIndex
          const isPast = cp.stepIndex < currentStepIndex
          const isExpanded = expandedId === cp.id
          const badge = cp.verdict ? verdictBadge[cp.verdict] : null

          return (
            <div
              key={cp.id}
              className={cn(
                'relative pl-8 pb-6 last:pb-0 group',
                'transition-all duration-200',
              )}
            >
              {/* Timeline dot */}
              <div
                className={cn(
                  'absolute left-0 top-1 w-[15px] h-[15px] rounded-full',
                  'border-2 border-gray-800',
                  'transition-all duration-300',
                  colors.dot,
                  isActive && 'ring-2 ring-amber-400/50 scale-125',
                  isPast && 'opacity-60',
                )}
              />

              {/* Connecting line segment */}
              {index < checkpoints.length - 1 && (
                <div
                  className={cn(
                    'absolute left-[6px] top-[18px] w-[3px] h-[calc(100%-18px)]',
                    'rounded-full',
                    colors.line,
                    isPast && 'opacity-40',
                  )}
                />
              )}

              {/* Checkpoint card */}
              <div
                onClick={() => setExpandedId(isExpanded ? null : cp.id)}
                className={cn(
                  'rounded-lg border transition-all duration-200 cursor-pointer',
                  'hover:border-amber-600/40',
                  isActive
                    ? 'bg-amber-500/5 border-amber-600/30'
                    : 'bg-gray-900/40 border-gray-700/30',
                  isExpanded && 'border-amber-500/40',
                )}
              >
                {/* Card header */}
                <div className="flex items-center gap-3 px-3 py-2">
                  {/* Step indicator */}
                  <span className={cn(
                    'text-[10px] font-mono font-bold px-1.5 py-0.5 rounded',
                    colors.bg,
                    isActive ? 'text-amber-300' : 'text-gray-400',
                  )}>
                    #{cp.stepIndex}
                  </span>

                  {/* Name */}
                  <span className={cn(
                    'text-sm font-mono flex-1 truncate',
                    isActive ? 'text-amber-200' : 'text-gray-300',
                  )}>
                    {cp.name}
                  </span>

                  {/* Verdict badge */}
                  {badge && (
                    <span className={cn(
                      'text-[9px] font-mono font-bold px-1.5 py-0.5 rounded border',
                      badge.color,
                    )}>
                      {badge.text}
                    </span>
                  )}

                  {/* Score */}
                  {cp.validationScore !== undefined && (
                    <span className={cn(
                      'text-xs font-mono font-bold',
                      cp.validationScore >= 80 ? 'text-emerald-400' :
                      cp.validationScore >= 50 ? 'text-amber-400' : 'text-red-400',
                    )}>
                      {cp.validationScore}%
                    </span>
                  )}

                  {/* Time */}
                  <span className="text-[10px] text-gray-500 font-mono">
                    {formatTime(cp.createdAt)}
                  </span>

                  {/* Expand indicator */}
                  <svg
                    className={cn(
                      'w-3 h-3 text-gray-500 transition-transform',
                      isExpanded && 'rotate-180',
                    )}
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
                  </svg>
                </div>

                {/* Expanded details */}
                {isExpanded && (
                  <div className="px-3 pb-3 border-t border-gray-800/50 mt-1 pt-2">
                    <p className="text-xs text-gray-400 font-mono mb-2">
                      {cp.description}
                    </p>

                    <div className="flex items-center gap-2 text-[10px] text-gray-500 font-mono mb-3">
                      <span>State: <span className="text-gray-300">{cp.state}</span></span>
                      {cp.attempt !== undefined && (
                        <span>Attempt: <span className="text-gray-300">{cp.attempt}</span></span>
                      )}
                      <span>ID: <span className="text-gray-300">{cp.id.slice(0, 8)}</span></span>
                    </div>

                    {/* Rollback button */}
                    {cp.canRestore && onRollback && cp.stepIndex < currentStepIndex && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          onRollback(cp.id)
                        }}
                        className={cn(
                          'w-full px-3 py-1.5 text-xs font-mono font-bold rounded',
                          'bg-red-500/10 text-red-400 border border-red-500/30',
                          'hover:bg-red-500/20 hover:border-red-500/50',
                          'transition-all duration-200',
                          'flex items-center justify-center gap-2',
                        )}
                      >
                        <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                          <path strokeLinecap="round" strokeLinejoin="round" d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6" />
                        </svg>
                        ROLLBACK TO THIS CHECKPOINT
                      </button>
                    )}
                  </div>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function formatTime(isoString: string): string {
  try {
    const date = new Date(isoString)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffSec = Math.floor(diffMs / 1000)

    if (diffSec < 60) return `${diffSec}s ago`
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`

    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  } catch {
    return ''
  }
}

export default CheckpointTimeline
