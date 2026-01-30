// APEX.BUILD Main Application
// Dark Demon Theme - Cloud Development Platform
// Optimized with React.lazy, Suspense, and code splitting

import React, { useState, Suspense, lazy, memo } from 'react'
import { useUser, useIsAuthenticated, useIsLoading, useLogin, useRegister } from './hooks/useStore'
import { LoadingOverlay, Card, CardContent, CardHeader, CardTitle, Button, Input } from './components/ui'
import { User, Mail, Lock, Eye, EyeOff, Zap, Rocket, Code2, Shield } from 'lucide-react'
import './styles/globals.css'

// Lazy load heavy components
const IDELayout = lazy(() => import('./components/ide/IDELayout').then(m => ({ default: m.IDELayout })))
const AppBuilder = lazy(() => import('./components/builder/AppBuilder').then(m => ({ default: m.AppBuilder })))
const AdminDashboard = lazy(() => import('./components/admin/AdminDashboard').then(m => ({ default: m.AdminDashboard })))

type AppView = 'builder' | 'ide' | 'admin'

// Loading fallback component
const ViewLoadingFallback = memo(() => (
  <div className="flex items-center justify-center h-full bg-black">
    <LoadingOverlay
      isVisible={true}
      text="Loading..."
      variant="orb"
    />
  </div>
))
ViewLoadingFallback.displayName = 'ViewLoadingFallback'

// Memoized Auth Form Component
interface AuthFormProps {
  isAuthMode: 'login' | 'register'
  authData: {
    username: string
    email: string
    password: string
    confirmPassword: string
  }
  authErrors: Record<string, string>
  showPassword: boolean
  isAuthenticating: boolean
  onAuthDataChange: (data: Partial<AuthFormProps['authData']>) => void
  onShowPasswordToggle: () => void
  onSubmit: (e: React.FormEvent) => void
  onSwitchMode: () => void
}

const AuthForm = memo<AuthFormProps>(({
  isAuthMode,
  authData,
  authErrors,
  showPassword,
  isAuthenticating,
  onAuthDataChange,
  onShowPasswordToggle,
  onSubmit,
  onSwitchMode,
}) => (
  <div className="min-h-screen bg-gradient-to-br from-black via-gray-950 to-red-950/20 flex items-center justify-center p-4">
    {/* Background effects - demon theme */}
    <div className="absolute inset-0 pointer-events-none">
      <div className="absolute top-1/4 left-1/4 w-64 h-64 bg-red-600/5 rounded-full" />
      <div className="absolute bottom-1/4 right-1/4 w-64 h-64 bg-red-900/5 rounded-full" />
    </div>

    <Card variant="cyberpunk" glow="intense" className="w-full max-w-md relative z-10 border-red-900/30">
      <CardHeader className="text-center">
        <div className="flex items-center justify-center gap-3 mb-4">
          <div className="w-12 h-12 bg-gradient-to-br from-red-600 to-red-900 rounded-lg flex items-center justify-center shadow-lg shadow-red-900/50">
            <Zap className="w-6 h-6 text-white" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-white">APEX.BUILD</h1>
            <p className="text-xs text-red-400">Cloud Development Platform</p>
          </div>
        </div>

        <CardTitle className="text-xl">
          {isAuthMode === 'login' ? 'Welcome Back' : 'Join APEX.BUILD'}
        </CardTitle>
        <p className="text-sm text-gray-400">
          {isAuthMode === 'login'
            ? 'Sign in to continue your development journey'
            : 'Create an account to start building the future'
          }
        </p>
      </CardHeader>

      <CardContent>
        <form onSubmit={onSubmit} className="space-y-4">
          {/* Username */}
          <Input
            label="Username"
            placeholder="Enter your username"
            value={authData.username}
            onChange={(e) => onAuthDataChange({ username: e.target.value })}
            error={authErrors.username}
            leftIcon={<User size={16} />}
            variant="cyberpunk"
            required
          />

          {/* Email (registration only) */}
          {isAuthMode === 'register' && (
            <Input
              label="Email"
              type="email"
              placeholder="Enter your email"
              value={authData.email}
              onChange={(e) => onAuthDataChange({ email: e.target.value })}
              error={authErrors.email}
              leftIcon={<Mail size={16} />}
              variant="cyberpunk"
              required
            />
          )}

          {/* Password */}
          <Input
            label="Password"
            type={showPassword ? 'text' : 'password'}
            placeholder="Enter your password"
            value={authData.password}
            onChange={(e) => onAuthDataChange({ password: e.target.value })}
            error={authErrors.password}
            leftIcon={<Lock size={16} />}
            rightIcon={
              <button
                type="button"
                onClick={onShowPasswordToggle}
                className="text-gray-400 hover:text-gray-300 transition-colors"
              >
                {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            }
            variant="cyberpunk"
            required
          />

          {/* Confirm Password (registration only) */}
          {isAuthMode === 'register' && (
            <Input
              label="Confirm Password"
              type={showPassword ? 'text' : 'password'}
              placeholder="Confirm your password"
              value={authData.confirmPassword}
              onChange={(e) => onAuthDataChange({ confirmPassword: e.target.value })}
              error={authErrors.confirmPassword}
              leftIcon={<Lock size={16} />}
              variant="cyberpunk"
              required
            />
          )}

          {/* General errors */}
          {authErrors.general && (
            <div className="p-3 bg-red-900/20 border border-red-500/30 rounded-lg">
              <p className="text-sm text-red-400">{authErrors.general}</p>
            </div>
          )}

          {/* Submit button */}
          <Button
            type="submit"
            variant="primary"
            size="lg"
            className="w-full"
            loading={isAuthenticating}
            disabled={isAuthenticating}
          >
            {isAuthMode === 'login' ? 'Sign In' : 'Create Account'}
          </Button>

          {/* Switch mode */}
          <div className="text-center pt-4 border-t border-gray-700/50">
            <p className="text-sm text-gray-400">
              {isAuthMode === 'login'
                ? "Don't have an account? "
                : "Already have an account? "
              }
              <button
                type="button"
                onClick={onSwitchMode}
                className="text-red-400 hover:text-red-300 transition-colors underline"
              >
                {isAuthMode === 'login' ? 'Sign up' : 'Sign in'}
              </button>
            </p>
          </div>
        </form>
      </CardContent>
    </Card>

    {/* Footer */}
    <div className="absolute bottom-4 left-1/2 transform -translate-x-1/2 text-center">
      <p className="text-xs text-gray-500">
        2026 APEX.BUILD - The Future of Development
      </p>
    </div>
  </div>
))
AuthForm.displayName = 'AuthForm'

// Memoized Navigation Component
interface NavigationProps {
  currentView: AppView
  user: any
  onViewChange: (view: AppView) => void
}

const Navigation = memo<NavigationProps>(({ currentView, user, onViewChange }) => (
  <div className="h-12 bg-black/90 border-b border-red-900/30 flex items-center justify-between px-4 z-50 relative">
    {/* Logo */}
    <div className="flex items-center gap-3">
      <div className="relative">
        <div className="w-8 h-8 bg-gradient-to-br from-red-600 to-red-900 rounded-lg flex items-center justify-center shadow-lg shadow-red-900/30">
          <Zap className="w-5 h-5 text-white" />
        </div>
        <div className="absolute -inset-0.5 bg-gradient-to-br from-red-600 to-red-900 rounded-lg opacity-30" style={{ filter: 'blur(4px)' }} />
      </div>
      <span className="text-xl font-bold bg-gradient-to-r from-red-400 to-red-600 bg-clip-text text-transparent">
        APEX.BUILD
      </span>
    </div>

    {/* View Toggle */}
    <div className="flex items-center gap-2 bg-gray-900/50 rounded-lg p-1 border border-gray-800">
      <button
        onClick={() => onViewChange('builder')}
        className={`flex items-center gap-2 px-4 py-1.5 rounded-md transition-all duration-200 ${
          currentView === 'builder'
            ? 'bg-red-900/20 text-red-400 border border-red-900/50 shadow-sm shadow-red-900/20'
            : 'text-gray-400 hover:text-white hover:bg-gray-800'
        }`}
      >
        <Rocket className="w-4 h-4" />
        <span className="text-sm font-medium">Build App</span>
      </button>
      <button
        onClick={() => onViewChange('ide')}
        className={`flex items-center gap-2 px-4 py-1.5 rounded-md transition-all duration-200 ${
          currentView === 'ide'
            ? 'bg-red-900/20 text-red-400 border border-red-900/50 shadow-sm shadow-red-900/20'
            : 'text-gray-400 hover:text-white hover:bg-gray-800'
        }`}
      >
        <Code2 className="w-4 h-4" />
        <span className="text-sm font-medium">IDE</span>
      </button>
      {/* Admin button - only show for admin users */}
      {(user?.is_admin || user?.is_super_admin) && (
        <button
          onClick={() => onViewChange('admin')}
          className={`flex items-center gap-2 px-4 py-1.5 rounded-md transition-all duration-200 ${
            currentView === 'admin'
              ? 'bg-purple-900/20 text-purple-400 border border-purple-900/50 shadow-sm shadow-purple-900/20'
              : 'text-gray-400 hover:text-white hover:bg-gray-800'
          }`}
        >
          <Shield className="w-4 h-4" />
          <span className="text-sm font-medium">Admin</span>
        </button>
      )}
    </div>

    {/* User Info */}
    {user && (
      <div className="flex items-center gap-3">
        <span className="text-sm text-gray-400">{user.username}</span>
        <div className="w-8 h-8 rounded-full bg-gradient-to-br from-red-600 to-red-900 flex items-center justify-center text-white text-sm font-bold shadow-lg shadow-red-900/30">
          {user.username?.charAt(0).toUpperCase()}
        </div>
      </div>
    )}
  </div>
))
Navigation.displayName = 'Navigation'

function App() {
  const [currentView, setCurrentView] = useState<AppView>('builder')
  const [isAuthMode, setIsAuthMode] = useState<'login' | 'register'>('login')
  const [authData, setAuthData] = useState({
    username: '',
    email: '',
    password: '',
    confirmPassword: '',
  })
  const [authErrors, setAuthErrors] = useState<Record<string, string>>({})
  const [showPassword, setShowPassword] = useState(false)
  const [isAuthenticating, setIsAuthenticating] = useState(false)

  // Use optimized individual selectors
  const user = useUser()
  const isAuthenticated = useIsAuthenticated()
  const isLoading = useIsLoading()
  const login = useLogin()
  const register = useRegister()

  // Handle authentication
  const handleAuth = async (e: React.FormEvent) => {
    e.preventDefault()
    setAuthErrors({})
    setIsAuthenticating(true)

    try {
      if (isAuthMode === 'login') {
        await login(authData.username, authData.password)
      } else {
        // Validate registration data
        const errors: Record<string, string> = {}

        if (!authData.username.trim()) {
          errors.username = 'Username is required'
        } else if (authData.username.length < 3) {
          errors.username = 'Username must be at least 3 characters'
        }

        if (!authData.email.trim()) {
          errors.email = 'Email is required'
        } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(authData.email)) {
          errors.email = 'Please enter a valid email'
        }

        if (!authData.password) {
          errors.password = 'Password is required'
        } else if (authData.password.length < 8) {
          errors.password = 'Password must be at least 8 characters'
        }

        if (authData.password !== authData.confirmPassword) {
          errors.confirmPassword = 'Passwords do not match'
        }

        if (Object.keys(errors).length > 0) {
          setAuthErrors(errors)
          return
        }

        await register({
          username: authData.username,
          email: authData.email,
          password: authData.password,
        })
      }

      // Reset form
      setAuthData({
        username: '',
        email: '',
        password: '',
        confirmPassword: '',
      })
    } catch (error: any) {
      console.error('Authentication error:', error)
      setAuthErrors({
        general: error.message || 'Authentication failed'
      })
    } finally {
      setIsAuthenticating(false)
    }
  }

  // Handle auth data changes
  const handleAuthDataChange = (data: Partial<typeof authData>) => {
    setAuthData(prev => ({ ...prev, ...data }))
  }

  // Switch between login and register
  const switchAuthMode = () => {
    setIsAuthMode(isAuthMode === 'login' ? 'register' : 'login')
    setAuthErrors({})
    setAuthData({
      username: '',
      email: '',
      password: '',
      confirmPassword: '',
    })
  }

  // Loading screen
  if (isLoading) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-black via-gray-950 to-red-950/20 flex items-center justify-center">
        <LoadingOverlay
          isVisible={true}
          text="Initializing APEX.BUILD..."
          variant="orb"
        />
      </div>
    )
  }

  // Authentication screen
  if (!isAuthenticated) {
    return (
      <AuthForm
        isAuthMode={isAuthMode}
        authData={authData}
        authErrors={authErrors}
        showPassword={showPassword}
        isAuthenticating={isAuthenticating}
        onAuthDataChange={handleAuthDataChange}
        onShowPasswordToggle={() => setShowPassword(!showPassword)}
        onSubmit={handleAuth}
        onSwitchMode={switchAuthMode}
      />
    )
  }

  // Main application with view switching
  return (
    <div className="h-screen flex flex-col bg-black">
      {/* Top Navigation */}
      <Navigation
        currentView={currentView}
        user={user}
        onViewChange={setCurrentView}
      />

      {/* Main Content - Keep all views mounted to preserve state */}
      <div className="flex-1 overflow-hidden relative">
        <div className={`absolute inset-0 ${currentView === 'builder' ? 'block' : 'hidden'}`}>
          <Suspense fallback={<ViewLoadingFallback />}>
            <AppBuilder onNavigateToIDE={() => setCurrentView('ide')} />
          </Suspense>
        </div>
        <div className={`absolute inset-0 ${currentView === 'ide' ? 'block' : 'hidden'}`}>
          <Suspense fallback={<ViewLoadingFallback />}>
            <IDELayout />
          </Suspense>
        </div>
        <div className={`absolute inset-0 ${currentView === 'admin' ? 'block' : 'hidden'}`}>
          <Suspense fallback={<ViewLoadingFallback />}>
            <AdminDashboard />
          </Suspense>
        </div>
      </div>
    </div>
  )
}

export default App
