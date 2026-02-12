// APEX.BUILD AI Code Review Panel
// Dark cyberpunk themed panel for AI-powered code analysis and quality scoring

import React, { useState, useCallback } from 'react'
import { cn } from '@/lib/utils'
import { useCodeReview, ReviewFocusFilter } from '@/hooks/useCodeReview'
import type { CodeReviewFinding } from '@/services/api'
import { Button, Badge, Card, Loading } from '@/components/ui'
import {
  Bug,
  ShieldAlert,
  Zap,
  Paintbrush,
  Lightbulb,
  AlertCircle,
  AlertTriangle,
  Info,
  HelpCircle,
  CheckCircle,
  CheckCircle2,
  XCircle,
  X,
  ChevronRight,
  ChevronDown,
  Play,
  FileCode,
  Search,
  Sparkles,
  Hash,
  BarChart3,
  RefreshCw,
  MousePointerClick,
  Clock
} from 'lucide-react'

// Cast icons to any to avoid strict type mismatches between lucide versions
const Icons = {
  bug: Bug as any,
  security: ShieldAlert as any,
  performance: Zap as any,
  style: Paintbrush as any,
  best_practice: Lightbulb as any,
  error: AlertCircle as any,
  warning: AlertTriangle as any,
  info: Info as any,
  hint: HelpCircle as any,
}


// ---------------------------------------------------------------------------
// Type icon mapping
// ---------------------------------------------------------------------------

type LucideIcon = any // Relaxed type for icon components to avoid strict mismatches

const TYPE_ICON: Record<string, LucideIcon> = {
  bug: Bug,
  security: ShieldAlert,
  performance: Zap,
  style: Paintbrush,
  best_practice: Lightbulb,
}

const SEVERITY_ICON: Record<string, LucideIcon> = {
  error: AlertCircle,
  warning: AlertTriangle,
  info: Info,
  hint: HelpCircle,
}

const SEVERITY_COLOR: Record<string, string> = {
  error: 'text-red-400',
  warning: 'text-yellow-400',
  info: 'text-blue-400',
  hint: 'text-gray-400',
}

const SEVERITY_BG: Record<string, string> = {
  error: 'bg-red-500/20 text-red-300 border-red-500/30',
  warning: 'bg-yellow-500/20 text-yellow-300 border-yellow-500/30',
  info: 'bg-blue-500/20 text-blue-300 border-blue-500/30',
  hint: 'bg-gray-500/20 text-gray-300 border-gray-500/30',
}

const SEVERITY_RING: Record<string, string> = {
  error: 'stroke-red-500',
  warning: 'stroke-yellow-500',
  info: 'stroke-blue-500',
  hint: 'stroke-gray-500',
}

const FOCUS_FILTERS: { key: ReviewFocusFilter; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'bugs', label: 'Bugs' },
  { key: 'security', label: 'Security' },
  { key: 'performance', label: 'Perf' },
  { key: 'style', label: 'Style' },
]

// ---------------------------------------------------------------------------
// Circular Score Gauge
// ---------------------------------------------------------------------------

interface ScoreGaugeProps {
  score: number
  size?: number
  strokeWidth?: number
}

function scoreColor(score: number): string {
  if (score >= 80) return '#22c55e' // green-500
  if (score >= 60) return '#eab308' // yellow-500
  if (score >= 40) return '#f97316' // orange-500
  return '#ef4444'                   // red-500
}

function scoreGrade(score: number): string {
  if (score >= 90) return 'A'
  if (score >= 80) return 'B'
  if (score >= 70) return 'C'
  if (score >= 60) return 'D'
  return 'F'
}

const ScoreGauge: React.FC<ScoreGaugeProps> = ({ score, size = 96, strokeWidth = 8 }) => {
  const radius = (size - strokeWidth) / 2
  const circumference = 2 * Math.PI * radius
  const progress = Math.max(0, Math.min(100, score))
  const offset = circumference - (progress / 100) * circumference
  const color = scoreColor(score)

  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      <svg width={size} height={size} className="-rotate-90">
        {/* Background track */}
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="rgba(255,255,255,0.08)"
          strokeWidth={strokeWidth}
        />
        {/* Progress arc */}
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke={color}
          strokeWidth={strokeWidth}
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          strokeLinecap="round"
          className="transition-all duration-700 ease-out"
          style={{ filter: `drop-shadow(0 0 6px ${color}80)` }}
        />
      </svg>
      {/* Center text */}
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span
          className="text-2xl font-bold tabular-nums"
          style={{ color }}
        >
          {score}
        </span>
        <span className="text-[10px] uppercase tracking-wider text-gray-500 font-medium">
          {scoreGrade(score)}
        </span>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Finding Card
// ---------------------------------------------------------------------------

interface FindingCardProps {
  finding: CodeReviewFinding
  index: number
  onLineClick?: (line: number) => void
}

const FindingCard: React.FC<FindingCardProps> = ({ finding, index, onLineClick }) => {
  const [expanded, setExpanded] = useState(false)

  const TypeIcon = TYPE_ICON[finding.type] || Lightbulb
  const SevIcon = SEVERITY_ICON[finding.severity] || Info
  const sevColor = SEVERITY_COLOR[finding.severity] || 'text-gray-400'
  const sevBg = SEVERITY_BG[finding.severity] || SEVERITY_BG.info

  return (
    <div
      className={cn(
        'group border border-gray-700/50 rounded-lg bg-gray-800/40 hover:bg-gray-800/70',
        'transition-colors duration-150',
      )}
    >
      {/* Header row */}
      <button
        onClick={() => setExpanded(prev => !prev)}
        className="w-full flex items-start gap-2.5 px-3 py-2.5 text-left"
      >
        <div className={cn('mt-0.5 shrink-0', sevColor)}>
          <TypeIcon size={16} />
        </div>

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <Badge
              variant="outline"
              size="xs"
              className={cn('border text-[10px] uppercase tracking-wide', sevBg)}
            >
              {finding.severity}
            </Badge>
            <span className="text-[10px] text-gray-500 uppercase tracking-wide font-medium">
              {finding.type.replace('_', ' ')}
            </span>
            {finding.line > 0 && (
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  onLineClick?.(finding.line)
                }}
                className="text-[10px] text-cyan-400 hover:text-cyan-300 transition-colors font-mono cursor-pointer"
                title={`Go to line ${finding.line}`}
              >
                L{finding.line}
                {finding.end_line > 0 && finding.end_line !== finding.line && `-${finding.end_line}`}
              </button>
            )}
          </div>
          <p className="text-sm text-gray-200 mt-1 leading-snug">{finding.message}</p>
        </div>

        <div className="shrink-0 mt-1 text-gray-500">
          {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        </div>
      </button>

      {/* Expanded details */}
      {expanded && (
        <div className="px-3 pb-3 border-t border-gray-700/30 mt-0">
          {finding.suggestion && (
            <div className="mt-2.5">
              <div className="flex items-center gap-1.5 mb-1">
                <Sparkles size={12} className="text-purple-400" />
                <span className="text-[11px] text-purple-400 font-medium uppercase tracking-wide">Suggestion</span>
              </div>
              <p className="text-xs text-gray-300 leading-relaxed pl-4">{finding.suggestion}</p>
            </div>
          )}
          {finding.code && (
            <div className="mt-2.5">
              <pre className="text-[11px] text-green-300 bg-black/40 rounded px-3 py-2 overflow-x-auto font-mono border border-gray-700/30">
                {finding.code}
              </pre>
            </div>
          )}
          {finding.rule_id && (
            <div className="mt-2 text-[10px] text-gray-500 font-mono">
              Rule: {finding.rule_id}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Metrics Display
// ---------------------------------------------------------------------------

interface MetricsBarProps {
  metrics: {
    total_lines: number
    code_lines: number
    comment_lines: number
    blank_lines: number
    complexity: number
    security_issues: number
    bug_risks: number
    style_issues: number
  }
}

const MetricsBar: React.FC<MetricsBarProps> = ({ metrics }) => {
  const items = [
    { label: 'Lines', value: metrics.total_lines, icon: FileCode },
    { label: 'Code', value: metrics.code_lines, icon: Hash },
    { label: 'Comments', value: metrics.comment_lines, icon: Hash },
    { label: 'Complexity', value: metrics.complexity, icon: BarChart3 },
  ]

  return (
    <div className="grid grid-cols-4 gap-2">
      {items.map((item) => {
        const Icon = item.icon
        return (
          <div
            key={item.label}
            className="flex flex-col items-center p-2 bg-gray-800/50 rounded-lg border border-gray-700/30"
          >
            <Icon size={12} className="text-gray-500 mb-1" />
            <span className="text-sm font-bold text-white tabular-nums">{item.value}</span>
            <span className="text-[9px] text-gray-500 uppercase tracking-wider">{item.label}</span>
          </div>
        )
      })}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Severity Summary Pills
// ---------------------------------------------------------------------------

interface SeveritySummaryProps {
  errorCount: number
  warningCount: number
  infoCount: number
  hintCount: number
}

const SeveritySummary: React.FC<SeveritySummaryProps> = ({
  errorCount,
  warningCount,
  infoCount,
  hintCount,
}) => {
  const pills = [
    { label: 'Errors', count: errorCount, color: 'bg-red-500/20 text-red-300' },
    { label: 'Warnings', count: warningCount, color: 'bg-yellow-500/20 text-yellow-300' },
    { label: 'Info', count: infoCount, color: 'bg-blue-500/20 text-blue-300' },
    { label: 'Hints', count: hintCount, color: 'bg-gray-500/20 text-gray-300' },
  ]

  return (
    <div className="flex items-center gap-1.5 flex-wrap">
      {pills.map(pill => (
        <span
          key={pill.label}
          className={cn(
            'inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium',
            pill.color,
          )}
        >
          <span className="tabular-nums">{pill.count}</span>
          {pill.label}
        </span>
      ))}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main Panel
// ---------------------------------------------------------------------------

export interface CodeReviewPanelProps {
  code: string
  language: string
  fileName?: string
  selectedCode?: string
  selectionStartLine?: number
  selectionEndLine?: number
  onLineClick?: (line: number) => void
  className?: string
}

export const CodeReviewPanel: React.FC<CodeReviewPanelProps> = ({
  code,
  language,
  fileName,
  selectedCode,
  selectionStartLine,
  selectionEndLine,
  onLineClick,
  className,
}) => {
  const review = useCodeReview()
  const { focusFilter, reviewCode, reviewSelection } = review
  const [showSuggestions, setShowSuggestions] = useState(false)

  const handleReviewCode = useCallback(async () => {
    try {
      const focusMap: Record<string, string[]> = {
        all: [],
        bugs: ['bugs'],
        security: ['security'],
        performance: ['performance'],
        style: ['style', 'best_practices', 'readability'],
        best_practice: ['best_practices'],
      }
      const focus = focusMap[focusFilter] || []
      await reviewCode(code, language, fileName, focus.length > 0 ? focus : undefined)
    } catch {
      // Error is captured in hook state
    }
  }, [code, language, fileName, focusFilter, reviewCode])

  const handleReviewSelection = useCallback(async () => {
    if (!selectedCode || !selectionStartLine || !selectionEndLine) return
    try {
      const focusMap: Record<string, string[]> = {
        all: [],
        bugs: ['bugs'],
        security: ['security'],
        performance: ['performance'],
        style: ['style', 'best_practices', 'readability'],
        best_practice: ['best_practices'],
      }
      const focus = focusMap[focusFilter] || []
      await reviewSelection(
        selectedCode,
        language,
        selectionStartLine,
        selectionEndLine,
        fileName,
        focus.length > 0 ? focus : undefined,
      )
    } catch {
      // Error is captured in hook state
    }
  }, [selectedCode, language, selectionStartLine, selectionEndLine, fileName, focusFilter, reviewSelection])

  // Group filtered findings by severity for ordered display
  const groupedFindings: { severity: string; items: CodeReviewFinding[] }[] = React.useMemo(() => {
    const order = ['error', 'warning', 'info', 'hint']
    const groups: Record<string, CodeReviewFinding[]> = {}
    for (const f of review.filteredFindings) {
      const sev = f.severity || 'info'
      if (!groups[sev]) groups[sev] = []
      groups[sev].push(f)
    }
    return order
      .filter(s => groups[s] && groups[s].length > 0)
      .map(s => ({ severity: s, items: groups[s] }))
  }, [review.filteredFindings])

  return (
    <Card variant="cyberpunk" padding="none" className={cn('h-full flex flex-col border-0 overflow-hidden', className)}>
      {/* Header */}
      <div className="px-4 py-3 border-b border-gray-700/50 bg-gray-900/60 shrink-0">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <ShieldAlert size={16} className="text-red-400" />
            <h3 className="text-sm font-semibold text-white">AI Code Review</h3>
          </div>
          {review.hasResults && (
            <button
              onClick={review.clearReview}
              className="text-gray-500 hover:text-gray-300 transition-colors"
              title="Clear results"
            >
              <X size={14} />
            </button>
          )}
        </div>

        {/* Focus filter buttons */}
        <div className="flex items-center gap-1 mb-3">
          {FOCUS_FILTERS.map(f => (
            <button
              key={f.key}
              onClick={() => review.setFocusFilter(f.key)}
              className={cn(
                'px-2.5 py-1 rounded text-[11px] font-medium transition-colors',
                review.focusFilter === f.key
                  ? 'bg-red-500/20 text-red-300 border border-red-500/40'
                  : 'bg-gray-800/60 text-gray-400 hover:text-gray-200 border border-transparent hover:border-gray-600/50',
              )}
            >
              {f.label}
            </button>
          ))}
        </div>

        {/* Action buttons */}
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="primary"
            onClick={handleReviewCode}
            disabled={review.isReviewing || !code}
            icon={review.isReviewing ? <Loading size="xs" variant="spinner" /> : <RefreshCw size={14} />}
            className="flex-1 bg-red-600 hover:bg-red-500 disabled:opacity-50"
          >
            {review.isReviewing ? 'Reviewing...' : 'Review Code'}
          </Button>

          {selectedCode && (
            <Button
              size="sm"
              variant="outline"
              onClick={handleReviewSelection}
              disabled={review.isReviewing}
              icon={<MousePointerClick size={14} />}
              className="shrink-0"
              title={`Review selection (lines ${selectionStartLine}-${selectionEndLine})`}
            >
              Selection
            </Button>
          )}
        </div>
      </div>

      {/* Scrollable content */}
      <div className="flex-1 overflow-y-auto scrollbar-thin scrollbar-thumb-gray-700 scrollbar-track-transparent">
        {/* Error state */}
        {review.error && (
          <div className="mx-4 mt-3 p-3 bg-red-500/10 border border-red-500/30 rounded-lg">
            <div className="flex items-start gap-2">
              <AlertCircle size={14} className="text-red-400 shrink-0 mt-0.5" />
              <div className="flex-1 min-w-0">
                <p className="text-xs text-red-300">{review.error}</p>
              </div>
              <button
                onClick={review.dismissError}
                className="text-red-400 hover:text-red-300 shrink-0"
              >
                <X size={12} />
              </button>
            </div>
          </div>
        )}

        {/* Loading state */}
        {review.isReviewing && (
          <div className="flex flex-col items-center justify-center py-12 px-4">
            <Loading size="lg" variant="spinner" />
            <p className="mt-4 text-sm text-gray-400">Analyzing code quality...</p>
            <p className="mt-1 text-xs text-gray-600">AI is reviewing your code for issues</p>
          </div>
        )}

        {/* Empty state */}
        {!review.isReviewing && !review.hasResults && !review.error && (
          <div className="flex flex-col items-center justify-center py-12 px-4 text-center">
            <div className="w-16 h-16 rounded-full bg-gray-800/60 border border-gray-700/40 flex items-center justify-center mb-4">
              <ShieldAlert size={24} className="text-gray-600" />
            </div>
            <p className="text-sm text-gray-400 mb-1">No review results yet</p>
            <p className="text-xs text-gray-600 max-w-[220px]">
              Click "Review Code" to run AI-powered analysis on your current file
            </p>
          </div>
        )}

        {/* Results */}
        {!review.isReviewing && review.hasResults && (
          <div className="p-4 space-y-4">
            {/* Score and Summary section */}
            <div className="flex items-start gap-4">
              <ScoreGauge score={review.score} size={88} strokeWidth={7} />
              <div className="flex-1 min-w-0">
                <p className="text-sm text-gray-200 leading-relaxed mb-2">{review.summary}</p>
                <SeveritySummary
                  errorCount={review.errorCount}
                  warningCount={review.warningCount}
                  infoCount={review.infoCount}
                  hintCount={review.hintCount}
                />
                {review.durationMs > 0 && (
                  <div className="flex items-center gap-1 mt-2 text-[10px] text-gray-600">
                    <Clock size={10} />
                    <span>Reviewed in {(review.durationMs / 1000).toFixed(1)}s</span>
                  </div>
                )}
              </div>
            </div>

            {/* Metrics */}
            {review.metrics && <MetricsBar metrics={review.metrics} />}

            {/* Suggestions toggle */}
            {review.suggestions.length > 0 && (
              <div className="bg-purple-500/10 border border-purple-500/20 rounded-lg">
                <button
                  onClick={() => setShowSuggestions(prev => !prev)}
                  className="w-full flex items-center justify-between px-3 py-2 text-left"
                >
                  <div className="flex items-center gap-2">
                    <Sparkles size={13} className="text-purple-400" />
                    <span className="text-xs text-purple-300 font-medium">
                      {review.suggestions.length} Improvement Suggestion{review.suggestions.length !== 1 ? 's' : ''}
                    </span>
                  </div>
                  {showSuggestions ? (
                    <ChevronDown size={13} className="text-purple-400" />
                  ) : (
                    <ChevronRight size={13} className="text-purple-400" />
                  )}
                </button>
                {showSuggestions && (
                  <div className="px-3 pb-3 space-y-1.5">
                    {review.suggestions.map((s, i) => (
                      <div key={i} className="flex items-start gap-2">
                        <CheckCircle2 size={11} className="text-purple-400 mt-0.5 shrink-0" />
                        <p className="text-xs text-gray-300 leading-relaxed">{s}</p>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* Findings grouped by severity */}
            {groupedFindings.length > 0 ? (
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <h4 className="text-xs font-medium text-gray-400 uppercase tracking-wider">
                    Findings ({review.filteredFindings.length})
                  </h4>
                </div>

                {groupedFindings.map(group => (
                  <div key={group.severity} className="space-y-1.5">
                    <div className="flex items-center gap-1.5 pt-1">
                      {React.createElement(
                        SEVERITY_ICON[group.severity] || Info,
                        { size: 12, className: SEVERITY_COLOR[group.severity] || 'text-gray-400' }
                      )}
                      <span className={cn(
                        'text-[10px] font-semibold uppercase tracking-wider',
                        SEVERITY_COLOR[group.severity] || 'text-gray-400',
                      )}>
                        {group.severity} ({group.items.length})
                      </span>
                    </div>
                    <div className="space-y-1.5">
                      {group.items.map((finding, idx) => (
                        <FindingCard
                          key={`${finding.rule_id}-${idx}`}
                          finding={finding}
                          index={idx}
                          onLineClick={onLineClick}
                        />
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              review.hasResults && review.findings.length > 0 && (
                <div className="text-center py-6">
                  <CheckCircle2 size={20} className="text-green-400 mx-auto mb-2" />
                  <p className="text-xs text-gray-400">No findings match the current filter</p>
                </div>
              )
            )}

            {/* Clean bill of health */}
            {review.hasResults && review.findings.length === 0 && (
              <div className="text-center py-6 bg-green-500/5 border border-green-500/20 rounded-lg">
                <CheckCircle2 size={28} className="text-green-400 mx-auto mb-2" />
                <p className="text-sm text-green-300 font-medium">Looking good!</p>
                <p className="text-xs text-gray-400 mt-1">No issues found in your code</p>
              </div>
            )}
          </div>
        )}
      </div>
    </Card>
  )
}

export default CodeReviewPanel
