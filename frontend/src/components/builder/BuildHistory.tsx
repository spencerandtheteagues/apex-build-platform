// APEX-BUILD Build History
// Shows completed builds with view/download/re-open functionality

import React, { useState, useEffect, useCallback } from 'react'
import { cn } from '@/lib/utils'
import apiService, { CompletedBuildSummary } from '@/services/api'
import {
  Clock,
  FileCode,
  Download,
  ChevronRight,
  AlertCircle,
  CheckCircle2,
  XCircle,
  Zap,
  Cpu,
  Rocket,
  FolderOpen,
  Trash2,
  Square,
} from 'lucide-react'

interface BuildHistoryProps {
  userId?: number | null
  onOpenBuild?: (buildId: string, action?: 'resume' | 'open_files') => void
}

export const BuildHistory: React.FC<BuildHistoryProps> = ({ userId, onOpenBuild }) => {
  const [builds, setBuilds] = useState<CompletedBuildSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [downloading, setDownloading] = useState<string | null>(null)
  const [actingBuildId, setActingBuildId] = useState<string | null>(null)

  const loadBuilds = useCallback(async () => {
    if (!userId) {
      setBuilds([])
      setError(null)
      setLoading(false)
      return
    }

    try {
      setLoading(true)
      setError(null)
      setActionError(null)
      const data = await apiService.listBuilds(1, 10)
      setBuilds(data.builds || [])
    } catch (err: any) {
      setBuilds([])
      setError('Unable to load recent builds right now.')
    } finally {
      setLoading(false)
    }
  }, [userId])

  useEffect(() => {
    void loadBuilds()
  }, [loadBuilds])

  useEffect(() => {
    if (!userId) return

    const reload = () => {
      if (document.visibilityState === 'hidden') {
        return
      }
      void loadBuilds()
    }

    window.addEventListener('focus', reload)
    document.addEventListener('visibilitychange', reload)

    return () => {
      window.removeEventListener('focus', reload)
      document.removeEventListener('visibilitychange', reload)
    }
  }, [loadBuilds, userId])

  const handleDownload = async (build: CompletedBuildSummary) => {
    try {
      setDownloading(build.build_id)
      const blob = await apiService.downloadBuildAsZip(build.build_id)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${build.project_name || 'apex-build'}-${build.build_id.slice(0, 8)}.zip`
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      console.error('Download failed:', err)
    } finally {
      setDownloading(null)
    }
  }

  const handleCancel = async (build: CompletedBuildSummary) => {
    const confirmed = window.confirm('Cancel this active build? It will stay in Recent Builds so you can inspect it later.')
    if (!confirmed) return

    try {
      setActingBuildId(build.build_id)
      setActionError(null)
      await apiService.cancelBuild(build.build_id)
      await loadBuilds()
    } catch (err) {
      setActionError('Unable to cancel that build right now.')
    } finally {
      setActingBuildId(null)
    }
  }

  const handleDelete = async (build: CompletedBuildSummary) => {
    const confirmed = window.confirm('Remove this saved build from Recent Builds? This only deletes the saved history entry.')
    if (!confirmed) return

    try {
      setActingBuildId(build.build_id)
      setActionError(null)
      await apiService.deleteBuild(build.build_id)
      await loadBuilds()
    } catch (err) {
      setActionError('Unable to remove that build right now.')
    } finally {
      setActingBuildId(null)
    }
  }

  const formatDuration = (ms: number) => {
    if (ms < 60000) return `${Math.round(ms / 1000)}s`
    return `${Math.round(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`
  }

  const formatDate = (dateStr: string) => {
    const d = new Date(dateStr)
    const now = new Date()
    const diff = now.getTime() - d.getTime()
    if (diff < 3600000) return `${Math.round(diff / 60000)}m ago`
    if (diff < 86400000) return `${Math.round(diff / 3600000)}h ago`
    if (diff < 604800000) return `${Math.round(diff / 86400000)}d ago`
    return d.toLocaleDateString()
  }

  const statusIcon = (status: string) => {
    switch (status) {
      case 'completed': return <CheckCircle2 className="w-4 h-4 text-green-400" />
      case 'failed': return <XCircle className="w-4 h-4 text-red-400" />
      default: return <AlertCircle className="w-4 h-4 text-yellow-400" />
    }
  }

  const powerModeIcon = (mode: string) => {
    switch (mode) {
      case 'max': return <Rocket className="w-3.5 h-3.5 text-red-400" />
      case 'balanced': return <Cpu className="w-3.5 h-3.5 text-yellow-400" />
      default: return <Zap className="w-3.5 h-3.5 text-green-400" />
    }
  }

  if (loading) {
    return (
      <div className="max-w-4xl mx-auto mt-8">
        <div className="flex items-center gap-2 mb-4">
          <Clock className="w-5 h-5 text-gray-500" />
          <h3 className="text-gray-500 font-medium">Recent Builds</h3>
        </div>
        <div className="animate-pulse space-y-3">
          {[1, 2, 3].map(i => (
            <div key={i} className="h-20 bg-gray-900/50 rounded-xl border border-gray-800" />
          ))}
        </div>
      </div>
    )
  }

  if (builds.length === 0) {
    if (!error) return null

    return (
      <div className="max-w-4xl mx-auto mt-8 rounded-xl border border-red-900/40 bg-red-950/20 px-4 py-3">
        <div className="flex items-center justify-between gap-3">
          <p className="text-sm text-red-200">{error}</p>
          <button
            type="button"
            onClick={() => { void loadBuilds() }}
            className="rounded-lg border border-red-900/60 px-3 py-1.5 text-xs font-semibold text-red-200 hover:bg-red-900/30"
          >
            Retry
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="max-w-4xl mx-auto mt-8">
      <div className="flex items-center gap-2 mb-4">
        <Clock className="w-5 h-5 text-gray-500" />
        <h3 className="text-gray-400 font-semibold text-sm uppercase tracking-wider">Recent Builds</h3>
        <span className="text-xs text-gray-600 ml-auto">{builds.length} builds</span>
      </div>

      <p className="mb-4 text-sm text-gray-500">
        The builder now opens to a fresh prompt by default. Previous runs stay saved here, and you can click any build to reopen its workflow or code when you actually want it.
      </p>

      {actionError ? (
        <div className="mb-4 rounded-xl border border-red-900/40 bg-red-950/20 px-4 py-3 text-sm text-red-200">
          {actionError}
        </div>
      ) : null}

      <div className="space-y-2.5">
        {builds.map((build) => (
          <div
            key={build.build_id}
            className={cn(
              'group flex items-center gap-4 p-4 rounded-xl',
              'bg-gray-900/40 border border-gray-800/80',
              'hover:bg-gray-900/60 hover:border-gray-700 transition-all duration-200',
              'cursor-pointer'
            )}
            onClick={() => onOpenBuild?.(build.build_id, 'resume')}
          >
            {/* Status icon */}
            <div className="shrink-0">{statusIcon(build.status)}</div>

            {/* Main info */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-0.5">
                <span className="text-white font-medium text-sm truncate">
                  {build.description.length > 60
                    ? build.description.slice(0, 60) + '...'
                    : build.description}
                </span>
              </div>
              <div className="flex items-center gap-3 text-xs text-gray-500">
                {build.tech_stack && (
                  <span className="text-gray-400">
                    {[build.tech_stack.frontend, build.tech_stack.backend, build.tech_stack.database]
                      .filter(Boolean)
                      .join(' + ')}
                  </span>
                )}
                <span className="flex items-center gap-1">
                  <FileCode className="w-3 h-3" />
                  {build.files_count} files
                </span>
                {build.duration_ms > 0 && (
                  <span>{formatDuration(build.duration_ms)}</span>
                )}
                <span className="flex items-center gap-1">
                  {powerModeIcon(build.power_mode)}
                  {build.power_mode}
                </span>
              </div>
            </div>

            {/* Timestamp */}
            <span className="text-xs text-gray-600 shrink-0">
              {formatDate(build.created_at)}
            </span>

            {/* Actions */}
            <div className="flex items-center gap-1.5 shrink-0">
              {build.resumable ? (
                <button
                  onClick={(e) => { e.stopPropagation(); void handleCancel(build) }}
                  className="inline-flex items-center gap-1 rounded-lg border border-red-900/50 px-2 py-1.5 text-xs font-medium text-red-200 hover:bg-red-900/20 transition-colors"
                  title="Cancel active build"
                  aria-label={`Cancel build ${build.description}`}
                  disabled={actingBuildId === build.build_id}
                >
                  {actingBuildId === build.build_id ? (
                    <div className="w-3.5 h-3.5 border-2 border-red-200/50 border-t-red-100 rounded-full animate-spin" />
                  ) : (
                    <Square className="w-3.5 h-3.5" />
                  )}
                  Cancel
                </button>
              ) : (
                <button
                  onClick={(e) => { e.stopPropagation(); void handleDelete(build) }}
                  className="inline-flex items-center gap-1 rounded-lg border border-gray-800 px-2 py-1.5 text-xs font-medium text-gray-300 hover:bg-gray-800 hover:text-white transition-colors"
                  title="Remove saved build"
                  aria-label={`Remove build ${build.description}`}
                  disabled={actingBuildId === build.build_id}
                >
                  {actingBuildId === build.build_id ? (
                    <div className="w-3.5 h-3.5 border-2 border-gray-500 border-t-white rounded-full animate-spin" />
                  ) : (
                    <Trash2 className="w-3.5 h-3.5" />
                  )}
                  Remove
                </button>
              )}
              <div className="flex items-center gap-1.5 opacity-100 md:opacity-0 md:group-hover:opacity-100 transition-opacity">
              <button
                onClick={(e) => { e.stopPropagation(); onOpenBuild?.(build.build_id, 'resume') }}
                className="hidden md:inline-flex items-center gap-1 rounded-lg px-2 py-1.5 text-xs font-medium text-gray-300 hover:bg-gray-800 hover:text-white transition-colors"
                title="Continue workflow"
              >
                <ChevronRight className="w-3.5 h-3.5" />
                Continue
              </button>
              <button
                onClick={(e) => { e.stopPropagation(); handleDownload(build) }}
                className="p-1.5 rounded-lg text-gray-500 hover:text-white hover:bg-gray-800 transition-colors"
                title="Download files"
              >
                {downloading === build.build_id ? (
                  <div className="w-4 h-4 border-2 border-gray-500 border-t-white rounded-full animate-spin" />
                ) : (
                  <Download className="w-4 h-4" />
                )}
              </button>
              <button
                onClick={(e) => { e.stopPropagation(); onOpenBuild?.(build.build_id, 'open_files') }}
                className="p-1.5 rounded-lg text-gray-500 hover:text-white hover:bg-gray-800 transition-colors"
                title="Open code in IDE"
              >
                <FolderOpen className="w-4 h-4" />
              </button>
              </div>
            </div>

            <ChevronRight className="w-4 h-4 text-gray-700 group-hover:text-gray-500 transition-colors shrink-0" />
          </div>
        ))}
      </div>
    </div>
  )
}

export default BuildHistory
