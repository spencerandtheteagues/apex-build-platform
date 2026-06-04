// KeepThisRunning — First-class post-build "Keep this app running" CTA
// TASK-104: Surfaces persistent-preview / always-on as a post-build action.
// Reuses apiService.getAlwaysOnStatus and apiService.setAlwaysOn.
// Paid users see an active toggle; free users see an upgrade prompt.
// If deploymentId is missing, auto-discovers the first running deployment.

import React, { useState, useEffect, useCallback, useRef } from 'react'
import { cn } from '@/lib/utils'
import { apiService, AlwaysOnStatus } from '@/services/api'
import { Badge } from '@/components/ui'
import {
  Zap,
  Power,
  AlertTriangle,
  CheckCircle,
  RefreshCw,
  Clock,
  ArrowUpRight,
  Info,
} from 'lucide-react'

export interface KeepThisRunningProps {
  projectId: number | null
  deploymentId?: string | null
  isPaid: boolean
  buildCompleted: boolean
  className?: string
}

export const KeepThisRunning: React.FC<KeepThisRunningProps> = ({
  projectId,
  deploymentId: explicitDeploymentId,
  isPaid,
  buildCompleted,
  className,
}) => {
  const [discoveredDeploymentId, setDiscoveredDeploymentId] = useState<string | null>(null)
  const [discoveringDeployment, setDiscoveringDeployment] = useState(false)
  const [enabled, setEnabled] = useState(false)
  const [loading, setLoading] = useState(false)
  const [status, setStatus] = useState<AlwaysOnStatus | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [showBreakdown, setShowBreakdown] = useState(false)
  const discoveryAttemptedRef = useRef(false)

  // Resolve effective deploymentId: explicit > discovered > pending discovery
  const deploymentId = explicitDeploymentId || discoveredDeploymentId

  // Auto-discover deployments when projectId is available but deploymentId is not
  useEffect(() => {
    if (!isPaid || !projectId || explicitDeploymentId) return
    if (discoveryAttemptedRef.current) return
    discoveryAttemptedRef.current = true

    const discoverDeployment = async () => {
      setDiscoveringDeployment(true)
      try {
        const response = await apiService.getNativeDeployments(projectId, 1, 20)
        const deployments = response?.deployments || []
        // Prefer a running deployment, then any deployment
        const running = deployments.find(
          (d) => d.status === 'running' && d.container_status === 'healthy'
        )
        const target = running || deployments[0] || null
        if (target) {
          setDiscoveredDeploymentId(target.id)
        }
      } catch {
        // Non-fatal: deployment discovery failed, component degrades gracefully
      } finally {
        setDiscoveringDeployment(false)
      }
    }

    void discoverDeployment()
  }, [isPaid, projectId, explicitDeploymentId])

  // Reset discovery when identifiers change
  useEffect(() => {
    discoveryAttemptedRef.current = false
    setDiscoveredDeploymentId(null)
  }, [projectId, explicitDeploymentId])

  const canCallApi = isPaid && Boolean(projectId && deploymentId)

  const loadStatus = useCallback(async () => {
    if (!canCallApi) return
    try {
      const data = await apiService.getAlwaysOnStatus(projectId!, deploymentId!)
      setStatus(data)
      setEnabled(data.always_on)
      setError(null)
    } catch (err: any) {
      if (err?.response?.status === 404) {
        setStatus(null)
        setEnabled(false)
      } else {
        setError(err?.response?.data?.error || 'Failed to load status')
      }
    }
  }, [canCallApi, projectId, deploymentId])

  // Load status on mount and when identifiers change; poll every 30s when enabled.
  useEffect(() => {
    if (!buildCompleted) return
    loadStatus()
    const interval = setInterval(() => {
      if (buildCompleted) loadStatus()
    }, 30000)
    return () => clearInterval(interval)
  }, [buildCompleted, loadStatus])

  const toggleAlwaysOn = async () => {
    if (!canCallApi) return
    setLoading(true)
    setError(null)
    try {
      const result = await apiService.setAlwaysOn(projectId!, deploymentId!, !enabled, 60)
      setEnabled(result.always_on)
      await loadStatus()
    } catch (err: any) {
      setError(err?.response?.data?.error || 'Failed to update always-on setting')
    } finally {
      setLoading(false)
    }
  }

  const formatUptime = (seconds: number): string => {
    if (seconds < 60) return `${seconds}s`
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m`
    if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
    return `${Math.floor(seconds / 86400)}d ${Math.floor((seconds % 86400) / 3600)}h`
  }

  const getStatusPill = () => {
    if (!status) return null
    const s = status.container_status
    const isOn = status.always_on
    if (isOn && s === 'healthy') {
      return (
        <Badge variant="success" icon={<CheckCircle size={10} />} className="text-[10px]">
          Running
        </Badge>
      )
    }
    if (isOn && s === 'starting') {
      return (
        <Badge variant="warning" icon={<RefreshCw size={10} className="animate-spin" />} className="text-[10px]">
          Warm
        </Badge>
      )
    }
    if (!isOn || s === 'stopped') {
      return (
        <Badge variant="outline" icon={<Power size={10} />} className="text-[10px] border-gray-600 text-gray-400 bg-gray-950/50">
          Cold
        </Badge>
      )
    }
    return (
      <Badge variant="error" icon={<AlertTriangle size={10} />} className="text-[10px]">
        Unhealthy
      </Badge>
    )
  }

  // ── Free user: upgrade prompt (disabled state) ────────────────────────────
  if (!isPaid) {
    return (
      <div
        className={cn(
          'shrink-0 flex items-center gap-3 px-3 py-2 border-t border-sky-500/10 bg-slate-950/80',
          className
        )}
      >
        <div className="flex items-center gap-2 text-gray-400">
          <Power className="w-4 h-4" />
          <span className="text-xs font-medium">Keep this app running</span>
        </div>
        <span className="text-[10px] text-gray-500">—</span>
        <button
          type="button"
          onClick={() => {
            if (typeof window !== 'undefined') {
              window.location.href = '/settings?tab=billing'
            }
          }}
          className="inline-flex items-center gap-1 text-[10px] font-semibold text-cyan-300 hover:text-cyan-200 underline underline-offset-2"
        >
          Upgrade to enable <ArrowUpRight className="w-3 h-3" />
        </button>
      </div>
    )
  }

  // ── Paid user, still discovering deployment ────────────────────────────────
  if (discoveringDeployment) {
    return (
      <div
        className={cn(
          'shrink-0 flex items-center gap-3 px-3 py-2 border-t border-sky-500/10 bg-slate-950/80',
          className
        )}
      >
        <div className="flex items-center gap-2 text-gray-400">
          <Zap className="w-4 h-4 text-cyan-400" />
          <span className="text-xs font-medium text-cyan-100">Keep this app running</span>
        </div>
        <span className="text-[10px] text-gray-500">—</span>
        <RefreshCw className="w-3 h-3 text-cyan-400 animate-spin" />
        <span className="text-[10px] text-gray-400">
          Checking deployment status...
        </span>
      </div>
    )
  }

  // ── Paid user but no deploymentId & discovery failed: "Deploy first" state ──
  if (isPaid && !canCallApi) {
    return (
      <div
        className={cn(
          'shrink-0 flex items-center gap-3 px-3 py-2 border-t border-sky-500/10 bg-slate-950/80',
          className
        )}
      >
        <div className="flex items-center gap-2 text-gray-400">
          <Zap className="w-4 h-4 text-cyan-400" />
          <span className="text-xs font-medium text-cyan-100">Keep this app running</span>
        </div>
        <span className="text-[10px] text-gray-500">—</span>
        <span className="text-[10px] text-gray-400">
          Deploy or publish first to enable always-on.
        </span>
        {getStatusPill()}
      </div>
    )
  }

  // ── Paid user with projectId + deploymentId: active toggle ─────────────────
  return (
    <div
      className={cn(
        'shrink-0 relative flex items-center gap-3 px-3 py-2 border-t border-sky-500/10 bg-slate-950/80',
        className
      )}
    >
      <div className="flex items-center gap-2">
        <Zap className={cn('w-4 h-4', enabled ? 'text-cyan-400' : 'text-gray-500')} />
        <span className={cn('text-xs font-medium', enabled ? 'text-cyan-100' : 'text-gray-300')}>
          Keep this app running
        </span>
      </div>

      {/* Toggle switch */}
      <button
        type="button"
        onClick={toggleAlwaysOn}
        disabled={loading}
        aria-label={enabled ? 'Disable always-on' : 'Enable always-on'}
        title={enabled ? 'Always-on is enabled' : 'Enable always-on to keep the app running 24/7'}
        className={cn(
          'relative w-11 h-5 rounded-full transition-colors duration-300 focus:outline-none focus:ring-2 focus:ring-cyan-500/50',
          enabled ? 'bg-cyan-500' : 'bg-gray-700',
          loading && 'opacity-50 cursor-not-allowed'
        )}
      >
        <span
          className={cn(
            'absolute top-0.5 left-0.5 w-4 h-4 bg-white rounded-full transition-transform duration-300 shadow',
            enabled && 'translate-x-6'
          )}
        >
          {loading && (
            <RefreshCw className="w-3 h-3 text-gray-600 absolute top-0.5 left-0.5 animate-spin" />
          )}
        </span>
      </button>

      {/* Status pill */}
      {getStatusPill()}

      {/* Breakdown toggle */}
      {status && (
        <button
          type="button"
          onClick={() => setShowBreakdown((v) => !v)}
          className="ml-auto inline-flex items-center gap-1 text-[10px] text-gray-400 hover:text-gray-200"
        >
          <Info className="w-3 h-3" />
          {showBreakdown ? 'Hide' : 'Details'}
        </button>
      )}

      {/* Inline error */}
      {error && (
        <span className="text-[10px] text-red-400 flex items-center gap-1">
          <AlertTriangle className="w-3 h-3" />
          {error}
        </span>
      )}

      {/* Expanded breakdown */}
      {showBreakdown && status && (
        <div className="absolute left-3 right-3 bottom-full mb-2 z-50 bg-gray-900/95 backdrop-blur-xl border border-gray-700/70 rounded-xl shadow-2xl p-3 space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-[10px] font-bold uppercase tracking-widest text-slate-400">Always-On Status</span>
            {getStatusPill()}
          </div>
          <div className="grid grid-cols-2 gap-2 text-xs">
            <div className="flex items-center gap-1.5 text-gray-400">
              <Clock className="w-3 h-3" />
              Uptime: <span className="text-white font-mono">{formatUptime(status.uptime_seconds)}</span>
            </div>
            <div className="flex items-center gap-1.5 text-gray-400">
              <RefreshCw className="w-3 h-3" />
              Restarts: <span className="text-white font-mono">{status.restart_count} / {status.max_restarts}</span>
            </div>
          </div>
          <div className="text-[10px] text-cyan-200/80 border-t border-gray-800/50 pt-1.5">
            {enabled
              ? 'Your deployment will run continuously with automatic restart on crash.'
              : 'Your deployment may sleep after 30 minutes of inactivity.'}
          </div>
        </div>
      )}
    </div>
  )
}

export default KeepThisRunning
