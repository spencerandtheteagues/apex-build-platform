import React, { useEffect, useState, useCallback } from 'react'
import { Shield, Plus, Trash2, AlertTriangle } from 'lucide-react'
import apiService from '@/services/api'

interface BudgetCap {
  id: number
  cap_type: string
  limit_usd: number
  action: string
  project_id?: number
  is_active: boolean
}

export const BudgetSettings: React.FC = () => {
  const [caps, setCaps] = useState<BudgetCap[]>([])
  const [loading, setLoading] = useState(true)
  const [newCap, setNewCap] = useState({ cap_type: 'daily', limit_usd: '', action: 'stop' })
  const [saving, setSaving] = useState(false)

  const loadCaps = useCallback(async () => {
    try {
      const res = await apiService.get('/budget/caps')
      setCaps(res.data?.caps || [])
    } catch (err) {
      console.error('Failed to load budget caps:', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadCaps() }, [loadCaps])

  const handleSave = async () => {
    const limitVal = parseFloat(newCap.limit_usd)
    if (isNaN(limitVal) || limitVal <= 0) return
    setSaving(true)
    try {
      await apiService.post('/budget/caps', {
        cap_type: newCap.cap_type,
        limit_usd: limitVal,
        action: newCap.action,
      })
      setNewCap({ cap_type: 'daily', limit_usd: '', action: 'stop' })
      await loadCaps()
    } catch (err) {
      console.error('Failed to save budget cap:', err)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: number) => {
    try {
      await apiService.delete(`/budget/caps/${id}`)
      setCaps(prev => prev.filter(c => c.id !== id))
    } catch (err) {
      console.error('Failed to delete cap:', err)
    }
  }

  const capTypeLabels: Record<string, string> = {
    daily: 'Daily Limit',
    monthly: 'Monthly Limit',
    per_build: 'Per-Build Limit',
  }

  return (
    <div className="space-y-4">
      {/* Existing Caps */}
      {caps.length > 0 && (
        <div className="space-y-2">
          {caps.map(cap => (
            <div key={cap.id} className="flex items-center justify-between bg-black/40 border border-gray-700 rounded-lg p-3">
              <div className="flex items-center gap-3">
                <Shield size={16} className="text-red-400" />
                <span className="text-white font-medium">{capTypeLabels[cap.cap_type] || cap.cap_type}</span>
                <span className="text-gray-400">—</span>
                <span className="text-green-400 font-mono">${cap.limit_usd.toFixed(2)}</span>
                <span className={`text-xs px-2 py-0.5 rounded-full ${cap.action === 'stop' ? 'bg-red-900/30 text-red-400' : 'bg-yellow-900/30 text-yellow-400'}`}>
                  {cap.action === 'stop' ? 'Hard Stop' : 'Warn Only'}
                </span>
              </div>
              <button onClick={() => handleDelete(cap.id)} className="text-gray-500 hover:text-red-400 transition-colors">
                <Trash2 size={16} />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Add New Cap */}
      <div className="flex items-end gap-3">
        <div className="flex-1">
          <label className="text-xs text-gray-400 mb-1 block">Type</label>
          <select
            value={newCap.cap_type}
            onChange={e => setNewCap(prev => ({ ...prev, cap_type: e.target.value }))}
            className="w-full bg-black/50 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm"
          >
            <option value="daily">Daily</option>
            <option value="monthly">Monthly</option>
            <option value="per_build">Per Build</option>
          </select>
        </div>
        <div className="flex-1">
          <label className="text-xs text-gray-400 mb-1 block">Limit (USD)</label>
          <input
            type="number"
            step="0.01"
            min="0.01"
            value={newCap.limit_usd}
            onChange={e => setNewCap(prev => ({ ...prev, limit_usd: e.target.value }))}
            placeholder="10.00"
            className="w-full bg-black/50 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm"
          />
        </div>
        <div className="flex-1">
          <label className="text-xs text-gray-400 mb-1 block">Action</label>
          <select
            value={newCap.action}
            onChange={e => setNewCap(prev => ({ ...prev, action: e.target.value }))}
            className="w-full bg-black/50 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm"
          >
            <option value="stop">Hard Stop</option>
            <option value="warn">Warn Only</option>
          </select>
        </div>
        <button
          onClick={handleSave}
          disabled={saving || !newCap.limit_usd}
          className="flex items-center gap-1.5 px-4 py-2 bg-red-600 hover:bg-red-500 disabled:opacity-50 rounded-lg text-white text-sm font-medium transition-colors"
        >
          <Plus size={16} />
          Add
        </button>
      </div>
    </div>
  )
}

export default BudgetSettings
