// APEX.BUILD Asset Uploader
// Users upload images, CSVs, PDFs, videos — AI agents automatically use them when building.
// "Use my logo", "build a dashboard from this CSV" — it just works.

import React, { useState, useRef, useCallback, useEffect } from 'react'
import { cn } from '@/lib/utils'
import { ProjectAsset } from '@/types'
import apiService from '@/services/api'
import {
  Upload,
  FileImage,
  FileText,
  FileVideo,
  File,
  Trash2,
  CheckCircle,
  AlertCircle,
  Loader2,
  X,
} from 'lucide-react'

interface AssetUploaderProps {
  projectId: number
  className?: string
}

const FILE_TYPE_ICONS: Record<string, React.ReactNode> = {
  image: <FileImage className="w-4 h-4" />,
  video: <FileVideo className="w-4 h-4" />,
  csv: <FileText className="w-4 h-4" />,
  pdf: <FileText className="w-4 h-4" />,
  text: <FileText className="w-4 h-4" />,
  other: <File className="w-4 h-4" />,
}

const FILE_TYPE_COLORS: Record<string, string> = {
  image: 'text-purple-400',
  video: 'text-blue-400',
  csv: 'text-green-400',
  pdf: 'text-red-400',
  text: 'text-yellow-400',
  other: 'text-gray-400',
}

function formatSize(bytes: number): string {
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${bytes} B`
}

interface UploadState {
  file: File
  status: 'uploading' | 'done' | 'error'
  error?: string
}

export const AssetUploader: React.FC<AssetUploaderProps> = ({ projectId, className }) => {
  const [assets, setAssets] = useState<ProjectAsset[]>([])
  const [uploads, setUploads] = useState<UploadState[]>([])
  const [dragging, setDragging] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  // Load existing assets
  useEffect(() => {
    let mounted = true
    apiService.listAssets(projectId).then(list => {
      if (mounted) { setAssets(list); setLoading(false) }
    }).catch(err => {
      if (mounted) { setError('Failed to load assets'); setLoading(false) }
    })
    return () => { mounted = false }
  }, [projectId])

  const uploadFiles = useCallback(async (files: FileList | File[]) => {
    const fileArray = Array.from(files)
    // Add to upload queue
    const newUploads: UploadState[] = fileArray.map(f => ({ file: f, status: 'uploading' }))
    setUploads(prev => [...prev, ...newUploads])

    for (let i = 0; i < fileArray.length; i++) {
      const file = fileArray[i]
      try {
        const { asset } = await apiService.uploadAsset(projectId, file)
        setAssets(prev => [asset, ...prev])
        setUploads(prev => prev.map((u, idx) =>
          u.file === file ? { ...u, status: 'done' } : u
        ))
        // Clear done uploads after 2s
        setTimeout(() => {
          setUploads(prev => prev.filter(u => u.file !== file))
        }, 2000)
      } catch (err: any) {
        const msg = err?.response?.data?.error || 'Upload failed'
        setUploads(prev => prev.map(u =>
          u.file === file ? { ...u, status: 'error', error: msg } : u
        ))
      }
    }
  }, [projectId])

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setDragging(false)
    if (e.dataTransfer.files.length > 0) {
      uploadFiles(e.dataTransfer.files)
    }
  }, [uploadFiles])

  const handleFileInput = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      uploadFiles(e.target.files)
      e.target.value = '' // Reset so same file can be re-uploaded
    }
  }, [uploadFiles])

  const deleteAsset = async (asset: ProjectAsset) => {
    try {
      await apiService.deleteAsset(projectId, asset.id)
      setAssets(prev => prev.filter(a => a.id !== asset.id))
    } catch {
      setError('Failed to delete asset')
    }
  }

  return (
    <div className={cn('flex flex-col gap-3', className)}>
      {/* Header */}
      <div className="flex items-center gap-2 text-sm text-gray-400">
        <Upload className="w-4 h-4 text-red-400" />
        <span>Upload files for AI agents</span>
        <span className="ml-auto text-xs text-gray-600">
          Images, CSV, PDF, text, video • Max 50 MB
        </span>
      </div>

      {/* Drop zone */}
      <div
        onDragOver={e => { e.preventDefault(); setDragging(true) }}
        onDragLeave={() => setDragging(false)}
        onDrop={handleDrop}
        onClick={() => fileInputRef.current?.click()}
        className={cn(
          'relative flex flex-col items-center justify-center gap-2 rounded border-2 border-dashed',
          'cursor-pointer select-none transition-colors duration-150 p-5',
          dragging
            ? 'border-red-500 bg-red-900/10 text-red-300'
            : 'border-gray-700 bg-gray-900/40 text-gray-500 hover:border-gray-600 hover:text-gray-400',
        )}
      >
        <Upload className="w-6 h-6" />
        <p className="text-xs text-center">
          {dragging ? 'Drop files here' : 'Drop files here or click to browse'}
        </p>
        <input
          ref={fileInputRef}
          type="file"
          multiple
          accept="image/*,video/*,text/csv,text/plain,application/pdf,application/json,.csv,.pdf,.txt,.json,.md"
          className="hidden"
          onChange={handleFileInput}
        />
      </div>

      {/* Active uploads */}
      {uploads.length > 0 && (
        <div className="flex flex-col gap-1">
          {uploads.map((u, i) => (
            <div key={i} className="flex items-center gap-2 rounded bg-gray-800/60 px-3 py-1.5 text-xs">
              {u.status === 'uploading' && <Loader2 className="w-3 h-3 animate-spin text-blue-400" />}
              {u.status === 'done' && <CheckCircle className="w-3 h-3 text-green-400" />}
              {u.status === 'error' && <AlertCircle className="w-3 h-3 text-red-400" />}
              <span className="flex-1 truncate text-gray-300">{u.file.name}</span>
              {u.status === 'error' && (
                <span className="text-red-400 ml-auto">{u.error}</span>
              )}
              {u.status === 'uploading' && (
                <span className="text-gray-500">{formatSize(u.file.size)}</span>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="flex items-center gap-2 rounded bg-red-900/20 border border-red-900/50 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="w-3 h-3 flex-shrink-0" />
          {error}
          <button onClick={() => setError(null)} className="ml-auto">
            <X className="w-3 h-3" />
          </button>
        </div>
      )}

      {/* Existing assets list */}
      {loading ? (
        <div className="flex items-center gap-2 text-xs text-gray-600">
          <Loader2 className="w-3 h-3 animate-spin" />
          Loading assets...
        </div>
      ) : assets.length > 0 ? (
        <div className="flex flex-col gap-1">
          <p className="text-xs text-gray-600">
            {assets.length} file{assets.length !== 1 ? 's' : ''} — AI agents will use these automatically
          </p>
          {assets.map(asset => (
            <div
              key={asset.id}
              className="group flex items-center gap-2 rounded bg-gray-800/40 px-3 py-1.5 text-xs hover:bg-gray-800/70 transition-colors"
            >
              <span className={cn(FILE_TYPE_COLORS[asset.file_type] || 'text-gray-400')}>
                {FILE_TYPE_ICONS[asset.file_type] || <File className="w-4 h-4" />}
              </span>
              <span className="flex-1 truncate text-gray-300" title={asset.original_name}>
                {asset.original_name}
              </span>
              <span className="text-gray-600 tabular-nums">{formatSize(asset.file_size)}</span>
              <button
                onClick={() => deleteAsset(asset)}
                className="ml-1 opacity-0 group-hover:opacity-100 transition-opacity text-gray-600 hover:text-red-400"
                title="Delete asset"
              >
                <Trash2 className="w-3 h-3" />
              </button>
            </div>
          ))}
        </div>
      ) : (
        <p className="text-xs text-gray-700 text-center">
          No files yet. Upload a logo, CSV dataset, PDF doc, or any file your app needs.
        </p>
      )}
    </div>
  )
}

export default AssetUploader
