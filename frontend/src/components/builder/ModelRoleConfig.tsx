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
  Sparkles,
  Settings,
  AlertCircle,
  Check,
} from 'lucide-react'

// User-facing role categories (maps to backend UserRoleCategory)
const ROLE_CATEGORIES = [
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

// Default assignments matching backend policy
const DEFAULT_ASSIGNMENTS: Record<string, string> = {
  architect: 'claude',
  coder: 'gpt4',
  tester: 'gemini',
  devops: 'grok',
}

// Chip color classes per role
const CHIP_COLORS: Record<string, { active: string; ring: string }> = {
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
}

export default function ModelRoleConfig({
  mode,
  onModeChange,
  assignments,
  onAssignmentsChange,
  providerStatuses,
}: ModelRoleConfigProps) {
  // Build visible providers: always show platform providers, add Ollama only when user has it
  const visibleProviders = 'ollama' in providerStatuses
    ? [...PLATFORM_PROVIDERS, OLLAMA_PROVIDER]
    : [...PLATFORM_PROVIDERS]

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
    <Card variant="cyberpunk" glow="intense" className="border-2 border-red-900/40 bg-black/60 backdrop-blur-xl">
      <CardContent className="p-6 md:p-8">
        {/* Header */}
        <h3 className="text-xl font-bold text-gray-200 mb-6 flex items-center gap-3">
          <Settings className="w-6 h-6 text-red-400" />
          Model Configuration
        </h3>

        {/* Auto / Manual Toggle */}
        <div className="flex items-center justify-center gap-4 mb-8">
          <button
            onClick={() => onModeChange('auto')}
            className={cn(
              'relative flex items-center gap-3 px-6 py-3 rounded-xl transition-all duration-300 overflow-hidden',
              mode === 'auto'
                ? 'bg-gradient-to-r from-red-900/50 to-orange-900/40 border-2 border-red-500 text-red-400 shadow-xl shadow-red-900/40'
                : 'bg-gray-900/60 border-2 border-gray-700 text-gray-400 hover:border-gray-600 hover:text-gray-300'
            )}
          >
            {mode === 'auto' && (
              <div className="absolute inset-0 bg-gradient-to-r from-red-600/10 via-orange-600/10 to-red-600/10 animate-pulse" />
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
                ? 'bg-gradient-to-r from-red-900/50 to-orange-900/40 border-2 border-red-500 text-red-400 shadow-xl shadow-red-900/40'
                : 'bg-gray-900/60 border-2 border-gray-700 text-gray-400 hover:border-gray-600 hover:text-gray-300'
            )}
          >
            {mode === 'manual' && (
              <div className="absolute inset-0 bg-gradient-to-r from-red-600/10 via-orange-600/10 to-red-600/10 animate-pulse" />
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
              Click a role chip to assign it to a model. Each role can only be assigned to one model.
            </p>

            <div className="space-y-3">
              {visibleProviders.map((provider) => {
                const available = isProviderAvailable(provider.id)
                const assignedRoles = ROLE_CATEGORIES.filter(
                  cat => assignments[cat.id] === provider.id
                )

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

            {/* Validation */}
            {!isValid && (
              <div className="mt-4 p-3 rounded-lg bg-red-900/20 border border-red-800/40">
                <p className="text-xs text-red-400 flex items-center gap-1.5">
                  <AlertCircle className="w-3.5 h-3.5 flex-shrink-0" />
                  Assign at least <strong>Architect</strong> and <strong>Coder</strong> roles to start a build.
                </p>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
