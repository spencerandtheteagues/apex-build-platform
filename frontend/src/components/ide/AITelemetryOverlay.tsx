import React, { useMemo } from 'react'
import { Activity, ShieldCheck, Target, Workflow } from 'lucide-react'
import { cn } from '@/lib/utils'
import BuildActivityTimeline, {
  type GlassBoxPatchBundle,
  type GlassBoxThought,
  type GlassBoxVerificationReport,
  type GlassBoxWorkOrder,
} from './BuildActivityTimeline'
import HydraRacePanel, { type HydraProviderPanel } from './HydraRacePanel'
import { patchBundlePendingReview } from './patchBundleReview'
import type {
  BuildLearningSummaryState,
  BuildPromptPackActivationEventState,
  BuildPromptPackActivationRequestState,
  BuildPromptPackVersionState,
} from '@/services/api'

export interface GlassBoxProviderScorecard {
  provider: string
  task_shape: string
  compile_pass_rate?: number
  first_pass_verification_pass_rate?: number
  repair_success_rate?: number
  average_latency_seconds?: number
  average_cost_per_success?: number
  sample_count?: number
  first_pass_sample_count?: number
  repair_attempt_count?: number
  promotion_attempt_count?: number
}

interface AITelemetryOverlayProps {
  buildStatus: string
  currentPhase?: string
  qualityGateStatus?: string
  qualityGateStage?: string
  aiThoughts: GlassBoxThought[]
  providerPanels: HydraProviderPanel[]
  workOrders?: GlassBoxWorkOrder[]
  verificationReports?: GlassBoxVerificationReport[]
  patchBundles?: GlassBoxPatchBundle[]
  providerScorecards?: GlassBoxProviderScorecard[]
  historicalLearning?: BuildLearningSummaryState
  promptPackActivationRequests?: BuildPromptPackActivationRequestState[]
  promptPackVersions?: BuildPromptPackVersionState[]
  promptPackActivationEvents?: BuildPromptPackActivationEventState[]
  promptProposalActionId?: string | null
  onReviewPromptProposal?: (proposalId: string, decision: 'approve' | 'reject') => void
  onBenchmarkPromptProposal?: (proposalId: string) => void
  onCreatePromptPackDraft?: () => void
  onRequestPromptPackActivation?: (draftId: string) => void
  onActivatePromptPackRequest?: (requestId: string) => void
  onRollbackPromptPackVersion?: (versionId: string) => void
}

const percent = (value?: number): string => {
  if (typeof value !== 'number' || Number.isNaN(value)) return 'n/a'
  if (value <= 1) return `${Math.round(value * 100)}%`
  return `${Math.round(value)}%`
}

const humanize = (value?: string): string =>
  String(value || '')
    .replace(/_/g, ' ')
    .trim()
    .replace(/\b\w/g, (part) => part.toUpperCase())

const observedScorecardSamples = (scorecard: GlassBoxProviderScorecard): number =>
  Math.max(
    scorecard.sample_count || 0,
    scorecard.first_pass_sample_count || 0,
    scorecard.repair_attempt_count || 0,
    scorecard.promotion_attempt_count || 0,
  )

export default function AITelemetryOverlay({
  buildStatus,
  currentPhase,
  qualityGateStatus,
  qualityGateStage,
  aiThoughts,
  providerPanels,
  workOrders = [],
  verificationReports = [],
  patchBundles = [],
  providerScorecards = [],
  historicalLearning,
  promptPackActivationRequests = [],
  promptPackVersions = [],
  promptPackActivationEvents = [],
  promptProposalActionId,
  onReviewPromptProposal,
  onBenchmarkPromptProposal,
  onCreatePromptPackDraft,
  onRequestPromptPackActivation,
  onActivatePromptPackRequest,
  onRollbackPromptPackVersion,
}: AITelemetryOverlayProps) {
  const summary = useMemo(() => {
    const activeProviders = providerPanels.filter((panel) => panel.status === 'working' || panel.status === 'thinking').length
    const reviewRequired = patchBundles.filter(patchBundlePendingReview).length
    const failedVerification = verificationReports.filter(
      (report) => report.status === 'failed' || report.status === 'blocked'
    ).length
    const observedScorecards = providerScorecards.filter((scorecard) => observedScorecardSamples(scorecard) > 0)
    const compileRates = observedScorecards
      .map((scorecard) => scorecard.compile_pass_rate)
      .filter((value): value is number => typeof value === 'number')
    const averageCompile = compileRates.length > 0
      ? compileRates.reduce((sum, current) => sum + current, 0) / compileRates.length
      : undefined

    return {
      activeProviders,
      reviewRequired,
      failedVerification,
      averageCompile,
      observedScorecards,
    }
  }, [patchBundles, providerPanels, providerScorecards, verificationReports])

  const learningSignals = [
    ...(historicalLearning?.repair_strategy_win_rates || []),
    ...(historicalLearning?.semantic_repair_hints || []),
    ...(historicalLearning?.recurring_failure_classes || []),
  ].slice(0, 5)
  const promptProposals = (historicalLearning?.prompt_improvement_proposals || []).slice(0, 3)
  const adoptionCandidates = (historicalLearning?.prompt_adoption_candidates || []).slice(0, 3)
  const promptPackDrafts = (historicalLearning?.prompt_pack_drafts || []).slice(0, 3)
  const activationRequestsByDraft = useMemo(() => {
    const entries = new Map<string, BuildPromptPackActivationRequestState>()
    for (const request of promptPackActivationRequests || []) {
      if (request?.draft_id) {
        entries.set(request.draft_id, request)
      }
    }
    return entries
  }, [promptPackActivationRequests])
  const registryVersionsByRequest = useMemo(() => {
    const entries = new Map<string, BuildPromptPackVersionState>()
    for (const version of promptPackVersions || []) {
      if (version?.source_request_id) {
        entries.set(version.source_request_id, version)
      }
    }
    return entries
  }, [promptPackVersions])

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3">
        <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-gray-500">Build State</div>
          <div className="mt-2 text-sm font-semibold text-white">{humanize(buildStatus)}</div>
          <div className="mt-1 text-xs text-gray-400">
            {currentPhase ? `Phase: ${humanize(currentPhase)}` : 'Phase not reported'}
          </div>
        </div>

        <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-gray-500">Live Providers</div>
          <div className="mt-2 text-sm font-semibold text-white">{summary.activeProviders}</div>
          <div className="mt-1 text-xs text-gray-400">
            {providerPanels.length} total panel{providerPanels.length === 1 ? '' : 's'}
          </div>
        </div>

        <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-gray-500">Quality Gate</div>
          <div className="mt-2 text-sm font-semibold text-white">{humanize(qualityGateStatus || 'pending')}</div>
          <div className="mt-1 text-xs text-gray-400">
            {qualityGateStage ? `Stage: ${humanize(qualityGateStage)}` : 'Stage not reported'}
          </div>
        </div>

        <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-3">
          <div className="text-[10px] uppercase tracking-[0.18em] text-gray-500">Provider Compile Avg</div>
          <div className="mt-2 text-sm font-semibold text-white">{percent(summary.averageCompile)}</div>
          <div className="mt-1 text-xs text-gray-400">
            {summary.observedScorecards.length} weighted scorecard{summary.observedScorecards.length === 1 ? '' : 's'}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-3">
        <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-3">
          <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.2em] text-gray-500">
            <Workflow className="w-3.5 h-3.5" />
            Work Orders
          </div>
          <div className="mt-2 text-lg font-semibold text-white">{workOrders.length}</div>
        </div>

        <div className={cn(
          'rounded-lg border px-3 py-3',
          summary.failedVerification > 0 ? 'border-amber-500/30 bg-amber-500/10' : 'border-gray-800 bg-gray-950/50'
        )}>
          <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.2em] text-gray-500">
            <ShieldCheck className="w-3.5 h-3.5" />
            Verification Issues
          </div>
          <div className="mt-2 text-lg font-semibold text-white">{summary.failedVerification}</div>
        </div>

        <div className={cn(
          'rounded-lg border px-3 py-3',
          summary.reviewRequired > 0 ? 'border-violet-500/30 bg-violet-500/10' : 'border-gray-800 bg-gray-950/50'
        )}>
          <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.2em] text-gray-500">
            <Target className="w-3.5 h-3.5" />
            Review Required Patches
          </div>
          <div className="mt-2 text-lg font-semibold text-white">{summary.reviewRequired}</div>
        </div>
      </div>

      <HydraRacePanel
        providerPanels={providerPanels}
        patchBundles={patchBundles}
        verificationReports={verificationReports}
      />

      {(historicalLearning && historicalLearning.observed_builds > 0) && (
        <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-3">
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="text-[11px] uppercase tracking-[0.2em] text-gray-500">Learning Priors</div>
              <div className="mt-1 text-xs text-gray-400">
                {historicalLearning.observed_builds} observed build{historicalLearning.observed_builds === 1 ? '' : 's'} in {historicalLearning.scope || 'this scope'}
              </div>
            </div>
            <div className="text-xs font-semibold text-cyan-200">{learningSignals.length} signal{learningSignals.length === 1 ? '' : 's'}</div>
          </div>
          {learningSignals.length > 0 && (
            <div className="mt-3 grid gap-2 md:grid-cols-2">
              {learningSignals.map((signal) => (
                <div key={signal} className="break-words rounded-lg border border-cyan-500/20 bg-cyan-500/10 px-3 py-2 text-xs leading-5 text-cyan-100">
                  {signal}
                </div>
              ))}
            </div>
          )}
          {promptProposals.length > 0 && (
            <div className="mt-4 space-y-3">
              <div className="flex items-center justify-between gap-3">
                <div className="text-[11px] uppercase tracking-[0.2em] text-gray-500">Prompt Proposals</div>
                <div className="text-[11px] text-amber-200">Benchmark-gated</div>
              </div>
              {promptProposals.map((proposal) => {
                const reviewState = proposal.review_state || 'proposed'
                const benchmarkStatus = proposal.benchmark_status || (reviewState === 'approved' ? 'not_started' : 'not_applicable')
                const isPending = reviewState === 'proposed'
                const canBenchmark = reviewState === 'approved' && benchmarkStatus !== 'running'
                const isBusy = promptProposalActionId === proposal.id
                return (
                  <div key={proposal.id} className="rounded-lg border border-amber-500/20 bg-amber-500/10 px-3 py-3">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="rounded-md border border-gray-700 bg-black/40 px-2 py-1 text-[11px] text-gray-300">
                        {humanize(proposal.target_prompt)}
                      </span>
                      <span className="rounded-md border border-gray-700 bg-black/40 px-2 py-1 text-[11px] text-gray-300">
                        {humanize(proposal.failure_cluster)}
                      </span>
                      <span className={cn(
                        'rounded-md border px-2 py-1 text-[11px]',
                        reviewState === 'approved'
                          ? 'border-green-500/40 bg-green-500/10 text-green-200'
                          : reviewState === 'rejected'
                            ? 'border-rose-500/40 bg-rose-500/10 text-rose-200'
                            : 'border-amber-500/40 bg-amber-500/10 text-amber-200'
                      )}>
                        {humanize(reviewState)}
                      </span>
                      <span className={cn(
                        'rounded-md border px-2 py-1 text-[11px]',
                        benchmarkStatus === 'passed'
                          ? 'border-green-500/40 bg-green-500/10 text-green-200'
                          : benchmarkStatus === 'failed'
                            ? 'border-rose-500/40 bg-rose-500/10 text-rose-200'
                            : benchmarkStatus === 'running'
                              ? 'border-cyan-500/40 bg-cyan-500/10 text-cyan-200'
                              : 'border-gray-700 bg-black/40 text-gray-300'
                      )}>
                        Benchmark: {humanize(benchmarkStatus)}
                      </span>
                    </div>
                    <div className="mt-2 break-words text-xs leading-5 text-amber-100">{proposal.proposal}</div>
                    {(proposal.evidence || []).slice(0, 3).map((item) => (
                      <div key={item} className="mt-1 break-words text-[11px] leading-4 text-amber-200/80">
                        Evidence: {item}
                      </div>
                    ))}
                    <div className="mt-2 break-words text-[11px] leading-4 text-gray-400">
                      Benchmark gate: {proposal.benchmark_gate}
                    </div>
                    {proposal.review_message && (
                      <div className="mt-2 break-words text-[11px] leading-4 text-gray-300">
                        Review note: {proposal.review_message}
                      </div>
                    )}
                    {(proposal.benchmark_results || []).length > 0 && (
                      <div className="mt-3 space-y-1">
                        {(proposal.benchmark_results || []).slice(0, 4).map((result) => (
                          <div key={result.name} className="break-words text-[11px] leading-4 text-gray-300">
                            {humanize(result.name)}: {humanize(result.status)}
                          </div>
                        ))}
                      </div>
                    )}
                    {(onReviewPromptProposal || onBenchmarkPromptProposal) && (
                      <div className="mt-3 flex flex-wrap gap-2">
                        {onReviewPromptProposal && (
                          <>
                            <button
                              type="button"
                              className="rounded-md border border-green-500/40 bg-green-500/10 px-3 py-1.5 text-xs font-semibold text-green-100 disabled:cursor-not-allowed disabled:opacity-50"
                              disabled={!isPending || isBusy}
                              onClick={() => onReviewPromptProposal(proposal.id, 'approve')}
                            >
                              {isBusy ? 'Saving...' : 'Approve'}
                            </button>
                            <button
                              type="button"
                              className="rounded-md border border-rose-500/40 bg-rose-500/10 px-3 py-1.5 text-xs font-semibold text-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                              disabled={!isPending || isBusy}
                              onClick={() => onReviewPromptProposal(proposal.id, 'reject')}
                            >
                              Reject
                            </button>
                          </>
                        )}
                        {onBenchmarkPromptProposal && (
                          <button
                            type="button"
                            className="rounded-md border border-cyan-500/40 bg-cyan-500/10 px-3 py-1.5 text-xs font-semibold text-cyan-100 disabled:cursor-not-allowed disabled:opacity-50"
                            disabled={!canBenchmark || isBusy}
                            onClick={() => onBenchmarkPromptProposal(proposal.id)}
                          >
                            {isBusy ? 'Saving...' : 'Run benchmark'}
                          </button>
                        )}
                        <div className="flex items-center text-[11px] text-gray-500">
                          Approval records intent; benchmark gates still control prompt source changes.
                        </div>
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          )}
          {adoptionCandidates.length > 0 && (
            <div className="mt-4 rounded-lg border border-green-500/20 bg-green-500/10 px-3 py-3">
              <div className="flex items-center justify-between gap-3">
                <div className="text-[11px] uppercase tracking-[0.2em] text-green-200">Adoption Registry</div>
                <div className="text-[11px] text-gray-400">Prompt source unchanged</div>
              </div>
              <div className="mt-2 space-y-2">
                {adoptionCandidates.map((candidate) => (
                  <div key={candidate.id} className="rounded-md border border-green-500/20 bg-black/30 px-3 py-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="rounded-md border border-green-500/30 bg-green-500/10 px-2 py-1 text-[11px] text-green-100">
                        {humanize(candidate.status)}
                      </span>
                      <span className="rounded-md border border-gray-700 bg-black/40 px-2 py-1 text-[11px] text-gray-300">
                        {humanize(candidate.target_prompt)}
                      </span>
                      <span className="rounded-md border border-gray-700 bg-black/40 px-2 py-1 text-[11px] text-gray-300">
                        Benchmark: {humanize(candidate.benchmark_status)}
                      </span>
                    </div>
                    <div className="mt-2 break-words text-xs leading-5 text-green-100">{candidate.proposal}</div>
                    <div className="mt-1 text-[11px] text-gray-400">
                      Ready for adoption storage only; live prompt generation has not changed.
                    </div>
                  </div>
                ))}
              </div>
              {onCreatePromptPackDraft && (
                <button
                  type="button"
                  className="mt-3 rounded-md border border-green-500/40 bg-green-500/10 px-3 py-1.5 text-xs font-semibold text-green-100 disabled:cursor-not-allowed disabled:opacity-50"
                  disabled={promptProposalActionId === 'prompt-pack-draft'}
                  onClick={onCreatePromptPackDraft}
                >
                  {promptProposalActionId === 'prompt-pack-draft' ? 'Creating draft...' : 'Create prompt-pack draft'}
                </button>
              )}
            </div>
          )}

          {promptPackDrafts.length > 0 && (
            <div className="mt-4 rounded-lg border border-cyan-500/20 bg-cyan-500/10 px-3 py-3">
              <div className="flex items-center justify-between gap-3">
                <div className="text-[11px] uppercase tracking-[0.2em] text-cyan-200">Prompt-Pack Drafts</div>
                <div className="text-[11px] text-gray-400">Inactive</div>
              </div>
              <div className="mt-2 space-y-2">
                {promptPackDrafts.map((draft) => {
                  const activationRequest = activationRequestsByDraft.get(draft.id)
                  const registryVersion = activationRequest ? registryVersionsByRequest.get(activationRequest.id) : undefined
                  const activationBusy = promptProposalActionId === `prompt-pack-activation:${draft.id}`
                  const registryBusy = activationRequest ? promptProposalActionId === `prompt-pack-registry:${activationRequest.id}` : false
                  return (
                    <div key={draft.id} className="rounded-md border border-cyan-500/20 bg-black/30 px-3 py-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="rounded-md border border-cyan-500/30 bg-cyan-500/10 px-2 py-1 text-[11px] text-cyan-100">
                          {draft.version}
                        </span>
                        <span className="rounded-md border border-gray-700 bg-black/40 px-2 py-1 text-[11px] text-gray-300">
                          {humanize(draft.status)}
                        </span>
                        <span className="rounded-md border border-gray-700 bg-black/40 px-2 py-1 text-[11px] text-gray-300">
                          {draft.changes?.length || 0} change{(draft.changes?.length || 0) === 1 ? '' : 's'}
                        </span>
                        {activationRequest && (
                          <span className="rounded-md border border-cyan-500/40 bg-cyan-500/10 px-2 py-1 text-[11px] text-cyan-100">
                            {humanize(activationRequest.status)}
                          </span>
                        )}
                        {registryVersion && (
                          <span className="rounded-md border border-green-500/40 bg-green-500/10 px-2 py-1 text-[11px] text-green-100">
                            {humanize(registryVersion.status)}
                          </span>
                        )}
                      </div>
                      <div className="mt-2 text-[11px] text-gray-400">
                        Inactive draft only; activation is still blocked from live builds.
                      </div>
                      {registryVersion ? (
                        <div className="mt-2 text-[11px] text-green-100">
                          Registry version {registryVersion.version} is stored for this request. Live prompt reads remain disabled.
                        </div>
                      ) : activationRequest ? (
                        <div className="mt-2 text-[11px] text-cyan-100">
                          Admin activation request recorded separately from historical learning.
                        </div>
                      ) : null}
                      {onRequestPromptPackActivation && !activationRequest && (
                        <button
                          type="button"
                          className="mt-3 rounded-md border border-cyan-500/40 bg-cyan-500/10 px-3 py-1.5 text-xs font-semibold text-cyan-100 disabled:cursor-not-allowed disabled:opacity-50"
                          disabled={activationBusy}
                          onClick={() => onRequestPromptPackActivation(draft.id)}
                        >
                          {activationBusy ? 'Requesting activation...' : 'Request admin activation'}
                        </button>
                      )}
                      {onActivatePromptPackRequest && activationRequest && !registryVersion && activationRequest.status === 'pending_admin_activation' && (
                        <button
                          type="button"
                          className="mt-3 rounded-md border border-green-500/40 bg-green-500/10 px-3 py-1.5 text-xs font-semibold text-green-100 disabled:cursor-not-allowed disabled:opacity-50"
                          disabled={registryBusy}
                          onClick={() => onActivatePromptPackRequest(activationRequest.id)}
                        >
                          {registryBusy ? 'Activating registry version...' : 'Activate registry version'}
                        </button>
                      )}
                      {onRollbackPromptPackVersion && registryVersion && !registryVersion.rollback_of_version_id && (
                        <button
                          type="button"
                          className="mt-3 rounded-md border border-rose-500/40 bg-rose-500/10 px-3 py-1.5 text-xs font-semibold text-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                          disabled={promptProposalActionId === `prompt-pack-rollback:${registryVersion.id}`}
                          onClick={() => onRollbackPromptPackVersion(registryVersion.id)}
                        >
                          {promptProposalActionId === `prompt-pack-rollback:${registryVersion.id}` ? 'Rolling back...' : 'Rollback registry version'}
                        </button>
                      )}
                    </div>
                  )
                })}
              </div>
              {promptPackActivationEvents.length > 0 && (
                <div className="mt-2 text-[11px] text-gray-400">
                  {promptPackActivationEvents.length} registry event{promptPackActivationEvents.length === 1 ? '' : 's'} recorded; live prompt generation is unchanged.
                </div>
              )}
            </div>
          )}
        </div>
      )}

      <BuildActivityTimeline
        currentPhase={currentPhase}
        qualityGateStatus={qualityGateStatus}
        qualityGateStage={qualityGateStage}
        aiThoughts={aiThoughts}
        workOrders={workOrders}
        verificationReports={verificationReports}
        patchBundles={patchBundles}
      />

      <div className="rounded-lg border border-gray-800 bg-gray-950/50 px-3 py-2 text-xs text-gray-500 flex items-center gap-2">
        <Activity className="w-3.5 h-3.5 text-cyan-300" />
        Timeline and race entries are derived from live telemetry, work orders, verification reports, patch bundles, and scorecards.
      </div>
    </div>
  )
}
