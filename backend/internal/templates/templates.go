// Package templates - Project Templates and Starters
// Provides pre-built project templates for quick starts
package templates

import (
	"fmt"
	"time"
)

// TemplateCategory organizes templates by type
type TemplateCategory string

const (
	CategoryFrontend   TemplateCategory = "frontend"
	CategoryBackend    TemplateCategory = "backend"
	CategoryFullStack  TemplateCategory = "fullstack"
	CategoryAPI        TemplateCategory = "api"
	CategoryMobile     TemplateCategory = "mobile"
	CategoryGame       TemplateCategory = "game"
	CategoryData       TemplateCategory = "data"
	CategoryAutomation TemplateCategory = "automation"
)

// Template represents a project template
type Template struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Category    TemplateCategory `json:"category"`
	Language    string           `json:"language"`
	Framework   string           `json:"framework,omitempty"`
	Icon        string           `json:"icon"`
	Tags        []string         `json:"tags"`
	Difficulty  string           `json:"difficulty"` // beginner, intermediate, advanced
	Files       []TemplateFile   `json:"files"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"dev_dependencies,omitempty"`
	Scripts     map[string]string `json:"scripts,omitempty"`
	EnvVars     []EnvVar         `json:"env_vars,omitempty"`
	Popular     bool             `json:"popular"`
	New         bool             `json:"new"`
	CreatedAt   time.Time        `json:"created_at"`
}

// TemplateFile represents a file in a template
type TemplateFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	IsEntry bool   `json:"is_entry,omitempty"`
}

// EnvVar represents an environment variable needed by the template
type EnvVar struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// GetAllTemplates returns all available project templates
func GetAllTemplates() []Template {
	return []Template{
		// ============ FRONTEND ============
		{
			ID:          "react-typescript",
			Name:        "React + TypeScript",
			Description: "Modern React app with TypeScript, Vite, and Tailwind CSS",
			Category:    CategoryFrontend,
			Language:    "typescript",
			Framework:   "React",
			Icon:        "⚛️",
			Tags:        []string{"react", "typescript", "vite", "tailwind"},
			Difficulty:  "beginner",
			Popular:     true,
			Files:       getReactTypeScriptFiles(),
			Dependencies: map[string]string{
				"react":     "^18.2.0",
				"react-dom": "^18.2.0",
			},
			DevDependencies: map[string]string{
				"@types/react":     "^18.2.0",
				"@types/react-dom": "^18.2.0",
				"typescript":       "^5.0.0",
				"vite":             "^5.0.0",
				"@vitejs/plugin-react": "^4.0.0",
				"tailwindcss":      "^3.4.0",
				"autoprefixer":     "^10.4.0",
				"postcss":          "^8.4.0",
			},
			Scripts: map[string]string{
				"dev":     "vite",
				"build":   "tsc && vite build",
				"preview": "vite preview",
			},
		},
		{
			ID:          "nextjs-app",
			Name:        "Next.js 14 App Router",
			Description: "Full-featured Next.js app with App Router, Server Components, and Tailwind",
			Category:    CategoryFullStack,
			Language:    "typescript",
			Framework:   "Next.js",
			Icon:        "▲",
			Tags:        []string{"nextjs", "react", "ssr", "fullstack"},
			Difficulty:  "intermediate",
			Popular:     true,
			New:         true,
			Files:       getNextJSFiles(),
			Dependencies: map[string]string{
				"next":      "^14.0.0",
				"react":     "^18.2.0",
				"react-dom": "^18.2.0",
			},
			DevDependencies: map[string]string{
				"@types/node":      "^20.0.0",
				"@types/react":     "^18.2.0",
				"@types/react-dom": "^18.2.0",
				"typescript":       "^5.0.0",
				"tailwindcss":      "^3.4.0",
				"autoprefixer":     "^10.4.0",
				"postcss":          "^8.4.0",
			},
			Scripts: map[string]string{
				"dev":   "next dev",
				"build": "next build",
				"start": "next start",
				"lint":  "next lint",
			},
		},
		{
			ID:          "vue-vite",
			Name:        "Vue 3 + Vite",
			Description: "Vue 3 with Composition API, TypeScript, and Vite",
			Category:    CategoryFrontend,
			Language:    "typescript",
			Framework:   "Vue",
			Icon:        "💚",
			Tags:        []string{"vue", "vite", "typescript"},
			Difficulty:  "beginner",
			Files:       getVueFiles(),
			Dependencies: map[string]string{
				"vue": "^3.4.0",
			},
			DevDependencies: map[string]string{
				"@vitejs/plugin-vue": "^5.0.0",
				"typescript":         "^5.0.0",
				"vite":               "^5.0.0",
				"vue-tsc":            "^1.8.0",
			},
			Scripts: map[string]string{
				"dev":     "vite",
				"build":   "vue-tsc && vite build",
				"preview": "vite preview",
			},
		},
		{
			ID:          "vanilla-js",
			Name:        "Vanilla JavaScript",
			Description: "Pure HTML, CSS, and JavaScript - no frameworks",
			Category:    CategoryFrontend,
			Language:    "javascript",
			Icon:        "🟨",
			Tags:        []string{"html", "css", "javascript", "vanilla"},
			Difficulty:  "beginner",
			Popular:     true,
			Files:       getVanillaJSFiles(),
		},

		// ============ BACKEND ============
		{
			ID:          "express-api",
			Name:        "Express.js REST API",
			Description: "Node.js REST API with Express, validation, and error handling",
			Category:    CategoryAPI,
			Language:    "javascript",
			Framework:   "Express",
			Icon:        "🚀",
			Tags:        []string{"node", "express", "api", "rest"},
			Difficulty:  "beginner",
			Popular:     true,
			Files:       getExpressAPIFiles(),
			Dependencies: map[string]string{
				"express":    "^4.18.0",
				"cors":       "^2.8.0",
				"dotenv":     "^16.0.0",
				"helmet":     "^7.0.0",
				"morgan":     "^1.10.0",
			},
			DevDependencies: map[string]string{
				"nodemon": "^3.0.0",
			},
			Scripts: map[string]string{
				"start": "node server.js",
				"dev":   "nodemon server.js",
			},
			EnvVars: []EnvVar{
				{Name: "PORT", Description: "Server port", Default: "3000"},
				{Name: "NODE_ENV", Description: "Environment", Default: "development"},
			},
		},
		{
			ID:          "fastapi-python",
			Name:        "FastAPI Python",
			Description: "Modern Python API with FastAPI, async support, and auto-docs",
			Category:    CategoryAPI,
			Language:    "python",
			Framework:   "FastAPI",
			Icon:        "🐍",
			Tags:        []string{"python", "fastapi", "api", "async"},
			Difficulty:  "intermediate",
			Popular:     true,
			Files:       getFastAPIFiles(),
			EnvVars: []EnvVar{
				{Name: "DATABASE_URL", Description: "Database connection string", Required: true},
			},
		},
		{
			ID:          "go-fiber",
			Name:        "Go Fiber API",
			Description: "High-performance Go API with Fiber framework",
			Category:    CategoryAPI,
			Language:    "go",
			Framework:   "Fiber",
			Icon:        "🔵",
			Tags:        []string{"go", "fiber", "api", "performance"},
			Difficulty:  "intermediate",
			Files:       getGoFiberFiles(),
		},

		// ============ FULLSTACK ============
		{
			ID:          "t3-stack",
			Name:        "T3 Stack",
			Description: "Next.js + tRPC + Prisma + Tailwind - type-safe fullstack",
			Category:    CategoryFullStack,
			Language:    "typescript",
			Framework:   "T3",
			Icon:        "🔺",
			Tags:        []string{"t3", "trpc", "prisma", "nextjs", "fullstack"},
			Difficulty:  "advanced",
			New:         true,
			Files:       getT3StackFiles(),
			Dependencies: map[string]string{
				"next":          "^14.0.0",
				"react":         "^18.2.0",
				"@trpc/server":  "^10.0.0",
				"@trpc/client":  "^10.0.0",
				"@prisma/client": "^5.0.0",
				"zod":           "^3.22.0",
			},
		},
		{
			ID:          "mern-stack",
			Name:        "MERN Stack",
			Description: "MongoDB + Express + React + Node.js fullstack app",
			Category:    CategoryFullStack,
			Language:    "javascript",
			Framework:   "MERN",
			Icon:        "🍃",
			Tags:        []string{"mongodb", "express", "react", "node", "fullstack"},
			Difficulty:  "intermediate",
			Popular:     true,
			Files:       getMERNStackFiles(),
			EnvVars: []EnvVar{
				{Name: "MONGODB_URI", Description: "MongoDB connection string", Required: true},
				{Name: "JWT_SECRET", Description: "JWT signing secret", Required: true},
			},
		},

		// ============ DATA & ML ============
		{
			ID:          "python-data",
			Name:        "Python Data Science",
			Description: "Jupyter-style Python with pandas, numpy, and matplotlib",
			Category:    CategoryData,
			Language:    "python",
			Icon:        "📊",
			Tags:        []string{"python", "data", "pandas", "jupyter"},
			Difficulty:  "beginner",
			Files:       getPythonDataFiles(),
		},

		// ============ AUTOMATION ============
		{
			ID:          "discord-bot",
			Name:        "Discord Bot",
			Description: "Discord bot with slash commands and event handling",
			Category:    CategoryAutomation,
			Language:    "javascript",
			Framework:   "discord.js",
			Icon:        "🤖",
			Tags:        []string{"discord", "bot", "automation"},
			Difficulty:  "intermediate",
			Files:       getDiscordBotFiles(),
			Dependencies: map[string]string{
				"discord.js": "^14.0.0",
				"dotenv":     "^16.0.0",
			},
			EnvVars: []EnvVar{
				{Name: "DISCORD_TOKEN", Description: "Discord bot token", Required: true},
				{Name: "CLIENT_ID", Description: "Discord application client ID", Required: true},
			},
		},
		{
			ID:          "cli-tool",
			Name:        "Node.js CLI Tool",
			Description: "Command-line tool with argument parsing and colors",
			Category:    CategoryAutomation,
			Language:    "javascript",
			Icon:        "💻",
			Tags:        []string{"cli", "node", "terminal"},
			Difficulty:  "beginner",
			Files:       getCLIToolFiles(),
			Dependencies: map[string]string{
				"commander": "^11.0.0",
				"chalk":     "^5.3.0",
				"ora":       "^7.0.0",
			},
		},

		// ============ GAME ============
		{
			ID:          "phaser-game",
			Name:        "Phaser 3 Game",
			Description: "2D game with Phaser 3 and TypeScript",
			Category:    CategoryGame,
			Language:    "typescript",
			Framework:   "Phaser",
			Icon:        "🎮",
			Tags:        []string{"game", "phaser", "2d", "typescript"},
			Difficulty:  "intermediate",
			Files:       getPhaserGameFiles(),
			Dependencies: map[string]string{
				"phaser": "^3.70.0",
			},
			DevDependencies: map[string]string{
				"typescript": "^5.0.0",
				"vite":       "^5.0.0",
			},
		},

		// ============ BLANK ============
		{
			ID:          "blank",
			Name:        "Blank Project",
			Description: "Empty project - start from scratch",
			Category:    CategoryFrontend,
			Language:    "javascript",
			Icon:        "📄",
			Tags:        []string{"blank", "empty", "scratch"},
			Difficulty:  "beginner",
			Files: []TemplateFile{
				{Path: "index.js", Content: "// Start coding here\nconsole.log('Hello, APEX.BUILD!');\n", IsEntry: true},
				{Path: "README.md", Content: "# My Project\n\nBuilt with APEX.BUILD\n"},
			},
		},
	}
}

// GetTemplateByID returns a specific template by ID
func GetTemplateByID(id string) (*Template, error) {
	templates := GetAllTemplates()
	for _, t := range templates {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("template not found: %s", id)
}

// GetTemplatesByCategory returns templates in a specific category
func GetTemplatesByCategory(category TemplateCategory) []Template {
	templates := GetAllTemplates()
	var result []Template
	for _, t := range templates {
		if t.Category == category {
			result = append(result, t)
		}
	}
	return result
}

// GetPopularTemplates returns the most popular templates
func GetPopularTemplates() []Template {
	templates := GetAllTemplates()
	var result []Template
	for _, t := range templates {
		if t.Popular {
			result = append(result, t)
		}
	}
	return result
}

// ============ FILE CONTENT GENERATORS ============

func getReactTypeScriptFiles() []TemplateFile {
	return []TemplateFile{
		{
			Path: "src/main.tsx",
			Content: `import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
`,
			IsEntry: true,
		},
		{
			Path: "src/App.tsx",
			Content: `import { useState } from 'react'

function App() {
  const [count, setCount] = useState(0)

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-900 via-purple-900 to-gray-900 flex items-center justify-center">
      <div className="text-center">
        <h1 className="text-5xl font-bold text-white mb-8">
          Welcome to <span className="text-cyan-400">APEX.BUILD</span>
        </h1>
        <div className="bg-gray-800/50 backdrop-blur-sm rounded-2xl p-8 border border-gray-700">
          <button
            onClick={() => setCount(count + 1)}
            className="px-6 py-3 bg-cyan-600 hover:bg-cyan-500 text-white font-semibold rounded-lg transition-colors"
          >
            Count: {count}
          </button>
          <p className="mt-4 text-gray-400">
            Edit <code className="text-cyan-400">src/App.tsx</code> and save to reload.
          </p>
        </div>
      </div>
    </div>
  )
}

export default App
`,
		},
		{
			Path: "src/index.css",
			Content: `@tailwind base;
@tailwind components;
@tailwind utilities;
`,
		},
		{
			Path: "index.html",
			Content: `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/vite.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>React + TypeScript | APEX.BUILD</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
`,
		},
		{
			Path: "vite.config.ts",
			Content: `import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
})
`,
		},
		{
			Path: "tailwind.config.js",
			Content: `/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {},
  },
  plugins: [],
}
`,
		},
		{
			Path: "tsconfig.json",
			Content: `{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
`,
		},
	}
}

func getNextJSFiles() []TemplateFile {
	return []TemplateFile{
		{
			Path: "app/page.tsx",
			Content: `export default function Home() {
  return (
    <main className="min-h-screen bg-gradient-to-br from-gray-900 via-purple-900 to-gray-900 flex items-center justify-center p-8">
      <div className="text-center max-w-2xl">
        <h1 className="text-6xl font-bold text-white mb-6">
          Welcome to <span className="text-cyan-400">Next.js 14</span>
        </h1>
        <p className="text-xl text-gray-300 mb-8">
          Built with App Router, Server Components, and Tailwind CSS on APEX.BUILD
        </p>
        <div className="flex gap-4 justify-center">
          <a
            href="https://nextjs.org/docs"
            className="px-6 py-3 bg-white text-gray-900 font-semibold rounded-lg hover:bg-gray-100 transition-colors"
          >
            Read Docs
          </a>
          <a
            href="https://nextjs.org/learn"
            className="px-6 py-3 bg-cyan-600 text-white font-semibold rounded-lg hover:bg-cyan-500 transition-colors"
          >
            Learn Next.js
          </a>
        </div>
      </div>
    </main>
  )
}
`,
			IsEntry: true,
		},
		{
			Path: "app/layout.tsx",
			Content: `import type { Metadata } from 'next'
import { Inter } from 'next/font/google'
import './globals.css'

const inter = Inter({ subsets: ['latin'] })

export const metadata: Metadata = {
  title: 'Next.js App | APEX.BUILD',
  description: 'Built with Next.js 14 on APEX.BUILD',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body className={inter.className}>{children}</body>
    </html>
  )
}
`,
		},
		{
			Path: "app/globals.css",
			Content: `@tailwind base;
@tailwind components;
@tailwind utilities;
`,
		},
		{
			Path: "next.config.js",
			Content: `/** @type {import('next').NextConfig} */
const nextConfig = {}

module.exports = nextConfig
`,
		},
		{
			Path: "tailwind.config.ts",
			Content: `import type { Config } from 'tailwindcss'

const config: Config = {
  content: [
    './pages/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
    './app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {},
  },
  plugins: [],
}
export default config
`,
		},
	}
}

func getVueFiles() []TemplateFile {
	return []TemplateFile{
		{
			Path: "src/App.vue",
			Content: `<script setup lang="ts">
import { ref } from 'vue'

const count = ref(0)
</script>

<template>
  <div class="min-h-screen bg-gradient-to-br from-gray-900 via-green-900 to-gray-900 flex items-center justify-center">
    <div class="text-center">
      <h1 class="text-5xl font-bold text-white mb-8">
        Welcome to <span class="text-green-400">Vue 3</span>
      </h1>
      <div class="bg-gray-800/50 backdrop-blur-sm rounded-2xl p-8 border border-gray-700">
        <button
          @click="count++"
          class="px-6 py-3 bg-green-600 hover:bg-green-500 text-white font-semibold rounded-lg transition-colors"
        >
          Count: {{ count }}
        </button>
        <p class="mt-4 text-gray-400">
          Edit <code class="text-green-400">src/App.vue</code> and save to reload.
        </p>
      </div>
    </div>
  </div>
</template>
`,
			IsEntry: true,
		},
		{
			Path: "src/main.ts",
			Content: `import { createApp } from 'vue'
import App from './App.vue'

createApp(App).mount('#app')
`,
		},
		{
			Path: "index.html",
			Content: `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Vue 3 | APEX.BUILD</title>
    <script src="https://cdn.tailwindcss.com"></script>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
`,
		},
		{
			Path: "vite.config.ts",
			Content: `import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
})
`,
		},
	}
}

func getVanillaJSFiles() []TemplateFile {
	return []TemplateFile{
		{
			Path: "index.html",
			Content: `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Vanilla JS | APEX.BUILD</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <div class="container">
        <h1>Welcome to <span class="highlight">APEX.BUILD</span></h1>
        <p>Pure HTML, CSS, and JavaScript - no frameworks needed.</p>
        <button id="counter-btn">Count: 0</button>
    </div>
    <script src="script.js"></script>
</body>
</html>
`,
			IsEntry: true,
		},
		{
			Path: "style.css",
			Content: `* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

body {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #1a1a2e 100%);
    font-family: 'Segoe UI', system-ui, sans-serif;
    color: white;
}

.container {
    text-align: center;
    padding: 2rem;
}

h1 {
    font-size: 3rem;
    margin-bottom: 1rem;
}

.highlight {
    color: #00d4ff;
}

p {
    color: #888;
    margin-bottom: 2rem;
}

button {
    padding: 1rem 2rem;
    font-size: 1.2rem;
    background: #00d4ff;
    color: #1a1a2e;
    border: none;
    border-radius: 8px;
    cursor: pointer;
    font-weight: 600;
    transition: transform 0.2s, background 0.2s;
}

button:hover {
    background: #00b8e6;
    transform: scale(1.05);
}
`,
		},
		{
			Path: "script.js",
			Content: `// Simple counter example
let count = 0;
const button = document.getElementById('counter-btn');

button.addEventListener('click', () => {
    count++;
    button.textContent = 'Count: ' + count;
});

console.log('Hello from APEX.BUILD!');
`,
		},
	}
}

func getExpressAPIFiles() []TemplateFile {
	return []TemplateFile{
		{
			Path: "server.js",
			Content: `const express = require('express');
const cors = require('cors');
const helmet = require('helmet');
const morgan = require('morgan');
require('dotenv').config();

const app = express();
const PORT = process.env.PORT || 3000;

// Middleware
app.use(helmet());
app.use(cors());
app.use(morgan('dev'));
app.use(express.json());

// In-memory data store (replace with database in production)
let items = [
  { id: 1, name: 'Item 1', description: 'First item' },
  { id: 2, name: 'Item 2', description: 'Second item' },
];

// Routes
app.get('/', (req, res) => {
  res.json({
    message: 'Welcome to the API',
    version: '1.0.0',
    endpoints: {
      'GET /items': 'List all items',
      'GET /items/:id': 'Get item by ID',
      'POST /items': 'Create new item',
      'PUT /items/:id': 'Update item',
      'DELETE /items/:id': 'Delete item',
    },
  });
});

// GET all items
app.get('/items', (req, res) => {
  res.json(items);
});

// GET item by ID
app.get('/items/:id', (req, res) => {
  const item = items.find(i => i.id === parseInt(req.params.id));
  if (!item) {
    return res.status(404).json({ error: 'Item not found' });
  }
  res.json(item);
});

// POST new item
app.post('/items', (req, res) => {
  const { name, description } = req.body;
  if (!name) {
    return res.status(400).json({ error: 'Name is required' });
  }
  const newItem = {
    id: items.length + 1,
    name,
    description: description || '',
  };
  items.push(newItem);
  res.status(201).json(newItem);
});

// PUT update item
app.put('/items/:id', (req, res) => {
  const item = items.find(i => i.id === parseInt(req.params.id));
  if (!item) {
    return res.status(404).json({ error: 'Item not found' });
  }
  const { name, description } = req.body;
  if (name) item.name = name;
  if (description) item.description = description;
  res.json(item);
});

// DELETE item
app.delete('/items/:id', (req, res) => {
  const index = items.findIndex(i => i.id === parseInt(req.params.id));
  if (index === -1) {
    return res.status(404).json({ error: 'Item not found' });
  }
  items.splice(index, 1);
  res.status(204).send();
});

// Error handling
app.use((err, req, res, next) => {
  console.error(err.stack);
  res.status(500).json({ error: 'Something went wrong!' });
});

// Start server
app.listen(PORT, () => {
  console.log('🚀 Server running on http://localhost:' + PORT);
});
`,
			IsEntry: true,
		},
		{
			Path: ".env.example",
			Content: `PORT=3000
NODE_ENV=development
`,
		},
	}
}

func getFastAPIFiles() []TemplateFile {
	return []TemplateFile{
		{
			Path: "main.py",
			Content: `from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from typing import Optional
import uvicorn

app = FastAPI(
    title="FastAPI App",
    description="Built with APEX.BUILD",
    version="1.0.0"
)

# CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Models
class Item(BaseModel):
    name: str
    description: Optional[str] = None
    price: float

class ItemUpdate(BaseModel):
    name: Optional[str] = None
    description: Optional[str] = None
    price: Optional[float] = None

# In-memory store
items_db = {
    1: {"id": 1, "name": "Item 1", "description": "First item", "price": 9.99},
    2: {"id": 2, "name": "Item 2", "description": "Second item", "price": 19.99},
}
next_id = 3

@app.get("/")
async def root():
    return {
        "message": "Welcome to FastAPI",
        "docs": "/docs",
        "redoc": "/redoc"
    }

@app.get("/items")
async def get_items():
    return list(items_db.values())

@app.get("/items/{item_id}")
async def get_item(item_id: int):
    if item_id not in items_db:
        raise HTTPException(status_code=404, detail="Item not found")
    return items_db[item_id]

@app.post("/items", status_code=201)
async def create_item(item: Item):
    global next_id
    new_item = {"id": next_id, **item.dict()}
    items_db[next_id] = new_item
    next_id += 1
    return new_item

@app.put("/items/{item_id}")
async def update_item(item_id: int, item: ItemUpdate):
    if item_id not in items_db:
        raise HTTPException(status_code=404, detail="Item not found")

    stored_item = items_db[item_id]
    update_data = item.dict(exclude_unset=True)
    updated_item = {**stored_item, **update_data}
    items_db[item_id] = updated_item
    return updated_item

@app.delete("/items/{item_id}", status_code=204)
async def delete_item(item_id: int):
    if item_id not in items_db:
        raise HTTPException(status_code=404, detail="Item not found")
    del items_db[item_id]

if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0", port=8000, reload=True)
`,
			IsEntry: true,
		},
		{
			Path: "requirements.txt",
			Content: `fastapi>=0.100.0
uvicorn[standard]>=0.23.0
pydantic>=2.0.0
python-dotenv>=1.0.0
`,
		},
	}
}

func getGoFiberFiles() []TemplateFile {
	return []TemplateFile{
		{
			Path: "main.go",
			Content: `package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

type Item struct {
	ID          int    ` + "`json:\"id\"`" + `
	Name        string ` + "`json:\"name\"`" + `
	Description string ` + "`json:\"description\"`" + `
}

var items = []Item{
	{ID: 1, Name: "Item 1", Description: "First item"},
	{ID: 2, Name: "Item 2", Description: "Second item"},
}
var nextID = 3

func main() {
	app := fiber.New(fiber.Config{
		AppName: "Go Fiber API",
	})

	// Middleware
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New())

	// Routes
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Welcome to Go Fiber API",
			"version": "1.0.0",
		})
	})

	app.Get("/items", getItems)
	app.Get("/items/:id", getItem)
	app.Post("/items", createItem)
	app.Put("/items/:id", updateItem)
	app.Delete("/items/:id", deleteItem)

	log.Fatal(app.Listen(":3000"))
}

func getItems(c *fiber.Ctx) error {
	return c.JSON(items)
}

func getItem(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	for _, item := range items {
		if item.ID == id {
			return c.JSON(item)
		}
	}
	return c.Status(404).JSON(fiber.Map{"error": "Item not found"})
}

func createItem(c *fiber.Ctx) error {
	item := new(Item)
	if err := c.BodyParser(item); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	item.ID = nextID
	nextID++
	items = append(items, *item)
	return c.Status(201).JSON(item)
}

func updateItem(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	for i, item := range items {
		if item.ID == id {
			if err := c.BodyParser(&items[i]); err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
			}
			items[i].ID = id
			return c.JSON(items[i])
		}
	}
	return c.Status(404).JSON(fiber.Map{"error": "Item not found"})
}

func deleteItem(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	for i, item := range items {
		if item.ID == id {
			items = append(items[:i], items[i+1:]...)
			return c.SendStatus(204)
		}
	}
	return c.Status(404).JSON(fiber.Map{"error": "Item not found"})
}
`,
			IsEntry: true,
		},
		{
			Path: "go.mod",
			Content: `module myapi

go 1.21

require github.com/gofiber/fiber/v2 v2.52.0
`,
		},
	}
}

func getT3StackFiles() []TemplateFile {
	return []TemplateFile{
		{
			Path: "src/app/page.tsx",
			Content: `export default function Home() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-center bg-gradient-to-b from-[#2e026d] to-[#15162c]">
      <div className="container flex flex-col items-center justify-center gap-12 px-4 py-16">
        <h1 className="text-5xl font-extrabold tracking-tight text-white sm:text-[5rem]">
          Create <span className="text-[hsl(280,100%,70%)]">T3</span> App
        </h1>
        <p className="text-2xl text-white">
          The best stack for your next project
        </p>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 md:gap-8">
          <div className="flex max-w-xs flex-col gap-4 rounded-xl bg-white/10 p-4 text-white hover:bg-white/20">
            <h3 className="text-2xl font-bold">tRPC →</h3>
            <p className="text-lg">End-to-end typesafe APIs made easy.</p>
          </div>
          <div className="flex max-w-xs flex-col gap-4 rounded-xl bg-white/10 p-4 text-white hover:bg-white/20">
            <h3 className="text-2xl font-bold">Prisma →</h3>
            <p className="text-lg">Type-safe database access.</p>
          </div>
        </div>
      </div>
    </main>
  );
}
`,
			IsEntry: true,
		},
	}
}

func getMERNStackFiles() []TemplateFile {
	return []TemplateFile{
		{
			Path: "server/index.js",
			Content: `const express = require('express');
const mongoose = require('mongoose');
const cors = require('cors');
require('dotenv').config();

const app = express();
app.use(cors());
app.use(express.json());

// Connect to MongoDB
mongoose.connect(process.env.MONGODB_URI)
  .then(() => console.log('Connected to MongoDB'))
  .catch(err => console.error('MongoDB connection error:', err));

// Item Schema
const itemSchema = new mongoose.Schema({
  name: { type: String, required: true },
  description: String,
  createdAt: { type: Date, default: Date.now }
});

const Item = mongoose.model('Item', itemSchema);

// Routes
app.get('/api/items', async (req, res) => {
  const items = await Item.find().sort({ createdAt: -1 });
  res.json(items);
});

app.post('/api/items', async (req, res) => {
  const item = new Item(req.body);
  await item.save();
  res.status(201).json(item);
});

app.delete('/api/items/:id', async (req, res) => {
  await Item.findByIdAndDelete(req.params.id);
  res.status(204).send();
});

const PORT = process.env.PORT || 5000;
app.listen(PORT, () => console.log('Server running on port ' + PORT));
`,
			IsEntry: true,
		},
		{
			Path: "client/src/App.jsx",
			Content: `import { useState, useEffect } from 'react';

function App() {
  const [items, setItems] = useState([]);
  const [name, setName] = useState('');

  useEffect(() => {
    fetch('/api/items')
      .then(res => res.json())
      .then(setItems);
  }, []);

  const addItem = async (e) => {
    e.preventDefault();
    const res = await fetch('/api/items', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name })
    });
    const item = await res.json();
    setItems([item, ...items]);
    setName('');
  };

  return (
    <div className="min-h-screen bg-gray-900 text-white p-8">
      <h1 className="text-4xl font-bold mb-8">MERN Stack App</h1>
      <form onSubmit={addItem} className="mb-8">
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Add item..."
          className="px-4 py-2 bg-gray-800 rounded mr-2"
        />
        <button className="px-4 py-2 bg-green-600 rounded">Add</button>
      </form>
      <ul className="space-y-2">
        {items.map(item => (
          <li key={item._id} className="bg-gray-800 p-4 rounded">
            {item.name}
          </li>
        ))}
      </ul>
    </div>
  );
}

export default App;
`,
		},
	}
}

func getPythonDataFiles() []TemplateFile {
	return []TemplateFile{
		{
			Path: "main.py",
			Content: `# Python Data Science Starter
import pandas as pd
import numpy as np
import matplotlib.pyplot as plt

# Create sample data
np.random.seed(42)
data = {
    'date': pd.date_range('2024-01-01', periods=100),
    'sales': np.random.randint(100, 1000, 100),
    'customers': np.random.randint(10, 100, 100),
}
df = pd.DataFrame(data)

# Basic analysis
print("Dataset Info:")
print(df.describe())

print("\nCorrelation:")
print(df[['sales', 'customers']].corr())

# Plot
plt.figure(figsize=(10, 6))
plt.plot(df['date'], df['sales'], label='Sales')
plt.xlabel('Date')
plt.ylabel('Sales')
plt.title('Sales Over Time')
plt.legend()
plt.savefig('sales_chart.png')
print("\nChart saved as sales_chart.png")
`,
			IsEntry: true,
		},
		{
			Path: "requirements.txt",
			Content: `pandas>=2.0.0
numpy>=1.24.0
matplotlib>=3.7.0
seaborn>=0.12.0
jupyter>=1.0.0
`,
		},
	}
}

func getDiscordBotFiles() []TemplateFile {
	return []TemplateFile{
		{
			Path: "index.js",
			Content: `const { Client, GatewayIntentBits, SlashCommandBuilder, REST, Routes } = require('discord.js');
require('dotenv').config();

const client = new Client({
  intents: [
    GatewayIntentBits.Guilds,
    GatewayIntentBits.GuildMessages,
  ]
});

// Commands
const commands = [
  new SlashCommandBuilder()
    .setName('ping')
    .setDescription('Check bot latency'),
  new SlashCommandBuilder()
    .setName('hello')
    .setDescription('Get a friendly greeting'),
].map(cmd => cmd.toJSON());

// Register commands
const rest = new REST({ version: '10' }).setToken(process.env.DISCORD_TOKEN);

(async () => {
  try {
    console.log('Registering slash commands...');
    await rest.put(
      Routes.applicationCommands(process.env.CLIENT_ID),
      { body: commands }
    );
    console.log('Commands registered!');
  } catch (error) {
    console.error(error);
  }
})();

// Event handlers
client.once('ready', () => {
  console.log('Bot is online! Logged in as ' + client.user.tag);
});

client.on('interactionCreate', async interaction => {
  if (!interaction.isChatInputCommand()) return;

  if (interaction.commandName === 'ping') {
    const latency = Date.now() - interaction.createdTimestamp;
    await interaction.reply('Pong! Latency: ' + latency + 'ms');
  }

  if (interaction.commandName === 'hello') {
    await interaction.reply('Hello! 👋 Built with APEX.BUILD');
  }
});

client.login(process.env.DISCORD_TOKEN);
`,
			IsEntry: true,
		},
		{
			Path: ".env.example",
			Content: `DISCORD_TOKEN=your_bot_token_here
CLIENT_ID=your_client_id_here
`,
		},
	}
}

func getCLIToolFiles() []TemplateFile {
	return []TemplateFile{
		{
			Path: "index.js",
			Content: `#!/usr/bin/env node
const { program } = require('commander');
const chalk = require('chalk');
const ora = require('ora');

program
  .name('mycli')
  .description('CLI tool built with APEX.BUILD')
  .version('1.0.0');

program
  .command('greet <name>')
  .description('Greet someone')
  .option('-e, --excited', 'Add excitement')
  .action((name, options) => {
    const greeting = options.excited
      ? chalk.green.bold('Hello, ' + name + '! 🎉')
      : chalk.blue('Hello, ' + name);
    console.log(greeting);
  });

program
  .command('spin')
  .description('Show a spinner')
  .action(async () => {
    const spinner = ora('Processing...').start();
    await new Promise(r => setTimeout(r, 2000));
    spinner.succeed(chalk.green('Done!'));
  });

program.parse();
`,
			IsEntry: true,
		},
	}
}

func getPhaserGameFiles() []TemplateFile {
	return []TemplateFile{
		{
			Path: "src/main.ts",
			Content: `import Phaser from 'phaser';

class MainScene extends Phaser.Scene {
  private player!: Phaser.GameObjects.Rectangle;
  private cursors!: Phaser.Types.Input.Keyboard.CursorKeys;
  private score = 0;
  private scoreText!: Phaser.GameObjects.Text;

  constructor() {
    super('MainScene');
  }

  create() {
    // Background
    this.add.rectangle(400, 300, 800, 600, 0x1a1a2e);

    // Player
    this.player = this.add.rectangle(400, 500, 50, 50, 0x00d4ff);
    this.physics.add.existing(this.player);

    // Score
    this.scoreText = this.add.text(16, 16, 'Score: 0', {
      fontSize: '24px',
      color: '#ffffff'
    });

    // Input
    this.cursors = this.input.keyboard!.createCursorKeys();

    // Instructions
    this.add.text(400, 50, 'Use arrow keys to move', {
      fontSize: '18px',
      color: '#888888'
    }).setOrigin(0.5);
  }

  update() {
    const body = this.player.body as Phaser.Physics.Arcade.Body;
    body.setVelocity(0);

    if (this.cursors.left.isDown) body.setVelocityX(-200);
    if (this.cursors.right.isDown) body.setVelocityX(200);
    if (this.cursors.up.isDown) body.setVelocityY(-200);
    if (this.cursors.down.isDown) body.setVelocityY(200);
  }
}

const config: Phaser.Types.Core.GameConfig = {
  type: Phaser.AUTO,
  width: 800,
  height: 600,
  physics: {
    default: 'arcade',
    arcade: { debug: false }
  },
  scene: MainScene
};

new Phaser.Game(config);
`,
			IsEntry: true,
		},
		{
			Path: "index.html",
			Content: `<!DOCTYPE html>
<html>
<head>
  <title>Phaser Game | APEX.BUILD</title>
  <style>
    body { margin: 0; display: flex; justify-content: center; align-items: center; min-height: 100vh; background: #0a0a0a; }
  </style>
</head>
<body>
  <script type="module" src="/src/main.ts"></script>
</body>
</html>
`,
		},
	}
}
