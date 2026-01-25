import React, { useState, useEffect, useCallback } from 'react'
import {
  GitBranch,
  GitCommit,
  GitPullRequest,
  GitMerge,
  Plus,
  RefreshCw,
  Check,
  X,
  ChevronDown,
  ChevronRight,
  Upload,
  Download,
  Link,
  Unlink,
  FileText,
  FilePlus,
  FileMinus,
  FileEdit,
  AlertCircle,
  Clock,
  User,
  ExternalLink,
  Loader2
} from 'lucide-react'
import api from '../../services/api'

interface Repository {
  id: number
  project_id: number
  remote_url: string
  provider: string
  repo_owner: string
  repo_name: string
  branch: string
  last_sync: string
  is_connected: boolean
}

interface Branch {
  name: string
  sha: string
  is_default: boolean
  protected: boolean
}

interface Commit {
  sha: string
  message: string
  author: string
  email: string
  timestamp: string
}

interface FileChange {
  path: string
  status: 'added' | 'modified' | 'deleted' | 'renamed'
  staged: boolean
}

interface PullRequest {
  number: number
  title: string
  body: string
  state: string
  author: string
  branch: string
  base_branch: string
  created_at: string
  url: string
}

interface GitPanelProps {
  projectId: number
  className?: string
}

const statusIcons: Record<string, React.ReactNode> = {
  added: <FilePlus className="w-4 h-4 text-green-400" />,
  modified: <FileEdit className="w-4 h-4 text-yellow-400" />,
  deleted: <FileMinus className="w-4 h-4 text-red-400" />,
  renamed: <FileText className="w-4 h-4 text-blue-400" />
}

export default function GitPanel({ projectId, className = '' }: GitPanelProps) {
  const [repo, setRepo] = useState<Repository | null>(null)
  const [branches, setBranches] = useState<Branch[]>([])
  const [commits, setCommits] = useState<Commit[]>([])
  const [changes, setChanges] = useState<{ staged: FileChange[]; unstaged: FileChange[] }>({ staged: [], unstaged: [] })
  const [pullRequests, setPullRequests] = useState<PullRequest[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<'changes' | 'commits' | 'branches' | 'prs'>('changes')

  // Connect dialog state
  const [showConnect, setShowConnect] = useState(false)
  const [connectUrl, setConnectUrl] = useState('')
  const [connectToken, setConnectToken] = useState('')

  // Commit dialog state
  const [showCommit, setShowCommit] = useState(false)
  const [commitMessage, setCommitMessage] = useState('')
  const [selectedFiles, setSelectedFiles] = useState<Set<string>>(new Set())

  // Branch dialog state
  const [showNewBranch, setShowNewBranch] = useState(false)
  const [newBranchName, setNewBranchName] = useState('')

  // PR dialog state
  const [showNewPR, setShowNewPR] = useState(false)
  const [prTitle, setPrTitle] = useState('')
  const [prBody, setPrBody] = useState('')

  // Fetch repository info
  const fetchRepo = useCallback(async () => {
    try {
      const response = await api.get(`/git/repo/${projectId}`)
      setRepo(response.data.repository)
    } catch (err) {
      setRepo(null)
    }
  }, [projectId])

  // Fetch all data
  const fetchAllData = useCallback(async () => {
    if (!repo) return

    setLoading(true)
    try {
      const [branchesRes, commitsRes, statusRes, prsRes] = await Promise.all([
        api.get(`/git/branches/${projectId}`),
        api.get(`/git/commits/${projectId}`),
        api.get(`/git/status/${projectId}`),
        api.get(`/git/pulls/${projectId}`)
      ])

      setBranches(branchesRes.data.branches || [])
      setCommits(commitsRes.data.commits || [])
      setChanges({
        staged: statusRes.data.staged || [],
        unstaged: statusRes.data.unstaged || []
      })
      setPullRequests(prsRes.data.pull_requests || [])
      setError(null)
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to fetch git data')
    } finally {
      setLoading(false)
    }
  }, [projectId, repo])

  useEffect(() => {
    fetchRepo()
  }, [fetchRepo])

  useEffect(() => {
    if (repo?.is_connected) {
      fetchAllData()
    } else {
      setLoading(false)
    }
  }, [repo, fetchAllData])

  // Connect repository
  const handleConnect = async () => {
    try {
      setLoading(true)
      await api.post('/git/connect', {
        project_id: projectId,
        remote_url: connectUrl,
        token: connectToken
      })
      setShowConnect(false)
      setConnectUrl('')
      setConnectToken('')
      fetchRepo()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to connect repository')
    } finally {
      setLoading(false)
    }
  }

  // Disconnect repository
  const handleDisconnect = async () => {
    if (!confirm('Disconnect this repository? Your files will remain, but git history will be lost.')) return

    try {
      await api.delete(`/git/repo/${projectId}`)
      setRepo(null)
      setBranches([])
      setCommits([])
      setChanges({ staged: [], unstaged: [] })
      setPullRequests([])
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to disconnect')
    }
  }

  // Commit changes
  const handleCommit = async () => {
    if (!commitMessage.trim()) {
      setError('Commit message is required')
      return
    }

    const filesToCommit = Array.from(selectedFiles)
    if (filesToCommit.length === 0) {
      setError('Select files to commit')
      return
    }

    try {
      setLoading(true)
      await api.post('/git/commit', {
        project_id: projectId,
        message: commitMessage,
        files: filesToCommit
      })
      setShowCommit(false)
      setCommitMessage('')
      setSelectedFiles(new Set())
      fetchAllData()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Commit failed')
    } finally {
      setLoading(false)
    }
  }

  // Pull changes
  const handlePull = async () => {
    try {
      setLoading(true)
      await api.post('/git/pull', { project_id: projectId })
      fetchAllData()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Pull failed')
    } finally {
      setLoading(false)
    }
  }

  // Push changes
  const handlePush = async () => {
    try {
      setLoading(true)
      await api.post('/git/push', { project_id: projectId })
      fetchAllData()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Push failed')
    } finally {
      setLoading(false)
    }
  }

  // Create branch
  const handleCreateBranch = async () => {
    if (!newBranchName.trim()) return

    try {
      setLoading(true)
      await api.post('/git/branch', {
        project_id: projectId,
        branch_name: newBranchName,
        base_branch: repo?.branch || 'main'
      })
      setShowNewBranch(false)
      setNewBranchName('')
      fetchAllData()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to create branch')
    } finally {
      setLoading(false)
    }
  }

  // Switch branch
  const handleSwitchBranch = async (branchName: string) => {
    try {
      setLoading(true)
      await api.post('/git/checkout', {
        project_id: projectId,
        branch_name: branchName
      })
      fetchRepo()
      fetchAllData()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to switch branch')
    } finally {
      setLoading(false)
    }
  }

  // Create pull request
  const handleCreatePR = async () => {
    if (!prTitle.trim()) return

    try {
      setLoading(true)
      await api.post('/git/pulls', {
        project_id: projectId,
        title: prTitle,
        body: prBody,
        head: repo?.branch,
        base: 'main'
      })
      setShowNewPR(false)
      setPrTitle('')
      setPrBody('')
      fetchAllData()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to create PR')
    } finally {
      setLoading(false)
    }
  }

  // Toggle file selection
  const toggleFileSelection = (path: string) => {
    setSelectedFiles(prev => {
      const next = new Set(prev)
      if (next.has(path)) {
        next.delete(path)
      } else {
        next.add(path)
      }
      return next
    })
  }

  // Select all files
  const selectAllFiles = () => {
    const allPaths = [...changes.staged, ...changes.unstaged].map(f => f.path)
    setSelectedFiles(new Set(allPaths))
  }

  if (!repo?.is_connected) {
    return (
      <div className={`flex flex-col bg-gray-900/50 rounded-lg border border-gray-700 p-6 ${className}`}>
        <div className="text-center">
          <GitBranch className="w-12 h-12 mx-auto mb-4 text-gray-600" />
          <h3 className="text-lg font-medium text-white mb-2">Connect to Git</h3>
          <p className="text-sm text-gray-500 mb-4">
            Connect to a GitHub repository to enable version control
          </p>
          <button
            onClick={() => setShowConnect(true)}
            className="flex items-center gap-2 mx-auto px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg"
          >
            <Link className="w-4 h-4" />
            Connect Repository
          </button>
        </div>

        {/* Connect Dialog */}
        {showConnect && (
          <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-gray-800 border border-gray-700 rounded-lg p-6 w-full max-w-md">
              <h3 className="text-lg font-medium text-white mb-4">Connect Repository</h3>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Repository URL</label>
                  <input
                    type="text"
                    value={connectUrl}
                    onChange={(e) => setConnectUrl(e.target.value)}
                    placeholder="https://github.com/user/repo"
                    className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-cyan-500 focus:outline-none"
                  />
                </div>
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Personal Access Token</label>
                  <input
                    type="password"
                    value={connectToken}
                    onChange={(e) => setConnectToken(e.target.value)}
                    placeholder="ghp_xxxxxxxxxxxx"
                    className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-cyan-500 focus:outline-none"
                  />
                  <p className="text-xs text-gray-500 mt-1">
                    Create a token at GitHub Settings &gt; Developer settings &gt; Personal access tokens
                  </p>
                </div>
              </div>
              <div className="flex justify-end gap-2 mt-6">
                <button
                  onClick={() => setShowConnect(false)}
                  className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-md"
                >
                  Cancel
                </button>
                <button
                  onClick={handleConnect}
                  disabled={!connectUrl || loading}
                  className="flex items-center gap-2 px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-md disabled:opacity-50"
                >
                  {loading && <Loader2 className="w-4 h-4 animate-spin" />}
                  Connect
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    )
  }

  return (
    <div className={`flex flex-col bg-gray-900/50 rounded-lg border border-gray-700 ${className}`}>
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700">
        <div className="flex items-center gap-3">
          <GitBranch className="w-5 h-5 text-cyan-400" />
          <div>
            <div className="text-sm text-white font-medium">{repo.repo_owner}/{repo.repo_name}</div>
            <div className="text-xs text-gray-500 flex items-center gap-2">
              <GitBranch className="w-3 h-3" />
              {repo.branch}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={handlePull}
            disabled={loading}
            className="p-2 hover:bg-gray-700 rounded-md text-gray-400 hover:text-white"
            title="Pull"
          >
            <Download className="w-4 h-4" />
          </button>
          <button
            onClick={handlePush}
            disabled={loading}
            className="p-2 hover:bg-gray-700 rounded-md text-gray-400 hover:text-white"
            title="Push"
          >
            <Upload className="w-4 h-4" />
          </button>
          <button
            onClick={fetchAllData}
            disabled={loading}
            className="p-2 hover:bg-gray-700 rounded-md text-gray-400 hover:text-white"
            title="Refresh"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={handleDisconnect}
            className="p-2 hover:bg-gray-700 rounded-md text-gray-400 hover:text-red-400"
            title="Disconnect"
          >
            <Unlink className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Error display */}
      {error && (
        <div className="mx-3 mt-3 p-2 bg-red-500/20 border border-red-500/50 rounded flex items-center gap-2 text-red-400 text-sm">
          <AlertCircle className="w-4 h-4" />
          {error}
          <button onClick={() => setError(null)} className="ml-auto">&times;</button>
        </div>
      )}

      {/* Tabs */}
      <div className="flex items-center gap-1 px-3 pt-3">
        {(['changes', 'commits', 'branches', 'prs'] as const).map(tab => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-3 py-1.5 text-sm rounded-md transition-colors ${
              activeTab === tab
                ? 'bg-cyan-600 text-white'
                : 'text-gray-400 hover:text-white hover:bg-gray-700'
            }`}
          >
            {tab === 'changes' && 'Changes'}
            {tab === 'commits' && 'History'}
            {tab === 'branches' && 'Branches'}
            {tab === 'prs' && 'PRs'}
            {tab === 'changes' && (changes.staged.length + changes.unstaged.length) > 0 && (
              <span className="ml-1.5 px-1.5 py-0.5 bg-yellow-500/30 text-yellow-400 rounded text-xs">
                {changes.staged.length + changes.unstaged.length}
              </span>
            )}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <div className="flex-1 overflow-y-auto p-3">
        {/* Changes Tab */}
        {activeTab === 'changes' && (
          <div className="space-y-4">
            {/* Commit button */}
            <button
              onClick={() => setShowCommit(true)}
              disabled={changes.staged.length + changes.unstaged.length === 0}
              className="w-full flex items-center justify-center gap-2 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-md disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <GitCommit className="w-4 h-4" />
              Commit Changes
            </button>

            {/* Staged changes */}
            {changes.staged.length > 0 && (
              <div>
                <div className="text-xs text-gray-400 mb-2 flex items-center gap-1">
                  <Check className="w-3 h-3 text-green-400" />
                  Staged ({changes.staged.length})
                </div>
                {changes.staged.map(file => (
                  <div key={file.path} className="flex items-center gap-2 py-1 px-2 hover:bg-gray-800 rounded text-sm">
                    {statusIcons[file.status]}
                    <span className="text-gray-300 truncate">{file.path}</span>
                  </div>
                ))}
              </div>
            )}

            {/* Unstaged changes */}
            {changes.unstaged.length > 0 && (
              <div>
                <div className="text-xs text-gray-400 mb-2">
                  Changes ({changes.unstaged.length})
                </div>
                {changes.unstaged.map(file => (
                  <div key={file.path} className="flex items-center gap-2 py-1 px-2 hover:bg-gray-800 rounded text-sm">
                    {statusIcons[file.status]}
                    <span className="text-gray-300 truncate">{file.path}</span>
                  </div>
                ))}
              </div>
            )}

            {changes.staged.length === 0 && changes.unstaged.length === 0 && (
              <div className="text-center py-8 text-gray-500">
                <Check className="w-8 h-8 mx-auto mb-2 opacity-50" />
                <p>No changes</p>
              </div>
            )}
          </div>
        )}

        {/* Commits Tab */}
        {activeTab === 'commits' && (
          <div className="space-y-2">
            {commits.map(commit => (
              <div key={commit.sha} className="p-2 hover:bg-gray-800 rounded">
                <div className="flex items-start gap-2">
                  <GitCommit className="w-4 h-4 text-cyan-400 mt-0.5" />
                  <div className="flex-1 min-w-0">
                    <div className="text-sm text-white truncate">{commit.message}</div>
                    <div className="text-xs text-gray-500 flex items-center gap-2 mt-1">
                      <span className="flex items-center gap-1">
                        <User className="w-3 h-3" />
                        {commit.author}
                      </span>
                      <span className="flex items-center gap-1">
                        <Clock className="w-3 h-3" />
                        {new Date(commit.timestamp).toLocaleDateString()}
                      </span>
                      <span className="font-mono">{commit.sha.slice(0, 7)}</span>
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Branches Tab */}
        {activeTab === 'branches' && (
          <div className="space-y-2">
            <button
              onClick={() => setShowNewBranch(true)}
              className="w-full flex items-center justify-center gap-2 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-md text-sm"
            >
              <Plus className="w-4 h-4" />
              New Branch
            </button>
            {branches.map(branch => (
              <button
                key={branch.name}
                onClick={() => handleSwitchBranch(branch.name)}
                className={`w-full flex items-center gap-2 p-2 rounded text-left ${
                  branch.name === repo.branch ? 'bg-cyan-600/20 border border-cyan-500/50' : 'hover:bg-gray-800'
                }`}
              >
                <GitBranch className={`w-4 h-4 ${branch.name === repo.branch ? 'text-cyan-400' : 'text-gray-400'}`} />
                <span className={`flex-1 text-sm ${branch.name === repo.branch ? 'text-white' : 'text-gray-300'}`}>
                  {branch.name}
                </span>
                {branch.is_default && (
                  <span className="text-xs px-1.5 py-0.5 bg-gray-700 rounded text-gray-400">default</span>
                )}
                {branch.protected && (
                  <span className="text-xs px-1.5 py-0.5 bg-yellow-500/20 rounded text-yellow-400">protected</span>
                )}
              </button>
            ))}
          </div>
        )}

        {/* Pull Requests Tab */}
        {activeTab === 'prs' && (
          <div className="space-y-2">
            <button
              onClick={() => setShowNewPR(true)}
              className="w-full flex items-center justify-center gap-2 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-md text-sm"
            >
              <Plus className="w-4 h-4" />
              New Pull Request
            </button>
            {pullRequests.map(pr => (
              <a
                key={pr.number}
                href={pr.url}
                target="_blank"
                rel="noopener noreferrer"
                className="block p-2 hover:bg-gray-800 rounded"
              >
                <div className="flex items-start gap-2">
                  <GitPullRequest className={`w-4 h-4 mt-0.5 ${pr.state === 'open' ? 'text-green-400' : 'text-purple-400'}`} />
                  <div className="flex-1 min-w-0">
                    <div className="text-sm text-white flex items-center gap-2">
                      <span className="truncate">{pr.title}</span>
                      <ExternalLink className="w-3 h-3 text-gray-500" />
                    </div>
                    <div className="text-xs text-gray-500 flex items-center gap-2 mt-1">
                      <span>#{pr.number}</span>
                      <span>{pr.branch} → {pr.base_branch}</span>
                      <span>by {pr.author}</span>
                    </div>
                  </div>
                </div>
              </a>
            ))}
            {pullRequests.length === 0 && (
              <div className="text-center py-8 text-gray-500">
                <GitPullRequest className="w-8 h-8 mx-auto mb-2 opacity-50" />
                <p>No pull requests</p>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Commit Dialog */}
      {showCommit && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-gray-800 border border-gray-700 rounded-lg p-6 w-full max-w-lg">
            <h3 className="text-lg font-medium text-white mb-4">Commit Changes</h3>

            <div className="space-y-4">
              <div>
                <label className="block text-sm text-gray-400 mb-1">Commit Message</label>
                <textarea
                  value={commitMessage}
                  onChange={(e) => setCommitMessage(e.target.value)}
                  placeholder="Describe your changes..."
                  rows={3}
                  className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white resize-none focus:border-cyan-500 focus:outline-none"
                />
              </div>

              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm text-gray-400">Files to commit</label>
                  <button onClick={selectAllFiles} className="text-xs text-cyan-400 hover:text-cyan-300">
                    Select all
                  </button>
                </div>
                <div className="max-h-48 overflow-y-auto space-y-1">
                  {[...changes.staged, ...changes.unstaged].map(file => (
                    <label
                      key={file.path}
                      className="flex items-center gap-2 py-1 px-2 hover:bg-gray-700 rounded cursor-pointer"
                    >
                      <input
                        type="checkbox"
                        checked={selectedFiles.has(file.path)}
                        onChange={() => toggleFileSelection(file.path)}
                        className="rounded bg-gray-700 border-gray-600 text-cyan-500"
                      />
                      {statusIcons[file.status]}
                      <span className="text-sm text-gray-300 truncate">{file.path}</span>
                    </label>
                  ))}
                </div>
              </div>
            </div>

            <div className="flex justify-end gap-2 mt-6">
              <button
                onClick={() => setShowCommit(false)}
                className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-md"
              >
                Cancel
              </button>
              <button
                onClick={handleCommit}
                disabled={!commitMessage.trim() || selectedFiles.size === 0 || loading}
                className="flex items-center gap-2 px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-md disabled:opacity-50"
              >
                {loading && <Loader2 className="w-4 h-4 animate-spin" />}
                Commit & Push
              </button>
            </div>
          </div>
        </div>
      )}

      {/* New Branch Dialog */}
      {showNewBranch && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-gray-800 border border-gray-700 rounded-lg p-6 w-full max-w-md">
            <h3 className="text-lg font-medium text-white mb-4">Create Branch</h3>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Branch Name</label>
              <input
                type="text"
                value={newBranchName}
                onChange={(e) => setNewBranchName(e.target.value)}
                placeholder="feature/my-feature"
                className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-cyan-500 focus:outline-none"
              />
              <p className="text-xs text-gray-500 mt-1">
                Will branch from: {repo.branch}
              </p>
            </div>
            <div className="flex justify-end gap-2 mt-6">
              <button
                onClick={() => setShowNewBranch(false)}
                className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-md"
              >
                Cancel
              </button>
              <button
                onClick={handleCreateBranch}
                disabled={!newBranchName.trim() || loading}
                className="flex items-center gap-2 px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-md disabled:opacity-50"
              >
                {loading && <Loader2 className="w-4 h-4 animate-spin" />}
                Create
              </button>
            </div>
          </div>
        </div>
      )}

      {/* New PR Dialog */}
      {showNewPR && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-gray-800 border border-gray-700 rounded-lg p-6 w-full max-w-lg">
            <h3 className="text-lg font-medium text-white mb-4">Create Pull Request</h3>
            <div className="space-y-4">
              <div className="text-sm text-gray-400">
                {repo.branch} → main
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">Title</label>
                <input
                  type="text"
                  value={prTitle}
                  onChange={(e) => setPrTitle(e.target.value)}
                  placeholder="Pull request title"
                  className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-cyan-500 focus:outline-none"
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">Description</label>
                <textarea
                  value={prBody}
                  onChange={(e) => setPrBody(e.target.value)}
                  placeholder="Describe your changes..."
                  rows={4}
                  className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white resize-none focus:border-cyan-500 focus:outline-none"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-6">
              <button
                onClick={() => setShowNewPR(false)}
                className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-md"
              >
                Cancel
              </button>
              <button
                onClick={handleCreatePR}
                disabled={!prTitle.trim() || loading}
                className="flex items-center gap-2 px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-md disabled:opacity-50"
              >
                {loading && <Loader2 className="w-4 h-4 animate-spin" />}
                Create PR
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
