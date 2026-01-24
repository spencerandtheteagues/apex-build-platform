// APEX.BUILD UI - Progress Component
// Cyberpunk-styled progress bar with neon glow effects

import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const progressVariants = cva(
  'relative h-4 w-full overflow-hidden rounded-full bg-black/50 border',
  {
    variants: {
      variant: {
        default: 'border-gray-700',
        cyberpunk: 'border-cyan-500/50 shadow-[0_0_10px_rgba(0,255,255,0.2)]',
        matrix: 'border-green-500/50 shadow-[0_0_10px_rgba(0,255,0,0.2)]',
        synthwave: 'border-pink-500/50 shadow-[0_0_10px_rgba(255,0,255,0.2)]',
        neonCity: 'border-purple-500/50 shadow-[0_0_10px_rgba(128,0,255,0.2)]',
      },
      size: {
        sm: 'h-2',
        md: 'h-4',
        lg: 'h-6',
        xl: 'h-8',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'md',
    },
  }
)

const progressIndicatorVariants = cva(
  'h-full transition-all duration-500 ease-out',
  {
    variants: {
      variant: {
        default: 'bg-gradient-to-r from-gray-500 to-gray-400',
        cyberpunk:
          'bg-gradient-to-r from-cyan-600 via-cyan-400 to-cyan-300 shadow-[0_0_15px_rgba(0,255,255,0.5)]',
        matrix:
          'bg-gradient-to-r from-green-600 via-green-400 to-green-300 shadow-[0_0_15px_rgba(0,255,0,0.5)]',
        synthwave:
          'bg-gradient-to-r from-pink-600 via-pink-400 to-pink-300 shadow-[0_0_15px_rgba(255,0,255,0.5)]',
        neonCity:
          'bg-gradient-to-r from-purple-600 via-purple-400 to-purple-300 shadow-[0_0_15px_rgba(128,0,255,0.5)]',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)

export interface ProgressProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof progressVariants> {
  value?: number
  max?: number
  showValue?: boolean
  animated?: boolean
}

const Progress = React.forwardRef<HTMLDivElement, ProgressProps>(
  (
    {
      className,
      value = 0,
      max = 100,
      variant,
      size,
      showValue = false,
      animated = true,
      ...props
    },
    ref
  ) => {
    const percentage = Math.min(Math.max((value / max) * 100, 0), 100)

    return (
      <div
        ref={ref}
        className={cn(progressVariants({ variant, size, className }))}
        role="progressbar"
        aria-valuenow={value}
        aria-valuemin={0}
        aria-valuemax={max}
        {...props}
      >
        <div
          className={cn(
            progressIndicatorVariants({ variant }),
            animated && 'animate-pulse'
          )}
          style={{ width: `${percentage}%` }}
        />
        {showValue && (
          <span className="absolute inset-0 flex items-center justify-center text-xs font-medium text-white drop-shadow-[0_0_2px_rgba(0,0,0,0.8)]">
            {Math.round(percentage)}%
          </span>
        )}
      </div>
    )
  }
)
Progress.displayName = 'Progress'

// Indeterminate progress for loading states
const IndeterminateProgress = React.forwardRef<
  HTMLDivElement,
  Omit<ProgressProps, 'value' | 'max' | 'showValue'>
>(({ className, variant, size, ...props }, ref) => {
  return (
    <div
      ref={ref}
      className={cn(progressVariants({ variant, size, className }))}
      role="progressbar"
      {...props}
    >
      <div
        className={cn(
          progressIndicatorVariants({ variant }),
          'w-1/3 animate-[progress-indeterminate_1.5s_ease-in-out_infinite]'
        )}
      />
    </div>
  )
})
IndeterminateProgress.displayName = 'IndeterminateProgress'

export { Progress, IndeterminateProgress, progressVariants }
