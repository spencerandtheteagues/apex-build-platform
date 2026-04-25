# APEX.BUILD API Contract v1.0
**Single Source of Truth for ALL Frontend/Backend Contracts**

> **RULE:** Every agent MUST read this file before implementing ANY feature that crosses the frontend/backend boundary.
> **RULE:** Any contract change requires updating this file FIRST, then backend, then frontend.
> **RULE:** Run `scripts/verify-contract.sh` after every deploy to verify live endpoints match this contract.

---

## Table of Contents
1. [Base URLs](#base-urls)
2. [Authentication](#authentication)
3. [HTTP Endpoints](#http-endpoints)
4. [WebSocket Events](#websocket-events)
5. [Type Mappings](#type-mappings)
6. [Common Response Shapes](#common-response-shapes)
7. [Error Codes](#error-codes)
8. [Auth Header Rules](#auth-header-rules)

---

## Base URLs

| Environment | Frontend | Backend API | WebSocket |
|-------------|----------|-------------|-----------|
| Production | https://apex-build.dev | https://apex-backend-5ypy.onrender.com/api/v1 | wss://apex-backend-5ypy.onrender.com/ws |
| Local | http://localhost:5173 | http://localhost:8080/api/v1 | ws://localhost:8080/ws |

**Note:** The frontend `api.ts` uses `/api/v1` as fallback (proxied in dev), and `DEFAULT_PRODUCTION_API_BASE_URL = 'https://api.apex-build.dev/api/v1'` for production.

---

## Authentication

### Token Storage
- Backend: HTTP-only secure cookies (primary) + JSON response (fallback)
- Frontend: Stores tokens via `authSession.ts`, reads from `document.cookie`

### Auth Flow
```
POST /auth/register     → {username, email, password} → AuthResponse
POST /auth/login        → {username_or_email, password} → AuthResponse
POST /auth/refresh      → (cookie) → TokenResponse
POST /auth/logout       → (cookie) → {message}
POST /auth/verify-email → {code, email?} → AuthResponse
POST /auth/resend-verification → {email?} → {message}
GET  /user/profile      → (auth) → User
PUT  /user/profile      → (auth) → User
```

### AuthResponse Shape
```typescript
interface AuthResponse {
  access_token: string
  refresh_token: string
  access_token_expires_at: string
  refresh_token_expires_at: string
  user: User
}
```

### TokenResponse Shape
```typescript
interface TokenResponse {
  access_token: string
  refresh_token: string
  access_token_expires_at: string
  refresh_token_expires_at: string
}
```

### Auth Headers
- All protected endpoints require: `Authorization: Bearer <access_token>` OR valid cookie session
- CSRF protection: `X-CSRF-Token` header on all mutating requests (POST/PUT/DELETE/PATCH)
- Frontend fetches CSRF token via: `GET /api/v1/csrf-token`

---

## HTTP Endpoints

### Endpoint Table Format
```
METHOD /path
- Auth: required | optional | none
- Backend: handler.go:funcName
- Frontend: api.ts:methodName
- Request: {shape}
- Response: {shape}
- Status Codes: 200, 401, etc.
- Contract Status: ✅ MATCH | ⚠️ MISMATCH | ❌ MISSING
```

---

### Health & Status (No Auth)

#### GET /health
- Auth: none
- Backend: `backend/internal/api/handlers.go:Health`
- Response: `{status: "ok", version: string, uptime: string}`

#### GET /health/deep
- Auth: none
- Backend: `backend/internal/api/handlers.go:DeepHealth`
- Response: `{overall: string, checks: Record<string, {status, latency}>}`

#### GET /health/features
- Auth: none
- Backend: `backend/internal/api/handlers.go:FeatureReadiness`
- Response: `FeatureReadinessSummary` (see types below)

#### GET /ready
- Auth: none
- Response: same as deep health (k8s readiness probe)

#### GET /metrics
- Auth: none
- Response: Prometheus metrics text

---

### Authentication Endpoints

#### POST /api/v1/auth/register
- Auth: none
- Backend: `backend/internal/api/handlers.go:Register`
- Frontend: `api.ts:register()`
- Request: `{username, email, password, full_name?}`
- Response: `AuthResponse`
- Status: 201 Created, 400 Bad Request, 409 Conflict

#### POST /api/v1/auth/login
- Auth: none
- Backend: `backend/internal/api/handlers.go:Login`
- Frontend: `api.ts:login()`
- Request: `{username_or_email, password}`
- Response: `AuthResponse`
- Status: 200, 401 Unauthorized, 403 Forbidden (unverified)

#### POST /api/v1/auth/refresh
- Auth: refresh cookie
- Backend: `backend/internal/api/handlers.go:RefreshToken`
- Frontend: `api.ts:refreshToken()`
- Request: none (cookie-based)
- Response: `TokenResponse`
- Status: 200, 401

#### POST /api/v1/auth/logout
- Auth: required
- Backend: `backend/internal/api/handlers.go:Logout`
- Frontend: `api.ts:logout()`
- Response: `{message: "Logged out successfully"}`

#### POST /api/v1/auth/verify-email
- Auth: optional ( Bearer for authenticated, body email for unauthenticated)
- Backend: `backend/internal/api/verification.go:VerifyEmail`
- Frontend: `api.ts:verifyEmail()`
- Request: `{code, email?}`
- Response: `AuthResponse`

#### POST /api/v1/auth/resend-verification
- Auth: optional
- Backend: `backend/internal/api/verification.go:ResendVerification`
- Frontend: `api.ts:resendVerification()`
- Request: `{email?}`
- Response: `{message: string}`

---

### User Endpoints

#### GET /api/v1/user/profile
- Auth: required
- Backend: `backend/internal/api/handlers.go:GetUserProfile`
- Frontend: `api.ts:getUserProfile()`
- Response: `User`

#### PUT /api/v1/user/profile
- Auth: required
- Backend: `backend/internal/api/handlers.go:UpdateUserProfile`
- Frontend: `api.ts:updateUserProfile()`
- Request: partial `User` fields
- Response: `User`

---

### Project Endpoints

#### POST /api/v1/projects
- Auth: required
- Backend: `backend/internal/handlers/projects_optimized.go:CreateProjectOptimized`
- Frontend: `api.ts:createProject()`
- Request: `{name, description?, language, framework?, is_public?}`
- Response: `Project`
- Status: 201

#### GET /api/v1/projects
- Auth: required
- Backend: `backend/internal/handlers/projects_optimized.go:GetProjectsOptimized`
- Frontend: `api.ts:getProjects()`
- Response: `PaginatedResponse<Project>`

#### GET /api/v1/projects/:id
- Auth: required
- Backend: `backend/internal/handlers/projects_optimized.go:GetProjectOptimized`
- Frontend: `api.ts:getProject()`
- Response: `Project`

#### PUT /api/v1/projects/:id
- Auth: required
- Backend: `backend/internal/handlers/projects_optimized.go:UpdateProjectOptimized`
- Frontend: `api.ts:updateProject()`
- Request: partial `Project`
- Response: `Project`

#### DELETE /api/v1/projects/:id
- Auth: required
- Backend: `backend/internal/handlers/projects_optimized.go:DeleteProjectOptimized`
- Frontend: `api.ts:deleteProject()`
- Response: `{message: string}`

#### GET /api/v1/projects/:id/download
- Auth: required
- Backend: `backend/internal/api/handlers.go:DownloadProject`
- Response: ZIP file stream

---

### File Endpoints

#### POST /api/v1/projects/:id/files
- Auth: required
- Backend: `backend/internal/api/handlers.go:CreateFile`
- Frontend: `api.ts:createFile()`
- Request: `{path, name, type, content, mime_type?}`
- Response: `File`
- Status: 201

#### GET /api/v1/projects/:id/files
- Auth: required
- Backend: `backend/internal/handlers/projects_optimized.go:GetProjectFilesOptimized`
- Frontend: `api.ts:getProjectFiles()`
- Response: `File[]`

#### GET /api/v1/files/:id
- Auth: required
- Backend: `backend/internal/api/handlers.go:GetFile`
- Frontend: `api.ts:getFile()`
- Response: `File`

#### PUT /api/v1/files/:id
- Auth: required
- Backend: `backend/internal/api/handlers.go:UpdateFile`
- Frontend: `api.ts:updateFile()`
- Request: `{content?, name?, path?}`
- Response: `File`

#### DELETE /api/v1/files/:id
- Auth: required
- Backend: `backend/internal/api/handlers.go:DeleteFile`
- Frontend: `api.ts:deleteFile()`
- Response: `{message: string}`

---

### Asset Endpoints

#### POST /api/v1/projects/:id/assets
- Auth: required
- Backend: `backend/internal/api/handlers.go:UploadAsset`
- Frontend: `api.ts:uploadAsset()`
- Request: multipart/form-data
- Response: `{asset_id, url, key}`

#### GET /api/v1/projects/:id/assets
- Auth: required
- Backend: `backend/internal/api/handlers.go:ListAssets`
- Frontend: `api.ts:listAssets()`
- Response: `Asset[]`

#### DELETE /api/v1/projects/:id/assets/:assetId
- Auth: required
- Backend: `backend/internal/api/handlers.go:DeleteAsset`
- Frontend: `api.ts:deleteAsset()`

#### GET /api/v1/assets/raw/*key
- Auth: none (public asset serving)
- Backend: `backend/internal/api/handlers.go:ServeAsset`
- Response: binary file

---

### AI Endpoints

#### POST /api/v1/ai/generate
- Auth: required + quota
- Backend: `backend/internal/api/handlers.go:AIGenerate`
- Frontend: `api.ts:generateAI()`
- Request: `AIRequest` (see types)
- Response: `{content, tokens_used, cost, provider, model}`

#### GET /api/v1/ai/usage
- Auth: required
- Backend: `backend/internal/api/handlers.go:GetAIUsage`
- Frontend: `api.ts:getAIUsage()`
- Response: `AIUsage`

---

### Preview Endpoints

#### POST /api/v1/preview/start
- Auth: required
- Backend: `backend/internal/handlers/preview.go:StartPreview`
- Frontend: `api.ts:startPreview()`

#### POST /api/v1/preview/fullstack/start
- Auth: required
- Backend: `backend/internal/handlers/preview.go:StartFullStackPreview`
- Frontend: `api.ts:startFullStackPreview()`

#### POST /api/v1/preview/stop
- Auth: required
- Backend: `backend/internal/handlers/preview.go:StopPreview`
- Frontend: `api.ts:stopPreview()`

#### GET /api/v1/preview/status/:projectId
- Auth: required
- Backend: `backend/internal/handlers/preview.go:GetPreviewStatus`
- Frontend: `api.ts:getPreviewStatus()`

#### POST /api/v1/preview/refresh
- Auth: required
- Backend: `backend/internal/handlers/preview.go:RefreshPreview`
- Frontend: `api.ts:refreshPreview()`

#### POST /api/v1/preview/hot-reload
- Auth: required
- Backend: `backend/internal/handlers/preview.go:HotReload`

#### GET /api/v1/preview/list
- Auth: required
- Backend: `backend/internal/handlers/preview.go:ListPreviews`

#### GET /api/v1/preview/url/:projectId
- Auth: required
- Backend: `backend/internal/handlers/preview.go:GetPreviewURL`
- Frontend: `api.ts:getPreviewURL()`

#### POST /api/v1/preview/build
- Auth: required
- Backend: `backend/internal/handlers/preview.go:BuildProject`

#### GET /api/v1/preview/bundler/status
- Auth: required
- Backend: `backend/internal/handlers/preview.go:GetBundlerStatus`

#### POST /api/v1/preview/bundler/invalidate
- Auth: required
- Backend: `backend/internal/handlers/preview.go:InvalidateBundleCache`

#### POST /api/v1/preview/server/start
- Auth: required
- Backend: `backend/internal/handlers/preview.go:StartServer`

#### POST /api/v1/preview/server/stop
- Auth: required
- Backend: `backend/internal/handlers/preview.go:StopServer`

#### GET /api/v1/preview/server/status/:projectId
- Auth: required
- Backend: `backend/internal/handlers/preview.go:GetServerStatus`

#### GET /api/v1/preview/server/logs/:projectId
- Auth: required
- Backend: `backend/internal/handlers/preview.go:GetServerLogs`

#### GET /api/v1/preview/server/detect/:projectId
- Auth: required
- Backend: `backend/internal/handlers/preview.go:DetectServer`

#### GET /api/v1/preview/docker/status
- Auth: required
- Backend: `backend/internal/handlers/preview.go:GetDockerStatus`

---

### Git Endpoints

#### POST /api/v1/git/connect
- Auth: required
- Backend: `backend/internal/handlers/git.go:ConnectRepository`

#### GET /api/v1/git/repo/:projectId
- Auth: required
- Backend: `backend/internal/handlers/git.go:GetRepository`

#### DELETE /api/v1/git/repo/:projectId
- Auth: required
- Backend: `backend/internal/handlers/git.go:DisconnectRepository`

#### GET /api/v1/git/branches/:projectId
- Auth: required
- Backend: `backend/internal/handlers/git.go:GetBranches`

#### GET /api/v1/git/commits/:projectId
- Auth: required
- Backend: `backend/internal/handlers/git.go:GetCommits`

#### GET /api/v1/git/status/:projectId
- Auth: required
- Backend: `backend/internal/handlers/git.go:GetStatus`

#### POST /api/v1/git/commit
- Auth: required
- Backend: `backend/internal/handlers/git.go:Commit`

#### POST /api/v1/git/push
- Auth: required
- Backend: `backend/internal/handlers/git.go:Push`

#### POST /api/v1/git/pull
- Auth: required
- Backend: `backend/internal/handlers/git.go:Pull`

#### POST /api/v1/git/branch
- Auth: required
- Backend: `backend/internal/handlers/git.go:CreateBranch`

#### POST /api/v1/git/checkout
- Auth: required
- Backend: `backend/internal/handlers/git.go:SwitchBranch`

#### GET /api/v1/git/pulls/:projectId
- Auth: required
- Backend: `backend/internal/handlers/git.go:GetPullRequests`

#### POST /api/v1/git/pulls
- Auth: required
- Backend: `backend/internal/handlers/git.go:CreatePullRequest`

#### POST /api/v1/git/export
- Auth: required
- Backend: `backend/internal/handlers/export.go:ExportToGitHub`

#### GET /api/v1/git/export/status/:projectId
- Auth: required
- Backend: `backend/internal/handlers/export.go:GetExportStatus`

---

### Billing Endpoints

#### POST /api/v1/billing/checkout
- Auth: required
- Backend: `backend/internal/handlers/payments.go:CreateCheckoutSession`

#### GET /api/v1/billing/subscription
- Auth: required
- Backend: `backend/internal/handlers/payments.go:GetSubscription`

#### POST /api/v1/billing/portal
- Auth: required
- Backend: `backend/internal/handlers/payments.go:CreateBillingPortalSession`

#### GET /api/v1/billing/plans
- Auth: required
- Backend: `backend/internal/handlers/payments.go:GetPlans`

#### GET /api/v1/billing/usage
- Auth: required
- Backend: `backend/internal/handlers/payments.go:GetUsage`

#### POST /api/v1/billing/change-plan
- Auth: required
- Backend: `backend/internal/handlers/payments.go:ChangePlan`

#### POST /api/v1/billing/cancel
- Auth: required
- Backend: `backend/internal/handlers/payments.go:CancelSubscription`

#### POST /api/v1/billing/reactivate
- Auth: required
- Backend: `backend/internal/handlers/payments.go:ReactivateSubscription`

#### GET /api/v1/billing/invoices
- Auth: required
- Backend: `backend/internal/handlers/payments.go:GetInvoices`

#### GET /api/v1/billing/payment-methods
- Auth: required
- Backend: `backend/internal/handlers/payments.go:GetPaymentMethods`

#### GET /api/v1/billing/check-limit/:type
- Auth: required
- Backend: `backend/internal/handlers/payments.go:CheckUsageLimit`

#### GET /api/v1/billing/config-status
- Auth: required
- Backend: `backend/internal/handlers/payments.go:StripeConfigStatus`

#### POST /api/v1/billing/credits/purchase
- Auth: required
- Backend: `backend/internal/handlers/payments.go:PurchaseCredits`

#### GET /api/v1/billing/credits/balance
- Auth: required
- Backend: `backend/internal/handlers/payments.go:GetCreditBalance`

#### GET /api/v1/billing/credits/ledger
- Auth: required
- Backend: `backend/internal/handlers/payments.go:GetCreditLedger`

#### POST /api/v1/billing/webhook
- Auth: none (Stripe webhook)
- Backend: `backend/internal/handlers/payments.go:HandleWebhook`

---

### BYOK Endpoints

#### POST /api/v1/byok/keys
- Auth: required
- Backend: `backend/internal/handlers/byok.go:SaveKey`
- Frontend: `api.ts:saveBYOKKey()`

#### GET /api/v1/byok/keys
- Auth: required
- Backend: `backend/internal/handlers/byok.go:GetKeys`
- Frontend: `api.ts:getBYOKKeys()`

#### DELETE /api/v1/byok/keys/:provider
- Auth: required
- Backend: `backend/internal/handlers/byok.go:DeleteKey`
- Frontend: `api.ts:deleteBYOKKey()`

#### PATCH /api/v1/byok/keys/:provider
- Auth: required
- Backend: `backend/internal/handlers/byok.go:UpdateKeySettings`
- Frontend: `api.ts:updateBYOKKeySettings()`

#### POST /api/v1/byok/keys/:provider/validate
- Auth: required
- Backend: `backend/internal/handlers/byok.go:ValidateKey`
- Frontend: `api.ts:validateBYOKKey()`

#### GET /api/v1/byok/usage
- Auth: required
- Backend: `backend/internal/handlers/byok.go:GetUsage`
- Frontend: `api.ts:getBYOKUsage()`

#### GET /api/v1/byok/models
- Auth: required
- Backend: `backend/internal/handlers/byok.go:GetModels`
- Frontend: `api.ts:getBYOKModels()`

---

### Budget Endpoints

#### GET /api/v1/budget/caps
- Auth: required
- Backend: `backend/internal/handlers/budget.go:GetCaps`
- Frontend: `api.ts:getBudgetCaps()`

#### POST /api/v1/budget/caps
- Auth: required
- Backend: `backend/internal/handlers/budget.go:SetCap`
- Frontend: `api.ts:setBudgetCap()`

#### DELETE /api/v1/budget/caps/:id
- Auth: required
- Backend: `backend/internal/handlers/budget.go:DeleteCap`
- Frontend: `api.ts:deleteBudgetCap()`

#### POST /api/v1/budget/pre-authorize
- Auth: required
- Backend: `backend/internal/handlers/budget.go:PreAuthorize`

#### POST /api/v1/budget/kill-all
- Auth: required
- Backend: `backend/internal/handlers/budget.go:KillAll`
- Frontend: `api.ts:killAllSpending()`

---

### Spend Endpoints

#### GET /api/v1/spend/dashboard
- Auth: required
- Backend: `backend/internal/handlers/spend.go`
- Frontend: `api.ts:getSpendDashboard()`

#### GET /api/v1/spend/breakdown
- Auth: required
- Backend: `backend/internal/handlers/spend.go`
- Frontend: `api.ts:getSpendBreakdown()`

#### GET /api/v1/spend/history
- Auth: required
- Backend: `backend/internal/handlers/spend.go`
- Frontend: `api.ts:getSpendHistory()`

#### GET /api/v1/spend/forecast
- Auth: required
- Backend: `backend/internal/handlers/spend.go`
- Frontend: `api.ts:getSpendForecast()`

---

### Collaboration Endpoints

#### POST /api/v1/collab/join/:projectId
- Auth: required
- Backend: `backend/internal/handlers/collaboration.go:JoinRoom`
- Frontend: `collaboration.ts:joinRoom()`

#### POST /api/v1/collab/leave/:roomId
- Auth: required
- Backend: `backend/internal/handlers/collaboration.go:LeaveRoom`

#### GET /api/v1/collab/users/:roomId
- Auth: required
- Backend: `backend/internal/handlers/collaboration.go:GetUsers`

---

### Search Endpoints

#### POST /api/v1/search
- Auth: required
- Backend: `backend/internal/handlers/search.go:Search`

#### GET /api/v1/search/quick
- Auth: required
- Backend: `backend/internal/handlers/search.go:QuickSearch`

#### GET /api/v1/search/symbols
- Auth: required
- Backend: `backend/internal/handlers/search.go:SearchSymbols`

#### GET /api/v1/search/files
- Auth: required
- Backend: `backend/internal/handlers/search.go:SearchFiles`

#### POST /api/v1/search/replace
- Auth: required
- Backend: `backend/internal/handlers/search.go:SearchAndReplace`

#### GET /api/v1/search/history
- Auth: required
- Backend: `backend/internal/handlers/search.go:GetSearchHistory`

#### DELETE /api/v1/search/history
- Auth: required
- Backend: `backend/internal/handlers/search.go:ClearSearchHistory`

---

### Template Endpoints

#### GET /api/v1/templates
- Auth: required
- Backend: `backend/internal/handlers/templates.go:ListTemplates`
- Frontend: `api.ts:getTemplates()`

#### GET /api/v1/templates/categories
- Auth: required
- Backend: `backend/internal/handlers/templates.go:GetCategories`
- Frontend: `api.ts:getTemplateCategories()`

#### GET /api/v1/templates/:id
- Auth: required
- Backend: `backend/internal/handlers/templates.go:GetTemplate`
- Frontend: `api.ts:getTemplate()`

#### POST /api/v1/templates/create-project
- Auth: required
- Backend: `backend/internal/handlers/templates.go:CreateProjectFromTemplate`
- Frontend: `api.ts:createProjectFromTemplate()`

---

### Admin Endpoints

#### GET /api/v1/admin/dashboard
- Auth: required + admin
- Backend: `backend/internal/api/handlers.go:AdminDashboard`

#### GET /api/v1/admin/users
- Auth: required + admin
- Backend: `backend/internal/api/handlers.go:AdminGetUsers`

#### GET /api/v1/admin/users/:id
- Auth: required + admin
- Backend: `backend/internal/api/handlers.go:AdminGetUser`

#### PUT /api/v1/admin/users/:id
- Auth: required + admin
- Backend: `backend/internal/api/handlers.go:AdminUpdateUser`

#### DELETE /api/v1/admin/users/:id
- Auth: required + admin
- Backend: `backend/internal/api/handlers.go:AdminDeleteUser`

#### POST /api/v1/admin/users/:id/credits
- Auth: required + admin
- Backend: `backend/internal/api/handlers.go:AdminAddCredits`

#### GET /api/v1/admin/stats
- Auth: required + admin
- Backend: `backend/internal/api/handlers.go:AdminGetSystemStats`

#### POST /api/v1/admin/rotate-secrets
- Auth: required + admin
- Backend: `backend/internal/handlers/rotation_handler.go:RotateSecrets`

#### GET /api/v1/admin/validate-secrets
- Auth: required + admin
- Backend: `backend/internal/handlers/rotation_handler.go:ValidateSecrets`

---

## WebSocket Events

### Connection URLs
- Build updates: `/ws/build/:buildId`
- Terminal: `/ws/terminal/:sessionId`
- Collaboration: `/ws/collab`
- Debugging: `/ws/debug/:sessionId`
- Deployment: `/ws/deploy/:deploymentId`
- MCP: `/mcp/ws`
- Autonomous agents: (registered via `autonomousHandler.RegisterWebSocketRoute`)

### Build WebSocket Events

#### Backend → Frontend (Broadcast)
```
"build:progress"     → {build_id, progress, status, phase, message}
"build:complete"     → {build_id, status, result, url, duration}
"build:error"        → {build_id, error, phase, recoverable}
"build:agent:message" → {build_id, agent_id, role, content, timestamp}
"build:checkpoint"   → {build_id, checkpoint_id, number, name, progress}
"build:paused"       → {build_id, reason, requires_input}
"build:permission"   → {build_id, request_id, scope, target, reason}
"build:approval"     → {build_id, request_id, decision, note}
"build:activity"     → {build_id, agent_id, role, type, content, timestamp}
"build:fsm:state"    → {build_id, state, previous_state, timestamp}
```

#### Frontend → Backend (Emit)
```
"build:pause"        → {build_id, reason}
"build:resume"       → {build_id}
"build:cancel"       → {build_id}
"build:approve"      → {build_id, request_id, decision, note}
"build:steer"        → {build_id, message, target_mode, target_agent_id}
"build:permission:rule" → {build_id, scope, target, decision, mode}
"build:kill"         → {build_id}
```

### Collaboration WebSocket Events

#### Backend Constants (from `websocket/hub.go`)
```go
MessageTypeJoinRoom     = "join_room"
MessageTypeLeaveRoom    = "leave_room"
MessageTypeCursorUpdate = "cursor_update"
MessageTypeFileChange   = "file_change"
MessageTypeChat         = "chat"
MessageTypeUserJoined   = "user_joined"
MessageTypeUserLeft     = "user_left"
MessageTypeUserList     = "user_list"
MessageTypeError        = "error"
MessageTypeHeartbeat    = "heartbeat"
```

#### Frontend Types (from `websocket.ts`)
```typescript
type CollaborationEvent =
  | 'user-joined'
  | 'user-left'
  | 'file-changed'
  | 'cursor-moved'
  | 'chat-message'
  | 'file-locked'
  | 'file-unlocked'
  | 'project-updated'
  | 'ai-request'
  | 'ai-response'
```

**⚠️ KNOWN MISMATCH:** Backend uses snake_case event names (`user_joined`), frontend uses kebab-case (`user-joined`). This is a CRITICAL contract mismatch.

### Terminal WebSocket
- Binary frames for PTY data
- Text frames for JSON control messages (resize, heartbeat)

### Debug WebSocket
- Events: `breakpoint:hit`, `step:complete`, `variable:update`, `callstack:update`

---

## Type Mappings

### User
```typescript
// Frontend (types/index.ts)
interface User {
  id: number
  username: string
  email: string
  full_name?: string        // camelCase
  avatar_url?: string         // camelCase
  is_active: boolean
  is_verified: boolean
  is_admin?: boolean
  is_super_admin?: boolean    // camelCase
  has_unlimited_credits?: boolean  // camelCase
  bypass_billing?: boolean
  credit_balance?: number
  subscription_type: 'free' | 'builder' | 'pro' | 'team' | 'enterprise' | 'owner'
  subscription_end?: string   // camelCase
  monthly_ai_requests: number   // camelCase
  monthly_ai_cost: number       // camelCase
  preferred_theme: 'cyberpunk' | ...
  preferred_ai: 'auto' | ...
  created_at: string
  updated_at: string
}

// Backend (models/user.go) — CHECK: uses snake_case JSON tags
```

### Project
```typescript
// Frontend
interface Project {
  id: number
  name: string
  description?: string
  language: string
  framework?: string
  owner_id: number            // camelCase
  owner?: User
  is_public: boolean
  is_archived: boolean
  root_directory: string       // camelCase
  entry_point?: string         // camelCase
  environment?: Record<string, any>
  dependencies?: Record<string, any>
  build_config?: Record<string, any>  // camelCase
  environment_config?: string
  provisioned_database_id?: number   // camelCase
  collab_room_id?: number      // camelCase
  created_at: string
  updated_at: string
}
```

### File
```typescript
interface File {
  id: number
  project_id: number          // camelCase
  project?: Project
  path: string
  name: string
  type: 'file' | 'directory'
  mime_type?: string           // camelCase
  content: string
  size: number
  hash?: string
  version: number
  last_edit_by?: number        // camelCase
  last_editor?: User
  is_locked: boolean
  locked_by?: number
  locked_at?: string
  created_at: string
  updated_at: string
}
```

### AuthResponse
```typescript
interface AuthResponse {
  access_token: string          // snake_case
  refresh_token: string         // snake_case
  access_token_expires_at: string  // snake_case
  refresh_token_expires_at: string // snake_case
  user: User
}
```

---

## Common Response Shapes

### Success Wrapper
```typescript
interface ApiResponse<T> {
  success: boolean
  data: T
  message?: string
}
```

### Paginated Response
```typescript
interface PaginatedResponse<T> {
  items: T[]
  total: number
  page: number
  per_page: number      // snake_case
  has_more: boolean     // snake_case
}
```

### Error Response
```typescript
interface ApiError {
  error: string
  code?: string
  details?: Record<string, any>
}
```

---

## Error Codes

| HTTP Status | Code | Meaning |
|-------------|------|---------|
| 400 | BAD_REQUEST | Invalid request parameters |
| 401 | UNAUTHORIZED | Missing or invalid token |
| 403 | FORBIDDEN | Valid auth but insufficient permissions |
| 404 | NOT_FOUND | Resource not found |
| 409 | CONFLICT | Resource already exists |
| 429 | RATE_LIMITED | Too many requests |
| 500 | INTERNAL_ERROR | Server error |
| 503 | SERVICE_UNAVAILABLE | Feature temporarily unavailable |

---

## Auth Header Rules

### Required Headers (Protected Endpoints)
```
Authorization: Bearer <access_token>
X-CSRF-Token: <csrf_token>  (for POST/PUT/DELETE/PATCH)
```

### Cookie Behavior
- Backend sets `access_token` and `refresh_token` as HTTP-only cookies on login
- Frontend reads tokens from cookies via `document.cookie` in `authSession.ts`
- On 401, frontend calls `POST /auth/refresh` automatically
- On refresh failure, frontend clears auth and reloads page

### CSRF Flow
1. Frontend calls `GET /api/v1/csrf-token` on app load
2. Stores token in memory (`ApiService.csrfToken`)
3. Attaches `X-CSRF-Token` header to all mutating requests
4. Token is time-limited HMAC

---

## Known Issues (To Be Fixed)

1. **WebSocket Event Names:** Backend uses `user_joined`, frontend uses `user-joined`. MISMATCH.
2. **Build WebSocket:** Need to verify all build event types match between backend FSM and frontend store.
3. **Field Name Casing:** Backend likely returns `snake_case`, frontend types use `camelCase` in some interfaces but `snake_case` in others. INCONSISTENT.
4. **BYOK endpoints:** Need verification of request/response shapes.
5. **Budget endpoints:** Need verification of cap shape fields.
6. **Preview endpoints:** Some frontend methods may call wrong paths.

---

## Verification Script

Run after every deploy:
```bash
./scripts/verify-contract.sh
```

See `scripts/verify-contract.sh` for implementation.

---

## Changelog

| Date | Version | Changes |
|------|---------|---------|
| 2026-04-25 | v1.0 | Initial contract documentation |
