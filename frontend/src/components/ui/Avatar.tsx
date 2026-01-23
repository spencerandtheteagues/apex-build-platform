// APEX.BUILD Cyberpunk Avatar Component
// Futuristic user avatars with holographic borders

import React, { forwardRef, useState } from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'
import { User } from 'lucide-react'

const avatarVariants = cva(
  'relative inline-flex items-center justify-center overflow-hidden rounded-full border-2 transition-all duration-300',
  {
    variants: {
      size: {
        xs: 'w-6 h-6 text-xs',
        sm: 'w-8 h-8 text-sm',
        md: 'w-10 h-10 text-sm',
        lg: 'w-12 h-12 text-base',
        xl: 'w-16 h-16 text-lg',
        '2xl': 'w-20 h-20 text-xl',
        '3xl': 'w-24 h-24 text-2xl',
      },
      variant: {
        // Cyberpunk cyan theme
        cyberpunk: 'border-cyan-400/60 bg-gradient-to-br from-cyan-500/20 to-blue-600/20 shadow-lg shadow-cyan-500/20',

        // Matrix green theme
        matrix: 'border-green-400/60 bg-gradient-to-br from-green-500/20 to-emerald-600/20 shadow-lg shadow-green-500/20',

        // Synthwave pink/purple theme
        synthwave: 'border-pink-400/60 bg-gradient-to-br from-pink-500/20 to-purple-600/20 shadow-lg shadow-pink-500/20',

        // Neon city blue theme
        neonCity: 'border-blue-400/60 bg-gradient-to-br from-blue-500/20 to-indigo-600/20 shadow-lg shadow-blue-500/20',

        // Neutral gray
        neutral: 'border-gray-500/60 bg-gradient-to-br from-gray-500/20 to-gray-600/20 shadow-lg shadow-gray-500/20',

        // Status variants
        online: 'border-green-400/60 bg-gradient-to-br from-green-500/20 to-emerald-600/20 shadow-lg shadow-green-500/20',
        offline: 'border-gray-400/60 bg-gradient-to-br from-gray-500/20 to-gray-600/20',
        away: 'border-yellow-400/60 bg-gradient-to-br from-yellow-500/20 to-orange-600/20 shadow-lg shadow-yellow-500/20',
        busy: 'border-red-400/60 bg-gradient-to-br from-red-500/20 to-pink-600/20 shadow-lg shadow-red-500/20',
      },
      glow: {
        none: '',
        subtle: 'hover:shadow-xl',
        intense: 'hover:shadow-2xl hover:scale-105',
      },
    },
    defaultVariants: {
      size: 'md',
      variant: 'cyberpunk',
      glow: 'subtle',
    },
  }
)

const avatarImageVariants = cva(
  'aspect-square h-full w-full object-cover',
  {
    variants: {
      shape: {
        circle: 'rounded-full',
        square: 'rounded-lg',
      },
    },
    defaultVariants: {
      shape: 'circle',
    },
  }
)

const avatarFallbackVariants = cva(
  'flex h-full w-full items-center justify-center font-medium text-white bg-gradient-to-br',
  {
    variants: {
      variant: {
        cyberpunk: 'from-cyan-500 to-blue-600',
        matrix: 'from-green-500 to-emerald-600',
        synthwave: 'from-pink-500 to-purple-600',
        neonCity: 'from-blue-500 to-indigo-600',
        neutral: 'from-gray-500 to-gray-600',
        online: 'from-green-500 to-emerald-600',
        offline: 'from-gray-500 to-gray-600',
        away: 'from-yellow-500 to-orange-600',
        busy: 'from-red-500 to-pink-600',
      },
    },
    defaultVariants: {
      variant: 'cyberpunk',
    },
  }
)

export interface AvatarProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof avatarVariants> {
  src?: string
  alt?: string
  fallback?: string
  status?: 'online' | 'offline' | 'away' | 'busy'
  showStatus?: boolean
  shape?: 'circle' | 'square'
}

const Avatar = forwardRef<HTMLDivElement, AvatarProps>(
  (
    {
      className,
      size,
      variant,
      glow,
      src,
      alt,
      fallback,
      status,
      showStatus = false,
      shape = 'circle',
      ...props
    },
    ref
  ) => {
    const [imageError, setImageError] = useState(false)

    // Use status as variant if status is provided and showStatus is true
    const currentVariant = showStatus && status ? status : variant

    // Generate initials from fallback text
    const getInitials = (text?: string) => {
      if (!text) return <User className="w-1/2 h-1/2" />

      const words = text.trim().split(' ')
      if (words.length === 1) {
        return words[0].charAt(0).toUpperCase()
      }
      return (words[0].charAt(0) + words[words.length - 1].charAt(0)).toUpperCase()
    }

    return (
      <div
        className={cn(avatarVariants({ size, variant: currentVariant, glow }), className)}
        ref={ref}
        {...props}
      >
        {/* Avatar image or fallback */}
        {src && !imageError ? (
          <img
            src={src}
            alt={alt}
            className={cn(avatarImageVariants({ shape }))}
            onError={() => setImageError(true)}
          />
        ) : (
          <div className={cn(avatarFallbackVariants({ variant: currentVariant }))}>
            {getInitials(fallback || alt)}
          </div>
        )}

        {/* Status indicator */}
        {showStatus && status && (
          <div className="absolute -bottom-0.5 -right-0.5">
            <div
              className={cn(
                'w-3 h-3 rounded-full border-2 border-gray-900',
                {
                  'bg-green-400': status === 'online',
                  'bg-gray-400': status === 'offline',
                  'bg-yellow-400': status === 'away',
                  'bg-red-400': status === 'busy',
                }
              )}
            />
          </div>
        )}

        {/* Holographic border shimmer */}
        <div className="absolute inset-0 rounded-full bg-gradient-to-r from-transparent via-white/10 to-transparent translate-x-[-100%] group-hover:translate-x-[100%] transition-transform duration-1000 ease-out pointer-events-none" />

        {/* Corner accents for cyberpunk look */}
        {shape === 'square' && (
          <>
            <div className="absolute top-0 left-0 w-2 h-2 border-t border-l border-current opacity-30" />
            <div className="absolute top-0 right-0 w-2 h-2 border-t border-r border-current opacity-30" />
            <div className="absolute bottom-0 left-0 w-2 h-2 border-b border-l border-current opacity-30" />
            <div className="absolute bottom-0 right-0 w-2 h-2 border-b border-r border-current opacity-30" />
          </>
        )}
      </div>
    )
  }
)

// Avatar group component for showing multiple avatars
export interface AvatarGroupProps extends React.HTMLAttributes<HTMLDivElement> {
  children: React.ReactNode
  max?: number
  size?: AvatarProps['size']
  spacing?: 'sm' | 'md' | 'lg'
}

const AvatarGroup = forwardRef<HTMLDivElement, AvatarGroupProps>(
  ({ className, children, max = 5, size = 'md', spacing = 'md', ...props }, ref) => {
    const avatars = React.Children.toArray(children)
    const displayAvatars = max ? avatars.slice(0, max) : avatars
    const remainingCount = avatars.length - displayAvatars.length

    const spacingMap = {
      sm: '-space-x-1',
      md: '-space-x-2',
      lg: '-space-x-3',
    }

    return (
      <div
        className={cn('flex', spacingMap[spacing], className)}
        ref={ref}
        {...props}
      >
        {displayAvatars.map((avatar, index) =>
          React.cloneElement(avatar as React.ReactElement, {
            key: index,
            size,
            className: 'ring-2 ring-gray-900',
          })
        )}

        {/* Overflow indicator */}
        {remainingCount > 0 && (
          <Avatar
            size={size}
            fallback={`+${remainingCount}`}
            variant="neutral"
            className="ring-2 ring-gray-900"
          />
        )}
      </div>
    )
  }
)

Avatar.displayName = 'Avatar'
AvatarGroup.displayName = 'AvatarGroup'

export { Avatar, AvatarGroup, avatarVariants }