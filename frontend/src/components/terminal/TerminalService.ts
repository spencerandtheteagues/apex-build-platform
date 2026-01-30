// APEX.BUILD Terminal WebSocket Service
// Handles real-time communication with backend PTY

import { TerminalMessage, TerminalSession } from './types';

export interface TerminalServiceCallbacks {
  onData: (data: string) => void;
  onConnect: () => void;
  onDisconnect: () => void;
  onError: (error: string) => void;
  onExit: (message: string) => void;
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

  // Get WebSocket URL based on environment
  private getWebSocketUrl(sessionId: string): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = import.meta.env.VITE_WS_HOST || window.location.host;

    // Check if we have a specific API URL
    const apiUrl = import.meta.env.VITE_API_URL;
    if (apiUrl) {
      const url = new URL(apiUrl);
      return `${protocol}//${url.host}/ws/terminal/${sessionId}`;
    }

    return `${protocol}//${host}/ws/terminal/${sessionId}`;
  }

  // Create a new terminal session via REST API
  async createSession(projectId?: number, workDir?: string): Promise<TerminalSession> {
    const token = localStorage.getItem('apex_access_token');
    const apiUrl = import.meta.env.VITE_API_URL || '/api/v1';

    const response = await fetch(`${apiUrl}/terminal/sessions`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
      },
      body: JSON.stringify({
        project_id: projectId || 0,
        work_dir: workDir || '',
      }),
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to create terminal session');
    }

    const result = await response.json();
    return {
      id: result.data.session_id,
      name: `Terminal`,
      projectId: result.data.project_id,
      workDir: result.data.work_dir,
      shell: 'bash',
      status: 'connected',
      createdAt: result.data.created_at,
      lastActive: new Date().toISOString(),
      rows: 24,
      cols: 80,
    };
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

  // Get session info
  async getSession(sessionId: string): Promise<TerminalSession | null> {
    const token = localStorage.getItem('apex_access_token');
    const apiUrl = import.meta.env.VITE_API_URL || '/api/v1';

    try {
      const response = await fetch(`${apiUrl}/terminal/sessions/${sessionId}`, {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        return null;
      }

      const result = await response.json();
      return result.data;
    } catch {
      return null;
    }
  }

  // List all sessions
  async listSessions(): Promise<TerminalSession[]> {
    const token = localStorage.getItem('apex_access_token');
    const apiUrl = import.meta.env.VITE_API_URL || '/api/v1';

    try {
      const response = await fetch(`${apiUrl}/terminal/sessions`, {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        return [];
      }

      const result = await response.json();
      return result.data || [];
    } catch {
      return [];
    }
  }

  // Delete session
  async deleteSession(sessionId: string): Promise<void> {
    const token = localStorage.getItem('apex_access_token');
    const apiUrl = import.meta.env.VITE_API_URL || '/api/v1';

    await fetch(`${apiUrl}/terminal/sessions/${sessionId}`, {
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${token}`,
      },
    });
  }

  // Get command history
  async getHistory(sessionId: string): Promise<string[]> {
    const token = localStorage.getItem('apex_access_token');
    const apiUrl = import.meta.env.VITE_API_URL || '/api/v1';

    try {
      const response = await fetch(`${apiUrl}/terminal/sessions/${sessionId}/history`, {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        return [];
      }

      const result = await response.json();
      return result.data?.history || [];
    } catch {
      return [];
    }
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
