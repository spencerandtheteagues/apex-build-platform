// Provider Status Bar - live model control for every build provider.

import React from 'react'
import { cn } from '@/lib/utils'

type ProviderStatus = 'idle' | 'working' | 'thinking' | 'completed' | 'error' | 'unavailable'
type SupportedProvider = 'claude' | 'gpt4' | 'gemini' | 'grok' | 'ollama'

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

const DISPLAY_ORDER: SupportedProvider[] = ['gpt4', 'gemini', 'grok', 'claude', 'ollama']

const PROVIDER_CONFIG: Record<SupportedProvider, {
  label: string
  tagline: string
  routeLabel: string
  accent: string
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
    routeLabel: 'Cloud',
    accent: 'bg-emerald-400',
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
    routeLabel: 'Cloud',
    accent: 'bg-sky-400',
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
    routeLabel: 'Cloud',
    accent: 'bg-fuchsia-400',
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
    routeLabel: 'Cloud',
    accent: 'bg-blue-300',
    borderActive: 'border-blue-500/70',
    borderIdle: 'border-blue-500/15',
    bgActive: 'bg-gradient-to-br from-blue-950/60 via-black to-slate-950/35',
    textActive: 'text-blue-200',
    glowActive: 'shadow-[0_0_24px_rgba(96,165,250,0.18)]',
    dotWorking: 'bg-blue-300',
    dotThinking: 'bg-cyan-300',
    badgeActive: 'bg-blue-500/20 border-blue-500/40 text-blue-200',
  },
  ollama: {
    label: 'Ollama',
    tagline: 'Kimi K2.6 / Cloud',
    routeLabel: 'Cloud',
    accent: 'bg-cyan-300',
    borderActive: 'border-cyan-500/70',
    borderIdle: 'border-cyan-500/15',
    bgActive: 'bg-gradient-to-br from-cyan-950/60 via-black to-slate-950/40',
    textActive: 'text-cyan-200',
    glowActive: 'shadow-[0_0_24px_rgba(34,211,238,0.2)]',
    dotWorking: 'bg-cyan-300',
    dotThinking: 'bg-yellow-400',
    badgeActive: 'bg-cyan-500/20 border-cyan-500/40 text-cyan-200',
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
    if (provider === 'ollama' && !hasBYOK && panel?.available === false) return 'BYOK ONLY'
    if (status === 'thinking') return 'THINKING'
    if (status === 'working') return 'WORKING'
    if (status === 'completed') return 'DONE'
    if (status === 'error') return 'ERROR'
    if (status === 'unavailable') return 'N/A'
    return isBuildActive ? 'STANDBY' : 'IDLE'
  }

  return (
    <div className="flex border-b border-sky-500/20 bg-slate-950/95 shrink-0 overflow-x-auto" style={{ minHeight: '112px' }}>
      {DISPLAY_ORDER.map((provider) => {
        const panel = panelMap[provider]
        const cfg = PROVIDER_CONFIG[provider]
        const status = panel?.status || 'idle'
        const isActive = status === 'working' || status === 'thinking'
        const isCompleted = status === 'completed'
        const isError = status === 'error'
        const isLocalProvider = provider === 'ollama'
        const localDisabled = isLocalProvider && !hasBYOK && panel?.available === false
        const isUnavailable = status === 'unavailable' || localDisabled

        const dotClass = cn(
          'w-2 h-2 rounded-full shrink-0',
          status === 'thinking' ? `${cfg.dotThinking} animate-pulse` :
            status === 'working' ? `${cfg.dotWorking} animate-pulse` :
              isCompleted ? 'bg-green-500' :
                isError ? 'bg-red-500 animate-pulse' :
                  localDisabled ? 'bg-gray-900' :
                    'bg-gray-800'
        )

        const statusLabel = getStatusLabel(panel, provider)
        const selectedModel = selectedModels?.[provider] || 'auto'
        const providerModelOptions = modelOptions?.[provider] || []
        const canConfigureModel = Boolean(onModelSelect && providerModelOptions.length > 0)
        const modelDescriptor = panel?.liveModelName || (localDisabled ? 'Add Ollama route' : 'Awaiting route')

        return (
          <div
            key={provider}
            className={cn(
              'relative flex-1 min-w-[132px] border-r last:border-r-0 flex flex-col justify-between gap-2 px-2 sm:px-3 py-2 sm:py-2.5 transition-all duration-500',
              isActive
                ? `${cfg.borderActive} ${cfg.bgActive} ${cfg.glowActive}`
                : `${cfg.borderIdle} bg-slate-950/82`,
              isUnavailable && 'opacity-55 grayscale',
              isError && 'border-red-500/30 bg-red-950/15 opacity-100 grayscale-0',
            )}
          >
            <div
              className={cn(
                'absolute inset-x-3 top-0 h-px rounded-full opacity-80',
                isError ? 'bg-red-400' : cfg.accent
              )}
            />

            <div className="flex items-center justify-between gap-2">
              <div className="min-w-0">
                <div className={cn(
                  'text-sm font-bold leading-tight truncate',
                  isActive ? cfg.textActive : localDisabled ? 'text-slate-500' : 'text-slate-200'
                )}>
                  {cfg.label}
                </div>
                <div className="text-[9px] text-slate-400 font-mono">{cfg.tagline}</div>
              </div>
              <div className={dotClass} />
            </div>

            <div className="flex flex-wrap items-center gap-1">
              <span className={cn(
                'rounded-full border px-1.5 py-0.5 text-[8px] font-bold uppercase tracking-[0.24em]',
                isError
                  ? 'border-red-500/30 bg-red-500/10 text-red-300'
                  : isLocalProvider
                    ? 'border-cyan-500/30 bg-cyan-500/10 text-cyan-200'
                    : 'border-sky-500/15 bg-sky-500/5 text-slate-300'
              )}>
                {cfg.routeLabel}
              </span>
              {isLocalProvider ? (
                <span className={cn(
                  'rounded-full border px-1.5 py-0.5 text-[8px] font-bold uppercase tracking-[0.24em]',
                  hasBYOK
                    ? 'border-cyan-500/30 bg-cyan-500/10 text-cyan-200'
                      : 'border-slate-700 bg-slate-950/70 text-slate-400'
                )}>
                  {hasBYOK ? 'BYOK Ready' : panel?.available !== false ? 'Hosted Ready' : 'BYOK Required'}
                </span>
              ) : null}
            </div>

            <div className="space-y-1">
              <div className="text-[9px] font-semibold uppercase tracking-[0.24em] text-sky-300/70">
                Active Model
              </div>
              <div className={cn(
                'text-[10px] font-mono leading-tight truncate',
                isActive ? 'text-slate-100' : localDisabled ? 'text-slate-500' : 'text-slate-300'
              )}>
                {modelDescriptor}
              </div>
            </div>

            <div className="space-y-1">
              <div className="text-[9px] font-semibold uppercase tracking-[0.24em] text-sky-300/70">
                Route Control
              </div>
              {canConfigureModel ? (
                <select
                  aria-label={`${cfg.label} model`}
                  className="w-full rounded border border-sky-500/20 bg-slate-950/85 px-1.5 py-1 text-[10px] text-slate-100 outline-none focus:border-cyan-300/60 disabled:cursor-not-allowed disabled:opacity-50"
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
              <div className="text-[9px] text-slate-400">
                {isLocalProvider
                  ? hasBYOK
                    ? 'BYOK routing is enabled for this build.'
                    : panel?.available !== false
                      ? 'Hosted Ollama Cloud route is available.'
                      : 'Connect an Ollama BYOK route to activate it.'
                  : 'Adjust model selection without restarting the workspace.'}
              </div>
            </div>

            <div className={cn(
              'text-[9px] font-bold uppercase tracking-widest px-1.5 py-0.5 rounded border self-start',
              isActive ? cfg.badgeActive :
                isCompleted ? 'bg-green-500/15 border-green-500/25 text-green-500' :
                  isError ? 'bg-red-500/15 border-red-500/25 text-red-400' :
                    localDisabled ? 'bg-slate-900/50 border-slate-700 text-slate-500' :
                      'bg-slate-900/70 border-slate-700 text-slate-300'
            )}>
              {statusLabel}
            </div>
          </div>
        )
      })}
    </div>
  )
}

export default ProviderStatusBar
