// APEX.BUILD Debug Call Stack View
// Displays the call stack with clickable frames to navigate source,
// highlights the current (top) frame

import React, { useCallback } from 'react'
import { cn } from '@/lib/utils'
import { StackFrame } from '@/services/debugService'
import {
  Layers,
  FileCode,
  ArrowRight,
  Zap,
} from 'lucide-react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface CallStackViewProps {
  className?: string
  frames: StackFrame[]
  activeFrameId?: string
  isPaused: boolean
  onFrameSelect?: (frame: StackFrame) => void
}

// ---------------------------------------------------------------------------
// CallStackView
// ---------------------------------------------------------------------------

const CallStackView: React.FC<CallStackViewProps> = ({
  className,
  frames,
  activeFrameId,
  isPaused,
  onFrameSelect,
}) => {
  const handleClick = useCallback(
    (frame: StackFrame) => {
      onFrameSelect?.(frame)
    },
    [onFrameSelect]
  )

  // Extract just the file name from a full path for compact display
  const fileName = (path: string): string => {
    const parts = path.split('/')
    return parts[parts.length - 1] || path
  }

  if (!isPaused) {
    return (
      <div className={cn('flex flex-col', className)}>
        <div className="flex items-center gap-2 px-3 py-2 text-xs text-gray-600 italic">
          <Layers className="w-3.5 h-3.5" />
          Not paused
        </div>
      </div>
    )
  }

  if (frames.length === 0) {
    return (
      <div className={cn('flex flex-col', className)}>
        <div className="flex items-center gap-2 px-3 py-2 text-xs text-gray-600 italic">
          <Layers className="w-3.5 h-3.5" />
          No call stack available
        </div>
      </div>
    )
  }

  return (
    <div className={cn('flex flex-col', className)}>
      {frames.map((frame, index) => {
        const isActive = activeFrameId
          ? frame.id === activeFrameId
          : index === 0
        const isTop = index === 0

        return (
          <button
            key={frame.id}
            onClick={() => handleClick(frame)}
            className={cn(
              'flex items-center gap-2 px-3 py-1.5 text-xs text-left transition-colors group',
              isActive
                ? 'bg-cyan-500/15 border-l-2 border-cyan-400 text-white'
                : 'border-l-2 border-transparent hover:bg-gray-800/60 text-gray-400 hover:text-gray-200'
            )}
          >
            {/* Frame index */}
            <span
              className={cn(
                'flex-shrink-0 w-5 h-5 flex items-center justify-center rounded text-[10px] font-mono',
                isTop
                  ? 'bg-cyan-500/20 text-cyan-300'
                  : 'bg-gray-800 text-gray-500'
              )}
            >
              {frame.index}
            </span>

            {/* Function name */}
            <span
              className={cn(
                'font-medium truncate',
                isActive ? 'text-white' : 'text-gray-300',
                frame.is_async && 'italic'
              )}
            >
              {frame.is_async && (
                <Zap className="w-3 h-3 inline mr-1 text-yellow-400" />
              )}
              {frame.function_name || '<anonymous>'}
            </span>

            {/* File location */}
            <span className="ml-auto flex items-center gap-1 flex-shrink-0 text-gray-500 group-hover:text-gray-400">
              <FileCode className="w-3 h-3" />
              <span className="font-mono truncate max-w-[100px]">
                {fileName(frame.file_path)}
              </span>
              <span className="text-gray-600">:</span>
              <span className="text-cyan-400/80 font-mono">
                {frame.line}
              </span>
            </span>

            {/* Navigate indicator */}
            <ArrowRight className="w-3 h-3 flex-shrink-0 opacity-0 group-hover:opacity-60 text-gray-400 transition-opacity" />
          </button>
        )
      })}
    </div>
  )
}

export { CallStackView }
export default CallStackView
