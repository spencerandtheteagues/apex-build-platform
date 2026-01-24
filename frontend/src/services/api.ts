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
  LoginRequest,
  RegisterRequest,
  AuthResponse,
  TokenResponse,
  ApiResponse,
  PaginatedResponse,
  AICapability,
  AIProvider,
} from '@/types'

// Get API URL from environment or use default
const getApiUrl = (): string => {
  // Check for Vite environment variable
  if (import.meta.env.VITE_API_URL) {
    return import.meta.env.VITE_API_URL
  }
  // Fallback to relative URL for development
  return '/api/v1'
}

export class ApiService {
  private client: AxiosInstance
  private baseURL: string

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
    localStorage.setItem('apex_token_expires', tokens.expires_at)
  }

  private clearAuth(): void {
    localStorage.removeItem('apex_access_token')
    localStorage.removeItem('apex_refresh_token')
    localStorage.removeItem('apex_token_expires')
    localStorage.removeItem('apex_user')
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
    const response = await this.client.post<ApiResponse<{ project: Project }>>('/projects', data)
    return response.data.data!.project
  }

  async getProjects(): Promise<Project[]> {
    const response = await this.client.get<ApiResponse<{ projects: Project[] }>>('/projects')
    return response.data.data!.projects
  }

  async getProject(id: number): Promise<Project> {
    const response = await this.client.get<ApiResponse<{ project: Project }>>(`/projects/${id}`)
    return response.data.data!.project
  }

  async updateProject(id: number, data: Partial<Project>): Promise<Project> {
    const response = await this.client.put<ApiResponse<{ project: Project }>>(`/projects/${id}`, data)
    return response.data.data!.project
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
    const response = await this.client.post<ApiResponse<{ file: File }>>(
      `/projects/${projectId}/files`,
      data
    )
    return response.data.data!.file
  }

  async getFiles(projectId: number): Promise<File[]> {
    const response = await this.client.get<ApiResponse<{ files: File[] }>>(
      `/projects/${projectId}/files`
    )
    return response.data.data!.files
  }

  async getFile(id: number): Promise<File> {
    const response = await this.client.get<ApiResponse<{ file: File }>>(`/files/${id}`)
    return response.data.data!.file
  }

  async updateFile(id: number, data: { content: string }): Promise<File> {
    const response = await this.client.put<ApiResponse<{ file: File }>>(`/files/${id}`, data)
    return response.data.data!.file
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
    await this.client.post(`/build/${buildId}/message`, { message })
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

  // Code execution endpoints
  async executeCode(data: {
    project_id: number
    command: string
    language: string
    input?: string
    environment?: Record<string, any>
  }): Promise<Execution> {
    const response = await this.client.post<ApiResponse<{ execution: Execution }>>('/execute', data)
    return response.data.data!.execution
  }

  async getExecution(id: number): Promise<Execution> {
    const response = await this.client.get<ApiResponse<{ execution: Execution }>>(`/execute/${id}`)
    return response.data.data!.execution
  }

  async getExecutionHistory(
    projectId: number,
    limit: number = 50,
    offset: number = 0
  ): Promise<Execution[]> {
    const response = await this.client.get<ApiResponse<{ executions: Execution[] }>>(
      `/execute/history?project_id=${projectId}&limit=${limit}&offset=${offset}`
    )
    return response.data.data!.executions
  }

  async stopExecution(id: number): Promise<void> {
    await this.client.post(`/execute/${id}/stop`)
  }

  // Collaboration endpoints
  async joinCollabRoom(projectId: number): Promise<{
    room_id: string
    websocket_url: string
  }> {
    const response = await this.client.post(`/collab/join/${projectId}`)
    return response.data
  }

  async leaveCollabRoom(roomId: string): Promise<void> {
    await this.client.post(`/collab/leave/${roomId}`)
  }

  async getCollabUsers(roomId: string): Promise<User[]> {
    const response = await this.client.get<ApiResponse<{ users: User[] }>>(`/collab/users/${roomId}`)
    return response.data.data!.users
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
    const response = await this.client.post(`/search/files/${projectId}`, {
      query,
      ...options,
    })
    return response.data.results
  }
}

// Create singleton instance
export const apiService = new ApiService()

// Export for easy importing
export default apiService