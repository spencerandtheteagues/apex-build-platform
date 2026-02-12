import React, { useEffect, useRef, useState, useCallback, useMemo } from 'react'
import { AlertCircle, AlertTriangle, Info, Bug, Terminal, Trash2, Filter, Search, ChevronDown, ChevronRight } from 'lucide-react'

export interface ConsoleEntry {
  id: string
  level: 'log' | 'warn' | 'error' | 'info' | 'debug'
  message: string
  stack?: string
  timestamp: string
}

interface ConsolePanelProps {
  entries: ConsoleEntry[]
  onClear: () => void
  className?: string
}

const levelConfig = {
  log: {
    icon: Terminal,
    color: 'text-gray-300',
    bg: 'bg-transparent',
    label: 'Log'
  },
  info: {
    icon: Info,
    color: 'text-blue-400',
    bg: 'bg-blue-500/10',
    label: 'Info'
  },
  warn: {
    icon: AlertTriangle,
    color: 'text-yellow-400',
    bg: 'bg-yellow-500/10',
    label: 'Warn'
  },
  error: {
    icon: AlertCircle,
    color: 'text-red-400',
    bg: 'bg-red-500/10',
    label: 'Error'
  },
  debug: {
    icon: Bug,
    color: 'text-purple-400',
    bg: 'bg-purple-500/10',
    label: 'Debug'
  }
}

export default function ConsolePanel({ entries, onClear, className = '' }: ConsolePanelProps) {
  const [filter, setFilter] = useState<string>('')
  const [levelFilter, setLevelFilter] = useState<Set<string>>(new Set(['log', 'info', 'warn', 'error', 'debug']))
  const [showFilters, setShowFilters] = useState(false)
  const [expandedErrors, setExpandedErrors] = useState<Set<string>>(new Set())
  const scrollRef = useRef<HTMLDivElement>(null)
  const [autoScroll, setAutoScroll] = useState(true)

  // Auto-scroll to bottom when new entries arrive
  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [entries, autoScroll])

  // Detect if user scrolled up manually - memoized to prevent recreation on each render
  const handleScroll = useCallback(() => {
    if (scrollRef.current) {
      const { scrollTop, scrollHeight, clientHeight } = scrollRef.current
      const isAtBottom = scrollHeight - scrollTop - clientHeight < 50
      setAutoScroll(isAtBottom)
    }
  }, [])

  // Memoized filtered entries to avoid recalculation on unrelated state changes
  const filteredEntries = useMemo(() => entries.filter(entry => {
    if (!levelFilter.has(entry.level)) return false
    if (filter && !entry.message.toLowerCase().includes(filter.toLowerCase())) return false
    return true
  }), [entries, levelFilter, filter])

  const toggleLevel = (level: string) => {
    const newFilter = new Set(levelFilter)
    if (newFilter.has(level)) {
      newFilter.delete(level)
    } else {
      newFilter.add(level)
    }
    setLevelFilter(newFilter)
  }

  const toggleError = (id: string) => {
    const newExpanded = new Set(expandedErrors)
    if (newExpanded.has(id)) {
      newExpanded.delete(id)
    } else {
      newExpanded.add(id)
    }
    setExpandedErrors(newExpanded)
  }

  const formatTime = (timestamp: string) => {
    try {
      const date = new Date(timestamp)
      const baseTime = date.toLocaleTimeString('en-US', {
        hour12: false,
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })
      const millis = date.getMilliseconds().toString().padStart(3, '0')
      return `${baseTime}.${millis}`
    } catch {
      return timestamp
    }
  }

  const levelCounts = entries.reduce((acc, entry) => {
    acc[entry.level] = (acc[entry.level] || 0) + 1
    return acc
  }, {} as Record<string, number>)

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

          {/* Level badges */}
          <div className="flex items-center gap-1">
            {Object.entries(levelCounts).map(([level, count]) => {
              const config = levelConfig[level as keyof typeof levelConfig]
              return (
                <button
                  key={level}
                  onClick={() => toggleLevel(level)}
                  className={`flex items-center gap-1 px-2 py-0.5 rounded text-xs transition-opacity ${
                    levelFilter.has(level) ? config.color : 'text-gray-600 opacity-50'
                  }`}
                >
                  <span className={`w-2 h-2 rounded-full ${level === 'error' ? 'bg-red-500' : level === 'warn' ? 'bg-yellow-500' : level === 'info' ? 'bg-blue-500' : 'bg-gray-500'}`} />
                  {count}
                </button>
              )
            })}
          </div>
        </div>

        <div className="flex items-center gap-2">
          {/* Search */}
          <div className="relative">
            <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-3 h-3 text-gray-500" />
            <input
              type="text"
              placeholder="Filter..."
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="w-32 pl-7 pr-2 py-1 bg-gray-800 border border-gray-700 rounded text-xs text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500"
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
          {Object.entries(levelConfig).map(([level, config]) => (
            <button
              key={level}
              onClick={() => toggleLevel(level)}
              className={`flex items-center gap-1.5 px-2 py-1 rounded text-xs transition-colors ${
                levelFilter.has(level)
                  ? `${config.bg} ${config.color}`
                  : 'bg-gray-800 text-gray-500'
              }`}
            >
              <config.icon className="w-3 h-3" />
              {config.label}
            </button>
          ))}
        </div>
      )}

      {/* Console entries */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto font-mono text-xs"
      >
        {filteredEntries.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-gray-500">
            <Terminal className="w-8 h-8 mb-2 opacity-30" />
            <p>No console output</p>
          </div>
        ) : (
          filteredEntries.map((entry) => {
            const config = levelConfig[entry.level]
            const Icon = config.icon
            const hasStack = entry.stack && entry.level === 'error'
            const isExpanded = expandedErrors.has(entry.id)

            return (
              <div
                key={entry.id}
                className={`flex items-start gap-2 px-3 py-1.5 border-b border-gray-800/50 hover:bg-gray-800/30 ${config.bg}`}
              >
                {/* Expand button for errors with stack */}
                {hasStack ? (
                  <button
                    onClick={() => toggleError(entry.id)}
                    className="mt-0.5 text-gray-500 hover:text-white"
                  >
                    {isExpanded ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                  </button>
                ) : (
                  <Icon className={`w-3 h-3 mt-0.5 ${config.color}`} />
                )}

                <div className="flex-1 min-w-0">
                  <div className="flex items-start gap-2">
                    {hasStack && <Icon className={`w-3 h-3 mt-0.5 flex-shrink-0 ${config.color}`} />}
                    <span className={`whitespace-pre-wrap break-all ${config.color}`}>
                      {entry.message}
                    </span>
                  </div>

                  {/* Stack trace */}
                  {hasStack && isExpanded && (
                    <pre className="mt-2 pl-5 text-gray-500 whitespace-pre-wrap text-[10px] leading-relaxed">
                      {entry.stack}
                    </pre>
                  )}
                </div>

                <span className="text-gray-600 flex-shrink-0">
                  {formatTime(entry.timestamp)}
                </span>
              </div>
            )
          })
        )}
      </div>

      {/* Auto-scroll indicator */}
      {!autoScroll && entries.length > 0 && (
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
