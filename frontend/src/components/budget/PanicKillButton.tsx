import React, { useState } from 'react'
import { OctagonX } from 'lucide-react'
import apiService from '@/services/api'

interface PanicKillButtonProps {
  visible?: boolean
}

export const PanicKillButton: React.FC<PanicKillButtonProps> = ({ visible = true }) => {
  const [confirming, setConfirming] = useState(false)
  const [killing, setKilling] = useState(false)

  if (!visible) return null

  const handleKill = async () => {
    if (!confirming) {
      setConfirming(true)
      setTimeout(() => setConfirming(false), 3000)
      return
    }
    setKilling(true)
    try {
      await apiService.post('/budget/kill-all')
    } catch (err) {
      console.error('Kill all failed:', err)
    } finally {
      setKilling(false)
      setConfirming(false)
    }
  }

  return (
    <button
      onClick={handleKill}
      disabled={killing}
      className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-bold transition-all duration-200 ${
        confirming
          ? 'bg-red-600 text-white animate-pulse shadow-lg shadow-red-600/50'
          : 'bg-red-900/30 text-red-400 hover:bg-red-800/50 border border-red-900/50'
      } ${killing ? 'opacity-50 cursor-not-allowed' : ''}`}
      title={confirming ? 'Click again to confirm KILL ALL' : 'Emergency stop all builds'}
    >
      <OctagonX size={16} />
      {killing ? 'Stopping...' : confirming ? 'CONFIRM KILL' : 'KILL ALL'}
    </button>
  )
}

export default PanicKillButton
