// BuildScreen — New simplified build screen
// Non-scrolling full-height layout: provider bar → activity feed → chat input → nav strip

import React, { useState, useRef, useCallback, useEffect } from 'react'
import { cn } from '@/lib/utils'
import {
  Pause, Play, RotateCcw, X, Send, FileCode, Terminal,
  Eye, AlertCircle, Cpu, History, ChevronLeft, Download, ExternalLink,
  CheckCircle2, MessageSquare, Copy, Check,
} from 'lucide-react'
import { ProviderStatusBar } from './ProviderStatusBar'
import { LiveActivityFeed } from './LiveActivityFeed'
import { BuildHistory } from './BuildHistory'
import DiffReviewPanel from '@/components/diff/DiffReviewPanel'
import AIRepairReviewPanel from '@/components/ide/AIRepairReviewPanel'
import AITelemetryOverlay from '@/components/ide/AITelemetryOverlay'
import { PanicKillButton } from '@/components/budget/PanicKillButton'
import type {
  BuildLearningSummaryState,
  BuildMessageTargetMode,
  BuildPermissionRequest,
  BuildInteractionState,
  BuildPromptPackActivationEventState,
  BuildPromptPackActivationRequestState,
  BuildPromptPackVersionState,
} from '@/services/api'

// ─── Minimal local types matching AppBuilder.tsx structures ──────────────────

type BuildStatus =
  | 'idle' | 'pending' | 'planning' | 'in_progress' | 'testing'
  | 'reviewing' | 'awaiting_review' | 'completed' | 'failed' | 'cancelled'

interface BuildAgent {
  id: string
  role: string
  provider: string
  model?: string
  status: 'idle' | 'working' | 'completed' | 'error'
  progress: number
  currentTask?: { type: string; description: string }
}

interface BuildCheckpoint {
  id: string
  number: number
  name: string
  description: string
  progress: number
  restorable?: boolean
  createdAt: string
}

interface BuildBlocker {
  id: string
  severity: string
  title: string
  summary?: string
  unblocks_with?: string
  category?: string
  type?: string
  who_must_act?: string
}

interface PlatformReadinessNotice {
  title: string
  body: string
  detail: string
  isCritical: boolean
}

interface BuildPatchBundle {
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

interface BuildWorkOrder {
  id: string
  role: string
  category: string
  task_shape: string
  summary?: string
  preferred_provider?: string
  contract_slice?: {
    surface?: string
    truth_tags?: string[]
  }
}

interface BuildVerificationReport {
  id: string
  phase: string
  surface: string
  status: 'passed' | 'failed' | 'blocked'
  warnings?: string[]
  errors?: string[]
  blockers?: string[]
  truth_tags?: string[]
  confidence_score?: number
  generated_at?: string
}

interface BuildProviderScorecard {
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

interface BuildGuaranteeState {
  status: 'validating' | 'retrying' | 'rolling_back' | 'passed' | 'failed'
  verdict?: 'pass' | 'soft_fail' | 'hard_fail'
  attempts: number
  score?: number
  rolledBack: boolean
  durationMs?: number
  error?: string
  taskId?: string
  taskType?: string
  updatedAt: string
}

interface BSBuildState {
  id: string
  status: BuildStatus
  progress: number
  description: string
  agents: BuildAgent[]
  tasks?: Array<{
    id: string
    type: string
    description: string
    status: 'pending' | 'in_progress' | 'completed' | 'failed' | 'cancelled'
  }>
  checkpoints?: BuildCheckpoint[]
  interaction?: BuildInteractionState
  currentPhase?: string
  blockers?: BuildBlocker[]
  patchBundles?: BuildPatchBundle[]
  powerMode?: 'fast' | 'balanced' | 'max'
  qualityGateStatus?: 'pending' | 'running' | 'passed' | 'failed'
  qualityGateStage?: string
  workOrders?: BuildWorkOrder[]
  verificationReports?: BuildVerificationReport[]
  providerScorecards?: BuildProviderScorecard[]
  historicalLearning?: BuildLearningSummaryState
  guarantee?: BuildGuaranteeState
  promptPackActivationRequests?: BuildPromptPackActivationRequestState[]
  promptPackVersions?: BuildPromptPackVersionState[]
  promptPackActivationEvents?: BuildPromptPackActivationEventState[]
}

interface AIThoughtItem {
  id: string
  agentId?: string
  agentRole?: string
  provider: string
  model?: string
  type: 'thinking' | 'action' | 'output' | 'error'
  content: string
  timestamp: Date
  isInternal?: boolean
  eventType?: string
  taskType?: string
  files?: string[]
  filesCount?: number
  retryCount?: number
  maxRetries?: number
}

interface ChatMsgItem {
  id: string
  role: 'user' | 'lead' | 'system'
  content: string
  timestamp: Date
  kind?: string
  status?: string
}

interface ProviderPanelItem {
  provider: 'claude' | 'gpt4' | 'gemini' | 'grok' | 'ollama'
  liveModelName: string
  available: boolean
  status: 'idle' | 'working' | 'thinking' | 'completed' | 'error' | 'unavailable'
  statusLabel: string
  thoughts: AIThoughtItem[]
  currentTaskLabel?: string
}

type SupportedProvider = ProviderPanelItem['provider']

interface ProviderModelOption {
  id: string
  name: string
}

interface GeneratedFile {
  path: string
  content: string
  language: string
}

type ProposedEdit = any

type OverlayId = 'activity' | 'files' | 'console' | 'issues' | 'detail' | 'history' | null

// ─── Props ───────────────────────────────────────────────────────────────────

interface BuildScreenProps {
  buildState: BSBuildState
  providerPanels: ProviderPanelItem[]
  aiThoughts: AIThoughtItem[]
  chatMessages: ChatMsgItem[]
  generatedFiles: GeneratedFile[]
  proposedEdits: ProposedEdit[]
  isBuildActive: boolean
  buildPaused: boolean
  pendingQuestion: string | undefined | null
  pendingPermissionRequests: BuildPermissionRequest[]
  pendingRevisionRequests: string[]
  buildActionPending: 'pause' | 'resume' | 'restart' | null
  hasBYOK: boolean
  phaseLabel: string
  visibleBlockers: BuildBlocker[]
  platformReadinessNotice: PlatformReadinessNotice | null
  buildFailureAttribution: { title: string; body: string; detail: string; capturedError?: string } | null
  showDiffReview: boolean
  userId: number | null | undefined
  isPreparingPreview: boolean
  isCreatingProject: boolean
  isStartingOver: boolean
  createdProjectId: number | null
  permissionActionId: string | null
  rollbackCheckpointId: string | null
  patchBundleActionId?: string | null
  promptProposalActionId?: string | null
  chatInput: string
  setChatInput: (v: string) => void
  plannerSendMode: BuildMessageTargetMode
  setPlannerSendMode: (m: BuildMessageTargetMode) => void
  plannerMessagePending: boolean
  providerModelOverrides: Record<SupportedProvider, string>
  providerModelOptions: Record<SupportedProvider, ProviderModelOption[]>
  providerModelPendingProvider: SupportedProvider | null
  agentMessageDrafts: Record<string, string>
  agentMessagePendingId: string | null
  onAgentMessageDraftChange: (agentId: string, value: string) => void
  onSendDirectAgentMessage: (agentId: string) => void
  onSendChatMessage: () => void
  onSelectProviderModel: (provider: SupportedProvider, model: string) => void
  onPause: () => void
  onResume: () => void
  onRestart: () => void
  onStartOver: () => void
  onPreviewWorkspace: () => void
  onOpenInIDE: () => void
  onDownload: () => void
  onRollbackCheckpoint: (id: string) => void
  onResolvePermission: (id: string, decision: 'allow' | 'deny', mode: 'once' | 'build') => void
  onApprovePatchBundle: (id: string) => void
  onRejectPatchBundle: (id: string) => void
  onReviewPromptProposal: (id: string, decision: 'approve' | 'reject') => void
  onBenchmarkPromptProposal: (id: string) => void
  onCreatePromptPackDraft: () => void
  onRequestPromptPackActivation?: (id: string) => void
  onActivatePromptPackRequest?: (id: string) => void
  onRollbackPromptPackVersion?: (id: string) => void
  onSetShowDiffReview: (v: boolean) => void
  onLoadProposedEdits: (buildId?: string) => void
  onOpenCompletedBuild: (buildId: string, action?: 'resume' | 'open_files') => void
}

// ─── Helper functions ─────────────────────────────────────────────────────────

const humanize = (s?: string) =>
  (s || '').replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())

const patchBundleNeedsReview = (bundle: BuildPatchBundle): boolean =>
  Boolean(bundle.review_required || bundle.merge_policy === 'review_required')

const patchBundlePendingReview = (bundle: BuildPatchBundle): boolean =>
  patchBundleNeedsReview(bundle) && bundle.review_status !== 'approved' && bundle.review_status !== 'rejected'

const copyTextToClipboard = async (value: string): Promise<boolean> => {
  const text = String(value || '')
  if (!text.trim()) return false

  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // Fall through to the DOM-based copy path when clipboard permissions fail.
    }
  }

  if (typeof document === 'undefined') return false

  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', 'true')
  textarea.style.position = 'fixed'
  textarea.style.top = '0'
  textarea.style.left = '0'
  textarea.style.opacity = '0'
  textarea.style.pointerEvents = 'none'
  document.body.appendChild(textarea)
  textarea.focus()
  textarea.select()
  textarea.setSelectionRange(0, text.length)

  try {
    return document.execCommand('copy')
  } finally {
    document.body.removeChild(textarea)
  }
}

// ─── Build Header ─────────────────────────────────────────────────────────────

interface BuildHeaderProps {
  buildState: BSBuildState
  phaseLabel: string
  isBuildActive: boolean
  buildPaused: boolean
  buildActionPending: 'pause' | 'resume' | 'restart' | null
  onPause: () => void
  onResume: () => void
  onRestart: () => void
  isPreparingPreview: boolean
  onPreviewWorkspace: () => void
  onOpenInIDE: () => void
  onDownload: () => void
  onOpenPlannerConsole: () => void
  createdProjectId: number | null
}

const BuildHeader: React.FC<BuildHeaderProps> = ({
  buildState, phaseLabel, isBuildActive, buildPaused,
  buildActionPending, onPause, onResume, onRestart,
  isPreparingPreview, onPreviewWorkspace, onOpenInIDE, onDownload, onOpenPlannerConsole, createdProjectId,
}) => {
  const { status, progress } = buildState
  const [copyState, setCopyState] = useState<'idle' | 'copied' | 'failed'>('idle')

  const statusColor = status === 'completed' ? 'text-green-400 bg-green-500/10 border-green-500/30'
    : status === 'failed' ? 'text-red-400 bg-red-500/10 border-red-500/30'
    : buildPaused ? 'text-amber-400 bg-amber-500/10 border-amber-500/30'
    : 'text-cyan-400 bg-cyan-500/10 border-cyan-500/30'

  const statusText = status === 'completed' ? 'Completed'
    : status === 'failed' ? 'Failed'
    : buildPaused ? 'Paused'
    : humanize(status)
  const guarantee = buildState.guarantee
  const gateStatusText = guarantee
    ? guarantee.status === 'passed'
      ? `Validation Passed${typeof guarantee.score === 'number' ? ` • ${Math.round(guarantee.score)}%` : ''}`
      : guarantee.status === 'retrying'
        ? `Retrying Validation${guarantee.attempts > 1 ? ` • Attempt ${guarantee.attempts}` : ''}`
        : guarantee.status === 'rolling_back'
          ? 'Rolling Back'
          : guarantee.rolledBack
            ? 'Rolled Back'
            : guarantee.error
              ? 'Validation Failed'
              : 'Validating'
    : buildState.qualityGateStatus === 'passed'
      ? 'Validation Passed'
      : buildState.qualityGateStatus === 'failed'
        ? 'Validation Failed'
        : buildState.qualityGateStatus === 'running'
          ? humanize(buildState.qualityGateStage || 'validation')
          : ''
  const gateStatusClass = guarantee?.status === 'passed' || buildState.qualityGateStatus === 'passed'
    ? 'text-green-300 bg-green-500/10 border-green-500/30'
    : guarantee?.status === 'retrying'
      ? 'text-amber-300 bg-amber-500/10 border-amber-500/30'
      : guarantee?.status === 'rolling_back'
        ? 'text-red-300 bg-red-500/10 border-red-500/30'
        : guarantee?.status === 'failed' || buildState.qualityGateStatus === 'failed'
          ? 'text-red-300 bg-red-500/10 border-red-500/30'
          : buildState.qualityGateStatus === 'running'
            ? 'text-sky-300 bg-sky-500/10 border-sky-500/30'
            : 'text-gray-300 bg-gray-500/10 border-gray-500/20'

  const desc = buildState.description || 'Building your app...'

  useEffect(() => {
    if (copyState === 'idle') return undefined
    const timeoutId = window.setTimeout(() => {
      setCopyState('idle')
    }, 1800)
    return () => window.clearTimeout(timeoutId)
  }, [copyState])

  const handleCopyPrompt = useCallback(async () => {
    const copied = await copyTextToClipboard(desc)
    setCopyState(copied ? 'copied' : 'failed')
  }, [desc])

  return (
    <div
      className="build-screen-panel shrink-0 flex flex-wrap items-center gap-3 px-4 py-4 sm:px-5"
      style={{ minHeight: '84px' }}
    >

      {/* Title + status */}
      <div className="flex-1 min-w-0 flex items-center gap-2 sm:gap-3">
        <div className="min-w-0">
          <div className="text-sm sm:text-[15px] font-semibold text-slate-100 truncate select-text" title={desc}>
            {desc}
          </div>
          <div className="flex flex-wrap items-center gap-2 text-[10px] sm:text-[11px] text-slate-500 font-mono uppercase tracking-[0.18em]">
            <span>{phaseLabel}</span>
            {gateStatusText && (
              <span className={cn('text-[9px] sm:text-[10px] font-bold uppercase tracking-widest px-1.5 py-0.5 rounded border', gateStatusClass)}>
                {gateStatusText}
              </span>
            )}
          </div>
        </div>
        <span className={cn('shrink-0 text-[9px] sm:text-[10px] font-bold uppercase tracking-widest px-1.5 sm:px-2 py-0.5 rounded border', statusColor)}>
          {statusText}
        </span>
      </div>

      {/* Progress bar (mobile: show inline, smaller) */}
      <div className="flex items-center gap-2 w-24 sm:w-44">
        <div className="flex-1 h-2 rounded-full bg-[rgba(184,226,255,0.08)] overflow-hidden">
          <div
            className={cn(
              'h-full rounded-full transition-all duration-500',
              status === 'completed' ? 'bg-green-500'
                : status === 'failed' ? 'bg-red-500'
                : 'bg-gradient-to-r from-sky-400 via-cyan-400 to-emerald-300'
            )}
            style={{ width: `${Math.min(100, progress)}%` }}
          />
        </div>
        <span className="text-[10px] font-mono text-slate-400 w-8 text-right">{progress}%</span>
      </div>

      {/* Build controls */}
      <div className="flex items-center gap-1.5 shrink-0">
        <button
          type="button"
          onClick={handleCopyPrompt}
          aria-label="Copy build prompt"
          title={copyState === 'copied' ? 'Build prompt copied' : 'Copy the full build prompt'}
          className={cn(
            'flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-xs font-semibold transition-colors',
            copyState === 'copied'
              ? 'border-emerald-700/60 bg-emerald-950/40 text-emerald-200'
              : copyState === 'failed'
                ? 'border-amber-700/60 bg-amber-950/30 text-amber-200'
                : 'border-[rgba(184,226,255,0.12)] bg-[rgba(255,255,255,0.02)] text-slate-200 hover:border-[rgba(184,226,255,0.24)] hover:bg-[rgba(56,189,248,0.08)]'
          )}
        >
          {copyState === 'copied' ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
          <span className="hidden md:inline">
            {copyState === 'copied' ? 'Copied' : copyState === 'failed' ? 'Retry Copy' : 'Copy Prompt'}
          </span>
        </button>
        {(isBuildActive || status === 'awaiting_review') && (
          <button
            type="button"
            onClick={onOpenPlannerConsole}
            aria-label="Steer Build"
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-[rgba(56,189,248,0.35)] bg-[rgba(56,189,248,0.08)] text-cyan-200 hover:bg-[rgba(56,189,248,0.14)] text-xs font-semibold uppercase tracking-wide"
          >
            <MessageSquare className="w-3 h-3" />
            Steer
          </button>
        )}
        {status === 'completed' && (
          <>
            <button
              onClick={onPreviewWorkspace}
              disabled={isPreparingPreview}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-green-600 hover:bg-green-500 text-white text-xs font-bold uppercase tracking-wide disabled:opacity-50"
            >
              <Eye className="w-3 h-3" />
              {isPreparingPreview ? '...' : 'Preview'}
            </button>
            <button
              onClick={onOpenInIDE}
            className="flex items-center gap-1 px-2.5 py-1.5 rounded-lg border border-[rgba(184,226,255,0.12)] text-slate-400 hover:text-slate-200 hover:border-[rgba(184,226,255,0.24)] text-xs"
            >
              <ExternalLink className="w-3 h-3" />
              IDE
            </button>
            <button
              onClick={onDownload}
            className="flex items-center gap-1 px-2.5 py-1.5 rounded-lg border border-[rgba(184,226,255,0.12)] text-slate-400 hover:text-slate-200 hover:border-[rgba(184,226,255,0.24)] text-xs"
            >
              <Download className="w-3 h-3" />
            </button>
          </>
        )}
        {status === 'failed' && (
          <button
            onClick={onRestart}
            disabled={buildActionPending !== null}
            aria-label="Restart failed build"
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-cyan-700 hover:bg-cyan-600 text-white text-xs font-bold uppercase tracking-wide disabled:opacity-50"
          >
            <RotateCcw className={cn('w-3 h-3', buildActionPending === 'restart' && 'animate-spin')} />
            Restart
          </button>
        )}
        {isBuildActive && status !== 'awaiting_review' && (
          buildPaused ? (
            <button
              onClick={onResume}
              disabled={buildActionPending !== null}
              className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-green-700 hover:bg-green-600 text-white text-xs font-semibold disabled:opacity-50"
            >
              <Play className="w-3 h-3" />
            </button>
          ) : (
            <button
              onClick={onPause}
              disabled={buildActionPending !== null}
            className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border border-[rgba(184,226,255,0.12)] text-slate-400 hover:text-cyan-200 hover:border-[rgba(56,189,248,0.35)] text-xs disabled:opacity-50"
            >
              <Pause className="w-3 h-3" />
            </button>
          )
        )}
        <PanicKillButton visible={isBuildActive} />
      </div>
    </div>
  )
}

// ─── Chat Input Bar ───────────────────────────────────────────────────────────

interface ChatInputBarProps {
  chatInput: string
  setChatInput: (v: string) => void
  plannerSendMode: BuildMessageTargetMode
  setPlannerSendMode: (m: BuildMessageTargetMode) => void
  onSend: () => void
  pending: boolean
  isBuildActive: boolean
  pendingQuestion: string | undefined | null
  buildPaused: boolean
  inputRef: React.RefObject<HTMLInputElement>
}

const ChatInputBar: React.FC<ChatInputBarProps> = ({
  chatInput, setChatInput, plannerSendMode, setPlannerSendMode,
  onSend, pending, isBuildActive, pendingQuestion, buildPaused, inputRef,
}) => {
  const placeholder = pendingQuestion ||
    (buildPaused
      ? 'Build is paused — steer or resume...'
      : plannerSendMode === 'all_agents' && isBuildActive
        ? 'Broadcast a directive to every active agent...'
        : 'Message the planner...')

  return (
    <div className="build-screen-panel shrink-0 flex items-center gap-1.5 sm:gap-2 px-3 py-3 sm:px-4">
      {/* Mode toggle */}
      <div className="flex rounded-lg overflow-hidden border border-gray-800 shrink-0">
        <button
          onClick={() => setPlannerSendMode('lead')}
          aria-label="Lead Planner"
          className={cn(
            'px-2 py-1.5 text-[10px] font-bold uppercase tracking-wide transition-colors',
            plannerSendMode === 'lead'
              ? 'bg-slate-100 text-slate-950'
              : 'bg-transparent text-slate-500 hover:text-slate-300'
          )}
        >
          Lead
        </button>
        <button
          onClick={() => setPlannerSendMode('all_agents')}
          disabled={!isBuildActive}
          aria-label="All Agents"
          className={cn(
            'px-2 py-1.5 text-[10px] font-bold uppercase tracking-wide transition-colors',
            plannerSendMode === 'all_agents'
              ? 'bg-cyan-500 text-slate-950'
              : 'bg-transparent text-slate-500 hover:text-slate-300 disabled:opacity-30'
          )}
        >
          All
        </button>
      </div>

      <input
        ref={inputRef}
        type="text"
        value={chatInput}
        onChange={(e) => setChatInput(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && !pending && onSend()}
        placeholder={placeholder}
        className="flex-1 min-w-0 bg-[rgba(4,8,14,0.86)] border border-[rgba(184,226,255,0.12)] rounded-xl px-3 sm:px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-[rgba(56,189,248,0.45)] focus:ring-1 focus:ring-cyan-500/25 transition-all"
      />

      <button
        onClick={onSend}
        disabled={!chatInput.trim() || pending}
        className="shrink-0 px-3 sm:px-4 py-2.5 bg-gradient-to-r from-sky-400 via-cyan-400 to-emerald-300 text-slate-950 rounded-xl text-sm font-semibold disabled:opacity-40 flex items-center gap-2"
      >
        {pending ? (
          <span className="inline-flex gap-0.5">
            <span className="w-1 h-1 rounded-full bg-white animate-bounce" style={{ animationDelay: '0ms' }} />
            <span className="w-1 h-1 rounded-full bg-white animate-bounce" style={{ animationDelay: '150ms' }} />
            <span className="w-1 h-1 rounded-full bg-white animate-bounce" style={{ animationDelay: '300ms' }} />
          </span>
        ) : (
          <Send className="w-4 h-4" />
        )}
      </button>
    </div>
  )
}

// ─── Bottom Nav Strip ────────────────────────────────────────────────────────

const BuildCommandSurface: React.FC<{
  buildStatus: BuildStatus
  generatedFilesCount: number
  issueCount: number
  hasBYOK: boolean
  qualityGateStatus?: BSBuildState['qualityGateStatus']
  qualityGateStage?: string
  guarantee?: BuildGuaranteeState
  createdProjectId: number | null
  isPreparingPreview: boolean
  onSelectOverlay: (overlay: OverlayId) => void
  onOpenPreview: () => void
  onOpenInIDE: () => void
  onDownload: () => void
}> = ({
  buildStatus,
  generatedFilesCount,
  issueCount,
  hasBYOK,
  qualityGateStatus,
  qualityGateStage,
  guarantee,
  createdProjectId,
  isPreparingPreview,
  onSelectOverlay,
  onOpenPreview,
  onOpenInIDE,
  onDownload,
}) => {
  const qualityGateSummary = guarantee
    ? guarantee.status === 'passed'
      ? `${Math.round(guarantee.score ?? 100)}% pass`
      : guarantee.status === 'retrying'
        ? `retry ${guarantee.attempts}`
        : guarantee.status === 'rolling_back'
          ? 'rollback'
          : guarantee.rolledBack
            ? 'rolled back'
            : 'failed'
    : qualityGateStatus === 'passed'
      ? 'passed'
      : qualityGateStatus === 'failed'
        ? 'failed'
        : qualityGateStatus === 'running'
          ? (qualityGateStage ? humanize(qualityGateStage) : 'running')
          : 'pending'
  const items: Array<{
    label: string
    value: string
    icon: React.ReactNode
    action: () => void
    disabled?: boolean
  }> = [
    {
      label: 'File stream',
      value: generatedFilesCount ? `${generatedFilesCount} files` : 'streaming',
      icon: <FileCode className="w-4 h-4" />,
      action: () => onSelectOverlay('files'),
    },
    {
      label: 'Agent console',
      value: 'steer agents',
      icon: <Terminal className="w-4 h-4" />,
      action: () => onSelectOverlay('console'),
    },
    {
      label: 'Issues and gates',
      value: issueCount ? `${issueCount} open` : 'clear',
      icon: <AlertCircle className="w-4 h-4" />,
      action: () => onSelectOverlay('issues'),
    },
    {
      label: 'Preview',
      value: isPreparingPreview ? 'preparing' : buildStatus === 'completed' ? 'ready' : 'queued',
      icon: <Eye className="w-4 h-4" />,
      action: onOpenPreview,
      disabled: buildStatus !== 'completed',
    },
    {
      label: 'IDE handoff',
      value: createdProjectId ? `project ${createdProjectId}` : 'create on completion',
      icon: <ExternalLink className="w-4 h-4" />,
      action: onOpenInIDE,
      disabled: !createdProjectId && buildStatus !== 'completed',
    },
    {
      label: 'Export',
      value: 'ZIP/GitHub',
      icon: <Download className="w-4 h-4" />,
      action: onDownload,
      disabled: !generatedFilesCount,
    },
    {
      label: 'BYOK route',
      value: hasBYOK ? 'active' : 'platform keys',
      icon: <Cpu className="w-4 h-4" />,
      action: () => onSelectOverlay('detail'),
    },
    {
      label: 'Review evidence',
      value: qualityGateSummary,
      icon: <CheckCircle2 className="w-4 h-4" />,
      action: () => onSelectOverlay('detail'),
    },
  ]

  return (
    <div className="build-command-surface">
      {items.map((item) => (
        <button
          key={item.label}
          type="button"
          onClick={item.action}
          disabled={item.disabled}
          className="build-command-surface__item"
        >
          <span className="build-command-surface__icon">{item.icon}</span>
          <span>
            <strong>{item.label}</strong>
            <em>{item.value}</em>
          </span>
        </button>
      ))}
    </div>
  )
}

interface BottomNavStripProps {
  activeOverlay: OverlayId
  onSelectOverlay: (id: OverlayId) => void
  onOpenPreview: () => void
  onStartOver: () => void
  isStartingOver: boolean
  isBuildActive: boolean
  generatedFilesCount: number
  issueCount: number
  hasUrgentIssue: boolean
  buildStatus: string
}

const NAV_BUTTONS = [
  { id: 'activity' as const, label: 'Activity', icon: MessageSquare },
  { id: 'files' as const, label: 'Files', icon: FileCode },
  { id: 'console' as const, label: 'Console', icon: Terminal },
  { id: 'issues' as const, label: 'Issues', icon: AlertCircle },
  { id: 'detail' as const, label: 'AI Detail', icon: Cpu },
  { id: 'history' as const, label: 'History', icon: History },
] as const

const BottomNavStrip: React.FC<BottomNavStripProps> = ({
  activeOverlay, onSelectOverlay, onOpenPreview, onStartOver, isStartingOver,
  isBuildActive, generatedFilesCount, issueCount, hasUrgentIssue, buildStatus,
}) => {
  return (
    <div className="build-screen-bottomnav shrink-0 flex items-center gap-0.5 sm:gap-1 px-2 sm:px-3 py-2 overflow-x-auto"
      style={{ WebkitOverflowScrolling: 'touch' }}>
      {/* Preview — special, always direct action */}
      <button
        onClick={onOpenPreview}
        className={cn(
          'flex items-center gap-1 sm:gap-1.5 px-2.5 sm:px-3 py-2 sm:py-1.5 rounded-lg text-xs font-semibold whitespace-nowrap transition-all',
          buildStatus === 'completed'
            ? 'bg-green-600/80 hover:bg-green-500 text-white shadow-[0_0_12px_rgba(34,197,94,0.2)]'
            : 'text-slate-500 hover:text-slate-300 hover:bg-[rgba(255,255,255,0.04)]'
        )}
      >
        <Eye className="w-3.5 h-3.5" />
        <span className="hidden sm:inline">Preview</span>
      </button>

      {/* Other nav buttons */}
      {NAV_BUTTONS.map(({ id, label, icon: Icon }) => {
        const isActive = activeOverlay === id
        const badge = id === 'files' ? generatedFilesCount : id === 'issues' ? issueCount : 0
        const urgent = id === 'issues' && hasUrgentIssue

        return (
          <button
            key={id}
            onClick={() => onSelectOverlay(isActive ? null : id)}
            className={cn(
              'relative flex items-center gap-1 sm:gap-1.5 px-2 sm:px-3 py-2 sm:py-1.5 rounded-lg text-xs font-semibold whitespace-nowrap transition-all',
              isActive
                ? 'bg-[rgba(56,189,248,0.18)] text-cyan-100 border border-[rgba(56,189,248,0.32)]'
                : urgent
                  ? 'text-amber-400 hover:text-amber-300 hover:bg-amber-900/20'
                  : 'text-slate-500 hover:text-slate-300 hover:bg-[rgba(255,255,255,0.04)]'
            )}
          >
            <Icon className="w-3.5 h-3.5" />
            <span className="hidden xs:inline">{label}</span>
            {badge > 0 && (
              <span className={cn(
                'ml-0.5 px-1.5 py-px rounded-full text-[9px] font-bold',
                urgent ? 'bg-amber-500/20 text-amber-300' : 'bg-[rgba(184,226,255,0.08)] text-slate-300'
              )}>
                {badge}
              </span>
            )}
          </button>
        )
      })}

      {/* Spacer */}
      <div className="flex-1" />

      {/* Back to Setup */}
      <button
        onClick={onStartOver}
        disabled={isBuildActive || isStartingOver}
        className="flex items-center gap-1 sm:gap-1.5 px-2 sm:px-3 py-2 sm:py-1.5 rounded-lg text-xs text-slate-500 hover:text-slate-300 hover:bg-[rgba(255,255,255,0.04)] whitespace-nowrap disabled:opacity-30 disabled:cursor-not-allowed transition-all"
      >
        <ChevronLeft className="w-3.5 h-3.5" />
        <span className="hidden sm:inline">Back to Setup</span>
      </button>
    </div>
  )
}

// ─── Overlay Panels ───────────────────────────────────────────────────────────

interface PanelOverlayProps {
  overlay: NonNullable<OverlayId>
  onClose: () => void
  // Data
  buildState: BSBuildState
  generatedFiles: GeneratedFile[]
  aiThoughts: AIThoughtItem[]
  chatMessages: ChatMsgItem[]
  isBuildActive: boolean
  providerPanels: ProviderPanelItem[]
  visibleBlockers: BuildBlocker[]
  platformReadinessNotice: PlatformReadinessNotice | null
  agentMessageDrafts: Record<string, string>
  agentMessagePendingId: string | null
  pendingPermissionRequests: BuildPermissionRequest[]
  pendingRevisionRequests: string[]
  buildFailureAttribution: { title: string; body: string; detail: string; capturedError?: string } | null
  proposedEdits: ProposedEdit[]
  showDiffReview: boolean
  permissionActionId: string | null
  rollbackCheckpointId: string | null
  patchBundleActionId?: string | null
  promptProposalActionId?: string | null
  userId: number | null | undefined
  // Callbacks
  onAgentMessageDraftChange: (agentId: string, value: string) => void
  onSendDirectAgentMessage: (agentId: string) => void
  onResolvePermission: (id: string, decision: 'allow' | 'deny', mode: 'once' | 'build') => void
  onApprovePatchBundle: (id: string) => void
  onRejectPatchBundle: (id: string) => void
  onReviewPromptProposal: (id: string, decision: 'approve' | 'reject') => void
  onBenchmarkPromptProposal: (id: string) => void
  onCreatePromptPackDraft: () => void
  onRequestPromptPackActivation?: (id: string) => void
  onActivatePromptPackRequest?: (id: string) => void
  onRollbackPromptPackVersion?: (id: string) => void
  onSetShowDiffReview: (v: boolean) => void
  onLoadProposedEdits: (buildId?: string) => void
  onRollbackCheckpoint: (id: string) => void
  onOpenCompletedBuild: (buildId: string, action?: 'resume' | 'open_files') => void
}

const OVERLAY_TITLES: Record<NonNullable<OverlayId>, string> = {
  activity: 'Activity',
  files: 'Generated Files',
  console: 'Planner Console',
  issues: 'Issues & Actions',
  detail: 'AI Providers — Live Detail',
  history: 'Build History',
}

const PanelOverlay: React.FC<PanelOverlayProps> = ({
  overlay, onClose,
  buildState, generatedFiles, aiThoughts, chatMessages, isBuildActive,
  providerPanels, visibleBlockers, platformReadinessNotice, agentMessageDrafts, agentMessagePendingId, pendingPermissionRequests,
  pendingRevisionRequests, buildFailureAttribution,
  proposedEdits, showDiffReview, permissionActionId, rollbackCheckpointId,
  patchBundleActionId, promptProposalActionId,
  userId, onAgentMessageDraftChange, onSendDirectAgentMessage, onResolvePermission, onSetShowDiffReview, onLoadProposedEdits,
  onApprovePatchBundle, onRejectPatchBundle, onReviewPromptProposal, onBenchmarkPromptProposal, onCreatePromptPackDraft, onRequestPromptPackActivation, onActivatePromptPackRequest, onRollbackPromptPackVersion,
  onRollbackCheckpoint, onOpenCompletedBuild,
}) => {
  const [selectedFile, setSelectedFile] = useState<GeneratedFile | null>(null)
  const liveAgents = (buildState.agents || []).filter((agent) => agent.status === 'working')
  const liveTasks = (buildState.tasks || []).filter((task) => task.status === 'in_progress')
  const reviewRequiredPatchBundles = React.useMemo(
    () => (buildState.patchBundles || []).filter(patchBundlePendingReview),
    [buildState.patchBundles]
  )
  const pendingProposedEditCount = React.useMemo(
    () => proposedEdits.filter((edit) => edit?.status === 'pending').length,
    [proposedEdits]
  )
  const canOpenProposedEditReview = buildState.status === 'awaiting_review' && pendingProposedEditCount > 0

  // Group files by root folder
  const fileGroups = React.useMemo(() => {
    const groups = new Map<string, GeneratedFile[]>()
    for (const f of generatedFiles) {
      const root = f.path.includes('/') ? f.path.split('/')[0] : '/'
      const arr = groups.get(root) || []
      arr.push(f)
      groups.set(root, arr)
    }
    return Array.from(groups.entries())
  }, [generatedFiles])

  return (
    <div
      className="absolute inset-0 z-50 flex flex-col bg-black/97 backdrop-blur-sm"
      style={{
        paddingTop: 'env(safe-area-inset-top, 0px)',
        paddingBottom: 'env(safe-area-inset-bottom, 0px)',
      }}
    >
      {/* Panel header */}
      <div className="shrink-0 flex items-center justify-between gap-3 px-4 py-3 border-b border-gray-800">
        <h2 className="min-w-0 text-sm font-bold text-gray-200 uppercase tracking-widest break-words">
          {OVERLAY_TITLES[overlay]}
        </h2>
        <button
          onClick={onClose}
          className="shrink-0 p-2 rounded-lg text-gray-500 hover:text-gray-200 hover:bg-gray-800 transition-colors"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Panel content */}
      <div className="flex-1 min-h-0 overflow-y-auto p-3 sm:p-4 overscroll-contain"
        style={{ WebkitOverflowScrolling: 'touch', touchAction: 'pan-y', overscrollBehavior: 'contain' }}>

        {/* ── ACTIVITY ───────────────────────────────────────── */}
        {overlay === 'activity' && (
          <div className="space-y-6">
            <div>
              <h3 className="text-xs font-bold uppercase tracking-widest text-gray-500 mb-3">AI Agents Working</h3>
              {liveAgents.length === 0 ? (
                <div className="rounded-xl border border-gray-800 bg-gray-950/40 p-4 text-sm text-gray-500">
                  No agents are actively working right now.
                </div>
              ) : (
                <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                  {liveAgents.map((agent) => {
                    const messageDraft = agentMessageDrafts[agent.id] || ''
                    const sendPending = agentMessagePendingId === agent.id

                    return (
                      <div key={agent.id} className="rounded-xl border border-gray-800 bg-gray-950/50 p-4">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <div className="text-sm font-semibold text-white break-words">{humanize(agent.role)}</div>
                            <div className="mt-1 text-xs text-gray-500 font-mono break-all">{agent.model || 'Model unavailable'}</div>
                          </div>
                          <span className="shrink-0 text-[9px] font-bold uppercase tracking-widest px-1.5 py-0.5 rounded border border-green-500/30 text-green-300 bg-green-500/10">
                            Working
                          </span>
                        </div>
                        {agent.currentTask?.description && (
                          <div className="mt-3 text-sm text-gray-300 break-words">{agent.currentTask.description}</div>
                        )}
                        <div className="mt-4 rounded-xl border border-gray-800 bg-black/35 px-3 py-3">
                          <div className="flex flex-wrap items-center justify-between gap-3">
                            <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-gray-500">
                              <MessageSquare className="w-3.5 h-3.5" />
                              Direct Control
                            </div>
                            <span className="text-[10px] border rounded px-2 py-0.5 border-cyan-500/40 bg-cyan-500/10 text-cyan-200">
                              Live
                            </span>
                          </div>
                          <div className="mt-2 text-xs text-gray-400">
                            Send an instruction straight to this agent. It stays visible in the planner timeline and this agent&apos;s telemetry.
                          </div>
                          <div className="mt-3 flex flex-col gap-2 sm:flex-row">
                            <input
                              type="text"
                              value={messageDraft}
                              onChange={(event) => onAgentMessageDraftChange(agent.id, event.target.value)}
                              onKeyDown={(event) => {
                                if (event.key === 'Enter' && !sendPending) {
                                  onSendDirectAgentMessage(agent.id)
                                }
                              }}
                              placeholder={`Message ${humanize(agent.role)} directly...`}
                              disabled={!isBuildActive || sendPending}
                              className="flex-1 min-w-0 rounded-lg border border-gray-700 bg-black px-3 py-2 text-sm text-white placeholder:text-gray-600 focus:border-cyan-500 focus:outline-none focus:ring-2 focus:ring-cyan-900/30 disabled:cursor-not-allowed disabled:opacity-50"
                            />
                            <button
                              type="button"
                              onClick={() => onSendDirectAgentMessage(agent.id)}
                              disabled={!isBuildActive || !messageDraft.trim() || sendPending}
                              aria-label={`Send message to ${humanize(agent.role)}`}
                              className="px-3 py-2 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-white disabled:opacity-40 sm:self-stretch"
                            >
                              <Send className="w-4 h-4" />
                            </button>
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>

            <div>
              <h3 className="text-xs font-bold uppercase tracking-widest text-gray-500 mb-3">Live Tasks</h3>
              {liveTasks.length === 0 ? (
                <div className="rounded-xl border border-gray-800 bg-gray-950/40 p-4 text-sm text-gray-500">
                  No active tasks right now.
                </div>
              ) : (
                <div className="space-y-2">
                  {liveTasks.map((task) => (
                    <div key={task.id} className="rounded-xl border border-gray-800 bg-gray-950/50 px-4 py-3">
                      <div className="text-sm font-semibold text-white break-words">{task.description}</div>
                      <div className="mt-1 text-xs text-gray-500 uppercase tracking-widest">{humanize(task.type)}</div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        {/* ── FILES ──────────────────────────────────────────── */}
        {overlay === 'files' && (
          generatedFiles.length === 0 ? (
            <div className="flex items-center justify-center h-full text-gray-600 text-sm">
              No files generated yet
            </div>
          ) : (
            <div className="flex h-full flex-col gap-4 md:flex-row">
              {/* File tree */}
              <div className="w-full shrink-0 overflow-y-auto space-y-3 md:w-64 md:max-h-none max-h-56 rounded-xl border border-gray-800 bg-gray-950/35 p-3">
                {fileGroups.map(([root, files]) => (
                  <div key={root}>
                    <div className="text-[10px] font-bold uppercase tracking-widest text-gray-600 mb-1 px-2">
                      {root === '/' ? 'root' : root}/
                    </div>
                    <div className="space-y-px">
                      {files.map((f) => (
                        <button
                          key={f.path}
                          onClick={() => setSelectedFile(f)}
                          className={cn(
                            'w-full text-left px-2 py-1.5 rounded text-xs transition-colors break-all',
                            selectedFile?.path === f.path
                              ? 'bg-red-700/30 text-red-300'
                              : 'text-gray-400 hover:text-gray-200 hover:bg-gray-900'
                          )}
                        >
                          <FileCode className="w-3 h-3 inline mr-1.5 opacity-60 align-text-bottom" />
                          {f.path}
                        </button>
                      ))}
                    </div>
                  </div>
                ))}
              </div>

              {/* File content preview */}
              <div className="flex-1 min-w-0 min-h-[18rem] rounded-xl border border-gray-800 bg-gray-950 overflow-auto">
                {selectedFile ? (
                  <pre className="p-4 text-xs font-mono text-gray-300 whitespace-pre-wrap break-all">
                    <div className="text-gray-600 mb-3 text-[10px] uppercase tracking-widest break-all">{selectedFile.path}</div>
                    {selectedFile.content}
                  </pre>
                ) : (
                  <div className="flex items-center justify-center h-full min-h-[18rem] text-gray-600 text-sm px-4 text-center">
                    Select a file to preview
                  </div>
                )}
              </div>
            </div>
          )
        )}

        {/* ── CONSOLE ────────────────────────────────────────── */}
        {overlay === 'console' && (
          <div className="rounded-xl border border-gray-800 bg-black/90 p-4 font-mono text-sm min-h-full">
            <div className="flex items-center gap-2 mb-4 pb-3 border-b border-gray-800">
              <div className="w-3 h-3 rounded-full bg-red-500" />
              <div className="w-3 h-3 rounded-full bg-yellow-500" />
              <div className="w-3 h-3 rounded-full bg-green-500" />
              <span className="ml-2 text-gray-600 text-[10px] uppercase tracking-widest">APEX Build Console</span>
            </div>
            <div className="space-y-2">
              {chatMessages.length === 0 ? (
                <div className="text-gray-600">No messages yet</div>
              ) : (
                chatMessages.map((msg) => (
                  <div
                    key={msg.id}
                    className={cn(
                      'flex items-start gap-2',
                      msg.role === 'system' && 'text-gray-400',
                      msg.role === 'lead' && 'text-orange-400',
                      msg.role === 'user' && 'text-cyan-400',
                    )}
                  >
                    <span className="text-red-600 font-bold select-none">{'>'}</span>
                    <span className="flex-1 break-words">
                      <span className="mr-2 text-[10px] uppercase tracking-widest text-gray-600">
                        [{msg.role === 'user' ? 'You' : msg.role === 'lead' ? 'Lead' : 'System'}]
                      </span>
                      {msg.content}
                    </span>
                    <span className="text-gray-700 text-[10px] shrink-0">
                      {msg.timestamp instanceof Date ? msg.timestamp.toLocaleTimeString() : ''}
                    </span>
                  </div>
                ))
              )}
              {isBuildActive && (
                <div className="flex items-center gap-2 text-red-500">
                  <span className="font-bold">{'>'}</span>
                  <span className="w-2 h-4 bg-red-500 animate-pulse" />
                </div>
              )}
            </div>
          </div>
        )}

        {/* ── ISSUES ─────────────────────────────────────────── */}
        {overlay === 'issues' && (
          <div className="space-y-4">
            {/* Platform readiness notice */}
            {platformReadinessNotice && (
              <div
                className={cn(
                  'rounded-xl border p-4',
                  platformReadinessNotice.isCritical
                    ? 'border-amber-700/40 bg-amber-950/20'
                    : 'border-sky-700/35 bg-sky-950/15'
                )}
              >
                <div className="flex items-center gap-2 mb-3">
                  <AlertCircle
                    className={cn(
                      'w-5 h-5',
                      platformReadinessNotice.isCritical ? 'text-amber-400' : 'text-sky-400'
                    )}
                  />
                  <h3
                    className={cn(
                      'font-semibold',
                      platformReadinessNotice.isCritical ? 'text-amber-300' : 'text-sky-300'
                    )}
                  >
                    {platformReadinessNotice.title}
                  </h3>
                </div>
                <p className="text-sm text-gray-300 mb-2">{platformReadinessNotice.body}</p>
                <p className="text-xs text-gray-500">{platformReadinessNotice.detail}</p>
              </div>
            )}

            {/* Build failure attribution */}
            {buildFailureAttribution && (
              <div className="rounded-xl border border-amber-700/40 bg-amber-950/20 p-4">
                <div className="flex items-center gap-2 mb-3">
                  <AlertCircle className="w-5 h-5 text-amber-400" />
                  <h3 className="font-semibold text-amber-300">{buildFailureAttribution.title}</h3>
                </div>
                <p className="text-sm text-gray-300 mb-2">{buildFailureAttribution.body}</p>
                <p className="text-xs text-gray-500">{buildFailureAttribution.detail}</p>
                {buildFailureAttribution.capturedError && (
                  <pre className="mt-3 text-xs text-red-400 font-mono bg-black/40 rounded p-2 overflow-auto">
                    {buildFailureAttribution.capturedError}
                  </pre>
                )}
              </div>
            )}

            {/* Repair patch review */}
            <AIRepairReviewPanel
              bundles={reviewRequiredPatchBundles}
              proposedEditsCount={pendingProposedEditCount}
              onOpenProposedEdits={canOpenProposedEditReview ? () => onSetShowDiffReview(true) : undefined}
              onApproveBundle={onApprovePatchBundle}
              onRejectBundle={onRejectPatchBundle}
              reviewActionId={patchBundleActionId}
            />

            {/* Blockers */}
            {visibleBlockers.length > 0 && (
              <div>
                <h3 className="text-xs font-bold uppercase tracking-widest text-gray-500 mb-2">Blockers</h3>
                <div className="space-y-2">
                  {visibleBlockers.map((blocker) => (
                    <div
                      key={blocker.id}
                      className={cn(
                        'rounded-xl border px-4 py-3',
                        blocker.severity === 'blocking'
                          ? 'border-red-500/30 bg-red-950/20'
                          : 'border-amber-500/25 bg-amber-950/15'
                      )}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <div className="text-sm font-semibold text-white break-words">{blocker.title}</div>
                          {blocker.summary && (
                            <div className="mt-1 text-sm text-gray-400 break-words">{blocker.summary}</div>
                          )}
                        </div>
                        <span className={cn(
                          'text-[9px] font-bold uppercase tracking-widest px-1.5 py-0.5 rounded border shrink-0',
                          blocker.severity === 'blocking'
                            ? 'border-red-500/30 text-red-400 bg-red-500/10'
                            : 'border-amber-500/30 text-amber-400 bg-amber-500/10'
                        )}>
                          {blocker.severity === 'blocking' ? 'Blocking' : 'Warning'}
                        </span>
                      </div>
                      {blocker.unblocks_with && (
                        <div className="mt-2 text-xs text-gray-300 bg-black/20 rounded px-2 py-1.5 break-words">
                          <span className="text-gray-500">Unblock: </span>
                          {blocker.unblocks_with}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Pending revisions */}
            {pendingRevisionRequests.length > 0 && (
              <div>
                <h3 className="text-xs font-bold uppercase tracking-widest text-gray-500 mb-2">
                  Pending Revisions ({pendingRevisionRequests.length})
                </h3>
                <div className="space-y-1">
                  {pendingRevisionRequests.map((rev, i) => (
                    <div key={i} className="text-sm text-gray-300 bg-gray-900/60 rounded px-3 py-2 border border-gray-800 break-words">
                      {rev}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Permission requests */}
            {pendingPermissionRequests.length > 0 && (
              <div>
                <h3 className="text-xs font-bold uppercase tracking-widest text-gray-500 mb-2">
                  Permission Requests ({pendingPermissionRequests.length})
                </h3>
                <div className="space-y-2">
                  {pendingPermissionRequests.map((req) => (
                    <div key={req.id} className="rounded-xl border border-violet-500/25 bg-violet-950/15 p-4">
                      <div className="text-sm font-semibold text-white mb-1 break-words">
                        {humanize(req.scope)}: {req.target}
                      </div>
                      <div className="text-xs text-gray-400 mb-3 break-words">{req.reason}</div>
                      {req.command_preview && (
                        <pre className="text-xs font-mono text-gray-500 bg-black/40 rounded px-2 py-1 mb-3 overflow-auto">
                          {req.command_preview}
                        </pre>
                      )}
                      <div className="flex flex-wrap gap-2">
                        <button
                          onClick={() => onResolvePermission(req.id, 'allow', 'once')}
                          disabled={permissionActionId === req.id}
                          className="text-xs px-3 py-1.5 rounded-lg bg-green-700/40 border border-green-500/30 text-green-300 hover:bg-green-700/60 font-semibold disabled:opacity-40"
                        >
                          Allow Once
                        </button>
                        <button
                          onClick={() => onResolvePermission(req.id, 'allow', 'build')}
                          disabled={permissionActionId === req.id}
                          className="text-xs px-3 py-1.5 rounded-lg bg-green-700/25 border border-green-500/20 text-green-400 hover:bg-green-700/40 font-semibold disabled:opacity-40"
                        >
                          Always Allow
                        </button>
                        <button
                          onClick={() => onResolvePermission(req.id, 'deny', 'once')}
                          disabled={permissionActionId === req.id}
                          className="text-xs px-3 py-1.5 rounded-lg bg-red-700/30 border border-red-500/20 text-red-400 hover:bg-red-700/50 font-semibold disabled:opacity-40"
                        >
                          Deny
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {(buildState.guarantee || (buildState.qualityGateStatus && buildState.qualityGateStatus !== 'pending')) && (
              <div>
                <h3 className="text-xs font-bold uppercase tracking-widest text-gray-500 mb-2">
                  Validation & Guarantee
                </h3>
                <div className="rounded-xl border border-gray-800 bg-gray-900/40 p-4">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className={cn(
                      'text-[10px] font-bold uppercase tracking-widest px-2 py-1 rounded border',
                      buildState.guarantee?.status === 'passed' || buildState.qualityGateStatus === 'passed'
                        ? 'border-green-500/30 bg-green-500/10 text-green-300'
                        : buildState.guarantee?.status === 'retrying'
                          ? 'border-amber-500/30 bg-amber-500/10 text-amber-300'
                          : buildState.guarantee?.status === 'rolling_back'
                            ? 'border-red-500/30 bg-red-500/10 text-red-300'
                            : buildState.guarantee?.status === 'failed' || buildState.qualityGateStatus === 'failed'
                              ? 'border-red-500/30 bg-red-500/10 text-red-300'
                              : 'border-sky-500/30 bg-sky-500/10 text-sky-300'
                    )}>
                      {buildState.guarantee?.status === 'passed' ? 'Passed'
                        : buildState.guarantee?.status === 'retrying' ? `Retrying • Attempt ${buildState.guarantee.attempts}`
                          : buildState.guarantee?.status === 'rolling_back' ? 'Rolling Back'
                            : buildState.guarantee?.rolledBack ? 'Rolled Back'
                              : buildState.guarantee?.status === 'failed' ? 'Failed'
                                : buildState.qualityGateStatus === 'passed' ? 'Passed'
                                  : buildState.qualityGateStatus === 'failed' ? 'Failed'
                                    : humanize(buildState.qualityGateStage || 'validation')}
                    </span>
                    {typeof buildState.guarantee?.score === 'number' && (
                      <span className="text-xs font-mono text-gray-400">
                        Score {Math.round(buildState.guarantee.score)}%
                      </span>
                    )}
                    {typeof buildState.guarantee?.durationMs === 'number' && (
                      <span className="text-xs font-mono text-gray-500">
                        {buildState.guarantee.durationMs}ms
                      </span>
                    )}
                  </div>
                  <div className="mt-2 text-sm text-gray-300 break-words">
                    {buildState.guarantee?.error
                      ? buildState.guarantee.error
                      : buildState.guarantee?.taskType
                        ? `Latest guarantee result came from ${humanize(buildState.guarantee.taskType)}.`
                        : buildState.qualityGateStage
                          ? `Current quality gate stage: ${humanize(buildState.qualityGateStage)}.`
                          : 'Validation telemetry is active for this build.'}
                  </div>
                </div>
              </div>
            )}

            {/* Diff Review Panel */}
            {buildState.status === 'awaiting_review' && proposedEdits.length > 0 && (
              <div>
                <div className="flex items-center justify-between mb-2 gap-3">
                  <h3 className="text-xs font-bold uppercase tracking-widest text-gray-500">
                    Code Review — {proposedEdits.length} Change{proposedEdits.length !== 1 ? 's' : ''}
                  </h3>
                  {!showDiffReview && (
                    <button
                      onClick={() => onSetShowDiffReview(true)}
                      className="text-xs text-sky-400 hover:text-sky-300 shrink-0"
                    >
                      Show changes
                    </button>
                  )}
                </div>
                {showDiffReview && (
                  <div className="rounded-xl border border-yellow-700/40 bg-black/60 overflow-hidden">
                    <DiffReviewPanel
                      buildId={buildState.id}
                      edits={proposedEdits}
                      onEditsUpdated={() => onLoadProposedEdits(buildState.id)}
                      onClose={() => onSetShowDiffReview(false)}
                    />
                  </div>
                )}
              </div>
            )}

            {/* Checkpoints */}
            {(buildState.checkpoints || []).length > 0 && (
              <div>
                <h3 className="text-xs font-bold uppercase tracking-widest text-gray-500 mb-2">Checkpoints</h3>
                <div className="space-y-2">
                  {(buildState.checkpoints || []).map((cp) => (
                    <div key={cp.id} className="flex flex-col gap-3 rounded-xl border border-gray-800 bg-gray-900/40 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
                      <div className="min-w-0">
                        <div className="text-sm font-semibold text-gray-200 break-words">{cp.name}</div>
                        <div className="text-xs text-gray-500 break-words">{cp.description} — {cp.progress}%</div>
                      </div>
                      <button
                        onClick={() => onRollbackCheckpoint(cp.id)}
                        disabled={rollbackCheckpointId === cp.id || cp.restorable === false}
                        className="w-full sm:w-auto text-xs px-2.5 py-1.5 rounded-lg border border-gray-700 text-gray-400 hover:text-white hover:border-gray-600 disabled:opacity-30 flex items-center justify-center gap-1"
                      >
                        <RotateCcw className={cn('w-3 h-3', rollbackCheckpointId === cp.id && 'animate-spin')} />
                        Rollback
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Empty state */}
            {!platformReadinessNotice &&
              !buildFailureAttribution && visibleBlockers.length === 0 &&
              reviewRequiredPatchBundles.length === 0 &&
              pendingPermissionRequests.length === 0 && pendingRevisionRequests.length === 0 &&
              buildState.status !== 'awaiting_review' &&
              (buildState.checkpoints || []).length === 0 && (
              <div className="flex items-center gap-3 text-sm text-gray-500 bg-gray-900/40 rounded-xl p-4 border border-gray-800">
                <CheckCircle2 className="w-5 h-5 text-green-500/60" />
                No open issues right now
              </div>
            )}
          </div>
        )}

        {/* ── AI DETAIL ─────────────────────────────────────── */}
        {overlay === 'detail' && (
          <AITelemetryOverlay
            buildStatus={buildState.status}
            currentPhase={buildState.currentPhase}
            qualityGateStatus={buildState.qualityGateStatus}
            qualityGateStage={buildState.qualityGateStage}
            aiThoughts={aiThoughts}
            providerPanels={providerPanels}
            workOrders={buildState.workOrders}
            verificationReports={buildState.verificationReports}
            patchBundles={buildState.patchBundles}
            providerScorecards={buildState.providerScorecards}
            historicalLearning={buildState.historicalLearning}
            promptPackActivationRequests={buildState.promptPackActivationRequests}
            promptPackVersions={buildState.promptPackVersions}
            promptPackActivationEvents={buildState.promptPackActivationEvents}
            promptProposalActionId={promptProposalActionId}
            onReviewPromptProposal={onReviewPromptProposal}
            onBenchmarkPromptProposal={onBenchmarkPromptProposal}
            onCreatePromptPackDraft={onCreatePromptPackDraft}
            onRequestPromptPackActivation={onRequestPromptPackActivation}
            onActivatePromptPackRequest={onActivatePromptPackRequest}
            onRollbackPromptPackVersion={onRollbackPromptPackVersion}
          />
        )}

        {/* ── HISTORY ───────────────────────────────────────── */}
        {overlay === 'history' && (
          <BuildHistory
            userId={userId}
            onOpenBuild={onOpenCompletedBuild}
          />
        )}
      </div>
    </div>
  )
}

// ─── Main BuildScreen Component ───────────────────────────────────────────────

export const BuildScreen: React.FC<BuildScreenProps> = (props) => {
  const {
    buildState, providerPanels, aiThoughts, chatMessages,
    generatedFiles, proposedEdits, isBuildActive, buildPaused,
    pendingQuestion, pendingPermissionRequests, pendingRevisionRequests,
    buildActionPending, hasBYOK, phaseLabel, visibleBlockers,
    platformReadinessNotice, buildFailureAttribution, showDiffReview, userId,
    isPreparingPreview, isCreatingProject, isStartingOver, createdProjectId,
    permissionActionId, rollbackCheckpointId, patchBundleActionId, promptProposalActionId, chatInput, setChatInput,
    plannerSendMode, setPlannerSendMode, plannerMessagePending, providerModelOverrides, providerModelOptions, providerModelPendingProvider,
    agentMessageDrafts, agentMessagePendingId, onAgentMessageDraftChange, onSendDirectAgentMessage,
    onSendChatMessage, onSelectProviderModel, onPause, onResume, onRestart, onStartOver,
    onPreviewWorkspace, onOpenInIDE, onDownload, onRollbackCheckpoint,
    onResolvePermission, onApprovePatchBundle, onRejectPatchBundle, onReviewPromptProposal, onBenchmarkPromptProposal, onCreatePromptPackDraft, onRequestPromptPackActivation, onActivatePromptPackRequest, onRollbackPromptPackVersion, onSetShowDiffReview, onLoadProposedEdits,
    onOpenCompletedBuild,
  } = props

  const [activeOverlay, setActiveOverlay] = useState<OverlayId>(null)
  const chatInputRef = useRef<HTMLInputElement>(null)

  const focusChatInput = useCallback(() => {
    chatInputRef.current?.focus()
  }, [])

  const openIssuesOverlay = useCallback(() => {
    setActiveOverlay('issues')
  }, [])
  const openPlannerConsole = useCallback(() => {
    setPlannerSendMode('lead')
    setActiveOverlay('console')
    requestAnimationFrame(() => {
      chatInputRef.current?.focus()
    })
  }, [setPlannerSendMode])

  const buildCompleted = buildState.status === 'completed'
  const hasUrgentIssue = Boolean(
    pendingQuestion ||
    buildPaused ||
    pendingPermissionRequests.length > 0 ||
    (buildState.patchBundles || []).some(patchBundlePendingReview) ||
    platformReadinessNotice?.isCritical ||
    buildFailureAttribution ||
    buildState.status === 'awaiting_review' ||
    visibleBlockers.some((b) => b.severity === 'blocking')
  )
  const issueCount = visibleBlockers.length + pendingPermissionRequests.length +
    (buildState.patchBundles || []).filter(patchBundlePendingReview).length +
    (platformReadinessNotice ? 1 : 0) +
    (buildFailureAttribution ? 1 : 0) +
    (buildState.status === 'awaiting_review' ? 1 : 0)

  return (
    <div className="build-screen-shell flex-1 min-h-0 flex flex-col overflow-hidden text-white">
      <h1 className="sr-only">Build Flow</h1>
      <div className="flex-1 min-h-0 flex flex-col gap-3 px-3 pb-3 pt-2 sm:px-4 sm:pb-4 lg:px-5 lg:pb-5">
        {/* Row 1: Build Header */}
        <BuildHeader
          buildState={buildState}
          phaseLabel={phaseLabel}
          isBuildActive={isBuildActive}
          buildPaused={buildPaused}
          buildActionPending={buildActionPending}
          onPause={onPause}
          onResume={onResume}
          onRestart={onRestart}
          isPreparingPreview={isPreparingPreview}
          onPreviewWorkspace={onPreviewWorkspace}
          onOpenInIDE={onOpenInIDE}
          onDownload={onDownload}
          onOpenPlannerConsole={openPlannerConsole}
          createdProjectId={createdProjectId}
        />

        {/* Row 2: Provider Status Bar */}
        <ProviderStatusBar
          providerPanels={providerPanels}
          hasBYOK={hasBYOK}
          isBuildActive={isBuildActive}
          selectedModels={providerModelOverrides}
          modelOptions={providerModelOptions}
          modelUpdatePendingProvider={providerModelPendingProvider}
          onModelSelect={onSelectProviderModel}
        />

        {/* Row 3: Production command surface */}
        <BuildCommandSurface
          buildStatus={buildState.status}
          generatedFilesCount={generatedFiles.length}
          issueCount={issueCount}
          hasBYOK={hasBYOK}
          qualityGateStatus={buildState.qualityGateStatus}
          qualityGateStage={buildState.qualityGateStage}
          guarantee={buildState.guarantee}
          createdProjectId={createdProjectId}
          isPreparingPreview={isPreparingPreview}
          onSelectOverlay={setActiveOverlay}
          onOpenPreview={onPreviewWorkspace}
          onOpenInIDE={onOpenInIDE}
          onDownload={onDownload}
        />

        {/* Row 4: Live Activity Feed (flex-1) */}
        <div className="build-screen-feed-shell relative flex-1 min-h-0 overflow-hidden">
          <LiveActivityFeed
            aiThoughts={aiThoughts}
            chatMessages={chatMessages}
            buildStatus={buildState.status}
            interaction={buildState.interaction}
            isBuildActive={isBuildActive}
            pendingQuestion={pendingQuestion}
            pendingPermissionRequests={pendingPermissionRequests}
            buildPaused={buildPaused}
            onFocusChatInput={focusChatInput}
            onOpenIssues={openIssuesOverlay}
            onResume={onResume}
            buildCompleted={buildCompleted}
            onOpenPreview={onPreviewWorkspace}
            isPreparingPreview={isPreparingPreview}
          />
        </div>

        {/* Row 5: Chat Input */}
        <ChatInputBar
          chatInput={chatInput}
          setChatInput={setChatInput}
          plannerSendMode={plannerSendMode}
          setPlannerSendMode={setPlannerSendMode}
          onSend={onSendChatMessage}
          pending={plannerMessagePending}
          isBuildActive={isBuildActive}
          pendingQuestion={pendingQuestion}
          buildPaused={buildPaused}
          inputRef={chatInputRef}
        />

        {/* Row 6: Bottom Nav Strip */}
        <BottomNavStrip
          activeOverlay={activeOverlay}
          onSelectOverlay={setActiveOverlay}
          onOpenPreview={onPreviewWorkspace}
          onStartOver={onStartOver}
          isStartingOver={isStartingOver}
          isBuildActive={isBuildActive}
          generatedFilesCount={generatedFiles.length}
          issueCount={issueCount}
          hasUrgentIssue={hasUrgentIssue}
          buildStatus={buildState.status}
        />
      </div>

      {/* Overlay panels */}
      {activeOverlay && (
        <PanelOverlay
          overlay={activeOverlay}
          onClose={() => setActiveOverlay(null)}
          buildState={buildState}
          generatedFiles={generatedFiles}
          aiThoughts={aiThoughts}
          chatMessages={chatMessages}
          isBuildActive={isBuildActive}
          providerPanels={providerPanels}
          visibleBlockers={visibleBlockers}
          platformReadinessNotice={platformReadinessNotice}
          agentMessageDrafts={agentMessageDrafts}
          agentMessagePendingId={agentMessagePendingId}
          pendingPermissionRequests={pendingPermissionRequests}
          pendingRevisionRequests={pendingRevisionRequests}
          buildFailureAttribution={buildFailureAttribution}
          proposedEdits={proposedEdits}
          showDiffReview={showDiffReview}
          permissionActionId={permissionActionId}
          rollbackCheckpointId={rollbackCheckpointId}
          patchBundleActionId={patchBundleActionId}
          promptProposalActionId={promptProposalActionId}
          userId={userId}
          onAgentMessageDraftChange={onAgentMessageDraftChange}
          onSendDirectAgentMessage={onSendDirectAgentMessage}
          onResolvePermission={onResolvePermission}
          onApprovePatchBundle={onApprovePatchBundle}
          onRejectPatchBundle={onRejectPatchBundle}
          onReviewPromptProposal={onReviewPromptProposal}
          onBenchmarkPromptProposal={onBenchmarkPromptProposal}
          onCreatePromptPackDraft={onCreatePromptPackDraft}
          onRequestPromptPackActivation={onRequestPromptPackActivation}
          onActivatePromptPackRequest={onActivatePromptPackRequest}
          onRollbackPromptPackVersion={onRollbackPromptPackVersion}
          onSetShowDiffReview={onSetShowDiffReview}
          onLoadProposedEdits={onLoadProposedEdits}
          onRollbackCheckpoint={onRollbackCheckpoint}
          onOpenCompletedBuild={onOpenCompletedBuild}
        />
      )}
    </div>
  )
}

export default BuildScreen
