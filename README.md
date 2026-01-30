# APEX.BUILD

**Next-Generation Cloud Development Platform with Multi-AI Integration**

APEX.BUILD is a cloud-based integrated development environment (IDE) that combines the power of three AI providers (Claude, GPT-4, and Gemini) with real-time collaboration, intelligent code generation, and one-click deployment. Build complete applications from natural language descriptions with an autonomous agent orchestration system.

---

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Technology Stack](#technology-stack)
- [Architecture Overview](#architecture-overview)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

---

## Features

### Multi-AI Integration

APEX.BUILD integrates three leading AI providers, each optimized for specific tasks:

| Provider | Primary Use Cases |
|----------|-------------------|
| **Claude (Anthropic)** | Code review, debugging, documentation, architecture design |
| **GPT-4 (OpenAI)** | Code generation, refactoring, comprehensive testing |
| **Gemini (Google)** | Code completion, explanations, interactive assistance |

The intelligent AI router automatically selects the best provider for each task, with automatic fallback and load balancing.

### Agent Orchestration System

Build complete applications with autonomous AI agents:

- **Planner Agent** - Analyzes requirements and creates build plans
- **Architect Agent** - Designs system architecture
- **Frontend Agent** - Builds UI components (React, Vue, Next.js)
- **Backend Agent** - Creates APIs and business logic (Go, Node.js, Python)
- **Database Agent** - Designs schemas and queries
- **Testing Agent** - Writes and runs tests
- **DevOps Agent** - Handles deployment configuration
- **Reviewer Agent** - Code review and quality assurance

### Cloud IDE Features

- **Monaco Editor** - Full-featured code editor with IntelliSense
- **Real-time Collaboration** - Work together with WebSocket-based sync
- **Live Preview** - Hot reload support for instant feedback
- **Integrated Terminal** - Full terminal access with persistent sessions
- **Code Execution** - Run code in 10+ languages (JavaScript, Python, Go, Rust, Java, and more)
- **Git Integration** - Clone, commit, push, and manage branches
- **Package Management** - NPM, PyPI, and Go Modules support

### Enterprise Features

- **AES-256 Encrypted Secrets** - Secure environment variable management
- **MCP Server Integration** - Connect to external Model Context Protocol servers
- **One-Click Deployment** - Deploy to Vercel, Netlify, or Render
- **Stripe Billing** - Subscription management with usage-based pricing
- **Role-Based Access Control** - Admin dashboard with user management

---

## Quick Start

### Prerequisites

- Go 1.21+
- Node.js 18+
- PostgreSQL 15+
- Docker & Docker Compose (optional)

### Local Development

```bash
# Clone the repository
git clone https://github.com/spencerandtheteagues/apex-build-platform.git
cd apex-build-platform

# Set up environment variables
cp .env.example .env
# Edit .env with your API keys and configuration

# Start the backend
cd backend
go mod download
go run cmd/main.go

# In a new terminal, start the frontend
cd frontend
npm install
npm run dev

# Visit http://localhost:5173
```

### Docker Compose

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Access the application
# Frontend: http://localhost:3000
# Backend API: http://localhost:8080
# Database Admin: http://localhost:8081
```

### Environment Variables

Create a `.env` file in the project root:

```bash
# AI API Keys
ANTHROPIC_API_KEY=your_claude_key
OPENAI_API_KEY=your_openai_key
GEMINI_API_KEY=your_gemini_key

# Database
DATABASE_URL=postgres://user:password@localhost:5432/apex_build

# Authentication
JWT_SECRET=your_secure_jwt_secret

# Server
PORT=8080
ENVIRONMENT=development

# Optional: Stripe (for billing)
STRIPE_SECRET_KEY=sk_test_xxx
STRIPE_WEBHOOK_SECRET=whsec_xxx

# Optional: Deployment providers
VERCEL_TOKEN=your_vercel_token
NETLIFY_TOKEN=your_netlify_token
RENDER_TOKEN=your_render_token

# Optional: Secrets encryption
SECRETS_MASTER_KEY=your_32_byte_key
```

---

## Technology Stack

### Backend
- **Language**: Go 1.21+
- **Framework**: Gin HTTP framework
- **Database**: PostgreSQL with GORM ORM
- **Authentication**: JWT with HS256 signing
- **WebSockets**: Gorilla WebSocket
- **AI APIs**: Anthropic Claude, OpenAI GPT-4, Google Gemini

### Frontend
- **Framework**: React 18 with TypeScript
- **Build Tool**: Vite
- **Styling**: Tailwind CSS with custom Cyberpunk theme
- **State Management**: Zustand with React Context
- **Code Editor**: Monaco Editor
- **Real-time**: WebSocket integration

### Infrastructure
- **Containerization**: Docker & Docker Compose
- **Deployment**: Render, Vercel, Netlify
- **Database**: PostgreSQL
- **Cache**: Redis (optional)

---

## Architecture Overview

```
APEX.BUILD Platform
+--------------------------------------------------+
|                     Frontend                      |
|  React + TypeScript + Monaco Editor + WebSocket   |
+--------------------------------------------------+
                        |
                        v
+--------------------------------------------------+
|                   API Gateway                     |
|         Gin HTTP Server + Auth Middleware         |
+--------------------------------------------------+
          |                           |
          v                           v
+-------------------+    +---------------------------+
|   AI Router       |    |   Agent Orchestrator     |
| Claude/GPT/Gemini |    | Planner/Architect/Dev    |
+-------------------+    +---------------------------+
          |                           |
          v                           v
+--------------------------------------------------+
|                Service Layer                      |
| Auth | Projects | Files | Execution | Deploy     |
+--------------------------------------------------+
                        |
                        v
+--------------------------------------------------+
|                    Database                       |
|                  PostgreSQL                       |
+--------------------------------------------------+
```

### Key Components

1. **AI Router** - Intelligent routing to optimal AI provider based on task type
2. **Agent Manager** - Spawns and coordinates AI agents for builds
3. **WebSocket Hub** - Real-time communication for collaboration and build updates
4. **Execution Engine** - Sandboxed code execution across multiple languages
5. **Deployment Service** - One-click deployment to cloud providers

---

## Documentation

| Document | Description |
|----------|-------------|
| [API Reference](docs/api.md) | Complete REST API documentation with examples |
| [Development Guide](docs/development.md) | Local setup, testing, and code style guidelines |
| [Deployment Guide](docs/deployment.md) | Production deployment instructions |
| [Architecture](docs/architecture.md) | System design and component interactions |

---

## Contributing

We welcome contributions to APEX.BUILD. Please follow these guidelines:

### Development Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes with clear commit messages
4. Write tests for new functionality
5. Ensure all tests pass: `go test ./...` and `npm test`
6. Submit a pull request

### Code Style

**Backend (Go)**
- Follow standard Go conventions
- Use `gofmt` for formatting
- Write clear comments for exported functions
- Keep functions focused and under 50 lines

**Frontend (TypeScript)**
- Use TypeScript for all new code
- Follow the existing component patterns
- Use functional components with hooks
- Prefer named exports

### Commit Messages

Use conventional commit format:
- `feat:` New features
- `fix:` Bug fixes
- `docs:` Documentation changes
- `refactor:` Code refactoring
- `test:` Test additions or modifications
- `chore:` Maintenance tasks

---

## License

MIT License

Copyright (c) 2024 APEX.BUILD

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

---

**Built with autonomous AI agents powered by Claude, GPT-4, and Gemini**
