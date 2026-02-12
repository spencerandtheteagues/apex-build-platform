// APEX.BUILD API Service
// Handles all communication with the Go backend

import axios, { AxiosInstance, AxiosRequestConfig, AxiosResponse } from 'axios'
import {
  User,
  Project,
  File,
  AIRequest,
  AIUsage,
  Execution,
  ExecutionResult,
  LoginRequest,
  RegisterRequest,
  AuthResponse,
  TokenResponse,
  ApiResponse,
  PaginatedResponse,
  AICapability,
  AIProvider,
  ExploreData,
  ProjectWithStats,
  ProjectComment,
  ProjectCategory,
  UserPublicProfile,
  UserFollowInfo,
  CodeComment,
  CodeCommentThread,
  CreateCommentRequest,
  UpdateCommentRequest,
  FileVersion,
  VersionDiff,
  ManagedDatabase,
  CreateDatabaseRequest,
  DatabaseMetrics,
  TableInfo,
  ColumnInfo,
  CompletionRequest,
  CompletionResponse,
  CompletionItem,
  CompletionStats,
} from '@/types'

// Get API URL from environment or use default
const getApiUrl = (): string => {
  // Check for Vite environment variable
  if (import.meta.env.VITE_API_URL) {
    return import.meta.env.VITE_API_URL
  }

  // Production detection - if running on Render, Firebase, or production domain
  const hostname = typeof window !== 'undefined' ? window.location.hostname : ''
  if (hostname.includes('onrender.com') || hostname.includes('apex.build') || hostname.includes('web.app') || hostname.includes('firebaseapp.com') || hostname === 'apex-frontend-gigq.onrender.com') {
    return 'https://apex-backend-5ypy.onrender.com/api/v1'
  }

  // Fallback to relative URL for development
  return '/api/v1'
}

export class ApiService {
  public client: AxiosInstance
  private baseURL: string
  private refreshPromise: Promise<TokenResponse> | null = null

  constructor(baseURL: string = getApiUrl()) {
    this.baseURL = baseURL
    this.client = axios.create({
      baseURL,
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    })

    this.setupInterceptors()
  }

  private setupInterceptors() {
    // Request interceptor - Add auth token
    this.client.interceptors.request.use(
      (config) => {
        const token = this.getAuthToken()
        if (token) {
          config.headers.Authorization = `Bearer ${token}`
        }
        return config
      },
      (error) => Promise.reject(error)
    )

    // Response interceptor - Handle errors and token refresh
    this.client.interceptors.response.use(
      (response) => response,
      async (error) => {
        const originalRequest = error.config

        if (error.response?.status === 401 && !originalRequest._retry) {
          originalRequest._retry = true

          try {
            await this.refreshToken()
            const token = this.getAuthToken()
            if (token) {
              originalRequest.headers.Authorization = `Bearer ${token}`
            }
            return this.client(originalRequest)
          } catch (refreshError) {
            this.clearAuth()
            window.location.href = '/login'
            return Promise.reject(refreshError)
          }
        }

        return Promise.reject(error)
      }
    )
  }

  // Auth token management
  private getAuthToken(): string | null {
    return localStorage.getItem('apex_access_token')
  }

  private getRefreshToken(): string | null {
    return localStorage.getItem('apex_refresh_token')
  }

  private setTokens(tokens: TokenResponse): void {
    localStorage.setItem('apex_access_token', tokens.access_token)
    localStorage.setItem('apex_refresh_token', tokens.refresh_token)
    localStorage.setItem('apex_token_expires', tokens.access_token_expires_at)
  }

  private clearAuth(): void {
    localStorage.removeItem('apex_access_token')
    localStorage.removeItem('apex_refresh_token')
    localStorage.removeItem('apex_token_expires')
    localStorage.removeItem('apex_user')
  }

  // Generic HTTP methods for components that need direct access
  public async get<T = any>(url: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.client.get<T>(url, config)
  }

  public async post<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.client.post<T>(url, data, config)
  }

  public async put<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.client.put<T>(url, data, config)
  }

  public async delete<T = any>(url: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.client.delete<T>(url, config)
  }

  // Health check
  async health(): Promise<any> {
    const response = await this.client.get('/health')
    return response.data
  }

  // Authentication endpoints
  async register(data: RegisterRequest): Promise<AuthResponse> {
    const response = await this.client.post<AuthResponse>('/auth/register', data)
    if (response.data.tokens) {
      this.setTokens(response.data.tokens)
      if (response.data.user) {
        localStorage.setItem('apex_user', JSON.stringify(response.data.user))
      }
    }
    return response.data
  }

  async login(data: LoginRequest): Promise<AuthResponse> {
    const response = await this.client.post<AuthResponse>('/auth/login', data)
    if (response.data.tokens) {
      this.setTokens(response.data.tokens)
      if (response.data.user) {
        localStorage.setItem('apex_user', JSON.stringify(response.data.user))
      }
    }
    return response.data
  }

  async refreshToken(): Promise<TokenResponse> {
    // Prevent multiple concurrent refresh attempts (mutex pattern)
    if (this.refreshPromise) {
      return this.refreshPromise
    }

    this.refreshPromise = this.doRefreshToken().finally(() => {
      this.refreshPromise = null
    })

    return this.refreshPromise
  }

  private async doRefreshToken(): Promise<TokenResponse> {
    const refreshToken = this.getRefreshToken()
    if (!refreshToken) {
      throw new Error('No refresh token available')
    }

    const response = await this.client.post<ApiResponse<TokenResponse>>('/auth/refresh', {
      refresh_token: refreshToken,
    })

    if (response.data.data) {
      this.setTokens(response.data.data)
      return response.data.data
    }

    throw new Error('Failed to refresh token')
  }

  async logout(): Promise<void> {
    try {
      await this.client.post('/auth/logout')
    } finally {
      this.clearAuth()
    }
  }

  // User endpoints
  async getUserProfile(): Promise<User> {
    const response = await this.client.get<ApiResponse<{ user: User }>>('/user/profile')
    return response.data.data!.user
  }

  async updateUserProfile(data: Partial<User>): Promise<User> {
    const response = await this.client.put<ApiResponse<{ user: User }>>('/user/profile', data)
    return response.data.data!.user
  }

  // Project endpoints
  async createProject(data: {
    name: string
    description?: string
    language: string
    framework?: string
    is_public?: boolean
    environment?: Record<string, any>
  }): Promise<Project> {
    const response = await this.client.post<{ message?: string; project: Project }>('/projects', data)
    // Backend returns { message, project } directly, not wrapped in { data: { project } }
    return response.data.project
  }

  async getProjects(): Promise<Project[]> {
    const response = await this.client.get<{ projects: Project[] }>('/projects')
    // Backend returns { projects } directly
    return response.data.projects || []
  }

  async getProject(id: number): Promise<Project> {
    const response = await this.client.get<{ project: Project }>(`/projects/${id}`)
    // Backend returns { project } directly
    return response.data.project
  }

  async updateProject(id: number, data: Partial<Project>): Promise<Project> {
    const response = await this.client.put<{ project?: Project; data?: { project?: Project } }>(`/projects/${id}`, data)
    return response.data.project || response.data.data?.project || (response.data as unknown as Project)
  }

  async deleteProject(id: number): Promise<void> {
    await this.client.delete(`/projects/${id}`)
  }

  // File endpoints
  async createFile(projectId: number, data: {
    path: string
    name: string
    type: 'file' | 'directory'
    content?: string
    mime_type?: string
  }): Promise<File> {
    const response = await this.client.post<{ file?: File; data?: File | { file?: File } }>(
      `/projects/${projectId}/files`,
      data
    )
    const dataValue = response.data.data as any
    return response.data.file || dataValue?.file || dataValue || (response.data as unknown as File)
  }

  async getFiles(projectId: number): Promise<File[]> {
    const response = await this.client.get<{ files?: File[]; data?: File[] }>(
      `/projects/${projectId}/files?include_content=true`
    )
    return response.data.files || response.data.data || []
  }

  async getFile(id: number): Promise<File> {
    const response = await this.client.get<{ file?: File; data?: File }>(`/files/${id}`)
    return response.data.file || response.data.data || (response.data as unknown as File)
  }

  async updateFile(id: number, data: { content?: string; name?: string; path?: string }): Promise<File> {
    await this.client.put(`/files/${id}`, data)
    return this.getFile(id)
  }

  async deleteFile(id: number): Promise<void> {
    await this.client.delete(`/files/${id}`)
  }

  // AI endpoints
  async generateAI(data: {
    capability: AICapability
    prompt: string
    code?: string
    language?: string
    context?: Record<string, any>
    max_tokens?: number
    temperature?: number
    project_id?: string
  }): Promise<{
    request_id: string
    provider: AIProvider
    content: string
    usage?: {
      prompt_tokens: number
      completion_tokens: number
      total_tokens: number
      cost: number
    }
    duration: number
    created_at: string
  }> {
    const response = await this.client.post('/ai/generate', data)
    return response.data
  }

  async getAIUsage(): Promise<AIUsage> {
    const response = await this.client.get<AIUsage>('/ai/usage')
    return response.data
  }

  async getAIHistory(limit: number = 50, offset: number = 0): Promise<AIRequest[]> {
    const response = await this.client.get<ApiResponse<{ requests: AIRequest[] }>>(
      `/ai/history?limit=${limit}&offset=${offset}`
    )
    return response.data.data!.requests
  }

  async rateAIResponse(requestId: string, rating: number, feedback?: string): Promise<void> {
    await this.client.post(`/ai/rate/${requestId}`, { rating, feedback })
  }

  // Build endpoints (Agent Orchestration System)
  async startBuild(data: {
    description: string
    mode: 'fast' | 'full'
    power_mode?: 'fast' | 'balanced' | 'max'
    provider_mode?: 'platform' | 'byok'
    tech_stack?: {
      frontend?: string
      backend?: string
      database?: string
      styling?: string
      extras?: string[]
    }
  }): Promise<{
    build_id: string
    websocket_url: string
    status: string
  }> {
    const response = await this.client.post('/build/start', data)
    return response.data
  }

  async getBuildStatus(buildId: string): Promise<any> {
    const response = await this.client.get(`/build/${buildId}/status`)
    return response.data
  }

  async getBuildDetails(buildId: string): Promise<any> {
    const response = await this.client.get(`/build/${buildId}`)
    return response.data
  }

  async sendBuildMessage(buildId: string, message: string): Promise<void> {
    await this.client.post(`/build/${buildId}/message`, { content: message })
  }

  async getBuildCheckpoints(buildId: string): Promise<any[]> {
    const response = await this.client.get(`/build/${buildId}/checkpoints`)
    return response.data.checkpoints || []
  }

  async rollbackBuild(buildId: string, checkpointId: string): Promise<void> {
    await this.client.post(`/build/${buildId}/rollback/${checkpointId}`)
  }

  async getBuildAgents(buildId: string): Promise<any[]> {
    const response = await this.client.get(`/build/${buildId}/agents`)
    return response.data.agents || []
  }

  async getBuildTasks(buildId: string): Promise<any[]> {
    const response = await this.client.get(`/build/${buildId}/tasks`)
    return response.data.tasks || []
  }

  async getBuildFiles(buildId: string): Promise<any[]> {
    const response = await this.client.get(`/build/${buildId}/files`)
    return response.data.files || []
  }

  async cancelBuild(buildId: string): Promise<void> {
    await this.client.post(`/build/${buildId}/cancel`)
  }

  // Build history endpoints
  async listBuilds(page = 1, limit = 20): Promise<{
    builds: CompletedBuildSummary[]
    total: number
    page: number
    limit: number
  }> {
    const response = await this.client.get('/builds', { params: { page, limit } })
    return response.data
  }

  async getCompletedBuild(buildId: string): Promise<CompletedBuildDetail> {
    const response = await this.client.get(`/builds/${buildId}`)
    return response.data
  }

  // Code execution endpoints
  async executeCode(data: {
    code: string
    language: string
    stdin?: string
    timeout?: number
    env?: Record<string, string>
    project_id?: number
  }): Promise<ExecutionResult> {
    const response = await this.client.post<{ data?: ExecutionResult }>('/execute', data)
    return response.data.data || (response.data as unknown as ExecutionResult)
  }

  async executeProject(data: {
    project_id: number
    command?: string
    env?: Record<string, string>
    timeout?: number
  }): Promise<ExecutionResult> {
    const response = await this.client.post<{ data?: ExecutionResult }>(
      '/execute/project',
      data
    )
    return response.data.data || (response.data as unknown as ExecutionResult)
  }

  async getExecution(executionId: string): Promise<Execution> {
    const response = await this.client.get<{ data?: Execution }>(`/execute/${executionId}`)
    return response.data.data || (response.data as unknown as Execution)
  }

  async getExecutionHistory(
    projectId: number,
    limit: number = 50,
    offset: number = 0
  ): Promise<Execution[]> {
    const page = Math.max(1, Math.floor(offset / limit) + 1)
    const response = await this.client.get<{ data?: Execution[] }>(
      `/execute/history?project_id=${projectId}&limit=${limit}&page=${page}`
    )
    return response.data.data || (response.data as unknown as Execution[]) || []
  }

  async stopExecution(executionId: string): Promise<void> {
    await this.client.post(`/execute/${executionId}/stop`)
  }

  // Collaboration endpoints
  async joinCollabRoom(projectId: number): Promise<{
    room_id: string
    project_id?: string
    users?: User[]
    user_count?: number
  }> {
    const response = await this.client.post<{ data?: any }>(`/collab/join/${projectId}`)
    return response.data.data || response.data
  }

  async leaveCollabRoom(roomId: string): Promise<void> {
    await this.client.post(`/collab/leave/${roomId}`)
  }

  async getCollabUsers(roomId: string): Promise<User[]> {
    const response = await this.client.get<{ data?: { users?: User[] } }>(`/collab/users/${roomId}`)
    return response.data.data?.users || []
  }

  // System information
  async getSystemInfo(): Promise<{
    version: string
    ai_providers: Record<AIProvider, boolean>
    features: string[]
    performance: {
      avg_response_time: number
      uptime: number
      requests_per_second: number
    }
  }> {
    const response = await this.client.get('/system/info')
    return response.data
  }

  // Project export/download functionality
  async downloadProjectAsZip(projectId: number): Promise<Blob> {
    const response = await this.client.get(`/projects/${projectId}/download`, {
      responseType: 'blob'
    })
    return response.data
  }

  // Build export/download functionality
  async downloadBuildAsZip(buildId: string): Promise<Blob> {
    const response = await this.client.get(`/builds/${buildId}/download`, {
      responseType: 'blob'
    })
    return response.data
  }

  // Download and save project as zip file
  async exportProject(projectId: number, projectName: string): Promise<void> {
    try {
      const blob = await this.downloadProjectAsZip(projectId)
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${projectName.replace(/[^a-z0-9]/gi, '_')}.zip`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      window.URL.revokeObjectURL(url)
    } catch (error) {
      console.error('Failed to export project:', error)
      throw error
    }
  }

  // ========== GITHUB EXPORT ENDPOINTS ==========

  // Export project to a new GitHub repository
  async exportToGitHub(data: {
    project_id: number
    repo_name: string
    description?: string
    is_private?: boolean
    token: string
  }): Promise<{
    success: boolean
    data?: {
      repo_url: string
      repo_owner: string
      repo_name: string
      commit_sha: string
      branch: string
      file_count: number
    }
    error?: string
    message?: string
  }> {
    const response = await this.client.post('/git/export', data)
    return response.data
  }

  // Check if project has been exported to GitHub
  async getExportStatus(projectId: number): Promise<{
    success: boolean
    exported: boolean
    repository: {
      remote_url: string
      repo_owner: string
      repo_name: string
      branch: string
      is_connected: boolean
    } | null
  }> {
    const response = await this.client.get(`/git/export/status/${projectId}`)
    return response.data
  }

  // ========== GIT INTEGRATION ENDPOINTS ==========

  async getGitRepo(projectId: number): Promise<{
    success: boolean
    repository: {
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
  }> {
    const response = await this.client.get(`/git/repo/${projectId}`)
    return response.data
  }

  async getGitBranches(projectId: number): Promise<{
    success: boolean
    branches: Array<{ name: string; sha: string; is_default: boolean; protected: boolean }>
  }> {
    const response = await this.client.get(`/git/branches/${projectId}`)
    return response.data
  }

  async getGitCommits(projectId: number, branch?: string, limit: number = 20): Promise<{
    success: boolean
    commits: Array<{ sha: string; message: string; author: string; email: string; timestamp: string }>
  }> {
    const params: Record<string, any> = { limit }
    if (branch) params.branch = branch
    const response = await this.client.get(`/git/commits/${projectId}`, { params })
    return response.data
  }

  async getGitStatus(projectId: number): Promise<{
    success: boolean
    staged: Array<{ path: string; status: string; staged: boolean }>
    unstaged: Array<{ path: string; status: string; staged: boolean }>
  }> {
    const response = await this.client.get(`/git/status/${projectId}`)
    return response.data
  }

  async gitCommit(projectId: number, message: string, files: string[]): Promise<{
    success: boolean
    commit: any
  }> {
    const response = await this.client.post('/git/commit', {
      project_id: projectId,
      message,
      files,
    })
    return response.data
  }

  async gitPush(projectId: number): Promise<{ success: boolean; message?: string }> {
    const response = await this.client.post('/git/push', { project_id: projectId })
    return response.data
  }

  async gitPull(projectId: number): Promise<{ success: boolean; message?: string }> {
    const response = await this.client.post('/git/pull', { project_id: projectId })
    return response.data
  }

  async gitCreateBranch(projectId: number, branchName: string, baseBranch?: string): Promise<{
    success: boolean
    branch: any
  }> {
    const response = await this.client.post('/git/branch', {
      project_id: projectId,
      branch_name: branchName,
      base_branch: baseBranch,
    })
    return response.data
  }

  async gitSwitchBranch(projectId: number, branchName: string): Promise<{
    success: boolean
    branch: string
  }> {
    const response = await this.client.post('/git/checkout', {
      project_id: projectId,
      branch_name: branchName,
    })
    return response.data
  }

  // Utility methods
  isAuthenticated(): boolean {
    const token = this.getAuthToken()
    const expires = localStorage.getItem('apex_token_expires')

    if (!token || !expires) {
      return false
    }

    // Check if token is expired
    const expiresAt = new Date(expires)
    const now = new Date()

    return now < expiresAt
  }

  getCurrentUser(): User | null {
    const userStr = localStorage.getItem('apex_user')
    if (!userStr) return null

    try {
      return JSON.parse(userStr)
    } catch {
      return null
    }
  }

  // File upload (for larger files or binary content)
  async uploadFile(
    projectId: number,
    file: Blob,
    path: string,
    name: string,
    onProgress?: (progress: number) => void
  ): Promise<File> {
    const formData = new FormData()
    formData.append('file', file)
    formData.append('path', path)
    formData.append('name', name)

    const response = await this.client.post<ApiResponse<{ file: File }>>(
      `/projects/${projectId}/upload`,
      formData,
      {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
        onUploadProgress: (progressEvent) => {
          if (onProgress && progressEvent.total) {
            const progress = (progressEvent.loaded / progressEvent.total) * 100
            onProgress(progress)
          }
        },
      }
    )

    return response.data.data!.file
  }

  // Batch operations
  async batchUpdateFiles(
    updates: Array<{ id: number; content: string }>
  ): Promise<{ success: number; failed: number; errors: any[] }> {
    const response = await this.client.post('/files/batch-update', { updates })
    return response.data
  }

  // Search
  async searchProjects(query: string): Promise<Project[]> {
    const response = await this.client.get<ApiResponse<{ projects: Project[] }>>(
      `/search/projects?q=${encodeURIComponent(query)}`
    )
    return response.data.data!.projects
  }

  async searchInFiles(
    projectId: number,
    query: string,
    options?: { case_sensitive?: boolean; regex?: boolean }
  ): Promise<Array<{
    file: File
    matches: Array<{ line: number; content: string; start: number; end: number }>
  }>> {
    const response = await this.client.post<{
      success: boolean
      results?: {
        files?: Array<{
          file_id: number
          file_name: string
          file_path: string
          language: string
          matches: Array<{
            line_number: number
            column_start: number
            column_end: number
            content: string
          }>
        }>
      }
    }>('/search', {
      query,
      project_id: projectId,
      case_sensitive: options?.case_sensitive,
      use_regex: options?.regex,
      include_content: true,
      context_lines: 0,
      search_type: 'content',
    })

    const files = response.data.results?.files || []
    return files.map((result) => ({
      file: {
        id: result.file_id,
        project_id: projectId,
        path: result.file_path,
        name: result.file_name || result.file_path.split('/').pop() || result.file_path,
        type: 'file',
        content: '' as string,
        size: 0,
        version: 1,
        is_locked: false,
        created_at: '',
        updated_at: '',
      },
      matches: result.matches.map(match => ({
        line: match.line_number,
        content: match.content,
        start: match.column_start,
        end: match.column_end,
      }))
    }))
  }

  // ========== COMMUNITY/SHARING MARKETPLACE ENDPOINTS ==========

  // Explore page data
  async getExplore(): Promise<ExploreData> {
    const response = await this.client.get<ExploreData>('/explore')
    return response.data
  }

  // Search public projects
  async searchPublicProjects(params: {
    q?: string
    category?: string
    language?: string
    sort?: 'trending' | 'recent' | 'stars' | 'forks'
    page?: number
    limit?: number
  }): Promise<{ projects: ProjectWithStats[]; pagination: any }> {
    const queryParams = new URLSearchParams()
    if (params.q) queryParams.append('q', params.q)
    if (params.category) queryParams.append('category', params.category)
    if (params.language) queryParams.append('language', params.language)
    if (params.sort) queryParams.append('sort', params.sort)
    if (params.page) queryParams.append('page', params.page.toString())
    if (params.limit) queryParams.append('limit', params.limit.toString())

    const response = await this.client.get(`/explore/search?${queryParams.toString()}`)
    return response.data
  }

  // Get projects by category
  async getProjectsByCategory(slug: string, page: number = 1, limit: number = 20): Promise<{
    category: ProjectCategory
    projects: ProjectWithStats[]
    pagination: any
  }> {
    const response = await this.client.get(`/explore/category/${slug}?page=${page}&limit=${limit}`)
    return response.data
  }

  // Get all categories
  async getCategories(): Promise<ProjectCategory[]> {
    const response = await this.client.get<{ categories: ProjectCategory[] }>('/explore/categories')
    return response.data.categories
  }

  async getProjectCategories(projectId: number): Promise<string[]> {
    const response = await this.client.get<ApiResponse<{ categories: string[] }>>(
      `/projects/${projectId}/categories`
    )
    return response.data.data?.categories || (response.data as any).categories || []
  }

  async setProjectCategories(projectId: number, categories: string[]): Promise<void> {
    await this.client.put(`/projects/${projectId}/categories`, { categories })
  }

  // Get public project page
  async getPublicProject(username: string, projectName: string): Promise<{
    project: Project
    stats: any
    is_starred: boolean
    is_fork: boolean
    original_id?: number
    categories: string[]
    readme: string
    comments: ProjectComment[]
  }> {
    const response = await this.client.get(`/project/${username}/${projectName}`)
    return response.data
  }

  // Star/unstar project
  async starProject(projectId: number): Promise<void> {
    await this.client.post(`/projects/${projectId}/star`)
  }

  async unstarProject(projectId: number): Promise<void> {
    await this.client.delete(`/projects/${projectId}/star`)
  }

  // Fork project
  async forkProject(projectId: number): Promise<{ project: Project }> {
    const response = await this.client.post(`/projects/${projectId}/fork`)
    return response.data
  }

  // Comments
  async getProjectComments(projectId: number, page: number = 1, limit: number = 20): Promise<{
    comments: ProjectComment[]
    pagination: any
  }> {
    const response = await this.client.get(`/projects/${projectId}/comments?page=${page}&limit=${limit}`)
    return response.data
  }

  async createProjectComment(projectId: number, content: string, parentId?: number): Promise<{
    comment: ProjectComment
  }> {
    const response = await this.client.post(`/projects/${projectId}/comments`, {
      content,
      parent_id: parentId,
    })
    return response.data
  }

  async deleteProjectComment(projectId: number, commentId: number): Promise<void> {
    await this.client.delete(`/projects/${projectId}/comments/${commentId}`)
  }

  // Public user profiles
  async getPublicUserProfile(username: string): Promise<{ profile: UserPublicProfile }> {
    const response = await this.client.get(`/users/${username}`)
    return response.data
  }

  async getUserProjects(username: string, page: number = 1, limit: number = 20): Promise<{
    projects: ProjectWithStats[]
    pagination: any
  }> {
    const response = await this.client.get(`/users/${username}/projects?page=${page}&limit=${limit}`)
    return response.data
  }

  async getUserStarredProjects(username: string, page: number = 1, limit: number = 20): Promise<{
    projects: ProjectWithStats[]
    pagination: any
  }> {
    const response = await this.client.get(`/users/${username}/starred?page=${page}&limit=${limit}`)
    return response.data
  }

  // Follow/unfollow users
  async followUser(username: string): Promise<void> {
    await this.client.post(`/users/${username}/follow`)
  }

  async unfollowUser(username: string): Promise<void> {
    await this.client.delete(`/users/${username}/follow`)
  }

  async getFollowers(username: string, page: number = 1, limit: number = 20): Promise<{
    followers: UserFollowInfo[]
    pagination: any
  }> {
    const response = await this.client.get(`/users/${username}/followers?page=${page}&limit=${limit}`)
    return response.data
  }

  async getFollowing(username: string, page: number = 1, limit: number = 20): Promise<{
    following: UserFollowInfo[]
    pagination: any
  }> {
    const response = await this.client.get(`/users/${username}/following?page=${page}&limit=${limit}`)
    return response.data
  }

  // ========== VERSION HISTORY ENDPOINTS (Replit parity) ==========

  async getFileVersions(fileId: number): Promise<FileVersion[]> {
    const response = await this.client.get<ApiResponse<{ versions: FileVersion[] }>>(
      `/versions/file/${fileId}`
    )
    return response.data.data!.versions
  }

  async getFileVersion(versionId: number): Promise<FileVersion> {
    const response = await this.client.get<ApiResponse<{ version: FileVersion }>>(
      `/versions/${versionId}`
    )
    return response.data.data!.version
  }

  async getFileVersionContent(versionId: number): Promise<string> {
    const response = await this.client.get<ApiResponse<{ content: string }>>(
      `/versions/${versionId}/content`
    )
    return response.data.data!.content
  }

  async restoreFileVersion(versionId: number): Promise<File> {
    const response = await this.client.post<ApiResponse<{ file: File }>>(
      `/versions/${versionId}/restore`
    )
    return response.data.data!.file
  }

  async pinFileVersion(versionId: number, pinned: boolean): Promise<FileVersion> {
    const response = await this.client.post<ApiResponse<{ version: FileVersion }>>(
      `/versions/${versionId}/pin`,
      { pinned }
    )
    return response.data.data!.version
  }

  async deleteFileVersion(versionId: number): Promise<void> {
    await this.client.delete(`/versions/${versionId}`)
  }

  async getVersionDiff(oldVersionId: number, newVersionId: number): Promise<VersionDiff> {
    const response = await this.client.get<ApiResponse<{ diff: VersionDiff }>>(
      `/versions/diff/${oldVersionId}/${newVersionId}`
    )
    return response.data.data!.diff
  }

  // ========== MANAGED DATABASE ENDPOINTS ==========

  async createDatabase(projectId: number, data: CreateDatabaseRequest): Promise<ManagedDatabase> {
    const response = await this.client.post<ApiResponse<{ database: ManagedDatabase }>>(
      `/projects/${projectId}/databases`,
      data
    )
    return response.data.data!.database
  }

  async getDatabases(projectId: number): Promise<ManagedDatabase[]> {
    const response = await this.client.get<ApiResponse<{ databases: ManagedDatabase[] }>>(
      `/projects/${projectId}/databases`
    )
    return response.data.data!.databases
  }

  async getDatabase(projectId: number, dbId: number, includeCredentials = false): Promise<ManagedDatabase> {
    const response = await this.client.get<ApiResponse<{ database: ManagedDatabase }>>(
      `/projects/${projectId}/databases/${dbId}?include_credentials=${includeCredentials}`
    )
    return response.data.data!.database
  }

  async deleteDatabase(projectId: number, dbId: number): Promise<void> {
    await this.client.delete(`/projects/${projectId}/databases/${dbId}`)
  }

  async resetDatabaseCredentials(projectId: number, dbId: number): Promise<{
    username: string
    password: string
    connection_string: string
  }> {
    const response = await this.client.post<ApiResponse<{ credentials: any }>>(
      `/projects/${projectId}/databases/${dbId}/reset`
    )
    return response.data.data!.credentials
  }

  async executeSQLQuery(projectId: number, dbId: number, query: string): Promise<{
    result: {
      columns: string[]
      rows: any[][]
      affected_rows: number
      duration_ms: number
    }
  }> {
    const response = await this.client.post<ApiResponse<{ result: any }>>(
      `/projects/${projectId}/databases/${dbId}/query`,
      { query }
    )
    return response.data.data!
  }

  async getDatabaseTables(projectId: number, dbId: number): Promise<TableInfo[]> {
    const response = await this.client.get<ApiResponse<{ tables: TableInfo[] }>>(
      `/projects/${projectId}/databases/${dbId}/tables`
    )
    return response.data.data!.tables
  }

  async getTableSchema(projectId: number, dbId: number, tableName: string): Promise<ColumnInfo[]> {
    const response = await this.client.get<ApiResponse<{ columns: ColumnInfo[] }>>(
      `/projects/${projectId}/databases/${dbId}/tables/${tableName}/schema`
    )
    return response.data.data!.columns
  }

  async getDatabaseMetrics(projectId: number, dbId: number): Promise<{
    metrics: DatabaseMetrics
    limits: { max_storage_mb: number; max_connections: number }
  }> {
    const response = await this.client.get<ApiResponse<{ metrics: DatabaseMetrics; limits: any }>>(
      `/projects/${projectId}/databases/${dbId}/metrics`
    )
    return response.data.data!
  }

  // ========== GITHUB IMPORT WIZARD ENDPOINTS ==========

  // Validate GitHub URL and get repo info
  async validateGitHubUrl(url: string, token?: string): Promise<{
    valid: boolean
    error?: string
    hint?: string
    private?: boolean
    owner?: string
    repo?: string
    name?: string
    description?: string
    default_branch?: string
    language?: string
    size?: number
    stars?: number
    forks?: number
    detected_stack?: {
      language: string
      framework: string
      package_manager: string
      entry_point: string
    }
  }> {
    const response = await this.client.post('/projects/import/github/validate', { url, token })
    return response.data
  }

  // Import GitHub repository
  async importGitHubRepo(data: {
    url: string
    project_name?: string
    description?: string
    is_public?: boolean
    token?: string
  }): Promise<{
    project_id: number
    project_name: string
    language: string
    framework: string
    detected_stack: {
      language: string
      framework: string
      package_manager: string
      entry_point: string
    }
    file_count: number
    status: string
    message: string
    import_duration_ms: number
    repository_url: string
    default_branch: string
  }> {
    const response = await this.client.post('/projects/import/github', data)
    return response.data
  }

  // ========== CODE COMMENTS ENDPOINTS (Replit parity) ==========

  // Create a new comment or reply
  async createCodeComment(data: CreateCommentRequest): Promise<CodeComment> {
    const response = await this.client.post<ApiResponse<CodeComment>>('/comments', data)
    return response.data.data!
  }

  // Get all comments for a file
  async getFileComments(fileId: number, options?: {
    include_resolved?: boolean
    line?: number
  }): Promise<{ file_id: number; comments: CodeComment[]; total: number }> {
    const params = new URLSearchParams()
    if (options?.include_resolved !== undefined) {
      params.append('include_resolved', options.include_resolved.toString())
    }
    if (options?.line) {
      params.append('line', options.line.toString())
    }
    const query = params.toString() ? `?${params.toString()}` : ''
    const response = await this.client.get<ApiResponse<{ file_id: number; comments: CodeComment[]; total: number }>>(
      `/comments/file/${fileId}${query}`
    )
    return response.data.data!
  }

  // Get a specific thread
  async getCommentThread(threadId: string): Promise<CodeCommentThread> {
    const response = await this.client.get<ApiResponse<CodeCommentThread>>(
      `/comments/thread/${threadId}`
    )
    return response.data.data!
  }

  // Get a single comment
  async getCodeComment(commentId: number): Promise<CodeComment> {
    const response = await this.client.get<ApiResponse<CodeComment>>(
      `/comments/${commentId}`
    )
    return response.data.data!
  }

  // Update a comment
  async updateCodeComment(commentId: number, data: UpdateCommentRequest): Promise<CodeComment> {
    const response = await this.client.put<ApiResponse<CodeComment>>(
      `/comments/${commentId}`,
      data
    )
    return response.data.data!
  }

  // Delete a comment
  async deleteCodeComment(commentId: number): Promise<void> {
    await this.client.delete(`/comments/${commentId}`)
  }

  // Resolve a thread
  async resolveCommentThread(commentId: number): Promise<{
    thread_id: string
    resolved_at: string
    resolved_by: number
  }> {
    const response = await this.client.post<ApiResponse<{
      thread_id: string
      resolved_at: string
      resolved_by: number
    }>>(`/comments/${commentId}/resolve`)
    return response.data.data!
  }

  // Unresolve a thread
  async unresolveCommentThread(commentId: number): Promise<{ thread_id: string }> {
    const response = await this.client.post<ApiResponse<{ thread_id: string }>>(
      `/comments/${commentId}/unresolve`
    )
    return response.data.data!
  }

  // Add reaction to a comment
  async addCommentReaction(commentId: number, emoji: string): Promise<Record<string, number[]>> {
    const response = await this.client.post<ApiResponse<Record<string, number[]>>>(
      `/comments/${commentId}/react`,
      { emoji }
    )
    return response.data.data!
  }

  // Remove reaction from a comment
  async removeCommentReaction(commentId: number, emoji: string): Promise<Record<string, number[]>> {
    const response = await this.client.delete<ApiResponse<Record<string, number[]>>>(
      `/comments/${commentId}/react`,
      { data: { emoji } }
    )
    return response.data.data!
  }

  // ========== BYOK (BRING YOUR OWN KEY) ENDPOINTS ==========

  async saveAPIKey(
    provider: string,
    apiKey: string,
    options?: { model_preference?: string }
  ): Promise<{
    success: boolean
    message: string
    provider: string
  }> {
    const response = await this.client.post('/byok/keys', {
      provider,
      api_key: apiKey,
      model_preference: options?.model_preference || '',
    })
    return response.data
  }

  async getAPIKeys(): Promise<{
    success: boolean
    data: Array<{
      provider: string
      model_preference: string
      is_active: boolean
      is_valid: boolean
      last_used?: string
      usage_count: number
      total_cost: number
    }>
  }> {
    const response = await this.client.get('/byok/keys')
    return response.data
  }

  async deleteAPIKey(provider: string): Promise<{ success: boolean; message: string }> {
    const response = await this.client.delete(`/byok/keys/${provider}`)
    return response.data
  }

  async validateAPIKey(provider: string): Promise<{
    success: boolean
    provider: string
    valid: boolean
    error_detail?: string
  }> {
    const response = await this.client.post(`/byok/keys/${provider}/validate`)
    return response.data
  }

  async updateAPIKeySettings(
    provider: string,
    settings: { is_active?: boolean; model_preference?: string }
  ): Promise<{ success: boolean; message: string }> {
    const response = await this.client.patch(`/byok/keys/${provider}`, settings)
    return response.data
  }

  async getBYOKUsage(month?: string): Promise<{
    success: boolean
    data: {
      total_cost: number
      total_tokens: number
      total_requests: number
      byok_cost?: number
      byok_tokens?: number
      byok_requests?: number
      platform_cost?: number
      platform_tokens?: number
      platform_requests?: number
      by_provider: Record<string, {
        provider: string
        cost: number
        tokens: number
        requests: number
        byok_requests: number
      }>
    }
    month: string
    billing?: {
      credit_balance: number
      has_unlimited_credits: boolean
      bypass_billing: boolean
    }
  }> {
    const params = month ? `?month=${month}` : ''
    const response = await this.client.get(`/byok/usage${params}`)
    return response.data
  }

  async getAvailableModels(): Promise<{
    success: boolean
    data: Record<string, Array<{
      id: string
      name: string
      speed: string
      cost_tier: string
      description: string
    }>>
  }> {
    const response = await this.client.get('/byok/models')
    return response.data
  }

  // ========== TERMINAL SESSION ENDPOINTS ==========

  /**
   * Create a new terminal session
   */
  async createTerminalSession(
    projectId?: number,
    options?: {
      shell?: string
      workDir?: string
      name?: string
      rows?: number
      cols?: number
      environment?: Record<string, string>
    }
  ): Promise<TerminalSessionResponse> {
    const response = await this.client.post<TerminalSessionCreateResponse>(
      '/terminal/sessions',
      {
        project_id: projectId || 0,
        work_dir: options?.workDir || '',
        shell: options?.shell || '',
        name: options?.name || '',
        rows: options?.rows || 24,
        cols: options?.cols || 80,
        environment: options?.environment || {},
      }
    )
    return response.data.data
  }

  /**
   * List all terminal sessions for the current user
   */
  async listTerminalSessions(projectId?: number): Promise<TerminalSessionInfo[]> {
    const params = projectId ? `?project_id=${projectId}` : ''
    const response = await this.client.get<{ success: boolean; data: TerminalSessionInfo[] }>(
      `/terminal/sessions${params}`
    )
    return response.data.data || []
  }

  /**
   * Get a specific terminal session by ID
   */
  async getTerminalSession(sessionId: string): Promise<TerminalSessionInfo | null> {
    try {
      const response = await this.client.get<{ success: boolean; data: TerminalSessionInfo }>(
        `/terminal/sessions/${sessionId}`
      )
      return response.data.data
    } catch {
      return null
    }
  }

  /**
   * Close/delete a terminal session
   */
  async closeTerminalSession(sessionId: string): Promise<void> {
    await this.client.delete(`/terminal/sessions/${sessionId}`)
  }

  /**
   * Resize a terminal session
   */
  async resizeTerminal(sessionId: string, cols: number, rows: number): Promise<void> {
    await this.client.post(`/terminal/sessions/${sessionId}/resize`, {
      rows,
      cols,
    })
  }

  /**
   * Get command history for a terminal session
   */
  async getCommandHistory(sessionId: string): Promise<string[]> {
    try {
      const response = await this.client.get<{ success: boolean; data: { history: string[] } }>(
        `/terminal/sessions/${sessionId}/history`
      )
      return response.data.data?.history || []
    } catch {
      return []
    }
  }

  /**
   * List available shells on the system
   */
  async listAvailableShells(): Promise<AvailableShell[]> {
    try {
      const response = await this.client.get<{ success: boolean; data: { shells: AvailableShell[] } }>(
        '/terminal/shells'
      )
      return response.data.data?.shells || []
    } catch {
      // Default shells if API fails
      return [
        { name: 'bash', path: '/bin/bash' },
        { name: 'sh', path: '/bin/sh' },
      ]
    }
  }

  /**
   * Get WebSocket URL for terminal connection
   */
  getTerminalWebSocketUrl(sessionId: string): string {
    const protocol = typeof window !== 'undefined' && window.location.protocol === 'https:' ? 'wss:' : 'ws:'

    // Use API URL if available
    if (this.baseURL) {
      try {
        const url = new URL(this.baseURL)
        return `${protocol}//${url.host}/ws/terminal/${sessionId}`
      } catch {
        // Fall through to default
      }
    }

    const host = typeof window !== 'undefined' ? window.location.host : 'localhost:8080'
    return `${protocol}//${host}/ws/terminal/${sessionId}`
  }

  // ========== AI CODE REVIEW ENDPOINTS ==========

  async reviewCode(data: {
    code: string
    language: string
    file_name?: string
    context?: string
    focus?: string[]
    max_results?: number
  }): Promise<CodeReviewResponse> {
    const response = await this.client.post<CodeReviewResponse>('/ai/code-review', data)
    return response.data
  }

  // ========== PACKAGE MANAGEMENT ENDPOINTS ==========

  /**
   * Search for packages across registries (npm, pypi, go)
   */
  async searchPackages(
    query: string,
    registry: PackageRegistry,
    limit: number = 20
  ): Promise<PackageSearchResult[]> {
    const response = await this.client.get<{
      success: boolean
      data: { packages: PackageSearchResult[]; query: string; type: string; count: number }
    }>(`/packages/search?q=${encodeURIComponent(query)}&type=${registry}&limit=${limit}`)
    return response.data.data?.packages || []
  }

  /**
   * Get detailed information about a specific package
   */
  async getPackageInfo(name: string, registry: PackageRegistry): Promise<PackageInfo> {
    const encodedName = encodeURIComponent(name)
    const response = await this.client.get<{ success: boolean; data: PackageInfo }>(
      `/packages/info/${encodedName}?type=${registry}`
    )
    return response.data.data
  }

  /**
   * Install a package to a project
   */
  async installPackage(
    projectId: number,
    packageName: string,
    version: string,
    registry: PackageRegistry,
    isDev: boolean = false
  ): Promise<{ package: string; version: string; is_dev: boolean; info?: PackageInfo }> {
    const response = await this.client.post<{
      success: boolean
      message: string
      data: { package: string; version: string; is_dev: boolean; info?: PackageInfo }
    }>('/packages/install', {
      project_id: projectId,
      package_name: packageName,
      version,
      type: registry,
      is_dev: isDev,
    })
    return response.data.data
  }

  /**
   * Uninstall a package from a project
   */
  async uninstallPackage(
    projectId: number,
    packageName: string,
    registry: PackageRegistry = 'npm'
  ): Promise<void> {
    const encodedName = encodeURIComponent(packageName)
    await this.client.delete(`/packages/${projectId}/${encodedName}?type=${registry}`)
  }

  /**
   * List all installed packages for a project
   */
  async listProjectPackages(
    projectId: number,
    registry: PackageRegistry | 'all' = 'all'
  ): Promise<{
    npm?: InstalledPackage[]
    pip?: InstalledPackage[]
    go?: InstalledPackage[]
  }> {
    const response = await this.client.get<{
      success: boolean
      data: {
        project_id: number
        packages: {
          npm?: InstalledPackage[]
          pip?: InstalledPackage[]
          go?: InstalledPackage[]
        }
      }
    }>(`/packages/project/${projectId}?type=${registry}`)
    return response.data.data?.packages || {}
  }

  /**
   * Update all packages for a project to their latest versions
   */
  async updateAllPackages(
    projectId: number,
    registry?: PackageRegistry
  ): Promise<{ packages: InstalledPackage[] }> {
    const typeParam = registry ? `?type=${registry}` : ''
    const response = await this.client.post<{
      success: boolean
      message: string
      data: { project_id: number; type: string; packages: InstalledPackage[] }
    }>(`/packages/project/${projectId}/update${typeParam}`)
    return { packages: response.data.data?.packages || [] }
  }

  /**
   * Get package suggestions based on project language and framework
   */
  async getPackageSuggestions(projectId: number): Promise<{
    language: string
    framework: string
    suggestions: PackageSuggestion[]
  }> {
    const response = await this.client.get<{
      success: boolean
      data: {
        project_id: number
        language: string
        framework: string
        suggestions: PackageSuggestion[]
      }
    }>(`/packages/suggestions/${projectId}`)
    return {
      language: response.data.data?.language || '',
      framework: response.data.data?.framework || '',
      suggestions: response.data.data?.suggestions || [],
    }
  }

  // ========== ALWAYS-ON DEPLOYMENT ENDPOINTS (Replit parity) ==========

  // Get always-on status for a deployment
  async getAlwaysOnStatus(projectId: number, deploymentId: string): Promise<AlwaysOnStatus> {
    const response = await this.client.get<ApiResponse<{ status: AlwaysOnStatus }>>(
      `/projects/${projectId}/deployments/${deploymentId}/always-on`
    )
    return response.data.data!.status
  }

  // Enable or disable always-on for a deployment
  async setAlwaysOn(
    projectId: number,
    deploymentId: string,
    enabled: boolean,
    keepAliveInterval?: number
  ): Promise<{ success: boolean; always_on: boolean; message: string }> {
    const response = await this.client.put<ApiResponse<{
      success: boolean
      always_on: boolean
      message: string
    }>>(`/projects/${projectId}/deployments/${deploymentId}/always-on`, {
      always_on: enabled,
      keep_alive_interval: keepAliveInterval || 60,
    })
    return response.data.data!
  }

  // Start a native deployment with always-on option
  async startNativeDeployment(projectId: number, config: NativeDeploymentConfig): Promise<{
    success: boolean
    deployment: NativeDeployment
    message: string
    websocket_url: string
  }> {
    const response = await this.client.post(`/projects/${projectId}/deploy`, config)
    return response.data
  }

  // Get deployments for a project
  async getNativeDeployments(projectId: number, page: number = 1, limit: number = 20): Promise<{
    deployments: NativeDeployment[]
    total: number
    page: number
    limit: number
  }> {
    const response = await this.client.get(`/projects/${projectId}/deployments?page=${page}&limit=${limit}`)
    return response.data
  }

  // Get a specific deployment
  async getNativeDeployment(projectId: number, deploymentId: string): Promise<NativeDeployment> {
    const response = await this.client.get<ApiResponse<{ deployment: NativeDeployment }>>(
      `/projects/${projectId}/deployments/${deploymentId}`
    )
    return response.data.data!.deployment
  }

  // Get deployment logs
  async getDeploymentLogs(projectId: number, deploymentId: string, limit: number = 100): Promise<DeploymentLog[]> {
    const response = await this.client.get<ApiResponse<{ logs: DeploymentLog[] }>>(
      `/projects/${projectId}/deployments/${deploymentId}/logs?limit=${limit}`
    )
    return response.data.data!.logs
  }

  // Stop a deployment
  async stopNativeDeployment(projectId: number, deploymentId: string): Promise<void> {
    await this.client.delete(`/projects/${projectId}/deployments/${deploymentId}`)
  }

  // Restart a deployment
  async restartNativeDeployment(projectId: number, deploymentId: string): Promise<void> {
    await this.client.post(`/projects/${projectId}/deployments/${deploymentId}/restart`)
  }

  // Get deployment metrics
  async getDeploymentMetrics(projectId: number, deploymentId: string): Promise<DeploymentMetrics> {
    const response = await this.client.get<ApiResponse<{ metrics: DeploymentMetrics }>>(
      `/projects/${projectId}/deployments/${deploymentId}/metrics`
    )
    return response.data.data!.metrics
  }

  // ========== CODE COMPLETIONS ENDPOINTS (Ghostwriter-equivalent) ==========

  /**
   * Get inline ghost-text completions for the Monaco editor
   * Returns a single completion item for displaying as ghost text
   */
  async getInlineCompletions(
    code: string,
    cursorPosition: { line: number; column: number },
    language: string,
    context?: {
      projectId?: number
      fileId?: number
      filePath?: string
      suffix?: string
      triggerKind?: 'invoked' | 'trigger_char' | 'automatic' | 'incomplete'
      fileImports?: string[]
      fileSymbols?: string[]
      framework?: string
    }
  ): Promise<CompletionItem | null> {
    const request: CompletionRequest = {
      project_id: context?.projectId,
      file_id: context?.fileId,
      file_path: context?.filePath || '',
      language,
      prefix: code,
      suffix: context?.suffix || '',
      line: cursorPosition.line,
      column: cursorPosition.column,
      trigger_kind: context?.triggerKind || 'automatic',
      context: {
        file_imports: context?.fileImports,
        file_symbols: context?.fileSymbols,
        framework: context?.framework,
      },
    }

    const response = await this.client.post<{
      success: boolean
      completion: CompletionItem | null
    }>('/completions/inline', request)

    return response.data.completion
  }

  /**
   * Get multiple completion suggestions
   * Returns a list of completion items for a completion menu
   */
  async getCompletions(
    code: string,
    cursorPosition: { line: number; column: number },
    language: string,
    context?: {
      projectId?: number
      fileId?: number
      filePath?: string
      suffix?: string
      triggerKind?: 'invoked' | 'trigger_char' | 'automatic' | 'incomplete'
      fileImports?: string[]
      fileSymbols?: string[]
      framework?: string
      maxTokens?: number
      temperature?: number
    }
  ): Promise<CompletionResponse> {
    const request: CompletionRequest = {
      project_id: context?.projectId,
      file_id: context?.fileId,
      file_path: context?.filePath || '',
      language,
      prefix: code,
      suffix: context?.suffix || '',
      line: cursorPosition.line,
      column: cursorPosition.column,
      trigger_kind: context?.triggerKind || 'invoked',
      context: {
        file_imports: context?.fileImports,
        file_symbols: context?.fileSymbols,
        framework: context?.framework,
      },
      max_tokens: context?.maxTokens,
      temperature: context?.temperature,
    }

    const response = await this.client.post<{
      success: boolean
      data: CompletionResponse
    }>('/completions', request)

    return response.data.data
  }

  /**
   * Record when a user accepts a completion (for analytics and improving suggestions)
   */
  async acceptCompletion(completionId: string, accepted: boolean = true): Promise<void> {
    await this.client.post('/completions/accept', {
      completion_id: completionId,
      accepted,
    })
  }

  /**
   * Get completion statistics (admin only)
   */
  async getCompletionStats(): Promise<CompletionStats> {
    const response = await this.client.get<{
      success: boolean
      data: CompletionStats
    }>('/completions/stats')
    return response.data.data
  }

  // ========== DEBUGGING SYSTEM ENDPOINTS ==========

  // Start a new debug session
  async startDebugSession(data: {
    project_id: number
    file_id?: number
    entry_point: string
    language: string
  }): Promise<{
    message: string
    session: DebugSession
    websocket_url: string
  }> {
    const response = await this.client.post('/debug/sessions', data)
    return response.data
  }

  // Get debug session info
  async getDebugSession(sessionId: string): Promise<{
    session: DebugSession
    breakpoints: DebugBreakpoint[]
  }> {
    const response = await this.client.get(`/debug/sessions/${sessionId}`)
    return response.data
  }

  // Stop debug session
  async stopDebugSession(sessionId: string): Promise<{ message: string }> {
    const response = await this.client.post(`/debug/sessions/${sessionId}/stop`)
    return response.data
  }

  // Set a breakpoint
  async setDebugBreakpoint(
    sessionId: string,
    data: {
      file_id: number
      file_path: string
      line: number
      column?: number
      type?: DebugBreakpointType
      condition?: string
    }
  ): Promise<{ message: string; breakpoint: DebugBreakpoint }> {
    const response = await this.client.post(
      `/debug/sessions/${sessionId}/breakpoints`,
      data
    )
    return response.data
  }

  // Remove a breakpoint
  async removeDebugBreakpoint(
    sessionId: string,
    breakpointId: string
  ): Promise<{ message: string }> {
    const response = await this.client.delete(
      `/debug/sessions/${sessionId}/breakpoints/${breakpointId}`
    )
    return response.data
  }

  // Toggle breakpoint enabled state
  async toggleDebugBreakpoint(
    sessionId: string,
    breakpointId: string,
    enabled: boolean
  ): Promise<{ message: string; enabled: boolean }> {
    const response = await this.client.patch(
      `/debug/sessions/${sessionId}/breakpoints/${breakpointId}`,
      { enabled }
    )
    return response.data
  }

  // Continue execution
  async debugContinue(sessionId: string): Promise<{ message: string }> {
    const response = await this.client.post(
      `/debug/sessions/${sessionId}/continue`
    )
    return response.data
  }

  // Pause execution
  async debugPause(sessionId: string): Promise<{ message: string }> {
    const response = await this.client.post(`/debug/sessions/${sessionId}/pause`)
    return response.data
  }

  // Step over
  async debugStepOver(sessionId: string): Promise<{ message: string }> {
    const response = await this.client.post(
      `/debug/sessions/${sessionId}/step-over`
    )
    return response.data
  }

  // Step into
  async debugStepInto(sessionId: string): Promise<{ message: string }> {
    const response = await this.client.post(
      `/debug/sessions/${sessionId}/step-into`
    )
    return response.data
  }

  // Step out
  async debugStepOut(sessionId: string): Promise<{ message: string }> {
    const response = await this.client.post(
      `/debug/sessions/${sessionId}/step-out`
    )
    return response.data
  }

  // Get call stack
  async getDebugCallStack(sessionId: string): Promise<{
    call_stack: DebugStackFrame[]
  }> {
    const response = await this.client.get(`/debug/sessions/${sessionId}/stack`)
    return response.data
  }

  // Get variables for a scope/object
  async getDebugVariables(
    sessionId: string,
    objectId: string
  ): Promise<{ variables: DebugVariable[] }> {
    const response = await this.client.get(
      `/debug/sessions/${sessionId}/variables/${objectId}`
    )
    return response.data
  }

  // Evaluate expression
  async evaluateDebugExpression(
    sessionId: string,
    expression: string
  ): Promise<{ result: DebugEvaluateResult }> {
    const response = await this.client.post(
      `/debug/sessions/${sessionId}/evaluate`,
      { expression }
    )
    return response.data
  }

  // Get watch expressions
  async getDebugWatches(sessionId: string): Promise<{
    watches: DebugWatchExpression[]
  }> {
    const response = await this.client.get(
      `/debug/sessions/${sessionId}/watches`
    )
    return response.data
  }

  // Add watch expression
  async addDebugWatch(
    sessionId: string,
    expression: string
  ): Promise<{ watch: DebugWatchExpression }> {
    const response = await this.client.post(
      `/debug/sessions/${sessionId}/watches`,
      { expression }
    )
    return response.data
  }

  // Remove watch expression
  async removeDebugWatch(
    sessionId: string,
    watchId: string
  ): Promise<{ message: string }> {
    const response = await this.client.delete(
      `/debug/sessions/${sessionId}/watches/${watchId}`
    )
    return response.data
  }

  // Get WebSocket URL for debug events
  getDebugWebSocketUrl(sessionId: string): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    // Use appropriate host based on environment
    let host: string
    if (this.baseURL.includes('onrender.com')) {
      host = 'apex-backend-5ypy.onrender.com'
    } else if (this.baseURL.startsWith('/')) {
      host = window.location.host
    } else {
      try {
        const url = new URL(this.baseURL)
        host = url.host
      } catch {
        host = window.location.host
      }
    }
    return `${protocol}//${host}/ws/debug/${sessionId}`
  }

  // ========== HOSTING SYSTEM ENDPOINTS (*.apex.app) ==========

  /**
   * Get hosting status for a project
   * Returns current deployment state, URL, and metrics
   */
  async getHostingStatus(projectId: number): Promise<HostingStatus> {
    const response = await this.client.get<{ success: boolean; data: HostingStatus }>(
      `/hosting/${projectId}/status`
    )
    return response.data.data
  }

  /**
   * Quick deploy a project to *.apex.app
   * Auto-detects build configuration if not provided
   */
  async quickDeploy(
    projectId: number,
    config?: QuickDeployConfig
  ): Promise<{
    success: boolean
    message: string
    deployment: NativeDeployment
    websocket_url: string
  }> {
    const response = await this.client.post(`/hosting/${projectId}/deploy`, config || {})
    return response.data
  }

  /**
   * Undeploy/stop a project's hosting
   */
  async undeploy(projectId: number): Promise<{ success: boolean; message: string }> {
    const response = await this.client.delete(`/hosting/${projectId}/undeploy`)
    return response.data
  }

  /**
   * Get deployment logs for a project
   */
  async getHostingLogs(
    projectId: number,
    options?: { limit?: number; level?: string; since?: string }
  ): Promise<DeploymentLog[]> {
    const params = new URLSearchParams()
    if (options?.limit) params.append('limit', options.limit.toString())
    if (options?.level) params.append('level', options.level)
    if (options?.since) params.append('since', options.since)
    const query = params.toString() ? `?${params.toString()}` : ''

    const response = await this.client.get<{ success: boolean; data: { logs: DeploymentLog[] } }>(
      `/hosting/${projectId}/logs${query}`
    )
    return response.data.data?.logs || []
  }

  /**
   * Get the deployment URL for a project
   */
  async getDeploymentUrl(projectId: number): Promise<{
    url: string
    subdomain: string
    status: string
    ssl_enabled: boolean
  }> {
    const response = await this.client.get<{
      success: boolean
      data: { url: string; subdomain: string; status: string; ssl_enabled: boolean }
    }>(`/hosting/${projectId}/url`)
    return response.data.data
  }

  /**
   * Check if a subdomain is available
   */
  async checkSubdomainAvailability(subdomain: string): Promise<{
    available: boolean
    subdomain: string
    suggestion?: string
  }> {
    const response = await this.client.get<{
      success: boolean
      data: { available: boolean; subdomain: string; suggestion?: string }
    }>(`/hosting/check-subdomain?subdomain=${encodeURIComponent(subdomain)}`)
    return response.data.data
  }

  // ========== CUSTOM DOMAIN MANAGEMENT ==========

  /**
   * Get all custom domains for a project
   */
  async getCustomDomains(projectId: number): Promise<CustomDomain[]> {
    const response = await this.client.get<{
      success: boolean
      data: { domains: CustomDomain[] }
    }>(`/hosting/${projectId}/domains`)
    return response.data.data?.domains || []
  }

  /**
   * Add a custom domain to a project
   */
  async addCustomDomain(
    projectId: number,
    domain: string
  ): Promise<{
    domain: CustomDomain
    verification_instructions: string
  }> {
    const response = await this.client.post<{
      success: boolean
      message: string
      data: { domain: CustomDomain; verification_instructions: string }
    }>(`/hosting/${projectId}/domains`, { domain })
    return response.data.data
  }

  /**
   * Verify a custom domain's DNS configuration
   */
  async verifyCustomDomain(
    projectId: number,
    domainId: number
  ): Promise<{
    verified: boolean
    domain: CustomDomain
    error?: string
  }> {
    const response = await this.client.post<{
      success: boolean
      data: { verified: boolean; domain: CustomDomain; error?: string }
    }>(`/hosting/${projectId}/domains/${domainId}/verify`)
    return response.data.data
  }

  /**
   * Delete a custom domain from a project
   */
  async deleteCustomDomain(projectId: number, domainId: number): Promise<void> {
    await this.client.delete(`/hosting/${projectId}/domains/${domainId}`)
  }

  /**
   * Get WebSocket URL for deployment logs streaming
   */
  getDeploymentLogsWebSocketUrl(projectId: number): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    let host: string
    if (this.baseURL.includes('onrender.com')) {
      host = 'apex-backend-5ypy.onrender.com'
    } else if (this.baseURL.startsWith('/')) {
      host = window.location.host
    } else {
      try {
        const url = new URL(this.baseURL)
        host = url.host
      } catch {
        host = window.location.host
      }
    }
    return `${protocol}//${host}/ws/deployment/${projectId}/logs`
  }

  // ========== PLAN USAGE QUOTA ENDPOINTS ==========

  /**
   * Get current usage and limits for the authenticated user
   * Returns usage across all tracked dimensions: projects, storage, AI requests, execution time
   */
  async getCurrentUsage(): Promise<CurrentUsageData> {
    const response = await this.client.get<{ success: boolean; data: CurrentUsageData }>(
      '/usage/current'
    )
    return response.data.data
  }

  /**
   * Get historical usage data for charts and trend analysis
   */
  async getUsageHistory(days: number = 30): Promise<UsageHistoryData> {
    const response = await this.client.get<{ success: boolean; data: UsageHistoryData }>(
      `/usage/history?days=${days}`
    )
    return response.data.data
  }

  /**
   * Get plan limits for the current user and all available plans
   * Useful for showing upgrade comparison
   */
  async getUsageLimits(): Promise<UsageLimitsData> {
    const response = await this.client.get<{ success: boolean; data: UsageLimitsData }>(
      '/usage/limits'
    )
    return response.data.data
  }

  /**
   * Force refresh the usage cache (useful after major operations)
   */
  async refreshUsageCache(): Promise<{ message: string }> {
    const response = await this.client.post<{ success: boolean; message: string }>(
      '/usage/refresh'
    )
    return { message: response.data.message }
  }
}

// ---------------------------------------------------------------------------
// Terminal Session types
// ---------------------------------------------------------------------------

export interface TerminalSessionResponse {
  session_id: string
  project_id: number
  work_dir: string
  shell: string
  name: string
  rows: number
  cols: number
  created_at: string
  ws_endpoint: string
}

export interface TerminalSessionCreateResponse {
  success: boolean
  data: TerminalSessionResponse
}

export interface TerminalSessionInfo {
  id: string
  project_id: number
  user_id: number
  work_dir: string
  shell: string
  shell_name?: string
  name: string
  created_at: string
  last_active: string
  is_active: boolean
  rows: number
  cols: number
  ws_endpoint: string
}

export interface AvailableShell {
  name: string
  path: string
}

// ---------------------------------------------------------------------------
// Always-On Deployment types (Replit parity feature)
// ---------------------------------------------------------------------------

export interface AlwaysOnStatus {
  always_on: boolean
  always_on_enabled: string | null
  last_keep_alive: string | null
  keep_alive_interval: number
  sleep_after_minutes: number
  restart_count: number
  max_restarts: number
  container_status: 'healthy' | 'unhealthy' | 'starting' | 'stopped'
  uptime_seconds: number
}

export interface NativeDeploymentConfig {
  subdomain?: string
  port?: number
  build_command?: string
  start_command?: string
  install_command?: string
  framework?: string
  node_version?: string
  python_version?: string
  go_version?: string
  memory_limit?: number
  cpu_limit?: number
  health_check_path?: string
  auto_scale?: boolean
  min_instances?: number
  max_instances?: number
  env_vars?: Record<string, string>
  always_on?: boolean
  keep_alive_interval?: number
}

export interface NativeDeployment {
  id: string
  project_id: number
  user_id: number
  subdomain: string
  url: string
  preview_url?: string
  status: 'pending' | 'provisioning' | 'building' | 'deploying' | 'running' | 'stopped' | 'failed' | 'deleted'
  container_status: 'healthy' | 'unhealthy' | 'starting' | 'stopped'
  error_message?: string
  container_port: number
  build_command?: string
  start_command?: string
  framework?: string
  memory_limit: number
  cpu_limit: number
  auto_scale: boolean
  min_instances: number
  max_instances: number
  current_instances: number
  always_on: boolean
  always_on_enabled?: string
  last_keep_alive?: string
  keep_alive_interval: number
  total_requests: number
  avg_response_time: number
  uptime_seconds: number
  created_at: string
  updated_at: string
  deployed_at?: string
  build_duration?: number
  deploy_duration?: number
}

export interface DeploymentLog {
  id: number
  deployment_id: string
  timestamp: string
  level: 'debug' | 'info' | 'warn' | 'error'
  source: string
  message: string
  metadata?: string
}

export interface DeploymentMetrics {
  total_requests: number
  avg_response_time: number
  uptime_seconds: number
  bandwidth_used: number
  current_instances: number
  container_status: string
  last_request_at?: string
  last_health_check?: string
  memory_limit: number
  cpu_limit: number
}

// ---------------------------------------------------------------------------
// AI Code Review types (matching backend codereview.ReviewResponse)
// ---------------------------------------------------------------------------

export interface CodeReviewFinding {
  type: string       // bug, security, performance, style, best_practice
  severity: string   // error, warning, info, hint
  line: number
  end_line: number
  column: number
  end_column: number
  message: string
  suggestion: string
  code: string
  rule_id: string
}

export interface CodeReviewMetrics {
  total_lines: number
  code_lines: number
  comment_lines: number
  blank_lines: number
  complexity: number
  security_issues: number
  bug_risks: number
  style_issues: number
}

export interface CodeReviewResponse {
  findings: CodeReviewFinding[]
  summary: string
  score: number
  metrics: CodeReviewMetrics
  suggestions: string[]
  reviewed_at: string
  duration_ms: number
}

// ---------------------------------------------------------------------------
// Package Management types (matching backend packages.go)
// ---------------------------------------------------------------------------

export type PackageRegistry = 'npm' | 'pip' | 'go'

export interface PackageSearchResult {
  name: string
  version: string
  description: string
  homepage?: string
  repository?: string
  license?: string
  downloads?: number
  keywords?: string[]
}

export interface PackageInfo {
  name: string
  version: string
  description: string
  homepage?: string
  repository?: string
  license?: string
  author?: string
  dependencies?: Record<string, string>
  dev_dependencies?: Record<string, string>
  versions?: string[]
  readme?: string
}

export interface InstalledPackage {
  name: string
  version: string
  is_dev: boolean
  type: PackageRegistry
}

export interface PackageSuggestion {
  name: string
  description: string
  category: string
  is_dev: boolean
}

// ---------------------------------------------------------------------------
// Debug System types (matching backend debugging handlers)
// ---------------------------------------------------------------------------

export type DebugSessionStatus = 'pending' | 'running' | 'paused' | 'completed' | 'error'
export type DebugBreakpointType = 'line' | 'conditional' | 'logpoint' | 'exception' | 'function'

export interface DebugSession {
  id: string
  project_id: number
  user_id: number
  file_id: number
  status: DebugSessionStatus
  language: string
  entry_point: string
  working_directory: string
  debug_port: number
  devtools_url?: string
  process_id?: number
  error_message?: string
  started_at?: string
  ended_at?: string
  current_line?: number
  current_file?: string
}

export interface DebugBreakpoint {
  id: string
  session_id: string
  file_id: number
  file_path: string
  line: number
  column: number
  type: DebugBreakpointType
  condition?: string
  log_message?: string
  hit_count: number
  enabled: boolean
  verified: boolean
  breakpoint_id?: string
}

export interface DebugStackFrame {
  id: string
  index: number
  function_name: string
  file_path: string
  line: number
  column: number
  script_id?: string
  is_async: boolean
  scopes?: DebugScope[]
  local_vars?: Record<string, string>
}

export interface DebugScope {
  type: 'local' | 'closure' | 'global' | 'with' | 'catch' | 'block' | 'script'
  name?: string
  start_line?: number
  end_line?: number
  variables?: DebugVariable[]
}

export interface DebugVariable {
  name: string
  value: string
  type: string
  object_id?: string
  has_children: boolean
  children?: DebugVariable[]
  preview?: string
}

export interface DebugWatchExpression {
  id: string
  expression: string
  value?: string
  type?: string
  error?: string
}

export interface DebugEvaluateResult {
  value: string
  type: string
  object_id?: string
  has_children: boolean
  preview?: string
  error?: string
}

export interface DebugEvent {
  type: DebugEventType
  timestamp: string
  data: any
}

export type DebugEventType =
  | 'session_started'
  | 'session_stopped'
  | 'paused'
  | 'resumed'
  | 'stepping'
  | 'breakpoint_hit'
  | 'breakpoint_verified'
  | 'breakpoint_added'
  | 'breakpoint_removed'
  | 'exception'
  | 'output'
  | 'error'

// ---------------------------------------------------------------------------
// Hosting System types (*.apex.app native hosting)
// ---------------------------------------------------------------------------

export type DeploymentStatus =
  | 'pending'
  | 'building'
  | 'deploying'
  | 'live'
  | 'stopped'
  | 'failed'
  | 'maintenance'

export type DomainVerificationStatus =
  | 'pending'
  | 'verifying'
  | 'verified'
  | 'failed'

export type SSLStatus =
  | 'pending'
  | 'provisioning'
  | 'active'
  | 'expired'
  | 'failed'

export interface HostingStatus {
  project_id: number
  deployment_id?: string
  status: DeploymentStatus
  url?: string
  subdomain?: string
  ssl_enabled: boolean
  container_status?: 'healthy' | 'unhealthy' | 'starting' | 'stopped'
  last_deployed_at?: string
  build_duration_ms?: number
  deploy_duration_ms?: number
  error_message?: string
  metrics?: {
    total_requests: number
    avg_response_time_ms: number
    uptime_seconds: number
    bandwidth_bytes: number
  }
  custom_domains?: CustomDomain[]
}

export interface QuickDeployConfig {
  subdomain?: string
  build_command?: string
  start_command?: string
  install_command?: string
  port?: number
  env_vars?: Record<string, string>
  framework?: string
  node_version?: string
  python_version?: string
  go_version?: string
  always_on?: boolean
  health_check_path?: string
  memory_limit?: number
  cpu_limit?: number
}

export interface CustomDomain {
  id: number
  project_id: number
  domain: string
  verification_status: DomainVerificationStatus
  verification_token: string
  ssl_status: SSLStatus
  ssl_expires_at?: string
  is_primary: boolean
  created_at: string
  verified_at?: string
  last_checked_at?: string
  error_message?: string
}

export interface DeploymentEvent {
  id: number
  deployment_id: string
  event_type: 'build_started' | 'build_completed' | 'build_failed' | 'deploy_started' | 'deploy_completed' | 'deploy_failed' | 'health_check' | 'restart' | 'scale' | 'stop'
  timestamp: string
  message: string
  metadata?: Record<string, any>
}

// ---------------------------------------------------------------------------
// Plan Usage Quota types (matching backend usage/tracker.go)
// ---------------------------------------------------------------------------

export type PlanType = 'free' | 'pro' | 'team' | 'enterprise' | 'owner'
export type UsageType = 'projects' | 'storage_bytes' | 'ai_requests' | 'execution_minutes'

export interface PlanLimits {
  projects: number
  storage_bytes: number
  ai_requests: number
  execution_minutes: number
}

export interface UsageWarning {
  type: UsageType
  severity: 'warning' | 'high' | 'critical'
  message: string
  percentage: number
}

export interface CurrentUsageData {
  user_id: number
  plan: PlanType
  unlimited: boolean
  usage: {
    projects: {
      current: number
      limit: number
      percentage: number
      unlimited: boolean
    }
    storage: {
      current: number
      limit: number
      percentage: number
      unlimited: boolean
      current_formatted: string
      limit_formatted: string
    }
    ai_requests: {
      current: number
      limit: number
      percentage: number
      unlimited: boolean
      period: string
      period_start: string
      period_end: string
    }
    execution_minutes: {
      current: number
      limit: number
      percentage: number
      unlimited: boolean
      period: string
    }
  }
  warnings: UsageWarning[]
  cached_at: string
}

export interface UsageHistoryData {
  user_id: number
  days: number
  daily: Array<{
    date: string
    projects: number
    storage_bytes: number
    ai_requests: number
    execution_minutes: number
  }>
  monthly: Array<{
    month: string
    projects: number
    storage_bytes: number
    ai_requests: number
    execution_minutes: number
  }>
}

export interface UsageLimitsData {
  current_plan: PlanType
  current_limits: PlanLimits
  all_plans: Record<PlanType, PlanLimits>
  pricing: Record<PlanType, {
    price_monthly: number
    price_yearly: number
  }>
}

// Build history types
export interface CompletedBuildSummary {
  id: number
  build_id: string
  project_id?: number | null
  project_name: string
  description: string
  status: string
  mode: string
  power_mode: string
  tech_stack: { frontend?: string; backend?: string; database?: string } | null
  files_count: number
  total_cost: number
  progress: number
  duration_ms: number
  created_at: string
  completed_at?: string
}

export interface CompletedBuildDetail extends CompletedBuildSummary {
  files: { path: string; content: string; language: string; size: number; is_new: boolean }[]
  error?: string
}

// Create singleton instance
export const apiService = new ApiService()

// Export for easy importing
export default apiService
