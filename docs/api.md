# APEX.BUILD API Reference

Complete REST API documentation for the APEX.BUILD platform.

**Base URL**: `https://your-domain.com/api/v1`

---

## Table of Contents

- [Authentication](#authentication)
- [Users](#users)
- [Projects](#projects)
- [Files](#files)
- [AI Generation](#ai-generation)
- [Build System](#build-system)
- [Code Execution](#code-execution)
- [Terminal](#terminal)
- [Deployment](#deployment)
- [Secrets Management](#secrets-management)
- [MCP Integration](#mcp-integration)
- [Git Operations](#git-operations)
- [Billing](#billing)
- [Admin](#admin)
- [WebSocket Protocol](#websocket-protocol)
- [Error Codes](#error-codes)

---

## Authentication

All authenticated endpoints require a Bearer token in the `Authorization` header:

```
Authorization: Bearer <access_token>
```

### Register User

Create a new user account.

```http
POST /auth/register
Content-Type: application/json

{
  "username": "johndoe",
  "email": "john@example.com",
  "password": "securepassword123",
  "full_name": "John Doe"
}
```

**Response** (201 Created):
```json
{
  "success": true,
  "message": "User registered successfully",
  "data": {
    "user": {
      "id": 1,
      "username": "johndoe",
      "email": "john@example.com",
      "full_name": "John Doe",
      "subscription_type": "free"
    },
    "tokens": {
      "access_token": "eyJhbGciOiJIUzI1NiIs...",
      "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
      "expires_at": "2024-01-15T12:00:00Z",
      "token_type": "Bearer"
    }
  }
}
```

### Login

Authenticate an existing user.

```http
POST /auth/login
Content-Type: application/json

{
  "username": "johndoe",
  "password": "securepassword123"
}
```

**Response** (200 OK):
```json
{
  "success": true,
  "message": "Login successful",
  "data": {
    "user": {
      "id": 1,
      "username": "johndoe",
      "email": "john@example.com",
      "subscription_type": "free",
      "preferred_theme": "cyberpunk",
      "preferred_ai": "auto"
    },
    "tokens": {
      "access_token": "eyJhbGciOiJIUzI1NiIs...",
      "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
      "expires_at": "2024-01-15T12:00:00Z",
      "token_type": "Bearer"
    }
  }
}
```

### Refresh Token

Get a new access token using a refresh token.

```http
POST /auth/refresh
Content-Type: application/json

{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_at": "2024-01-15T12:15:00Z",
    "token_type": "Bearer"
  }
}
```

### Token Lifecycle

| Token Type | Duration | Purpose |
|------------|----------|---------|
| Access Token | 15 minutes | API authentication |
| Refresh Token | 7 days | Obtain new access tokens |

---

## Users

### Get Current User Profile

```http
GET /user/profile
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "id": 1,
    "username": "johndoe",
    "email": "john@example.com",
    "full_name": "John Doe",
    "avatar_url": "https://...",
    "subscription_type": "pro",
    "subscription_end": "2024-12-31T23:59:59Z",
    "is_verified": true,
    "preferred_theme": "cyberpunk",
    "preferred_ai": "auto",
    "monthly_ai_requests": 150,
    "monthly_ai_cost": 12.50,
    "created_at": "2024-01-01T00:00:00Z",
    "project_count": 5
  }
}
```

### Update User Profile

```http
PUT /user/profile
Authorization: Bearer <token>
Content-Type: application/json

{
  "full_name": "John D. Doe",
  "avatar_url": "https://example.com/avatar.jpg",
  "preferred_theme": "matrix",
  "preferred_ai": "claude"
}
```

**Valid Themes**: `cyberpunk`, `matrix`, `synthwave`, `neonCity`

**Valid AI Preferences**: `auto`, `claude`, `gpt4`, `gemini`

---

## Projects

### Create Project

```http
POST /projects
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "my-react-app",
  "description": "A React application with Tailwind CSS",
  "language": "typescript",
  "framework": "react",
  "is_public": false,
  "environment": {
    "NODE_ENV": "development"
  }
}
```

**Response** (201 Created):
```json
{
  "message": "Project created successfully",
  "project": {
    "id": 1,
    "name": "my-react-app",
    "description": "A React application with Tailwind CSS",
    "language": "typescript",
    "framework": "react",
    "owner_id": 1,
    "is_public": false,
    "root_directory": "/",
    "created_at": "2024-01-15T10:00:00Z"
  }
}
```

### List Projects

```http
GET /projects
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "projects": [
    {
      "id": 1,
      "name": "my-react-app",
      "language": "typescript",
      "framework": "react",
      "created_at": "2024-01-15T10:00:00Z"
    }
  ]
}
```

### Get Project Details

```http
GET /projects/:id
Authorization: Bearer <token>
```

### Download Project as ZIP

```http
GET /projects/:id/download
Authorization: Bearer <token>
```

Returns a ZIP file containing all project files.

---

## Files

### Create File

```http
POST /projects/:projectId/files
Authorization: Bearer <token>
Content-Type: application/json

{
  "path": "/src",
  "name": "App.tsx",
  "type": "file",
  "content": "import React from 'react';\n\nexport default function App() {\n  return <div>Hello World</div>;\n}",
  "mime_type": "text/typescript"
}
```

### List Project Files

```http
GET /projects/:projectId/files
Authorization: Bearer <token>
```

### Update File Content

```http
PUT /files/:id
Authorization: Bearer <token>
Content-Type: application/json

{
  "content": "// Updated content here"
}
```

---

## AI Generation

### Generate with AI

Request AI-powered code generation, review, or assistance.

```http
POST /ai/generate
Authorization: Bearer <token>
Content-Type: application/json

{
  "capability": "code_generation",
  "prompt": "Create a React component for a user profile card",
  "language": "typescript",
  "context": {
    "framework": "react",
    "styling": "tailwind"
  },
  "max_tokens": 2000,
  "temperature": 0.7,
  "project_id": "1"
}
```

**AI Capabilities**:

| Capability | Description | Best Provider |
|------------|-------------|---------------|
| `code_generation` | Generate new code | GPT-4 |
| `natural_language_to_code` | Convert descriptions to code | Claude |
| `code_review` | Review and suggest improvements | Claude |
| `code_completion` | Complete partial code | Gemini |
| `debugging` | Find and fix bugs | Claude |
| `explanation` | Explain code behavior | Gemini |
| `refactoring` | Improve code structure | GPT-4 |
| `testing` | Generate tests | GPT-4 |
| `documentation` | Write documentation | Claude |
| `architecture` | Design system architecture | Claude |

**Response** (200 OK):
```json
{
  "request_id": "uuid-1234",
  "provider": "gpt4",
  "content": "```typescript\nimport React from 'react';\n\ninterface ProfileCardProps {\n  name: string;\n  email: string;\n  avatar: string;\n}\n\nexport const ProfileCard: React.FC<ProfileCardProps> = ({ name, email, avatar }) => {\n  return (\n    <div className=\"bg-white rounded-lg shadow-md p-6\">\n      <img src={avatar} alt={name} className=\"w-24 h-24 rounded-full mx-auto\" />\n      <h2 className=\"text-xl font-bold text-center mt-4\">{name}</h2>\n      <p className=\"text-gray-600 text-center\">{email}</p>\n    </div>\n  );\n};\n```",
  "usage": {
    "prompt_tokens": 45,
    "completion_tokens": 120,
    "total_tokens": 165,
    "cost": 0.0049
  },
  "duration": 1250,
  "created_at": "2024-01-15T10:30:00Z"
}
```

### Get AI Usage Statistics

```http
GET /ai/usage
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "total_requests": 150,
  "total_cost": 12.50,
  "total_tokens": 45000,
  "by_provider": {
    "claude": { "requests": 60, "cost": 5.00, "tokens": 18000 },
    "gpt4": { "requests": 50, "cost": 6.00, "tokens": 15000 },
    "gemini": { "requests": 40, "cost": 1.50, "tokens": 12000 }
  }
}
```

---

## Build System

The build system uses autonomous AI agents to generate complete applications.

### Start Build

```http
POST /build/start
Authorization: Bearer <token>
Content-Type: application/json

{
  "description": "Create a task management app with user authentication, project boards, and drag-and-drop tasks",
  "mode": "full"
}
```

**Build Modes**:
- `fast` - Quick build (3-5 minutes)
- `full` - Comprehensive build (10+ minutes)

**Response** (200 OK):
```json
{
  "build_id": "build-uuid-1234",
  "websocket_url": "wss://api.apex.build/ws/build/build-uuid-1234",
  "status": "planning"
}
```

### Get Build Status

```http
GET /build/:buildId/status
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "build_id": "build-uuid-1234",
  "status": "in_progress",
  "progress": 45,
  "phase": "generating",
  "agents": [
    { "role": "frontend", "status": "working", "progress": 60 },
    { "role": "backend", "status": "working", "progress": 40 },
    { "role": "database", "status": "completed", "progress": 100 }
  ],
  "tasks_completed": 12,
  "tasks_total": 28,
  "files_generated": 15,
  "duration_ms": 180000
}
```

### Get Build Files

```http
GET /build/:buildId/files
Authorization: Bearer <token>
```

### Send Message to Build

Communicate with the Lead Agent during a build.

```http
POST /build/:buildId/message
Authorization: Bearer <token>
Content-Type: application/json

{
  "message": "Add dark mode support to the UI"
}
```

### Cancel Build

```http
POST /build/:buildId/cancel
Authorization: Bearer <token>
```

### Get Build Checkpoints

```http
GET /build/:buildId/checkpoints
Authorization: Bearer <token>
```

### Rollback to Checkpoint

```http
POST /build/:buildId/rollback/:checkpointId
Authorization: Bearer <token>
```

---

## Code Execution

### Execute Code

```http
POST /execute
Authorization: Bearer <token>
Content-Type: application/json

{
  "project_id": 1,
  "command": "node index.js",
  "language": "javascript",
  "input": "",
  "environment": {
    "NODE_ENV": "development"
  }
}
```

**Supported Languages**:
- JavaScript / TypeScript (Node.js)
- Python 3
- Go
- Rust
- Java
- C / C++
- Ruby
- PHP

### Get Execution Status

```http
GET /execute/:id
Authorization: Bearer <token>
```

### Stop Execution

```http
POST /execute/:id/stop
Authorization: Bearer <token>
```

### Get Supported Languages

```http
GET /execute/languages
Authorization: Bearer <token>
```

---

## Terminal

### Create Terminal Session

```http
POST /terminal/sessions
Authorization: Bearer <token>
Content-Type: application/json

{
  "project_id": 1
}
```

**Response** (201 Created):
```json
{
  "session_id": "term-uuid-1234",
  "websocket_url": "wss://api.apex.build/ws/terminal/term-uuid-1234"
}
```

### List Terminal Sessions

```http
GET /terminal/sessions
Authorization: Bearer <token>
```

### Delete Terminal Session

```http
DELETE /terminal/sessions/:id
Authorization: Bearer <token>
```

### Resize Terminal

```http
POST /terminal/sessions/:id/resize
Authorization: Bearer <token>
Content-Type: application/json

{
  "cols": 120,
  "rows": 40
}
```

---

## Deployment

### Start Deployment

```http
POST /deploy
Authorization: Bearer <token>
Content-Type: application/json

{
  "project_id": 1,
  "provider": "vercel",
  "environment": "production",
  "environment_variables": {
    "API_URL": "https://api.example.com"
  }
}
```

**Supported Providers**: `vercel`, `netlify`, `render`

### Get Deployment Status

```http
GET /deploy/:id/status
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "id": "deploy-uuid-1234",
  "status": "deployed",
  "provider": "vercel",
  "url": "https://my-app.vercel.app",
  "logs": ["Building...", "Deploying...", "Success!"],
  "created_at": "2024-01-15T11:00:00Z",
  "deployed_at": "2024-01-15T11:02:00Z"
}
```

### Get Deployment Logs

```http
GET /deploy/:id/logs
Authorization: Bearer <token>
```

### Redeploy

```http
POST /deploy/:id/redeploy
Authorization: Bearer <token>
```

### Get Available Providers

```http
GET /deploy/providers
Authorization: Bearer <token>
```

---

## Secrets Management

### List Secrets

```http
GET /secrets
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "secrets": [
    {
      "id": 1,
      "name": "DATABASE_URL",
      "description": "PostgreSQL connection string",
      "project_id": 1,
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-15T10:00:00Z"
    }
  ]
}
```

Note: Secret values are never returned in API responses.

### Create Secret

```http
POST /secrets
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "API_KEY",
  "value": "sk-1234567890",
  "description": "External API key",
  "project_id": 1
}
```

### Update Secret

```http
PUT /secrets/:id
Authorization: Bearer <token>
Content-Type: application/json

{
  "value": "sk-new-value"
}
```

### Delete Secret

```http
DELETE /secrets/:id
Authorization: Bearer <token>
```

### Rotate Secret

```http
POST /secrets/:id/rotate
Authorization: Bearer <token>
```

### Get Secret Audit Log

```http
GET /secrets/:id/audit
Authorization: Bearer <token>
```

---

## MCP Integration

APEX.BUILD can connect to external MCP (Model Context Protocol) servers.

### List External MCP Servers

```http
GET /mcp/servers
Authorization: Bearer <token>
```

### Add External MCP Server

```http
POST /mcp/servers
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "GitHub Tools",
  "url": "wss://mcp.github.com/v1",
  "api_key": "ghp_xxxx"
}
```

### Connect to MCP Server

```http
POST /mcp/servers/:id/connect
Authorization: Bearer <token>
```

### Disconnect from MCP Server

```http
POST /mcp/servers/:id/disconnect
Authorization: Bearer <token>
```

### Call MCP Tool

```http
POST /mcp/servers/:id/tools/call
Authorization: Bearer <token>
Content-Type: application/json

{
  "tool": "search_code",
  "arguments": {
    "query": "authentication",
    "repo": "owner/repo"
  }
}
```

### Get Available Tools

```http
GET /mcp/tools
Authorization: Bearer <token>
```

---

## Git Operations

### Connect Repository

```http
POST /git/connect
Authorization: Bearer <token>
Content-Type: application/json

{
  "project_id": 1,
  "repo_url": "https://github.com/owner/repo.git",
  "branch": "main"
}
```

### Get Repository Info

```http
GET /git/repo/:projectId
Authorization: Bearer <token>
```

### List Branches

```http
GET /git/branches/:projectId
Authorization: Bearer <token>
```

### Get Commits

```http
GET /git/commits/:projectId
Authorization: Bearer <token>
```

### Create Commit

```http
POST /git/commit
Authorization: Bearer <token>
Content-Type: application/json

{
  "project_id": 1,
  "message": "Add user authentication",
  "files": [
    { "path": "src/auth.ts", "action": "add" }
  ]
}
```

### Push to Remote

```http
POST /git/push
Authorization: Bearer <token>
Content-Type: application/json

{
  "project_id": 1,
  "branch": "main"
}
```

### Pull from Remote

```http
POST /git/pull
Authorization: Bearer <token>
Content-Type: application/json

{
  "project_id": 1,
  "branch": "main"
}
```

### Create Branch

```http
POST /git/branch
Authorization: Bearer <token>
Content-Type: application/json

{
  "project_id": 1,
  "name": "feature/new-feature",
  "from": "main"
}
```

### Create Pull Request

```http
POST /git/pulls
Authorization: Bearer <token>
Content-Type: application/json

{
  "project_id": 1,
  "title": "Add user authentication",
  "description": "Implements JWT-based authentication",
  "head": "feature/auth",
  "base": "main"
}
```

---

## Billing

### Get Subscription Plans

```http
GET /billing/plans
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "plans": [
    {
      "id": "free",
      "name": "Free",
      "price": 0,
      "features": [
        "3 projects",
        "500 AI requests/month",
        "Community support"
      ]
    },
    {
      "id": "pro",
      "name": "Pro",
      "price": 12,
      "features": [
        "Unlimited projects",
        "5000 AI requests/month",
        "Priority support",
        "Custom domains"
      ]
    },
    {
      "id": "team",
      "name": "Team",
      "price": 29,
      "features": [
        "Everything in Pro",
        "10 team members",
        "Collaboration features",
        "Admin dashboard"
      ]
    },
    {
      "id": "enterprise",
      "name": "Enterprise",
      "price": 99,
      "features": [
        "Everything in Team",
        "Unlimited team members",
        "SSO integration",
        "Dedicated support",
        "SLA guarantee"
      ]
    }
  ]
}
```

### Create Checkout Session

```http
POST /billing/checkout
Authorization: Bearer <token>
Content-Type: application/json

{
  "plan": "pro",
  "success_url": "https://apex.build/billing/success",
  "cancel_url": "https://apex.build/billing/cancel"
}
```

### Get Current Subscription

```http
GET /billing/subscription
Authorization: Bearer <token>
```

### Cancel Subscription

```http
POST /billing/cancel
Authorization: Bearer <token>
```

### Get Usage Statistics

```http
GET /billing/usage
Authorization: Bearer <token>
```

### Get Invoices

```http
GET /billing/invoices
Authorization: Bearer <token>
```

---

## Admin

Admin endpoints require admin privileges.

### Get Dashboard Statistics

```http
GET /admin/dashboard
Authorization: Bearer <admin_token>
```

**Response** (200 OK):
```json
{
  "users": {
    "total": 1500,
    "active": 1200,
    "new_today": 25
  },
  "projects": {
    "total": 5000,
    "created_today": 120
  },
  "ai_usage": {
    "requests_today": 15000,
    "cost_today": 250.00
  },
  "builds": {
    "active": 45,
    "completed_today": 320
  }
}
```

### List Users

```http
GET /admin/users?page=1&limit=20
Authorization: Bearer <admin_token>
```

### Get User Details

```http
GET /admin/users/:id
Authorization: Bearer <admin_token>
```

### Update User

```http
PUT /admin/users/:id
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "is_active": true,
  "subscription_type": "pro"
}
```

### Delete User

```http
DELETE /admin/users/:id
Authorization: Bearer <admin_token>
```

### Add Credits

```http
POST /admin/users/:id/credits
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "amount": 50.00,
  "reason": "Promotional credit"
}
```

### Get System Statistics

```http
GET /admin/stats
Authorization: Bearer <admin_token>
```

---

## WebSocket Protocol

### Build WebSocket

Connect to receive real-time build updates:

```
wss://api.apex.build/ws/build/:buildId
```

**Message Types**:

| Type | Description |
|------|-------------|
| `build:started` | Build has started |
| `build:progress` | Build progress update |
| `build:checkpoint` | Checkpoint created |
| `build:completed` | Build completed successfully |
| `build:error` | Build error occurred |
| `agent:spawned` | New agent spawned |
| `agent:working` | Agent started working |
| `agent:progress` | Agent progress update |
| `agent:completed` | Agent finished task |
| `agent:message` | Message from agent |
| `file:created` | New file generated |
| `file:updated` | File was updated |
| `user:message` | User sent message |
| `lead:response` | Lead agent response |

**Example Message**:
```json
{
  "type": "agent:progress",
  "build_id": "build-uuid-1234",
  "agent_id": "agent-frontend-1",
  "timestamp": "2024-01-15T10:35:00Z",
  "data": {
    "role": "frontend",
    "status": "working",
    "progress": 75,
    "message": "Creating React components..."
  }
}
```

### Terminal WebSocket

Connect for interactive terminal session:

```
wss://api.apex.build/ws/terminal/:sessionId
```

### Collaboration WebSocket

Connect for real-time collaboration:

```
wss://api.apex.build/ws/collab?room_id=<roomId>&project_id=<projectId>
```

**Message Types**:

| Type | Description |
|------|-------------|
| `join_room` | User joined room |
| `leave_room` | User left room |
| `cursor_update` | Cursor position change |
| `file_change` | File content change |
| `chat` | Chat message |
| `user_joined` | New user notification |
| `user_left` | User left notification |
| `user_list` | Current user list |

---

## Error Codes

All error responses follow this format:

```json
{
  "success": false,
  "error": "Error message description",
  "code": "ERROR_CODE"
}
```

### HTTP Status Codes

| Status | Description |
|--------|-------------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 409 | Conflict |
| 422 | Validation Error |
| 429 | Rate Limit Exceeded |
| 500 | Internal Server Error |

### Error Codes Reference

| Code | Description |
|------|-------------|
| `INVALID_REQUEST` | Request body is malformed |
| `INVALID_CREDENTIALS` | Wrong username or password |
| `USER_EXISTS` | Username or email already taken |
| `USER_NOT_FOUND` | User does not exist |
| `ACCOUNT_DISABLED` | User account is disabled |
| `TOKEN_EXPIRED` | JWT token has expired |
| `TOKEN_INVALID` | JWT token is invalid |
| `NOT_AUTHENTICATED` | Authentication required |
| `NOT_AUTHORIZED` | Insufficient permissions |
| `PROJECT_NOT_FOUND` | Project does not exist |
| `FILE_NOT_FOUND` | File does not exist |
| `BUILD_NOT_FOUND` | Build does not exist |
| `RATE_LIMIT_EXCEEDED` | Too many requests |
| `AI_PROVIDER_ERROR` | AI provider returned error |
| `EXECUTION_TIMEOUT` | Code execution timed out |
| `DEPLOY_FAILED` | Deployment failed |
| `DATABASE_ERROR` | Database operation failed |

---

## Rate Limits

| Endpoint Category | Limit |
|-------------------|-------|
| Authentication | 10 requests/minute |
| AI Generation | 100 requests/minute (varies by plan) |
| File Operations | 200 requests/minute |
| Build Operations | 10 builds/hour |
| General API | 500 requests/minute |

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1705312800
```

---

## SDK Examples

### JavaScript/TypeScript

```typescript
import { ApiService } from './api';

const api = new ApiService('https://api.apex.build/api/v1');

// Login
const auth = await api.login({
  username: 'johndoe',
  password: 'password123'
});

// Create project
const project = await api.createProject({
  name: 'my-app',
  language: 'typescript',
  framework: 'react'
});

// Generate code with AI
const result = await api.generateAI({
  capability: 'code_generation',
  prompt: 'Create a login form component',
  language: 'typescript'
});

console.log(result.content);
```

### cURL

```bash
# Login
curl -X POST https://api.apex.build/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "johndoe", "password": "password123"}'

# Create project (with token)
curl -X POST https://api.apex.build/api/v1/projects \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-app", "language": "typescript"}'

# Generate with AI
curl -X POST https://api.apex.build/api/v1/ai/generate \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"capability": "code_generation", "prompt": "Create a button component"}'
```

---

**API Version**: 1.0.0

**Last Updated**: January 2024
