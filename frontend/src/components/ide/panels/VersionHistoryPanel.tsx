// APEX.BUILD Version History Panel
// Displays file version history and enables diff viewing/restoring

import React, { useEffect, useState } from 'react'
import { format } from 'date-fns'
import {
  RotateCcw,
  GitCommit,
  Clock,
  Pin,
  MoreVertical,
  Eye,
  Trash2,
  FileDiff
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { File, FileVersion } from '@/types'
import apiService from '@/services/api'
import { Button, Badge, Loading, Card } from '@/components/ui'
import { useStore } from '@/hooks/useStore'

interface VersionHistoryPanelProps {
  file: File | null
  projectId: number
  onPreviewVersion: (version: FileVersion) => void
  onRestoreVersion: (version: FileVersion) => void
  className?: string
}

export const VersionHistoryPanel: React.FC<VersionHistoryPanelProps> = ({
  file,
  projectId,
  onPreviewVersion,
  onRestoreVersion,
  className
}) => {
  const [versions, setVersions] = useState<FileVersion[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const { user } = useStore()

  // Fetch versions when file changes
  useEffect(() => {
    if (!file) {
      setVersions([])
      return
    }

    const fetchVersions = async () => {
      setLoading(true)
      setError(null)
      try {
        const data = await apiService.getFileVersions(file.id)
        setVersions(data)
      } catch (err) {
        console.error('Failed to fetch versions:', err)
        setError('Failed to load version history')
      } finally {
        setLoading(false)
      }
    }

    fetchVersions()
  }, [file?.id])

  // Handle pin toggle
  const handleTogglePin = async (e: React.MouseEvent, version: FileVersion) => {
    e.stopPropagation()
    try {
      const updatedVersion = await apiService.pinFileVersion(version.id, !version.is_pinned)
      setVersions(prev => prev.map(v => v.id === version.id ? updatedVersion : v))
    } catch (err) {
      console.error('Failed to toggle pin:', err)
    }
  }

  // Handle delete (if unpinned)
  const handleDelete = async (e: React.MouseEvent, version: FileVersion) => {
    e.stopPropagation()
    if (version.is_pinned) return

    try {
      await apiService.deleteFileVersion(version.id)
      setVersions(prev => prev.filter(v => v.id !== version.id))
    } catch (err) {
      console.error('Failed to delete version:', err)
    }
  }

  if (!file) {
    return (
      <div className={cn("h-full flex flex-col items-center justify-center p-4 text-center", className)}>
        <Clock className="w-12 h-12 text-gray-600 mb-4" />
        <h3 className="text-gray-400 font-medium">No File Selected</h3>
        <p className="text-gray-500 text-sm mt-2">Select a file to view its history</p>
      </div>
    )
  }

  return (
    <div className={cn("h-full flex flex-col bg-gray-900/50", className)}>
      {/* Header */}
      <div className="p-4 border-b border-gray-800">
        <h3 className="text-white font-semibold flex items-center gap-2">
          <RotateCcw className="w-4 h-4 text-cyan-400" />
          Version History
        </h3>
        <p className="text-xs text-gray-500 mt-1 truncate">{file.name}</p>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-2 space-y-2">
        {loading ? (
          <div className="flex justify-center py-8">
            <Loading size="sm" />
          </div>
        ) : error ? (
          <div className="text-red-400 text-sm text-center py-4">{error}</div>
        ) : versions.length === 0 ? (
          <div className="text-gray-500 text-sm text-center py-8">
            No history available for this file
          </div>
        ) : (
          versions.map((version) => (
            <div
              key={version.id}
              className="group relative bg-gray-800/50 hover:bg-gray-800 rounded-lg p-3 border border-gray-700/50 hover:border-gray-600 transition-all cursor-pointer"
              onClick={() => onPreviewVersion(version)}
            >
              {/* Version Header */}
              <div className="flex justify-between items-start mb-2">
                <div className="flex items-center gap-2">
                  <Badge variant={version.is_auto_save ? 'outline' : 'primary'} size="xs">
                    v{version.version}
                  </Badge>
                  <span className="text-xs text-gray-400">
                    {format(new Date(version.created_at), 'MMM d, HH:mm')}
                  </span>
                </div>
                {version.is_pinned && (
                  <Pin className="w-3 h-3 text-cyan-400 fill-cyan-400" />
                )}
              </div>

              {/* Author & Summary */}
              <div className="mb-2">
                <div className="flex items-center gap-2 mb-1">
                  <div className="w-4 h-4 rounded-full bg-gradient-to-br from-purple-500 to-blue-500 flex items-center justify-center text-[8px] font-bold text-white">
                    {version.author_name?.[0]?.toUpperCase() || 'U'}
                  </div>
                  <span className="text-xs text-gray-300">{version.author_name}</span>
                </div>
                {version.change_summary && (
                  <p className="text-xs text-gray-400 line-clamp-2">
                    {version.change_summary}
                  </p>
                )}
              </div>

              {/* Stats */}
              <div className="flex items-center gap-3 text-[10px] text-gray-500 mb-2">
                <span className="text-green-400">+{version.lines_added}</span>
                <span className="text-red-400">-{version.lines_removed}</span>
                <span>{version.size}B</span>
              </div>

              {/* Actions (visible on hover) */}
              <div className="flex items-center gap-1 mt-2 opacity-0 group-hover:opacity-100 transition-opacity">
                <Button
                  size="xs"
                  variant="ghost"
                  className="h-6 px-2 text-cyan-400 hover:text-cyan-300 hover:bg-cyan-950/30"
                  onClick={(e) => {
                    e.stopPropagation()
                    onPreviewVersion(version)
                  }}
                  icon={<FileDiff size={12} />}
                >
                  Diff
                </Button>
                <Button
                  size="xs"
                  variant="ghost"
                  className="h-6 px-2 text-yellow-400 hover:text-yellow-300 hover:bg-yellow-950/30"
                  onClick={(e) => {
                    e.stopPropagation()
                    onRestoreVersion(version)
                  }}
                  icon={<RotateCcw size={12} />}
                >
                  Restore
                </Button>
                <div className="flex-1" />
                <button
                  className={cn(
                    "p-1 rounded hover:bg-gray-700 transition-colors",
                    version.is_pinned ? "text-cyan-400" : "text-gray-400 hover:text-white"
                  )}
                  onClick={(e) => handleTogglePin(e, version)}
                  title={version.is_pinned ? "Unpin version" : "Pin version"}
                >
                  <Pin size={12} className={cn(version.is_pinned && "fill-cyan-400")} />
                </button>
                {!version.is_pinned && (
                  <button
                    className="p-1 rounded hover:bg-red-900/30 text-gray-400 hover:text-red-400 transition-colors"
                    onClick={(e) => handleDelete(e, version)}
                    title="Delete version"
                  >
                    <Trash2 size={12} />
                  </button>
                )}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}
