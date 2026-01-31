// APEX.BUILD Error Boundary Component
// Catches and handles React errors with cyberpunk styling

import React, { Component, ErrorInfo, ReactNode } from 'react'
import { Card, CardContent, CardHeader, CardTitle, Button } from '@/components/ui'
import { AlertTriangle, RefreshCw, Bug, Home } from 'lucide-react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
  onError?: (error: Error, errorInfo: ErrorInfo) => void
}

interface State {
  hasError: boolean
  error: Error | null
  errorInfo: ErrorInfo | null
  errorId: string
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = {
      hasError: false,
      error: null,
      errorInfo: null,
      errorId: ''
    }
  }

  static getDerivedStateFromError(error: Error): Partial<State> {
    // Update state so the next render will show the fallback UI
    return {
      hasError: true,
      error,
      errorId: `error-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
    }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    // Log error details
    console.error('ErrorBoundary caught an error:', error, errorInfo)

    this.setState({
      error,
      errorInfo
    })

    // Call custom error handler if provided
    this.props.onError?.(error, errorInfo)

    // Log to external service (would be configured in real app)
    this.logErrorToService(error, errorInfo)
  }

  logErrorToService = (error: Error, errorInfo: ErrorInfo) => {
    // In a real app, this would send to Sentry, LogRocket, etc.
    const errorReport = {
      message: error.message,
      stack: error.stack,
      componentStack: errorInfo.componentStack,
      errorId: this.state.errorId,
      timestamp: new Date().toISOString(),
      userAgent: navigator.userAgent,
      url: window.location.href
    }

    // For now, just console log (replace with actual service)
    console.error('Error Report:', errorReport)
  }

  handleRefresh = () => {
    // Clear error state and try again
    this.setState({
      hasError: false,
      error: null,
      errorInfo: null,
      errorId: ''
    })
  }

  handleReload = () => {
    window.location.reload()
  }

  handleGoHome = () => {
    window.location.href = '/'
  }

  copyErrorDetails = async () => {
    const errorDetails = `
APEX.BUILD Error Report
=======================
Error ID: ${this.state.errorId}
Time: ${new Date().toISOString()}
Message: ${this.state.error?.message}
Stack: ${this.state.error?.stack}
Component Stack: ${this.state.errorInfo?.componentStack}
URL: ${window.location.href}
User Agent: ${navigator.userAgent}
    `.trim()

    // Use modern Clipboard API as primary, with fallback for older browsers
    if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      try {
        await navigator.clipboard.writeText(errorDetails)
        alert('Error details copied to clipboard!')
        return
      } catch {
        // Fall through to legacy method
      }
    }

    // Legacy fallback for older browsers (execCommand is deprecated but still works)
    try {
      const textArea = document.createElement('textarea')
      textArea.value = errorDetails
      textArea.style.position = 'fixed'
      textArea.style.left = '-9999px'
      textArea.style.top = '-9999px'
      document.body.appendChild(textArea)
      textArea.focus()
      textArea.select()
      document.execCommand('copy')
      document.body.removeChild(textArea)
      alert('Error details copied to clipboard!')
    } catch {
      alert('Failed to copy error details. Please manually copy the error information.')
    }
  }

  render() {
    if (this.state.hasError) {
      // Custom fallback UI
      if (this.props.fallback) {
        return this.props.fallback
      }

      // Default cyberpunk error UI
      return (
        <div className="min-h-screen bg-gradient-to-br from-gray-950 via-gray-900 to-red-950 flex items-center justify-center p-4">
          {/* Background effects - simplified for performance */}
          <div className="absolute inset-0 pointer-events-none">
            <div className="absolute top-1/4 left-1/4 w-64 h-64 bg-red-500/5 rounded-full" />
            <div className="absolute bottom-1/4 right-1/4 w-64 h-64 bg-orange-500/5 rounded-full" />
          </div>

          <Card variant="cyberpunk" className="w-full max-w-2xl relative z-10 border-red-500/50">
            <CardHeader className="text-center">
              <div className="flex items-center justify-center gap-3 mb-4">
                <div className="w-12 h-12 bg-red-500/20 rounded-lg flex items-center justify-center border border-red-500/50">
                  <AlertTriangle className="w-6 h-6 text-red-400" />
                </div>
                <div>
                  <h1 className="text-xl font-bold text-white">System Error Detected</h1>
                  <p className="text-xs text-red-400">APEX.BUILD Error Handler</p>
                </div>
              </div>

              <CardTitle className="text-lg text-red-400">
                Something went wrong in the matrix...
              </CardTitle>
              <p className="text-sm text-gray-400">
                Error ID: <code className="text-red-400 bg-red-900/20 px-2 py-1 rounded">{this.state.errorId}</code>
              </p>
            </CardHeader>

            <CardContent className="space-y-4">
              {/* Error Message */}
              <div className="bg-red-900/20 border border-red-500/30 rounded-lg p-4">
                <h3 className="text-sm font-semibold text-red-400 mb-2 flex items-center gap-2">
                  <Bug size={16} />
                  Error Details
                </h3>
                <p className="text-sm text-gray-300 font-mono">
                  {this.state.error?.message || 'Unknown error occurred'}
                </p>
              </div>

              {/* Component Stack (collapsible) */}
              <details className="bg-gray-900/50 border border-gray-700/50 rounded-lg">
                <summary className="p-4 cursor-pointer text-sm font-medium text-gray-300 hover:text-white">
                  Component Stack Trace
                </summary>
                <div className="px-4 pb-4">
                  <pre className="text-xs text-gray-400 font-mono overflow-auto max-h-40 whitespace-pre-wrap">
                    {this.state.errorInfo?.componentStack || 'No component stack available'}
                  </pre>
                </div>
              </details>

              {/* Error Stack (collapsible) */}
              <details className="bg-gray-900/50 border border-gray-700/50 rounded-lg">
                <summary className="p-4 cursor-pointer text-sm font-medium text-gray-300 hover:text-white">
                  JavaScript Stack Trace
                </summary>
                <div className="px-4 pb-4">
                  <pre className="text-xs text-gray-400 font-mono overflow-auto max-h-40 whitespace-pre-wrap">
                    {this.state.error?.stack || 'No stack trace available'}
                  </pre>
                </div>
              </details>

              {/* Action Buttons */}
              <div className="flex flex-col sm:flex-row gap-3 pt-4 border-t border-gray-700/50">
                <Button
                  onClick={this.handleRefresh}
                  variant="primary"
                  icon={<RefreshCw size={16} />}
                  className="flex-1"
                >
                  Try Again
                </Button>

                <Button
                  onClick={this.handleReload}
                  variant="secondary"
                  icon={<RefreshCw size={16} />}
                  className="flex-1"
                >
                  Reload Page
                </Button>

                <Button
                  onClick={this.handleGoHome}
                  variant="ghost"
                  icon={<Home size={16} />}
                  className="flex-1"
                >
                  Go Home
                </Button>
              </div>

              <div className="flex justify-between items-center pt-2 text-xs text-gray-500">
                <span>Need help? Copy error details and contact support.</span>
                <Button
                  onClick={this.copyErrorDetails}
                  variant="ghost"
                  size="sm"
                >
                  Copy Details
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )
    }

    return this.props.children
  }
}

// Higher-order component for wrapping components with error boundary
export function withErrorBoundary<P extends object>(
  Component: React.ComponentType<P>,
  errorBoundaryProps?: Omit<Props, 'children'>
) {
  return function WrappedComponent(props: P) {
    return (
      <ErrorBoundary {...errorBoundaryProps}>
        <Component {...props} />
      </ErrorBoundary>
    )
  }
}

// Hook for error reporting
export function useErrorHandler() {
  return (error: Error, errorInfo?: any) => {
    console.error('Manual error report:', error, errorInfo)

    // In a real app, send to error tracking service
    const errorReport = {
      message: error.message,
      stack: error.stack,
      context: errorInfo,
      timestamp: new Date().toISOString(),
      url: window.location.href
    }

    console.error('Error Report:', errorReport)
  }
}

export default ErrorBoundary