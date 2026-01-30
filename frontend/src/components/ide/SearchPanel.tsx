// APEX.BUILD Search Panel
// Project-wide search with regex, case-sensitive, whole-word toggles,
// file filter, grouped results, replace functionality, and keyboard navigation

import React, { useState, useEffect, useCallback, useRef } from 'react'
import { cn } from '@/lib/utils'
import { useProjectSearch, SearchFileResult, SearchMatch } from '@/hooks/useProjectSearch'
import { File } from '@/types'
import { Button, Badge, Loading } from '@/components/ui'
import {
  Search,
  X,
  ChevronDown,
  ChevronRight,
  CaseSensitive,
  Regex,
  WholeWord,
  Replace,
  ReplaceAll,
  ArrowUp,
  ArrowDown,
  FileText,
  Filter,
  RotateCcw,
} from 'lucide-react'

export interface SearchPanelProps {
  projectId: number | undefined
  onFileOpen?: (file: File, line?: number) => void
  className?: string
}

export const SearchPanel: React.FC<SearchPanelProps> = ({
  projectId,
  onFileOpen,
  className,
}) => {
  const search = useProjectSearch(projectId)

  const [showReplace, setShowReplace] = useState(false)
  const [showFileFilter, setShowFileFilter] = useState(false)
  const [expandedFiles, setExpandedFiles] = useState<Set<number>>(new Set())

  const searchInputRef = useRef<HTMLInputElement>(null)
  const replaceInputRef = useRef<HTMLInputElement>(null)
  const debounceRef = useRef<NodeJS.Timeout | null>(null)

  // Focus search input on Cmd+Shift+F
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.shiftKey && e.key === 'f') {
        e.preventDefault()
        searchInputRef.current?.focus()
        searchInputRef.current?.select()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  // Auto-expand all result files when results change
  useEffect(() => {
    if (search.results.length > 0) {
      setExpandedFiles(new Set(search.results.map(r => r.file.id)))
    }
  }, [search.results])

  // Debounced search
  const handleQueryChange = useCallback((value: string) => {
    search.setQuery(value)

    if (debounceRef.current) {
      clearTimeout(debounceRef.current)
    }

    if (value.trim()) {
      debounceRef.current = setTimeout(() => {
        search.search(value)
      }, 300)
    } else {
      search.clearSearch()
    }
  }, [search])

  const handleSearchKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      if (e.shiftKey) {
        search.previousMatch()
      } else {
        search.search()
      }
    }
    if (e.key === 'Escape') {
      search.clearSearch()
      searchInputRef.current?.blur()
    }
  }, [search])

  const handleReplaceKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      if (e.shiftKey) {
        search.replaceAllMatches()
      } else {
        search.replaceCurrentMatch()
      }
    }
  }, [search])

  const toggleFileExpanded = useCallback((fileId: number) => {
    setExpandedFiles(prev => {
      const next = new Set(prev)
      if (next.has(fileId)) {
        next.delete(fileId)
      } else {
        next.add(fileId)
      }
      return next
    })
  }, [])

  const handleResultClick = useCallback((file: File, line: number) => {
    onFileOpen?.(file, line)
  }, [onFileOpen])

  const toggleOption = useCallback((option: 'caseSensitive' | 'regex' | 'wholeWord') => {
    search.setOptions({ [option]: !search.options[option] })
    // Re-search with new options if query exists
    if (search.query.trim()) {
      setTimeout(() => search.search(), 50)
    }
  }, [search])

  // Highlight matched text in a line
  const renderHighlightedContent = (content: string, match: SearchMatch) => {
    const { start, end } = match
    const before = content.substring(0, start)
    const matched = content.substring(start, end)
    const after = content.substring(end)

    return (
      <span className="text-xs font-mono whitespace-pre">
        <span className="text-gray-400">{before}</span>
        <span className="bg-yellow-500/30 text-yellow-200 rounded px-0.5">{matched}</span>
        <span className="text-gray-400">{after}</span>
      </span>
    )
  }

  const getFileExtension = (name: string) => {
    return name.split('.').pop()?.toLowerCase() || ''
  }

  const getFileStatusColor = (ext: string) => {
    const colorMap: Record<string, string> = {
      ts: 'text-blue-400',
      tsx: 'text-blue-300',
      js: 'text-yellow-400',
      jsx: 'text-yellow-300',
      go: 'text-cyan-400',
      py: 'text-green-400',
      rs: 'text-orange-400',
      css: 'text-pink-400',
      html: 'text-red-400',
      json: 'text-gray-300',
      md: 'text-gray-400',
    }
    return colorMap[ext] || 'text-gray-400'
  }

  const stagedChanges = search.results.filter(r => r.matches.length > 0)
  const totalFiles = stagedChanges.length

  return (
    <div className={cn('flex flex-col h-full bg-gray-900/80', className)}>
      {/* Search Header */}
      <div className="p-3 space-y-2 border-b border-gray-700/50">
        {/* Search input row */}
        <div className="flex items-center gap-1">
          <div className="flex-1 relative">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-500" />
            <input
              ref={searchInputRef}
              type="text"
              value={search.query}
              onChange={(e) => handleQueryChange(e.target.value)}
              onKeyDown={handleSearchKeyDown}
              placeholder="Search in project..."
              className="w-full bg-gray-800 border border-gray-600 rounded pl-8 pr-2 py-1.5 text-sm text-white placeholder:text-gray-500 focus:border-red-500 focus:outline-none focus:ring-1 focus:ring-red-500/30"
            />
            {search.query && (
              <button
                onClick={() => search.clearSearch()}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300"
              >
                <X size={14} />
              </button>
            )}
          </div>

          {/* Toggle buttons */}
          <div className="flex items-center">
            <button
              onClick={() => toggleOption('caseSensitive')}
              className={cn(
                'p-1.5 rounded transition-colors',
                search.options.caseSensitive
                  ? 'bg-red-500/20 text-red-400 border border-red-500/40'
                  : 'text-gray-500 hover:text-gray-300 hover:bg-gray-700'
              )}
              title="Case Sensitive (Alt+C)"
            >
              <CaseSensitive size={14} />
            </button>
            <button
              onClick={() => toggleOption('wholeWord')}
              className={cn(
                'p-1.5 rounded transition-colors',
                search.options.wholeWord
                  ? 'bg-red-500/20 text-red-400 border border-red-500/40'
                  : 'text-gray-500 hover:text-gray-300 hover:bg-gray-700'
              )}
              title="Whole Word (Alt+W)"
            >
              <WholeWord size={14} />
            </button>
            <button
              onClick={() => toggleOption('regex')}
              className={cn(
                'p-1.5 rounded transition-colors',
                search.options.regex
                  ? 'bg-red-500/20 text-red-400 border border-red-500/40'
                  : 'text-gray-500 hover:text-gray-300 hover:bg-gray-700'
              )}
              title="Regular Expression (Alt+R)"
            >
              <Regex size={14} />
            </button>
          </div>
        </div>

        {/* Replace row */}
        {showReplace && (
          <div className="flex items-center gap-1">
            <div className="flex-1 relative">
              <Replace className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-500" />
              <input
                ref={replaceInputRef}
                type="text"
                value={search.replaceText}
                onChange={(e) => search.setReplaceText(e.target.value)}
                onKeyDown={handleReplaceKeyDown}
                placeholder="Replace..."
                className="w-full bg-gray-800 border border-gray-600 rounded pl-8 pr-2 py-1.5 text-sm text-white placeholder:text-gray-500 focus:border-red-500 focus:outline-none focus:ring-1 focus:ring-red-500/30"
              />
            </div>
            <button
              onClick={() => search.replaceCurrentMatch()}
              className="p-1.5 text-gray-500 hover:text-gray-300 hover:bg-gray-700 rounded transition-colors"
              title="Replace (Enter)"
              disabled={search.totalMatchCount === 0}
            >
              <Replace size={14} />
            </button>
            <button
              onClick={() => search.replaceAllMatches()}
              className="p-1.5 text-gray-500 hover:text-gray-300 hover:bg-gray-700 rounded transition-colors"
              title="Replace All (Shift+Enter)"
              disabled={search.totalMatchCount === 0}
            >
              <ReplaceAll size={14} />
            </button>
          </div>
        )}

        {/* File filter row */}
        {showFileFilter && (
          <div className="relative">
            <Filter className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-500" />
            <input
              type="text"
              value={search.options.fileFilter}
              onChange={(e) => {
                search.setOptions({ fileFilter: e.target.value })
                if (search.query.trim()) {
                  if (debounceRef.current) clearTimeout(debounceRef.current)
                  debounceRef.current = setTimeout(() => search.search(), 300)
                }
              }}
              placeholder="File filter (e.g. *.ts, *.go)"
              className="w-full bg-gray-800 border border-gray-600 rounded pl-8 pr-2 py-1.5 text-sm text-white placeholder:text-gray-500 focus:border-red-500 focus:outline-none focus:ring-1 focus:ring-red-500/30"
            />
          </div>
        )}

        {/* Controls row */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-1">
            <button
              onClick={() => setShowReplace(!showReplace)}
              className={cn(
                'px-2 py-0.5 text-xs rounded transition-colors',
                showReplace
                  ? 'bg-gray-700 text-white'
                  : 'text-gray-500 hover:text-gray-300'
              )}
            >
              Replace
            </button>
            <button
              onClick={() => setShowFileFilter(!showFileFilter)}
              className={cn(
                'px-2 py-0.5 text-xs rounded transition-colors',
                showFileFilter
                  ? 'bg-gray-700 text-white'
                  : 'text-gray-500 hover:text-gray-300'
              )}
            >
              Filter
            </button>
          </div>

          <div className="flex items-center gap-2">
            {search.totalMatchCount > 0 && (
              <span className="text-xs text-gray-400">
                {search.currentMatchIndex + 1} / {search.totalMatchCount}
                {totalFiles > 0 && (
                  <span className="text-gray-600"> in {totalFiles} files</span>
                )}
              </span>
            )}

            {search.totalMatchCount > 0 && (
              <div className="flex items-center gap-0.5">
                <button
                  onClick={() => {
                    const result = search.previousMatch()
                    if (result) onFileOpen?.(result.file, result.line)
                  }}
                  className="p-1 text-gray-500 hover:text-gray-300 hover:bg-gray-700 rounded transition-colors"
                  title="Previous Match (Shift+Enter)"
                >
                  <ArrowUp size={12} />
                </button>
                <button
                  onClick={() => {
                    const result = search.nextMatch()
                    if (result) onFileOpen?.(result.file, result.line)
                  }}
                  className="p-1 text-gray-500 hover:text-gray-300 hover:bg-gray-700 rounded transition-colors"
                  title="Next Match (Enter)"
                >
                  <ArrowDown size={12} />
                </button>
              </div>
            )}

            {search.query && (
              <button
                onClick={() => search.clearSearch()}
                className="p-1 text-gray-500 hover:text-gray-300 hover:bg-gray-700 rounded transition-colors"
                title="Clear Search"
              >
                <RotateCcw size={12} />
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Results */}
      <div className="flex-1 overflow-y-auto">
        {/* Loading state */}
        {search.isSearching && (
          <div className="flex items-center justify-center p-6">
            <Loading size="md" variant="spinner" />
          </div>
        )}

        {/* Error state */}
        {search.error && (
          <div className="p-3">
            <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-3 text-sm text-red-400">
              {search.error}
            </div>
          </div>
        )}

        {/* No results */}
        {!search.isSearching && !search.error && search.query && search.results.length === 0 && (
          <div className="p-6 text-center">
            <Search className="w-8 h-8 mx-auto mb-2 text-gray-600" />
            <p className="text-sm text-gray-500">No results found</p>
            <p className="text-xs text-gray-600 mt-1">
              Try different search terms or adjust filters
            </p>
          </div>
        )}

        {/* Empty state */}
        {!search.query && !search.isSearching && (
          <div className="p-6 text-center">
            <Search className="w-8 h-8 mx-auto mb-2 text-gray-600" />
            <p className="text-sm text-gray-500">Search across all files</p>
            <p className="text-xs text-gray-600 mt-1">
              Use Cmd+Shift+F to focus
            </p>
          </div>
        )}

        {/* Results grouped by file */}
        {!search.isSearching && search.results.length > 0 && (
          <div className="py-1">
            {search.results.map((result) => {
              const isExpanded = expandedFiles.has(result.file.id)
              const ext = getFileExtension(result.file.name)

              return (
                <div key={result.file.id}>
                  {/* File header */}
                  <button
                    onClick={() => toggleFileExpanded(result.file.id)}
                    className="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-gray-800/50 transition-colors group"
                  >
                    {isExpanded ? (
                      <ChevronDown size={12} className="text-gray-500 shrink-0" />
                    ) : (
                      <ChevronRight size={12} className="text-gray-500 shrink-0" />
                    )}
                    <FileText size={12} className={cn('shrink-0', getFileStatusColor(ext))} />
                    <span className="text-sm text-white truncate flex-1 text-left">
                      {result.file.name}
                    </span>
                    <span className="text-xs text-gray-500 truncate max-w-[120px]" title={result.file.path}>
                      {result.file.path}
                    </span>
                    <Badge variant="outline" size="xs" className="shrink-0">
                      {result.matches.length}
                    </Badge>
                  </button>

                  {/* Match lines */}
                  {isExpanded && (
                    <div className="pl-6">
                      {result.matches.map((match, idx) => (
                        <button
                          key={`${result.file.id}-${match.line}-${idx}`}
                          onClick={() => handleResultClick(result.file, match.line)}
                          className="w-full flex items-start gap-2 px-3 py-1 hover:bg-gray-800/70 transition-colors text-left"
                        >
                          <span className="text-xs text-gray-600 font-mono shrink-0 w-8 text-right pt-0.5">
                            {match.line}
                          </span>
                          <div className="flex-1 overflow-hidden">
                            {renderHighlightedContent(match.content.trim(), {
                              ...match,
                              // Adjust start/end for trimmed content
                              start: match.start - (match.content.length - match.content.trimStart().length),
                              end: match.end - (match.content.length - match.content.trimStart().length),
                            })}
                          </div>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Footer with keyboard shortcuts hint */}
      {search.totalMatchCount > 0 && (
        <div className="px-3 py-1.5 border-t border-gray-700/50 flex items-center justify-between">
          <span className="text-xs text-gray-600">
            {search.totalMatchCount} match{search.totalMatchCount !== 1 ? 'es' : ''} in {totalFiles} file{totalFiles !== 1 ? 's' : ''}
          </span>
          {showReplace && (
            <span className="text-xs text-gray-600">
              Enter: replace | Shift+Enter: replace all
            </span>
          )}
        </div>
      )}
    </div>
  )
}

export default SearchPanel
