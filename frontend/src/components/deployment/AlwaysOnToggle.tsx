// APEX.BUILD Always-On Toggle Component
// Replit parity feature - keeps deployments running 24/7 with auto-restart

import React, { useState, useEffect } from 'react'
import { cn } from '@/lib/utils'
import { apiService, AlwaysOnStatus } from '@/services/api'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import {
  Power,
  Zap,
  Activity,
  Clock,
  RefreshCw,
  Heart,
  AlertTriangle,
  CheckCircle,
  Info,
} from 'lucide-react'

export interface AlwaysOnToggleProps {
  projectId: number
  deploymentId: string
  initialEnabled?: boolean
  onStatusChange?: (enabled: boolean) => void
  className?: string
}

export const AlwaysOnToggle: React.FC<AlwaysOnToggleProps> = ({
  projectId,
  deploymentId,
  initialEnabled = false,
  onStatusChange,
  className,
}) => {
  const [enabled, setEnabled] = useState(initialEnabled)
  const [loading, setLoading] = useState(false)
  const [status, setStatus] = useState<AlwaysOnStatus | null>(null)
  const [error, setError] = useState<string | null>(null)

  // Load always-on status on mount
  useEffect(() => {
    loadStatus()
    // Poll status every 30 seconds when enabled
    const interval = setInterval(() => {
      if (enabled) {
        loadStatus()
      }
    }, 30000)
    return () => clearInterval(interval)
  }, [projectId, deploymentId, enabled])

  const loadStatus = async () => {
    try {
      const statusData = await apiService.getAlwaysOnStatus(projectId, deploymentId)
      setStatus(statusData)
      setEnabled(statusData.always_on)
      setError(null)
    } catch (err) {
      console.error('Failed to load always-on status:', err)
    }
  }

  const toggleAlwaysOn = async () => {
    setLoading(true)
    setError(null)

    try {
      const result = await apiService.setAlwaysOn(projectId, deploymentId, !enabled, 60)
      setEnabled(result.always_on)
      onStatusChange?.(result.always_on)
      await loadStatus()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to update always-on setting')
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

  const getStatusColor = (containerStatus: string): string => {
    switch (containerStatus) {
      case 'healthy':
        return 'text-green-400'
      case 'unhealthy':
        return 'text-red-400'
      case 'starting':
        return 'text-yellow-400'
      default:
        return 'text-gray-400'
    }
  }

  const getStatusBadge = (containerStatus: string) => {
    switch (containerStatus) {
      case 'healthy':
        return <Badge variant="success" icon={<CheckCircle size={12} />}>Healthy</Badge>
      case 'unhealthy':
        return <Badge variant="error" icon={<AlertTriangle size={12} />}>Unhealthy</Badge>
      case 'starting':
        return <Badge variant="warning" icon={<RefreshCw size={12} className="animate-spin" />}>Starting</Badge>
      default:
        return <Badge variant="default" icon={<Power size={12} />}>Stopped</Badge>
    }
  }

  return (
    <Card variant="cyberpunk" className={cn('relative overflow-hidden', className)}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className={cn(
              'p-2 rounded-lg transition-colors',
              enabled ? 'bg-cyan-500/20' : 'bg-gray-700/50'
            )}>
              <Zap className={cn(
                'w-5 h-5 transition-colors',
                enabled ? 'text-cyan-400' : 'text-gray-500'
              )} />
            </div>
            <div>
              <CardTitle className="text-lg">Always-On</CardTitle>
              <CardDescription>
                Keep your deployment running 24/7
              </CardDescription>
            </div>
          </div>

          {/* Toggle Switch */}
          <button
            onClick={toggleAlwaysOn}
            disabled={loading}
            className={cn(
              'relative w-14 h-7 rounded-full transition-colors duration-300 focus:outline-none focus:ring-2 focus:ring-cyan-500/50',
              enabled ? 'bg-cyan-500' : 'bg-gray-700',
              loading && 'opacity-50 cursor-not-allowed'
            )}
          >
            <span
              className={cn(
                'absolute top-0.5 left-0.5 w-6 h-6 bg-white rounded-full transition-transform duration-300 shadow-lg',
                enabled && 'translate-x-7'
              )}
            >
              {loading && (
                <RefreshCw className="w-4 h-4 text-gray-600 absolute top-1 left-1 animate-spin" />
              )}
            </span>
          </button>
        </div>
      </CardHeader>

      <CardContent>
        {/* Error Message */}
        {error && (
          <div className="mb-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg flex items-center gap-2 text-sm text-red-400">
            <AlertTriangle size={16} />
            {error}
          </div>
        )}

        {/* Status Info */}
        {status && (
          <div className="space-y-4">
            {/* Container Status */}
            <div className="flex items-center justify-between">
              <span className="text-sm text-gray-400">Container Status</span>
              {getStatusBadge(status.container_status)}
            </div>

            {/* Stats Grid */}
            <div className="grid grid-cols-2 gap-3">
              {/* Uptime */}
              <div className="p-3 bg-gray-800/50 rounded-lg border border-gray-700/50">
                <div className="flex items-center gap-2 text-xs text-gray-400 mb-1">
                  <Clock size={12} />
                  Uptime
                </div>
                <div className="text-lg font-semibold text-white">
                  {formatUptime(status.uptime_seconds)}
                </div>
              </div>

              {/* Restarts */}
              <div className="p-3 bg-gray-800/50 rounded-lg border border-gray-700/50">
                <div className="flex items-center gap-2 text-xs text-gray-400 mb-1">
                  <RefreshCw size={12} />
                  Restarts
                </div>
                <div className="text-lg font-semibold text-white">
                  {status.restart_count} / {status.max_restarts}
                </div>
              </div>

              {/* Keep-Alive Interval */}
              <div className="p-3 bg-gray-800/50 rounded-lg border border-gray-700/50">
                <div className="flex items-center gap-2 text-xs text-gray-400 mb-1">
                  <Heart size={12} />
                  Keep-Alive
                </div>
                <div className="text-lg font-semibold text-white">
                  {status.keep_alive_interval}s
                </div>
              </div>

              {/* Last Ping */}
              <div className="p-3 bg-gray-800/50 rounded-lg border border-gray-700/50">
                <div className="flex items-center gap-2 text-xs text-gray-400 mb-1">
                  <Activity size={12} />
                  Last Ping
                </div>
                <div className="text-sm font-semibold text-white truncate">
                  {status.last_keep_alive
                    ? new Date(status.last_keep_alive).toLocaleTimeString()
                    : 'Never'
                  }
                </div>
              </div>
            </div>

            {/* Info Box */}
            <div className="p-3 bg-cyan-500/10 border border-cyan-500/30 rounded-lg">
              <div className="flex items-start gap-2">
                <Info size={16} className="text-cyan-400 mt-0.5 flex-shrink-0" />
                <div className="text-sm text-cyan-300">
                  {enabled ? (
                    <>
                      <strong>Always-On is active.</strong> Your deployment will run continuously with automatic
                      restart on crash. Health checks run every {status.keep_alive_interval} seconds.
                    </>
                  ) : (
                    <>
                      <strong>Always-On is disabled.</strong> Your deployment may sleep after 30 minutes
                      of inactivity. Enable to keep it running 24/7.
                    </>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Loading State */}
        {!status && (
          <div className="flex items-center justify-center py-8">
            <RefreshCw className="w-6 h-6 text-cyan-400 animate-spin" />
          </div>
        )}
      </CardContent>

      {/* Animated glow when enabled */}
      {enabled && (
        <div className="absolute inset-0 pointer-events-none">
          <div className="absolute top-0 left-1/2 -translate-x-1/2 w-32 h-1 bg-gradient-to-r from-transparent via-cyan-400 to-transparent opacity-50" />
          <div className="absolute bottom-0 left-1/2 -translate-x-1/2 w-32 h-1 bg-gradient-to-r from-transparent via-cyan-400 to-transparent opacity-50" />
        </div>
      )}
    </Card>
  )
}

export default AlwaysOnToggle
