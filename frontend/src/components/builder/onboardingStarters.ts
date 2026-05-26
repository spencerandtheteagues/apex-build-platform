import {
  LayoutTemplate,
  ListTodo,
  PanelsTopLeft,
  type LucideIcon,
} from 'lucide-react'

export interface OnboardingStarter {
  id: string
  title: string
  description: string
  prompt: string
  mode: 'fast' | 'full'
  icon: LucideIcon
}

export const onboardingStarters: OnboardingStarter[] = [
  {
    id: 'portfolio-site',
    title: 'Portfolio Site',
    description: 'Personal homepage, projects, testimonials, and contact flow.',
    mode: 'fast',
    icon: LayoutTemplate,
    prompt: `Build a polished portfolio website for a senior product designer with:
- a bold hero section with intro copy, profile image placeholder, and primary CTA
- featured project case studies with metrics, screenshots, and process notes
- testimonials, services, contact form UI, and responsive navigation
- dark modern React + TypeScript frontend with Tailwind CSS
- realistic seed content and complete loading/empty states
- no backend, no auth, no database, and no server runtime claims`,
  },
  {
    id: 'todo-app',
    title: 'To-Do App',
    description: 'Productive task board with filters, priorities, and saved views.',
    mode: 'fast',
    icon: ListTodo,
    prompt: `Build a production-quality to-do and task planning app with:
- responsive React + TypeScript frontend and Tailwind CSS
- task lists, priority labels, due dates, status filters, and search
- today, upcoming, completed, and backlog views
- drag-ready visual task cards and helpful empty states
- local seeded task data so the preview feels complete immediately
- no backend, no auth, no database, and no server runtime claims`,
  },
  {
    id: 'landing-page',
    title: 'Landing Page',
    description: 'Launch page with offer, proof, pricing, FAQ, and conversion CTA.',
    mode: 'fast',
    icon: PanelsTopLeft,
    prompt: `Build a high-converting SaaS landing page for an AI operations tool with:
- first-screen hero, product screenshot placeholder, and clear CTA
- feature sections, integrations row, customer proof, pricing cards, and FAQ
- responsive React + TypeScript frontend with Tailwind CSS
- realistic copy, complete hover states, and mobile navigation
- no backend, no auth, no database, and no server runtime claims`,
  },
]
