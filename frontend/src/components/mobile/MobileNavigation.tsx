// APEX.BUILD Mobile Navigation Component
// Bottom tab bar and hamburger menu for mobile devices

import React, { useState, useRef, useEffect } from 'react'
import { cn } from '@/lib/utils'
import { useSwipeGesture, useIsMobile, useSafeAreaInsets } from '@/hooks/useMobile'
import {
  Menu,
  X,
  Folder,
  Code,
  Terminal,
  Brain,
  Settings,
  Search,
  GitBranch,
  User,
  Bell,
  Home,
  Rocket,
  Shield,
  ChevronRight,
  Layers,
  LogOut,
} from 'lucide-react'

export interface MobileNavigationProps {
  activeTab: 'explorer' | 'editor' | 'terminal' | 'ai' | 'settings'
  onTabChange: (tab: 'explorer' | 'editor' | 'terminal' | 'ai' | 'settings') => void
  onMenuToggle?: (isOpen: boolean) => void
  viewMode?: 'builder' | 'ide' | 'admin'
  onViewModeChange?: (mode: 'builder' | 'ide' | 'admin') => void
  user?: { username: string; avatar_url?: string; is_admin?: boolean }
  onLogout?: () => void
  className?: string
}

export const MobileNavigation: React.FC<MobileNavigationProps> = ({
  activeTab,
  onTabChange,
  onMenuToggle,
  viewMode = 'ide',
  onViewModeChange,
  user,
  onLogout,
  className,
}) => {
  const [isMenuOpen, setIsMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
  const safeArea = useSafeAreaInsets()

  // Handle menu open/close
  const toggleMenu = () => {
    const newState = !isMenuOpen
    setIsMenuOpen(newState)
    onMenuToggle?.(newState)
  }

  // Close menu on swipe left
  useSwipeGesture(menuRef, {
    onSwipeLeft: () => {
      setIsMenuOpen(false)
      onMenuToggle?.(false)
    },
  })

  // Close menu when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setIsMenuOpen(false)
        onMenuToggle?.(false)
      }
    }

    if (isMenuOpen) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [isMenuOpen, onMenuToggle])

  // Tab items for bottom navigation
  const tabs = [
    { id: 'explorer' as const, icon: Folder, label: 'Files' },
    { id: 'editor' as const, icon: Code, label: 'Editor' },
    { id: 'terminal' as const, icon: Terminal, label: 'Terminal' },
    { id: 'ai' as const, icon: Brain, label: 'AI' },
    { id: 'settings' as const, icon: Settings, label: 'More' },
  ]

  return (
    <>
      {/* Slide-out Menu Overlay */}
      <div
        className={cn(
          'fixed inset-0 bg-black/60 backdrop-blur-sm z-40 transition-opacity duration-300',
          isMenuOpen ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'
        )}
        onClick={() => {
          setIsMenuOpen(false)
          onMenuToggle?.(false)
        }}
      />

      {/* Slide-out Menu */}
      <div
        ref={menuRef}
        className={cn(
          'fixed top-0 left-0 h-full w-72 bg-gray-900 border-r border-gray-800 z-50',
          'transform transition-transform duration-300 ease-out',
          isMenuOpen ? 'translate-x-0' : '-translate-x-full'
        )}
        style={{ paddingTop: safeArea.top }}
      >
        {/* Menu Header */}
        <div className="h-14 flex items-center justify-between px-4 border-b border-gray-800">
          <div className="flex items-center gap-2">
            <div className="w-10 h-10 bg-gradient-to-br from-red-600 to-red-900 rounded-lg flex items-center justify-center p-1">
              <img src="/logo.png" alt="APEX" className="w-full h-full object-contain" />
            </div>
            <span className="text-lg font-bold text-white">APEX.BUILD</span>
          </div>
          <button
            onClick={toggleMenu}
            className="p-2 rounded-lg hover:bg-gray-800 transition-colors touch-target"
          >
            <X className="w-5 h-5 text-gray-400" />
          </button>
        </div>

        {/* User Section */}
        {user && (
          <div className="p-4 border-b border-gray-800">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-full bg-gradient-to-br from-red-600 to-red-900 flex items-center justify-center text-white font-bold">
                {user.username?.charAt(0).toUpperCase()}
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium text-white truncate">{user.username}</div>
                <div className="text-xs text-gray-400">Online</div>
              </div>
              <button className="p-2 rounded-lg hover:bg-gray-800 transition-colors touch-target">
                <Bell className="w-4 h-4 text-gray-400" />
              </button>
            </div>
          </div>
        )}

        {/* View Mode Switcher */}
        <div className="p-4 border-b border-gray-800">
          <div className="text-xs uppercase text-gray-500 mb-2 font-medium">Workspace</div>
          <div className="space-y-1">
            <button
              onClick={() => onViewModeChange?.('builder')}
              className={cn(
                'w-full flex items-center gap-3 px-3 py-3 rounded-lg transition-colors touch-target',
                viewMode === 'builder'
                  ? 'bg-red-900/20 text-red-400 border border-red-900/50'
                  : 'hover:bg-gray-800 text-gray-300'
              )}
            >
              <img src="/logo.png" alt="APEX" className="w-6 h-6 object-contain" />
              <span className="flex-1 text-left">Build App</span>
              <ChevronRight className="w-4 h-4 opacity-50" />
            </button>
            <button
              onClick={() => onViewModeChange?.('ide')}
              className={cn(
                'w-full flex items-center gap-3 px-3 py-3 rounded-lg transition-colors touch-target',
                viewMode === 'ide'
                  ? 'bg-red-900/20 text-red-400 border border-red-900/50'
                  : 'hover:bg-gray-800 text-gray-300'
              )}
            >
              <Code className="w-5 h-5" />
              <span className="flex-1 text-left">IDE</span>
              <ChevronRight className="w-4 h-4 opacity-50" />
            </button>
            {user?.is_admin && (
              <button
                onClick={() => onViewModeChange?.('admin')}
                className={cn(
                  'w-full flex items-center gap-3 px-3 py-3 rounded-lg transition-colors touch-target',
                  viewMode === 'admin'
                    ? 'bg-purple-900/20 text-purple-400 border border-purple-900/50'
                    : 'hover:bg-gray-800 text-gray-300'
                )}
              >
                <Shield className="w-5 h-5" />
                <span className="flex-1 text-left">Admin</span>
                <ChevronRight className="w-4 h-4 opacity-50" />
              </button>
            )}
          </div>
        </div>

        {/* Quick Actions */}
        <div className="p-4 border-b border-gray-800">
          <div className="text-xs uppercase text-gray-500 mb-2 font-medium">Quick Actions</div>
          <div className="space-y-1">
            <button className="w-full flex items-center gap-3 px-3 py-3 rounded-lg hover:bg-gray-800 text-gray-300 transition-colors touch-target">
              <Search className="w-5 h-5" />
              <span className="flex-1 text-left">Search</span>
              <span className="text-xs text-gray-500">Cmd+K</span>
            </button>
            <button className="w-full flex items-center gap-3 px-3 py-3 rounded-lg hover:bg-gray-800 text-gray-300 transition-colors touch-target">
              <GitBranch className="w-5 h-5" />
              <span className="flex-1 text-left">Git</span>
            </button>
            <button className="w-full flex items-center gap-3 px-3 py-3 rounded-lg hover:bg-gray-800 text-gray-300 transition-colors touch-target">
              <Layers className="w-5 h-5" />
              <span className="flex-1 text-left">Projects</span>
            </button>
          </div>
        </div>

        {/* Bottom Actions */}
        <div className="absolute bottom-0 left-0 right-0 p-4 border-t border-gray-800 bg-gray-900">
          <div className="space-y-1">
            <button className="w-full flex items-center gap-3 px-3 py-3 rounded-lg hover:bg-gray-800 text-gray-300 transition-colors touch-target">
              <Settings className="w-5 h-5" />
              <span className="flex-1 text-left">Settings</span>
            </button>
            <button
              onClick={onLogout}
              className="w-full flex items-center gap-3 px-3 py-3 rounded-lg hover:bg-red-900/20 text-red-400 transition-colors touch-target"
            >
              <LogOut className="w-5 h-5" />
              <span className="flex-1 text-left">Sign Out</span>
            </button>
          </div>
        </div>
      </div>

      {/* Bottom Tab Bar */}
      <nav
        className={cn(
          'fixed bottom-0 left-0 right-0 z-30 bg-gray-900/95 backdrop-blur-lg border-t border-gray-800',
          'md:hidden', // Only show on mobile
          className
        )}
        style={{ paddingBottom: safeArea.bottom }}
      >
        <div className="flex items-center justify-around h-16">
          {tabs.map((tab) => {
            const Icon = tab.icon
            const isActive = activeTab === tab.id

            return (
              <button
                key={tab.id}
                onClick={() => {
                  if (tab.id === 'settings') {
                    toggleMenu()
                  } else {
                    onTabChange(tab.id)
                  }
                }}
                className={cn(
                  'flex flex-col items-center justify-center flex-1 h-full touch-target',
                  'transition-colors duration-200',
                  isActive ? 'text-red-500' : 'text-gray-500 hover:text-gray-300'
                )}
              >
                <Icon className={cn('w-5 h-5 mb-1', isActive && 'text-red-500')} />
                <span className={cn('text-xs', isActive && 'text-red-500 font-medium')}>
                  {tab.label}
                </span>
                {isActive && (
                  <div className="absolute top-0 left-1/2 -translate-x-1/2 w-8 h-0.5 bg-red-500 rounded-full" />
                )}
              </button>
            )
          })}
        </div>
      </nav>

      {/* Mobile Header Bar */}
      <header
        className={cn(
          'fixed top-0 left-0 right-0 z-30 bg-gray-900/95 backdrop-blur-lg border-b border-gray-800',
          'md:hidden', // Only show on mobile
          className
        )}
        style={{ paddingTop: safeArea.top }}
      >
        <div className="flex items-center justify-between h-14 px-4">
          {/* Menu Button */}
          <button
            onClick={toggleMenu}
            className="p-2 -ml-2 rounded-lg hover:bg-gray-800 transition-colors touch-target"
          >
            <Menu className="w-5 h-5 text-gray-400" />
          </button>

          {/* Logo */}
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 bg-gradient-to-br from-red-600 to-red-900 rounded flex items-center justify-center p-0.5">
              <img src="/logo.png" alt="APEX" className="w-full h-full object-contain" />
            </div>
            <span className="text-sm font-bold text-white">APEX.BUILD</span>
          </div>

          {/* Actions */}
          <div className="flex items-center gap-1">
            <button className="p-2 rounded-lg hover:bg-gray-800 transition-colors touch-target">
              <Search className="w-5 h-5 text-gray-400" />
            </button>
          </div>
        </div>
      </header>
    </>
  )
}

// Mobile Panel Switcher Component
export interface MobilePanelSwitcherProps {
  panels: Array<{
    id: string
    label: string
    icon: React.ReactNode
  }>
  activePanel: string
  onPanelChange: (panelId: string) => void
  className?: string
}

export const MobilePanelSwitcher: React.FC<MobilePanelSwitcherProps> = ({
  panels,
  activePanel,
  onPanelChange,
  className,
}) => {
  const containerRef = useRef<HTMLDivElement>(null)
  const activeIndex = panels.findIndex((p) => p.id === activePanel)

  // Handle swipe between panels
  useSwipeGesture(containerRef, {
    onSwipeLeft: () => {
      const nextIndex = Math.min(activeIndex + 1, panels.length - 1)
      onPanelChange(panels[nextIndex].id)
    },
    onSwipeRight: () => {
      const prevIndex = Math.max(activeIndex - 1, 0)
      onPanelChange(panels[prevIndex].id)
    },
  })

  return (
    <div ref={containerRef} className={cn('flex overflow-x-auto scrollbar-hide', className)}>
      <div className="flex gap-2 p-2 min-w-full">
        {panels.map((panel) => {
          const isActive = panel.id === activePanel
          return (
            <button
              key={panel.id}
              onClick={() => onPanelChange(panel.id)}
              className={cn(
                'flex items-center gap-2 px-4 py-2 rounded-lg whitespace-nowrap transition-colors touch-target',
                isActive
                  ? 'bg-red-900/20 text-red-400 border border-red-900/50'
                  : 'bg-gray-800/50 text-gray-400 hover:bg-gray-800'
              )}
            >
              {panel.icon}
              <span className="text-sm font-medium">{panel.label}</span>
            </button>
          )
        })}
      </div>
    </div>
  )
}

// Export utility for touch target class
export const touchTargetClass = 'touch-target'

export default MobileNavigation
