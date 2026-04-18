// Provider Status Bar — 5 color-coded AI provider boxes
// Shows ChatGPT, Gemini, Grok, Claude, and Local/Ollama (BYOK only)

import React from 'react'
import { cn } from '@/lib/utils'

type ProviderStatus = 'idle' | 'working' | 'thinking' | 'completed' | 'error' | 'unavailable'
type SupportedProvider = 'claude' | 'gpt4' | 'gemini' | 'grok'

interface PanelData {
  provider: SupportedProvider
  status: ProviderStatus
  statusLabel: string
  liveModelName: string
  available: boolean
}

interface ProviderStatusBarProps {
  providerPanels: PanelData[]
  hasBYOK: boolean
  isBuildActive: boolean
  selectedModels?: Partial<Record<SupportedProvider, string>>
  modelOptions?: Partial<Record<SupportedProvider, Array<{ id: string; name: string }>>>
  modelUpdatePendingProvider?: SupportedProvider | null
  onModelSelect?: (provider: SupportedProvider, model: string) => void
}

// Display order per user request: ChatGPT, Gemini, Grok, Claude, Local
const DISPLAY_ORDER: SupportedProvider[] = ['gpt4', 'gemini', 'grok', 'claude']

const PROVIDER_CONFIG: Record<SupportedProvider, {
  label: string
  tagline: string
  borderActive: string
  borderIdle: string
  bgActive: string
  textActive: string
  glowActive: string
  dotWorking: string
  dotThinking: string
  badgeActive: string
}> = {
  gpt4: {
    label: 'ChatGPT',
    tagline: 'OpenAI',
    borderActive: 'border-emerald-500/70',
    borderIdle: 'border-emerald-500/15',
    bgActive: 'bg-gradient-to-br from-emerald-950/60 via-black to-emerald-950/25',
    textActive: 'text-emerald-300',
    glowActive: 'shadow-[0_0_24px_rgba(16,185,129,0.18)]',
    dotWorking: 'bg-emerald-400',
    dotThinking: 'bg-yellow-400',
    badgeActive: 'bg-emerald-500/20 border-emerald-500/40 text-emerald-300',
  },
  gemini: {
    label: 'Gemini',
    tagline: 'Google',
    borderActive: 'border-sky-500/70',
    borderIdle: 'border-sky-500/15',
    bgActive: 'bg-gradient-to-br from-sky-950/60 via-black to-sky-950/25',
    textActive: 'text-sky-300',
    glowActive: 'shadow-[0_0_24px_rgba(56,189,248,0.18)]',
    dotWorking: 'bg-sky-400',
    dotThinking: 'bg-yellow-400',
    badgeActive: 'bg-sky-500/20 border-sky-500/40 text-sky-300',
  },
  grok: {
    label: 'Grok',
    tagline: 'xAI',
    borderActive: 'border-fuchsia-500/70',
    borderIdle: 'border-fuchsia-500/15',
    bgActive: 'bg-gradient-to-br from-fuchsia-950/60 via-black to-fuchsia-950/25',
    textActive: 'text-fuchsia-300',
    glowActive: 'shadow-[0_0_24px_rgba(217,70,239,0.18)]',
    dotWorking: 'bg-fuchsia-400',
    dotThinking: 'bg-yellow-400',
    badgeActive: 'bg-fuchsia-500/20 border-fuchsia-500/40 text-fuchsia-300',
  },
  claude: {
    label: 'Claude',
    tagline: 'Anthropic',
    borderActive: 'border-orange-500/70',
    borderIdle: 'border-orange-500/15',
    bgActive: 'bg-gradient-to-br from-orange-950/60 via-black to-orange-950/25',
    textActive: 'text-orange-300',
    glowActive: 'shadow-[0_0_24px_rgba(251,146,60,0.18)]',
    dotWorking: 'bg-orange-400',
    dotThinking: 'bg-yellow-400',
    badgeActive: 'bg-orange-500/20 border-orange-500/40 text-orange-300',
  },
}

export const ProviderStatusBar: React.FC<ProviderStatusBarProps> = ({
  providerPanels,
  hasBYOK,
  isBuildActive,
  selectedModels,
  modelOptions,
  modelUpdatePendingProvider,
  onModelSelect,
}) => {
  const panelMap = Object.fromEntries(
    providerPanels.map((p) => [p.provider, p])
  ) as Partial<Record<SupportedProvider, PanelData>>

  const getStatusLabel = (panel: PanelData | undefined, provider: SupportedProvider): string => {
    const status = panel?.status || 'idle'
    if (status === 'thinking') return 'THINKING'
    if (status === 'working') return 'WORKING'
    if (status === 'completed') return 'DONE'
    if (status === 'error') return 'ERROR'
    if (status === 'unavailable') return 'N/A'
    return isBuildActive ? 'STANDBY' : 'IDLE'
  }

  return (
    <div className="flex border-b border-gray-900 shrink-0 overflow-x-auto" style={{ minHeight: '108px' }}>
      {DISPLAY_ORDER.map((provider) => {
        const panel = panelMap[provider]
        const cfg = PROVIDER_CONFIG[provider]
        const status = panel?.status || 'idle'
        const isActive = status === 'working' || status === 'thinking'
        const isCompleted = status === 'completed'
        const isError = status === 'error'
        const isUnavailable = status === 'unavailable'

        const dotClass = cn(
          'w-2 h-2 rounded-full shrink-0',
          status === 'thinking' ? `${cfg.dotThinking} animate-pulse` :
          status === 'working' ? `${cfg.dotWorking} animate-pulse` :
          isCompleted ? 'bg-green-500' :
          isError ? 'bg-red-500 animate-pulse' :
          'bg-gray-800'
        )

        const statusLabel = getStatusLabel(panel, provider)
        const selectedModel = selectedModels?.[provider] || 'auto'
        const providerModelOptions = modelOptions?.[provider] || []
        const canConfigureModel = Boolean(onModelSelect && providerModelOptions.length > 0)

        return (
          <div
            key={provider}
            className={cn(
              'flex-1 min-w-[90px] border-r last:border-r-0 flex flex-col justify-between gap-1 px-2 sm:px-3 py-2 sm:py-2.5 transition-all duration-500',
              isActive
                ? `${cfg.borderActive} ${cfg.bgActive} ${cfg.glowActive}`
                : `${cfg.borderIdle} bg-black/30`,
              isUnavailable && 'opacity-25',
              isError && 'border-red-500/30 bg-red-950/15',
            )}
          >
            {/* Name row */}
            <div className="flex items-center justify-between gap-2">
              <div className="min-w-0">
                <div className={cn(
                  'text-sm font-bold leading-tight truncate',
                  isActive ? cfg.textActive : 'text-gray-500'
                )}>
                  {cfg.label}
                </div>
                <div className="text-[9px] text-gray-700 font-mono">{cfg.tagline}</div>
              </div>
              <div className={dotClass} />
            </div>

            {/* Model name */}
            <div className={cn(
              'text-[10px] font-mono leading-tight truncate',
              isActive ? 'text-gray-400' : 'text-gray-700'
            )}>
              {panel?.liveModelName || '—'}
            </div>

            {canConfigureModel ? (
              <select
                aria-label={`${cfg.label} model`}
                className="w-full rounded border border-gray-800 bg-black/70 px-1.5 py-1 text-[10px] text-gray-200 outline-none disabled:cursor-not-allowed disabled:opacity-50"
                value={selectedModel}
                disabled={!isBuildActive || isUnavailable || modelUpdatePendingProvider === provider}
                onChange={(event) => onModelSelect?.(provider, event.target.value)}
              >
                <option value="auto">Auto</option>
                {providerModelOptions.map((option) => (
                  <option key={option.id} value={option.id}>{option.name}</option>
                ))}
              </select>
            ) : null}

            {/* Status badge */}
            <div className={cn(
              'text-[9px] font-bold uppercase tracking-widest px-1.5 py-0.5 rounded border self-start',
              isActive ? cfg.badgeActive :
              isCompleted ? 'bg-green-500/15 border-green-500/25 text-green-500' :
              isError ? 'bg-red-500/15 border-red-500/25 text-red-400' :
              'bg-gray-900/60 border-gray-800 text-gray-600'
            )}>
              {statusLabel}
            </div>
          </div>
        )
      })}

      {/* Local / Ollama — 5th box, always rendered */}
      <div
        className={cn(
          'flex-1 min-w-[90px] flex flex-col justify-between px-2 sm:px-3 py-2 sm:py-2.5 transition-all duration-500',
          hasBYOK
            ? 'border-l border-gray-600/40 bg-gradient-to-br from-gray-800/30 via-black to-gray-800/15'
            : 'border-l border-gray-900/60 bg-black/20 opacity-30 grayscale',
        )}
      >
        <div className="flex items-center justify-between gap-2">
          <div className="min-w-0">
            <div className={cn('text-sm font-bold leading-tight', hasBYOK ? 'text-gray-300' : 'text-gray-700')}>
              Local
            </div>
            <div className="text-[9px] text-gray-700 font-mono">Ollama / BYOK</div>
          </div>
          <div className={cn('w-2 h-2 rounded-full shrink-0', hasBYOK ? 'bg-gray-500' : 'bg-gray-900')} />
        </div>
        <div className="text-[10px] font-mono text-gray-700 truncate">
          {hasBYOK ? 'local model' : 'not active'}
        </div>
        <div className={cn(
          'text-[9px] font-bold uppercase tracking-widest px-1.5 py-0.5 rounded border self-start',
          hasBYOK
            ? 'bg-gray-700/40 border-gray-600 text-gray-400'
            : 'bg-gray-900/40 border-gray-800 text-gray-700'
        )}>
          {hasBYOK ? 'ACTIVE' : 'BYOK ONLY'}
        </div>
      </div>
    </div>
  )
}

export default ProviderStatusBar
