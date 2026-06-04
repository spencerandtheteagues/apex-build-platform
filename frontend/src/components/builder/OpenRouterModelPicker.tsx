import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { cn } from '@/lib/utils'
import apiService from '@/services/api'
import { Search, X, Zap, DollarSign, Filter } from 'lucide-react'

interface LiveModel {
  id: string
  name: string
  context_window?: number
  input_per_1m?: number
  output_per_1m?: number
  is_free?: boolean
  quality_code?: string
  quality_reason?: string
  speed_rating?: number
  tier?: string
  tags?: string[]
  multimodal?: boolean
}

interface OpenRouterModelPickerProps {
  isOpen: boolean
  onClose: () => void
  onSelect: (modelId: string, modelName: string) => void
  title?: string
}

const QUALITY_ORDER: Record<string, number> = { A: 5, B: 4, C: 3, D: 2, F: 1 }
const TIER_COLORS: Record<string, string> = {
  elite: 'text-yellow-300 bg-yellow-500/10 border-yellow-500/30',
  pro: 'text-violet-300 bg-violet-500/10 border-violet-500/30',
  balanced: 'text-cyan-300 bg-cyan-500/10 border-cyan-500/30',
  fast: 'text-emerald-300 bg-emerald-500/10 border-emerald-500/30',
  free: 'text-green-300 bg-green-500/10 border-green-500/30',
}
const QUALITY_COLORS: Record<string, string> = {
  A: 'text-green-300',
  B: 'text-emerald-300',
  C: 'text-yellow-300',
  D: 'text-orange-300',
  F: 'text-red-400',
}

const formatCost = (per1m?: number): string => {
  if (per1m === undefined || per1m === null) return '—'
  if (per1m === 0) return 'FREE'
  if (per1m < 0.01) return `$${(per1m * 1000).toFixed(3)}/1B`
  if (per1m < 1) return `$${per1m.toFixed(3)}/1M`
  return `$${per1m.toFixed(2)}/1M`
}

export default function OpenRouterModelPicker({ isOpen, onClose, onSelect, title }: OpenRouterModelPickerProps) {
  const [models, setModels] = useState<LiveModel[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [freeOnly, setFreeOnly] = useState(false)
  const [tierFilter, setTierFilter] = useState<string>('all')
  const [sortBy, setSortBy] = useState<'quality' | 'cost' | 'speed' | 'name'>('quality')

  useEffect(() => {
    if (!isOpen) return
    setLoading(true)
    setError(null)
    apiService.client
      .get<{ success: boolean; data: { models: LiveModel[] } }>('/ai/openrouter/models')
      .then((res) => {
        setModels(res.data?.data?.models ?? [])
      })
      .catch(() => setError('Failed to load models. Check your OpenRouter API key.'))
      .finally(() => setLoading(false))
  }, [isOpen])

  const filtered = useMemo(() => {
    let list = models
    if (freeOnly) list = list.filter((m) => m.is_free)
    if (tierFilter !== 'all') list = list.filter((m) => m.tier === tierFilter)
    if (search.trim()) {
      const q = search.toLowerCase()
      list = list.filter(
        (m) =>
          m.id.toLowerCase().includes(q) ||
          m.name.toLowerCase().includes(q) ||
          m.tags?.some((t) => t.toLowerCase().includes(q))
      )
    }
    return [...list].sort((a, b) => {
      if (sortBy === 'quality') {
        const qa = QUALITY_ORDER[a.quality_code ?? ''] ?? 0
        const qb = QUALITY_ORDER[b.quality_code ?? ''] ?? 0
        return qb - qa
      }
      if (sortBy === 'cost') return (a.input_per_1m ?? 999) - (b.input_per_1m ?? 999)
      if (sortBy === 'speed') return (b.speed_rating ?? 0) - (a.speed_rating ?? 0)
      return a.name.localeCompare(b.name)
    })
  }, [models, freeOnly, tierFilter, search, sortBy])

  const handleKey = useCallback(
    (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() },
    [onClose]
  )
  useEffect(() => {
    if (!isOpen) return
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [isOpen, handleKey])

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/70 backdrop-blur-sm" onClick={onClose} />
      <div className="relative w-full max-w-3xl max-h-[85vh] flex flex-col rounded-2xl border border-violet-500/30 bg-gray-950 shadow-2xl shadow-violet-500/10">
        {/* Header */}
        <div className="flex items-center gap-3 p-5 border-b border-gray-800/60">
          <div className="flex-1">
            <h2 className="text-lg font-bold text-white">{title ?? 'OpenRouter Model Picker'}</h2>
            <p className="text-xs text-gray-500 mt-0.5">
              {loading ? 'Loading models…' : `${filtered.length} of ${models.length} models`}
            </p>
          </div>
          <button onClick={onClose} className="p-2 rounded-lg text-gray-500 hover:text-white hover:bg-gray-800 transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Filters */}
        <div className="flex flex-wrap items-center gap-2 px-5 py-3 border-b border-gray-800/40">
          <div className="flex-1 min-w-48 relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
            <input
              type="text"
              placeholder="Search models…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full pl-9 pr-3 py-2 rounded-lg bg-gray-900 border border-gray-700 text-sm text-gray-100 placeholder-gray-600 focus:border-violet-500/60 focus:outline-none"
              autoFocus
            />
          </div>
          <button
            onClick={() => setFreeOnly(!freeOnly)}
            className={cn(
              'flex items-center gap-1.5 px-3 py-2 rounded-lg border text-xs font-semibold transition-colors',
              freeOnly ? 'border-green-500/60 bg-green-500/15 text-green-300' : 'border-gray-700 bg-gray-900 text-gray-500 hover:border-gray-600 hover:text-gray-300'
            )}
          >
            <DollarSign className="w-3.5 h-3.5" />Free only
          </button>
          <div className="flex items-center gap-1">
            <Filter className="w-3.5 h-3.5 text-gray-600" />
            <select
              value={tierFilter}
              onChange={(e) => setTierFilter(e.target.value)}
              className="bg-gray-900 border border-gray-700 rounded-lg px-2 py-2 text-xs text-gray-300 focus:outline-none focus:border-violet-500/60"
            >
              <option value="all">All tiers</option>
              <option value="elite">Elite</option>
              <option value="pro">Pro</option>
              <option value="balanced">Balanced</option>
              <option value="fast">Fast</option>
              <option value="free">Free</option>
            </select>
          </div>
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as typeof sortBy)}
            className="bg-gray-900 border border-gray-700 rounded-lg px-2 py-2 text-xs text-gray-300 focus:outline-none focus:border-violet-500/60"
          >
            <option value="quality">Sort: Quality</option>
            <option value="cost">Sort: Cost</option>
            <option value="speed">Sort: Speed</option>
            <option value="name">Sort: Name</option>
          </select>
        </div>

        {/* Model list */}
        <div className="flex-1 overflow-y-auto min-h-0">
          {loading && <div className="flex items-center justify-center py-16 text-gray-500 text-sm">Loading models from OpenRouter…</div>}
          {error && <div className="flex items-center justify-center py-16 text-red-400 text-sm">{error}</div>}
          {!loading && !error && filtered.length === 0 && <div className="flex items-center justify-center py-16 text-gray-600 text-sm">No models match your filters.</div>}
          {!loading && !error && filtered.length > 0 && (
            <div className="divide-y divide-gray-800/40">
              {filtered.map((model) => (
                <button
                  key={model.id}
                  onClick={() => { onSelect(model.id, model.name); onClose() }}
                  className="w-full flex items-center gap-3 px-5 py-3 text-left hover:bg-violet-500/5 transition-colors group"
                >
                  <div className={cn(
                    'shrink-0 w-7 h-7 rounded-lg flex items-center justify-center font-bold text-sm border',
                    model.quality_code ? `${QUALITY_COLORS[model.quality_code]} border-current/30 bg-current/5` : 'text-gray-600 border-gray-700'
                  )}>
                    {model.quality_code ?? '?'}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-semibold text-sm text-gray-100 group-hover:text-white truncate">{model.name}</span>
                      {model.is_free && <span className="shrink-0 text-[10px] font-bold px-1.5 py-0.5 rounded-full border border-green-500/40 bg-green-500/10 text-green-300">FREE</span>}
                      {model.tier && <span className={cn('shrink-0 text-[10px] font-semibold px-1.5 py-0.5 rounded border capitalize', TIER_COLORS[model.tier] ?? 'text-gray-500 border-gray-700')}>{model.tier}</span>}
                    </div>
                    <div className="text-[11px] text-gray-600 truncate mt-0.5">{model.id}</div>
                    {model.quality_reason && <div className="text-[10px] text-gray-700 truncate">{model.quality_reason}</div>}
                  </div>
                  <div className="shrink-0 flex flex-col items-end gap-1 text-[11px]">
                    <div className="flex items-center gap-1 text-gray-500"><DollarSign className="w-3 h-3" /><span>{formatCost(model.input_per_1m)}</span></div>
                    {(model.speed_rating ?? 0) > 0 && <div className="flex items-center gap-1 text-gray-600"><Zap className="w-3 h-3" /><span>{model.speed_rating}/10</span></div>}
                    {model.context_window && <div className="text-gray-700">{model.context_window >= 1000000 ? `${(model.context_window / 1000000).toFixed(0)}M ctx` : `${(model.context_window / 1000).toFixed(0)}K ctx`}</div>}
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>

        <div className="px-5 py-3 border-t border-gray-800/40 text-[11px] text-gray-700">
          Powered by OpenRouter — {models.length} models available · Press Esc to close
        </div>
      </div>
    </div>
  )
}
