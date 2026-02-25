import React from 'react'
import { AlertTriangle, DollarSign, Zap } from 'lucide-react'

interface CostConfirmationModalProps {
  estimatedCostMin: number
  estimatedCostMax: number
  dailyRemaining: number
  monthlyRemaining: number
  onConfirm: () => void
  onCancel: () => void
}

export const CostConfirmationModal: React.FC<CostConfirmationModalProps> = ({
  estimatedCostMin,
  estimatedCostMax,
  dailyRemaining,
  monthlyRemaining,
  onConfirm,
  onCancel,
}) => {
  const isOverBudget = estimatedCostMax > Math.min(dailyRemaining, monthlyRemaining)

  return (
    <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-gray-900 border border-gray-700 rounded-2xl p-6 max-w-md w-full space-y-4">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-full bg-yellow-900/30 flex items-center justify-center">
            <DollarSign size={20} className="text-yellow-400" />
          </div>
          <div>
            <h3 className="text-lg font-bold text-white">Cost Estimate</h3>
            <p className="text-sm text-gray-400">Review before starting build</p>
          </div>
        </div>

        <div className="bg-black/40 border border-gray-800 rounded-xl p-4 space-y-2">
          <div className="flex justify-between text-sm">
            <span className="text-gray-400">Estimated cost</span>
            <span className="text-white font-mono">
              ${estimatedCostMin.toFixed(2)} - ${estimatedCostMax.toFixed(2)}
            </span>
          </div>
          <div className="flex justify-between text-sm">
            <span className="text-gray-400">Daily remaining</span>
            <span className={`font-mono ${dailyRemaining < estimatedCostMax ? 'text-yellow-400' : 'text-green-400'}`}>
              ${dailyRemaining.toFixed(2)}
            </span>
          </div>
          <div className="flex justify-between text-sm">
            <span className="text-gray-400">Monthly remaining</span>
            <span className={`font-mono ${monthlyRemaining < estimatedCostMax ? 'text-yellow-400' : 'text-green-400'}`}>
              ${monthlyRemaining.toFixed(2)}
            </span>
          </div>
        </div>

        {isOverBudget && (
          <div className="flex items-start gap-2 p-3 bg-red-900/20 border border-red-800/50 rounded-lg">
            <AlertTriangle size={16} className="text-red-400 mt-0.5 shrink-0" />
            <p className="text-sm text-red-300">
              This build may exceed your budget cap. It will be stopped if the limit is reached.
            </p>
          </div>
        )}

        <div className="flex gap-3">
          <button
            onClick={onCancel}
            className="flex-1 px-4 py-2.5 bg-gray-800 hover:bg-gray-700 border border-gray-700 rounded-xl text-gray-300 font-medium transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 bg-red-600 hover:bg-red-500 rounded-xl text-white font-medium transition-colors"
          >
            <Zap size={16} />
            Start Build
          </button>
        </div>
      </div>
    </div>
  )
}

export default CostConfirmationModal
