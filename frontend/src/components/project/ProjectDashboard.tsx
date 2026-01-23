// APEX.BUILD Project Dashboard
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
}

export const ProjectDashboard: React.FC<ProjectDashboardProps> = ({
  className,
  projectId,
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
    setCurrentProject
  } = useStore()

  // Load project data
  useEffect(() => {
    if (projectId && projectId !== currentProject?.id) {
      loadProject(projectId)
    }
  }, [projectId])

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
    <div className={cn('space-y-6 p-6', className)}>
      {/* Project Header */}
      <Card variant="cyberpunk" glow="intense">
        <CardHeader>
          <div className="flex items-start justify-between">
            <div className="space-y-2">
              <CardTitle className="text-2xl flex items-center gap-3">
                <div className="p-2 bg-cyan-500/20 rounded-lg">
                  <Folder className="w-6 h-6 text-cyan-400" />
                </div>
                {currentProject.name}
                {currentProject.is_public && (
                  <Badge variant="success" icon={<Eye size={12} />}>
                    Public
                  </Badge>
                )}
              </CardTitle>

              {currentProject.description && (
                <p className="text-gray-400">{currentProject.description}</p>
              )}

              <div className="flex items-center gap-4 text-sm text-gray-400">
                <span className="flex items-center gap-1">
                  <Code size={14} />
                  {currentProject.language}
                </span>
                {currentProject.framework && (
                  <span className="flex items-center gap-1">
                    <Settings size={14} />
                    {currentProject.framework}
                  </span>
                )}
                <span className="flex items-center gap-1">
                  <Calendar size={14} />
                  Created {formatRelativeTime(currentProject.created_at)}
                </span>
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Button size="sm" variant="ghost" icon={<Share2 size={14} />}>
                Share
              </Button>
              <Button size="sm" variant="ghost" icon={<Settings size={14} />}>
                Settings
              </Button>
              <Button size="sm" variant="primary" icon={<Play size={14} />}>
                Run Project
              </Button>
            </div>
          </div>
        </CardHeader>

        {/* Project Stats */}
        {projectStats && (
          <CardContent>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div className="text-center p-3 bg-gray-800/30 rounded-lg border border-gray-700/50">
                <FileText className="w-5 h-5 text-cyan-400 mx-auto mb-1" />
                <div className="text-lg font-semibold text-white">{projectStats.totalFiles}</div>
                <div className="text-xs text-gray-400">Files</div>
              </div>

              <div className="text-center p-3 bg-gray-800/30 rounded-lg border border-gray-700/50">
                <Folder className="w-5 h-5 text-pink-400 mx-auto mb-1" />
                <div className="text-lg font-semibold text-white">{projectStats.totalDirectories}</div>
                <div className="text-xs text-gray-400">Folders</div>
              </div>

              <div className="text-center p-3 bg-gray-800/30 rounded-lg border border-gray-700/50">
                <HardDrive className="w-5 h-5 text-green-400 mx-auto mb-1" />
                <div className="text-lg font-semibold text-white">{formatFileSize(projectStats.totalSize)}</div>
                <div className="text-xs text-gray-400">Size</div>
              </div>

              <div className="text-center p-3 bg-gray-800/30 rounded-lg border border-gray-700/50">
                <Activity className="w-5 h-5 text-yellow-400 mx-auto mb-1" />
                <div className="text-lg font-semibold text-white">
                  {projectStats.lastActivity
                    ? formatRelativeTime(new Date(projectStats.lastActivity).toISOString())
                    : 'Never'
                  }
                </div>
                <div className="text-xs text-gray-400">Last Activity</div>
              </div>
            </div>

            {/* Languages */}
            {projectStats.languages.length > 0 && (
              <div className="mt-4">
                <h4 className="text-sm font-medium text-gray-300 mb-2">Languages</h4>
                <div className="flex flex-wrap gap-1">
                  {projectStats.languages.map(lang => (
                    <Badge key={lang} variant="outline" size="xs">
                      {lang.toUpperCase()}
                    </Badge>
                  ))}
                </div>
              </div>
            )}
          </CardContent>
        )}
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Recent Files */}
        <Card variant="cyberpunk">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Clock className="w-5 h-5 text-cyan-400" />
              Recent Files
            </CardTitle>
          </CardHeader>
          <CardContent>
            {recentFiles.length > 0 ? (
              <div className="space-y-2">
                {recentFiles.map(file => (
                  <div
                    key={file.id}
                    className="flex items-center gap-3 p-2 hover:bg-gray-800/50 rounded-lg transition-colors cursor-pointer group"
                  >
                    <div className="text-lg">{getFileIcon(file.name)}</div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between">
                        <span className="text-sm font-medium text-gray-300 truncate">
                          {file.name}
                        </span>
                        <span className="text-xs text-gray-500">
                          {formatFileSize(file.size)}
                        </span>
                      </div>
                      <div className="flex items-center gap-2 text-xs text-gray-400">
                        <span>{file.path}</span>
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
                      <Badge variant="warning" size="xs">
                        Locked
                      </Badge>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-8 text-gray-400">
                <FileText className="w-8 h-8 mx-auto mb-2 opacity-50" />
                <p className="text-sm">No files yet</p>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Recent Executions */}
        <Card variant="cyberpunk">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Terminal className="w-5 h-5 text-green-400" />
              Recent Runs
            </CardTitle>
          </CardHeader>
          <CardContent>
            {recentExecutions.length > 0 ? (
              <div className="space-y-2">
                {recentExecutions.map(execution => (
                  <div
                    key={execution.id}
                    className="flex items-center gap-3 p-2 hover:bg-gray-800/50 rounded-lg transition-colors"
                  >
                    <StatusBadge
                      status={execution.status === 'completed' ? 'online' :
                             execution.status === 'failed' ? 'busy' :
                             execution.status === 'running' ? 'away' : 'offline'}
                    />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between">
                        <span className="text-sm font-medium text-gray-300 truncate">
                          {execution.command}
                        </span>
                        <span className="text-xs text-gray-500">
                          {execution.duration}ms
                        </span>
                      </div>
                      <div className="text-xs text-gray-400">
                        {formatRelativeTime(execution.started_at)}
                        {execution.exit_code !== 0 && (
                          <span className="text-red-400 ml-2">
                            Exit code: {execution.exit_code}
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-8 text-gray-400">
                <Terminal className="w-8 h-8 mx-auto mb-2 opacity-50" />
                <p className="text-sm">No executions yet</p>
              </div>
            )}
          </CardContent>
        </Card>

        {/* AI Usage */}
        {aiUsage && (
          <Card variant="matrix">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Brain className="w-5 h-5 text-green-400" />
                AI Usage
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 gap-4">
                <div className="text-center">
                  <div className="text-2xl font-bold text-green-400">{aiUsage.total_requests}</div>
                  <div className="text-xs text-gray-400">Total Requests</div>
                </div>
                <div className="text-center">
                  <div className="text-2xl font-bold text-green-400">${aiUsage.total_cost.toFixed(3)}</div>
                  <div className="text-xs text-gray-400">Total Cost</div>
                </div>
              </div>

              <div className="mt-4 space-y-2">
                {Object.entries(aiUsage.by_provider).map(([provider, usage]) => (
                  <div key={provider} className="flex items-center justify-between">
                    <span className="text-sm text-gray-300 capitalize">{provider}</span>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-gray-400">{usage.requests} req</span>
                      <span className="text-xs text-green-400">${usage.cost.toFixed(3)}</span>
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        )}

        {/* Collaboration */}
        <Card variant="synthwave">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Users className="w-5 h-5 text-pink-400" />
              Collaboration
            </CardTitle>
          </CardHeader>
          <CardContent>
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

                <div className="text-sm text-gray-400">
                  {collaborationUsers.length} user(s) currently active
                </div>

                <Button size="sm" variant="ghost" icon={<Share2 size={14} />} className="w-full">
                  Invite Collaborators
                </Button>
              </div>
            ) : (
              <div className="text-center py-8 text-gray-400">
                <Users className="w-8 h-8 mx-auto mb-2 opacity-50" />
                <p className="text-sm mb-3">No active collaborators</p>
                <Button size="sm" variant="ghost" icon={<Share2 size={14} />}>
                  Invite People
                </Button>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

export default ProjectDashboard