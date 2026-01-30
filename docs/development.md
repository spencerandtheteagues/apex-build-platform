# APEX.BUILD Development Guide

Complete guide for setting up a local development environment and contributing to APEX.BUILD.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Local Setup](#local-setup)
- [Project Structure](#project-structure)
- [Environment Variables](#environment-variables)
- [Database Setup](#database-setup)
- [Running the Application](#running-the-application)
- [Running Tests](#running-tests)
- [Code Style Guidelines](#code-style-guidelines)
- [Development Workflow](#development-workflow)
- [Debugging](#debugging)
- [Common Issues](#common-issues)

---

## Prerequisites

### Required Software

| Software | Version | Purpose |
|----------|---------|---------|
| Go | 1.21+ | Backend development |
| Node.js | 18+ | Frontend development |
| PostgreSQL | 15+ | Database |
| Git | 2.30+ | Version control |
| Docker | 24+ | Containerization (optional) |

### Recommended Tools

- **VS Code** with Go and TypeScript extensions
- **Postman** or **Insomnia** for API testing
- **pgAdmin** or **DBeaver** for database management
- **Docker Desktop** for container management

### Verify Installation

```bash
# Check Go version
go version
# Expected: go version go1.21.x or higher

# Check Node.js version
node --version
# Expected: v18.x.x or higher

# Check PostgreSQL
psql --version
# Expected: psql (PostgreSQL) 15.x or higher

# Check Docker (optional)
docker --version
# Expected: Docker version 24.x.x or higher
```

---

## Local Setup

### 1. Clone the Repository

```bash
git clone https://github.com/spencerandtheteagues/apex-build-platform.git
cd apex-build-platform
```

### 2. Install Backend Dependencies

```bash
cd backend
go mod download
go mod verify
```

### 3. Install Frontend Dependencies

```bash
cd frontend
npm install
```

### 4. Set Up Environment Variables

```bash
# Copy the example environment file
cp .env.example .env

# Edit with your configuration
nano .env
```

### 5. Initialize the Database

```bash
# Create the database
createdb apex_build

# The application will auto-migrate on startup
```

### 6. Start Development Servers

```bash
# Terminal 1: Backend
cd backend
go run cmd/main.go

# Terminal 2: Frontend
cd frontend
npm run dev
```

### 7. Access the Application

- **Frontend**: http://localhost:5173
- **Backend API**: http://localhost:8080
- **Health Check**: http://localhost:8080/health
- **API Docs**: http://localhost:8080/docs

---

## Project Structure

```
apex-build/
+-- backend/                    # Go backend application
|   +-- cmd/
|   |   +-- main.go            # Application entry point
|   +-- internal/
|   |   +-- agents/            # Agent orchestration system
|   |   |   +-- handlers.go    # Build HTTP handlers
|   |   |   +-- manager.go     # Agent lifecycle management
|   |   |   +-- orchestrator.go# Build orchestration
|   |   |   +-- types.go       # Agent type definitions
|   |   |   +-- websocket.go   # Agent WebSocket hub
|   |   +-- ai/                # AI provider integrations
|   |   |   +-- claude.go      # Anthropic Claude client
|   |   |   +-- gemini.go      # Google Gemini client
|   |   |   +-- openai.go      # OpenAI GPT-4 client
|   |   |   +-- router.go      # Intelligent AI routing
|   |   |   +-- types.go       # AI type definitions
|   |   +-- api/               # API handlers
|   |   +-- auth/              # Authentication
|   |   |   +-- jwt.go         # JWT token handling
|   |   |   +-- password.go    # Password hashing
|   |   |   +-- oauth.go       # OAuth providers
|   |   +-- db/                # Database layer
|   |   |   +-- database.go    # Connection management
|   |   |   +-- seed.go        # Database seeding
|   |   +-- deploy/            # Deployment providers
|   |   |   +-- providers/     # Vercel, Netlify, Render
|   |   +-- execution/         # Code execution engine
|   |   +-- git/               # Git integration
|   |   +-- handlers/          # HTTP handlers
|   |   +-- mcp/               # MCP server integration
|   |   +-- middleware/        # HTTP middleware
|   |   +-- packages/          # Package managers
|   |   +-- payments/          # Stripe integration
|   |   +-- preview/           # Live preview server
|   |   +-- search/            # Code search engine
|   |   +-- secrets/           # Encrypted secrets
|   |   +-- security/          # Security features
|   |   +-- templates/         # Project templates
|   |   +-- websocket/         # WebSocket hub
|   +-- pkg/
|   |   +-- models/            # Database models
|   +-- go.mod
|   +-- go.sum
+-- frontend/                   # React frontend application
|   +-- src/
|   |   +-- components/        # React components
|   |   |   +-- admin/         # Admin dashboard
|   |   |   +-- ai/            # AI assistant
|   |   |   +-- builder/       # App builder
|   |   |   +-- editor/        # Monaco editor
|   |   |   +-- explorer/      # File explorer
|   |   |   +-- git/           # Git panel
|   |   |   +-- ide/           # IDE layout
|   |   |   +-- mcp/           # MCP manager
|   |   |   +-- preview/       # Live preview
|   |   |   +-- project/       # Project management
|   |   |   +-- search/        # Code search
|   |   |   +-- secrets/       # Secrets manager
|   |   |   +-- ui/            # Shared UI components
|   |   +-- hooks/             # Custom React hooks
|   |   +-- services/          # API services
|   |   |   +-- api.ts         # API client
|   |   |   +-- websocket.ts   # WebSocket client
|   |   +-- styles/            # CSS and themes
|   |   +-- types/             # TypeScript types
|   |   +-- App.tsx            # Root component
|   |   +-- main.tsx           # Entry point
|   +-- package.json
|   +-- tsconfig.json
|   +-- vite.config.ts
+-- docs/                       # Documentation
+-- docker-compose.yml          # Docker Compose config
+-- render.yaml                 # Render deployment config
+-- .env.example                # Environment template
```

---

## Environment Variables

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://user:pass@localhost:5432/apex_build` |
| `JWT_SECRET` | Secret for signing JWTs | Random 32+ character string |
| `PORT` | Server port | `8080` |
| `ENVIRONMENT` | Environment mode | `development` or `production` |

### AI Provider Keys

| Variable | Description | Required |
|----------|-------------|----------|
| `ANTHROPIC_API_KEY` | Claude API key | At least one AI key |
| `OPENAI_API_KEY` | OpenAI API key | At least one AI key |
| `GEMINI_API_KEY` | Gemini API key | At least one AI key |

### Optional Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SECRETS_MASTER_KEY` | AES encryption key | Auto-generated |
| `STRIPE_SECRET_KEY` | Stripe API key | Disabled |
| `STRIPE_WEBHOOK_SECRET` | Stripe webhook secret | Disabled |
| `VERCEL_TOKEN` | Vercel deployment token | Disabled |
| `NETLIFY_TOKEN` | Netlify deployment token | Disabled |
| `RENDER_TOKEN` | Render deployment token | Disabled |
| `PROJECTS_DIR` | Code execution directory | `/tmp/apex-build-projects` |
| `REDIS_URL` | Redis connection URL | Disabled |

### Environment File Example

```bash
# .env file
# Database
DATABASE_URL=postgres://postgres:password@localhost:5432/apex_build

# Or individual database settings
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=apex_build
DB_SSL_MODE=disable
DB_TIMEZONE=UTC

# Authentication
JWT_SECRET=your-super-secret-jwt-key-minimum-32-chars

# Server
PORT=8080
ENVIRONMENT=development

# AI Providers (at least one required)
ANTHROPIC_API_KEY=sk-ant-api...
OPENAI_API_KEY=sk-...
GEMINI_API_KEY=AIza...

# Secrets encryption
SECRETS_MASTER_KEY=32-byte-encryption-key-here123

# Optional: Stripe
STRIPE_SECRET_KEY=sk_test_...
STRIPE_WEBHOOK_SECRET=whsec_...

# Optional: Deployment
VERCEL_TOKEN=...
NETLIFY_TOKEN=...
RENDER_TOKEN=...
```

---

## Database Setup

### Using Docker (Recommended)

```bash
# Start PostgreSQL container
docker run -d \
  --name apex-postgres \
  -e POSTGRES_DB=apex_build \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=password \
  -p 5432:5432 \
  postgres:15-alpine

# Verify connection
psql -h localhost -U postgres -d apex_build
```

### Using Local PostgreSQL

```bash
# Create database
createdb apex_build

# Or using psql
psql -U postgres -c "CREATE DATABASE apex_build;"
```

### Database Migrations

The application uses GORM's AutoMigrate feature. Migrations run automatically on startup.

### Seeding Data

The application seeds default admin users on startup. Check `backend/internal/db/seed.go` for details.

### Database Models

Key models defined in `backend/pkg/models/models.go`:

- **User** - User accounts with subscriptions
- **Project** - Coding projects
- **File** - Project files
- **Session** - User sessions
- **AIRequest** - AI request history
- **Execution** - Code execution records
- **CollabRoom** - Collaboration rooms

---

## Running the Application

### Development Mode

```bash
# Backend with hot reload (using air)
cd backend
air

# Or standard go run
go run cmd/main.go

# Frontend with hot reload
cd frontend
npm run dev
```

### Using Docker Compose

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down

# Rebuild after changes
docker-compose up -d --build
```

### Production Build

```bash
# Backend
cd backend
go build -o apex-server ./cmd/main.go

# Frontend
cd frontend
npm run build
```

---

## Running Tests

### Backend Tests

```bash
cd backend

# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/ai/...

# Verbose output
go test -v ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Frontend Tests

```bash
cd frontend

# Run tests
npm test

# Run with coverage
npm run test:coverage

# Run in watch mode
npm run test:watch

# Run specific test file
npm test -- src/components/Button.test.tsx
```

### Integration Tests

```bash
# Run integration tests (requires running services)
cd tests
go test -v ./...
```

### End-to-End Tests

```bash
cd frontend

# Run Playwright tests
npm run test:e2e

# Run with UI
npm run test:e2e:ui
```

---

## Code Style Guidelines

### Go Code Style

**Formatting**
```bash
# Format code
gofmt -w .

# Or use goimports for import organization
goimports -w .
```

**Naming Conventions**
- Use `camelCase` for unexported names
- Use `PascalCase` for exported names
- Keep names short but descriptive
- Use acronyms consistently (e.g., `ID`, `URL`, `API`)

**Function Guidelines**
- Keep functions under 50 lines
- Single responsibility principle
- Return errors as the last return value
- Document exported functions

**Example**
```go
// CreateProject creates a new project for the given user.
// It validates the input and returns an error if validation fails.
func (h *Handler) CreateProject(c *gin.Context) {
    userID, exists := middleware.GetUserID(c)
    if !exists {
        c.JSON(http.StatusUnauthorized, StandardResponse{
            Success: false,
            Error:   "User not authenticated",
            Code:    "NOT_AUTHENTICATED",
        })
        return
    }

    var req CreateProjectRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, StandardResponse{
            Success: false,
            Error:   "Invalid request format",
            Code:    "INVALID_REQUEST",
        })
        return
    }

    // Create project logic...
}
```

### TypeScript Code Style

**Formatting**
```bash
# Format code
npm run format

# Lint code
npm run lint

# Fix linting issues
npm run lint:fix
```

**Naming Conventions**
- Use `camelCase` for variables and functions
- Use `PascalCase` for components and types
- Use `UPPER_SNAKE_CASE` for constants
- Prefix interfaces with descriptive names (not `I`)

**Component Guidelines**
- Use functional components with hooks
- Keep components focused and small
- Extract reusable logic into custom hooks
- Use TypeScript for all new code

**Example**
```typescript
import React, { useState, useCallback } from 'react';

interface ProfileCardProps {
  user: User;
  onUpdate: (user: User) => void;
}

export const ProfileCard: React.FC<ProfileCardProps> = ({ user, onUpdate }) => {
  const [isEditing, setIsEditing] = useState(false);

  const handleSave = useCallback(async (data: Partial<User>) => {
    try {
      const updated = await api.updateUser(user.id, data);
      onUpdate(updated);
      setIsEditing(false);
    } catch (error) {
      console.error('Failed to update user:', error);
    }
  }, [user.id, onUpdate]);

  return (
    <div className="bg-white rounded-lg shadow-md p-6">
      {/* Component content */}
    </div>
  );
};
```

### Commit Message Guidelines

Use conventional commit format:

```
type(scope): description

[optional body]

[optional footer]
```

**Types**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples**
```
feat(ai): add intelligent AI provider routing

Implements capability-based routing that selects the optimal
AI provider for each request type.

- Claude for code review and documentation
- GPT-4 for code generation
- Gemini for code completion

fix(auth): resolve token refresh race condition

test(api): add integration tests for project endpoints
```

---

## Development Workflow

### Feature Development

1. **Create a branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make changes**
   - Write code following style guidelines
   - Add tests for new functionality
   - Update documentation if needed

3. **Test locally**
   ```bash
   # Backend tests
   cd backend && go test ./...

   # Frontend tests
   cd frontend && npm test
   ```

4. **Commit changes**
   ```bash
   git add .
   git commit -m "feat(scope): description"
   ```

5. **Push and create PR**
   ```bash
   git push -u origin feature/your-feature-name
   ```

### Code Review Process

1. Create a pull request with a clear description
2. Ensure all CI checks pass
3. Request review from a team member
4. Address feedback and update PR
5. Merge after approval

---

## Debugging

### Backend Debugging

**Using VS Code**
```json
// .vscode/launch.json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Backend",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/backend/cmd/main.go",
      "env": {
        "ENVIRONMENT": "development"
      }
    }
  ]
}
```

**Using Delve**
```bash
cd backend
dlv debug cmd/main.go
```

**Logging**
```go
log.Printf("Debug: user=%+v", user)
```

### Frontend Debugging

**Browser DevTools**
- Use React Developer Tools extension
- Check Network tab for API calls
- Use Console for debugging output

**VS Code Debugging**
```json
// .vscode/launch.json
{
  "version": "0.2.0",
  "configurations": [
    {
      "type": "chrome",
      "request": "launch",
      "name": "Debug Frontend",
      "url": "http://localhost:5173",
      "webRoot": "${workspaceFolder}/frontend/src"
    }
  ]
}
```

### Database Debugging

**Query Logging**
Enable GORM query logging:
```go
db.Debug().Where("id = ?", id).First(&user)
```

**Direct Database Access**
```bash
psql -h localhost -U postgres -d apex_build

-- View tables
\dt

-- Describe table
\d users

-- Query data
SELECT * FROM users LIMIT 10;
```

---

## Common Issues

### Port Already in Use

```bash
# Find process using port
lsof -i :8080

# Kill the process
kill -9 <PID>
```

### Database Connection Failed

1. Verify PostgreSQL is running
2. Check DATABASE_URL format
3. Ensure database exists
4. Check firewall settings

### Go Module Issues

```bash
# Clear module cache
go clean -modcache

# Re-download dependencies
go mod download
```

### Frontend Build Errors

```bash
# Clear node_modules
rm -rf node_modules
npm install

# Clear Vite cache
rm -rf node_modules/.vite
```

### AI API Errors

1. Verify API keys are correct
2. Check rate limits
3. Ensure sufficient credits/quota
4. Review API response for details

---

**Last Updated**: January 2024
