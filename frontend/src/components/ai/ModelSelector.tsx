// APEX.BUILD Model Selector
// Dropdown for selecting AI provider and model with speed/cost indicators

import React, { useState, useEffect, useRef, useCallback } from 'react'
import { ChevronDown, Zap, DollarSign, Brain, Sparkles, Check } from 'lucide-react'
import { cn } from '@/lib/utils'
import apiService from '@/services/api'

interface ModelInfo {
  id: string
  name: string
  speed: string
  cost_tier: string
  description: string
}

interface ModelSelectorProps {
  value?: string
  onChange?: (provider: string, model: string) => void
  compact?: boolean
  className?: string
}

const PROVIDER_META: Record<string, { label: string; color: string; icon: string }> = {
  claude: { label: 'Claude', color: 'text-orange-400', icon: 'A' },
  gpt4: { label: 'OpenAI', color: 'text-green-400', icon: 'G' },
  gemini: { label: 'Gemini', color: 'text-blue-400', icon: 'G' },
  grok: { label: 'Grok', color: 'text-purple-400', icon: 'X' },
  ollama: { label: 'Ollama (Local)', color: 'text-cyan-400', icon: 'O' },
}

const SPEED_ICONS: Record<string, React.ReactNode> = {
  fast: <Zap className="w-3 h-3 text-yellow-400" />,
  medium: <Brain className="w-3 h-3 text-blue-400" />,
  slow: <Sparkles className="w-3 h-3 text-purple-400" />,
  variable: <Brain className="w-3 h-3 text-cyan-400" />,
}

const COST_LABELS: Record<string, { label: string; color: string }> = {
  free: { label: 'Free', color: 'text-cyan-400' },
  low: { label: '$', color: 'text-green-400' },
  medium: { label: '$$', color: 'text-yellow-400' },
  high: { label: '$$$', color: 'text-red-400' },
}

export default function ModelSelector({ value, onChange, compact = false, className }: ModelSelectorProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [models, setModels] = useState<Record<string, ModelInfo[]>>({})
  const [selectedProvider, setSelectedProvider] = useState<string>('')
  const [selectedModel, setSelectedModel] = useState<string>(value || '')
  const dropdownRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const fetchModels = async () => {
      try {
        const response = await apiService.getAvailableModels()
        if (response.success) {
          setModels(response.data)
        }
      } catch {
        // Fallback models if API is unavailable
        setModels({
          claude: [
            { id: 'claude-opus-4-5-20251101', name: 'Claude Opus 4.5', speed: 'slow', cost_tier: 'high', description: 'Flagship reasoning' },
            { id: 'claude-sonnet-4-20250514', name: 'Claude Sonnet 4', speed: 'medium', cost_tier: 'medium', description: 'Balanced' },
            { id: 'claude-haiku-3-5-20241022', name: 'Claude Haiku 3.5', speed: 'fast', cost_tier: 'low', description: 'Fast and affordable' },
          ],
          gpt4: [
            { id: 'gpt-4o', name: 'GPT-4o', speed: 'medium', cost_tier: 'medium', description: 'Flagship multimodal' },
            { id: 'gpt-4o-mini', name: 'GPT-4o Mini', speed: 'fast', cost_tier: 'low', description: 'Fast and affordable' },
          ],
          gemini: [
            { id: 'gemini-2.0-flash', name: 'Gemini 2.0 Flash', speed: 'fast', cost_tier: 'low', description: 'Fast multimodal' },
            { id: 'gemini-1.5-pro', name: 'Gemini 1.5 Pro', speed: 'medium', cost_tier: 'medium', description: 'Advanced' },
          ],
          grok: [
            { id: 'grok-4', name: 'Grok 4', speed: 'medium', cost_tier: 'medium', description: 'Frontier intelligence' },
            { id: 'grok-4-fast', name: 'Grok 4 Fast', speed: 'fast', cost_tier: 'low', description: 'Fast and affordable' },
          ],
          ollama: [
            { id: 'llama3.1', name: 'Llama 3.1', speed: 'variable', cost_tier: 'free', description: 'Meta open-source (local)' },
            { id: 'codellama', name: 'Code Llama', speed: 'variable', cost_tier: 'free', description: 'Code-specialized (local)' },
            { id: 'deepseek-coder-v2', name: 'DeepSeek Coder V2', speed: 'variable', cost_tier: 'free', description: 'Top-tier code model (local)' },
            { id: 'qwen2.5-coder', name: 'Qwen 2.5 Coder', speed: 'variable', cost_tier: 'free', description: 'Alibaba code model (local)' },
            { id: 'mistral', name: 'Mistral', speed: 'variable', cost_tier: 'free', description: 'Fast general-purpose (local)' },
          ],
        })
      }
    }
    fetchModels()
  }, [])

  // Close on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const handleSelect = useCallback(
    (provider: string, modelId: string) => {
      setSelectedProvider(provider)
      setSelectedModel(modelId)
      setIsOpen(false)
      onChange?.(provider, modelId)
    },
    [onChange]
  )

  // Find the selected model info for display
  const getSelectedDisplay = () => {
    if (!selectedModel || selectedModel === 'auto') {
      return { label: 'Auto', sublabel: 'Intelligent routing', provider: '' }
    }
    for (const [provider, providerModels] of Object.entries(models)) {
      const model = providerModels.find((m) => m.id === selectedModel)
      if (model) {
        return {
          label: model.name,
          sublabel: model.description,
          provider,
        }
      }
    }
    return { label: selectedModel, sublabel: '', provider: selectedProvider }
  }

  const display = getSelectedDisplay()

  return (
    <div ref={dropdownRef} className={cn('relative', className)}>
      {/* Trigger button */}
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={cn(
          'flex items-center gap-2 rounded-lg border transition-all duration-200',
          'bg-gray-900/70 border-gray-700/50 hover:border-red-500/40 text-white',
          compact ? 'h-8 px-2.5 text-xs' : 'h-10 px-3 text-sm',
          isOpen && 'border-red-500/60 shadow-lg shadow-red-500/10'
        )}
      >
        {display.provider && PROVIDER_META[display.provider] && (
          <span
            className={cn(
              'w-5 h-5 rounded flex items-center justify-center text-xs font-bold bg-gray-800',
              PROVIDER_META[display.provider].color
            )}
          >
            {PROVIDER_META[display.provider].icon}
          </span>
        )}
        {!display.provider && (
          <Sparkles className="w-4 h-4 text-red-400" />
        )}
        <span className="truncate max-w-[120px]">{display.label}</span>
        <ChevronDown
          className={cn(
            'w-3.5 h-3.5 text-gray-500 transition-transform',
            isOpen && 'rotate-180'
          )}
        />
      </button>

      {/* Dropdown */}
      {isOpen && (
        <div
          className={cn(
            'absolute top-full left-0 mt-1.5 z-50 min-w-[300px]',
            'bg-gray-900/95 backdrop-blur-xl border border-gray-700/70 rounded-xl',
            'shadow-2xl shadow-black/50',
            'animate-in fade-in slide-in-from-top-2 duration-150'
          )}
        >
          {/* Auto option */}
          <button
            type="button"
            onClick={() => handleSelect('', 'auto')}
            className={cn(
              'w-full flex items-center gap-3 px-4 py-3 text-left transition-colors',
              'hover:bg-red-500/10 border-b border-gray-800',
              selectedModel === 'auto' && 'bg-red-500/5'
            )}
          >
            <Sparkles className="w-4 h-4 text-red-400 shrink-0" />
            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium text-white">Auto (Recommended)</div>
              <div className="text-xs text-gray-500">Intelligent routing picks the best model per task</div>
            </div>
            {(selectedModel === 'auto' || !selectedModel) && (
              <Check className="w-4 h-4 text-red-400 shrink-0" />
            )}
          </button>

          {/* Provider groups */}
          <div className="max-h-[400px] overflow-y-auto">
            {Object.entries(models).map(([provider, providerModels]) => {
              const meta = PROVIDER_META[provider]
              if (!meta) return null

              return (
                <div key={provider}>
                  {/* Provider header */}
                  <div className="px-4 py-2 text-xs font-semibold text-gray-500 uppercase tracking-wider bg-gray-800/30 border-b border-gray-800/50 flex items-center gap-2">
                    <span className={meta.color}>{meta.icon}</span>
                    {meta.label}
                  </div>
                  {/* Models */}
                  {providerModels.map((model) => (
                    <button
                      key={model.id}
                      type="button"
                      onClick={() => handleSelect(provider, model.id)}
                      className={cn(
                        'w-full flex items-center gap-3 px-4 py-2.5 text-left transition-colors',
                        'hover:bg-gray-800/50',
                        selectedModel === model.id && 'bg-red-500/5'
                      )}
                    >
                      <div className="flex-1 min-w-0">
                        <div className="text-sm text-white">{model.name}</div>
                        <div className="text-xs text-gray-500">{model.description}</div>
                      </div>
                      <div className="flex items-center gap-2 shrink-0">
                        {/* Speed indicator */}
                        <span className="flex items-center gap-0.5" title={`Speed: ${model.speed}`}>
                          {SPEED_ICONS[model.speed]}
                        </span>
                        {/* Cost indicator */}
                        <span
                          className={cn(
                            'text-xs font-mono',
                            COST_LABELS[model.cost_tier]?.color || 'text-gray-500'
                          )}
                          title={`Cost: ${model.cost_tier}`}
                        >
                          {COST_LABELS[model.cost_tier]?.label || '?'}
                        </span>
                        {/* Selected check */}
                        {selectedModel === model.id && (
                          <Check className="w-4 h-4 text-red-400" />
                        )}
                      </div>
                    </button>
                  ))}
                </div>
              )
            })}
          </div>

          {/* Footer hint */}
          <div className="px-4 py-2 text-[10px] text-gray-600 border-t border-gray-800 flex items-center gap-3">
            <span className="flex items-center gap-1"><Zap className="w-3 h-3" /> Fast</span>
            <span className="flex items-center gap-1"><Brain className="w-3 h-3" /> Balanced</span>
            <span className="flex items-center gap-1"><Sparkles className="w-3 h-3" /> Deep</span>
            <span className="ml-auto flex items-center gap-1"><DollarSign className="w-3 h-3" /> = cost tier</span>
          </div>
        </div>
      )}
    </div>
  )
}
