import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { AlertTriangle, BrainCircuit, Database, FileCode2, GitBranch, RefreshCw, ShieldAlert } from 'lucide-react'

import { Badge, Button, Card, Loading } from '@/components/ui'
import { cn } from '@/lib/utils'
import type { ArchitectureIntelligenceMap } from '@/services/api'
import { useStore } from '@/hooks/useStore'

const topEntries = (values?: Record<string, number>, limit = 5): Array<[string, number]> => (
  Object.entries(values || {})
    .sort((a, b) => b[1] - a[1])
    .slice(0, limit)
)

const riskBadgeVariant = (risk: string): React.ComponentProps<typeof Badge>['variant'] => {
  switch (risk) {
    case 'critical':
      return 'error'
    case 'high':
      return 'warning'
    case 'medium':
      return 'info'
    default:
      return 'neutral'
  }
}

export interface ArchitectureIntelligencePanelProps {
  className?: string
}

export const ArchitectureIntelligencePanel: React.FC<ArchitectureIntelligencePanelProps> = ({ className }) => {
  const { apiService } = useStore()
  const [map, setMap] = useState<ArchitectureIntelligenceMap | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchMap = useCallback(async () => {
    try {
      setLoading(true)
      setError(null)
      setMap(await apiService.getAdminArchitectureMap())
    } catch (err) {
      console.error('Failed to fetch architecture intelligence map:', err)
      setError('Unable to load architecture intelligence map.')
    } finally {
      setLoading(false)
    }
  }, [apiService])

  useEffect(() => {
    fetchMap()
  }, [fetchMap])

  const highSignalNodes = useMemo(() => (
    [...(map?.nodes || [])]
      .sort((a, b) => (b.references || 0) - (a.references || 0) || b.risk_score - a.risk_score)
      .slice(0, 6)
  ), [map])
  const topContracts = topEntries(map?.reference_telemetry?.by_contract)
  const topDatabases = topEntries(map?.reference_telemetry?.by_database)
  const topStructures = topEntries(map?.reference_telemetry?.by_structure)

  return (
    <Card variant="cyberpunk" padding="lg" className={cn('mb-8 border-cyan-500/30', className)}>
      <div className="mb-6 flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div className="flex items-start gap-3">
          <BrainCircuit className="mt-1 h-7 w-7 text-cyan-300" />
          <div>
            <h2 className="text-xl font-bold text-white">Architecture Intelligence</h2>
            <p className="mt-1 max-w-3xl text-sm text-gray-400">
              Admin-only repo map plus privacy-safe build reference counts. It shows which directories,
              contracts, database surfaces, and structures agents are actually using as context.
            </p>
          </div>
        </div>
        <Button variant="ghost" size="sm" onClick={fetchMap} disabled={loading}>
          <RefreshCw className="mr-2 h-4 w-4" />
          Refresh Map
        </Button>
      </div>

      {loading ? (
        <div className="flex min-h-40 items-center justify-center">
          <Loading size="md" variant="spinner" label="Scanning architecture map..." />
        </div>
      ) : error ? (
        <div className="flex items-center gap-3 rounded-xl border border-red-500/30 bg-red-950/20 p-4 text-red-300">
          <AlertTriangle className="h-5 w-5" />
          <span>{error}</span>
        </div>
      ) : map ? (
        <div className="space-y-6">
          <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
            <div className="rounded-xl border border-gray-800 bg-black/30 p-4">
              <div className="flex items-center justify-between text-sm text-gray-400">
                <span>Nodes</span>
                <GitBranch className="h-4 w-4 text-cyan-300" />
              </div>
              <div className="mt-2 text-2xl font-bold text-white">{map.summary.node_count}</div>
              <div className="text-xs text-gray-500">{map.summary.file_count} files scanned</div>
            </div>
            <div className="rounded-xl border border-gray-800 bg-black/30 p-4">
              <div className="flex items-center justify-between text-sm text-gray-400">
                <span>Contracts</span>
                <ShieldAlert className="h-4 w-4 text-yellow-300" />
              </div>
              <div className="mt-2 text-2xl font-bold text-white">{map.summary.contract_count}</div>
              <div className="text-xs text-gray-500">{map.summary.critical_nodes} critical nodes</div>
            </div>
            <div className="rounded-xl border border-gray-800 bg-black/30 p-4">
              <div className="flex items-center justify-between text-sm text-gray-400">
                <span>Reference Counts</span>
                <FileCode2 className="h-4 w-4 text-green-300" />
              </div>
              <div className="mt-2 text-2xl font-bold text-white">{map.reference_telemetry?.total_references || 0}</div>
              <div className="text-xs text-gray-500">Metadata only, no prompt text</div>
            </div>
            <div className="rounded-xl border border-gray-800 bg-black/30 p-4">
              <div className="flex items-center justify-between text-sm text-gray-400">
                <span>Confidence</span>
                <Database className="h-4 w-4 text-blue-300" />
              </div>
              <div className="mt-2 text-2xl font-bold text-white">{Math.round(map.confidence * 100)}%</div>
              <div className="text-xs text-gray-500">{map.source}</div>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            <div className="rounded-xl border border-gray-800 bg-gray-950/60">
              <div className="border-b border-gray-800 px-4 py-3">
                <h3 className="text-sm font-semibold uppercase tracking-[0.2em] text-gray-400">Hot Knowledge Pockets</h3>
              </div>
              <div className="divide-y divide-gray-800/80">
                {highSignalNodes.map((node) => (
                  <div key={node.id} className="flex items-start justify-between gap-4 px-4 py-3">
                    <div>
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium text-white">{node.name}</span>
                        <Badge variant={riskBadgeVariant(node.risk_level)} size="xs">{node.risk_level}</Badge>
                      </div>
                      <div className="mt-1 text-xs text-gray-500">{node.code_locations[0]?.path || node.id}</div>
                    </div>
                    <div className="text-right">
                      <div className="text-sm font-semibold text-cyan-300">{node.references || 0}</div>
                      <div className="text-[10px] uppercase tracking-wide text-gray-500">refs</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div className="grid grid-cols-1 gap-4">
              <ReferenceList title="Contracts Referenced" entries={topContracts} empty="No contract references captured yet." />
              <ReferenceList title="Database Surfaces Referenced" entries={topDatabases} empty="No database references captured yet." />
              <ReferenceList title="Structures Referenced" entries={topStructures} empty="No structure references captured yet." />
            </div>
          </div>
        </div>
      ) : null}
    </Card>
  )
}

const ReferenceList: React.FC<{ title: string; entries: Array<[string, number]>; empty: string }> = ({ title, entries, empty }) => (
  <div className="rounded-xl border border-gray-800 bg-gray-950/60 p-4">
    <h3 className="mb-3 text-sm font-semibold text-white">{title}</h3>
    {entries.length === 0 ? (
      <p className="text-sm text-gray-500">{empty}</p>
    ) : (
      <div className="space-y-2">
        {entries.map(([name, count]) => (
          <div key={name} className="flex items-center justify-between gap-3 text-sm">
            <span className="min-w-0 truncate text-gray-300">{name}</span>
            <span className="rounded-full bg-cyan-500/10 px-2 py-0.5 text-xs font-medium text-cyan-200">{count}</span>
          </div>
        ))}
      </div>
    )}
  </div>
)
