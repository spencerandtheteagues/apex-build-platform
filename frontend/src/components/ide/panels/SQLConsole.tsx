// APEX.BUILD SQL Console
// Interactive SQL query execution with results visualization

import React, { useState, useEffect } from 'react'
import { Play, RotateCcw, Download, Table, Database } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button, Loading, Badge } from '@/components/ui'
import apiService from '@/services/api'

interface SQLConsoleProps {
  projectId: number
  dbId: number
  onExecute?: (query: string) => void
  className?: string
}

export const SQLConsole: React.FC<SQLConsoleProps> = ({
  projectId,
  dbId,
  onExecute,
  className
}) => {
  const [query, setQuery] = useState('SELECT * FROM information_schema.tables LIMIT 10;')
  const [results, setResults] = useState<{
    columns: string[]
    rows: any[][]
    affected_rows: number
    duration_ms: number
  } | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [history, setHistory] = useState<string[]>([])

  const executeQuery = async () => {
    if (!query.trim()) return

    setLoading(true)
    setError(null)
    try {
      const data = await apiService.executeSQLQuery(projectId, dbId, query)
      setResults(data.result)
      setHistory(prev => [query, ...prev].slice(0, 10))
      onExecute?.(query)
    } catch (err: any) {
      console.error('Query failed:', err)
      setError(err.response?.data?.error || 'Query execution failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className={cn("flex flex-col h-full bg-gray-950", className)}>
      {/* Toolbar */}
      <div className="flex items-center justify-between p-2 bg-gray-900 border-b border-gray-800">
        <div className="flex items-center gap-2">
          <Database className="w-4 h-4 text-cyan-400" />
          <span className="text-sm font-medium text-white">SQL Console</span>
        </div>
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="ghost"
            onClick={() => setQuery('')}
            icon={<RotateCcw size={14} />}
            disabled={loading}
          >
            Clear
          </Button>
          <Button
            size="sm"
            variant="success"
            onClick={executeQuery}
            loading={loading}
            icon={<Play size={14} />}
            disabled={!query.trim()}
          >
            Run Query
          </Button>
        </div>
      </div>

      {/* Editor Area */}
      <div className="h-40 border-b border-gray-800 relative">
        <textarea
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="w-full h-full bg-gray-900/50 p-3 text-sm font-mono text-green-400 focus:outline-none resize-none"
          placeholder="Enter SQL query..."
          spellCheck={false}
        />
      </div>

      {/* Results Area */}
      <div className="flex-1 overflow-auto bg-gray-900/30 p-2">
        {loading ? (
          <div className="flex items-center justify-center h-full">
            <Loading text="Executing query..." />
          </div>
        ) : error ? (
          <div className="p-4 text-red-400 bg-red-900/10 border border-red-900/50 rounded">
            <h4 className="font-bold mb-1">Error Executing Query</h4>
            <pre className="text-xs whitespace-pre-wrap">{error}</pre>
          </div>
        ) : results ? (
          <div className="h-full flex flex-col">
            <div className="flex items-center justify-between mb-2 px-1">
              <div className="flex items-center gap-3 text-xs text-gray-400">
                <span>{results.affected_rows} rows affected</span>
                <span>{results.duration_ms}ms</span>
              </div>
              <Button size="xs" variant="ghost" icon={<Download size={12} />}>
                CSV
              </Button>
            </div>
            
            {results.rows.length > 0 ? (
              <div className="overflow-auto border border-gray-700 rounded-lg">
                <table className="w-full text-left text-xs border-collapse">
                  <thead className="bg-gray-800 text-gray-300 sticky top-0">
                    <tr>
                      {results.columns.map((col, i) => (
                        <th key={i} className="p-2 border-b border-r border-gray-700 font-medium whitespace-nowrap">
                          {col}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-800 text-gray-300 bg-gray-900/50">
                    {results.rows.map((row, i) => (
                      <tr key={i} className="hover:bg-gray-800/50">
                        {row.map((cell, j) => (
                          <td key={j} className="p-2 border-r border-gray-800 whitespace-nowrap max-w-xs truncate">
                            {cell === null ? <span className="text-gray-600">NULL</span> : String(cell)}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center h-full text-gray-500">
                <Table className="w-8 h-8 mb-2 opacity-50" />
                <p>No results returned</p>
              </div>
            )}
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center h-full text-gray-600">
            <Play className="w-8 h-8 mb-2 opacity-50" />
            <p>Run a query to see results</p>
          </div>
        )}
      </div>
    </div>
  )
}
