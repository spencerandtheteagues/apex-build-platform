// APEX.BUILD API Key Settings
// Per-provider BYOK key management with validation and usage stats

import React, { useState, useEffect, useCallback } from 'react'
import { Key, Shield, CheckCircle, XCircle, Trash2, Loader2, Eye, EyeOff, RefreshCw } from 'lucide-react'
import { cn } from '@/lib/utils'
import apiService from '@/services/api'
import type { AIProvider } from '@/types'

interface ProviderConfig {
  id: AIProvider
  name: string
  description: string
  color: string
  bgGlow: string
  borderColor: string
  placeholder: string
}

const PROVIDERS: ProviderConfig[] = [
  {
    id: 'claude',
    name: 'Claude (Anthropic)',
    description: 'Opus 4.5, Sonnet 4, Haiku 3.5 — flagship reasoning',
    color: 'text-orange-400',
    bgGlow: 'shadow-orange-500/20',
    borderColor: 'border-orange-500/30 hover:border-orange-400/60',
    placeholder: 'sk-ant-...',
  },
  {
    id: 'gpt4',
    name: 'GPT-5.2 (OpenAI)',
    description: 'Pro, Thinking, Instant, Codex — 100% AIME, 400K context',
    color: 'text-green-400',
    bgGlow: 'shadow-green-500/20',
    borderColor: 'border-green-500/30 hover:border-green-400/60',
    placeholder: 'sk-...',
  },
  {
    id: 'gemini',
    name: 'Gemini 3 (Google)',
    description: 'Pro, Deep Think, Flash — 90.4% GPQA Diamond',
    color: 'text-blue-400',
    bgGlow: 'shadow-blue-500/20',
    borderColor: 'border-blue-500/30 hover:border-blue-400/60',
    placeholder: 'AIza...',
  },
  {
    id: 'grok',
    name: 'Grok 4 (xAI)',
    description: 'Heavy, 4.1 Thinking, 4.1 — 50% HLE, #1 LMArena',
    color: 'text-purple-400',
    bgGlow: 'shadow-purple-500/20',
    borderColor: 'border-purple-500/30 hover:border-purple-400/60',
    placeholder: 'xai-...',
  },
  {
    id: 'ollama',
    name: 'Ollama (Local)',
    description: 'DeepSeek-R1, Llama 3.3, Qwen — free, private, local',
    color: 'text-cyan-400',
    bgGlow: 'shadow-cyan-500/20',
    borderColor: 'border-cyan-500/30 hover:border-cyan-400/60',
    placeholder: 'http://localhost:11434',
  },
]

interface KeyState {
  provider: string
  isConfigured: boolean
  isValid: boolean
  lastUsed?: string
  usageCount: number
  totalCost: number
  modelPreference: string
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
            lastUsed: k.last_used,
            usageCount: k.usage_count,
            totalCost: k.total_cost,
            modelPreference: k.model_preference,
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
      await apiService.saveAPIKey(provider, value.trim())
      setSuccesses(prev => ({ ...prev, [provider]: 'API key saved successfully' }))
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
        setSuccesses(prev => ({ ...prev, [provider]: 'Key is valid' }))
        setTimeout(() => setSuccesses(prev => ({ ...prev, [provider]: '' })), 3000)
      } else {
        setErrors(prev => ({
          ...prev,
          [provider]: result.error_detail || 'Key validation failed',
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
      setSuccesses(prev => ({ ...prev, [provider]: 'Key removed' }))
      setTimeout(() => setSuccesses(prev => ({ ...prev, [provider]: '' })), 3000)
    } catch (err: any) {
      setErrors(prev => ({
        ...prev,
        [provider]: err.response?.data?.error || 'Failed to delete key',
      }))
    } finally {
      setDeleting(prev => ({ ...prev, [provider]: false }))
    }
  }

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
      <div className="flex items-center gap-3 mb-6">
        <div className="p-2 rounded-lg bg-red-500/10 border border-red-500/30">
          <Key className="w-5 h-5 text-red-400" />
        </div>
        <div>
          <h2 className="text-lg font-semibold text-white">API Keys</h2>
          <p className="text-sm text-gray-400">
            Enter your own API keys to use your accounts directly. Your keys are encrypted with AES-256-GCM.
          </p>
        </div>
      </div>

      {/* Info banner */}
      <div className="flex items-start gap-3 p-4 rounded-lg bg-red-500/5 border border-red-500/20">
        <Shield className="w-5 h-5 text-red-400 mt-0.5 shrink-0" />
        <div className="text-sm text-gray-300">
          <strong className="text-red-400">Bring Your Own Key (BYOK):</strong> Use your own API keys for
          unlimited requests with no platform markup. Keys that aren't set fall back to platform-managed keys
          within your plan limits.
        </div>
      </div>

      {/* Provider Cards */}
      <div className="grid gap-4">
        {PROVIDERS.map((provider) => {
          const keyState = keys[provider.id]
          const isConfigured = !!keyState?.isConfigured
          const isValid = keyState?.isValid ?? false

          return (
            <div
              key={provider.id}
              className={cn(
                'relative rounded-xl overflow-hidden transition-all duration-300',
                'bg-gray-900/70 backdrop-blur-xl border',
                isConfigured ? provider.borderColor : 'border-gray-700/50 hover:border-gray-600/70',
                isConfigured && `shadow-lg ${provider.bgGlow}`
              )}
            >
              <div className="p-5">
                {/* Provider header */}
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-3">
                    <span className={cn('font-semibold', provider.color)}>
                      {provider.name}
                    </span>
                    {isConfigured && (
                      <span
                        className={cn(
                          'inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium',
                          isValid
                            ? 'bg-green-500/10 text-green-400 border border-green-500/30'
                            : 'bg-yellow-500/10 text-yellow-400 border border-yellow-500/30'
                        )}
                      >
                        {isValid ? (
                          <><CheckCircle className="w-3 h-3" /> Valid</>
                        ) : (
                          <><XCircle className="w-3 h-3" /> Not validated</>
                        )}
                      </span>
                    )}
                    {!isConfigured && (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-gray-700/50 text-gray-400 border border-gray-600/30">
                        Using platform key
                      </span>
                    )}
                  </div>
                  <p className="text-xs text-gray-500 hidden sm:block">{provider.description}</p>
                </div>

                {/* Key input */}
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
                        className="w-full h-10 px-4 pr-10 rounded-lg border border-gray-700 bg-gray-800/50 text-white text-sm placeholder:text-gray-600 focus:border-red-500 focus:outline-none focus:ring-1 focus:ring-red-500/30 transition-all"
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
                      {saving[provider.id] ? (
                        <Loader2 className="w-4 h-4 animate-spin" />
                      ) : (
                        'Save'
                      )}
                    </button>
                  </div>
                ) : (
                  <div className="flex items-center gap-2">
                    <div className="flex-1 h-10 px-4 rounded-lg border border-gray-700/50 bg-gray-800/30 flex items-center text-sm text-gray-500 font-mono">
                      {'*'.repeat(32)}
                    </div>
                    <button
                      onClick={() => handleValidateKey(provider.id)}
                      disabled={validating[provider.id]}
                      className="px-3 h-10 rounded-lg border border-gray-700 hover:border-green-500/50 text-gray-400 hover:text-green-400 text-sm transition-all flex items-center gap-1.5"
                      title="Validate key"
                    >
                      {validating[provider.id] ? (
                        <Loader2 className="w-4 h-4 animate-spin" />
                      ) : (
                        <RefreshCw className="w-4 h-4" />
                      )}
                      Validate
                    </button>
                    <button
                      onClick={() => handleDeleteKey(provider.id)}
                      disabled={deleting[provider.id]}
                      className="px-3 h-10 rounded-lg border border-gray-700 hover:border-red-500/50 text-gray-400 hover:text-red-400 text-sm transition-all flex items-center gap-1.5"
                      title="Remove key"
                    >
                      {deleting[provider.id] ? (
                        <Loader2 className="w-4 h-4 animate-spin" />
                      ) : (
                        <Trash2 className="w-4 h-4" />
                      )}
                      Remove
                    </button>
                  </div>
                )}

                {/* Usage stats when configured */}
                {isConfigured && keyState && (
                  <div className="flex gap-6 mt-3 text-xs text-gray-500">
                    <span>Requests: <span className="text-gray-300">{keyState.usageCount}</span></span>
                    <span>Cost: <span className="text-gray-300">${keyState.totalCost.toFixed(4)}</span></span>
                    {keyState.lastUsed && (
                      <span>Last used: <span className="text-gray-300">{new Date(keyState.lastUsed).toLocaleDateString()}</span></span>
                    )}
                  </div>
                )}

                {/* Error / Success messages */}
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
