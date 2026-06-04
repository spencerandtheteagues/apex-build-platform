// APEX-BUILD Onboarding Tour
// First-time user guided walkthrough that ends in a guided first-run flow

import React, { useState, useEffect, useCallback, useRef } from 'react'
import { cn } from '@/lib/utils'
import {
  Sparkles,
  Rocket,
  Zap,
  Cpu,
  MessageSquare,
  ChevronRight,
  ChevronLeft,
  X,
  CheckCircle2,
  Bot,
  Code2,
  LayoutTemplate,
  ListTodo,
  PanelsTopLeft,
} from 'lucide-react'
import { onboardingStarters, type OnboardingStarter } from './onboardingStarters'

const ONBOARDING_KEY = 'apex_onboarding_completed'

interface TourStep {
  title: string
  description: string
  icon: React.ReactNode
  details: string[]
  tip?: string
}

const tourSteps: TourStep[] = [
  {
    title: 'Welcome to APEX-BUILD',
    description: 'Build complete applications from a simple description. Our AI agents handle everything — planning, coding, testing, and deployment.',
    icon: <Rocket className="w-8 h-8" />,
    details: [
      'Describe what you want to build in plain English',
      'Multiple AI agents collaborate to build your app',
      'Get production-ready code in minutes, not days',
      'Download, edit, and deploy your finished project',
    ],
    tip: 'The more detailed your description, the better the result. Include features, pages, and functionality you need.',
  },
  {
    title: 'Describe Your App',
    description: 'Start by writing a clear description of the application you want to build. Be specific about features, user flows, and design preferences.',
    icon: <MessageSquare className="w-8 h-8" />,
    details: [
      '"Build a task management app with user auth, drag-and-drop boards, and dark mode"',
      '"Create an e-commerce store with product listings, shopping cart, and Stripe checkout"',
      '"Make a real-time chat app with rooms, file sharing, and typing indicators"',
    ],
    tip: 'Include the tech stack you prefer (React, Node.js, PostgreSQL) or leave it on Auto and let the AI choose.',
  },
  {
    title: 'Choose Your Tech Stack',
    description: 'Select the technologies you want, or let the AI choose the best fit. You can pick frontend, backend, and database separately.',
    icon: <Cpu className="w-8 h-8" />,
    details: [
      'Auto (Best Fit) — AI selects optimal technologies for your app',
      'Mix and match — pick React frontend with Go backend and PostgreSQL',
      'Popular stacks: Next.js + Node.js, React + Python FastAPI, Vue + Go',
      'Free plan supports static frontend sites; Builder+ unlocks backend and full-stack builds',
    ],
    tip: 'Auto mode analyzes your description and picks the most suitable stack. Great for when you\'re unsure.',
  },
  {
    title: 'Select AI Power Mode',
    description: 'Control which AI models build your app. Higher power means better code quality and higher cost.',
    icon: <Zap className="w-8 h-8" />,
    details: [
      'Fast & Cheap (1.6x) — Budget models for quick prototypes and experiments',
      'Balanced (1.8x) — Mid-tier models for solid production code',
      'Max Power (2.0x) - Flagship routing across Claude Opus 4.7, ChatGPT 5.4, Gemini 3.1 Pro, Grok 4.20, and Kimi K2.6/local models',
    ],
    tip: 'Start with Fast mode to test your idea, then rebuild with Max Power once you\'re happy with the concept.',
  },
  {
    title: 'Watch the Build',
    description: 'Once you click Build, multiple AI agents spring into action. Watch them plan, code, and test in real-time.',
    icon: <Bot className="w-8 h-8" />,
    details: [
      'Planner agent analyzes requirements and creates a build plan',
      'Architect agent designs the system structure',
      'Frontend/Backend agents generate actual code files',
      'Tester agent validates the code, Reviewer checks quality',
    ],
    tip: 'You can chat with the Lead agent during the build to ask questions or request changes.',
  },
  {
    title: 'What Are You Building?',
    description: 'Choose a guided starter to jump into your first build right away, or open a blank workspace to start from scratch.',
    icon: <Sparkles className="w-8 h-8" />,
    details: [
      'Each starter pre-fills the builder with a production-ready prompt',
      'Builds begin immediately after you select a starter or choose blank',
      'You can refine the prompt, change settings, or edit the generated app',
      'Every build produces a working preview you can inspect and share',
    ],
    tip: 'Starters use Fast mode for rapid iteration. Switch to Full or Max Power for deeper backend and architecture work.',
  },
]

interface OnboardingTourProps {
  onComplete?: () => void
  onStarterSelect?: (starter: OnboardingStarter) => void
  onOpenBlankWorkspace?: () => void
  forceShow?: boolean
  onPromptPrefill?: (prompt: string, mode: 'fast' | 'full') => void
}

export const OnboardingTour: React.FC<OnboardingTourProps> = ({
  onComplete,
  onStarterSelect,
  onOpenBlankWorkspace,
  forceShow = false,
  onPromptPrefill,
}) => {
  const [isVisible, setIsVisible] = useState(false)
  const [currentStep, setCurrentStep] = useState(0)
  const [isExiting, setIsExiting] = useState(false)
  const modalRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (forceShow) {
      setIsVisible(true)
      return
    }
    const completed = localStorage.getItem(ONBOARDING_KEY)
    if (!completed) {
      // Small delay so the page loads first
      const timer = setTimeout(() => setIsVisible(true), 800)
      return () => clearTimeout(timer)
    }
  }, [forceShow])

  useEffect(() => {
    if (isVisible) {
      modalRef.current?.focus()
    }
  }, [currentStep, isVisible])

  const completeTour = useCallback(() => {
    localStorage.setItem(ONBOARDING_KEY, 'true')
    onComplete?.()
  }, [onComplete])

  const handleClose = useCallback(() => {
    setIsExiting(true)
    setTimeout(() => {
      setIsVisible(false)
      setIsExiting(false)
      completeTour()
    }, 300)
  }, [completeTour])

  const handleStarterSelect = useCallback((starter: OnboardingStarter) => {
    if (onPromptPrefill) {
      onPromptPrefill(starter.prompt, starter.mode);
    }
    onStarterSelect?.(starter)
    handleClose()
  }, [handleClose, onStarterSelect, onPromptPrefill])

  const handleOpenBlankWorkspace = useCallback(() => {
    onOpenBlankWorkspace?.()
    handleClose()
  }, [handleClose, onOpenBlankWorkspace])

  const handleNext = useCallback(() => {
    if (currentStep < tourSteps.length - 1) {
      setCurrentStep(prev => prev + 1)
    }
  }, [currentStep])

  const handlePrev = useCallback(() => {
    if (currentStep > 0) {
      setCurrentStep(prev => prev - 1)
    }
  }, [currentStep])

  const handleSkip = useCallback(() => {
    handleClose()
  }, [handleClose])

  if (!isVisible) return null

  const step = tourSteps[currentStep]
  const isLast = currentStep === tourSteps.length - 1
  const progress = ((currentStep + 1) / tourSteps.length) * 100
  const titleId = 'apex-onboarding-title'
  const descriptionId = 'apex-onboarding-description'

  return (
    <div className={cn(
      'fixed inset-0 z-[9999] flex items-center justify-center p-4',
      'transition-all duration-300',
      isExiting ? 'opacity-0' : 'opacity-100'
    )}>
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/80 backdrop-blur-sm"
        aria-hidden="true"
        onClick={handleSkip}
      />

      {/* Modal */}
      <div className={cn(
        'relative w-full max-w-lg rounded-2xl overflow-hidden',
        'bg-gradient-to-b from-gray-900 to-gray-950 border border-gray-700/50',
        'shadow-2xl shadow-red-500/10',
        'transition-all duration-300',
        isExiting ? 'scale-95 opacity-0' : 'scale-100 opacity-100'
      )}
        ref={modalRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={descriptionId}
        tabIndex={-1}
      >
        {/* Progress bar */}
        <div className="h-1 bg-gray-800">
          <div
            className="h-full bg-gradient-to-r from-red-500 to-orange-500 transition-all duration-500"
            style={{ width: `${progress}%` }}
          />
        </div>

        {/* Close button */}
        <button
          type="button"
          aria-label="Close onboarding tour"
          onClick={handleSkip}
          className="absolute top-4 right-4 p-1.5 rounded-lg text-gray-500 hover:text-gray-300 hover:bg-gray-800/50 transition-colors z-10"
        >
          <X className="w-4 h-4" />
        </button>

        {/* Step counter */}
        <div className="absolute top-4 left-5 text-xs text-gray-500 font-mono">
          {currentStep + 1} / {tourSteps.length}
        </div>

        {/* Content */}
        <div className="p-8 pt-12">
          {/* Icon */}
          <div className="mb-5 flex justify-center">
            <div className="w-16 h-16 rounded-2xl bg-red-500/10 border border-red-500/30 flex items-center justify-center text-red-400">
              {step.icon}
            </div>
          </div>

          {/* Title */}
          <h2 id={titleId} className="text-2xl font-bold text-white text-center mb-3">
            {step.title}
          </h2>

          {/* Description */}
          <p id={descriptionId} className="text-gray-400 text-center text-sm leading-relaxed mb-6">
            {step.description}
          </p>

          {/* Details */}
          <div className="space-y-2.5 mb-5">
            {step.details.map((detail, i) => (
              <div key={i} className="flex items-start gap-3 text-sm">
                <CheckCircle2 className="w-4 h-4 text-green-400 shrink-0 mt-0.5" />
                <span className="text-gray-300 leading-relaxed">{detail}</span>
              </div>
            ))}
          </div>

          {/* Tip */}
          {step.tip && (
            <div className="p-3 rounded-lg bg-yellow-500/5 border border-yellow-500/20 mb-6">
              <p className="text-xs text-yellow-400/80 leading-relaxed">
                <strong className="text-yellow-400">Pro tip:</strong> {step.tip}
              </p>
            </div>
          )}

          {isLast && (
            <div className="mb-6 space-y-3">
              <div className="grid gap-2 sm:grid-cols-3">
                {onboardingStarters.map((starter) => {
                  const StarterIcon = starter.icon
                  return (
                    <button
                      key={starter.id}
                      type="button"
                      onClick={() => handleStarterSelect(starter)}
                      aria-label={`Prefill prompt: ${starter.title}`}
                      className="group min-h-[132px] rounded-xl border border-red-500/20 bg-red-500/[0.04] p-3 text-left transition-all hover:border-red-400/45 hover:bg-red-500/10"
                    >
                      <span className="mb-3 flex h-9 w-9 items-center justify-center rounded-lg border border-red-400/25 bg-black/35 text-red-300 transition-colors group-hover:text-red-200">
                        <StarterIcon className="h-5 w-5" />
                      </span>
                      <span className="block text-sm font-bold text-white">{starter.title}</span>
                      <span className="mt-2 block text-[11px] leading-5 text-gray-400">{starter.description}</span>
                      <span className="mt-3 inline-flex items-center gap-1 text-[11px] font-semibold text-red-200">
                        Prefill prompt
                        <ChevronRight className="h-3 w-3" />
                      </span>
                    </button>
                  )
                })}
              </div>
              {onOpenBlankWorkspace && (
                <button
                  type="button"
                  onClick={handleOpenBlankWorkspace}
                  className="flex w-full items-center justify-center gap-2 rounded-xl border border-gray-700 bg-gray-900/70 px-4 py-3 text-sm font-semibold text-gray-200 transition-colors hover:border-gray-600 hover:bg-gray-800 hover:text-white"
                >
                  <Code2 className="h-4 w-4" />
                  Open blank workspace
                </button>
              )}
            </div>
          )}
        </div>

        {/* Footer with navigation */}
        <div className="px-8 pb-6 flex items-center justify-between">
          <div className="flex items-center gap-2">
            {currentStep > 0 ? (
              <button
                type="button"
                onClick={handlePrev}
                className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm text-gray-400 hover:text-white hover:bg-gray-800/50 transition-colors"
              >
                <ChevronLeft className="w-4 h-4" />
                Back
              </button>
            ) : (
              <button
                type="button"
                onClick={handleSkip}
                className="px-3 py-2 rounded-lg text-sm text-gray-500 hover:text-gray-300 transition-colors"
              >
                Skip tour
              </button>
            )}
          </div>

          <div className="flex items-center gap-1.5">
            {tourSteps.map((_, i) => (
              <button
                key={i}
                type="button"
                aria-label={`Go to onboarding step ${i + 1}`}
                onClick={() => setCurrentStep(i)}
                className={cn(
                  'w-2 h-2 rounded-full transition-all duration-200',
                  i === currentStep
                    ? 'w-6 bg-red-500'
                    : i < currentStep
                    ? 'bg-red-500/40'
                    : 'bg-gray-700'
                )}
              />
            ))}
          </div>

          {/* Only show Next button for non-last steps; last step requires explicit starter/blank selection */}
          {!isLast && (
            <button
              type="button"
              onClick={handleNext}
              className="flex items-center gap-1.5 px-5 py-2.5 rounded-lg text-sm font-medium transition-all bg-gray-800 text-white hover:bg-gray-700"
            >
              Next
              <ChevronRight className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

export default OnboardingTour
