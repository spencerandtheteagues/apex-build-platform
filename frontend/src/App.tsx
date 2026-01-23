// APEX.BUILD Main Application
// Cyberpunk cloud development platform

import React, { useEffect, useState } from 'react'
import { useStore } from './hooks/useStore'
import { IDELayout } from './components/ide/IDELayout'
import { LoadingOverlay, Card, CardContent, CardHeader, CardTitle, Button, Input } from './components/ui'
import { User, Mail, Lock, Eye, EyeOff, Zap } from 'lucide-react'
import './styles/globals.css'

function App() {
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

  const {
    user,
    isAuthenticated,
    isLoading,
    apiService,
    theme,
    login,
    register,
    checkAuth,
    setTheme
  } = useStore()

  // Check authentication status on app load
  useEffect(() => {
    checkAuth()
  }, [checkAuth])

  // Handle authentication
  const handleAuth = async (e: React.FormEvent) => {
    e.preventDefault()
    setAuthErrors({})
    setIsAuthenticating(true)

    try {
      if (isAuthMode === 'login') {
        await login({
          username: authData.username,
          password: authData.password,
        })
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
      <div className={`min-h-screen bg-gradient-to-br ${theme.colors.background} flex items-center justify-center`}>
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
      <div className="min-h-screen bg-gradient-to-br from-gray-950 via-gray-900 to-cyan-950 flex items-center justify-center p-4">
        {/* Background effects */}
        <div className="absolute inset-0 overflow-hidden pointer-events-none">
          <div className="absolute top-1/4 left-1/4 w-64 h-64 bg-cyan-500/10 rounded-full blur-3xl" />
          <div className="absolute bottom-1/4 right-1/4 w-64 h-64 bg-pink-500/10 rounded-full blur-3xl" />
          <div className="absolute top-1/2 left-1/2 w-32 h-32 bg-green-400/10 rounded-full blur-2xl transform -translate-x-1/2 -translate-y-1/2" />
        </div>

        <Card variant="cyberpunk" glow="intense" className="w-full max-w-md relative z-10">
          <CardHeader className="text-center">
            <div className="flex items-center justify-center gap-3 mb-4">
              <div className="w-12 h-12 bg-gradient-to-br from-cyan-400 to-blue-600 rounded-lg flex items-center justify-center">
                <Zap className="w-6 h-6 text-white" />
              </div>
              <div>
                <h1 className="text-2xl font-bold text-white">APEX.BUILD</h1>
                <p className="text-xs text-cyan-400">Cloud Development Platform</p>
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
            <form onSubmit={handleAuth} className="space-y-4">
              {/* Username */}
              <Input
                label="Username"
                placeholder="Enter your username"
                value={authData.username}
                onChange={(e) => setAuthData(prev => ({ ...prev, username: e.target.value }))}
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
                  onChange={(e) => setAuthData(prev => ({ ...prev, email: e.target.value }))}
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
                onChange={(e) => setAuthData(prev => ({ ...prev, password: e.target.value }))}
                error={authErrors.password}
                leftIcon={<Lock size={16} />}
                rightIcon={
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
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
                  onChange={(e) => setAuthData(prev => ({ ...prev, confirmPassword: e.target.value }))}
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
                    onClick={switchAuthMode}
                    className="text-cyan-400 hover:text-cyan-300 transition-colors underline"
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
            © 2026 APEX.BUILD - The Future of Development
          </p>
        </div>
      </div>
    )
  }

  // Main application
  return (
    <div className="h-screen overflow-hidden bg-gray-950">
      <IDELayout />
    </div>
  )
}

export default App