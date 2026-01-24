// APEX.BUILD Glass Panel Component
// Frosted glass morphism with steampunk accents

import React from 'react'
import { cn } from '@/lib/utils'

export interface GlassPanelProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'dark' | 'light'
  blur?: 'none' | 'sm' | 'md' | 'lg' | 'xl'
  border?: 'none' | 'subtle' | 'glow' | 'brass'
  glow?: 'none' | 'cyan' | 'pink' | 'purple' | 'green'
  rounded?: 'none' | 'sm' | 'md' | 'lg' | 'xl' | 'full'
  padding?: 'none' | 'sm' | 'md' | 'lg' | 'xl'
  hoverable?: boolean
  animated?: boolean
}

const variantStyles = {
  default: 'bg-gray-900/40',
  dark: 'bg-black/60',
  light: 'bg-gray-800/30',
}

const blurStyles = {
  none: '',
  sm: 'backdrop-blur-sm',
  md: 'backdrop-blur-md',
  lg: 'backdrop-blur-lg',
  xl: 'backdrop-blur-xl',
}

const borderStyles = {
  none: '',
  subtle: 'border border-gray-700/50',
  glow: 'border border-cyan-500/30 shadow-lg shadow-cyan-500/10',
  brass: 'border-2 border-amber-600/50 shadow-lg shadow-amber-500/20',
}

const glowStyles = {
  none: '',
  cyan: 'shadow-lg shadow-cyan-500/20 hover:shadow-cyan-500/30',
  pink: 'shadow-lg shadow-pink-500/20 hover:shadow-pink-500/30',
  purple: 'shadow-lg shadow-purple-500/20 hover:shadow-purple-500/30',
  green: 'shadow-lg shadow-green-500/20 hover:shadow-green-500/30',
}

const roundedStyles = {
  none: 'rounded-none',
  sm: 'rounded-sm',
  md: 'rounded-md',
  lg: 'rounded-lg',
  xl: 'rounded-xl',
  full: 'rounded-full',
}

const paddingStyles = {
  none: '',
  sm: 'p-2',
  md: 'p-4',
  lg: 'p-6',
  xl: 'p-8',
}

export const GlassPanel = React.forwardRef<HTMLDivElement, GlassPanelProps>(
  ({
    className,
    variant = 'default',
    blur = 'md',
    border = 'subtle',
    glow = 'none',
    rounded = 'lg',
    padding = 'md',
    hoverable = false,
    animated = false,
    children,
    ...props
  }, ref) => {
    return (
      <div
        ref={ref}
        className={cn(
          // Base
          'relative overflow-hidden',

          // Variant (background)
          variantStyles[variant],

          // Blur
          blurStyles[blur],

          // Border
          borderStyles[border],

          // Glow
          glowStyles[glow],

          // Rounded
          roundedStyles[rounded],

          // Padding
          paddingStyles[padding],

          // Hoverable
          hoverable && [
            'transition-all duration-300 ease-out',
            'hover:scale-[1.02] hover:shadow-xl',
            'cursor-pointer',
          ],

          // Animated shimmer
          animated && 'animate-shimmer',

          className
        )}
        {...props}
      >
        {/* Inner reflection/highlight */}
        <div className="absolute inset-0 bg-gradient-to-br from-white/5 to-transparent pointer-events-none" />

        {/* Brass corner accents for steampunk effect */}
        {border === 'brass' && (
          <>
            <div className="absolute top-0 left-0 w-4 h-4 border-t-2 border-l-2 border-amber-500/70 rounded-tl-lg" />
            <div className="absolute top-0 right-0 w-4 h-4 border-t-2 border-r-2 border-amber-500/70 rounded-tr-lg" />
            <div className="absolute bottom-0 left-0 w-4 h-4 border-b-2 border-l-2 border-amber-500/70 rounded-bl-lg" />
            <div className="absolute bottom-0 right-0 w-4 h-4 border-b-2 border-r-2 border-amber-500/70 rounded-br-lg" />
          </>
        )}

        {/* Content */}
        <div className="relative z-10">
          {children}
        </div>
      </div>
    )
  }
)

GlassPanel.displayName = 'GlassPanel'

export default GlassPanel
