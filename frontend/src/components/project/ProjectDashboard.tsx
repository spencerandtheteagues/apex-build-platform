// APEX-BUILD Project Dashboard
// Cyberpunk project overview and management interface

import React, { useState, useEffect } from 'react'
import { cn, formatRelativeTime, formatFileSize, getFileIcon } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import { Project, File, Execution, AIUsage } from '@/types'
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardFooter,
  Button,
  Badge,
  Avatar,
  AvatarGroup,
  Loading,
  StatusBadge
} from '@/components/ui'
import {
  Folder,
  Play,
  Code,
  Users,
  Activity,
  Zap,
  FileText,
  Clock,
  HardDrive,
  Settings,
  Share2,
  Download,
  GitBranch,
  Terminal,
  Brain,
  TrendingUp,
  Calendar,
  Star,
  Eye
} from 'lucide-react'

export interface ProjectDashboardProps {
  className?: string
  projectId?: number
  onShare?: () => void
  onSettings?: () => void
  onRunProject?: () => void
  onDownload?: () => void
}

export const ProjectDashboard: React.FC<ProjectDashboardProps> = ({
  className,
  projectId,
  onShare,
  onSettings,
  onRunProject,
  onDownload,
}) => {
  const [recentFiles, setRecentFiles] = useState<File[]>([])
  const [recentExecutions, setRecentExecutions] = useState<Execution[]>([])
  const [aiUsage, setAIUsage] = useState<AIUsage | null>(null)
  const [isLoading, setIsLoading] = useState(false)

  const {
    currentProject,
    files,
    user,
    apiService,
    collaborationUsers,
    setCurrentProject,
    addNotification
  } = useStore()

  const handleShare = async () => {
    if (onShare) {
      onShare()
      return
    }
    if (!currentProject) return
    const url = `${window.location.origin}/project/${currentProject.id}`
    try {
      await navigator.clipboard.writeText(url)
      addNotification({
        type: 'success',
        title: 'Link Copied',
        message: 'Project URL copied to clipboard.',
      })
    } catch {
      addNotification({
        type: 'info',
        title: 'Project URL',
        message: url,
      })
    }
  }

  const handleDownload = async () => {
    if (onDownload) {
      onDownload()
      return
    }
    if (!currentProject) return
    try {
      await apiService.exportProject(currentProject.id, currentProject.name)
      addNotification({
        type: 'success',
        title: 'Download Started',
        message: 'Your ZIP export is downloading.',
      })
    } catch (error) {
      addNotification({
        type: 'error',
        title: 'Download Failed',
        message: 'Unable to export ZIP. Please try again.',
      })
    }
  }

  const handleRunProject = async () => {
    if (onRunProject) {
      onRunProject()
      return
    }
    if (!currentProject) return
    const language = currentProject.language?.toLowerCase() || ''
    if (language === 'javascript' || language === 'typescript') {
      addNotification({
        type: 'info',
        title: 'Run Project',
        message: 'Open the IDE and use Preview to run this project.',
      })
      return
    }
    try {
      const result = await apiService.executeProject({
        project_id: currentProject.id,
      })
      addNotification({
        type: 'success',
        title: 'Run Completed',
        message: result.output ? result.output.slice(0, 200) : 'Project execution completed.',
      })
    } catch {
      addNotification({
        type: 'error',
        title: 'Run Failed',
        message: 'Failed to start the project. Check terminal output.',
      })
    }
  }

  const handleSettings = () => {
    if (onSettings) {
      onSettings()
      return
    }
    addNotification({
      type: 'info',
      title: 'Project Settings',
      message: 'Open the Settings tab in the right panel.',
    })
  }

  // Load project data
  useEffect(() => {
    if (projectId && projectId !== currentProject?.id) {
      loadProject(projectId)
    }
  }, [projectId]) // eslint-disable-line react-hooks/exhaustive-deps -- loadProject is intentionally scoped for project-switch behavior.

  const loadProject = async (id: number) => {
    setIsLoading(true)
    try {
      const project = await apiService.getProject(id)
      setCurrentProject(project)

      // Load additional data in parallel
      const [filesData, executionsData, aiUsageData] = await Promise.all([
        apiService.getFiles(id),
        apiService.getExecutionHistory(id, 10),
        apiService.getAIUsage().catch(() => null)
      ])

      // Sort files by last modified
      const sortedFiles = filesData
        .filter(f => f.type === 'file')
        .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
        .slice(0, 10)

      setRecentFiles(sortedFiles)
      setRecentExecutions(executionsData)
      setAIUsage(aiUsageData)
    } catch (error) {
      console.error('Failed to load project:', error)
    } finally {
      setIsLoading(false)
    }
  }

  // Calculate project stats
  const projectStats = React.useMemo(() => {
    if (!files) return null

    const totalFiles = files.filter(f => f.type === 'file').length
    const totalDirectories = files.filter(f => f.type === 'directory').length
    const totalSize = files.reduce((sum, f) => sum + f.size, 0)
    const languages = new Set(
      files
        .filter(f => f.type === 'file')
        .map(f => f.name.split('.').pop()?.toLowerCase())
        .filter(Boolean)
    )

    return {
      totalFiles,
      totalDirectories,
      totalSize,
      languages: Array.from(languages),
      lastActivity: files.length > 0
        ? Math.max(...files.map(f => new Date(f.updated_at).getTime()))
        : null
    }
  }, [files])

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loading variant="orb" color="cyberpunk" text="Loading project..." />
      </div>
    )
  }

  if (!currentProject) {
    return (
      <div className="flex flex-col items-center justify-center h-64 text-center">
        <Folder className="w-16 h-16 text-gray-600 mb-4" />
        <h3 className="text-lg font-semibold text-gray-300 mb-2">No Project Selected</h3>
        <p className="text-gray-400">Select a project to view its dashboard</p>
      </div>
    )
  }

  return (
    <div className={cn('min-h-full space-y-6 p-4 md:p-6 lg:p-8 text-white', className)}>
      <section className="overflow-hidden rounded-[32px] border border-[rgba(138,223,255,0.16)] bg-[linear-gradient(180deg,rgba(7,15,31,0.96),rgba(4,8,18,0.92))] p-6 shadow-[0_30px_80px_rgba(0,0,0,0.32)]">
        <div className="absolute inset-x-0 top-0 h-28 bg-[radial-gradient(circle_at_top_left,rgba(138,223,255,0.18),transparent_58%)] pointer-events-none" />
        <div className="relative flex flex-col gap-6 xl:flex-row xl:items-start xl:justify-between">
          <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-3">
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-[rgba(138,223,255,0.24)] bg-[rgba(47,168,255,0.12)]">
                <Folder className="h-6 w-6 text-[#8adfff]" />
              </div>
              <div>
                <div className="text-[11px] uppercase tracking-[0.28em] text-[#8adfff]/80">Project Workspace</div>
                <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">{currentProject.name}</h1>
              </div>
              {currentProject.is_public && (
                <Badge variant="outline" className="border-[rgba(138,223,255,0.28)] bg-[rgba(47,168,255,0.12)] text-[#bdeeff]">
                  <Eye size={12} className="mr-1.5" />
                  Public
                </Badge>
              )}
            </div>

            {currentProject.description && (
              <p className="max-w-3xl text-sm leading-7 text-[#9ab0c6] md:text-base">{currentProject.description}</p>
            )}

            <div className="flex flex-wrap gap-2">
              <div className="rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] px-3 py-2 text-sm text-[#c4d6e7]">
                <span className="mr-2 text-[#6f89a4]">Language</span>
                <span className="font-medium text-white">{currentProject.language}</span>
              </div>
              {currentProject.framework && (
                <div className="rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] px-3 py-2 text-sm text-[#c4d6e7]">
                  <span className="mr-2 text-[#6f89a4]">Framework</span>
                  <span className="font-medium text-white">{currentProject.framework}</span>
                </div>
              )}
              <div className="rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] px-3 py-2 text-sm text-[#c4d6e7]">
                <span className="mr-2 text-[#6f89a4]">Created</span>
                <span className="font-medium text-white">{formatRelativeTime(currentProject.created_at)}</span>
              </div>
            </div>
          </div>

          <div className="flex flex-wrap gap-2 xl:max-w-[420px] xl:justify-end">
            <Button size="sm" variant="ghost" icon={<Share2 size={14} />} onClick={handleShare} className="rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] text-[#d8ebff] hover:bg-[rgba(11,20,35,0.92)]">
              Share
            </Button>
            <Button size="sm" variant="ghost" icon={<Settings size={14} />} onClick={handleSettings} className="rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] text-[#d8ebff] hover:bg-[rgba(11,20,35,0.92)]">
              Settings
            </Button>
            <Button size="sm" variant="ghost" icon={<Download size={14} />} onClick={handleDownload} className="rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] text-[#d8ebff] hover:bg-[rgba(11,20,35,0.92)]">
              Download ZIP
            </Button>
            <Button size="sm" variant="primary" icon={<Play size={14} />} onClick={handleRunProject} className="rounded-2xl border border-[rgba(138,223,255,0.45)] bg-[linear-gradient(135deg,rgba(138,223,255,0.22),rgba(47,168,255,0.26))] text-white hover:bg-[linear-gradient(135deg,rgba(138,223,255,0.28),rgba(47,168,255,0.32))]">
              Run Project
            </Button>
          </div>
        </div>

        {projectStats && (
          <>
            <div className="mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <div className="rounded-[24px] border border-[rgba(138,223,255,0.12)] bg-[rgba(7,13,24,0.74)] p-4">
                <FileText className="mb-3 h-5 w-5 text-[#8adfff]" />
                <div className="text-2xl font-semibold text-white">{projectStats.totalFiles}</div>
                <div className="mt-1 text-xs uppercase tracking-[0.2em] text-[#718aa3]">Files</div>
              </div>
              <div className="rounded-[24px] border border-[rgba(138,223,255,0.12)] bg-[rgba(7,13,24,0.74)] p-4">
                <Folder className="mb-3 h-5 w-5 text-[#9ecfff]" />
                <div className="text-2xl font-semibold text-white">{projectStats.totalDirectories}</div>
                <div className="mt-1 text-xs uppercase tracking-[0.2em] text-[#718aa3]">Folders</div>
              </div>
              <div className="rounded-[24px] border border-[rgba(138,223,255,0.12)] bg-[rgba(7,13,24,0.74)] p-4">
                <HardDrive className="mb-3 h-5 w-5 text-[#b8e6ff]" />
                <div className="text-2xl font-semibold text-white">{formatFileSize(projectStats.totalSize)}</div>
                <div className="mt-1 text-xs uppercase tracking-[0.2em] text-[#718aa3]">Size</div>
              </div>
              <div className="rounded-[24px] border border-[rgba(138,223,255,0.12)] bg-[rgba(7,13,24,0.74)] p-4">
                <Activity className="mb-3 h-5 w-5 text-[#8adfff]" />
                <div className="text-2xl font-semibold text-white">
                  {projectStats.lastActivity ? formatRelativeTime(new Date(projectStats.lastActivity).toISOString()) : 'Never'}
                </div>
                <div className="mt-1 text-xs uppercase tracking-[0.2em] text-[#718aa3]">Last Activity</div>
              </div>
            </div>

            {projectStats.languages.length > 0 && (
              <div className="mt-5 flex flex-wrap items-center gap-2">
                <span className="text-xs uppercase tracking-[0.24em] text-[#6d86a0]">Detected</span>
                {projectStats.languages.map((lang, idx) => (
                  <Badge
                    key={lang || idx}
                    variant="outline"
                    size="xs"
                    className="border-[#17314d] bg-[rgba(7,13,24,0.82)] px-2.5 py-1 text-[#d7e9fb]"
                  >
                    {(lang || 'unknown').toUpperCase()}
                  </Badge>
                ))}
              </div>
            )}
          </>
        )}
      </section>

      <div className="grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
        <div className="space-y-6">
          <section className="rounded-[28px] border border-[rgba(138,223,255,0.14)] bg-[rgba(5,10,18,0.82)] p-5 shadow-[0_20px_60px_rgba(0,0,0,0.28)]">
            <div className="mb-4 flex items-center gap-2">
              <Clock className="h-5 w-5 text-[#8adfff]" />
              <h2 className="text-lg font-semibold text-white">Recent Files</h2>
            </div>
            {recentFiles.length > 0 ? (
              <div className="space-y-2">
                {recentFiles.map(file => (
                  <div
                    key={file.id}
                    className="group flex items-center gap-3 rounded-2xl border border-transparent bg-[rgba(7,13,24,0.72)] p-3 transition hover:border-[rgba(138,223,255,0.14)] hover:bg-[rgba(10,18,31,0.92)]"
                  >
                    <div className="text-lg">{getFileIcon(file.name)}</div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center justify-between gap-3">
                        <span className="truncate text-sm font-medium text-[#eef8ff]">{file.name}</span>
                        <span className="text-xs text-[#6c87a3]">{formatFileSize(file.size)}</span>
                      </div>
                      <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-[#88a0b8]">
                        <span className="truncate">{file.path}</span>
                        {file.last_editor && (
                          <>
                            <span>•</span>
                            <span>by {file.last_editor.username}</span>
                          </>
                        )}
                        <span>•</span>
                        <span>{formatRelativeTime(file.updated_at)}</span>
                      </div>
                    </div>
                    {file.is_locked && (
                      <Badge variant="outline" size="xs" className="border-[rgba(255,184,107,0.3)] bg-[rgba(255,184,107,0.08)] text-[#ffd3a2]">
                        Locked
                      </Badge>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-[24px] border border-dashed border-[#17314d] bg-[rgba(7,13,24,0.62)] px-4 py-10 text-center text-[#7f95ad]">
                <FileText className="mx-auto mb-3 h-8 w-8 opacity-70" />
                <p className="text-sm">No files yet</p>
              </div>
            )}
          </section>

          <section className="rounded-[28px] border border-[rgba(138,223,255,0.14)] bg-[rgba(5,10,18,0.82)] p-5 shadow-[0_20px_60px_rgba(0,0,0,0.28)]">
            <div className="mb-4 flex items-center gap-2">
              <Terminal className="h-5 w-5 text-[#9ce8c2]" />
              <h2 className="text-lg font-semibold text-white">Recent Runs</h2>
            </div>
            {recentExecutions.length > 0 ? (
              <div className="space-y-2">
                {recentExecutions.map(execution => (
                  <div
                    key={execution.id}
                    className="flex items-center gap-3 rounded-2xl border border-transparent bg-[rgba(7,13,24,0.72)] p-3 transition hover:border-[rgba(138,223,255,0.14)] hover:bg-[rgba(10,18,31,0.92)]"
                  >
                    <StatusBadge
                      status={execution.status === 'completed' ? 'online' :
                             execution.status === 'failed' ? 'busy' :
                             execution.status === 'running' ? 'away' : 'offline'}
                    />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center justify-between gap-3">
                        <span className="truncate text-sm font-medium text-[#eef8ff]">{execution.command}</span>
                        <span className="text-xs text-[#6c87a3]">{execution.duration}ms</span>
                      </div>
                      <div className="mt-1 text-xs text-[#88a0b8]">
                        {formatRelativeTime(execution.started_at)}
                        {execution.exit_code !== 0 && (
                          <span className="ml-2 text-[#ff9494]">Exit code: {execution.exit_code}</span>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-[24px] border border-dashed border-[#17314d] bg-[rgba(7,13,24,0.62)] px-4 py-10 text-center text-[#7f95ad]">
                <Terminal className="mx-auto mb-3 h-8 w-8 opacity-70" />
                <p className="text-sm">No executions yet</p>
              </div>
            )}
          </section>
        </div>

        <div className="space-y-6">
          {aiUsage && (
            <section className="rounded-[28px] border border-[rgba(138,223,255,0.14)] bg-[rgba(5,10,18,0.82)] p-5 shadow-[0_20px_60px_rgba(0,0,0,0.28)]">
              <div className="mb-4 flex items-center gap-2">
                <Brain className="h-5 w-5 text-[#8adfff]" />
                <h2 className="text-lg font-semibold text-white">AI Usage</h2>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="rounded-[22px] border border-[#17314d] bg-[rgba(7,13,24,0.72)] p-4 text-center">
                  <div className="text-2xl font-semibold text-white">{aiUsage.total_requests}</div>
                  <div className="mt-1 text-xs uppercase tracking-[0.2em] text-[#6f89a4]">Requests</div>
                </div>
                <div className="rounded-[22px] border border-[#17314d] bg-[rgba(7,13,24,0.72)] p-4 text-center">
                  <div className="text-2xl font-semibold text-white">${aiUsage.total_cost.toFixed(3)}</div>
                  <div className="mt-1 text-xs uppercase tracking-[0.2em] text-[#6f89a4]">Cost</div>
                </div>
              </div>
              <div className="mt-4 space-y-2">
                {Object.entries(aiUsage.by_provider).map(([provider, usage]) => (
                  <div key={provider} className="flex items-center justify-between rounded-2xl border border-transparent bg-[rgba(7,13,24,0.72)] px-3 py-2 text-sm">
                    <span className="capitalize text-[#d8ebff]">{provider}</span>
                    <div className="flex items-center gap-2 text-xs text-[#88a0b8]">
                      <span>{usage.requests} req</span>
                      <span className="text-[#bdeeff]">${usage.cost.toFixed(3)}</span>
                    </div>
                  </div>
                ))}
              </div>
            </section>
          )}

          <section className="rounded-[28px] border border-[rgba(138,223,255,0.14)] bg-[rgba(5,10,18,0.82)] p-5 shadow-[0_20px_60px_rgba(0,0,0,0.28)]">
            <div className="mb-4 flex items-center gap-2">
              <Users className="h-5 w-5 text-[#8adfff]" />
              <h2 className="text-lg font-semibold text-white">Collaboration</h2>
            </div>
            {collaborationUsers.length > 0 ? (
              <div className="space-y-4">
                <AvatarGroup max={5} size="md">
                  {collaborationUsers.map(user => (
                    <Avatar
                      key={user.id}
                      src={user.avatar_url}
                      fallback={user.username}
                      status="online"
                      showStatus
                    />
                  ))}
                </AvatarGroup>
                <div className="text-sm text-[#91a6bc]">{collaborationUsers.length} collaborator(s) currently active</div>
                <Button size="sm" variant="ghost" icon={<Share2 size={14} />} className="w-full rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] text-[#d8ebff] hover:bg-[rgba(11,20,35,0.92)]">
                  Invite Collaborators
                </Button>
              </div>
            ) : (
              <div className="rounded-[24px] border border-dashed border-[#17314d] bg-[rgba(7,13,24,0.62)] px-4 py-10 text-center text-[#7f95ad]">
                <Users className="mx-auto mb-3 h-8 w-8 opacity-70" />
                <p className="mb-3 text-sm">No active collaborators</p>
                <Button size="sm" variant="ghost" icon={<Share2 size={14} />} className="rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] text-[#d8ebff] hover:bg-[rgba(11,20,35,0.92)]">
                  Invite People
                </Button>
              </div>
            )}
          </section>
        </div>
      </div>
    </div>
  )
}

export default ProjectDashboard
