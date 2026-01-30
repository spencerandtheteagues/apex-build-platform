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

// Simulated Git data for when no real backend Git API exists yet.
// This provides a working UI that demonstrates the panel's capabilities.
function generateSimulatedData(projectId: number | undefined) {
  const branches: GitBranch[] = [
    { name: 'main', isCurrent: true, isRemote: false, lastCommit: 'abc1234' },
    { name: 'develop', isCurrent: false, isRemote: false, lastCommit: 'def5678' },
    { name: 'feature/search-panel', isCurrent: false, isRemote: false, lastCommit: 'ghi9012' },
    { name: 'origin/main', isCurrent: false, isRemote: true, lastCommit: 'abc1234' },
    { name: 'origin/develop', isCurrent: false, isRemote: true, lastCommit: 'xyz7890' },
  ]

  const changes: GitChangedFile[] = [
    { path: 'src/components/ide/SearchPanel.tsx', name: 'SearchPanel.tsx', status: 'added', stage: 'staged', additions: 340, deletions: 0 },
    { path: 'src/components/ide/GitPanel.tsx', name: 'GitPanel.tsx', status: 'added', stage: 'staged', additions: 520, deletions: 0 },
    { path: 'src/hooks/useProjectSearch.ts', name: 'useProjectSearch.ts', status: 'added', stage: 'unstaged', additions: 220, deletions: 0 },
    { path: 'src/hooks/useGitIntegration.ts', name: 'useGitIntegration.ts', status: 'added', stage: 'unstaged', additions: 280, deletions: 0 },
    { path: 'src/components/ide/IDELayout.tsx', name: 'IDELayout.tsx', status: 'modified', stage: 'unstaged', additions: 8, deletions: 12 },
    { path: 'src/services/api.ts', name: 'api.ts', status: 'modified', stage: 'unstaged', additions: 45, deletions: 2 },
  ]

  const commits: GitCommit[] = [
    { hash: 'd3dcffa1b2c3d4e5', shortHash: 'd3dcffa', message: 'Fix database connection by removing SQLite dependency', author: 'Spencer', authorEmail: 'spencer@apex.build', date: '2026-01-30T10:00:00Z', relativeDate: '2 hours ago' },
    { hash: 'a72cff71a2b3c4d5', shortHash: 'a72cff7', message: 'Add Gemini session handoff log', author: 'Spencer', authorEmail: 'spencer@apex.build', date: '2026-01-29T18:00:00Z', relativeDate: '18 hours ago' },
    { hash: '78eeb4b1a2b3c4d5', shortHash: '78eeb4b', message: 'Fix duplicate exports in ApexComponents.tsx', author: 'Spencer', authorEmail: 'spencer@apex.build', date: '2026-01-29T14:00:00Z', relativeDate: '22 hours ago' },
    { hash: '5f540741a2b3c4d5', shortHash: '5f54074', message: 'Final Upgrade: Replit Parity achieved. Added 3D visuals, Replit Migration, Enterprise Workspace, and Database management.', author: 'Spencer', authorEmail: 'spencer@apex.build', date: '2026-01-28T20:00:00Z', relativeDate: '2 days ago' },
    { hash: '493b0221a2b3c4d5', shortHash: '493b022', message: 'Integrate real API data for Explore page and implement fork functionality', author: 'Spencer', authorEmail: 'spencer@apex.build', date: '2026-01-28T15:00:00Z', relativeDate: '2 days ago' },
    { hash: 'e1f2a3b4c5d6e7f8', shortHash: 'e1f2a3b', message: 'Add code comments panel with reactions and threading', author: 'Spencer', authorEmail: 'spencer@apex.build', date: '2026-01-27T22:00:00Z', relativeDate: '3 days ago' },
    { hash: 'f8e7d6c5b4a39281', shortHash: 'f8e7d6c', message: 'Implement GitHub import wizard with stack detection', author: 'Spencer', authorEmail: 'spencer@apex.build', date: '2026-01-27T16:00:00Z', relativeDate: '3 days ago' },
    { hash: '0a1b2c3d4e5f6789', shortHash: '0a1b2c3', message: 'Add real-time collaboration with WebSocket cursors', author: 'Spencer', authorEmail: 'spencer@apex.build', date: '2026-01-26T12:00:00Z', relativeDate: '4 days ago' },
    { hash: '9876543210abcdef', shortHash: '9876543', message: 'Implement AI assistant with multi-provider support', author: 'Spencer', authorEmail: 'spencer@apex.build', date: '2026-01-25T09:00:00Z', relativeDate: '5 days ago' },
    { hash: 'fedcba9876543210', shortHash: 'fedcba9', message: 'Initial project scaffold with dark cyberpunk theme', author: 'Spencer', authorEmail: 'spencer@apex.build', date: '2026-01-24T08:00:00Z', relativeDate: '6 days ago' },
  ]

  const remoteStatus: GitRemoteStatus = {
    ahead: 2,
    behind: 0,
    remote: 'origin',
    branch: 'main',
  }

  return { branches, changes, commits, remoteStatus }
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
  changesRef.current = changes

  // Fetch git status on mount and when projectId changes
  useEffect(() => {
    if (projectId) {
      fetchStatus()
    }
  }, [projectId])

  const fetchStatus = useCallback(async () => {
    if (!projectId) return

    setIsLoading(true)
    setError(null)

    try {
      // Use simulated data - in a real implementation this would call
      // apiService.getGitStatus(projectId) or similar endpoint
      const data = generateSimulatedData(projectId)

      const currentBr = data.branches.find(b => b.isCurrent)
      if (currentBr) {
        setCurrentBranch(currentBr.name)
      }

      setBranches(data.branches)
      setChanges(data.changes)
      setRecentCommits(data.commits)
      setRemoteStatus(data.remoteStatus)
    } catch (err: any) {
      setError(err.message || 'Failed to fetch git status')
    } finally {
      setIsLoading(false)
    }
  }, [projectId])

  const stageFile = useCallback(async (path: string) => {
    setChanges(prev =>
      prev.map(f =>
        f.path === path ? { ...f, stage: 'staged' as FileStage } : f
      )
    )
  }, [])

  const unstageFile = useCallback(async (path: string) => {
    setChanges(prev =>
      prev.map(f =>
        f.path === path
          ? { ...f, stage: (f.status === 'untracked' ? 'untracked' : 'unstaged') as FileStage }
          : f
      )
    )
  }, [])

  const stageAll = useCallback(async () => {
    setChanges(prev =>
      prev.map(f => ({ ...f, stage: 'staged' as FileStage }))
    )
  }, [])

  const unstageAll = useCallback(async () => {
    setChanges(prev =>
      prev.map(f => ({
        ...f,
        stage: (f.status === 'untracked' ? 'untracked' : 'unstaged') as FileStage,
      }))
    )
  }, [])

  const commit = useCallback(async (message?: string) => {
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
      // Simulate commit - real implementation: apiService.gitCommit(projectId, msg)
      await new Promise(resolve => setTimeout(resolve, 800))

      const newCommit: GitCommit = {
        hash: Math.random().toString(16).substring(2, 18),
        shortHash: Math.random().toString(16).substring(2, 9),
        message: msg,
        author: 'You',
        authorEmail: 'you@apex.build',
        date: new Date().toISOString(),
        relativeDate: 'just now',
      }

      setRecentCommits(prev => [newCommit, ...prev.slice(0, 9)])
      setChanges(prev => prev.filter(f => f.stage !== 'staged'))
      setCommitMessage('')

      if (remoteStatus) {
        setRemoteStatus({
          ...remoteStatus,
          ahead: remoteStatus.ahead + 1,
        })
      }
    } catch (err: any) {
      setError(err.message || 'Commit failed')
    } finally {
      setIsCommitting(false)
    }
  }, [commitMessage, remoteStatus])

  const push = useCallback(async () => {
    setIsPushing(true)
    setError(null)

    try {
      // Simulate push - real implementation: apiService.gitPush(projectId)
      await new Promise(resolve => setTimeout(resolve, 1200))

      if (remoteStatus) {
        setRemoteStatus({ ...remoteStatus, ahead: 0 })
      }
    } catch (err: any) {
      setError(err.message || 'Push failed')
    } finally {
      setIsPushing(false)
    }
  }, [remoteStatus])

  const pull = useCallback(async () => {
    setIsPulling(true)
    setError(null)

    try {
      // Simulate pull - real implementation: apiService.gitPull(projectId)
      await new Promise(resolve => setTimeout(resolve, 1000))

      if (remoteStatus) {
        setRemoteStatus({ ...remoteStatus, behind: 0 })
      }
    } catch (err: any) {
      setError(err.message || 'Pull failed')
    } finally {
      setIsPulling(false)
    }
  }, [remoteStatus])

  const switchBranch = useCallback(async (branchName: string) => {
    setIsLoading(true)
    setError(null)

    try {
      // Simulate branch switch
      await new Promise(resolve => setTimeout(resolve, 500))

      setBranches(prev =>
        prev.map(b => ({
          ...b,
          isCurrent: b.name === branchName,
        }))
      )
      setCurrentBranch(branchName)
    } catch (err: any) {
      setError(err.message || 'Failed to switch branch')
    } finally {
      setIsLoading(false)
    }
  }, [])

  const createBranch = useCallback(async (branchName: string) => {
    if (!branchName.trim()) {
      setError('Branch name is required')
      return
    }

    setIsLoading(true)
    setError(null)

    try {
      // Simulate branch creation
      await new Promise(resolve => setTimeout(resolve, 400))

      const newBranch: GitBranch = {
        name: branchName,
        isCurrent: true,
        isRemote: false,
        lastCommit: recentCommits[0]?.shortHash,
      }

      setBranches(prev => [
        newBranch,
        ...prev.map(b => ({ ...b, isCurrent: false })),
      ])
      setCurrentBranch(branchName)
    } catch (err: any) {
      setError(err.message || 'Failed to create branch')
    } finally {
      setIsLoading(false)
    }
  }, [recentCommits])

  const discardFileChanges = useCallback(async (path: string) => {
    try {
      // Simulate discarding changes
      setChanges(prev => prev.filter(f => f.path !== path))
    } catch (err: any) {
      setError(err.message || 'Failed to discard changes')
    }
  }, [])

  const refresh = useCallback(async () => {
    await fetchStatus()
  }, [fetchStatus])

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

export default useGitIntegration
