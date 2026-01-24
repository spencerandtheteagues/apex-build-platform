// APEX.BUILD UI - Textarea Component
// Cyberpunk-styled textarea with neon glow effects

import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const textareaVariants = cva(
  'flex min-h-[80px] w-full rounded-md border bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 transition-all duration-300 resize-none',
  {
    variants: {
      variant: {
        default:
          'border-gray-700 bg-black/50 text-gray-100 focus:border-cyan-500 focus:ring-1 focus:ring-cyan-500/50 focus:shadow-[0_0_15px_rgba(0,255,255,0.3)]',
        cyberpunk:
          'border-cyan-500/50 bg-black/70 text-cyan-100 focus:border-cyan-400 focus:ring-2 focus:ring-cyan-500/50 focus:shadow-[0_0_20px_rgba(0,255,255,0.4)] placeholder:text-cyan-700',
        matrix:
          'border-green-500/50 bg-black/80 text-green-100 focus:border-green-400 focus:ring-2 focus:ring-green-500/50 focus:shadow-[0_0_20px_rgba(0,255,0,0.4)] placeholder:text-green-700',
        synthwave:
          'border-pink-500/50 bg-black/70 text-pink-100 focus:border-pink-400 focus:ring-2 focus:ring-pink-500/50 focus:shadow-[0_0_20px_rgba(255,0,255,0.4)] placeholder:text-pink-700',
        neonCity:
          'border-purple-500/50 bg-black/70 text-purple-100 focus:border-purple-400 focus:ring-2 focus:ring-purple-500/50 focus:shadow-[0_0_20px_rgba(128,0,255,0.4)] placeholder:text-purple-700',
        error:
          'border-red-500/50 bg-black/70 text-red-100 focus:border-red-400 focus:ring-2 focus:ring-red-500/50 focus:shadow-[0_0_20px_rgba(255,0,0,0.4)]',
        success:
          'border-emerald-500/50 bg-black/70 text-emerald-100 focus:border-emerald-400 focus:ring-2 focus:ring-emerald-500/50 focus:shadow-[0_0_20px_rgba(0,255,128,0.4)]',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)

export interface TextareaProps
  extends React.TextareaHTMLAttributes<HTMLTextAreaElement>,
    VariantProps<typeof textareaVariants> {}

const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, variant, ...props }, ref) => {
    return (
      <textarea
        className={cn(textareaVariants({ variant, className }))}
        ref={ref}
        {...props}
      />
    )
  }
)
Textarea.displayName = 'Textarea'

export { Textarea, textareaVariants }
