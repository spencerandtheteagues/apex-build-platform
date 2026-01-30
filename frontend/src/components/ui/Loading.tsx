// APEX.BUILD Cyberpunk Loading Components
// Futuristic loading spinners and animations

import React, { forwardRef } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const loadingVariants = cva(
  'animate-spin',
  {
    variants: {
      variant: {
        // Spinning circle loader
        spinner: 'rounded-full border-2 border-transparent border-t-current',

        // Primary (alias for spinner)
        primary: 'rounded-full border-2 border-transparent border-t-current',

        // Pulsing dots
        dots: 'flex space-x-1',

        // Glowing orb
        orb: 'rounded-full bg-gradient-to-r animate-pulse shadow-lg',

        // Cyberpunk matrix rain effect
        matrix: 'flex flex-col space-y-1',

        // Neon pulse ring
        ring: 'rounded-full border-2 border-current animate-pulse',

        // Holographic shimmer
        shimmer: 'bg-gradient-to-r from-transparent via-current to-transparent animate-pulse',
      },
      size: {
        xs: 'w-3 h-3',
        sm: 'w-4 h-4',
        md: 'w-6 h-6',
        lg: 'w-8 h-8',
        xl: 'w-12 h-12',
        '2xl': 'w-16 h-16',
      },
      color: {
        cyberpunk: 'text-cyan-400',
        matrix: 'text-green-400',
        synthwave: 'text-pink-400',
        neonCity: 'text-blue-400',
        white: 'text-white',
        current: 'text-current',
      },
    },
    defaultVariants: {
      variant: 'spinner',
      size: 'md',
      color: 'cyberpunk',
    },
  }
)

export interface LoadingProps
  extends Omit<React.HTMLAttributes<HTMLDivElement>, 'color'>,
    VariantProps<typeof loadingVariants> {
  text?: string
  label?: string // Alias for text
}

const Loading = forwardRef<HTMLDivElement, LoadingProps>(
  ({ className, variant, size, color, text, label, ...props }, ref) => {
    const displayText = text || label
    const renderSpinner = () => {
      switch (variant) {
        case 'dots':
          return (
            <div className="flex space-x-1">
              {[0, 1, 2].map((i) => (
                <div
                  key={i}
                  className={cn(
                    'rounded-full bg-current animate-pulse',
                    size === 'xs' && 'w-1 h-1',
                    size === 'sm' && 'w-1.5 h-1.5',
                    size === 'md' && 'w-2 h-2',
                    size === 'lg' && 'w-3 h-3',
                    size === 'xl' && 'w-4 h-4',
                    size === '2xl' && 'w-6 h-6'
                  )}
                  style={{ animationDelay: `${i * 0.2}s` }}
                />
              ))}
            </div>
          )

        case 'orb':
          return (
            <div
              className={cn(
                'rounded-full bg-gradient-to-r from-current to-transparent animate-pulse shadow-lg',
                loadingVariants({ size })
              )}
              style={{ boxShadow: '0 0 20px currentColor' }}
            />
          )

        case 'matrix':
          return (
            <div className="flex flex-col space-y-1">
              {[0, 1, 2, 3].map((i) => (
                <div
                  key={i}
                  className={cn(
                    'bg-current animate-pulse',
                    size === 'xs' && 'w-8 h-0.5',
                    size === 'sm' && 'w-12 h-0.5',
                    size === 'md' && 'w-16 h-1',
                    size === 'lg' && 'w-20 h-1',
                    size === 'xl' && 'w-24 h-1.5',
                    size === '2xl' && 'w-32 h-2'
                  )}
                  style={{ animationDelay: `${i * 0.3}s`, opacity: 1 - i * 0.2 }}
                />
              ))}
            </div>
          )

        case 'ring':
          return (
            <div
              className={cn(
                'rounded-full border-2 border-current animate-ping',
                loadingVariants({ size })
              )}
            />
          )

        case 'shimmer':
          return (
            <div
              className={cn(
                'bg-gradient-to-r from-transparent via-current to-transparent opacity-60',
                size === 'xs' && 'w-12 h-1',
                size === 'sm' && 'w-16 h-1',
                size === 'md' && 'w-20 h-2',
                size === 'lg' && 'w-24 h-2',
                size === 'xl' && 'w-32 h-3',
                size === '2xl' && 'w-40 h-4'
              )}
            />
          )

        default: // spinner
          return (
            <div
              className={cn(loadingVariants({ variant, size }), 'border-2 border-transparent border-t-current')}
            />
          )
      }
    }

    return (
      <div
        className={cn('flex items-center justify-center', color && `text-${color}`, className)}
        ref={ref}
        {...props}
      >
        <div className={cn(loadingVariants({ color }))}>
          {renderSpinner()}
        </div>
        {displayText && (
          <span className="ml-3 text-sm font-medium animate-pulse">
            {displayText}
          </span>
        )}
      </div>
    )
  }
)

// Skeleton loading component for content placeholders
export interface SkeletonProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: 'text' | 'card' | 'avatar' | 'button'
  lines?: number
  avatar?: boolean
}

const Skeleton = forwardRef<HTMLDivElement, SkeletonProps>(
  ({ className, variant = 'text', lines = 1, avatar = false, ...props }, ref) => {
    const renderSkeleton = () => {
      switch (variant) {
        case 'card':
          return (
            <div className="space-y-4">
              {avatar && <div className="w-12 h-12 bg-gray-700 rounded-full animate-pulse" />}
              <div className="space-y-2">
                <div className="h-4 bg-gray-700 rounded animate-pulse" />
                <div className="h-4 bg-gray-700 rounded w-3/4 animate-pulse" />
                <div className="h-4 bg-gray-700 rounded w-1/2 animate-pulse" />
              </div>
            </div>
          )

        case 'avatar':
          return <div className="w-10 h-10 bg-gray-700 rounded-full animate-pulse" />

        case 'button':
          return <div className="h-10 bg-gray-700 rounded-lg w-24 animate-pulse" />

        default: // text
          return (
            <div className="space-y-2">
              {Array.from({ length: lines }).map((_, i) => (
                <div
                  key={i}
                  className={cn(
                    'h-4 bg-gray-700 rounded animate-pulse',
                    i === lines - 1 && lines > 1 && 'w-3/4'
                  )}
                />
              ))}
            </div>
          )
      }
    }

    return (
      <div
        className={cn('animate-pulse', className)}
        ref={ref}
        {...props}
      >
        {renderSkeleton()}
      </div>
    )
  }
)

// Full page loading overlay
export interface LoadingOverlayProps {
  isVisible: boolean
  text?: string
  variant?: LoadingProps['variant']
  backdrop?: 'blur' | 'dark' | 'glass'
}

const LoadingOverlay: React.FC<LoadingOverlayProps> = ({
  isVisible,
  text = 'Loading...',
  variant = 'orb',
  backdrop = 'blur',
}) => {
  if (!isVisible) return null

  const backdropClasses = {
    blur: 'backdrop-blur-sm bg-black/30',
    dark: 'bg-black/80',
    glass: 'backdrop-blur-md bg-gray-900/50',
  }

  return (
    <div
      className={cn(
        'fixed inset-0 z-50 flex items-center justify-center transition-all duration-300',
        backdropClasses[backdrop]
      )}
    >
      <div className="flex flex-col items-center space-y-4 p-8 rounded-lg bg-gray-900/90 backdrop-blur-sm border border-gray-700">
        <Loading variant={variant} size="xl" color="cyberpunk" />
        <p className="text-white font-medium">{text}</p>
      </div>
    </div>
  )
}

Loading.displayName = 'Loading'
Skeleton.displayName = 'Skeleton'
LoadingOverlay.displayName = 'LoadingOverlay'

export { Loading, Skeleton, LoadingOverlay, loadingVariants }