// APEX.BUILD Frontend Types
// Matching the Go backend models for type safety

export interface User {
  id: number
  username: string
  email: string
  full_name?: string
  avatar_url?: string
  is_active: boolean
  is_verified: boolean
  is_admin?: boolean
  is_super_admin?: boolean
  subscription_type: 'free' | 'pro' | 'team'
  subscription_end?: string
  monthly_ai_requests: number
  monthly_ai_cost: number
  preferred_theme: 'cyberpunk' | 'matrix' | 'synthwave' | 'neonCity'
  preferred_ai: 'auto' | 'claude' | 'gpt4' | 'gemini' | 'grok' | 'ollama'
  created_at: string
  updated_at: string
}

export interface Project {
  id: number
  name: string
  description?: string
  language: string
  framework?: string
  owner_id: number
  owner?: User
  is_public: boolean
  is_archived: boolean
  root_directory: string
  entry_point?: string
  environment?: Record<string, any>
  dependencies?: Record<string, any>
  build_config?: Record<string, any>
  environment_config?: string // JSON-encoded EnvironmentConfig (Nix-like - Replit parity)
  provisioned_database_id?: number // Auto-provisioned PostgreSQL database ID (Replit parity)
  collab_room_id?: number
  created_at: string
  updated_at: string
}

export interface File {
  id: number
  project_id: number
  project?: Project
  path: string
  name: string
  type: 'file' | 'directory'
  mime_type?: string
  content: string
  size: number
  hash?: string
  version: number
  last_edit_by?: number
  last_editor?: User
  is_locked: boolean
  locked_by?: number
  locked_at?: string
  created_at: string
  updated_at: string
}

export interface AIRequest {
  id: number
  request_id: string
  user_id: number
  user?: User
  project_id?: number
  project?: Project
  provider: AIProvider
  capability: AICapability
  prompt: string
  code?: string
  language?: string
  context?: Record<string, any>
  response: string
  tokens_used: number
  cost: number
  duration: number
  status: 'pending' | 'completed' | 'failed'
  error_msg?: string
  user_rating?: number
  user_feedback?: string
  created_at: string
  updated_at: string
}

export type AICapability =
  | 'code_generation'
  | 'code_review'
  | 'code_completion'
  | 'debugging'
  | 'explanation'
  | 'refactoring'
  | 'testing'
  | 'documentation'

export type AIProvider = 'claude' | 'gpt4' | 'gemini' | 'grok' | 'ollama'

export interface AIUsage {
  total_requests: number
  total_cost: number
  total_tokens: number
  by_provider: Record<AIProvider, ProviderUsage>
  recent_requests: AIRequest[]
}

export interface ProviderUsage {
  requests: number
  cost: number
  tokens: number
}

export interface Execution {
  id: number
  execution_id: string
  project_id: number
  project?: Project
  user_id: number
  user?: User
  command: string
  language: string
  environment?: Record<string, any>
  input?: string
  output: string
  error_out?: string
  exit_code: number
  duration: number
  status: 'running' | 'completed' | 'failed' | 'timeout'
  started_at: string
  completed_at?: string
  memory_used: number
  cpu_time: number
  created_at: string
  updated_at: string
}

export interface CollabRoom {
  id: number
  room_id: string
  project_id: number
  project?: Project
  is_active: boolean
  max_users: number
  current_users: number
  allow_anonymous: boolean
  require_auth: boolean
  password?: string
  created_at: string
  updated_at: string
}

export interface CursorPosition {
  id: number
  room_id: number
  room?: CollabRoom
  user_id: number
  user?: User
  file_id: number
  file?: File
  line: number
  column: number
  selection_start_line?: number
  selection_start_column?: number
  selection_end_line?: number
  selection_end_column?: number
  is_active: boolean
  last_active: string
  created_at: string
  updated_at: string
}

export interface ChatMessage {
  id: number
  room_id: number
  room?: CollabRoom
  user_id: number
  user?: User
  message: string
  type: 'text' | 'system' | 'code' | 'file'
  is_edited: boolean
  edited_at?: string
  created_at: string
  updated_at: string
}

// Authentication types
export interface LoginRequest {
  username: string
  password: string
}

export interface RegisterRequest {
  username: string
  email: string
  password: string
  full_name?: string
}

export interface TokenResponse {
  access_token: string
  refresh_token: string
  access_token_expires_at: string
  refresh_token_expires_at?: string
  token_type: string
}

export interface AuthResponse {
  message: string
  user: Partial<User>
  tokens: TokenResponse
}

// API Response types
export interface ApiResponse<T = any> {
  message?: string
  data?: T
  error?: string
  errors?: Record<string, string[]>
}

export interface PaginatedResponse<T = any> {
  data: T[]
  total: number
  page: number
  per_page: number
  total_pages: number
}

// UI State types
export interface Theme {
  id: string
  name: string
  colors: {
    primary: string
    secondary: string
    accent: string
    background: string
    surface: string
    text: string
    textSecondary: string
    error: string
    warning: string
    success: string
    info: string
  }
  effects: {
    glassMorphism: string
    neonGlow: string
    holographic: string
  }
}

export interface EditorState {
  activeFile?: File
  openFiles: File[]
  cursorPosition: { line: number; column: number }
  selection?: {
    startLine: number
    startColumn: number
    endLine: number
    endColumn: number
  }
  isAIAssistantOpen: boolean
  aiProvider: AIProvider | 'auto'
  theme: string
}

export interface AppState {
  user?: User
  isAuthenticated: boolean
  currentProject?: Project
  projects: Project[]
  files: File[]
  editor: EditorState
  collaboration: {
    room?: CollabRoom
    connectedUsers: User[]
    cursors: CursorPosition[]
    chat: ChatMessage[]
  }
  ui: {
    sidebarOpen: boolean
    terminalOpen: boolean
    aiPanelOpen: boolean
    theme: Theme
    loading: boolean
    notifications: Notification[]
  }
}

export interface Notification {
  id: string
  type: 'success' | 'error' | 'warning' | 'info'
  title: string
  message: string
  duration?: number
  actions?: NotificationAction[]
  timestamp: string
}

export interface NotificationAction {
  label: string
  action: () => void
  primary?: boolean
}

// WebSocket message types
export interface WSMessage<T = any> {
  type: string
  payload: T
  timestamp: string
  user_id?: number
  room_id?: string
}

export interface FileChangeMessage {
  file_id: number
  content: string
  line: number
  column: number
  change_type: 'insert' | 'delete' | 'replace'
  user_id: number
}

export interface CursorUpdateMessage {
  user_id: number
  file_id: number
  line: number
  column: number
  selection?: {
    startLine: number
    startColumn: number
    endLine: number
    endColumn: number
  }
}

// Monaco Editor types
export interface EditorTheme {
  base: 'vs' | 'vs-dark' | 'hc-black'
  inherit: boolean
  rules: Array<{
    token: string
    foreground?: string
    background?: string
    fontStyle?: string
  }>
  colors: Record<string, string>
}

// Language support
export interface LanguageConfig {
  id: string
  name: string
  extensions: string[]
  icon: string
  color: string
  monacoLanguage: string
  defaultCode: string
  runCommand: string
  buildCommand?: string
  testCommand?: string
}

// File explorer types
export interface FileTreeNode {
  id: string
  name: string
  type: 'file' | 'directory'
  path: string
  children?: FileTreeNode[]
  isExpanded?: boolean
  isLoading?: boolean
  file?: File
}

// AI Assistant types
export interface AIMessage {
  id: string
  type: 'user' | 'assistant' | 'system'
  content: string
  provider?: AIProvider
  capability?: AICapability
  code?: string
  language?: string
  timestamp: string
  isStreaming?: boolean
  error?: string
  usage?: {
    tokens: number
    cost: number
    duration: number
  }
}

export interface AIConversation {
  id: string
  messages: AIMessage[]
  project_id?: number
  file_id?: number
  created_at: string
  updated_at: string
}

// Terminal types
export interface TerminalSession {
  id: string
  name: string
  status: 'running' | 'stopped' | 'error'
  output: string[]
  input: string
  project_id?: number
  created_at: string
}

// Component prop types
export interface ComponentBaseProps {
  className?: string
  children?: React.ReactNode
  id?: string
  'data-testid'?: string
}

// Form types
export interface FormField {
  name: string
  label: string
  type: 'text' | 'email' | 'password' | 'textarea' | 'select' | 'checkbox'
  placeholder?: string
  required?: boolean
  validation?: {
    pattern?: RegExp
    minLength?: number
    maxLength?: number
    custom?: (value: any) => string | undefined
  }
  options?: Array<{ value: string; label: string }>
}

export interface FormState {
  values: Record<string, any>
  errors: Record<string, string>
  touched: Record<string, boolean>
  isSubmitting: boolean
  isValid: boolean
}

// Hook return types
export interface UseApiReturn<T> {
  data: T | null
  loading: boolean
  error: string | null
  refetch: () => void
}

export interface UseAsyncReturn<T> {
  data: T | null
  loading: boolean
  error: string | null
  execute: (...args: any[]) => Promise<void>
}

// Utility types
export type DeepPartial<T> = {
  [P in keyof T]?: T[P] extends object ? DeepPartial<T[P]> : T[P]
}

export type Optional<T, K extends keyof T> = Omit<T, K> & Partial<Pick<T, K>>

export type WithId<T> = T & { id: string }

export type WithTimestamp<T> = T & {
  created_at: string
  updated_at: string
}

// Community/Sharing Marketplace Types

export interface ProjectStar {
  id: number
  user_id: number
  project_id: number
  created_at: string
}

export interface ProjectFork {
  id: number
  original_id: number
  forked_id: number
  user_id: number
  created_at: string
}

export interface ProjectComment {
  id: number
  project_id: number
  user_id: number
  user?: User
  parent_id?: number
  content: string
  is_edited: boolean
  created_at: string
  updated_at: string
  replies?: ProjectComment[]
}

export interface ProjectCategory {
  id: number
  name: string
  slug: string
  description: string
  icon: string
  color: string
  sort_order: number
  created_at: string
}

export interface ProjectStats {
  project_id: number
  star_count: number
  fork_count: number
  view_count: number
  comment_count: number
  trend_score: number
  updated_at: string
}

export interface UserStats {
  user_id: number
  follower_count: number
  following_count: number
  project_count: number
  total_stars: number
  total_forks: number
  updated_at: string
}

export interface ProjectWithStats extends Project {
  stats?: ProjectStats
  is_starred?: boolean
  is_fork?: boolean
  original_id?: number
  categories?: string[]
  // Flattened fields for UI convenience
  owner_username?: string
  owner_avatar_url?: string
  stars?: number
  forks?: number
  views?: number
  topics?: string[]
  is_verified?: boolean
}

export interface UserPublicProfile {
  id: number
  username: string
  full_name: string
  avatar_url: string
  bio?: string
  website?: string
  location?: string
  joined_at: string
  follower_count: number
  following_count: number
  project_count: number
  total_stars: number
  total_forks: number
  is_following?: boolean
}

export interface ExploreData {
  featured: ProjectWithStats[]
  trending: ProjectWithStats[]
  recent: ProjectWithStats[]
  categories: ProjectCategory[]
}

export interface UserFollowInfo {
  id: number
  username: string
  full_name: string
  avatar_url: string
  followed_at: string
}

// AI Autonomous Agent Types
export * from './agent'

// Version History Types (Replit parity feature)

export interface FileVersion {
  id: number
  file_id: number
  project_id: number
  version: number
  version_hash: string
  content: string
  size: number
  line_count: number
  change_type: 'create' | 'edit' | 'rename' | 'restore'
  change_summary?: string
  lines_added: number
  lines_removed: number
  author_id: number
  author_name: string
  file_path: string
  file_name: string
  is_pinned: boolean
  is_auto_save: boolean
  created_at: string
}

export interface VersionDiff {
  old_version_id: number
  new_version_id: number
  diff_content: string // Unified diff format
  insertions: number
  deletions: number
  files_changed: number
}

// GitHub Import Wizard Types

export interface DetectedStack {
  language: string
  framework: string
  package_manager: string
  entry_point: string
}

export interface GitHubRepoValidation {
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
  detected_stack?: DetectedStack
}

export interface GitHubImportRequest {
  url: string
  project_name?: string
  description?: string
  is_public?: boolean
  token?: string
}

export interface GitHubImportResponse {
  project_id: number
  project_name: string
  language: string
  framework: string
  detected_stack: DetectedStack
  file_count: number
  status: string
  message: string
  import_duration_ms: number
  repository_url: string
  default_branch: string
}

// Managed Database Types

export type DatabaseType = 'postgresql' | 'redis' | 'sqlite'
export type DatabaseStatus = 'provisioning' | 'active' | 'suspended' | 'deleting' | 'failed'

export interface ManagedDatabase {
  id: number
  project_id: number
  type: DatabaseType
  name: string
  host: string
  port: number
  database_name: string
  status: DatabaseStatus
  storage_used_mb: number
  connection_count: number
  query_count: number
  last_queried?: string
  backup_enabled: boolean
  backup_schedule?: string
  last_backup?: string
  next_backup?: string
  max_storage_mb: number
  max_connections: number
  is_auto_provisioned?: boolean // True if auto-created with project (Replit parity)
  created_at: string
  updated_at: string
  credentials?: Record<string, string> // Only if requested with reveal=true
}

export interface CreateDatabaseRequest {
  name: string
  type: DatabaseType
}

export interface DatabaseMetrics {
  cpu_usage_percent: number
  memory_usage_mb: number
  storage_usage_mb: number
  active_connections: number
  queries_per_second: number
  cache_hit_rate?: number // Redis only
}

export interface TableInfo {
  name: string
  schema: string
  row_count: number
  size_bytes: number
}

export interface ColumnInfo {
  name: string
  type: string
  is_nullable: boolean
  default_value?: string
  is_primary_key: boolean
}

// Code Comments Types (Replit parity feature)

export interface CodeComment {
  id: number
  file_id: number
  project_id: number
  start_line: number
  end_line: number
  start_column: number
  end_column: number
  content: string
  parent_id?: number
  thread_id: string
  author_id: number
  author_name: string
  is_resolved: boolean
  resolved_at?: string
  resolved_by_id?: number
  reactions?: Record<string, number[]>
  replies?: CodeComment[]
  reply_count: number
  created_at: string
  updated_at: string
}

export interface CodeCommentThread {
  thread_id: string
  file_id: number
  project_id: number
  start_line: number
  end_line: number
  is_resolved: boolean
  comment_count: number
  comments: CodeComment[]
  created_at: string
  updated_at: string
}

export interface CreateCommentRequest {
  file_id: number
  project_id: number
  start_line: number
  end_line: number
  start_column?: number
  end_column?: number
  content: string
  parent_id?: number
  thread_id?: string
}

export interface UpdateCommentRequest {
  content: string
}

export interface ReactionRequest {
  emoji: string
}

// Environment Configuration Types (Nix-like reproducible environments - Replit parity)

export interface PackageDependency {
  name: string
  version?: string
  source?: string
}

export interface EnvironmentConfig {
  language: string
  version: string
  packages: PackageDependency[]
  dev_packages: PackageDependency[]
  system: string[]
  env_vars: Record<string, string>
  build_command?: string
  start_command?: string
  install_command?: string
  options?: Record<string, any>
}

export interface RuntimeInfo {
  id: string
  name: string
  description: string
  versions: string[]
  default: string
  package_manager: string
  icon: string
}

export interface EnvironmentPreset {
  id: string
  name: string
  description: string
  language: string
  version: string
  packages: PackageDependency[]
  dev_packages: PackageDependency[]
  system: string[]
}

export interface EnvironmentPackageInfo {
  name: string
  description: string
  category: string
}

export interface DetectedEnvironment {
  environment: EnvironmentConfig
  confidence: number
  suggestions: string[]
}

// BYOK (Bring Your Own Key) Types

export interface BYOKKeyInfo {
  provider: AIProvider
  model_preference: string
  is_active: boolean
  is_valid: boolean
  last_used?: string
  usage_count: number
  total_cost: number
}

export interface BYOKModelInfo {
  id: string
  name: string
  speed: 'slow' | 'medium' | 'fast' | 'variable'
  cost_tier: 'low' | 'medium' | 'high' | 'free'
  description: string
}

export interface BYOKUsageSummary {
  total_cost: number
  total_tokens: number
  total_requests: number
  by_provider: Record<string, {
    provider: string
    cost: number
    tokens: number
    requests: number
    byok_requests: number
  }>
}

// Code Completions Types (Ghostwriter-equivalent)

export type CompletionTriggerKind = 'invoked' | 'trigger_char' | 'automatic' | 'incomplete'

export type CompletionKind =
  | 'text'
  | 'method'
  | 'function'
  | 'constructor'
  | 'field'
  | 'variable'
  | 'class'
  | 'interface'
  | 'module'
  | 'property'
  | 'unit'
  | 'value'
  | 'enum'
  | 'keyword'
  | 'snippet'
  | 'color'
  | 'file'
  | 'reference'
  | 'customcolor'
  | 'folder'
  | 'type_parameter'

export interface CompletionRange {
  start_line: number
  start_column: number
  end_line: number
  end_column: number
}

export interface CompletionTextEdit {
  range: CompletionRange
  new_text: string
}

export interface CompletionItem {
  id: string
  text: string
  display_text: string
  insert_text: string
  kind: CompletionKind
  detail?: string
  documentation?: string
  sort_text?: string
  filter_text?: string
  confidence: number
  range?: CompletionRange
  additional_edits?: CompletionTextEdit[]
}

export interface CompletionUsage {
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  estimated_cost: number
}

export interface CompletionResponse {
  id: string
  completions: CompletionItem[]
  provider: string
  model: string
  processing_time_ms: number
  cached_hit: boolean
  usage?: CompletionUsage
}

export interface RecentEdit {
  line: number
  old_text: string
  new_text: string
  timestamp: number
}

export interface RelatedFile {
  path: string
  language: string
  snippet: string
}

export interface CompletionContext {
  file_imports?: string[]
  file_symbols?: string[]
  project_symbols?: string[]
  recent_edits?: RecentEdit[]
  related_files?: RelatedFile[]
  framework?: string
  dependencies?: Record<string, string>
}

export interface CompletionRequest {
  project_id?: number
  file_id?: number
  file_path?: string
  language: string
  prefix: string
  suffix: string
  line: number
  column: number
  trigger_kind: CompletionTriggerKind
  context?: CompletionContext
  max_tokens?: number
  temperature?: number
  stop_tokens?: string[]
}

export interface CompletionStats {
  total_requests: number
  avg_latency_ms: number
  cache_hit_rate: number
  provider_requests: Record<string, number>
}