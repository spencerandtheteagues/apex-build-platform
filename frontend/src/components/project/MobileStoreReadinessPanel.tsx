import React, { useMemo } from 'react'
import { ClipboardCheck, Download, FileCheck2, ShieldAlert } from 'lucide-react'

import { Button } from '@/components/ui'
import { cn } from '@/lib/utils'
import type { File, Project } from '@/types'
import type { MobileReadinessScorecard, MobileValidationReport } from '@/services/api'

interface StoreDataSafetyDraft {
  data_collected?: string[]
  data_linked_to_user?: string[]
  data_used_for_tracking?: string[]
  privacy_notes?: string[]
}

interface StoreScreenshotTarget {
  platform?: string
  device?: string
  purpose?: string
}

interface StoreReadinessPackage {
  status?: string
  app_name?: string
  short_description?: string
  full_description?: string
  category?: string
  release_notes?: string
  android_package?: string
  ios_bundle_identifier?: string
  version?: string
  version_code?: number
  build_number?: string
  data_safety_draft?: StoreDataSafetyDraft
  screenshot_checklist?: StoreScreenshotTarget[]
  manual_prerequisites?: string[]
  truthful_status_notes?: string[]
  missing_items?: string[]
}

interface MobileStoreReadinessPanelProps {
  project: Project
  files: File[]
  validation: MobileValidationReport | null
  scorecard: MobileReadinessScorecard | null
  onDownloadPackage?: () => void
}

const storePackagePath = 'mobile/store/store-readiness.json'

const statusClasses: Record<string, string> = {
  complete: 'border-emerald-300/20 bg-emerald-300/10 text-emerald-100',
  partial: 'border-cyan-300/20 bg-cyan-300/10 text-cyan-100',
  blocked: 'border-red-300/20 bg-red-300/10 text-red-100',
  warning: 'border-amber-300/20 bg-amber-300/10 text-amber-100',
  missing: 'border-white/10 bg-white/[0.03] text-gray-300',
}

const formatStoreStatus = (status?: string) => {
  if (!status) return 'Not generated'
  return status
    .split(/[_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

const parseStorePackage = (files: File[]) => {
  const file = files.find((candidate) => candidate.path === storePackagePath)
  if (!file?.content?.trim()) {
    return { file, package: null, error: null as string | null }
  }
  try {
    return {
      file,
      package: JSON.parse(file.content) as StoreReadinessPackage,
      error: null as string | null,
    }
  } catch {
    return { file, package: null, error: 'store-readiness.json is not valid JSON.' }
  }
}

const firstNonEmpty = (...values: Array<string | undefined | null>) =>
  values.find((value) => value && value.trim())?.trim() || ''

const countList = (items: unknown[] | undefined) => Array.isArray(items) ? items.length : 0

export const MobileStoreReadinessPanel: React.FC<MobileStoreReadinessPanelProps> = ({
  project,
  files,
  validation,
  scorecard,
  onDownloadPackage,
}) => {
  const storeEvidence = useMemo(() => parseStorePackage(files), [files])
  const storePackage = storeEvidence.package
  const storeCheck = validation?.checks.find((check) => check.id === 'store_readiness')
  const releaseTruthCheck = validation?.checks.find((check) => check.id === 'release_truth')
  const storeCategory = scorecard?.categories.find((category) => category.id === 'store_readiness')
  const missingItems = storePackage?.missing_items || []
  const manualPrerequisites = storePackage?.manual_prerequisites || []
  const truthfulNotes = storePackage?.truthful_status_notes || []
  const statusClass = storeCheck?.status === 'passed'
    ? statusClasses.partial
    : storeCheck?.status === 'failed' || storeEvidence.error
      ? statusClasses.blocked
      : statusClasses.missing
  const appName = firstNonEmpty(storePackage?.app_name, project.app_display_name, project.name)
  const androidPackage = firstNonEmpty(storePackage?.android_package, project.android_package)
  const iosBundle = firstNonEmpty(storePackage?.ios_bundle_identifier, project.ios_bundle_identifier)
  const version = firstNonEmpty(storePackage?.version, project.app_version, '1.0.0')

  const checklistItems = [
    {
      label: 'Listing metadata',
      value: storePackage?.short_description && storePackage?.category ? 'Draft present' : 'Needs owner review',
      detail: storePackage?.short_description || 'Short description/category will be generated into the store package.',
      state: storePackage?.short_description && storePackage?.category ? 'partial' : 'warning',
    },
    {
      label: 'Privacy/Data safety',
      value: `${countList(storePackage?.data_safety_draft?.data_collected)} data items`,
      detail: storePackage?.data_safety_draft?.privacy_notes?.[0] || 'Draft privacy answers must be reviewed before submission.',
      state: storePackage?.data_safety_draft ? 'partial' : 'warning',
    },
    {
      label: 'Screenshots',
      value: `${countList(storePackage?.screenshot_checklist)} targets`,
      detail: 'Native-device screenshots are required; Expo Web screenshots are not proof of native behavior.',
      state: countList(storePackage?.screenshot_checklist) > 0 ? 'partial' : 'warning',
    },
    {
      label: 'Release notes',
      value: storePackage?.release_notes ? 'Draft present' : 'Initial draft needed',
      detail: storePackage?.release_notes || 'Release notes should match the actual signed build users receive.',
      state: storePackage?.release_notes ? 'partial' : 'warning',
    },
  ]

  return (
    <section
      className="rounded-[28px] border border-amber-300/16 bg-[linear-gradient(180deg,rgba(18,20,14,0.94)_0%,rgba(7,8,10,0.98)_100%)] shadow-[0_24px_80px_rgba(0,0,0,0.24)]"
      data-testid="mobile-store-readiness-panel"
    >
      <div className="grid gap-0 lg:grid-cols-[minmax(0,1.1fr)_minmax(320px,0.9fr)]">
        <div className="relative overflow-hidden px-6 py-6 md:px-8">
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(251,191,36,0.14),transparent_34%),radial-gradient(circle_at_bottom_right,rgba(34,211,238,0.08),transparent_30%)]" />
          <div className="relative">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-amber-300/20 bg-amber-300/10 text-amber-100">
                  <ClipboardCheck className="h-6 w-6" />
                </div>
                <div>
                  <div className="text-[11px] font-semibold uppercase tracking-[0.28em] text-amber-200/80">
                    Store Readiness
                  </div>
                  <h2 className="mt-1 text-2xl font-semibold tracking-[-0.03em] text-white">
                    Metadata and release checklist
                  </h2>
                </div>
              </div>
              <span className={cn('rounded-full border px-3 py-1 text-[10px] font-semibold uppercase tracking-[0.16em]', statusClass)}>
                {storeCheck?.status === 'passed' ? 'Draft valid' : storeCheck?.status === 'failed' ? 'Needs fixes' : 'Missing package'}
              </span>
            </div>

            <p className="mt-5 max-w-3xl text-sm leading-7 text-gray-300">
              Apex generates store metadata, privacy/data-safety drafts, screenshot targets, and release notes as launch-prep assets.
              This is separate from EAS Submit, App Store review, Google Play review, and final approval.
            </p>

            <div className="mt-5 grid gap-3 md:grid-cols-2">
              <div className="rounded-2xl border border-white/8 bg-black/24 px-4 py-4">
                <div className="text-[11px] uppercase tracking-[0.22em] text-gray-500">App identity</div>
                <div className="mt-2 text-sm font-semibold text-white">{appName}</div>
                <div className="mt-1 text-xs text-gray-400">Version {version}</div>
              </div>
              <div className="rounded-2xl border border-white/8 bg-black/24 px-4 py-4">
                <div className="text-[11px] uppercase tracking-[0.22em] text-gray-500">Store status</div>
                <div className="mt-2 text-sm font-semibold text-white">
                  {formatStoreStatus(storePackage?.status || validation?.store_readiness_state)}
                </div>
                <div className="mt-1 text-xs text-gray-400">{storeCategory ? `${storeCategory.score}% evidence score` : 'Score pending'}</div>
              </div>
            </div>

            <div className="mt-5 grid gap-3 md:grid-cols-2">
              {checklistItems.map((item) => (
                <div key={item.label} className="rounded-2xl border border-white/8 bg-black/22 px-4 py-4">
                  <div className="flex items-center justify-between gap-3">
                    <div className="text-sm font-semibold text-white">{item.label}</div>
                    <span className={cn('rounded-full border px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.14em]', statusClasses[item.state])}>
                      {item.value}
                    </span>
                  </div>
                  <p className="mt-2 text-xs leading-5 text-gray-400">{item.detail}</p>
                </div>
              ))}
            </div>

            {releaseTruthCheck ? (
              <div className={cn(
                'mt-5 rounded-2xl border px-4 py-3',
                releaseTruthCheck.status === 'passed'
                  ? 'border-emerald-300/14 bg-emerald-300/8'
                  : 'border-amber-300/14 bg-amber-300/8',
              )}>
                <div className="flex items-start gap-3">
                  <ShieldAlert className={cn('mt-0.5 h-4 w-4 shrink-0', releaseTruthCheck.status === 'passed' ? 'text-emerald-200' : 'text-amber-200')} />
                  <div>
                    <div className="text-xs font-semibold uppercase tracking-[0.16em] text-white/80">
                      Release truth gate
                    </div>
                    <p className="mt-1 text-xs leading-5 text-gray-200">{releaseTruthCheck.detail}</p>
                  </div>
                </div>
              </div>
            ) : null}
          </div>
        </div>

        <div className="border-t border-white/6 bg-black/22 p-6 lg:border-l lg:border-t-0">
          <div className="rounded-2xl border border-white/8 bg-black/24 px-4 py-4">
            <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.22em] text-amber-200/80">
              <FileCheck2 className="h-3.5 w-3.5" />
              Exported Files
            </div>
            <div className="mt-4 space-y-2 text-xs leading-5 text-gray-300">
              <div className="font-mono text-amber-50/90">mobile/store/store-readiness.json</div>
              <div className="font-mono text-amber-50/90">mobile/store/privacy-data-safety.md</div>
              <div className="font-mono text-amber-50/90">mobile/store/screenshot-checklist.md</div>
              <div className="font-mono text-amber-50/90">mobile/store/release-notes.md</div>
            </div>
            {onDownloadPackage ? (
              <Button
                type="button"
                size="sm"
                variant="ghost"
                onClick={onDownloadPackage}
                className="mt-4 w-full justify-center rounded-xl border border-amber-300/14 bg-amber-300/8 px-3 text-xs text-amber-50"
              >
                <Download className="mr-2 h-3.5 w-3.5" />
                Download ZIP with store package
              </Button>
            ) : null}
          </div>

          <div className="mt-4 rounded-2xl border border-white/8 bg-black/24 px-4 py-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-gray-500">Identifiers</div>
            <div className="mt-3 space-y-2 text-xs leading-5 text-gray-300">
              <div>
                <span className="text-gray-500">Android:</span> {androidPackage || 'Not set'}
              </div>
              <div>
                <span className="text-gray-500">iOS:</span> {iosBundle || 'Not set'}
              </div>
            </div>
          </div>

          <div className="mt-4 rounded-2xl border border-white/8 bg-black/24 px-4 py-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.22em] text-gray-500">Manual Before Submission</div>
            <div className="mt-3 space-y-2 text-xs leading-5 text-gray-300">
              {(missingItems.length > 0 ? missingItems : manualPrerequisites).slice(0, 5).map((item) => (
                <div key={item} className="flex items-start gap-2">
                  <span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-amber-200/80" />
                  <span>{item}</span>
                </div>
              ))}
              {!missingItems.length && !manualPrerequisites.length ? (
                <div className="text-gray-400">Manual prerequisites will appear once the store-readiness package is generated.</div>
              ) : null}
            </div>
          </div>

          <div className="mt-4 rounded-2xl border border-red-300/14 bg-red-300/8 px-4 py-3">
            <p className="text-xs leading-5 text-red-50/90">
              {truthfulNotes[0] || 'This panel does not mean Apple or Google approved the app. Approval remains a separate external review state.'}
            </p>
            {storeEvidence.error ? (
              <p className="mt-2 text-xs leading-5 text-red-50/90">{storeEvidence.error}</p>
            ) : null}
          </div>
        </div>
      </div>
    </section>
  )
}

export default MobileStoreReadinessPanel
