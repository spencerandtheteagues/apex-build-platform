import type { GlassBoxPatchBundle } from './BuildActivityTimeline'

export const patchBundleNeedsReview = (bundle: GlassBoxPatchBundle): boolean =>
  Boolean(bundle.review_required || bundle.merge_policy === 'review_required')

export const patchBundlePendingReview = (bundle: GlassBoxPatchBundle): boolean =>
  patchBundleNeedsReview(bundle) && bundle.review_status !== 'approved' && bundle.review_status !== 'rejected'
