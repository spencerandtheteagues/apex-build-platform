// APEX.BUILD Cyberpunk Badge Component
// Neon status badges and tags

import React, { forwardRef } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const badgeVariants = cva(
  'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium transition-all duration-300 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2',
  {
    variants: {
      variant: {
        // Primary cyberpunk cyan
        primary: 'bg-cyan-500/20 text-cyan-300 border border-cyan-400/50 shadow-lg shadow-cyan-500/10',

        // Secondary pink accent
        secondary: 'bg-pink-500/20 text-pink-300 border border-pink-400/50 shadow-lg shadow-pink-500/10',

        // Success green
        success: 'bg-green-400/20 text-green-300 border border-green-400/50 shadow-lg shadow-green-400/10',

        // Warning yellow/orange
        warning: 'bg-yellow-500/20 text-yellow-300 border border-yellow-400/50 shadow-lg shadow-yellow-500/10',

        // Error red
        error: 'bg-red-500/20 text-red-300 border border-red-400/50 shadow-lg shadow-red-500/10',

        // Info blue
        info: 'bg-blue-500/20 text-blue-300 border border-blue-400/50 shadow-lg shadow-blue-500/10',

        // Default (alias for neutral)
        default: 'bg-gray-500/20 text-gray-300 border border-gray-400/50 shadow-lg shadow-gray-500/10',

        // Neutral gray
        neutral: 'bg-gray-500/20 text-gray-300 border border-gray-400/50 shadow-lg shadow-gray-500/10',

        // Matrix green theme
        matrix: 'bg-green-400/20 text-green-300 border border-green-400/60 shadow-lg shadow-green-400/20',

        // Synthwave theme
        synthwave: 'bg-gradient-to-r from-pink-500/20 to-purple-500/20 text-pink-300 border border-pink-400/50 shadow-lg shadow-pink-500/10',

        // Neon city theme
        neonCity: 'bg-blue-500/20 text-blue-300 border border-blue-400/60 shadow-lg shadow-blue-500/20',

        // Outline styles
        outline: 'border border-gray-600 text-gray-300 hover:bg-gray-800/50',
        outlinePrimary: 'border border-cyan-400 text-cyan-400 hover:bg-cyan-400/10',
        outlineSuccess: 'border border-green-400 text-green-400 hover:bg-green-400/10',
        outlineWarning: 'border border-yellow-400 text-yellow-400 hover:bg-yellow-400/10',
        outlineError: 'border border-red-400 text-red-400 hover:bg-red-400/10',
      },
      size: {
        xs: 'text-[10px] px-1.5 py-0.5',
        sm: 'text-xs px-2 py-0.5',
        md: 'text-xs px-2.5 py-1',
        lg: 'text-sm px-3 py-1.5',
        xl: 'text-base px-4 py-2',
      },
      glow: {
        none: '',
        subtle: 'shadow-lg',
        intense: 'shadow-xl animate-pulse',
      },
    },
    defaultVariants: {
      variant: 'primary',
      size: 'sm',
      glow: 'subtle',
    },
  }
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {
  icon?: React.ReactNode
  pulse?: boolean
  removable?: boolean
  onRemove?: () => void
}

const Badge = forwardRef<HTMLDivElement, BadgeProps>(
  ({ className, variant, size, glow, icon, pulse, removable, onRemove, children, ...props }, ref) => {
    return (
      <div
        className={cn(
          badgeVariants({ variant, size, glow }),
          pulse && 'animate-pulse',
          removable && 'pr-1',
          className
        )}
        ref={ref}
        {...props}
      >
        {/* Icon */}
        {icon && (
          <span className="mr-1 inline-flex items-center">
            {icon}
          </span>
        )}

        {/* Content */}
        <span className="truncate">{children}</span>

        {/* Remove button */}
        {removable && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              onRemove?.()
            }}
            className="ml-1 inline-flex items-center justify-center w-3 h-3 rounded-full hover:bg-current/20 focus:outline-none focus:bg-current/20 transition-colors"
          >
            <svg
              className="w-2 h-2"
              stroke="currentColor"
              fill="none"
              viewBox="0 0 8 8"
            >
              <path strokeLinecap="round" strokeWidth="1.5" d="M1 1l6 6m0-6L1 7" />
            </svg>
          </button>
        )}

        {/* Holographic shimmer effect */}
        <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/10 to-transparent -translate-x-full group-hover:translate-x-full transition-transform duration-1000 ease-out rounded-full" />
      </div>
    )
  }
)

Badge.displayName = 'Badge'

// Status badge variants for common use cases
export const StatusBadge = ({ status, ...props }: { status: 'online' | 'offline' | 'away' | 'busy' } & Omit<BadgeProps, 'variant'>) => {
  const variantMap = {
    online: 'success',
    offline: 'neutral',
    away: 'warning',
    busy: 'error',
  } as const

  const iconMap = {
    online: <div className="w-1.5 h-1.5 rounded-full bg-green-400" />,
    offline: <div className="w-1.5 h-1.5 rounded-full bg-gray-400" />,
    away: <div className="w-1.5 h-1.5 rounded-full bg-yellow-400" />,
    busy: <div className="w-1.5 h-1.5 rounded-full bg-red-400" />,
  }

  return (
    <Badge
      variant={variantMap[status]}
      icon={iconMap[status]}
      {...props}
    >
      {status}
    </Badge>
  )
}

// AI provider badge
export const AIProviderBadge = ({ provider, ...props }: { provider: 'claude' | 'gpt4' | 'gemini' | 'grok' | 'ollama' | 'auto' } & Omit<BadgeProps, 'variant' | 'children'>) => {
  const config: Record<string, { variant: BadgeProps['variant']; text: string }> = {
    claude: { variant: 'primary', text: 'Claude' },
    gpt4: { variant: 'success', text: 'GPT-4' },
    gemini: { variant: 'warning', text: 'Gemini' },
    grok: { variant: 'neonCity', text: 'Grok' },
    ollama: { variant: 'info', text: 'Ollama' },
    auto: { variant: 'synthwave', text: 'Auto' },
  }

  const entry = config[provider] || config.auto
  return (
    <Badge variant={entry.variant} {...props}>
      {entry.text}
    </Badge>
  )
}

export { Badge, badgeVariants }