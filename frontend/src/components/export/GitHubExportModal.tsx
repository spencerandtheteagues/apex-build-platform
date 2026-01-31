// APEX.BUILD GitHub Export Modal
// One-click export of projects to new GitHub repositories

import React, { useState, useEffect } from 'react'
import {
  Github, Lock, Globe, Loader2, Check, ExternalLink,
  AlertCircle, Download, X, FolderGit2, KeyRound,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import apiService from '@/services/api'

interface GitHubExportModalProps {
  projectId: number
  projectName: string
  projectDescription?: string
  isOpen: boolean
  onClose: () => void
}

type ExportStep = 'configure' | 'exporting' | 'success' | 'error'

export default function GitHubExportModal({
  projectId,
  projectName,
  projectDescription,
  isOpen,
  onClose,
}: GitHubExportModalProps) {
  const [step, setStep] = useState<ExportStep>('configure')
  const [repoName, setRepoName] = useState(
    projectName.toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/-+/g, '-')
  )
  const [description, setDescription] = useState(projectDescription || '')
  const [isPrivate, setIsPrivate] = useState(false)
  const [token, setToken] = useState('')
  const [error, setError] = useState('')
  const [result, setResult] = useState<{
    repo_url: string
    repo_owner: string
    repo_name: string
    commit_sha: string
    file_count: number
  } | null>(null)

  // Check existing export status
  const [existingRepo, setExistingRepo] = useState<string | null>(null)

  useEffect(() => {
    if (isOpen) {
      apiService.getExportStatus(projectId).then((res) => {
        if (res.exported && res.repository) {
          setExistingRepo(res.repository.remote_url)
        }
      }).catch(() => {})
    }
  }, [isOpen, projectId])

  // Reset state when modal opens
  useEffect(() => {
    if (isOpen) {
      setStep('configure')
      setError('')
      setResult(null)
    }
  }, [isOpen])

  const handleExport = async () => {
    if (!token.trim()) {
      setError('GitHub personal access token is required')
      return
    }
    if (!repoName.trim()) {
      setError('Repository name is required')
      return
    }

    setStep('exporting')
    setError('')

    try {
      const response = await apiService.exportToGitHub({
        project_id: projectId,
        repo_name: repoName.trim(),
        description: description.trim(),
        is_private: isPrivate,
        token: token.trim(),
      })

      if (response.success && response.data) {
        setResult(response.data)
        setStep('success')
      } else {
        setError(response.error || 'Export failed')
        setStep('error')
      }
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: string } }; message?: string }
      const msg = axiosErr?.response?.data?.error || axiosErr?.message || 'Export failed'
      setError(msg)
      setStep('error')
    }
  }

  const handleDownloadZip = async () => {
    try {
      await apiService.exportProject(projectId, projectName)
    } catch {
      setError('Failed to download ZIP')
    }
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/70 backdrop-blur-sm" onClick={onClose} />

      {/* Modal */}
      <div className="relative w-full max-w-lg mx-4 bg-gray-900/95 border border-gray-700/70 rounded-2xl shadow-2xl shadow-black/50 overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800/50">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-red-500/10 border border-red-500/30">
              <FolderGit2 className="w-5 h-5 text-red-400" />
            </div>
            <div>
              <h2 className="text-lg font-semibold text-white">Export Project</h2>
              <p className="text-xs text-gray-400">Push to GitHub or download as ZIP</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-gray-500 hover:text-white hover:bg-gray-800 transition-all"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Body */}
        <div className="px-6 py-5">
          {step === 'configure' && (
            <div className="space-y-5">
              {/* Existing repo notice */}
              {existingRepo && (
                <div className="flex items-start gap-2.5 p-3 rounded-lg bg-blue-500/10 border border-blue-500/20">
                  <Github className="w-4 h-4 text-blue-400 mt-0.5 shrink-0" />
                  <div className="text-sm text-gray-300">
                    Already connected to{' '}
                    <a
                      href={existingRepo}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-blue-400 hover:underline"
                    >
                      {existingRepo.replace('https://github.com/', '')}
                    </a>
                    . Exporting will create a <strong>new</strong> repository.
                  </div>
                </div>
              )}

              {/* GitHub Token */}
              <div>
                <label className="flex items-center gap-1.5 text-sm font-medium text-gray-300 mb-1.5">
                  <KeyRound className="w-3.5 h-3.5 text-gray-500" />
                  GitHub Personal Access Token
                </label>
                <input
                  type="password"
                  value={token}
                  onChange={(e) => { setToken(e.target.value); setError('') }}
                  placeholder="ghp_xxxxxxxxxxxxxxxxxxxx"
                  className="w-full px-3 py-2 bg-gray-800/70 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-red-500/50 focus:border-red-500/50 font-mono"
                />
                <p className="mt-1 text-[11px] text-gray-500">
                  Requires <code className="px-1 py-0.5 bg-gray-800 rounded text-gray-400">repo</code> scope.{' '}
                  <a
                    href="https://github.com/settings/tokens/new?scopes=repo&description=APEX.BUILD+Export"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-red-400 hover:underline"
                  >
                    Create token
                  </a>
                </p>
              </div>

              {/* Repo Name */}
              <div>
                <label className="text-sm font-medium text-gray-300 mb-1.5 block">
                  Repository Name
                </label>
                <input
                  type="text"
                  value={repoName}
                  onChange={(e) => {
                    setRepoName(e.target.value.replace(/[^a-zA-Z0-9._-]/g, '-'))
                    setError('')
                  }}
                  placeholder="my-project"
                  className="w-full px-3 py-2 bg-gray-800/70 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-red-500/50 focus:border-red-500/50"
                />
              </div>

              {/* Description */}
              <div>
                <label className="text-sm font-medium text-gray-300 mb-1.5 block">
                  Description <span className="text-gray-600">(optional)</span>
                </label>
                <input
                  type="text"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="A brief description of your project"
                  className="w-full px-3 py-2 bg-gray-800/70 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-red-500/50 focus:border-red-500/50"
                />
              </div>

              {/* Visibility */}
              <div>
                <label className="text-sm font-medium text-gray-300 mb-2 block">
                  Visibility
                </label>
                <div className="flex gap-3">
                  <button
                    type="button"
                    onClick={() => setIsPrivate(false)}
                    className={cn(
                      'flex-1 flex items-center gap-2 px-3 py-2.5 rounded-lg border text-sm transition-all',
                      !isPrivate
                        ? 'border-red-500/50 bg-red-500/10 text-white'
                        : 'border-gray-700 bg-gray-800/50 text-gray-400 hover:border-gray-600'
                    )}
                  >
                    <Globe className="w-4 h-4" />
                    Public
                  </button>
                  <button
                    type="button"
                    onClick={() => setIsPrivate(true)}
                    className={cn(
                      'flex-1 flex items-center gap-2 px-3 py-2.5 rounded-lg border text-sm transition-all',
                      isPrivate
                        ? 'border-red-500/50 bg-red-500/10 text-white'
                        : 'border-gray-700 bg-gray-800/50 text-gray-400 hover:border-gray-600'
                    )}
                  >
                    <Lock className="w-4 h-4" />
                    Private
                  </button>
                </div>
              </div>

              {/* Error */}
              {error && (
                <div className="flex items-start gap-2 p-3 rounded-lg bg-red-500/10 border border-red-500/20">
                  <AlertCircle className="w-4 h-4 text-red-400 mt-0.5 shrink-0" />
                  <span className="text-sm text-red-300">{error}</span>
                </div>
              )}
            </div>
          )}

          {step === 'exporting' && (
            <div className="flex flex-col items-center justify-center py-10 space-y-4">
              <Loader2 className="w-10 h-10 text-red-500 animate-spin" />
              <div className="text-center">
                <p className="text-white font-medium">Exporting to GitHub...</p>
                <p className="text-sm text-gray-400 mt-1">Creating repository and pushing files</p>
              </div>
            </div>
          )}

          {step === 'success' && result && (
            <div className="space-y-5">
              <div className="flex flex-col items-center py-6 space-y-3">
                <div className="p-3 rounded-full bg-green-500/20 border border-green-500/30">
                  <Check className="w-8 h-8 text-green-400" />
                </div>
                <div className="text-center">
                  <p className="text-lg font-semibold text-white">Export Complete</p>
                  <p className="text-sm text-gray-400 mt-1">
                    {result.file_count} files pushed to GitHub
                  </p>
                </div>
              </div>

              <div className="rounded-xl bg-gray-800/50 border border-gray-700/50 p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <span className="text-xs text-gray-500 uppercase">Repository</span>
                  <span className="text-sm text-white font-mono">
                    {result.repo_owner}/{result.repo_name}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-xs text-gray-500 uppercase">Commit</span>
                  <span className="text-sm text-gray-300 font-mono">
                    {result.commit_sha.substring(0, 7)}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-xs text-gray-500 uppercase">Files</span>
                  <span className="text-sm text-gray-300">{result.file_count}</span>
                </div>
              </div>

              <a
                href={result.repo_url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center justify-center gap-2 w-full px-4 py-2.5 rounded-lg bg-white/10 hover:bg-white/15 border border-white/20 text-white text-sm font-medium transition-all"
              >
                <Github className="w-4 h-4" />
                Open on GitHub
                <ExternalLink className="w-3.5 h-3.5" />
              </a>
            </div>
          )}

          {step === 'error' && (
            <div className="space-y-5">
              <div className="flex flex-col items-center py-6 space-y-3">
                <div className="p-3 rounded-full bg-red-500/20 border border-red-500/30">
                  <AlertCircle className="w-8 h-8 text-red-400" />
                </div>
                <div className="text-center">
                  <p className="text-lg font-semibold text-white">Export Failed</p>
                  <p className="text-sm text-red-300 mt-1 max-w-sm">{error}</p>
                </div>
              </div>

              <button
                onClick={() => setStep('configure')}
                className="w-full px-4 py-2.5 rounded-lg bg-gray-800 hover:bg-gray-700 border border-gray-700 text-white text-sm font-medium transition-all"
              >
                Try Again
              </button>
            </div>
          )}
        </div>

        {/* Footer */}
        {step === 'configure' && (
          <div className="px-6 py-4 border-t border-gray-800/50 flex items-center justify-between gap-3">
            {/* ZIP download as alternative */}
            <button
              onClick={handleDownloadZip}
              className="flex items-center gap-1.5 px-3 py-2 text-xs text-gray-400 hover:text-white border border-gray-700 hover:border-gray-600 rounded-lg transition-all"
            >
              <Download className="w-3.5 h-3.5" />
              Download ZIP
            </button>

            <div className="flex gap-2">
              <button
                onClick={onClose}
                className="px-4 py-2 text-sm text-gray-400 hover:text-white transition-all"
              >
                Cancel
              </button>
              <button
                onClick={handleExport}
                disabled={!token.trim() || !repoName.trim()}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-red-600 hover:bg-red-500 disabled:opacity-40 disabled:cursor-not-allowed text-white text-sm font-medium transition-all"
              >
                <Github className="w-4 h-4" />
                Export to GitHub
              </button>
            </div>
          </div>
        )}

        {step === 'success' && (
          <div className="px-6 py-4 border-t border-gray-800/50 flex justify-end">
            <button
              onClick={onClose}
              className="px-4 py-2 rounded-lg bg-gray-800 hover:bg-gray-700 text-white text-sm font-medium transition-all"
            >
              Done
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
