// APEX.BUILD 100% Success Guarantee Badge
// A showpiece component — animated shield with real-time build confidence score,
// pulsing when builds are in progress, triumphant glow on success.

import React, { useMemo } from 'react'
import { cn } from '@/lib/utils'

export interface GuaranteeBadgeProps {
  status: 'idle' | 'building' | 'validating' | 'retrying' | 'success' | 'failed' | 'rolling_back'
  score: number // 0–100
  attempt: number
  maxRetries: number
  className?: string
  size?: 'sm' | 'md' | 'lg'
}

const statusConfig = {
  idle: {
    label: 'READY',
    color: 'text-gray-400',
    ringColor: 'ring-gray-600/30',
    bgGlow: '',
    pulseClass: '',
  },
  building: {
    label: 'BUILDING',
    color: 'text-amber-400',
    ringColor: 'ring-amber-500/40',
    bgGlow: 'shadow-amber-500/20',
    pulseClass: 'animate-pulse',
  },
  validating: {
    label: 'VALIDATING',
    color: 'text-blue-400',
    ringColor: 'ring-blue-500/40',
    bgGlow: 'shadow-blue-500/20',
    pulseClass: 'animate-pulse',
  },
  retrying: {
    label: 'RETRYING',
    color: 'text-orange-400',
    ringColor: 'ring-orange-500/40',
    bgGlow: 'shadow-orange-500/20',
    pulseClass: 'animate-bounce',
  },
  success: {
    label: '100% GUARANTEED',
    color: 'text-emerald-400',
    ringColor: 'ring-emerald-500/50',
    bgGlow: 'shadow-emerald-500/30',
    pulseClass: '',
  },
  failed: {
    label: 'ROLLED BACK',
    color: 'text-red-400',
    ringColor: 'ring-red-500/40',
    bgGlow: 'shadow-red-500/20',
    pulseClass: '',
  },
  rolling_back: {
    label: 'ROLLING BACK',
    color: 'text-red-400',
    ringColor: 'ring-red-500/40',
    bgGlow: 'shadow-red-500/20',
    pulseClass: 'animate-pulse',
  },
}

const sizeConfig = {
  sm: { outer: 'w-16 h-16', inner: 'text-xs', scoreSize: 'text-sm' },
  md: { outer: 'w-24 h-24', inner: 'text-xs', scoreSize: 'text-lg' },
  lg: { outer: 'w-32 h-32', inner: 'text-sm', scoreSize: 'text-2xl' },
}

export const GuaranteeBadge: React.FC<GuaranteeBadgeProps> = ({
  status,
  score,
  attempt,
  maxRetries,
  className,
  size = 'md',
}) => {
  const config = statusConfig[status]
  const sizeConf = sizeConfig[size]

  // SVG shield path for the badge shape
  const shieldPath = useMemo(() => {
    return 'M12 2L3 7v5c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V7L12 2z'
  }, [])

  // Score ring circumference
  const circumference = 2 * Math.PI * 40
  const strokeDashoffset = circumference - (score / 100) * circumference

  return (
    <div className={cn('relative inline-flex flex-col items-center gap-2', className)}>
      {/* Main badge circle */}
      <div
        className={cn(
          'relative rounded-full flex items-center justify-center',
          'bg-gradient-to-br from-gray-900/90 to-gray-950/95',
          'ring-2',
          config.ringColor,
          'shadow-lg',
          config.bgGlow,
          config.pulseClass,
          sizeConf.outer,
        )}
      >
        {/* Score ring SVG */}
        <svg
          className="absolute inset-0 -rotate-90"
          viewBox="0 0 100 100"
        >
          {/* Background ring */}
          <circle
            cx="50"
            cy="50"
            r="40"
            fill="none"
            stroke="currentColor"
            strokeWidth="3"
            className="text-gray-800/50"
          />
          {/* Score progress ring */}
          <circle
            cx="50"
            cy="50"
            r="40"
            fill="none"
            stroke="currentColor"
            strokeWidth="3"
            strokeLinecap="round"
            strokeDasharray={circumference}
            strokeDashoffset={strokeDashoffset}
            className={cn(config.color, 'transition-all duration-1000 ease-out')}
          />
        </svg>

        {/* Shield icon in center */}
        <div className="relative z-10 flex flex-col items-center">
          <svg
            viewBox="0 0 24 24"
            className={cn('w-6 h-6 fill-current', config.color)}
          >
            <path d={shieldPath} />
            {status === 'success' && (
              <path
                d="M9 12l2 2 4-4"
                fill="none"
                stroke="#0a0a0a"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            )}
          </svg>
          <span className={cn('font-mono font-bold', config.color, sizeConf.scoreSize)}>
            {Math.round(score)}%
          </span>
        </div>

        {/* Success glow overlay */}
        {status === 'success' && (
          <div className="absolute inset-0 rounded-full bg-emerald-500/10 animate-ping pointer-events-none" />
        )}
      </div>

      {/* Label */}
      <div className="flex flex-col items-center gap-0.5">
        <span className={cn('font-mono font-bold tracking-wider', config.color, sizeConf.inner)}>
          {config.label}
        </span>
        {(status === 'retrying' || status === 'building' || status === 'validating') && (
          <span className="text-[10px] text-gray-500 font-mono">
            Attempt {attempt}/{maxRetries}
          </span>
        )}
      </div>
    </div>
  )
}

export default GuaranteeBadge
