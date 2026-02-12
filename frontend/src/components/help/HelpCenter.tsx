// APEX.BUILD Help Center
// Comprehensive guide to every feature — written for both developers and beginners

import React, { useState, useMemo } from 'react'
import { cn } from '@/lib/utils'
import {
  HelpCircle,
  Search,
  X,
  ChevronRight,
  ChevronDown,
  Rocket,
  Code2,
  Cpu,
  Zap,
  Globe,
  Settings,
  Key,
  Bot,
  FileCode,
  Terminal,
  Eye,
  Download,
  Upload,
  GitBranch,
  MessageSquare,
  Shield,
  CreditCard,
  Layers,
  Database,
  Play,
  Share2,
  Users,
  History,
  Sparkles,
  BookOpen,
  Layout,
  Palette,
  Bug,
  TestTube,
  RefreshCw,
  FolderOpen,
  Monitor,
  Smartphone,
  Tablet,
  ExternalLink,
  Lock,
  Star,
  ArrowRight,
} from 'lucide-react'

// ============================================================================
// HELP CONTENT DATA
// ============================================================================

interface HelpSection {
  id: string
  title: string
  icon: React.ReactNode
  description: string
  articles: HelpArticle[]
}

interface HelpArticle {
  id: string
  title: string
  content: string // Markdown-like content rendered as JSX
  tags: string[] // For search
}

const helpSections: HelpSection[] = [
  // ========================================================================
  // 1. GETTING STARTED
  // ========================================================================
  {
    id: 'getting-started',
    title: 'Getting Started',
    icon: <Rocket className="w-5 h-5" />,
    description: 'New to APEX.BUILD? Start here.',
    articles: [
      {
        id: 'what-is-apex',
        title: 'What is APEX.BUILD?',
        tags: ['intro', 'overview', 'about', 'what'],
        content: `APEX.BUILD is an AI-powered cloud development platform that lets you build complete applications from a simple text description. Multiple AI agents work together to plan, code, test, and deploy your app — all from your browser.

**What you can build:**
- Full-stack web applications (frontend + backend + database)
- REST APIs and backend services
- Single-page applications (SPAs)
- E-commerce stores, dashboards, chat apps, and more

**How it works in a nutshell:**
1. Describe what you want to build in plain English
2. Choose your tech stack (or let the AI pick for you)
3. Click "Launch Build" and watch AI agents build your app in real time
4. Open the result in the IDE to view, edit, download, or deploy

No local setup required — everything runs in the cloud.`,
      },
      {
        id: 'create-account',
        title: 'Creating your account',
        tags: ['register', 'signup', 'account', 'login'],
        content: `**To create an account:**
1. Go to the APEX.BUILD login page
2. Click "Sign up" at the bottom
3. Enter a username (3+ characters), email address, and password (8+ characters)
4. Click "Create Account"

**To log in:**
1. Enter your username or email and password
2. Click "Sign In"

Your session stays active until you log out. All your projects, builds, and settings are saved to your account.`,
      },
      {
        id: 'first-build',
        title: 'Your first build (step by step)',
        tags: ['first', 'tutorial', 'walkthrough', 'beginner', 'start'],
        content: `**Step 1: Go to the Build App tab**
Click "Build App" in the top navigation bar. This is the main page for creating new apps.

**Step 2: Write a description**
In the large text box, describe what you want to build. Be as specific as you can. For example:
- "Build a todo app with user login, task categories, due dates, and a dark theme"
- "Create a blog with posts, comments, user profiles, and an admin panel"

The more detail you include, the better the result. Mention specific features, pages, and design preferences.

**Step 3: Choose your settings**
- **Build Mode**: Pick "Fast Build" (~3-5 min) for quick prototypes, or "Full Build" (~10+ min) for production-quality code
- **Power Mode**: Pick "Fast & Cheap" (default) for budget models, or "Max Power" for the highest-quality AI models
- **Tech Stack**: Leave on "Auto (Best Fit)" to let the AI choose, or pick specific technologies yourself

**Step 4: Click "Launch Build"**
The AI agents will start working. You'll see them appear on screen with real-time progress:
- The Planner creates a build plan
- The Architect designs the system
- Frontend/Backend/Database agents write the actual code
- The Tester and Reviewer check everything

**Step 5: Get your app**
When the build finishes:
- Click "Open in IDE" to view and edit the code
- Click "Download ZIP" to save everything to your computer
- Use the preview to see your app running

**Quick tip:** Try one of the quick example buttons below the text box to get started fast.`,
      },
      {
        id: 'navigation',
        title: 'Navigating the app',
        tags: ['navigation', 'tabs', 'menu', 'ui', 'layout'],
        content: `The top navigation bar has these sections:

**Build App** — The starting point. Describe an app and have AI build it for you. Also shows your recent builds.

**IDE** — A full code editor (like VS Code in your browser). Opens after you create or select a project. Includes a file explorer, code editor, terminal, live preview, AI assistant, git integration, and more.

**Explore** — Browse projects shared by the community. Star, fork, and download other people's work for inspiration.

**Settings** (gear icon) — Configure your default AI model and add your own API keys (BYOK).

**Admin** (shield icon) — Only visible to admins. Manage users, view system stats, and adjust credits.

The user avatar in the top-right shows your username. Your session persists across browser reloads.`,
      },
    ],
  },

  // ========================================================================
  // 2. APP BUILDER
  // ========================================================================
  {
    id: 'app-builder',
    title: 'App Builder',
    icon: <Bot className="w-5 h-5" />,
    description: 'How the AI build system works.',
    articles: [
      {
        id: 'build-description',
        title: 'Writing a good app description',
        tags: ['description', 'prompt', 'input', 'tips'],
        content: `Your description is the single most important input. Here's how to write a great one:

**Include these details:**
- What type of app (web app, API, dashboard, etc.)
- Key features and pages
- User roles (admin, regular user, guest)
- Data it should store (users, posts, products, etc.)
- Design preferences (dark theme, minimal, colorful)
- Any specific libraries or tools you want

**Good example:**
"Build a project management app with Kanban boards. Users can sign up, create projects, add tasks with titles, descriptions, and due dates. Tasks can be dragged between columns: To Do, In Progress, Done. Include a dashboard showing task counts and overdue items. Use React with Tailwind CSS on the frontend and Node.js with PostgreSQL on the backend. Dark theme."

**Weak example:**
"Make me an app" — This is too vague. The AI won't know what to build.

**Character limit:** 2,000 characters. The progress bar below the text box shows how much you've used.`,
      },
      {
        id: 'build-modes',
        title: 'Build modes: Fast vs Full',
        tags: ['fast', 'full', 'mode', 'speed', 'quality'],
        content: `**Fast Build (~3-5 minutes)**
- Fewer AI agents involved
- Generates core functionality quickly
- Great for prototyping and testing ideas
- Lower credit usage
- Best for: Quick experiments, simple apps, proof-of-concepts

**Full Build (~10-30 minutes)**
- All AI agents participate (planner, architect, frontend, backend, database, tester, reviewer)
- Comprehensive code with tests and code review
- Better architecture and error handling
- Higher credit usage
- Best for: Production apps, complex features, apps you plan to deploy

**When to use which:**
- Start with Fast Build to test your idea
- If you like the concept, rebuild with Full Build for production quality
- Fast Build is fine for demos and internal tools`,
      },
      {
        id: 'power-modes',
        title: 'Power Modes: Fast, Balanced, Max',
        tags: ['power', 'models', 'quality', 'cost', 'credits', 'pricing'],
        content: `Power Mode controls which AI models build your app. Higher power = better code quality and higher cost.

**Fast & Cheap (1.6x platform cost) — Default**
- Models: Claude Haiku 4.5, GPT-4o Mini, Gemini 2.5 Flash Lite
- Speed: Fastest responses
- Quality: Good for simple apps and prototypes
- Best for: Testing ideas, learning, budget-conscious builds

**Balanced (1.8x platform cost)**
- Models: Claude Sonnet 4.5, GPT-5, Gemini 3 Flash
- Speed: Moderate
- Quality: Solid production-quality code
- Best for: Most real-world applications

**Max Power (2.0x platform cost)**
- Models: Claude Opus 4.6, GPT-5.2 Codex, Gemini 3 Pro
- Speed: Slower (larger models take longer)
- Quality: Highest possible — most capable models
- Best for: Complex apps, mission-critical code, production builds

**Transparent pricing:** The exact cost breakdown is shown in the UI, including per‑million‑token rates and live spend.`,
      },
      {
        id: 'tech-stack',
        title: 'Choosing a tech stack',
        tags: ['stack', 'react', 'node', 'python', 'go', 'frontend', 'backend', 'database', 'auto'],
        content: `You can let the AI pick your tech stack automatically or choose each layer yourself.

**Auto (Best Fit) — Recommended for most users**
The AI analyzes your description and selects the best technologies. This is the default and works great for most projects.

**Manual selection — For developers who have preferences**
Click individual tech cards to choose:

**Frontend options:**
- React — Most popular, huge ecosystem
- Next.js — React with server-side rendering and API routes
- Vue — Easy to learn, great documentation
- Angular — Enterprise-grade, TypeScript-first
- Svelte — Lightweight, fast, minimal boilerplate

**Backend options:**
- Node.js (Express) — JavaScript everywhere
- Python (FastAPI/Flask) — Great for data-heavy apps
- Go (Gin) — Fast, compiled, excellent for APIs
- Rust (Axum) — Maximum performance and safety
- Java (Spring Boot) — Enterprise standard
- Ruby (Rails) — Rapid development
- PHP (Laravel) — Widely deployed

**Database options:**
- PostgreSQL — Full-featured relational database (recommended)
- MySQL — Popular relational alternative
- MongoDB — Document database for flexible schemas
- SQLite — Lightweight, embedded, great for small apps
- Firestore — Google's serverless NoSQL

**Styling options:**
- Tailwind CSS — Utility-first, most popular modern choice
- Bootstrap — Pre-built components
- Material UI — Google's design system
- CSS Modules — Scoped styles

You can mix and match. For example: React + Go + PostgreSQL + Tailwind.`,
      },
      {
        id: 'agents',
        title: 'AI Agents explained',
        tags: ['agents', 'planner', 'architect', 'frontend', 'backend', 'tester', 'reviewer', 'lead'],
        content: `During a build, multiple specialized AI agents work together like a software team:

**Lead Agent**
- Coordinates all other agents
- Your main point of contact during the build
- You can chat with the Lead agent to ask questions or request changes

**Planner Agent**
- Reads your description and creates a detailed build plan
- Identifies features, data models, API endpoints, and UI components
- Decides the order of operations

**Architect Agent**
- Designs the system architecture
- Plans how components connect
- Defines the folder structure and file organization

**Frontend Agent**
- Generates all UI components (pages, forms, buttons, layouts)
- Implements responsive design
- Handles client-side routing and state management

**Backend Agent**
- Creates API endpoints and business logic
- Implements authentication and authorization
- Handles data validation and error handling

**Database Agent**
- Designs database schemas (tables, columns, relationships)
- Creates migrations and seed data
- Writes database queries

**Testing Agent**
- Writes unit tests and integration tests
- Validates that the code works correctly
- Reports test coverage

**Reviewer Agent**
- Reviews all generated code for quality
- Checks for security issues, bugs, and best practices
- Suggests improvements

Each agent has a status indicator: Idle, Working, Completed, or Error. You can watch them work in real time.`,
      },
      {
        id: 'during-build',
        title: 'What happens during a build',
        tags: ['build', 'progress', 'chat', 'checkpoint', 'realtime'],
        content: `Once you click "Launch Build," here's what happens:

**1. Connection**
A WebSocket connection is established for real-time updates. You'll see a progress bar and agent cards appear.

**2. Planning phase**
The Planner agent analyzes your description and creates a structured build plan including features, data models, API endpoints, and file structure.

**3. Architecture phase**
The Architect agent designs the system — how files are organized, how components communicate, and what the API looks like.

**4. Code generation phase**
Multiple agents work in parallel:
- Frontend Agent generates React/Vue components, pages, and layouts
- Backend Agent creates API routes, handlers, and middleware
- Database Agent writes schemas and queries

**5. Testing phase (Full Build only)**
The Testing Agent writes and runs automated tests.

**6. Review phase (Full Build only)**
The Reviewer Agent checks code quality, security, and best practices.

**7. Completion**
All generated files are collected and presented to you. You'll see:
- A success celebration with file count
- Options to Open in IDE, Download ZIP, or Preview

**Chatting during the build:**
You can send messages to the Lead Agent while the build is running. Use this to:
- Ask questions about what's being built
- Request specific changes
- Clarify requirements

**Checkpoints:**
The system saves progress checkpoints during the build. If something goes wrong, you can roll back to a previous checkpoint.`,
      },
      {
        id: 'after-build',
        title: 'After your build completes',
        tags: ['complete', 'open', 'download', 'zip', 'ide', 'preview', 'deploy'],
        content: `When a build finishes successfully, you have several options:

**Open in IDE**
Click this to switch to the full IDE view with all your generated files loaded. You can:
- Browse the file tree
- Edit any file
- Run the project in the terminal
- See a live preview
- Use the AI assistant to modify code

**Download ZIP**
Downloads all generated files as a ZIP archive to your computer. You can then:
- Open the project in your local editor (VS Code, etc.)
- Run it locally
- Deploy it to any hosting service

**Live Preview**
If available, see your app running right in the browser. Use the responsive viewport controls to test different screen sizes (mobile, tablet, desktop).

**Build History**
All completed builds are saved to your account. Scroll down on the Build App page to see your recent builds. You can re-open or download any previous build at any time.`,
      },
      {
        id: 'build-history',
        title: 'Build History',
        tags: ['history', 'past', 'previous', 'builds', 'saved'],
        content: `Every build you run is automatically saved to your account.

**Viewing your builds:**
Scroll down on the Build App page. Below the build form, you'll see a "Recent Builds" section showing your past builds.

**Each build shows:**
- The app description
- Build status (Completed, Failed)
- Tech stack used
- Number of files generated
- Build duration
- Power mode used
- When it was built

**Actions:**
- Click a build to open it
- Click the Download icon to get the files as a ZIP
- Click the folder icon to open the build's files

Builds are persisted in the database — they survive server restarts and are available from any device you log into.`,
      },
      {
        id: 'import-options',
        title: 'Importing existing projects',
        tags: ['import', 'github', 'replit', 'migrate', 'existing'],
        content: `You can bring existing projects into APEX.BUILD from other platforms:

**Import from GitHub**
1. Click "Import from GitHub" on the Build App page
2. Paste the GitHub repository URL
3. Optionally add a GitHub personal access token (required for private repos)
4. The wizard validates the URL and shows repo info (stars, language, size)
5. Customize the project name and description
6. Click Import — all files are pulled into a new APEX.BUILD project

**Migrate from Replit**
1. Click "Migrate from Replit" on the Build App page
2. Enter the Replit project URL
3. Click "Import & Migrate"
4. APEX.BUILD's agents analyze and reconstruct the project

After import, the project opens in the IDE where you can edit, run, and deploy it.`,
      },
    ],
  },

  // ========================================================================
  // 3. IDE (CODE EDITOR)
  // ========================================================================
  {
    id: 'ide',
    title: 'IDE (Code Editor)',
    icon: <Code2 className="w-5 h-5" />,
    description: 'The full development environment.',
    articles: [
      {
        id: 'ide-overview',
        title: 'IDE overview',
        tags: ['ide', 'editor', 'overview', 'layout', 'panels'],
        content: `The IDE is a full code editor that runs in your browser — similar to VS Code.

**Layout:**
- **Left panel** — File Explorer, Search, Git, Version History
- **Center** — Code editor with tabs for open files
- **Right panel** — AI Assistant, Code Comments, Collaboration, Database, Settings
- **Bottom panel** — Terminal, Output, Problems

All panels can be toggled open/closed. Drag the dividers to resize them.

**To access the IDE:**
- Click "IDE" in the top navigation
- Click "Open in IDE" after a build completes
- Note: You need to have a project selected. If you see "No Project Selected," go to Build App first to create one.`,
      },
      {
        id: 'file-explorer',
        title: 'File Explorer',
        tags: ['files', 'explorer', 'tree', 'create', 'delete', 'rename', 'folder'],
        content: `The File Explorer (left panel, first tab) shows all files and folders in your project.

**Creating files/folders:**
- Click the "New File" or "New Folder" icon at the top of the explorer
- Enter the file name (include the extension, e.g., "utils.ts")
- For nested files, include the path: "src/components/Header.tsx"

**Opening files:**
Click any file to open it in the editor. It appears as a tab in the center panel. You can have multiple files open at once.

**Deleting files:**
Right-click a file or use the delete button that appears on hover.

**Renaming files:**
Right-click and select rename, or double-click the file name.

**Folder structure:**
Files are organized in a tree view. Click folder arrows to expand/collapse them.`,
      },
      {
        id: 'code-editor',
        title: 'Code editor features',
        tags: ['editor', 'monaco', 'autocomplete', 'syntax', 'format', 'shortcuts'],
        content: `The code editor is powered by Monaco (the same engine as VS Code) and supports 50+ programming languages.

**Key features:**
- Syntax highlighting for all major languages
- IntelliSense auto-completion (suggestions as you type)
- Code folding (collapse/expand code blocks)
- Bracket matching and auto-closing
- Multi-cursor editing (hold Alt and click to add cursors)
- Minimap for quick navigation
- Line numbers and indentation guides

**Keyboard shortcuts:**
- Ctrl+S (Cmd+S on Mac) — Save file
- Ctrl+Z / Ctrl+Y — Undo / Redo
- Ctrl+F — Find in file
- Ctrl+H — Find and Replace
- Ctrl+D — Select next occurrence
- Ctrl+Shift+K — Delete line
- Alt+Up/Down — Move line up/down
- Ctrl+/ — Toggle comment
- F12 — Go to definition
- Alt+F12 — Peek definition
- Shift+Alt+F — Format document

**Split editor:**
You can split the editor to view multiple files side by side. Right-click a tab and select "Split Right" or "Split Down."`,
      },
      {
        id: 'search-files',
        title: 'Searching across files',
        tags: ['search', 'find', 'grep', 'regex'],
        content: `The Search panel (left panel, second tab) lets you search across all files in your project.

**How to use:**
1. Click the Search icon in the left panel
2. Type your query in the search box
3. Results appear instantly with file paths and line numbers

**Options:**
- **Case sensitive** — Toggle to match exact capitalization
- **Regex** — Use regular expressions for advanced patterns (e.g., "function\\s+\\w+" finds all function declarations)

**Search results:**
Each result shows the file name, line number, and matching text. Click a result to jump directly to that line in the editor.

**Find and Replace in a file:**
Press Ctrl+H (Cmd+H on Mac) to open Find and Replace within the current file.`,
      },
      {
        id: 'terminal',
        title: 'Terminal',
        tags: ['terminal', 'shell', 'command', 'npm', 'run', 'bash'],
        content: `The Terminal (bottom panel) gives you a full command-line shell in your browser.

**What you can do:**
- Run your project: npm start, python app.py, go run main.go
- Install packages: npm install, pip install, go get
- Run tests: npm test, pytest, go test
- Use git commands: git status, git commit
- Run any shell command

**Controls:**
- Click the Terminal tab in the bottom panel to open it
- Type commands and press Enter
- Use Up/Down arrows to scroll through command history
- Press Ctrl+C to stop a running process
- Click "Clear" to clear the terminal output

**Supported shells:** bash, sh, zsh

**Tips:**
- The terminal has full interactive support (it uses a real PTY)
- You can resize the terminal by dragging the top edge
- Multiple terminal sessions are supported`,
      },
      {
        id: 'live-preview',
        title: 'Live Preview',
        tags: ['preview', 'browser', 'viewport', 'responsive', 'hot reload'],
        content: `The Live Preview panel shows your running application right inside the IDE.

**Starting the preview:**
1. Click the Preview toggle button in the IDE toolbar
2. Click the Play button to start the dev server
3. Your app loads in an iframe

**Viewport controls:**
Switch between different screen sizes to test responsive design:
- Mobile (375x667)
- Tablet (768x1024)
- Desktop (1280x800)
- Full (fills the panel)
- Fullscreen (takes over the whole screen)

**Preview tabs:**
- **Preview** — The actual running app
- **Console** — Browser console output (logs, errors, warnings)
- **Network** — HTTP request inspector (see API calls, response times, status codes)

**Hot reload:**
When you edit a file, the preview automatically refreshes to show your changes. For React apps, this uses Fast Refresh (instant updates without losing state).

**Opening externally:**
Click the external link icon to open the preview in a new browser tab.`,
      },
      {
        id: 'ide-toolbar',
        title: 'IDE toolbar buttons',
        tags: ['toolbar', 'save', 'run', 'download', 'share', 'deploy'],
        content: `The toolbar at the top of the IDE has these action buttons:

**Save All** — Saves all open files (Ctrl+Shift+S)

**Run** — Executes your project. Starts the dev server and opens the terminal.

**Download ZIP** — Exports your entire project as a downloadable ZIP file. All files are included.

**Share** — Copies a shareable link to your clipboard so others can view your project.

**Deploy** — Opens deployment options to push your app live to Vercel, Netlify, Render, or APEX.BUILD's native hosting (.apex.app).

**Terminal Toggle** — Show/hide the bottom terminal panel.

**Preview Toggle** — Show/hide the live preview panel.

**AI Chat Toggle** — Show/hide the AI assistant panel.`,
      },
    ],
  },

  // ========================================================================
  // 4. AI ASSISTANT
  // ========================================================================
  {
    id: 'ai-assistant',
    title: 'AI Assistant',
    icon: <Sparkles className="w-5 h-5" />,
    description: 'Your in-IDE AI coding partner.',
    articles: [
      {
        id: 'ai-chat',
        title: 'Using the AI Assistant',
        tags: ['ai', 'chat', 'assistant', 'generate', 'help', 'code'],
        content: `The AI Assistant lives in the right panel of the IDE. It can help with any coding task.

**How to use it:**
1. Open the AI panel (right side, or click the AI toggle in the toolbar)
2. Type your request in the message box
3. Press Enter or click Send

**What you can ask:**
- "Generate a login form with email and password validation"
- "Fix the bug in this function" (paste the code)
- "Explain what this code does" (paste the code)
- "Write unit tests for the UserService class"
- "Refactor this to use async/await"
- "Add error handling to this API endpoint"
- "Create documentation for this module"

**Working with responses:**
When the AI generates code, you'll see:
- **Copy button** — Copy the code to your clipboard
- **Insert button** — Insert the code directly into your current editor file
- Syntax-highlighted code blocks with language badges

**Capabilities dropdown:**
Choose a specific AI capability to get better results:
- Code Generation — Create new code from a description
- Code Completion — Finish partially written code
- Debugging — Find and fix bugs
- Refactoring — Improve code structure
- Explanation — Understand existing code
- Code Review — Get quality feedback
- Testing — Generate test cases
- Documentation — Create comments and docs

**Provider selection:**
Choose which AI provider to use, or leave on "Auto" for intelligent routing.`,
      },
      {
        id: 'ai-providers',
        title: 'AI providers and models',
        tags: ['claude', 'gpt', 'gemini', 'grok', 'ollama', 'models', 'provider'],
        content: `APEX.BUILD integrates with multiple AI providers. Each has different strengths:

**Claude (Anthropic)**
- Models: Opus 4.6 (most capable), Sonnet 4.5 (balanced), Haiku 4.5 (fast)
- Strengths: Excellent code quality, long context understanding, careful reasoning
- Best for: Complex code generation, debugging, architecture

**GPT (OpenAI)**
- Models: GPT-5.2 Codex (most capable), GPT-5 (balanced), GPT-4o Mini (fast)
- Strengths: Wide knowledge, creative solutions, strong at many languages
- Best for: General coding tasks, multi-language projects

**Gemini (Google)**
- Models: Gemini 3 Pro (most capable), Gemini 3 Flash (balanced), Gemini 2.5 Flash Lite (fast)
- Strengths: Fast responses, good at documentation, strong reasoning
- Best for: Quick tasks, documentation, data processing code

**Grok (xAI)**
- Models: Grok 4 Heavy, Grok 4.1 Thinking, Grok 4.1, Grok 4 Fast
- Strengths: Real-time knowledge, creative problem-solving
- Best for: Cutting-edge tasks, up-to-date solutions

**Ollama (Local)**
- Models: DeepSeek-R1, Qwen 3 Coder, Llama 3.3, and more
- Strengths: Free, runs on your own hardware, no API costs
- Best for: Privacy-sensitive work, unlimited usage
- Requires: A running Ollama server on your machine or network

**Auto mode (default):**
When set to "Auto," APEX.BUILD intelligently routes your request to the best available provider based on the task complexity, available models, and your configuration.`,
      },
    ],
  },

  // ========================================================================
  // 5. SETTINGS & BYOK
  // ========================================================================
  {
    id: 'settings',
    title: 'Settings & API Keys',
    icon: <Key className="w-5 h-5" />,
    description: 'Configuration and Bring Your Own Key (BYOK).',
    articles: [
      {
        id: 'settings-overview',
        title: 'Settings page overview',
        tags: ['settings', 'configure', 'preferences'],
        content: `Access Settings by clicking the gear icon in the top navigation bar.

**The Settings page has two sections:**

**1. Default AI Model**
Choose which AI model is used by default for builds and AI assistant requests. You can pick "Auto" (recommended) or select a specific provider and model. This preference is saved to your account and applies everywhere.

**2. API Keys (BYOK)**
This is where you add your own API keys for each AI provider. See the BYOK articles below for full details.`,
      },
      {
        id: 'byok-overview',
        title: 'BYOK: Bring Your Own Key',
        tags: ['byok', 'api key', 'own key', 'free', 'bring your own'],
        content: `BYOK (Bring Your Own Key) lets you use your personal API keys from AI providers instead of APEX.BUILD's shared platform keys. This is one of APEX.BUILD's most powerful features.

**Why use BYOK?**
- **Lower costs** — You pay the AI provider directly at their rates plus a small routing fee ($0.25 per 1M tokens).
- **Unlimited usage** — BYOK requests don't count against your plan's monthly AI request limit
- **Your preferred models** — Use any model available on the provider's API
- **Full control** — Enable/disable providers, validate keys, track your own usage

**BYOK is available on all plans**, including the Free tier. You just need your own API key from the provider.

**Supported providers for BYOK:**
- Claude (Anthropic) — Get your key at console.anthropic.com
- GPT (OpenAI) — Get your key at platform.openai.com
- Gemini (Google) — Get your key at aistudio.google.com
- Grok (xAI) — Get your key at console.x.ai
- Ollama (Local) — Just provide your server URL (e.g., http://localhost:11434)`,
      },
      {
        id: 'byok-setup',
        title: 'Setting up BYOK (step by step)',
        tags: ['byok', 'setup', 'add key', 'configure', 'how to'],
        content: `**To add an API key:**

1. Go to Settings (gear icon in top nav)
2. Scroll to "API Keys (BYOK)"
3. Find the provider you want (Claude, GPT, Gemini, Grok, or Ollama)
4. Click on the provider's card to expand it
5. Paste your API key into the input field
6. Click "Save & Activate"

**To validate your key:**
After saving, click "Validate" to test that the key works. A green checkmark means it's valid.

**To choose a specific model:**
Each provider card has a model dropdown. Select your preferred model (e.g., "claude-opus-4-6" for Claude's most powerful model).

**To disable a provider:**
Toggle the "Active" switch off. Your key stays saved but won't be used.

**To remove a key:**
Click the "Delete" button on the provider card. This permanently removes the key.

**Security:** All API keys are encrypted with AES-256-GCM before storage. They are never stored in plaintext and never exposed in API responses.`,
      },
      {
        id: 'byok-ollama',
        title: 'Using Ollama (free local AI)',
        tags: ['ollama', 'local', 'free', 'self-hosted', 'deepseek', 'llama'],
        content: `Ollama lets you run AI models on your own computer for free. No API keys or payments needed.

**Setup:**
1. Install Ollama from ollama.com
2. Pull a model: ollama pull deepseek-r1:8b (or any model you want)
3. Start Ollama: ollama serve (it runs on http://localhost:11434 by default)
4. In APEX.BUILD Settings > BYOK > Ollama, enter your server URL
5. Click "Save & Activate"

**Available local models:**
- DeepSeek-R1 (18b / 8b) — Excellent for code generation
- Qwen 3 Coder (30b) — Strong coding model
- DeepSeek-V3.2 — General purpose
- Llama 3.3 70B — Meta's largest open model (needs powerful hardware)

**Requirements:**
- Ollama must be running and accessible from your browser
- If running APEX.BUILD in the cloud, Ollama needs to be on a reachable URL (not localhost)
- More powerful models need more RAM (8b models need ~8GB, 70B models need ~48GB)

**Pros:** Completely free, unlimited usage, full privacy
**Cons:** Requires local hardware, slower than cloud APIs, smaller models are less capable`,
      },
    ],
  },

  // ========================================================================
  // 6. GIT INTEGRATION
  // ========================================================================
  {
    id: 'git',
    title: 'Git & GitHub',
    icon: <GitBranch className="w-5 h-5" />,
    description: 'Version control and GitHub integration.',
    articles: [
      {
        id: 'git-panel',
        title: 'Git panel in the IDE',
        tags: ['git', 'version control', 'commit', 'push', 'pull', 'branch'],
        content: `The Git panel is in the left sidebar of the IDE (third tab, branch icon).

**Tabs in the Git panel:**

**Changes tab:**
Shows files you've modified since the last commit.
- Staged changes — Files ready to commit
- Unstaged changes — Modified files not yet staged
- Click the + button next to a file to stage it
- Click "Stage All" to stage everything
- Enter a commit message and click "Commit"

**Commits tab:**
Shows your commit history with:
- Commit hash (short form)
- Commit message
- Author and timestamp

**Branches tab:**
Lists all branches in the repository.
- The current branch is highlighted
- Click "Switch" to change branches
- Click "New Branch" to create a new one
- Protected branches can't be deleted

**Pull Requests tab:**
Lists open PRs with title, state, author, and branch info.

**Remote operations:**
- **Push** — Upload your commits to GitHub
- **Pull** — Download and merge changes from GitHub`,
      },
      {
        id: 'github-export',
        title: 'Exporting to GitHub',
        tags: ['github', 'export', 'push', 'repository', 'new repo'],
        content: `You can export any APEX.BUILD project to a GitHub repository.

**How to export:**
1. Open your project in the IDE
2. Go to the Git panel
3. Click "Connect" and enter a GitHub repository URL
4. Or use the export feature to create a new GitHub repo from your project

**Requirements:**
- A GitHub account
- A personal access token with repo permissions (for private repos)
- Pro plan or higher (GitHub export is not available on the Free tier)`,
      },
    ],
  },

  // ========================================================================
  // 7. LIVE PREVIEW & DEPLOYMENT
  // ========================================================================
  {
    id: 'deployment',
    title: 'Preview & Deployment',
    icon: <Globe className="w-5 h-5" />,
    description: 'Running and deploying your apps.',
    articles: [
      {
        id: 'preview-details',
        title: 'Live Preview in detail',
        tags: ['preview', 'dev server', 'hot reload', 'console', 'network'],
        content: `The Live Preview gives you a real browser view of your running application.

**Starting the preview:**
Click the Preview toggle in the IDE toolbar, then click Play. APEX.BUILD auto-detects your project type and starts the appropriate dev server (Node.js, Python, Go, etc.).

**Console tab:**
Shows browser console output — equivalent to the DevTools console:
- Log messages (console.log)
- Warnings (console.warn)
- Errors (console.error)
- Filter by type to focus on what matters

**Network tab:**
Shows all HTTP requests your app makes:
- Request method (GET, POST, etc.) and URL
- Status code (200, 404, 500, etc.)
- Response time and size
- Click a request to see headers and response body
- Filter by type (XHR, JS, CSS, Images)

**Server auto-detection:**
APEX.BUILD automatically detects your backend framework (Express, FastAPI, Gin, etc.) and starts the right server with the right command.`,
      },
      {
        id: 'deploy-options',
        title: 'Deployment options',
        tags: ['deploy', 'vercel', 'netlify', 'render', 'hosting', 'apex.app'],
        content: `Deploy your app to make it accessible on the internet.

**Vercel**
- Best for: Next.js, React, static sites
- Features: Serverless functions, edge network, auto-scaling
- Free tier available

**Netlify**
- Best for: Static sites, JAMstack apps
- Features: Netlify Functions, form handling, edge functions
- Free tier available

**Render**
- Best for: Full-stack apps, backends, Docker containers
- Features: Persistent storage, cron jobs, background workers
- Free tier available

**APEX.BUILD Native (.apex.app)**
- Best for: Apps you want to host directly on APEX
- Features: Container-based hosting, auto-scaling, SSL, health monitoring
- Your app gets a URL like: your-app.apex.app
- Always-on option available
- Built-in monitoring (CPU, memory, traffic metrics)

**How to deploy:**
1. Click the "Deploy" button in the IDE toolbar
2. Select your deployment provider
3. Follow the provider-specific configuration steps
4. Click Deploy
5. Your app is live!

**Deployment history:**
Each deployment is logged. You can view past deployments, redeploy previous versions, and check deployment logs.`,
      },
    ],
  },

  // ========================================================================
  // 8. COLLABORATION
  // ========================================================================
  {
    id: 'collaboration',
    title: 'Collaboration',
    icon: <Users className="w-5 h-5" />,
    description: 'Working with others in real time.',
    articles: [
      {
        id: 'collab-overview',
        title: 'Real-time collaboration',
        tags: ['collab', 'collaboration', 'team', 'multiplayer', 'real-time'],
        content: `APEX.BUILD supports real-time collaborative editing, similar to Google Docs but for code.

**Features:**
- **Live cursors** — See where other people are editing in real time
- **Presence indicators** — See who's online and which file they're viewing
- **Conflict-free editing** — Uses Operational Transformation (OT) to merge edits seamlessly
- **Code comments** — Add threaded comments on specific lines of code

**How to collaborate:**
1. Share your project link with teammates
2. They open the project in their browser
3. You'll see their cursors and edits in real time

**Code Comments:**
- Click the Comments tab in the right panel
- Select a line range in the editor
- Add your comment
- Others can reply, creating a threaded discussion
- Resolve comments when the issue is addressed
- Add emoji reactions

**Plan limits:**
- Free: 1 collaborator per project
- Pro: 3 collaborators per project
- Team: Unlimited collaborators
- Enterprise: Unlimited with RBAC`,
      },
    ],
  },

  // ========================================================================
  // 9. VERSION HISTORY
  // ========================================================================
  {
    id: 'version-history',
    title: 'Version History',
    icon: <History className="w-5 h-5" />,
    description: 'Track and restore file changes.',
    articles: [
      {
        id: 'versions',
        title: 'File version history',
        tags: ['version', 'history', 'restore', 'diff', 'undo', 'revert'],
        content: `Every change to every file is tracked with a full version history.

**Viewing history:**
1. In the IDE left panel, click the History tab (clock icon)
2. Select a file to see all its versions
3. Each version shows the author, timestamp, and what changed (lines added/removed)

**Restoring a version:**
Click "Restore" on any previous version to revert the file to that state. A new version is created for the restore (so you can always undo it).

**Comparing versions:**
View diffs between any two versions to see exactly what changed. Added lines show in green, removed lines in red.

**Pinning versions:**
Pin important versions (like "before refactor" or "working state") to prevent them from being automatically cleaned up.

**Auto-save vs manual save:**
Both are tracked separately in the history, so you can see every change regardless of how it was saved.`,
      },
    ],
  },

  // ========================================================================
  // 10. EXPLORE & COMMUNITY
  // ========================================================================
  {
    id: 'explore',
    title: 'Explore & Community',
    icon: <Globe className="w-5 h-5" />,
    description: 'Discover and share projects.',
    articles: [
      {
        id: 'explore-page',
        title: 'Browsing community projects',
        tags: ['explore', 'community', 'discover', 'browse', 'templates'],
        content: `The Explore page lets you discover projects shared by other APEX.BUILD users.

**Tabs:**
- **Trending** — Projects gaining popularity
- **New** — Recently shared projects
- **Popular** — All-time most starred projects

**Project cards show:**
- Project thumbnail
- Title and description
- Author and avatar
- Star count, fork count, view count
- Technology tags
- Last updated date

**Actions:**
- **Star** — Like a project (helps it trend)
- **Fork** — Create your own copy to modify
- **Download** — Get the project files
- **View** — Open the full project details

**Sharing your projects:**
1. Open your project in the IDE
2. Click "Share" in the toolbar
3. Choose to make it public
4. Add a description and tags
5. Your project appears on the Explore page for others to find`,
      },
    ],
  },

  // ========================================================================
  // 11. PLANS & BILLING
  // ========================================================================
  {
    id: 'billing',
    title: 'Plans & Billing',
    icon: <CreditCard className="w-5 h-5" />,
    description: 'Subscription tiers and credit system.',
    articles: [
      {
        id: 'plans',
        title: 'Subscription plans',
        tags: ['plan', 'subscription', 'free', 'pro', 'team', 'enterprise', 'pricing'],
        content: `APEX.BUILD offers four subscription tiers:

**Free — $0/month**
- 500 AI requests/month (using platform keys)
- 3 projects
- 1 GB storage
- 50 min/day code execution
- 1 collaborator per project
- BYOK: Available (use your own keys + routing fee)

**Pro — $12/month**
- 5,000 AI requests/month
- Unlimited projects
- 10 GB storage
- 500 min/day execution
- 3 collaborators per project
- GitHub export
- Priority AI routing
- 14-day free trial

**Team — $29/month**
- 25,000 AI requests/month
- Unlimited projects
- 50 GB storage
- 2,000 min/day execution
- Unlimited collaborators
- Custom integrations
- 14-day free trial

**Enterprise — $79/month**
- Unlimited AI requests
- Unlimited everything
- SAML SSO
- SCIM provisioning
- RBAC (Role-Based Access Control)
- Audit logging
- 99.9% SLA
- Dedicated support
- 30-day free trial

**Important:** BYOK (Bring Your Own Key) is available on ALL plans and does not count toward your monthly AI request limits, but it does include a small routing fee.

Annual billing saves 20%.`,
      },
      {
        id: 'credits',
        title: 'Credits and usage',
        tags: ['credits', 'usage', 'cost', 'tokens', 'limit', 'quota'],
        content: `**How credits work:**

Each AI request uses a small amount of credits based on the model used and the number of tokens processed. Your plan includes a monthly allowance of platform-key AI requests.

**Checking your usage:**
Your credit balance and usage are visible in your profile and in the admin panel (for admins).

**What counts as a request:**
- Each AI generation call (building apps, AI assistant chat, code completion)
- Token count varies by prompt size and response length

**What doesn't count:**
- BYOK requests (don't count toward plan limits, but do incur routing fee)
- File editing, terminal usage, preview
- Git operations
- Browsing and navigation

**Running low on credits:**
- Upgrade your plan for more monthly requests
- Add your own API keys via BYOK (lower cost + routing fee)
- Wait for monthly reset (limits refresh each billing cycle)`,
      },
    ],
  },

  // ========================================================================
  // 12. DATABASE
  // ========================================================================
  {
    id: 'database',
    title: 'Database',
    icon: <Database className="w-5 h-5" />,
    description: 'Managed databases for your projects.',
    articles: [
      {
        id: 'managed-db',
        title: 'Managed databases',
        tags: ['database', 'postgresql', 'sql', 'tables', 'queries', 'managed'],
        content: `Each project can have an auto-provisioned PostgreSQL database.

**Database panel (IDE right sidebar):**
The Database tab shows:
- Connection status (running/stopped)
- Connection details (host, port, username, password)
- List of tables
- Column information for each table
- A SQL console for running queries

**What you can do:**
- View your tables and their structure
- Run SQL queries directly
- Create new tables
- Add columns
- View data previews
- Export/import data
- Create and restore backups

**Connection details:**
Your database connection string is available in the Database panel. Use it in your application code to connect:
- Host, Port, Username, Password, Database Name
- Or use the full connection URL

**Auto-provisioning:**
When your build generates database schemas, a PostgreSQL database is automatically created and the schemas are applied. You don't need to set anything up manually.`,
      },
    ],
  },

  // ========================================================================
  // 13. ADMIN
  // ========================================================================
  {
    id: 'admin',
    title: 'Admin Dashboard',
    icon: <Shield className="w-5 h-5" />,
    description: 'For platform administrators only.',
    articles: [
      {
        id: 'admin-overview',
        title: 'Admin Dashboard',
        tags: ['admin', 'dashboard', 'users', 'management', 'stats'],
        content: `The Admin Dashboard is only visible to users with admin or super admin privileges.

**Overview Stats:**
- Total users and active users
- Admin count and pro subscribers
- Total projects and active projects
- Total AI requests and cost

**System Stats:**
- AI provider usage breakdown (Claude, GPT, Gemini requests)
- Subscription tier breakdown (Free, Pro, Team, Enterprise)
- Storage metrics (total files, total storage used)

**User Management:**
Search and manage all users with:
- Username, email, status
- Subscription type and credit balance
- Admin/privilege badges
- Actions: Edit, Add Credits, Toggle Status, View Details, Delete

**Editing a user:**
- Toggle admin, super admin, unlimited credits, bypass billing, bypass rate limits
- Change subscription type
- Activate/deactivate account
- Adjust credit balance

**Adding credits:**
Click "Add Credits" on any user to manually add credit balance with a reason (tracked for auditing).`,
      },
    ],
  },

  // ========================================================================
  // 14. SECRETS & SECURITY
  // ========================================================================
  {
    id: 'security',
    title: 'Secrets & Security',
    icon: <Lock className="w-5 h-5" />,
    description: 'Environment variables and security features.',
    articles: [
      {
        id: 'secrets',
        title: 'Managing secrets (environment variables)',
        tags: ['secrets', 'env', 'environment', 'variables', 'api keys', 'sensitive'],
        content: `Secrets are sensitive values (API keys, database passwords, etc.) that your application needs at runtime.

**How to manage secrets:**
- Secrets are stored encrypted (AES-256-GCM)
- They're injected as environment variables when your code runs
- Secret values are never shown in plain text in the UI — only the name is visible

**Adding a secret:**
1. Go to the Secrets panel for your project
2. Enter a name (e.g., DATABASE_URL, STRIPE_API_KEY)
3. Enter the value
4. Click Save

**Using secrets in your code:**
Access them as environment variables:
- Node.js: process.env.DATABASE_URL
- Python: os.environ['DATABASE_URL']
- Go: os.Getenv("DATABASE_URL")

**Audit trail:**
All secret accesses are logged for security. You can view the audit log to see who accessed what and when.

**Rotation:**
Click "Rotate" to generate a new encryption key for a secret without changing its value.`,
      },
      {
        id: 'code-execution-security',
        title: 'Code execution security',
        tags: ['sandbox', 'docker', 'security', 'execution', 'safe'],
        content: `When you run code in APEX.BUILD, it executes in a secure sandbox:

**Docker container sandboxing (default):**
- Each execution runs in an isolated container
- Read-only filesystem (can't modify system files)
- Memory limit: 256MB
- CPU limit: 0.5 cores
- Network isolation (optional)
- Seccomp syscall filtering (blocks dangerous system calls)
- Automatic timeout enforcement

**Supported languages:**
JavaScript/TypeScript, Python, Go, Rust, Java, C, C++, Ruby, PHP — each with its own runtime environment.

**Process sandbox (fallback):**
If Docker isn't available, a lighter process-based sandbox is used with OS-level restrictions.

**What this means for you:**
Your code runs safely without being able to affect other users or the platform. You can experiment freely — even if your code crashes, nothing else is impacted.`,
      },
    ],
  },

  // ========================================================================
  // 15. TEMPLATES & PACKAGES
  // ========================================================================
  {
    id: 'templates',
    title: 'Templates & Packages',
    icon: <Layers className="w-5 h-5" />,
    description: 'Starter templates and package management.',
    articles: [
      {
        id: 'templates-list',
        title: 'Project templates',
        tags: ['template', 'starter', 'boilerplate', 'scaffold'],
        content: `APEX.BUILD includes 15+ starter templates for common project types:

- React SPA with Tailwind
- Next.js Full-stack
- Vue 3 Application
- Python FastAPI Backend
- Go REST API
- Rust Web Service
- Django Full-stack
- Express.js Server
- Svelte Application
- Astro Static Site
- React Native Mobile
- Electron Desktop App
- Nuxt Framework
- And more...

**Using a template:**
Templates are available when creating a new project. Select one and it pre-populates all the starter files, configuration, and dependencies for that tech stack.`,
      },
      {
        id: 'packages',
        title: 'Package management',
        tags: ['packages', 'npm', 'pip', 'install', 'dependencies'],
        content: `Install and manage packages (libraries) for your project.

**Supported package managers:**
- **npm** — JavaScript/TypeScript packages (React, Express, etc.)
- **pip** — Python packages (FastAPI, Django, etc.)
- **Go Modules** — Go packages

**How to install packages:**
1. Use the terminal: npm install lodash, pip install requests, go get github.com/...
2. Or use the Package panel to search and install visually

**Package search:**
Search for packages by name to find the right library. Results show version, download count, and description.

**Checking for updates:**
The package manager can check for outdated dependencies and suggest updates.`,
      },
    ],
  },

  // ========================================================================
  // 16. KEYBOARD SHORTCUTS
  // ========================================================================
  {
    id: 'shortcuts',
    title: 'Keyboard Shortcuts',
    icon: <Monitor className="w-5 h-5" />,
    description: 'Speed up your workflow.',
    articles: [
      {
        id: 'shortcuts-list',
        title: 'All keyboard shortcuts',
        tags: ['shortcuts', 'keyboard', 'hotkeys', 'keys', 'commands'],
        content: `**File operations:**
- Ctrl+S (Cmd+S) — Save current file
- Ctrl+Shift+S — Save all files

**Editing:**
- Ctrl+Z — Undo
- Ctrl+Y (Ctrl+Shift+Z) — Redo
- Ctrl+X — Cut line (no selection) or cut selection
- Ctrl+C — Copy line (no selection) or copy selection
- Ctrl+D — Select next occurrence of current word
- Ctrl+Shift+K — Delete current line
- Alt+Up/Down — Move line up/down
- Ctrl+Shift+Up/Down — Copy line up/down
- Ctrl+/ — Toggle line comment
- Ctrl+Shift+A — Toggle block comment
- Tab — Indent
- Shift+Tab — Outdent

**Navigation:**
- Ctrl+G — Go to line number
- Ctrl+P — Quick open file
- F12 — Go to definition
- Alt+F12 — Peek definition
- Ctrl+Shift+O — Go to symbol

**Search:**
- Ctrl+F — Find in file
- Ctrl+H — Find and Replace
- Ctrl+Shift+F — Search across all files

**View:**
- Ctrl+B — Toggle left sidebar
- Ctrl+J — Toggle bottom panel
- Ctrl+\\ — Split editor
- Ctrl+= / Ctrl+- — Zoom in/out

**Code:**
- Shift+Alt+F — Format document
- Ctrl+Space — Trigger IntelliSense suggestions
- Ctrl+Shift+Space — Trigger parameter hints

Note: On Mac, use Cmd instead of Ctrl.`,
      },
    ],
  },

  // ========================================================================
  // 17. TROUBLESHOOTING
  // ========================================================================
  {
    id: 'troubleshooting',
    title: 'Troubleshooting',
    icon: <Bug className="w-5 h-5" />,
    description: 'Common issues and solutions.',
    articles: [
      {
        id: 'common-issues',
        title: 'Common issues and fixes',
        tags: ['error', 'problem', 'fix', 'issue', 'not working', 'bug', 'troubleshoot'],
        content: `**"No Project Selected" when opening IDE**
You need to create or open a project first. Go to the Build App tab and either:
- Build a new app
- Open a build from your Build History
The IDE requires an active project to display.

**Build shows 0 files**
This can happen if the WebSocket connection dropped during the build. Try:
1. Refresh the page
2. Check your Build History — the files may be saved there
3. Rebuild the app

**AI Assistant not responding**
- Check that you have available credits or a BYOK key configured
- Try switching to a different AI provider
- If using Ollama, make sure the server is running

**Preview not loading**
- Click the Stop button, wait a moment, then click Play again
- Check the terminal for error messages (the dev server may have crashed)
- Make sure your project has a valid entry point (index.html, package.json with start script, etc.)

**Terminal not working**
- Try closing and reopening the terminal panel
- Refresh the page if commands aren't executing

**Login issues**
- Make sure your password is at least 8 characters
- Check that you're using the correct username (not email) to log in
- Clear your browser's localStorage if your session seems stuck

**BYOK key validation failing**
- Double-check that you copied the full API key (no extra spaces)
- Verify the key is active on the provider's dashboard
- For Ollama, make sure the server URL is accessible from your browser`,
      },
      {
        id: 'performance',
        title: 'Performance tips',
        tags: ['slow', 'performance', 'speed', 'optimize', 'lag'],
        content: `**If the app feels slow:**
- Close unused editor tabs (many open files use memory)
- Collapse IDE panels you're not using
- Use Fast Build mode for quick prototypes (Full Build takes longer but produces better results)
- For large projects, the file explorer may take a moment to load — this is normal

**If builds are slow:**
- Fast & Cheap power mode gives the quickest responses
- Fast Build mode uses fewer agents (faster than Full Build)
- Make sure your description is clear — vague descriptions cause the AI to spend more time reasoning

**Browser recommendations:**
- Chrome or Edge perform best
- Firefox is fully supported
- Safari works but may be slightly slower
- Keep your browser updated to the latest version`,
      },
    ],
  },
]

// ============================================================================
// HELP CENTER COMPONENT
// ============================================================================

interface HelpCenterProps {
  isOpen: boolean
  onClose: () => void
}

export const HelpCenter: React.FC<HelpCenterProps> = ({ isOpen, onClose }) => {
  const [searchQuery, setSearchQuery] = useState('')
  const [activeSection, setActiveSection] = useState<string | null>(null)
  const [activeArticle, setActiveArticle] = useState<string | null>(null)
  const [expandedSections, setExpandedSections] = useState<Set<string>>(new Set())

  // Search filtering
  const filteredSections = useMemo(() => {
    if (!searchQuery.trim()) return helpSections

    const query = searchQuery.toLowerCase()
    return helpSections
      .map((section) => ({
        ...section,
        articles: section.articles.filter(
          (article) =>
            article.title.toLowerCase().includes(query) ||
            article.content.toLowerCase().includes(query) ||
            article.tags.some((tag) => tag.includes(query)) ||
            section.title.toLowerCase().includes(query)
        ),
      }))
      .filter((section) => section.articles.length > 0)
  }, [searchQuery])

  const toggleSection = (sectionId: string) => {
    setExpandedSections((prev) => {
      const next = new Set(prev)
      if (next.has(sectionId)) {
        next.delete(sectionId)
      } else {
        next.add(sectionId)
      }
      return next
    })
    setActiveArticle(null)
  }

  const openArticle = (sectionId: string, articleId: string) => {
    setActiveSection(sectionId)
    setActiveArticle(articleId)
  }

  const goBack = () => {
    setActiveArticle(null)
    setActiveSection(null)
  }

  // Find the active article content
  const currentArticle = activeSection && activeArticle
    ? helpSections
        .find((s) => s.id === activeSection)
        ?.articles.find((a) => a.id === activeArticle)
    : null

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-[9999] flex items-center justify-center p-4">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/80 backdrop-blur-sm" onClick={onClose} />

      {/* Modal */}
      <div className="relative w-full max-w-3xl max-h-[85vh] rounded-2xl overflow-hidden bg-gray-950 border border-gray-800 shadow-2xl shadow-red-900/10 flex flex-col">
        {/* Header */}
        <div className="shrink-0 border-b border-gray-800 p-5 bg-gradient-to-r from-gray-950 to-gray-900">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              {currentArticle ? (
                <button
                  onClick={goBack}
                  className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800 transition-colors"
                >
                  <ChevronRight className="w-5 h-5 rotate-180" />
                </button>
              ) : (
                <div className="p-2 rounded-xl bg-red-500/10 border border-red-500/30">
                  <BookOpen className="w-5 h-5 text-red-400" />
                </div>
              )}
              <div>
                <h2 className="text-lg font-bold text-white">
                  {currentArticle ? currentArticle.title : 'Help Center'}
                </h2>
                {!currentArticle && (
                  <p className="text-xs text-gray-500 mt-0.5">
                    Everything you need to know about APEX.BUILD
                  </p>
                )}
              </div>
            </div>
            <button
              onClick={onClose}
              className="p-2 rounded-lg text-gray-500 hover:text-white hover:bg-gray-800 transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
          </div>

          {/* Search (only in index view) */}
          {!currentArticle && (
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
              <input
                type="text"
                placeholder="Search help articles..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="w-full pl-10 pr-4 py-2.5 bg-gray-900 border border-gray-800 rounded-xl text-sm text-white placeholder-gray-600 focus:outline-none focus:border-red-500/50 focus:ring-1 focus:ring-red-500/20 transition-colors"
                autoFocus
              />
              {searchQuery && (
                <button
                  onClick={() => setSearchQuery('')}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-white"
                >
                  <X className="w-4 h-4" />
                </button>
              )}
            </div>
          )}
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto">
          {currentArticle ? (
            // Article view
            <div className="p-6">
              <ArticleRenderer content={currentArticle.content} />
            </div>
          ) : (
            // Section index view
            <div className="p-4 space-y-1.5">
              {filteredSections.length === 0 ? (
                <div className="text-center py-12">
                  <Search className="w-10 h-10 text-gray-700 mx-auto mb-3" />
                  <p className="text-gray-500 text-sm">No results found for "{searchQuery}"</p>
                  <p className="text-gray-600 text-xs mt-1">Try different keywords</p>
                </div>
              ) : (
                filteredSections.map((section) => (
                  <div key={section.id} className="rounded-xl overflow-hidden">
                    {/* Section header */}
                    <button
                      onClick={() => toggleSection(section.id)}
                      className={cn(
                        'w-full flex items-center gap-3 p-3.5 text-left transition-colors rounded-xl',
                        expandedSections.has(section.id)
                          ? 'bg-gray-900/80 border border-gray-800'
                          : 'hover:bg-gray-900/50'
                      )}
                    >
                      <div className="p-2 rounded-lg bg-gray-800/80 text-gray-400 shrink-0">
                        {section.icon}
                      </div>
                      <div className="flex-1 min-w-0">
                        <span className="text-sm font-semibold text-white block">
                          {section.title}
                        </span>
                        <span className="text-xs text-gray-500">{section.description}</span>
                      </div>
                      <div className="flex items-center gap-2 shrink-0">
                        <span className="text-xs text-gray-600">
                          {section.articles.length} {section.articles.length === 1 ? 'article' : 'articles'}
                        </span>
                        <ChevronDown
                          className={cn(
                            'w-4 h-4 text-gray-600 transition-transform duration-200',
                            expandedSections.has(section.id) && 'rotate-180'
                          )}
                        />
                      </div>
                    </button>

                    {/* Articles list */}
                    {expandedSections.has(section.id) && (
                      <div className="ml-12 mt-1 space-y-0.5 pb-2">
                        {section.articles.map((article) => (
                          <button
                            key={article.id}
                            onClick={() => openArticle(section.id, article.id)}
                            className="w-full flex items-center gap-2 px-3 py-2 text-left text-sm text-gray-400 hover:text-white hover:bg-gray-800/50 rounded-lg transition-colors group"
                          >
                            <ArrowRight className="w-3.5 h-3.5 text-gray-700 group-hover:text-red-400 transition-colors shrink-0" />
                            <span className="truncate">{article.title}</span>
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="shrink-0 border-t border-gray-800 px-5 py-3 bg-gray-950/80 flex items-center justify-between">
          <span className="text-xs text-gray-600">
            {helpSections.reduce((acc, s) => acc + s.articles.length, 0)} articles across{' '}
            {helpSections.length} topics
          </span>
          <span className="text-xs text-gray-600">APEX.BUILD v1.0</span>
        </div>
      </div>
    </div>
  )
}

// ============================================================================
// ARTICLE RENDERER - Renders markdown-like content as styled JSX
// ============================================================================

const ArticleRenderer: React.FC<{ content: string }> = ({ content }) => {
  const lines = content.split('\n')
  const elements: React.ReactNode[] = []
  let i = 0

  while (i < lines.length) {
    const line = lines[i]

    // Empty line
    if (line.trim() === '') {
      elements.push(<div key={i} className="h-3" />)
      i++
      continue
    }

    // Bold header lines (starting with **)
    if (line.trim().startsWith('**') && line.trim().endsWith('**')) {
      const text = line.trim().slice(2, -2)
      elements.push(
        <h3 key={i} className="text-white font-bold text-sm mt-4 mb-1.5">
          {text}
        </h3>
      )
      i++
      continue
    }

    // Lines starting with ** but containing more text (like **Bold:** rest)
    if (line.trim().startsWith('**') && line.includes(':**')) {
      const parts = line.trim().match(/^\*\*(.+?)\*\*\s*(.*)$/)
      if (parts) {
        elements.push(
          <p key={i} className="text-gray-300 text-sm leading-relaxed mb-1">
            <strong className="text-white">{parts[1]}</strong> {parts[2]}
          </p>
        )
        i++
        continue
      }
    }

    // Bullet points
    if (line.trim().startsWith('- ')) {
      const bulletLines: string[] = []
      while (i < lines.length && lines[i].trim().startsWith('- ')) {
        bulletLines.push(lines[i].trim().slice(2))
        i++
      }
      elements.push(
        <ul key={`ul-${i}`} className="space-y-1 mb-2 ml-1">
          {bulletLines.map((bl, idx) => (
            <li key={idx} className="flex items-start gap-2 text-sm text-gray-300 leading-relaxed">
              <span className="text-red-500 mt-1.5 shrink-0 text-[6px]">&#9679;</span>
              <span dangerouslySetInnerHTML={{ __html: formatInlineText(bl) }} />
            </li>
          ))}
        </ul>
      )
      continue
    }

    // Numbered lines (e.g., "1. something")
    if (/^\d+\.\s/.test(line.trim())) {
      const numberedLines: string[] = []
      while (i < lines.length && /^\d+\.\s/.test(lines[i].trim())) {
        numberedLines.push(lines[i].trim().replace(/^\d+\.\s/, ''))
        i++
      }
      elements.push(
        <ol key={`ol-${i}`} className="space-y-1.5 mb-2 ml-1">
          {numberedLines.map((nl, idx) => (
            <li key={idx} className="flex items-start gap-2.5 text-sm text-gray-300 leading-relaxed">
              <span className="text-red-400 font-mono text-xs mt-0.5 shrink-0 w-4 text-right">
                {idx + 1}.
              </span>
              <span dangerouslySetInnerHTML={{ __html: formatInlineText(nl) }} />
            </li>
          ))}
        </ol>
      )
      continue
    }

    // Regular paragraph
    elements.push(
      <p
        key={i}
        className="text-gray-300 text-sm leading-relaxed mb-1.5"
        dangerouslySetInnerHTML={{ __html: formatInlineText(line) }}
      />
    )
    i++
  }

  return <div className="space-y-0.5">{elements}</div>
}

// Format inline text: **bold**, `code`, "quotes"
function formatInlineText(text: string): string {
  return text
    .replace(/\*\*(.+?)\*\*/g, '<strong class="text-white font-semibold">$1</strong>')
    .replace(/`(.+?)`/g, '<code class="px-1.5 py-0.5 bg-gray-800 text-red-400 rounded text-xs font-mono">$1</code>')
    .replace(/"(.+?)"/g, '<span class="text-gray-200">"$1"</span>')
}

// ============================================================================
// HELP BUTTON (floating trigger)
// ============================================================================

export const HelpButton: React.FC = () => {
  const [isOpen, setIsOpen] = useState(false)

  return (
    <>
      <button
        onClick={() => setIsOpen(true)}
        className={cn(
          'fixed bottom-5 right-5 z-[9990]',
          'w-12 h-12 rounded-full',
          'bg-gradient-to-br from-red-600 to-red-800 shadow-lg shadow-red-900/40',
          'flex items-center justify-center',
          'hover:from-red-500 hover:to-red-700 hover:shadow-red-900/60 hover:scale-110',
          'transition-all duration-200',
          'border border-red-500/30'
        )}
        title="Help Center"
      >
        <HelpCircle className="w-6 h-6 text-white" />
      </button>
      <HelpCenter isOpen={isOpen} onClose={() => setIsOpen(false)} />
    </>
  )
}

export default HelpCenter
