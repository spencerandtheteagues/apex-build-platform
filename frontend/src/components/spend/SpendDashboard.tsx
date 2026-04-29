import React, { useEffect, useState, useCallback } from 'react'
import { DollarSign, Download, TrendingUp, Cpu, BarChart3 } from 'lucide-react'
import apiService from '@/services/api'

interface SpendSummary {
  daily_spend: number
  monthly_spend: number
  daily_count: number
  monthly_count: number
}

interface BreakdownItem {
  key: string
  billed_cost: number
  raw_cost: number
  input_tokens: number
  output_tokens: number
  count: number
}

interface SpendEvent {
  id: number
  created_at: string
  provider: string
  model: string
  agent_role: string
  build_id: string
  input_tokens: number
  output_tokens: number
  billed_cost: number
  is_byok: boolean
  target_file: string
}

const unwrapApiData = <T,>(payload: any): T => {
  if (payload && typeof payload === 'object' && 'data' in payload) {
    return payload.data as T
  }
  return payload as T
}

const summaryCardTone: Record<'sky' | 'cyan' | 'emerald' | 'violet', string> = {
  sky: 'text-sky-300',
  cyan: 'text-cyan-300',
  emerald: 'text-emerald-300',
  violet: 'text-violet-300',
}

const meterPalette = [
  { match: ['gpt4', 'openai', 'chatgpt'], from: '#60a5fa', via: '#38bdf8', to: '#22d3ee', text: 'text-sky-200' },
  { match: ['claude', 'anthropic'], from: '#8b5cf6', via: '#a78bfa', to: '#c084fc', text: 'text-violet-200' },
  { match: ['grok', 'xai'], from: '#f472b6', via: '#fb7185', to: '#f97316', text: 'text-rose-200' },
  { match: ['gemini', 'google'], from: '#22d3ee', via: '#2dd4bf', to: '#34d399', text: 'text-cyan-200' },
  { match: ['ollama', 'kimi', 'local'], from: '#4ade80', via: '#22c55e', to: '#84cc16', text: 'text-emerald-200' },
]

const fallbackMeterPalette = [
  { from: '#38bdf8', via: '#818cf8', to: '#c084fc', text: 'text-sky-200' },
  { from: '#2dd4bf', via: '#22d3ee', to: '#60a5fa', text: 'text-cyan-200' },
  { from: '#a78bfa', via: '#f0abfc', to: '#fb7185', text: 'text-violet-200' },
]

const meterToneForKey = (key: string, index: number) => {
  const lower = key.toLowerCase()
  return meterPalette.find((tone) => tone.match.some((needle) => lower.includes(needle)))
    ?? fallbackMeterPalette[index % fallbackMeterPalette.length]
}

const formatCurrency = (value: number) => `$${Number(value || 0).toFixed(4)}`

type SummaryCard = {
  label: string
  value: number
  icon: typeof DollarSign
  color: keyof typeof summaryCardTone
  isCurrency?: boolean
}

export const SpendDashboard: React.FC = () => {
  const [summary, setSummary] = useState<SpendSummary | null>(null)
  const [breakdown, setBreakdown] = useState<BreakdownItem[]>([])
  const [history, setHistory] = useState<SpendEvent[]>([])
  const [groupBy, setGroupBy] = useState('provider')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const loadData = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [summaryRes, breakdownRes, historyRes] = await Promise.all([
        apiService.get('/spend/summary'),
        apiService.get(`/spend/breakdown?group_by=${groupBy}`),
        apiService.get('/spend/history?limit=50'),
      ])

      const summaryData = unwrapApiData<SpendSummary>(summaryRes.data)
      const breakdownData = unwrapApiData<BreakdownItem[] | { items?: BreakdownItem[] }>(breakdownRes.data)
      const historyData = unwrapApiData<SpendEvent[] | { events?: SpendEvent[] }>(historyRes.data)

      setSummary(summaryData || null)
      setBreakdown(Array.isArray(breakdownData) ? breakdownData : (breakdownData?.items || []))
      setHistory(Array.isArray(historyData) ? historyData : (historyData?.events || []))
    } catch (err) {
      console.error('Failed to load spend data:', err)
      setError('Failed to load spend data.')
    } finally {
      setLoading(false)
    }
  }, [groupBy])

  useEffect(() => { loadData() }, [loadData])

  const handleExportCSV = async () => {
    try {
      const res = await apiService.get('/spend/export/csv', { responseType: 'blob' })
      const url = window.URL.createObjectURL(new Blob([res.data]))
      const a = document.createElement('a')
      a.href = url
      a.download = `apex-spend-${new Date().toISOString().slice(0, 10)}.csv`
      a.click()
      window.URL.revokeObjectURL(url)
    } catch (err) {
      console.error('CSV export failed:', err)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-400 animate-pulse">Loading spend data...</div>
      </div>
    )
  }

  const summaryCards: SummaryCard[] = [
    { label: "Today's Spend", value: summary?.daily_spend ?? 0, icon: DollarSign, color: 'sky' },
    { label: "Monthly Spend", value: summary?.monthly_spend ?? 0, icon: TrendingUp, color: 'cyan' },
    { label: "Today's Requests", value: summary?.daily_count ?? 0, icon: Cpu, color: 'emerald', isCurrency: false },
    { label: "Monthly Requests", value: summary?.monthly_count ?? 0, icon: BarChart3, color: 'violet', isCurrency: false },
  ]

  const maxBreakdownCost = Math.max(...breakdown.map(item => Number(item.billed_cost || 0)), 0)
  const totalBreakdownCost = breakdown.reduce((sum, item) => sum + Number(item.billed_cost || 0), 0)

  return (
    <div className="min-h-full bg-black p-6 pb-16 md:pb-6">
      <div className="max-w-6xl mx-auto space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-black text-transparent bg-clip-text bg-gradient-to-r from-sky-200 via-cyan-300 to-blue-400">
              Spend Dashboard
            </h1>
            <p className="text-gray-400 mt-1">Track your AI usage costs in real-time</p>
          </div>
          <button
            onClick={handleExportCSV}
            className="flex items-center gap-2 px-4 py-2 bg-gray-800 hover:bg-gray-700 border border-gray-700 rounded-lg text-gray-300 text-sm transition-colors"
          >
            <Download size={16} />
            Export CSV
          </button>
        </div>

        {error && (
          <div className="rounded-xl border border-red-900/50 bg-red-950/20 px-4 py-3 text-sm text-red-300">
            {error}
          </div>
        )}

        {/* Summary Cards */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          {summaryCards.map((card) => (
            <div key={card.label} className="bg-slate-950/70 border border-sky-950/80 rounded-xl p-4 shadow-[0_0_30px_rgba(14,165,233,0.06)]">
              <div className="flex items-center gap-2 text-gray-400 text-sm mb-2">
                <card.icon size={16} className={summaryCardTone[card.color]} />
                {card.label}
              </div>
              <div className="text-2xl font-bold text-white">
                {card.isCurrency === false ? card.value.toLocaleString() : formatCurrency(card.value)}
              </div>
            </div>
          ))}
        </div>

        {/* Breakdown */}
        <div className="bg-slate-950/70 border border-sky-950/80 rounded-xl p-6 shadow-[0_0_40px_rgba(14,165,233,0.08)]">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-bold text-white">Cost Breakdown</h2>
            <select
              value={groupBy}
              onChange={(e) => setGroupBy(e.target.value)}
              className="bg-slate-900 border border-sky-900/70 rounded-lg px-3 py-1.5 text-sm text-gray-300 focus:border-cyan-400 focus:outline-none"
            >
              <option value="provider">By Provider</option>
              <option value="model">By Model</option>
              <option value="agent_role">By Agent Role</option>
              <option value="build_id">By Build</option>
            </select>
          </div>
          {breakdown.length === 0 ? (
            <p className="text-gray-500 text-sm">No spend data yet</p>
          ) : (
            <div className="space-y-3">
              {breakdown.map((item, i) => {
                const cost = Number(item.billed_cost || 0)
                const pct = maxBreakdownCost > 0 ? Math.max((cost / maxBreakdownCost) * 100, cost > 0 ? 3 : 0) : 0
                const sharePct = totalBreakdownCost > 0 ? (cost / totalBreakdownCost) * 100 : 0
                const tone = meterToneForKey(item.key || 'unknown', i)
                return (
                  <div key={`${item.key || 'unknown'}-${i}`} className="grid grid-cols-[8rem_1fr_6rem_5rem] items-center gap-3">
                    <span className={`text-sm ${tone.text} w-32 truncate capitalize`}>{item.key || 'Unknown'}</span>
                    <div
                      className="relative h-7 overflow-hidden rounded-full border border-sky-900/70 bg-slate-900/90"
                      aria-label={`${item.key || 'Unknown'} spend meter ${sharePct.toFixed(1)} percent of total spend`}
                    >
                      <div
                        data-testid={`spend-meter-${item.key || i}`}
                        className="h-full rounded-full transition-all duration-500"
                        style={{
                          width: `${pct}%`,
                          background: `linear-gradient(90deg, ${tone.from}, ${tone.via}, ${tone.to})`,
                          boxShadow: `0 0 18px ${tone.to}66`,
                        }}
                      />
                      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(90deg,rgba(255,255,255,0.12),transparent_35%,rgba(255,255,255,0.08))]" />
                    </div>
                    <span className="text-sm text-white font-mono w-24 text-right">{formatCurrency(cost)}</span>
                    <span className="text-xs text-gray-500 w-20 text-right">{item.count} calls</span>
                  </div>
                )
              })}
            </div>
          )}
        </div>

        {/* History Table */}
        <div className="bg-slate-950/70 border border-sky-950/80 rounded-xl p-6 shadow-[0_0_40px_rgba(14,165,233,0.06)]">
          <h2 className="text-lg font-bold text-white mb-4">Recent Activity</h2>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-gray-400 border-b border-gray-800">
                  <th className="text-left py-2 px-3">Time</th>
                  <th className="text-left py-2 px-3">Provider</th>
                  <th className="text-left py-2 px-3">Model</th>
                  <th className="text-left py-2 px-3">Agent</th>
                  <th className="text-right py-2 px-3">Tokens</th>
                  <th className="text-right py-2 px-3">Cost</th>
                </tr>
              </thead>
              <tbody>
                {history.map((event) => (
                  <tr key={event.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                    <td className="py-2 px-3 text-gray-400">{new Date(event.created_at).toLocaleTimeString()}</td>
                    <td className="py-2 px-3 text-gray-300">{event.provider}</td>
                    <td className="py-2 px-3 text-gray-300 font-mono text-xs">{event.model}</td>
                    <td className="py-2 px-3 text-gray-300">{event.agent_role || '-'}</td>
                    <td className="py-2 px-3 text-right text-gray-400">{(event.input_tokens + event.output_tokens).toLocaleString()}</td>
                    <td className="py-2 px-3 text-right text-white font-mono">
                      ${event.billed_cost.toFixed(4)}
                      {event.is_byok && <span className="ml-1 text-xs text-green-400">BYOK</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  )
}

export default SpendDashboard
