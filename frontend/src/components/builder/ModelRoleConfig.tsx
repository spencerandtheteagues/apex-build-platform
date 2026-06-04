import React, { useState } from 'react'
import { cn } from '@/lib/utils'
import {
  Card,
  CardContent,
} from '@/components/ui'
import {
  Brain,
  Code2,
  TestTube,
  Server,
  Sparkles,
  Settings,
  AlertCircle,
  Check,
  ChevronDown,
  Bot,
} from 'lucide-react'
import OpenRouterModelPicker from './OpenRouterModelPicker'

// User-facing role categories (maps to backend UserRoleCategory)
const ROLE_CATEGORIES = [
  { id: 'architect', label: 'Architect', desc: 'Planning, design, review', icon: Brain, color: 'cyan' },
  { id: 'coder', label: 'Coder', desc: 'Frontend, backend, database', icon: Code2, color: 'blue' },
  { id: 'tester', label: 'Tester', desc: 'Testing & QA', icon: TestTube, color: 'green' },
  { id: 'devops', label: 'DevOps', desc: 'Deploy, debug, fixes', icon: Server, color: 'cyan' },
] as const

// Provider metadata — all routed providers are shown so users can see and control Ollama beside the hosted models.
const PLATFORM_PROVIDERS = [
  { id: 'claude', label: 'Claude', subtitle: 'Anthropic', letter: 'A', borderActive: 'border-blue-500/60', bgActive: 'bg-blue-500/10', text: 'text-blue-300', shadow: 'shadow-blue-500/10' },
  { id: 'gpt4', label: 'ChatGPT', subtitle: 'OpenAI', letter: 'G', borderActive: 'border-emerald-500/60', bgActive: 'bg-emerald-500/10', text: 'text-emerald-400', shadow: 'shadow-emerald-500/10' },
  { id: 'gemini', label: 'Gemini', subtitle: 'Google', letter: 'G', borderActive: 'border-blue-500/60', bgActive: 'bg-blue-500/10', text: 'text-blue-400', shadow: 'shadow-blue-500/10' },
  { id: 'grok', label: 'Grok', subtitle: 'xAI', letter: 'X', borderActive: 'border-sky-500/60', bgActive: 'bg-sky-500/10', text: 'text-sky-300', shadow: 'shadow-sky-500/10' },
] as const

const OLLAMA_PROVIDER = { id: 'ollama', label: 'Ollama', subtitle: 'Kimi K2.6 Cloud/BYOK', letter: 'O', borderActive: 'border-cyan-500/60', bgActive: 'bg-cyan-500/10', text: 'text-cyan-400', shadow: 'shadow-cyan-500/10' } as const
const OPENROUTER_PROVIDER = { id: 'openrouter', label: 'OpenRouter', subtitle: '300+ models', letter: 'R', borderActive: 'border-violet-500/60', bgActive: 'bg-violet-500/10', text: 'text-violet-300', shadow: 'shadow-violet-500/10' } as const
type RoutedProviderId = typeof PLATFORM_PROVIDERS[number]['id'] | typeof OLLAMA_PROVIDER['id'] | typeof OPENROUTER_PROVIDER['id']

// Default assignments matching backend policy
const DEFAULT_ASSIGNMENTS: Record<string, string> = {
  architect: 'claude',
  coder: 'gpt4',
  tester: 'gemini',
  devops: 'grok',
}

const AUTO_PROVIDER_ROUTES: Record<RoutedProviderId, string> = {
  claude: 'Planning, architecture, review',
  gpt4: 'Implementation, repair, synthesis',
  gemini: 'Testing, validation, fast checks',
  grok: 'Risk review, alternate repair',
  ollama: 'Open-model orchestration and fallback',
  openrouter: 'Smart dispatch across 300+ models',
}

// Chip color classes per role
const CHIP_COLORS: Record<string, { active: string; ring: string }> = {
  purple: { active: 'border-cyan-500/70 bg-cyan-500/20 text-cyan-300', ring: 'ring-cyan-500/30' },
  blue: { active: 'border-blue-500/70 bg-blue-500/20 text-blue-300', ring: 'ring-blue-500/30' },
  green: { active: 'border-green-500/70 bg-green-500/20 text-green-300', ring: 'ring-green-500/30' },
  orange: { active: 'border-cyan-500/70 bg-cyan-500/20 text-cyan-300', ring: 'ring-cyan-500/30' },
  cyan: { active: 'border-cyan-500/70 bg-cyan-500/20 text-cyan-300', ring: 'ring-cyan-500/30' },
}

interface ModelRoleConfigProps {
  mode: 'auto' | 'manual'
  onModeChange: (mode: 'auto' | 'manual') => void
  assignments: Record<string, string>
  onAssignmentsChange: (assignments: Record<string, string>) => void
  providerStatuses: Record<string, string>
  selectedModels?: Record<string, string>
  modelOptions?: Record<string, Array<{ id: string; name: string }>>
  onModelSelect?: (provider: RoutedProviderId, model: string) => void
}

export default function ModelRoleConfig({
  mode,
  onModeChange,
  assignments,
  onAssignmentsChange,
  providerStatuses,
  selectedModels = {},
  modelOptions = {},
  onModelSelect,
}: ModelRoleConfigProps) {
  const visibleProviders = [...PLATFORM_PROVIDERS, OLLAMA_PROVIDER, OPENROUTER_PROVIDER]
  const [pickerOpen, setPickerOpen] = useState(false)
  const [pickerTargetRole, setPickerTargetRole] = useState<string | null>(null)

  const isValid = 'architect' in assignments && 'coder' in assignments

  const toggleRole = (roleId: string, providerId: string) => {
    const updated = { ...assignments }
    if (updated[roleId] === providerId) {
      // Unassign
      delete updated[roleId]
    } else {
      // Assign (moves from previous provider if any)
      updated[roleId] = providerId
    }
    onAssignmentsChange(updated)
  }

  const isProviderAvailable = (providerId: string) => {
    // If we have no status data yet (preflight hasn't returned), assume available
    if (Object.keys(providerStatuses).length === 0) return true
    return providerStatuses[providerId] === 'available'
  }

  return (
    <Card variant="cyberpunk" glow="intense" className="border-2 border-sky-500/30 bg-black/60 backdrop-blur-xl">
      <CardContent className="p-6 md:p-8">
        {/* Header */}
        <h3 className="text-xl font-bold text-gray-200 mb-6 flex items-center gap-3">
          <Settings className="w-6 h-6 text-sky-300" />
          Model Configuration
        </h3>

        {/* Auto / Manual Toggle */}
        <div className="flex items-center justify-center gap-4 mb-8">
          <button
            onClick={() => onModeChange('auto')}
            className={cn(
              'relative flex items-center gap-3 px-6 py-3 rounded-xl transition-all duration-300 overflow-hidden',
              mode === 'auto'
                ? 'bg-gradient-to-r from-sky-900/55 to-cyan-900/40 border-2 border-sky-400 text-sky-200 shadow-xl shadow-sky-900/35'
                : 'bg-gray-900/60 border-2 border-gray-700 text-gray-400 hover:border-gray-600 hover:text-gray-300'
            )}
          >
            {mode === 'auto' && (
              <div className="absolute inset-0 bg-gradient-to-r from-sky-500/10 via-cyan-500/10 to-blue-500/10 animate-pulse" />
            )}
            <Sparkles className={cn('w-5 h-5 relative z-10', mode === 'auto' && 'animate-pulse')} />
            <span className="relative z-10 font-bold">Auto</span>
          </button>
          <button
            onClick={() => {
              onModeChange('manual')
              // Initialize with defaults if empty
              if (Object.keys(assignments).length === 0) {
                onAssignmentsChange({ ...DEFAULT_ASSIGNMENTS })
              }
            }}
            className={cn(
              'relative flex items-center gap-3 px-6 py-3 rounded-xl transition-all duration-300 overflow-hidden',
              mode === 'manual'
                ? 'bg-gradient-to-r from-sky-900/55 to-cyan-900/40 border-2 border-sky-400 text-sky-200 shadow-xl shadow-sky-900/35'
                : 'bg-gray-900/60 border-2 border-gray-700 text-gray-400 hover:border-gray-600 hover:text-gray-300'
            )}
          >
            {mode === 'manual' && (
              <div className="absolute inset-0 bg-gradient-to-r from-sky-500/10 via-cyan-500/10 to-blue-500/10 animate-pulse" />
            )}
            <Settings className={cn('w-5 h-5 relative z-10', mode === 'manual' && 'animate-pulse')} />
            <span className="relative z-10 font-bold">Manual</span>
          </button>
        </div>

        {mode === 'auto' ? (
          /* Auto Mode: show default assignments read-only */
          <div>
            <p className="text-sm text-gray-500 mb-5 leading-relaxed">
              APEX automatically assigns the best model for each task based on provider health, power mode, and task type. Ollama is part of the automatic route pool when hosted Cloud or BYOK/local routing is available.
            </p>
            <div className="space-y-2.5">
              {ROLE_CATEGORIES.map((cat) => {
                const provider = PLATFORM_PROVIDERS.find(p => p.id === DEFAULT_ASSIGNMENTS[cat.id])
                const Icon = cat.icon
                return (
                  <div
                    key={cat.id}
                    className="flex items-center justify-between p-3.5 rounded-xl bg-gray-900/50 border border-gray-800/60"
                  >
                    <div className="flex items-center gap-3">
                      <Icon className={cn('w-4 h-4', CHIP_COLORS[cat.color]?.active.split(' ').pop())} />
                      <div>
                        <span className="text-sm font-semibold text-gray-200">{cat.label}</span>
                        <span className="text-xs text-gray-600 ml-2">{cat.desc}</span>
                      </div>
                    </div>
                    <span className={cn('text-xs font-bold px-3 py-1 rounded-lg', provider?.bgActive, provider?.text)}>
                      {provider?.label}
                    </span>
                  </div>
                )
              })}
            </div>
            <div className="mt-5 rounded-2xl border border-cyan-500/20 bg-slate-950/60 p-4">
              <div className="mb-3 flex items-center justify-between gap-3">
                <div>
                  <p className="text-xs font-bold uppercase tracking-[0.2em] text-cyan-300">Automatic Provider Pool</p>
                  <p className="mt-1 text-[11px] leading-5 text-gray-600">
                    All five routes are eligible in Auto. Use Manual only when you want to force a provider or Ollama Cloud model for the next build.
                  </p>
                </div>
              </div>
              <div className="grid gap-3 md:grid-cols-2">
                {visibleProviders.map((provider) => {
                  const available = isProviderAvailable(provider.id)
                  const selectedModel = selectedModels[provider.id] || 'auto'
                  const modelName = selectedModel === 'auto'
                    ? 'Auto'
                    : modelOptions[provider.id]?.find((model) => model.id === selectedModel)?.name || selectedModel
                  return (
                    <div
                      key={provider.id}
                      className={cn(
                        'rounded-xl border p-3 transition-colors',
                        available
                          ? `${provider.borderActive} ${provider.bgActive}`
                          : 'border-gray-800/70 bg-gray-950/50 opacity-70'
                      )}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <div className="flex items-center gap-2">
                            <span className={cn('font-bold text-sm', available ? provider.text : 'text-gray-500')}>
                              {provider.label}
                            </span>
                            <span className="text-[10px] uppercase tracking-[0.16em] text-gray-600">
                              {available ? 'Ready' : 'Offline'}
                            </span>
                          </div>
                          <p className="mt-1 text-[11px] leading-5 text-gray-500">{AUTO_PROVIDER_ROUTES[provider.id]}</p>
                        </div>
                        <span className={cn('shrink-0 rounded-lg border px-2 py-1 text-[10px] font-semibold', available ? `${provider.borderActive} ${provider.text}` : 'border-gray-800 text-gray-600')}>
                          {modelName}
                        </span>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
            <p className="mt-4 text-[11px] text-gray-600 text-center">
              Switch to Manual to customize which model handles each role.
            </p>
          </div>
        ) : (
          /* Manual Mode: provider cards with role chips */
          <div>
            <p className="text-sm text-gray-500 mb-5 leading-relaxed">
              Click a role chip to assign it to a provider. OpenRouter lets you pick any of 300+ models per role. Ollama routes through hosted Ollama Cloud or BYOK when available.
            </p>

            {/* Auto ✦ quick-assign strip */}
            <div className="mb-4 p-3 rounded-xl border border-violet-500/20 bg-violet-500/5">
              <div className="flex items-center gap-2 mb-2">
                <Bot className="w-4 h-4 text-violet-400" />
                <span className="text-xs font-bold text-violet-300 uppercase tracking-wide">Auto ✦ — Assign role to OpenRouter dispatcher</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {ROLE_CATEGORIES.map((cat) => {
                  const isAutoAssigned = assignments[cat.id] === 'openrouter'
                  const Icon = cat.icon
                  return (
                    <button
                      key={cat.id}
                      onClick={() => {
                        const updated = { ...assignments }
                        if (isAutoAssigned) {
                          delete updated[cat.id]
                        } else {
                          updated[cat.id] = 'openrouter'
                        }
                        onAssignmentsChange(updated)
                        if (!isAutoAssigned) onModelSelect?.('openrouter' as RoutedProviderId, 'auto')
                      }}
                      className={cn(
                        'flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all duration-150 border',
                        isAutoAssigned
                          ? 'border-violet-500/70 bg-violet-500/20 text-violet-300 ring-1 ring-violet-500/30'
                          : 'border-gray-700/50 bg-gray-800/40 text-gray-500 hover:border-violet-500/40 hover:text-violet-400'
                      )}
                    >
                      <Icon className="w-3.5 h-3.5" />
                      {cat.label}
                      {isAutoAssigned && <Sparkles className="w-3 h-3 ml-0.5" />}
                    </button>
                  )
                })}
                <button
                  onClick={() => { setPickerTargetRole(null); setPickerOpen(true) }}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border border-violet-500/40 bg-violet-500/10 text-violet-300 hover:bg-violet-500/20 transition-colors"
                >
                  <ChevronDown className="w-3.5 h-3.5" />Pick specific model
                </button>
              </div>
            </div>

            <div className="space-y-3">
              {visibleProviders.map((provider) => {
                const available = isProviderAvailable(provider.id)
                const assignedRoles = ROLE_CATEGORIES.filter(
                  cat => assignments[cat.id] === provider.id
                )
                const providerModelOptions = modelOptions[provider.id] || []
                const selectedModel = selectedModels[provider.id] || 'auto'

                return (
                  <div
                    key={provider.id}
                    className={cn(
                      'p-4 rounded-xl border-2 transition-all duration-200',
                      available
                        ? assignedRoles.length > 0
                          ? `${provider.borderActive} ${provider.bgActive} shadow-lg ${provider.shadow}`
                          : 'border-gray-700/50 bg-gray-900/30 hover:border-gray-600/60'
                        : 'border-gray-800/40 bg-gray-950/30 opacity-40'
                    )}
                  >
                    {/* Provider header */}
                    <div className="flex items-center gap-3 mb-3">
                      <div className={cn(
                        'w-9 h-9 rounded-lg flex items-center justify-center font-bold text-base',
                        available ? `${provider.bgActive} ${provider.text}` : 'bg-gray-900 text-gray-600'
                      )}>
                        {provider.letter}
                      </div>
                      <div className="flex-1 min-w-0">
                        <span className={cn('font-bold text-sm', available ? 'text-white' : 'text-gray-600')}>
                          {provider.label}
                        </span>
                        <span className="text-[11px] text-gray-600 ml-1.5">{provider.subtitle}</span>
                      </div>
                      {available ? (
                        <span className="text-[10px] font-medium px-2 py-0.5 rounded-full bg-green-500/15 text-green-400 border border-green-500/20">
                          <Check className="w-3 h-3 inline mr-0.5" />Online
                        </span>
                      ) : (
                        <span className="text-[10px] font-medium px-2 py-0.5 rounded-full bg-sky-500/15 text-sky-300 border border-sky-500/20">
                          Offline
                        </span>
                      )}
                    </div>

                    {/* Role chips */}
                    <div className="flex flex-wrap gap-2">
                      {ROLE_CATEGORIES.map((cat) => {
                        const isAssigned = assignments[cat.id] === provider.id
                        const Icon = cat.icon
                        const colors = CHIP_COLORS[cat.color]

                        return (
                          <button
                            key={cat.id}
                            disabled={!available}
                            onClick={() => toggleRole(cat.id, provider.id)}
                            className={cn(
                              'flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all duration-150 border',
                              isAssigned
                                ? `${colors.active} ring-1 ${colors.ring}`
                                : 'border-gray-700/50 bg-gray-800/40 text-gray-500 hover:border-gray-600 hover:text-gray-400',
                              !available && 'cursor-not-allowed'
                            )}
                          >
                            <Icon className="w-3.5 h-3.5" />
                            {cat.label}
                          </button>
                        )
                      })}
                    </div>

                    <div className="mt-4 rounded-xl border border-white/10 bg-black/25 p-3">
                      <label className="mb-2 flex items-center justify-between gap-3 text-[10px] font-semibold uppercase tracking-[0.18em] text-gray-500">
                        <span>{provider.id === 'ollama' ? 'Ollama Cloud Model' : provider.id === 'openrouter' ? 'OpenRouter Model' : 'Model Override'}</span>
                        {selectedModel !== 'auto' && (
                          <span className={cn('rounded-full border px-2 py-0.5 normal-case tracking-normal', provider.borderActive, provider.text)}>
                            Locked
                          </span>
                        )}
                      </label>
                      {provider.id === 'openrouter' ? (
                        <div className="space-y-2">
                          <button
                            disabled={!available}
                            onClick={() => { setPickerTargetRole(null); setPickerOpen(true) }}
                            className={cn(
                              'w-full flex items-center justify-between gap-2 rounded-lg border border-violet-500/40 bg-violet-500/10 px-3 py-2 text-sm transition-colors',
                              available ? 'text-violet-200 hover:bg-violet-500/20' : 'opacity-50 cursor-not-allowed text-gray-600'
                            )}
                          >
                            <span className="truncate">{selectedModel === 'auto' ? 'Auto — dispatcher chooses per task' : selectedModel}</span>
                            <ChevronDown className="w-4 h-4 shrink-0" />
                          </button>
                          <p className="text-[11px] leading-5 text-gray-600">Browse 300+ models. GPT-5.5 auto-dispatches when set to Auto.</p>
                        </div>
                      ) : (
                        <>
                          <select
                            value={selectedModel}
                            disabled={!available || !onModelSelect}
                            onChange={(event) => onModelSelect?.(provider.id, event.target.value)}
                            className={cn(
                              'w-full rounded-lg border border-slate-700/80 bg-slate-950/90 px-3 py-2 text-sm text-slate-100 outline-none transition-colors',
                              'focus:border-cyan-400/70 focus:ring-2 focus:ring-cyan-400/15',
                              (!available || !onModelSelect) && 'cursor-not-allowed opacity-50'
                            )}
                          >
                            <option value="auto">Auto — Apex chooses per task</option>
                            {providerModelOptions.map((model) => (
                              <option key={model.id} value={model.id}>{model.name}</option>
                            ))}
                          </select>
                          <p className="mt-2 text-[11px] leading-5 text-gray-600">
                            {provider.id === 'ollama'
                              ? 'Hosted Ollama Cloud choices include Kimi, GLM, DeepSeek, Qwen, and Devstral routes; BYOK/local endpoints can still use their configured model.'
                              : 'Leave on Auto unless you want this provider locked to a specific tier for the next build.'}
                          </p>
                        </>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>

            {/* Validation */}
            {!isValid && (
              <div className="mt-4 p-3 rounded-lg bg-sky-900/20 border border-sky-700/40">
                <p className="text-xs text-sky-300 flex items-center gap-1.5">
                  <AlertCircle className="w-3.5 h-3.5 flex-shrink-0" />
                  Assign at least <strong>Architect</strong> and <strong>Coder</strong> roles to start a build.
                </p>
              </div>
            )}
          </div>
        )}
      </CardContent>

      <OpenRouterModelPicker
        isOpen={pickerOpen}
        onClose={() => setPickerOpen(false)}
        title="Pick an OpenRouter Model"
        onSelect={(modelId, _modelName) => {
          onModelSelect?.('openrouter' as RoutedProviderId, modelId)
          if (pickerTargetRole) {
            onAssignmentsChange({ ...assignments, [pickerTargetRole]: 'openrouter' })
          }
          setPickerTargetRole(null)
        }}
      />
    </Card>
  )
}
