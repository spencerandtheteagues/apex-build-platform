// APEX.BUILD State Management
// Zustand-based store with shallow equality selectors for performance optimization

import { create } from 'zustand'
import { devtools, subscribeWithSelector } from 'zustand/middleware'
import { immer } from 'zustand/middleware/immer'
import { useShallow } from 'zustand/react/shallow'
import {
  User,
  Project,
  File,
  AIRequest,
  AIUsage,
  AICapability,
  Execution,
  CollabRoom,
  CursorPosition,
  ChatMessage,
  Theme,
  EditorState,
  Notification,
  AIConversation,
  TerminalSession,
} from '@/types'
import { themes, getTheme } from '@/styles/themes'
import apiService from '@/services/api'
import websocketService from '@/services/websocket'

// Helper function to extract error message from unknown error type
const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message
  }
  if (typeof error === 'object' && error !== null) {
    const errObj = error as Record<string, unknown>
    if (errObj.response && typeof errObj.response === 'object') {
      const response = errObj.response as Record<string, unknown>
      if (response.data && typeof response.data === 'object') {
        const data = response.data as Record<string, unknown>
        if (typeof data.error === 'string') return data.error
        if (typeof data.message === 'string') return data.message
      }
    }
    if (typeof errObj.message === 'string') return errObj.message
  }
  return 'An unknown error occurred'
}

// Auth slice
interface AuthState {
  user: User | null
  isAuthenticated: boolean
  isLoading: boolean  // Legacy - kept for backwards compatibility
  isAuthLoading: boolean  // Domain-specific loading state for auth operations
  error: string | null
}

interface AuthActions {
  login: (usernameOrEmail: string, password: string) => Promise<void>
  register: (data: {
    username: string
    email: string
    password: string
    full_name?: string
  }) => Promise<void>
  logout: () => Promise<void>
  refreshUser: () => Promise<void>
  updateProfile: (data: Partial<User>) => Promise<void>
  clearError: () => void
}

// Projects slice
interface ProjectsState {
  projects: Project[]
  currentProject: Project | null
  isProjectsLoading: boolean  // Domain-specific loading state for project operations
  error: string | null
}

interface ProjectsActions {
  fetchProjects: () => Promise<void>
  createProject: (data: {
    name: string
    description?: string
    language: string
    framework?: string
    is_public?: boolean
    environment?: Record<string, any>
  }) => Promise<Project>
  selectProject: (id: number) => Promise<void>
  setCurrentProject: (project: Project) => void  // Direct setter for current project
  updateProject: (id: number, data: Partial<Project>) => Promise<void>
  deleteProject: (id: number) => Promise<void>
  clearCurrentProject: () => void
}

// Files slice
interface FilesState {
  files: File[]
  fileTree: any[]
  openFiles: File[]
  activeFileId: number | null
  isFilesLoading: boolean  // Domain-specific loading state for file operations
  error: string | null
}

interface FilesActions {
  fetchFiles: (projectId: number) => Promise<void>
  createFile: (projectId: number, data: {
    path: string
    name: string
    type: 'file' | 'directory'
    content?: string
    mime_type?: string
  }) => Promise<File>
  updateFile: (id: number, content: string) => Promise<void>
  hydrateFile: (file: File) => void
  deleteFile: (id: number) => Promise<void>
  openFile: (file: File) => void
  closeFile: (id: number) => void
  setActiveFile: (id: number) => void
  buildFileTree: () => void
}

// Editor slice
interface EditorSliceState extends EditorState {
  isAIGenerating: boolean
  aiConversations: AIConversation[]
}

interface EditorActions {
  setCursorPosition: (line: number, column: number) => void
  setSelection: (startLine: number, startColumn: number, endLine: number, endColumn: number) => void
  toggleAIAssistant: () => void
  setAIProvider: (provider: 'claude' | 'gpt4' | 'gemini' | 'grok' | 'auto') => void
  setTheme: (theme: string) => void
  addAIConversation: (conversation: AIConversation) => void
  updateAIConversation: (id: string, messages: any[]) => void
}

// Collaboration slice
interface CollaborationState {
  room: CollabRoom | null
  connectedUsers: User[]
  collaborationUsers: User[]  // Alias for connectedUsers for component compatibility
  cursors: CursorPosition[]
  chat: ChatMessage[]
  isConnected: boolean
  isConnecting: boolean
}

interface CollaborationActions {
  joinRoom: (projectId: number) => Promise<void>
  leaveRoom: () => Promise<void>
  sendChatMessage: (message: string) => void
  updateCursor: (fileId: number, line: number, column: number) => void
  connect: (projectId: number) => Promise<void>  // Convenience method for joining collaboration
  disconnect: () => Promise<void>  // Convenience method for leaving collaboration
}

// AI slice
interface AIState {
  usage: AIUsage | null
  history: AIRequest[]
  isAILoading: boolean  // Domain-specific loading state for AI operations
}

interface AIActions {
  generateAI: (data: {
    capability: AICapability
    prompt: string
    code?: string
    language?: string
    context?: Record<string, any>
  }) => Promise<any>
  fetchUsage: () => Promise<void>
  fetchHistory: () => Promise<void>
  rateResponse: (requestId: string, rating: number, feedback?: string) => Promise<void>
}

// Notification timeout tracking (stored outside Zustand to avoid serialization issues)
const notificationTimeouts = new Map<string, ReturnType<typeof setTimeout>>()

// UI slice
interface UIState {
  currentTheme: Theme  // Renamed from 'theme' to avoid conflict with EditorState.theme
  sidebarOpen: boolean
  terminalOpen: boolean
  aiPanelOpen: boolean
  loading: boolean
  notifications: Notification[]
  terminals: TerminalSession[]
  activeTerminalId: string | null
}

interface UIActions {
  toggleSidebar: () => void
  toggleTerminal: () => void
  toggleAIPanel: () => void
  setLoading: (loading: boolean) => void
  addNotification: (notification: Omit<Notification, 'id' | 'timestamp'>) => void
  removeNotification: (id: string) => void
  clearNotifications: () => void
  createTerminal: (projectId?: number) => string
  closeTerminal: (id: string) => void
  setActiveTerminal: (id: string) => void
  updateTerminalOutput: (id: string, output: string) => void
}

// Spend slice
interface SpendState {
  dailySpend: number
  monthlySpend: number
  currentBuildSpend: number
  spendEvents: Array<{
    id: number
    agent_role: string
    provider: string
    model: string
    billed_cost: number
    input_tokens: number
    output_tokens: number
    created_at: string
  }>
}

interface SpendActions {
  addSpendEvent: (event: SpendState['spendEvents'][0]) => void
  setDailySpend: (amount: number) => void
  setMonthlySpend: (amount: number) => void
  addBuildSpend: (amount: number) => void
  resetBuildSpend: () => void
}

// Budget slice
interface BudgetCap {
  id: number
  cap_type: 'daily' | 'monthly' | 'per_build'
  limit_usd: number
  action: 'stop' | 'warn'
  is_active: boolean
  project_id?: number
}

interface BudgetState {
  caps: BudgetCap[]
  budgetExceeded: boolean
  budgetWarning: boolean
}

interface BudgetActions {
  setCaps: (caps: BudgetCap[]) => void
  addCap: (cap: BudgetCap) => void
  removeCap: (id: number) => void
  setBudgetExceeded: (exceeded: boolean) => void
  setBudgetWarning: (warning: boolean) => void
  fetchCaps: () => Promise<void>
}

// Diff slice
interface ProposedEdit {
  id: string
  build_id: string
  agent_id: string
  agent_role: string
  file_path: string
  original_content: string
  proposed_content: string
  language: string
  status: 'pending' | 'approved' | 'rejected'
}

interface DiffState {
  proposedEdits: ProposedEdit[]
  diffMode: boolean
}

interface DiffActions {
  setProposedEdits: (edits: ProposedEdit[]) => void
  addProposedEdits: (edits: ProposedEdit[]) => void
  updateEditStatus: (id: string, status: ProposedEdit['status']) => void
  setDiffMode: (enabled: boolean) => void
  clearProposedEdits: () => void
}

// Combined store interface
interface StoreState
  extends AuthState,
    ProjectsState,
    FilesState,
    EditorSliceState,
    CollaborationState,
    AIState,
    UIState,
    SpendState,
    BudgetState,
    DiffState {
      apiService: typeof apiService
    }

interface StoreActions
  extends AuthActions,
    ProjectsActions,
    FilesActions,
    EditorActions,
    CollaborationActions,
    AIActions,
    UIActions,
    SpendActions,
    BudgetActions,
    DiffActions {}

// Create the store
export const useStore = create<StoreState & StoreActions>()(
  devtools(
    subscribeWithSelector(
      immer((set, get) => ({
        // Initial state
        apiService, // Expose apiService

        // Auth
        user: apiService.getCurrentUser(),
        isAuthenticated: apiService.isAuthenticated(),
        isLoading: false,  // Legacy - kept for backwards compatibility
        isAuthLoading: false,  // Domain-specific loading state
        error: null,

        // Projects
        projects: [],
        currentProject: null,
        isProjectsLoading: false,  // Domain-specific loading state

        // Files
        files: [],
        fileTree: [],
        openFiles: [],
        activeFileId: null,
        isFilesLoading: false,  // Domain-specific loading state

        // Editor
        activeFile: undefined,
        cursorPosition: { line: 1, column: 1 },
        selection: undefined,
        isAIAssistantOpen: false,
        aiProvider: 'auto' as const,
        theme: 'cyberpunk', // Initial editor theme
        isAIGenerating: false,
        aiConversations: [],

        // Collaboration
        room: null,
        connectedUsers: [],
        collaborationUsers: [],  // Alias for connectedUsers
        cursors: [],
        chat: [],
        isConnected: false,
        isConnecting: false,

        // AI
        usage: null,
        history: [],
        isAILoading: false,  // Domain-specific loading state

        // UI
        currentTheme: getTheme('cyberpunk'),
        sidebarOpen: true,
        terminalOpen: false,
        aiPanelOpen: false,
        loading: false,
        notifications: [],
        terminals: [],
        activeTerminalId: null,

        // Spend
        dailySpend: 0,
        monthlySpend: 0,
        currentBuildSpend: 0,
        spendEvents: [],

        // Budget
        caps: [],
        budgetExceeded: false,
        budgetWarning: false,

        // Diff
        proposedEdits: [],
        diffMode: false,

        // Auth actions
        login: async (usernameOrEmail: string, password: string) => {
          set((state) => {
            state.isLoading = true  // Legacy
            state.isAuthLoading = true
            state.error = null
          })

          try {
            // Always send as username — backend detects @ and looks up by email
            const response = await apiService.login(
              { username: usernameOrEmail, password }
            )
            const user = response.user as User

            set((state) => {
              state.user = user
              state.isAuthenticated = true
              state.isLoading = false  // Legacy
              state.isAuthLoading = false
            })

            get().addNotification({
              type: 'success',
              title: 'Login Successful',
              message: `Welcome back, ${user.username}!`,
            })
          } catch (error: unknown) {
            set((state) => {
              state.error = getErrorMessage(error)
              state.isLoading = false  // Legacy
              state.isAuthLoading = false
            })

            get().addNotification({
              type: 'error',
              title: 'Login Failed',
              message: getErrorMessage(error),
            })
            throw error
          }
        },

        register: async (data) => {
          set((state) => {
            state.isLoading = true  // Legacy
            state.isAuthLoading = true
            state.error = null
          })

          try {
            const response = await apiService.register(data)
            const user = response.user as User

            set((state) => {
              state.user = user
              state.isAuthenticated = true
              state.isLoading = false  // Legacy
              state.isAuthLoading = false
            })

            get().addNotification({
              type: 'success',
              title: 'Registration Successful',
              message: `Welcome to APEX.BUILD, ${user.username}!`,
            })
          } catch (error: unknown) {
            set((state) => {
              state.error = getErrorMessage(error)
              state.isLoading = false  // Legacy
              state.isAuthLoading = false
            })

            get().addNotification({
              type: 'error',
              title: 'Registration Failed',
              message: getErrorMessage(error),
            })
            throw error
          }
        },

        logout: async () => {
          try {
            await apiService.logout()
            websocketService.disconnect()

            set((state) => {
              state.user = null
              state.isAuthenticated = false
              state.projects = []
              state.currentProject = null
              state.files = []
              state.openFiles = []
              state.room = null
              state.connectedUsers = []
              state.isConnected = false
            })

            get().addNotification({
              type: 'info',
              title: 'Logged Out',
              message: 'You have been logged out successfully.',
            })
          } catch (error: unknown) {
            console.error('Logout error:', error)
          }
        },

        refreshUser: async () => {
          try {
            const user = await apiService.getUserProfile()
            set((state) => {
              state.user = user
            })
          } catch (error: unknown) {
            console.error('Failed to refresh user:', error)
          }
        },

        updateProfile: async (data) => {
          try {
            const user = await apiService.updateUserProfile(data)
            set((state) => {
              state.user = user
            })

            get().addNotification({
              type: 'success',
              title: 'Profile Updated',
              message: 'Your profile has been updated successfully.',
            })
          } catch (error: unknown) {
            get().addNotification({
              type: 'error',
              title: 'Update Failed',
              message: getErrorMessage(error),
            })
            throw error
          }
        },

        clearError: () => {
          set((state) => {
            state.error = null
          })
        },

        // Projects actions
        fetchProjects: async () => {
          set((state) => {
            state.isLoading = true  // Legacy
            state.isProjectsLoading = true
          })

          try {
            const projects = await apiService.getProjects()
            set((state) => {
              state.projects = projects
              state.isLoading = false  // Legacy
              state.isProjectsLoading = false
            })
          } catch (error: unknown) {
            set((state) => {
              state.error = getErrorMessage(error)
              state.isLoading = false  // Legacy
              state.isProjectsLoading = false
            })
          }
        },

        createProject: async (data) => {
          try {
            const project = await apiService.createProject(data)
            set((state) => {
              state.projects.unshift(project)
            })

            get().addNotification({
              type: 'success',
              title: 'Project Created',
              message: `${project.name} has been created successfully.`,
            })

            return project
          } catch (error: unknown) {
            get().addNotification({
              type: 'error',
              title: 'Creation Failed',
              message: getErrorMessage(error),
            })
            throw error
          }
        },

        selectProject: async (id: number) => {
          set((state) => {
            state.isLoading = true  // Legacy
            state.isProjectsLoading = true
          })

          try {
            const project = await apiService.getProject(id)
            set((state) => {
              state.currentProject = project
              state.isLoading = false  // Legacy
              state.isProjectsLoading = false
            })

            // Fetch files for the project
            await get().fetchFiles(id)

            get().addNotification({
              type: 'info',
              title: 'Project Loaded',
              message: `${project.name} is now active.`,
            })
          } catch (error: unknown) {
            set((state) => {
              state.error = getErrorMessage(error)
              state.isLoading = false  // Legacy
              state.isProjectsLoading = false
            })

            get().addNotification({
              type: 'error',
              title: 'Failed to Load Project',
              message: getErrorMessage(error),
            })
          }
        },

        // Direct setter for current project (synchronous, no API call)
        setCurrentProject: (project: Project) => {
          set((state) => {
            const changedProject = state.currentProject?.id !== project.id
            state.currentProject = project
            if (changedProject) {
              state.files = []
              state.fileTree = []
              state.openFiles = []
              state.activeFileId = null
              state.isLoading = true  // Legacy
              state.isFilesLoading = true
            }
          })
          // Fetch files for the project
          get().fetchFiles(project.id)
        },

        updateProject: async (id: number, data) => {
          try {
            const project = await apiService.updateProject(id, data)
            set((state) => {
              const index = state.projects.findIndex((p) => p.id === id)
              if (index !== -1) {
                state.projects[index] = project
              }
              if (state.currentProject?.id === id) {
                state.currentProject = project
              }
            })

            get().addNotification({
              type: 'success',
              title: 'Project Updated',
              message: `${project.name} has been updated.`,
            })
          } catch (error: unknown) {
            get().addNotification({
              type: 'error',
              title: 'Update Failed',
              message: getErrorMessage(error),
            })
            throw error
          }
        },

        deleteProject: async (id: number) => {
          try {
            await apiService.deleteProject(id)
            set((state) => {
              state.projects = state.projects.filter((p) => p.id !== id)
              if (state.currentProject?.id === id) {
                state.currentProject = null
                state.files = []
                state.openFiles = []
              }
            })

            get().addNotification({
              type: 'success',
              title: 'Project Deleted',
              message: 'Project has been deleted successfully.',
            })
          } catch (error: unknown) {
            get().addNotification({
              type: 'error',
              title: 'Deletion Failed',
              message: getErrorMessage(error),
            })
            throw error
          }
        },

        clearCurrentProject: () => {
          set((state) => {
            state.currentProject = null
            state.files = []
            state.openFiles = []
            state.activeFileId = null
          })
        },

        // Files actions
        fetchFiles: async (projectId: number) => {
          set((state) => {
            state.isLoading = true  // Legacy
            state.isFilesLoading = true
          })

          try {
            const files = await apiService.getFiles(projectId)
            set((state) => {
              state.files = files
              state.isLoading = false  // Legacy
              state.isFilesLoading = false
            })
            get().buildFileTree()
          } catch (error: unknown) {
            set((state) => {
              state.error = getErrorMessage(error)
              state.isLoading = false  // Legacy
              state.isFilesLoading = false
            })
          }
        },

        createFile: async (projectId: number, data) => {
          try {
            const file = await apiService.createFile(projectId, data)
            set((state) => {
              state.files.push(file)
            })
            get().buildFileTree()

            get().addNotification({
              type: 'success',
              title: 'File Created',
              message: `${file.name} has been created.`,
            })

            return file
          } catch (error: unknown) {
            get().addNotification({
              type: 'error',
              title: 'Creation Failed',
              message: getErrorMessage(error),
            })
            throw error
          }
        },

        updateFile: async (id: number, content: string) => {
          try {
            const file = await apiService.updateFile(id, { content })
            set((state) => {
              const index = state.files.findIndex((f) => f.id === id)
              if (index !== -1) {
                state.files[index] = file
              }
              const openIndex = state.openFiles.findIndex((f) => f.id === id)
              if (openIndex !== -1) {
                state.openFiles[openIndex] = file
              }
            })

            // Send file change to collaboration room
            if (websocketService.isConnected()) {
              websocketService.sendFileChange(id, content, 1, 1)
            }
          } catch (error: unknown) {
            get().addNotification({
              type: 'error',
              title: 'Save Failed',
              message: getErrorMessage(error),
            })
            throw error
          }
        },

        hydrateFile: (file: File) => {
          set((state) => {
            const index = state.files.findIndex((f) => f.id === file.id)
            if (index !== -1) {
              state.files[index] = { ...state.files[index], ...file }
            } else {
              state.files.push(file)
            }
            const openIndex = state.openFiles.findIndex((f) => f.id === file.id)
            if (openIndex !== -1) {
              state.openFiles[openIndex] = { ...state.openFiles[openIndex], ...file }
            }
          })
        },

        deleteFile: async (id: number) => {
          try {
            await apiService.deleteFile(id)
            set((state) => {
              state.files = state.files.filter((f) => f.id !== id)
              state.openFiles = state.openFiles.filter((f) => f.id !== id)
              if (state.activeFileId === id) {
                state.activeFileId = state.openFiles[0]?.id || null
              }
            })
            get().buildFileTree()

            get().addNotification({
              type: 'success',
              title: 'File Deleted',
              message: 'File has been deleted successfully.',
            })
          } catch (error: unknown) {
            get().addNotification({
              type: 'error',
              title: 'Deletion Failed',
              message: getErrorMessage(error),
            })
            throw error
          }
        },

        openFile: (file: File) => {
          set((state) => {
            if (!state.openFiles.find((f) => f.id === file.id)) {
              state.openFiles.push(file)
            }
            state.activeFileId = file.id
            state.activeFile = file
          })
        },

        closeFile: (id: number) => {
          set((state) => {
            state.openFiles = state.openFiles.filter((f) => f.id !== id)
            if (state.activeFileId === id) {
              state.activeFileId = state.openFiles[0]?.id || null
              state.activeFile = state.openFiles[0]
            }
          })
        },

        setActiveFile: (id: number) => {
          set((state) => {
            state.activeFileId = id
            state.activeFile = state.openFiles.find((f) => f.id === id)
          })
        },

        buildFileTree: () => {
          // Build file tree structure for UI
          const { files } = get()
          // Implementation for building nested file tree would go here
          // For now, simple structure
          set((state) => {
            state.fileTree = files
          })
        },

        // Editor actions
        setCursorPosition: (line: number, column: number) => {
          set((state) => {
            state.cursorPosition = { line, column }
          })

          // Send cursor update to collaboration
          const { activeFileId } = get()
          if (activeFileId && websocketService.isConnected()) {
            websocketService.sendCursorUpdate(activeFileId, line, column)
          }
        },

        setSelection: (startLine: number, startColumn: number, endLine: number, endColumn: number) => {
          set((state) => {
            state.selection = { startLine, startColumn, endLine, endColumn }
          })
        },

        toggleAIAssistant: () => {
          set((state) => {
            state.isAIAssistantOpen = !state.isAIAssistantOpen
            state.aiPanelOpen = state.isAIAssistantOpen
          })
        },

        setAIProvider: (provider) => {
          set((state) => {
            state.aiProvider = provider
          })
        },

        setTheme: (themeId: string) => {
          const themeConfig = getTheme(themeId)
          set((state) => {
            state.theme = themeId
            state.currentTheme = themeConfig
          })
        },

        addAIConversation: (conversation: AIConversation) => {
          set((state) => {
            state.aiConversations.push(conversation)
          })
        },

        updateAIConversation: (id: string, messages: any[]) => {
          set((state) => {
            const conversation = state.aiConversations.find((c) => c.id === id)
            if (conversation) {
              conversation.messages = messages
              conversation.updated_at = new Date().toISOString()
            }
          })
        },

        // Collaboration actions
        joinRoom: async (projectId: number) => {
          set((state) => {
            state.isConnecting = true
          })

          try {
            const roomData = await apiService.joinCollabRoom(projectId)

            if (!websocketService.isConnected()) {
              const token = localStorage.getItem('apex_access_token')
              if (token) {
                await websocketService.connect(token)
              }
            }

            await websocketService.joinRoom(roomData.room_id)

            set((state) => {
              state.room = { id: 0, room_id: roomData.room_id, project_id: projectId } as CollabRoom
              state.isConnected = true
              state.isConnecting = false
            })

            get().addNotification({
              type: 'success',
              title: 'Collaboration Started',
              message: 'You are now collaborating in real-time!',
            })
          } catch (error: unknown) {
            set((state) => {
              state.isConnecting = false
            })

            get().addNotification({
              type: 'error',
              title: 'Collaboration Failed',
              message: getErrorMessage(error),
            })
          }
        },

        leaveRoom: async () => {
          try {
            await websocketService.leaveRoom()
            set((state) => {
              state.room = null
              state.connectedUsers = []
              state.collaborationUsers = []
              state.cursors = []
              state.isConnected = false
            })
          } catch (error: unknown) {
            console.error('Failed to leave room:', error)
          }
        },

        sendChatMessage: (message: string) => {
          if (websocketService.isConnected()) {
            websocketService.sendChatMessage(message)
          }
        },

        updateCursor: (fileId: number, line: number, column: number) => {
          if (websocketService.isConnected()) {
            websocketService.sendCursorUpdate(fileId, line, column)
          }
        },

        // Convenience methods for connecting/disconnecting collaboration
        connect: async (projectId: number) => {
          await get().joinRoom(projectId)
        },

        disconnect: async () => {
          await get().leaveRoom()
        },

        // AI actions
        generateAI: async (data) => {
          set((state) => {
            state.isLoading = true  // Legacy
            state.isAILoading = true
            state.isAIGenerating = true
          })

          try {
            const response = await apiService.generateAI(data)

            set((state) => {
              state.isLoading = false  // Legacy
              state.isAILoading = false
              state.isAIGenerating = false
            })

            // Broadcast AI request to collaboration room
            if (websocketService.isConnected()) {
              websocketService.broadcastAIRequest(response.request_id, data.capability)
            }

            return response
          } catch (error: unknown) {
            set((state) => {
              state.isLoading = false  // Legacy
              state.isAILoading = false
              state.isAIGenerating = false
            })

            get().addNotification({
              type: 'error',
              title: 'AI Request Failed',
              message: getErrorMessage(error),
            })
            throw error
          }
        },

        fetchUsage: async () => {
          try {
            const usage = await apiService.getAIUsage()
            set((state) => {
              state.usage = usage
            })
          } catch (error: unknown) {
            console.error('Failed to fetch AI usage:', error)
          }
        },

        fetchHistory: async () => {
          try {
            const history = await apiService.getAIHistory()
            set((state) => {
              state.history = history
            })
          } catch (error: unknown) {
            console.error('Failed to fetch AI history:', error)
          }
        },

        rateResponse: async (requestId: string, rating: number, feedback?: string) => {
          try {
            await apiService.rateAIResponse(requestId, rating, feedback)

            get().addNotification({
              type: 'success',
              title: 'Rating Submitted',
              message: 'Thank you for your feedback!',
            })
          } catch (error: unknown) {
            get().addNotification({
              type: 'error',
              title: 'Rating Failed',
              message: getErrorMessage(error),
            })
          }
        },

        // UI actions
        toggleSidebar: () => {
          set((state) => {
            state.sidebarOpen = !state.sidebarOpen
          })
        },

        toggleTerminal: () => {
          set((state) => {
            state.terminalOpen = !state.terminalOpen
          })
        },

        toggleAIPanel: () => {
          set((state) => {
            state.aiPanelOpen = !state.aiPanelOpen
          })
        },

        setLoading: (loading: boolean) => {
          set((state) => {
            state.loading = loading
          })
        },

        addNotification: (notification) => {
          const id = Math.random().toString(36).substr(2, 9)
          const timestamp = new Date().toISOString()

          set((state) => {
            state.notifications.push({
              ...notification,
              id,
              timestamp,
            })
          })

          // Auto-remove after duration with proper timeout tracking for cleanup
          const duration = notification.duration || 5000
          const timeoutId = setTimeout(() => {
            get().removeNotification(id)
          }, duration)

          // Store timeout ID for potential cleanup
          notificationTimeouts.set(id, timeoutId)
        },

        removeNotification: (id: string) => {
          // Clear the timeout to prevent memory leaks
          const timeoutId = notificationTimeouts.get(id)
          if (timeoutId) {
            clearTimeout(timeoutId)
            notificationTimeouts.delete(id)
          }

          set((state) => {
            state.notifications = state.notifications.filter((n) => n.id !== id)
          })
        },

        clearNotifications: () => {
          // Clear all timeouts before clearing notifications
          notificationTimeouts.forEach((timeoutId) => {
            clearTimeout(timeoutId)
          })
          notificationTimeouts.clear()

          set((state) => {
            state.notifications = []
          })
        },

        createTerminal: (projectId?: number) => {
          const id = Math.random().toString(36).substr(2, 9)
          const terminal: TerminalSession = {
            id,
            name: `Terminal ${get().terminals.length + 1}`,
            status: 'running',
            output: [],
            input: '',
            project_id: projectId,
            created_at: new Date().toISOString(),
          }

          set((state) => {
            state.terminals.push(terminal)
            state.activeTerminalId = id
            state.terminalOpen = true
          })

          return id
        },

        closeTerminal: (id: string) => {
          set((state) => {
            state.terminals = state.terminals.filter((t) => t.id !== id)
            if (state.activeTerminalId === id) {
              state.activeTerminalId = state.terminals[0]?.id || null
            }
            if (state.terminals.length === 0) {
              state.terminalOpen = false
            }
          })
        },

        setActiveTerminal: (id: string) => {
          set((state) => {
            state.activeTerminalId = id
          })
        },

        updateTerminalOutput: (id: string, output: string) => {
          set((state) => {
            const terminal = state.terminals.find((t) => t.id === id)
            if (terminal) {
              terminal.output.push(output)
            }
          })
        },

        // Spend actions
        addSpendEvent: (event) => {
          set((state) => {
            state.spendEvents = [...state.spendEvents.slice(-99), event]
            state.currentBuildSpend += event.billed_cost
          })
        },

        setDailySpend: (amount: number) => {
          set((state) => { state.dailySpend = amount })
        },

        setMonthlySpend: (amount: number) => {
          set((state) => { state.monthlySpend = amount })
        },

        addBuildSpend: (amount: number) => {
          set((state) => { state.currentBuildSpend += amount })
        },

        resetBuildSpend: () => {
          set((state) => {
            state.currentBuildSpend = 0
            state.spendEvents = []
          })
        },

        // Budget actions
        setCaps: (caps) => {
          set((state) => { state.caps = caps })
        },

        addCap: (cap) => {
          set((state) => { state.caps.push(cap) })
        },

        removeCap: (id: number) => {
          set((state) => {
            state.caps = state.caps.filter(c => c.id !== id)
          })
        },

        setBudgetExceeded: (exceeded: boolean) => {
          set((state) => { state.budgetExceeded = exceeded })
        },

        setBudgetWarning: (warning: boolean) => {
          set((state) => { state.budgetWarning = warning })
        },

        fetchCaps: async () => {
          try {
            const response = await apiService.client.get('/budget/caps')
            set((state) => { state.caps = response.data?.caps || [] })
          } catch (error) {
            console.error('Failed to fetch budget caps:', error)
          }
        },

        // Diff actions
        setProposedEdits: (edits) => {
          set((state) => { state.proposedEdits = edits })
        },

        addProposedEdits: (edits) => {
          set((state) => {
            state.proposedEdits = [...state.proposedEdits, ...edits]
          })
        },

        updateEditStatus: (id: string, status: 'pending' | 'approved' | 'rejected') => {
          set((state) => {
            const edit = state.proposedEdits.find(e => e.id === id)
            if (edit) edit.status = status
          })
        },

        setDiffMode: (enabled: boolean) => {
          set((state) => { state.diffMode = enabled })
        },

        clearProposedEdits: () => {
          set((state) => { state.proposedEdits = [] })
        },
      }))
    ),
    {
      name: 'apex-build-store',
    }
  )
)

// ============================================================================
// OPTIMIZED SELECTORS WITH SHALLOW EQUALITY
// These selectors use useShallow to prevent unnecessary re-renders
// ============================================================================

// Individual atomic selectors for fine-grained subscriptions
export const useUser = () => useStore((state) => state.user)
export const useIsAuthenticated = () => useStore((state) => state.isAuthenticated)
export const useIsLoading = () => useStore((state) => state.isLoading)  // Legacy - use domain-specific selectors
export const useIsAuthLoading = () => useStore((state) => state.isAuthLoading)
export const useIsProjectsLoading = () => useStore((state) => state.isProjectsLoading)
export const useIsFilesLoading = () => useStore((state) => state.isFilesLoading)
export const useIsAILoading = () => useStore((state) => state.isAILoading)
export const useError = () => useStore((state) => state.error)
export const useCurrentProject = () => useStore((state) => state.currentProject)
export const useProjects = () => useStore((state) => state.projects)
export const useFiles = () => useStore((state) => state.files)
export const useOpenFiles = () => useStore((state) => state.openFiles)
export const useActiveFileId = () => useStore((state) => state.activeFileId)
export const useActiveFile = () => useStore((state) => state.activeFile)
export const useCursorPosition = () => useStore((state) => state.cursorPosition)
export const useCurrentTheme = () => useStore((state) => state.currentTheme)
export const useSidebarOpen = () => useStore((state) => state.sidebarOpen)
export const useTerminalOpen = () => useStore((state) => state.terminalOpen)
export const useAiPanelOpen = () => useStore((state) => state.aiPanelOpen)
export const useNotifications = () => useStore((state) => state.notifications)
export const useTerminals = () => useStore((state) => state.terminals)
export const useActiveTerminalId = () => useStore((state) => state.activeTerminalId)
export const useIsConnected = () => useStore((state) => state.isConnected)
export const useCollaborationUsers = () => useStore((state) => state.collaborationUsers)
export const useConnectedUsers = () => useStore((state) => state.connectedUsers)
export const useIsAIGenerating = () => useStore((state) => state.isAIGenerating)
export const useAIUsage = () => useStore((state) => state.usage)
export const useAIHistory = () => useStore((state) => state.history)

// Action selectors (these don't need shallow comparison since functions are stable)
export const useLogin = () => useStore((state) => state.login)
export const useRegister = () => useStore((state) => state.register)
export const useLogout = () => useStore((state) => state.logout)
export const useRefreshUser = () => useStore((state) => state.refreshUser)
export const useUpdateProfile = () => useStore((state) => state.updateProfile)
export const useClearError = () => useStore((state) => state.clearError)
export const useFetchProjects = () => useStore((state) => state.fetchProjects)
export const useCreateProject = () => useStore((state) => state.createProject)
export const useSelectProject = () => useStore((state) => state.selectProject)
export const useSetCurrentProject = () => useStore((state) => state.setCurrentProject)
export const useUpdateProject = () => useStore((state) => state.updateProject)
export const useDeleteProject = () => useStore((state) => state.deleteProject)
export const useFetchFiles = () => useStore((state) => state.fetchFiles)
export const useCreateFile = () => useStore((state) => state.createFile)
export const useUpdateFile = () => useStore((state) => state.updateFile)
export const useDeleteFile = () => useStore((state) => state.deleteFile)
export const useOpenFileAction = () => useStore((state) => state.openFile)
export const useCloseFile = () => useStore((state) => state.closeFile)
export const useSetActiveFile = () => useStore((state) => state.setActiveFile)
export const useToggleSidebar = () => useStore((state) => state.toggleSidebar)
export const useToggleTerminal = () => useStore((state) => state.toggleTerminal)
export const useToggleAIPanel = () => useStore((state) => state.toggleAIPanel)
export const useAddNotification = () => useStore((state) => state.addNotification)
export const useRemoveNotification = () => useStore((state) => state.removeNotification)
export const useSetTheme = () => useStore((state) => state.setTheme)
export const useGenerateAI = () => useStore((state) => state.generateAI)
export const useConnect = () => useStore((state) => state.connect)
export const useDisconnect = () => useStore((state) => state.disconnect)
export const useCreateTerminal = () => useStore((state) => state.createTerminal)
export const useCloseTerminal = () => useStore((state) => state.closeTerminal)
export const useSetActiveTerminal = () => useStore((state) => state.setActiveTerminal)
export const useDailySpend = () => useStore((state) => state.dailySpend)
export const useMonthlySpend = () => useStore((state) => state.monthlySpend)
export const useCurrentBuildSpend = () => useStore((state) => state.currentBuildSpend)
export const useSpendEvents = () => useStore((state) => state.spendEvents)
export const useBudgetCaps = () => useStore((state) => state.caps)
export const useBudgetExceeded = () => useStore((state) => state.budgetExceeded)
export const useBudgetWarning = () => useStore((state) => state.budgetWarning)
export const useProposedEdits = () => useStore((state) => state.proposedEdits)
export const useDiffMode = () => useStore((state) => state.diffMode)
export const useSetDiffMode = () => useStore((state) => state.setDiffMode)
export const useFetchCaps = () => useStore((state) => state.fetchCaps)
export const useResetBuildSpend = () => useStore((state) => state.resetBuildSpend)

// Composite selectors with shallow equality for grouped state
export const useAuth = () => useStore(
  useShallow((state) => ({
    user: state.user,
    isAuthenticated: state.isAuthenticated,
    isLoading: state.isAuthLoading,  // Use domain-specific loading state
    isAuthLoading: state.isAuthLoading,
    error: state.error,
    login: state.login,
    register: state.register,
    logout: state.logout,
    refreshUser: state.refreshUser,
    updateProfile: state.updateProfile,
    clearError: state.clearError,
  }))
)

export const useProjectsState = () => useStore(
  useShallow((state) => ({
    projects: state.projects,
    currentProject: state.currentProject,
    isLoading: state.isProjectsLoading,  // Use domain-specific loading state
    isProjectsLoading: state.isProjectsLoading,
    fetchProjects: state.fetchProjects,
    createProject: state.createProject,
    selectProject: state.selectProject,
    setCurrentProject: state.setCurrentProject,
    updateProject: state.updateProject,
    deleteProject: state.deleteProject,
  }))
)

export const useFilesState = () => useStore(
  useShallow((state) => ({
    files: state.files,
    openFiles: state.openFiles,
    activeFileId: state.activeFileId,
    activeFile: state.activeFile,
    isLoading: state.isFilesLoading,  // Use domain-specific loading state
    isFilesLoading: state.isFilesLoading,
    fetchFiles: state.fetchFiles,
    createFile: state.createFile,
    updateFile: state.updateFile,
    deleteFile: state.deleteFile,
    openFile: state.openFile,
    closeFile: state.closeFile,
    setActiveFile: state.setActiveFile,
  }))
)

export const useEditor = () => useStore(
  useShallow((state) => ({
    cursorPosition: state.cursorPosition,
    selection: state.selection,
    isAIAssistantOpen: state.isAIAssistantOpen,
    aiProvider: state.aiProvider,
    theme: state.currentTheme,  // Return Theme object for component compatibility
    themeId: state.theme,       // Also expose raw theme string
    isAIGenerating: state.isAIGenerating,
    setCursorPosition: state.setCursorPosition,
    setSelection: state.setSelection,
    toggleAIAssistant: state.toggleAIAssistant,
    setAIProvider: state.setAIProvider,
    setTheme: state.setTheme,
  }))
)

export const useCollaboration = () => useStore(
  useShallow((state) => ({
    room: state.room,
    connectedUsers: state.connectedUsers,
    collaborationUsers: state.collaborationUsers,
    cursors: state.cursors,
    chat: state.chat,
    isConnected: state.isConnected,
    isConnecting: state.isConnecting,
    joinRoom: state.joinRoom,
    leaveRoom: state.leaveRoom,
    sendChatMessage: state.sendChatMessage,
    updateCursor: state.updateCursor,
    connect: state.connect,
    disconnect: state.disconnect,
  }))
)

export const useAI = () => useStore(
  useShallow((state) => ({
    usage: state.usage,
    history: state.history,
    isLoading: state.isAILoading,  // Use domain-specific loading state
    isAILoading: state.isAILoading,
    generateAI: state.generateAI,
    fetchUsage: state.fetchUsage,
    fetchHistory: state.fetchHistory,
    rateResponse: state.rateResponse,
  }))
)

export const useUI = () => useStore(
  useShallow((state) => ({
    theme: state.currentTheme,
    currentTheme: state.currentTheme,
    sidebarOpen: state.sidebarOpen,
    terminalOpen: state.terminalOpen,
    aiPanelOpen: state.aiPanelOpen,
    loading: state.loading,
    notifications: state.notifications,
    terminals: state.terminals,
    activeTerminalId: state.activeTerminalId,
    toggleSidebar: state.toggleSidebar,
    toggleTerminal: state.toggleTerminal,
    toggleAIPanel: state.toggleAIPanel,
    setLoading: state.setLoading,
    addNotification: state.addNotification,
    removeNotification: state.removeNotification,
    createTerminal: state.createTerminal,
    closeTerminal: state.closeTerminal,
    setActiveTerminal: state.setActiveTerminal,
  }))
)

export default useStore
