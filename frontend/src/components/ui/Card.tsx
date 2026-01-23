// APEX.BUILD Cyberpunk Card Component
// Holographic glass morphism cards with neon accents

import React, { forwardRef } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const cardVariants = cva(
  'relative rounded-lg overflow-hidden transition-all duration-300 group',
  {
    variants: {
      variant: {
        // Glass morphism with subtle glow
        default: 'bg-gray-900/80 backdrop-blur-md border border-gray-700/50 hover:border-gray-600/70 shadow-xl hover:shadow-2xl',

        // Cyberpunk style with cyan accents
        cyberpunk: 'bg-gradient-to-br from-gray-900/90 to-cyan-950/30 backdrop-blur-md border border-cyan-500/30 hover:border-cyan-400/60 shadow-lg shadow-cyan-500/10 hover:shadow-cyan-400/20',

        // Matrix green theme
        matrix: 'bg-gradient-to-br from-black/90 to-green-950/30 backdrop-blur-md border border-green-500/30 hover:border-green-400/60 shadow-lg shadow-green-500/10 hover:shadow-green-400/20',

        // Synthwave pink/purple
        synthwave: 'bg-gradient-to-br from-purple-900/80 to-pink-950/40 backdrop-blur-md border border-pink-500/30 hover:border-pink-400/60 shadow-lg shadow-pink-500/10 hover:shadow-pink-400/20',

        // Neon city blue
        neonCity: 'bg-gradient-to-br from-blue-950/80 to-cyan-950/40 backdrop-blur-md border border-blue-500/30 hover:border-blue-400/60 shadow-lg shadow-blue-500/10 hover:shadow-blue-400/20',

        // Solid dark card
        solid: 'bg-gray-800 border border-gray-700 hover:border-gray-600 shadow-lg hover:shadow-xl',

        // Interactive card (clickable)
        interactive: 'bg-gray-900/80 backdrop-blur-md border border-gray-700/50 hover:border-cyan-400/70 hover:bg-gray-800/90 cursor-pointer transform hover:scale-[1.02] shadow-lg hover:shadow-cyan-500/20',
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
        subtle: 'before:absolute before:inset-0 before:bg-gradient-to-r before:from-transparent before:via-white/5 before:to-transparent before:translate-x-[-100%] group-hover:before:translate-x-[100%] before:transition-transform before:duration-1000',
        intense: 'before:absolute before:inset-0 before:bg-gradient-to-r before:from-transparent before:via-white/10 before:to-transparent before:translate-x-[-100%] group-hover:before:translate-x-[100%] before:transition-transform before:duration-700',
      },
    },
    defaultVariants: {
      variant: 'cyberpunk',
      padding: 'md',
      glow: 'subtle',
    },
  }
)

export interface CardProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof cardVariants> {
  as?: React.ElementType
}

const Card = forwardRef<HTMLDivElement, CardProps>(
  ({ className, variant, padding, glow, as: Component = 'div', children, ...props }, ref) => {
    return (
      <Component
        className={cn(cardVariants({ variant, padding, glow }), className)}
        ref={ref}
        {...props}
      >
        {children}

        {/* Corner accents for cyberpunk aesthetic */}
        <div className="absolute top-0 left-0 w-3 h-3 border-t-2 border-l-2 border-current opacity-20" />
        <div className="absolute top-0 right-0 w-3 h-3 border-t-2 border-r-2 border-current opacity-20" />
        <div className="absolute bottom-0 left-0 w-3 h-3 border-b-2 border-l-2 border-current opacity-20" />
        <div className="absolute bottom-0 right-0 w-3 h-3 border-b-2 border-r-2 border-current opacity-20" />

        {/* Holographic shimmer overlay */}
        <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/[0.02] to-transparent translate-x-[-100%] group-hover:translate-x-[100%] transition-transform duration-1000 ease-out pointer-events-none" />
      </Component>
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
      className={cn('text-xl font-semibold leading-none tracking-tight text-white', className)}
      {...props}
    />
  )
)

const CardDescription = forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLParagraphElement>>(
  ({ className, ...props }, ref) => (
    <p
      ref={ref}
      className={cn('text-sm text-gray-400', className)}
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