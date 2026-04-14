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
  review_status?: 'pending' | 'approved' | 'rejected'
  reviewed_at?: string
  review_message?: string
  created_at?: string
}

interface AIRepairReviewPanelProps {
  bundles: AIRepairReviewBundle[]
  proposedEditsCount?: number
  onOpenProposedEdits?: () => void
  onApproveBundle?: (bundleId: string) => void
  onRejectBundle?: (bundleId: string) => void
  reviewActionId?: string | null
  maxVisible?: number
}

const humanize = (value?: string): string =>
  String(value || '')
    .replace(/_/g, ' ')
    .trim()
    .replace(/\b\w/g, (part) => part.toUpperCase())

const bundleNeedsReview = (bundle: AIRepairReviewBundle): boolean =>
  Boolean(bundle.review_required || bundle.merge_policy === 'review_required')

const bundlePendingReview = (bundle: AIRepairReviewBundle): boolean =>
  bundleNeedsReview(bundle) && bundle.review_status !== 'approved' && bundle.review_status !== 'rejected'

export default function AIRepairReviewPanel({
  bundles,
  proposedEditsCount = 0,
  onOpenProposedEdits,
  onApproveBundle,
  onRejectBundle,
  reviewActionId = null,
  maxVisible = 3,
}: AIRepairReviewPanelProps) {
  const reviewBundles = bundles.filter(bundlePendingReview)
  if (reviewBundles.length === 0) {
    return null
  }

  const visibleBundles = reviewBundles.slice(0, maxVisible)
  const hiddenCount = Math.max(reviewBundles.length - visibleBundles.length, 0)
  const hasProposedEdits = proposedEditsCount > 0 && Boolean(onOpenProposedEdits)
  const proposedEditsMessage = hasProposedEdits
    ? `${proposedEditsCount} proposed edit${proposedEditsCount === 1 ? '' : 's'} available for approve/reject review.`
    : proposedEditsCount > 0
      ? `${proposedEditsCount} proposed edit${proposedEditsCount === 1 ? '' : 's'} recorded; code review opens when the build reaches review state.`
      : 'Patch metadata is recorded; no proposed-edit diff is attached yet.'

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
                <div className="mt-2 text-[11px] text-gray-500">
                  {proposedEditsMessage}
                </div>
              </div>
              <div className="flex shrink-0 flex-col items-end gap-1.5">
                {hasProposedEdits && (
                  <button
                    type="button"
                    onClick={onOpenProposedEdits}
                    className="text-[10px] font-bold uppercase tracking-widest px-2 py-1 rounded border border-violet-500/40 text-violet-200 bg-violet-500/10 hover:bg-violet-500/20"
                  >
                    Open Diff Review
                  </button>
                )}
                <div className="flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => onRejectBundle?.(bundle.id)}
                    disabled={!onRejectBundle || reviewActionId === bundle.id}
                    className="text-[10px] font-bold uppercase tracking-widest px-2 py-1 rounded border border-red-500/35 text-red-200 bg-red-500/10 hover:bg-red-500/20 disabled:opacity-50"
                  >
                    Reject
                  </button>
                  <button
                    type="button"
                    onClick={() => onApproveBundle?.(bundle.id)}
                    disabled={!onApproveBundle || reviewActionId === bundle.id}
                    className="text-[10px] font-bold uppercase tracking-widest px-2 py-1 rounded border border-emerald-500/35 text-emerald-100 bg-emerald-500/15 hover:bg-emerald-500/25 disabled:opacity-50"
                  >
                    {reviewActionId === bundle.id ? 'Saving' : 'Approve'}
                  </button>
                </div>
              </div>
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
