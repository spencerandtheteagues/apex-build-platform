// Live Activity Feed — scrolling real-time AI agent activity stream
// with pinned User Communication section at the bottom

import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { cn } from '@/lib/utils'
import { AlertTriangle, MessageSquare, Play, ChevronRight } from 'lucide-react'
import type { BuildInteractionState } from '@/services/api'

interface AIThoughtEntry {
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

interface ChatMsgEntry {
  id: string
  role: 'user' | 'lead' | 'system'
  content: string
  timestamp: Date
}

interface BuildPermissionReq {
  id: string
  scope: string
  target: string
  reason: string
  status: string
}

interface LiveActivityFeedProps {
  aiThoughts: AIThoughtEntry[]
  chatMessages: ChatMsgEntry[]
  buildStatus: string
  interaction: BuildInteractionState | undefined
  isBuildActive: boolean
  pendingQuestion: string | undefined | null
  pendingPermissionRequests: BuildPermissionReq[]
  buildPaused: boolean
  onFocusChatInput: () => void
  onOpenIssues: () => void
  onResume: () => void
  buildCompleted: boolean
  onOpenPreview: () => void
  isPreparingPreview: boolean
  previewAvailable?: boolean
  previewPending?: boolean
}

const normalizeProvider = (p: string): string => {
  const v = String(p || '').toLowerCase()
  if (v === 'openai' || v === 'gpt' || v === 'gpt4') return 'gpt4'
  return v
}

const PROVIDER_LABEL: Record<string, string> = {
  claude: 'Claude',
  gpt4: 'ChatGPT',
  gemini: 'Gemini',
  grok: 'Grok',
  ollama: 'Kimi',
}

const PROVIDER_BADGE: Record<string, string> = {
  claude: 'bg-orange-500/20 text-orange-300 border-orange-500/30',
  gpt4: 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30',
  gemini: 'bg-sky-500/20 text-sky-300 border-sky-500/30',
  grok: 'bg-fuchsia-500/20 text-fuchsia-300 border-fuchsia-500/30',
  ollama: 'bg-cyan-500/20 text-cyan-200 border-cyan-500/30',
}

const humanize = (value?: string): string =>
  String(value || '')
    .replace(/[_-]+/g, ' ')
    .trim()
    .replace(/\b\w/g, (part) => part.toUpperCase())

const truncate = (value: string, maxLength = 260): string => {
  const compact = value.replace(/\s+/g, ' ').trim()
  if (compact.length <= maxLength) return compact
  return `${compact.slice(0, maxLength - 1).trimEnd()}…`
}

const lowerFirst = (value: string): string => (
  value ? value.charAt(0).toLowerCase() + value.slice(1) : value
)

const taskWorkLabel = (taskType?: string): string => {
  switch (taskType) {
    case 'plan':
      return 'build plan and work-order breakdown'
    case 'architecture':
      return 'architecture and contract boundaries'
    case 'generate_ui':
      return 'frontend UI shell and interactive screens'
    case 'generate_file':
      return 'generated project files'
    case 'generate_api':
      return 'backend API routes and contracts'
    case 'generate_schema':
      return 'database schema and persistence model'
    case 'test':
      return 'regression tests and runtime checks'
    case 'review':
      return 'acceptance review and quality gate'
    case 'fix':
      return 'repair patch for verification blockers'
    case 'deploy':
      return 'deployment configuration'
    case 'code_generation':
      return 'generated-code validation'
    default:
      return taskType ? lowerFirst(humanize(taskType)) : 'current work order'
  }
}

const taskActionPhrase = (taskType?: string): string => {
  switch (taskType) {
    case 'plan':
      return 'drafting the build plan, splitting work across agents, and defining acceptance checks'
    case 'architecture':
      return 'locking architecture boundaries before code generation'
    case 'generate_ui':
      return 'building the visible app UI, state flow, and responsive screens'
    case 'generate_file':
      return 'writing project files into the generated app'
    case 'generate_api':
      return 'implementing backend API routes and response contracts'
    case 'generate_schema':
      return 'creating database models, seed data, and persistence wiring'
    case 'test':
      return 'running regression checks against the generated project'
    case 'review':
      return 'reviewing output against the scaffold, contract, and acceptance checklist'
    case 'fix':
      return 'repairing blockers found by tests, preview verification, or code review'
    case 'deploy':
      return 'preparing deploy scripts and runtime configuration'
    case 'code_generation':
      return 'verifying generated files and integration points'
    default:
      return `working on ${taskWorkLabel(taskType)}`
  }
}

const extractElapsed = (content: string): string => {
  const match = content.match(/\(([^()]*elapsed)\)/i)
  return match?.[1]?.trim() || ''
}

const cleanActivityDetail = (thought: AIThoughtEntry): string => {
  let detail = String(thought.content || '').trim()
  if (!detail) return ''

  detail = detail
    .replace(/\s*\([^()]*elapsed\)/gi, '')
    .replace(/^[\w\s/-]+agent\s+is\s+still\s+(?:analyzing|generating|working on)\s*/i, '')
    .replace(/^[\w\s/-]+agent\s+is\s+(?:analyzing|generating|working on)\s*:?\s*/i, '')
    .replace(/^[\w\s/-]+agent\s+is\s+generating\s+code\s+with\s+[\w.-]+\.{0,3}$/i, '')
    .replace(/^[\w\s/-]+\s+is\s+still\s+(?:drafting|generating|working on)\s*/i, '')
    .replace(/^[\w\s/-]+\s+is\s+(?:analyzing|generating)\s*/i, '')
    .replace(/\s+Active heartbeat; no user action required\.?$/i, '')
    .replace(/\s+Waiting on provider output; not stalled\.?$/i, '')
    .replace(/^completed\s+/i, 'Completed ')
    .replace(/\s+/g, ' ')
    .trim()

  if (!detail || /^code with [\w.-]+\.{0,3}$/i.test(detail)) {
    return ''
  }
  return truncate(detail)
}

const providerRunLabel = (label: string, thought: AIThoughtEntry): string => {
  const model = String(thought.model || '').trim()
  return model ? `${label} / ${model}` : label
}

const activityCopy = (thought: AIThoughtEntry, providerLabel: string): { summary: string; detail: string } => {
  const actor = humanize(thought.agentRole) || providerLabel || 'Agent'
  const taskLabel = taskWorkLabel(thought.taskType)
  const actionPhrase = taskActionPhrase(thought.taskType)
  const detail = cleanActivityDetail(thought)
  const elapsed = extractElapsed(thought.content)
  const runner = providerRunLabel(providerLabel, thought)
  const filesCount = thought.filesCount || thought.files?.length || 0
  const stillWorking = /still\s+(?:analyzing|generating|working)/i.test(thought.content)

  let summary = `${actor} is ${actionPhrase}.`

  switch (thought.eventType) {
    case 'agent:spawned':
      summary = `${actor} joined the build and is ready for ${taskLabel}.`
      break
    case 'agent:working':
      summary = `${actor} started ${taskLabel}.`
      break
    case 'agent:thinking':
      summary = stillWorking
        ? `${actor} is still ${actionPhrase}.`
        : `${actor} is analyzing ${taskLabel}.`
      break
    case 'agent:generating':
      summary = stillWorking
        ? `${actor} is still generating output for ${taskLabel}.`
        : `${actor} is generating output for ${taskLabel}.`
      break
    case 'agent:completed':
      summary = `${actor} completed ${taskLabel}.`
      break
    case 'agent:output':
      summary = `${actor} produced output for ${taskLabel}.`
      break
    case 'code:generated':
      summary = `${actor} wrote ${filesCount || 'new'} project file${filesCount === 1 ? '' : 's'} for ${taskLabel}.`
      break
    case 'agent:retrying':
      summary = `${actor} is retrying ${taskLabel} with recovery context.`
      break
    case 'agent:verification_failed':
      summary = `${actor} found a verification issue and is routing a repair for ${taskLabel}.`
      break
    case 'agent:coordination_failed':
      summary = `${actor} hit a coordination issue and is reassigning ${taskLabel}.`
      break
    case 'agent:provider_switched':
      summary = `${actor} switched provider/model to keep ${taskLabel} moving.`
      break
    case 'agent:generation_failed':
    case 'agent:error':
      summary = `${actor} hit an error while handling ${taskLabel}.`
      break
    case 'spend:update':
      summary = `${actor} recorded model spend for this build step.`
      break
    case 'glassbox:provider_route_selected':
      summary = `${actor} routed ${taskLabel} to ${runner}.`
      break
    case 'glassbox:work_order_compiled':
      summary = `${actor} compiled a concrete work order for ${taskLabel}.`
      break
    case 'glassbox:deterministic_gate_passed':
      summary = `${actor} passed a deterministic gate for ${taskLabel}.`
      break
    case 'glassbox:deterministic_gate_failed':
      summary = `${actor} failed a deterministic gate and is preparing repair context.`
      break
    case 'glassbox:hydra_candidate_started':
      summary = `${actor} started a repair candidate for ${taskLabel}.`
      break
    case 'glassbox:hydra_candidate_passed':
      summary = `${actor} passed a repair candidate for ${taskLabel}.`
      break
    case 'glassbox:hydra_candidate_failed':
      summary = `${actor} rejected a failed repair candidate for ${taskLabel}.`
      break
    case 'glassbox:hydra_winner_selected':
      summary = `${actor} selected the safest patch candidate for ${taskLabel}.`
      break
    case 'glassbox:patch_review_required':
      summary = `${actor} marked a generated patch for review before merge.`
      break
    case 'glassbox:war_room_critique_started':
      summary = `${actor} opened a war-room critique for the current build risk.`
      break
    case 'glassbox:war_room_critique_resolved':
      summary = `${actor} resolved the war-room critique and returned to execution.`
      break
    default:
      if (thought.type === 'error') {
        summary = `${actor} needs recovery on ${taskLabel}.`
      }
      break
  }

  const detailParts = [
    elapsed ? `${elapsed}; still active, not stalled.` : '',
    detail,
  ].filter(Boolean)

  return {
    summary,
    detail: detailParts.length > 0 ? detailParts.join(' ') : `Provider: ${runner}. Task: ${taskLabel}.`,
  }
}

const eventLabel = (eventType?: string): string => {
  switch (eventType) {
    case 'agent:spawned':
      return 'joined'
    case 'agent:working':
      return 'started'
    case 'agent:thinking':
      return 'thinking'
    case 'agent:generating':
      return 'generating'
    case 'agent:completed':
      return 'completed'
    case 'agent:output':
      return 'output'
    case 'code:generated':
      return 'files'
    case 'agent:retrying':
      return 'retrying'
    case 'agent:verification_failed':
      return 'verification retry'
    case 'agent:coordination_failed':
      return 'coordination retry'
    case 'agent:provider_switched':
      return 'provider switch'
    case 'spend:update':
      return 'spend'
    default:
      return eventType ? humanize(eventType.replace(/^agent:/, '').replace(/^build:/, '').replace(/^glassbox:/, '')) : ''
  }
}

export const LiveActivityFeed: React.FC<LiveActivityFeedProps> = ({
  aiThoughts,
  chatMessages,
  buildStatus,
  interaction,
  isBuildActive,
  pendingQuestion,
  pendingPermissionRequests,
  buildPaused,
  onFocusChatInput,
  onOpenIssues,
  onResume,
  buildCompleted,
  onOpenPreview,
  isPreparingPreview,
  previewAvailable = buildCompleted,
  previewPending = false,
}) => {
  const feedRef = useRef<HTMLDivElement>(null)
  const bottomAnchorRef = useRef<HTMLDivElement>(null)
  const [userScrolledUp, setUserScrolledUp] = useState(false)

  // Last 200 displayable thoughts
  const displayThoughts = useMemo(
    () => aiThoughts.filter((t) => t.content?.trim()).slice(-200),
    [aiThoughts]
  )

  const latestThoughtKey = useMemo(() => {
    const latestThought = displayThoughts[displayThoughts.length - 1]
    if (!latestThought) return 'empty'
    return `${latestThought.id}:${latestThought.content}`
  }, [displayThoughts])

  const isNearBottom = useCallback((el: HTMLDivElement) => (
    el.scrollHeight - el.scrollTop - el.clientHeight < 80
  ), [])

  const scrollToLatest = useCallback((behavior: ScrollBehavior = 'auto') => {
    const el = feedRef.current
    if (!el) return
    // Use scrollTop directly — scrollIntoView can fight with mobile browser chrome
    if (typeof el.scrollTo === 'function') {
      el.scrollTo({ top: el.scrollHeight, behavior })
      return
    }
    el.scrollTop = el.scrollHeight
  }, [])

  // Keep the feed pinned to the latest event unless the user intentionally scrolls away.
  useLayoutEffect(() => {
    if (userScrolledUp || displayThoughts.length === 0) return undefined
    const frameId = window.requestAnimationFrame(() => {
      scrollToLatest('auto')
    })
    return () => window.cancelAnimationFrame(frameId)
  }, [displayThoughts.length, latestThoughtKey, scrollToLatest, userScrolledUp])

  const handleFeedScroll = () => {
    const el = feedRef.current
    if (!el) return
    const shouldPinToLatest = isNearBottom(el)
    setUserScrolledUp((prev) => (
      prev === !shouldPinToLatest ? prev : !shouldPinToLatest
    ))
  }

  // Last agent message for comms strip
  const lastAgentMsg = useMemo(() => {
    for (let i = chatMessages.length - 1; i >= 0; i--) {
      if (chatMessages[i].role !== 'user') return chatMessages[i]
    }
    return null
  }, [chatMessages])

  const awaitingReview = buildStatus === 'awaiting_review'
  const needsAttention = Boolean(
    pendingQuestion ||
    buildPaused ||
    pendingPermissionRequests.length > 0 ||
    awaitingReview ||
    interaction?.attention_required ||
    interaction?.waiting_for_user
  )
  const needsUserInput = Boolean(
    pendingQuestion || interaction?.waiting_for_user
  )

  let attentionMsg = ''
  if (pendingQuestion) attentionMsg = pendingQuestion
  else if (buildPaused) attentionMsg = 'Build is paused. Resume it or leave a note below.'
  else if (awaitingReview) attentionMsg = 'Code changes are ready for your review.'
  else if (pendingPermissionRequests.length > 0)
    attentionMsg = `${pendingPermissionRequests.length} permission request${pendingPermissionRequests.length > 1 ? 's' : ''} need your decision.`
  else if (lastAgentMsg) attentionMsg = lastAgentMsg.content

  return (
    <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden bg-slate-950/95">

      {/* Scrollable activity feed */}
      <div
        ref={feedRef}
        onScroll={handleFeedScroll}
        aria-label="Live activity feed"
        className="min-h-0 flex-1 overflow-y-auto px-4 py-3 overscroll-contain"
        style={{
          scrollbarWidth: 'thin',
          scrollbarColor: 'rgba(56,189,248,0.55) rgba(15,23,42,0.55)',
          WebkitOverflowScrolling: 'touch',
          touchAction: 'pan-y',
          overscrollBehavior: 'contain',
        }}
      >
        {displayThoughts.length === 0 ? (
          <div className="h-full flex flex-col items-center justify-center gap-4 text-center">
            <div className="w-8 h-8 rounded-full border-2 border-slate-700 border-t-cyan-300 animate-spin shadow-[0_0_18px_rgba(34,211,238,0.35)]" />
            <div className="text-slate-300 text-sm font-mono">
              {isBuildActive ? 'Agents are initializing...' : 'No activity yet'}
            </div>
          </div>
        ) : (
          <div className="space-y-2 font-mono text-sm">
            {displayThoughts.map((thought, i) => {
              const prov = normalizeProvider(thought.provider)
              const badge = PROVIDER_BADGE[prov] || 'bg-slate-500/20 text-slate-200 border-slate-500/30'
              const label = PROVIDER_LABEL[prov] || thought.provider || 'AI'
              const isError = thought.type === 'error'
              const metaItems = [
                humanize(thought.agentRole),
                thought.model,
                eventLabel(thought.eventType),
                humanize(thought.taskType),
                thought.filesCount ? `${thought.filesCount} files` : '',
                thought.retryCount && thought.maxRetries ? `try ${thought.retryCount}/${thought.maxRetries}` : '',
              ].filter(Boolean)
              const ts = thought.timestamp instanceof Date
                ? thought.timestamp.toLocaleTimeString('en-US', {
                    hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit',
                  })
                : ''
              const message = activityCopy(thought, label)

              return (
                <div
                  key={thought.id || i}
                  className={cn(
                    'group flex items-start gap-2 rounded-xl border px-2.5 py-2 leading-relaxed transition-colors',
                    isError
                      ? 'border-rose-400/30 bg-rose-950/20'
                      : 'border-sky-400/10 bg-slate-900/45 hover:border-sky-300/25 hover:bg-slate-900/70'
                  )}
                >
                  <span className="text-slate-400 text-[11px] sm:text-[10px] shrink-0 mt-1 w-[52px] sm:w-[54px] tabular-nums">
                    {ts}
                  </span>
                  <span className={cn(
                    'text-[10px] sm:text-[10px] font-bold uppercase border rounded-md px-1.5 py-0.5 shrink-0 mt-0.5 shadow-[0_0_12px_rgba(56,189,248,0.08)]',
                    badge
                  )}>
                    {label}
                  </span>
                  <span className={cn(
                    'flex-1 min-w-0 break-words text-[13px] sm:text-sm leading-snug',
                    isError ? 'text-rose-100' : 'text-slate-100',
                    thought.isInternal && 'not-italic'
                  )}>
                    {metaItems.length > 0 && (
                      <span className="block text-[10px] uppercase tracking-[0.14em] text-sky-300/80 not-italic">
                        {metaItems.join(' / ')}
                      </span>
                    )}
                    <span className="block font-semibold text-slate-50">
                      {message.summary}
                    </span>
                    <span className="block pt-0.5 text-xs leading-relaxed text-slate-300">
                      {message.detail}
                    </span>
                    {Array.isArray(thought.files) && thought.files.length > 0 && (
                      <span className="block pt-1 text-[11px] text-cyan-200/80 not-italic">
                        {thought.files.slice(0, 3).join(', ')}{thought.files.length > 3 ? ` (+${thought.files.length - 3})` : ''}
                      </span>
                    )}
                  </span>
                </div>
              )
            })}
          </div>
        )}

        {/* Scroll anchor — not used directly but kept for layout */}
        <div ref={bottomAnchorRef} aria-hidden="true" />
      </div>

      {/* Jump-to-bottom button when user has scrolled up */}
      {userScrolledUp && displayThoughts.length > 0 && (
        <div className="absolute bottom-24 right-6 z-10">
          <button
            onClick={() => {
              setUserScrolledUp(false)
              scrollToLatest('smooth')
            }}
            className="text-[10px] px-3 py-1.5 rounded-full bg-sky-500/20 border border-sky-300/40 text-sky-100 hover:bg-sky-500/30 font-bold uppercase tracking-wide flex items-center gap-1 shadow-[0_0_18px_rgba(56,189,248,0.22)]"
          >
            ↓ Latest
          </button>
        </div>
      )}

      {/* ── User Communication Strip (pinned bottom of feed area) ── */}
      {buildCompleted ? (
        /* Build complete banner */
        <div className="shrink-0 border-t border-green-500/30 bg-green-950/20 px-4 py-3 flex items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="w-5 h-5 rounded-full bg-green-500/30 flex items-center justify-center">
              <svg className="w-3 h-3 text-green-400" fill="currentColor" viewBox="0 0 20 20">
                <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
              </svg>
            </div>
            <div>
              <div className="text-[10px] font-bold uppercase tracking-widest text-green-400">Build Complete</div>
              <div className="text-sm text-gray-300 leading-snug">
                Your app is ready. Launch the preview or explore the files below.
              </div>
            </div>
          </div>
          <button
            onClick={onOpenPreview}
            disabled={isPreparingPreview}
            className="shrink-0 flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-green-600 hover:bg-green-500 text-white text-xs font-bold uppercase tracking-wide disabled:opacity-50"
          >
            {isPreparingPreview ? (
              <span className="animate-pulse">Loading...</span>
            ) : (
              <>
                <Play className="w-3 h-3" />
                Preview
              </>
            )}
          </button>
        </div>
      ) : previewAvailable ? (
        <div className="shrink-0 border-t border-cyan-500/30 bg-cyan-950/20 px-4 py-3 flex items-center justify-between gap-4">
          <div className="flex items-center gap-3 min-w-0">
            <div className="w-5 h-5 rounded-full bg-cyan-500/25 flex items-center justify-center shrink-0">
              <Play className="w-3 h-3 text-cyan-300" />
            </div>
            <div className="min-w-0">
              <div className="text-[10px] font-bold uppercase tracking-widest text-cyan-300">Frontend preview is ready</div>
              <div className="text-sm text-gray-300 leading-snug">
                Backend work may still be gated, but the generated UI can be opened now.
              </div>
            </div>
          </div>
          <button
            onClick={onOpenPreview}
            disabled={isPreparingPreview}
            aria-label="Open frontend preview"
            className="shrink-0 flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-bold uppercase tracking-wide disabled:opacity-50"
          >
            {isPreparingPreview ? (
              <span className="animate-pulse">Opening...</span>
            ) : (
              <>
                <Play className="w-3 h-3" />
                Preview
              </>
            )}
          </button>
        </div>
      ) : previewPending ? (
        <div className="shrink-0 border-t border-cyan-500/20 bg-cyan-950/10 px-4 py-3 flex items-center justify-between gap-4">
          <div className="flex items-center gap-3 min-w-0">
            <div className="w-5 h-5 rounded-full border border-cyan-500/35 border-t-cyan-300 animate-spin shrink-0" />
            <div className="min-w-0">
              <div className="text-[10px] font-bold uppercase tracking-widest text-cyan-300">Frontend preview is still building</div>
              <div className="text-sm text-gray-400 leading-snug">
                The free-plan fallback is generating UI artifacts; the Preview control unlocks when files are ready.
              </div>
            </div>
          </div>
          <button
            type="button"
            disabled
            aria-label="Preview is still building"
            className="shrink-0 flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-gray-800 bg-gray-950/70 text-gray-500 text-xs font-bold uppercase tracking-wide cursor-not-allowed"
          >
            Preview building
          </button>
        </div>
      ) : (
        /* Normal / Attention comms strip */
        <div
          className={cn(
            'shrink-0 border-t transition-all duration-300 px-4 py-3',
            needsUserInput
              ? 'border-amber-500/60 bg-amber-950/25'
              : needsAttention
                ? 'border-amber-500/25 bg-amber-950/10'
                : 'border-sky-500/15 bg-slate-950/90',
          )}
          style={{ minHeight: '68px' }}
        >
          <div className="flex items-start gap-3">
            {/* Icon */}
            <div className={cn(
              'shrink-0 w-6 h-6 rounded-full flex items-center justify-center mt-0.5',
              needsUserInput
                ? 'bg-amber-500/25 text-amber-300'
                : needsAttention
                  ? 'bg-amber-500/15 text-amber-500'
                  : 'bg-sky-500/10 text-sky-300'
            )}>
              {needsAttention
                ? <AlertTriangle className="w-3.5 h-3.5" />
                : <MessageSquare className="w-3.5 h-3.5" />
              }
            </div>

            {/* Text */}
            <div className="flex-1 min-w-0">
              {needsAttention && (
                <div className={cn(
                  'text-[10px] font-bold uppercase tracking-widest mb-0.5',
                  needsUserInput ? 'text-amber-400' :
                  buildPaused ? 'text-amber-600' :
                  awaitingReview ? 'text-sky-500' :
                  'text-amber-600'
                )}>
                  {needsUserInput ? 'Your input needed' :
                    buildPaused ? 'Build paused' :
                    awaitingReview ? 'Review required' :
                    pendingPermissionRequests.length > 0 ? 'Permissions needed' :
                    'Attention needed'}
                </div>
              )}
              <div className={cn(
                'text-sm leading-snug',
                needsAttention ? 'text-gray-100' : 'text-slate-300'
              )}>
                {attentionMsg || (isBuildActive ? 'Agents are working...' : 'Ready')}
              </div>
            </div>

            {/* Action buttons */}
            {needsAttention && (
              <div className="flex gap-1.5 shrink-0 mt-0.5">
                {(needsUserInput || buildPaused) && (
                  <button
                    onClick={onFocusChatInput}
                    className="text-[10px] px-2 py-1 rounded bg-amber-500/20 border border-amber-500/40 text-amber-300 hover:bg-amber-500/30 font-bold uppercase tracking-wide"
                  >
                    {buildPaused ? 'Send Note' : 'Answer'}
                  </button>
                )}
                {buildPaused && (
                  <button
                    onClick={onResume}
                    className="text-[10px] px-2 py-1 rounded bg-green-500/20 border border-green-500/40 text-green-300 hover:bg-green-500/30 font-bold uppercase tracking-wide flex items-center gap-1"
                  >
                    <Play className="w-2.5 h-2.5" />
                    Resume
                  </button>
                )}
                {(awaitingReview || pendingPermissionRequests.length > 0) && (
                  <button
                    onClick={onOpenIssues}
                    className="text-[10px] px-2 py-1 rounded bg-sky-500/20 border border-sky-500/40 text-sky-300 hover:bg-sky-500/30 font-bold uppercase tracking-wide flex items-center gap-1"
                  >
                    View
                    <ChevronRight className="w-2.5 h-2.5" />
                  </button>
                )}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

export default LiveActivityFeed
