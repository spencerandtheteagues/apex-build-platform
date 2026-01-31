// APEX.BUILD API Key Settings
// Multi-provider BYOK with active toggles and per-provider model selectors

import React, { useState, useEffect, useCallback } from 'react'
import { Key, Shield, CheckCircle, XCircle, Trash2, Loader2, Eye, EyeOff, RefreshCw, Power, ChevronDown, Zap, Brain, Sparkles } from 'lucide-react'
import { cn } from '@/lib/utils'
import apiService from '@/services/api'
import type { AIProvider } from '@/types'

interface ProviderConfig {
  id: AIProvider
  name: string
  description: string
  color: string
  bgColor: string
  bgGlow: string
  borderColor: string
  activeBorder: string
  placeholder: string
  models: ModelOption[]
}

interface ModelOption {
  id: string
  name: string
  speed: 'fast' | 'medium' | 'slow'
  cost: 'low' | 'medium' | 'high' | 'free'
  description: string
}

const PROVIDERS: ProviderConfig[] = [
  {
    id: 'claude',
    name: 'Claude (Anthropic)',
    description: 'Flagship reasoning models',
    color: 'text-orange-400',
    bgColor: 'bg-orange-500/10',
    bgGlow: 'shadow-orange-500/20',
    borderColor: 'border-orange-500/30',
    activeBorder: 'border-orange-500',
    placeholder: 'sk-ant-...',
    models: [
      { id: 'claude-opus-4-5-20251101', name: 'Claude Opus 4.5', speed: 'slow', cost: 'high', description: 'Most intelligent' },
      { id: 'claude-sonnet-4-20250514', name: 'Claude Sonnet 4', speed: 'medium', cost: 'medium', description: 'Balanced' },
      { id: 'claude-haiku-3-5-20241022', name: 'Claude Haiku 3.5', speed: 'fast', cost: 'low', description: 'Fast & cheap' },
    ],
  },
  {
    id: 'gpt4',
    name: 'GPT-5.2 (OpenAI)',
    description: '100% AIME, 400K context',
    color: 'text-green-400',
    bgColor: 'bg-green-500/10',
    bgGlow: 'shadow-green-500/20',
    borderColor: 'border-green-500/30',
    activeBorder: 'border-green-500',
    placeholder: 'sk-...',
    models: [
      { id: 'gpt-5.2-pro', name: 'GPT-5.2 Pro', speed: 'slow', cost: 'high', description: 'Most intelligent' },
      { id: 'gpt-5.2-thinking', name: 'GPT-5.2 Thinking', speed: 'medium', cost: 'medium', description: 'Deep reasoning' },
      { id: 'gpt-5.2-instant', name: 'GPT-5.2 Instant', speed: 'fast', cost: 'low', description: 'Fast everyday' },
      { id: 'gpt-5.2-codex', name: 'GPT-5.2 Codex', speed: 'medium', cost: 'high', description: 'Agentic coding' },
    ],
  },
  {
    id: 'gemini',
    name: 'Gemini 3 (Google)',
    description: '90.4% GPQA Diamond',
    color: 'text-blue-400',
    bgColor: 'bg-blue-500/10',
    bgGlow: 'shadow-blue-500/20',
    borderColor: 'border-blue-500/30',
    activeBorder: 'border-blue-500',
    placeholder: 'AIza...',
    models: [
      { id: 'gemini-3-pro', name: 'Gemini 3 Pro', speed: 'medium', cost: 'medium', description: 'Advanced' },
      { id: 'gemini-3-deep-think', name: 'Gemini 3 Deep Think', speed: 'slow', cost: 'high', description: 'Extended thinking' },
      { id: 'gemini-3-flash', name: 'Gemini 3 Flash', speed: 'fast', cost: 'low', description: 'Pro at Flash speed' },
    ],
  },
  {
    id: 'grok',
    name: 'Grok 4 (xAI)',
    description: '50% HLE, #1 LMArena',
    color: 'text-purple-400',
    bgColor: 'bg-purple-500/10',
    bgGlow: 'shadow-purple-500/20',
    borderColor: 'border-purple-500/30',
    activeBorder: 'border-purple-500',
    placeholder: 'xai-...',
    models: [
      { id: 'grok-4-heavy', name: 'Grok 4 Heavy', speed: 'slow', cost: 'high', description: 'Most powerful' },
      { id: 'grok-4.1-thinking', name: 'Grok 4.1 Thinking', speed: 'medium', cost: 'medium', description: '#1 LMArena' },
      { id: 'grok-4.1', name: 'Grok 4.1', speed: 'fast', cost: 'medium', description: '#2 non-reasoning' },
      { id: 'grok-4-fast', name: 'Grok 4 Fast', speed: 'fast', cost: 'low', description: 'Budget option' },
    ],
  },
  {
    id: 'ollama',
    name: 'Ollama (Local)',
    description: 'Free, private, local',
    color: 'text-cyan-400',
    bgColor: 'bg-cyan-500/10',
    bgGlow: 'shadow-cyan-500/20',
    borderColor: 'border-cyan-500/30',
    activeBorder: 'border-cyan-500',
    placeholder: 'http://localhost:11434',
    models: [
      { id: 'deepseek-r1', name: 'DeepSeek-R1', speed: 'medium', cost: 'free', description: 'O3-tier reasoning' },
      { id: 'deepseek-v3.2', name: 'DeepSeek-V3.2', speed: 'fast', cost: 'free', description: 'Efficient' },
      { id: 'llama3.3-70b', name: 'Llama 3.3 70B', speed: 'medium', cost: 'free', description: '405B performance' },
      { id: 'codellama', name: 'Code Llama', speed: 'fast', cost: 'free', description: 'Code specialist' },
    ],
  },
]

interface KeyState {
  provider: string
  isConfigured: boolean
  isValid: boolean
  isActive: boolean
  lastUsed?: string
  usageCount: number
  totalCost: number
  selectedModel: string
}

const SPEED_ICONS = {
  fast: <Zap className="w-3 h-3 text-yellow-400" />,
  medium: <Brain className="w-3 h-3 text-blue-400" />,
  slow: <Sparkles className="w-3 h-3 text-purple-400" />,
}

const COST_COLORS = {
  free: 'text-cyan-400',
  low: 'text-green-400',
  medium: 'text-yellow-400',
  high: 'text-red-400',
}

export default function APIKeySettings() {
  const [keys, setKeys] = useState<Record<string, KeyState>>({})
  const [inputValues, setInputValues] = useState<Record<string, string>>({})
  const [showKey, setShowKey] = useState<Record<string, boolean>>({})
  const [validating, setValidating] = useState<Record<string, boolean>>({})
  const [saving, setSaving] = useState<Record<string, boolean>>({})
  const [deleting, setDeleting] = useState<Record<string, boolean>>({})
  const [loading, setLoading] = useState(true)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [successes, setSuccesses] = useState<Record<string, string>>({})
  const [openDropdown, setOpenDropdown] = useState<string | null>(null)

  const fetchKeys = useCallback(async () => {
    try {
      const response = await apiService.getAPIKeys()
      if (response.success) {
        const keyMap: Record<string, KeyState> = {}
        for (const k of response.data) {
          keyMap[k.provider] = {
            provider: k.provider,
            isConfigured: true,
            isValid: k.is_valid,
            isActive: k.is_active ?? true,
            lastUsed: k.last_used,
            usageCount: k.usage_count,
            totalCost: k.total_cost,
            selectedModel: k.model_preference || PROVIDERS.find(p => p.id === k.provider)?.models[0]?.id || '',
          }
        }
        setKeys(keyMap)
      }
    } catch {
      // No keys configured yet
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchKeys()
  }, [fetchKeys])

  const handleToggleActive = async (provider: string) => {
    const current = keys[provider]
    if (!current?.isConfigured) return

    const newActive = !current.isActive
    setKeys(prev => ({
      ...prev,
      [provider]: { ...prev[provider], isActive: newActive }
    }))

    try {
      await apiService.updateAPIKeySettings(provider, { is_active: newActive })
    } catch {
      // Revert on error
      setKeys(prev => ({
        ...prev,
        [provider]: { ...prev[provider], isActive: !newActive }
      }))
    }
  }

  const handleModelChange = async (provider: string, modelId: string) => {
    setKeys(prev => ({
      ...prev,
      [provider]: { ...prev[provider], selectedModel: modelId }
    }))
    setOpenDropdown(null)

    if (keys[provider]?.isConfigured) {
      try {
        await apiService.updateAPIKeySettings(provider, { model_preference: modelId })
      } catch {
        // Silently fail
      }
    }
  }

  const handleSaveKey = async (provider: string) => {
    const value = inputValues[provider]
    if (!value?.trim()) {
      setErrors(prev => ({ ...prev, [provider]: provider === 'ollama' ? 'Server URL is required' : 'API key is required' }))
      return
    }

    setSaving(prev => ({ ...prev, [provider]: true }))
    setErrors(prev => ({ ...prev, [provider]: '' }))
    setSuccesses(prev => ({ ...prev, [provider]: '' }))

    try {
      const defaultModel = PROVIDERS.find(p => p.id === provider)?.models[0]?.id || ''
      await apiService.saveAPIKey(provider, value.trim(), { model_preference: keys[provider]?.selectedModel || defaultModel })
      setSuccesses(prev => ({ ...prev, [provider]: 'Saved & activated' }))
      setInputValues(prev => ({ ...prev, [provider]: '' }))
      await fetchKeys()

      setTimeout(() => {
        setSuccesses(prev => ({ ...prev, [provider]: '' }))
      }, 3000)
    } catch (err: any) {
      setErrors(prev => ({
        ...prev,
        [provider]: err.response?.data?.error || 'Failed to save key',
      }))
    } finally {
      setSaving(prev => ({ ...prev, [provider]: false }))
    }
  }

  const handleValidateKey = async (provider: string) => {
    setValidating(prev => ({ ...prev, [provider]: true }))
    setErrors(prev => ({ ...prev, [provider]: '' }))

    try {
      const result = await apiService.validateAPIKey(provider)
      if (result.valid) {
        setSuccesses(prev => ({ ...prev, [provider]: 'Key validated' }))
        setTimeout(() => setSuccesses(prev => ({ ...prev, [provider]: '' })), 3000)
      } else {
        setErrors(prev => ({
          ...prev,
          [provider]: result.error_detail || 'Validation failed',
        }))
      }
      await fetchKeys()
    } catch (err: any) {
      setErrors(prev => ({
        ...prev,
        [provider]: err.response?.data?.error || 'Validation failed',
      }))
    } finally {
      setValidating(prev => ({ ...prev, [provider]: false }))
    }
  }

  const handleDeleteKey = async (provider: string) => {
    setDeleting(prev => ({ ...prev, [provider]: true }))
    try {
      await apiService.deleteAPIKey(provider)
      setKeys(prev => {
        const copy = { ...prev }
        delete copy[provider]
        return copy
      })
      setSuccesses(prev => ({ ...prev, [provider]: 'Removed' }))
      setTimeout(() => setSuccesses(prev => ({ ...prev, [provider]: '' })), 3000)
    } catch (err: any) {
      setErrors(prev => ({
        ...prev,
        [provider]: err.response?.data?.error || 'Failed to delete',
      }))
    } finally {
      setDeleting(prev => ({ ...prev, [provider]: false }))
    }
  }

  const activeCount = Object.values(keys).filter(k => k.isConfigured && k.isActive).length

  if (loading) {
    return (
      <div className="flex items-center justify-center p-12">
        <Loader2 className="w-6 h-6 text-red-500 animate-spin" />
        <span className="ml-3 text-gray-400">Loading API keys...</span>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-red-500/10 border border-red-500/30">
            <Key className="w-5 h-5 text-red-400" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-white">Multi-Provider AI Configuration</h2>
            <p className="text-sm text-gray-400">
              Enable multiple providers to work together on complex builds
            </p>
          </div>
        </div>
        {activeCount > 0 && (
          <div className="px-3 py-1.5 rounded-full bg-green-500/10 border border-green-500/30 text-green-400 text-sm font-medium">
            {activeCount} Active
          </div>
        )}
      </div>

      {/* Info banner */}
      <div className="flex items-start gap-3 p-4 rounded-lg bg-gradient-to-r from-red-500/5 to-purple-500/5 border border-red-500/20">
        <Shield className="w-5 h-5 text-red-400 mt-0.5 shrink-0" />
        <div className="text-sm text-gray-300">
          <strong className="text-red-400">Multi-Provider Builds:</strong> Enable multiple AI providers
          simultaneously. Each can spawn specialized agents — use Claude for architecture, GPT for code,
          Gemini for docs, and Grok for testing all at once. Keys are encrypted with AES-256-GCM.
        </div>
      </div>

      {/* Provider Cards */}
      <div className="space-y-4">
        {PROVIDERS.map((provider) => {
          const keyState = keys[provider.id]
          const isConfigured = !!keyState?.isConfigured
          const isActive = keyState?.isActive ?? false
          const isValid = keyState?.isValid ?? false
          const selectedModel = keyState?.selectedModel || provider.models[0]?.id
          const selectedModelInfo = provider.models.find(m => m.id === selectedModel) || provider.models[0]

          return (
            <div
              key={provider.id}
              className={cn(
                'relative rounded-xl overflow-hidden transition-all duration-300',
                'bg-gray-900/70 backdrop-blur-xl border-2',
                isConfigured && isActive
                  ? `${provider.activeBorder} shadow-lg ${provider.bgGlow}`
                  : isConfigured
                    ? 'border-gray-600/50'
                    : 'border-gray-700/30 hover:border-gray-600/50'
              )}
            >
              <div className="p-5">
                {/* Provider header with toggle */}
                <div className="flex items-center justify-between mb-4">
                  <div className="flex items-center gap-3">
                    {/* Active toggle */}
                    <button
                      onClick={() => isConfigured && handleToggleActive(provider.id)}
                      disabled={!isConfigured}
                      className={cn(
                        'relative w-12 h-6 rounded-full transition-all duration-300',
                        isConfigured && isActive
                          ? provider.bgColor + ' ' + provider.borderColor
                          : 'bg-gray-800 border border-gray-700',
                        !isConfigured && 'opacity-40 cursor-not-allowed'
                      )}
                      title={isConfigured ? (isActive ? 'Click to deactivate' : 'Click to activate') : 'Add API key first'}
                    >
                      <div className={cn(
                        'absolute top-0.5 w-5 h-5 rounded-full transition-all duration-300 flex items-center justify-center',
                        isConfigured && isActive
                          ? 'left-6 bg-white shadow-lg'
                          : 'left-0.5 bg-gray-600'
                      )}>
                        <Power className={cn('w-3 h-3', isActive ? provider.color : 'text-gray-400')} />
                      </div>
                    </button>

                    <div>
                      <span className={cn('font-semibold', isActive ? provider.color : 'text-gray-400')}>
                        {provider.name}
                      </span>
                      <p className="text-xs text-gray-500">{provider.description}</p>
                    </div>
                  </div>

                  {/* Status badges */}
                  <div className="flex items-center gap-2">
                    {isConfigured && isValid && (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-500/10 text-green-400 border border-green-500/30">
                        <CheckCircle className="w-3 h-3" /> Valid
                      </span>
                    )}
                    {isConfigured && !isValid && (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-yellow-500/10 text-yellow-400 border border-yellow-500/30">
                        <XCircle className="w-3 h-3" /> Check key
                      </span>
                    )}
                    {!isConfigured && (
                      <span className="text-xs text-gray-500">Platform key fallback</span>
                    )}
                  </div>
                </div>

                {/* Key input + Model selector row */}
                <div className="space-y-3">
                  {/* API Key input */}
                  {!isConfigured ? (
                    <div className="flex gap-2">
                      <div className="relative flex-1">
                        <input
                          type={showKey[provider.id] ? 'text' : 'password'}
                          value={inputValues[provider.id] || ''}
                          onChange={(e) =>
                            setInputValues(prev => ({ ...prev, [provider.id]: e.target.value }))
                          }
                          placeholder={provider.placeholder}
                          className={cn(
                            'w-full h-10 px-4 pr-10 rounded-lg border bg-gray-800/50 text-white text-sm',
                            'placeholder:text-gray-600 focus:outline-none focus:ring-1 transition-all',
                            'border-gray-700 focus:border-red-500 focus:ring-red-500/30'
                          )}
                        />
                        <button
                          type="button"
                          onClick={() => setShowKey(prev => ({ ...prev, [provider.id]: !prev[provider.id] }))}
                          className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300"
                        >
                          {showKey[provider.id] ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                        </button>
                      </div>
                      <button
                        onClick={() => handleSaveKey(provider.id)}
                        disabled={saving[provider.id] || !inputValues[provider.id]?.trim()}
                        className="px-4 h-10 rounded-lg bg-red-600 hover:bg-red-500 text-white text-sm font-medium disabled:opacity-40 disabled:cursor-not-allowed transition-all flex items-center gap-2"
                      >
                        {saving[provider.id] ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Save & Activate'}
                      </button>
                    </div>
                  ) : (
                    <div className="flex items-center gap-2">
                      <div className="flex-1 h-10 px-4 rounded-lg border border-gray-700/50 bg-gray-800/30 flex items-center text-sm text-gray-500 font-mono">
                        {'•'.repeat(24)}
                      </div>
                      <button
                        onClick={() => handleValidateKey(provider.id)}
                        disabled={validating[provider.id]}
                        className="px-3 h-10 rounded-lg border border-gray-700 hover:border-green-500/50 text-gray-400 hover:text-green-400 text-sm transition-all flex items-center gap-1.5"
                      >
                        {validating[provider.id] ? <Loader2 className="w-4 h-4 animate-spin" /> : <RefreshCw className="w-4 h-4" />}
                      </button>
                      <button
                        onClick={() => handleDeleteKey(provider.id)}
                        disabled={deleting[provider.id]}
                        className="px-3 h-10 rounded-lg border border-gray-700 hover:border-red-500/50 text-gray-400 hover:text-red-400 text-sm transition-all flex items-center gap-1.5"
                      >
                        {deleting[provider.id] ? <Loader2 className="w-4 h-4 animate-spin" /> : <Trash2 className="w-4 h-4" />}
                      </button>
                    </div>
                  )}

                  {/* Model selector dropdown */}
                  <div className="relative">
                    <button
                      onClick={() => setOpenDropdown(openDropdown === provider.id ? null : provider.id)}
                      className={cn(
                        'w-full h-10 px-4 rounded-lg border bg-gray-800/50 text-sm transition-all',
                        'flex items-center justify-between',
                        openDropdown === provider.id
                          ? `${provider.borderColor} ring-1 ring-${provider.color.replace('text-', '')}/30`
                          : 'border-gray-700 hover:border-gray-600'
                      )}
                    >
                      <div className="flex items-center gap-3">
                        <span className="text-gray-400 text-xs">Model:</span>
                        <span className={cn('font-medium', provider.color)}>{selectedModelInfo?.name}</span>
                        <span className="flex items-center gap-1">
                          {SPEED_ICONS[selectedModelInfo?.speed || 'medium']}
                          <span className={cn('text-xs', COST_COLORS[selectedModelInfo?.cost || 'medium'])}>
                            {selectedModelInfo?.cost === 'free' ? 'Free' : selectedModelInfo?.cost === 'low' ? '$' : selectedModelInfo?.cost === 'medium' ? '$$' : '$$$'}
                          </span>
                        </span>
                      </div>
                      <ChevronDown className={cn('w-4 h-4 text-gray-500 transition-transform', openDropdown === provider.id && 'rotate-180')} />
                    </button>

                    {/* Dropdown menu */}
                    {openDropdown === provider.id && (
                      <div className="absolute top-full left-0 right-0 mt-1 z-50 bg-gray-900/95 backdrop-blur-xl border border-gray-700 rounded-lg shadow-2xl overflow-hidden">
                        {provider.models.map((model) => (
                          <button
                            key={model.id}
                            onClick={() => handleModelChange(provider.id, model.id)}
                            className={cn(
                              'w-full px-4 py-2.5 flex items-center justify-between text-left transition-colors',
                              'hover:bg-gray-800/50',
                              selectedModel === model.id && 'bg-gray-800/70'
                            )}
                          >
                            <div>
                              <div className={cn('text-sm font-medium', selectedModel === model.id ? provider.color : 'text-white')}>
                                {model.name}
                              </div>
                              <div className="text-xs text-gray-500">{model.description}</div>
                            </div>
                            <div className="flex items-center gap-2">
                              {SPEED_ICONS[model.speed]}
                              <span className={cn('text-xs font-mono', COST_COLORS[model.cost])}>
                                {model.cost === 'free' ? 'Free' : model.cost === 'low' ? '$' : model.cost === 'medium' ? '$$' : '$$$'}
                              </span>
                              {selectedModel === model.id && <CheckCircle className={cn('w-4 h-4', provider.color)} />}
                            </div>
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                </div>

                {/* Usage stats */}
                {isConfigured && keyState && (
                  <div className="flex gap-6 mt-3 pt-3 border-t border-gray-800/50 text-xs text-gray-500">
                    <span>Requests: <span className="text-gray-300">{keyState.usageCount}</span></span>
                    <span>Cost: <span className="text-gray-300">${keyState.totalCost.toFixed(4)}</span></span>
                    {keyState.lastUsed && (
                      <span>Last: <span className="text-gray-300">{new Date(keyState.lastUsed).toLocaleDateString()}</span></span>
                    )}
                  </div>
                )}

                {/* Messages */}
                {errors[provider.id] && (
                  <p className="mt-2 text-xs text-red-400 flex items-center gap-1">
                    <XCircle className="w-3 h-3" /> {errors[provider.id]}
                  </p>
                )}
                {successes[provider.id] && (
                  <p className="mt-2 text-xs text-green-400 flex items-center gap-1">
                    <CheckCircle className="w-3 h-3" /> {successes[provider.id]}
                  </p>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
