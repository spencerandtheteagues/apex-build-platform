// APEX.BUILD Terminal WebSocket Service
// Handles real-time communication with backend PTY

import { apiService, TerminalSessionInfo, AvailableShell } from '@/services/api';
import { TerminalMessage, TerminalSession } from './types';

export interface TerminalServiceCallbacks {
  onData: (data: string) => void;
  onConnect: () => void;
  onDisconnect: () => void;
  onError: (error: string) => void;
  onExit: (message: string) => void;
}

// Options for creating a terminal session
export interface CreateSessionOptions {
  projectId?: number;
  workDir?: string;
  shell?: string;
  name?: string;
  rows?: number;
  cols?: number;
  environment?: Record<string, string>;
}

export class TerminalService {
  private ws: WebSocket | null = null;
  private sessionId: string | null = null;
  private callbacks: TerminalServiceCallbacks;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000;
  private heartbeatInterval: number | null = null;
  private isConnecting = false;
  private pendingMessages: TerminalMessage[] = [];

  constructor(callbacks: TerminalServiceCallbacks) {
    this.callbacks = callbacks;
  }

  // Get WebSocket URL using the API service
  private getWebSocketUrl(sessionId: string): string {
    return apiService.getTerminalWebSocketUrl(sessionId);
  }

  // Create a new terminal session via REST API
  async createSession(projectId?: number, workDir?: string, options?: Partial<CreateSessionOptions>): Promise<TerminalSession> {
    const response = await apiService.createTerminalSession(projectId, {
      workDir: workDir || options?.workDir,
      shell: options?.shell,
      name: options?.name,
      rows: options?.rows || 24,
      cols: options?.cols || 80,
      environment: options?.environment,
    });

    return {
      id: response.session_id,
      name: response.name || 'Terminal',
      projectId: response.project_id,
      workDir: response.work_dir,
      shell: response.shell || 'bash',
      status: 'connected',
      createdAt: response.created_at,
      lastActive: new Date().toISOString(),
      rows: response.rows || 24,
      cols: response.cols || 80,
    };
  }

  // Get available shells using the API service
  async getAvailableShells(): Promise<AvailableShell[]> {
    return apiService.listAvailableShells();
  }

  // Connect to terminal WebSocket
  async connect(sessionId: string): Promise<void> {
    if (this.ws?.readyState === WebSocket.OPEN) {
      console.log('WebSocket already connected');
      return;
    }

    if (this.isConnecting) {
      console.log('WebSocket connection in progress');
      return;
    }

    this.isConnecting = true;
    this.sessionId = sessionId;

    return new Promise((resolve, reject) => {
      const wsUrl = this.getWebSocketUrl(sessionId);
      console.log('Connecting to terminal WebSocket:', wsUrl);

      try {
        this.ws = new WebSocket(wsUrl);

        const connectionTimeout = setTimeout(() => {
          if (this.ws?.readyState !== WebSocket.OPEN) {
            this.isConnecting = false;
            reject(new Error('Connection timeout'));
            this.ws?.close();
          }
        }, 10000);

        this.ws.onopen = () => {
          clearTimeout(connectionTimeout);
          this.isConnecting = false;
          this.reconnectAttempts = 0;
          console.log('Terminal WebSocket connected');
          this.callbacks.onConnect();
          this.startHeartbeat();
          this.flushPendingMessages();
          resolve();
        };

        this.ws.onmessage = (event) => {
          try {
            const message: TerminalMessage = JSON.parse(event.data);
            this.handleMessage(message);
          } catch (err) {
            // Might be raw data
            this.callbacks.onData(event.data);
          }
        };

        this.ws.onerror = (event) => {
          console.error('Terminal WebSocket error:', event);
          this.isConnecting = false;
          this.callbacks.onError('WebSocket connection error');
        };

        this.ws.onclose = (event) => {
          console.log('Terminal WebSocket closed:', event.code, event.reason);
          this.isConnecting = false;
          this.stopHeartbeat();
          this.callbacks.onDisconnect();

          // Auto-reconnect unless intentionally closed
          if (event.code !== 1000 && this.sessionId) {
            this.scheduleReconnect();
          }
        };
      } catch (err) {
        this.isConnecting = false;
        reject(err);
      }
    });
  }

  private handleMessage(message: TerminalMessage): void {
    switch (message.type) {
      case 'output':
        if (message.data) {
          this.callbacks.onData(message.data);
        }
        break;
      case 'error':
        this.callbacks.onError(message.data || 'Unknown error');
        break;
      case 'exit':
        this.callbacks.onExit(message.data || 'Terminal session ended');
        break;
      case 'pong':
        // Heartbeat acknowledged
        break;
      default:
        console.log('Unknown terminal message type:', message.type);
    }
  }

  // Send input to terminal
  sendInput(data: string): void {
    this.send({ type: 'input', data });
  }

  // Send resize event
  resize(rows: number, cols: number): void {
    this.send({ type: 'resize', rows, cols });

    // Also notify backend via REST API for persistent state
    if (this.sessionId) {
      apiService.resizeTerminal(this.sessionId, cols, rows).catch((err) => {
        console.warn('Failed to update terminal size via REST:', err);
      });
    }
  }

  // Send signal (SIGINT, SIGTERM, etc.)
  sendSignal(signal: string): void {
    this.send({ type: 'signal', signal });
  }

  // Generic send method
  private send(message: TerminalMessage): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    } else {
      // Queue message for later
      this.pendingMessages.push(message);
    }
  }

  private flushPendingMessages(): void {
    while (this.pendingMessages.length > 0) {
      const message = this.pendingMessages.shift();
      if (message) {
        this.send(message);
      }
    }
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.heartbeatInterval = window.setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.send({ type: 'ping' });
      }
    }, 30000);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.log('Max reconnect attempts reached');
      return;
    }

    this.reconnectAttempts++;
    const delay = Math.min(
      this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1),
      10000
    );

    console.log(`Scheduling reconnect attempt ${this.reconnectAttempts} in ${delay}ms`);

    setTimeout(() => {
      if (this.sessionId && !this.isConnecting && this.ws?.readyState !== WebSocket.OPEN) {
        console.log('Attempting to reconnect...');
        this.connect(this.sessionId).catch(err => {
          console.error('Reconnect failed:', err);
        });
      }
    }, delay);
  }

  // Get session info using the API service
  async getSession(sessionId: string): Promise<TerminalSession | null> {
    const info = await apiService.getTerminalSession(sessionId);
    if (!info) return null;

    return {
      id: info.id,
      name: info.name || 'Terminal',
      projectId: info.project_id,
      workDir: info.work_dir,
      shell: info.shell,
      status: info.is_active ? 'connected' : 'disconnected',
      createdAt: info.created_at,
      lastActive: info.last_active,
      rows: info.rows,
      cols: info.cols,
    };
  }

  // List all sessions using the API service
  async listSessions(projectId?: number): Promise<TerminalSession[]> {
    const sessions = await apiService.listTerminalSessions(projectId);
    return sessions.map((info: TerminalSessionInfo) => ({
      id: info.id,
      name: info.name || 'Terminal',
      projectId: info.project_id,
      workDir: info.work_dir,
      shell: info.shell,
      status: info.is_active ? 'connected' as const : 'disconnected' as const,
      createdAt: info.created_at,
      lastActive: info.last_active,
      rows: info.rows || 24,
      cols: info.cols || 80,
    }));
  }

  // Delete session using the API service
  async deleteSession(sessionId: string): Promise<void> {
    await apiService.closeTerminalSession(sessionId);
  }

  // Get command history using the API service
  async getHistory(sessionId: string): Promise<string[]> {
    return apiService.getCommandHistory(sessionId);
  }

  // Disconnect
  disconnect(): void {
    this.stopHeartbeat();
    this.sessionId = null;
    this.pendingMessages = [];

    if (this.ws) {
      this.ws.close(1000, 'Client disconnect');
      this.ws = null;
    }
  }

  // Check connection status
  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  // Get current session ID
  getSessionId(): string | null {
    return this.sessionId;
  }
}

export default TerminalService;
