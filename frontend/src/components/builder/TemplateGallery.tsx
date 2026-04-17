import React, { useState } from 'react'
import { cn } from '@/lib/utils'
import { ChevronDown, ChevronUp } from 'lucide-react'

export interface ProjectTemplate {
  id: string
  label: string
  emoji: string
  description: string
  prompt: string
  stack?: string[]
}

export const PROJECT_TEMPLATES: ProjectTemplate[] = [
  {
    id: 'saas-dashboard',
    label: 'SaaS Dashboard',
    emoji: '📊',
    description: 'Multi-tenant analytics dashboard with auth',
    prompt:
      'Build a multi-tenant SaaS analytics dashboard with JWT authentication, role-based access control (admin/member), an interactive dashboard with charts showing key metrics, a user management table, and a billing/subscription page. Use React, TypeScript, and a Node.js REST API with PostgreSQL.',
    stack: ['react', 'node', 'postgres'],
  },
  {
    id: 'ecommerce',
    label: 'E-Commerce Store',
    emoji: '🛒',
    description: 'Full product catalog, cart, and checkout',
    prompt:
      'Build a full-stack e-commerce store with a product catalog, category filtering, search, shopping cart with quantity controls, a checkout flow with Stripe payment integration, order history, and an admin panel to manage products and orders. Use React, TypeScript, and a Node.js backend with PostgreSQL.',
    stack: ['react', 'node', 'postgres'],
  },
  {
    id: 'realtime-chat',
    label: 'Real-Time Chat',
    emoji: '💬',
    description: 'WebSocket chat with rooms and auth',
    prompt:
      'Build a real-time chat application with WebSocket connections, multiple chat rooms, direct messaging, user presence indicators (online/offline), message history, emoji reactions, and JWT-based authentication. Use React, TypeScript, Node.js with Socket.io, and PostgreSQL for persistence.',
    stack: ['react', 'node'],
  },
  {
    id: 'portfolio',
    label: 'Portfolio Site',
    emoji: '🎨',
    description: 'Personal portfolio with projects and blog',
    prompt:
      'Build a personal portfolio website with a hero section, animated skills section, project showcase with filtering by technology, a blog with markdown rendering, a contact form, and dark/light mode toggle. Deploy-ready static site using React, TypeScript, and Tailwind CSS.',
    stack: ['react'],
  },
  {
    id: 'todo-project-manager',
    label: 'Project Manager',
    emoji: '✅',
    description: 'Kanban board with drag-and-drop tasks',
    prompt:
      'Build a project management app with kanban boards, drag-and-drop task cards, multiple projects, task assignment, due dates, priority labels, progress tracking, and a dashboard with charts showing task completion rates. Use React, TypeScript, and a Go REST API with PostgreSQL.',
    stack: ['react', 'go', 'postgres'],
  },
  {
    id: 'auth-starter',
    label: 'Auth Starter',
    emoji: '🔐',
    description: 'Complete auth with OAuth and email verify',
    prompt:
      'Build a production-ready authentication starter with email/password signup, email verification, Google OAuth, forgot password with reset email, JWT refresh tokens stored in httpOnly cookies, rate limiting, and a protected dashboard. Use React, TypeScript, Node.js, and PostgreSQL.',
    stack: ['react', 'node', 'postgres'],
  },
  {
    id: 'api-backend',
    label: 'REST API',
    emoji: '⚡',
    description: 'Typed REST API with docs and auth',
    prompt:
      'Build a production-ready REST API with JWT authentication, role-based authorization middleware, CRUD endpoints for users and resources, request validation, rate limiting, structured logging, OpenAPI/Swagger documentation, and a health check endpoint. Use Go with a PostgreSQL database.',
    stack: ['go', 'postgres'],
  },
  {
    id: 'ai-chatbot',
    label: 'AI Chatbot',
    emoji: '🤖',
    description: 'LLM-powered chatbot with conversation history',
    prompt:
      'Build an AI chatbot application with a conversational UI, multiple chat sessions with history, streaming responses from an OpenAI-compatible API, markdown rendering for responses, code syntax highlighting, a system prompt configurator, and user authentication. Use React, TypeScript, and a Node.js backend.',
    stack: ['react', 'node'],
  },
  {
    id: 'social-feed',
    label: 'Social Feed',
    emoji: '📱',
    description: 'Social platform with posts, likes, follows',
    prompt:
      'Build a social media feed application with user profiles, posts with images, likes, comments, follow/unfollow functionality, a personalized feed algorithm, notifications, and hashtag support. Use React, TypeScript, Node.js, and PostgreSQL with file uploads to cloud storage.',
    stack: ['react', 'node', 'postgres'],
  },
  {
    id: 'finance-tracker',
    label: 'Finance Tracker',
    emoji: '💰',
    description: 'Budget and expense tracking with charts',
    prompt:
      'Build a personal finance tracker with transaction entry, category tagging, monthly budget goals, spending trend charts, income vs expense summaries, recurring transaction detection, CSV import, and a net worth tracker. Use React, TypeScript, Node.js, and PostgreSQL.',
    stack: ['react', 'node', 'postgres'],
  },
  {
    id: 'blog-cms',
    label: 'Blog / CMS',
    emoji: '📝',
    description: 'Full CMS with markdown editor and SEO',
    prompt:
      'Build a blog CMS with a markdown editor, rich text formatting, draft/publish workflow, category and tag management, SEO-friendly URLs, sitemap generation, image uploads, author profiles, comments with moderation, and an analytics dashboard. Use React, TypeScript, and a Go backend.',
    stack: ['react', 'go', 'postgres'],
  },
  {
    id: 'admin-panel',
    label: 'Admin Panel',
    emoji: '🛠️',
    description: 'Data tables, charts, and user management',
    prompt:
      'Build an admin panel with a sidebar navigation, data tables with sorting, filtering, and pagination, user management with role assignment, an analytics dashboard with charts, activity logs, settings pages, and export to CSV/Excel. Use React, TypeScript, and a Node.js backend with PostgreSQL.',
    stack: ['react', 'node', 'postgres'],
  },
]

interface TemplateGalleryProps {
  onSelect: (prompt: string) => void
}

export default function TemplateGallery({ onSelect }: TemplateGalleryProps) {
  const [expanded, setExpanded] = useState(false)
  const visible = expanded ? PROJECT_TEMPLATES : PROJECT_TEMPLATES.slice(0, 6)

  return (
    <div className="mb-8">
      <div className="flex items-center justify-between mb-3">
        <p className="text-xs uppercase tracking-[0.18em] text-gray-500 font-semibold">
          Start from a template
        </p>
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="flex items-center gap-1 text-xs text-gray-500 hover:text-gray-300 transition-colors"
        >
          {expanded ? (
            <>
              Show less <ChevronUp className="w-3.5 h-3.5" />
            </>
          ) : (
            <>
              Show all {PROJECT_TEMPLATES.length} <ChevronDown className="w-3.5 h-3.5" />
            </>
          )}
        </button>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
        {visible.map((tpl) => (
          <button
            key={tpl.id}
            type="button"
            onClick={() => onSelect(tpl.prompt)}
            className={cn(
              'group text-left rounded-xl border border-gray-800 bg-gray-900/50 px-3 py-2.5',
              'hover:border-red-800/60 hover:bg-red-950/20 transition-all duration-200'
            )}
          >
            <div className="flex items-center gap-2 mb-0.5">
              <span className="text-base leading-none">{tpl.emoji}</span>
              <span className="text-sm font-semibold text-gray-200 group-hover:text-red-300 transition-colors">
                {tpl.label}
              </span>
            </div>
            <p className="text-[11px] text-gray-500 leading-tight">{tpl.description}</p>
          </button>
        ))}
      </div>
    </div>
  )
}
