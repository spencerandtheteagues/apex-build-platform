// APEX.BUILD Premium Futuristic Card Component
// Glass morphism, holographic effects, plasma borders, and HUD-style corners

import React, { forwardRef, useState } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

// Keyframe animations
const keyframes = `
@keyframes hologramFlicker {
  0%, 100% { opacity: 1; }
  92% { opacity: 1; }
  93% { opacity: 0.8; }
  94% { opacity: 1; }
  95% { opacity: 0.9; }
  96% { opacity: 1; }
}

@keyframes hologramScanline {
  0% { transform: translateY(-100%); }
  100% { transform: translateY(100%); }
}

@keyframes plasmaBorder {
  0% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
  100% { background-position: 0% 50%; }
}

@keyframes terminalBlink {
  0%, 49% { opacity: 1; }
  50%, 100% { opacity: 0; }
}

@keyframes glowPulse {
  0%, 100% { box-shadow: 0 0 20px rgba(34, 211, 238, 0.3), inset 0 0 20px rgba(34, 211, 238, 0.05); }
  50% { box-shadow: 0 0 40px rgba(34, 211, 238, 0.5), inset 0 0 30px rgba(34, 211, 238, 0.1); }
}

@keyframes cornerPulse {
  0%, 100% { opacity: 0.5; }
  50% { opacity: 1; }
}

@keyframes gradientRotate {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

@keyframes innerGlow {
  0%, 100% { opacity: 0.3; }
  50% { opacity: 0.6; }
}

@keyframes dataStream {
  0% { background-position: 0% 0%; }
  100% { background-position: 0% 100%; }
}
`

const cardVariants = cva(
  'relative rounded-xl overflow-hidden transition-all duration-300 ease-out group',
  {
    variants: {
      variant: {
        // Glass morphism with premium glow
        default: 'bg-gray-900/70 backdrop-blur-xl border border-gray-700/50 hover:border-gray-500/70 shadow-2xl hover:shadow-3xl',

        // Cyberpunk style with cyan accents
        cyberpunk: 'bg-gradient-to-br from-gray-900/90 to-cyan-950/30 backdrop-blur-xl border border-cyan-500/30 hover:border-cyan-400/60 shadow-xl shadow-cyan-500/10 hover:shadow-cyan-400/30',

        // Matrix green theme
        matrix: 'bg-gradient-to-br from-black/90 to-green-950/30 backdrop-blur-xl border border-green-500/30 hover:border-green-400/60 shadow-xl shadow-green-500/10 hover:shadow-green-400/30',

        // Synthwave pink/purple
        synthwave: 'bg-gradient-to-br from-purple-900/80 to-pink-950/40 backdrop-blur-xl border border-pink-500/30 hover:border-pink-400/60 shadow-xl shadow-pink-500/10 hover:shadow-pink-400/30',

        // Neon city blue
        neonCity: 'bg-gradient-to-br from-blue-950/80 to-cyan-950/40 backdrop-blur-xl border border-blue-500/30 hover:border-blue-400/60 shadow-xl shadow-blue-500/10 hover:shadow-blue-400/30',

        // Solid dark card
        solid: 'bg-gray-800/95 border border-gray-700 hover:border-gray-500 shadow-2xl hover:shadow-3xl',

        // Interactive card (clickable)
        interactive: 'bg-gray-900/80 backdrop-blur-xl border border-gray-700/50 hover:border-cyan-400/70 hover:bg-gray-800/90 cursor-pointer shadow-xl hover:shadow-cyan-500/30',

        // Error card
        error: 'bg-red-950/30 backdrop-blur-xl border border-red-500/30 hover:border-red-400/50 shadow-xl shadow-red-500/10',

        // Neutral card
        neutral: 'bg-gray-900/50 backdrop-blur-xl border border-gray-800 hover:border-gray-700',

        // HOLOGRAM - Flickering holographic effect
        hologram: 'bg-cyan-950/20 backdrop-blur-xl border border-cyan-400/40 shadow-xl shadow-cyan-500/20',

        // TERMINAL - Green-on-black terminal style
        terminal: 'bg-black/95 border border-green-500/50 shadow-xl shadow-green-500/10',

        // PLASMA - Animated plasma border
        plasma: 'bg-gray-900/80 backdrop-blur-xl',
      },
      padding: {
        none: 'p-0',
        sm: 'p-4',
        md: 'p-6',
        lg: 'p-8',
        xl: 'p-10',
      },
      glow: {
        none: '',
        subtle: '',
        intense: '',
      },
      hover: {
        none: '',
        lift: 'hover:-translate-y-1 hover:scale-[1.01]',
        glow: '',
        expand: 'hover:scale-[1.02]',
      },
    },
    defaultVariants: {
      variant: 'cyberpunk',
      padding: 'md',
      glow: 'subtle',
      hover: 'lift',
    },
  }
)

export interface CardProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof cardVariants> {
  as?: React.ElementType
  corners?: 'none' | 'brackets' | 'dots' | 'lines'
}

const Card = forwardRef<HTMLDivElement, CardProps>(
  ({ className, variant, padding, glow, hover, corners = 'brackets', as: Component = 'div', children, ...props }, ref) => {
    const [isHovered, setIsHovered] = useState(false)

    // Get variant-specific styles
    const getVariantStyles = (): React.CSSProperties => {
      switch (variant) {
        case 'hologram':
          return {
            animation: 'hologramFlicker 4s infinite',
          }
        case 'plasma':
          return {}
        default:
          return {}
      }
    }

    // Get glow styles
    const getGlowStyles = (): React.CSSProperties => {
      if (glow === 'none') return {}
      if (glow === 'intense') {
        return {
          boxShadow: isHovered
            ? '0 0 40px rgba(34, 211, 238, 0.4), inset 0 0 30px rgba(34, 211, 238, 0.05)'
            : '0 0 20px rgba(34, 211, 238, 0.2), inset 0 0 15px rgba(34, 211, 238, 0.02)',
        }
      }
      return {}
    }

    // Render corner decorations based on type
    const renderCorners = () => {
      if (corners === 'none') return null

      if (corners === 'brackets') {
        return (
          <>
            {/* Top-left bracket */}
            <div className="absolute top-2 left-2 w-4 h-4 pointer-events-none">
              <div className="absolute top-0 left-0 w-full h-0.5 bg-current opacity-50 group-hover:opacity-100 group-hover:w-6 transition-all duration-300" style={{ animation: 'cornerPulse 2s ease-in-out infinite' }} />
              <div className="absolute top-0 left-0 h-full w-0.5 bg-current opacity-50 group-hover:opacity-100 group-hover:h-6 transition-all duration-300" style={{ animation: 'cornerPulse 2s ease-in-out infinite' }} />
            </div>
            {/* Top-right bracket */}
            <div className="absolute top-2 right-2 w-4 h-4 pointer-events-none">
              <div className="absolute top-0 right-0 w-full h-0.5 bg-current opacity-50 group-hover:opacity-100 group-hover:w-6 transition-all duration-300" style={{ animation: 'cornerPulse 2s ease-in-out infinite 0.5s' }} />
              <div className="absolute top-0 right-0 h-full w-0.5 bg-current opacity-50 group-hover:opacity-100 group-hover:h-6 transition-all duration-300" style={{ animation: 'cornerPulse 2s ease-in-out infinite 0.5s' }} />
            </div>
            {/* Bottom-left bracket */}
            <div className="absolute bottom-2 left-2 w-4 h-4 pointer-events-none">
              <div className="absolute bottom-0 left-0 w-full h-0.5 bg-current opacity-50 group-hover:opacity-100 group-hover:w-6 transition-all duration-300" style={{ animation: 'cornerPulse 2s ease-in-out infinite 1s' }} />
              <div className="absolute bottom-0 left-0 h-full w-0.5 bg-current opacity-50 group-hover:opacity-100 group-hover:h-6 transition-all duration-300" style={{ animation: 'cornerPulse 2s ease-in-out infinite 1s' }} />
            </div>
            {/* Bottom-right bracket */}
            <div className="absolute bottom-2 right-2 w-4 h-4 pointer-events-none">
              <div className="absolute bottom-0 right-0 w-full h-0.5 bg-current opacity-50 group-hover:opacity-100 group-hover:w-6 transition-all duration-300" style={{ animation: 'cornerPulse 2s ease-in-out infinite 1.5s' }} />
              <div className="absolute bottom-0 right-0 h-full w-0.5 bg-current opacity-50 group-hover:opacity-100 group-hover:h-6 transition-all duration-300" style={{ animation: 'cornerPulse 2s ease-in-out infinite 1.5s' }} />
            </div>
          </>
        )
      }

      if (corners === 'dots') {
        return (
          <>
            <div className="absolute top-2 left-2 w-2 h-2 rounded-full bg-current opacity-50 group-hover:opacity-100 group-hover:scale-150 transition-all duration-300" />
            <div className="absolute top-2 right-2 w-2 h-2 rounded-full bg-current opacity-50 group-hover:opacity-100 group-hover:scale-150 transition-all duration-300" />
            <div className="absolute bottom-2 left-2 w-2 h-2 rounded-full bg-current opacity-50 group-hover:opacity-100 group-hover:scale-150 transition-all duration-300" />
            <div className="absolute bottom-2 right-2 w-2 h-2 rounded-full bg-current opacity-50 group-hover:opacity-100 group-hover:scale-150 transition-all duration-300" />
          </>
        )
      }

      if (corners === 'lines') {
        return (
          <>
            <div className="absolute top-0 left-4 right-4 h-0.5 bg-gradient-to-r from-transparent via-current to-transparent opacity-30 group-hover:opacity-60 transition-opacity duration-300" />
            <div className="absolute bottom-0 left-4 right-4 h-0.5 bg-gradient-to-r from-transparent via-current to-transparent opacity-30 group-hover:opacity-60 transition-opacity duration-300" />
            <div className="absolute left-0 top-4 bottom-4 w-0.5 bg-gradient-to-b from-transparent via-current to-transparent opacity-30 group-hover:opacity-60 transition-opacity duration-300" />
            <div className="absolute right-0 top-4 bottom-4 w-0.5 bg-gradient-to-b from-transparent via-current to-transparent opacity-30 group-hover:opacity-60 transition-opacity duration-300" />
          </>
        )
      }

      return null
    }

    return (
      <>
        <style>{keyframes}</style>
        <Component
          className={cn(cardVariants({ variant, padding, glow, hover }), className)}
          ref={ref}
          onMouseEnter={() => setIsHovered(true)}
          onMouseLeave={() => setIsHovered(false)}
          style={{
            ...getVariantStyles(),
            ...getGlowStyles(),
          }}
          {...props}
        >
          {/* Plasma border effect */}
          {variant === 'plasma' && (
            <div
              className="absolute inset-0 rounded-xl pointer-events-none"
              style={{
                padding: '2px',
                background: 'linear-gradient(90deg, #ff0080, #7928ca, #00d4ff, #00ff87, #ff0080)',
                backgroundSize: '300% 300%',
                animation: 'plasmaBorder 4s ease infinite',
                WebkitMask: 'linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0)',
                WebkitMaskComposite: 'xor',
                maskComposite: 'exclude',
              }}
            />
          )}

          {/* Hologram scanline effect */}
          {variant === 'hologram' && (
            <div
              className="absolute inset-0 pointer-events-none overflow-hidden"
            >
              <div
                className="absolute inset-x-0 h-1/3 bg-gradient-to-b from-cyan-400/10 via-cyan-400/5 to-transparent"
                style={{ animation: 'hologramScanline 3s linear infinite' }}
              />
              {/* Hologram noise overlay */}
              <div
                className="absolute inset-0 opacity-[0.03]"
                style={{
                  backgroundImage: 'url("data:image/svg+xml,%3Csvg viewBox=\'0 0 256 256\' xmlns=\'http://www.w3.org/2000/svg\'%3E%3Cfilter id=\'noiseFilter\'%3E%3CfeTurbulence type=\'fractalNoise\' baseFrequency=\'0.9\' numOctaves=\'4\' stitchTiles=\'stitch\'/%3E%3C/filter%3E%3Crect width=\'100%25\' height=\'100%25\' filter=\'url(%23noiseFilter)\'/%3E%3C/svg%3E")',
                }}
              />
            </div>
          )}

          {/* Terminal scanlines and cursor */}
          {variant === 'terminal' && (
            <>
              <div
                className="absolute inset-0 pointer-events-none opacity-[0.03]"
                style={{
                  backgroundImage: 'repeating-linear-gradient(0deg, transparent, transparent 2px, rgba(0, 255, 0, 0.1) 2px, rgba(0, 255, 0, 0.1) 4px)',
                }}
              />
              <div className="absolute top-3 left-3 flex items-center gap-1.5 pointer-events-none">
                <div className="w-2 h-2 rounded-full bg-red-500" />
                <div className="w-2 h-2 rounded-full bg-yellow-500" />
                <div className="w-2 h-2 rounded-full bg-green-500" />
              </div>
            </>
          )}

          {/* Frosted glass inner glow */}
          <div
            className="absolute inset-0 rounded-xl pointer-events-none transition-opacity duration-300"
            style={{
              background: 'radial-gradient(ellipse at 50% 0%, rgba(255,255,255,0.05) 0%, transparent 70%)',
              opacity: isHovered ? 0.8 : 0.4,
            }}
          />

          {/* Gradient border animation */}
          {glow !== 'none' && variant !== 'plasma' && (
            <div
              className="absolute inset-0 rounded-xl opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none"
              style={{
                background: `conic-gradient(from 0deg, transparent, ${
                  variant === 'matrix' ? 'rgba(34, 197, 94, 0.3)' :
                  variant === 'synthwave' ? 'rgba(236, 72, 153, 0.3)' :
                  variant === 'hologram' ? 'rgba(34, 211, 238, 0.3)' :
                  'rgba(34, 211, 238, 0.2)'
                }, transparent)`,
                animation: isHovered ? 'gradientRotate 4s linear infinite' : 'none',
                WebkitMask: 'linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0)',
                WebkitMaskComposite: 'xor',
                maskComposite: 'exclude',
                padding: '1px',
              }}
            />
          )}

          {/* Content with relative positioning */}
          <div className="relative z-10">
            {children}
          </div>

          {/* Corner decorations */}
          {renderCorners()}

          {/* Holographic shimmer overlay */}
          <div
            className="absolute inset-0 bg-gradient-to-r from-transparent via-white/[0.03] to-transparent translate-x-[-100%] group-hover:translate-x-[100%] transition-transform duration-1000 ease-out pointer-events-none"
          />

          {/* Secondary shimmer for depth */}
          <div
            className="absolute inset-0 bg-gradient-to-br from-transparent via-white/[0.02] to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none"
          />

          {/* Inner glow animation */}
          <div
            className="absolute inset-0 rounded-xl pointer-events-none"
            style={{
              boxShadow: 'inset 0 0 60px rgba(255, 255, 255, 0.02)',
              animation: glow === 'intense' ? 'innerGlow 3s ease-in-out infinite' : 'none',
            }}
          />
        </Component>
      </>
    )
  }
)

// Card sub-components for better composition
const CardHeader = forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn('flex flex-col space-y-1.5 p-6 pb-4', className)}
      {...props}
    />
  )
)

const CardTitle = forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLHeadingElement>>(
  ({ className, ...props }, ref) => (
    <h3
      ref={ref}
      className={cn(
        'text-xl font-semibold leading-none tracking-tight text-white',
        'bg-gradient-to-r from-white to-gray-300 bg-clip-text text-transparent',
        className
      )}
      {...props}
    />
  )
)

const CardDescription = forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLParagraphElement>>(
  ({ className, ...props }, ref) => (
    <p
      ref={ref}
      className={cn('text-sm text-gray-400 leading-relaxed', className)}
      {...props}
    />
  )
)

const CardContent = forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn('p-6 pt-0', className)} {...props} />
  )
)

const CardFooter = forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn('flex items-center p-6 pt-0', className)}
      {...props}
    />
  )
)

Card.displayName = 'Card'
CardHeader.displayName = 'CardHeader'
CardTitle.displayName = 'CardTitle'
CardDescription.displayName = 'CardDescription'
CardContent.displayName = 'CardContent'
CardFooter.displayName = 'CardFooter'

export {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
  cardVariants,
}
