// APEX.BUILD Build History
// Shows completed builds with view/download/re-open functionality

import React, { useState, useEffect } from 'react'
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
} from 'lucide-react'

interface BuildHistoryProps {
  onOpenBuild?: (buildId: string, action?: 'resume' | 'open_files') => void
}

export const BuildHistory: React.FC<BuildHistoryProps> = ({ onOpenBuild }) => {
  const [builds, setBuilds] = useState<CompletedBuildSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [downloading, setDownloading] = useState<string | null>(null)

  useEffect(() => {
    loadBuilds()
  }, [])

  const loadBuilds = async () => {
    try {
      setLoading(true)
      const data = await apiService.listBuilds(1, 10)
      setBuilds(data.builds || [])
    } catch (err: any) {
      // Silently handle — might not have any builds yet
      setBuilds([])
    } finally {
      setLoading(false)
    }
  }

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

  if (builds.length === 0) return null

  return (
    <div className="max-w-4xl mx-auto mt-8">
      <div className="flex items-center gap-2 mb-4">
        <Clock className="w-5 h-5 text-gray-500" />
        <h3 className="text-gray-400 font-semibold text-sm uppercase tracking-wider">Recent Builds</h3>
        <span className="text-xs text-gray-600 ml-auto">{builds.length} builds</span>
      </div>

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
            <div className="flex items-center gap-1.5 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
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
                title="Open build"
              >
                <FolderOpen className="w-4 h-4" />
              </button>
            </div>

            <ChevronRight className="w-4 h-4 text-gray-700 group-hover:text-gray-500 transition-colors shrink-0" />
          </div>
        ))}
      </div>
    </div>
  )
}

export default BuildHistory
