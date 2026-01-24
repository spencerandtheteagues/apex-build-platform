// APEX.BUILD Neon Button Component
// 22nd Century Steampunk design with multi-layer glow effects

import React from 'react'
import { cn } from '@/lib/utils'

export interface NeonButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'cyan' | 'pink' | 'green' | 'purple' | 'gold'
  size?: 'sm' | 'md' | 'lg' | 'xl'
  glow?: 'none' | 'subtle' | 'normal' | 'intense'
  pulse?: boolean
  loading?: boolean
  icon?: React.ReactNode
  iconPosition?: 'left' | 'right'
}

const variantStyles = {
  cyan: {
    base: 'from-cyan-500 to-blue-600',
    glow: 'shadow-cyan-500/50',
    glowIntense: 'shadow-cyan-400/70',
    text: 'text-cyan-100',
    border: 'border-cyan-400/50',
    hoverBorder: 'hover:border-cyan-300',
  },
  pink: {
    base: 'from-pink-500 to-purple-600',
    glow: 'shadow-pink-500/50',
    glowIntense: 'shadow-pink-400/70',
    text: 'text-pink-100',
    border: 'border-pink-400/50',
    hoverBorder: 'hover:border-pink-300',
  },
  green: {
    base: 'from-green-500 to-emerald-600',
    glow: 'shadow-green-500/50',
    glowIntense: 'shadow-green-400/70',
    text: 'text-green-100',
    border: 'border-green-400/50',
    hoverBorder: 'hover:border-green-300',
  },
  purple: {
    base: 'from-purple-500 to-indigo-600',
    glow: 'shadow-purple-500/50',
    glowIntense: 'shadow-purple-400/70',
    text: 'text-purple-100',
    border: 'border-purple-400/50',
    hoverBorder: 'hover:border-purple-300',
  },
  gold: {
    base: 'from-amber-500 to-orange-600',
    glow: 'shadow-amber-500/50',
    glowIntense: 'shadow-amber-400/70',
    text: 'text-amber-100',
    border: 'border-amber-400/50',
    hoverBorder: 'hover:border-amber-300',
  },
}

const sizeStyles = {
  sm: 'px-3 py-1.5 text-sm gap-1.5',
  md: 'px-4 py-2 text-base gap-2',
  lg: 'px-6 py-3 text-lg gap-2.5',
  xl: 'px-8 py-4 text-xl gap-3',
}

const glowSizes = {
  none: '',
  subtle: 'shadow-md',
  normal: 'shadow-lg',
  intense: 'shadow-xl shadow-2xl',
}

export const NeonButton = React.forwardRef<HTMLButtonElement, NeonButtonProps>(
  ({
    className,
    variant = 'cyan',
    size = 'md',
    glow = 'normal',
    pulse = false,
    loading = false,
    icon,
    iconPosition = 'left',
    children,
    disabled,
    ...props
  }, ref) => {
    const styles = variantStyles[variant]
    const isDisabled = disabled || loading

    return (
      <button
        ref={ref}
        disabled={isDisabled}
        className={cn(
          // Base styles
          'relative inline-flex items-center justify-center font-semibold rounded-lg',
          'transition-all duration-300 ease-out',
          'border-2',

          // Gradient background
          `bg-gradient-to-r ${styles.base}`,

          // Border
          styles.border,
          styles.hoverBorder,

          // Text
          styles.text,

          // Size
          sizeStyles[size],

          // Glow effect
          glow !== 'none' && [
            glowSizes[glow],
            styles.glow,
            `hover:${styles.glowIntense}`,
          ],

          // Pulse animation
          pulse && 'animate-pulse-glow',

          // Hover effects
          'hover:scale-105 hover:brightness-110',
          'active:scale-95 active:brightness-90',

          // Focus
          'focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-black',
          `focus:ring-${variant}-400`,

          // Disabled
          isDisabled && 'opacity-50 cursor-not-allowed hover:scale-100 hover:brightness-100',

          className
        )}
        {...props}
      >
        {/* Inner glow layer */}
        <span className={cn(
          'absolute inset-0 rounded-lg opacity-0 transition-opacity duration-300',
          `bg-gradient-to-r ${styles.base}`,
          'blur-sm group-hover:opacity-50'
        )} />

        {/* Content */}
        <span className="relative flex items-center justify-center gap-2">
          {loading ? (
            <span className="w-5 h-5 border-2 border-current border-t-transparent rounded-full animate-spin" />
          ) : (
            <>
              {icon && iconPosition === 'left' && <span className="shrink-0">{icon}</span>}
              {children}
              {icon && iconPosition === 'right' && <span className="shrink-0">{icon}</span>}
            </>
          )}
        </span>

        {/* Electric spark effect on hover */}
        <span className={cn(
          'absolute inset-0 rounded-lg overflow-hidden',
          'opacity-0 hover:opacity-100 transition-opacity duration-150'
        )}>
          <span className="absolute top-0 left-1/4 w-0.5 h-full bg-gradient-to-b from-transparent via-white to-transparent animate-spark" />
        </span>
      </button>
    )
  }
)

NeonButton.displayName = 'NeonButton'

// Add these animations to your global CSS or tailwind config:
/*
@keyframes pulse-glow {
  0%, 100% {
    box-shadow: 0 0 15px currentColor, 0 0 30px currentColor;
  }
  50% {
    box-shadow: 0 0 25px currentColor, 0 0 50px currentColor;
  }
}

@keyframes spark {
  0% {
    transform: translateY(-100%);
    opacity: 0;
  }
  50% {
    opacity: 1;
  }
  100% {
    transform: translateY(100%);
    opacity: 0;
  }
}

.animate-pulse-glow {
  animation: pulse-glow 2s ease-in-out infinite;
}

.animate-spark {
  animation: spark 0.5s ease-out;
}
*/

export default NeonButton
