// APEX.BUILD Cyberpunk Button Component
// Ultra-futuristic button with holographic effects

import React, { forwardRef } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'

const buttonVariants = cva(
  'relative inline-flex items-center justify-center rounded-lg font-medium transition-all duration-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 overflow-hidden group',
  {
    variants: {
      variant: {
        // Primary - Electric cyan with neon glow
        primary: 'bg-gradient-to-r from-cyan-500 to-blue-600 text-white hover:from-cyan-400 hover:to-blue-500 shadow-lg shadow-cyan-500/25 hover:shadow-cyan-400/40 border border-cyan-400/30',

        // Secondary - Hot pink accent
        secondary: 'bg-gradient-to-r from-pink-500 to-purple-600 text-white hover:from-pink-400 hover:to-purple-500 shadow-lg shadow-pink-500/25 hover:shadow-pink-400/40 border border-pink-400/30',

        // Success - Acid green
        success: 'bg-gradient-to-r from-green-400 to-emerald-500 text-black hover:from-green-300 hover:to-emerald-400 shadow-lg shadow-green-400/25 hover:shadow-green-300/40 border border-green-400/30',

        // Danger - Electric red
        danger: 'bg-gradient-to-r from-red-500 to-pink-600 text-white hover:from-red-400 hover:to-pink-500 shadow-lg shadow-red-500/25 hover:shadow-red-400/40 border border-red-400/30',

        // Ghost - Transparent with neon border
        ghost: 'bg-transparent border-2 border-cyan-400/50 text-cyan-400 hover:bg-cyan-400/10 hover:border-cyan-300 shadow-lg shadow-cyan-500/10',

        // Outline - Neon outline style
        outline: 'bg-transparent border border-gray-700 text-gray-300 hover:bg-gray-800 hover:text-white hover:border-gray-600',

        // Link - Text link style
        link: 'text-cyan-400 underline-offset-4 hover:underline hover:text-cyan-300',
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
        intense: 'before:absolute before:inset-0 before:bg-gradient-to-r before:from-transparent before:via-white/20 before:to-transparent before:translate-x-[-100%] hover:before:translate-x-[100%] before:transition-transform before:duration-700 after:absolute after:inset-0 after:bg-gradient-to-r after:from-transparent after:via-cyan-400/20 after:to-transparent after:opacity-0 hover:after:opacity-100 after:transition-opacity after:duration-300',
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