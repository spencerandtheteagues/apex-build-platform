import React from 'react'

export interface AIRepairReviewBundle {
  id: string
  provider?: string
  merge_policy?: 'auto_merge_safe' | 'review_required'
  review_required?: boolean
  review_branch?: string
  suggested_commit_title?: string
  risk_reasons?: string[]
  justification?: string
  created_at?: string
}

interface AIRepairReviewPanelProps {
  bundles: AIRepairReviewBundle[]
  maxVisible?: number
}

const humanize = (value?: string): string =>
  String(value || '')
    .replace(/_/g, ' ')
    .trim()
    .replace(/\b\w/g, (part) => part.toUpperCase())

export default function AIRepairReviewPanel({
  bundles,
  maxVisible = 3,
}: AIRepairReviewPanelProps) {
  const reviewBundles = bundles.filter((bundle) => bundle.review_required || bundle.merge_policy === 'review_required')
  if (reviewBundles.length === 0) {
    return null
  }

  const visibleBundles = reviewBundles.slice(0, maxVisible)
  const hiddenCount = Math.max(reviewBundles.length - visibleBundles.length, 0)

  return (
    <div>
      <h3 className="text-xs font-bold uppercase tracking-widest text-gray-500 mb-2">
        Repair Patch Review ({reviewBundles.length})
      </h3>
      <div className="space-y-2">
        {visibleBundles.map((bundle) => (
          <div key={bundle.id} className="rounded-xl border border-violet-500/30 bg-violet-950/15 p-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <div className="text-sm font-semibold text-white">
                  {bundle.justification || 'Patch bundle requires review before merge.'}
                </div>
                <div className="mt-1 text-xs text-gray-400">
                  {bundle.provider ? `${bundle.provider} · ` : ''}merge policy: review required
                </div>
                {bundle.review_branch && (
                  <div className="mt-1 text-[11px] font-mono text-cyan-300">
                    review branch: {bundle.review_branch}
                  </div>
                )}
                {bundle.suggested_commit_title && (
                  <div className="mt-1 text-[11px] text-gray-400">
                    suggested commit: {bundle.suggested_commit_title}
                  </div>
                )}
              </div>
              <span className="text-[9px] font-bold uppercase tracking-widest px-1.5 py-0.5 rounded border border-violet-500/40 text-violet-300 bg-violet-500/10 shrink-0">
                Review
              </span>
            </div>
            {Array.isArray(bundle.risk_reasons) && bundle.risk_reasons.length > 0 && (
              <div className="mt-3 flex flex-wrap gap-1.5">
                {bundle.risk_reasons.slice(0, 3).map((reason) => (
                  <span key={`${bundle.id}-${reason}`} className="text-[10px] border border-gray-700 bg-black/35 text-gray-300 rounded px-1.5 py-0.5">
                    {humanize(reason)}
                  </span>
                ))}
              </div>
            )}
          </div>
        ))}
        {hiddenCount > 0 && (
          <div className="text-xs text-gray-500">
            +{hiddenCount} more review-required patch bundles recorded in this build.
          </div>
        )}
      </div>
    </div>
  )
}
