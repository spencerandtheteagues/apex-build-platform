import React from 'react'
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
  Network,
  Sparkles,
  Settings,
  AlertCircle,
  Check,
  Lock,
  ChevronDown,
  Zap,
} from 'lucide-react'

// User-facing role categories (maps to backend UserRoleCategory)
const ROLE_CATEGORIES = [
  { id: 'orchestrator', label: 'Orchestrator', desc: 'Coordinates all agents', icon: Network, color: 'rose' },
  { id: 'architect', label: 'Architect', desc: 'Planning, design, review', icon: Brain, color: 'purple' },
  { id: 'coder', label: 'Coder', desc: 'Frontend, backend, database', icon: Code2, color: 'blue' },
  { id: 'tester', label: 'Tester', desc: 'Testing & QA', icon: TestTube, color: 'green' },
  { id: 'devops', label: 'DevOps', desc: 'Deploy, debug, fixes', icon: Server, color: 'orange' },
] as const

// Provider metadata — platform providers always shown; Ollama shown only when user has it configured
const PLATFORM_PROVIDERS = [
  { id: 'claude', label: 'Claude', subtitle: 'Anthropic', letter: 'A', borderActive: 'border-orange-500/60', bgActive: 'bg-orange-500/10', text: 'text-orange-400', shadow: 'shadow-orange-500/10' },
  { id: 'gpt4', label: 'ChatGPT', subtitle: 'OpenAI', letter: 'G', borderActive: 'border-emerald-500/60', bgActive: 'bg-emerald-500/10', text: 'text-emerald-400', shadow: 'shadow-emerald-500/10' },
  { id: 'gemini', label: 'Gemini', subtitle: 'Google', letter: 'G', borderActive: 'border-blue-500/60', bgActive: 'bg-blue-500/10', text: 'text-blue-400', shadow: 'shadow-blue-500/10' },
  { id: 'grok', label: 'Grok', subtitle: 'xAI', letter: 'X', borderActive: 'border-violet-500/60', bgActive: 'bg-violet-500/10', text: 'text-violet-400', shadow: 'shadow-violet-500/10' },
] as const

const OLLAMA_PROVIDER = { id: 'ollama', label: 'Ollama', subtitle: 'Local', letter: 'O', borderActive: 'border-cyan-500/60', bgActive: 'bg-cyan-500/10', text: 'text-cyan-400', shadow: 'shadow-cyan-500/10' } as const

// Ollama Cloud models available under Pro+ flat-rate subscription
const OLLAMA_CLOUD_MODELS = [
  { id: 'kimi-k2.6',         name: 'Kimi K2.6',         desc: '128B MoE · Frontier reasoning + long context' },
  { id: 'glm-5.1',           name: 'GLM-5.1',            desc: '128B · Fast + capable, excels at code' },
  { id: 'deepseek-v4-pro',   name: 'DeepSeek V4 Pro',    desc: '236B MoE · Top code + math reasoning' },
  { id: 'deepseek-v4-flash', name: 'DeepSeek V4 Flash',  desc: '21B · Ultra-fast DeepSeek V4 distill' },
  { id: 'qwen3.5:397b',      name: 'Qwen 3.5 (397B)',    desc: '397B MoE · Best open-weight general model' },
  { id: 'gemma4:31b',        name: 'Gemma 4 (31B)',       desc: '31B · Google\'s efficient open model' },
  { id: 'devstral-2:123b',   name: 'Devstral 2 (123B)',  desc: '123B · Mistral\'s best coding model' },
] as const

// Default assignments matching backend policy
const DEFAULT_ASSIGNMENTS: Record<string, string> = {
  orchestrator: 'claude',
  architect: 'claude',
  coder: 'gpt4',
  tester: 'gemini',
  devops: 'grok',
}

// Chip color classes per role
const CHIP_COLORS: Record<string, { active: string; ring: string }> = {
  rose: { active: 'border-rose-500/70 bg-rose-500/20 text-rose-300', ring: 'ring-rose-500/30' },
  purple: { active: 'border-purple-500/70 bg-purple-500/20 text-purple-300', ring: 'ring-purple-500/30' },
  blue: { active: 'border-blue-500/70 bg-blue-500/20 text-blue-300', ring: 'ring-blue-500/30' },
  green: { active: 'border-green-500/70 bg-green-500/20 text-green-300', ring: 'ring-green-500/30' },
  orange: { active: 'border-orange-500/70 bg-orange-500/20 text-orange-300', ring: 'ring-orange-500/30' },
  cyan: { active: 'border-cyan-500/70 bg-cyan-500/20 text-cyan-300', ring: 'ring-cyan-500/30' },
}

interface ModelRoleConfigProps {
  mode: 'auto' | 'manual'
  onModeChange: (mode: 'auto' | 'manual') => void
  assignments: Record<string, string>
  onAssignmentsChange: (assignments: Record<string, string>) => void
  providerStatuses: Record<string, string>
  isPro?: boolean
  ollamaCloudModel?: string
  onOllamaCloudModelChange?: (model: string) => void
}

export default function ModelRoleConfig({
  mode,
  onModeChange,
  assignments,
  onAssignmentsChange,
  providerStatuses,
  isPro = false,
  ollamaCloudModel = 'kimi-k2.6',
  onOllamaCloudModelChange,
}: ModelRoleConfigProps) {
  const [cloudModelOpen, setCloudModelOpen] = React.useState(false)

  // Build visible providers: always show platform providers, add Ollama only when user has it
  const visibleProviders = 'ollama' in providerStatuses
    ? [...PLATFORM_PROVIDERS, OLLAMA_PROVIDER]
    : [...PLATFORM_PROVIDERS]

  const isValid = 'orchestrator' in assignments && 'architect' in assignments && 'coder' in assignments

  const toggleRole = (roleId: string, providerId: string) => {
    const updated = { ...assignments }
    if (updated[roleId] === providerId) {
      delete updated[roleId]
    } else {
      updated[roleId] = providerId
    }
    onAssignmentsChange(updated)
  }

  const isProviderAvailable = (providerId: string) => {
    if (Object.keys(providerStatuses).length === 0) return true
    return providerStatuses[providerId] === 'available'
  }

  const ollamaCloudAssignedRoles = ROLE_CATEGORIES.filter(cat => assignments[cat.id] === 'ollama_cloud')
  const selectedCloudModel = OLLAMA_CLOUD_MODELS.find(m => m.id === ollamaCloudModel) ?? OLLAMA_CLOUD_MODELS[0]

  return (
    <Card variant="cyberpunk" glow="intense" className="border border-[rgba(188,239,255,0.18)] bg-[rgba(7,15,32,0.78)] backdrop-blur-xl">
      <CardContent className="p-6 md:p-8">
        {/* Header */}
        <h3 className="text-xl font-bold text-gray-200 mb-6 flex items-center gap-3">
          <Settings className="w-6 h-6 text-[#8adfff]" />
          Model Configuration
        </h3>

        {/* Auto / Manual Toggle */}
        <div className="flex items-center justify-center gap-4 mb-8">
          <button
            onClick={() => onModeChange('auto')}
            className={cn(
              'relative flex items-center gap-3 px-6 py-3 rounded-xl transition-all duration-300 overflow-hidden',
              mode === 'auto'
                ? 'bg-gradient-to-r from-[rgba(47,168,255,0.18)] to-[rgba(138,223,255,0.12)] border border-[rgba(138,223,255,0.42)] text-[#c8f4ff] shadow-xl shadow-[rgba(47,168,255,0.16)]'
                : 'bg-gray-900/60 border-2 border-gray-700 text-gray-400 hover:border-gray-600 hover:text-gray-300'
            )}
          >
            {mode === 'auto' && (
              <div className="absolute inset-0 bg-gradient-to-r from-[rgba(47,168,255,0.14)] via-[rgba(138,223,255,0.1)] to-[rgba(47,168,255,0.14)] animate-pulse" />
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
                ? 'bg-gradient-to-r from-[rgba(47,168,255,0.18)] to-[rgba(138,223,255,0.12)] border border-[rgba(138,223,255,0.42)] text-[#c8f4ff] shadow-xl shadow-[rgba(47,168,255,0.16)]'
                : 'bg-gray-900/60 border-2 border-gray-700 text-gray-400 hover:border-gray-600 hover:text-gray-300'
            )}
          >
            {mode === 'manual' && (
              <div className="absolute inset-0 bg-gradient-to-r from-[rgba(47,168,255,0.14)] via-[rgba(138,223,255,0.1)] to-[rgba(47,168,255,0.14)] animate-pulse" />
            )}
            <Settings className={cn('w-5 h-5 relative z-10', mode === 'manual' && 'animate-pulse')} />
            <span className="relative z-10 font-bold">Manual</span>
          </button>
        </div>

        {mode === 'auto' ? (
          /* Auto Mode: show default assignments read-only */
          <div>
            <p className="text-sm text-gray-500 mb-5 leading-relaxed">
              APEX automatically assigns the best model for each task based on provider strengths.
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
            <p className="mt-4 text-[11px] text-gray-600 text-center">
              Switch to Manual to customize which model handles each role.
            </p>
          </div>
        ) : (
          /* Manual Mode: provider cards with role chips */
          <div>
            <p className="text-sm text-gray-500 mb-5 leading-relaxed">
              Click a role chip to assign it to a provider. Each role can only be assigned to one provider. Ollama (local) requires BYOK; Ollama Cloud is available to Pro+ subscribers at flat-rate.
            </p>

            <div className="space-y-3">
              {visibleProviders.map((provider) => {
                const available = isProviderAvailable(provider.id)
                const assignedRoles = ROLE_CATEGORIES.filter(
                  cat => assignments[cat.id] === provider.id
                )

                // Ollama always shows its cyan identity border, even with no roles assigned.
                const isOllama = provider.id === 'ollama'
                return (
                  <div
                    key={provider.id}
                    className={cn(
                      'p-4 rounded-xl border-2 transition-all duration-200',
                      available
                        ? assignedRoles.length > 0
                          ? `${provider.borderActive} ${provider.bgActive} shadow-lg ${provider.shadow}`
                          : isOllama
                            ? `${provider.borderActive} bg-gray-900/30`
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
                        <span className="text-[10px] font-medium px-2 py-0.5 rounded-full bg-red-500/15 text-red-400 border border-red-500/20">
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
                  </div>
                )
              })}
            </div>

            {/* Ollama Cloud section */}
            <div className={cn(
              'mt-1 p-4 rounded-xl border-2 transition-all duration-200',
              isPro
                ? ollamaCloudAssignedRoles.length > 0
                  ? 'border-purple-500/60 bg-purple-500/10 shadow-lg shadow-purple-500/10'
                  : 'border-purple-700/40 bg-gray-900/30 hover:border-purple-600/50'
                : 'border-gray-800/40 bg-gray-950/30 opacity-60'
            )}>
              {/* Header */}
              <div className="flex items-center gap-3 mb-3">
                <div className={cn(
                  'w-9 h-9 rounded-lg flex items-center justify-center font-bold text-base',
                  isPro ? 'bg-purple-500/20 text-purple-300' : 'bg-gray-900 text-gray-600'
                )}>
                  <Zap className="w-5 h-5" />
                </div>
                <div className="flex-1 min-w-0">
                  <span className={cn('font-bold text-sm', isPro ? 'text-white' : 'text-gray-600')}>
                    Ollama Cloud
                  </span>
                  <span className="text-[11px] text-gray-600 ml-1.5">7 top open-weight models · flat-rate</span>
                </div>
                {isPro ? (
                  <span className="text-[10px] font-medium px-2 py-0.5 rounded-full bg-purple-500/15 text-purple-400 border border-purple-500/20">
                    Pro
                  </span>
                ) : (
                  <span className="text-[10px] font-medium px-2 py-0.5 rounded-full bg-gray-700/40 text-gray-500 border border-gray-700/40 flex items-center gap-1">
                    <Lock className="w-2.5 h-2.5" />Pro+
                  </span>
                )}
              </div>

              {isPro ? (
                <>
                  {/* Model selector */}
                  <div className="mb-3 relative">
                    <button
                      onClick={() => setCloudModelOpen(v => !v)}
                      className="w-full flex items-center justify-between px-3 py-2 rounded-lg bg-gray-900/60 border border-gray-700/60 hover:border-purple-600/50 transition-colors text-left"
                    >
                      <div>
                        <span className="text-sm font-medium text-gray-200">{selectedCloudModel.name}</span>
                        <span className="text-[11px] text-gray-500 ml-2">{selectedCloudModel.desc}</span>
                      </div>
                      <ChevronDown className={cn('w-4 h-4 text-gray-500 flex-shrink-0 transition-transform', cloudModelOpen && 'rotate-180')} />
                    </button>
                    {cloudModelOpen && (
                      <div className="absolute z-20 top-full mt-1 w-full rounded-xl bg-gray-900 border border-gray-700/60 shadow-xl overflow-hidden">
                        {OLLAMA_CLOUD_MODELS.map(model => (
                          <button
                            key={model.id}
                            onClick={() => {
                              onOllamaCloudModelChange?.(model.id)
                              setCloudModelOpen(false)
                            }}
                            className={cn(
                              'w-full text-left px-3 py-2.5 hover:bg-purple-500/10 transition-colors flex items-start gap-2',
                              ollamaCloudModel === model.id && 'bg-purple-500/15'
                            )}
                          >
                            {ollamaCloudModel === model.id && <Check className="w-3.5 h-3.5 text-purple-400 mt-0.5 flex-shrink-0" />}
                            {ollamaCloudModel !== model.id && <span className="w-3.5 h-3.5 flex-shrink-0" />}
                            <div>
                              <div className="text-sm font-medium text-gray-200">{model.name}</div>
                              <div className="text-[11px] text-gray-500">{model.desc}</div>
                            </div>
                          </button>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Role chips */}
                  <div className="flex flex-wrap gap-2">
                    {ROLE_CATEGORIES.map((cat) => {
                      const isAssigned = assignments[cat.id] === 'ollama_cloud'
                      const Icon = cat.icon
                      const colors = CHIP_COLORS[cat.color]
                      return (
                        <button
                          key={cat.id}
                          onClick={() => toggleRole(cat.id, 'ollama_cloud')}
                          className={cn(
                            'flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all duration-150 border',
                            isAssigned
                              ? `${colors.active} ring-1 ${colors.ring}`
                              : 'border-gray-700/50 bg-gray-800/40 text-gray-500 hover:border-purple-600/40 hover:text-gray-400'
                          )}
                        >
                          <Icon className="w-3.5 h-3.5" />
                          {cat.label}
                        </button>
                      )
                    })}
                  </div>
                </>
              ) : (
                <p className="text-xs text-gray-500">
                  Upgrade to <strong className="text-gray-400">Pro ($59/mo)</strong> to route agents through Ollama Cloud — kimi-k2.6, GLM-5.1, DeepSeek V4, Qwen 3.5, Gemma 4, Devstral 2, and more at flat-rate.
                </p>
              )}
            </div>

            {/* Validation */}
            {!isValid && (
              <div className="mt-4 p-3 rounded-lg bg-red-900/20 border border-red-800/40">
                <p className="text-xs text-red-400 flex items-center gap-1.5">
                  <AlertCircle className="w-3.5 h-3.5 flex-shrink-0" />
                  Assign at least <strong>Orchestrator</strong>, <strong>Architect</strong>, and <strong>Coder</strong> roles to start a build.
                </p>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
