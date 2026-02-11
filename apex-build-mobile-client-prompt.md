# Claude Code Master Prompt: Build APEX.BUILD Mobile Apps (iOS & Android)

## 🎯 Mission

Build **production-ready iOS and Android mobile applications** that serve as native clients for the **existing APEX.BUILD platform**. The mobile apps connect to the existing Go backend API and WebSocket servers. Do NOT build a new backend—use the existing API at `https://apex-backend-5ypy.onrender.com/api/v1`.

**The apps must be fully functional, visually stunning with APEX.BUILD's black and red cyberpunk aesthetic, optimized for phones and tablets, and completely ready for App Store and Google Play submission.**

### Critical Design Requirements

1. **Color Theme:** Pure black background (#000000) with red (#ff0033) accents - NO steampunk, brass, or gold colors
2. **Header Logo:** Use enlarged `Apex-Build-Logo1.png` in ALL headers - the logo must be clearly visible (40-48px height), not tiny icons
3. **Font:** Use Fira Code for monospace, Inter for body, Orbitron for display

---

## 📋 Project Context

**Product:** APEX.BUILD Mobile Client
**Tagline:** "Describe it. Build it. Deploy it."
**Platform:** iOS 15+ and Android 8+ (API 26+)
**Framework:** React Native with Expo SDK 52+

### What APEX.BUILD Is

APEX.BUILD is an AI-powered cloud development platform (direct Replit competitor) with an **existing, fully functional backend**. The mobile apps are native clients that:

1. Authenticate users against the existing API
2. Let users describe apps in natural language
3. Show real-time AI agent build progress via WebSocket
4. Provide a mobile code editor for viewing/editing files
5. Allow deployment to Vercel, Netlify, Render, or native hosting
6. Manage BYOK (Bring Your Own Key) API keys

---

## 🔗 Existing Backend API Reference

### Base URLs

```typescript
const API_CONFIG = {
  // REST API
  baseUrl: 'https://apex-backend-5ypy.onrender.com/api/v1',
  
  // WebSocket for real-time features
  wsUrl: 'wss://apex-backend-5ypy.onrender.com',
  
  // WebSocket namespaces
  wsBuild: '/ws/build',      // Build progress
  wsCollab: '/ws',           // Collaboration
  wsTerminal: '/ws/terminal' // Terminal sessions
};
```

### Authentication

The backend uses **JWT authentication** with access + refresh tokens.

```typescript
// POST /auth/register
interface RegisterRequest {
  username: string;
  email: string;
  password: string;
  full_name?: string;
}

// POST /auth/login
interface LoginRequest {
  username: string;
  password: string;
}

// Response for both register and login
interface AuthResponse {
  message: string;
  user: User;
  tokens: {
    access_token: string;      // 15 min expiry
    refresh_token: string;     // 7 day expiry
    access_token_expires_at: string;
    token_type: 'Bearer';
  };
}

// POST /auth/refresh
interface RefreshRequest {
  refresh_token: string;
}

// All authenticated requests require:
// Authorization: Bearer <access_token>
```

### User Model

```typescript
interface User {
  id: number;
  username: string;
  email: string;
  full_name?: string;
  avatar_url?: string;
  is_active: boolean;
  is_verified: boolean;
  subscription_type: 'free' | 'pro' | 'team';
  subscription_end?: string;
  monthly_ai_requests: number;
  monthly_ai_cost: number;
  preferred_theme: 'cyberpunk' | 'matrix' | 'synthwave' | 'neonCity';
  preferred_ai: 'auto' | 'claude' | 'gpt4' | 'gemini' | 'grok' | 'ollama';
  created_at: string;
  updated_at: string;
}

// GET /user/profile - Get current user
// PUT /user/profile - Update profile
```

### Projects API

```typescript
interface Project {
  id: number;
  name: string;
  description?: string;
  language: string;
  framework?: string;
  owner_id: number;
  is_public: boolean;
  is_archived: boolean;
  root_directory: string;
  entry_point?: string;
  environment?: Record<string, any>;
  created_at: string;
  updated_at: string;
}

// GET /projects - List user's projects
// Response: { projects: Project[] }

// POST /projects - Create project
// Request: { name, description?, language, framework?, is_public?, environment? }
// Response: { message, project: Project }

// GET /projects/:id - Get project details
// Response: { project: Project }

// PUT /projects/:id - Update project
// DELETE /projects/:id - Delete project
// GET /projects/:id/download - Download as ZIP
```

### Files API

```typescript
interface File {
  id: number;
  project_id: number;
  path: string;
  name: string;
  type: 'file' | 'directory';
  mime_type?: string;
  content: string;
  size: number;
  version: number;
  is_locked: boolean;
  created_at: string;
  updated_at: string;
}

// GET /projects/:projectId/files - List files
// Response: { data: { files: File[] } }

// POST /projects/:projectId/files - Create file
// Request: { path, name, type, content?, mime_type? }

// PUT /files/:id - Update file content
// Request: { content: string }

// DELETE /files/:id - Delete file
```

### AI Build System API

```typescript
// POST /build/start - Start an AI build
interface StartBuildRequest {
  description: string;  // Natural language app description
  mode: 'fast' | 'full'; // fast=3-5min, full=10+min
}

interface StartBuildResponse {
  build_id: string;
  websocket_url: string; // wss://apex-backend-5ypy.onrender.com/ws/build/{build_id}
  status: 'planning';
}

// GET /build/:buildId/status - Get build status
interface BuildStatus {
  build_id: string;
  status: 'planning' | 'in_progress' | 'completed' | 'failed' | 'cancelled';
  progress: number; // 0-100
  phase: string;
  agents: AgentStatus[];
  tasks_completed: number;
  tasks_total: number;
  files_generated: number;
  duration_ms: number;
}

interface AgentStatus {
  role: 'lead' | 'planner' | 'architect' | 'frontend' | 'backend' | 'database' | 'testing' | 'reviewer';
  status: 'waiting' | 'working' | 'completed' | 'error';
  progress: number;
  message?: string;
}

// POST /build/:buildId/message - Send message to Lead Agent
// Request: { message: string }

// POST /build/:buildId/cancel - Cancel build
// GET /build/:buildId/checkpoints - Get checkpoints
// POST /build/:buildId/rollback/:checkpointId - Rollback
```

### AI Generation API

```typescript
// POST /ai/generate
interface AIGenerateRequest {
  capability: AICapability;
  prompt: string;
  language?: string;
  context?: Record<string, any>;
  max_tokens?: number;
  temperature?: number;
  project_id?: string;
}

type AICapability = 
  | 'code_generation'
  | 'code_review'
  | 'code_completion'
  | 'debugging'
  | 'explanation'
  | 'refactoring'
  | 'testing'
  | 'documentation'
  | 'architecture';

interface AIGenerateResponse {
  request_id: string;
  provider: 'claude' | 'gpt4' | 'gemini' | 'grok' | 'ollama';
  content: string;
  usage: {
    prompt_tokens: number;
    completion_tokens: number;
    total_tokens: number;
    cost: number;
  };
  duration: number;
}

// GET /ai/usage - Get AI usage statistics
```

### BYOK (Bring Your Own Key) API

```typescript
// GET /byok/keys - List user's API keys
interface BYOKKeyInfo {
  provider: 'openai' | 'anthropic' | 'google' | 'xai' | 'ollama';
  model_preference: string;
  is_active: boolean;
  is_valid: boolean;
  last_used?: string;
  usage_count: number;
  total_cost: number;
}

// POST /byok/keys - Add new API key
// Request: { provider, api_key, model_preference? }

// DELETE /byok/keys/:provider - Remove API key

// PUT /byok/keys/:provider/toggle - Enable/disable key

// GET /byok/usage - Get BYOK usage stats
// GET /byok/models - Get available models per provider
```

### Deployment API

```typescript
// POST /projects/:projectId/deploy - Start deployment
interface DeploymentConfig {
  subdomain?: string;
  port?: number;
  build_command?: string;
  start_command?: string;
  framework?: string;
  always_on?: boolean;
  env_vars?: Record<string, string>;
}

interface Deployment {
  id: string;
  project_id: number;
  subdomain: string;
  url: string;
  status: 'pending' | 'provisioning' | 'building' | 'deploying' | 'running' | 'stopped' | 'failed';
  always_on: boolean;
  created_at: string;
  deployed_at?: string;
}

// GET /projects/:projectId/deployments - List deployments
// GET /projects/:projectId/deployments/:deploymentId - Get deployment
// DELETE /projects/:projectId/deployments/:deploymentId - Stop deployment
// POST /projects/:projectId/deployments/:deploymentId/restart - Restart

// External deployment providers
// POST /deploy/vercel - Deploy to Vercel
// POST /deploy/netlify - Deploy to Netlify
// POST /deploy/render - Deploy to Render
```

### Billing API

```typescript
// GET /billing/plans - Get subscription plans
interface Plan {
  id: 'free' | 'pro' | 'team' | 'enterprise';
  name: string;
  price: number;
  features: string[];
}

// POST /billing/checkout - Create Stripe checkout session
// Request: { plan: string, success_url: string, cancel_url: string }
// Response: { checkout_url: string }

// GET /billing/subscription - Get current subscription
// POST /billing/cancel - Cancel subscription
// GET /billing/usage - Get usage statistics
// GET /billing/invoices - Get invoices
```

---

## 🔌 WebSocket Events Reference

### Build WebSocket (`/ws/build/:buildId`)

Connect to receive real-time build updates:

```typescript
// Outgoing events (client → server)
interface WSBuildSubscribe {
  type: 'subscribe';
  build_id: string;
  token: string; // JWT access token
}

// Incoming events (server → client)
type BuildWSEvent = 
  | { type: 'build:started'; build_id: string; timestamp: string }
  | { type: 'build:progress'; build_id: string; progress: number; phase: string }
  | { type: 'build:checkpoint'; build_id: string; checkpoint: Checkpoint }
  | { type: 'build:completed'; build_id: string; project_id: number }
  | { type: 'build:error'; build_id: string; error: string }
  | { type: 'agent:spawned'; build_id: string; agent: AgentInfo }
  | { type: 'agent:working'; build_id: string; agent_id: string; task: string }
  | { type: 'agent:progress'; build_id: string; data: AgentProgress }
  | { type: 'agent:completed'; build_id: string; agent_id: string }
  | { type: 'agent:message'; build_id: string; agent_id: string; message: string }
  | { type: 'file:created'; build_id: string; file: FileInfo }
  | { type: 'file:updated'; build_id: string; file: FileInfo }
  | { type: 'lead:response'; build_id: string; message: string };

interface AgentProgress {
  role: string;
  status: string;
  progress: number;
  message: string;
}

interface FileInfo {
  path: string;
  name: string;
  language?: string;
  size?: number;
}
```

### Collaboration WebSocket (`/ws`)

For real-time collaboration features:

```typescript
// Connect with Socket.io
const socket = io('wss://apex-backend-5ypy.onrender.com/ws', {
  auth: { token: accessToken },
  transports: ['websocket', 'polling']
});

// Events
type CollabEvent =
  | 'user-joined'    // { user: User, room_id: string }
  | 'user-left'      // { user_id: number, room_id: string }
  | 'file-changed'   // { file_id, content, line, column, change_type, user_id }
  | 'cursor-moved'   // { user_id, file_id, line, column, selection? }
  | 'chat-message'   // { ChatMessage }
  | 'file-locked'    // { file_id, user_id, user }
  | 'file-unlocked'  // { file_id, user_id }

// Room management
socket.emit('join-room', { room_id: string });
socket.emit('leave-room', { room_id: string });
```

---

## 🛠 Technical Requirements

### Framework & Dependencies

```json
{
  "framework": "React Native 0.76+ with Expo SDK 52+",
  "language": "TypeScript 5.x (strict mode)",
  "stateManagement": "Zustand + TanStack Query (React Query v5)",
  "navigation": "Expo Router v4 (file-based routing)",
  "ui": {
    "components": "Tamagui or NativeWind + custom cyberpunk components",
    "icons": "Lucide React Native",
    "animations": "React Native Reanimated 3 + Moti"
  },
  "storage": {
    "secure": "expo-secure-store (for tokens)",
    "async": "@react-native-async-storage/async-storage"
  },
  "networking": {
    "http": "Axios with interceptors for token refresh",
    "websocket": "socket.io-client (match backend)"
  },
  "auth": {
    "biometric": "expo-local-authentication",
    "apple": "expo-apple-authentication (required for iOS)"
  },
  "editor": {
    "mobile": "react-native-code-editor or Monaco in WebView",
    "syntax": "Shiki for highlighting"
  },
  "payments": {
    "subscriptions": "RevenueCat (handles both App Store & Play Store)"
  },
  "notifications": "expo-notifications"
}
```

### Project Structure

```
apex-mobile/
├── app/                              # Expo Router
│   ├── (auth)/
│   │   ├── _layout.tsx
│   │   ├── login.tsx
│   │   ├── register.tsx
│   │   └── forgot-password.tsx
│   ├── (tabs)/
│   │   ├── _layout.tsx
│   │   ├── index.tsx                 # Dashboard
│   │   ├── projects.tsx              # Project list
│   │   ├── build.tsx                 # AI Build
│   │   └── settings.tsx              # Settings
│   ├── project/
│   │   └── [id]/
│   │       ├── index.tsx             # Project overview
│   │       ├── editor.tsx            # Code editor
│   │       ├── files.tsx             # File browser
│   │       ├── terminal.tsx          # Terminal
│   │       └── deploy.tsx            # Deployment
│   ├── build/
│   │   ├── new.tsx                   # Start new build
│   │   └── [buildId].tsx             # Build progress
│   ├── settings/
│   │   ├── account.tsx
│   │   ├── api-keys.tsx              # BYOK management
│   │   ├── subscription.tsx
│   │   └── ai-preferences.tsx
│   ├── _layout.tsx
│   └── +not-found.tsx
├── components/
│   ├── ui/                           # Black & red design system
│   │   ├── Button.tsx
│   │   ├── Card.tsx
│   │   ├── Input.tsx
│   │   ├── Badge.tsx
│   │   ├── LoadingSpinner.tsx        # Red glow loading spinner
│   │   ├── RedGlow.tsx
│   │   ├── CyberButton.tsx
│   │   └── index.ts
│   ├── agent/
│   │   ├── AgentCard.tsx
│   │   ├── AgentOrchestration.tsx    # Visual agent flow
│   │   ├── BuildProgress.tsx
│   │   └── AgentChat.tsx
│   ├── editor/
│   │   ├── CodeEditor.tsx
│   │   ├── FileTree.tsx
│   │   └── TabBar.tsx
│   ├── project/
│   │   ├── ProjectCard.tsx
│   │   └── ProjectList.tsx
│   └── common/
│       ├── Header.tsx
│       └── EmptyState.tsx
├── services/
│   ├── api.ts                        # API client (match existing)
│   ├── websocket.ts                  # WebSocket client
│   ├── auth.ts                       # Auth helpers
│   └── storage.ts                    # Secure storage
├── stores/
│   ├── authStore.ts
│   ├── projectStore.ts
│   ├── buildStore.ts
│   └── settingsStore.ts
├── hooks/
│   ├── useAuth.ts
│   ├── useProjects.ts
│   ├── useBuild.ts
│   ├── useWebSocket.ts
│   └── useSubscription.ts
├── types/
│   ├── api.ts                        # Match backend types exactly
│   ├── agent.ts
│   └── index.ts
├── theme/
│   ├── colors.ts                     # Black & red palette
│   ├── fonts.ts
│   └── index.ts
├── constants/
│   └── api.ts                        # API URLs
├── app.json
├── eas.json
└── package.json
```

---

## 🎨 Design System: Black & Red Cyberpunk

**IMPORTANT:** Match the existing web app's black and red theme exactly. Do NOT use steampunk/brass/gold colors.

### Color Palette

```typescript
export const colors = {
  // Primary - Red
  primary: {
    DEFAULT: '#ff0033',    // Main red
    light: '#ff3355',      // Light red
    dark: '#cc0028',       // Dark red
  },
  
  // Secondary - Deep Red
  secondary: {
    DEFAULT: '#990022',    // Dark red
    light: '#bb0033',
    dark: '#660018',
  },
  
  // Accent
  accent: {
    DEFAULT: '#ff4444',
    light: '#ff6666',
    dark: '#cc2222',
  },
  
  // Background - Pure Black
  background: {
    DEFAULT: '#000000',    // Pure black
    elevated: '#0a0a0a',   // Slightly elevated
    surface: '#111111',    // Surface color
    surfaceElevated: '#1a1a1a',
    surfaceHover: '#222222',
  },
  
  // Text
  text: {
    primary: '#f0f0f0',
    secondary: '#999999',
    muted: '#666666',
  },
  
  // Border - Red tinted
  border: {
    DEFAULT: 'rgba(255, 0, 51, 0.2)',
    bright: 'rgba(255, 0, 51, 0.4)',
    subtle: 'rgba(255, 255, 255, 0.05)',
  },
  
  // Glow effects
  glow: {
    primary: 'rgba(255, 0, 51, 0.3)',
    secondary: 'rgba(153, 0, 34, 0.3)',
    accent: 'rgba(255, 68, 68, 0.3)',
  },
  
  // Agent colors (keep distinct for visibility)
  agents: {
    lead: '#ff0033',      // Red (primary)
    planner: '#00d4ff',   // Cyan
    architect: '#ff00aa', // Magenta
    frontend: '#00ff66',  // Green
    backend: '#ff6600',   // Orange
    database: '#aa00ff',  // Purple
    testing: '#ffcc00',   // Yellow
    reviewer: '#ff0066',  // Pink
  },
  
  // Status
  status: {
    success: '#00ff66',
    warning: '#ffcc00',
    error: '#ff4444',
    info: '#00d4ff',
  },
};
```

### Typography

```typescript
export const fonts = {
  display: 'Orbitron',       // Futuristic display headers
  heading: 'Rajdhani',       // Technical headings
  body: 'Inter',             // Clean body text
  mono: 'Fira Code',         // Code/terminal (match web app)
};
```

### Logo Usage

**CRITICAL:** The header logo must use the enlarged `Apex-Build-Logo1.png` file, NOT small icons.

```typescript
// Logo configuration
export const logo = {
  // Use this for ALL header logos - sized appropriately
  source: require('@/assets/images/Apex-Build-Logo1.png'),
  
  // Header sizes (adjust based on screen)
  headerHeight: {
    mobile: 40,      // Height in header on phones
    tablet: 48,      // Height in header on tablets
  },
  
  // Splash screen size
  splashSize: 200,
  
  // Maintain aspect ratio - do NOT stretch
  resizeMode: 'contain',
};

// Example header component
const Header = () => (
  <View style={styles.header}>
    <Image 
      source={logo.source}
      style={{ 
        height: logo.headerHeight.mobile,
        width: undefined,
        aspectRatio: 3.5, // Adjust based on actual logo proportions
      }}
      resizeMode="contain"
    />
  </View>
);
```

### Glow Effects

```typescript
// React Native shadow styles for red glow
export const glowEffects = {
  primary: {
    shadowColor: '#ff0033',
    shadowOffset: { width: 0, height: 0 },
    shadowOpacity: 0.5,
    shadowRadius: 10,
    elevation: 10,
  },
  strong: {
    shadowColor: '#ff0033',
    shadowOffset: { width: 0, height: 0 },
    shadowOpacity: 0.7,
    shadowRadius: 20,
    elevation: 15,
  },
  subtle: {
    shadowColor: '#ff0033',
    shadowOffset: { width: 0, height: 4 },
    shadowOpacity: 0.15,
    shadowRadius: 8,
    elevation: 5,
  },
};
```

---

## 📱 Key Screens Implementation

### 1. API Client (services/api.ts)

Port the existing web API client to React Native:

```typescript
import axios, { AxiosInstance } from 'axios';
import * as SecureStore from 'expo-secure-store';

const API_BASE = 'https://apex-backend-5ypy.onrender.com/api/v1';

class ApiService {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: API_BASE,
      timeout: 30000,
      headers: { 'Content-Type': 'application/json' },
    });
    
    this.setupInterceptors();
  }

  private async getToken(): Promise<string | null> {
    return SecureStore.getItemAsync('apex_access_token');
  }

  private async setTokens(tokens: TokenResponse): Promise<void> {
    await SecureStore.setItemAsync('apex_access_token', tokens.access_token);
    await SecureStore.setItemAsync('apex_refresh_token', tokens.refresh_token);
  }

  private setupInterceptors() {
    // Request: Add auth header
    this.client.interceptors.request.use(async (config) => {
      const token = await this.getToken();
      if (token) {
        config.headers.Authorization = `Bearer ${token}`;
      }
      return config;
    });

    // Response: Handle 401, refresh token
    this.client.interceptors.response.use(
      (response) => response,
      async (error) => {
        if (error.response?.status === 401 && !error.config._retry) {
          error.config._retry = true;
          try {
            await this.refreshToken();
            const token = await this.getToken();
            error.config.headers.Authorization = `Bearer ${token}`;
            return this.client(error.config);
          } catch {
            await this.clearAuth();
            // Navigate to login
          }
        }
        return Promise.reject(error);
      }
    );
  }

  // Auth
  async login(username: string, password: string): Promise<AuthResponse> {
    const { data } = await this.client.post<AuthResponse>('/auth/login', { username, password });
    if (data.tokens) await this.setTokens(data.tokens);
    return data;
  }

  async register(req: RegisterRequest): Promise<AuthResponse> {
    const { data } = await this.client.post<AuthResponse>('/auth/register', req);
    if (data.tokens) await this.setTokens(data.tokens);
    return data;
  }

  async refreshToken(): Promise<void> {
    const refreshToken = await SecureStore.getItemAsync('apex_refresh_token');
    if (!refreshToken) throw new Error('No refresh token');
    const { data } = await this.client.post('/auth/refresh', { refresh_token: refreshToken });
    await this.setTokens(data.data);
  }

  // Projects
  async getProjects(): Promise<Project[]> {
    const { data } = await this.client.get<{ projects: Project[] }>('/projects');
    return data.projects || [];
  }

  async createProject(req: CreateProjectRequest): Promise<Project> {
    const { data } = await this.client.post<{ project: Project }>('/projects', req);
    return data.project;
  }

  async getProject(id: number): Promise<Project> {
    const { data } = await this.client.get<{ project: Project }>(`/projects/${id}`);
    return data.project;
  }

  // Files
  async getFiles(projectId: number): Promise<File[]> {
    const { data } = await this.client.get(`/projects/${projectId}/files`);
    return data.data?.files || [];
  }

  async updateFile(fileId: number, content: string): Promise<File> {
    const { data } = await this.client.put(`/files/${fileId}`, { content });
    return data.data?.file;
  }

  // AI Build
  async startBuild(description: string, mode: 'fast' | 'full'): Promise<StartBuildResponse> {
    const { data } = await this.client.post('/build/start', { description, mode });
    return data;
  }

  async getBuildStatus(buildId: string): Promise<BuildStatus> {
    const { data } = await this.client.get(`/build/${buildId}/status`);
    return data;
  }

  async sendBuildMessage(buildId: string, message: string): Promise<void> {
    await this.client.post(`/build/${buildId}/message`, { message });
  }

  // AI Generate
  async generateAI(req: AIGenerateRequest): Promise<AIGenerateResponse> {
    const { data } = await this.client.post('/ai/generate', req);
    return data;
  }

  // BYOK
  async getBYOKKeys(): Promise<BYOKKeyInfo[]> {
    const { data } = await this.client.get('/byok/keys');
    return data.data?.keys || [];
  }

  async addBYOKKey(provider: string, apiKey: string): Promise<void> {
    await this.client.post('/byok/keys', { provider, api_key: apiKey });
  }

  async removeBYOKKey(provider: string): Promise<void> {
    await this.client.delete(`/byok/keys/${provider}`);
  }

  // Deploy
  async deploy(projectId: number, config: DeploymentConfig): Promise<Deployment> {
    const { data } = await this.client.post(`/projects/${projectId}/deploy`, config);
    return data.deployment;
  }

  // Billing
  async getPlans(): Promise<Plan[]> {
    const { data } = await this.client.get('/billing/plans');
    return data.plans;
  }

  async createCheckout(plan: string): Promise<string> {
    const { data } = await this.client.post('/billing/checkout', {
      plan,
      success_url: 'apexbuild://billing/success',
      cancel_url: 'apexbuild://billing/cancel',
    });
    return data.checkout_url;
  }

  // ... implement remaining endpoints
}

export const api = new ApiService();
```

### 2. WebSocket Service (services/websocket.ts)

```typescript
import { io, Socket } from 'socket.io-client';
import * as SecureStore from 'expo-secure-store';

const WS_URL = 'wss://apex-backend-5ypy.onrender.com';

class WebSocketService {
  private socket: Socket | null = null;
  private buildSocket: WebSocket | null = null;

  // Collaboration WebSocket (Socket.io)
  async connectCollab(): Promise<void> {
    const token = await SecureStore.getItemAsync('apex_access_token');
    
    this.socket = io(`${WS_URL}/ws`, {
      auth: { token },
      transports: ['websocket', 'polling'],
    });

    this.socket.on('connect', () => console.log('Collab WebSocket connected'));
    this.socket.on('disconnect', () => console.log('Collab WebSocket disconnected'));
  }

  // Build WebSocket (native WebSocket for build progress)
  connectBuild(buildId: string, onMessage: (event: BuildWSEvent) => void): void {
    const wsUrl = `${WS_URL}/ws/build/${buildId}`;
    this.buildSocket = new WebSocket(wsUrl);

    this.buildSocket.onopen = async () => {
      const token = await SecureStore.getItemAsync('apex_access_token');
      this.buildSocket?.send(JSON.stringify({ 
        type: 'subscribe', 
        build_id: buildId, 
        token 
      }));
    };

    this.buildSocket.onmessage = (event) => {
      const data = JSON.parse(event.data);
      onMessage(data);
    };

    this.buildSocket.onerror = (error) => console.error('Build WS error:', error);
  }

  disconnectBuild(): void {
    this.buildSocket?.close();
    this.buildSocket = null;
  }

  // Collab events
  joinRoom(roomId: string): void {
    this.socket?.emit('join-room', { room_id: roomId });
  }

  leaveRoom(roomId: string): void {
    this.socket?.emit('leave-room', { room_id: roomId });
  }

  onUserJoined(callback: (data: any) => void): void {
    this.socket?.on('user-joined', callback);
  }

  onFileChanged(callback: (data: any) => void): void {
    this.socket?.on('file-changed', callback);
  }

  sendFileChange(fileId: number, content: string, line: number, column: number): void {
    this.socket?.emit('file-change', { file_id: fileId, content, line, column });
  }

  disconnect(): void {
    this.socket?.disconnect();
    this.buildSocket?.close();
  }
}

export const ws = new WebSocketService();
```

### 3. Build Progress Screen (app/build/[buildId].tsx)

```tsx
import { useEffect, useState } from 'react';
import { View, Text, ScrollView } from 'react-native';
import { useLocalSearchParams } from 'expo-router';
import Animated, { FadeIn, SlideInRight } from 'react-native-reanimated';
import { api } from '@/services/api';
import { ws } from '@/services/websocket';
import { AgentOrchestration } from '@/components/agent/AgentOrchestration';
import { BuildProgress } from '@/components/agent/BuildProgress';
import { LoadingSpinner } from '@/components/ui/LoadingSpinner';
import { colors } from '@/theme';

export default function BuildProgressScreen() {
  const { buildId } = useLocalSearchParams<{ buildId: string }>();
  const [status, setStatus] = useState<BuildStatus | null>(null);
  const [agents, setAgents] = useState<AgentStatus[]>([]);
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [logs, setLogs] = useState<string[]>([]);

  useEffect(() => {
    // Initial status fetch
    api.getBuildStatus(buildId).then(setStatus);

    // Connect to build WebSocket
    ws.connectBuild(buildId, (event) => {
      switch (event.type) {
        case 'build:progress':
          setStatus(prev => prev ? { ...prev, progress: event.progress, phase: event.phase } : null);
          break;
        case 'agent:progress':
          setAgents(prev => {
            const idx = prev.findIndex(a => a.role === event.data.role);
            if (idx >= 0) {
              const updated = [...prev];
              updated[idx] = { ...updated[idx], ...event.data };
              return updated;
            }
            return [...prev, event.data];
          });
          break;
        case 'file:created':
        case 'file:updated':
          setFiles(prev => [...prev, event.file]);
          break;
        case 'agent:message':
          setLogs(prev => [...prev, `[${event.agent_id}] ${event.message}`]);
          break;
        case 'build:completed':
          setStatus(prev => prev ? { ...prev, status: 'completed', progress: 100 } : null);
          break;
        case 'build:error':
          setStatus(prev => prev ? { ...prev, status: 'failed', error: event.error } : null);
          break;
      }
    });

    return () => ws.disconnectBuild();
  }, [buildId]);

  if (!status) {
    return (
      <View style={{ flex: 1, justifyContent: 'center', alignItems: 'center', backgroundColor: colors.background.DEFAULT }}>
        <LoadingSpinner size={80} color={colors.primary.DEFAULT} />
        <Text style={{ color: colors.text.secondary, marginTop: 16 }}>Connecting to build...</Text>
      </View>
    );
  }

  return (
    <ScrollView style={{ flex: 1, backgroundColor: colors.background.DEFAULT }}>
      {/* Header */}
      <View style={{ padding: 20 }}>
        <Text style={{ fontSize: 24, fontWeight: 'bold', color: colors.text.primary }}>
          Building Your App
        </Text>
        <Text style={{ color: colors.text.secondary, marginTop: 4 }}>
          {status.phase} • {Math.round(status.progress)}% complete
        </Text>
      </View>

      {/* Progress bar */}
      <BuildProgress progress={status.progress} phase={status.phase} />

      {/* Agent orchestration visualization */}
      <AgentOrchestration agents={agents} />

      {/* Files generated */}
      <View style={{ padding: 20 }}>
        <Text style={{ fontSize: 18, fontWeight: '600', color: colors.primary.DEFAULT, marginBottom: 12 }}>
          📁 Files Generated ({files.length})
        </Text>
        {files.slice(-10).map((file, i) => (
          <Animated.View 
            key={i} 
            entering={SlideInRight.delay(i * 50)}
            style={{ 
              backgroundColor: colors.background.surface, 
              padding: 12, 
              borderRadius: 8, 
              marginBottom: 8,
              borderLeftWidth: 3,
              borderLeftColor: colors.primary.DEFAULT,
            }}
          >
            <Text style={{ color: colors.text.primary, fontFamily: 'Fira Code' }}>
              {file.path}/{file.name}
            </Text>
          </Animated.View>
        ))}
      </View>

      {/* Live logs */}
      <View style={{ padding: 20 }}>
        <Text style={{ fontSize: 18, fontWeight: '600', color: colors.primary.DEFAULT, marginBottom: 12 }}>
          📝 Agent Activity
        </Text>
        <View style={{ 
          backgroundColor: '#000', 
          borderRadius: 8, 
          padding: 12,
          maxHeight: 200,
          borderWidth: 1,
          borderColor: colors.border.DEFAULT,
        }}>
          {logs.slice(-20).map((log, i) => (
            <Text key={i} style={{ color: colors.primary.light, fontFamily: 'Fira Code', fontSize: 12 }}>
              {log}
            </Text>
          ))}
        </View>
      </View>
    </ScrollView>
  );
}
```

### 4. Auth Store (stores/authStore.ts)

```typescript
import { create } from 'zustand';
import * as SecureStore from 'expo-secure-store';
import { api } from '@/services/api';
import { User, AuthResponse } from '@/types';

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  
  login: (username: string, password: string) => Promise<void>;
  register: (data: RegisterRequest) => Promise<void>;
  logout: () => Promise<void>;
  loadUser: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isAuthenticated: false,
  isLoading: true,

  login: async (username, password) => {
    const response = await api.login(username, password);
    set({ user: response.user as User, isAuthenticated: true });
  },

  register: async (data) => {
    const response = await api.register(data);
    set({ user: response.user as User, isAuthenticated: true });
  },

  logout: async () => {
    await SecureStore.deleteItemAsync('apex_access_token');
    await SecureStore.deleteItemAsync('apex_refresh_token');
    set({ user: null, isAuthenticated: false });
  },

  loadUser: async () => {
    try {
      const token = await SecureStore.getItemAsync('apex_access_token');
      if (token) {
        const user = await api.getUserProfile();
        set({ user, isAuthenticated: true, isLoading: false });
      } else {
        set({ isLoading: false });
      }
    } catch {
      set({ isLoading: false });
    }
  },
}));
```

---

## 📱 App Store Requirements

### iOS App Store

```yaml
app_store:
  name: "APEX.BUILD"
  subtitle: "AI-Powered App Builder"
  category: "Developer Tools"
  
  requirements:
    - Sign in with Apple (required if any OAuth)
    - Privacy Policy URL
    - App Review notes with test account
    
  assets:
    icon: "1024x1024 PNG"
    screenshots:
      - "iPhone 6.5 inch (1284x2778)"
      - "iPhone 5.5 inch (1242x2208)"
      - "iPad 12.9 inch (2048x2732)"
```

### Google Play Store

```yaml
play_store:
  name: "APEX.BUILD - AI App Builder"
  category: "Tools"
  content_rating: "Everyone"
  
  requirements:
    - Privacy Policy URL
    - Data safety form
    - Target API level 34+
    
  assets:
    icon: "512x512 PNG"
    feature_graphic: "1024x500 PNG"
    screenshots: "Min 2, 16:9 or 9:16"
```

### RevenueCat Products

```typescript
const PRODUCTS = {
  ios: {
    pro_monthly: 'apex_pro_monthly',
    pro_yearly: 'apex_pro_yearly',
    team_monthly: 'apex_team_monthly',
  },
  android: {
    pro_monthly: 'apex_pro_monthly',
    pro_yearly: 'apex_pro_yearly',
    team_monthly: 'apex_team_monthly',
  },
};
```

---

## ✅ Implementation Checklist

### Core Features
- [ ] Authentication (login, register, biometric, Apple Sign-In)
- [ ] Project list with CRUD
- [ ] AI Build wizard with real-time progress via WebSocket
- [ ] Code editor with syntax highlighting
- [ ] File browser
- [ ] BYOK API key management
- [ ] Deployment to all providers
- [ ] Subscription management via RevenueCat

### UI/UX
- [ ] Black & red cyberpunk design system
- [ ] Enlarged Apex-Build-Logo1.png in all headers
- [ ] Responsive for phones AND tablets
- [ ] Loading states with red glow spinner
- [ ] Error boundaries
- [ ] Pull-to-refresh
- [ ] Haptic feedback

### App Store Ready
- [ ] App icons (all sizes)
- [ ] Splash screen
- [ ] Screenshots
- [ ] Sign in with Apple
- [ ] Privacy policy
- [ ] EAS Build configuration

---

## 🚀 Build Commands

```bash
# Create project
npx create-expo-app apex-mobile --template tabs

# Install dependencies
npx expo install expo-router expo-secure-store expo-local-authentication \
  expo-apple-authentication axios socket.io-client zustand @tanstack/react-query \
  react-native-reanimated moti react-native-purchases

# Development
npx expo start

# Build
eas build --platform ios --profile production
eas build --platform android --profile production

# Submit
eas submit --platform ios
eas submit --platform android
```

---

## 🎯 Success Criteria

1. **Users can authenticate** against the existing APEX.BUILD backend
2. **Users can create projects** and view their existing projects
3. **AI Build works** with real-time WebSocket progress showing agents
4. **Code editor** displays and edits files from the backend
5. **BYOK management** lets users add/remove their API keys
6. **Subscriptions work** via RevenueCat connected to Stripe
7. **Apps pass review** on first submission to both stores
8. **Black & red aesthetic** matches the web app exactly with proper logo sizing

---

**Now build the APEX.BUILD mobile clients that connect to the existing backend!** 🚀⚙️
