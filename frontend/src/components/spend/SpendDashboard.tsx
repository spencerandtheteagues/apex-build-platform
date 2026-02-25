import React, { useEffect, useState, useCallback } from 'react'
import { DollarSign, Download, TrendingUp, Clock, Cpu, BarChart3 } from 'lucide-react'
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

export const SpendDashboard: React.FC = () => {
  const [summary, setSummary] = useState<SpendSummary | null>(null)
  const [breakdown, setBreakdown] = useState<BreakdownItem[]>([])
  const [history, setHistory] = useState<SpendEvent[]>([])
  const [groupBy, setGroupBy] = useState('provider')
  const [loading, setLoading] = useState(true)

  const loadData = useCallback(async () => {
    try {
      const [summaryRes, breakdownRes, historyRes] = await Promise.all([
        apiService.get('/spend/summary'),
        apiService.get(`/spend/breakdown?group_by=${groupBy}`),
        apiService.get('/spend/history?limit=50'),
      ])
      setSummary(summaryRes.data)
      setBreakdown(breakdownRes.data?.items || [])
      setHistory(historyRes.data?.events || [])
    } catch (err) {
      console.error('Failed to load spend data:', err)
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

  return (
    <div className="min-h-full bg-black p-6">
      <div className="max-w-6xl mx-auto space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-black text-transparent bg-clip-text bg-gradient-to-r from-red-500 to-orange-500">
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

        {/* Summary Cards */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          {[
            { label: "Today's Spend", value: summary?.daily_spend ?? 0, icon: DollarSign, color: 'red' },
            { label: "Monthly Spend", value: summary?.monthly_spend ?? 0, icon: TrendingUp, color: 'orange' },
            { label: "Today's Requests", value: summary?.daily_count ?? 0, icon: Cpu, color: 'yellow', isCurrency: false },
            { label: "Monthly Requests", value: summary?.monthly_count ?? 0, icon: BarChart3, color: 'purple', isCurrency: false },
          ].map((card) => (
            <div key={card.label} className="bg-gray-900/50 border border-gray-800 rounded-xl p-4">
              <div className="flex items-center gap-2 text-gray-400 text-sm mb-2">
                <card.icon size={16} className={`text-${card.color}-400`} />
                {card.label}
              </div>
              <div className="text-2xl font-bold text-white">
                {card.isCurrency === false ? card.value.toLocaleString() : `$${card.value.toFixed(4)}`}
              </div>
            </div>
          ))}
        </div>

        {/* Breakdown */}
        <div className="bg-gray-900/50 border border-gray-800 rounded-xl p-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-bold text-white">Cost Breakdown</h2>
            <select
              value={groupBy}
              onChange={(e) => setGroupBy(e.target.value)}
              className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-sm text-gray-300"
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
            <div className="space-y-2">
              {breakdown.map((item, i) => {
                const maxCost = Math.max(...breakdown.map(b => b.billed_cost), 0.001)
                const pct = (item.billed_cost / maxCost) * 100
                return (
                  <div key={i} className="flex items-center gap-3">
                    <span className="text-sm text-gray-400 w-32 truncate">{item.key || 'Unknown'}</span>
                    <div className="flex-1 h-6 bg-gray-800 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-gradient-to-r from-red-600 to-orange-500 rounded-full transition-all"
                        style={{ width: `${pct}%` }}
                      />
                    </div>
                    <span className="text-sm text-white font-mono w-24 text-right">${item.billed_cost.toFixed(4)}</span>
                    <span className="text-xs text-gray-500 w-20 text-right">{item.count} calls</span>
                  </div>
                )
              })}
            </div>
          )}
        </div>

        {/* History Table */}
        <div className="bg-gray-900/50 border border-gray-800 rounded-xl p-6">
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
