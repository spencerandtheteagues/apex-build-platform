// APEX-BUILD App Builder - Command Center Interface
// Dark Demon Theme - AI-Powered App Generation with Futuristic UI

import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import { getConfiguredApiUrl, getConfiguredWsUrl } from '@/config/runtime'
import ModelRoleConfig from './ModelRoleConfig'
import { useThemeLogo } from '@/hooks/useThemeLogo'
import apiService, {
  BuildApproval,
  BuildBlocker,
  BuildCapabilityState,
  BuildMessageTargetMode,
  BuildIntentBrief,
  BuildContractSummary,
  BuildPatchBundleState,
  BuildPolicyState,
  BuildPromptImprovementProposalState,
  BuildPromotionDecisionState,
  BuildProviderScorecardState,
  BuildFailureFingerprintState,
  BuildLearningSummaryState,
  BuildPromptPackActivationEventState,
  BuildPromptPackActivationRequestState,
  BuildPromptPackVersionState,
  BuildVerificationReportState,
  BuildWorkOrderState,
  BuildConversationMessage as ApiBuildConversationMessage,
  FeatureReadinessSummary,
  BuildInteractionState as ApiBuildInteractionState,
  BuildPermissionRequest as ApiBuildPermissionRequest,
  BuildPermissionRule as ApiBuildPermissionRule,
  CompletedBuildDetail,
  ProposedBuildEdit,
} from '@/services/api'
import {
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Badge,
  Avatar,
  LoadingOverlay,
  AnimatedBackground
} from '@/components/ui'
import {
  Zap,
  Sparkles,
  Rocket,
  Bot,
  Code2,
  FileCode,
  CheckCircle2,
  Circle,
  AlertCircle,
  Clock,
  ChevronRight,
  MessageSquare,
  Send,
  Pause,
  Play,
  RotateCcw,
  Eye,
  Download,
  ExternalLink,
  Cpu,
  Database,
  Layout,
  TestTube,
  Shield,
  Terminal,
  Server,
  Globe,
  Layers,
  Github,
  Upload,
  KeyRound,
  PlugZap,
  LockKeyhole,
  CreditCard,
  MonitorUp
} from 'lucide-react'
import { GitHubImportWizard } from '@/components/import/GitHubImportWizard'
import { BuyCreditsModal } from '@/components/billing/BuyCreditsModal'
import { OnboardingTour } from './OnboardingTour'
import { BuildHistory } from './BuildHistory'
import {
  extractBuildFailureReason,
  isTerminalBuildStatus,
  mergeBuildStatusWithTerminalPrecedence,
  normalizeBuildStatus,
  parseBuildTelemetryThoughts,
  PersistedAIThought,
  readBuildTelemetrySnapshot,
  reconcileBuildPayloadWithCompletedDetail,
  resolveBuildCompletedEventStatus,
  upsertBuildTelemetrySnapshot,
} from './buildRestore'
import { buildAuthenticatedWebSocketUrl } from '@/services/authSession'
import { AssetUploader } from '@/components/project/AssetUploader'
import DiffReviewPanel from '@/components/diff/DiffReviewPanel'
import SpendToast from '@/components/spend/SpendToast'
import OrchestrationOverview from './OrchestrationOverview'
import BuildPieProgress from './BuildPieProgress'
import BuildScreen from './BuildScreen'
import TemplateGallery from './TemplateGallery'

// ============================================================================
// TYPES
// ============================================================================

interface Agent {
  id: string
  role: string
  provider: string
  model?: string
  status: 'idle' | 'working' | 'completed' | 'error'
  progress: number
  currentTask?: {
    type: string
    description: string
  }
}

interface Task {
  id: string
  type: string
  description: string
  status: 'pending' | 'in_progress' | 'completed' | 'failed' | 'cancelled'
  assignedTo?: string
  output?: {
    files?: Array<{ path: string; language: string }>
    messages?: string[]
    metrics?: Record<string, any>
  }
}

interface Checkpoint {
  id: string
  number: number
  name: string
  description: string
  progress: number
  restorable?: boolean
  createdAt: string
}

interface ChatMessage {
  id: string
  role: 'user' | 'lead' | 'system'
  content: string
  timestamp: Date
  kind?: string
  agentRole?: string
  targetMode?: BuildMessageTargetMode
  targetAgentId?: string
  targetAgentRole?: string
  clientToken?: string
  status?: 'pending' | 'sent' | 'failed'
}

interface AIThought {
  id: string
  agentId: string
  agentRole: string
  provider: string
  model?: string
  type: 'thinking' | 'action' | 'output' | 'error'
  eventType?: string
  taskId?: string
  taskType?: string
  files?: string[]
  filesCount?: number
  retryCount?: number
  maxRetries?: number
  isInternal?: boolean
  content: string
  timestamp: Date
}

interface BuildPlatformIssueContext {
  service?: string
  issueType?: string
  summary?: string
  retryable?: boolean
  maintenanceWindow?: boolean
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

interface BuildState {
  id: string
  status: 'idle' | 'pending' | 'planning' | 'in_progress' | 'testing' | 'reviewing' | 'awaiting_review' | 'completed' | 'failed' | 'cancelled'
  progress: number
  agents: Agent[]
  tasks: Task[]
  checkpoints: Checkpoint[]
  description: string
  availableProviders?: string[]
  powerMode?: 'fast' | 'balanced' | 'max'
  providerModelOverrides?: Partial<Record<SupportedBuildProvider, string>>
  currentPhase?: string
  qualityGateRequired?: boolean
  qualityGateStatus?: 'pending' | 'running' | 'passed' | 'failed'
  qualityGateStage?: string
  errorMessage?: string
  websocketUrl?: string
  liveSession?: boolean
  artifactRevision?: string
  diffMode?: boolean
  capabilityState?: BuildCapabilityState
  policyState?: BuildPolicyState
  blockers?: BuildBlocker[]
  approvals?: BuildApproval[]
  intentBrief?: BuildIntentBrief
  buildContract?: BuildContractSummary
  workOrders?: BuildWorkOrderState[]
  patchBundles?: BuildPatchBundleState[]
  verificationReports?: BuildVerificationReportState[]
  promotionDecision?: BuildPromotionDecisionState
  providerScorecards?: BuildProviderScorecardState[]
  failureFingerprints?: BuildFailureFingerprintState[]
  historicalLearning?: BuildLearningSummaryState
  promptPackActivationRequests?: BuildPromptPackActivationRequestState[]
  promptPackVersions?: BuildPromptPackVersionState[]
  promptPackActivationEvents?: BuildPromptPackActivationEventState[]
  truthBySurface?: Record<string, string[]>
  interaction?: ApiBuildInteractionState
  platformIssue?: BuildPlatformIssueContext
  guarantee?: BuildGuaranteeState
  previewUrl?: string
}

interface UpgradePromptState {
  reason: string
  suggestion: string
  requiredPlan: string
  buildId?: string
  source: 'start' | 'message'
}

type BuildWorkspaceView = 'overview' | 'activity' | 'files' | 'timeline' | 'issues' | 'diagnostics' | 'console'
type BuildWorkflowStageStatus = 'pending' | 'current' | 'complete' | 'blocked'

type BuildWorkflowStage = {
  key: string
  label: string
  description: string
  status: BuildWorkflowStageStatus
}

type BuildPermissionRequest = ApiBuildPermissionRequest
type BuildPermissionRule = ApiBuildPermissionRule
type BuildInteractionState = ApiBuildInteractionState
type ProposedEdit = ProposedBuildEdit

type BuildMode = 'fast' | 'full'

interface TechStack {
  id: string
  name: string
  icon: React.ReactNode
  category: 'frontend' | 'backend' | 'database' | 'deploy' | 'auto'
  description: string
}

interface BuildTechStack {
  frontend?: string
  backend?: string
  database?: string
  styling?: string
  extras?: string[]
}

interface AppBuilderProps {
  onNavigateToIDE?: (options?: { target?: 'dashboard' | 'editor' | 'preview'; projectId?: number | null }) => void
  onNavigateToView?: (view: 'settings' | 'spending') => void
  startOverSignal?: number
}

const ACTIVE_BUILD_STORAGE_KEY = 'apex_active_build_id'
const LAST_WORKFLOW_BUILD_STORAGE_KEY = 'apex_last_workflow_build_id'
const BUILD_TELEMETRY_STORAGE_KEY = 'apex_build_telemetry_cache'
const DEFAULT_RESTART_FAILED_MESSAGE = 'Restart the failed build from the last workable state, keep the valid work, fix the failure, and continue until the app is runnable.'

const extractPlatformIssue = (source: any): BuildPlatformIssueContext | undefined => {
  const payload = source?.response?.data ?? source
  if (!payload || payload.platform_issue !== true) {
    return undefined
  }

  return {
    service: typeof payload.platform_service === 'string' ? payload.platform_service : undefined,
    issueType: typeof payload.platform_issue_type === 'string' ? payload.platform_issue_type : undefined,
    summary: typeof payload.platform_issue_summary === 'string' ? payload.platform_issue_summary : undefined,
    retryable: typeof payload.retryable === 'boolean' ? payload.retryable : undefined,
    maintenanceWindow: payload.maintenance_window === true,
  }
}

const asFiniteNumber = (value: unknown): number | undefined => {
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : undefined
}

const normalizeGuaranteeVerdict = (value: unknown): BuildGuaranteeState['verdict'] | undefined => {
  const verdict = String(value || '').trim().toLowerCase()
  if (verdict === 'pass' || verdict === 'soft_fail' || verdict === 'hard_fail') {
    return verdict
  }
  return undefined
}

const extractGuaranteeState = (source: any): BuildGuaranteeState | undefined => {
  const payload = source?.output?.metrics ?? source?.metrics ?? source
  if (!payload || typeof payload !== 'object') return undefined

  const attempts = asFiniteNumber(payload.guarantee_attempts ?? payload.attempts)
  const score = asFiniteNumber(payload.guarantee_score ?? payload.score)
  const durationMs = asFiniteNumber(payload.guarantee_duration_ms ?? payload.duration_ms)
  const verdict = normalizeGuaranteeVerdict(payload.guarantee_verdict ?? payload.verdict)
  const rolledBack = payload.guarantee_rolled_back === true || payload.rolled_back === true
  const error = typeof payload.error === 'string' ? payload.error : undefined
  const taskId = typeof payload.task_id === 'string' ? payload.task_id : undefined
  const taskType = typeof payload.task_type === 'string' ? payload.task_type : undefined

  if (
    attempts === undefined &&
    score === undefined &&
    durationMs === undefined &&
    verdict === undefined &&
    !rolledBack &&
    !error
  ) {
    return undefined
  }

  let status: BuildGuaranteeState['status']
  if (rolledBack) {
    status = 'failed'
  } else if (verdict === 'hard_fail') {
    status = 'failed'
  } else if (verdict === 'soft_fail') {
    status = 'retrying'
  } else if (verdict === 'pass') {
    status = 'passed'
  } else {
    status = 'validating'
  }

  return {
    status,
    verdict,
    attempts: attempts ?? 1,
    score,
    rolledBack,
    durationMs,
    error,
    taskId,
    taskType,
    updatedAt: new Date().toISOString(),
  }
}

const extractLatestGuaranteeStateFromTasks = (tasks: Task[]): BuildGuaranteeState | undefined => {
  for (let index = tasks.length - 1; index >= 0; index -= 1) {
    const task = tasks[index]
    const guarantee = extractGuaranteeState({
      metrics: task.output?.metrics,
      task_id: task.id,
      task_type: task.type,
    })
    if (guarantee) return guarantee
  }
  return undefined
}

const BUILD_WORKFLOW_STAGE_DEFS = [
  {
    key: 'scaffold',
    label: 'Scaffold',
    description: 'Freeze the stack, screen map, and API/data contract before runtime work starts.',
  },
  {
    key: 'frontend_ui',
    label: 'Frontend UI',
    description: 'Build the first usable interface and core screens before backend wiring.',
  },
  {
    key: 'backend_data',
    label: 'Backend & Data',
    description: 'Add schema, persistence, and APIs behind the visible UI contract.',
  },
  {
    key: 'integration',
    label: 'Integration',
    description: 'Connect the UI to backend flows and verify the main vertical slice.',
  },
  {
    key: 'ship',
    label: 'Preview / Ship',
    description: 'Run final review, package the build, and prepare it for handoff.',
  },
] as const

const isActiveBuildStatus = (status?: string) =>
  status === 'pending' ||
  status === 'planning' ||
  status === 'in_progress' ||
  status === 'testing' ||
  status === 'reviewing' ||
  status === 'awaiting_review'

type SupportedBuildProvider = 'claude' | 'gpt4' | 'gemini' | 'grok' | 'ollama'

type ProviderModelTier = {
  id: string
  name: string
}

type ProviderPanelState = {
  provider: SupportedBuildProvider
  configuredModel: ProviderModelTier
  liveModelId: string
  liveModelName: string
  available: boolean
  status: 'idle' | 'working' | 'thinking' | 'completed' | 'error' | 'unavailable'
  statusLabel: string
  agentCount: number
  activeRoles: string[]
  totalUpdates: number
  currentTaskLabel?: string
  latestThought?: AIThought
  thoughts: AIThought[]
  mismatch: boolean
  multipleLiveModels: boolean
}

const MODEL_PANEL_ORDER: SupportedBuildProvider[] = ['claude', 'gpt4', 'gemini', 'grok', 'ollama']
const MAX_AI_THOUGHTS = 240
const MAX_PROVIDER_THOUGHTS = 36

const POWER_MODE_MODEL_CATALOG: Record<'fast' | 'balanced' | 'max', Record<SupportedBuildProvider, ProviderModelTier>> = {
  fast: {
    claude: { id: 'claude-haiku-4-5-20251001', name: 'Claude Haiku 4.5' },
    gpt4: { id: 'gpt-4o-mini', name: 'GPT-4o Mini' },
    gemini: { id: 'gemini-2.5-flash-lite', name: 'Gemini 2.5 Flash Lite' },
    grok: { id: 'grok-3-mini', name: 'Grok 3 Mini' },
    ollama: { id: 'glm-5.1', name: 'GLM-5.1' },
  },
  balanced: {
    claude: { id: 'claude-sonnet-4-6', name: 'Claude Sonnet 4.6' },
    gpt4: { id: 'gpt-4.1', name: 'GPT-4.1' },
    gemini: { id: 'gemini-3-flash-preview', name: 'Gemini 3 Flash Preview' },
    grok: { id: 'grok-3', name: 'Grok 3' },
    ollama: { id: 'kimi-k2.6', name: 'Kimi K2.6' },
  },
  max: {
    claude: { id: 'claude-opus-4-7', name: 'Claude Opus 4.7' },
    gpt4: { id: 'gpt-codex-5.5', name: 'ChatGPT Codex 5.5' },
    gemini: { id: 'gemini-3.1-pro-preview', name: 'Gemini 3.1 Pro Preview' },
    grok: { id: 'grok-4.20-0309-reasoning', name: 'Grok 4.20' },
    ollama: { id: 'kimi-k2.6', name: 'Kimi K2.6' },
  },
}

const PROVIDER_UI: Record<SupportedBuildProvider, {
  label: string
  badgeClass: string
  cardClass: string
  activeClass: string
  titleClass: string
  dotClass: string
}> = {
  claude: {
    label: 'Claude',
    badgeClass: 'border-orange-500/60 text-orange-300 bg-orange-500/10',
    cardClass: 'border-orange-500/35 bg-gradient-to-br from-orange-950/55 via-black to-orange-950/25',
    activeClass: 'shadow-[0_0_28px_rgba(251,146,60,0.16)]',
    titleClass: 'text-orange-200',
    dotClass: 'bg-orange-400',
  },
  gpt4: {
    label: 'OpenAI',
    badgeClass: 'border-emerald-500/60 text-emerald-300 bg-emerald-500/10',
    cardClass: 'border-emerald-500/35 bg-gradient-to-br from-emerald-950/55 via-black to-emerald-950/25',
    activeClass: 'shadow-[0_0_28px_rgba(16,185,129,0.16)]',
    titleClass: 'text-emerald-200',
    dotClass: 'bg-emerald-400',
  },
  gemini: {
    label: 'Gemini',
    badgeClass: 'border-sky-500/60 text-sky-300 bg-sky-500/10',
    cardClass: 'border-sky-500/35 bg-gradient-to-br from-sky-950/55 via-black to-sky-950/25',
    activeClass: 'shadow-[0_0_28px_rgba(56,189,248,0.16)]',
    titleClass: 'text-sky-200',
    dotClass: 'bg-sky-400',
  },
  grok: {
    label: 'Grok',
    badgeClass: 'border-fuchsia-500/60 text-fuchsia-300 bg-fuchsia-500/10',
    cardClass: 'border-fuchsia-500/35 bg-gradient-to-br from-fuchsia-950/55 via-black to-fuchsia-950/25',
    activeClass: 'shadow-[0_0_28px_rgba(217,70,239,0.16)]',
    titleClass: 'text-fuchsia-200',
    dotClass: 'bg-fuchsia-400',
  },
  ollama: {
    label: 'Local',
    badgeClass: 'border-cyan-500/60 text-cyan-200 bg-cyan-500/10',
    cardClass: 'border-cyan-500/35 bg-gradient-to-br from-cyan-950/45 via-black to-slate-950/35',
    activeClass: 'shadow-[0_0_28px_rgba(34,211,238,0.18)]',
    titleClass: 'text-cyan-100',
    dotClass: 'bg-cyan-300',
  },
}

const normalizeProviderKey = (provider?: string): SupportedBuildProvider | null => {
  const value = String(provider || '').toLowerCase()
  if (value === 'gpt' || value === 'gpt4' || value === 'openai') return 'gpt4'
  if (value === 'ollama' || value === 'local' || value === 'kimi' || value === 'kimi-k2.6') return 'ollama'
  if (value === 'claude' || value === 'gemini' || value === 'grok') return value
  return null
}

const getModelTier = (mode: 'fast' | 'balanced' | 'max') => POWER_MODE_MODEL_CATALOG[mode]

const getPowerModeModelSummary = (mode: 'fast' | 'balanced' | 'max') =>
  MODEL_PANEL_ORDER.map((provider) => getModelTier(mode)[provider].name).join(' / ')

const DEFAULT_PROVIDER_MODEL_SELECTIONS: Record<SupportedBuildProvider, string> = {
  claude: 'auto',
  gpt4: 'auto',
  gemini: 'auto',
  grok: 'auto',
  ollama: 'auto',
}

const canonicalizeModelId = (model?: string) => {
  const value = String(model || '').trim()
  if (!value) return ''
  if (value.toLowerCase() === 'auto') return 'auto'

  if (value.startsWith('gpt-codex-5.5')) return 'gpt-codex-5.5'
  if (value.startsWith('gpt-5.4-pro')) return 'gpt-5.4-pro'
  if (value.startsWith('gpt-5.4')) return value
  if (value.startsWith('gpt-4.1')) return 'gpt-4.1'
  if (value.startsWith('gpt-4o-mini')) return 'gpt-4o-mini'
  if (value.startsWith('gpt-4o')) return 'gpt-4o'

  if (value.startsWith('claude-opus-4-7')) return 'claude-opus-4-7'
  if (value.startsWith('claude-opus-4-6')) return 'claude-opus-4-6'
  if (value.startsWith('claude-sonnet-4-6')) return 'claude-sonnet-4-6'
  if (value.startsWith('claude-haiku-4-5')) return 'claude-haiku-4-5-20251001'

  if (value.startsWith('gemini-3.1-pro-preview')) return 'gemini-3.1-pro-preview'
  if (value.startsWith('gemini-3.1-pro')) return 'gemini-3.1-pro'
  if (value.startsWith('gemini-3-pro')) return 'gemini-3-pro-preview'
  if (value.startsWith('gemini-3-flash-preview')) return 'gemini-3-flash-preview'
  if (value.startsWith('gemini-2.5-flash-lite')) return 'gemini-2.5-flash-lite'

  if (value.startsWith('grok-4.20')) return 'grok-4.20-0309-reasoning'
  if (value.startsWith('grok-3-mini')) return 'grok-3-mini'
  if (value.startsWith('grok-3')) return 'grok-3'

  if (value.startsWith('kimi-k2.6') || value.startsWith('kimi-k2')) return 'kimi-k2.6'
  if (value.startsWith('glm-5.1')) return 'glm-5.1'
  if (value.startsWith('qwen-3.6-27b')) return 'qwen-3.6-27b'
  if (value.startsWith('devstral-small-24b') || value.startsWith('devstral-24b')) return 'devstral-small-24b'
  if (value.startsWith('deepseek-v4-flash')) return 'deepseek-v4-flash'

  return value
}

const providerForModelId = (model?: string): SupportedBuildProvider | null => {
  const canonicalModel = canonicalizeModelId(model)
  if (!canonicalModel) return null
  if (canonicalModel.startsWith('claude-')) return 'claude'
  if (
    canonicalModel.startsWith('gpt-') ||
    canonicalModel.startsWith('chatgpt-') ||
    canonicalModel.startsWith('o1') ||
    canonicalModel.startsWith('o3') ||
    canonicalModel.startsWith('o4')
  ) return 'gpt4'
  if (canonicalModel.startsWith('gemini-')) return 'gemini'
  if (canonicalModel.startsWith('grok-')) return 'grok'
  if (
    canonicalModel.startsWith('kimi-') ||
    canonicalModel.startsWith('glm-') ||
    canonicalModel.startsWith('qwen-') ||
    canonicalModel.startsWith('devstral') ||
    canonicalModel.startsWith('deepseek') ||
    canonicalModel.startsWith('llama') ||
    canonicalModel.startsWith('mistral')
  ) return 'ollama'
  return null
}

const modelBelongsToProvider = (provider: SupportedBuildProvider, model?: string) =>
  provider === 'ollama'
    ? Boolean(canonicalizeModelId(model)) && canonicalizeModelId(model) !== 'auto'
    : providerForModelId(model) === provider

const getModelDisplayName = (model?: string, fallbackMode: 'fast' | 'balanced' | 'max' = 'fast') => {
  const canonicalModel = canonicalizeModelId(model)
  if (!canonicalModel) return ''
  if (canonicalModel === 'auto') return 'Auto'
  for (const tier of Object.values(POWER_MODE_MODEL_CATALOG)) {
    for (const entry of Object.values(tier)) {
      if (entry.id === canonicalModel) return entry.name
    }
  }
  const provider = normalizeProviderKey(canonicalModel)
  if (provider) return getModelTier(fallbackMode)[provider].name
  return canonicalModel
}

const normalizeProviderModelOverrides = (
  overrides?: Record<string, string> | null
): Record<SupportedBuildProvider, string> => {
  const normalized = { ...DEFAULT_PROVIDER_MODEL_SELECTIONS }
  for (const provider of MODEL_PANEL_ORDER) {
    const raw = typeof overrides?.[provider] === 'string' ? canonicalizeModelId(overrides[provider]) : ''
    if (raw === 'auto' || !raw) {
      normalized[provider] = 'auto'
      continue
    }
    if (modelBelongsToProvider(provider, raw)) {
      normalized[provider] = raw
    }
  }
  return normalized
}

const serializeProviderModelOverrides = (
  overrides: Record<SupportedBuildProvider, string>
): Record<string, string> | undefined => {
  const serialized = MODEL_PANEL_ORDER.reduce<Record<string, string>>((acc, provider) => {
    const model = canonicalizeModelId(overrides[provider])
    if (model && model !== 'auto' && modelBelongsToProvider(provider, model)) {
      acc[provider] = model
    }
    return acc
  }, {})
  return Object.keys(serialized).length > 0 ? serialized : undefined
}

const PROVIDER_MODEL_OPTIONS: Record<SupportedBuildProvider, ProviderModelTier[]> = MODEL_PANEL_ORDER.reduce(
  (acc, provider) => {
    const seen = new Set<string>()
    acc[provider] = []
    for (const mode of ['max', 'balanced', 'fast'] as const) {
      const model = POWER_MODE_MODEL_CATALOG[mode][provider]
      if (!seen.has(model.id)) {
        seen.add(model.id)
        acc[provider].push(model)
      }
    }
    return acc
  },
  {} as Record<SupportedBuildProvider, ProviderModelTier[]>
)

const humanizeIdentifier = (value?: string) => {
  const normalized = String(value || '')
    .replace(/[_-]+/g, ' ')
    .trim()
  if (!normalized) return ''
  return normalized.replace(/\b\w/g, (match) => match.toUpperCase())
}

const getThoughtEventLabel = (thought: AIThought) => {
  switch (thought.eventType) {
    case 'agent:spawned':
      return 'Agent Joined'
    case 'agent:working':
      return 'Task Started'
    case 'agent:thinking':
      return thought.isInternal ? 'Internal Thinking' : 'Thinking'
    case 'agent:generating':
      return 'Generating'
    case 'agent:retrying':
      return 'Retrying'
    case 'agent:verification_failed':
      return 'Verification Retry'
    case 'agent:coordination_failed':
      return 'Coordination Retry'
    case 'agent:provider_switched':
      return 'Provider Switch'
    case 'agent:message':
      return 'Directed Message'
    case 'agent:completed':
      return 'Task Complete'
    case 'agent:error':
    case 'agent:generation_failed':
      return 'Error'
    case 'code:generated':
      return 'Files Generated'
    case 'spend:update':
      return 'Spend'
    case 'glassbox:war_room_critique_started':
      return 'War Room Critique Started'
    case 'glassbox:war_room_critique_resolved':
      return 'War Room Critique Resolved'
    case 'glassbox:work_order_compiled':
      return 'Work Orders Compiled'
    case 'glassbox:provider_route_selected':
      return 'Provider Route Selected'
    case 'glassbox:deterministic_gate_passed':
      return 'Deterministic Gate Passed'
    case 'glassbox:deterministic_gate_failed':
      return 'Deterministic Gate Failed'
    case 'glassbox:hydra_candidate_started':
      return 'Hydra Candidate Started'
    case 'glassbox:hydra_candidate_passed':
      return 'Hydra Candidate Passed'
    case 'glassbox:hydra_candidate_failed':
      return 'Hydra Candidate Failed'
    case 'glassbox:hydra_winner_selected':
      return 'Hydra Winner Selected'
    case 'glassbox:patch_review_required':
      return 'Patch Review Required'
    default:
      return humanizeIdentifier(thought.type)
  }
}

const getConversationTargetLabel = (message: Pick<ChatMessage, 'targetMode' | 'targetAgentRole' | 'targetAgentId'>) => {
  switch (message.targetMode) {
    case 'lead':
      return 'Planner'
    case 'agent':
      return humanizeIdentifier(message.targetAgentRole || message.targetAgentId || 'agent')
    case 'role':
      return humanizeIdentifier(message.targetAgentRole || 'role')
    case 'all_agents':
      return 'All Agents'
    default:
      return ''
  }
}

const getConversationRouteLabel = (message: ChatMessage) => {
  const source = message.role === 'user'
    ? 'You'
    : message.role === 'lead'
      ? 'Planner'
      : 'System'
  const target = getConversationTargetLabel(message)
  return target ? `${source} -> ${target}` : source
}

const upsertPromptPackActivationRequest = (
  existing: BuildPromptPackActivationRequestState[] | undefined,
  request: BuildPromptPackActivationRequestState
): BuildPromptPackActivationRequestState[] => {
  if (!request?.id) return existing || []
  const current = existing || []
  const index = current.findIndex(candidate => candidate.id === request.id)
  if (index >= 0) {
    return current.map(candidate => candidate.id === request.id ? { ...candidate, ...request } : candidate)
  }
  return [...current, request]
}

const upsertPromptPackVersion = (
  existing: BuildPromptPackVersionState[] | undefined,
  version: BuildPromptPackVersionState
): BuildPromptPackVersionState[] => {
  if (!version?.id) return existing || []
  const current = existing || []
  const index = current.findIndex(candidate => candidate.id === version.id)
  if (index >= 0) {
    return current.map(candidate => candidate.id === version.id ? { ...candidate, ...version } : candidate)
  }
  return [...current, version]
}

const upsertPromptPackActivationEvent = (
  existing: BuildPromptPackActivationEventState[] | undefined,
  event: BuildPromptPackActivationEventState
): BuildPromptPackActivationEventState[] => {
  if (!event?.id) return existing || []
  const current = existing || []
  const index = current.findIndex(candidate => candidate.id === event.id)
  if (index >= 0) {
    return current.map(candidate => candidate.id === event.id ? { ...candidate, ...event } : candidate)
  }
  return [...current, event]
}

const ThinkingDots: React.FC<{ className?: string }> = ({ className }) => {
  const [count, setCount] = useState(1)

  useEffect(() => {
    const id = window.setInterval(() => {
      setCount((prev) => (prev === 3 ? 1 : prev + 1))
    }, 420)
    return () => window.clearInterval(id)
  }, [])

  return <span className={cn('font-mono tracking-[0.2em]', className)}>{'.'.repeat(count)}</span>
}

// ============================================================================
// ANIMATED BACKGROUND COMPONENTS
// ============================================================================

const HexGrid: React.FC = () => {
  return (
    <div className="app-builder-hex-grid absolute inset-0 overflow-hidden opacity-30 pointer-events-none">
      <svg className="absolute inset-0 w-full h-full" xmlns="http://www.w3.org/2000/svg">
        <defs>
          <pattern id="hexagons" width="50" height="43.4" patternUnits="userSpaceOnUse" patternTransform="scale(2)">
            <polygon
              fill="none"
              strokeWidth="0.5"
              style={{ stroke: 'var(--builder-hex-stroke, rgba(204, 0, 0, 0.3))' }}
              points="24.8,22 37.3,29.2 37.3,43.7 24.8,50.9 12.3,43.7 12.3,29.2"
              transform="translate(0, -21.7)"
            />
            <polygon
              fill="none"
              strokeWidth="0.5"
              style={{ stroke: 'var(--builder-hex-stroke, rgba(204, 0, 0, 0.3))' }}
              points="24.8,22 37.3,29.2 37.3,43.7 24.8,50.9 12.3,43.7 12.3,29.2"
              transform="translate(25, 0)"
            />
          </pattern>
        </defs>
        <rect width="100%" height="100%" fill="url(#hexagons)" />
      </svg>
    </div>
  )
}

const FloatingParticles: React.FC = () => {
  const particles = useMemo(() =>
    Array.from({ length: 25 }, (_, i) => ({
      id: i,
      size: Math.random() * 3 + 1,
      x: Math.random() * 100,
      y: Math.random() * 100,
      duration: Math.random() * 20 + 10,
      delay: Math.random() * 5,
    })), []
  )

  return (
    <div className="app-builder-particles absolute inset-0 overflow-hidden pointer-events-none">
      {particles.map((particle) => (
        <div
          key={particle.id}
          className="app-builder-particle absolute rounded-full"
          style={{
            width: particle.size,
            height: particle.size,
            left: `${particle.x}%`,
            top: `${particle.y}%`,
            animation: `float ${particle.duration}s ease-in-out infinite`,
            animationDelay: `${particle.delay}s`,
            background: 'var(--builder-particle-bg, rgba(47, 168, 255, 0.18))',
            boxShadow: 'var(--builder-particle-shadow, 0 0 8px rgba(138, 223, 255, 0.25))',
          }}
        />
      ))}
    </div>
  )
}

// ============================================================================
// ANIMATED LOGO COMPONENT
// ============================================================================

const AnimatedLogo: React.FC = () => {
  const logoSrc = useThemeLogo()
  return (
    <div className="app-builder-logo relative group">
      {/* Main logo - large and clean, no background box */}
      <div className="relative w-[20rem] h-[20rem] md:w-[26rem] md:h-[26rem] flex items-center justify-center group-hover:scale-105 transition-transform duration-500">
        <img
          src={logoSrc}
          alt="APEX"
          className="app-builder-logo-image w-full h-full object-contain relative z-10 drop-shadow-[0_0_30px_rgba(126,231,255,0.55)]"
        />
      </div>

      {/* Corner accents - HUD style brackets */}
      <div className="app-builder-logo-corner absolute -top-4 -left-4 w-6 h-6 border-t-2 border-l-2 border-[rgba(188,239,255,0.45)] rounded-tl" />
      <div className="app-builder-logo-corner absolute -top-4 -right-4 w-6 h-6 border-t-2 border-r-2 border-[rgba(188,239,255,0.45)] rounded-tr" />
      <div className="app-builder-logo-corner absolute -bottom-4 -left-4 w-6 h-6 border-b-2 border-l-2 border-[rgba(188,239,255,0.45)] rounded-bl" />
      <div className="app-builder-logo-corner absolute -bottom-4 -right-4 w-6 h-6 border-b-2 border-r-2 border-[rgba(188,239,255,0.45)] rounded-br" />
    </div>
  )
}

// ============================================================================
// ANIMATED TITLE COMPONENT
// ============================================================================

const AnimatedTitle: React.FC = () => {
  return (
    <h1 className="app-builder-title text-5xl md:text-6xl font-black relative tracking-tight">
      <span
        className="bg-gradient-to-r from-white via-[#8adfff] to-[#2fa8ff] bg-clip-text text-transparent"
        style={{
          backgroundSize: '200% auto',
          animation: 'gradient-shift 4s linear infinite',
        }}
      >
        Build Your App
      </span>
      {/* Glow layer */}
      <span
        className="absolute inset-0 bg-gradient-to-r from-white via-[#8adfff] to-[#2fa8ff] bg-clip-text text-transparent blur-xl opacity-30 pointer-events-none"
        style={{
          backgroundSize: '200% auto',
          animation: 'gradient-shift 4s linear infinite',
        }}
        aria-hidden="true"
      >
        Build Your App
      </span>
    </h1>
  )
}

// ============================================================================
// TYPEWRITER SUBTITLE COMPONENT
// ============================================================================

const TypewriterSubtitle: React.FC<{ text: string }> = ({ text }) => {
  const [displayedText, setDisplayedText] = useState('')
  const [showCursor, setShowCursor] = useState(true)

  useEffect(() => {
    let index = 0
    const timer = setInterval(() => {
      if (index < text.length) {
        setDisplayedText(text.slice(0, index + 1))
        index++
      } else {
        clearInterval(timer)
      }
    }, 35)

    return () => clearInterval(timer)
  }, [text])

  useEffect(() => {
    const cursorTimer = setInterval(() => {
      setShowCursor(prev => !prev)
    }, 530)
    return () => clearInterval(cursorTimer)
  }, [])

  return (
    <p className="app-builder-subtitle text-gray-400 text-lg md:text-xl font-light tracking-wide">
      {displayedText}
      <span className={cn("inline-block w-0.5 h-5 bg-red-500 ml-1 align-middle transition-opacity duration-100", showCursor ? "opacity-100" : "opacity-0")} />
    </p>
  )
}

// ============================================================================
// PREMIUM TEXTAREA COMPONENT
// ============================================================================

interface PremiumTextareaProps {
  value: string
  onChange: (value: string) => void
  maxLength?: number
}

const FAST_BUILD_PROMPT_MAX_LENGTH = 2000
const BALANCED_FULL_BUILD_PROMPT_MAX_LENGTH = 25000
const FULL_BUILD_PROMPT_MAX_LENGTH = 50000

const getBuildPromptMaxLength = (mode: BuildMode, powerMode: 'fast' | 'balanced' | 'max') =>
  mode !== 'full'
    ? FAST_BUILD_PROMPT_MAX_LENGTH
    : powerMode === 'max'
      ? FULL_BUILD_PROMPT_MAX_LENGTH
      : powerMode === 'balanced'
        ? BALANCED_FULL_BUILD_PROMPT_MAX_LENGTH
        : FAST_BUILD_PROMPT_MAX_LENGTH

const PremiumTextarea: React.FC<PremiumTextareaProps> = ({ value, onChange, maxLength = FAST_BUILD_PROMPT_MAX_LENGTH }) => {
  const [isFocused, setIsFocused] = useState(false)
  const isEmpty = value.length === 0
  const progressPercent = (value.length / maxLength) * 100

  return (
    <div className="premium-textarea relative group">
      {/* Animated border container */}
      <div className={cn(
        "premium-textarea-border absolute -inset-[2px] rounded-2xl transition-all duration-500",
        isEmpty && !isFocused && "animate-pulse",
        isFocused
          ? "bg-gradient-to-r from-[#8adfff] via-[#2fa8ff] to-[#8adfff] shadow-lg shadow-[rgba(47,168,255,0.28)]"
          : "bg-gradient-to-r from-[rgba(138,223,255,0.28)] to-[rgba(47,168,255,0.18)]"
      )} style={isFocused ? { backgroundSize: '200% auto', animation: 'gradient-shift 2s linear infinite' } : {}} />

      {/* Glass effect background */}
      <div className="premium-textarea-shell absolute inset-0 rounded-xl bg-black/90 backdrop-blur-xl" />

      {/* Inner glow on focus */}
      {isFocused && (
        <div className="absolute inset-0 rounded-xl bg-gradient-to-b from-[rgba(47,168,255,0.14)] via-transparent to-[rgba(47,168,255,0.08)] pointer-events-none" />
      )}

      {/* Textarea */}
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onFocus={() => setIsFocused(true)}
        onBlur={() => setIsFocused(false)}
        maxLength={maxLength}
        placeholder="Describe the app you want to build...

For example:
- Build a full-stack project management app with kanban boards, drag-and-drop task cards, team member assignment, due dates, priority levels, and real-time progress tracking. Include JWT auth, a REST API backend, and a React dashboard with charts showing sprint velocity and burndown.
- Create a personal finance dashboard that connects to mock bank data, categorizes transactions automatically, shows spending trends with interactive charts, supports budget goals, and sends alerts when limits are exceeded. Use React, TypeScript, and a Node.js API.
- Build a real-time collaborative whiteboard app where multiple users can draw, add sticky notes, shapes, and text together. Include room creation with shareable links, undo/redo history, and export to PNG."
        className={cn(
          "relative w-full h-56 bg-transparent rounded-xl px-5 py-4",
          "text-white placeholder-gray-500 text-base leading-relaxed",
          "focus:outline-none resize-none",
          "transition-all duration-300",
          "z-10"
        )}
      />

      {/* Character count and progress bar */}
      <div className="absolute bottom-4 right-4 flex items-center gap-3 z-20">
        <div className="w-28 h-2 bg-gray-800 rounded-full overflow-hidden border border-gray-700">
          <div
            className={cn(
              "h-full rounded-full transition-all duration-500 relative overflow-hidden",
              progressPercent > 80 ? "bg-orange-500" : progressPercent > 50 ? "bg-yellow-500" : "bg-red-500"
            )}
            style={{ width: `${Math.min(progressPercent, 100)}%` }}
          >
            <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/30 to-transparent" style={{ animation: 'shimmer 1.5s infinite' }} />
          </div>
        </div>
        <span className="text-xs text-gray-500 font-mono tabular-nums">
          {value.length.toLocaleString()}/{maxLength.toLocaleString()}
        </span>
      </div>

      {/* Corner decorations */}
      <div className="absolute top-2 left-2 w-5 h-5 border-t-2 border-l-2 border-[rgba(188,239,255,0.3)] rounded-tl pointer-events-none" />
      <div className="absolute top-2 right-2 w-5 h-5 border-t-2 border-r-2 border-[rgba(188,239,255,0.3)] rounded-tr pointer-events-none" />
      <div className="absolute bottom-12 left-2 w-5 h-5 border-b-2 border-l-2 border-[rgba(188,239,255,0.3)] rounded-bl pointer-events-none" />
      <div className="absolute bottom-12 right-2 w-5 h-5 border-b-2 border-r-2 border-[rgba(188,239,255,0.3)] rounded-br pointer-events-none" />
    </div>
  )
}

// ============================================================================
// TECH STACK CARD COMPONENT
// ============================================================================

interface TechStackCardProps {
  stack: TechStack
  isSelected: boolean
  onClick: () => void
}

const TechStackCard: React.FC<TechStackCardProps> = ({ stack, isSelected, onClick }) => {
  return (
    <button
      onClick={onClick}
      type="button"
      aria-pressed={isSelected}
      className={cn(
        "tech-stack-card relative group min-h-[6rem] sm:min-h-[8.6rem] p-2 sm:p-4 rounded-xl transition-all duration-300 text-left overflow-hidden",
        "border-2 backdrop-blur-sm",
        isSelected
          ? "is-selected border-[rgba(138,223,255,0.48)] bg-[rgba(47,168,255,0.12)] shadow-lg shadow-[rgba(47,168,255,0.18)] sm:scale-[1.02]"
          : "border-gray-800 bg-gray-900/50 hover:border-gray-600 hover:bg-gray-900/70 sm:hover:scale-[1.01]"
      )}
    >
      {/* Holographic scan effect */}
      <div className={cn(
        "absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none",
        "bg-gradient-to-r from-transparent via-white/5 to-transparent"
      )} style={{ animation: 'scan-horizontal 2s linear infinite' }} />

      {/* Selected glow */}
      {isSelected && (
        <>
          <div className="absolute inset-0 bg-gradient-to-br from-[rgba(47,168,255,0.18)] via-transparent to-[rgba(138,223,255,0.14)] pointer-events-none" />
          <div className="absolute -inset-[1px] bg-gradient-to-r from-[#8adfff] via-[#2fa8ff] to-[#8adfff] rounded-xl opacity-30 blur-sm -z-10" />
          <div className="tech-stack-selected-indicator absolute top-2 right-2 z-20 rounded-full border border-[rgba(138,223,255,0.7)] bg-[rgba(4,18,34,0.82)] p-1 text-[#f4fdff] shadow-lg shadow-[rgba(47,168,255,0.22)]">
            <CheckCircle2 className="w-3.5 h-3.5" />
          </div>
        </>
      )}

      {/* Content */}
      <div className="relative z-10 flex items-start gap-2 sm:gap-3">
        <div className={cn(
          "tech-stack-icon w-8 h-8 sm:w-10 sm:h-10 rounded-lg flex items-center justify-center transition-all duration-300 flex-shrink-0",
          isSelected
            ? "bg-[#2fa8ff] text-[#00111d] shadow-lg shadow-[rgba(47,168,255,0.28)]"
            : "bg-gray-800 text-gray-400 group-hover:text-white group-hover:bg-gray-700"
        )}>
          {stack.icon}
        </div>
        <div className="flex-1 min-w-0 pt-0.5">
          <h4 className={cn(
            "tech-stack-title [overflow-wrap:anywhere] break-words font-bold text-xs sm:text-[0.92rem] leading-tight transition-colors",
            isSelected ? "text-white" : "text-gray-200"
          )}>
            {stack.name}
          </h4>
          <p className={cn(
            "tech-stack-description mt-0.5 sm:mt-1 [overflow-wrap:anywhere] break-words text-[10px] sm:text-xs leading-snug transition-colors",
            isSelected ? "text-[#c8f4ff]" : "text-gray-500"
          )}>{stack.description}</p>
        </div>
      </div>
    </button>
  )
}

// ============================================================================
// EPIC BUILD BUTTON COMPONENT
// ============================================================================

interface EpicBuildButtonProps {
  onClick: () => void
  disabled: boolean
  isLoading: boolean
}

const EpicBuildButton: React.FC<EpicBuildButtonProps> = ({ onClick, disabled, isLoading }) => {
  const [ripples, setRipples] = useState<{ id: number; x: number; y: number }[]>([])
  const buttonRef = useRef<HTMLButtonElement>(null)

  const handleClick = (e: React.MouseEvent<HTMLButtonElement>) => {
    if (disabled) return

    // Create ripple effect
    const rect = buttonRef.current?.getBoundingClientRect()
    if (rect) {
      const x = e.clientX - rect.left
      const y = e.clientY - rect.top
      const newRipple = { id: Date.now(), x, y }
      setRipples(prev => [...prev, newRipple])
      setTimeout(() => {
        setRipples(prev => prev.filter(r => r.id !== newRipple.id))
      }, 800)
    }

    onClick()
  }

  return (
    <button
      ref={buttonRef}
      onClick={handleClick}
      disabled={disabled}
      className={cn(
        "launch-build-btn",
        "relative w-full py-5 rounded-2xl font-black text-xl overflow-hidden",
        "transition-all duration-300 transform",
        disabled
          ? "opacity-50 cursor-not-allowed"
          : "hover:scale-[1.02] hover:shadow-2xl hover:shadow-[rgba(47,168,255,0.4)] active:scale-[0.98]"
      )}
    >
      {/* Animated gradient background */}
      <div className={cn(
        "launch-build-bg absolute inset-0 bg-gradient-to-r from-[#1a6fa8] via-[#2fa8ff] to-[#1a6fa8]",
        !disabled && !isLoading && "animate-gradient-shift"
      )} style={{ backgroundSize: '200% auto' }} />

      {/* Pulsing glow effect */}
      {!disabled && !isLoading && (
        <div className="launch-build-glow absolute -inset-1 bg-gradient-to-r from-[#2fa8ff] via-[#8adfff] to-[#2fa8ff] rounded-2xl opacity-50 blur-lg animate-pulse" />
      )}

      {/* Inner shine */}
      <div className="launch-build-shine absolute inset-0 bg-gradient-to-b from-white/25 via-transparent to-black/30" />

      {/* Animated border for loading */}
      {isLoading && (
        <div className="absolute inset-0 rounded-2xl overflow-hidden">
          <div
            className="absolute inset-[-100%] bg-gradient-conic from-[#2fa8ff] via-transparent to-[#2fa8ff]"
            style={{ animation: 'spin 1.5s linear infinite' }}
          />
          <div className="absolute inset-[2px] bg-gradient-to-r from-[#1a6fa8] via-[#2fa8ff] to-[#1a6fa8] rounded-2xl" />
        </div>
      )}

      {/* Ripple effects */}
      {ripples.map(ripple => (
        <span
          key={ripple.id}
          className="absolute bg-white/40 rounded-full pointer-events-none"
          style={{
            left: ripple.x - 20,
            top: ripple.y - 20,
            width: 40,
            height: 40,
            animation: 'ripple-expand 0.8s ease-out forwards',
          }}
        />
      ))}

      {/* Scan line effect when not loading */}
      {!isLoading && !disabled && (
        <div className="absolute inset-0 overflow-hidden rounded-2xl pointer-events-none">
          <div
            className="absolute inset-0 bg-gradient-to-r from-transparent via-white/20 to-transparent w-1/3"
            style={{ animation: 'scan-horizontal 2s linear infinite' }}
          />
        </div>
      )}

      {/* Content */}
      <span className="relative z-10 flex items-center justify-center gap-4 text-white drop-shadow-lg">
        {isLoading ? (
          <>
            <div className="w-7 h-7 border-[3px] border-white/30 border-t-white rounded-full animate-spin" />
            <span className="tracking-wide">INITIALIZING SYSTEMS...</span>
          </>
        ) : (
          <>
            <Rocket className="w-7 h-7" style={{ animation: 'float 2s ease-in-out infinite' }} />
            <span className="tracking-wider">LAUNCH BUILD</span>
            <Sparkles className="w-6 h-6 animate-pulse" />
          </>
        )}
      </span>
    </button>
  )
}

interface PlanUpgradeModalProps {
  currentPlan?: string
  upgrade: UpgradePromptState
  loading?: boolean
  onClose: () => void
  onUpgrade: () => void
}

const PlanUpgradeModal: React.FC<PlanUpgradeModalProps> = ({ currentPlan, upgrade, loading, onClose, onUpgrade }) => {
  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 1000,
        background: 'rgba(0,0,0,0.75)',
        backdropFilter: 'blur(8px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '1rem',
      }}
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onClose()
        }
      }}
    >
      <div
        style={{
          width: '100%',
          maxWidth: 520,
          borderRadius: 18,
          border: '1px solid rgba(239,68,68,0.26)',
          background: 'linear-gradient(180deg, rgba(13,13,13,0.98), rgba(6,6,6,0.98))',
          boxShadow: '0 24px 80px rgba(0,0,0,0.72), 0 0 80px rgba(239,68,68,0.12)',
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            padding: '20px 24px 16px',
            borderBottom: '1px solid rgba(255,255,255,0.08)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 12,
          }}
        >
          <div>
            <div style={{ fontFamily: '"Orbitron", sans-serif', fontWeight: 800, fontSize: '0.95rem', letterSpacing: '0.08em', color: '#f8fafc' }}>
              UPGRADE TO CONTINUE BACKEND WORK
            </div>
            <div style={{ marginTop: 6, fontSize: '0.8rem', color: 'rgba(255,255,255,0.5)' }}>
              The frontend preview stays available now. Runtime implementation unlocks after payment.
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close upgrade prompt"
            style={{
              background: 'none',
              border: 'none',
              color: 'rgba(255,255,255,0.55)',
              cursor: 'pointer',
              fontSize: '1.4rem',
              lineHeight: 1,
              minWidth: 40,
              minHeight: 40,
            }}
          >
            ×
          </button>
        </div>

        <div style={{ padding: '22px 24px 24px', display: 'grid', gap: 16 }}>
          <div
            style={{
              borderRadius: 14,
              border: '1px solid rgba(248,113,113,0.2)',
              background: 'rgba(127,29,29,0.18)',
              padding: '14px 16px',
            }}
          >
            <div style={{ fontSize: '0.72rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: '#fca5a5' }}>
              Current Plan
            </div>
            <div style={{ marginTop: 4, fontSize: '1rem', fontWeight: 700, color: '#fff' }}>
              {(currentPlan || 'free').toUpperCase()} {'->'} {(upgrade.requiredPlan || 'builder').toUpperCase()}
            </div>
            <div style={{ marginTop: 10, fontSize: '0.92rem', lineHeight: 1.55, color: 'rgba(255,255,255,0.78)' }}>
              {upgrade.suggestion}
            </div>
          </div>

          <div style={{ display: 'grid', gap: 10, color: 'rgba(255,255,255,0.72)', fontSize: '0.9rem', lineHeight: 1.55 }}>
            <div>What you keep right now: the preview-first frontend build continues so the app stays visible in the interactive preview pane.</div>
            <div>What payment unlocks: {upgrade.reason || 'backend/runtime implementation'} on this same app without starting over.</div>
            <div>
              {upgrade.source === 'start'
                ? 'This request crossed into paid capability territory during planning, so the system is preserving the frontend preview and deferring runtime work honestly.'
                : 'This follow-up request crossed into paid capability territory, so the system stopped before pretending the backend work happened.'}
            </div>
          </div>

          <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
            <Button
              type="button"
              onClick={onUpgrade}
              disabled={loading}
              className="min-h-[48px] flex-1 border border-red-500/60 bg-red-500/15 text-red-100 hover:bg-red-500/20"
            >
              {loading ? 'Opening Checkout...' : `Upgrade to ${(upgrade.requiredPlan || 'builder').replace(/\b\w/g, (match) => match.toUpperCase())}`}
            </Button>
            <Button
              type="button"
              onClick={onClose}
              disabled={loading}
              variant="outline"
              className="min-h-[48px] flex-1 border border-gray-700 text-gray-300 hover:bg-gray-900"
            >
              Keep Preview Only
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}

// ============================================================================
// AGENT CARD COMPONENT (Animated)
// ============================================================================

interface AgentCardProps {
  agent: Agent
  index: number
  canDirectMessage: boolean
  getAgentEmoji: (role: string) => string
  getStatusIcon: (status: string) => React.ReactNode
  messageDraft: string
  onMessageDraftChange: (agentId: string, value: string) => void
  onSendMessage: (agent: Agent) => void
  recentThoughts: AIThought[]
  sendPending: boolean
}

const AgentCard: React.FC<AgentCardProps> = ({
  agent,
  index,
  canDirectMessage,
  getAgentEmoji,
  getStatusIcon,
  messageDraft,
  onMessageDraftChange,
  onSendMessage,
  recentThoughts,
  sendPending,
}) => {
  return (
    <div
      className={cn(
        "p-4 rounded-xl border-2 transition-all duration-500",
        agent.status === 'working'
          ? "bg-gradient-to-br from-red-950/60 to-orange-950/40 border-red-600/60 shadow-lg shadow-red-900/40"
          : agent.status === 'completed'
          ? "bg-gradient-to-br from-green-950/40 to-emerald-950/30 border-green-600/40"
          : agent.status === 'error'
          ? "bg-gradient-to-br from-orange-950/40 to-red-950/30 border-orange-600/40"
          : "bg-gray-900/60 border-gray-800"
      )}
      style={{
        animation: 'fade-in-up 0.5s ease-out forwards',
        animationDelay: `${index * 100}ms`,
        opacity: 0,
      }}
    >
      <div className="flex items-start gap-4">
        {/* Agent Avatar */}
        <div className={cn(
          "relative w-14 h-14 rounded-xl flex items-center justify-center text-2xl transition-all duration-300",
          agent.status === 'working'
            ? "bg-red-900/50"
            : agent.status === 'completed'
            ? "bg-green-900/40"
            : "bg-gray-800"
        )}>
          {agent.status === 'working' && (
            <>
              <div className="absolute inset-0 rounded-xl border-2 border-red-500/50 animate-ping" style={{ animationDuration: '1.5s' }} />
              <div className="absolute inset-0 rounded-xl bg-red-500/20 animate-pulse" />
            </>
          )}
          <span className="relative z-10">{getAgentEmoji(agent.role)}</span>
        </div>

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <span className="font-bold text-white capitalize text-lg">{agent.role}</span>
            {getStatusIcon(agent.status)}
          </div>
          {agent.currentTask && (
            <p className="text-sm text-gray-400 truncate">
              {agent.currentTask.description}
            </p>
          )}
          {agent.model && (
            <p className="text-xs text-gray-500 mt-1 font-mono truncate">
              Model: {agent.model}
            </p>
          )}

          {/* Progress bar for working agents */}
          {agent.status === 'working' && (
            <div className="mt-2 h-2 bg-gray-800 rounded-full overflow-hidden border border-gray-700">
              <div
                className="h-full bg-gradient-to-r from-red-500 via-orange-500 to-red-500 rounded-full transition-all duration-500 relative overflow-hidden"
                style={{ width: `${agent.progress || 50}%`, backgroundSize: '200% auto', animation: 'gradient-shift 2s linear infinite' }}
              >
                <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/30 to-transparent" style={{ animation: 'shimmer 1s linear infinite' }} />
              </div>
            </div>
          )}
        </div>

        <Badge
          variant="outline"
          className={cn(
            "shrink-0 uppercase text-xs font-bold tracking-wider px-3 py-1",
            agent.provider === 'claude' && "border-orange-500/60 text-orange-400 bg-orange-500/10",
            (agent.provider === 'gpt' || agent.provider === 'gpt4') && "border-green-500/60 text-green-400 bg-green-500/10",
            agent.provider === 'gemini' && "border-blue-500/60 text-blue-400 bg-blue-500/10"
          )}
        >
          {agent.provider}
        </Badge>
      </div>

      <div className="mt-4 space-y-3">
        <div className="rounded-xl border border-gray-800 bg-black/35 px-3 py-3">
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-gray-500">
              <MessageSquare className="w-3.5 h-3.5" />
              Direct Control
            </div>
            <Badge
              variant="outline"
              className={cn(
                'text-[10px]',
                canDirectMessage
                  ? 'border-cyan-500/40 bg-cyan-500/10 text-cyan-200'
                  : 'border-gray-700 bg-gray-900/70 text-gray-500'
              )}
            >
              {canDirectMessage ? 'Live' : 'Read Only'}
            </Badge>
          </div>
          <div className="mt-2 text-xs text-gray-400">
            {canDirectMessage
              ? 'Send an instruction straight to this agent. It stays visible in the planner timeline and this agent’s telemetry.'
              : 'Direct agent messaging is only enabled while the build is actively running.'}
          </div>
          <div className="mt-3 flex gap-2">
            <input
              type="text"
              value={messageDraft}
              onChange={(event) => onMessageDraftChange(agent.id, event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  onSendMessage(agent)
                }
              }}
              placeholder={`Message ${humanizeIdentifier(agent.role)} directly...`}
              disabled={!canDirectMessage || sendPending}
              className="flex-1 rounded-lg border border-gray-700 bg-black px-3 py-2 text-sm text-white placeholder:text-gray-600 focus:border-cyan-500 focus:outline-none focus:ring-2 focus:ring-cyan-900/30 disabled:cursor-not-allowed disabled:opacity-50"
            />
            <Button
              size="sm"
              onClick={() => onSendMessage(agent)}
              disabled={!canDirectMessage || !messageDraft.trim() || sendPending}
              className="bg-cyan-600 hover:bg-cyan-500"
            >
              <Send className="w-4 h-4" />
            </Button>
          </div>
        </div>

        <div className="rounded-xl border border-gray-800 bg-black/35 px-3 py-3">
          <div className="text-[11px] uppercase tracking-[0.18em] text-gray-500">Recent Visible Activity</div>
          {recentThoughts.length === 0 ? (
            <div className="mt-2 text-xs text-gray-500">
              No visible telemetry from this agent yet.
            </div>
          ) : (
            <div className="mt-3 space-y-2">
              {recentThoughts.map((thought) => (
                <div key={thought.id} className="rounded-lg border border-gray-800 bg-black/40 px-3 py-2">
                  <div className="flex items-center gap-2 text-[10px] text-gray-500">
                    <span className="font-mono">
                      {thought.timestamp.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                    </span>
                    <Badge variant="outline" className="border-white/10 bg-white/5 text-[10px] text-gray-300">
                      {getThoughtEventLabel(thought)}
                    </Badge>
                  </div>
                  <div className="mt-1 text-xs leading-relaxed text-gray-200">
                    {thought.content}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ============================================================================
// BUILD COMPLETE CELEBRATION
// ============================================================================

const BuildCompleteCard: React.FC<{
  filesCount: number
  onPreviewWorkspace: () => void
  onOpenIDE: () => void
  onDownload: () => void
  onStartOver: () => void
  isCreating: boolean
  isPreparingPreview: boolean
  isResetting: boolean
  isPreviewReady: boolean
}> = ({
  filesCount,
  onPreviewWorkspace,
  onOpenIDE,
  onDownload,
  onStartOver,
  isCreating,
  isPreparingPreview,
  isResetting,
  isPreviewReady
}) => {
  return (
    <Card variant="cyberpunk" className="relative overflow-hidden border-2 border-green-500/60 bg-gradient-to-br from-green-950/40 via-black to-emerald-950/30">
      {/* Success particles */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        {Array.from({ length: 15 }).map((_, i) => (
          <div
            key={i}
            className="absolute w-1.5 h-1.5 bg-green-400 rounded-full"
            style={{
              left: `${Math.random() * 100}%`,
              top: `${Math.random() * 100}%`,
              animation: `sparkle ${1 + Math.random()}s ease-out infinite`,
              animationDelay: `${Math.random() * 2}s`,
            }}
          />
        ))}
      </div>

      <CardContent className="p-6 relative z-10">
        <div className="space-y-5">
          <div className="rounded-xl border border-green-400/30 bg-green-400/10 px-4 py-3 flex items-center justify-between gap-3 flex-wrap">
            <div className="flex items-center gap-2 text-green-200">
              <Sparkles className="w-4 h-4 text-green-300" />
              <span className="text-sm font-semibold">
                Preview, ZIP export, and editor handoff are ready.
              </span>
            </div>
            {isPreviewReady && (
              <span className="text-xs font-mono text-green-300/90">preview primed</span>
            )}
          </div>

          <div className="flex items-center justify-between flex-wrap gap-4">
            <div className="flex items-center gap-4">
              <div className="relative shrink-0">
                <CheckCircle2 className="w-12 h-12 text-green-400" style={{ animation: 'bounce-slow 2s ease-in-out infinite' }} />
                <div className="absolute inset-0 bg-green-400/40 rounded-full blur-xl animate-pulse" />
              </div>
              <div>
                <h3 className="font-black text-2xl text-white mb-0.5">Build Complete!</h3>
                <p className="text-green-400 font-mono text-base">
                  {filesCount} files generated
                </p>
              </div>
            </div>
            <div className="flex gap-3 flex-wrap items-center">
              <Button
                size="lg"
                className={cn(
                  "bg-gradient-to-r from-green-500 via-emerald-500 to-lime-500 text-black font-black shadow-xl shadow-green-900/60 border border-green-300/40 px-6",
                  "hover:from-green-400 hover:via-emerald-400 hover:to-lime-400",
                  !isPreparingPreview && "animate-pulse"
                )}
                onClick={onPreviewWorkspace}
                disabled={isPreparingPreview}
              >
                {isPreparingPreview ? (
                  <>
                    <Clock className="w-5 h-5 mr-2 animate-spin" />
                    {isPreviewReady ? 'Launching Preview...' : 'Preparing Preview...'}
                  </>
                ) : (
                  <>
                    <Eye className="w-5 h-5 mr-2" />
                    Launch Preview Workspace
                  </>
                )}
              </Button>
              <Button
                variant="outline"
                size="lg"
                className={cn(
                  "border-2 border-red-600 text-red-400 hover:bg-red-950/50 transition-all font-semibold"
                )}
                onClick={onDownload}
                disabled={filesCount === 0}
              >
                <Download className="w-5 h-5 mr-2" />
                Download ZIP
              </Button>
              <Button
                variant="outline"
                size="lg"
                className="border-2 border-gray-700 text-gray-300 hover:bg-gray-800/60 transition-all font-semibold"
                onClick={onOpenIDE}
                disabled={isCreating}
              >
                <ExternalLink className="w-5 h-5 mr-2" />
                {isCreating ? 'Creating Project...' : 'Open Editor'}
              </Button>
              <Button
                variant="outline"
                size="lg"
                className="border-2 border-gray-700 text-gray-300 hover:bg-gray-800/60 transition-all font-semibold"
                onClick={onStartOver}
                disabled={isResetting}
              >
                <RotateCcw className={cn("w-5 h-5 mr-2", isResetting && "animate-spin")} />
                {isResetting ? 'Starting Fresh...' : 'Start Fresh'}
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

// ============================================================================
// TERMINAL OUTPUT COMPONENT
// ============================================================================

const TerminalOutput: React.FC<{ messages: ChatMessage[]; isBuilding: boolean }> = ({ messages, isBuilding }) => {
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    containerRef.current?.scrollTo({ top: containerRef.current.scrollHeight, behavior: 'smooth' })
  }, [messages.length])

  return (
    <div
      ref={containerRef}
      className="bg-black/90 rounded-xl p-4 font-mono text-sm h-72 overflow-y-auto border border-gray-800"
    >
      {/* Terminal header */}
      <div className="flex items-center gap-2 mb-4 pb-3 border-b border-gray-800">
        <div className="w-3 h-3 rounded-full bg-red-500 shadow-lg shadow-red-500/50" />
        <div className="w-3 h-3 rounded-full bg-yellow-500 shadow-lg shadow-yellow-500/50" />
        <div className="w-3 h-3 rounded-full bg-green-500 shadow-lg shadow-green-500/50" />
        <span className="ml-3 text-gray-500 text-xs tracking-wider uppercase">APEX Build Terminal</span>
      </div>

      {messages.map((msg, index) => (
        <div
          key={msg.id}
          className={cn(
            "flex items-start gap-2 mb-2",
            msg.role === 'system' && "text-gray-400",
            msg.role === 'lead' && "text-orange-400",
            msg.role === 'user' && "text-cyan-400",
            msg.status === 'failed' && "text-red-300",
            msg.status === 'pending' && "opacity-80"
          )}
          style={{ animation: 'fade-in 0.2s ease-out', animationDelay: `${index * 30}ms` }}
        >
          <span className="text-red-500 select-none font-bold">{'>'}</span>
          <span className="flex-1 break-words">
            <span className="mr-2 text-[10px] font-semibold uppercase tracking-[0.2em] text-gray-500">
              [{getConversationRouteLabel(msg)}]
            </span>
            {msg.kind === 'directive' && (
              <span className="mr-2 text-[10px] uppercase tracking-[0.18em] text-cyan-400">
                directive
              </span>
            )}
            {msg.content}
            {msg.status === 'pending' && <span className="ml-2 text-[10px] text-yellow-400">sending</span>}
            {msg.status === 'failed' && <span className="ml-2 text-[10px] text-red-400">failed</span>}
          </span>
          <span className="text-gray-600 text-xs shrink-0">{msg.timestamp.toLocaleTimeString()}</span>
        </div>
      ))}

      {/* Blinking cursor */}
      {isBuilding && (
        <div className="flex items-center gap-2 text-red-500">
          <span className="font-bold">{'>'}</span>
          <span className="w-2.5 h-5 bg-red-500 animate-pulse" />
        </div>
      )}
    </div>
  )
}

const BuilderControlSurface: React.FC<{
  onImportReplit: () => void
  onImportGitHub: () => void
  onAttachImage: () => void
  onOpenIDE: () => void
  onOpenSettings?: () => void
  onOpenBilling?: () => void
}> = ({ onImportReplit, onImportGitHub, onAttachImage, onOpenIDE, onOpenSettings, onOpenBilling }) => {
  const controls: Array<{
    title: string
    body: string
    icon: React.ReactNode
    actionLabel: string
    onClick?: () => void
  }> = [
    {
      title: 'GitHub import/export',
      body: 'Bring in a repo now; push or download generated code after the build.',
      icon: <Github className="w-5 h-5" />,
      actionLabel: 'Import GitHub',
      onClick: onImportGitHub,
    },
    {
      title: 'Replit migration',
      body: 'Analyze an existing Replit app and rebuild it with Apex contracts.',
      icon: <Download className="w-5 h-5" />,
      actionLabel: 'Import Replit',
      onClick: onImportReplit,
    },
    {
      title: 'Files and images',
      body: 'Attach wireframes, screenshots, ZIPs, and assets as agent context.',
      icon: <Upload className="w-5 h-5" />,
      actionLabel: 'Attach image',
      onClick: onAttachImage,
    },
    {
      title: 'BYOK model roles',
      body: 'OpenAI, Claude, Gemini, Grok, Kimi, and Ollama keys stay configurable.',
      icon: <KeyRound className="w-5 h-5" />,
      actionLabel: 'Open settings',
      onClick: onOpenSettings,
    },
    {
      title: 'MCP connectors',
      body: 'Connect external tools and APIs through the project integration surface.',
      icon: <PlugZap className="w-5 h-5" />,
      actionLabel: 'Open IDE',
      onClick: onOpenIDE,
    },
    {
      title: 'Secrets vault',
      body: 'Store API keys, OAuth values, DB URLs, SSH credentials, and env vars.',
      icon: <LockKeyhole className="w-5 h-5" />,
      actionLabel: 'Open IDE',
      onClick: onOpenIDE,
    },
    {
      title: 'Deploy and preview',
      body: 'Preview, verify, export, and deploy from the workspace after generation.',
      icon: <MonitorUp className="w-5 h-5" />,
      actionLabel: 'Open IDE',
      onClick: onOpenIDE,
    },
    {
      title: 'Billing controls',
      body: 'Budget caps, credits, trials, and live spend stay visible before runs.',
      icon: <CreditCard className="w-5 h-5" />,
      actionLabel: 'Billing',
      onClick: onOpenBilling,
    },
  ]

  return (
    <section className="builder-control-surface mb-6 md:mb-8">
      <div className="builder-control-surface__header">
        <div>
          <div className="builder-control-surface__kicker">Production control surface</div>
          <h2>Everything around the prompt is configurable.</h2>
        </div>
        <div className="builder-control-surface__badges" aria-label="Included surfaces">
          <span>Monaco IDE</span>
          <span>Terminal</span>
          <span>Git</span>
          <span>Deploy</span>
          <span>BYOK</span>
        </div>
      </div>

      <div className="builder-control-surface__grid">
        {controls.map((control) => {
          const content = (
            <>
              <div className="builder-control-surface__icon">{control.icon}</div>
              <div className="min-w-0">
                <h3>{control.title}</h3>
                <p>{control.body}</p>
                <span>{control.actionLabel}</span>
              </div>
            </>
          )

          return (
            <button
              key={control.title}
              type="button"
              className="builder-control-surface__card"
              onClick={control.onClick}
              disabled={!control.onClick}
            >
              {content}
            </button>
          )
        })}
      </div>
    </section>
  )
}

const BUILDER_LAUNCH_PRESETS = [
  {
    title: 'Realtime crypto tracker',
    body: 'Top 15 coins, price chart, market cap, volume, and BUY / SELL / HOLD signals.',
    prompt: 'Build a real-time cryptocurrency price tracker for the top 15 cryptocurrencies using the CoinGecko free API. Show live prices, 24h % change, market cap, volume, and a clean dashboard. Add buy/sell signal logic based on RSI (14-period), 20/50 EMA crossover, and a 24h volume spike above 2x average. Show each coin signal as BUY / SELL / HOLD with color coding and auto-refresh every 30 seconds. No API key required.',
  },
  {
    title: 'SaaS analytics cockpit',
    body: 'Traffic, funnels, retention, conversion, alerts, and multi-site reporting.',
    prompt: 'Build a SaaS analytics dashboard with multi-site tracking, pageviews, unique visitors, bounce rate, average session duration, top pages, and referrer sources. Include a date range picker, realtime visitor count, and interactive charts for traffic over time, top pages, and geography.',
  },
  {
    title: 'Ops control center',
    body: 'Incidents, deploys, runbooks, budgets, and team approvals in one workspace.',
    prompt: 'Build an operations control center with incident timelines, service health, deployment history, on-call ownership, budget thresholds, and approval workflows. Include role-based access, audit logs, and a responsive dark interface.',
  },
] as const

interface BuilderLaunchpadProps {
  buildMode: BuildMode
  powerMode: 'fast' | 'balanced' | 'max'
  appDescription: string
  promptMaxLength: number
  isBuilding: boolean
  isRoleAssignmentValid: boolean
  maxPowerPromptLimitEnabled: boolean
  balancedPromptLimitEnabled: boolean
  wireframeImage: string
  createdProjectId: number | null
  selectedStack: Set<string>
  autoStackId: string
  techStacks: TechStack[]
  roleConfigMode: 'auto' | 'manual'
  roleAssignments: Record<string, string>
  providerStatuses: Record<string, string>
  userPlan?: string
  userId?: number | null
  wireframeInputRef: React.RefObject<HTMLInputElement | null>
  onSetBuildMode: (mode: BuildMode) => void
  onSetPowerMode: (mode: 'fast' | 'balanced' | 'max') => void
  onSetAppDescription: (value: string) => void
  onSetWireframeImage: (value: string) => void
  onClearWireframe: () => void
  onToggleStack: (id: string) => void
  onSetRoleConfigMode: (mode: 'auto' | 'manual') => void
  onSetRoleAssignments: (assignments: Record<string, string>) => void
  onStartBuild: () => void
  onImportReplit: () => void
  onImportGitHub: () => void
  onOpenIDE: () => void
  onOpenSettings: () => void
  onOpenBilling: () => void
  onBuildHistoryOpen?: (buildId: string, action?: 'resume' | 'open_files') => void
  buildTechStackSummary: () => string
}

const BuilderLaunchpad: React.FC<BuilderLaunchpadProps> = ({
  buildMode,
  powerMode,
  appDescription,
  promptMaxLength,
  isBuilding,
  isRoleAssignmentValid,
  maxPowerPromptLimitEnabled,
  balancedPromptLimitEnabled,
  wireframeImage,
  createdProjectId,
  selectedStack,
  autoStackId,
  techStacks,
  roleConfigMode,
  roleAssignments,
  providerStatuses,
  userPlan,
  userId,
  wireframeInputRef,
  onSetBuildMode,
  onSetPowerMode,
  onSetAppDescription,
  onSetWireframeImage,
  onClearWireframe,
  onToggleStack,
  onSetRoleConfigMode,
  onSetRoleAssignments,
  onStartBuild,
  onImportReplit,
  onImportGitHub,
  onOpenIDE,
  onOpenSettings,
  onOpenBilling,
  onBuildHistoryOpen,
  buildTechStackSummary,
}) => {
  const planAllowsBackend = ['builder', 'pro', 'team', 'enterprise', 'owner'].includes(userPlan || '')

  return (
    <div className="mx-auto max-w-7xl space-y-8">
      <section className="overflow-hidden rounded-[28px] border border-[rgba(188,239,255,0.18)] bg-[linear-gradient(135deg,rgba(5,12,24,0.96),rgba(6,18,40,0.88),rgba(4,8,18,0.98))] px-6 py-8 shadow-[0_28px_90px_rgba(2,8,20,0.55)] sm:px-8 sm:py-10 md:px-10">
        <div className="grid gap-8 lg:grid-cols-[minmax(0,1.2fr),minmax(320px,0.8fr)] lg:items-end">
          <div className="space-y-5">
            <div className="inline-flex items-center gap-2 rounded-full border border-[rgba(138,223,255,0.35)] bg-[rgba(47,168,255,0.08)] px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.24em] text-[#8adfff]">
              Authenticated Build Surface
            </div>
            <div className="space-y-3">
              <h1 className="max-w-4xl text-4xl font-black tracking-[-0.04em] text-white sm:text-5xl lg:text-6xl">
                One builder. One control surface. No fallback to the legacy shell.
              </h1>
              <p className="max-w-3xl text-sm leading-7 text-[#9eb3c7] sm:text-base">
                Describe the product, choose the build depth, lock the model routing, attach visual references, and launch the workflow without dropping back into the older card stack.
              </p>
            </div>
            <div className="flex flex-wrap gap-2 text-[11px] font-medium text-[#c8f4ff]">
              {['Realtime orchestration', 'Frontend + backend', 'BYOK + hosted providers', 'Preview / IDE / deploy'].map((item) => (
                <span
                  key={item}
                  className="rounded-full border border-white/10 bg-white/[0.04] px-3 py-1.5"
                >
                  {item}
                </span>
              ))}
            </div>
          </div>

          <div className="rounded-[24px] border border-white/10 bg-[rgba(7,15,32,0.78)] p-5 backdrop-blur-xl">
            <div className="text-[11px] uppercase tracking-[0.22em] text-[#6f89a3]">Launch policy</div>
            <div className="mt-3 space-y-3 text-sm leading-6 text-[#c7d4e2]">
              <p>
                {planAllowsBackend
                  ? 'Your plan can execute full-stack work: frontend, backend, database, auth, billing, realtime, preview, and deploy handoff.'
                  : 'Free stays honest: frontend-only builds and static experiences complete without pretending backend, auth, billing, or database work happened.'}
              </p>
              <div className="rounded-2xl border border-[rgba(138,223,255,0.18)] bg-[rgba(47,168,255,0.07)] px-4 py-3 text-xs text-[#9ecfe4]">
                Build mode controls scope. Power mode controls model quality and orchestration depth. Both remain visible before you spend anything.
              </div>
            </div>
          </div>
        </div>
      </section>

      <BuilderControlSurface
        onImportReplit={onImportReplit}
        onImportGitHub={onImportGitHub}
        onAttachImage={() => wireframeInputRef.current?.click()}
        onOpenIDE={onOpenIDE}
        onOpenSettings={onOpenSettings}
        onOpenBilling={onOpenBilling}
      />

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.2fr),380px]">
        <div className="space-y-6">
          <section className="rounded-[28px] border border-[rgba(188,239,255,0.16)] bg-[rgba(6,14,28,0.78)] p-5 shadow-[0_20px_60px_rgba(2,8,20,0.32)] backdrop-blur-xl sm:p-6 md:p-8">
            <div className="flex flex-col gap-6">
              <div className="flex flex-wrap gap-3">
                {([
                  { id: 'fast' as const, label: 'Fast Build', hint: '~3-5 min' },
                  { id: 'full' as const, label: 'Full Build', hint: '10+ min' },
                ]).map((mode) => (
                  <button
                    key={mode.id}
                    type="button"
                    onClick={() => onSetBuildMode(mode.id)}
                    className={cn(
                      'min-w-[11rem] rounded-2xl border px-4 py-3 text-left transition-all duration-200',
                      buildMode === mode.id
                        ? 'border-[rgba(138,223,255,0.45)] bg-[rgba(47,168,255,0.12)] text-white shadow-[0_0_24px_rgba(47,168,255,0.16)]'
                        : 'border-white/10 bg-white/[0.03] text-[#9eb3c7] hover:border-white/20 hover:text-white'
                    )}
                  >
                    <div className="text-sm font-semibold">{mode.label}</div>
                    <div className="mt-1 text-xs text-inherit/70">{mode.hint}</div>
                  </button>
                ))}
              </div>

              <div>
                <div className="mb-3 flex items-center justify-between gap-3">
                  <div>
                    <div className="text-[11px] uppercase tracking-[0.22em] text-[#6f89a3]">Build brief</div>
                    <h2 className="mt-1 text-2xl font-black tracking-[-0.03em] text-white">Describe the app exactly once.</h2>
                  </div>
                  <div className="rounded-full border border-white/10 bg-black/20 px-3 py-1 text-xs text-[#9eb3c7]">
                    {buildMode === 'full' ? 'Full orchestration path' : 'Fast delivery path'}
                  </div>
                </div>
                <PremiumTextarea
                  value={appDescription}
                  onChange={onSetAppDescription}
                  maxLength={promptMaxLength}
                />
                <p className="mt-3 text-xs leading-6 text-[#7e97ad]">
                  {maxPowerPromptLimitEnabled
                    ? `Full Build with Max Power supports up to ${FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()} characters.`
                    : balancedPromptLimitEnabled
                      ? `Full Build with Balanced supports up to ${BALANCED_FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()} characters. Switch to Max Power for ${FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()}.`
                      : buildMode === 'full'
                        ? `Full Build stays at ${FAST_BUILD_PROMPT_MAX_LENGTH.toLocaleString()} characters on Fast power. Switch to Balanced for ${BALANCED_FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()} or Max Power for ${FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()}.`
                        : `Fast Build stays at ${FAST_BUILD_PROMPT_MAX_LENGTH.toLocaleString()} characters. Switch to Full Build with Balanced for ${BALANCED_FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()} or Max Power for ${FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()}.`}
                </p>
              </div>

              <div className="grid gap-3 md:grid-cols-3">
                {BUILDER_LAUNCH_PRESETS.map((preset) => (
                  <button
                    key={preset.title}
                    type="button"
                    onClick={() => onSetAppDescription(preset.prompt)}
                    className="rounded-2xl border border-white/10 bg-white/[0.03] p-4 text-left transition-all hover:border-[rgba(138,223,255,0.35)] hover:bg-[rgba(47,168,255,0.08)]"
                  >
                    <div className="text-sm font-semibold text-white">{preset.title}</div>
                    <p className="mt-2 text-xs leading-6 text-[#90a5bb]">{preset.body}</p>
                  </button>
                ))}
              </div>

              <div className="rounded-2xl border border-white/10 bg-black/20 p-4">
                <div className="mb-2 text-[11px] uppercase tracking-[0.22em] text-[#6f89a3]">Visual reference</div>
                <input
                  ref={wireframeInputRef as React.RefObject<HTMLInputElement>}
                  type="file"
                  accept="image/*"
                  className="hidden"
                  onChange={(e) => {
                    const file = e.target.files?.[0]
                    if (!file) return
                    const reader = new FileReader()
                    reader.onload = (ev) => {
                      const result = ev.target?.result as string
                      if (result) onSetWireframeImage(result)
                    }
                    reader.readAsDataURL(file)
                    e.target.value = ''
                  }}
                />
                {wireframeImage ? (
                  <div className="flex items-center gap-3">
                    <img src={wireframeImage} alt="Wireframe preview" className="h-14 w-14 rounded-xl border border-white/10 object-cover" />
                    <div className="flex-1 text-sm text-[#b8d6e6]">Wireframe attached. The next build will include it as visual context.</div>
                    <button
                      type="button"
                      onClick={onClearWireframe}
                      className="rounded-lg border border-white/10 px-3 py-1.5 text-xs font-medium text-[#9eb3c7] hover:border-white/20 hover:text-white"
                    >
                      Remove
                    </button>
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={() => wireframeInputRef.current?.click()}
                    className="inline-flex items-center gap-2 rounded-lg border border-[rgba(138,223,255,0.24)] bg-[rgba(47,168,255,0.06)] px-3 py-2 text-sm font-medium text-[#c8f4ff] hover:bg-[rgba(47,168,255,0.12)]"
                  >
                    <Upload className="h-4 w-4" />
                    Attach screenshot or wireframe
                  </button>
                )}
              </div>

              {createdProjectId && (
                <div className="rounded-2xl border border-white/10 bg-black/20 p-4">
                  <div className="mb-3 text-[11px] uppercase tracking-[0.22em] text-[#6f89a3]">Project files</div>
                  <AssetUploader projectId={createdProjectId} />
                  <p className="mt-2 text-xs text-[#7e97ad]">
                    Uploaded files are kept in project context and can be used by the next orchestration pass.
                  </p>
                </div>
              )}

              <div className="flex flex-col gap-3 sm:flex-row">
                <Button
                  onClick={onStartBuild}
                  disabled={!appDescription.trim() || !isRoleAssignmentValid || isBuilding}
                  size="lg"
                  className="h-14 flex-1 rounded-2xl border border-[rgba(188,239,255,0.4)] bg-gradient-to-r from-[#f4fdff] via-[#8adfff] to-[#2fa8ff] text-[#00111d] shadow-[0_0_26px_rgba(47,168,255,0.22)] hover:from-white hover:via-[#9fe6ff] hover:to-[#45b5ff]"
                >
                  <Rocket className="mr-2 h-5 w-5" />
                  {isBuilding ? 'Launching orchestration…' : 'Start build'}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="lg"
                  onClick={onImportGitHub}
                  className="h-14 rounded-2xl border-white/10 bg-white/[0.03] px-5 text-[#c8d5e3] hover:border-white/20 hover:bg-white/[0.06] hover:text-white"
                >
                  <Github className="mr-2 h-4 w-4" />
                  Import GitHub
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="lg"
                  onClick={onImportReplit}
                  className="h-14 rounded-2xl border-white/10 bg-white/[0.03] px-5 text-[#c8d5e3] hover:border-white/20 hover:bg-white/[0.06] hover:text-white"
                >
                  <Download className="mr-2 h-4 w-4" />
                  Import Replit
                </Button>
              </div>
            </div>
          </section>

          <BuildHistory userId={userId} onOpenBuild={onBuildHistoryOpen} />
        </div>

        <div className="space-y-6">
          <section className="rounded-[24px] border border-white/10 bg-[rgba(7,15,32,0.78)] p-5 backdrop-blur-xl">
            <div className="mb-4 text-[11px] uppercase tracking-[0.22em] text-[#6f89a3]">Power mode</div>
            <div className="space-y-3">
              {([
                { id: 'fast' as const, label: 'Fast', copy: 'Lowest cost, fastest routing, smaller models first.' },
                { id: 'balanced' as const, label: 'Balanced', copy: 'Best cost / quality tradeoff for most builds.' },
                { id: 'max' as const, label: 'Max Power', copy: 'Highest quality orchestration with premium models and deeper review.' },
              ]).map((mode) => (
                <button
                  key={mode.id}
                  type="button"
                  onClick={() => onSetPowerMode(mode.id)}
                  className={cn(
                    'w-full rounded-2xl border px-4 py-3 text-left transition-all',
                    powerMode === mode.id
                      ? 'border-[rgba(138,223,255,0.45)] bg-[rgba(47,168,255,0.1)] text-white shadow-[0_0_18px_rgba(47,168,255,0.14)]'
                      : 'border-white/10 bg-white/[0.03] text-[#9eb3c7] hover:border-white/20 hover:text-white'
                  )}
                >
                  <div className="flex items-center justify-between gap-3">
                    <span className="font-semibold">{mode.label}</span>
                    <span className="text-[11px] uppercase tracking-[0.18em] text-[#8adfff]">
                      {mode.id === 'fast' ? '1.5x' : mode.id === 'balanced' ? '1.68x' : '1.88x'}
                    </span>
                  </div>
                  <p className="mt-2 text-xs leading-6 text-inherit/75">{mode.copy}</p>
                </button>
              ))}
            </div>
          </section>

          <section className="rounded-[24px] border border-white/10 bg-[rgba(7,15,32,0.78)] p-5 backdrop-blur-xl">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div className="text-[11px] uppercase tracking-[0.22em] text-[#6f89a3]">Tech stack targets</div>
              <span className="text-[11px] text-[#8adfff]">{buildTechStackSummary()}</span>
            </div>
            <div className="grid grid-cols-2 gap-3">
              {techStacks.map((stack) => (
                <TechStackCard
                  key={stack.id}
                  stack={stack}
                  isSelected={selectedStack.has(stack.id) || (stack.id === autoStackId && selectedStack.size === 0)}
                  onClick={() => onToggleStack(stack.id)}
                />
              ))}
            </div>
          </section>

          <ModelRoleConfig
            mode={roleConfigMode}
            onModeChange={onSetRoleConfigMode}
            assignments={roleAssignments}
            onAssignmentsChange={onSetRoleAssignments}
            providerStatuses={providerStatuses}
          />
        </div>
      </div>
    </div>
  )
}

// ============================================================================
// MAIN APP BUILDER COMPONENT
// ============================================================================

export const AppBuilder: React.FC<AppBuilderProps> = ({ onNavigateToIDE, onNavigateToView, startOverSignal }) => {
  // Build state
  const [buildMode, setBuildMode] = useState<BuildMode>('full')
  const [appDescription, setAppDescription] = useState('')
  const [wireframeImage, setWireframeImage] = useState<string>('')
  const wireframeInputRef = useRef<HTMLInputElement>(null)
  const [buildState, setBuildState] = useState<BuildState | null>(null)
  const [isBuilding, setIsBuilding] = useState(false)
  const [showChat, setShowChat] = useState(true)
  const [isPreparingPreview, setIsPreparingPreview] = useState(false)
  const [generatedFiles, setGeneratedFiles] = useState<Array<{ path: string; content: string; language: string }>>([])
  const [createdProjectId, setCreatedProjectId] = useState<number | null>(null)
  const [isCreatingProject, setIsCreatingProject] = useState(false)
  const [telemetryNow, setTelemetryNow] = useState(() => Date.now())
  const AUTO_STACK_ID = 'auto'
  const [selectedStack, setSelectedStack] = useState<Set<string>>(new Set([AUTO_STACK_ID]))
  const [powerMode, setPowerMode] = useState<'fast' | 'balanced' | 'max'>('fast')
  const promptMaxLength = getBuildPromptMaxLength(buildMode, powerMode)
  const maxPowerPromptLimitEnabled = buildMode === 'full' && powerMode === 'max'
  const balancedPromptLimitEnabled = buildMode === 'full' && powerMode === 'balanced'

  // Model role assignment state
  const [roleConfigMode, setRoleConfigMode] = useState<'auto' | 'manual'>('auto')
  const [roleAssignments, setRoleAssignments] = useState<Record<string, string>>({})
  const [providerModelOverrides, setProviderModelOverrides] = useState<Record<SupportedBuildProvider, string>>(DEFAULT_PROVIDER_MODEL_SELECTIONS)
  const [providerModelPendingProvider, setProviderModelPendingProvider] = useState<SupportedBuildProvider | null>(null)
  const [providerStatuses, setProviderStatuses] = useState<Record<string, string>>({})
  const [hasBYOK, setHasBYOK] = useState(false)

  // Chat state
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([])
  const [chatInput, setChatInput] = useState('')
  const [plannerSendMode, setPlannerSendMode] = useState<BuildMessageTargetMode>('lead')
  const [agentMessageDrafts, setAgentMessageDrafts] = useState<Record<string, string>>({})
  const [agentMessagePendingId, setAgentMessagePendingId] = useState<string | null>(null)
  const [plannerMessagePending, setPlannerMessagePending] = useState(false)
  const [permissionActionId, setPermissionActionId] = useState<string | null>(null)
  const [buildActionPending, setBuildActionPending] = useState<'pause' | 'resume' | 'restart' | null>(null)
  const [buildWorkspaceView, setBuildWorkspaceView] = useState<BuildWorkspaceView>('overview')
  const [platformReadiness, setPlatformReadiness] = useState<FeatureReadinessSummary | null>(null)
  const [proposedEdits, setProposedEdits] = useState<ProposedEdit[]>([])
  const [showDiffReview, setShowDiffReview] = useState(true)
  const [spendToasts, setSpendToasts] = useState<Array<{ id: string; agentRole: string; cost: number }>>([])

  const addSpendToast = useCallback((agentRole: string, cost: number) => {
    const id = `${Date.now()}-${Math.random()}`
    setSpendToasts(prev => [...prev.slice(-4), { id, agentRole, cost }])
  }, [])

  const dismissSpendToast = useCallback((id: string) => {
    setSpendToasts(prev => prev.filter(t => t.id !== id))
  }, [])
  const [patchBundleActionId, setPatchBundleActionId] = useState<string | null>(null)
  const [promptProposalActionId, setPromptProposalActionId] = useState<string | null>(null)

  // AI Activity state
  const [aiThoughts, setAiThoughts] = useState<AIThought[]>([])
  const providerActivityRefs = useRef<Record<SupportedBuildProvider, HTMLDivElement | null>>({
    claude: null,
    gpt4: null,
    gemini: null,
    grok: null,
    ollama: null,
  })
  const previewPreparedRef = useRef(false)
  const [showBuyCredits, setShowBuyCredits] = useState(false)
  const [buyCreditsReason, setBuyCreditsReason] = useState<string | undefined>(undefined)
  const [upgradePrompt, setUpgradePrompt] = useState<UpgradePromptState | null>(null)
  const [upgradeCheckoutPending, setUpgradeCheckoutPending] = useState(false)
  const [showImportModal, setShowImportModal] = useState(false)
  const [showGitHubImport, setShowGitHubImport] = useState(false)
  const [replitUrl, setReplitUrl] = useState('')
  const [isImporting, setIsImporting] = useState(false)
  const [rollbackCheckpointId, setRollbackCheckpointId] = useState<string | null>(null)
  const [isStartingOver, setIsStartingOver] = useState(false)
  const plannerInputRef = useRef<HTMLInputElement | null>(null)
  const { user, currentProject, createProject, setCurrentProject, addNotification } = useStore()

  // WebSocket
  const wsRef = useRef<WebSocket | null>(null)
  const wsBuildIdRef = useRef<string | null>(null)
  const wsMessageHandlerRef = useRef<(message: any) => Promise<void>>(async () => {})
  const connectWebSocketRef = useRef<(buildId: string, providedUrl?: string) => void>(() => {})
  const chatEndRef = useRef<HTMLDivElement>(null)
  const wsReconnectAttempts = useRef(0)
  const maxWsReconnectAttempts = 8

  // Ref to track current isBuilding state (prevents stale closure in WebSocket onclose)
  const isBuildingRef = useRef(isBuilding)
  useEffect(() => {
    isBuildingRef.current = isBuilding
  }, [isBuilding])

  const generatedFilesRef = useRef(generatedFiles)
  useEffect(() => {
    generatedFilesRef.current = generatedFiles
  }, [generatedFiles])

  const buildStateRef = useRef<BuildState | null>(buildState)
  useEffect(() => {
    buildStateRef.current = buildState
  }, [buildState])

  const dismissUpgradePrompt = useCallback(() => {
    setUpgradePrompt(null)
    setUpgradeCheckoutPending(false)
  }, [])

  const openUpgradePrompt = useCallback((payload: {
    reason?: string
    suggestion?: string
    requiredPlan?: string
    buildId?: string
    source: 'start' | 'message'
  }) => {
    const requiredPlan = (payload.requiredPlan || 'builder').trim() || 'builder'
    const reason = (payload.reason || 'backend/runtime implementation').trim() || 'backend/runtime implementation'
    const suggestion = (payload.suggestion || `The frontend preview stays available right now. Upgrade to ${requiredPlan.replace(/\b\w/g, (match) => match.toUpperCase())} or higher to unlock ${reason} on this same app.`).trim()

    setUpgradePrompt({
      requiredPlan,
      reason,
      suggestion,
      buildId: payload.buildId || buildStateRef.current?.id,
      source: payload.source,
    })
    addNotification({
      type: 'warning',
      title: 'Upgrade Required',
      message: suggestion,
    })
  }, [addNotification])

  const addSystemMessage = useCallback((content: string) => {
    setChatMessages(prev => [...prev, {
      id: Date.now().toString(),
      role: 'system',
      content,
      timestamp: new Date(),
    }])
  }, [])

  useEffect(() => {
    setAgentMessageDrafts({})
    setAgentMessagePendingId(null)
    setPlannerMessagePending(false)
    setPlannerSendMode('lead')
    setBuildWorkspaceView('overview')
  }, [buildState?.id])

  useEffect(() => {
    if (buildWorkspaceView !== 'console' || !showChat) {
      return
    }
    const timer = window.setTimeout(() => {
      plannerInputRef.current?.focus()
    }, 0)
    return () => window.clearTimeout(timer)
  }, [buildWorkspaceView, showChat])

  // Fetch provider statuses for model role config
  useEffect(() => {
    apiService.buildPreflight().then(result => {
      if (result.provider_statuses) {
        setProviderStatuses(result.provider_statuses)
      }
      if (result.has_byok) {
        setHasBYOK(true)
      }
    }).catch(() => {
      // Non-fatal: preflight may not be available
    })
  }, [])

  useEffect(() => {
    if (!buildState?.id && !isBuilding && !createdProjectId) {
      setPlatformReadiness(null)
      return
    }

    let cancelled = false
    const loadPlatformReadiness = async () => {
      try {
        const summary = await apiService.featureReadiness()
        if (!cancelled) {
          setPlatformReadiness(summary)
        }
      } catch {
        if (!cancelled) {
          setPlatformReadiness(null)
        }
      }
    }

    void loadPlatformReadiness()
    const intervalId = window.setInterval(() => {
      void loadPlatformReadiness()
    }, 45000)

    return () => {
      cancelled = true
      window.clearInterval(intervalId)
    }
  }, [buildState?.id, buildState?.status, createdProjectId, isBuilding])

  // Validate role assignments in manual mode
  const isRoleAssignmentValid = useMemo(() => {
    if (roleConfigMode === 'auto') return true
    return 'architect' in roleAssignments && 'coder' in roleAssignments
  }, [roleConfigMode, roleAssignments])

  const builderRootRef = useRef<HTMLDivElement>(null)
  const startOverSignalRef = useRef<number | undefined>(undefined)
  const getScopedStorageKey = useCallback((baseKey: string) => {
    if (!user?.id) {
      return baseKey
    }
    return `${baseKey}:${user.id}`
  }, [user?.id])
  const migrateLegacyStoredValue = useCallback((baseKey: string) => {
    if (!user?.id) {
      return null
    }

    try {
      const legacyValue = localStorage.getItem(baseKey)
      if (!legacyValue) {
        return null
      }

      const scopedKey = getScopedStorageKey(baseKey)
      localStorage.setItem(scopedKey, legacyValue)
      localStorage.removeItem(baseKey)
      return legacyValue
    } catch {
      return null
    }
  }, [getScopedStorageKey, user?.id])
  const readStoredValue = useCallback((baseKey: string) => {
    try {
      if (user?.id) {
        const scopedValue = localStorage.getItem(getScopedStorageKey(baseKey))
        if (scopedValue) {
          return scopedValue
        }
        return migrateLegacyStoredValue(baseKey)
      }
      return localStorage.getItem(baseKey)
    } catch {
      return null
    }
  }, [getScopedStorageKey, migrateLegacyStoredValue, user?.id])
  const writeStoredValue = useCallback((baseKey: string, value: string) => {
    try {
      if (user?.id) {
        localStorage.setItem(getScopedStorageKey(baseKey), value)
        return
      }
      localStorage.setItem(baseKey, value)
    } catch {
      // Ignore localStorage failures (private mode, quota, etc.)
    }
  }, [getScopedStorageKey, user?.id])
  const clearStoredValue = useCallback((baseKey: string) => {
    try {
      if (user?.id) {
        localStorage.removeItem(getScopedStorageKey(baseKey))
        return
      }
      localStorage.removeItem(baseKey)
    } catch {
      // Ignore localStorage failures
    }
  }, [getScopedStorageKey, user?.id])
  const persistActiveBuildId = useCallback((buildId: string) => {
    writeStoredValue(ACTIVE_BUILD_STORAGE_KEY, buildId)
  }, [writeStoredValue])
  const clearActiveBuildId = useCallback(() => {
    clearStoredValue(ACTIVE_BUILD_STORAGE_KEY)
  }, [clearStoredValue])
  const persistLastWorkflowBuildId = useCallback((buildId: string) => {
    writeStoredValue(LAST_WORKFLOW_BUILD_STORAGE_KEY, buildId)
  }, [writeStoredValue])
  const clearLastWorkflowBuildId = useCallback(() => {
    clearStoredValue(LAST_WORKFLOW_BUILD_STORAGE_KEY)
  }, [clearStoredValue])
  const buildUpgradeReturnUrl = useCallback((status: 'success' | 'canceled', buildId?: string) => {
    const url = new URL(window.location.href)
    url.searchParams.delete('upgrade')
    url.searchParams.delete('resume_build')
    url.searchParams.set('upgrade', status)
    if (buildId) {
      url.searchParams.set('resume_build', buildId)
    }
    return url.toString()
  }, [])
  const serializeTelemetryThought = useCallback((thought: AIThought): PersistedAIThought => ({
    id: thought.id,
    agentId: thought.agentId,
    agentRole: thought.agentRole,
    provider: thought.provider,
    model: thought.model,
    type: thought.type,
    eventType: thought.eventType,
    taskId: thought.taskId,
    taskType: thought.taskType,
    files: thought.files,
    filesCount: thought.filesCount,
    retryCount: thought.retryCount,
    maxRetries: thought.maxRetries,
    isInternal: thought.isInternal,
    content: thought.content,
    timestamp: thought.timestamp.toISOString(),
  }), [])
  const restorePersistedTelemetry = useCallback((buildId: string): AIThought[] => {
    const snapshot = readBuildTelemetrySnapshot(readStoredValue(BUILD_TELEMETRY_STORAGE_KEY), buildId)
    if (!snapshot) return []

    return snapshot.thoughts
      .map((thought) => {
        const timestamp = new Date(thought.timestamp)
        if (Number.isNaN(timestamp.getTime())) return null
        return {
          ...thought,
          timestamp,
        }
      })
      .filter((thought): thought is AIThought => thought !== null)
  }, [readStoredValue])
  const restoreServerTelemetry = useCallback((timeline: unknown): AIThought[] => {
    return parseBuildTelemetryThoughts(timeline)
      .map((thought) => {
        const timestamp = new Date(thought.timestamp)
        if (Number.isNaN(timestamp.getTime())) return null
        return {
          ...thought,
          timestamp,
        }
      })
      .filter((thought): thought is AIThought => thought !== null)
  }, [])
  const activePowerMode = buildState?.powerMode || powerMode
  const activeProviderModelOverrides = buildState?.providerModelOverrides
    ? normalizeProviderModelOverrides(buildState.providerModelOverrides)
    : providerModelOverrides
  const activeBuildStatuses = useMemo(
    () => new Set<BuildState['status']>(['planning', 'in_progress', 'testing', 'reviewing', 'awaiting_review']),
    []
  )
  const isBuildActive = buildState ? activeBuildStatuses.has(buildState.status) : false
  useEffect(() => {
    if (!isBuildActive && plannerSendMode === 'all_agents') {
      setPlannerSendMode('lead')
    }
  }, [isBuildActive, plannerSendMode])
  const normalizeFSMStateToPhase = useCallback((state: string): string => {
    const FSM_STATE_LABELS: Record<string, string> = {
      // Core FSM states from state_machine.go
      planning:        'Planning',
      executing:       'Building',
      validating:      'Validating',
      retrying:        'Fixing Issues',
      rolling_back:    'Recovering',
      rolled_back:     'Recovered',
      paused:          'Paused',
      completed:       'Completed',
      failed:          'Failed',
      // Aliases that may come through current_phase
      provider_check:  'Checking Providers',
      scaffolding:     'Scaffolding',
      generating:      'Generating Code',
      analyzing:       'Analyzing',
      repairing:       'Repairing',
      finalizing:      'Finalizing',
      deploying:       'Deploying',
      reviewing:       'Review',
      testing:         'Testing',
      scaffold:        'Scaffold',
      frontend_ui:     'Frontend UI',
      data_foundation: 'Data Foundation',
      backend_services:'Backend Services',
      integration:     'Integration',
      ship:            'Preview / Ship',
      planning_complete: 'Scaffold',
      scaffold_bootstrapped: 'Scaffold',
      contract_verification: 'Scaffold',
    }
    const key = state.toLowerCase().trim()
    return FSM_STATE_LABELS[key] ?? null
  }, [])
  const humanizePhase = useCallback((phase: string) => {
    const fsm = normalizeFSMStateToPhase(phase)
    if (fsm) return fsm
    const normalized = phase.replace(/_/g, ' ').trim()
    if (!normalized) return 'Planning'
    return normalized.replace(/\b\w/g, (m) => m.toUpperCase())
  }, [normalizeFSMStateToPhase])
  const phaseLabel = useMemo(() => {
    if (!buildState) return 'Planning'
    if (buildState.currentPhase) return humanizePhase(buildState.currentPhase)
    switch (buildState.status) {
      case 'planning':
        return 'Planning'
      case 'testing':
        return 'Testing'
      case 'reviewing':
        return 'Review'
      case 'completed':
        return 'Completed'
      case 'failed':
        return 'Failed'
      default:
        return 'Code Generation'
    }
  }, [buildState, humanizePhase])
  const qualityGateLabel = useMemo(() => {
    if (!buildState) return 'Pending'
    if (buildState.qualityGateStatus) {
      switch (buildState.qualityGateStatus) {
        case 'passed':
          return 'Passed'
        case 'failed':
          return 'Failed'
        case 'running':
          return 'Running'
        default:
          return 'Pending'
      }
    }
    if (buildState.status === 'completed') return 'Passed'
    if (buildState.status === 'failed') return 'Failed'
    if (buildState.status === 'testing' || buildState.status === 'reviewing') return 'Running'
    return 'Pending'
  }, [buildState])
  const qualityGateToneClass = useMemo(() => {
    switch (qualityGateLabel) {
      case 'Passed':
        return 'border-green-500/60 bg-green-500/15 text-green-300'
      case 'Failed':
        return 'border-red-500/60 bg-red-500/15 text-red-300'
      case 'Running':
        return 'border-blue-500/60 bg-blue-500/15 text-blue-300'
      default:
        return 'border-gray-600 bg-gray-500/10 text-gray-300'
    }
  }, [qualityGateLabel])
  const providerPanels = useMemo<ProviderPanelState[]>(() => {
    const configuredTier = getModelTier(activePowerMode)
    const availableProviders = new Set(
      (buildState?.availableProviders || [])
        .map((provider) => normalizeProviderKey(provider))
        .filter((provider): provider is SupportedBuildProvider => provider !== null)
    )

    return MODEL_PANEL_ORDER.map((provider) => {
      const manualModel = activeProviderModelOverrides[provider]
      const configuredModel = manualModel !== 'auto' && modelBelongsToProvider(provider, manualModel)
        ? {
            id: manualModel,
            name: getModelDisplayName(manualModel, activePowerMode) || manualModel,
          }
        : configuredTier[provider]
      const providerAgents = (buildState?.agents || []).filter((agent) => normalizeProviderKey(agent.provider) === provider)
      const thoughts = aiThoughts
        .filter((thought) => normalizeProviderKey(thought.provider) === provider)
        .slice(-MAX_PROVIDER_THOUGHTS)
      const latestThought = thoughts[thoughts.length - 1]
      const actualModelIds = Array.from(new Set(
        [
          ...providerAgents.map((agent) => canonicalizeModelId(agent.model)).filter(Boolean),
          ...thoughts.map((thought) => canonicalizeModelId(thought.model)).filter(Boolean),
        ].filter((modelId) => modelBelongsToProvider(provider, modelId))
      ))
      const incompatibleModelIds = [
        ...providerAgents.map((agent) => canonicalizeModelId(agent.model)).filter(Boolean),
        ...thoughts.map((thought) => canonicalizeModelId(thought.model)).filter(Boolean),
      ].filter((modelId) => !modelBelongsToProvider(provider, modelId))
      const liveModelId = actualModelIds[actualModelIds.length - 1] || configuredModel.id
      const available = availableProviders.size === 0 || availableProviders.has(provider)

      const latestTaskDescription = providerAgents
        .map((agent) => agent.currentTask?.description || agent.currentTask?.type)
        .find(Boolean)
      const latestTaskType = latestThought?.taskType || providerAgents
        .map((agent) => agent.currentTask?.type)
        .find(Boolean)
      const hasActiveProviderError = providerAgents.some((agent) => agent.status === 'error') ||
        (providerAgents.length > 0 && latestThought?.type === 'error')

      let status: ProviderPanelState['status'] = 'idle'
      if (!available) {
        status = 'unavailable'
      } else if (providerAgents.some((agent) => agent.status === 'working')) {
        status = latestThought?.type === 'thinking' && latestThought.isInternal ? 'thinking' : 'working'
      } else if (providerAgents.some((agent) => agent.status === 'completed')) {
        status = 'completed'
      } else if (hasActiveProviderError) {
        status = 'error'
      } else if (isBuildActive && latestThought?.type === 'thinking' && latestThought.isInternal) {
        status = 'thinking'
      }

      const statusLabel = status === 'unavailable'
        ? 'Unavailable'
        : status === 'error'
          ? 'Attention Needed'
          : status === 'thinking'
            ? 'Thinking Internally'
            : status === 'working'
              ? (latestThought?.eventType === 'agent:generating' ? 'Generating' : 'Working')
              : status === 'completed'
                ? 'Completed'
                : isBuildActive
                  ? 'Standby'
                  : 'Waiting'

      return {
        provider,
        configuredModel,
        liveModelId,
        liveModelName: getModelDisplayName(liveModelId, activePowerMode) || configuredModel.name,
        available,
        status,
        statusLabel,
        agentCount: providerAgents.length,
        activeRoles: Array.from(new Set(providerAgents.map((agent) => humanizeIdentifier(agent.role)).filter(Boolean))),
        totalUpdates: thoughts.length,
        currentTaskLabel: latestThought?.content || latestTaskDescription || (latestTaskType ? humanizeIdentifier(latestTaskType) : undefined),
        latestThought,
        thoughts,
        mismatch: actualModelIds.some((modelId) => modelId !== configuredModel.id) || incompatibleModelIds.length > 0,
        multipleLiveModels: actualModelIds.length > 1,
      }
    })
  }, [activePowerMode, activeProviderModelOverrides, aiThoughts, buildState?.agents, buildState?.availableProviders, isBuildActive])

  useEffect(() => {
    if (aiThoughts.length === 0) return
    for (const provider of MODEL_PANEL_ORDER) {
      providerActivityRefs.current[provider]?.scrollTo({
        top: providerActivityRefs.current[provider]?.scrollHeight ?? 0,
        behavior: 'smooth',
      })
    }
  }, [aiThoughts.length])

  useEffect(() => {
    if (!isBuildActive) return
    const interval = window.setInterval(() => {
      setTelemetryNow(Date.now())
    }, 15000)
    return () => window.clearInterval(interval)
  }, [isBuildActive])

  useEffect(() => {
    if (!buildState?.id || aiThoughts.length === 0) return

    const latestThought = aiThoughts[aiThoughts.length - 1]
    const nextCache = upsertBuildTelemetrySnapshot(
      readStoredValue(BUILD_TELEMETRY_STORAGE_KEY),
      {
        buildId: buildState.id,
        updatedAt: latestThought?.timestamp.toISOString() || new Date().toISOString(),
        thoughts: aiThoughts.map(serializeTelemetryThought),
      }
    )
    writeStoredValue(BUILD_TELEMETRY_STORAGE_KEY, nextCache)
  }, [aiThoughts, buildState?.id, readStoredValue, serializeTelemetryThought, writeStoredValue])

  const interactionState = buildState?.interaction
  const pendingQuestion = interactionState?.pending_question
  const buildPaused = Boolean(interactionState?.paused)
  const pendingRevisionRequests = interactionState?.pending_revisions || []
  const pendingPermissionRequests = useMemo(
    () => (interactionState?.permission_requests || []).filter((request) => request.status === 'pending'),
    [interactionState?.permission_requests]
  )
  const grantedPermissionRules = useMemo(
    () => (interactionState?.permission_rules || []).filter((rule) => rule.decision === 'allow'),
    [interactionState?.permission_rules]
  )
  const hasBuildControlsPanel = Boolean(
    pendingQuestion ||
    pendingRevisionRequests.length > 0 ||
    pendingPermissionRequests.length > 0 ||
    grantedPermissionRules.length > 0 ||
    buildState?.status === 'awaiting_review'
  )
  const telemetrySummary = useMemo(() => {
    const lastThought = aiThoughts[aiThoughts.length - 1]
    const lastThoughtTime = lastThought?.timestamp instanceof Date ? lastThought.timestamp.getTime() : null
    const secondsSinceLastThought = lastThoughtTime ? Math.max(0, Math.floor((telemetryNow - lastThoughtTime) / 1000)) : null
    const activeProviders = isBuildActive
      ? providerPanels.filter((panel) => panel.status === 'working' || panel.status === 'thinking').length
      : 0
    const activeAgents = isBuildActive
      ? (buildState?.agents.filter((agent) => agent.status === 'working').length ?? 0)
      : 0
    const legacyBlockerCount =
      (buildPaused ? 1 : 0) +
      (pendingQuestion ? 1 : 0) +
      pendingPermissionRequests.length +
      (pendingRevisionRequests.length > 0 ? 1 : 0)
    const blockerCount = Math.max(legacyBlockerCount, buildState?.blockers?.length ?? 0)

    return {
      activeProviders,
      activeAgents,
      totalUpdates: aiThoughts.length,
      blockerCount,
      checkpointCount: buildState?.checkpoints.length ?? 0,
      lastThoughtLabel: lastThought == null
        ? 'No AI activity yet'
        : secondsSinceLastThought == null
          ? 'Live now'
          : secondsSinceLastThought < 10
            ? 'Live now'
            : `${secondsSinceLastThought}s ago`,
    }
  }, [
    aiThoughts,
    buildPaused,
    buildState?.agents,
    buildState?.checkpoints.length,
    pendingPermissionRequests.length,
    pendingQuestion,
    pendingRevisionRequests.length,
    providerPanels,
    telemetryNow,
    buildState?.blockers?.length,
    isBuildActive,
  ])
  const recentThoughtsByAgent = useMemo(() => {
    const next = new Map<string, AIThought[]>()
    for (const thought of aiThoughts) {
      if (!thought.agentId) continue
      const existing = next.get(thought.agentId) || []
      existing.push(thought)
      next.set(thought.agentId, existing.slice(-3))
    }
    return next
  }, [aiThoughts])
  const liveProviderPanels = useMemo(
    () => isBuildActive
      ? providerPanels.filter((panel) => panel.status === 'thinking' || panel.status === 'working')
      : [],
    [providerPanels, isBuildActive]
  )
  const liveAgents = useMemo(
    () => isBuildActive
      ? (buildState?.agents || []).filter((agent) => agent.status === 'working')
      : [],
    [buildState?.agents, isBuildActive]
  )
  const liveTasks = useMemo(
    () => isBuildActive
      ? (buildState?.tasks || []).filter((task) => task.status === 'in_progress')
      : [],
    [buildState?.tasks, isBuildActive]
  )
  const hasBackendDataStage = useMemo(() => {
    if (!buildState) return false
    const workOrderRoles = new Set((buildState.workOrders || []).map((order) => String(order.role || '').toLowerCase()))
    return (
      workOrderRoles.has('backend') ||
      workOrderRoles.has('database') ||
      Boolean(buildState.capabilityState?.requires_backend_runtime) ||
      Boolean(buildState.capabilityState?.requires_database) ||
      Boolean(buildState.capabilityState?.requires_storage) ||
      Boolean(buildState.capabilityState?.requires_jobs)
    )
  }, [buildState])
  const workflowStageDefs = useMemo(
    () => BUILD_WORKFLOW_STAGE_DEFS.filter((stage) => stage.key !== 'backend_data' || hasBackendDataStage),
    [hasBackendDataStage]
  )
  const currentWorkflowStageKey = useMemo(() => {
    if (!buildState) return workflowStageDefs[0]?.key || 'scaffold'
    const currentPhase = String(buildState.currentPhase || '').trim().toLowerCase()

    if (buildState.status === 'completed') return 'ship'
    if (
      currentPhase.includes('review') ||
      buildState.status === 'reviewing' ||
      buildState.status === 'awaiting_review'
    ) {
      return 'ship'
    }
    if (
      currentPhase.includes('integration') ||
      currentPhase.includes('testing') ||
      currentPhase.includes('validation') ||
      buildState.status === 'testing'
    ) {
      return 'integration'
    }
    if (
      currentPhase.includes('backend') ||
      currentPhase.includes('database') ||
      currentPhase.includes('data_foundation') ||
      currentPhase.includes('data')
    ) {
      return hasBackendDataStage ? 'backend_data' : 'integration'
    }
    if (currentPhase.includes('frontend')) {
      return 'frontend_ui'
    }
    return 'scaffold'
  }, [buildState, hasBackendDataStage, workflowStageDefs])
  const currentWorkflowStageIndex = useMemo(() => {
    const index = workflowStageDefs.findIndex((stage) => stage.key === currentWorkflowStageKey)
    return index >= 0 ? index : 0
  }, [currentWorkflowStageKey, workflowStageDefs])
  const workflowStages = useMemo<BuildWorkflowStage[]>(() => {
    return workflowStageDefs.map((stage, index) => {
      let status: BuildWorkflowStageStatus = 'pending'
      if (buildState?.status === 'completed') {
        status = 'complete'
      } else if (index < currentWorkflowStageIndex) {
        status = 'complete'
      } else if (index === currentWorkflowStageIndex) {
        status = buildState?.status === 'failed' || buildState?.status === 'cancelled' ? 'blocked' : 'current'
      }

      return {
        ...stage,
        status,
      }
    })
  }, [buildState?.status, currentWorkflowStageIndex, workflowStageDefs])
  const currentWorkflowStage = workflowStages[currentWorkflowStageIndex] || workflowStages[0]
  const impactedPlatformServices = useMemo(() => {
    if (!platformReadiness || platformReadiness.status === 'healthy') {
      return []
    }

    return (platformReadiness.services || [])
      .filter(service => service.state !== 'ready')
      .slice()
      .sort((left, right) => {
        const weight = (tier: string) => tier === 'critical' ? 0 : 1
        if (weight(left.tier) !== weight(right.tier)) {
          return weight(left.tier) - weight(right.tier)
        }
        return left.name.localeCompare(right.name)
      })
  }, [platformReadiness])
  const buildFailureAttribution = useMemo(() => {
    if (buildState?.status !== 'failed') {
      return null
    }

    const issue = buildState.platformIssue
    const primaryService = issue?.service
      ? impactedPlatformServices.find(service => service.name === issue.service) || impactedPlatformServices[0]
      : impactedPlatformServices[0]

    if (!issue && !primaryService) {
      return null
    }

    const serviceName = issue?.service || primaryService?.name
    const primaryServiceDetails = (primaryService?.details ?? {}) as Record<string, unknown>
    const recommendedFix = typeof primaryServiceDetails.recommended_fix === 'string' ? primaryServiceDetails.recommended_fix : ''
    const fallbackReason = typeof primaryServiceDetails.fallback_reason === 'string' ? primaryServiceDetails.fallback_reason.toLowerCase() : ''
    const maintenanceWindow = issue?.maintenanceWindow === true || issue?.issueType === 'platform_maintenance'
    let title = maintenanceWindow ? 'Build paused by platform maintenance' : 'This failure may be platform-related'
    let body = issue?.summary || 'A platform interruption may have stopped this build before the current section completed.'
    let detail = issue?.retryable === false
      ? 'Open diagnostics before retrying so the underlying platform issue can be understood.'
      : 'The generated app work may still be valid. Retry once platform health returns, or open diagnostics for the captured build error.'

    switch (serviceName) {
      case 'primary_database':
        body = issue?.summary || 'Primary database connectivity dropped while the build was running. Build recovery, history reads, and status sync can fail even when the generated app code is still intact.'
        detail = maintenanceWindow
          ? 'This usually clears after the platform finishes reconnecting the database. Retry the build once the maintenance window ends.'
          : 'Retry after database connectivity returns, then reopen the build or request a restart from the last healthy checkpoint.'
        break
      case 'redis_cache':
        if (issue?.issueType === 'platform_configuration' || fallbackReason.includes('allowlist')) {
          title = 'Redis cache is misconfigured'
          body = issue?.summary || 'Redis is pointed at an external allowlisted endpoint, so live build coordination can fail even though the generated files may still be intact.'
          detail = recommendedFix || 'Update REDIS_URL to the internal Render Key Value connection string, redeploy the backend, then retry the build.'
          break
        }
        title = maintenanceWindow ? 'Live build timing affected by platform maintenance' : 'Live build timing may be platform-related'
        body = issue?.summary || 'Redis connectivity is degraded. Live coordination can stall or look incomplete even when the generated files are still intact.'
        detail = 'Build output may still be usable. Open Files or Diagnostics before assuming the app code itself is broken.'
        break
      default:
        if (primaryService && primaryService.name !== serviceName) {
          title = maintenanceWindow ? 'Build paused by platform maintenance' : 'This failure may be platform-related'
        }
        break
    }

    return {
      title,
      body,
      detail,
      isCritical: serviceName !== 'redis_cache',
      capturedError: buildState.errorMessage,
    }
  }, [buildState?.errorMessage, buildState?.platformIssue, buildState?.status, impactedPlatformServices])
  const primaryBuildUpdate = useMemo(() => {
    if (buildState?.status === 'failed') {
      return buildFailureAttribution?.body || buildState.errorMessage || 'The build stopped before the current section completed.'
    }
    if (buildPaused) {
      return 'The build is paused. Resume it or leave a planner note to change direction.'
    }
    if (pendingQuestion) {
      return pendingQuestion
    }
    if (pendingPermissionRequests.length > 0) {
      return `${pendingPermissionRequests.length} permission request${pendingPermissionRequests.length === 1 ? '' : 's'} need a decision before the build can continue cleanly.`
    }
    if (liveTasks.length > 0) {
      return liveTasks[0]?.description || currentWorkflowStage?.description || 'Build work is in progress.'
    }
    return currentWorkflowStage?.description || 'Build work is in progress.'
  }, [
    buildPaused,
    buildState?.errorMessage,
    buildState?.status,
    buildFailureAttribution?.body,
    currentWorkflowStage?.description,
    liveTasks,
    pendingPermissionRequests.length,
    pendingQuestion,
  ])
  const workflowUpdates = useMemo(
    () => chatMessages
      .filter((message) => message.role !== 'user')
      .slice(-4)
      .reverse(),
    [chatMessages]
  )
  const visibleBlockers = useMemo(
    () => (buildState?.blockers || []).filter((blocker) =>
      blocker.severity === 'blocking' ||
      blocker.severity === 'warning' ||
      Boolean(blocker.summary) ||
      Boolean(blocker.unblocks_with)
    ),
    [buildState?.blockers]
  )
  const activityViewIsEmpty = liveProviderPanels.length === 0 && liveAgents.length === 0 && liveTasks.length === 0
  const fileGroups = useMemo(() => {
    const groups = new Map<string, Array<{ path: string; content: string; language: string }>>()
    for (const file of generatedFiles) {
      const root = file.path.includes('/') ? file.path.split('/')[0] : 'root'
      const existing = groups.get(root) || []
      existing.push(file)
      groups.set(root, existing)
    }
    return Array.from(groups.entries())
      .map(([root, files]) => ({
        root,
        files: files.slice().sort((left, right) => left.path.localeCompare(right.path)),
      }))
      .sort((left, right) => left.root.localeCompare(right.root))
  }, [generatedFiles])
  const artifactSummary = useMemo(() => {
    const counts = {
      frontend: 0,
      backend: 0,
      data: 0,
      config: 0,
    }
    for (const file of generatedFiles) {
      const path = file.path.toLowerCase()
      if (
        path.startsWith('src/') ||
        path.startsWith('app/') ||
        path.startsWith('components/') ||
        path.startsWith('public/') ||
        path === 'index.html'
      ) {
        counts.frontend += 1
        continue
      }
      if (
        path.startsWith('server/') ||
        path.startsWith('api/') ||
        path.startsWith('backend/') ||
        path.endsWith('.go') ||
        path.endsWith('.py')
      ) {
        counts.backend += 1
        continue
      }
      if (path.startsWith('migrations/') || path.includes('schema') || path.startsWith('db/') || path.startsWith('prisma/')) {
        counts.data += 1
        continue
      }
      counts.config += 1
    }
    return counts
  }, [generatedFiles])
  const timelineEventCount = useMemo(
    () => workflowUpdates.length + (buildState?.checkpoints.length || 0) + (buildState?.verificationReports?.length || 0) + visibleBlockers.length,
    [buildState?.checkpoints.length, buildState?.verificationReports?.length, visibleBlockers.length, workflowUpdates.length]
  )
  const statusRailMetrics = useMemo(() => ([
    {
      label: 'Section',
      value: currentWorkflowStage?.label || phaseLabel,
      hint: `${currentWorkflowStageIndex + 1}/${workflowStages.length} active`,
    },
    {
      label: 'Live',
      value: isBuildActive ? telemetrySummary.activeAgents : 0,
      hint: isBuildActive
        ? `${liveTasks.length} task${liveTasks.length === 1 ? '' : 's'} running`
        : 'Quiet',
    },
    {
      label: 'Attention',
      value: telemetrySummary.blockerCount,
      hint: telemetrySummary.blockerCount > 0 ? 'Open Issues' : 'Clear',
    },
    {
      label: 'Files',
      value: generatedFiles.length,
      hint: generatedFiles.length > 0 ? 'Open Files' : 'Streaming in',
    },
  ]), [
    currentWorkflowStage?.label,
    currentWorkflowStageIndex,
    generatedFiles.length,
    isBuildActive,
    liveTasks.length,
    phaseLabel,
    telemetrySummary.activeAgents,
    telemetrySummary.blockerCount,
    workflowStages.length,
  ])
  const hasIssueViewContent = Boolean(
    buildFailureAttribution ||
    visibleBlockers.length > 0 ||
    buildState?.checkpoints.length ||
    hasBuildControlsPanel ||
    (buildState?.status === 'awaiting_review' && showDiffReview && proposedEdits.length > 0)
  )
  const buildWorkspaceViews = useMemo(() => {
    return [
      { id: 'overview' as const, label: 'Overview', hint: currentWorkflowStage?.label || 'Build summary' },
      { id: 'activity' as const, label: 'Activity', hint: activityViewIsEmpty ? 'Quiet' : `${liveAgents.length + liveTasks.length} live` },
      { id: 'files' as const, label: 'Files', hint: generatedFiles.length > 0 ? `${generatedFiles.length} artifacts` : 'Waiting' },
      { id: 'timeline' as const, label: 'Timeline', hint: timelineEventCount > 0 ? `${timelineEventCount} events` : 'Stage history' },
      {
        id: 'issues' as const,
        label: 'Issues',
        hint: telemetrySummary.blockerCount > 0
          ? `${telemetrySummary.blockerCount} blocker${telemetrySummary.blockerCount === 1 ? '' : 's'}`
          : hasBuildControlsPanel
            ? 'Controls ready'
            : 'Clear',
      },
      { id: 'diagnostics' as const, label: 'Diagnostics', hint: 'Deep orchestration detail' },
      { id: 'console' as const, label: 'Console', hint: 'Steer build and directives' },
    ]
  }, [
    activityViewIsEmpty,
    currentWorkflowStage?.label,
    generatedFiles.length,
    hasBuildControlsPanel,
    liveAgents.length,
    liveTasks.length,
    telemetrySummary.blockerCount,
    timelineEventCount,
  ])
  const hasBuilderSession = Boolean(
    buildState ||
    isBuilding ||
    generatedFiles.length > 0 ||
    chatMessages.length > 0 ||
    aiThoughts.length > 0 ||
    proposedEdits.length > 0 ||
    createdProjectId
  )
  const platformReadinessNotice = useMemo(() => {
    if (buildState?.status === 'failed' || !platformReadiness || platformReadiness.status === 'healthy') {
      return null
    }

    const impactedServices = impactedPlatformServices
    if (impactedServices.length === 0) {
      return null
    }

    const primaryService = impactedServices[0]
    const primaryServiceDetails = (primaryService.details ?? {}) as Record<string, unknown>
    const recommendedFix = typeof primaryServiceDetails.recommended_fix === 'string' ? primaryServiceDetails.recommended_fix : ''
    const fallbackReason = typeof primaryServiceDetails.fallback_reason === 'string' ? primaryServiceDetails.fallback_reason.toLowerCase() : ''
    const isCritical = primaryService.tier === 'critical' || !platformReadiness.ready
    let title = isCritical ? 'Platform services interrupted' : 'Platform services degraded'
    let body = primaryService.summary
    let detail = impactedServices.length > 1
      ? `${impactedServices.length} platform services currently need attention.`
      : isCritical
        ? 'Build recovery, status sync, or preview actions may pause until the service returns.'
        : 'Builds continue with fallbacks where possible while maintenance completes.'

    switch (primaryService.name) {
      case 'redis_cache':
        if (fallbackReason.includes('allowlist')) {
          title = 'Redis cache is misconfigured'
          body = 'Redis is using an external allowlisted endpoint. The backend should use the internal Render Key Value URL instead.'
          detail = recommendedFix || 'Update REDIS_URL to the apex-redis internal connection string and redeploy the backend.'
          break
        }
        body = 'Redis cache is degraded. Builds continue with in-memory fallback, but live coordination can feel slower until maintenance finishes.'
        detail = impactedServices.length > 1
          ? `${impactedServices.length} platform services are affected right now.`
          : 'Background cache fallbacks are active, so builds can keep moving.'
        break
      case 'primary_database':
        body = 'Primary database connectivity is interrupted. New writes, build recovery, and status sync can pause until the database returns.'
        break
      default:
        title = isCritical ? 'Platform service interrupted' : 'Platform service degraded'
        break
    }

    return {
      title,
      body,
      detail,
      isCritical,
    }
  }, [buildState?.status, impactedPlatformServices, platformReadiness])

  const resetBuilderState = useCallback((options?: { clearPrompt?: boolean }) => {
    isBuildingRef.current = false
    buildStateRef.current = null
    wsReconnectAttempts.current = 0

    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
    wsBuildIdRef.current = null

    clearActiveBuildId()
    previewPreparedRef.current = false
    setBuildState(null)
    setIsBuilding(false)
    setShowChat(true)
    setIsPreparingPreview(false)
    setGeneratedFiles([])
    setCreatedProjectId(null)
    setIsCreatingProject(false)
    setChatMessages([])
    setChatInput('')
    setPermissionActionId(null)
    setBuildActionPending(null)
    setProposedEdits([])
    setShowDiffReview(true)
    setAiThoughts([])
    setShowImportModal(false)
    setShowGitHubImport(false)
    setReplitUrl('')
    setIsImporting(false)
    setRollbackCheckpointId(null)
    setShowBuyCredits(false)
    setBuyCreditsReason(undefined)
    setUpgradePrompt(null)
    setUpgradeCheckoutPending(false)
    setPlatformReadiness(null)
    if (options?.clearPrompt) {
      setAppDescription('')
    }
    builderRootRef.current?.scrollTo({ top: 0, behavior: 'smooth' })
  }, [clearActiveBuildId])

  const handleStartOver = useCallback(async (options?: { skipConfirm?: boolean; clearPrompt?: boolean }) => {
    const currentBuild = buildStateRef.current
    const activeBuild = Boolean(currentBuild?.id && isActiveBuildStatus(currentBuild.status))
    const clearPrompt = options?.clearPrompt ?? true

    if (!hasBuilderSession && !activeBuild) {
      if (clearPrompt) {
        setAppDescription('')
      }
      builderRootRef.current?.scrollTo({ top: 0, behavior: 'smooth' })
      return
    }

    if (!options?.skipConfirm) {
      const confirmed = window.confirm(
        activeBuild
          ? 'Cancel the current build and return to a fresh prompt? Your saved work will still appear in Recent Builds.'
          : 'Clear this workflow and return to a fresh prompt? Your saved work will still appear in Recent Builds.'
      )
      if (!confirmed) return
    }

    setIsStartingOver(true)
    let cancelFailed = false

    try {
      if (activeBuild && currentBuild?.id) {
        try {
          await apiService.cancelBuild(currentBuild.id)
        } catch {
          cancelFailed = true
        }
      }
    } finally {
      if (cancelFailed && currentBuild?.id) {
        persistLastWorkflowBuildId(currentBuild.id)
      } else {
        clearLastWorkflowBuildId()
      }
      resetBuilderState({ clearPrompt })
      addNotification({
        type: cancelFailed ? 'warning' : 'info',
        title: cancelFailed ? 'Fresh Start Ready' : 'Fresh Build Ready',
        message: cancelFailed
          ? 'The live build could not be cancelled cleanly, but the builder has been reset to a blank prompt. Your saved files are still available in Recent Builds.'
          : activeBuild
            ? 'The current build was cancelled. Your saved work is still available in Recent Builds.'
            : 'The builder was reset to a blank prompt. You can reopen old work from Recent Builds at any time.',
      })
      setIsStartingOver(false)
    }
  }, [addNotification, clearLastWorkflowBuildId, hasBuilderSession, persistLastWorkflowBuildId, resetBuilderState])

  useEffect(() => {
    if (startOverSignal === undefined) return
    if (startOverSignalRef.current === startOverSignal) return

    startOverSignalRef.current = startOverSignal
    if (startOverSignal === 0) return

    void handleStartOver({ skipConfirm: true })
  }, [handleStartOver, startOverSignal])

  // Tech stack options
  const techStacks: TechStack[] = [
    { id: AUTO_STACK_ID, name: 'Auto (Best Fit)', icon: <Sparkles className="w-5 h-5" />, category: 'auto', description: 'Let AI choose' },
    { id: 'nextjs', name: 'Next.js', icon: <Globe className="w-5 h-5" />, category: 'frontend', description: 'React Framework' },
    { id: 'react', name: 'React', icon: <Layout className="w-5 h-5" />, category: 'frontend', description: 'UI Library' },
    { id: 'vue', name: 'Vue.js', icon: <Layers className="w-5 h-5" />, category: 'frontend', description: 'Progressive Framework' },
    { id: 'node', name: 'Node.js', icon: <Server className="w-5 h-5" />, category: 'backend', description: 'JavaScript Runtime' },
    { id: 'python', name: 'Python', icon: <Code2 className="w-5 h-5" />, category: 'backend', description: 'FastAPI/Django' },
    { id: 'go', name: 'Go', icon: <Zap className="w-5 h-5" />, category: 'backend', description: 'High Performance' },
    { id: 'postgresql', name: 'PostgreSQL', icon: <Database className="w-5 h-5" />, category: 'database', description: 'Relational DB' },
    { id: 'mongodb', name: 'MongoDB', icon: <Database className="w-5 h-5" />, category: 'database', description: 'Document DB' },
    { id: 'vercel', name: 'Vercel', icon: <Rocket className="w-5 h-5" />, category: 'deploy', description: 'Edge Deployment' },
    { id: 'docker', name: 'Docker', icon: <Server className="w-5 h-5" />, category: 'deploy', description: 'Containerized' },
  ]

  const toggleStack = (id: string) => {
    setSelectedStack(prev => {
      const next = new Set(prev)

      if (id === AUTO_STACK_ID) {
        return new Set([AUTO_STACK_ID])
      }

      next.delete(AUTO_STACK_ID)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }

      if (next.size === 0) {
        next.add(AUTO_STACK_ID)
      }

      return next
    })
  }

  const getSelectedStacks = () => {
    if (selectedStack.has(AUTO_STACK_ID)) return []
    return techStacks.filter((stack) => stack.id !== AUTO_STACK_ID && selectedStack.has(stack.id))
  }

  const buildTechStackOverride = (): BuildTechStack | null => {
    if (selectedStack.has(AUTO_STACK_ID)) return null

    const selected = getSelectedStacks()
    if (selected.length === 0) return null

    const primary: Record<string, string> = {}
    const extras: string[] = []

    for (const stack of selected) {
      if (stack.category === 'frontend' || stack.category === 'backend' || stack.category === 'database') {
        if (!primary[stack.category]) {
          primary[stack.category] = stack.name
        } else {
          extras.push(stack.name)
        }
      } else {
        extras.push(stack.name)
      }
    }

    const override: BuildTechStack = {
      frontend: primary.frontend || undefined,
      backend: primary.backend || undefined,
      database: primary.database || undefined,
      styling: undefined,
      extras: extras.length > 0 ? extras : undefined,
    }

    return override
  }

  const buildTechStackSummary = () => {
    if (selectedStack.has(AUTO_STACK_ID)) return 'Auto (AI chooses best)'
    const override = buildTechStackOverride()
    if (!override) return 'Auto (AI chooses best)'

    const parts: string[] = []
    if (override.frontend) parts.push(`Frontend: ${override.frontend}`)
    if (override.backend) parts.push(`Backend: ${override.backend}`)
    if (override.database) parts.push(`Database: ${override.database}`)
    if (override.styling) parts.push(`Styling: ${override.styling}`)
    if (override.extras && override.extras.length > 0) parts.push(`Extras: ${override.extras.join(', ')}`)

    return parts.length > 0 ? parts.join(' | ') : 'Auto (AI chooses best)'
  }

  // Scroll chat to bottom
  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [chatMessages])

  // Cleanup WebSocket
  useEffect(() => {
    return () => {
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
      wsBuildIdRef.current = null
    }
  }, [])

  const clampPercent = (value: number) => {
    if (!Number.isFinite(value)) return 0
    return Math.max(0, Math.min(100, Math.round(value)))
  }

  const hasUsableWebSocketConnection = useCallback((buildId: string) => {
    const ws = wsRef.current
    if (!ws || wsBuildIdRef.current !== buildId) return false
    return ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN
  }, [])

  const computeAgentProgressFloor = (agents: Agent[]) => {
    const workers = agents.filter(a => a.role !== 'lead')
    if (workers.length === 0) return 20
    const done = workers.filter(a => a.status === 'completed' || a.status === 'error').length
    return clampPercent(20 + Math.round((done / workers.length) * 70))
  }

  // WebSocket URL builder
  const buildWebSocketUrl = useCallback((buildId: string, providedUrl?: string): string => {
    let wsUrl: string

    if (providedUrl && providedUrl.trim()) {
      const raw = providedUrl.trim()
      if (/^wss?:\/\//i.test(raw)) {
        wsUrl = raw
      } else {
        // Relative URL from server — resolve against the configured backend WS URL,
        // NOT window.location.host (which is the frontend host, not the backend).
        const configuredWsBase = getConfiguredWsUrl()
        const configuredApiBase = getConfiguredApiUrl()
        if (configuredWsBase) {
          const backendRoot = configuredWsBase.replace(/\/ws\/?$/, '').replace(/\/$/, '')
          const normalized = raw.startsWith('/') ? raw : `/${raw}`
          wsUrl = `${backendRoot}${normalized}`
        } else if (configuredApiBase) {
          const apiRoot = configuredApiBase.replace('/api/v1', '').replace(/\/$/, '')
          const wsProtocol = apiRoot.startsWith('https') ? 'wss' : 'ws'
          const wsHost = apiRoot.replace(/^https?:\/\//, '')
          const normalized = raw.startsWith('/') ? raw : `/${raw}`
          wsUrl = `${wsProtocol}://${wsHost}${normalized}`
        } else {
          const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
          const normalized = raw.startsWith('/') ? raw : `/${raw}`
          wsUrl = `${protocol}//${window.location.host}${normalized}`
        }
      }
    } else {
      const configuredWsUrl = getConfiguredWsUrl()
      const configuredApiUrl = getConfiguredApiUrl()

      if (configuredWsUrl) {
        const baseWsUrl = configuredWsUrl.replace(/\/ws\/?$/, '').replace(/\/$/, '')
        wsUrl = `${baseWsUrl}/ws/build/${buildId}`
      } else if (configuredApiUrl) {
        const apiUrl = configuredApiUrl.replace('/api/v1', '').replace(/\/$/, '')
        const wsProtocol = apiUrl.startsWith('https') ? 'wss' : 'ws'
        const wsHost = apiUrl.replace(/^https?:\/\//, '')
        wsUrl = `${wsProtocol}://${wsHost}/ws/build/${buildId}`
      } else {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
        wsUrl = `${protocol}//${window.location.host}/ws/build/${buildId}`
      }
    }

    return buildAuthenticatedWebSocketUrl(wsUrl)
  }, [])

  // WebSocket connection
  const connectWebSocket = useCallback((buildId: string, providedUrl?: string) => {
    if (hasUsableWebSocketConnection(buildId)) {
      return
    }

    const wsUrl = buildWebSocketUrl(buildId, providedUrl)
    console.log('Connecting to WebSocket:', wsUrl)

    if (wsRef.current && wsRef.current.readyState !== WebSocket.CLOSED) {
      wsRef.current.onopen = null
      wsRef.current.onmessage = null
      wsRef.current.onerror = null
      wsRef.current.onclose = null
      wsRef.current.close()
    }

    const ws = new WebSocket(wsUrl)
    wsBuildIdRef.current = buildId

    ws.onopen = () => {
      console.log('WebSocket connected')
      wsReconnectAttempts.current = 0
      setBuildState(prev => prev && prev.id === buildId ? { ...prev, liveSession: true } : prev)
      addSystemMessage('Connected to build server')
    }

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data)
        void wsMessageHandlerRef.current(message)
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e)
      }
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }

    ws.onclose = (event) => {
      console.log('WebSocket disconnected, code:', event.code)
      if (wsRef.current === ws) {
        wsRef.current = null
      }
      if (wsBuildIdRef.current === buildId) {
        wsBuildIdRef.current = null
      }
      setBuildState(prev => prev && prev.id === buildId ? { ...prev, liveSession: false } : prev)

      // Use ref to access current isBuilding state (prevents stale closure)
      if (isBuildingRef.current && wsReconnectAttempts.current < maxWsReconnectAttempts) {
        wsReconnectAttempts.current++
        const delay = Math.min(1000 * Math.pow(2, wsReconnectAttempts.current - 1), 10000)
        addSystemMessage(`Connection lost. Reconnecting in ${delay / 1000}s...`)

        setTimeout(() => {
          if (isBuildingRef.current) {
            connectWebSocket(buildId, buildStateRef.current?.websocketUrl)
          }
        }, delay)
      } else if (wsReconnectAttempts.current >= maxWsReconnectAttempts) {
        addSystemMessage('Connection failed after multiple attempts. Please refresh to retry.')
      }
    }

    wsRef.current = ws
  }, [addSystemMessage, buildWebSocketUrl, hasUsableWebSocketConnection])

  connectWebSocketRef.current = connectWebSocket

  // Mobile: reconnect WebSocket when page becomes visible again (e.g. user switches back to browser)
  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.hidden) return
      // Page just became visible — attempt WS reconnect if build is active
      const activeBuild = buildStateRef.current
      if (!activeBuild?.id || !isBuildingRef.current) return
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) return
      // Reset reconnect counter so we get fresh attempts
      wsReconnectAttempts.current = 0
      connectWebSocketRef.current(activeBuild.id, activeBuild.websocketUrl)
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange)
  }, [])

  // Handle WebSocket messages
  const handleWebSocketMessage = async (message: any) => {
    const { type, data } = message

    switch (type) {
      case 'build:state':
        setBuildState(prev => {
          const nextStatus = mergeBuildStatusWithTerminalPrecedence(prev?.status, data.status)
          const nextErrorMessage = extractBuildFailureReason(data)
          const nextGuarantee = Array.isArray(data.tasks)
            ? extractLatestGuaranteeStateFromTasks(data.tasks)
            : extractGuaranteeState(data)
          return ({
            ...prev,
            ...data,
            status: (nextStatus || prev?.status || 'pending') as BuildState['status'],
            progress: (prev && isTerminalBuildStatus(prev.status) && typeof data.progress === 'number')
              ? (prev.status === 'completed' ? 100 : prev.progress)
              : (typeof data.progress === 'number' ? clampPercent(data.progress) : prev?.progress ?? 0),
            agents: Object.values(data.agents || {}),
            powerMode: data.power_mode || data.powerMode || prev?.powerMode,
            providerModelOverrides: data.provider_model_overrides
              ? normalizeProviderModelOverrides(data.provider_model_overrides)
              : prev?.providerModelOverrides,
            currentPhase: data.phase_key || data.phase || data.current_phase || prev?.currentPhase,
            qualityGateRequired: typeof data.quality_gate_required === 'boolean' ? data.quality_gate_required : prev?.qualityGateRequired,
            qualityGateStage: data.quality_gate_stage || prev?.qualityGateStage,
            capabilityState: data.capability_state || prev?.capabilityState,
            policyState: data.policy_state || prev?.policyState,
            blockers: Array.isArray(data.blockers) ? data.blockers : prev?.blockers,
            approvals: Array.isArray(data.approvals) ? data.approvals : prev?.approvals,
            intentBrief: data.intent_brief || prev?.intentBrief,
            buildContract: data.build_contract || prev?.buildContract,
            workOrders: Array.isArray(data.work_orders) ? data.work_orders : prev?.workOrders,
            patchBundles: Array.isArray(data.patch_bundles) ? data.patch_bundles : prev?.patchBundles,
            verificationReports: Array.isArray(data.verification_reports) ? data.verification_reports : prev?.verificationReports,
            promotionDecision: data.promotion_decision || prev?.promotionDecision,
            providerScorecards: Array.isArray(data.provider_scorecards) ? data.provider_scorecards : prev?.providerScorecards,
            failureFingerprints: Array.isArray(data.failure_fingerprints) ? data.failure_fingerprints : prev?.failureFingerprints,
            historicalLearning: data.historical_learning || prev?.historicalLearning,
            promptPackActivationRequests: data.prompt_pack_activation_request
              ? upsertPromptPackActivationRequest(prev?.promptPackActivationRequests, data.prompt_pack_activation_request)
              : Array.isArray(data.prompt_pack_activation_requests)
                ? data.prompt_pack_activation_requests
                : prev?.promptPackActivationRequests,
            promptPackVersions: data.prompt_pack_version
              ? upsertPromptPackVersion(prev?.promptPackVersions, data.prompt_pack_version)
              : Array.isArray(data.prompt_pack_versions)
                ? data.prompt_pack_versions
                : prev?.promptPackVersions,
            promptPackActivationEvents: data.prompt_pack_activation_event
              ? upsertPromptPackActivationEvent(prev?.promptPackActivationEvents, data.prompt_pack_activation_event)
              : Array.isArray(data.prompt_pack_activation_events)
                ? data.prompt_pack_activation_events
                : prev?.promptPackActivationEvents,
            truthBySurface: data.truth_by_surface || prev?.truthBySurface,
            qualityGateStatus:
              typeof data.quality_gate_passed === 'boolean'
                ? (data.quality_gate_passed ? 'passed' : 'failed')
                : data.quality_gate_active
                  ? 'running'
                  : prev?.qualityGateStatus,
            errorMessage: nextErrorMessage || prev?.errorMessage,
            interaction: normalizeInteraction(data.interaction, data.messages) || prev?.interaction,
            guarantee: nextGuarantee || prev?.guarantee,
          })
        })
        syncInteractionState(data.interaction, data.messages)
        if (data.status === 'awaiting_review') {
          void loadProposedEdits(message.build_id || buildStateRef.current?.id)
        }
        break

      case 'build:progress':
        if (data.user_update && typeof data.message === 'string' && data.message.trim()) {
          addSystemMessage(data.message.trim())
        }
        setBuildState(prev => {
          if (!prev) return null
          const updates: Partial<BuildState> = {}

          // Apply status transition (e.g. planning → in_progress)
          const mergedStatus = mergeBuildStatusWithTerminalPrecedence(prev.status, data.status)
          const resumingFromTerminal = isTerminalBuildStatus(prev.status) && !!mergedStatus && !isTerminalBuildStatus(mergedStatus)
          if (isTerminalBuildStatus(prev.status) && !resumingFromTerminal) {
            return prev
          }

          if (typeof data.progress === 'number') {
            updates.progress = clampPercent(data.progress)
          }

          if (mergedStatus) {
            updates.status = mergedStatus as BuildState['status']
          }

          if (data.phase === 'provider_check' && data.available_providers) {
            updates.availableProviders = data.available_providers
            addSystemMessage(`AI Providers available: ${data.available_providers.join(', ')} (${data.provider_count} total)`)
          }
          if (data.provider_model_overrides) {
            updates.providerModelOverrides = normalizeProviderModelOverrides(data.provider_model_overrides)
          }

          if (typeof (data.phase_key || data.phase || data.current_phase) === 'string' && String(data.phase_key || data.phase || data.current_phase).trim()) {
            updates.currentPhase = data.phase_key || data.phase || data.current_phase
          }
          if (data.capability_state) {
            updates.capabilityState = data.capability_state
          }
          if (data.policy_state) {
            updates.policyState = data.policy_state
          }
          if (Array.isArray(data.blockers)) {
            updates.blockers = data.blockers
          }
          if (Array.isArray(data.approvals)) {
            updates.approvals = data.approvals
          }
          if (data.intent_brief) {
            updates.intentBrief = data.intent_brief
          }
          if (data.build_contract) {
            updates.buildContract = data.build_contract
          }
          if (Array.isArray(data.work_orders)) {
            updates.workOrders = data.work_orders
          }
          if (Array.isArray(data.patch_bundles)) {
            updates.patchBundles = data.patch_bundles
          }
          if (Array.isArray(data.verification_reports)) {
            updates.verificationReports = data.verification_reports
          }
          if (data.promotion_decision) {
            updates.promotionDecision = data.promotion_decision
          }
          if (Array.isArray(data.provider_scorecards)) {
            updates.providerScorecards = data.provider_scorecards
          }
          if (Array.isArray(data.failure_fingerprints)) {
            updates.failureFingerprints = data.failure_fingerprints
          }
          if (data.historical_learning) {
            updates.historicalLearning = data.historical_learning
          }
          if (data.prompt_pack_activation_request) {
            updates.promptPackActivationRequests = upsertPromptPackActivationRequest(prev.promptPackActivationRequests, data.prompt_pack_activation_request)
          } else if (Array.isArray(data.prompt_pack_activation_requests)) {
            updates.promptPackActivationRequests = data.prompt_pack_activation_requests
          }
          if (data.prompt_pack_version) {
            updates.promptPackVersions = upsertPromptPackVersion(prev.promptPackVersions, data.prompt_pack_version)
          } else if (Array.isArray(data.prompt_pack_versions)) {
            updates.promptPackVersions = data.prompt_pack_versions
          }
          if (data.prompt_pack_activation_event) {
            updates.promptPackActivationEvents = upsertPromptPackActivationEvent(prev.promptPackActivationEvents, data.prompt_pack_activation_event)
          } else if (Array.isArray(data.prompt_pack_activation_events)) {
            updates.promptPackActivationEvents = data.prompt_pack_activation_events
          }
          if (data.truth_by_surface) {
            updates.truthBySurface = data.truth_by_surface
          }
          if (typeof data.quality_gate_required === 'boolean') {
            updates.qualityGateRequired = data.quality_gate_required
          }
          if (typeof data.quality_gate_stage === 'string') {
            updates.qualityGateStage = data.quality_gate_stage
          }
          if (typeof data.quality_gate_passed === 'boolean') {
            updates.qualityGateStatus = data.quality_gate_passed ? 'passed' : 'failed'
          } else if (data.quality_gate_active === true) {
            updates.qualityGateStatus = 'running'
          } else if (data.quality_gate_required === true && prev.status !== 'completed' && prev.status !== 'failed') {
            updates.qualityGateStatus = prev.qualityGateStatus || 'pending'
          }

          if (data.inactivity_warning) {
            addSystemMessage(`${data.message}`)
          }

          if (resumingFromTerminal) {
            setIsBuilding(true)
            persistActiveBuildId(prev.id)
          }

          return { ...prev, ...updates }
        })
        break

      case 'agent:spawned':
        addSystemMessage(`${getAgentEmoji(data.role)} ${formatRole(data.role)} agent joined the team`)
        setBuildState(prev => {
          if (!prev) return null
          const newAgent: Agent = {
            id: message.agent_id,
            role: data.role,
            provider: data.provider,
            model: data.model,
            status: 'idle',
            progress: 0,
          }
          const existing = prev.agents.find(a => a.id === message.agent_id)
          if (existing) {
            return {
              ...prev,
              agents: prev.agents.map(a =>
                a.id === message.agent_id
                  ? { ...a, role: data.role ?? a.role, provider: data.provider ?? a.provider, model: data.model ?? a.model }
                  : a
              ),
            }
          }
          return { ...prev, agents: [...prev.agents, newAgent] }
        })
        addAiThought(
          message.agent_id,
          data.role,
          data.provider,
          data.model,
          'action',
          `${formatRole(data.role)} agent joined with ${humanizeIdentifier(data.provider) || 'configured provider'}${data.model ? ` / ${getModelDisplayName(data.model, activePowerMode) || data.model}` : ''}`,
          { eventType: 'agent:spawned' }
        )
        break

      case 'agent:working':
        setBuildState(prev => {
          if (!prev) return null
          const nextAgents: Agent[] = prev.agents.map((a): Agent =>
            a.id === message.agent_id
              ? {
                ...a,
                status: 'working' as Agent['status'],
                provider: data.provider ?? a.provider,
                model: data.model ?? a.model,
                currentTask: { type: data.task_type, description: data.description }
              }
              : a
          )
          return {
            ...prev,
            agents: nextAgents,
            progress: Math.max(prev.progress, computeAgentProgressFloor(nextAgents)),
          }
        })
        addAiThought(
          message.agent_id,
          data.agent_role,
          data.provider,
          data.model,
          'action',
          data.description || `Working on ${humanizeIdentifier(data.task_type) || 'current task'}`,
          {
            eventType: 'agent:working',
            taskId: data.task_id,
            taskType: data.task_type,
          }
        )
        break

      case 'agent:completed':
        {
          const guarantee = extractGuaranteeState(data)
          if (guarantee) {
            setBuildState(prev => prev ? { ...prev, guarantee } : prev)
          }
        }
        setBuildState(prev => {
          if (!prev) return null
          const nextAgents: Agent[] = prev.agents.map((a): Agent =>
            a.id === message.agent_id
              ? {
                ...a,
                status: 'completed' as Agent['status'],
                progress: 100,
                provider: data.provider ?? a.provider,
                model: data.model ?? a.model,
              }
              : a
          )
          return {
            ...prev,
            agents: nextAgents,
            progress: Math.max(prev.progress, computeAgentProgressFloor(nextAgents)),
          }
        })
        addAiThought(
          message.agent_id,
          data.agent_role,
          data.provider,
          data.model,
          'output',
          data.content || `${humanizeIdentifier(data.agent_role) || 'Agent'} completed the current task`,
          {
            eventType: 'agent:completed',
            taskId: data.task_id,
            taskType: data.task_type,
          }
        )
        break

      case 'agent:error':
        addSystemMessage(`Agent encountered an error: ${data.error}`)
        setBuildState(prev => {
          if (!prev) return null
          const nextAgents: Agent[] = prev.agents.map((a): Agent =>
            a.id === message.agent_id
              ? {
                ...a,
                status: 'error' as Agent['status'],
                provider: data.provider ?? a.provider,
                model: data.model ?? a.model,
              }
              : a
          )
          return {
            ...prev,
            agents: nextAgents,
            progress: Math.max(prev.progress, computeAgentProgressFloor(nextAgents)),
          }
        })
        addAiThought(
          message.agent_id,
          data.agent_role,
          data.provider,
          data.model,
          'error',
          data.error || 'Agent error',
          {
            eventType: 'agent:error',
            taskId: data.task_id,
            taskType: data.task_type,
          }
        )
        break

      case 'file:created':
        addSystemMessage(`Created: ${data.path}`)
        if (data.content) {
          setGeneratedFiles(prev => {
            const filtered = prev.filter(f => f.path !== data.path)
            return [...filtered, {
              path: data.path,
              content: data.content,
              language: data.language || 'text'
            }]
          })
        }
        break

      case 'file:updated':
        if (data.path && data.content !== undefined) {
          setGeneratedFiles(prev => prev.map(f =>
            f.path === data.path
              ? { ...f, content: data.content, language: data.language || f.language }
              : f
          ))
        }
        break

      case 'terminal:output':
        if (data.output) {
          addSystemMessage(data.output)
        }
        break

      case 'build:checkpoint':
        addSystemMessage(`Checkpoint ${data.number}: ${data.name}`)
        setBuildState(prev => {
          if (!prev) return null
          const checkpoint: Checkpoint = {
            id: data.checkpoint_id,
            number: data.number,
            name: data.name,
            description: data.description,
            progress: typeof data.progress === 'number' ? clampPercent(data.progress) : prev.progress,
            restorable: data.restorable !== false,
            createdAt: new Date().toISOString(),
          }
          return { ...prev, checkpoints: [...prev.checkpoints, checkpoint] }
        })
        break

      case 'build:completed': {
        setIsBuilding(false)
        clearActiveBuildId()
        const finalStatus = resolveBuildCompletedEventStatus(data.status)
        const failureReason = finalStatus === 'failed' ? extractBuildFailureReason(data) : undefined
        if (finalStatus === 'completed') {
          addSystemMessage(`Build completed successfully! ${data.files_count || 0} files generated.`)
        } else {
          addSystemMessage(`Build finished with errors. ${data.files_count || 0} files generated.`)
          if (failureReason) {
            addSystemMessage(`Failure reason: ${failureReason}`)
          }
        }
        setBuildState(prev => prev
          ? {
            ...prev,
            status: finalStatus,
            progress: finalStatus === 'completed' ? 100 : (typeof data.progress === 'number' ? clampPercent(data.progress) : prev.progress),
            currentPhase: finalStatus === 'completed' ? 'Completed' : (prev.currentPhase || 'Validation'),
            qualityGateRequired: typeof data.quality_gate_required === 'boolean' ? data.quality_gate_required : true,
            qualityGateStage: typeof data.quality_gate_stage === 'string' ? data.quality_gate_stage : prev.qualityGateStage,
            qualityGateStatus: typeof data.quality_gate_passed === 'boolean'
              ? (data.quality_gate_passed ? 'passed' : 'failed')
              : (finalStatus === 'completed' ? 'passed' : 'failed'),
            errorMessage: failureReason,
          }
          : null
        )

        // Reconcile file manifest — merge any files not already in state.
        if (data.files && Array.isArray(data.files)) {
          const normalized = normalizeGeneratedFiles(data.files)
          mergeGeneratedFiles(normalized)
        }

        if (generatedFilesRef.current.length === 0 && (!data.files || data.files.length === 0)) {
          const buildId = message.build_id || buildState?.id
          if (buildId) {
            await resolveGeneratedFiles(buildId)
          }
        }
        const completedBuildId = String(message.build_id || buildState?.id || '')

        if (finalStatus === 'completed' && !previewPreparedRef.current && completedBuildId) {
          previewPreparedRef.current = true
          void (async () => {
            const project = await preparePreview(true)
            if (project) {
              onNavigateToIDE?.({ target: 'preview', projectId: project.id })
            } else {
              previewPreparedRef.current = false
            }
          })()
        }
        break
      }

      case 'lead:response': {
        if (data.message) {
          upsertConversationMessage(data.message)
        } else {
          const content = typeof data.content === 'string'
            ? data.content
            : JSON.stringify(data.content ?? '')
          setChatMessages(prev => [...prev, {
            id: Date.now().toString(),
            role: 'lead',
            content,
            timestamp: new Date(),
            status: 'sent',
          }])
        }
        syncInteractionState(data.interaction, data.message ? [data.message] : undefined)
        break
      }

      case 'user:message':
        if (data.message) {
          upsertConversationMessage(data.message)
        }
        syncInteractionState(data.interaction, data.message ? [data.message] : undefined)
        break

      case 'agent:message': {
        const sourceLabel = humanizeIdentifier(data.source_agent_role || data.source_role || 'planner')
        const content = typeof data.content === 'string' && data.content.trim()
          ? `From ${sourceLabel}: ${data.content.trim()}`
          : `From ${sourceLabel}`
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            agents: prev.agents.map(a =>
              a.id === message.agent_id
                ? { ...a, provider: data.provider ?? a.provider, model: data.model ?? a.model }
                : a
            ),
          }
        })
        addAiThought(
          message.agent_id,
          data.agent_role,
          data.provider,
          data.model,
          'action',
          content,
          {
            eventType: 'agent:message',
          }
        )
        break
      }

      case 'build:interaction':
        syncInteractionState(data.interaction, data.messages)
        break

      case 'build:user-input-required':
        syncInteractionState(data.interaction, data.messages)
        if (data.question) {
          addSystemMessage(`Action needed: ${data.question}`)
        }
        break

      case 'build:user-input-resolved':
        syncInteractionState(data.interaction, data.messages)
        break

      case 'build:permission-request':
        syncInteractionState(data.interaction, data.messages)
        if (data.request?.reason) {
          addSystemMessage(`Permission requested: ${data.request.reason}`)
        }
        break

      case 'build:permission-update':
        syncInteractionState(data.interaction, data.messages)
        break

      case 'agent:thinking':
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            agents: prev.agents.map(a =>
              a.id === message.agent_id
                ? {
                  ...a,
                  status: 'working' as Agent['status'],
                  provider: data.provider ?? a.provider,
                  model: data.model ?? a.model,
                  currentTask: data.task_type
                    ? {
                      type: data.task_type,
                      description: a.currentTask?.description || humanizeIdentifier(data.task_type),
                    }
                    : a.currentTask,
                }
                : a
            ),
          }
        })
        addAiThought(message.agent_id, data.agent_role, data.provider, data.model, 'thinking', data.content, {
          eventType: 'agent:thinking',
          taskId: data.task_id,
          taskType: data.task_type,
          isInternal: true,
        })
        break

      case 'agent:action':
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            agents: prev.agents.map(a =>
              a.id === message.agent_id
                ? { ...a, provider: data.provider ?? a.provider, model: data.model ?? a.model }
                : a
            ),
          }
        })
        addAiThought(message.agent_id, data.agent_role, data.provider, data.model, 'action', data.content, {
          eventType: 'agent:action',
          taskId: data.task_id,
          taskType: data.task_type,
        })
        break

      case 'agent:output':
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            agents: prev.agents.map(a =>
              a.id === message.agent_id
                ? { ...a, provider: data.provider ?? a.provider, model: data.model ?? a.model }
                : a
            ),
          }
        })
        addAiThought(message.agent_id, data.agent_role, data.provider, data.model, 'output', data.content, {
          eventType: 'agent:output',
          taskId: data.task_id,
          taskType: data.task_type,
          filesCount: typeof data.files_count === 'number' ? data.files_count : undefined,
        })
        break

      case 'message:error':
        addSystemMessage(`Message failed: ${data.message || data.error || 'AI provider unavailable'}`)
        break

      case 'build:error':
        addSystemMessage(`Build Error: ${data.error || 'Unknown error'}${data.details ? ` - ${data.details}` : ''}`)
        if (data.files && Array.isArray(data.files)) {
          mergeGeneratedFiles(normalizeGeneratedFiles(data.files))
        }
        if (data.recoverable) {
          break
        }
        {
          const failureReason = extractBuildFailureReason(data)
          if (failureReason && failureReason !== data.error) {
            addSystemMessage(`Failure reason: ${failureReason}`)
          }
        }
        setIsBuilding(false)
        clearActiveBuildId()
        setBuildState(prev => prev
          ? {
            ...prev,
            status: 'failed',
            progress: typeof data.progress === 'number' ? clampPercent(data.progress) : prev.progress,
            currentPhase: prev.currentPhase || 'Validation',
            qualityGateRequired: typeof data.quality_gate_required === 'boolean' ? data.quality_gate_required : true,
            qualityGateStage: typeof data.quality_gate_stage === 'string' ? data.quality_gate_stage : prev.qualityGateStage,
            qualityGateStatus: 'failed',
            errorMessage: extractBuildFailureReason(data) || prev.errorMessage,
          }
          : null
        )
        break

      case 'build:phase':
        addSystemMessage(typeof data.message === 'string' && data.message.trim()
          ? data.message.trim()
          : `Phase: ${data.phase || 'Next phase starting'}`)
        setBuildState(prev => prev
          ? {
            ...prev,
            status: (mergeBuildStatusWithTerminalPrecedence(prev.status, data.status) || prev.status) as BuildState['status'],
            currentPhase: data.phase_key || data.phase || prev.currentPhase,
            qualityGateRequired: typeof data.quality_gate_required === 'boolean' ? data.quality_gate_required : prev.qualityGateRequired,
            qualityGateStage: typeof data.quality_gate_stage === 'string' ? data.quality_gate_stage : prev.qualityGateStage,
            qualityGateStatus: data.quality_gate_active ? 'running' : prev.qualityGateStatus,
          }
          : null
        )
        break

      case 'build:started':
        addSystemMessage('Build initialized, spawning agents...')
        setBuildState(prev => prev ? {
          ...prev,
          status: ((normalizeBuildStatus(data.status) || 'planning') as BuildState['status']),
          powerMode: data.power_mode || data.powerMode || prev.powerMode,
          currentPhase: data.phase || prev.currentPhase || 'Planning',
          qualityGateRequired: true,
          qualityGateStatus: prev.qualityGateStatus || 'pending',
          errorMessage: undefined,
        } : null)
        break

      case 'agent:generation_failed':
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            agents: prev.agents.map(a =>
              a.id === message.agent_id
                ? { ...a, provider: data.provider ?? a.provider, model: data.model ?? a.model }
                : a
            ),
          }
        })
        addSystemMessage(`AI generation failed for ${data.agent_role || 'agent'} (${data.provider || 'unknown'}): ${data.error || 'Unknown error'}`)
        addAiThought(
          message.agent_id,
          data.agent_role,
          data.provider,
          data.model,
          'error',
          data.error || 'Generation failed',
          {
            eventType: 'agent:generation_failed',
            taskId: data.task_id,
            taskType: data.task_type,
            retryCount: Number.isFinite(Number(data.retry_count ?? data.attempt)) ? Number(data.retry_count ?? data.attempt) : undefined,
            maxRetries: Number.isFinite(Number(data.max_retries)) ? Number(data.max_retries) : undefined,
          }
        )
        {
          const retryCountRaw = data.retry_count ?? data.attempt
          const maxRetriesRaw = data.max_retries
          const retryCount = Number.isFinite(Number(retryCountRaw)) ? Number(retryCountRaw) : undefined
          const maxRetries = Number.isFinite(Number(maxRetriesRaw)) ? Number(maxRetriesRaw) : undefined
          const willRetry = data.will_retry

          if (retryCount !== undefined && maxRetries !== undefined) {
            if (willRetry === true || (willRetry === undefined && retryCount < maxRetries)) {
              addSystemMessage(`Retrying... (attempt ${retryCount + 1}/${maxRetries})`)
            } else {
              addSystemMessage('Max retries reached. The AI provider may be unavailable.')
            }
          } else {
            addSystemMessage('The AI provider reported an unrecoverable error.')
          }
        }
        break

      case 'agent:generating':
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            agents: prev.agents.map(a =>
              a.id === message.agent_id
                ? {
                  ...a,
                  status: 'working' as Agent['status'],
                  provider: data.provider ?? a.provider,
                  model: data.model ?? a.model,
                  currentTask: data.task_type
                    ? {
                      type: data.task_type,
                      description: data.content || a.currentTask?.description || humanizeIdentifier(data.task_type),
                    }
                    : a.currentTask,
                }
                : a
            ),
          }
        })
        addAiThought(
          message.agent_id,
          data.agent_role,
          data.provider,
          data.model,
          'action',
          data.content || `Generating code with ${data.provider}...`,
          {
            eventType: 'agent:generating',
            taskId: data.task_id,
            taskType: data.task_type,
          }
        )
        break

      case 'agent:retrying':
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            agents: prev.agents.map(a =>
              a.id === message.agent_id
                ? { ...a, provider: data.provider ?? a.provider, model: data.model ?? a.model }
                : a
            ),
          }
        })
        addAiThought(
          message.agent_id,
          data.agent_role,
          data.provider,
          data.model,
          'action',
          data.message || `Retrying ${humanizeIdentifier(data.task_type) || 'task'}`,
          {
            eventType: 'agent:retrying',
            taskId: data.task_id,
            taskType: data.task_type,
            retryCount: Number.isFinite(Number(data.retry_count ?? data.attempt)) ? Number(data.retry_count ?? data.attempt) : undefined,
            maxRetries: Number.isFinite(Number(data.max_retries)) ? Number(data.max_retries) : undefined,
          }
        )
        {
          const retryCountRaw = data.retry_count ?? data.attempt
          const maxRetriesRaw = data.max_retries
          const retryCount = Number.isFinite(Number(retryCountRaw)) ? Number(retryCountRaw) : undefined
          const maxRetries = Number.isFinite(Number(maxRetriesRaw)) ? Number(maxRetriesRaw) : undefined
          if (retryCount !== undefined && maxRetries !== undefined) {
            addSystemMessage(`${data.agent_role || 'Agent'} retrying task (attempt ${retryCount}/${maxRetries})...`)
          } else {
            addSystemMessage(`${data.agent_role || 'Agent'} retrying task...`)
          }
        }
        break

      case 'agent:verification_failed':
      case 'agent:coordination_failed': {
        const knownAgent = buildStateRef.current?.agents.find(a => a.id === message.agent_id)
        const errors = Array.isArray(data.errors)
          ? data.errors.map((entry: unknown) => String(entry)).filter(Boolean)
          : []
        const isVerificationFailure = type === 'agent:verification_failed'
        const content = [
          data.message || (isVerificationFailure
            ? 'Build verification failed; retrying with error context.'
            : 'Coordination contract failed; retrying with work-order context.'),
          errors.length > 0 ? errors.slice(0, 3).join('; ') : '',
        ].filter(Boolean).join(' ')
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            agents: prev.agents.map(a =>
              a.id === message.agent_id
                ? {
                  ...a,
                  status: 'working' as Agent['status'],
                  provider: data.provider ?? a.provider,
                  model: data.model ?? a.model,
                }
                : a
            ),
          }
        })
        addSystemMessage(`${knownAgent?.role || data.agent_role || 'Agent'} ${isVerificationFailure ? 'verification' : 'coordination'} retry: ${errors[0] || data.message || 'repairing failed output'}`)
        addAiThought(
          message.agent_id,
          data.agent_role || knownAgent?.role || 'agent',
          data.provider || knownAgent?.provider || '',
          data.model || knownAgent?.model,
          'error',
          content,
          {
            eventType: type,
            taskId: data.task_id,
            taskType: data.task_type,
            retryCount: Number.isFinite(Number(data.retry_count ?? data.attempt)) ? Number(data.retry_count ?? data.attempt) : undefined,
            maxRetries: Number.isFinite(Number(data.max_retries)) ? Number(data.max_retries) : undefined,
          }
        )
        break
      }

      case 'code:generated':
        setBuildState(prev => {
          if (!prev) return null
          const nextAgents = prev.agents.map(a =>
            a.id === message.agent_id
              ? { ...a, provider: data.provider ?? a.provider, model: data.model ?? a.model, progress: Math.max(a.progress, 95) }
              : a
          )
          return {
            ...prev,
            agents: nextAgents,
            progress: Math.max(prev.progress, computeAgentProgressFloor(nextAgents)),
          }
        })
        addSystemMessage(`${data.agent_role || 'Agent'} generated ${data.files_count || 0} file(s)`)
        // Add output thought to thinking box for visibility
        {
          const fileList = Array.isArray(data.files) ? data.files.map((f: any) => f.path).filter(Boolean) : []
          const outputSummary = fileList.length > 0
            ? `Generated ${fileList.length} file(s): ${fileList.slice(0, 5).join(', ')}${fileList.length > 5 ? ` (+${fileList.length - 5} more)` : ''}`
            : `Generated ${data.files_count || 0} file(s)`
          addAiThought(
            message.agent_id,
            data.agent_role || 'agent',
            data.provider || '',
            data.model,
            'output',
            outputSummary,
            {
              eventType: 'code:generated',
              taskId: data.task_id,
              taskType: data.task_type,
              files: fileList,
              filesCount: typeof data.files_count === 'number' ? data.files_count : fileList.length,
            }
          )
        }
        if (data.files && Array.isArray(data.files)) {
          const newFiles = data.files
            .filter((file: any) => file.path && file.content)
            .map((file: any) => ({
              path: file.path,
              content: file.content,
              language: file.language || 'text'
            }))
          if (newFiles.length > 0) {
            setGeneratedFiles(prev => {
              const existingPaths = new Set(newFiles.map((f: any) => f.path))
              const filtered = prev.filter(f => !existingPaths.has(f.path))
              return [...filtered, ...newFiles]
            })
          }
        }
        break

      case 'agent:progress':
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            agents: prev.agents.map(a =>
              a.id === message.agent_id
                ? {
                  ...a,
                  status: 'working' as Agent['status'],
                  progress: typeof data.progress === 'number' ? clampPercent(data.progress) : a.progress,
                  provider: data.provider ?? a.provider,
                  model: data.model ?? a.model,
                }
                : a
            )
          }
        })
        break

      case 'agent:provider_switched':
        addSystemMessage(`${data.agent_role || 'Agent'} switched provider: ${data.old_provider || 'unknown'} → ${data.new_provider || 'unknown'}`)
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            agents: prev.agents.map(a =>
              a.id === message.agent_id
                ? {
                  ...a,
                  status: 'working' as Agent['status'],
                  provider: data.new_provider ?? a.provider,
                  model: data.model ?? a.model,
                }
                : a
            )
          }
        })
        addAiThought(
          message.agent_id,
          data.agent_role,
          data.new_provider || data.provider,
          data.model,
          'action',
          `${humanizeIdentifier(data.agent_role) || 'Agent'} switched provider from ${humanizeIdentifier(data.old_provider) || 'unknown'} to ${humanizeIdentifier(data.new_provider) || 'unknown'}`,
          {
            eventType: 'agent:provider_switched',
          }
        )
        break

      case 'spend:update':
        if (data.billed_cost) {
          addSpendToast(data.agent_role || 'agent', Number(data.billed_cost))
          addAiThought(
            message.agent_id,
            data.agent_role || 'agent',
            data.provider || '',
            data.model,
            'output',
            `Spend recorded: $${Number(data.billed_cost).toFixed(4)} billed`,
            {
              eventType: 'spend:update',
            }
          )
        }
        break

      case 'glassbox:war_room_critique_started':
      case 'glassbox:war_room_critique_resolved':
      case 'glassbox:work_order_compiled':
      case 'glassbox:provider_route_selected':
      case 'glassbox:deterministic_gate_passed':
      case 'glassbox:deterministic_gate_failed':
      case 'glassbox:hydra_candidate_started':
      case 'glassbox:hydra_candidate_passed':
      case 'glassbox:hydra_candidate_failed':
      case 'glassbox:hydra_winner_selected':
      case 'glassbox:patch_review_required': {
        const eventType = String(type)
        const thoughtType: AIThought['type'] = eventType.includes('_failed') || eventType.includes('gate_failed')
          ? 'error'
          : eventType.includes('_passed') || eventType.includes('_resolved') || eventType.includes('winner_selected')
            ? 'output'
            : 'action'
        const fallbackContent = humanizeIdentifier(eventType.replace(/^glassbox:/, ''))
        addAiThought(
          message.agent_id || `glassbox-${eventType}`,
          data.agent_role || 'orchestrator',
          data.provider || 'orchestrator',
          data.model,
          thoughtType,
          data.content || data.error || fallbackContent,
          {
            eventType,
            taskId: data.task_id,
            taskType: data.task_type,
            files: Array.isArray(data.files) ? data.files : undefined,
            filesCount: typeof data.files_count === 'number' ? data.files_count : undefined,
          }
        )
        setBuildState(prev => {
          if (!prev) return prev
          const updates: Partial<BuildState> = {}
          if (Array.isArray(data.work_orders)) {
            updates.workOrders = data.work_orders
          }
          if (Array.isArray(data.verification_reports)) {
            updates.verificationReports = data.verification_reports
          }
          if (Array.isArray(data.patch_bundles)) {
            updates.patchBundles = data.patch_bundles
          } else if (data.patch_bundle && typeof data.patch_bundle === 'object') {
            const patchBundle = data.patch_bundle as BuildPatchBundleState
            const existing = prev.patchBundles || []
            const existingIndex = patchBundle?.id ? existing.findIndex((bundle) => bundle.id === patchBundle.id) : -1
            updates.patchBundles = existingIndex >= 0
              ? existing.map((bundle, index) => index === existingIndex ? patchBundle : bundle)
              : [...existing, patchBundle]
          }
          return Object.keys(updates).length > 0 ? { ...prev, ...updates } : prev
        })
        break
      }

      case 'budget:exceeded':
        addSystemMessage(`BUDGET EXCEEDED: ${data.message || 'Spending cap reached. Build stopped.'}`)
        setBuildState(prev => prev ? { ...prev, status: 'failed', errorMessage: 'Budget exceeded' } : null)
        break

      case 'budget:warning':
        addSystemMessage(`Budget warning: ${data.message || 'Approaching spending cap'}`)
        break

      case 'agent:propose-diff':
        addSystemMessage(`Agent ${data.agent_role || 'unknown'} proposed changes to ${data.file_count || 'multiple'} file(s) — review required`)
        setBuildState(prev => prev ? { ...prev, status: 'awaiting_review' } : null)
        void loadProposedEdits(message.build_id || buildStateRef.current?.id)
        break

      case 'build:edits-applied':
        addSystemMessage(`Approved edits applied: ${data.files_count || 0} file(s) written`)
        setBuildState(prev => prev ? { ...prev, status: 'in_progress' } : null)
        break

      case 'build:awaiting-review':
        addSystemMessage('Build paused — awaiting diff review')
        setBuildState(prev => prev ? { ...prev, status: 'awaiting_review' } : null)
        void loadProposedEdits(message.build_id || buildStateRef.current?.id)
        break

      case 'agent:protected-path':
        addSystemMessage(`Protected path: ${data.path || 'unknown'} — agent cannot modify this file`)
        break

      case 'build:rollback':
        addSystemMessage(`Rolled back to checkpoint "${data.checkpoint_name || 'unknown'}". ${data.files_restored || 0} file(s) restored.`)
        setBuildState(prev => {
          if (!prev) return null
          return {
            ...prev,
            status: 'in_progress',
            progress: typeof data.progress === 'number' ? data.progress : prev.progress,
            guarantee: {
              status: 'retrying',
              verdict: prev.guarantee?.verdict || 'soft_fail',
              attempts: prev.guarantee?.attempts || 1,
              score: prev.guarantee?.score,
              rolledBack: true,
              durationMs: prev.guarantee?.durationMs,
              error: prev.guarantee?.error,
              taskId: prev.guarantee?.taskId,
              taskType: prev.guarantee?.taskType,
              updatedAt: new Date().toISOString(),
            },
          }
        })
        break

      case 'preview:ready':
        if (data.url) {
          addSystemMessage(`Preview ready: ${data.url}`)
          setBuildState(prev => prev ? { ...prev, previewUrl: data.url } : null)
        }
        break

      case 'build:fsm:started':
        addSystemMessage(`Build engine started`)
        setBuildState(prev => prev ? { ...prev, status: 'planning' } : null)
        break

      case 'build:fsm:initialized':
        addSystemMessage(`Build pipeline initialized`)
        break

      case 'build:fsm:plan_ready':
        addSystemMessage(`Build plan ready — executing`)
        setBuildState(prev => prev ? { ...prev, status: 'in_progress' } : null)
        break

      case 'build:fsm:step_complete':
        if (typeof data.progress === 'number') {
          setBuildState(prev => prev ? { ...prev, progress: Math.round(data.progress * 100) } : null)
        }
        break

      case 'build:fsm:all_steps_complete':
        addSystemMessage(`All build steps complete — validating`)
        setBuildState(prev => prev ? {
          ...prev,
          status: 'reviewing',
          progress: 95,
          qualityGateStatus: 'running',
          guarantee: prev.guarantee || {
            status: 'validating',
            attempts: 1,
            rolledBack: false,
            updatedAt: new Date().toISOString(),
          },
        } : null)
        break

      case 'build:fsm:validation_pass':
        addSystemMessage(`Build validated successfully`)
        setBuildState(prev => prev ? {
          ...prev,
          qualityGateStatus: 'passed',
          progress: 100,
          guarantee: {
            status: 'passed',
            verdict: 'pass',
            attempts: prev.guarantee?.attempts || 1,
            score: prev.guarantee?.score ?? 100,
            rolledBack: false,
            durationMs: prev.guarantee?.durationMs,
            error: undefined,
            taskId: prev.guarantee?.taskId,
            taskType: prev.guarantee?.taskType,
            updatedAt: new Date().toISOString(),
          },
        } : null)
        break

      case 'build:fsm:validation_fail':
        addSystemMessage(`Validation failed — retrying (attempt ${(data.retry_count ?? 0) + 1})`)
        setBuildState(prev => prev ? {
          ...prev,
          qualityGateStatus: 'failed',
          guarantee: {
            status: 'retrying',
            verdict: 'soft_fail',
            attempts: Number.isFinite(Number(data.retry_count)) ? Number(data.retry_count) + 1 : (prev.guarantee?.attempts || 1),
            score: prev.guarantee?.score,
            rolledBack: false,
            durationMs: prev.guarantee?.durationMs,
            error: typeof data.error === 'string' ? data.error : prev.guarantee?.error,
            taskId: prev.guarantee?.taskId,
            taskType: prev.guarantee?.taskType,
            updatedAt: new Date().toISOString(),
          },
        } : null)
        break

      case 'build:fsm:retry_exhausted':
        addSystemMessage(`All retry attempts exhausted — initiating rollback`)
        setBuildState(prev => prev ? {
          ...prev,
          guarantee: {
            status: 'rolling_back',
            verdict: prev.guarantee?.verdict || 'hard_fail',
            attempts: prev.guarantee?.attempts || 1,
            score: prev.guarantee?.score,
            rolledBack: false,
            durationMs: prev.guarantee?.durationMs,
            error: prev.guarantee?.error,
            taskId: prev.guarantee?.taskId,
            taskType: prev.guarantee?.taskType,
            updatedAt: new Date().toISOString(),
          },
        } : null)
        break

      case 'build:fsm:rollback_complete':
        addSystemMessage(`Rollback complete — build failed after exhausting retries`)
        setBuildState(prev => prev ? {
          ...prev,
          status: 'failed',
          errorMessage: 'Build failed after exhausting all retry attempts',
          guarantee: {
            status: 'failed',
            verdict: 'hard_fail',
            attempts: prev.guarantee?.attempts || 1,
            score: prev.guarantee?.score,
            rolledBack: true,
            durationMs: prev.guarantee?.durationMs,
            error: 'Build failed after exhausting all retry attempts',
            taskId: prev.guarantee?.taskId,
            taskType: prev.guarantee?.taskType,
            updatedAt: new Date().toISOString(),
          },
        } : null)
        break

      case 'build:fsm:rollback_failed':
        addSystemMessage(`Rollback failed: ${data.error || 'unknown error'}`)
        setBuildState(prev => prev ? {
          ...prev,
          status: 'failed',
          errorMessage: data.error || 'Rollback failed',
          guarantee: {
            status: 'failed',
            verdict: 'hard_fail',
            attempts: prev.guarantee?.attempts || 1,
            score: prev.guarantee?.score,
            rolledBack: true,
            durationMs: prev.guarantee?.durationMs,
            error: data.error || 'Rollback failed',
            taskId: prev.guarantee?.taskId,
            taskType: prev.guarantee?.taskType,
            updatedAt: new Date().toISOString(),
          },
        } : null)
        break

      case 'build:guarantee:result': {
        const guarantee = extractGuaranteeState(data)
        if (guarantee) {
          const summary = guarantee.status === 'passed'
            ? `Guarantee check passed at ${Math.round(guarantee.score ?? 100)}% confidence`
            : guarantee.status === 'retrying'
              ? `Guarantee loop requested retry ${guarantee.attempts}`
              : guarantee.rolledBack
                ? 'Guarantee loop rolled the build back'
                : 'Guarantee check reported a failure'
          addSystemMessage(summary)
          setBuildState(prev => prev ? { ...prev, guarantee } : prev)
        }
        break
      }

      case 'build:fsm:paused':
        addSystemMessage(`Build paused`)
        syncInteractionState(data.interaction, data.messages)
        break

      case 'build:fsm:resumed':
        addSystemMessage(`Build resumed`)
        setBuildState(prev => prev ? {
          ...prev,
          status: 'in_progress',
          interaction: normalizeInteraction(data.interaction, data.messages) || prev.interaction,
        } : null)
        syncInteractionState(data.interaction, data.messages)
        break

      case 'build:fsm:cancelled':
        addSystemMessage(`Build cancelled`)
        setBuildState(prev => prev ? { ...prev, status: 'cancelled' } : null)
        break

      case 'build:fsm:fatal_error':
        addSystemMessage(`Fatal build error: ${data.error || 'unknown'}`)
        setBuildState(prev => prev ? { ...prev, status: 'failed', errorMessage: data.error || 'Fatal error' } : null)
        break

      case 'build:fsm:checkpoint_created':
        if (data.checkpoint_id) {
          addSystemMessage(`Checkpoint saved`)
          setBuildState(prev => {
            if (!prev) return null
            const checkpoint = { id: data.checkpoint_id, number: (prev.checkpoints?.length ?? 0) + 1, name: `Checkpoint ${(prev.checkpoints?.length ?? 0) + 1}`, description: data.step_id || '', progress: prev.progress, restorable: true, createdAt: data.timestamp || new Date().toISOString() }
            return { ...prev, checkpoints: [...(prev.checkpoints ?? []), checkpoint] }
          })
        }
        break
    }
  }

  wsMessageHandlerRef.current = handleWebSocketMessage

  // Add AI thought
  const addAiThought = (
    agentId: string,
    agentRole: string,
    provider: string,
    model: string | undefined,
    type: AIThought['type'],
    content: string,
    metadata: Partial<Omit<AIThought, 'id' | 'agentId' | 'agentRole' | 'provider' | 'model' | 'type' | 'content' | 'timestamp'>> = {}
  ) => {
    const knownAgent = buildStateRef.current?.agents.find(a => a.id === agentId)
    const thought: AIThought = {
      id: Date.now().toString() + Math.random(),
      agentId,
      agentRole: agentRole || knownAgent?.role || 'agent',
      provider: provider || knownAgent?.provider || 'unknown',
      model: model || knownAgent?.model,
      type,
      content: content || `${agentRole || knownAgent?.role || 'Agent'} update`,
      eventType: metadata.eventType,
      taskId: metadata.taskId,
      taskType: metadata.taskType,
      files: metadata.files,
      filesCount: metadata.filesCount,
      retryCount: metadata.retryCount,
      maxRetries: metadata.maxRetries,
      isInternal: metadata.isInternal,
      timestamp: new Date(),
    }
    setAiThoughts(prev => {
      const updated = [...prev, thought]
      return updated.slice(-MAX_AI_THOUGHTS)
    })
  }

  const normalizeConversationMessages = useCallback((messages: unknown): ChatMessage[] => {
    if (!Array.isArray(messages)) return []
    return messages
      .filter((message): message is ApiBuildConversationMessage => Boolean(message && typeof message === 'object'))
      .map((message) => ({
        id: String(message.id || `${message.role || 'system'}-${message.timestamp || Date.now()}`),
        role: (message.role === 'user' || message.role === 'lead' ? message.role : 'system') as ChatMessage['role'],
        content: String(message.content || ''),
        timestamp: message.timestamp ? new Date(message.timestamp) : new Date(),
        kind: message.kind,
        agentRole: message.agent_role,
        targetMode: message.target_mode,
        targetAgentId: message.target_agent_id,
        targetAgentRole: message.target_agent_role,
        clientToken: message.client_token,
        status: (message.status === 'failed' ? 'failed' : 'sent') as ChatMessage['status'],
      }))
      .filter((message) => message.content.trim().length > 0)
  }, [])

  const normalizeInteraction = useCallback((interaction: any, fallbackMessages?: unknown): BuildInteractionState | undefined => {
    if (!interaction || typeof interaction !== 'object') {
      if (Array.isArray(fallbackMessages) && fallbackMessages.length > 0) {
        return { messages: fallbackMessages as ApiBuildConversationMessage[] }
      }
      return undefined
    }

    const normalizedMessages = Array.isArray(interaction.messages)
      ? interaction.messages
      : Array.isArray(fallbackMessages)
        ? fallbackMessages
        : []

    return {
      ...interaction,
      messages: normalizedMessages,
      steering_notes: Array.isArray(interaction.steering_notes) ? interaction.steering_notes : [],
      pending_revisions: Array.isArray(interaction.pending_revisions) ? interaction.pending_revisions : [],
      permission_rules: Array.isArray(interaction.permission_rules) ? interaction.permission_rules : [],
      permission_requests: Array.isArray(interaction.permission_requests) ? interaction.permission_requests : [],
      approval_events: Array.isArray(interaction.approval_events) ? interaction.approval_events : [],
    }
  }, [])

  const syncInteractionState = useCallback((interaction: any, fallbackMessages?: unknown) => {
    const normalizedInteraction = normalizeInteraction(interaction, fallbackMessages)
    if (!normalizedInteraction) return

    setBuildState(prev => prev ? { ...prev, interaction: normalizedInteraction } : prev)
    if (Array.isArray(normalizedInteraction.messages)) {
      setChatMessages(normalizeConversationMessages(normalizedInteraction.messages))
    }
  }, [normalizeConversationMessages, normalizeInteraction])

  const upsertConversationMessage = useCallback((message: any) => {
    if (!message || typeof message !== 'object') return
    const [normalized] = normalizeConversationMessages([message])
    if (!normalized) return

    setChatMessages(prev => {
      const byId = prev.findIndex((entry) => entry.id === normalized.id)
      if (byId >= 0) {
        const next = [...prev]
        next[byId] = normalized
        return next
      }

      if (normalized.clientToken) {
        const byToken = prev.findIndex((entry) => entry.clientToken === normalized.clientToken)
        if (byToken >= 0) {
          const next = [...prev]
          next[byToken] = normalized
          return next
        }
      }

      return [...prev, normalized]
    })
  }, [normalizeConversationMessages])

  const loadProposedEdits = useCallback(async (buildIdOverride?: string) => {
    const buildId = buildIdOverride || buildStateRef.current?.id
    if (!buildId) return []

    try {
      const response = await apiService.getBuildProposedEdits(buildId)
      const edits = Array.isArray(response.edits) ? response.edits : []
      setProposedEdits(edits)
      setShowDiffReview(edits.length > 0)
      return edits
    } catch (error) {
      setProposedEdits([])
      return []
    }
  }, [])

  const updateReviewedPatchBundle = useCallback((bundle: BuildPatchBundleState) => {
    if (!bundle?.id) return
    setBuildState(prev => {
      if (!prev) return prev
      const existing = prev.patchBundles || []
      const index = existing.findIndex(candidate => candidate.id === bundle.id)
      const patchBundles = index >= 0
        ? existing.map(candidate => candidate.id === bundle.id ? { ...candidate, ...bundle } : candidate)
        : [...existing, bundle]
      return { ...prev, patchBundles }
    })
  }, [])

  const markPatchBundleReviewStatus = useCallback((bundleId: string, reviewStatus: 'approved' | 'rejected') => {
    setBuildState(prev => {
      if (!prev) return prev
      return {
        ...prev,
        patchBundles: (prev.patchBundles || []).map(bundle => bundle.id === bundleId
          ? {
              ...bundle,
              review_status: reviewStatus,
              reviewed_at: bundle.reviewed_at || new Date().toISOString(),
            }
          : bundle
        ),
      }
    })
  }, [])

  const updateReviewedPromptProposal = useCallback((proposal: BuildPromptImprovementProposalState, historicalLearning?: BuildLearningSummaryState) => {
    if (!proposal?.id) return
    setBuildState(prev => {
      if (!prev) return prev
      const currentLearning = historicalLearning || prev.historicalLearning
      if (!currentLearning) return prev
      const existing = currentLearning.prompt_improvement_proposals || []
      const index = existing.findIndex(candidate => candidate.id === proposal.id)
      const promptImprovementProposals = index >= 0
        ? existing.map(candidate => candidate.id === proposal.id ? { ...candidate, ...proposal } : candidate)
        : [...existing, proposal]
      return {
        ...prev,
        historicalLearning: {
          ...currentLearning,
          prompt_improvement_proposals: promptImprovementProposals,
        },
      }
    })
  }, [])

  const markPromptProposalReviewStatus = useCallback((proposalId: string, reviewStatus: 'approved' | 'rejected') => {
    setBuildState(prev => {
      if (!prev?.historicalLearning) return prev
      return {
        ...prev,
        historicalLearning: {
          ...prev.historicalLearning,
          prompt_improvement_proposals: (prev.historicalLearning.prompt_improvement_proposals || []).map(proposal => proposal.id === proposalId
            ? {
                ...proposal,
                review_state: reviewStatus,
                reviewed_at: proposal.reviewed_at || new Date().toISOString(),
              }
            : proposal
          ),
        },
      }
    })
  }, [])

  const reviewPatchBundle = useCallback(async (bundleId: string, decision: 'approve' | 'reject') => {
    const buildId = buildStateRef.current?.id
    if (!buildId || !bundleId || patchBundleActionId) return

    setPatchBundleActionId(bundleId)
    try {
      const response = decision === 'approve'
        ? await apiService.approveBuildPatchBundle(buildId, bundleId)
        : await apiService.rejectBuildPatchBundle(buildId, bundleId)
      if (response.patch_bundle) {
        updateReviewedPatchBundle(response.patch_bundle)
      } else {
        markPatchBundleReviewStatus(bundleId, decision === 'approve' ? 'approved' : 'rejected')
      }
      const nextStatus = normalizeBuildStatus(response.status)
      if (nextStatus) {
        setBuildState(prev => prev ? { ...prev, status: nextStatus as BuildState['status'] } : prev)
      }
      addSystemMessage(response.message || (decision === 'approve' ? 'Patch bundle approved' : 'Patch bundle rejected'))
    } catch (error) {
      addSystemMessage(`Patch bundle review failed: ${error instanceof Error ? error.message : 'Unable to update patch bundle'}`)
    } finally {
      setPatchBundleActionId(null)
    }
  }, [addSystemMessage, markPatchBundleReviewStatus, patchBundleActionId, updateReviewedPatchBundle])

  const reviewPromptProposal = useCallback(async (proposalId: string, decision: 'approve' | 'reject') => {
    const buildId = buildStateRef.current?.id
    if (!buildId || !proposalId || promptProposalActionId) return

    setPromptProposalActionId(proposalId)
    try {
      const response = decision === 'approve'
        ? await apiService.approveBuildPromptProposal(buildId, proposalId)
        : await apiService.rejectBuildPromptProposal(buildId, proposalId)
      if (response.prompt_proposal) {
        updateReviewedPromptProposal(response.prompt_proposal, response.historical_learning)
      } else {
        markPromptProposalReviewStatus(proposalId, decision === 'approve' ? 'approved' : 'rejected')
      }
      const nextStatus = normalizeBuildStatus(response.status)
      if (nextStatus) {
        setBuildState(prev => prev ? { ...prev, status: nextStatus as BuildState['status'] } : prev)
      }
      addSystemMessage(response.message || (decision === 'approve' ? 'Prompt proposal approved' : 'Prompt proposal rejected'))
    } catch (error) {
      addSystemMessage(`Prompt proposal review failed: ${error instanceof Error ? error.message : 'Unable to update prompt proposal'}`)
    } finally {
      setPromptProposalActionId(null)
    }
  }, [addSystemMessage, markPromptProposalReviewStatus, promptProposalActionId, updateReviewedPromptProposal])

  const updatePromptLearning = useCallback((historicalLearning?: BuildLearningSummaryState) => {
    if (!historicalLearning) return
    setBuildState(prev => prev ? { ...prev, historicalLearning } : prev)
  }, [])

  const benchmarkPromptProposal = useCallback(async (proposalId: string) => {
    const buildId = buildStateRef.current?.id
    if (!buildId || !proposalId || promptProposalActionId) return

    setPromptProposalActionId(proposalId)
    try {
      const response = await apiService.benchmarkBuildPromptProposal(buildId, proposalId)
      if (response.prompt_proposal) {
        updateReviewedPromptProposal(response.prompt_proposal, response.historical_learning)
      }
      const nextStatus = normalizeBuildStatus(response.status)
      if (nextStatus) {
        setBuildState(prev => prev ? { ...prev, status: nextStatus as BuildState['status'] } : prev)
      }
      addSystemMessage(response.message || `Prompt proposal benchmark ${response.benchmark_status || 'recorded'}`)
    } catch (error) {
      addSystemMessage(`Prompt proposal benchmark failed: ${error instanceof Error ? error.message : 'Unable to run prompt proposal benchmark'}`)
    } finally {
      setPromptProposalActionId(null)
    }
  }, [addSystemMessage, promptProposalActionId, updateReviewedPromptProposal])

  const createPromptPackDraft = useCallback(async () => {
    const buildId = buildStateRef.current?.id
    if (!buildId || promptProposalActionId) return

    setPromptProposalActionId('prompt-pack-draft')
    try {
      const response = await apiService.createBuildPromptPackDraft(buildId)
      updatePromptLearning(response.historical_learning)
      const nextStatus = normalizeBuildStatus(response.status)
      if (nextStatus) {
        setBuildState(prev => prev ? { ...prev, status: nextStatus as BuildState['status'] } : prev)
      }
      addSystemMessage(response.message || 'Prompt pack draft created')
    } catch (error) {
      addSystemMessage(`Prompt pack draft failed: ${error instanceof Error ? error.message : 'Unable to create prompt pack draft'}`)
    } finally {
      setPromptProposalActionId(null)
    }
  }, [addSystemMessage, promptProposalActionId, updatePromptLearning])

  const requestPromptPackActivation = useCallback(async (draftId: string) => {
    const buildId = buildStateRef.current?.id
    if (!buildId || !draftId || promptProposalActionId) return

    setPromptProposalActionId(`prompt-pack-activation:${draftId}`)
    try {
      const response = await apiService.requestBuildPromptPackActivation(buildId, draftId)
      if (response.prompt_pack_activation_request) {
        setBuildState(prev => prev
          ? {
              ...prev,
              promptPackActivationRequests: upsertPromptPackActivationRequest(prev.promptPackActivationRequests, response.prompt_pack_activation_request!),
            }
          : prev
        )
      }
      const nextStatus = normalizeBuildStatus(response.status)
      if (nextStatus) {
        setBuildState(prev => prev ? { ...prev, status: nextStatus as BuildState['status'] } : prev)
      }
      addSystemMessage(response.message || 'Prompt pack activation request recorded')
    } catch (error) {
      addSystemMessage(`Prompt pack activation request failed: ${error instanceof Error ? error.message : 'Unable to request prompt pack activation'}`)
    } finally {
      setPromptProposalActionId(null)
    }
  }, [addSystemMessage, promptProposalActionId])

  const activatePromptPackRequest = useCallback(async (requestId: string) => {
    const buildId = buildStateRef.current?.id
    if (!buildId || !requestId || promptProposalActionId) return

    setPromptProposalActionId(`prompt-pack-registry:${requestId}`)
    try {
      const response = await apiService.activateBuildPromptPackRequest(buildId, requestId)
      setBuildState(prev => {
        if (!prev) return prev
        return {
          ...prev,
          promptPackActivationRequests: response.prompt_pack_activation_request
            ? upsertPromptPackActivationRequest(prev.promptPackActivationRequests, response.prompt_pack_activation_request)
            : prev.promptPackActivationRequests,
          promptPackVersions: response.prompt_pack_version
            ? upsertPromptPackVersion(prev.promptPackVersions, response.prompt_pack_version)
            : prev.promptPackVersions,
          promptPackActivationEvents: response.prompt_pack_activation_event
            ? upsertPromptPackActivationEvent(prev.promptPackActivationEvents, response.prompt_pack_activation_event)
            : prev.promptPackActivationEvents,
        }
      })
      const nextStatus = normalizeBuildStatus(response.status)
      if (nextStatus) {
        setBuildState(prev => prev ? { ...prev, status: nextStatus as BuildState['status'] } : prev)
      }
      addSystemMessage(response.message || 'Prompt pack version activated in registry')
    } catch (error) {
      addSystemMessage(`Prompt pack registry activation failed: ${error instanceof Error ? error.message : 'Unable to activate prompt pack request'}`)
    } finally {
      setPromptProposalActionId(null)
    }
  }, [addSystemMessage, promptProposalActionId])

  const rollbackPromptPackVersion = useCallback(async (versionId: string) => {
    const buildId = buildStateRef.current?.id
    if (!buildId || !versionId || promptProposalActionId) return

    setPromptProposalActionId(`prompt-pack-rollback:${versionId}`)
    try {
      const response = await apiService.rollbackBuildPromptPackVersion(buildId, versionId)
      setBuildState(prev => {
        if (!prev) return prev
        return {
          ...prev,
          promptPackVersions: response.prompt_pack_version
            ? upsertPromptPackVersion(prev.promptPackVersions, response.prompt_pack_version)
            : prev.promptPackVersions,
          promptPackActivationEvents: response.prompt_pack_activation_event
            ? upsertPromptPackActivationEvent(prev.promptPackActivationEvents, response.prompt_pack_activation_event)
            : prev.promptPackActivationEvents,
        }
      })
      const nextStatus = normalizeBuildStatus(response.status)
      if (nextStatus) {
        setBuildState(prev => prev ? { ...prev, status: nextStatus as BuildState['status'] } : prev)
      }
      addSystemMessage(response.message || 'Prompt pack version rolled back in registry')
    } catch (error) {
      addSystemMessage(`Prompt pack rollback failed: ${error instanceof Error ? error.message : 'Unable to rollback prompt pack version'}`)
    } finally {
      setPromptProposalActionId(null)
    }
  }, [addSystemMessage, promptProposalActionId])

  const normalizeGeneratedFiles = useCallback((files: Array<any>) => {
    if (!Array.isArray(files)) return []
    return files
      .filter((file) => file && file.path && typeof file.content === 'string')
      .map((file) => ({
        path: file.path,
        content: file.content,
        language: file.language || 'text',
      }))
  }, [])

  const normalizeArtifactManifestFiles = useCallback((payload: any) => {
    const manifestFiles = payload?.manifest?.files
    return normalizeGeneratedFiles(Array.isArray(manifestFiles) ? manifestFiles : [])
  }, [normalizeGeneratedFiles])

  const mergeGeneratedFiles = useCallback((incoming: Array<{ path: string; content: string; language: string }>) => {
    if (!incoming || incoming.length === 0) return
    setGeneratedFiles(prev => {
      const map = new Map(prev.map(f => [f.path, f]))
      for (const file of incoming) {
        map.set(file.path, file)
      }
      return Array.from(map.values())
    })
  }, [])

  const resolveGeneratedFiles = useCallback(async (buildIdOverride?: string) => {
    if (generatedFilesRef.current.length > 0) {
      return generatedFilesRef.current
    }

    const buildId = buildIdOverride || buildState?.id
    if (!buildId) return []

    try {
      const artifacts = await apiService.getBuildArtifacts(buildId)
      const normalized = normalizeArtifactManifestFiles(artifacts)
      if (normalized.length > 0) {
        mergeGeneratedFiles(normalized)
        setBuildState(prev => prev && prev.id === buildId
          ? { ...prev, artifactRevision: artifacts.revision || prev.artifactRevision }
          : prev
        )
        return normalized
      }
    } catch (error) {
      // Ignore and fall back to legacy build files/completed build fetch
    }

    try {
      const buildFiles = await apiService.getBuildFiles(buildId)
      const normalized = normalizeGeneratedFiles(buildFiles)
      if (normalized.length > 0) {
        mergeGeneratedFiles(normalized)
        return normalized
      }
    } catch (error) {
      // Ignore and fall back to completed build fetch
    }

    try {
      const completed = await apiService.getCompletedBuild(buildId)
      const normalized = normalizeGeneratedFiles(completed.files || [])
      if (normalized.length > 0) {
        mergeGeneratedFiles(normalized)
        return normalized
      }
    } catch (error) {
      // Ignore and return empty
    }

    return []
  }, [buildState?.id, mergeGeneratedFiles, normalizeArtifactManifestFiles, normalizeGeneratedFiles])

  useEffect(() => {
    if (buildWorkspaceView !== 'files') return
    if (!buildState?.id) return
    if (generatedFiles.length > 0) return
    void resolveGeneratedFiles(buildState.id)
  }, [buildState?.id, buildWorkspaceView, generatedFiles.length, resolveGeneratedFiles])

  const deriveProjectName = useCallback((source: string) => {
    const base = source || 'Generated App'
    return base
      .slice(0, 60)
      .replace(/[^a-zA-Z0-9\s-]/g, '')
      .trim() || 'Generated App'
  }, [])

  const ensureProjectCreated = useCallback(async (options?: {
    files?: Array<{ path: string; content: string; language: string }>
    projectName?: string
    description?: string
    forceNew?: boolean
    buildId?: string
  }) => {
    if (!options?.forceNew && createdProjectId && currentProject?.id === createdProjectId) {
      return currentProject
    }

    if (!options?.forceNew && createdProjectId && currentProject?.id !== createdProjectId) {
      try {
        const existingProject = await apiService.getProject(createdProjectId)
        setCurrentProject(existingProject)
        addSystemMessage(`Opened existing project "${existingProject.name}"`)
        return existingProject
      } catch {
        addSystemMessage('Existing project for this build was not found. Recreating from build files...')
      }
    }

    setIsCreatingProject(true)
    try {
      const projectNameSource = options?.projectName || appDescription || buildState?.description || 'Generated App'
      const projectName = deriveProjectName(projectNameSource)
      const projectDescription = options?.description || appDescription || buildState?.description || ''
      const buildIdForApply = options?.buildId || buildState?.id

      if (buildIdForApply) {
        try {
          const applyResponse = await apiService.applyBuildArtifacts(buildIdForApply, {
            project_id: !options?.forceNew && createdProjectId ? createdProjectId : undefined,
            project_name: projectName,
            replace_missing: true,
          })
          const appliedProject = await apiService.getProject(applyResponse.project_id)
          setCreatedProjectId(appliedProject.id)
          setCurrentProject(appliedProject)
          addSystemMessage(
            `Project "${appliedProject.name}" ${applyResponse.created_project ? 'created' : 'updated'} from canonical build artifacts (${applyResponse.applied_files ?? 0} files)`
          )
          return appliedProject
        } catch (applyError: any) {
          const errorMsg = applyError?.response?.data?.error || applyError?.message || 'Unknown error'
          addSystemMessage(`Failed to apply build artifacts: ${errorMsg}`)
          throw new Error(`Build artifact apply failed: ${errorMsg}`)
        }
      }

      const files = options?.files && options.files.length > 0
        ? options.files
        : await resolveGeneratedFiles(options?.buildId)

      if (files.length === 0) {
        throw new Error('No files available to create project from')
      }

      const extensions = files.map(f => f.path.split('.').pop()?.toLowerCase() || '')
      let language = 'javascript'
      if (extensions.some(e => ['tsx', 'ts'].includes(e))) language = 'typescript'
      else if (extensions.some(e => ['py'].includes(e))) language = 'python'
      else if (extensions.some(e => ['go'].includes(e))) language = 'go'
      else if (extensions.some(e => ['rs'].includes(e))) language = 'rust'

      const project = await createProject({
        name: projectName,
        description: projectDescription,
        language,
        is_public: false,
      })

      const filesToSave = files.filter(f => f.path && f.content)
      let savedCount = 0

      for (const file of filesToSave) {
        try {
          await apiService.createFile(project.id, {
            path: file.path,
            name: file.path.split('/').pop() || file.path,
            type: 'file',
            content: file.content,
          })
          savedCount++
        } catch (err) {
          console.error(`Failed to save file ${file.path}:`, err)
        }
      }

      setCreatedProjectId(project.id)
      setCurrentProject(project)
      addSystemMessage(`Project "${projectName}" created with ${savedCount}/${filesToSave.length} files!`)
      return project
    } finally {
      setIsCreatingProject(false)
    }
  }, [
    addSystemMessage,
    appDescription,
    buildState?.description,
    buildState?.id,
    createProject,
    currentProject,
    createdProjectId,
    deriveProjectName,
    resolveGeneratedFiles,
    setCurrentProject,
  ])

  const preparePreview = useCallback(async (auto: boolean) => {
    setIsPreparingPreview(true)
    try {
      const project = await ensureProjectCreated()
      if (auto) {
        addSystemMessage('Preview workspace is ready.')
      }
      return project
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : 'Failed to prepare preview'
      addSystemMessage(`Preview error: ${message}`)
      return null
    } finally {
      setIsPreparingPreview(false)
    }
  }, [addSystemMessage, ensureProjectCreated])

  const openPreviewWorkspace = useCallback(async () => {
    const project = await preparePreview(false)
    if (!project) return
    onNavigateToIDE?.({ target: 'preview', projectId: project.id })
  }, [onNavigateToIDE, preparePreview])

  const handleDownloadBuild = async () => {
    try {
      if (createdProjectId && currentProject?.id === createdProjectId) {
        await apiService.exportProject(createdProjectId, currentProject.name)
        return
      }
      if (buildState?.id) {
        const buildId = buildState.id
        const buildDescription = buildState.description
        const blob = await apiService.downloadBuildAsZip(buildId)
        const url = window.URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = url
        link.download = `${deriveProjectName(appDescription || buildDescription || 'apex-build')}-${buildId.slice(0, 8)}.zip`
        document.body.appendChild(link)
        link.click()
        document.body.removeChild(link)
        window.URL.revokeObjectURL(url)
        return
      }
      addSystemMessage('No build available to download')
    } catch (error) {
      addSystemMessage('Download failed. Please try again.')
    }
  }

  const hydrateBuildContext = useCallback(async (
    buildId: string,
    options?: {
      reconnectLive?: boolean
      notify?: boolean
      fallbackDetail?: CompletedBuildDetail
      payload?: any
    }
  ) => {
    let payload: any
    let loadPlatformIssue: BuildPlatformIssueContext | undefined
    if (options?.payload) {
      payload = options.payload
    } else {
      try {
        payload = await apiService.getBuildDetails(buildId)
      } catch (error) {
        loadPlatformIssue = extractPlatformIssue(error)
        const fallback = options?.fallbackDetail || await apiService.getCompletedBuild(buildId)
        payload = {
          id: fallback.build_id,
          build_id: fallback.build_id,
          project_id: fallback.project_id,
          status: fallback.status,
          mode: fallback.mode,
          description: fallback.description,
          progress: fallback.progress,
          agents: [],
          tasks: [],
          checkpoints: [],
          files: fallback.files || [],
          power_mode: fallback.power_mode,
          live: false,
        }
      }
    }

    payload = reconcileBuildPayloadWithCompletedDetail(payload, options?.fallbackDetail)

    const status = (normalizeBuildStatus(payload.status) || 'pending') as BuildState['status']
    const files = normalizeGeneratedFiles(payload.files || [])
    const interaction = normalizeInteraction(payload.interaction, payload.messages)
    const serverThoughts = restoreServerTelemetry(payload.activity_timeline)

    const agents: Agent[] = Array.isArray(payload.agents)
      ? payload.agents.map((agent: any, index: number) => ({
        id: String(agent.id || `${buildId}-agent-${index}`),
        role: String(agent.role || 'agent'),
        provider: String(agent.provider || 'unknown'),
        model: agent.model ? String(agent.model) : undefined,
        status: (agent.status === 'working' || agent.status === 'completed' || agent.status === 'error') ? agent.status : 'idle',
        progress: typeof agent.progress === 'number' ? clampPercent(agent.progress) : 0,
        currentTask: agent.current_task || agent.currentTask
          ? {
            type: String((agent.current_task || agent.currentTask).type || ''),
            description: String((agent.current_task || agent.currentTask).description || ''),
          }
          : undefined,
      }))
      : []

    const tasks: Task[] = Array.isArray(payload.tasks)
      ? payload.tasks.map((task: any, index: number) => ({
        id: String(task.id || `${buildId}-task-${index}`),
        type: String(task.type || 'task'),
        description: String(task.description || ''),
        status: (task.status === 'pending' || task.status === 'in_progress' || task.status === 'completed' || task.status === 'failed' || task.status === 'cancelled')
          ? task.status
          : 'pending',
        assignedTo: task.assigned_to || task.assignedTo,
        output: task.output,
      }))
      : []
    const guarantee = extractLatestGuaranteeStateFromTasks(tasks)

    const checkpoints: Checkpoint[] = Array.isArray(payload.checkpoints)
      ? payload.checkpoints.map((checkpoint: any, index: number) => ({
        id: String(checkpoint.id || `${buildId}-checkpoint-${index}`),
        number: typeof checkpoint.number === 'number' ? checkpoint.number : index + 1,
        name: String(checkpoint.name || `Checkpoint ${index + 1}`),
        description: String(checkpoint.description || ''),
        progress: typeof checkpoint.progress === 'number' ? clampPercent(checkpoint.progress) : 0,
        restorable: checkpoint.restorable !== false,
        createdAt: String(checkpoint.created_at || checkpoint.createdAt || new Date().toISOString()),
      }))
      : []

    setAppDescription(String(payload.description || appDescription || ''))
    setGeneratedFiles(files)
    setChatMessages(normalizeConversationMessages(interaction?.messages || payload.messages))
    setAiThoughts(serverThoughts.length > 0 ? serverThoughts : restorePersistedTelemetry(buildId))
    setProposedEdits([])
    setShowDiffReview(true)
    setCreatedProjectId(typeof payload.project_id === 'number' ? payload.project_id : null)
    setIsPreparingPreview(false)
    previewPreparedRef.current = false
    wsReconnectAttempts.current = 0
    const liveSessionAvailable = !isTerminalBuildStatus(status) && payload.live !== false

    setBuildState({
      id: String(payload.id || payload.build_id || buildId),
      status,
      progress: typeof payload.progress === 'number' ? clampPercent(payload.progress) : 0,
      agents,
      tasks,
      checkpoints,
      description: String(payload.description || appDescription || ''),
      powerMode: payload.power_mode || payload.powerMode || powerMode,
      providerModelOverrides: normalizeProviderModelOverrides(payload.provider_model_overrides),
      currentPhase: payload.phase || payload.current_phase || payload.currentPhase || undefined,
      qualityGateRequired: typeof payload.quality_gate_required === 'boolean' ? payload.quality_gate_required : true,
      qualityGateStatus: typeof payload.quality_gate_status === 'string'
        ? payload.quality_gate_status
        : typeof payload.quality_gate_passed === 'boolean'
          ? (payload.quality_gate_passed ? 'passed' : 'failed')
          : payload.quality_gate_active === true
            ? 'running'
        : status === 'completed'
          ? 'passed'
          : status === 'failed' || status === 'cancelled'
            ? 'failed'
            : (status === 'testing' || status === 'reviewing')
              ? 'running'
              : 'pending',
      qualityGateStage: payload.quality_gate_stage || undefined,
      availableProviders: Array.isArray(payload.available_providers) ? payload.available_providers : undefined,
      capabilityState: payload.capability_state,
      policyState: payload.policy_state,
      blockers: Array.isArray(payload.blockers) ? payload.blockers : [],
      approvals: Array.isArray(payload.approvals) ? payload.approvals : [],
      intentBrief: payload.intent_brief,
      buildContract: payload.build_contract,
      workOrders: Array.isArray(payload.work_orders) ? payload.work_orders : [],
      patchBundles: Array.isArray(payload.patch_bundles) ? payload.patch_bundles : [],
      verificationReports: Array.isArray(payload.verification_reports) ? payload.verification_reports : [],
      promotionDecision: payload.promotion_decision,
      providerScorecards: Array.isArray(payload.provider_scorecards) ? payload.provider_scorecards : [],
      failureFingerprints: Array.isArray(payload.failure_fingerprints) ? payload.failure_fingerprints : [],
      historicalLearning: payload.historical_learning,
      truthBySurface: payload.truth_by_surface || payload.promotion_decision?.truth_by_surface || payload.build_contract?.truth_by_surface,
      errorMessage: extractBuildFailureReason(payload),
      platformIssue: extractPlatformIssue(payload) || loadPlatformIssue,
      guarantee,
      websocketUrl: typeof payload.websocket_url === 'string' ? payload.websocket_url : undefined,
      liveSession: liveSessionAvailable,
      artifactRevision: typeof payload.artifact_revision === 'string' ? payload.artifact_revision : undefined,
      interaction,
    })
    persistLastWorkflowBuildId(buildId)

    const shouldReconnectLive = liveSessionAvailable && options?.reconnectLive !== false
    const active = isActiveBuildStatus(status)

    if (active) {
      persistActiveBuildId(buildId)
      if (shouldReconnectLive) {
        setIsBuilding(true)
        connectWebSocket(buildId)
        if (options?.notify) {
          addSystemMessage(`Resumed build ${buildId.slice(0, 8)} from live session`)
        }
      } else {
        setIsBuilding(false)
        if (options?.notify) {
          addSystemMessage('Restored latest build snapshot. Live session is no longer active.')
        }
      }
    } else {
      setIsBuilding(false)
      clearActiveBuildId()
      if (options?.notify) {
        addSystemMessage(`Opened saved build (${status.replace('_', ' ')})`)
      }
    }

    if (files.length === 0) {
      void resolveGeneratedFiles(buildId)
    }
    if (status === 'awaiting_review') {
      void loadProposedEdits(buildId)
    }
  }, [
    addSystemMessage,
    appDescription,
    clearActiveBuildId,
    connectWebSocket,
    loadProposedEdits,
    normalizeGeneratedFiles,
    normalizeConversationMessages,
    normalizeInteraction,
    persistActiveBuildId,
    persistLastWorkflowBuildId,
    powerMode,
    restorePersistedTelemetry,
    restoreServerTelemetry,
    resolveGeneratedFiles,
  ])

  const reconcileActiveBuildTerminalState = useCallback(async (buildId: string): Promise<boolean> => {
    try {
      const detail = await apiService.getBuildDetails(buildId)
      const status = normalizeBuildStatus(detail.status)
      if (status && isTerminalBuildStatus(status)) {
        await hydrateBuildContext(buildId, {
          reconnectLive: false,
          notify: false,
          payload: detail,
        })
        return true
      }
    } catch {
      try {
        const completed = await apiService.getCompletedBuild(buildId)
        const status = normalizeBuildStatus(String(completed.status || ''))
        if (status && isTerminalBuildStatus(status)) {
          await hydrateBuildContext(buildId, {
            reconnectLive: false,
            notify: false,
            payload: completed,
            fallbackDetail: completed,
          })
          return true
        }
      } catch {
        return false
      }
    }

    return false
  }, [hydrateBuildContext])


  useEffect(() => {
    if (!buildState?.id || !isBuildActive) {
      return
    }

    let cancelled = false
    const activeBuildId = buildState.id
    const reconcile = async () => {
      if (cancelled) {
        return
      }

      const healed = await reconcileActiveBuildTerminalState(activeBuildId)
      if (healed || cancelled) {
        return
      }
    }

    if (buildState.liveSession === false) {
      void reconcile()
    }

    const intervalId = window.setInterval(() => {
      void reconcile()
    }, 2000)

    return () => {
      cancelled = true
      window.clearInterval(intervalId)
    }
  }, [buildState?.id, buildState?.liveSession, isBuildActive, reconcileActiveBuildTerminalState])

  const handleUpgradeCheckout = useCallback(async () => {
    const requiredPlan = upgradePrompt?.requiredPlan || 'builder'
    const normalizedRequiredPlan = requiredPlan.trim().toLowerCase()
    const resumeBuildId = upgradePrompt?.buildId || buildStateRef.current?.id

    setUpgradeCheckoutPending(true)
    try {
      const plans = await apiService.getPlans()
      const availablePlans = plans.data?.plans || []
      const targetPlan = availablePlans.find((plan) => String(plan.type || '').trim().toLowerCase() === normalizedRequiredPlan)

      if (!targetPlan?.monthly_price_id) {
        throw new Error(`Stripe is not configured for the ${requiredPlan} plan in this environment.`)
      }

      const result = await apiService.createCheckoutSession({
        price_id: targetPlan.monthly_price_id,
        success_url: buildUpgradeReturnUrl('success', resumeBuildId),
        cancel_url: buildUpgradeReturnUrl('canceled', resumeBuildId),
      })

      if (!result.success || !result.data?.checkout_url) {
        throw new Error(result.error || 'Failed to start checkout. Please try again.')
      }

      window.location.href = result.data.checkout_url
    } catch (error: unknown) {
      const message = (error as any)?.response?.data?.error || (error instanceof Error ? error.message : 'Failed to start checkout')
      addSystemMessage(`Upgrade checkout failed: ${message}`)
      addNotification({
        type: 'error',
        title: 'Checkout Failed',
        message,
      })
    } finally {
      setUpgradeCheckoutPending(false)
    }
  }, [addNotification, addSystemMessage, buildUpgradeReturnUrl, upgradePrompt])

  useEffect(() => {
    if (typeof window === 'undefined' || !user?.id) return

    const url = new URL(window.location.href)
    const upgradeState = url.searchParams.get('upgrade')
    const resumeBuildId = url.searchParams.get('resume_build')
    if (!upgradeState && !resumeBuildId) {
      return
    }

    const clearUpgradeParams = () => {
      const nextUrl = new URL(window.location.href)
      nextUrl.searchParams.delete('upgrade')
      nextUrl.searchParams.delete('resume_build')
      window.history.replaceState({}, '', `${nextUrl.pathname}${nextUrl.search}${nextUrl.hash}`)
    }

    let cancelled = false
    const restoreBuildAfterUpgrade = async () => {
      try {
        if (resumeBuildId) {
          await hydrateBuildContext(resumeBuildId, {
            reconnectLive: true,
            notify: false,
          })
        }
        if (cancelled) return

        if (upgradeState === 'success') {
          dismissUpgradePrompt()
          addSystemMessage('Upgrade confirmed. Resuming the same app with backend/runtime work unlocked.')
          addNotification({
            type: 'success',
            title: 'Upgrade Confirmed',
            message: 'Resumed the same app so backend/runtime work can continue.',
          })
        } else if (upgradeState === 'canceled') {
          addSystemMessage('Upgrade canceled. The frontend preview remains available, and backend/runtime work stays gated on the free plan.')
          addNotification({
            type: 'info',
            title: 'Upgrade Canceled',
            message: 'Your frontend preview is still available. Upgrade later to continue backend/runtime work on this app.',
          })
        }
      } catch {
        if (!cancelled) {
          addSystemMessage('Returned from checkout, but the previous build could not be restored automatically. You can reopen it from Recent Builds.')
        }
      } finally {
        if (!cancelled) {
          clearUpgradeParams()
        }
      }
    }

    void restoreBuildAfterUpgrade()
    return () => {
      cancelled = true
    }
  }, [addNotification, addSystemMessage, dismissUpgradePrompt, hydrateBuildContext, user?.id])

  const openBuildFilesInIDE = useCallback(async (buildId: string, detail?: CompletedBuildDetail) => {
    const completed = detail || await apiService.getCompletedBuild(buildId)
    persistLastWorkflowBuildId(buildId)

    if (completed.project_id) {
      try {
        const existingProject = await apiService.getProject(completed.project_id)
        setCreatedProjectId(existingProject.id)
        setCurrentProject(existingProject)
        addSystemMessage(`Opened project "${existingProject.name}" from recent build`)
        onNavigateToIDE?.({ target: 'editor', projectId: existingProject.id })
        return
      } catch {
        addSystemMessage('Stored project not found, restoring from build artifacts...')
      }
    }

    let files = normalizeGeneratedFiles(completed.files || [])
    if (files.length === 0) {
      files = await resolveGeneratedFiles(buildId)
    }

    if (files.length === 0) {
      addSystemMessage('No files found for that build yet. Opening the IDE project browser instead.')
      onNavigateToIDE?.({ target: 'editor' })
      return
    }

    setGeneratedFiles(files)
    const project = await ensureProjectCreated({
      files,
      buildId,
      projectName: completed.project_name || deriveProjectName(completed.description || 'Completed Build'),
      description: completed.description || '',
      forceNew: true,
    })
    onNavigateToIDE?.({ target: 'editor', projectId: project.id })
  }, [
    addSystemMessage,
    deriveProjectName,
    ensureProjectCreated,
    normalizeGeneratedFiles,
    onNavigateToIDE,
    persistLastWorkflowBuildId,
    resolveGeneratedFiles,
    setCurrentProject,
  ])

  const openCompletedBuild = async (buildId: string, action: 'resume' | 'open_files' = 'resume') => {
    try {
      const completed = await apiService.getCompletedBuild(buildId)
      if (action === 'open_files') {
        await openBuildFilesInIDE(buildId, completed)
        return
      }
      await hydrateBuildContext(buildId, {
        reconnectLive: !isTerminalBuildStatus(String(completed.status || '')),
        notify: true,
        fallbackDetail: completed,
      })
    } catch (error) {
      const platformIssue = extractPlatformIssue(error)
      addSystemMessage(platformIssue?.summary || 'Failed to open build. Please try again.')
    }
  }

  useEffect(() => {
    if (!buildState?.id) return
    if (isActiveBuildStatus(buildState.status)) {
      persistActiveBuildId(buildState.id)
    } else {
      clearActiveBuildId()
    }
  }, [buildState?.id, buildState?.status, clearActiveBuildId, persistActiveBuildId])

  // Start build
  const startBuild = async () => {
    if (!appDescription.trim()) return
    if (appDescription.length > promptMaxLength) {
      addSystemMessage(
        buildMode === 'fast'
          ? `Fast Build prompts are capped at ${FAST_BUILD_PROMPT_MAX_LENGTH.toLocaleString()} characters. Switch to Full Build with Balanced for ${BALANCED_FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()} or Max Power for ${FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()}.`
          : powerMode === 'max'
            ? `Full Build Max Power prompts are capped at ${FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()} characters.`
            : powerMode === 'balanced'
              ? `Full Build Balanced prompts are capped at ${BALANCED_FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()} characters. Switch AI Power to Max for up to ${FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()}.`
              : `Full Build prompts stay capped at ${FAST_BUILD_PROMPT_MAX_LENGTH.toLocaleString()} characters on Fast power. Switch AI Power to Balanced for ${BALANCED_FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()} or Max for ${FULL_BUILD_PROMPT_MAX_LENGTH.toLocaleString()}.`
      )
      return
    }

    setIsBuilding(true)
    setGeneratedFiles([])
    setAiThoughts([])
    setChatMessages([])
    setProposedEdits([])
    setShowDiffReview(true)
    setIsPreparingPreview(false)
    setCreatedProjectId(null)
    wsReconnectAttempts.current = 0
    previewPreparedRef.current = false

    addSystemMessage(`Starting ${buildMode} build for: "${appDescription}"`)
    addSystemMessage(`Tech stack: ${buildTechStackSummary()}`)
    addSystemMessage(`AI Power: ${powerMode === 'max' ? 'MAX POWER' : powerMode === 'balanced' ? 'Balanced' : 'Fast'} (${getPowerModeModelSummary(powerMode)})`)

    try {
      // Preflight: verify providers are available before starting build.
      // Always use the fresh preflight result for provider_mode to avoid stale state
      // (e.g. user added/removed BYOK keys in Settings since this component mounted).
      let freshProviderMode: 'platform' | 'byok' | undefined
      let preflightCapabilityState: BuildCapabilityState | undefined
      let preflightPolicyState: BuildPolicyState | undefined
      const techStackOverride = buildTechStackOverride()
      try {
        const preflight = await apiService.buildPreflight({
          description: appDescription,
          prompt: appDescription,
          require_preview_ready: true,
          tech_stack: techStackOverride || undefined,
        })
        if (!preflight.ready) {
          const errorMsg = preflight.suggestion || preflight.error || 'AI providers unavailable'
          addSystemMessage(`Preflight failed: ${errorMsg}`)
          setIsBuilding(false)
          return
        }
        // Update state with fresh values
        const freshHasBYOK = !!preflight.has_byok
        setHasBYOK(freshHasBYOK)
        freshProviderMode = freshHasBYOK ? 'byok' : 'platform'
        if (preflight.provider_statuses) {
          setProviderStatuses(preflight.provider_statuses)
        }
        preflightCapabilityState = preflight.capability_detector
        preflightPolicyState = preflight.policy
      } catch (preflightErr: any) {
        const errData = preflightErr?.response?.data
        if (errData?.error_code) {
          addSystemMessage(`Cannot start build: ${errData.error || errData.suggestion || 'Provider check failed'}`)
          setIsBuilding(false)
          return
        }
        // Non-fatal: preflight endpoint may not exist on older backends.
        // Leave freshProviderMode undefined so backend uses its own detection.
        console.warn('Preflight check failed (non-fatal):', preflightErr)
      }

      const response = await apiService.startBuild({
        description: appDescription,
        prompt: appDescription,
        mode: buildMode,
        power_mode: powerMode,
        provider_mode: freshProviderMode,
        require_preview_ready: true,
        tech_stack: techStackOverride || undefined,
        diff_mode: false,
        role_assignments: roleConfigMode === 'manual' ? roleAssignments : undefined,
        provider_model_overrides: serializeProviderModelOverrides(providerModelOverrides),
        wireframe_image: wireframeImage || undefined,
      })

      if (!response || !response.build_id) {
        throw new Error('Invalid response from build API - no build_id returned')
      }

      const buildId = response.build_id
      persistActiveBuildId(buildId)
      persistLastWorkflowBuildId(buildId)
      setWireframeImage('')

      setBuildState({
        id: buildId,
        status: 'planning',
        progress: 5,
        agents: [],
        tasks: [],
        checkpoints: [],
        description: appDescription,
        powerMode,
        providerModelOverrides: normalizeProviderModelOverrides(serializeProviderModelOverrides(providerModelOverrides)),
        currentPhase: 'Planning',
        qualityGateRequired: true,
        qualityGateStatus: 'pending',
        qualityGateStage: '',
        capabilityState: preflightCapabilityState,
        policyState: preflightPolicyState,
        blockers: [],
        approvals: [],
        websocketUrl: typeof response.websocket_url === 'string' ? response.websocket_url : undefined,
        liveSession: true,
      })

      connectWebSocket(buildId, response.websocket_url)
      addSystemMessage(`Build started! Build ID: ${buildId}`)
      if (preflightPolicyState?.classification === 'upgrade_required' && preflightPolicyState.static_frontend_only) {
        const suggestion = `The frontend preview is building now. Upgrade to ${(preflightPolicyState.required_plan || 'builder').replace(/\b\w/g, (match) => match.toUpperCase())} or higher to unlock ${preflightPolicyState.upgrade_reason || 'backend/runtime implementation'} on this same app.`
        addSystemMessage(suggestion)
        openUpgradePrompt({
          source: 'start',
          buildId,
          requiredPlan: preflightPolicyState.required_plan,
          reason: preflightPolicyState.upgrade_reason,
          suggestion,
        })
      }

    } catch (error: unknown) {
      console.error('Build start failed:', error)

      let errorMsg = 'Unknown error occurred'

      // Type-safe error handling with proper narrowing
      const isAxiosError = (err: unknown): err is { response?: { data?: { error?: string; details?: string; message?: string }; status?: number }; message?: string } => {
        return typeof err === 'object' && err !== null
      }

      if (isAxiosError(error)) {
        if (error.response?.data?.error) {
          errorMsg = error.response.data.error
        } else if (error.response?.data?.details) {
          errorMsg = error.response.data.details
        } else if (error.response?.data?.message) {
          errorMsg = error.response.data.message
        } else if (error.message) {
          errorMsg = error.message
        }

        const errorCode = (error.response?.data as any)?.error_code

        if (errorCode === 'BACKEND_SUBSCRIPTION_REQUIRED') {
          const suggestion = (error.response?.data as any)?.suggestion || 'Upgrade to Builder or higher to unlock backend and full-stack app generation.'
          addSystemMessage(`Build blocked: ${suggestion}`)
          openUpgradePrompt({
            source: 'start',
            requiredPlan: (error.response?.data as any)?.required_plan,
            reason: (error.response?.data as any)?.blocked_reason,
            suggestion,
          })
          setIsBuilding(false)
          return
        }

        if (error.response?.status === 402 || errorCode === 'INSUFFICIENT_CREDITS') {
          setBuyCreditsReason('Your credit balance has run out. Purchase credits to continue building.')
          setShowBuyCredits(true)
          setIsBuilding(false)
          return
        } else if (error.response?.status === 401) {
          errorMsg = 'Authentication required. Please log in to start a build.'
        } else if (error.response?.status === 403) {
          errorMsg = 'You do not have permission to start builds.'
        } else if (error.response?.status === 429) {
          errorMsg = 'Too many requests. Please wait a moment before trying again.'
        } else if (error.response?.status && error.response.status >= 500) {
          errorMsg = 'Server error. Please try again later.'
        } else if (!error.response && error.message?.includes('Network')) {
          errorMsg = 'Network error. Please check your connection and try again.'
        }
      } else if (error instanceof Error) {
        errorMsg = error.message
      }

      addSystemMessage(`Error: ${errorMsg}`)
      setIsBuilding(false)
      clearActiveBuildId()
      setBuildState(null)
    }
  }

  const handlePauseBuild = async () => {
    if (!buildState?.id || buildActionPending) return
    setBuildActionPending('pause')
    try {
      const response = await apiService.pauseBuild(buildState.id, 'Paused from builder UI')
      syncInteractionState(response.interaction)
      addSystemMessage('Build paused')
    } catch (error) {
      addSystemMessage('Failed to pause build')
    } finally {
      setBuildActionPending(null)
    }
  }

  const handleResumeBuild = async () => {
    if (!buildState?.id || buildActionPending) return
    setBuildActionPending('resume')
    try {
      const response = await apiService.resumeBuild(buildState.id, 'Resumed from builder UI')
      syncInteractionState(response.interaction)
      addSystemMessage('Build resumed')
    } catch (error) {
      addSystemMessage('Failed to resume build')
    } finally {
      setBuildActionPending(null)
    }
  }

  const handleSelectProviderModel = async (provider: SupportedBuildProvider, model: string) => {
    const nextOverrides = normalizeProviderModelOverrides({
      ...activeProviderModelOverrides,
      [provider]: model,
    })
    setProviderModelOverrides(nextOverrides)

    if (!buildState?.id || !isBuildActive) {
      setBuildState((prev) => prev ? { ...prev, providerModelOverrides: nextOverrides } : prev)
      return
    }

    setProviderModelPendingProvider(provider)
    try {
      const response = await apiService.setBuildProviderModel(buildState.id, {
        provider,
        model,
      })
      const persistedOverrides = normalizeProviderModelOverrides(response.provider_model_overrides || nextOverrides)
      setProviderModelOverrides(persistedOverrides)
      setBuildState((prev) => prev ? {
        ...prev,
        providerModelOverrides: persistedOverrides,
        agents: Array.isArray(response.agents) ? response.agents : prev.agents,
      } : prev)
      if (canonicalizeModelId(model) === 'auto') {
        addSystemMessage(`${humanizeIdentifier(provider)} returned to Auto model selection`)
      } else {
        addSystemMessage(`${humanizeIdentifier(provider)} locked to ${getModelDisplayName(model, activePowerMode) || model}`)
      }
    } catch (error) {
      setProviderModelOverrides(normalizeProviderModelOverrides(buildState.providerModelOverrides))
      addSystemMessage(`Failed to update ${humanizeIdentifier(provider)} model selection`)
    } finally {
      setProviderModelPendingProvider(null)
    }
  }

  const handleResolvePermissionRequest = async (
    requestId: string,
    decision: 'allow' | 'deny',
    mode: 'once' | 'build'
  ) => {
    if (!buildState?.id) return
    setPermissionActionId(requestId)
    try {
      const response = await apiService.resolveBuildPermissionRequest(buildState.id, requestId, {
        decision,
        mode,
      })
      syncInteractionState(response.interaction)
    } catch (error) {
      addSystemMessage('Failed to update permission request')
    } finally {
      setPermissionActionId(null)
    }
  }

  const handleSetPermissionPreset = async (
    scope: string,
    target: string,
    decision: 'allow' | 'deny',
    mode: 'build' = 'build',
    reason?: string
  ) => {
    if (!buildState?.id) return
    const presetId = `${scope}:${target}:${decision}`
    setPermissionActionId(presetId)
    try {
      const response = await apiService.setBuildPermissionRule(buildState.id, {
        scope,
        target,
        decision,
        mode,
        reason,
      })
      syncInteractionState(response.interaction)
      addSystemMessage(`${decision === 'allow' ? 'Approved' : 'Denied'} ${target} for this build`)
    } catch (error) {
      addSystemMessage(`Failed to update ${target} permission`)
    } finally {
      setPermissionActionId(null)
    }
  }

  const setPendingConversationMessageStatus = useCallback((clientToken: string, status: ChatMessage['status']) => {
    setChatMessages(prev => prev.map(message =>
      message.clientToken === clientToken ? { ...message, status } : message
    ))
  }, [])

  const sendBuildConversationMessage = useCallback(async (options: {
    content?: string
    optimisticContent?: string
    command?: 'restart_failed'
    targetMode?: BuildMessageTargetMode
    targetAgentId?: string
    targetAgentRole?: string
  }) => {
    if (!buildStateRef.current?.id) return false

    const content = (options.content || '').trim()
    const optimisticContent = (options.optimisticContent || content).trim()
    if (!content && !options.command) return false

    const clientToken = `chat-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    if (optimisticContent) {
      setChatMessages(prev => [...prev, {
        id: clientToken,
        role: 'user',
        content: optimisticContent,
        timestamp: new Date(),
        kind: options.targetMode && options.targetMode !== 'lead' ? 'directive' : 'message',
        targetMode: options.targetMode || 'lead',
        targetAgentId: options.targetAgentId,
        targetAgentRole: options.targetAgentRole,
        clientToken,
        status: 'pending',
      }])
    }

    try {
      const response = await apiService.sendBuildMessage(buildStateRef.current.id, content, {
        clientToken,
        command: options.command,
        targetMode: options.targetMode || 'lead',
        targetAgentId: options.targetAgentId,
        targetAgentRole: options.targetAgentRole,
      })
      if (optimisticContent) {
        setPendingConversationMessageStatus(clientToken, 'sent')
      }
      syncInteractionState(response.interaction)
      const buildId = buildStateRef.current.id
      if (response.live === true && !hasUsableWebSocketConnection(buildId)) {
        const websocketUrl = buildStateRef.current.websocketUrl
        setBuildState(prev => prev && prev.id === buildId ? { ...prev, liveSession: true } : prev)
        connectWebSocket(buildId, websocketUrl)
      }
      return true
    } catch (error) {
      if (optimisticContent) {
        setPendingConversationMessageStatus(clientToken, 'failed')
      }
      const errorData = (error as any)?.response?.data
      const errorCode = errorData?.error_code
      if (errorCode === 'BACKEND_SUBSCRIPTION_REQUIRED') {
        const suggestion = errorData?.suggestion || 'Upgrade to Builder or higher to continue backend/runtime work on this app.'
        addSystemMessage(suggestion)
        openUpgradePrompt({
          source: 'message',
          buildId: buildStateRef.current?.id,
          requiredPlan: errorData?.required_plan,
          reason: errorData?.blocked_reason,
          suggestion,
        })
        return false
      }
      if ((error as any)?.response?.status === 402 || errorCode === 'INSUFFICIENT_CREDITS') {
        setBuyCreditsReason(errorData?.suggestion || 'Your credit balance has run out. Purchase credits to continue building.')
        setShowBuyCredits(true)
        return false
      }
      addSystemMessage('Message failed to send. Please try again.')
      return false
    }
  }, [addSystemMessage, connectWebSocket, hasUsableWebSocketConnection, openUpgradePrompt, setPendingConversationMessageStatus, syncInteractionState])

  const sendChatMessage = async () => {
    const content = chatInput.trim()
    if (!content) return

    setPlannerMessagePending(true)
    setChatInput('')
    const targetMode = plannerSendMode === 'all_agents' && isBuildActive ? 'all_agents' : 'lead'
    try {
      await sendBuildConversationMessage({
        content,
        targetMode,
      })
    } finally {
      setPlannerMessagePending(false)
    }
  }

  const openPlannerConsole = useCallback((mode: BuildMessageTargetMode = 'lead') => {
    setPlannerSendMode(mode)
    setShowChat(true)
    setBuildWorkspaceView('console')
  }, [])

  const sendDirectAgentMessage = async (agent: Agent) => {
    const content = agentMessageDrafts[agent.id]?.trim()
    if (!content || !isBuildActive) return

    setAgentMessagePendingId(agent.id)
    try {
      const sent = await sendBuildConversationMessage({
        content,
        targetMode: 'agent',
        targetAgentId: agent.id,
        targetAgentRole: agent.role,
      })
      if (sent) {
        setAgentMessageDrafts(prev => ({ ...prev, [agent.id]: '' }))
      }
    } finally {
      setAgentMessagePendingId((current) => current === agent.id ? null : current)
    }
  }

  const handleRestartFailedBuild = async () => {
    if (!buildState?.id || buildState.status !== 'failed' || buildActionPending !== null) return
    setBuildActionPending('restart')
    try {
      const sent = await sendBuildConversationMessage({
        command: 'restart_failed',
        content: DEFAULT_RESTART_FAILED_MESSAGE,
        optimisticContent: DEFAULT_RESTART_FAILED_MESSAGE,
        targetMode: 'lead',
      })
      if (sent) {
        setIsBuilding(true)
        persistActiveBuildId(buildState.id)
      }
    } finally {
      setBuildActionPending(null)
    }
  }

  // Create project and open in IDE
  const openInIDE = async () => {
    try {
      const project = await ensureProjectCreated()
      if (onNavigateToIDE) {
        onNavigateToIDE({ target: 'editor', projectId: project.id })
      }
    } catch (error: unknown) {
      console.error('Failed to create project:', error)
      const message = error instanceof Error ? error.message : 'Unknown error'
      addSystemMessage(`Failed to create project: ${message}. Opening IDE without a project instead.`)
      onNavigateToIDE?.({ target: 'editor' })
    }
  }

  // Helper functions
  const getAgentEmoji = (role: string) => {
    const emojis: Record<string, string> = {
      lead: '👨‍💼',
      planner: '📋',
      architect: '🏗️',
      frontend: '🎨',
      backend: '⚙️',
      database: '🗄️',
      testing: '🧪',
      reviewer: '🔍',
      solver: '🛠️',
    }
    return emojis[role] || '🤖'
  }

  const formatRole = (role: string) => {
    return role.charAt(0).toUpperCase() + role.slice(1)
  }

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'working': return <Circle className="w-4 h-4 fill-red-400 text-red-400 animate-pulse" />
      case 'completed': return <CheckCircle2 className="w-4 h-4 text-green-400" />
      case 'error': return <AlertCircle className="w-4 h-4 text-orange-400" />
      default: return <Circle className="w-4 h-4 text-gray-500" />
    }
  }

  const handleReplitImport = async () => {
    if (!replitUrl.trim()) return
    setIsImporting(true)
    try {
      alert('Replit import initialized. Our agents are analyzing the project...')
      setShowImportModal(false)
      setAppDescription(`Imported from Replit: ${replitUrl}`)
      startBuild()
    } catch (error) {
      console.error('Import failed:', error)
    } finally {
      setIsImporting(false)
    }
  }

  const handleGitHubImportSuccess = useCallback(async (projectId: number) => {
    try {
      const project = await apiService.getProject(projectId)
      setCurrentProject(project)
      setCreatedProjectId(project.id)
      addSystemMessage(`Imported "${project.name}" from GitHub`)
      onNavigateToIDE?.({ target: 'dashboard', projectId: project.id })
    } catch (error) {
      addSystemMessage('Import completed, but opening the project failed. Please open it from Projects.')
    } finally {
      setShowGitHubImport(false)
    }
  }, [addSystemMessage, onNavigateToIDE, setCurrentProject])

  const handleRollbackCheckpoint = async (checkpointId: string) => {
    if (!buildState?.id) return
    const checkpoint = buildState.checkpoints.find((entry) => entry.id === checkpointId)
    if (checkpoint?.restorable === false) {
      addSystemMessage('Historical checkpoints from a restored snapshot are view-only and cannot be rolled back.')
      return
    }
    if (!isActiveBuildStatus(buildState.status)) {
      addSystemMessage('Rollback is only available while a live build is still active.')
      return
    }
    setRollbackCheckpointId(checkpointId)
    try {
      await apiService.rollbackBuild(buildState.id, checkpointId)
      addSystemMessage(`Rolled back to checkpoint ${checkpointId}`)
    } catch (error: unknown) {
      const message =
        (error as any)?.response?.data?.details ||
        (error as any)?.response?.data?.error ||
        (error instanceof Error ? error.message : 'Rollback failed')
      addSystemMessage(`Rollback error: ${message}`)
    } finally {
      setRollbackCheckpointId(null)
    }
  }

  // ============================================================================
  // RENDER
  // ============================================================================

  return (
    <div ref={builderRootRef} className={cn("app-builder-root h-full min-h-0 bg-black text-white relative", buildState ? "overflow-hidden flex flex-col" : "overflow-y-auto overscroll-contain")}>
      {/* Spend toasts */}
      {spendToasts.length > 0 && (
        <div className="fixed bottom-20 right-4 z-50 flex flex-col gap-2 pointer-events-none">
          {spendToasts.map(t => (
            <SpendToast key={t.id} agentRole={t.agentRole} cost={t.cost} onDismiss={() => dismissSpendToast(t.id)} />
          ))}
        </div>
      )}
      {/* Buy Credits Modal */}
      {showBuyCredits && (
        <BuyCreditsModal
          reason={buyCreditsReason}
          onClose={() => setShowBuyCredits(false)}
        />
      )}
      {upgradePrompt && (
        <PlanUpgradeModal
          currentPlan={buildState?.policyState?.plan_type || user?.subscription_type}
          upgrade={upgradePrompt}
          loading={upgradeCheckoutPending}
          onClose={dismissUpgradePrompt}
          onUpgrade={handleUpgradeCheckout}
        />
      )}

      {/* Onboarding Tour - shows on first visit */}
      <OnboardingTour />

      {/* CSS Keyframe Animations */}
      <style>{`
        @keyframes gradient-shift {
          0%, 100% { background-position: 0% 50%; }
          50% { background-position: 100% 50%; }
        }
        @keyframes scan-line {
          0% { transform: translateY(-100%); }
          100% { transform: translateY(400%); }
        }
        @keyframes scan-horizontal {
          0% { transform: translateX(-100%); }
          100% { transform: translateX(400%); }
        }
        @keyframes shimmer {
          0% { transform: translateX(-100%); }
          100% { transform: translateX(100%); }
        }
        @keyframes float {
          0%, 100% { transform: translateY(0); }
          50% { transform: translateY(-10px); }
        }
        @keyframes fade-in {
          from { opacity: 0; }
          to { opacity: 1; }
        }
        @keyframes fade-in-up {
          from { opacity: 0; transform: translateY(20px); }
          to { opacity: 1; transform: translateY(0); }
        }
        @keyframes ripple-expand {
          0% { transform: scale(0); opacity: 1; }
          100% { transform: scale(10); opacity: 0; }
        }
        @keyframes sparkle {
          0%, 100% { opacity: 0; transform: scale(0); }
          50% { opacity: 1; transform: scale(1); }
        }
        @keyframes bounce-slow {
          0%, 100% { transform: translateY(0); }
          50% { transform: translateY(-8px); }
        }
        .animate-gradient-shift {
          animation: gradient-shift 3s linear infinite;
        }
      `}</style>

      {/* Animated background layers */}
      <div className="app-builder-background fixed inset-0 pointer-events-none">
        {/* Base gradient — dark, matches landing palette */}
        <div className="app-builder-bg-base absolute inset-0 bg-gradient-to-b from-[rgba(2,10,28,0.9)] via-[#020408] to-[#020408]" />

        {/* Floating particles */}
        <FloatingParticles />

        {/* Radial gradient accents — subtle cyan */}
        <div className="app-builder-accent app-builder-accent-a absolute top-0 left-1/4 w-[600px] h-[600px] bg-[rgba(47,168,255,0.05)] rounded-full blur-3xl" />
        <div className="app-builder-accent app-builder-accent-b absolute bottom-0 right-1/4 w-[500px] h-[500px] bg-[rgba(138,223,255,0.04)] rounded-full blur-3xl" />
        <div className="app-builder-accent app-builder-accent-c absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[900px] h-[900px] bg-[rgba(15,30,60,0.5)] rounded-full blur-3xl" />
      </div>

      {/* Main content */}
      <div className={buildState ? "relative z-10 flex-1 min-h-0 flex flex-col overflow-hidden" : "relative z-10 p-4 pb-8 sm:p-6 sm:pb-10 md:p-8 md:pb-12 lg:p-12 lg:pb-16"}>
        {/* Replit Import Modal */}
        {showImportModal && (
          <div className="fixed inset-0 z-[100] overflow-y-auto bg-black/95 p-4 backdrop-blur-md">
            <div className="flex min-h-full items-center justify-center">
              <Card variant="cyberpunk" glow="intense" className="w-full max-w-lg max-h-[calc(100vh-2rem)] overflow-y-auto border-2 border-[rgba(188,239,255,0.2)]" style={{ animation: 'fade-in-up 0.3s ease-out' }}>
                <CardHeader>
                  <CardTitle className="text-2xl flex items-center gap-3">
                    <Download className="w-7 h-7 text-red-500" />
                    Import from Replit
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-6">
                  <p className="text-gray-400 leading-relaxed">
                    Enter the URL of the Replit project you want to migrate to APEX-BUILD.
                    Our agents will analyze the source and reconstruct it with optimized performance.
                  </p>
                  <div className="space-y-2">
                    <label className="text-sm font-semibold text-gray-300">Replit URL</label>
                    <input
                      type="text"
                      value={replitUrl}
                      onChange={(e) => setReplitUrl(e.target.value)}
                      placeholder="https://replit.com/@username/project-name"
                      className="w-full bg-black border-2 border-gray-700 rounded-xl px-4 py-3 text-white focus:border-red-600 focus:ring-2 focus:ring-red-900/30 outline-none transition-all"
                    />
                  </div>
                  <div className="flex gap-4">
                    <Button
                      onClick={handleReplitImport}
                      disabled={isImporting || !replitUrl.includes('replit.com')}
                      className="flex-1 bg-red-600 hover:bg-red-500 font-semibold"
                    >
                      {isImporting ? 'Analyzing...' : 'Start Migration'}
                    </Button>
                    <Button
                      variant="ghost"
                      onClick={() => setShowImportModal(false)}
                      className="border-2 border-gray-700 hover:bg-gray-900"
                    >
                      Cancel
                    </Button>
                  </div>
                </CardContent>
              </Card>
            </div>
          </div>
        )}

        {/* Main Content */}
        {!buildState ? (
          <BuilderLaunchpad
            buildMode={buildMode}
            powerMode={powerMode}
            appDescription={appDescription}
            promptMaxLength={promptMaxLength}
            isBuilding={isBuilding}
            isRoleAssignmentValid={isRoleAssignmentValid}
            maxPowerPromptLimitEnabled={maxPowerPromptLimitEnabled}
            balancedPromptLimitEnabled={balancedPromptLimitEnabled}
            wireframeImage={wireframeImage}
            createdProjectId={createdProjectId}
            selectedStack={selectedStack}
            autoStackId={AUTO_STACK_ID}
            techStacks={techStacks}
            roleConfigMode={roleConfigMode}
            roleAssignments={roleAssignments}
            providerStatuses={providerStatuses}
            userPlan={user?.subscription_type}
            userId={user?.id ?? null}
            wireframeInputRef={wireframeInputRef}
            onSetBuildMode={setBuildMode}
            onSetPowerMode={setPowerMode}
            onSetAppDescription={setAppDescription}
            onSetWireframeImage={setWireframeImage}
            onClearWireframe={() => setWireframeImage('')}
            onToggleStack={toggleStack}
            onSetRoleConfigMode={setRoleConfigMode}
            onSetRoleAssignments={setRoleAssignments}
            onStartBuild={startBuild}
            onImportReplit={() => setShowImportModal(true)}
            onImportGitHub={() => setShowGitHubImport(true)}
            onOpenIDE={() => onNavigateToIDE?.({ target: 'editor', projectId: createdProjectId })}
            onOpenSettings={() => onNavigateToView?.('settings')}
            onOpenBilling={() => onNavigateToView?.('settings')}
            onBuildHistoryOpen={openCompletedBuild}
            buildTechStackSummary={buildTechStackSummary}
          />
        ) : (
          <BuildScreen
            buildState={buildState}
            providerPanels={providerPanels}
            aiThoughts={aiThoughts}
            chatMessages={chatMessages}
            generatedFiles={generatedFiles}
            proposedEdits={proposedEdits}
            isBuildActive={isBuildActive}
            buildPaused={buildPaused}
            pendingQuestion={pendingQuestion}
            pendingPermissionRequests={pendingPermissionRequests}
            pendingRevisionRequests={pendingRevisionRequests}
            buildActionPending={buildActionPending}
            hasBYOK={hasBYOK}
            phaseLabel={phaseLabel}
            visibleBlockers={visibleBlockers}
            platformReadinessNotice={platformReadinessNotice}
            buildFailureAttribution={buildFailureAttribution}
            showDiffReview={showDiffReview}
            userId={user?.id}
            isPreparingPreview={isPreparingPreview}
            isCreatingProject={isCreatingProject}
            isStartingOver={isStartingOver}
            createdProjectId={createdProjectId}
            permissionActionId={permissionActionId}
            rollbackCheckpointId={rollbackCheckpointId}
            patchBundleActionId={patchBundleActionId}
            promptProposalActionId={promptProposalActionId}
            chatInput={chatInput}
            setChatInput={setChatInput}
            plannerSendMode={plannerSendMode}
            setPlannerSendMode={setPlannerSendMode}
            plannerMessagePending={plannerMessagePending}
            providerModelOverrides={activeProviderModelOverrides}
            providerModelOptions={PROVIDER_MODEL_OPTIONS}
            providerModelPendingProvider={providerModelPendingProvider}
            agentMessageDrafts={agentMessageDrafts}
            agentMessagePendingId={agentMessagePendingId}
            onAgentMessageDraftChange={(agentId, value) => {
              setAgentMessageDrafts((prev) => ({ ...prev, [agentId]: value }))
            }}
            onSendDirectAgentMessage={(agentId) => {
              const agent = buildState?.agents.find((candidate) => candidate.id === agentId)
              if (agent) {
                void sendDirectAgentMessage(agent)
              }
            }}
            onSendChatMessage={sendChatMessage}
            onSelectProviderModel={handleSelectProviderModel}
            onPause={handlePauseBuild}
            onResume={handleResumeBuild}
            onRestart={handleRestartFailedBuild}
            onStartOver={() => { void handleStartOver() }}
            onPreviewWorkspace={openPreviewWorkspace}
            onOpenInIDE={openInIDE}
            onDownload={handleDownloadBuild}
            onRollbackCheckpoint={handleRollbackCheckpoint}
            onResolvePermission={handleResolvePermissionRequest}
            onApprovePatchBundle={(bundleId) => { void reviewPatchBundle(bundleId, 'approve') }}
            onRejectPatchBundle={(bundleId) => { void reviewPatchBundle(bundleId, 'reject') }}
            onReviewPromptProposal={(proposalId, decision) => { void reviewPromptProposal(proposalId, decision) }}
            onBenchmarkPromptProposal={(proposalId) => { void benchmarkPromptProposal(proposalId) }}
            onCreatePromptPackDraft={() => { void createPromptPackDraft() }}
            onRequestPromptPackActivation={(user?.is_admin || user?.is_super_admin) ? (draftId) => { void requestPromptPackActivation(draftId) } : undefined}
            onActivatePromptPackRequest={(user?.is_admin || user?.is_super_admin) ? (requestId) => { void activatePromptPackRequest(requestId) } : undefined}
            onRollbackPromptPackVersion={(user?.is_admin || user?.is_super_admin) ? (versionId) => { void rollbackPromptPackVersion(versionId) } : undefined}
            onSetShowDiffReview={setShowDiffReview}
            onLoadProposedEdits={loadProposedEdits}
            onOpenCompletedBuild={openCompletedBuild}
          />
        )}
      </div>

      {/* GitHub Import Modal */}
      {showGitHubImport && (
        <div className="fixed inset-0 z-[100] overflow-y-auto bg-black/90 backdrop-blur-sm p-4">
          <div className="flex min-h-full items-center justify-center">
            <GitHubImportWizard
              onClose={() => setShowGitHubImport(false)}
              onImported={handleGitHubImportSuccess}
            />
          </div>
        </div>
      )}
    </div>
  )
}

export default AppBuilder
