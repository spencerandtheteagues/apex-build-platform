// APEX.BUILD Git Integration Hook
// Manages Git state including branches, file changes, commits, and remote operations

import { useState, useCallback, useEffect, useRef } from 'react'
import apiService from '@/services/api'

export type FileStatus = 'modified' | 'added' | 'deleted' | 'renamed' | 'untracked'
export type FileStage = 'staged' | 'unstaged' | 'untracked'

export interface GitChangedFile {
  path: string
  name: string
  status: FileStatus
  stage: FileStage
  additions?: number
  deletions?: number
}

export interface GitCommit {
  hash: string
  shortHash: string
  message: string
  author: string
  authorEmail: string
  date: string
  relativeDate: string
}

export interface GitBranch {
  name: string
  isCurrent: boolean
  isRemote: boolean
  lastCommit?: string
}

export interface GitRemoteStatus {
  ahead: number
  behind: number
  remote: string
  branch: string
}

export interface GitState {
  currentBranch: string
  branches: GitBranch[]
  changes: GitChangedFile[]
  recentCommits: GitCommit[]
  remoteStatus: GitRemoteStatus | null
  isLoading: boolean
  isCommitting: boolean
  isPushing: boolean
  isPulling: boolean
  isFetching: boolean
  error: string | null
  commitMessage: string
}

export interface UseGitIntegrationReturn extends GitState {
  setCommitMessage: (message: string) => void
  fetchStatus: () => Promise<void>
  stageFile: (path: string) => Promise<void>
  unstageFile: (path: string) => Promise<void>
  stageAll: () => Promise<void>
  unstageAll: () => Promise<void>
  commit: (message?: string) => Promise<void>
  push: () => Promise<void>
  pull: () => Promise<void>
  switchBranch: (branchName: string) => Promise<void>
  createBranch: (branchName: string) => Promise<void>
  discardFileChanges: (path: string) => Promise<void>
  refresh: () => Promise<void>
}

const formatRelativeDate = (isoDate: string) => {
  const date = new Date(isoDate)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  if (diffMs < 60000) return 'just now'
  if (diffMs < 3600000) return `${Math.round(diffMs / 60000)}m ago`
  if (diffMs < 86400000) return `${Math.round(diffMs / 3600000)}h ago`
  if (diffMs < 604800000) return `${Math.round(diffMs / 86400000)}d ago`
  return date.toLocaleDateString()
}

const mapStatusToStage = (staged: boolean, status: string): FileStage => {
  if (staged) return 'staged'
  if (status === 'added') return 'untracked'
  return 'unstaged'
}

export function useGitIntegration(projectId: number | undefined): UseGitIntegrationReturn {
  const [currentBranch, setCurrentBranch] = useState('main')
  const [branches, setBranches] = useState<GitBranch[]>([])
  const [changes, setChanges] = useState<GitChangedFile[]>([])
  const [recentCommits, setRecentCommits] = useState<GitCommit[]>([])
  const [remoteStatus, setRemoteStatus] = useState<GitRemoteStatus | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isCommitting, setIsCommitting] = useState(false)
  const [isPushing, setIsPushing] = useState(false)
  const [isPulling, setIsPulling] = useState(false)
  const [isFetching, setIsFetching] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [commitMessage, setCommitMessage] = useState('')

  const changesRef = useRef(changes)
  const stageOverridesRef = useRef<Record<string, FileStage>>({})
  changesRef.current = changes

  const applyStageOverrides = useCallback((items: GitChangedFile[]) => {
    const overrides = stageOverridesRef.current
    if (!overrides || Object.keys(overrides).length === 0) return items
    return items.map((item) => {
      const override = overrides[item.path]
      return override ? { ...item, stage: override } : item
    })
  }, [])

  const buildChangesFromStatus = useCallback((status: {
    staged?: Array<{ path: string; status: string; staged?: boolean; additions?: number; deletions?: number }>
    unstaged?: Array<{ path: string; status: string; staged?: boolean; additions?: number; deletions?: number }>
  }) => {
    const staged = (status.staged || []).map((item) => ({
      path: item.path,
      name: item.path.split('/').pop() || item.path,
      status: (item.status as FileStatus) || 'modified',
      stage: mapStatusToStage(true, item.status),
      additions: item.additions || undefined,
      deletions: item.deletions || undefined,
    }))

    const unstaged = (status.unstaged || []).map((item) => ({
      path: item.path,
      name: item.path.split('/').pop() || item.path,
      status: (item.status as FileStatus) || 'modified',
      stage: mapStatusToStage(false, item.status),
      additions: item.additions || undefined,
      deletions: item.deletions || undefined,
    }))

    return applyStageOverrides([...staged, ...unstaged])
  }, [applyStageOverrides])

  const fetchStatus = useCallback(async () => {
    if (!projectId) return

    setIsFetching(true)
    setError(null)

    try {
      const statusRes = await apiService.getGitStatus(projectId)
      const mapped = buildChangesFromStatus(statusRes)
      setChanges(mapped)
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Failed to fetch git status')
      setChanges([])
    } finally {
      setIsFetching(false)
      setIsLoading(false)
    }
  }, [projectId, buildChangesFromStatus])

  const refresh = useCallback(async () => {
    if (!projectId) return
    setIsLoading(true)
    setError(null)

    try {
      const repoPromise = apiService.getGitRepo(projectId)
      const branchesPromise = apiService.getGitBranches(projectId)
      const statusPromise = apiService.getGitStatus(projectId)

      const repo = await repoPromise
      const branchName = repo?.repository?.branch || 'main'
      setCurrentBranch(branchName)

      const [branchesRes, statusRes, commitsRes] = await Promise.all([
        branchesPromise,
        statusPromise,
        apiService.getGitCommits(projectId, branchName, 20),
      ])

      const normalizedBranches: GitBranch[] = (branchesRes.branches || []).map((b) => ({
        name: b.name,
        isCurrent: b.name === branchName,
        isRemote: b.name.startsWith('origin/'),
        lastCommit: b.sha,
      }))
      setBranches(normalizedBranches)

      const normalizedCommits: GitCommit[] = (commitsRes.commits || []).map((c) => ({
        hash: c.sha,
        shortHash: c.sha.slice(0, 7),
        message: c.message,
        author: c.author,
        authorEmail: c.email,
        date: c.timestamp,
        relativeDate: formatRelativeDate(c.timestamp),
      }))
      setRecentCommits(normalizedCommits)

      const mappedChanges = buildChangesFromStatus(statusRes)
      setChanges(mappedChanges)

      if (repo?.repository?.branch) {
        setRemoteStatus({
          ahead: 0,
          behind: 0,
          remote: 'origin',
          branch: repo.repository.branch,
        })
      } else {
        setRemoteStatus(null)
      }
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Failed to refresh git data')
    } finally {
      setIsLoading(false)
      setIsFetching(false)
    }
  }, [projectId, buildChangesFromStatus])

  useEffect(() => {
    if (projectId) {
      refresh()
    } else {
      setChanges([])
      setBranches([])
      setRecentCommits([])
      setRemoteStatus(null)
      setError(null)
      setIsLoading(false)
    }
  }, [projectId, refresh])

  const stageFile = useCallback(async (path: string) => {
    stageOverridesRef.current = { ...stageOverridesRef.current, [path]: 'staged' }
    setChanges(prev => prev.map(f => f.path === path ? { ...f, stage: 'staged' } : f))
  }, [])

  const unstageFile = useCallback(async (path: string) => {
    stageOverridesRef.current = { ...stageOverridesRef.current, [path]: 'unstaged' }
    setChanges(prev => prev.map(f => f.path === path ? { ...f, stage: f.status === 'added' ? 'untracked' : 'unstaged' } : f))
  }, [])

  const stageAll = useCallback(async () => {
    const overrides: Record<string, FileStage> = {}
    changesRef.current.forEach(file => { overrides[file.path] = 'staged' })
    stageOverridesRef.current = { ...stageOverridesRef.current, ...overrides }
    setChanges(prev => prev.map(f => ({ ...f, stage: 'staged' })))
  }, [])

  const unstageAll = useCallback(async () => {
    const overrides: Record<string, FileStage> = {}
    changesRef.current.forEach(file => {
      overrides[file.path] = file.status === 'added' ? 'untracked' : 'unstaged'
    })
    stageOverridesRef.current = { ...stageOverridesRef.current, ...overrides }
    setChanges(prev => prev.map(f => ({ ...f, stage: f.status === 'added' ? 'untracked' : 'unstaged' })))
  }, [])

  const commit = useCallback(async (message?: string) => {
    if (!projectId) return
    const msg = message || commitMessage
    if (!msg.trim()) {
      setError('Commit message is required')
      return
    }

    const stagedFiles = changesRef.current.filter(f => f.stage === 'staged')
    if (stagedFiles.length === 0) {
      setError('No staged changes to commit')
      return
    }

    setIsCommitting(true)
    setError(null)

    try {
      await apiService.gitCommit(projectId, msg, stagedFiles.map(f => f.path))
      setCommitMessage('')

      // Clear stage overrides for committed files
      const overrides = { ...stageOverridesRef.current }
      stagedFiles.forEach(file => { delete overrides[file.path] })
      stageOverridesRef.current = overrides

      await refresh()
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Commit failed')
    } finally {
      setIsCommitting(false)
    }
  }, [projectId, commitMessage, refresh])

  const push = useCallback(async () => {
    if (!projectId) return
    setIsPushing(true)
    setError(null)

    try {
      await apiService.gitPush(projectId)
      await refresh()
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Push failed')
    } finally {
      setIsPushing(false)
    }
  }, [projectId, refresh])

  const pull = useCallback(async () => {
    if (!projectId) return
    setIsPulling(true)
    setError(null)

    try {
      await apiService.gitPull(projectId)
      await refresh()
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Pull failed')
    } finally {
      setIsPulling(false)
    }
  }, [projectId, refresh])

  const switchBranch = useCallback(async (branchName: string) => {
    if (!projectId) return
    setIsFetching(true)
    setError(null)

    try {
      await apiService.gitSwitchBranch(projectId, branchName)
      await refresh()
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Failed to switch branch')
    } finally {
      setIsFetching(false)
    }
  }, [projectId, refresh])

  const createBranch = useCallback(async (branchName: string) => {
    if (!projectId) return
    setIsFetching(true)
    setError(null)

    try {
      await apiService.gitCreateBranch(projectId, branchName, currentBranch)
      await refresh()
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Failed to create branch')
    } finally {
      setIsFetching(false)
    }
  }, [projectId, currentBranch, refresh])

  const discardFileChanges = useCallback(async (path: string) => {
    if (!projectId) return
    setIsFetching(true)
    setError(null)

    try {
      const files = await apiService.getFiles(projectId)
      const target = files.find(f => f.path === path)
      if (!target) {
        throw new Error('File not found for discard')
      }

      const versions = await apiService.getFileVersions(target.id)
      const sorted = [...versions].sort((a, b) => b.version - a.version)
      const currentVersion = target.version ?? sorted[0]?.version
      const previous = sorted.find(v => v.version < (currentVersion || 0))

      if (!previous) {
        throw new Error('No previous version to restore')
      }

      await apiService.restoreFileVersion(previous.id)
      await refresh()
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Discard failed')
    } finally {
      setIsFetching(false)
    }
  }, [projectId, refresh])

  return {
    currentBranch,
    branches,
    changes,
    recentCommits,
    remoteStatus,
    isLoading,
    isCommitting,
    isPushing,
    isPulling,
    isFetching,
    error,
    commitMessage,
    setCommitMessage,
    fetchStatus,
    stageFile,
    unstageFile,
    stageAll,
    unstageAll,
    commit,
    push,
    pull,
    switchBranch,
    createBranch,
    discardFileChanges,
    refresh,
  }
}
