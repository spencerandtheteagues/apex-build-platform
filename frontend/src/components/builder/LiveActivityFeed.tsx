// Live Activity Feed — scrolling real-time AI agent activity stream
// with pinned User Communication section at the bottom

import React, { useEffect, useRef, useState, useMemo } from 'react'
import { cn } from '@/lib/utils'
import { AlertTriangle, MessageSquare, Play, ChevronRight } from 'lucide-react'
import type { BuildInteractionState } from '@/services/api'

interface AIThoughtEntry {
  id: string
  provider: string
  type: 'thinking' | 'action' | 'output' | 'error'
  content: string
  timestamp: Date
  isInternal?: boolean
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
  ollama: 'Local',
}

const PROVIDER_BADGE: Record<string, string> = {
  claude: 'bg-orange-500/20 text-orange-300 border-orange-500/30',
  gpt4: 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30',
  gemini: 'bg-sky-500/20 text-sky-300 border-sky-500/30',
  grok: 'bg-fuchsia-500/20 text-fuchsia-300 border-fuchsia-500/30',
  ollama: 'bg-gray-500/20 text-gray-300 border-gray-500/30',
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
}) => {
  const feedRef = useRef<HTMLDivElement>(null)
  const [userScrolledUp, setUserScrolledUp] = useState(false)

  // Last 200 displayable thoughts
  const displayThoughts = useMemo(
    () => aiThoughts.filter((t) => t.content?.trim()).slice(-200),
    [aiThoughts]
  )

  // Auto-scroll to bottom when new thoughts arrive (unless user scrolled up)
  useEffect(() => {
    if (userScrolledUp) return
    if (feedRef.current) {
      feedRef.current.scrollTop = feedRef.current.scrollHeight
    }
  }, [displayThoughts.length, userScrolledUp])

  const handleFeedScroll = () => {
    const el = feedRef.current
    if (!el) return
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 60
    setUserScrolledUp(!atBottom)
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
    <div className="flex flex-col flex-1 min-h-0">

      {/* Scrollable activity feed */}
      <div
        ref={feedRef}
        onScroll={handleFeedScroll}
        className="flex-1 overflow-y-auto px-4 py-3"
        style={{ scrollbarWidth: 'thin', scrollbarColor: 'rgba(255,255,255,0.08) transparent' }}
      >
        {displayThoughts.length === 0 ? (
          <div className="h-full flex flex-col items-center justify-center gap-4 text-center">
            <div className="w-8 h-8 rounded-full border-2 border-gray-800 border-t-red-600 animate-spin" />
            <div className="text-gray-600 text-sm font-mono">
              {isBuildActive ? 'Agents are initializing...' : 'No activity yet'}
            </div>
          </div>
        ) : (
          <div className="space-y-0.5 font-mono text-sm">
            {displayThoughts.map((thought, i) => {
              const prov = normalizeProvider(thought.provider)
              const badge = PROVIDER_BADGE[prov] || 'bg-gray-500/20 text-gray-300 border-gray-500/30'
              const label = PROVIDER_LABEL[prov] || thought.provider || 'AI'
              const isError = thought.type === 'error'
              const ts = thought.timestamp instanceof Date
                ? thought.timestamp.toLocaleTimeString('en-US', {
                    hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit',
                  })
                : ''

              return (
                <div
                  key={thought.id || i}
                  className={cn(
                    'flex items-start gap-2 py-[3px] leading-relaxed',
                    isError && 'text-red-400'
                  )}
                >
                  <span className="text-gray-700 text-[10px] shrink-0 mt-px w-[54px] tabular-nums">
                    {ts}
                  </span>
                  <span className={cn(
                    'text-[10px] font-bold uppercase border rounded px-1 py-px shrink-0 mt-px',
                    badge
                  )}>
                    {label}
                  </span>
                  <span className={cn(
                    'flex-1 break-words text-gray-300',
                    isError && 'text-red-400',
                    thought.isInternal && 'text-gray-500 italic'
                  )}>
                    {thought.content}
                  </span>
                </div>
              )
            })}
          </div>
        )}

        {/* Scroll anchor — not used directly but kept for layout */}
        <div />
      </div>

      {/* Jump-to-bottom button when user has scrolled up */}
      {userScrolledUp && displayThoughts.length > 0 && (
        <div className="absolute bottom-[200px] right-6 z-10">
          <button
            onClick={() => {
              setUserScrolledUp(false)
              if (feedRef.current) {
                feedRef.current.scrollTop = feedRef.current.scrollHeight
              }
            }}
            className="text-[10px] px-2.5 py-1.5 rounded-full bg-gray-800 border border-gray-700 text-gray-300 hover:bg-gray-700 font-semibold flex items-center gap-1"
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
      ) : (
        /* Normal / Attention comms strip */
        <div
          className={cn(
            'shrink-0 border-t transition-all duration-300 px-4 py-3',
            needsUserInput
              ? 'border-amber-500/60 bg-amber-950/25'
              : needsAttention
                ? 'border-amber-500/25 bg-amber-950/10'
                : 'border-gray-800/60 bg-black/30',
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
                  : 'bg-gray-800/80 text-gray-600'
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
                needsAttention ? 'text-gray-200' : 'text-gray-600'
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
