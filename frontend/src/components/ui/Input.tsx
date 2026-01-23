// APEX.BUILD Cyberpunk Input Component
// Futuristic input fields with neon glowing effects

import React, { forwardRef, useState } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'
import { Eye, EyeOff } from 'lucide-react'

const inputVariants = cva(
  'flex w-full rounded-lg border bg-transparent px-3 py-2 text-sm transition-all duration-300 placeholder:text-gray-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
  {
    variants: {
      variant: {
        default: 'border-gray-700 bg-gray-900/50 text-white focus:border-cyan-400 focus:ring-cyan-400/50 hover:border-gray-600',
        cyberpunk: 'border-cyan-500/50 bg-gray-900/70 text-cyan-100 focus:border-cyan-400 focus:ring-cyan-400/50 focus:shadow-lg focus:shadow-cyan-500/25',
        matrix: 'border-green-500/50 bg-black/70 text-green-400 focus:border-green-400 focus:ring-green-400/50 focus:shadow-lg focus:shadow-green-500/25',
        synthwave: 'border-pink-500/50 bg-purple-900/50 text-pink-100 focus:border-pink-400 focus:ring-pink-400/50 focus:shadow-lg focus:shadow-pink-500/25',
        neonCity: 'border-blue-500/50 bg-blue-900/30 text-blue-100 focus:border-blue-400 focus:ring-blue-400/50 focus:shadow-lg focus:shadow-blue-500/25',
        error: 'border-red-500 bg-red-900/20 text-red-100 focus:border-red-400 focus:ring-red-400/50',
        success: 'border-green-500 bg-green-900/20 text-green-100 focus:border-green-400 focus:ring-green-400/50',
      },
      size: {
        sm: 'h-8 px-2 text-xs',
        md: 'h-10 px-3 text-sm',
        lg: 'h-12 px-4 text-base',
      },
    },
    defaultVariants: {
      variant: 'cyberpunk',
      size: 'md',
    },
  }
)

export interface InputProps
  extends React.InputHTMLAttributes<HTMLInputElement>,
    VariantProps<typeof inputVariants> {
  label?: string
  error?: string
  success?: string
  leftIcon?: React.ReactNode
  rightIcon?: React.ReactNode
  showPasswordToggle?: boolean
}

const Input = forwardRef<HTMLInputElement, InputProps>(
  (
    {
      className,
      variant,
      size,
      type,
      label,
      error,
      success,
      leftIcon,
      rightIcon,
      showPasswordToggle = false,
      ...props
    },
    ref
  ) => {
    const [showPassword, setShowPassword] = useState(false)
    const [isFocused, setIsFocused] = useState(false)

    const inputType = type === 'password' && showPasswordToggle ? (showPassword ? 'text' : 'password') : type

    // Determine variant based on state
    const currentVariant = error ? 'error' : success ? 'success' : variant

    return (
      <div className="space-y-2">
        {label && (
          <label className="text-sm font-medium text-gray-300 block">
            {label}
          </label>
        )}

        <div className="relative">
          {/* Left icon */}
          {leftIcon && (
            <div className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400">
              {leftIcon}
            </div>
          )}

          {/* Input field */}
          <input
            type={inputType}
            className={cn(
              inputVariants({ variant: currentVariant, size }),
              leftIcon && 'pl-10',
              (rightIcon || showPasswordToggle) && 'pr-10',
              isFocused && 'ring-2 ring-offset-2 ring-offset-gray-900',
              className
            )}
            ref={ref}
            onFocus={(e) => {
              setIsFocused(true)
              props.onFocus?.(e)
            }}
            onBlur={(e) => {
              setIsFocused(false)
              props.onBlur?.(e)
            }}
            {...props}
          />

          {/* Right icon or password toggle */}
          {(rightIcon || showPasswordToggle) && (
            <div className="absolute right-3 top-1/2 -translate-y-1/2">
              {showPasswordToggle && type === 'password' ? (
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="text-gray-400 hover:text-gray-300 transition-colors"
                >
                  {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              ) : (
                rightIcon && <div className="text-gray-400">{rightIcon}</div>
              )}
            </div>
          )}

          {/* Glowing border effect on focus */}
          {isFocused && (
            <div className="absolute inset-0 -z-10 rounded-lg bg-gradient-to-r from-transparent via-current to-transparent opacity-20 blur-sm" />
          )}

          {/* Corner accents for futuristic look */}
          <div className="absolute top-0 left-0 w-2 h-2 border-t border-l border-current opacity-30" />
          <div className="absolute top-0 right-0 w-2 h-2 border-t border-r border-current opacity-30" />
          <div className="absolute bottom-0 left-0 w-2 h-2 border-b border-l border-current opacity-30" />
          <div className="absolute bottom-0 right-0 w-2 h-2 border-b border-r border-current opacity-30" />
        </div>

        {/* Error/Success message */}
        {error && (
          <p className="text-xs text-red-400 flex items-center gap-1">
            <span className="w-1 h-1 bg-red-400 rounded-full" />
            {error}
          </p>
        )}
        {success && (
          <p className="text-xs text-green-400 flex items-center gap-1">
            <span className="w-1 h-1 bg-green-400 rounded-full" />
            {success}
          </p>
        )}
      </div>
    )
  }
)

Input.displayName = 'Input'

export { Input, inputVariants }