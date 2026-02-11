// APEX.BUILD Explore / Community Page
// Dark Demon Theme - Marketplace for projects, templates, and extensions

import React, { useState, useEffect } from 'react'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import { Project } from '@/types'
import apiService from '@/services/api'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Button,
  Badge,
  Avatar,
  Input,
  LoadingOverlay
} from '@/components/ui'
import {
  Search,
  Filter,
  TrendingUp,
  Clock,
  Star,
  GitFork,
  Download,
  Eye,
  User,
  Tag,
  Rocket,
  Code2,
  Box,
  Puzzle,
  Globe,
  Heart,
  Brain,
  X
} from 'lucide-react'

// Types
interface ProjectCard {
  id: string
  title: string
  description: string
  author: {
    name: string
    avatar: string
  }
  stars: number
  forks: number
  views: number
  tags: string[]
  updatedAt: string
  thumbnail?: string
  verified?: boolean
  isStarred?: boolean
}

export const ExplorePage = () => {
  const [activeTab, setActiveTab] = useState<'trending' | 'new' | 'popular'>('trending')
  const [searchQuery, setSearchQuery] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [projects, setProjects] = useState<ProjectCard[]>([])
  const [showFilters, setShowFilters] = useState(false)
  const [likedProjects, setLikedProjects] = useState<Set<string>>(new Set())
  const [showPublishModal, setShowPublishModal] = useState(false)
  const [publishLoading, setPublishLoading] = useState(false)
  const [publishError, setPublishError] = useState<string | null>(null)
  const [publishSuccess, setPublishSuccess] = useState<string | null>(null)
  const [publishSearch, setPublishSearch] = useState('')
  const [userProjects, setUserProjects] = useState<Project[]>([])

  const { setCurrentProject } = useStore()

  const handleLike = async (projectId: string) => {
    const numericId = Number(projectId)
    if (!Number.isFinite(numericId)) return

    const isLiked = likedProjects.has(projectId)

    setLikedProjects(prev => {
      const next = new Set(prev)
      if (isLiked) {
        next.delete(projectId)
      } else {
        next.add(projectId)
      }
      return next
    })

    setProjects(prev => prev.map(p =>
      p.id === projectId
        ? { ...p, stars: Math.max(0, p.stars + (isLiked ? -1 : 1)), isStarred: !isLiked }
        : p
    ))

    try {
      if (isLiked) {
        await apiService.unstarProject(numericId)
      } else {
        await apiService.starProject(numericId)
      }
    } catch (error) {
      console.error('Failed to update star:', error)
      setLikedProjects(prev => {
        const next = new Set(prev)
        if (isLiked) {
          next.add(projectId)
        } else {
          next.delete(projectId)
        }
        return next
      })
      setProjects(prev => prev.map(p =>
        p.id === projectId
          ? { ...p, stars: Math.max(0, p.stars + (isLiked ? 1 : -1)), isStarred: isLiked }
          : p
      ))
    }
  }

  const handlePublish = async () => {
    setPublishError(null)
    setPublishSuccess(null)
    setPublishSearch('')
    setShowPublishModal(true)
    setPublishLoading(true)
    try {
      const projectsData = await apiService.getProjects()
      setUserProjects(projectsData)
    } catch (error) {
      console.error('Failed to load projects:', error)
      setPublishError('Failed to load your projects.')
    } finally {
      setPublishLoading(false)
    }
  }

  // Fetch projects from API
  useEffect(() => {
    const fetchProjects = async () => {
      setIsLoading(true)
      try {
        // Map tab to sort order
        let sort: 'trending' | 'recent' | 'stars' | 'forks' = 'trending'
        if (activeTab === 'new') sort = 'recent'
        if (activeTab === 'popular') sort = 'stars'

        const response = await apiService.searchPublicProjects({
          q: searchQuery,
          sort,
          limit: 12
        })
        
        // Transform API response to UI model if necessary, or use directly if compatible
        // Assuming API returns compatible ProjectWithStats objects
        const mapped = response.projects.map(p => ({
          id: String(p.id),
          title: p.name,
          description: p.description || 'No description provided',
          author: {
            name: p.owner_username || 'Unknown',
            avatar: p.owner_avatar_url || ''
          },
          stars: p.stats?.star_count || 0,
          forks: p.stats?.fork_count || 0,
          views: p.stats?.view_count || 0,
          tags: p.topics || [p.language].filter(Boolean) as string[],
          updatedAt: new Date(p.updated_at).toLocaleDateString(),
          verified: p.is_verified || false,
          isStarred: p.is_starred || false
        }))

        setProjects(mapped)
        setLikedProjects(new Set(mapped.filter(p => p.isStarred).map(p => p.id)))
      } catch (error) {
        console.error('Failed to fetch projects:', error)
      } finally {
        setIsLoading(false)
      }
    }

    // Debounce search
    const timer = setTimeout(() => {
      fetchProjects()
    }, 300)

    return () => clearTimeout(timer)
  }, [activeTab, searchQuery])

  const handleFork = async (projectId: string) => {
    try {
      const forked = await apiService.forkProject(Number(projectId))
      if (forked) {
        setCurrentProject(forked.project)
        alert('Project forked successfully! You can now find it in your projects.')
      }
    } catch (error) {
      console.error('Failed to fork project:', error)
      alert('Failed to fork project. Please try again.')
    }
  }

  const handleTogglePublish = async (project: Project) => {
    setPublishError(null)
    setPublishSuccess(null)
    setPublishLoading(true)
    try {
      const updated = await apiService.updateProject(project.id, {
        is_public: !project.is_public
      })
      setUserProjects(prev => prev.map(p => (p.id === updated.id ? updated : p)))
      setPublishSuccess(updated.is_public ? 'Project published successfully.' : 'Project unpublished successfully.')
    } catch (error) {
      console.error('Failed to update project visibility:', error)
      setPublishError('Failed to update project visibility.')
    } finally {
      setPublishLoading(false)
    }
  }

  const filteredUserProjects = userProjects.filter(project =>
    project.name.toLowerCase().includes(publishSearch.toLowerCase()) ||
    project.description?.toLowerCase().includes(publishSearch.toLowerCase())
  )

  return (
    <div className="h-full overflow-y-auto bg-black text-white p-6 pb-20">
      {/* Background effects */}
      <div className="fixed inset-0 pointer-events-none">
        <div className="absolute top-0 left-0 w-full h-96 bg-gradient-to-b from-purple-900/10 to-transparent" />
        <div className="absolute top-20 right-20 w-64 h-64 bg-purple-900/5 rounded-full blur-3xl" />
        <div className="absolute bottom-20 left-20 w-64 h-64 bg-blue-900/5 rounded-full blur-3xl" />
      </div>

      <div className="relative z-10 max-w-7xl mx-auto">
        {/* Header */}
        <div className="flex flex-col md:flex-row md:items-end justify-between gap-6 mb-12">
          <div>
            <div className="flex items-center gap-3 mb-2">
              <div className="p-2 bg-gradient-to-br from-purple-600 to-blue-600 rounded-lg shadow-lg shadow-purple-900/30">
                <Globe className="w-6 h-6 text-white" />
              </div>
              <h1 className="text-3xl font-bold bg-gradient-to-r from-purple-400 via-pink-400 to-blue-400 bg-clip-text text-transparent">
                Explore Community
              </h1>
            </div>
            <p className="text-gray-400 max-w-xl">
              Discover, fork, and contribute to thousands of open-source projects built on APEX.BUILD.
            </p>
          </div>

          <div className="flex gap-2">
            <Button variant="outline" className="border-gray-700 hover:bg-gray-800" onClick={handlePublish}>
              <img src="/logo.png" alt="APEX" className="w-5 h-5 mr-2 object-contain" />
              Publish Project
            </Button>
          </div>
        </div>

        {/* Search and Filter */}
        <div className="flex flex-col md:flex-row gap-4 mb-8">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-500" />
            <input
              type="text"
              placeholder="Search projects, templates, libraries..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full bg-gray-900/50 border border-gray-800 rounded-lg pl-10 pr-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-purple-500 transition-colors"
            />
          </div>
          <div className="flex gap-2">
            <Button
              variant={activeTab === 'trending' ? 'primary' : 'ghost'}
              className={cn(activeTab === 'trending' ? 'bg-purple-900/20 text-purple-400 border border-purple-900/50' : '')}
              onClick={() => setActiveTab('trending')}
            >
              <TrendingUp className="w-4 h-4 mr-2" />
              Trending
            </Button>
            <Button
              variant={activeTab === 'new' ? 'primary' : 'ghost'}
              className={cn(activeTab === 'new' ? 'bg-purple-900/20 text-purple-400 border border-purple-900/50' : '')}
              onClick={() => setActiveTab('new')}
            >
              <Clock className="w-4 h-4 mr-2" />
              New
            </Button>
            <Button
              variant={activeTab === 'popular' ? 'primary' : 'ghost'}
              className={cn(activeTab === 'popular' ? 'bg-purple-900/20 text-purple-400 border border-purple-900/50' : '')}
              onClick={() => setActiveTab('popular')}
            >
              <Star className="w-4 h-4 mr-2" />
              Popular
            </Button>
            <Button variant="ghost" className="border border-gray-800" onClick={() => setShowFilters(!showFilters)}>
              <Filter className="w-4 h-4 mr-2" />
              Filters
            </Button>
          </div>
        </div>

        {/* Content Grid */}
        {isLoading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {[1, 2, 3, 4, 5, 6].map((i) => (
              <Card key={i} variant="cyberpunk" className="h-64 animate-pulse">
                <div className="h-full bg-gray-900/50" />
              </Card>
            ))}
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {projects.map((project) => (
              <Card
                key={project.id}
                variant="cyberpunk"
                className="group border border-gray-800 hover:border-purple-500/50 transition-all duration-300 hover:transform hover:-translate-y-1"
              >
                <CardContent className="p-5 h-full flex flex-col">
                  {/* Project Header */}
                  <div className="flex items-start justify-between mb-3">
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-gray-800 to-gray-900 flex items-center justify-center group-hover:from-purple-900/20 group-hover:to-blue-900/20 transition-colors">
                        {project.tags.includes('GameDev') ? (
                          <Puzzle className="w-5 h-5 text-gray-400 group-hover:text-purple-400" />
                        ) : project.tags.includes('AI') ? (
                          <Brain className="w-5 h-5 text-gray-400 group-hover:text-purple-400" />
                        ) : (
                          <Box className="w-5 h-5 text-gray-400 group-hover:text-purple-400" />
                        )}
                      </div>
                      <div>
                        <h3 className="font-bold text-lg text-white group-hover:text-purple-400 transition-colors line-clamp-1">
                          {project.title}
                        </h3>
                        <div className="flex items-center gap-1 text-xs text-gray-500">
                          <span>by</span>
                          <span className="text-gray-400 hover:text-white cursor-pointer">{project.author.name}</span>
                          {project.verified && (
                            <Badge variant="outline" size="xs" className="ml-1 h-4 px-1 border-blue-900/50 text-blue-400 bg-blue-900/10">
                              ✓
                            </Badge>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Description */}
                  <p className="text-sm text-gray-400 mb-4 line-clamp-2 flex-1">
                    {project.description}
                  </p>

                  {/* Tags */}
                  <div className="flex flex-wrap gap-1.5 mb-4">
                    {project.tags.slice(0, 3).map((tag) => (
                      <Badge key={tag} variant="outline" className="bg-gray-900/50 border-gray-700 text-gray-400">
                        {tag}
                      </Badge>
                    ))}
                    {project.tags.length > 3 && (
                      <Badge variant="outline" className="bg-gray-900/50 border-gray-700 text-gray-500">
                        +{project.tags.length - 3}
                      </Badge>
                    )}
                  </div>

                  {/* Stats & Actions */}
                  <div className="flex items-center justify-between pt-4 border-t border-gray-800">
                    <div className="flex items-center gap-3 text-xs text-gray-500">
                      <div className="flex items-center gap-1">
                        <Star className="w-3.5 h-3.5" />
                        <span>{project.stars}</span>
                      </div>
                      <div className="flex items-center gap-1">
                        <GitFork className="w-3.5 h-3.5" />
                        <span>{project.forks}</span>
                      </div>
                    </div>

                    <div className="flex items-center gap-2">
                      <Button size="xs" variant="ghost" className="h-7 text-gray-400 hover:text-white" onClick={() => handleLike(project.id)}>
                        <Heart className={cn("w-3.5 h-3.5", likedProjects.has(project.id) && "fill-red-500 text-red-500")} />
                      </Button>
                      <Button
                        size="xs"
                        className="h-7 bg-gray-800 hover:bg-gray-700 text-white border border-gray-700"
                        onClick={() => handleFork(project.id)}
                      >
                        <GitFork className="w-3.5 h-3.5 mr-1.5" />
                        Fork
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>

      {showPublishModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
          <Card variant="cyberpunk" padding="lg" className="w-full max-w-2xl">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
              <CardTitle className="text-white">Publish a Project</CardTitle>
              <Button
                variant="ghost"
                size="xs"
                onClick={() => {
                  setShowPublishModal(false)
                  setPublishSearch('')
                  setPublishError(null)
                  setPublishSuccess(null)
                }}
              >
                <X className="w-4 h-4" />
              </Button>
            </CardHeader>

            <CardContent className="space-y-4">
              <div className="flex items-center gap-2">
                <Search className="w-4 h-4 text-gray-500" />
                <input
                  type="text"
                  placeholder="Search your projects..."
                  value={publishSearch}
                  onChange={(e) => setPublishSearch(e.target.value)}
                  className="flex-1 bg-gray-900/50 border border-gray-800 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:outline-none focus:border-purple-500 transition-colors"
                />
              </div>

              {publishError && (
                <div className="text-sm text-red-400 bg-red-500/10 border border-red-500/30 rounded-lg px-3 py-2">
                  {publishError}
                </div>
              )}
              {publishSuccess && (
                <div className="text-sm text-green-400 bg-green-500/10 border border-green-500/30 rounded-lg px-3 py-2">
                  {publishSuccess}
                </div>
              )}

              {publishLoading && userProjects.length === 0 ? (
                <div className="flex items-center justify-center py-8 text-gray-400">
                  Loading projects...
                </div>
              ) : filteredUserProjects.length === 0 ? (
                <div className="text-center py-8 text-gray-500">
                  No projects found.
                </div>
              ) : (
                <div className="max-h-[50vh] overflow-y-auto space-y-3 pr-1">
                  {filteredUserProjects.map(project => (
                    <div
                      key={project.id}
                      className="flex items-center justify-between gap-4 bg-gray-900/60 border border-gray-800 rounded-lg p-3"
                    >
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="text-white font-medium truncate">{project.name}</span>
                          {project.is_public ? (
                            <Badge variant="success" size="xs">
                              Public
                            </Badge>
                          ) : (
                            <Badge variant="neutral" size="xs">
                              Private
                            </Badge>
                          )}
                        </div>
                        {project.description && (
                          <p className="text-xs text-gray-500 truncate">{project.description}</p>
                        )}
                      </div>
                      <Button
                        size="xs"
                        variant={project.is_public ? 'ghost' : 'primary'}
                        onClick={() => handleTogglePublish(project)}
                        disabled={publishLoading}
                      >
                        {project.is_public ? 'Unpublish' : 'Publish'}
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  )
}

export default ExplorePage
