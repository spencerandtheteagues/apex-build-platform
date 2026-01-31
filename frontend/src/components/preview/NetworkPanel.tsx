import React, { useState, useRef, useEffect, useCallback, useMemo } from 'react'
import { Globe, Trash2, Filter, Search, CheckCircle, XCircle, Clock, ArrowDown, ArrowUp } from 'lucide-react'

export interface NetworkRequest {
  id: number
  method: string
  url: string
  status: number
  statusText: string
  duration: number
  error?: string
  timestamp: string
}

interface NetworkPanelProps {
  requests: NetworkRequest[]
  onClear: () => void
  className?: string
}

const methodColors: Record<string, string> = {
  GET: 'text-green-400 bg-green-500/20',
  POST: 'text-blue-400 bg-blue-500/20',
  PUT: 'text-yellow-400 bg-yellow-500/20',
  PATCH: 'text-orange-400 bg-orange-500/20',
  DELETE: 'text-red-400 bg-red-500/20',
  OPTIONS: 'text-purple-400 bg-purple-500/20',
  HEAD: 'text-gray-400 bg-gray-500/20',
}

const getStatusColor = (status: number) => {
  if (status === 0) return 'text-red-400'
  if (status < 300) return 'text-green-400'
  if (status < 400) return 'text-blue-400'
  if (status < 500) return 'text-yellow-400'
  return 'text-red-400'
}

const getStatusBg = (status: number) => {
  if (status === 0) return 'bg-red-500/10'
  if (status < 300) return 'bg-green-500/10'
  if (status < 400) return 'bg-blue-500/10'
  if (status < 500) return 'bg-yellow-500/10'
  return 'bg-red-500/10'
}

export default function NetworkPanel({ requests, onClear, className = '' }: NetworkPanelProps) {
  const [filter, setFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | '2xx' | '3xx' | '4xx' | '5xx' | 'error'>('all')
  const [showFilters, setShowFilters] = useState(false)
  const [sortBy, setSortBy] = useState<'time' | 'duration' | 'status'>('time')
  const [sortAsc, setSortAsc] = useState(false)
  const scrollRef = useRef<HTMLDivElement>(null)
  const [autoScroll, setAutoScroll] = useState(true)

  // Auto-scroll to bottom
  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [requests, autoScroll])

  // Memoized scroll handler to prevent recreation on each render
  const handleScroll = useCallback(() => {
    if (scrollRef.current) {
      const { scrollTop, scrollHeight, clientHeight } = scrollRef.current
      const isAtBottom = scrollHeight - scrollTop - clientHeight < 50
      setAutoScroll(isAtBottom)
    }
  }, [])

  // Memoized filtered requests to avoid recalculation on unrelated state changes
  const filteredRequests = useMemo(() => requests.filter(req => {
    // URL filter
    if (filter && !req.url.toLowerCase().includes(filter.toLowerCase())) return false

    // Status filter
    if (statusFilter !== 'all') {
      if (statusFilter === 'error' && req.status !== 0) return false
      if (statusFilter === '2xx' && (req.status < 200 || req.status >= 300)) return false
      if (statusFilter === '3xx' && (req.status < 300 || req.status >= 400)) return false
      if (statusFilter === '4xx' && (req.status < 400 || req.status >= 500)) return false
      if (statusFilter === '5xx' && req.status < 500) return false
    }

    return true
  }), [requests, filter, statusFilter])

  // Memoized sorted requests to avoid recalculation on unrelated state changes
  const sortedRequests = useMemo(() => [...filteredRequests].sort((a, b) => {
    let cmp = 0
    if (sortBy === 'time') {
      cmp = new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
    } else if (sortBy === 'duration') {
      cmp = a.duration - b.duration
    } else if (sortBy === 'status') {
      cmp = a.status - b.status
    }
    return sortAsc ? cmp : -cmp
  }), [filteredRequests, sortBy, sortAsc])

  const formatTime = (timestamp: string) => {
    try {
      const date = new Date(timestamp)
      return date.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
    } catch {
      return timestamp
    }
  }

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`
    return `${(ms / 1000).toFixed(2)}s`
  }

  const truncateUrl = (url: string, maxLength: number = 60) => {
    if (url.length <= maxLength) return url
    return url.substring(0, maxLength - 3) + '...'
  }

  // Memoized stats calculation to avoid recalculation on unrelated state changes
  const stats = useMemo(() => ({
    total: requests.length,
    success: requests.filter(r => r.status >= 200 && r.status < 300).length,
    errors: requests.filter(r => r.status === 0 || r.status >= 400).length,
    avgDuration: requests.length > 0
      ? Math.round(requests.reduce((sum, r) => sum + r.duration, 0) / requests.length)
      : 0
  }), [requests])

  const handleSort = (field: 'time' | 'duration' | 'status') => {
    if (sortBy === field) {
      setSortAsc(!sortAsc)
    } else {
      setSortBy(field)
      setSortAsc(false)
    }
  }

  return (
    <div className={`flex flex-col bg-gray-900 relative ${className}`}>
      {/* Toolbar */}
      <div className="flex items-center justify-between px-3 py-2 bg-gray-800/50 border-b border-gray-700">
        <div className="flex items-center gap-2">
          {/* Filter toggle */}
          <button
            onClick={() => setShowFilters(!showFilters)}
            className={`flex items-center gap-1 px-2 py-1 rounded text-xs ${
              showFilters ? 'bg-cyan-600 text-white' : 'bg-gray-700 text-gray-400 hover:text-white'
            }`}
          >
            <Filter className="w-3 h-3" />
            Filter
          </button>

          {/* Stats */}
          <div className="flex items-center gap-3 text-xs">
            <span className="text-gray-400">{stats.total} requests</span>
            {stats.success > 0 && (
              <span className="flex items-center gap-1 text-green-400">
                <CheckCircle className="w-3 h-3" />
                {stats.success}
              </span>
            )}
            {stats.errors > 0 && (
              <span className="flex items-center gap-1 text-red-400">
                <XCircle className="w-3 h-3" />
                {stats.errors}
              </span>
            )}
            <span className="flex items-center gap-1 text-gray-500">
              <Clock className="w-3 h-3" />
              avg {formatDuration(stats.avgDuration)}
            </span>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {/* Search */}
          <div className="relative">
            <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-3 h-3 text-gray-500" />
            <input
              type="text"
              placeholder="Filter URL..."
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="w-40 pl-7 pr-2 py-1 bg-gray-800 border border-gray-700 rounded text-xs text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500"
            />
          </div>

          {/* Clear button */}
          <button
            onClick={onClear}
            className="flex items-center gap-1 px-2 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-400 hover:text-white transition-colors"
          >
            <Trash2 className="w-3 h-3" />
            Clear
          </button>
        </div>
      </div>

      {/* Filter bar */}
      {showFilters && (
        <div className="flex items-center gap-2 px-3 py-2 bg-gray-800/30 border-b border-gray-700">
          {(['all', '2xx', '3xx', '4xx', '5xx', 'error'] as const).map((status) => (
            <button
              key={status}
              onClick={() => setStatusFilter(status)}
              className={`px-2 py-1 rounded text-xs transition-colors ${
                statusFilter === status
                  ? status === 'error' ? 'bg-red-500/20 text-red-400'
                    : status === '2xx' ? 'bg-green-500/20 text-green-400'
                    : status === '3xx' ? 'bg-blue-500/20 text-blue-400'
                    : status === '4xx' ? 'bg-yellow-500/20 text-yellow-400'
                    : status === '5xx' ? 'bg-red-500/20 text-red-400'
                    : 'bg-cyan-600 text-white'
                  : 'bg-gray-800 text-gray-500 hover:text-white'
              }`}
            >
              {status === 'all' ? 'All' : status === 'error' ? 'Errors' : status}
            </button>
          ))}
        </div>
      )}

      {/* Table header */}
      <div className="flex items-center gap-2 px-3 py-1.5 bg-gray-800/30 border-b border-gray-700 text-xs font-medium text-gray-500">
        <div className="w-16">Method</div>
        <div className="flex-1">URL</div>
        <button
          onClick={() => handleSort('status')}
          className="w-16 flex items-center gap-1 hover:text-white"
        >
          Status
          {sortBy === 'status' && (sortAsc ? <ArrowUp className="w-3 h-3" /> : <ArrowDown className="w-3 h-3" />)}
        </button>
        <button
          onClick={() => handleSort('duration')}
          className="w-20 flex items-center gap-1 hover:text-white"
        >
          Duration
          {sortBy === 'duration' && (sortAsc ? <ArrowUp className="w-3 h-3" /> : <ArrowDown className="w-3 h-3" />)}
        </button>
        <button
          onClick={() => handleSort('time')}
          className="w-20 flex items-center gap-1 hover:text-white"
        >
          Time
          {sortBy === 'time' && (sortAsc ? <ArrowUp className="w-3 h-3" /> : <ArrowDown className="w-3 h-3" />)}
        </button>
      </div>

      {/* Network requests */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto font-mono text-xs"
      >
        {sortedRequests.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-gray-500">
            <Globe className="w-8 h-8 mb-2 opacity-30" />
            <p>No network requests</p>
          </div>
        ) : (
          sortedRequests.map((req) => (
            <div
              key={`${req.id}-${req.timestamp}`}
              className={`flex items-center gap-2 px-3 py-1.5 border-b border-gray-800/50 hover:bg-gray-800/30 ${getStatusBg(req.status)}`}
            >
              {/* Method */}
              <div className="w-16">
                <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${methodColors[req.method] || 'text-gray-400 bg-gray-500/20'}`}>
                  {req.method}
                </span>
              </div>

              {/* URL */}
              <div className="flex-1 min-w-0">
                <span className="text-gray-300 truncate block" title={req.url}>
                  {truncateUrl(req.url)}
                </span>
                {req.error && (
                  <span className="text-red-400 text-[10px]">{req.error}</span>
                )}
              </div>

              {/* Status */}
              <div className="w-16">
                <span className={`${getStatusColor(req.status)}`}>
                  {req.status === 0 ? 'Error' : req.status}
                </span>
              </div>

              {/* Duration */}
              <div className="w-20 text-gray-500">
                {formatDuration(req.duration)}
              </div>

              {/* Time */}
              <div className="w-20 text-gray-600">
                {formatTime(req.timestamp)}
              </div>
            </div>
          ))
        )}
      </div>

      {/* Auto-scroll indicator */}
      {!autoScroll && requests.length > 0 && (
        <button
          onClick={() => {
            setAutoScroll(true)
            if (scrollRef.current) {
              scrollRef.current.scrollTop = scrollRef.current.scrollHeight
            }
          }}
          className="absolute bottom-4 right-4 px-3 py-1.5 bg-cyan-600 hover:bg-cyan-500 rounded-full text-xs text-white shadow-lg"
        >
          Scroll to bottom
        </button>
      )}
    </div>
  )
}
