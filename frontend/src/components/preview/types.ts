export interface PreviewStatus {
  project_id: number
  active: boolean
  port: number
  url: string
  started_at: string
  last_access: string
  connected_clients: number
}

export interface ServerStatus {
  running: boolean
  port?: number
  pid?: number
  uptime_seconds?: number
  command?: string
  entry_file?: string
  url?: string
  ready?: boolean
  exited_at?: string
  exit_code?: number
  last_error?: string
}

export interface ServerDetection {
  has_backend: boolean
  server_type?: string
  entry_file?: string
  command?: string
  framework?: string
}

export type ViewportSize = 'mobile' | 'tablet' | 'desktop' | 'full'
export type ActiveTab = 'preview' | 'console' | 'network'
