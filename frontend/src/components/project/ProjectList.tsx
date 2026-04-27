// APEX-BUILD Project List
// Redesigned project browser for the blue/navy workspace shell

import React, { useState, useEffect } from 'react'
import { cn, formatRelativeTime } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import { Project, LanguageConfig } from '@/types'
import {
  Button,
  Badge,
  Loading,
  Avatar
} from '@/components/ui'
import {
  Plus,
  Search,
  Grid,
  List,
  Clock,
  Eye,
  EyeOff,
  Code,
  Trash2,
  GitBranch,
  Play,
  Edit3,
  FolderOpen,
  Sparkles,
  Globe2,
  Lock,
  ChevronRight,
} from 'lucide-react'

// Language configurations
const LANGUAGE_CONFIGS: LanguageConfig[] = [
  { id: 'javascript', name: 'JavaScript', extensions: ['js', 'jsx'], icon: '📜', color: '#f7df1e', monacoLanguage: 'javascript', defaultCode: 'console.log("Hello APEX-BUILD!");', runCommand: 'node' },
  { id: 'typescript', name: 'TypeScript', extensions: ['ts', 'tsx'], icon: '🔷', color: '#3178c6', monacoLanguage: 'typescript', defaultCode: 'console.log("Hello APEX-BUILD!");', runCommand: 'tsx' },
  { id: 'python', name: 'Python', extensions: ['py'], icon: '🐍', color: '#3776ab', monacoLanguage: 'python', defaultCode: 'print("Hello APEX-BUILD!")', runCommand: 'python' },
  { id: 'go', name: 'Go', extensions: ['go'], icon: '🐹', color: '#00add8', monacoLanguage: 'go', defaultCode: 'package main\n\nimport "fmt"\n\nfunc main() {\n    fmt.Println("Hello APEX-BUILD!")\n}', runCommand: 'go run' },
  { id: 'rust', name: 'Rust', extensions: ['rs'], icon: '🦀', color: '#dea584', monacoLanguage: 'rust', defaultCode: 'fn main() {\n    println!("Hello APEX-BUILD!");\n}', runCommand: 'cargo run' },
  { id: 'java', name: 'Java', extensions: ['java'], icon: '☕', color: '#ed8b00', monacoLanguage: 'java', defaultCode: 'public class Main {\n    public static void main(String[] args) {\n        System.out.println("Hello APEX-BUILD!");\n    }\n}', runCommand: 'java' },
  { id: 'cpp', name: 'C++', extensions: ['cpp', 'cc', 'cxx'], icon: '⚙️', color: '#00599c', monacoLanguage: 'cpp', defaultCode: '#include <iostream>\n\nint main() {\n    std::cout << "Hello APEX-BUILD!" << std::endl;\n    return 0;\n}', runCommand: 'g++' },
  { id: 'html', name: 'HTML/CSS/JS', extensions: ['html', 'css'], icon: '🌐', color: '#e34f26', monacoLanguage: 'html', defaultCode: '<!DOCTYPE html>\n<html>\n<head>\n    <title>APEX-BUILD</title>\n</head>\n<body>\n    <h1>Hello APEX-BUILD!</h1>\n</body>\n</html>', runCommand: 'serve' }
]

const FRAMEWORKS = {
  javascript: ['React', 'Vue', 'Angular', 'Express', 'Next.js', 'Svelte', 'Node.js'],
  typescript: ['React', 'Vue', 'Angular', 'Express', 'Next.js', 'NestJS', 'Deno'],
  python: ['FastAPI', 'Django', 'Flask', 'Streamlit', 'Jupyter', 'Pygame'],
  go: ['Gin', 'Echo', 'Fiber', 'Chi', 'Gorilla'],
  rust: ['Actix', 'Rocket', 'Warp', 'Axum', 'Tauri'],
  java: ['Spring Boot', 'Quarkus', 'Micronaut', 'Android'],
  cpp: ['Qt', 'SFML', 'Unreal Engine', 'OpenGL'],
  html: ['Bootstrap', 'Tailwind CSS', 'Materialize', 'Bulma']
}

export interface ProjectListProps {
  className?: string
  onProjectSelect?: (project: Project) => void
  onProjectCreate?: (project: Project) => void
  onProjectRun?: (project: Project) => void
  showCreateButton?: boolean
}

export const ProjectList: React.FC<ProjectListProps> = ({
  className,
  onProjectSelect,
  onProjectCreate,
  onProjectRun,
  showCreateButton = true,
}) => {
  const [projects, setProjects] = useState<Project[]>([])
  const [filteredProjects, setFilteredProjects] = useState<Project[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState('')
  const [filterLanguage, setFilterLanguage] = useState('')
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid')
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [newProject, setNewProject] = useState({
    name: '',
    description: '',
    language: '',
    framework: '',
    is_public: false
  })
  const [isCreating, setIsCreating] = useState(false)
  const [editProject, setEditProject] = useState<{
    id: number
    name: string
    description: string
    language: string
    framework: string
    is_public: boolean
  } | null>(null)
  const [isUpdating, setIsUpdating] = useState(false)

  const { apiService, setCurrentProject, currentProject, user } = useStore()
  const canPublishProjects = user != null && ['builder', 'pro', 'team', 'enterprise', 'owner'].includes(user.subscription_type)

  // Load projects
  useEffect(() => {
    loadProjects()
  }, []) // eslint-disable-line react-hooks/exhaustive-deps -- list bootstrap is intentionally one-shot on mount.

  const loadProjects = async () => {
    setIsLoading(true)
    try {
      const projectsData = await apiService.getProjects()
      setProjects(projectsData)
      setFilteredProjects(projectsData)
    } catch (error) {
      console.error('Failed to load projects:', error)
    } finally {
      setIsLoading(false)
    }
  }

  // Filter projects
  useEffect(() => {
    let filtered = projects

    if (searchQuery) {
      filtered = filtered.filter(project =>
        project.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        project.description?.toLowerCase().includes(searchQuery.toLowerCase())
      )
    }

    if (filterLanguage) {
      filtered = filtered.filter(project =>
        project.language.toLowerCase() === filterLanguage.toLowerCase()
      )
    }

    setFilteredProjects(filtered)
  }, [projects, searchQuery, filterLanguage])

  // Handle project creation
  const handleCreateProject = async () => {
    if (!newProject.name.trim() || !newProject.language) return

    setIsCreating(true)
    try {
      const project = await apiService.createProject({
        name: newProject.name.trim(),
        description: newProject.description.trim() || undefined,
        language: newProject.language,
        framework: newProject.framework || undefined,
        is_public: canPublishProjects && newProject.is_public,
        environment: {
          language: newProject.language,
          framework: newProject.framework
        }
      })

      setProjects(prev => [project, ...prev])
      setShowCreateModal(false)
      setNewProject({
        name: '',
        description: '',
        language: '',
        framework: '',
        is_public: false
      })

      setCurrentProject(project)
      onProjectCreate?.(project)
    } catch (error) {
      console.error('Failed to create project:', error)
    } finally {
      setIsCreating(false)
    }
  }

  // Handle project selection
  const handleProjectSelect = (project: Project) => {
    setCurrentProject(project)
    onProjectSelect?.(project)
  }

  const handleRunProject = (project: Project) => {
    setCurrentProject(project)
    if (onProjectRun) {
      onProjectRun(project)
      return
    }
    onProjectSelect?.(project)
  }

  const handleEditProject = (project: Project) => {
    setEditProject({
      id: project.id,
      name: project.name || '',
      description: project.description || '',
      language: project.language || '',
      framework: project.framework || '',
      is_public: project.is_public || false
    })
    setShowEditModal(true)
  }

  const handleUpdateProject = async () => {
    if (!editProject) return
    if (!editProject.name.trim() || !editProject.language) return

    setIsUpdating(true)
    try {
      const updated = await apiService.updateProject(editProject.id, {
        name: editProject.name.trim(),
        description: editProject.description.trim() || undefined,
        language: editProject.language,
        framework: editProject.framework || undefined,
        is_public: editProject.is_public && canPublishProjects,
        environment: {
          language: editProject.language,
          framework: editProject.framework
        }
      })

      setProjects(prev => prev.map(p => (p.id === updated.id ? updated : p)))
      if (currentProject?.id === updated.id) {
        setCurrentProject(updated)
      }
      setShowEditModal(false)
      setEditProject(null)
    } catch (error) {
      console.error('Failed to update project:', error)
    } finally {
      setIsUpdating(false)
    }
  }

  // Delete project
  const handleDeleteProject = async (project: Project) => {
    if (!confirm(`Are you sure you want to delete "${project.name}"?`)) return

    try {
      await apiService.deleteProject(project.id)
      setProjects(prev => prev.filter(p => p.id !== project.id))
    } catch (error) {
      console.error('Failed to delete project:', error)
    }
  }

  // Get language config
  const getLanguageConfig = (languageId: string) => {
    return LANGUAGE_CONFIGS.find(lang => lang.id === languageId) || LANGUAGE_CONFIGS[0]
  }

  const panelClass = 'rounded-[30px] border border-[rgba(138,223,255,0.14)] bg-[linear-gradient(180deg,rgba(6,12,24,0.94),rgba(4,8,16,0.9))] shadow-[0_24px_70px_rgba(0,0,0,0.34)]'
  const shellInputClass = 'h-11 w-full rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.84)] px-4 text-sm text-white placeholder:text-[#51667f] transition focus:border-[rgba(138,223,255,0.42)] focus:outline-none focus:ring-2 focus:ring-[rgba(47,168,255,0.18)]'
  const shellSelectClass = 'h-11 rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.84)] px-4 text-sm text-white transition focus:border-[rgba(138,223,255,0.42)] focus:outline-none focus:ring-2 focus:ring-[rgba(47,168,255,0.18)]'

  // Render project card
  const renderProjectCard = (project: Project) => {
    const langConfig = getLanguageConfig(project.language)
    const ownerLabel = project.owner?.username || 'Private workspace'

    return (
      <article
        key={project.id}
        className="group relative overflow-hidden rounded-[30px] border border-[rgba(138,223,255,0.16)] bg-[linear-gradient(180deg,rgba(8,14,28,0.94),rgba(4,8,16,0.92))] p-5 shadow-[0_20px_60px_rgba(0,0,0,0.28)] transition duration-200 hover:border-[rgba(138,223,255,0.28)] hover:bg-[linear-gradient(180deg,rgba(10,18,35,0.96),rgba(6,10,18,0.94))]"
        onClick={() => handleProjectSelect(project)}
      >
        <div className="pointer-events-none absolute inset-x-0 top-0 h-24 bg-[radial-gradient(circle_at_top_left,rgba(138,223,255,0.2),transparent_58%)]" />
        <div className="relative flex h-full flex-col gap-5">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 flex items-start gap-4">
              <div
                className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border text-xl shadow-[0_12px_30px_rgba(0,0,0,0.22)]"
                style={{ backgroundColor: `${langConfig.color}18`, borderColor: `${langConfig.color}3f`, color: langConfig.color }}
              >
                {langConfig.icon}
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="truncate text-xl font-semibold tracking-tight text-white">{project.name}</h3>
                  {project.is_public ? (
                    <Badge
                      variant="outline"
                      size="xs"
                      className="border-[rgba(125,226,164,0.28)] bg-[rgba(92,214,143,0.12)] text-[#b8f3cf]"
                      icon={<Globe2 size={10} />}
                    >
                      Public
                    </Badge>
                  ) : (
                    <Badge
                      variant="outline"
                      size="xs"
                      className="border-[rgba(138,223,255,0.2)] bg-[rgba(47,168,255,0.1)] text-[#cbeeff]"
                      icon={<Lock size={10} />}
                    >
                      Private
                    </Badge>
                  )}
                </div>
                {project.description ? (
                  <p className="mt-2 line-clamp-2 text-sm leading-6 text-[#91a6bc]">{project.description}</p>
                ) : (
                  <p className="mt-2 text-sm leading-6 text-[#6c8299]">Ready for builds, edits, previews, and deployment handoff.</p>
                )}
              </div>
            </div>

            <div className="flex items-center gap-1 opacity-100 transition md:opacity-0 md:group-hover:opacity-100">
              <Button
                size="xs"
                variant="ghost"
                onClick={(e) => {
                  e.stopPropagation()
                  handleEditProject(project)
                }}
                icon={<Edit3 size={12} />}
                className="rounded-xl border border-[#17314d] bg-[rgba(7,13,24,0.76)] text-[#cfe6ff] hover:bg-[rgba(12,21,36,0.92)] hover:text-white"
              />
              <Button
                size="xs"
                variant="ghost"
                onClick={(e) => {
                  e.stopPropagation()
                  handleDeleteProject(project)
                }}
                icon={<Trash2 size={12} />}
                className="rounded-xl border border-[rgba(255,126,126,0.18)] bg-[rgba(42,10,14,0.72)] text-[#ffb0b0] hover:bg-[rgba(64,14,20,0.92)] hover:text-white"
              />
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline" size="xs" className="border-[#17314d] bg-[rgba(7,13,24,0.8)] text-[#d9ebff]">
              {langConfig.name}
            </Badge>
            {project.framework && (
              <Badge variant="outline" size="xs" className="border-[#17314d] bg-[rgba(7,13,24,0.8)] text-[#d9ebff]">
                {project.framework}
              </Badge>
            )}
          </div>

          <div className="grid gap-3 text-xs text-[#7f95ad] sm:grid-cols-2">
            <div className="rounded-2xl border border-[#13283f] bg-[rgba(7,13,24,0.66)] px-3 py-2.5">
              <div className="mb-1 flex items-center gap-2 text-[11px] uppercase tracking-[0.22em] text-[#67819d]">
                <Clock size={12} />
                Updated
              </div>
              <div className="text-sm text-[#dceeff]">{formatRelativeTime(project.updated_at)}</div>
            </div>
            <div className="rounded-2xl border border-[#13283f] bg-[rgba(7,13,24,0.66)] px-3 py-2.5">
              <div className="mb-1 flex items-center gap-2 text-[11px] uppercase tracking-[0.22em] text-[#67819d]">
                <GitBranch size={12} />
                Branch
              </div>
              <div className="text-sm text-[#dceeff]">main</div>
            </div>
          </div>

          <div className="mt-auto flex items-center justify-between gap-3 border-t border-[rgba(138,223,255,0.08)] pt-4">
            <div className="flex min-w-0 items-center gap-2 text-xs text-[#88a0b8]">
              <Avatar
                size="xs"
                src={project.owner?.avatar_url}
                fallback={project.owner?.username || project.name}
              />
              <span className="truncate">{ownerLabel}</span>
            </div>

            <div className="flex items-center gap-2">
              <Button
                size="xs"
                variant="ghost"
                onClick={(e) => {
                  e.stopPropagation()
                  handleRunProject(project)
                }}
                icon={<Play size={12} />}
                className="rounded-xl border border-[#17314d] bg-[rgba(7,13,24,0.76)] text-[#cfe6ff] hover:bg-[rgba(12,21,36,0.92)] hover:text-white"
              />
              <Button
                size="xs"
                variant="ghost"
                onClick={(e) => {
                  e.stopPropagation()
                  handleProjectSelect(project)
                }}
                icon={<ChevronRight size={12} />}
                className="rounded-xl border border-[rgba(138,223,255,0.3)] bg-[linear-gradient(135deg,rgba(138,223,255,0.22),rgba(47,168,255,0.2))] text-white hover:bg-[linear-gradient(135deg,rgba(138,223,255,0.28),rgba(47,168,255,0.28))]"
              >
                Open
              </Button>
            </div>
          </div>
        </div>
      </article>
    )
  }

  // Render project row (list view)
  const renderProjectRow = (project: Project) => {
    const langConfig = getLanguageConfig(project.language)

    return (
      <div
        key={project.id}
        className="group grid cursor-pointer gap-4 rounded-[26px] border border-[rgba(138,223,255,0.12)] bg-[rgba(7,13,24,0.76)] p-4 transition hover:border-[rgba(138,223,255,0.24)] hover:bg-[rgba(10,18,31,0.9)] lg:grid-cols-[minmax(0,1.8fr)_minmax(0,1fr)_auto]"
        onClick={() => handleProjectSelect(project)}
      >
        <div className="min-w-0">
          <div className="flex items-start gap-3">
            <div
              className="mt-1 flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl border text-base"
              style={{ backgroundColor: `${langConfig.color}18`, borderColor: `${langConfig.color}3f`, color: langConfig.color }}
            >
              {langConfig.icon}
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h3 className="truncate text-base font-semibold text-white">{project.name}</h3>
                <Badge variant="outline" size="xs" className="border-[#17314d] bg-[rgba(7,13,24,0.8)] text-[#d9ebff]">
                  {langConfig.name}
                </Badge>
                {project.framework && (
                  <Badge variant="outline" size="xs" className="border-[#17314d] bg-[rgba(7,13,24,0.8)] text-[#d9ebff]">
                    {project.framework}
                  </Badge>
                )}
              </div>
              {project.description ? (
                <p className="mt-1 truncate text-sm text-[#8ea4bb]">{project.description}</p>
              ) : (
                <p className="mt-1 truncate text-sm text-[#6d849c]">No description yet.</p>
              )}
            </div>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-3 text-sm text-[#88a0b8] lg:justify-end">
          <span className="flex items-center gap-1">
            <Clock size={12} />
            {formatRelativeTime(project.updated_at)}
          </span>
          {project.owner && (
            <div className="flex items-center gap-2">
              <Avatar
                size="xs"
                src={project.owner.avatar_url}
                fallback={project.owner.username}
              />
              <span className="truncate">{project.owner.username}</span>
            </div>
          )}
          {project.is_public ? (
            <Badge
              variant="outline"
              size="xs"
              className="border-[rgba(125,226,164,0.28)] bg-[rgba(92,214,143,0.12)] text-[#b8f3cf]"
              icon={<Eye size={10} />}
            >
              Public
            </Badge>
          ) : (
            <Badge
              variant="outline"
              size="xs"
              className="border-[rgba(138,223,255,0.2)] bg-[rgba(47,168,255,0.1)] text-[#cbeeff]"
              icon={<EyeOff size={10} />}
            >
              Private
            </Badge>
          )}
        </div>

        <div className="flex items-center justify-end gap-2 opacity-100 transition md:opacity-0 md:group-hover:opacity-100">
          <Button
            size="xs"
            variant="ghost"
            onClick={(e) => {
              e.stopPropagation()
              handleRunProject(project)
            }}
            icon={<Play size={12} />}
            className="rounded-xl border border-[#17314d] bg-[rgba(7,13,24,0.76)] text-[#cfe6ff] hover:bg-[rgba(12,21,36,0.92)] hover:text-white"
          />
          <Button
            size="xs"
            variant="ghost"
            onClick={(e) => {
              e.stopPropagation()
              handleProjectSelect(project)
            }}
            icon={<Code size={12} />}
            className="rounded-xl border border-[rgba(138,223,255,0.3)] bg-[linear-gradient(135deg,rgba(138,223,255,0.22),rgba(47,168,255,0.2))] text-white hover:bg-[linear-gradient(135deg,rgba(138,223,255,0.28),rgba(47,168,255,0.28))]"
          >
            Open
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className={cn('space-y-6', className)}>
      <section className={cn(panelClass, 'relative overflow-hidden p-6 md:p-7')}>
        <div className="pointer-events-none absolute inset-x-0 top-0 h-28 bg-[radial-gradient(circle_at_top_left,rgba(138,223,255,0.18),transparent_60%)]" />
        <div className="relative flex flex-col gap-6 xl:flex-row xl:items-end xl:justify-between">
          <div className="space-y-4">
            <div className="flex items-start gap-4">
              <div className="flex h-14 w-14 items-center justify-center rounded-[22px] border border-[rgba(138,223,255,0.22)] bg-[rgba(47,168,255,0.12)]">
                <FolderOpen className="h-7 w-7 text-[#8adfff]" />
              </div>
              <div>
                <div className="text-[11px] uppercase tracking-[0.26em] text-[#8adfff]/80">Workspace Index</div>
                <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">Projects</h1>
                <p className="mt-2 max-w-2xl text-sm leading-7 text-[#95a9be] md:text-base">
                  Open recent workspaces, restart existing builds, or spin up a clean project shell without dropping back into the legacy card stack.
                </p>
              </div>
            </div>

            <div className="flex flex-wrap gap-2">
              <div className="rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] px-3 py-2 text-sm text-[#c4d6e7]">
                <span className="mr-2 text-[#6f89a4]">Projects</span>
                <span className="font-medium text-white">{projects.length}</span>
              </div>
              <div className="rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] px-3 py-2 text-sm text-[#c4d6e7]">
                <span className="mr-2 text-[#6f89a4]">Visible</span>
                <span className="font-medium text-white">{filteredProjects.length}</span>
              </div>
              <div className="rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] px-3 py-2 text-sm text-[#c4d6e7]">
                <span className="mr-2 text-[#6f89a4]">Mode</span>
                <span className="font-medium text-white capitalize">{viewMode}</span>
              </div>
            </div>
          </div>

          {showCreateButton && (
            <Button
              onClick={() => setShowCreateModal(true)}
              icon={<Plus size={16} />}
              variant="ghost"
              className="rounded-2xl border border-[rgba(138,223,255,0.3)] bg-[linear-gradient(135deg,rgba(138,223,255,0.22),rgba(47,168,255,0.2))] px-5 text-white hover:bg-[linear-gradient(135deg,rgba(138,223,255,0.28),rgba(47,168,255,0.28))]"
            >
              New Project
            </Button>
          )}
        </div>
      </section>

      <section className={cn(panelClass, 'p-4 md:p-5')}>
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-1 flex-col gap-4 md:flex-row">
            <div className="relative min-w-0 flex-1">
              <Search className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-[#5f7892]" />
              <input
                placeholder="Search projects, prompts, and descriptions"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className={cn(shellInputClass, 'pl-11')}
              />
            </div>

            <select
              value={filterLanguage}
              onChange={(e) => setFilterLanguage(e.target.value)}
              className={cn(shellSelectClass, 'min-w-[220px]')}
            >
              <option value="">All languages</option>
              {LANGUAGE_CONFIGS.map(lang => (
                <option key={lang.id} value={lang.id}>
                  {lang.name}
                </option>
              ))}
            </select>
          </div>

          <div className="flex items-center gap-2">
            <div className="flex items-center gap-1 rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] p-1">
              <Button
                size="xs"
                variant="ghost"
                onClick={() => setViewMode('grid')}
                icon={<Grid size={14} />}
                className={cn(
                  'rounded-xl border px-3 text-[#b8d6f4] hover:text-white',
                  viewMode === 'grid'
                    ? 'border-[rgba(138,223,255,0.3)] bg-[rgba(47,168,255,0.18)] text-white'
                    : 'border-transparent bg-transparent'
                )}
              >
                Grid
              </Button>
              <Button
                size="xs"
                variant="ghost"
                onClick={() => setViewMode('list')}
                icon={<List size={14} />}
                className={cn(
                  'rounded-xl border px-3 text-[#b8d6f4] hover:text-white',
                  viewMode === 'list'
                    ? 'border-[rgba(138,223,255,0.3)] bg-[rgba(47,168,255,0.18)] text-white'
                    : 'border-transparent bg-transparent'
                )}
              >
                List
              </Button>
            </div>
          </div>
        </div>
      </section>

      {/* Projects */}
      {isLoading ? (
        <div className={cn(panelClass, 'flex items-center justify-center py-20')}>
          <Loading variant="orb" color="cyberpunk" text="Loading projects..." />
        </div>
      ) : filteredProjects.length > 0 ? (
        <div className={cn(
          viewMode === 'grid'
            ? 'grid grid-cols-1 gap-4 xl:grid-cols-2 2xl:grid-cols-3'
            : 'space-y-3'
        )}>
          {filteredProjects.map(project =>
            viewMode === 'grid' ? renderProjectCard(project) : renderProjectRow(project)
          )}
        </div>
      ) : (
        <div className={cn(panelClass, 'px-6 py-20 text-center')}>
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-3xl border border-[rgba(138,223,255,0.18)] bg-[rgba(47,168,255,0.08)]">
            <Sparkles className="h-8 w-8 text-[#8adfff]" />
          </div>
          <h3 className="mt-5 text-xl font-semibold text-white">
            {searchQuery || filterLanguage ? 'No matching projects' : 'No projects yet'}
          </h3>
          <p className="mx-auto mt-3 max-w-xl text-sm leading-7 text-[#8ea4bb]">
            {searchQuery || filterLanguage
              ? 'Adjust the search terms or language filter to widen the project list.'
              : 'Create your first project shell here, then hand it off to builds, previews, edits, and deployment.'
            }
          </p>
          {showCreateButton && (
            <Button
              onClick={() => setShowCreateModal(true)}
              icon={<Plus size={16} />}
              variant="ghost"
              className="mt-6 rounded-2xl border border-[rgba(138,223,255,0.3)] bg-[linear-gradient(135deg,rgba(138,223,255,0.22),rgba(47,168,255,0.2))] px-5 text-white hover:bg-[linear-gradient(135deg,rgba(138,223,255,0.28),rgba(47,168,255,0.28))]"
            >
              Create Project
            </Button>
          )}
        </div>
      )}

      {/* Create Project Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/50 backdrop-blur-sm p-4">
          <div className="flex min-h-full items-center justify-center">
            <div className="w-full max-w-2xl rounded-[32px] border border-[rgba(138,223,255,0.18)] bg-[linear-gradient(180deg,rgba(7,15,31,0.98),rgba(4,8,16,0.96))] p-6 shadow-[0_30px_90px_rgba(0,0,0,0.4)]">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <div className="text-[11px] uppercase tracking-[0.26em] text-[#8adfff]/80">Create workspace</div>
                  <h2 className="mt-2 flex items-center gap-2 text-2xl font-semibold text-white">
                    <Plus className="h-5 w-5 text-[#8adfff]" />
                    New Project
                  </h2>
                  <p className="mt-2 text-sm leading-7 text-[#8ea4bb]">Define the shell now and let builds, previews, and deployment bind to this workspace later.</p>
                </div>
              </div>

              <div className="mt-6 space-y-4">
                <div>
                  <label className="mb-2 block text-sm font-medium text-[#d9ebff]">Project Name</label>
                  <input
                    placeholder="My Awesome Project"
                    value={newProject.name}
                    onChange={(e) => setNewProject(prev => ({ ...prev, name: e.target.value }))}
                    className={shellInputClass}
                  />
                </div>

                <div>
                  <label className="mb-2 block text-sm font-medium text-[#d9ebff]">Description</label>
                  <textarea
                    placeholder="A brief description of your project"
                    value={newProject.description}
                    onChange={(e) => setNewProject(prev => ({ ...prev, description: e.target.value }))}
                    className="min-h-[112px] w-full rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.84)] px-4 py-3 text-sm text-white placeholder:text-[#51667f] transition focus:border-[rgba(138,223,255,0.42)] focus:outline-none focus:ring-2 focus:ring-[rgba(47,168,255,0.18)]"
                  />
                </div>

                <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                  <div>
                    <label className="mb-2 block text-sm font-medium text-[#d9ebff]">
                      Language
                    </label>
                    <select
                      value={newProject.language}
                      onChange={(e) => setNewProject(prev => ({
                        ...prev,
                        language: e.target.value,
                        framework: '' // Reset framework when language changes
                      }))}
                      className={cn(shellSelectClass, 'w-full')}
                      required
                    >
                      <option value="">Select Language</option>
                      {LANGUAGE_CONFIGS.map(lang => (
                        <option key={lang.id} value={lang.id}>
                          {lang.icon} {lang.name}
                        </option>
                      ))}
                    </select>
                  </div>

                  <div>
                    <label className="mb-2 block text-sm font-medium text-[#d9ebff]">
                      Framework
                    </label>
                    <select
                      value={newProject.framework}
                      onChange={(e) => setNewProject(prev => ({ ...prev, framework: e.target.value }))}
                      className={cn(shellSelectClass, 'w-full')}
                      disabled={!newProject.language}
                    >
                      <option value="">Select Framework</option>
                      {newProject.language && FRAMEWORKS[newProject.language as keyof typeof FRAMEWORKS]?.map(framework => (
                        <option key={framework} value={framework}>
                          {framework}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>

                <label className="flex items-center gap-3 rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.68)] px-4 py-3 text-sm text-[#d7e9fb]">
                  <input
                    type="checkbox"
                    checked={newProject.is_public}
                    onChange={(e) => setNewProject(prev => ({ ...prev, is_public: e.target.checked }))}
                    className="h-4 w-4 rounded border-[#2d4764] bg-[rgba(7,13,24,0.84)]"
                    disabled={!canPublishProjects}
                  />
                  Make project public
                </label>
                {!canPublishProjects && (
                  <p className="text-xs text-amber-300">Publishing projects requires Builder or higher.</p>
                )}
              </div>

              <div className="mt-6 flex justify-end gap-2">
                <Button
                  variant="ghost"
                  onClick={() => setShowCreateModal(false)}
                  disabled={isCreating}
                  className="rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] text-[#d8ebff] hover:bg-[rgba(11,20,35,0.92)]"
                >
                  Cancel
                </Button>
                <Button
                  variant="ghost"
                  onClick={handleCreateProject}
                  loading={isCreating}
                  disabled={!newProject.name.trim() || !newProject.language}
                  icon={<Plus size={16} />}
                  className="rounded-2xl border border-[rgba(138,223,255,0.3)] bg-[linear-gradient(135deg,rgba(138,223,255,0.22),rgba(47,168,255,0.2))] text-white hover:bg-[linear-gradient(135deg,rgba(138,223,255,0.28),rgba(47,168,255,0.28))]"
                >
                  Create Project
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Edit Project Modal */}
      {showEditModal && editProject && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/50 backdrop-blur-sm p-4">
          <div className="flex min-h-full items-center justify-center">
            <div className="w-full max-w-2xl rounded-[32px] border border-[rgba(138,223,255,0.18)] bg-[linear-gradient(180deg,rgba(7,15,31,0.98),rgba(4,8,16,0.96))] p-6 shadow-[0_30px_90px_rgba(0,0,0,0.4)]">
              <div>
                <div className="text-[11px] uppercase tracking-[0.26em] text-[#8adfff]/80">Refine workspace</div>
                <h2 className="mt-2 flex items-center gap-2 text-2xl font-semibold text-white">
                  <Edit3 className="h-5 w-5 text-[#8adfff]" />
                  Edit Project
                </h2>
              </div>

              <div className="mt-6 space-y-4">
                <div>
                  <label className="mb-2 block text-sm font-medium text-[#d9ebff]">Project Name</label>
                  <input
                    placeholder="My Awesome Project"
                    value={editProject.name}
                    onChange={(e) => setEditProject(prev => prev ? ({ ...prev, name: e.target.value }) : prev)}
                    className={shellInputClass}
                  />
                </div>

                <div>
                  <label className="mb-2 block text-sm font-medium text-[#d9ebff]">Description</label>
                  <textarea
                    placeholder="A brief description of your project"
                    value={editProject.description}
                    onChange={(e) => setEditProject(prev => prev ? ({ ...prev, description: e.target.value }) : prev)}
                    className="min-h-[112px] w-full rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.84)] px-4 py-3 text-sm text-white placeholder:text-[#51667f] transition focus:border-[rgba(138,223,255,0.42)] focus:outline-none focus:ring-2 focus:ring-[rgba(47,168,255,0.18)]"
                  />
                </div>

                <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                  <div>
                    <label className="mb-2 block text-sm font-medium text-[#d9ebff]">
                      Language
                    </label>
                    <select
                      value={editProject.language}
                      onChange={(e) => setEditProject(prev => prev ? ({
                        ...prev,
                        language: e.target.value,
                        framework: ''
                      }) : prev)}
                      className={cn(shellSelectClass, 'w-full')}
                      required
                    >
                      <option value="">Select Language</option>
                      {LANGUAGE_CONFIGS.map(lang => (
                        <option key={lang.id} value={lang.id}>
                          {lang.icon} {lang.name}
                        </option>
                      ))}
                    </select>
                  </div>

                  <div>
                    <label className="mb-2 block text-sm font-medium text-[#d9ebff]">
                      Framework
                    </label>
                    <select
                      value={editProject.framework}
                      onChange={(e) => setEditProject(prev => prev ? ({ ...prev, framework: e.target.value }) : prev)}
                      className={cn(shellSelectClass, 'w-full')}
                      disabled={!editProject.language}
                    >
                      <option value="">Select Framework</option>
                      {editProject.language && FRAMEWORKS[editProject.language as keyof typeof FRAMEWORKS]?.map(framework => (
                        <option key={framework} value={framework}>
                          {framework}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>

                <label className="flex items-center gap-3 rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.68)] px-4 py-3 text-sm text-[#d7e9fb]">
                  <input
                    type="checkbox"
                    checked={editProject.is_public}
                    onChange={(e) => setEditProject(prev => prev ? ({ ...prev, is_public: e.target.checked }) : prev)}
                    className="h-4 w-4 rounded border-[#2d4764] bg-[rgba(7,13,24,0.84)]"
                    disabled={!canPublishProjects && !editProject.is_public}
                  />
                  Make project public
                </label>
                {!canPublishProjects && !editProject.is_public && (
                  <p className="text-xs text-amber-300">Publishing projects requires Builder or higher.</p>
                )}
              </div>

              <div className="mt-6 flex justify-end gap-2">
                <Button
                  variant="ghost"
                  onClick={() => {
                    setShowEditModal(false)
                    setEditProject(null)
                  }}
                  disabled={isUpdating}
                  className="rounded-2xl border border-[#17314d] bg-[rgba(7,13,24,0.82)] text-[#d8ebff] hover:bg-[rgba(11,20,35,0.92)]"
                >
                  Cancel
                </Button>
                <Button
                  variant="ghost"
                  onClick={handleUpdateProject}
                  loading={isUpdating}
                  disabled={!editProject.name.trim() || !editProject.language}
                  icon={<Edit3 size={16} />}
                  className="rounded-2xl border border-[rgba(138,223,255,0.3)] bg-[linear-gradient(135deg,rgba(138,223,255,0.22),rgba(47,168,255,0.2))] text-white hover:bg-[linear-gradient(135deg,rgba(138,223,255,0.28),rgba(47,168,255,0.28))]"
                >
                  Save Changes
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default ProjectList
