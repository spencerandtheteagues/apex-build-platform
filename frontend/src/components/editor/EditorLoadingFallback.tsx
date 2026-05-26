// APEX-BUILD — accessible code-editor loading fallback
//
// Rendered inside <Suspense> while the lazy-loaded Monaco editor chunk arrives.
// It replaces a bare spinner with a lightweight code-editor skeleton (a gutter +
// shimmer lines) for better perceived load, and exposes a screen-reader status so
// the load is announced.
//
// IMPORTANT: import this from its own path (not the editor barrel index.ts) so the
// fallback stays free of the heavy monaco-editor dependency — it must be able to
// render *before* Monaco loads.

import React from 'react'
import { cn } from '@/lib/utils'

export interface EditorLoadingFallbackProps {
  /** Visible text that also serves as the screen-reader status announcement. */
  label?: string
  className?: string
}

// Varying widths so the skeleton reads like lines of code rather than a block.
const SKELETON_LINE_WIDTHS = ['78%', '54%', '88%', '40%', '66%', '72%', '34%', '60%']

export const EditorLoadingFallback: React.FC<EditorLoadingFallbackProps> = ({
  label = 'Loading editor...',
  className,
}) => (
  // role="status" carries an implicit aria-live="polite"; do not add an explicit
  // aria-live or some screen readers double-announce. The visible <p> text is the
  // accessible name, so no aria-label (which would override the visible text).
  <div
    role="status"
    aria-busy="true"
    className={cn('flex h-full w-full flex-col bg-gray-900/50', className)}
  >
    {/* Decorative skeleton — hidden from assistive tech, which gets the status text. */}
    <div className="flex min-h-0 flex-1 gap-3 overflow-hidden p-3" aria-hidden="true">
      <div className="flex w-8 shrink-0 flex-col gap-2 pt-1">
        {SKELETON_LINE_WIDTHS.map((_, i) => (
          <div
            key={i}
            className="h-3 w-4 rounded bg-gray-700/40 animate-pulse motion-reduce:animate-none"
          />
        ))}
      </div>
      <div className="flex flex-1 flex-col gap-2 pt-1">
        {SKELETON_LINE_WIDTHS.map((width, i) => (
          <div
            key={i}
            style={{ width }}
            className="h-3 rounded bg-gray-700/50 animate-pulse motion-reduce:animate-none"
          />
        ))}
      </div>
    </div>
    <p className="px-3 pb-3 text-sm text-gray-400">{label}</p>
  </div>
)

export default EditorLoadingFallback
