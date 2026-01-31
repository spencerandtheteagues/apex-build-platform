// APEX.BUILD Main Application
// Dark Demon Theme - Cloud Development Platform

import React, { useState, useEffect, useRef } from 'react'
import { useStore } from './hooks/useStore'
import { IDELayout } from './components/ide/IDELayout'
import { AppBuilder } from './components/builder/AppBuilder'
import { AdminDashboard } from './components/admin/AdminDashboard'
import { ExplorePage } from './pages/Explore'
import { GitHubImportWizard } from './components/import/GitHubImportWizard'
import APIKeySettings from './components/settings/APIKeySettings'
import ModelSelector from './components/ai/ModelSelector'
// Import ErrorBoundary directly to be safe
import { ErrorBoundary } from './components/ui/ErrorBoundary'
import { LoadingOverlay, Card, CardContent, CardHeader, CardTitle, Button, Input, AnimatedBackground } from './components/ui'
import { User, Mail, Lock, Eye, EyeOff, Zap, Rocket, Code2, Shield, AlertTriangle, Check, Sparkles, Globe, Settings, Github, ChevronDown, Key } from 'lucide-react'
import './styles/globals.css'

// Premium Auth Screen Styles
const authStyles = `
  @keyframes particleFloat {
    0%, 100% { transform: translateY(0) translateX(0) rotate(0deg); opacity: 0.6; }
    25% { transform: translateY(-30px) translateX(10px) rotate(90deg); opacity: 0.8; }
    50% { transform: translateY(-20px) translateX(-10px) rotate(180deg); opacity: 0.4; }
    75% { transform: translateY(-40px) translateX(5px) rotate(270deg); opacity: 0.7; }
  }

  @keyframes gridPulse {
    0%, 100% { opacity: 0.03; }
    50% { opacity: 0.08; }
  }

  @keyframes gradientShift {
    0% { background-position: 0% 50%; }
    50% { background-position: 100% 50%; }
    100% { background-position: 0% 50%; }
  }

  @keyframes borderRotate {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
  }

  @keyframes cardFloat {
    0%, 100% { transform: translateY(0); }
    50% { transform: translateY(-8px); }
  }

  @keyframes logoPulse {
    0%, 100% { box-shadow: 0 0 20px rgba(220, 38, 38, 0.5), 0 0 40px rgba(220, 38, 38, 0.3); }
    50% { box-shadow: 0 0 30px rgba(220, 38, 38, 0.8), 0 0 60px rgba(220, 38, 38, 0.5), 0 0 80px rgba(220, 38, 38, 0.3); }
  }

  @keyframes textGradient {
    0% { background-position: 0% 50%; }
    50% { background-position: 100% 50%; }
    100% { background-position: 0% 50%; }
  }

  @keyframes fadeInUp {
    0% { opacity: 0; transform: translateY(20px); }
    100% { opacity: 1; transform: translateY(0); }
  }

  @keyframes slideInError {
    0% { opacity: 0; transform: translateX(-20px); }
    100% { opacity: 1; transform: translateX(0); }
  }

  @keyframes buttonGlow {
    0%, 100% { box-shadow: 0 0 20px rgba(220, 38, 38, 0.4); }
    50% { box-shadow: 0 0 35px rgba(220, 38, 38, 0.7), 0 0 50px rgba(220, 38, 38, 0.4); }
  }

  @keyframes spinBorder {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
  }

  @keyframes checkmarkDraw {
    0% { stroke-dashoffset: 50; }
    100% { stroke-dashoffset: 0; }
  }

  @keyframes underlineExpand {
    0% { width: 0; }
    100% { width: 100%; }
  }

  @keyframes counterFade {
    0% { opacity: 0; transform: scale(0.8); }
    100% { opacity: 1; transform: scale(1); }
  }

  @keyframes inputFocus {
    0% { box-shadow: 0 0 0 rgba(220, 38, 38, 0); }
    100% { box-shadow: 0 0 20px rgba(220, 38, 38, 0.3), inset 0 0 10px rgba(220, 38, 38, 0.1); }
  }

  @keyframes iconBounce {
    0%, 100% { transform: scale(1); }
    50% { transform: scale(1.2); }
  }

  @keyframes hudCorner {
    0%, 100% { opacity: 0.5; }
    50% { opacity: 1; }
  }

  @keyframes staggerIn {
    0% { opacity: 0; transform: translateY(15px); }
    100% { opacity: 1; transform: translateY(0); }
  }

  .auth-particle {
    position: absolute;
    width: 6px;
    height: 6px;
    background: radial-gradient(circle, rgba(220, 38, 38, 0.8) 0%, transparent 70%);
    border-radius: 50%;
    pointer-events: none;
  }

  .auth-grid-bg {
    background-image:
      linear-gradient(rgba(220, 38, 38, 0.1) 1px, transparent 1px),
      linear-gradient(90deg, rgba(220, 38, 38, 0.1) 1px, transparent 1px);
    background-size: 50px 50px;
    animation: gridPulse 4s ease-in-out infinite;
  }

  .auth-gradient-bg {
    background: linear-gradient(-45deg, #000000, #1a0000, #0d0d0d, #1a0505, #000000);
    background-size: 400% 400%;
    animation: gradientShift 15s ease infinite;
  }

  .auth-card {
    position: relative;
    background: rgba(10, 10, 10, 0.85);
    backdrop-filter: blur(20px);
    -webkit-backdrop-filter: blur(20px);
    border: 1px solid rgba(220, 38, 38, 0.2);
    transition: all 0.3s ease;
  }

  .auth-card:hover {
    animation: cardFloat 3s ease-in-out infinite;
    border-color: rgba(220, 38, 38, 0.4);
  }

  .auth-card::before {
    content: '';
    position: absolute;
    inset: -2px;
    border-radius: inherit;
    padding: 2px;
    background: conic-gradient(from 0deg, transparent, rgba(220, 38, 38, 0.8), transparent, rgba(220, 38, 38, 0.4), transparent);
    -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
    -webkit-mask-composite: xor;
    mask-composite: exclude;
    animation: borderRotate 4s linear infinite;
    opacity: 0;
    transition: opacity 0.3s ease;
  }

  .auth-card:hover::before {
    opacity: 1;
  }

  .auth-logo {
    animation: logoPulse 2s ease-in-out infinite;
    transition: all 0.3s ease;
  }

  .auth-logo:hover {
    transform: scale(1.05);
  }

  .auth-title {
    background: linear-gradient(90deg, #ef4444, #dc2626, #b91c1c, #dc2626, #ef4444);
    background-size: 200% auto;
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    animation: textGradient 3s linear infinite;
  }

  .auth-tagline {
    animation: fadeInUp 0.8s ease-out 0.3s both;
  }

  .auth-input-wrapper {
    position: relative;
  }

  .auth-input-wrapper input:focus {
    animation: inputFocus 0.3s ease forwards;
    border-color: rgba(220, 38, 38, 0.6) !important;
  }

  .auth-input-wrapper.focused .auth-input-icon {
    animation: iconBounce 0.3s ease;
    color: #ef4444;
  }

  .auth-input-label {
    transition: all 0.2s ease;
  }

  .auth-input-wrapper.focused .auth-input-label,
  .auth-input-wrapper.has-value .auth-input-label {
    transform: translateY(-24px) scale(0.85);
    color: #ef4444;
  }

  .auth-button {
    position: relative;
    background: linear-gradient(135deg, #dc2626 0%, #991b1b 50%, #dc2626 100%);
    background-size: 200% 200%;
    transition: all 0.3s ease;
    overflow: hidden;
  }

  .auth-button:hover:not(:disabled) {
    background-position: 100% 0;
    animation: buttonGlow 1.5s ease-in-out infinite;
    transform: translateY(-2px);
  }

  .auth-button:active:not(:disabled) {
    transform: translateY(0);
  }

  .auth-button.loading::after {
    content: '';
    position: absolute;
    inset: -4px;
    border: 2px solid transparent;
    border-top-color: rgba(255, 255, 255, 0.8);
    border-radius: inherit;
    animation: spinBorder 1s linear infinite;
  }

  .auth-button.success {
    background: linear-gradient(135deg, #16a34a 0%, #15803d 100%);
  }

  .auth-link {
    position: relative;
    display: inline-block;
  }

  .auth-link::after {
    content: '';
    position: absolute;
    bottom: -2px;
    left: 0;
    width: 0;
    height: 1px;
    background: linear-gradient(90deg, #ef4444, #dc2626);
    transition: width 0.3s ease;
  }

  .auth-link:hover::after {
    width: 100%;
  }

  .auth-link:hover {
    color: #f87171;
  }

  .auth-footer {
    animation: counterFade 0.8s ease-out 0.5s both;
  }

  .auth-error {
    animation: slideInError 0.3s ease-out;
  }

  .auth-field-stagger-1 { animation: staggerIn 0.5s ease-out 0.1s both; }
  .auth-field-stagger-2 { animation: staggerIn 0.5s ease-out 0.2s both; }
  .auth-field-stagger-3 { animation: staggerIn 0.5s ease-out 0.3s both; }
  .auth-field-stagger-4 { animation: staggerIn 0.5s ease-out 0.4s both; }
  .auth-field-stagger-5 { animation: staggerIn 0.5s ease-out 0.5s both; }

  .hud-corner {
    position: absolute;
    width: 20px;
    height: 20px;
    border: 2px solid rgba(220, 38, 38, 0.5);
    animation: hudCorner 2s ease-in-out infinite;
  }

  .hud-corner-tl { top: -1px; left: -1px; border-right: none; border-bottom: none; }
  .hud-corner-tr { top: -1px; right: -1px; border-left: none; border-bottom: none; }
  .hud-corner-bl { bottom: -1px; left: -1px; border-right: none; border-top: none; }
  .hud-corner-br { bottom: -1px; right: -1px; border-left: none; border-top: none; }

  .auth-mode-transition {
    transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  }
`;

// Particle component for animated background
const AuthParticle: React.FC<{ delay: number; startX: number; startY: number }> = ({ delay, startX, startY }) => (
  <div
    className="auth-particle"
    style={{
      left: `${startX}%`,
      top: `${startY}%`,
      animationDelay: `${delay}s`,
      animation: `particleFloat ${4 + Math.random() * 4}s ease-in-out infinite`,
    }}
  />
);

type AppView = 'builder' | 'ide' | 'admin' | 'explore' | 'settings'

function App() {
  const [currentView, setCurrentView] = useState<AppView>('builder')
  const [showSettingsDropdown, setShowSettingsDropdown] = useState(false)
  const [showGitHubImport, setShowGitHubImport] = useState(false)
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
    currentProject, // We need this to check if we can safely render the IDE
    login,
    register,
  } = useStore()

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
      <div className="min-h-screen flex items-center justify-center relative overflow-hidden">
        {/* Animated cyberpunk background */}
        <AnimatedBackground variant="full" intensity="medium" interactive={false} />

        <div className="flex flex-col items-center space-y-6 relative z-10">
          <div className="w-48 h-48 flex items-center justify-center animate-pulse">
            <img src="/logo.png" alt="APEX Logo" className="w-full h-full object-contain drop-shadow-[0_0_30px_rgba(239,68,68,0.5)]" />
          </div>
          <div className="flex flex-col items-center gap-2">
            <p className="text-red-400 text-sm animate-pulse">Initializing System...</p>
          </div>
        </div>
      </div>
    )
  }

  // Authentication screen - PREMIUM SHOWSTOPPER
  if (!isAuthenticated) {
    return (
      <>
        {/* Inject premium styles */}
        <style dangerouslySetInnerHTML={{ __html: authStyles }} />

        <div className="min-h-screen flex items-center justify-center p-4 relative overflow-hidden">
          {/* Animated cyberpunk background with full effects */}
          <AnimatedBackground variant="full" intensity="high" interactive={true} />

          {/* Premium glassmorphism card */}
          <div className="auth-card w-full max-w-md relative z-10 rounded-2xl p-8 auth-mode-transition">
            {/* HUD Corner decorations */}
            <div className="hud-corner hud-corner-tl rounded-tl-2xl" />
            <div className="hud-corner hud-corner-tr rounded-tr-2xl" />
            <div className="hud-corner hud-corner-bl rounded-bl-2xl" />
            <div className="hud-corner hud-corner-br rounded-br-2xl" />

            {/* Logo Section */}
            <div className="text-center mb-8">
              <div className="flex flex-col items-center gap-4">
                {/* Animated Logo */}
                <div className="auth-logo w-[8.4rem] h-[8.4rem] flex items-center justify-center relative">
                  <img src="/Apex-Build-Logo1.png" alt="APEX Logo" className="w-full h-full object-contain relative z-10 drop-shadow-[0_0_20px_rgba(239,68,68,0.4)]" />
                </div>

                {/* Animated title */}
                <div>
                  <h1 className="auth-title text-3xl font-black tracking-wider">
                    APEX.BUILD
                  </h1>
                  <p className="auth-tagline text-sm text-red-400/80 mt-1 flex items-center justify-center gap-2">
                    <Sparkles size={14} className="text-red-500" />
                    Cloud Development Platform
                    <Sparkles size={14} className="text-red-500" />
                  </p>
                </div>
              </div>

              {/* Welcome text with mode transition */}
              <div className="mt-6 auth-mode-transition">
                <h2 className="text-xl font-bold text-white mb-2">
                  {isAuthMode === 'login' ? 'Welcome Back' : 'Join the Future'}
                </h2>
                <p className="text-sm text-gray-400">
                  {isAuthMode === 'login'
                    ? 'Sign in to continue your development journey'
                    : 'Create an account to start building tomorrow'}
                </p>
              </div>
            </div>

            {/* Form */}
            <form onSubmit={handleAuth} className="space-y-5">
              {/* Username field */}
              <div className={`auth-input-wrapper auth-field-stagger-1 ${authData.username ? 'has-value' : ''}`}>
                <div className="relative">
                  <div className="absolute left-3 top-1/2 -translate-y-1/2 auth-input-icon text-gray-500 transition-colors z-10">
                    <User size={18} />
                  </div>
                  <input
                    type="text"
                    value={authData.username}
                    onChange={(e) => setAuthData(prev => ({ ...prev, username: e.target.value }))}
                    onFocus={(e) => e.target.closest('.auth-input-wrapper')?.classList.add('focused')}
                    onBlur={(e) => e.target.closest('.auth-input-wrapper')?.classList.remove('focused')}
                    className="w-full bg-black/50 border border-gray-700/50 rounded-xl py-3.5 pl-11 pr-4 text-white placeholder-transparent focus:outline-none focus:border-red-600/60 transition-all duration-300"
                    placeholder="Username"
                    required
                  />
                  <label className="auth-input-label absolute left-11 top-1/2 -translate-y-1/2 text-gray-500 pointer-events-none transition-all duration-200">
                    {authData.username ? '' : 'Username'}
                  </label>
                </div>
                {authErrors.username && (
                  <p className="auth-error text-xs text-red-400 mt-1.5 pl-3">{authErrors.username}</p>
                )}
              </div>

              {/* Email field (register only) */}
              {isAuthMode === 'register' && (
                <div className={`auth-input-wrapper auth-field-stagger-2 ${authData.email ? 'has-value' : ''}`}>
                  <div className="relative">
                    <div className="absolute left-3 top-1/2 -translate-y-1/2 auth-input-icon text-gray-500 transition-colors z-10">
                      <Mail size={18} />
                    </div>
                    <input
                      type="email"
                      value={authData.email}
                      onChange={(e) => setAuthData(prev => ({ ...prev, email: e.target.value }))}
                      onFocus={(e) => e.target.closest('.auth-input-wrapper')?.classList.add('focused')}
                      onBlur={(e) => e.target.closest('.auth-input-wrapper')?.classList.remove('focused')}
                      className="w-full bg-black/50 border border-gray-700/50 rounded-xl py-3.5 pl-11 pr-4 text-white placeholder-transparent focus:outline-none focus:border-red-600/60 transition-all duration-300"
                      placeholder="Email"
                      required
                    />
                    <label className="auth-input-label absolute left-11 top-1/2 -translate-y-1/2 text-gray-500 pointer-events-none transition-all duration-200">
                      {authData.email ? '' : 'Email'}
                    </label>
                  </div>
                  {authErrors.email && (
                    <p className="auth-error text-xs text-red-400 mt-1.5 pl-3">{authErrors.email}</p>
                  )}
                </div>
              )}

              {/* Password field */}
              <div className={`auth-input-wrapper ${isAuthMode === 'register' ? 'auth-field-stagger-3' : 'auth-field-stagger-2'} ${authData.password ? 'has-value' : ''}`}>
                <div className="relative">
                  <div className="absolute left-3 top-1/2 -translate-y-1/2 auth-input-icon text-gray-500 transition-colors z-10">
                    <Lock size={18} />
                  </div>
                  <input
                    type={showPassword ? 'text' : 'password'}
                    value={authData.password}
                    onChange={(e) => setAuthData(prev => ({ ...prev, password: e.target.value }))}
                    onFocus={(e) => e.target.closest('.auth-input-wrapper')?.classList.add('focused')}
                    onBlur={(e) => e.target.closest('.auth-input-wrapper')?.classList.remove('focused')}
                    className="w-full bg-black/50 border border-gray-700/50 rounded-xl py-3.5 pl-11 pr-12 text-white placeholder-transparent focus:outline-none focus:border-red-600/60 transition-all duration-300"
                    placeholder="Password"
                    required
                  />
                  <label className="auth-input-label absolute left-11 top-1/2 -translate-y-1/2 text-gray-500 pointer-events-none transition-all duration-200">
                    {authData.password ? '' : 'Password'}
                  </label>
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-red-400 transition-colors duration-200 p-1"
                  >
                    {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
                  </button>
                </div>
                {authErrors.password && (
                  <p className="auth-error text-xs text-red-400 mt-1.5 pl-3">{authErrors.password}</p>
                )}
              </div>

              {/* Confirm Password (register only) */}
              {isAuthMode === 'register' && (
                <div className={`auth-input-wrapper auth-field-stagger-4 ${authData.confirmPassword ? 'has-value' : ''}`}>
                  <div className="relative">
                    <div className="absolute left-3 top-1/2 -translate-y-1/2 auth-input-icon text-gray-500 transition-colors z-10">
                      <Lock size={18} />
                    </div>
                    <input
                      type={showPassword ? 'text' : 'password'}
                      value={authData.confirmPassword}
                      onChange={(e) => setAuthData(prev => ({ ...prev, confirmPassword: e.target.value }))}
                      onFocus={(e) => e.target.closest('.auth-input-wrapper')?.classList.add('focused')}
                      onBlur={(e) => e.target.closest('.auth-input-wrapper')?.classList.remove('focused')}
                      className="w-full bg-black/50 border border-gray-700/50 rounded-xl py-3.5 pl-11 pr-4 text-white placeholder-transparent focus:outline-none focus:border-red-600/60 transition-all duration-300"
                      placeholder="Confirm Password"
                      required
                    />
                    <label className="auth-input-label absolute left-11 top-1/2 -translate-y-1/2 text-gray-500 pointer-events-none transition-all duration-200">
                      {authData.confirmPassword ? '' : 'Confirm Password'}
                    </label>
                  </div>
                  {authErrors.confirmPassword && (
                    <p className="auth-error text-xs text-red-400 mt-1.5 pl-3">{authErrors.confirmPassword}</p>
                  )}
                </div>
              )}

              {/* General error message */}
              {authErrors.general && (
                <div className="auth-error p-4 bg-red-900/20 border border-red-500/30 rounded-xl backdrop-blur-sm">
                  <div className="flex items-center gap-2">
                    <AlertTriangle size={16} className="text-red-400 flex-shrink-0" />
                    <p className="text-sm text-red-400">{authErrors.general}</p>
                  </div>
                </div>
              )}

              {/* Premium Submit Button */}
              <button
                type="submit"
                disabled={isAuthenticating}
                className={`auth-button ${isAuthMode === 'register' ? 'auth-field-stagger-5' : 'auth-field-stagger-3'} ${isAuthenticating ? 'loading' : ''} w-full py-4 rounded-xl text-white font-bold text-lg tracking-wide disabled:opacity-70 disabled:cursor-not-allowed relative overflow-hidden`}
              >
                <span className={`flex items-center justify-center gap-2 transition-all duration-300 ${isAuthenticating ? 'opacity-0' : 'opacity-100'}`}>
                  {isAuthMode === 'login' ? (
                    <>
                      <Zap size={20} />
                      Sign In
                    </>
                  ) : (
                    <>
                      <Rocket size={20} />
                      Create Account
                    </>
                  )}
                </span>
                {isAuthenticating && (
                  <span className="absolute inset-0 flex items-center justify-center">
                    <svg className="animate-spin h-6 w-6 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                  </span>
                )}
              </button>

              {/* Switch auth mode */}
              <div className="text-center pt-6 border-t border-gray-800/50 mt-6">
                <p className="text-sm text-gray-400">
                  {isAuthMode === 'login' ? "Don't have an account? " : 'Already have an account? '}
                  <button
                    type="button"
                    onClick={switchAuthMode}
                    className="auth-link text-red-400 font-medium transition-colors duration-200"
                  >
                    {isAuthMode === 'login' ? 'Sign up' : 'Sign in'}
                  </button>
                </p>
              </div>
            </form>
          </div>

          {/* Premium Footer */}
          <div className="auth-footer absolute bottom-6 left-1/2 transform -translate-x-1/2 text-center">
            <p className="text-xs text-gray-600 flex items-center gap-2">
              <span className="inline-block w-8 h-px bg-gradient-to-r from-transparent via-red-900/50 to-transparent" />
              <span className="text-gray-500">
                <span className="text-red-500/70">2026</span> APEX.BUILD - The Future of Development
              </span>
              <span className="inline-block w-8 h-px bg-gradient-to-r from-transparent via-red-900/50 to-transparent" />
            </p>
          </div>
        </div>
      </>
    )
  }

  // Main application with view switching
  return (
    <div className="h-screen flex flex-col bg-black">
      {/* Top Navigation */}
      <div className="h-12 bg-black/90 border-b border-red-900/30 flex items-center justify-between px-4 z-50 relative">
        {/* Logo */}
        <div className="flex items-center gap-3">
          <div className="relative">
            <div className="w-12 h-12 bg-gradient-to-br from-red-600 to-red-900 rounded-lg flex items-center justify-center shadow-lg shadow-red-900/30 p-1">
              <img src="/logo.png" alt="APEX Logo" className="w-full h-full object-contain" />
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
            onClick={() => setCurrentView('builder')}
            className={`flex items-center gap-2 px-4 py-1.5 rounded-md transition-all duration-200 ${
              currentView === 'builder'
                ? 'bg-red-900/20 text-red-400 border border-red-900/50 shadow-sm shadow-red-900/20'
                : 'text-gray-400 hover:text-white hover:bg-gray-800'
            }`}
          >
            <img src="/logo.png" alt="APEX" className="w-5 h-5 object-contain" />
            <span className="text-sm font-medium">Build App</span>
          </button>
          <button
            onClick={() => setCurrentView('ide')}
            className={`flex items-center gap-2 px-4 py-1.5 rounded-md transition-all duration-200 ${
              currentView === 'ide'
                ? 'bg-red-900/20 text-red-400 border border-red-900/50 shadow-sm shadow-red-900/20'
                : 'text-gray-400 hover:text-white hover:bg-gray-800'
            }`}
          >
            <Code2 className="w-4 h-4" />
            <span className="text-sm font-medium">IDE</span>
          </button>
          <button
            onClick={() => setCurrentView('explore')}
            className={`flex items-center gap-2 px-4 py-1.5 rounded-md transition-all duration-200 ${
              currentView === 'explore'
                ? 'bg-purple-900/20 text-purple-400 border border-purple-900/50 shadow-sm shadow-purple-900/20'
                : 'text-gray-400 hover:text-white hover:bg-gray-800'
            }`}
          >
            <Globe className="w-4 h-4" />
            <span className="text-sm font-medium">Explore</span>
          </button>
          {/* Admin button - only show for admin users */}
          {(user?.is_admin || user?.is_super_admin) && (
            <button
              onClick={() => setCurrentView('admin')}
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
            <button
              onClick={() => setCurrentView('settings')}
              className={`flex items-center gap-2 px-3 py-1.5 rounded-md transition-all duration-200 ${
                currentView === 'settings'
                  ? 'bg-red-900/20 text-red-400 border border-red-900/50'
                  : 'text-gray-400 hover:text-white hover:bg-gray-800'
              }`}
              title="Settings & API Keys"
            >
              <Settings className="w-4 h-4" />
            </button>
            <span className="text-sm text-gray-400">{user.username}</span>
            <div className="w-8 h-8 rounded-full bg-gradient-to-br from-red-600 to-red-900 flex items-center justify-center text-white text-sm font-bold shadow-lg shadow-red-900/30">
              {user.username?.charAt(0).toUpperCase()}
            </div>
          </div>
        )}
      </div>

      {/* Main Content - Wrapped in ErrorBoundary and safely rendered */}
      <div className="flex-1 overflow-hidden relative">
        <div className={`absolute inset-0 overflow-y-auto ${currentView === 'builder' ? 'block' : 'hidden'}`}>
          <ErrorBoundary>
            <AppBuilder onNavigateToIDE={() => setCurrentView('ide')} />
          </ErrorBoundary>
        </div>
        
        <div className={`absolute inset-0 ${currentView === 'ide' ? 'block' : 'hidden'}`}>
          <ErrorBoundary>
            {currentProject ? (
               <IDELayout />
            ) : (
               <div className="h-full flex flex-col items-center justify-center bg-black text-gray-400">
                  <AlertTriangle className="w-16 h-16 text-yellow-500 mb-4" />
                  <h2 className="text-xl font-bold text-white mb-2">No Project Selected</h2>
                  <p className="max-w-md text-center mb-6">
                    Please use the <span className="text-red-400 font-bold">Build App</span> tab to create or select a project before opening the IDE.
                  </p>
                  <Button onClick={() => setCurrentView('builder')} variant="primary">
                    Go to Builder
                  </Button>
               </div>
            )}
          </ErrorBoundary>
        </div>

        <div className={`absolute inset-0 ${currentView === 'admin' ? 'block' : 'hidden'}`}>
          <ErrorBoundary>
            <AdminDashboard />
          </ErrorBoundary>
        </div>

        <div className={`absolute inset-0 overflow-y-auto ${currentView === 'explore' ? 'block' : 'hidden'}`}>
          <ErrorBoundary>
            <ExplorePage />
          </ErrorBoundary>
        </div>

        <div className={`absolute inset-0 overflow-y-auto ${currentView === 'settings' ? 'block' : 'hidden'}`}>
          <ErrorBoundary>
            <div className="min-h-full bg-black p-6">
              <div className="max-w-4xl mx-auto space-y-8">
                <div>
                  <h1 className="text-3xl font-black text-transparent bg-clip-text bg-gradient-to-r from-red-500 to-orange-500 mb-2">
                    Settings
                  </h1>
                  <p className="text-gray-400">Configure your AI providers and API keys</p>
                </div>

                {/* Model Selector Section */}
                <div className="bg-gray-900/50 border border-gray-800 rounded-xl p-6">
                  <h2 className="text-xl font-bold text-white mb-4 flex items-center gap-2">
                    <Sparkles className="w-5 h-5 text-red-400" />
                    Default AI Model
                  </h2>
                  <p className="text-gray-400 text-sm mb-4">
                    Select the default AI model for your builds. You can override this per-project.
                  </p>
                  <ModelSelector
                    onChange={(provider, model) => console.log('Selected:', provider, model)}
                    className="w-full max-w-md"
                  />
                </div>

                {/* API Keys Section */}
                <div className="bg-gray-900/50 border border-gray-800 rounded-xl p-6">
                  <h2 className="text-xl font-bold text-white mb-4 flex items-center gap-2">
                    <Key className="w-5 h-5 text-red-400" />
                    API Keys (BYOK)
                  </h2>
                  <p className="text-gray-400 text-sm mb-6">
                    Bring Your Own Keys - Add your own API keys to use your personal quotas and get better rates.
                  </p>
                  <APIKeySettings />
                </div>
              </div>
            </div>
          </ErrorBoundary>
        </div>
      </div>

      {/* GitHub Import Modal */}
      {showGitHubImport && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <ErrorBoundary>
            <GitHubImportWizard onClose={() => setShowGitHubImport(false)} />
          </ErrorBoundary>
        </div>
      )}
    </div>
  )
}

export default App