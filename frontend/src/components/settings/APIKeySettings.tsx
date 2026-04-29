// APEX-BUILD API Key Settings
// Multi-provider BYOK with active toggles and per-provider model selectors

import React, { useState, useEffect, useCallback } from 'react'
import { Key, Shield, CheckCircle, XCircle, Trash2, Loader2, Eye, EyeOff, RefreshCw, Power, ChevronDown, Zap, Brain, Sparkles } from 'lucide-react'
import { cn } from '@/lib/utils'
import apiService from '@/services/api'
import type { AIProvider } from '@/types'
import { useUser } from '@/hooks/useStore'

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
    description: 'Flagship reasoning + coding models',
    color: 'text-orange-400',
    bgColor: 'bg-orange-500/10',
    bgGlow: 'shadow-orange-500/20',
    borderColor: 'border-orange-500/30',
    activeBorder: 'border-orange-500',
    placeholder: 'sk-ant-...',
    models: [
      { id: 'claude-opus-4-7', name: 'Claude Opus 4.7', speed: 'slow', cost: 'high', description: 'Most powerful — reasoning & coding' },
      { id: 'claude-sonnet-4-6', name: 'Claude Sonnet 4.6', speed: 'medium', cost: 'medium', description: 'Balanced quality and speed' },
      { id: 'claude-haiku-4-5-20251001', name: 'Claude Haiku 4.5', speed: 'fast', cost: 'low', description: 'Fast mini tier' },
    ],
  },
  {
    id: 'gpt4',
    name: 'OpenAI',
    description: 'Latest OpenAI build tiers',
    color: 'text-green-400',
    bgColor: 'bg-green-500/10',
    bgGlow: 'shadow-green-500/20',
    borderColor: 'border-green-500/30',
    activeBorder: 'border-green-500',
    placeholder: 'sk-...',
    models: [
      { id: 'gpt-5.4-codex', name: 'ChatGPT 5.4 Codex', speed: 'slow', cost: 'high', description: 'Max tier - current OpenAI Codex route' },
      { id: 'gpt-4.1', name: 'GPT-4.1', speed: 'medium', cost: 'medium', description: 'Balanced tier — best current mid-tier coder' },
      { id: 'gpt-4o-mini', name: 'GPT-4o Mini', speed: 'fast', cost: 'low', description: 'Fast mini tier' },
    ],
  },
  {
    id: 'gemini',
    name: 'Gemini (Google)',
    description: 'Latest Gemini preview tiers',
    color: 'text-blue-400',
    bgColor: 'bg-blue-500/10',
    bgGlow: 'shadow-blue-500/20',
    borderColor: 'border-blue-500/30',
    activeBorder: 'border-blue-500',
    placeholder: 'AIza...',
    models: [
      { id: 'gemini-3.1-pro', name: 'Gemini 3.1 Pro', speed: 'slow', cost: 'high', description: 'Max tier — Pro first' },
      { id: 'gemini-3.1-pro-preview', name: 'Gemini 3.1 Pro Preview', speed: 'slow', cost: 'high', description: 'Max fallback — preview tier' },
      { id: 'gemini-3-flash-preview', name: 'Gemini 3 Flash Preview', speed: 'medium', cost: 'medium', description: 'Balanced tier — fast reasoning' },
      { id: 'gemini-2.5-flash-lite', name: 'Gemini 2.5 Flash Lite', speed: 'fast', cost: 'low', description: 'Fast mini tier' },
    ],
  },
  {
    id: 'grok',
    name: 'Grok (xAI)',
    description: 'Current xAI coding tiers',
    color: 'text-purple-400',
    bgColor: 'bg-purple-500/10',
    bgGlow: 'shadow-purple-500/20',
    borderColor: 'border-purple-500/30',
    activeBorder: 'border-purple-500',
    placeholder: 'xai-...',
    models: [
      { id: 'grok-4.20-0309-reasoning', name: 'Grok 4.20', speed: 'medium', cost: 'high', description: 'Max tier — frontier reasoning model' },
      { id: 'grok-3', name: 'Grok 3', speed: 'medium', cost: 'medium', description: 'Balanced tier — stronger reasoning' },
      { id: 'grok-3-mini', name: 'Grok 3 Mini', speed: 'fast', cost: 'low', description: 'Fast mini tier' },
    ],
  },
  {
    id: 'ollama',
    name: 'Ollama (Local / Cloud)',
    description: 'Local inference or Ollama Cloud BYOK (API key + optional base URL)',
    color: 'text-cyan-400',
    bgColor: 'bg-cyan-500/10',
    bgGlow: 'shadow-cyan-500/20',
    borderColor: 'border-cyan-500/30',
    activeBorder: 'border-cyan-500',
    placeholder: 'Ollama API key, http://localhost:11434, or key / OLLAMA_BASE_URL:https://ollama.com',
    models: [
      { id: 'kimi-k2.6', name: 'Kimi K2.6', speed: 'fast', cost: 'low', description: 'Default conductor/cloud model' },
      { id: 'glm-5.1', name: 'GLM-5.1', speed: 'fast', cost: 'low', description: 'Fast open-model coding route' },
      { id: 'qwen3.5', name: 'Qwen 3.5', speed: 'fast', cost: 'low', description: 'Latest Qwen cloud large route' },
      { id: 'devstral-small-24b', name: 'Devstral Small 24B', speed: 'medium', cost: 'low', description: 'Agentic coding model' },
      { id: 'deepseek-v4-flash', name: 'DeepSeek V4 Flash', speed: 'fast', cost: 'low', description: 'Budget reasoning/coding route' },
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

const asNumber = (value: unknown): number => {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

const normalizeKeyStates = (value: unknown): Record<string, KeyState> => {
  if (!Array.isArray(value)) {
    return {}
  }

  const next: Record<string, KeyState> = {}
  for (const candidate of value) {
    if (!candidate || typeof candidate !== 'object') continue

    const record = candidate as Record<string, unknown>
    const provider = typeof record.provider === 'string' ? record.provider : ''
    if (!provider) continue

    const defaultModel = PROVIDERS.find((entry) => entry.id === provider)?.models[0]?.id || ''
    next[provider] = {
      provider,
      isConfigured: true,
      isValid: Boolean(record.is_valid),
      isActive: typeof record.is_active === 'boolean' ? record.is_active : true,
      lastUsed: typeof record.last_used === 'string' ? record.last_used : undefined,
      usageCount: asNumber(record.usage_count),
      totalCost: asNumber(record.total_cost),
      selectedModel: typeof record.model_preference === 'string' && record.model_preference
        ? record.model_preference
        : defaultModel,
    }
  }

  return next
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

function isLocalOllamaUrl(value: string): boolean {
  const trimmed = value.trim()
  if (!trimmed) return false

  const parseCandidate = /^[a-z]+:\/\//i.test(trimmed) ? trimmed : `http://${trimmed}`
  try {
    const parsed = new URL(parseCandidate)
    const host = parsed.hostname.toLowerCase()
    return host === 'localhost' || host === '127.0.0.1' || host === '0.0.0.0' || host === '::1'
  } catch {
    return false
  }
}

export default function APIKeySettings() {
  const user = useUser()
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
  const canUseBYOK = user != null && ['builder', 'pro', 'team', 'enterprise', 'owner'].includes(user.subscription_type)

  const fetchKeys = useCallback(async () => {
    if (!canUseBYOK) {
      setKeys({})
      setLoading(false)
      return
    }

    try {
      const response = await apiService.getAPIKeys()
      setKeys(response.success ? normalizeKeyStates(response.data) : {})
    } catch {
      setKeys({})
    } finally {
      setLoading(false)
    }
  }, [canUseBYOK])

  useEffect(() => {
    fetchKeys()
  }, [fetchKeys])

  if (!canUseBYOK) {
    return (
      <div className="rounded-2xl border border-amber-500/30 bg-amber-500/10 p-5">
        <div className="flex items-start gap-3">
          <Shield className="mt-0.5 h-5 w-5 text-amber-300" />
          <div>
            <h3 className="text-sm font-semibold text-white">BYOK is a paid-plan feature</h3>
            <p className="mt-1 text-sm text-gray-300">
              Bring Your Own Key is available on Builder and higher. Free accounts can build static frontend websites, but connecting personal provider keys requires an active subscription.
            </p>
            <a
              href="/settings/billing"
              className="mt-3 inline-flex items-center rounded-lg border border-amber-400/30 px-3 py-1.5 text-sm font-medium text-amber-200 transition-colors hover:border-amber-300/50 hover:text-white"
            >
              Upgrade to unlock BYOK
            </a>
          </div>
        </div>
      </div>
    )
  }

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
      setSuccesses(prev => ({ ...prev, [provider]: newActive ? 'Activated' : 'Deactivated' }))
      setTimeout(() => setSuccesses(prev => ({ ...prev, [provider]: '' })), 2000)
    } catch (err: any) {
      // Revert on error
      setKeys(prev => ({
        ...prev,
        [provider]: { ...prev[provider], isActive: !newActive }
      }))
      setErrors(prev => ({
        ...prev,
        [provider]: err.response?.data?.error || 'Failed to update settings'
      }))
    }
  }

  const handleModelChange = async (provider: string, modelId: string) => {
    const prevModel = keys[provider]?.selectedModel
    setKeys(prev => ({
      ...prev,
      [provider]: { ...prev[provider], selectedModel: modelId }
    }))
    setOpenDropdown(null)

    if (keys[provider]?.isConfigured) {
      try {
        await apiService.updateAPIKeySettings(provider, { model_preference: modelId })
      } catch (err: any) {
        // Revert on error
        if (prevModel) {
          setKeys(prev => ({
            ...prev,
            [provider]: { ...prev[provider], selectedModel: prevModel }
          }))
        }
        setErrors(prev => ({
          ...prev,
          [provider]: err.response?.data?.error || 'Failed to update model'
        }))
      }
    }
  }

  const handleSaveKey = async (provider: string) => {
    const value = inputValues[provider]
    if (!value?.trim()) {
      setErrors(prev => ({ ...prev, [provider]: provider === 'ollama' ? 'Ollama API key or server URL is required' : 'API key is required' }))
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
          const isLocalOllama = provider.id === 'ollama' && isLocalOllamaUrl(inputValues[provider.id] || '')

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

                  {isLocalOllama && (
                    <div className="rounded-lg border border-cyan-500/30 bg-cyan-500/10 p-3 space-y-2">
                      <p className="text-sm text-cyan-100">
                        <span className="font-medium">📡 Local Ollama detected.</span>{' '}
                        Your Ollama server at localhost is only accessible from your browser. To use it with cloud builds,
                        expose it via: <code className="px-1 py-0.5 rounded bg-black/30 text-cyan-200">ngrok http 11434</code>{' '}
                        and use the public ngrok URL instead.
                      </p>
                      <details className="group">
                        <summary className="cursor-pointer text-xs text-cyan-300 hover:text-cyan-200 select-none">
                          Quick Setup
                        </summary>
                        <div className="mt-2 rounded-md border border-cyan-500/20 bg-black/20 p-3 text-xs text-cyan-100 space-y-2">
                          <p>Option A (ngrok):</p>
                          <code className="block px-2 py-1 rounded bg-black/30 text-cyan-200">ngrok http 11434</code>
                          <p>Use the generated <span className="font-mono">https://*.ngrok-free.app</span> URL in this field.</p>
                          <p>Option B (Cloudflare Tunnel):</p>
                          <code className="block px-2 py-1 rounded bg-black/30 text-cyan-200">cloudflared tunnel --url http://localhost:11434</code>
                          <p>Then paste the public <span className="font-mono">https://*.trycloudflare.com</span> URL here.</p>
                        </div>
                      </details>
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
