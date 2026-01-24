// APEX.BUILD Gear Loader Component
// Animated steampunk gears for loading states

import React from 'react'
import { cn } from '@/lib/utils'

export interface GearLoaderProps {
  className?: string
  size?: 'sm' | 'md' | 'lg' | 'xl'
  variant?: 'cyan' | 'pink' | 'gold' | 'green'
  text?: string
  showText?: boolean
}

const sizeStyles = {
  sm: { gear: 'w-8 h-8', container: 'w-16 h-16', text: 'text-xs' },
  md: { gear: 'w-12 h-12', container: 'w-24 h-24', text: 'text-sm' },
  lg: { gear: 'w-16 h-16', container: 'w-32 h-32', text: 'text-base' },
  xl: { gear: 'w-24 h-24', container: 'w-48 h-48', text: 'text-lg' },
}

const variantColors = {
  cyan: { primary: '#00FFFF', secondary: '#00CED1', glow: 'rgba(0, 255, 255, 0.3)' },
  pink: { primary: '#FF00FF', secondary: '#C71585', glow: 'rgba(255, 0, 255, 0.3)' },
  gold: { primary: '#CD7F32', secondary: '#B87333', glow: 'rgba(205, 127, 50, 0.3)' },
  green: { primary: '#00FF00', secondary: '#32CD32', glow: 'rgba(0, 255, 0, 0.3)' },
}

const GearSVG: React.FC<{ color: string; glow: string; className?: string; reverse?: boolean }> = ({
  color,
  glow,
  className,
  reverse = false,
}) => (
  <svg
    viewBox="0 0 100 100"
    className={cn(
      className,
      reverse ? 'animate-gear-reverse' : 'animate-gear'
    )}
    style={{
      filter: `drop-shadow(0 0 8px ${glow})`,
    }}
  >
    <path
      fill={color}
      d="M97.6,55.7V44.3l-13.6-2.9c-0.8-3.4-1.9-6.6-3.4-9.6l8.4-11.1l-8.1-8.1L69.8,21c-3-1.5-6.2-2.6-9.6-3.4L57.3,4h-14.6
        l-2.9,13.6c-3.4,0.8-6.6,1.9-9.6,3.4L19.1,12.6L11,20.7l8.4,11.1c-1.5,3-2.6,6.2-3.4,9.6L2.4,44.3v14.4l13.6,2.9
        c0.8,3.4,1.9,6.6,3.4,9.6l-8.4,11.1l8.1,8.1l11.1-8.4c3,1.5,6.2,2.6,9.6,3.4l2.9,13.6h14.4l2.9-13.6c3.4-0.8,6.6-1.9,9.6-3.4
        l11.1,8.4l8.1-8.1l-8.4-11.1c1.5-3,2.6-6.2,3.4-9.6L97.6,55.7z M50,65c-8.3,0-15-6.7-15-15c0-8.3,6.7-15,15-15s15,6.7,15,15
        C65,58.3,58.3,65,50,65z"
    />
  </svg>
)

export const GearLoader: React.FC<GearLoaderProps> = ({
  className,
  size = 'md',
  variant = 'cyan',
  text = 'Loading...',
  showText = true,
}) => {
  const sizes = sizeStyles[size]
  const colors = variantColors[variant]

  return (
    <div className={cn('flex flex-col items-center justify-center gap-4', className)}>
      <div className={cn('relative', sizes.container)}>
        {/* Main gear */}
        <GearSVG
          color={colors.primary}
          glow={colors.glow}
          className={cn('absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2', sizes.gear)}
        />

        {/* Secondary gear (smaller, reverse direction) */}
        <GearSVG
          color={colors.secondary}
          glow={colors.glow}
          className={cn(
            'absolute',
            size === 'sm' && 'w-4 h-4 -top-1 -right-1',
            size === 'md' && 'w-6 h-6 -top-2 -right-2',
            size === 'lg' && 'w-8 h-8 -top-3 -right-3',
            size === 'xl' && 'w-12 h-12 -top-4 -right-4'
          )}
          reverse
        />

        {/* Tertiary gear (smallest) */}
        <GearSVG
          color={colors.secondary}
          glow={colors.glow}
          className={cn(
            'absolute opacity-70',
            size === 'sm' && 'w-3 h-3 -bottom-1 -left-1',
            size === 'md' && 'w-5 h-5 -bottom-2 -left-2',
            size === 'lg' && 'w-7 h-7 -bottom-3 -left-3',
            size === 'xl' && 'w-10 h-10 -bottom-4 -left-4'
          )}
          reverse
        />

        {/* Steam/smoke particles */}
        <div className="absolute -top-4 left-1/2 -translate-x-1/2">
          {[...Array(3)].map((_, i) => (
            <div
              key={i}
              className="absolute w-2 h-2 rounded-full bg-gray-400/30 animate-steam"
              style={{
                left: `${(i - 1) * 8}px`,
                animationDelay: `${i * 0.5}s`,
              }}
            />
          ))}
        </div>
      </div>

      {showText && (
        <span
          className={cn(
            'font-medium tracking-wide animate-pulse',
            sizes.text
          )}
          style={{ color: colors.primary }}
        >
          {text}
        </span>
      )}
    </div>
  )
}

// Simple inline gear animation for buttons etc
export const InlineGear: React.FC<{ className?: string; color?: string }> = ({
  className,
  color = '#00FFFF'
}) => (
  <svg
    viewBox="0 0 100 100"
    className={cn('animate-gear', className)}
    fill={color}
  >
    <path d="M97.6,55.7V44.3l-13.6-2.9c-0.8-3.4-1.9-6.6-3.4-9.6l8.4-11.1l-8.1-8.1L69.8,21c-3-1.5-6.2-2.6-9.6-3.4L57.3,4h-14.6
      l-2.9,13.6c-3.4,0.8-6.6,1.9-9.6,3.4L19.1,12.6L11,20.7l8.4,11.1c-1.5,3-2.6,6.2-3.4,9.6L2.4,44.3v14.4l13.6,2.9
      c0.8,3.4,1.9,6.6,3.4,9.6l-8.4,11.1l8.1,8.1l11.1-8.4c3,1.5,6.2,2.6,9.6,3.4l2.9,13.6h14.4l2.9-13.6c3.4-0.8,6.6-1.9,9.6-3.4
      l11.1,8.4l8.1-8.1l-8.4-11.1c1.5-3,2.6-6.2,3.4-9.6L97.6,55.7z M50,65c-8.3,0-15-6.7-15-15c0-8.3,6.7-15,15-15s15,6.7,15,15
      C65,58.3,58.3,65,50,65z" />
  </svg>
)

export default GearLoader

/* Add these to your tailwind.config.js or CSS:

.animate-gear {
  animation: gear-spin 10s linear infinite;
}

.animate-gear-reverse {
  animation: gear-spin-reverse 8s linear infinite;
}

@keyframes gear-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@keyframes gear-spin-reverse {
  from { transform: rotate(360deg); }
  to { transform: rotate(0deg); }
}

.animate-steam {
  animation: steam-rise 3s ease-out infinite;
}

@keyframes steam-rise {
  0% { transform: translateY(0) scale(1); opacity: 0.5; }
  50% { opacity: 0.3; }
  100% { transform: translateY(-50px) scale(1.5); opacity: 0; }
}
*/
