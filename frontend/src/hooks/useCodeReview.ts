// APEX.BUILD Code Review Hook
// State management for AI-powered code review features

import { useState, useCallback, useMemo } from 'react'
import apiService from '@/services/api'
import type { CodeReviewResponse, CodeReviewFinding, CodeReviewMetrics } from '@/services/api'

export type ReviewFocusFilter = 'all' | 'bugs' | 'security' | 'performance' | 'style' | 'best_practice'
export type FindingSeverity = 'error' | 'warning' | 'info' | 'hint'

interface CodeReviewState {
  findings: CodeReviewFinding[]
  summary: string
  score: number
  metrics: CodeReviewMetrics | null
  suggestions: string[]
  isReviewing: boolean
  error: string | null
  reviewedAt: string | null
  durationMs: number
  focusFilter: ReviewFocusFilter
}

const initialState: CodeReviewState = {
  findings: [],
  summary: '',
  score: -1,
  metrics: null,
  suggestions: [],
  isReviewing: false,
  error: null,
  reviewedAt: null,
  durationMs: 0,
  focusFilter: 'all',
}

export function useCodeReview() {
  const [state, setState] = useState<CodeReviewState>(initialState)

  // Review full file code
  const reviewCode = useCallback(async (
    code: string,
    language: string,
    fileName?: string,
    focus?: string[],
    maxResults?: number,
  ) => {
    setState(prev => ({
      ...prev,
      isReviewing: true,
      error: null,
    }))

    try {
      const response = await apiService.reviewCode({
        code,
        language,
        file_name: fileName,
        focus,
        max_results: maxResults,
      })

      setState(prev => ({
        ...prev,
        findings: response.findings,
        summary: response.summary,
        score: response.score,
        metrics: response.metrics,
        suggestions: response.suggestions,
        isReviewing: false,
        error: null,
        reviewedAt: response.reviewed_at,
        durationMs: response.duration_ms,
      }))

      return response
    } catch (err: any) {
      const errorMessage = err.response?.data?.error || err.message || 'Code review failed'
      setState(prev => ({
        ...prev,
        isReviewing: false,
        error: errorMessage,
      }))
      throw err
    }
  }, [])

  // Review a selected portion of code
  const reviewSelection = useCallback(async (
    code: string,
    language: string,
    startLine: number,
    endLine: number,
    fileName?: string,
    focus?: string[],
  ) => {
    const context = `Lines ${startLine}-${endLine} selected for review`

    setState(prev => ({
      ...prev,
      isReviewing: true,
      error: null,
    }))

    try {
      const response = await apiService.reviewCode({
        code,
        language,
        file_name: fileName,
        context,
        focus,
      })

      // Offset line numbers to match actual file positions
      const offsetFindings = response.findings.map(f => ({
        ...f,
        line: f.line > 0 ? f.line + startLine - 1 : f.line,
        end_line: f.end_line > 0 ? f.end_line + startLine - 1 : f.end_line,
      }))

      setState(prev => ({
        ...prev,
        findings: offsetFindings,
        summary: response.summary,
        score: response.score,
        metrics: response.metrics,
        suggestions: response.suggestions,
        isReviewing: false,
        error: null,
        reviewedAt: response.reviewed_at,
        durationMs: response.duration_ms,
      }))

      return { ...response, findings: offsetFindings }
    } catch (err: any) {
      const errorMessage = err.response?.data?.error || err.message || 'Code review failed'
      setState(prev => ({
        ...prev,
        isReviewing: false,
        error: errorMessage,
      }))
      throw err
    }
  }, [])

  // Set the active focus filter
  const setFocusFilter = useCallback((filter: ReviewFocusFilter) => {
    setState(prev => ({ ...prev, focusFilter: filter }))
  }, [])

  // Clear all review data
  const clearReview = useCallback(() => {
    setState(initialState)
  }, [])

  // Dismiss error
  const dismissError = useCallback(() => {
    setState(prev => ({ ...prev, error: null }))
  }, [])

  // Computed: findings filtered by focus
  const filteredFindings = useMemo(() => {
    if (state.focusFilter === 'all') return state.findings
    return state.findings.filter(f => f.type === state.focusFilter)
  }, [state.findings, state.focusFilter])

  // Computed: findings grouped by type
  const findingsByType = useMemo(() => {
    const groups: Record<string, CodeReviewFinding[]> = {}
    for (const f of state.findings) {
      const key = f.type || 'other'
      if (!groups[key]) groups[key] = []
      groups[key].push(f)
    }
    return groups
  }, [state.findings])

  // Computed: findings grouped by severity
  const findingsBySeverity = useMemo(() => {
    const groups: Record<FindingSeverity, CodeReviewFinding[]> = {
      error: [],
      warning: [],
      info: [],
      hint: [],
    }
    for (const f of state.findings) {
      const sev = (f.severity as FindingSeverity) || 'info'
      if (groups[sev]) {
        groups[sev].push(f)
      }
    }
    return groups
  }, [state.findings])

  // Computed: counts
  const errorCount = useMemo(() =>
    state.findings.filter(f => f.severity === 'error').length
  , [state.findings])

  const warningCount = useMemo(() =>
    state.findings.filter(f => f.severity === 'warning').length
  , [state.findings])

  const infoCount = useMemo(() =>
    state.findings.filter(f => f.severity === 'info').length
  , [state.findings])

  const hintCount = useMemo(() =>
    state.findings.filter(f => f.severity === 'hint').length
  , [state.findings])

  const hasResults = state.score >= 0

  return {
    // State
    findings: state.findings,
    filteredFindings,
    summary: state.summary,
    score: state.score,
    metrics: state.metrics,
    suggestions: state.suggestions,
    isReviewing: state.isReviewing,
    error: state.error,
    reviewedAt: state.reviewedAt,
    durationMs: state.durationMs,
    focusFilter: state.focusFilter,
    hasResults,

    // Actions
    reviewCode,
    reviewSelection,
    setFocusFilter,
    clearReview,
    dismissError,

    // Computed helpers
    findingsByType,
    findingsBySeverity,
    errorCount,
    warningCount,
    infoCount,
    hintCount,
  }
}

export default useCodeReview
