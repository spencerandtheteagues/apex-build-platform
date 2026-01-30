// APEX.BUILD Explore / Community Page
// Dark Demon Theme - Marketplace for projects, templates, and extensions

import React, { useState, useEffect } from 'react'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
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
  Heart
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
}

export const ExplorePage = () => {
  const [activeTab, setActiveTab] = useState<'trending' | 'new' | 'popular'>('trending')
  const [searchQuery, setSearchQuery] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [projects, setProjects] = useState<ProjectCard[]>([])

  const { createProject, setCurrentProject } = useStore()

  // Mock data for initial render
  useEffect(() => {
    setIsLoading(true)
    // Simulate API call
    setTimeout(() => {
      setProjects([
        {
          id: '1',
          title: 'Cyberpunk Portfolio Template',
          description: 'A futuristic, neon-styled portfolio website template with React and Three.js animations.',
          author: { name: 'NeonCoder', avatar: '' },
          stars: 1240,
          forks: 350,
          views: 15000,
          tags: ['React', 'Three.js', 'Portfolio', 'Cyberpunk'],
          updatedAt: '2 hours ago',
          verified: true
        },
        {
          id: '2',
          title: 'AI Chatbot Starter',
          description: 'Full-stack AI chat application boilerplate using OpenAI API, Node.js, and WebSocket.',
          author: { name: 'AIGuru', avatar: '' },
          stars: 890,
          forks: 210,
          views: 8900,
          tags: ['AI', 'Node.js', 'WebSocket', 'Chat'],
          updatedAt: '5 hours ago',
          verified: true
        },
        {
          id: '3',
          title: 'DeFi Dashboard',
          description: 'Cryptocurrency dashboard with real-time price tracking, charts, and wallet integration.',
          author: { name: 'CryptoKing', avatar: '' },
          stars: 750,
          forks: 120,
          views: 6200,
          tags: ['Web3', 'Crypto', 'Dashboard', 'React'],
          updatedAt: '1 day ago'
        },
        {
          id: '4',
          title: 'E-commerce API',
          description: 'Production-ready REST API for e-commerce with Stripe integration, inventory management, and auth.',
          author: { name: 'BackendPro', avatar: '' },
          stars: 620,
          forks: 180,
          views: 5400,
          tags: ['Go', 'API', 'Stripe', 'E-commerce'],
          updatedAt: '2 days ago',
          verified: true
        },
        {
          id: '5',
          title: 'Multiplayer Game Server',
          description: 'Scalable game server engine written in Rust with WebSocket and UDP support.',
          author: { name: 'Rustacean', avatar: '' },
          stars: 580,
          forks: 90,
          views: 4100,
          tags: ['Rust', 'GameDev', 'Server', 'UDP'],
          updatedAt: '3 days ago'
        },
        {
          id: '6',
          title: 'Mobile App Kit',
          description: 'Cross-platform mobile app starter kit using React Native and Expo with pre-built UI components.',
          author: { name: 'AppMaster', avatar: '' },
          stars: 450,
          forks: 130,
          views: 3800,
          tags: ['React Native', 'Mobile', 'iOS', 'Android'],
          updatedAt: '1 week ago'
        }
      ])
      setIsLoading(false)
    }, 1000)
  }, [activeTab])

  const handleFork = async (projectId: string) => {
    // Logic to fork project
    console.log(`Forking project ${projectId}`)
  }

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
            <Button variant="outline" className="border-gray-700 hover:bg-gray-800">
              <Rocket className="w-4 h-4 mr-2 text-purple-400" />
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
            <Button variant="ghost" className="border border-gray-800">
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
                      <Button size="xs" variant="ghost" className="h-7 text-gray-400 hover:text-white">
                        <Heart className="w-3.5 h-3.5" />
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
    </div>
  )
}

export default ExplorePage
