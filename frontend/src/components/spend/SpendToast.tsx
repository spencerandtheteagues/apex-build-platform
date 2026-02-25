import React, { useEffect, useState } from 'react'
import { DollarSign } from 'lucide-react'

interface SpendToastProps {
  agentRole: string
  cost: number
  onDismiss: () => void
}

export const SpendToast: React.FC<SpendToastProps> = ({ agentRole, cost, onDismiss }) => {
  const [visible, setVisible] = useState(true)

  useEffect(() => {
    const timer = setTimeout(() => {
      setVisible(false)
      setTimeout(onDismiss, 300)
    }, 3000)
    return () => clearTimeout(timer)
  }, [onDismiss])

  return (
    <div className={`flex items-center gap-2 px-3 py-2 bg-gray-900/90 border border-gray-700 rounded-lg text-sm transition-all duration-300 ${visible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-2'}`}>
      <DollarSign size={14} className="text-green-400" />
      <span className="text-gray-400">Agent</span>
      <span className="text-white font-medium">{agentRole}</span>
      <span className="text-gray-400">spent</span>
      <span className="text-green-400 font-mono">${cost.toFixed(4)}</span>
    </div>
  )
}

export default SpendToast
