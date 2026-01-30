// APEX.BUILD Git Integration Panel
// Full Git workflow: branch display, changed files, staging, commit, push/pull,
// branch switching, and recent commit history

import React, { useState, useCallback, useMemo } from 'react'
import { cn } from '@/lib/utils'
import { useGitIntegration, GitChangedFile, GitCommit, GitBranch } from '@/hooks/useGitIntegration'
import { Button, Badge, Loading } from '@/components/ui'
import {
  GitBranch as GitBranchIcon,
  GitCommit as GitCommitIcon,
  GitPullRequest,
  Plus,
  Minus,
  FileText,
  FilePlus,
  FileMinus,
  FileEdit,
  FileQuestion,
  Check,
  CheckCheck,
  X,
  ChevronDown,
  ChevronRight,
  Upload,
  Download,
  RotateCcw,
  Clock,
  User,
  Hash,
  AlertCircle,
  Undo2,
  RefreshCw,
} from 'lucide-react'

export interface GitPanelProps {
  projectId: number | undefined
  className?: string
}

type GitSection = 'changes' | 'commits'

export const GitPanel: React.FC<GitPanelProps> = ({
  projectId,
  className,
}) => {
  const git = useGitIntegration(projectId)

  const [activeSection, setActiveSection] = useState<GitSection>('changes')
  const [showBranchDropdown, setShowBranchDropdown] = useState(false)
  const [newBranchName, setNewBranchName] = useState('')
  const [showNewBranch, setShowNewBranch] = useState(false)
  const [expandStaged, setExpandStaged] = useState(true)
  const [expandUnstaged, setExpandUnstaged] = useState(true)

  // Categorize files
  const stagedFiles = useMemo(
    () => git.changes.filter(f => f.stage === 'staged'),
    [git.changes]
  )
  const unstagedFiles = useMemo(
    () => git.changes.filter(f => f.stage === 'unstaged' || f.stage === 'untracked'),
    [git.changes]
  )

  const hasChanges = git.changes.length > 0
  const hasStagedChanges = stagedFiles.length > 0

  const handleCommit = useCallback(() => {
    if (git.commitMessage.trim() && hasStagedChanges) {
      git.commit()
    }
  }, [git, hasStagedChanges])

  const handleCreateBranch = useCallback(() => {
    if (newBranchName.trim()) {
      git.createBranch(newBranchName.trim())
      setNewBranchName('')
      setShowNewBranch(false)
      setShowBranchDropdown(false)
    }
  }, [git, newBranchName])

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'modified':
        return <FileEdit size={14} className="text-yellow-400" />
      case 'added':
        return <FilePlus size={14} className="text-green-400" />
      case 'deleted':
        return <FileMinus size={14} className="text-red-400" />
      case 'renamed':
        return <FileText size={14} className="text-blue-400" />
      case 'untracked':
        return <FileQuestion size={14} className="text-gray-400" />
      default:
        return <FileText size={14} className="text-gray-400" />
    }
  }

  const getStatusLabel = (status: string) => {
    switch (status) {
      case 'modified': return 'M'
      case 'added': return 'A'
      case 'deleted': return 'D'
      case 'renamed': return 'R'
      case 'untracked': return 'U'
      default: return '?'
    }
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'modified': return 'text-yellow-400'
      case 'added': return 'text-green-400'
      case 'deleted': return 'text-red-400'
      case 'renamed': return 'text-blue-400'
      case 'untracked': return 'text-gray-400'
      default: return 'text-gray-400'
    }
  }

  const renderFileItem = (file: GitChangedFile, isStaged: boolean) => (
    <div
      key={file.path}
      className="flex items-center gap-2 px-3 py-1.5 hover:bg-gray-800/50 transition-colors group"
    >
      {/* Stage/Unstage checkbox */}
      <button
        onClick={() => isStaged ? git.unstageFile(file.path) : git.stageFile(file.path)}
        className={cn(
          'w-4 h-4 rounded border flex items-center justify-center transition-colors shrink-0',
          isStaged
            ? 'bg-green-500/20 border-green-500 text-green-400'
            : 'border-gray-600 hover:border-gray-400'
        )}
        title={isStaged ? 'Unstage file' : 'Stage file'}
      >
        {isStaged && <Check size={10} />}
      </button>

      {/* Status icon */}
      {getStatusIcon(file.status)}

      {/* File name */}
      <span className="text-sm text-gray-300 truncate flex-1" title={file.path}>
        {file.name}
      </span>

      {/* Additions/Deletions */}
      {(file.additions !== undefined || file.deletions !== undefined) && (
        <div className="flex items-center gap-1 text-xs shrink-0">
          {file.additions !== undefined && file.additions > 0 && (
            <span className="text-green-400">+{file.additions}</span>
          )}
          {file.deletions !== undefined && file.deletions > 0 && (
            <span className="text-red-400">-{file.deletions}</span>
          )}
        </div>
      )}

      {/* Status letter badge */}
      <span className={cn('text-xs font-mono shrink-0', getStatusColor(file.status))}>
        {getStatusLabel(file.status)}
      </span>

      {/* Discard button (only for unstaged modified files) */}
      {!isStaged && file.status !== 'untracked' && (
        <button
          onClick={() => git.discardFileChanges(file.path)}
          className="opacity-0 group-hover:opacity-100 p-0.5 text-gray-600 hover:text-red-400 transition-all"
          title="Discard changes"
        >
          <Undo2 size={12} />
        </button>
      )}
    </div>
  )

  const renderCommit = (commit: GitCommit, index: number) => (
    <div
      key={commit.hash}
      className={cn(
        'px-3 py-2.5 transition-colors',
        index === 0 ? 'bg-gray-800/30' : 'hover:bg-gray-800/30'
      )}
    >
      <div className="flex items-start gap-2">
        {/* Commit graph dot */}
        <div className="flex flex-col items-center mt-1.5 shrink-0">
          <div className={cn(
            'w-2.5 h-2.5 rounded-full border-2',
            index === 0
              ? 'bg-red-500 border-red-400'
              : 'bg-gray-700 border-gray-500'
          )} />
          {index < git.recentCommits.length - 1 && (
            <div className="w-px h-full bg-gray-700 mt-1" style={{ minHeight: '20px' }} />
          )}
        </div>

        <div className="flex-1 min-w-0">
          {/* Commit message */}
          <p className="text-sm text-white truncate" title={commit.message}>
            {commit.message}
          </p>

          {/* Meta info */}
          <div className="flex items-center gap-3 mt-1">
            <div className="flex items-center gap-1 text-xs text-gray-500">
              <Hash size={10} />
              <span className="font-mono">{commit.shortHash}</span>
            </div>
            <div className="flex items-center gap-1 text-xs text-gray-500">
              <User size={10} />
              <span>{commit.author}</span>
            </div>
            <div className="flex items-center gap-1 text-xs text-gray-500">
              <Clock size={10} />
              <span>{commit.relativeDate}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )

  if (git.isLoading && git.changes.length === 0) {
    return (
      <div className={cn('flex flex-col h-full bg-gray-900/80 items-center justify-center', className)}>
        <Loading size="md" variant="spinner" />
        <p className="text-sm text-gray-400 mt-3">Loading git status...</p>
      </div>
    )
  }

  return (
    <div className={cn('flex flex-col h-full bg-gray-900/80', className)}>
      {/* Branch header */}
      <div className="p-3 border-b border-gray-700/50">
        {/* Current branch */}
        <div className="relative">
          <button
            onClick={() => setShowBranchDropdown(!showBranchDropdown)}
            className="flex items-center gap-2 px-2.5 py-1.5 bg-gray-800 border border-gray-600 rounded hover:border-gray-500 transition-colors w-full"
          >
            <GitBranchIcon size={14} className="text-red-400 shrink-0" />
            <span className="text-sm text-white truncate flex-1 text-left">{git.currentBranch}</span>
            <ChevronDown size={12} className="text-gray-500 shrink-0" />
          </button>

          {/* Branch dropdown */}
          {showBranchDropdown && (
            <div className="absolute top-full left-0 right-0 mt-1 bg-gray-800 border border-gray-600 rounded-lg shadow-xl z-20 max-h-64 overflow-y-auto">
              {/* Local branches */}
              <div className="p-1.5 border-b border-gray-700">
                <span className="text-xs font-medium text-gray-500 px-2">LOCAL BRANCHES</span>
              </div>
              {git.branches
                .filter(b => !b.isRemote)
                .map(branch => (
                  <button
                    key={branch.name}
                    onClick={() => {
                      git.switchBranch(branch.name)
                      setShowBranchDropdown(false)
                    }}
                    className={cn(
                      'w-full flex items-center gap-2 px-3 py-1.5 hover:bg-gray-700 transition-colors text-left',
                      branch.isCurrent && 'bg-gray-700/50'
                    )}
                  >
                    <GitBranchIcon size={12} className={branch.isCurrent ? 'text-red-400' : 'text-gray-500'} />
                    <span className={cn('text-sm flex-1', branch.isCurrent ? 'text-white' : 'text-gray-300')}>
                      {branch.name}
                    </span>
                    {branch.isCurrent && (
                      <Check size={12} className="text-green-400" />
                    )}
                  </button>
                ))}

              {/* Remote branches */}
              {git.branches.some(b => b.isRemote) && (
                <>
                  <div className="p-1.5 border-t border-gray-700">
                    <span className="text-xs font-medium text-gray-500 px-2">REMOTE BRANCHES</span>
                  </div>
                  {git.branches
                    .filter(b => b.isRemote)
                    .map(branch => (
                      <button
                        key={branch.name}
                        onClick={() => {
                          git.switchBranch(branch.name.replace('origin/', ''))
                          setShowBranchDropdown(false)
                        }}
                        className="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-gray-700 transition-colors text-left"
                      >
                        <GitBranchIcon size={12} className="text-gray-600" />
                        <span className="text-sm text-gray-400">{branch.name}</span>
                      </button>
                    ))}
                </>
              )}

              {/* Create new branch */}
              <div className="border-t border-gray-700 p-2">
                {showNewBranch ? (
                  <div className="flex items-center gap-1">
                    <input
                      type="text"
                      value={newBranchName}
                      onChange={(e) => setNewBranchName(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') handleCreateBranch()
                        if (e.key === 'Escape') {
                          setShowNewBranch(false)
                          setNewBranchName('')
                        }
                      }}
                      placeholder="branch-name"
                      className="flex-1 bg-gray-900 border border-gray-600 rounded px-2 py-1 text-sm text-white placeholder:text-gray-500 focus:border-red-500 focus:outline-none"
                      autoFocus
                    />
                    <Button
                      size="xs"
                      variant="primary"
                      onClick={handleCreateBranch}
                      disabled={!newBranchName.trim()}
                      icon={<Check size={12} />}
                    />
                    <Button
                      size="xs"
                      variant="ghost"
                      onClick={() => {
                        setShowNewBranch(false)
                        setNewBranchName('')
                      }}
                      icon={<X size={12} />}
                    />
                  </div>
                ) : (
                  <button
                    onClick={() => setShowNewBranch(true)}
                    className="w-full flex items-center gap-2 px-2 py-1.5 text-sm text-gray-400 hover:text-white hover:bg-gray-700 rounded transition-colors"
                  >
                    <Plus size={12} />
                    Create new branch
                  </button>
                )}
              </div>
            </div>
          )}
        </div>

        {/* Remote status and actions */}
        <div className="flex items-center gap-2 mt-2">
          {git.remoteStatus && (
            <div className="flex items-center gap-2 text-xs text-gray-500 flex-1">
              {git.remoteStatus.ahead > 0 && (
                <span className="flex items-center gap-1 text-yellow-400">
                  <Upload size={10} />
                  {git.remoteStatus.ahead} ahead
                </span>
              )}
              {git.remoteStatus.behind > 0 && (
                <span className="flex items-center gap-1 text-blue-400">
                  <Download size={10} />
                  {git.remoteStatus.behind} behind
                </span>
              )}
              {git.remoteStatus.ahead === 0 && git.remoteStatus.behind === 0 && (
                <span className="flex items-center gap-1 text-green-400">
                  <CheckCheck size={10} />
                  Up to date
                </span>
              )}
            </div>
          )}

          <div className="flex items-center gap-1">
            <Button
              size="xs"
              variant="ghost"
              onClick={() => git.pull()}
              loading={git.isPulling}
              disabled={git.isPulling || git.isPushing}
              icon={<Download size={12} />}
              title="Pull"
              className="text-gray-400 hover:text-white"
            />
            <Button
              size="xs"
              variant="ghost"
              onClick={() => git.push()}
              loading={git.isPushing}
              disabled={git.isPulling || git.isPushing}
              icon={<Upload size={12} />}
              title="Push"
              className="text-gray-400 hover:text-white"
            />
            <Button
              size="xs"
              variant="ghost"
              onClick={() => git.refresh()}
              loading={git.isLoading}
              icon={<RefreshCw size={12} />}
              title="Refresh"
              className="text-gray-400 hover:text-white"
            />
          </div>
        </div>
      </div>

      {/* Section tabs */}
      <div className="flex border-b border-gray-700/50">
        <button
          onClick={() => setActiveSection('changes')}
          className={cn(
            'flex-1 flex items-center justify-center gap-1.5 px-3 py-2 text-xs font-medium transition-colors',
            activeSection === 'changes'
              ? 'text-white border-b-2 border-red-500 bg-gray-800/30'
              : 'text-gray-500 hover:text-gray-300'
          )}
        >
          <FileEdit size={12} />
          Changes
          {hasChanges && (
            <Badge variant="outline" size="xs">{git.changes.length}</Badge>
          )}
        </button>
        <button
          onClick={() => setActiveSection('commits')}
          className={cn(
            'flex-1 flex items-center justify-center gap-1.5 px-3 py-2 text-xs font-medium transition-colors',
            activeSection === 'commits'
              ? 'text-white border-b-2 border-red-500 bg-gray-800/30'
              : 'text-gray-500 hover:text-gray-300'
          )}
        >
          <GitCommitIcon size={12} />
          Commits
          <Badge variant="outline" size="xs">{git.recentCommits.length}</Badge>
        </button>
      </div>

      {/* Error display */}
      {git.error && (
        <div className="px-3 py-2">
          <div className="bg-red-500/10 border border-red-500/30 rounded p-2 flex items-start gap-2">
            <AlertCircle size={14} className="text-red-400 mt-0.5 shrink-0" />
            <div className="flex-1">
              <p className="text-xs text-red-400">{git.error}</p>
            </div>
            <button
              onClick={() => git.refresh()}
              className="text-red-400 hover:text-red-300"
            >
              <X size={12} />
            </button>
          </div>
        </div>
      )}

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {activeSection === 'changes' && (
          <div className="flex flex-col h-full">
            {/* Staged changes */}
            <div>
              <button
                onClick={() => setExpandStaged(!expandStaged)}
                className="w-full flex items-center gap-2 px-3 py-2 hover:bg-gray-800/30 transition-colors"
              >
                {expandStaged ? (
                  <ChevronDown size={12} className="text-gray-500" />
                ) : (
                  <ChevronRight size={12} className="text-gray-500" />
                )}
                <span className="text-xs font-medium text-green-400 uppercase tracking-wide">
                  Staged Changes
                </span>
                <Badge variant="success" size="xs">{stagedFiles.length}</Badge>
                <div className="flex-1" />
                {stagedFiles.length > 0 && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation()
                      git.unstageAll()
                    }}
                    className="text-xs text-gray-500 hover:text-gray-300 transition-colors"
                    title="Unstage all"
                  >
                    <Minus size={12} />
                  </button>
                )}
              </button>

              {expandStaged && stagedFiles.length > 0 && (
                <div className="pb-1">
                  {stagedFiles.map(file => renderFileItem(file, true))}
                </div>
              )}

              {expandStaged && stagedFiles.length === 0 && (
                <p className="px-3 py-2 text-xs text-gray-600 italic">
                  No staged changes
                </p>
              )}
            </div>

            {/* Unstaged / untracked changes */}
            <div>
              <button
                onClick={() => setExpandUnstaged(!expandUnstaged)}
                className="w-full flex items-center gap-2 px-3 py-2 hover:bg-gray-800/30 transition-colors"
              >
                {expandUnstaged ? (
                  <ChevronDown size={12} className="text-gray-500" />
                ) : (
                  <ChevronRight size={12} className="text-gray-500" />
                )}
                <span className="text-xs font-medium text-yellow-400 uppercase tracking-wide">
                  Changes
                </span>
                <Badge variant="warning" size="xs">{unstagedFiles.length}</Badge>
                <div className="flex-1" />
                {unstagedFiles.length > 0 && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation()
                      git.stageAll()
                    }}
                    className="text-xs text-gray-500 hover:text-gray-300 transition-colors"
                    title="Stage all"
                  >
                    <Plus size={12} />
                  </button>
                )}
              </button>

              {expandUnstaged && unstagedFiles.length > 0 && (
                <div className="pb-1">
                  {unstagedFiles.map(file => renderFileItem(file, false))}
                </div>
              )}

              {expandUnstaged && unstagedFiles.length === 0 && (
                <p className="px-3 py-2 text-xs text-gray-600 italic">
                  No changes
                </p>
              )}
            </div>

            {/* No changes at all */}
            {!hasChanges && (
              <div className="flex-1 flex items-center justify-center p-6">
                <div className="text-center">
                  <CheckCheck className="w-8 h-8 mx-auto mb-2 text-green-500/50" />
                  <p className="text-sm text-gray-500">Working tree clean</p>
                  <p className="text-xs text-gray-600 mt-1">No uncommitted changes</p>
                </div>
              </div>
            )}

            {/* Commit section - always visible at bottom */}
            <div className="mt-auto border-t border-gray-700/50 p-3 space-y-2">
              <textarea
                value={git.commitMessage}
                onChange={(e) => git.setCommitMessage(e.target.value)}
                placeholder="Commit message..."
                rows={3}
                className="w-full bg-gray-800 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white placeholder:text-gray-500 focus:border-red-500 focus:outline-none focus:ring-1 focus:ring-red-500/30 resize-none"
                onKeyDown={(e) => {
                  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                    e.preventDefault()
                    handleCommit()
                  }
                }}
              />
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="primary"
                  onClick={handleCommit}
                  disabled={!git.commitMessage.trim() || !hasStagedChanges || git.isCommitting}
                  loading={git.isCommitting}
                  icon={<Check size={14} />}
                  className="flex-1 bg-red-600 hover:bg-red-500 disabled:bg-gray-700"
                >
                  Commit{hasStagedChanges ? ` (${stagedFiles.length})` : ''}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => git.push()}
                  disabled={git.isPushing || (!git.remoteStatus || git.remoteStatus.ahead === 0)}
                  loading={git.isPushing}
                  icon={<Upload size={14} />}
                  title="Push to remote"
                />
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => git.pull()}
                  disabled={git.isPulling}
                  loading={git.isPulling}
                  icon={<Download size={14} />}
                  title="Pull from remote"
                />
              </div>
              <p className="text-xs text-gray-600 text-center">
                Cmd+Enter to commit
              </p>
            </div>
          </div>
        )}

        {activeSection === 'commits' && (
          <div>
            {git.recentCommits.length === 0 ? (
              <div className="p-6 text-center">
                <GitCommitIcon className="w-8 h-8 mx-auto mb-2 text-gray-600" />
                <p className="text-sm text-gray-500">No commits yet</p>
              </div>
            ) : (
              <div className="divide-y divide-gray-800/50">
                {git.recentCommits.map((commit, index) =>
                  renderCommit(commit, index)
                )}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

export default GitPanel
