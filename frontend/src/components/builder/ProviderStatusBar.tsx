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
    borderActive: 'border-violet-500/70',
    borderIdle: 'border-violet-500/15',
    bgActive: 'bg-gradient-to-br from-violet-950/60 via-black to-violet-950/25',
    textActive: 'text-violet-300',
    glowActive: 'shadow-[0_0_24px_rgba(139,92,246,0.18)]',
    dotWorking: 'bg-violet-400',
    dotThinking: 'bg-yellow-400',
    badgeActive: 'bg-violet-500/20 border-violet-500/40 text-violet-300',
  },
  ollama: {
    label: 'Kimi / Local',
    tagline: 'Ollama BYOK',
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
    if (provider === 'ollama' && !hasBYOK) return 'BYOK ONLY'
    if (status === 'thinking') return 'THINKING'
    if (status === 'working') return 'WORKING'
    if (status === 'completed') return 'DONE'
    if (status === 'error') return 'ERROR'
    if (status === 'unavailable') return 'N/A'
    return isBuildActive ? 'STANDBY' : 'IDLE'
  }

  return (
    <div className="build-screen-panel build-screen-provider-grid shrink-0" style={{ minHeight: '124px' }}>
      {DISPLAY_ORDER.map((provider) => {
        const panel = panelMap[provider]
        const cfg = PROVIDER_CONFIG[provider]
        const status = panel?.status || 'idle'
        const isActive = status === 'working' || status === 'thinking'
        const isCompleted = status === 'completed'
        const isError = status === 'error'
        const isLocalProvider = provider === 'ollama'
        const localDisabled = isLocalProvider && !hasBYOK
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

        return (
          <div
            key={provider}
            className={cn(
              'min-w-0 rounded-[20px] border flex flex-col justify-between gap-2 px-3 py-3 transition-all duration-500',
              isActive
                ? `${cfg.borderActive} ${cfg.bgActive} ${cfg.glowActive}`
                : `${cfg.borderIdle} bg-[rgba(7,12,20,0.72)]`,
              isUnavailable && 'opacity-35 grayscale',
              isError && 'border-red-500/30 bg-red-950/15 opacity-100 grayscale-0 shadow-[0_0_18px_rgba(239,68,68,0.12)]',
            )}
          >
            <div className="flex items-center justify-between gap-2">
              <div className="min-w-0">
                <div className={cn(
                  'text-sm font-bold leading-tight truncate',
                  isActive ? cfg.textActive : localDisabled ? 'text-gray-700' : 'text-slate-300'
                )}>
                  {cfg.label}
                </div>
                <div className="text-[9px] text-slate-500 font-mono uppercase tracking-[0.18em]">{cfg.tagline}</div>
              </div>
              <div className={dotClass} />
            </div>

            <div className={cn(
              'text-[10px] font-mono leading-tight truncate',
              isActive ? 'text-slate-200' : localDisabled ? 'text-gray-800' : 'text-slate-500'
            )}>
              {panel?.liveModelName || (localDisabled ? 'connect key' : '-')}
            </div>

            {canConfigureModel ? (
              <select
                aria-label={`${cfg.label} model`}
                className="w-full rounded-lg border border-[rgba(184,226,255,0.12)] bg-[rgba(4,8,14,0.82)] px-2 py-1.5 text-[10px] text-slate-200 outline-none disabled:cursor-not-allowed disabled:opacity-50"
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

            <div className={cn(
              'text-[9px] font-bold uppercase tracking-widest px-1.5 py-0.5 rounded border self-start',
              isActive ? cfg.badgeActive :
                isCompleted ? 'bg-green-500/15 border-green-500/25 text-green-500' :
                  isError ? 'bg-red-500/15 border-red-500/25 text-red-400' :
                    localDisabled ? 'bg-gray-900/40 border-gray-800 text-gray-700' :
                      'bg-[rgba(255,255,255,0.04)] border-[rgba(184,226,255,0.1)] text-slate-400'
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
