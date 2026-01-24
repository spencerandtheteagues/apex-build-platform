// APEX.BUILD Cyberpunk Button Component
// Ultra-futuristic button with holographic effects

import React, { forwardRef } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'

const buttonVariants = cva(
  'relative inline-flex items-center justify-center rounded-lg font-medium transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 overflow-hidden group',
  {
    variants: {
      variant: {
        // Primary - Demon red with scary glow
        primary: 'bg-gradient-to-r from-red-600 to-red-900 text-white hover:from-red-500 hover:to-red-800 shadow-lg shadow-red-600/30 hover:shadow-red-500/50 border border-red-500/30',

        // Secondary - Dark crimson accent
        secondary: 'bg-gradient-to-r from-red-700 to-rose-900 text-white hover:from-red-600 hover:to-rose-800 shadow-lg shadow-red-700/25 hover:shadow-red-600/40 border border-red-600/30',

        // Success - Blood green (dark tinted)
        success: 'bg-gradient-to-r from-green-600 to-emerald-700 text-white hover:from-green-500 hover:to-emerald-600 shadow-lg shadow-green-600/25 hover:shadow-green-500/40 border border-green-500/30',

        // Danger - Intense red
        danger: 'bg-gradient-to-r from-red-500 to-red-700 text-white hover:from-red-400 hover:to-red-600 shadow-lg shadow-red-500/30 hover:shadow-red-400/50 border border-red-400/30',

        // Ghost - Transparent with red border
        ghost: 'bg-transparent border-2 border-red-500/50 text-red-400 hover:bg-red-500/10 hover:border-red-400 shadow-lg shadow-red-600/10',

        // Outline - Dark outline style
        outline: 'bg-transparent border border-gray-700 text-gray-300 hover:bg-gray-800 hover:text-white hover:border-red-900/50',

        // Link - Red link style
        link: 'text-red-400 underline-offset-4 hover:underline hover:text-red-300',
      },
      size: {
        xs: 'h-7 px-2 text-xs',
        sm: 'h-8 px-3 text-sm',
        md: 'h-10 px-4 text-sm',
        lg: 'h-11 px-8 text-base',
        xl: 'h-12 px-10 text-lg',
        icon: 'h-10 w-10',
      },
      glow: {
        none: '',
        subtle: 'before:absolute before:inset-0 before:bg-gradient-to-r before:from-transparent before:via-white/10 before:to-transparent before:translate-x-[-100%] hover:before:translate-x-[100%] before:transition-transform before:duration-1000',
        intense: 'before:absolute before:inset-0 before:bg-gradient-to-r before:from-transparent before:via-white/20 before:to-transparent before:translate-x-[-100%] hover:before:translate-x-[100%] before:transition-transform before:duration-700 after:absolute after:inset-0 after:bg-gradient-to-r after:from-transparent after:via-red-500/20 after:to-transparent after:opacity-0 hover:after:opacity-100 after:transition-opacity after:duration-300',
      },
    },
    defaultVariants: {
      variant: 'primary',
      size: 'md',
      glow: 'subtle',
    },
  }
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  loading?: boolean
  icon?: React.ReactNode
  iconPosition?: 'left' | 'right'
}

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, glow, loading, icon, iconPosition = 'left', children, disabled, ...props }, ref) => {
    const { theme } = useStore()

    const isDisabled = disabled || loading

    return (
      <button
        className={cn(buttonVariants({ variant, size, glow }), className)}
        ref={ref}
        disabled={isDisabled}
        {...props}
      >
        {/* Loading spinner */}
        {loading && (
          <div className="absolute inset-0 flex items-center justify-center">
            <div className="w-4 h-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {/* Content */}
        <div className={cn('flex items-center gap-2', loading && 'invisible')}>
          {icon && iconPosition === 'left' && (
            <span className="inline-flex shrink-0">{icon}</span>
          )}
          {children}
          {icon && iconPosition === 'right' && (
            <span className="inline-flex shrink-0">{icon}</span>
          )}
        </div>

        {/* Holographic overlay effect */}
        <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/5 to-transparent translate-x-[-100%] group-hover:translate-x-[100%] transition-transform duration-1000 ease-out" />

        {/* Corner accents for futuristic look */}
        <div className="absolute top-0 left-0 w-2 h-2 border-t-2 border-l-2 border-current opacity-30" />
        <div className="absolute top-0 right-0 w-2 h-2 border-t-2 border-r-2 border-current opacity-30" />
        <div className="absolute bottom-0 left-0 w-2 h-2 border-b-2 border-l-2 border-current opacity-30" />
        <div className="absolute bottom-0 right-0 w-2 h-2 border-b-2 border-r-2 border-current opacity-30" />
      </button>
    )
  }
)

Button.displayName = 'Button'

export { Button, buttonVariants }