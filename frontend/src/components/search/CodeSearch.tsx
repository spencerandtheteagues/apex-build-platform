import React, { useState, useCallback, useRef, useEffect } from 'react'
import {
  Search,
  FileCode,
  Code2,
  Replace,
  Settings2,
  X,
  ChevronDown,
  ChevronRight,
  FileText,
  Hash,
  Clock,
  ArrowRight,
  RefreshCw,
  Filter,
  CaseSensitive,
  Regex,
  WholeWord,
  Folder,
  AlertCircle
} from 'lucide-react'
import api from '../../services/api'

interface SearchMatch {
  line_number: number
  column_start: number
  column_end: number
  content: string
  context_before?: string[]
  context_after?: string[]
  match_text: string
}

interface FileResult {
  file_id: number
  file_name: string
  file_path: string
  language: string
  matches: SearchMatch[]
  score: number
}

interface SearchResults {
  query: string
  total_matches: number
  file_matches: number
  files: FileResult[]
  suggestions?: string[]
  duration: number
  truncated: boolean
  stats?: {
    files_searched: number
    lines_searched: number
    bytes_searched: number
    matches_by_type: Record<string, number>
    top_files: string[]
  }
}

interface SymbolResult {
  name: string
  kind: string
  file_id?: number
  file_path: string
  line_number: number
  signature?: string
  container?: string
}

interface CodeSearchProps {
  projectId?: number
  onFileSelect?: (fileId: number, line?: number) => void
  isOpen?: boolean
  onClose?: () => void
}

const languageColors: Record<string, string> = {
  go: 'text-cyan-400',
  typescript: 'text-blue-400',
  javascript: 'text-yellow-400',
  python: 'text-green-400',
  rust: 'text-orange-400',
  java: 'text-red-400',
  css: 'text-pink-400',
  html: 'text-orange-300',
  json: 'text-gray-400',
  markdown: 'text-purple-400',
  default: 'text-gray-400'
}

const symbolIcons: Record<string, React.ReactNode> = {
  function: <Code2 className="w-4 h-4 text-yellow-400" />,
  class: <Hash className="w-4 h-4 text-blue-400" />,
  interface: <FileCode className="w-4 h-4 text-purple-400" />,
  type: <FileCode className="w-4 h-4 text-cyan-400" />,
  struct: <Hash className="w-4 h-4 text-green-400" />,
  constant: <FileText className="w-4 h-4 text-orange-400" />,
  variable: <FileText className="w-4 h-4 text-gray-400" />,
  default: <Code2 className="w-4 h-4 text-gray-400" />
}

export default function CodeSearch({ projectId, onFileSelect, isOpen = true, onClose }: CodeSearchProps) {
  const [query, setQuery] = useState('')
  const [searchType, setSearchType] = useState<'content' | 'files' | 'symbols'>('content')
  const [results, setResults] = useState<SearchResults | null>(null)
  const [symbols, setSymbols] = useState<SymbolResult[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [expandedFiles, setExpandedFiles] = useState<Set<number>>(new Set())
  const [showOptions, setShowOptions] = useState(false)
  const [showReplace, setShowReplace] = useState(false)
  const [replaceText, setReplaceText] = useState('')
  const [replacePreview, setReplacePreview] = useState<any>(null)

  // Search options
  const [caseSensitive, setCaseSensitive] = useState(false)
  const [wholeWord, setWholeWord] = useState(false)
  const [useRegex, setUseRegex] = useState(false)
  const [fileTypes, setFileTypes] = useState<string[]>([])
  const [excludePaths, setExcludePaths] = useState<string[]>([])

  const searchInputRef = useRef<HTMLInputElement>(null)
  const debounceRef = useRef<NodeJS.Timeout | null>(null)

  // Focus search input on open
  useEffect(() => {
    if (isOpen && searchInputRef.current) {
      searchInputRef.current.focus()
    }
  }, [isOpen])

  // Keyboard shortcut (Cmd/Ctrl + Shift + F)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.shiftKey && e.key === 'f') {
        e.preventDefault()
        searchInputRef.current?.focus()
      }
      if (e.key === 'Escape' && onClose) {
        onClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  const performSearch = useCallback(async (searchQuery: string) => {
    if (!searchQuery.trim()) {
      setResults(null)
      setSymbols([])
      return
    }

    setLoading(true)
    setError(null)

    try {
      if (searchType === 'symbols') {
        const response = await api.get('/search/symbols', {
          params: {
            q: searchQuery,
            project_id: projectId,
            limit: 100
          }
        })
        setSymbols(response.data.symbols || [])
        setResults(null)
      } else if (searchType === 'files') {
        const response = await api.get('/search/files', {
          params: {
            q: searchQuery,
            project_id: projectId,
            limit: 100
          }
        })
        // Convert to results format
        setResults({
          query: searchQuery,
          total_matches: response.data.count,
          file_matches: response.data.count,
          files: response.data.files.map((f: any) => ({
            file_id: f.id,
            file_name: f.name,
            file_path: f.path,
            language: f.language,
            matches: [],
            score: f.score
          })),
          duration: 0,
          truncated: false
        })
        setSymbols([])
      } else {
        const response = await api.post('/search', {
          query: searchQuery,
          project_id: projectId,
          case_sensitive: caseSensitive,
          whole_word: wholeWord,
          use_regex: useRegex,
          file_types: fileTypes.length > 0 ? fileTypes : undefined,
          exclude_paths: excludePaths.length > 0 ? excludePaths : undefined,
          include_content: true,
          context_lines: 2,
          max_results: 100,
          search_type: 'content'
        })
        setResults(response.data.results)
        setSymbols([])

        // Auto-expand first few files
        if (response.data.results?.files) {
          const expanded = new Set<number>()
          response.data.results.files.slice(0, 3).forEach((f: FileResult) => {
            expanded.add(f.file_id)
          })
          setExpandedFiles(expanded)
        }
      }
    } catch (err: any) {
      setError(err.response?.data?.error || 'Search failed')
    } finally {
      setLoading(false)
    }
  }, [projectId, searchType, caseSensitive, wholeWord, useRegex, fileTypes, excludePaths])

  const handleSymbolClick = useCallback((symbol: SymbolResult) => {
    if (!onFileSelect) return
    if (!symbol.file_id) {
      setError('Unable to open symbol: missing file reference. Please retry the search.')
      return
    }
    onFileSelect(symbol.file_id, symbol.line_number)
  }, [onFileSelect])

  // Debounced search
  const handleSearchChange = (value: string) => {
    setQuery(value)

    if (debounceRef.current) {
      clearTimeout(debounceRef.current)
    }

    debounceRef.current = setTimeout(() => {
      performSearch(value)
    }, 300)
  }

  const handleReplace = async (apply: boolean = false) => {
    if (!query || !projectId) return

    setLoading(true)
    try {
      const response = await api.post('/search/replace', {
        project_id: projectId,
        search: query,
        replace: replaceText,
        case_sensitive: caseSensitive,
        whole_word: wholeWord,
        use_regex: useRegex,
        file_types: fileTypes.length > 0 ? fileTypes : undefined,
        preview: !apply
      })

      if (apply) {
        setReplacePreview(null)
        performSearch(query) // Refresh results
      } else {
        setReplacePreview(response.data.results)
      }
    } catch (err: any) {
      setError(err.response?.data?.error || 'Replace failed')
    } finally {
      setLoading(false)
    }
  }

  const toggleFileExpanded = (fileId: number) => {
    setExpandedFiles(prev => {
      const next = new Set(prev)
      if (next.has(fileId)) {
        next.delete(fileId)
      } else {
        next.add(fileId)
      }
      return next
    })
  }

  const highlightMatch = (content: string, matchText: string) => {
    if (!matchText) return content
    const parts = content.split(new RegExp(`(${matchText.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi'))
    return parts.map((part, i) =>
      part.toLowerCase() === matchText.toLowerCase()
        ? <span key={i} className="bg-yellow-500/30 text-yellow-200 font-medium">{part}</span>
        : part
    )
  }

  if (!isOpen) return null

  return (
    <div className="flex flex-col h-full bg-gray-900/95 border-l border-gray-700">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700">
        <div className="flex items-center gap-2 text-white font-medium">
          <Search className="w-5 h-5 text-cyan-400" />
          Search
        </div>
        {onClose && (
          <button onClick={onClose} className="p-1 hover:bg-gray-700 rounded">
            <X className="w-4 h-4 text-gray-400" />
          </button>
        )}
      </div>

      {/* Search Input */}
      <div className="p-3 border-b border-gray-700">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
          <input
            ref={searchInputRef}
            type="text"
            value={query}
            onChange={(e) => handleSearchChange(e.target.value)}
            placeholder="Search in files..."
            className="w-full pl-10 pr-20 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white placeholder-gray-500 focus:border-cyan-500 focus:outline-none"
          />
          <div className="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-1">
            <button
              onClick={() => setCaseSensitive(!caseSensitive)}
              className={`p-1 rounded ${caseSensitive ? 'bg-cyan-600 text-white' : 'text-gray-500 hover:text-gray-300'}`}
              title="Match Case"
            >
              <CaseSensitive className="w-4 h-4" />
            </button>
            <button
              onClick={() => setWholeWord(!wholeWord)}
              className={`p-1 rounded ${wholeWord ? 'bg-cyan-600 text-white' : 'text-gray-500 hover:text-gray-300'}`}
              title="Whole Word"
            >
              <WholeWord className="w-4 h-4" />
            </button>
            <button
              onClick={() => setUseRegex(!useRegex)}
              className={`p-1 rounded ${useRegex ? 'bg-cyan-600 text-white' : 'text-gray-500 hover:text-gray-300'}`}
              title="Use Regular Expression"
            >
              <Regex className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Search Type Tabs */}
        <div className="flex items-center gap-1 mt-2">
          <button
            onClick={() => { setSearchType('content'); performSearch(query) }}
            className={`px-3 py-1.5 text-sm rounded-md transition-colors ${
              searchType === 'content'
                ? 'bg-cyan-600 text-white'
                : 'text-gray-400 hover:text-white hover:bg-gray-700'
            }`}
          >
            <FileText className="w-4 h-4 inline mr-1" />
            Content
          </button>
          <button
            onClick={() => { setSearchType('files'); performSearch(query) }}
            className={`px-3 py-1.5 text-sm rounded-md transition-colors ${
              searchType === 'files'
                ? 'bg-cyan-600 text-white'
                : 'text-gray-400 hover:text-white hover:bg-gray-700'
            }`}
          >
            <Folder className="w-4 h-4 inline mr-1" />
            Files
          </button>
          <button
            onClick={() => { setSearchType('symbols'); performSearch(query) }}
            className={`px-3 py-1.5 text-sm rounded-md transition-colors ${
              searchType === 'symbols'
                ? 'bg-cyan-600 text-white'
                : 'text-gray-400 hover:text-white hover:bg-gray-700'
            }`}
          >
            <Code2 className="w-4 h-4 inline mr-1" />
            Symbols
          </button>
          <div className="ml-auto flex items-center gap-1">
            <button
              onClick={() => setShowReplace(!showReplace)}
              className={`p-1.5 rounded ${showReplace ? 'bg-cyan-600 text-white' : 'text-gray-500 hover:text-gray-300'}`}
              title="Replace"
            >
              <Replace className="w-4 h-4" />
            </button>
            <button
              onClick={() => setShowOptions(!showOptions)}
              className={`p-1.5 rounded ${showOptions ? 'bg-cyan-600 text-white' : 'text-gray-500 hover:text-gray-300'}`}
              title="Search Options"
            >
              <Settings2 className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Replace Input */}
        {showReplace && (
          <div className="mt-2 flex items-center gap-2">
            <div className="relative flex-1">
              <Replace className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
              <input
                type="text"
                value={replaceText}
                onChange={(e) => setReplaceText(e.target.value)}
                placeholder="Replace with..."
                className="w-full pl-10 pr-4 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white placeholder-gray-500 focus:border-cyan-500 focus:outline-none"
              />
            </div>
            <button
              onClick={() => handleReplace(false)}
              className="px-3 py-2 bg-gray-700 hover:bg-gray-600 text-white text-sm rounded-md"
            >
              Preview
            </button>
            <button
              onClick={() => handleReplace(true)}
              className="px-3 py-2 bg-cyan-600 hover:bg-cyan-500 text-white text-sm rounded-md"
            >
              Replace All
            </button>
          </div>
        )}

        {/* Advanced Options */}
        {showOptions && (
          <div className="mt-3 p-3 bg-gray-800/50 rounded-lg border border-gray-700">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs text-gray-400 mb-1">File Types (comma separated)</label>
                <input
                  type="text"
                  value={fileTypes.join(', ')}
                  onChange={(e) => setFileTypes(e.target.value.split(',').map(s => s.trim()).filter(Boolean))}
                  placeholder=".ts, .js, .go"
                  className="w-full px-3 py-1.5 bg-gray-900 border border-gray-700 rounded text-sm text-white"
                />
              </div>
              <div>
                <label className="block text-xs text-gray-400 mb-1">Exclude Paths</label>
                <input
                  type="text"
                  value={excludePaths.join(', ')}
                  onChange={(e) => setExcludePaths(e.target.value.split(',').map(s => s.trim()).filter(Boolean))}
                  placeholder="node_modules, dist"
                  className="w-full px-3 py-1.5 bg-gray-900 border border-gray-700 rounded text-sm text-white"
                />
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Results */}
      <div className="flex-1 overflow-y-auto">
        {loading && (
          <div className="flex items-center justify-center py-8">
            <RefreshCw className="w-5 h-5 text-cyan-500 animate-spin" />
          </div>
        )}

        {error && (
          <div className="m-3 p-3 bg-red-500/20 border border-red-500/50 rounded-lg flex items-center gap-2 text-red-400 text-sm">
            <AlertCircle className="w-4 h-4" />
            {error}
          </div>
        )}

        {/* Replace Preview */}
        {replacePreview && (
          <div className="m-3 p-3 bg-yellow-500/10 border border-yellow-500/30 rounded-lg">
            <div className="text-sm text-yellow-400 mb-2">
              Will modify {replacePreview.files_modified} files ({replacePreview.total_replaces} replacements)
            </div>
            {replacePreview.modified_files?.slice(0, 5).map((file: any) => (
              <div key={file.file_id} className="text-xs text-gray-400 mt-1">
                {file.file_path} ({file.replacements} changes)
              </div>
            ))}
          </div>
        )}

        {/* Symbol Results */}
        {searchType === 'symbols' && symbols.length > 0 && (
          <div className="p-2">
            {symbols.map((symbol, idx) => (
              <button
                key={idx}
                onClick={() => handleSymbolClick(symbol)}
                className="w-full flex items-center gap-2 px-3 py-2 hover:bg-gray-800 rounded-lg text-left group"
              >
                {symbolIcons[symbol.kind] || symbolIcons.default}
                <div className="flex-1 min-w-0">
                  <div className="text-white font-mono text-sm truncate">{symbol.name}</div>
                  <div className="text-xs text-gray-500 truncate">{symbol.file_path}:{symbol.line_number}</div>
                </div>
                <span className="text-xs px-2 py-0.5 bg-gray-700 rounded text-gray-400">
                  {symbol.kind}
                </span>
              </button>
            ))}
          </div>
        )}

        {/* Content/File Results */}
        {results && results.files.length > 0 && (
          <div className="p-2">
            {/* Stats */}
            <div className="flex items-center justify-between px-2 py-1 text-xs text-gray-500 mb-2">
              <span>
                {results.total_matches} results in {results.file_matches} files
              </span>
              {results.duration && (
                <span className="flex items-center gap-1">
                  <Clock className="w-3 h-3" />
                  {(results.duration / 1000000).toFixed(0)}ms
                </span>
              )}
            </div>

            {/* File Results */}
            {results.files.map((file) => (
              <div key={file.file_id} className="mb-2">
                <button
                  onClick={() => toggleFileExpanded(file.file_id)}
                  className="w-full flex items-center gap-2 px-2 py-1.5 hover:bg-gray-800 rounded text-left"
                >
                  {expandedFiles.has(file.file_id) ? (
                    <ChevronDown className="w-4 h-4 text-gray-500" />
                  ) : (
                    <ChevronRight className="w-4 h-4 text-gray-500" />
                  )}
                  <FileCode className={`w-4 h-4 ${languageColors[file.language] || languageColors.default}`} />
                  <span className="flex-1 text-sm text-white truncate">{file.file_path}</span>
                  <span className="text-xs px-1.5 py-0.5 bg-gray-700 rounded text-gray-400">
                    {file.matches.length}
                  </span>
                </button>

                {/* Matches */}
                {expandedFiles.has(file.file_id) && file.matches.length > 0 && (
                  <div className="ml-6 mt-1 space-y-1">
                    {file.matches.slice(0, 20).map((match, idx) => (
                      <button
                        key={idx}
                        onClick={() => onFileSelect?.(file.file_id, match.line_number)}
                        className="w-full flex items-start gap-2 px-2 py-1 hover:bg-gray-800/50 rounded text-left group"
                      >
                        <span className="text-xs text-gray-500 font-mono w-8 text-right flex-shrink-0">
                          {match.line_number}
                        </span>
                        <code className="flex-1 text-xs text-gray-300 truncate font-mono">
                          {highlightMatch(match.content, match.match_text)}
                        </code>
                        <ArrowRight className="w-3 h-3 text-gray-600 opacity-0 group-hover:opacity-100" />
                      </button>
                    ))}
                    {file.matches.length > 20 && (
                      <div className="text-xs text-gray-500 px-2 py-1">
                        +{file.matches.length - 20} more matches
                      </div>
                    )}
                  </div>
                )}
              </div>
            ))}

            {results.truncated && (
              <div className="text-center py-2 text-xs text-gray-500">
                Results truncated. Refine your search for more specific results.
              </div>
            )}
          </div>
        )}

        {/* Empty State */}
        {!loading && query && results?.files.length === 0 && symbols.length === 0 && (
          <div className="flex flex-col items-center justify-center py-12 text-gray-500">
            <Search className="w-12 h-12 mb-3 opacity-30" />
            <p>No results found for "{query}"</p>
            {results?.suggestions && results.suggestions.length > 0 && (
              <div className="mt-4 text-sm">
                <p className="text-gray-400 mb-2">Suggestions:</p>
                <ul className="list-disc list-inside">
                  {results.suggestions.map((s, i) => (
                    <li key={i}>{s}</li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}

        {/* Initial State */}
        {!loading && !query && (
          <div className="flex flex-col items-center justify-center py-12 text-gray-500">
            <Search className="w-12 h-12 mb-3 opacity-30" />
            <p>Search across all files</p>
            <p className="text-xs mt-2">Press Cmd/Ctrl + Shift + F to focus</p>
          </div>
        )}
      </div>
    </div>
  )
}
