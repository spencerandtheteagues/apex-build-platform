// APEX.BUILD Project List
// Cyberpunk project browser and creator

import React, { useState, useEffect } from 'react'
import { cn, formatRelativeTime, getFileIcon } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import { Project, LanguageConfig } from '@/types'
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardFooter,
  Button,
  Badge,
  Loading,
  Input,
  Avatar
} from '@/components/ui'
import {
  Plus,
  Search,
  Filter,
  Grid,
  List,
  Star,
  Clock,
  Users,
  Eye,
  EyeOff,
  Code,
  Settings,
  Trash2,
  GitBranch,
  Play,
  Edit3
} from 'lucide-react'

// Language configurations
const LANGUAGE_CONFIGS: LanguageConfig[] = [
  { id: 'javascript', name: 'JavaScript', extensions: ['js', 'jsx'], icon: '📜', color: '#f7df1e', monacoLanguage: 'javascript', defaultCode: 'console.log("Hello APEX.BUILD!");', runCommand: 'node' },
  { id: 'typescript', name: 'TypeScript', extensions: ['ts', 'tsx'], icon: '🔷', color: '#3178c6', monacoLanguage: 'typescript', defaultCode: 'console.log("Hello APEX.BUILD!");', runCommand: 'tsx' },
  { id: 'python', name: 'Python', extensions: ['py'], icon: '🐍', color: '#3776ab', monacoLanguage: 'python', defaultCode: 'print("Hello APEX.BUILD!")', runCommand: 'python' },
  { id: 'go', name: 'Go', extensions: ['go'], icon: '🐹', color: '#00add8', monacoLanguage: 'go', defaultCode: 'package main\n\nimport "fmt"\n\nfunc main() {\n    fmt.Println("Hello APEX.BUILD!")\n}', runCommand: 'go run' },
  { id: 'rust', name: 'Rust', extensions: ['rs'], icon: '🦀', color: '#dea584', monacoLanguage: 'rust', defaultCode: 'fn main() {\n    println!("Hello APEX.BUILD!");\n}', runCommand: 'cargo run' },
  { id: 'java', name: 'Java', extensions: ['java'], icon: '☕', color: '#ed8b00', monacoLanguage: 'java', defaultCode: 'public class Main {\n    public static void main(String[] args) {\n        System.out.println("Hello APEX.BUILD!");\n    }\n}', runCommand: 'java' },
  { id: 'cpp', name: 'C++', extensions: ['cpp', 'cc', 'cxx'], icon: '⚙️', color: '#00599c', monacoLanguage: 'cpp', defaultCode: '#include <iostream>\n\nint main() {\n    std::cout << "Hello APEX.BUILD!" << std::endl;\n    return 0;\n}', runCommand: 'g++' },
  { id: 'html', name: 'HTML/CSS/JS', extensions: ['html', 'css'], icon: '🌐', color: '#e34f26', monacoLanguage: 'html', defaultCode: '<!DOCTYPE html>\n<html>\n<head>\n    <title>APEX.BUILD</title>\n</head>\n<body>\n    <h1>Hello APEX.BUILD!</h1>\n</body>\n</html>', runCommand: 'serve' }
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
  showCreateButton?: boolean
}

export const ProjectList: React.FC<ProjectListProps> = ({
  className,
  onProjectSelect,
  onProjectCreate,
  showCreateButton = true,
}) => {
  const [projects, setProjects] = useState<Project[]>([])
  const [filteredProjects, setFilteredProjects] = useState<Project[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState('')
  const [filterLanguage, setFilterLanguage] = useState('')
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid')
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [newProject, setNewProject] = useState({
    name: '',
    description: '',
    language: '',
    framework: '',
    is_public: false
  })
  const [isCreating, setIsCreating] = useState(false)

  const { user, apiService, setCurrentProject } = useStore()

  // Load projects
  useEffect(() => {
    loadProjects()
  }, [])

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
        is_public: newProject.is_public,
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

  // Render project card
  const renderProjectCard = (project: Project) => {
    const langConfig = getLanguageConfig(project.language)

    return (
      <Card
        key={project.id}
        variant="interactive"
        className="group hover:shadow-cyan-500/20 transition-all duration-300"
        onClick={() => handleProjectSelect(project)}
      >
        <CardHeader>
          <div className="flex items-start justify-between">
            <div className="flex items-center gap-3">
              <div
                className="w-10 h-10 rounded-lg flex items-center justify-center text-lg border border-gray-700"
                style={{ backgroundColor: `${langConfig.color}20`, borderColor: `${langConfig.color}40` }}
              >
                {langConfig.icon}
              </div>
              <div>
                <CardTitle className="text-lg">{project.name}</CardTitle>
                {project.description && (
                  <p className="text-sm text-gray-400 mt-1 line-clamp-2">{project.description}</p>
                )}
              </div>
            </div>

            <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
              <Button
                size="xs"
                variant="ghost"
                onClick={(e) => {
                  e.stopPropagation()
                  // Edit project
                }}
                icon={<Edit3 size={12} />}
              />
              <Button
                size="xs"
                variant="ghost"
                onClick={(e) => {
                  e.stopPropagation()
                  handleDeleteProject(project)
                }}
                icon={<Trash2 size={12} />}
              />
            </div>
          </div>
        </CardHeader>

        <CardContent>
          <div className="flex items-center gap-2 mb-3">
            <Badge variant="outline" size="xs">
              {langConfig.name}
            </Badge>
            {project.framework && (
              <Badge variant="outline" size="xs">
                {project.framework}
              </Badge>
            )}
            {project.is_public ? (
              <Badge variant="success" size="xs" icon={<Eye size={10} />}>
                Public
              </Badge>
            ) : (
              <Badge variant="neutral" size="xs" icon={<EyeOff size={10} />}>
                Private
              </Badge>
            )}
          </div>

          <div className="text-xs text-gray-400 space-y-1">
            <div className="flex items-center justify-between">
              <span className="flex items-center gap-1">
                <Clock size={12} />
                Updated {formatRelativeTime(project.updated_at)}
              </span>
              {project.owner && (
                <div className="flex items-center gap-1">
                  <Avatar
                    size="xs"
                    src={project.owner.avatar_url}
                    fallback={project.owner.username}
                  />
                  <span>{project.owner.username}</span>
                </div>
              )}
            </div>
          </div>
        </CardContent>

        <CardFooter>
          <div className="flex items-center justify-between w-full">
            <div className="flex items-center gap-2 text-xs text-gray-400">
              <span className="flex items-center gap-1">
                <GitBranch size={12} />
                main
              </span>
            </div>

            <div className="flex items-center gap-1">
              <Button
                size="xs"
                variant="ghost"
                onClick={(e) => {
                  e.stopPropagation()
                  // Run project
                }}
                icon={<Play size={12} />}
              />
              <Button
                size="xs"
                variant="primary"
                onClick={(e) => {
                  e.stopPropagation()
                  handleProjectSelect(project)
                }}
                icon={<Code size={12} />}
              >
                Open
              </Button>
            </div>
          </div>
        </CardFooter>
      </Card>
    )
  }

  // Render project row (list view)
  const renderProjectRow = (project: Project) => {
    const langConfig = getLanguageConfig(project.language)

    return (
      <div
        key={project.id}
        className="flex items-center gap-4 p-4 hover:bg-gray-800/50 rounded-lg cursor-pointer group transition-colors border border-transparent hover:border-gray-700"
        onClick={() => handleProjectSelect(project)}
      >
        <div
          className="w-8 h-8 rounded flex items-center justify-center text-sm border border-gray-700"
          style={{ backgroundColor: `${langConfig.color}20`, borderColor: `${langConfig.color}40` }}
        >
          {langConfig.icon}
        </div>

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h3 className="font-medium text-white">{project.name}</h3>
            <Badge variant="outline" size="xs">
              {langConfig.name}
            </Badge>
            {project.framework && (
              <Badge variant="outline" size="xs">
                {project.framework}
              </Badge>
            )}
            {project.is_public ? (
              <Badge variant="success" size="xs" icon={<Eye size={10} />}>
                Public
              </Badge>
            ) : (
              <Badge variant="neutral" size="xs" icon={<EyeOff size={10} />}>
                Private
              </Badge>
            )}
          </div>
          {project.description && (
            <p className="text-sm text-gray-400 truncate">{project.description}</p>
          )}
        </div>

        <div className="flex items-center gap-4 text-sm text-gray-400">
          <span>{formatRelativeTime(project.updated_at)}</span>
          {project.owner && (
            <div className="flex items-center gap-1">
              <Avatar
                size="xs"
                src={project.owner.avatar_url}
                fallback={project.owner.username}
              />
              <span>{project.owner.username}</span>
            </div>
          )}
        </div>

        <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          <Button
            size="xs"
            variant="primary"
            onClick={(e) => {
              e.stopPropagation()
              handleProjectSelect(project)
            }}
            icon={<Code size={12} />}
          >
            Open
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className={cn('space-y-6', className)}>
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Projects</h1>
          <p className="text-gray-400">Manage your development projects</p>
        </div>

        {showCreateButton && (
          <Button
            onClick={() => setShowCreateModal(true)}
            icon={<Plus size={16} />}
            variant="primary"
          >
            New Project
          </Button>
        )}
      </div>

      {/* Filters and Controls */}
      <Card variant="cyberpunk" padding="md">
        <div className="flex items-center gap-4 flex-wrap">
          <div className="flex-1 min-w-64">
            <Input
              placeholder="Search projects..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              leftIcon={<Search size={14} />}
              size="sm"
            />
          </div>

          <select
            value={filterLanguage}
            onChange={(e) => setFilterLanguage(e.target.value)}
            className="bg-gray-800 border border-gray-600 rounded px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
          >
            <option value="">All Languages</option>
            {LANGUAGE_CONFIGS.map(lang => (
              <option key={lang.id} value={lang.id}>
                {lang.name}
              </option>
            ))}
          </select>

          <div className="flex items-center gap-1 bg-gray-800 rounded-lg p-1">
            <Button
              size="xs"
              variant={viewMode === 'grid' ? 'primary' : 'ghost'}
              onClick={() => setViewMode('grid')}
              icon={<Grid size={14} />}
            />
            <Button
              size="xs"
              variant={viewMode === 'list' ? 'primary' : 'ghost'}
              onClick={() => setViewMode('list')}
              icon={<List size={14} />}
            />
          </div>
        </div>
      </Card>

      {/* Projects */}
      {isLoading ? (
        <div className="flex items-center justify-center py-16">
          <Loading variant="orb" color="cyberpunk" text="Loading projects..." />
        </div>
      ) : filteredProjects.length > 0 ? (
        <div className={cn(
          viewMode === 'grid'
            ? 'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4'
            : 'space-y-2'
        )}>
          {filteredProjects.map(project =>
            viewMode === 'grid' ? renderProjectCard(project) : renderProjectRow(project)
          )}
        </div>
      ) : (
        <div className="text-center py-16">
          <Code className="w-16 h-16 text-gray-600 mx-auto mb-4" />
          <h3 className="text-lg font-semibold text-gray-300 mb-2">
            {searchQuery || filterLanguage ? 'No matching projects' : 'No projects yet'}
          </h3>
          <p className="text-gray-400 mb-6">
            {searchQuery || filterLanguage
              ? 'Try adjusting your search or filters'
              : 'Create your first project to get started'
            }
          </p>
          {showCreateButton && (
            <Button
              onClick={() => setShowCreateModal(true)}
              icon={<Plus size={16} />}
              variant="primary"
            >
              Create Project
            </Button>
          )}
        </div>
      )}

      {/* Create Project Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 p-4">
          <Card variant="cyberpunk" className="w-full max-w-lg">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Plus className="w-5 h-5 text-cyan-400" />
                Create New Project
              </CardTitle>
            </CardHeader>

            <CardContent className="space-y-4">
              <Input
                label="Project Name"
                placeholder="My Awesome Project"
                value={newProject.name}
                onChange={(e) => setNewProject(prev => ({ ...prev, name: e.target.value }))}
                variant="cyberpunk"
              />

              <Input
                label="Description (optional)"
                placeholder="A brief description of your project"
                value={newProject.description}
                onChange={(e) => setNewProject(prev => ({ ...prev, description: e.target.value }))}
                variant="cyberpunk"
              />

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium text-gray-300 block mb-2">
                    Language
                  </label>
                  <select
                    value={newProject.language}
                    onChange={(e) => setNewProject(prev => ({
                      ...prev,
                      language: e.target.value,
                      framework: '' // Reset framework when language changes
                    }))}
                    className="w-full bg-gray-800 border border-gray-600 rounded-lg px-3 py-2 text-white focus:border-cyan-400 focus:outline-none"
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
                  <label className="text-sm font-medium text-gray-300 block mb-2">
                    Framework (optional)
                  </label>
                  <select
                    value={newProject.framework}
                    onChange={(e) => setNewProject(prev => ({ ...prev, framework: e.target.value }))}
                    className="w-full bg-gray-800 border border-gray-600 rounded-lg px-3 py-2 text-white focus:border-cyan-400 focus:outline-none"
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

              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={newProject.is_public}
                  onChange={(e) => setNewProject(prev => ({ ...prev, is_public: e.target.checked }))}
                  className="w-4 h-4 bg-gray-800 border border-gray-600 rounded focus:ring-cyan-400"
                />
                <span className="text-sm text-gray-300">Make project public</span>
              </label>
            </CardContent>

            <CardFooter>
              <div className="flex justify-end gap-2 w-full">
                <Button
                  variant="ghost"
                  onClick={() => setShowCreateModal(false)}
                  disabled={isCreating}
                >
                  Cancel
                </Button>
                <Button
                  variant="primary"
                  onClick={handleCreateProject}
                  loading={isCreating}
                  disabled={!newProject.name.trim() || !newProject.language}
                  icon={<Plus size={16} />}
                >
                  Create Project
                </Button>
              </div>
            </CardFooter>
          </Card>
        </div>
      )}
    </div>
  )
}

export default ProjectList