// APEX.BUILD Validation Panel
// Shows real-time validation results during the guarantee loop:
// check statuses, placeholder hits, smoke test output, score gauge.

import React from 'react'
import { cn } from '@/lib/utils'

export interface ValidationCheck {
  name: string
  passed: boolean
  message: string
  severity: 'error' | 'warning' | 'info'
}

export interface PlaceholderHit {
  filePath: string
  line: number
  column: number
  match: string
}

export interface ValidationPanelProps {
  checks: ValidationCheck[]
  placeholders: PlaceholderHit[]
  score: number // 0–100
  verdict: 'pass' | 'soft_fail' | 'hard_fail' | 'pending'
  smokeTestOutput?: string
  duration?: number // ms
  className?: string
}

const verdictConfig = {
  pass:      { label: 'ALL CHECKS PASSED',  color: 'text-emerald-400', bg: 'bg-emerald-500/10', border: 'border-emerald-500/30', icon: 'M5 13l4 4L19 7' },
  soft_fail: { label: 'SOFT FAILURE — RETRYING', color: 'text-orange-400', bg: 'bg-orange-500/10', border: 'border-orange-500/30', icon: 'M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z' },
  hard_fail: { label: 'HARD FAILURE — ROLLING BACK', color: 'text-red-400', bg: 'bg-red-500/10', border: 'border-red-500/30', icon: 'M6 18L18 6M6 6l12 12' },
  pending:   { label: 'VALIDATING...', color: 'text-blue-400', bg: 'bg-blue-500/10', border: 'border-blue-500/30', icon: '' },
}

const severityIcon = {
  error:   { color: 'text-red-400',    bg: 'bg-red-500/10' },
  warning: { color: 'text-amber-400',  bg: 'bg-amber-500/10' },
  info:    { color: 'text-blue-400',   bg: 'bg-blue-500/10' },
}

export const ValidationPanel: React.FC<ValidationPanelProps> = ({
  checks,
  placeholders,
  score,
  verdict,
  smokeTestOutput,
  duration,
  className,
}) => {
  const vConfig = verdictConfig[verdict]
  const passedCount = checks.filter(c => c.passed).length
  const totalCount = checks.length

  return (
    <div className={cn(
      'rounded-lg border bg-gray-900/60 backdrop-blur-sm overflow-hidden',
      'border-gray-700/40',
      className,
    )}>
      {/* Header with verdict */}
      <div className={cn(
        'flex items-center gap-3 px-4 py-3 border-b',
        vConfig.bg,
        vConfig.border,
      )}>
        {/* Verdict icon */}
        {vConfig.icon && (
          <svg className={cn('w-5 h-5', vConfig.color)} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d={vConfig.icon} />
          </svg>
        )}
        {verdict === 'pending' && (
          <div className="w-5 h-5 border-2 border-blue-400 border-t-transparent rounded-full animate-spin" />
        )}

        <span className={cn('text-sm font-mono font-bold tracking-wider', vConfig.color)}>
          {vConfig.label}
        </span>

        <span className="ml-auto text-xs text-gray-500 font-mono">
          {passedCount}/{totalCount} checks
        </span>

        {duration !== undefined && (
          <span className="text-xs text-gray-500 font-mono">
            {duration}ms
          </span>
        )}
      </div>

      {/* Score bar */}
      <div className="px-4 py-2 border-b border-gray-800/50">
        <div className="flex items-center gap-3">
          <span className="text-xs text-gray-500 font-mono w-14">SCORE</span>
          <div className="flex-1 h-2 bg-gray-800 rounded-full overflow-hidden">
            <div
              className={cn(
                'h-full rounded-full transition-all duration-1000 ease-out',
                score >= 80 ? 'bg-gradient-to-r from-emerald-600 to-emerald-400' :
                score >= 50 ? 'bg-gradient-to-r from-amber-600 to-amber-400' :
                              'bg-gradient-to-r from-red-600 to-red-400',
              )}
              style={{ width: `${score}%` }}
            />
          </div>
          <span className={cn(
            'text-sm font-mono font-bold w-12 text-right',
            score >= 80 ? 'text-emerald-400' :
            score >= 50 ? 'text-amber-400' : 'text-red-400',
          )}>
            {Math.round(score)}%
          </span>
        </div>
      </div>

      {/* Check list */}
      <div className="divide-y divide-gray-800/30">
        {checks.map((check, i) => {
          const sev = severityIcon[check.severity]
          return (
            <div
              key={i}
              className={cn(
                'flex items-center gap-3 px-4 py-2',
                'hover:bg-gray-800/20 transition-colors',
              )}
            >
              {/* Pass/fail indicator */}
              <div className={cn(
                'w-5 h-5 rounded flex items-center justify-center flex-shrink-0',
                check.passed ? 'bg-emerald-500/20' : sev.bg,
              )}>
                {check.passed ? (
                  <svg className="w-3 h-3 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                  </svg>
                ) : (
                  <svg className={cn('w-3 h-3', sev.color)} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                )}
              </div>

              {/* Check name */}
              <span className="text-xs text-gray-400 font-mono w-32 flex-shrink-0">
                {check.name}
              </span>

              {/* Message */}
              <span className={cn(
                'text-xs font-mono flex-1 truncate',
                check.passed ? 'text-gray-500' : sev.color,
              )}>
                {check.message}
              </span>
            </div>
          )
        })}
      </div>

      {/* Placeholder hits */}
      {placeholders.length > 0 && (
        <div className="border-t border-gray-800/50 px-4 py-3">
          <div className="flex items-center gap-2 mb-2">
            <svg className="w-4 h-4 text-orange-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span className="text-xs font-mono font-bold text-orange-400">
              {placeholders.length} PLACEHOLDER{placeholders.length !== 1 ? 'S' : ''} DETECTED
            </span>
          </div>
          <div className="space-y-1 max-h-32 overflow-y-auto">
            {placeholders.slice(0, 10).map((ph, i) => (
              <div key={i} className="flex items-center gap-2 text-[10px] font-mono text-gray-500">
                <span className="text-gray-400">{ph.filePath}:{ph.line}:{ph.column}</span>
                <span className="text-orange-300/70 truncate">{ph.match}</span>
              </div>
            ))}
            {placeholders.length > 10 && (
              <span className="text-[10px] font-mono text-gray-500">
                ...and {placeholders.length - 10} more
              </span>
            )}
          </div>
        </div>
      )}

      {/* Smoke test output */}
      {smokeTestOutput && (
        <div className="border-t border-gray-800/50 px-4 py-3">
          <div className="flex items-center gap-2 mb-2">
            <span className="text-xs font-mono font-bold text-gray-400">SMOKE TEST OUTPUT</span>
          </div>
          <pre className="text-[10px] font-mono text-gray-500 bg-black/30 rounded p-2 max-h-24 overflow-y-auto whitespace-pre-wrap">
            {smokeTestOutput}
          </pre>
        </div>
      )}
    </div>
  )
}

export default ValidationPanel
