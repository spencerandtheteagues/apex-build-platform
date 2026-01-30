// APEX.BUILD Project Search Hook
// Manages project-wide search state, results navigation, and replace functionality

import { useState, useCallback, useRef } from 'react'
import apiService from '@/services/api'
import { File } from '@/types'

export interface SearchMatch {
  line: number
  content: string
  start: number
  end: number
}

export interface SearchFileResult {
  file: File
  matches: SearchMatch[]
}

export interface SearchOptions {
  caseSensitive: boolean
  regex: boolean
  wholeWord: boolean
  fileFilter: string
}

export interface SearchState {
  query: string
  replaceText: string
  results: SearchFileResult[]
  isSearching: boolean
  error: string | null
  totalMatchCount: number
  currentMatchIndex: number
  options: SearchOptions
}

export interface UseProjectSearchReturn extends SearchState {
  setQuery: (query: string) => void
  setReplaceText: (text: string) => void
  setOptions: (options: Partial<SearchOptions>) => void
  search: (query?: string) => Promise<void>
  clearSearch: () => void
  nextMatch: () => { file: File; line: number } | null
  previousMatch: () => { file: File; line: number } | null
  replaceCurrentMatch: () => Promise<void>
  replaceAllMatches: () => Promise<void>
  getCurrentMatch: () => { file: File; match: SearchMatch } | null
}

export function useProjectSearch(projectId: number | undefined): UseProjectSearchReturn {
  const [query, setQuery] = useState('')
  const [replaceText, setReplaceText] = useState('')
  const [results, setResults] = useState<SearchFileResult[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [totalMatchCount, setTotalMatchCount] = useState(0)
  const [currentMatchIndex, setCurrentMatchIndex] = useState(-1)
  const [options, setOptionsState] = useState<SearchOptions>({
    caseSensitive: false,
    regex: false,
    wholeWord: false,
    fileFilter: '',
  })

  const abortControllerRef = useRef<AbortController | null>(null)

  const setOptions = useCallback((newOptions: Partial<SearchOptions>) => {
    setOptionsState(prev => ({ ...prev, ...newOptions }))
  }, [])

  const flattenMatches = useCallback((searchResults: SearchFileResult[]): Array<{ file: File; match: SearchMatch }> => {
    const flat: Array<{ file: File; match: SearchMatch }> = []
    for (const result of searchResults) {
      for (const match of result.matches) {
        flat.push({ file: result.file, match })
      }
    }
    return flat
  }, [])

  const search = useCallback(async (searchQuery?: string) => {
    const q = searchQuery ?? query
    if (!q.trim() || !projectId) {
      setResults([])
      setTotalMatchCount(0)
      setCurrentMatchIndex(-1)
      return
    }

    // Cancel any in-flight request
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
    }
    abortControllerRef.current = new AbortController()

    setIsSearching(true)
    setError(null)

    try {
      const apiResults = await apiService.searchInFiles(projectId, q, {
        case_sensitive: options.caseSensitive,
        regex: options.regex,
      })

      // Apply client-side whole-word filter if needed
      let filteredResults: SearchFileResult[] = apiResults.map(r => ({
        file: r.file,
        matches: r.matches,
      }))

      // Apply file filter glob pattern (client-side)
      if (options.fileFilter.trim()) {
        const pattern = options.fileFilter.trim()
        filteredResults = filteredResults.filter(r => {
          return matchGlob(r.file.path || r.file.name, pattern)
        })
      }

      // Apply whole-word filter client-side
      if (options.wholeWord && !options.regex) {
        filteredResults = filteredResults.map(r => ({
          ...r,
          matches: r.matches.filter(m => {
            const wordBoundary = new RegExp(
              `\\b${escapeRegex(q)}\\b`,
              options.caseSensitive ? '' : 'i'
            )
            return wordBoundary.test(m.content)
          }),
        })).filter(r => r.matches.length > 0)
      }

      const total = filteredResults.reduce((sum, r) => sum + r.matches.length, 0)

      setResults(filteredResults)
      setTotalMatchCount(total)
      setCurrentMatchIndex(total > 0 ? 0 : -1)
    } catch (err: any) {
      if (err.name !== 'AbortError') {
        setError(err.message || 'Search failed')
        setResults([])
        setTotalMatchCount(0)
        setCurrentMatchIndex(-1)
      }
    } finally {
      setIsSearching(false)
    }
  }, [query, projectId, options])

  const clearSearch = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
    }
    setQuery('')
    setReplaceText('')
    setResults([])
    setTotalMatchCount(0)
    setCurrentMatchIndex(-1)
    setError(null)
    setIsSearching(false)
  }, [])

  const getCurrentMatch = useCallback((): { file: File; match: SearchMatch } | null => {
    if (currentMatchIndex < 0 || results.length === 0) return null
    const flat = flattenMatches(results)
    if (currentMatchIndex >= flat.length) return null
    return flat[currentMatchIndex]
  }, [currentMatchIndex, results, flattenMatches])

  const nextMatch = useCallback((): { file: File; line: number } | null => {
    if (totalMatchCount === 0) return null
    const nextIndex = (currentMatchIndex + 1) % totalMatchCount
    setCurrentMatchIndex(nextIndex)
    const flat = flattenMatches(results)
    const item = flat[nextIndex]
    if (!item) return null
    return { file: item.file, line: item.match.line }
  }, [currentMatchIndex, totalMatchCount, results, flattenMatches])

  const previousMatch = useCallback((): { file: File; line: number } | null => {
    if (totalMatchCount === 0) return null
    const prevIndex = (currentMatchIndex - 1 + totalMatchCount) % totalMatchCount
    setCurrentMatchIndex(prevIndex)
    const flat = flattenMatches(results)
    const item = flat[prevIndex]
    if (!item) return null
    return { file: item.file, line: item.match.line }
  }, [currentMatchIndex, totalMatchCount, results, flattenMatches])

  const replaceCurrentMatch = useCallback(async () => {
    const current = getCurrentMatch()
    if (!current || !replaceText && replaceText !== '') return

    try {
      const fileContent = current.file.content || ''
      const lines = fileContent.split('\n')
      const lineIndex = current.match.line - 1

      if (lineIndex >= 0 && lineIndex < lines.length) {
        const line = lines[lineIndex]
        const before = line.substring(0, current.match.start)
        const after = line.substring(current.match.end)
        lines[lineIndex] = before + replaceText + after

        await apiService.updateFile(current.file.id, {
          content: lines.join('\n'),
        })

        // Re-search to update results
        await search()
      }
    } catch (err: any) {
      setError(err.message || 'Replace failed')
    }
  }, [getCurrentMatch, replaceText, search])

  const replaceAllMatches = useCallback(async () => {
    if (results.length === 0 || !query.trim()) return

    try {
      const updates: Array<{ id: number; content: string }> = []

      for (const result of results) {
        let content = result.file.content || ''
        let searchPattern: RegExp

        if (options.regex) {
          searchPattern = new RegExp(query, options.caseSensitive ? 'g' : 'gi')
        } else if (options.wholeWord) {
          searchPattern = new RegExp(
            `\\b${escapeRegex(query)}\\b`,
            options.caseSensitive ? 'g' : 'gi'
          )
        } else {
          searchPattern = new RegExp(
            escapeRegex(query),
            options.caseSensitive ? 'g' : 'gi'
          )
        }

        const newContent = content.replace(searchPattern, replaceText)
        if (newContent !== content) {
          updates.push({ id: result.file.id, content: newContent })
        }
      }

      if (updates.length > 0) {
        await apiService.batchUpdateFiles(updates)
      }

      // Clear results after replace all
      setResults([])
      setTotalMatchCount(0)
      setCurrentMatchIndex(-1)
    } catch (err: any) {
      setError(err.message || 'Replace all failed')
    }
  }, [results, query, replaceText, options])

  return {
    query,
    replaceText,
    results,
    isSearching,
    error,
    totalMatchCount,
    currentMatchIndex,
    options,
    setQuery,
    setReplaceText,
    setOptions,
    search,
    clearSearch,
    nextMatch,
    previousMatch,
    replaceCurrentMatch,
    replaceAllMatches,
    getCurrentMatch,
  }
}

// Helper: escape special regex characters
function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

// Helper: simple glob matching (supports *, ?, and comma-separated patterns)
function matchGlob(filePath: string, pattern: string): boolean {
  // Support comma-separated patterns like "*.ts, *.go"
  const patterns = pattern.split(',').map(p => p.trim()).filter(Boolean)
  return patterns.some(p => {
    const regexStr = p
      .replace(/[.+^${}()|[\]\\]/g, '\\$&')
      .replace(/\*/g, '.*')
      .replace(/\?/g, '.')
    const regex = new RegExp(`(^|/)${regexStr}$`, 'i')
    return regex.test(filePath)
  })
}

export default useProjectSearch
