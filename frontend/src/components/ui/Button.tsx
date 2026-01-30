// APEX.BUILD Ultra-Futuristic Button Component
// Sharp, stylish buttons with glowing effects, ripples, and holographic variants

import React, { forwardRef, useState, useRef, useCallback } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'

// Keyframe animations as CSS-in-JS
const keyframes = `
@keyframes borderGlow {
  0%, 100% { box-shadow: 0 0 5px currentColor, 0 0 10px currentColor, 0 0 15px currentColor; }
  50% { box-shadow: 0 0 10px currentColor, 0 0 20px currentColor, 0 0 30px currentColor; }
}

@keyframes neonPulse {
  0%, 100% { filter: brightness(1) drop-shadow(0 0 5px currentColor); }
  50% { filter: brightness(1.3) drop-shadow(0 0 15px currentColor); }
}

@keyframes holographicShift {
  0% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
  100% { background-position: 0% 50%; }
}

@keyframes borderTrace {
  0% { clip-path: inset(0 100% 100% 0); }
  25% { clip-path: inset(0 0 100% 0); }
  50% { clip-path: inset(0 0 0 0); }
  75% { clip-path: inset(100% 0 0 0); }
  100% { clip-path: inset(100% 100% 0 0); }
}

@keyframes loadingPulse {
  0%, 100% { opacity: 0.5; transform: scale(0.98); }
  50% { opacity: 1; transform: scale(1.02); }
}

@keyframes circuitFlow {
  0% { background-position: 0% 0%; }
  100% { background-position: 200% 200%; }
}

@keyframes rippleExpand {
  0% { transform: scale(0); opacity: 0.6; }
  100% { transform: scale(4); opacity: 0; }
}

@keyframes iconSpin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

@keyframes iconPulse {
  0%, 100% { transform: scale(1); }
  50% { transform: scale(1.2); }
}
`

const buttonVariants = cva(
  'relative inline-flex items-center justify-center rounded-lg font-medium transition-all duration-300 ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 overflow-hidden group',
  {
    variants: {
      variant: {
        // Primary - Demon red with intense glow
        primary: 'bg-gradient-to-r from-red-600 to-red-900 text-white hover:from-red-500 hover:to-red-800 shadow-lg shadow-red-600/30 hover:shadow-red-500/50 hover:shadow-xl border border-red-500/30 hover:border-red-400/60 hover:scale-[1.02] active:scale-[0.98]',

        // Default (alias for primary)
        default: 'bg-gradient-to-r from-red-600 to-red-900 text-white hover:from-red-500 hover:to-red-800 shadow-lg shadow-red-600/30 hover:shadow-red-500/50 hover:shadow-xl border border-red-500/30 hover:border-red-400/60 hover:scale-[1.02] active:scale-[0.98]',

        // Secondary - Dark crimson accent
        secondary: 'bg-gradient-to-r from-red-700 to-rose-900 text-white hover:from-red-600 hover:to-rose-800 shadow-lg shadow-red-700/25 hover:shadow-red-600/40 hover:shadow-xl border border-red-600/30 hover:border-red-500/60 hover:scale-[1.02] active:scale-[0.98]',

        // Success - Vibrant green
        success: 'bg-gradient-to-r from-green-600 to-emerald-700 text-white hover:from-green-500 hover:to-emerald-600 shadow-lg shadow-green-600/25 hover:shadow-green-500/40 hover:shadow-xl border border-green-500/30 hover:border-green-400/60 hover:scale-[1.02] active:scale-[0.98]',

        // Danger - Intense red
        danger: 'bg-gradient-to-r from-red-500 to-red-700 text-white hover:from-red-400 hover:to-red-600 shadow-lg shadow-red-500/30 hover:shadow-red-400/50 hover:shadow-xl border border-red-400/30 hover:border-red-300/60 hover:scale-[1.02] active:scale-[0.98]',

        // Ghost - Transparent with red border
        ghost: 'bg-transparent border-2 border-red-500/50 text-red-400 hover:bg-red-500/10 hover:border-red-400 hover:text-red-300 shadow-lg shadow-red-600/10 hover:shadow-red-500/30 hover:scale-[1.02] active:scale-[0.98]',

        // Outline - Dark outline style
        outline: 'bg-transparent border border-gray-700 text-gray-300 hover:bg-gray-800 hover:text-white hover:border-red-900/50 hover:scale-[1.02] active:scale-[0.98]',

        // Link - Red link style
        link: 'text-red-400 underline-offset-4 hover:underline hover:text-red-300',

        // NEON - Bright glowing outline button
        neon: 'bg-transparent border-2 border-cyan-400 text-cyan-400 hover:text-cyan-300 hover:border-cyan-300 hover:scale-[1.02] active:scale-[0.98]',

        // HOLOGRAPHIC - Iridescent shifting gradient
        holographic: 'text-white border border-white/20 hover:border-white/40 hover:scale-[1.02] active:scale-[0.98]',

        // CYBER - Sharp angular with circuit pattern
        cyber: 'bg-gray-900 text-cyan-400 border border-cyan-500/50 hover:border-cyan-400 hover:text-cyan-300 hover:scale-[1.02] active:scale-[0.98] clip-path-cyber',
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
        subtle: '',
        intense: '',
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
  iconAnimation?: 'none' | 'spin' | 'pulse'
}

interface RippleState {
  x: number
  y: number
  id: number
}

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, glow, loading, icon, iconPosition = 'left', iconAnimation = 'none', children, disabled, onClick, ...props }, ref) => {
    const { theme } = useStore()
    const buttonRef = useRef<HTMLButtonElement>(null)
    const [ripples, setRipples] = useState<RippleState[]>([])
    const [isPressed, setIsPressed] = useState(false)

    const isDisabled = disabled || loading

    // Handle ripple effect on click
    const handleClick = useCallback((e: React.MouseEvent<HTMLButtonElement>) => {
      if (isDisabled) return

      const button = buttonRef.current || (ref as React.RefObject<HTMLButtonElement>)?.current
      if (button) {
        const rect = button.getBoundingClientRect()
        const x = e.clientX - rect.left
        const y = e.clientY - rect.top
        const id = Date.now()

        setRipples(prev => [...prev, { x, y, id }])
        setIsPressed(true)

        // Remove ripple after animation
        setTimeout(() => {
          setRipples(prev => prev.filter(r => r.id !== id))
        }, 600)

        setTimeout(() => setIsPressed(false), 150)
      }

      onClick?.(e)
    }, [isDisabled, onClick, ref])

    // Variant-specific styles
    const getVariantStyles = () => {
      switch (variant) {
        case 'neon':
          return {
            boxShadow: '0 0 10px rgba(34, 211, 238, 0.5), 0 0 20px rgba(34, 211, 238, 0.3), inset 0 0 10px rgba(34, 211, 238, 0.1)',
            animation: 'neonPulse 2s ease-in-out infinite',
          }
        case 'holographic':
          return {
            background: 'linear-gradient(135deg, #ff0080, #7928ca, #00d4ff, #00ff87, #ff0080)',
            backgroundSize: '400% 400%',
            animation: 'holographicShift 3s ease infinite',
          }
        case 'cyber':
          return {
            clipPath: 'polygon(0 0, calc(100% - 10px) 0, 100% 10px, 100% 100%, 10px 100%, 0 calc(100% - 10px))',
            backgroundImage: 'repeating-linear-gradient(90deg, transparent, transparent 10px, rgba(34, 211, 238, 0.03) 10px, rgba(34, 211, 238, 0.03) 20px)',
          }
        default:
          return {}
      }
    }

    // Loading animation styles
    const getLoadingStyles = () => {
      if (!loading) return {}
      return {
        animation: 'loadingPulse 1.5s ease-in-out infinite',
      }
    }

    // Icon animation class
    const getIconAnimationStyle = () => {
      if (iconAnimation === 'spin') return { animation: 'iconSpin 1s linear infinite' }
      if (iconAnimation === 'pulse') return { animation: 'iconPulse 1s ease-in-out infinite' }
      return {}
    }

    const combinedRef = (node: HTMLButtonElement) => {
      (buttonRef as React.MutableRefObject<HTMLButtonElement | null>).current = node
      if (typeof ref === 'function') ref(node)
      else if (ref) ref.current = node
    }

    return (
      <>
        <style>{keyframes}</style>
        <button
          className={cn(buttonVariants({ variant, size, glow }), className)}
          ref={combinedRef}
          disabled={isDisabled}
          onClick={handleClick}
          style={{
            ...getVariantStyles(),
            ...getLoadingStyles(),
            transform: isPressed ? 'scale(0.97)' : undefined,
          }}
          {...props}
        >
          {/* Glowing border animation on hover */}
          <div
            className="absolute inset-0 rounded-lg opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"
            style={{
              boxShadow: variant === 'neon'
                ? '0 0 15px rgba(34, 211, 238, 0.8), 0 0 30px rgba(34, 211, 238, 0.4)'
                : variant === 'holographic'
                ? '0 0 20px rgba(255, 0, 128, 0.4), 0 0 40px rgba(121, 40, 202, 0.3)'
                : '0 0 15px currentColor',
              animation: 'borderGlow 2s ease-in-out infinite',
            }}
          />

          {/* Ripple effects */}
          {ripples.map(ripple => (
            <span
              key={ripple.id}
              className="absolute rounded-full bg-white/30 pointer-events-none"
              style={{
                left: ripple.x,
                top: ripple.y,
                width: 20,
                height: 20,
                marginLeft: -10,
                marginTop: -10,
                animation: 'rippleExpand 0.6s ease-out forwards',
              }}
            />
          ))}

          {/* Loading state with pulsing glow and tracing border */}
          {loading && (
            <>
              <div className="absolute inset-0 flex items-center justify-center">
                <div
                  className="w-5 h-5 border-2 border-current border-t-transparent rounded-full"
                  style={{ animation: 'iconSpin 0.8s linear infinite' }}
                />
              </div>
              {/* Animated tracing border */}
              <div
                className="absolute inset-0 rounded-lg border-2 border-current pointer-events-none"
                style={{
                  animation: 'borderTrace 2s linear infinite',
                }}
              />
            </>
          )}

          {/* Content */}
          <div className={cn('flex items-center gap-2 relative z-10', loading && 'invisible')}>
            {icon && iconPosition === 'left' && (
              <span
                className="inline-flex shrink-0 group-hover:scale-110 transition-transform duration-200"
                style={getIconAnimationStyle()}
              >
                {icon}
              </span>
            )}
            {children}
            {icon && iconPosition === 'right' && (
              <span
                className="inline-flex shrink-0 group-hover:scale-110 transition-transform duration-200"
                style={getIconAnimationStyle()}
              >
                {icon}
              </span>
            )}
          </div>

          {/* Holographic overlay sweep effect */}
          <div
            className="absolute inset-0 bg-gradient-to-r from-transparent via-white/10 to-transparent translate-x-[-100%] group-hover:translate-x-[100%] transition-transform duration-700 ease-out pointer-events-none"
          />

          {/* Secondary shimmer for extra depth */}
          <div
            className="absolute inset-0 bg-gradient-to-r from-transparent via-white/5 to-transparent translate-x-[100%] group-hover:translate-x-[-100%] transition-transform duration-1000 ease-out pointer-events-none delay-100"
          />

          {/* Circuit pattern overlay for cyber variant */}
          {variant === 'cyber' && (
            <div
              className="absolute inset-0 pointer-events-none opacity-20"
              style={{
                backgroundImage: `
                  linear-gradient(90deg, rgba(34, 211, 238, 0.1) 1px, transparent 1px),
                  linear-gradient(rgba(34, 211, 238, 0.1) 1px, transparent 1px)
                `,
                backgroundSize: '8px 8px',
                animation: 'circuitFlow 10s linear infinite',
              }}
            />
          )}

          {/* Corner accents - animated on hover */}
          <div className="absolute top-0 left-0 w-3 h-3 border-t-2 border-l-2 border-current opacity-40 group-hover:opacity-80 group-hover:w-4 group-hover:h-4 transition-all duration-300" />
          <div className="absolute top-0 right-0 w-3 h-3 border-t-2 border-r-2 border-current opacity-40 group-hover:opacity-80 group-hover:w-4 group-hover:h-4 transition-all duration-300" />
          <div className="absolute bottom-0 left-0 w-3 h-3 border-b-2 border-l-2 border-current opacity-40 group-hover:opacity-80 group-hover:w-4 group-hover:h-4 transition-all duration-300" />
          <div className="absolute bottom-0 right-0 w-3 h-3 border-b-2 border-r-2 border-current opacity-40 group-hover:opacity-80 group-hover:w-4 group-hover:h-4 transition-all duration-300" />

          {/* Inner glow for pressed state */}
          <div
            className="absolute inset-0 rounded-lg transition-opacity duration-150 pointer-events-none"
            style={{
              background: 'radial-gradient(circle at center, rgba(255,255,255,0.2) 0%, transparent 70%)',
              opacity: isPressed ? 1 : 0,
            }}
          />
        </button>
      </>
    )
  }
)

Button.displayName = 'Button'

export { Button, buttonVariants }
