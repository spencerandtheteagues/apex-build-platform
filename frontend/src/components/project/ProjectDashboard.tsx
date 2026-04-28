import React, { useEffect, useMemo, useState } from 'react'
import { cn, formatFileSize, formatRelativeTime, getFileIcon } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import { AIUsage, Execution, File } from '@/types'
import { Badge, Button, Loading } from '@/components/ui'
import {
  Activity,
  ArrowRight,
  Brain,
  Calendar,
  Clock3,
  Code2,
  Download,
  Eye,
  FileText,
  Folder,
  HardDrive,
  Play,
  Settings,
  Share2,
  Sparkles,
  Terminal,
  Users,
} from 'lucide-react'

export interface ProjectDashboardProps {
  className?: string
  projectId?: number
  onShare?: () => void
  onSettings?: () => void
  onRunProject?: () => void
  onDownload?: () => void
}

const panelClass =
  'rounded-[28px] border border-white/8 bg-[linear-gradient(180deg,rgba(18,23,34,0.92)_0%,rgba(8,11,18,0.96)_100%)] shadow-[0_24px_80px_rgba(0,0,0,0.28)]'

const statCardClass =
  'rounded-[24px] border border-white/8 bg-black/24 px-5 py-5 shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]'

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
    collaborationUsers,
    apiService,
    setCurrentProject,
    addNotification,
  } = useStore()

  useEffect(() => {
    if (projectId && projectId !== currentProject?.id) {
      void loadProject(projectId)
    }
  }, [projectId]) // eslint-disable-line react-hooks/exhaustive-deps

  const loadProject = async (id: number) => {
    setIsLoading(true)
    try {
      const project = await apiService.getProject(id)
      setCurrentProject(project)

      const [filesData, executionsData, aiUsageData] = await Promise.all([
        apiService.getFiles(id),
        apiService.getExecutionHistory(id, 10),
        apiService.getAIUsage().catch(() => null),
      ])

      const sortedFiles = filesData
        .filter((file) => file.type === 'file')
        .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
        .slice(0, 8)

      setRecentFiles(sortedFiles)
      setRecentExecutions(executionsData.slice(0, 6))
      setAIUsage(aiUsageData)
    } catch (error) {
      console.error('Failed to load project:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const projectStats = useMemo(() => {
    if (!files) return null

    const totalFiles = files.filter((file) => file.type === 'file').length
    const totalDirectories = files.filter((file) => file.type === 'directory').length
    const totalSize = files.reduce((sum, file) => sum + file.size, 0)
    const languages = Array.from(
      new Set(
        files
          .filter((file) => file.type === 'file')
          .map((file) => file.name.split('.').pop()?.toLowerCase())
          .filter(Boolean),
      ),
    ).slice(0, 6)

    return {
      totalFiles,
      totalDirectories,
      totalSize,
      languages,
      lastActivity:
        files.length > 0 ? Math.max(...files.map((file) => new Date(file.updated_at).getTime())) : null,
    }
  }, [files])

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
    } catch {
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

  if (isLoading) {
    return (
      <div className="flex h-72 items-center justify-center">
        <Loading variant="orb" color="cyberpunk" text="Loading workspace..." />
      </div>
    )
  }

  if (!currentProject) {
    return (
      <div className="flex h-72 flex-col items-center justify-center rounded-[28px] border border-dashed border-white/10 bg-black/20 text-center">
        <Folder className="mb-4 h-14 w-14 text-white/25" />
        <h3 className="text-lg font-semibold text-white">No project selected</h3>
        <p className="mt-2 text-sm text-gray-400">Choose a project to load the workspace dashboard.</p>
      </div>
    )
  }

  const languageLabel = currentProject.language || 'workspace'
  const frameworkLabel = currentProject.framework || 'custom'

  return (
    <div className={cn('mx-auto flex w-full max-w-[1500px] flex-col gap-6 p-4 md:p-6 xl:p-8', className)}>
      <section className={cn(panelClass, 'overflow-hidden')}>
        <div className="grid gap-0 xl:grid-cols-[minmax(0,1.6fr)_360px]">
          <div className="relative overflow-hidden px-6 py-6 md:px-8 md:py-8">
            <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(120,180,255,0.14),transparent_36%),radial-gradient(circle_at_bottom_right,rgba(255,94,58,0.10),transparent_30%)]" />
            <div className="relative">
              <div className="mb-5 flex flex-wrap items-center gap-3">
                <div className="flex h-16 w-16 items-center justify-center rounded-[22px] border border-cyan-400/18 bg-cyan-400/10 text-cyan-200 shadow-[0_0_0_1px_rgba(103,232,249,0.06)]">
                  <Folder className="h-8 w-8" />
                </div>
                <div className="min-w-0">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.35em] text-cyan-200/75">
                    Project Workspace
                  </div>
                  <h1 className="mt-2 max-w-4xl text-balance text-4xl font-semibold leading-[0.95] tracking-[-0.04em] text-white md:text-6xl">
                    {currentProject.name}
                  </h1>
                </div>
              </div>

              {currentProject.description ? (
                <p className="max-w-3xl text-base leading-8 text-gray-300 md:text-lg">
                  {currentProject.description}
                </p>
              ) : (
                <p className="max-w-3xl text-base leading-8 text-gray-400 md:text-lg">
                  {currentProject.name}
                </p>
              )}

              <div className="mt-6 flex flex-wrap items-center gap-3">
                <Badge variant="outline" className="border-cyan-400/25 bg-cyan-400/8 px-4 py-2 text-sm text-cyan-100">
                  <Code2 className="mr-2 h-4 w-4" />
                  {languageLabel}
                </Badge>
                <Badge variant="outline" className="border-white/10 bg-white/[0.03] px-4 py-2 text-sm text-gray-200">
                  <Sparkles className="mr-2 h-4 w-4" />
                  {frameworkLabel}
                </Badge>
                <Badge variant="outline" className="border-white/10 bg-white/[0.03] px-4 py-2 text-sm text-gray-200">
                  <Calendar className="mr-2 h-4 w-4" />
                  Created {formatRelativeTime(currentProject.created_at)}
                </Badge>
                {currentProject.is_public && (
                  <Badge variant="outline" className="border-emerald-400/25 bg-emerald-400/8 px-4 py-2 text-sm text-emerald-100">
                    <Eye className="mr-2 h-4 w-4" />
                    Public
                  </Badge>
                )}
              </div>

              {projectStats?.languages?.length ? (
                <div className="mt-6 flex flex-wrap gap-2">
                  {projectStats.languages.map((language) => (
                    <span
                      key={language}
                      className="rounded-full border border-white/8 bg-black/18 px-3 py-1.5 text-[11px] font-medium uppercase tracking-[0.2em] text-gray-300"
                    >
                      {language}
                    </span>
                  ))}
                </div>
              ) : null}
            </div>
          </div>

          <div className="border-t border-white/6 bg-black/18 p-6 xl:border-l xl:border-t-0">
            <div className="flex flex-col gap-3">
              <Button size="sm" variant="ghost" onClick={handleShare} className="justify-between rounded-2xl border border-white/10 bg-white/[0.03] px-4 py-3 text-left text-gray-100 hover:bg-white/[0.05]">
                <span className="flex items-center gap-3">
                  <Share2 className="h-4 w-4 text-cyan-200" />
                  Share
                </span>
                <ArrowRight className="h-4 w-4 text-gray-500" />
              </Button>
              <Button size="sm" variant="ghost" onClick={handleSettings} className="justify-between rounded-2xl border border-white/10 bg-white/[0.03] px-4 py-3 text-left text-gray-100 hover:bg-white/[0.05]">
                <span className="flex items-center gap-3">
                  <Settings className="h-4 w-4 text-cyan-200" />
                  Settings
                </span>
                <ArrowRight className="h-4 w-4 text-gray-500" />
              </Button>
              <Button size="sm" variant="ghost" onClick={handleDownload} className="justify-between rounded-2xl border border-white/10 bg-white/[0.03] px-4 py-3 text-left text-gray-100 hover:bg-white/[0.05]">
                <span className="flex items-center gap-3">
                  <Download className="h-4 w-4 text-cyan-200" />
                  Download ZIP
                </span>
                <ArrowRight className="h-4 w-4 text-gray-500" />
              </Button>
              <Button size="sm" variant="primary" onClick={handleRunProject} className="justify-between rounded-2xl border border-cyan-300/18 bg-[linear-gradient(135deg,rgba(115,179,255,0.30),rgba(255,84,56,0.18))] px-4 py-3 text-left text-white shadow-[0_12px_36px_rgba(47,112,255,0.18)] hover:bg-[linear-gradient(135deg,rgba(115,179,255,0.38),rgba(255,84,56,0.24))]">
                <span className="flex items-center gap-3">
                  <Play className="h-4 w-4 text-white" />
                  Run Project
                </span>
                <ArrowRight className="h-4 w-4 text-white/75" />
              </Button>
            </div>

            <div className="mt-6 grid grid-cols-2 gap-3 text-left">
              <div className="rounded-2xl border border-white/8 bg-black/24 px-4 py-4">
                <div className="text-[11px] uppercase tracking-[0.24em] text-gray-500">Owner</div>
                <div className="mt-2 text-sm font-medium text-white">{currentProject.owner?.username || 'workspace'}</div>
              </div>
              <div className="rounded-2xl border border-white/8 bg-black/24 px-4 py-4">
                <div className="text-[11px] uppercase tracking-[0.24em] text-gray-500">Activity</div>
                <div className="mt-2 text-sm font-medium text-white">
                  {projectStats?.lastActivity
                    ? formatRelativeTime(new Date(projectStats.lastActivity).toISOString())
                    : 'No activity'}
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {projectStats ? (
        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <div className={statCardClass}>
            <div className="flex items-center justify-between">
              <FileText className="h-5 w-5 text-cyan-200" />
              <span className="text-[11px] uppercase tracking-[0.24em] text-gray-500">Files</span>
            </div>
            <div className="mt-6 text-5xl font-semibold tracking-[-0.05em] text-white">{projectStats.totalFiles}</div>
            <div className="mt-2 text-sm text-gray-400">Tracked source artifacts in this workspace.</div>
          </div>

          <div className={statCardClass}>
            <div className="flex items-center justify-between">
              <Folder className="h-5 w-5 text-cyan-200" />
              <span className="text-[11px] uppercase tracking-[0.24em] text-gray-500">Folders</span>
            </div>
            <div className="mt-6 text-5xl font-semibold tracking-[-0.05em] text-white">{projectStats.totalDirectories}</div>
            <div className="mt-2 text-sm text-gray-400">Workspace structure currently materialized.</div>
          </div>

          <div className={statCardClass}>
            <div className="flex items-center justify-between">
              <HardDrive className="h-5 w-5 text-cyan-200" />
              <span className="text-[11px] uppercase tracking-[0.24em] text-gray-500">Size</span>
            </div>
            <div className="mt-6 text-5xl font-semibold tracking-[-0.05em] text-white">{formatFileSize(projectStats.totalSize)}</div>
            <div className="mt-2 text-sm text-gray-400">Current file payload across the project.</div>
          </div>

          <div className={statCardClass}>
            <div className="flex items-center justify-between">
              <Activity className="h-5 w-5 text-cyan-200" />
              <span className="text-[11px] uppercase tracking-[0.24em] text-gray-500">Last Activity</span>
            </div>
            <div className="mt-6 text-3xl font-semibold tracking-[-0.05em] text-white">
              {projectStats.lastActivity
                ? formatRelativeTime(new Date(projectStats.lastActivity).toISOString())
                : 'Never'}
            </div>
            <div className="mt-2 text-sm text-gray-400">Most recent file or workspace update.</div>
          </div>
        </section>
      ) : null}

      <section className="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(320px,0.8fr)]">
        <div className="flex flex-col gap-6">
          <div className={cn(panelClass, 'p-6')}>
            <div className="mb-5 flex items-center justify-between gap-4">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.3em] text-cyan-200/75">Recent Files</div>
                <h2 className="mt-2 text-2xl font-semibold tracking-[-0.03em] text-white">Latest workspace activity</h2>
              </div>
              <div className="rounded-full border border-white/8 bg-black/18 px-3 py-1 text-xs text-gray-400">
                {recentFiles.length} shown
              </div>
            </div>

            {recentFiles.length > 0 ? (
              <div className="space-y-3">
                {recentFiles.map((file) => (
                  <div
                    key={file.id}
                    className="grid gap-3 rounded-2xl border border-white/8 bg-black/20 px-4 py-4 md:grid-cols-[minmax(0,1fr)_auto]"
                  >
                    <div className="min-w-0">
                      <div className="flex items-center gap-3">
                        <span className="text-xl">{getFileIcon(file.name)}</span>
                        <div className="min-w-0">
                          <div className="truncate text-sm font-medium text-white">{file.name}</div>
                          <div className="truncate text-xs text-gray-500">{file.path}</div>
                        </div>
                      </div>
                    </div>
                    <div className="flex flex-wrap items-center gap-3 text-xs text-gray-400 md:justify-end">
                      <span>{formatFileSize(file.size)}</span>
                      <span>{formatRelativeTime(file.updated_at)}</span>
                      {file.last_editor?.username ? <span>by {file.last_editor.username}</span> : null}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-2xl border border-dashed border-white/10 bg-black/18 px-6 py-12 text-center">
                <FileText className="mx-auto h-10 w-10 text-white/20" />
                <p className="mt-4 text-sm text-gray-400">No files have been updated yet.</p>
              </div>
            )}
          </div>

          <div className={cn(panelClass, 'p-6')}>
            <div className="mb-5 flex items-center justify-between gap-4">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.3em] text-cyan-200/75">Runtime History</div>
                <h2 className="mt-2 text-2xl font-semibold tracking-[-0.03em] text-white">Recent runs</h2>
              </div>
              <Terminal className="h-5 w-5 text-cyan-200" />
            </div>

            {recentExecutions.length > 0 ? (
              <div className="space-y-3">
                {recentExecutions.map((execution) => {
                  const statusTone =
                    execution.status === 'completed'
                      ? 'border-emerald-400/18 bg-emerald-400/8 text-emerald-100'
                      : execution.status === 'failed'
                        ? 'border-red-400/18 bg-red-400/8 text-red-100'
                        : 'border-amber-400/18 bg-amber-400/8 text-amber-100'

                  return (
                    <div
                      key={execution.id}
                      className="grid gap-3 rounded-2xl border border-white/8 bg-black/20 px-4 py-4 md:grid-cols-[minmax(0,1fr)_auto]"
                    >
                      <div className="min-w-0">
                        <div className="truncate text-sm font-medium text-white">{execution.command}</div>
                        <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500">
                          <Clock3 className="h-3.5 w-3.5" />
                          <span>{formatRelativeTime(execution.started_at)}</span>
                          {execution.duration ? <span>{execution.duration}ms</span> : null}
                          {typeof execution.exit_code === 'number' && execution.exit_code !== 0 ? (
                            <span className="text-red-300">exit {execution.exit_code}</span>
                          ) : null}
                        </div>
                      </div>
                      <div className="md:text-right">
                        <span className={cn('inline-flex rounded-full border px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em]', statusTone)}>
                          {execution.status}
                        </span>
                      </div>
                    </div>
                  )
                })}
              </div>
            ) : (
              <div className="rounded-2xl border border-dashed border-white/10 bg-black/18 px-6 py-12 text-center">
                <Play className="mx-auto h-10 w-10 text-white/20" />
                <p className="mt-4 text-sm text-gray-400">This project has not been executed yet.</p>
              </div>
            )}
          </div>
        </div>

        <div className="flex flex-col gap-6">
          <div className={cn(panelClass, 'p-6')}>
            <div className="mb-5 flex items-center justify-between gap-4">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.3em] text-cyan-200/75">Collaboration</div>
                <h2 className="mt-2 text-2xl font-semibold tracking-[-0.03em] text-white">Team presence</h2>
              </div>
              <Users className="h-5 w-5 text-cyan-200" />
            </div>

            {collaborationUsers.length > 0 ? (
              <div className="space-y-3">
                {collaborationUsers.map((collaborator) => (
                  <div
                    key={collaborator.id}
                    className="flex items-center justify-between rounded-2xl border border-white/8 bg-black/20 px-4 py-3"
                  >
                    <div>
                      <div className="text-sm font-medium text-white">{collaborator.username}</div>
                      <div className="text-xs text-gray-500">Active now</div>
                    </div>
                    <span className="inline-flex items-center gap-2 text-xs text-emerald-200">
                      <span className="h-2 w-2 rounded-full bg-emerald-400" />
                      Online
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-2xl border border-dashed border-white/10 bg-black/18 px-6 py-10 text-center">
                <Users className="mx-auto h-10 w-10 text-white/20" />
                <p className="mt-4 text-sm text-gray-400">No collaborators are active in this workspace.</p>
              </div>
            )}
          </div>

          <div className={cn(panelClass, 'p-6')}>
            <div className="mb-5 flex items-center justify-between gap-4">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.3em] text-cyan-200/75">AI Usage</div>
                <h2 className="mt-2 text-2xl font-semibold tracking-[-0.03em] text-white">Provider spend</h2>
              </div>
              <Brain className="h-5 w-5 text-cyan-200" />
            </div>

            {aiUsage ? (
              <>
                <div className="grid grid-cols-2 gap-3">
                  <div className="rounded-2xl border border-white/8 bg-black/20 px-4 py-4">
                    <div className="text-[11px] uppercase tracking-[0.24em] text-gray-500">Requests</div>
                    <div className="mt-3 text-3xl font-semibold tracking-[-0.04em] text-white">
                      {aiUsage.total_requests}
                    </div>
                  </div>
                  <div className="rounded-2xl border border-white/8 bg-black/20 px-4 py-4">
                    <div className="text-[11px] uppercase tracking-[0.24em] text-gray-500">Cost</div>
                    <div className="mt-3 text-3xl font-semibold tracking-[-0.04em] text-white">
                      ${aiUsage.total_cost.toFixed(3)}
                    </div>
                  </div>
                </div>

                <div className="mt-4 space-y-3">
                  {Object.entries(aiUsage.by_provider).map(([provider, usage]) => (
                    <div
                      key={provider}
                      className="flex items-center justify-between rounded-2xl border border-white/8 bg-black/20 px-4 py-3"
                    >
                      <div>
                        <div className="text-sm font-medium capitalize text-white">{provider}</div>
                        <div className="text-xs text-gray-500">{usage.requests} requests</div>
                      </div>
                      <div className="text-sm font-medium text-cyan-100">${usage.cost.toFixed(3)}</div>
                    </div>
                  ))}
                </div>
              </>
            ) : (
              <div className="rounded-2xl border border-dashed border-white/10 bg-black/18 px-6 py-10 text-center">
                <Brain className="mx-auto h-10 w-10 text-white/20" />
                <p className="mt-4 text-sm text-gray-400">AI usage data is not available for this workspace yet.</p>
              </div>
            )}
          </div>
        </div>
      </section>
    </div>
  )
}

export default ProjectDashboard
